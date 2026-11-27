package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// Service is a TrueNAS service (SMB, NFS, SSH, ...) from `service.query`.
// Only services that are enabled or running are kept: one that is off and
// not set to start on boot is noise.
type Service struct {
	Name   string `json:"name"`   // middleware name: cifs, nfs, ssh, iscsitarget, ups, ...
	State  string `json:"state"`  // RUNNING, STOPPED, ...
	Enable bool   `json:"enable"` // start on boot
	// Problem: set to start on boot but not running. Computed here so the UI
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
}

type rawService struct {
	Service string `json:"service"`
	State   string `json:"state"`
	Enable  bool   `json:"enable"`
}

// Tolerant on purpose: none of these fields has been checked against a
// recorded response. NFS shares carry `path` on newer releases and `paths`
// (a list) on older ones.
type rawShare struct {
	Name    string   `json:"name"`
	Comment string   `json:"comment"`
	Path    string   `json:"path"`
	Paths   []string `json:"paths"`
	Enabled *bool    `json:"enabled"`
	RO      bool     `json:"ro"`
}

// ListServices queries service.query and returns the services that are
// enabled or running, sorted by name. A TrueNAS without the method yields an
// empty list, not an error.
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
		running := r.State == "RUNNING"
		if !running && !r.Enable {
			continue // off and not meant to be on
		}
		out = append(out, Service{
			Name:    r.Service,
			State:   r.State,
			Enable:  r.Enable,
			Problem: r.Enable && !running,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
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
			out = append(out, Share{Service: service, Name: name, Path: p, Enabled: enabled, ReadOnly: r.RO})
		}
	}
	return out
}
