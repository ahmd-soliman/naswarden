package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func dial(t *testing.T, srv *httptest.Server, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	return websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), h)
}

func read(t *testing.T, c *websocket.Conn) string {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, b, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func TestBroadcastAndReplay(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(h)
	defer srv.Close()

	h.Broadcast([]byte("one"))

	a, _, err := dial(t, srv, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if got := read(t, a); got != "one" {
		t.Fatalf("new client should get the last state, got %q", got)
	}

	b, _, err := dial(t, srv, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	read(t, b) // replay

	h.Broadcast([]byte("two"))
	if got := read(t, a); got != "two" {
		t.Fatalf("a: got %q", got)
	}
	if got := read(t, b); got != "two" {
		t.Fatalf("b: got %q", got)
	}
}

// A client that never reads must not stall broadcasts to others.
func TestSlowClientDoesNotBlockBroadcast(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(h)
	defer srv.Close()

	slow, _, err := dial(t, srv, "")
	if err != nil {
		t.Fatal(err)
	}
	defer slow.Close() // never reads

	fast, _, err := dial(t, srv, "")
	if err != nil {
		t.Fatal(err)
	}
	defer fast.Close()

	big := []byte(strings.Repeat("x", 256*1024))
	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			h.Broadcast(big)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Broadcast blocked on a slow client")
	}
}

func TestConcurrentBroadcastAndJoin(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(h)
	defer srv.Close()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				h.Broadcast([]byte("tick"))
				time.Sleep(time.Millisecond)
			}
		}
	}()
	for i := 0; i < 20; i++ {
		c, _, err := dial(t, srv, "")
		if err != nil {
			t.Fatal(err)
		}
		read(t, c)
		c.Close()
	}
	close(stop)
	wg.Wait()
}

func TestOriginCheck(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(h)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	if c, _, err := dial(t, srv, "http://"+host); err != nil {
		t.Fatalf("same-origin browser must be allowed: %v", err)
	} else {
		c.Close()
	}
	if c, _, err := dial(t, srv, "http://evil.example"); err == nil {
		c.Close()
		t.Fatal("cross-origin browser must be rejected")
	}
}

func TestBroadcastNoticeDoesNotReplaceState(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(h)
	defer srv.Close()

	// No state yet: a notice is cached so a late client still learns why.
	h.BroadcastNotice([]byte("no-data-reason"))
	a, _, err := dial(t, srv, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if got := read(t, a); got != "no-data-reason" {
		t.Fatalf("a client with no state should get the notice, got %q", got)
	}

	// Real state replaces it, and later notices go to live clients only.
	h.Broadcast([]byte("state"))
	if got := read(t, a); got != "state" {
		t.Fatalf("got %q", got)
	}
	h.BroadcastNotice([]byte("transient"))
	if got := read(t, a); got != "transient" {
		t.Fatalf("live client should see the notice, got %q", got)
	}
	b, _, err := dial(t, srv, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if got := read(t, b); got != "state" {
		t.Fatalf("a new client must get the last real state, not the notice: %q", got)
	}
}
