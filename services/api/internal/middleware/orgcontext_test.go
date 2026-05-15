package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// failNextHandler returns an http.Handler that fails the test if invoked.
// Used in reject-path tests to assert the middleware short-circuits without
// reaching the inner handler.
func failNextHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler must NOT be invoked when OrgContext rejects the request")
	})
}

// decodeErrorBody parses a canonical {error, reason} JSON body.
func decodeErrorBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

// TestOrgContext_MissingHeader400 covers FOUND-03 / D-20 / T-1-01.
// No X-Org-Id at all → 400 invalid_org_id / missing_header.
func TestOrgContext_MissingHeader400(t *testing.T) {
	t.Parallel()
	h := OrgContext(failNextHandler(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/x/_scaffold", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorBody(t, rec)
	require.Equal(t, "invalid_org_id", body["error"])
	require.Equal(t, "missing_header", body["reason"])
}

// TestOrgContext_MalformedHeader400 covers D-20.
// X-Org-Id is set but is not a parseable UUID → 400 / malformed_uuid.
func TestOrgContext_MalformedHeader400(t *testing.T) {
	t.Parallel()
	h := OrgContext(failNextHandler(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/x/_scaffold", nil)
	req.Header.Set("X-Org-Id", "not-a-uuid-at-all")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorBody(t, rec)
	require.Equal(t, "invalid_org_id", body["error"])
	require.Equal(t, "malformed_uuid", body["reason"])
}

// TestOrgContext_UUIDv4Rejected400 covers D-19 / T-1-INJ-V4.
// A syntactically valid UUIDv4 is rejected because D-19 requires v7+.
func TestOrgContext_UUIDv4Rejected400(t *testing.T) {
	t.Parallel()
	v4 := uuid.New() // google/uuid New() returns a v4 by default
	require.Equal(t, uuid.Version(4), v4.Version(), "test setup: uuid.New() must be v4")

	h := OrgContext(failNextHandler(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/x/_scaffold", nil)
	req.Header.Set("X-Org-Id", v4.String())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorBody(t, rec)
	require.Equal(t, "invalid_org_id", body["error"])
	require.Equal(t, "uuidv7_required", body["reason"])
}

// TestOrgContext_NilUUIDRejected400 covers the edge case of the zero UUID.
// uuid.Nil parses cleanly but has Version() == 0 — must be rejected as v7+
// is required.
func TestOrgContext_NilUUIDRejected400(t *testing.T) {
	t.Parallel()
	h := OrgContext(failNextHandler(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/x/_scaffold", nil)
	req.Header.Set("X-Org-Id", uuid.Nil.String())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorBody(t, rec)
	require.Equal(t, "invalid_org_id", body["error"])
	require.Equal(t, "uuidv7_required", body["reason"], "uuid.Nil (Version()==0) must be uuidv7_required")
}

// TestOrgContext_EmptyStringHeader400 covers the empty-string edge case.
// Per S3 contract, an explicit empty value is indistinguishable from missing.
func TestOrgContext_EmptyStringHeader400(t *testing.T) {
	t.Parallel()
	h := OrgContext(failNextHandler(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/x/_scaffold", nil)
	req.Header.Set("X-Org-Id", "")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorBody(t, rec)
	require.Equal(t, "invalid_org_id", body["error"])
	require.Equal(t, "missing_header", body["reason"], "empty string treated as missing")
}

// TestOrgContext_ValidUUIDv7_StoresInContext is the happy path.
// A valid UUIDv7 is stored in ctx as uuid.UUID and the next handler
// observes it via orgkey.OrgIDFromContext.
func TestOrgContext_ValidUUIDv7_StoresInContext(t *testing.T) {
	t.Parallel()
	orgID := uuid.Must(uuid.NewV7())

	var (
		captured uuid.UUID
		captOK   bool
	)
	h := OrgContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, captOK = orgkey.OrgIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/x/_scaffold", nil)
	req.Header.Set("X-Org-Id", orgID.String())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, captOK, "ctx must carry org_id after middleware")
	require.Equal(t, orgID, captured, "stored org_id must equal header value")
}

// TestOrgContext_FreshUUIDv7PerRequest exercises Pattern S8 — every test
// mints a fresh UUIDv7 so parallel runs never share state. This is also a
// smoke test that the middleware does not somehow cache values across
// requests.
func TestOrgContext_FreshUUIDv7PerRequest(t *testing.T) {
	t.Parallel()

	seen := make(map[uuid.UUID]struct{})
	h := OrgContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := orgkey.OrgIDFromContext(r.Context())
		require.True(t, ok)
		seen[id] = struct{}{}
		w.WriteHeader(http.StatusOK)
	}))

	const n = 3
	for i := 0; i < n; i++ {
		orgID := uuid.Must(uuid.NewV7())
		req := httptest.NewRequest(http.MethodGet, "/v1/orgs/x/_scaffold", nil)
		req.Header.Set("X-Org-Id", orgID.String())
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	require.Len(t, seen, n, "each request's org_id must be observed in ctx")
}
