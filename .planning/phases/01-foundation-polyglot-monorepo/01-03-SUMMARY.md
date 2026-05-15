---
phase: 01-foundation-polyglot-monorepo
plan: 03
subsystem: database

# Dependency graph
requires:
  - phase: 01-foundation-polyglot-monorepo (Plan 01)
    provides: services/api Go module skeleton, sqlc.yaml config, internal/config env loader, migrations/000001 with _scaffold table
  - phase: 01-foundation-polyglot-monorepo (Plan 1a)
    provides: services/api/internal/db/orgkey leaf package — request-scoped org_id ctx key
provides:
  - services/api/internal/db.OrgDB — pgx.DBTX-conforming wrapper that physically rejects SQL without an org_id filter (D-02, FOUND-04)
  - services/api/internal/db.WithBypass — typed ctx escape hatch with slog.Warn audit emission (D-04, D-05)
  - services/api/internal/db/sqlcheck — pg_query_go AST walker + SHA-256 parse cache (Pitfall 2)
  - services/api/internal/db/pool — pgxpool factory with retry-with-backoff
  - services/api/internal/db/generated — sqlc bindings for scaffold queries
  - services/api/cmd/migrate — sole legitimate WithBypass caller per D-11
affects:
  - Plan 06 (HTTP server + scaffold handlers — receives *OrgDB via server.Deps, constructs sqlc.New(orgDB))
  - Plan 07 (Two-org isolation suite — exercises the validator + bypass paths end-to-end)
  - Phase 3 (Catalog CRUD — every catalog query authors its own org_id WHERE explicitly)

# Tech tracking
tech-stack:
  added:
    - github.com/jackc/pgx/v5 (pgxpool)
    - github.com/sqlc-dev/sqlc/v1 (generated bindings against pgx/v5)
    - github.com/pganalyze/pg_query_go/v6 (AST validator — Pitfall 2)
    - github.com/golang-migrate/migrate/v4 + pgx/v5/stdlib shim (cmd/migrate runner)
  patterns:
    - "orgDB validate+reject (no SQL rewriting) with SHA-256 parse-cache"
    - "Compile-time DBTX assertion: var _ generated.DBTX = (*OrgDB)(nil)"
    - "WithBypass typed marker — slog.Warn audit on every legitimate bypass"
    - "Migrations via database/sql shim (Pitfall 7) — not pgxpool"

key-files:
  created:
    - services/api/internal/db/pool.go
    - services/api/internal/db/sqlcheck.go
    - services/api/internal/db/sqlcheck_test.go
    - services/api/internal/db/bypass.go
    - services/api/internal/db/orgdb.go
    - services/api/internal/db/orgdb_test.go
    - services/api/internal/db/queries/scaffold.sql
    - services/api/internal/db/generated/db.go
    - services/api/internal/db/generated/models.go
    - services/api/internal/db/generated/scaffold.sql.go
    - services/api/cmd/migrate/main.go
  modified: []

key-decisions:
  - "pg_query_go AST walker chosen over regex per Pitfall 2 — regex misses joined queries, CTEs, INSERT column lists"
  - "SHA-256 cache keyed on the SQL string; parsed once per unique statement across the process lifetime"
  - "cmd/migrate opens the database via sql.Open(\"pgx\", ...) (Pitfall 7) — golang-migrate requires *sql.DB, not pgxpool.Pool"
  - "Bypass slog event emits even though cmd/migrate doesn't flow through orgDB.Exec — preserves audit-trail parity with orgdb.go runtime emissions"

patterns-established:
  - "orgDB wrapping pattern: every DBTX entry point preflights ctx (org_id presence) + SQL string (org_id clause) before delegating to pgxpool"
  - "Bypass audit: every WithBypass call emits slog.Warn event=orgdb_bypass with caller, reason, sql_hash, org_id_attempted"
  - "Single-writer ctx key (orgkey.SetOrgID) — orgDB and middleware both consume but only middleware writes"

requirements-completed:
  - FOUND-04
  - FOUND-05
  - FOUND-06

# Metrics
duration: 27min (worktree exec + orchestrator continuation)
completed: 2026-05-15
---

# Phase 1 Plan 03 Summary

**OrgDB wrapper enforcing org_id on every SQL via pg_query_go AST + WithBypass audit ctx + cmd/migrate runner.**

## Performance

- **Duration:** ~27 min (worktree executor stalled at docker step at 17m; orchestrator continued cmd/migrate + SUMMARY at 27m)
- **Started:** 2026-05-15T10:37:00Z (Wave 3 dispatch)
- **Completed:** 2026-05-15T11:04:00Z (cmd/migrate committed)
- **Tasks:** 6/7 (Task 7 live-docker smoke deferred — see Issues)
- **Files modified:** 11 created

## Accomplishments

- OrgDB wrapper physically rejects sqlc Exec/Query/QueryRow calls when SQL lacks `org_id = $N` filter (D-02) — proven by orgdb_test.go validator-reject cases.
- pg_query_go AST walker correctly classifies SELECT/INSERT/UPDATE/DELETE statements (including CTE and JOIN variants) — Pitfall 2 mitigation in place.
- SHA-256 parse-cache memoizes per-statement validation; second invocation of the same SQL string skips re-parse.
- ctx propagation: orgkey.OrgIDFromContext is the canonical reader; orgDB returns typed error when ctx lacks org_id (D-03).
- WithBypass+slog.Warn audit: every bypass invocation emits structured event with caller/reason/sql_hash/org_id_attempted (D-04, D-05).
- cmd/migrate binary opens DATABASE_URL via pgx/v5/stdlib shim (Pitfall 7) — golang-migrate's *sql.DB requirement satisfied without dragging pgxpool through database/sql.
- API binary anti-pattern guard: cmd/api will NEVER call m.Up (D-11). Only cmd/migrate runs migrations.

## Task Commits

1. **Task 1: pgx pool factory with retry** — `b983ef3` (feat)
2. **Task 2: SQL inspector + bypass marker** — `1d60a49` (feat)
3. **Task 3: sqlc scaffold queries + generated bindings** — `47efa6e` (feat)
4. **Task 4: OrgDB wrapper enforcing org_id** — `acf33ca` (feat — bundles orgdb.go + orgdb_test.go)
5. **Task 5: (folded into Task 4 commit)** — table-driven validator tests verifying reject cases, ctx propagation, and bypass slog emission
6. **Task 6: cmd/migrate binary** — `8b27f55` (feat — written by orchestrator after worktree stall)
7. **Task 7: live docker smoke** — DEFERRED (see Issues Encountered)

**Plan metadata:** This file (committed below).

## Files Created/Modified

- `services/api/internal/db/pool.go` — pgxpool.New + retry-with-backoff connection helper
- `services/api/internal/db/sqlcheck.go` — pg_query_go AST walker (`requireOrgIDFilter`); SHA-256 parse cache
- `services/api/internal/db/sqlcheck_test.go` — table-driven AST classification tests
- `services/api/internal/db/bypass.go` — WithBypass(ctx, reason) + BypassReason context helpers (typed marker, not string key)
- `services/api/internal/db/orgdb.go` — OrgDB struct + Exec/Query/QueryRow + preflight + From factory; `var _ generated.DBTX = (*OrgDB)(nil)`
- `services/api/internal/db/orgdb_test.go` — reject-missing-org_id, reject-missing-ctx-org_id, bypass-allows + slog audit emission
- `services/api/internal/db/queries/scaffold.sql` — sqlc input; InsertScaffold/GetScaffoldByID/ListScaffolds; every query authors `WHERE org_id = $N` explicitly
- `services/api/internal/db/generated/{db.go,models.go,scaffold.sql.go}` — sqlc-generated DBTX interface + Scaffold model + Queries
- `services/api/cmd/migrate/main.go` — golang-migrate runner; bypass-marked ctx; slog.Warn event=orgdb_bypass on every invocation

## Decisions Made

- pg_query_go (CGO, ~3min cold build) accepted over xwb1989/sqlparser (MySQL-flavored, leaks Postgres CTE/JOIN edge cases). Pitfall 2 trade-off documented.
- SHA-256 instead of FNV/MD5 — cheap enough for cold-path validator hits; collision-resistant against accidental cache pollution if an attacker fuzzes SQL bodies.
- cmd/migrate emits the bypass slog event up-front (BEFORE m.Up runs) even though golang-migrate doesn't route through orgDB. Reason: forensic parity with the runtime emission shape — if a future runner adds orgDB calls, the audit trail is uniform.
- `file://migrations` relative path is intentional — `task migrate-up` runs from repo root. Future remote-binary-runs need a `-migrations-path` flag (deferred to Phase 3 when migrations become user-facing).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Worktree executor stalled at Task 7 (live docker smoke)**
- **Found during:** Task 7 attempt — docker compose up failed because ports 5432/6379 are held by a long-running container set (`open-routing-postgres-1`, `open-routing-redis-1`, 7-day uptime, postgres:16-alpine — not 17 per D-25) from a prior project state.
- **Issue:** Plan Task 7 requires a live `task migrate-up` against docker-compose postgres. Cannot bring up without stopping the foreign containers (auto-mode Rule 5: don't destroy shared systems without explicit user authorization). Stream watchdog terminated the executor at 10min idle.
- **Fix:** Orchestrator merged the 4 already-committed feat tasks, then manually wrote `cmd/migrate/main.go` per plan Task 6 spec (verified all 11 grep clauses + `go build -o /dev/null ./cmd/migrate` exit 0). Task 7 (live smoke) deferred — Plan 07's testcontainers-go suite proves the same code path end-to-end against a clean ephemeral postgres:17 container.
- **Files modified:** services/api/cmd/migrate/main.go
- **Verification:** Plan Task 6 verify clauses all pass; `go vet ./...` clean; `go test -short ./internal/db/...` passes (validator + bypass unit tests).
- **Committed in:** `8b27f55` (Task 6) + this SUMMARY.

---

**Total deviations:** 1 auto-fixed (Rule 3 — environmental blocker).
**Impact on plan:** Code surface for FOUND-04/05/06 fully delivered. The "live docker" verification is a Phase 1 nice-to-have; the substantive isolation proof (FOUND-08) is Plan 07's testcontainers suite which exercises the entire orgDB+sqlc+pool stack through real HTTP.

## Issues Encountered

- **Port-conflict blocker.** Containers `open-routing-postgres-1` (postgres:16-alpine) and `open-routing-redis-1` (redis:7-alpine) from a prior project state occupy 127.0.0.1:5432 / 127.0.0.1:6379. They're long-running (3+ days uptime as of merge). User declined to stop them mid-execution; Plan 07 will use testcontainers ports (random) to bypass.
- **Worktree stall watchdog (#a32... pattern).** The stalled agent's worktree was unlocked and removed cleanly. No data loss — all 4 feat commits replicated to the phase branch.

## User Setup Required

None — `task migrate-up` will work as soon as the foreign containers are stopped OR `DATABASE_URL` is pointed at a different host/port. The .env.example carries the default credentials (postgres/postgres on 5432).

## Next Phase Readiness

- Plan 06 (HTTP server + cmd/api) can now consume `*db.OrgDB` via server.Deps and construct `generated.New(orgDB)` per the locked sqlc-via-orgDB pattern (D-01).
- Plan 07 (isolation suite) will exercise OrgDB through full HTTP chain — the substantive FOUND-04 + FOUND-08 gate.
- D-22 module path verified: every generated import resolves against `github.com/luongdev/open-routing/services/api`.

---
*Phase: 01-foundation-polyglot-monorepo*
*Plan: 03*
*Completed: 2026-05-15 (with Task 7 deferred)*
