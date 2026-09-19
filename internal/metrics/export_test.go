package metrics

import (
	"testing"

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
		{Name: "P1/media", Used: 1000, Quota: 5000},
		{Name: "P2/backup", Used: 2500, Quota: 3000},
	}
	UpdateDatasets(datasets)

	if val := gaugeValue(datasetUsedBytes.WithLabelValues("P1/media")); val != 1000 {
		t.Errorf("expected 1000, got %f", val)
	}
	if val := gaugeValue(datasetQuotaBytes.WithLabelValues("P1/media")); val != 5000 {
		t.Errorf("expected 5000, got %f", val)
	}

	UpdateDatasets(nil)
}

func TestUpdatePools(t *testing.T) {
	pools := []truenas.Pool{
		{Name: "P1", Healthy: true, Allocated: 500000, Size: 1000000},
		{Name: "S_BKP", Healthy: false, Allocated: 200000, Size: 800000},
	}
	UpdatePools(pools)

	if val := gaugeValue(poolAllocatedBytes.WithLabelValues("P1")); val != 500000 {
		t.Errorf("expected 500000, got %f", val)
	}
	if val := gaugeValue(poolSizeBytes.WithLabelValues("P1")); val != 1000000 {
		t.Errorf("expected 1000000, got %f", val)
	}
	if val := gaugeValue(poolHealthy.WithLabelValues("P1")); val != 1.0 {
		t.Errorf("expected 1.0 healthy, got %f", val)
	}
	if val := gaugeValue(poolHealthy.WithLabelValues("S_BKP")); val != 0.0 {
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
