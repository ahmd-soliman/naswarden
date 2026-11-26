package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ahmd-soliman/naswarden/internal/buildinfo"
	"github.com/ahmd-soliman/naswarden/internal/docker"
	"github.com/ahmd-soliman/naswarden/internal/incus"
	"github.com/ahmd-soliman/naswarden/internal/metrics"
	"github.com/ahmd-soliman/naswarden/internal/truenas"
	"github.com/ahmd-soliman/naswarden/internal/vm"
	"github.com/ahmd-soliman/naswarden/internal/web"
	"github.com/ahmd-soliman/naswarden/internal/ws"
)

const refreshInterval = 60 * time.Second

func main() {
	buildinfo.Register()
	slog.Info("naswarden starting", "version", buildinfo.Version, "commit", buildinfo.Commit)

	host := os.Getenv("TRUENAS_HOST")
	apiKey := os.Getenv("TRUENAS_API_KEY")
	if host == "" || apiKey == "" {
		slog.Error("TRUENAS_HOST and TRUENAS_API_KEY must both be set")
		os.Exit(1)
	}
	host, useTLS, hostWarnings, err := truenasAddress(host, os.Getenv("TRUENAS_TLS"))
	if err != nil {
		slog.Error("invalid TrueNAS address", "err", err)
		os.Exit(1)
	}
	for _, w := range hostWarnings {
		slog.Warn("TRUENAS_HOST is not plain host:port", "note", w)
	}
	insecureTLS := os.Getenv("TRUENAS_INSECURE_TLS") == "true"
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// The connection is established lazily and retried on every refresh: the
	// app must come up (and say why it has no data) even when TrueNAS is not
	// ready yet, e.g. right after a boot or with a wrong host or API key.
	client := truenas.NewClient(host, apiKey, useTLS, insecureTLS)
	defer client.Close()
	slog.Info("will connect to TrueNAS", "host", host)

	// Stacks and containers are optional: see dockerAddress for where the
	// Docker API is found. naswarden only sends GET requests; a proxy
	// enforces that, a mounted socket does not. A bad value or an unusable
	// socket is reported once here, not as a failed refresh every minute; the
	// rest of the dashboard runs without Docker.
	var dockerClient *docker.Client
	dockerVar, dockerHost, warning, err := dockerAddress(os.Getenv, docker.IsSocket)
	if warning != "" {
		slog.Warn(warning, "docker_proxy_url", os.Getenv("DOCKER_PROXY_URL"), "docker_host", os.Getenv("DOCKER_HOST"))
	}
	if path, ok := strings.CutPrefix(dockerHost, "unix://"); ok && err == nil {
		err = docker.CheckSocket(path)
	}
	switch {
	case err != nil:
		slog.Error("docker container stats disabled", "err", err)
	case dockerHost != "":
		dockerClient = docker.NewClient(dockerHost)
		slog.Info("docker container stats enabled", "from", dockerVar, "address", dockerHost)
	}
	dockerStat := newDockerStatus(dockerVar, dockerHost, err)

	// Incus VM & container monitoring is optional -- naswarden runs fine
	// without it if INCUS_URL or credentials aren't set.
	var incusClient *incus.Client
	incusURL := os.Getenv("INCUS_URL")
	incusCert := os.Getenv("INCUS_CLIENT_CERT")
	if incusCert == "" {
		incusCert = os.Getenv("INCUS_CLIENT_CRT")
	}
	incusKey := os.Getenv("INCUS_CLIENT_KEY")
	insecureIncusTLS := os.Getenv("INCUS_INSECURE_TLS") != "false"

	if incusURL != "" && incusCert != "" && incusKey != "" {
		c, err := incus.NewClient(incusURL, incusCert, incusKey, insecureIncusTLS)
		if err != nil {
			slog.Warn("failed to initialize Incus client", "err", err)
		} else {
			incusClient = c
			slog.Info("incus instance monitoring enabled", "url", incusURL)
		}
	}

	hub := ws.NewHub()
	health := newHealth(refreshInterval * 3)
	cache := &lastGood{docker: dockerStat}
	diskRates := truenas.NewDiskRates()

	// Refresh loop: poll TrueNAS, Docker, and Incus, push to every connected
	// client. Decoupled from any client's own connection lifecycle or Prometheus
	// scrape interval -- one internal cadence, fan out to everyone.
	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for {
			refresh(client, dockerClient, incusClient, hub, cache, health, diskRates)
			<-ticker.C
		}
	}()

	uiHandler, err := web.Handler()
	if err != nil {
		slog.Error("failed to load embedded UI", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeHTTP)
	mux.Handle("/healthz", health)
	mux.Handle("/version", buildinfo.Handler())
	mux.Handle("/", uiHandler)
	mux.Handle("/metrics", promhttp.Handler())

	slog.Info("listening", "port", port)
	// No Read/WriteTimeout: /ws is a long-lived push connection. Header and
	// idle timeouts still shut out slow-loris clients.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           newHostGuard(os.Getenv("NASWARDEN_ALLOWED_HOSTS"), mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

// refresh fetches everything naswarden tracks in one pass: pushed to
// WebSocket clients as a single combined message (so the UI always
// renders a consistent snapshot, not pools, datasets, and instances from
// different refresh moments), and separately fed into the Prometheus
// gauges for /metrics.
func refresh(client *truenas.Client, dockerClient *docker.Client, incusClient *incus.Client, hub *ws.Hub, cache *lastGood, health *healthState, diskRates *truenas.DiskRates) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server, err := truenas.GetServerInfo(ctx, client)
	if err != nil {
		slog.Error("failed to refresh server info", "err", err)
		notifyError(hub, "server info", err)
		return
	}

	pools, err := truenas.ListPools(ctx, client)
	if err != nil {
		slog.Error("failed to refresh pools", "err", err)
		notifyError(hub, "pools", err)
		return
	}
	metrics.UpdatePools(pools)

	datasets, err := truenas.ListDatasets(ctx, client)
	if err != nil {
		slog.Error("failed to refresh datasets", "err", err)
		notifyError(hub, "datasets", err)
		return
	}
	metrics.UpdateDatasets(datasets)

	// Docker is optional and refreshed best-effort -- a failure here
	// (proxy briefly unreachable) shouldn't take down pool/dataset
	// reporting, which is why this doesn't early-return like the TrueNAS
	// calls above.
	var stale []string
	containers := cache.containers
	if dockerClient != nil {
		fresh, err := dockerClient.ListContainers(ctx)
		if err != nil {
			slog.Error("failed to refresh containers", "err", err)
			stale = append(stale, "docker")
		} else {
			containers = fresh
			cache.containers = fresh
		}
	}
	metrics.UpdateContainers(containers)

	// Virtual machines and containers from TrueNAS Native and Incus. Like
	// Docker, a failed source keeps its last good list (flagged stale)
	// rather than emptying -- an outage of one backend must not read as
	// "every VM was deleted" in the UI or in the Prometheus gauges.
	truenasVMs := cache.truenasVMs
	if fresh, err := truenas.ListVMs(ctx, client); err != nil {
		slog.Error("failed to refresh truenas vms", "err", err)
		stale = append(stale, "truenas-vms")
	} else {
		truenasVMs = fresh
		cache.truenasVMs = fresh
	}

	incusInstances := cache.incus
	if incusClient != nil {
		if fresh, err := incusClient.ListInstances(ctx); err != nil {
			slog.Error("failed to refresh incus instances", "err", err)
			stale = append(stale, "incus")
		} else {
			incusInstances = fresh
			cache.incus = fresh
		}
	}

	vms := make([]vm.Instance, 0, len(truenasVMs)+len(incusInstances))
	vms = append(vms, truenasVMs...)
	vms = append(vms, incusInstances...)
	vm.Sort(vms)
	metrics.UpdateInstances(vms)

	// Physical disks (identity, pool, temperature range, ZFS rates): same
	// last-good rule as the other optional sources.
	disks := cache.disks
	if fresh, err := truenas.ListDisks(ctx, client, diskRates); err != nil {
		slog.Error("failed to refresh disks", "err", err)
		stale = append(stale, "disks")
	} else {
		disks = fresh
		cache.disks = fresh
	}
	if disks == nil {
		disks = []truenas.Disk{}
	}

	// TrueNAS alerts and replication tasks: like the other optional
	// sources, a failed call keeps the last good list (flagged stale)
	// instead of blanking the UI and the metrics.
	alerts := cache.alerts
	if fresh, err := truenas.ListAlerts(ctx, client); err != nil {
		slog.Error("failed to refresh truenas alerts", "err", err)
		stale = append(stale, "alerts")
	} else {
		alerts = fresh
		cache.alerts = fresh
	}
	if alerts == nil {
		alerts = []truenas.Alert{}
	}
	metrics.UpdateAlerts(alerts)

	replications := cache.replications
	if fresh, err := truenas.ListReplications(ctx, client); err != nil {
		slog.Error("failed to refresh truenas replications", "err", err)
		stale = append(stale, "replication")
	} else {
		replications = fresh
		cache.replications = fresh
	}
	if replications == nil {
		replications = []truenas.ReplicationTask{}
	}
	metrics.UpdateReplications(replications)

	payload, err := json.Marshal(map[string]any{
		"type": "state",
		// When this snapshot was taken. The UI shows its age and flags it
		// stale: if a refresh fails nothing is broadcast, so without this a
		// connected browser would keep showing old numbers under a "live"
		// indicator with no way to tell.
		"updated_at": time.Now().Unix(),
		// Sources whose data is the previous snapshot because this refresh
		// failed ("docker", "incus", "truenas-vms", ...).
		"stale_sources": append([]string{}, stale...),
		// Whether Docker is available and how (or why not), for the footer
		// note and the "Docker not connected" card.
		"docker":       cache.docker,
		"server":       server,
		"pools":        pools,
		"datasets":     datasets,
		"containers":   containers,
		"vms":          vms,
		"disks":        disks,
		"alerts":       alerts,
		"replications": replications,
	})
	if err != nil {
		slog.Error("failed to marshal state payload", "err", err)
		return
	}
	hub.Broadcast(payload)
	health.markOK()
	metrics.UpdateRefresh(time.Now(), stale)
}

// lastGood holds the previous successful result of each optional source.
// Only the refresh goroutine touches it.
type lastGood struct {
	containers   []docker.Container
	truenasVMs   []vm.Instance
	incus        []vm.Instance
	disks        []truenas.Disk
	alerts       []truenas.Alert
	replications []truenas.ReplicationTask
	docker       dockerStatus // fixed at startup; sent with every state
}

// notifyError tells connected browsers why the core TrueNAS data could not be
// refreshed, so a wrong host, wrong API key or a TrueNAS that is still booting
// shows a reason instead of an empty page.
func notifyError(hub *ws.Hub, what string, err error) {
	payload, mErr := json.Marshal(map[string]any{
		"type":    "error",
		"message": fmt.Sprintf("could not refresh %s from TrueNAS: %v", what, err),
		// The likely fix, when the error is a recognisable one (wrong port,
		// untrusted certificate, rejected key, ...); "" otherwise.
		"hint": truenas.Hint(err),
	})
	if mErr != nil {
		return
	}
	hub.BroadcastNotice(payload)
}
