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
	// Load average normalized to % of total CPU capacity (load / cores *
	// 100), not the raw Unix load average -- a raw number like "3.42" is
	// meaningless without already knowing this box's core count, and is
	// largely redundant with CPUPercent above anyway. As a percentage it's
	// self-explanatory and can still exceed 100% (genuine queueing/
	// overload), which is real, useful information worth keeping visible
	// rather than clamping away.
	LoadPercent1  float64 `json:"load_percent_1"`
	LoadPercent5  float64 `json:"load_percent_5"`
	LoadPercent15 float64 `json:"load_percent_15"`
	// ArcBytes is the ZFS ARC (Adaptive Replacement Cache) size -- often
	// the dominant chunk of "used" memory on a NAS, and the usual reason
	// memory usage looks alarmingly high when it's actually just
	// reclaimable disk cache, not an app leak. Shown separately so that
	// distinction is visible instead of buried inside a single number.
	ArcBytes   int64       `json:"arc_bytes"`
	Interfaces []Interface `json:"interfaces"`
}

// Interface is a network interface's current link state and addresses --
// confirmed against a live TrueNAS box via `midclt call interface.query`
// (method name has no "network." prefix on SCALE 25.10.7, despite older
// docs suggesting otherwise).
type Interface struct {
	Name      string   `json:"name"`
	LinkState string   `json:"link_state"` // "LINK_STATE_UP" / "LINK_STATE_DOWN"
	Speed     string   `json:"speed"`      // e.g. "1000Mb/s Twisted Pair"
	Addresses []string `json:"addresses"`  // e.g. "192.0.2.10/24"
}

type systemInfoResponse struct {
	Hostname   string  `json:"hostname"`
	Version    string  `json:"version"`
	Physmem    int64   `json:"physmem"`
	UptimeSecs float64 `json:"uptime_seconds"`
	Cores      int64   `json:"cores"`
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

// rawInterface mirrors interface.query's response shape -- confirmed
// against a live box. Only INET/INET6 aliases under `state` are real
// addresses; the top-level `aliases` field is the *configured* (not
// necessarily active) address list, which is why state.aliases is used
// instead.
type rawInterface struct {
	Name  string `json:"name"`
	State struct {
		LinkState          string `json:"link_state"`
		ActiveMediaSubtype string `json:"active_media_subtype"`
		Aliases            []struct {
			Type    string `json:"type"`
			Address string `json:"address"`
			Netmask int    `json:"netmask"`
		} `json:"aliases"`
	} `json:"state"`
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
		map[string]string{"name": "arcsize"},
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
			if sysInfo.Cores <= 0 {
				continue // avoid a divide-by-zero if this ever comes back 0
			}
			if v, ok := column(g.Legend, last, "shortterm"); ok {
				info.LoadPercent1 = v / float64(sysInfo.Cores) * 100
			}
			if v, ok := column(g.Legend, last, "midterm"); ok {
				info.LoadPercent5 = v / float64(sysInfo.Cores) * 100
			}
			if v, ok := column(g.Legend, last, "longterm"); ok {
				info.LoadPercent15 = v / float64(sysInfo.Cores) * 100
			}
		case "arcsize":
			if v, ok := column(g.Legend, last, "size"); ok {
				info.ArcBytes = int64(v)
			}
		}
	}

	raw, err = c.Call(ctx, "interface.query", []any{})
	if err != nil {
		return nil, fmt.Errorf("interface.query: %w", err)
	}
	var rawIfaces []rawInterface
	if err := json.Unmarshal(raw, &rawIfaces); err != nil {
		return nil, fmt.Errorf("interface.query: decode: %w", err)
	}
	for _, ri := range rawIfaces {
		if ri.State.LinkState != "LINK_STATE_UP" {
			continue // down/unused interfaces are noise on an overview page
		}
		iface := Interface{Name: ri.Name, LinkState: ri.State.LinkState, Speed: ri.State.ActiveMediaSubtype}
		for _, a := range ri.State.Aliases {
			iface.Addresses = append(iface.Addresses, fmt.Sprintf("%s/%d", a.Address, a.Netmask))
		}
		info.Interfaces = append(info.Interfaces, iface)
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
