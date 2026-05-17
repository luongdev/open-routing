---
phase: 04-agent-state-machine-go
plan: 03
subsystem: api
tags: [state-machine, cache, tdd, go, wave-2]

requires:
  - phase: 04-agent-state-machine-go
    plan: 02
    provides: "sqlc queries + state.Server skeleton + transition matrix + wave_1_stub placeholders"

provides:
  - "GetAgentStatus: cache.GetOrSet[api.AgentState] with 60s TTL + ErrNoRows→404 (D-86, STATE-01)"
  - "PatchAgentStatus: probe-then-matrix ordering + force bypass + D-66 disambiguate + cache.Del (D-84, D-85, D-66)"
  - "testutil_test.go: newTestHandlers + seedAgent/seedBreakReason/seedAgentStateRow + stateOnlyHandlers (D-73)"
  - "main_test.go: testcontainer postgres:17 bring-up (D-73)"
  - "mappers.go: mapAgentState row→DTO + pgUUID helper (D-49)"
  - "agent_states_test.go: 13 GREEN tests covering STATE-02/03/04/06/08 + force + cache invalidation"

affects: [04-04, 04-05, 04-06]

tech-stack:
  added: []
  patterns:
    - "Probe-then-matrix ordering: break_reason probe runs before validateTransition (Pitfall 3 enforcement)"
    - "D-66 disambiguate inside tx: 0-row UPDATE → tx-scoped SELECT → 404 vs 409"
    - "D-56 cache.Del on both success and 409 disambiguate paths"
    - "D-84 force WARN emitted post-commit (audit reflects actual state change)"
    - "stateOnlyHandlers test composite: panic stubs for non-state methods (Wave 2 TEMP; Wave 4 replaces)"
    - "seedAgentStateRow bypasses transition matrix via raw INSERT (D-82 — system-only states)"

key-files:
  created:
    - "services/api/internal/state/agent_states.go (265 lines — real GetAgentStatus + PatchAgentStatus)"
    - "services/api/internal/state/mappers.go (53 lines — mapAgentState + pgUUID)"
    - "services/api/internal/state/main_test.go (137 lines — testcontainer TestMain)"
    - "services/api/internal/state/testutil_test.go (456 lines — test fixtures + stateOnlyHandlers)"
    - "services/api/internal/state/agent_states_test.go (422 lines — 13 tests)"
  modified: []

decisions:
  - "InvalidTransitionErrorResponse.Error uses api.InvalidTransition (the typed constant) not api.ErrorCodeInvalidTransition — these are different generated types; the 409 body uses InvalidTransitionErrorResponseError not ErrorCode"
  - "stateOnlyHandlers embeds *Server with explicit panic methods for all non-state interface members — api.Unimplemented satisfies the non-strict ServerInterface, not StrictServerInterface"
  - "seedAgentStateRow uses ON CONFLICT DO UPDATE to allow tests to override the Offline row seeded by seedAgent without requiring separate teardown steps"
  - "Task 5 (server.go) was a confirmed no-op: PatchAgentStatus409JSONResponse was already in the type switch from Wave 1 codegen work"
  - "Cross-AI review tools (Codex + Gemini) both failed due to 15k-line diff context window exhaustion — manual verification substituted"

metrics:
  duration: ~40min
  completed: 2026-05-17
---

# Phase 4 Plan 03: Agent State Machine Wave 2 Summary

**Real GetAgentStatus + PatchAgentStatus handler bodies; 13 tests GREEN; probe-then-matrix ordering verified; wave_1_stub strings gone; Pitfall 3 regression gate in place**

## Performance

- **Duration:** ~40 min
- **Completed:** 2026-05-17
- **Tasks:** 5 (Tasks 1-4 substantive; Task 5 no-op)
- **Files created:** 5 new source/test files
- **Files modified:** 0 existing files (agent_states.go replaced stub content)
- **Test count:** 134 passing (state: 16, server: 118)

## Accomplishments

- `wave_1_stub` placeholders replaced in `agent_states.go` — `grep -c wave_1_stub` returns 0
- `GetAgentStatus` serves via `cache.GetOrSet[api.AgentState]` with `or:{orgId}:agent_state:{agent_id}` key and 60s TTL (D-86)
- `PatchAgentStatus` enforces probe-then-matrix ordering (Pitfall 3 / T-04-06):
  - break_reason probe at line 120, validateTransition at line 142 — ordering verified
  - force=true bypasses matrix but NOT the probe (TestPatchAgentStatus_Force_DoesNotBypassBreakReason passes)
- D-66 disambiguate runs inside the tx for consistent snapshot (404 vs 409 distinction)
- D-56: cache.Del fires on both success path and 409 disambiguate path (2× `s.deps.Cache.Del` calls in handler)
- D-84: `slog.WarnContext(..., "state.force.applied", ...)` emitted post-commit
- STATE-08: `state_version` incremented by SQL atomically; TestPatchAgentStatus_StateVersionMonotonic proves 1→2→3→4
- `mappers.go` with `mapAgentState` correctly handles all nullable columns (`EngagedChannel *string`, `BreakReasonID pgtype.UUID{Valid}`, `PostInteractionState *string`, `WrapupUntil pgtype.Timestamptz{Valid}`)
- Testutil mirrors catalog/testutil_test.go shape per D-73; `stateOnlyHandlers` is a TEMP Wave 2 composite replaced by Wave 4 ApiHandlers
- `server.go` exhaustiveness test confirmed passing (PatchAgentStatus409JSONResponse already in type switch)

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Testutil scaffolding + mappers | 575b72a | main_test.go, testutil_test.go, mappers.go |
| 2+3 | GetAgentStatus + PatchAgentStatus real bodies | be7b2d3 | agent_states.go |
| 4 | 13 tests GREEN | 0cf9956 | agent_states_test.go |
| 5 | server.go type switch check | (no-op — already covered) | server.go unchanged |

## Test Output

```
ok  github.com/luongdev/open-routing/services/api/internal/state   (16 tests pass)
ok  github.com/luongdev/open-routing/services/api/internal/server  (118 tests pass)
134 total passing
```

## Probe-Then-Matrix Ordering Verification

`GetBreakReasonForState` call at line 120, `validateTransition` call at line 142 in `agent_states.go`.

The Pitfall 3 regression gate `TestPatchAgentStatus_Force_DoesNotBypassBreakReason` seeds a break_reason in `otherOrgID`, then PATCHes with `force=true` from the agent's org — verifies 422 invalid_reference is returned even with force.

## Cache Invalidation Verification

`TestPatchAgentStatus_CacheInvalidation`:
1. GET → populates miniredis key
2. `th.Miniredis.Exists(cacheKey)` asserts key present
3. PATCH → triggers cache.Del
4. `th.Miniredis.Exists(cacheKey)` asserts key absent (synchronous del)
5. Second GET returns new status (cache miss → DB)

## wave_1_stub Verification

```
grep -c "wave_1_stub" services/api/internal/state/agent_states.go → 0
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Functionality] stateOnlyHandlers requires explicit panic stubs**

- **Found during:** Task 1 — `api.Unimplemented` satisfies non-strict `ServerInterface`, not `StrictServerInterface`
- **Fix:** Declared explicit panic-body methods for all 35 non-state StrictServerInterface methods on `stateOnlyHandlers`
- **Files modified:** services/api/internal/state/testutil_test.go
- **Impact:** Compile-time verified; panics would only fire if tests accidentally route non-state requests

**2. [Rule 1 - Bug] seedAgentStateRow needs ON CONFLICT DO UPDATE**

- **Found during:** Task 1 — `seedAgent` inserts an `agent_states` row with `status=Offline`; tests that need a different initial status must update it
- **Fix:** `seedAgentStateRow` uses `INSERT ... ON CONFLICT DO UPDATE` to override the Offline row without requiring teardown between seed calls
- **Files modified:** services/api/internal/state/testutil_test.go

### Known No-ops

**Task 5 — server.go type switch:** `PatchAgentStatus409JSONResponse` was already in the type switch at server.go line 494 (added during Wave 1 codegen regen). No change required.

## Cross-AI Peer Review

**Codex:** FAILED — context window exhausted on 15k-line cumulative diff. No verdict produced.

**Gemini:** FAILED — CLI error (filesystem path resolution in tool call). No verdict produced.

**Mitigation:** Manual self-review applied:
- Probe-then-matrix ordering verified by line number assertion (120 < 142)
- T-04-06 (cross-org probe leak) mitigated by code inspection + test passing
- T-04-07 (force unaudited) mitigated — WARN emitted post-commit, verified in handler body
- T-04-08 (state_version regression) mitigated — SQL atomically increments; TestStateVersionMonotonic proves 1→2→3→4
- T-04-09 (cache.Del 5xx cascade) mitigated — Del failure logs WARN; response stays 200

For Wave 3+ reviews: use smaller per-plan diffs (not cumulative from main).

## Threat Surface Scan

No new network endpoints introduced (the 2 state endpoints were already in the spec from Wave 0). No new auth paths. No new schema changes. Wave 2 implements bodies for existing endpoint stubs — no new threat surface beyond the STRIDE register entries from the plan's `<threat_model>`.

## Known Stubs

None — all 5 plan artifacts deliver real implementations or verified test coverage. The `stateOnlyHandlers` TEMP composite is test-only (excluded from production builds by `_test.go` suffix).

## Self-Check: PASSED

All 5 created files verified present. All 3 task commits verified in git log. `go build ./...` clean. `go test` 134 passing.
