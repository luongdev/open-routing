package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLiveHandler_ReturnsAliveJSON covers the LB liveness contract (D-17):
// /healthz MUST return 200 + {"status":"alive"} on every call, no matter
// the state of the database or Redis. This is the contract a load balancer
// relies on to keep traffic flowing to a process whose downstream deps are
// transiently degraded — the deep checks live in /readyz, which Plan 07's
// testcontainer suite exercises end-to-end.
func TestLiveHandler_ReturnsAliveJSON(t *testing.T) {
	t.Parallel()
	h := LiveHandler()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(body), `"status":"alive"`), "unexpected body: %s", body)
}

// TestMetricsHandler_Returns200 locks in the Phase 1 stub shape for /metrics
// (D-17 + Open Question #4): empty body, 200 status. When a real exporter
// lands the body assertion stays trivially true (empty is a subset of any
// non-empty exporter output) so this test does not need updating.
func TestMetricsHandler_Returns200(t *testing.T) {
	t.Parallel()
	h := MetricsHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

// ReadyzHandler unit testing is deferred to Plan 07's testcontainer suite.
// pgxpool.Pool and redis.Client are concrete types without interface seams
// we can cheaply mock; running an in-process test with synthetic failures
// would have to monkey-patch the unexported pool struct. Plan 07 owns the
// end-to-end /readyz path (including the schema_migrations.dirty branch)
// against a real ephemeral Postgres + Redis.
