// bodylimit_test.go — unit tests for BodyLimit middleware (D5-21).
//
// Coverage matrix (mirrors PATTERNS.md / RESEARCH §Validation Architecture):
//
//	TestBodyLimit_ContentLengthOverMax_413_BeforeBodyRead — pre-flight kicks
//	  in BEFORE the downstream handler runs; body never reaches the handler.
//	TestBodyLimit_LyingContentLength_413_MidStream — chunked transfer encoding
//	  or a deliberately understated Content-Length still trips MaxBytesError
//	  when the handler reads r.Body. Middleware succeeds (returns 200 path-
//	  through); the handler is responsible for mapping the typed error to
//	  413 — this test documents that contract.
//	TestBodyLimit_PathMismatch_Passthrough — request to /healthz does NOT
//	  wrap r.Body; a 10 MB body reads fine even with maxBytes=50.
//	TestBodyLimit_AtExactlyMaxBytes_Passes — Content-Length == maxBytes
//	  reaches the handler (boundary).
//	TestBodyLimit_OverMaxBytesByOneByte_413 — Content-Length == maxBytes+1
//	  short-circuits 413 (boundary).
//	TestBodyLimit_NoContentLength_StillWraps — request with no Content-Length
//	  header is wrapped anyway; over-limit chunked body trips MaxBytesError.
//	TestBodyLimit_ZeroByteBody_Passes — empty body under the cap reaches the
//	  handler.
//
// Test shape adapted from services/api/internal/middleware/uuidv7path_test.go
// (closest analog: path-scoped chi MiddlewareFunc with table-driven
// assertions). The bodylimit test cases include both pre-flight and
// wrap-side branches so the test file documents the full contract D5-21
// locks against.
package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	mw "github.com/luongdev/open-routing/services/api/internal/middleware"
)

const (
	importPrefix = "/v1/orgs/" // any prefix; tests treat it as opaque.
	healthPath   = "/healthz"
	importPath   = "/v1/orgs/01900000-0000-7000-8000-000000000000/catalog/import"
)

// noopHandler returns a handler that asserts NOT-CALLED via the supplied
// pointer. The middleware-level pre-flight rejection tests use this to
// prove the downstream handler never observes the request.
func noopHandlerExpectingNoCall(t *testing.T) (http.Handler, *bool) {
	t.Helper()
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	return h, &called
}

// TestBodyLimit_ContentLengthOverMax_413_BeforeBodyRead documents the
// pre-flight short-circuit: a Content-Length that exceeds maxBytes
// produces an immediate 413 without the downstream handler ever running.
// The 413 body carries the locked {error, reason} pair from
// httputil.go WriteError.
func TestBodyLimit_ContentLengthOverMax_413_BeforeBodyRead(t *testing.T) {
	t.Parallel()

	const maxBytes int64 = 50
	downstream, called := noopHandlerExpectingNoCall(t)
	h := mw.BodyLimit(maxBytes, importPrefix)(downstream)

	body := bytes.NewReader(make([]byte, 100)) // Content-Length 100, maxBytes 50.
	req := httptest.NewRequest(http.MethodPost, importPath, body)
	req.ContentLength = 100
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code,
		"Content-Length > maxBytes must short-circuit 413")
	require.False(t, *called, "downstream handler must NEVER run on pre-flight reject")

	var body413 map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body413))
	require.Equal(t, "invalid_body", body413["error"], "wire shape locked: error=invalid_body")
	require.Equal(t, "request_too_large_use_async_pathway", body413["reason"],
		"wire shape locked: reason points to deferred IMP-09 async pathway")
}

// TestBodyLimit_LyingContentLength_413_MidStream documents the wrap-side
// branch: a body whose Content-Length is *understated* still trips a
// typed *http.MaxBytesError when the downstream handler reads through
// r.Body. The middleware itself does NOT produce the 413 here — the
// HANDLER is responsible for errors.As(err, &maxBytesErr) and emitting
// 413. This test composes a minimal downstream handler that does
// exactly that, locking the contract that mid-stream limit enforcement
// is the handler's job (the middleware can't peek into io.Reader use).
//
// Pitfall 5 (RESEARCH): Content-Length is best-effort; the MaxBytesReader
// wrap is the actual guard.
func TestBodyLimit_LyingContentLength_413_MidStream(t *testing.T) {
	t.Parallel()

	const maxBytes int64 = 50
	// Downstream handler simulates what Phase 5's bulk-import handler
	// will do (it reads r.Body, gets *http.MaxBytesError, maps to 413).
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "request_too_large_use_async_pathway",
					http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	h := mw.BodyLimit(maxBytes, importPrefix)(downstream)

	// 200 bytes of payload with Content-Length set to 50 (the lie).
	payload := bytes.NewReader(make([]byte, 200))
	req := httptest.NewRequest(http.MethodPost, importPath, payload)
	req.ContentLength = 50 // lie — pre-flight passes (50 == maxBytes).
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code,
		"mid-stream over-limit must surface as 413 via MaxBytesError")
}

// TestBodyLimit_PathMismatch_Passthrough proves the path-scope invariant.
// A request to /healthz (which does NOT start with importPrefix) must
// pass through untouched — the downstream handler reads an arbitrarily
// large body without 413, because BodyLimit did not wrap r.Body.
func TestBodyLimit_PathMismatch_Passthrough(t *testing.T) {
	t.Parallel()

	const maxBytes int64 = 50
	// Downstream handler asserts it CAN read more than maxBytes bytes
	// from r.Body (proving MaxBytesReader was NOT installed).
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err, "/healthz body must read in full (not wrapped)")
		require.Greater(t, len(b), int(maxBytes), "body bigger than cap must pass through")
		w.WriteHeader(http.StatusOK)
	})
	h := mw.BodyLimit(maxBytes, importPrefix)(downstream)

	body := bytes.NewReader(make([]byte, 1000))
	req := httptest.NewRequest(http.MethodPost, healthPath, body)
	req.ContentLength = 1000
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code,
		"path mismatch must pass through with no body wrap")
}

// TestBodyLimit_AtExactlyMaxBytes_Passes locks the boundary condition.
// Content-Length == maxBytes is ALLOWED (the cap is "must not exceed",
// not "must be strictly less than"). Mirrors http.MaxBytesReader's own
// behaviour where reading exactly maxBytes succeeds.
func TestBodyLimit_AtExactlyMaxBytes_Passes(t *testing.T) {
	t.Parallel()

	const maxBytes int64 = 50
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err, "body exactly == maxBytes must read clean")
		require.Equal(t, int(maxBytes), len(b))
		w.WriteHeader(http.StatusOK)
	})
	h := mw.BodyLimit(maxBytes, importPrefix)(downstream)

	body := bytes.NewReader(make([]byte, 50))
	req := httptest.NewRequest(http.MethodPost, importPath, body)
	req.ContentLength = 50
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code,
		"Content-Length == maxBytes must pass (boundary)")
}

// TestBodyLimit_OverMaxBytesByOneByte_413 locks the off-by-one boundary
// from the OTHER side. Content-Length == maxBytes+1 must short-circuit
// 413 immediately. Without this test a future refactor swapping `>` for
// `>=` would silently regress the 50 MB cap by 1 byte.
func TestBodyLimit_OverMaxBytesByOneByte_413(t *testing.T) {
	t.Parallel()

	const maxBytes int64 = 50
	downstream, called := noopHandlerExpectingNoCall(t)
	h := mw.BodyLimit(maxBytes, importPrefix)(downstream)

	body := bytes.NewReader(make([]byte, 51))
	req := httptest.NewRequest(http.MethodPost, importPath, body)
	req.ContentLength = 51
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code,
		"Content-Length == maxBytes+1 must 413 (off-by-one guard)")
	require.False(t, *called, "downstream must NOT run on +1 boundary")
}

// TestBodyLimit_NoContentLength_StillWraps documents the chunked-transfer
// case: a request that omits Content-Length entirely must STILL be wrapped
// in MaxBytesReader so a mid-stream over-limit trips MaxBytesError. The
// pre-flight branch silently passes (no header to parse) but the wrap
// branch catches the actual oversized payload.
//
// This is the single most important guarantee for chunked uploads —
// without the wrap, a client could chunk-stream a 1 GB body and the
// import handler would OOM before any error fired.
func TestBodyLimit_NoContentLength_StillWraps(t *testing.T) {
	t.Parallel()

	const maxBytes int64 = 50
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		var maxBytesErr *http.MaxBytesError
		require.True(t, errors.As(err, &maxBytesErr),
			"expected MaxBytesError when chunked body exceeds cap; got %v", err)
		http.Error(w, "request_too_large_use_async_pathway",
			http.StatusRequestEntityTooLarge)
	})
	h := mw.BodyLimit(maxBytes, importPrefix)(downstream)

	// 200-byte body but Content-Length deliberately cleared so the wrap is
	// the only guard. We mimic the chunked-transfer scenario by setting
	// ContentLength = -1, which httptest.NewRequest accepts and forwards
	// to the handler as an unknown size. We also strip the header so the
	// pre-flight pass shouldn't take that branch.
	payload := bytes.NewReader(make([]byte, 200))
	req := httptest.NewRequest(http.MethodPost, importPath, payload)
	req.ContentLength = -1
	req.Header.Del("Content-Length")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code,
		"chunked body over cap must 413 via the wrap branch")
}

// TestBodyLimit_ZeroByteBody_Passes documents the trivial bottom edge:
// an empty body (no payload at all) passes through unchanged. Locked so
// a future refactor that overzealously short-circuits "small" requests
// doesn't silently start rejecting legitimate PUT-with-no-body operations.
func TestBodyLimit_ZeroByteBody_Passes(t *testing.T) {
	t.Parallel()

	const maxBytes int64 = 50
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err, "empty body must read without error")
		require.Empty(t, b, "empty body stays empty after wrap")
		w.WriteHeader(http.StatusOK)
	})
	h := mw.BodyLimit(maxBytes, importPrefix)(downstream)

	req := httptest.NewRequest(http.MethodPost, importPath, strings.NewReader(""))
	req.ContentLength = 0
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "zero-byte body must pass")
}
