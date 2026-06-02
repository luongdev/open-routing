package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func corsFailNextHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler must NOT be invoked")
	})
}

func corsNoopOK(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAllowedOriginsFromEnv_Empty(t *testing.T) {
	t.Parallel()
	res := AllowedOriginsFromEnv("")
	require.Equal(t, []string{}, res)
}

func TestAllowedOriginsFromEnv_Single(t *testing.T) {
	t.Parallel()
	res := AllowedOriginsFromEnv("https://example.com")
	require.Equal(t, []string{"https://example.com"}, res)
}

func TestAllowedOriginsFromEnv_Multiple(t *testing.T) {
	t.Parallel()
	res := AllowedOriginsFromEnv("https://a.com, https://b.com,https://c.com")
	require.Equal(t, []string{"https://a.com", "https://b.com", "https://c.com"}, res)
}

func TestAllowedOriginsFromEnv_Wildcard(t *testing.T) {
	t.Parallel()
	res := AllowedOriginsFromEnv("*")
	require.Equal(t, []string{"*"}, res)
}

func TestAllowedOriginsFromEnv_BlankSegments(t *testing.T) {
	t.Parallel()
	res := AllowedOriginsFromEnv("https://a.com,,https://b.com,")
	require.Equal(t, []string{"https://a.com", "https://b.com"}, res)
}

func TestCORS_PreflightShortCircuits(t *testing.T) {
	t.Parallel()
	h := NewCORS([]string{"https://example.com"})(corsFailNextHandler(t))
	req := httptest.NewRequest(http.MethodOptions, "/v1/orgs/some-org/agents", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_AllowedOriginGetsAllowOriginHeader(t *testing.T) {
	t.Parallel()
	h := NewCORS([]string{"https://example.com"})(corsNoopOK(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/some-org/agents", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_RejectedOriginNoAllowOriginHeader(t *testing.T) {
	t.Parallel()
	h := NewCORS([]string{"https://example.com"})(corsNoopOK(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/some-org/agents", nil)
	req.Header.Set("Origin", "https://malicious.example")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_EmptyAllowedOriginsDenyByDefault(t *testing.T) {
	t.Parallel()
	h := NewCORS([]string{})(corsNoopOK(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/some-org/agents", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_AllowedMethodsHeader(t *testing.T) {
	t.Parallel()
	h := NewCORS([]string{"https://example.com"})(corsFailNextHandler(t))
	req := httptest.NewRequest(http.MethodOptions, "/v1/orgs/some-org/agents", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "POST", rec.Header().Get("Access-Control-Allow-Methods"))
}

func TestCORS_AllowedHeadersHeader(t *testing.T) {
	t.Parallel()
	h := NewCORS([]string{"https://example.com"})(corsFailNextHandler(t))
	req := httptest.NewRequest(http.MethodOptions, "/v1/orgs/some-org/agents", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "X-Org-Id, Content-Type, Idempotency-Key, Accept-Language")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "X-Org-Id")
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Idempotency-Key")
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Accept-Language")
}

func TestCORS_ExposedHeadersHeader(t *testing.T) {
	t.Parallel()
	h := NewCORS([]string{"https://example.com"})(corsNoopOK(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/some-org/agents", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Access-Control-Expose-Headers"), "X-Request-Id")
}

func TestCORS_MaxAgeHeader(t *testing.T) {
	t.Parallel()
	h := NewCORS([]string{"https://example.com"})(corsFailNextHandler(t))
	req := httptest.NewRequest(http.MethodOptions, "/v1/orgs/some-org/agents", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "300", rec.Header().Get("Access-Control-Max-Age"))
}
