package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestRequestID_StoresUUIDv7InContextAndHeader proves the middleware:
//   1. mints a UUIDv7
//   2. stores it in ctx (retrievable via RequestIDFromContext)
//   3. echoes the SAME value via the X-Request-Id response header
//   4. allows the next handler to run
//
// Covers Two-Org Isolation Specification case 10 (TestRequestID_IsUUIDv7).
func TestRequestID_StoresUUIDv7InContextAndHeader(t *testing.T) {
	t.Parallel()

	var captured uuid.UUID
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := RequestIDFromContext(r.Context())
		require.True(t, ok, "ctx must carry request id")
		captured = id
		w.WriteHeader(http.StatusOK)
	})

	h := RequestID(next)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	headerID := rec.Header().Get("X-Request-Id")
	require.NotEmpty(t, headerID, "X-Request-Id header must be set")
	require.Equal(t, captured.String(), headerID, "header must match ctx value")

	// D-28: the request id must be UUIDv7.
	parsed, err := uuid.Parse(headerID)
	require.NoError(t, err, "X-Request-Id must be a valid UUID")
	require.Equal(t, uuid.Version(7), parsed.Version(), "request id must be UUIDv7 per D-28")
}

// TestRequestID_GeneratesUniqueIDsPerRequest proves each request gets a
// fresh UUIDv7 (no caching, no static seed).
func TestRequestID_GeneratesUniqueIDsPerRequest(t *testing.T) {
	t.Parallel()

	ids := make(map[uuid.UUID]struct{})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := RequestIDFromContext(r.Context())
		require.True(t, ok)
		ids[id] = struct{}{}
		w.WriteHeader(http.StatusOK)
	})
	h := RequestID(next)

	const n = 5
	for i := 0; i < n; i++ {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	require.Len(t, ids, n, "each request must get a fresh UUID; got duplicate(s)")
}

// TestRequestIDFromContext_MissingReturnsFalse documents the contract for
// callers that may run outside the middleware (bypass paths, raw tests).
func TestRequestIDFromContext_MissingReturnsFalse(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	id, ok := RequestIDFromContext(req.Context())
	require.False(t, ok, "ctx without middleware must report missing")
	require.Equal(t, uuid.Nil, id, "missing id must return uuid.Nil")
}
