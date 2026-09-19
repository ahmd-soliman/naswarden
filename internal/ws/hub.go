// Package ws implements a small WebSocket broadcast hub: naswarden polls
// TrueNAS on an internal ticker and pushes updates to every connected
// browser client, rather than clients polling an HTTP endpoint.
package ws

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 70 * time.Second
	pingPeriod = 30 * time.Second
	sendBuffer = 4 // a client further behind than this is dropped
)

// sameOrigin allows non-browser clients (no Origin header) and browsers
// whose page came from the same host they are connecting to. Without this
// any web page open in a LAN browser could read the dashboard state.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

var upgrader = websocket.Upgrader{CheckOrigin: sameOrigin}

// client owns one connection. All writes happen on its writer goroutine, so
// a slow browser can never block a broadcast or another client.
type client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
	last    []byte // most recent broadcast payload, replayed to new joiners
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

// ServeHTTP upgrades the connection and registers it. A client that
// connects between two refresh ticks would otherwise see nothing until the
// next tick -- it is queued the last known state immediately instead. The
// read loop exists to process pongs and detect disconnects; naswarden only
// pushes.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "err", err)
		return
	}

	c := &client{conn: conn, send: make(chan []byte, sendBuffer)}

	// Replay and registration happen under one lock so a broadcast cannot
	// slip between them and be missed, or written concurrently.
	h.mu.Lock()
	if h.last != nil {
		c.send <- h.last
	}
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	done := make(chan struct{})
	go c.writeLoop(done)

	defer func() {
		h.remove(c)
		close(done)
		conn.Close()
	}()

	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *client) writeLoop(done <-chan struct{}) {
	ping := time.NewTicker(pingPeriod)
	defer ping.Stop()
	for {
		select {
		case msg := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				c.conn.Close() // unblocks the read loop, which cleans up
				return
			}
		case <-ping.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.conn.Close()
				return
			}
		case <-done:
			return
		}
	}
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// BroadcastNotice sends a payload to the connected clients without replacing
// the cached state, so a transient problem message is not replayed to every
// later client in place of real data. If there is no state at all yet (nothing
// has ever been fetched), the notice is cached so a client that connects later
// still learns why there is no data.
func (h *Hub) BroadcastNotice(payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.last == nil {
		h.last = payload
	}
	for c := range h.clients {
		select {
		case c.send <- payload:
		default:
			slog.Warn("dropping slow websocket client")
			delete(h.clients, c)
			c.conn.Close()
		}
	}
}

// Broadcast queues the payload for every connected client and caches it so
// clients connecting later get the current state immediately. It never
// blocks on a client: one whose queue is full is too slow and is dropped.
func (h *Hub) Broadcast(payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.last = payload
	for c := range h.clients {
		select {
		case c.send <- payload:
		default:
			slog.Warn("dropping slow websocket client")
			delete(h.clients, c)
			c.conn.Close()
		}
	}
}
