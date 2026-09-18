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
	"net/http"
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
	Name       string  `json:"name"`
	Image      string  `json:"image"`
	State      string  `json:"state"`
	Status     string  `json:"status"`
	CPUPercent float64 `json:"cpu_percent"`
	MemUsed    int64   `json:"mem_used"`
	MemLimit   int64   `json:"mem_limit"`
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
func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	var summaries []containerSummary
	if err := c.get(ctx, "/containers/json?all=true", &summaries); err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	containers := make([]Container, 0, len(summaries))
	for _, s := range summaries {
		name := s.ID
		if len(s.Names) > 0 {
			name = trimLeadingSlash(s.Names[0])
		}

		cont := Container{
			Name:   name,
			Image:  s.Image,
			State:  s.State,
			Status: s.Status,
		}

		if s.State == "running" {
			var stats statsResponse
			if err := c.get(ctx, fmt.Sprintf("/containers/%s/stats?stream=false", s.ID), &stats); err == nil {
				cont.CPUPercent = cpuPercent(stats)
				cont.MemUsed = stats.MemoryStats.Usage
				cont.MemLimit = stats.MemoryStats.Limit
			}
			// A stats failure for one container (e.g. it stopped between
			// the list call and now) shouldn't fail the whole refresh --
			// it just reports zeroed usage for that one.
		}

		containers = append(containers, cont)
	}
	return containers, nil
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
