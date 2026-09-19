package truenas

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	// Total live throughput over PHYSICAL interfaces (a bridge would double
	// count its member's traffic), kilobits/s.
	NetRxKbps *float64 `json:"net_rx_kbps,omitempty"`
	NetTxKbps *float64 `json:"net_tx_kbps,omitempty"`
	// CPUModel/Cores/PhysicalCores come straight from system.info -- static
	// hardware facts, not something that needs the reporting subsystem.
	CPUModel      string  `json:"cpu_model"`
	Cores         int64   `json:"cores"`
	PhysicalCores int64   `json:"physical_cores"`
	CPUTempC      float64 `json:"cpu_temp_c"`
}

// Interface is a network interface's current link state and addresses --
// confirmed against a live TrueNAS box via `midclt call interface.query`
// (method name has no "network." prefix on SCALE 25.10.7, despite older
// docs suggesting otherwise).
type Interface struct {
	Name      string `json:"name"`
	Type      string `json:"type"`       // "PHYSICAL" | "BRIDGE" | "VLAN" | ...
	SpeedMbps int    `json:"speed_mbps"` // parsed from Speed; 0 if unknown
	// Live rate in kilobits/s (netdata's own unit), averaged over the last
	// minute; nil when the graph has no data.
	RxKbps    *float64 `json:"rx_kbps,omitempty"`
	TxKbps    *float64 `json:"tx_kbps,omitempty"`
	LinkState string   `json:"link_state"` // "LINK_STATE_UP" / "LINK_STATE_DOWN"
	Speed     string   `json:"speed"`      // e.g. "1000Mb/s Twisted Pair"
	Addresses []string `json:"addresses"`  // e.g. "192.168.8.100/24"
}

type systemInfoResponse struct {
	Hostname      string  `json:"hostname"`
	Version       string  `json:"version"`
	Physmem       int64   `json:"physmem"`
	UptimeSecs    float64 `json:"uptime_seconds"`
	Cores         int64   `json:"cores"` // logical cores (threads)
	PhysicalCores int64   `json:"physical_cores"`
	Model         string  `json:"model"` // e.g. "AMD Ryzen 5 5600G with Radeon Graphics"
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
	Type  string `json:"type"`
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
		map[string]string{"name": "cputemp"},
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
		Hostname:      sysInfo.Hostname,
		Version:       sysInfo.Version,
		UptimeSecs:    sysInfo.UptimeSecs,
		MemTotal:      sysInfo.Physmem,
		CPUModel:      sysInfo.Model,
		Cores:         sysInfo.Cores,
		PhysicalCores: sysInfo.PhysicalCores,
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
		case "cputemp":
			// "cpu" is the aggregate/package column; the rest (cpu0, cpu1,
			// ...) are per-core, more detail than an overview card needs.
			if v, ok := column(g.Legend, last, "cpu"); ok {
				info.CPUTempC = v
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
	// Explicitly non-nil -- a Go nil slice marshals to JSON `null`, and the
	// frontend calls .length/v-for on this unconditionally (same class of
	// bug confirmed for docker.Container's array fields, which broke the
	// container detail drawer for real on the live deployment).
	info.Interfaces = []Interface{}
	for _, ri := range rawIfaces {
		if ri.State.LinkState != "LINK_STATE_UP" {
			continue // down/unused interfaces are noise on an overview page
		}
		iface := Interface{Name: ri.Name, Type: ri.Type, LinkState: ri.State.LinkState, Speed: ri.State.ActiveMediaSubtype, SpeedMbps: parseSpeedMbps(ri.State.ActiveMediaSubtype), Addresses: []string{}}
		for _, a := range ri.State.Aliases {
			iface.Addresses = append(iface.Addresses, fmt.Sprintf("%s/%d", a.Address, a.Netmask))
		}
		info.Interfaces = append(info.Interfaces, iface)
	}

	// Best-effort: a failure here just leaves the rates blank.
	addInterfaceRates(ctx, c, info)

	return info, nil
}

// addInterfaceRates fills in per-interface and total throughput from
// netdata's "interface" graph (the same source as TrueNAS's own dashboard).
func addInterfaceRates(ctx context.Context, c *Client, info *ServerInfo) {
	if len(info.Interfaces) == 0 {
		return
	}
	graphs := make([]any, 0, len(info.Interfaces))
	for _, i := range info.Interfaces {
		graphs = append(graphs, map[string]string{"name": "interface", "identifier": i.Name})
	}
	raw, err := c.Call(ctx, "reporting.netdata_get_data", []any{graphs, map[string]any{"unit": "HOUR", "page": 1}})
	if err != nil {
		return
	}
	var out []reportingGraph
	if json.Unmarshal(raw, &out) != nil || len(out) != len(info.Interfaces) {
		return
	}
	var rx, tx float64
	var any bool
	for i := range info.Interfaces {
		r, t := meanTail(out[i], "received", 60), meanTail(out[i], "sent", 60)
		info.Interfaces[i].RxKbps, info.Interfaces[i].TxKbps = r, t
		if info.Interfaces[i].Type == "PHYSICAL" && r != nil && t != nil {
			rx, tx, any = rx+*r, tx+*t, true
		}
	}
	if any {
		info.NetRxKbps, info.NetTxKbps = &rx, &tx
	}
}

// parseSpeedMbps extracts the link speed from strings like
// "1000Mb/s Twisted Pair"; 0 if there is none.
func parseSpeedMbps(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 || !strings.HasPrefix(strings.TrimLeft(s, "0123456789"), "Mb/s") {
		return 0
	}
	return n
}

// meanTail averages the last n points of a legend column (netdata returns one
// point per second, so 60 is "the last minute", smoothing single-second
// spikes). nil if the column is missing or has no points.
func meanTail(g reportingGraph, name string, n int) *float64 {
	if len(g.Data) == 0 {
		return nil
	}
	rows := g.Data
	if len(rows) > n {
		rows = rows[len(rows)-n:]
	}
	var sum float64
	var count int
	for _, row := range rows {
		if v, ok := column(g.Legend, row, name); ok {
			sum += v
			count++
		}
	}
	if count == 0 {
		return nil
	}
	m := sum / float64(count)
	return &m
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
