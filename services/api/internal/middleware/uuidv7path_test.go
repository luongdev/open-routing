package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	mw "github.com/luongdev/open-routing/services/api/internal/middleware"
)

// TestUUIDv7PathParams_AcceptsV7 verifies that a well-formed UUIDv7 in the
// {id} path param passes the middleware and reaches the handler.
func TestUUIDv7PathParams_AcceptsV7(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.With(mw.UUIDv7PathParams).Get("/v1/orgs/{org_id}/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Valid UUIDv7 in path
	req := httptest.NewRequest(http.MethodGet,
		"/v1/orgs/01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0a/agents/01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0b", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "valid UUIDv7 must reach handler")
}

// TestUUIDv7PathParams_RejectsV4 verifies that a UUIDv4 in the {id} path
// param is rejected with 400 invalid_id/uuidv7_required BEFORE the handler.
//
// REVIEWS HIGH #3: without this gate a UUIDv4 id silently reaches the handler
// and produces a 404 (no row for a v4 id in a v7-keyed table) — the same
// status as the FOUND-08 cross-org probe disposition, making it impossible to
// distinguish bad client input from a legitimate not-found.
func TestUUIDv7PathParams_RejectsV4(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	r := chi.NewRouter()
	r.With(mw.UUIDv7PathParams).Get("/v1/orgs/{org_id}/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// UUIDv4 in the {id} param
	req := httptest.NewRequest(http.MethodGet,
		"/v1/orgs/01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0a/agents/550e8400-e29b-41d4-a716-446655440000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.False(t, handlerCalled, "handler must NOT be called when id is not UUIDv7")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"error":"invalid_id"`)
	require.Contains(t, rec.Body.String(), `"reason":"uuidv7_required"`)
}

// TestUUIDv7PathParams_RejectsMalformedID verifies that a non-UUID {id} path
// param is rejected with 400 invalid_id/malformed_uuid.
func TestUUIDv7PathParams_RejectsMalformedID(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.With(mw.UUIDv7PathParams).Get("/v1/orgs/{org_id}/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/v1/orgs/01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0a/agents/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"error":"invalid_id"`)
	require.Contains(t, rec.Body.String(), `"reason":"malformed_uuid"`)
}

// TestUUIDv7PathParams_SkipsOrgId verifies that the {org_id} path param is
// NOT validated by UUIDv7PathParams (OrgContext owns that validation). A v4
// org_id must pass through to orgContextMiddleware which will reject it
// separately.
func TestUUIDv7PathParams_SkipsOrgId(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	r := chi.NewRouter()
	r.With(mw.UUIDv7PathParams).Get("/v1/orgs/{org_id}/agents", func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// v4 org_id — UUIDv7PathParams should NOT reject this (OrgContext does it)
	req := httptest.NewRequest(http.MethodGet,
		"/v1/orgs/550e8400-e29b-41d4-a716-446655440000/agents", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.True(t, handlerCalled, "UUIDv7PathParams must NOT intercept {org_id} — OrgContext handles it")
	require.Equal(t, http.StatusOK, rec.Code)
}

// TestUUIDv7PathParams_NoIdParam verifies that routes without an {id} path
// parameter pass through the middleware without any validation.
func TestUUIDv7PathParams_NoIdParam(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.With(mw.UUIDv7PathParams).Get("/v1/orgs/{org_id}/agents", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/v1/orgs/01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0a/agents", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code, "routes without {id} must not be affected")
}

// TestUUIDv7PathParams_RequestIDPropagated verifies that when the middleware
// rejects a request it includes the request_id in the error body (WriteError
// reads it from ctx via middleware.RequestIDFromContext).
func TestUUIDv7PathParams_RequestIDPropagated(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.Use(mw.RequestID) // sets request_id in ctx
	r.With(mw.UUIDv7PathParams).Get("/v1/orgs/{org_id}/agents/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// UUIDv4 in the {id} param
	req := httptest.NewRequest(http.MethodGet,
		"/v1/orgs/01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0a/agents/550e8400-e29b-41d4-a716-446655440000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `"error":"invalid_id"`)
	// request_id is populated by WriteError when ctx has one
	require.True(t,
		strings.Contains(body, `"request_id":"`),
		"WriteError must embed request_id from ctx when RequestID middleware ran: %s", body,
	)
}
