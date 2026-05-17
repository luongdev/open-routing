# Phase 4: Agent State Machine (Go) - Context

**Gathered:** 2026-05-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the agent status state machine (STATE-01 through STATE-10) on top of Phase 3's catalog: a single `agent_states` row per agent persists `status`, `engaged_channel`, `break_reason_id`, `post_interaction_state`, `wrapup_until`, and `state_version`; `PATCH /v1/orgs/{org_id}/agents/{id}/status` validates against the locked transition matrix (D-91); a Go background goroutine fires `WrapUp → post_interaction_state` transitions independently of client connectivity (STATE-07); `IsRoutable` ships as a pure function in `services/api/internal/domain`; an admin `force` flag on PATCH unblocks operational recovery; agent_states cache reuses Phase 3's `cache.GetOrSet[T]`.

**In scope:** new `services/api/internal/state/` package implementing `state.Handlers` (StrictServerInterface methods for `GetAgentStatus` + `PatchAgentStatus`); new `services/api/internal/domain/` package with `IsRoutable` pure function + 4 unit tests; transition matrix encoded as `map[AgentStatus]map[AgentStatus]TransitionRule` in `internal/state/transitions.go`; WrapUp TTL goroutine via `time.AfterFunc` per agent + startup sweep + 30s safety sweep; appended migration `agent_states` table in `migrations/000002_catalog_v0_1.up.sql` (per D-61, not a new migration file); sqlc queries at `services/api/internal/db/queries/agent_states.sql`; spec amendment adding `force: bool` to `PatchAgentStatusRequest` (codegen drift cycle); modification to `services/api/internal/catalog/agents.go` `CreateAgent` to also INSERT initial agent_states row inside the existing OrgTx (Codex C4 atomic pattern); top-level `ApiHandlers` composite struct (D-70 activation) embedding `*catalog.Handlers` + `*state.Handlers` wired in `cmd/api/main.go`; cross-org isolation tests in `services/api/test/isolation/state_test.go`.

**Out of scope:** system-initiated transitions Ready→Engaged + Engaged→WrapUp (deferred to v0.2 runtime engine — schema columns ship in v0.1, transitions don't fire); login/logout transitions for STATE-09 (deferred to AUTH phase — no real session lifecycle with stub `X-Org-Id` auth); per-org configurable post_interaction_state default (deferred to v0.2 — v0.1 uses prior-status semantic per D-83); per-org configurable WrapUp duration (deferred — v0.1 hardcodes duration constant); multi-replica distributed scheduling for TTL sweeper (deferred to v1 — v0.1 single-replica idempotent UPDATE handles concurrent firings safely); real RBAC enforcement of `force=true` (deferred to AUTH phase — v0.1 stub auth lets any caller pass it, logged WARN); audit-trail for force=true usage (deferred); Phase 5 bulk import; Phase 6/7 frontend code.

</domain>

<decisions>
## Implementation Decisions

### Schema (agent_states table)

- **D-78:** **One row per agent_id, PRIMARY KEY (agent_id).** `agent_states` table appended to `migrations/000002_catalog_v0_1.up.sql` per D-61 (do NOT create `000003_*` for v0.1). Columns: `agent_id UUID PRIMARY KEY`, `org_id UUID NOT NULL`, `status TEXT NOT NULL CHECK (status IN ('Ready','NotReady','Break','Engaged','WrapUp','Offline'))`, `engaged_channel TEXT NULL CHECK (engaged_channel IS NULL OR engaged_channel IN ('voice','chat','email'))`, `break_reason_id UUID NULL`, `post_interaction_state TEXT NULL CHECK (post_interaction_state IS NULL OR post_interaction_state IN ('ready','not_ready'))`, `wrapup_until TIMESTAMPTZ NULL`, `state_version BIGINT NOT NULL DEFAULT 1`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`. Indexes: `ix_agent_states_org_status (org_id, status)` for sweeper queries; `ix_agent_states_wrapup_until (wrapup_until) WHERE status = 'WrapUp'` partial index for sweeper hot path. `agent_id` column is denormalized to `org_id` for orgDB SQLChecker (Phase 1 D-25 invariant — every query mentions org_id).
- **D-79:** **TEXT + CHECK encoding for status, engaged_channel, post_interaction_state** (NOT Postgres ENUM). Adding a value = ALTER constraint, not ALTER TYPE (which locks the table on PG14-). Matches Phase 3 patterns (`channels.channel_type` already TEXT+CHECK). sqlc emits Go strings; the Go code wraps them as `type AgentStatus string` typed constants in `internal/state/types.go`.
- **D-80:** **No foreign key constraints.** Extension of D-76 (Phase 3) — re-confirmed by user during Phase 4 discussion: "Hệ thống này không nên dùng FK nhé, nếu đã dùng thì phải lên plan bỏ đi. Rất khó maintain." `agent_states.agent_id` does NOT `REFERENCES agents(id)`; `agent_states.break_reason_id` does NOT `REFERENCES break_reasons(id)`. App-layer probes validate references at write time (D-76 pattern). Verified: Phase 3 migration `000002_catalog_v0_1.up.sql` introduces zero FKs (grep -E "REFERENCES|FOREIGN KEY" returns nothing). Phase 4 maintains this invariant; any future PR adding REFERENCES must be rejected by review.

### State Machine (transitions, IsRoutable, force)

- **D-81:** **WrapUp TTL via `time.AfterFunc` per agent (primary) + startup sweep + 30s periodic safety sweep.** Sub-millisecond firing on normal path. On Engaged→WrapUp transition (deferred to v0.2 but schema-ready), the handler calls `state.Handlers.scheduleWrapUpExpiry(agentID, wrapupUntil)` which spawns a `time.AfterFunc` keyed by agent_id in an in-memory map (mu-locked). Startup sweep on `main.go` boot scans `WHERE wrapup_until < NOW() AND status='WrapUp'` and fires-immediately + re-schedules timers for future expirations. 30s safety-sweep ticker catches anything timer missed (panic, drift). All paths use the same idempotent UPDATE: `UPDATE agent_states SET status=post_interaction_state, wrapup_until=NULL, state_version=state_version+1 WHERE agent_id=$1 AND status='WrapUp' AND wrapup_until < NOW()` — concurrent firings (timer + sweep, or future multi-replica) compete safely; only one wins per row. Clock injected via `state.WithClock(c clock.Clock)` Option (mirrors catalog D-71). v1 multi-replica needs distributed lock — documented deferred.
- **D-82:** **System-initiated transitions schema-ready, fires deferred.** `agent_states` columns (engaged_channel, post_interaction_state, wrapup_until) ship in v0.1 but the v0.1 server only fires AGENT-initiated transitions via PATCH /status (per matrix in PatchAgentStatusRequest description) + WrapUp TTL sweeper. Ready→Engaged + Engaged→WrapUp deferred to v0.2 runtime engine (no caller for them in v0.1). Login/logout STATE-09 (Offline↔NotReady) deferred to AUTH phase. Tests cover Engaged/WrapUp states by direct DB INSERT/UPDATE in test fixtures (bypasses transition matrix — testutil_test.go helper).
- **D-83:** **`post_interaction_state` semantics: default to prior status at Ready/NotReady→Engaged transition.** When v0.2 runtime fires Ready→Engaged, the handler captures the agent's prior status into `agent_states.post_interaction_state` (Ready agent → ready; NotReady agent → not_ready). Agent may override via PATCH during Engaged (STATE-06 explicit choice). WrapUp TTL reads `post_interaction_state` directly. User clarification: "Sau wrapup thì nó phải về trạng thái trước khi nhận interaction (trước đó là ready thì sẽ là ready, là not-ready thì là not ready)."
- **D-84:** **`force: bool` flag on PatchAgentStatusRequest.** Spec amendment adds `force` field (default false). `force=true` bypasses the transition matrix (skips 409 invalid_transition validation). Cross-row probes STILL RUN (break_reason_id must exist in same org → 422 invalid_reference per D-75/D-76). Logged WARN: `slog.WarnContext(ctx, "state.force.applied", "agent_id", id, "from", from, "to", to, "requester_org", orgID)`. v0.1 stub auth: any caller can pass `force=true` (no role enforcement). v1 AUTH phase: restrict to `org_admin` role; non-admin gets 403. Rationale: operational recovery story — if runtime crashes leave an agent stuck (e.g., Engaged with no runtime to fire Engaged→WrapUp), admin needs to force-reset without SQL-level DB access.
- **D-85:** **`state_version` is read-side only — no expected_state_version on PATCH.** Matches the OpenAPI spec (`PatchAgentStatusRequest` has NO version field). Concurrent PATCHes from the same agent surface as transition-matrix `from` mismatches (second PATCH sees `from=Break` expecting `Ready` → 409 invalid_transition). UI uses `state_version` to detect stale polling responses per STATE-08. No spec edit required for this decision. Departs from CAT-08's optimistic locking — state transitions are single-agent-single-user in practice; the matrix prevents most conflicting writes.
- **D-90:** **`IsRoutable` pure function in `services/api/internal/domain/state.go`.** Signature: `func IsRoutable(in AgentStateInputs) bool` where `AgentStateInputs struct { Status string; BreakReasonRoutable bool }`. Returns `true` only when `Status == "Ready"` OR (`Status == "Break"` AND `BreakReasonRoutable`). Caller responsible for loading `break_reasons.routable` (may hit cache or DB). 4 unit tests cover the matrix: Ready=true; Break+routable=true; Break+!routable=false; NotReady/Engaged/WrapUp/Offline=false. NO db access from `domain` package — pure transformation.
- **D-91:** **Transition matrix encoded as `map[AgentStatus]map[AgentStatus]TransitionRule`.** Pure data structure in `services/api/internal/state/transitions.go`. Each rule carries flags: `requiresBreakReasonId bool`, `requiresEngagedChannel bool`, `agentInitiated bool`. Validator function `validateTransition(from, to AgentStatus, req PatchAgentStatusRequest) (TransitionRule, error)` returns the rule on success, or an error wrapping the 409 InvalidTransitionErrorResponse on failure. Table-driven test coverage exhaustively walks every (from, to) pair from the matrix. NO XState, NO state-machine library (PROJECT.md locked decision — pure Go).

### Cache + Concurrency

- **D-86:** **Cache state lookups via existing `cache.GetOrSet[T]` (D-49..D-60).** Key: `or:{orgId}:agent_state:{agent_id}`. TTL 60s same as catalog (D-59). DEL on every state mutation per D-55. Refresh-ahead (D-53) + singleflight (D-52) apply unchanged. State type cached is the DTO `api.AgentState` (NOT the sqlc row type) — same pattern as catalog handlers per D-49.
- **D-87:** **Thundering herd protection — jitter `wrapup_until` ±100ms when system sets it.** When v0.2 runtime fires Engaged→WrapUp (or v0.1 tests seed WrapUp state directly), wrapup_until is computed as `NOW() + duration + time.Duration(rand.Intn(200)-100)*time.Millisecond`. Spreads expiry over a 200ms window when many agents enter WrapUp simultaneously. Cheap (one rand call), transparent to clients (timestamp already opaque, polled). Avoids hot batch UPDATE when many AfterFunc timers fire in the same Go scheduler tick.

### Package Layout (state.Handlers + ApiHandlers composite)

- **D-88:** **New `services/api/internal/state/` package** with files: `handlers.go` (Deps struct + New constructor + Start/Stop lifecycle for sweeper), `transitions.go` (matrix + validator), `ttl.go` (AfterFunc map + sweeper goroutine + startup sweep), `agent_states.go` (GetAgentStatus + PatchAgentStatus implementations), `agent_states_test.go` beside source (D-72), `testutil_test.go` (D-73). Constructor mirrors catalog: `state.New(deps Deps, opts ...Option) *Handlers` where `Deps{ OrgDBFactory db.Factory; Cache *cache.Cache; Logger *slog.Logger; WrapUpDuration time.Duration }` and `WithClock(c clock.Clock) Option` exposed for tests.
- **D-89:** **`ApiHandlers` composite struct in `cmd/api/main.go`** — D-70 activation. Declaration:
  ```go
  type ApiHandlers struct {
      *catalog.Handlers
      *state.Handlers
  }
  ```
  `main.go` constructs both, wraps in ApiHandlers, passes to `api.NewStrictHandler`. Method-set resolution merges disjoint endpoint sets (catalog owns 41 catalog endpoints; state owns 2 status endpoints); the v0.1 `Unimplemented` fallbacks for bulk-import endpoints remain in `catalog/notimpl.go` for Phase 5 to displace. Compile-time check: if two embedded types claimed the same method, Go would reject — guards against accidental overlap.
- **D-95:** **Sweeper goroutine lifecycle owned by `state.Handlers`.** `main.go` calls `stateHandlers.Start(ctx)` after construction and `defer stateHandlers.Stop()` (Stop cancels the internal ctx, waits for sweeper to drain in-flight UPDATEs, cancels all pending AfterFunc timers via `timer.Stop()` on each entry in the in-memory map). Startup sweep runs synchronously inside `Start(ctx)` before returning — guarantees no stuck WrapUps after a restart. Graceful shutdown matches Phase 1's pattern (server shutdown with 30s ctx).

### Spec amendment + Carry-forward modifications

- **D-92:** **Spec amendment to `openapi/openapi.yaml`** — add `force: { type: boolean, default: false, nullable: true, description: "Admin override..." }` to `PatchAgentStatusRequest.properties`. Codegen drift triggers `task gen` (oapi-codegen + openapi-typescript regeneration); CI gate catches any commit missing the regenerated `.gen.go` / `generated.ts` (per D-47, D-48). D-32 (Phase 3 freeze on PATHS) does NOT apply — D-32 only locked paths, not request-body field additions.
- **D-93:** **Agent CREATE inserts initial `agent_states` row atomically.** Phase 4 modifies `services/api/internal/catalog/agents.go` `CreateAgent` handler to issue a second INSERT inside the existing `OrgDB.BeginTx` (the Codex C4 atomic pattern for agent+skills replace). The state row is created with `status='Offline'`, `state_version=1`, all nullable columns NULL. Initial row is REQUIRED because `GET /agents/{id}/status` returns the row directly — no synthesis. Failure of the state INSERT fails the entire transaction (rollback rolls back the agent INSERT too). Pitfall to capture in PATTERNS.md: any future agent INSERT path (bulk import in Phase 5) must also insert the agent_states row.
- **D-94:** **Cross-org isolation tests at `services/api/test/isolation/state_test.go`** (new file, mirrors `catalog_test.go` pattern from Phase 3 Wave 6). FOUND-08 acceptance probes: (a) GET /status across orgs returns 404, never 200; (b) PATCH /status with cross-org break_reason_id returns 422 invalid_reference; (c) PATCH /status with force=true does NOT bypass cross-org break_reason_id check (still 422); (d) PATCH /status with cross-org agent_id returns 404. All tests acquire fresh org UUIDs via `freshOrg(t)` per Pitfall 6 (no package-level org constants).

### Claude's Discretion

- Sweeper safety-sweep interval (locked at 30s; planner may tune to 10-60s based on load testing).
- AfterFunc handle storage shape (`map[uuid.UUID]*time.Timer` keyed by agent_id with `sync.RWMutex`, or `sync.Map` — planner picks; mutex preferred for explicit lock scope).
- WrapUp duration constant location (`internal/state/config.go` vs hardcoded in handlers.go — planner picks; const preferred).
- Clock interface (`type Clock interface { Now() time.Time; AfterFunc(...) *time.Timer }`) vs `func() time.Time` — planner picks; interface preferred for testability of AfterFunc-driven code.
- `notimpl.go` cleanup — Phase 4 removes the two state-machine 501-stub methods from `catalog/notimpl.go` since `state.Handlers` now owns them.
- Test seed pattern for Engaged/WrapUp states — direct sqlc-generated `InsertAgentStateRaw` helper in testutil_test.go vs orchestrating through `state.Handlers` internal API — planner picks; raw INSERT preferred for isolation.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level locks
- `.planning/PROJECT.md` — Locked stack (Go + chi + sqlc + pgx + slog; PostgreSQL 17 + Redis); Key Decisions include "No XState in v0.1; pure Go transition table in `services/api/internal/domain`" and "Performance is part of the product value, not a later optimization."
- `.planning/REQUIREMENTS.md` §Agent State Model — **STATE-01 through STATE-10**. STATE-01 (status enum), STATE-02 (allowed transitions), STATE-03 (invalid → 409 with from/to), STATE-04 (Break requires break_reason_id; cross-org → 422), STATE-05 (Engaged carries engaged_channel), STATE-06 (post_interaction_state set while Engaged), STATE-07 (server-owned WrapUp TTL goroutine), STATE-08 (monotonic state_version), STATE-09 (login/logout system transitions — deferred to AUTH per D-82), STATE-10 (IsRoutable helper).
- `.planning/ROADMAP.md` §Phase 4 — Goal statement and 5 success criteria.
- `.planning/STATE.md` §Accumulated Context — LOCKED carry-forwards including "WrapUp TTL with multi-replica (Phase 4): Go goroutine scheduler works single-process; flag before v1 multi-replica" and "postInteractionState default configurability (Phase 4): product decision needed."

### Phase 1 carry-forward (LOCKED)
- `.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md` — D-01 through D-31. Especially D-19/D-20 (UUIDv7 + X-Org-Id), D-21 (bypass list), D-22/D-23 (golang-migrate + project-root migrations/), D-25 (per-request *db.OrgDB + SQLChecker), D-28/D-29 (RequestID + slog trace_id), D-30 (CI shape).

### Phase 2 carry-forward (LOCKED)
- `.planning/phases/02-openapi-contract-codegen/02-CONTEXT.md` — D-32 through D-48. Especially D-32 (PATHS frozen — field additions OK), D-35/D-37 (error envelope + request_id), D-41/D-42/D-44 (strict-server + NewStrictHandler), D-47/D-48 (codegen-drift CI).

### Phase 3 carry-forward (LOCKED — heavily reused)
- `.planning/phases/03-catalog-crud-go/03-CONTEXT.md` — D-49 through D-77 + A6/A7 amendments. Especially:
  - **D-49..D-60 (cache layer):** Phase 4 reuses `cache.GetOrSet[T]` (D-51), singleflight (D-52), refresh-ahead (D-53), DEL after commit (D-55), 409 also DELs (D-56), key format (D-58), 60s TTL (D-59). State cache key: `or:{orgId}:agent_state:{agent_id}` per D-86.
  - **D-61:** Single editable migration `migrations/000002_catalog_v0_1.up.sql`. Phase 4 APPENDS `agent_states` table + indexes here.
  - **D-66:** Atomic UPDATE pattern with 0-row → SELECT disambiguate 404/409. Phase 4's PATCH /status uses analogous pattern (no version check in WHERE per D-85).
  - **D-68:** Single `internal/catalog/` package. Phase 4 introduces SEPARATE `internal/state/` package per D-88.
  - **D-70:** Forward-compat composite `ApiHandlers` — Phase 4 ACTIVATES this struct per D-89.
  - **D-71:** Hybrid constructor `New(deps Deps, opts ...Option)`. Phase 4 mirrors for `state.New`.
  - **D-72, D-73:** Per-entity `_test.go` beside source + `testutil_test.go`. Phase 4 follows.
  - **D-74, D-75, D-76:** Two-layer validation; `invalid_reference` ErrorCode; D-76 NO database-level FKs across catalog tables. Phase 4 **extends D-76** with D-80: no FKs from agent_states.
  - **D-77:** Scaffold deletion already done in Phase 3.

### Existing code Phase 4 touches/extends
- `openapi/openapi.yaml` (lines 980-1121, 1753-1850) — `AgentStatus` enum, `AgentState` schema, `PatchAgentStatusRequest` body, GET/PATCH /status endpoints. Phase 4 amends to add `force` field (D-92).
- `services/api/internal/api/server.gen.go` — `GetAgentStatusRequestObject`, `PatchAgentStatusRequestObject`, `AgentState`. Regenerated on spec change.
- `services/api/internal/api/types.gen.go` — `AgentStatus`/`EngagedChannel`/`PostInteractionState` enum constants. Regenerated on spec change.
- `services/api/internal/catalog/agents.go` — Phase 4 modifies `CreateAgent` to INSERT agent_states row inside existing OrgTx (D-93).
- `services/api/internal/catalog/notimpl.go` — Phase 4 removes `GetAgentStatus` + `PatchAgentStatus` 501-stubs (state.Handlers owns them now).
- `services/api/internal/server/server.go` — `injectRequestIDIntoErrorResponse` type switch must add new state response types or `request_id_exhaustiveness_test.go` will fail.
- `services/api/cmd/api/main.go` — wires `state.New`, ApiHandlers composite, `stateHandlers.Start(ctx)` for sweeper, `defer stateHandlers.Stop()`.
- `migrations/000002_catalog_v0_1.up.sql` — APPENDED with agent_states table + indexes per D-78. Down migration: DROP TABLE IF EXISTS agent_states at top.
- `services/api/internal/db/queries/agent_states.sql` — new file with InsertAgentState, GetAgentState, UpdateAgentStateStatus, ListWrapUpsToExpire, ExpireWrapUp.
- `services/api/internal/db/orgdb.go` — SQLChecker `tenantTables` allowlist must add `agent_states` (executor deviation precedent from Phase 3 Wave 1).
- `web/packages/ui/src/api/generated.ts` — REGENERATED when openapi.yaml gains `force`. No Phase 4 hand-edits.

### External standards
- [Go time.AfterFunc](https://pkg.go.dev/time#AfterFunc) — primary firing (D-81)
- [Go time.NewTicker](https://pkg.go.dev/time#NewTicker) — 30s safety sweep (D-81)
- [PostgreSQL CHECK constraints](https://www.postgresql.org/docs/17/ddl-constraints.html#DDL-CONSTRAINTS-CHECK-CONSTRAINTS) — enum encoding (D-79)
- [PostgreSQL partial indexes](https://www.postgresql.org/docs/17/indexes-partial.html) — sweeper index (D-78)
- No new Go module deps. time, math/rand stdlib; pgx/singleflight already in go.mod.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`cache.GetOrSet[T]`** at `services/api/internal/cache/cache.go` — Generic GetOrSet with singleflight + refresh-ahead. Phase 4 reuses identically: `cache.GetOrSet(ctx, c, cache.Key(orgID, "agent_state", agentID), 60*time.Second, loader)`.
- **`OrgDB.BeginTx(ctx) → *OrgTx`** at `services/api/internal/db/orgdb.go` — Phase 4's PATCH /status (with cache DEL) uses a tx; CreateAgent extension reuses existing tx wrapping agent INSERT + skills replace + new agent_states INSERT.
- **`generated.New(orgDB)` sqlc constructor** — Phase 4 adds agent_states.sql.go to the generated set.
- **`middleware.WriteError` + strict-server typed responses** — Phase 4 uses `PatchAgentStatus409JSONResponse(InvalidTransitionErrorResponse{...})` and `PatchAgentStatus422JSONResponse(ErrorResponse{...})`. RequestIDInjectionMiddleware adds new response types to its type switch.
- **`middleware.UUIDv7PathParams`** — Already validates `{id}` path params. Phase 4 handlers can assume id is a valid UUIDv7.
- **`testsupport.Setup(t)` testcontainers + miniredis pattern (D-73)** — Phase 4 test files reuse setup; no new test infra.
- **Phase 3 Codex C4 atomic pattern in `catalog/agents.go`** — Phase 4 extends CreateAgent to also INSERT agent_states inside this tx.

### Established Patterns
- **TEXT + CHECK enum encoding** (Phase 3 `channels.channel_type`, `adapters.adapter_type`) — Phase 4 follows for status, engaged_channel, post_interaction_state per D-79.
- **No FK constraints** (D-76, re-confirmed D-80) — App-layer probes only. `BreakReasonExistsAndRoutableInOrg` probe for STATE-04 + IsRoutable cache hits.
- **Per-entity `_test.go` beside source + `testutil_test.go`** (D-72/D-73) — Phase 4 follows for `internal/state/`.
- **D-66 atomic UPDATE + 0-row → SELECT disambiguate** — Phase 4 PATCH /status UPDATE returns 0 rows → handler issues `GetAgentState` to distinguish 404 vs 409 invalid_transition.

### Integration Points
- **Phase 2 codegen-drift CI** — Phase 4 spec amendment (D-92 force field) triggers `task gen`. CI fails if regenerated artifacts not committed.
- **Phase 3 cache package** — Phase 4 imports `internal/cache`; no changes to the cache package itself.
- **Phase 3 `catalog.Handlers`** — Phase 4 modifies `catalog/agents.go` CreateAgent for agent_states initial INSERT (D-93). Other catalog handlers untouched.
- **Phase 5 (Bulk Import)** — MUST insert agent_states row for every new agent inserted via CSV/JSON import. Pitfall captured in PATTERNS.md per D-93.
- **Phase 6/7 (Frontend)** — Consume regenerated TS types (`force` field on PatchAgentStatusJSONRequestBody).
- **v0.2 Runtime engine** — Will fire Ready→Engaged and Engaged→WrapUp. Phase 4's schema (engaged_channel, post_interaction_state, wrapup_until) is the contract.
- **v1 AUTH phase** — Will wire login/logout transitions per STATE-09; will restrict `force=true` to org_admin role.

</code_context>

<specifics>
## Specific Ideas

- **"Performance is part of the product value"** — User pushed back on the 2s sweeper proposal: "Goal là hiệu suất cao mà mày để tận 2s mới đổi được trạng thái??? SLA 50ms cơ mà?" Even though p95 < 50ms SLA in PROJECT.md applies to runtime route-decision (not WrapUp expiry), the architectural principle holds: don't ship lazy code. WrapUp TTL goes per-agent timer (sub-ms) instead of polling sweeper.
- **No FK constraints — project-wide invariant** — User: "Hệ thống này không nên dùng FK nhé, nếu đã dùng thì phải lên plan bỏ đi. Rất khó maintain." Phase 3 D-76 established this; Phase 4 D-80 extends. Verified zero FKs in current migrations.
- **Admin force-override for operational recovery** — User: "Trong thực tế, hệ thống lỗi không recover được trạng thái agents thì sao? Vậy thì phải cho org admin có quyền force trạng thái." Solved via D-84 (`force: bool` flag).
- **post_interaction_state semantic = prior status** — User: "Sau wrapup thì nó phải về trạng thái trước khi nhận interaction (trước đó là ready thì sẽ là ready, là not-ready thì là not ready)." Default semantic via D-83; agent override via PATCH per STATE-06.
- **Single-replica acknowledged for v0.1, multi-replica deferred to v1** — Both PROJECT.md and STATE.md flagged this. D-81 documents v1 distributed-lock path; idempotent UPDATE protects against early-stage scaling experiments.
- **Goroutine scale concern raised + dismissed** — User: "có sợ khi số lượng agent tăng lớn, goroutine không chạy nổi nữa không?" Go handles 100K+ goroutines easily; real bottleneck = thundering herd DB writes; mitigated by D-87 ±100ms jitter.

</specifics>

<deferred>
## Deferred Ideas

- **Distributed lock for multi-replica TTL sweeping (v1)** — Use `pg_advisory_xact_lock` or `SELECT FOR UPDATE SKIP LOCKED` once v1 ships multi-replica.
- **Per-org configurable post_interaction_state default (v0.2)** — v0.1 uses prior-status semantic; per-org override deferred.
- **Per-org configurable WrapUp duration (v0.2)** — v0.1 hardcodes duration constant.
- **Real RBAC for `force=true` (v1 AUTH phase)** — v0.1 stub auth lets any caller pass force; logged WARN is audit trail.
- **Audit-trail for force-override usage** — Append to a `state_audit` table or v1 outbox.
- **Engaged → WrapUp auto-transition (v0.2 runtime engine)** — Schema-ready in v0.1.
- **Login/logout system transitions for STATE-09 (AUTH phase)** — Schema-ready in v0.1; fires deferred.
- **Ready → Engaged auto-transition (v0.2 runtime engine)** — System fires when assigning interaction.
- **OTel state-transition metrics export** — v0.1 uses slog attrs only.
- **State history table (event sourcing)** — v0.1 only has state_version; full event log defers to runtime + outbox.
- **Per-channel sub-states for Engaged (voice-call-active, voice-call-on-hold, ...)** — v1 adapter SDK concern.
- **WrapUp extension by agent ("give me 30 more seconds")** — Not in STATE-* requirements.
- **State change webhooks / SSE push** — REST polling only in v0.1; push channels arrive with runtime engine.
- **`force=true` with idempotency key** — Deferred; current design: force on valid transition is silently allowed (no-op behavior change).

</deferred>

<amendments>
## Amendments

*(No amendments yet — initial Phase 4 CONTEXT.md, written 2026-05-17.)*

</amendments>

---

*Phase: 4-Agent State Machine (Go)*
*Context gathered: 2026-05-17*
