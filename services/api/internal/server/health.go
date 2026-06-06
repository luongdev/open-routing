// Package server: health.go owns the /metrics route. Liveness (/healthz) and
// readiness (/readyz) bodies are served by catalog.Handlers's strict-server
// bypass methods (D-69) — the chi root no longer hangs raw handlers for those.
package server

import (
	"net/http"

	"github.com/luongdev/open-routing/services/api/internal/metrics"
)

// MetricsHandler serves the process metric registry in Prometheus text format
// (v0.3 W6). Mounted as a bare chi route at root because /metrics is NOT in the
// OpenAPI spec and therefore never reaches the strict-server pipeline. A real
// otelhttp / Prometheus exporter remains an additive swap; the route signature
// is stable.
func MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		metrics.Default.WritePrometheus(w)
	}
}
