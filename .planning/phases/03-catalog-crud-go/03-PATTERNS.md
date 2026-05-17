# Phase 3: Catalog CRUD (Go) - Pattern Map

**Mapped:** 2026-05-16
**Files analyzed:** ~38 new/modified files (including 7 entity handlers, 7 sqlc query files, 1 migration pair, 1 cache package, 6 catalog shared files, 4 wiring edits, 4 deletions, 1 spec amendment, 1 cross-org test extension)
**Analogs found:** 33 / 38 (5 NEW patterns — cache package, cursor encoding, errors.go pgx mapper, mappers.go, notImplemented helper — derive from CONTEXT/RESEARCH excerpts since no analog exists yet)
**Padded phase:** 03

> **CRITICAL FOR PLANNER:** All pattern hazards in §6 are exhaustiveness-gated by `TestRequestIDInjection_Exhaustiveness` (a 121-subtest matrix at `services/api/internal/server/request_id_exhaustiveness_test.go`). Every new `*JSONResponse` wrapper type that Phase 3's spec amendment introduces (e.g., `CreateChannel422JSONResponse`, `UpdateAgent422JSONResponse`, etc.) MUST be added to BOTH the type switch in `server.go:injectRequestIDIntoErrorResponse` AND the test. Same applies to the `CreateAgent409` union wrapper type which has different semantics than the flat 409 wrappers — see Pattern 4 below.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `services/api/internal/cache/cache.go` | cache infra | request-response | RESEARCH.md §Pattern 3 + main.go redis init | role-match (no in-tree analog) |
| `services/api/internal/cache/cache_test.go` | cache test | request-response | `scaffold/handler_test.go` (testcontainer scaffolding, abstracted to miniredis) | role-match |
| `services/api/internal/catalog/handlers.go` | wiring/scaffold | n/a | `scaffold/handler.go` Handler struct + NewHandler constructor | exact (role + structural intent identical) |
| `services/api/internal/catalog/agents.go` | controller | CRUD + cross-row | `scaffold/handler.go` (CRUD shape) + RESEARCH.md Pattern 1/2/5/6 | exact (CRUD shape) + role-match (cache/version/skills additions) |
| `services/api/internal/catalog/skills.go` | controller | CRUD | `scaffold/handler.go` | exact |
| `services/api/internal/catalog/queues.go` | controller | CRUD (with array field) | `scaffold/handler.go` + RESEARCH.md Pattern 5 | exact + role-match |
| `services/api/internal/catalog/channels.go` | controller | CRUD + cross-row FK | `scaffold/handler.go` + RESEARCH.md Pattern 5 | exact + role-match |
| `services/api/internal/catalog/adapters.go` | controller | CRUD (with JSONB) | `scaffold/handler.go` + RESEARCH.md Pitfall 9 | exact + role-match |
| `services/api/internal/catalog/break_reasons.go` | controller | CRUD | `scaffold/handler.go` | exact |
| `services/api/internal/catalog/agent_skills.go` | controller (join) | event-driven (tx-wrapped) | RESEARCH.md Pattern 6 (no in-tree analog) | role-match |
| `services/api/internal/catalog/mappers.go` | mapper | transform | `scaffold/handler.go:toAPIScaffold` | exact |
| `services/api/internal/catalog/cursor.go` | utility | encode/decode | RESEARCH.md Pattern 4 (no in-tree analog) | role-match |
| `services/api/internal/catalog/errors.go` | utility | transform | `server.go:injectRequestIDIntoErrorResponse` (type-switch pattern only) | role-match |
| `services/api/internal/catalog/testutil.go` | test scaffold | n/a | `isolation/main_test.go` TestMain bring-up | role-match |
| `services/api/internal/catalog/*_test.go` | tests | CRUD | `scaffold/handler_test.go` (httptest + testcontainer + ctx injection) | exact |
| `services/api/internal/db/queries/agents.sql` | sqlc query | CRUD + paginated SELECT + version-checked UPDATE | `services/api/internal/db/queries/scaffold.sql` + RESEARCH.md Pattern 2/4 | exact (basic shape) + role-match (UPDATE+RETURNING, cursor tuple) |
| `services/api/internal/db/queries/skills.sql` | sqlc query | CRUD | `scaffold/scaffold.sql` | exact |
| `services/api/internal/db/queries/queues.sql` | sqlc query | CRUD + array | `scaffold/scaffold.sql` + sqlc pgtype.Array docs | role-match |
| `services/api/internal/db/queries/channels.sql` | sqlc query | CRUD + FK existence probe | `scaffold/scaffold.sql` | role-match |
| `services/api/internal/db/queries/adapters.sql` | sqlc query | CRUD + JSONB | `scaffold/scaffold.sql` | role-match |
| `services/api/internal/db/queries/break_reasons.sql` | sqlc query | CRUD | `scaffold/scaffold.sql` | exact |
| `services/api/internal/db/queries/agent_skills.sql` | sqlc query (join) | bulk INSERT + DELETE | `scaffold/scaffold.sql` (basic syntax) | role-match |
| `migrations/000002_catalog_v0_1.up.sql` | migration | schema | `migrations/000001_create_scaffold.up.sql` | exact |
| `migrations/000002_catalog_v0_1.down.sql` | migration | schema | RESEARCH.md §Common Operation 3 (recreate _scaffold pattern) | role-match |
| `services/api/test/isolation/catalog_test.go` | integration test | cross-org probe | `services/api/test/isolation/isolation_test.go` (Case 2) | exact |
| `services/api/test/isolation/main_test.go` | test wiring | n/a | itself — EDIT for new catalog handler construction | exact |
| `services/api/test/isolation/isolation_test.go` | test | cross-org | itself — EDIT to remove _scaffold or keep alongside | exact |
| `openapi/openapi.yaml` | spec-edit | n/a | itself — EDIT to add `invalid_reference` enum + 422 responses | exact (in-place amendment) |
| `services/api/internal/server/server.go` | wiring | n/a | itself — EDIT NewMux signature + add 422 type switch cases | exact (in-place amendment) |
| `services/api/internal/server/request_id_exhaustiveness_test.go` | test | n/a | itself — EDIT to add 422 cases | exact |
| `services/api/cmd/api/main.go` | wiring | n/a | itself — EDIT to construct cache.New + catalog.New | exact (in-place amendment) |
| `services/api/go.mod` | config | n/a | itself — EDIT `go get` deltas | exact |
| `Taskfile.yml` | config | n/a | itself — EDIT (optional `task db:reset`) | exact |
| `services/api/internal/scaffold/` | delete | n/a | N/A (D-77 deletion) | n/a |
| `services/api/internal/server/stubs.go` | delete | n/a | N/A (D-77 deletion) | n/a |
| `services/api/internal/db/queries/scaffold.sql` | delete | n/a | N/A (D-77 deletion) | n/a |
| `services/api/internal/api/*.gen.go` | codegen output | n/a | regenerated via `task gen` | n/a (DO NOT hand-edit) |
| `services/api/internal/db/generated/*.sql.go` | codegen output | n/a | regenerated via `task gen` | n/a (DO NOT hand-edit) |
| `web/packages/ui/src/api/generated.ts` | codegen output | n/a | regenerated via `pnpm gen:api` | n/a (DO NOT hand-edit) |

## Pattern Assignments

### `services/api/internal/cache/cache.go` (cache infra, NEW)

**Analog rationale:** No in-tree analog exists — Phase 3 introduces caching for the first time. Pattern derives from RESEARCH.md §Pattern 3 + main.go redis client init. Hazard: Use `context.WithoutCancel(ctx)` for refresh goroutines (Go 1.21+, locked at `go 1.25`) — `context.Background()` strips org_id + trace span values and triggers `db.ErrOrgIDMissingFromContext` panic in dev/test.

**Reference pattern from `services/api/cmd/api/main.go` lines 102-108 (Redis client construction baseline):**

```go
// (5) Redis client. ParseURL handles "redis://host:port/db" and
// "rediss://..." for TLS.
redisOpts, err := redis.ParseURL(cfg.RedisURL)
if err != nil {
    slog.ErrorContext(ctx, "redis url parse", "err", err)
    return 1
}
rdb := redis.NewClient(redisOpts)
defer func() { _ = rdb.Close() }()
```

**Locked construction signature (D-50, D-51):**

```go
package cache

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log/slog"
    "time"

    "github.com/redis/go-redis/v9"
    "golang.org/x/sync/singleflight"
)

var ErrNotFound = errors.New("cache: not found")

const refreshThreshold = 10 * time.Second  // D-53

type Cache struct {
    rdb    *redis.Client
    sf     *singleflight.Group
    logger *slog.Logger
}

func New(rdb *redis.Client, logger *slog.Logger) *Cache {
    return &Cache{rdb: rdb, sf: &singleflight.Group{}, logger: logger}
}

// Key builds the canonical cache key "or:{orgID}:{entity}:{id}" (D-58).
func Key(orgID, entity, id any) string {
    return fmt.Sprintf("or:%s:%s:%s", orgID, entity, id)
}

// GetOrSet (D-51) — see RESEARCH.md Pattern 3 lines 499-546 for full body.
func GetOrSet[T any](ctx context.Context, c *Cache, key string, ttl time.Duration,
    load func(context.Context) (T, error)) (T, error) {
    // ... hit path + refresh-ahead + miss singleflight ...
}

// Del removes a cache entry. Returns an error from go-redis but
// callers MUST log+continue on failure (D-55).
func (c *Cache) Del(ctx context.Context, key string) error {
    return c.rdb.Del(ctx, key).Err()
}
```

**Hazard:** Refresh-ahead goroutine MUST use `context.WithoutCancel(ctx)` (Go 1.21+) — `context.Background()` strips org_id from ctx and crashes orgDB in dev/test (Pitfall 4).

**Hazard:** Cache failures NEVER turn a successful DB write into 5xx (D-55). Pattern: `if delErr := h.deps.Cache.Del(ctx, key); delErr != nil { logger.WarnContext(ctx, ...) }` — do not `return ...500`.

---

### `services/api/internal/cache/cache_test.go` (cache unit test)

**Analog:** `services/api/internal/scaffold/handler_test.go` lines 67-135 (TestMain testcontainer bring-up pattern). For cache tests, the testcontainer is replaced by miniredis to keep the unit-level suite fast and `-short`-clean.

**Imports pattern (adapt from scaffold handler_test.go lines 17-49):**

```go
import (
    "context"
    "testing"
    "time"

    "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/require"

    "github.com/luongdev/open-routing/services/api/internal/cache"
    "github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)
```

**Test fixture pattern (derive from scaffold_test.go lines 195-242):**

```go
func newTestCache(t *testing.T) (*cache.Cache, *miniredis.Miniredis) {
    t.Helper()
    s := miniredis.RunT(t)
    rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
    t.Cleanup(func() { _ = rdb.Close() })
    return cache.New(rdb, slog.New(slog.NewTextHandler(io.Discard, nil))), s
}
```

**Hazard:** Tests for refresh-ahead need `s.FastForward(50 * time.Second)` to drop PTTL into the <10s window. Without that, the refresh path never fires.

**Hazard:** Tests for `GetOrSet[T]` with `ErrNotFound` loader MUST assert the key is NOT written to Redis (miniredis `s.Exists(key)` should be false). D-54 mandates no negative caching.

---

### `services/api/internal/catalog/handlers.go` (Deps struct + New constructor)

**Analog:** `services/api/internal/scaffold/handler.go` lines 29-46.

**Imports pattern from scaffold (lines 14-27):**

```go
import (
    "context"
    "errors"
    "strings"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/luongdev/open-routing/services/api/internal/api"
    "github.com/luongdev/open-routing/services/api/internal/db"
    "github.com/luongdev/open-routing/services/api/internal/db/generated"
    "github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)
```

**Struct pattern (extend scaffold's pattern with cache + logger + clock):**

```go
// scaffold/handler.go lines 29-46 — minimal Handler shape:
type Handler struct {
    orgDB *db.OrgDB
}

func NewHandler(orgDB *db.OrgDB) *Handler {
    return &Handler{orgDB: orgDB}
}
```

**Phase 3 extension (per D-71 — Deps struct + functional Options):**

```go
package catalog

import (
    "log/slog"
    "time"

    "github.com/luongdev/open-routing/services/api/internal/cache"
    "github.com/luongdev/open-routing/services/api/internal/db"
)

// Deps bundles required dependencies. Compile-error if a field is missing
// at New() callsite — typed struct catches misconfiguration at build time.
type Deps struct {
    OrgDB  *db.OrgDB     // required (see OQ-4 in research — keep existing shared instance)
    Cache  *cache.Cache  // required
    Logger *slog.Logger  // required
}

// Option enables optional knobs without expanding Deps. Only WithClock in v0.1.
type Option func(*Handlers)

func WithClock(c func() time.Time) Option {
    return func(h *Handlers) { h.clock = c }
}

type Handlers struct {
    deps  Deps
    clock func() time.Time
}

func New(deps Deps, opts ...Option) *Handlers {
    h := &Handlers{deps: deps, clock: time.Now}
    for _, opt := range opts {
        opt(h)
    }
    return h
}
```

**notImplemented helper (D-69):**

```go
// notImplemented returns a 501-flavored 500 stub for Phase 4/5 methods that
// catalog.Handlers must declare (since it implements StrictServerInterface)
// but does not implement until those phases land.
// Returns a value of the operation-specific *500JSONResponse type via a
// generic factory — planner picks: per-method tiny wrapper OR a private
// helper that takes the response type via type parameter. Either way the
// reason string is "not_implemented" so RequestIDInjection still injects.
```

**Hazard:** OQ-4 from RESEARCH.md (line 1048-1052) — `db.OrgDBFactory` is wording from CONTEXT.md D-71 but no `db.Factory` type exists. The right pattern is the existing shared `*db.OrgDB` (matching scaffold). Planner: use `Deps.OrgDB *db.OrgDB`, NOT a new factory abstraction.

---

### `services/api/internal/catalog/agents.go` (controller, CRUD + version + skills + cache)

**Analog (primary):** `services/api/internal/scaffold/handler.go` lines 55-162 — the GET/POST/LIST pattern. CRUD shape is identical; Phase 3 wraps the DB load in `cache.GetOrSet` (D-49) and adds version-checked UPDATE + skills replace.

**Auth pattern from scaffold (lines 56-67) — verbatim across all handlers:**

```go
func (h *Handler) CreateScaffold(ctx context.Context, req api.CreateScaffoldRequestObject) (api.CreateScaffoldResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        // Defensive guard: OrgContext middleware guarantees presence inside
        // /v1; this branch only fires on a wiring regression. RequestId
        // stays empty here; server.RequestIDInjectionMiddleware fills it.
        return api.CreateScaffold500JSONResponse{
            InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
                Error:  api.ErrorCodeInternal,
                Reason: "missing_org_id_in_context",
            },
        }, nil
    }
```

**Core Create pattern (scaffold/handler.go lines 77-94):**

```go
q := generated.New(h.orgDB)
id := uuid.Must(uuid.NewV7()) // D-19 UUIDv7 everywhere
row, err := q.InsertScaffold(ctx, generated.InsertScaffoldParams{
    ID:         toPgUUID(id),
    OrgID:      toPgUUID(orgID),
    ExternalID: req.Body.ExternalId,
    Name:       req.Body.Name,
})
if err != nil {
    return api.CreateScaffold500JSONResponse{
        InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error:  api.ErrorCodeInternal,
            Reason: "insert_failed",
        },
    }, nil
}
return api.CreateScaffold201JSONResponse(toAPIScaffold(row)), nil
```

**Core Get pattern with FOUND-08 leakage guard (scaffold/handler.go lines 128-162):**

```go
func (h *Handler) GetScaffoldById(ctx context.Context, req api.GetScaffoldByIdRequestObject) (api.GetScaffoldByIdResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok { /* 500 */ }
    q := generated.New(h.orgDB)
    row, err := q.GetScaffoldByID(ctx, generated.GetScaffoldByIDParams{
        ID:    toPgUUID(uuid.UUID(req.Id)),
        OrgID: toPgUUID(orgID),
    })
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return api.GetScaffoldById404JSONResponse{
                NotFoundJSONResponse: api.NotFoundJSONResponse{
                    Error:  api.ErrorCodeNotFound,
                    Reason: "no_such_scaffold",
                },
            }, nil
        }
        return api.GetScaffoldById500JSONResponse{...}, nil
    }
    return api.GetScaffoldById200JSONResponse(toAPIScaffold(row)), nil
}
```

**Phase 3 extension — cache wrap (D-49) per RESEARCH.md Pattern 1 lines 367-401:**

```go
// GET by ID — cache wraps the DB load:
key := cache.Key(orgID, "agents", uuid.UUID(req.Id))
agent, err := cache.GetOrSet[api.Agent](ctx, h.deps.Cache, key, 60*time.Second,
    func(ctx context.Context) (api.Agent, error) {
        q := generated.New(h.deps.OrgDB)
        row, err := q.GetAgent(ctx, generated.GetAgentParams{
            ID:    pgUUID(uuid.UUID(req.Id)),
            OrgID: pgUUID(orgID),
        })
        if errors.Is(err, pgx.ErrNoRows) {
            return api.Agent{}, cache.ErrNotFound
        }
        if err != nil { return api.Agent{}, err }
        // load embedded skills (CAT-03; only on detail GET, OQ-1A)
        skills, err := q.ListSkillsForAgent(ctx, ...)
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
```

**Phase 3 extension — version-checked UPDATE + 404/409 disambiguation (D-66, D-56) per RESEARCH.md Pattern 2:**

See RESEARCH.md lines 422-462 for the full pattern. Key invariant:
- Atomic UPDATE `WHERE id = $1 AND org_id = $2 AND version = $3 RETURNING *`
- 0 rows → fallback SELECT to disambiguate 404 (no row) vs 409 (version mismatch)
- On 409: cache.Del + return `UpdateAgent409JSONResponse{Current: ..., Error: "version_conflict", Reason: "version_mismatch"}`
- On success: cache.Del with log-on-fail (D-55)

**Phase 3 extension — agent_skills full-replace inside tx (D-76 + CAT-03 + OQ-1A PUT-semantics):**

See RESEARCH.md lines 666-696 for the full pattern. Key invariant: the agent UPDATE + DELETE agent_skills + INSERT new agent_skills MUST be ONE transaction. See open question #5 in RESEARCH.md — orgDB needs a `BeginTx` method to preserve SQL validation OR plan accepts bypass; planner picks. **Strong recommendation:** add `(*OrgDB).BeginTx(ctx) (*OrgTx, error)` in Wave 1 to keep validator engaged.

**Hazard:** `req.Id` is `EntityIdPath` (alias for `openapi_types.UUID` = `uuid.UUID`). Convert to `pgtype.UUID` via the helper. The CAT-08 versioning DOES apply to agents; CHANNEL and ADAPTER currently lack `version` in spec — Pitfall 8 calls this out, planner picks whether to amend the spec to add `version` to those entities (recommended yes).

---

### `services/api/internal/catalog/{skills,queues,break_reasons}.go` (controller, CRUD)

**Analog:** `services/api/internal/scaffold/handler.go` (full file, lines 1-188) — the GET/POST/LIST shape applies directly. Add UPDATE with version check (D-66), soft-delete via UPDATE enabled=false (RESEARCH.md §Common Operation 2 lines 828-853), and cache wrap on GET (D-49).

**Hazard for `queues.sql`:** `channel_types` is `TEXT[]` in Postgres (per migration in RESEARCH.md line 905). sqlc + pgx/v5 emits `[]string` for `TEXT[]` columns — confirm at first sqlc gen. The api `Queue.ChannelTypes` is `[]ChannelType` (string-aliased enum), so the mapper needs `[]string` → `[]api.ChannelType` conversion.

**Hazard for `break_reasons.sql`:** No `external_id` column — `name` is the user-facing unique key, UNIQUE `(org_id, name)` constraint (RESEARCH.md migration line 958). DO NOT carry the `external_id` pattern from agents.sql blindly.

---

### `services/api/internal/catalog/channels.go` (controller, CRUD + FK cross-row)

**Analog:** `services/api/internal/scaffold/handler.go` (CRUD shape) + RESEARCH.md §Pattern 5 lines 642-660 (FK existence probe before write).

**Cross-row validation pattern (RESEARCH.md Pattern 5):**

```go
// channels.go — CreateChannel checks default_queue_id exists + enabled in same org
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
```

**Hazard:** Channel has NO `version` field in current spec (Pitfall 8). Planner should amend spec to add `version` for CAT-08 parity OR document the v0.1 exception explicitly.

**Hazard:** Spec amendment (D-75) generates new `CreateChannel422JSONResponse`, `UpdateChannel422JSONResponse` types. These MUST be added to BOTH `server.go:injectRequestIDIntoErrorResponse` and `request_id_exhaustiveness_test.go`. Build-gated by the exhaustiveness test.

---

### `services/api/internal/catalog/adapters.go` (controller, CRUD + JSONB)

**Analog:** `services/api/internal/scaffold/handler.go` (CRUD shape).

**Hazard:** `config` is `JSONB` — sqlc + pgx/v5 emits `[]byte` by default (Pitfall 9, RESEARCH.md line 791-795). Handler must `json.Marshal(req.Body.Config)` on write (`*map[string]interface{}` → `[]byte`) and `json.Unmarshal(row.Config, &adapter.Config)` on read. Pattern:

```go
// helper in mappers.go:
func marshalAdapterConfig(cfg *map[string]interface{}) ([]byte, error) {
    if cfg == nil { return []byte("{}"), nil }  // jsonb DEFAULT '{}'
    return json.Marshal(*cfg)
}
func unmarshalAdapterConfig(b []byte) (*map[string]interface{}, error) {
    if len(b) == 0 { return nil, nil }
    var m map[string]interface{}
    if err := json.Unmarshal(b, &m); err != nil { return nil, err }
    return &m, nil
}
```

**Hazard:** Adapter has NO `external_id` field (RESEARCH.md line 102, line 936 of migration). The UNIQUE constraint is omitted; only `id` is unique.

---

### `services/api/internal/catalog/agent_skills.go` (controller/join, tx-wrapped bulk)

**Analog:** None in-tree. Derive from RESEARCH.md §Pattern 6 lines 667-696.

**Core pattern (RESEARCH.md Pattern 6 verbatim — full-replace inside single tx):**

```go
// PUT-semantics on the skills join (OQ-1A resolution per Phase 2)
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
    tx, err := h.deps.Pool.Begin(ctx)  // ← see OQ-5 — recommend orgDB.BeginTx wrapper
    if err != nil { /* 500 */ }
    defer tx.Rollback(ctx)
    qtx := generated.New(tx) // sqlc.DBTX satisfied by pgx.Tx
    if _, err := qtx.DeleteAgentSkills(ctx, ...); err != nil { /* 500 */ }
    for _, s := range *req.Body.Skills {
        if _, err := qtx.InsertAgentSkill(ctx, ...); err != nil { /* 500 */ }
    }
    if err := tx.Commit(ctx); err != nil { /* 500 */ }
}
```

**Hazard (Pitfall 5):** Without a single tx wrapping DELETE + INSERTs, a partial-failure leaves the agent with a wrong skill set. Add a test that injects a mid-INSERT error and asserts the original skills survive.

**Hazard (OQ-5):** Phase 1's orgDB has no `BeginTx`. Without it, you either bypass orgDB validation for the tx (loses FOUND-04 enforcement) OR add `(*OrgDB).BeginTx(ctx) (*OrgTx, error)` to keep validator engaged. Planner: lean toward adding the BeginTx wrapper in Wave 1.

---

### `services/api/internal/catalog/mappers.go` (utility, transform)

**Analog:** `services/api/internal/scaffold/handler.go` lines 164-187 — the `toPgUUID` + `toAPIScaffold` pattern.

**Verbatim pattern from scaffold (lines 167-187):**

```go
// toPgUUID wraps a google/uuid value into pgtype.UUID for sqlc parameter
// passing. Valid=true because every UUID we pass is a real non-null value
// (NOT NULL columns in _scaffold).
func toPgUUID(id uuid.UUID) pgtype.UUID {
    return pgtype.UUID{Bytes: id, Valid: true}
}

// toAPIScaffold converts the sqlc-generated row into the api.Scaffold type.
// pgtype.UUID.Bytes is the raw 16-byte UUID; Valid is always true for NOT
// NULL columns. pgtype.Timestamptz.Time is the Go time.Time value.
// api.Scaffold.CreatedAt is *time.Time (omitempty); we set it only when valid.
func toAPIScaffold(s generated.Scaffold) api.Scaffold {
    out := api.Scaffold{
        Id:         uuid.UUID(s.ID.Bytes),
        OrgId:      uuid.UUID(s.OrgID.Bytes),
        ExternalId: s.ExternalID,
        Name:       s.Name,
    }
    if s.CreatedAt.Valid {
        t := s.CreatedAt.Time
        out.CreatedAt = &t
    }
    return out
}
```

**Phase 3 extension — one mapper per entity:**

```go
func pgUUID(id uuid.UUID) pgtype.UUID                 { return pgtype.UUID{Bytes: id, Valid: true} }
func mapAgent(r generated.Agent, skills []generated.AgentSkillRow) api.Agent { ... }
func mapAgentListItem(r generated.Agent) api.AgentListItem                   { ... }
func mapSkill(r generated.Skill) api.Skill                                   { ... }
func mapQueue(r generated.Queue) api.Queue                                   { ... }
func mapChannel(r generated.Channel) api.Channel                             { ... }
func mapAdapter(r generated.Adapter) (api.Adapter, error)                    { ... } // unmarshals JSONB
func mapBreakReason(r generated.BreakReason) api.BreakReason                 { ... }
```

**Hazard:** `pgtype.Timestamptz.Valid` MUST be checked before reading `.Time`. The scaffold pattern (line 182-185) is correct; do not blindly copy `t := s.CreatedAt.Time` without the `if s.CreatedAt.Valid` guard.

**Hazard:** `api.UUIDv7` is `type UUIDv7 = openapi_types.UUID` = `uuid.UUID` (types.gen.go:882). Conversions are aliases, not casts — `api.UUIDv7(parsed)` works because they share the underlying type.

---

### `services/api/internal/catalog/cursor.go` (utility, encode/decode)

**Analog:** None in-tree. Derive from RESEARCH.md §Pattern 4 lines 583-601.

**Locked format (D-63 — base64 of JSON):**

```go
package catalog

import (
    "encoding/base64"
    "encoding/json"
    "errors"
    "time"

    "github.com/google/uuid"
)

var errBadCursor = errors.New("cursor: malformed")

type cursorPayload struct {
    CreatedAt time.Time `json:"created_at"`  // RFC3339Nano
    ID        uuid.UUID `json:"id"`           // UUIDv7
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
```

**Hazard (Pitfall 3):** The cursor JSON drops microseconds if you use `time.Time.Format(time.RFC3339)` instead of `RFC3339Nano`. The sqlc-generated `pgtype.Timestamptz.Time` carries microsecond precision; cursor round-trip must preserve it or pagination silently skips/duplicates rows.

**Hazard (Pitfall 9 — security):** Cursors are plaintext base64(JSON). Clients can craft cursors. Server filters by org_id still apply so worst case is observing one's own ID space. HMAC-signed cursors deferred to v0.2 security hardening.

---

### `services/api/internal/catalog/errors.go` (utility, transform pgx → response)

**Analog:** No in-tree analog for centralized pgx-error mapping (scaffold inlines the check). The type-switch shape derives from `server.go:injectRequestIDIntoErrorResponse` lines 205-610 — pattern: type-switch over a known set, default returns input.

**Skeleton:**

```go
package catalog

import (
    "errors"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
)

// IsUniqueViolation returns true when err is a pgconn.PgError 23505
// (unique_violation). Used by Create handlers to detect external_id
// conflicts → 409 version_conflict-flavored response.
func IsUniqueViolation(err error) bool {
    var pgErr *pgconn.PgError
    if !errors.As(err, &pgErr) { return false }
    return pgErr.Code == "23505"
}

// IsNoRows is a thin wrapper over errors.Is for pgx.ErrNoRows.
// Distinguished here so handlers grep for one canonical check.
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
```

**Hazard:** Default disposition for unique_violation on CREATE is `409 with version_conflict-flavored body` for `external_id` collisions per CONTEXT.md Claude's Discretion. Planner: pick 409 with reason "external_id_conflict" (NOT 422; clients differentiate by error code).

---

### `services/api/internal/catalog/testutil.go` (test scaffold)

**Analog:** `services/api/test/isolation/main_test.go` (full file lines 75-207) — TestMain testcontainer + httptest server bring-up; adapt to package-local sharedPool + miniredis.

**Imports pattern (isolation/main_test.go lines 18-43, slimmed):**

```go
import (
    "context"
    "database/sql"
    "errors"
    "flag"
    "os"
    "testing"
    "time"

    "github.com/alicebob/miniredis/v2"
    "github.com/golang-migrate/migrate/v4"
    pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/jackc/pgx/v5/pgxpool"
    _ "github.com/jackc/pgx/v5/stdlib"
    "github.com/redis/go-redis/v9"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)
```

**TestMain pattern (isolation/main_test.go lines 67-135 + scaffold/handler_test.go lines 137-169 buildStrictHandler):**

```go
// Pseudocode — combines testcontainer + miniredis + catalog handler:
func TestMain(m *testing.M) {
    if !flag.Parsed() { flag.Parse() }
    if testing.Short() { os.Exit(m.Run()); return }
    // 1) Postgres testcontainer + migrate up (lines 88-146 of main_test.go)
    // 2) miniredis (per-test if isolation needed, or shared at package level)
    // 3) sharedPool = pgxpool.New
    // 4) On Run, each _test.go builds a *Handlers via testHandlers(t)
}

func testHandlers(t testing.TB) *catalog.Handlers {
    t.Helper()
    s := miniredis.RunT(t)
    rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
    t.Cleanup(func() { _ = rdb.Close() })

    orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)
    return catalog.New(catalog.Deps{
        OrgDB:  orgDB,
        Cache:  cache.New(rdb, slog.New(slog.NewTextHandler(io.Discard, nil))),
        Logger: slog.Default(),
    })
}
```

**buildStrictHandler analog (scaffold/handler_test.go lines 153-169):**

```go
func buildStrictHandler(catalogHandlers *catalog.Handlers) http.Handler {
    strictHandler := api.NewStrictHandler(
        catalogHandlers, // D-69: catalog IS the StrictServerInterface impl now
        []api.StrictMiddlewareFunc{server.RequestIDInjectionMiddleware()},
    )
    r := chi.NewRouter()
    r.Use(appmw.RequestID)
    api.HandlerFromMux(strictHandler, r)
    return r
}
```

**callWithOrg pattern (scaffold/handler_test.go lines 176-190 — verbatim):**

```go
func callWithOrg(t *testing.T, h http.Handler, orgID uuid.UUID, method, path string, body []byte) *httptest.ResponseRecorder {
    t.Helper()
    var req *http.Request
    if len(body) > 0 {
        req = httptest.NewRequest(method, path, bytes.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
    } else {
        req = httptest.NewRequest(method, path, nil)
    }
    req = req.WithContext(orgkey.SetOrgID(req.Context(), orgID))
    rec := httptest.NewRecorder()
    h.ServeHTTP(rec, req)
    return rec
}
```

**Hazard (Pitfall 4):** Tests that exercise refresh-ahead need `s.FastForward(50 * time.Second)` to drop PTTL below 10s.

**Hazard:** The `containerFailureExitCode()` pattern (main_test.go lines 56-64) returns 1 in CI, 0 locally — keep this so docker-less local runs don't fail loudly. Reuse it.

**Hazard:** The `freshOrg(t)` pattern (isolation_test.go lines 42-46) — every test mints its own UUIDv7 org_id. Never use a package-level constant org_id (Pitfall 6 in Phase 1).

---

### `services/api/internal/catalog/*_test.go` (per-entity tests)

**Analog:** `services/api/internal/scaffold/handler_test.go` lines 195-346 — the per-test pattern.

**Test naming + structure (scaffold_test.go lines 195-202 + 247-263 + 270-286):**

```go
// TestAgents_CreateAndGet_Roundtrip proves the POST→GET-by-id→GET-list cycle.
func TestAgents_CreateAndGet_Roundtrip(t *testing.T) {
    if testing.Short() { t.Skip("integration: requires postgres testcontainer") }
    if sharedPool == nil { t.Skip("sharedPool unavailable") }
    t.Parallel()
    orgID := uuid.Must(uuid.NewV7())
    h := testHandlersForOrg(t)  // ← from testutil.go
    api := buildStrictHandler(h)

    // POST creates row.
    rec := callWithOrg(t, api, orgID, http.MethodPost,
        fmt.Sprintf("/v1/orgs/%s/agents", orgID), createBody)
    require.Equal(t, http.StatusCreated, rec.Code)
    // …
}

// TestAgents_GetNotFound_Returns404 — cross-org/missing → 404 (FOUND-08).
// TestAgents_UpdateVersionMismatch_Returns409 — CAT-08.
// TestAgents_SoftDelete_HiddenByDefault — CAT-09.
// TestAgents_ListCursorPagination — CAT-10.
// TestAgents_NameSearchILike — CAT-10.
// TestAgents_CacheHitMiss — CAT-11 (uses miniredis assertions).
// TestAgents_SkillsAtomicReplace — CAT-03 PUT-semantics.
```

**Hazard:** The B-1 assertion (scaffold_test.go lines 311-342) MUST also be replicated for each new entity: every error response body must carry `request_id` matching `X-Request-Id` header. This guards against missing cases in the `injectRequestIDIntoErrorResponse` switch.

---

### `services/api/internal/db/queries/agents.sql` (sqlc query, CRUD + paginated + version-checked)

**Analog (basic shape):** `services/api/internal/db/queries/scaffold.sql` lines 1-17.

**Pattern from scaffold (lines 1-17 verbatim):**

```sql
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

**Phase 3 extension — atomic version-checked UPDATE (D-66, RESEARCH.md Pattern 2 lines 407-417):**

```sql
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

**Phase 3 extension — cursor pagination + name filter + LIMIT N+1 (D-63/D-64/D-65, RESEARCH.md Pattern 4 lines 559-579):**

```sql
-- name: ListAgents :many
-- default-list path: enabled=true (D-65)
SELECT id, org_id, external_id, name, email, enabled, version, created_at, updated_at
FROM agents
WHERE org_id = $1
  AND enabled = TRUE
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL OR (created_at, id) < (sqlc.narg('cursor_at'), sqlc.narg('cursor_id')))
  AND (sqlc.narg('name_filter')::text IS NULL OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;  -- N + 1 sentinel

-- name: ListAgentsIncludingDisabled :many
-- ?include_disabled=true: omits enabled=TRUE (D-65)
SELECT id, org_id, external_id, name, email, enabled, version, created_at, updated_at
FROM agents
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL OR (created_at, id) < (sqlc.narg('cursor_at'), sqlc.narg('cursor_id')))
  AND (sqlc.narg('name_filter')::text IS NULL OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;
```

**Phase 3 extension — soft-delete (RESEARCH.md §Common Operation 2 line 848-855):**

```sql
-- name: SoftDeleteAgent :execrows
UPDATE agents SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;
```

**Hazard (Pitfall 3):** Use `sqlc.narg('cursor_at')` for nullable cursor params. Without `narg`, sqlc generates non-pointer params and the IS NULL branch never fires. The sqlc.yaml has `emit_pointers_for_null_types: true` (verified line 13 of sqlc.yaml) so narg emits a `*pgtype.Timestamptz`.

**Hazard:** Every query MUST mention `org_id = $N` in its WHERE / INSERT column list, or the orgDB SQLChecker panics in dev/test (D-02). The pattern is enforced by `db.SQLChecker` at runtime, not at sqlc generation time.

**Hazard:** UPDATE statements MUST include `org_id = $N` in WHERE even when the UPDATE-target ID is already unique — orgDB validator runs on the SQL string, not the planner outcome. Pitfall: a query like `UPDATE agents SET ... WHERE id = $1 AND version = $2` will be REJECTED by SQLChecker.

---

### `migrations/000002_catalog_v0_1.up.sql` (migration, schema)

**Analog:** `migrations/000001_create_scaffold.up.sql` (10 lines) — UNIQUE constraint pattern + index pattern.

**Pattern from existing migration (verbatim):**

```sql
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

**Phase 3 — single editable migration carrying ALL catalog tables + composite DESC indexes + partial functional indexes (RESEARCH.md §Common Operation 3 lines 860-998). See that section verbatim.**

Per D-61, this migration is **EDITABLE in place during v0.1**. Phase 4 will edit it to add `agent_states` columns/CHECK; Phase 5 will edit it to add `import_jobs`. Once v0.1 ships to a real env, this migration freezes and Phase 4/5 changes become migration 003+.

**Hazard:** `DROP TABLE IF EXISTS _scaffold` MUST come BEFORE the new CREATEs to ensure `task db:reset` flow rebuilds cleanly. See RESEARCH.md line 867.

**Hazard (Pitfall 7):** Partial functional index `CREATE INDEX ix_*_org_name ON {entity}(org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE` DOES NOT help substring search `'%foo%'` with leading wildcard — btree text_pattern_ops only supports prefix matches. Index is still useful for prefix searches and as a partial-index gate; document the limitation in a SQL comment in the migration.

**Hazard:** `agent_skills` table has a denormalized `org_id` column (RESEARCH.md line 966 with comment "denormalized to satisfy orgDB SQLChecker on every query"). Without it, the orgDB validator rejects every agent_skills query. Do NOT omit this column.

---

### `migrations/000002_catalog_v0_1.down.sql`

**Analog:** None — Phase 1's `000001_create_scaffold.down.sql` is `DROP TABLE _scaffold;` only.

**Pattern from RESEARCH.md §Common Operation 3 lines 980-998 (recreate _scaffold for migrate-down safety):**

```sql
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

---

### `services/api/test/isolation/catalog_test.go` (integration test, cross-org probe)

**Analog:** `services/api/test/isolation/isolation_test.go` lines 115-137 (Case 2 — TestTwoOrgsIsolation_GetByIDIsScoped).

**Core pattern verbatim (isolation_test.go lines 115-137):**

```go
func TestTwoOrgsIsolation_GetByIDIsScoped(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    orgA := freshOrg(t)
    orgB := freshOrg(t)
    rowA := testsupport.PostScaffold(t, baseURL(), orgA, "ext-shared", "A")
    rowB := testsupport.PostScaffold(t, baseURL(), orgB, "ext-shared", "B")

    // orgA cannot see orgB's row by ID; orgB cannot see orgA's.
    require.Equal(t, http.StatusNotFound,
        testsupport.GetScaffoldStatus(t, baseURL(), orgA, rowB.ID),
        "orgA must not be able to GET orgB's row by id")
    require.Equal(t, http.StatusNotFound,
        testsupport.GetScaffoldStatus(t, baseURL(), orgB, rowA.ID),
        "orgB must not be able to GET orgA's row by id")

    // Each org CAN see its own row (proves the 404 above is not a blanket
    // reject — the row exists, just not for that org).
    require.Equal(t, http.StatusOK,
        testsupport.GetScaffoldStatus(t, baseURL(), orgA, rowA.ID))
    require.Equal(t, http.StatusOK,
        testsupport.GetScaffoldStatus(t, baseURL(), orgB, rowB.ID))
}
```

**Phase 3 extension:** Repeat the same shape for each new entity (agents, skills, queues, channels, adapters, break_reasons, agent_skills). 6 × 4 cases (GET, UPDATE, DELETE, list) = ~24 cross-org probe asserts. Use `testsupport` helpers extended with `PostAgent`, `GetAgentStatus`, etc.

**Hazard:** Cross-org write probe must assert the header X-Org-Id is the source of truth, NOT the URL path. See `TestTwoOrgsIsolation_PostRespectsHeaderOrg` (isolation_test.go lines 147-183) for the canonical pattern.

---

### `services/api/internal/server/server.go` (EDIT — extend injectRequestIDIntoErrorResponse, simplify NewMux)

**Current state to edit:** lines 60-106 (NewMux) + lines 205-610 (injectRequestIDIntoErrorResponse type switch).

**EDIT 1 — NewMux signature simplification (D-69):**

Replace `Deps.StrictHandlers api.StrictServerInterface` with direct injection of `*catalog.Handlers`:

```go
// BEFORE (Phase 2):
type Deps struct {
    Pool           *pgxpool.Pool
    Redis          *redis.Client
    OrgDB          *db.OrgDB
    Config         *config.Config
    StrictHandlers api.StrictServerInterface
    SpecBytes      []byte
}

// AFTER (Phase 3):
type Deps struct {
    Pool      *pgxpool.Pool
    Redis     *redis.Client
    OrgDB     *db.OrgDB
    Config    *config.Config
    Catalog   *catalog.Handlers  // D-69: directly hand catalog as the strict-server impl
    SpecBytes []byte
}
```

**EDIT 2 — add new error type cases to `injectRequestIDIntoErrorResponse` type switch (lines 205-610):**

Reference pattern (lines 209-228 for the base types):

```go
case api.BadRequestJSONResponse:
    v := api.ErrorResponse(r)
    v.RequestId = setIfNil(v.RequestId, id)
    return api.BadRequestJSONResponse(v)
```

Reference pattern (lines 230-248 for embedded-base types with their own discriminator):

```go
case api.CreateChannel409JSONResponse:
    v := api.ErrorResponse(r)
    v.RequestId = setIfNil(v.RequestId, id)
    return api.CreateChannel409JSONResponse(v)
```

Reference pattern (lines 449-453 for VersionConflictErrorResponse variants with custom-typed fields):

```go
case api.UpdateAgent409JSONResponse:
    // VersionConflictErrorResponse variant — has its own RequestId field.
    r.RequestId = setIfNil(r.RequestId, id)
    return r
```

**New cases to add (per D-75 spec amendment):**

```go
// Phase 3 spec amendment (D-75) — 422 invalid_reference on FK-bearing CREATE/UPDATE
case api.CreateChannel422JSONResponse:
    v := api.ErrorResponse(r)
    v.RequestId = setIfNil(v.RequestId, id)
    return api.CreateChannel422JSONResponse(v)
case api.UpdateChannel422JSONResponse:
    v := api.ErrorResponse(r)
    v.RequestId = setIfNil(v.RequestId, id)
    return api.UpdateChannel422JSONResponse(v)
case api.UpdateAgent422JSONResponse:
    v := api.ErrorResponse(r)
    v.RequestId = setIfNil(v.RequestId, id)
    return api.UpdateAgent422JSONResponse(v)
// ... per Wave 0 spec edit, mirror for every CREATE/UPDATE endpoint with FK fields
```

**Hazard:** `CreateAgent409JSONResponse` is currently passed through (line 508 of server.go: comment "CreateAgent409JSONResponse is a union alias (unmarshal required) — pass through"). After Phase 3 returns this from a real handler, the inject middleware can't easily set request_id inside the union. Phase 3 must EITHER (a) inject before the .FromXxx() call by stamping into the underlying ErrorResponse/VersionConflictErrorResponse first, then calling From, OR (b) accept that CreateAgent409 omits request_id. Strongly recommend (a) — see Pattern Hazard 4 below.

**Hazard:** Removing `Deps.StrictHandlers` breaks every test that uses `server.NewCompositeServer`. The isolation/main_test.go (line 181, 187) and scaffold/handler_test.go (line 155) both use the composite. Plan: delete `NewCompositeServer` along with `stubs.go` and update all callers in one wave.

---

### `services/api/internal/server/request_id_exhaustiveness_test.go` (EDIT — add 422 + new wrapper cases)

**Analog:** itself (the file already exists and tests every existing wrapper).

**Pattern from current file lines 77-103 (per-type assertion):**

```go
// Adapters
case api.ListAdapters400JSONResponse:
    require.NotNil(t, r.RequestId, "%s", typeName)
case api.CreateAdapter400JSONResponse:
    require.NotNil(t, r.RequestId, "%s", typeName)
// ... 121 cases total
```

**Phase 3 extension:** Add new cases for each `*422JSONResponse` introduced by the spec amendment. Build-gated — without the new cases, the test fails with "RequestId must be set after injection".

**Hazard:** The test asserts EVERY error response wrapper sets `RequestId`. If a new wrapper is generated by codegen but neither added to the server.go switch nor here, the build still passes but production silently drops request_id. Both edits must land together.

---

### `services/api/cmd/api/main.go` (EDIT — construct cache.Cache, catalog.Handlers)

**Current state to edit:** lines 102-143 (Redis + composite server construction).

**EDIT pattern — replace `NewCompositeServer` with `cache.New + catalog.New + server.Deps.Catalog`:**

```go
// BEFORE (Phase 2, lines 131-143):
strictServer := server.NewCompositeServer(orgDB, pool, rdb, specBytes)
mux := server.NewMux(&server.Deps{
    Pool:           pool,
    Redis:          rdb,
    OrgDB:          orgDB,
    Config:         cfg,
    StrictHandlers: strictServer,
    SpecBytes:      specBytes,
})

// AFTER (Phase 3):
cacheLayer := cache.New(rdb, slog.Default())
catalogHandlers := catalog.New(catalog.Deps{
    OrgDB:  orgDB,
    Cache:  cacheLayer,
    Logger: slog.Default(),
})
mux := server.NewMux(&server.Deps{
    Pool:      pool,
    Redis:     rdb,
    OrgDB:     orgDB,
    Config:    cfg,
    Catalog:   catalogHandlers,
    SpecBytes: specBytes,
})
```

**Hazard:** Phase 4 will add `state.Handlers` and embed `catalog.Handlers + state.Handlers` via the `ApiHandlers` composite struct (D-70). Phase 3 ships `catalog.Handlers` ALONE. Plan the seam: `server.Deps.Catalog *catalog.Handlers` is forward-compat — Phase 4 just changes the field type to `*ApiHandlers` and the embedding handles method dispatch.

---

### `services/api/go.mod` (EDIT — add singleflight + miniredis)

**Verified deps (RESEARCH.md §Standard Stack lines 144-152):**

```bash
cd services/api && go get golang.org/x/sync@v0.20.0 && \
  go get -t github.com/alicebob/miniredis/v2@latest && go mod tidy
```

**Hazard:** slopcheck flags miniredis as `[SLOP]` (false positive — see RESEARCH.md line 153). Planner should insert a one-line `checkpoint:human-verify` task before `go get` lands, per defense-in-depth policy.

---

### `Taskfile.yml` (OPTIONAL EDIT — add `task db:reset`)

**Per D-61 dev workflow:** `task db:reset` should drop the database, recreate it, then run `migrate up` from scratch — supports the editable-migration-002 pattern. Pattern based on existing `task migrate-up`. Optional in Phase 3 if not needed during plan execution.

---

## Shared Patterns

### Cross-Cutting Pattern 1: Org context extraction (every handler entry)

**Source:** `services/api/internal/scaffold/handler.go` lines 56-67.
**Apply to:** Every method on `catalog.Handlers` (all 41 StrictServerInterface methods).

```go
orgID, ok := orgkey.OrgIDFromContext(ctx)
if !ok {
    return api.{Op}500JSONResponse{
        InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error:  api.ErrorCodeInternal,
            Reason: "missing_org_id_in_context",
        },
    }, nil
}
```

**Hazard:** Always read org_id from CTX (never from `req.OrgId` URL param). FOUND-08 leakage guard depends on this — header is authoritative (D-20), URL is decoration (isolation_test.go Case 3).

---

### Cross-Cutting Pattern 2: Per-request sqlc Queries construction

**Source:** `services/api/internal/scaffold/handler.go` line 77.
**Apply to:** Every handler entry that performs DB I/O.

```go
q := generated.New(h.deps.OrgDB)
// ... q.InsertX(ctx, ...) / q.GetX(ctx, ...) / q.UpdateX(ctx, ...)
```

**Hazard (FOUND-04):** Always pass `*db.OrgDB` to `generated.New`. Never pass the raw `*pgxpool.Pool` — bypasses SQL validator and breaks isolation. The compile-time guard at orgdb.go:151 (`var _ generated.DBTX = (*OrgDB)(nil)`) ensures OrgDB satisfies the interface.

---

### Cross-Cutting Pattern 3: pgx.ErrNoRows → 404 mapping

**Source:** `services/api/internal/scaffold/handler.go` lines 145-153.
**Apply to:** Every GET-by-id handler.

```go
if err != nil {
    if errors.Is(err, pgx.ErrNoRows) {
        return api.GetX404JSONResponse{
            NotFoundJSONResponse: api.NotFoundJSONResponse{
                Error:  api.ErrorCodeNotFound,
                Reason: "{entity}_not_found",
            },
        }, nil
    }
    return api.GetX500JSONResponse{...}, nil
}
```

**Hazard (FOUND-08):** Cross-org probes return pgx.ErrNoRows (because sqlc query filters by `id AND org_id`) → 404. This is intentional — never return 200 or 403 (existence oracle).

---

### Cross-Cutting Pattern 4: Cache invalidation on write (D-55)

**Source:** None in-tree; locked by D-55 + D-56.
**Apply to:** Every write handler (Create — only if cache-hit conceivable for new ID, but usually skip; Update — always; Delete — always; 409 version conflict — always).

```go
// After successful DB commit:
if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "{entity}", id)); delErr != nil {
    h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "{entity}", id), "err", delErr)
    // do NOT return 5xx — DB write succeeded.
}
return api.UpdateX200JSONResponse(mapped), nil
```

**Hazard:** Cache failure NEVER turns a successful write into a 5xx (D-55). Microsecond race between commit and DEL is acceptable.

---

### Cross-Cutting Pattern 5: typed-response return (strict-server contract, D-44/D-69)

**Source:** Every scaffold handler return; locked across all 41 catalog endpoints.
**Apply to:** Every method on catalog.Handlers.

**Never:** call `middleware.WriteError(ctx, w, ...)` from inside a strict handler. `WriteError` is for chi middleware (OrgContext, UUIDv7PathParams) that short-circuit BEFORE strict-server runs.

**Always:** return a `<Op>{Status}JSONResponse` value satisfying the operation's `<Op>ResponseObject` interface. The strict pipeline marshals it correctly.

---

### Cross-Cutting Pattern 6: UUIDv7 minted at write boundary (D-19)

**Source:** `services/api/internal/scaffold/handler.go` line 78.
**Apply to:** Every CREATE handler.

```go
id := uuid.Must(uuid.NewV7()) // D-19 UUIDv7 everywhere
```

**Hazard:** Never accept client-supplied IDs for create paths in v0.1. Spec schemas omit `id` from `CreateXRequest` bodies.

---

## Pattern Hazards (compile-time gated)

### Hazard 1: `injectRequestIDIntoErrorResponse` exhaustiveness regression

**Source:** `services/api/internal/server/request_id_exhaustiveness_test.go` (entire file — 524 lines, 121 subtests).
**Triggered by:** Any new generated `*JSONResponse` type from spec amendment.

The exhaustiveness test individually asserts every error wrapper sets `RequestId`. After spec amendment D-75 adds 422 responses, codegen emits new wrappers (e.g., `CreateChannel422JSONResponse`). Both:
1. The type switch in `server.go:injectRequestIDIntoErrorResponse` (lines 205-610)
2. The test cases in `request_id_exhaustiveness_test.go`

MUST receive new cases. Build fails otherwise — that's the point.

**Mitigation:** After every codegen regen, run `go test ./internal/server -run TestRequestIDInjection_Exhaustiveness`. Failure points to a missing case.

---

### Hazard 2: `CreateAgent409JSONResponse` is a oneOf union, not a flat struct

**Source:** `services/api/internal/api/types.gen.go` lines 1065-1259 (union accessors).
**Triggered by:** A real CreateAgent handler returning 409 (currently 501 stub).

The 409 body is `oneOf: ErrorResponse | VersionConflictErrorResponse`, so oapi-codegen generates:

```go
// types.gen.go:1066:
type CreateAgent409JSONResponseBody struct {
    union json.RawMessage
}
// Builder methods:
func (t *CreateAgent409JSONResponseBody) FromErrorResponse(v ErrorResponse) error
func (t *CreateAgent409JSONResponseBody) FromVersionConflictErrorResponse(v VersionConflictErrorResponse) error
```

You CANNOT struct-init this type. The Phase 3 handler pattern:

```go
// Correct:
body := api.CreateAgent409JSONResponseBody{}
err := body.FromErrorResponse(api.ErrorResponse{
    Error:     "version_conflict",  // or invalid_reference for FK fail
    Reason:    "external_id_conflict",
    RequestId: nil, // injection middleware stamps here
})
if err != nil { return api.CreateAgent500JSONResponse{...}, nil }
return api.CreateAgent409JSONResponse(body), nil
```

**Mitigation:** Plan a unit test that round-trips a CreateAgent 409 through `json.Marshal/Unmarshal` and asserts wire shape `{"error":"...", "reason":"...", "request_id":"..."}`.

**Hazard within hazard:** The `injectRequestIDIntoErrorResponse` middleware currently passes `CreateAgent409JSONResponse` through (server.go line 508 — comment "union alias — pass through"). Phase 3 must EITHER (a) stamp request_id INTO the underlying ErrorResponse before calling FromErrorResponse (handler-side, easier), OR (b) update the middleware to unmarshal-modify-marshal (centralized but complex). Recommend (a): set `RequestId` inside the ErrorResponse BEFORE handing it to FromErrorResponse, OR accept that 409 unions omit request_id and document the gap.

---

### Hazard 3: orgDB SQL validator rejects unscoped queries (D-02)

**Source:** `services/api/internal/db/orgdb.go` lines 120-145 (preflight); `services/api/internal/db/sqlcheck.go` (10KB validator).
**Triggered by:** Any sqlc query missing `WHERE ... org_id = $N` or omitting org_id from INSERT column list.

The validator parses every SQL string with `pg_query_go` and rejects statements that don't reference org_id correctly. In `ValidationPanic` mode (dev/test), this panics — surfacing the bug at first run.

**Mitigation:** EVERY new query in `internal/db/queries/*.sql` must mention `org_id` in WHERE / INSERT / UPDATE / DELETE. The denormalized `agent_skills.org_id` column exists specifically to satisfy this for the join table. Test by running the catalog test suite with `db.ValidationPanic` — if any query is unscoped, the test panics with the SQL hash.

---

### Hazard 4: refresh-ahead goroutine ctx pitfall (Pitfall 4 in RESEARCH.md)

**Source:** RESEARCH.md Pitfall 4 lines 761-765 + Go 1.21 stdlib `context.WithoutCancel`.
**Triggered by:** Refresh-ahead goroutine using `context.Background()` instead of `context.WithoutCancel(ctx)`.

`context.Background()` strips all values from ctx — including org_id and trace span. When the refresh loader calls orgDB.Query, the preflight check fails (no org_id) → `ErrOrgIDMissingFromContext` → panic in ValidationPanic mode.

**Mitigation:** Lock the refresh goroutine ctx to `bgCtx := context.WithoutCancel(ctx)`. Add a unit test that:
1. Uses miniredis + Postgres testcontainer
2. Triggers a GET, observes cache hit
3. `s.FastForward(50 * time.Second)` to push PTTL below 10s
4. Triggers another GET, observes the refresh goroutine completes without panic
5. Asserts the cache entry was rewritten with fresh TTL

---

### Hazard 5: agent_skills replace must be ONE transaction

**Source:** RESEARCH.md Pitfall 5 lines 768-772.
**Triggered by:** UpdateAgent with non-nil Skills field, where DELETE succeeds but a follow-up INSERT fails.

Without a single tx wrapping `DeleteAgentSkills` + N×`InsertAgentSkill`, partial failure leaves the agent with a wrong (typically empty) skill set. The agent UPDATE itself must also be in the same tx so a version-mismatch later doesn't leave skills changed.

**Mitigation (OQ-5):** Add `(*OrgDB).BeginTx(ctx) (*OrgTx, error)` in Wave 1 to keep validator engaged for the tx. The OrgTx wraps `pgx.Tx` and forwards Exec/Query/QueryRow through SQLChecker. Without it, the handler must bypass orgDB for the tx duration — acceptable but loses FOUND-04 enforcement. Strongly recommend adding the wrapper.

---

### Hazard 6: spec `limit` param default mismatch (Pitfall 6 in RESEARCH.md)

**Source:** `openapi/openapi.yaml` line 1341-1351 (current spec: default 20, max 100); D-67 requires default 25.
**Triggered by:** Wave 0 spec amendment choice.

The recommended minimal edit is `default: 20 → 25` only. Do NOT rename `limit → page_size` — the rename has no functional benefit and breaks the not-yet-existing Phase 6 admin SPA's expected types.

**Mitigation:** Wave 0 plan should edit ONLY the default; leave the parameter name and max unchanged.

---

### Hazard 7: Channel + Adapter version field (Pitfall 8 in RESEARCH.md)

**Source:** RESEARCH.md Pitfall 8 lines 785-789.
**Triggered by:** REQ-CAT-08 "all update endpoints require version field" interpretation.

Channel and Adapter currently have NO `version` field in the OpenAPI spec. CAT-08 implies optimistic concurrency is universal but the spec was authored without version for these two.

**Mitigation:** Recommended path — amend spec to ADD `version` to Channel and Adapter (consistency). Alternative — document the v0.1 exception (Channel + Adapter use last-write-wins). Surface in plan-check. The amendment is non-breaking (clients ignore the new field).

---

### Hazard 8: JSONB column emits `[]byte` from sqlc (Pitfall 9 in RESEARCH.md)

**Source:** RESEARCH.md Pitfall 9 lines 791-795 + sqlc docs.
**Triggered by:** Adapter `config JSONB` column handling.

sqlc + pgx/v5 defaults JSONB → `[]byte`. The handler must marshal/unmarshal between `*map[string]interface{}` (api type) and `[]byte` (sqlc type) at the mapper layer.

**Mitigation:** Add `marshalAdapterConfig` + `unmarshalAdapterConfig` helpers in `mappers.go`. Test with nil/empty/nested map cases.

---

### Hazard 9: `Deps.OrgDBFactory` wording in CONTEXT.md (OQ-4)

**Source:** CONTEXT.md D-71 wording vs RESEARCH.md §Open Question 4 lines 1048-1052.
**Triggered by:** Literal interpretation of "OrgDBFactory db.Factory" in CONTEXT.md.

There is NO `db.Factory` type in the existing codebase. The existing pattern is the shared `*db.OrgDB` constructed once in main.go and handed to handlers (scaffold/handler.go line 40). Phase 3 should reuse this — the "factory" wording is unfortunate but the right semantics are "shared instance".

**Mitigation:** Use `Deps{OrgDB *db.OrgDB}` (not Factory). Plan-check should align on this.

---

## No Analog Found

Files with NO close in-tree match (planner should use RESEARCH.md patterns instead):

| File | Role | Data Flow | Reason | Reference |
|------|------|-----------|--------|-----------|
| `internal/cache/cache.go` | cache infra | request-response | First cache in repo; no prior pattern | RESEARCH.md §Pattern 3 lines 466-555 |
| `internal/catalog/cursor.go` | encode/decode | transform | First cursor pagination in repo | RESEARCH.md §Pattern 4 lines 583-636 |
| `internal/catalog/errors.go` | pgx-error mapper | transform | scaffold inlines this check; centralizing is new | type-switch shape mimics server.go:injectRequestIDIntoErrorResponse |
| `internal/catalog/agent_skills.go` | join handler | event-driven (tx-wrapped) | First N:M join handler | RESEARCH.md §Pattern 6 lines 664-696 |
| `(*OrgDB).BeginTx` (proposed extension) | db wrapper method | n/a | Phase 1 didn't ship a tx-wrapping method | RESEARCH.md §OQ-5 + Pitfall 5 |

## Metadata

**Analog search scope:**
- `services/api/internal/scaffold/` (entire — primary CRUD analog for all entity handlers)
- `services/api/internal/server/` (`server.go`, `stubs.go`, `request_id_exhaustiveness_test.go`)
- `services/api/internal/db/` (`orgdb.go`, `orgkey/orgkey.go`, `generated/scaffold.sql.go`, `generated/db.go`, `generated/models.go`)
- `services/api/internal/middleware/` (`httputil.go`, `uuidv7path.go`)
- `services/api/internal/api/` (`types.gen.go` partial — Agent/Skill/Queue/Channel/Adapter/BreakReason types + 409/422 wrappers; `server.gen.go` partial — Agent endpoint wrappers)
- `services/api/test/isolation/` (`isolation_test.go`, `main_test.go`)
- `services/api/cmd/api/main.go` (full)
- `services/api/internal/db/queries/scaffold.sql` (full — 17 lines)
- `migrations/000001_create_scaffold.up.sql` (full — 10 lines)
- `openapi/openapi.yaml` (ErrorCode + LimitQuery + VersionConflictErrorResponse + InvalidTransitionErrorResponse blocks)
- `services/api/sqlc.yaml` (full — 13 lines)

**Files scanned:** 25 source files + 1 sqlc config + 1 migration + 1 OpenAPI yaml = 28 files total

**Pattern extraction date:** 2026-05-16

## PATTERN MAPPING COMPLETE
