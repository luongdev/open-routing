package testsupport

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql shim for migrate (Pitfall 7)
	"github.com/stretchr/testify/require"
)

// ApplyMigrations opens a database/sql connection to dsn (using the pgx
// stdlib shim — Pitfall 7), constructs a golang-migrate driver, and applies
// all up migrations from the repo-root migrations/ directory.
//
// Pitfall 7: golang-migrate's postgres driver requires database/sql, not
// pgxpool. The `_ "github.com/jackc/pgx/v5/stdlib"` blank import registers a
// "pgx" driver name with the database/sql package so sql.Open("pgx", dsn)
// returns a real connection. Without the blank import the call panics with
// "unknown driver".
//
// The migrations path is computed relative to THIS file's location so the
// helper works regardless of the caller's package directory.
func ApplyMigrations(t testing.TB, dsn string) {
	t.Helper()
	sqlDB, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer sqlDB.Close()

	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	require.NoError(t, err)

	migrationsURL := migrationsFileURL(t)
	m, err := migrate.NewWithDatabaseInstance(migrationsURL, "postgres", driver)
	require.NoError(t, err, "new migrate from %s", migrationsURL)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		require.NoError(t, err, "migrate up")
	}
}

// migrationsFileURL builds a file:// URL pointing at repo-root/migrations.
// This file lives at services/api/internal/testsupport/migrate.go; the
// migrations directory is therefore 4 levels up: ../../../../migrations.
//
// Per-segment unwrap from services/api/internal/testsupport/migrate.go:
//
//	..             = services/api/internal/
//	../..          = services/api/
//	../../..       = services/
//	../../../..    = <repo root>/
//	../../../../migrations = <repo root>/migrations/
//
// runtime.Caller(0) returns the absolute path of this source file at compile
// time; resolving relative to that is independent of the test binary's
// invocation cwd, so the helper works whether the caller runs `go test`
// from services/api/, from a sub-package, or from a worktree.
func migrationsFileURL(t testing.TB) string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve testsupport file path")
	}
	migrations := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "migrations")
	return fmt.Sprintf("file://%s", migrations)
}
