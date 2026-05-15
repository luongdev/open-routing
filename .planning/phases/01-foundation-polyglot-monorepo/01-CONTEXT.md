# Phase 1: Foundation & Polyglot Monorepo - Context

**Gathered:** 2026-05-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Bootstrap a polyglot Go + pnpm monorepo with org-isolation infrastructure that is provably correct via a two-org integration test before any catalog entity, OpenAPI contract, or domain logic exists. This phase delivers the scaffold every later phase builds on — once shipped, the orgDB enforcement, `X-Org-Id` middleware, OTel pipeline, golang-migrate workflow, Taskfile orchestrator, and pnpm workspace skeleton are frozen contracts for Phases 2–7.

**In scope:** repo layout (`services/api/`, `web/`, `openapi/`, `migrations/`), Go skeleton (chi + pgx + sqlc + slog + OTel), orgDB wrapper, PostgreSQL 17 + Redis via Docker Compose, golang-migrate CLI, pnpm workspace stub, Taskfile orchestrator, GitHub Actions CI, `/healthz` + `/readyz` + `_scaffold` test resource, two-org isolation suite (HTTP-level via httptest + testcontainers-go).

**Out of scope:** any of the 6 catalog entities (Phase 3), OpenAPI spec content beyond a placeholder file (Phase 2), agent state machine (Phase 4), bulk import (Phase 5), admin UI components (Phase 6), embed bundle (Phase 7), real auth/RBAC, audit-log infrastructure beyond slog WARN events, Redis usage beyond healthcheck.

</domain>

<decisions>
## Implementation Decisions

### orgDB Pattern + sqlc Integration

- **D-01:** `orgDB` is a wrapper around `*pgx.Pool` implementing the `pgx.DBTX` interface; sqlc-generated `*Queries` receives an `orgDB` instance (not the raw pool). Handlers physically cannot construct sqlc `Queries` against the raw pool — only against `orgDB`.
- **D-02:** Enforcement model is **validate + reject**. On every `Exec/Query/QueryRow` call, the wrapper inspects the SQL string (parsed once per unique string, cached by SHA-256 hash) and rejects any statement that lacks an `org_id = $N` clause in WHERE / INSERT column list / UPDATE / DELETE. Validation failures panic in dev/test and return an error in prod. No runtime SQL rewriting — sqlc query files MUST author the filter explicitly.
- **D-03:** The wrapper also asserts that `ctx` carries an `org_id` (UUIDv7) before delegating. Missing org_id → typed error.
- **D-04:** **Cross-org escape hatch:** `orgDB.WithBypass(ctx, reason string) context.Context` returns a context with a typed marker. The validator detects the marker and skips the org_id check. In Phase 1 the **only** legitimate bypass caller is the migrations runner (`cmd/migrate`); every other surface must use the strict path.
- **D-05:** Bypass events emit `slog.Warn` with structured fields `event=orgdb_bypass`, `caller`, `reason`, `sql_hash`, `org_id_attempted`. No DB-backed audit table in v0.1 — when v1 audit infrastructure lands, the same slog event becomes the source for a structured audit row with zero code change.

### Two-Org Isolation Proof Harness

- **D-06:** Postgres provisioning for integration tests uses **testcontainers-go** (ephemeral container per suite). Local `go test ./...` and GitHub Actions CI follow the same code path — no environment divergence. The migration set runs against the container on startup via golang-migrate.
- **D-07:** The FOUND-08 isolation test exercises the **full HTTP chain** via `httptest.NewServer` (or direct `chi.Mux.ServeHTTP`). Real HTTP requests carry `X-Org-Id: <uuidv7-A>` / `<uuidv7-B>`. The test seeds two orgs with identical `external_id` values in the `_scaffold` table, drives create/read/list through real chi routes, and asserts each org sees only its own rows. This single suite covers FOUND-02 (schema), FOUND-03 (header extraction), FOUND-04 (orgDB enforcement), FOUND-05 (ctx propagation), FOUND-06 (UNIQUE constraint), and FOUND-08 (zero leakage).

### Monorepo Orchestration + Migrations

- **D-08:** Top-level orchestrator is **Taskfile** (`Taskfile.yml` at repo root). Targets: `task test`, `task lint`, `task gen`, `task migrate-up`, `task migrate-down`, `task migrate-create NAME=...`, `task dev`, `task build`, `task ci`. Frontend tasks delegate to `pnpm -F <pkg> ...`.
- **D-09:** **Turborepo lives inside `web/`** (`web/turbo.json`) and owns caching/parallelism for the pnpm workspace only. Go side never touches Turborepo. Two tools, each owning its native ecosystem.
- **D-10:** Migrations live at **`/migrations/`** at repo root (matches FOUND-01). Filenames follow golang-migrate convention: `NNNNNN_name.up.sql` / `NNNNNN_name.down.sql`. Phase 1 ships a single migration creating the `_scaffold` table.
- **D-11:** Migrations are invoked via `task migrate-up` (which calls the `migrate` CLI binary). **API never auto-runs migrations** on startup — prevents multi-replica race in v1, makes schema drift visible. Migration runner is a separate Go `cmd/migrate` binary (or invokes the `migrate` CLI directly via Taskfile) and is the **only** legitimate user of `orgDB.WithBypass`.

### Frontend (pnpm) Stub Scope

- **D-12:** Phase 1 ships **minimal pnpm scaffold** sufficient to make CI typecheck/lint meaningful. Files created:
  - `web/pnpm-workspace.yaml`
  - `web/package.json` (root pnpm workspace)
  - `web/tsconfig.base.json` (shared strict TS config)
  - `web/apps/admin/{package.json, tsconfig.json, src/index.ts}` — `index.ts` exports nothing
  - `web/apps/embed/{package.json, tsconfig.json, src/index.ts}` — `index.ts` exports nothing
  - `web/packages/ui/{package.json, tsconfig.json, src/index.ts}` — `index.ts` exports nothing
  - `web/turbo.json` with `typecheck` / `lint` pipeline definitions
- **D-13:** No Vite config, no Lit components, no Shoelace integration in Phase 1. CI runs `pnpm typecheck` and `pnpm lint` against the scaffolded packages, proving FOUND-09 frontend jobs work. Phase 6 adds real Vite/Lit/Shoelace tooling to `admin` and `packages/ui`; Phase 7 fills in `embed`.

### OTel + Scaffold Surface

- **D-14:** OTel SDK initializes **before** chi router setup (FOUND-07 hard rule). Exporter target driven by `OTEL_EXPORTER` env var:
  - Dev + CI default → **stdout exporter** (`stdouttrace.New`). Spans flush to logs, zero infrastructure.
  - Prod → OTLP HTTP exporter (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`).
- **D-15:** No Jaeger / no OTel collector in docker-compose. Catalog CRUD spans are not yet interesting enough to justify the container.
- **D-16:** Auto-instrumentation via `otelhttp.NewHandler` wrapping the chi mux at the root. The org-context middleware adds `org_id` as a span attribute via `trace.SpanFromContext(ctx).SetAttributes(attribute.String("org_id", id.String()))`.
- **D-17:** Phase 1 ships these HTTP routes:
  - `GET /healthz` — always returns HTTP 200 with `{"status":"alive"}`. Bypasses org middleware. Liveness probe only.
  - `GET /readyz` — deep readiness check. Pings `pgx.Pool`, pings Redis, queries `schema_migrations` for current version. Body shape: `{"status":"ok"|"degraded","checks":{"db":"ok","redis":"ok","migrations":{"version":N}}}`. Returns HTTP 503 on any failed check. Bypasses org middleware.
  - `GET /metrics` — Prometheus scrape endpoint (placeholder — registers the OTel-derived metrics handler). Bypasses org middleware.
  - `POST /v1/orgs/{org_id}/_scaffold` — create scaffold row (`external_id`, `name`). Org middleware enforced.
  - `GET /v1/orgs/{org_id}/_scaffold` — list scaffold rows. Org middleware enforced.
  - `GET /v1/orgs/{org_id}/_scaffold/{id}` — single scaffold row. Org middleware enforced.
- **D-18:** `_scaffold` table schema:
  ```sql
  CREATE TABLE _scaffold (
    id          UUID PRIMARY KEY,             -- UUIDv7+ enforced at app layer
    org_id      UUID NOT NULL,
    external_id TEXT NOT NULL,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, external_id)
  );
  ```
  Phase 3 drops the `_scaffold` table and routes when real catalog entities land. The migration set will include both the up migration in Phase 1 and the down migration in Phase 3's first migration file.

### org_id Validation + ID Convention (Project-Wide)

- **D-19:** **All IDs in the project (org_id, entity primary keys, foreign keys, request body IDs) MUST be UUIDv7 or higher.** UUIDv4 is rejected. Time-ordered variants (v7, v8) are accepted; the constraint is enforced both at the middleware (for `X-Org-Id`) and inside `orgDB` when emitting new IDs (use `uuid.NewV7()` from `github.com/google/uuid` v1.6+).
- **D-20:** The `X-Org-Id` header is parsed with `uuid.Parse`, then `parsed.Version()` is checked: anything `< 7` returns HTTP 400 with body `{"error":"invalid_org_id","reason":"uuidv7_required"}`. Org_id stored in ctx as `uuid.UUID` (not `string`) so handlers get type safety.
- **D-21:** Middleware bypass list: `/healthz`, `/readyz`, `/metrics`. Every other route is gated by the org middleware. `/openapi.yaml` is **not** served from the API in Phase 1 (Phase 2 decides if it's served and from where).

### Go Module + Repo Layout

- **D-22:** Single Go module at **`services/api/go.mod`** (matches FOUND-01). Module path: `github.com/luongdev/open-routing/services/api`. No root `go.mod`, no `go.work`. Future `services/runtime/` (v1) will get its own `go.mod` and `go.work` will be introduced at that point.
- **D-23:** Internal package layout under `services/api/internal/`:
  - `internal/server/` — chi mux setup, middleware chain wiring, route registration
  - `internal/middleware/` — `OrgContext`, `RequestID`, `Recover`, OTel/slog adapters
  - `internal/db/` — `orgDB` wrapper, pgx pool init, SQL validator
  - `internal/db/queries/` — sqlc `.sql` source files
  - `internal/db/generated/` — sqlc output (committed)
  - `internal/scaffold/` — `_scaffold` handler + service + sqlc queries (deleted in Phase 3)
  - `internal/telemetry/` — OTel + slog initialization
  - `internal/config/` — env-var loader (12-factor)
  - `cmd/api/` — `main.go` for API binary
  - `cmd/migrate/` — `main.go` for the migration runner that uses `orgDB.WithBypass`

### Dev Experience

- **D-24:** `task dev` flow:
  1. `docker compose up -d postgres redis` (infra only, detached)
  2. Wait until `postgres` and `redis` healthchecks pass
  3. Optional reminder to run `task migrate-up` if migrations are pending
  4. Foreground `air` for Go hot-reload against `services/api`
- **D-25:** Docker Compose in Phase 1 contains exactly: `postgres:17`, `redis:7-alpine`. No `api` service (devs run native via `air` for speed). Phase 6/7 may add `web-admin` / `web-embed` Vite services if useful.
- **D-26:** Postgres + Redis ship with native docker-compose healthchecks (`pg_isready`, `redis-cli ping`). API connection pool retries with backoff until healthy.

### Logging + Request Correlation

- **D-27:** slog uses **JSON format everywhere** (dev, CI, prod). Devs filter logs with `jq` or VS Code log highlighters. No `tint` / human-readable handler — keeps log shape consistent across environments and easy for grep/jq.
- **D-28:** A `RequestID` middleware sits second in the chain (after `Recover`). It generates a **UUIDv7** request_id, injects it into `context.Context`, and writes it back to the response as the `X-Request-Id` header.
- **D-29:** A custom `slog.Handler` wrapper reads the active OTel span from `context.Context` on every log emission and injects `trace_id` and `span_id` as log fields. Every log line carries: `time`, `level`, `msg`, `org_id` (when present), `request_id`, `trace_id`, `span_id`, plus the caller's structured attributes.

### CI

- **D-30:** GitHub Actions workflow at `.github/workflows/ci.yml` runs on every push and pull request. Jobs:
  - `go-vet` — `go vet ./...`
  - `go-lint` — `golangci-lint run` (config at `services/api/.golangci.yml`)
  - `go-test` — `go test -race ./...` (includes testcontainers-go integration suite)
  - `web-typecheck` — `pnpm -F '*' typecheck`
  - `web-lint` — `pnpm -F '*' lint`
  - `migration-drift` — `task migrate-up` against an ephemeral Postgres then `migrate version` sanity check
- **D-31:** All jobs are required for merge. Concurrency cancels in-flight runs for the same branch. No test coverage threshold gate in Phase 1 — defer until catalog CRUD lands in Phase 3.

### Claude's Discretion

- Specific `golangci-lint` linter set — start with `default` + `errcheck`, `govet`, `staticcheck`, `gosec`, `gocritic`; tune in code-review.
- Exact `air.toml` configuration for hot-reload.
- ESLint config flavor for the pnpm scaffold (use `@typescript-eslint/recommended-strict` as a sane starting point).
- Specific Postgres + Redis image tags and resource limits in docker-compose.
- The Taskfile target list and dependencies graph beyond the named targets above.
- Whether to ship a `.editorconfig` and `.gitattributes` in Phase 1 (recommend yes).
- The `slog.Handler` library choice (stdlib `slog.NewJSONHandler` is sufficient; no third-party handler required other than the trace_id wrapper).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level locks
- `.planning/PROJECT.md` — Locked stack (Go + chi + sqlc + pgx + golang-migrate + slog, PostgreSQL 17 + Redis, OpenAPI 3.1 + oapi-codegen + openapi-typescript, Vite + Lit + Shoelace, Web Components, polyglot monorepo). Naming lock: `org_id` and "org" (never `tenant_id` / "tenant"). Constraints: REST polling only, no WebSocket/SSE; no browser workers; no XState; app-layer orgDB primary defense; no PostgreSQL RLS in v0.1.
- `.planning/REQUIREMENTS.md` §Foundation & Multi-Org Isolation — Requirements **FOUND-01 through FOUND-10**. The acceptance criteria for this phase.
- `.planning/ROADMAP.md` §Phase 1 — Goal statement and 5 success criteria (docker-compose up, X-Org-Id contract, two-org isolation suite, OTel attribute carrying, GitHub Actions gates).
- `.planning/STATE.md` §Accumulated Context — Locked stack decisions plus pre-existing blockers/concerns from the research synthesis.

### Research (translation note: stack pivoted to Go; framework specifics superseded, patterns and pitfalls retained)
- `.planning/research/SUMMARY.md` — Domain findings are authoritative; stack/framework recommendations (Fastify, Drizzle, React, MF) are **superseded** by the Go + Lit + Web Components pivot. Use the multi-org enforcement stack diagram + bulk import patterns + agent state patterns as the **pattern source** — translate `OrgScopedDb` → `orgDB`, `AsyncLocalStorage` → `context.Context`, `Fastify middleware` → `chi middleware`.
- `.planning/research/PITFALLS.md` §1 (Multi-Org Isolation Pitfalls) — Sections 1.1 through 1.5 are all in Phase 1's scope. Required reading for the planner before writing the orgDB wrapper, middleware, and isolation test.
  - §1.1 Cache key omits `org_id` — informs Redis key construction even though Phase 1 only uses Redis for healthcheck.
  - §1.2 Background job missing org context — informs the migration runner's bypass mechanism design.
  - §1.3 Bulk import cross-org rows — informs the `org_id NOT NULL` schema constraint that lands in the `_scaffold` migration.
  - §1.4 Admin / management endpoints leak — informs the middleware-at-root-vs-per-route decision (must be at root; `_scaffold` proves this).
  - §1.5 Test fixtures bleed between orgs — informs the per-test fresh UUIDv7 org_id pattern.
- `.planning/research/ARCHITECTURE.md` §1 (Layered Architecture and Module Structure) and §2 (Multi-Org Data Isolation) — Pattern reference. **Note:** translate the Node/Hono layout to the Go layout in D-23. The three-layer isolation diagram in §2 is conceptually correct but in v0.1 we ship layers 1 and 2 only (HTTP middleware + orgDB), with RLS deferred per PROJECT.md.
- `.planning/research/STACK.md` — **Superseded** for tech choices. Skip during Phase 1 planning unless cross-checking historical context.
- `.planning/research/FEATURES.md` — Out of scope for Phase 1 (catalog entity field shapes are Phase 3 concerns); ignore.

### External standards
- [RFC 9562 — UUID Formats](https://www.rfc-editor.org/rfc/rfc9562.html) §5.7 (UUIDv7) — Time-ordered UUID spec for D-19.
- [golang-migrate documentation](https://github.com/golang-migrate/migrate) — Filename convention and CLI usage for D-10/D-11.
- [sqlc documentation](https://docs.sqlc.dev/) — Query annotation conventions and DBTX interface for D-01/D-02.
- [pgx v5 documentation](https://pkg.go.dev/github.com/jackc/pgx/v5) — Pool API and `DBTX` interface signature.
- [chi router documentation](https://go-chi.io/) — Middleware composition and routing patterns.
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/) — `otelhttp.NewHandler`, span attribute API for D-16.
- [testcontainers-go Postgres module](https://golang.testcontainers.org/modules/postgres/) — Container lifecycle and Postgres image setup for D-06.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
**None — this is a greenfield phase.** The repository contains only planning documents (`.planning/`, `.git/`, agent/IDE config dirs). No Go code, no TypeScript code, no migrations, no docker-compose. Phase 1 creates every implementation artifact from scratch.

### Established Patterns
**None in the codebase.** The patterns this phase establishes (orgDB wrapper, middleware chain order, structured slog, Taskfile target layout, testcontainers-go suite structure) become the conventions every subsequent phase follows.

### Integration Points
- **Phase 2 (OpenAPI Contract & Codegen)** integrates by adding `openapi/openapi.yaml`, wiring `oapi-codegen` against the chi handler interfaces, and replacing the hand-written `_scaffold` handlers with generated stubs (or leaving `_scaffold` untouched until Phase 3 deletes it).
- **Phase 3 (Catalog CRUD)** integrates by adding the 6 entity tables, sqlc queries, and chi handlers, then deleting the `_scaffold` table and routes. Phase 3's first migration is the `_scaffold` drop.
- **Phase 4 (Agent State Machine)** integrates by adding `agent_states` migration and `internal/domain/agentstate/` package alongside Phase 3's catalog packages.
- **Phase 6 (Shared UI Library & Standalone Admin)** integrates by filling in the `web/packages/ui/` and `web/apps/admin/` packages scaffolded here.
- **Phase 7 (Web Component Embed Bundle)** integrates by filling in `web/apps/embed/`.

</code_context>

<specifics>
## Specific Ideas

- **UUIDv7+ everywhere** — applies to every ID in the system (org_id, entity PKs, FKs, request_id). This is a **project-wide constraint**, not just Phase 1. Downstream phases MUST use `uuid.NewV7()` for ID generation and validate any externally-supplied ID has `Version() >= 7`.
- **Bypass mechanism is sysadmin-aware** — user explicitly flagged the need for a sysadmin/cross-org escape hatch despite v0.1 having stub auth only. `orgDB.WithBypass(ctx, reason)` is the contract; Phase 1 uses it only in `cmd/migrate`; v1 RBAC will gate it properly.
- **Throwaway `_scaffold` is acceptable** — Phase 1 owns the table/routes; Phase 3 deletes them. No reuse anxiety — they exist purely to make the FOUND-08 isolation test exercisable through the full HTTP chain.
- **No Jaeger in dev** — devs read OTel via stdout exporter in `docker compose logs` (or `air` foreground). Keeps the dev infra minimal.
- **JSON logging in dev too** — user explicitly chose JSON-everywhere over the tint/text-pretty hybrid. Devs grep/jq for filtering. Trade-off accepted: slightly harder to skim, but log shape consistency between dev and prod is the priority.
- **`task migrate-up` is manual** — even though docker-compose has healthchecks gating service startup, migrations are a separate step. Devs run `task migrate-up` before `task dev` on initial setup, and again whenever a new migration file appears. No auto-migration init container — keeps migrations explicit and prevents multi-replica races in v1.

</specifics>

<deferred>
## Deferred Ideas

- **Real audit log table** — Bypass events currently land in slog only. v1 (with runtime engine + audit-log infrastructure) will add a `audit_events` table and rewire the slog WARN handler to also persist to DB.
- **Jaeger / OTel collector in docker-compose** — Defer until catalog spans are interesting (likely Phase 4 or Phase 5 when state machine + import latency matter).
- **Distributed lock for migrations** — `golang-migrate` advisory-lock guard is needed for v1 multi-replica deployments. Out of scope for v0.1 single-replica.
- **`/openapi.yaml` served from API** — Phase 2 will decide where the spec is hosted (API endpoint vs static file vs separate docs site).
- **Coverage threshold gate** — CI runs tests but doesn't enforce a coverage minimum. Defer until Phase 3 has real catalog test surface area.
- **Prometheus scrape config** — `/metrics` endpoint is registered but no actual metrics exposition is configured. Defer scrape setup to a deployment-focused phase.
- **`go.work` multi-module workspace** — Single module sufficient for v0.1. Introduce when `services/runtime/` lands in v1.
- **Vite dev servers in docker-compose** — Phase 6/7 may add `web-admin` / `web-embed` services if useful.
- **`.editorconfig` / `.gitattributes`** — Standard hygiene files; planner may include in Phase 1 or defer.
- **Web Worker for CSV preview** (raised in research) — Out of scope per PROJECT.md `Browser workers deferred to v0.2`.

</deferred>

---

*Phase: 1-Foundation & Polyglot Monorepo*
*Context gathered: 2026-05-15*
