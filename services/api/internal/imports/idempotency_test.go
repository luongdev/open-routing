// idempotency_test.go — Wave 5 integration tests for the
// Idempotency-Key replay path (D5-13 / D5-27 / IMP-03).
//
// Coverage:
//   - Idempotency-Key miss → proceeds normally; a new import_jobs row
//     is created carrying the supplied key.
//   - Idempotency-Key hit → returns the persisted prior result with
//     idempotent_replay=true; succeeded[] is empty (KNOWN LIMITATION
//     per D5-13); no new work performed.
//   - Cross-org same-key → BOTH proceed (composite UNIQUE on
//     (org_id, idempotency_key)).
//   - No Idempotency-Key + double-run → TWO import_jobs rows but ONE
//     set of unique agents per (org_id, code) (IMP-03 / D04_1-01).
//   - Different body, same key → SECOND POST still returns FIRST
//     payload (replay short-circuits BEFORE Content-Type dispatch).
package imports

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// TestBulkImport_IdempotencyKey_MissProceeds_NewJobCreated — first
// POST with a fresh Idempotency-Key K1 → 200 + idempotent_replay
// nil/false + ONE import_jobs row carrying idempotency_key=K1.
func TestBulkImport_IdempotencyKey_MissProceeds_NewJobCreated(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []interface{}{
		map[string]interface{}{"code": "emp_idem_001", "name": "A", "email": "a@example.com"},
	}
	k1 := uuid.Must(uuid.NewV7())
	resp, body := postImportJSONWithIdempotencyKey(t, th, api.Agents, rows, k1)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	r := decodeBulkImportResult(t, body)
	require.Len(t, r.Succeeded, 1)
	// idempotent_replay is nil OR false on a miss path.
	if r.IdempotentReplay != nil {
		require.False(t, *r.IdempotentReplay, "miss path must not set replay=true")
	}

	// Exactly ONE import_jobs row exists, carrying the key.
	require.Equal(t, 1, countImportJobsByOrg(t, ctx, th.Pool, th.OrgID))
	jobID := extractLatestJobID(t, ctx, th.Pool, th.OrgID)
	var persistedKey *string
	err := th.Pool.QueryRow(ctx,
		`SELECT idempotency_key FROM import_jobs WHERE id = $1 AND org_id = $2`,
		jobID, th.OrgID,
	).Scan(&persistedKey)
	require.NoError(t, err)
	require.NotNil(t, persistedKey, "miss path must persist the supplied Idempotency-Key")
	require.Equal(t, k1.String(), *persistedKey)
}

// TestBulkImport_IdempotencyKey_HitReturnsReplay_NoNewWork — POST
// twice with the same K1 + same body. The second POST hits the replay
// path: returns idempotent_replay=true + succeeded:[] + failed[] from
// the persisted prior row. EXACTLY ONE import_jobs row exists after
// both calls (no second row).
func TestBulkImport_IdempotencyKey_HitReturnsReplay_NoNewWork(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []interface{}{
		map[string]interface{}{"code": "emp_idem_hit_001", "name": "A", "email": "a@example.com"},
	}
	k1 := uuid.Must(uuid.NewV7())

	resp1, body1 := postImportJSONWithIdempotencyKey(t, th, api.Agents, rows, k1)
	require.Equal(t, http.StatusOK, resp1.StatusCode, "body=%s", string(body1))
	require.Len(t, decodeBulkImportResult(t, body1).Succeeded, 1)
	jobsAfterFirst := countImportJobsByOrg(t, ctx, th.Pool, th.OrgID)
	require.Equal(t, 1, jobsAfterFirst)

	resp2, body2 := postImportJSONWithIdempotencyKey(t, th, api.Agents, rows, k1)
	require.Equal(t, http.StatusOK, resp2.StatusCode, "body=%s", string(body2))
	r2 := decodeBulkImportResult(t, body2)
	require.NotNil(t, r2.IdempotentReplay, "replay flag must be set on hit")
	require.True(t, *r2.IdempotentReplay, "replay flag must be true on hit")
	require.Empty(t, r2.Succeeded,
		"D5-13 KNOWN LIMITATION: succeeded[] is always empty on replay; got %v",
		r2.Succeeded)

	// No second import_jobs row was created.
	require.Equal(t, 1, countImportJobsByOrg(t, ctx, th.Pool, th.OrgID),
		"replay must NOT create a second import_jobs row")
}

// TestBulkImport_IdempotencyKey_HitReturnsPriorFailures — pre-seed an
// import_jobs row with status=completed + errors JSONB; POST with the
// stored key; the response failed[] MUST mirror the persisted entries
// byte-for-byte (D5-12 zero-transformation guarantee).
func TestBulkImport_IdempotencyKey_HitReturnsPriorFailures(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed a completed import_jobs row with structured failures.
	jobID := uuid.Must(uuid.NewV7())
	k2 := uuid.Must(uuid.NewV7())
	keyStr := k2.String()
	errorsJSON := []byte(`[
		{"row":1,"field":"code","error":"import_failed","reason":"invalid_code_format"},
		{"row":2,"error":"import_failed","reason":"duplicate_code:agents"},
		{"row":3,"field":"email","error":"import_failed","reason":"invalid_email"}
	]`)
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO import_jobs
			(id, org_id, entity_type, status, total_rows, succeeded_rows, failed_rows, errors, idempotency_key, created_at, updated_at)
		 VALUES ($1, $2, 'agents', 'completed', 3, 0, 3, $3::jsonb, $4, NOW(), NOW())`,
		jobID, th.OrgID, errorsJSON, keyStr,
	)
	require.NoError(t, err)

	rows := []interface{}{
		map[string]interface{}{"code": "emp_idem_failures_001", "name": "A", "email": "a@example.com"},
	}
	resp, body := postImportJSONWithIdempotencyKey(t, th, api.Agents, rows, k2)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	r := decodeBulkImportResult(t, body)
	require.NotNil(t, r.IdempotentReplay)
	require.True(t, *r.IdempotentReplay)
	require.Len(t, r.Failed, 3, "all 3 persisted failures must surface; body=%s", string(body))
	require.Equal(t, 1, r.Failed[0].Row)
	require.Equal(t, "invalid_code_format", r.Failed[0].Reason)
	require.Equal(t, 2, r.Failed[1].Row)
	require.Equal(t, "duplicate_code:agents", r.Failed[1].Reason)
	require.Equal(t, 3, r.Failed[2].Row)
}

// TestBulkImport_IdempotencyKey_DifferentBodySameKey_StillReturnsPriorResult —
// idempotency replay short-circuits BEFORE Content-Type dispatch. A
// second POST with the same key but DIFFERENT body must return the
// FIRST result (no re-execution; no honoring the new body).
func TestBulkImport_IdempotencyKey_DifferentBodySameKey_StillReturnsPriorResult(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rowsA := []interface{}{
		map[string]interface{}{"code": "emp_idem_body_a", "name": "Body A", "email": "a@example.com"},
	}
	k3 := uuid.Must(uuid.NewV7())

	resp1, body1 := postImportJSONWithIdempotencyKey(t, th, api.Agents, rowsA, k3)
	require.Equal(t, http.StatusOK, resp1.StatusCode, "body=%s", string(body1))

	// Different body B — but same key. The replay path returns the
	// FIRST result; the new body is NEVER parsed or persisted.
	rowsB := []interface{}{
		map[string]interface{}{"code": "emp_idem_body_b", "name": "Body B", "email": "b@example.com"},
		map[string]interface{}{"code": "emp_idem_body_c", "name": "Body C", "email": "c@example.com"},
	}
	resp2, body2 := postImportJSONWithIdempotencyKey(t, th, api.Agents, rowsB, k3)
	require.Equal(t, http.StatusOK, resp2.StatusCode, "body=%s", string(body2))
	r2 := decodeBulkImportResult(t, body2)
	require.NotNil(t, r2.IdempotentReplay)
	require.True(t, *r2.IdempotentReplay)

	// Verify the SECOND body was not processed: emp_idem_body_b /
	// emp_idem_body_c agents do NOT exist.
	for _, code := range []string{"emp_idem_body_b", "emp_idem_body_c"} {
		var count int
		require.NoError(t, th.Pool.QueryRow(ctx,
			`SELECT count(*) FROM agents WHERE org_id = $1 AND code = $2`,
			th.OrgID, code,
		).Scan(&count))
		require.Equalf(t, 0, count,
			"replay must NOT process second body's rows; %s found %d times", code, count)
	}
}

// TestBulkImport_IdempotencyKey_DifferentOrgSameKey_BothProceed — the
// partial UNIQUE on (org_id, idempotency_key) includes org_id, so two
// orgs may use the same key concurrently. Each org's import is
// independently persisted.
func TestBulkImport_IdempotencyKey_DifferentOrgSameKey_BothProceed(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	orgB := uuid.Must(uuid.NewV7())
	ctxB := orgkey.SetOrgID(context.Background(), orgB)
	defer cleanImportTables(t, ctxB, th.Pool, orgB)

	k4 := uuid.Must(uuid.NewV7())

	// orgA path uses the standard helper (th.OrgID).
	rowsA := []interface{}{
		map[string]interface{}{"code": "emp_idem_orga", "name": "OrgA", "email": "a@example.com"},
	}
	resp1, body1 := postImportJSONWithIdempotencyKey(t, th, api.Agents, rowsA, k4)
	require.Equal(t, http.StatusOK, resp1.StatusCode, "orgA POST; body=%s", string(body1))
	r1 := decodeBulkImportResult(t, body1)
	require.Len(t, r1.Succeeded, 1)

	// orgB path: manually build the request (helper hardcodes th.OrgID).
	rawB, mErr := json.Marshal([]interface{}{
		map[string]interface{}{"code": "emp_idem_orgb", "name": "OrgB", "email": "b@example.com"},
	})
	require.NoError(t, mErr)
	req2, rErr := http.NewRequest(http.MethodPost,
		th.HTTP.URL+importPath(orgB)+"?entity="+string(api.Agents),
		bytes.NewReader(rawB))
	require.NoError(t, rErr)
	req2.Header.Set("X-Org-Id", orgB.String())
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", k4.String())
	resp2, dErr := th.HTTP.Client().Do(req2)
	require.NoError(t, dErr)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode,
		"orgB POST with same key MUST proceed (composite UNIQUE includes org_id)")

	// Both orgs have ONE import_jobs row with the same idempotency_key.
	require.Equal(t, 1, countImportJobsByOrg(t, ctx, th.Pool, th.OrgID))
	require.Equal(t, 1, countImportJobsByOrg(t, ctxB, th.Pool, orgB))
}

// TestBulkImport_NoIdempotencyKey_DoubleRunCreatesTwoJobs_NoDupesByCode —
// without Idempotency-Key, POSTing the same body twice creates TWO
// import_jobs rows (each call gets a fresh row) but ONE set of unique
// agents per (org_id, code) (UpsertAgentByCode dedupes — IMP-03 +
// D04_1-01).
func TestBulkImport_NoIdempotencyKey_DoubleRunCreatesTwoJobs_NoDupesByCode(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []interface{}{
		map[string]interface{}{"code": "emp_double_001", "name": "A", "email": "a@example.com"},
		map[string]interface{}{"code": "emp_double_002", "name": "B", "email": "b@example.com"},
	}

	// First POST — no idempotency key. Both rows imported.
	resp1, body1 := postImportJSON(t, th, api.Agents, rows)
	require.Equal(t, http.StatusOK, resp1.StatusCode, "body=%s", string(body1))
	require.Len(t, decodeBulkImportResult(t, body1).Succeeded, 2)

	// Second POST — same body, no key. Upsert UPDATE branch hit; both
	// rows succeed again at the import level.
	resp2, body2 := postImportJSON(t, th, api.Agents, rows)
	require.Equal(t, http.StatusOK, resp2.StatusCode, "body=%s", string(body2))
	require.Len(t, decodeBulkImportResult(t, body2).Succeeded, 2)

	// IMP-03 invariant: exactly TWO import_jobs rows (one per POST) but
	// the agents table still has exactly TWO unique rows by (org_id, code).
	require.Equal(t, 2, countImportJobsByOrg(t, ctx, th.Pool, th.OrgID))

	var agentCount int
	require.NoError(t, th.Pool.QueryRow(ctx,
		`SELECT count(*) FROM agents WHERE org_id = $1`, th.OrgID,
	).Scan(&agentCount))
	require.Equal(t, 2, agentCount,
		"IMP-03: UpsertAgentByCode dedupes — two POSTs of the same code set produce TWO agents (not 4)")
}

