// imports_migration_idempotent_test.go — D-90 / Plan 05-07 Task 4
// idempotency smoke for migration 000003_create_import_jobs.
//
// Strategy (mirrors migration_idempotent_test.go's Phase 04.1 template
// for the backfill CTE):
//
//  1. The shared testcontainer (main_test.go TestMain) already applied
//     all 3 migrations during bring-up; import_jobs exists with the
//     CHECK constraints + 3 indexes (ix_import_jobs_org_created,
//     ix_import_jobs_pending_updated, uq_import_jobs_org_idempotency).
//
//  2. INSERT 3 import_jobs rows in a fresh org with disambiguated
//     entity types + idempotency_key shapes (1 with key, 2 without)
//     so the partial-unique index sees both code paths.
//
//  3. Capture the post-state pg_catalog snapshot: (a) table columns +
//     types, (b) index definitions, (c) check constraints. These are
//     the migration's invariants; replaying must not mutate them.
//
//  4. Re-execute the CREATE TABLE / CREATE INDEX statements from
//     migrations/000003_create_import_jobs.up.sql verbatim against the
//     LIVE pool. The migration uses bare CREATE TABLE (NOT IF NOT
//     EXISTS), so PostgreSQL MUST raise "relation already exists" —
//     this is the SAFE/EXPECTED idempotency signal (the migration is
//     not designed to be replayed via raw SQL; it is replay-safe in
//     the golang-migrate-bracketed sense because the schema_migrations
//     table guards against double-application).
//
//  5. Assert: the error is the EXPECTED "already exists" PostgreSQL
//     SQLSTATE 42P07 — NOT some other failure shape that would indicate
//     schema drift. Then re-capture the snapshot and assert byte-for-
//     byte equality with the pre-replay snapshot.
//
// Why this is the right shape:
//   - golang-migrate guards replay via the schema_migrations table;
//     running migrate.Up twice is already a no-op (Plan 05-04 tested).
//   - The risk migration_idempotent_test.go defends against is schema
//     DRIFT — if the migration accidentally evolves and an old +
//     a new version produce different schema state, the test catches it.
//   - For migration 000003 there is no backfill CTE (unlike 000002),
//     so we test the DDL-only idempotency path: the same DDL must
//     fail consistently with the SAME error code, and the schema
//     state must be unchanged.
package isolation_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// snapshotImportJobsSchema returns three parallel slices describing
// the import_jobs schema state: column definitions, index definitions,
// and check constraint expressions. Used to assert zero schema drift
// between pre + post replay.
type schemaSnapshot struct {
	Columns     []string
	Indexes     []string
	CheckConstr []string
}

func snapshotImportJobsSchema(t *testing.T, ctx context.Context) schemaSnapshot {
	t.Helper()
	snap := schemaSnapshot{}

	// (1) Column definitions: name + type + nullable + default. The
	// ORDER BY ordinal_position pins the column order so a future
	// migration that reorders columns lands as a diff.
	colRows, err := sharedPool.Query(ctx, `
		SELECT column_name, data_type, is_nullable, column_default
		  FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = 'import_jobs'
		 ORDER BY ordinal_position`)
	require.NoError(t, err)
	for colRows.Next() {
		var name, dataType, nullable string
		var defaultExpr *string
		require.NoError(t, colRows.Scan(&name, &dataType, &nullable, &defaultExpr))
		def := "NULL"
		if defaultExpr != nil {
			def = *defaultExpr
		}
		snap.Columns = append(snap.Columns, name+"|"+dataType+"|"+nullable+"|"+def)
	}
	colRows.Close()

	// (2) Index definitions: pg_indexes captures the full CREATE INDEX
	// statement including the partial WHERE clause.
	idxRows, err := sharedPool.Query(ctx, `
		SELECT indexname, indexdef
		  FROM pg_indexes
		 WHERE schemaname = 'public' AND tablename = 'import_jobs'
		 ORDER BY indexname`)
	require.NoError(t, err)
	for idxRows.Next() {
		var name, def string
		require.NoError(t, idxRows.Scan(&name, &def))
		snap.Indexes = append(snap.Indexes, name+"|"+def)
	}
	idxRows.Close()

	// (3) Check constraints: pg_constraint with contype='c'. Captures
	// the entity_type + status enumerated value lists.
	chkRows, err := sharedPool.Query(ctx, `
		SELECT con.conname, pg_get_constraintdef(con.oid)
		  FROM pg_constraint con
		  JOIN pg_class cls ON cls.oid = con.conrelid
		 WHERE cls.relname = 'import_jobs' AND con.contype = 'c'
		 ORDER BY con.conname`)
	require.NoError(t, err)
	for chkRows.Next() {
		var name, def string
		require.NoError(t, chkRows.Scan(&name, &def))
		snap.CheckConstr = append(snap.CheckConstr, name+"|"+def)
	}
	chkRows.Close()

	return snap
}

// TestMigration_ImportJobs_Idempotent — replay the migration 000003
// DDL verbatim and assert (a) PostgreSQL rejects each statement with
// the canonical "already exists" SQLSTATE, (b) the post-replay schema
// snapshot is byte-equivalent to the pre-replay snapshot.
//
// The migration uses bare CREATE TABLE / CREATE INDEX (no IF NOT
// EXISTS) because golang-migrate guarantees one-shot application via
// the schema_migrations version table; raw SQL replay is testing the
// "what if golang-migrate forgot to track this" scenario.
func TestMigration_ImportJobs_Idempotent(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	ctx := t.Context()

	org := uuid.Must(uuid.NewV7())

	// (1) Seed 3 rows: 1 with idempotency_key (exercises the partial
	// UNIQUE index) + 2 without (exercises the NULL-skip branch).
	type seedRow struct {
		entity string
		key    *string
	}
	keyA := uuid.Must(uuid.NewV7()).String()
	seeds := []seedRow{
		{entity: "agents", key: &keyA},
		{entity: "skills", key: nil},
		{entity: "queues", key: nil},
	}
	for _, s := range seeds {
		_, err := sharedPool.Exec(ctx,
			`INSERT INTO import_jobs (id, org_id, entity_type, status, total_rows, succeeded_rows, failed_rows, idempotency_key)
			 VALUES ($1, $2, $3, 'completed', 0, 0, 0, $4)`,
			uuid.Must(uuid.NewV7()), org, s.entity, s.key,
		)
		require.NoError(t, err, "seed import_jobs (entity=%s)", s.entity)
	}

	// (2) Pre-replay snapshot.
	preSnap := snapshotImportJobsSchema(t, ctx)
	require.NotEmpty(t, preSnap.Columns, "pre-replay snapshot must include columns")
	require.NotEmpty(t, preSnap.Indexes, "pre-replay snapshot must include indexes")
	require.NotEmpty(t, preSnap.CheckConstr, "pre-replay snapshot must include CHECK constraints")

	// (3) Replay each migration statement individually so the test can
	// assert the EXPECTED SQLSTATE per statement. The DDL is reproduced
	// verbatim from migrations/000003_create_import_jobs.up.sql lines
	// 31-63 (CREATE TABLE + CREATE INDEX × 3). Drift between the test
	// and the migration is caught by manually re-syncing this block
	// when the migration changes (acceptable v0.1 trade-off — Phase 5
	// has only one such test).
	replayStmts := []struct {
		name string
		sql  string
		// expectedSQLState is the PostgreSQL SQLSTATE we expect on
		// replay. 42P07 = duplicate_table; 42P06 = duplicate_schema;
		// 42710 = duplicate_object (covers INDEX exists in newer PG
		// versions but the practical code returned for CREATE INDEX
		// when relation exists is also 42P07 in PG 17).
		expectedSQLState string
	}{
		{
			name: "CREATE TABLE import_jobs",
			sql: `CREATE TABLE import_jobs (
				id              UUID PRIMARY KEY,
				org_id          UUID NOT NULL,
				entity_type     TEXT NOT NULL
				                  CHECK (entity_type IN ('agents','skills','queues','channels','adapters','break_reasons')),
				status          TEXT NOT NULL DEFAULT 'pending'
				                  CHECK (status IN ('pending','completed','failed')),
				total_rows      INTEGER NOT NULL DEFAULT 0,
				succeeded_rows  INTEGER NOT NULL DEFAULT 0,
				failed_rows     INTEGER NOT NULL DEFAULT 0,
				errors          JSONB,
				idempotency_key TEXT,
				created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			expectedSQLState: "42P07",
		},
		{
			name: "CREATE INDEX ix_import_jobs_org_created",
			sql: `CREATE INDEX ix_import_jobs_org_created
				ON import_jobs (org_id, created_at DESC, id DESC)`,
			expectedSQLState: "42P07",
		},
		{
			name: "CREATE INDEX ix_import_jobs_pending_updated",
			sql: `CREATE INDEX ix_import_jobs_pending_updated
				ON import_jobs (updated_at)
				WHERE status = 'pending'`,
			expectedSQLState: "42P07",
		},
		{
			name: "CREATE UNIQUE INDEX uq_import_jobs_org_idempotency",
			sql: `CREATE UNIQUE INDEX uq_import_jobs_org_idempotency
				ON import_jobs (org_id, idempotency_key)
				WHERE idempotency_key IS NOT NULL`,
			expectedSQLState: "42P07",
		},
	}

	for _, stmt := range replayStmts {
		_, err := sharedPool.Exec(ctx, stmt.sql)
		require.Errorf(t, err, "replay %q must error — schema already exists", stmt.name)
		var pgErr *pgconn.PgError
		require.ErrorAsf(t, err, &pgErr,
			"replay %q error must be a *pgconn.PgError; got %T", stmt.name, err)
		require.Equalf(t, stmt.expectedSQLState, pgErr.Code,
			"replay %q SQLSTATE mismatch — expected %s (already exists); got %s (%s)",
			stmt.name, stmt.expectedSQLState, pgErr.Code, pgErr.Message)
	}

	// (4) Post-replay snapshot. Must equal pre-replay byte-for-byte.
	postSnap := snapshotImportJobsSchema(t, ctx)
	require.Equal(t, preSnap.Columns, postSnap.Columns,
		"idempotent: columns must be unchanged by replay")
	require.Equal(t, preSnap.Indexes, postSnap.Indexes,
		"idempotent: indexes must be unchanged by replay")
	require.Equal(t, preSnap.CheckConstr, postSnap.CheckConstr,
		"idempotent: CHECK constraints must be unchanged by replay")

	// (5) Sanity: seeded rows still exist with their original shape.
	var rowCount int
	require.NoError(t, sharedPool.QueryRow(ctx,
		`SELECT count(*) FROM import_jobs WHERE org_id = $1`, org,
	).Scan(&rowCount))
	require.Equal(t, 3, rowCount, "seeded rows must survive DDL-replay failure")
}
