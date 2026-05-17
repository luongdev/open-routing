---
phase: 04-agent-state-machine-go
verified: 2026-05-17T00:00:00Z
status: passed
score: 5/5 ROADMAP success criteria verified; 10/10 STATE-* requirements verified
overrides_applied: 0
re_verification: false
---

# Phase 4: Agent State Machine (Go) Verification Report

**Phase Goal:** The system enforces a strict, org-scoped agent status model where invalid transitions are rejected at the domain layer, WrapUp cannot get stuck on browser disconnect, and IsRoutable is correct for every Break sub-reason combination.
**Verified:** 2026-05-17
**Status:** PASS
**Re-verification:** No — initial verification

---

## Verdict: PASS

All 5 ROADMAP success criteria verified with file:line evidence. All 10 STATE-* requirements verified. All 4 cross-AI HIGH fixes confirmed in code. All 5 Pitfall mitigations confirmed. Full test suite passed (`go test -count=1 ./...` and `go test -race -count=1 ./...`). `go vet ./...` clean.

---

## ROADMAP Success Criteria

| # | Criterion | Status | Evidence |
|---|-----------|--------|----------|
| 1 | `PATCH /agents/{id}/status` accepts every allowed transition and rejects others with 409 `invalid_transition` | VERIFIED | `transitions_test.go:12` — `TestTransitionMatrix_Exhaustive` walks all 6×6 = 36 pairs; 7 allowed confirmed; `agent_states_test.go:392` — `TestAcceptance_AllTransitions` runs all 7 allowed + 3 rejected live against real Postgres |
| 2 | `Ready→Break` with cross-org or non-existent `break_reason_id` returns 422; valid same-org reason succeeds and records the reason | VERIFIED | `agent_states_test.go:173` — `TestPatchAgentStatus_BreakReasonRequired_422`; line 201 — `TestPatchAgentStatus_BreakReasonProbe_422_missing`; line 229 — `TestPatchAgentStatus_BreakReasonProbe_200_valid`; `test/isolation/state_test.go:121` — `TestState_PatchStatusCrossOrgBreakReason_422`; `agent_states.go:179` — probe runs BEFORE matrix check |
| 3 | WrapUp agent returns to `post_interaction_state` automatically after `wrapup_until` expires, server-side | VERIFIED | `ttl.go:30` — `scheduleWrapUpExpiry` with real clock AfterFunc; `ttl_test.go:38` — `TestWrapUpTTL_FiresOnExpiry` (clockwork-driven); `agent_states_test.go:496` — `TestAcceptance_WrapUpExpiresWithoutClient` uses real 1s TTL, 8s Eventually; `agent_states.sql:85` — `ExpireWrapUp` idempotent UPDATE |
| 4 | `IsRoutable` returns `true` only for `Ready` or `Break+routable=true`; all four combinations covered by unit tests | VERIFIED | `domain/state.go:18` — pure `IsRoutable` function; `domain/state_test.go:9` — `TestIsRoutable` with 10 cases (Ready, Break+routable, Break+!routable, NotReady, Engaged, WrapUp, Offline, unknown, empty); `agent_states_test.go:539` — `TestAcceptance_IsRoutableMatrix` traceability wrapper |
| 5 | Every state mutation increments `state_version` monotonically; `agent_states` table carries all required columns from initial migration | VERIFIED | `agent_states.sql:35` — `state_version = state_version + 1` in `UpdateAgentStateStatus`; `agent_states.sql:55` — same in `ForceUpdateAgentStateStatus`; `agent_states.sql:93` — same in `ExpireWrapUp`; `agent_states_test.go:546` — `TestAcceptance_StateVersionMonotonic` (5 transitions verify versions 1→2→3→4→5→6); migration line 191-204 — all required columns present |

---

## STATE-* Requirements Coverage

| Req | Description | Status | Evidence |
|-----|-------------|--------|----------|
| STATE-01 | `agent_states` row per agent with `status ∈ {Ready,NotReady,Break,Engaged,WrapUp,Offline}` | VERIFIED | `migrations/000002_catalog_v0_1.up.sql:191` — table definition with CHECK constraint; `catalog/agents.go:152` — `InsertAgentState` called atomically inside `CreateAgent` tx |
| STATE-02 | PATCH accepts: NotReady↔Ready, Ready→Break, Break→Ready, Break→NotReady, WrapUp→Ready, WrapUp→NotReady | VERIFIED | `transitions.go:32` — matrix encodes exactly 7 edges; `transitions_test.go:12` — exhaustive walk confirms all 7 allowed (require.Len check enforces count); `agent_states_test.go:392` — live acceptance test |
| STATE-03 | Invalid transitions → 409 with `{"from","to","error":"invalid_transition"}` | VERIFIED | `agent_states.go:208-215` — `validateTransition` error → `PatchAgentStatus409JSONResponse(InvalidTransitionErrorResponse{...})`; `agent_states_test.go:143` — `TestPatchAgentStatus_InvalidTransition_409` asserts `api.InvalidTransition` + `from/to` fields |
| STATE-04 | `Ready→Break` requires same-org `break_reason_id`; missing/cross-org → 422 | VERIFIED | `agent_states.go:179-203` — break_reason probe runs first (before matrix); `agent_states_test.go:173` — nil 422; line 201 — non-existent 422; `test/isolation/state_test.go:121` — cross-org 422; line 167 — force=true also 422 |
| STATE-05 | `Engaged` carries `engaged_channel`; system-initiated transitions schema-ready | VERIFIED | `migrations/000002_catalog_v0_1.up.sql:196` — `engaged_channel CHECK`; `agent_states.sql:30-34` — preserves channel only while target remains Engaged and clears it otherwise; `agent_states_test.go:293` — forced Engaged→NotReady clears stale channel; `transitions_test.go:57` — `TestMatrix_EngagedDeferred` confirms Ready→Engaged is NOT in v0.1 matrix |
| STATE-06 | Agent can set `post_interaction_state` while Engaged | VERIFIED | `agent_states.go:315-318` — `buildUpdateParams` gates PIS write on `expectedFrom==Engaged AND target∈{Engaged,WrapUp}`; `agent_states.go:338-342` — same in `buildForceUpdateParams`; `agent_states.sql:33` — explicit assignment (not COALESCE) so nil clears stale value |
| STATE-07 | Server-owned WrapUp TTL goroutine fires `WrapUp → post_interaction_state` on expiry | VERIFIED | `ttl.go:30` — `scheduleWrapUpExpiry` per-agent `clockwork.AfterFunc`; `ttl.go:132` — `startupSweep`; `ttl.go:160` — `safetySweep` 30s ticker; `handlers.go:97` — `Start` runs startup sweep synchronously; `ttl_test.go:38,91,135,173` — clockwork-driven TTL tests |
| STATE-08 | Monotonic `state_version` on every mutation | VERIFIED | `agent_states.sql:35,55,93` — `state_version = state_version + 1` in all three UPDATE queries; `agent_states_test.go:102` — `TestPatchAgentStatus_StateVersionMonotonic`; line 546 — `TestAcceptance_StateVersionMonotonic` |
| STATE-09 | Login/logout system transitions deferred; schema-ready | VERIFIED (deferred per D-82) | `04-CONTEXT.md` — "Login/logout STATE-09 (Offline↔NotReady) deferred to AUTH phase"; `transitions_test.go:40` — `TestMatrix_SystemOnlyTransitionsAbsent` confirms Offline→NotReady not in v0.1 matrix; schema ships all required columns |
| STATE-10 | `IsRoutable` helper returns correct values | VERIFIED | `domain/state.go:18` — pure function, no I/O; `domain/state_test.go:9` — 10-case table including all required 4 combinations + boundary cases |

---

## Cross-AI HIGH Fix Audit

| Fix | Expected Pattern | Status | Evidence |
|-----|-----------------|--------|----------|
| F-1 | Wave 3 depends_on bump to 04-03 (executor-time only; code-invisible) | VERIFIED (not observable in code; executor-time plan fix) | Plan dependency graph documented in `04-06-SUMMARY.md` frontmatter |
| F-2 | `ExpireWrapUp` uses CASE post_interaction_state translation (not COALESCE) and clears PIS after expiry — Gemini HIGH-1 | VERIFIED | `agent_states.sql:92-99` — `CASE post_interaction_state WHEN 'ready' THEN 'Ready' WHEN 'not_ready' THEN 'NotReady' ELSE 'NotReady' END`, then `post_interaction_state = NULL`; `ttl_test.go:135` covers both target states |
| F-3 | `scheduleWrapUpExpiry` uses `var t clockwork.Timer` + `cur == t` pointer-equality — Gemini HIGH-2 | VERIFIED | `ttl.go:57` — `var t clockwork.Timer`; `ttl.go:76` — `if cur, ok := s.timers.t[agentID]; ok && cur == t`; comment on lines 26-29 documents the pointer-equality rationale |
| F-4 | `jitter()` function, `uuid.New().Time()`, `pgtype.Text` guard, and `pgtype` import all ABSENT (Codex HIGH-2 + Gemini MED) | VERIFIED (absent) | grep across `ttl.go`, `handlers.go`, `agent_states.go` returns zero matches for `jitter`, `uuid.New`, `pgtype.Text`; `pgtype` IS imported in `agent_states.go:10` as `pgtype.UUID` (correct use); no spurious pgtype.Text |

---

## Pitfall Mitigation Audit

| Pitfall | Expected | Status | Evidence |
|---------|----------|--------|----------|
| P1 | Type is `state.Server` (NOT `state.Handlers`) | VERIFIED | `handlers.go:52` — `type Server struct`; `main.go:180` — `*state.Server` in ApiHandlers composite; `transitions.go:7` — package comment "Pitfall 1: the exported handler type is `Server` (NOT `Handlers`)" |
| P3 | Probe-then-matrix ordering preserved | VERIFIED | `agent_states.go:102` — `PatchAgentStatus` docstring states "break_reason probe FIRST regardless of force flag"; `agent_states.go:176-203` — STEP 1 (break_reason probe) before STEP 2 (matrix validation); `test/isolation/state_test.go:167` — `TestState_ForceDoesNotBypassCrossOrgBreakReason_422` regression gate |
| P4 | `tenantTables` allowlist contains `agent_states` | VERIFIED | `services/api/internal/db/sqlcheck.go:41` — `"agent_states": {}, // STATE-01 (Phase 4...)` |
| P6 | Phase 5 inheritance reminder appended to `04-PATTERNS.md` | VERIFIED | `04-PATTERNS.md:1249` — "Hazard 7: Phase 5 bulk import skips agent_states INSERT (Pitfall 6)"; documents the requirement that Phase 5 must INSERT agent_states for every new agent |
| P10 | `CreateAgent` uses `qtx.InsertAgentState` (not `generated.New` on the pool) | VERIFIED | `catalog/agents.go:152` — `qtx.InsertAgentState(ctx, ...)` inside the existing OrgTx; comment on lines 142-149 documents the D-93 atomicity invariant |

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/api/internal/state/handlers.go` | Server type + lifecycle (D-88, Pitfall 1) | VERIFIED | 164 lines; `Server` struct with deps, clock, sweepInterval, timers, ctx, cancel, wg; `Start`/`Stop` lifecycle |
| `services/api/internal/state/transitions.go` | 7-edge matrix + validateTransition | VERIFIED | 83 lines; `matrix` encodes exactly 7 agent-initiated edges; `allStatuses()` for exhaustive test |
| `services/api/internal/state/ttl.go` | AfterFunc registry + safety sweep + startup sweep | VERIFIED | 197 lines; `scheduleWrapUpExpiry`, `expireWrapUp`, `startupSweep`, `safetySweep`, `runSweepPastDue` |
| `services/api/internal/state/agent_states.go` | GetAgentStatus + PatchAgentStatus handlers | VERIFIED | 347 lines; both handlers implemented with probe-then-matrix, cache, force, enum validation |
| `services/api/internal/domain/state.go` | Pure `IsRoutable` function (STATE-10, D-90) | VERIFIED | 27 lines; no I/O; string comparison against api constants |
| `services/api/internal/domain/state_test.go` | 4+ cases for IsRoutable | VERIFIED | 10 test cases covering all required combinations + boundary cases |
| `services/api/internal/state/transitions_test.go` | Exhaustive matrix walk | VERIFIED | 62 lines; `TestTransitionMatrix_Exhaustive` asserts 7 allowed + all others rejected |
| `services/api/internal/state/agent_states_test.go` | Integration tests + acceptance tests | VERIFIED | ~630 lines; 42 tests covering all STATE-* + acceptance criteria |
| `services/api/internal/state/ttl_test.go` | Clockwork-driven TTL tests | VERIFIED | ~230 lines; 6 TTL tests using `clockwork.NewFakeClock()` |
| `services/api/test/isolation/state_test.go` | D-94 cross-org probes (4 tests + D-93 gate) | VERIFIED | 215 lines; 5 tests: cross-org GET 404, PATCH 404, break_reason 422, force 422, D-93 seed gate |
| `migrations/000002_catalog_v0_1.up.sql` | agent_states table appended (D-78) | VERIFIED | Lines 191-212; all required columns + CHECK constraints + 2 indexes |
| `services/api/internal/db/queries/agent_states.sql` | 5 SQL queries | VERIFIED | InsertAgentState, GetAgentStateByAgentId, UpdateAgentStateStatus, ForceUpdateAgentStateStatus, ListExpiringWrapUps, ExpireWrapUp |
| `services/api/internal/db/sqlcheck.go` | `agent_states` in tenantTables | VERIFIED | Line 41 |
| `services/api/internal/catalog/agents.go` | `InsertAgentState` in CreateAgent tx (D-93, Pitfall 10) | VERIFIED | Lines 142-159; `qtx.InsertAgentState` inside existing OrgTx |
| `services/api/internal/catalog/notimpl.go` | `GetAgentStatus`/`PatchAgentStatus` stubs removed | VERIFIED | File comment confirms removal; only Phase 5 stubs remain |
| `services/api/cmd/api/main.go` | ApiHandlers composite + Start/Stop lifecycle (D-89, D-95) | VERIFIED | Lines 178-194; `type ApiHandlers struct { *catalog.Handlers; *state.Server }`; compile-time assertion; `stateServer.Start(ctx)` + `defer stateServer.Stop()` |
| `openapi/openapi.yaml` | `force: boolean` field on PatchAgentStatusRequest (D-92) | VERIFIED | Line 1122; force field with description documenting matrix bypass + probe still runs |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `agent_states.go:PatchAgentStatus` | `transitions.go:validateTransition` | direct call `validateTransition(observedFrom, targetStatus)` | WIRED | line 209 |
| `agent_states.go:PatchAgentStatus` | `db/queries:GetBreakReasonForState` | `q.GetBreakReasonForState(ctx, ...)` | WIRED | line 187 |
| `agent_states.go:PatchAgentStatus` | `db/queries:UpdateAgentStateStatus` | `qtx.UpdateAgentStateStatus(ctx, ...)` | WIRED | line 235 |
| `agent_states.go:PatchAgentStatus` | `cache.Del` | `s.deps.Cache.Del(ctx, s.cacheKeyFor(...))` | WIRED | line 279 |
| `agent_states.go:GetAgentStatus` | `cache.GetOrSet` | `cache.GetOrSet[api.AgentState](ctx, ...)` | WIRED | line 69 |
| `handlers.go:Start` | `ttl.go:startupSweep` | `s.startupSweep(s.ctx)` | WIRED | line 108 |
| `handlers.go:Start` | `ttl.go:safetySweep` | `go s.safetySweep()` | WIRED | line 115 |
| `ttl.go:scheduleWrapUpExpiry` | `db/queries:ExpireWrapUp` | via `s.expireWrapUp(...)` → `q.ExpireWrapUp(...)` | WIRED | lines 73, 95 |
| `catalog/agents.go:CreateAgent` | `db/queries:InsertAgentState` | `qtx.InsertAgentState(ctx, ...)` inside OrgTx | WIRED | line 152 |
| `main.go:ApiHandlers` | `api.StrictServerInterface` | compile-time `var _ api.StrictServerInterface = (*ApiHandlers)(nil)` | WIRED | line 189 |

---

## Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `agent_states.go:GetAgentStatus` | `state api.AgentState` | `q.GetAgentStateByAgentId` → real Postgres SELECT | Yes | FLOWING |
| `agent_states.go:PatchAgentStatus` | `row generated.AgentState` | `qtx.UpdateAgentStateStatus` / `ForceUpdateAgentStateStatus` → real Postgres UPDATE RETURNING | Yes | FLOWING |
| `ttl.go:expireWrapUp` | `row generated.ExpireWrapUp result` | `q.ExpireWrapUp` → real Postgres UPDATE RETURNING | Yes | FLOWING |
| `ttl.go:startupSweep` | `rows []generated.ListExpiringWrapUps result` | `q.ListExpiringWrapUps` → real Postgres SELECT | Yes | FLOWING |
| `domain/state.go:IsRoutable` | `AgentStateInputs.Status + BreakReasonRoutable` | Pure function — caller provides scalars loaded from DB/cache | N/A (pure) | VERIFIED |

---

## Behavioral Spot-Checks

| Behavior | Evidence | Status |
|----------|----------|--------|
| `TestTransitionMatrix_Exhaustive` — 7 allowed, 29 rejected | `transitions_test.go:12` — `require.Len(t, allowed, 7)` + full 6×6 walk | PASS |
| `TestAcceptance_AllTransitions` — live DB | `agent_states_test.go:392` — 10 sub-tests on real Postgres | PASS |
| `TestAcceptance_WrapUpExpiresWithoutClient` — real clock 1s TTL | `agent_states_test.go:496` — 8s timeout, 100ms poll | PASS |
| `TestState_ForceDoesNotBypassCrossOrgBreakReason_422` — D-94(c) | `test/isolation/state_test.go:167` — force=true + cross-org still 422 | PASS |
| `TestCreateAgentSeedsState` — D-93 atomic seeding | `test/isolation/state_test.go:199` — GET /status immediately after POST /agents → 200 | PASS |
| Full suite: `go test -count=1 ./...` | passed | PASS |
| `go vet ./...` | No issues | PASS |

---

## Probe Execution

Step 7c: SKIPPED (no probe-*.sh scripts declared in any phase plan or found under `scripts/*/tests/`). Phase relied on `go test` suite exclusively.

---

## Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| STATE-01 | 04-01-PLAN.md | agent_states schema | SATISFIED | migration:191 + catalog/agents.go:152 |
| STATE-02 | 04-02-PLAN.md | Allowed transitions via PATCH | SATISFIED | transitions.go:32 + transitions_test.go:12 |
| STATE-03 | 04-03-PLAN.md | Invalid → 409 with from/to | SATISFIED | agent_states.go:208 + agent_states_test.go:143 |
| STATE-04 | 04-03-PLAN.md | Break requires same-org break_reason_id | SATISFIED | agent_states.go:179 + isolation/state_test.go:121 |
| STATE-05 | 04-02-PLAN.md | Engaged carries engaged_channel | SATISFIED | migration:196 + agent_states.sql:30-34 + agent_states_test.go:293 |
| STATE-06 | 04-03-PLAN.md | post_interaction_state while Engaged | SATISFIED | agent_states.go:315-318 |
| STATE-07 | 04-04-PLAN.md | Server-owned WrapUp TTL | SATISFIED | ttl.go:30 + ttl_test.go:38 + agent_states_test.go:496 |
| STATE-08 | 04-03-PLAN.md | Monotonic state_version | SATISFIED | agent_states.sql:35,55,93 + agent_states_test.go:546 |
| STATE-09 | 04-02-PLAN.md | Login/logout deferred; schema-ready | SATISFIED (deferred) | D-82 documented; transitions_test.go:40 verifies absence from matrix |
| STATE-10 | 04-02-PLAN.md | IsRoutable helper | SATISFIED | domain/state.go:18 + domain/state_test.go:9 |

---

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `catalog/notimpl.go` | 28,33 | `BulkImportCatalog` / `GetImportJob` return 501-equivalent | Info | Intentional Phase 5 stubs; comment explicitly attributes them to Phase 5 |

No debt-marker comments (TBD/FIXME/XXX) found in Phase 4 files. No unreferenced TODOs. No empty implementations in state package files.

---

## Human Verification Required

None. All Phase 4 acceptance criteria are covered by automated integration tests. The VALIDATION.md explicitly states "All Phase 4 acceptance criteria are covered by automated integration tests (clockwork enables deterministic TTL expiry; testcontainers provide real Postgres; miniredis covers cache)."

---

## Test Run Results

| Suite | Tests | Result |
|-------|-------|--------|
| `internal/state` | 42 | PASS |
| `internal/domain` | 11 | PASS |
| `test/isolation` | 24 | PASS |
| Full suite (`./...`) | **404 total across 16 packages** | **PASS** |

Command: `cd services/api && go test -count=1 ./... 2>&1` — passed.

Command: `cd services/api && go test -race -count=1 ./... 2>&1` — passed.
`go vet ./...` — No issues.

---

## Gaps Summary

No gaps. All ROADMAP success criteria, STATE-* requirements, cross-AI HIGH fixes, and Pitfall mitigations are verified in code and confirmed by a green test suite.

The only Phase 5 stubs present (`BulkImportCatalog`, `GetImportJob` in `catalog/notimpl.go`) are intentional and correctly attributed to Phase 5 per the phase boundary in 04-CONTEXT.md.

---

## Recommended Next Step

**Mark Phase 4 complete. Proceed to Phase 5 (Bulk Import).**

Phase 5 MUST observe the following invariants captured in `04-PATTERNS.md:1249`:
- Every new agent inserted via bulk import must also INSERT an `agent_states` row atomically in the same transaction (Hazard 7 / Pitfall 6).
- `InsertAgentState` must use `qtx.InsertAgentState` (not `generated.New` on the pool) to ensure the state row participates in the same OrgTx as the agent row.

---

_Verified: 2026-05-17_
_Verifier: Claude (gsd-verifier) — goal-backward analysis against ROADMAP.md success criteria_
