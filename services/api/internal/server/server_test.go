package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNewMux_HealthzAccessibleWithoutOrgHeader proves the LOCKED middleware
// chain in NewMux keeps /healthz reachable without an X-Org-Id header. This
// is the contract D-21 codifies (bypass paths skip OrgContext) and that
// Plan 07's TestBypassPaths_NoHeaderRequired will assert against the full
// testcontainer-backed mux. Here we run the unit-level form with a nil Pool
// and nil Redis because GetHealthz (the compositeServer bypass method) does
// not touch those dependencies.
//
// The test constructs a compositeServer with nil Pool / nil Redis / nil OrgDB
// to prove that the bypass path /healthz literally does not depend on those
// dependencies — a regression that routes /healthz through OrgContext or
// touches the pool here would panic immediately on nil deref.
func TestNewMux_HealthzAccessibleWithoutOrgHeader(t *testing.T) {
	t.Parallel()

	// compositeServer.GetHealthz only returns the static alive response —
	// it does not touch pool, redis, or orgDB. Passing nil for all three
	// proves /healthz is truly dependency-free for the live path.
	// Phase 3 Wave 0 (Plan 03-01 D-77): NewCompositeServer was deleted with
	// the Phase 2 scaffold. Wave0TempStubs replaces it as the
	// StrictServerInterface implementation; nil pool/redis is safe for these
	// bypass-path tests because GetHealthz / GetReadyz / metrics handler do
	// not touch the deps when the test only exercises /healthz or /metrics
	// (each test that uses /readyz must supply non-nil pool + redis).
	strictHandlers := NewWave0TempStubs(nil, nil, nil)
	deps := &Deps{
		Pool:           nil, // safe — GetHealthz does not touch the pool
		Redis:          nil, // safe — GetHealthz does not touch redis
		OrgDB:          nil, // safe — bypass paths do not reach scaffold handlers
		Config:         nil, // unused by bypass paths
		StrictHandlers: strictHandlers,
		SpecBytes:      nil,
	}
	mux := NewMux(deps)

	// /healthz — no X-Org-Id header, expect 200 + alive shape.
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "healthz must succeed without X-Org-Id (D-21)")
	require.Contains(t, rec.Body.String(), `"status":"alive"`, "healthz body shape (D-17)")

	// X-Request-Id response header must be set (D-28: RequestID middleware
	// runs before bypass route handlers).
	require.NotEmpty(t, rec.Header().Get("X-Request-Id"), "RequestID middleware must populate X-Request-Id")
}

// TestNewMux_MetricsAccessibleWithoutOrgHeader covers the /metrics bypass
// path with the same nil-dep harness. /metrics is a Phase 1 stub but the
// route still routes through Recoverer + RequestID at root.
func TestNewMux_MetricsAccessibleWithoutOrgHeader(t *testing.T) {
	t.Parallel()

	// Phase 3 Wave 0 (Plan 03-01 D-77): NewCompositeServer was deleted with
	// the Phase 2 scaffold. Wave0TempStubs replaces it as the
	// StrictServerInterface implementation; nil pool/redis is safe for these
	// bypass-path tests because GetHealthz / GetReadyz / metrics handler do
	// not touch the deps when the test only exercises /healthz or /metrics
	// (each test that uses /readyz must supply non-nil pool + redis).
	strictHandlers := NewWave0TempStubs(nil, nil, nil)
	deps := &Deps{
		Pool:           nil,
		Redis:          nil,
		OrgDB:          nil,
		Config:         nil,
		StrictHandlers: strictHandlers,
		SpecBytes:      nil,
	}
	mux := NewMux(deps)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

// TestNewMux_V1RequiresOrgHeader proves the /v1 sub-router gate works:
// hitting /v1/orgs/{anything}/agents without X-Org-Id returns 400
// invalid_org_id/missing_header per the OrgContext middleware contract
// (Plan 05 Task 2). This is the spoofing mitigation T-1-01.
//
// Phase 3 Wave 0 swap: /_scaffold -> /agents (D-77 scaffold deletion);
// the middleware behavior under test is path-agnostic.
func TestNewMux_V1RequiresOrgHeader(t *testing.T) {
	t.Parallel()

	// Phase 3 Wave 0 (Plan 03-01 D-77): NewCompositeServer was deleted with
	// the Phase 2 scaffold. Wave0TempStubs replaces it as the
	// StrictServerInterface implementation; nil pool/redis is safe for these
	// bypass-path tests because GetHealthz / GetReadyz / metrics handler do
	// not touch the deps when the test only exercises /healthz or /metrics
	// (each test that uses /readyz must supply non-nil pool + redis).
	strictHandlers := NewWave0TempStubs(nil, nil, nil)
	deps := &Deps{
		Pool:           nil,
		Redis:          nil,
		OrgDB:          nil,
		Config:         nil,
		StrictHandlers: strictHandlers,
		SpecBytes:      nil,
	}
	mux := NewMux(deps)

	// No X-Org-Id header — OrgContext should reject with 400.
	// Use a UUIDv7 path so the v7 middleware doesn't reject before OrgContext.
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0e/agents", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, "v1 routes must require X-Org-Id")
	require.Contains(t, rec.Body.String(), `"error":"invalid_org_id"`, "rejection code must be invalid_org_id")
	require.Contains(t, rec.Body.String(), `"reason":"missing_header"`, "rejection reason must be missing_header")
}
