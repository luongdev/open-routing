-- name: InsertContinuation :one
INSERT INTO continuations (
    id, org_id, kind, route_request_id, reservation_id, agent_id,
    flow_version_id, cursor, due_at, run_seq, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'pending')
RETURNING *;

-- ClaimDueContinuations atomically leases due rows for this worker. FOR UPDATE
-- SKIP LOCKED lets multiple replicas claim disjoint rows. It picks pending rows
-- OR claimed rows whose lease expired (crash recovery). $1=now, $2=worker id,
-- $3=lease expiry, $4=limit, $5=max attempts. The attempt_count gate stops a
-- poison row from being reclaimed at the head of the queue forever (review H6).
-- name: ClaimDueContinuations :many
UPDATE continuations
SET status = 'claimed', claimed_at = $1, claimed_by = $2, claim_expires_at = $3,
    attempt_count = attempt_count + 1, updated_at = $1
WHERE id IN (
    SELECT c.id FROM continuations c
    WHERE c.due_at <= $1
      AND (c.status = 'pending' OR (c.status = 'claimed' AND c.claim_expires_at < $1))
      AND c.attempt_count < $5
    ORDER BY c.due_at
    FOR UPDATE SKIP LOCKED
    LIMIT $4
)
RETURNING *;

-- ResolveContinuation marks a claimed row done/cancelled, FENCED by claimed_by:
-- if a lost-lease worker tries to resolve a row another worker re-claimed, 0
-- rows update and it drops the work. $4=new status.
-- name: ResolveContinuation :execrows
UPDATE continuations
SET status = $4, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND claimed_by = $3;

-- name: FailContinuation :execrows
UPDATE continuations
SET status = 'cancelled', last_error = $4, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND claimed_by = $3;

-- name: GetContinuation :one
SELECT * FROM continuations WHERE id = $1 AND org_id = $2;
