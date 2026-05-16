// isolation_test.go contains the FOUND-08 acceptance cases (VALIDATION.md
// §"Two-Org Isolation Specification" lines 1617-1628). Every test goes
// through the package-shared httptest server which wraps server.NewMux —
// the same code path production uses (D-07).
//
// Cross-references to other Plans' tests:
//   - Case 8 (SQL without org_id filter is rejected in dev): proven by
//     internal/db/sqlcheck_test.go via the SQLChecker classification table.
//   - Case 9 (WithBypass allows unscoped SQL): proven by
//     internal/db/sqlcheck_test.go + the cmd/migrate audit emission path.
//   - Case 11 (logs carry trace_id): proven by internal/telemetry/slog_test.go
//     which exercises the same handler this server uses.
//
// This file therefore implements 9 cases directly (Cases 1, 2, 3, 4, 5, 6, 7,
// 10) + one additional FOUND-06 duplicate-insert HTTP path that proves the
// UNIQUE(org_id, external_id) constraint reaches the API layer.
package isolation_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/testsupport"
)

// baseURL returns the package-shared httptest server URL.
//
// Helper rather than direct sharedSrv.URL reads so tests grep for one
// canonical access point — and so a future migration to a per-test server
// fixture changes one function, not 11 test bodies.
func baseURL() string {
	return sharedSrv.URL
}

// freshOrg mints a fresh UUIDv7 org_id for a single test invocation.
// Pitfall 6: every test pays this cost; package-level orgID constants
// are forbidden.
func freshOrg(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.Must(uuid.NewV7())
}

// mustReader is a small body-reader helper used by the few tests that
// build requests manually (rather than through testsupport.PostScaffold)
// to exercise header-vs-URL divergence.
func mustReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

// requireContainer is a single guard the docker-less unit lane uses to
// skip every isolation test in one consistent way. -short is the canonical
// signal; sharedPool/sharedSrv being nil is the docker-unavailable signal.
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
// Case 1 — TestTwoOrgsIsolation_ListsExcludeOtherOrg (FOUND-08 happy path).
// =============================================================================
//
// Two orgs seeded with overlapping external_ids; each org's list MUST contain
// only its own rows. This is the canonical FOUND-08 proof — the entire
// purpose of the org_id isolation infrastructure exists to make this case
// pass.
func TestTwoOrgsIsolation_ListsExcludeOtherOrg(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA := freshOrg(t)
	orgB := freshOrg(t)

	// Seed: orgA gets 3 rows, orgB gets 2 rows. Use overlapping external_ids
	// to prove the UNIQUE(org_id, external_id) constraint allows different
	// orgs to share external_id values (FOUND-06 nuance).
	testsupport.SeedScaffold(t, baseURL(), orgA, []testsupport.ScaffoldSeed{
		{ExternalID: "ext-1", Name: "A-1"},
		{ExternalID: "ext-2", Name: "A-2"},
		{ExternalID: "ext-3", Name: "A-3"},
	})
	testsupport.SeedScaffold(t, baseURL(), orgB, []testsupport.ScaffoldSeed{
		{ExternalID: "ext-1", Name: "B-1"},
		{ExternalID: "ext-2", Name: "B-2"},
	})

	rowsA := testsupport.ListScaffolds(t, baseURL(), orgA)
	require.Len(t, rowsA, 3, "orgA must see exactly its 3 rows")
	for _, r := range rowsA {
		require.Equal(t, orgA, r.OrgID, "orgA's list contains a foreign org_id — isolation breach")
	}

	rowsB := testsupport.ListScaffolds(t, baseURL(), orgB)
	require.Len(t, rowsB, 2, "orgB must see exactly its 2 rows")
	for _, r := range rowsB {
		require.Equal(t, orgB, r.OrgID, "orgB's list contains a foreign org_id — isolation breach")
	}
}

// =============================================================================
// Case 2 — TestTwoOrgsIsolation_GetByIDIsScoped (cross-org GET returns 404).
// =============================================================================
//
// Two orgs each create one row with the same external_id. Each org CAN see
// its own row; each org CANNOT see the other's row (must return 404, not 200
// — would expose existence-of-id signal otherwise). The sqlc query body
// filters by (id, org_id) so a cross-org probe returns pgx.ErrNoRows which
// the handler maps to 404.
func TestTwoOrgsIsolation_GetByIDIsScoped(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA := freshOrg(t)
	orgB := freshOrg(t)
	rowA := testsupport.PostScaffold(t, baseURL(), orgA, "ext-shared", "A")
	rowB := testsupport.PostScaffold(t, baseURL(), orgB, "ext-shared", "B")

	// orgA cannot see orgB's row by ID; orgB cannot see orgA's.
	require.Equal(t, http.StatusNotFound,
		testsupport.GetScaffoldStatus(t, baseURL(), orgA, rowB.ID),
		"orgA must not be able to GET orgB's row by id")
	require.Equal(t, http.StatusNotFound,
		testsupport.GetScaffoldStatus(t, baseURL(), orgB, rowA.ID),
		"orgB must not be able to GET orgA's row by id")

	// Each org CAN see its own row (proves the 404 above is not a blanket
	// reject — the row exists, just not for that org).
	require.Equal(t, http.StatusOK,
		testsupport.GetScaffoldStatus(t, baseURL(), orgA, rowA.ID))
	require.Equal(t, http.StatusOK,
		testsupport.GetScaffoldStatus(t, baseURL(), orgB, rowB.ID))
}

// =============================================================================
// Case 3 — TestTwoOrgsIsolation_PostRespectsHeaderOrg (FOUND-05 header wins).
// =============================================================================
//
// POST with X-Org-Id = orgA; URL path contains orgB. The header is the
// authoritative org_id (FOUND-05); the URL is decoration. The row written
// MUST have org_id = orgA, not orgB. Without this proof a hostile client
// could drive cross-org writes by manipulating the URL.
func TestTwoOrgsIsolation_PostRespectsHeaderOrg(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA := freshOrg(t)
	orgB := freshOrg(t)

	body := []byte(`{"external_id":"ext-header-vs-url","name":"hdr"}`)
	req, err := http.NewRequest(http.MethodPost,
		baseURL()+"/v1/orgs/"+orgB.String()+"/_scaffold",
		mustReader(body))
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgA.String())
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var row testsupport.ScaffoldRow
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&row))
	require.Equal(t, orgA, row.OrgID, "header is the source of truth, not URL path (FOUND-05)")

	// Belt-and-braces: orgA SHOULD see the row in its list; orgB MUST NOT.
	rowsA := testsupport.ListScaffolds(t, baseURL(), orgA)
	foundInA := false
	for _, r := range rowsA {
		if r.ID == row.ID {
			foundInA = true
			break
		}
	}
	require.True(t, foundInA, "row written with header=orgA must appear in orgA's list")

	require.Equal(t, http.StatusNotFound,
		testsupport.GetScaffoldStatus(t, baseURL(), orgB, row.ID),
		"orgB must not see the row that was written via its URL but with header=orgA")
}

// =============================================================================
// Case 4 — TestOrgContext_MissingHeader400 (D-20 reject path 1).
// =============================================================================
//
// No X-Org-Id header → 400 {"error":"invalid_org_id","reason":"missing_header"}.
// The reason string is LOCKED (D-20); clients branch on it.
func TestOrgContext_MissingHeader400(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	// Use a syntactically-valid org_id in the URL path so chi resolves the
	// route; the OrgContext middleware then rejects on missing header BEFORE
	// the handler runs.
	resp, body := testsupport.DoBare(t, baseURL(), http.MethodGet,
		"/v1/orgs/"+uuid.Must(uuid.NewV7()).String()+"/_scaffold", nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, string(body), `"reason":"missing_header"`)
}

// =============================================================================
// Case 5 — TestOrgContext_MalformedHeader400 (D-20 reject path 2).
// =============================================================================
//
// X-Org-Id present but not a UUID → 400 {"reason":"malformed_uuid"}.
func TestOrgContext_MalformedHeader400(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	resp, body := testsupport.DoBare(t, baseURL(), http.MethodGet,
		"/v1/orgs/"+uuid.Must(uuid.NewV7()).String()+"/_scaffold",
		map[string]string{"X-Org-Id": "not-a-uuid"})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, string(body), `"reason":"malformed_uuid"`)
}

// =============================================================================
// Case 6 — TestOrgContext_UUIDv4Rejected400 (D-19 / D-20 reject path 3).
// =============================================================================
//
// X-Org-Id is a valid UUID but Version() < 7 → 400 {"reason":"uuidv7_required"}.
// Project-wide constraint: every ID must be UUIDv7+ (D-19). uuid.New() returns
// a v4 deterministically (the function explicitly sets version=4), so this
// test is not flaky.
func TestOrgContext_UUIDv4Rejected400(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	v4 := uuid.New()
	require.Equal(t, uuid.Version(4), v4.Version(), "uuid.New() must return v4 deterministically")

	resp, body := testsupport.DoBare(t, baseURL(), http.MethodGet,
		"/v1/orgs/"+uuid.Must(uuid.NewV7()).String()+"/_scaffold",
		map[string]string{"X-Org-Id": v4.String()})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, string(body), `"reason":"uuidv7_required"`)
}

// =============================================================================
// Case 7 — TestBypassPaths_NoHeaderRequired (D-21 bypass list).
// =============================================================================
//
// /healthz /readyz /metrics MUST be reachable without X-Org-Id. /healthz is
// strictly 200; /readyz may degrade to 503 (no redis env) or to 500 (chi
// Recoverer catching a nil-redis Ping panic) — both are valid for proving
// D-21 (no org gate). /metrics is a Phase 1 stub returning 200 + empty body.
func TestBypassPaths_NoHeaderRequired(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
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
// Every response carries an X-Request-Id header, and that header MUST be a
// UUIDv7 (D-19's UUIDv7-everywhere rule applies to request_id too — D-28
// explicitly). Using /healthz keeps the test free of /readyz's redis
// dependence; the RequestID middleware runs before bypass routes so the
// header is present on /healthz responses.
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
// D-35 contracts that every ErrorResponse body carries a non-empty request_id
// whose value equals the X-Request-Id response header. The strict-server
// pipeline bypasses middleware.WriteError, so RequestIDInjectionMiddleware
// (a StrictMiddlewareFunc) must inject the field. This test proves the full
// end-to-end chain works: appmw.RequestID mints the UUIDv7 into ctx →
// RequestIDInjectionMiddleware reads it → the scaffold GetById handler
// returns a 404 ErrorResponse → the middleware sets its RequestId field →
// the generated response encoder writes it → the integration test observes it.
//
// We hit a GET /_scaffold/{nonexistent_id} which always returns 404 (no row
// for a fresh UUIDv7). The 200 path for /healthz would not exercise the
// ErrorResponse path (GetHealthz returns a non-error response type).
func TestRequestID_PresentInErrorBody(t *testing.T) {
	requireContainer(t)
	t.Parallel()

	orgA := freshOrg(t)
	missingID := uuid.Must(uuid.NewV7())

	resp, body := testsupport.DoBare(t, baseURL(), http.MethodGet,
		"/v1/orgs/"+orgA.String()+"/_scaffold/"+missingID.String(),
		map[string]string{"X-Org-Id": orgA.String()})

	require.Equal(t, http.StatusNotFound, resp.StatusCode,
		"GET with missing scaffold id must return 404; body=%s", body)

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
		"B-1: request_id must be present in 404 error body; raw=%s", body)
	require.Equal(t, headerID, errBody.RequestID,
		"B-1: body.request_id must equal X-Request-Id header (D-35 end-to-end)")
}

// =============================================================================
// Additional case — TestUniqueOrgExternalIdConstraint_DoublePost (FOUND-06 HTTP).
// =============================================================================
//
// Schema test proves FOUND-06 at the constraint layer (schema_test.go).
// This test proves FOUND-06 reaches the API: posting two rows with the same
// (org_id, external_id) MUST fail. Phase 1 has no dedicated 409 mapping, so
// the handler returns 500 — we accept either 409 (future) or 500 (Phase 1).
// The point is that the duplicate is NOT silently accepted.
func TestUniqueOrgExternalIdConstraint_DoublePost(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA := freshOrg(t)
	// First insert succeeds.
	testsupport.PostScaffold(t, baseURL(), orgA, "ext-dup", "first")

	// Second insert with same (orgA, ext-dup) must fail.
	body := []byte(`{"external_id":"ext-dup","name":"second"}`)
	req, err := http.NewRequest(http.MethodPost,
		baseURL()+"/v1/orgs/"+orgA.String()+"/_scaffold",
		mustReader(body))
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgA.String())
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.True(t,
		resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusInternalServerError,
		"duplicate (org_id, external_id) must fail at the HTTP layer; got %d", resp.StatusCode)
}
