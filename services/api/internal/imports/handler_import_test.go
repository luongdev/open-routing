// handler_import_test.go — Wave 4 dispatch-level coverage for the
// BulkImportCatalog handler. Tests focus on the steering branches:
//
//   - orgID missing in ctx → 500.
//   - ?entity= invalid → 400.
//   - Idempotency-Key hit → replay returned.
//   - No JSONBody + no Body → 400 unsupported_content_type.
//   - CSV without/with-wrong schema_version → 400.
//   - JSON over 500 rows → 413.
//   - CSV header-only (zero data rows) → 200 with empty arrays.
//   - Status decisions (200 / 207 / 422) from real chunk loop results.
//
// Integration tests (entity × format matrix, full HTTP path) land in
// Wave 5 / Plan 05-07 — this file ships the dispatch contract.
//
// Tests requiring DB writes use the testcontainer-backed TestImports
// fixture and skip automatically under -short. Pure-dispatch tests use
// a stripped-down Importer that touches no DB.
package imports

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// newDispatchOnlyImporter constructs an *Importer suitable for tests
// that exercise dispatch branches BEFORE any DB call. Logger discards
// every line so test output stays clean. The OrgDB + Cache fields are
// left nil because the dispatch branches we cover do not reach them.
func newDispatchOnlyImporter() *Importer {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return New(Deps{
		OrgDB:  nil,
		Cache:  nil,
		Logger: logger,
	})
}

// schemaV01 returns a *string pointer to "v0.1" for setting the CSV
// schema_version param in tests.
func schemaV01() *string {
	v := "v0.1"
	return &v
}

// TestBulkImportCatalog_MissingOrgID_500 — ctx without orgkey → 500
// missing_org_id_in_context (Cross-Cutting Pattern 1).
func TestBulkImportCatalog_MissingOrgID_500(t *testing.T) {
	t.Parallel()
	imp := newDispatchOnlyImporter()
	req := api.BulkImportCatalogRequestObject{
		Params: api.BulkImportCatalogParams{Entity: api.Agents},
	}
	resp, err := imp.BulkImportCatalog(context.Background(), req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog500JSONResponse)
	require.True(t, ok, "expected 500 response; got %T", resp)
	require.Equal(t, api.ErrorCodeInternal, r.Error)
	require.Equal(t, "missing_org_id_in_context", r.Reason)
}

// TestBulkImportCatalog_InvalidEntity_400 — entity="" or unknown rejects
// before any work.
func TestBulkImportCatalog_InvalidEntity_400(t *testing.T) {
	t.Parallel()
	imp := newDispatchOnlyImporter()
	ctx := orgkey.SetOrgID(context.Background(), uuid.Must(uuid.NewV7()))

	req := api.BulkImportCatalogRequestObject{
		Params: api.BulkImportCatalogParams{Entity: api.ImportEntityType("")},
	}
	resp, err := imp.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog400JSONResponse)
	require.True(t, ok, "expected 400; got %T", resp)
	require.Equal(t, api.ErrorCodeInvalidBody, r.Error)
	require.Equal(t, "unsupported_entity", r.Reason)

	// Unknown but non-empty also rejects.
	req.Params.Entity = api.ImportEntityType("widgets")
	resp, err = imp.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r2, ok := resp.(api.BulkImportCatalog400JSONResponse)
	require.True(t, ok)
	require.Equal(t, "unsupported_entity", r2.Reason)
}

// TestBulkImportCatalog_NoContentType_400_unsupported — JSONBody nil +
// Body nil + valid entity → 400 unsupported_content_type.
func TestBulkImportCatalog_NoContentType_400_unsupported(t *testing.T) {
	t.Parallel()
	imp := newDispatchOnlyImporter()
	ctx := orgkey.SetOrgID(context.Background(), uuid.Must(uuid.NewV7()))

	req := api.BulkImportCatalogRequestObject{
		Params: api.BulkImportCatalogParams{Entity: api.Agents},
	}
	resp, err := imp.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog400JSONResponse)
	require.True(t, ok, "expected 400; got %T", resp)
	require.Equal(t, "unsupported_content_type", r.Reason)
}

// TestBulkImportCatalog_CSVMissingSchemaVersion_400 — CSV body + nil
// SchemaVersion → 400 with reason starting "unsupported_schema_version".
func TestBulkImportCatalog_CSVMissingSchemaVersion_400(t *testing.T) {
	t.Parallel()
	imp := newDispatchOnlyImporter()
	ctx := orgkey.SetOrgID(context.Background(), uuid.Must(uuid.NewV7()))

	body := bytes.NewReader([]byte("code,name,email\n"))
	req := api.BulkImportCatalogRequestObject{
		Params: api.BulkImportCatalogParams{Entity: api.Agents, SchemaVersion: nil},
		Body:   body,
	}
	resp, err := imp.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog400JSONResponse)
	require.True(t, ok, "expected 400; got %T", resp)
	require.True(t, strings.HasPrefix(r.Reason, "unsupported_schema_version"),
		"reason %q must start with unsupported_schema_version", r.Reason)
}

// TestBulkImportCatalog_CSVWrongSchemaVersion_400 — SchemaVersion="v0.2"
// → 400 with the same prefix.
func TestBulkImportCatalog_CSVWrongSchemaVersion_400(t *testing.T) {
	t.Parallel()
	imp := newDispatchOnlyImporter()
	ctx := orgkey.SetOrgID(context.Background(), uuid.Must(uuid.NewV7()))

	body := bytes.NewReader([]byte("code,name,email\n"))
	v02 := "v0.2"
	req := api.BulkImportCatalogRequestObject{
		Params: api.BulkImportCatalogParams{Entity: api.Agents, SchemaVersion: &v02},
		Body:   body,
	}
	resp, err := imp.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog400JSONResponse)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(r.Reason, "unsupported_schema_version"))
}

// TestBulkImportCatalog_JSONOversize501Rows_413 — JSONBody with 501
// elements → 413 with the canonical reason. No DB touched.
func TestBulkImportCatalog_JSONOversize501Rows_413(t *testing.T) {
	t.Parallel()
	imp := newDispatchOnlyImporter()
	ctx := orgkey.SetOrgID(context.Background(), uuid.Must(uuid.NewV7()))

	body := make([]interface{}, 501)
	for i := range body {
		body[i] = map[string]interface{}{"code": "a", "name": "b", "email": "c@d.com"}
	}
	req := api.BulkImportCatalogRequestObject{
		Params:   api.BulkImportCatalogParams{Entity: api.Agents},
		JSONBody: &body,
	}
	resp, err := imp.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog413JSONResponse)
	require.True(t, ok, "expected 413; got %T", resp)
	require.Equal(t, "request_too_large_use_async_pathway", r.Reason)
}

// TestBulkImportCatalog_ZeroRowsAfterHeader_200_EmptyArrays — CSV with
// header only, no data rows → 200 + empty Succeeded + empty Failed
// (Open Q6 — "zero-rows-after-header is vacuously a success").
//
// Uses the testcontainer fixture because runImportPipeline's empty
// guard short-circuits BEFORE createJob — but the CSV path's
// validateHeader call requires a valid entity, which agents has;
// dispatch-only Importer suffices.
func TestBulkImportCatalog_ZeroRowsAfterHeader_200_EmptyArrays(t *testing.T) {
	t.Parallel()
	imp := newDispatchOnlyImporter()
	ctx := orgkey.SetOrgID(context.Background(), uuid.Must(uuid.NewV7()))

	body := bytes.NewReader([]byte("code,name,email\n"))
	req := api.BulkImportCatalogRequestObject{
		Params: api.BulkImportCatalogParams{Entity: api.Agents, SchemaVersion: schemaV01()},
		Body:   body,
	}
	resp, err := imp.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog200JSONResponse)
	require.True(t, ok, "expected 200 empty result; got %T", resp)
	require.Empty(t, r.Succeeded, "Succeeded must be []")
	require.NotNil(t, r.Succeeded, "Succeeded must be empty slice (not nil)")
	require.Empty(t, r.Failed, "Failed must be []")
	require.NotNil(t, r.Failed, "Failed must be empty slice (not nil)")
}

// TestBulkImportCatalog_EmptyCSVBody_200_EmptyArrays — entirely empty
// CSV body (no header, no rows) → 200 empty (Open Q6 extension).
func TestBulkImportCatalog_EmptyCSVBody_200_EmptyArrays(t *testing.T) {
	t.Parallel()
	imp := newDispatchOnlyImporter()
	ctx := orgkey.SetOrgID(context.Background(), uuid.Must(uuid.NewV7()))

	body := bytes.NewReader(nil)
	req := api.BulkImportCatalogRequestObject{
		Params: api.BulkImportCatalogParams{Entity: api.Agents, SchemaVersion: schemaV01()},
		Body:   body,
	}
	resp, err := imp.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog200JSONResponse)
	require.True(t, ok, "expected 200 empty result; got %T", resp)
	require.Empty(t, r.Succeeded)
	require.Empty(t, r.Failed)
}

// TestBulkImportCatalog_InvalidUTF8_400 — CSV with invalid UTF-8 → 400
// csv_not_utf8 (D5-08).
func TestBulkImportCatalog_InvalidUTF8_400(t *testing.T) {
	t.Parallel()
	imp := newDispatchOnlyImporter()
	ctx := orgkey.SetOrgID(context.Background(), uuid.Must(uuid.NewV7()))

	// Byte sequence with an invalid UTF-8 continuation byte.
	body := bytes.NewReader([]byte{0xC3, 0x28, 'a', 'b'})
	req := api.BulkImportCatalogRequestObject{
		Params: api.BulkImportCatalogParams{Entity: api.Agents, SchemaVersion: schemaV01()},
		Body:   body,
	}
	resp, err := imp.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog400JSONResponse)
	require.True(t, ok, "expected 400; got %T", resp)
	require.Equal(t, "csv_not_utf8", r.Reason)
}

// TestBulkImportCatalog_InvalidHeader_400 — CSV header missing required
// columns → 400 invalid_header:missing=... .
func TestBulkImportCatalog_InvalidHeader_400(t *testing.T) {
	t.Parallel()
	imp := newDispatchOnlyImporter()
	ctx := orgkey.SetOrgID(context.Background(), uuid.Must(uuid.NewV7()))

	// Agents requires code,name,email; we only ship code.
	body := bytes.NewReader([]byte("code\n"))
	req := api.BulkImportCatalogRequestObject{
		Params: api.BulkImportCatalogParams{Entity: api.Agents, SchemaVersion: schemaV01()},
		Body:   body,
	}
	resp, err := imp.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog400JSONResponse)
	require.True(t, ok, "expected 400; got %T", resp)
	require.True(t, strings.HasPrefix(r.Reason, "invalid_header"),
		"reason %q must start with invalid_header", r.Reason)
	require.Contains(t, r.Reason, "name")
	require.Contains(t, r.Reason, "email")
}

// TestBulkImportCatalog_IdempotencyHit_ReturnsReplay — pre-seed an
// import_jobs row with idempotency_key K; call BulkImportCatalog with
// the same key; verify the replay path returns 200 with
// idempotent_replay=true and the persisted Failed errors.
//
// Uses the testcontainer fixture because lookupIdempotentReplay runs
// a real DB query.
func TestBulkImportCatalog_IdempotencyHit_ReturnsReplay(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Seed an import_jobs row with a known idempotency_key. The errors
	// JSONB carries the wire shape that rehydrateBulkImportResult must
	// echo verbatim.
	jobID := uuid.Must(uuid.NewV7())
	key := uuid.Must(uuid.NewV7())
	keyStr := key.String()
	errorsJSON := []byte(`[{"row":1,"error":"import_failed","reason":"duplicate_code"}]`)
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO import_jobs
			(id, org_id, entity_type, status, total_rows, succeeded_rows, failed_rows, errors, idempotency_key, created_at, updated_at)
		 VALUES ($1, $2, 'agents', 'completed', 1, 0, 1, $3::jsonb, $4, NOW(), NOW())`,
		jobID, th.OrgID, errorsJSON, keyStr,
	)
	require.NoError(t, err)

	openKey := openapi_types.UUID(key)
	body := []interface{}{} // empty body; replay short-circuits before importJSON.
	req := api.BulkImportCatalogRequestObject{
		Params: api.BulkImportCatalogParams{
			Entity:         api.Agents,
			IdempotencyKey: &openKey,
		},
		JSONBody: &body,
	}
	resp, err := th.I.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog200JSONResponse)
	require.True(t, ok, "expected 200 replay; got %T", resp)
	require.NotNil(t, r.IdempotentReplay)
	require.True(t, *r.IdempotentReplay, "replay flag must be true")
	require.Empty(t, r.Succeeded, "v0.1 KNOWN LIMITATION: succeeded[] is empty on replay (D5-13)")
	require.Len(t, r.Failed, 1, "failed[] rehydrated from persisted errors JSONB")
	require.Equal(t, 1, r.Failed[0].Row)
	require.Equal(t, "duplicate_code", r.Failed[0].Reason)
}

// TestBulkImportCatalog_StatusDecision_AllSucceed_200 — JSON path with
// 3 valid agents; chunk loop succeeds for all; status decision lands
// at 200.
func TestBulkImportCatalog_StatusDecision_AllSucceed_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	body := []interface{}{
		map[string]interface{}{"code": "agent_a", "name": "Agent A", "email": "a@example.com"},
		map[string]interface{}{"code": "agent_b", "name": "Agent B", "email": "b@example.com"},
		map[string]interface{}{"code": "agent_c", "name": "Agent C", "email": "c@example.com"},
	}
	req := api.BulkImportCatalogRequestObject{
		Params:   api.BulkImportCatalogParams{Entity: api.Agents},
		JSONBody: &body,
	}
	resp, err := th.I.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog200JSONResponse)
	require.True(t, ok, "expected 200; got %T", resp)
	require.Len(t, r.Succeeded, 3, "all rows must land")
	require.Empty(t, r.Failed)
}

// TestBulkImportCatalog_StatusDecision_AllFail_422 — every row violates
// the code regex; chunk loop returns only failures; status decision
// lands at 422.
func TestBulkImportCatalog_StatusDecision_AllFail_422(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Invalid code format (uppercase) — Layer 1 regex rejects each row
	// before reaching the DB.
	body := []interface{}{
		map[string]interface{}{"code": "INVALID_UPPER", "name": "X", "email": "x@example.com"},
		map[string]interface{}{"code": "ANOTHER_BAD", "name": "Y", "email": "y@example.com"},
	}
	req := api.BulkImportCatalogRequestObject{
		Params:   api.BulkImportCatalogParams{Entity: api.Agents},
		JSONBody: &body,
	}
	resp, err := th.I.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog422JSONResponse)
	require.True(t, ok, "expected 422; got %T", resp)
	require.Empty(t, r.Succeeded)
	require.Len(t, r.Failed, 2)
	require.Equal(t, "invalid_code_format", r.Failed[0].Reason)
}

// TestBulkImportCatalog_StatusDecision_Partial_207 — one valid row + one
// invalid row → status decision lands at 207.
func TestBulkImportCatalog_StatusDecision_Partial_207(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	body := []interface{}{
		map[string]interface{}{"code": "valid_one", "name": "Valid", "email": "v@example.com"},
		map[string]interface{}{"code": "INVALID_UPPER", "name": "X", "email": "x@example.com"},
	}
	req := api.BulkImportCatalogRequestObject{
		Params:   api.BulkImportCatalogParams{Entity: api.Agents},
		JSONBody: &body,
	}
	resp, err := th.I.BulkImportCatalog(ctx, req)
	require.NoError(t, err)
	r, ok := resp.(api.BulkImportCatalog207JSONResponse)
	require.True(t, ok, "expected 207; got %T", resp)
	require.Len(t, r.Succeeded, 1)
	require.Len(t, r.Failed, 1)
	require.Equal(t, 2, r.Failed[0].Row, "row index preserved 1-based from JSON array position")
}

// _ = io.Discard keep-alive — guards against the test file losing its io
// import after edits.
var _ = io.Discard
