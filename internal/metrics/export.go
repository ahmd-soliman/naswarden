// Package metrics exposes naswarden's data as Prometheus gauges, for
// anyone who already runs Prometheus/Grafana. Deliberately exposes raw
// used/quota byte counts, not a pre-computed percentage -- PromQL can
// compute the ratio itself, and predict_linear() over these raw gauges
// gives time-to-exhaustion forecasting for free, no custom trend logic
// needed in naswarden itself.
package metrics

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

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

	poolUsableBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_pool_usable_bytes",
		Help: "Usable capacity in bytes of a ZFS pool (used plus available, from its root dataset): without RAIDZ parity or the slop reservation.",
	}, []string{"pool"})

	poolAvailableBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_pool_available_bytes",
		Help: "Bytes still available for files in a ZFS pool (the root dataset's available space).",
	}, []string{"pool"})

	poolHealthy = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_pool_healthy",
		Help: "1 if the ZFS pool is healthy and ONLINE, 0 otherwise.",
	}, []string{"pool"})

	poolFragmentation = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_pool_fragmentation_percent",
		Help: "Free-space fragmentation of a ZFS pool, in percent. Absent when TrueNAS reports none.",
	}, []string{"pool"})

	poolScanInProgress = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_pool_scan_in_progress",
		Help: "1 while the pool's latest scan (scrub or resilver) is running, 0 once it is not. Absent for a pool that never scanned.",
	}, []string{"pool", "function"})

	poolScanProgress = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_pool_scan_progress_percent",
		Help: "How far the running scrub or resilver is, in percent. Only present while one is running and TrueNAS reports it.",
	}, []string{"pool", "function"})

	poolLastScrub = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_pool_last_scrub_timestamp_seconds",
		Help: "Unix time the pool's last scrub finished. Only present while the pool's latest scan is a finished scrub (a later resilver replaces it).",
	}, []string{"pool"})

	diskErrors = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_disk_errors",
		Help: "ZFS error count of a pool member disk since its pool was imported. type is read, write or checksum.",
	}, []string{"pool", "disk", "type"})

	diskReadRate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_disk_read_bytes_per_second",
		Help: "Bytes per second read from a pool member disk between the last two refreshes. Absent until two refreshes have seen it.",
	}, []string{"pool", "disk"})

	diskWriteRate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_disk_write_bytes_per_second",
		Help: "Bytes per second written to a pool member disk between the last two refreshes. Absent until two refreshes have seen it.",
	}, []string{"pool", "disk"})

	datasetSnapshotBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_dataset_snapshot_bytes",
		Help: "Bytes held only by snapshots of a ZFS dataset that has a quota or refquota.",
	}, []string{"dataset"})

	datasetCompressRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_dataset_compression_ratio",
		Help: "Compression ratio of a ZFS dataset that has a quota or refquota (1.5 means 1.5x). Absent when TrueNAS reports none.",
	}, []string{"dataset"})

	truenasInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_truenas_info",
		Help: "Always 1; the version label is the TrueNAS version NasWarden is connected to.",
	}, []string{"version"})

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

	replicationLastSuccess = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_replication_last_success_timestamp_seconds",
		Help: "Unix time the replication task last finished successfully, as seen by this process.",
	}, []string{"name"})

	serviceUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_service_up",
		Help: "1 if the TrueNAS service is running, 0 if not; only for services that are running, set to start on boot, or have enabled shares.",
	}, []string{"service"})

	lastRefreshSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "naswarden_last_refresh_success_timestamp_seconds",
		Help: "Unix time of the last refresh in which the core TrueNAS data (server, pools, datasets) was fetched.",
	})

	sourceStale = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "naswarden_source_stale",
		Help: "1 if the source's last refresh failed and its previous data is being served, else 0.",
	}, []string{"source"})
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
	datasetUsedSeries     = newTracked(datasetUsedBytes)
	datasetQuotaSeries    = newTracked(datasetQuotaBytes)
	poolAllocSeries       = newTracked(poolAllocatedBytes)
	poolSizeSeries        = newTracked(poolSizeBytes)
	poolUsableSeries      = newTracked(poolUsableBytes)
	poolAvailableSeries   = newTracked(poolAvailableBytes)
	poolHealthySeries     = newTracked(poolHealthy)
	instanceSeries        = newTracked(instanceUp)
	containerSeries       = newTracked(containerUp)
	diskTempSeries        = newTracked(diskTemperature)
	alertSeries           = newTracked(alertCount)
	replicationSeries     = newTracked(replicationTaskStatus)
	replicationLastSeries = newTracked(replicationLastSuccess)
	poolFragSeries        = newTracked(poolFragmentation)
	poolScanRunSeries     = newTracked(poolScanInProgress)
	poolScanPctSeries     = newTracked(poolScanProgress)
	poolLastScrubSeries   = newTracked(poolLastScrub)
	diskErrorSeries       = newTracked(diskErrors)
	diskReadRateSeries    = newTracked(diskReadRate)
	diskWriteRateSeries   = newTracked(diskWriteRate)
	datasetSnapSeries     = newTracked(datasetSnapshotBytes)
	datasetRatioSeries    = newTracked(datasetCompressRatio)
	truenasInfoSeries     = newTracked(truenasInfo)
	serviceSeries         = newTracked(serviceUp)
)

func init() {
	prometheus.MustRegister(
		datasetUsedBytes,
		datasetQuotaBytes,
		poolAllocatedBytes,
		poolSizeBytes,
		poolUsableBytes,
		poolAvailableBytes,
		poolHealthy,
		instanceUp,
		containerUp,
		diskTemperature,
		alertCount,
		replicationTaskStatus,
		replicationLastSuccess,
		poolFragmentation,
		poolScanInProgress,
		poolScanProgress,
		poolLastScrub,
		diskErrors,
		diskReadRate,
		diskWriteRate,
		datasetSnapshotBytes,
		datasetCompressRatio,
		truenasInfo,
		serviceUp,
		lastRefreshSuccess,
		sourceStale,
	)
}

// UpdateDatasets refreshes the dataset quota gauges.
func UpdateDatasets(datasets []truenas.Dataset) {
	setUsed, doneUsed := datasetUsedSeries.begin()
	setQuota, doneQuota := datasetQuotaSeries.begin()
	setSnap, doneSnap := datasetSnapSeries.begin()
	setRatio, doneRatio := datasetRatioSeries.begin()
	for _, d := range datasets {
		setUsed(float64(d.Used), d.Name)
		setQuota(float64(d.Quota), d.Name)
		setSnap(float64(d.UsedBySnapshots), d.Name)
		if r, ok := parseRatio(d.CompressRatio); ok {
			setRatio(r, d.Name)
		}
	}
	doneUsed()
	doneQuota()
	doneSnap()
	doneRatio()
}

// UpdatePoolUsable exports the usable capacity of the pools that have it.
// A pool whose root dataset could not be read has no series (never 0), and a
// series disappears when its pool does.
func UpdatePoolUsable(pools []truenas.Pool) {
	setUsable, doneUsable := poolUsableSeries.begin()
	setAvail, doneAvail := poolAvailableSeries.begin()
	for _, p := range pools {
		if p.UsableSize <= 0 {
			continue
		}
		setUsable(float64(p.UsableSize), p.Name)
		setAvail(float64(p.UsableAvailable), p.Name)
	}
	doneUsable()
	doneAvail()
}

// UpdatePools refreshes the ZFS pool capacity, health, and disk temperature gauges.
func UpdatePools(pools []truenas.Pool) {
	setAlloc, doneAlloc := poolAllocSeries.begin()
	setSize, doneSize := poolSizeSeries.begin()
	setHealthy, doneHealthy := poolHealthySeries.begin()
	setTemp, doneTemp := diskTempSeries.begin()
	setFrag, doneFrag := poolFragSeries.begin()
	setScanRun, doneScanRun := poolScanRunSeries.begin()
	setScanPct, doneScanPct := poolScanPctSeries.begin()
	setScrub, doneScrub := poolLastScrubSeries.begin()
	for _, p := range pools {
		setAlloc(float64(p.Allocated), p.Name)
		setSize(float64(p.Size), p.Name)
		val := 0.0
		if p.Healthy {
			val = 1.0
		}
		setHealthy(val, p.Name)

		if f, err := strconv.ParseFloat(strings.TrimSpace(p.Fragmentation), 64); err == nil {
			setFrag(f, p.Name)
		}
		if sc := p.Scan; sc != nil && sc.Function != "" {
			fn := strings.ToLower(sc.Function)
			running := sc.State == "IN_PROGRESS" || sc.State == "SCANNING"
			run := 0.0
			if running {
				run = 1.0
				if sc.Percentage != nil {
					setScanPct(*sc.Percentage, p.Name, fn)
				}
			}
			setScanRun(run, p.Name, fn)
			if sc.Function == "SCRUB" && sc.State == "FINISHED" && sc.EndTime != nil {
				setScrub(float64(*sc.EndTime), p.Name)
			}
		}

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
	doneFrag()
	doneScanRun()
	doneScanPct()
	doneScrub()
}

// UpdateDisks exports the ZFS error counts and I/O rates of pool member
// disks. Only members have them; a rate is left out until two refreshes have
// seen the disk, never exported as 0.
func UpdateDisks(disks []truenas.Disk) {
	setErr, doneErr := diskErrorSeries.begin()
	setRead, doneRead := diskReadRateSeries.begin()
	setWrite, doneWrite := diskWriteRateSeries.begin()
	for _, d := range disks {
		if d.Pool == "" || d.ReadBytes == nil {
			continue // not a pool member: ZFS has no counters for it
		}
		setErr(float64(d.ReadErrors), d.Pool, d.Name, "read")
		setErr(float64(d.WriteErrors), d.Pool, d.Name, "write")
		setErr(float64(d.ChecksumErrors), d.Pool, d.Name, "checksum")
		if d.ReadRate != nil {
			setRead(*d.ReadRate, d.Pool, d.Name)
		}
		if d.WriteRate != nil {
			setWrite(*d.WriteRate, d.Pool, d.Name)
		}
	}
	doneErr()
	doneRead()
	doneWrite()
}

// UpdateServer exports the TrueNAS version as an info metric.
func UpdateServer(s *truenas.ServerInfo) {
	set, done := truenasInfoSeries.begin()
	if s.Version != "" {
		set(1, s.Version)
	}
	done()
}

// parseRatio reads a TrueNAS compression ratio such as "1.08x".
func parseRatio(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "x"), 64)
	return v, err == nil && v > 0
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

// lastSuccess remembers, per task, when it last finished successfully. The
// API only reports the most recent run, so once a run fails the previous
// success would otherwise be forgotten and "no success for N hours" could
// never fire. Only the refresh goroutine touches it.
var lastSuccess = map[string]float64{}

// UpdateReplications refreshes the ZFS replication task gauges.
func UpdateReplications(tasks []truenas.ReplicationTask) {
	set, done := replicationSeries.begin()
	setOK, doneOK := replicationLastSeries.begin()
	live := map[string]struct{}{}
	for _, t := range tasks {
		if !t.Enabled {
			continue
		}
		live[t.Name] = struct{}{}
		taskID := fmt.Sprintf("%d", t.ID)
		set(1.0, taskID, t.Name, t.TargetPool, t.State, t.JobState)
		if t.JobState == "SUCCESS" && t.TimeFinished != nil {
			lastSuccess[t.Name] = float64(*t.TimeFinished)
		}
	}
	for name, ts := range lastSuccess {
		if _, ok := live[name]; !ok {
			delete(lastSuccess, name) // task deleted or disabled
			continue
		}
		setOK(ts, name)
	}
	done()
	doneOK()
}

// UpdateServices refreshes naswarden_service_up. Only the service name (a
// fixed middleware identifier) is a label; shares are never exported.
func UpdateServices(services []truenas.Service) {
	set, done := serviceSeries.begin()
	for _, s := range services {
		v := 0.0
		if s.State == "RUNNING" {
			v = 1.0
		}
		set(v, s.Name)
	}
	done()
}

// KnownSources are the optional data sources whose staleness is exported.
var KnownSources = []string{"docker", "truenas-vms", "incus", "disks", "alerts", "replication", "services", "shares"}

// UpdateRefresh records a completed refresh: when it happened and which
// sources are currently serving their previous snapshot.
func UpdateRefresh(now time.Time, stale []string) {
	lastRefreshSuccess.Set(float64(now.Unix()))
	isStale := make(map[string]bool, len(stale))
	for _, s := range stale {
		isStale[s] = true
	}
	for _, src := range KnownSources {
		v := 0.0
		if isStale[src] {
			v = 1.0
		}
		sourceStale.WithLabelValues(src).Set(v)
	}
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
