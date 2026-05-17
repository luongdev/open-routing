# Phase 4 Final Cross-AI Review — Codex

## Review Method
Multiple rounds due to context window constraints on the full 17K-line diff.
Round 1: Full diff → CONTEXT OVERFLOW (no findings produced).
Round 2: State-package diff (3354 lines) → CONTEXT OVERFLOW.
Round 3: Targeted diff (169 lines — only the 5 fix commits) → SUCCESS.

## Codex Round 1 (Full diff)
- **Status:** Context window exceeded. No findings produced.

## Codex Round 2 (State-package diff 3354 lines)

### Concerns [HIGH/MED/LOW]

**[HIGH] Stop() can race with AfterFunc because wg.Add(1) happens inside the timer callback.**
File: services/api/internal/state/ttl.go
Stop() may call wg.Wait() while the waitgroup counter is zero, then a timer callback starts, passes the ctx.Done() check, calls wg.Add(1), and performs DB work after Stop() returned. Track scheduled callbacks differently, or call wg.Add(1) before scheduling and Done() in the callback including early exits.

**[MED] post_interaction_state can persist on non-Engaged targets when forced from Engaged.**
File: services/api/internal/state/agent_states.go
buildForceUpdateParams sets PostInteractionState whenever currentStatus == Engaged, regardless of body.To. A request like Engaged -> NotReady with post_interaction_state=ready leaves PIS on a NotReady row. Gate on currentStatus == Engaged && body.To == Engaged (or WrapUp) or otherwise clear when target is not Engaged/WrapUp.

**[MED] PATCH can update state after an agent is soft-deleted concurrently.**
File: services/api/internal/state/agent_states.go
agentEnabledCheck runs outside the transaction. Pre-existing v0.1 TOCTOU pattern; deferred to Phase 5 / AUTH phase.

**[LOW] Project comment policy.**
Some production comments explain control flow (WHAT) rather than WHY.

### Verdict: READY WITH FIXES
Fixes applied: HIGH → Stop() reordered (timers cancelled BEFORE wg.Wait()); MED1 → PIS invariant tightened in both update/force-update paths.

## Codex Round 3 (Targeted 169-line fix diff)

### Concerns [HIGH/MED/LOW]

**[LOW] Stop() ordering is correct** for the shown timer model: cancel context, stop/delete timers, compensate wg.Done() only when Timer.Stop() returns true, then wg.Wait(). Assumption: no new scheduleWrapUpExpiry calls can start after Stop() begins.

**[LOW] break_reason_id invariant is correct:** only set when to == Break, otherwise cleared.

**[LOW] post_interaction_state invariant applied consistently:** accepted only from Engaged to Engaged|WrapUp, otherwise cleared by explicit SQL assignment.

**[LOW] engaged_channel COALESCE fix is correct** for preserving existing channel when handlers pass nil.

### Verdict: READY
tokens used: 8,013
