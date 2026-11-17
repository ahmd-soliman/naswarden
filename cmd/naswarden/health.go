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
	ever   atomic.Bool  // whether any refresh has succeeded since start
	now    func() time.Time
}

func newHealth(maxAge time.Duration) *healthState {
	h := &healthState{maxAge: maxAge, now: time.Now}
	// The first refresh runs immediately at startup; give it maxAge to land.
	h.last.Store(h.now().UnixNano())
	return h
}

func (h *healthState) markOK() {
	h.last.Store(h.now().UnixNano())
	h.ever.Store(true)
}

func (h *healthState) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	age := h.now().Sub(time.Unix(0, h.last.Load()))
	if age > h.maxAge {
		if !h.ever.Load() {
			// A first install with a wrong host, port or key: say so
			// rather than suggest the data went stale.
			http.Error(w, fmt.Sprintf("no successful refresh since start (%s): check the TrueNAS address, port and API key", age.Round(time.Second)), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, fmt.Sprintf("last successful refresh %s ago", age.Round(time.Second)), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
