// orgdb_savepoint_test.go — integration tests for (*OrgTx).BeginSavepoint
// added in Phase 5 Wave 0 (D5-09). Proves savepoint rollback isolates
// per-savepoint INSERTs while preserving outer-tx work and that the
// SQLChecker preflight pointer carries into the child Tx.
//
// Test isolation strategy:
//   - One testcontainer Postgres per test (not a shared TestMain pool —
//     this is plumbing-validation, not a hot test suite, so the bring-up
//     budget is acceptable and avoids introducing a TestMain into the db
//     package which currently has only unit tests).
//   - `skills` table from migration 000002 serves as the sentinel — it is
//     the smallest catalog table that exists post-Phase-04.1 (the legacy
//     `_scaffold` table was dropped by 000002). Schema essentials:
//     (id, org_id, code, external_id?, name, description?, skill_type,
//     enabled, version, created_at, updated_at) — minimum non-null INSERT
//     columns: id, org_id, code, name, skill_type.
//   - testing.Short() skips: docker not needed for the existing -short
//     unit tests in this package (TestOrgDB_*_NoOrg suite).
package db

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/testsupport"
)

// insertSkill is a tiny sentinel writer used across the savepoint tests
// to keep test bodies focused on the savepoint semantics, not on
// recapping the skills NOT-NULL surface. Targets an OrgTx; OrgTx and
// OrgDB share the same Exec signature but only OrgTx is needed here.
func insertSkill(ctx context.Context, t *testing.T, tx *OrgTx, orgID uuid.UUID, code string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := tx.Exec(ctx,
		`INSERT INTO skills (id, org_id, code, name, skill_type) VALUES ($1, $2, $3, $4, $5)`,
		id, orgID, code, code, "general",
	)
	require.NoError(t, err, "INSERT skills(%s)", code)
	return id
}

// TestOrgTx_BeginSavepoint_RollbackIsolatesPerRowFailure proves the
// canonical Phase 5 chunk-loop semantic (D5-09): a savepoint INSERT can
// roll back without aborting the enclosing tx. The outer-tx INSERT
// must survive Commit even after the savepoint rolled back.
//
// Hazard 1 (RESEARCH §Pitfall 8): pgx auto-generates SAVEPOINT names —
// we never issue `SAVEPOINT row_N` strings ourselves.
//
// Hazard 2 (D5-09 contract): the child *OrgTx carries the SAME checker
// pointer + ValidationMode as the parent — preflight semantics must NOT
// diverge. This test exercises the child via .Exec which routes through
// preflight; if the pointer was dropped the checker would reject our
// org-id-scoped INSERT for the WRONG reason (missing checker, not
// missing filter).
func TestOrgTx_BeginSavepoint_RollbackIsolatesPerRowFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("docker required: skipping savepoint integration test under -short")
	}

	tdb := testsupport.StartPostgres(t)
	ctx := context.Background()
	orgID := testsupport.FreshOrgID(t)
	scopedCtx := orgkey.SetOrgID(ctx, orgID)

	checker := NewSQLChecker()
	orgDB := NewOrgDB(tdb.Pool, checker, ValidationError)

	outerTx, err := orgDB.BeginTx(scopedCtx)
	require.NoError(t, err, "begin outer tx")
	defer func() { _ = outerTx.Rollback(scopedCtx) }() // safe after commit — pgx ignores.

	// (1) Outer-tx INSERT — survives the whole test.
	survivorID := insertSkill(scopedCtx, t, outerTx, orgID, "outer_tx_survives")

	// (2) Open savepoint, INSERT sentinel row, ROLLBACK TO savepoint.
	sp1, err := outerTx.BeginSavepoint(scopedCtx)
	require.NoError(t, err, "begin savepoint 1")

	_ = insertSkill(scopedCtx, t, sp1, orgID, "sp1_rolled_back")

	require.NoError(t, sp1.Rollback(scopedCtx), "savepoint Rollback (ROLLBACK TO)")

	// (3) Outer tx commits — the survivor row lands, the rolled-back row does not.
	require.NoError(t, outerTx.Commit(scopedCtx), "outer-tx Commit")

	// (4) Verify with a fresh transactional read scoped to the same org.
	verifyTx, err := orgDB.BeginTx(scopedCtx)
	require.NoError(t, err, "begin verify tx")
	defer func() { _ = verifyTx.Rollback(scopedCtx) }()

	var count int
	row := verifyTx.QueryRow(scopedCtx,
		`SELECT COUNT(*) FROM skills WHERE org_id = $1`, orgID)
	require.NoError(t, row.Scan(&count), "count scan")
	require.Equal(t, 1, count, "exactly one row should survive (the outer-tx INSERT)")

	var survivedCode string
	row = verifyTx.QueryRow(scopedCtx,
		`SELECT code FROM skills WHERE id = $1 AND org_id = $2`,
		survivorID, orgID)
	require.NoError(t, row.Scan(&survivedCode), "survivor scan")
	require.Equal(t, "outer_tx_survives", survivedCode, "outer-tx INSERT must be the survivor")
}

// TestOrgTx_BeginSavepoint_ReleaseLandsSavepointWrite proves the inverse:
// when the savepoint Commits (RELEASE SAVEPOINT) and the outer tx commits,
// the savepoint INSERT lands in the final committed state. This is the
// happy-path branch of the chunk loop (D5-09): success row → release.
func TestOrgTx_BeginSavepoint_ReleaseLandsSavepointWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("docker required: skipping savepoint integration test under -short")
	}

	tdb := testsupport.StartPostgres(t)
	ctx := context.Background()
	orgID := testsupport.FreshOrgID(t)
	scopedCtx := orgkey.SetOrgID(ctx, orgID)

	checker := NewSQLChecker()
	orgDB := NewOrgDB(tdb.Pool, checker, ValidationError)

	outerTx, err := orgDB.BeginTx(scopedCtx)
	require.NoError(t, err, "begin outer tx")
	defer func() { _ = outerTx.Rollback(scopedCtx) }()

	// Outer-tx INSERT.
	_ = insertSkill(scopedCtx, t, outerTx, orgID, "outer_row")

	// Savepoint INSERT + RELEASE.
	sp, err := outerTx.BeginSavepoint(scopedCtx)
	require.NoError(t, err, "begin savepoint")

	_ = insertSkill(scopedCtx, t, sp, orgID, "sp_row_kept")
	require.NoError(t, sp.Commit(scopedCtx), "savepoint Commit (RELEASE)")

	require.NoError(t, outerTx.Commit(scopedCtx), "outer-tx Commit")

	// Verify both rows landed.
	verifyTx, err := orgDB.BeginTx(scopedCtx)
	require.NoError(t, err)
	defer func() { _ = verifyTx.Rollback(scopedCtx) }()

	var count int
	row := verifyTx.QueryRow(scopedCtx,
		`SELECT COUNT(*) FROM skills WHERE org_id = $1`, orgID)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 2, count, "both outer-tx and savepoint rows should land")
}

// TestOrgTx_BeginSavepoint_PreservesSQLCheckerPreflight proves that the
// child Tx's SQLChecker is the SAME pointer as the parent's. Without this,
// a savepoint Exec could bypass the org_id filter validator (D-02 / D5-09
// contract). The test uses ValidationError mode + an unscoped UPDATE
// missing org_id; the child Tx must reject with ErrSQLMissingOrgFilter,
// proving the checker pointer carried into the child.
//
// This is a stronger guarantee than just "savepoint works" — it locks the
// invariant that BeginSavepoint preserves the entire orgDB enforcement
// surface, not just the transactional semantics.
func TestOrgTx_BeginSavepoint_PreservesSQLCheckerPreflight(t *testing.T) {
	if testing.Short() {
		t.Skip("docker required: skipping savepoint integration test under -short")
	}

	tdb := testsupport.StartPostgres(t)
	ctx := context.Background()
	orgID := testsupport.FreshOrgID(t)
	scopedCtx := orgkey.SetOrgID(ctx, orgID)

	checker := NewSQLChecker()
	orgDB := NewOrgDB(tdb.Pool, checker, ValidationError)

	outerTx, err := orgDB.BeginTx(scopedCtx)
	require.NoError(t, err)
	defer func() { _ = outerTx.Rollback(scopedCtx) }()

	sp, err := outerTx.BeginSavepoint(scopedCtx)
	require.NoError(t, err, "begin savepoint")

	// Unscoped SQL — no org_id filter. Preflight on the CHILD Tx must
	// reject; if BeginSavepoint dropped the checker the call would reach
	// pgx and a server-side error would surface instead.
	_, err = sp.Exec(scopedCtx, `UPDATE skills SET name = $1 WHERE id = $2`, "x", uuid.New())
	require.Error(t, err, "unscoped SQL on child savepoint Tx must error")
	require.True(t, errors.Is(err, ErrSQLMissingOrgFilter),
		"expected ErrSQLMissingOrgFilter, got %v", err)

	// Cleanup: roll back the savepoint then the outer tx (both safe even
	// though the unscoped SQL never reached the DB — preflight rejected it).
	_ = sp.Rollback(scopedCtx)
}

// TestOrgTx_BeginSavepoint_OnClosedTx_ReturnsError documents RESEARCH
// Pitfall 8: calling BeginSavepoint on a Tx that has already committed
// or rolled back returns a typed error from pgx (`tx is closed`). The
// Phase 5 chunk loop must not retry savepoint creation after the outer
// tx terminates; this test locks that contract.
func TestOrgTx_BeginSavepoint_OnClosedTx_ReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("docker required: skipping savepoint integration test under -short")
	}

	tdb := testsupport.StartPostgres(t)
	ctx := context.Background()
	orgID := testsupport.FreshOrgID(t)
	scopedCtx := orgkey.SetOrgID(ctx, orgID)

	checker := NewSQLChecker()
	orgDB := NewOrgDB(tdb.Pool, checker, ValidationError)

	outerTx, err := orgDB.BeginTx(scopedCtx)
	require.NoError(t, err)

	// Close the outer tx before attempting a savepoint.
	require.NoError(t, outerTx.Rollback(scopedCtx))

	_, err = outerTx.BeginSavepoint(scopedCtx)
	require.Error(t, err, "BeginSavepoint on closed Tx must return error")
	// pgx returns "tx is closed" — match on the wrapped prefix to avoid
	// coupling to the exact pgx version's wording while still confirming
	// the error is surfaced through the orgtx layer.
	require.Contains(t, err.Error(), "orgtx: begin savepoint:",
		"error must be wrapped with orgtx: prefix; got %v", err)
}
