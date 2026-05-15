// Command migrate applies all pending golang-migrate migrations against DATABASE_URL.
// Per D-11 the API binary never auto-runs migrations; this is the dedicated runner.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql shim for migrate (Pitfall 7)

	"github.com/luongdev/open-routing/services/api/internal/db"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	os.Exit(run())
}

func run() int {
	databaseURL, ok := os.LookupEnv("DATABASE_URL")
	if !ok || databaseURL == "" {
		slog.Error("migrate: DATABASE_URL is required")
		return 1
	}

	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		slog.Error("migrate: open db", "err", err)
		return 1
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		slog.Error("migrate: ping db", "err", err)
		return 1
	}

	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		slog.Error("migrate: build driver", "err", err)
		return 1
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations", // expects working dir == repo root (Taskfile's task migrate-up runs from there)
		"postgres",
		driver,
	)
	if err != nil {
		slog.Error("migrate: new migrate", "err", err)
		return 1
	}

	// D-05/D-11: emit the bypass event for audit. golang-migrate does not flow
	// through orgDB, but the audit trail is required for forensic clarity.
	ctx := db.WithBypass(context.Background(), "schema_migration")
	slog.WarnContext(ctx, "orgdb bypass",
		"event", "orgdb_bypass",
		"reason", "schema_migration",
		"caller", "cmd/migrate",
	)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		slog.Error("migrate: up", "err", err)
		return 1
	}

	ver, dirty, verr := m.Version()
	if verr != nil && !errors.Is(verr, migrate.ErrNilVersion) {
		slog.Error("migrate: version", "err", verr)
		return 1
	}
	slog.Info("migrations applied", "version", ver, "dirty", dirty)
	return 0
}
