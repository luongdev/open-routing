// migration_idempotent_test.go — D04_1-11 idempotency smoke. The Phase
// 04.1 backfill (row_number window dedup) must produce zero changes
// when re-executed on identical seed data. Asserts the migration is
// replay-safe (cf. Plan 01 §Pattern 4, RESEARCH §Pitfall 5).
//
// Strategy:
//  1. The shared testcontainer (main_test.go TestMain) already applied
//     the migration once during bring-up; all 6 catalog tables exist
//     with code TEXT NOT NULL and the partial unique index.
//  2. INSERT 3 agents in a fresh org with already-disambiguated codes
//     (post-dedup state). The codes are distinct so the first run of
//     the dedup CTE is a no-op too — re-running it MUST be a no-op.
//  3. Capture the post-backfill state (id, code) per row.
//  4. Re-execute the Phase 04.1 backfill dedup CTE for agents.
//  5. Assert: (a) row count preserved, (b) every code unchanged, (c)
//     id ordering unchanged.
//
// The test operates directly on sharedPool (bypassing handler validation)
// because the dedup CTE is a schema-migration concern, not an HTTP
// contract concern. The matching SQL fragment lives in
// migrations/000002_catalog_v0_1.up.sql lines 277-285 (agents) and the
// per-entity blocks at 296-304 / 315-323 / 334-342 / 353-361 / 372-380.
// Drift between this test and the migration is caught by T-04.1-22.
package isolation_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMigration_Idempotent_BackfillProducesNoOpOnRerun(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	ctx := context.Background()

	org := uuid.Must(uuid.NewV7())

	// Insert 3 agents in the same org with already-disambiguated codes
	// (post-dedup state). The row_number CTE numbers them deterministically
	// by (created_at, id); after the first run rn=1 for each since codes
	// differ. Re-running the CTE MUST then be a no-op because rn=1 for
	// every row again.
	seed := []struct {
		code string
		ext  string
		name string
	}{
		{"hr_emp_001", "HR-EMP-001", "Agent One"},
		{"hr_emp_001_a", "HR_EMP_001", "Agent Two"},
		{"hr_emp_001_b", "hr-emp-001", "Agent Three"},
	}
	for _, s := range seed {
		_, err := sharedPool.Exec(ctx,
			`INSERT INTO agents (id, org_id, code, external_id, name, email)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			uuid.Must(uuid.NewV7()), org, s.code, s.ext, s.name, s.code+"@example.test",
		)
		require.NoError(t, err, "insert seed agent code=%s", s.code)
	}

	// Capture pre-state: (id, code) tuples for the seeded org.
	type rec struct {
		ID   uuid.UUID
		Code string
	}
	preRows, err := sharedPool.Query(ctx,
		`SELECT id, code FROM agents WHERE org_id = $1 ORDER BY code`, org)
	require.NoError(t, err)
	var pre []rec
	for preRows.Next() {
		var r rec
		require.NoError(t, preRows.Scan(&r.ID, &r.Code))
		pre = append(pre, r)
	}
	preRows.Close()
	require.Len(t, pre, 3, "expected 3 seeded agents in org")

	// Re-execute the Phase 04.1 backfill dedup CTE for agents. If the
	// dedup is idempotent (D04_1-11), no rows are mutated because
	// rn=1 for every row (codes are already distinct).
	//
	// SQL fragment is byte-identical to migrations/000002_catalog_v0_1.up.sql
	// lines 277-285 modulo the WHERE org_id = $1 scoping clause added
	// here so a parallel test does not affect other rows.
	tag, err := sharedPool.Exec(ctx, `
        WITH dups AS (
            SELECT id,
                   row_number() OVER (PARTITION BY org_id, code ORDER BY created_at, id) AS rn
            FROM agents
            WHERE org_id = $1
        )
        UPDATE agents a
        SET code = a.code || '_' || dups.rn::text
        FROM dups
        WHERE a.id = dups.id AND dups.rn > 1;
    `, org)
	require.NoError(t, err, "re-running dedup CTE must not error")
	require.Equalf(t, int64(0), tag.RowsAffected(),
		"D04_1-11: dedup re-run must mutate ZERO rows on already-disambiguated seed data; got %d", tag.RowsAffected())

	// Capture post-state and compare row-by-row.
	postRows, err := sharedPool.Query(ctx,
		`SELECT id, code FROM agents WHERE org_id = $1 ORDER BY code`, org)
	require.NoError(t, err)
	var post []rec
	for postRows.Next() {
		var r rec
		require.NoError(t, postRows.Scan(&r.ID, &r.Code))
		post = append(post, r)
	}
	postRows.Close()

	require.Equal(t, len(pre), len(post), "row count must be preserved")
	for i := range pre {
		require.Equal(t, pre[i].ID, post[i].ID, "id at index %d unchanged", i)
		require.Equalf(t, pre[i].Code, post[i].Code,
			"D04_1-11: code at index %d unchanged on rerun (idempotent dedup); pre=%s post=%s",
			i, pre[i].Code, post[i].Code)
	}
}
