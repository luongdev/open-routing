// Package isolation_test contains the FOUND-08 two-org isolation proof
// (VALIDATION.md §"Two-Org Isolation Specification"). The package shares one
// Postgres testcontainer + one httptest server across every test for speed;
// each test mints its own UUIDv7 org_ids (Pitfall 6).
//
// The bootstrap lives in TestMain because *testing.M does not satisfy
// testing.TB — so we cannot call testsupport.StartPostgres / ApplyMigrations
// directly here. Instead we inline the same sequence: postgres.Run + sql.Open
// shim + migrate.Up + pgxpool.New. Per-test bootstrap helpers in the
// internal/testsupport package satisfy TB and are used in unit-level suites
// (internal/scaffold/handler_test.go does the same dance).
//
// Test wiring uses server.NewMux directly (D-07): every test request flows
// through the real chi router + RequestID + OrgContext + scaffold routes —
// the same code path production uses. There is no test-only fork of the mux.
package isolation_test

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql shim for migrate (Pitfall 7)
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/yaml.v3"

	"github.com/jonboulle/clockwork"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/catalog"
	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/flowrt"
	"github.com/luongdev/open-routing/services/api/internal/imports"
	"github.com/luongdev/open-routing/services/api/internal/server"
	"github.com/luongdev/open-routing/services/api/internal/state"
)

// Package-level shared state. Every test reads from these without taking a
// lock because TestMain finishes initialization before any t.Parallel test
// runs (the m.Run() barrier guarantees this).
var (
	sharedPool  *pgxpool.Pool
	sharedSrv   *httptest.Server
	sharedRedis *redis.Client // may be nil if no REDIS_URL env; /readyz then degrades but bypass paths still work
	sharedPgC   *postgres.PostgresContainer
)

// containerFailureExitCode returns 1 when the CI env var is non-empty (GitHub
// Actions always sets CI=true). Used by TestMain to distinguish "Docker
// unavailable locally — skip gracefully" from "Docker unavailable in CI — fail
// loudly".
func containerFailureExitCode() int {
	if os.Getenv("CI") != "" {
		return 1
	}
	return 0
}

// TestMain provisions a Postgres testcontainer, applies migrations, opens a
// pgxpool, builds the production chi mux, and stands up an httptest server
// against it. m.Run executes every test in the package; teardown closes the
// server, drops the redis client, closes the pool, and terminates the
// container.
//
// -short skip: the isolation suite requires Docker. Unit-level CI lanes pass
// -short and skip the entire bring-up. The docker-available lane omits -short
// and runs the full FOUND-08 proof.
func TestMain(m *testing.M) {
	// Parse test flags so testing.Short() is readable here in TestMain. Go's
	// runtime defers flag parsing to m.Run; we force it now so the -short check
	// is safe before bring-up.
	if !flag.Parsed() {
		flag.Parse()
	}
	if testing.Short() {
		os.Exit(m.Run())
	}
	ctx := context.Background()

	// (1) Postgres testcontainer + migrations.
	pgC, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("isolation_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		// Docker unavailable / image-pull denied: best-effort log and exit 0
		// so the test binary does not fail when running in a docker-less
		// environment. The docker-available CI lane fails fast on the actual
		// test cases if Docker silently disappeared.
		os.Stderr.WriteString("isolation: testcontainer postgres unavailable: " + err.Error() + "\n")
		os.Exit(containerFailureExitCode())
	}
	sharedPgC = pgC

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		os.Stderr.WriteString("isolation: connection string: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}

	// (2) Apply migrations via database/sql shim (Pitfall 7).
	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		os.Stderr.WriteString("isolation: sql.Open: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		os.Stderr.WriteString("isolation: pgmigrate driver: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	// Path from services/api/test/isolation/ to repo-root/migrations/:
	// ..             = services/api/test/
	// ../..          = services/api/
	// ../../..       = services/
	// ../../../..    = <repo root>/
	m2, err := migrate.NewWithDatabaseInstance("file://../../../../migrations", "postgres", driver)
	if err != nil {
		os.Stderr.WriteString("isolation: migrate.NewWithDatabaseInstance: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	if err := m2.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		os.Stderr.WriteString("isolation: migrate.Up: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	_ = sqlDB.Close()

	sharedPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		os.Stderr.WriteString("isolation: pgxpool.New: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}

	// (3) Redis: prefer REDIS_URL env when set; otherwise spin up miniredis
	// so catalog.Handlers always has a working Cache dep (CAT-11). miniredis.Run
	// (not RunT) because TestMain has no testing.TB; the in-process server is
	// process-lifetime — no Terminate needed.
	if dsn := os.Getenv("REDIS_URL"); dsn != "" {
		if opts, perr := redis.ParseURL(dsn); perr == nil {
			sharedRedis = redis.NewClient(opts)
		}
	}
	if sharedRedis == nil {
		mr, mrErr := miniredis.Run()
		if mrErr != nil {
			os.Stderr.WriteString("isolation: miniredis run: " + mrErr.Error() + "\n")
			sharedPool.Close()
			_ = pgC.Terminate(ctx)
			os.Exit(containerFailureExitCode())
		}
		sharedRedis = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	}

	// (4) Build the production mux. ValidationMode=panic mirrors the
	// dev/test contract from D-02 — a missing org_id filter at the SQL layer
	// surfaces immediately rather than being masked.
	cfg := &config.Config{
		DatabaseURL:        "n/a",
		RedisURL:           "n/a",
		OTelExporter:       "stdout",
		ListenAddr:         ":0",
		ValidationMode:     "panic",
		CORSAllowedOrigins: []string{"https://example.com"},
	}
	orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)

	swagger, _ := api.GetSpec()
	specBytes, _ := yaml.Marshal(swagger)

	// Phase 4: ApiHandlers composite satisfies the full StrictServerInterface.
	// catalog.Handlers alone no longer satisfies it after Phase 4 removed the
	// state stubs (D-89). state.Server is wired with the same OrgDB + Cache
	// so GetAgentStatus / PatchAgentStatus are live for isolation probes.
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	catalogCache := cache.New(sharedRedis, logger)
	catalogHandlers := catalog.New(catalog.Deps{
		OrgDB:  orgDB,
		Pool:   sharedPool,
		Cache:  catalogCache,
		Logger: logger,
	})
	stateServer := state.New(state.Deps{
		OrgDB:  orgDB,
		Cache:  catalogCache,
		Logger: logger,
	}, state.WithClock(clockwork.NewRealClock()))
	// Start runs the synchronous startup sweep (D-95) so any stuck WrapUp
	// rows from a prior test run are cleared before tests execute.
	if err := stateServer.Start(ctx); err != nil {
		os.Stderr.WriteString("isolation: stateServer.Start: " + err.Error() + "\n")
		sharedPool.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	// Phase 5: wire imports.Importer into the same composite so the
	// isolation suite exercises BulkImportCatalog / GetImportJob via the
	// same chi mux production uses (Plan 05-06 third embed; F3).
	importer := imports.New(imports.Deps{
		OrgDB:  orgDB,
		Cache:  catalogCache,
		Logger: logger,
	}, imports.WithClock(clockwork.NewRealClock()))
	if err := importer.Start(ctx); err != nil {
		os.Stderr.WriteString("isolation: importer.Start: " + err.Error() + "\n")
		stateServer.Stop()
		sharedPool.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	type apiHandlers struct {
		*catalog.Handlers
		*state.Server
		*imports.Importer
		*flowrt.Endpoints
	}
	handlers := &apiHandlers{
		Handlers:  catalogHandlers,
		Server:    stateServer,
		Importer:  importer,
		Endpoints: flowrt.New(flowrt.Deps{OrgDB: orgDB, Cache: catalogCache, Logger: logger}),
	}
	mux := server.NewMux(&server.Deps{
		Pool:           sharedPool,
		Redis:          sharedRedis,
		OrgDB:          orgDB,
		Config:         cfg,
		StrictHandlers: handlers,
		SpecBytes:      specBytes,
	})

	// (5) httptest server. Use the production mux directly — no otelhttp wrap
	// is needed for FOUND-08 (we are not asserting span behaviour here).
	sharedSrv = httptest.NewServer(mux)

	// (6) Run tests.
	code := m.Run()

	// (7) Cleanup. Order matches production LIFO:
	//   HTTP → importer sweep → state sweeper → Redis → pool.
	sharedSrv.Close()
	importer.Stop()
	stateServer.Stop()
	if sharedRedis != nil {
		_ = sharedRedis.Close()
	}
	sharedPool.Close()
	_ = pgC.Terminate(ctx)
	os.Exit(code)
}
