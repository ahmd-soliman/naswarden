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

func decodeServices(t *testing.T, fixture string) []Service {
	t.Helper()
	var raw []rawService
	if err := json.Unmarshal([]byte(fixture), &raw); err != nil {
		t.Fatal(err)
	}
	return buildServices(raw)
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

func TestServicesEmptyIsNotNil(t *testing.T) {
	if got := decodeServices(t, `[]`); got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want an empty non-nil list", got)
	}
}

func TestBuildShares(t *testing.T) {
	smb := []rawShare{
		{Name: "media", Path: "/mnt/tank/media", Enabled: ptrBool(true), RO: true},
		{Name: "old", Path: "/mnt/tank/old", Enabled: ptrBool(false)},
		{Name: "nopath"}, // missing fields tolerated
	}
	got := buildShares("cifs", smb)
	if len(got) != 3 || !got[0].ReadOnly || got[1].Enabled || !got[2].Enabled || got[2].Path != "" {
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
	missing := map[string]any{"error": "ENOMETHOD", "reason": `Method "sharing.nfs.query" not found`}
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
	c := scriptedClient(t, nil, map[string]map[string]any{"sharing.smb.query": {"error": "EACCES", "reason": "denied"}})
	_, err := ListShares(context.Background(), c)
	if err == nil || errors.Is(err, ErrNoMethod) {
		t.Fatalf("want a real error, got %v", err)
	}
}
