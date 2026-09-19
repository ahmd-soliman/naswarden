// Package buildinfo carries the version stamped into the binary at build time
// (see the Dockerfile) and exposes it over HTTP and as a Prometheus metric, so
// a running instance can say which build it is.
package buildinfo

import (
	"encoding/json"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

// Set with -ldflags "-X .../buildinfo.Version=1.0.0" etc.; a plain `go build`
// reports "dev".
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Info is the JSON served at /version.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Current returns the stamped build info.
func Current() Info { return Info{Version: Version, Commit: Commit, Date: Date} }

// Handler serves Current() as JSON.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(Current())
	})
}

var buildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "naswarden_build_info",
	Help: "Build information of the running naswarden; the value is always 1.",
}, []string{"version", "commit"})

// Register publishes naswarden_build_info. Call once at startup, after the
// build variables are final.
func Register() {
	prometheus.MustRegister(buildInfo)
	buildInfo.WithLabelValues(Version, Commit).Set(1)
}
