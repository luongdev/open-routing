package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMetricsHandler_Returns200 locks in the Phase 1 stub shape for /metrics:
// empty body, 200 status. When a real exporter lands the body assertion stays
// trivially true (empty is a subset of any non-empty exporter output) so the
// test does not need updating.
func TestMetricsHandler_Returns200(t *testing.T) {
	t.Parallel()
	h := MetricsHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}
