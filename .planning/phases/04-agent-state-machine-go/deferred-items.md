# Phase 04 Deferred Items

Discovered during 04-05 Wave 4 peer review. These issues exist in Wave 2-3 code
(state/agent_states.go, state/ttl.go) — out of scope for 04-05 which only touched
catalog/agents.go, catalog/notimpl.go, catalog/handlers.go, cmd/api/main.go,
and test files.

Wave 5 (04-06 isolation + acceptance tests) is the appropriate landing zone for these fixes.

---

## CODEX HIGH: PATCH cross-field state invariant violation

**File:** services/api/internal/state/agent_states.go (lines 112, 240)
**Issue:** `buildUpdateParams` writes any supplied `break_reason_id` and
`post_interaction_state` on ANY status transition. The `break_reason_id` probe
only runs when `to == Break`, so a client can send `break_reason_id` with
`to=Ready` and it persists in the DB, violating OpenAPI semantics.
**Required fix:** Only accept/probe/store `break_reason_id` for `to == Break`;
clear it on exit from Break. Only accept/store `post_interaction_state` while
current status is `Engaged`; clear incompatible nullable columns on status changes.

**STATUS: FIXED — commit 2aaee09**
- buildUpdateParams: break_reason_id only written when body.To==AgentStatusBreak
- buildForceUpdateParams: same invariant, signature extended with currentStatus param
- Tests: TestPatchAgentStatus_BreakReasonIgnoredOnNonBreak,
  TestPatchAgentStatus_PostInteractionStateClearedOnExitEngaged

## CODEX MED: Enum validation missing in PatchAgentStatus

**File:** services/api/internal/state/agent_states.go (line 84)
**Issue:** `req.Body.To` and `PostInteractionState` are never validated with
`.Valid()`. Non-force invalid `to` surfaces as a 409 (transition matrix
mismatch); force=true with an invalid `to` reaches Postgres CHECK constraint
and returns 500 instead of 422.
**Required fix:** Call `req.Body.To.Valid()` (and `PostInteractionState.Valid()`)
before DB work; return 422 `invalid_value` if false.

**STATUS: FIXED — commit 2aaee09**
- Added req.Body.To.Valid() + req.Body.PostInteractionState.Valid() checks
  before GetAgentStateByAgentId call
- Test: TestPatchAgentStatus_InvalidEnum_422

## CODEX MED: AfterFunc goroutines not tracked in WaitGroup

**File:** services/api/internal/state/ttl.go (lines 43, 55) + handlers.go (line 125)
**Issue:** Timer callback goroutines that pass the pre-DB `s.ctx.Done()` check
before shutdown then use `context.Background()` internally and are NOT added to
`s.wg`. `Stop()` can return while in-flight DB UPDATEs (ExpireWrapUp calls) are
still running, leaving connections in-flight against a closing pool.
**Required fix:** Add timer body to `s.wg` before dispatching; or use
`context.WithTimeout(s.ctx, gracePeriod)` consistently.

**STATUS: FIXED — commit 2aaee09**
- s.wg.Add(1) / defer s.wg.Done() added inside AfterFunc body
- WHY comment documents the shutdown-race trade-off (Add inside func, not at schedule time)

## GEMINI HIGH: Soft-deleted agents have status read/modified

**File:** services/api/internal/db/generated/agent_states.sql.go (GetAgentState query)
**Issue:** `GetAgentState` queries `agent_states` by `agent_id + org_id` with no
join to `agents.enabled`. A soft-deleted agent (enabled=false) still returns 200
from GetAgentStatus and accepts PatchAgentStatus transitions.
**Required fix:** Either JOIN agents in the SQL query with `AND agents.enabled = true`,
or add an application-level soft-delete check after fetching the agent_state row.

**STATUS: FIXED — commit 2aaee09**
- Application-level: agentEnabledCheck() helper added using existing GetAgent query
- Both GetAgentStatus and PatchAgentStatus call agentEnabledCheck before any state ops
- Returns 404 for enabled=false, preserving FOUND-08 surface
- Tests: TestGetAgentStatus_SoftDeletedAgent_Returns404,
  TestPatchAgentStatus_SoftDeletedAgent_Returns404
