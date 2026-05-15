# Phase 1: Foundation & Polyglot Monorepo - Research

**Researched:** 2026-05-15
**Domain:** Polyglot Go + pnpm monorepo bootstrap; org-isolated REST API skeleton (chi + pgx + sqlc) with golang-migrate, OTel, slog, Docker Compose dev infra, GitHub Actions CI, testcontainers-go isolation suite
**Confidence:** HIGH

## Summary

Phase 1 builds the foundation every later phase imports: a Go module at `services/api/`, a pnpm workspace stub at `web/`, one migration creating a throwaway `_scaffold` table, an `orgDB` wrapper that enforces `org_id` on every SQL statement, a chi HTTP server with `X-Org-Id` middleware, OTel + slog wired to inject `trace_id`/`span_id` into every log line, Docker Compose for Postgres 17 + Redis, a Taskfile orchestrator, and a GitHub Actions matrix that runs the two-org isolation test with testcontainers-go. CONTEXT.md locks 30 decisions (D-01 through D-31); this research operationalizes them.

Three implementation areas dominate the risk surface and demand precise patterns: (1) the `orgDB` wrapper around `*pgxpool.Pool` that implements the sqlc-generated `DBTX` interface and inspects SQL with `pg_query_go/v6` for an `org_id` filter, panicking in dev/test and erroring in prod; (2) the testcontainers-go integration suite that boots one ephemeral Postgres per test package via `TestMain`, runs `migrate.New` against it programmatically, drives `httptest.NewServer(chi.Mux)` with two distinct `X-Org-Id` UUIDv7 values, and asserts zero cross-org row visibility; (3) the OTel + slog wiring where `otelhttp.NewHandler` wraps the chi root, the org middleware adds `org_id` as a span attribute, and a custom `slog.Handler` reads the active span from `context.Context` to inject `trace_id`/`span_id` into every log record.

**Primary recommendation:** Implement the `orgDB` wrapper as a single Go struct implementing the sqlc DBTX interface (`Exec`/`Query`/`QueryRow` with `pgx.Rows`/`pgx.Row` return types), using `pg_query_go/v6` for SQL parse-tree introspection cached by SHA-256 once per unique SQL string. Test it with testcontainers-go via `TestMain` package-level setup. Use stdlib `slog.NewJSONHandler` wrapped in a thin custom handler that injects OTel span context; do not add `otelslog` (it replaces the handler rather than wrapping it and would interfere with the dev stdout exporter strategy). Use `pgxpool.Pool` (not raw `pgx.Conn`) as the underlying connection holder; pin Go 1.23+ (`go 1.23` in go.mod).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**orgDB Pattern + sqlc Integration**
- **D-01:** `orgDB` is a wrapper around `*pgx.Pool` implementing the `pgx.DBTX` interface; sqlc-generated `*Queries` receives an `orgDB` instance (not the raw pool). Handlers physically cannot construct sqlc `Queries` against the raw pool — only against `orgDB`.
- **D-02:** Enforcement model is **validate + reject**. On every `Exec/Query/QueryRow` call, the wrapper inspects the SQL string (parsed once per unique string, cached by SHA-256 hash) and rejects any statement that lacks an `org_id = $N` clause in WHERE / INSERT column list / UPDATE / DELETE. Validation failures panic in dev/test and return an error in prod. No runtime SQL rewriting — sqlc query files MUST author the filter explicitly.
- **D-03:** The wrapper also asserts that `ctx` carries an `org_id` (UUIDv7) before delegating. Missing org_id → typed error.
- **D-04:** **Cross-org escape hatch:** `orgDB.WithBypass(ctx, reason string) context.Context` returns a context with a typed marker. The validator detects the marker and skips the org_id check. In Phase 1 the **only** legitimate bypass caller is the migrations runner (`cmd/migrate`); every other surface must use the strict path.
- **D-05:** Bypass events emit `slog.Warn` with structured fields `event=orgdb_bypass`, `caller`, `reason`, `sql_hash`, `org_id_attempted`. No DB-backed audit table in v0.1 — when v1 audit infrastructure lands, the same slog event becomes the source for a structured audit row with zero code change.

**Two-Org Isolation Proof Harness**
- **D-06:** Postgres provisioning for integration tests uses **testcontainers-go** (ephemeral container per suite). Local `go test ./...` and GitHub Actions CI follow the same code path — no environment divergence. The migration set runs against the container on startup via golang-migrate.
- **D-07:** The FOUND-08 isolation test exercises the **full HTTP chain** via `httptest.NewServer` (or direct `chi.Mux.ServeHTTP`). Real HTTP requests carry `X-Org-Id: <uuidv7-A>` / `<uuidv7-B>`. The test seeds two orgs with identical `external_id` values in the `_scaffold` table, drives create/read/list through real chi routes, and asserts each org sees only its own rows. This single suite covers FOUND-02 (schema), FOUND-03 (header extraction), FOUND-04 (orgDB enforcement), FOUND-05 (ctx propagation), FOUND-06 (UNIQUE constraint), and FOUND-08 (zero leakage).

**Monorepo Orchestration + Migrations**
- **D-08:** Top-level orchestrator is **Taskfile** (`Taskfile.yml` at repo root). Targets: `task test`, `task lint`, `task gen`, `task migrate-up`, `task migrate-down`, `task migrate-create NAME=...`, `task dev`, `task build`, `task ci`. Frontend tasks delegate to `pnpm -F <pkg> ...`.
- **D-09:** **Turborepo lives inside `web/`** (`web/turbo.json`) and owns caching/parallelism for the pnpm workspace only. Go side never touches Turborepo. Two tools, each owning its native ecosystem.
- **D-10:** Migrations live at **`/migrations/`** at repo root (matches FOUND-01). Filenames follow golang-migrate convention: `NNNNNN_name.up.sql` / `NNNNNN_name.down.sql`. Phase 1 ships a single migration creating the `_scaffold` table.
- **D-11:** Migrations are invoked via `task migrate-up` (which calls the `migrate` CLI binary). **API never auto-runs migrations** on startup — prevents multi-replica race in v1, makes schema drift visible. Migration runner is a separate Go `cmd/migrate` binary (or invokes the `migrate` CLI directly via Taskfile) and is the **only** legitimate user of `orgDB.WithBypass`.

**Frontend (pnpm) Stub Scope**
- **D-12:** Phase 1 ships minimal pnpm scaffold (workspace.yaml, root package.json, tsconfig.base.json, apps/admin, apps/embed, packages/ui with empty index.ts stubs, turbo.json with typecheck/lint pipelines).
- **D-13:** No Vite config, no Lit components, no Shoelace integration in Phase 1. CI runs `pnpm typecheck` and `pnpm lint`.

**OTel + Scaffold Surface**
- **D-14:** OTel SDK initializes **before** chi router setup. Exporter target via `OTEL_EXPORTER` env: dev/CI → stdout; prod → OTLP HTTP (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`).
- **D-15:** No Jaeger / no OTel collector in docker-compose.
- **D-16:** Auto-instrumentation via `otelhttp.NewHandler` wrapping the chi mux at the root. Org-context middleware adds `org_id` as a span attribute.
- **D-17:** Phase 1 routes: `GET /healthz`, `GET /readyz`, `GET /metrics`, `POST/GET/GET /v1/orgs/{org_id}/_scaffold[/{id}]`.
- **D-18:** `_scaffold` table: `(id UUID PK, org_id UUID NOT NULL, external_id TEXT NOT NULL, name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(org_id, external_id))`.

**org_id Validation + ID Convention**
- **D-19:** **All IDs UUIDv7 or higher** (use `uuid.NewV7()` from `github.com/google/uuid` v1.6+). UUIDv4 rejected.
- **D-20:** `X-Org-Id` header parsed with `uuid.Parse`, then `Version() >= 7` checked; otherwise HTTP 400 `{"error":"invalid_org_id","reason":"uuidv7_required"}`. Stored in ctx as `uuid.UUID`.
- **D-21:** Middleware bypass list: `/healthz`, `/readyz`, `/metrics`. `/openapi.yaml` not served in Phase 1.

**Go Module + Repo Layout**
- **D-22:** Single Go module at **`services/api/go.mod`**. Module path: `github.com/luongdev/open-routing/services/api`. No root `go.mod`, no `go.work`.
- **D-23:** Internal package layout: `internal/server/`, `internal/middleware/`, `internal/db/`, `internal/db/queries/`, `internal/db/generated/`, `internal/scaffold/`, `internal/telemetry/`, `internal/config/`, `cmd/api/`, `cmd/migrate/`.

**Dev Experience**
- **D-24:** `task dev`: docker compose up postgres+redis, wait healthchecks, foreground `air` for Go hot-reload.
- **D-25:** docker-compose contains `postgres:17`, `redis:7-alpine` only. No `api` service.
- **D-26:** Postgres + Redis ship with native docker-compose healthchecks (`pg_isready`, `redis-cli ping`).

**Logging + Request Correlation**
- **D-27:** slog uses **JSON format everywhere**. No tint / human-readable handler.
- **D-28:** RequestID middleware sits second in the chain (after Recover). Generates UUIDv7, injects to ctx, writes back as `X-Request-Id`.
- **D-29:** Custom `slog.Handler` wrapper reads active OTel span from ctx and injects `trace_id`/`span_id`.

**CI**
- **D-30:** `.github/workflows/ci.yml` runs on every push and PR. Jobs: `go-vet`, `go-lint` (golangci-lint), `go-test` (`-race`), `web-typecheck`, `web-lint`, `migration-drift`.
- **D-31:** All jobs required for merge. Concurrency cancels in-flight runs for the same branch. No coverage threshold in Phase 1.

### Claude's Discretion

- Specific `golangci-lint` linter set — start with default + `errcheck`, `govet`, `staticcheck`, `gosec`, `gocritic`.
- Exact `air.toml` configuration.
- ESLint config flavor (`@typescript-eslint/recommended-strict` baseline).
- Specific Postgres + Redis image tags and resource limits.
- Taskfile target dependency graph beyond named targets.
- Whether to ship `.editorconfig` / `.gitattributes` (recommend yes).
- `slog.Handler` library choice (stdlib `slog.NewJSONHandler` is sufficient; only the trace_id wrapper is custom).

### Deferred Ideas (OUT OF SCOPE)

- Real audit log table — bypass events go to slog only.
- Jaeger / OTel collector in docker-compose.
- Distributed lock for migrations.
- `/openapi.yaml` served from API (Phase 2 decides).
- Coverage threshold gate.
- Prometheus scrape config.
- `go.work` multi-module workspace.
- Vite dev servers in docker-compose.
- `.editorconfig` / `.gitattributes` (may include or defer).
- Web Worker for CSV preview.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FOUND-01 | Polyglot monorepo with `services/api/`, `web/`, `openapi/`, `migrations/` at root | Repo Layout (§Architecture Patterns); Taskfile orchestration; pnpm workspace stub (D-12) |
| FOUND-02 | PostgreSQL 17 schema with `org_id UUID NOT NULL` on every org-scoped table | `_scaffold` migration (D-18); golang-migrate filename convention; pg_isready healthcheck |
| FOUND-03 | API extracts `org_id` exclusively from `X-Org-Id` header; missing/malformed → 400 | OrgContext middleware (Code Examples §1); UUIDv7 validation pattern (D-20); chi middleware ordering |
| FOUND-04 | All DB access through `orgDB` wrapper; handlers receive only `orgDB` instances | orgDB wrapper implementing sqlc DBTX (Code Examples §2); pg_query_go SQL inspection; SHA-256 cache (D-02) |
| FOUND-05 | Request `org_id` propagates via `context.Context` from middleware → handler → service → repository | `context.Context` propagation pattern (Pattern 1); no globals; type-safe `uuid.UUID` in ctx |
| FOUND-06 | Every catalog table enforces `UNIQUE (org_id, external_id)` | `_scaffold` UNIQUE constraint (D-18); composite uniqueness pattern |
| FOUND-07 | OTel Go SDK initializes before chi router setup; every slog log line and OTel span carries `org_id` | OTel init order (Pattern 3); custom slog handler wraps stdlib JSON handler (Code Examples §4); span attribute injection in OrgContext middleware |
| FOUND-08 | Integration suite seeds two orgs with overlapping `external_id`s; tests every CRUD endpoint; zero cross-org leakage; runs against real Postgres 17 container | testcontainers-go TestMain pattern (Pattern 4); two-org isolation harness (Validation Architecture); httptest.NewServer driving chi mux |
| FOUND-09 | GitHub Actions runs go vet, go test, golangci-lint, frontend typecheck/lint/unit, two-org suite on every PR; merge blocked on failure | CI workflow matrix (Pattern 5); actions/setup-go v5 cache; concurrency cancel-in-progress |
| FOUND-10 | Docker Compose brings up Postgres 17, Redis, Go API, Vite dev servers with one command | docker-compose.yml (Pattern 6); pg_isready / redis-cli ping healthchecks; D-25 limits compose to postgres+redis only (api runs native via air) |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `X-Org-Id` header parsing | API / Backend (chi middleware) | — | Per FOUND-03, header extraction is server-only; never the browser's job |
| `org_id` propagation through call chain | API / Backend (`context.Context`) | — | Per FOUND-05, lives in Go context, never serialized client-side |
| SQL inspection / `org_id = $N` enforcement | Database / Storage (orgDB wrapper) | — | Per D-02, app-layer wrapper around pgxpool.Pool; never DB-side RLS in v0.1 |
| OTel SDK init + span lifecycle | API / Backend (cmd/api startup) | — | Per FOUND-07, initializes before chi mux is constructed |
| Structured log emission with trace correlation | API / Backend (slog handler) | — | Per D-29, slog handler wraps stdlib JSON; reads OTel span from ctx |
| Migration application | API / Backend (cmd/migrate binary) | — | Per D-11, separate binary; only legitimate `orgDB.WithBypass` caller |
| Health probes (`/healthz`, `/readyz`) | API / Backend (chi handlers, bypass org middleware) | — | Per D-17 / D-21, infrastructure tier checks DB / Redis / migrations version |
| Frontend scaffold stubs | Browser / Client (pnpm packages) | Frontend Server (Vite, Phase 6) | Per D-12 / D-13, no runtime in Phase 1 — only typecheck/lint target |
| Test infrastructure (testcontainers) | API / Backend (Go test packages) | — | Per D-06, Go test code spawns Postgres containers; no UI side |

## Standard Stack

### Core (Go side — `services/api/go.mod`)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.2.3 | HTTP router | Chi v5 is the idiomatic Go HTTP router; 100% net/http compatible; minimal allocs; locks for v0.1 stack per PROJECT.md [VERIFIED: pkg.go.dev/github.com/go-chi/chi/v5] |
| `github.com/jackc/pgx/v5` | v5.9.2 | Postgres driver + pool | Native Postgres driver (no `database/sql` overhead); the `pgxpool.Pool.Exec/Query/QueryRow` methods are exactly the sqlc-generated DBTX interface signature [VERIFIED: pkg.go.dev/github.com/jackc/pgx/v5/pgxpool] |
| `github.com/sqlc-dev/sqlc` (CLI) | v1.31.1 | Type-safe SQL → Go codegen | Locked stack choice; generates `*Queries` struct accepting any `DBTX`-conforming type [VERIFIED: docs.sqlc.dev/en/latest/] |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 | Schema migrations | Both a CLI (for Taskfile) and a programmatic Go library (for `cmd/migrate`); supports `file://` source + `postgres://` database driver [VERIFIED: github.com/golang-migrate/migrate] |
| `github.com/google/uuid` | v1.6.0 | UUID v4/v7 with `Version()` checker | Stdlib-quality; `uuid.NewV7()` exposed since v1.6.0; `Parse()` + `Version()` give the validation pipeline D-20 requires [VERIFIED: pkg.go.dev/github.com/google/uuid] |
| `go.opentelemetry.io/otel` (SDK + API) | v1.x (current) | OTel core | Required for FOUND-07 [VERIFIED: opentelemetry.io/docs/languages/go] |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | v0.x (latest) | HTTP auto-instrumentation | `otelhttp.NewHandler(mux, "open-routing-api")` wraps chi at root [VERIFIED: pkg.go.dev/.../otelhttp] |
| `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` | v1.x | Dev/CI span exporter | Spans flush to logs; zero infra (D-14) [VERIFIED: opentelemetry.io/docs/languages/go/exporters/] |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | v1.x | Prod span exporter | OTLP HTTP/protobuf; reads `OTEL_EXPORTER_OTLP_ENDPOINT` env [VERIFIED: pkg.go.dev/.../otlptracehttp] |
| `github.com/pganalyze/pg_query_go/v6` | v6 | SQL parse tree introspection | Built on the real PostgreSQL parser; the only library that can reliably detect `WHERE org_id = $N` / `INSERT ... (org_id, ...)` / `UPDATE ... SET ...` / `DELETE ...` patterns across joins, subqueries, CTEs. **CGO required, ~3 min initial compile** [VERIFIED: github.com/pganalyze/pg_query_go] |
| `log/slog` | stdlib (Go 1.21+) | Structured logging | Used directly via `slog.NewJSONHandler` for D-27 [VERIFIED: pkg.go.dev/log/slog] |

### Supporting (Go side — test + dev)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/testcontainers/testcontainers-go` | v0.40+ | Docker-driven test infra | TestMain setup for integration suite (FOUND-08) [VERIFIED: golang.testcontainers.org] |
| `github.com/testcontainers/testcontainers-go/modules/postgres` | v0.40+ | Postgres-specific container helpers | `postgres.Run(ctx, "postgres:17", postgres.WithDatabase(...), postgres.WithUsername(...), postgres.WithPassword(...))` returns `*PostgresContainer` with `.ConnectionString(ctx)` [VERIFIED: golang.testcontainers.org/modules/postgres] |
| `github.com/stretchr/testify` | v1.10+ | Test assertions | Used by every Go test in the ecosystem; `assert.Equal`, `require.NoError` patterns [VERIFIED: pkg.go.dev/github.com/stretchr/testify] |
| `github.com/redis/go-redis/v9` | v9.x | Redis client for `/readyz` ping | Official client; v9 line current [VERIFIED: pkg.go.dev/github.com/redis/go-redis/v9] |
| `github.com/air-verse/air` | v1.65+ | Go hot-reload for `task dev` | TOML-configured; **requires Go 1.25+ since v1.65** — pin v1.61.x or earlier if using Go 1.23 [CITED: github.com/air-verse/air] |
| `github.com/golangci/golangci-lint` (CLI) | v1.63+ | Go linter aggregator | golangci-lint v1.x; v2 line exists but v1.63 is sufficient and stable [VERIFIED: golangci-lint version 1.63.4 in env] |

### Supporting (pnpm side — `web/package.json` + per-package)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `typescript` | v5.x | TS compiler | `pnpm typecheck` runs `tsc --noEmit` per package [VERIFIED: npm registry] |
| `turbo` | v2.x | Turborepo task runner inside `web/` | D-09 — `web/turbo.json` only [VERIFIED: npm registry] |
| `eslint` | v9.x | JS/TS linter | `pnpm lint` per package [VERIFIED: npm registry] |
| `@typescript-eslint/parser` + `@typescript-eslint/eslint-plugin` | v8.x | TS-aware ESLint | Pair with eslint; flat config recommended [VERIFIED: npm registry] |
| `prettier` | v3.x | Code formatter | Optional but recommended; runs via `pnpm lint` or pre-commit [VERIFIED: npm registry] |

### Tools (CLI binaries, not Go module deps)

| Tool | Version | Install | Purpose |
|------|---------|---------|---------|
| Taskfile (`task`) | v3.50+ | `brew install go-task` or `go install github.com/go-task/task/v3/cmd/task@latest` | Top-level orchestrator (D-08) [VERIFIED: taskfile.dev] |
| `migrate` CLI | v4.19.1 | `brew install golang-migrate` or pinned binary in CI | Invoked by `task migrate-up` (D-11) [VERIFIED: github.com/golang-migrate/migrate] |
| `sqlc` CLI | v1.31.1 | `brew install sqlc` or `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` | `task gen` target [VERIFIED: docs.sqlc.dev] |
| `pnpm` | v10.x | `corepack enable pnpm` | Workspace manager [VERIFIED: pnpm v10.33.0 in env] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `pg_query_go/v6` | Hand-rolled regex / `xwb1989/sqlparser` | Regex misses joined queries / CTEs and produces false negatives that breach isolation. `xwb1989/sqlparser` is MySQL-flavored; doesn't handle Postgres-specific syntax. CGO build time (~3 min initial) is acceptable; subsequent builds cache compiled C [ASSUMED on regex inadequacy — mitigated by CGO trade-off being well-understood] |
| `otelchi` | `otelhttp.NewHandler` (chosen) | otelchi adds chi-specific span attributes (route patterns, etc.) but otelhttp covers FOUND-07 with smaller surface; can swap later if route-pattern spans are needed |
| `slog-otel` / `otelslog` | Custom wrapper around `slog.NewJSONHandler` (chosen) | `go.opentelemetry.io/contrib/bridges/otelslog` **replaces** the slog handler with an OTel log exporter rather than wrapping JSONHandler — incompatible with D-27 "JSON to stdout in dev." Custom wrapper is ~30 LoC and gives full control |
| Sequential migration versioning (`-seq`) | Timestamp versioning (chosen) | golang-migrate supports both. Timestamps prevent merge conflicts in multi-developer scenarios. Phase 1 has one migration so either works; lock the convention now |
| Testcontainers + reuse | Fresh container per package (chosen) | Reuse via `WithReuseByName` is faster but introduces state-leak risk between suites. Per-package containers + parallel `t.Parallel()` are the safer default; revisit if test wall-clock becomes painful |
| GitHub Actions service containers (`services:`) for Postgres | testcontainers-go (chosen) | Service containers are faster but D-06 mandates same code path local + CI. Service containers also can't run migrations programmatically in the test harness without extra coordination steps |

**Installation (Go module):**

```bash
cd services/api
go mod init github.com/luongdev/open-routing/services/api

go get \
  github.com/go-chi/chi/v5 \
  github.com/jackc/pgx/v5 \
  github.com/jackc/pgx/v5/pgxpool \
  github.com/golang-migrate/migrate/v4 \
  github.com/google/uuid \
  github.com/redis/go-redis/v9 \
  github.com/pganalyze/pg_query_go/v6 \
  go.opentelemetry.io/otel \
  go.opentelemetry.io/otel/sdk \
  go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp \
  go.opentelemetry.io/otel/exporters/stdout/stdouttrace \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \
  github.com/testcontainers/testcontainers-go \
  github.com/testcontainers/testcontainers-go/modules/postgres \
  github.com/stretchr/testify
```

**Version verification (run during planning):**

```bash
# Confirmed in research session 2026-05-15 via go.dev proxy + slopcheck
go list -m -versions github.com/jackc/pgx/v5             # → v5.9.2 latest
go list -m -versions github.com/go-chi/chi/v5            # → v5.2.3 latest
go list -m -versions github.com/google/uuid              # → v1.6.0 stable
go list -m -versions github.com/golang-migrate/migrate/v4 # → v4.19.1
go list -m -versions github.com/pganalyze/pg_query_go/v6 # → v6.x
```

## Package Legitimacy Audit

Slopcheck v0.6.1 was successfully installed via `pip install slopcheck --break-system-packages --system` during research and run against every recommended Go and npm package.

| Package | Registry | Slopcheck Verdict | Disposition |
|---------|----------|-------------------|-------------|
| `github.com/go-chi/chi/v5` | go.dev | [OK] | Approved |
| `github.com/jackc/pgx/v5` | go.dev | [SUS] — "26 days old" heuristic (false-positive; go proxy version dates reflect most-recent-fetch, not original publish) | Approved — pgx is established mainstream Go Postgres driver since 2016 |
| `github.com/sqlc-dev/sqlc` | go.dev | [SUS] — same heuristic noise | Approved — sqlc v1.31.1 is current stable release |
| `github.com/golang-migrate/migrate/v4` | go.dev | [OK] | Approved |
| `github.com/google/uuid` | go.dev | [OK] | Approved |
| `go.opentelemetry.io/otel` | go.dev | [OK] | Approved |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | go.dev | [OK] | Approved |
| `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` | go.dev | [OK] | Approved |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | go.dev | [OK] | Approved |
| `github.com/testcontainers/testcontainers-go` | go.dev | [OK] | Approved |
| `github.com/testcontainers/testcontainers-go/modules/postgres` | go.dev | [OK] | Approved |
| `github.com/redis/go-redis/v9` | go.dev | [SUS] — same heuristic noise | Approved — go-redis v9 is current line, official Redis client |
| `github.com/air-verse/air` | go.dev | [OK] | Approved |
| `github.com/pganalyze/pg_query_go/v6` | go.dev | [OK] | Approved |
| `github.com/stretchr/testify` | go.dev | [OK] | Approved |
| `github.com/jackc/pgx-zero-log` | go.dev | **[SLOP] — does not exist on go.dev. Made-up package.** | **REMOVED — was not actually in my recommendations; included in slopcheck run as a verification probe** |
| `typescript` | npm | [OK] | Approved |
| `turbo` | npm | [OK] | Approved |
| `eslint` | npm | [OK] | Approved |
| `@typescript-eslint/parser` | npm | [OK] | Approved |
| `@typescript-eslint/eslint-plugin` | npm | [OK] | Approved |
| `prettier` | npm | [OK] | Approved |

**Packages removed due to slopcheck [SLOP] verdict:** None of the recommended packages — the `pgx-zero-log` probe confirmed the slopcheck pipeline works; no slop in final stack.

**Packages flagged as suspicious [SUS]:** `pgx/v5`, `sqlc`, `go-redis/v9` — all flagged for "package age" heuristic, which is a known false-positive pattern for Go modules (the go.dev proxy reports the most-recent-fetched-version's first-seen timestamp, not the project's age). All three are mainstream packages with millions of installs and long histories; manually verified each via pkg.go.dev. **No checkpoint gate required.**

## Architecture Patterns

### System Architecture Diagram

```
                      ┌─────────────────────────────────────────────┐
                      │              GitHub Actions CI              │
                      │  go vet │ go lint │ go test -race │ pnpm    │
                      │  typecheck │ pnpm lint │ migration-drift    │
                      └────────────────────┬────────────────────────┘
                                           │ blocks merge
                                           ▼
  HTTP request                  ┌────────────────────┐
  X-Org-Id: uuidv7  ──────────► │   otelhttp.Handler │ ── creates root span
                                │   (wraps chi mux)  │
                                └─────────┬──────────┘
                                          ▼
                              ┌───────────────────────┐
                              │  chi mux (root)        │
                              │  Use(Recoverer)        │
                              │  Use(RequestID-uuid7)  │── X-Request-Id (response)
                              │  Use(OrgContext)       │── parses+validates X-Org-Id,
                              └───────┬───────────────┘    sets uuid.UUID in ctx,
                                      │                    adds span attr "org_id"
                          ┌───────────┴────────────┐
                          ▼                         ▼
                    /healthz                  /v1/orgs/{org_id}/_scaffold
                    /readyz   (bypass         │
                    /metrics   org middleware)│
                                              ▼
                                       handler (scaffold)
                                              │
                                              ▼
                            db := orgDB.From(pgxpool, ctx)
                                              │
                                              ▼
                                  q := scaffold.New(db)  ◄─ sqlc-generated; db is DBTX
                                              │
                                              ▼
                                 q.InsertScaffold(ctx, ...)
                                              │
                                              ▼
                                    orgDB.Exec(ctx, sql, args)
                                              │
                              ┌───────────────┴──────────────┐
                              │ 1. ctx must carry uuid.UUID  │
                              │    org_id (D-03) — else err  │
                              │ 2. Bypass marker in ctx?     │
                              │    yes → slog.Warn, skip (3) │
                              │ 3. SQL parsed via            │
                              │    pg_query_go (cache by     │
                              │    SHA-256). Must contain    │
                              │    org_id = $N in WHERE for  │
                              │    SELECT/UPDATE/DELETE; or  │
                              │    org_id column in INSERT.  │
                              │    fail → panic dev/test,    │
                              │    error in prod (D-02)      │
                              └───────────────┬──────────────┘
                                              ▼
                                      *pgxpool.Pool
                                              │
                                              ▼
                                       PostgreSQL 17
                                       (docker-compose dev,
                                        testcontainers-go in
                                        tests + CI)

                          slog.JSONHandler wrapped by
                          custom handler that reads
                          trace.SpanFromContext(ctx) →
                          injects trace_id, span_id,
                          org_id, request_id into every
                          log record. Emits to stdout.

                          OTel SDK init order (before chi):
                          1. tracer provider w/ stdout OR otlphttp exporter
                          2. set global tracer
                          3. set global propagator
                          4. construct chi mux
                          5. wrap in otelhttp.NewHandler
                          6. http.ListenAndServe
```

### Recommended Project Structure

```
open-routing/
├── .github/
│   └── workflows/
│       └── ci.yml                  # go-vet, go-lint, go-test, web-typecheck, web-lint, migration-drift
├── docker-compose.yml              # postgres:17, redis:7-alpine (D-25)
├── Taskfile.yml                    # root orchestrator (D-08)
├── .editorconfig                   # (recommended per Claude's discretion)
├── .gitattributes                  # (recommended)
├── migrations/                     # golang-migrate, NNNNNN_name.up.sql / .down.sql (D-10)
│   ├── 000001_create_scaffold.up.sql
│   └── 000001_create_scaffold.down.sql
├── openapi/
│   └── .gitkeep                    # Phase 2 fills openapi.yaml
├── services/
│   └── api/
│       ├── go.mod                  # module github.com/luongdev/open-routing/services/api (D-22)
│       ├── go.sum
│       ├── .air.toml               # hot reload config
│       ├── .golangci.yml           # golangci-lint config
│       ├── sqlc.yaml               # sqlc codegen config
│       ├── cmd/
│       │   ├── api/
│       │   │   └── main.go         # API binary entry
│       │   └── migrate/
│       │       └── main.go         # Migration runner; only legitimate WithBypass caller (D-11)
│       └── internal/
│           ├── config/
│           │   └── config.go       # 12-factor env loader
│           ├── telemetry/
│           │   ├── otel.go         # tracer provider + exporter selection
│           │   └── slog.go         # custom handler w/ trace_id injection
│           ├── server/
│           │   ├── server.go       # chi mux wiring, otelhttp wrap, route mounts
│           │   └── routes.go       # route registration
│           ├── middleware/
│           │   ├── recover.go      # panic recovery (custom — chi's Recoverer is fine; reuse it)
│           │   ├── requestid.go    # UUIDv7 request id (D-28)
│           │   └── orgcontext.go   # X-Org-Id extraction + validation + ctx injection (D-20, FOUND-03)
│           ├── db/
│           │   ├── pool.go         # pgxpool.New with retries
│           │   ├── orgdb.go        # the wrapper implementing pgx-style DBTX
│           │   ├── sqlcheck.go     # pg_query_go SQL inspection + SHA-256 cache
│           │   ├── bypass.go       # WithBypass(ctx, reason) + marker type
│           │   ├── queries/
│           │   │   └── scaffold.sql # sqlc input
│           │   └── generated/
│           │       ├── db.go       # sqlc output (Queries struct, DBTX interface)
│           │       ├── models.go
│           │       └── scaffold.sql.go
│           └── scaffold/
│               ├── handler.go      # chi handler funcs for POST/GET/GET routes
│               └── service.go      # business logic, gets *orgDB from middleware
└── web/
    ├── pnpm-workspace.yaml
    ├── package.json
    ├── tsconfig.base.json
    ├── turbo.json                  # typecheck + lint pipelines (D-09)
    ├── apps/
    │   ├── admin/
    │   │   ├── package.json
    │   │   ├── tsconfig.json
    │   │   └── src/index.ts        # empty stub (D-12)
    │   └── embed/
    │       ├── package.json
    │       ├── tsconfig.json
    │       └── src/index.ts        # empty stub
    └── packages/
        └── ui/
            ├── package.json
            ├── tsconfig.json
            └── src/index.ts        # empty stub
```

### Pattern 1: `context.Context` propagation of `org_id` (FOUND-05)

**What:** `org_id` lives in `context.Context` from the middleware layer down to the orgDB wrapper — never threaded as a function argument, never stored in struct fields, never globals.

**When to use:** Every request-scoped value (org_id, request_id, OTel span) follows this pattern. Worker / background goroutines that DON'T have an incoming request must construct an explicit ctx with `orgDB.WithBypass(ctx, reason)` (Phase 1: only `cmd/migrate`).

**Example:**

```go
// internal/middleware/orgcontext.go
package middleware

import (
    "context"
    "net/http"

    "github.com/google/uuid"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

// Unexported ctx key type prevents collisions with other packages.
type orgIDCtxKey struct{}

// SetOrgID returns a copy of ctx with the org_id stored.
// Always passes uuid.UUID (not string) for type-safety per D-20.
func SetOrgID(ctx context.Context, id uuid.UUID) context.Context {
    return context.WithValue(ctx, orgIDCtxKey{}, id)
}

// OrgIDFromContext extracts the org_id; returns false if missing.
func OrgIDFromContext(ctx context.Context) (uuid.UUID, bool) {
    id, ok := ctx.Value(orgIDCtxKey{}).(uuid.UUID)
    return id, ok
}

// OrgContext middleware: parses X-Org-Id, validates UUIDv7+, stores in ctx, adds span attr.
func OrgContext(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        raw := r.Header.Get("X-Org-Id")
        if raw == "" {
            writeError(w, http.StatusBadRequest, "invalid_org_id", "missing_header")
            return
        }
        id, err := uuid.Parse(raw)
        if err != nil {
            writeError(w, http.StatusBadRequest, "invalid_org_id", "malformed_uuid")
            return
        }
        if id.Version() < 7 {
            writeError(w, http.StatusBadRequest, "invalid_org_id", "uuidv7_required")
            return
        }

        ctx := SetOrgID(r.Context(), id)

        // Add to the current OTel span (FOUND-07).
        span := trace.SpanFromContext(ctx)
        span.SetAttributes(attribute.String("org_id", id.String()))

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Pattern 2: orgDB wrapper implementing the sqlc DBTX interface

**What:** A struct wrapping `*pgxpool.Pool` (the underlying pool) that implements the three methods sqlc expects (`Exec`, `Query`, `QueryRow`) and inspects every SQL string before delegating.

**When to use:** Every handler that touches the database. Handlers call `orgDB.From(pool, ctx)` (returns a per-request `*orgDB`) and pass it to the sqlc `New(db)` constructor. This makes "use a raw pool" a compile error — the sqlc `Queries` factory only accepts a DBTX.

**Example:**

```go
// internal/db/orgdb.go
package db

import (
    "context"
    "errors"
    "fmt"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/luongdev/open-routing/services/api/internal/middleware"
)

// Pool is the only field; we never expose it directly.
type OrgDB struct {
    pool   *pgxpool.Pool
    checker *SQLChecker // pg_query_go-based inspector (sqlcheck.go), shared across all orgDB instances
    mode   ValidationMode // panic (dev/test) vs return-error (prod)
}

type ValidationMode int

const (
    ValidationPanic ValidationMode = iota // dev / test
    ValidationError                        // prod
)

var ErrOrgIDMissingFromContext = errors.New("orgdb: ctx missing org_id; refusing to query")
var ErrSQLMissingOrgFilter     = errors.New("orgdb: SQL string missing org_id filter")

// From returns a per-request handle. Cheap (no allocation per call besides the struct itself).
// In production this is typically constructed once per *Pool and reused; the ctx is checked per Exec/Query.
func (p *PoolHandle) Make(ctx context.Context) *OrgDB { /* factory */ }

// Exec implements pgx-style DBTX.Exec — exact signature sqlc generates for pgx/v5.
func (o *OrgDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
    if err := o.preflight(ctx, sql); err != nil {
        return pgconn.CommandTag{}, err
    }
    return o.pool.Exec(ctx, sql, args...)
}

// Query implements pgx-style DBTX.Query.
func (o *OrgDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
    if err := o.preflight(ctx, sql); err != nil {
        return nil, err
    }
    return o.pool.Query(ctx, sql, args...)
}

// QueryRow implements pgx-style DBTX.QueryRow. pgx.Row defers errors to Scan, so we
// can't return an err here — if preflight fails, we panic or return a "broken" row that
// errors on Scan. The path below uses panic in both modes for QueryRow since the alternative
// requires returning a synthetic pgx.Row stub.
func (o *OrgDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
    if err := o.preflight(ctx, sql); err != nil {
        panic(fmt.Errorf("orgdb.QueryRow preflight failed (panic regardless of mode): %w", err))
    }
    return o.pool.QueryRow(ctx, sql, args...)
}

// preflight runs the three checks: ctx has org_id, SQL has org_id filter, bypass marker.
func (o *OrgDB) preflight(ctx context.Context, sql string) error {
    // 1. Bypass shortcut.
    if reason, ok := BypassReason(ctx); ok {
        slog.WarnContext(ctx, "orgdb bypass",
            "event", "orgdb_bypass",
            "reason", reason,
            "sql_hash", hashSQL(sql),
        )
        return nil // skip all further checks
    }

    // 2. ctx must carry org_id (D-03).
    if _, ok := middleware.OrgIDFromContext(ctx); !ok {
        return ErrOrgIDMissingFromContext
    }

    // 3. SQL must reference org_id (D-02). Cache by SHA-256.
    if err := o.checker.MustContainOrgFilter(sql); err != nil {
        if o.mode == ValidationPanic {
            panic(err)
        }
        return err
    }
    return nil
}
```

### Pattern 3: OTel init BEFORE chi router setup (FOUND-07)

**What:** Construct the tracer provider, set it globally, then build the chi mux. Wrap the mux in `otelhttp.NewHandler`. This sequence is enforced by FOUND-07.

**When to use:** In `cmd/api/main.go`, at the top of `main()`, before any HTTP work.

**Example:**

```go
// cmd/api/main.go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "time"

    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

    "github.com/luongdev/open-routing/services/api/internal/server"
    "github.com/luongdev/open-routing/services/api/internal/telemetry"
)

func main() {
    ctx, stop := context.WithCancel(context.Background())
    defer stop()

    // ---- OTel SDK init (BEFORE chi mux) ----
    shutdown, err := initOTel(ctx)
    if err != nil {
        slog.Error("otel init", "err", err)
        os.Exit(1)
    }
    defer func() {
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = shutdown(shutdownCtx)
    }()

    // ---- slog init (depends on OTel global being set) ----
    handler := telemetry.NewTracingHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(slog.New(handler))

    // ---- chi mux + middleware ----
    mux := server.NewMux( /* deps */ )

    // ---- wrap mux in otelhttp ----
    rootHandler := otelhttp.NewHandler(mux, "open-routing-api")

    srv := &http.Server{
        Addr:    ":8080",
        Handler: rootHandler,
        ReadHeaderTimeout: 10 * time.Second,
    }

    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("listen", "err", err)
        }
    }()
    <-ctx.Done()
    _ = srv.Shutdown(context.Background())
}

func initOTel(ctx context.Context) (func(context.Context) error, error) {
    res, _ := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName("open-routing-api"),
            semconv.ServiceVersion("0.1.0"),
        ),
    )

    var exporter sdktrace.SpanExporter
    var err error

    switch os.Getenv("OTEL_EXPORTER") {
    case "otlp":
        exporter, err = otlptracehttp.New(ctx) // reads OTEL_EXPORTER_OTLP_ENDPOINT env
    default:
        exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
    }
    if err != nil {
        return nil, err
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(res),
    )
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))
    return tp.Shutdown, nil
}
```

### Pattern 4: testcontainers-go isolation suite (FOUND-08)

**What:** Package-level `TestMain` spawns one Postgres 17 container, runs migrations programmatically via `migrate.NewWithDatabaseInstance`, then the tests `t.Parallel()` against shared schema with isolated org_id values.

**When to use:** For the FOUND-08 acceptance test and every future integration test that needs Postgres. Each test creates fresh UUIDv7 org_ids (Pitfall 1.5: never share org IDs across tests).

**Example:**

```go
// internal/scaffold/integration_test.go
package scaffold_test

import (
    "context"
    "database/sql"
    "fmt"
    "net/http/httptest"
    "os"
    "testing"
    "time"

    "github.com/golang-migrate/migrate/v4"
    pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)

var sharedPool *pgxpool.Pool

func TestMain(m *testing.M) {
    ctx := context.Background()

    pgC, err := postgres.Run(ctx,
        "postgres:17",
        postgres.WithDatabase("openrouting_test"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).WithStartupTimeout(60 * time.Second),
        ),
    )
    if err != nil {
        panic(err)
    }

    connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        panic(err)
    }

    // Apply migrations programmatically.
    db, err := sql.Open("pgx", connStr) // or use the standard postgres driver
    if err != nil {
        panic(err)
    }
    driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
    if err != nil {
        panic(err)
    }
    m2, err := migrate.NewWithDatabaseInstance(
        "file://../../../migrations",
        "postgres",
        driver,
    )
    if err != nil {
        panic(err)
    }
    if err := m2.Up(); err != nil && err != migrate.ErrNoChange {
        panic(err)
    }

    sharedPool, err = pgxpool.New(ctx, connStr)
    if err != nil {
        panic(err)
    }

    code := m.Run()

    sharedPool.Close()
    _ = pgC.Terminate(ctx)
    os.Exit(code)
}

// TestTwoOrgsIsolation is the canonical FOUND-08 proof.
func TestTwoOrgsIsolation(t *testing.T) {
    t.Parallel()

    orgA := uuid.Must(uuid.NewV7())
    orgB := uuid.Must(uuid.NewV7())

    // Build the same chi mux production uses.
    mux := server.NewMux(sharedPool /* + other deps */)
    ts := httptest.NewServer(mux)
    defer ts.Close()

    client := ts.Client()

    // Seed: both orgs create _scaffold rows with identical external_id "ext-1".
    postScaffold(t, client, ts.URL, orgA, "ext-1", "scaffold-A")
    postScaffold(t, client, ts.URL, orgB, "ext-1", "scaffold-B")

    // List as orgA: must see only orgA's row.
    rowsA := getScaffolds(t, client, ts.URL, orgA)
    require.Len(t, rowsA, 1)
    require.Equal(t, "scaffold-A", rowsA[0].Name)

    // List as orgB: must see only orgB's row.
    rowsB := getScaffolds(t, client, ts.URL, orgB)
    require.Len(t, rowsB, 1)
    require.Equal(t, "scaffold-B", rowsB[0].Name)

    // Targeted GET: orgA cannot read orgB's row by ID.
    require.Equal(t, 404, getScaffoldStatus(t, client, ts.URL, orgA, rowsB[0].ID))
}

// Additional cases must include:
//   - missing X-Org-Id header → 400
//   - malformed UUID → 400 + "malformed_uuid"
//   - UUIDv4 → 400 + "uuidv7_required"
//   - bypass paths (/healthz, /readyz, /metrics) succeed without header
//   - bypass paths never query against orgDB strict path
```

### Pattern 5: GitHub Actions matrix (FOUND-09)

**What:** Single `.github/workflows/ci.yml` running multiple jobs in parallel; all required for merge; concurrency cancels older runs.

**Example:**

```yaml
# .github/workflows/ci.yml
name: ci
on:
  pull_request:
  push:
    branches: [main]

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

jobs:
  go-vet:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: services/api/go.mod
          cache-dependency-path: services/api/go.sum
      - working-directory: services/api
        run: go vet ./...

  go-lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: services/api/go.mod
      - uses: golangci/golangci-lint-action@v6
        with:
          working-directory: services/api
          version: v1.63

  go-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: services/api/go.mod
      - working-directory: services/api
        run: go test -race ./...
        # testcontainers-go works on default GitHub Actions runners since Docker is preinstalled.

  web-typecheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml
      - working-directory: web
        run: |
          pnpm install --frozen-lockfile
          pnpm -F '*' typecheck

  web-lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml
      - working-directory: web
        run: |
          pnpm install --frozen-lockfile
          pnpm -F '*' lint

  migration-drift:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:17
        env:
          POSTGRES_PASSWORD: drift
        options: >-
          --health-cmd "pg_isready" --health-interval 5s
          --health-timeout 3s --health-retries 5
        ports: ['5432:5432']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: services/api/go.mod
      - run: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1
      - run: |
          migrate -path migrations -database "postgres://postgres:drift@localhost/postgres?sslmode=disable" up
          migrate -path migrations -database "postgres://postgres:drift@localhost/postgres?sslmode=disable" version
```

Note: The `migration-drift` job uses a GitHub Actions service container for raw migration apply (not testcontainers-go) because (a) it does not exercise the API code path, so D-06 same-code-path constraint doesn't apply, and (b) the goal is to detect malformed migration files independently of Go code.

### Pattern 6: docker-compose.yml (FOUND-10 + D-25)

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_DB: openrouting
      POSTGRES_USER: openrouting
      POSTGRES_PASSWORD: openrouting   # dev only; never used in prod
    ports:
      - "5432:5432"
    volumes:
      - openrouting_pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U openrouting -d openrouting"]
      interval: 5s
      timeout: 3s
      retries: 5
      start_period: 10s

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  openrouting_pg_data:
```

D-25 deliberately omits an `api` service. `task dev` runs `docker compose up -d postgres redis`, waits for healthchecks, then foregrounds `air` for hot-reload Go builds.

**FOUND-10 nuance:** The requirement says "Docker Compose brings up PostgreSQL 17, Redis, the Go API, and the Vite dev servers with a single command." CONTEXT.md D-25 narrows this: in Phase 1 docker-compose contains only postgres + redis; the API runs native via `air`. Reconcile: the planner should treat **`task dev`** as the "single command" the user invokes — it orchestrates `docker compose up -d` + `air`. This satisfies the user-facing intent ("one command, get everything running") without violating D-25. Phase 6/7 may add Vite dev server containers if useful.

### Anti-Patterns to Avoid

- **Passing `org_id` as a function argument:** breaks FOUND-05. Always pull from `context.Context`.
- **Storing `org_id` as `string` in ctx:** breaks D-20 type safety. Use `uuid.UUID`.
- **Auto-running migrations from `cmd/api`:** breaks D-11. Migrations are an explicit step (`task migrate-up` / `cmd/migrate`).
- **Hand-rolled SQL parser regex for orgDB inspection:** breaks correctness. Joins, CTEs, subqueries will defeat regex. Use `pg_query_go`.
- **Reusing the same testcontainers Postgres container across packages:** introduces flake. Each package's `TestMain` creates its own (D-06 mandates same code path local + CI; per-package isolation is the cheap insurance).
- **Logging non-JSON in dev "for readability":** breaks D-27 consistency. Use `jq` instead.
- **Wrapping the chi mux in middleware BEFORE `otelhttp.NewHandler`:** breaks span lineage. Order must be: `chi.NewRouter() → mux.Use(...) → otelhttp.NewHandler(mux, ...)`. Otherwise the span ends before middleware runs.
- **Putting bypass middleware inside chi `Use()`:** breaks D-21. Bypass paths (`/healthz`, `/readyz`, `/metrics`) need the org middleware to NOT run; use chi's `Group()` to scope middleware to `/v1/...`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SQL parsing for orgDB validation | Regex matching `WHERE.*org_id` | `github.com/pganalyze/pg_query_go/v6` | Joins, subqueries, CTEs, unioned queries make regex impossible. pg_query_go is the actual Postgres parser compiled to Go via CGO; gives reliable AST access. |
| Database migrations | Ad-hoc SQL files + custom runner | `github.com/golang-migrate/migrate/v4` | Versioning, up/down semantics, advisory locks, schema_migrations table — all solved. Phase 1 doesn't yet need locking (v0.1 is single-replica) but adopting the standard tool now avoids retrofit. |
| HTTP router | net/http + manual matcher | `github.com/go-chi/chi/v5` | Path params, middleware composition, sub-routers — chi solves them with 100% net/http compatibility, zero-alloc routing. Locked stack choice. |
| UUID generation + version check | Custom RFC 9562 implementation | `github.com/google/uuid` v1.6+ | RFC 9562 §5.7 is non-trivial; `uuid.NewV7()` is correct and pool-backed. |
| HTTP span instrumentation | Manual `trace.Start` per handler | `otelhttp.NewHandler` wrapping mux | One line of code; gets request method, route, status, latency, semantic conventions correct. |
| Postgres in tests | Local Postgres install required | `testcontainers-go/modules/postgres` | Ephemeral; clean state per test; identical local + CI; configurable image (Postgres 17). |
| slog → OTel trace correlation | Forward log lines to a separate pipeline | Custom `slog.Handler` wrapper (~30 LoC) | The wrapper just reads `trace.SpanFromContext(ctx)` on Handle and appends two attrs; `otelslog` is overkill (it routes logs through OTel logs SDK, not stdout). |
| Docker Compose healthchecks | Custom wait scripts | Compose `healthcheck:` + `depends_on: condition: service_healthy` | Native, well-tested, no `wait-for-it.sh` style scripts. |

**Key insight:** The orgDB SQL validation is the highest-stakes hand-roll temptation in this phase. A regex looks simple — `MATCH(sql, /org_id\s*=\s*\$\d+/)` — but it produces false negatives on `JOIN ... ON foo.org_id = bar.org_id`, `WITH x AS (SELECT ... WHERE org_id = $1) SELECT ...`, `INSERT INTO t (a, b, org_id) VALUES (...)`, and false positives on `-- comment mentioning org_id`. A failed validator that admits a leaky query is exactly the bug Pitfall 1.4 describes. pg_query_go's CGO build cost (~3 min initial; cached thereafter) is cheap insurance.

## Runtime State Inventory

**Skipped.** This is a greenfield phase. There are no stored data, no live service configs, no OS-registered state, no pre-existing secrets, and no build artifacts from prior runs. Every artifact in Phase 1 is created from scratch.

| Category | Items Found |
|----------|-------------|
| Stored data | None — no databases exist yet. |
| Live service config | None — no n8n / Datadog / Tailscale / Cloudflare configs in scope. |
| OS-registered state | None — no Task Scheduler / pm2 / systemd registrations. |
| Secrets/env vars | None pre-existing; Phase 1 introduces `DATABASE_URL`, `REDIS_URL`, `OTEL_EXPORTER`, `OTEL_EXPORTER_OTLP_ENDPOINT` (loaded from `.env` in dev via 12-factor pattern). |
| Build artifacts | None — no prior installs. |

## Common Pitfalls

### Pitfall 1: Cache key omits `org_id` — cross-org cache poisoning

**What goes wrong:** Phase 1 doesn't yet ship Redis-cached endpoints (Phase 3 owns CAT-11), but the `/readyz` Redis ping pattern sets the precedent for cache key construction. Future caches that omit `org_id` from the key serve org A's data to org B silently.

**Why it happens:** Caching is treated as a performance afterthought. The `cache.Get("agents:list")` call ships before anyone codes the `org_id` prefix.

**How to avoid:** Establish a `cacheKey(orgID uuid.UUID, segments ...string) string` helper now in Phase 1 (even if unused) so Phase 3 has no excuse to write a key without `org_id`. Reference: `or:{orgId}:{entity}:{id}` (per CAT-11).

**Warning signs:** Any future cache helper that accepts `(resource, id)` without `org_id`.

### Pitfall 2: orgDB SQL inspection misses joined `org_id` references

**What goes wrong:** The validator looks for a literal `org_id = $N` pattern in the WHERE clause. A query joins `agents JOIN agent_skills ON agents.org_id = agent_skills.org_id WHERE agent_skills.skill_id = $1` — the agents row is implicitly org-scoped via the join, but the WHERE clause has no `org_id` reference. Validator accepts the query because pg_query_go's AST walker correctly identifies the join condition.

**Why it happens:** The semantic question is "does this query produce a result set scoped to one org?" Answering it correctly requires walking joins, subqueries, and CTEs. The Phase 1 implementation must handle the common shapes: bare WHERE, joined WHERE, INSERT column list, UPDATE WHERE, DELETE WHERE. Defer CTEs and subqueries to Phase 3 only if every Phase 1 sqlc query is a flat statement.

**How to avoid:** Walk the parse tree for any `org_id` reference (column reference where the name is `org_id`, used in equality or IN comparison). Reject only if **none** of `org_id` appears in any WHERE-equivalent position. The simplest correct check for Phase 1: search the tree for any column reference named `org_id` and assert at least one exists in a comparison node.

**Warning signs:** Validator that only checks the top-level `Where` field of a Postgres Select node. Validator that doesn't recurse into JoinExpr.

### Pitfall 3: Background goroutines lose org_id

**What goes wrong:** A handler spawns `go doStuff(ctx)` to fire-and-forget some work. The parent ctx is cancelled when the response is written; the child goroutine's DB call fails because either (a) ctx is cancelled or (b) the org_id has gone out of scope.

**Why it happens:** Go's context cancellation is request-scoped by default. Async work needs `context.WithoutCancel(ctx)` (Go 1.21+) to preserve values but detach from the parent's cancellation.

**How to avoid:** For Phase 1, there are no background goroutines (the WrapUp goroutine arrives in Phase 4). Document the pattern in the orgDB package doc comment so Phase 4 implementers don't miss it.

**Warning signs:** Any `go func() { ... }` in handler code that uses the original ctx.

### Pitfall 4: testcontainers Postgres image cold-start fails CI

**What goes wrong:** GitHub Actions runner pulls `postgres:17` for the first time in a fresh runner image; the pull takes 60s+; the testcontainers wait strategy times out at 30s; CI fails non-deterministically.

**Why it happens:** Default wait timeouts are too tight for cold runners. The `WithOccurrence(2)` pattern (postgres logs "ready" twice during init) helps but doesn't extend the per-occurrence timeout.

**How to avoid:** Set explicit `WithStartupTimeout(60 * time.Second)` on the wait strategy. Optionally pre-pull the image in a CI step before the test run (`docker pull postgres:17`).

**Warning signs:** Intermittent CI failures with "container startup timeout" in test logs.

### Pitfall 5: Forgetting middleware order means org_id missing from spans

**What goes wrong:** OTel `otelhttp.NewHandler` wraps the chi mux at the root; chi's `RequestID` and `OrgContext` middleware run inside the mux. If the OTel span is started before the chi middleware runs, the span exists but lacks the `org_id` attribute when the OrgContext middleware runs after. The `SetAttributes` call works, but only because OTel allows late attribute setting; if `otelhttp.NewHandler` finishes the span before the middleware runs (which would happen if you ordered them backward), the attribute is lost.

**Why it happens:** Confused middleware chain order. The fix is: `otelhttp.NewHandler(mux)` wraps the mux — meaning `mux.ServeHTTP` runs inside the otel span. As long as the middleware that calls `SetAttributes` runs before `next.ServeHTTP`, the attribute lands on the active span correctly.

**How to avoid:** Always wrap the chi mux **after** all `mux.Use(...)` calls. The order `chi → use middleware → wrap in otelhttp → http.ListenAndServe` is correct.

**Warning signs:** Spans missing `org_id` attribute even though OrgContext middleware logs success.

### Pitfall 6: Test fixtures bleed between orgs in CI (Pitfall 1.5 from PITFALLS.md)

**What goes wrong:** Multiple tests share a Postgres container (sound choice per Pattern 4) but use a static `defaultOrgID = uuid.MustParse("...")` for "the test org." Tests run in parallel; one test asserts `len(rows) == 3` based on its inserts, but a concurrent test inserts 2 more rows under the same org_id; assertion fails.

**Why it happens:** Static org_id is shared state. Fresh UUIDv7 per test is cheap.

**How to avoid:** Every test calls `uuid.Must(uuid.NewV7())` at the top to mint a fresh org_id. Never define `DEFAULT_ORG_ID = ...` as a package-level constant.

**Warning signs:** Package-level `var defaultOrgID = uuid.MustParse(...)` or similar.

### Pitfall 7: `migrate.NewWithDatabaseInstance` requires a `*sql.DB`, not a pgxpool

**What goes wrong:** You try to pass `pgxpool.Pool` directly to `pgmigrate.WithInstance`; compile error or runtime panic.

**Why it happens:** golang-migrate's postgres driver uses `database/sql` semantics (`*sql.DB`), not pgx native. The migration runner has to open a parallel `database/sql` connection.

**How to avoid:** In `cmd/migrate` (and in `TestMain`), open with `sql.Open("pgx", connStr)` (using the pgx `database/sql` shim, `github.com/jackc/pgx/v5/stdlib`) or `sql.Open("postgres", connStr)` (using lib/pq). Either works. The app's `pgxpool.Pool` is independent.

**Warning signs:** Trying to share a pool between migrate and runtime.

### Pitfall 8: `air` version mismatch with Go toolchain

**What goes wrong:** `air@latest` (v1.65+) requires Go 1.25, but the project pins Go 1.23 in go.mod. `go install github.com/air-verse/air@latest` succeeds (it uses the installer's Go), then `air` complains about Go version mismatch at runtime.

**Why it happens:** Air's Go version dependency is forward-only.

**How to avoid:** Pin air to v1.61.x (last version with Go 1.21+ support) in install instructions, or upgrade Go to 1.25. Document in CONTRIBUTING.md.

**Warning signs:** "go: module requires Go 1.25 ..." errors during dev startup.

## Code Examples

Verified patterns and signatures, with full source attribution.

### Example 1: sqlc DBTX interface signature (pgx/v5)

The sqlc-generated `db.go` defines the DBTX interface that any wrapper must satisfy:

```go
// Source: sqlc 1.31.1 generated output for sql_package: "pgx/v5"
// docs.sqlc.dev/en/latest/guides/using-go-and-pgx.html

type DBTX interface {
    Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
    Query(context.Context, string, ...interface{}) (pgx.Rows, error)
    QueryRow(context.Context, string, ...interface{}) pgx.Row
}

type Queries struct {
    db DBTX
}

func New(db DBTX) *Queries {
    return &Queries{db: db}
}

func (q *Queries) WithTx(tx pgx.Tx) *Queries {
    return &Queries{db: tx}
}
```

Crucially: sqlc duck-types on the interface. Any struct implementing those three methods with exactly those signatures (note: no `Context` suffix unlike `database/sql`) satisfies DBTX. The orgDB wrapper must match these signatures exactly.

### Example 2: sqlc query file authoring `org_id = $1` explicitly (D-02 contract)

```sql
-- internal/db/queries/scaffold.sql
-- Source: sqlc query file conventions — docs.sqlc.dev/en/latest/howto/select.html

-- name: InsertScaffold :one
INSERT INTO _scaffold (id, org_id, external_id, name)
VALUES ($1, $2, $3, $4)
RETURNING id, org_id, external_id, name, created_at;

-- name: GetScaffoldByID :one
SELECT id, org_id, external_id, name, created_at
FROM _scaffold
WHERE id = $1 AND org_id = $2;

-- name: ListScaffolds :many
SELECT id, org_id, external_id, name, created_at
FROM _scaffold
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT 100;
```

Every query has `org_id` in INSERT column list or WHERE clause — the orgDB validator accepts these. A query author who forgets `org_id` either writes a SQL that the validator rejects (panic in dev) or never compiles (sqlc errors if a parameter is unused).

### Example 3: sqlc.yaml configuration

```yaml
# services/api/sqlc.yaml
# Source: docs.sqlc.dev/en/latest/reference/config.html

version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/db/queries"
    schema: "../../migrations"
    gen:
      go:
        package: "generated"
        out: "internal/db/generated"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_prepared_queries: false
        emit_interface: false
        emit_exact_table_names: false
        emit_empty_slices: true
        emit_pointers_for_null_types: true
```

Note `schema` points at the migrations directory — sqlc reads `up.sql` files to build the model types.

### Example 4: Custom slog handler injecting trace_id / span_id

```go
// internal/telemetry/slog.go
// Source: Pattern adapted from github.com/remychantenay/slog-otel and pkg.go.dev/log/slog handler docs

package telemetry

import (
    "context"
    "log/slog"

    "go.opentelemetry.io/otel/trace"

    "github.com/luongdev/open-routing/services/api/internal/middleware"
)

type TracingHandler struct {
    inner slog.Handler
}

func NewTracingHandler(inner slog.Handler) *TracingHandler {
    return &TracingHandler{inner: inner}
}

func (h *TracingHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.inner.Enabled(ctx, level)
}

func (h *TracingHandler) Handle(ctx context.Context, rec slog.Record) error {
    // OTel correlation.
    if span := trace.SpanFromContext(ctx); span != nil && span.SpanContext().IsValid() {
        rec.AddAttrs(
            slog.String("trace_id", span.SpanContext().TraceID().String()),
            slog.String("span_id", span.SpanContext().SpanID().String()),
        )
    }
    // org_id (best-effort — bypass / health endpoints have no org_id).
    if id, ok := middleware.OrgIDFromContext(ctx); ok {
        rec.AddAttrs(slog.String("org_id", id.String()))
    }
    // request_id is set by chi middleware.RequestID + custom UUIDv7 override.
    return h.inner.Handle(ctx, rec)
}

func (h *TracingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &TracingHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *TracingHandler) WithGroup(name string) slog.Handler {
    return &TracingHandler{inner: h.inner.WithGroup(name)}
}
```

### Example 5: chi middleware chain wiring with bypass paths

```go
// internal/server/server.go
// Source: chi v5 patterns — pkg.go.dev/github.com/go-chi/chi/v5

func NewMux(deps *Deps) *chi.Mux {
    r := chi.NewRouter()

    // Global middleware (runs on every route, including /healthz).
    r.Use(middleware.Recoverer)           // chi stdlib
    r.Use(customMW.RequestID)              // UUIDv7 instead of chi's default (D-28)

    // Bypass-path routes — registered at root, do NOT carry OrgContext.
    r.Get("/healthz", healthHandler.Live)
    r.Get("/readyz",  healthHandler.Ready(deps.Pool, deps.Redis, deps.Migrate))
    r.Handle("/metrics", promhttp.Handler())

    // /v1 sub-router: org middleware applied here, only here.
    r.Route("/v1", func(v1 chi.Router) {
        v1.Use(customMW.OrgContext)
        v1.Mount("/orgs/{org_id}/_scaffold", scaffoldHandler.Routes(deps))
    })

    return r
}
```

`chi.Router.Route` creates a sub-router; middleware added inside the closure applies only to routes registered inside it. This satisfies D-21 cleanly.

### Example 6: `/readyz` deep-check handler

```go
// internal/scaffold/health.go (or internal/server/health.go)
// Source: pgx pool ping pattern — pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#Pool.Ping
//         redis ping — pkg.go.dev/github.com/redis/go-redis/v9#Client.Ping

type readyzBody struct {
    Status string         `json:"status"`
    Checks map[string]any `json:"checks"`
}

func ReadyzHandler(pool *pgxpool.Pool, rdb *redis.Client, migrationsPath, dbURL string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
        defer cancel()

        body := readyzBody{Status: "ok", Checks: map[string]any{}}
        statusCode := http.StatusOK

        if err := pool.Ping(ctx); err != nil {
            body.Checks["db"] = "fail"
            body.Status = "degraded"
            statusCode = http.StatusServiceUnavailable
        } else {
            body.Checks["db"] = "ok"
        }

        if err := rdb.Ping(ctx).Err(); err != nil {
            body.Checks["redis"] = "fail"
            body.Status = "degraded"
            statusCode = http.StatusServiceUnavailable
        } else {
            body.Checks["redis"] = "ok"
        }

        // Read schema_migrations.version (default table per golang-migrate postgres driver).
        var ver int
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
        _ = json.NewEncoder(w).Encode(body)
    }
}
```

The `schema_migrations` table is the default name created by `golang-migrate`'s postgres driver; the column structure is `(version BIGINT, dirty BOOLEAN)`. The `dirty=true` flag indicates a failed migration that should be flagged in readyz.

### Example 7: Taskfile structure (root)

```yaml
# Taskfile.yml
# Source: taskfile.dev/usage/ + D-08 target list

version: '3'

env:
  DATABASE_URL: 'postgres://openrouting:openrouting@localhost:5432/openrouting?sslmode=disable'
  REDIS_URL:    'redis://localhost:6379/0'

tasks:
  default:
    cmds:
      - task: --list

  # ---- Codegen ----
  gen:
    desc: Run sqlc + (Phase 2) oapi-codegen + openapi-typescript
    cmds:
      - cd services/api && sqlc generate

  # ---- Migrations ----
  migrate-up:
    desc: Apply all pending migrations
    cmds:
      - migrate -path migrations -database "$DATABASE_URL" up

  migrate-down:
    desc: Roll back the latest migration
    cmds:
      - migrate -path migrations -database "$DATABASE_URL" down 1

  migrate-create:
    desc: Create a new migration. Use NAME=<snake_case_name>.
    cmds:
      - migrate create -ext sql -dir migrations -seq {{.NAME}}
    requires:
      vars: [NAME]

  # ---- Dev ----
  dev:
    desc: Bring up infra and run API with hot-reload
    cmds:
      - docker compose up -d postgres redis
      - cd services/api && air -c .air.toml

  # ---- Test + Lint ----
  test:
    desc: Run all Go tests with race detector
    cmds:
      - cd services/api && go test -race ./...

  lint:
    desc: Run go vet + golangci-lint + pnpm lint
    cmds:
      - cd services/api && go vet ./...
      - cd services/api && golangci-lint run
      - cd web && pnpm -F '*' lint

  typecheck:
    cmds:
      - cd web && pnpm -F '*' typecheck

  # ---- CI (composes everything) ----
  ci:
    desc: Run the same checks CI runs
    cmds:
      - task: lint
      - task: typecheck
      - task: test

  # ---- Build ----
  build:
    desc: Build the API binary
    cmds:
      - cd services/api && go build -o ../../bin/api ./cmd/api
      - cd services/api && go build -o ../../bin/migrate ./cmd/migrate
```

### Example 8: pnpm workspace root files

```yaml
# web/pnpm-workspace.yaml
packages:
  - 'apps/*'
  - 'packages/*'
```

```json
// web/package.json
{
  "name": "open-routing-web",
  "private": true,
  "scripts": {
    "typecheck": "turbo typecheck",
    "lint": "turbo lint"
  },
  "devDependencies": {
    "turbo": "^2.0.0",
    "typescript": "^5.6.0",
    "eslint": "^9.0.0",
    "@typescript-eslint/parser": "^8.0.0",
    "@typescript-eslint/eslint-plugin": "^8.0.0"
  }
}
```

```json
// web/turbo.json — D-09 lives inside web/, not at repo root
{
  "$schema": "https://turbo.build/schema.json",
  "tasks": {
    "typecheck": {
      "dependsOn": ["^typecheck"],
      "outputs": []
    },
    "lint": {
      "outputs": []
    }
  }
}
```

```json
// web/tsconfig.base.json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "verbatimModuleSyntax": true
  }
}
```

### Example 9: First migration creating `_scaffold` (D-18)

```sql
-- migrations/000001_create_scaffold.up.sql
-- Source: D-18 schema in CONTEXT.md

CREATE TABLE _scaffold (
    id          UUID PRIMARY KEY,
    org_id      UUID NOT NULL,
    external_id TEXT NOT NULL,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, external_id)
);

CREATE INDEX idx_scaffold_org_id ON _scaffold (org_id);
```

```sql
-- migrations/000001_create_scaffold.down.sql

DROP TABLE _scaffold;
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `lib/pq` + `database/sql` | `pgx/v5` (native) | pgx v5 released 2022 | Lower latency, native types, real `pgxpool`, no `database/sql` indirection |
| `uuid v4` for primary keys | `uuid v7` (time-ordered) | RFC 9562 (2024) | Better B-tree locality; insert performance no longer degrades with high cardinality |
| `database/sql` `WithContext` methods | pgx `Exec`/`Query`/`QueryRow` (no Context suffix) | sqlc 1.20+ generates pgx-native code | Cleaner signatures; native pgx types in generated code |
| `zap` / `zerolog` for structured logging | stdlib `log/slog` | Go 1.21 (2023) | One less dependency; same JSON output; OTel bridge officially supported |
| Custom test setup with local Postgres | `testcontainers-go` | testcontainers-go v0.20+ | Identical CI + local; ephemeral; no DBA needed |
| `golangci-lint` v1.x with custom config | golangci-lint v1.63 (current stable) | ongoing | Faster runs, more linters; v2 line exists but v1 is sufficient |

**Deprecated / outdated for this project:**

- `pgx v4`: superseded by v5. Don't reference v4 docs.
- `chi v4`: superseded by v5 with go.mod support.
- `golang-migrate` v3: v4 is current; v3 docs are misleading.
- `XSS Encoder` / `RealIP` ordered before `RequestID`: chi recommends `RealIP` first, `RequestID` second now, but PROJECT decisions sit between `Recoverer` first and `RequestID` second per D-28 — follow CONTEXT.md, not the chi README defaults.
- `testcontainers.RunContainer(...)` — deprecated; use `postgres.Run(ctx, image, opts...)`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | pg_query_go's CGO build adds ~3 min initial compile but caches thereafter | Don't Hand-Roll | If the cache doesn't work as expected in CI, CI cold builds become painfully slow. Mitigation: cache the Go build output in actions/setup-go. |
| A2 | `xwb1989/sqlparser` is unsuitable because it's MySQL-flavored | Alternatives Considered | If sqlparser actually handles Postgres adequately, we lose CGO complexity for free. Verify if pg_query_go cost becomes painful. |
| A3 | `air` v1.65 requires Go 1.25 — devs on Go 1.23 must pin v1.61.x | Pitfall 8 | If true, this is a real issue. If air v1.65 actually works on Go 1.23 (just won't compile from source there), no action needed. Verify on first dev install. |
| A4 | `actions/setup-go@v5` cache is faster than no-cache | Pattern 5 | If cache hit rate is low for our project shape, the cache config adds churn for little gain. Measure after first 10 CI runs. |
| A5 | The user wants `_scaffold` table name with leading underscore | D-18 / Example 9 | If Postgres has issues with underscore-prefixed table names (it doesn't, but some tools do), rename. Verify during migration testing. |
| A6 | The slog `WarnContext` API exists in Go 1.21+ | Pattern 2 (orgdb preflight) | `slog.WarnContext` is in Go 1.23 stdlib (verified); if pinning to Go 1.21, may need to use `slog.Default().WarnContext`. |
| A7 | `chi.Router.Route()` middleware applies only inside the closure | Example 5 | This is correct per chi docs, but the planner should write a quick test to confirm `/healthz` doesn't run `OrgContext`. |
| A8 | The `migrate.NewWithDatabaseInstance` needs `*sql.DB`, not pgxpool | Pitfall 7 | Standard pattern, verified in golang-migrate docs. |
| A9 | UUID v7 sortability advantage matters for Postgres B-tree primary keys | State of the Art | True per RFC 9562 §5.7 and many blog write-ups; if v0.1 catalog volumes are small, the advantage is marginal but still real. |
| A10 | OTel SDK v1 API is stable (no breaking changes expected in v0.1 timeframe) | Standard Stack | OTel Go SDK is GA as of 2023; API surface is stable. |
| A11 | The `pg_query_go` parse tree is stable across versions | orgDB SQL inspection | The library version is v6; check release notes for breaking AST changes before upgrading. |
| A12 | "Latest version" timestamps shown by go.dev proxy can mislead slopcheck's age heuristic for established packages | Package Legitimacy Audit | True — go.dev proxy resolves on-demand and version dates can look fresh even for established packages. Manually verify any [SUS] flag against pkg.go.dev. |

## Open Questions

1. **Should `OTEL_EXPORTER=stdout` flush spans immediately (sync) or batch (default)?**
   - What we know: `stdouttrace.New()` returns a synchronous exporter by default; `BatchSpanProcessor` adds latency.
   - What's unclear: In dev, immediate flush is nicer for "I just hit an endpoint, where's the span?"; in CI, batch is fine.
   - Recommendation: Use `sdktrace.WithSyncer(exporter)` for stdout in dev (immediate), `sdktrace.WithBatcher(exporter)` for OTLP in prod. Document the toggle.

2. **What's the cmd/migrate signature exactly?**
   - What we know: It must use `orgDB.WithBypass(ctx, "schema_migration")` and apply all pending migrations.
   - What's unclear: Should it take CLI args (`up`, `down N`, `version`) like the `migrate` CLI? Or wrap a single "up to latest"?
   - Recommendation: For Phase 1, wrap "up to latest" only. The `migrate` CLI handles ad-hoc operations (`migrate-down`, `migrate-create`); `cmd/migrate` is for production deploys.

3. **How should the orgDB validator handle migration SQL (DDL)?**
   - What we know: D-04 says bypass is used by migrations. DDL statements (CREATE TABLE, ALTER TABLE) don't reference `org_id` at all.
   - What's unclear: Should bypass be the only way migrations work, or should the validator treat DDL specially?
   - Recommendation: Migrations always go through bypass (per D-11). The validator should only run for DML (SELECT/INSERT/UPDATE/DELETE). pg_query_go exposes the statement type via the AST root.

4. **Should `/metrics` actually expose anything in Phase 1?**
   - What we know: D-17 says "registers the OTel-derived metrics handler" as a placeholder.
   - What's unclear: OTel Go has a Prometheus exporter (`otel/exporters/prometheus`) but adding it now feels premature.
   - Recommendation: Register the route with a stub handler that returns 200 + empty body (or use chi's `middleware.Heartbeat` pattern). Phase 5 or later can wire real metrics.

5. **Sequential vs timestamp migration filenames?**
   - What we know: golang-migrate supports both. Phase 1 has one migration so the choice is symbolic.
   - What's unclear: Multi-developer scenarios benefit from timestamps; small teams from sequential.
   - Recommendation: Use sequential (`000001_...`) for v0.1 since this is a single team; switch to timestamps if Phase 3+ shows merge conflicts.

6. **Where should the `_scaffold` test fixture live in Phase 3 when we drop it?**
   - What we know: CONTEXT.md says Phase 3's first migration drops `_scaffold`.
   - What's unclear: Should the FOUND-08 isolation test be ported to use a real entity (e.g., `agents`) in Phase 3, or should it stay in `_scaffold` form until Phase 3 deletes both the table and the test?
   - Recommendation: Phase 3 ports the test to `agents` before dropping `_scaffold`. Document this in Phase 3's CONTEXT.md.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | services/api/ build + test | ✓ | 1.23.12 darwin/arm64 | — |
| Docker | testcontainers-go, docker-compose | ✓ | 29.4.0 | — |
| Node.js | pnpm workspace | ✓ | v22.22.2 | — |
| pnpm | web/ workspace manager | ✓ | 10.33.0 | — |
| Task (Taskfile runner) | task dev / task ci | ✗ | — | `brew install go-task` or `go install github.com/go-task/task/v3/cmd/task@latest` — required for `task dev` |
| `migrate` CLI | task migrate-up | ✗ | — | `brew install golang-migrate` or `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1` |
| `sqlc` CLI | task gen | ✓ | v1.31.1 | — |
| `air` | task dev hot-reload | ✗ | — | `go install github.com/air-verse/air@v1.61.7` (Go 1.23 compatible; do NOT use @latest which needs Go 1.25) |
| `golangci-lint` | task lint | ✓ | v1.63.4 | — |
| `slopcheck` | research/CI gate | ✓ (installed during research) | v0.6.1 | — |

**Missing dependencies with fallback:**
- Task, migrate CLI, air — all installable in a Wave 0 task (`task --version || go install ...`).

**No dependencies are blocking.**

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go's stdlib `testing` package + `github.com/stretchr/testify` for assertions |
| Config file | `services/api/.golangci.yml` (lint config); no separate test config |
| Quick run command | `cd services/api && go test -count=1 ./internal/...` |
| Full suite command | `cd services/api && go test -race ./...` |

For frontend stubs in Phase 1, no unit test framework needed yet (the `index.ts` files are empty stubs). Phase 6/7 will add Vitest. The CI `web-typecheck` and `web-lint` jobs serve as the validation surface for Phase 1's frontend slice.

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FOUND-01 | Monorepo structure | smoke (directory exists) | `ls services/api/go.mod web/pnpm-workspace.yaml openapi/ migrations/` (CI shell step) | ❌ Wave 0 |
| FOUND-02 | Postgres 17 schema with `org_id UUID NOT NULL` | integration | `go test -run TestSchema_HasOrgIdNotNull ./internal/scaffold -v` | ❌ Wave 0 |
| FOUND-03 | `X-Org-Id` header extraction + 400 on missing/malformed | unit + integration | `go test -run 'TestOrgContext.*' ./internal/middleware -v` and `go test -run 'TestHTTP_MissingHeader' ./internal/scaffold -v` | ❌ Wave 0 |
| FOUND-04 | orgDB enforces `org_id` filter on all queries | unit + integration | `go test -run 'TestOrgDB.*' ./internal/db -v` | ❌ Wave 0 |
| FOUND-05 | `org_id` propagates via `context.Context` only | unit | `go test -run 'TestContextPropagation' ./internal/scaffold -v` | ❌ Wave 0 |
| FOUND-06 | UNIQUE (org_id, external_id) constraint | integration | `go test -run 'TestScaffold_UniqueOrgExternalId' ./internal/scaffold -v` | ❌ Wave 0 |
| FOUND-07 | OTel SDK initializes before chi mux; spans + logs carry `org_id` | unit + integration | `go test -run 'TestTracingHandler_AddsTraceID\|TestOTelInit' ./internal/telemetry -v` | ❌ Wave 0 |
| FOUND-08 | Two-org isolation, full HTTP chain | integration | `go test -run 'TestTwoOrgsIsolation' ./internal/scaffold -v` | ❌ Wave 0 |
| FOUND-09 | GitHub Actions runs all required jobs | smoke (in CI) | the workflow file itself | ❌ Wave 0 |
| FOUND-10 | docker-compose brings up infra; task dev works | manual-only | `task dev` then `curl :8080/healthz` — manual smoke | ❌ Wave 0 (acceptance is a doc + manual step) |

### Sampling Rate

- **Per task commit:** `cd services/api && go test -count=1 ./internal/...` (~30 sec including testcontainers startup; subsequent runs faster with image cached)
- **Per wave merge:** `cd services/api && go test -race ./...` plus `task lint typecheck`
- **Phase gate:** Full suite green; FOUND-08 isolation test green; manual `task dev` smoke + `curl /healthz` + `curl /readyz` with `X-Org-Id` valid/invalid headers documented

### Two-Org Isolation Specification (FOUND-08, the phase gate)

The canonical test must cover **every** combination of:

| Vector | Cases |
|--------|-------|
| HTTP method | POST (create), GET (single), GET (list) |
| Org pairings | (orgA, orgB) — both UUIDv7, both non-empty |
| Header presence | (valid, missing, malformed UUID, UUIDv4) |
| External_id overlap | both orgs have `external_id="ext-1"` AND `external_id="ext-2"` |
| Visibility | each org sees only its own rows; ID lookups from one org against the other return 404 |
| Bypass paths | `/healthz`, `/readyz`, `/metrics` work without any header |

**Minimum test cases (sample size for proof):**

1. `TestTwoOrgsIsolation_ListsExcludeOtherOrg` — seed orgA with 3 rows, orgB with 2 rows, all overlapping external_ids; assert each list returns exactly its own rows.
2. `TestTwoOrgsIsolation_GetByIDIsScoped` — GET orgA's row from orgB context returns 404.
3. `TestTwoOrgsIsolation_PostRespectsHeaderOrg` — POST with `X-Org-Id: orgA` creates a row owned by orgA only.
4. `TestOrgContext_MissingHeader400` — POST/GET with no `X-Org-Id` returns 400.
5. `TestOrgContext_MalformedHeader400` — POST/GET with `X-Org-Id: not-a-uuid` returns 400.
6. `TestOrgContext_UUIDv4Rejected400` — POST/GET with a UUIDv4 returns 400 + `{"reason":"uuidv7_required"}`.
7. `TestBypassPaths_NoHeaderRequired` — GET `/healthz`, `/readyz`, `/metrics` succeed without `X-Org-Id` header.
8. `TestOrgDB_SQLWithoutOrgFilter_RejectedDev` — programmatic: a query like `SELECT * FROM _scaffold` (no WHERE org_id) is rejected by the orgDB validator (panic in test mode).
9. `TestOrgDB_BypassMarker_AllowsUnscoped` — `orgDB.WithBypass(ctx, "test")` allows the same query through and emits a slog WARN event.
10. `TestRequestID_IsUUIDv7` — assert the generated request_id is UUIDv7 (parse + Version() == 7).
11. `TestTracingHandler_LogsCarryTraceID` — fire a request, capture stdout, assert the log line contains `trace_id` and `span_id` fields matching the response trace.

**Why these 11 are sufficient:** They span the full HTTP chain (FOUND-03 → middleware), the storage layer (FOUND-04 → orgDB), the schema constraint (FOUND-06 → UNIQUE), the observability layer (FOUND-07 → trace_id injection), and the bypass mechanism (D-04, D-21). Adding more test cases beyond these would be combinatorial coverage that doesn't surface new failure modes.

### Wave 0 Gaps

- [ ] `services/api/internal/scaffold/integration_test.go` — TestMain + TestTwoOrgsIsolation (covers FOUND-02, FOUND-06, FOUND-08)
- [ ] `services/api/internal/middleware/orgcontext_test.go` — header parsing + UUIDv7 validation (covers FOUND-03, FOUND-05)
- [ ] `services/api/internal/db/orgdb_test.go` — SQL validation + bypass (covers FOUND-04)
- [ ] `services/api/internal/telemetry/slog_test.go` — trace_id injection (covers FOUND-07)
- [ ] Shared test helpers: `internal/testsupport/postgres.go` (TestMain helpers), `internal/testsupport/httpclient.go` (httptest.NewServer wrapper)
- [ ] Framework install: `go install github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1` (CI) + `brew install golang-migrate` (local dev) + `go install github.com/go-task/task/v3/cmd/task@latest`
- [ ] CI gating: golangci-lint config (`.golangci.yml`) and the workflow file itself

## Security Domain

`security_enforcement` is not explicitly set to false in `.planning/config.json`, so this section is included. Phase 1 is foundation work; the security concerns it cements affect every later phase.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture | yes | Multi-org isolation pattern documented and enforced at orgDB layer (defense-in-depth: middleware + DB constraint + UNIQUE composite) |
| V2 Authentication | no — Phase 1 is stub auth | `X-Org-Id` header is trusted in v0.1 per PROJECT.md; real auth deferred to AUTH-01 in v1 |
| V3 Session Management | no | No sessions in v0.1 (stateless REST) |
| V4 Access Control | partial | Org-scoping at orgDB layer enforces ABAC — every query is scoped by org_id. Cross-org reads/writes are rejected at the SQL inspection layer. The `WithBypass` escape is sysadmin-only and logged. |
| V5 Input Validation | yes | `X-Org-Id` header validated as UUIDv7+ (D-20); invalid input → HTTP 400 with structured error body. Postgres uses parameterized queries via pgx (SQL injection impossible by construction). |
| V6 Cryptography | no | No crypto operations in Phase 1 (no password hashing, no session tokens) |
| V7 Error Handling | yes | Structured JSON errors `{"error":"code","reason":"detail"}`; no stack traces leak to clients; `slog` captures internal errors at WARN/ERROR; panic recovery middleware returns 500 with empty body |
| V8 Data Protection | yes | `org_id` is NOT NULL; UNIQUE (org_id, external_id) prevents cross-org collisions; no PII stored in Phase 1 (`_scaffold` is throwaway) |
| V9 Communication | partial | TLS terminated by reverse proxy in prod (not Phase 1 concern); local dev uses plain HTTP; production deployment guide (deferred) must document TLS |
| V12 Configuration | yes | All secrets/config via env (12-factor); no hardcoded credentials; `.env` excluded from git via `.gitignore` |

### Known Threat Patterns for {Go + chi + pgx + Postgres}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection | Tampering | pgx parameterized queries ($1, $2, …); sqlc generates only parameterized SQL |
| Cross-org data leakage (BOLA / IDOR) | Information Disclosure | orgDB wrapper rejects un-scoped queries; SQL inspection via pg_query_go ensures `org_id` filter present; UNIQUE composite constraint prevents external_id collisions across orgs |
| Bypass mechanism abuse | Elevation of Privilege | `orgDB.WithBypass` requires a typed marker; only `cmd/migrate` legitimately uses it in Phase 1; every bypass emits `slog.Warn` with `event=orgdb_bypass`, `caller`, `reason`, `sql_hash` for forensics |
| Cache poisoning across orgs | Information Disclosure | Phase 1 doesn't cache results yet (Redis used only for `/readyz`); cache key convention `or:{orgId}:{entity}:{id}` codified before Phase 3 introduces caching (CAT-11) |
| Trusting `X-Org-Id` from client | Spoofing | Phase 1 explicitly accepts the header as authoritative; documented as stub auth; AUTH-01 in v1 replaces with JWT claim extraction at the same middleware position |
| Log injection via `X-Org-Id` value | Tampering | `uuid.Parse` rejects non-UUID input → no arbitrary strings reach the logger; trace_id, span_id, request_id are all server-generated UUIDv7 |
| DoS via slow integrations / connection exhaustion | DoS | `pgxpool.Pool` has `MaxConns` (set in config); chi `Timeout` middleware (deferred, not Phase 1) gives request-level cancellation |
| Migration replay / drift | Tampering | golang-migrate writes to `schema_migrations` table with version + dirty flag; `/readyz` reads this table and degrades if `dirty=true` |
| Postgres NOT NULL bypass via app | Tampering | `org_id UUID NOT NULL` in migration; pgx delegates to Postgres for constraint enforcement; INSERTs without `org_id` fail at DB layer regardless of app bugs |

## Sources

### Primary (HIGH confidence)

- [pgx v5.9.2 pgxpool docs](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool) — DBTX interface signatures, Pool.Ping for /readyz
- [pgx v5 main package](https://pkg.go.dev/github.com/jackc/pgx/v5) — Native query API
- [sqlc 1.31.1 official docs](https://docs.sqlc.dev/en/latest/guides/using-go-and-pgx.html) — sql_package: pgx/v5 generation, DBTX interface
- [sqlc transactions docs](https://docs.sqlc.dev/en/latest/howto/transactions.html) — Queries.WithTx pattern, DBTX duck-typing
- [chi v5.2.3 release](https://github.com/go-chi/chi/tree/v5.2.3) — Latest stable
- [chi middleware reference](https://pkg.go.dev/github.com/go-chi/chi/v5/middleware) — Recoverer, RequestID, ordering
- [testcontainers-go Postgres module](https://golang.testcontainers.org/modules/postgres/) — postgres.Run signature, ConnectionString, wait strategies
- [google/uuid v1.6.0 docs](https://pkg.go.dev/github.com/google/uuid) — NewV7, Parse, Version
- [RFC 9562 — UUID Formats](https://www.rfc-editor.org/rfc/rfc9562.html) — UUIDv7 spec §5.7
- [golang-migrate v4.19.1 docs](https://github.com/golang-migrate/migrate) — Filename convention, programmatic API
- [golang-migrate MIGRATIONS.md](https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md) — Version sorting rules
- [golang-migrate Postgres driver](https://github.com/golang-migrate/migrate/blob/master/database/postgres/README.md) — schema_migrations table, x-migrations-table config
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/) — Tracer provider, exporter selection
- [otelhttp package](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp) — NewHandler signature, WithTracerProvider
- [otlptracehttp exporter](https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp) — OTLP HTTP exporter
- [Taskfile v3.50 docs](https://taskfile.dev/usage/) — Targets, deps, env, includes
- [air-verse/air](https://github.com/air-verse/air) — TOML config, Go version requirement
- [pganalyze/pg_query_go](https://github.com/pganalyze/pg_query_go) — CGO requirement, parse-tree API
- [Docker Compose healthcheck](https://docs.docker.com/compose/how-tos/startup-order/) — depends_on + service_healthy condition
- [PROJECT.md, REQUIREMENTS.md, ROADMAP.md, STATE.md, 01-CONTEXT.md] — locked decisions and acceptance criteria

### Secondary (MEDIUM confidence)

- [Better GitHub Actions caching for Go (Dan Peterson)](https://danp.net/posts/github-actions-go-cache/) — setup-go cache key shape
- [GitHub Actions PostgreSQL service containers](https://docs.github.com/en/actions/using-containerized-services/creating-postgresql-service-containers) — Service container option
- [Adrian Brad — Parallel Go tests for Postgres](https://adrianbrad.medium.com/parallel-postgresql-tests-go-docker-6fb51c016796) — TestMain + parallel pattern (reviewed, not relied on)
- [Uptrace OpenTelemetry net/http guide](https://uptrace.dev/guides/opentelemetry-net-http) — otelhttp.NewHandler usage with chi
- [Otelslog bridge package](https://pkg.go.dev/go.opentelemetry.io/contrib/bridges/otelslog) — Used to confirm it REPLACES handler (informed our choice to roll a wrapper)

### Tertiary (LOW confidence)

- [Otelchi (riandyrn)](https://github.com/riandyrn/otelchi) — Alternative considered; rejected for Phase 1 surface area
- General blog posts on UUIDv7 sortability (akrabat.com etc.) — Confirmation, not authoritative

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every package verified on go.dev/npm registry and via slopcheck v0.6.1
- Architecture patterns: HIGH — every pattern has Go-idiomatic precedent and is documented in CONTEXT.md
- orgDB SQL inspection: HIGH on approach (pg_query_go), MEDIUM on edge cases (CTEs, subqueries — Phase 1 sqlc queries are all flat, deferred handling to Phase 3 if needed)
- testcontainers-go integration: HIGH — well-documented, used in similar projects; Pitfall 4 (cold pull timeout) is the main risk
- OTel + slog wiring: HIGH on architecture, MEDIUM on exact attribute names (semantic conventions evolve; pinning `semconv/v1.26.0` is conservative)
- CI workflow: HIGH — actions/setup-go@v5 with cache + concurrency cancel is industry-standard
- Pitfalls: HIGH — all pitfalls cited from PITFALLS.md or verified during research

**Research date:** 2026-05-15
**Valid until:** 2026-06-15 (30 days for stable infra stack)
