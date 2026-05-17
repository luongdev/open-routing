# Phase 4 Final Cross-AI Review — Gemini

## Review Method
Multiple rounds due to Gemini CLI tool-use behavior on diff filenames.
Round 1: Full 17K-line diff → Gemini attempted filesystem operations on embedded paths; partial response with HIGH/MED findings. Used findings to drive fixes.
Round 2: Focused state-package diff (3354 lines) → Same tool-use failure.
Round 3: Text description of fix patterns → SUCCESS.

## Gemini Round 1 (Full diff — partial response before tool-use failure)

### Summary
Phase 4 is a comprehensive and technically robust implementation of the agent state machine. The separation of pure logic into internal/domain and lifecycle management into state.Server is excellent.

### Strengths
- Naming Discipline (Pitfall 1): Proactively renaming the state type to Server avoids anonymous embedding collisions in ApiHandlers.
- Atomic Seeding (D-93): Atomic agent_states insertion during CreateAgent ensures no "half-born" agents exist in the system.
- Probe Ordering (Pitfall 3): Correctly running the break_reason probe before the matrix check ensures force=true cannot bypass org-isolation.
- Lifecycle Management (D-95): Synchronous startup sweep and graceful WaitGroup drain in Stop() prevent data races and stuck states.
- TDD Rigor: High test coverage across domain, state, and isolation packages (including the Pitfall 3 regression gate).

### Concerns [HIGH/MED/LOW]

**[HIGH] Status Invariant Violation — engaged_channel Wiped on Engaged → Engaged**
In PatchAgentStatus, buildUpdateParams does not assign a value to EngagedChannel, and the SQL uses `engaged_channel = sqlc.narg('engaged_channel')::text`. Since the field is nil in Go, it resolves to NULL in SQL, wiping the agent's active channel while they remain in the Engaged state.
Impact: Violates STATE-05 (Engaged must have a channel).

**[MED] Column Leak — post_interaction_state not cleared on exit**
UpdateAgentStateStatus uses COALESCE for this column. If an agent transitions Engaged (PIS=ready) → Ready, the handler passes nil, and COALESCE preserves the now-stale ready value. Also ExpireWrapUp clears wrapup_until and engaged_channel but leaves post_interaction_state populated.
Impact: Violates STATE-01 schema invariants.

**[MED] SQL Injection Hazard in Name Search**
ListAgents uses LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%'. While sqlc uses prepared statements, this is pre-existing pattern not introduced in Phase 4. Out of scope for Phase 4.

**[LOW] D-78 Rationale Discrepancy**
D-78 claims the sweeper relies on org_id in WHERE clauses for auditability. ListExpiringWrapUps omits it (intentional cross-org scan). Minor doc-to-code mismatch.

### Verdict: READY WITH FIXES
Fixes applied: HIGH → SQL uses COALESCE for engaged_channel; MED1 → SQL uses explicit assignment for post_interaction_state (not COALESCE) enabling Go handler to clear it.

## Gemini Round 2 (Text description of fix patterns)

### Concerns: None (all LOW or none)

**Stop() deadlock-free**: The Stop() implementation correctly manages the WaitGroup and timer cancellation. By calling t.Stop() and compensating with wg.Done() only when true (timer was stopped), it avoids hangs. The WaitGroup count remains consistent because scheduleWrapUpExpiry calls wg.Add(1) BEFORE scheduling the AfterFunc. Releasing startMu before wg.Wait() and the bounded 10s timeout on DB calls further ensure a clean shutdown without deadlocks.

**PIS Cross-field Invariant**: buildUpdateParams and buildForceUpdateParams correctly restrict PIS updates to the Engaged state. Specifically, PIS is only passed to the DB layer when current == Engaged AND target ∈ {Engaged, WrapUp}, matching the STATE-06 requirement. In all other transitions, it is passed as nil, which clears the DB column (explicitly assigned in SQL, not COALESCEd).

**engaged_channel COALESCE**: Using COALESCE in the UPDATE SQL is correct for v0.1. Since the current PATCH handler does not pass an engaged_channel value (system-set only), COALESCE(NULL, existing) prevents the existing channel from being wiped. The ExpireWrapUp SQL correctly handles final cleanup by explicitly setting engaged_channel = NULL.

**agentEnabledCheck (404)**: The check correctly treats enabled=false (soft-deleted) agents as non-existent for state operations, satisfying FOUND-08 requirement.

### Verdict: READY
