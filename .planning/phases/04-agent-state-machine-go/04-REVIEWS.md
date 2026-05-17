# Phase 4 — Cross-AI Plan Review

**Date:** 2026-05-17
**Reviewed:** PATTERNS.md + 6 PLAN.md files (5,546 lines)
**Reviewers:** Codex (codex-cli 0.130.0) + Gemini (0.41.2) in parallel per `~/.claude/CLAUDE.md` HARD RULE
**Final verdict:** READY WITH FIXES — 4 HIGH findings applied inline; MED findings carried forward as Claude's discretion for executor

---

## Codex Review (codex exec --skip-git-repo-check)

**Verdict:** READY WITH FIXES

### Concerns

| Severity | Concern | Status |
|----------|---------|--------|
| HIGH | Wave 3 (04-04) is not parallel-safe with Wave 2 — TTL tests depend on testutil_test.go from 04-03 | **FIXED** — `depends_on: [04-02]` → `[04-03]` |
| HIGH | `ttl.go` jitter() uses `int64(uuid.New().Time())` — unsafe API + unused dead code | **FIXED** — function removed entirely (D-87 deferred to v0.2) |
| MED | `TestAcceptance_IsRoutableMatrix` is a logging wrapper, not a real assertion | DEFERRED to executor (Wave 5 cross-AI gate catches if test stays empty) |
| MED | Comment hygiene weak in some plan snippets (WHAT/HOW vs WHY) | DEFERRED to executor — code-reviewer agent pass enforces |
| MED | `UpdateAgentStateStatus` NULL clearing semantics not specified per target status | DEFERRED to executor — sqlc.narg() pattern is sparse PATCH (carries Phase 3 D-66 semantic) |
| LOW | Wave 0 clockwork human checkpoint is unnecessary friction | KEPT — planner-chosen safety gate, executor can downgrade if desired |
| LOW | Cross-AI invocation says "Codex + Gemini parallel" — user rule says Codex→Gemini chain | KEPT — `~/.claude/CLAUDE.md` says Claude→Codex+Gemini parallel (Codex misread `./AGENTS.md`) |

### Strengths (Codex)
- All 5 critical pitfalls captured (P1, P3, P6, P8, P10)
- D-94 cross-org probes named explicitly including force=true probe
- Cache strategy correct: singular agent_state key, DEL after write, sweeper invalidates with returned org_id
- D-93 well-covered: CreateAgent seeds agent_states inside qtx
- Spec amendment scoped to request body only; codegen drift gate included

---

## Gemini Review (gemini -p)

**Verdict:** READY WITH FIXES

### Concerns

| Severity | Concern | Status |
|----------|---------|--------|
| HIGH | PostInteractionState lowercase (`ready`/`not_ready`) vs status PascalCase (`Ready`/`NotReady`) — `ExpireWrapUp` SQL `COALESCE(post_interaction_state, 'NotReady')` triggers CHECK violation on every WrapUp expiry | **FIXED** — `CASE post_interaction_state WHEN 'ready' THEN 'Ready' WHEN 'not_ready' THEN 'NotReady' END` translation |
| HIGH | Timer registry corruption — AfterFunc closure deletes by key without pointer-equality check. Successor schedule (rapid state changes, v0.2 extensions) causes old AfterFunc body to delete the new timer's handle → Stop() leaks goroutines | **FIXED** — `var t clockwork.Timer; t = AfterFunc(...);` capture + `if cur == t { delete(...) }` |
| MED | `jitter()` calls `uuid.New().Time()` — v4 UUIDs PANIC on .Time() | **FIXED** — function removed |
| MED | Dead code: `jitter()` + `_ = pgtype.Text` guard unreachable in v0.1 | **FIXED** — both removed; pgtype import dropped |

### Strengths (Gemini)
- Pitfall 1 rename to state.Server proactively avoids embed collision
- Probe-then-Matrix ordering (Pitfall 3) ensures force=true cannot leak cross-org break_reason
- Clockwork integration (D-81) allows deterministic AfterFunc testing
- SQLChecker lifecycle: Wave 0 registers `agent_states` allowlist; sweeper bypass mentioned org_id
- Every ROADMAP success criterion maps to a named acceptance test

---

## Synthesis: 4 HIGH Fixes Applied Inline

### Fix 1 — Wave 3 dependency (Codex HIGH 1)
- **File:** [.planning/phases/04-agent-state-machine-go/04-04-PLAN.md](.planning/phases/04-agent-state-machine-go/04-04-PLAN.md:6)
- **Change:** `depends_on: [04-02]` → `depends_on: [04-03]`
- **Reason:** TTL tests need shared fixtures (testutil_test.go) created in Wave 2 (04-03), not Wave 1 (04-02). The "parallel with Wave 2" claim was incorrect.

### Fix 2 — ExpireWrapUp case translation (Gemini HIGH 1)
- **File:** [.planning/phases/04-agent-state-machine-go/04-02-PLAN.md](.planning/phases/04-agent-state-machine-go/04-02-PLAN.md:237)
- **Change:** `SET status = COALESCE(post_interaction_state, 'NotReady')` → `SET status = CASE post_interaction_state WHEN 'ready' THEN 'Ready' WHEN 'not_ready' THEN 'NotReady' ELSE 'NotReady' END`
- **Reason:** OpenAPI `PostInteractionState` enum is lowercase (line 1007-1009: `ready`, `not_ready`) but `AgentStatus` is PascalCase (line 995-1000: `Ready`, `NotReady`). The original COALESCE would fail the status CHECK constraint (23514) on every WrapUp expiry, breaking ROADMAP acceptance test #3.
- **Acceptance criteria added** to verify the translation is present + the broken COALESCE path is gone.

### Fix 3 — Timer registry pointer-equality (Gemini HIGH 2)
- **File:** [.planning/phases/04-agent-state-machine-go/04-04-PLAN.md](.planning/phases/04-agent-state-machine-go/04-04-PLAN.md:222)
- **Change:** Capture timer handle before AfterFunc install; delete only if pointer matches.
  ```diff
  -    s.timers.t[agentID] = s.clock.AfterFunc(remaining, func() {
  -        ...
  -        delete(s.timers.t, agentID)
  +    var t clockwork.Timer
  +    t = s.clock.AfterFunc(remaining, func() {
  +        ...
  +        if cur, ok := s.timers.t[agentID]; ok && cur == t {
  +            delete(s.timers.t, agentID)
  +        }
  +    })
  +    s.timers.t[agentID] = t
  ```
- **Reason:** A successor schedule (rapid state changes, v0.2 WrapUp extension, re-seed after restart) replaces the timer; without pointer-equality check the old AfterFunc body would erase the successor's handle on fire, leaving Stop() unable to cancel it. Race condition leaks goroutines after Stop.
- **Acceptance criteria added** to grep for `var t clockwork.Timer` and `cur == t`.

### Fix 4 — Remove jitter() + pgtype guard (Codex HIGH 2 + Gemini MED)
- **File:** [.planning/phases/04-agent-state-machine-go/04-04-PLAN.md](.planning/phases/04-agent-state-machine-go/04-04-PLAN.md:356)
- **Change:** Deleted `jitter()` function (40 LOC), `jitterMaxMs` const, `var _ pgtype.Text` guard, and `github.com/jackc/pgx/v5/pgtype` import; replaced with WHY-comment explaining v0.2 deferral.
- **Reason:** Gemini caught a panic risk — `uuid.New()` returns v4 UUIDs which PANIC on `.Time()` (only v1/v2/v6/v7 have time components). Codex caught the dead-code aspect. Both pointed to D-82 deferring system-initiated WrapUp transitions to v0.2 — no v0.1 caller exists for jitter, so removing it cleanly is the right answer.
- **Acceptance criteria added** to negate-grep for `jitter`, `uuid.New().Time()`, `pgtype.Text`, and pgtype import.

---

## MED Findings Carried Forward (executor's discretion or Wave 5 cross-AI gate catches)

1. **IsRoutable acceptance test** — currently a logging wrapper. Executor should make Wave 5 task 2's `TestAcceptance_IsRoutableMatrix` call `domain.IsRoutable` directly for ROADMAP's 4 cases, OR rely on Wave 1's `domain/state_test.go::TestIsRoutable` as the acceptance.
2. **Comment hygiene** — Some plan snippets include WHAT/HOW comments. Executor must apply WHY-not-WHAT rule (project CLAUDE.md) when transcribing.
3. **UpdateAgentStateStatus NULL semantics per target status** — Gemini-MED concern. sqlc.narg() pattern produces NULL on omission, which is intended for Phase 3 sparse PATCH. Executor should add tests in Wave 2 (04-03) verifying:
   - `Ready → Break` sets `break_reason_id`
   - `Break → Ready` should clear `break_reason_id`? — open question, executor decides per UX: leaving stale break_reason_id might be acceptable for audit; executor's call.

## LOW Findings Deferred

1. **Wave 0 clockwork human checkpoint** — kept. Executor can downgrade `checkpoint:human-verify` to `auto` if confident in clockwork supply-chain.
2. **Cross-AI invocation chain interpretation** — kept. User's `~/.claude/CLAUDE.md` explicitly says "Claude → Codex + Gemini in parallel", which is what Wave 5 task 5 specifies.

---

## Files Modified by Cross-AI Fixes

- [04-02-PLAN.md](.planning/phases/04-agent-state-machine-go/04-02-PLAN.md) — ExpireWrapUp CASE translation + 3 new acceptance criteria
- [04-04-PLAN.md](.planning/phases/04-agent-state-machine-go/04-04-PLAN.md) — depends_on bump + scheduleWrapUpExpiry pointer-equality + jitter/pgtype removal + 8 new acceptance criteria + import list trim + hazards section updated

## Final Verdict (after fixes)

**READY for `/gsd-execute-phase 4`.** All HIGH findings resolved inline. MED + LOW findings either deferred to executor (with rationale captured here) or accepted as planner-chosen safety gates. Wave 5 cross-AI peer review (per CLAUDE.md HARD RULE) will catch any remaining issues against the actual code diff.

---

*Reviews date: 2026-05-17*
*Reviewer count: 2 (Codex + Gemini parallel per HARD RULE)*
*HIGH findings: 4 — all applied inline*
*MED findings: 5 — 3 carried forward to executor, 2 covered by fixes*
*LOW findings: 2 — kept as planner-chosen behavior*
