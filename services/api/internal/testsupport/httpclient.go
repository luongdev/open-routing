package testsupport

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// DoBare sends a request with arbitrary headers and returns the full
// response + body bytes. Used by the FOUND-08 isolation suite when a test
// asserts both status code AND structured error body fields, where the
// per-entity httpHelper helpers would auto-set X-Org-Id (which is exactly
// what those tests are trying to omit or forge).
//
// Caller does NOT need to close resp.Body — DoBare reads it fully and
// closes it before returning. The returned *http.Response is for header /
// status inspection only.
func DoBare(t testing.TB, baseURL, method, path string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, baseURL+path, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	data, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, data
}
