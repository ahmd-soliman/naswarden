package main

import (
	"strings"
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
