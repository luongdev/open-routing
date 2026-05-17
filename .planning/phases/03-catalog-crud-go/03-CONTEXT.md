# Phase 3: Catalog CRUD (Go) - Context

**Gathered:** 2026-05-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the 6 catalog entity CRUDs (agents, skills, queues, channels, adapters, break_reasons) + the agent_skills join (CAT-03) against the strict-server stubs generated in Phase 2. Add the Redis cache layer for hot-path single-entity GETs (CAT-11). Add a single editable v0.1 migration carrying every catalog table and index. Delete the Phase 1 scaffold + Phase 2 501-stubs in the first commit of this phase.

**In scope:** Real handler bodies for all 41 `StrictServerInterface` methods in `services/api/internal/api/server.gen.go` (everything except agent state-machine endpoints owned by Phase 4 + bulk import owned by Phase 5 — these stay 501-stubs until their phase lands); sqlc query files at `services/api/internal/db/queries/{agents,skills,queues,channels,adapters,break_reasons,agent_skills}.sql`; one editable migration `migrations/000002_catalog_v0_1.{up,down}.sql` (project root, matches Phase 1 layout — see A6) covering every catalog table + indexes + constraints; new `services/api/internal/cache/` package shipping `cache.Cache` + `cache.GetOrSet[T any]` + `cache.Del`; new `services/api/internal/catalog/` package shipping `catalog.Handlers` with one .go file per entity; small spec amendment to add `invalid_reference` ErrorCode + 422 responses to CREATE/UPDATE endpoints (regenerated via codegen-drift); per-entity `_test.go` beside source + cross-org probe extensions in `services/api/test/isolation/`.

**Out of scope:** Phase 4 — agent state machine (`PATCH /agents/{id}/status`, `IsRoutable`, state-machine validation rules, `agent_states` table); Phase 5 — bulk import (`POST /v1/orgs/{org_id}/catalog/import`, CSV parsing, `import_jobs` table); Phase 6 — admin SPA UI consuming these endpoints; Phase 7 — Web Component embed; rate limiting; cache for list endpoints (CAT-11 specs single-entity only); cache for negative results (404s) — deferred to v0.2 with metrics-driven decision; bloom filter; OTel cache metrics export (slog attrs only in v0.1); auth beyond Phase 1 stub `X-Org-Id` header.

</domain>

<decisions>
## Implementation Decisions

### Cache Layer (CAT-11)

- **D-49:** **Handler-level explicit cache calls.** Each `GetById` handler calls `cache.GetOrSet(ctx, key, ttl, loadFromDB)`; each write handler calls `cache.Del(ctx, key)` AFTER the DB transaction commits. The cache contains DTO types (e.g., `api.Agent`), NOT sqlc row types — the cache sits AFTER the sqlc→DTO mapping step in the handler. List endpoints (`CAT-09`, `CAT-10`) are NOT cached.
- **D-50:** **Cache lives in standalone `services/api/internal/cache/` package** — reusable across phases (Phase 4 will use it for status lookups; Phase 5 may use it for import session reads). Shape: `cache.Cache struct { rdb *redis.Client; sf *singleflight.Group; logger *slog.Logger }` constructed in `main.go` and passed via the catalog `Deps` struct.
- **D-51:** **Generic `cache.GetOrSet[T any]` helper** using Go type parameters. Signature: `func GetOrSet[T any](ctx context.Context, c *Cache, key string, ttl time.Duration, load func(context.Context) (T, error)) (T, error)`. JSON marshal/unmarshal happens inside the helper; callers see typed values. For not-found semantics the loader returns `(T{}, ErrNotFound)` and the helper propagates without caching the empty value.
- **D-52:** **`x/sync/singleflight` from day 1.** Cache misses go through a `singleflight.Group` so concurrent requests for the same (orgId, entity, id) key issue only one DB load per process. Cross-instance stampede protection (Redis SETNX lock) NOT in v0.1 — accept the bounded fan-out (one DB load per Go process per key per miss).
- **D-53:** **Refresh-ahead at 10s remaining (~16% of TTL).** Inside `GetOrSet`, after a hit, check `PTTL(key)`; if remaining < 10s, kick off an async singleflight refresh that re-loads from DB and writes a fresh cache entry. The current reader gets the cached value immediately. Async refresh uses a goroutine spawned from `GetOrSet` with a derived context (`context.WithoutCancel(ctx)` so request cancellation doesn't kill the refresh). Refresh failures are logged but never propagated to the caller.
- **D-54:** **No negative caching in v0.1.** 404 responses are not cached. FOUND-08's "cross-org returns 404, never 200" policy already prevents the cache from being a cross-org probe oracle. If v0.2 metrics show DB pressure from ID-enumeration, revisit with a 5-10s sentinel (NOT bloom filter — see deferred).
- **D-55:** **Invalidation order: DEL after DB commit succeeds.** Pattern in every write handler:
  ```
  tx, err := orgDB.Begin(ctx)
  // ... UPDATE ... COMMIT ...
  if err := cache.Del(ctx, key); err != nil {
      logger.WarnContext(ctx, "cache del failed", "key", key, "err", err)
      // do NOT return 500 — DB write succeeded, cache will TTL out in 60s
  }
  return 200
  ```
  Cache failure NEVER turns a successful write into a 5xx. The microsecond race between commit and DEL is acceptable.
- **D-56:** **409 (version mismatch) also DELs the cache key.** A version conflict suggests our cache may be stale relative to the true current row. The 409 handler re-fetches the current row from DB, calls `cache.Del(ctx, key)`, then returns 409 with the fresh row as `current` per D-37 `VersionConflictErrorResponse`.
- **D-57:** **Observability via slog attrs only in v0.1.** Inside `GetOrSet`: `slog.DebugContext(ctx, "cache", "key", k, "outcome", "hit|miss|refresh|error")`. No OTel counter metrics yet — Phase 1 wired traces + slog handler but NOT the metrics export pipeline. Defer dashboard metrics to v0.2.
- **D-58:** **Cache key format `or:{orgId}:{entity}:{id}`** (verbatim from CAT-11). The entity slug is the URL plural form (`agents`, `skills`, `queues`, `channels`, `adapters`, `break_reasons`). Helper constructor: `cache.Key(orgID, "agents", agentID)`.
- **D-59:** **TTL is fixed 60s** (no jitter). Refresh-ahead supersedes jitter as the stampede defense; CAT-11 spec wording is literal "60-second TTL" — don't deviate.
- **D-60:** **JSON encoding inside the cache uses `encoding/json` (stdlib).** No alternative serializer (no `json-iterator`, no msgpack). Forward-compat: any DTO change that breaks JSON round-trip is caught by codegen-drift CI before merge.

### Schema + sqlc Layout

- **D-61:** **Single editable migration for the entire v0.1 milestone.** File: `migrations/000002_catalog_v0_1.up.sql` (project root, matches Phase 1 layout from `migrations/000001_create_scaffold.up.sql` — see A6) + matching `.down.sql`. Phase 3 introduces all 6 catalog tables + `agent_skills` + indexes + constraints. Phases 4 and 5 EDIT this migration in-place during v0.1 development (Phase 4 adds `agent_states` columns + state transition CHECK; Phase 5 adds `import_jobs`). Once v0.1 ships to a real environment, freeze 002 and ratchet forward with 003+. Dev workflow: `task db:reset` drops the database, then `migrate up` re-runs all migrations from scratch. Testcontainers run `migrate up` on each container boot — this works unchanged.
- **D-62:** **Per-entity sqlc query files** at `services/api/internal/db/queries/`: `agents.sql`, `skills.sql`, `queues.sql`, `channels.sql`, `adapters.sql`, `break_reasons.sql`, `agent_skills.sql`. Each ~80 LOC containing the CRUD set for one entity. sqlc emits one `*.sql.go` per `*.sql` so generated code mirrors the layout.
- **D-63:** **Cursor pagination format: `base64(JSON {created_at: RFC3339Nano, id: UUIDv7})`.** Client opaque, server-decodable. SQL: `WHERE (created_at, id) < ($cursor_ts, $cursor_id) ORDER BY created_at DESC, id DESC LIMIT $page_size + 1`. The `+1` row tells us whether to emit a `next_cursor` in the response. The composite key (`created_at, id`) is stable under inserts (UUIDv7 carries time, so cursor ordering aligns with insertion order).
- **D-64:** **Name search uses `ILIKE '%' || $name || '%'`** with a functional index `CREATE INDEX ix_{entity}_org_name ON {entity}(org_id, lower(name) text_pattern_ops) WHERE enabled = true`. Substring search with leading `%` won't use the index, but at v0.1 scale (≤1K rows per entity per org) the table scan is fine. Trigram (`pg_trgm`) deferred to v0.2 — see deferred ideas.
- **D-65:** **Default-list `WHERE enabled = true` filter** applied unconditionally in the sqlc List query. `?include_disabled=true` (CAT-09) swaps to a separate query that omits the filter. Two queries per entity (`ListAgents`, `ListAgentsIncludingDisabled`) keeps each plan-able and statement-cacheable.
- **D-66:** **Atomic UPDATE with version check + RETURNING.** sqlc query pattern for every entity UPDATE:
  ```sql
  -- name: UpdateAgent :one
  UPDATE agents
  SET name = $2, email = $3, enabled = $4, version = version + 1, updated_at = NOW()
  WHERE id = $1 AND org_id = $5 AND version = $6
  RETURNING *;
  ```
  Single round-trip happy path. If 0 rows returned, the handler issues a follow-up `SELECT ... WHERE id = $1 AND org_id = $2` to disambiguate 404 (no row) from 409 (row exists but version mismatch). The `version_conflict` 409 response embeds the fresh row as `current` per D-37.
- **D-67:** **Page size cap: 25 default, 100 max.** `?page_size=N` query param accepted in [1, 100]; out-of-range returns 400 invalid_body. Default 25 keeps response payloads small for catalog list UIs.

### Handler Package Layout

- **D-68:** **Single `services/api/internal/catalog/` package** with one `.go` file per entity: `agents.go`, `skills.go`, `queues.go`, `channels.go`, `adapters.go`, `break_reasons.go`, `agent_skills.go`. Plus shared helpers in the same package: `handlers.go` (Deps struct + New constructor), `mappers.go` (sqlc.X → api.X conversions), `cursor.go` (encode/decode), `errors.go` (sqlc/pgx error → strict-server response mapping).
- **D-69:** **`catalog.Handlers` IS the StrictServerInterface implementer.** No composite server. The Phase 2 `services/api/internal/server/stubs.go` + `services/api/internal/scaffold/handler.go` get DELETED in Phase 3's first commit. `server.NewMux` constructs `catalog.New(deps)` and passes it directly to `api.NewStrictHandler`. Until Phases 4 and 5 land, the catalog handler returns 501 for state-machine + bulk-import methods via a `notImplemented` helper (keeps the codebase compiling without revived stubs).
- **D-70:** **Forward-compat for Phases 4/5 via struct embedding.** When Phase 4 lands, it introduces `services/api/internal/state/` with `state.Handlers` implementing ONLY the state-machine methods. Phase 5 introduces `services/api/internal/imports/` with `imports.Handlers`. A top-level `ApiHandlers` struct embeds all three:
  ```go
  type ApiHandlers struct {
      *catalog.Handlers
      *state.Handlers   // Phase 4
      *imports.Handlers // Phase 5
  }
  ```
  Go's method-set resolution merges them; conflicts surface at compile time (none expected — each phase owns disjoint endpoints). Phase 3 ships `catalog.Handlers` alone; the embedding struct comes online in Phase 4.
- **D-71:** **Hybrid constructor `catalog.New(deps Deps, opts ...Option)`.** Required deps in a typed struct (compile-error if a field is missing); optional knobs (Clock, future cache TTL override) via functional options.
  ```go
  type Deps struct {
      OrgDBFactory db.Factory      // required
      Cache        *cache.Cache    // required
      Logger       *slog.Logger    // required
  }
  type Option func(*Handlers)
  func WithClock(c func() time.Time) Option { ... }
  func New(deps Deps, opts ...Option) *Handlers { ... }
  ```
  In v0.1 only `WithClock` is exposed (for deterministic tests of UUIDv7 minting + created_at timestamps).
- **D-72:** **Per-entity `_test.go` beside source.** `catalog/agents_test.go` covers happy-path + edge cases for agents (CRUD, version mismatch, soft-delete filtering, cursor pagination, name search, cache hit/miss); same for each other entity. Tests use `httptest.NewRecorder` + miniredis + Postgres testcontainers (extends Phase 1 FOUND-08 setup). Cross-org probe coverage lives in `services/api/test/isolation/catalog_test.go` (extends the existing isolation suite — every new entity gets a cross-org probe test enforcing 404, never 200).
- **D-73:** **Shared test setup** in `services/api/internal/catalog/testutil_test.go` (package-local `_test.go` so miniredis/testify do NOT leak into the production build — see A6). Unexported helpers compile only for `go test` runs. Avoids re-writing miniredis + pgxpool boilerplate in every _test.go.

### Concurrency + Validation

- **D-74:** **Two-layer validation.** Layer 1 (automatic, free): oapi-codegen runtime schema validation rejects malformed bodies, missing required fields, type mismatches, out-of-range numbers (proficiency 1-10 from CAT-03), bad string formats. Returns 400 with `invalid_body` ErrorCode. Layer 2 (manual, handler-level): cross-row rules requiring DB queries — e.g., `agent_skills.skill_id` must exist in same org; `channels.default_queue_id` must exist + be enabled in same org; `agent.state.break_reason_id` must exist in same org (Phase 4). Returns 422 with new `invalid_reference` ErrorCode.
- **D-75:** **Spec amendment in Phase 3** to add `invalid_reference` ErrorCode + 422 responses. Phase 2's D-32 ("Phases 3-5 do NOT add new paths to openapi.yaml") covers PATHS — refining error response codes on existing paths is allowed. The spec edit:
  - Adds `invalid_reference` to `components.schemas.ErrorCode` enum (now 11 values).
  - Adds 422 response on every CREATE/UPDATE that takes an FK reference (the relevant CRUD endpoints for agent_skills, channels, queues, and Phase 4's PATCH /agents/{id}/status).
  - Codegen-drift CI catches missing regeneration after the edit.
- **D-76:** **Cross-row validation runs BEFORE the UPDATE/INSERT statement** in the handler. Pattern:
  ```
  if req.DefaultQueueId != nil {
      exists, err := q.QueueExistsInOrg(ctx, *req.DefaultQueueId, orgID)
      if err != nil { return internal500 }
      if !exists { return 422 invalid_reference }
  }
  // ... UPDATE ...
  ```
  Race window (the row could be soft-deleted between check and write) is acceptable — the next read returns the soft-deleted entry as part of a now-broken relationship; v0.2 can add FK CASCADE handling. Phase 3 does NOT add database-level FKs across catalog tables for v0.1 (avoids the soft-delete-vs-FK semantic mismatch) — references are validated at the application layer only.

### Phase 1/2 Carry-Forward

- **D-77:** **Scaffold + 501-stubs deleted in Phase 3 commit 1.** Plan order: (1) delete `services/api/internal/scaffold/`, `services/api/internal/server/stubs.go`, scaffold sqlc queries, scaffold paths from `openapi.yaml`; (2) regenerate `services/api/internal/api/*.gen.go` (drift CI passes); (3) introduce migration `migrations/000002_catalog_v0_1.up.sql` (project root per A6) + sqlc files; (4) introduce `internal/cache/` package; (5) introduce `internal/catalog/` package with first entity (agents) end-to-end; (6) replicate for skills/queues/channels/adapters/break_reasons; (7) add agent_skills join. Planner sequences these as waves.

### Claude's Discretion

- Exact slog attribute keys for cache observability (`outcome`, `cache_key`, etc.) — planner picks consistent names.
- The Go file ordering inside `internal/catalog/` (alphabetical vs domain-grouped) — alphabetical is the default.
- Default page size between 25 and 50 — locked at 25 here; planner can revisit if UI feedback wants larger.
- Whether `pgError` 23505 (unique violation) on CREATE returns 409 or 422 — planner picks; default 409 with `version_conflict`-flavored body for external_id collisions.
- Whether `ListAgents` and `ListAgentsIncludingDisabled` are two sqlc queries or one query with a `$show_disabled bool` parameter — both work; planner picks based on query-plan inspection.
- Mapper file naming (`mappers.go` vs `convert.go`) — planner picks; consistency across phases matters.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level locks
- `.planning/PROJECT.md` — Locked stack: Go + chi + sqlc + pgx + golang-migrate + slog; PostgreSQL 17 + Redis. Catalog entity field shapes (CAT-01 through CAT-07).
- `.planning/REQUIREMENTS.md` §Catalog CRUD — **CAT-01 through CAT-11**. The acceptance criteria for this phase. CAT-08 (version mismatch + `current`), CAT-09 (soft-delete via `enabled=false`), CAT-10 (cursor + filter + name search), CAT-11 (Redis cache key/TTL/invalidation).
- `.planning/ROADMAP.md` §Phase 3 — Goal statement and 5 success criteria (SOFT-DELETE behavior, version conflict, list filter combinations, proficiency validation, Redis cache).
- `.planning/STATE.md` §Accumulated Context — Phase 1 + Phase 2 closeout status; LOCKED carry-forwards.

### Phase 1 carry-forward (LOCKED — do not rewrite)
- `.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md` — All D-01 through D-31 are LOCKED. Especially:
  - **D-17, D-21:** Bypass list `{/healthz, /readyz, /metrics, /openapi.yaml, /docs}` (Phase 2 extended). Org middleware applies to everything under `/v1/`.
  - **D-19, D-20:** UUIDv7 server-minted at write boundary. `org_id` parsed as `uuid.UUID` from `X-Org-Id` header.
  - **D-22:** golang-migrate is the locked migration tool. Phase 3 uses ONE editable migration `000002_catalog_v0_1` per D-61.
  - **D-23:** Internal package layout — `internal/api/` is generated, `internal/db/queries/` is sqlc source, `internal/db/generated/` is sqlc output, **migrations live at project root `./migrations/`** (matches Phase 1 layout — `migrations/000001_create_scaffold.up.sql`; see A6 for the alignment with Phase 1 codebase reality).
  - **D-25:** `internal/db/orgdb.go` per-request `*db.OrgDB` pattern — every catalog handler builds an `orgDB` from ctx and constructs `generated.New(orgDB)`. FOUND-08 enforces this via cross-org probe tests.
  - **D-28, D-29:** RequestID middleware mints UUIDv7 and writes `X-Request-Id` header; slog handler injects `trace_id`/`span_id`. Phase 2's D-35 + `WriteError` embed `request_id` in error body.
  - **D-30:** GitHub Actions CI shape — Phase 3 plugs into `go-test` job (no new CI job needed; cache tests run under the existing test matrix).
- `.planning/phases/02-openapi-contract-codegen/02-CONTEXT.md` — All D-32 through D-48 are LOCKED. Especially:
  - **D-35, D-36, D-37:** Error envelope, 10 ErrorCode values, special-case response shapes. Phase 3 extends with `invalid_reference` (11th value) per D-75.
  - **D-41, D-42:** Strict-server mode with three .gen.go files (server, types, spec). Phase 3 does NOT edit these — they are codegen output.
  - **D-44, D-46:** `server.NewMux` accepts strict-server handlers; Phase 2 scaffold migration proved the pattern. Phase 3 D-77 deletes scaffold.
  - **D-47, D-48:** codegen-drift CI job. Phase 3 spec edit (D-75) triggers regeneration of `services/api/internal/api/*.gen.go` + `web/packages/ui/src/api/generated.ts`.

### Phase 2 hardening (from cross-AI review)
- `services/api/internal/middleware/uuidv7path.go` — UUIDv7PathParams middleware. Already applied to all `/v1/.../{id}` routes; rejects non-v7 UUIDs with 400 `invalid_id`. Phase 3 handlers do NOT need to re-validate UUID format — the middleware handles it before the handler runs.
- `services/api/internal/server/request_id_exhaustiveness_test.go` — 121 sub-tests asserting every `*JSONResponse` wrapper is covered by `injectRequestIDIntoErrorResponse`. Phase 3 adds new error response types (`VersionConflictErrorResponse`, `InvalidReferenceErrorResponse`) which MUST be added to the type switch in `services/api/internal/server/server.go`; the exhaustiveness test will fail the build otherwise.

### Existing code Phase 3 touches/extends
- `services/api/internal/api/server.gen.go` — `StrictServerInterface` with 41 methods (do NOT edit; regenerated by codegen).
- `services/api/internal/api/types.gen.go` — 88 DTO types (do NOT edit; codegen output).
- `services/api/internal/api/spec.gen.go` — embedded openapi.yaml bytes (do NOT edit; regenerated when openapi.yaml changes).
- `openapi/openapi.yaml` — Phase 3 amends to add `invalid_reference` enum value + 422 responses on CREATE/UPDATE endpoints (D-75).
- `services/api/internal/db/queries/scaffold.sql` — DELETED in Phase 3 commit 1.
- `services/api/internal/db/generated/scaffold.sql.go` — DELETED via codegen regeneration after scaffold queries removed.
- `services/api/internal/scaffold/handler.go` — DELETED in Phase 3 commit 1.
- `services/api/internal/server/stubs.go` — DELETED in Phase 3 commit 1.
- `services/api/internal/server/server.go` — Phase 3 simplifies `NewMux` to wire `catalog.New(deps)` directly into `api.NewStrictHandler`; removes scaffold mount and composite assembly. The type switch in `injectRequestIDIntoErrorResponse` gains new cases for `VersionConflictErrorResponse` + `InvalidReferenceErrorResponse`.
- `services/api/internal/middleware/httputil.go` — `WriteError` unchanged; Phase 3 catalog handlers use it for error responses outside the strict-server typed response objects (e.g., before strict-server runs).
- `services/api/internal/db/orgdb.go` — Pattern carried forward; every catalog handler entry-point pulls `orgDB` from ctx and constructs `generated.New(orgDB)`.
- `services/api/cmd/api/main.go` — Phase 3 adds `cache.New(rdb, slogHandler)` construction + passes `cache.Cache` into `catalog.New(Deps{...})`. Also passes new `catalog.Handlers` to `server.NewMux` in place of the Phase 2 composite.
- `services/api/test/isolation/isolation_test.go` — Phase 3 extends with cross-org probe tests for each new entity (6 new test cases + 1 for agent_skills).
- `services/api/test/isolation/main_test.go` — Phase 3 may add a Redis testcontainer alongside the Postgres one (if isolation tests need cache assertions) — though most cache testing can live in catalog/_test.go with miniredis.
- `services/api/go.mod` — Phase 3 adds `golang.org/x/sync` (for `singleflight`) + `github.com/alicebob/miniredis/v2` (test-only).
- `migrations/000001_create_scaffold.up.sql` — Phase 1 scaffold migration at project root (verified existing layout). Phase 3 does NOT edit; Phase 3 adds `migrations/000002_catalog_v0_1.up.sql` next to it (per A6).
- `Taskfile.yml` — Phase 3 may add `task db:reset` (drop + create + migrate up) for the dev workflow per D-61. `task gen` already covers sqlc + go-generate + openapi-typescript regeneration.
- `web/packages/ui/src/api/generated.ts` — REGENERATED in Phase 3 when openapi.yaml gains `invalid_reference` (D-75). TypeScript consumers get autocomplete on the new error code via `ErrorCodes.INVALID_REFERENCE`. No Phase 3 hand-edits to web/.

### External standards
- [PostgreSQL 17 docs §11.10 Indexes on Expressions](https://www.postgresql.org/docs/17/indexes-expressional.html) — functional index pattern for `lower(name)` (D-64).
- [PostgreSQL 17 docs §70 Index Access Method Interface](https://www.postgresql.org/docs/17/index-access-methods.html) — partial index with `WHERE enabled = true` (D-64).
- [golang.org/x/sync/singleflight](https://pkg.go.dev/golang.org/x/sync/singleflight) — singleflight.Group API (D-52).
- [go-redis/v9 docs](https://redis.uptrace.dev/) — `PTTL`, `Del`, JSON-encoded payloads (D-50, D-53, D-55).
- [oapi-codegen v2 runtime validation](https://github.com/oapi-codegen/oapi-codegen) — schema enforcement at decode time (D-74 Layer 1).
- [miniredis](https://github.com/alicebob/miniredis) — in-process Redis for tests (D-73).
- [RFC 9562 — UUIDv7](https://www.rfc-editor.org/rfc/rfc9562.html) §5.7 — cursor stability rationale (D-63).
- [keyset pagination guide](https://use-the-index-luke.com/no-offset) — rationale for D-63 cursor format.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`middleware.WriteError(ctx, w, status, code, reason)`** at `services/api/internal/middleware/httputil.go` — Locked by Phase 2 to embed `request_id` from ctx. Phase 3 catalog handlers use it for pre-strict-server errors (rare; most errors flow through typed `*JSONResponse` objects).
- **`middleware.UUIDv7PathParams` middleware** at `services/api/internal/middleware/uuidv7path.go` — Already applied to all `/v1/.../{id}` routes. Catalog handlers can assume any `{id}` path param is a valid UUIDv7; no re-validation needed.
- **`middleware.RequestIDInjectionMiddleware`** (StrictMiddlewareFunc) at `services/api/internal/server/server.go` — Already wraps every strict-server handler. Phase 3 just ensures any new error response type is added to the `injectRequestIDIntoErrorResponse` type switch.
- **`db.OrgDB` per-request pattern** at `services/api/internal/db/orgdb.go` — Catalog handler entry pulls `orgDB` from ctx (`db.OrgDBFromContext(ctx)`) and constructs `generated.New(orgDB)`. FOUND-08 cross-org probe enforces this.
- **`generated.New(orgDB)` sqlc Queries constructor** — Unchanged. Phase 3 adds queries for each catalog entity; sqlc regenerates the `*Queries` struct with new methods.
- **`redis.Client` constructed in `main.go`** at `services/api/cmd/api/main.go:107` — Already wired with `redis.ParseURL(cfg.RedisURL)`. Phase 3 wraps it in `cache.New(rdb, logger)` and passes to `catalog.New(Deps{Cache: ...})`.
- **`testsupport.Setup(t)` testcontainers helper** — Phase 1 wiring; FOUND-08 isolation tests use it. Phase 3 catalog `_test.go` files extend the pattern (add miniredis alongside if Redis assertions are needed).
- **Phase 2 `api.NewStrictHandler(serverImpl, []StrictMiddlewareFunc{...})`** in `server.go` — Wired with `[]StrictMiddlewareFunc{middleware.RequestIDInjectionMiddleware}`. Phase 3 just swaps `serverImpl` from the composite to `catalog.New(deps)`.

### Established Patterns
- **Bypass routes at root, `/v1/*` gated by OrgContext** (D-21) — Phase 3 doesn't touch root routes.
- **Response body shape `{error, reason, request_id}` + extension fields for CAT-08/STATE-03** (D-35, D-37) — Phase 3 implements `VersionConflictErrorResponse` (with `current`) per the strict-server typed response objects already generated in Phase 2.
- **UUIDv7 minted at write boundary** (D-19, scaffold `uuid.Must(uuid.NewV7())` at create) — Phase 3 catalog handlers continue this pattern. `cache.Cache` does NOT mint IDs.
- **Per-request `*db.OrgDB`** — Phase 3 handlers retrieve via `db.OrgDBFromContext(ctx)`; cache layer is NOT org-scoped at the type level (orgID is in the key) — singleton `*cache.Cache` shared across requests.
- **sqlc `:one` for write-with-RETURNING; `:many` for List; `:exec` for delete (rare in v0.1 — most "delete" is soft-delete UPDATE)** — Phase 3 catalog queries follow this. Version-checked UPDATEs use `:one` (returns the updated row); 0-row response triggers Go-side fallback SELECT.
- **httputil + strict-server coexistence** (Phase 2 D-44) — Phase 3 prefers typed `*JSONResponse` returns; falls back to `WriteError` only outside the strict-server flow (e.g., if a middleware needs to short-circuit).

### Integration Points
- **Phase 2 codegen-drift CI** — Phase 3 spec amendments (`invalid_reference` + 422 responses) re-trigger codegen on PR. Drift gate enforces regeneration of all `*.gen.go` + `generated.ts`.
- **Phase 4 (Agent State Machine)** — Will introduce `services/api/internal/state/` with `state.Handlers` for `PATCH /agents/{id}/status` etc. Will EMBED catalog handlers via the `ApiHandlers` composition struct (D-70). Will use `cache.GetOrSet[T any]` for status lookups with key `or:{orgId}:agent_state:{agent_id}`.
- **Phase 5 (Bulk Import)** — Will introduce `services/api/internal/imports/` with `imports.Handlers` for `POST /catalog/import` + `GET /imports/{id}`. Will append `import_jobs` table to the same `migrations/000002_catalog_v0_1.up.sql` migration (per D-61 + A6).
- **Phase 6 (Shared UI Library & Standalone Admin)** — Will consume Phase 3's expanded ErrorCodes record (now includes `INVALID_REFERENCE`). No Phase 3 frontend code; Phase 3 only triggers regeneration of `web/packages/ui/src/api/generated.ts` via the spec amendment.
- **Phase 7 (Web Component Embed)** — Same as Phase 6 — consumes the regenerated types only.

</code_context>

<specifics>
## Specific Ideas

- **Single editable migration during v0.1** — User explicitly wanted "đơn giản hóa cho v0.1, chưa triển khai mà nhiều file quá thì tao rất ghét". Don't generate per-phase migration files; edit one file. Freeze post-deploy.
- **Singleflight from day 1, not "add later if needed"** — User pushed for defensive concurrency primitive even at v0.1 scale. Acceptable cost (small, stdlib-adjacent dep).
- **Refresh-ahead at 10s remaining (not TTL jitter)** — User preferred the active-refresh approach over passive jitter. Pairs well with singleflight (refresh runs in a singleflight, so concurrent refreshers dedupe).
- **Hybrid constructor `New(Deps, ...Option)`** — User chose to combine both patterns. Required deps stay typed (compile-error if missing); optional knobs use Go's idiomatic functional options. Only `WithClock` is exposed in v0.1.
- **Bloom filter pushback was honored** — User initially asked about bloom for 404 protection; agreed with the "over-engineering for v0.1" pushback after seeing the Redis-module dependency + bloom-maintenance overhead.
- **Per-entity test files** — User explicitly preferred test files beside source over a separate `test/integration/catalog/` package.
- **Standalone `internal/cache/` package** — Cache is reusable infra; lives in its own package so Phase 4/5 can import it cleanly.
- **Spec amendment IS allowed in Phase 3** — Phase 2 D-32 only locked PATHS. Refining error response codes / adding ErrorCode enum values is a non-breaking spec change that flows through codegen-drift cleanly.

</specifics>

<deferred>
## Deferred Ideas

- **Bloom filter for 404 protection** — Revisit in v0.2 if metrics (DB-side slow log + cache hit/miss ratio in slog) show pressure from ID enumeration. Requires RedisBloom module install + bloom-maintenance subsystem (every CREATE adds, every hard-delete rebuilds).
- **OTel cache_ops_total counter export** — Wait until v0.2 sets up the metrics export pipeline. v0.1 has slog attrs only.
- **5-10s negative caching sentinel** — Fallback option if D-54 (no negative caching) shows pressure. Simpler than bloom; revisit before bloom.
- **TTL jitter (60s ± 5s)** — Refresh-ahead supersedes this in v0.1. If refresh-ahead behaves poorly under load, fall back to jitter.
- **pg_trgm extension + gin index for name search** — Defer until v0.2 OR until v0.1 scale crosses 10K rows per entity per org. ILIKE on lower(name) functional index is the v0.1 plan.
- **Database-level FK constraints across catalog tables** — Deferred per D-76. Application-layer cross-row validation only in v0.1 (avoids soft-delete-vs-FK semantic mismatch). Revisit when hard-delete or audit-trail tables land.
- **Cache for list endpoints** — CAT-11 specs single-entity only. List endpoints hit DB every time. If v0.2 metrics show list-endpoint pressure, add a separate `cache.ListCache` with explicit invalidation on any entity write in that org.
- **`cache.Health()` integration with `/readyz`** — Phase 1's `/readyz` already pings Redis directly via `rdb.Ping`. Phase 3's `cache.Cache` could expose a richer `Health()` method (e.g., do a SET+GET roundtrip), but Phase 3 sticks with the existing ping.
- **Cursor signing / HMAC** — Cursors are currently base64(JSON) plaintext; client can craft arbitrary cursors. Server validates ranges so this is mostly harmless, but a malicious cursor could probe row counts. Defer signed cursors to a security hardening phase.
- **Mapper code generation** — Manual `sqlc.Agent → api.Agent` mappers (D-68 mappers.go) repeat 6 times. Codegen with something like `goverter` could automate. Defer; manual mappers are easy to read and v0.1 has only 6 entities.
- **Audit-trail tables for catalog writes** — REQUIREMENTS.md mentions "org-facing audit across routing, flow publish, config changes, and runtime traces" but that's a v1 concern. Phase 3 only updates `updated_at` per entity row.

</deferred>

<amendments>
## Amendments (post-context, post-planning iterations)

- **A1..A5:** (placeholders if any prior iteration captured amendments inline in earlier revision passes — none recorded in this CONTEXT.md.)
- **A6 (revision iter 3, 2026-05-16):** **Reconciled migration path with Phase 1 codebase reality.** Migrations live at project-root `./migrations/`, not `services/api/internal/db/migrations/`. The aspirational path in the original D-22/D-23/D-61/D-77 wording was a misalignment with already-existing Phase 1 code (`migrations/000001_create_scaffold.up.sql`). golang-migrate's six-digit zero-padding convention is preserved (`000001`, `000002`, ...). All four mentions above (Phase Boundary "In scope", D-23 internal package layout, D-61 migration filename, D-77 Plan order step 3, plus the "Existing code" entry for the Phase 1 migration) updated. **Plan 03-02 already uses the correct project-root path** so no plan-level change required; this amendment closes the doc-vs-code gap surfaced by Codex review.
- **A7 (revision iter 3, 2026-05-16):** **testutil moved to `_test.go` to keep miniredis/testify out of production builds.** D-73 originally referenced `services/api/internal/catalog/testutil.go` (with a `//go:build test` fallback). Renamed to `testutil_test.go` per Codex review (Concern C7) — Go's standard `_test.go` suffix is the idiomatic way to make a file test-only without a build tag. Plans 03-05 + 03-06 updated to match; agents_test.go etc. stay `package catalog` so they can call unexported helpers in testutil_test.go.

</amendments>

---

*Phase: 3-Catalog CRUD (Go)*
*Context gathered: 2026-05-16*
*Last revised: 2026-05-16 (iter 3, codex BLOCKER fixes)*
