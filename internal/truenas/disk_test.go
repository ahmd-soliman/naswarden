package truenas

import (
	"encoding/json"
	"testing"
	"time"
)

func f(v float64) *float64 { return &v }
func s(v string) *string   { return &v }

func TestDiskRates(t *testing.T) {
	r := NewDiskRates()
	t0 := time.Unix(1_000, 0)
	if rr, wr := r.Rate(t0, "sda", 1000, 5000); rr != nil || wr != nil {
		t.Fatal("first sample has no rate")
	}
	rr, wr := r.Rate(t0.Add(60*time.Second), "sda", 1000+60*1_000_000, 5000+60*3_000_000)
	if rr == nil || wr == nil || *rr != 1_000_000 || *wr != 3_000_000 {
		t.Fatalf("want 1MB/s and 3MB/s, got %v %v", rr, wr)
	}
	// counter went backwards (pool re-imported): no bogus number
	if rr, wr := r.Rate(t0.Add(120*time.Second), "sda", 10, 10); rr != nil || wr != nil {
		t.Fatal("a counter reset must not produce a rate")
	}
	// and it recovers on the next sample
	if rr, _ := r.Rate(t0.Add(180*time.Second), "sda", 10+60*100, 10); rr == nil || *rr != 100 {
		t.Fatalf("should recover after reset, got %v", rr)
	}
	r.Forget(map[string]struct{}{})
	if rr, _ := r.Rate(t0.Add(240*time.Second), "sda", 1, 1); rr != nil {
		t.Fatal("forgotten disk starts over")
	}
}

// The pool.query shape below is a trimmed copy of the real response from a
// TrueNAS SCALE 25.10.7 box (mirror vdev with per-disk stats.bytes counters).
const poolQueryFixture = `[{"name":"tank","topology":{"data":[
 {"name":"mirror-0","type":"MIRROR","status":"ONLINE","stats":{"read_errors":0},"children":[
  {"name":"a","disk":"sda","status":"ONLINE","stats":{"read_errors":0,"write_errors":0,"checksum_errors":2,"bytes":[0,1039663050752,2480755752960,0,0,0,0]}},
  {"name":"b","disk":"sdb","status":"DEGRADED","stats":{"read_errors":1,"write_errors":0,"checksum_errors":0,"bytes":[0,10,20,0,0,0,0]}}]},
 {"name":"sdz","disk":"sdz","status":"ONLINE","stats":{"bytes":[]}}]}}]`

func TestCollectMembers(t *testing.T) {
	var pools []rawPoolStats
	if err := json.Unmarshal([]byte(poolQueryFixture), &pools); err != nil {
		t.Fatal(err)
	}
	m := map[string]poolDisk{}
	collectMembers(pools[0].Topology.Data, "", m)
	a := m["sda"]
	if a.vdev != "mirror-0" || a.read != 1039663050752 || a.write != 2480755752960 || !a.hasStats || a.ce != 2 {
		t.Fatalf("sda: %+v", a)
	}
	if m["sdb"].status != "DEGRADED" || m["sdb"].rerr != 1 {
		t.Fatalf("sdb: %+v", m["sdb"])
	}
	if z, ok := m["sdz"]; !ok || z.hasStats {
		t.Fatalf("a bare disk vdev is a member without counters: %+v", z)
	}
}

func TestBuildDisks(t *testing.T) {
	rot := 7200
	all := []rawDiskDetail{
		{Name: "sdg", Model: "WD", Size: 6, Type: "HDD", ImportedZpool: s("backup")},
		{Name: "sda", Model: "ST24000", Size: 24, Type: "HDD", RotationRate: &rot, Bus: "ATA", ImportedZpool: s("tank")},
		{Name: "sdd", Model: "INTENSO", Type: "SSD", ImportedZpool: s("boot-pool")},
		{Name: "sdx", Model: "spare", Type: "SSD"},
	}
	temps := map[string]*float64{"sda": f(49), "sdg": nil, "sdd": f(40)}
	agg := map[string]*tempAgg{"sda": {Min: 47, Max: 55, Avg: 49.7}, "sdg": {}}
	members := map[string]poolDisk{"sda": {vdev: "mirror-0", status: "ONLINE", read: 100, write: 200, hasStats: true, ce: 3}}
	rates := NewDiskRates()
	now := time.Unix(5_000, 0)

	got := buildDisks(all, temps, agg, members, rates, now)
	names := []string{}
	for _, d := range got {
		names = append(names, d.Name)
	}
	// sorted by pool name, case-insensitive ("" first), then disk name
	if want := []string{"sdx", "sdg", "sdd", "sda"}; !equal(names, want) {
		t.Fatalf("order: got %v want %v", names, want)
	}
	byName := map[string]Disk{}
	for _, d := range got {
		byName[d.Name] = d
	}
	a := byName["sda"]
	if a.Pool != "tank" || a.Vdev != "mirror-0" || a.Status != "ONLINE" || a.RotationRate != 7200 || a.ChecksumErrors != 3 {
		t.Fatalf("sda: %+v", a)
	}
	if a.TempC == nil || *a.TempC != 49 || a.Temp7dMax == nil || *a.Temp7dMax != 55 || a.Standby {
		t.Fatalf("sda temps: %+v", a)
	}
	if a.ReadBytes == nil || *a.ReadBytes != 100 || a.ReadRate != nil {
		t.Fatalf("first refresh has counters but no rate: %+v", a)
	}
	if g := byName["sdg"]; !g.Standby || g.TempC != nil || g.Temp7dMax != nil {
		t.Fatalf("a null temperature means standby: %+v", g)
	}
	if d := byName["sdd"]; d.ReadBytes != nil || d.Status != "" {
		t.Fatalf("a boot disk (not in pool.query) has no counters: %+v", d)
	}
	if byName["sdx"].Pool != "" {
		t.Fatal("an unused disk has no pool")
	}
	// second refresh 60 s later -> rate
	members["sda"] = poolDisk{vdev: "mirror-0", status: "ONLINE", read: 100 + 60*10, write: 200 + 60*20, hasStats: true}
	again := buildDisks(all, temps, agg, members, rates, now.Add(60*time.Second))
	for _, d := range again {
		if d.Name == "sda" && (d.ReadRate == nil || *d.ReadRate != 10 || *d.WriteRate != 20) {
			t.Fatalf("rate: %+v", d)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseSpeedMbps(t *testing.T) {
	for in, want := range map[string]int{"1000Mb/s Twisted Pair": 1000, "2500Mb/s": 2500, "10Mb/s": 10, "": 0, "autoselect": 0, "1000 Mb/s": 0, "Mb/s": 0} {
		if got := parseSpeedMbps(in); got != want {
			t.Errorf("%q: got %d want %d", in, got, want)
		}
	}
}

func TestMeanTail(t *testing.T) {
	g := reportingGraph{Legend: []string{"time", "received", "sent"}}
	for i := 0; i < 100; i++ {
		g.Data = append(g.Data, []json.Number{json.Number("1"), json.Number("10"), json.Number("2")})
	}
	// only the last 60 points count: make the older ones very different
	for i := 0; i < 40; i++ {
		g.Data[i] = []json.Number{"1", "9999", "9999"}
	}
	if m := meanTail(g, "received", 60); m == nil || *m != 10 {
		t.Fatalf("received: %v", m)
	}
	if m := meanTail(g, "nope", 60); m != nil {
		t.Fatal("a missing column is nil")
	}
	if m := meanTail(reportingGraph{Legend: g.Legend}, "sent", 60); m != nil {
		t.Fatal("no data is nil")
	}
}
