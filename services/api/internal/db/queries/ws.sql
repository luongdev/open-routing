-- v0.3 W2 realtime transport queries: agent outbox (durable outbound), command
-- dedupe (at-most-once inbound), and session inventory (revocation).

-- LockAgentOutboxSeq serializes server_seq allocation per (org, agent) WITHIN the
-- caller's tx so the MAX(server_seq)+1 below is race-free across writers (the
-- per-agent advisory lock releases at tx end). Must be called before
-- AppendAgentOutbox in the same tx. Keyed off the agents row so the query
-- carries an org_id filter — SQLChecker rejects a bare pg_advisory_xact_lock as
-- an unscoped statement under OrgDB (review HIGH-2: W4's producer locks+appends
-- in one OrgDB tx).
--
-- :execrows (NOT :exec) so an ABSENT agent row is detectable: with no row the
-- SELECT locks nothing and would otherwise return nil, leaving AppendAgentOutbox
-- to race the unguarded MAX(server_seq)+1 (review HIGH — silent no-op lock). The
-- producer MUST treat a 0 row count as agent_not_found and abort before append.
-- name: LockAgentOutboxSeq :execrows
SELECT pg_advisory_xact_lock(hashtextextended(a.org_id::text || ':' || a.id::text, 0))
FROM agents a
WHERE a.org_id = $1 AND a.id = $2;

-- AppendAgentOutbox inserts the next per-agent outbox row. 0 rows ⇒ event_key
-- already present (idempotent re-derivation on reconnect) — caller treats as
-- "already enqueued".
-- name: AppendAgentOutbox :one
INSERT INTO agent_outbox (org_id, agent_id, server_seq, event_key, type, reservation_id, payload)
VALUES ($1, $2,
    (SELECT COALESCE(MAX(o.server_seq), 0) + 1 FROM agent_outbox o WHERE o.org_id = $1 AND o.agent_id = $2),
    $3, $4, $5, $6)
ON CONFLICT (org_id, agent_id, event_key) DO NOTHING
RETURNING *;

-- ReadAgentOutboxSince returns committed rows after last_seq, in seq order — the
-- relay's read + the reconnect replay both use this.
-- name: ReadAgentOutboxSince :many
SELECT * FROM agent_outbox
WHERE org_id = $1 AND agent_id = $2 AND server_seq > $3
ORDER BY server_seq
LIMIT $4;

-- BeginCommand claims a client command id. 0 rows ⇒ it already exists (a retry) —
-- the caller reads the existing row (GetCommandForUpdate) and returns its result.
-- name: BeginCommand :one
INSERT INTO ws_command_dedupe (org_id, agent_id, client_msg_id, command_type, request_hash, status)
VALUES ($1, $2, $3, $4, $5, 'pending')
ON CONFLICT (org_id, agent_id, client_msg_id) DO NOTHING
RETURNING *;

-- GetCommandForUpdate locks the dedupe row so a concurrent duplicate waits for
-- the first to finish, then reads its result.
-- name: GetCommandForUpdate :one
SELECT * FROM ws_command_dedupe
WHERE org_id = $1 AND agent_id = $2 AND client_msg_id = $3
FOR UPDATE;

-- FinishCommand stores the result and marks the command done.
-- name: FinishCommand :execrows
UPDATE ws_command_dedupe
SET status = 'done', result = $4, updated_at = NOW()
WHERE org_id = $1 AND agent_id = $2 AND client_msg_id = $3;

-- name: CreateAgentSession :one
INSERT INTO agent_sessions (org_id, session_id, agent_id, gateway_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: TouchAgentSession :execrows
UPDATE agent_sessions SET last_seen_at = NOW()
WHERE org_id = $1 AND session_id = $2 AND terminated_at IS NULL;

-- name: TerminateAgentSession :execrows
UPDATE agent_sessions SET terminated_at = NOW()
WHERE org_id = $1 AND session_id = $2 AND terminated_at IS NULL;

-- IsAgentSessionLive returns 1 when the session exists and is not terminated.
-- name: IsAgentSessionLive :one
SELECT 1::int AS live FROM agent_sessions
WHERE org_id = $1 AND session_id = $2 AND terminated_at IS NULL;
