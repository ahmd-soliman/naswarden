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
