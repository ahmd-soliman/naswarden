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

// Client is a self-healing connection to the TrueNAS middleware. If the
// websocket drops (TrueNAS reboot, network blip) every in-flight call fails
// immediately and the next Call transparently redials, re-handshakes and
// re-authenticates -- previously the read loop just exited and every later
// call timed out until the whole process was restarted.
type Client struct {
	host, apiKey        string
	useTLS, insecureTLS bool

	nextID atomic.Int64

	connMu sync.Mutex // guards conn and serializes (re)dialing
	conn   *websocket.Conn

	writeMu sync.Mutex // gorilla/websocket allows one concurrent writer

	mu      sync.Mutex // guards pending
	pending map[string]pendingCall
}

type pendingCall struct {
	ch   chan ddpMessage
	conn *websocket.Conn // so a dropped conn only fails its own calls
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
// certificate on a LAN. The returned client redials on its own after a drop.
func Connect(ctx context.Context, host, apiKey string, useTLS, insecureTLS bool) (*Client, error) {
	c := &Client{
		host: host, apiKey: apiKey, useTLS: useTLS, insecureTLS: insecureTLS,
		pending: make(map[string]pendingCall),
	}
	if _, err := c.current(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// current returns the live connection, dialing a fresh one if there is none.
func (c *Client) current(ctx context.Context) (*websocket.Conn, error) {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn != nil {
		return c.conn, nil
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

func (c *Client) dial(ctx context.Context) (*websocket.Conn, error) {
	scheme := "ws"
	if c.useTLS {
		scheme = "wss"
	}
	url := fmt.Sprintf("%s://%s/websocket", scheme, c.host)

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	if c.useTLS && c.insecureTLS {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in for self-signed LAN certs
	}
	conn, _, err := dialer.DialContext(ctx, url, nil) //nolint:bodyclose // gorilla hands the upgraded conn over; no body to close
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", url, err)
	}

	connected := make(chan struct{})
	go c.readLoop(conn, connected)

	if err := c.write(conn, ddpMessage{Msg: "connect", Version: "1", Support: []string{"1"}}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send connect handshake: %w", err)
	}
	select {
	case <-connected:
	case <-ctx.Done():
		conn.Close()
		return nil, fmt.Errorf("waiting for DDP \"connected\" ack: %w", ctx.Err())
	}

	if _, err := c.callOn(ctx, conn, "auth.login_with_api_key", []any{c.apiKey}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	return conn, nil
}

// Call invokes a method and waits for its response, correlated by ID. It
// reconnects first if the previous connection was lost.
func (c *Client) Call(ctx context.Context, method string, params []any) (json.RawMessage, error) {
	conn, err := c.current(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	return c.callOn(ctx, conn, method, params)
}

func (c *Client) write(conn *websocket.Conn, msg ddpMessage) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(msg)
}

func (c *Client) callOn(ctx context.Context, conn *websocket.Conn, method string, params []any) (json.RawMessage, error) {
	id := fmt.Sprintf("%d", c.nextID.Add(1))
	respCh := make(chan ddpMessage, 1)

	c.mu.Lock()
	c.pending[id] = pendingCall{ch: respCh, conn: conn}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	req := ddpMessage{Msg: "method", Method: method, Params: params, ID: id}
	if err := c.write(conn, req); err != nil {
		c.drop(conn)
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

// drop discards a dead connection so the next Call redials, and fails every
// call still waiting on it instead of letting each sit out its timeout.
func (c *Client) drop(conn *websocket.Conn) {
	conn.Close()

	c.connMu.Lock()
	if c.conn == conn {
		c.conn = nil
	}
	c.connMu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.pending {
		if p.conn == conn {
			select {
			case p.ch <- ddpMessage{Msg: "error", Error: &ddpError{Reason: "connection lost"}}:
			default:
			}
		}
	}
}

func (c *Client) readLoop(conn *websocket.Conn, connected chan struct{}) {
	defer c.drop(conn)
	acked := false
	for {
		var msg ddpMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		switch msg.Msg {
		case "connected":
			if !acked {
				acked = true
				close(connected)
			}
		case "result", "error":
			c.mu.Lock()
			p, ok := c.pending[msg.ID]
			c.mu.Unlock()
			if ok {
				select {
				case p.ch <- msg:
				default:
				}
			}
		}
	}
}

func (c *Client) Close() error {
	c.connMu.Lock()
	conn := c.conn
	c.conn = nil
	c.connMu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}
