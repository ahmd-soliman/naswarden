package truenas

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Disk is one physical drive: what it is, where it belongs, how warm it runs
// and how busy it is. SMART health is deliberately absent -- TrueNAS 25.10
// removed SMART from its middleware, so there is nothing to read.
type Disk struct {
	Name         string   `json:"name"` // device name, e.g. "sda"
	Model        string   `json:"model"`
	Serial       string   `json:"serial"`
	Size         int64    `json:"size"`
	Type         string   `json:"type"` // "HDD" | "SSD"
	RotationRate int      `json:"rotation_rate,omitempty"`
	Bus          string   `json:"bus,omitempty"`
	Pool         string   `json:"pool"`           // "" if the disk is in no pool
	Vdev         string   `json:"vdev,omitempty"` // e.g. "mirror-0"
	Status       string   `json:"status"`         // pool member state (ONLINE, DEGRADED, ...), "" for non-members
	TempC        *float64 `json:"temp_c,omitempty"`
	Standby      bool     `json:"standby"`
	Temp7dMin    *float64 `json:"temp_7d_min,omitempty"`
	Temp7dAvg    *float64 `json:"temp_7d_avg,omitempty"`
	Temp7dMax    *float64 `json:"temp_7d_max,omitempty"`
	// Cumulative ZFS counters since the pool was imported, and the rate
	// between the last two refreshes. Only pool members have them.
	ReadBytes      *int64   `json:"read_bytes,omitempty"`
	WriteBytes     *int64   `json:"write_bytes,omitempty"`
	ReadRate       *float64 `json:"read_bytes_per_sec,omitempty"`
	WriteRate      *float64 `json:"write_bytes_per_sec,omitempty"`
	ReadErrors     int64    `json:"read_errors"`
	WriteErrors    int64    `json:"write_errors"`
	ChecksumErrors int64    `json:"checksum_errors"`
}

type rawDiskDetail struct {
	Name          string  `json:"name"`
	Model         string  `json:"model"`
	Serial        string  `json:"serial"`
	Size          int64   `json:"size"`
	Type          string  `json:"type"`
	RotationRate  *int    `json:"rotationrate"`
	Bus           string  `json:"bus"`
	ImportedZpool *string `json:"imported_zpool"`
}

type diskDetailsResponse struct {
	Used   []rawDiskDetail `json:"used"`
	Unused []rawDiskDetail `json:"unused"`
}

type tempAgg struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
}

// poolDisk is what pool.query knows about one member disk.
type poolDisk struct {
	vdev, status   string
	read, write    int64
	hasStats       bool
	rerr, werr, ce int64
}

type rawPoolStats struct {
	Name     string `json:"name"`
	Topology struct {
		Data []rawTopoNode `json:"data"`
	} `json:"topology"`
}

type rawTopoNode struct {
	Name     string        `json:"name"`
	Disk     string        `json:"disk"`
	Status   string        `json:"status"`
	Stats    rawNodeStats  `json:"stats"`
	Children []rawTopoNode `json:"children"`
}

type rawNodeStats struct {
	ReadErrors     int64   `json:"read_errors"`
	WriteErrors    int64   `json:"write_errors"`
	ChecksumErrors int64   `json:"checksum_errors"`
	Bytes          []int64 `json:"bytes"`
}

// ZFS vdev_stat indexes into stats.bytes (zio_type): 0 null, 1 read, 2 write.
const (
	zioRead  = 1
	zioWrite = 2
)

// DiskRates turns cumulative per-disk byte counters into a rate between two
// refreshes. It only ever sees the refresh goroutine.
type DiskRates struct {
	mu   sync.Mutex
	prev map[string]rateSample
}

type rateSample struct {
	at          time.Time
	read, write int64
}

func NewDiskRates() *DiskRates { return &DiskRates{prev: map[string]rateSample{}} }

// Rate records the counters and returns bytes/second since the previous call
// for this disk. It returns nil the first time, and after a counter reset
// (pool re-import), rather than a bogus negative or huge number.
func (t *DiskRates) Rate(now time.Time, disk string, read, write int64) (readRate, writeRate *float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, seen := t.prev[disk]
	t.prev[disk] = rateSample{at: now, read: read, write: write}
	if !seen {
		return nil, nil
	}
	dt := now.Sub(p.at).Seconds()
	if dt <= 0 || read < p.read || write < p.write {
		return nil, nil
	}
	r, w := float64(read-p.read)/dt, float64(write-p.write)/dt
	return &r, &w
}

// Forget drops samples for disks that are no longer present.
func (t *DiskRates) Forget(keep map[string]struct{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for name := range t.prev {
		if _, ok := keep[name]; !ok {
			delete(t.prev, name)
		}
	}
}

// ListDisks combines disk.details (identity + pool), disk.temperatures and
// disk.temperature_agg (7-day range), and pool.query's per-disk ZFS counters.
// Only disk.details is required; the rest are best-effort so a hiccup in one
// call does not blank the whole table.
func ListDisks(ctx context.Context, c *Client, rates *DiskRates) ([]Disk, error) {
	raw, err := c.Call(ctx, "disk.details", []any{})
	if err != nil {
		return nil, fmt.Errorf("disk.details: %w", err)
	}
	var details diskDetailsResponse
	if err := json.Unmarshal(raw, &details); err != nil {
		return nil, fmt.Errorf("disk.details: decode: %w", err)
	}

	temps, _ := GetDiskTemperatures(ctx, c)

	all := append(append([]rawDiskDetail{}, details.Used...), details.Unused...)
	names := make([]string, 0, len(all))
	for _, d := range all {
		names = append(names, d.Name)
	}

	var agg map[string]*tempAgg
	if r, err := c.Call(ctx, "disk.temperature_agg", []any{names, 7}); err == nil {
		_ = json.Unmarshal(r, &agg)
	}

	members := map[string]poolDisk{}
	if r, err := c.Call(ctx, "pool.query", []any{}); err == nil {
		var pools []rawPoolStats
		if json.Unmarshal(r, &pools) == nil {
			for _, p := range pools {
				collectMembers(p.Topology.Data, "", members)
			}
		}
	}

	return buildDisks(all, temps, agg, members, rates, time.Now()), nil
}

func collectMembers(nodes []rawTopoNode, vdev string, out map[string]poolDisk) {
	for _, n := range nodes {
		if len(n.Children) > 0 {
			collectMembers(n.Children, n.Name, out)
			continue
		}
		if n.Disk == "" {
			continue
		}
		pd := poolDisk{vdev: vdev, status: n.Status, rerr: n.Stats.ReadErrors, werr: n.Stats.WriteErrors, ce: n.Stats.ChecksumErrors}
		if len(n.Stats.Bytes) > zioWrite {
			pd.read, pd.write, pd.hasStats = n.Stats.Bytes[zioRead], n.Stats.Bytes[zioWrite], true
		}
		out[n.Disk] = pd
	}
}

// buildDisks is the pure assembly step, separated so it can be tested
// without a TrueNAS connection.
func buildDisks(all []rawDiskDetail, temps map[string]*float64, agg map[string]*tempAgg, members map[string]poolDisk, rates *DiskRates, now time.Time) []Disk {
	disks := make([]Disk, 0, len(all))
	present := make(map[string]struct{}, len(all))
	for _, d := range all {
		present[d.Name] = struct{}{}
		disk := Disk{Name: d.Name, Model: d.Model, Serial: d.Serial, Size: d.Size, Type: d.Type, Bus: d.Bus}
		if d.RotationRate != nil {
			disk.RotationRate = *d.RotationRate
		}
		// TrueNAS reports some spinning disks (e.g. SMR WD Blue) as SSD with no
		// rotation rate; nothing better is available, so show what it says.
		if d.ImportedZpool != nil {
			disk.Pool = *d.ImportedZpool
		}
		if t, ok := temps[d.Name]; ok {
			if t != nil {
				disk.TempC = t
			} else {
				disk.Standby = true
			}
		}
		// A drive that has been asleep all week comes back as 0/0/0, which
		// is "no reading", not a 0 degree drive.
		if a := agg[d.Name]; a != nil && (a.Min != 0 || a.Max != 0 || a.Avg != 0) {
			mn, av, mx := a.Min, a.Avg, a.Max
			disk.Temp7dMin, disk.Temp7dAvg, disk.Temp7dMax = &mn, &av, &mx
		}
		if m, ok := members[d.Name]; ok {
			disk.Vdev, disk.Status = m.vdev, m.status
			disk.ReadErrors, disk.WriteErrors, disk.ChecksumErrors = m.rerr, m.werr, m.ce
			if m.hasStats {
				r, w := m.read, m.write
				disk.ReadBytes, disk.WriteBytes = &r, &w
				disk.ReadRate, disk.WriteRate = rates.Rate(now, d.Name, m.read, m.write)
			}
		}
		disks = append(disks, disk)
	}
	rates.Forget(present)
	sort.Slice(disks, func(i, j int) bool {
		pi, pj := strings.ToLower(disks[i].Pool), strings.ToLower(disks[j].Pool)
		if pi != pj {
			return pi < pj
		}
		return disks[i].Name < disks[j].Name
	})
	return disks
}
