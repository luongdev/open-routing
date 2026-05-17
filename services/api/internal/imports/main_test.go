// main_test.go — package-level TestMain for the imports suite (Plan
// 05-04 Wave 2).
//
// Brings up ONE Postgres testcontainer + applies all migrations
// (000001 + 000002 + 000003) once per `go test ./internal/imports/...`
// invocation; per-test bring-up reuses sharedPool (declared in
// testutil_test.go) for speed (D-73).
//
// Pattern adapted verbatim from services/api/internal/state/main_test.go.
// Differences:
//   - Database name: imports_test (vs state_test) for log grep clarity.
//   - Migrations path: file://../../../../migrations from
//     services/api/internal/imports/ (4 levels up to repo root) —
//     same depth as the catalog + state suites.
//   - No httptest server / no redis here — those are per-test in
//     testutil_test.go's newTestImports (each test gets a fresh
//     Importer wired against sharedPool).
//
// -short skip + CI exit-code policy mirror the state suite (D-77 +
// Phase 4 Wave 1 review): docker-unavailable in CI = fail loudly;
// docker-unavailable locally = skip gracefully.
package imports

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

// containerFailureExitCode returns 1 when CI env var is non-empty
// (GitHub Actions sets CI=true); 0 otherwise. Distinguishes "Docker
// unavailable locally — skip gracefully" from "Docker unavailable in
// CI — fail loudly". Mirrors state/main_test.go.
func containerFailureExitCode() int {
	if os.Getenv("CI") != "" {
		return 1
	}
	return 0
}

// TestMain brings up the shared Postgres testcontainer + migrations,
// then runs every test in the imports package against it via
// sharedPool (declared in testutil_test.go).
func TestMain(m *testing.M) {
	if !flag.Parsed() {
		flag.Parse()
	}
	if testing.Short() {
		os.Exit(m.Run())
	}
	ctx := context.Background()

	pgC, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("imports_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		os.Stderr.WriteString("imports: testcontainer postgres unavailable: " + err.Error() + "\n")
		os.Exit(containerFailureExitCode())
	}

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		os.Stderr.WriteString("imports: connection string: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}

	// Apply migrations via the database/sql shim (Pitfall 7 — pgx-stdlib).
	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		os.Stderr.WriteString("imports: sql.Open: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		os.Stderr.WriteString("imports: pgmigrate driver: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	// Path from services/api/internal/imports/ to repo-root/migrations/:
	//   ..             = services/api/internal/
	//   ../..          = services/api/
	//   ../../..       = services/
	//   ../../../..    = <repo root>/
	m2, err := migrate.NewWithDatabaseInstance("file://../../../../migrations", "postgres", driver)
	if err != nil {
		os.Stderr.WriteString("imports: migrate.NewWithDatabaseInstance: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	if err := m2.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		os.Stderr.WriteString("imports: migrate.Up: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	_ = sqlDB.Close()

	sharedPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		os.Stderr.WriteString("imports: pgxpool.New: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}

	code := m.Run()

	sharedPool.Close()
	_ = pgC.Terminate(ctx)
	os.Exit(code)
}
