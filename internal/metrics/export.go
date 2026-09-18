// Package metrics exposes naswarden's data as Prometheus gauges, for
// anyone who already runs Prometheus/Grafana. Deliberately exposes raw
// used/quota byte counts, not a pre-computed percentage -- PromQL can
// compute the ratio itself, and predict_linear() over these raw gauges
// gives time-to-exhaustion forecasting for free, no custom trend logic
// needed in naswarden itself.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ahmd-soliman/naswarden/internal/truenas"
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
)

func init() {
	prometheus.MustRegister(datasetUsedBytes, datasetQuotaBytes)
}

// UpdateDatasets refreshes the gauges to match exactly the given dataset
// list -- any dataset previously reported but no longer present (quota
// removed, dataset deleted) is dropped rather than left stale.
func UpdateDatasets(datasets []truenas.Dataset) {
	datasetUsedBytes.Reset()
	datasetQuotaBytes.Reset()
	for _, d := range datasets {
		datasetUsedBytes.WithLabelValues(d.Name).Set(float64(d.Used))
		datasetQuotaBytes.WithLabelValues(d.Name).Set(float64(d.Quota))
	}
}
