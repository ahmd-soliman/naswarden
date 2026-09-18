// Package truenas implements a minimal client for TrueNAS SCALE's
// middleware WebSocket API (/websocket).
//
// Despite this API sometimes being described as "JSON-RPC 2.0," it's
// actually Meteor's DDP protocol: a "connect" handshake must be sent
// before any method call, and method calls/responses are wrapped in a
// {"msg": ...} envelope with string IDs -- not the plain
// {"jsonrpc": "2.0", ...} envelope the name suggests. Confirmed by reading
// the actual (working) implementation in bmanojlovic/terraform-provider-truenas
// after a first attempt using plain JSON-RPC hung silently against a real
// TrueNAS SCALE 25.10.7 box -- the server never responded to a message
// shape it didn't recognize.
package truenas

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn   *websocket.Conn
	nextID atomic.Int64

	mu       sync.Mutex
	pending  map[string]chan ddpMessage
	connOnce chan struct{}
}

// ddpMessage covers every shape this client needs to send or receive:
// {"msg": "connect", ...}, {"msg": "connected", ...},
// {"msg": "method", ...} (outgoing calls), {"msg": "result"|"error", ...}.
type ddpMessage struct {
	Msg     string          `json:"msg"`
	Version string          `json:"version,omitempty"`
	Support []string        `json:"support,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  []any           `json:"params,omitempty"`
	ID      string          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ddpError       `json:"error,omitempty"`
}

type ddpError struct {
	Error   string `json:"error"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func (e *ddpError) String() string {
	if e.Reason != "" {
		return e.Reason
	}
	return e.Message
}

// Connect dials the TrueNAS websocket endpoint, performs the DDP "connect"
// handshake, then authenticates with an API key. host should include the
// port if non-default (e.g. "192.0.2.10:8443"). insecureTLS skips
// certificate verification, needed for TrueNAS's default self-signed UI
// certificate on a LAN.
func Connect(ctx context.Context, host, apiKey string, useTLS, insecureTLS bool) (*Client, error) {
	scheme := "ws"
	if useTLS {
		scheme = "wss"
	}
	url := fmt.Sprintf("%s://%s/websocket", scheme, host)

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	if useTLS && insecureTLS {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in for self-signed LAN certs
	}
	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", url, err)
	}

	c := &Client{
		conn:     conn,
		pending:  make(map[string]chan ddpMessage),
		connOnce: make(chan struct{}),
	}
	go c.readLoop()

	if err := conn.WriteJSON(ddpMessage{Msg: "connect", Version: "1", Support: []string{"1"}}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send connect handshake: %w", err)
	}
	select {
	case <-c.connOnce:
	case <-ctx.Done():
		conn.Close()
		return nil, fmt.Errorf("waiting for DDP \"connected\" ack: %w", ctx.Err())
	}

	if _, err := c.Call(ctx, "auth.login_with_api_key", []any{apiKey}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("authenticate: %w", err)
	}

	return c, nil
}

// Call invokes a method and waits for its response, correlated by ID.
func (c *Client) Call(ctx context.Context, method string, params []any) (json.RawMessage, error) {
	id := fmt.Sprintf("%d", c.nextID.Add(1))
	respCh := make(chan ddpMessage, 1)

	c.mu.Lock()
	c.pending[id] = respCh
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	req := ddpMessage{Msg: "method", Method: method, Params: params, ID: id}
	if err := c.conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("write %s: %w", method, err)
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, resp.Error.String())
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("%s: %w", method, ctx.Err())
	}
}

func (c *Client) readLoop() {
	connAckSent := false
	for {
		var msg ddpMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}

		switch msg.Msg {
		case "connected":
			if !connAckSent {
				connAckSent = true
				close(c.connOnce)
			}
		case "result", "error":
			c.mu.Lock()
			ch, ok := c.pending[msg.ID]
			c.mu.Unlock()
			if ok {
				ch <- msg
			}
		}
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}
