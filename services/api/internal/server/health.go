// Package server: health.go currently owns only the Phase 1 /metrics
// placeholder. Liveness (/healthz) and readiness (/readyz) bodies are now
// served by catalog.Handlers's strict-server bypass methods (D-69) — the
// chi root no longer hangs raw handlers for those paths.
package server

import "net/http"

// MetricsHandler is the Phase 1 placeholder for /metrics (D-17 + Research
// Open Question #4). Mounted as a bare chi route at root because /metrics
// is NOT in the OpenAPI spec and therefore never reaches the strict-server
// pipeline. Phase 4+ may replace this with a real otelhttp / Prometheus
// exporter registration; the route signature is stable.
func MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(""))
	}
}
