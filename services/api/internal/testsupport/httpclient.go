package testsupport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// ScaffoldRow mirrors the JSON shape emitted by the scaffold handler.
// Field ordering matches services/api/internal/scaffold/handler.go's
// scaffoldResponse struct so a future client SDK can use the same struct.
type ScaffoldRow struct {
	ID         uuid.UUID `json:"id"`
	OrgID      uuid.UUID `json:"org_id"`
	ExternalID string    `json:"external_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
}

// PostScaffold sends POST /v1/orgs/{orgID}/_scaffold with the given body
// and X-Org-Id header. Returns the parsed ScaffoldRow on 201.
//
// The URL contains orgID and the header contains orgID — both intentionally —
// because the production middleware reads org_id from the header (FOUND-05)
// while the URL is decoration. Callers that want to test the
// header-vs-URL-divergence case (TestTwoOrgsIsolation_PostRespectsHeaderOrg)
// build the request manually rather than going through this helper.
func PostScaffold(t testing.TB, baseURL string, orgID uuid.UUID, externalID, name string) ScaffoldRow {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"external_id": externalID, "name": name})
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/v1/orgs/%s/_scaffold", baseURL, orgID),
		bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("PostScaffold: expected 201, got %d: %s", resp.StatusCode, data)
	}

	var row ScaffoldRow
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&row))
	return row
}

// ListScaffolds sends GET /v1/orgs/{orgID}/_scaffold; returns the array.
//
// On unexpected status the helper calls t.Fatalf so the failure message
// includes the body — useful when the OrgContext middleware rejects the
// header for a reason that surfaced after the test was written.
func ListScaffolds(t testing.TB, baseURL string, orgID uuid.UUID) []ScaffoldRow {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/v1/orgs/%s/_scaffold", baseURL, orgID), nil)
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("ListScaffolds: expected 200, got %d: %s", resp.StatusCode, data)
	}
	var rows []ScaffoldRow
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rows))
	return rows
}

// GetScaffoldStatus sends GET /v1/orgs/{orgID}/_scaffold/{id} and returns
// only the status code. Used to assert 404 for cross-org reads — the test
// does not care about the response body, only the status the server emitted
// after orgDB.QueryRow filtered the row out via WHERE id = $1 AND org_id = $2.
func GetScaffoldStatus(t testing.TB, baseURL string, orgID uuid.UUID, id uuid.UUID) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/v1/orgs/%s/_scaffold/%s", baseURL, orgID, id), nil)
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// DoBare sends a request with arbitrary headers and returns the full
// response + body bytes. Used for testing the OrgContext reject paths
// (missing header, malformed UUID, UUIDv4 rejected) where the test asserts
// both status code AND structured error body fields.
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
