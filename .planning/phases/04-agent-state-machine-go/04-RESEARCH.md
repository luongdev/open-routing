# Phase 4: Agent State Machine (Go) — Research

**Researched:** 2026-05-17
**Domain:** Go server-side state-machine + WrapUp TTL goroutine + cross-row probes + spec amendment
**Confidence:** HIGH (all primary patterns verified against in-tree Phase 3 code; one MEDIUM area: clock library choice)

## Summary

Phase 4 sits entirely on top of Phase 3 infrastructure. No new conceptual primitives are required — every load-bearing pattern (cache, OrgTx, D-76 probe, D-66 disambiguate, spec amendment + codegen drift, per-entity `_test.go` + shared `testutil_test.go`, cross-org isolation suite) is already a working pattern in the Phase 3 codebase. The novel work is:

1. A small Go transition matrix (data, not framework) + 422/409 validator.
2. A `time.AfterFunc` registry (in-memory map + RWMutex) wrapped by Start/Stop lifecycle and a 30s safety-sweep ticker.
3. A new `internal/state/` package implementing two StrictServerInterface methods and a new `internal/domain/` package containing a single pure function (`IsRoutable`).
4. Mechanical extensions: spec amendment for the `force` flag (D-92), migration append for the `agent_states` table (D-78), `tenantTables` allowlist entry, two new sqlc probes for break_reasons, and `CreateAgent` extended to INSERT the initial state row inside the existing tx.
5. `ApiHandlers` composite struct activation in `cmd/api/main.go` (D-89 — first time D-70 is exercised).

**Primary recommendation:** Sequence Phase 4 in 6 waves (Wave 0 = spec + migration + codegen + tenantTables; Wave 1 = sqlc queries + domain.IsRoutable + state pkg skeleton; Wave 2 = PATCH/GET handlers + cache + break_reason probes; Wave 3 = TTL goroutine + sweeper + Start/Stop; Wave 4 = ApiHandlers composite wiring + CreateAgent tx extension + notimpl.go cleanup; Wave 5 = cross-org isolation tests + force=true tests + acceptance harness). Use `github.com/jonboulle/clockwork v0.4+` as the injected Clock (MEDIUM confidence — Quartz is younger but more deterministic; clockwork is the proven default).

## User Constraints (from CONTEXT.md)

### Locked Decisions (D-78 .. D-95)

- **D-78:** `agent_states` appended to `migrations/000002_catalog_v0_1.up.sql`. PK=agent_id. Columns: `agent_id UUID PK`, `org_id UUID NOT NULL`, `status TEXT NOT NULL CHECK (status IN (...))`, `engaged_channel TEXT NULL CHECK (...)`, `break_reason_id UUID NULL`, `post_interaction_state TEXT NULL CHECK (...)`, `wrapup_until TIMESTAMPTZ NULL`, `state_version BIGINT NOT NULL DEFAULT 1`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`. Indexes: `ix_agent_states_org_status (org_id, status)` + partial `ix_agent_states_wrapup_until (wrapup_until) WHERE status='WrapUp'`. NO FK.
- **D-79:** TEXT + CHECK encoding for `status`, `engaged_channel`, `post_interaction_state` (NOT Postgres ENUM).
- **D-80:** NO foreign-key constraints — extends D-76 project-wide invariant. App-layer probes only.
- **D-81:** WrapUp TTL = per-agent `time.AfterFunc` (sub-ms) + startup sweep on boot + 30s periodic safety sweep. Idempotent UPDATE SQL: `UPDATE agent_states SET status=post_interaction_state, wrapup_until=NULL, state_version=state_version+1 WHERE agent_id=$1 AND status='WrapUp' AND wrapup_until < NOW()`. Clock injected via Option (D-71 mirror).
- **D-82:** System-initiated transitions schema-ready, fires deferred to v0.2. v0.1 only fires agent-initiated PATCH + WrapUp TTL.
- **D-83:** `post_interaction_state` defaults to prior status at Ready/NotReady→Engaged transition. Agent may override via PATCH during Engaged.
- **D-84:** `force: bool` flag on `PatchAgentStatusRequest`. `force=true` bypasses transition matrix (skips 409). Cross-row probes still run (break_reason_id same-org → 422). Logged WARN. v1 AUTH: org_admin role only.
- **D-85:** `state_version` is read-side only — no `expected_state_version` in PATCH body.
- **D-86:** Cache reuses `cache.GetOrSet[T]` with key `or:{orgId}:agent_state:{agent_id}`, TTL 60s, DEL on write.
- **D-87:** Jitter `wrapup_until` ±100ms when system sets it. Spreads expiry over a 200ms window.
- **D-88:** New `services/api/internal/state/` package: `handlers.go`, `transitions.go`, `ttl.go`, `agent_states.go`, plus `agent_states_test.go` + `testutil_test.go`.
- **D-89:** `ApiHandlers` composite struct in `cmd/api/main.go` — D-70 activation. Embeds `*catalog.Handlers` + `*state.Handlers`.
- **D-90:** `IsRoutable` pure function in `services/api/internal/domain/state.go`. Signature: `func IsRoutable(in AgentStateInputs) bool` where `AgentStateInputs{Status string; BreakReasonRoutable bool}`. 4 unit tests.
- **D-91:** Transition matrix as `map[AgentStatus]map[AgentStatus]TransitionRule`. NO XState. Validator wraps 409 `InvalidTransitionErrorResponse`.
- **D-92:** Spec amendment adds `force: bool` to `PatchAgentStatusRequest`. Codegen drift CI re-triggers.
- **D-93:** `CreateAgent` INSERTs initial `agent_states` row atomically inside existing `OrgTx` (Codex C4 pattern). Initial: `status='Offline'`, `state_version=1`. Pitfall: Phase 5 bulk import must do the same.
- **D-94:** Cross-org isolation tests at `services/api/test/isolation/state_test.go`. Probes: (a) GET cross-org → 404; (b) PATCH cross-org break_reason_id → 422; (c) `force=true` does NOT bypass cross-org probe → 422; (d) PATCH cross-org agent → 404.
- **D-95:** Sweeper goroutine lifecycle owned by `state.Handlers`. `main.go` calls `Start(ctx)` + `defer Stop()`. Startup sweep synchronous. Stop drains in-flight + cancels timers.

### Claude's Discretion (planner picks)

- Sweeper safety-sweep interval (locked at 30s; tunable 10–60s).
- AfterFunc handle storage shape (`map[uuid.UUID]*time.Timer` keyed by agent_id with `sync.RWMutex` — recommended) vs `sync.Map` (discouraged: harder to iterate at Stop()).
- WrapUp duration constant location (`internal/state/config.go` vs hardcoded in handlers.go — pick `config.go` const).
- Clock interface (`type Clock interface { Now() time.Time; AfterFunc(d, f) Timer }` vs `func() time.Time` — pick **interface** so AfterFunc is also injectable).
- `notimpl.go` cleanup — Phase 4 removes both `GetAgentStatus` + `PatchAgentStatus` stubs.
- Test seed pattern for Engaged/WrapUp states — direct `qtx.InsertAgentStateRaw` helper in `testutil_test.go` (preferred — bypasses transition matrix for fixture setup).

### Deferred Ideas (OUT OF SCOPE)

- Distributed lock for multi-replica TTL sweeping (v1).
- Per-org configurable `post_interaction_state` default (v0.2).
- Per-org configurable WrapUp duration (v0.2).
- Real RBAC for `force=true` (v1 AUTH).
- Audit-trail for force-override usage (v1).
- Engaged→WrapUp auto-transition (v0.2 runtime).
- Login/logout system transitions for STATE-09 (AUTH phase).
- Ready→Engaged auto-transition (v0.2 runtime).
- OTel state-transition metrics (v0.2).
- State history table (event sourcing) (runtime + outbox).
- Per-channel sub-states for Engaged (v1 adapter SDK).
- WrapUp extension by agent (not in STATE-* reqs).
- State-change webhooks / SSE push (runtime engine).
- `force=true` with idempotency key.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| STATE-01 | agent_states row per agent with status enum | §1.1 schema; §3 sqlc queries; §11 D-93 atomic INSERT |
| STATE-02 | PATCH /agents/{id}/status accepts agent-initiated transitions | §2 matrix; §3 atomic UPDATE; §6 handler shape |
| STATE-03 | Invalid transitions → HTTP 409 with `{from, to, error}` | §2 validator; §6 PatchAgentStatus409JSONResponse usage |
| STATE-04 | Ready→Break requires same-org break_reason_id; cross-org/missing → 422 | §9 BreakReasonExistsInOrg probe; §11 isolation tests |
| STATE-05 | Engaged carries engaged_channel | §1 schema column + CHECK; §6 spec already requires it |
| STATE-06 | Agent sets post_interaction_state while Engaged | §6 PATCH branch; §2 transition rule flag |
| STATE-07 | Server-owned TTL goroutine fires WrapUp→post_interaction_state | §4 ttl.go design; §5 startup sweep; §10 lifecycle |
| STATE-08 | Monotonic state_version | §1 BIGINT column + `state_version+1` on every write; §3 UPDATE SQL |
| STATE-09 | Login/logout transitions (SCHEMA-READY; fires deferred per D-82) | §1 columns reserved; no v0.1 caller |
| STATE-10 | IsRoutable helper | §8 domain package; 4 unit tests |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Transition validation | API / Backend (state pkg) | — | Pure server-side logic; UI never enforces |
| `agent_states` persistence | Database / Storage | API (sqlc) | Single source of truth; cache is derivative |
| State cache | API (cache pkg) | Database (fallback) | Phase 3 D-49 invariant: cache is DTO over DB |
| WrapUp TTL firing | API (state.Handlers goroutine) | Database (idempotent UPDATE) | Single-replica owner; UPDATE makes multi-firing safe (D-81) |
| `IsRoutable` decision | API / Backend (domain pkg) | — | Pure function; consumers load break_reason.routable |
| Cross-org `break_reason_id` probe | API (state handler) | Database (sqlc query) | Phase 3 D-76 invariant — no DB-level FKs |
| `force` admin override | API (state handler) | — | Stub auth in v0.1; AUTH phase will tier-shift to RBAC layer |
| Initial state row creation | API (catalog.CreateAgent tx) | Database | Codex C4 atomic invariant |
| Spec contract | API (openapi.yaml) → codegen | Frontend (consumes types) | D-32 → field additions allowed |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `time` | 1.25 | `time.AfterFunc`, `time.NewTicker`, `time.Timer.Stop` for sweeper | Built-in; D-81 names it directly [CITED: pkg.go.dev/time] |
| Go stdlib `sync` | 1.25 | `sync.RWMutex` guarding AfterFunc map; `sync.WaitGroup` for Stop() drain | Built-in; standard concurrency primitives |
| Go stdlib `math/rand` | 1.25 | ±100ms jitter (D-87) for `wrapup_until` | Built-in; jitter generation [CITED: pkg.go.dev/math/rand] |
| Go stdlib `context` | 1.25 | Sweeper `ctx.Done()` shutdown; `WithCancel` for internal ctx | Built-in |
| `github.com/google/uuid` v1.6+ | already in go.mod | UUIDv7 keys for AfterFunc map; agent_id parsing | Phase 1 D-19 [VERIFIED: services/api/go.mod] |
| `github.com/jackc/pgx/v5` | already in go.mod | sqlc DBTX backend; OrgTx wrapping | Phase 1 D-22 [VERIFIED: services/api/go.mod] |
| `github.com/jackc/pgx/v5/pgtype` | already in go.mod | pgtype.UUID / Timestamptz for nullable columns | Phase 3 carry-forward [VERIFIED: services/api/internal/catalog/mappers.go] |
| sqlc-generated `generated.New(DBTX)` | already in repo | `qtx.InsertAgentState`, `qtx.GetAgentState`, `qtx.UpdateAgentStateStatus`, etc. | Phase 3 D-62 pattern [VERIFIED: services/api/internal/db/generated/] |
| `github.com/luongdev/open-routing/services/api/internal/cache` | already in repo | `cache.GetOrSet[api.AgentState]` for GET; `cache.Del` after PATCH | Phase 3 D-49..D-60 [VERIFIED: services/api/internal/cache/cache.go] |
| `github.com/luongdev/open-routing/services/api/internal/db` | already in repo | OrgDB + OrgTx + SQLChecker; extend `tenantTables` map | Phase 1 D-25 [VERIFIED: services/api/internal/db/orgdb.go] |
| `log/slog` (stdlib) | 1.25 | `slog.WarnContext` for force=true audit (D-84); `slog.DebugContext` for sweeper observability | Phase 1 D-29 [CITED: pkg.go.dev/log/slog] |

### Supporting (new dependency)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/jonboulle/clockwork` | v0.4.0+ | Injectable Clock interface; `clockwork.NewFakeClock()` for deterministic AfterFunc tests | Required for D-81 testability of TTL goroutine [CITED: github.com/jonboulle/clockwork] |

**Verification (slopcheck unavailable in research env — see Package Legitimacy Audit; treat as `[ASSUMED]` until plan-time `checkpoint:human-verify` runs):**
- `npm view` N/A (Go module). `go mod download github.com/jonboulle/clockwork@v0.4.0` is the Go-equivalent verification.
- Origin: github.com/jonboulle/clockwork — active since 2014, MIT-licensed, used by Kubernetes, etcd, CockroachDB. AfterFunc support landed in v0.3 [CITED: github.com/jonboulle/clockwork README].
- ⚠️ Planner MUST add a `checkpoint:human-verify` task before `go get github.com/jonboulle/clockwork@v0.4.0` to confirm the version and current README, and slopcheck this name on demand.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `jonboulle/clockwork` | `benbjohnson/clock` | Equally popular but archived since 2023; clockwork is the maintained successor (clockwork's README explicitly cites benbjohnson as inspiration) [CITED: github.com/benbjohnson/clock readme]. |
| `jonboulle/clockwork` | `coder/quartz` | Quartz offers stricter monotonic-time semantics; younger (2024); fewer field references. clockwork is the conservative pick consistent with rest-of-stack maturity [CITED: coder.com/blog/introducing-quartz]. |
| `jonboulle/clockwork` | plain `func() time.Time` | Insufficient — D-81 requires `AfterFunc(d, fn) -> Timer` to be mockable, not just `Now()`. A bare clock-func can't drive timer firing in tests. |
| Postgres ENUM for status | `TEXT + CHECK` | Phase 3 already standardized on TEXT + CHECK (channel_type, adapter_type). ALTER on CHECK avoids the ALTER TYPE lock on PG14- (D-79). |
| `map[uuid.UUID]*time.Timer` + RWMutex | `sync.Map` | sync.Map is harder to iterate atomically at Stop(); mutex is preferred for explicit lock scope (CONTEXT.md Claude's Discretion). |
| XState-Go / fsm libraries | Plain `map[AgentStatus]map[AgentStatus]TransitionRule` | PROJECT.md locked: "No XState in v0.1; pure Go transition table." Matrix is ~9 entries — a library would be heavier than the data. |
| sqlc `:execrows` for UPDATE | sqlc `:one` with `RETURNING` | Need the returned row to populate the response DTO without a follow-up SELECT (consistent with catalog UpdateX). |

**Installation:** No new Go modules beyond clockwork. CLI:

```bash
cd services/api && go get github.com/jonboulle/clockwork@v0.4.0
```

**Version verification (planner runs at task time):**

```bash
go mod download github.com/jonboulle/clockwork@v0.4.0
go mod why github.com/jonboulle/clockwork
```

## Package Legitimacy Audit

> **Status:** Best-effort. Slopcheck not available in this research environment; treat all packages below as `[ASSUMED]` until the planner adds `checkpoint:human-verify` before install.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| github.com/jonboulle/clockwork | Go (pkg.go.dev) | 11 yrs (since 2014) | high — used by k8s, etcd, cockroach | github.com/jonboulle/clockwork | not run | **[ASSUMED]** — planner must add `checkpoint:human-verify` |

**Packages removed due to slopcheck [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none (best-effort assessment).

*If slopcheck remains unavailable at plan time, the planner MUST gate `go get github.com/jonboulle/clockwork` behind a `checkpoint:human-verify` task that runs `go mod why` and confirms the README against pkg.go.dev.*

## Architecture Patterns

### System Architecture Diagram

```
HTTP request                                    Internal goroutine
     │                                                  │
     ▼                                                  ▼
chi router (server.NewMux)                ┌── 30s safety-sweep ticker ──┐
     │                                    │                              │
     ▼                                    │   SELECT ... WHERE wrapup    │
StrictMiddleware chain:                   │   _until < NOW() AND         │
  Recoverer → RequestID                   │   status='WrapUp'            │
  → OrgContext (X-Org-Id)                 │            │                 │
  → UUIDv7PathParams                      │            ▼                 │
  → RequestIDInjection                    │   for each row:              │
     │                                    │     idempotent UPDATE        │
     ▼                                    │            │                 │
api.NewStrictHandler                      │            ▼                 │
     │                                    │     cache.Del(state key)     │
     ▼                                    └──────────────────────────────┘
ApiHandlers (D-89 composite)                              ▲
   ├── *catalog.Handlers                                  │
   │     └── CreateAgent (extended: INSERT agent_states)  │
   │         └─[ tx ]──┬─ qtx.InsertAgent                 │
   │                   ├─ qtx.InsertAgentState (Offline)  │
   │                   └─ qtx.replaceAgentSkills          │
   │                                                      │
   └── *state.Handlers                                    │
         ├── GetAgentStatus ─── cache.GetOrSet ─── q.GetAgentState
         └── PatchAgentStatus ──┐
                                ▼
                    ┌──── transition matrix ─────┐
                    │  if force=false:           │
                    │    validateTransition()    │
                    │      → 409 InvalidTransition│
                    │  always:                   │
                    │    if to==Break:           │
                    │      probe break_reason    │
                    │      → 422 invalid_reference│
                    │  qtx.UpdateAgentStateStatus│
                    │    (no version check)      │
                    │  0 rows → SELECT + 409 mat │
                    │  1 row  → cache.Del → 200  │
                    └────────────────────────────┘
                              │
                              ▼ (only when system fires Engaged→WrapUp in v0.2)
                    scheduleWrapUpExpiry(agent_id, wrapup_until)
                              │
                              ▼
                    clock.AfterFunc(remaining, expire)
                              │
                              ▼  on fire
                    same idempotent UPDATE + cache.Del
```

### Component Responsibilities

| File | Role | Key invariants |
|------|------|----------------|
| `migrations/000002_catalog_v0_1.up.sql` (APPENDED) | `agent_states` schema + indexes | TEXT+CHECK; NO FK; partial index on `wrapup_until` |
| `migrations/000002_catalog_v0_1.down.sql` (APPENDED at top) | `DROP TABLE IF EXISTS agent_states;` | Down migration prepended (drops in reverse order) |
| `services/api/internal/db/queries/agent_states.sql` | sqlc DML | InsertAgentState, GetAgentState, UpdateAgentStateStatus, ListWrapUpsToExpire, ExpireWrapUp |
| `services/api/internal/db/sqlcheck.go` (EDITED) | `tenantTables["agent_states"] = struct{}{}` | SQLChecker allowlist extension |
| `services/api/internal/domain/state.go` (NEW) | `IsRoutable` pure func + `AgentStateInputs` | NO DB access, NO imports of api/db/cache |
| `services/api/internal/domain/state_test.go` (NEW) | 4 unit tests for matrix | No fixtures; pure table-driven |
| `services/api/internal/state/handlers.go` (NEW) | `Deps`, `New`, `Start`, `Stop`, `Option` (WithClock, WithSweepInterval) | Mirrors `catalog.handlers.go` shape per D-71 |
| `services/api/internal/state/transitions.go` (NEW) | `AgentStatus` typed constants; matrix; `validateTransition()` | Pure data; no I/O |
| `services/api/internal/state/ttl.go` (NEW) | `scheduleWrapUpExpiry`, sweeper goroutine, startup sweep, AfterFunc map + RWMutex | Idempotent UPDATE; Stop drains; AfterFunc keyed by agent_id |
| `services/api/internal/state/agent_states.go` (NEW) | StrictServer impl: GetAgentStatus, PatchAgentStatus | Cache + tx + probes; force=true branch |
| `services/api/internal/state/agent_states_test.go` (NEW) | per-entity tests | Mirrors catalog/agents_test.go |
| `services/api/internal/state/testutil_test.go` (NEW) | per-test fixture builder | `_test.go` suffix keeps miniredis out of prod builds (A7) |
| `services/api/internal/catalog/agents.go` (EDITED) | Extend `CreateAgent` to INSERT agent_states inside existing tx | Codex C4 atomicity preserved |
| `services/api/internal/catalog/notimpl.go` (EDITED) | DELETE `GetAgentStatus` + `PatchAgentStatus` stubs | state.Handlers owns them now via embedding |
| `services/api/internal/db/queries/break_reasons.sql` (EDITED) | Add `BreakReasonExistsInOrg` + `GetBreakReasonRoutable` probes | D-76 pattern reuse |
| `services/api/cmd/api/main.go` (EDITED) | Declare `ApiHandlers`; construct `state.New(...)`; wire composite; call `Start(ctx)` + `defer Stop()` | D-89 activation |
| `services/api/internal/server/server.go` (EDITED) | Add cases for any new error response types in `injectRequestIDIntoErrorResponse` type switch | Phase 2 exhaustiveness invariant |
| `services/api/test/isolation/state_test.go` (NEW) | 4 cross-org probes (D-94) | freshOrg + reuse postEntity/getEntityStatus helpers |
| `openapi/openapi.yaml` (EDITED) | Add `force: { type: boolean, default: false }` to `PatchAgentStatusRequest.properties` | D-32 PATHS frozen; field additions allowed |
| `web/packages/ui/src/api/generated.ts` (REGENERATED) | Adds `force?: boolean` to TS body type | No hand-edits |

### Recommended Project Structure

```
services/api/internal/
├── catalog/                  # Phase 3 (one .go per entity)
│   ├── agents.go             # EDITED — CreateAgent tx extension
│   └── notimpl.go            # EDITED — remove 2 state stubs
├── state/                    # NEW Phase 4 (one focused package)
│   ├── handlers.go           # Deps + New(deps, opts...) + Start/Stop
│   ├── transitions.go        # AgentStatus consts + matrix + validator
│   ├── ttl.go                # AfterFunc map + sweeper + startup
│   ├── agent_states.go       # GetAgentStatus + PatchAgentStatus
│   ├── agent_states_test.go
│   └── testutil_test.go
├── domain/                   # NEW Phase 4 (pure functions only)
│   ├── state.go              # IsRoutable + AgentStateInputs
│   └── state_test.go
├── db/queries/
│   ├── agent_states.sql      # NEW
│   └── break_reasons.sql     # EDITED — add probes
├── db/sqlcheck.go            # EDITED — tenantTables += agent_states
└── ...

services/api/cmd/api/main.go  # EDITED — D-89 composite + sweeper lifecycle
services/api/test/isolation/state_test.go  # NEW — 4 cross-org probes
migrations/000002_catalog_v0_1.up.sql       # APPENDED
migrations/000002_catalog_v0_1.down.sql     # PREPENDED drop
openapi/openapi.yaml                        # EDITED — force field
```

### Pattern 1: Transition Matrix (D-91)

**What:** Encode the allowed transitions and per-edge requirements as static Go data, validated at request time.

**When to use:** Any time a state-machine rule must be enforced server-side and the rule set is small enough to fit in memory.

**Example:**
```go
// services/api/internal/state/transitions.go
// Source: D-91 + STATE-02 + STATE-04 + STATE-05.

package state

import (
	"errors"
	"fmt"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// AgentStatus is the typed enum mirroring api.AgentStatus values.
// Kept inside the state package so transitions.go is the single
// source of truth for the matrix; api.AgentStatus stays the wire type.
type AgentStatus = api.AgentStatus

// TransitionRule captures the per-edge requirements:
//   - RequiresBreakReasonID  → caller MUST supply break_reason_id (STATE-04)
//   - RequiresEngagedChannel → caller MUST supply engaged_channel (STATE-05)
//   - AgentInitiated         → true for v0.1; false reserved for v0.2 runtime
type TransitionRule struct {
	RequiresBreakReasonID  bool
	RequiresEngagedChannel bool
	AgentInitiated         bool
}

var matrix = map[AgentStatus]map[AgentStatus]TransitionRule{
	api.AgentStatusNotReady: {
		api.AgentStatusReady: {AgentInitiated: true},
	},
	api.AgentStatusReady: {
		api.AgentStatusNotReady: {AgentInitiated: true},
		api.AgentStatusBreak:    {RequiresBreakReasonID: true, AgentInitiated: true},
		// Ready→Engaged is system-initiated (deferred to v0.2).
	},
	api.AgentStatusBreak: {
		api.AgentStatusReady:    {AgentInitiated: true},
		api.AgentStatusNotReady: {AgentInitiated: true},
	},
	api.AgentStatusWrapUp: {
		api.AgentStatusReady:    {AgentInitiated: true},
		api.AgentStatusNotReady: {AgentInitiated: true},
	},
	// Engaged → WrapUp is system-initiated only (v0.2).
	// * → Offline + Offline → NotReady deferred to AUTH phase.
}

// ErrInvalidTransition wraps the 409 path. PatchAgentStatus translates
// this into PatchAgentStatus409JSONResponse with from/to fields.
var ErrInvalidTransition = errors.New("state: invalid transition")

// validateTransition returns the rule on success, ErrInvalidTransition
// when the (from, to) pair is not in the matrix. force=true bypasses
// the matrix entirely (D-84) — caller is responsible for that branch.
func validateTransition(from, to AgentStatus) (TransitionRule, error) {
	next, ok := matrix[from]
	if !ok {
		return TransitionRule{}, fmt.Errorf("%w: from=%s", ErrInvalidTransition, from)
	}
	rule, ok := next[to]
	if !ok {
		return TransitionRule{}, fmt.Errorf("%w: from=%s to=%s", ErrInvalidTransition, from, to)
	}
	return rule, nil
}
```

**Exhaustive test:** Generate Cartesian product of all 6 statuses × all 6 targets (36 pairs). Assert each pair matches `matrix[from][to]` presence.

### Pattern 2: WrapUp TTL with `time.AfterFunc` + sweep + Clock injection (D-81)

**What:** Per-agent timer registers a callback that fires the idempotent UPDATE. Concurrent firings (timer + sweep, or multi-replica in v1) compete safely — only one wins per row.

**Example:**
```go
// services/api/internal/state/ttl.go
// Source: D-81 + D-87 + D-95.

package state

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
)

// timersRegistry holds per-agent AfterFunc handles. RWMutex preferred
// over sync.Map because Stop() iterates the whole map to cancel each
// timer; sync.Map's Range is harder to reason about under cancel.
type timersRegistry struct {
	mu  sync.RWMutex
	t   map[uuid.UUID]clockwork.Timer // injectable Timer abstraction
}

// scheduleWrapUpExpiry registers (or replaces) a timer for agent_id.
// The fn closure carries the idempotent UPDATE — concurrent firings
// (timer + safety-sweep + multi-replica in v1) race the UPDATE WHERE
// status='WrapUp' AND wrapup_until < NOW(); only the first wins.
func (h *Handlers) scheduleWrapUpExpiry(agentID uuid.UUID, until time.Time) {
	remaining := until.Sub(h.clock.Now())
	if remaining < 0 {
		remaining = 0 // sweep will handle past-due immediately on Start
	}
	h.timers.mu.Lock()
	defer h.timers.mu.Unlock()
	// Replace any prior timer (e.g., extension scenario in v0.2).
	if prior, ok := h.timers.t[agentID]; ok {
		prior.Stop()
	}
	h.timers.t[agentID] = h.clock.AfterFunc(remaining, func() {
		// Use a derived ctx with bounded lifetime so a hung DB call
		// can never leak the goroutine indefinitely.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		h.expireWrapUp(ctx, agentID)
	})
}

// expireWrapUp executes the idempotent UPDATE. Safe to call from
// timer, safety sweep, and startup sweep. cache.Del on success.
func (h *Handlers) expireWrapUp(ctx context.Context, agentID uuid.UUID) {
	// Bypass org context: this path runs from the sweeper goroutine, not
	// a request. We pass org_id via the SQL row predicate (UPDATE WHERE
	// agent_id=$1 AND status='WrapUp'); the UPDATE's WHERE clause already
	// scopes by agent_id which is org-unique.
	//
	// SQLChecker compliance: the UPDATE must still mention org_id —
	// agent_states.org_id is the denormalized column for exactly this
	// case. The sweeper-issued query reads the row's org_id first via
	// ListWrapUpsToExpire, then issues UPDATE with that org_id explicit.
	rows, err := h.db.ExpireWrapUp(ctx, agentID)
	if err != nil {
		h.logger.WarnContext(ctx, "state.ttl.expire", "agent_id", agentID, "err", err)
		return
	}
	if rows == 0 {
		// Already expired by another firing or state changed — idempotent.
		h.logger.DebugContext(ctx, "state.ttl.expire.noop", "agent_id", agentID)
		return
	}
	// Cache invalidation — fire-and-forget warn on failure (D-55 spirit).
	_ = h.cache.Del(ctx, h.cacheKeyFor(agentID))
}
```

**Clock abstraction** (clockwork interface matches stdlib `time` 1:1):
- `Now() time.Time`
- `AfterFunc(d time.Duration, f func()) clockwork.Timer`
- Production: `clockwork.NewRealClock()`
- Tests: `clockwork.NewFakeClock()`; advance via `fakeClock.Advance(duration)`

### Pattern 3: Atomic UPDATE without version check (D-66 adapted for D-85)

**What:** UPDATE returns the new row; 0-row response is disambiguated by a follow-up SELECT — but unlike Phase 3's version-conflict path, Phase 4 uses transition-matrix mismatch as the 409 trigger.

**SQL:**
```sql
-- name: UpdateAgentStateStatus :one
-- WHERE clause:
--   id = $1 AND org_id = $2 — same-org isolation (FOUND-08)
--   AND status = $3         — "expected from" — drives 409 detection
--
-- A concurrent PATCH that already shifted the agent off `$3` produces
-- 0 rows. Handler runs GetAgentState to fetch the current row and
-- returns 409 invalid_transition with the OBSERVED from + REQUESTED to.
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
```

Handler flow:
1. Run validator (skip on `force=true`).
2. Run cross-row probe if `to=Break` (always, even on `force=true` — D-84).
3. Begin tx; `qtx.UpdateAgentStateStatus(...)` with `expected_from = currentObserved` OR — on force=true — fetch current first to populate `expected_from`.
4. If 0 rows: `qtx.GetAgentState(...)` → if absent → 404 `not_found`; if present → 409 `invalid_transition` with `{from: row.Status, to: req.To}`.
5. If 1 row: commit, `cache.Del`, return 200 `AgentState`.

**Distinction from Phase 3 D-66:** No version conflict — only transition validation. The matrix is the conflict authority.

### Pattern 4: `CreateAgent` extended atomicity (D-93)

**What:** The Codex C4 tx that wraps agent INSERT + skills replace gains a third statement: agent_states INSERT.

**Example diff (services/api/internal/catalog/agents.go):**
```go
// After the existing qtx.InsertAgent call in CreateAgent:
qtx := generated.New(tx)
row, err := qtx.InsertAgent(ctx, generated.InsertAgentParams{ /* ... */ })
if err != nil { /* existing branch */ }

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

// Skills replace inside the SAME tx (existing Codex C4) ...
```

A failure here triggers the existing `defer tx.Rollback`, rolling back the agent row too. Phase 5 (bulk import) MUST follow the same pattern — captured as a PATTERNS.md pitfall.

### Pattern 5: D-76 cross-row probe extension for `break_reason_id` (D-84 force-aware)

**What:** Two probes are required because `IsRoutable` (STATE-10) needs `routable` and STATE-04 needs only existence-in-org. Keep them separate for caller clarity — same as Phase 3 (`QueueExistsAndEnabledInOrg` vs `GetQueueByIdAnyVersion`).

**Recommendation:** One combined probe — `GetBreakReasonStatus(ctx, id, org_id) returns (routable bool, found bool)`. Two callers:
- STATE-04 PATCH path: `found==false → 422 invalid_reference`; `found==true` → proceed.
- STATE-10 IsRoutable path (Phase 4 doesn't ship the routing decision but ships the helper): caller passes `routable` into `IsRoutable(AgentStateInputs{Status, BreakReasonRoutable: routable})`.

**SQL:**
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

**Force=true does NOT bypass this probe** (D-84 explicit). The probe runs regardless; only the transition matrix is bypassed.

### Pattern 6: `state.Handlers` package scaffold (D-88 + D-95 lifecycle)

```go
// services/api/internal/state/handlers.go
// Source: D-71 mirror + D-88 + D-95.

package state

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db"
)

// Constants. Sweep interval and WrapUp duration live here for grep-ability.
const (
	defaultSweepInterval = 30 * time.Second // D-81
	defaultWrapUpDur     = 60 * time.Second // v0.1 hardcode — per-org config deferred
	stateCacheTTL        = 60 * time.Second // D-86
	jitterMaxMs          = 100              // ±100ms (D-87)
)

type Deps struct {
	OrgDB          *db.OrgDB
	Cache          *cache.Cache
	Logger         *slog.Logger
	WrapUpDuration time.Duration // optional override; falls back to defaultWrapUpDur
}

type Option func(*Handlers)

func WithClock(c clockwork.Clock) Option            { return func(h *Handlers) { h.clock = c } }
func WithSweepInterval(d time.Duration) Option      { return func(h *Handlers) { h.sweepInterval = d } }

type Handlers struct {
	deps          Deps
	clock         clockwork.Clock
	sweepInterval time.Duration

	timers *timersRegistry

	// shutdown plumbing
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	started bool
	startMu sync.Mutex
}

// Compile-time guarantee: *Handlers implements the StrictServerInterface
// SUBSET for state operations. The composite ApiHandlers struct in
// main.go covers the full StrictServerInterface via embedding (D-89).

func New(deps Deps, opts ...Option) *Handlers {
	h := &Handlers{
		deps:          deps,
		clock:         clockwork.NewRealClock(),
		sweepInterval: defaultSweepInterval,
		timers:        &timersRegistry{t: make(map[uuid.UUID]clockwork.Timer)},
	}
	if deps.WrapUpDuration == 0 {
		h.deps.WrapUpDuration = defaultWrapUpDur
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Start runs the startup sweep synchronously (D-95) — guarantees no
// stuck WrapUps after a restart — then spawns the 30s safety-sweep
// goroutine. Idempotent: re-entry no-ops.
func (h *Handlers) Start(ctx context.Context) error {
	h.startMu.Lock()
	defer h.startMu.Unlock()
	if h.started {
		return nil
	}
	h.ctx, h.cancel = context.WithCancel(context.Background())

	// Synchronous startup sweep — fire-immediate + schedule futures.
	if err := h.runStartupSweep(ctx); err != nil {
		return err
	}

	// Safety sweep goroutine.
	h.wg.Add(1)
	go h.safetySweepLoop()
	h.started = true
	return nil
}

// Stop cancels the internal ctx, waits for the safety-sweep goroutine
// to exit, then iterates the timer map cancelling each AfterFunc. Safe
// to call from `defer` in main.go.
func (h *Handlers) Stop() {
	h.startMu.Lock()
	defer h.startMu.Unlock()
	if !h.started {
		return
	}
	h.cancel()
	h.wg.Wait()

	h.timers.mu.Lock()
	for id, t := range h.timers.t {
		t.Stop()
		delete(h.timers.t, id)
	}
	h.timers.mu.Unlock()
	h.started = false
}

func (h *Handlers) cacheKeyFor(agentID uuid.UUID) string {
	// orgID is encoded via the row predicate; cache key follows D-58 +
	// D-86 with the agent_states scope. Caller passes orgID at PATCH /
	// GET time; the sweeper looks up via the SQL row.
	// For sweeper, we cache.Del using the row's org_id.
	return "" // implementation lives in agent_states.go — orgID-aware
}
```

### Pattern 7: `ApiHandlers` composite struct (D-89, D-70 activation)

**What:** Method-set merging via embedding. Go's method-set resolution surfaces `state.Handlers`'s `GetAgentStatus` + `PatchAgentStatus` and `catalog.Handlers`'s 39 other methods through one value passed to `api.NewStrictHandler`.

```go
// services/api/cmd/api/main.go (or a small composite.go beside it):
type ApiHandlers struct {
	*catalog.Handlers
	*state.Handlers
}

// Compile-time guarantee: if both embedded types tried to satisfy the
// same method (none expected — disjoint endpoint sets), Go would reject.
var _ api.StrictServerInterface = (*ApiHandlers)(nil)
```

Wiring:
```go
catalogHandlers := catalog.New(catalog.Deps{ /* ... */ })
stateHandlers   := state.New(state.Deps{
	OrgDB: orgDB, Cache: catalogCache, Logger: slog.Default(),
})
api := &ApiHandlers{Handlers: catalogHandlers, /* state.Handlers */}
// Note: anonymous embed conflict — both embeds named `Handlers`. Use
// explicit field names:

type ApiHandlers struct {
	Catalog *catalog.Handlers
	State   *state.Handlers
}
// ...then forward methods explicitly? NO — anon embed works if we
// rename one type. SIMPLER pattern: name the embedded types via
// type aliases at use-site, OR just rename state.Handlers' method
// receivers to delegate from a wrapper. Cleanest: embed by type
// (Go picks the type name as field name) and call NewStrictHandler
// on the address:

type ApiHandlers struct {
	*catalog.Handlers // field accessible as a.Handlers — COLLIDES
	*state.Handlers   // also "Handlers" — Go REJECTS at compile time
}
```

⚠️ **PITFALL (research hazard):** `*catalog.Handlers` and `*state.Handlers` are BOTH called `Handlers` — the anonymous-embed field names collide. The compiler emits "duplicate field name." Options to resolve, ranked:

1. **Rename one struct.** `state.Handlers` → `state.StateHandlers` (or `catalog.Handlers` → `catalog.CatalogHandlers`). Pro: simple. Con: cosmetic churn across Phase 3 codebase.
2. **Manually forward the two methods.** `ApiHandlers` is a non-embedding struct that holds `Catalog *catalog.Handlers` and `State *state.Handlers`, and implements every `StrictServerInterface` method by delegation. Pro: zero risk of accidental method shadow. Con: 41 boilerplate forwarding methods.
3. **Use named embed via wrapper types.** Declare `type stateHandlers = *state.Handlers` aliased so the embedded field name differs. Go's "field name" for an alias-embedded type follows the alias unqualified name — so we can do `type stateImpl struct{ *state.Handlers }` then embed `stateImpl`. Pro: tiny shim. Con: indirection adds a layer.

**Recommendation (planner choice):** Option 1 — rename `state.Handlers` to `state.Handlers` in its own package but expose a top-level `type Handlers = ...` … no, simpler: just **keep both named `Handlers`**, and embed via `composition` (option 2 manual forwarding) **OR** rename `state.Handlers` to `state.Server`.

The CLEANEST is **option 1 with rename**: `catalog.Handlers` stays (39 methods, more mass), `state.Server` is the new type. Mention in PATTERNS.md as a Phase 4 hazard captured during research — saves at least one cross-AI review cycle.

```go
// Recommended final shape:
package state
type Server struct { /* ... */ }  // rename Handlers → Server
func New(deps Deps, opts ...Option) *Server { /* ... */ }

// main.go:
type ApiHandlers struct {
	*catalog.Handlers
	*state.Server
}
var _ api.StrictServerInterface = (*ApiHandlers)(nil)
```

Alternative: keep the name `Handlers` in `state` package and use ALIAS pattern. Either way, the planner MUST verify with `go build ./...` before claiming the embedding works. Cross-AI review (Codex/Gemini) will catch this in plan check if missed.

### Pattern 8: Cache integration (D-86)

```go
// services/api/internal/state/agent_states.go

func (h *Handlers) GetAgentStatus(ctx context.Context, req api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetAgentStatus500JSONResponse{ /* missing_org_id_in_context */ }, nil
	}
	agentID := uuid.UUID(req.Id)
	key := cache.Key(orgID, "agent_state", agentID) // D-86 key

	state, err := cache.GetOrSet[api.AgentState](ctx, h.deps.Cache, key, stateCacheTTL,
		func(ctx context.Context) (api.AgentState, error) {
			q := generated.New(h.deps.OrgDB)
			row, ferr := q.GetAgentState(ctx, generated.GetAgentStateParams{
				AgentID: pgUUID(agentID),
				OrgID:   pgUUID(orgID),
			})
			if errors.Is(ferr, pgx.ErrNoRows) {
				return api.AgentState{}, cache.ErrNotFound
			}
			if ferr != nil {
				return api.AgentState{}, ferr
			}
			return mapAgentState(row), nil
		})
	switch {
	case errors.Is(err, cache.ErrNotFound):
		return api.GetAgentStatus404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "agent_state_not_found",
		}}, nil
	case err != nil:
		h.deps.Logger.ErrorContext(ctx, "get agent status", "err", err)
		return api.GetAgentStatus500JSONResponse{ /* internal */ }, nil
	}
	return api.GetAgentStatus200JSONResponse(state), nil
}
```

PatchAgentStatus invalidates via `h.deps.Cache.Del(ctx, key)` AFTER commit (D-55).

### Anti-Patterns to Avoid

- **Polling-style WrapUp expiry without AfterFunc.** User explicitly rejected the 2s sweep proposal ("Goal là hiệu suất cao mà mày để tận 2s..."). Sweeper is the SAFETY NET; AfterFunc is the PRIMARY firing path.
- **`context.Background()` in TTL goroutine.** Strips trace_id + org_id. Use `context.WithoutCancel(ctx)` if propagating from a request (e.g., scheduleWrapUpExpiry called from PatchAgentStatus); use `context.WithTimeout(context.Background(), 10s)` for sweep-triggered firings since there's no parent request ctx.
- **Returning 500 on cache.Del failure in PatchAgentStatus.** D-55 spirit: cache failures NEVER turn successful DB writes into 5xx.
- **State INSERT outside the agent INSERT tx (D-93).** Two separate txs = two failure modes; rollback semantics diverge. ALWAYS inside the existing Codex C4 tx.
- **Hand-rolled state-machine library.** PROJECT.md locked: pure Go transition table. The matrix IS the state machine.
- **Caching a `nil` state row.** D-54 invariant — loader returns `cache.ErrNotFound`; helper propagates without writing.
- **Adding a FK constraint to agent_states.agent_id.** D-80 prohibits this. Agent deletion in v0.1 is soft-delete (enabled=FALSE); the state row persists by design (re-enable returns the agent's last state).
- **Letting `state_version` regress.** Every write does `state_version + 1`. Sweeper UPDATE also increments. UI relies on monotonic STATE-08.
- **Allowing `force=true` to bypass break_reason_id probe.** D-84 explicit: probes always run. Only the transition matrix is bypassed.
- **Per-replica TTL sweeper without the idempotent UPDATE.** v1 multi-replica deferral relies on the WHERE-clause guard. If anyone weakens that clause for "performance," concurrent firings can double-decrement (or just confuse the audit trail).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Mock-able time for AfterFunc tests | Custom `Clock interface` + custom fake | `github.com/jonboulle/clockwork` | Battle-tested fake supports AfterFunc with Advance() + drains; custom implementations leak goroutines and race-detect failures (multiple field reports per Coder Quartz blog) |
| Cache + DTO + singleflight | Re-implement read-through | `cache.GetOrSet[api.AgentState]` (already in repo) | D-49..D-60 already enshrines this — every catalog entity uses it; deviating creates a maintenance fork |
| Transition validation | Library / DSL | Plain `map[from]map[to]Rule` Go data | PROJECT.md hard lock; 9-edge matrix is smaller than any library |
| SQL state assertion | App-layer `state.Version != cached.Version` check | Postgres `WHERE status = $expected_from` predicate | Push the race-free check into the DB; 0-row = mismatch, no separate version compare |
| Cross-org isolation | Repeat orgID-in-WHERE assertions in every test | SQLChecker + `tenantTables` allowlist | Phase 1 D-25 invariant — add `agent_states` to the allowlist; SQLChecker validates every emitted SQL string |
| Spec-then-generated-code drift | Manual edits to `*.gen.go` | `task gen` + codegen-drift CI gate | D-47/D-48 already in CI; any hand-edit reverted at the next regeneration |
| In-flight goroutine drain | `time.Sleep(5s)` at shutdown | `sync.WaitGroup` + `context.CancelFunc` | Phase 1 graceful-shutdown pattern; deterministic |
| AfterFunc map operations | `sync.Map` or lock-free | `map[uuid.UUID]Timer` + `sync.RWMutex` | Stop() needs deterministic iteration to cancel all timers; sync.Map's Range visibility is weakly defined under writes |

**Key insight:** Phase 4's value is in correct composition of Phase 3 primitives, not in new tooling. Any new module spent here is a smell.

## Runtime State Inventory

> N/A — Phase 4 is greenfield code, not a rename/refactor/migration. The single editable migration `000002_catalog_v0_1.up.sql` is the only state-bearing artifact, and it's only appended to. No existing data needs migration.

## Common Pitfalls

### Pitfall 1: `*catalog.Handlers` and `*state.Handlers` collide on embed
**What goes wrong:** `type ApiHandlers struct { *catalog.Handlers; *state.Handlers }` fails to compile with "duplicate field name Handlers". Plan-checker (or worse, dev's local build) catches it; reviewers don't.
**Why it happens:** Anonymous embedding uses the type's unqualified name as field name.
**How to avoid:** Rename `state.Handlers` → `state.Server` (recommended) before publishing the package. Alternative: manual forwarding (41 methods).
**Warning signs:** Plan iter 0 build fails with "duplicate field"; reviewer says "just rename."

### Pitfall 2: Sweeper goroutine outlives `Stop()` because timer Stop() races AfterFunc body
**What goes wrong:** `timer.Stop()` returns true only if the AfterFunc has NOT yet started firing. If the goroutine is mid-flight when Stop is called, the AfterFunc body keeps running and may emit an UPDATE after the pool is closed.
**Why it happens:** `time.Timer.Stop()` doc: "Stop does not close the channel, to prevent a read succeeding incorrectly... Stop returns false if [the func has] already expired."
**How to avoid:** (a) inside the AfterFunc body, check `select { case <-h.ctx.Done(): return; default: }` BEFORE the DB call; (b) clockwork's mocked Timer respects the same semantic so tests can assert clean shutdown.
**Warning signs:** Test failure: "pool closed" on shutdown; race-detect output in `go test -race`.

### Pitfall 3: `force=true` bypasses break_reason probe (BUG)
**What goes wrong:** Engineer reads "force bypasses validation" and skips ALL validation including the cross-org break_reason probe. Result: org admin can poke a Break state with an org B's break_reason_id, leaking cross-org existence.
**Why it happens:** D-84 contract is subtle — matrix bypass != probe bypass.
**How to avoid:** Explicit isolation test (D-94 (c)) asserts `force=true + cross-org break_reason_id → 422`. Plan task body must call this out; cross-AI review re-validates.
**Warning signs:** Isolation test fails; reviewer comment "force=true must NOT bypass probes."

### Pitfall 4: `tenantTables` allowlist forgotten — SQLChecker panic on every agent_states query
**What goes wrong:** First sqlc-generated query against `agent_states` panics in dev/test because SQLChecker's allowlist doesn't include the table.
**Why it happens:** Phase 1 D-25 allowlist is opt-in; Wave 0 doesn't include the table addition explicitly.
**How to avoid:** Wave 0 task description must list `tenantTables["agent_states"] = struct{}{}` as a step adjacent to the migration append.
**Warning signs:** First `go test ./internal/state/...` panics with `ErrSQLMissingOrgFilter`.

### Pitfall 5: Forgetting to extend `injectRequestIDIntoErrorResponse` type switch
**What goes wrong:** New error response types from the spec amendment (force-related responses, if any new wrappers appear) silently drop request_id. The exhaustiveness test `request_id_exhaustiveness_test.go` catches this in CI.
**Why it happens:** Phase 2 invariant — adding a new `*JSONResponse` wrapper requires updating the type switch.
**How to avoid:** Wave 4 task that runs `task gen` MUST diff `injectRequestIDIntoErrorResponse` against the new spec.gen.go enum; add any missing case BEFORE running the exhaustiveness test.
**Warning signs:** `TestRequestIDInjection_Exhaustiveness` fails on a 122+th subtest.

### Pitfall 6: `agent_states` row missing for an existing agent (Phase 5 will break)
**What goes wrong:** Phase 5 bulk import creates an agent row via `INSERT ... ON CONFLICT (org_id, external_id) DO UPDATE` BUT skips the agent_states INSERT. Subsequent GET /agents/{id}/status returns 404 even though the agent exists.
**Why it happens:** D-93 atomicity is enforced in `CreateAgent`, not in the schema. Future code paths can bypass it.
**How to avoid:** Capture in PATTERNS.md (Wave 0 task). Phase 5 plan must include the agent_states INSERT in the import upsert path.
**Warning signs:** Phase 5 integration test fails on first GET /agents/{id}/status after bulk import.

### Pitfall 7: Cache invalidation forgot the new key prefix
**What goes wrong:** Tests for PatchAgentStatus pass but the agent_state cache key is `agents` (the catalog entity slug) not `agent_state`. D-86 says `or:{orgId}:agent_state:{agent_id}` — agent + agent_state are distinct namespaces. Mixing them invalidates the wrong key.
**Why it happens:** Copy-paste from catalog/agents.go's GetAgent (uses `cache.Key(orgID, "agents", agentID)`).
**How to avoid:** Define a single package-local helper `cacheKeyFor(orgID, agentID) = cache.Key(orgID, "agent_state", agentID)` and route all cache calls through it.
**Warning signs:** Test "patch updates state, GET returns fresh row" fails — GET returns the cached pre-patch row.

### Pitfall 8: WrapUp TTL goroutine context strips org_id, then panics SQLChecker
**What goes wrong:** Sweeper-triggered `expireWrapUp` calls SQL without org_id in ctx. SQLChecker in ValidationPanic mode crashes the process.
**Why it happens:** Phase 1 D-03 invariant — every DB call's ctx must carry org_id. The sweeper has no request ctx.
**How to avoid:** The sweeper SQL queries explicitly include org_id in the predicate AND the sweeper uses `OrgDB.WithBypass(ctx, "wrapup_sweeper")` (Phase 1 D-04) for that path, with a structured slog event matching the existing audit shape.
**Warning signs:** Process panic at startup with `orgdb: ctx missing org_id`.

### Pitfall 9: `time.AfterFunc` clock injection breaks at boot when the test fake clock isn't advanced
**What goes wrong:** Test uses `clockwork.NewFakeClock()`, the production handler calls `clock.AfterFunc(d, f)` — and waits forever because `f` only fires when `fakeClock.Advance(d)` is called.
**Why it happens:** Fake clock doesn't tick automatically — test must explicitly advance time.
**How to avoid:** Test pattern: `h.scheduleWrapUpExpiry(agentID, fakeClock.Now().Add(60*time.Second))` → `fakeClock.Advance(61*time.Second)` → `require.Eventually(t, func() bool { /* assert UPDATE landed */ }, ...)`.
**Warning signs:** Test times out with "WrapUp expiry never fired."

### Pitfall 10: Initial state row INSERT uses raw orgDB instead of qtx
**What goes wrong:** Engineer adds `qtx := generated.New(h.deps.OrgDB)` (pool-level) for the new InsertAgentState call inside the existing tx, breaking atomicity — the state row commits independently.
**Why it happens:** Easy copy-paste error from non-tx handlers.
**How to avoid:** Code reviewer (and PATTERNS.md guidance) flags any `generated.New(h.deps.OrgDB)` inside a `tx, _ := h.deps.OrgDB.BeginTx(ctx)` block.
**Warning signs:** Cross-AI review (Codex) emits "C4 atomicity broken — state INSERT uses pool not tx."

## Code Examples

### Migration append (D-78)

```sql
-- migrations/000002_catalog_v0_1.up.sql
-- ── APPEND inside the existing BEGIN; ... COMMIT; block at the end ──

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

-- Index 1: sweeper-friendly org-scoped status lookup.
CREATE INDEX ix_agent_states_org_status ON agent_states (org_id, status);

-- Index 2: partial index for the WrapUp sweeper hot path. Sweeper SQL is
-- `WHERE wrapup_until < NOW() AND status='WrapUp'` — partial filter on
-- `status='WrapUp'` keeps the index narrow (only ~N agents currently in
-- WrapUp, not all rows).
CREATE INDEX ix_agent_states_wrapup_until
    ON agent_states (wrapup_until)
    WHERE status = 'WrapUp';
```

Corresponding down migration prepends:

```sql
-- migrations/000002_catalog_v0_1.down.sql (PREPEND inside BEGIN/COMMIT)
DROP TABLE IF EXISTS agent_states;
```

### sqlc queries (NEW agent_states.sql)

```sql
-- services/api/internal/db/queries/agent_states.sql
-- Phase 4 queries for the agent_states table (D-78, D-81, D-85, D-93).

-- name: InsertAgentState :one
-- Phase 4 D-93 — called from catalog.CreateAgent inside the existing tx
-- so the agent + state rows commit atomically. Initial status='Offline',
-- state_version=1, all nullable columns NULL.
INSERT INTO agent_states (agent_id, org_id, status, state_version)
VALUES ($1, $2, $3, 1)
RETURNING agent_id, org_id, status, engaged_channel, break_reason_id,
         post_interaction_state, wrapup_until, state_version, updated_at;

-- name: GetAgentState :one
-- Single-row lookup by (agent_id, org_id). pgx.ErrNoRows when the row
-- does not exist or belongs to another org (FOUND-08).
SELECT agent_id, org_id, status, engaged_channel, break_reason_id,
       post_interaction_state, wrapup_until, state_version, updated_at
FROM agent_states
WHERE agent_id = $1 AND org_id = $2;

-- name: UpdateAgentStateStatus :one
-- D-85 — no state_version in WHERE. Instead, status (expected_from)
-- gates the matrix. 0 rows → handler runs GetAgentState to choose 404
-- vs 409 invalid_transition.
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
-- D-84 force=true variant. Same as UpdateAgentStateStatus but WITHOUT
-- the status = expected_from gate. Cross-org isolation preserved via
-- agent_id + org_id WHERE clause (FOUND-08).
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

-- name: ListWrapUpsToExpire :many
-- D-81 startup sweep + 30s safety sweep. Returns past-due AND future
-- WrapUps so the boot path can fire-now-for-past-due AND re-schedule
-- AfterFunc timers for future expirations.
-- NB: this query is called from the sweeper goroutine which uses
-- OrgDB.WithBypass — the org_id mention here satisfies SQLChecker.
SELECT agent_id, org_id, wrapup_until, post_interaction_state
FROM agent_states
WHERE status = 'WrapUp'
ORDER BY org_id, wrapup_until;

-- name: ExpireWrapUp :execrows
-- D-81 idempotent UPDATE. Concurrent firings (timer + sweeper + future
-- multi-replica) compete on the WHERE clause; only one wins per row.
-- Returns rows affected so the caller can detect no-op (already expired
-- or status changed between SELECT and UPDATE).
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

### Domain pure function (D-90)

```go
// services/api/internal/domain/state.go
// Source: D-90 + STATE-10.

// Package domain hosts pure, DB-free, framework-free helper functions
// used by the routing layer. v0.1 ships only IsRoutable. Tests live in
// state_test.go with a 4-row table covering the entire matrix.
package domain

import "github.com/luongdev/open-routing/services/api/internal/api"

type AgentStateInputs struct {
	Status              string
	BreakReasonRoutable bool
}

// IsRoutable returns true only when the agent is eligible to receive
// new interactions per STATE-10. The caller is responsible for loading
// the break_reason.routable flag — domain has no DB access.
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

Tests (4 cases):
```go
// services/api/internal/domain/state_test.go
func TestIsRoutable(t *testing.T) {
	cases := []struct {
		name    string
		in      AgentStateInputs
		want    bool
	}{
		{"Ready→true", AgentStateInputs{Status: "Ready"}, true},
		{"Break+routable=true→true", AgentStateInputs{Status: "Break", BreakReasonRoutable: true}, true},
		{"Break+routable=false→false", AgentStateInputs{Status: "Break", BreakReasonRoutable: false}, false},
		{"NotReady/Engaged/WrapUp/Offline→false", AgentStateInputs{Status: "Engaged"}, false},
	}
	// ...table-driven assertions
}
```

### Spec amendment (D-92)

```yaml
# openapi/openapi.yaml — INSERT into PatchAgentStatusRequest.properties
# (existing schema at lines 1075-1121 per CONTEXT.md):
    PatchAgentStatusRequest:
      type: object
      description: |
        ... existing description ...
      required:
        - to
      properties:
        to:
          $ref: "#/components/schemas/AgentStatus"
          description: The target state for this transition.
        break_reason_id:
          allOf:
            - $ref: "#/components/schemas/UUIDv7"
          type: string
          nullable: true
          description: >
            Required when `to == Break` (STATE-04). Must reference a
            `break_reason` belonging to the same org. ...
        post_interaction_state:
          allOf:
            - $ref: "#/components/schemas/PostInteractionState"
          type: string
          nullable: true
          description: >
            May be set while `status == Engaged` (STATE-06). ...
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

Then `task gen` regenerates:
- `services/api/internal/api/types.gen.go` (`PatchAgentStatusJSONRequestBody.Force *bool`)
- `services/api/internal/api/server.gen.go` (unchanged signature; body adds Force)
- `services/api/internal/api/spec.gen.go` (embedded YAML bytes refresh)
- `web/packages/ui/src/api/generated.ts` (TS body gains `force?: boolean | null`)

### Cross-org isolation tests (D-94)

```go
// services/api/test/isolation/state_test.go
// Source: D-94 + Phase 3 catalog_test.go pattern.

package isolation_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// (a) GET /status across orgs returns 404.
func TestState_GetStatusCrossOrg(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)
	code, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
		"external_id": "s-iso-a", "name": "A", "email": "a@example.com",
	})
	require.Equal(t, http.StatusCreated, code)
	// GET status from A → 200; from B → 404.
	require.Equal(t, http.StatusOK, getEntityStatus(t, baseURL(), "agents/"+agentA.String()+"/status", orgA, uuid.Nil))
	require.Equal(t, http.StatusNotFound, getEntityStatus(t, baseURL(), "agents/"+agentA.String()+"/status", orgB, uuid.Nil))
}

// (b) PATCH /status with cross-org break_reason_id → 422.
// (c) force=true does NOT bypass the cross-org probe → still 422.
// (d) PATCH cross-org agent → 404.
// ... (analogous tests)
```

⚠️ Note: `getEntityStatus` helper takes (urlBase, entityPath, orgID, id) — for a `/agents/{id}/status` nested path, the planner may want a new helper `getStatusEndpoint(t, urlBase, orgID, agentID)` that builds `/v1/orgs/{orgID}/agents/{agentID}/status`. Phase 3 doesn't have this — Wave 5 task adds it.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Sleep-poll for TTL expiry | `time.AfterFunc` + per-key timer + sweeper safety net | Go 1.4+ (AfterFunc stable) | Sub-millisecond expiry vs poll-interval drift |
| Manual time mocks | `clockwork`/`quartz` Clock interface | 2022+ (clockwork 0.3 AfterFunc) | Deterministic AfterFunc tests; no `time.Sleep` in unit tests |
| Postgres ENUM for state | TEXT + CHECK | Phase 3 (D-79 lineage) | ALTER ENUM locks table on PG14-; ALTER CHECK does not |
| FK constraints across catalog tables | App-layer probes | Phase 3 (D-76, D-80) | Soft-delete + FK CASCADE semantics mismatch; app-layer probes handle the gap |
| `version` in WHERE for state | `status` in WHERE (matrix-based) | Phase 4 (D-85) | Matrix is the conflict authority; matches OpenAPI spec (no version field on PATCH body) |
| Composite server via switch | Struct embedding (D-70) | Phase 4 activates D-89 | Compile-time method-set merging; one StrictHandler value |

**Deprecated/outdated:**
- `benbjohnson/clock` — Archived since 2023; `jonboulle/clockwork` is the maintained successor. No reason to choose archived library when actively maintained one with the same API is available.
- Polling-style TTL sweepers without idempotent UPDATE — Multi-replica unsafe; v1 will require this anyway.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `github.com/jonboulle/clockwork v0.4.0+` is current and supports AfterFunc with FakeClock | Standard Stack / Pitfall 9 | Medium — alternative is `coder/quartz` or hand-rolled Clock interface. Planner verifies via `go mod download` before install. |
| A2 | Composite struct embedding of `*catalog.Handlers` + `*state.Handlers` fails compilation due to duplicate field name `Handlers` | Pattern 7 | Medium — Codex/Gemini review will catch. Mitigation: rename `state.Handlers` → `state.Server` (Pitfall 1). |
| A3 | The `force=true` codegen-drift change adds only a `Force *bool` field to `PatchAgentStatusJSONRequestBody` without altering any existing response type | Spec amendment / Pitfall 5 | Low — verified by re-running `task gen` locally; reviewer can confirm in CI. |
| A4 | Sweeper goroutine using `OrgDB.WithBypass(ctx, "wrapup_sweeper")` is the cleanest way to satisfy SQLChecker while still emitting the audit slog event | Pitfall 8 | Low — pattern is in `cmd/migrate` already (Phase 1 D-04). |
| A5 | `agent_states.org_id` denormalized column is sufficient for SQLChecker MustContainOrgFilter without further classifier changes | §1 schema / pitfall 4 | Low — Phase 3 D-72 set the precedent (`agent_skills` is also denormalized; SQLChecker accepts queries against it). Verified by reading sqlcheck.go. |
| A6 | The cache key namespace `agent_state` (singular, not `agent_states`) per D-86 is intentional | Pattern 8 / Pitfall 7 | Low — D-86 wording is explicit; planner should add a code comment quoting D-86. |
| A7 | No new error response type is introduced by the spec amendment (D-92) — `force` is a request-body field only | Pitfall 5 | Low — verified by reading openapi.yaml; planner re-confirms after running `task gen`. |
| A8 | Sub-1ms AfterFunc firing satisfies the user's "high performance" expectation — no need for a tighter scheduler | Specifics in CONTEXT.md | Low — Go's runtime timer wheel resolution is well under 1ms in normal operation. |

**If A1–A2 prove wrong:** A1 is a planner verification step before `go get`; A2 is a compile-fail recoverable via rename. Both are addressed by Wave 0 spike tasks rather than expensive late-stage rework.

## Open Questions

1. **Cache key for the sweeper-side cache.Del:**
   - What we know: D-86 mandates `or:{orgId}:agent_state:{agent_id}` keys.
   - What's unclear: The sweeper goroutine doesn't receive a request orgID — it must extract from the row data returned by `ListWrapUpsToExpire`. Implementation detail; not a blocker.
   - Recommendation: `ExpireWrapUp` returns the orgID alongside rowsAffected so the sweeper can construct the key without a second SELECT.

2. **Should `BreakReasonExistsInOrg` and `GetBreakReasonRoutable` be one query or two?**
   - What we know: Phase 3 D-76 used separate `QueueExistsAndEnabledInOrg` (1-row probe) and `GetQueueByIdAnyVersion` (full row).
   - What's unclear: For Phase 4, the STATE-04 probe only needs existence; the STATE-10 helper needs `routable`. One combined `GetBreakReasonForState (id, org) -> (routable bool, found bool)` covers both with one round-trip.
   - Recommendation: **One combined probe** (`GetBreakReasonForState`). The 422 path uses `found==false`; the IsRoutable path uses `routable`. Keeps the sqlc surface minimal.

3. **Where does `WrapUpDuration` constant live?**
   - What we know: D-83 hardcodes a single duration in v0.1; per-org config is v0.2.
   - What's unclear: `internal/state/config.go` standalone file vs constant near `defaultSweepInterval` in `handlers.go`.
   - Recommendation: Single-block of `const (...)` at top of `handlers.go` keeps related lifetime constants grouped — already shown in Pattern 6.

4. **Does the v0.1 WrapUp goroutine need to be started at all if no v0.1 code path INSERTs into WrapUp state?**
   - What we know: D-82 defers Engaged→WrapUp to v0.2. D-93 inserts `Offline` initial state. Tests will seed WrapUp directly via fixtures (D-88 + Claude discretion bullet 6).
   - What's unclear: Should the sweeper be a no-op until v0.2, or run as designed but never see rows?
   - Recommendation: **Run as designed.** Cost is one goroutine + one 30s ticker; the value is verifying STATE-07 acceptance (success criterion 3: "WrapUp with closed browser returns to post_interaction_state automatically after wrapup_until — verified by killing connection + waiting TTL+5s"). The test seeds a WrapUp row and waits — sweeper must fire it.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25 | All phase tasks | ✓ | 1.25 (verified by go.mod) | — |
| PostgreSQL 17 | Migration + integration tests | ✓ | testcontainer postgres:17 already used by Phase 3 (isolation/main_test.go:93) | docker-less skip |
| miniredis | per-test cache fixtures | ✓ | already in go.mod | — |
| testcontainers-go | integration tests | ✓ | already in go.mod | — |
| `github.com/jonboulle/clockwork` v0.4.0+ | TTL goroutine tests | ✗ | — | Planner runs `go get` at Wave 0; if blocked by network, fall back to plain `func() time.Time` + skip AfterFunc determinism tests (degraded coverage) |
| `task` (Taskfile) | codegen-drift cycle | ✓ | already in repo Taskfile.yml | — |
| oapi-codegen v2.7 | server.gen.go regen | ✓ | already in tools.go | — |
| openapi-typescript | TS regen | ✓ | already in web/ | — |
| Docker | testcontainers for integration tests | ⚠️ | only in CI + dev macOS | docker-less lane uses `-short` skip |

**Missing dependencies with no fallback:** none.

**Missing dependencies with fallback:**
- clockwork — fall back to a hand-rolled `Clock interface { Now(); AfterFunc(d, f) }` if `go get` is blocked; reduce AfterFunc test fidelity to `time.Sleep` (acceptable for v0.1 but adds 60s+ to suite — clockwork preferred).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | `testing` stdlib + `github.com/stretchr/testify/require` |
| Config file | N/A (Go test, no separate config) |
| Quick run command | `cd services/api && go test -short ./internal/state/... ./internal/domain/...` |
| Full suite command | `cd services/api && go test ./...` (includes testcontainer + isolation lanes) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STATE-01 | agent_states row per agent with status enum | integration | `go test ./internal/state/... -run TestCreateAgentSeedsState` | ❌ Wave 0 |
| STATE-02 | PATCH accepts allowed transitions | unit (table-driven) | `go test ./internal/state/ -run TestTransitionMatrix_Exhaustive -x` | ❌ Wave 1 |
| STATE-03 | Invalid transitions → 409 with from/to | integration | `go test ./internal/state/ -run TestPatchAgentStatus_InvalidTransition` | ❌ Wave 2 |
| STATE-04 | Ready→Break requires same-org break_reason_id | integration | `go test ./internal/state/ -run TestPatchAgentStatus_BreakReasonProbe` | ❌ Wave 2 |
| STATE-04 | cross-org break_reason → 422 | isolation | `go test ./test/isolation/ -run TestState_BreakReasonCrossOrg` | ❌ Wave 5 |
| STATE-05 | Engaged carries engaged_channel (system-only path schema-ready) | unit + schema | `go test ./internal/state/ -run TestTransitionMatrix_EngagedRequiresChannel` | ❌ Wave 1 |
| STATE-06 | Agent sets post_interaction_state while Engaged | integration | `go test ./internal/state/ -run TestPatchAgentStatus_PostInteractionState` | ❌ Wave 2 |
| STATE-07 | WrapUp TTL fires server-side | integration (clockwork) | `go test ./internal/state/ -run TestWrapUpTTL_FiresOnExpiry` | ❌ Wave 3 |
| STATE-07 | WrapUp survives browser disconnect (acceptance) | integration | `go test ./internal/state/ -run TestWrapUpTTL_AcceptanceTTLPlus5` | ❌ Wave 5 |
| STATE-08 | Monotonic state_version | unit | `go test ./internal/state/ -run TestStateVersion_Monotonic` | ❌ Wave 2 |
| STATE-09 | Login/logout — schema-only (no v0.1 fire) | unit | `go test ./internal/state/ -run TestMatrix_LogoutDeferred` | ❌ Wave 1 |
| STATE-10 | IsRoutable 4-case matrix | unit | `go test ./internal/domain/ -run TestIsRoutable` | ❌ Wave 1 |
| Acceptance #1 | All allowed/rejected transitions | integration (table-driven) | `go test ./internal/state/ -run TestAcceptance_AllTransitions` | ❌ Wave 5 |
| Acceptance #2 | Cross-org break_reason rejection | isolation | `go test ./test/isolation/ -run TestState_CrossOrgBreakReason` | ❌ Wave 5 |
| Acceptance #3 | WrapUp+disconnect+TTL+5s | integration | `go test ./internal/state/ -run TestAcceptance_WrapUpExpiresWithoutClient` | ❌ Wave 5 |
| Acceptance #4 | IsRoutable 4 cases | unit | `go test ./internal/domain/ -run TestIsRoutable` | covered by STATE-10 |
| Acceptance #5 | state_version monotonic on every mutation | integration | `go test ./internal/state/ -run TestAcceptance_StateVersionMonotonic` | ❌ Wave 5 |
| force=true happy path | force bypasses matrix | integration | `go test ./internal/state/ -run TestForce_BypassesMatrix` | ❌ Wave 2 |
| force=true with cross-org break_reason still 422 | isolation | `go test ./test/isolation/ -run TestState_ForceDoesNotBypassCrossOrgProbe` | ❌ Wave 5 |

### Sampling Rate
- **Per task commit:** `go test -short ./internal/state/... ./internal/domain/...` (unit-only; ~5s)
- **Per wave merge:** `go test ./internal/state/... ./internal/domain/... ./test/isolation/...` (integration + isolation; ~60s with testcontainers)
- **Phase gate:** `go test ./... && task gen && git diff --exit-code` (full suite + codegen-drift gate)

### Wave 0 Gaps
- [ ] `services/api/internal/state/agent_states_test.go` — covers STATE-01..STATE-06, STATE-08, force
- [ ] `services/api/internal/state/transitions_test.go` — covers STATE-02, STATE-05, STATE-09 (matrix exhaustive walk)
- [ ] `services/api/internal/state/ttl_test.go` — covers STATE-07 with clockwork.FakeClock
- [ ] `services/api/internal/state/testutil_test.go` — shared fixtures (mirrors catalog/testutil_test.go)
- [ ] `services/api/internal/domain/state_test.go` — covers STATE-10
- [ ] `services/api/test/isolation/state_test.go` — covers D-94 cross-org probes
- [ ] `services/api/internal/server/request_id_exhaustiveness_test.go` (EDITED) — covers any new response types from D-92
- [ ] Framework install: `go get github.com/jonboulle/clockwork@v0.4.0` — Wave 0

## Wave Decomposition Proposal

Six waves matching the focus-area structure in CONTEXT.md additional context.

| Wave | Goal | Tasks | Dependencies | Verify |
|------|------|-------|--------------|--------|
| **Wave 0 — Contract + Schema + Allowlist** | Make the system compile against new schema + spec | (a) Edit `openapi/openapi.yaml`: add `force` field. (b) Append `agent_states` table + indexes to `migrations/000002_catalog_v0_1.up.sql`. (c) Prepend `DROP TABLE IF EXISTS agent_states;` to `.down.sql`. (d) Add `agent_states` to `tenantTables` in `sqlcheck.go`. (e) Run `task gen` + commit `*.gen.go` + `generated.ts`. (f) `go get github.com/jonboulle/clockwork`. | — (root) | `task gen && git diff --exit-code`; existing tests pass; `go build ./...` clean |
| **Wave 1 — Pure code: sqlc + domain + state skeleton** | New code with NO behavior change in existing handlers | (a) Add `agent_states.sql` queries; sqlc regenerates `agent_states.sql.go`. (b) Add `BreakReasonExistsInOrg`/`GetBreakReasonForState` probe to `break_reasons.sql`. (c) Create `internal/domain/state.go` (IsRoutable) + 4-case `state_test.go`. (d) Create `internal/state/` package skeleton (`handlers.go`, `transitions.go` with matrix data, `agent_states.go` with method stubs that delegate to TBD). | Wave 0 | `go test ./internal/domain/...` passes; `go vet ./internal/state/` clean; transitions_test.go matrix walk passes |
| **Wave 2 — Handler bodies: GET/PATCH** | Replace state.Handlers stubs with real bodies; cache + tx + probes | (a) `state.agent_states.go`: GetAgentStatus via `cache.GetOrSet[api.AgentState]`. (b) PatchAgentStatus: validator → break_reason probe → tx → UPDATE with expected_from → 0-row disambig → cache.Del → 200/404/409/422. (c) force=true branch using ForceUpdateAgentStateStatus. (d) Wire force WARN slog. (e) `agent_states_test.go` covering STATE-02/03/04/06/08 + force. | Wave 1 | per-test + per-wave full integration sweep; transitions table walk |
| **Wave 3 — TTL goroutine + sweeper + lifecycle** | WrapUp expiry fires deterministically | (a) `state/ttl.go`: timersRegistry + scheduleWrapUpExpiry + expireWrapUp. (b) Start(ctx): synchronous startup sweep (loads WrapUps, fires past-due, schedules future). (c) Safety-sweep loop (30s ticker). (d) Stop(): cancel ctx, wg.Wait, iterate timers, Stop each. (e) clockwork.Clock injection via WithClock Option. (f) `ttl_test.go` using FakeClock; advance time + assert UPDATE landed. | Wave 1 (for sqlc queries) | TestWrapUpTTL_FiresOnExpiry; `go test -race ./internal/state/...` passes |
| **Wave 4 — Composite wiring + atomic agent INSERT extension** | One running binary that exposes the new endpoints | (a) Rename `state.Handlers` → `state.Server` (Pitfall 1). (b) Declare `ApiHandlers` composite in `cmd/api/main.go`. (c) Construct `stateServer := state.New(...)`. (d) Call `stateServer.Start(ctx)` + `defer stateServer.Stop()`. (e) Pass `&ApiHandlers{Handlers: catalogHandlers, Server: stateServer}` to `server.NewMux`. (f) Edit `catalog/agents.go` CreateAgent to add `qtx.InsertAgentState(...)` inside the existing tx. (g) Delete `GetAgentStatus`/`PatchAgentStatus` stubs from `catalog/notimpl.go`. (h) Extend `injectRequestIDIntoErrorResponse` if any new types appear from codegen. | Waves 2 + 3 | `go build ./...` clean; isolation main_test bring-up updated; TestRequestIDInjection_Exhaustiveness passes |
| **Wave 5 — Cross-org + force + acceptance suite** | All 5 success criteria green | (a) Create `services/api/test/isolation/state_test.go` with D-94 (a)(b)(c)(d). (b) Acceptance tests for the 5 ROADMAP success criteria. (c) Cross-AI peer review (Codex + Gemini) per project rule. (d) PATTERNS.md amended with Pitfall 6 (Phase 5 bulk import must seed agent_states). | Wave 4 | `go test ./test/isolation/...` passes; full `go test ./...` passes; cross-AI verdict "READY" or "READY WITH FIXES" |

Inter-wave dependencies:
- Wave 1 depends on Wave 0 (schema must exist; sqlc gen needs the table).
- Waves 2 and 3 can proceed in parallel after Wave 1 (different files; no shared state changes).
- Wave 4 depends on both 2 and 3 (needs handler bodies + Start/Stop).
- Wave 5 depends on Wave 4 (needs running endpoints).

## Project Constraints (from CLAUDE.md)

The project-root `CLAUDE.md` adds two hard rules every wave must respect:

1. **WHY-not-WHAT comments.** Default to no comments. Use comments only for hidden constraints, invariants, bug workarounds, or trade-offs. Code reviewer rejects WHAT/HOW comments. The research above adopts this style — every comment in the example snippets explains intent (D-NN reference, Codex iter reference, hazard, race condition, etc.), never restates the code.
2. **Cross-AI peer review at end of each work unit.** After any wave (≥3 commits or ≥5 files), invoke Codex + Gemini in parallel on the diff using the protocol in `~/.claude/CLAUDE.md`. Wave 5 ends with a final cross-AI sweep against the full Phase 4 diff before declaring done.

These constraints are LOCKED with the same authority as CONTEXT.md decisions.

## Sources

### Primary (HIGH confidence)
- `services/api/internal/cache/cache.go` — cache layer (verified in-tree code).
- `services/api/internal/db/orgdb.go` — OrgTx and SQLChecker (verified in-tree code).
- `services/api/internal/db/sqlcheck.go` — `tenantTables` map (verified in-tree code).
- `services/api/internal/catalog/agents.go` — Codex C4 tx pattern + D-66 disambiguate (verified in-tree code).
- `services/api/internal/catalog/channels.go` — D-76 cross-row probe pattern (verified in-tree code).
- `services/api/internal/catalog/handlers.go` — D-71 hybrid constructor (verified in-tree code).
- `services/api/internal/catalog/notimpl.go` — D-70 forward-compat 501 stubs (verified in-tree code).
- `services/api/internal/db/queries/agents.sql` — sqlc patterns (verified in-tree code).
- `services/api/internal/server/server.go` — RequestID injection type switch (verified in-tree code).
- `services/api/internal/api/server.gen.go` — GetAgentStatus / PatchAgentStatus request/response objects (verified in-tree code).
- `services/api/internal/api/types.gen.go` — AgentState / AgentStatus typed enum (verified in-tree code).
- `migrations/000002_catalog_v0_1.up.sql` — extant Phase 3 migration (verified in-tree code).
- `services/api/test/isolation/main_test.go` — testcontainer wiring (verified in-tree code).
- `services/api/test/isolation/catalog_test.go` — cross-org probe pattern (verified in-tree code).
- `openapi/openapi.yaml` lines 980-1121, 1753-1850 — AgentState schema, PatchAgentStatusRequest, endpoints (verified spec).
- `.planning/phases/04-agent-state-machine-go/04-CONTEXT.md` — D-78..D-95 (locked decisions, verified).
- `.planning/phases/03-catalog-crud-go/03-CONTEXT.md` — D-49..D-77 + A6/A7 (locked decisions, verified).
- `.planning/phases/02-openapi-contract-codegen/02-CONTEXT.md` — D-32..D-48 (locked decisions, verified).
- `.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md` — D-01..D-31 (locked decisions, verified).
- `.planning/REQUIREMENTS.md` — STATE-01..STATE-10 (verified).

### Secondary (MEDIUM confidence)
- `pkg.go.dev/time` — `time.AfterFunc`, `time.Timer.Stop` semantics [CITED: pkg.go.dev/time].
- `pkg.go.dev/sync` — `sync.RWMutex`, `sync.WaitGroup` patterns [CITED: pkg.go.dev/sync].
- `pkg.go.dev/log/slog` — structured logging [CITED: pkg.go.dev/log/slog].
- `github.com/jonboulle/clockwork` README — FakeClock + AfterFunc semantics [CITED: github.com/jonboulle/clockwork].
- `github.com/benbjohnson/clock` README — archived; clockwork named as successor [CITED: github.com/benbjohnson/clock].
- `coder.com/blog/introducing-quartz` — Quartz vs clockwork trade-offs [CITED: coder.com/blog/introducing-quartz].

### Tertiary (LOW confidence)
- WebSearch result on Go fake-clock libraries (verified by reading README excerpts — no LOW-only claims survive into final research).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every dependency except clockwork is already in go.mod; clockwork is the widely-adopted choice for AfterFunc mocking.
- Architecture: HIGH — Phase 3 has already exercised every load-bearing pattern (cache, tx, probe, codegen-drift, exhaustiveness test, cross-org isolation).
- Pitfalls: HIGH — Pitfalls 1 (composite-naming collision), 8 (sweeper org bypass), and 6 (Phase 5 inheritance) come from reading the codebase + locked decisions. Pitfall 2 (timer race) is documented in `pkg.go.dev/time`.
- Schema: HIGH — D-78 verbatim from CONTEXT.md; Phase 3 migration verifies the append pattern works.
- TTL goroutine: MEDIUM-HIGH — `time.AfterFunc` semantics are stable; clockwork integration verified via README; first time this pattern lands in the repo.

**Research date:** 2026-05-17
**Valid until:** 2026-06-16 (30 days for stable Go/Postgres/Phase 3-extending work).

## RESEARCH COMPLETE
