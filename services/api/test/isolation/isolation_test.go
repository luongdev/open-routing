// isolation_test.go owns the middleware-layer FOUND-08 acceptance cases —
// behaviour that is entity-agnostic (X-Org-Id validation, bypass-path
// routing, request_id propagation). The per-entity cross-org probes live
// in catalog_test.go where the assertions can name the catalog table
// being probed.
//
// Cross-references to other packages' tests:
//   - SQLChecker classification table: internal/db/sqlcheck_test.go.
//   - WithBypass allowance + cmd/migrate audit emission:
//     internal/db/sqlcheck_test.go + migration audit tests.
//   - Logs carry trace_id: internal/telemetry/slog_test.go.
package isolation_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/testsupport"
)

// baseURL returns the package-shared httptest server URL. Helper rather
// than direct sharedSrv.URL reads so tests grep for one canonical access
// point and a future migration to a per-test server fixture changes one
// function, not 11 test bodies.
func baseURL() string {
	return sharedSrv.URL
}

// freshOrg mints a fresh UUIDv7 org_id for a single test invocation.
// Pitfall 6: every test pays this cost; package-level orgID constants
// are forbidden so cross-test contamination cannot mask a regression.
func freshOrg(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.Must(uuid.NewV7())
}

// requireContainer is the single guard the docker-less unit lane uses to
// skip every isolation test consistently. -short is the canonical signal;
// sharedPool/sharedSrv being nil is the docker-unavailable signal.
func requireContainer(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("isolation: requires postgres testcontainer; -short set")
	}
	if sharedPool == nil || sharedSrv == nil {
		t.Skip("isolation: shared testcontainer/server unavailable")
	}
}

// =============================================================================
// Case 4 — TestOrgContext_MissingHeader400 (D-20 reject path 1).
// =============================================================================
//
// No X-Org-Id header → 400 invalid_org_id / missing_header. The reason
// string is LOCKED (D-20); clients branch on it.
func TestOrgContext_MissingHeader400(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	resp, body := testsupport.DoBare(t, baseURL(), http.MethodGet,
		"/v1/orgs/"+uuid.Must(uuid.NewV7()).String()+"/agents", nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, string(body), `"reason":"missing_header"`)
}

// =============================================================================
// Case 5 — TestOrgContext_MalformedHeader400 (D-20 reject path 2).
// =============================================================================
//
// X-Org-Id present but not a UUID → 400 malformed_uuid.
func TestOrgContext_MalformedHeader400(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	resp, body := testsupport.DoBare(t, baseURL(), http.MethodGet,
		"/v1/orgs/"+uuid.Must(uuid.NewV7()).String()+"/agents",
		map[string]string{"X-Org-Id": "not-a-uuid"})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, string(body), `"reason":"malformed_uuid"`)
}

// =============================================================================
// Case 6 — TestOrgContext_UUIDv4Rejected400 (D-19 / D-20 reject path 3).
// =============================================================================
//
// X-Org-Id is a valid UUID but Version() < 7 → 400 uuidv7_required.
// uuid.New() returns a v4 deterministically (function explicitly sets
// version=4), so the test is not flaky.
func TestOrgContext_UUIDv4Rejected400(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	v4 := uuid.New()
	require.Equal(t, uuid.Version(4), v4.Version(), "uuid.New() must return v4 deterministically")
	resp, body := testsupport.DoBare(t, baseURL(), http.MethodGet,
		"/v1/orgs/"+uuid.Must(uuid.NewV7()).String()+"/agents",
		map[string]string{"X-Org-Id": v4.String()})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, string(body), `"reason":"uuidv7_required"`)
}

// =============================================================================
// Case 7 — TestBypassPaths_NoHeaderRequired (D-21 bypass list).
// =============================================================================
//
// /healthz, /readyz, /openapi.yaml, /docs, /metrics MUST be reachable
// without X-Org-Id. /healthz is strictly 200; /readyz can be 200 or 503
// depending on Postgres + Redis state (both are non-nil in TestMain, so
// expect 200); /openapi.yaml is 200 + YAML body; /docs is 200 + HTML;
// /metrics is the Phase 1 stub returning 200 + empty body.
func TestBypassPaths_NoHeaderRequired(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	for _, path := range []string{"/healthz", "/readyz", "/openapi.yaml", "/docs", "/metrics"} {
		resp, _ := testsupport.DoBare(t, baseURL(), http.MethodGet, path, nil)
		require.NotEqual(t, http.StatusBadRequest, resp.StatusCode,
			"bypass path %s must not require X-Org-Id; got %d", path, resp.StatusCode)
		if path == "/healthz" {
			require.Equal(t, http.StatusOK, resp.StatusCode,
				"/healthz must always be 200 (liveness probe contract)")
		}
		if path == "/metrics" {
			require.Equal(t, http.StatusOK, resp.StatusCode,
				"/metrics Phase 1 stub returns 200")
		}
	}
}

// =============================================================================
// Case 10 — TestRequestID_IsUUIDv7 (D-28: X-Request-Id is UUIDv7).
// =============================================================================
//
// Every response carries an X-Request-Id header, and that header MUST be
// a UUIDv7 (D-19's UUIDv7-everywhere rule applies to request_id too —
// D-28 explicitly). Using /healthz keeps the test free of /readyz's redis
// dependence; the RequestID middleware runs before bypass route handlers
// so the header is present on /healthz responses.
func TestRequestID_IsUUIDv7(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	resp, _ := testsupport.DoBare(t, baseURL(), http.MethodGet, "/healthz", nil)
	headerID := resp.Header.Get("X-Request-Id")
	require.NotEmpty(t, headerID, "X-Request-Id must be present on every response")
	parsed, err := uuid.Parse(headerID)
	require.NoError(t, err, "X-Request-Id must be a parseable UUID")
	require.Equal(t, uuid.Version(7), parsed.Version(),
		"X-Request-Id must be UUIDv7 per D-28; got version %d", parsed.Version())
}

// =============================================================================
// B-1 case — TestRequestID_PresentInErrorBody (D-35: request_id in error body).
// =============================================================================
//
// D-35 contracts that every ErrorResponse body carries a non-empty
// request_id whose value equals the X-Request-Id response header. The
// strict-server pipeline bypasses middleware.WriteError, so
// RequestIDInjectionMiddleware (a StrictMiddlewareFunc) must inject the
// field. This test proves the full end-to-end chain: appmw.RequestID
// mints the UUIDv7 into ctx → RequestIDInjectionMiddleware reads it →
// the catalog GetAgent handler returns 404 (no row for a fresh UUIDv7)
// → the middleware sets RequestId on the response → the generated
// encoder writes it → the test observes it.
func TestRequestID_PresentInErrorBody(t *testing.T) {
	requireContainer(t)
	t.Parallel()

	orgA := freshOrg(t)
	missingID := uuid.Must(uuid.NewV7())

	resp, body := testsupport.DoBare(t, baseURL(), http.MethodGet,
		"/v1/orgs/"+orgA.String()+"/agents/"+missingID.String(),
		map[string]string{"X-Org-Id": orgA.String()})

	require.Equal(t, http.StatusNotFound, resp.StatusCode,
		"GET missing agent must return 404; body=%s", body)

	headerID := resp.Header.Get("X-Request-Id")
	require.NotEmpty(t, headerID, "X-Request-Id must be set on every response (D-28)")

	var errBody struct {
		Error     string `json:"error"`
		Reason    string `json:"reason"`
		RequestID string `json:"request_id"`
	}
	require.NoError(t, json.Unmarshal(body, &errBody),
		"404 body must be valid JSON; raw=%s", body)
	require.NotEmpty(t, errBody.RequestID,
		"B-1: request_id must be present in error body; raw=%s", body)
	require.Equal(t, headerID, errBody.RequestID,
		"B-1: body.request_id must equal X-Request-Id header (D-35 end-to-end)")
}
