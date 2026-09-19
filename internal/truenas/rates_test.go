package truenas

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// methodServer answers every middleware method with the given result (login
// always succeeds), or with an error when result is empty.
func methodServer(t *testing.T, results map[string]string) *Client {
	t.Helper()
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var m ddpMessage
			if conn.ReadJSON(&m) != nil {
				return
			}
			switch m.Msg {
			case "connect":
				_ = conn.WriteJSON(ddpMessage{Msg: "connected"})
			case "method":
				res, ok := results[m.Method]
				switch {
				case m.Method == "auth.login_with_api_key":
					_ = conn.WriteJSON(ddpMessage{Msg: "result", ID: m.ID, Result: json.RawMessage(`true`)})
				case !ok:
					_ = conn.WriteJSON(ddpMessage{Msg: "result", ID: m.ID, Error: &ddpError{Error: "EFAULT", Reason: "not scripted"}})
				default:
					_ = conn.WriteJSON(ddpMessage{Msg: "result", ID: m.ID, Result: json.RawMessage(res)})
				}
			}
		}
	}))
	t.Cleanup(srv.Close)
	c := NewClient(strings.TrimPrefix(srv.URL, "http://"), "key", false, false)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func ifaces() *ServerInfo {
	return &ServerInfo{Interfaces: []Interface{
		{Name: "eno1", Type: "PHYSICAL"},
		{Name: "br0", Type: "BRIDGE"},
	}}
}

// graph builds a netdata "interface" graph with a constant rx/tx.
func graph(rx, tx string) string {
	return `{"name":"interface","legend":["time","received","sent"],"data":[[1,` + rx + `,` + tx + `],[2,` + rx + `,` + tx + `]]}`
}

func TestAddInterfaceRatesTotalsOnlyPhysicalLinks(t *testing.T) {
	c := methodServer(t, map[string]string{
		"reporting.netdata_get_data": `[` + graph("100", "40") + `,` + graph("999", "999") + `]`,
	})
	info := ifaces()
	addInterfaceRates(context.Background(), c, info)

	if r := info.Interfaces[0].RxKbps; r == nil || *r != 100 {
		t.Errorf("eno1 rx = %v, want 100", r)
	}
	if r := info.Interfaces[1].TxKbps; r == nil || *r != 999 {
		t.Errorf("br0 tx = %v, want 999 (bridges still get their own rate)", r)
	}
	// The bridge carries the same traffic as its physical port, so it must
	// not be added to the total or the host would show double.
	if info.NetRxKbps == nil || *info.NetRxKbps != 100 || *info.NetTxKbps != 40 {
		t.Errorf("total = %v/%v, want 100/40", info.NetRxKbps, info.NetTxKbps)
	}
}

func TestAddInterfaceRatesIsBestEffort(t *testing.T) {
	for name, results := range map[string]map[string]string{
		"call fails":        {},
		"unparseable":       {"reporting.netdata_get_data": `"nope"`},
		"wrong graph count": {"reporting.netdata_get_data": `[` + graph("1", "1") + `]`},
	} {
		t.Run(name, func(t *testing.T) {
			info := ifaces()
			addInterfaceRates(context.Background(), methodServer(t, results), info)
			if info.NetRxKbps != nil || info.Interfaces[0].RxKbps != nil {
				t.Errorf("rates set despite %s: %+v", name, info)
			}
		})
	}
}

func TestAddInterfaceRatesWithNoInterfacesMakesNoCall(t *testing.T) {
	// No script: any call would error, and there must be none to fail.
	info := &ServerInfo{}
	addInterfaceRates(context.Background(), methodServer(t, nil), info)
	if info.NetRxKbps != nil {
		t.Errorf("total = %v, want nil", info.NetRxKbps)
	}
}

func TestLatestRow(t *testing.T) {
	if latestRow(reportingGraph{}) != nil {
		t.Error("no data must give nil")
	}
	g := reportingGraph{Data: [][]json.Number{{"1"}, {"2"}}}
	if r := latestRow(g); len(r) != 1 || r[0] != "2" {
		t.Errorf("latestRow = %v, want the last row", r)
	}
}

func TestListCallsReturnNamedErrors(t *testing.T) {
	c := methodServer(t, nil) // every call errors
	ctx := context.Background()
	checks := map[string]func() error{
		"pool.query":         func() error { _, err := ListPools(ctx, c); return err },
		"pool.dataset.query": func() error { _, err := ListDatasets(ctx, c); return err },
		"alert.list":         func() error { _, err := ListAlerts(ctx, c); return err },
		"replication.query":  func() error { _, err := ListReplications(ctx, c); return err },
		"disk.details":       func() error { _, err := ListDisks(ctx, c, NewDiskRates()); return err },
		"system.info":        func() error { _, err := GetServerInfo(ctx, c); return err },
	}
	for method, call := range checks {
		if err := call(); err == nil || !strings.Contains(err.Error(), method) {
			t.Errorf("%s: err = %v, want one that names the call", method, err)
		}
	}
}
