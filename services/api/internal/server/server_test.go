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
// and nil Redis because LiveHandler does not touch them.
//
// The test deliberately constructs a Deps with nil Pool / nil Redis to
// prove that the bypass path /healthz literally does not depend on those
// dependencies — a regression that routes /healthz through OrgContext or
// touches the pool here would panic immediately on nil deref.
func TestNewMux_HealthzAccessibleWithoutOrgHeader(t *testing.T) {
	t.Parallel()

	deps := &Deps{
		Pool:   nil, // safe — LiveHandler does not touch the pool
		Redis:  nil, // safe — LiveHandler does not touch redis
		OrgDB:  nil, // safe — bypass paths do not reach scaffold handlers
		Config: nil, // unused by bypass paths
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

	deps := &Deps{
		Pool: nil, Redis: nil, OrgDB: nil, Config: nil,
	}
	mux := NewMux(deps)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

// TestNewMux_V1RequiresOrgHeader proves the /v1 sub-router gate works:
// hitting /v1/orgs/{anything}/_scaffold without X-Org-Id returns 400
// invalid_org_id/missing_header per the OrgContext middleware contract
// (Plan 05 Task 2). This is the spoofing mitigation T-1-01.
func TestNewMux_V1RequiresOrgHeader(t *testing.T) {
	t.Parallel()

	deps := &Deps{
		Pool: nil, Redis: nil, OrgDB: nil, Config: nil,
	}
	mux := NewMux(deps)

	// No X-Org-Id header — OrgContext should reject with 400.
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/01234567-89ab-cdef-0123-456789abcdef/_scaffold", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, "v1 routes must require X-Org-Id")
	require.Contains(t, rec.Body.String(), `"error":"invalid_org_id"`, "rejection code must be invalid_org_id")
	require.Contains(t, rec.Body.String(), `"reason":"missing_header"`, "rejection reason must be missing_header")
}
