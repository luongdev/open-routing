---
phase: "04"
plan: "06"
subsystem: agent-state-machine
tags: [state-machine, tdd, cross-ai-review, test-isolation, wave-5-close]
dependency_graph:
  requires: [04-01, 04-02, 04-03, 04-04, 04-05]
  provides: [phase-4-complete, state-machine-verified]
  affects: [services/api/internal/state, services/api/test/isolation]
tech_stack:
  added: []
  patterns:
    - per-org DELETE replaces TRUNCATE for parallel test isolation
    - wg.Add(1) BEFORE AfterFunc + wg.Done() compensation on timer cancel
    - cross-field state invariant gated at both Go and SQL layer
    - typed constant assertion (api.InvalidTransition vs api.ErrorCodeInvalidTransition)
key_files:
  created:
    - services/api/test/isolation/state_test.go
    - .planning/phases/04-agent-state-machine-go/reviews/phase4-final-codex.md
    - .planning/phases/04-agent-state-machine-go/reviews/phase4-final-gemini.md
  modified:
    - services/api/internal/state/agent_states.go
    - services/api/internal/state/agent_states_test.go
    - services/api/internal/state/handlers.go
    - services/api/internal/state/ttl.go
    - services/api/internal/state/testutil_test.go
    - services/api/internal/state/ttl_test.go
    - services/api/internal/db/queries/agent_states.sql
    - services/api/internal/db/generated/agent_states.sql.go
    - .planning/phases/04-agent-state-machine-go/04-PATTERNS.md
    - .planning/phases/04-agent-state-machine-go/deferred-items.md
decisions:
  - "agentEnabledCheck uses GetAgent query at app layer; TOCTOU race deferred to Phase 5 AUTH phase"
  - "COALESCE for engaged_channel (preserves system-set value); explicit assignment for post_interaction_state (allows Go nil to clear stale DB value)"
  - "wg.Add(1) placed BEFORE AfterFunc creation per Codex HIGH; Stop() drains timers before wg.Wait()"
  - "cleanStateTables changed to per-org DELETE so parallel tests isolate by orgID"
metrics:
  duration: "~3 hours (including 3-round peer review)"
  completed_date: "2026-05-17"
  task_count: 9
  file_count: 13
---

# Phase 4 Plan 6: Wave 5 Close — Cross-AI Review + Acceptance Tests Summary

Wave 5 closes Phase 4 by addressing four deferred peer-review findings (Part A), adding the planned cross-org isolation and acceptance test suites (Part B), and completing the cross-AI review mandated by CLAUDE.md.

## What Was Built

**Part A — Deferred HIGH/MED Fixes (from Wave 4 peer review):**

1. `agentEnabledCheck` helper — soft-deleted agents (enabled=false) return 404 from both GetAgentStatus and PatchAgentStatus. Uses GetAgent query at app layer; TOCTOU race documented as v0.1 known limitation.

2. Cross-field state invariant — `buildUpdateParams` and `buildForceUpdateParams` now gate `break_reason_id` (only persisted when `to==Break`) and `post_interaction_state` (only accepted when `currentStatus==Engaged` AND `target ∈ {Engaged, WrapUp}`). Both SQL and Go layers enforce this.

3. Enum validation before DB work — unknown `to` or `post_interaction_state` values return 422 at the Go handler layer before any Postgres CHECK constraint fires.

4. Stop() deadlock fix — timer-drain loop with `t.Stop()==true → wg.Done()` compensation moved BEFORE `wg.Wait()`. `wg.Add(1)` placed BEFORE AfterFunc creation to close the Stop-concurrent-with-Add window.

**Part B — Wave 5 Planned Tests:**

5. Cross-org isolation suite (`test/isolation/state_test.go`) — 4 tests + D-93 seeding regression gate:
   - `TestState_GetStatusCrossOrg_404` — cross-org GET returns 404
   - `TestState_PatchStatusCrossOrgAgent_404` — cross-org PATCH returns 404
   - `TestState_PatchStatusCrossOrgBreakReason_422` — cross-org break_reason rejected
   - `TestState_ForceDoesNotBypassCrossOrgBreakReason_422` — force=true still rejects cross-org
   - `TestCreateAgentSeedsState` — D-93 regression gate

6. Acceptance tests (`internal/state/agent_states_test.go`) — 5 new tests:
   - `TestAcceptance_AllTransitions` — 7 allowed + 3 rejected transitions
   - `TestAcceptance_BreakReasonValidation` — valid same-org break_reason recorded
   - `TestAcceptance_WrapUpExpiresWithoutClient` — real clock, 1s TTL, 8s timeout
   - `TestAcceptance_IsRoutableMatrix` — traceability wrapper for STATE-10
   - `TestAcceptance_StateVersionMonotonic` — 5 sequential transitions verify version 1→2→3→4→5→6

7. Phase 5 Inheritance Reminder appended to `04-PATTERNS.md`.

**Rule 1 fixes discovered during execution:**

- `TestAcceptance_AllTransitions` type mismatch: used `api.ErrorCodeInvalidTransition` (type `ErrorCode`) where `err409.Error` is type `InvalidTransitionErrorResponseError`; corrected to `api.InvalidTransition`.
- Pre-existing test isolation race: `cleanStateTables` used `TRUNCATE` on shared pool while all tests run with `t.Parallel()`. Changed to per-org `DELETE WHERE org_id = $1` so parallel tests are isolated by unique orgID. This resolved 18 pre-existing failures; 42 state tests now pass.

## Test Counts

| Suite | Tests |
|-------|-------|
| `internal/state` | 42 |
| `internal/domain` | 11 |
| `test/isolation` | 24 |
| **Full suite** | **404 total across 16 packages** |

All 404 tests pass without `-short` (live DB).
All 229 tests pass with `-short` (confirmed before wave).

## Cross-AI Peer Review

**Codex (3 rounds):**
- Round 1: Context window exceeded on 17K-line diff (no findings)
- Round 2: HIGH — wg.Add(1) inside AfterFunc creates Stop-concurrent-with-Add race; MED — PIS gate incomplete for Engaged→non-WrapUp transitions; MED — TOCTOU on soft-delete check (pre-existing v0.1 pattern)
- Round 3 (targeted 169-line fix diff): All 4 LOWs — Stop() ordering correct, break_reason_id invariant correct, PIS invariant consistent, engaged_channel COALESCE correct
- **Final verdict: READY**

**Gemini (2 rounds):**
- Round 1: HIGH — engaged_channel wiped on Engaged→Engaged PATCH; MED — post_interaction_state not cleared on Engaged→Ready; LOW — D-78 rationale discrepancy (doc-only)
- Round 2 (text description): Stop() deadlock-free, PIS invariant correct, COALESCE correct, agentEnabledCheck(404) correct
- **Final verdict: READY**

Fixes applied from HIGH/MED findings before final round:
- SQL: `engaged_channel` uses COALESCE; `post_interaction_state` uses explicit assignment
- Go: `buildForceUpdateParams` now accepts `currentStatus` param; PIS cleared when target not Engaged/WrapUp
- handlers.go: timer drain loop before wg.Wait()
- ttl.go: wg.Add(1) before AfterFunc; prior.Stop()==true compensates wg.Done()

## Verification Results

- `go test -count=1 ./...` → 404 passed, 0 failed
- `go vet ./...` → no issues
- `task gen && git diff --exit-code` → clean (no codegen drift)

## STATE-* Requirements Crossed Off

All 10 STATE-* requirements verified complete:

| Requirement | Description | Status |
|-------------|-------------|--------|
| STATE-01 | agent_states schema + valid_status_check constraint | COMPLETE |
| STATE-02 | GetAgentStatus — 200 + cache hit/miss | COMPLETE |
| STATE-03 | PatchAgentStatus — 409 invalid_transition with from+to | COMPLETE |
| STATE-04 | break_reason probe — 422 before matrix, force=true included | COMPLETE |
| STATE-05 | Engaged must have engaged_channel (COALESCE preserves) | COMPLETE |
| STATE-06 | post_interaction_state only accepted Engaged→Engaged|WrapUp | COMPLETE |
| STATE-07 | state_version monotonically increasing on each PATCH | COMPLETE |
| STATE-08 | WrapUp expiry — AfterFunc timer + safety sweep idempotent | COMPLETE |
| STATE-09 | Soft-delete returns 404 for state operations | COMPLETE |
| STATE-10 | IsRoutable matrix — break reason routable flag check | COMPLETE |

## Files Modified Across Wave 5

13 files modified/created in Wave 5:

**Go source (5 files):**
- `services/api/internal/state/agent_states.go` — agentEnabledCheck, enum validation, cross-field invariant fixes
- `services/api/internal/state/handlers.go` — Stop() deadlock fix (timer drain before wg.Wait)
- `services/api/internal/state/ttl.go` — wg.Add(1) before AfterFunc, prior.Stop() compensation

**SQL + generated (2 files):**
- `services/api/internal/db/queries/agent_states.sql` — COALESCE for engaged_channel, explicit assignment for PIS
- `services/api/internal/db/generated/agent_states.sql.go` — regenerated by sqlc generate

**Test files (4 files):**
- `services/api/test/isolation/state_test.go` — new: cross-org isolation + D-93 seeding gate
- `services/api/internal/state/agent_states_test.go` — Part A fix tests + Part B acceptance tests
- `services/api/internal/state/testutil_test.go` — per-org DELETE fix (was TRUNCATE)
- `services/api/internal/state/ttl_test.go` — cleanStateTables signature update

**Docs (4 files):**
- `.planning/phases/04-agent-state-machine-go/04-PATTERNS.md` — Phase 5 Inheritance Reminder
- `.planning/phases/04-agent-state-machine-go/deferred-items.md` — all items marked FIXED
- `.planning/phases/04-agent-state-machine-go/reviews/phase4-final-codex.md` — Codex review transcript
- `.planning/phases/04-agent-state-machine-go/reviews/phase4-final-gemini.md` — Gemini review transcript

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestAcceptance_AllTransitions type mismatch**
- Found during: Part B Task 2 verification
- Issue: `api.ErrorCodeInvalidTransition` (type `ErrorCode`) compared against `err409.Error` (type `InvalidTransitionErrorResponseError`) via `require.Equal` which uses deep equality
- Fix: Changed to `api.InvalidTransition` (correct typed constant)
- Files modified: `internal/state/agent_states_test.go`
- Commit: 7ad7dd2

**2. [Rule 1 - Bug] Pre-existing test isolation race in cleanStateTables**
- Found during: Part B Task 4 full verification
- Issue: All parallel tests called `cleanStateTables` which did `TRUNCATE TABLE agents, agent_states, break_reasons CASCADE` on the shared pool. With `t.Parallel()`, one test's TRUNCATE would delete another test's freshly-seeded rows mid-flight. This caused ~18 flaky failures pre-wave.
- Fix: Changed to `DELETE WHERE org_id = $1` using each test's unique UUIDv7 orgID; updated 31 call sites across 3 files
- Files modified: `testutil_test.go`, `agent_states_test.go`, `ttl_test.go`
- Commit: 7ad7dd2

**3. [Rule 1 - Bugs from Wave 4 peer review] Part A deferred HIGH/MED findings**
- Codex HIGH (wg.Add inside AfterFunc), Gemini HIGH (engaged_channel wiped), Gemini MED (PIS stale on exit Engaged) — all addressed before Wave 5 planned tests
- Commits: 2aaee09, 4bf39a2, 7d8fe85, f4789bf

## Known Stubs

None.

## Threat Flags

None — Wave 5 is test-only plus targeted bug fixes; no new network endpoints, auth paths, or schema changes introduced.

## Wave 5 Commits

| Hash | Message |
|------|---------|
| 2aaee09 | fix(04-06): address deferred HIGH/MED wave 4 peer-review findings |
| 35721c8 | feat(04-06): add D-94 cross-org isolation tests + D-93 seeding regression gate |
| 8b79754 | feat(04-06): add 5 acceptance tests covering ROADMAP Phase 4 success criteria |
| 52b5713 | docs(04-06): append Phase 5 Inheritance Reminder to 04-PATTERNS.md |
| 4bf39a2 | fix(04-06): address Gemini HIGH/MED SQL column invariant findings |
| 7d8fe85 | fix(04-06): address Codex HIGH WaitGroup race + MED PIS invariant findings |
| f4789bf | fix(04-06): fix Stop() deadlock + tighten PIS cross-field invariant |
| a6e8163 | docs(04-06): add cross-AI peer review findings (Codex + Gemini) |
| 7ad7dd2 | fix(04-06): fix test isolation race + type mismatch in acceptance tests |

## Self-Check: PASSED

All created files exist on disk. All commit hashes verified in git log.
Full test suite: 404 passed, 0 failed. go vet: clean. task gen: no drift.
