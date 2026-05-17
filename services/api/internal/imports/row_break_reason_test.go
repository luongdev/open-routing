// row_break_reason_test.go — testcontainers integration suite for
// the break_reason row processor. Phase 04.1 dropped
// UNIQUE(org_id, name) so two rows with the same name + org are
// legal as long as their `code` values differ (IDENT-03).
//
// Test inventory (≥ 4 cases per plan):
//
//   - HappyPath_FirstImport — new break_reason lands.
//   - TwoBreakReasons_SameNameSameOrg_BothSucceed — IDENT-03 (the
//     formerly-blocking UNIQUE(name) is gone).
//   - InvalidCodeFormat_PerRow — Layer 1 reject.
//   - Reimport_UpdatesViaUpsertPath — same code → ON CONFLICT UPDATE.
package imports

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

func rawBreakReasonRow(code, name string, routable *bool) map[string]interface{} {
	out := map[string]interface{}{
		"code": code,
		"name": name,
	}
	if routable != nil {
		out["routable"] = *routable
	}
	return out
}

func fetchBreakReasonByCode(t testing.TB, ctx context.Context, th *TestImports, code string) (int, string, bool) {
	t.Helper()
	var n int
	var name string
	var routable bool
	err := th.Pool.QueryRow(ctx,
		`SELECT count(*), COALESCE(MAX(name), ''), COALESCE(BOOL_OR(routable), FALSE)
		   FROM break_reasons WHERE org_id = $1 AND code = $2`,
		th.OrgID, code,
	).Scan(&n, &name, &routable)
	require.NoError(t, err)
	return n, name, routable
}

func fetchBreakReasonsByName(t testing.TB, ctx context.Context, th *TestImports, name string) int {
	t.Helper()
	var n int
	err := th.Pool.QueryRow(ctx,
		`SELECT count(*) FROM break_reasons WHERE org_id = $1 AND name = $2`,
		th.OrgID, name,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

func TestProcessRow_BreakReason_HappyPath_FirstImport(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	r := true
	rows := []parsedRow{
		{lineNo: 1, raw: rawBreakReasonRow("br_lunch", "Lunch", &r)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.BreakReasons, &breakReasonRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)

	n, name, routable := fetchBreakReasonByCode(t, ctx, th, "br_lunch")
	require.Equal(t, 1, n)
	require.Equal(t, "Lunch", name)
	require.True(t, routable)
}

func TestProcessRow_BreakReason_TwoSameName_SameOrg_BothSucceed(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// IDENT-03 — Phase 04.1 dropped UNIQUE(org_id, name). Two break
	// reasons with the same name + org are legal as long as their
	// codes differ.
	rows := []parsedRow{
		{lineNo: 1, raw: rawBreakReasonRow("br_lunch_short", "Lunch", nil)},
		{lineNo: 2, raw: rawBreakReasonRow("br_lunch_long", "Lunch", nil)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.BreakReasons, &breakReasonRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 2)
	require.Len(t, failed, 0)
	require.Equal(t, 2, fetchBreakReasonsByName(t, ctx, th, "Lunch"),
		"IDENT-03 — Phase 04.1 dropped UNIQUE(org_id, name); two same-name break reasons in one org are legal")
}

func TestProcessRow_BreakReason_InvalidCodeFormat_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawBreakReasonRow("BadCode!", "Bad", nil)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.BreakReasons, &breakReasonRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.NotNil(t, failed[0].Field)
	require.Equal(t, "code", *failed[0].Field)
	require.Equal(t, "invalid_code_format", failed[0].Reason)
}

func TestProcessRow_BreakReason_Reimport_UpdatesViaUpsertPath(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed.
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO break_reasons (id, org_id, code, name, routable, display_order, enabled, version, created_at, updated_at)
		 VALUES (gen_random_uuid(), $1, 'br_lunch', 'Old Lunch', TRUE, 0, TRUE, 1, NOW(), NOW())`,
		th.OrgID,
	)
	require.NoError(t, err)

	r := false
	rows := []parsedRow{
		{lineNo: 1, raw: rawBreakReasonRow("br_lunch", "New Lunch", &r)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.BreakReasons, &breakReasonRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)

	n, name, routable := fetchBreakReasonByCode(t, ctx, th, "br_lunch")
	require.Equal(t, 1, n)
	require.Equal(t, "New Lunch", name)
	require.False(t, routable, "ON CONFLICT UPDATE must overwrite routable when payload sets it")
}
