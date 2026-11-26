package truenas

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// UpdateInfo says whether TrueNAS has an update waiting and how many catalog
// apps can be upgraded. Shapes confirmed against a live TrueNAS SCALE 25.10.7
// box (`update.status` and `app.query`, with a Readonly Admin API key):
//
//	update.status -> {"code":"NORMAL","status":{"current_version":{...},
//	                  "new_version":null},"error":null,"update_download_progress":null}
//
// new_version is null when nothing newer exists; when one does, the TrueNAS
// API documents it as an object with version, manifest, release_notes and
// release_notes_url. That branch has not been seen on a real box yet, so it is
// read tolerantly: only the version and release notes link are used, and
// anything unexpected means "no update shown", never an error in the UI.
type UpdateInfo struct {
	Available       bool   `json:"available"`
	Version         string `json:"version,omitempty"`
	ReleaseNotesURL string `json:"release_notes_url,omitempty"`
	// Catalog apps with a newer version of the software. Chart-only updates
	// (same app version, new packaging) and Docker containers are not counted.
	AppsWithUpdates int `json:"apps_with_updates"`
	// When TrueNAS was last asked, unix seconds.
	CheckedAt int64 `json:"checked_at"`
}

type updateStatusResponse struct {
	Error  json.RawMessage `json:"error"`
	Status struct {
		NewVersion json.RawMessage `json:"new_version"`
	} `json:"status"`
}

// checkUpdates asks TrueNAS once. A failed update.status is an error (the
// caller keeps the previous answer); a failed app.query only leaves the app
// count at 0, since that is a side detail.
func checkUpdates(ctx context.Context, c *Client, now time.Time) (*UpdateInfo, error) {
	raw, err := c.Call(ctx, "update.status", []any{})
	if err != nil {
		return nil, fmt.Errorf("update.status: %w", err)
	}
	var st updateStatusResponse
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("update.status: decode: %w", err)
	}
	// error is null when the check worked. TrueNAS without internet access
	// reports the failure here instead of failing the call.
	if e := strings.TrimSpace(string(st.Error)); e != "" && e != "null" {
		return nil, fmt.Errorf("update.status: TrueNAS could not check for updates: %s", e)
	}

	info := &UpdateInfo{CheckedAt: now.Unix()}
	if nv := strings.TrimSpace(string(st.Status.NewVersion)); nv != "" && nv != "null" {
		var v struct {
			Version         string `json:"version"`
			ReleaseNotesURL string `json:"release_notes_url"`
		}
		// Tolerant: a shape that does not decode shows no update.
		if json.Unmarshal(st.Status.NewVersion, &v) == nil && v.Version != "" {
			info.Available = true
			info.Version = v.Version
			info.ReleaseNotesURL = safeURL(v.ReleaseNotesURL)
		}
	}
	info.AppsWithUpdates = appsWithUpdates(ctx, c)
	return info, nil
}

// safeURL keeps only http(s) links: the UI puts this in an href.
func safeURL(u string) string {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
		return u
	}
	return ""
}

// appsWithUpdates counts catalog apps that have a newer version of the
// software itself. TrueNAS also flags an app (upgrade_available) when only its
// catalog packaging changed -- same application version, newer chart -- which
// is noise on a dashboard, so those are left out; TrueNAS still lists them in
// its own UI. app.query gives the installed application version, app.latest
// the newest one in the catalog. When the catalog lookup fails, an app TrueNAS
// flags is counted rather than hidden.
func appsWithUpdates(ctx context.Context, c *Client) int {
	raw, err := c.Call(ctx, "app.query", []any{[]any{}, map[string]any{"select": []string{"name", "upgrade_available", "metadata.app_version", "metadata.train"}}})
	if err != nil {
		return 0 // no catalog apps, or not allowed: not worth an error
	}
	var apps []struct {
		Name     string `json:"name"`
		Upgrade  bool   `json:"upgrade_available"`
		Metadata struct {
			AppVersion string `json:"app_version"`
			Train      string `json:"train"`
		} `json:"metadata"`
	}
	if json.Unmarshal(raw, &apps) != nil {
		return 0
	}
	var flagged []string
	for _, a := range apps {
		if a.Upgrade {
			flagged = append(flagged, a.Name)
		}
	}
	if len(flagged) == 0 {
		return 0
	}

	latest := latestAppVersions(ctx, c, flagged)
	n := 0
	for _, a := range apps {
		if !a.Upgrade {
			continue
		}
		newest, known := latest[a.Name+"\x00"+a.Metadata.Train]
		if known && newest == a.Metadata.AppVersion {
			continue // chart-only update: same software
		}
		n++
	}
	return n
}

// latestAppVersions maps "name\x00train" to the newest application version in
// the catalog. Empty when the lookup fails.
func latestAppVersions(ctx context.Context, c *Client, names []string) map[string]string {
	out := map[string]string{}
	raw, err := c.Call(ctx, "app.latest", []any{[]any{[]any{"name", "in", names}}, map[string]any{"select": []string{"name", "train", "latest_app_version"}}})
	if err != nil {
		return out
	}
	var rows []struct {
		Name    string `json:"name"`
		Train   string `json:"train"`
		Version string `json:"latest_app_version"`
	}
	if json.Unmarshal(raw, &rows) != nil {
		return out
	}
	for _, r := range rows {
		if r.Version != "" {
			out[r.Name+"\x00"+r.Train] = r.Version
		}
	}
	return out
}

// UpdateChecker caches the update answer. update.status asks TrueNAS's update
// server, so it runs every few hours and once at start, not on every refresh,
// and in the background so a slow or unreachable update server never delays
// the refresh. A failed check (TrueNAS without internet) keeps the previous
// answer and is retried sooner; before the first success there is simply no
// answer.
type UpdateChecker struct {
	mu    sync.Mutex
	every time.Duration
	retry time.Duration
	next  time.Time
	last  *UpdateInfo
	busy  bool

	check func(ctx context.Context, now time.Time) (*UpdateInfo, error)
	now   func() time.Time
}

func NewUpdateChecker(c *Client) *UpdateChecker {
	return &UpdateChecker{
		every: 6 * time.Hour,
		retry: 15 * time.Minute,
		now:   time.Now,
		check: func(ctx context.Context, now time.Time) (*UpdateInfo, error) {
			return checkUpdates(ctx, c, now)
		},
	}
}

// Get returns the cached answer (nil until a check has succeeded) and starts a
// background check when one is due.
func (u *UpdateChecker) Get() *UpdateInfo {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.busy && !u.now().Before(u.next) {
		u.busy = true
		go u.run()
	}
	return u.last
}

func (u *UpdateChecker) run() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	now := u.now()
	info, err := u.check(ctx, now)

	u.mu.Lock()
	defer u.mu.Unlock()
	u.busy = false
	if err != nil {
		slog.Warn("update check failed; will retry", "err", err, "retry_in", u.retry.String())
		u.next = now.Add(u.retry)
		return
	}
	u.last = info
	u.next = now.Add(u.every)
}
