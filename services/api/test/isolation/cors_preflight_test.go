package isolation_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/testsupport"
)

func TestCORSPreflight_V1AgentsRouteShortCircuits(t *testing.T) {
	requireContainer(t)
	t.Parallel()

	orgID := freshOrg(t)
	resp, body := testsupport.DoBare(t, baseURL(), http.MethodOptions, "/v1/orgs/"+orgID.String()+"/agents", map[string]string{
		"Origin":                        "https://example.com",
		"Access-Control-Request-Method": "GET",
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Empty(t, string(body)) // Preflight short-circuits without invalid_org_id JSON body
}

func TestCORSPreflight_BypassHealthz(t *testing.T) {
	requireContainer(t)
	t.Parallel()

	resp, _ := testsupport.DoBare(t, baseURL(), http.MethodOptions, "/healthz", map[string]string{
		"Origin":                        "https://example.com",
		"Access-Control-Request-Method": "GET",
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))
}
