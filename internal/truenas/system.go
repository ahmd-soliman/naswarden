package truenas

import (
	"context"
	"encoding/json"
	"fmt"
)

// ServerInfo is a snapshot of the TrueNAS host's own resource usage --
// confirmed against a live TrueNAS SCALE 25.10.7 box, not assumed:
// `system.info` gives static facts (hostname, version, total memory,
// uptime); `reporting.netdata_get_data` (TrueNAS's reporting subsystem is
// Netdata-backed in this version) gives live CPU/memory/load, the same
// data source that feeds TrueNAS's own UI Dashboard.
type ServerInfo struct {
	Hostname   string  `json:"hostname"`
	Version    string  `json:"version"`
	UptimeSecs float64 `json:"uptime_seconds"`
	CPUPercent float64 `json:"cpu_percent"`
	MemUsed    int64   `json:"mem_used"`
	MemTotal   int64   `json:"mem_total"`
	LoadAvg1   float64 `json:"load_avg_1"`
	LoadAvg5   float64 `json:"load_avg_5"`
	LoadAvg15  float64 `json:"load_avg_15"`
}

type systemInfoResponse struct {
	Hostname   string  `json:"hostname"`
	Version    string  `json:"version"`
	Physmem    int64   `json:"physmem"`
	UptimeSecs float64 `json:"uptime_seconds"`
}

// reportingGraph mirrors reporting.netdata_get_data's response shape:
// `legend` names each column in `data` (index 0 is always "time"), so the
// column to read is found by name, not a hardcoded position -- the exact
// column order isn't documented anywhere, only discoverable by calling
// the API, so hardcoding an index would be fragile if it ever changes.
type reportingGraph struct {
	Name   string          `json:"name"`
	Legend []string        `json:"legend"`
	Data   [][]json.Number `json:"data"`
}

// GetServerInfo fetches a fresh snapshot of host resource usage.
func GetServerInfo(ctx context.Context, c *Client) (*ServerInfo, error) {
	raw, err := c.Call(ctx, "system.info", []any{})
	if err != nil {
		return nil, fmt.Errorf("system.info: %w", err)
	}
	var sysInfo systemInfoResponse
	if err := json.Unmarshal(raw, &sysInfo); err != nil {
		return nil, fmt.Errorf("system.info: decode: %w", err)
	}

	graphs := []any{
		map[string]string{"name": "cpu"},
		map[string]string{"name": "memory"},
		map[string]string{"name": "load"},
	}
	opts := map[string]any{"unit": "HOUR", "page": 1}
	raw, err = c.Call(ctx, "reporting.netdata_get_data", []any{graphs, opts})
	if err != nil {
		return nil, fmt.Errorf("reporting.netdata_get_data: %w", err)
	}
	var reportGraphs []reportingGraph
	if err := json.Unmarshal(raw, &reportGraphs); err != nil {
		return nil, fmt.Errorf("reporting.netdata_get_data: decode: %w", err)
	}

	info := &ServerInfo{
		Hostname:   sysInfo.Hostname,
		Version:    sysInfo.Version,
		UptimeSecs: sysInfo.UptimeSecs,
		MemTotal:   sysInfo.Physmem,
	}

	for _, g := range reportGraphs {
		last := latestRow(g)
		if last == nil {
			continue
		}
		switch g.Name {
		case "cpu":
			if v, ok := column(g.Legend, last, "cpu"); ok {
				info.CPUPercent = v
			}
		case "memory":
			if v, ok := column(g.Legend, last, "available"); ok {
				info.MemUsed = info.MemTotal - int64(v)
			}
		case "load":
			if v, ok := column(g.Legend, last, "shortterm"); ok {
				info.LoadAvg1 = v
			}
			if v, ok := column(g.Legend, last, "midterm"); ok {
				info.LoadAvg5 = v
			}
			if v, ok := column(g.Legend, last, "longterm"); ok {
				info.LoadAvg15 = v
			}
		}
	}

	return info, nil
}

// latestRow returns the most recent data point, or nil if there isn't one
// (e.g. a graph with no data yet).
func latestRow(g reportingGraph) []json.Number {
	if len(g.Data) == 0 {
		return nil
	}
	return g.Data[len(g.Data)-1]
}

// column finds the value in row corresponding to legend entry name,
// looking the position up by name rather than assuming a fixed index.
func column(legend []string, row []json.Number, name string) (float64, bool) {
	for i, label := range legend {
		if label == name && i < len(row) {
			f, err := row[i].Float64()
			return f, err == nil
		}
	}
	return 0, false
}
