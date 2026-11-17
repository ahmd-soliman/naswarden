package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthGoesStaleAndRecovers(t *testing.T) {
	clock := time.Now()
	h := newHealth(3 * time.Minute)
	h.now = func() time.Time { return clock }
	h.markOK()

	get := func() int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		return rec.Code
	}
	if c := get(); c != 200 {
		t.Fatalf("fresh: got %d", c)
	}
	clock = clock.Add(2 * time.Minute)
	if c := get(); c != 200 {
		t.Fatalf("within max age: got %d", c)
	}
	clock = clock.Add(2 * time.Minute)
	if c := get(); c != 503 {
		t.Fatalf("stale: got %d, want 503", c)
	}
	h.markOK()
	if c := get(); c != 200 {
		t.Fatalf("after a good refresh: got %d", c)
	}
}

func TestHealthSaysWhenItNeverRefreshed(t *testing.T) {
	clock := time.Unix(1000, 0)
	h := newHealth(time.Minute)
	h.now = func() time.Time { return clock }
	h.last.Store(clock.UnixNano())

	clock = clock.Add(2 * time.Minute)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "no successful refresh since start") {
		t.Errorf("never refreshed: %d %q, want 503 naming the cause", rec.Code, rec.Body.String())
	}

	h.markOK()
	clock = clock.Add(2 * time.Minute)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "last successful refresh") {
		t.Errorf("stale after a success: %d %q, want the age message", rec.Code, rec.Body.String())
	}
}
