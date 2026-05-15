# Phase 1: Foundation & Polyglot Monorepo - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-15
**Phase:** 1-Foundation & Polyglot Monorepo
**Areas discussed:** orgDB + sqlc pattern, Two-org isolation proof harness, Monorepo orchestration + migrations, OTel + scaffold surface, Go module + repo layout, Local dev experience + hot-reload, slog format + request correlation, /readyz semantics + healthchecks

---

## orgDB + sqlc pattern

### Q1: How should orgDB enforce org_id on every sqlc query?

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit `$org_id` param + wrapper | Every sqlc query file declares `org_id = $1` as a required parameter. orgDB is a thin Go struct wrapping `*Queries`; each method reads org_id from ctx and passes it as the first arg. Compile-time safe, idiomatic with sqlc. | |
| DBTX-level interceptor | Custom `DBTX` interface wraps pgx.Pool and validates/enforces SQL contains `WHERE org_id = $N` at every Exec/Query call. Catches misses automatically. | ✓ |
| Hybrid: param convention + runtime validator | Param convention plus a startup-time reflection pass that scans every sqlc-generated method. Belt-and-suspenders. | |

**User's choice:** DBTX-level interceptor
**Notes:** User wants automatic enforcement at the data-access layer (not just convention).

### Q2: How should the DBTX interceptor actually enforce org_id?

| Option | Description | Selected |
|--------|-------------|----------|
| Validate + reject | Wrapper inspects SQL once per unique string (cached), rejects any statement missing `org_id = $N` in WHERE/INSERT/UPDATE/DELETE. Asserts ctx carries org_id. No runtime SQL rewriting. | ✓ |
| Static codegen check + thin runtime wrapper | Build-time hook scans every `.sql` file and fails build if any query lacks `org_id = $1`. Runtime wrapper minimal. | |
| Runtime SQL injection (parse + rewrite) | Wrapper actively parses SQL AST with pg_query_go, injects `WHERE org_id = $N` if absent, mutates args. Most magical. | |

**User's choice:** Validate + reject (Option 1)
**Notes:** User added important caveat in Vietnamese: "lưu ý case sysadmin có thể xem của mọi tenant" — noted the need for a sysadmin/cross-org escape hatch. Followed up with Q3 to handle this.

### Q3: How to handle legitimate cross-org access (sysadmin, system jobs, migrations)?

| Option | Description | Selected |
|--------|-------------|----------|
| Separate SystemDB type, no Phase 1 endpoints | Phase 1 ships a second distinct wrapper `SystemDB` that bypasses org enforcement. Handlers physically cannot receive it; only `cmd/migrate` etc. construct it. | |
| Context bypass flag with audit log | `orgDB.WithBypass(ctx, reason)` sets a typed marker; validator skips check; writes audit log. More flexible. | ✓ |
| Defer entirely — strict-only in Phase 1 | No bypass paths in v0.1. Migrations run as separate process with raw pgx. | |

**User's choice:** Context bypass flag with audit log
**Notes:** Single orgDB type with explicit bypass marker — Phase 1's only legitimate caller is migrations.

### Q4: Where does the bypass audit record land in v0.1?

| Option | Description | Selected |
|--------|-------------|----------|
| Structured slog WARN only | `slog.Warn` with `event=orgdb_bypass`, `caller`, `reason`, `sql_hash`. No DB table. When v1 audit infra lands, same event becomes audit row. | ✓ |
| Minimal `bypass_events` table now | Add a tiny migration in Phase 1 for `bypass_events(id, ts, caller, reason, sql_hash)`. Durable, queryable. | |
| Both — slog + table | Belt-and-suspenders. | |

**User's choice:** Structured slog WARN only
**Notes:** Stays inside REQUIREMENTS Out of Scope `Audit logging` constraint while still capturing the event.

---

## Two-org isolation proof harness

### Q5: How is Postgres provisioned for the FOUND-08 isolation test?

| Option | Description | Selected |
|--------|-------------|----------|
| testcontainers-go ephemeral | Each integration test suite spins up a fresh Postgres 17 container. Local + CI same path. | ✓ |
| GHA `services: postgres` + local docker-compose | CI uses GHA services block; locally devs hit the same docker-compose Postgres. Diverging paths. | |
| Hybrid — testcontainers locally, GHA service in CI | Best of both, double maintenance. | |
| MCP k3s (user proposal) | User initially proposed using an MCP k3s setup. Clarified in follow-up that this is a deployment-time concern, not test-time. | |

**User's choice:** testcontainers-go ephemeral
**Notes:** User first responded in Vietnamese about MCP k3s for deployment; after clarification on the scope (test infra vs deployment), settled on testcontainers-go.

### Q6: What layer does the FOUND-08 two-org isolation test exercise?

| Option | Description | Selected |
|--------|-------------|----------|
| Full HTTP chain via httptest | chi router invoked via httptest.NewServer; real HTTP requests with `X-Org-Id`. Catches middleware + handler + service + orgDB enforcement in one suite. | ✓ |
| orgDB-level only (skip HTTP) | Direct orgDB calls with overlapping `external_id`s. Faster, but misses middleware bypass risk. | |
| Both layers in separate suites | DB-level unit + HTTP-level integration. More test code. | |

**User's choice:** Full HTTP chain via httptest
**Notes:** Aligns with PITFALLS §1.4 (admin/management endpoints leak risk is at the middleware layer, not orgDB).

---

## Monorepo orchestration + migrations

### Q7: What is the top-level task orchestrator?

| Option | Description | Selected |
|--------|-------------|----------|
| Taskfile at root + Turborepo in web/ | Taskfile.yml at root for Go + pnpm tasks; web/turbo.json owns pnpm caching. Two tools, each owning its native ecosystem. | ✓ |
| Make at root + pnpm scripts in web/ | Single Makefile, no Turborepo. Simpler tooling, slower frontend CI. | |
| Turborepo + Go-as-package shim | Single turbo.json with Go wrapped as pnpm script. Fights Go's native tooling. | |

**User's choice:** Taskfile at root + Turborepo in web/
**Notes:** Standard 2025-2026 polyglot Go monorepo pattern.

### Q8: How and when are migrations executed?

| Option | Description | Selected |
|--------|-------------|----------|
| golang-migrate CLI via Taskfile, never auto | Migrations at `/migrations/`; `task migrate-up` wraps the CLI binary. CI runs before integration tests. API never auto-runs. | ✓ |
| Embedded golang-migrate library, runs on API startup | API binary imports the library; runs migrations on startup. Multi-replica race risk. | |
| `services/api/migrations/` + embedded with lock guard | Move migrations into Go module; embed via `embed.FS`; advisory-lock guard. Diverges from FOUND-01. | |

**User's choice:** golang-migrate CLI via Taskfile, never auto
**Notes:** Keeps migrations explicit; avoids multi-replica race and DB drift hidden behind `docker compose up`.

### Q9: How much of the pnpm workspace is scaffolded in Phase 1?

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal scaffold for CI typecheck | `pnpm-workspace.yaml` + three placeholder packages with `package.json` + `tsconfig.json` + stub `index.ts`. No Vite. CI exercises pnpm toolchain. | ✓ |
| Empty workspace marker only | Just `pnpm-workspace.yaml`; CI frontend jobs noop. Latent breakage. | |
| Full Vite scaffold with Hello World | Vite + Lit + Shoelace running. Bleeds Phase 6/7 scope. | |

**User's choice:** Minimal scaffold for CI typecheck
**Notes:** Proves CI toolchain works without committing to UI tooling decisions yet.

---

## OTel + scaffold surface

### Q10: Where do OTel spans go in dev and CI?

| Option | Description | Selected |
|--------|-------------|----------|
| Jaeger in docker-compose for dev, stdout for CI | Jaeger all-in-one container; OTLP exporter. Dev sees traces at localhost:16686. | |
| OTLP endpoint only — no dev visualizer | API exports to OTLP at configurable endpoint; dev skips. | |
| Stdout in dev + CI, OTLP in prod only | All non-prod runs export to stdout. Smaller footprint. | ✓ |

**User's choice:** Stdout in dev + CI, OTLP in prod only
**Notes:** Catalog CRUD traces aren't interesting enough in v0.1 to justify a Jaeger container; revisit when runtime engine lands.

### Q11: What endpoints does Phase 1 actually ship?

| Option | Description | Selected |
|--------|-------------|----------|
| Scaffold `_scaffold` resource, removed in Phase 3 | `/healthz`, `/readyz`, and `/v1/orgs/{org_id}/_scaffold` CRUD on a placeholder table. Phase 3 deletes. | ✓ |
| First catalog entity early — ship `agents` skeleton | Phase 1 ships a thin `/v1/orgs/{org_id}/agents`. Phase 3 fills the rest. Scope creep risk. | |
| Header echo only — no DB writes | `/v1/orgs/{org_id}/_echo` reflects org_id from ctx. No DB write. Conflicts with Q6 (full chain test). | |

**User's choice:** Scaffold `_scaffold` resource, removed in Phase 3
**Notes:** Clean separation — throwaway scaffold keeps Phase 1 self-contained and the FOUND-08 test exercisable through the full chain.

### Q12: Org middleware contract — exemptions + X-Org-Id validation strictness?

| Option | Description | Selected |
|--------|-------------|----------|
| Exempt /healthz /readyz /metrics, strict UUID v4 | Strict UUIDv4 validation via `uuid.Parse`. | |
| Exempt only /healthz, accept any non-empty string | Loose validation deferred to Phase 3. Risk of corrupted rows. | |
| Exempt /healthz /readyz /metrics + OpenAPI spec endpoint | Strict UUID + serve `/openapi.yaml` from API. | |

**User's choice:** Option 1 — exempt /healthz /readyz /metrics, BUT **UUIDv7+ instead of v4** ("bất kì id nào cũng PHẢI LÀ UUIDv7 +")
**Notes:** Project-wide constraint surfaced: ALL IDs (org_id, entity PKs, FKs, request IDs) must be UUIDv7 or higher. Time-ordered variants (v7, v8) accepted; v4 rejected. This propagates beyond Phase 1 into every entity in Phase 3+. Captured as D-19 in CONTEXT.md.

---

## Go module + repo root layout

### Q13: Go module placement and import path?

| Option | Description | Selected |
|--------|-------------|----------|
| `services/api/go.mod`, path tùy user | Module at `services/api/`; path `github.com/luongdev/open-routing/services/api`. Future `services/runtime/` gets its own. | ✓ |
| Root go.mod, services/api as main package | Single module at repo root. Uber/Google internal-style. | |
| Root go.mod + go.work multi-module | `go.work` aggregates each module. Most flexible, early complexity. | |

**User's choice:** services/api/go.mod
**Notes:** Matches FOUND-01 layout. Defers `go.work` until v1 runtime service exists.

---

## Local dev experience + hot-reload

### Q14: `task dev` flow and docker-compose service composition?

| Option | Description | Selected |
|--------|-------------|----------|
| compose up infra + air hot-reload | docker-compose contains only postgres+redis; API runs native via `air` foreground. | ✓ |
| Full docker-compose with API container + air bind-mount | API runs inside compose with bind-mount. Slower iteration on macOS. | |
| Native — postgres+redis local-install, no docker | Breaks FOUND-10. | |

**User's choice:** compose up infra + air hot-reload
**Notes:** Fast dev iteration; FOUND-10 still satisfied (docker compose up brings up infra components).

---

## slog format + request correlation

### Q15: slog format and request correlation strategy?

| Option | Description | Selected |
|--------|-------------|----------|
| JSON prod/CI, tint dev + request_id + trace_id | Format changes by env; full correlation. Mixed dev experience. | |
| JSON everywhere + request_id only | JSON in all envs; no auto trace_id. | (modified) |
| Text dev / JSON prod, no request_id middleware | Format by env; trace_id from OTel only. | |

**User's choice:** Option 2 modified — **JSON everywhere with FULL request_id + trace_id + span_id correlation**
**Notes:** Hybrid: keep Option 2's JSON-everywhere shape but add the full correlation from Option 1 (request_id middleware + OTel trace_id/span_id auto-injection into every log line). Devs filter via jq.

---

## /readyz semantics + healthchecks

### Q16: /readyz checks and docker-compose dependency model?

| Option | Description | Selected |
|--------|-------------|----------|
| Deep readyz + structured 503 + healthy gates | /healthz alive only; /readyz pings DB + Redis + queries schema_migrations version. Structured JSON body. Docker-compose healthchecks gate startup. | ✓ |
| Shallow readyz + healthy gates | /readyz just pings DB + Redis. | |
| Healthz only + migration init container | No separate /readyz; migrate init container; conflicts with Q8's "never auto" decision. | |

**User's choice:** Deep readyz + structured 503 + healthy gates
**Notes:** Migration version included in readiness response is useful for ops; structured body shape forward-compatible.

---

## Claude's Discretion

- Exact `golangci-lint` linter set (start with `default + errcheck, govet, staticcheck, gosec, gocritic`).
- Exact `air.toml` hot-reload configuration.
- ESLint flavor for pnpm scaffold.
- Specific Postgres/Redis image tags and resource limits in docker-compose.
- Taskfile target dependency graph beyond the named targets.
- Inclusion of `.editorconfig` and `.gitattributes`.
- `slog.Handler` library choice (stdlib `JSONHandler` is sufficient).

## Deferred Ideas

- Real audit log table (v1, alongside runtime engine).
- Jaeger / OTel collector in docker-compose (likely Phase 4 or 5).
- Distributed lock for migrations (`go.work` + advisory-lock guard) — needed at v1 multi-replica.
- `/openapi.yaml` served from API (Phase 2 decision).
- Coverage threshold gate (Phase 3+).
- Prometheus scrape config (deployment-focused phase).
- Vite dev servers in docker-compose (Phase 6/7).
- Web Worker for CSV preview (deferred to v0.2 per PROJECT.md).
