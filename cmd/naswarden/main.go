package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ahmd-soliman/naswarden/internal/metrics"
	"github.com/ahmd-soliman/naswarden/internal/truenas"
	"github.com/ahmd-soliman/naswarden/internal/web"
	"github.com/ahmd-soliman/naswarden/internal/ws"
)

func main() {
	host := os.Getenv("TRUENAS_HOST")
	apiKey := os.Getenv("TRUENAS_API_KEY")
	if host == "" || apiKey == "" {
		slog.Error("TRUENAS_HOST and TRUENAS_API_KEY must both be set")
		os.Exit(1)
	}
	useTLS := os.Getenv("TRUENAS_TLS") == "true"
	insecureTLS := os.Getenv("TRUENAS_INSECURE_TLS") == "true"
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	client, err := truenas.Connect(ctx, host, apiKey, useTLS, insecureTLS)
	cancel()
	if err != nil {
		slog.Error("failed to connect to TrueNAS", "err", err)
		os.Exit(1)
	}
	defer client.Close()
	slog.Info("connected to TrueNAS", "host", host)

	hub := ws.NewHub()

	// Refresh loop: poll TrueNAS, push to every connected client. Decoupled
	// from any client's own connection lifecycle or Prometheus scrape
	// interval -- one internal cadence, fan out to everyone.
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			refresh(client, hub)
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
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/", uiHandler)
	mux.Handle("/metrics", promhttp.Handler())

	slog.Info("listening", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

// refresh fetches everything naswarden tracks in one pass: pushed to
// WebSocket clients as a single combined message (so the UI always
// renders a consistent snapshot, not pools and datasets from two
// different refresh moments), and separately fed into the Prometheus
// gauges for /metrics.
func refresh(client *truenas.Client, hub *ws.Hub) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pools, err := truenas.ListPools(ctx, client)
	if err != nil {
		slog.Error("failed to refresh pools", "err", err)
		return
	}

	datasets, err := truenas.ListDatasets(ctx, client)
	if err != nil {
		slog.Error("failed to refresh datasets", "err", err)
		return
	}
	metrics.UpdateDatasets(datasets)

	payload, err := json.Marshal(map[string]any{
		"type":     "state",
		"pools":    pools,
		"datasets": datasets,
	})
	if err != nil {
		slog.Error("failed to marshal state payload", "err", err)
		return
	}
	hub.Broadcast(payload)
}
