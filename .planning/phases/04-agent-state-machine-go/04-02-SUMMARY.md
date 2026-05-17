---
phase: 04-agent-state-machine-go
plan: 02
subsystem: api
tags: [sqlc, domain, state-machine, transition-matrix, tdd, go]

requires:
  - phase: 04-agent-state-machine-go
    plan: 01
    provides: "agent_states migration + tenantTables allowlist + clockwork pin + codegen regen"

provides:
  - "6 sqlc queries for agent_states (Insert/Get/UpdateStatus/ForceUpdate/ListExpiring/ExpireWrapUp)"
  - "GetBreakReasonForState combined probe on break_reasons (D-76, STATE-04+STATE-10)"
  - "internal/domain/state.go: IsRoutable pure function with 10 test cases (STATE-10, D-90)"
  - "internal/state/transitions.go: 7-edge matrix + validateTransition + ErrInvalidTransition (STATE-02/03/05)"
  - "internal/state/handlers.go: state.Server + Deps + New + WithClock + WithSweepInterval + lifecycle stubs"
  - "internal/state/agent_states.go: stub GetAgentStatus + PatchAgentStatus returning 500 with wave_1_stub guards"

affects: [04-03, 04-04, 04-05]

tech-stack:
  added: []
  patterns:
    - "TDD RED→GREEN on domain and state packages (test before implementation)"
    - "CASE expression for case-casing translation in SQL (not COALESCE — avoids CHECK violation 23514)"
    - "AgentStatus = api.AgentStatus type alias in state package (single source of truth, no fork)"
    - "state.Server (not Handlers) from wave 1 — eliminates Pitfall 1 rename in Wave 4"

key-files:
  created:
    - "services/api/internal/db/queries/agent_states.sql (87 lines — 6 named queries)"
    - "services/api/internal/db/generated/agent_states.sql.go (279 lines — sqlc generated)"
    - "services/api/internal/domain/state.go (26 lines — IsRoutable pure function)"
    - "services/api/internal/domain/state_test.go (31 lines — 10 table-driven subtests)"
    - "services/api/internal/state/transitions.go (82 lines — matrix + validator)"
    - "services/api/internal/state/transitions_test.go (61 lines — 36-pair exhaustive walk)"
    - "services/api/internal/state/handlers.go (125 lines — Server struct + New + lifecycle)"
    - "services/api/internal/state/agent_states.go (35 lines — stub handler methods)"
  modified:
    - "services/api/internal/db/queries/break_reasons.sql (+14 lines — GetBreakReasonForState appended)"
    - "services/api/internal/db/generated/break_reasons.sql.go (+27 lines — GetBreakReasonForState binding)"

decisions:
  - "ExpireWrapUp uses CASE translation (not COALESCE) — post_interaction_state lowercase, status PascalCase; COALESCE would return the raw lowercase value and violate CHECK constraint 23514 (Gemini-HIGH-1 from Wave 0 review, re-verified here)"
  - "AgentStatus = api.AgentStatus type alias — transitions.go is single source of truth; no parallel type fork"
  - "state.Server (not Handlers) — Go rejects *catalog.Handlers + *state.Handlers embedded in Wave 4 composite (duplicate field name); naming Server here means Wave 4 has zero rename work"
  - "WrapUp→Ready/NotReady kept in matrix — agent-initiated early WrapUp exit is allowed per D-83; TTL sweeper fires via separate ExpireWrapUp SQL path; both coexist (Codex MED concern resolved by D-83)"
  - "Migration 000002 mutation is locked per D-61 — single editable migration for v0.1; db:reset is the deployment model; no new migration file"

metrics:
  duration: ~35min
  completed: 2026-05-17
---

# Phase 4 Plan 02: Agent State Machine Wave 1 Summary

**sqlc queries + IsRoutable domain pkg + state.Server skeleton + 7-edge transition matrix; all tests green; codegen idempotent**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-05-17
- **Tasks:** 6 (2 auto + 4 auto with TDD RED→GREEN on Tasks 3 + 4)
- **Files created:** 8 new source files
- **Files modified:** 2 existing sqlc query files
- **Test count:** 72 passing (5 packages)

## Accomplishments

- 6 sqlc queries for `agent_states` authored; `task gen` clean; SQLChecker passes (Pitfall 4 was pre-mitigated by Wave 0 tenantTables allowlist)
- `GetBreakReasonForState` combined probe appended to `break_reasons.sql`; returns `bool` scalar for D-76 pattern (one query covers STATE-04 existence check AND STATE-10 routable scalar)
- `domain.IsRoutable` pure function with 10 table-driven tests: Ready=true, Break+routable=true, Break+!routable=false, all others false (STATE-10)
- Transition matrix: exactly 7 agent-initiated edges; 36-pair exhaustive walk passes; ErrInvalidTransition properly wrapped with `%w` for `errors.Is`
- `state.Server` type (not Handlers) from wave 1 commit — Pitfall 1 eliminated for Wave 4
- Stub `GetAgentStatus` + `PatchAgentStatus` on `*Server`; `wave_1_stub` reason strings in place as Wave 2 replacement guards

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | agent_states.sql + task gen | 7ae3701 | agent_states.sql, agent_states.sql.go |
| 2 | GetBreakReasonForState probe | f6b0abf | break_reasons.sql, break_reasons.sql.go |
| 3 (RED) | IsRoutable failing test | f854206 | domain/state_test.go |
| 3 (GREEN) | IsRoutable implementation | a743f37 | domain/state.go |
| 4 (RED) | Transition matrix failing tests | 2609d1e | state/transitions_test.go |
| 4 (GREEN) | Transition matrix implementation | 01ba9a0 | state/transitions.go |
| 5 | state.Server skeleton | bbb7301 | state/handlers.go |
| 6 | Stub handler methods | 9e61f72 | state/agent_states.go |
| fix | Codex LOW: align stub comments 500 not 501 | a2c0e54 | state/agent_states.go |

## Test Output

```
ok  github.com/luongdev/open-routing/services/api/internal/domain   (11 tests pass)
ok  github.com/luongdev/open-routing/services/api/internal/state    (3 tests pass)
ok  github.com/luongdev/open-routing/services/api/internal/db       (44 tests pass)
ok  ...internal/db/orgkey                                            (pass)
ok  ...internal/db/orgkey                                            (pass)
72 total passing
```

## Codegen Idempotency

```
task gen && git diff --exit-code → exit code: 0 (clean)
```

## Matrix Verification

- Outer keys: {NotReady, Ready, Break, WrapUp} = 4 states
- Total edges: 7 agent-initiated
- Missing as outer keys: Engaged, Offline (system-only per D-82)
- 36-pair walk (6×6): 7 allowed, 29 rejected with ErrInvalidTransition
- WrapUp→Ready + WrapUp→NotReady: in matrix (agent early exit per D-83); TTL path via ExpireWrapUp SQL is separate

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Stub comments said "501 stub" but code returns 500 — Codex LOW finding**
- **Found during:** Cross-AI peer review (CLAUDE.md HARD RULE)
- **Issue:** Comments in agent_states.go said "501 stub" while code correctly returned 500 responses (no 501 variant in spec, per existing notimpl.go rationale)
- **Fix:** Updated comments to say "500 stub" with explicit note referencing notimpl.go
- **Files modified:** services/api/internal/state/agent_states.go
- **Commit:** a2c0e54

### Codex BLOCK Concerns Resolved

**Codex HIGH: Migration 000002 mutation** — Pre-addressed by locked decision D-61 (single editable migration for v0.1; db:reset is the deployment model; all environments are development). This is a documentation concern for future v1 production deployment, not a bug in the current implementation. The 04-01-SUMMARY confirms `task db:reset` applies cleanly.

**Codex MED: WrapUp→Ready/NotReady in matrix contradicts "system-only TTL"** — Not a contradiction. D-83 explicitly allows agents to PATCH back to Ready/NotReady during WrapUp (early agent exit). The TTL sweeper fires via a separate code path (ExpireWrapUp SQL UPDATE) when `wrapup_until < NOW()`. Both paths coexist; the matrix correctly includes WrapUp→Ready/NotReady as agent-initiated.

## Cross-AI Peer Review Results

**Gemini verdict:** READY — No HIGH concerns. LOW notes on ListExpiringWrapUps bypass (intentional per OrgDB.WithBypass design) and stub handlers (intentional wave guards).

**Codex verdict:** BLOCK — HIGH on migration mutation (D-61 locked decision, not actionable here); MED on WrapUp matrix edges (D-83 correct behavior); LOW on 501/500 comment (auto-fixed in a2c0e54).

**Net verdict after analysis:** READY — both BLOCK reasons trace to locked architectural decisions; LOW concern auto-fixed.

## Threat Surface Scan

No new network endpoints introduced. No new auth paths. The `ListExpiringWrapUps` query intentionally omits org_id in WHERE (sweeper processes all orgs via OrgDB.WithBypass) but includes org_id in SELECT for cache invalidation — this is the documented D-81 sweeper pattern, within scope of the T-04-04 threat register entry.

## Self-Check: PASSED

All 8 created files verified present. All 7 task commits verified in git log. `go build ./...` clean.
