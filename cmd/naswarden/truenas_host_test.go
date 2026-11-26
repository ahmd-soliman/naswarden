package main

import (
	"strings"
	"testing"
)

func TestTruenasAddress(t *testing.T) {
	tests := []struct {
		name, host, tls string
		wantHost        string
		wantTLS         bool
		wantWarn        string // substring of the warning, "" for none
		wantErr         string // substring of the error, "" for none
	}{
		{name: "plain", host: "truenas.local:8443", tls: "true", wantHost: "truenas.local:8443", wantTLS: true},
		{name: "no port", host: "truenas.local", wantHost: "truenas.local"},
		{name: "ip and port, tls off", host: "192.168.1.10:80", wantHost: "192.168.1.10:80"},
		{name: "ipv6", host: "[fd00::1]:443", tls: "true", wantHost: "[fd00::1]:443", wantTLS: true},
		{name: "surrounding space", host: "  nas:443 ", tls: "true", wantHost: "nas:443", wantTLS: true},
		{name: "https scheme sets tls", host: "https://192.168.8.100:8443", wantHost: "192.168.8.100:8443", wantTLS: true, wantWarn: "without a scheme"},
		{name: "https with slash", host: "https://192.168.8.100:8443/", wantHost: "192.168.8.100:8443", wantTLS: true, wantWarn: "without a scheme"},
		{name: "wss with path", host: "wss://nas/websocket", wantHost: "nas", wantTLS: true, wantWarn: "without a scheme"},
		{name: "http scheme means no tls", host: "http://nas:80", wantHost: "nas:80", wantWarn: "TLS false"},
		{name: "uppercase scheme", host: "HTTPS://nas:443", wantHost: "nas:443", wantTLS: true, wantWarn: "without a scheme"},
		{name: "scheme agrees with TRUENAS_TLS", host: "https://nas:443", tls: "true", wantHost: "nas:443", wantTLS: true, wantWarn: "without a scheme"},
		{name: "TRUENAS_TLS wins over scheme", host: "https://nas:443", tls: "false", wantHost: "nas:443", wantTLS: false, wantWarn: "TRUENAS_TLS wins"},
		{name: "path without scheme", host: "nas:443/ui", tls: "true", wantHost: "nas:443", wantTLS: true, wantWarn: "should be host:port"},
		{name: "empty", host: "", wantErr: "no host"},
		{name: "scheme only", host: "https://", wantErr: "no host"},
		{name: "unsupported scheme", host: "ftp://nas", wantErr: "unsupported scheme"},
		{name: "bad port", host: "nas:abc", wantErr: "not a number"},
		{name: "port too big", host: "nas:70000", wantErr: "not a number"},
		{name: "empty port", host: "nas:", wantErr: "empty port"},
		{name: "space inside", host: "my nas:443", wantErr: "not a host name"},
		{name: "user info", host: "https://admin@nas", wantErr: "not a host name"},
		{name: "ipv6 without brackets", host: "fd00::1:443", wantErr: "needs brackets"},
		{name: "unclosed bracket", host: "[fd00::1:443", wantErr: "unclosed"},
		{name: "no host before port", host: ":443", wantErr: "no host"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, useTLS, warnings, err := truenasAddress(tt.host, tt.tls)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				if !strings.Contains(err.Error(), "host:port") {
					t.Errorf("error does not say what to use: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if host != tt.wantHost || useTLS != tt.wantTLS {
				t.Errorf("got host %q tls %v, want %q %v", host, useTLS, tt.wantHost, tt.wantTLS)
			}
			joined := strings.Join(warnings, "\n")
			switch {
			case tt.wantWarn == "" && len(warnings) != 0:
				t.Errorf("unexpected warnings: %v", warnings)
			case tt.wantWarn != "" && !strings.Contains(joined, tt.wantWarn):
				t.Errorf("warnings %q do not contain %q", joined, tt.wantWarn)
			}
		})
	}
}
