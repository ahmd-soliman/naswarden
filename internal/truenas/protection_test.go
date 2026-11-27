package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// Shapes recorded from TrueNAS 25.10.7 (pool.snapshottask.query and
// cloudsync.query), with dataset, path, description and remote names replaced
// and credentials removed. Timestamps are the recorded ones. Rsync and
// TrueCloud backup had no tasks on that box: their fixtures follow the
// middleware's API schema, and their failed/running jobs the job object seen
// on the other kinds.
var (
	berlin = mustZone("Europe/Berlin")
	// 2026-11-27 15:49 Berlin: the box's own clock when the fixtures were taken.
	fixtureNow = time.Unix(1795790941, 0)
)

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

const snapshotFixture = `[
 {"dataset":"tank/data","recursive":true,"lifetime_value":2,"lifetime_unit":"WEEK","enabled":true,"exclude":["tank/data/cache"],
  "naming_schema":"auto-%Y-%m-%d_%H-%M","allow_empty":true,
  "schedule":{"minute":"0","hour":"0","dom":"*","month":"*","dow":"*","begin":"00:00","end":"23:59"},"id":13,
  "state":{"state":"FINISHED","datetime":{"$date":1795734004000},"last_snapshot":"tank/data@auto-2026-11-27_00-00"},"vmware_sync":false},
 {"dataset":"tank/old","recursive":false,"lifetime_value":1,"lifetime_unit":"MONTH","enabled":false,"exclude":[],
  "schedule":{"minute":"0","hour":"0","dom":"*","month":"*","dow":"*","begin":"00:00","end":"23:59"},"id":14,
  "state":{"state":"FINISHED","datetime":{"$date":1795734004000},"last_snapshot":"tank/old@auto-2026-11-27_00-00"}},
 {"dataset":"tank/new","recursive":false,"lifetime_value":1,"lifetime_unit":"DAY","enabled":true,"exclude":[],
  "schedule":{"minute":"0","hour":"0","dom":"*","month":"*","dow":"*","begin":"00:00","end":"23:59"},"id":15,
  "state":{"state":"PENDING"}},
 {"dataset":"tank/broken","recursive":false,"lifetime_value":1,"lifetime_unit":"DAY","enabled":true,"exclude":[],
  "schedule":{"minute":"0","hour":"*/6","dom":"*","month":"*","dow":"*","begin":"08:00","end":"18:00"},"id":16,
  "state":{"state":"ERROR","datetime":{"$date":1795734004000},"error":"Dataset 'tank/broken' does not exist."}}
]`

func decode(t *testing.T, kind, fixture string) map[string]ProtectionTask {
	t.Helper()
	list, err := decodeProtection(kind, json.RawMessage(fixture), berlin, fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]ProtectionTask{}
	for _, task := range list {
		byName[task.Name] = task
	}
	return byName
}

func TestSnapshotTaskStates(t *testing.T) {
	got := decode(t, KindSnapshot, snapshotFixture)

	ok := got["tank/data"]
	if ok.State != TaskOK || ok.Overdue || ok.Retention != "2 WEEK" || ok.Schedule != "0 0 * * *" || !ok.Recursive ||
		ok.LastSnapshot != "tank/data@auto-2026-11-27_00-00" || ok.TimeFinished == nil || *ok.TimeFinished != 1795734004 ||
		len(ok.Exclude) != 1 || ok.Window != "" {
		t.Errorf("success: %+v", ok)
	}
	if d := got["tank/old"]; d.State != TaskDisabled || d.Enabled {
		t.Errorf("disabled: %+v", d)
	}
	if n := got["tank/new"]; n.State != TaskNever || n.TimeFinished != nil || n.Overdue {
		t.Errorf("never ran: %+v", n)
	}
	f := got["tank/broken"]
	if f.State != TaskFailed || f.Error != "Dataset 'tank/broken' does not exist." || f.Window != "08:00-18:00" {
		t.Errorf("failed: %+v", f)
	}
}

func TestSnapshotStateAsBareStringIsFailed(t *testing.T) {
	// When zettarepl itself is in error the middleware puts its message in
	// place of the state object.
	got := decode(t, KindSnapshot, `[{"id":1,"dataset":"tank/x","enabled":true,"state":"zettarepl is not running"}]`)
	if x := got["tank/x"]; x.State != TaskFailed || x.Error != "zettarepl is not running" {
		t.Errorf("%+v", x)
	}
}

const cloudFixture = `[
 {"id":1,"description":"Drive mirror","path":"/mnt/tank/mirror",
  "credentials":{"id":1,"name":"cred","provider":{"type":"GOOGLE_DRIVE","client_secret":"SECRET-VALUE","token":"TOKEN-VALUE"}},
  "attributes":{"folder":"backup"},
  "schedule":{"minute":"0","hour":"2","dom":"*","month":"*","dow":"*"},
  "enabled":true,"locked":false,"direction":"PULL","transfer_mode":"SYNC",
  "job":{"id":56403,"method":"cloudsync.sync","logs_excerpt":"NOTICE: private-file-name.pdf: Duplicate object","progress":{"percent":100,"description":"2 / 2, 12.458 KiB / 12.458 KiB, 0 B/s, ETA -","extra":null},
         "error":null,"exception":null,"state":"SUCCESS","time_started":{"$date":1795741201000},"time_finished":{"$date":1795741249000}}},
 {"id":2,"description":"Running","path":"/mnt/tank/a","credentials":{"provider":{"type":"S3"}},"attributes":{},
  "schedule":{"minute":"0","hour":"2","dom":"*","month":"*","dow":"*"},"enabled":true,"locked":true,"direction":"PUSH",
  "job":{"state":"RUNNING","progress":{"percent":42.5,"description":"Checking"},"time_started":{"$date":1795790000000},"time_finished":null}},
 {"id":3,"description":"Failed","path":"/mnt/tank/b","credentials":{"provider":{"type":"S3"}},"attributes":{},
  "schedule":{"minute":"0","hour":"2","dom":"*","month":"*","dow":"*"},"enabled":true,"locked":false,"direction":"PUSH",
  "job":{"state":"FAILED","error":"[EFAULT] rclone failed: couldn't login","progress":{"percent":10,"description":"x"},
         "time_started":{"$date":1795741201000},"time_finished":{"$date":1795741260000}}},
 {"id":4,"description":"Never","path":"/mnt/tank/c","credentials":{"provider":{"type":"S3"}},"attributes":{},
  "schedule":{"minute":"0","hour":"2","dom":"*","month":"*","dow":"*"},"enabled":true,"locked":false,"direction":"PUSH","job":null},
 {"id":5,"description":"Off","path":"/mnt/tank/d","credentials":{"provider":{"type":"S3"}},"attributes":{},
  "schedule":{"minute":"0","hour":"2","dom":"*","month":"*","dow":"*"},"enabled":false,"locked":false,"direction":"PUSH","job":null},
 {"id":6,"description":"Missed","path":"/mnt/tank/e","credentials":{"provider":{"type":"S3"}},"attributes":{},
  "schedule":{"minute":"0","hour":"2","dom":"*","month":"*","dow":"*"},"enabled":true,"locked":false,"direction":"PUSH",
  "job":{"state":"SUCCESS","time_started":{"$date":1795654801000},"time_finished":{"$date":1795654900000}}}
]`

func TestCloudSyncTaskStates(t *testing.T) {
	got := decode(t, KindCloudSync, cloudFixture)

	ok := got["Drive mirror"]
	if ok.State != TaskOK || ok.Overdue || ok.Direction != "PULL" || ok.Remote != "GOOGLE_DRIVE backup" || ok.Source != "/mnt/tank/mirror" ||
		ok.DurationSeconds == nil || *ok.DurationSeconds != 48 || ok.ProgressPercent != nil || ok.Error != "" {
		t.Errorf("success: %+v", ok)
	}
	if r := got["Running"]; r.State != TaskRunning || r.ProgressPercent == nil || *r.ProgressPercent != 42.5 || r.ProgressDescription != "Checking" || r.Overdue {
		t.Errorf("running: %+v", r)
	}
	if f := got["Failed"]; f.State != TaskFailed || f.Error != "[EFAULT] rclone failed: couldn't login" || f.Overdue {
		t.Errorf("failed: %+v", f)
	}
	if n := got["Never"]; n.State != TaskNever || n.TimeStarted != nil {
		t.Errorf("never ran: %+v", n)
	}
	if d := got["Off"]; d.State != TaskDisabled {
		t.Errorf("disabled: %+v", d)
	}
	// Last ran on the 25th; the 02:00 runs of the 26th and 27th are missing.
	if m := got["Missed"]; m.State != TaskOK || !m.Overdue {
		t.Errorf("overdue: %+v", m)
	}
}

func TestProtectionNeverCarriesSecretsOrLogs(t *testing.T) {
	list, err := decodeProtection(KindCloudSync, json.RawMessage(cloudFixture), berlin, fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(list)
	for _, leak := range []string{"SECRET-VALUE", "TOKEN-VALUE", "private-file-name", "client_secret"} {
		if strings.Contains(string(out), leak) {
			t.Errorf("state message contains %q", leak)
		}
	}
}

func TestRsyncAndCloudBackupTasks(t *testing.T) {
	rs := decode(t, KindRsync, `[
	 {"id":1,"desc":"Offsite","path":"/mnt/tank/data","mode":"SSH","remotehost":"backup.example","remotepath":"/srv/in","direction":"PUSH",
	  "recursive":true,"enabled":true,"locked":false,"schedule":{"minute":"30","hour":"3","dom":"*","month":"*","dow":"mon,thu"},
	  "job":{"state":"FAILED","error":"rsync error: timeout","time_started":{"$date":1795741201000},"time_finished":{"$date":1795741260000}}},
	 {"id":2,"desc":"","path":"/mnt/tank/m","mode":"MODULE","remotehost":"h","remotemodule":"mod","direction":"PULL","enabled":true,"locked":true,"job":null}
	]`)
	if r := rs["Offsite"]; r.State != TaskFailed || r.Remote != "backup.example:/srv/in" || r.Error != "rsync error: timeout" || r.Schedule != "30 3 * * mon,thu" {
		t.Errorf("rsync failed: %+v", r)
	}
	if r := rs["/mnt/tank/m"]; r.State != TaskRunning || r.Remote != "h::mod" {
		t.Errorf("rsync locked without a job reads as running: %+v", r)
	}

	cb := decode(t, KindCloudBackup, `[
	 {"id":1,"description":"Restic","path":"/mnt/tank/p","keep_last":7,"credentials":{"provider":{"type":"S3"}},"attributes":{"folder":"bk"},
	  "schedule":{"minute":"0","hour":"1","dom":"*","month":"*","dow":"*"},"enabled":true,"locked":false,
	  "job":{"state":"SUCCESS","time_started":{"$date":1795737601000},"time_finished":{"$date":1795737700000}}}
	]`)
	if r := cb["Restic"]; r.State != TaskOK || r.Retention != "last 7" || r.Kind != KindCloudBackup || r.Overdue {
		t.Errorf("cloud backup: %+v", r)
	}
}

func TestEmptyProtectionIsNotNil(t *testing.T) {
	for _, kind := range []string{KindSnapshot, KindCloudSync, KindRsync, KindCloudBackup} {
		list, err := decodeProtection(kind, json.RawMessage(`[]`), berlin, fixtureNow)
		if err != nil || list == nil || len(list) != 0 {
			t.Errorf("%s: %#v %v", kind, list, err)
		}
	}
	// Absent exclude lists still go out as [].
	if x := decode(t, KindRsync, `[{"id":1,"path":"/p","enabled":true}]`)["/p"]; x.Exclude == nil {
		t.Error("exclude = null")
	}
}

func TestListProtectionTasksMissingMethod(t *testing.T) {
	missing := map[string]any{"error": 32, "errname": "ENOMETHOD", "reason": "Method 'query' not found in 'cloud_backup'"}
	c := scriptedClient(t,
		map[string]string{"rsynctask.query": `[{"id":1,"desc":"x","path":"/p","enabled":true,"job":null}]`},
		map[string]map[string]any{"cloud_backup.query": missing, "cloudsync.query": {"error": 13, "errname": "EACCES", "reason": "denied"}})
	ctx := context.Background()

	if got, err := ListProtectionTasks(ctx, c, KindCloudBackup, berlin, fixtureNow); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("a missing method is empty, not stale: %#v %v", got, err)
	}
	if got, err := ListProtectionTasks(ctx, c, KindRsync, berlin, fixtureNow); err != nil || len(got) != 1 {
		t.Fatalf("rsync: %v %v", got, err)
	}
	if _, err := ListProtectionTasks(ctx, c, KindCloudSync, berlin, fixtureNow); err == nil || errors.Is(err, ErrNoMethod) {
		t.Fatalf("a real error must be an error (stale), got %v", err)
	}
}

func TestLastSlot(t *testing.T) {
	at := func(s string) time.Time {
		tm, err := time.ParseInLocation("2006-01-02 15:04", s, berlin)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	sched := func(m, h, dom, mon, dow string) rawSchedule {
		return rawSchedule{Minute: m, Hour: h, Dom: dom, Month: mon, Dow: dow}
	}
	cases := []struct {
		name string
		s    rawSchedule
		from string
		want string
	}{
		{"daily", sched("0", "2", "*", "*", "*"), "2026-11-27 15:49", "2026-11-27 02:00"},
		{"before today's slot", sched("0", "2", "*", "*", "*"), "2026-11-27 01:59", "2026-11-26 02:00"},
		{"every 15 min", sched("*/15", "*", "*", "*", "*"), "2026-11-27 15:49", "2026-11-27 15:45"},
		{"weekdays", sched("0", "3", "*", "*", "1-5"), "2026-11-28 12:00", "2026-11-27 03:00"}, // Saturday -> Friday
		{"names and 7=sunday", sched("0", "3", "*", "*", "sun"), "2026-11-27 12:00", "2026-11-22 03:00"},
		{"sunday as 7", sched("0", "3", "*", "*", "7"), "2026-11-27 12:00", "2026-11-22 03:00"},
		{"first of month", sched("0", "4", "1", "*", "*"), "2026-11-27 12:00", "2026-11-01 04:00"},
		{"list of hours", sched("0", "6,18", "*", "*", "*"), "2026-11-27 15:49", "2026-11-27 06:00"},
		{"window", rawSchedule{Minute: "0", Hour: "*", Dom: "*", Month: "*", Dow: "*", Begin: "08:00", End: "18:00"}, "2026-11-27 23:30", "2026-11-27 18:00"},
		{"yearly", sched("0", "0", "1", "1", "*"), "2026-11-27 12:00", "2026-01-01 00:00"},
	}
	for _, c := range cases {
		got, ok := lastSlot(c.s, berlin, at(c.from))
		if !ok || !got.Equal(at(c.want)) {
			t.Errorf("%s: got %v (%v), want %s", c.name, got, ok, c.want)
		}
	}
	for _, bad := range []rawSchedule{sched("", "", "", "", ""), sched("x", "2", "*", "*", "*"), sched("0", "25", "*", "*", "*"), sched("0", "2", "*", "*", "*/0")} {
		if _, ok := lastSlot(bad, berlin, fixtureNow); ok {
			t.Errorf("%+v parsed, want unreadable", bad)
		}
	}
}

func TestOverdueNeedsZoneAndReadableSchedule(t *testing.T) {
	old := int64(1)
	daily := rawSchedule{Minute: "0", Hour: "2", Dom: "*", Month: "*", Dow: "*"}
	if !overdue(daily, berlin, fixtureNow, &old) {
		t.Error("a run in 1970 must be overdue")
	}
	if overdue(daily, nil, fixtureNow, &old) {
		t.Error("without the NAS zone overdue is unknown, not true")
	}
	if overdue(rawSchedule{Minute: "?"}, berlin, fixtureNow, &old) {
		t.Error("unreadable schedule: unknown, not true")
	}
	// A run that began right at the slot counts, and so does the grace period:
	// at 02:10 a task whose 02:00 run has not appeared yet is not late.
	slot := time.Date(2026, 11, 27, 2, 0, 0, 0, berlin)
	prev := slot.Add(-24 * time.Hour).Unix()
	if overdue(daily, berlin, slot.Add(10*time.Minute), &prev) {
		t.Error("10 min after the slot is inside the grace period")
	}
	if !overdue(daily, berlin, slot.Add(30*time.Minute), &prev) {
		t.Error("30 min after the slot with no run is overdue")
	}
}
