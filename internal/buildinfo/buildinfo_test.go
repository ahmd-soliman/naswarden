package buildinfo

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestHandlerServesTheStampedBuild(t *testing.T) {
	old := Current()
	t.Cleanup(func() { Version, Commit, Date = old.Version, old.Commit, old.Date })
	Version, Commit, Date = "1.2.3", "abc1234", "2026-09-19T12:00:00Z"

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/version", nil))

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q", ct)
	}
	var got Info
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got != (Info{Version: "1.2.3", Commit: "abc1234", Date: "2026-09-19T12:00:00Z"}) {
		t.Errorf("got %+v", got)
	}
}

func TestDefaultsIdentifyAnUnstampedBuild(t *testing.T) {
	if Version != "dev" {
		t.Skip("stamped by -ldflags")
	}
	if Current().Commit != "unknown" {
		t.Errorf("commit = %q, want unknown", Current().Commit)
	}
}

func TestRegisterPublishesBuildInfoMetric(t *testing.T) {
	Register()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != "naswarden_build_info" {
			continue
		}
		m := f.GetMetric()
		if len(m) != 1 || m[0].GetGauge().GetValue() != 1 {
			t.Fatalf("build_info = %v, want a single series with value 1", m)
		}
		labels := map[string]string{}
		for _, l := range m[0].GetLabel() {
			labels[l.GetName()] = l.GetValue()
		}
		if labels["version"] != Version || labels["commit"] != Commit {
			t.Errorf("labels = %v", labels)
		}
		return
	}
	t.Fatal("naswarden_build_info not exported")
}
