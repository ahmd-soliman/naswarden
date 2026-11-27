package truenas

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const poolsJSON = `[
 {"name":"tank","status":"ONLINE","healthy":true,"warning":false,"size":1000,"allocated":400,"free":600,
  "scan":{"function":"SCRUB","state":"FINISHED","errors":0},"fragmentation":"12",
  "topology":{"data":[{"name":"mirror-0","type":"MIRROR","status":"ONLINE","children":[
    {"disk":"sda","status":"ONLINE","stats":{"read_errors":1,"write_errors":2,"checksum_errors":3}},
    {"disk":"sdb","status":"ONLINE","stats":{}}]}]}},
 {"name":"boot-pool","status":"ONLINE","healthy":true,"size":10,"allocated":1,"free":9,"scan":null,
  "topology":{"data":[{"name":"sdc","type":"DISK","status":"ONLINE","disk":"sdc","stats":{"checksum_errors":4}}]}},
 {"name":"empty","status":"ONLINE","healthy":true,"topology":{"data":[]}}
]`

func TestListPoolsBuildsVdevsAndAttachesTemperatures(t *testing.T) {
	c := methodServer(t, map[string]string{
		"pool.query":        poolsJSON,
		"disk.temperatures": `{"sda":41,"sdb":null,"sdc":30}`, // sdb is asleep
	})
	pools, err := ListPools(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(pools) != 3 {
		t.Fatalf("pools = %d, want 3", len(pools))
	}

	tank := pools[0]
	if tank.Name != "tank" || tank.Scan == nil || tank.Scan.State != "FINISHED" || tank.Fragmentation != "12" {
		t.Errorf("tank = %+v", tank)
	}
	kids := tank.Vdevs[0].Children
	if len(kids) != 2 {
		t.Fatalf("mirror children = %d, want 2", len(kids))
	}
	if kids[0].ReadErrors != 1 || kids[0].WriteErrors != 2 || kids[0].ChecksumErrors != 3 {
		t.Errorf("sda errors = %+v, want 1/2/3 (nested under stats)", kids[0])
	}
	if kids[0].Temperature == nil || *kids[0].Temperature != 41 || kids[0].Standby {
		t.Errorf("sda = %+v, want 41C and awake", kids[0])
	}
	if kids[1].Temperature != nil || !kids[1].Standby {
		t.Errorf("sdb = %+v, want asleep (null temperature means standby)", kids[1])
	}

	// A bare-disk vdev (boot-pool) has no children: the disk IS the vdev.
	boot := pools[1].Vdevs[0]
	if len(boot.Children) != 1 || boot.Children[0].Disk != "sdc" || boot.Children[0].ChecksumErrors != 4 {
		t.Errorf("boot vdev = %+v, want one child sdc with 4 checksum errors", boot)
	}

	// The UI calls .length on these, so they must serialise as [], never null.
	out, _ := json.Marshal(pools[2])
	if !strings.Contains(string(out), `"vdevs":[]`) {
		t.Errorf("empty pool JSON = %s, want vdevs as []", out)
	}
}

func TestListPoolsStillWorksWithoutTemperatures(t *testing.T) {
	// disk.temperatures is best-effort: pools must still be listed if it fails.
	c := methodServer(t, map[string]string{"pool.query": poolsJSON})
	pools, err := ListPools(context.Background(), c)
	if err != nil || len(pools) != 3 {
		t.Fatalf("pools = %d, err = %v; want 3 and no error", len(pools), err)
	}
	if k := pools[0].Vdevs[0].Children[0]; k.Temperature != nil || k.Standby {
		t.Errorf("child = %+v, want no temperature and NOT marked standby when the call failed", k)
	}
}

func TestListPoolsRejectsMalformedResponse(t *testing.T) {
	c := methodServer(t, map[string]string{"pool.query": `{"not":"a list"}`})
	if _, err := ListPools(context.Background(), c); err == nil {
		t.Fatal("expected a decode error")
	}
}

func TestListDatasetsOnlyKeepsQuotaedAndPicksTheRightUsage(t *testing.T) {
	c := methodServer(t, map[string]string{"pool.dataset.query": `[
	 {"name":"tank/quota","used":{"parsed":700},"usedbydataset":{"parsed":100},"usedbysnapshots":{"parsed":50},
	  "quota":{"parsed":1000},"refquota":{"parsed":null},"mountpoint":"/mnt/tank/quota",
	  "compression":{"value":"LZ4"},"compressratio":{"value":"1.08x"},"recordsize":{"value":"128K"},"encrypted":true},
	 {"name":"tank/ref","used":{"parsed":900},"usedbydataset":{"parsed":300},"usedbysnapshots":{"parsed":600},
	  "quota":{"parsed":null},"refquota":{"parsed":500}},
	 {"name":"tank/none","used":{"parsed":5},"quota":{"parsed":null},"refquota":{"parsed":null}},
	 {"name":"tank/zero","used":{"parsed":5},"quota":{"parsed":0},"refquota":{"parsed":0}}
	]`})
	got, err := ListDatasets(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("datasets = %+v, want only the two with a quota", got)
	}
	q, r := got[0], got[1]
	if q.QuotaSource != "quota" || q.Used != 700 || q.Quota != 1000 || q.CompressRatio != "1.08x" || !q.Encrypted || q.UsedBySnapshots != 50 {
		t.Errorf("quota dataset = %+v", q)
	}
	// refquota excludes snapshot space, so usage must be usedbydataset (300),
	// not used (900) -- otherwise a dataset that never broke its limit would
	// read as over quota.
	if r.QuotaSource != "refquota" || r.Used != 300 || r.Quota != 500 {
		t.Errorf("refquota dataset = %+v, want used 300 against quota 500", r)
	}
}

func TestListAlertsFiltersNormalisesAndSorts(t *testing.T) {
	c := methodServer(t, map[string]string{"alert.list": `[
	 {"id":"old-info","level":"info","formatted":"info","datetime":{"$date":1700000000000}},
	 {"id":"gone","level":"CRITICAL","formatted":"dismissed","dismissed":true,"datetime":{"$date":1700000900000}},
	 {"uuid":"u-warn","level":"WARNING","text":"only text","datetime":1700000500},
	 {"id":"new-crit","level":"critical","formatted":"newer","datetime":{"$date":1700000800000}},
	 {"id":"old-crit","level":"ERROR","formatted":"older","datetime":"2023-11-14T22:13:20Z"}
	]`})
	got, err := ListAlerts(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, a := range got {
		ids = append(ids, a.ID)
	}
	// Dismissed is dropped; CRITICAL/ERROR first (newest first within a level
	// group), then WARNING, then INFO.
	if want := "new-crit,old-crit,u-warn,old-info"; strings.Join(ids, ",") != want {
		t.Fatalf("order = %v, want %s", ids, want)
	}
	if got[0].Level != "CRITICAL" {
		t.Errorf("level = %q, want upper-cased", got[0].Level)
	}
	if got[2].Formatted != "only text" || got[2].Datetime != 1700000500 {
		t.Errorf("warn = %+v, want text as the fallback message and seconds kept as-is", got[2])
	}
}

func TestListReplicationsDerivesPoolsAndJobState(t *testing.T) {
	c := methodServer(t, map[string]string{"replication.query": `[
	 {"id":1,"name":"tank to backup","direction":"PUSH","transport":"LOCAL","enabled":true,
	  "source_datasets":["tank/b","tank/a","fast/x"],"target_dataset":"backup/tank",
	  "state":{"state":"FINISHED","datetime":{"$date":1700000000000},"last_snapshot":"tank/a@auto-1"}},
	 {"id":2,"name":"running","direction":"PUSH","transport":"SSH","enabled":false,
	  "source_datasets":null,"target_dataset":"",
	  "state":{"state":"RUNNING"},
	  "job":{"id":9,"state":"RUNNING","progress":{"percent":42.5,"description":"sending"}}}
	]`})
	got, err := ListReplications(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("tasks = %d, want 2", len(got))
	}
	a := got[0]
	if a.TargetPool != "backup" || strings.Join(a.SourcePools, ",") != "fast,tank" || a.LastSnapshot != "tank/a@auto-1" || !a.Enabled {
		t.Errorf("task 1 = %+v, want target backup, sorted source pools fast,tank", a)
	}
	b := got[1]
	if b.SourceDatasets == nil || b.SourcePools == nil {
		t.Errorf("task 2 has nil slices (would serialise as null): %+v", b)
	}
	if b.JobState != "RUNNING" || b.ProgressPercent == nil || *b.ProgressPercent != 42.5 || b.ProgressDescription != "sending" {
		t.Errorf("task 2 job = %+v, want running at 42.5%%", b)
	}
	if b.Enabled {
		t.Error("task 2 must stay disabled")
	}
}

func TestGetServerInfoAssemblesGraphsAndInterfaces(t *testing.T) {
	g := func(name, legend, row string) string {
		return `{"name":"` + name + `","legend":["time",` + legend + `],"data":[[1,` + row + `]]}`
	}
	c := methodServer(t, map[string]string{
		"system.info": `{"hostname":"nas","version":"25.10","uptime_seconds":3600,"physmem":1000,"model":"Some CPU","cores":4,"physical_cores":2}`,
		"reporting.netdata_get_data": `[` +
			g("cpu", `"cpu"`, `12.5`) + `,` +
			g("memory", `"available"`, `400`) + `,` +
			g("load", `"shortterm","midterm","longterm"`, `2,1,0.4`) + `,` +
			g("arcsize", `"size"`, `777`) + `,` +
			g("cputemp", `"cpu","cpu0"`, `55,54`) + `]`,
		"interface.query": `[
		 {"name":"eno1","type":"PHYSICAL","state":{"link_state":"LINK_STATE_UP","active_media_subtype":"1000Mb/s Twisted Pair","aliases":[{"type":"INET","address":"192.0.2.10","netmask":24}]}},
		 {"name":"eno2","type":"PHYSICAL","state":{"link_state":"LINK_STATE_DOWN"}}
		]`,
	})
	info, err := GetServerInfo(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if info.Hostname != "nas" || info.Cores != 4 || info.MemTotal != 1000 {
		t.Errorf("identity = %+v", info)
	}
	if info.CPUPercent != 12.5 || info.CPUTempC != 55 || info.ArcBytes != 777 {
		t.Errorf("cpu/temp/arc = %v/%v/%d", info.CPUPercent, info.CPUTempC, info.ArcBytes)
	}
	if info.MemUsed != 600 {
		t.Errorf("mem used = %d, want total 1000 minus available 400", info.MemUsed)
	}
	// Load is shown as a percentage of the core count: 2 on 4 cores = 50%.
	if info.LoadPercent1 != 50 || info.LoadPercent5 != 25 || info.LoadPercent15 != 10 {
		t.Errorf("load = %v/%v/%v, want 50/25/10", info.LoadPercent1, info.LoadPercent5, info.LoadPercent15)
	}
	if len(info.Interfaces) != 1 || info.Interfaces[0].Name != "eno1" || info.Interfaces[0].SpeedMbps != 1000 ||
		info.Interfaces[0].Addresses[0] != "192.0.2.10/24" {
		t.Errorf("interfaces = %+v, want only the link-up eno1 at 1000 Mb/s", info.Interfaces)
	}
}

// With no quota on any dataset (common on a fresh TrueNAS) the list must
// still marshal as [], not null: a null blanked the whole UI.
func TestListDatasetsWithoutQuotasIsEmptyNotNull(t *testing.T) {
	c := methodServer(t, map[string]string{
		"pool.dataset.query": `[{"name":"tank","used":{"parsed":10},"available":{"parsed":90}}]`,
	})
	datasets, err := ListDatasets(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(datasets)
	if string(out) != "[]" {
		t.Errorf("datasets = %s, want []", out)
	}
}

// The pool card shows what files can use, not the zpool's raw size: the root
// dataset's used + available, which is what `zfs list -o used,avail <pool>`
// shows. Taken from the same pool.dataset.query answer as the quotas.
func TestListDatasetsWithUsageReadsRootDatasetsOnly(t *testing.T) {
	c := methodServer(t, map[string]string{"pool.dataset.query": `[
	 {"name":"tank","used":{"parsed":1900},"available":{"parsed":30100},"quota":{"parsed":null},"refquota":{"parsed":null}},
	 {"name":"tank/media","used":{"parsed":700},"available":{"parsed":30100},"quota":{"parsed":1000},"refquota":{"parsed":null}},
	 {"name":"boot-pool","used":{"parsed":5},"available":{"parsed":95}},
	 {"name":"nospace","used":{"parsed":5}},
	 {"name":"","used":{"parsed":1},"available":{"parsed":1}}
	]`})
	datasets, usage, err := ListDatasetsWithUsage(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(datasets) != 1 || datasets[0].Name != "tank/media" {
		t.Errorf("datasets = %+v, want only the quota'd child (the quota list is unchanged)", datasets)
	}
	if len(usage) != 2 || usage["tank"] != (Usage{Used: 1900, Available: 30100}) || usage["boot-pool"] != (Usage{Used: 5, Available: 95}) {
		t.Errorf("usage = %+v, want tank and boot-pool only (children, no-available and unnamed are skipped)", usage)
	}
}

func TestApplyUsableAndFallbackToRaw(t *testing.T) {
	pools := []Pool{
		{Name: "tank", Size: 43600, Allocated: 2900}, // 4x12 TB RAIDZ1: raw size counts parity
		{Name: "gone", Size: 100, Allocated: 10},     // root dataset not returned
		{Name: "odd", Size: 100, Allocated: 10},      // reports nothing at all
		{Name: "neg", Size: 100, Allocated: 10},      // nonsense values
	}
	ApplyUsable(pools, map[string]Usage{
		"tank": {Used: 1900, Available: 30100},
		"odd":  {},
		"neg":  {Used: -1, Available: 5},
	})
	tank := pools[0]
	if tank.UsableSize != 32000 || tank.UsableUsed != 1900 || tank.UsableAvailable != 30100 {
		t.Errorf("tank = %+v, want usable 1900 / 32000 with 30100 free", tank)
	}
	if tank.Size != 43600 || tank.Allocated != 2900 {
		t.Errorf("raw figures changed: %+v (they stay for the detail drawer and old metrics)", tank)
	}
	for _, p := range pools[1:] {
		if p.UsableSize != 0 || p.UsableUsed != 0 || p.UsableAvailable != 0 {
			t.Errorf("%s = %+v, want no usable figures so the UI falls back to raw", p.Name, p)
		}
	}
	// Absent, not 0, on the wire.
	out, _ := json.Marshal(pools[1])
	if strings.Contains(string(out), "usable") {
		t.Errorf("json = %s, want no usable_* keys without a root dataset", out)
	}
}

func TestScanReadsEndTimeAndProgress(t *testing.T) {
	var p Pool
	in := `{"name":"t","scan":{"function":"SCRUB","state":"FINISHED","errors":0,"end_time":{"$date":1700000000000},"percentage":100.0}}`
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	if p.Scan == nil || p.Scan.EndTime == nil || *p.Scan.EndTime != 1700000000 {
		t.Fatalf("end time = %+v, want 1700000000", p.Scan)
	}
	if p.Scan.Percentage == nil || *p.Scan.Percentage != 100 {
		t.Errorf("percentage = %v, want 100", p.Scan.Percentage)
	}
	in = `{"name":"t","scan":{"function":"SCRUB","state":"IN_PROGRESS","end_time":null}}`
	p = Pool{}
	if err := json.Unmarshal([]byte(in), &p); err != nil || p.Scan.EndTime != nil {
		t.Errorf("a running scan has no end time: %+v, %v", p.Scan, err)
	}
}

// Trimmed copy of a real pool.query reply (TrueNAS SCALE 25.10): two mirror
// vdevs' own `stats` sit next to their children's, a bare-disk vdev has stats
// of its own and no children. Names and ids are replaced.
const vdevStatsJSON = `[
 {"name":"tank","status":"ONLINE","healthy":true,
  "topology":{"data":[
   {"name":"mirror-0","type":"MIRROR","status":"ONLINE",
    "stats":{"read_errors":0,"write_errors":0,"checksum_errors":5},
    "children":[
     {"disk":"sda","name":"id-1","type":"DISK","status":"ONLINE","stats":{"read_errors":0,"write_errors":0,"checksum_errors":5}},
     {"disk":"sdb","name":"id-2","type":"DISK","status":"ONLINE","stats":{"read_errors":0,"write_errors":0,"checksum_errors":0}}]},
   {"name":"mirror-1","type":"MIRROR","status":"ONLINE","children":[{"disk":"sdc","status":"ONLINE","stats":{}}]},
   {"name":"id-3","type":"DISK","disk":"sdd","status":"ONLINE","stats":{"read_errors":0,"write_errors":0,"checksum_errors":0}}]}}
]`

func TestListPoolsReadsTheVdevsOwnErrorCounts(t *testing.T) {
	c := methodServer(t, map[string]string{"pool.query": vdevStatsJSON})
	pools, err := ListPools(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	v := pools[0].Vdevs
	if v[0].Errors == nil || *v[0].Errors != (VdevErrors{Checksum: 5}) {
		t.Errorf("mirror-0 errors = %+v, want checksum 5", v[0].Errors)
	}
	if v[1].Errors != nil {
		t.Errorf("mirror-1 sent no stats, errors = %+v, want nil (unknown, not 0)", v[1].Errors)
	}
	if v[2].Errors == nil || len(v[2].Children) != 1 {
		t.Errorf("bare-disk vdev = %+v", v[2])
	}
	out, _ := json.Marshal(pools[0])
	if strings.Contains(string(out), "Errors") || strings.Contains(string(out), `"Read"`) {
		t.Errorf("vdev errors leaked into the UI JSON: %s", out)
	}
}
