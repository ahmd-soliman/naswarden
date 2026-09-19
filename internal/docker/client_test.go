package docker

import (
	"testing"
)

func TestIsStopped(t *testing.T) {
	tests := []struct {
		state    string
		expected bool
	}{
		{"exited", true},
		{"created", true},
		{"dead", true},
		{"running", false},
		{"restarting", false},
		{"paused", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		if got := IsStopped(tt.state); got != tt.expected {
			t.Errorf("IsStopped(%q) = %v; want %v", tt.state, got, tt.expected)
		}
	}
}

func TestApplyInspect(t *testing.T) {
	var c Container
	var insp inspectResponse

	insp.State.StartedAt = "2026-09-19T10:00:00Z"
	insp.State.ExitCode = 137
	insp.State.OOMKilled = true
	insp.HostConfig.RestartPolicy.Name = "unless-stopped"
	insp.Config.Entrypoint = []string{"/bin/sh"}
	insp.Config.Cmd = []string{"-c", "exit 1"}
	insp.Mounts = []struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	}{
		{Type: "bind", Source: "/host/data", Destination: "/data", RW: false},
	}
	insp.NetworkSettings.Networks = map[string]struct {
		IPAddress string `json:"IPAddress"`
	}{
		"bridge": {IPAddress: "172.17.0.2"},
	}
	insp.HostConfig.PortBindings = map[string][]struct {
		HostPort string `json:"HostPort"`
	}{
		"80/tcp": {{HostPort: "8080"}},
	}

	applyInspect(&c, insp)

	if c.StartedAt != "2026-09-19T10:00:00Z" {
		t.Errorf("StartedAt = %q; want %q", c.StartedAt, "2026-09-19T10:00:00Z")
	}
	if c.ExitCode != 137 {
		t.Errorf("ExitCode = %d; want 137", c.ExitCode)
	}
	if !c.OOMKilled {
		t.Errorf("OOMKilled = false; want true")
	}
	if c.RestartPolicy != "unless-stopped" {
		t.Errorf("RestartPolicy = %q; want %q", c.RestartPolicy, "unless-stopped")
	}
	if c.Command != "/bin/sh -c exit 1" {
		t.Errorf("Command = %q; want %q", c.Command, "/bin/sh -c exit 1")
	}
	if len(c.Mounts) != 1 || !c.Mounts[0].ReadOnly {
		t.Errorf("Mounts = %+v; want 1 read-only mount", c.Mounts)
	}
	if len(c.Networks) != 1 || c.Networks[0].IP != "172.17.0.2" {
		t.Errorf("Networks = %+v; want 1 network with IP 172.17.0.2", c.Networks)
	}
	if len(c.Ports) != 1 || c.Ports[0] != "80/tcp → 8080" {
		t.Errorf("Ports = %+v; want ['80/tcp → 8080']", c.Ports)
	}
}
