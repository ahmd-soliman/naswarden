package truenas

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// fakeMiddleware speaks just enough DDP for the client: connect, login, and
// an echo-ish method. dropAfter closes the connection after that many method
// calls on one connection, simulating a TrueNAS restart or network drop.
func fakeMiddleware(t *testing.T, conns *atomic.Int32, dropAfter int) *httptest.Server {
	t.Helper()
	up := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conns.Add(1)
		calls := 0
		for {
			var m ddpMessage
			if err := conn.ReadJSON(&m); err != nil {
				return
			}
			switch m.Msg {
			case "connect":
				_ = conn.WriteJSON(ddpMessage{Msg: "connected"})
			case "method":
				calls++
				if m.Method != "auth.login_with_api_key" && dropAfter > 0 && calls > dropAfter {
					conn.Close()
					return
				}
				_ = conn.WriteJSON(ddpMessage{Msg: "result", ID: m.ID, Result: json.RawMessage(`true`)})
			}
		}
	}))
}

func hostOf(s *httptest.Server) string { return strings.TrimPrefix(s.URL, "http://") }

func TestClientReconnectsAfterDrop(t *testing.T) {
	var conns atomic.Int32
	// login counts as call 1, so dropAfter=2 answers one real call then drops.
	srv := fakeMiddleware(t, &conns, 2)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Connect(ctx, hostOf(srv), "key", false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if _, err := c.Call(ctx, "system.info", nil); err != nil {
		t.Fatalf("first call: %v", err)
	}

	// The server drops on the next call: it must fail fast, not hang.
	start := time.Now()
	if _, err := c.Call(ctx, "system.info", nil); err == nil {
		t.Fatal("expected an error when the connection is dropped mid-call")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("dropped call took %v; pending calls must fail fast", time.Since(start))
	}

	// The next call redials and succeeds.
	if _, err := c.Call(ctx, "system.info", nil); err != nil {
		t.Fatalf("call after reconnect: %v", err)
	}
	if n := conns.Load(); n != 2 {
		t.Fatalf("expected 2 connections (initial + redial), got %d", n)
	}
}

func TestClientCallFailsWhenRedialFails(t *testing.T) {
	var conns atomic.Int32
	srv := fakeMiddleware(t, &conns, 1) // drops on the first real call
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Connect(ctx, hostOf(srv), "key", false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if _, err := c.Call(ctx, "system.info", nil); err == nil {
		t.Fatal("expected the dropped call to fail")
	}
	srv.Close() // TrueNAS is now unreachable: the redial must error, not hang
	if _, err := c.Call(ctx, "system.info", nil); err == nil {
		t.Fatal("expected an error when the redial fails")
	}
}
