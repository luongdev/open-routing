# Phase 3: Catalog CRUD (Go) - Research

**Researched:** 2026-05-16
**Domain:** Go REST handlers backed by sqlc + pgx + golang-migrate + Redis (go-redis v9) + chi strict-server (oapi-codegen v2.7.0); optimistic concurrency, cursor pagination, refresh-ahead caching with x/sync/singleflight
**Confidence:** HIGH (verified directly against the in-tree code, in-tree go.mod, sqlc/oapi-codegen output, pkg.go.dev for libraries; assumptions clearly tagged)

## Summary

This phase converts Phase 2's 33-method 501-stub strict-server (`services/api/internal/server/stubs.go`) into a real CRUD layer for six catalog entities + one join table. The strict-server contract, error envelope, request_id injection, UUIDv7 middleware, `orgDB` SQL-validator, and codegen-drift CI are **already in place** — Phase 3 only fills bodies, adds queries, adds a single editable migration `002_catalog_v0_1`, introduces a brand-new `internal/cache/` package with a generic `GetOrSet[T any]` + singleflight + refresh-ahead, and emits a small spec amendment (new `invalid_reference` ErrorCode + 422 responses on FK-bearing CREATE/UPDATE).

The hardest correctness corners are: (1) the **strict-server response wrappers** for 409/422 that already exist in `server.gen.go` but are NOT yet in the `injectRequestIDIntoErrorResponse` exhaustive switch (the request_id-exhaustiveness test fires the build), (2) the **CreateAgent409JSONResponseBody union** type which is an oneOf wrapper that needs `.FromErrorResponse(...)` rather than struct init, (3) **cursor pagination tuple comparison** must use `(created_at, id) < ($1, $2)` with `ORDER BY created_at DESC, id DESC LIMIT $page_size+1` and a composite DESC index, (4) **soft-delete filter** by default `enabled=true` swapping to no filter on `?include_disabled=true` (two sqlc queries, not a runtime branch), (5) **cache invalidation race** — DEL after COMMIT and never let cache-fail become a 500, (6) **refresh-ahead** uses `context.WithoutCancel(ctx)` so the request finishing does NOT cancel the goroutine, (7) **agent_skills full-replace semantics** when UpdateAgent has a non-nil `skills` array (PUT semantics, atomic transaction).

**Primary recommendation:** Adopt the wave plan in the additional_context (Wave 0 spec + scaffold deletion + drift CI; Wave 1 migration 002 + sqlc + cache pkg in parallel; Wave 2 catalog skeleton + agents end-to-end as the template; Wave 3 the other 5 entities + agent_skills in parallel; Wave 4 isolation probes + main.go wiring + final exhaustiveness sweep). Lock the standard stack to `golang.org/x/sync v0.20.0` (singleflight — already in go.mod indirect) + `github.com/alicebob/miniredis/v2 v2.37.0+` (test-only). Do **not** add any other library.

## User Constraints (from CONTEXT.md)

### Locked Decisions

> Copied verbatim from `.planning/phases/03-catalog-crud-go/03-CONTEXT.md` § Implementation Decisions (D-49..D-77).

**Cache Layer (CAT-11)**

- **D-49:** Handler-level explicit cache calls. Each `GetById` handler calls `cache.GetOrSet(ctx, key, ttl, loadFromDB)`; each write handler calls `cache.Del(ctx, key)` AFTER the DB transaction commits. The cache contains DTO types (e.g., `api.Agent`), NOT sqlc row types — the cache sits AFTER the sqlc→DTO mapping step in the handler. List endpoints (CAT-09, CAT-10) are NOT cached.
- **D-50:** Cache lives in standalone `services/api/internal/cache/` package — reusable across phases. Shape: `cache.Cache struct { rdb *redis.Client; sf *singleflight.Group; logger *slog.Logger }` constructed in `main.go` and passed via the catalog `Deps` struct.
- **D-51:** Generic `cache.GetOrSet[T any]` helper using Go type parameters. Signature: `func GetOrSet[T any](ctx context.Context, c *Cache, key string, ttl time.Duration, load func(context.Context) (T, error)) (T, error)`. JSON marshal/unmarshal happens inside the helper; callers see typed values. For not-found semantics the loader returns `(T{}, ErrNotFound)` and the helper propagates without caching the empty value.
- **D-52:** `x/sync/singleflight` from day 1. Cache misses go through a `singleflight.Group` so concurrent requests for the same (orgId, entity, id) key issue only one DB load per process. Cross-instance stampede protection (Redis SETNX lock) NOT in v0.1.
- **D-53:** Refresh-ahead at 10s remaining (~16% of TTL). Inside `GetOrSet`, after a hit, check `PTTL(key)`; if remaining < 10s, kick off an async singleflight refresh that re-loads from DB and writes a fresh cache entry. Async refresh uses `context.WithoutCancel(ctx)` so request cancellation doesn't kill the refresh.
- **D-54:** No negative caching in v0.1.
- **D-55:** Invalidation order: `DEL` after DB commit succeeds. Cache failure NEVER turns a successful write into a 5xx; log a warn and continue.
- **D-56:** 409 (version mismatch) also DELs the cache key. Re-fetches the current row from DB, calls `cache.Del(ctx, key)`, then returns 409 with the fresh row as `current`.
- **D-57:** Observability via slog attrs only in v0.1 (`outcome=hit|miss|refresh|error`). No OTel metrics export pipeline.
- **D-58:** Cache key format `or:{orgId}:{entity}:{id}`. Helper constructor: `cache.Key(orgID, "agents", agentID)`.
- **D-59:** TTL is fixed 60s (no jitter).
- **D-60:** JSON encoding inside the cache uses `encoding/json` (stdlib).

**Schema + sqlc Layout**

- **D-61:** Single editable migration `002_catalog_v0_1.up.sql` + `.down.sql` for the entire v0.1 milestone. Phases 4/5 EDIT this migration in-place. Once v0.1 ships, freeze and ratchet forward with 003+.
- **D-62:** Per-entity sqlc query files at `services/api/internal/db/queries/{agents,skills,queues,channels,adapters,break_reasons,agent_skills}.sql`.
- **D-63:** Cursor pagination format: `base64(JSON {created_at: RFC3339Nano, id: UUIDv7})`. SQL: `WHERE (created_at, id) < ($cursor_ts, $cursor_id) ORDER BY created_at DESC, id DESC LIMIT $page_size + 1`. The `+1` row tells us whether to emit `next_cursor`.
- **D-64:** Name search uses `ILIKE '%' || $name || '%'` with a functional index `CREATE INDEX ix_{entity}_org_name ON {entity}(org_id, lower(name) text_pattern_ops) WHERE enabled = true`. Trigram/`pg_trgm` deferred to v0.2.
- **D-65:** Default-list `WHERE enabled = true` filter applied unconditionally in the sqlc List query. `?include_disabled=true` swaps to a separate query that omits the filter. Two queries per entity.
- **D-66:** Atomic UPDATE with version check + `RETURNING *`. Single round-trip happy path. If 0 rows returned, the handler issues a follow-up `SELECT ... WHERE id = $1 AND org_id = $2` to disambiguate 404 vs 409.
- **D-67:** Page size cap: **25 default, 100 max**. Out-of-range returns 400 invalid_body.

**Handler Package Layout**

- **D-68:** Single `services/api/internal/catalog/` package with one `.go` per entity + shared helpers (`handlers.go`, `mappers.go`, `cursor.go`, `errors.go`).
- **D-69:** `catalog.Handlers` IS the `api.StrictServerInterface` implementer. No composite server. Phase 2 stubs + scaffold DELETED in Phase 3 commit 1. Until Phases 4 and 5 land, `catalog.Handlers` returns 501 for state-machine + bulk-import methods via a `notImplemented` helper.
- **D-70:** Forward-compat for Phases 4/5 via struct embedding (`type ApiHandlers struct { *catalog.Handlers; *state.Handlers; *imports.Handlers }`). Phase 3 ships `catalog.Handlers` alone.
- **D-71:** Hybrid constructor `catalog.New(deps Deps, opts ...Option)`. Required: `OrgDBFactory`, `Cache`, `Logger`. Options: `WithClock` only in v0.1.
- **D-72:** Per-entity `_test.go` beside source. Tests use `httptest.NewRecorder` + miniredis + Postgres testcontainers. Cross-org probe coverage lives in `services/api/test/isolation/catalog_test.go`.
- **D-73:** Shared test setup in `services/api/internal/catalog/testutil.go`.

**Concurrency + Validation**

- **D-74:** Two-layer validation. Layer 1: oapi-codegen runtime schema validation (400 `invalid_body`). Layer 2: handler-level cross-row rules requiring DB queries (422 `invalid_reference`).
- **D-75:** Spec amendment in Phase 3 adds `invalid_reference` to the ErrorCode enum (11th value) and 422 responses on every FK-bearing CREATE/UPDATE.
- **D-76:** Cross-row validation runs BEFORE the UPDATE/INSERT in the handler. No DB-level FKs across catalog tables in v0.1.

**Phase 1/2 Carry-Forward**

- **D-77:** Scaffold + 501-stubs deleted in Phase 3 commit 1. Plan order: (1) delete scaffold + stubs; (2) regenerate codegen (drift CI passes); (3) introduce migration 002 + sqlc files; (4) introduce `internal/cache/`; (5) introduce `internal/catalog/` with agents end-to-end; (6) replicate for 5 other entities; (7) add agent_skills join.

### Claude's Discretion

- Exact slog attribute keys for cache observability (`outcome`, `cache_key`, etc.) — planner picks consistent names.
- The Go file ordering inside `internal/catalog/` (alphabetical vs domain-grouped) — alphabetical is the default.
- Default page size between 25 and 50 — locked at 25 here.
- Whether `pgError` 23505 (unique violation) on CREATE returns 409 or 422 — default 409 with `version_conflict`-flavored body for external_id collisions.
- Whether `ListAgents` and `ListAgentsIncludingDisabled` are two sqlc queries or one query with a `$show_disabled bool` parameter.
- Mapper file naming (`mappers.go` vs `convert.go`).

### Deferred Ideas (OUT OF SCOPE)

- Bloom filter for 404 protection
- OTel cache_ops_total counter export
- 5-10s negative caching sentinel
- TTL jitter
- `pg_trgm` + GIN index for name search
- DB-level FK constraints across catalog tables
- Cache for list endpoints
- `cache.Health()` integration with `/readyz`
- Cursor signing / HMAC
- Mapper code generation (`goverter`)
- Audit-trail tables for catalog writes

## Project Constraints (from CLAUDE.md)

`./CLAUDE.md` is the project README, not a directive file — no in-tree rules to extract. The project-wide rule from `~/.claude/CLAUDE.md` (every `gh` issue/PR created on this machine must use `--assignee mpt-luongld`) applies if Phase 3 plans involve GitHub issue/PR creation, but plain plan files do not.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CAT-01 | Org admin can CRUD + soft-delete `agents` via REST | `agents.go` handler in `internal/catalog/`; sqlc `agents.sql`; migration 002 `agents` table; DTO already generated at `types.gen.go:341 type Agent struct` |
| CAT-02 | Org admin can CRUD + soft-delete `skills` (incl. `description`, `skill_type`) | `skills.go`; sqlc `skills.sql`; migration 002 `skills` table; DTO `types.gen.go:854 type Skill` |
| CAT-03 | Org admin can assign/unassign skills to agents with `proficiency 1-10` | `agent_skills.go` + agent UPDATE skills-replace logic in `agents.go`; sqlc `agent_skills.sql`; OpenAPI already encodes `minimum: 1, maximum: 10` on `AgentSkillAssignment.proficiency`; runtime oapi-codegen schema validation rejects out-of-range with 400 `invalid_body` (D-74 Layer 1) — Layer 2 cross-row check ensures `skill_id` exists in same org → 422 `invalid_reference` (D-75/D-76) |
| CAT-04 | Org admin can CRUD + soft-delete `queues` (incl. `channel_types[]`, `priority`, `acw_sec`) | `queues.go`; sqlc `queues.sql`; migration 002 `queues` table; DTO `types.gen.go:787`. `channel_types` is `text[]` in Postgres with `pgtype.Array[string]` in pgx/v5 |
| CAT-05 | Org admin can CRUD + soft-delete `channels` (incl. `default_queue_id` nullable FK) | `channels.go`; sqlc `channels.sql`; migration 002 `channels`; DTO `types.gen.go:527`. `default_queue_id` cross-row validation BEFORE write (D-76) — must exist + be enabled in same org → 422 invalid_reference. **Channel has NO `version` column** in the OpenAPI schema (verified `openapi.yaml:667-709`) — `UpdateChannelRequest` has no `version` field; CAT-08 does NOT apply to channels. Confirm with planner whether to keep parity (recommend yes; add version for forward-compat in v0.2) |
| CAT-06 | Org admin can CRUD + soft-delete `adapters` (incl. `config jsonb`) | `adapters.go`; sqlc `adapters.sql`; migration 002 `adapters`. `config` is `jsonb` — sqlc emits `[]byte` by default for pgx/v5; the handler marshals/unmarshals against `api.Adapter.Config` (which is `map[string]any` from OpenAPI `additionalProperties: true`). **Adapter has NO `external_id` column** (verified spec `openapi.yaml:751-803`) — different from other entities. **Adapter has NO `version`** — same parity question as channel |
| CAT-07 | Org admin can CRUD + soft-delete `break_reasons` (incl. `routable BOOLEAN`, `display_order`) | `break_reasons.go`; sqlc `break_reasons.sql`; migration 002 `break_reasons`. **break_reasons has NO `external_id`** — `display_order` is the user-meaningful ordering key. `routable` flag consumed by Phase 4 `IsRoutable` helper (STATE-10) |
| CAT-08 | Update endpoints require `version`; mismatch returns 409 with current record | sqlc UPDATE pattern in D-66; handler 404-vs-409 disambiguation via fallback SELECT; cache DEL on 409 per D-56. **Wire 4 typed 409 responses already generated** (`UpdateAgent409`, `UpdateSkill409`, `UpdateQueue409`, `UpdateBreakReason409`) — channels & adapters omit version. `CreateAgent409` is a union (oneOf ErrorResponse vs VersionConflictErrorResponse) — needs `.FromErrorResponse(...)` or `.FromVersionConflictErrorResponse(...)` builder, NOT struct init |
| CAT-09 | Soft-deleted rows excluded by default; `?include_disabled=true` surfaces them | Per D-65: two sqlc queries per entity. List handlers branch on `params.IncludeDisabled` (already in generated `ListAgentsParams` etc.) |
| CAT-10 | Cursor pagination + `?include_disabled` + case-insensitive `name` substring search | Per D-63/D-64: composite (created_at, id) tuple comparison; ILIKE on `lower(name)` with partial functional index. Generated params already include `Cursor *CursorQuery`, `Limit *LimitQuery`, `IncludeDisabled *IncludeDisabledQuery`, `Name *NameSearchQuery`. **Conflict to resolve in Wave 0 spec amendment:** spec defines `LimitQuery` default=20 max=100 — D-67 mandates default 25, max 100. Edit `openapi.yaml` parameters block + rename to `page_size` OR accept the deviation. Recommend: edit spec to `default: 25` and rename param to `page_size` to match D-67 exactly. Codegen will regenerate `LimitQuery` and rename ListXParams field; drift CI catches missing regeneration |
| CAT-11 | Single-entity GETs cached under `or:{orgId}:{entity}:{id}` with 60s TTL; invalidate on write | All cache decisions D-49..D-60. Handler `GetById` wraps DB load in `cache.GetOrSet[T]`; write handlers `cache.Del` after commit. Only single-entity GETs cached; lists are not |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| HTTP routing | Frontend Server (chi) | — | Strict-server pipeline at `api.HandlerWithOptions`; routes registered from generated spec |
| Org isolation | Frontend Server (middleware) + Backend (orgDB) | DB | `OrgContext` middleware sets `org_id` in ctx; `orgDB.preflight` rejects unscoped SQL at runtime |
| Request validation | API codegen (oapi-codegen) | Backend (catalog handler cross-row) | Layer 1 schema enforcement at decode time; Layer 2 cross-row FK-style checks in handler before write (D-74) |
| Business logic | Backend (catalog handlers) | — | One file per entity in `internal/catalog/` |
| Data persistence | Backend (sqlc + pgx) | DB (Postgres 17) | sqlc generates typed `*Queries`; orgDB-wrapped pool enforces filter at SQL parse time |
| Hot-path read cache | Backend (cache pkg) | Redis | Standalone `internal/cache/` with singleflight + refresh-ahead; per-process dedup, not cross-instance |
| Soft delete | Backend (handler + sqlc) | DB (partial index) | UPDATE `enabled=false`; list queries filter `WHERE enabled = true`; functional index keeps default lists fast |
| Cursor pagination | Backend (cursor.go) | DB (composite DESC index) | Server encodes/decodes opaque base64 cursor; SQL uses `(created_at, id) < ($1, $2) ORDER BY ... DESC LIMIT N+1` |
| Optimistic concurrency | Backend (sqlc UPDATE with version check) | DB (atomic UPDATE) | `WHERE version = $N`; 0 rows + fallback SELECT disambiguates 404 vs 409 |
| Cross-row validation (FKs) | Backend (catalog handler) | — | App-layer only in v0.1 per D-76; no DB FKs across catalog tables |
| Observability | Backend (slog + OTel span attrs) | — | slog attrs `outcome=hit|miss|refresh|error`; no OTel metrics export pipeline yet |
| Error envelope + request_id | Frontend Server (strict-server pipeline middleware) | — | `RequestIDInjectionMiddleware` already wraps every operation; Phase 3 only extends the type switch with new 409/422 wrappers |

## Standard Stack

### Core (already in go.mod, no install required)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.2.3 | HTTP router | Locked in Phase 1 D-23; strict-server wires onto chi root [VERIFIED: services/api/go.mod] |
| `github.com/jackc/pgx/v5` | v5.9.2 | Postgres driver + pool | Locked in Phase 1; orgDB wraps `*pgxpool.Pool` [VERIFIED: services/api/go.mod] |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 | Migration tool | Locked Phase 1 D-22 [VERIFIED: services/api/go.mod] |
| `github.com/google/uuid` | v1.6.0 | UUIDv7 generation | Locked Phase 1 D-19 [VERIFIED: services/api/go.mod] |
| `github.com/redis/go-redis/v9` | v9.19.0 | Redis client | Already in main.go [VERIFIED: services/api/go.mod] |
| `github.com/oapi-codegen/oapi-codegen/v2` | v2.7.0 | OpenAPI codegen | Locked Phase 2 D-41 [VERIFIED: services/api/go.mod] |
| `github.com/oapi-codegen/runtime` | v1.4.0 | Strict-server runtime helpers | Locked Phase 2 [VERIFIED: services/api/go.mod] |
| `log/slog` | stdlib | Structured logging | Locked Phase 1 D-27/D-29 |
| `encoding/json` | stdlib | JSON encoding (cache payloads) | Locked D-60 (no third-party) |

### Supporting (already indirect — promote to direct in Phase 3)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/sync` | v0.20.0 | `singleflight.Group` for cache stampede dedup | D-52 — required; already in go.sum as indirect, will become direct in Phase 3 [VERIFIED: services/api/go.sum line 295] |

### Supporting (test-only, NEW)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/alicebob/miniredis/v2` | v2.37.0 or newer (latest v2.38.0 May 2026) | In-process Redis for unit tests | D-73 — every entity `_test.go` that asserts cache hit/miss/refresh uses miniredis. Integration suite in `test/isolation/` continues to use the real Redis testcontainer (optional, currently nil-tolerant). [VERIFIED: pkg.go.dev, MIT license, active maintenance through May 2026] |

**Note on slopcheck verdict:** `slopcheck install --ecosystem go github.com/alicebob/miniredis/v2` flagged the package as `[SLOP]` based on stale pkg.go.dev metadata ("created 4 days ago, no source repository linked"). This is a **false positive** — the project has been live since 2015 with continuous releases (v2.0 in 2018, v2.30.x in 2023, v2.37.0 in Feb 2026, v2.38.0 in May 2026), MIT license, source at `github.com/alicebob/miniredis`. The Phase 1/2 in-tree pattern explicitly cites it via `.planning/phases/02-openapi-contract-codegen/02-CONTEXT.md` canonical_refs. **Disposition: APPROVED** despite slopcheck verdict. Planner SHOULD insert a single `checkpoint:human-verify` task before `go get` lands in go.mod to confirm version, since slopcheck's metadata reading is the canonical legitimacy oracle the project uses elsewhere.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `cache.GetOrSet[T any]` generic helper (D-51) | `cache.Cache[T any]` parameterized type | Type-parameterized struct couples cache instance to T; same `*Cache` cannot service `Agent` and `Skill`. Function-level generic keeps one shared `*Cache` and varies per call. |
| singleflight + refresh-ahead (D-52/D-53) | TTL jitter (60s ± 5s) | Jitter randomizes stampede over a window but does not prevent the cold-expiry stampede. Refresh-ahead pre-warms, singleflight dedupes. Locked. |
| Two sqlc queries per entity (`ListAgents` + `ListAgentsIncludingDisabled`) per D-65 | One query with `$show_disabled bool` and `WHERE enabled = TRUE OR $show_disabled` | Composite predicate `enabled = TRUE OR $show_disabled` defeats the partial functional index `ix_*_org_name WHERE enabled = true`. Two queries each statement-cache cleanly. Locked. |
| `encoding/json` (D-60) | `goccy/go-json` or msgpack | Performance wins are <2× at v0.1 scale; introduces a new dep + serialization-format coupling. Stdlib stays. |
| Manual mappers (`sqlc.Agent → api.Agent`) | `goverter` codegen | 6 entities × ~30 LOC = ~180 LOC of straightforward field mapping. Codegen step adds tooling complexity. Manual is fine in v0.1. |

**Installation (single delta):**

```bash
cd services/api && go get golang.org/x/sync@v0.20.0 github.com/alicebob/miniredis/v2@latest && go mod tidy
```

**Version verification (confirmed against authoritative registries 2026-05-16):**

```bash
# Already in indirect deps — promotes to direct on import
go list -m golang.org/x/sync           # v0.20.0 (in go.sum line 295) [VERIFIED]
go list -m github.com/redis/go-redis/v9 # v9.19.0 (in go.mod)         [VERIFIED]

# New direct dep (test-only)
# https://pkg.go.dev/github.com/alicebob/miniredis/v2 → v2.38.0 (May 12, 2026)  [CITED]
# https://github.com/alicebob/miniredis/releases → v2.38.0, v2.37.0, v2.36.x   [CITED]
```

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| golang.org/x/sync | Go (pkg.go.dev) | 7+ yrs (golang.org/x mono) | Standard golang.org mirror | github.com/golang/sync | [OK] | Approved (already indirect; promote to direct) |
| github.com/alicebob/miniredis/v2 | Go (pkg.go.dev) | 11 yrs (since 2015); v2 series since 2018; v2.38.0 May 2026 | Listed by ~7,000 importers on pkg.go.dev | github.com/alicebob/miniredis (MIT) | [SLOP - false positive] | Approved with planner-inserted `checkpoint:human-verify` task before `go get` lands |

**Packages removed due to slopcheck [SLOP] verdict:** none — only false-positive flagging on a verifiably legitimate package.
**Packages flagged as suspicious [SUS]:** none.

slopcheck's `[SLOP]` verdict for `miniredis/v2` is a documented false-positive class: pkg.go.dev's API exposes only the most recent module-revision timestamp, not the project age. The package has been continuously maintained since 2015 with v2.0 in 2018. Phase 2 context already references it as the canonical Redis test double (`02-CONTEXT.md`). The planner should still gate the install behind a one-line human-verify checkpoint as defensive practice, but the package is approved for use.

## Architecture Patterns

### System Architecture Diagram

```
                                ┌──────────────────────────────────────────┐
                                │  HTTP request (X-Org-Id header + body)  │
                                └──────────────────┬───────────────────────┘
                                                   │
                  ┌────────────────────────────────▼─────────────────────────────────┐
                  │ chi root mux (server.NewMux)                                       │
                  │   1. chimw.Recoverer                                               │
                  │   2. appmw.RequestID  (mints UUIDv7 X-Request-Id, sets ctx)        │
                  │   3. /metrics  (bypass)                                            │
                  │   4. api.HandlerWithOptions → strict-server pipeline               │
                  │        - orgContextMiddleware       (only /v1/*)                   │
                  │        - uuidv7PathParamsMiddleware (only /v1/* with {id})         │
                  │        - StrictMiddleware: RequestIDInjectionMiddleware            │
                  │        - decode JSON body, validate against generated schema       │
                  │          → 400 invalid_body on schema fail                         │
                  └──────────────────┬───────────────────────────────────────────────┘
                                     │ catalog.Handlers.<MethodName>(ctx, req)
                                     ▼
   ┌──────────────────────────────────────────────────────────────────────────────┐
   │ internal/catalog/<entity>.go                                                  │
   │                                                                                │
   │  GetByID flow                          Write flow (Create/Update/Delete)       │
   │  ──────────────                        ────────────────────────────────         │
   │  1. orgID = orgkey.FromContext(ctx)    1. orgID = orgkey.FromContext(ctx)       │
   │  2. key = cache.Key(orgID, ent, id)    2. cross-row validation (D-76):          │
   │  3. result = cache.GetOrSet[T](         │   if FK fields present:                │
   │       ctx, c, key, 60s,                 │     q.<XX>ExistsInOrg() → 422          │
   │       func(c) { /* DB load */ })       3. q = generated.New(orgDB)              │
   │  4. → 404 if ErrNotFound                4. UPDATE … WHERE version = $N RETURNING│
   │                                            ↓ 0 rows                              │
   │                                          5. fallback SELECT → 404 or 409+current │
   │                                          6. commit                               │
   │                                          7. cache.Del(ctx, key)  (log+continue)  │
   │                                          8. return 200/201/204                   │
   └────────────────┬────────────────────────────┬────────────────────────────────────┘
                    │ generated.New(orgDB)       │ orgDB.preflight rejects unscoped SQL
                    ▼                            ▼
   ┌──────────────────────────────┐    ┌─────────────────────────────────┐
   │ db.OrgDB.QueryRow/Exec       │    │ internal/cache/cache.go         │
   │  - SQLChecker.Parse SQL once │    │  GetOrSet[T any]                │
   │  - assert WHERE org_id=$N    │    │   1. GET key                    │
   │  - delegate to pgxpool       │    │   2a. hit: PTTL → if <10s:      │
   └──────────────┬───────────────┘    │       spawn refresh goroutine   │
                  │                    │       (context.WithoutCancel)   │
                  ▼                    │   2b. miss: sf.Do(key, loader)  │
   ┌──────────────────────────────┐    │       SET key value EX 60s      │
   │ PostgreSQL 17                │    │   3. unmarshal JSON → T         │
   │  agents, skills, queues,     │    │  Del(key) — log on fail         │
   │  channels, adapters,         │    └──────────────┬──────────────────┘
   │  break_reasons, agent_skills │                   │
   │  + composite DESC indexes    │                   ▼
   │  + partial functional idx on │       ┌─────────────────────────────┐
   │    lower(name)               │       │ Redis (go-redis v9 client)   │
   └──────────────────────────────┘       └─────────────────────────────┘
```

Trace from request to response (CAT-01 happy GET):

1. `GET /v1/orgs/{org_id}/agents/{id}` with `X-Org-Id: <orgA>`
2. RequestID middleware mints UUIDv7 for `X-Request-Id`
3. orgContextMiddleware reads header, sets `org_id` in ctx
4. uuidv7PathParamsMiddleware validates `{id}` is UUIDv7 (400 invalid_id else)
5. Strict pipeline decodes empty body, calls `catalog.GetAgent(ctx, req)`
6. Handler reads `orgID` from ctx, builds key `or:{orgA}:agents:{id}`, calls `cache.GetOrSet[api.Agent]`
7. miss → `sf.Do(key, loader)` → loader calls `q.GetAgent(ctx, params)` → orgDB preflight validates SQL has `WHERE … org_id = $N` → pgxpool query
8. mapper converts sqlc row to `api.Agent` (with embedded skills via `q.ListSkillsForAgent`)
9. JSON-encode, `SET key value EX 60s`, return
10. Handler returns `GetAgent200JSONResponse(agent)`
11. `RequestIDInjectionMiddleware` is a pass-through on success
12. Strict-server emits 200 with JSON body

### Recommended Project Structure (new + edited files)

```
services/api/
├── cmd/api/main.go                       # EDIT: construct cache.New, catalog.New
│
├── internal/
│   ├── api/                              # CODEGEN OUTPUT (do not hand-edit)
│   │   ├── server.gen.go                 # REGEN after spec amendment
│   │   ├── types.gen.go                  # REGEN after spec amendment
│   │   └── spec.gen.go                   # REGEN after spec amendment
│   │
│   ├── cache/                            # NEW package (D-50)
│   │   ├── cache.go                      # Cache struct + GetOrSet[T] + Del + Key
│   │   ├── cache_test.go                 # unit tests with miniredis
│   │   └── doc.go                        # package doc + locked contracts
│   │
│   ├── catalog/                          # NEW package (D-68/D-69)
│   │   ├── handlers.go                   # Deps struct + New(deps,opts) constructor
│   │   ├── mappers.go                    # sqlc → api DTO conversions
│   │   ├── cursor.go                     # Encode/Decode opaque base64 cursor
│   │   ├── errors.go                     # pgx/pgconn error → strict response mapping
│   │   ├── notimpl.go                    # 501 stubs for Phase 4/5 methods
│   │   ├── testutil.go                   # shared test setup (miniredis + pg pool)
│   │   ├── agents.go         agents_test.go         # CAT-01, CAT-03
│   │   ├── agent_skills.go   agent_skills_test.go   # CAT-03 join helpers
│   │   ├── skills.go         skills_test.go         # CAT-02
│   │   ├── queues.go         queues_test.go         # CAT-04
│   │   ├── channels.go       channels_test.go       # CAT-05
│   │   ├── adapters.go       adapters_test.go       # CAT-06
│   │   └── break_reasons.go  break_reasons_test.go  # CAT-07
│   │
│   ├── db/
│   │   ├── generated/                    # sqlc OUTPUT (auto-regenerated)
│   │   │   ├── agents.sql.go             # NEW (sqlc emits one file per .sql)
│   │   │   ├── skills.sql.go             # NEW
│   │   │   ├── queues.sql.go             # NEW
│   │   │   ├── channels.sql.go           # NEW
│   │   │   ├── adapters.sql.go           # NEW
│   │   │   ├── break_reasons.sql.go      # NEW
│   │   │   ├── agent_skills.sql.go       # NEW
│   │   │   ├── scaffold.sql.go           # DELETE
│   │   │   ├── models.go                 # REGEN — sqlc adds catalog models, drops Scaffold
│   │   │   └── db.go                     # unchanged
│   │   ├── queries/
│   │   │   ├── agents.sql                # NEW
│   │   │   ├── skills.sql                # NEW
│   │   │   ├── queues.sql                # NEW
│   │   │   ├── channels.sql              # NEW
│   │   │   ├── adapters.sql              # NEW
│   │   │   ├── break_reasons.sql         # NEW
│   │   │   ├── agent_skills.sql          # NEW
│   │   │   └── scaffold.sql              # DELETE
│   │   └── (orgdb.go, pool.go, sqlcheck.go, bypass.go, orgkey/ — unchanged)
│   │
│   ├── scaffold/                         # DELETE entire directory
│   │   ├── handler.go                    # DELETE
│   │   └── handler_test.go               # DELETE
│   │
│   ├── server/
│   │   ├── server.go                     # EDIT: NewMux signature reduced; injectRequestIDIntoErrorResponse extended for new 409/422 types; CreateAgent409 union special case
│   │   ├── stubs.go                      # DELETE (compositeServer + 501 stubs)
│   │   ├── health.go                     # unchanged
│   │   ├── openapi.go                    # unchanged
│   │   └── request_id_exhaustiveness_test.go  # EDIT: add cases for new 409/422 wrappers (UpdateAdapter409 if added, etc.)
│   │
│   └── (middleware/, telemetry/, config/, testsupport/ — unchanged)
│
├── test/isolation/
│   ├── catalog_test.go                   # NEW: per-entity cross-org 404 probes (6 × 4 ops)
│   ├── isolation_test.go                 # EDIT: replace _scaffold seed with one catalog entity seed (or keep + add a parallel agents probe)
│   └── main_test.go                      # EDIT: composite server construction → catalog.New(deps)
│
└── go.mod, go.sum                        # EDIT: golang.org/x/sync direct; alicebob/miniredis/v2 test-only

migrations/
├── 000001_create_scaffold.up.sql         # OBSOLETE — keep on disk (golang-migrate records version)
├── 000001_create_scaffold.down.sql       # OBSOLETE — keep on disk
└── 000002_catalog_v0_1.up.sql            # NEW (DROP _scaffold + CREATE all 7 catalog tables + indexes)
└── 000002_catalog_v0_1.down.sql          # NEW (DROP catalog tables + recreate _scaffold for migrate-down safety)

openapi/openapi.yaml                      # EDIT: add invalid_reference enum value + 422 response on FK-bearing endpoints; rename limit→page_size, default 25; remove Scaffold paths + CreateScaffoldRequest/Scaffold schemas

web/packages/ui/src/api/generated.ts      # REGEN
```

### Pattern 1: Strict-server catalog handler (typed response objects)

**What:** Each catalog method on `catalog.Handlers` returns a typed `<Op>ResponseObject` interface value. oapi-codegen converts that to the wire response.

**When to use:** Always — Phase 2 D-44 + D-69 lock this. Never fall through to `middleware.WriteError` from inside a strict handler; only use `WriteError` outside the strict pipeline (in plain chi middleware).

**Example (Get by ID with cache):**

```go
// Source: pattern lifted from scaffold/handler.go GetScaffoldById + extended with cache (D-49)
func (h *Handlers) GetAgent(ctx context.Context, req api.GetAgentRequestObject) (api.GetAgentResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        return api.GetAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
    }
    key := cache.Key(orgID, "agents", uuid.UUID(req.Id))
    agent, err := cache.GetOrSet[api.Agent](ctx, h.deps.Cache, key, 60*time.Second,
        func(ctx context.Context) (api.Agent, error) {
            q := generated.New(h.deps.OrgDB)
            row, err := q.GetAgent(ctx, generated.GetAgentParams{
                ID:    pgtype.UUID{Bytes: req.Id, Valid: true},
                OrgID: pgtype.UUID{Bytes: orgID, Valid: true},
            })
            if errors.Is(err, pgx.ErrNoRows) {
                return api.Agent{}, cache.ErrNotFound
            }
            if err != nil { return api.Agent{}, err }
            // load embedded skills
            skills, err := q.ListSkillsForAgent(ctx, generated.ListSkillsForAgentParams{...})
            if err != nil { return api.Agent{}, err }
            return mapAgent(row, skills), nil
        })
    switch {
    case errors.Is(err, cache.ErrNotFound):
        return api.GetAgent404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
            Error: api.ErrorCodeNotFound, Reason: "agent_not_found"}}, nil
    case err != nil:
        h.deps.Logger.ErrorContext(ctx, "get agent", "err", err)
        return api.GetAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error: api.ErrorCodeInternal, Reason: "internal"}}, nil
    }
    return api.GetAgent200JSONResponse(agent), nil
}
```

### Pattern 2: Atomic UPDATE with version check + RETURNING + 404/409 disambiguation (D-66, D-56)

```sql
-- agents.sql
-- name: UpdateAgent :one
UPDATE agents
SET name = $2, email = $3, enabled = $4, version = version + 1, updated_at = NOW()
WHERE id = $1 AND org_id = $5 AND version = $6
RETURNING id, org_id, external_id, name, email, enabled, version, created_at, updated_at;

-- name: GetAgentByIdAnyVersion :one
-- used for 404-vs-409 disambiguation when UpdateAgent returns 0 rows
SELECT id, org_id, external_id, name, email, enabled, version, created_at, updated_at
FROM agents
WHERE id = $1 AND org_id = $2;
```

```go
// Source: pattern derived from CONTEXT.md D-66 + verified against pgx v5 + generated UpdateAgent409JSONResponse
func (h *Handlers) UpdateAgent(ctx context.Context, req api.UpdateAgentRequestObject) (api.UpdateAgentResponseObject, error) {
    orgID, _ := orgkey.OrgIDFromContext(ctx)
    q := generated.New(h.deps.OrgDB)

    row, err := q.UpdateAgent(ctx, generated.UpdateAgentParams{
        ID: pgtype.UUID{Bytes: req.Id, Valid: true},
        Name: deref(req.Body.Name),
        Email: deref(req.Body.Email),
        Enabled: deref(req.Body.Enabled),
        OrgID: pgtype.UUID{Bytes: orgID, Valid: true},
        Version: int32(req.Body.Version),
    })
    if errors.Is(err, pgx.ErrNoRows) {
        // 0 rows — was it 404 or 409?
        current, perr := q.GetAgentByIdAnyVersion(ctx, generated.GetAgentByIdAnyVersionParams{
            ID: pgtype.UUID{Bytes: req.Id, Valid: true},
            OrgID: pgtype.UUID{Bytes: orgID, Valid: true},
        })
        if errors.Is(perr, pgx.ErrNoRows) {
            return api.UpdateAgent404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
                Error: api.ErrorCodeNotFound, Reason: "agent_not_found"}}, nil
        }
        if perr != nil {
            return api.UpdateAgent500JSONResponse{...}, nil
        }
        // 409: invalidate cache (D-56) so next GET reads fresh
        _ = h.deps.Cache.Del(ctx, cache.Key(orgID, "agents", uuid.UUID(req.Id)))
        return api.UpdateAgent409JSONResponse{
            Current: mapAgent(current, nil),
            Error:   "version_conflict",
            Reason:  "version_mismatch",
        }, nil
    }
    if err != nil { /* 500 */ }

    // happy path: invalidate cache after commit (D-55). Failure logs, doesn't 500.
    if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "agents", uuid.UUID(req.Id))); delErr != nil {
        h.deps.Logger.WarnContext(ctx, "cache del failed", "key", "...", "err", delErr)
    }
    return api.UpdateAgent200JSONResponse(mapAgent(row, /* reload skills if changed */)), nil
}
```

### Pattern 3: Cache `GetOrSet[T]` with singleflight + refresh-ahead (D-51..D-53)

```go
// Source: pattern derived from CONTEXT.md D-51, D-52, D-53 + singleflight godoc
// Verified: golang.org/x/sync/singleflight.Do returns (any, error, shared)

package cache

import (
    "context"
    "encoding/json"
    "errors"
    "log/slog"
    "time"

    "github.com/redis/go-redis/v9"
    "golang.org/x/sync/singleflight"
)

var ErrNotFound = errors.New("cache: not found")

type Cache struct {
    rdb    *redis.Client
    sf     *singleflight.Group
    logger *slog.Logger
}

func New(rdb *redis.Client, logger *slog.Logger) *Cache {
    return &Cache{rdb: rdb, sf: &singleflight.Group{}, logger: logger}
}

// refreshThreshold is the locked 10s remaining window per D-53.
const refreshThreshold = 10 * time.Second

func GetOrSet[T any](ctx context.Context, c *Cache, key string, ttl time.Duration,
    load func(context.Context) (T, error)) (T, error) {
    var zero T

    // 1. Hit path: GET + PTTL → maybe refresh-ahead
    raw, err := c.rdb.Get(ctx, key).Bytes()
    if err == nil {
        var v T
        if jerr := json.Unmarshal(raw, &v); jerr == nil {
            c.logger.DebugContext(ctx, "cache", "key", key, "outcome", "hit")
            // refresh-ahead check (D-53)
            if pttl, perr := c.rdb.PTTL(ctx, key).Result(); perr == nil && pttl > 0 && pttl < refreshThreshold {
                bgCtx := context.WithoutCancel(ctx) // request cancel must NOT cancel refresh
                go func() {
                    _, _, _ = c.sf.Do("refresh:"+key, func() (any, error) {
                        c.logger.DebugContext(bgCtx, "cache", "key", key, "outcome", "refresh")
                        nv, lerr := load(bgCtx)
                        if lerr != nil { return nil, lerr }
                        nb, jerr := json.Marshal(nv)
                        if jerr != nil { return nil, jerr }
                        _ = c.rdb.Set(bgCtx, key, nb, ttl).Err()
                        return nv, nil
                    })
                }()
            }
            return v, nil
        }
        // Unmarshal failed — treat as miss; logged.
        c.logger.WarnContext(ctx, "cache unmarshal", "key", key, "err", err)
    } else if !errors.Is(err, redis.Nil) {
        c.logger.WarnContext(ctx, "cache get", "key", key, "err", err)
    }

    // 2. Miss path: singleflight-dedup'd DB load
    raw2, sferr, _ := c.sf.Do(key, func() (any, error) {
        c.logger.DebugContext(ctx, "cache", "key", key, "outcome", "miss")
        nv, lerr := load(ctx)
        if lerr != nil { return nil, lerr }
        nb, jerr := json.Marshal(nv)
        if jerr != nil { return nil, jerr }
        if serr := c.rdb.Set(ctx, key, nb, ttl).Err(); serr != nil {
            c.logger.WarnContext(ctx, "cache set", "key", key, "err", serr)
        }
        return nv, nil
    })
    if sferr != nil { return zero, sferr } // ErrNotFound propagates
    return raw2.(T), nil
}

func (c *Cache) Del(ctx context.Context, key string) error {
    return c.rdb.Del(ctx, key).Err()
}

func Key(orgID, entity, id any) string {
    return "or:" + fmt.Sprintf("%s:%s:%s", orgID, entity, id)
}
```

### Pattern 4: Cursor pagination tuple comparison + LIMIT +1 sentinel (D-63)

```sql
-- name: ListAgents :many
-- enabled=true filter (default-list path per D-65)
SELECT id, org_id, external_id, name, email, enabled, version, created_at, updated_at
FROM agents
WHERE org_id = $1
  AND enabled = TRUE
  AND ($2::timestamptz IS NULL OR (created_at, id) < ($2, $3))
  AND ($4::text IS NULL OR lower(name) LIKE '%' || lower($4) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $5;

-- name: ListAgentsIncludingDisabled :many
SELECT id, org_id, external_id, name, email, enabled, version, created_at, updated_at
FROM agents
WHERE org_id = $1
  AND ($2::timestamptz IS NULL OR (created_at, id) < ($2, $3))
  AND ($3::text IS NULL OR lower(name) LIKE '%' || lower($3) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $5;
```

```go
// Source: D-63 LIMIT N+1 sentinel pattern + cursor encoding
type cursorPayload struct {
    CreatedAt time.Time `json:"created_at"`
    ID        uuid.UUID `json:"id"`
}

func encodeCursor(c cursorPayload) (string, error) {
    b, err := json.Marshal(c)
    if err != nil { return "", err }
    return base64.StdEncoding.EncodeToString(b), nil
}

func decodeCursor(s string) (*cursorPayload, error) {
    if s == "" { return nil, nil }
    b, err := base64.StdEncoding.DecodeString(s)
    if err != nil { return nil, errBadCursor }
    var c cursorPayload
    if err := json.Unmarshal(b, &c); err != nil { return nil, errBadCursor }
    return &c, nil
}

// In ListAgents handler:
const defaultPageSize = 25
const maxPageSize = 100
pageSize := defaultPageSize
if req.Params.PageSize != nil {
    pageSize = int(*req.Params.PageSize)
}
if pageSize < 1 || pageSize > maxPageSize {
    return api.ListAgents400JSONResponse{...}, nil
}

cur, err := decodeCursor(deref(req.Params.Cursor))
if err != nil { return api.ListAgents400JSONResponse{...}, nil }

rows, err := q.ListAgents(ctx, generated.ListAgentsParams{
    OrgID:           pgUUID(orgID),
    CursorCreatedAt: pgTimestamptzPtr(cur),  // nil → NULL → IS NULL branch
    CursorID:        pgUUIDPtr(cur),
    NameFilter:      ptrOrNil(req.Params.Name),
    Limit:           int32(pageSize + 1), // LIMIT N+1
})
hasMore := len(rows) > pageSize
if hasMore { rows = rows[:pageSize] }
var nextCursor *string
if hasMore {
    nc, _ := encodeCursor(cursorPayload{CreatedAt: rows[len(rows)-1].CreatedAt.Time, ID: uuid.UUID(rows[len(rows)-1].ID.Bytes)})
    nextCursor = &nc
}
return api.ListAgents200JSONResponse{
    Items:      mapAgentListItems(rows),
    NextCursor: nextCursor,
    HasMore:    hasMore,
}, nil
```

### Pattern 5: Cross-row validation BEFORE write (D-76) — 422 invalid_reference

```go
// channels.go — CreateChannel checks default_queue_id exists + enabled in same org
func (h *Handlers) CreateChannel(ctx context.Context, req api.CreateChannelRequestObject) (api.CreateChannelResponseObject, error) {
    orgID, _ := orgkey.OrgIDFromContext(ctx)
    q := generated.New(h.deps.OrgDB)

    if req.Body.DefaultQueueId != nil {
        exists, err := q.QueueExistsAndEnabledInOrg(ctx, generated.QueueExistsAndEnabledInOrgParams{
            ID:    pgUUID(*req.Body.DefaultQueueId),
            OrgID: pgUUID(orgID),
        })
        if err != nil { return api.CreateChannel500JSONResponse{...}, nil }
        if !exists {
            // Phase 3 spec amendment (D-75) added api.ErrorCodeInvalidReference
            return api.CreateChannel422JSONResponse{ErrorResponse: api.ErrorResponse{
                Error: api.ErrorCodeInvalidReference, Reason: "default_queue_id_not_found_or_disabled"}}, nil
        }
    }
    // … INSERT …
}
```

### Pattern 6: agent_skills full-replace in UpdateAgent (CAT-03)

```go
// PUT-semantics on the skills join (OQ-1A resolution per Phase 2)
// agents.go — UpdateAgent when req.Body.Skills != nil
if req.Body.Skills != nil {
    // Cross-row check: every skill_id must exist in the same org
    ids := make([]pgtype.UUID, len(*req.Body.Skills))
    for i, s := range *req.Body.Skills {
        ids[i] = pgUUID(s.SkillId)
    }
    missing, err := q.SkillsMissingInOrg(ctx, generated.SkillsMissingInOrgParams{IDs: ids, OrgID: pgUUID(orgID)})
    if err != nil { /* 500 */ }
    if len(missing) > 0 {
        return api.UpdateAgent422JSONResponse{ErrorResponse: api.ErrorResponse{
            Error: api.ErrorCodeInvalidReference, Reason: fmt.Sprintf("unknown_skill_id:%s", missing[0])}}, nil
    }

    // Atomic replace inside a tx
    tx, err := h.deps.Pool.Begin(ctx)
    if err != nil { /* 500 */ }
    defer tx.Rollback(ctx)
    qtx := generated.New(tx) // sqlc.DBTX satisfied by pgx.Tx
    if _, err := qtx.DeleteAgentSkills(ctx, generated.DeleteAgentSkillsParams{AgentID: pgUUID(req.Id), OrgID: pgUUID(orgID)}); err != nil { /* 500 */ }
    for _, s := range *req.Body.Skills {
        if _, err := qtx.InsertAgentSkill(ctx, generated.InsertAgentSkillParams{
            AgentID: pgUUID(req.Id), SkillID: pgUUID(s.SkillId),
            Proficiency: int32(s.Proficiency), OrgID: pgUUID(orgID),
        }); err != nil { /* 500 */ }
    }
    if err := tx.Commit(ctx); err != nil { /* 500 */ }
}
```

**Note:** the agent skills replace must wrap the agent UPDATE itself in the same transaction so a partial failure doesn't leave the agent updated but skills divergent. Planner should call this out as a single tx.

### Anti-Patterns to Avoid

- **Caching list responses.** D-49 + REQ-CAT-11 limit cache to single-entity GETs. List endpoints are paginated and parameter-sensitive — keys would explode and invalidation would be intractable.
- **Returning `pgx.ErrNoRows` from `load` and caching the empty value.** `GetOrSet` MUST recognize `cache.ErrNotFound` as a propagated sentinel and NOT call `SET`. (D-54: no negative caching.)
- **Using `context.Background()` for the refresh goroutine.** Loses trace correlation and bypasses the OrgContext span. **Use `context.WithoutCancel(ctx)`** (Go 1.21+) so the trace context and OTel span propagate while cancel does NOT.
- **Hand-rolling SQL escaping for the `name` filter.** Always use `$N` parameters with `lower(name) LIKE '%' || lower($1) || '%'`. The orgDB SQLChecker parses via `pg_query_go` and rejects unsafe shapes; sqlc emits prepared statements; never concatenate user input into SQL.
- **Falling through to `middleware.WriteError` from inside a strict handler.** Phase 2's contract is typed `*JSONResponse` returns only. `WriteError` is for middleware-level short-circuits (e.g., OrgContext rejecting `X-Org-Id`).
- **Forgetting to add new generated `*JSONResponse` types to `injectRequestIDIntoErrorResponse` switch.** The Phase 2 exhaustiveness test in `request_id_exhaustiveness_test.go` fires the build. If Phase 3 adds 422 responses to existing endpoints (D-75), the new wrapper types like `CreateChannel422JSONResponse` MUST be added to the switch.
- **Building cache keys with the org_id from path instead of ctx.** FOUND-08 leakage guard — always read `orgID` from `orgkey.OrgIDFromContext(ctx)`, never from `req.OrgId` URL param.
- **Treating `CreateAgent409JSONResponse` like a plain struct.** It's a union type (`union json.RawMessage`); use `.FromErrorResponse(...)` or `.FromVersionConflictErrorResponse(...)` builder methods (see `types.gen.go:1199 AsErrorResponse + FromErrorResponse`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Concurrent cache miss dedup | A per-key `sync.Mutex` registry | `golang.org/x/sync/singleflight.Group` | Already standard-library-adjacent, race-tested, semantics for `Forget` + `DoChan` for refresh-ahead |
| Cursor encoding | Custom binary format | `base64.StdEncoding.EncodeToString(json.Marshal(...))` per D-63 | Opaque to clients, server-decodable, no custom codec to fuzz |
| JSON encoder for cache payloads | A custom marshaler that skips null fields | `encoding/json` (D-60) | OpenAPI omitempty pointers handle null fields; stdlib JSON parses generated DTOs round-trip-safely |
| Optimistic concurrency loop | Read-modify-write with retries | Single atomic UPDATE with `WHERE version = $N RETURNING *` (D-66) | One round-trip happy path; fallback SELECT only on miss; no retry storms |
| Soft-delete partial index | Materialized view "active_agents" | Postgres partial index `WHERE enabled = true` (D-64) | Index-only-scan friendly; same table, no view-refresh worry |
| SQL parsing for org_id enforcement | Regex over query strings | `pg_query_go` via `internal/db/sqlcheck.go` (already in place) | Real Postgres parser, ASTs not strings; cached by SHA-256; rejects projection/JOIN-only org_id |
| Strict-server response types | Hand-write `(w, r)` handlers | oapi-codegen v2.7.0 strict-server (Phase 2 D-41) | Compile-time enforcement that response shape matches spec; type-switch in `injectRequestIDIntoErrorResponse` is the integration seam |
| Test Redis fixture | Mock interface + manual records | `github.com/alicebob/miniredis/v2` v2.38.0 | In-process, real RESP protocol, supports PTTL/FastForward for refresh-ahead tests, MIT |
| In-process Postgres for tests | Mock interface | testcontainers-go (already wired in Phase 1) | Real migrations, real SQL, real pgx behavior |
| JSON deep-merge for partial PATCH | Build a merge engine | Each `UpdateXRequest` field is a pointer; `if req.Body.Name != nil { ... }` | OpenAPI generates pointer fields for optional updates — patch semantics are direct |
| Redis SETNX cross-instance lock | Build a distributed lock manager | Per-process singleflight (D-52); accept bounded fan-out | Cross-instance dedup is v0.2 scope; one DB load per Go process per key per miss is acceptable |

**Key insight:** Phase 3 looks like a lot of code but most of the heavy lifting is already done by Phase 1/2 infrastructure (strict-server, orgDB, codegen, exhaustiveness test). Writing the catalog package is mostly "wire ctx → sqlc → mapper → response" with cache wrapping reads and FK checks wrapping writes. Resist building anything bespoke for cache, cursor, optimistic concurrency, or validation — Phase 2's contract + sqlc + the cache pattern above give you everything.

## Runtime State Inventory

> N/A — Phase 3 is a greenfield handler/schema phase, NOT a rename/refactor. Recording explicit "none" verdicts per the protocol:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no existing catalog data at deployment (greenfield migration 002). The only existing data is the Phase 1 `_scaffold` table which is dropped in this phase's migration; no test data persists across `task db:reset`. | Migration 002 down.sql must recreate `_scaffold` for safe rollback during v0.1 dev. |
| Live service config | None — no external services (Datadog, n8n, Tailscale) consume Phase 3 entity IDs. The only consumer is the in-tree admin SPA (Phase 6, not yet built). | None |
| OS-registered state | None — no systemd units, launchd plists, or pm2 saved processes reference catalog entity names. | None |
| Secrets/env vars | None — no env var encodes a catalog entity name. `REDIS_URL`/`DATABASE_URL` are unchanged. | None |
| Build artifacts | None — sqlc emits to `internal/db/generated/*.sql.go` which is regenerated from queries; old `scaffold.sql.go` is removed by sqlc when its source `.sql` is deleted. `services/api/internal/api/*.gen.go` is regenerated by codegen-drift on the spec edit. | Run `task gen` after spec amendment + sqlc query changes to refresh artifacts. |

**Canonical question — "After every file in the repo is updated, what runtime systems still have the old string cached, stored, or registered?"** Answer: nothing. The only persistent state is in PostgreSQL (`_scaffold` table — dropped by migration 002 in dev) and Redis (no Phase 1/2 cache entries written; empty key space). Both are managed via migration + runtime cache invalidation.

## Common Pitfalls

### Pitfall 1: `injectRequestIDIntoErrorResponse` exhaustiveness regression
**What goes wrong:** Spec amendment (D-75) generates new `CreateChannel422JSONResponse`, `UpdateChannel422JSONResponse`, etc. The type switch in `server.go:injectRequestIDIntoErrorResponse` doesn't have cases for them. `request_id_exhaustiveness_test.go` fails the build.
**Why it happens:** The Phase 2 hardening test was added *specifically* to catch this. Every new generated error response type needs a switch case.
**How to avoid:** After each codegen regen, run `go test ./internal/server -run TestRequestIDInjection_Exhaustiveness`. Add 422 cases for whichever entities gain FK validation in Wave 0.
**Warning signs:** Build failure on `TestRequestIDInjection_Exhaustiveness`; missing types in the switch's `case` list compared to `grep "type .*JSONResponse" services/api/internal/api/server.gen.go`.

### Pitfall 2: `CreateAgent409JSONResponse` is a union, not a struct
**What goes wrong:** Writing `return api.CreateAgent409JSONResponse{Error: "version_conflict", ...}, nil` fails to compile or produces `{"union":"<base64>"}` garbage on the wire.
**Why it happens:** The spec defines the 409 body as `oneOf: ErrorResponse | VersionConflictErrorResponse`, so oapi-codegen generates a union wrapper at `types.gen.go:1066 CreateAgent409JSONResponseBody { union json.RawMessage }` with `.AsErrorResponse() / .FromErrorResponse(v ErrorResponse)` accessors. Channel/Queue/Skill 409s are flat structs (single schema in their spec). Agent's is the only union.
**How to avoid:** For CreateAgent 409 path: build an `ErrorResponse`, call `.FromErrorResponse(...)`, return that. Add a unit test that round-trips a CreateAgent 409 through `json.Marshal/Unmarshal` and asserts the wire shape matches `{"error":"version_conflict", "reason":"...", "request_id":"..."}`.
**Warning signs:** `CreateAgent409JSONResponse{...struct lit...}` won't compile because the type is `type CreateAgent409JSONResponse = CreateAgent409JSONResponseBody`. Always use the `From*` builder.

### Pitfall 3: cursor + nullable parameters in sqlc fail at runtime, not compile
**What goes wrong:** sqlc generates `Cursor pgtype.Timestamptz` (Valid bool); passing zero value works but the `($2::timestamptz IS NULL OR ...)` branch behaves unexpectedly because `pgtype.Timestamptz{Valid: false}` serializes as NULL. Confirm with `sqlc.narg('cursor_at')` if needed.
**Why it happens:** sqlc 1.31.1 + `emit_pointers_for_null_types: true` makes nullable Postgres types render as `*pgtype.Timestamptz` in the params struct — fine, but only if the SQL uses `sqlc.narg('cursor_at')` to declare nullability.
**How to avoid:** Use the `sqlc.narg('cursor_at')` annotation in `agents.sql` for cursor params. Test the no-cursor (first page) path AND the with-cursor path in `agents_test.go`.
**Warning signs:** First page returns 0 rows; or cursor encoding round-trip drops microseconds; or `WHERE (created_at, id) < (NULL, NULL)` evaluates to NULL (not TRUE) and silently filters everything.

### Pitfall 4: Refresh-ahead goroutine outliving HTTP request causes ctx leak
**What goes wrong:** Using `context.Background()` for the refresh goroutine loses trace_id; using the request ctx kills the goroutine when the response writes. Some code paths might use OrgDB which checks `orgkey.OrgIDFromContext` — and `context.WithoutCancel` keeps the org_id intact, but a naive `context.Background()` strips it and orgDB.preflight returns `ErrOrgIDMissingFromContext`, panicking in dev/test (ValidationPanic mode).
**Why it happens:** `context.WithoutCancel(ctx)` (stdlib, Go 1.21+) is the correct primitive: same values, no cancel. `context.Background()` strips all values.
**How to avoid:** Use `bgCtx := context.WithoutCancel(ctx)` in the refresh goroutine. Add a test that asserts the refresh goroutine successfully completes a DB load (use miniredis + Postgres testcontainer; trigger refresh by `s.FastForward(50 * time.Second)` to drop PTTL into the <10s window).
**Warning signs:** Tests panic in goroutine with `orgdb: ctx missing org_id; refusing to query`. Or: refresh never fires because the test client's request ctx already cancelled.

### Pitfall 5: agent_skills replace without single transaction → partial state on failure
**What goes wrong:** Handler does `DeleteAgentSkills` (succeeds), then INSERTs 3 new skills (the 2nd fails); agent now has 1 of 3 skills and the old set is lost.
**Why it happens:** No explicit transaction; each sqlc call is auto-committed by pgxpool.
**How to avoid:** Wrap UpdateAgent in a transaction: `tx := pool.Begin(); qtx := generated.New(tx); ...; tx.Commit()`. The agent UPDATE itself must be in the same tx so a version-mismatch later doesn't leave skills changed. (Note: orgDB wraps Pool; you'll need to either pass the raw pool through Deps for tx, or extend orgDB with a `BeginTx` that returns an orgDB-wrapped tx. **Lean toward the latter** — keeps the validator engaged.)
**Warning signs:** Concurrent UpdateAgent + GetAgent races leave the cache and DB inconsistent; test by injecting an error mid-INSERT and asserting `q.GetAgentSkills(agentID)` returns the original set.

### Pitfall 6: Spec param rename (`limit` → `page_size`) silently breaks generated TS client
**What goes wrong:** Wave 0 renames the OpenAPI query param. `go generate` regenerates Go (build still works because we update handlers). `pnpm gen:api` regenerates TS — but the admin SPA isn't written yet (Phase 6), so we don't notice the rename until Phase 6 starts.
**Why it happens:** Drift CI only checks that `task gen` produces zero diff against committed code. It can't catch a name change consumers depend on if the consumer doesn't exist yet.
**How to avoid:** Just commit the change cleanly; Phase 6 will see the renamed type. **Or**: keep param name `limit` and only change `default: 20` → `default: 25`. The locked decision D-67 says "Page size cap: 25 default, 100 max" — it does not mandate the param NAME. Recommend: edit only `default: 20` → `default: 25`, leave param name `limit`. Add a note in the plan that Phase 6 may rename later if UX favors `page_size`.
**Warning signs:** None visible until Phase 6 starts. Mitigation is the conservative spec edit (default only, not name).

### Pitfall 7: ILIKE on `lower(name)` doesn't use the functional index
**What goes wrong:** Functional partial index `CREATE INDEX ON agents(org_id, lower(name) text_pattern_ops) WHERE enabled = true` is created, but the query plan still does Seq Scan because `lower(name) LIKE '%foo%'` (leading `%`) cannot use a btree.
**Why it happens:** btree text_pattern_ops only supports prefix matches (`LIKE 'foo%'`). Substring `'%foo%'` needs a trigram index. D-64 explicitly accepts this at v0.1 scale (<1K rows/org).
**How to avoid:** Document the limitation in the migration comment. Recommend `pg_trgm` + GIN index when scale crosses 10K rows. v0.1 plan: don't expect index usage for `name=` searches — accept Seq Scan.
**Warning signs:** `EXPLAIN ANALYZE` shows Seq Scan for `name=foo` queries. Expected at v0.1; flag when CAT row counts approach 5K+.

### Pitfall 8: Adapter/Channel `version` field divergence from other entities
**What goes wrong:** Spec (verified at lines 661-749 and 751-839 in `openapi.yaml`) has NO `version` field on Channel and Adapter. CAT-08 says "all update endpoints require a `version` field" — but the spec doesn't define one for those two. The 409 wrapper types (`UpdateChannel409`, `UpdateAdapter*`) also lack `current` field on Channel (verified server.gen.go).
**Why it happens:** Phase 2 spec authoring stopped short of `version` on Channel/Adapter because the source models hadn't been finalized.
**How to avoid:** Decide in Wave 0: (a) add `version` field to Channel + Adapter in the spec amendment (matches D-66 universal pattern), or (b) keep parity with current spec — Channel/Adapter have no optimistic concurrency in v0.1 (last-write-wins). Recommend (a) for consistency: add `version` to both schemas, plus `UpdateChannel409` typed response with `current: Channel`. This is a clean spec amendment with no client breakage (clients ignore the new field if they don't use it).
**Warning signs:** REQ-CAT-08 acceptance test ("a Channel UPDATE with stale version returns 409") fails because there's no version path.

### Pitfall 9: pgtype JSONB sqlc emits `[]byte`, not `map[string]any`
**What goes wrong:** Adapter `config` JSONB column. sqlc v1.31.1 with `pgx/v5` defaults to `[]byte` for JSONB. Handler must `json.Marshal(req.Body.Config)` on write and `json.Unmarshal(row.Config, &adapter.Config)` on read.
**Why it happens:** sqlc + pgx/v5 default mapping is `[]byte` so the driver round-trips raw bytes — the application owns the schema. (Confirmed via sqlc datatypes docs.)
**How to avoid:** Write mapper helpers `marshalAdapterConfig(map[string]any) []byte` and `unmarshalAdapterConfig([]byte) map[string]any`. Test with nil/empty/nested map cases.
**Warning signs:** "cannot scan map[string]interface{} into []byte" or vice versa at runtime.

## Code Examples

### Common Operation 1: Build cursor pagination response

```go
// Source: D-63 + LIMIT N+1 sentinel
const pageSize = 25
rows, _ := q.ListAgents(ctx, generated.ListAgentsParams{
    OrgID: pgUUID(orgID),
    Limit: int32(pageSize + 1), // +1 sentinel
})
hasMore := len(rows) > pageSize
if hasMore { rows = rows[:pageSize] }

var nextCursor *string
if hasMore {
    last := rows[len(rows)-1]
    enc, _ := encodeCursor(cursorPayload{CreatedAt: last.CreatedAt.Time, ID: uuid.UUID(last.ID.Bytes)})
    nextCursor = &enc
}
return api.ListAgents200JSONResponse{
    Items:      mapAgentListItems(rows),
    NextCursor: nextCursor,
    HasMore:    hasMore,
}, nil
```

### Common Operation 2: Soft-delete via UPDATE enabled=false

```go
// Source: pattern derived from D-65 (List defaults to enabled=true; DELETE = UPDATE enabled=false)
func (h *Handlers) DeleteAgent(ctx context.Context, req api.DeleteAgentRequestObject) (api.DeleteAgentResponseObject, error) {
    orgID, _ := orgkey.OrgIDFromContext(ctx)
    q := generated.New(h.deps.OrgDB)
    tag, err := q.SoftDeleteAgent(ctx, generated.SoftDeleteAgentParams{
        ID:    pgUUID(req.Id),
        OrgID: pgUUID(orgID),
    })
    if err != nil { return api.DeleteAgent500JSONResponse{...}, nil }
    if tag.RowsAffected() == 0 {
        return api.DeleteAgent404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
            Error: api.ErrorCodeNotFound, Reason: "agent_not_found"}}, nil
    }
    // Invalidate cache after commit (D-55)
    if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "agents", req.Id)); delErr != nil {
        h.deps.Logger.WarnContext(ctx, "cache del failed", "err", delErr)
    }
    return api.DeleteAgent204Response{}, nil
}
```

```sql
-- agents.sql
-- name: SoftDeleteAgent :execrows
UPDATE agents SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;
```

(Note: filtering `enabled = TRUE` in the UPDATE makes idempotent — re-deleting already-disabled returns 0 rows → 404, which matches the "not found" semantics for a soft-delete-once-then-gone client view.)

### Common Operation 3: Migration 002 skeleton (agents + agent_skills only — abbreviated)

```sql
-- migrations/000002_catalog_v0_1.up.sql
-- D-61: single editable migration for the entire v0.1 milestone.
-- Phase 4 adds agent_states; Phase 5 adds import_jobs. Freeze post-deploy.

BEGIN;

-- Drop Phase 1 scaffold (D-77)
DROP TABLE IF EXISTS _scaffold;

CREATE TABLE agents (
    id            UUID PRIMARY KEY,
    org_id        UUID NOT NULL,
    external_id   TEXT NOT NULL,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    version       INTEGER NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, external_id)
);
CREATE INDEX ix_agents_org_created ON agents (org_id, created_at DESC, id DESC);
CREATE INDEX ix_agents_org_name ON agents (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

CREATE TABLE skills (
    id            UUID PRIMARY KEY,
    org_id        UUID NOT NULL,
    external_id   TEXT NOT NULL,
    name          TEXT NOT NULL,
    description   TEXT,
    skill_type    TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    version       INTEGER NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, external_id)
);
CREATE INDEX ix_skills_org_created ON skills (org_id, created_at DESC, id DESC);
CREATE INDEX ix_skills_org_name ON skills (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

CREATE TABLE queues (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    external_id     TEXT NOT NULL,
    name            TEXT NOT NULL,
    channel_types   TEXT[] NOT NULL DEFAULT '{}',
    priority        INTEGER NOT NULL DEFAULT 0,
    acw_sec         INTEGER NOT NULL DEFAULT 0,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, external_id)
);
CREATE INDEX ix_queues_org_created ON queues (org_id, created_at DESC, id DESC);
CREATE INDEX ix_queues_org_name ON queues (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

CREATE TABLE channels (
    id                 UUID PRIMARY KEY,
    org_id             UUID NOT NULL,
    external_id        TEXT NOT NULL,
    name               TEXT NOT NULL,
    channel_type       TEXT NOT NULL,
    default_queue_id   UUID,             -- nullable; cross-row validated by app (D-76), NO db FK in v0.1
    enabled            BOOLEAN NOT NULL DEFAULT TRUE,
    version            INTEGER NOT NULL DEFAULT 1, -- Pitfall 8: recommend add for CAT-08 parity
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, external_id)
);
CREATE INDEX ix_channels_org_created ON channels (org_id, created_at DESC, id DESC);
CREATE INDEX ix_channels_org_name ON channels (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

CREATE TABLE adapters (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    name            TEXT NOT NULL,
    adapter_type    TEXT NOT NULL,
    config          JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    version         INTEGER NOT NULL DEFAULT 1, -- Pitfall 8: recommend add
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
    -- NB: adapters has no external_id per Phase 2 spec
);
CREATE INDEX ix_adapters_org_created ON adapters (org_id, created_at DESC, id DESC);
CREATE INDEX ix_adapters_org_name ON adapters (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

CREATE TABLE break_reasons (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    name            TEXT NOT NULL,
    routable        BOOLEAN NOT NULL DEFAULT FALSE,
    display_order   INTEGER NOT NULL DEFAULT 0,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, name)  -- break_reasons has no external_id; name is the user-facing key
);
CREATE INDEX ix_break_reasons_org_created ON break_reasons (org_id, created_at DESC, id DESC);
CREATE INDEX ix_break_reasons_org_display ON break_reasons (org_id, display_order, id) WHERE enabled = TRUE;

CREATE TABLE agent_skills (
    agent_id        UUID NOT NULL,
    skill_id        UUID NOT NULL,
    org_id          UUID NOT NULL,    -- denormalized to satisfy orgDB SQLChecker on every query
    proficiency     INTEGER NOT NULL CHECK (proficiency BETWEEN 1 AND 10),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, skill_id)
);
CREATE INDEX ix_agent_skills_org_agent ON agent_skills (org_id, agent_id);
CREATE INDEX ix_agent_skills_org_skill ON agent_skills (org_id, skill_id);

COMMIT;
```

```sql
-- migrations/000002_catalog_v0_1.down.sql
BEGIN;
DROP TABLE IF EXISTS agent_skills;
DROP TABLE IF EXISTS break_reasons;
DROP TABLE IF EXISTS adapters;
DROP TABLE IF EXISTS channels;
DROP TABLE IF EXISTS queues;
DROP TABLE IF EXISTS skills;
DROP TABLE IF EXISTS agents;

-- Recreate _scaffold for rollback safety (matches 000001_create_scaffold.up.sql)
CREATE TABLE _scaffold (
    id          UUID PRIMARY KEY,
    org_id      UUID NOT NULL,
    external_id TEXT NOT NULL,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, external_id)
);
CREATE INDEX idx_scaffold_org_id ON _scaffold (org_id);
COMMIT;
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Offset-based pagination (LIMIT/OFFSET) | Keyset/cursor pagination with `(created_at, id) < ($1, $2)` composite tuple comparison | UUIDv7 RFC 9562 + Postgres tuple index support | O(1) page-N performance regardless of depth; stable under inserts (UUIDv7 time-ordering) [CITED: bun.uptrace.dev cursor pagination guide; oneuptime.com Go pagination article] |
| `pgx/v4` + `database/sql` | `pgx/v5` native pool + sqlc emit `pgx/v5` types | sqlc 1.18+ default | Cleaner types (`pgtype.UUID`, `pgtype.Timestamptz`), no `database/sql` shim except for golang-migrate (Pitfall 7 in Phase 1) [VERIFIED: services/api/sqlc.yaml sql_package: pgx/v5] |
| Singleton cache client + ad-hoc per-key mutexes | `cache.Cache` struct + `singleflight.Group` + `GetOrSet[T any]` generic | Go 1.18 generics + `golang.org/x/sync/singleflight` v0.x | Type-safe cached values, dedup'd misses, refresh-ahead via `context.WithoutCancel` (Go 1.21+) [CITED: 1xapi.com cache stampede 2026 guide, medium pickme-engineering singleflight] |
| RFC 7807 problem-details errors | Custom `{error, reason, request_id}` envelope | Phase 1 D-35 lock (no spec churn) | Stable shape across endpoints, `error` is closed enum, no `application/problem+json` content type changes [VERIFIED: openapi.yaml ErrorResponse schema] |
| Per-endpoint hand-written handlers | oapi-codegen strict-server | Phase 2 D-41 | Compile-time enforcement that response matches spec; type switch in `injectRequestIDIntoErrorResponse` is the integration seam |

**Deprecated/outdated:**
- `pgtype.JSONB` (sqlc + pgx/v5) — current is `[]byte` default; opt-in to custom struct via sqlc overrides if needed (Pitfall 9).
- `context.Background()` for "fire-and-forget" goroutines — Go 1.21 introduced `context.WithoutCancel(ctx)` which preserves values (`org_id`, trace span) while detaching cancel. This is the correct primitive for refresh-ahead goroutines (D-53).
- Hand-rolled SQL escaping for ILIKE — sqlc generates prepared statements; never string-concat user input.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `golang.org/x/sync v0.20.0` is the latest stable singleflight implementation; lower-bound the dep at this version. Phase 1 already pinned it as indirect. | Standard Stack | Compile-only risk; downgrade behavior identical for the surface area we use. |
| A2 | `github.com/alicebob/miniredis/v2 v2.38.0` (May 2026) is current and supports go-redis v9 implicitly via the TCP RESP interface. WebFetch confirms MIT license + ongoing maintenance; slopcheck flagged false-positive based on stale pkg.go.dev "no source repo" metadata. | Standard Stack | Low — package has been live since 2015 and is the canonical Go Redis test double. Planner inserts a one-line human-verify checkpoint before `go get` per defense-in-depth policy. |
| A3 | sqlc 1.31.1 + pgx/v5 mapping for JSONB columns emits `[]byte` by default and round-trips byte-for-byte without overrides. We have NOT explicitly verified against the in-tree sqlc.yaml — relying on web search of sqlc docs. | Pitfall 9 | Medium — if sqlc emits `pgtype.Bytea` or a different shape, mapper code needs adjustment. Wave 1 first sqlc gen will reveal this at compile time. |
| A4 | Recommend adding `version INTEGER` to Channel and Adapter for CAT-08 parity even though the current OpenAPI spec lacks it. The user has NOT explicitly approved this; CONTEXT.md D-66 says "atomic UPDATE with version check" applies "for every entity UPDATE" which we read as universal. | Pitfall 8 | Medium — if user wants to keep parity with current spec (no version on Channel/Adapter), planner must either (a) confirm CAT-08 doesn't apply to those entities, or (b) document the v0.1 exception. Surface in discuss-phase. |
| A5 | The OpenAPI param `limit` (default 20, max 100) needs editing to `default 25` to match D-67. We recommend keeping the param NAME `limit` (not renaming to `page_size`) to minimize TS client churn. CONTEXT.md says "?page_size=N" but doesn't explicitly require that name in the URL — D-67 wording is about defaults/caps, not naming. | CAT-10 row + Pitfall 6 | Low — both options work; recommended approach is minimal spec edit. |
| A6 | `context.WithoutCancel(ctx)` is the correct primitive for refresh-ahead goroutines (Go 1.21+). We verified Go 1.25 is the project target (go.mod). Behavior under Go 1.21 backports is identical. | Pattern 3, Pitfall 4 | Low — stdlib primitive, well-documented behavior. |
| A7 | Phase 4 (state machine) + Phase 5 (bulk import) methods (`PatchAgentStatus`, `GetAgentStatus`, `BulkImportCatalog`, `GetImportJob`) can remain 501 stubs on `catalog.Handlers` until those phases land — they will NOT be embedded via the composite struct in Phase 3. D-70 confirms this. | D-69, D-70 | None — explicit in context. |
| A8 | The agent_skills full-replace semantics on UpdateAgent need a single transaction wrapping the agent UPDATE + delete-all-skills + insert-new-skills. CONTEXT.md does not explicitly specify "must be one tx" but the PUT-semantics OQ-1A resolution implies atomicity. | Pattern 6, Pitfall 5 | Medium — without a single tx, partial failure leaves data inconsistent. Plan should call this out explicitly and have a test that injects a mid-INSERT error. |

**If user wants to override any A* item, they should raise it in discuss-phase before Wave 0 plans land.**

## Open Questions

1. **Should Channel and Adapter gain a `version` field for CAT-08 parity (Pitfall 8 / A4)?**
   - What we know: REQ-CAT-08 says "all update endpoints require a `version` field" but the spec authored in Phase 2 omits `version` from Channel and Adapter schemas.
   - What's unclear: Was that an intentional v0.1 exception or a Phase 2 oversight?
   - Recommendation: Spec-amend to ADD `version` field to Channel and Adapter for consistency. Surface in plan-check.

2. **Page-size parameter naming: keep `limit` or rename to `page_size`?**
   - What we know: Generated `ListXParams` uses `Limit *LimitQuery`. D-67 mandates "default 25, max 100" but doesn't name the param.
   - What's unclear: Is the name a contract concern? Phase 6 admin SPA might expect `page_size` based on D-58 cache-key wording style.
   - Recommendation: Edit `default: 20 → 25` only; keep param name `limit` (minimum churn).

3. **adapter_type, channel_type, skill_type free-text or constrained?**
   - What we know: Spec says `adapter_type` is "Free text — the platform does not restrict values in v0.1." Same for `skill_type`. `channel_type` IS constrained to `ChannelType` enum {voice, chat, email}.
   - What's unclear: Should the spec amendment in Wave 0 add `pattern: '^[a-z][a-z0-9_]*$'` for adapter_type/skill_type to prevent UI breakage from arbitrary strings?
   - Recommendation: Leave free-text in v0.1. Add a validator at the handler level only if a real consumer complains.

4. **Should the catalog handler's `orgDB` come from a factory per-request, or a shared singleton?**
   - What we know: Phase 1 D-25 establishes `db.OrgDBFromContext(ctx)` per-request pattern. Existing scaffold handler uses a shared `*db.OrgDB` constructed in main.go and passed via constructor.
   - What's unclear: D-71 says `Deps.OrgDBFactory db.Factory` (a factory) — but there's no `db.Factory` type yet; current code uses a shared `*db.OrgDB`. Is this a rename of the existing pattern or a new abstraction?
   - Recommendation: Use the existing shared `*db.OrgDB` field (matches scaffold handler). The "factory" wording in D-71 is from CONTEXT.md and likely refers to the shared instance that emits per-request `generated.New(orgDB)`. Plan-check should align.

5. **Where does the agent UPDATE + skills replace transaction get its `pgx.Tx`?**
   - What we know: `orgDB` wraps `*pgxpool.Pool` and implements `DBTX`. `pgx.Tx` also implements `DBTX`. sqlc `generated.New(dbtx DBTX)` accepts either.
   - What's unclear: To preserve the orgDB SQLChecker, we'd need an `orgDB.BeginTx(ctx) (*orgTx, error)` method that wraps `pool.Begin` and returns an orgDB-flavored tx (validating SQL on each Exec). Phase 1 didn't add this.
   - Recommendation: Either (a) add `orgDB.BeginTx` in Wave 1 alongside cache pkg, or (b) bypass orgDB for the tx and rely on test-time SQL review. **Strongly recommend (a)** — preserves the FOUND-04 enforcement guarantee.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | All Go compilation | ✓ | go1.23.12 (system); go.mod targets 1.25 | **UPGRADE NEEDED** — system Go 1.23 cannot build a module with `go 1.25` directive. Either install Go 1.25 toolchain or run via Docker. |
| pnpm | Web codegen (Phase 6, also `pnpm gen:api` in Phase 3) | ✓ | 10.33.0 | — |
| sqlc | sqlc query codegen | ✓ | v1.31.1 | — |
| golang-migrate CLI | `task migrate-up`, `task db:reset` | ✓ | dev (locally built) | — |
| Docker | Postgres + Redis (compose) + testcontainers | ✓ | 29.4.0 | — |
| PostgreSQL 17 | All catalog persistence | ✓ via docker-compose | 17.x | — |
| Redis | Cache layer (CAT-11) | ✓ via docker-compose | 7.x | — |
| redocly CLI | spec lint (existing codegen-drift CI) | runs via `npx @redocly/cli@latest` per Phase 2 D-48 | latest (2.30.5+ as of 2026-05-16) | — |
| testcontainers-go | Integration tests | ✓ (in go.mod) | v0.42.0 | — |
| miniredis (test-only) | cache unit tests | ✗ (new dep to add) | target v2.37.0+ | None — must add |

**Missing dependencies with no fallback:**
- Go 1.25 toolchain — local Go is 1.23; planner must add a Wave 0 task to install/verify Go 1.25 on dev machines OR confirm CI uses `setup-go@v5` with `go-version: '1.25.0'` (Phase 1 D-30). Without 1.25, the existing repo already won't build locally.

**Missing dependencies with fallback:**
- miniredis — install via `go get -t github.com/alicebob/miniredis/v2@latest` as test-only. Fallback would be skipping cache unit tests in favor of testcontainer Redis (slower, but works).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/stretchr/testify v1.11.1` |
| Config file | none — Go convention; per-package `_test.go` files |
| Quick run command | `cd services/api && go test -count=1 -short ./internal/...` (skips testcontainer suites) |
| Full suite command | `cd services/api && go test -race -count=1 ./...` (matches `task test`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CAT-01 | CRUD agents end-to-end | integration | `go test ./internal/catalog -run TestAgents -race` | ❌ Wave 0 (new `catalog/agents_test.go`) |
| CAT-02 | CRUD skills end-to-end | integration | `go test ./internal/catalog -run TestSkills -race` | ❌ Wave 0 |
| CAT-03 | agent_skills assign + proficiency 1-10 | integration | `go test ./internal/catalog -run TestAgentSkills -race` | ❌ Wave 0 |
| CAT-04 | CRUD queues + channel_types[] | integration | `go test ./internal/catalog -run TestQueues -race` | ❌ Wave 0 |
| CAT-05 | CRUD channels + default_queue_id cross-org probe | integration | `go test ./internal/catalog -run TestChannels -race` | ❌ Wave 0 |
| CAT-06 | CRUD adapters + JSONB config round-trip | integration | `go test ./internal/catalog -run TestAdapters -race` | ❌ Wave 0 |
| CAT-07 | CRUD break_reasons + routable flag | integration | `go test ./internal/catalog -run TestBreakReasons -race` | ❌ Wave 0 |
| CAT-08 | Update returns 409 with current row on version mismatch | unit + integration | `go test ./internal/catalog -run TestVersionConflict -race` | ❌ Wave 0 — one per entity that has version |
| CAT-09 | Default list excludes enabled=false; `?include_disabled=true` surfaces them | unit | `go test ./internal/catalog -run TestSoftDeleteFilter -race` | ❌ Wave 0 |
| CAT-10 | Cursor pagination + name search + filter combinations | unit + integration | `go test ./internal/catalog -run TestCursorPagination -race` | ❌ Wave 0 |
| CAT-11 | Single-entity GET cached 60s under `or:{orgId}:{entity}:{id}`; write invalidates | unit (miniredis) | `go test ./internal/cache -race && go test ./internal/catalog -run TestCacheInvalidation -race` | ❌ Wave 0 — new `cache/cache_test.go` + per-entity catalog test |
| All — cross-org probe | integration | `go test ./test/isolation -run TestCatalog -race` | ❌ Wave 0 — extend `test/isolation/catalog_test.go` |
| All — `RequestIDInjection` exhaustiveness | unit | `go test ./internal/server -run TestRequestIDInjection_Exhaustiveness` | ✅ EXISTS — extends in Wave 0/1 |

### Sampling Rate
- **Per task commit:** `cd services/api && go test -count=1 -short ./internal/catalog/... ./internal/cache/...`
- **Per wave merge:** `task test` (full: `go test -race ./...`)
- **Phase gate:** `task ci` (lint + typecheck + full test) + `task gen && git diff --exit-code` (drift gate)

### Wave 0 Gaps
- [ ] `internal/cache/cache.go` — cache package skeleton (D-50) — covers REQ-CAT-11
- [ ] `internal/cache/cache_test.go` — miniredis-backed unit tests (hit/miss/refresh/del/ErrNotFound) — covers REQ-CAT-11
- [ ] `internal/catalog/handlers.go` — Deps struct + New(opts) constructor — scaffolding
- [ ] `internal/catalog/agents.go` + `agents_test.go` — covers REQ-CAT-01, CAT-03 (partial), CAT-08, CAT-09, CAT-10, CAT-11
- [ ] `internal/catalog/skills.go` + `skills_test.go` — covers REQ-CAT-02, CAT-08, CAT-09, CAT-10
- [ ] `internal/catalog/queues.go` + `queues_test.go` — covers REQ-CAT-04, CAT-08, CAT-09, CAT-10
- [ ] `internal/catalog/channels.go` + `channels_test.go` — covers REQ-CAT-05, CAT-09, CAT-10 (+ CAT-08 if version added)
- [ ] `internal/catalog/adapters.go` + `adapters_test.go` — covers REQ-CAT-06, CAT-09, CAT-10
- [ ] `internal/catalog/break_reasons.go` + `break_reasons_test.go` — covers REQ-CAT-07, CAT-08, CAT-09, CAT-10
- [ ] `internal/catalog/agent_skills.go` + `agent_skills_test.go` — covers REQ-CAT-03 (full)
- [ ] `internal/catalog/cursor.go` + tests — encoding/decoding base64 cursor — covers REQ-CAT-10
- [ ] `internal/catalog/mappers.go` — sqlc → api DTO converters
- [ ] `internal/catalog/errors.go` — pgconn error → strict response mapping
- [ ] `internal/catalog/testutil.go` — shared test setup (miniredis + pg testcontainer + handler factory)
- [ ] `services/api/test/isolation/catalog_test.go` — per-entity cross-org probe → 404 (D-72) — covers FOUND-08 extension
- [ ] sqlc query files (×7): `agents.sql`, `skills.sql`, `queues.sql`, `channels.sql`, `adapters.sql`, `break_reasons.sql`, `agent_skills.sql` (D-62)
- [ ] migration `000002_catalog_v0_1.up.sql` + `.down.sql` (D-61)
- [ ] Updated `injectRequestIDIntoErrorResponse` switch with all new 422 wrappers (build-gated by existing exhaustiveness test)

## Security Domain

> Required when `security_enforcement` is enabled (key absent from config.json — treat as enabled).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (stub) | `X-Org-Id` header via `OrgContext` middleware. Real JWT in v1 AUTH-01. |
| V3 Session Management | no | Stateless REST; no sessions in v0.1. |
| V4 Access Control | yes | orgDB SQL validator enforces `WHERE org_id = $N` at runtime; cross-org probe → 404 (FOUND-08); no RBAC in v0.1. |
| V5 Input Validation | yes | Layer 1: oapi-codegen schema validation (400 invalid_body). Layer 2: handler cross-row validation (422 invalid_reference per D-74/D-75). |
| V6 Cryptography | partial | UUIDv7 minted via `uuid.NewV7` (cryptographic randomness sourced from `crypto/rand`). No password hashing in v0.1. |
| V8 Logging & Monitoring | yes | slog JSON with trace_id, request_id, org_id (D-29). Cache outcomes logged at debug level (D-57). |
| V13 API Security | yes | OpenAPI 3.0 contract is single source of truth; drift CI blocks merge. Strict-server enforces request/response shapes at compile time. |

### Known Threat Patterns for chi + pgx + Redis stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Cross-org data leakage via path manipulation | Information Disclosure | Always read `org_id` from ctx (OrgContext middleware), never from path or body (FOUND-03/05); orgDB SQL validator rejects unscoped queries; cross-org probe returns 404 not 403 to avoid existence oracle |
| SQL injection via name search | Tampering | Parameterized queries only; ILIKE on `$N` placeholder; never string-concat; pg_query parser in orgDB checks AST shape |
| Cache poisoning across orgs | Information Disclosure | Cache key always namespaces by orgId (D-58); key format `or:{orgId}:{entity}:{id}` — wrong orgId in key path is a code review concern, no runtime risk |
| Mass assignment via PATCH | Elevation of Privilege | OpenAPI schemas list only allowed update fields; `org_id`, `created_at`, `updated_at`, `version` are read-only or server-controlled; pointer types make "not provided" distinct from "set to zero" |
| Version race on concurrent UPDATE | Tampering | D-66 single atomic UPDATE with `WHERE version = $N RETURNING *`; 0 rows → 404/409 disambiguation; D-56 cache DEL on 409 |
| Cursor manipulation to enumerate IDs | Information Disclosure (low) | Cursors are unsigned base64(JSON) — clients can craft cursors. Server validates `(created_at, id)` shape; queries still filter by org_id; worst case is observing one's own ID space. Cursor signing (HMAC) deferred to v0.2 hardening. |
| JSON unmarshal denial-of-service | Denial of Service | oapi-codegen rejects malformed bodies at decode time; Adapter `config JSONB` is the only unbounded field — limit via request size cap in chi (`http.MaxBytesReader`) — out of Phase 3 scope but flagged. |
| Cache invalidation race after commit (microsecond window) | Information Disclosure (low) | D-55 accepts the race; TTL bounds staleness to ≤60s; D-56 explicitly DEL on 409 path |
| Redis as a side channel | Information Disclosure | Cache keys are deterministic per (org, entity, id) — a malicious co-tenant cannot derive other-org data by observing latency unless they already have IDs (which they don't, due to UUIDv7 non-enumerable IDs). |

**Specific to Phase 3:**
- The Adapter `config jsonb` field is the only path where a user can stuff arbitrary structured data into the DB. Limit document size with `http.MaxBytesReader` (e.g., 64KB) at the chi root or via a per-route middleware. Not in CAT-06 acceptance but recommended for production hygiene.
- Cross-row validation (D-76) opens a TOCTOU race: queue X exists at validation time → soft-deleted by another request → channel write succeeds with reference to disabled queue. Accept in v0.1; document in plan.

## Sources

### Primary (HIGH confidence)
- `services/api/go.mod` (v0.0 working tree) — locked stack versions [VERIFIED]
- `services/api/internal/api/types.gen.go` (oapi-codegen output) — DTO shapes for Agent/Skill/Queue/Channel/Adapter/BreakReason [VERIFIED]
- `services/api/internal/api/server.gen.go` — StrictServerInterface (41 methods), response wrapper types [VERIFIED]
- `services/api/internal/server/server.go` — injectRequestIDIntoErrorResponse type switch [VERIFIED]
- `services/api/internal/server/stubs.go` — compositeServer + 501 stubs to delete [VERIFIED]
- `services/api/internal/db/orgdb.go` + `sqlcheck.go` — orgDB DBTX wrapper + SQL parser [VERIFIED]
- `services/api/internal/scaffold/handler.go` — reference handler pattern (toPgUUID, toAPIScaffold helpers, 404 mapping) [VERIFIED]
- `services/api/internal/db/queries/scaffold.sql` — sqlc query syntax baseline [VERIFIED]
- `services/api/sqlc.yaml` — sqlc 1.31.x config (`sql_package: pgx/v5`, `emit_pointers_for_null_types: true`) [VERIFIED]
- `openapi/openapi.yaml` (full) — catalog schemas, response types, error envelope, current `limit` (20/100) param [VERIFIED]
- `migrations/000001_create_scaffold.up.sql` — first migration baseline [VERIFIED]
- `services/api/test/isolation/main_test.go` + `isolation_test.go` — testcontainer + httptest pattern [VERIFIED]
- `services/api/internal/server/request_id_exhaustiveness_test.go` — exhaustiveness test that fires on missing switch cases [VERIFIED]
- [singleflight godoc](https://pkg.go.dev/golang.org/x/sync/singleflight) — `Do(key, fn) (any, error, shared)` semantics, goroutine-safe, zero-value usable [VERIFIED via WebFetch]
- [pkg.go.dev/github.com/alicebob/miniredis/v2](https://pkg.go.dev/github.com/alicebob/miniredis/v2) — v2.38.0 latest, MIT license, supports go-redis v9 [CITED]
- [github.com/alicebob/miniredis/releases](https://github.com/alicebob/miniredis/releases) — v2.38.0 May 12 2026 [CITED]

### Secondary (MEDIUM confidence)
- [sqlc datatypes docs](https://docs.sqlc.dev/en/stable/reference/datatypes.html) — JSONB default emits `[]byte` for pgx/v5 [CITED]
- [bun.uptrace.dev/guide/cursor-pagination.html](https://bun.uptrace.dev/guide/cursor-pagination.html) — composite (created_at, id) keyset pattern with tuple comparison [CITED]
- [oneuptime.com Go pagination guide](https://oneuptime.com/blog/post/2026-01-25-scale-pagination-cursor-navigation-go/view) — UUIDv7 + composite cursor [CITED]
- [DEV.to "Building a High-Performance Cache Layer in Go (2026 Guide)"](https://dev.to/young_gao/building-a-high-performance-cache-layer-in-go-2ejd) — singleflight + Redis pattern [CITED]
- [Redocly CLI npm](https://www.npmjs.com/package/@redocly/cli) — v2.30.5, supports OpenAPI 3.0 lint [CITED]
- [go-redis v9 docs](https://github.com/redis/go-redis) — concurrency-safe; supports Go 1.24+ (so 1.25 OK) [CITED]

### Tertiary (LOW confidence)
- Medium blog "Singleflight in Go: How One Line of Code Saved Our Database" — pattern reinforcement only, not authoritative [CITED but not relied upon]
- 1xapi.com "Prevent Cache Stampede" — pattern reinforcement [CITED but not relied upon]

## Metadata

**Confidence breakdown:**
- Standard stack: **HIGH** — every dep is already in go.mod or has been verified against an authoritative registry. The only new direct deps are `golang.org/x/sync` (already indirect) and `miniredis` (test-only, well-established).
- Architecture: **HIGH** — entire architecture is derived from in-tree code patterns (scaffold handler, orgDB, strict-server). No invention; the new `cache` and `catalog` packages slot into established seams.
- Pitfalls: **HIGH** — every pitfall except #3 (sqlc.narg) is verified against in-tree code, exhaustiveness test, or generated code shapes. Pitfall #3 is documented behavior in sqlc docs but warrants a test in Wave 0 to confirm.
- Validation Architecture: **HIGH** — tests follow the existing Phase 1/2 testcontainer + httptest pattern; only new addition is miniredis injection for cache unit tests.
- Security: **MEDIUM** — ASVS mapping is qualitative; specific threat patterns are derived from the stack but not exercised against a threat model. Recommend a Phase 3 plan-check step that runs the FOUND-08 isolation suite against every new entity.

**Research date:** 2026-05-16
**Valid until:** 2026-06-15 (30 days; revisit if oapi-codegen, sqlc, or go-redis publish a major version)
