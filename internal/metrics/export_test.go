package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/ahmd-soliman/naswarden/internal/docker"
	"github.com/ahmd-soliman/naswarden/internal/truenas"
	"github.com/ahmd-soliman/naswarden/internal/vm"
)

func gaugeValue(g prometheus.Gauge) float64 {
	var m dto.Metric
	if err := g.Write(&m); err != nil {
		return 0
	}
	return m.GetGauge().GetValue()
}

func TestUpdateDatasets(t *testing.T) {
	datasets := []truenas.Dataset{
		{Name: "tank/media", Used: 1000, Quota: 5000},
		{Name: "fast/backup", Used: 2500, Quota: 3000},
	}
	UpdateDatasets(datasets)

	if val := gaugeValue(datasetUsedBytes.WithLabelValues("tank/media")); val != 1000 {
		t.Errorf("expected 1000, got %f", val)
	}
	if val := gaugeValue(datasetQuotaBytes.WithLabelValues("tank/media")); val != 5000 {
		t.Errorf("expected 5000, got %f", val)
	}

	UpdateDatasets(nil)
}

func TestUpdatePools(t *testing.T) {
	pools := []truenas.Pool{
		{Name: "tank", Healthy: true, Allocated: 500000, Size: 1000000},
		{Name: "backup", Healthy: false, Allocated: 200000, Size: 800000},
	}
	UpdatePools(pools)

	if val := gaugeValue(poolAllocatedBytes.WithLabelValues("tank")); val != 500000 {
		t.Errorf("expected 500000, got %f", val)
	}
	if val := gaugeValue(poolSizeBytes.WithLabelValues("tank")); val != 1000000 {
		t.Errorf("expected 1000000, got %f", val)
	}
	if val := gaugeValue(poolHealthy.WithLabelValues("tank")); val != 1.0 {
		t.Errorf("expected 1.0 healthy, got %f", val)
	}
	if val := gaugeValue(poolHealthy.WithLabelValues("backup")); val != 0.0 {
		t.Errorf("expected 0.0 healthy for degraded pool, got %f", val)
	}
}

func TestUpdatePoolUsable(t *testing.T) {
	UpdatePoolUsable([]truenas.Pool{
		{Name: "tank", UsableSize: 32000, UsableUsed: 1900, UsableAvailable: 30100},
		{Name: "noroot"}, // root dataset unreadable: no series, never 0
	})
	if val := gaugeValue(poolUsableBytes.WithLabelValues("tank")); val != 32000 {
		t.Errorf("usable = %f, want 32000", val)
	}
	if val := gaugeValue(poolAvailableBytes.WithLabelValues("tank")); val != 30100 {
		t.Errorf("available = %f, want 30100", val)
	}
	if n := seriesCount(t, poolUsableBytes); n != 1 {
		t.Errorf("usable series = %d, want 1 (no series for a pool without a root dataset)", n)
	}

	// A pool that disappears takes its series with it.
	UpdatePoolUsable(nil)
	if n := seriesCount(t, poolUsableBytes); n != 0 {
		t.Errorf("usable series = %d after the pool went away, want 0", n)
	}
}

func TestUpdateInstances(t *testing.T) {
	instances := []vm.Instance{
		{Name: "win", Manager: "truenas", Type: "virtual-machine", Status: "RUNNING"},
		{Name: "k8s-node-1", Manager: "incus", Type: "virtual-machine", Status: "Stopped"},
	}
	UpdateInstances(instances)

	if val := gaugeValue(instanceUp.WithLabelValues("win", "truenas", "virtual-machine")); val != 1.0 {
		t.Errorf("expected 1.0, got %f", val)
	}
	if val := gaugeValue(instanceUp.WithLabelValues("k8s-node-1", "incus", "virtual-machine")); val != 0.0 {
		t.Errorf("expected 0.0, got %f", val)
	}
}

func TestUpdateContainers(t *testing.T) {
	containers := []docker.Container{
		{Name: "jellyfin", Stack: "media", State: "running"},
		{Name: "backup-runner", Stack: "ops", State: "exited"},
	}
	UpdateContainers(containers)

	if val := gaugeValue(containerUp.WithLabelValues("jellyfin", "media")); val != 1.0 {
		t.Errorf("expected 1.0, got %f", val)
	}
	if val := gaugeValue(containerUp.WithLabelValues("backup-runner", "ops")); val != 0.0 {
		t.Errorf("expected 0.0, got %f", val)
	}
}

func seriesCount(t *testing.T, vec *prometheus.GaugeVec) int {
	t.Helper()
	ch := make(chan prometheus.Metric, 100)
	vec.Collect(ch)
	close(ch)
	n := 0
	for range ch {
		n++
	}
	return n
}

func TestVanishedSeriesAreDeletedNotReset(t *testing.T) {
	UpdateContainers([]docker.Container{{Name: "a", Stack: "s", State: "running"}, {Name: "b", Stack: "s", State: "running"}})
	if n := seriesCount(t, containerUp); n != 2 {
		t.Fatalf("want 2 series, got %d", n)
	}
	UpdateContainers([]docker.Container{{Name: "a", Stack: "s", State: "running"}})
	if n := seriesCount(t, containerUp); n != 1 {
		t.Fatalf("removed container should drop its series, got %d", n)
	}
	UpdateContainers(nil)
	if n := seriesCount(t, containerUp); n != 0 {
		t.Fatalf("want 0 series, got %d", n)
	}
}

func TestUpdateAlerts(t *testing.T) {
	alerts := []truenas.Alert{
		{ID: "1", Level: "WARNING", Dismissed: false},
		{ID: "2", Level: "WARNING", Dismissed: false},
		{ID: "3", Level: "CRITICAL", Dismissed: false},
		{ID: "4", Level: "INFO", Dismissed: true},
	}
	UpdateAlerts(alerts)

	if val := gaugeValue(alertCount.WithLabelValues("WARNING")); val != 2.0 {
		t.Errorf("expected 2 warnings, got %f", val)
	}
	if val := gaugeValue(alertCount.WithLabelValues("CRITICAL")); val != 1.0 {
		t.Errorf("expected 1 critical, got %f", val)
	}
}

func TestUpdateReplications(t *testing.T) {
	tasks := []truenas.ReplicationTask{
		{ID: 24, Name: "tank-backup", TargetPool: "backup", State: "FINISHED", JobState: "SUCCESS", Enabled: true},
	}
	UpdateReplications(tasks)

	if val := gaugeValue(replicationTaskStatus.WithLabelValues("24", "tank-backup", "backup", "FINISHED", "SUCCESS")); val != 1.0 {
		t.Errorf("expected 1.0, got %f", val)
	}
}

func TestAlertAndReplicationSeriesDropWhenResolved(t *testing.T) {
	UpdateAlerts([]truenas.Alert{{Level: "WARNING"}, {Level: "CRITICAL"}})
	if n := seriesCount(t, alertCount); n != 2 {
		t.Fatalf("want 2 alert series, got %d", n)
	}
	UpdateAlerts([]truenas.Alert{{Level: "WARNING"}})
	if n := seriesCount(t, alertCount); n != 1 {
		t.Fatalf("resolved alert level should drop its series, got %d", n)
	}
	UpdateAlerts(nil)

	UpdateReplications([]truenas.ReplicationTask{{ID: 1, Name: "a", TargetPool: "p", State: "RUNNING", JobState: "RUNNING", Enabled: true}})
	UpdateReplications([]truenas.ReplicationTask{{ID: 1, Name: "a", TargetPool: "p", State: "FINISHED", JobState: "SUCCESS", Enabled: true}})
	if n := seriesCount(t, replicationTaskStatus); n != 1 {
		t.Fatalf("a task changing state must replace its series, got %d", n)
	}
	UpdateReplications(nil)
}

func TestReplicationLastSuccessSurvivesAFailedRun(t *testing.T) {
	lastSuccess = map[string]float64{}
	done := int64(1_700_000_000)
	UpdateReplications([]truenas.ReplicationTask{{ID: 1, Name: "bkp", TargetPool: "p", State: "FINISHED", JobState: "SUCCESS", TimeFinished: &done, Enabled: true}})
	if v := gaugeValue(replicationLastSuccess.WithLabelValues("bkp")); v != float64(done) {
		t.Fatalf("want %d, got %f", done, v)
	}
	failed := done + 3600
	UpdateReplications([]truenas.ReplicationTask{{ID: 1, Name: "bkp", TargetPool: "p", State: "ERROR", JobState: "FAILED", TimeFinished: &failed, Enabled: true}})
	if v := gaugeValue(replicationLastSuccess.WithLabelValues("bkp")); v != float64(done) {
		t.Fatalf("a failed run must not overwrite the last success, got %f", v)
	}
	UpdateReplications(nil)
	if n := seriesCount(t, replicationLastSuccess); n != 0 {
		t.Fatalf("a removed task should drop its series, got %d", n)
	}
}

func TestUpdateRefreshMarksStaleSources(t *testing.T) {
	now := time.Unix(1_700_000_500, 0)
	UpdateRefresh(now, []string{"docker"})
	if v := gaugeValue(lastRefreshSuccess); v != float64(now.Unix()) {
		t.Fatalf("timestamp: got %f", v)
	}
	if v := gaugeValue(sourceStale.WithLabelValues("docker")); v != 1 {
		t.Fatalf("docker should be stale, got %f", v)
	}
	if v := gaugeValue(sourceStale.WithLabelValues("incus")); v != 0 {
		t.Fatalf("incus should be fresh, got %f", v)
	}
	UpdateRefresh(now, nil)
	if v := gaugeValue(sourceStale.WithLabelValues("docker")); v != 0 {
		t.Fatalf("docker should recover, got %f", v)
	}
}

// The gauges are only useful if they are registered: a metric that is set but
// never registered is silently absent from /metrics. This reads what the real
// registry exposes, which is what Prometheus scrapes.
func TestPoolUsableMetricsAreExposedOnTheRegistry(t *testing.T) {
	UpdatePoolUsable([]truenas.Pool{{Name: "tank", UsableSize: 32000, UsableUsed: 1900, UsableAvailable: 30100}})
	t.Cleanup(func() { UpdatePoolUsable(nil) })

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{"naswarden_pool_usable_bytes": 32000, "naswarden_pool_available_bytes": 30100}
	for _, f := range families {
		if v, ok := want[f.GetName()]; ok {
			if got := f.GetMetric()[0].GetGauge().GetValue(); got != v {
				t.Errorf("%s = %v, want %v", f.GetName(), got, v)
			}
			delete(want, f.GetName())
		}
	}
	for name := range want {
		t.Errorf("%s is not exposed: register it in init()", name)
	}
}

func TestUpdateDisksExportsErrorsAndRatesOnlyForMembers(t *testing.T) {
	rd, wr := 1500.0, 700.0
	rb := int64(10)
	UpdateDisks([]truenas.Disk{
		{Name: "sda", Pool: "tank", ReadBytes: &rb, ReadErrors: 1, WriteErrors: 2, ChecksumErrors: 3, ReadRate: &rd, WriteRate: &wr},
		{Name: "sdb", Pool: "tank", ReadBytes: &rb}, // first refresh: no rate yet
		{Name: "sdc"}, // not in a pool
	})
	for typ, want := range map[string]float64{"read": 1, "write": 2, "checksum": 3} {
		if got := gaugeValue(diskErrors.WithLabelValues("tank", "sda", typ)); got != want {
			t.Errorf("errors{%s} = %v, want %v", typ, got, want)
		}
	}
	if got := gaugeValue(diskReadRate.WithLabelValues("tank", "sda")); got != 1500 {
		t.Errorf("read rate = %v, want 1500", got)
	}
	if n := seriesCount(t, diskReadRate); n != 1 {
		t.Errorf("%d read-rate series, want 1: a disk without a rate must have none, not 0", n)
	}
	if n := seriesCount(t, diskErrors); n != 6 {
		t.Errorf("%d error series, want 6 (two member disks x three types, none for the non-member)", n)
	}
	UpdateDisks(nil)
	if n := seriesCount(t, diskErrors); n != 0 {
		t.Errorf("%d error series left after the disks went away", n)
	}
}

func TestUpdatePoolsExportsFragmentationAndScans(t *testing.T) {
	end, pct := int64(1700000000), 41.5
	UpdatePools([]truenas.Pool{
		{Name: "tank", Fragmentation: "12", Scan: &truenas.Scan{Function: "SCRUB", State: "FINISHED", EndTime: &end}},
		{Name: "fast", Fragmentation: "-", Scan: &truenas.Scan{Function: "RESILVER", State: "IN_PROGRESS", Percentage: &pct}},
		{Name: "boot-pool"}, // never scanned, no fragmentation
	})
	if got := gaugeValue(poolFragmentation.WithLabelValues("tank")); got != 12 {
		t.Errorf("fragmentation = %v, want 12", got)
	}
	if n := seriesCount(t, poolFragmentation); n != 1 {
		t.Errorf("%d fragmentation series, want 1: '-' must not become 0", n)
	}
	if got := gaugeValue(poolLastScrub.WithLabelValues("tank")); got != float64(end) {
		t.Errorf("last scrub = %v, want %v", got, end)
	}
	if n := seriesCount(t, poolLastScrub); n != 1 {
		t.Errorf("%d last-scrub series, want 1: a resilver is not a scrub", n)
	}
	if got := gaugeValue(poolScanInProgress.WithLabelValues("fast", "resilver")); got != 1 {
		t.Errorf("resilver in progress = %v, want 1", got)
	}
	if got := gaugeValue(poolScanInProgress.WithLabelValues("tank", "scrub")); got != 0 {
		t.Errorf("finished scrub in progress = %v, want 0", got)
	}
	if got := gaugeValue(poolScanProgress.WithLabelValues("fast", "resilver")); got != 41.5 {
		t.Errorf("scan progress = %v, want 41.5", got)
	}
	if n := seriesCount(t, poolScanProgress); n != 1 {
		t.Errorf("%d progress series, want 1: only a running scan has one", n)
	}
	UpdatePools(nil)
}

func TestUpdateDatasetsExportsSnapshotSpaceAndRatio(t *testing.T) {
	UpdateDatasets([]truenas.Dataset{
		{Name: "tank/a", Used: 1, Quota: 2, UsedBySnapshots: 300, CompressRatio: "1.08x"},
		{Name: "tank/b", Used: 1, Quota: 2, UsedBySnapshots: 0, CompressRatio: ""},
	})
	if got := gaugeValue(datasetSnapshotBytes.WithLabelValues("tank/a")); got != 300 {
		t.Errorf("snapshot bytes = %v, want 300", got)
	}
	if got := gaugeValue(datasetCompressRatio.WithLabelValues("tank/a")); got != 1.08 {
		t.Errorf("ratio = %v, want 1.08", got)
	}
	if n := seriesCount(t, datasetCompressRatio); n != 1 {
		t.Errorf("%d ratio series, want 1: an unreadable ratio must have none", n)
	}
	UpdateDatasets(nil)
}

func TestUpdateServerExportsVersionInfo(t *testing.T) {
	UpdateServer(&truenas.ServerInfo{Version: "25.10.1"})
	if got := gaugeValue(truenasInfo.WithLabelValues("25.10.1")); got != 1 {
		t.Errorf("info = %v, want 1", got)
	}
	UpdateServer(&truenas.ServerInfo{Version: "26.04"})
	if n := seriesCount(t, truenasInfo); n != 1 {
		t.Errorf("%d info series after an upgrade, want 1 (the old version must go)", n)
	}
}

func TestUpdateServicesOnlyServiceNameLabel(t *testing.T) {
	UpdateServices([]truenas.Service{
		{Name: "cifs", State: "STOPPED", Enable: true, Problem: true},
		{Name: "nfs", State: "RUNNING", Enable: true},
	})
	if v := gaugeValue(serviceUp.WithLabelValues("cifs")); v != 0 {
		t.Errorf("stopped cifs = %f, want 0", v)
	}
	if v := gaugeValue(serviceUp.WithLabelValues("nfs")); v != 1 {
		t.Errorf("running nfs = %f, want 1", v)
	}
	UpdateServices(nil)
	if n := seriesCount(t, serviceUp); n != 0 {
		t.Fatalf("a service that is no longer shown must drop its series, got %d", n)
	}
}

func TestUpdateProtectionCountsByKindAndState(t *testing.T) {
	t.Cleanup(func() { UpdateProtection(nil) })
	UpdateProtection([]truenas.ProtectionTask{
		{Kind: "snapshot", Name: "tank/secret-dataset", State: "OK"},
		{Kind: "snapshot", Name: "tank/b", State: "OK"},
		{Kind: "snapshot", Name: "tank/c", State: "FAILED"},
		{Kind: "cloudsync", Name: "Photos to cloud", State: "OK", Overdue: true},
		{Kind: "cloudsync", Name: "off", State: "DISABLED"},
	})
	for _, c := range []struct {
		kind, state string
		want        float64
	}{
		{"snapshot", "OK", 2}, {"snapshot", "FAILED", 1}, {"snapshot", "RUNNING", 0},
		{"cloudsync", "OK", 1}, {"cloudsync", "DISABLED", 1}, {"cloudsync", "FAILED", 0},
	} {
		if got := gaugeValue(protectionTasks.WithLabelValues(c.kind, c.state)); got != c.want {
			t.Errorf("tasks{%s,%s} = %v, want %v", c.kind, c.state, got, c.want)
		}
	}
	if got := gaugeValue(protectionOverdue.WithLabelValues("cloudsync")); got != 1 {
		t.Errorf("overdue{cloudsync} = %v, want 1", got)
	}
	if got := gaugeValue(protectionOverdue.WithLabelValues("snapshot")); got != 0 {
		t.Errorf("overdue{snapshot} = %v, want 0", got)
	}
	// Two kinds present, six states each; no series for a kind without tasks.
	if n := seriesCount(t, protectionTasks); n != 12 {
		t.Errorf("task series = %d, want 12", n)
	}

	// Names never reach /metrics.
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, f := range families {
		if !strings.HasPrefix(f.GetName(), "naswarden_protection_") {
			continue
		}
		seen = true // also proves both are registered
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() != "kind" && l.GetName() != "state" {
					t.Errorf("%s has label %q: only kind and state are allowed", f.GetName(), l.GetName())
				}
				if strings.Contains(l.GetValue(), "secret") || strings.Contains(l.GetValue(), "Photos") {
					t.Errorf("%s leaks a task name in %q", f.GetName(), l.GetValue())
				}
			}
		}
	}
	if !seen {
		t.Error("naswarden_protection_* is not exposed: register it in init()")
	}
}

func TestProtectionSeriesDropWhenAKindDisappears(t *testing.T) {
	t.Cleanup(func() { UpdateProtection(nil) })
	UpdateProtection([]truenas.ProtectionTask{{Kind: "rsync", State: "OK"}, {Kind: "snapshot", State: "OK"}})
	UpdateProtection([]truenas.ProtectionTask{{Kind: "snapshot", State: "OK"}})
	if n := seriesCount(t, protectionTasks); n != 6 {
		t.Errorf("task series = %d, want 6 once rsync is gone", n)
	}
	if n := seriesCount(t, protectionOverdue); n != 1 {
		t.Errorf("overdue series = %d, want 1", n)
	}
	UpdateProtection(nil)
	if n := seriesCount(t, protectionTasks); n != 0 {
		t.Errorf("no tasks should leave no series, got %d", n)
	}
}

func TestUpdatePoolsExportsVdevErrorsForGroupVdevsOnly(t *testing.T) {
	t.Cleanup(func() { UpdatePools(nil) })
	UpdatePools([]truenas.Pool{
		{Name: "tank", Vdevs: []truenas.Vdev{
			{Name: "mirror-0", Type: "MIRROR", Errors: &truenas.VdevErrors{Read: 1, Write: 2, Checksum: 3},
				Children: []truenas.VdevChild{{Disk: "sda"}, {Disk: "sdb"}}},
			{Name: "mirror-1", Type: "MIRROR", Children: []truenas.VdevChild{{Disk: "sdc"}}}, // no stats sent
			{Name: "raidz1-0", Type: "RAIDZ1", Errors: &truenas.VdevErrors{Checksum: 1},
				Children: []truenas.VdevChild{{Disk: "sde"}, {Disk: "sdf"}, {Disk: "sdg"}}}, // assumed TrueNAS naming, not seen on a real box
			{Name: "5f0c", Type: "DISK", Errors: &truenas.VdevErrors{Checksum: 9},
				Children: []truenas.VdevChild{{Disk: "sdd"}}}, // bare disk: disk_errors has it
		}},
	})
	for typ, want := range map[string]float64{"read": 1, "write": 2, "checksum": 3} {
		if got := gaugeValue(vdevErrors.WithLabelValues("tank", "mirror-0", typ)); got != want {
			t.Errorf("vdev errors{%s} = %v, want %v", typ, got, want)
		}
	}
	if got := gaugeValue(vdevErrors.WithLabelValues("tank", "raidz1-0", "checksum")); got != 1 {
		t.Errorf("raidz vdev checksum = %v, want 1", got)
	}
	if n := seriesCount(t, vdevErrors); n != 6 {
		t.Errorf("%d vdev error series, want 6 (mirror-0 and raidz1-0, three types each): no stats and bare disks must have none, not 0", n)
	}

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range families {
		found = found || f.GetName() == "naswarden_vdev_errors"
	}
	if !found {
		t.Error("naswarden_vdev_errors is not exposed: register it in init()")
	}

	UpdatePools([]truenas.Pool{{Name: "tank"}})
	if n := seriesCount(t, vdevErrors); n != 0 {
		t.Errorf("%d vdev error series left after the vdev went away", n)
	}
}

func TestScanProgressNeverExceedsOneHundred(t *testing.T) {
	t.Cleanup(func() { UpdatePools(nil) })
	pct := 100.0004 // what a real pool reported for a scrub
	UpdatePools([]truenas.Pool{{Name: "tank", Scan: &truenas.Scan{Function: "SCRUB", State: "IN_PROGRESS", Percentage: &pct}}})
	if got := gaugeValue(poolScanProgress.WithLabelValues("tank", "scrub")); got != 100 {
		t.Errorf("progress = %v, want 100", got)
	}
}
