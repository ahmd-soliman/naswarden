package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// healthState backs /healthz. It reports healthy only while refreshes keep
// succeeding: a process that is up but has lost TrueNAS (and so serves ever
// older data) should fail its container healthcheck, not report 200.
type healthState struct {
	maxAge time.Duration
	last   atomic.Int64 // unix nanos of the last successful refresh
	now    func() time.Time
}

func newHealth(maxAge time.Duration) *healthState {
	h := &healthState{maxAge: maxAge, now: time.Now}
	// The first refresh runs immediately at startup; give it maxAge to land.
	h.last.Store(h.now().UnixNano())
	return h
}

func (h *healthState) markOK() { h.last.Store(h.now().UnixNano()) }

func (h *healthState) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	age := h.now().Sub(time.Unix(0, h.last.Load()))
	if age > h.maxAge {
		http.Error(w, fmt.Sprintf("last successful refresh %s ago", age.Round(time.Second)), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
