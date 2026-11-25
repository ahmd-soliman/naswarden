package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(t *testing.T, allowed, host string) *httptest.ResponseRecorder {
	t.Helper()
	g := newHostGuard(allowed, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = host
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

// enforced is a list that turns the check on without allowing any of the
// hosts the tests probe.
const enforced = "unrelated.example"

func TestHostGuardOffUntilConfigured(t *testing.T) {
	for _, host := range []string{"evil.example", "naswarden.home.example:443", "192.168.1.10", ""} {
		if got := serve(t, "", host).Code; got != 200 {
			t.Errorf("host %q: status %d, want 200 while NASWARDEN_ALLOWED_HOSTS is unset", host, got)
		}
	}
}

func TestHostGuardDefaults(t *testing.T) {
	tests := []struct {
		host string
		want int
	}{
		{"192.168.1.10", 200},
		{"192.168.1.10:30597", 200},
		{"10.0.0.5:8080", 200},
		{"[::1]:8080", 200},
		{"[::1]", 200},
		{"[fe80::1%eth0]:8080", 200},
		{"2001:db8::1", 200},
		{"localhost", 200},
		{"localhost:8080", 200},
		{"LOCALHOST:8080", 200},
		{"nas", 200},
		{"truenas:30597", 200},
		{"nas.", 200},
		{"evil.example", 421},
		{"evil.example:30597", 421},
		{"nas.evil.example", 421},
		{"naswarden.home.example:443", 421},
		{"192.168.1.10.evil.example", 421},
		{"", 421},
	}
	for _, tt := range tests {
		if got := serve(t, enforced, tt.host).Code; got != tt.want {
			t.Errorf("host %q: status %d, want %d", tt.host, got, tt.want)
		}
	}
}

func TestHostGuardAllowedHosts(t *testing.T) {
	allowed := " Naswarden.Home.Example , other.example:8443 "
	tests := []struct {
		host string
		want int
	}{
		{"naswarden.home.example", 200},
		{"naswarden.home.example:443", 200},
		{"NASWARDEN.HOME.EXAMPLE", 200},
		{"other.example:8443", 200},
		{"evil.example", 421},
		{"nas", 200}, // defaults still apply
	}
	for _, tt := range tests {
		if got := serve(t, allowed, tt.host).Code; got != tt.want {
			t.Errorf("host %q: status %d, want %d", tt.host, got, tt.want)
		}
	}
}

func TestHostGuardWildcardTurnsItOff(t *testing.T) {
	if got := serve(t, "*", "evil.example").Code; got != 200 {
		t.Errorf("status %d, want 200 with *", got)
	}
	if got := serve(t, "a.example, *", "evil.example").Code; got != 200 {
		t.Errorf("status %d, want 200 with * in a list", got)
	}
}

func TestHostGuardRejectionNamesTheVariable(t *testing.T) {
	rec := serve(t, enforced, `evil"><script>.example`)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("status %d, want 421", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "NASWARDEN_ALLOWED_HOSTS") {
		t.Errorf("body does not name the variable: %q", body)
	}
	if strings.ContainsAny(body, "<>") {
		t.Errorf("body echoes unsafe characters: %q", body)
	}
}
