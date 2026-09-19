package truenas

import (
	"context"
	"encoding/json"
	"fmt"
)

// Pool is the subset of pool.query's response naswarden cares about.
// Field names/shape confirmed directly against a live TrueNAS SCALE
// 25.10.7 box via `midclt call pool.query` -- not guessed from docs.
type Pool struct {
	Name          string `json:"name"`
	Status        string `json:"status"` // ONLINE / DEGRADED / FAULTED / ...
	Healthy       bool   `json:"healthy"`
	Warning       bool   `json:"warning"`
	Size          int64  `json:"size"`
	Allocated     int64  `json:"allocated"`
	Free          int64  `json:"free"`
	Scan          *Scan  `json:"scan"`
	Fragmentation string `json:"fragmentation"` // e.g. "31" (percent, as a string on the wire)
	Vdevs         []Vdev `json:"vdevs"`
}

type Scan struct {
	Function string `json:"function"` // SCRUB / RESILVER / ...
	State    string `json:"state"`    // FINISHED / IN_PROGRESS / ...
	Errors   int64  `json:"errors"`
}

// Vdev is one top-level vdev group (mirror-0, raidz1-0, a bare disk, ...)
// and its member disks -- this is pool.query's `topology.data` entries,
// not the cache/log/spare groups (not useful for an at-a-glance layout
// view, and most homelab pools don't have them).
type Vdev struct {
	Name     string      `json:"name"` // e.g. "mirror-0", "raidz1-0"
	Type     string      `json:"type"` // MIRROR / RAIDZ1 / DISK / ...
	Status   string      `json:"status"`
	Children []VdevChild `json:"children"`
}

type VdevChild struct {
	Disk           string `json:"disk"`
	Status         string `json:"status"`
	ReadErrors     int64  `json:"read_errors"`
	WriteErrors    int64  `json:"write_errors"`
	ChecksumErrors int64  `json:"checksum_errors"`
}

// rawPool mirrors pool.query's actual response shape -- topology.data[]
// entries nest their disk-error counts under `stats`, and a bare-disk
// vdev's own name IS the disk device name (no `children`), confirmed
// directly against a live TrueNAS box rather than assumed.
type rawPool struct {
	Pool
	Topology struct {
		Data []struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Status   string `json:"status"`
			Disk     string `json:"disk"` // set instead of Children for a bare-disk vdev
			Children []struct {
				Disk   string `json:"disk"`
				Status string `json:"status"`
				Stats  struct {
					ReadErrors     int64 `json:"read_errors"`
					WriteErrors    int64 `json:"write_errors"`
					ChecksumErrors int64 `json:"checksum_errors"`
				} `json:"stats"`
			} `json:"children"`
			Stats struct {
				ReadErrors     int64 `json:"read_errors"`
				WriteErrors    int64 `json:"write_errors"`
				ChecksumErrors int64 `json:"checksum_errors"`
			} `json:"stats"`
		} `json:"data"`
	} `json:"topology"`
}

// ListPools calls pool.query with no filters and returns every pool.
func ListPools(ctx context.Context, c *Client) ([]Pool, error) {
	raw, err := c.Call(ctx, "pool.query", []any{})
	if err != nil {
		return nil, fmt.Errorf("pool.query: %w", err)
	}
	var rawPools []rawPool
	if err := json.Unmarshal(raw, &rawPools); err != nil {
		return nil, fmt.Errorf("pool.query: decode: %w", err)
	}

	pools := make([]Pool, len(rawPools))
	for i, rp := range rawPools {
		pool := rp.Pool
		// Explicitly non-nil -- a Go nil slice marshals to JSON `null`, and
		// the frontend calls .length/v-for on this unconditionally (same
		// class of bug confirmed for docker.Container's array fields).
		pool.Vdevs = []Vdev{}
		for _, vd := range rp.Topology.Data {
			vdev := Vdev{Name: vd.Name, Type: vd.Type, Status: vd.Status, Children: []VdevChild{}}
			if len(vd.Children) > 0 {
				for _, ch := range vd.Children {
					vdev.Children = append(vdev.Children, VdevChild{
						Disk: ch.Disk, Status: ch.Status,
						ReadErrors: ch.Stats.ReadErrors, WriteErrors: ch.Stats.WriteErrors,
						ChecksumErrors: ch.Stats.ChecksumErrors,
					})
				}
			} else if vd.Disk != "" {
				// A bare-disk vdev (e.g. boot-pool) has no children -- it
				// IS the disk.
				vdev.Children = []VdevChild{{
					Disk: vd.Disk, Status: vd.Status,
					ReadErrors: vd.Stats.ReadErrors, WriteErrors: vd.Stats.WriteErrors,
					ChecksumErrors: vd.Stats.ChecksumErrors,
				}}
			}
			pool.Vdevs = append(pool.Vdevs, vdev)
		}
		pools[i] = pool
	}
	return pools, nil
}
