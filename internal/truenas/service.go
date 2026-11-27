package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// Service is a TrueNAS service (SMB, NFS, SSH, ...) from `service.query`.
// ListServices returns all of them; VisibleServices drops the ones that are
// off and not worth showing.
type Service struct {
	Name   string `json:"name"`   // middleware name: cifs, nfs, ssh, iscsitarget, ups, ...
	State  string `json:"state"`  // RUNNING, STOPPED, ...
	Enable bool   `json:"enable"` // start on boot
	// Problem: not running although it should be: set to start on boot, or (for
	// SMB and NFS) it has enabled shares. Computed by VisibleServices so the UI
	// and any other consumer agree on what counts as a problem.
	Problem bool `json:"problem"`
}

// Share is an SMB or NFS share. Name and Path say a lot about the system, so
// they are only ever sent in the state message, never in metrics or logs.
type Share struct {
	Service  string `json:"service"` // "cifs" or "nfs": the Service.Name it belongs to
	Name     string `json:"name"`    // SMB share name; NFS comment (NFS shares have no name)
	Path     string `json:"path"`
	Enabled  bool   `json:"enabled"`
	ReadOnly bool   `json:"read_only"`
	// Locked: the share's dataset is locked (encrypted, not unlocked), so it
	// cannot be served. TrueNAS reports it on both share types.
	Locked bool `json:"locked"`
}

type rawService struct {
	Service string `json:"service"`
	State   string `json:"state"`
	Enable  bool   `json:"enable"`
}

// Fields as TrueNAS 25.10.7 returns them. SMB calls read-only `readonly`, NFS
// calls it `ro`. NFS shares carry `path` there and `paths` (a list) on older
// releases. Everything is optional: a missing field reads as its zero value.
type rawShare struct {
	Name     string   `json:"name"`
	Comment  string   `json:"comment"`
	Path     string   `json:"path"`
	Paths    []string `json:"paths"`
	Enabled  *bool    `json:"enabled"`
	RO       bool     `json:"ro"`       // NFS
	Readonly bool     `json:"readonly"` // SMB
	Locked   bool     `json:"locked"`
}

// ListServices queries service.query and returns every service, sorted by
// name. A TrueNAS without the method yields an empty list, not an error.
func ListServices(ctx context.Context, c *Client) ([]Service, error) {
	raw, err := c.Call(ctx, "service.query", []any{})
	if errors.Is(err, ErrNoMethod) {
		return []Service{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("service.query: %w", err)
	}
	var rawList []rawService
	if err := json.Unmarshal(raw, &rawList); err != nil {
		return nil, fmt.Errorf("service.query: decode: %w", err)
	}
	return buildServices(rawList), nil
}

func buildServices(rawList []rawService) []Service {
	out := make([]Service, 0, len(rawList))
	for _, r := range rawList {
		if r.Service == "" {
			continue
		}
		out = append(out, Service{Name: r.Service, State: r.State, Enable: r.Enable})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// VisibleServices keeps the services worth showing and sets Problem. A service
// is kept if it is running, set to start on boot, or has an enabled share (the
// first thing to check when a share stops working). It is a problem when it
// should be running -- start on boot, or enabled shares -- and is not.
func VisibleServices(all []Service, shares []Share) []Service {
	served := map[string]bool{}
	for _, sh := range shares {
		if sh.Enabled {
			served[sh.Service] = true
		}
	}
	out := make([]Service, 0, len(all))
	for _, s := range all {
		running := s.State == "RUNNING"
		wanted := s.Enable || served[s.Name]
		if !running && !wanted {
			continue // off, and nothing needs it
		}
		s.Problem = wanted && !running
		out = append(out, s)
	}
	return out
}

// ListShares queries sharing.smb.query and sharing.nfs.query. A method the
// TrueNAS does not have contributes no shares; any other error fails the call.
func ListShares(ctx context.Context, c *Client) ([]Share, error) {
	shares := []Share{}
	for _, src := range []struct{ method, service string }{
		{"sharing.smb.query", "cifs"},
		{"sharing.nfs.query", "nfs"},
	} {
		raw, err := c.Call(ctx, src.method, []any{})
		if errors.Is(err, ErrNoMethod) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", src.method, err)
		}
		var rawList []rawShare
		if err := json.Unmarshal(raw, &rawList); err != nil {
			return nil, fmt.Errorf("%s: decode: %w", src.method, err)
		}
		shares = append(shares, buildShares(src.service, rawList)...)
	}
	sort.SliceStable(shares, func(i, j int) bool {
		if shares[i].Service != shares[j].Service {
			return shares[i].Service < shares[j].Service
		}
		return shares[i].Name < shares[j].Name
	})
	return shares, nil
}

func buildShares(service string, rawList []rawShare) []Share {
	var out []Share
	for _, r := range rawList {
		name := r.Name
		if name == "" {
			name = r.Comment
		}
		paths := r.Paths
		if r.Path != "" {
			paths = []string{r.Path}
		}
		if len(paths) == 0 {
			paths = []string{""} // still a share; the UI flags the missing path
		}
		enabled := r.Enabled == nil || *r.Enabled // field absent: assume on
		for _, p := range paths {
			out = append(out, Share{Service: service, Name: name, Path: p, Enabled: enabled, ReadOnly: r.RO || r.Readonly, Locked: r.Locked})
		}
	}
	return out
}
