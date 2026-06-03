// main_test.go — package-level TestMain for the flowrt suite. Brings up ONE
// Postgres testcontainer + applies all migrations once per
// `go test ./internal/flowrt/...`; tests reuse sharedPool. Mirrors the catalog
// suite (internal/catalog/main_test.go) verbatim except the db name and the
// suite's table-cleanup set.
package flowrt

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
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var sharedPool *pgxpool.Pool

func containerFailureExitCode() int {
	if os.Getenv("CI") != "" {
		return 1
	}
	return 0
}

func TestMain(m *testing.M) {
	if !flag.Parsed() {
		flag.Parse()
	}
	if testing.Short() {
		os.Exit(m.Run())
	}
	ctx := context.Background()

	pgC, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("flowrt_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		os.Stderr.WriteString("flowrt: testcontainer postgres unavailable: " + err.Error() + "\n")
		os.Exit(containerFailureExitCode())
	}

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		os.Stderr.WriteString("flowrt: connection string: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}

	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		os.Stderr.WriteString("flowrt: sql.Open: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		os.Stderr.WriteString("flowrt: pgmigrate driver: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	m2, err := migrate.NewWithDatabaseInstance("file://../../../../migrations", "postgres", driver)
	if err != nil {
		os.Stderr.WriteString("flowrt: migrate.NewWithDatabaseInstance: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	if err := m2.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		os.Stderr.WriteString("flowrt: migrate.Up: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	_ = sqlDB.Close()

	sharedPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		os.Stderr.WriteString("flowrt: pgxpool.New: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}

	code := m.Run()
	sharedPool.Close()
	_ = pgC.Terminate(ctx)
	os.Exit(code)
}
