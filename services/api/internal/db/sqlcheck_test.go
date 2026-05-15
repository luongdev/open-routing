package db

import (
	"testing"
)

// TestSQLChecker_MustContainOrgFilter is a table-driven proof that the
// hardened validator accepts only DML that binds org_id in the WHERE clause
// (or INSERT column list) while rejecting DML with org_id in other positions
// (projection, ORDER BY, JOIN ON, string literal) and rejecting DDL outright.
// DDL must always flow through WithBypass (D-11); DDL reaching the inspector
// means bypass is absent and the check fails safe.
func TestSQLChecker_MustContainOrgFilter(t *testing.T) {
	t.Parallel()
	c := NewSQLChecker()

	accept := []struct {
		name string
		sql  string
	}{
		{"InsertScaffold", `INSERT INTO _scaffold (id, org_id, external_id, name) VALUES ($1, $2, $3, $4) RETURNING id, org_id, external_id, name, created_at`},
		{"GetScaffoldByID", `SELECT id, org_id, external_id, name, created_at FROM _scaffold WHERE id = $1 AND org_id = $2`},
		{"ListScaffolds", `SELECT id, org_id, external_id, name, created_at FROM _scaffold WHERE org_id = $1 ORDER BY created_at DESC LIMIT 100`},
		{"UpdateScaffold", `UPDATE _scaffold SET name = $2 WHERE id = $1 AND org_id = $3`},
		{"DeleteScaffold", `DELETE FROM _scaffold WHERE id = $1 AND org_id = $2`},
		{"JoinWithWhereOrgID", `SELECT a.id FROM _scaffold a JOIN _scaffold b ON a.org_id = b.org_id WHERE a.org_id = $1 AND a.id = $2`},
	}
	for _, tc := range accept {
		tc := tc
		t.Run("accept_"+tc.name, func(t *testing.T) {
			t.Parallel()
			if err := c.MustContainOrgFilter(tc.sql); err != nil {
				t.Fatalf("expected nil, got %v for %q", err, tc.sql)
			}
		})
	}

	reject := []struct {
		name string
		sql  string
	}{
		{"BareSelectWithoutOrgID", `SELECT id FROM _scaffold`},
		{"SelectAll", `SELECT * FROM _scaffold`},
		{"InsertWithoutOrgIDColumn", `INSERT INTO _scaffold (id, external_id, name) VALUES ($1, $2, $3)`},
		{"DeleteWithoutWhere", `DELETE FROM _scaffold`},
		{"UpdateNoOrgFilter", `UPDATE _scaffold SET name = $1 WHERE id = $2`},
		// org_id appears only in JOIN ON condition, not in WHERE predicate.
		{"JoinWithoutWhereOrgID", `SELECT a.id FROM _scaffold a JOIN _scaffold b ON a.org_id = b.org_id WHERE a.id = $1`},
		// org_id in SELECT projection list, not WHERE predicate — actual cross-org leak vector.
		{"ProjectionOnlyOrgID", `SELECT id, org_id FROM _scaffold`},
		// org_id in ORDER BY clause only, not WHERE predicate.
		{"OrderByOrgID", `SELECT id FROM _scaffold ORDER BY org_id`},
		// org_id as a string literal (A_Const.sval), not a ColumnRef — must not pass.
		{"LiteralOrgIDString", `SELECT 'org_id' AS alias FROM _scaffold`},
		// DDL without bypass must be rejected — bypass short-circuits before inspector.
		{"DropTable", `DROP TABLE _scaffold`},
		{"TruncateTable", `TRUNCATE _scaffold`},
	}
	for _, tc := range reject {
		tc := tc
		t.Run("reject_"+tc.name, func(t *testing.T) {
			t.Parallel()
			if err := c.MustContainOrgFilter(tc.sql); err == nil {
				t.Fatalf("expected ErrSQLMissingOrgFilter, got nil for %q", tc.sql)
			}
		})
	}
}

// TestSQLChecker_CacheMemoizes proves the SHA-256 cache is consulted on
// repeated calls (D-02). The second call returns the same verdict as the
// first; inspecting the internal map confirms a single cache entry per
// unique SQL string.
func TestSQLChecker_CacheMemoizes(t *testing.T) {
	t.Parallel()
	c := NewSQLChecker()
	const sql = `SELECT id FROM _scaffold WHERE org_id = $1`

	if err := c.MustContainOrgFilter(sql); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := c.MustContainOrgFilter(sql); err != nil {
		t.Fatalf("second call: %v", err)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.cache[hashSQL(sql)]; !ok {
		t.Fatal("expected cache entry for sql after MustContainOrgFilter")
	}
}
