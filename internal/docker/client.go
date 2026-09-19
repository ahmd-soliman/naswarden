// Package docker talks to the Docker Engine API over plain HTTP -- not a
// direct /var/run/docker.sock mount, which would be root-equivalent host
// access. naswarden is meant to connect to a tecnativa/docker-socket-proxy
// instance instead, configured with only CONTAINERS=1 (read-only GET
// access to list/inspect/stats; POST/write access is disabled by default
// in that proxy, confirmed against its own docs -- naswarden can never
// issue a start/stop/exec/delete call even if it wanted to).
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient takes the socket-proxy's base URL, e.g. "http://docker-proxy:2375".
func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{}}
}

type containerSummary struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`  // "running", "exited", ...
	Status string            `json:"Status"` // human-readable, e.g. "Up 12 minutes (healthy)"
	Labels map[string]string `json:"Labels"`
}

// Compose labels -- confirmed present on the list response itself against
// the live Docker API (no extra per-container call needed).
const (
	projectLabel  = "com.docker.compose.project"
	configLabel   = "com.docker.compose.project.config_files"
	oneoffLabel   = "com.docker.compose.oneoff" // "True" for `docker compose run` containers
	iconLabel     = "naswarden.icon"            // optional: icon slug override, set on any service
	ignoreLabel   = "naswarden.ignore"          // optional: "true" to exclude from stack health count
	optionalLabel = "naswarden.optional"        // optional: "true" to exclude from stack health count
)

// Container is one running/stopped container with its current resource
// usage. CPUPercent/MemUsed/MemLimit are zero for non-running containers
// (the stats endpoint only returns live data for running ones).
type Container struct {
	Name          string      `json:"name"`
	Image         string      `json:"image"`
	State         string      `json:"state"`
	Status        string      `json:"status"`
	CPUPercent    float64     `json:"cpu_percent"`
	MemUsed       int64       `json:"mem_used"`
	MemLimit      int64       `json:"mem_limit"`
	StartedAt     string      `json:"started_at"`
	RestartPolicy string      `json:"restart_policy"`
	Command       string      `json:"command"`
	Mounts        []Mount     `json:"mounts"`
	Networks      []NetworkIP `json:"networks"`
	Ports         []string    `json:"ports"`
	Stack         string      `json:"stack"`        // compose project, "" for a loose container
	ComposeFile   string      `json:"compose_file"` // compose file(s) that define it
	Icon          string      `json:"icon"`         // optional `naswarden.icon` label
	ExitCode      int         `json:"exit_code"`    // last exit code; meaningful once stopped
	OOMKilled     bool        `json:"oom_killed"`   // whether kernel OOM killer terminated the container
	Optional      bool        `json:"optional"`     // true if labeled naswarden.ignore or naswarden.optional
}

// Mount is one bind mount or named volume attached to a container --
// field names confirmed against a real container's inspect response
// through the live read-only proxy, not assumed. Type is Docker's own
// "bind" / "volume" / "tmpfs" distinction, kept as-is rather than
// collapsed, since a bind mount (host path) and a named volume
// (Docker-managed storage) have different operational implications.
type Mount struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"read_only"`
}

type NetworkIP struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// inspectResponse covers only the fields naswarden's detail drawer needs
// from GET /containers/{id}/json -- confirmed against the real Docker
// Engine API on the live NAS through the actual read-only proxy.
type inspectResponse struct {
	Mounts []struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
	Config struct {
		Cmd        []string `json:"Cmd"`
		Entrypoint []string `json:"Entrypoint"`
	} `json:"Config"`
	HostConfig struct {
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
		PortBindings map[string][]struct {
			HostPort string `json:"HostPort"`
		} `json:"PortBindings"`
	} `json:"HostConfig"`
	State struct {
		StartedAt string `json:"StartedAt"`
		ExitCode  int    `json:"ExitCode"`
		OOMKilled bool   `json:"OOMKilled"`
	} `json:"State"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

// statsResponse covers only the fields needed for the standard Docker CPU%
// formula and memory usage -- confirmed against the real Docker Engine API
// on the live NAS, not assumed from memory.
type statsResponse struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage int64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage int64 `json:"system_cpu_usage"`
		OnlineCPUs     int64 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage int64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage int64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage int64 `json:"usage"`
		Limit int64 `json:"limit"`
	} `json:"memory_stats"`
}

// ListContainers returns the containers naswarden shows, with live stats for
// the ones that are up.
//
// The list includes STOPPED containers that belong to a compose project
// (`all=true`): a stack can only report "2/3 running" if it can see the
// member that is down. Stopped containers with no compose project, and
// stopped one-off `docker compose run` containers, are dropped here -- no
// view shows them (retired runner build containers etc. are just noise).
//
// Stats are fetched CONCURRENTLY, not in a loop -- confirmed directly
// against the real Docker Engine API that a single non-streaming stats
// call takes ~1 second (Docker samples cgroup counters twice internally
// to compute the CPU delta). A typical host easily has 50+
// containers; fetching sequentially took 50+ seconds against a single
// refresh cycle's budget, silently timing out partway through and
// leaving most containers zeroed with the error swallowed. Caught this
// by comparing what the live deployment actually returned against what
// local testing (a handful of containers) had shown.
func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	var listed []containerSummary
	if err := c.get(ctx, "/containers/json?all=true", &listed); err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	kept := make([]containerSummary, 0, len(listed))
	for _, s := range listed {
		if IsStopped(s.State) && (s.Labels[projectLabel] == "" || s.Labels[oneoffLabel] == "True") {
			continue
		}
		kept = append(kept, s)
	}

	containers := make([]Container, len(kept))
	var wg sync.WaitGroup
	var failures atomic.Int64

	for i, s := range kept {
		name := s.ID
		if len(s.Names) > 0 {
			name = trimLeadingSlash(s.Names[0])
		}

		containers[i] = Container{
			Name:        name,
			Image:       s.Image,
			State:       s.State,
			Status:      s.Status,
			Stack:       s.Labels[projectLabel],
			ComposeFile: s.Labels[configLabel],
			Icon:        s.Labels[iconLabel],
			Optional:    s.Labels[ignoreLabel] == "true" || s.Labels[optionalLabel] == "true",
			// Explicitly non-nil -- a Go nil slice marshals to JSON `null`,
			// not `[]`, and the frontend calls .length/.map on these
			// unconditionally (every container has these fields, empty or
			// not). Confirmed this broke the detail drawer for real: a
			// container with zero published ports
			// serialized `"ports": null`, and Vue threw
			// "Cannot read properties of null (reading 'length')",
			// silently leaving the entire drawer body blank.
			Mounts:   []Mount{},
			Networks: []NetworkIP{},
			Ports:    []string{},
		}

		wg.Add(1)
		go func(i int, id string, live bool) {
			defer wg.Done()
			// Stats only exist for a container that is up. A failure for one
			// (it stopped between the list call and now, or a genuine
			// timeout) shouldn't fail the whole refresh -- it just reports
			// zeroed usage for that one. Counted, not silently dropped, so a
			// systemic problem (like the sequential-fetch bug this replaced)
			// is visible in logs instead of just showing up as
			// mysteriously-zeroed stats in the UI.
			if live {
				var stats statsResponse
				if err := c.get(ctx, fmt.Sprintf("/containers/%s/stats?stream=false", id), &stats); err != nil {
					failures.Add(1)
				} else {
					containers[i].CPUPercent = cpuPercent(stats)
					containers[i].MemUsed = stats.MemoryStats.Usage
					containers[i].MemLimit = stats.MemoryStats.Limit
				}
			}

			// Inspect is independent of stats (a stopped container has no
			// stats but still has mounts, an exit code, ...) and best-effort
			// too: a failure just leaves the detail drawer sparse for this
			// one container.
			var insp inspectResponse
			if err := c.get(ctx, fmt.Sprintf("/containers/%s/json", id), &insp); err != nil {
				return
			}
			applyInspect(&containers[i], insp)
		}(i, s.ID, !IsStopped(s.State))
	}

	wg.Wait()
	if n := failures.Load(); n > 0 {
		slog.Warn("failed to fetch stats for some containers", "count", n, "total", len(kept))
	}
	return containers, nil
}

// IsStopped reports whether a Docker container state means "not running and
// not coming back on its own" (as opposed to running/restarting/paused).
func IsStopped(state string) bool {
	return state == "exited" || state == "created" || state == "dead"
}

// applyInspect fills in the detail-drawer fields from a container's
// inspect response.
func applyInspect(c *Container, insp inspectResponse) {
	c.StartedAt = insp.State.StartedAt
	c.ExitCode = insp.State.ExitCode
	c.OOMKilled = insp.State.OOMKilled
	c.RestartPolicy = insp.HostConfig.RestartPolicy.Name

	cmd := insp.Config.Entrypoint
	cmd = append(cmd, insp.Config.Cmd...)
	c.Command = strings.Join(cmd, " ")

	for _, m := range insp.Mounts {
		c.Mounts = append(c.Mounts, Mount{
			Type: m.Type, Source: m.Source, Destination: m.Destination, ReadOnly: !m.RW,
		})
	}

	for name, net := range insp.NetworkSettings.Networks {
		c.Networks = append(c.Networks, NetworkIP{Name: name, IP: net.IPAddress})
	}
	// Map iteration order is random -- sort so the UI doesn't jitter
	// between refreshes for a container on more than one network.
	sort.Slice(c.Networks, func(i, j int) bool { return c.Networks[i].Name < c.Networks[j].Name })

	// Only published ports (there's a host-visible mapping) are useful on
	// an overview page -- an internal-only exposed port has nothing to
	// show. Sorted for the same reason as Networks above.
	for containerPort, bindings := range insp.HostConfig.PortBindings {
		for _, b := range bindings {
			if b.HostPort == "" {
				continue
			}
			c.Ports = append(c.Ports, fmt.Sprintf("%s → %s", containerPort, b.HostPort))
		}
	}
	sort.Strings(c.Ports)
}

// cpuPercent implements Docker's own documented formula: CPU delta over
// system delta, scaled by the number of online CPUs.
func cpuPercent(s statsResponse) float64 {
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemCPUUsage - s.PreCPUStats.SystemCPUUsage)
	if systemDelta <= 0 || cpuDelta <= 0 {
		return 0
	}
	return (cpuDelta / systemDelta) * float64(s.CPUStats.OnlineCPUs) * 100.0
}

func trimLeadingSlash(s string) string {
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
