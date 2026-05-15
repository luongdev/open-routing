package db

import (
	"context"
	"testing"
)

// TestOrgDB_QueryRow_ValidationError_NoOrg proves that QueryRow in
// ValidationError mode returns a typed error from Scan instead of panicking
// when preflight fails due to a missing org_id in ctx (D-02, HIGH-3).
//
// A nil pool is safe here: preflight fails on missing ctx org_id before
// pool.QueryRow is ever called, so no real DB connection is required.
func TestOrgDB_QueryRow_ValidationError_NoOrg(t *testing.T) {
	t.Parallel()
	// ctx has no org_id — preflight returns ErrOrgIDMissingFromContext.
	ctx := context.Background()
	checker := NewSQLChecker()
	// nil pool: QueryRow only reaches pool.QueryRow if preflight passes;
	// preflight fails on missing ctx org_id before touching the pool.
	o := NewOrgDB(nil, checker, ValidationError)
	row := o.QueryRow(ctx, `SELECT id FROM _scaffold WHERE org_id = $1`, "ignored")
	if err := row.Scan(new(string)); err == nil {
		t.Fatal("expected error from Scan on errRow, got nil")
	}
}
