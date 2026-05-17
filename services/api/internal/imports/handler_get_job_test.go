// handler_get_job_test.go — Wave 4 coverage for the GetImportJob
// handler. Tests cover:
//
//   - orgID missing in ctx → 500.
//   - Row found in same org → 200 with full ImportJob wire shape.
//   - Unknown id (no row) → 404.
//   - Cross-org probe → 404 (FOUND-08 isolation; never 403).
//
// All tests except the orgID-missing case use the testcontainer-backed
// TestImports fixture (real Postgres + migrations + sqlc).
package imports

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// TestGetImportJob_MissingOrgID_500 — ctx without orgkey → 500.
func TestGetImportJob_MissingOrgID_500(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	imp := New(Deps{OrgDB: nil, Cache: nil, Logger: logger})

	req := api.GetImportJobRequestObject{
		OrgId:    openapi_types.UUID(uuid.Must(uuid.NewV7())),
		ImportId: openapi_types.UUID(uuid.Must(uuid.NewV7())),
	}
	resp, err := imp.GetImportJob(context.Background(), req)
	require.NoError(t, err)
	r, ok := resp.(api.GetImportJob500JSONResponse)
	require.True(t, ok, "expected 500; got %T", resp)
	require.Equal(t, api.ErrorCodeInternal, r.Error)
	require.Equal(t, "missing_org_id_in_context", r.Reason)
}

// TestGetImportJob_Found_200_FullShape — seed a row with all fields
// populated (terminal status + errors JSONB) and verify the wire shape
// round-trips through mapImportJob.
func TestGetImportJob_Found_200_FullShape(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	jobID := uuid.Must(uuid.NewV7())
	errorsJSON := []byte(`[
		{"row":1,"error":"import_failed","reason":"duplicate_code","field":"code"},
		{"row":3,"error":"import_failed","reason":"invalid_value"}
	]`)
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO import_jobs
			(id, org_id, entity_type, status, total_rows, succeeded_rows, failed_rows, errors, idempotency_key, created_at, updated_at)
		 VALUES ($1, $2, 'agents', 'completed', 5, 3, 2, $3::jsonb, NULL, NOW(), NOW())`,
		jobID, th.OrgID, errorsJSON,
	)
	require.NoError(t, err)

	req := api.GetImportJobRequestObject{
		OrgId:    openapi_types.UUID(th.OrgID),
		ImportId: openapi_types.UUID(jobID),
	}
	resp, err := th.I.GetImportJob(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.GetImportJob200JSONResponse)
	require.True(t, ok, "expected 200; got %T", resp)

	require.Equal(t, openapi_types.UUID(jobID), r.Id)
	require.Equal(t, openapi_types.UUID(th.OrgID), r.OrgId)
	require.Equal(t, api.ImportEntityType("agents"), r.EntityType)
	require.Equal(t, api.ImportJobStatus("completed"), r.Status)
	require.Equal(t, 5, r.TotalRows)
	require.Equal(t, 3, r.SucceededRows)
	require.Equal(t, 2, r.FailedRows)
	require.NotNil(t, r.Errors, "errors JSONB must rehydrate")
	require.Len(t, *r.Errors, 2)
	require.Equal(t, 1, (*r.Errors)[0].Row)
	require.Equal(t, "duplicate_code", (*r.Errors)[0].Reason)
	require.NotNil(t, (*r.Errors)[0].Field)
	require.Equal(t, "code", *(*r.Errors)[0].Field)
	require.Equal(t, 3, (*r.Errors)[1].Row)
	require.Equal(t, "invalid_value", (*r.Errors)[1].Reason)
	require.NotNil(t, r.CreatedAt, "created_at must populate")
}

// TestGetImportJob_NotFound_404 — random UUID → 404 with reason
// "import_job_not_found".
func TestGetImportJob_NotFound_404(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	req := api.GetImportJobRequestObject{
		OrgId:    openapi_types.UUID(th.OrgID),
		ImportId: openapi_types.UUID(uuid.Must(uuid.NewV7())),
	}
	resp, err := th.I.GetImportJob(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.GetImportJob404JSONResponse)
	require.True(t, ok, "expected 404; got %T", resp)
	require.Equal(t, api.ErrorCodeNotFound, r.Error)
	require.Equal(t, "import_job_not_found", r.Reason)
}

// TestGetImportJob_CrossOrg_404 — seed a row in orgA; query as orgB;
// FOUND-08 isolation guarantees a 404, never a 403 (which would leak
// the existence of other-org rows).
func TestGetImportJob_CrossOrg_404(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Seed row in th.OrgID (orgA).
	jobID := uuid.Must(uuid.NewV7())
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO import_jobs
			(id, org_id, entity_type, status, total_rows, succeeded_rows, failed_rows, errors, idempotency_key, created_at, updated_at)
		 VALUES ($1, $2, 'skills', 'completed', 1, 1, 0, NULL, NULL, NOW(), NOW())`,
		jobID, th.OrgID,
	)
	require.NoError(t, err)

	// Query as a DIFFERENT org (orgB). Even though the jobID is valid
	// and present, the composite (id, org_id) WHERE clause filters it
	// out → 404.
	orgB := uuid.Must(uuid.NewV7())
	ctxB := orgkey.SetOrgID(context.Background(), orgB)
	// Clean orgB too (defensive — should not have rows but symmetric
	// fixture hygiene).
	defer cleanImportTables(t, ctxB, th.Pool, orgB)

	req := api.GetImportJobRequestObject{
		OrgId:    openapi_types.UUID(orgB),
		ImportId: openapi_types.UUID(jobID),
	}
	resp, err := th.I.GetImportJob(ctxB, req)
	require.NoError(t, err)
	r, ok := resp.(api.GetImportJob404JSONResponse)
	require.True(t, ok, "cross-org probe must return 404 (FOUND-08); got %T", resp)
	require.Equal(t, "import_job_not_found", r.Reason,
		"reason must NEVER hint the row exists in another org")
}
