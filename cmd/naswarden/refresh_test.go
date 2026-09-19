package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/ahmd-soliman/naswarden/internal/truenas"
	"github.com/ahmd-soliman/naswarden/internal/ws"
)

// scriptedTrueNAS answers each middleware method with a canned JSON result, or
// an error while a method is in the failing set. Unscripted methods return an
// empty list, which every List* call accepts.
type scriptedTrueNAS struct {
	*httptest.Server
	mu      sync.Mutex
	results map[string]string
	failing map[string]bool
}

func (s *scriptedTrueNAS) fail(method string, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failing[method] = on
}

func (s *scriptedTrueNAS) set(method, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[method] = result
}

func newScriptedTrueNAS(t *testing.T) *scriptedTrueNAS {
	t.Helper()
	s := &scriptedTrueNAS{results: map[string]string{}, failing: map[string]bool{}}
	up := websocket.Upgrader{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var m struct {
				Msg    string `json:"msg"`
				ID     string `json:"id"`
				Method string `json:"method"`
			}
			if conn.ReadJSON(&m) != nil {
				return
			}
			switch m.Msg {
			case "connect":
				_ = conn.WriteJSON(map[string]any{"msg": "connected"})
			case "method":
				s.mu.Lock()
				failing, result := s.failing[m.Method], s.results[m.Method]
				s.mu.Unlock()
				switch {
				case m.Method == "auth.login_with_api_key":
					_ = conn.WriteJSON(map[string]any{"msg": "result", "id": m.ID, "result": true})
				case failing:
					_ = conn.WriteJSON(map[string]any{"msg": "result", "id": m.ID, "error": map[string]any{"error": "EFAULT", "reason": "boom"}})
				default:
					if result == "" {
						result = "[]"
					}
					_ = conn.WriteJSON(map[string]any{"msg": "result", "id": m.ID, "result": json.RawMessage(result)})
				}
			}
		}
	}))
	t.Cleanup(s.Close)
	return s
}

type refreshRig struct {
	tn     *scriptedTrueNAS
	client *truenas.Client
	hub    *ws.Hub
	cache  *lastGood
	health *healthState
	rates  *truenas.DiskRates
	conn   *websocket.Conn
}

func newRefreshRig(t *testing.T) *refreshRig {
	t.Helper()
	tn := newScriptedTrueNAS(t)
	tn.set("disk.details", `{"used":[],"unused":[]}`)
	tn.set("system.info", `{"hostname":"nas","version":"25.10","uptime_seconds":100,"physmem":1024}`)
	client := truenas.NewClient(strings.TrimPrefix(tn.URL, "http://"), "key", false, false)
	t.Cleanup(func() { _ = client.Close() })

	hub := ws.NewHub()
	hs := httptest.NewServer(hub)
	t.Cleanup(hs.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(hs.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return &refreshRig{tn: tn, client: client, hub: hub, cache: &lastGood{}, health: newHealth(time.Minute), rates: truenas.NewDiskRates(), conn: conn}
}

func (r *refreshRig) run() { refresh(r.client, nil, nil, r.hub, r.cache, r.health, r.rates) }

// next returns the next message the browser would receive.
func (r *refreshRig) next(t *testing.T) map[string]any {
	t.Helper()
	_ = r.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var msg map[string]any
	if err := r.conn.ReadJSON(&msg); err != nil {
		t.Fatalf("no message received: %v", err)
	}
	return msg
}

func TestRefreshBroadcastsStateAndMarksHealthy(t *testing.T) {
	r := newRefreshRig(t)
	r.tn.set("pool.query", `[{"name":"tank","status":"ONLINE","healthy":true,"size":100,"allocated":10,"free":90,"topology":{"data":[]}}]`)
	r.health.last.Store(0) // pretend the last refresh was long ago

	r.run()

	msg := r.next(t)
	if msg["type"] != "state" {
		t.Fatalf("type = %v, want state (%v)", msg["type"], msg)
	}
	if stale, _ := msg["stale_sources"].([]any); len(stale) != 0 {
		t.Errorf("stale_sources = %v, want none", stale)
	}
	if pools, _ := msg["pools"].([]any); len(pools) != 1 {
		t.Errorf("pools = %v, want 1", msg["pools"])
	}
	rec := httptest.NewRecorder()
	r.health.ServeHTTP(rec, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("healthz = %d after a good refresh, want 200", rec.Code)
	}
}

func TestRefreshKeepsLastGoodWhenAnOptionalSourceFails(t *testing.T) {
	r := newRefreshRig(t)
	r.tn.set("alert.list", `[{"id":"a1","level":"WARNING","klass":"X","formatted":"disk warm","datetime":{"$date":1700000000000},"dismissed":false}]`)
	r.run()
	first := r.next(t)
	if alerts, _ := first["alerts"].([]any); len(alerts) != 1 {
		t.Fatalf("alerts = %v, want 1 on the first pass", first["alerts"])
	}

	r.tn.fail("alert.list", true)
	r.run()
	second := r.next(t)

	if second["type"] != "state" {
		t.Fatalf("type = %v, want state: one failing source must not stop the broadcast", second["type"])
	}
	if alerts, _ := second["alerts"].([]any); len(alerts) != 1 {
		t.Errorf("alerts = %v, want the previous list kept", second["alerts"])
	}
	stale, _ := second["stale_sources"].([]any)
	if len(stale) != 1 || stale[0] != "alerts" {
		t.Errorf("stale_sources = %v, want [alerts]", stale)
	}
}

func TestRefreshCoreFailureSendsErrorAndStaysUnhealthy(t *testing.T) {
	r := newRefreshRig(t)
	r.tn.fail("pool.query", true)
	r.health.last.Store(0)

	r.run()

	msg := r.next(t)
	if msg["type"] != "error" {
		t.Fatalf("type = %v, want error", msg["type"])
	}
	if m, _ := msg["message"].(string); !strings.Contains(m, "pools") {
		t.Errorf("message = %q, want it to name the failing call", m)
	}
	rec := httptest.NewRecorder()
	r.health.ServeHTTP(rec, nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("healthz = %d after a failed core refresh, want 503", rec.Code)
	}
}
