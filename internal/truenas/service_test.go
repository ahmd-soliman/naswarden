package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func decodeServices(t *testing.T, fixture string, shares ...Share) []Service {
	t.Helper()
	var raw []rawService
	if err := json.Unmarshal([]byte(fixture), &raw); err != nil {
		t.Fatal(err)
	}
	return VisibleServices(buildServices(raw), shares)
}

func TestServicesRunningStoppedAndFlagged(t *testing.T) {
	got := decodeServices(t, `[
		{"id":1,"service":"nfs","enable":true,"state":"RUNNING","pids":[1]},
		{"id":2,"service":"cifs","enable":true,"state":"STOPPED","pids":[]},
		{"id":3,"service":"ssh","enable":false,"state":"RUNNING","pids":[2]},
		{"id":4,"service":"ftp","enable":false,"state":"STOPPED","pids":[]}
	]`)
	want := []Service{
		{Name: "cifs", State: "STOPPED", Enable: true, Problem: true}, // enabled but stopped
		{Name: "nfs", State: "RUNNING", Enable: true},
		{Name: "ssh", State: "RUNNING", Enable: false}, // running by hand: shown, not a problem
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v (disabled-and-stopped ftp must be hidden)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestStoppedServiceWithEnabledSharesIsShownAsAProblem(t *testing.T) {
	const fixture = `[{"service":"cifs","enable":false,"state":"STOPPED"},{"service":"nfs","enable":false,"state":"STOPPED"}]`
	got := decodeServices(t, fixture,
		Share{Service: "cifs", Name: "media", Enabled: true},
		Share{Service: "nfs", Name: "old", Enabled: false}) // a disabled share needs nothing
	if len(got) != 1 || got[0].Name != "cifs" || !got[0].Problem {
		t.Fatalf("want only cifs, flagged: %+v", got)
	}
	if got := decodeServices(t, fixture); len(got) != 0 {
		t.Fatalf("no shares, off and not enabled: want hidden, got %+v", got)
	}
}

func TestServicesEmptyIsNotNil(t *testing.T) {
	if got := decodeServices(t, `[]`); got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want an empty non-nil list", got)
	}
}

func TestBuildShares(t *testing.T) {
	smb := []rawShare{
		{Name: "media", Path: "/mnt/tank/media", Enabled: ptrBool(true), Readonly: true, Locked: true},
		{Name: "old", Path: "/mnt/tank/old", Enabled: ptrBool(false)},
		{Name: "nopath"}, // missing fields tolerated
	}
	got := buildShares("cifs", smb)
	if len(got) != 3 || !got[0].ReadOnly || !got[0].Locked || got[1].Enabled || !got[2].Enabled || got[2].Path != "" {
		t.Fatalf("smb: %+v", got)
	}
	nfs := buildShares("nfs", []rawShare{{Comment: "isos", Paths: []string{"/mnt/a", "/mnt/b"}, Enabled: ptrBool(true)}})
	if len(nfs) != 2 || nfs[0].Name != "isos" || nfs[1].Path != "/mnt/b" || nfs[0].Service != "nfs" {
		t.Fatalf("nfs (legacy paths list): %+v", nfs)
	}
}

func ptrBool(b bool) *bool { return &b }

// scripted answers each method with a result, or an error reply.
func scriptedClient(t *testing.T, results map[string]string, errs map[string]map[string]any) *Client {
	t.Helper()
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var m struct{ Msg, ID, Method string }
			if conn.ReadJSON(&m) != nil {
				return
			}
			switch {
			case m.Msg == "connect":
				_ = conn.WriteJSON(map[string]any{"msg": "connected"})
			case m.Method == "auth.login_with_api_key":
				_ = conn.WriteJSON(map[string]any{"msg": "result", "id": m.ID, "result": true})
			case errs[m.Method] != nil:
				_ = conn.WriteJSON(map[string]any{"msg": "result", "id": m.ID, "error": errs[m.Method]})
			default:
				res := results[m.Method]
				if res == "" {
					res = "[]"
				}
				_ = conn.WriteJSON(map[string]any{"msg": "result", "id": m.ID, "result": json.RawMessage(res)})
			}
		}
	}))
	t.Cleanup(srv.Close)
	c := NewClient(strings.TrimPrefix(srv.URL, "http://"), "k", false, false)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestListServicesAndSharesMissingMethod(t *testing.T) {
	// The reply as TrueNAS 25.10.7 sends it: a numeric errno plus its name.
	missing := map[string]any{"error": 32, "errname": "ENOMETHOD", "reason": "Method 'query' not found in 'sharing.nfs'"}
	c := scriptedClient(t,
		map[string]string{
			"service.query":     `[{"service":"cifs","enable":true,"state":"RUNNING"}]`,
			"sharing.smb.query": `[{"name":"media","path":"/mnt/tank/media","enabled":true}]`,
		},
		map[string]map[string]any{"sharing.nfs.query": missing})
	svc, err := ListServices(context.Background(), c)
	if err != nil || len(svc) != 1 {
		t.Fatalf("services: %v %v", svc, err)
	}
	shares, err := ListShares(context.Background(), c)
	if err != nil || len(shares) != 1 || shares[0].Service != "cifs" {
		t.Fatalf("a missing NFS method must leave the SMB shares: %v %v", shares, err)
	}

	c2 := scriptedClient(t, nil, map[string]map[string]any{"service.query": missing})
	if svc, err := ListServices(context.Background(), c2); err != nil || svc == nil || len(svc) != 0 {
		t.Fatalf("missing service.query: %#v %v", svc, err)
	}
}

func TestListSharesRealErrorIsAnError(t *testing.T) {
	c := scriptedClient(t, nil, map[string]map[string]any{"sharing.smb.query": {"error": 13, "errname": "EACCES", "reason": "denied"}})
	_, err := ListShares(context.Background(), c)
	if err == nil || errors.Is(err, ErrNoMethod) {
		t.Fatalf("want a real error, got %v", err)
	}
}

// A numeric "error" used to fail decoding, which dropped the connection and
// every call in flight with it. After an error reply the same client must keep
// working.
func TestNumericErrorCodeDoesNotDropTheConnection(t *testing.T) {
	c := scriptedClient(t,
		map[string]string{"service.query": `[{"service":"ssh","enable":true,"state":"RUNNING"}]`},
		map[string]map[string]any{"sharing.smb.query": {"error": 13, "errname": "EACCES", "reason": "denied"}})
	if _, err := ListShares(context.Background(), c); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("want the middleware's reason, got %v", err)
	}
	svc, err := ListServices(context.Background(), c)
	if err != nil || len(svc) != 1 {
		t.Fatalf("the connection must survive an error reply: %v %v", svc, err)
	}
}
