// testutil_test.go — per-test scaffolding shared across the imports
// suite (D-73 carry-forward from state/testutil_test.go).
//
// Wave 2 (this plan) needs only the bare minimum:
//
//   - sharedPool aliasing for sweep_test.go.
//   - cleanImportTables for per-org DELETE before each test (mirrors
//     state.cleanStateTables; per-org instead of TRUNCATE so parallel
//     tests with distinct orgIDs cannot trample each other).
//   - newTestImports for building an Importer wired against
//     sharedPool with the SQLChecker in panic mode (FOUND-04 dev-test
//     contract) and a discarded logger (test output stays clean).
//
// Wave 5 (Plan 05-07) will extend this with the httptest harness +
// composite ApiHandlers wiring once Plan 05-06 ships the
// BulkImportCatalog / GetImportJob method bodies on *Importer.
//
// Why `_test.go` suffix: the Go build tool excludes _test.go files
// from production builds, so miniredis (Wave 5) and testcontainers
// never end up in the shipped binary.
//
// Why `package imports`: this lets test bodies in sweep_test.go reach
// unexported methods (runSweepPastDue, etc.) without re-exporting
// them.
package imports

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonboulle/clockwork"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db"
)

// sharedPool is the package-level pgxpool reused across every test in
// the imports package (D-73). Assigned in main_test.go's TestMain
// after the Postgres testcontainer is up + migrations applied.
// nil when -short is set or Docker is unavailable.
var sharedPool *pgxpool.Pool

// TestImports bundles the per-test fixture so test bodies can read
// orgID, the Importer under test, and the pool for raw-SQL row
// seeding (sweep_test.go uses raw INSERT to seed past-due pending
// rows — the sqlc query InsertImportJob hardcodes status='pending'
// which is fine for one path but not for the "past-due timestamp"
// shape sweep_test needs).
//
// The Pool field aliases sharedPool — kept on the struct so test
// bodies can write `th.Pool.Exec(...)` without reaching for the
// package-level var.
type TestImports struct {
	I        *Importer
	OrgDB    *db.OrgDB
	Pool     *pgxpool.Pool
	OrgID    uuid.UUID
	FakeClk  clockwork.FakeClock
	Logger   *slog.Logger
	cleanup  func()
}

// newTestImports constructs ONE Importer per test (D-73):
//   - one OrgDB wrapping sharedPool with the SQLChecker in panic
//     mode (FOUND-04 dev-test contract — any sqlc query that fails
//     the SQLChecker panics here, surfacing org-scoping regressions
//     immediately).
//   - one cache instance backed by miniredis (per-test fresh state,
//     no leaks between tests — D-49 spirit).
//   - one clockwork.FakeClock for deterministic ticker advancement.
//   - one Importer with the above deps + a 1h sweep interval (Wave 2
//     sweep tests advance the fake clock manually; the test harness
//     never relies on real-time elapsing).
//
// Skips the calling test when sharedPool == nil (TestMain skipped
// bring-up due to Docker unavailability or -short).
func newTestImports(t testing.TB) *TestImports {
	t.Helper()
	if sharedPool == nil {
		t.Skip("imports: sharedPool nil — Docker testcontainer unavailable (run without -short)")
		return nil
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	// miniredis + cache — Wave 2 sweep tests do not exercise the
	// cache (sweep only writes import_jobs), but newTestImports
	// constructs one anyway so Wave 5 handler tests inherit the
	// fixture shape with no re-wiring.
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	c := cache.New(rdb, logger)

	orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)

	fakeClock := clockwork.NewFakeClock()
	imp := New(Deps{
		OrgDB:  orgDB,
		Cache:  c,
		Logger: logger,
	}, WithClock(fakeClock), WithSweepInterval(1*time.Hour))

	// Per-test orgID. UUIDv7 because production OrgContext middleware
	// rejects UUIDv4 with 400 invalid_org_id/uuidv7_required (D-19).
	// Wave 2 tests don't go through the middleware (no httptest
	// server) but using UUIDv7 keeps the fixture shape consistent
	// with Wave 5 expectations.
	orgID := uuid.Must(uuid.NewV7())

	th := &TestImports{
		I:       imp,
		OrgDB:   orgDB,
		Pool:    sharedPool,
		OrgID:   orgID,
		FakeClk: fakeClock,
		Logger:  logger,
		cleanup: func() {
			// Importer.Stop is idempotent so unconditional cleanup is
			// safe even when the test never called Start.
			imp.Stop()
		},
	}
	t.Cleanup(th.cleanup)
	return th
}

// cleanImportTables removes import_jobs (and the catalog + state
// tables that Wave 5 handler tests will touch) for a single org.
// Per-org DELETE instead of TRUNCATE so parallel tests with distinct
// orgIDs do not stomp each other's fixtures (D-73 isolation contract).
//
// Order matches state/testutil_test.go's cleanStateTables: import_jobs
// first (it has no FKs), then the 6 catalog tables + agent_states +
// agent_skills (Wave 5 will need these clean for handler tests).
//
// No FK constraints across these tables (D-76, D-80) so order is
// technically free, but keeping it consistent helps when reading
// query plans.
func cleanImportTables(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) {
	t.Helper()
	if pool == nil {
		return
	}
	for _, stmt := range []string{
		`DELETE FROM import_jobs   WHERE org_id = $1`,
		`DELETE FROM agent_states  WHERE org_id = $1`,
		`DELETE FROM agent_skills  WHERE org_id = $1`,
		`DELETE FROM agents        WHERE org_id = $1`,
		`DELETE FROM skills        WHERE org_id = $1`,
		`DELETE FROM queues        WHERE org_id = $1`,
		`DELETE FROM channels      WHERE org_id = $1`,
		`DELETE FROM adapters      WHERE org_id = $1`,
		`DELETE FROM break_reasons WHERE org_id = $1`,
	} {
		_, err := pool.Exec(ctx, stmt, orgID)
		require.NoError(t, err, "cleanImportTables: DELETE failed for %s", stmt)
	}
}

// seedPendingImportJob inserts an import_jobs row directly via raw
// SQL with a caller-specified updated_at — necessary because the sqlc
// InsertImportJob query lets Postgres NOW() populate updated_at, and
// sweep_test needs to seed rows whose updated_at is in the past.
//
// Returns the minted job ID for later assertion.
func seedPendingImportJob(
	t testing.TB,
	ctx context.Context,
	pool *pgxpool.Pool,
	orgID uuid.UUID,
	entityType string,
	updatedAt time.Time,
) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx,
		`INSERT INTO import_jobs
			(id, org_id, entity_type, status, total_rows, succeeded_rows, failed_rows, errors, idempotency_key, created_at, updated_at)
		 VALUES ($1, $2, $3, 'pending', 0, 0, 0, NULL, NULL, $4, $4)`,
		id, orgID, entityType, updatedAt,
	)
	require.NoError(t, err, "seedPendingImportJob: INSERT")
	return id
}

// fetchImportJobStatus returns (status, errorsJSONB) for a given
// (orgID, jobID). Used by sweep_test.go to assert post-sweep state.
func fetchImportJobStatus(
	t testing.TB,
	ctx context.Context,
	pool *pgxpool.Pool,
	orgID, jobID uuid.UUID,
) (string, []byte) {
	t.Helper()
	row := pool.QueryRow(ctx,
		`SELECT status, errors FROM import_jobs WHERE id = $1 AND org_id = $2`,
		jobID, orgID,
	)
	var status string
	var errorsRaw []byte
	require.NoError(t, row.Scan(&status, &errorsRaw), "fetchImportJobStatus: scan")
	return status, errorsRaw
}
