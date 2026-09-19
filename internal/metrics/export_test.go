package metrics

import (
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
		{ID: 24, Name: "tank_BKP", TargetPool: "backup", State: "FINISHED", JobState: "SUCCESS", Enabled: true},
	}
	UpdateReplications(tasks)

	if val := gaugeValue(replicationTaskStatus.WithLabelValues("24", "tank_BKP", "backup", "FINISHED", "SUCCESS")); val != 1.0 {
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
