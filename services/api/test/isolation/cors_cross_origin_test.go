package isolation_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/testsupport"
)

func TestCORSCrossOriginAllow(t *testing.T) {
	requireContainer(t)
	t.Parallel()

	orgID := freshOrg(t)
	resp, _ := testsupport.DoBare(t, baseURL(), http.MethodGet, "/v1/orgs/"+orgID.String()+"/agents", map[string]string{
		"X-Org-Id": orgID.String(),
		"Origin":   "https://example.com",
	})

	// Body served (200 OK because we provided X-Org-Id, or maybe 404/empty depending on handler,
	// but isolation test returns 200 OK for GET /agents if empty)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Contains(t, resp.Header.Get("Access-Control-Expose-Headers"), "X-Request-Id")
}

func TestCORSCrossOriginReject(t *testing.T) {
	requireContainer(t)
	t.Parallel()

	orgID := freshOrg(t)
	resp, _ := testsupport.DoBare(t, baseURL(), http.MethodGet, "/v1/orgs/"+orgID.String()+"/agents", map[string]string{
		"X-Org-Id": orgID.String(),
		"Origin":   "https://rejected.example",
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "", resp.Header.Get("Access-Control-Allow-Origin"))
}
