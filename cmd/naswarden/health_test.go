package main

import (
	"net/http"
	"net/http/httptest"
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
