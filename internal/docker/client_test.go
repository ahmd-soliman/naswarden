package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
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

func TestTrimLeadingSlash(t *testing.T) {
	for in, want := range map[string]string{"/web": "web", "web": "web", "": ""} {
		if got := trimLeadingSlash(in); got != want {
			t.Errorf("trimLeadingSlash(%q) = %q, want %q", in, got, want)
		}
	}
}
