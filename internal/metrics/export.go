// Package metrics exposes naswarden's data as Prometheus gauges, for
// anyone who already runs Prometheus/Grafana. Deliberately exposes raw
// used/quota byte counts, not a pre-computed percentage -- PromQL can
// compute the ratio itself, and predict_linear() over these raw gauges
// gives time-to-exhaustion forecasting for free, no custom trend logic
// needed in naswarden itself.
package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ahmd-soliman/naswarden/internal/docker"
	"github.com/ahmd-soliman/naswarden/internal/truenas"
	"github.com/ahmd-soliman/naswarden/internal/vm"
)

var (
	datasetUsedBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_dataset_quota_used_bytes",
		Help: "Bytes used on a ZFS dataset that has a quota or refquota configured.",
	}, []string{"dataset"})

	datasetQuotaBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_dataset_quota_bytes",
		Help: "The configured quota/refquota, in bytes, for a ZFS dataset.",
	}, []string{"dataset"})

	poolAllocatedBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_pool_allocated_bytes",
		Help: "Allocated storage space in bytes for a ZFS storage pool.",
	}, []string{"pool"})

	poolSizeBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_pool_size_bytes",
		Help: "Total raw storage size in bytes for a ZFS storage pool.",
	}, []string{"pool"})

	poolHealthy = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_pool_healthy",
		Help: "1 if the ZFS pool is healthy and ONLINE, 0 otherwise.",
	}, []string{"pool"})

	instanceUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_instance_up",
		Help: "1 if the virtual machine or LXC container is running, 0 otherwise.",
	}, []string{"instance", "manager", "type"})

	containerUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_container_up",
		Help: "1 if the Docker container is running, 0 otherwise.",
	}, []string{"container", "stack"})
)

func init() {
	prometheus.MustRegister(
		datasetUsedBytes,
		datasetQuotaBytes,
		poolAllocatedBytes,
		poolSizeBytes,
		poolHealthy,
		instanceUp,
		containerUp,
	)
}

// UpdateDatasets refreshes the dataset quota gauges.
func UpdateDatasets(datasets []truenas.Dataset) {
	datasetUsedBytes.Reset()
	datasetQuotaBytes.Reset()
	for _, d := range datasets {
		datasetUsedBytes.WithLabelValues(d.Name).Set(float64(d.Used))
		datasetQuotaBytes.WithLabelValues(d.Name).Set(float64(d.Quota))
	}
}

// UpdatePools refreshes the ZFS pool capacity and health gauges.
func UpdatePools(pools []truenas.Pool) {
	poolAllocatedBytes.Reset()
	poolSizeBytes.Reset()
	poolHealthy.Reset()
	for _, p := range pools {
		poolAllocatedBytes.WithLabelValues(p.Name).Set(float64(p.Allocated))
		poolSizeBytes.WithLabelValues(p.Name).Set(float64(p.Size))
		val := 0.0
		if p.Healthy {
			val = 1.0
		}
		poolHealthy.WithLabelValues(p.Name).Set(val)
	}
}

// UpdateInstances refreshes the VM and LXC container status gauges.
func UpdateInstances(instances []vm.Instance) {
	instanceUp.Reset()
	for _, inst := range instances {
		val := 0.0
		if strings.EqualFold(inst.Status, "running") {
			val = 1.0
		}
		instanceUp.WithLabelValues(inst.Name, inst.Manager, inst.Type).Set(val)
	}
}

// UpdateContainers refreshes the Docker container status gauges.
func UpdateContainers(containers []docker.Container) {
	containerUp.Reset()
	for _, c := range containers {
		val := 0.0
		if strings.EqualFold(c.State, "running") {
			val = 1.0
		}
		containerUp.WithLabelValues(c.Name, c.Stack).Set(val)
	}
}
