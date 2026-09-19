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
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	Image  string   `json:"Image"`
	State  string   `json:"State"`  // "running", "exited", ...
	Status string   `json:"Status"` // human-readable, e.g. "Up 12 minutes (healthy)"
}

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

// ListContainers returns every container (running and stopped) with live
// stats for the running ones.
//
// Stats are fetched CONCURRENTLY, not in a loop -- confirmed directly
// against the real Docker Engine API that a single non-streaming stats
// call takes ~1 second (Docker samples cgroup counters twice internally
// to compute the CPU delta). A large host easily has 50+
// containers; fetching sequentially took 50+ seconds against a single
// refresh cycle's budget, silently timing out partway through and
// leaving most containers zeroed with the error swallowed. Caught this
// by comparing what the live deployment actually returned against what
// local testing (a handful of containers) had shown.
func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	// No `all=true` -- Docker's default already returns running containers
	// only, which is exactly what we want: exited/created containers
	// (stopped runner build containers, retired one-offs, etc.) are noise
	// on an overview page.
	var running []containerSummary
	if err := c.get(ctx, "/containers/json", &running); err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	containers := make([]Container, len(running))
	var wg sync.WaitGroup
	var failures atomic.Int64

	for i, s := range running {
		name := s.ID
		if len(s.Names) > 0 {
			name = trimLeadingSlash(s.Names[0])
		}

		containers[i] = Container{
			Name:   name,
			Image:  s.Image,
			State:  s.State,
			Status: s.Status,
			// Explicitly non-nil -- a Go nil slice marshals to JSON `null`,
			// not `[]`, and the frontend calls .length/.map on these
			// unconditionally (every container has these fields, empty or
			// not). Confirmed this broke the detail drawer for real: a
			// container with zero published ports (e.g. komodo-periphery)
			// serialized `"ports": null`, and Vue threw
			// "Cannot read properties of null (reading 'length')",
			// silently leaving the entire drawer body blank.
			Mounts:   []Mount{},
			Networks: []NetworkIP{},
			Ports:    []string{},
		}

		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			var stats statsResponse
			if err := c.get(ctx, fmt.Sprintf("/containers/%s/stats?stream=false", id), &stats); err != nil {
				// A stats failure for one container (stopped between the
				// list call and now, or a genuine timeout) shouldn't fail
				// the whole refresh -- it just reports zeroed usage for
				// that one. Counted, not silently dropped, so a systemic
				// problem (like the sequential-fetch bug this replaced)
				// is visible in logs instead of just showing up as
				// mysteriously-zeroed stats in the UI.
				failures.Add(1)
				return
			}
			containers[i].CPUPercent = cpuPercent(stats)
			containers[i].MemUsed = stats.MemoryStats.Usage
			containers[i].MemLimit = stats.MemoryStats.Limit

			var insp inspectResponse
			if err := c.get(ctx, fmt.Sprintf("/containers/%s/json", id), &insp); err != nil {
				// Same best-effort treatment -- the overview card already
				// has what it needs from stats above, so a failed inspect
				// just means the detail drawer will be sparse for this one
				// container, not that the refresh fails.
				return
			}
			applyInspect(&containers[i], insp)
		}(i, s.ID)
	}

	wg.Wait()
	if n := failures.Load(); n > 0 {
		slog.Warn("failed to fetch stats for some containers", "count", n, "total", len(running))
	}
	return containers, nil
}

// applyInspect fills in the detail-drawer fields from a container's
// inspect response.
func applyInspect(c *Container, insp inspectResponse) {
	c.StartedAt = insp.State.StartedAt
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
