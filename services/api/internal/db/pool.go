// Package db owns the org-scoped database wrapper (OrgDB), the SQL inspector
// (SQLChecker), the bypass marker context plumbing, and the pgxpool factory
// the API binary uses to connect to PostgreSQL. Plan 01-03 introduces this
// package; downstream plans (06 onward) construct OrgDB and hand it to the
// sqlc-generated *Queries.
//
// The package depends only on the orgkey leaf package (Plan 01-1a) and the
// sqlc-generated package for compile-time DBTX interface checks. It never
// imports internal/middleware so the bidirectional cycle (middleware reads
// org_id from ctx; db reads org_id from ctx) is broken via orgkey.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgxpool with retry-with-backoff against the dsn (D-26).
// Retries up to 10 times with exponential backoff capped at 10s; total
// worst-case wait is ~50s which comfortably covers cold docker-compose
// startup before the postgres healthcheck flips to "healthy". Ctx
// cancellation short-circuits the wait loop so a shutdown signal during
// startup aborts immediately rather than blocking on the next backoff.
//
// The last underlying Postgres error is preserved and wrapped in the final
// returned error so debugging reveals the actual failure mode (e.g.,
// "connection refused" during boot vs. "FATAL: database does not exist"
// when the migration step was skipped).
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("orgdb: parse dsn: %w", err)
	}

	var (
		pool    *pgxpool.Pool
		lastErr error
	)
	backoff := time.Second
	for attempt := 0; attempt < 10; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if perr := pool.Ping(ctx); perr == nil {
				return pool, nil
			} else {
				lastErr = perr
				pool.Close()
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		if backoff < 10*time.Second {
			backoff *= 2
			if backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
		}
	}
	return nil, fmt.Errorf("orgdb: pool unreachable after 10 attempts: %w", lastErr)
}
