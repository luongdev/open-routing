// imports_test.go — FOUND-08 cross-org probes for the Phase 5 bulk-
// import endpoints (BulkImportCatalog + GetImportJob). Mirrors the
// per-entity catalog probes in catalog_test.go and extends them with
// the Phase 5 specifics:
//
//   - GetImportJob cross-org → 404 (FOUND-08; orgB must NEVER see
//     orgA's import_jobs row).
//   - BulkImportCatalog cross-org same-code → BOTH succeed because the
//     composite UNIQUE(org_id, code) on Phase 04.1's entity tables
//     includes org_id (D04_1-24 / Pitfall 4.4).
//   - BulkImportCatalog cross-org FK probe → orgB importing a channel
//     referencing a queue code owned by orgA returns per-row
//     invalid_reference (D-76 same-org FK probe).
//   - BulkImportCatalog cross-org Idempotency-Key → composite UNIQUE
//     (org_id, idempotency_key) lets orgA + orgB share a key without
//     conflict.
package isolation_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// postImport submits a JSON bulk-import request against the shared
// httptest server for a given org. Returns (statusCode, responseBody).
//
// orgID is set as both the URL path segment AND the X-Org-Id header.
// The orgcontext middleware enforces equality (D-21).
func postImport(t *testing.T, urlBase string, orgID uuid.UUID, entity string, rows []any) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(rows)
	require.NoError(t, err)
	u := fmt.Sprintf("%s/v1/orgs/%s/catalog/import?entity=%s", urlBase, orgID, entity)
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

// postImportWithIdempotencyKey is like postImport but adds the
// Idempotency-Key header (D5-13).
func postImportWithIdempotencyKey(t *testing.T, urlBase string, orgID uuid.UUID, entity string, rows []any, key uuid.UUID) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(rows)
	require.NoError(t, err)
	u := fmt.Sprintf("%s/v1/orgs/%s/catalog/import?entity=%s", urlBase, orgID, entity)
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key.String())
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

// getImportJobAsOrg issues GET /v1/orgs/{orgID}/imports/{jobID} with
// the matching X-Org-Id header. Returns the status code.
func getImportJobAsOrg(t *testing.T, urlBase string, orgID, jobID uuid.UUID) int {
	t.Helper()
	u := fmt.Sprintf("%s/v1/orgs/%s/imports/%s", urlBase, orgID, jobID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// extractLatestJobIDForOrg fetches the most recent import_jobs.id for
// (org_id) directly from sharedPool. Used to obtain a job ID when the
// v0.1 BulkImportResult does not carry it on the wire (the GET endpoint
// is the only documented retrieval path; an admin UI would have read
// it from the response Location header in v0.2, which v0.1 omits).
func extractLatestJobIDForOrg(t *testing.T, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := sharedPool.QueryRow(t.Context(),
		`SELECT id FROM import_jobs WHERE org_id = $1
		   ORDER BY created_at DESC, id DESC LIMIT 1`, orgID,
	).Scan(&id)
	require.NoError(t, err, "extractLatestJobIDForOrg: %s", orgID)
	return id
}

// =============================================================================
// TestImport_CrossOrgGetJob_404 — FOUND-08 isolation. orgA POST creates
// an import_jobs row; orgB GET probes /imports/{id} and gets 404.
// Never 403 — that would leak the existence of orgA's row.
// =============================================================================
func TestImport_CrossOrgGetJob_404(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)

	rows := []any{
		map[string]any{
			"code":  "iso_imp_emp_a",
			"name":  "Alice A",
			"email": "alice@example.com",
		},
	}
	code, body := postImport(t, baseURL(), orgA, "agents", rows)
	require.Equal(t, http.StatusOK, code, "orgA import must succeed; body=%s", string(body))

	jobID := extractLatestJobIDForOrg(t, orgA)
	// orgA can fetch the job.
	require.Equal(t, http.StatusOK, getImportJobAsOrg(t, baseURL(), orgA, jobID),
		"orgA GET must succeed (sanity)")
	// orgB must get 404 (never 403, never 200).
	require.Equal(t, http.StatusNotFound, getImportJobAsOrg(t, baseURL(), orgB, jobID),
		"orgB GET on orgA's import_jobs row MUST be 404 (FOUND-08 / D04_1-22 mirror)")
}

// =============================================================================
// TestImport_CrossOrgSameCode_BothSucceed — Phase 04.1 D04_1-24 mirror
// for the import path. Two orgs may both import agents with code
// "emp_001" because the composite UNIQUE on (org_id, code) includes
// org_id. Pitfall 4.4 / IMP-03.
//
// Mirrors catalog_test.go's TestCatalog_CrossOrgSameCode_BothSucceed
// pattern (line 375); proves the import endpoint inherits the same
// invariant via UpsertAgentByCode.
// =============================================================================
func TestImport_CrossOrgSameCode_BothSucceed(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)

	rowsA := []any{
		map[string]any{
			"code":  "iso_imp_shared_code",
			"name":  "Alice A",
			"email": "alicea@example.com",
		},
	}
	codeA, bodyA := postImport(t, baseURL(), orgA, "agents", rowsA)
	require.Equalf(t, http.StatusOK, codeA,
		"orgA import of code=iso_imp_shared_code must succeed; body=%s", string(bodyA))

	rowsB := []any{
		map[string]any{
			"code":  "iso_imp_shared_code", // SAME code as orgA's row.
			"name":  "Alice B",
			"email": "aliceb@example.com",
		},
	}
	codeB, bodyB := postImport(t, baseURL(), orgB, "agents", rowsB)
	require.Equalf(t, http.StatusOK, codeB,
		"orgB import of SAME code=iso_imp_shared_code must succeed — composite UNIQUE (org_id, code) includes org_id (D04_1-24 / Pitfall 4.4); body=%s",
		string(bodyB))

	// Both orgs own their own agent row.
	var countA, countB int
	require.NoError(t, sharedPool.QueryRow(t.Context(),
		`SELECT count(*) FROM agents WHERE org_id = $1 AND code = $2`,
		orgA, "iso_imp_shared_code",
	).Scan(&countA))
	require.Equal(t, 1, countA, "orgA owns 1 agent with shared code")
	require.NoError(t, sharedPool.QueryRow(t.Context(),
		`SELECT count(*) FROM agents WHERE org_id = $1 AND code = $2`,
		orgB, "iso_imp_shared_code",
	).Scan(&countB))
	require.Equal(t, 1, countB, "orgB owns 1 agent with shared code")
}

// =============================================================================
// TestImport_CrossOrg_Channel_DefaultQueueCode_invalid_reference —
// Phase 5 D-76 same-org FK probe. orgA seeds a queue with code
// q_iso_main; orgB attempts to import a channel referencing
// default_queue_code=q_iso_main; the channel import must fail with
// per-row reason="invalid_reference" (orgB's GetQueueByCode returns
// pgx.ErrNoRows because the composite WHERE clause filters out orgA's
// queue).
// =============================================================================
func TestImport_CrossOrg_Channel_DefaultQueueCode_invalid_reference(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)

	// orgA seeds queue q_iso_main via the catalog handler (idiomatic;
	// the import endpoint also works but we just need the side-effect).
	queueRows := []any{
		map[string]any{
			"code":          "q_iso_main",
			"name":          "Main",
			"channel_types": []string{"voice"},
		},
	}
	codeA, bodyA := postImport(t, baseURL(), orgA, "queues", queueRows)
	require.Equal(t, http.StatusOK, codeA, "orgA queue import; body=%s", string(bodyA))

	// orgB attempts to import a channel referencing the orgA-owned queue
	// by code. The FK probe (GetQueueByCode in the savepoint Tx) fails;
	// per-row reason is invalid_reference.
	dqc := "q_iso_main"
	channelRows := []any{
		map[string]any{
			"code":               "ch_iso_voice",
			"name":               "Iso Voice",
			"channel_type":       "voice",
			"default_queue_code": dqc,
		},
	}
	codeB, bodyB := postImport(t, baseURL(), orgB, "channels", channelRows)
	// All-fail → 422.
	require.Equalf(t, http.StatusUnprocessableEntity, codeB,
		"orgB channel import referencing orgA's queue MUST 422 (all-fail; D-76 cross-org probe); body=%s",
		string(bodyB))

	var r struct {
		Failed []struct {
			Row    int    `json:"row"`
			Field  string `json:"field,omitempty"`
			Reason string `json:"reason"`
		} `json:"failed"`
	}
	require.NoError(t, json.Unmarshal(bodyB, &r))
	require.Len(t, r.Failed, 1)
	require.Equal(t, "invalid_reference", r.Failed[0].Reason,
		"D-76 cross-org FK probe failure must surface as invalid_reference; body=%s",
		string(bodyB))
}

// =============================================================================
// TestImport_CrossOrgSameIdempotencyKey_BothProceed — partial UNIQUE
// (org_id, idempotency_key) on import_jobs. orgA POSTs with key K;
// orgB POSTs with same K. BOTH proceed; each org gets its own
// import_jobs row.
// =============================================================================
func TestImport_CrossOrgSameIdempotencyKey_BothProceed(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)
	key := uuid.Must(uuid.NewV7())

	rowsA := []any{
		map[string]any{"code": "iso_idem_a_001", "name": "A", "email": "a@example.com"},
	}
	codeA, bodyA := postImportWithIdempotencyKey(t, baseURL(), orgA, "agents", rowsA, key)
	require.Equal(t, http.StatusOK, codeA, "orgA import; body=%s", string(bodyA))

	rowsB := []any{
		map[string]any{"code": "iso_idem_b_001", "name": "B", "email": "b@example.com"},
	}
	codeB, bodyB := postImportWithIdempotencyKey(t, baseURL(), orgB, "agents", rowsB, key)
	require.Equalf(t, http.StatusOK, codeB,
		"orgB import with SAME idempotency_key must proceed (composite UNIQUE includes org_id); body=%s",
		string(bodyB))

	// Each org has exactly ONE import_jobs row carrying the shared key.
	var countA, countB int
	keyStr := key.String()
	require.NoError(t, sharedPool.QueryRow(t.Context(),
		`SELECT count(*) FROM import_jobs WHERE org_id = $1 AND idempotency_key = $2`,
		orgA, keyStr,
	).Scan(&countA))
	require.Equal(t, 1, countA)
	require.NoError(t, sharedPool.QueryRow(t.Context(),
		`SELECT count(*) FROM import_jobs WHERE org_id = $1 AND idempotency_key = $2`,
		orgB, keyStr,
	).Scan(&countB))
	require.Equal(t, 1, countB)
}
