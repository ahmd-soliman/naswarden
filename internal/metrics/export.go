// Package metrics exposes naswarden's data as Prometheus gauges, for
// anyone who already runs Prometheus/Grafana. Deliberately exposes raw
// used/quota byte counts, not a pre-computed percentage -- PromQL can
// compute the ratio itself, and predict_linear() over these raw gauges
// gives time-to-exhaustion forecasting for free, no custom trend logic
// needed in naswarden itself.
package metrics

import (
	"fmt"
	"strings"
	"sync"

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

	diskTemperature = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_disk_temperature_celsius",
		Help: "Current drive temperature in Celsius, if monitored and active.",
	}, []string{"pool", "disk"})

	alertCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_truenas_alert_count",
		Help: "Count of active un-dismissed TrueNAS alerts by severity level.",
	}, []string{"level"})

	replicationTaskStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_replication_task_status",
		Help: "ZFS replication task state (1 for active state).",
	}, []string{"task_id", "name", "target_pool", "state", "job_state"})
)

// tracked remembers which label sets a gauge vector currently holds, so a
// refresh can delete just the series that disappeared. Reset()-then-refill
// left a window where a Prometheus scrape saw no series at all, which reads
// as every pool/container/instance vanishing.
type tracked struct {
	vec  *prometheus.GaugeVec
	mu   sync.Mutex
	last map[string][]string
}

func newTracked(vec *prometheus.GaugeVec) *tracked {
	return &tracked{vec: vec, last: map[string][]string{}}
}

// begin returns a setter for one refresh; call done() after the last set.
func (t *tracked) begin() (set func(v float64, labels ...string), done func()) {
	cur := map[string][]string{}
	set = func(v float64, labels ...string) {
		t.vec.WithLabelValues(labels...).Set(v)
		cur[strings.Join(labels, "\x00")] = labels
	}
	done = func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		for k, labels := range t.last {
			if _, ok := cur[k]; !ok {
				t.vec.DeleteLabelValues(labels...)
			}
		}
		t.last = cur
	}
	return set, done
}

var (
	datasetUsedSeries  = newTracked(datasetUsedBytes)
	datasetQuotaSeries = newTracked(datasetQuotaBytes)
	poolAllocSeries    = newTracked(poolAllocatedBytes)
	poolSizeSeries     = newTracked(poolSizeBytes)
	poolHealthySeries  = newTracked(poolHealthy)
	instanceSeries     = newTracked(instanceUp)
	containerSeries    = newTracked(containerUp)
	diskTempSeries     = newTracked(diskTemperature)
	alertSeries        = newTracked(alertCount)
	replicationSeries  = newTracked(replicationTaskStatus)
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
		diskTemperature,
		alertCount,
		replicationTaskStatus,
	)
}

// UpdateDatasets refreshes the dataset quota gauges.
func UpdateDatasets(datasets []truenas.Dataset) {
	setUsed, doneUsed := datasetUsedSeries.begin()
	setQuota, doneQuota := datasetQuotaSeries.begin()
	for _, d := range datasets {
		setUsed(float64(d.Used), d.Name)
		setQuota(float64(d.Quota), d.Name)
	}
	doneUsed()
	doneQuota()
}

// UpdatePools refreshes the ZFS pool capacity, health, and disk temperature gauges.
func UpdatePools(pools []truenas.Pool) {
	setAlloc, doneAlloc := poolAllocSeries.begin()
	setSize, doneSize := poolSizeSeries.begin()
	setHealthy, doneHealthy := poolHealthySeries.begin()
	setTemp, doneTemp := diskTempSeries.begin()
	for _, p := range pools {
		setAlloc(float64(p.Allocated), p.Name)
		setSize(float64(p.Size), p.Name)
		val := 0.0
		if p.Healthy {
			val = 1.0
		}
		setHealthy(val, p.Name)

		for _, vd := range p.Vdevs {
			for _, child := range vd.Children {
				if child.Temperature != nil {
					setTemp(*child.Temperature, p.Name, child.Disk)
				}
			}
		}
	}
	doneAlloc()
	doneSize()
	doneHealthy()
	doneTemp()
}

// UpdateAlerts refreshes the TrueNAS alert count gauges.
func UpdateAlerts(alerts []truenas.Alert) {
	set, done := alertSeries.begin()
	counts := make(map[string]float64)
	for _, a := range alerts {
		if !a.Dismissed {
			counts[a.Level]++
		}
	}
	for lvl, cnt := range counts {
		set(cnt, lvl)
	}
	done()
}

// UpdateReplications refreshes the ZFS replication task status gauges.
func UpdateReplications(tasks []truenas.ReplicationTask) {
	set, done := replicationSeries.begin()
	for _, t := range tasks {
		if t.Enabled {
			taskID := fmt.Sprintf("%d", t.ID)
			set(1.0, taskID, t.Name, t.TargetPool, t.State, t.JobState)
		}
	}
	done()
}

// UpdateInstances refreshes the VM and LXC container status gauges.
func UpdateInstances(instances []vm.Instance) {
	set, done := instanceSeries.begin()
	for _, inst := range instances {
		val := 0.0
		if strings.EqualFold(inst.Status, "running") {
			val = 1.0
		}
		set(val, inst.Name, inst.Manager, inst.Type)
	}
	done()
}

// UpdateContainers refreshes the Docker container status gauges.
func UpdateContainers(containers []docker.Container) {
	set, done := containerSeries.begin()
	for _, c := range containers {
		val := 0.0
		if strings.EqualFold(c.State, "running") {
			val = 1.0
		}
		set(val, c.Name, c.Stack)
	}
	done()
}
