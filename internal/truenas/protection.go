package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Kinds of data protection task. They double as the stale source names of the
// four middleware calls, so each can fail (or not exist) on its own.
const (
	KindSnapshot    = "snapshot"
	KindCloudSync   = "cloudsync"
	KindRsync       = "rsync"
	KindCloudBackup = "cloud_backup"
)

// Task states, computed here so the UI and any other consumer agree.
const (
	TaskOK       = "OK"
	TaskRunning  = "RUNNING"
	TaskFailed   = "FAILED"
	TaskNever    = "NEVER"    // enabled, but no run recorded yet
	TaskDisabled = "DISABLED" // switched off in TrueNAS
	TaskUnknown  = "UNKNOWN"  // a state this version does not know
)

// overdueGrace is how long after a scheduled time a run may still be missing
// before the task counts as overdue: jobs queue behind each other.
const overdueGrace = 15 * time.Minute

// maxErrorLen bounds the error text sent to the browser; tracebacks can be long.
const maxErrorLen = 2000

// ProtectionTask is one periodic snapshot, Cloud Sync, Rsync or TrueCloud
// backup task. Names, paths and remotes say a lot about the system, so like
// shares they only go to the state message, never to metrics or logs.
type ProtectionTask struct {
	Kind      string   `json:"kind"`
	ID        int64    `json:"id"`
	Name      string   `json:"name"`   // description; the dataset for a snapshot task
	Source    string   `json:"source"` // dataset or local path covered
	Remote    string   `json:"remote,omitempty"`
	Direction string   `json:"direction,omitempty"`
	Schedule  string   `json:"schedule,omitempty"` // cron fields "m h dom mon dow"
	Window    string   `json:"window,omitempty"`   // snapshot tasks only, when not all day: "08:00-18:00"
	Retention string   `json:"retention,omitempty"`
	Recursive bool     `json:"recursive,omitempty"`
	Exclude   []string `json:"exclude"`
	Enabled   bool     `json:"enabled"`
	State     string   `json:"state"`
	// Overdue: enabled, last run fine, but a scheduled run since then did not
	// happen (see overdue). Never set for a task that has not run or failed.
	Overdue             bool     `json:"overdue"`
	ProgressPercent     *float64 `json:"progress_percent,omitempty"`
	ProgressDescription string   `json:"progress_description,omitempty"`
	Error               string   `json:"error,omitempty"`
	LastSnapshot        string   `json:"last_snapshot,omitempty"`
	TimeStarted         *int64   `json:"time_started,omitempty"`
	TimeFinished        *int64   `json:"time_finished,omitempty"`
	DurationSeconds     *int64   `json:"duration_seconds,omitempty"`
}

type rawSchedule struct {
	Minute string `json:"minute"`
	Hour   string `json:"hour"`
	Dom    string `json:"dom"`
	Month  string `json:"month"`
	Dow    string `json:"dow"`
	Begin  string `json:"begin"`
	End    string `json:"end"`
}

// rawJob is the last job of a Cloud Sync, Rsync or backup task. Only these
// fields are decoded: the job also carries a log excerpt with file names.
type rawJob struct {
	State        string          `json:"state"`
	Error        string          `json:"error"`
	TimeStarted  json.RawMessage `json:"time_started"`
	TimeFinished json.RawMessage `json:"time_finished"`
	Progress     *struct {
		Percent     *float64 `json:"percent"`
		Description string   `json:"description"`
	} `json:"progress"`
}

type rawSnapshotTask struct {
	ID            int64       `json:"id"`
	Dataset       string      `json:"dataset"`
	Recursive     bool        `json:"recursive"`
	Exclude       []string    `json:"exclude"`
	LifetimeValue int         `json:"lifetime_value"`
	LifetimeUnit  string      `json:"lifetime_unit"`
	Enabled       bool        `json:"enabled"`
	Schedule      rawSchedule `json:"schedule"`
	// state is an object from zettarepl, or a bare string when zettarepl
	// itself is in error.
	State json.RawMessage `json:"state"`
}

type rawCloudTask struct {
	ID          int64       `json:"id"`
	Description string      `json:"description"`
	Path        string      `json:"path"`
	Direction   string      `json:"direction"`
	Enabled     bool        `json:"enabled"`
	Schedule    rawSchedule `json:"schedule"`
	KeepLast    int         `json:"keep_last"`
	Exclude     []string    `json:"exclude"`
	// Only the provider type and folder are read; credentials also hold the
	// provider's secrets and tokens, which must never be decoded.
	Credentials struct {
		Provider struct {
			Type string `json:"type"`
		} `json:"provider"`
	} `json:"credentials"`
	Attributes struct {
		Folder string `json:"folder"`
	} `json:"attributes"`
	Locked bool    `json:"locked"`
	Job    *rawJob `json:"job"`
}

type rawRsyncTask struct {
	ID           int64       `json:"id"`
	Desc         string      `json:"desc"`
	Path         string      `json:"path"`
	Mode         string      `json:"mode"`
	RemoteHost   string      `json:"remotehost"`
	RemoteModule string      `json:"remotemodule"`
	RemotePath   string      `json:"remotepath"`
	Direction    string      `json:"direction"`
	Recursive    bool        `json:"recursive"`
	Enabled      bool        `json:"enabled"`
	Schedule     rawSchedule `json:"schedule"`
	Locked       bool        `json:"locked"`
	Job          *rawJob     `json:"job"`
}

// ListProtectionTasks queries one task kind. A TrueNAS without the method
// yields an empty list, not an error. loc is the NAS's time zone (cron times
// are local to it); nil disables the overdue check.
func ListProtectionTasks(ctx context.Context, c *Client, kind string, loc *time.Location, now time.Time) ([]ProtectionTask, error) {
	method := map[string]string{
		KindSnapshot:    "pool.snapshottask.query",
		KindCloudSync:   "cloudsync.query",
		KindRsync:       "rsynctask.query",
		KindCloudBackup: "cloud_backup.query",
	}[kind]
	if method == "" {
		return nil, fmt.Errorf("unknown task kind %q", kind)
	}
	raw, err := c.Call(ctx, method, []any{})
	if errors.Is(err, ErrNoMethod) {
		return []ProtectionTask{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	tasks, err := decodeProtection(kind, raw, loc, now)
	if err != nil {
		return nil, fmt.Errorf("%s: decode: %w", method, err)
	}
	return tasks, nil
}

func decodeProtection(kind string, raw json.RawMessage, loc *time.Location, now time.Time) ([]ProtectionTask, error) {
	out := []ProtectionTask{}
	switch kind {
	case KindSnapshot:
		var list []rawSnapshotTask
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		for _, r := range list {
			out = append(out, snapshotTask(r, loc, now))
		}
	case KindCloudSync, KindCloudBackup:
		var list []rawCloudTask
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		for _, r := range list {
			out = append(out, cloudTask(kind, r, loc, now))
		}
	case KindRsync:
		var list []rawRsyncTask
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		for _, r := range list {
			out = append(out, rsyncTask(r, loc, now))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func snapshotTask(r rawSnapshotTask, loc *time.Location, now time.Time) ProtectionTask {
	t := ProtectionTask{
		Kind: KindSnapshot, ID: r.ID, Name: r.Dataset, Source: r.Dataset,
		Recursive: r.Recursive, Exclude: nonNil(r.Exclude), Enabled: r.Enabled,
		Schedule: cronString(r.Schedule), Window: window(r.Schedule),
	}
	if r.LifetimeValue > 0 && r.LifetimeUnit != "" {
		t.Retention = strconv.Itoa(r.LifetimeValue) + " " + r.LifetimeUnit
	}
	var st struct {
		State        string          `json:"state"`
		Datetime     json.RawMessage `json:"datetime"`
		LastSnapshot string          `json:"last_snapshot"`
		Error        string          `json:"error"`
	}
	var bare string
	switch {
	case json.Unmarshal(r.State, &st) == nil && st.State != "":
	case json.Unmarshal(r.State, &bare) == nil && bare != "":
		st.State, st.Error = "ERROR", bare
	}
	t.LastSnapshot = st.LastSnapshot
	t.TimeFinished = parseTrueNASTime(st.Datetime)
	t.Error = truncate(st.Error)
	switch {
	case !r.Enabled:
		t.State = TaskDisabled
	case st.State == "FINISHED":
		t.State = TaskOK
	case st.State == "RUNNING":
		t.State = TaskRunning
	case st.State == "ERROR":
		t.State = TaskFailed
	case st.State == "PENDING" || st.State == "":
		// Also what TrueNAS reports for a task zettarepl has not run yet.
		t.State = TaskNever
	default:
		t.State = TaskUnknown
	}
	if t.State == TaskOK {
		t.Overdue = overdue(r.Schedule, loc, now, t.TimeFinished)
	}
	return t
}

func cloudTask(kind string, r rawCloudTask, loc *time.Location, now time.Time) ProtectionTask {
	name := r.Description
	if name == "" {
		name = r.Path
	}
	t := ProtectionTask{
		Kind: kind, ID: r.ID, Name: name, Source: r.Path, Direction: r.Direction,
		Remote:  strings.TrimSpace(r.Credentials.Provider.Type + " " + r.Attributes.Folder),
		Exclude: nonNil(r.Exclude), Enabled: r.Enabled, Schedule: cronString(r.Schedule),
	}
	if kind == KindCloudBackup && r.KeepLast > 0 {
		t.Retention = "last " + strconv.Itoa(r.KeepLast)
	}
	applyJob(&t, r.Job, r.Locked, r.Schedule, loc, now)
	return t
}

func rsyncTask(r rawRsyncTask, loc *time.Location, now time.Time) ProtectionTask {
	name := r.Desc
	if name == "" {
		name = r.Path
	}
	remote := r.RemoteHost
	switch {
	case r.Mode == "MODULE" && r.RemoteModule != "":
		remote += "::" + r.RemoteModule
	case r.RemotePath != "":
		remote += ":" + r.RemotePath
	}
	t := ProtectionTask{
		Kind: KindRsync, ID: r.ID, Name: name, Source: r.Path, Remote: remote,
		Direction: r.Direction, Recursive: r.Recursive, Exclude: []string{},
		Enabled: r.Enabled, Schedule: cronString(r.Schedule),
	}
	applyJob(&t, r.Job, r.Locked, r.Schedule, loc, now)
	return t
}

// applyJob sets the state and timing fields from a task's last job. locked is
// TrueNAS's own "a run is in progress" flag, trusted when the job says nothing.
func applyJob(t *ProtectionTask, j *rawJob, locked bool, sch rawSchedule, loc *time.Location, now time.Time) {
	if j != nil {
		t.TimeStarted = parseTrueNASTime(j.TimeStarted)
		t.TimeFinished = parseTrueNASTime(j.TimeFinished)
		if t.TimeStarted != nil && t.TimeFinished != nil && *t.TimeFinished >= *t.TimeStarted {
			d := *t.TimeFinished - *t.TimeStarted
			t.DurationSeconds = &d
		}
		if j.Progress != nil {
			t.ProgressPercent = j.Progress.Percent
			t.ProgressDescription = j.Progress.Description
		}
		t.Error = truncate(j.Error)
	}
	switch {
	case !t.Enabled:
		t.State = TaskDisabled
	case j == nil && locked:
		t.State = TaskRunning
	case j == nil:
		t.State = TaskNever
	case j.State == "RUNNING" || j.State == "WAITING":
		t.State = TaskRunning
	case j.State == "SUCCESS":
		t.State = TaskOK
	case j.State == "FAILED" || j.State == "ABORTED":
		t.State = TaskFailed
		if t.Error == "" && j.State == "ABORTED" {
			t.Error = "Aborted"
		}
	default:
		t.State = TaskUnknown
	}
	if t.State != TaskRunning {
		// Progress of a finished run is just noise next to the result.
		t.ProgressPercent, t.ProgressDescription = nil, ""
	}
	if t.State == TaskOK {
		t.Overdue = overdue(sch, loc, now, t.TimeStarted)
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxErrorLen {
		return strings.ToValidUTF8(s[:maxErrorLen], "") + "…"
	}
	return s
}

func cronString(s rawSchedule) string {
	if s.Minute == "" && s.Hour == "" {
		return ""
	}
	return strings.Join([]string{s.Minute, s.Hour, s.Dom, s.Month, s.Dow}, " ")
}

func window(s rawSchedule) string {
	if (s.Begin == "" && s.End == "") || (s.Begin == "00:00" && s.End == "23:59") {
		return ""
	}
	return s.Begin + "-" + s.End
}

// overdue reports whether the newest scheduled time that is more than
// overdueGrace in the past has no run after it. lastRun is when the last run
// started (snapshots: finished); a run counts for a slot if it began no
// earlier than a minute before it. Needs the NAS time zone and a schedule it
// can read; otherwise it says false (unknown), never a guess.
func overdue(s rawSchedule, loc *time.Location, now time.Time, lastRun *int64) bool {
	if loc == nil || lastRun == nil {
		return false
	}
	slot, ok := lastSlot(s, loc, now.Add(-overdueGrace))
	if !ok {
		return false
	}
	return time.Unix(*lastRun, 0).Add(time.Minute).Before(slot)
}

// lastSlot finds the latest time at or before t that the cron schedule fires.
// It walks back day by day (at most 400), so even a yearly schedule is cheap.
func lastSlot(s rawSchedule, loc *time.Location, t time.Time) (time.Time, bool) {
	minutes, ok1 := parseField(s.Minute, 0, 59, nil)
	hours, ok2 := parseField(s.Hour, 0, 23, nil)
	doms, ok3 := parseField(s.Dom, 1, 31, nil)
	months, ok4 := parseField(s.Month, 1, 12, monthNames)
	dows, ok5 := parseField(s.Dow, 0, 7, dowNames)
	if !(ok1 && ok2 && ok3 && ok4 && ok5) {
		return time.Time{}, false
	}
	if dows[7] {
		dows[0] = true // 7 is Sunday too
	}
	lo, hi := 0, 24*60-1
	if s.Begin != "" && s.End != "" {
		b, bok := clock(s.Begin)
		e, eok := clock(s.End)
		if !bok || !eok {
			return time.Time{}, false
		}
		lo, hi = b, e
	}
	// cron: when both day fields are restricted, either one matching is enough.
	domStar, dowStar := strings.HasPrefix(s.Dom, "*"), strings.HasPrefix(s.Dow, "*")
	local := t.In(loc)
	for back := 0; back <= 400; back++ {
		day := time.Date(local.Year(), local.Month(), local.Day()-back, 0, 0, 0, 0, loc)
		if !months[int(day.Month())] {
			continue
		}
		domOK, dowOK := doms[day.Day()], dows[int(day.Weekday())]
		dayMatches := domOK && dowOK
		if !domStar && !dowStar {
			dayMatches = domOK || dowOK
		}
		if !dayMatches {
			continue
		}
		for h := 23; h >= 0; h-- {
			if !hours[h] {
				continue
			}
			for m := 59; m >= 0; m-- {
				if !minutes[m] || h*60+m < lo || h*60+m > hi {
					continue
				}
				slot := time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, loc)
				if !slot.After(t) {
					return slot, true
				}
			}
		}
	}
	return time.Time{}, false
}

func clock(s string) (int, bool) {
	h, m, ok := strings.Cut(s, ":")
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, false
	}
	return hh*60 + mm, true
}

var monthNames = map[string]int{"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6, "jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12}
var dowNames = map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}

// parseField reads one cron field: "*", "*/n", "a", "a-b", "a-b/n", or a
// comma list of those. ok is false for anything else.
func parseField(f string, min, max int, names map[string]int) (set map[int]bool, ok bool) {
	set = map[int]bool{}
	if f == "" {
		return nil, false
	}
	num := func(s string) (int, bool) {
		if v, found := names[strings.ToLower(s)]; found {
			return v, true
		}
		v, err := strconv.Atoi(s)
		return v, err == nil && v >= min && v <= max
	}
	for _, part := range strings.Split(f, ",") {
		rng, stepS, hasStep := strings.Cut(part, "/")
		step := 1
		if hasStep {
			v, err := strconv.Atoi(stepS)
			if err != nil || v < 1 {
				return nil, false
			}
			step = v
		}
		lo, hi := min, max
		if rng != "*" {
			a, b, isRange := strings.Cut(rng, "-")
			var aok bool
			if lo, aok = num(a); !aok {
				return nil, false
			}
			hi = lo
			if isRange {
				var bok bool
				if hi, bok = num(b); !bok || hi < lo {
					return nil, false
				}
			} else if hasStep {
				hi = max // "5/10" means 5 through the end
			}
		}
		for v := lo; v <= hi; v += step {
			set[v] = true
		}
	}
	return set, true
}
