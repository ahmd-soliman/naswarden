package main

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
)

func TestDockerAddress(t *testing.T) {
	tests := []struct {
		name                 string
		proxyURL, host       string
		socketMounted        bool
		wantVar, wantAddr    string
		wantWarning, wantErr string // substrings; "" means none
	}{
		{name: "neither set, no socket: Docker off"},
		{name: "neither set, socket mounted: use it", socketMounted: true,
			wantVar: "mounted socket", wantAddr: "unix:///var/run/docker.sock"},
		{name: "a set variable beats a mounted socket", host: "tcp://docker-proxy:2375", socketMounted: true,
			wantVar: "DOCKER_HOST", wantAddr: "tcp://docker-proxy:2375"},
		{name: "the proxy beats a mounted socket", proxyURL: "http://docker-proxy:2375", socketMounted: true,
			wantVar: "DOCKER_PROXY_URL", wantAddr: "http://docker-proxy:2375"},
		{name: "proxy", proxyURL: "http://docker-proxy:2375",
			wantVar: "DOCKER_PROXY_URL", wantAddr: "http://docker-proxy:2375"},
		{name: "proxy in tcp form", proxyURL: "tcp://docker-proxy:2375",
			wantVar: "DOCKER_PROXY_URL", wantAddr: "tcp://docker-proxy:2375"},
		{name: "socket", host: "unix:///var/run/docker.sock",
			wantVar: "DOCKER_HOST", wantAddr: "unix:///var/run/docker.sock"},
		{name: "proxy through DOCKER_HOST", host: "tcp://docker-proxy:2375",
			wantVar: "DOCKER_HOST", wantAddr: "tcp://docker-proxy:2375"},
		{name: "both set: the proxy wins, with a warning",
			proxyURL: "http://docker-proxy:2375", host: "unix:///var/run/docker.sock",
			wantVar: "DOCKER_PROXY_URL", wantAddr: "http://docker-proxy:2375", wantWarning: "DOCKER_HOST is ignored"},
		{name: "both set to the same value: no warning",
			proxyURL: "http://docker-proxy:2375", host: "http://docker-proxy:2375",
			wantVar: "DOCKER_PROXY_URL", wantAddr: "http://docker-proxy:2375"},
		{name: "a socket is not a proxy", proxyURL: "unix:///var/run/docker.sock",
			wantVar: "DOCKER_PROXY_URL", wantAddr: "unix:///var/run/docker.sock", wantErr: "set DOCKER_HOST for a socket"},
		{name: "invalid DOCKER_HOST is named", host: "ssh://user@nas",
			wantVar: "DOCKER_HOST", wantAddr: "ssh://user@nas", wantErr: "invalid DOCKER_HOST"},
		{name: "invalid DOCKER_PROXY_URL is named", proxyURL: "docker-proxy:2375",
			wantVar: "DOCKER_PROXY_URL", wantAddr: "docker-proxy:2375", wantErr: "invalid DOCKER_PROXY_URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{"DOCKER_PROXY_URL": tt.proxyURL, "DOCKER_HOST": tt.host}
			isSocket := func(path string) bool { return tt.socketMounted && path == "/var/run/docker.sock" }
			variable, address, warning, err := dockerAddress(func(k string) string { return env[k] }, isSocket)
			if variable != tt.wantVar || address != tt.wantAddr {
				t.Errorf("got %s=%q, want %s=%q", variable, address, tt.wantVar, tt.wantAddr)
			}
			if (tt.wantWarning == "") != (warning == "") || !strings.Contains(warning, tt.wantWarning) {
				t.Errorf("warning = %q, want %q", warning, tt.wantWarning)
			}
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("err = %v, want none", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestNewDockerStatus(t *testing.T) {
	tests := []struct {
		variable, address string
		err               error
		want              dockerStatus
	}{
		{"", "", nil, dockerStatus{State: "off"}},
		{"mounted socket", "unix:///var/run/docker.sock", nil, dockerStatus{State: "on", Access: "socket"}},
		{"DOCKER_HOST", "unix:///run/docker.sock", nil, dockerStatus{State: "on", Access: "socket"}},
		{"DOCKER_PROXY_URL", "http://docker-proxy:2375", nil, dockerStatus{State: "on", Access: "proxy"}},
		{"DOCKER_HOST", "tcp://docker-proxy:2375", nil, dockerStatus{State: "on", Access: "api"}},
		{"mounted socket", "unix:///var/run/docker.sock", errors.New("no permission"), dockerStatus{State: "error", Error: "no permission"}},
	}
	for _, tt := range tests {
		if got := newDockerStatus(tt.variable, tt.address, tt.err); got != tt.want {
			t.Errorf("newDockerStatus(%q, %q, %v) = %+v, want %+v", tt.variable, tt.address, tt.err, got, tt.want)
		}
	}
}

func TestCheckDockerSocket(t *testing.T) {
	refused := fmt.Errorf("cannot connect to the Docker socket /var/run/docker.sock: %w", syscall.ECONNREFUSED)
	denied := errors.New("no permission to use the Docker socket /var/run/docker.sock: run naswarden with the socket's group")
	ok := func(string) error { return nil }
	fail := func(err error) func(string) error { return func(string) error { return err } }

	tests := []struct {
		name, variable, address string
		check                   func(string) error
		wantAddr                string
		wantNote, wantErr       bool
	}{
		{name: "usable default socket stays on", variable: "mounted socket", address: "unix:///var/run/docker.sock", check: ok,
			wantAddr: "unix:///var/run/docker.sock"},
		{name: "default socket nothing listens on: Docker off, a note, no error", variable: "mounted socket", address: "unix:///var/run/docker.sock", check: fail(refused),
			wantAddr: "", wantNote: true},
		{name: "default socket without permission is still an error", variable: "mounted socket", address: "unix:///var/run/docker.sock", check: fail(denied),
			wantAddr: "unix:///var/run/docker.sock", wantErr: true},
		{name: "a socket the user asked for that nothing listens on is an error", variable: "DOCKER_HOST", address: "unix:///var/run/docker.sock", check: fail(refused),
			wantAddr: "unix:///var/run/docker.sock", wantErr: true},
		{name: "a network address is not probed here", variable: "DOCKER_PROXY_URL", address: "http://docker-proxy:2375",
			check: func(string) error { t.Error("must not probe a non-socket address"); return nil }, wantAddr: "http://docker-proxy:2375"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, note, err := checkDockerSocket(tt.variable, tt.address, tt.check)
			if addr != tt.wantAddr || (note != "") != tt.wantNote || (err != nil) != tt.wantErr {
				t.Errorf("got addr=%q note=%q err=%v; want addr=%q note=%v err=%v", addr, note, err, tt.wantAddr, tt.wantNote, tt.wantErr)
			}
		})
	}
}

func TestUnreachableDefaultSocketShowsDockerOffNotError(t *testing.T) {
	// What the UI gets in that case: the "Docker not connected" card, no error.
	got := newDockerStatus("", "", nil)
	if got != (dockerStatus{State: "off"}) {
		t.Errorf("status = %+v, want off", got)
	}
}
