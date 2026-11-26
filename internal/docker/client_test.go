package docker

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsStopped(t *testing.T) {
	tests := []struct {
		state    string
		expected bool
	}{
		{"exited", true},
		{"created", true},
		{"dead", true},
		{"running", false},
		{"restarting", false},
		{"paused", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		if got := IsStopped(tt.state); got != tt.expected {
			t.Errorf("IsStopped(%q) = %v; want %v", tt.state, got, tt.expected)
		}
	}
}

func TestApplyInspect(t *testing.T) {
	var c Container
	var insp inspectResponse

	insp.State.StartedAt = "2026-09-19T10:00:00Z"
	insp.State.ExitCode = 137
	insp.State.OOMKilled = true
	insp.HostConfig.RestartPolicy.Name = "unless-stopped"
	insp.Config.Entrypoint = []string{"/bin/sh"}
	insp.Config.Cmd = []string{"-c", "exit 1"}
	insp.Mounts = []struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	}{
		{Type: "bind", Source: "/host/data", Destination: "/data", RW: false},
	}
	insp.NetworkSettings.Networks = map[string]struct {
		IPAddress string `json:"IPAddress"`
	}{
		"bridge": {IPAddress: "172.17.0.2"},
	}
	insp.HostConfig.PortBindings = map[string][]struct {
		HostPort string `json:"HostPort"`
	}{
		"80/tcp": {{HostPort: "8080"}},
	}

	applyInspect(&c, insp)

	if c.StartedAt != "2026-09-19T10:00:00Z" {
		t.Errorf("StartedAt = %q; want %q", c.StartedAt, "2026-09-19T10:00:00Z")
	}
	if c.ExitCode != 137 {
		t.Errorf("ExitCode = %d; want 137", c.ExitCode)
	}
	if !c.OOMKilled {
		t.Errorf("OOMKilled = false; want true")
	}
	if c.RestartPolicy != "unless-stopped" {
		t.Errorf("RestartPolicy = %q; want %q", c.RestartPolicy, "unless-stopped")
	}
	if c.Command != "/bin/sh -c exit 1" {
		t.Errorf("Command = %q; want %q", c.Command, "/bin/sh -c exit 1")
	}
	if len(c.Mounts) != 1 || !c.Mounts[0].ReadOnly {
		t.Errorf("Mounts = %+v; want 1 read-only mount", c.Mounts)
	}
	if len(c.Networks) != 1 || c.Networks[0].IP != "172.17.0.2" {
		t.Errorf("Networks = %+v; want 1 network with IP 172.17.0.2", c.Networks)
	}
	if len(c.Ports) != 1 || c.Ports[0] != "80/tcp → 8080" {
		t.Errorf("Ports = %+v; want ['80/tcp → 8080']", c.Ports)
	}
}

// dockerAPI serves just enough of the Docker Engine API for ListContainers.
func dockerAPI(t *testing.T, list string, failStatsFor string) *Client {
	t.Helper()
	srv := httptest.NewServer(dockerMux(list, failStatsFor))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
}

func dockerMux(list string, failStatsFor string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(list)) })
	mux.HandleFunc("/containers/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/stats"):
			if strings.Contains(r.URL.Path, failStatsFor) && failStatsFor != "" {
				http.Error(w, "gone", http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"cpu_stats":{"cpu_usage":{"total_usage":300},"system_cpu_usage":2000,"online_cpus":2},
				"precpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":1000},
				"memory_stats":{"usage":500,"limit":1000}}`))
		case strings.HasSuffix(r.URL.Path, "/json"):
			_, _ = w.Write([]byte(`{"State":{"StartedAt":"2026-01-01T00:00:00Z","ExitCode":0},
				"HostConfig":{"RestartPolicy":{"Name":"always"}}}`))
		default:
			http.NotFound(w, r)
		}
	})
	return mux
}

const containerList = `[
 {"Id":"aaa","Names":["/web"],"Image":"nginx","State":"running","Status":"Up 1 hour",
  "Labels":{"com.docker.compose.project":"site","naswarden.optional":"true"}},
 {"Id":"bbb","Names":["/old"],"Image":"busybox","State":"exited","Status":"Exited (0)","Labels":{}},
 {"Id":"ccc","Names":["/job"],"Image":"busybox","State":"exited","Status":"Exited (0)",
  "Labels":{"com.docker.compose.project":"site","com.docker.compose.oneoff":"True"}},
 {"Id":"ddd","Names":["/db"],"Image":"pg","State":"exited","Status":"Exited (1)",
  "Labels":{"com.docker.compose.project":"site"}}
]`

func TestListContainersFiltersAndFillsStats(t *testing.T) {
	c := dockerAPI(t, containerList, "")
	got, err := c.ListContainers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, x := range got {
		names = append(names, x.Name)
	}
	// A loose stopped container and a one-off `compose run` are dropped; a
	// stopped service of a compose project is kept (it should be running).
	if strings.Join(names, ",") != "web,db" {
		t.Fatalf("containers = %v, want [web db]", names)
	}
	web := got[0]
	if web.Stack != "site" || !web.Optional || web.RestartPolicy != "always" {
		t.Errorf("web = %+v, want stack site, optional, restart always", web)
	}
	if web.MemUsed != 500 || web.MemLimit != 1000 {
		t.Errorf("mem = %d/%d, want 500/1000", web.MemUsed, web.MemLimit)
	}
	// cpu delta 100 over system delta 1000, 2 CPUs => 20%.
	if web.CPUPercent < 19.99 || web.CPUPercent > 20.01 {
		t.Errorf("cpu = %v, want 20", web.CPUPercent)
	}
	if got[1].CPUPercent != 0 || got[1].MemUsed != 0 {
		t.Errorf("stopped container has usage %+v, want zero", got[1])
	}
	// Slices must be non-nil so they marshal as [] not null (the UI calls
	// .length on them).
	if web.Mounts == nil || web.Networks == nil || web.Ports == nil {
		t.Errorf("nil slice in %+v", web)
	}
}

func TestListContainersSurvivesAStatsFailure(t *testing.T) {
	c := dockerAPI(t, containerList, "aaa")
	got, err := c.ListContainers(context.Background())
	if err != nil {
		t.Fatalf("a failing stats call must not fail the listing: %v", err)
	}
	if len(got) != 2 || got[0].CPUPercent != 0 || got[0].RestartPolicy != "always" {
		t.Errorf("got %+v, want both containers, web with zero stats but inspect data", got)
	}
}

// A mounted Docker socket ("unix:///var/run/docker.sock") is used as is,
// without a proxy in front.
func TestListContainersOverUnixSocket(t *testing.T) {
	// Short path: a unix socket path is limited to ~108 bytes, t.TempDir()
	// can exceed that on some systems.
	dir, err := os.MkdirTemp("", "nwdock")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "docker.sock")

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: dockerMux(containerList, "")}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { srv.Close() })

	got, err := NewClient("unix://" + sock).ListContainers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "web" || got[0].MemUsed != 500 {
		t.Errorf("got %+v, want web and db with web's stats", got)
	}
}

// shortSocket listens on a Unix socket in a short temp path (socket paths are
// limited to ~108 bytes; t.TempDir() can exceed that).
func shortSocket(t *testing.T) (string, net.Listener) {
	t.Helper()
	dir, err := os.MkdirTemp("", "nwsock")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "docker.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return path, ln
}

func TestIsSocket(t *testing.T) {
	path, _ := shortSocket(t)
	if !IsSocket(path) {
		t.Errorf("IsSocket(%q) = false for a listening socket", path)
	}
	file := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if IsSocket(file) || IsSocket(filepath.Join(t.TempDir(), "missing")) {
		t.Error("IsSocket is true for a regular or missing file")
	}
}

func TestCheckSocket(t *testing.T) {
	path, _ := shortSocket(t)
	if err := CheckSocket(path); err != nil {
		t.Errorf("CheckSocket on a listening socket = %v", err)
	}
	if err := CheckSocket(path + ".missing"); err == nil || !strings.Contains(err.Error(), "cannot connect") {
		t.Errorf("missing socket: err = %v, want cannot connect", err)
	}

	// No permission: the error names the fix. Root ignores file
	// permissions, so this part only runs as a normal user.
	if os.Geteuid() == 0 {
		t.Log("running as root: skipping the permission case")
		return
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	if err := CheckSocket(path); err == nil || !strings.Contains(err.Error(), "socket's group") {
		t.Errorf("no permission: err = %v, want the group hint", err)
	}
}

func TestNewClientAddresses(t *testing.T) {
	for in, want := range map[string]string{
		"http://docker-proxy:2375/":   "http://docker-proxy:2375",
		"tcp://docker-proxy:2375":     "http://docker-proxy:2375", // DOCKER_HOST form
		"unix:///var/run/docker.sock": "http://docker",            // dialled over the socket
	} {
		if c := NewClient(in); c.baseURL != want {
			t.Errorf("NewClient(%q).baseURL = %q, want %q", in, c.baseURL, want)
		}
	}
}

func TestValidateHost(t *testing.T) {
	for _, ok := range []string{
		"tcp://docker-proxy:2375",
		"http://docker-proxy:2375",
		"http://docker-proxy:2375/",
		"https://docker.example:2376",
		"unix:///var/run/docker.sock",
	} {
		if err := ValidateHost(ok); err != nil {
			t.Errorf("ValidateHost(%q) = %v, want ok", ok, err)
		}
	}
	for _, bad := range []string{
		"docker-proxy:2375",     // no scheme
		"ssh://user@nas",        // Docker form naswarden can't use
		"fd://",                 // same
		"unix://",               // no socket path
		"unix://var/run/x.sock", // relative path
		"tcp://",                // no host
		"http://proxy:2375/api", // a path would be prefixed to every call
		"http://proxy:2375?x=1", // so would a query
		"http://proxy:2375?",    // even an empty one
		"tcp://proxy:2375#frag", // or a fragment
		"",                      // empty
	} {
		if err := ValidateHost(bad); err == nil {
			t.Errorf("ValidateHost(%q) = nil, want an error", bad)
		}
	}
}

// DOCKER_HOST=tcp://... reaches a proxy over plain HTTP.
func TestListContainersOverTCPAddress(t *testing.T) {
	srv := httptest.NewServer(dockerMux(containerList, ""))
	t.Cleanup(srv.Close)
	got, err := NewClient("tcp://" + strings.TrimPrefix(srv.URL, "http://")).ListContainers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("got %d containers, want 2", len(got))
	}
}

func TestListContainersReportsUnreachableProxy(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	c := NewClient(srv.URL)
	srv.Close()
	if _, err := c.ListContainers(context.Background()); err == nil {
		t.Fatal("expected an error when the proxy is unreachable")
	}
}

func TestCPUPercentGuardsAgainstBadDeltas(t *testing.T) {
	var s statsResponse
	if got := cpuPercent(s); got != 0 {
		t.Errorf("empty stats = %v, want 0", got)
	}
	s.CPUStats.CPUUsage.TotalUsage, s.CPUStats.SystemCPUUsage = 100, 100
	s.PreCPUStats.CPUUsage.TotalUsage, s.PreCPUStats.SystemCPUUsage = 200, 200 // counters went backwards
	if got := cpuPercent(s); got != 0 {
		t.Errorf("negative delta = %v, want 0", got)
	}
}

func TestCPUForUsesDockersSamplesWhenPresent(t *testing.T) {
	c := &Client{}
	var s statsResponse
	s.CPUStats.CPUUsage.TotalUsage, s.CPUStats.SystemCPUUsage, s.CPUStats.OnlineCPUs = 300, 2000, 2
	s.PreCPUStats.CPUUsage.TotalUsage, s.PreCPUStats.SystemCPUUsage = 200, 1000
	// cpu delta 100 over system delta 1000, 2 CPUs => 20%.
	if got := c.cpuFor("a", s, time.Now()); got < 19.99 || got > 20.01 {
		t.Errorf("cpu = %v, want 20", got)
	}
	if len(c.cpuPrev) != 0 {
		t.Errorf("remembered a reading it did not need: %v", c.cpuPrev)
	}
}

// Podman leaves precpu_stats empty: the use must come from the previous
// refresh's reading.
func TestCPUForUsesThePreviousReadingWhenPrecpuIsEmpty(t *testing.T) {
	c := &Client{}
	t0 := time.Unix(1000, 0)
	reading := func(total int64) statsResponse {
		var s statsResponse
		s.CPUStats.CPUUsage.TotalUsage = total
		return s
	}
	// A container that has used 600 s of CPU over its life reads 0 the first
	// time: there is nothing to compare with yet.
	if got := c.cpuFor("a", reading(600e9), t0); got != 0 {
		t.Errorf("first reading = %v, want 0", got)
	}
	// 60 s later it has used 60 more seconds: one full core.
	if got := c.cpuFor("a", reading(660e9), t0.Add(60*time.Second)); got < 99.99 || got > 100.01 {
		t.Errorf("full core = %v, want 100", got)
	}
	// Another 60 s, 15 s of CPU: a quarter of a core.
	if got := c.cpuFor("a", reading(675e9), t0.Add(120*time.Second)); got < 24.99 || got > 25.01 {
		t.Errorf("quarter core = %v, want 25", got)
	}
	// Two cores busy can read more than 100, like docker stats.
	if got := c.cpuFor("a", reading(795e9), t0.Add(180*time.Second)); got < 199.99 || got > 200.01 {
		t.Errorf("two cores = %v, want 200", got)
	}
	// Containers are tracked separately.
	if got := c.cpuFor("b", reading(5e9), t0.Add(180*time.Second)); got != 0 {
		t.Errorf("other container's first reading = %v, want 0", got)
	}
	// A restarted container's counter goes back to a small number.
	if got := c.cpuFor("a", reading(1e9), t0.Add(240*time.Second)); got != 0 {
		t.Errorf("after a restart = %v, want 0", got)
	}
}

func TestForgetCPUDropsContainersThatAreGone(t *testing.T) {
	c := &Client{cpuPrev: map[string]cpuSample{"keep": {}, "gone": {}}}
	c.forgetCPU([]containerSummary{{ID: "keep"}})
	if _, ok := c.cpuPrev["gone"]; ok || len(c.cpuPrev) != 1 {
		t.Errorf("cpuPrev = %v, want only keep", c.cpuPrev)
	}
}

func TestTrimLeadingSlash(t *testing.T) {
	for in, want := range map[string]string{"/web": "web", "web": "web", "": ""} {
		if got := trimLeadingSlash(in); got != want {
			t.Errorf("trimLeadingSlash(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNotListening(t *testing.T) {
	// A stale socket file: the path exists but nothing accepts connections.
	dir := t.TempDir()
	path := dir + "/docker.sock"
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	// Leave the socket file behind when closing, as a crashed Docker would.
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = l.Close()
	if err := CheckSocket(path); err == nil || !NotListening(err) {
		t.Errorf("CheckSocket on a dead socket = %v; NotListening should be true", err)
	}
	if NotListening(errors.New("something else")) {
		t.Error("an unrelated error is not NotListening")
	}
}
