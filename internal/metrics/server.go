package metrics

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// StartMetricsServer starts a background HTTP server to expose Prometheus metrics.
// This is non-blocking (runs in a goroutine).
func StartMetricsServer(addr string) {
	if addr == "" {
		addr = ":9090"
	}

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		slog.Info("Starting metrics server", "addr", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			slog.Error("metrics server failed", "error", err)
		}
	}()
}
