package truenas

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// Recorded from a live TrueNAS SCALE 25.10.7 box (up to date).
const updateStatusUpToDate = `{"code":"NORMAL","status":{"current_version":{"train":"TrueNAS-SCALE-Goldeye","profile":"MISSION_CRITICAL","matches_profile":true},"new_version":null},"error":null,"update_download_progress":null}`

// The documented shape of new_version (api.truenas.com update.status); not yet
// seen on a real box.
const updateStatusAvailable = `{"code":"NORMAL","status":{"current_version":{"train":"TrueNAS-SCALE-Goldeye"},"new_version":{"version":"25.10.8","manifest":{"changelog":"x"},"release_notes":"notes","release_notes_url":"https://www.truenas.com/docs/scale/25.10/gettingstarted/scalereleasenotes/"}},"error":null,"update_download_progress":null}`

// Apps as app.query returns them with the select NasWarden uses. naswarden has
// only a chart update (same app version); nextcloud and plex have real ones.
const appsJSON = `[
 {"name":"naswarden","upgrade_available":true,"metadata":{"app_version":"1.1.1","train":"community"}},
 {"name":"plex","upgrade_available":false,"metadata":{"app_version":"1.41","train":"community"}},
 {"name":"nextcloud","upgrade_available":true,"metadata":{"app_version":"30.0.1","train":"stable"}},
 {"name":"jellyfin","upgrade_available":true,"metadata":{"app_version":"10.9","train":"community"}}]`

const appsLatestJSON = `[
 {"name":"naswarden","train":"community","latest_app_version":"1.1.1"},
 {"name":"nextcloud","train":"stable","latest_app_version":"30.0.2"},
 {"name":"jellyfin","train":"community","latest_app_version":"10.10"}]`

func TestCheckUpdatesUpToDate(t *testing.T) {
	c := methodServer(t, map[string]string{"update.status": updateStatusUpToDate, "app.query": `[{"name":"a","upgrade_available":false}]`})
	info, err := checkUpdates(context.Background(), c, time.Unix(1000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if info.Available || info.Version != "" || info.AppsWithUpdates != 0 || info.CheckedAt != 1000 {
		t.Errorf("info = %+v, want nothing available", info)
	}
}

func TestCheckUpdatesAvailableAndApps(t *testing.T) {
	c := methodServer(t, map[string]string{"update.status": updateStatusAvailable, "app.query": appsJSON, "app.latest": appsLatestJSON})
	info, err := checkUpdates(context.Background(), c, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	// naswarden's update is chart-only (1.1.1 -> 1.1.1) and is not counted.
	if !info.Available || info.Version != "25.10.8" || info.AppsWithUpdates != 2 {
		t.Errorf("info = %+v, want 25.10.8 and 2 apps (nextcloud, jellyfin; not the chart-only naswarden)", info)
	}
	if info.ReleaseNotesURL == "" {
		t.Error("release notes link was dropped")
	}
}

func TestCheckUpdatesIsTolerantOfUnexpectedShapes(t *testing.T) {
	for name, status := range map[string]string{
		"version is not a string":    `{"status":{"new_version":{"version":25}},"error":null}`,
		"new_version is not object":  `{"status":{"new_version":"25.10.8"},"error":null}`,
		"new_version has no version": `{"status":{"new_version":{}},"error":null}`,
	} {
		c := methodServer(t, map[string]string{"update.status": status}) // app.query not scripted: fails
		info, err := checkUpdates(context.Background(), c, time.Now())
		if err != nil || info.Available || info.AppsWithUpdates != 0 {
			t.Errorf("%s: info=%+v err=%v, want no update and no error", name, info, err)
		}
	}
}

func TestCheckUpdatesDropsUnsafeReleaseNotesLink(t *testing.T) {
	c := methodServer(t, map[string]string{"update.status": `{"status":{"new_version":{"version":"1","release_notes_url":"javascript:alert(1)"}},"error":null}`})
	info, err := checkUpdates(context.Background(), c, time.Now())
	if err != nil || !info.Available || info.ReleaseNotesURL != "" {
		t.Errorf("info=%+v err=%v, want the update shown without the javascript: link", info, err)
	}
}

func TestCheckUpdatesReportsAFailedCheck(t *testing.T) {
	// TrueNAS without internet: the call works but reports the failure.
	c := methodServer(t, map[string]string{"update.status": `{"status":{"new_version":null},"error":"could not reach update server"}`})
	if _, err := checkUpdates(context.Background(), c, time.Now()); err == nil {
		t.Error("a failed check must be an error so the cached answer is kept")
	}
	// And a call that fails outright.
	c = methodServer(t, map[string]string{})
	if _, err := checkUpdates(context.Background(), c, time.Now()); err == nil {
		t.Error("update.status failure must be an error")
	}
}

func TestUpdateCheckerCachesAndKeepsLastGoodOnFailure(t *testing.T) {
	var calls atomic.Int32
	fail := false
	now := time.Unix(10_000, 0)
	u := &UpdateChecker{
		every: 6 * time.Hour, retry: 15 * time.Minute,
		now: func() time.Time { return now },
		check: func(_ context.Context, at time.Time) (*UpdateInfo, error) {
			calls.Add(1)
			if fail {
				return nil, errors.New("no internet")
			}
			return &UpdateInfo{Available: true, Version: "25.10.8", CheckedAt: at.Unix()}, nil
		},
	}
	wait := func() {
		t.Helper()
		for i := 0; i < 200; i++ {
			u.mu.Lock()
			busy := u.busy
			u.mu.Unlock()
			if !busy {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("check did not finish")
	}

	if u.Get() != nil {
		t.Error("before the first check there is no answer")
	}
	wait()
	if got := u.Get(); got == nil || got.Version != "25.10.8" {
		t.Fatalf("got %+v after the first check", got)
	}
	u.Get()
	wait()
	if calls.Load() != 1 {
		t.Errorf("calls = %d within the interval, want 1 (cached)", calls.Load())
	}

	now = now.Add(7 * time.Hour)
	fail = true
	u.Get()
	wait()
	if got := u.Get(); got == nil || got.Version != "25.10.8" {
		t.Errorf("a failed check lost the last good answer: %+v", got)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
	// Retried after 15 minutes, not 6 hours.
	now = now.Add(16 * time.Minute)
	fail = false
	u.Get()
	wait()
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want a retry after the short interval", calls.Load())
	}
}

func TestAppsWithUpdatesIgnoresChartOnlyUpdates(t *testing.T) {
	// The case from a real box: TrueNAS flags naswarden (chart 1.0.2 -> 1.0.3)
	// but the application stays at 1.1.1.
	c := methodServer(t, map[string]string{
		"app.query":  `[{"name":"naswarden","upgrade_available":true,"metadata":{"app_version":"1.1.1","train":"community"}}]`,
		"app.latest": `[{"name":"naswarden","train":"community","latest_app_version":"1.1.1"}]`,
	})
	if n := appsWithUpdates(context.Background(), c); n != 0 {
		t.Errorf("apps with updates = %d, want 0 for a chart-only update", n)
	}
}

func TestAppsWithUpdatesCountsFlaggedAppsWhenTheCatalogLookupFails(t *testing.T) {
	// app.latest not scripted: fall back to what TrueNAS itself says.
	c := methodServer(t, map[string]string{
		"app.query": `[{"name":"naswarden","upgrade_available":true,"metadata":{"app_version":"1.1.1","train":"community"}},{"name":"plex","upgrade_available":false,"metadata":{"app_version":"1","train":"community"}}]`,
	})
	if n := appsWithUpdates(context.Background(), c); n != 1 {
		t.Errorf("apps with updates = %d, want 1 (flagged, catalog unknown)", n)
	}
}

func TestAppsWithUpdatesIsZeroWhenNothingIsFlagged(t *testing.T) {
	// No app.latest call is needed (and none is scripted) when nothing is flagged.
	c := methodServer(t, map[string]string{
		"app.query": `[{"name":"plex","upgrade_available":false,"metadata":{"app_version":"1","train":"community"}}]`,
	})
	if n := appsWithUpdates(context.Background(), c); n != 0 {
		t.Errorf("apps with updates = %d, want 0", n)
	}
}
