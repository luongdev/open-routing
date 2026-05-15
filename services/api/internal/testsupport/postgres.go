package testsupport

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDB bundles a started Postgres container and an open pgxpool connected to it.
//
// Container is held so callers can reference it for connection-string
// regeneration in unusual scenarios; Pool is the everyday handle every test
// uses. The two fields are kept side-by-side so a single TestDB struct is
// sufficient to drive both schema-level assertions (Pool.QueryRow against
// information_schema) and full HTTP exercises (handing Pool to an OrgDB and
// mounting server.NewMux against it).
type TestDB struct {
	Pool      *pgxpool.Pool
	Container *postgres.PostgresContainer
}

// StartPostgres spawns a postgres:17 container, applies migrations, returns
// a TestDB ready for use. Calls t.Cleanup to close the pool and terminate the
// container at test end.
//
// Pitfall 4: WithStartupTimeout(60 * time.Second) accommodates cold runners.
// First-time image pulls on CI lanes can take 45-50 seconds on slow nodes; the
// 60-second budget leaves a small headroom without making fast lanes wait.
//
// Pitfall 6: Caller still mints fresh UUIDv7 org_ids per test via FreshOrgID.
// Do NOT add a "default org_id" parameter here — every test must pay the
// fresh-UUID cost so parallel runs do not collide.
//
// Note for TestMain callers: *testing.M does not satisfy testing.TB. If you
// need the same bootstrap in TestMain (which has only *testing.M), inline the
// postgres.Run + ApplyMigrations + pgxpool.New sequence — that path is
// exercised in services/api/test/isolation/main_test.go.
func StartPostgres(t testing.TB) *TestDB {
	t.Helper()
	ctx := context.Background()

	pgC, err := postgres.Run(ctx,
		"postgres:17",
		postgres.WithDatabase("isolation_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "start postgres container")

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "container connection string")

	ApplyMigrations(t, connStr)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err, "open pgxpool")

	t.Cleanup(func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	})

	return &TestDB{Pool: pool, Container: pgC}
}
