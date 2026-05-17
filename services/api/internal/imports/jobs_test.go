// jobs_test.go — Wave 5 integration tests for the import_jobs row
// lifecycle (D5-10 / IMP-06).
//
// Coverage:
//   - GetImportJob round-trip: POST returns the persisted job; GET
//     by id returns the ImportJob wire shape with terminal status +
//     counters + nil/populated errors.
//   - GetImportJob partial-failure round-trip: errors[] JSONB
//     populates the wire shape (D5-12 zero-transformation).
//   - GetImportJob not-found: random UUID → 404 import_job_not_found.
//   - finaliseJob on happy path: counters match Succeeded count;
//     failed_rows=0.
//   - finaliseJob on partial: counters match Succeeded + Failed counts.
package imports

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// TestBulkImport_GetImportJob_RoundTrip — POST agents → 200 + a row
// lands in import_jobs; GET .../imports/{id} → 200 + ImportJob wire
// shape with status=completed + counters=correct.
func TestBulkImport_GetImportJob_RoundTrip(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []interface{}{
		map[string]interface{}{"code": "emp_jobs_001", "name": "A", "email": "a@example.com"},
		map[string]interface{}{"code": "emp_jobs_002", "name": "B", "email": "b@example.com"},
	}
	resp, body := postImportJSON(t, th, api.Agents, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))

	// v0.1 BulkImportResult does not carry the job id on the wire — we
	// fetch via direct DB query, mirroring how admin UI would query
	// after a /healthz polling loop in a future async flow (IMP-06).
	jobID := extractLatestJobID(t, ctx, th.Pool, th.OrgID)

	resp2, body2 := getImportJob(t, th, jobID)
	require.Equal(t, http.StatusOK, resp2.StatusCode, "body=%s", string(body2))
	j := decodeImportJob(t, body2)
	require.Equal(t, jobID, uuid.UUID(j.Id))
	require.Equal(t, th.OrgID, uuid.UUID(j.OrgId))
	require.Equal(t, api.ImportEntityType("agents"), j.EntityType)
	require.Equal(t, api.ImportJobStatus("completed"), j.Status)
	require.Equal(t, 2, j.TotalRows)
	require.Equal(t, 2, j.SucceededRows)
	require.Equal(t, 0, j.FailedRows)
	require.NotNil(t, j.CreatedAt, "created_at must populate")
	// Happy path: errors should be nil OR an empty pointer to []
	if j.Errors != nil {
		require.Empty(t, *j.Errors,
			"happy path: errors[] must be empty or nil; got %v", *j.Errors)
	}
}

// TestBulkImport_GetImportJob_PartialFailure_ReturnsErrors — POST
// partial-success.csv → 207; GET .../imports/{id} → 200 with errors[]
// populated. D5-12: zero-transformation between POST and GET shapes.
func TestBulkImport_GetImportJob_PartialFailure_ReturnsErrors(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.Agents, loadTestData(t, "partial-success.csv"))
	require.Equal(t, http.StatusMultiStatus, resp.StatusCode, "body=%s", string(body))
	postResult := decodeBulkImportResult(t, body)
	require.NotEmpty(t, postResult.Failed)

	jobID := extractLatestJobID(t, ctx, th.Pool, th.OrgID)
	resp2, body2 := getImportJob(t, th, jobID)
	require.Equal(t, http.StatusOK, resp2.StatusCode, "body=%s", string(body2))
	j := decodeImportJob(t, body2)
	require.Equal(t, api.ImportJobStatus("completed"), j.Status,
		"partial-success terminal status is 'completed' (succeeded_rows > 0); D5-10")
	require.Equal(t, len(postResult.Failed), j.FailedRows,
		"GET FailedRows must equal POST Failed length")
	require.NotNil(t, j.Errors, "errors[] must rehydrate on partial-failure GET")
	require.Equal(t, len(postResult.Failed), len(*j.Errors),
		"errors[] count must match POST Failed[] count")
	require.Equal(t, postResult.Failed[0].Reason, (*j.Errors)[0].Reason,
		"D5-12: zero-transformation — POST Failed[0].Reason must equal GET errors[0].reason")
}

// TestBulkImport_GetImportJob_NotFound_404 — random UUID → 404
// import_job_not_found.
func TestBulkImport_GetImportJob_NotFound_404(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	randID := uuid.Must(uuid.NewV7())
	resp, body := getImportJob(t, th, randID)
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "body=%s", string(body))
	require.Contains(t, string(body), "import_job_not_found")
}

// TestBulkImport_FinaliseJob_OnAllSucceed_UpdatesStatus — POST 3
// valid rows; assert the persisted import_jobs row has status=completed
// + succeeded_rows=3 + failed_rows=0.
func TestBulkImport_FinaliseJob_OnAllSucceed_UpdatesStatus(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := decodeJSONArray(t, loadTestData(t, "agents-basic.json"))
	resp, body := postImportJSON(t, th, api.Agents, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))

	jobID := extractLatestJobID(t, ctx, th.Pool, th.OrgID)
	status, total, succ, fail, errorsRaw := fetchImportJob(t, ctx, th.Pool, th.OrgID, jobID)
	require.Equal(t, "completed", status)
	require.Equal(t, 3, total)
	require.Equal(t, 3, succ)
	require.Equal(t, 0, fail)
	// errors column should be NULL on happy path (no Failed[] to persist).
	require.Empty(t, errorsRaw, "happy path: errors JSONB must be NULL/empty; got %s", string(errorsRaw))
}

// TestBulkImport_FinaliseJob_OnPartial_UpdatesCounters — POST
// partial-success.csv; assert succeeded + failed counters are correct
// and the errors JSONB column is non-empty.
func TestBulkImport_FinaliseJob_OnPartial_UpdatesCounters(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.Agents, loadTestData(t, "partial-success.csv"))
	require.Equal(t, http.StatusMultiStatus, resp.StatusCode, "body=%s", string(body))
	r := decodeBulkImportResult(t, body)

	jobID := extractLatestJobID(t, ctx, th.Pool, th.OrgID)
	status, total, succ, fail, errorsRaw := fetchImportJob(t, ctx, th.Pool, th.OrgID, jobID)
	require.Equal(t, "completed", status, "completed even on partial — failed != all (D5-10)")
	require.Equal(t, 4, total, "partial-success.csv has 4 data rows")
	require.Equal(t, len(r.Succeeded), succ)
	require.Equal(t, len(r.Failed), fail)
	require.NotEmpty(t, errorsRaw, "partial-failure: errors JSONB must persist")
}
