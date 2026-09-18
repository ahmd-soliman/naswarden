// Package ws implements a small WebSocket broadcast hub: naswarden polls
// TrueNAS on an internal ticker and pushes updates to every connected
// browser client, rather than clients polling an HTTP endpoint.
package ws

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// Same-origin dashboard behind Caddy; no cross-origin browser clients
	// expected. Revisit if naswarden is ever embedded cross-site.
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
	last    []byte // most recent broadcast payload, replayed to new joiners
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]struct{})}
}

// ServeHTTP upgrades the connection and registers it. A client that
// connects between two refresh ticks would otherwise see nothing until the
// next tick (up to naswarden's whole internal refresh interval) -- send it
// the last known state immediately instead. Each connection is read-only
// from the client's side (naswarden only pushes), so the read loop below
// only exists to detect disconnects and clean up.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "err", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	last := h.last
	h.mu.Unlock()

	if last != nil {
		if err := conn.WriteMessage(websocket.TextMessage, last); err != nil {
			slog.Warn("failed to replay last state to new client", "err", err)
		}
	}

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// Broadcast sends the given JSON payload to every connected client, and
// caches it so clients connecting later get the current state immediately.
func (h *Hub) Broadcast(payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.last = payload
	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			slog.Warn("broadcast failed, dropping client", "err", err)
			conn.Close()
			delete(h.clients, conn)
		}
	}
}
