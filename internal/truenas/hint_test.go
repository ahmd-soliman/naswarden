package truenas

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// connectErr is the real error a Call returns for host, so each hint is
// checked against what the client actually produces.
func connectErr(t *testing.T, host string, useTLS, insecure bool) error {
	t.Helper()
	c := NewClient(host, "key", useTLS, insecure)
	t.Cleanup(func() { _ = c.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := c.Call(ctx, "system.info", nil)
	if err == nil {
		t.Fatalf("connecting to %s succeeded, want an error", host)
	}
	return fmt.Errorf("system.info: %w", err) // as GetServerInfo wraps it
}

func TestHint(t *testing.T) {
	// A port nothing listens on: the default-443-but-UI-on-8443 case.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := ln.Addr().String()
	ln.Close()

	plain := httptest.NewServer(http.NotFoundHandler()) // answers, but no websocket
	t.Cleanup(plain.Close)
	selfSigned := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(selfSigned.Close)
	host := func(s *httptest.Server) string {
		return strings.TrimPrefix(strings.TrimPrefix(s.URL, "http://"), "https://")
	}

	tests := []struct {
		name string
		err  error
		want string // substring of the hint
	}{
		{"connection refused", connectErr(t, closed, true, true), "HTTPS port"},
		{"not a websocket endpoint", connectErr(t, host(plain), false, false), "not as expected"},
		{"TLS to a plain HTTP port", connectErr(t, host(plain), true, true), "not as expected"},
		{"self-signed certificate", connectErr(t, host(selfSigned), true, false), "Skip certificate verification"},
		{"unknown host", connectErr(t, "naswarden-test.invalid:443", true, true), "does not resolve"},
		{"key rejected", fmt.Errorf("authenticate: %w", ErrKeyRejected), "Readonly Admin"},
		{"timeout", fmt.Errorf("system.info: %w", context.DeadlineExceeded), "did not answer in time"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Hint(tt.err); !strings.Contains(got, tt.want) {
				t.Errorf("Hint(%v) = %q, want it to contain %q", tt.err, got, tt.want)
			}
		})
	}

	if got := Hint(errors.New("something else")); got != "" {
		t.Errorf("Hint(unrecognised) = %q, want empty", got)
	}
	if Hint(nil) != "" {
		t.Error("Hint(nil) is not empty")
	}
}
