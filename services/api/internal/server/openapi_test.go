// Package server_test: openapi_test.go verifies that OpenAPISpecHandler and
// DocsHandler serve the expected content with the correct Content-Type headers.
//
// These are pure unit tests — no Postgres, no Redis, no testcontainers. They
// run in -short mode and in every CI lane.
package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/server"
)

// TestOpenAPISpecHandler_ServesEmbeddedYAML asserts that OpenAPISpecHandler:
//   - responds with HTTP 200
//   - sets Content-Type: application/yaml
//   - writes the exact bytes that were passed as specBytes
func TestOpenAPISpecHandler_ServesEmbeddedYAML(t *testing.T) {
	t.Parallel()
	specBytes := []byte("openapi: 3.1.0\ninfo:\n  title: test\n  version: v0.1\n")

	h := server.OpenAPISpecHandler(specBytes)
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/yaml", rec.Header().Get("Content-Type"))
	require.Equal(t, specBytes, rec.Body.Bytes(), "response body must equal the supplied specBytes verbatim")
}

// TestDocsHandler_ServesEmbeddedHTML asserts that DocsHandler:
//   - responds with HTTP 200
//   - sets Content-Type: text/html; charset=utf-8
//   - response body contains the Scalar api-reference script element
//   - response body references /openapi.yaml as the spec URL
func TestDocsHandler_ServesEmbeddedHTML(t *testing.T) {
	t.Parallel()
	h := server.DocsHandler()
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))

	body := rec.Body.String()
	require.True(t, strings.Contains(body, "api-reference"),
		"body must contain the Scalar api-reference script tag id; got: %s", body)
	require.True(t, strings.Contains(body, `data-url="/openapi.yaml"`),
		`body must reference /openapi.yaml as the spec URL; got: %s`, body)
}
