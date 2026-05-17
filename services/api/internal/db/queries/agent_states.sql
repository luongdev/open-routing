-- name: InsertAgentState :one
-- D-93: called from catalog.CreateAgent inside the existing OrgTx so
-- agent + agent_state INSERTs commit atomically (Codex C4 pattern).
INSERT INTO agent_states (agent_id, org_id, status, state_version)
VALUES ($1, $2, $3, 1)
RETURNING agent_id, org_id, status, engaged_channel, break_reason_id,
         post_interaction_state, wrapup_until, state_version, updated_at;

-- name: GetAgentStateByAgentId :one
-- Used by GET /agents/{id}/status handler (cache loader) and by the
-- 0-row disambiguate path in PatchAgentStatus (D-66 adapted for D-85).
SELECT agent_id, org_id, status, engaged_channel, break_reason_id,
       post_interaction_state, wrapup_until, state_version, updated_at
FROM agent_states
WHERE agent_id = $1 AND org_id = $2;

-- name: UpdateAgentStateStatus :one
-- D-85: WHERE clause uses `status = expected_from` (matrix-driven gate),
-- NOT `state_version = expected_version`. 0 rows → handler runs
-- GetAgentStateByAgentId to disambiguate 404 vs 409 invalid_transition.
--
-- engaged_channel is only meaningful while Engaged; leaving Engaged must
-- clear stale routing context even when the caller does not pass a channel.
-- post_interaction_state: explicit assignment (not COALESCE) so the handler
-- can clear it by passing nil (Gemini MED fix: cross-field invariant — only
-- meaningful while Engaged or WrapUp; buildUpdateParams nil-gates it per
-- the cross-field invariant in agent_states.go).
UPDATE agent_states
SET status                 = COALESCE(sqlc.narg('to_status')::text, status),
    engaged_channel        = CASE
                                 WHEN COALESCE(sqlc.narg('to_status')::text, status) = 'Engaged'
                                 THEN COALESCE(sqlc.narg('engaged_channel')::text, engaged_channel)
                                 ELSE NULL
                             END,
    break_reason_id        = sqlc.narg('break_reason_id')::uuid,
    post_interaction_state = sqlc.narg('post_interaction_state')::text,
    wrapup_until           = sqlc.narg('wrapup_until')::timestamptz,
    state_version          = state_version + 1,
    updated_at             = NOW()
WHERE agent_id = sqlc.arg('agent_id')
  AND org_id   = sqlc.arg('org_id')
  AND status   = sqlc.arg('expected_from')
RETURNING agent_id, org_id, status, engaged_channel, break_reason_id,
         post_interaction_state, wrapup_until, state_version, updated_at;

-- name: ForceUpdateAgentStateStatus :one
-- D-84: bypasses the transition matrix (no `status = expected_from`).
-- Cross-row break_reason probe STILL runs at handler layer (Pitfall 3 —
-- force does NOT bypass cross-org probes).
--
-- Same cross-field column semantics as UpdateAgentStateStatus.
UPDATE agent_states
SET status                 = COALESCE(sqlc.narg('to_status')::text, status),
    engaged_channel        = CASE
                                 WHEN COALESCE(sqlc.narg('to_status')::text, status) = 'Engaged'
                                 THEN COALESCE(sqlc.narg('engaged_channel')::text, engaged_channel)
                                 ELSE NULL
                             END,
    break_reason_id        = sqlc.narg('break_reason_id')::uuid,
    post_interaction_state = sqlc.narg('post_interaction_state')::text,
    wrapup_until           = sqlc.narg('wrapup_until')::timestamptz,
    state_version          = state_version + 1,
    updated_at             = NOW()
WHERE agent_id = sqlc.arg('agent_id')
  AND org_id   = sqlc.arg('org_id')
RETURNING agent_id, org_id, status, engaged_channel, break_reason_id,
         post_interaction_state, wrapup_until, state_version, updated_at;

-- name: ListExpiringWrapUps :many
-- Called from sweeper goroutine via OrgDB.WithBypass (D-04). SELECT
-- mentions org_id explicitly so SQLChecker approves the query
-- (tenantTables entry from Wave 0 + denormalized column from D-78).
-- Returns ALL WrapUp rows so the sweeper can schedule per-agent
-- AfterFunc timers OR fire-immediately for past-due rows (D-81).
SELECT agent_id, org_id, wrapup_until, post_interaction_state
FROM agent_states
WHERE status = 'WrapUp'
ORDER BY org_id, wrapup_until;

-- name: ExpireWrapUp :one
-- Idempotent (D-81): only the first concurrent firing matches the
-- predicate. Returns org_id + new status so the sweeper can invalidate
-- the right cache key (Pitfall 8 — sweeper has no ctx-injected org_id).
--
-- Gemini-HIGH-1 fix: post_interaction_state column stores lowercase
-- (ready/not_ready per OpenAPI PostInteractionState enum) but the
-- status column requires PascalCase (Ready/NotReady per AgentStatus
-- CHECK constraint). The CASE expression translates at write time —
-- failing to translate would CHECK-violate (23514) on every WrapUp
-- expiry and break ROADMAP acceptance test #3.
UPDATE agent_states
SET status          = CASE post_interaction_state
                          WHEN 'ready'     THEN 'Ready'
                          WHEN 'not_ready' THEN 'NotReady'
                          ELSE 'NotReady'
                      END,
    wrapup_until    = NULL,
    engaged_channel = NULL,
    post_interaction_state = NULL,
    state_version   = state_version + 1,
    updated_at      = NOW()
WHERE agent_id = $1
  AND org_id   = $2
  AND status   = 'WrapUp'
  AND wrapup_until < NOW()
RETURNING agent_id, org_id, status, state_version;
