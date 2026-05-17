// main_test.go — package-level TestMain for the state suite (Plan 04-03).
//
// Brings up ONE Postgres testcontainer + applies all migrations once per
// `go test ./internal/state/...` invocation; per-test bring-up reuses
// `sharedPool` (declared in testutil_test.go) for speed (D-73).
//
// Pattern adapted verbatim from services/api/internal/catalog/main_test.go.
// The only differences:
//   - Database name: `state_test` (vs `catalog_test`) for log grep clarity.
//   - Migrations path: `file://../../../../migrations` from
//     services/api/internal/state/  (4 levels up to repo root) — same
//     depth as the catalog suite.
//   - No httptest server / no redis here — those are per-test in
//     testutil_test.go's newTestHandlers (one miniredis per test, so cache
//     state never leaks between tests, D-49 spirit).
//
// -short skip + CI exit-code policy mirror the catalog suite (D-77 +
// Wave 1 review): docker-unavailable in CI = fail loudly; docker-unavailable
// locally = skip gracefully.
package state

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql shim for migrate (Pitfall 7)
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// containerFailureExitCode returns 1 when CI env var is non-empty (GitHub
// Actions sets CI=true); 0 otherwise. Distinguishes "Docker unavailable
// locally — skip gracefully" from "Docker unavailable in CI — fail loudly".
// Mirrors services/api/internal/catalog/main_test.go.
func containerFailureExitCode() int {
	if os.Getenv("CI") != "" {
		return 1
	}
	return 0
}

// TestMain brings up the shared Postgres testcontainer + migrations, then
// runs every test in the state package against it via sharedPool. The
// pool is assigned to the package-level `sharedPool` var declared in
// testutil_test.go so per-test helpers (newTestHandlers) can construct
// an OrgDB + Queries against it.
func TestMain(m *testing.M) {
	// Parse test flags so testing.Short() is readable here in TestMain.
	if !flag.Parsed() {
		flag.Parse()
	}
	if testing.Short() {
		// -short skips the whole suite: Docker not needed for short runs.
		os.Exit(m.Run())
	}
	ctx := context.Background()

	pgC, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("state_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		os.Stderr.WriteString("state: testcontainer postgres unavailable: " + err.Error() + "\n")
		os.Exit(containerFailureExitCode())
	}

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		os.Stderr.WriteString("state: connection string: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}

	// Apply migrations via the database/sql shim (Pitfall 7 — pgx-stdlib).
	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		os.Stderr.WriteString("state: sql.Open: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		os.Stderr.WriteString("state: pgmigrate driver: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	// Path from services/api/internal/state/ to repo-root/migrations/:
	//   ..             = services/api/internal/
	//   ../..          = services/api/
	//   ../../..       = services/
	//   ../../../..    = <repo root>/
	m2, err := migrate.NewWithDatabaseInstance("file://../../../../migrations", "postgres", driver)
	if err != nil {
		os.Stderr.WriteString("state: migrate.NewWithDatabaseInstance: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	if err := m2.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		os.Stderr.WriteString("state: migrate.Up: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	_ = sqlDB.Close()

	sharedPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		os.Stderr.WriteString("state: pgxpool.New: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}

	// Run all tests.
	code := m.Run()

	// Teardown: close pool then terminate container.
	sharedPool.Close()
	_ = pgC.Terminate(ctx)
	os.Exit(code)
}
