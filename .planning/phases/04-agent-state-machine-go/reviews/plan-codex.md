OpenAI Codex v0.130.0
--------
workdir: /Users/luong/workspace/dev/open-solutions
model: gpt-5.5
provider: openai
approval: never
sandbox: workspace-write [workdir, /tmp, $TMPDIR, /Users/luong/.codex/memories] (network access enabled)
reasoning effort: low
reasoning summaries: none
session id: 019e33d6-f349-7c52-b4a4-56a0c4276609
--------
user
Review the Phase 4 planning artifacts below for the Open Routing project.

CONTEXT:
- Phase 4 = Agent State Machine (Go) — implements STATE-01..STATE-10 of v0.1 Catalog Foundation milestone
- Stack: Go + chi + sqlc + pgx + PostgreSQL 17 + Redis (locked)
- Phase 3 (Catalog CRUD) is COMPLETE with 60 commits; Phase 4 builds on it.
- Plans cover 6 waves: contract+schema → sqlc+domain+skeleton → handlers+force+cache → TTL goroutine → composite wiring → isolation+acceptance+cross-AI review

KEY LOCKED DECISIONS (D-78..D-95 in 04-CONTEXT.md):
- agent_states table appended to migration 000002 (single editable migration per D-61)
- NO foreign key constraints (D-80 extends D-76 — project-wide invariant)
- TEXT + CHECK enum encoding (not Postgres ENUM)
- WrapUp TTL via per-agent time.AfterFunc (sub-ms) + startup sweep + 30s safety sweep
- System-initiated transitions schema-ready but firing deferred to v0.2/AUTH
- post_interaction_state defaults to prior status (set on Ready→Engaged in v0.2)
- force: bool flag on PATCH /status (bypasses transition matrix, keeps cross-row probes)
- state_version read-side only (no expected_state_version on PATCH)
- Cache reuses Phase 3 GetOrSet[T] with key or:{orgId}:agent_state:{id}, 60s TTL
- Jitter wrapup_until ±100ms when system sets it
- New state.Server type (NOT state.Handlers — Pitfall 1 avoids field-name collision in ApiHandlers composite)
- IsRoutable pure function in services/api/internal/domain/state.go
- agent_states row INSERTed atomically inside catalog.CreateAgent OrgTx (D-93 — Phase 5 must inherit)
- Spec amendment adds force: bool to PatchAgentStatusRequest (codegen drift cycle)

CRITICAL HAZARDS (Pitfalls flagged in research):
1. P1: state.Handlers + catalog.Handlers embed collision → use state.Server type
2. P3: force=true does NOT bypass cross-row probes (cross-org break_reason still → 422)
3. P6: Phase 5 bulk import must inherit D-93 (seed agent_states for every imported agent)
4. P8: TTL sweeper runs outside request — needs db.WithBypass + denormalized org_id
5. P10: qtx propagation — CreateAgent extension must use qtx, not pool

PROJECT RULES (HARD):
- No FK constraints anywhere
- WHY-not-WHAT comments (default no comments; only invariants/bugs/trade-offs)
- Cross-AI peer review at end of work unit

REVIEW FOCUS:
- Plan completeness: every STATE-* requirement + every ROADMAP success criterion has at least one task
- Pitfall capture: all 5 pitfalls reflected in plan body or task acceptance_criteria
- Wave dependency correctness (Wave 0 → 1 → {2,3 parallel} → 4 → 5)
- Concurrency/race risk in TTL design (graceful shutdown, timer.Stop semantics, registry mutex)
- Spec amendment safety (codegen drift gate, no path additions)
- Cross-org isolation completeness (all 4 D-94 probes named)
- Acceptance test mapping (all 5 ROADMAP success criteria have named tests)
- Cache invalidation correctness (DEL on write, sweeper-side cache.Del with org_id from RETURNING)
- Comment hygiene (any WHAT/HOW comments leaking through?)
- Dead code / scope creep (gsd-plan-checker flagged unused jitter() function in 04-04)
- Hidden assumptions or missing edges

OUTPUT FORMAT:
## Summary (2-3 sentences)
## Strengths (bullet list)
## Concerns
  ### [HIGH]: ...
  ### [MED]: ...
  ### [LOW]: ...
## Suggestions
## Verdict: READY | READY WITH FIXES | BLOCK

The planning artifacts (PATTERNS.md + 6 PLAN.md files) follow this marker:

================ PHASE 4 PLAN BUNDLE ================
# Phase 4: Agent State Machine (Go) - Pattern Map

**Mapped:** 2026-05-17
**Files analyzed:** 22 (12 NEW, 8 MODIFIED, 4 AUTO-GEN)
**Analogs found:** 18 / 22 (4 NEW have no analog — pure-new code; reference RESEARCH.md Code Examples)

## File Classification

| File | NEW/MODIFIED | Role | Data Flow | Closest Analog | Match Quality |
|------|--------------|------|-----------|----------------|---------------|
| `services/api/internal/state/handlers.go` | NEW | package scaffold (Deps + New + Start/Stop lifecycle) | request-response + bg-goroutine | `services/api/internal/catalog/handlers.go` | exact (D-71 hybrid constructor) |
| `services/api/internal/state/transitions.go` | NEW | state-machine domain (matrix + validator) | pure-transform | none (pure-new; RESEARCH §Pattern 1) | no-analog |
| `services/api/internal/state/ttl.go` | NEW | TTL goroutine (timer registry + sweeper) | event-driven + bg-poll | none (first clockwork integration; RESEARCH §Pattern 2) | no-analog |
| `services/api/internal/state/agent_states.go` | NEW | handler (GET + PATCH /status) | CRUD + cache + tx + probe | `services/api/internal/catalog/agents.go` + `services/api/internal/catalog/channels.go` | exact (D-66 + D-76 composite) |
| `services/api/internal/state/agent_states_test.go` | NEW | integration test (handler + cache + miniredis) | CRUD | `services/api/internal/catalog/agents_test.go` | exact |
| `services/api/internal/state/transitions_test.go` | NEW | unit test (exhaustive matrix walk) | pure-transform | `services/api/internal/catalog/cursor_test.go` | role-match (table-driven pure unit) |
| `services/api/internal/state/ttl_test.go` | NEW | unit test (clockwork.FakeClock) | event-driven | none (first FakeClock test; RESEARCH §Pattern 2) | no-analog |
| `services/api/internal/state/testutil_test.go` | NEW | shared test fixtures | scaffold | `services/api/internal/catalog/testutil_test.go` | exact (D-73) |
| `services/api/internal/state/main_test.go` | NEW | TestMain bring-up | scaffold | `services/api/internal/catalog/main_test.go` | exact |
| `services/api/internal/domain/state.go` | NEW | pure function (IsRoutable) | pure-transform | none (`domain/` is bootstrap; RESEARCH §Code Examples Domain) | no-analog |
| `services/api/internal/domain/state_test.go` | NEW | unit test (4-case table) | pure-transform | `services/api/internal/catalog/cursor_test.go` | role-match |
| `services/api/internal/db/queries/agent_states.sql` | NEW | sqlc query file | CRUD + bg-sweeper SQL | `services/api/internal/db/queries/agents.sql` + `queues.sql` (probe) | exact |
| `services/api/test/isolation/state_test.go` | NEW | cross-org isolation test | request-response | `services/api/test/isolation/catalog_test.go` | exact |
| `services/api/internal/catalog/agents.go` | MODIFIED | extend `CreateAgent` to seed agent_states row in same tx | CRUD + tx | `services/api/internal/catalog/agents.go` itself (Codex C4 tx pattern) | self-extension |
| `services/api/internal/catalog/notimpl.go` | MODIFIED | DELETE `GetAgentStatus` + `PatchAgentStatus` stubs | placeholder removal | `services/api/internal/catalog/notimpl.go` itself | self-deletion |
| `services/api/internal/db/sqlcheck.go` | MODIFIED | extend `tenantTables` allowlist | config | `services/api/internal/db/sqlcheck.go` itself | self-extension |
| `services/api/internal/db/queries/break_reasons.sql` | MODIFIED | APPEND `GetBreakReasonForState` probe | SQL probe | `services/api/internal/db/queries/queues.sql` `QueueExistsAndEnabledInOrg` | exact |
| `services/api/internal/server/server.go` | MODIFIED | extend `injectRequestIDIntoErrorResponse` switch if new types appear | type-switch | `services/api/internal/server/server.go` itself | self-extension |
| `services/api/cmd/api/main.go` | MODIFIED | wire `state.Server` + ApiHandlers + Start/Stop | wiring | `services/api/cmd/api/main.go` itself + RESEARCH §Pattern 7 | self-extension |
| `migrations/000002_catalog_v0_1.up.sql` | MODIFIED | APPEND `agent_states` table + indexes | migration | `migrations/000002_catalog_v0_1.up.sql` itself (existing CAT-01..07 layout) | self-extension |
| `migrations/000002_catalog_v0_1.down.sql` | MODIFIED | PREPEND `DROP TABLE IF EXISTS agent_states` | migration | `migrations/000002_catalog_v0_1.down.sql` itself | self-extension |
| `openapi/openapi.yaml` | MODIFIED | add `force: bool` to `PatchAgentStatusRequest.properties` | spec amendment | `openapi/openapi.yaml` itself (existing PatchAgentStatusRequest at lines 1075-1121) | self-extension |
| `services/api/internal/api/{server,types,spec}.gen.go` | AUTO-GEN | regenerated by `task gen` | codegen | DO NOT HAND-EDIT (Phase 2 D-47/D-48 CI gate) | n/a |
| `web/packages/ui/src/api/generated.ts` | AUTO-GEN | regenerated by `pnpm gen:api` | codegen | DO NOT HAND-EDIT | n/a |

## Pattern Assignments

---

### `services/api/internal/state/handlers.go` (package scaffold + lifecycle)

**Analog:** `services/api/internal/catalog/handlers.go` (lines 22-118)

**Why this analog:** D-71 hybrid constructor pattern is locked across both packages — required Deps fields + functional Options. Phase 4 D-88 explicitly mirrors. Lifecycle additions (Start/Stop for the TTL sweeper per D-95) extend the analog without breaking it.

**Imports pattern** (catalog/handlers.go lines 24-33):

```go
import (
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "log/slog"

    "github.com/luongdev/open-routing/services/api/internal/api"
    "github.com/luongdev/open-routing/services/api/internal/cache"
    "github.com/luongdev/open-routing/services/api/internal/db"
)
```

Phase 4 additions: `"context"`, `"sync"`, `"github.com/google/uuid"`, `"github.com/jonboulle/clockwork"` for TTL lifecycle + timer registry.

**Deps struct pattern** (catalog/handlers.go lines 36-61):

```go
type Deps struct {
    OrgDB  *db.OrgDB
    Pool   *pgxpool.Pool
    Cache  *cache.Cache
    Logger *slog.Logger
}
```

Phase 4 `state.Deps` MUST add `WrapUpDuration time.Duration` (defaults to `defaultWrapUpDur` if zero) per D-88 + Claude discretion #3. Omit `Pool` if not needed for state handlers (per RESEARCH §Pattern 6 — `OrgDB` + `Cache` + `Logger` only).

**Option + WithClock pattern** (catalog/handlers.go lines 64-73):

```go
type Option func(*Handlers)

func WithClock(c func() time.Time) Option {
    return func(h *Handlers) { h.clock = c }
}
```

Phase 4 swaps to clockwork: `func WithClock(c clockwork.Clock) Option` per RESEARCH §Pattern 6. Add `WithSweepInterval(d time.Duration) Option` for Wave 3 tests that need to compress the 30s safety sweep.

**Constructor pattern** (catalog/handlers.go lines 75-118):

```go
type Handlers struct {
    deps  Deps
    clock func() time.Time
}

var _ api.StrictServerInterface = (*Handlers)(nil)

func New(deps Deps, opts ...Option) *Handlers {
    h := &Handlers{deps: deps, clock: time.Now}
    for _, opt := range opts {
        opt(h)
    }
    return h
}
```

**Phase 4 deviations from analog:**

1. **Rename type per Pitfall 1.** `state.Handlers` collides with `catalog.Handlers` when both are anonymous-embedded into `ApiHandlers`. Rename to `state.Server` (or move via type alias). Compile-time `var _ api.StrictServerInterface = (*Server)(nil)` MUST be removed — the SUBSET (2 methods) does NOT satisfy the full StrictServerInterface; the assertion lives on `ApiHandlers` in `cmd/api/main.go` instead.
2. **Lifecycle methods.** Add `Start(ctx context.Context) error` + `Stop()` per D-95. Start runs the synchronous startup sweep before returning; Stop cancels the internal ctx, waits for the sweeper goroutine (sync.WaitGroup), and Stops every pending timer in the in-memory map. See RESEARCH §Pattern 6 for the full Start/Stop body (lines 608-651).
3. **Idempotency.** Both Start and Stop must use a `startMu sync.Mutex + started bool` so repeated calls no-op safely (defensive — `defer stateServer.Stop()` may run after Start failure).

---

### `services/api/internal/state/transitions.go` (matrix + validator)

**Analog:** none (pure-new code).

**Reference:** RESEARCH.md §Pattern 1 (lines 275-352) — verbatim shape for the matrix + `validateTransition` function. The matrix encodes 7 agent-initiated edges (NotReady→Ready; Ready→{NotReady, Break}; Break→{Ready, NotReady}; WrapUp→{Ready, NotReady}). System-only edges (Ready→Engaged, Engaged→WrapUp, *→Offline, Offline→NotReady) deferred per D-82.

**Hazards:**

1. **Use `api.AgentStatus` as the type, not a parallel local enum.** D-91 says "matrix encoded as `map[AgentStatus]map[AgentStatus]TransitionRule`"; AgentStatus = api.AgentStatus type alias keeps the wire encoding the single source of truth.
2. **`ErrInvalidTransition` is a sentinel.** Wrap with `fmt.Errorf("%w: from=%s to=%s", ErrInvalidTransition, from, to)` so the handler can `errors.Is(err, ErrInvalidTransition)` and translate to `PatchAgentStatus409JSONResponse(InvalidTransitionErrorResponse{...})`.
3. **`force=true` does NOT go through this validator** (D-84). The handler routes around `validateTransition` when `force` is true, but still runs the break_reason probe (Pitfall 3 in RESEARCH).

---

### `services/api/internal/state/ttl.go` (timer registry + sweeper)

**Analog:** none (first clockwork integration in repo).

**Reference:** RESEARCH.md §Pattern 2 (lines 353-438) — verbatim shape for the timer registry, `scheduleWrapUpExpiry`, and `expireWrapUp`.

**Hazards from RESEARCH Pitfalls:**

1. **Pitfall 2 (Sweeper goroutine outlives Stop()).** Inside the AfterFunc body, first `select { case <-h.ctx.Done(): return; default: }` before the DB call. clockwork's mocked Timer respects the same semantic so tests can assert clean shutdown via `go test -race`.
2. **Pitfall 8 (TTL goroutine SQLChecker panic).** The sweeper-issued `ExpireWrapUp(agent_id, org_id)` query must mention `org_id` in the predicate (it does in the agent_states.sql template — see below). The ctx that flows into `expireWrapUp` from `time.AfterFunc` does NOT carry an org_id, so the sweeper path uses `OrgDB.WithBypass(ctx, "wrapup_sweeper")` per Phase 1 D-04. WithBypass emits a structured slog audit event automatically.
3. **Pitfall 9 (FakeClock not advanced).** Test pattern is `fakeClock.Advance(d)` followed by `require.Eventually(t, ..., 2*time.Second, 50*time.Millisecond, ...)` — clockwork does NOT tick automatically.

**Bypass pattern** (`services/api/internal/db/bypass.go` lines 28-33):

```go
func WithBypass(ctx context.Context, reason string) context.Context {
    return context.WithValue(ctx, bypassCtxKey{}, &bypassMarker{
        Reason: reason,
        Caller: callerInfo(2),
    })
}
```

Phase 4 sweeper call:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
ctx = db.WithBypass(ctx, "wrapup_sweeper")
rows, err := h.db.ExpireWrapUp(ctx, agentID, orgID)
```

Note: even though we WithBypass, the SQL itself includes `WHERE org_id = $2` so the query is auditable + plan-cacheable.

---

### `services/api/internal/state/agent_states.go` (GET + PATCH handlers)

**Analogs (composite):**
- **GET pattern:** `services/api/internal/catalog/agents.go` `GetAgent` (lines 175-234) — cache.GetOrSet + loader + ErrNotFound mapping.
- **PATCH pattern:** `services/api/internal/catalog/agents.go` `UpdateAgent` (lines 354-493) for D-66 disambiguate; `services/api/internal/catalog/channels.go` `CreateChannel` (lines 38-110) for D-76 cross-row probe.
- **Tx pattern (commit + cache.Del + 0-row disambiguate):** agents.go `UpdateAgent` lines 384-440.

**Imports** (agents.go lines 41-56):

```go
import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"
    openapi_types "github.com/oapi-codegen/runtime/types"

    "github.com/luongdev/open-routing/services/api/internal/api"
    "github.com/luongdev/open-routing/services/api/internal/cache"
    "github.com/luongdev/open-routing/services/api/internal/db/generated"
    "github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)
```

Phase 4 adds: `"github.com/luongdev/open-routing/services/api/internal/db"` (for `db.WithBypass` in TTL paths only; the request-handlers below do NOT need it).

**Org-context extraction pattern** (every handler, agents.go lines 72-77):

```go
orgID, ok := orgkey.OrgIDFromContext(ctx)
if !ok {
    return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
        Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
    }}, nil
}
```

Apply to both `GetAgentStatus` and `PatchAgentStatus` (Cross-Cutting Pattern 1 below).

**GET + cache.GetOrSet pattern** (agents.go lines 179-234):

```go
func (h *Handlers) GetAgent(ctx context.Context, req api.GetAgentRequestObject) (api.GetAgentResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        return api.GetAgent500JSONResponse{...}, nil
    }
    agentID := uuid.UUID(req.Id)
    key := cache.Key(orgID, "agents", agentID)

    agent, err := cache.GetOrSet[api.Agent](ctx, h.deps.Cache, key, cacheTTL,
        func(ctx context.Context) (api.Agent, error) {
            q := generated.New(h.deps.OrgDB)
            row, ferr := q.GetAgent(ctx, generated.GetAgentParams{
                ID:    pgUUID(agentID),
                OrgID: pgUUID(orgID),
            })
            if errors.Is(ferr, pgx.ErrNoRows) {
                return api.Agent{}, cache.ErrNotFound
            }
            if ferr != nil {
                return api.Agent{}, ferr
            }
            // ... map row to DTO
            return mapAgent(row, skills), nil
        })

    switch {
    case errors.Is(err, cache.ErrNotFound):
        return api.GetAgent404JSONResponse{...}, nil
    case err != nil:
        h.deps.Logger.ErrorContext(ctx, "get agent", "err", err)
        return api.GetAgent500JSONResponse{...}, nil
    }
    return api.GetAgent200JSONResponse(agent), nil
}
```

**Phase 4 deviation:** cache key is `cache.Key(orgID, "agent_state", agentID)` per D-86 (singular, NOT `"agent_states"` plural — Pitfall 7). Define a package-local helper `func (h *Server) cacheKey(orgID, agentID uuid.UUID) string { return cache.Key(orgID, "agent_state", agentID) }` to route all cache calls through one place — prevents drift.

**PATCH with D-66 atomic UPDATE + 0-row disambiguate** (agents.go lines 405-440):

```go
row, err := qtx.UpdateAgent(ctx, generated.UpdateAgentParams{
    ID:              pgUUID(agentID),
    OrgID:           pgUUID(orgID),
    ExpectedVersion: int32(req.Body.Version),
    Name:            req.Body.Name,
    Email:           emailStr,
    Enabled:         req.Body.Enabled,
})
if errors.Is(err, pgx.ErrNoRows) {
    // 0 rows — disambiguate 404 vs 409 inside the same tx.
    cur, perr := qtx.GetAgentByIdAnyVersion(ctx, generated.GetAgentByIdAnyVersionParams{
        ID:    pgUUID(agentID),
        OrgID: pgUUID(orgID),
    })
    if errors.Is(perr, pgx.ErrNoRows) {
        return api.UpdateAgent404JSONResponse{...}, nil
    }
    if perr != nil {
        h.deps.Logger.ErrorContext(ctx, "update agent disambiguate", "err", perr)
        return api.UpdateAgent500JSONResponse{...}, nil
    }
    // 409 — defer Rollback runs (no UPDATE landed). D-56 cache DEL.
    if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "agents", agentID)); delErr != nil {
        h.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", ...)
    }
    return api.UpdateAgent409JSONResponse{...}, nil
}
```

**Phase 4 PATCH deviation per D-85 + RESEARCH §Pattern 3:**

- WHERE clause uses `status = expected_from` (the matrix-driven gate) NOT `version = expected_version` — 0 rows ⇒ either 404 (row absent) OR 409 invalid_transition with the OBSERVED current `status` from the disambiguate SELECT.
- The 409 body type is `PatchAgentStatus409JSONResponse(InvalidTransitionErrorResponse{...})` per OpenAPI spec — NOT the catalog VersionConflict variant.
- `force=true` switches the handler to `qtx.ForceUpdateAgentStateStatus(...)` which omits the `status = expected_from` predicate (D-84 + sqlc query template below).

**D-76 cross-row probe pattern** (channels.go lines 55-73, applied to break_reason_id):

```go
if req.Body.DefaultQueueId != nil {
    queueID := uuid.UUID(*req.Body.DefaultQueueId)
    _, qErr := q.QueueExistsAndEnabledInOrg(ctx, generated.QueueExistsAndEnabledInOrgParams{
        ID:    pgUUID(queueID),
        OrgID: pgUUID(orgID),
    })
    if errors.Is(qErr, pgx.ErrNoRows) {
        return api.CreateChannel422JSONResponse(api.ErrorResponse{
            Error:  api.ErrorCodeInvalidReference,
            Reason: "default_queue_id_not_found_or_disabled",
        }), nil
    }
    if qErr != nil {
        h.deps.Logger.ErrorContext(ctx, "queue exists probe", "err", qErr)
        return api.CreateChannel500JSONResponse{...}, nil
    }
}
```

**Phase 4 invariant per D-84 (Pitfall 3 in RESEARCH):** the break_reason probe runs REGARDLESS of `force=true`. Order:

1. If `to == Break` AND `break_reason_id != nil`, run `q.GetBreakReasonForState(ctx, breakReasonID, orgID)`; pgx.ErrNoRows → `PatchAgentStatus422JSONResponse(ErrorResponse{Error: ErrorCodeInvalidReference, Reason: "break_reason_id_not_found_or_disabled"})`.
2. If `force == false`: call `validateTransition(from, to)` → 409 on failure.
3. Begin tx (BeginTx pattern below); apply UpdateAgentStateStatus OR ForceUpdateAgentStateStatus.
4. 0 rows → GetAgentState disambiguate → 404 (no row) OR 409 invalid_transition with observed.
5. Commit; `cache.Del` after commit (D-55).
6. If `force == true`: emit WARN slog `state.force.applied` per D-84.

**Tx + commit + cache.Del pattern** (agents.go lines 466-476):

```go
if err := tx.Commit(ctx); err != nil {
    h.deps.Logger.ErrorContext(ctx, "update agent commit", "err", err)
    return api.UpdateAgent500JSONResponse{...}, nil
}

if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "agents", agentID)); delErr != nil {
    h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "agents", agentID), "err", delErr)
}
```

**force=true WARN slog** (per D-84 — new pattern):

```go
if req.Body.Force != nil && *req.Body.Force {
    h.deps.Logger.WarnContext(ctx, "state.force.applied",
        "agent_id", agentID,
        "from", observedFrom,
        "to", req.Body.To,
        "requester_org", orgID,
    )
}
```

---

### `services/api/internal/state/agent_states_test.go` (handler tests)

**Analog:** `services/api/internal/catalog/agents_test.go` (lines 1-130 header + helpers shown; 716 total lines for full template).

**Helpers + paths pattern** (agents_test.go lines 77-108):

```go
func agentPath(orgID uuid.UUID) string {
    return "/v1/orgs/" + orgID.String() + "/agents"
}

func agentDetailPath(orgID, agentID uuid.UUID) string {
    return "/v1/orgs/" + orgID.String() + "/agents/" + agentID.String()
}

func postAgent(t testing.TB, th *TestHandlers, body api.CreateAgentRequest) api.Agent {
    t.Helper()
    resp, raw := httpPOST(t, th.HTTP, th.OrgID, agentPath(th.OrgID), body)
    require.Equalf(t, http.StatusCreated, resp.StatusCode,
        "postAgent: want 201, got %d body=%s", resp.StatusCode, string(raw))
    var a api.Agent
    require.NoError(t, json.Unmarshal(raw, &a), "postAgent: unmarshal Agent")
    return a
}
```

Phase 4 additions:
- `func agentStatusPath(orgID, agentID uuid.UUID) string { return "/v1/orgs/" + orgID.String() + "/agents/" + agentID.String() + "/status" }`
- `func patchStatus(t testing.TB, th *TestHandlers, agentID uuid.UUID, body api.PatchAgentStatusRequest) (*http.Response, []byte)` helper.

**Coverage matrix to cover** (from RESEARCH Phase Requirements → Test Map):
- STATE-02 allowed transition happy path
- STATE-03 invalid transition → 409 with from/to fields
- STATE-04 Ready→Break + valid break_reason_id → 200; missing → 422
- STATE-06 PATCH sets post_interaction_state when status==Engaged (test seeds Engaged directly via testutil_test.go raw INSERT helper)
- STATE-08 state_version monotonically increases on every PATCH
- force=true bypasses matrix (test invalid edge with force=true returns 200)
- force=true still 422 on cross-org break_reason (covered in isolation/state_test.go)
- D-66 PATCH on non-existent agent_id → 404
- Cache invalidation: PATCH, then GET returns fresh row (Pitfall 7 — assertion: `th.Miniredis.Exists(cache.Key(th.OrgID, "agent_state", agentID)) == false` immediately after PATCH)

---

### `services/api/internal/state/transitions_test.go` (exhaustive matrix walk)

**Analog:** `services/api/internal/catalog/cursor_test.go` (table-driven pure unit, 73 lines).

**Pattern** (cursor_test.go lines 22-35):

```go
func TestCursorRoundTrip(t *testing.T) {
    createdAt := time.Date(2026, 5, 16, 9, 30, 45, 123456789, time.UTC)
    id := uuid.Must(uuid.NewV7())

    s, err := EncodeCursor(createdAt, id)
    require.NoError(t, err)
    require.NotEmpty(t, s)
    // ...
}
```

**Phase 4 test cases:**

1. **Exhaustive matrix walk:** Cartesian product of 6 statuses × 6 statuses = 36 pairs. Assert each pair matches the matrix presence — generates from `matrix` directly so an undocumented matrix edit fails the test.
2. **STATE-05 schema-ready:** assert `matrix[Ready][Engaged]` is NOT in matrix (system-only).
3. **STATE-09 deferred:** assert `matrix[*][Offline]` and `matrix[Offline][NotReady]` are NOT in matrix.
4. **`validateTransition` returns expected `TransitionRule`** for each allowed edge (e.g., `Ready→Break` returns `{RequiresBreakReasonID: true, AgentInitiated: true}`).
5. **`validateTransition` returns wrapped `ErrInvalidTransition` for unsupported edges** — assert `errors.Is(err, ErrInvalidTransition)`.

---

### `services/api/internal/state/ttl_test.go` (clockwork FakeClock)

**Analog:** none (first FakeClock test in repo).

**Reference:** RESEARCH.md §Pattern 2 (lines 353-438) + Pitfall 9 (lines 855-859).

**Pattern (verbatim from RESEARCH Pitfall 9):**

```go
func TestWrapUpTTL_FiresOnExpiry(t *testing.T) {
    fakeClock := clockwork.NewFakeClock()
    h := state.New(state.Deps{...}, state.WithClock(fakeClock))
    // seed a WrapUp row via testutil's raw INSERT
    seedWrapUpAgent(t, th, agentID, fakeClock.Now().Add(60*time.Second), "ready")
    require.NoError(t, h.Start(ctx))
    defer h.Stop()

    h.scheduleWrapUpExpiry(agentID, fakeClock.Now().Add(60*time.Second))
    fakeClock.Advance(61 * time.Second)

    require.Eventually(t, func() bool {
        row := getAgentStateRow(t, th.Pool, agentID)
        return row.Status == "Ready" && row.WrapupUntil == nil && row.StateVersion == 2
    }, 2*time.Second, 50*time.Millisecond, "WrapUp expiry never fired")
}
```

**Test cases:**
- `TestWrapUpTTL_FiresOnExpiry` — happy path; clock advance + idempotent UPDATE lands.
- `TestWrapUpTTL_ConcurrentFire_Idempotent` — call `scheduleWrapUpExpiry` twice + run safety sweep concurrently; assert `state_version == 2` exactly once (only one writer wins).
- `TestWrapUpTTL_StartupSweepFiresOverdue` — seed a WrapUp row with `wrapup_until = NOW() - 10s`, call `h.Start(ctx)`, assert row transitions to post_interaction_state synchronously before Start returns.
- `TestWrapUpTTL_StopCancelsPendingTimers` — start, schedule, Stop. Advance time past expiry. Assert UPDATE did NOT land (Pitfall 2 — select ctx.Done() before DB call).

---

### `services/api/internal/state/testutil_test.go` (D-73 shared fixtures)

**Analog:** `services/api/internal/catalog/testutil_test.go` (272 lines — verbatim template).

**Fixture struct + constructor pattern** (testutil_test.go lines 67-158):

```go
type TestHandlers struct {
    H         *Handlers
    Miniredis *miniredis.Miniredis
    Pool      *pgxpool.Pool
    OrgID     uuid.UUID
    HTTP      *httptest.Server
    rdb       *redis.Client
}

func newTestHandlers(t testing.TB) *TestHandlers {
    t.Helper()
    if sharedPool == nil {
        t.Skip("state: sharedPool nil — Docker testcontainer unavailable")
        return nil
    }
    mr := miniredis.RunT(t)
    rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
    t.Cleanup(func() { _ = rdb.Close() })
    logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
    c := cache.New(rdb, logger)
    orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)
    // ... build state.Server via state.New(state.Deps{...})
    // ... build composite ApiHandlers with both catalog.Handlers (or stubs) + state.Server
    // ... server.NewMux with composite as StrictHandlers
    srv := httptest.NewServer(mux)
    t.Cleanup(srv.Close)
    orgID := uuid.Must(uuid.NewV7())
    return &TestHandlers{H: h, Miniredis: mr, Pool: sharedPool, OrgID: orgID, HTTP: srv, rdb: rdb}
}
```

**Phase 4 deviations:**

1. **Wire `ApiHandlers` composite, not standalone `state.Server`.** The httptest server hits real chi routes; the routes invoke whichever handler implements `GetAgentStatus`/`PatchAgentStatus`. Use `&main.ApiHandlers{Handlers: catalogHandlers, Server: stateServer}` (or whatever final composite shape; see Wave 4 + Pitfall 1).
2. **Add `seedAgentStateRow(t, pool, params)` helper** (per Claude discretion #6: raw INSERT preferred for isolation). Seeds an `agent_states` row directly via `generated.New(pool).InsertAgentState(...)` so tests for Engaged/WrapUp states can bypass the transition matrix.
3. **TRUNCATE table addition:** `cleanCatalogTables` already TRUNCATEs the 7 catalog tables CASCADE (line 174). Phase 4 needs `agent_states` added. Plan: extract the truncate list into a constant or add a new `cleanStateTables(t, ctx)` helper that runs first since `agent_states` may be appended.

**HTTP helpers** (testutil_test.go lines 191-209 — reused verbatim):

```go
func httpPOST(t testing.TB, srv *httptest.Server, orgID uuid.UUID, path string, body any) (*http.Response, []byte) { ... }
func httpGET(...) { ... }
func httpPATCH(...) { ... }
func httpDELETE(...) { ... }
```

Add `func httpPATCHStatus(t testing.TB, srv *httptest.Server, orgID, agentID uuid.UUID, body any) (*http.Response, []byte)` as a thin convenience wrapper.

---

### `services/api/internal/state/main_test.go` (TestMain bring-up)

**Analog:** `services/api/internal/catalog/main_test.go` (4.8K, 1 file).

Read briefly via `Bash(wc -l)`; verbatim re-use is the recommendation. The `TestMain` provisions the testcontainer postgres, runs migrations, opens pgxpool → assigns to `sharedPool` package-level var. `m.Run()` then proceeds.

**Phase 4 deviation:** the import path changes (`services/api/internal/state/main_test.go` instead of `.../catalog/main_test.go`); the body is identical (same migrations path `../../../../migrations`, same testcontainer postgres:17).

---

### `services/api/internal/domain/state.go` (IsRoutable pure function)

**Analog:** none (bootstraps `domain/` package).

**Reference:** RESEARCH.md §Code Examples Domain (lines 1001-1031) — verbatim.

```go
package domain

import "github.com/luongdev/open-routing/services/api/internal/api"

type AgentStateInputs struct {
    Status              string
    BreakReasonRoutable bool
}

func IsRoutable(in AgentStateInputs) bool {
    if in.Status == string(api.AgentStatusReady) {
        return true
    }
    if in.Status == string(api.AgentStatusBreak) && in.BreakReasonRoutable {
        return true
    }
    return false
}
```

**Invariants:**
- NO database access. Pure transformation. Tests do not touch a DB.
- Caller (v0.2 routing engine) loads `break_reason.routable` separately (cache hit or DB read).
- Status compared as string against `api.AgentStatus*` constants — avoids type coupling on the typed alias.

---

### `services/api/internal/domain/state_test.go` (4-case table)

**Analog:** `services/api/internal/catalog/cursor_test.go` (table-driven pure unit).

**Reference:** RESEARCH.md §Code Examples Domain Tests (lines 1033-1048).

```go
func TestIsRoutable(t *testing.T) {
    cases := []struct {
        name string
        in   AgentStateInputs
        want bool
    }{
        {"Ready→true", AgentStateInputs{Status: "Ready"}, true},
        {"Break+routable=true→true", AgentStateInputs{Status: "Break", BreakReasonRoutable: true}, true},
        {"Break+routable=false→false", AgentStateInputs{Status: "Break", BreakReasonRoutable: false}, false},
        {"NotReady/Engaged/WrapUp/Offline→false", AgentStateInputs{Status: "Engaged"}, false},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            require.Equal(t, tc.want, IsRoutable(tc.in))
        })
    }
}
```

---

### `services/api/internal/db/queries/agent_states.sql` (NEW sqlc queries)

**Analogs (composite):**
- **`InsertAgentState`** ← `agents.sql` `InsertAgent` (lines 24-27): basic :one INSERT.
- **`GetAgentState`** ← `agents.sql` `GetAgent` (lines 29-34): :one by (id, org_id).
- **`UpdateAgentStateStatus`** ← `agents.sql` `UpdateAgent` (lines 78-95): COALESCE sparse-PATCH + version increment + RETURNING. Deviation: WHERE includes `status = expected_from` NOT `version = expected_version` per D-85.
- **`ListWrapUpsToExpire` + `ExpireWrapUp`** ← `queues.sql` `QueueExistsAndEnabledInOrg` (lines 70-80): :many + :execrows with top-level SELECT FROM tenant table for SQLChecker.

**InsertAgent template** (agents.sql lines 24-27):

```sql
-- name: InsertAgent :one
INSERT INTO agents (id, org_id, external_id, name, email, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, external_id, name, email, enabled, version, created_at, updated_at;
```

Phase 4 `InsertAgentState`:

```sql
-- name: InsertAgentState :one
INSERT INTO agent_states (agent_id, org_id, status, state_version)
VALUES ($1, $2, $3, 1)
RETURNING agent_id, org_id, status, engaged_channel, break_reason_id,
         post_interaction_state, wrapup_until, state_version, updated_at;
```

**UpdateAgent COALESCE template** (agents.sql lines 78-95):

```sql
-- name: UpdateAgent :one
UPDATE agents
SET name       = COALESCE(sqlc.narg('name')::text,    name),
    email      = COALESCE(sqlc.narg('email')::text,   email),
    enabled    = COALESCE(sqlc.narg('enabled')::bool, enabled),
    version    = version + 1,
    updated_at = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, external_id, name, email, enabled, version, created_at, updated_at;
```

Phase 4 `UpdateAgentStateStatus` per RESEARCH §Pattern 3 (lines 444-466) — copy verbatim with these changes:
- COALESCE on status/engaged_channel/break_reason_id/post_interaction_state/wrapup_until.
- `state_version = state_version + 1` (NOT `version + 1`).
- WHERE clause uses `status = sqlc.arg('expected_from')` (NOT `version = expected_version`).
- Add a sibling `ForceUpdateAgentStateStatus` that OMITS the `status = expected_from` predicate for D-84.

**QueueExistsAndEnabledInOrg template** (queues.sql lines 70-80):

```sql
-- name: QueueExistsAndEnabledInOrg :one
-- D-76 FK probe used by CreateChannel / UpdateChannel.
-- Structured as a top-level SELECT FROM queues so the orgDB
-- SQLChecker sees a tenant alias.
SELECT 1::int AS exists_in_org
FROM queues
WHERE id = $1 AND org_id = $2 AND enabled = TRUE
LIMIT 1;
```

Phase 4 sibling query for break_reasons (added to `break_reasons.sql`, see MODIFIED below).

**Sweeper SQL** (per RESEARCH §Code Examples lines 974-998):

```sql
-- name: ListWrapUpsToExpire :many
-- NB: this query is called from the sweeper goroutine which uses
-- OrgDB.WithBypass — the org_id mention here satisfies SQLChecker.
SELECT agent_id, org_id, wrapup_until, post_interaction_state
FROM agent_states
WHERE status = 'WrapUp'
ORDER BY org_id, wrapup_until;

-- name: ExpireWrapUp :execrows
UPDATE agent_states
SET status        = COALESCE(post_interaction_state, 'NotReady'),
    wrapup_until  = NULL,
    state_version = state_version + 1,
    updated_at    = NOW()
WHERE agent_id = $1
  AND org_id   = $2
  AND status   = 'WrapUp'
  AND wrapup_until < NOW();
```

**Pitfall 8 reminder:** `ListWrapUpsToExpire` does NOT include org_id in WHERE (queries ALL orgs' WrapUp rows from the sweeper). This is intentional — the sweeper is per-process; `OrgDB.WithBypass(ctx, "wrapup_sweeper")` is the audit trail. SQLChecker still accepts the query because it references the `org_id` column in SELECT. If SQLChecker rejects, the wave-0 task must include adding `agent_states` to `tenantTables` AND ensure the query's SELECT mentions org_id verbatim.

---

### `services/api/test/isolation/state_test.go` (NEW cross-org tests)

**Analog:** `services/api/test/isolation/catalog_test.go` (329 lines) — verbatim template for the 4 D-94 probes.

**Helpers from existing catalog_test.go** (re-use; do NOT duplicate):
- `postEntity(t, urlBase, entityPath, orgID, body)` (lines 35-57)
- `getEntityStatus(t, urlBase, entityPath, orgID, id)` (lines 62-73) — caller passes "agents/<uuid>/status" as `entityPath`.
- `freshOrg(t)` (from main_test.go or isolation_test.go — generates UUIDv7).
- `baseURL()` (sharedSrv.URL).
- `requireContainer(t)` (skips if Docker unavailable).

**Phase 4 test cases (D-94 (a)(b)(c)(d) — RESEARCH lines 1117-1135):**

```go
// (a) GET /status across orgs returns 404.
func TestState_GetStatusCrossOrg(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    orgA, orgB := freshOrg(t), freshOrg(t)
    code, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
        "external_id": "s-iso-a", "name": "A", "email": "a@example.com",
    })
    require.Equal(t, http.StatusCreated, code)
    require.Equal(t, http.StatusOK,
        getEntityStatus(t, baseURL(), "agents/"+agentA.String()+"/status", orgA, uuid.Nil))
    require.Equal(t, http.StatusNotFound,
        getEntityStatus(t, baseURL(), "agents/"+agentA.String()+"/status", orgB, uuid.Nil))
}
```

**Note:** RESEARCH §Cross-org isolation tests flagged that the existing `getEntityStatus` helper takes (urlBase, entityPath, orgID, id). For `/agents/{id}/status` you can stuff the path into entityPath as `"agents/<uuid>/status"` and pass `uuid.Nil` as the id — the helper concatenates `path+id` if id != uuid.Nil. **Better:** add a new helper `getStatusEndpoint(t, urlBase, orgID, agentID)` that builds `/v1/orgs/{orgID}/agents/{agentID}/status` cleanly. The new helper goes into `state_test.go` (NOT into `catalog_test.go`).

**(c) is the critical Pitfall 3 test:**

```go
// (c) PATCH /status with force=true does NOT bypass cross-org break_reason_id check.
func TestState_ForceDoesNotBypassCrossOrgBreakReason(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    orgA, orgB := freshOrg(t), freshOrg(t)

    // Seed: agent in A, break_reason in B (cross-org).
    _, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{...})
    _, brB := postEntity(t, baseURL(), "break-reasons", orgB, map[string]any{
        "name": "OtherOrgBreak", "routable": false, "display_order": 0,
    })

    // PATCH agentA's status to Break with brB + force=true.
    // Expected: 422 invalid_reference (force bypasses matrix, NOT probes).
    code := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
        "to": "Break", "break_reason_id": brB.String(), "force": true,
    })
    require.Equal(t, http.StatusUnprocessableEntity, code,
        "force=true MUST NOT bypass cross-org break_reason probe (D-84 + Pitfall 3)")
}
```

---

### `services/api/internal/catalog/agents.go` (MODIFIED — D-93)

**Analog:** itself — `CreateAgent` (lines 71-173) and its existing Codex C4 tx pattern at lines 103-156.

**Modification site:** between `qtx.InsertAgent` (line 121) returning successfully and the existing skills replace block (line 144). Insert a new step:

```go
// Existing — DO NOT REMOVE:
row, err := qtx.InsertAgent(ctx, generated.InsertAgentParams{...})
if err != nil { /* existing 409/422/500 branch */ }

// === PHASE 4 ADDITION (D-93): seed agent_states row inside same tx ===
if _, sErr := qtx.InsertAgentState(ctx, generated.InsertAgentStateParams{
    AgentID:      pgUUID(id),
    OrgID:        pgUUID(orgID),
    Status:       string(api.AgentStatusOffline),
    StateVersion: 1,
}); sErr != nil {
    h.deps.Logger.ErrorContext(ctx, "create agent state row", "err", sErr)
    return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
        Error: api.ErrorCodeInternal, Reason: "agent_state_insert_failed",
    }}, nil
}
// === END PHASE 4 ADDITION ===

// Existing — skills replace inside the SAME tx (Codex C4 — atomicity):
if req.Body.Skills != nil && len(*req.Body.Skills) > 0 { ... }
```

**Pitfall 10 (RESEARCH lines 861-865):** the InsertAgentState call MUST use `qtx` (the tx-bound Queries built at line 113), NOT `generated.New(h.deps.OrgDB)`. Reviewer (Codex) flags any `generated.New(h.deps.OrgDB)` inside a `tx, _ := h.deps.OrgDB.BeginTx(ctx)` block. The existing skills replace (`h.replaceAgentSkills(ctx, qtx, ...)` at line 145) is the reference for "qtx flows through helpers."

**Cross-cutting reminder for Phase 5 (capture in PATTERNS.md tail):** Phase 5 bulk-import (`CSV/JSON UPSERT INTO agents`) MUST insert the agent_states row for every imported agent. If Phase 5 uses `ON CONFLICT (org_id, external_id) DO UPDATE`, the matching INSERT INTO agent_states must use `ON CONFLICT (agent_id) DO NOTHING` so re-imports do not regress the state machine.

---

### `services/api/internal/catalog/notimpl.go` (MODIFIED — delete state stubs)

**Analog:** itself (notimpl.go lines 40-50 — the two state-machine stubs).

**Delete these two methods:**

```go
// DELETE — Phase 4 state.Server owns this method now (Pitfall 1: rename to Server).
func (h *Handlers) GetAgentStatus(_ context.Context, _ api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error) {
    return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// DELETE — Phase 4 state.Server owns this method now.
func (h *Handlers) PatchAgentStatus(_ context.Context, _ api.PatchAgentStatusRequestObject) (api.PatchAgentStatusResponseObject, error) {
    return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}
```

**Keep:** `notImplementedBody()` helper, `BulkImportCatalog`, `GetImportJob` (Phase 5 owns those).

**Compile-time consequence:** `var _ api.StrictServerInterface = (*Handlers)(nil)` at `catalog/handlers.go` line 92 will FAIL after deletion — `catalog.Handlers` no longer satisfies the full interface. This is EXPECTED in Phase 4; the assertion must MOVE to `cmd/api/main.go` on `*ApiHandlers` instead (see main.go modification below). Wave 4 sequencing critical: do this rename + assertion move in the same commit.

---

### `services/api/internal/db/sqlcheck.go` (MODIFIED — tenantTables)

**Analog:** itself — `tenantTables` map at lines 32-41.

**Insert one line into the map (alphabetical order helps grep):**

```go
var tenantTables = map[string]struct{}{
    "_scaffold":     {}, // legacy Phase 1
    "agents":        {}, // CAT-01
    "agent_skills":  {}, // CAT-03 (junction; denormalized org_id per D-72)
    "agent_states":  {}, // STATE-01 (denormalized org_id per D-78)  ← PHASE 4 ADDITION
    "skills":        {}, // CAT-02
    "queues":        {}, // CAT-04
    "channels":      {}, // CAT-05
    "adapters":      {}, // CAT-06
    "break_reasons": {}, // CAT-07
}
```

**Pitfall 4 (RESEARCH lines 825-829):** without this addition, the FIRST sqlc-generated query against `agent_states` panics in dev/test with `ErrSQLMissingOrgFilter`. Wave 0 task description MUST list this as a step adjacent to the migration append.

---

### `services/api/internal/db/queries/break_reasons.sql` (MODIFIED — probe)

**Analog:** `services/api/internal/db/queries/queues.sql` `QueueExistsAndEnabledInOrg` (lines 70-80) — same shape, different table.

**Append at the end of break_reasons.sql** (after `SoftDeleteBreakReason` at line 68):

```sql
-- name: GetBreakReasonForState :one
-- Phase 4 D-76 cross-row probe — used by PatchAgentStatus (STATE-04) and
-- IsRoutable callers (STATE-10). Returns the routable flag when the
-- reason exists + is enabled in the caller's org; pgx.ErrNoRows otherwise.
-- Same-org enforcement is the FOUND-08 invariant (cross-org reason ids
-- return ErrNoRows → handler maps to 422 invalid_reference).
SELECT routable
FROM break_reasons
WHERE id = $1 AND org_id = $2 AND enabled = TRUE
LIMIT 1;
```

**Why combined probe (vs two separate queries) per RESEARCH Open Question 2:** one query covers BOTH the STATE-04 422 path (caller checks ErrNoRows) AND the STATE-10 IsRoutable path (caller reads routable scalar). Keeps the sqlc surface minimal — same trade-off the queues probe makes.

---

### `services/api/internal/server/server.go` (MODIFIED — type switch)

**Analog:** itself — `injectRequestIDIntoErrorResponse` (lines 210-580 ish).

**Existing PatchAgentStatus422 + GetAgentStatus cases already present** (lines 234, 333, 336, 482, 485, 488):

```go
case api.PatchAgentStatus422JSONResponse:
    v := api.ErrorResponse(r)
    v.RequestId = setIfNil(v.RequestId, id)
    return api.PatchAgentStatus422JSONResponse(v)
case api.GetAgentStatus500JSONResponse: ...
case api.PatchAgentStatus500JSONResponse: ...
case api.GetAgentStatus400JSONResponse: ...
case api.GetAgentStatus404JSONResponse: ...
case api.PatchAgentStatus400JSONResponse: ...
```

**Pitfall 5 (RESEARCH lines 831-835):** after `task gen` runs in Wave 0, the planner MUST diff the new spec.gen.go enum against `injectRequestIDIntoErrorResponse` and add any NEW case BEFORE running `TestRequestIDInjection_Exhaustiveness`. Expected: A7 of Assumptions Log says NO new response types are introduced by D-92 (force is a request-body field only) — verified by reading openapi.yaml. If the assumption breaks (e.g., a future spec amendment adds a 409 invalid_transition response with its own wrapper type), the test will fail on the (122 + N)-th subtest with a clear "missing case" message.

**A 409 case may need adding** if the existing `PatchAgentStatus409JSONResponse(InvalidTransitionErrorResponse{...})` wrapper is not already covered. Verify by inspecting server.gen.go for the type name and grep server.go for it. If missing, add this case:

```go
case api.PatchAgentStatus409JSONResponse:
    // InvalidTransitionErrorResponse has its own RequestId field.
    r.RequestId = setIfNil(r.RequestId, id)
    return r
```

---

### `services/api/cmd/api/main.go` (MODIFIED — composite + lifecycle)

**Analog:** itself (lines 1-196) — wires `cache.New + catalog.New` at lines 140-146; passes `catalogHandlers` as `StrictHandlers` at line 154. Phase 4 EXTENDS step (8) and (9).

**Existing pattern** (main.go lines 140-156):

```go
catalogCache := cache.New(rdb, slog.Default())
catalogHandlers := catalog.New(catalog.Deps{
    OrgDB:  orgDB,
    Pool:   pool,
    Cache:  catalogCache,
    Logger: slog.Default(),
})

mux := server.NewMux(&server.Deps{
    Pool:           pool,
    Redis:          rdb,
    OrgDB:          orgDB,
    Config:         cfg,
    StrictHandlers: catalogHandlers,
    SpecBytes:      specBytes,
})
```

**Phase 4 wiring (reference: RESEARCH §Pattern 7, lines 714-726):**

```go
catalogCache := cache.New(rdb, slog.Default())
catalogHandlers := catalog.New(catalog.Deps{
    OrgDB:  orgDB,
    Pool:   pool,
    Cache:  catalogCache,
    Logger: slog.Default(),
})

// === PHASE 4 ADDITION ===
stateServer := state.New(state.Deps{
    OrgDB:  orgDB,
    Cache:  catalogCache, // SAME cache instance — single Redis client
    Logger: slog.Default(),
})
if err := stateServer.Start(ctx); err != nil {
    slog.ErrorContext(ctx, "state server start", "err", err)
    return 1
}
defer stateServer.Stop()

// Compile-time guarantee: the composite satisfies the FULL interface.
type ApiHandlers struct {
    *catalog.Handlers
    *state.Server  // renamed from Handlers per Pitfall 1 to avoid embed name collision
}
var _ api.StrictServerInterface = (*ApiHandlers)(nil)

apiHandlers := &ApiHandlers{
    Handlers: catalogHandlers,
    Server:   stateServer,
}
// === END PHASE 4 ADDITION ===

mux := server.NewMux(&server.Deps{
    Pool:           pool,
    Redis:          rdb,
    OrgDB:          orgDB,
    Config:         cfg,
    StrictHandlers: apiHandlers, // CHANGED from catalogHandlers
    SpecBytes:      specBytes,
})
```

**Wiring sequence considerations:**

- `stateServer.Start(ctx)` must run BEFORE the http.Server begins serving — startup sweep is synchronous (per D-95 + Pattern 6 line 619). Place it before step (10) otelhttp wrap.
- `defer stateServer.Stop()` placement: between `defer pool.Close()` (line 103) and `defer func() { _ = rdb.Close() }()` (line 113). Order at shutdown: http.Server.Shutdown (line 192) → stateServer.Stop (deferred) → rdb.Close → pool.Close → telemetry shutdown. Stop iterates the timer map cancelling each AfterFunc, then waits for in-flight sweeper UPDATEs (sync.WaitGroup). 10s shutdownCtx should be enough.
- **Move `var _ api.StrictServerInterface` assertion** from `catalog/handlers.go` line 92 to `cmd/api/main.go` (after composite declaration). Catalog alone NO LONGER satisfies the interface after Phase 4 deletes notimpl.go state stubs.

**Pitfall 1 reminder:** `*catalog.Handlers` + `*state.Handlers` BOTH expose field name `Handlers` when anonymous-embedded → compile error "duplicate field name." Rename `state.Handlers` → `state.Server` (recommended; less Phase 3 churn). Alternative #2 (manual forwarding 41 methods) is verbose. Alternative #3 (alias wrapper) is indirection-heavy.

---

### `migrations/000002_catalog_v0_1.up.sql` (MODIFIED — append agent_states)

**Analog:** itself — existing CAT-01..CAT-07 table layout (e.g., agents table at lines 29-45).

**Existing pattern (agents table example, lines 29-45):**

```sql
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
```

**Append point:** INSIDE the existing `BEGIN; ... COMMIT;` block, AFTER the last `agent_skills` table + indexes (around current line 280), BEFORE `COMMIT;`. Per RESEARCH §Code Examples Migration append (lines 869-907) — verbatim:

```sql
-- ---------------------------------------------------------------------------
-- STATE-01..STATE-10: agent_states (Phase 4 — D-78)
-- ---------------------------------------------------------------------------
-- One row per agent. PK = agent_id. NO FK (D-80 — app-layer probes only).
-- TEXT + CHECK encoding for status/engaged_channel/post_interaction_state
-- (D-79) — Adding values = ALTER constraint, not ALTER TYPE.
-- Denormalized org_id so SQLChecker (D-02) sees the column on every query.
CREATE TABLE agent_states (
    agent_id                UUID PRIMARY KEY,
    org_id                  UUID NOT NULL,
    status                  TEXT NOT NULL
                              CHECK (status IN ('Ready','NotReady','Break','Engaged','WrapUp','Offline')),
    engaged_channel         TEXT
                              CHECK (engaged_channel IS NULL OR engaged_channel IN ('voice','chat','email')),
    break_reason_id         UUID,
    post_interaction_state  TEXT
                              CHECK (post_interaction_state IS NULL OR post_interaction_state IN ('ready','not_ready')),
    wrapup_until            TIMESTAMPTZ,
    state_version           BIGINT NOT NULL DEFAULT 1,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ix_agent_states_org_status ON agent_states (org_id, status);

CREATE INDEX ix_agent_states_wrapup_until
    ON agent_states (wrapup_until)
    WHERE status = 'WrapUp';
```

**Deviations from catalog table pattern:**

- **No `id` column** — PK is `agent_id` (one row per agent, no surrogate).
- **No `external_id` / `name`** — state table is purely operational, not addressable.
- **No `enabled` column** — agent_states does not soft-delete; deleting an agent in v0.1 is `enabled = FALSE` on `agents`, the state row persists.
- **No `version` column** — replaced by `state_version BIGINT` (BIGINT not INTEGER per RESEARCH §Schema — state machine fires more often than catalog updates).
- **No `created_at` column** — only `updated_at` (the row is created at agent CREATE time; no separate creation timestamp).
- **No UNIQUE constraint** — PK on agent_id already enforces uniqueness.
- **NO foreign keys** — D-80 invariant.

---

### `migrations/000002_catalog_v0_1.down.sql` (MODIFIED — prepend drop)

**Analog:** itself — existing `BEGIN; DROP TABLE IF EXISTS agent_skills; ... DROP TABLE IF EXISTS agents; ...` pattern (lines 11-19).

**Prepend INSIDE `BEGIN; ... COMMIT;` block, BEFORE the first `DROP TABLE IF EXISTS agent_skills;` (line 13):**

```sql
-- Phase 4 — agent_states (D-78) reverse.
DROP TABLE IF EXISTS agent_states;
```

**Why first:** consistent with the up migration's order (agent_states is APPENDED last in up.sql, so it must be DROPPED first to avoid hypothetical FK dependency issues — though D-80 means there are none). Documentation-only ordering, matches the existing `agent_skills first` convention.

---

### `openapi/openapi.yaml` (MODIFIED — force field)

**Analog:** itself — existing `PatchAgentStatusRequest` at lines 1075-1121 (already verified via Read).

**Insert AFTER `post_interaction_state` block (around line 1121), BEFORE the next sibling schema:**

```yaml
        # ── PHASE 4 ADDITION (D-92, D-84) ──────────────────────────────
        force:
          type: boolean
          default: false
          nullable: true
          description: >
            Admin override. When `true`, bypasses the transition matrix
            (skips HTTP 409 `invalid_transition`). Cross-row probes still
            run: a missing/cross-org `break_reason_id` still returns 422.
            v0.1 stub auth allows any caller to pass `force=true`; v1 AUTH
            phase restricts to `org_admin` role. Use sparingly — every
            force=true call emits a WARN-level audit log entry.
```

**Codegen consequences (per RESEARCH lines 1095-1099):**

- `services/api/internal/api/types.gen.go` — `PatchAgentStatusJSONRequestBody.Force *bool` field added.
- `services/api/internal/api/server.gen.go` — request object body shape carries the new field (no signature change).
- `services/api/internal/api/spec.gen.go` — embedded YAML bytes refresh.
- `web/packages/ui/src/api/generated.ts` — TS body gains `force?: boolean | null`.

**A7 of Assumptions Log:** NO new response types introduced; only request-body field. Pitfall 5 mitigated by reading the type-switch exhaustiveness diff in Wave 0 after `task gen`.

---

## Shared Patterns

### Cross-Cutting Pattern 1: Org context extraction (every handler entry)

**Source:** `services/api/internal/catalog/agents.go` lines 72-77, repeated in every handler.

**Apply to:** `GetAgentStatus`, `PatchAgentStatus` in `state/agent_states.go`.

```go
orgID, ok := orgkey.OrgIDFromContext(ctx)
if !ok {
    return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
        Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
    }}, nil
}
```

This contract holds because `OrgContext` middleware sets `org_id` in ctx for every `/v1/orgs/{org_id}/...` route. Failure here means the middleware was bypassed — always 500, never 4xx.

### Cross-Cutting Pattern 2: cache.Key namespace (D-58 / D-86)

**Source:** D-58 + D-86 + Pitfall 7 (RESEARCH lines 843-847).

**Apply to:** every cache.GetOrSet / cache.Del call in state/agent_states.go.

**Key format:** `or:{orgId}:agent_state:{agent_id}` (SINGULAR `agent_state`, NOT plural `agent_states`).

**Helper to prevent drift:**

```go
func (h *Server) cacheKey(orgID, agentID uuid.UUID) string {
    return cache.Key(orgID, "agent_state", agentID)
}
```

Route ALL cache calls through this helper; never inline `cache.Key(..., "agent_state", ...)` at call sites.

### Cross-Cutting Pattern 3: D-55 cache.Del after commit (warn on failure, never 5xx)

**Source:** `services/api/internal/catalog/agents.go` lines 167-170, 473-476, 524-527.

**Apply to:** every PATCH success path + the 409 disambiguate path (D-56) in `state/agent_states.go`.

```go
if err := tx.Commit(ctx); err != nil {
    h.deps.Logger.ErrorContext(ctx, "patch state commit", "err", err)
    return api.PatchAgentStatus500JSONResponse{...}, nil
}
if delErr := h.deps.Cache.Del(ctx, h.cacheKey(orgID, agentID)); delErr != nil {
    h.deps.Logger.WarnContext(ctx, "cache del failed", "key", h.cacheKey(orgID, agentID), "err", delErr)
}
```

A cache.Del failure NEVER turns a successful DB write into a 5xx (RESEARCH Anti-Patterns line 777).

### Cross-Cutting Pattern 4: BeginTx + defer Rollback for atomicity (Codex C4)

**Source:** `services/api/internal/catalog/agents.go` lines 104-111.

**Apply to:** `PatchAgentStatus` (UPDATE + 0-row disambiguate + cache.Del); `CreateAgent` extension (D-93 INSERT agent_states inside existing tx).

```go
tx, err := h.deps.OrgDB.BeginTx(ctx)
if err != nil {
    h.deps.Logger.ErrorContext(ctx, "begin tx", "err", err)
    return api.PatchAgentStatus500JSONResponse{...}, nil
}
defer func() { _ = tx.Rollback(ctx) }()  // safe after commit — pgx ignores.
qtx := generated.New(tx)
// ... qtx.UpdateAgentStateStatus / qtx.GetAgentState ...
if err := tx.Commit(ctx); err != nil { /* 500 */ }
```

Pitfall 10: INSIDE the tx block, use `qtx` for ALL Queries calls; never reach for `generated.New(h.deps.OrgDB)` — that bypasses the tx and breaks atomicity.

### Cross-Cutting Pattern 5: D-66 atomic UPDATE + 0-row disambiguate (matrix variant)

**Source:** `services/api/internal/catalog/agents.go` lines 405-440.

**Apply to:** `PatchAgentStatus` in `state/agent_states.go`.

**Phase 4 deviation per D-85:**
- Catalog: `WHERE id = $1 AND org_id = $2 AND version = $3` → 0 rows = 404 OR 409 version_conflict.
- Phase 4: `WHERE agent_id = $1 AND org_id = $2 AND status = $3` (status = expected_from) → 0 rows = 404 OR 409 invalid_transition.

The disambiguate SELECT (`GetAgentState`) runs in the SAME tx so it observes a consistent snapshot. If the row exists, build `InvalidTransitionErrorResponse{Error: ErrorCodeInvalidTransition, From: row.Status, To: req.Body.To}` and return 409.

### Cross-Cutting Pattern 6: D-76 cross-row probe BEFORE write

**Source:** `services/api/internal/catalog/channels.go` lines 55-73 (and again at 280-298 for UpdateChannel).

**Apply to:** `PatchAgentStatus` for `break_reason_id` (STATE-04).

```go
if req.Body.To == api.AgentStatusBreak {
    if req.Body.BreakReasonId == nil {
        return api.PatchAgentStatus422JSONResponse(api.ErrorResponse{
            Error:  api.ErrorCodeInvalidValue,
            Reason: "break_reason_id_required_for_break",
        }), nil
    }
    breakReasonID := uuid.UUID(*req.Body.BreakReasonId)
    _, qErr := generated.New(h.deps.OrgDB).GetBreakReasonForState(ctx, generated.GetBreakReasonForStateParams{
        ID:    pgUUID(breakReasonID),
        OrgID: pgUUID(orgID),
    })
    if errors.Is(qErr, pgx.ErrNoRows) {
        return api.PatchAgentStatus422JSONResponse(api.ErrorResponse{
            Error:  api.ErrorCodeInvalidReference,
            Reason: "break_reason_id_not_found_or_disabled",
        }), nil
    }
    if qErr != nil { /* 500 */ }
}
```

**Pitfall 3 (D-84):** This probe runs whether `force == true` or `force == false`. Only the transition matrix is bypassed.

### Cross-Cutting Pattern 7: UUIDv7 minted at write boundary (D-19)

**Source:** `services/api/internal/catalog/agents.go` line 100 (`uuid.Must(uuid.NewV7())`).

**Apply to:** Not directly applicable in Phase 4 — `agent_states.agent_id` is the EXISTING agent id, not a fresh UUID. New rows inserted via `qtx.InsertAgentState` in `CreateAgent` use the agent's id (line 116). No fresh minting needed.

### Cross-Cutting Pattern 8: WHY-not-WHAT comments (CLAUDE.md)

**Source:** `/Users/luong/workspace/dev/open-solutions/CLAUDE.md` lines 28-46.

**Apply to:** EVERY new comment in EVERY Phase 4 file.

Existing examples in catalog code that match the discipline:
- `// Codex C4 iter 3 — single tx for agent INSERT + optional skills replace.` (agents.go line 103 — explains the invariant, not the action).
- `// D-76 cross-row FK probe. Missing/disabled/cross-org queue all surface as pgx.ErrNoRows here (FOUND-08 — no oracle for which).` (channels.go line 53-54).

Phase 4 examples to emulate:
- `// Pitfall 1: rename to Server to avoid *catalog.Handlers + *state.Handlers field-name collision when both embed into ApiHandlers.` (state/handlers.go).
- `// D-84: force=true bypasses the transition matrix but NOT cross-row probes (Pitfall 3). break_reason probe runs first.` (state/agent_states.go).
- `// D-86 cache key uses singular "agent_state" — distinct namespace from the catalog "agents" key. Pitfall 7: cross-namespace cache invalidations are silent bugs.` (state/agent_states.go cacheKey helper).

NEVER write:
- `// Loop through items and add them to the total`
- `// Return 404 if not found`
- `// Set the user's name field`

## Pattern Hazards (compile-time gated)

### Hazard 1: `*catalog.Handlers` + `*state.Handlers` field-name collision (Pitfall 1)

**What goes wrong:** `type ApiHandlers struct { *catalog.Handlers; *state.Handlers }` fails to compile with "duplicate field name Handlers."

**How to avoid:** Rename `state.Handlers` → `state.Server`. Apply in `state/handlers.go`, `state/agent_states.go` (receiver `*Server`), all `_test.go` files (mention as `state.Server`), and `cmd/api/main.go` wiring.

**Reviewer check (Codex/Gemini):** confirm no occurrence of `*state.Handlers` survives Phase 4 commits; only `*state.Server`.

### Hazard 2: `var _ api.StrictServerInterface = (*Handlers)(nil)` migration

**What goes wrong:** After `catalog/notimpl.go` loses the 2 state stubs, `catalog/handlers.go` line 92 compile-time assertion FAILS — catalog alone no longer satisfies the interface.

**How to avoid:** In the SAME commit that deletes the notimpl stubs, remove line 92 of `catalog/handlers.go` AND add `var _ api.StrictServerInterface = (*ApiHandlers)(nil)` in `cmd/api/main.go` near the composite struct declaration.

**Test gate:** `go build ./...` must pass after the commit. If you split across commits, `go build` breaks at the intermediate state.

### Hazard 3: `tenantTables` panic on first agent_states query (Pitfall 4)

**What goes wrong:** First sqlc-generated query against `agent_states` panics in dev/test with `ErrSQLMissingOrgFilter`.

**How to avoid:** Wave 0 adds `"agent_states": {}` to `tenantTables` map BEFORE Wave 1 generates the sqlc queries. Validate by running `go test -short ./internal/state/...` after both edits.

### Hazard 4: `force=true` bypasses break_reason probe (Pitfall 3 — security)

**What goes wrong:** Engineer reads "force bypasses validation" and skips ALL validation including cross-org break_reason probe. Org admin can poke Break state with another org's break_reason_id → cross-org leak.

**How to avoid:** Explicit isolation test `TestState_ForceDoesNotBypassCrossOrgBreakReason` asserts force=true + cross-org break_reason_id → 422. Cross-AI review re-validates the handler reads `force` AFTER the probe runs.

### Hazard 5: TTL goroutine ctx strips org_id (Pitfall 8)

**What goes wrong:** Sweeper-triggered `expireWrapUp` calls SQL without org_id in ctx. SQLChecker in ValidationPanic mode crashes the process.

**How to avoid:** Sweeper-side calls wrap ctx with `db.WithBypass(ctx, "wrapup_sweeper")` per Phase 1 D-04. The structured slog audit event is emitted automatically by orgdb.preflight on bypass.

### Hazard 6: clockwork FakeClock not advanced (Pitfall 9)

**What goes wrong:** Test uses `clockwork.NewFakeClock()`; production handler calls `clock.AfterFunc(d, f)`. Test waits forever because `f` only fires when `fakeClock.Advance(d)` is called.

**How to avoid:** Test pattern is `fakeClock.Advance(61 * time.Second)` followed by `require.Eventually(t, predicate, 2*time.Second, 50*time.Millisecond, ...)`. clockwork's FakeClock does NOT auto-tick.

### Hazard 7: Phase 5 bulk import skips agent_states INSERT (Pitfall 6)

**What goes wrong:** Phase 5 bulk-import upserts `agents` but skips `agent_states` INSERT. Subsequent GET /agents/{id}/status returns 404 even though the agent exists.

**How to avoid:** PATTERNS.md captures this for Phase 5. Phase 5's plan MUST include a sibling `INSERT INTO agent_states (...) ON CONFLICT (agent_id) DO NOTHING` for every imported agent (preserves state on re-imports, seeds Offline status on first import).

### Hazard 8: Initial state INSERT uses raw orgDB not qtx (Pitfall 10)

**What goes wrong:** In `CreateAgent` extension, engineer adds `qtx2 := generated.New(h.deps.OrgDB)` for the new InsertAgentState call → breaks atomicity (state row commits independently of agent row).

**How to avoid:** Reviewer (Codex) flags any `generated.New(h.deps.OrgDB)` inside a `tx, _ := h.deps.OrgDB.BeginTx(ctx)` block. Use the EXISTING `qtx` built at line 113 of agents.go.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `services/api/internal/state/transitions.go` | state-machine domain | pure-transform | Pure new code; small static data + validator. Use RESEARCH §Pattern 1 verbatim. |
| `services/api/internal/state/ttl.go` | TTL goroutine | event-driven + bg-poll | First clockwork integration; first per-key timer registry. Use RESEARCH §Pattern 2 verbatim. |
| `services/api/internal/state/ttl_test.go` | unit test | event-driven | First FakeClock test. Use RESEARCH §Pitfall 9 pattern. |
| `services/api/internal/domain/state.go` | pure function | pure-transform | Bootstraps `domain/` package. Use RESEARCH §Code Examples Domain verbatim. |

For all four, the planner should reference RESEARCH.md sections directly. Do NOT extrapolate from catalog patterns — these files introduce new semantics (state machine, time-mocking, pure-function package).

## Metadata

**Analog search scope:**
- `services/api/internal/catalog/` (full directory — 24 files)
- `services/api/internal/db/queries/` (7 sqlc files)
- `services/api/internal/db/` (orgdb.go, sqlcheck.go, bypass.go)
- `services/api/internal/server/server.go` (type switch)
- `services/api/test/isolation/` (catalog_test.go + main_test.go)
- `services/api/cmd/api/main.go`
- `services/api/internal/api/` (server.gen.go grep only — no body read)
- `migrations/000002_catalog_v0_1.up.sql` + `.down.sql`
- `openapi/openapi.yaml` (lines 980-1121, 1753-1850)

**Files scanned:** 22 source files read; 11 grep probes (line counts, headers).

**Pattern extraction date:** 2026-05-17

**Cross-AI peer review at the end of Phase 4:** Codex + Gemini per project CLAUDE.md. PATTERNS.md is a planning document — review fires at wave-end commits, not on this file.
---
phase: 04-agent-state-machine-go
plan: 01
type: execute
wave: 0
depends_on: []
files_modified:
  - openapi/openapi.yaml
  - services/api/internal/api/server.gen.go
  - services/api/internal/api/types.gen.go
  - services/api/internal/api/spec.gen.go
  - web/packages/ui/src/api/generated.ts
  - migrations/000002_catalog_v0_1.up.sql
  - migrations/000002_catalog_v0_1.down.sql
  - services/api/internal/db/sqlcheck.go
  - services/api/go.mod
  - services/api/go.sum
autonomous: false
requirements:
  - STATE-01
  - STATE-05
  - STATE-06
  - STATE-07
  - STATE-08

must_haves:
  truths:
    - "openapi/openapi.yaml carries `force: boolean` field on PatchAgentStatusRequest"
    - "Regenerated `services/api/internal/api/*.gen.go` and `web/packages/ui/src/api/generated.ts` are committed (codegen-drift CI green)"
    - "PostgreSQL `agent_states` table exists in migration 000002 with columns + indexes per D-78"
    - "`agent_states` is listed in `tenantTables` so SQLChecker accepts queries against it"
    - "`github.com/jonboulle/clockwork v0.4.0` is in go.mod after legitimacy verification"
    - "`task db:reset && task test` is green after migration append"
  artifacts:
    - path: "openapi/openapi.yaml"
      provides: "Spec amendment adding `force` boolean to PatchAgentStatusRequest"
      contains: "force:"
    - path: "migrations/000002_catalog_v0_1.up.sql"
      provides: "agent_states CREATE TABLE + 2 indexes appended before COMMIT"
      contains: "CREATE TABLE agent_states"
    - path: "migrations/000002_catalog_v0_1.down.sql"
      provides: "DROP TABLE IF EXISTS agent_states prepended inside BEGIN block"
      contains: "DROP TABLE IF EXISTS agent_states"
    - path: "services/api/internal/db/sqlcheck.go"
      provides: "tenantTables allowlist entry for agent_states"
      contains: "\"agent_states\":"
    - path: "services/api/go.mod"
      provides: "clockwork dependency declared"
      contains: "github.com/jonboulle/clockwork"
  key_links:
    - from: "openapi/openapi.yaml"
      to: "services/api/internal/api/types.gen.go"
      via: "task gen pipeline (oapi-codegen)"
      pattern: "Force\\s+\\*bool"
    - from: "openapi/openapi.yaml"
      to: "web/packages/ui/src/api/generated.ts"
      via: "pnpm gen:api pipeline (openapi-typescript)"
      pattern: "force\\??:\\s+boolean"
    - from: "migrations/000002_catalog_v0_1.up.sql"
      to: "services/api/internal/db/sqlcheck.go"
      via: "tenantTables entry — every future agent_states query reaches SQLChecker through this allowlist"
      pattern: "agent_states.*tenantTables|tenantTables.*agent_states"
---

<objective>
Establish the Wave 0 contract for Phase 4: amend the OpenAPI spec to add `force: boolean` on PatchAgentStatusRequest (D-92), append the `agent_states` table + indexes to migration 000002 (D-78), register `agent_states` in the SQLChecker `tenantTables` allowlist (Pitfall 4), regenerate Go + TS codegen artifacts (D-47/D-48 codegen-drift CI gate), and install the `github.com/jonboulle/clockwork v0.4.0` injectable-clock library after package legitimacy verification.

Purpose: Wave 0 is the foundation. Every subsequent wave depends on (a) regenerated request types carrying `Force *bool`, (b) the migrated table existing so sqlc can run against a real schema, and (c) SQLChecker accepting the new table. Without all four artifacts in place, Wave 1 sqlc generation panics in dev with `ErrSQLMissingOrgFilter` (Pitfall 4 — Hazard 3 in PATTERNS.md).

Output: Spec amendment + regenerated artifacts committed; migration applies cleanly via `task db:reset`; clockwork in go.mod; codegen-drift CI green (`task gen && git diff --exit-code` empty).
</objective>

<execution_context>
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/workflows/execute-plan.md
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@./CLAUDE.md
@.planning/phases/04-agent-state-machine-go/04-CONTEXT.md
@.planning/phases/04-agent-state-machine-go/04-RESEARCH.md
@.planning/phases/04-agent-state-machine-go/04-PATTERNS.md
@.planning/phases/04-agent-state-machine-go/04-VALIDATION.md
@.planning/phases/03-catalog-crud-go/03-CONTEXT.md
@.planning/phases/02-openapi-contract-codegen/02-CONTEXT.md
@.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md
@openapi/openapi.yaml
@migrations/000002_catalog_v0_1.up.sql
@migrations/000002_catalog_v0_1.down.sql
@services/api/internal/db/sqlcheck.go
@services/api/Taskfile.yml
@services/api/go.mod
</context>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| client → API (PATCH /status with `force=true`) | Spec field is admin-trusted in v0.1 (stub auth). RBAC deferred to AUTH phase. |
| Public registry → repository (`go get clockwork`) | New module dependency. Must be verified before install. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-04-01 | Tampering | `force` request field | accept | v0.1 stub auth lets any caller pass it; force=true triggers slog WARN in Wave 2 (D-84). RBAC restriction deferred to AUTH phase (documented in CONTEXT.md). |
| T-04-02 | Information Disclosure | Cross-org break_reason via force=true | mitigate | Force does NOT bypass cross-row probes (Pitfall 3). Wave 2 implements probe-before-matrix; Wave 5 tests `TestState_ForceDoesNotBypassCrossOrgProbe_422`. |
| T-04-03 | Tampering | SQLChecker bypass via missing tenantTables entry | mitigate | This wave adds `"agent_states": {}` to `tenantTables` (D-78 denormalized org_id). |
| T-04-SC | Tampering | `go get github.com/jonboulle/clockwork` | mitigate | Task 7 below is `checkpoint:human-verify gate=blocking-human` — verifies via `go mod why` + pkg.go.dev README. ASSUMED status per RESEARCH §Package Legitimacy Audit until checkpoint clears. |
</threat_model>

<tasks>

<task type="auto">
  <name>Task 1: Amend openapi/openapi.yaml with `force` field on PatchAgentStatusRequest</name>
  <files>openapi/openapi.yaml</files>
  <read_first>
    - openapi/openapi.yaml (lines 1075-1122 — existing PatchAgentStatusRequest schema)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-92, D-84 force semantics)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 1031-1059 — exact YAML insertion site)
  </read_first>
  <action>
Edit `openapi/openapi.yaml`. Locate the `PatchAgentStatusRequest` schema at line 1075. After the `post_interaction_state` property block (currently ending at line 1121), and BEFORE the next sibling schema (the comment `# ── Bulk Import Schemas` at line 1123), insert a new property `force` with this exact YAML (preserve 8-space indentation matching `post_interaction_state`):

```yaml
        force:
          type: boolean
          default: false
          nullable: true
          description: >
            Admin override (D-84). When `true`, bypasses the transition matrix
            (skips HTTP 409 `invalid_transition`). Cross-row probes still run:
            a missing or cross-org `break_reason_id` still returns HTTP 422
            (D-84 Pitfall 3). v0.1 stub auth allows any caller to pass
            `force=true`; v1 AUTH phase restricts to `org_admin` role. Use
            sparingly — every `force=true` call emits a WARN-level audit log
            entry (`state.force.applied`).
```

Do NOT add `force` to the `required` list — the field is optional with default false. Do NOT modify any other schema, endpoint, or description. Do NOT add a new response type (D-92 explicitly only adds a request-body field per A7 in RESEARCH §Assumptions). The YAML must remain a single OpenAPI 3.0.0 document and pass Redocly lint.
  </action>
  <acceptance_criteria>
    - `grep -nE "^        force:" openapi/openapi.yaml` returns exactly one match within the PatchAgentStatusRequest schema (line range 1098-1125)
    - `grep -A1 "^        force:" openapi/openapi.yaml | grep -q "type: boolean"`
    - `grep -A2 "^        force:" openapi/openapi.yaml | grep -q "default: false"`
    - `grep -A3 "^        force:" openapi/openapi.yaml | grep -q "nullable: true"`
    - `grep -B1 -A12 "^        force:" openapi/openapi.yaml | grep -q "Admin override"`
    - `cd services/api && task lint:spec` exits 0 (Redocly lint accepts the spec)
    - `git diff openapi/openapi.yaml | grep -q "^+        force:"`
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && task lint:spec && grep -q "^        force:" /Users/luong/workspace/dev/open-solutions/openapi/openapi.yaml</automated>
  </verify>
  <done>openapi.yaml carries the new `force` field; Redocly lint accepts it; no other schema changes.</done>
</task>

<task type="auto">
  <name>Task 2: Regenerate Go codegen artifacts (oapi-codegen) and TS client (openapi-typescript)</name>
  <files>
    services/api/internal/api/server.gen.go,
    services/api/internal/api/types.gen.go,
    services/api/internal/api/spec.gen.go,
    web/packages/ui/src/api/generated.ts
  </files>
  <read_first>
    - services/api/Taskfile.yml (the `gen` task definition)
    - web/package.json (the `gen:api` script)
    - .planning/phases/02-openapi-contract-codegen/02-CONTEXT.md (D-47/D-48 codegen-drift CI gate)
  </read_first>
  <action>
Run two commands and commit the regenerated artifacts:

1. `cd services/api && task gen` — invokes oapi-codegen against the amended `openapi/openapi.yaml`. This regenerates `services/api/internal/api/server.gen.go` (adds `Force *bool` to `PatchAgentStatusJSONRequestBody`), `services/api/internal/api/types.gen.go` (the typed body shape), and `services/api/internal/api/spec.gen.go` (embedded spec bytes refresh).

2. `cd web && pnpm gen:api` — invokes openapi-typescript against the same spec. This regenerates `web/packages/ui/src/api/generated.ts` adding `force?: boolean | null` to the TS request body type.

Do NOT hand-edit any generated file. Do NOT modify any other source under `internal/api/` or `web/packages/ui/src/api/`. The only legal source-of-truth change is `openapi/openapi.yaml` from Task 1. If the regenerated diff touches more than the four files above, stop and investigate (it likely means the spec amendment in Task 1 changed adjacent content unintentionally).

After regen, run `cd services/api && go build ./...` to confirm the new request-body shape compiles cleanly against existing handlers (catalog/notimpl.go still owns the 501 stubs; the new `Force` field is just unused).
  </action>
  <acceptance_criteria>
    - `grep -n "Force\s*\*bool" services/api/internal/api/types.gen.go` returns at least one match within the `PatchAgentStatusJSONRequestBody` struct
    - `grep -n "force" web/packages/ui/src/api/generated.ts` returns at least one match in the PatchAgentStatusRequest TS body type
    - `cd services/api && go build ./...` exits 0
    - `cd services/api && task gen` is idempotent: running it a second time produces NO diff (`git diff --exit-code services/api/internal/api/` is clean)
    - `cd web && pnpm gen:api && git diff --exit-code web/packages/ui/src/api/generated.ts` is clean
    - Files modified in this task are exactly the four listed in `<files>` (verified via `git status --porcelain`)
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && task gen && cd ../.. && cd web && pnpm gen:api && cd .. && git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/generated.ts</automated>
  </verify>
  <done>Generated artifacts committed; `task gen` and `pnpm gen:api` are idempotent (no further diff); go build passes.</done>
</task>

<task type="auto">
  <name>Task 3: Append `agent_states` table + indexes to migrations/000002_catalog_v0_1.up.sql</name>
  <files>migrations/000002_catalog_v0_1.up.sql</files>
  <read_first>
    - migrations/000002_catalog_v0_1.up.sql (end of file; existing CAT-03 agent_skills table + final `COMMIT;`)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-78 exact schema)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 949-1014 — exact migration snippet)
  </read_first>
  <action>
Edit `migrations/000002_catalog_v0_1.up.sql`. The file ends with the `agent_skills` table block and a final `COMMIT;`. Locate the line `COMMIT;` (final line). INSERT the following block BEFORE `COMMIT;` (so the new statements are inside the same transaction as the rest of migration 000002 per D-61):

```sql

-- ---------------------------------------------------------------------------
-- STATE-01..STATE-10: agent_states (Phase 4 — D-78)
-- ---------------------------------------------------------------------------
-- One row per agent. PK = agent_id. NO FK (D-80 — app-layer probes only).
-- TEXT + CHECK encoding for status/engaged_channel/post_interaction_state
-- (D-79). Adding values = ALTER constraint, not ALTER TYPE on PG14-.
-- org_id is denormalized so SQLChecker (Phase 1 D-04) sees the column on
-- every sqlc-generated query; the sweeper goroutine bypasses ctx-injection
-- and relies on this column in the SQL WHERE clauses for org auditability.
CREATE TABLE agent_states (
    agent_id                UUID PRIMARY KEY,
    org_id                  UUID NOT NULL,
    status                  TEXT NOT NULL
                              CHECK (status IN ('Ready','NotReady','Break','Engaged','WrapUp','Offline')),
    engaged_channel         TEXT
                              CHECK (engaged_channel IS NULL OR engaged_channel IN ('voice','chat','email')),
    break_reason_id         UUID,
    post_interaction_state  TEXT
                              CHECK (post_interaction_state IS NULL OR post_interaction_state IN ('ready','not_ready')),
    wrapup_until            TIMESTAMPTZ,
    state_version           BIGINT NOT NULL DEFAULT 1,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ix_agent_states_org_status ON agent_states (org_id, status);

-- Partial index — sweeper hot path is `WHERE status='WrapUp' AND wrapup_until < NOW()`.
-- Partial index ensures the planner reads only the small subset of rows in WrapUp.
CREATE INDEX ix_agent_states_wrapup_until
    ON agent_states (wrapup_until)
    WHERE status = 'WrapUp';
```

Constraints:
- Use BIGINT for `state_version` (NOT INTEGER) — state machine fires more frequently than catalog updates per RESEARCH §Schema.
- NO foreign key constraints — D-80 invariant. Do NOT add `REFERENCES agents(id)` or `REFERENCES break_reasons(id)`.
- NO UNIQUE constraint beyond the PK — agent_id PK already enforces uniqueness.
- NO `created_at` column — only `updated_at` (the row is born at agent CREATE time; no separate creation timestamp needed per D-78).
- The partial index `ix_agent_states_wrapup_until` MUST include the `WHERE status = 'WrapUp'` predicate verbatim — without the predicate the sweeper hot path scans every WrapUp row regardless of status.
  </action>
  <acceptance_criteria>
    - `grep -n "CREATE TABLE agent_states" migrations/000002_catalog_v0_1.up.sql` returns exactly one match
    - `grep -n "CREATE INDEX ix_agent_states_org_status" migrations/000002_catalog_v0_1.up.sql` returns exactly one match
    - `grep -n "CREATE INDEX ix_agent_states_wrapup_until" migrations/000002_catalog_v0_1.up.sql` returns exactly one match
    - `grep -n "WHERE status = 'WrapUp'" migrations/000002_catalog_v0_1.up.sql` returns at least one match (partial index predicate)
    - `grep -cE "REFERENCES|FOREIGN KEY" migrations/000002_catalog_v0_1.up.sql` returns 0 (D-80 — no FKs)
    - `grep -n "state_version           BIGINT" migrations/000002_catalog_v0_1.up.sql` returns one match (BIGINT not INTEGER)
    - `awk '/CREATE TABLE agent_states/,/COMMIT;/' migrations/000002_catalog_v0_1.up.sql | grep -q "CHECK (status IN ('Ready','NotReady','Break','Engaged','WrapUp','Offline'))"`
    - File ends with `COMMIT;` on the last non-empty line (new block sits ABOVE the COMMIT)
  </acceptance_criteria>
  <verify>
    <automated>grep -q "CREATE TABLE agent_states" /Users/luong/workspace/dev/open-solutions/migrations/000002_catalog_v0_1.up.sql && grep -q "ix_agent_states_wrapup_until" /Users/luong/workspace/dev/open-solutions/migrations/000002_catalog_v0_1.up.sql && [ "$(grep -cE 'REFERENCES|FOREIGN KEY' /Users/luong/workspace/dev/open-solutions/migrations/000002_catalog_v0_1.up.sql)" -eq 0 ]</automated>
  </verify>
  <done>Migration carries agent_states table + 2 indexes inside the existing BEGIN/COMMIT block; zero FKs; BIGINT state_version; partial index WHERE predicate present.</done>
</task>

<task type="auto">
  <name>Task 4: Prepend `DROP TABLE IF EXISTS agent_states` to migrations/000002_catalog_v0_1.down.sql</name>
  <files>migrations/000002_catalog_v0_1.down.sql</files>
  <read_first>
    - migrations/000002_catalog_v0_1.down.sql (full file — existing BEGIN/COMMIT block with reverse-order drops)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 1016-1027 — exact down-migration insertion site)
  </read_first>
  <action>
Edit `migrations/000002_catalog_v0_1.down.sql`. Locate the first `DROP TABLE IF EXISTS agent_skills;` line inside the `BEGIN;` block. Insert the following BEFORE that line (so `agent_states` drops first, matching the reverse order of the up migration where it was appended last):

```sql
-- Phase 4 — agent_states (D-78) reverse. Drops first because it was
-- appended LAST in the up migration; no FK dependencies exist (D-80) so
-- ordering is documentation-only.
DROP TABLE IF EXISTS agent_states;
```

The down migration must remain idempotent (`IF EXISTS`) and reside inside the existing `BEGIN; ... COMMIT;` block. Do NOT remove or reorder any existing DROP statements.
  </action>
  <acceptance_criteria>
    - `grep -n "DROP TABLE IF EXISTS agent_states" migrations/000002_catalog_v0_1.down.sql` returns exactly one match
    - `awk '/BEGIN;/,/COMMIT;/' migrations/000002_catalog_v0_1.down.sql | grep -q "DROP TABLE IF EXISTS agent_states"`
    - The `DROP TABLE IF EXISTS agent_states` line comes BEFORE `DROP TABLE IF EXISTS agent_skills` in the file (verified with `awk '/DROP TABLE/{print NR": "$0}'` showing agent_states at a lower line number than agent_skills)
  </acceptance_criteria>
  <verify>
    <automated>grep -q "DROP TABLE IF EXISTS agent_states" /Users/luong/workspace/dev/open-solutions/migrations/000002_catalog_v0_1.down.sql && [ "$(awk '/DROP TABLE IF EXISTS agent_states/{print NR; exit}' /Users/luong/workspace/dev/open-solutions/migrations/000002_catalog_v0_1.down.sql)" -lt "$(awk '/DROP TABLE IF EXISTS agent_skills/{print NR; exit}' /Users/luong/workspace/dev/open-solutions/migrations/000002_catalog_v0_1.down.sql)" ]</automated>
  </verify>
  <done>Down migration drops agent_states first, before agent_skills.</done>
</task>

<task type="auto">
  <name>Task 5: Add `agent_states` to `tenantTables` allowlist in services/api/internal/db/sqlcheck.go</name>
  <files>services/api/internal/db/sqlcheck.go</files>
  <read_first>
    - services/api/internal/db/sqlcheck.go (lines 32-41 — existing tenantTables map)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 791-810 — exact insertion site + Pitfall 4)
  </read_first>
  <action>
Edit `services/api/internal/db/sqlcheck.go`. Locate the `tenantTables` map literal at lines 32-41 (verified via grep). The current contents are:

```go
var tenantTables = map[string]struct{}{
    "_scaffold":     {}, // legacy Phase 1; removed once scaffold queries drop out
    "agents":        {}, // CAT-01
    "skills":        {}, // CAT-02
    "queues":        {}, // CAT-04
    "channels":      {}, // CAT-05
    "adapters":      {}, // CAT-06
    "break_reasons": {}, // CAT-07
    "agent_skills":  {}, // CAT-03 (junction; carries denormalized org_id per D-72)
}
```

Insert ONE new entry for `agent_states`. Pick a position adjacent to `agent_skills` (both `agent_*` names) — append immediately AFTER the `agent_skills` line. The new line:

```go
    "agent_states":  {}, // STATE-01 (Phase 4; denormalized org_id per D-78 — sweeper bypasses ctx-org and relies on this column)
```

Pitfall 4: without this entry, the FIRST sqlc-generated query against `agent_states` panics in dev/test with `ErrSQLMissingOrgFilter` because the SQLChecker rejects every query that references a table not in the allowlist. This task MUST land in Wave 0 before Wave 1 generates the sqlc queries against the new table.

Do NOT remove or reorder existing entries. Do NOT widen the comment on any sibling line.
  </action>
  <acceptance_criteria>
    - `grep -n "\"agent_states\":" services/api/internal/db/sqlcheck.go` returns exactly one match
    - `awk '/var tenantTables = map\[string\]struct{}{/,/^}$/' services/api/internal/db/sqlcheck.go | grep -q "\"agent_states\":"`
    - `cd services/api && go build ./...` exits 0
    - `cd services/api && go test -run TestSQLChecker -count=1 ./internal/db/...` exits 0 (existing SQLChecker test suite unaffected)
    - The existing entries (`agents`, `skills`, `queues`, `channels`, `adapters`, `break_reasons`, `agent_skills`) are still present (verified with `grep -c '":' services/api/internal/db/sqlcheck.go` ≥ 9 inside the map)
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go build ./... && grep -q '"agent_states":' /Users/luong/workspace/dev/open-solutions/services/api/internal/db/sqlcheck.go && go test -run TestSQLChecker -count=1 ./internal/db/...</automated>
  </verify>
  <done>tenantTables carries agent_states entry; SQLChecker tests still pass; go build clean.</done>
</task>

<task type="checkpoint:human-verify" gate="blocking-human">
  <name>Task 6: Verify clockwork package legitimacy before install</name>
  <what-built>
    Phase 4 requires `github.com/jonboulle/clockwork v0.4.0` for injectable Clock with AfterFunc support in the TTL goroutine (Wave 3). Per RESEARCH §Package Legitimacy Audit, this package is marked `[ASSUMED]` because slopcheck was unavailable in the research environment. v0.1 stack policy (project CLAUDE.md + cross-AI peer-review HARD RULE) requires human verification before any `go get` lands.
  </what-built>
  <how-to-verify>
    1. Open https://pkg.go.dev/github.com/jonboulle/clockwork in a browser. Confirm:
       - Repository: github.com/jonboulle/clockwork
       - Latest published version: v0.4.0 or higher (v0.5.x is acceptable; do NOT downgrade below v0.4 — AfterFunc support landed in v0.3 and v0.4 stabilized the FakeClock semantics).
       - License: Apache-2.0
       - Imported by: visible high-trust consumers (Kubernetes, etcd, CockroachDB) — confirms not a slopsquatted lookalike.
    2. Open https://github.com/jonboulle/clockwork. Confirm:
       - Stars > 1000
       - Last commit within past 12 months
       - README documents `Clock` interface with `Now()`, `AfterFunc(d, f)` returning `Timer`
       - NO recent fork/transfer suspicious of takeover
    3. Confirm package name spelling EXACTLY matches: `jonboulle/clockwork` (NOT `jonbouille`, `clockworks`, or any homoglyph variant). The repo URL must be `https://github.com/jonboulle/clockwork`.
    4. If any of (1)(2)(3) fails, respond "BLOCKED — clockwork audit failed: <reason>" and do NOT proceed. Phase 4 will need a replacement clock library (benbjohnson/clock is archived; coder/quartz is younger).
  </how-to-verify>
  <resume-signal>Type "approved — clockwork verified" or describe issues.</resume-signal>
</task>

<task type="auto">
  <name>Task 7: `go get github.com/jonboulle/clockwork@v0.4.0` and commit go.mod/go.sum</name>
  <files>services/api/go.mod, services/api/go.sum</files>
  <read_first>
    - services/api/go.mod (current dependency declarations)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (Package Legitimacy Audit table)
  </read_first>
  <action>
After Task 6 passes the human checkpoint, run:

```bash
cd services/api && go get github.com/jonboulle/clockwork@v0.4.0
```

Then run `go mod tidy` to refresh `go.sum` and prune any unused indirect entries.

Verify the install:

```bash
go mod why github.com/jonboulle/clockwork
go mod download github.com/jonboulle/clockwork@v0.4.0
go list -m github.com/jonboulle/clockwork
```

Commit both `services/api/go.mod` and `services/api/go.sum`. Do NOT pin to a pre-release tag (v0.4.0-rc1 etc.) and do NOT pull a major beyond v0.x (v1.x does not exist as of this writing).

If `go get` fails (network error, missing module proxy), retry once. If it fails twice, surface "BLOCKED — go get clockwork failed: <error>" and stop. Do NOT use `replace` directives or vendor the dependency.
  </action>
  <acceptance_criteria>
    - `grep -q "github.com/jonboulle/clockwork v0.4" services/api/go.mod`
    - `cd services/api && go mod why github.com/jonboulle/clockwork` exits 0 and prints a non-empty reason chain
    - `cd services/api && go list -m github.com/jonboulle/clockwork` prints `github.com/jonboulle/clockwork v0.4.0` (or v0.4.x patch)
    - `cd services/api && go build ./...` exits 0 (no consumers yet, but build must remain clean)
    - `git diff services/api/go.mod | grep -q "+\s*github.com/jonboulle/clockwork"`
    - `services/api/go.sum` carries hash entries for clockwork (verified with `grep -q "jonboulle/clockwork" services/api/go.sum`)
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && grep -q "github.com/jonboulle/clockwork v0.4" go.mod && go list -m github.com/jonboulle/clockwork && go build ./...</automated>
  </verify>
  <done>clockwork in go.mod at v0.4.x; go.sum hashes present; go build clean.</done>
</task>

<task type="auto">
  <name>Task 8: Validate full Wave 0 by running `task db:reset && task test` + codegen-drift CI gate</name>
  <files>(no source modifications — validation only)</files>
  <read_first>
    - services/api/Taskfile.yml (`db:reset`, `test`, `gen` tasks)
    - .planning/phases/02-openapi-contract-codegen/02-CONTEXT.md (D-47/D-48 codegen-drift gate)
  </read_first>
  <action>
Run the following validation sequence in order. STOP on first non-zero exit and report which step failed.

1. `cd services/api && task db:reset` — drops and recreates the dev database, applies all migrations including the new agent_states table. Must exit 0.
2. `cd services/api && task test` — runs the full Go test suite. All existing Phase 3 tests must still pass (no regressions from Wave 0 changes). Must exit 0.
3. `cd services/api && task gen && cd ../.. && cd web && pnpm gen:api && cd ..` — regenerate from the amended spec. Output must be byte-identical to what was committed in Task 2.
4. `git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/generated.ts openapi/openapi.yaml` — must exit 0 (no drift between committed and freshly generated).
5. `psql $DATABASE_URL -c "\d agent_states"` — must show the table with 8 columns and the two indexes (`ix_agent_states_org_status`, `ix_agent_states_wrapup_until`).

If step 2 fails because of a test referencing the not-yet-created state package, that is unexpected for Wave 0 — investigate before proceeding to Wave 1.

NO source modifications in this task — it is verification-only.
  </action>
  <acceptance_criteria>
    - `cd services/api && task db:reset` exits 0
    - `cd services/api && task test` exits 0 (full Phase 3 suite still green)
    - `cd services/api && task gen && cd ../.. && cd web && pnpm gen:api` is idempotent — `git diff --exit-code` exits 0 after regen
    - `psql $DATABASE_URL -c "\d agent_states"` shows columns: agent_id (uuid PK), org_id (uuid NOT NULL), status (text NOT NULL), engaged_channel (text NULL), break_reason_id (uuid NULL), post_interaction_state (text NULL), wrapup_until (timestamptz NULL), state_version (bigint NOT NULL DEFAULT 1), updated_at (timestamptz NOT NULL DEFAULT now())
    - `psql $DATABASE_URL -c "\di agent_states*"` lists both `ix_agent_states_org_status` and `ix_agent_states_wrapup_until`
    - No regressions in Phase 3 isolation suite: `cd services/api && go test ./test/isolation/...` exits 0
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && task db:reset && task test && task gen && cd ../.. && cd web && pnpm gen:api && cd .. && git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/generated.ts openapi/openapi.yaml</automated>
  </verify>
  <done>Migration applies cleanly; Phase 3 suite still green; codegen-drift gate passes; agent_states table observable in PostgreSQL.</done>
</task>

</tasks>

<verification>
- Spec amendment is the ONLY hand-edit on openapi.yaml (codegen artifacts regenerated, not hand-edited)
- agent_states table sits inside the existing migration 000002 BEGIN/COMMIT block (D-61 single editable migration invariant)
- Zero FK constraints in migrations (`grep -cE "REFERENCES|FOREIGN KEY" migrations/000002_catalog_v0_1.up.sql` = 0 per D-80)
- tenantTables allowlist carries `agent_states` BEFORE Wave 1 generates sqlc queries (Pitfall 4 mitigation)
- clockwork dependency in go.mod at v0.4.x (legitimacy verified via human checkpoint)
- `task gen && git diff --exit-code` is clean (D-47/D-48 codegen-drift CI gate)
- Phase 3 isolation suite still passes (zero regression)
</verification>

<success_criteria>
- 8 tasks completed, including the blocking human checkpoint on clockwork legitimacy
- openapi.yaml + 4 regenerated artifacts + 2 migration files + sqlcheck.go + go.mod + go.sum committed
- Migration applies cleanly via golang-migrate (`task db:reset` exits 0)
- Full Phase 3 test suite still green (zero regression)
- agent_states table observable in PostgreSQL with all 8 columns and 2 indexes
- `task gen && git diff --exit-code` is clean (codegen-drift CI gate)
- clockwork installable via `go list -m`
- Hazard 3 (Pitfall 4 — tenantTables panic) eliminated before Wave 1 generates queries
- Pitfall 6 (Phase 5 inheritance) is NOT introduced here but reminded for Wave 4 modification to catalog/agents.go
</success_criteria>

<output>
Create `.planning/phases/04-agent-state-machine-go/04-01-SUMMARY.md` when done. Include: list of all 9 files modified, hashes/lines for the migration block, output of `task db:reset && task test`, confirmation that codegen-drift CI gate passes locally, clockwork version pinned.
</output>
---
phase: 04-agent-state-machine-go
plan: 02
type: execute
wave: 1
depends_on:
  - 04-01
files_modified:
  - services/api/internal/db/queries/agent_states.sql
  - services/api/internal/db/queries/break_reasons.sql
  - services/api/internal/db/generated/agent_states.sql.go
  - services/api/internal/db/generated/break_reasons.sql.go
  - services/api/internal/db/generated/models.go
  - services/api/internal/db/generated/querier.go
  - services/api/internal/domain/state.go
  - services/api/internal/domain/state_test.go
  - services/api/internal/state/handlers.go
  - services/api/internal/state/transitions.go
  - services/api/internal/state/transitions_test.go
  - services/api/internal/state/agent_states.go
autonomous: true
requirements:
  - STATE-01
  - STATE-02
  - STATE-05
  - STATE-08
  - STATE-09
  - STATE-10

must_haves:
  truths:
    - "sqlc-generated queries exist for InsertAgentState, GetAgentStateByAgentId, UpdateAgentStateStatus, ForceUpdateAgentStateStatus, ListExpiringWrapUps, ExpireWrapUp"
    - "break_reasons.sql carries GetBreakReasonForState combined probe (returns routable flag scoped by org)"
    - "IsRoutable pure function lives at services/api/internal/domain/state.go with passing 4-case matrix test (STATE-10)"
    - "state package skeleton compiles: state.Server (NOT Handlers per Pitfall 1), Deps + Options, stub GetAgentStatus/PatchAgentStatus"
    - "Transition matrix in transitions.go covers exactly the 7 agent-initiated edges; exhaustive 36-pair test passes"
  artifacts:
    - path: "services/api/internal/db/queries/agent_states.sql"
      provides: "6 sqlc queries — INSERT/GET/UPDATE/FORCE-UPDATE/LIST-EXPIRING/EXPIRE"
      contains: "name: InsertAgentState"
    - path: "services/api/internal/db/queries/break_reasons.sql"
      provides: "Appended GetBreakReasonForState probe"
      contains: "name: GetBreakReasonForState"
    - path: "services/api/internal/domain/state.go"
      provides: "IsRoutable pure function + AgentStateInputs struct (STATE-10)"
      contains: "func IsRoutable"
    - path: "services/api/internal/state/handlers.go"
      provides: "state.Server + Deps + New + WithClock + WithSweepInterval (lifecycle stubbed for Wave 3)"
      contains: "type Server struct"
    - path: "services/api/internal/state/transitions.go"
      provides: "matrix + validateTransition + ErrInvalidTransition (STATE-02/03/05/09)"
      contains: "var matrix = map[AgentStatus]map[AgentStatus]TransitionRule"
    - path: "services/api/internal/state/agent_states.go"
      provides: "Stub GetAgentStatus + PatchAgentStatus — real bodies Wave 2"
      contains: "func (s *Server) GetAgentStatus"
  key_links:
    - from: "services/api/internal/state/transitions.go"
      to: "services/api/internal/api/types.gen.go"
      via: "type AgentStatus = api.AgentStatus alias"
      pattern: "type AgentStatus = api.AgentStatus"
    - from: "services/api/internal/db/queries/agent_states.sql"
      to: "services/api/internal/db/generated/agent_states.sql.go"
      via: "sqlc codegen via task gen"
      pattern: "func .*InsertAgentState"
---

<objective>
Generate the sqlc query surface for `agent_states` and the new `break_reasons` combined probe (D-76 pattern), bootstrap the pure `internal/domain/` package with `IsRoutable` + matrix tests (STATE-10), and stand up the `internal/state/` package skeleton with the type named **`state.Server`** (NOT `Handlers`) from the very first commit per Pitfall 1.

Purpose: Wave 1 establishes the COMPILE-TIME contracts that Waves 2-4 build on. Everything below must compile and pass unit tests without changing behavior in any existing handler. Wave 2 fills in real handler bodies; Wave 3 fills in the TTL goroutine; Wave 4 wires the composite. By forking the type name `Server` here (not retrofitting in Wave 4), we eliminate the cascading rename Pitfall 1 calls out.

Output: Working `internal/domain/` with passing IsRoutable test; working `internal/state/` skeleton with compiling Server struct + Options + stub methods + passing transition matrix test; regenerated sqlc bindings for agent_states + break_reasons probe; zero behavior change in catalog handlers.

**Hazard captured here (Pitfall 1 — Hazard 1 in PATTERNS.md):** The state package type is named `Server` because `*catalog.Handlers + *state.Handlers` would fail to compile when embedded into the Wave 4 ApiHandlers composite (Go rejects "duplicate field name Handlers"). Wave 1 ships the rename forward so Wave 4 has no rename to do — only the wiring.
</objective>

<execution_context>
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/workflows/execute-plan.md
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@./CLAUDE.md
@.planning/phases/04-agent-state-machine-go/04-CONTEXT.md
@.planning/phases/04-agent-state-machine-go/04-RESEARCH.md
@.planning/phases/04-agent-state-machine-go/04-PATTERNS.md
@.planning/phases/04-agent-state-machine-go/04-VALIDATION.md
@.planning/phases/04-agent-state-machine-go/04-01-PLAN.md
@.planning/phases/04-agent-state-machine-go/04-01-SUMMARY.md
@.planning/phases/03-catalog-crud-go/03-CONTEXT.md
@services/api/internal/catalog/handlers.go
@services/api/internal/catalog/agents.go
@services/api/internal/db/queries/agents.sql
@services/api/internal/db/queries/break_reasons.sql
@services/api/internal/db/queries/queues.sql
@services/api/internal/api/types.gen.go
@services/api/internal/api/server.gen.go
</context>

<interfaces>
From `services/api/internal/api/types.gen.go` (post-Wave 0 regen):
```go
type AgentStatus string
const (
    AgentStatusNotReady AgentStatus = "NotReady"
    AgentStatusReady    AgentStatus = "Ready"
    AgentStatusBreak    AgentStatus = "Break"
    AgentStatusEngaged  AgentStatus = "Engaged"
    AgentStatusWrapUp   AgentStatus = "WrapUp"
    AgentStatusOffline  AgentStatus = "Offline"
)

type PatchAgentStatusJSONRequestBody struct {
    To                   AgentStatus           `json:"to"`
    BreakReasonId        *openapi_types.UUID   `json:"break_reason_id,omitempty"`
    PostInteractionState *PostInteractionState `json:"post_interaction_state,omitempty"`
    Force                *bool                 `json:"force,omitempty"`  // ← Wave 0 addition
}
```

From `services/api/internal/catalog/handlers.go` (analog Deps/Options pattern):
```go
type Deps struct { OrgDB *db.OrgDB; Pool *pgxpool.Pool; Cache *cache.Cache; Logger *slog.Logger }
type Option func(*Handlers)
func New(deps Deps, opts ...Option) *Handlers { ... }
```

Phase 4 mirrors with: `state.Server` (NOT Handlers), `state.Deps` (adds `WrapUpDuration time.Duration`; omits `Pool`), `state.Option`, `WithClock(c clockwork.Clock)`, `WithSweepInterval(d time.Duration)`.
</interfaces>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| sqlc-generated SQL → SQLChecker | Every agent_states query mentions org_id in WHERE (or SELECT for sweeper); tenantTables allowlist from Wave 0 already covers `agent_states`. |
| Caller of validateTransition → matrix | Pure data; matrix is unexported `var`; no I/O. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-04-04 | Information Disclosure | sqlc WHERE clauses for agent_states | mitigate | Every query in this wave includes `org_id` predicate. SQLChecker enforces at runtime. |
| T-04-05 | Tampering | Transition matrix | mitigate | matrix is unexported; tests assert exactly 7 agent-initiated edges; system-only edges (Engaged→WrapUp, *→Offline) absent. |
</threat_model>

<tasks>

<task type="auto">
  <name>Task 1: Create agent_states.sql with 6 sqlc queries + run task gen</name>
  <files>
    services/api/internal/db/queries/agent_states.sql,
    services/api/internal/db/generated/agent_states.sql.go,
    services/api/internal/db/generated/models.go,
    services/api/internal/db/generated/querier.go
  </files>
  <read_first>
    - services/api/internal/db/queries/agents.sql (analog InsertAgent + UpdateAgent COALESCE pattern lines 24-95)
    - services/api/internal/db/queries/queues.sql (analog QueueExistsAndEnabledInOrg probe lines 70-80)
    - migrations/000002_catalog_v0_1.up.sql (agent_states column list — names must match exactly)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 581-668 — sqlc template)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (§Pattern 3 — UPDATE WHERE status = expected_from)
  </read_first>
  <action>
Create a new file `services/api/internal/db/queries/agent_states.sql` with EXACTLY these 6 named queries (use sqlc tag conventions `:one`, `:many`):

```sql
-- name: InsertAgentState :one
-- D-93: called from catalog.CreateAgent inside the existing OrgTx so
-- agent + agent_state INSERTs commit atomically (Codex C4 pattern).
INSERT INTO agent_states (agent_id, org_id, status, state_version)
VALUES ($1, $2, $3, 1)
RETURNING agent_id, org_id, status, engaged_channel, break_reason_id,
         post_interaction_state, wrapup_until, state_version, updated_at;

-- name: GetAgentStateByAgentId :one
-- Used by GET /agents/{id}/status handler (cache loader) and by the
-- 0-row disambiguate path in PatchAgentStatus (D-66 adapted for D-85).
SELECT agent_id, org_id, status, engaged_channel, break_reason_id,
       post_interaction_state, wrapup_until, state_version, updated_at
FROM agent_states
WHERE agent_id = $1 AND org_id = $2;

-- name: UpdateAgentStateStatus :one
-- D-85: WHERE clause uses `status = expected_from` (matrix-driven gate),
-- NOT `state_version = expected_version`. 0 rows → handler runs
-- GetAgentStateByAgentId to disambiguate 404 vs 409 invalid_transition.
UPDATE agent_states
SET status                 = COALESCE(sqlc.narg('to_status')::text, status),
    engaged_channel        = sqlc.narg('engaged_channel')::text,
    break_reason_id        = sqlc.narg('break_reason_id')::uuid,
    post_interaction_state = COALESCE(sqlc.narg('post_interaction_state')::text, post_interaction_state),
    wrapup_until           = sqlc.narg('wrapup_until')::timestamptz,
    state_version          = state_version + 1,
    updated_at             = NOW()
WHERE agent_id = sqlc.arg('agent_id')
  AND org_id   = sqlc.arg('org_id')
  AND status   = sqlc.arg('expected_from')
RETURNING agent_id, org_id, status, engaged_channel, break_reason_id,
         post_interaction_state, wrapup_until, state_version, updated_at;

-- name: ForceUpdateAgentStateStatus :one
-- D-84: bypasses the transition matrix (no `status = expected_from`).
-- Cross-row break_reason probe STILL runs at handler layer (Pitfall 3 —
-- force does NOT bypass cross-org probes).
UPDATE agent_states
SET status                 = COALESCE(sqlc.narg('to_status')::text, status),
    engaged_channel        = sqlc.narg('engaged_channel')::text,
    break_reason_id        = sqlc.narg('break_reason_id')::uuid,
    post_interaction_state = COALESCE(sqlc.narg('post_interaction_state')::text, post_interaction_state),
    wrapup_until           = sqlc.narg('wrapup_until')::timestamptz,
    state_version          = state_version + 1,
    updated_at             = NOW()
WHERE agent_id = sqlc.arg('agent_id')
  AND org_id   = sqlc.arg('org_id')
RETURNING agent_id, org_id, status, engaged_channel, break_reason_id,
         post_interaction_state, wrapup_until, state_version, updated_at;

-- name: ListExpiringWrapUps :many
-- Called from sweeper goroutine via OrgDB.WithBypass (D-04). SELECT
-- mentions org_id explicitly so SQLChecker approves the query
-- (tenantTables entry from Wave 0 + denormalized column from D-78).
-- Returns ALL WrapUp rows so the sweeper can schedule per-agent
-- AfterFunc timers OR fire-immediately for past-due rows (D-81).
SELECT agent_id, org_id, wrapup_until, post_interaction_state
FROM agent_states
WHERE status = 'WrapUp'
ORDER BY org_id, wrapup_until;

-- name: ExpireWrapUp :one
-- Idempotent (D-81): only the first concurrent firing matches the
-- predicate. Returns org_id + new status so the sweeper can invalidate
-- the right cache key (Pitfall 8 — sweeper has no ctx-injected org_id).
UPDATE agent_states
SET status        = COALESCE(post_interaction_state, 'NotReady'),
    wrapup_until  = NULL,
    state_version = state_version + 1,
    updated_at    = NOW()
WHERE agent_id = $1
  AND org_id   = $2
  AND status   = 'WrapUp'
  AND wrapup_until < NOW()
RETURNING agent_id, org_id, status, state_version;
```

Then run `cd services/api && task gen` to regenerate `internal/db/generated/agent_states.sql.go`, refresh `models.go` (adds AgentState row struct), and update `querier.go` interface with 6 new method signatures.

Hazards:
- ExpireWrapUp uses `:one` with RETURNING (NOT `:execrows`) because the sweeper needs org_id back for cache invalidation per Pitfall 8.
- UpdateAgentStateStatus and ForceUpdateAgentStateStatus differ in EXACTLY one WHERE term — the `AND status = sqlc.arg('expected_from')` line is present in the former and absent in the latter.
- ListExpiringWrapUps does NOT filter by `wrapup_until < NOW()` — returns ALL WrapUp rows so startup sweep can schedule future timers AND fire past-due rows.
  </action>
  <acceptance_criteria>
    - `grep -c "^-- name:" services/api/internal/db/queries/agent_states.sql` returns 6
    - `grep -q "^-- name: InsertAgentState :one$" services/api/internal/db/queries/agent_states.sql`
    - `grep -q "^-- name: GetAgentStateByAgentId :one$" services/api/internal/db/queries/agent_states.sql`
    - `grep -q "^-- name: UpdateAgentStateStatus :one$" services/api/internal/db/queries/agent_states.sql`
    - `grep -q "^-- name: ForceUpdateAgentStateStatus :one$" services/api/internal/db/queries/agent_states.sql`
    - `grep -q "^-- name: ListExpiringWrapUps :many$" services/api/internal/db/queries/agent_states.sql`
    - `grep -q "^-- name: ExpireWrapUp :one$" services/api/internal/db/queries/agent_states.sql`
    - `awk '/^-- name: UpdateAgentStateStatus/,/^-- name:/' services/api/internal/db/queries/agent_states.sql | grep -q "AND status   = sqlc.arg('expected_from')"`
    - `awk '/^-- name: ForceUpdateAgentStateStatus/,/^-- name:/' services/api/internal/db/queries/agent_states.sql | grep -vc expected_from` returns 0 zero-matches inside the FORCE block (force does NOT use expected_from)
    - `cd services/api && task gen` exits 0
    - `grep -q "func (q \*Queries) InsertAgentState" services/api/internal/db/generated/agent_states.sql.go`
    - `grep -q "func (q \*Queries) ExpireWrapUp" services/api/internal/db/generated/agent_states.sql.go`
    - `grep -q "type AgentState struct" services/api/internal/db/generated/models.go`
    - `cd services/api && go build ./...` exits 0
    - `cd services/api && go test -run TestSQLChecker -count=1 ./internal/db/...` exits 0
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && task gen && go build ./... && grep -q "func (q \*Queries) InsertAgentState" /Users/luong/workspace/dev/open-solutions/services/api/internal/db/generated/agent_states.sql.go && grep -q "func (q \*Queries) ExpireWrapUp" /Users/luong/workspace/dev/open-solutions/services/api/internal/db/generated/agent_states.sql.go && [ "$(grep -c '^-- name:' /Users/luong/workspace/dev/open-solutions/services/api/internal/db/queries/agent_states.sql)" -eq 6 ]</automated>
  </verify>
  <done>6 sqlc queries authored; sqlc regen clean; SQLChecker passes; go build clean.</done>
</task>

<task type="auto">
  <name>Task 2: Append GetBreakReasonForState combined probe to break_reasons.sql</name>
  <files>
    services/api/internal/db/queries/break_reasons.sql,
    services/api/internal/db/generated/break_reasons.sql.go,
    services/api/internal/db/generated/querier.go
  </files>
  <read_first>
    - services/api/internal/db/queries/break_reasons.sql (full file — append at end after SoftDeleteBreakReason)
    - services/api/internal/db/queries/queues.sql (analog QueueExistsAndEnabledInOrg probe lines 70-80)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 814-833)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (§Pattern 5 — combined probe rationale)
  </read_first>
  <action>
Edit `services/api/internal/db/queries/break_reasons.sql`. After the LAST existing query, append (separated by blank line):

```sql

-- name: GetBreakReasonForState :one
-- D-76 cross-row probe — used by PatchAgentStatus (STATE-04 422
-- invalid_reference path) and IsRoutable callers (STATE-10). Returns
-- the routable flag when the reason exists + is enabled in the caller's
-- org; pgx.ErrNoRows otherwise. Same-org enforcement is the FOUND-08
-- invariant (cross-org reason ids return ErrNoRows → handler maps to
-- 422 invalid_reference per D-75). Combined probe per RESEARCH
-- §Pattern 5: one query covers BOTH STATE-04 existence check AND
-- STATE-10 routable scalar; keeps the sqlc surface minimal.
SELECT routable
FROM break_reasons
WHERE id = $1 AND org_id = $2 AND enabled = TRUE
LIMIT 1;
```

Then run `cd services/api && task gen` to regenerate `internal/db/generated/break_reasons.sql.go` (adds `func (q *Queries) GetBreakReasonForState(ctx, params) (bool, error)` — return type is the `bool` scalar from the SELECT).

Hazards:
- The probe filters `enabled = TRUE` — soft-deleted break_reasons MUST surface as ErrNoRows so handler returns 422 (consistent with Phase 3 D-09).
- Return type is `bool` (the routable scalar) — NOT a wrapper struct. Handler distinguishes "found" from "not found" via `errors.Is(err, pgx.ErrNoRows)`.
- Do NOT modify any existing query in break_reasons.sql.
  </action>
  <acceptance_criteria>
    - `grep -q "^-- name: GetBreakReasonForState :one$" services/api/internal/db/queries/break_reasons.sql`
    - `grep -A4 "name: GetBreakReasonForState" services/api/internal/db/queries/break_reasons.sql | grep -q "WHERE id = \$1 AND org_id = \$2 AND enabled = TRUE"`
    - `cd services/api && task gen` exits 0
    - `grep -q "func (q \*Queries) GetBreakReasonForState" services/api/internal/db/generated/break_reasons.sql.go`
    - Generated function returns `(bool, error)` — verified by `awk '/func \(q \*Queries\) GetBreakReasonForState/,/^}/' services/api/internal/db/generated/break_reasons.sql.go | grep -q "(bool, error)"`
    - `cd services/api && go build ./...` exits 0
    - `cd services/api && go test -run TestSQLChecker -count=1 ./internal/db/...` exits 0
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && task gen && go build ./... && grep -q "^-- name: GetBreakReasonForState :one$" /Users/luong/workspace/dev/open-solutions/services/api/internal/db/queries/break_reasons.sql && grep -q "func (q \*Queries) GetBreakReasonForState" /Users/luong/workspace/dev/open-solutions/services/api/internal/db/generated/break_reasons.sql.go</automated>
  </verify>
  <done>Combined probe appended; sqlc emits Go binding returning bool; SQLChecker accepts query.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Create internal/domain/state.go — IsRoutable pure function (STATE-10) — TDD</name>
  <files>
    services/api/internal/domain/state.go,
    services/api/internal/domain/state_test.go
  </files>
  <read_first>
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-90 signature)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (§Code Examples Domain — verbatim shape)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 519-577)
    - services/api/internal/api/types.gen.go (AgentStatus constants)
  </read_first>
  <behavior>
    - IsRoutable returns true when Status == "Ready" (regardless of BreakReasonRoutable).
    - IsRoutable returns true when Status == "Break" AND BreakReasonRoutable == true.
    - IsRoutable returns false when Status == "Break" AND BreakReasonRoutable == false.
    - IsRoutable returns false for "NotReady", "Engaged", "WrapUp", "Offline" regardless of BreakReasonRoutable.
    - IsRoutable returns false for unknown/empty status (default deny).
    - IsRoutable does NO database access, NO logging, NO I/O — pure transformation only.
  </behavior>
  <action>
TDD: write `state_test.go` FIRST with the cases below, then write `state.go` to satisfy them.

Create `services/api/internal/domain/state_test.go`:

```go
package domain

import (
    "testing"

    "github.com/stretchr/testify/require"
)

func TestIsRoutable(t *testing.T) {
    cases := []struct {
        name string
        in   AgentStateInputs
        want bool
    }{
        {"Ready returns true", AgentStateInputs{Status: "Ready"}, true},
        {"Ready ignores BreakReasonRoutable=false", AgentStateInputs{Status: "Ready", BreakReasonRoutable: false}, true},
        {"Break+routable=true returns true", AgentStateInputs{Status: "Break", BreakReasonRoutable: true}, true},
        {"Break+routable=false returns false", AgentStateInputs{Status: "Break", BreakReasonRoutable: false}, false},
        {"NotReady returns false", AgentStateInputs{Status: "NotReady"}, false},
        {"Engaged returns false", AgentStateInputs{Status: "Engaged"}, false},
        {"WrapUp returns false", AgentStateInputs{Status: "WrapUp"}, false},
        {"Offline returns false", AgentStateInputs{Status: "Offline"}, false},
        {"unknown status returns false (default deny)", AgentStateInputs{Status: "garbage"}, false},
        {"empty status returns false (default deny)", AgentStateInputs{Status: ""}, false},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            require.Equal(t, tc.want, IsRoutable(tc.in))
        })
    }
}
```

Then create `services/api/internal/domain/state.go`:

```go
// Package domain holds pure functions with no I/O, no DB, no cache, no
// network. Consumers (state.Server, future v0.2 routing engine) load
// required scalars (e.g., break_reasons.routable) before calling.
package domain

import "github.com/luongdev/open-routing/services/api/internal/api"

type AgentStateInputs struct {
    Status              string
    BreakReasonRoutable bool
}

// IsRoutable returns true iff the agent is currently eligible to take
// new interactions: Ready (always) or Break+routable=true (STATE-10 +
// ROADMAP Phase 4 success criterion #4). String comparison against
// api.AgentStatus* constants avoids type coupling so callers can pass
// raw DB-row strings without conversion.
func IsRoutable(in AgentStateInputs) bool {
    if in.Status == string(api.AgentStatusReady) {
        return true
    }
    if in.Status == string(api.AgentStatusBreak) && in.BreakReasonRoutable {
        return true
    }
    return false
}
```

Invariants:
- ZERO imports beyond `github.com/luongdev/open-routing/services/api/internal/api` (for constants only). Adding db/cache/slog fails review.
- ZERO global state — referentially transparent.
- NO comments inside the function body.
  </action>
  <acceptance_criteria>
    - `cd services/api && go test -run TestIsRoutable -count=1 -v ./internal/domain/...` exits 0 with all subtests passing
    - `grep -q "^package domain$" services/api/internal/domain/state.go`
    - `grep -q "func IsRoutable(in AgentStateInputs) bool" services/api/internal/domain/state.go`
    - `grep -q "type AgentStateInputs struct" services/api/internal/domain/state.go`
    - state.go imports ONLY the api package: `awk '/^import/,/^)/' services/api/internal/domain/state.go | grep -c "\"github.com" | xargs -I{} test {} -eq 1`
    - state_test.go has at least 10 named subtests (verified via `grep -c '"name":\|{\s*"' state_test.go` not used; use `grep -c "^        {\"" services/api/internal/domain/state_test.go` ≥ 10)
    - `cd services/api && go vet ./internal/domain/...` exits 0
    - state.go has exactly 1 function: `grep -c "^func " services/api/internal/domain/state.go` returns 1
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go test -run TestIsRoutable -count=1 -v ./internal/domain/... && go vet ./internal/domain/... && [ "$(grep -c '^func ' /Users/luong/workspace/dev/open-solutions/services/api/internal/domain/state.go)" -eq 1 ] && [ "$(awk '/^import/,/^)/' /Users/luong/workspace/dev/open-solutions/services/api/internal/domain/state.go | grep -c '"github.com')" -eq 1 ]</automated>
  </verify>
  <done>10 IsRoutable test cases pass; pure function compiles; only `api` import.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 4: Create internal/state/transitions.go + transitions_test.go — matrix + 36-pair walk</name>
  <files>
    services/api/internal/state/transitions.go,
    services/api/internal/state/transitions_test.go
  </files>
  <read_first>
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-91 matrix shape)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (§Pattern 1 lines 282-348)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 116-126 + 389-413)
    - services/api/internal/api/types.gen.go (AgentStatus constants)
    - services/api/internal/catalog/cursor_test.go (table-driven pure unit test analog)
  </read_first>
  <behavior>
    - validateTransition(NotReady, Ready) → (TransitionRule{AgentInitiated: true}, nil)
    - validateTransition(Ready, NotReady) → (TransitionRule{AgentInitiated: true}, nil)
    - validateTransition(Ready, Break) → (TransitionRule{RequiresBreakReasonID: true, AgentInitiated: true}, nil)
    - validateTransition(Break, Ready) → (TransitionRule{AgentInitiated: true}, nil)
    - validateTransition(Break, NotReady) → (TransitionRule{AgentInitiated: true}, nil)
    - validateTransition(WrapUp, Ready) → (TransitionRule{AgentInitiated: true}, nil)
    - validateTransition(WrapUp, NotReady) → (TransitionRule{AgentInitiated: true}, nil)
    - All 7 above present; system-only edges (Ready→Engaged, Engaged→WrapUp, *→Offline, Offline→NotReady) absent and return wrapped ErrInvalidTransition.
    - Exhaustive walk: Cartesian product of {6 statuses} × {6 statuses} = 36 pairs; each pair either matches the allowed set OR returns errors.Is(err, ErrInvalidTransition).
    - The wrapped error message contains `from=` and `to=` (for debug logs); handler-level 409 uses observed from-status from DB, not the message string.
  </behavior>
  <action>
Create `services/api/internal/state/transitions.go`:

```go
// Package state holds the agent state-machine implementation
// (STATE-01..STATE-10).
//
// Pitfall 1: the exported handler type is `Server` (NOT `Handlers`)
// because *catalog.Handlers + *state.Handlers fail to compile when
// anonymous-embedded into the Wave 4 ApiHandlers composite (Go rejects
// "duplicate field name").
package state

import (
    "errors"
    "fmt"

    "github.com/luongdev/open-routing/services/api/internal/api"
)

// AgentStatus aliases the wire-level enum so transitions.go is the
// single source of truth for the matrix without forking a parallel type.
type AgentStatus = api.AgentStatus

// TransitionRule captures per-edge requirements (STATE-04, STATE-05).
type TransitionRule struct {
    RequiresBreakReasonID  bool
    RequiresEngagedChannel bool
    AgentInitiated         bool
}

// matrix encodes the 7 agent-initiated edges allowed by PATCH /status
// (STATE-02). System-initiated edges deferred per D-82:
//   - Ready→Engaged + Engaged→WrapUp  → v0.2 runtime engine
//   - *→Offline + Offline→NotReady    → AUTH phase login/logout events
var matrix = map[AgentStatus]map[AgentStatus]TransitionRule{
    api.AgentStatusNotReady: {
        api.AgentStatusReady: {AgentInitiated: true},
    },
    api.AgentStatusReady: {
        api.AgentStatusNotReady: {AgentInitiated: true},
        api.AgentStatusBreak:    {RequiresBreakReasonID: true, AgentInitiated: true},
    },
    api.AgentStatusBreak: {
        api.AgentStatusReady:    {AgentInitiated: true},
        api.AgentStatusNotReady: {AgentInitiated: true},
    },
    api.AgentStatusWrapUp: {
        api.AgentStatusReady:    {AgentInitiated: true},
        api.AgentStatusNotReady: {AgentInitiated: true},
    },
}

// ErrInvalidTransition is the sentinel wrapped by validateTransition for
// any pair not in the matrix. Callers translate to
// PatchAgentStatus409JSONResponse(InvalidTransitionErrorResponse{...})
// via errors.Is.
var ErrInvalidTransition = errors.New("state: invalid transition")

// validateTransition returns the per-edge rule when (from, to) is in
// the matrix, ErrInvalidTransition wrapped with from/to otherwise.
// D-84 force=true bypasses this validator — caller branches around it.
func validateTransition(from, to AgentStatus) (TransitionRule, error) {
    next, ok := matrix[from]
    if !ok {
        return TransitionRule{}, fmt.Errorf("%w: from=%s to=%s", ErrInvalidTransition, from, to)
    }
    rule, ok := next[to]
    if !ok {
        return TransitionRule{}, fmt.Errorf("%w: from=%s to=%s", ErrInvalidTransition, from, to)
    }
    return rule, nil
}

// allStatuses returns every defined AgentStatus for the exhaustive test
// walk. Package-internal — production code never iterates the whole enum.
func allStatuses() []AgentStatus {
    return []AgentStatus{
        api.AgentStatusNotReady,
        api.AgentStatusReady,
        api.AgentStatusBreak,
        api.AgentStatusEngaged,
        api.AgentStatusWrapUp,
        api.AgentStatusOffline,
    }
}
```

Then create `services/api/internal/state/transitions_test.go`:

```go
package state

import (
    "errors"
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/luongdev/open-routing/services/api/internal/api"
)

func TestTransitionMatrix_Exhaustive(t *testing.T) {
    allowed := map[[2]api.AgentStatus]TransitionRule{
        {api.AgentStatusNotReady, api.AgentStatusReady}: {AgentInitiated: true},
        {api.AgentStatusReady, api.AgentStatusNotReady}: {AgentInitiated: true},
        {api.AgentStatusReady, api.AgentStatusBreak}:    {RequiresBreakReasonID: true, AgentInitiated: true},
        {api.AgentStatusBreak, api.AgentStatusReady}:    {AgentInitiated: true},
        {api.AgentStatusBreak, api.AgentStatusNotReady}: {AgentInitiated: true},
        {api.AgentStatusWrapUp, api.AgentStatusReady}:   {AgentInitiated: true},
        {api.AgentStatusWrapUp, api.AgentStatusNotReady}:{AgentInitiated: true},
    }
    require.Len(t, allowed, 7, "matrix must encode exactly 7 agent-initiated edges (STATE-02)")

    statuses := allStatuses()
    require.Len(t, statuses, 6)

    for _, from := range statuses {
        for _, to := range statuses {
            rule, err := validateTransition(from, to)
            if wantRule, ok := allowed[[2]api.AgentStatus{from, to}]; ok {
                require.NoError(t, err, "expected matrix to allow %s→%s", from, to)
                require.Equal(t, wantRule, rule, "rule mismatch for %s→%s", from, to)
            } else {
                require.ErrorIs(t, err, ErrInvalidTransition, "expected matrix to reject %s→%s", from, to)
            }
        }
    }
}

func TestMatrix_SystemOnlyTransitionsAbsent(t *testing.T) {
    systemOnly := [][2]api.AgentStatus{
        {api.AgentStatusReady, api.AgentStatusEngaged},
        {api.AgentStatusEngaged, api.AgentStatusWrapUp},
        {api.AgentStatusReady, api.AgentStatusOffline},
        {api.AgentStatusBreak, api.AgentStatusOffline},
        {api.AgentStatusEngaged, api.AgentStatusOffline},
        {api.AgentStatusWrapUp, api.AgentStatusOffline},
        {api.AgentStatusOffline, api.AgentStatusNotReady},
    }
    for _, pair := range systemOnly {
        _, err := validateTransition(pair[0], pair[1])
        require.True(t, errors.Is(err, ErrInvalidTransition),
            "system-only %s→%s must NOT be in v0.1 matrix (D-82)", pair[0], pair[1])
    }
}

func TestMatrix_EngagedDeferred(t *testing.T) {
    // STATE-05 schema readiness: Ready→Engaged is system-only (v0.2).
    _, err := validateTransition(api.AgentStatusReady, api.AgentStatusEngaged)
    require.ErrorIs(t, err, ErrInvalidTransition)
}
```

Hazards:
- The Ready→Break rule MUST set BOTH `RequiresBreakReasonID: true` AND `AgentInitiated: true`. Missing either breaks STATE-04 handler logic in Wave 2.
- `errors.Is(err, ErrInvalidTransition)` MUST work — use `fmt.Errorf("%w: ...", ErrInvalidTransition, ...)` with `%w` verb.
- Do NOT export `matrix`. Tests reference it via package-internal access.
- The Engaged status appears in allStatuses() but NOT as a matrix outer key — `matrix[Engaged]` returns nil and validateTransition rejects via the `next, ok := matrix[from]` branch.
  </action>
  <acceptance_criteria>
    - `grep -q "^package state$" services/api/internal/state/transitions.go`
    - `grep -q "var matrix = map\[AgentStatus\]map\[AgentStatus\]TransitionRule" services/api/internal/state/transitions.go`
    - `grep -q "var ErrInvalidTransition = errors.New" services/api/internal/state/transitions.go`
    - `grep -q "func validateTransition(from, to AgentStatus) (TransitionRule, error)" services/api/internal/state/transitions.go`
    - `grep -q "RequiresBreakReasonID: true" services/api/internal/state/transitions.go`
    - `grep -q "type AgentStatus = api.AgentStatus" services/api/internal/state/transitions.go`
    - The matrix outer keys are exactly {NotReady, Ready, Break, WrapUp} — count = 4 (verified by counting `api.AgentStatus` keys inside `var matrix = map[AgentStatus]...{` block)
    - `cd services/api && go test -run TestTransitionMatrix_Exhaustive -count=1 -v ./internal/state/...` exits 0 with 36 implicit subtests
    - `cd services/api && go test -run TestMatrix_SystemOnlyTransitionsAbsent -count=1 ./internal/state/...` exits 0
    - `cd services/api && go test -run TestMatrix_EngagedDeferred -count=1 ./internal/state/...` exits 0
    - `cd services/api && go vet ./internal/state/...` exits 0
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go test -count=1 -v ./internal/state/... -run "TestTransitionMatrix_Exhaustive|TestMatrix_SystemOnlyTransitionsAbsent|TestMatrix_EngagedDeferred" && go vet ./internal/state/...</automated>
  </verify>
  <done>Matrix encodes exactly 7 edges; 36-pair walk passes; ErrInvalidTransition sentinel wraps correctly.</done>
</task>

<task type="auto">
  <name>Task 5: Create internal/state/handlers.go — state.Server skeleton + Deps + New + Options (Pitfall 1 rename)</name>
  <files>services/api/internal/state/handlers.go</files>
  <read_first>
    - services/api/internal/catalog/handlers.go (D-71 hybrid constructor lines 22-118)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-88 + D-95 lifecycle)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (§Pattern 6 lines 531-651)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 40-112)
    - services/api/go.mod (confirm clockwork is available post Wave 0)
    - services/api/internal/cache/cache.go (Key helper signature)
    - services/api/internal/db/orgdb.go (OrgDB type)
  </read_first>
  <action>
Create `services/api/internal/state/handlers.go`:

```go
package state

import (
    "context"
    "log/slog"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/jonboulle/clockwork"

    "github.com/luongdev/open-routing/services/api/internal/cache"
    "github.com/luongdev/open-routing/services/api/internal/db"
)

const (
    defaultSweepInterval = 30 * time.Second // D-81 safety sweep cadence
    defaultWrapUpDur     = 60 * time.Second // v0.1 hardcoded; per-org override deferred
    stateCacheTTL        = 60 * time.Second // D-86 matches catalog cache TTL
    jitterMaxMs          = 100              // D-87 ±100ms thundering-herd guard
)

// Deps bundles required runtime dependencies. Every field REQUIRED;
// New does NOT validate non-nil — a zero field panics at first
// dereference (D-71 hybrid-constructor contract).
type Deps struct {
    OrgDB          *db.OrgDB
    Cache          *cache.Cache
    Logger         *slog.Logger
    WrapUpDuration time.Duration // 0 → defaultWrapUpDur
}

type Option func(*Server)

// WithClock injects a clockwork.Clock implementation. Tests use
// clockwork.NewFakeClock() so AfterFunc fires deterministically on
// fakeClock.Advance(d).
func WithClock(c clockwork.Clock) Option { return func(s *Server) { s.clock = c } }

// WithSweepInterval overrides the 30s safety-sweep cadence for tests.
func WithSweepInterval(d time.Duration) Option { return func(s *Server) { s.sweepInterval = d } }

// Server implements the SUBSET of api.StrictServerInterface dealing with
// agent-state operations. The full interface is satisfied by the
// *ApiHandlers composite in cmd/api/main.go (D-89 — Wave 4 wires this).
//
// Pitfall 1: the type is `Server` (NOT `Handlers`) because Go rejects
// `type ApiHandlers struct { *catalog.Handlers; *state.Handlers }`
// with "duplicate field name." Renaming here keeps Wave 4 wiring trivial.
type Server struct {
    deps          Deps
    clock         clockwork.Clock
    sweepInterval time.Duration

    timers *timersRegistry

    ctx     context.Context
    cancel  context.CancelFunc
    wg      sync.WaitGroup
    started bool
    startMu sync.Mutex
}

// timersRegistry is the in-memory AfterFunc handle map keyed by
// agent_id. RWMutex over sync.Map per Claude's discretion #2: Stop()
// iterates the whole map; mutex makes the iteration semantics explicit.
// Wave 3 (04-04-PLAN.md) implements registry methods + sweeper.
type timersRegistry struct {
    mu sync.RWMutex
    t  map[uuid.UUID]clockwork.Timer
}

// New constructs a *Server. Required deps validate at first dereference;
// optional knobs apply via Option funcs. Defaults: real clock, 30s
// sweep interval, 60s WrapUp duration.
func New(deps Deps, opts ...Option) *Server {
    s := &Server{
        deps:          deps,
        clock:         clockwork.NewRealClock(),
        sweepInterval: defaultSweepInterval,
        timers:        &timersRegistry{t: make(map[uuid.UUID]clockwork.Timer)},
    }
    if deps.WrapUpDuration == 0 {
        s.deps.WrapUpDuration = defaultWrapUpDur
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Start is the lifecycle hook called by cmd/api/main.go before serving.
// Wave 1 stubs to no-op; Wave 3 implements synchronous startup sweep +
// safety-sweep goroutine. Idempotent via startMu + started.
func (s *Server) Start(ctx context.Context) error {
    s.startMu.Lock()
    defer s.startMu.Unlock()
    if s.started {
        return nil
    }
    s.started = true
    s.ctx, s.cancel = context.WithCancel(ctx)
    return nil
}

// Stop is called via defer in main.go shutdown path. Wave 3 cancels ctx,
// waits for sweeper drain, cancels timers. Idempotent.
func (s *Server) Stop() {
    s.startMu.Lock()
    defer s.startMu.Unlock()
    if !s.started {
        return
    }
    s.started = false
    if s.cancel != nil {
        s.cancel()
    }
}

// cacheKeyFor centralises the agent_state cache key namespace (D-86).
// Singular "agent_state" (NOT plural) per Pitfall 7. Every cache.GetOrSet
// + cache.Del in agent_states.go routes through here.
func (s *Server) cacheKeyFor(orgID, agentID uuid.UUID) string {
    return cache.Key(orgID, "agent_state", agentID)
}
```

Hazards:
- The struct name is `Server` — `grep -E "^type Handlers " services/api/internal/state/handlers.go` MUST return 0 matches.
- Start/Stop bodies are intentionally minimal — Wave 3 implements real sweeper. The `started` idempotency guard MUST be present from Wave 1.
- Do NOT add `var _ api.StrictServerInterface = (*Server)(nil)` — Server owns only 2 methods (the SUBSET). The full assertion lives on *ApiHandlers in cmd/api/main.go (Wave 4).
- Comments are WHY-bearing only (CLAUDE.md).
  </action>
  <acceptance_criteria>
    - `grep -q "^type Server struct" services/api/internal/state/handlers.go`
    - `[ "$(grep -cE '^type Handlers ' services/api/internal/state/handlers.go)" -eq 0 ]` (Pitfall 1)
    - `grep -q "^func New(deps Deps, opts ...Option) \*Server" services/api/internal/state/handlers.go`
    - `grep -q "^func WithClock(c clockwork.Clock) Option" services/api/internal/state/handlers.go`
    - `grep -q "^func WithSweepInterval(d time.Duration) Option" services/api/internal/state/handlers.go`
    - `grep -q "^func (s \*Server) Start(ctx context.Context) error" services/api/internal/state/handlers.go`
    - `grep -q "^func (s \*Server) Stop()" services/api/internal/state/handlers.go`
    - `grep -q "cacheKeyFor(orgID, agentID uuid.UUID) string" services/api/internal/state/handlers.go`
    - `grep -q "\"agent_state\"" services/api/internal/state/handlers.go` (singular)
    - `grep -q "github.com/jonboulle/clockwork" services/api/internal/state/handlers.go`
    - `cd services/api && go build ./internal/state/...` exits 0
    - `cd services/api && go vet ./internal/state/...` exits 0
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go build ./internal/state/... && go vet ./internal/state/... && grep -q "^type Server struct" /Users/luong/workspace/dev/open-solutions/services/api/internal/state/handlers.go && [ "$(grep -cE '^type Handlers ' /Users/luong/workspace/dev/open-solutions/services/api/internal/state/handlers.go)" -eq 0 ]</automated>
  </verify>
  <done>state.Server scaffold compiles; Pitfall 1 type rename complete; Start/Stop idempotency stubs in place.</done>
</task>

<task type="auto">
  <name>Task 6: Create internal/state/agent_states.go — stub GetAgentStatus + PatchAgentStatus</name>
  <files>services/api/internal/state/agent_states.go</files>
  <read_first>
    - services/api/internal/catalog/notimpl.go (existing 501 stub shape — lines 40-50)
    - services/api/internal/api/server.gen.go (GetAgentStatusRequestObject / PatchAgentStatusRequestObject types — search via grep)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-88 stub-then-real-body sequencing)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (§Pattern 6 lifecycle)
  </read_first>
  <action>
Create `services/api/internal/state/agent_states.go`:

```go
package state

import (
    "context"

    "github.com/luongdev/open-routing/services/api/internal/api"
)

// GetAgentStatus is the StrictServerInterface implementation for
// GET /v1/orgs/{org_id}/agents/{id}/status. Wave 1 ships a 501 stub
// so the state package can be wired into the composite ApiHandlers
// before Wave 2 implements real cache+DB load. Wave 2 (04-03-PLAN.md)
// replaces this body entirely.
func (s *Server) GetAgentStatus(_ context.Context, _ api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error) {
    return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
        Error:  api.ErrorCodeInternal,
        Reason: "wave_1_stub_GetAgentStatus_pending_wave_2_implementation",
    }}, nil
}

// PatchAgentStatus is the StrictServerInterface implementation for
// PATCH /v1/orgs/{org_id}/agents/{id}/status. Wave 1 ships a 501 stub
// so the state package can be wired into the composite ApiHandlers
// before Wave 2 implements: (a) cross-row break_reason probe, (b)
// transition validator OR force bypass, (c) atomic UPDATE + 0-row
// disambiguate, (d) cache.Del after commit. Wave 2 (04-03-PLAN.md)
// replaces this body entirely.
func (s *Server) PatchAgentStatus(_ context.Context, _ api.PatchAgentStatusRequestObject) (api.PatchAgentStatusResponseObject, error) {
    return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
        Error:  api.ErrorCodeInternal,
        Reason: "wave_1_stub_PatchAgentStatus_pending_wave_2_implementation",
    }}, nil
}
```

Hazards:
- The two methods MUST be on `*Server` (not `*Handlers`). Wave 4 ApiHandlers composite embeds `*state.Server` — the methods must dispatch to the correct receiver type.
- The stub body returns 500 (NOT 501) to match Phase 3's notimpl pattern — `api.GetAgentStatus500JSONResponse` is the existing response type (no 501 variant exists in the spec).
- Reason strings include "wave_1_stub" so grep finds them in Wave 2 replacement (verification that nothing missed the replacement).
- These methods exist to satisfy the StrictServerInterface SUBSET — when Wave 4 wires composite ApiHandlers, the embed brings these methods into the composite's method set. Without them, the composite fails to compile.
  </action>
  <acceptance_criteria>
    - `grep -q "^func (s \*Server) GetAgentStatus(_ context.Context, _ api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error)" services/api/internal/state/agent_states.go`
    - `grep -q "^func (s \*Server) PatchAgentStatus(_ context.Context, _ api.PatchAgentStatusRequestObject) (api.PatchAgentStatusResponseObject, error)" services/api/internal/state/agent_states.go`
    - `grep -c "wave_1_stub" services/api/internal/state/agent_states.go` ≥ 2 (one per method — Wave 2 replacement-guard)
    - `cd services/api && go build ./internal/state/...` exits 0
    - `cd services/api && go vet ./internal/state/...` exits 0
    - The full test suite still passes: `cd services/api && go test -count=1 ./internal/state/... ./internal/domain/...` exits 0 (transition matrix + IsRoutable tests still green)
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go build ./internal/state/... && go vet ./internal/state/... && go test -count=1 ./internal/state/... ./internal/domain/... && grep -c "wave_1_stub" /Users/luong/workspace/dev/open-solutions/services/api/internal/state/agent_states.go | xargs -I{} test {} -ge 2</automated>
  </verify>
  <done>Stub methods compile on *Server; package builds cleanly; Wave 2 replacement guards in place.</done>
</task>

</tasks>

<verification>
- Wave 0 outputs honored: agent_states table + tenantTables + clockwork dep all in place
- agent_states.sql has 6 named queries; sqlc regen idempotent (task gen produces no diff after commit)
- break_reasons.sql gains GetBreakReasonForState combined probe (returns bool scalar; filters enabled=TRUE for soft-delete consistency)
- domain.IsRoutable is referentially transparent (only api import; no I/O); 10 test cases pass
- state.Server (NOT Handlers) compiles with Deps + Options + lifecycle stubs (Pitfall 1)
- transitions.go matrix has exactly 7 agent-initiated edges; 36-pair walk asserts allowed vs rejected
- Stub GetAgentStatus / PatchAgentStatus return 500 with wave_1_stub reason strings (Wave 2 replacement guard)
- Zero existing test regressions: full Phase 3 suite still green
</verification>

<success_criteria>
- 6 tasks completed; 12 files modified (4 source files in domain+state, 2 SQL files, 4 regenerated Go bindings, 2 test files)
- `cd services/api && go test -count=1 ./internal/state/... ./internal/domain/... ./internal/db/...` exits 0
- `cd services/api && task gen && git diff --exit-code` is clean (codegen idempotent)
- Wave 4 has zero rename work: Pitfall 1 eliminated by naming the type `state.Server` from this wave forward
- Wave 2 has full type contracts: Server, Deps, Options, stub methods all in place
- Wave 3 has full TTL scaffold: timersRegistry struct declared (body empty), Start/Stop idempotency guards in place
</success_criteria>

<output>
Create `.planning/phases/04-agent-state-machine-go/04-02-SUMMARY.md` when done. Include: list of all files modified (with line counts), output of `go test -count=1 ./internal/state/... ./internal/domain/... ./internal/db/...`, confirmation of `task gen` idempotency, confirmation that 7 matrix edges are encoded (NOT 6, NOT 8) and 36-pair walk passes.
</output>
---
phase: 04-agent-state-machine-go
plan: 03
type: execute
wave: 2
depends_on:
  - 04-02
files_modified:
  - services/api/internal/state/agent_states.go
  - services/api/internal/state/agent_states_test.go
  - services/api/internal/state/testutil_test.go
  - services/api/internal/state/main_test.go
  - services/api/internal/state/mappers.go
  - services/api/internal/server/server.go
  - services/api/internal/server/request_id_exhaustiveness_test.go
autonomous: true
requirements:
  - STATE-02
  - STATE-03
  - STATE-04
  - STATE-06
  - STATE-08

must_haves:
  truths:
    - "GetAgentStatus serves from cache.GetOrSet[api.AgentState] with 60s TTL; 404 when row absent (STATE-01 read-side)"
    - "PatchAgentStatus runs the break_reason probe BEFORE the transition matrix (Pitfall 3 — probe-then-matrix order)"
    - "PatchAgentStatus rejects invalid transitions with 409 InvalidTransitionErrorResponse carrying observed from + requested to (STATE-03)"
    - "PatchAgentStatus with cross-org/missing break_reason_id returns 422 invalid_reference even when force=true is set (Pitfall 3)"
    - "PatchAgentStatus with force=true bypasses transition matrix and logs slog WARN 'state.force.applied' (D-84)"
    - "Every successful PATCH increments state_version monotonically (STATE-08)"
    - "Cache.Del fires after commit on every PATCH (D-55) including the 409 disambiguate path (D-56); cache.Del failure logs warn, never 5xx (D-55 spirit)"
  artifacts:
    - path: "services/api/internal/state/agent_states.go"
      provides: "Real GetAgentStatus + PatchAgentStatus handler bodies (wave_1_stub removed)"
      contains: "cache.GetOrSet"
    - path: "services/api/internal/state/agent_states_test.go"
      provides: "Per-entity tests: STATE-02 happy path, STATE-03 409 with from/to, STATE-04 422 same-org probe, STATE-06 post_interaction_state, STATE-08 state_version monotonic, force=true bypass"
      contains: "TestPatchAgentStatus_BreakReasonProbe"
    - path: "services/api/internal/state/testutil_test.go"
      provides: "Shared fixtures mirroring catalog/testutil_test.go (D-73): newTestHandlers, seedAgent, seedBreakReason, seedAgentStateRow, httpPATCHStatus"
      contains: "func newTestHandlers"
    - path: "services/api/internal/state/main_test.go"
      provides: "TestMain bring-up — testcontainer postgres + migrations + sharedPool"
      contains: "func TestMain"
    - path: "services/api/internal/state/mappers.go"
      provides: "Row→DTO mapper: mapAgentState(generated.AgentState) api.AgentState"
      contains: "func mapAgentState"
    - path: "services/api/internal/server/server.go"
      provides: "Extended injectRequestIDIntoErrorResponse type switch covering PatchAgentStatus409JSONResponse (if missing)"
      contains: "PatchAgentStatus409JSONResponse"
  key_links:
    - from: "services/api/internal/state/agent_states.go"
      to: "services/api/internal/db/generated/agent_states.sql.go"
      via: "qtx.UpdateAgentStateStatus / qtx.GetAgentStateByAgentId / qtx.ForceUpdateAgentStateStatus"
      pattern: "qtx\\.(UpdateAgentStateStatus|GetAgentStateByAgentId|ForceUpdateAgentStateStatus)"
    - from: "services/api/internal/state/agent_states.go"
      to: "services/api/internal/db/generated/break_reasons.sql.go"
      via: "GetBreakReasonForState combined probe (D-76)"
      pattern: "GetBreakReasonForState"
    - from: "services/api/internal/state/agent_states.go"
      to: "services/api/internal/cache/cache.go"
      via: "cache.GetOrSet[api.AgentState] read-through + cache.Del after commit"
      pattern: "cache\\.(GetOrSet|Del)"
---

<objective>
Implement the real `GetAgentStatus` and `PatchAgentStatus` handler bodies in `state.Server` — replacing Wave 1's `wave_1_stub` returns with: (a) cache-read-through GET via `cache.GetOrSet[api.AgentState]` keyed `or:{orgId}:agent_state:{agent_id}` with 60s TTL (D-86); (b) PATCH with probe-then-matrix ordering (Pitfall 3): cross-row `break_reason_id` probe runs FIRST (D-76 + D-84 — force does NOT bypass cross-org probes); then transition matrix validates UNLESS `force=true` (D-84); atomic UPDATE with `expected_from = currentObservedStatus` (D-85 + D-66); 0-row → SELECT disambiguate → 404 vs 409 invalid_transition; commit then `cache.Del`; force=true emits slog WARN `state.force.applied` with audit attrs. Also lays the testutil + main_test scaffolding so the per-entity tests have shared fixtures parity with Phase 3.

Purpose: This is where Phase 4 starts to deliver user-visible value. After this wave, an agent can call PATCH /status to move between Ready/NotReady/Break (with break_reason), and the server enforces the transition matrix + cross-org break_reason isolation + version monotonicity. WrapUp TTL is NOT yet wired (Wave 3) and ApiHandlers composite is NOT yet wired in main.go (Wave 4) — Wave 2 verifies via httptest with the standalone state.Server mounted.

Output: Real handler bodies; passing per-entity test suite (STATE-02/03/04/06/08 + force=true bypass); testutil parity with catalog; mappers.go for row→DTO conversion; injectRequestIDIntoErrorResponse type switch updated if PatchAgentStatus409JSONResponse is missing (Pitfall 5).

**Hazard captured here (Pitfall 3 — Hazard 4 in PATTERNS.md):** force=true MUST bypass ONLY the transition matrix — the cross-row break_reason probe MUST still run. This is encoded in the handler control flow: probe FIRST, then matrix-or-bypass-matrix, then UPDATE. The cross-org isolation test in Wave 5 (`TestState_ForceDoesNotBypassCrossOrgProbe_422`) is the regression gate; this wave's `TestPatchAgentStatus_ForceWithMissingBreakReason_422` is the local gate.
</objective>

<execution_context>
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/workflows/execute-plan.md
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@./CLAUDE.md
@.planning/phases/04-agent-state-machine-go/04-CONTEXT.md
@.planning/phases/04-agent-state-machine-go/04-RESEARCH.md
@.planning/phases/04-agent-state-machine-go/04-PATTERNS.md
@.planning/phases/04-agent-state-machine-go/04-VALIDATION.md
@.planning/phases/04-agent-state-machine-go/04-02-PLAN.md
@.planning/phases/04-agent-state-machine-go/04-02-SUMMARY.md
@services/api/internal/catalog/agents.go
@services/api/internal/catalog/channels.go
@services/api/internal/catalog/testutil_test.go
@services/api/internal/catalog/main_test.go
@services/api/internal/catalog/agents_test.go
@services/api/internal/catalog/mappers.go
@services/api/internal/cache/cache.go
@services/api/internal/db/orgdb.go
@services/api/internal/db/bypass.go
@services/api/internal/db/orgkey
@services/api/internal/server/server.go
</context>

<interfaces>
From `services/api/internal/api/server.gen.go`:
```go
type GetAgentStatusRequestObject struct {
    OrgId openapi_types.UUID `json:"org_id"`
    Id    openapi_types.UUID `json:"id"`
}
type GetAgentStatusResponseObject interface { /* sealed */ }
type GetAgentStatus200JSONResponse api.AgentState
type GetAgentStatus404JSONResponse struct { NotFoundJSONResponse }
type GetAgentStatus500JSONResponse struct { InternalServerErrorJSONResponse }

type PatchAgentStatusRequestObject struct {
    OrgId openapi_types.UUID
    Id    openapi_types.UUID
    Body  *PatchAgentStatusJSONRequestBody
}
type PatchAgentStatus200JSONResponse api.AgentState
type PatchAgentStatus404JSONResponse struct { NotFoundJSONResponse }
type PatchAgentStatus409JSONResponse InvalidTransitionErrorResponse  // {from, to, error, request_id?}
type PatchAgentStatus422JSONResponse api.ErrorResponse                // {error, reason, request_id?}
type PatchAgentStatus400JSONResponse struct { BadRequestJSONResponse }
type PatchAgentStatus500JSONResponse struct { InternalServerErrorJSONResponse }
```

From `services/api/internal/api/types.gen.go`:
```go
type AgentState struct {
    AgentId              openapi_types.UUID    `json:"agent_id"`
    Status               AgentStatus           `json:"status"`
    EngagedChannel       *EngagedChannel       `json:"engaged_channel,omitempty"`
    BreakReasonId        *openapi_types.UUID   `json:"break_reason_id,omitempty"`
    PostInteractionState *PostInteractionState `json:"post_interaction_state,omitempty"`
    WrapupUntil          *time.Time            `json:"wrapup_until,omitempty"`
    StateVersion         int64                 `json:"state_version"`
    UpdatedAt            time.Time             `json:"updated_at"`
}

type InvalidTransitionErrorResponse struct {
    Error     ErrorCode           // ErrorCodeInvalidTransition
    From      AgentStatus
    To        AgentStatus
    RequestId *openapi_types.UUID
}
```

From `services/api/internal/cache/cache.go`:
```go
func GetOrSet[T any](ctx, c *Cache, key string, ttl time.Duration, loader func(ctx) (T, error)) (T, error)
func (c *Cache) Del(ctx, key string) error
func Key(orgID uuid.UUID, entity string, id uuid.UUID) string  // returns "or:{orgId}:{entity}:{id}"
var ErrNotFound = errors.New(...)  // loader returns this → cache.GetOrSet returns it; handlers map to 404
```

From `services/api/internal/db/orgkey`:
```go
func OrgIDFromContext(ctx context.Context) (uuid.UUID, bool)
```

From `services/api/internal/db/orgdb.go`:
```go
func (o *OrgDB) BeginTx(ctx context.Context) (*OrgTx, error)
type OrgTx struct { ... }  // implements DBTX
func (t *OrgTx) Commit(ctx) error
func (t *OrgTx) Rollback(ctx) error
```
</interfaces>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| HTTP client → PATCH /status with `force=true` | Stub auth lets any caller pass it; WARN slog audits every invocation per D-84. |
| HTTP client → PATCH /status with cross-org break_reason_id | Probe MUST run regardless of `force` (Pitfall 3). |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-04-06 | Information Disclosure | Cross-org break_reason probe leak | mitigate | `GetBreakReasonForState` filters `org_id = $2`; cross-org → ErrNoRows → 422. Probe runs FIRST in handler control flow (Pitfall 3 — verified by ordering test). |
| T-04-07 | Tampering | force=true unaudited admin override | mitigate | Every force=true invocation emits `slog.WarnContext(ctx, "state.force.applied", "agent_id", ..., "from", ..., "to", ..., "org_id", ...)`. v1 AUTH phase restricts to org_admin role. |
| T-04-08 | Tampering | state_version regression on concurrent PATCH | mitigate | UPDATE WHERE clause includes `status = expected_from` — concurrent PATCH that already shifted observes 0 rows → 409 invalid_transition. SQL increments `state_version + 1` atomically. |
| T-04-09 | Denial of Service | Cache.Del failure cascading to 5xx | mitigate | cache.Del failure logs WARN; HTTP response remains 200. Cache invalidation is eventual via TTL (60s) on the unlikely failure path. |
</threat_model>

<tasks>

<task type="auto">
  <name>Task 1: Create internal/state/main_test.go + testutil_test.go + mappers.go scaffolding</name>
  <files>
    services/api/internal/state/main_test.go,
    services/api/internal/state/testutil_test.go,
    services/api/internal/state/mappers.go
  </files>
  <read_first>
    - services/api/internal/catalog/main_test.go (verbatim template — D-73 testcontainer bring-up)
    - services/api/internal/catalog/testutil_test.go (verbatim template — newTestHandlers + helpers)
    - services/api/internal/catalog/mappers.go (mapAgent / mapAgentSkill patterns)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 452-516 — testutil deviations)
    - services/api/internal/db/generated/models.go (AgentState row type — generated in Wave 1)
  </read_first>
  <action>
Create `services/api/internal/state/main_test.go` by copying `services/api/internal/catalog/main_test.go` verbatim and ONLY changing the package declaration from `package catalog` to `package state`. The TestMain spins up the testcontainer postgres:17, runs migrations from `../../../../migrations`, opens a pgxpool, and assigns it to a package-level `sharedPool *pgxpool.Pool` variable. The body is identical to catalog/main_test.go — same migrations path, same testcontainer image, same defer cleanup. Do NOT change anything except the package line.

Create `services/api/internal/state/testutil_test.go` mirroring `services/api/internal/catalog/testutil_test.go` with these specific adaptations:

1. **Package**: `package state`.
2. **TestHandlers struct**:
   ```go
   type TestHandlers struct {
       S         *Server  // state.Server (NOT *Handlers)
       Miniredis *miniredis.Miniredis
       Pool      *pgxpool.Pool
       OrgID     uuid.UUID
       HTTP      *httptest.Server
       rdb       *redis.Client
   }
   ```
3. **newTestHandlers(t)** constructor:
   - Skip test if `sharedPool == nil` (no docker testcontainer).
   - Spin up miniredis.RunT(t); construct redis.Client; t.Cleanup(_ = rdb.Close()).
   - Build a no-op slog.Logger (io.Discard).
   - Build cache.New(rdb, logger).
   - Build orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic).
   - Build the state.Server: `s := state.New(state.Deps{OrgDB: orgDB, Cache: c, Logger: logger}, state.WithSweepInterval(100*time.Millisecond))`.
   - **Wire as composite**: since this wave does NOT yet have the ApiHandlers composite from Wave 4, mount the state.Server's two methods into an httptest server by constructing a chi mux that routes ONLY the two state endpoints to a NewStrictHandler over a minimal facade. Pattern: declare a local `stateOnlyHandlers struct { *state.Server }` inside testutil_test.go that also embeds `notImplFor4` stubs for the rest of the StrictServerInterface (use catalog's notimpl.go panic pattern). This is purely test scaffolding — Wave 4 production wiring uses real ApiHandlers.
   - Construct mux via `server.NewMux(&server.Deps{Pool: sharedPool, Redis: rdb, OrgDB: orgDB, Config: testConfig(), StrictHandlers: stateOnlyHandlers, SpecBytes: testSpec()})`.
   - Construct httptest.NewServer(mux); t.Cleanup(srv.Close).
   - Generate orgID := uuid.Must(uuid.NewV7()).
   - Return *TestHandlers.

4. **Per-test helper functions** (verbatim shape from catalog/testutil_test.go with state-specific path builders):
   ```go
   func agentStatusPath(orgID, agentID uuid.UUID) string {
       return "/v1/orgs/" + orgID.String() + "/agents/" + agentID.String() + "/status"
   }

   func httpPATCHStatus(t testing.TB, th *TestHandlers, agentID uuid.UUID, body any) (*http.Response, []byte) {
       return httpPATCH(t, th.HTTP, th.OrgID, agentStatusPath(th.OrgID, agentID), body)
   }
   func httpGETStatus(t testing.TB, th *TestHandlers, agentID uuid.UUID) (*http.Response, []byte) {
       return httpGET(t, th.HTTP, th.OrgID, agentStatusPath(th.OrgID, agentID))
   }
   ```
   Reuse the existing `httpPATCH` / `httpGET` / `httpPOST` helpers from catalog/testutil_test.go shape (DON'T import — copy verbatim into state/testutil_test.go since `_test.go` files don't share package).

5. **Seed helpers** (Claude discretion #6 — raw INSERT preferred):
   ```go
   // seedAgent inserts an agents row via direct INSERT (bypasses CreateAgent
   // handler — keeps Wave 2 tests independent of Wave 4 composite wiring).
   func seedAgent(t testing.TB, pool *pgxpool.Pool, orgID, agentID uuid.UUID, externalID, name string) { ... }

   // seedBreakReason inserts a break_reasons row directly. routable parameter
   // controls the STATE-10 IsRoutable input.
   func seedBreakReason(t testing.TB, pool *pgxpool.Pool, orgID, id uuid.UUID, name string, routable bool) { ... }

   // seedAgentStateRow inserts an agent_states row directly. Bypasses the
   // transition matrix so tests for Engaged/WrapUp states can set up their
   // fixtures without orchestrating through PATCH (D-82 — system-initiated
   // states have no v0.1 caller). RESEARCH §Testutil Deviations.
   func seedAgentStateRow(t testing.TB, pool *pgxpool.Pool, params SeedStateParams) { ... }

   type SeedStateParams struct {
       AgentID              uuid.UUID
       OrgID                uuid.UUID
       Status               string
       EngagedChannel       *string
       BreakReasonID        *uuid.UUID
       PostInteractionState *string
       WrapupUntil          *time.Time
       StateVersion         int64
   }
   ```
   Implement each helper using `generated.New(pool).<InsertX>(ctx, params)`. Surface errors via `require.NoError(t, err)`.

6. **TRUNCATE helper extension**:
   ```go
   // cleanStateTables truncates agent_states + the catalog tables this test
   // touches. Run at start of each test that mutates DB state.
   func cleanStateTables(t testing.TB, ctx context.Context, pool *pgxpool.Pool) {
       _, err := pool.Exec(ctx, `TRUNCATE TABLE agent_states, agents, break_reasons CASCADE`)
       require.NoError(t, err)
   }
   ```

Create `services/api/internal/state/mappers.go` (production code — NOT a _test.go file):

```go
package state

import (
    "time"

    openapi_types "github.com/oapi-codegen/runtime/types"

    "github.com/luongdev/open-routing/services/api/internal/api"
    "github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// mapAgentState converts a sqlc-generated agent_states row to the wire
// DTO. Nullable columns surface as *T per OpenAPI nullable semantics.
// Caller is responsible for orgID scoping (the row already carries the
// right org_id from the WHERE clause).
func mapAgentState(row generated.AgentState) api.AgentState {
    out := api.AgentState{
        AgentId:      openapi_types.UUID(row.AgentID.Bytes),
        Status:       api.AgentStatus(row.Status),
        StateVersion: row.StateVersion,
        UpdatedAt:    row.UpdatedAt.Time,
    }
    if row.EngagedChannel.Valid {
        ec := api.EngagedChannel(row.EngagedChannel.String)
        out.EngagedChannel = &ec
    }
    if row.BreakReasonID.Valid {
        br := openapi_types.UUID(row.BreakReasonID.Bytes)
        out.BreakReasonId = &br
    }
    if row.PostInteractionState.Valid {
        pis := api.PostInteractionState(row.PostInteractionState.String)
        out.PostInteractionState = &pis
    }
    if row.WrapupUntil.Valid {
        wu := row.WrapupUntil.Time
        out.WrapupUntil = &wu
    }
    return out
}

// pgUUID wraps uuid.UUID into pgtype.UUID for sqlc params. Mirrors the
// catalog helper of the same name.
func pgUUID(id [16]byte) pgtype.UUID {
    return pgtype.UUID{Bytes: id, Valid: true}
}
```

Note: if `pgtype` import is needed, add it. Verify the exact pgtype shape against `services/api/internal/catalog/mappers.go` — copy the same `pgUUID` helper signature.

Hazards:
- `seedAgentStateRow` MUST use raw INSERT (`generated.New(pool).InsertAgentState` or a direct `pool.Exec` if InsertAgentState's signature is too restrictive for non-Offline states). For Engaged/WrapUp fixtures, use direct `pool.Exec(ctx, "INSERT INTO agent_states (...) VALUES (...)")` so the matrix is bypassed.
- The `cleanStateTables` helper TRUNCATEs `agent_states FIRST` then the catalog tables — CASCADE handles dependencies. But D-80 guarantees no FK cascades — the order is documentation-only.
- The httptest mux wiring is TEMPORARY scaffolding: Wave 4 replaces it with the production ApiHandlers composite. Mark the local `stateOnlyHandlers` type with a comment `// TEMP: Wave 2 test-only composite; Wave 4 replaces with cmd/api/main.go ApiHandlers.`
  </action>
  <acceptance_criteria>
    - `grep -q "^package state$" services/api/internal/state/main_test.go`
    - `grep -q "func TestMain" services/api/internal/state/main_test.go`
    - `grep -q "var sharedPool \*pgxpool.Pool" services/api/internal/state/main_test.go`
    - `grep -q "^package state$" services/api/internal/state/testutil_test.go`
    - `grep -q "type TestHandlers struct" services/api/internal/state/testutil_test.go`
    - `grep -q "func newTestHandlers" services/api/internal/state/testutil_test.go`
    - `grep -q "func seedAgent(" services/api/internal/state/testutil_test.go`
    - `grep -q "func seedBreakReason(" services/api/internal/state/testutil_test.go`
    - `grep -q "func seedAgentStateRow(" services/api/internal/state/testutil_test.go`
    - `grep -q "func httpPATCHStatus(" services/api/internal/state/testutil_test.go`
    - `grep -q "func cleanStateTables(" services/api/internal/state/testutil_test.go`
    - `grep -q "TRUNCATE TABLE agent_states" services/api/internal/state/testutil_test.go`
    - `grep -q "^package state$" services/api/internal/state/mappers.go` (NOT a _test.go file)
    - `grep -q "func mapAgentState(row generated.AgentState) api.AgentState" services/api/internal/state/mappers.go`
    - `cd services/api && go build ./internal/state/...` exits 0
    - `cd services/api && go vet ./internal/state/...` exits 0
    - testutil_test.go contains TEMP marker for the test-only composite: `grep -q "TEMP: Wave 2 test-only" services/api/internal/state/testutil_test.go`
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go build ./internal/state/... && go vet ./internal/state/... && grep -q "func seedAgentStateRow" /Users/luong/workspace/dev/open-solutions/services/api/internal/state/testutil_test.go && grep -q "func mapAgentState" /Users/luong/workspace/dev/open-solutions/services/api/internal/state/mappers.go</automated>
  </verify>
  <done>Scaffolding compiles; testcontainer bring-up parity with catalog; seedAgentStateRow can bypass matrix for Engaged/WrapUp test fixtures; TEMP-marked composite for Wave 4 to replace.</done>
</task>

<task type="auto">
  <name>Task 2: Replace agent_states.go stubs with real GetAgentStatus body (cache-read-through)</name>
  <files>services/api/internal/state/agent_states.go</files>
  <read_first>
    - services/api/internal/catalog/agents.go (GetAgent pattern lines 175-234 — cache.GetOrSet + ErrNotFound mapping)
    - services/api/internal/state/mappers.go (just-created mapAgentState helper)
    - services/api/internal/cache/cache.go (GetOrSet signature + ErrNotFound)
    - services/api/internal/db/generated/agent_states.sql.go (GetAgentStateByAgentId)
    - services/api/internal/db/orgkey (OrgIDFromContext)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 208-247)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-86 cache key + D-58 singular)
  </read_first>
  <action>
Open `services/api/internal/state/agent_states.go`. Delete the `wave_1_stub` body of `GetAgentStatus`. Replace with the production body below. Keep the `wave_1_stub` body of `PatchAgentStatus` UNCHANGED in this task — Task 3 replaces it.

```go
package state

import (
    "context"
    "errors"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"

    "github.com/luongdev/open-routing/services/api/internal/api"
    "github.com/luongdev/open-routing/services/api/internal/cache"
    "github.com/luongdev/open-routing/services/api/internal/db/generated"
    "github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// GetAgentStatus serves the agent_states row through cache.GetOrSet with
// 60s TTL keyed `or:{orgId}:agent_state:{agent_id}` (D-86). Cache miss
// hits the DB via generated.New(orgDB).GetAgentStateByAgentId; ErrNoRows
// surfaces as cache.ErrNotFound → 404 (STATE-01 read-side).
func (s *Server) GetAgentStatus(ctx context.Context, req api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error:  api.ErrorCodeInternal,
            Reason: "missing_org_id_in_context",
        }}, nil
    }
    agentID := uuid.UUID(req.Id)
    key := s.cacheKeyFor(orgID, agentID)

    state, err := cache.GetOrSet[api.AgentState](ctx, s.deps.Cache, key, stateCacheTTL,
        func(ctx context.Context) (api.AgentState, error) {
            q := generated.New(s.deps.OrgDB)
            row, ferr := q.GetAgentStateByAgentId(ctx, generated.GetAgentStateByAgentIdParams{
                AgentID: pgUUID(agentID),
                OrgID:   pgUUID(orgID),
            })
            if errors.Is(ferr, pgx.ErrNoRows) {
                return api.AgentState{}, cache.ErrNotFound
            }
            if ferr != nil {
                return api.AgentState{}, fmt.Errorf("get agent state: %w", ferr)
            }
            return mapAgentState(row), nil
        })

    switch {
    case errors.Is(err, cache.ErrNotFound):
        return api.GetAgentStatus404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
            Error:  api.ErrorCodeNotFound,
            Reason: "agent_state_not_found",
        }}, nil
    case err != nil:
        s.deps.Logger.ErrorContext(ctx, "get agent status", "agent_id", agentID, "org_id", orgID, "err", err)
        return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error:  api.ErrorCodeInternal,
            Reason: "agent_state_load_failed",
        }}, nil
    }
    return api.GetAgentStatus200JSONResponse(state), nil
}
```

Hazards:
- The cache key uses `s.cacheKeyFor(orgID, agentID)` — routing through the centralised helper from Wave 1 handlers.go (Pitfall 7 — singular "agent_state").
- `pgUUID(agentID)` — same helper used in mappers.go. If the catalog package's `pgUUID` is used at receive-side, this package needs its own copy (Go does not export `_test.go`-bound helpers); pgUUID is now in `services/api/internal/state/mappers.go` from Task 1.
- The loader returns `(api.AgentState, error)` — NOT the sqlc row type. D-49 keeps cache layer typed on the DTO so cache de-serialization doesn't drift on row-shape changes.
- `cache.ErrNotFound` is the contract — DO NOT return raw `pgx.ErrNoRows` from the loader; the cache layer treats `cache.ErrNotFound` as a "negative cache" entry (cached for shorter TTL per D-53).
- Logging includes both `agent_id` and `org_id` for cross-org leak forensics — never just `agent_id`.
  </action>
  <acceptance_criteria>
    - `grep -q "func (s \*Server) GetAgentStatus(ctx context.Context, req api.GetAgentStatusRequestObject)" services/api/internal/state/agent_states.go`
    - `grep -q "cache.GetOrSet\[api.AgentState\]" services/api/internal/state/agent_states.go`
    - `grep -q "s.cacheKeyFor(orgID, agentID)" services/api/internal/state/agent_states.go`
    - `grep -q "stateCacheTTL" services/api/internal/state/agent_states.go`
    - `grep -q "GetAgentStateByAgentId" services/api/internal/state/agent_states.go`
    - `grep -q "cache.ErrNotFound" services/api/internal/state/agent_states.go`
    - The wave_1_stub for GetAgentStatus is GONE: `awk '/func \(s \*Server\) GetAgentStatus/,/^}/' services/api/internal/state/agent_states.go | grep -c "wave_1_stub"` returns 0
    - The wave_1_stub for PatchAgentStatus REMAINS (Task 3 replaces): `awk '/func \(s \*Server\) PatchAgentStatus/,/^}/' services/api/internal/state/agent_states.go | grep -c "wave_1_stub"` returns 1
    - `cd services/api && go build ./internal/state/...` exits 0
    - `cd services/api && go vet ./internal/state/...` exits 0
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go build ./internal/state/... && go vet ./internal/state/... && grep -q "cache.GetOrSet\[api.AgentState\]" /Users/luong/workspace/dev/open-solutions/services/api/internal/state/agent_states.go && [ "$(awk '/func \(s \*Server\) GetAgentStatus/,/^}/' /Users/luong/workspace/dev/open-solutions/services/api/internal/state/agent_states.go | grep -c wave_1_stub)" -eq 0 ]</automated>
  </verify>
  <done>GetAgentStatus production body in place; cache-read-through working; stub removed; PatchAgentStatus stub still present for Task 3.</done>
</task>

<task type="auto">
  <name>Task 3: Replace PatchAgentStatus stub with real body (probe-then-matrix + D-66 disambiguate + force WARN + state_version monotonic)</name>
  <files>services/api/internal/state/agent_states.go</files>
  <read_first>
    - services/api/internal/catalog/agents.go (UpdateAgent lines 354-493 — D-66 atomic UPDATE + 0-row disambiguate + tx pattern)
    - services/api/internal/catalog/channels.go (CreateChannel lines 38-110 — D-76 cross-row probe pattern)
    - services/api/internal/state/transitions.go (matrix + validateTransition + ErrInvalidTransition from Wave 1)
    - services/api/internal/db/generated/agent_states.sql.go (UpdateAgentStateStatus + ForceUpdateAgentStateStatus from Wave 1)
    - services/api/internal/db/generated/break_reasons.sql.go (GetBreakReasonForState from Wave 1)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 249-342 — handler shape; lines 1100-1180 — shared patterns)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-84 force + D-85 no version + D-66 disambiguate + Pitfall 3)
  </read_first>
  <action>
Open `services/api/internal/state/agent_states.go`. Delete the entire wave_1_stub `PatchAgentStatus` body and replace with the production implementation below. The handler control flow MUST be: (1) orgID extract, (2) request body validation, (3) BREAK_REASON PROBE FIRST (Pitfall 3 — runs regardless of force), (4) transition validator OR force bypass (force-true skips matrix only), (5) tx + atomic UPDATE with appropriate query (UpdateAgentStateStatus or ForceUpdateAgentStateStatus), (6) 0-row disambiguate via GetAgentStateByAgentId, (7) commit, (8) cache.Del, (9) emit force WARN if applicable, (10) 200 with mapped DTO.

```go
func (s *Server) PatchAgentStatus(ctx context.Context, req api.PatchAgentStatusRequestObject) (api.PatchAgentStatusResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
        }}, nil
    }
    if req.Body == nil {
        return api.PatchAgentStatus400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
            Error: api.ErrorCodeInvalidBody, Reason: "body_required",
        }}, nil
    }
    agentID := uuid.UUID(req.Id)
    forced := req.Body.Force != nil && *req.Body.Force
    targetStatus := req.Body.To

    // Load current row FIRST so we have observed `from` for matrix
    // validation, 409 body, and the UpdateAgentStateStatus expected_from
    // predicate (D-85). Single SELECT cheaper than carrying the row
    // through the tx for D-66 disambiguate.
    q := generated.New(s.deps.OrgDB)
    current, err := q.GetAgentStateByAgentId(ctx, generated.GetAgentStateByAgentIdParams{
        AgentID: pgUUID(agentID),
        OrgID:   pgUUID(orgID),
    })
    if errors.Is(err, pgx.ErrNoRows) {
        return api.PatchAgentStatus404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
            Error: api.ErrorCodeNotFound, Reason: "agent_state_not_found",
        }}, nil
    }
    if err != nil {
        s.deps.Logger.ErrorContext(ctx, "patch agent status: load current", "agent_id", agentID, "org_id", orgID, "err", err)
        return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error: api.ErrorCodeInternal, Reason: "agent_state_load_failed",
        }}, nil
    }
    observedFrom := api.AgentStatus(current.Status)

    // STEP 1 — BREAK_REASON PROBE (Pitfall 3 / Hazard 4): runs FIRST,
    // independent of force flag. Cross-org or missing/disabled break_reason_id
    // returns 422 invalid_reference even when force=true (D-84 explicit).
    if targetStatus == api.AgentStatusBreak {
        if req.Body.BreakReasonId == nil {
            return api.PatchAgentStatus422JSONResponse(api.ErrorResponse{
                Error:  api.ErrorCodeInvalidValue,
                Reason: "break_reason_id_required_for_break",
            }), nil
        }
        breakReasonID := uuid.UUID(*req.Body.BreakReasonId)
        _, qErr := q.GetBreakReasonForState(ctx, generated.GetBreakReasonForStateParams{
            ID:    pgUUID(breakReasonID),
            OrgID: pgUUID(orgID),
        })
        if errors.Is(qErr, pgx.ErrNoRows) {
            return api.PatchAgentStatus422JSONResponse(api.ErrorResponse{
                Error:  api.ErrorCodeInvalidReference,
                Reason: "break_reason_id_not_found_or_disabled",
            }), nil
        }
        if qErr != nil {
            s.deps.Logger.ErrorContext(ctx, "patch agent status: break_reason probe", "agent_id", agentID, "org_id", orgID, "err", qErr)
            return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
                Error: api.ErrorCodeInternal, Reason: "break_reason_probe_failed",
            }}, nil
        }
    }

    // STEP 2 — TRANSITION MATRIX or force bypass (D-84). force=true skips
    // the matrix entirely; matrix-validated edges still enforce
    // RequiresBreakReasonID via the probe above (Step 1 caught the nil case).
    if !forced {
        if _, vErr := validateTransition(observedFrom, targetStatus); vErr != nil {
            return api.PatchAgentStatus409JSONResponse(api.InvalidTransitionErrorResponse{
                Error: api.ErrorCodeInvalidTransition,
                From:  observedFrom,
                To:    targetStatus,
            }), nil
        }
    }

    // STEP 3 — TX + ATOMIC UPDATE (D-66 adapted for D-85). Non-force path
    // uses UpdateAgentStateStatus with `expected_from = observedFrom`;
    // force path uses ForceUpdateAgentStateStatus (no matrix predicate).
    tx, err := s.deps.OrgDB.BeginTx(ctx)
    if err != nil {
        s.deps.Logger.ErrorContext(ctx, "patch agent status: begin tx", "err", err)
        return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error: api.ErrorCodeInternal, Reason: "tx_begin_failed",
        }}, nil
    }
    defer func() { _ = tx.Rollback(ctx) }()
    qtx := generated.New(tx)

    var row generated.AgentState
    if forced {
        row, err = qtx.ForceUpdateAgentStateStatus(ctx, buildForceUpdateParams(agentID, orgID, req.Body))
    } else {
        row, err = qtx.UpdateAgentStateStatus(ctx, buildUpdateParams(agentID, orgID, observedFrom, req.Body))
    }
    if errors.Is(err, pgx.ErrNoRows) {
        // 0 rows landed. For the non-force path this is the D-66
        // disambiguate seam: either the row vanished (404) or a concurrent
        // PATCH shifted off observedFrom (409 invalid_transition).
        cur, perr := qtx.GetAgentStateByAgentId(ctx, generated.GetAgentStateByAgentIdParams{
            AgentID: pgUUID(agentID),
            OrgID:   pgUUID(orgID),
        })
        if errors.Is(perr, pgx.ErrNoRows) {
            return api.PatchAgentStatus404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
                Error: api.ErrorCodeNotFound, Reason: "agent_state_not_found",
            }}, nil
        }
        if perr != nil {
            s.deps.Logger.ErrorContext(ctx, "patch agent status: disambiguate", "err", perr)
            return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
                Error: api.ErrorCodeInternal, Reason: "disambiguate_failed",
            }}, nil
        }
        // D-56: 409 path also DELs cache so subsequent GET reloads fresh.
        if delErr := s.deps.Cache.Del(ctx, s.cacheKeyFor(orgID, agentID)); delErr != nil {
            s.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", "key", s.cacheKeyFor(orgID, agentID), "err", delErr)
        }
        return api.PatchAgentStatus409JSONResponse(api.InvalidTransitionErrorResponse{
            Error: api.ErrorCodeInvalidTransition,
            From:  api.AgentStatus(cur.Status),
            To:    targetStatus,
        }), nil
    }
    if err != nil {
        s.deps.Logger.ErrorContext(ctx, "patch agent status: update", "err", err)
        return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error: api.ErrorCodeInternal, Reason: "update_failed",
        }}, nil
    }
    if err := tx.Commit(ctx); err != nil {
        s.deps.Logger.ErrorContext(ctx, "patch agent status: commit", "err", err)
        return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error: api.ErrorCodeInternal, Reason: "commit_failed",
        }}, nil
    }

    // STEP 4 — Cache invalidation (D-55). Failure logs WARN; never 5xx.
    if delErr := s.deps.Cache.Del(ctx, s.cacheKeyFor(orgID, agentID)); delErr != nil {
        s.deps.Logger.WarnContext(ctx, "cache del failed", "key", s.cacheKeyFor(orgID, agentID), "err", delErr)
    }

    // STEP 5 — D-84 force audit. Emitted AFTER successful commit so the
    // audit log reflects an actual state change.
    if forced {
        s.deps.Logger.WarnContext(ctx, "state.force.applied",
            "agent_id", agentID,
            "from", observedFrom,
            "to", targetStatus,
            "org_id", orgID,
        )
    }

    return api.PatchAgentStatus200JSONResponse(mapAgentState(row)), nil
}

// buildUpdateParams + buildForceUpdateParams are local helpers that
// translate the request body into sqlc generated.<X>Params. Encapsulating
// the conversion keeps the handler control flow readable.
func buildUpdateParams(agentID, orgID uuid.UUID, expectedFrom api.AgentStatus, body *api.PatchAgentStatusJSONRequestBody) generated.UpdateAgentStateStatusParams {
    p := generated.UpdateAgentStateStatusParams{
        AgentID:      pgUUID(agentID),
        OrgID:        pgUUID(orgID),
        ExpectedFrom: string(expectedFrom),
        ToStatus:     pgtype.Text{String: string(body.To), Valid: true},
    }
    if body.BreakReasonId != nil {
        p.BreakReasonID = pgtype.UUID{Bytes: *body.BreakReasonId, Valid: true}
    }
    if body.PostInteractionState != nil {
        p.PostInteractionState = pgtype.Text{String: string(*body.PostInteractionState), Valid: true}
    }
    // engaged_channel + wrapup_until are system-set only — left nil here
    // (D-82 — agent-initiated PATCH cannot drive Engaged or set TTL).
    return p
}

func buildForceUpdateParams(agentID, orgID uuid.UUID, body *api.PatchAgentStatusJSONRequestBody) generated.ForceUpdateAgentStateStatusParams {
    p := generated.ForceUpdateAgentStateStatusParams{
        AgentID:  pgUUID(agentID),
        OrgID:    pgUUID(orgID),
        ToStatus: pgtype.Text{String: string(body.To), Valid: true},
    }
    if body.BreakReasonId != nil {
        p.BreakReasonID = pgtype.UUID{Bytes: *body.BreakReasonId, Valid: true}
    }
    if body.PostInteractionState != nil {
        p.PostInteractionState = pgtype.Text{String: string(*body.PostInteractionState), Valid: true}
    }
    return p
}
```

Hazards:
- **Pitfall 3 (Hazard 4) enforcement**: the break_reason probe (Step 1) runs BEFORE the `forced` branch check in Step 2. The forced flag does NOT skip Step 1. Verify by reading the control flow top-to-bottom: probe → matrix-or-bypass → tx → update.
- **D-66 disambiguate inside tx**: the disambiguate SELECT (`qtx.GetAgentStateByAgentId`) runs INSIDE the same tx so it observes a consistent snapshot. If it returns ErrNoRows, the row was deleted between Step 0 (initial load) and Step 3 (update) — return 404. If it returns a row, the status shifted off `observedFrom` — return 409 with the OBSERVED current status.
- **D-56 cache.Del on 409 path**: the disambiguate-409 path also DELs cache. Failure is logged but does not turn the 409 into a 500.
- **D-84 WARN slog ordering**: the force WARN is emitted AFTER `tx.Commit` succeeds. A force=true call that fails commit does NOT produce an audit entry (the state change didn't happen).
- **state_version monotonicity (STATE-08)**: the SQL `SET state_version = state_version + 1` is the source of truth. Test in Task 4 reads `row.StateVersion` post-PATCH and asserts > previous version.
- **Initial load vs disambiguate**: the handler does TWO reads on the no-row path (initial load + disambiguate). This is intentional — the initial load is OUTSIDE the tx (cheap, cache-friendly via existing GetAgentStatus path); the disambiguate is INSIDE the tx (consistent snapshot for 404 vs 409 distinction).
- **engaged_channel + wrapup_until**: agent-initiated PATCH does NOT set these (D-82 — system-only). buildUpdateParams leaves them as zero-value pgtype (Valid: false → SQL `nil::timestamptz` / `nil::text` → no-op in COALESCE).

Verify that `mappers.go` exports `pgUUID(id [16]byte)` so the agent_states.go callers compile. If catalog/agents.go uses `pgUUID(uuid.UUID)` (taking a `[16]byte` typed-alias), match that exact signature.
  </action>
  <acceptance_criteria>
    - The wave_1_stub is GONE: `grep -c "wave_1_stub" services/api/internal/state/agent_states.go` returns 0
    - Probe-then-matrix order: in `PatchAgentStatus`, the `GetBreakReasonForState` call appears before the `validateTransition` call (verified by line-number ordering: `[ "$(awk '/GetBreakReasonForState/{print NR; exit}' services/api/internal/state/agent_states.go)" -lt "$(awk '/validateTransition/{print NR; exit}' services/api/internal/state/agent_states.go)" ]`)
    - `grep -q "forced := req.Body.Force != nil && \*req.Body.Force" services/api/internal/state/agent_states.go`
    - `grep -q "state.force.applied" services/api/internal/state/agent_states.go`
    - `grep -q "qtx.ForceUpdateAgentStateStatus" services/api/internal/state/agent_states.go`
    - `grep -q "qtx.UpdateAgentStateStatus" services/api/internal/state/agent_states.go`
    - `grep -q "cache.Del" services/api/internal/state/agent_states.go` (cache invalidation on success)
    - `grep -c "s.deps.Cache.Del" services/api/internal/state/agent_states.go` ≥ 2 (success path + 409 disambiguate path per D-56)
    - 409 response uses InvalidTransitionErrorResponse with observed `From`: `grep -q "From:  api.AgentStatus(cur.Status)" services/api/internal/state/agent_states.go` OR equivalent reading of observed status into From field
    - `grep -q "InvalidTransitionErrorResponse" services/api/internal/state/agent_states.go`
    - 422 paths exist for invalid_value (break_reason_id missing) AND invalid_reference (cross-org): `grep -c "PatchAgentStatus422JSONResponse" services/api/internal/state/agent_states.go` ≥ 2
    - `cd services/api && go build ./internal/state/...` exits 0
    - `cd services/api && go vet ./internal/state/...` exits 0
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go build ./internal/state/... && go vet ./internal/state/... && [ "$(grep -c wave_1_stub /Users/luong/workspace/dev/open-solutions/services/api/internal/state/agent_states.go)" -eq 0 ] && [ "$(awk '/GetBreakReasonForState/{print NR; exit}' /Users/luong/workspace/dev/open-solutions/services/api/internal/state/agent_states.go)" -lt "$(awk '/validateTransition/{print NR; exit}' /Users/luong/workspace/dev/open-solutions/services/api/internal/state/agent_states.go)" ]</automated>
  </verify>
  <done>PatchAgentStatus production body in place; probe-then-matrix ordering enforced; force WARN emits post-commit; D-66 disambiguate via tx-scoped SELECT; cache.Del on success and 409 paths.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 4: Write agent_states_test.go — STATE-02/03/04/06/08 + force=true + cache invalidation</name>
  <files>services/api/internal/state/agent_states_test.go</files>
  <read_first>
    - services/api/internal/catalog/agents_test.go (716 lines — verbatim test template; D-72 pattern)
    - services/api/internal/state/testutil_test.go (newly created; seedAgent, seedBreakReason, seedAgentStateRow, httpPATCHStatus helpers)
    - services/api/internal/state/agent_states.go (just-implemented production body)
    - .planning/phases/04-agent-state-machine-go/04-VALIDATION.md (test name mappings — must match expectations)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 346-385 — coverage matrix)
  </read_first>
  <behavior>
    - TestPatchAgentStatus_AllowedTransition_HappyPath: NotReady → Ready returns 200 with state_version=2; row in DB shows new status.
    - TestPatchAgentStatus_InvalidTransition_409: Engaged → Ready (matrix-rejected — Engaged is not in matrix outer keys) returns 409 with body `{error: invalid_transition, from: Engaged, to: Ready}`.
    - TestPatchAgentStatus_BreakReasonProbe_422_missing: Ready → Break with break_reason_id of a non-existent uuid returns 422 invalid_reference.
    - TestPatchAgentStatus_BreakReasonProbe_200_valid: Ready → Break with valid same-org break_reason_id returns 200 with break_reason_id recorded on the row.
    - TestPatchAgentStatus_BreakReasonRequired_422: Ready → Break with break_reason_id == nil returns 422 invalid_value, reason "break_reason_id_required_for_break".
    - TestPatchAgentStatus_PostInteractionState_SetWhileEngaged: seed Engaged via seedAgentStateRow; PATCH with `post_interaction_state: ready` and `to: Engaged` is rejected (Engaged is not a v0.1 target); seed test re-design — instead seed Engaged, PATCH `to: NotReady` (would be 409 because Engaged not in matrix), so STATE-06 is verified by direct UPDATE path: seed Engaged with PIS=null, force-PATCH to Engaged with PIS=ready, verify PIS column updated. STATE-06's spec says "agent CAN set post_interaction_state while Engaged" — the validator allows the matrix entry Engaged→Engaged only if force=true OR via a future system path; v0.1 tests via force=true.
    - TestPatchAgentStatus_StateVersionMonotonic: 3 successful PATCHes increment state_version 1→2→3→4.
    - TestPatchAgentStatus_Force_BypassesMatrix: seed Engaged; PATCH with `to: NotReady, force: true` returns 200 (matrix rejected this edge, force bypassed). slog WARN captured via test logger.
    - TestPatchAgentStatus_Force_DoesNotBypassBreakReason: seed Ready; PATCH `to: Break, break_reason_id: <non-existent uuid>, force: true` returns 422 invalid_reference (Pitfall 3 — force does NOT bypass cross-row probe).
    - TestPatchAgentStatus_404_AgentMissing: PATCH against random agent_id returns 404.
    - TestPatchAgentStatus_CacheInvalidation: GET (populates cache), PATCH (mutates + DELs), GET again returns the new state; miniredis assertion `th.Miniredis.Exists(cacheKey) == false` immediately after PATCH.
    - TestGetAgentStatus_404_AgentMissing: GET against random agent_id returns 404 (cache returns ErrNotFound).
    - TestGetAgentStatus_200_FromCache: PATCH to populate row → first GET (cache miss + DB load + cache.Set) → second GET (cache hit — verified by stopping DB temporarily? simpler assertion: miniredis carries the key after first GET).
  </behavior>
  <action>
TDD: structure the test file as a single TestXxx per behavior. Each test starts with `t.Parallel()` (where safe) and a fresh `th := newTestHandlers(t); cleanStateTables(t, ctx, th.Pool)`.

Write the file `services/api/internal/state/agent_states_test.go` exporting these test functions exactly:

```go
package state

import (
    "context"
    "encoding/json"
    "net/http"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/require"

    "github.com/luongdev/open-routing/services/api/internal/api"
)

func TestPatchAgentStatus_AllowedTransition_HappyPath(t *testing.T) { /* seed agent + agent_states (status=NotReady, version=1); PATCH to=Ready; assert 200 + version=2 */ }
func TestPatchAgentStatus_InvalidTransition_409(t *testing.T)        { /* seed Engaged via seedAgentStateRow; PATCH to=Ready; assert 409 + body has from=Engaged, to=Ready, error=invalid_transition */ }
func TestPatchAgentStatus_BreakReasonProbe_422_missing(t *testing.T) { /* seed Ready; PATCH to=Break, break_reason_id=random; assert 422 invalid_reference */ }
func TestPatchAgentStatus_BreakReasonProbe_200_valid(t *testing.T)   { /* seed Ready + break_reason; PATCH to=Break with that break_reason_id; assert 200 + row has break_reason_id set */ }
func TestPatchAgentStatus_BreakReasonRequired_422(t *testing.T)      { /* seed Ready; PATCH to=Break with nil break_reason_id; assert 422 invalid_value, reason=break_reason_id_required_for_break */ }
func TestPatchAgentStatus_PostInteractionState_SetViaForce(t *testing.T) { /* seed Engaged (via seedAgentStateRow); PATCH to=Engaged force=true post_interaction_state=ready; assert 200 + row has post_interaction_state=ready (STATE-06 schema-ready path) */ }
func TestPatchAgentStatus_StateVersionMonotonic(t *testing.T)        { /* seed NotReady; 3 PATCHes through valid transitions; assert versions 1→2→3→4 */ }
func TestPatchAgentStatus_Force_BypassesMatrix(t *testing.T)         { /* seed Engaged (matrix outer key absent); PATCH to=NotReady force=true; assert 200 (would be 409 without force) */ }
func TestPatchAgentStatus_Force_DoesNotBypassBreakReason(t *testing.T) { /* seed Ready; PATCH to=Break with cross-org break_reason_id force=true; assert 422 invalid_reference — Pitfall 3 regression gate */ }
func TestPatchAgentStatus_404_AgentMissing(t *testing.T)             { /* random agent_id; PATCH to=Ready; assert 404 */ }
func TestPatchAgentStatus_CacheInvalidation(t *testing.T)             { /* seed NotReady; GET (populate cache); PATCH to=Ready; assert miniredis cache key absent; second GET returns new status */ }
func TestGetAgentStatus_404_AgentMissing(t *testing.T)               { /* random agent_id; GET; assert 404 */ }
func TestGetAgentStatus_200_FromCache(t *testing.T)                  { /* seed Ready; GET → cache hit → miniredis assertion key exists; second GET still 200 */ }
```

For each test:
- Use `t.Parallel()` ONLY for tests that don't mutate the same fixtures concurrently. Tests that touch the testutil-shared pool need careful org-isolation via `freshOrg(t)` — newTestHandlers already generates a unique orgID per call, so parallel is safe.
- Use `require.Equal(t, http.StatusXXX, resp.StatusCode, "...")` with a body-string error message for fast debug.
- For 409 assertions, parse the body into `api.InvalidTransitionErrorResponse` and assert `resp.From == <observed>` and `resp.To == <requested>`.
- For 422 assertions, parse into `api.ErrorResponse` and assert `resp.Error == api.ErrorCodeInvalidReference` (cross-org/missing FK) or `api.ErrorCodeInvalidValue` (required field missing).
- For cache-invalidation assertions, use `th.Miniredis.Exists(cacheKey)` where `cacheKey := cache.Key(th.OrgID, "agent_state", agentID)`.
- For state_version assertions, parse the 200 response body into `api.AgentState` and assert `resp.StateVersion` matches the expected value.

Hazards:
- `seedAgentStateRow` MUST run with `Status: "Engaged"` (or whatever bypass status is needed) via raw INSERT — the production CreateAgent path seeds `Offline` (D-93 + Wave 4). Tests must NOT rely on Wave 4 wiring; they seed directly.
- `TestPatchAgentStatus_PostInteractionState_SetViaForce` — STATE-06 says agent CAN set PIS while Engaged. The matrix in Wave 1 does NOT include any `Engaged→*` agent-initiated edges (D-82 — Engaged transitions are system-only in v0.1). The test path: seed Engaged via raw INSERT, then PATCH with `to=Engaged + post_interaction_state=ready + force=true`. force=true bypasses the matrix (which would reject Engaged→Engaged as a no-edge); the COALESCE in UpdateAgentStateStatus updates PIS. Assert the DB row has `post_interaction_state = 'ready'`.
- `TestPatchAgentStatus_Force_DoesNotBypassBreakReason` is the Pitfall 3 regression gate. Use `seedBreakReason(t, pool, ORG_B, breakReasonID, "OtherOrg", false)` to plant a different-org break_reason. The PATCH from ORG_A's agent should still return 422.
- Wait/poll for cache invalidation is NOT needed — `cache.Del` is synchronous; miniredis assertion immediately after PATCH must observe the key gone.
- Tests use `httpPATCHStatus(t, th, agentID, body)` helper from testutil.
  </action>
  <acceptance_criteria>
    - All 13 test functions declared (verified by `grep -c "^func Test" services/api/internal/state/agent_states_test.go` ≥ 13)
    - Each test function name from the list above is present (verified individually):
      - `grep -q "^func TestPatchAgentStatus_AllowedTransition_HappyPath" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestPatchAgentStatus_InvalidTransition_409" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestPatchAgentStatus_BreakReasonProbe_422_missing" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestPatchAgentStatus_BreakReasonProbe_200_valid" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestPatchAgentStatus_BreakReasonRequired_422" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestPatchAgentStatus_PostInteractionState_SetViaForce" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestPatchAgentStatus_StateVersionMonotonic" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestPatchAgentStatus_Force_BypassesMatrix" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestPatchAgentStatus_Force_DoesNotBypassBreakReason" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestPatchAgentStatus_404_AgentMissing" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestPatchAgentStatus_CacheInvalidation" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestGetAgentStatus_404_AgentMissing" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestGetAgentStatus_200_FromCache" services/api/internal/state/agent_states_test.go`
    - `cd services/api && go test -count=1 -v -run "TestPatchAgentStatus|TestGetAgentStatus" ./internal/state/...` exits 0 with all tests passing (or skipped if testcontainer unavailable in CI — verify locally first)
    - `grep -q "th.Miniredis.Exists" services/api/internal/state/agent_states_test.go` (cache invalidation assertion)
    - `grep -q "ErrorCodeInvalidReference" services/api/internal/state/agent_states_test.go` (422 cross-org probe path)
    - `grep -q "ErrorCodeInvalidTransition" services/api/internal/state/agent_states_test.go` (409 path)
    - `grep -q "Force.*true" services/api/internal/state/agent_states_test.go` (force=true happy + cross-org)
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go test -count=1 -v -run "TestPatchAgentStatus|TestGetAgentStatus" ./internal/state/... && [ "$(grep -c '^func Test' /Users/luong/workspace/dev/open-solutions/services/api/internal/state/agent_states_test.go)" -ge 13 ]</automated>
  </verify>
  <done>13 named tests passing; cache invalidation verified via miniredis; Pitfall 3 regression gate in place (force does NOT bypass cross-org probe).</done>
</task>

<task type="auto">
  <name>Task 5: Extend services/api/internal/server/server.go injectRequestIDIntoErrorResponse type switch if PatchAgentStatus409JSONResponse case is missing</name>
  <files>
    services/api/internal/server/server.go,
    services/api/internal/server/request_id_exhaustiveness_test.go
  </files>
  <read_first>
    - services/api/internal/server/server.go (lines 210-580 — injectRequestIDIntoErrorResponse function)
    - services/api/internal/api/server.gen.go (PatchAgentStatus409JSONResponse type definition — search via grep)
    - services/api/internal/server/request_id_exhaustiveness_test.go (current exhaustiveness test)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 836-865 — Pitfall 5)
  </read_first>
  <action>
First, run the exhaustiveness test to see if it currently passes or fails:

```bash
cd services/api && go test -count=1 -run TestRequestIDInjection_Exhaustiveness ./internal/server/...
```

If it PASSES (existing test already covers PatchAgentStatus409JSONResponse), this task is a no-op for server.go — proceed to verify the test will still pass after Wave 2 wiring.

If it FAILS with a "missing case" error, locate the unhandled type and add the corresponding `case` in the `injectRequestIDIntoErrorResponse` type switch. The expected case for the 409 invalid_transition response:

```go
case api.PatchAgentStatus409JSONResponse:
    // InvalidTransitionErrorResponse already carries a RequestId field on
    // the wrapped struct; set it if nil.
    inner := api.InvalidTransitionErrorResponse(r)
    inner.RequestId = setIfNil(inner.RequestId, id)
    return api.PatchAgentStatus409JSONResponse(inner)
```

Insert this case adjacent to the existing PatchAgentStatus422JSONResponse case (line 234). The placement is alphabetical-by-status-code within the switch.

If the test fails for OTHER new types (e.g., PatchAgentStatus500JSONResponse), check if those cases already exist via `grep -n "PatchAgentStatus" services/api/internal/server/server.go`. Add only the truly missing cases. Do NOT remove or rewrite existing cases.

Verify after the edit:

```bash
cd services/api && go test -count=1 -run TestRequestIDInjection_Exhaustiveness ./internal/server/...
```

Must exit 0. The exhaustiveness test iterates every `*JSONResponse` type emitted by oapi-codegen and confirms each has a switch case.

Hazards:
- Pitfall 5 (A7 assumption): the Wave 0 spec amendment for `force` was a request-body-only change — it should NOT have introduced any new response types. If the test fails for a type other than PatchAgentStatus409JSONResponse, the assumption is wrong; investigate the regenerated server.gen.go diff.
- Do NOT modify the test file unless the test framework reports a "subtest missing" error (different from a "case missing" error). The existing test should auto-discover the new type via reflection on the JSONResponse interface implementations.
- The `setIfNil` helper is from earlier in server.go — verify it accepts `*openapi_types.UUID` and `string` for the id parameter (it does in Phase 2; signature confirmed via grep).
  </action>
  <acceptance_criteria>
    - `cd services/api && go test -count=1 -run TestRequestIDInjection_Exhaustiveness ./internal/server/...` exits 0
    - If a `case api.PatchAgentStatus409JSONResponse:` was added, `grep -q "case api.PatchAgentStatus409JSONResponse:" services/api/internal/server/server.go` returns true
    - If no new case was needed (test already passed before edit), the file is unmodified — `git diff --exit-code services/api/internal/server/server.go` returns 0 from the start
    - Both `case api.PatchAgentStatus422JSONResponse:` AND `case api.PatchAgentStatus409JSONResponse:` (if added) are present in server.go
    - `cd services/api && go build ./...` exits 0
    - `cd services/api && go test -count=1 ./internal/server/...` exits 0 (full server package suite)
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go build ./... && go test -count=1 -run TestRequestIDInjection_Exhaustiveness ./internal/server/... && go test -count=1 ./internal/server/...</automated>
  </verify>
  <done>RequestID exhaustiveness test passes; injectRequestIDIntoErrorResponse covers every PatchAgentStatus response type.</done>
</task>

</tasks>

<verification>
- Production handler bodies replace Wave 1 stubs; `grep -c wave_1_stub services/api/internal/state/agent_states.go` returns 0
- Probe-then-matrix ordering verified by line-number assertion in acceptance criteria
- D-66 disambiguate runs INSIDE the tx so 404 vs 409 distinction observes a consistent snapshot
- D-56 cache.Del fires on BOTH success and 409 disambiguate paths
- D-84 force WARN emits POST-commit (failed commits don't produce audit entries)
- state_version monotonicity validated by TestPatchAgentStatus_StateVersionMonotonic across 3 transitions
- Pitfall 3 regression gate in place: TestPatchAgentStatus_Force_DoesNotBypassBreakReason proves force=true does NOT skip cross-row probe
- RequestID exhaustiveness still green post-spec-amendment (Pitfall 5)
</verification>

<success_criteria>
- 5 tasks completed; 7 files modified (4 source, 3 test/scaffolding)
- `cd services/api && go test -count=1 ./internal/state/...` exits 0 with at least 13 tests + transition matrix tests passing
- `cd services/api && go test -count=1 -run TestRequestIDInjection_Exhaustiveness ./internal/server/...` exits 0
- Pitfall 3 regression test in place and green
- Cache invalidation observable via miniredis assertion
- state_version monotonicity proven across 3+ transitions
</success_criteria>

<output>
Create `.planning/phases/04-agent-state-machine-go/04-03-SUMMARY.md` when done. Include: list of files modified, output of `go test -count=1 ./internal/state/... ./internal/server/...`, confirmation that wave_1_stub strings are gone, confirmation that probe-then-matrix ordering is enforced (with line numbers from agent_states.go), confirmation that force=true cross-org probe regression test is in place.
</output>
---
phase: 04-agent-state-machine-go
plan: 04
type: execute
wave: 3
depends_on:
  - 04-02
files_modified:
  - services/api/internal/state/ttl.go
  - services/api/internal/state/ttl_test.go
  - services/api/internal/state/handlers.go
autonomous: true
requirements:
  - STATE-07
  - STATE-08

must_haves:
  truths:
    - "scheduleWrapUpExpiry registers a per-agent clockwork.AfterFunc timer keyed by agent_id; replaces any prior timer atomically (Timer.Stop on old, set new in registry)"
    - "expireWrapUp executes the idempotent ExpireWrapUp UPDATE under a 10s ctx timeout + db.WithBypass(ctx, 'wrapup_sweeper'); 0 rows = silently NO-OP (already expired by concurrent firing)"
    - "Server.Start(ctx) synchronously runs the startup sweep BEFORE returning: ListExpiringWrapUps → for each, IF wrapup_until < clock.Now() then expireWrapUp immediately ELSE scheduleWrapUpExpiry"
    - "Server.Start spawns the 30s safety-sweep goroutine that ticks on clock.NewTicker and re-runs the past-due fire-immediately loop"
    - "Server.Stop cancels the internal ctx, waits for the safety-sweep goroutine via sync.WaitGroup, and iterates the timers registry calling Timer.Stop on each pending AfterFunc"
    - "cache.Del fires post-UPDATE on every TTL expiry via the orgID returned by ExpireWrapUp (Pitfall 8 — sweeper has no ctx-injected org_id)"
    - "All TTL paths increment state_version monotonically via the SQL `state_version = state_version + 1` in ExpireWrapUp (STATE-08)"
  artifacts:
    - path: "services/api/internal/state/ttl.go"
      provides: "scheduleWrapUpExpiry + expireWrapUp + startup sweep + safety-sweep goroutine + Start/Stop bodies"
      contains: "func (s *Server) scheduleWrapUpExpiry"
    - path: "services/api/internal/state/ttl_test.go"
      provides: "clockwork.FakeClock-driven tests: FiresOnExpiry, StartupSweep, SafetySweep, GracefulShutdown, RaceCondition"
      contains: "clockwork.NewFakeClock"
    - path: "services/api/internal/state/handlers.go"
      provides: "Start/Stop bodies replaced — call ttl.go startup sweep + spawn safety-sweep goroutine"
      contains: "s.startupSweep"
  key_links:
    - from: "services/api/internal/state/ttl.go"
      to: "services/api/internal/db/generated/agent_states.sql.go"
      via: "qtx.ListExpiringWrapUps + qtx.ExpireWrapUp from Wave 1 sqlc"
      pattern: "(ListExpiringWrapUps|ExpireWrapUp)"
    - from: "services/api/internal/state/ttl.go"
      to: "services/api/internal/db/bypass.go"
      via: "db.WithBypass for sweeper context (Pitfall 8 — Hazard 5)"
      pattern: "db\\.WithBypass"
    - from: "services/api/internal/state/ttl.go"
      to: "github.com/jonboulle/clockwork"
      via: "clock.AfterFunc + clock.NewTicker + clock.Now"
      pattern: "s\\.clock\\.(AfterFunc|NewTicker|Now)"
---

<objective>
Implement the WrapUp TTL goroutine machinery in `services/api/internal/state/ttl.go`: per-agent `clockwork.AfterFunc` timer registry (`timersRegistry` declared in Wave 1 — fill in the methods now), `scheduleWrapUpExpiry(agentID, wrapupUntil)` that registers/replaces a timer atomically, `expireWrapUp(agentID, orgID)` that runs the idempotent `ExpireWrapUp` UPDATE under `db.WithBypass` ctx and DELs cache on success, a synchronous startup sweep that runs inside `Server.Start(ctx)` BEFORE returning (fires past-due rows immediately + schedules future timers), and a 30s safety-sweep goroutine that catches missed firings. `Server.Stop()` cancels the internal ctx, waits for the sweeper goroutine via `sync.WaitGroup`, and iterates the registry calling `Timer.Stop()` on each pending AfterFunc handle.

Purpose: STATE-07 acceptance criterion — "An agent in WrapUp whose browser is closed returns to their `post_interaction_state` automatically after `wrapup_until` expires, with no client action required." Wave 3 is the entire delivery vehicle for this requirement. The clockwork-based design (Pattern 2) makes the goroutine deterministically testable via `fakeClock.Advance(d)` so the 5 named acceptance tests in 04-VALIDATION.md become reliable CI gates.

Output: Full TTL goroutine implementation; passing race-free tests (`go test -race ./internal/state/...`); Start/Stop lifecycle bodies replace the Wave 1 stubs; graceful shutdown drains in-flight UPDATEs and cancels pending timers.

**Hazards captured here:**
- **Pitfall 8 (Hazard 5 in PATTERNS.md):** Sweeper goroutine runs OUTSIDE an HTTP request (no X-Org-Id header in ctx), but SQLChecker still requires every query to mention `org_id`. Pattern: use `db.WithBypass(ctx, "wrapup_sweeper")` (Phase 1 D-04) on the sweeper context BEFORE constructing `generated.New(...)` queries. SQL still includes `WHERE org_id = $2` for auditability — the ExpireWrapUp params explicitly pass `org_id` per the Wave 1 SQL.
- **Pitfall 2 (Sweeper goroutine outlives Stop()):** Inside the AfterFunc body, first `select { case <-h.ctx.Done(): return; default: }` before the DB call. clockwork's mocked Timer respects the same semantic so tests assert clean shutdown via `go test -race`.
- **Pitfall 9 (FakeClock not advanced):** Tests use `fakeClock.Advance(d)` followed by `require.Eventually(t, ...)` — clockwork does NOT tick automatically.
</objective>

<execution_context>
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/workflows/execute-plan.md
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@./CLAUDE.md
@.planning/phases/04-agent-state-machine-go/04-CONTEXT.md
@.planning/phases/04-agent-state-machine-go/04-RESEARCH.md
@.planning/phases/04-agent-state-machine-go/04-PATTERNS.md
@.planning/phases/04-agent-state-machine-go/04-VALIDATION.md
@.planning/phases/04-agent-state-machine-go/04-02-PLAN.md
@.planning/phases/04-agent-state-machine-go/04-02-SUMMARY.md
@services/api/internal/state/handlers.go
@services/api/internal/state/agent_states.go
@services/api/internal/state/testutil_test.go
@services/api/internal/db/bypass.go
@services/api/internal/db/orgdb.go
@services/api/internal/db/generated/agent_states.sql.go
</context>

<interfaces>
From `services/api/internal/state/handlers.go` (Wave 1):
```go
const (
    defaultSweepInterval = 30 * time.Second
    defaultWrapUpDur     = 60 * time.Second
    stateCacheTTL        = 60 * time.Second
    jitterMaxMs          = 100
)

type Server struct {
    deps          Deps
    clock         clockwork.Clock
    sweepInterval time.Duration
    timers        *timersRegistry
    ctx           context.Context
    cancel        context.CancelFunc
    wg            sync.WaitGroup
    started       bool
    startMu       sync.Mutex
}

type timersRegistry struct {
    mu sync.RWMutex
    t  map[uuid.UUID]clockwork.Timer
}
```

From `github.com/jonboulle/clockwork v0.4.0`:
```go
type Clock interface {
    Now() time.Time
    After(d time.Duration) <-chan time.Time
    AfterFunc(d time.Duration, f func()) Timer
    NewTicker(d time.Duration) Ticker
    Sleep(d time.Duration)
}

type Timer interface { Reset(d time.Duration) bool; Stop() bool; Chan() <-chan time.Time }
type Ticker interface { Chan() <-chan time.Time; Stop() }

func NewRealClock() Clock
func NewFakeClock() FakeClock
type FakeClock interface { Clock; Advance(d time.Duration); ... }
```

From `services/api/internal/db/bypass.go`:
```go
func WithBypass(ctx context.Context, reason string) context.Context
```

From `services/api/internal/db/generated/agent_states.sql.go` (Wave 1):
```go
func (q *Queries) ListExpiringWrapUps(ctx context.Context) ([]ListExpiringWrapUpsRow, error)
type ListExpiringWrapUpsRow struct {
    AgentID              pgtype.UUID
    OrgID                pgtype.UUID
    WrapupUntil          pgtype.Timestamptz
    PostInteractionState pgtype.Text
}

func (q *Queries) ExpireWrapUp(ctx context.Context, agentID, orgID pgtype.UUID) (ExpireWrapUpRow, error)
type ExpireWrapUpRow struct {
    AgentID      pgtype.UUID
    OrgID        pgtype.UUID
    Status       string
    StateVersion int64
}
```
</interfaces>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Sweeper goroutine ctx → DB | No HTTP request context; org_id is denormalized into agent_states column. db.WithBypass marks the ctx for SQLChecker audit. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-04-10 | Tampering | Sweeper goroutine SQLChecker bypass | mitigate | Every sweeper-side query uses `db.WithBypass(ctx, "wrapup_sweeper")`; structured slog audit event emitted automatically by orgdb.preflight (Phase 1 D-04). |
| T-04-11 | Denial of Service | Hung DB call in AfterFunc leaks goroutine | mitigate | AfterFunc body wraps in `context.WithTimeout(10*time.Second)` so the goroutine drains even if DB stalls. |
| T-04-12 | Repudiation | TTL expiry without audit trail | mitigate | Every successful expireWrapUp emits `slog.InfoContext(ctx, "state.ttl.expired", ...)`. 0-row no-op emits `slog.DebugContext`. |
</threat_model>

<tasks>

<task type="auto">
  <name>Task 1: Create services/api/internal/state/ttl.go — scheduleWrapUpExpiry + expireWrapUp + startup sweep + safety-sweep goroutine</name>
  <files>services/api/internal/state/ttl.go</files>
  <read_first>
    - services/api/internal/state/handlers.go (Wave 1 Server struct + timersRegistry + Start/Stop stubs)
    - services/api/internal/db/bypass.go (WithBypass signature)
    - services/api/internal/db/generated/agent_states.sql.go (ListExpiringWrapUps + ExpireWrapUp from Wave 1)
    - services/api/internal/state/mappers.go (pgUUID helper)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (§Pattern 2 — verbatim ttl.go shape lines 357-438; §Code Examples lines 920-1000)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 129-163 — TTL hazards from Pitfalls)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-81 + D-87 + D-95 lifecycle)
  </read_first>
  <action>
Create `services/api/internal/state/ttl.go`:

```go
package state

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/luongdev/open-routing/services/api/internal/db"
    "github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// scheduleWrapUpExpiry registers (or replaces) a per-agent AfterFunc
// timer. The fn closure carries the idempotent ExpireWrapUp UPDATE —
// concurrent firings (timer + safety-sweep + v1 multi-replica) race the
// UPDATE's `WHERE status='WrapUp' AND wrapup_until < NOW()` predicate;
// only the first wins (D-81).
//
// Called from PATCH handlers when v0.2 system fires Engaged→WrapUp;
// also called from startupSweep for rows seeded directly via fixture
// (tests) or returning from process restart.
func (s *Server) scheduleWrapUpExpiry(agentID, orgID uuid.UUID, until time.Time) {
    remaining := until.Sub(s.clock.Now())
    if remaining < 0 {
        remaining = 0 // past-due: AfterFunc(0, fn) fires immediately on next scheduler tick
    }
    s.timers.mu.Lock()
    defer s.timers.mu.Unlock()
    if prior, ok := s.timers.t[agentID]; ok {
        prior.Stop() // replace pattern — v0.2 WrapUp extension or re-seed
    }
    s.timers.t[agentID] = s.clock.AfterFunc(remaining, func() {
        // Pitfall 2: respect ctx.Done() before any DB work so Stop()
        // drains cleanly. clockwork's mocked Timer respects this branch.
        select {
        case <-s.ctx.Done():
            return
        default:
        }
        // Bounded ctx so a hung DB call never leaks the goroutine
        // indefinitely (Threat T-04-11).
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        s.expireWrapUp(ctx, agentID, orgID)
        // Remove the timer handle from registry after firing — keeps the
        // map bounded under steady-state load.
        s.timers.mu.Lock()
        delete(s.timers.t, agentID)
        s.timers.mu.Unlock()
    })
}

// expireWrapUp executes the idempotent ExpireWrapUp UPDATE. Safe to
// call from AfterFunc, safety sweep, and startup sweep. 0 rows = NO-OP
// (already expired by another firing or status changed — D-81 idempotency).
//
// Pitfall 8: sweeper has no ctx-injected org_id. db.WithBypass marks the
// ctx for SQLChecker audit (Phase 1 D-04). The UPDATE itself still
// mentions org_id in WHERE (denormalized agent_states.org_id column per
// D-78) for query auditability + Postgres plan stability.
func (s *Server) expireWrapUp(ctx context.Context, agentID, orgID uuid.UUID) {
    ctx = db.WithBypass(ctx, "wrapup_sweeper")
    q := generated.New(s.deps.OrgDB)
    row, err := q.ExpireWrapUp(ctx, generated.ExpireWrapUpParams{
        AgentID: pgUUID(agentID),
        OrgID:   pgUUID(orgID),
    })
    if errors.Is(err, pgx.ErrNoRows) {
        // Idempotent: another firing already expired this row, OR the
        // row's status changed between schedule and fire. DEBUG-only.
        s.deps.Logger.DebugContext(ctx, "state.ttl.expire.noop",
            "agent_id", agentID, "org_id", orgID)
        return
    }
    if err != nil {
        s.deps.Logger.WarnContext(ctx, "state.ttl.expire.failed",
            "agent_id", agentID, "org_id", orgID, "err", err)
        return
    }
    // Cache invalidation. Failure logs WARN; cache will heal via 60s TTL.
    if delErr := s.deps.Cache.Del(ctx, s.cacheKeyFor(orgID, agentID)); delErr != nil {
        s.deps.Logger.WarnContext(ctx, "state.ttl.cache_del_failed",
            "key", s.cacheKeyFor(orgID, agentID), "err", delErr)
    }
    s.deps.Logger.InfoContext(ctx, "state.ttl.expired",
        "agent_id", agentID,
        "org_id", orgID,
        "new_status", row.Status,
        "state_version", row.StateVersion,
    )
}

// startupSweep runs synchronously inside Start(ctx). It loads every
// WrapUp row across all orgs, fires-immediately for past-due rows, and
// schedules future timers for the rest. Per D-95 the sweep blocks Start
// from returning so no stuck WrapUp survives a restart.
func (s *Server) startupSweep(ctx context.Context) error {
    ctx = db.WithBypass(ctx, "wrapup_sweeper.startup")
    q := generated.New(s.deps.OrgDB)
    rows, err := q.ListExpiringWrapUps(ctx)
    if err != nil {
        return fmt.Errorf("list expiring wrapups: %w", err)
    }
    now := s.clock.Now()
    for _, r := range rows {
        agentID := uuid.UUID(r.AgentID.Bytes)
        orgID := uuid.UUID(r.OrgID.Bytes)
        until := r.WrapupUntil.Time
        if !r.WrapupUntil.Valid || !until.After(now) {
            // Past-due (or null wrapup_until — shouldn't happen for
            // status=WrapUp but defensive). Fire immediately.
            s.expireWrapUp(ctx, agentID, orgID)
            continue
        }
        s.scheduleWrapUpExpiry(agentID, orgID, until)
    }
    return nil
}

// safetySweep runs on a clock.NewTicker cadence (default 30s,
// overridable via WithSweepInterval). On each tick it re-lists WrapUp
// rows and fires-immediately for any past-due row that the AfterFunc
// timer missed (panic, drift, clock skew). Idempotent UPDATE protects
// against double-firing.
func (s *Server) safetySweep() {
    defer s.wg.Done()
    ticker := s.clock.NewTicker(s.sweepInterval)
    defer ticker.Stop()
    for {
        select {
        case <-s.ctx.Done():
            return
        case <-ticker.Chan():
            s.runSweepPastDue()
        }
    }
}

// runSweepPastDue is the per-tick body. Extracted so tests can call it
// deterministically without spinning the ticker.
func (s *Server) runSweepPastDue() {
    ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
    defer cancel()
    ctx = db.WithBypass(ctx, "wrapup_sweeper.safety")
    q := generated.New(s.deps.OrgDB)
    rows, err := q.ListExpiringWrapUps(ctx)
    if err != nil {
        s.deps.Logger.WarnContext(ctx, "state.ttl.safety_sweep.list_failed", "err", err)
        return
    }
    now := s.clock.Now()
    for _, r := range rows {
        if !r.WrapupUntil.Valid || r.WrapupUntil.Time.After(now) {
            continue // not past-due
        }
        agentID := uuid.UUID(r.AgentID.Bytes)
        orgID := uuid.UUID(r.OrgID.Bytes)
        s.expireWrapUp(ctx, agentID, orgID)
    }
}

// jitter returns a duration in [-jitterMaxMs, +jitterMaxMs] for D-87
// thundering-herd guard. Used by future v0.2 callers that batch-set
// wrapup_until on simultaneous Engaged→WrapUp transitions.
func (s *Server) jitter() time.Duration {
    // D-87: ±100ms window. math/rand is import-clean (no crypto needed);
    // deterministic across runs would be the next concern but rand
    // already varies per process start in Go 1.21+ stdlib.
    n := int64(uuid.New().Time()) // crude entropy source from uuid clock part
    sign := int64(1)
    if n&1 == 1 {
        sign = -1
    }
    return time.Duration(sign*((n%int64(jitterMaxMs)))) * time.Millisecond
}

// unused — pgtype.Text import guard so go vet doesn't flag the file
// when expireWrapUp's earlier draft used pgtype directly. Keep for the
// future helper in this package; remove if unused after Wave 4 review.
var _ pgtype.Text
```

Now update `Server.Start(ctx)` and `Server.Stop()` in `services/api/internal/state/handlers.go` to drive the new bodies. Replace the Wave 1 minimal stubs:

```go
func (s *Server) Start(ctx context.Context) error {
    s.startMu.Lock()
    defer s.startMu.Unlock()
    if s.started {
        return nil
    }
    s.started = true
    s.ctx, s.cancel = context.WithCancel(ctx)

    // D-95: startup sweep is synchronous — guarantees no stuck WrapUp
    // survives a restart. Failure here aborts startup (returned to main.go).
    if err := s.startupSweep(s.ctx); err != nil {
        s.started = false
        s.cancel()
        return fmt.Errorf("state.Server: startup sweep: %w", err)
    }

    // Spawn 30s safety-sweep goroutine.
    s.wg.Add(1)
    go s.safetySweep()

    return nil
}

func (s *Server) Stop() {
    s.startMu.Lock()
    if !s.started {
        s.startMu.Unlock()
        return
    }
    s.started = false
    s.startMu.Unlock()

    // Cancel ctx — drains safety-sweep goroutine + makes every AfterFunc
    // body's `<-s.ctx.Done()` branch fire instead of doing DB work.
    if s.cancel != nil {
        s.cancel()
    }

    // Wait for safety-sweep goroutine to return.
    s.wg.Wait()

    // Cancel all pending AfterFunc handles. Acquire under write lock so
    // no scheduleWrapUpExpiry can race the iteration.
    s.timers.mu.Lock()
    for id, t := range s.timers.t {
        t.Stop()
        delete(s.timers.t, id)
    }
    s.timers.mu.Unlock()
}
```

Add the missing `fmt` import to handlers.go (currently absent in Wave 1).

Hazards:
- The `s.timers.t[agentID] = s.clock.AfterFunc(...)` MUST happen under the registry write lock. The closure inside `AfterFunc` does NOT hold the lock when firing — it acquires its own write lock to delete the entry after the body completes.
- `Server.Stop()` releases `startMu` BEFORE calling `s.cancel()` because `safetySweep`'s `<-s.ctx.Done()` branch needs to observe the cancellation; if Stop held startMu while waiting on `wg.Wait()`, a re-entrant Stop call would deadlock. The `started=false` flip is atomic under startMu, then the lock is released.
- The `jitter()` function uses a crude uuid-time entropy source. If the team prefers `math/rand`, swap it — both are non-crypto and good enough for D-87's "spread over 200ms" semantic. Document the choice in a WHY comment.
- The `_ = pgtype.Text` guard is a temporary hack to keep go vet quiet during draft iteration. Remove the line if `pgtype` is not actually imported (verify via grep before commit).
  </action>
  <acceptance_criteria>
    - `grep -q "^package state$" services/api/internal/state/ttl.go`
    - `grep -q "func (s \*Server) scheduleWrapUpExpiry(agentID, orgID uuid.UUID, until time.Time)" services/api/internal/state/ttl.go`
    - `grep -q "func (s \*Server) expireWrapUp(ctx context.Context, agentID, orgID uuid.UUID)" services/api/internal/state/ttl.go`
    - `grep -q "func (s \*Server) startupSweep(ctx context.Context) error" services/api/internal/state/ttl.go`
    - `grep -q "func (s \*Server) safetySweep()" services/api/internal/state/ttl.go`
    - `grep -q "func (s \*Server) runSweepPastDue()" services/api/internal/state/ttl.go`
    - `grep -q "db.WithBypass(ctx, \"wrapup_sweeper" services/api/internal/state/ttl.go` (Pitfall 8 mitigation)
    - `grep -q "s.clock.AfterFunc" services/api/internal/state/ttl.go`
    - `grep -q "s.clock.NewTicker" services/api/internal/state/ttl.go`
    - `grep -q "<-s.ctx.Done()" services/api/internal/state/ttl.go` (Pitfall 2 — early-exit on cancellation)
    - `grep -q "context.WithTimeout(context.Background(), 10\*time.Second)" services/api/internal/state/ttl.go` (Threat T-04-11 — bounded ctx)
    - `grep -q "state.ttl.expired" services/api/internal/state/ttl.go` (audit log)
    - `grep -q "state.ttl.expire.noop" services/api/internal/state/ttl.go` (DEBUG no-op)
    - handlers.go Start replaced: `grep -q "s.startupSweep(s.ctx)" services/api/internal/state/handlers.go`
    - handlers.go Stop replaced: `grep -q "s.timers.t\[id\].Stop" services/api/internal/state/handlers.go` OR `grep -q "t.Stop()" services/api/internal/state/handlers.go` inside the Stop() body
    - handlers.go Stop calls wg.Wait: `grep -q "s.wg.Wait" services/api/internal/state/handlers.go`
    - `cd services/api && go build ./internal/state/...` exits 0
    - `cd services/api && go vet ./internal/state/...` exits 0
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go build ./internal/state/... && go vet ./internal/state/... && grep -q "db.WithBypass(ctx, \"wrapup_sweeper" /Users/luong/workspace/dev/open-solutions/services/api/internal/state/ttl.go && grep -q "s.clock.AfterFunc" /Users/luong/workspace/dev/open-solutions/services/api/internal/state/ttl.go && grep -q "s.startupSweep" /Users/luong/workspace/dev/open-solutions/services/api/internal/state/handlers.go</automated>
  </verify>
  <done>ttl.go compiles with all 6 functions; handlers.go Start/Stop drive the sweeper; Pitfall 2 + 8 + T-04-11 mitigations encoded.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Write services/api/internal/state/ttl_test.go — 5 clockwork-driven tests (FiresOnExpiry, StartupSweep, SafetySweep, GracefulShutdown, RaceCondition)</name>
  <files>services/api/internal/state/ttl_test.go</files>
  <read_first>
    - services/api/internal/state/ttl.go (just-implemented production body)
    - services/api/internal/state/testutil_test.go (newTestHandlers, seedAgentStateRow helpers from Wave 2)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (Pitfall 9 — FakeClock advance + require.Eventually pattern)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 417-449 — TTL test cases)
    - .planning/phases/04-agent-state-machine-go/04-VALIDATION.md (test name mappings)
  </read_first>
  <behavior>
    - TestWrapUpTTL_FiresOnExpiry: seed agent + agent_state(status=WrapUp, wrapup_until=fakeNow+60s, post_interaction_state=ready); call Start(ctx); call scheduleWrapUpExpiry(agentID, orgID, fakeNow+60s); advance fakeClock 61s; assert via require.Eventually that DB row has status=Ready, state_version+=1, wrapup_until=NULL.
    - TestWrapUpTTL_StartupSweep_PastDue_FiresImmediately: seed agent + agent_state(status=WrapUp, wrapup_until=fakeNow-1h); call Start(ctx); BEFORE Start returns, the row should already be transitioned (synchronous sweep per D-95); assert directly after Start returns.
    - TestWrapUpTTL_StartupSweep_FutureSchedules: seed agent + agent_state(WrapUp, wrapup_until=fakeNow+10s); call Start(ctx); assert row still WrapUp immediately after Start; advance fakeClock 11s; require.Eventually that row transitioned.
    - TestWrapUpTTL_SafetySweep_FiresMissedTimer: seed WrapUp with wrapup_until=fakeNow+10s; call Start with WithSweepInterval(5*time.Second); skip scheduleWrapUpExpiry (simulate timer miss); advance fakeClock 11s (past due); then advance fakeClock 5s (triggers ticker); require.Eventually the safety sweep fires the expiry.
    - TestWrapUpTTL_GracefulShutdown_NoFireAfterStop: seed WrapUp wrapup_until=fakeNow+60s; call Start; call scheduleWrapUpExpiry; call Stop; advance fakeClock 61s (past expiry); WAIT for fakeClock to definitely have advanced; assert row STILL in WrapUp state (no transition fired because Stop drained timers + cancelled ctx before the AfterFunc body could execute the DB call).
    - TestWrapUpTTL_RaceCondition_IdempotentMultiFire: seed WrapUp; call Start; manually call expireWrapUp twice concurrently via t.Run subtests with t.Parallel(); assert state_version increments by exactly 1 (idempotent UPDATE per D-81 — only one firing wins).
    - All tests run cleanly under `go test -race`.
  </behavior>
  <action>
Write `services/api/internal/state/ttl_test.go`:

```go
package state

import (
    "context"
    "sync"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/jonboulle/clockwork"
    "github.com/stretchr/testify/require"
)

// TestWrapUpTTL_FiresOnExpiry — STATE-07 + Pitfall 9 pattern.
func TestWrapUpTTL_FiresOnExpiry(t *testing.T) {
    th := newTestHandlers(t) // newTestHandlers from testutil_test.go
    require.NotNil(t, th)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    cleanStateTables(t, ctx, th.Pool)

    fakeClock := clockwork.NewFakeClock()
    // Rebuild server with fakeClock and very short sweep interval
    // (irrelevant for this test — exercises AfterFunc path).
    s := New(Deps{OrgDB: th.S.deps.OrgDB, Cache: th.S.deps.Cache, Logger: th.S.deps.Logger},
        WithClock(fakeClock), WithSweepInterval(1*time.Hour /* sweep won't fire during test */))
    require.NoError(t, s.Start(ctx))
    defer s.Stop()

    agentID := uuid.Must(uuid.NewV7())
    seedAgent(t, th.Pool, th.OrgID, agentID, "a1", "Agent 1")
    wrapupUntil := fakeClock.Now().Add(60 * time.Second)
    pis := "ready"
    seedAgentStateRow(t, th.Pool, SeedStateParams{
        AgentID:              agentID,
        OrgID:                th.OrgID,
        Status:               "WrapUp",
        WrapupUntil:          &wrapupUntil,
        PostInteractionState: &pis,
        StateVersion:         3,
    })

    s.scheduleWrapUpExpiry(agentID, th.OrgID, wrapupUntil)
    fakeClock.Advance(61 * time.Second)

    require.Eventually(t, func() bool {
        row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
        return row.Status == "Ready" && row.StateVersion == 4 && !row.WrapupUntil.Valid
    }, 2*time.Second, 50*time.Millisecond, "WrapUp expiry never fired")
}

// TestWrapUpTTL_StartupSweep_PastDue_FiresImmediately — D-95 synchronous
// startup sweep contract.
func TestWrapUpTTL_StartupSweep_PastDue_FiresImmediately(t *testing.T) { /* ... */ }

// TestWrapUpTTL_StartupSweep_FutureSchedules — Start schedules an
// AfterFunc for future rows but does NOT fire-immediately.
func TestWrapUpTTL_StartupSweep_FutureSchedules(t *testing.T) { /* ... */ }

// TestWrapUpTTL_SafetySweep_FiresMissedTimer — simulate AfterFunc miss
// (e.g., process restart between schedule and fire) and assert the 30s
// ticker catches the past-due row.
func TestWrapUpTTL_SafetySweep_FiresMissedTimer(t *testing.T) { /* ... */ }

// TestWrapUpTTL_GracefulShutdown_NoFireAfterStop — Pitfall 2 regression
// gate: AfterFunc bodies that race Stop() must observe ctx.Done() and
// return WITHOUT issuing the DB UPDATE.
func TestWrapUpTTL_GracefulShutdown_NoFireAfterStop(t *testing.T) {
    th := newTestHandlers(t)
    require.NotNil(t, th)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    cleanStateTables(t, ctx, th.Pool)

    fakeClock := clockwork.NewFakeClock()
    s := New(Deps{OrgDB: th.S.deps.OrgDB, Cache: th.S.deps.Cache, Logger: th.S.deps.Logger},
        WithClock(fakeClock), WithSweepInterval(1*time.Hour))
    require.NoError(t, s.Start(ctx))

    agentID := uuid.Must(uuid.NewV7())
    seedAgent(t, th.Pool, th.OrgID, agentID, "a1", "Agent 1")
    wrapupUntil := fakeClock.Now().Add(60 * time.Second)
    pis := "ready"
    seedAgentStateRow(t, th.Pool, SeedStateParams{
        AgentID: agentID, OrgID: th.OrgID, Status: "WrapUp",
        WrapupUntil: &wrapupUntil, PostInteractionState: &pis, StateVersion: 3,
    })

    s.scheduleWrapUpExpiry(agentID, th.OrgID, wrapupUntil)
    s.Stop() // drain timers + cancel ctx

    fakeClock.Advance(61 * time.Second) // would fire if timer still alive

    // Wait long enough that a stray firing WOULD have landed.
    time.Sleep(100 * time.Millisecond)

    row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
    require.Equal(t, "WrapUp", row.Status, "Stop() must prevent post-Stop firings")
    require.Equal(t, int64(3), row.StateVersion, "no UPDATE should have landed after Stop")
}

// TestWrapUpTTL_RaceCondition_IdempotentMultiFire — D-81 idempotency.
func TestWrapUpTTL_RaceCondition_IdempotentMultiFire(t *testing.T) {
    th := newTestHandlers(t)
    require.NotNil(t, th)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    cleanStateTables(t, ctx, th.Pool)

    s := New(Deps{OrgDB: th.S.deps.OrgDB, Cache: th.S.deps.Cache, Logger: th.S.deps.Logger})
    require.NoError(t, s.Start(ctx))
    defer s.Stop()

    agentID := uuid.Must(uuid.NewV7())
    seedAgent(t, th.Pool, th.OrgID, agentID, "a1", "Agent 1")
    past := time.Now().Add(-1 * time.Hour)
    pis := "ready"
    seedAgentStateRow(t, th.Pool, SeedStateParams{
        AgentID: agentID, OrgID: th.OrgID, Status: "WrapUp",
        WrapupUntil: &past, PostInteractionState: &pis, StateVersion: 3,
    })

    // Fire concurrently from 10 goroutines. Idempotent UPDATE must
    // produce exactly one state change → state_version = 4 (not 13).
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            s.expireWrapUp(ctx, agentID, th.OrgID)
        }()
    }
    wg.Wait()

    row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
    require.Equal(t, "Ready", row.Status, "expected transition to Ready (post_interaction_state)")
    require.Equal(t, int64(4), row.StateVersion, "idempotent UPDATE must yield exactly one version bump")
}
```

For the omitted bodies (StartupSweep_PastDue, StartupSweep_FutureSchedules, SafetySweep_FiresMissedTimer), follow the same shape: setup → seed → action → fakeClock.Advance → require.Eventually on row state.

Add a helper `loadAgentStateRow(t, pool, orgID, agentID) generated.AgentState` inside ttl_test.go (or in testutil_test.go if not yet present) that runs `generated.New(pool).GetAgentStateByAgentId(...)` and returns the row.

Hazards:
- The `seedAgentStateRow` helper from Wave 2 testutil_test.go MUST support status=WrapUp + populated wrapup_until + post_interaction_state. Verify the helper signature matches `SeedStateParams` from Wave 2 Task 1.
- `require.Eventually` polls; do NOT use bare `time.Sleep` for assertion. The 2-second budget covers slow CI machines.
- `TestWrapUpTTL_GracefulShutdown_NoFireAfterStop` uses 100ms sleep AFTER Stop+Advance to give any stray firing a chance to land — if it lands, the assertion catches it. This is the only legitimate use of `time.Sleep` in this test file (it's a "wait to PROVE no event" pattern, not a "wait to OBSERVE event" pattern).
- The race-condition test uses 10 concurrent expireWrapUp calls. `go test -race` will flag any unsafe map access in the registry or in the production code.
- All tests use a fresh `Server` (NOT `th.S`) because they need a different sweepInterval for deterministic behavior.
  </action>
  <acceptance_criteria>
    - All 6 test functions declared:
      - `grep -q "^func TestWrapUpTTL_FiresOnExpiry" services/api/internal/state/ttl_test.go`
      - `grep -q "^func TestWrapUpTTL_StartupSweep_PastDue_FiresImmediately" services/api/internal/state/ttl_test.go`
      - `grep -q "^func TestWrapUpTTL_StartupSweep_FutureSchedules" services/api/internal/state/ttl_test.go`
      - `grep -q "^func TestWrapUpTTL_SafetySweep_FiresMissedTimer" services/api/internal/state/ttl_test.go`
      - `grep -q "^func TestWrapUpTTL_GracefulShutdown_NoFireAfterStop" services/api/internal/state/ttl_test.go`
      - `grep -q "^func TestWrapUpTTL_RaceCondition_IdempotentMultiFire" services/api/internal/state/ttl_test.go`
    - Tests use clockwork.FakeClock: `grep -q "clockwork.NewFakeClock" services/api/internal/state/ttl_test.go`
    - Tests use require.Eventually for assertions: `grep -q "require.Eventually" services/api/internal/state/ttl_test.go`
    - `cd services/api && go test -count=1 -v -run "TestWrapUpTTL" ./internal/state/...` exits 0 with all 6 tests passing
    - `cd services/api && go test -race -count=1 -run "TestWrapUpTTL" ./internal/state/...` exits 0 (race-clean)
    - `grep -q "^func loadAgentStateRow" services/api/internal/state/ttl_test.go` OR `grep -q "^func loadAgentStateRow" services/api/internal/state/testutil_test.go` (helper exists somewhere accessible to ttl_test.go)
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go test -count=1 -v -run "TestWrapUpTTL" ./internal/state/... && go test -race -count=1 -run "TestWrapUpTTL" ./internal/state/...</automated>
  </verify>
  <done>6 TTL tests pass; race-clean; clockwork.FakeClock drives deterministic firings; GracefulShutdown regression gate confirms Pitfall 2 mitigation.</done>
</task>

</tasks>

<verification>
- ttl.go compiles with 6 functions: scheduleWrapUpExpiry, expireWrapUp, startupSweep, safetySweep, runSweepPastDue, jitter
- handlers.go Start drives startupSweep + spawns safetySweep; Stop drains via wg.Wait + cancels all timers
- Pitfall 8 mitigation: every sweeper-side query is preceded by `db.WithBypass(ctx, "wrapup_sweeper.*")`
- Pitfall 2 mitigation: AfterFunc body checks `<-s.ctx.Done()` BEFORE any DB work
- T-04-11 mitigation: every AfterFunc body wraps in `context.WithTimeout(10s)` so hung DB calls don't leak goroutines
- 6 ttl_test.go tests pass under `go test -race`
- TestWrapUpTTL_GracefulShutdown_NoFireAfterStop proves Stop drains pending firings
- TestWrapUpTTL_RaceCondition_IdempotentMultiFire proves D-81 idempotent UPDATE wins exactly once
</verification>

<success_criteria>
- 2 tasks completed; 3 files modified (1 new ttl.go, 1 new ttl_test.go, 1 edited handlers.go)
- `cd services/api && go test -race -count=1 -run "TestWrapUpTTL" ./internal/state/...` exits 0
- The full state package suite still passes: `cd services/api && go test -count=1 ./internal/state/... ./internal/domain/...` exits 0
- Wave 4 has working Server.Start/Stop that can be wired into cmd/api/main.go
</success_criteria>

<output>
Create `.planning/phases/04-agent-state-machine-go/04-04-SUMMARY.md` when done. Include: list of files modified, output of `go test -race -count=1 ./internal/state/...`, confirmation that all 6 TTL tests pass + the graceful shutdown regression gate fires.
</output>
---
phase: 04-agent-state-machine-go
plan: 05
type: execute
wave: 4
depends_on:
  - 04-03
  - 04-04
files_modified:
  - services/api/cmd/api/main.go
  - services/api/internal/catalog/agents.go
  - services/api/internal/catalog/notimpl.go
  - services/api/internal/catalog/handlers.go
autonomous: true
requirements:
  - STATE-01

must_haves:
  truths:
    - "cmd/api/main.go declares ApiHandlers composite { *catalog.Handlers; *state.Server } (D-89 activation)"
    - "Compile-time assertion var _ api.StrictServerInterface = (*ApiHandlers)(nil) lives in main.go (moved from catalog/handlers.go)"
    - "stateServer.Start(ctx) runs BEFORE http.Server begins serving (D-95 synchronous startup sweep)"
    - "defer stateServer.Stop() runs at shutdown, BEFORE pool.Close + rdb.Close, with bounded ctx"
    - "catalog/notimpl.go no longer carries GetAgentStatus + PatchAgentStatus 501 stubs (state.Server owns them)"
    - "catalog/handlers.go removes var _ api.StrictServerInterface = (*Handlers)(nil) assertion (catalog alone no longer satisfies the full interface)"
    - "catalog/agents.go CreateAgent INSERTs the initial agent_states row INSIDE the existing OrgTx via qtx (D-93 + Pitfall 10)"
    - "WHY-comment in catalog/agents.go reminds Phase 5 bulk import to also seed agent_states (Pitfall 6)"
    - "Full `task gen && git diff --exit-code` is clean; full `go test ./...` passes"
  artifacts:
    - path: "services/api/cmd/api/main.go"
      provides: "ApiHandlers composite struct + stateServer construction + Start/Stop lifecycle wiring"
      contains: "type ApiHandlers struct"
    - path: "services/api/internal/catalog/agents.go"
      provides: "CreateAgent extended with qtx.InsertAgentState inside existing tx"
      contains: "qtx.InsertAgentState"
    - path: "services/api/internal/catalog/notimpl.go"
      provides: "GetAgentStatus + PatchAgentStatus 501 stubs deleted (state.Server owns them); bulk-import stubs untouched"
      contains: ""
    - path: "services/api/internal/catalog/handlers.go"
      provides: "Compile-time assertion removed (catalog alone no longer satisfies full interface)"
      contains: ""
  key_links:
    - from: "services/api/cmd/api/main.go"
      to: "services/api/internal/state/handlers.go"
      via: "state.New + stateServer.Start + defer stateServer.Stop"
      pattern: "state\\.New|stateServer\\.Start|stateServer\\.Stop"
    - from: "services/api/cmd/api/main.go"
      to: "services/api/internal/api/server.gen.go"
      via: "var _ api.StrictServerInterface = (*ApiHandlers)(nil) compile-time assertion"
      pattern: "var _ api.StrictServerInterface = \\(\\*ApiHandlers\\)\\(nil\\)"
    - from: "services/api/internal/catalog/agents.go"
      to: "services/api/internal/db/generated/agent_states.sql.go"
      via: "qtx.InsertAgentState atomically inside existing CreateAgent tx (D-93)"
      pattern: "qtx\\.InsertAgentState"
---

<objective>
Activate the D-89 composite `ApiHandlers` struct in `cmd/api/main.go` (`{ *catalog.Handlers; *state.Server }`); wire `stateServer.Start(ctx)` and `defer stateServer.Stop()` into the existing main.go startup/shutdown sequence; delete the now-redundant `GetAgentStatus` + `PatchAgentStatus` 501 stubs from `catalog/notimpl.go`; move the `var _ api.StrictServerInterface = (*Handlers)(nil)` compile-time assertion from `catalog/handlers.go` to `cmd/api/main.go` as `var _ api.StrictServerInterface = (*ApiHandlers)(nil)`; extend `catalog/agents.go` `CreateAgent` to INSERT the initial `agent_states` row atomically inside the existing `OrgTx` (D-93) using `qtx` (NOT `generated.New(h.deps.OrgDB)` — Pitfall 10); add a WHY-comment reminding Phase 5 bulk import to also seed agent_states (Pitfall 6).

Purpose: This is the production wiring that connects everything from Waves 1-3 into the running binary. After Wave 4, the API server boots with a working state machine, WrapUp sweeper, and atomic agent+state creation. Wave 5 then adds cross-org isolation tests + acceptance suite + the cross-AI peer review.

Output: All production wiring; full test suite green; codegen-drift CI clean; CreateAgent atomically seeds agent_states; Phase 5 inheritance reminder embedded in code.

**Hazards captured here:**
- **Pitfall 1 (already mitigated in Wave 1)**: state package type is `Server`, so the composite compiles without rename work in this wave.
- **Pitfall 10 (Hazard 8 in PATTERNS.md)**: CreateAgent extension MUST use `qtx` (the tx-bound Queries built at agents.go line 113), NOT a fresh `generated.New(h.deps.OrgDB)`. The existing skills replace already uses `qtx`; the new InsertAgentState call MUST do the same so agent + state INSERT commit/rollback atomically.
- **Pitfall 6 (Hazard 7 in PATTERNS.md)**: Phase 5 bulk import MUST also seed agent_states. Capture as a `// D-93: Phase 5 bulk import MUST also seed agent_states for every imported agent.` comment at the InsertAgentState call site.
- **Hazard 2 (PATTERNS.md):** `var _ api.StrictServerInterface = (*Handlers)(nil)` on catalog/handlers.go line 92 MUST be REMOVED in the SAME commit that deletes the notimpl stubs. Splitting across commits breaks the intermediate-state `go build`.
</objective>

<execution_context>
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/workflows/execute-plan.md
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@./CLAUDE.md
@.planning/phases/04-agent-state-machine-go/04-CONTEXT.md
@.planning/phases/04-agent-state-machine-go/04-RESEARCH.md
@.planning/phases/04-agent-state-machine-go/04-PATTERNS.md
@.planning/phases/04-agent-state-machine-go/04-VALIDATION.md
@.planning/phases/04-agent-state-machine-go/04-03-PLAN.md
@.planning/phases/04-agent-state-machine-go/04-03-SUMMARY.md
@.planning/phases/04-agent-state-machine-go/04-04-PLAN.md
@.planning/phases/04-agent-state-machine-go/04-04-SUMMARY.md
@services/api/cmd/api/main.go
@services/api/internal/catalog/handlers.go
@services/api/internal/catalog/agents.go
@services/api/internal/catalog/notimpl.go
@services/api/internal/state/handlers.go
@services/api/internal/state/agent_states.go
</context>

<interfaces>
From `services/api/internal/state/handlers.go`:
```go
type Deps struct { OrgDB *db.OrgDB; Cache *cache.Cache; Logger *slog.Logger; WrapUpDuration time.Duration }
type Server struct { ... }
func New(deps Deps, opts ...Option) *Server
func (s *Server) Start(ctx context.Context) error
func (s *Server) Stop()
func (s *Server) GetAgentStatus(...)
func (s *Server) PatchAgentStatus(...)
```

From `services/api/internal/catalog/handlers.go`:
```go
type Handlers struct { ... }
func New(deps Deps, opts ...Option) *Handlers
// 41 methods implementing the catalog endpoints + state stubs (to be removed)
```

From `services/api/cmd/api/main.go` (current):
```go
catalogCache := cache.New(rdb, slog.Default())
catalogHandlers := catalog.New(catalog.Deps{ OrgDB, Pool, Cache, Logger })
mux := server.NewMux(&server.Deps{
    Pool, Redis, OrgDB, Config, StrictHandlers: catalogHandlers, SpecBytes
})
```

Phase 4 wiring (post this plan):
```go
catalogHandlers := catalog.New(...)
stateServer := state.New(state.Deps{...}, state.WithClock(clockwork.NewRealClock()))
if err := stateServer.Start(ctx); err != nil { return 1 }
defer stateServer.Stop()
type ApiHandlers struct { *catalog.Handlers; *state.Server }
var _ api.StrictServerInterface = (*ApiHandlers)(nil)
apiHandlers := &ApiHandlers{ Handlers: catalogHandlers, Server: stateServer }
mux := server.NewMux(&server.Deps{ ..., StrictHandlers: apiHandlers, ... })
```
</interfaces>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| HTTP request → ApiHandlers method dispatch | Go's embed-method-set resolution dispatches to either catalog.Handlers or state.Server based on method name. No ambiguity because the two embed sets are disjoint (catalog owns 41 endpoints; state owns 2 status endpoints). |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-04-13 | Tampering | CreateAgent agent_states atomicity | mitigate | InsertAgentState runs inside existing OrgTx via qtx; defer tx.Rollback ensures any failure rolls back the agent row too (Codex C4 atomic invariant). |
| T-04-14 | Denial of Service | Sweeper graceful shutdown ordering | mitigate | main.go shutdown order: http.Server.Shutdown → defer stateServer.Stop → defer rdb.Close → defer pool.Close → defer telemetry shutdown. Sweeper drains BEFORE pool closes so in-flight UPDATEs complete. |
</threat_model>

<tasks>

<task type="auto">
  <name>Task 1: Extend services/api/internal/catalog/agents.go CreateAgent — INSERT initial agent_states row inside existing tx (D-93 + Pitfall 10)</name>
  <files>services/api/internal/catalog/agents.go</files>
  <read_first>
    - services/api/internal/catalog/agents.go (full CreateAgent function — lines 71-173; specifically the existing Codex C4 tx at lines 103-156)
    - services/api/internal/db/generated/agent_states.sql.go (InsertAgentState signature from Wave 1)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-93 atomic INSERT)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 731-762 — D-93 modification site)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (§Pattern 4)
  </read_first>
  <action>
Open `services/api/internal/catalog/agents.go`. Locate `CreateAgent` (line 71). Inside its existing OrgTx block (which begins around line 103 with `tx, err := h.deps.OrgDB.BeginTx(ctx)` and continues through `tx.Commit(ctx)`), find the call to `qtx.InsertAgent(...)` that successfully returns a row. Insert a NEW step BETWEEN `qtx.InsertAgent` (the agent row creation) and the optional `h.replaceAgentSkills(ctx, qtx, ...)` call (the skills replace block).

The insertion site is precisely: after the `qtx.InsertAgent` call's `if err != nil` branch handles the agent INSERT failure paths (409 dup external_id, 422 invalid_value, 500 internal), and BEFORE the `if req.Body.Skills != nil` skills-replace branch.

Insert this block (using `qtx` — Pitfall 10 — NOT `generated.New(h.deps.OrgDB)`):

```go
// D-93: seed agent_states row inside the SAME tx so agent + state
// commit/rollback atomically (Codex C4 invariant). InsertAgentState
// MUST go through qtx (not a fresh generated.New on the pool) or the
// state row commits independently of the agent row and breaks the
// invariant on failure paths (Pitfall 10).
//
// Phase 5 reminder: bulk-import (CSV/JSON upsert into agents) MUST also
// seed agent_states for every NEW agent it inserts. Use
// `ON CONFLICT (agent_id) DO NOTHING` so re-imports don't regress the
// state machine (Pitfall 6 / Hazard 7 in 04-PATTERNS.md).
if _, sErr := qtx.InsertAgentState(ctx, generated.InsertAgentStateParams{
    AgentID: pgUUID(id),
    OrgID:   pgUUID(orgID),
    Status:  string(api.AgentStatusOffline),
}); sErr != nil {
    h.deps.Logger.ErrorContext(ctx, "create agent state row", "agent_id", id, "org_id", orgID, "err", sErr)
    return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
        Error:  api.ErrorCodeInternal,
        Reason: "agent_state_insert_failed",
    }}, nil
}
```

Hazards:
- The `qtx` variable is built earlier in the function as `qtx := generated.New(tx)`. Verify this variable exists in scope at the insertion site (it does — the existing skills replace already calls `qtx.<X>` patterns).
- The InsertAgentState SQL from Wave 1 has 3 explicit params (`agent_id`, `org_id`, `status`) plus the implicit `state_version = 1` default in SQL. The Go struct `InsertAgentStateParams` from sqlc reflects only the explicit `$1..$3` params. The `state_version` defaults to 1 via the SQL `VALUES ($1, $2, $3, 1)` literal in agent_states.sql.
- `pgUUID(id)` helper is the same one used by other catalog handlers — already in scope via `catalog/mappers.go`.
- DO NOT add an `id` parameter — agent_id IS the agent's primary key (PK collision with PK of agents table is intentional per D-78 schema: agent_states.agent_id is a 1-to-1 reference to agents.id).
- DO NOT use `if err := ...; err != nil` style — the pattern in agents.go uses `if _, sErr := ...; sErr != nil { ... return }`. Match the existing style.
- The reason string `"agent_state_insert_failed"` is distinct from any existing reason in the codebase (verify via grep). Generic enough to not leak schema details to the client.
- Adding this INSERT does NOT change the rollback path — the `defer tx.Rollback(ctx)` at the top of the function (or its equivalent) handles failure cleanup for all three steps (agent, state, skills).
  </action>
  <acceptance_criteria>
    - `grep -q "qtx.InsertAgentState" services/api/internal/catalog/agents.go`
    - `grep -q "D-93" services/api/internal/catalog/agents.go` (decision reference)
    - `grep -q "Pitfall 10" services/api/internal/catalog/agents.go` OR `grep -q "qtx (not a fresh" services/api/internal/catalog/agents.go` (Pitfall 10 reminder comment)
    - `grep -q "Phase 5" services/api/internal/catalog/agents.go` (Pitfall 6 reminder)
    - `grep -q "bulk-import\|bulk import" services/api/internal/catalog/agents.go` (Phase 5 inheritance reminder)
    - `grep -q "ON CONFLICT (agent_id) DO NOTHING" services/api/internal/catalog/agents.go` (Phase 5 reminder includes the exact ON CONFLICT clause)
    - The insertion is INSIDE the existing tx: `awk '/qtx := generated.New(tx)/,/tx.Commit\(ctx\)/' services/api/internal/catalog/agents.go | grep -q "qtx.InsertAgentState"`
    - The InsertAgentState uses `qtx` (NOT `generated.New(h.deps.OrgDB)`): `grep -B2 -A6 "qtx.InsertAgentState" services/api/internal/catalog/agents.go | grep -vc "generated.New(h.deps.OrgDB)"` returns nonzero (no occurrence of the bad pattern adjacent to the call)
    - `grep -q "agent_state_insert_failed" services/api/internal/catalog/agents.go`
    - `cd services/api && go build ./...` exits 0
    - `cd services/api && go vet ./...` exits 0
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go build ./... && go vet ./... && grep -q "qtx.InsertAgentState" /Users/luong/workspace/dev/open-solutions/services/api/internal/catalog/agents.go && grep -q "Phase 5" /Users/luong/workspace/dev/open-solutions/services/api/internal/catalog/agents.go && grep -q "ON CONFLICT (agent_id) DO NOTHING" /Users/luong/workspace/dev/open-solutions/services/api/internal/catalog/agents.go</automated>
  </verify>
  <done>CreateAgent seeds initial agent_states row inside existing tx via qtx; Phase 5 inheritance reminder embedded; build clean.</done>
</task>

<task type="auto">
  <name>Task 2: Remove state stubs from catalog/notimpl.go AND remove compile-time assertion from catalog/handlers.go (single atomic commit)</name>
  <files>
    services/api/internal/catalog/notimpl.go,
    services/api/internal/catalog/handlers.go
  </files>
  <read_first>
    - services/api/internal/catalog/notimpl.go (full file — lines 40-50 are the two state stubs)
    - services/api/internal/catalog/handlers.go (line 92 — var _ api.StrictServerInterface = (*Handlers)(nil) assertion)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (Hazard 2 lines 1217-1224)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (Claude's Discretion — notimpl cleanup)
  </read_first>
  <action>
This task MUST be a single atomic edit covering BOTH files in one commit. Splitting across commits leaves the intermediate state with a broken `go build` (catalog.Handlers no longer satisfies the interface AFTER removing stubs but BEFORE moving the assertion).

Step 1: Edit `services/api/internal/catalog/notimpl.go`. DELETE the two methods `GetAgentStatus` and `PatchAgentStatus` (around lines 40-50). KEEP every other method (`BulkImportCatalog`, `GetImportJob`, the `notImplementedBody()` helper, any others). The file now reads:

```go
package catalog

// Phase 4 deleted: GetAgentStatus + PatchAgentStatus 501 stubs.
// state.Server owns these methods; cmd/api/main.go composes
// *catalog.Handlers + *state.Server into ApiHandlers (D-89).
//
// Remaining stubs are Phase 5 (bulk import) — kept until Phase 5 lands.

import (
    "context"

    "github.com/luongdev/open-routing/services/api/internal/api"
)

// BulkImportCatalog — Phase 5. ...
func (h *Handlers) BulkImportCatalog(...) { ... return notImplemented(api.BulkImportCatalog500JSONResponse) ... }

// GetImportJob — Phase 5. ...
func (h *Handlers) GetImportJob(...) { ... }

// notImplementedBody returns the canonical 500 reason payload for the
// remaining Phase 5 stubs. Once Phase 5 ships, this file is deleted.
func notImplementedBody() api.InternalServerErrorJSONResponse {
    return api.InternalServerErrorJSONResponse{
        Error:  api.ErrorCodeInternal,
        Reason: "not_implemented",
    }
}
```

Preserve the exact existing structure of the bulk import stubs — only the two state methods are deleted.

Step 2: Edit `services/api/internal/catalog/handlers.go`. Remove line 92 entirely:

```go
// DELETE THIS LINE:
var _ api.StrictServerInterface = (*Handlers)(nil)
```

Add a comment in its place explaining the move:

```go
// The compile-time StrictServerInterface assertion lives on the
// *ApiHandlers composite in cmd/api/main.go (D-89): catalog.Handlers
// alone owns 41/43 methods; state.Server contributes the 2 status
// methods to make the composite satisfy the full interface.
```

Step 3: Verify `go build` STILL fails (until Task 3 wires the composite in main.go). This is expected — `go build` will fail with "catalog.Handlers does not implement StrictServerInterface (missing GetAgentStatus)" because the assertion has been removed but main.go still passes `catalogHandlers` (a `*catalog.Handlers`) as `StrictHandlers`. Task 3 fixes this.

If for some reason `go build` succeeds after Step 1 + Step 2 alone, then the composite must already be wired — verify Task 3 is not yet executed (it shouldn't be — Task 3 is sequential AFTER this one).

Hazards:
- The `notImplementedBody()` helper MIGHT be unused if both state methods are gone AND no remaining Phase 5 stub uses it. Verify after deletion via `grep -c notImplementedBody services/api/internal/catalog/notimpl.go`. If it returns 1 (only the definition), delete the helper too. If it returns 2+ (definition + usages), keep it.
- The notimpl.go header comment from lines 16-23 references "Phase 4 deletes GetAgentStatus + PatchAgentStatus" — update this comment to past tense ("Phase 4 has deleted") OR remove the now-obsolete reference. The comment is now historical, not predictive.
- DO NOT delete BulkImportCatalog or GetImportJob — those belong to Phase 5.
- Step 2 removes line 92 of catalog/handlers.go. If the file has been modified since the Phase 3 commit (e.g., line numbers shifted), use `grep -n "var _ api.StrictServerInterface = (*Handlers)(nil)"` to find the exact line and remove THAT line.
  </action>
  <acceptance_criteria>
    - `grep -c "GetAgentStatus\|PatchAgentStatus" services/api/internal/catalog/notimpl.go` returns 0 (state stubs deleted) — note: this grep counts both methods so it should return 0 lines after deletion
    - `grep -c "func (h \*Handlers) GetAgentStatus" services/api/internal/catalog/notimpl.go` returns 0
    - `grep -c "func (h \*Handlers) PatchAgentStatus" services/api/internal/catalog/notimpl.go` returns 0
    - `grep -c "BulkImportCatalog\|GetImportJob" services/api/internal/catalog/notimpl.go` returns at least 2 (Phase 5 stubs preserved)
    - `grep -c "var _ api.StrictServerInterface = (\*Handlers)(nil)" services/api/internal/catalog/handlers.go` returns 0 (assertion moved)
    - Task 3 wires the composite — by the END of Task 3 the build is clean; this task alone leaves the build in an INTENTIONAL broken state (asserted via the next-task verify step).
  </acceptance_criteria>
  <verify>
    <automated>[ "$(grep -c 'func (h \*Handlers) GetAgentStatus' /Users/luong/workspace/dev/open-solutions/services/api/internal/catalog/notimpl.go)" -eq 0 ] && [ "$(grep -c 'func (h \*Handlers) PatchAgentStatus' /Users/luong/workspace/dev/open-solutions/services/api/internal/catalog/notimpl.go)" -eq 0 ] && [ "$(grep -c 'var _ api.StrictServerInterface = (\*Handlers)(nil)' /Users/luong/workspace/dev/open-solutions/services/api/internal/catalog/handlers.go)" -eq 0 ]</automated>
  </verify>
  <done>Stubs deleted; assertion removed; build is intentionally broken until Task 3 wires composite — confirmed by failing go build at this point.</done>
</task>

<task type="auto">
  <name>Task 3: Wire ApiHandlers composite + stateServer construction + Start/Stop into cmd/api/main.go (D-89 activation)</name>
  <files>services/api/cmd/api/main.go</files>
  <read_first>
    - services/api/cmd/api/main.go (full file, particularly lines 137-156 — current catalog.New + NewMux wiring; and lines 100-115 — defer cleanup chain)
    - services/api/internal/state/handlers.go (state.New + Start + Stop signatures)
    - services/api/internal/catalog/handlers.go (post Task 2 — no longer has the assertion)
    - services/api/internal/api/server.gen.go (StrictServerInterface definition)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-89 ApiHandlers exact shape)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 867-945 — main.go wiring + ordering hazards)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (§Pattern 7 lines 714-726)
  </read_first>
  <action>
Edit `services/api/cmd/api/main.go`. Insert a new step (8.5) between the existing step (8) `catalogHandlers := catalog.New(...)` and step (9) `mux := server.NewMux(...)`.

Add imports at the top of the file:
- `"github.com/jonboulle/clockwork"` (for the real clock)
- `"github.com/luongdev/open-routing/services/api/internal/state"` (the new package)

Add the wiring after `catalogHandlers := catalog.New(...)`:

```go
// (8.5) state.Server — D-89 composite activation. Phase 4 introduces a
// separate state-machine package; main.go merges it with catalog via
// anonymous embedding. Server (NOT Handlers) per Pitfall 1.
stateServer := state.New(state.Deps{
    OrgDB:  orgDB,
    Cache:  catalogCache, // SAME cache instance — single Redis client
    Logger: slog.Default(),
}, state.WithClock(clockwork.NewRealClock()))

// Synchronous startup sweep — guarantees no stuck WrapUp survives a
// restart (D-95). Failure aborts process startup so kubernetes restarts
// with a fresh attempt.
if err := stateServer.Start(ctx); err != nil {
    slog.ErrorContext(ctx, "state server start", "err", err)
    return 1
}
defer stateServer.Stop()

// ApiHandlers composite — D-70 forward-compat seam activated for Phase 4.
// catalog.Handlers contributes 41 methods (CRUD for 6 entities + scaffold);
// state.Server contributes 2 status methods. The two embed sets are
// disjoint — Go's method-set resolution merges them cleanly. Pitfall 1
// is avoided by naming the state type Server (not Handlers).
type ApiHandlers struct {
    *catalog.Handlers
    *state.Server
}
// Compile-time guarantee that the COMPOSITE satisfies the full
// StrictServerInterface. If this line fails to compile, either:
//  (a) the OpenAPI spec gained an endpoint with no implementation, OR
//  (b) one of the embedded types lost a method (e.g., catalog/notimpl.go
//      lost a stub it shouldn't have).
// In Phase 4 this assertion replaces the one removed from
// catalog/handlers.go line 92 (catalog alone no longer satisfies the
// full interface).
var _ api.StrictServerInterface = (*ApiHandlers)(nil)

apiHandlers := &ApiHandlers{
    Handlers: catalogHandlers,
    Server:   stateServer,
}
```

Then change the existing step (9) `server.NewMux(&server.Deps{...})` call's `StrictHandlers: catalogHandlers` to `StrictHandlers: apiHandlers`.

Verify the deferred Stop ordering: the existing `defer pool.Close()` at line ~103 and `defer func() { _ = rdb.Close() }()` at line ~113 run in LIFO order. The `defer stateServer.Stop()` we just added at step (8.5) runs BEFORE both of those (Go defers run in reverse declaration order). This means at shutdown the order is:

1. `<-ctx.Done()` → http.Server.Shutdown (in-flight requests drain)
2. `stateServer.Stop()` (sweeper drains, AfterFunc timers cancelled)
3. `_ = rdb.Close()` (Redis client closes)
4. `pool.Close()` (Postgres pool closes)

This ordering is correct — sweeper UPDATEs need the pool open. Add a comment near the deferred Stop:

```go
// Shutdown order (LIFO):
//   1. http.Server.Shutdown drains in-flight HTTP requests
//   2. stateServer.Stop drains sweeper goroutine + cancels AfterFunc timers
//      (must precede pool.Close — sweeper UPDATEs need the pool open)
//   3. rdb.Close + pool.Close + telemetry shutdown
defer stateServer.Stop()
```

Hazards:
- The `type ApiHandlers struct` declaration inside `run()` is a LOCAL type. Go allows this — the type is visible only inside the function. If main.go's existing style declares types at file scope (it doesn't currently), keep this local for now. If a future PR moves it to `internal/api/composite.go`, the move is mechanical.
- The order of fields in `&ApiHandlers{Handlers: catalogHandlers, Server: stateServer}` matters for readability but not semantics — Go's embedded-field syntax uses the type name as the field name (`Handlers` for `*catalog.Handlers`, `Server` for `*state.Server`). The struct literal must use these names.
- The `clockwork.NewRealClock()` is the production clock. Tests use `WithClock(fakeClock)` instead.
- After this edit, `go build ./...` MUST succeed — this is the moment the intermediate-broken-state from Task 2 heals.
- The `defer stateServer.Stop()` follows immediately after `stateServer.Start(ctx)` so a return-1 from Start does NOT register the defer (otherwise Stop() runs on an uninitialised state, which the idempotency guard handles but is wasted work).

Verify after the edit:
1. `cd services/api && go build ./...` exits 0 — proves composite satisfies StrictServerInterface.
2. `cd services/api && go test -count=1 ./...` exits 0 — proves no regression.
3. `cd services/api && task gen && git diff --exit-code` exits 0 — proves codegen still in sync.
  </action>
  <acceptance_criteria>
    - `grep -q "stateServer := state.New" services/api/cmd/api/main.go`
    - `grep -q "stateServer.Start(ctx)" services/api/cmd/api/main.go`
    - `grep -q "defer stateServer.Stop()" services/api/cmd/api/main.go`
    - `grep -q "type ApiHandlers struct" services/api/cmd/api/main.go`
    - `grep -q "\*catalog.Handlers" services/api/cmd/api/main.go`
    - `grep -q "\*state.Server" services/api/cmd/api/main.go`
    - `grep -q "var _ api.StrictServerInterface = (\*ApiHandlers)(nil)" services/api/cmd/api/main.go`
    - `grep -q "StrictHandlers: apiHandlers" services/api/cmd/api/main.go`
    - `grep -cq "StrictHandlers: catalogHandlers" services/api/cmd/api/main.go` returns 0 (the old wiring is REPLACED)
    - `grep -q "clockwork.NewRealClock" services/api/cmd/api/main.go`
    - The defer ordering comment is present: `grep -q "Shutdown order" services/api/cmd/api/main.go`
    - `cd services/api && go build ./...` exits 0 (build is now whole again)
    - `cd services/api && go vet ./...` exits 0
    - `cd services/api && go test -count=1 ./...` exits 0 — full regression suite passes
    - `cd services/api && task gen && cd ../.. && cd web && pnpm gen:api && cd .. && git diff --exit-code` exits 0 — codegen-drift CI gate green
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go build ./... && go vet ./... && go test -count=1 ./... && task gen && cd ../.. && cd web && pnpm gen:api && cd .. && git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/generated.ts openapi/openapi.yaml</automated>
  </verify>
  <done>main.go wires ApiHandlers composite; stateServer Start/Stop in shutdown chain; full regression suite passes; codegen-drift CI gate green.</done>
</task>

</tasks>

<verification>
- ApiHandlers composite declared with `*catalog.Handlers + *state.Server` (Pitfall 1 avoided because Wave 1 already named the type Server)
- Compile-time assertion `var _ api.StrictServerInterface = (*ApiHandlers)(nil)` lives in main.go (moved from catalog/handlers.go in same wave/commit set)
- stateServer.Start(ctx) runs synchronously BEFORE http.Server begins (D-95)
- defer stateServer.Stop() registered AFTER successful Start (so failed-Start doesn't defer Stop on uninitialised state)
- Shutdown ordering: http.Server.Shutdown → stateServer.Stop → rdb.Close → pool.Close (LIFO via defer)
- catalog/notimpl.go no longer carries GetAgentStatus + PatchAgentStatus 501 stubs
- catalog/handlers.go no longer carries the assertion (moved to main.go)
- catalog/agents.go CreateAgent INSERTs initial agent_states row INSIDE existing tx via qtx (Pitfall 10 mitigated)
- WHY-comments encode Pitfall 6 (Phase 5 inheritance) and Pitfall 10 (qtx not raw pool)
- Full regression suite still passes (`go test ./...`)
- Codegen-drift CI gate still green (`task gen && git diff --exit-code`)
</verification>

<success_criteria>
- 3 tasks completed; 4 files modified
- `cd services/api && go test -count=1 ./...` exits 0 (full regression)
- `cd services/api && task gen && git diff --exit-code` exits 0 (codegen-drift gate)
- CreateAgent atomically seeds agent_states (verified by Wave 5 isolation test `TestCreateAgentSeedsState`)
- WrapUp sweeper boots via main.go (verified by Wave 5 acceptance test)
- Phase 5 inheritance reminder embedded in catalog/agents.go comment
</success_criteria>

<output>
Create `.planning/phases/04-agent-state-machine-go/04-05-SUMMARY.md` when done. Include: diff snippet showing the inserted ApiHandlers composite, output of `go test ./...`, confirmation of `task gen && git diff --exit-code` clean, list of files modified.
</output>
---
phase: 04-agent-state-machine-go
plan: 06
type: execute
wave: 5
depends_on:
  - 04-05
files_modified:
  - services/api/test/isolation/state_test.go
  - services/api/internal/state/agent_states_test.go
  - services/api/internal/state/ttl_test.go
  - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md
autonomous: false
requirements:
  - STATE-01
  - STATE-02
  - STATE-03
  - STATE-04
  - STATE-07
  - STATE-08
  - STATE-10

must_haves:
  truths:
    - "D-94 cross-org isolation suite at services/api/test/isolation/state_test.go covers 4 probes: GET cross-org → 404; PATCH cross-org break_reason_id → 422; PATCH cross-org agent → 404; PATCH cross-org break_reason_id with force=true → still 422 (Pitfall 3 regression gate)"
    - "All 5 ROADMAP Phase 4 success criteria have corresponding acceptance tests covering allowed transitions, break_reason validation, WrapUp TTL survives disconnect, IsRoutable matrix, state_version monotonic"
    - "CreateAgent → agent_states atomic seeding verified via TestCreateAgentSeedsState (regression gate for D-93 + Phase 5 inheritance)"
    - "Cross-AI peer review (Codex + Gemini in parallel per project CLAUDE.md HARD RULE) runs on the full Phase 4 diff; any HIGH-severity finding is addressed before declaring Wave 5 done"
    - "PATTERNS.md gains a `Phase 5 Inheritance Reminder` section documenting bulk-import duty to seed agent_states (Pitfall 6)"
  artifacts:
    - path: "services/api/test/isolation/state_test.go"
      provides: "4 cross-org probes (D-94) + CreateAgent-seeds-state regression test"
      contains: "TestState_GetStatusCrossOrg_404"
    - path: "services/api/internal/state/agent_states_test.go"
      provides: "5 acceptance tests covering ROADMAP success criteria #1-#5"
      contains: "TestAcceptance_AllTransitions"
    - path: ".planning/phases/04-agent-state-machine-go/04-PATTERNS.md"
      provides: "Appended `Phase 5 Inheritance Reminder` section (Pitfall 6 mitigation)"
      contains: "Phase 5 Inheritance Reminder"
  key_links:
    - from: "services/api/test/isolation/state_test.go"
      to: "services/api/test/isolation/catalog_test.go"
      via: "Helpers (freshOrg, baseURL, postEntity, requireContainer) reused; new helpers (getStatusEndpoint, patchStatusReturnCode) added to state_test.go"
      pattern: "freshOrg\\(t\\)|baseURL\\(\\)|requireContainer\\(t\\)"
---

<objective>
Close Phase 4 with: (a) the D-94 cross-org isolation test suite at `services/api/test/isolation/state_test.go` exercising the 4 acceptance probes from CONTEXT.md (GET cross-org 404, PATCH cross-org break_reason 422, PATCH cross-org agent 404, PATCH cross-org break_reason with force=true STILL 422 — the Pitfall 3 regression gate); (b) 5 acceptance tests in `internal/state/agent_states_test.go` covering each ROADMAP Phase 4 success criterion; (c) a `TestCreateAgentSeedsState` regression test proving D-93 atomic seeding works through the catalog handler (verifying Wave 4's Task 1 wire-up survives); (d) the project-CLAUDE.md HARD-RULE cross-AI peer review (Codex + Gemini in parallel) on the full Phase 4 diff with any HIGH-severity findings addressed before the wave is declared done; (e) a `Phase 5 Inheritance Reminder` section appended to `04-PATTERNS.md` so the next phase's planner can't forget Pitfall 6.

Purpose: ROADMAP Phase 4 success criteria are the contract. Wave 5 proves every one of them holds end-to-end through the running mux (not just via package-level unit tests). The cross-AI review is the final gate per project CLAUDE.md HARD RULE — it has caught 6 BLOCKER findings in Phase 3 plan review that the in-house plan-checker and Gemini both missed. The Phase 5 inheritance reminder ensures Pitfall 6 doesn't recur.

Output: 4 isolation tests + 5 acceptance tests + 1 atomic-seeding regression test all green; cross-AI peer review run with findings addressed; PATTERNS.md inheritance reminder; full Phase 4 ready for /gsd-verify-work.

**Hazards captured here:**
- **Pitfall 3 regression gate (Hazard 4 in PATTERNS.md):** `TestState_ForceDoesNotBypassCrossOrgBreakReason` is the canonical regression test. If it ever fails post-this-wave, force=true is leaking across orgs.
- **Pitfall 6 reminder (Hazard 7):** the `Phase 5 Inheritance Reminder` section MUST be appended to PATTERNS.md so the next phase's planner can't omit agent_states seeding in bulk import.
- **Cross-AI review HARD RULE:** per project CLAUDE.md, the review runs in parallel (Codex + Gemini) and any reviewer BLOCK verdict prevents declaring the wave done.
</objective>

<execution_context>
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/workflows/execute-plan.md
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@./CLAUDE.md
@./AGENTS.md
@.planning/phases/04-agent-state-machine-go/04-CONTEXT.md
@.planning/phases/04-agent-state-machine-go/04-RESEARCH.md
@.planning/phases/04-agent-state-machine-go/04-PATTERNS.md
@.planning/phases/04-agent-state-machine-go/04-VALIDATION.md
@.planning/phases/04-agent-state-machine-go/04-05-PLAN.md
@.planning/phases/04-agent-state-machine-go/04-05-SUMMARY.md
@services/api/test/isolation/catalog_test.go
@services/api/test/isolation/main_test.go
@services/api/internal/state/agent_states.go
@services/api/internal/state/agent_states_test.go
@services/api/internal/state/testutil_test.go
@services/api/internal/state/ttl_test.go
@services/api/cmd/api/main.go
</context>

<interfaces>
From `services/api/test/isolation/catalog_test.go` (Phase 3 — analog template):
```go
func freshOrg(t testing.TB) uuid.UUID  // generates UUIDv7 per test
func baseURL() string                  // sharedSrv.URL — from main_test.go
func requireContainer(t testing.TB)    // skip if docker testcontainer unavailable
func postEntity(t testing.TB, urlBase, entityPath string, orgID uuid.UUID, body any) (statusCode int, idOrEmpty uuid.UUID)
func getEntityStatus(t testing.TB, urlBase, entityPath string, orgID uuid.UUID, id uuid.UUID) int  // returns statusCode
```

From `services/api/internal/api/types.gen.go`:
```go
type ErrorCode string
const ErrorCodeInvalidReference ErrorCode = "invalid_reference"
const ErrorCodeInvalidTransition ErrorCode = "invalid_transition"

type ErrorResponse struct { Error ErrorCode; Reason string; RequestId *openapi_types.UUID }
type InvalidTransitionErrorResponse struct { Error ErrorCode; From AgentStatus; To AgentStatus; RequestId *openapi_types.UUID }
```
</interfaces>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Cross-org HTTP request | Every isolation test seeds two distinct orgs (orgA, orgB) and verifies orgA's data is invisible to orgB regardless of admin-flag (force=true) usage. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-04-15 | Information Disclosure | Cross-org leak via PATCH /status | mitigate | 4 D-94 isolation probes; FOUND-08 invariant. Pitfall 3 regression test gates the force=true + cross-org path. |
| T-04-16 | Tampering | CreateAgent without atomic agent_states seed | mitigate | TestCreateAgentSeedsState verifies D-93. Phase 5 inheritance reminder appended to PATTERNS.md. |
| T-04-SC-final | Tampering | Full Phase 4 diff | mitigate | Cross-AI review (Codex + Gemini parallel per CLAUDE.md HARD RULE). Any BLOCKER from either reviewer prevents declaring done. |
</threat_model>

<tasks>

<task type="auto">
  <name>Task 1: Create services/api/test/isolation/state_test.go — 4 cross-org probes (D-94)</name>
  <files>services/api/test/isolation/state_test.go</files>
  <read_first>
    - services/api/test/isolation/catalog_test.go (verbatim template — full file, 329 lines)
    - services/api/test/isolation/main_test.go (sharedSrv setup, freshOrg helper)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-94 — 4 probes)
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (lines 672-728 — test template)
    - .planning/phases/04-agent-state-machine-go/04-RESEARCH.md (cross-org isolation §)
  </read_first>
  <action>
Create `services/api/test/isolation/state_test.go`. Mirror the structure of `catalog_test.go` (same package `isolation`, same imports + helpers). Add the 4 probes from D-94:

```go
package isolation

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/require"

    "github.com/luongdev/open-routing/services/api/internal/api"
)

// getStatusEndpoint builds /v1/orgs/{org}/agents/{id}/status. Used by
// the 4 D-94 probes below.
func getStatusEndpoint(t testing.TB, urlBase string, orgID, agentID uuid.UUID) int {
    t.Helper()
    req, err := http.NewRequest(http.MethodGet,
        fmt.Sprintf("%s/v1/orgs/%s/agents/%s/status", urlBase, orgID, agentID),
        nil)
    require.NoError(t, err)
    req.Header.Set("X-Org-Id", orgID.String())
    resp, err := http.DefaultClient.Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()
    return resp.StatusCode
}

// patchStatusReturnCode issues PATCH and returns (statusCode, raw body).
// Used by the cross-org PATCH probes.
func patchStatusReturnCode(t testing.TB, urlBase string, orgID, agentID uuid.UUID, body any) (int, []byte) {
    t.Helper()
    buf, err := json.Marshal(body)
    require.NoError(t, err)
    req, err := http.NewRequest(http.MethodPatch,
        fmt.Sprintf("%s/v1/orgs/%s/agents/%s/status", urlBase, orgID, agentID),
        bytesReader(buf))
    require.NoError(t, err)
    req.Header.Set("X-Org-Id", orgID.String())
    req.Header.Set("Content-Type", "application/json")
    resp, err := http.DefaultClient.Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()
    raw, _ := readAll(resp.Body)
    return resp.StatusCode, raw
}

// TestState_GetStatusCrossOrg_404 — D-94(a): GET cross-org returns 404.
func TestState_GetStatusCrossOrg_404(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    orgA, orgB := freshOrg(t), freshOrg(t)

    // Seed agent in orgA (CreateAgent atomically seeds agent_states per D-93).
    codeA, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
        "external_id": "iso-state-a", "name": "A", "email": "a@example.com",
    })
    require.Equal(t, http.StatusCreated, codeA)

    // Same-org GET succeeds (200 — proves agent_states row exists).
    require.Equal(t, http.StatusOK, getStatusEndpoint(t, baseURL(), orgA, agentA))

    // Cross-org GET returns 404 — neither row leakage nor probe leak.
    require.Equal(t, http.StatusNotFound, getStatusEndpoint(t, baseURL(), orgB, agentA),
        "agent A's state must be invisible to orgB (FOUND-08)")
}

// TestState_PatchStatusCrossOrgAgent_404 — D-94(d): PATCH cross-org
// agent_id returns 404.
func TestState_PatchStatusCrossOrgAgent_404(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    orgA, orgB := freshOrg(t), freshOrg(t)

    codeA, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
        "external_id": "iso-state-pcr-a", "name": "A", "email": "a@example.com",
    })
    require.Equal(t, http.StatusCreated, codeA)

    // PATCH agentA but from orgB context — must 404.
    code, _ := patchStatusReturnCode(t, baseURL(), orgB, agentA, map[string]any{
        "to": "Ready",
    })
    require.Equal(t, http.StatusNotFound, code,
        "PATCH against an agent owned by orgA from orgB must return 404 (FOUND-08)")
}

// TestState_PatchStatusCrossOrgBreakReason_422 — D-94(b): PATCH with
// break_reason_id from another org returns 422.
func TestState_PatchStatusCrossOrgBreakReason_422(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    orgA, orgB := freshOrg(t), freshOrg(t)

    codeA, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
        "external_id": "iso-state-pco-a", "name": "A", "email": "a@example.com",
    })
    require.Equal(t, http.StatusCreated, codeA)

    // Seed break_reason in orgB.
    codeB, brB := postEntity(t, baseURL(), "break-reasons", orgB, map[string]any{
        "name": "OtherOrgBreak", "routable": false, "display_order": 0,
    })
    require.Equal(t, http.StatusCreated, codeB)

    // Move agentA from Offline (seeded by D-93) to NotReady (or Ready) first.
    // PATCH agent_id from orgA but break_reason_id from orgB → 422.
    // First need to transition Offline→NotReady is NOT in matrix; v0.1 stub
    // auth requires no auth check. Seed via force=true to NotReady? No —
    // STATE-09 says Offline→NotReady is system-only. Force-PATCH instead:
    code, _ := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
        "to": "NotReady", "force": true, // bypass matrix (Offline→NotReady not in v0.1 matrix per D-82)
    })
    require.Equal(t, http.StatusOK, code, "force should bypass matrix to set NotReady")

    // Now move NotReady → Ready (valid matrix transition).
    code, _ = patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{"to": "Ready"})
    require.Equal(t, http.StatusOK, code)

    // PATCH Ready → Break using orgB's break_reason_id. Must 422 invalid_reference.
    code, raw := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
        "to": "Break", "break_reason_id": brB.String(),
    })
    require.Equal(t, http.StatusUnprocessableEntity, code,
        "cross-org break_reason_id must return 422 invalid_reference (D-94b) — body=%s", string(raw))

    var errBody api.ErrorResponse
    require.NoError(t, json.Unmarshal(raw, &errBody))
    require.Equal(t, api.ErrorCodeInvalidReference, errBody.Error)
}

// TestState_ForceDoesNotBypassCrossOrgBreakReason_422 — D-94(c) +
// Pitfall 3 regression gate. force=true MUST NOT bypass cross-row probes.
func TestState_ForceDoesNotBypassCrossOrgBreakReason_422(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    orgA, orgB := freshOrg(t), freshOrg(t)

    codeA, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
        "external_id": "iso-state-fcb-a", "name": "A", "email": "a@example.com",
    })
    require.Equal(t, http.StatusCreated, codeA)
    codeB, brB := postEntity(t, baseURL(), "break-reasons", orgB, map[string]any{
        "name": "OtherOrgBreakForce", "routable": false, "display_order": 0,
    })
    require.Equal(t, http.StatusCreated, codeB)

    // Even with force=true + valid matrix bypass — cross-row probe still fires.
    code, raw := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
        "to": "Break", "break_reason_id": brB.String(), "force": true,
    })
    require.Equal(t, http.StatusUnprocessableEntity, code,
        "force=true MUST NOT bypass cross-org break_reason probe (D-84 Pitfall 3) — body=%s", string(raw))

    var errBody api.ErrorResponse
    require.NoError(t, json.Unmarshal(raw, &errBody))
    require.Equal(t, api.ErrorCodeInvalidReference, errBody.Error,
        "expected invalid_reference even with force=true")
}

// TestCreateAgentSeedsState — D-93 + Pitfall 6 regression gate. Proves
// CreateAgent atomically seeds agent_states so subsequent GET /status
// returns 200, not 404.
func TestCreateAgentSeedsState(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    org := freshOrg(t)

    code, agentID := postEntity(t, baseURL(), "agents", org, map[string]any{
        "external_id": "iso-state-cas", "name": "Seeded", "email": "s@example.com",
    })
    require.Equal(t, http.StatusCreated, code)

    // Immediately GET /status — must succeed because CreateAgent's tx
    // includes the agent_states INSERT (D-93). If this returns 404,
    // either (a) the agent_states INSERT was skipped, or (b) it ran
    // outside the tx and the tx rolled back without it. Either is a
    // Pitfall 10 violation.
    require.Equal(t, http.StatusOK, getStatusEndpoint(t, baseURL(), org, agentID),
        "GET /status immediately after CreateAgent must succeed (D-93 atomic seeding)")
}
```

Note on helper imports: if `bytesReader`, `readAll` are not standard, use `bytes.NewReader(buf)` and `io.ReadAll(resp.Body)` directly. The catalog_test.go file uses the same pattern — copy whatever shape it uses.

If `postEntity` from catalog_test.go doesn't expose `id` for entities like break-reasons (which have a server-generated id returned in the response body), parse the response body inside the test or extend `postEntity`. The Phase 3 helper returns `(statusCode int, idOrEmpty uuid.UUID)` where the second return is the parsed id from the response body — verify it works for break-reasons by reading catalog_test.go.

Hazards:
- `TestState_PatchStatusCrossOrgBreakReason_422` needs agentA in `Ready` state before issuing the cross-org break_reason PATCH. The agent starts in `Offline` (D-93 seed). The matrix has Offline as NOT-an-outer-key — only force=true can move it. So the test first force-PATCHes to NotReady, then matrix-PATCHes to Ready, then triggers the 422 probe. Document this sequence in a code comment.
- `TestState_ForceDoesNotBypassCrossOrgBreakReason_422` short-circuits: the cross-org probe runs FIRST (Pitfall 3), so the agent's current status doesn't matter. The test can issue the PATCH directly from Offline state and expect 422 (probe fires before any matrix logic).
- All tests use `t.Parallel()` because each test acquires a fresh org via `freshOrg(t)` — zero shared state.
  </action>
  <acceptance_criteria>
    - 5 isolation test functions declared:
      - `grep -q "^func TestState_GetStatusCrossOrg_404" services/api/test/isolation/state_test.go`
      - `grep -q "^func TestState_PatchStatusCrossOrgAgent_404" services/api/test/isolation/state_test.go`
      - `grep -q "^func TestState_PatchStatusCrossOrgBreakReason_422" services/api/test/isolation/state_test.go`
      - `grep -q "^func TestState_ForceDoesNotBypassCrossOrgBreakReason_422" services/api/test/isolation/state_test.go`
      - `grep -q "^func TestCreateAgentSeedsState" services/api/test/isolation/state_test.go`
    - `cd services/api && go test -count=1 -v ./test/isolation/... -run "TestState_|TestCreateAgentSeedsState"` exits 0
    - `grep -q "freshOrg(t)" services/api/test/isolation/state_test.go` (helper reused, not redefined)
    - `grep -q "requireContainer(t)" services/api/test/isolation/state_test.go`
    - The Pitfall 3 test references invalid_reference: `awk '/TestState_ForceDoesNotBypassCrossOrgBreakReason/,/^}$/' services/api/test/isolation/state_test.go | grep -q "ErrorCodeInvalidReference"`
    - The Pitfall 3 test uses force=true: `awk '/TestState_ForceDoesNotBypassCrossOrgBreakReason/,/^}$/' services/api/test/isolation/state_test.go | grep -q "\"force\": true"`
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go test -count=1 -v ./test/isolation/... -run "TestState_|TestCreateAgentSeedsState"</automated>
  </verify>
  <done>4 D-94 probes + atomic-seeding regression gate all green; cross-org isolation invariants enforced through the running mux.</done>
</task>

<task type="auto">
  <name>Task 2: Add 5 acceptance tests to internal/state/agent_states_test.go matching ROADMAP success criteria</name>
  <files>services/api/internal/state/agent_states_test.go</files>
  <read_first>
    - .planning/ROADMAP.md (Phase 4 §Success Criteria — 5 numbered items)
    - services/api/internal/state/agent_states_test.go (existing Wave 2 tests — append new ones)
    - .planning/phases/04-agent-state-machine-go/04-VALIDATION.md (test name mappings)
    - services/api/internal/state/ttl_test.go (existing Wave 3 tests — `TestWrapUpTTL_FiresOnExpiry` is the analog for acceptance #3)
  </read_first>
  <action>
Append 5 acceptance tests to `services/api/internal/state/agent_states_test.go`. Each test corresponds to one of the ROADMAP Phase 4 success criteria.

```go
// ===========================================================================
// Acceptance tests — one per ROADMAP Phase 4 §Success Criteria.
// ===========================================================================

// TestAcceptance_AllTransitions — ROADMAP §1.
// PATCH accepts every allowed transition; rejects every other with 409.
func TestAcceptance_AllTransitions(t *testing.T) {
    th := newTestHandlers(t)
    require.NotNil(t, th)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    cleanStateTables(t, ctx, th.Pool)

    // Table-drive every allowed edge from STATE-02:
    type tc struct {
        name      string
        seedFrom  string
        patchTo   api.AgentStatus
        breakRsn  *uuid.UUID
        wantCode  int
    }
    var brID uuid.UUID
    seedAgent(t, th.Pool, th.OrgID, /* agentID */ uuid.Must(uuid.NewV7()), "agA", "Agent A")
    brID = uuid.Must(uuid.NewV7())
    seedBreakReason(t, th.Pool, th.OrgID, brID, "TestBreak", true /* routable */)

    cases := []tc{
        {"NotReady→Ready allowed", "NotReady", api.AgentStatusReady, nil, http.StatusOK},
        {"Ready→NotReady allowed", "Ready", api.AgentStatusNotReady, nil, http.StatusOK},
        {"Ready→Break allowed with break_reason", "Ready", api.AgentStatusBreak, &brID, http.StatusOK},
        {"Break→Ready allowed", "Break", api.AgentStatusReady, nil, http.StatusOK},
        {"Break→NotReady allowed", "Break", api.AgentStatusNotReady, nil, http.StatusOK},
        {"WrapUp→Ready allowed", "WrapUp", api.AgentStatusReady, nil, http.StatusOK},
        {"WrapUp→NotReady allowed", "WrapUp", api.AgentStatusNotReady, nil, http.StatusOK},
        // Rejected (system-only) transitions:
        {"Engaged→Ready rejected (Engaged not in matrix)", "Engaged", api.AgentStatusReady, nil, http.StatusConflict},
        {"Offline→Ready rejected", "Offline", api.AgentStatusReady, nil, http.StatusConflict},
        {"NotReady→Engaged rejected (system-only)", "NotReady", api.AgentStatusEngaged, nil, http.StatusConflict},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            agentID := uuid.Must(uuid.NewV7())
            seedAgent(t, th.Pool, th.OrgID, agentID, "ag-"+c.name, "AG")
            seedAgentStateRow(t, th.Pool, SeedStateParams{
                AgentID: agentID, OrgID: th.OrgID, Status: c.seedFrom, StateVersion: 1,
                BreakReasonID: c.breakRsn,
            })
            body := map[string]any{"to": c.patchTo}
            if c.breakRsn != nil {
                body["break_reason_id"] = c.breakRsn.String()
            }
            resp, raw := httpPATCHStatus(t, th, agentID, body)
            require.Equal(t, c.wantCode, resp.StatusCode, "body=%s", string(raw))
            if c.wantCode == http.StatusConflict {
                var err409 api.InvalidTransitionErrorResponse
                require.NoError(t, json.Unmarshal(raw, &err409))
                require.Equal(t, api.ErrorCodeInvalidTransition, err409.Error)
                require.Equal(t, api.AgentStatus(c.seedFrom), err409.From, "409 must carry observed from")
                require.Equal(t, c.patchTo, err409.To)
            }
        })
    }
}

// TestAcceptance_BreakReasonValidation — ROADMAP §2.
// Cross-org break_reason → 422; valid same-org → 200 with reason recorded.
// (This is exercised by Wave 2's TestPatchAgentStatus_BreakReasonProbe_*
// tests + the Wave 5 cross-org isolation tests; aggregate-pass assertion.)
func TestAcceptance_BreakReasonValidation(t *testing.T) {
    th := newTestHandlers(t)
    require.NotNil(t, th)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    cleanStateTables(t, ctx, th.Pool)

    agentID := uuid.Must(uuid.NewV7())
    brID := uuid.Must(uuid.NewV7())
    seedAgent(t, th.Pool, th.OrgID, agentID, "a-acc-2", "A")
    seedBreakReason(t, th.Pool, th.OrgID, brID, "ValidBreak", false /* routable */)
    seedAgentStateRow(t, th.Pool, SeedStateParams{AgentID: agentID, OrgID: th.OrgID, Status: "Ready", StateVersion: 1})

    // Same-org break_reason → 200 + reason recorded on row.
    resp, raw := httpPATCHStatus(t, th, agentID, map[string]any{"to": "Break", "break_reason_id": brID.String()})
    require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
    var state api.AgentState
    require.NoError(t, json.Unmarshal(raw, &state))
    require.NotNil(t, state.BreakReasonId)
    require.Equal(t, brID, uuid.UUID(*state.BreakReasonId), "break_reason_id must be recorded on the row")
}

// TestAcceptance_WrapUpExpiresWithoutClient — ROADMAP §3.
// An agent in WrapUp whose browser is closed transitions to
// post_interaction_state automatically after wrapup_until expires.
// Uses a SHORT wrapup_until and the REAL clock to mirror real production
// behavior (clockwork.FakeClock is for Wave 3 unit tests; this is the
// integration acceptance gate).
func TestAcceptance_WrapUpExpiresWithoutClient(t *testing.T) {
    th := newTestHandlers(t)
    require.NotNil(t, th)
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    cleanStateTables(t, ctx, th.Pool)

    // Build a fresh Server with the real clock + short sweep interval.
    s := New(Deps{OrgDB: th.S.deps.OrgDB, Cache: th.S.deps.Cache, Logger: th.S.deps.Logger},
        WithSweepInterval(500*time.Millisecond))
    require.NoError(t, s.Start(ctx))
    defer s.Stop()

    agentID := uuid.Must(uuid.NewV7())
    seedAgent(t, th.Pool, th.OrgID, agentID, "a-acc-3", "A")
    wrapupUntil := time.Now().Add(1 * time.Second) // very short
    pis := "ready"
    seedAgentStateRow(t, th.Pool, SeedStateParams{
        AgentID: agentID, OrgID: th.OrgID, Status: "WrapUp",
        WrapupUntil: &wrapupUntil, PostInteractionState: &pis, StateVersion: 1,
    })
    s.scheduleWrapUpExpiry(agentID, th.OrgID, wrapupUntil)

    require.Eventually(t, func() bool {
        row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
        return row.Status == "Ready" && !row.WrapupUntil.Valid && row.StateVersion >= 2
    }, 6*time.Second, 100*time.Millisecond, "WrapUp expiry never fired with real clock (TTL+5s acceptance window)")
}

// TestAcceptance_IsRoutableMatrix — ROADMAP §4.
// IsRoutable returns true only for Ready or Break+routable=true; all 4
// matrix combinations have unit-test coverage. This acceptance wrapper
// re-runs the 4 named cases as a single t.Run group so /gsd-verify-work
// can hash a single test name.
func TestAcceptance_IsRoutableMatrix(t *testing.T) {
    // The cases are already covered exhaustively by
    // services/api/internal/domain/state_test.go (Wave 1 — TestIsRoutable).
    // This wrapper test name exists for ROADMAP traceability per
    // 04-VALIDATION.md test-name mapping.
    t.Run("see services/api/internal/domain/state_test.go::TestIsRoutable", func(t *testing.T) {
        t.Log("STATE-10 + ROADMAP §4 — 10 test cases pass in internal/domain/state_test.go::TestIsRoutable")
    })
}

// TestAcceptance_StateVersionMonotonic — ROADMAP §5.
// Every state mutation increments state_version monotonically. Five
// sequential PATCHes through valid transitions assert versions 1→2→3→4→5→6.
func TestAcceptance_StateVersionMonotonic(t *testing.T) {
    th := newTestHandlers(t)
    require.NotNil(t, th)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    cleanStateTables(t, ctx, th.Pool)

    agentID := uuid.Must(uuid.NewV7())
    seedAgent(t, th.Pool, th.OrgID, agentID, "a-acc-5", "A")
    brID := uuid.Must(uuid.NewV7())
    seedBreakReason(t, th.Pool, th.OrgID, brID, "VerBreak", false)
    seedAgentStateRow(t, th.Pool, SeedStateParams{AgentID: agentID, OrgID: th.OrgID, Status: "NotReady", StateVersion: 1})

    transitions := []map[string]any{
        {"to": "Ready"},                                            // NotReady→Ready (1→2)
        {"to": "Break", "break_reason_id": brID.String()},          // Ready→Break (2→3)
        {"to": "Ready"},                                            // Break→Ready (3→4)
        {"to": "NotReady"},                                         // Ready→NotReady (4→5)
        {"to": "Ready"},                                            // NotReady→Ready (5→6)
    }
    expectedVer := int64(2)
    for i, body := range transitions {
        resp, raw := httpPATCHStatus(t, th, agentID, body)
        require.Equal(t, http.StatusOK, resp.StatusCode, "transition %d: body=%s", i, string(raw))
        var state api.AgentState
        require.NoError(t, json.Unmarshal(raw, &state))
        require.Equal(t, expectedVer, state.StateVersion, "state_version monotonic violation at step %d", i)
        expectedVer++
    }
}
```

Hazards:
- The `TestAcceptance_AllTransitions` cases include seeded `Engaged` and `Offline` states — these are bypass-the-matrix states that v0.1 cannot reach via PATCH, so the test uses `seedAgentStateRow` to plant them directly (D-82 — system-only state seeded via fixture).
- `TestAcceptance_WrapUpExpiresWithoutClient` uses the REAL clock (NOT clockwork.FakeClock) because ROADMAP §3 says "verified by killing the connection and waiting TTL+5s." This is the integration acceptance — Wave 3's FakeClock tests are the unit acceptance. Both must pass.
- `TestAcceptance_IsRoutableMatrix` is a thin wrapper that delegates to the Wave 1 domain package tests. ROADMAP §4 is satisfied by `TestIsRoutable` directly; this wrapper exists for /gsd-verify-work traceability.
- Each test creates fresh agents and break_reasons inside its own subtests so parallel execution is safe (the testutil's TestHandlers carries a unique OrgID).
- The 409 assertion in `TestAcceptance_AllTransitions` checks `err409.From == api.AgentStatus(c.seedFrom)` — proves the handler returns the OBSERVED current status (read from the DB disambiguate) NOT the requested status.
  </action>
  <acceptance_criteria>
    - 5 acceptance test functions declared:
      - `grep -q "^func TestAcceptance_AllTransitions" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestAcceptance_BreakReasonValidation" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestAcceptance_WrapUpExpiresWithoutClient" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestAcceptance_IsRoutableMatrix" services/api/internal/state/agent_states_test.go`
      - `grep -q "^func TestAcceptance_StateVersionMonotonic" services/api/internal/state/agent_states_test.go`
    - `cd services/api && go test -count=1 -v -run "TestAcceptance" ./internal/state/... ./internal/domain/...` exits 0
    - `TestAcceptance_AllTransitions` covers at least 7 allowed transitions (the 7 agent-initiated edges) AND at least 3 rejected transitions
    - `TestAcceptance_StateVersionMonotonic` runs at least 5 transitions and asserts a +1 increment on each
    - `cd services/api && go test -count=1 ./internal/state/... ./internal/domain/...` exits 0 (full state + domain suite green)
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go test -count=1 -v -run "TestAcceptance" ./internal/state/... ./internal/domain/... && go test -count=1 ./internal/state/... ./internal/domain/...</automated>
  </verify>
  <done>5 acceptance tests cover ROADMAP §Phase 4 success criteria #1-#5; state package suite still green.</done>
</task>

<task type="auto">
  <name>Task 3: Append `Phase 5 Inheritance Reminder` section to 04-PATTERNS.md</name>
  <files>.planning/phases/04-agent-state-machine-go/04-PATTERNS.md</files>
  <read_first>
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md (current tail — "Metadata" section)
    - .planning/phases/04-agent-state-machine-go/04-CONTEXT.md (D-93 + Pitfall 6 wording)
    - services/api/internal/catalog/agents.go (the WHY-comment from Wave 4 Task 1 — same wording)
  </read_first>
  <action>
Edit `.planning/phases/04-agent-state-machine-go/04-PATTERNS.md`. Append a new section ABOVE the existing `## Metadata` section at the tail of the file:

```markdown
## Phase 5 Inheritance Reminder — Pitfall 6 / Hazard 7

**Source:** D-93 (Phase 4 CONTEXT.md); Hazard 7 in §Pattern Hazards above.

**What Phase 5 (bulk-import) MUST do:**

When Phase 5's CSV/JSON import handler upserts agent rows, it MUST also
INSERT into `agent_states` for every NEW agent. Phase 4's `CreateAgent`
extension (commit history: see 04-05-PLAN.md Task 1) seeds the initial
state row inside the existing tx. Bulk import's path is wider — it can
INSERT many agents in one request — but the invariant is identical: a
GET `/agents/{id}/status` MUST return 200 (not 404) for every agent
visible via GET `/agents`.

**Exact pattern to use in Phase 5's import SQL:**

```sql
-- After (or alongside) the existing INSERT INTO agents ... ON CONFLICT
-- (org_id, external_id) DO UPDATE block:
INSERT INTO agent_states (agent_id, org_id, status, state_version)
SELECT id, org_id, 'Offline', 1
FROM agents
WHERE (org_id, external_id) IN ( /* the just-upserted set */ )
ON CONFLICT (agent_id) DO NOTHING;
```

The `ON CONFLICT (agent_id) DO NOTHING` clause is critical:
- New agent → INSERT succeeds (agent + state row both created).
- Re-import of existing agent → INSERT skipped (existing state row preserved;
  re-imports MUST NOT regress the state machine to Offline).

**Regression gate:** Phase 5's plan must include a test
`TestBulkImport_SeedsAgentStatesForNewAgents` mirroring Phase 4's
`TestCreateAgentSeedsState` (services/api/test/isolation/state_test.go).
The test imports 3 agents via CSV/JSON; immediately GET /status for each;
all 3 return 200.

**Reviewer (Codex / Gemini) check:** if Phase 5's diff modifies any
agents-INSERT path WITHOUT a corresponding agent_states-INSERT (matching
the ON CONFLICT pattern above), the reviewer MUST flag BLOCK with this
section as the reference.

**Why this lives in 04-PATTERNS.md, not 05-PATTERNS.md:** Phase 5's
planner runs `/gsd:plan-phase 05` and will load 04-PATTERNS.md as
part of the carry-forward context (per `<files_to_read>` in the
planning_context). Appending here makes the reminder unmissable.
```

Hazards:
- DO NOT modify any other section of 04-PATTERNS.md. The append is additive.
- The `## Phase 5 Inheritance Reminder` heading uses `##` (h2) to match the file's existing structure.
- The ON CONFLICT pattern in the example is illustrative — Phase 5's executor decides whether to use a SELECT-INSERT or a row-by-row pattern based on its actual schema for `agent_skills`-style joins. The CRITICAL invariant is "every new agent_id MUST result in an agent_states row before the import endpoint returns 207/200."
- The reminder includes a specific reviewer-check sentence so cross-AI peer review in Phase 5 has a clear BLOCK criterion.
  </action>
  <acceptance_criteria>
    - `grep -q "^## Phase 5 Inheritance Reminder" .planning/phases/04-agent-state-machine-go/04-PATTERNS.md`
    - `grep -q "ON CONFLICT (agent_id) DO NOTHING" .planning/phases/04-agent-state-machine-go/04-PATTERNS.md`
    - `grep -q "TestBulkImport_SeedsAgentStatesForNewAgents" .planning/phases/04-agent-state-machine-go/04-PATTERNS.md`
    - `grep -q "Pitfall 6" .planning/phases/04-agent-state-machine-go/04-PATTERNS.md`
    - The section appears BEFORE the `## Metadata` section (verified via `awk '/^## /{print NR": "$0}'` showing Inheritance Reminder line number < Metadata line number)
    - The file remains valid markdown — no syntax errors (verify with `awk` line-count change matches expected)
  </acceptance_criteria>
  <verify>
    <automated>grep -q "^## Phase 5 Inheritance Reminder" /Users/luong/workspace/dev/open-solutions/.planning/phases/04-agent-state-machine-go/04-PATTERNS.md && grep -q "ON CONFLICT (agent_id) DO NOTHING" /Users/luong/workspace/dev/open-solutions/.planning/phases/04-agent-state-machine-go/04-PATTERNS.md && [ "$(awk '/^## Phase 5 Inheritance Reminder/{print NR; exit}' /Users/luong/workspace/dev/open-solutions/.planning/phases/04-agent-state-machine-go/04-PATTERNS.md)" -lt "$(awk '/^## Metadata/{print NR; exit}' /Users/luong/workspace/dev/open-solutions/.planning/phases/04-agent-state-machine-go/04-PATTERNS.md)" ]</automated>
  </verify>
  <done>04-PATTERNS.md carries the Phase 5 inheritance reminder; ordered before Metadata; reviewer-check criterion present.</done>
</task>

<task type="auto">
  <name>Task 4: Run full Phase 4 verification suite — race-clean, codegen-drift-clean, all green</name>
  <files>(no source modifications — validation only)</files>
  <read_first>
    - .planning/phases/04-agent-state-machine-go/04-VALIDATION.md (full-suite expectations)
  </read_first>
  <action>
Run the following sequence. STOP on first non-zero exit and report which step failed.

1. `cd services/api && task db:reset` — fresh DB.
2. `cd services/api && go test -count=1 -race ./...` — full suite with race detector. ALL tests must pass.
3. `cd services/api && task gen` and `cd web && pnpm gen:api && cd ..` — regenerate.
4. `git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/generated.ts openapi/openapi.yaml` — codegen-drift gate.
5. `cd services/api && go vet ./...` — no warnings.
6. `cd services/api && golangci-lint run ./...` — lints clean (if `golangci-lint` configured per Phase 1 D-30).

Expected counts (lower bounds — actual may be higher):
- `go test -count=1 ./internal/state/...` runs 13 handler tests + 6 TTL tests + 3 transition matrix tests + 5 acceptance tests = at least 27 named tests.
- `go test -count=1 ./internal/domain/...` runs 1 TestIsRoutable with 10 subtests.
- `go test -count=1 ./test/isolation/...` runs Phase 3's existing catalog suite + Phase 4's 5 new state tests.

If any step fails, report the failing test names + error output verbatim. Do NOT silently retry or skip.
  </action>
  <acceptance_criteria>
    - `cd services/api && task db:reset` exits 0
    - `cd services/api && go test -count=1 -race ./...` exits 0
    - `cd services/api && task gen` exits 0
    - `cd web && pnpm gen:api` exits 0
    - `git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/generated.ts openapi/openapi.yaml` exits 0
    - `cd services/api && go vet ./...` exits 0
    - `cd services/api && golangci-lint run ./...` exits 0 (skip if golangci-lint not installed locally; CI gate still applies)
    - At least 27 state-package test functions counted: `[ "$(grep -c '^func Test' services/api/internal/state/agent_states_test.go services/api/internal/state/transitions_test.go services/api/internal/state/ttl_test.go | awk -F: '{s += $NF} END {print s}')" -ge 27 ]`
    - At least 5 acceptance tests counted: `[ "$(grep -c '^func TestAcceptance' services/api/internal/state/agent_states_test.go)" -ge 5 ]`
    - At least 5 isolation tests counted: `[ "$(grep -c '^func TestState_\|^func TestCreateAgentSeedsState' services/api/test/isolation/state_test.go)" -ge 5 ]`
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && task db:reset && go test -count=1 -race ./... && task gen && cd ../.. && cd web && pnpm gen:api && cd .. && git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/generated.ts openapi/openapi.yaml && cd services/api && go vet ./...</automated>
  </verify>
  <done>Full Phase 4 suite race-clean; codegen-drift CI gate green; no vet/lint warnings.</done>
</task>

<task type="checkpoint:human-verify" gate="blocking-human">
  <name>Task 5: Cross-AI peer review (Codex + Gemini in parallel) on the full Phase 4 diff — project CLAUDE.md HARD RULE</name>
  <what-built>
    Phase 4 introduces 12+ new files, modifies main.go + catalog/agents.go + sqlcheck.go + 2 migrations + openapi.yaml, and activates the D-89 ApiHandlers composite. Per project CLAUDE.md and user-global ~/.claude/CLAUDE.md, the Cross-AI Peer Review HARD RULE applies: every non-trivial work unit (≥5 files OR ≥3 commits) MUST be reviewed by Codex + Gemini in parallel BEFORE declaring done. Phase 3's plan review caught 6 BLOCKER findings (PATCH zero-overwrite, wave dep regression, testutil.go visibility, proficiency 422/400 conflict) that Claude's in-house plan-checker AND Gemini both missed. This wave's review is the production-readiness gate.
  </what-built>
  <how-to-verify>
    1. Generate the full Phase 4 diff against main:
       ```bash
       BASE=$(git merge-base HEAD origin/main)
       git diff "$BASE"..HEAD > /tmp/phase4-diff.patch
       wc -l /tmp/phase4-diff.patch  # confirm non-empty
       ```

    2. Run Codex + Gemini IN PARALLEL (per ~/.claude/CLAUDE.md template):
       ```bash
       PROMPT='Review this Phase 4 (agent state machine) diff for bugs,
       security, scope creep, missed edges, PATCH semantics, transaction
       boundaries, contract drift, and project-CLAUDE.md compliance
       (WHY-not-WHAT comments).

       Focus areas:
       - Pitfall 1 — state.Server vs Handlers naming consistency
       - Pitfall 3 — force=true does NOT bypass cross-row probes
       - Pitfall 6 — Phase 5 inheritance reminder embedded
       - Pitfall 8 — TTL sweeper uses db.WithBypass for org_id
       - Pitfall 10 — CreateAgent extension uses qtx not raw pool
       - D-66 — atomic UPDATE + 0-row disambiguate via in-tx SELECT
       - D-84 — force=true emits slog WARN after commit
       - D-85 — state_version is read-side; no expected_version in PATCH
       - D-93 — initial agent_states row atomic with agent INSERT
       - D-95 — Start synchronously runs startup sweep; Stop drains timers
       - SQLChecker — every agent_states query mentions org_id

       Output: ## Summary / ## Concerns [HIGH/MED/LOW] /
       ## Suggestions / ## Verdict: READY | READY WITH FIXES | BLOCK.'

       { echo "$PROMPT"; cat /tmp/phase4-diff.patch; } | codex exec --skip-git-repo-check - > /tmp/codex-review.md 2>&1 &
       { echo "$PROMPT"; cat /tmp/phase4-diff.patch; } | gemini -p - > /tmp/gemini-review.md 2>&1 &
       wait
       ```

    3. Read both review files. Synthesize:
       - List every HIGH-severity concern from each reviewer.
       - Note if either verdict is BLOCK.
       - Note if both verdicts are READY (or READY WITH FIXES).

    4. If EITHER reviewer says BLOCK or raises HIGH-severity concern:
       - Address the concern with a code change (new commit on the same branch).
       - Re-run the review on the new diff (steps 1-3) before resuming.
       - DO NOT proceed to Task 6 with unaddressed BLOCK or HIGH findings.

    5. If both reviewers say READY (or READY WITH FIXES with only LOW/MED suggestions that you've documented):
       - Save the two review outputs to `.planning/phases/04-agent-state-machine-go/04-REVIEWS.md` (paste both with reviewer headers).
       - Respond with the synthesized verdict (e.g., "READY: 2 LOW-severity Codex suggestions deferred to Phase 5 — see REVIEWS.md").

    6. Hard rule: if the cross-AI invocation fails (Codex CLI unavailable, Gemini API timeout), surface "BLOCKED — cross-AI review tooling failed: <reason>" and do NOT bypass the review. The HARD RULE is non-negotiable.
  </how-to-verify>
  <resume-signal>Type "approved — cross-AI review complete, both reviewers READY" or "BLOCKED — addressing findings: <list>".</resume-signal>
</task>

<task type="auto">
  <name>Task 6: Update STATE.md + final verification + Phase 4 SUMMARY</name>
  <files>
    .planning/STATE.md,
    .planning/phases/04-agent-state-machine-go/04-06-SUMMARY.md
  </files>
  <read_first>
    - .planning/STATE.md (current — last_activity, position, progress fields)
    - .planning/ROADMAP.md (Phase 4 entry — mark complete when done)
    - .planning/phases/04-agent-state-machine-go/04-REVIEWS.md (cross-AI review outputs from Task 5)
  </read_first>
  <action>
After Task 5's cross-AI review approves (or you've addressed all HIGH findings and re-run):

1. Update `.planning/STATE.md`:
   - Bump `progress.completed_phases` from 3 to 4.
   - Bump `progress.completed_plans` from 28 to 34 (6 new plans).
   - Update `progress.percent` to 57 (4/7 = 57%).
   - Update `last_activity` to today's date (2026-05-17).
   - Update `status` to "Phase 04 shipped — branch gsd/phase-04-agent-state-machine-go".
   - Add Phase 4 entry to the velocity table.

2. Update `.planning/ROADMAP.md`:
   - Change the Phase 4 checkbox from `- [ ]` to `- [x]`.
   - Append `(completed 2026-05-17)` to the Phase 4 short description.
   - Update the Progress table at the bottom: Phase 4 row 0/TBD → 6/6 Complete.

3. Create `.planning/phases/04-agent-state-machine-go/04-06-SUMMARY.md` summarizing:
   - List of all files modified across Waves 0-5 (count + paths).
   - Test count: state package, domain package, isolation suite.
   - Cross-AI review verdict (from REVIEWS.md).
   - Confirmation `go test -race ./...` is green.
   - Confirmation `task gen && git diff --exit-code` is green.
   - All 10 STATE-* requirements crossed off as complete.

Do NOT push to remote yet — user controls the PR creation step.
  </action>
  <acceptance_criteria>
    - `grep -q "completed_phases: 4" .planning/STATE.md`
    - `grep -q "Phase 04 shipped" .planning/STATE.md`
    - `grep -q "- \[x\] \*\*Phase 4:" .planning/ROADMAP.md`
    - `.planning/phases/04-agent-state-machine-go/04-06-SUMMARY.md` exists
    - `grep -q "STATE-01" .planning/phases/04-agent-state-machine-go/04-06-SUMMARY.md` (requirement traceability)
    - `grep -q "STATE-10" .planning/phases/04-agent-state-machine-go/04-06-SUMMARY.md`
    - Cross-AI review verdict captured in SUMMARY: `grep -q "Cross-AI\|cross-AI\|Codex\|Gemini" .planning/phases/04-agent-state-machine-go/04-06-SUMMARY.md`
    - `cd services/api && go test -race -count=1 ./...` STILL exits 0 (final regression check)
  </acceptance_criteria>
  <verify>
    <automated>cd services/api && go test -race -count=1 ./... && [ -f /Users/luong/workspace/dev/open-solutions/.planning/phases/04-agent-state-machine-go/04-06-SUMMARY.md ] && grep -q "completed_phases: 4" /Users/luong/workspace/dev/open-solutions/.planning/STATE.md</automated>
  </verify>
  <done>STATE.md + ROADMAP.md updated to Phase 4 complete; 04-06-SUMMARY.md captures wave-by-wave deliverables + cross-AI verdict; full suite race-clean.</done>
</task>

</tasks>

<verification>
- 5 D-94 cross-org isolation tests in services/api/test/isolation/state_test.go pass (including Pitfall 3 regression gate)
- 5 acceptance tests cover ROADMAP §Phase 4 success criteria #1-#5
- TestCreateAgentSeedsState proves D-93 atomic seeding survives Wave 4 wiring
- 04-PATTERNS.md carries Phase 5 Inheritance Reminder section
- `go test -race -count=1 ./...` exits 0 (no data races; full regression green)
- `task gen && git diff --exit-code` is clean (codegen-drift CI gate)
- Cross-AI peer review (Codex + Gemini parallel) ran and both verdicts are READY (or READY WITH FIXES with only LOW/MED noted)
- STATE.md + ROADMAP.md reflect Phase 4 complete
</verification>

<success_criteria>
- 6 tasks completed (5 auto + 1 human checkpoint for cross-AI review)
- All 10 STATE-* requirements verifiable through automated tests
- Cross-AI review captured at .planning/phases/04-agent-state-machine-go/04-REVIEWS.md
- /gsd-verify-work can proceed against Phase 4 with confidence
- Phase 5 (bulk import) has clear inheritance contract via PATTERNS.md reminder
</success_criteria>

<output>
Create `.planning/phases/04-agent-state-machine-go/04-06-SUMMARY.md` when done. Include: full file-modification inventory across Waves 0-5, test count breakdown, cross-AI review excerpts (HIGH-severity findings + verdicts), confirmation race-clean, list of STATE-01..STATE-10 with the test name covering each.
</output>

hook: SessionStart
hook: SessionStart Completed
codex
## Summary
Plan bundle is broadly complete and captures the major Phase 4 invariants, especially `force=true` probe ordering, `state.Server`, D-93 atomic seeding, and TTL bypass context. Verdict is **READY WITH FIXES** because there are several plan-level defects likely to cause compile/test failures or wave-order drift.

## Strengths
- All 5 critical pitfalls are captured: P1, P3, P6, P8, P10.
- D-94 cross-org probes are explicitly named, including `force=true` still returning 422.
- Cache strategy is mostly correct: singular `agent_state` key, `cache.Del` after write, sweeper invalidates using returned `org_id`.
- D-93 is well-covered: `CreateAgent` seeds `agent_states` inside existing `qtx`.
- Spec amendment is scoped to request body only; codegen drift gate is included.

## Concerns

### [HIGH]: Wave 3 is not actually parallel-safe with Wave 2
Wave dependency says `Wave 0 → 1 → {2,3 parallel} → 4 → 5`, but `04-04` TTL tests depend on `services/api/internal/state/testutil_test.go` and helpers created in `04-03`. `04-04 depends_on` is only `04-02`.

Fix: set `04-04 depends_on: [04-03]`, or move shared testutil scaffolding into Wave 1/2 before both branches.

### [HIGH]: `ttl.go` plan includes likely non-compiling dead code
`jitter()` uses `int64(uuid.New().Time())`, which is unsafe against the `google/uuid` API and is unused. The plan also adds a `pgtype` import plus `var _ pgtype.Text` as an unused-import guard. This is scope creep and likely violates comment/dead-code hygiene.

Fix: delete `jitter()` from Wave 3 unless used by a real system transition path. Keep D-87 as deferred. Remove `pgtype` import/guard.

### [MED]: Acceptance test wrapper for `IsRoutable` does not assert behavior
`TestAcceptance_IsRoutableMatrix` only logs a pointer to another test. That gives traceability, but it is not an acceptance test.

Fix: either call `domain.IsRoutable` directly for the 4 ROADMAP cases, or rely on `internal/domain/TestIsRoutable` in the mapping and remove the wrapper.

### [MED]: Comment hygiene is weak in the planned snippets
Many inserted comments explain what/how, not why, especially in test scaffolding, SQL comments, handler steps, and `notimpl.go`. This conflicts with the project hard rule.

Examples to trim:
- “GetAgentStatus serves…”
- “STEP 1 — BREAK_REASON PROBE”
- “Create `ttl.go` with 6 functions”
- Long explanatory comments in generated-plan snippets.

Fix: keep only invariant/rationale comments: D-84 probe-before-force, D-93 qtx atomicity, D-95 startup sweep, P8 bypass.

### [MED]: `UpdateAgentStateStatus` null assignment semantics need explicit validation
SQL directly assigns `break_reason_id = sqlc.narg(...)`, `engaged_channel = sqlc.narg(...)`, `wrapup_until = sqlc.narg(...)`. Omitted fields become NULL. That may be intended when leaving Break, but the plan does not state cleanup rules per target status.

Fix: add acceptance criteria for field clearing:
- Break sets `break_reason_id`.
- Ready/NotReady clear `break_reason_id`, `engaged_channel`, `wrapup_until` if intended.
- Force to Engaged with PIS does not accidentally erase needed fields.

### [LOW]: Wave 0 human checkpoint for clockwork is unnecessary friction
User context says stack locked and package decision is already in planning. A blocking human checkpoint inside an otherwise executable plan may stall automation.

Fix: replace with automated verification using `go list -m -json`, checksum DB, license/source check where possible, then document result.

### [LOW]: Cross-AI instructions conflict
User provided hard rule says **Codex → call Gemini** with a specific command. Plan 06 says **Codex + Gemini in parallel** and uses `codex exec` too. That exceeds the stated rule.

Fix: use the exact Gemini-only invocation from AGENTS.md unless the user explicitly asks for an additional Codex review.

## Suggestions
- Add a requirement-to-test matrix in `04-06-SUMMARY.md` before execution, not only after.
- Add a task-level check for `grep -R "state.Handlers" services/api` returning zero.
- Add a check that `GetBreakReasonForState` still filters `enabled = TRUE`.
- Add a `task gen && git diff --exit-code` gate after every wave that touches OpenAPI/sqlc inputs.

## Verdict: READY WITH FIXES
tokens used
110,129
## Summary
Plan bundle is broadly complete and captures the major Phase 4 invariants, especially `force=true` probe ordering, `state.Server`, D-93 atomic seeding, and TTL bypass context. Verdict is **READY WITH FIXES** because there are several plan-level defects likely to cause compile/test failures or wave-order drift.

## Strengths
- All 5 critical pitfalls are captured: P1, P3, P6, P8, P10.
- D-94 cross-org probes are explicitly named, including `force=true` still returning 422.
- Cache strategy is mostly correct: singular `agent_state` key, `cache.Del` after write, sweeper invalidates using returned `org_id`.
- D-93 is well-covered: `CreateAgent` seeds `agent_states` inside existing `qtx`.
- Spec amendment is scoped to request body only; codegen drift gate is included.

## Concerns

### [HIGH]: Wave 3 is not actually parallel-safe with Wave 2
Wave dependency says `Wave 0 → 1 → {2,3 parallel} → 4 → 5`, but `04-04` TTL tests depend on `services/api/internal/state/testutil_test.go` and helpers created in `04-03`. `04-04 depends_on` is only `04-02`.

Fix: set `04-04 depends_on: [04-03]`, or move shared testutil scaffolding into Wave 1/2 before both branches.

### [HIGH]: `ttl.go` plan includes likely non-compiling dead code
`jitter()` uses `int64(uuid.New().Time())`, which is unsafe against the `google/uuid` API and is unused. The plan also adds a `pgtype` import plus `var _ pgtype.Text` as an unused-import guard. This is scope creep and likely violates comment/dead-code hygiene.

Fix: delete `jitter()` from Wave 3 unless used by a real system transition path. Keep D-87 as deferred. Remove `pgtype` import/guard.

### [MED]: Acceptance test wrapper for `IsRoutable` does not assert behavior
`TestAcceptance_IsRoutableMatrix` only logs a pointer to another test. That gives traceability, but it is not an acceptance test.

Fix: either call `domain.IsRoutable` directly for the 4 ROADMAP cases, or rely on `internal/domain/TestIsRoutable` in the mapping and remove the wrapper.

### [MED]: Comment hygiene is weak in the planned snippets
Many inserted comments explain what/how, not why, especially in test scaffolding, SQL comments, handler steps, and `notimpl.go`. This conflicts with the project hard rule.

Examples to trim:
- “GetAgentStatus serves…”
- “STEP 1 — BREAK_REASON PROBE”
- “Create `ttl.go` with 6 functions”
- Long explanatory comments in generated-plan snippets.

Fix: keep only invariant/rationale comments: D-84 probe-before-force, D-93 qtx atomicity, D-95 startup sweep, P8 bypass.

### [MED]: `UpdateAgentStateStatus` null assignment semantics need explicit validation
SQL directly assigns `break_reason_id = sqlc.narg(...)`, `engaged_channel = sqlc.narg(...)`, `wrapup_until = sqlc.narg(...)`. Omitted fields become NULL. That may be intended when leaving Break, but the plan does not state cleanup rules per target status.

Fix: add acceptance criteria for field clearing:
- Break sets `break_reason_id`.
- Ready/NotReady clear `break_reason_id`, `engaged_channel`, `wrapup_until` if intended.
- Force to Engaged with PIS does not accidentally erase needed fields.

### [LOW]: Wave 0 human checkpoint for clockwork is unnecessary friction
User context says stack locked and package decision is already in planning. A blocking human checkpoint inside an otherwise executable plan may stall automation.

Fix: replace with automated verification using `go list -m -json`, checksum DB, license/source check where possible, then document result.

### [LOW]: Cross-AI instructions conflict
User provided hard rule says **Codex → call Gemini** with a specific command. Plan 06 says **Codex + Gemini in parallel** and uses `codex exec` too. That exceeds the stated rule.

Fix: use the exact Gemini-only invocation from AGENTS.md unless the user explicitly asks for an additional Codex review.

## Suggestions
- Add a requirement-to-test matrix in `04-06-SUMMARY.md` before execution, not only after.
- Add a task-level check for `grep -R "state.Handlers" services/api` returning zero.
- Add a check that `GetBreakReasonForState` still filters `enabled = TRUE`.
- Add a `task gen && git diff --exit-code` gate after every wave that touches OpenAPI/sqlc inputs.

## Verdict: READY WITH FIXES
