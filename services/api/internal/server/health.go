// Package server wires the chi router that fronts the open-routing API.
//
// health.go owns the three operator-facing bypass routes (D-17, D-21):
//
//   - /healthz   liveness   always-200 + {"status":"alive"}
//   - /readyz    readiness  pings pgxpool + redis + reads schema_migrations
//   - /metrics   metrics    placeholder 200 + empty body (Open Question #4)
//
// All three are mounted at the chi root in NewMux BEFORE the /v1 sub-router so
// they are reachable without an X-Org-Id header (D-21 bypass list).
//
// /readyz intentionally uses the raw *pgxpool.Pool (not OrgDB) — readyz has
// no org_id context, and the schema_migrations table is operator metadata
// shared across orgs. This is the only legitimate raw-pool surface in the
// running API binary; every catalog handler routes through OrgDB.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// readyzBody is the canonical /readyz JSON shape (D-17).
//
// Status is "ok" when every check passed and "degraded" when at least one
// check failed (or schema_migrations.dirty == true). Checks is a heterogeneous
// map so per-check failure detail (e.g. {"migrations":{"error":"..."}}) can
// land alongside the simple "ok" / "fail" markers without inventing a new
// response shape per kind of check.
type readyzBody struct {
	Status string         `json:"status"`
	Checks map[string]any `json:"checks"`
}

// LiveHandler returns 200 + {"status":"alive"} unconditionally. Liveness
// probes (D-17 bypass via D-21) MUST NOT touch the database or Redis; their
// purpose is to answer the load balancer's "is this process still running?"
// question. Deep checks (db + redis + schema) live in ReadyzHandler.
//
// The body literal is written via w.Write (not json.Encoder) because the
// payload is a fixed string; this avoids the encoder allocation on every
// LB probe.
func LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"alive"}`))
	}
}

// ReadyzHandler pings pgxpool, pings Redis, and reads the latest
// schema_migrations row. Returns 200 + {"status":"ok","checks":{...}} when
// every check passes; 503 + {"status":"degraded","checks":{...}} when any
// check fails or schema_migrations.dirty == true (D-17, Research §Example 6).
//
// Timeout: each underlying call shares a 2-second context budget (T-1-DOS-READYZ
// in the threat model). A wedged Postgres connection cannot hang the handler
// beyond the 2s ceiling regardless of pool wait time.
//
// Information disclosure note: the migration-error case puts the raw
// pgconn error string into the JSON body. Phase 1 accepts this disposition
// (T-1-INFO-503 mitigate-by-scope) — /readyz is an operator-facing endpoint
// meant to live behind an internal load balancer, never on the public internet.
// When v1 exposes the API past an internal-only LB, gate /readyz with a
// CHECK-INTERNAL header / IP allowlist.
func ReadyzHandler(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		body := readyzBody{Status: "ok", Checks: map[string]any{}}
		statusCode := http.StatusOK

		// (1) Postgres ping.
		if err := pool.Ping(ctx); err != nil {
			body.Checks["db"] = "fail"
			body.Status = "degraded"
			statusCode = http.StatusServiceUnavailable
		} else {
			body.Checks["db"] = "ok"
		}

		// (2) Redis ping.
		if err := rdb.Ping(ctx).Err(); err != nil {
			body.Checks["redis"] = "fail"
			body.Status = "degraded"
			statusCode = http.StatusServiceUnavailable
		} else {
			body.Checks["redis"] = "ok"
		}

		// (3) Latest schema_migrations row.
		// The table is created automatically by golang-migrate's postgres
		// driver and follows the shape (version BIGINT, dirty BOOLEAN). Reading
		// version + dirty lets us flag both "no migrations applied yet" (no
		// row) and "last migration crashed" (dirty=true) as degraded.
		var ver int64
		var dirty bool
		if err := pool.QueryRow(ctx,
			`SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1`,
		).Scan(&ver, &dirty); err != nil {
			body.Checks["migrations"] = map[string]any{"error": err.Error()}
			body.Status = "degraded"
			statusCode = http.StatusServiceUnavailable
		} else {
			body.Checks["migrations"] = map[string]any{"version": ver, "dirty": dirty}
			if dirty {
				body.Status = "degraded"
				statusCode = http.StatusServiceUnavailable
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if err := json.NewEncoder(w).Encode(body); err != nil {
			// We have already sent headers — there is no recovery path. Log
			// the failure for operators and return; the client receives a
			// partial / empty body, which is acceptable for a /readyz scrape.
			slog.ErrorContext(r.Context(), "readyz: encode response", "err", err)
		}
	}
}

// MetricsHandler is the Phase 1 placeholder for /metrics (D-17 + Research
// Open Question #4). It returns 200 with an empty body so Prometheus scrapers
// stop fast-failing on the route. Phase 4+ may replace this with a real
// otelhttp / Prometheus exporter registration; the route signature is stable.
func MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(""))
	}
}
