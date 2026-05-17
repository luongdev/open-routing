# Phase 3: Catalog CRUD (Go) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-16
**Phase:** 3-Catalog CRUD (Go)
**Areas discussed:** Cache layer (CAT-11), sqlc + migrations layout, Handler package layout, Concurrency 409 + validation

---

## Cache layer (CAT-11)

### Q1: Cache placement in request path

| Option | Description | Selected |
|--------|-------------|----------|
| Handler-level explicit | Each GetById calls cache.GetOrSet; each write calls cache.Del. Caches DTO. | ✓ |
| sqlc-wrap CachedQueries | Wrap *generated.Queries; caches sqlc.Agent row type. | |
| Decorator on StrictServerInterface | CachedStrictServer wraps every method. | |
| chi middleware on /v1/.../{id} | URL-pattern middleware reads cache before handler. | |

**User's choice:** Handler-level explicit (Recommended)
**Notes:** Selected without pushback. The cache must contain DTO types (`api.Agent`), not sqlc row types — so the cache sits AFTER sqlc→DTO mapping.

### Q2: Cache helper structure

| Option | Description | Selected |
|--------|-------------|----------|
| Generic Cached[T] with type param | `func GetOrSet[T any](ctx, c, key, ttl, load) (T, error)`. JSON inside. | ✓ |
| Per-entity typed methods | `cache.GetAgent / SetAgent` for each entity. | |
| Raw []byte interface | Caller handles JSON. | |

**User's choice:** Generic Cached[T] with type param (Recommended)
**Notes:** Go generics give type-safe single implementation with JSON inside the helper.

### Q3: Stampede protection on cache miss

| Option | Description | Selected |
|--------|-------------|----------|
| Naive (no single-flight) | Each goroutine hits DB on miss. Acceptable for v0.1 scale. | |
| x/sync/singleflight from day 1 | Wrap loader with singleflight.Group; per-process dedup. | ✓ |
| Redis SETNX lock + spin | Cross-instance lock. Overkill for v0.1. | |

**User's choice:** x/sync/singleflight from day 1
**Notes:** User chose the more defensive option over YAGNI. Accept the small stdlib-adjacent dep.

### Q4: Cache invalidation ordering on write

| Option | Description | Selected |
|--------|-------------|----------|
| DEL after DB commit succeeds | Best-effort post-commit DEL; log on failure, never 5xx. | ✓ |
| DEL before DB commit | DEL → UPDATE → COMMIT; never serves stale-after-write. | |
| DEL + SET new value | Repopulate after commit. | |
| Redis MULTI + DEL on COMMIT hook | pgx-level commit hook. | |

**User's choice:** DEL after DB commit succeeds (Recommended)
**Notes:** DB is the source of truth; cache failures never cascade into 5xx.

### Q5: Negative caching (cache 404s)

| Option | Description | Selected |
|--------|-------------|----------|
| Don't cache 404s in v0.1 | Simplest; FOUND-08 already prevents probe oracle. | ✓ |
| 5-10s negative sentinel | Light 404 protection without bloom infra. | |
| Ship bloom filter anyway | RedisBloom module + bloom-maintenance subsystem. | |

**User's choice:** Don't cache 404s in v0.1 (Recommended) — after pushback on bloom filter
**Notes:** User initially asked "Dùng bloom filter chắc là ngon chứ hả?" — bloom is a legit pattern but requires RedisBloom Redis module + bloom-maintenance code (every CREATE, soft-delete, cold-start). Over-engineering for v0.1 where rate limiting doesn't exist and adversarial probing isn't a real threat. User accepted the pushback.

### Q6: Refresh-ahead threshold (PTTL check)

| Option | Description | Selected |
|--------|-------------|----------|
| 10s remaining (~16%) | Comfortable buffer for DB roundtrip + cache-fill before expiry. | ✓ |
| 20s remaining (~33%) | Earlier trigger; wasteful for cold-ish keys. | |
| 30s remaining (50%) | Halves TTL on hot keys; defeats the 60s spec. | |

**User's choice:** 10s remaining (~16% of TTL) — Recommended
**Notes:** User initially picked "60s with hot-key refresh-ahead" over fixed TTL or jitter, then selected the 10s buffer.

### Q7: Cache metrics

| Option | Description | Selected |
|--------|-------------|----------|
| slog attrs only | Debug-level slog with cache outcome attr. | ✓ |
| slog + OTel counter metrics | Prometheus counters via OTel. | |
| No instrumentation | Skip entirely. | |

**User's choice:** slog attrs only (Recommended for v0.1)
**Notes:** Phase 1 wired slog handler + OTel traces but NOT the metrics export pipeline. Adding metrics is a v0.2 concern.

---

## sqlc + migrations layout

### Q1: Migration grain

| Option | Description | Selected |
|--------|-------------|----------|
| Single 002_catalog_v0_1.up.sql for the entire v0.1 milestone | One file edited in-place by Phases 3/4/5 during dev; frozen post-deploy. | ✓ |
| One migration per entity (7 files) | 7 separate migrations. | |
| Two migrations: entities + agent_skills | Split join table. | |

**User's choice:** "Tôi muốn toàn bộ milestone 1 sẽ dùng chung 1 file migrations, cứ update liên tục vào là được, còn khi migration local thì tự chạy sql"
**Translation/Decision:** Single editable migration file for all of v0.1. Dev workflow: drop DB + re-run. Followup confirmed: keep golang-migrate (don't drop the tool), just keep ONE evolving 002_catalog_v0_1 for the whole milestone. Post-deploy freeze, ratchet from 003+ for v0.2 changes. Reasoning given: "đơn giản hóa cho v0.1, chưa triển khai mà nhiều file quá thì tao rất ghét".

### Q2: sqlc query file layout

| Option | Description | Selected |
|--------|-------------|----------|
| One file per entity (7 files in queries/) | agents.sql, skills.sql, …, agent_skills.sql. | ✓ |
| One catalog.sql | All ~30 queries in one big file. | |
| Two files: entities + relations | Arbitrary split. | |

**User's choice:** One file per entity (Recommended)
**Notes:** Diverges from the migration choice — queries grow per-entity and benefit from separation more than migrations do.

### Q3: Cursor pagination encoding

| Option | Description | Selected |
|--------|-------------|----------|
| Base64(JSON {created_at, id}) keyset | Opaque to client; composite key stable under inserts. | ✓ |
| Base64(id-only) keyset | UUIDv7 is time-ordered; simpler. | |
| Offset/limit (?page=N&size=M) | Violates CAT-10 ("cursor-based"). | |

**User's choice:** Base64(JSON {created_at, id}) keyset cursor (Recommended)
**Notes:** SQL pattern `WHERE (created_at, id) < ($cts, $cid) ORDER BY created_at DESC, id DESC LIMIT $page+1`.

### Q4: Case-insensitive name search index

| Option | Description | Selected |
|--------|-------------|----------|
| ILIKE '%foo%' + functional index on lower(name) | Stock PG17; v0.1 scale tolerates leading-wildcard scan. | ✓ |
| pg_trgm extension + gin index | Real substring at scale; needs extension install. | |
| Postgres FTS (tsvector) | Stemming would surprise users; spec says substring. | |

**User's choice:** ILIKE '%foo%' + functional lower(name) index (Recommended)
**Notes:** pg_trgm deferred to v0.2 if scale crosses 10K rows per org per entity.

---

## Handler package layout

### Q1: Package structure for 41 StrictServerInterface methods

| Option | Description | Selected |
|--------|-------------|----------|
| Single internal/catalog/, one .go per entity | 7 entity files + cache.go + cursor.go + validation.go. | ✓ |
| Per-entity packages (6 + helpers + relations) | agents, skills, queues, … each own a package. | |
| internal/domain/ + internal/catalog/ split | DDD-flavored layering. | |

**User's choice:** Single internal/catalog/ package, one .go file per entity (Recommended)
**Notes:** Shared helpers live next to callers; composite server stays thin.

### Q2: Composite server pattern after Phase 3

| Option | Description | Selected |
|--------|-------------|----------|
| Delete stubs.go + scaffold; catalog.Handlers IS the StrictServerInterface | No composite for Phase 3; revisit at Phase 4 boundary. | ✓ |
| Keep compositeServer pattern | Explicit delegation. | |
| Use api.UnimplementedStrictServerInterface embedding | Depends on oapi-codegen behavior. | |

**User's choice:** Delete stubs.go + scaffold.go; catalog.Handlers IS the StrictServerInterface (Recommended)
**Notes:** Phase 4 will add forward-compat (next question).

### Q3: Phase 4/5 extension pattern

| Option | Description | Selected |
|--------|-------------|----------|
| Struct embedding: type ApiHandlers struct { *catalog; *state; *imports } | Method-set merger; compile-time conflict detection. | ✓ |
| Single growing catalog.Handlers | All methods accumulate here. | |
| Decide at Phase 4 boundary | Punt the decision. | |

**User's choice:** Struct embedding (Recommended)
**Notes:** Each phase owns its file; no central composite to edit; conflicts surface at compile time.

### Q4: Constructor pattern for dependency wiring

| Option | Description | Selected |
|--------|-------------|----------|
| Constructor: catalog.New(deps Deps) | Required fields in typed struct. | partial |
| Functional options: catalog.New(WithCache(c), ...) | Every dep optional. | partial |
| Global vars | Set at startup; hard to test. | |

**User's choice:** "Cả 1 & 2 thì sao?" — hybrid New(Deps, ...Option)
**Notes:** Required deps in `Deps` struct (compile-error if missing); optional knobs via `...Option`. In v0.1 only `WithClock` is exposed.

### Q5: Cache helper package location

| Option | Description | Selected |
|--------|-------------|----------|
| Standalone internal/cache/ package | Reusable in Phases 4/5; isolated tests. | ✓ |
| Inside catalog/cache.go | Less navigation; Phase 4 must reach in. | |
| Inside internal/middleware/cache.go | Misleading location. | |

**User's choice:** Standalone internal/cache/ package (Recommended)

### Q6: Test layout

| Option | Description | Selected |
|--------|-------------|----------|
| Per-entity _test.go beside source | catalog/agents_test.go, etc. Cross-org tests in services/api/test/isolation/. | ✓ |
| Single catalog/handlers_test.go with table matrix | Table-driven entity loop. | |
| services/api/test/integration/catalog/ | Separate package, black-box. | |

**User's choice:** Per-entity _test.go beside source (Recommended)

---

## Concurrency 409 + validation

### Q1: Optimistic concurrency SQL pattern

| Option | Description | Selected |
|--------|-------------|----------|
| Single UPDATE … WHERE version=$N RETURNING *; if 0 rows → SELECT | Atomic check-and-increment in one RT; disambig on miss. | ✓ |
| Read-then-write with SELECT FOR UPDATE | Holds row lock; serializes writers. | |
| Postgres xmin-based (no version column) | Violates CAT-08 (requires `version` field). | |

**User's choice:** Single statement: UPDATE … WHERE id=$id AND version=$version RETURNING * (Recommended)

### Q2: Cache DEL on 409

| Option | Description | Selected |
|--------|-------------|----------|
| Del cache on 409 too | Conflict suggests possible cache staleness. | ✓ |
| Don't Del on 409 | No actual mutation happened. | |
| Refresh cache with fresh row on 409 | Race-prone; just DEL is safer. | |

**User's choice:** Del cache on 409 too (Recommended)

### Q3: Validation layering

| Option | Description | Selected |
|--------|-------------|----------|
| oapi-codegen schema + handler-layer cross-row | Layer 1 auto (400); layer 2 manual (422). | ✓ |
| DB FK + trigger | Defense in depth; triggers hard to debug. | |
| internal/domain/validate package | Over-engineered for v0.1's ~4 cross-row rules. | |

**User's choice:** Schema validation by oapi-codegen + cross-row checks in handler (Recommended)

### Q4: Status code for cross-row validation failures

| Option | Description | Selected |
|--------|-------------|----------|
| Add `invalid_reference` ErrorCode + 422 to spec | Phase 3 amendment via codegen-drift. | ✓ |
| Reuse `invalid_body` with reason text | Defeats closed-enum autocomplete. | |
| Return 409 with cross_org or invalid_transition | Semantically wrong. | |

**User's choice:** Add `invalid_reference` ErrorCode + 422 response to spec (Recommended)
**Notes:** Phase 2's D-32 freeze was on PATHS, not on refining error responses. Spec amendment flows through codegen-drift cleanly. Now 11 ErrorCode values total.

---

## Claude's Discretion

- Exact slog attribute keys for cache observability (`outcome`, `cache_key`, etc.).
- Go file ordering inside `internal/catalog/` (alphabetical default).
- Default page size between 25 and 50 (locked at 25 in D-67).
- `pgError` 23505 unique violation: planner picks 409 (`version_conflict`-flavored body for external_id collisions).
- Two `List` queries vs one with `$show_disabled` bool param.
- Mapper file naming (`mappers.go` vs `convert.go`).

---

## Deferred Ideas

- Bloom filter for 404 protection — revisit in v0.2 if metrics show DB pressure
- OTel cache_ops_total counter export — wait until v0.2 metrics pipeline lands
- 5-10s negative caching sentinel — fallback before bloom
- TTL jitter (60s ± 5s) — refresh-ahead supersedes; fallback if refresh-ahead struggles
- pg_trgm + gin index for name search — v0.2 or scale-driven
- Database-level FK constraints across catalog tables — application-layer only in v0.1 (soft-delete-vs-FK semantic mismatch)
- Cache for list endpoints — CAT-11 specs single-entity only
- `cache.Health()` integration with `/readyz` — Phase 1's `rdb.Ping` is enough
- Cursor signing / HMAC — defer to security hardening phase
- Mapper code generation (e.g., goverter) — manual mappers OK for 6 entities
- Audit-trail tables for catalog writes — v1 concern
