-- v0.3 W4 matcher queries. The route_requests row carries the queue/match state
-- (status waiting_match/offering, priority, required_skills, match_offer_token,
-- excluded_agent_ids). Every claim/requeue/recover is fenced by match_offer_token
-- so a stalled worker can't clobber a fresh offer (plan rev2, codex BLOCK).

-- EnqueueRouteForMatch parks a running route in the queue: the reservation node
-- found no available agent, so instead of falling through to fallback the route
-- waits for the matcher. Captures the queue/skills/SLA context.
-- name: EnqueueRouteForMatch :one
UPDATE route_requests
SET status = 'waiting_match',
    queue_id = $3,
    priority = $4,
    required_skills = $5,
    waiting_since = now(),
    next_match_at = now(),
    match_deadline = $6,
    resume_cursor = $7,
    updated_at = now()
WHERE id = $1 AND org_id = $2 AND status = 'running'
RETURNING *;

-- ClaimWaitingRoute is the matcher's availability-driven pull for ONE agent: the
-- best-ranked eligible waiting route, claimed atomically. Locks ONLY rr (FOR
-- UPDATE OF rr SKIP LOCKED — a bare lock would grab joined rows → cross-agent
-- deadlock); the same statement flips status='offering' + stamps a fresh
-- match_offer_token so a second puller can't grab it (pull-to-offer race).
-- Eligibility: required_skills <@ the agent's skills, agent not already excluded.
-- Ranking computed IN SQL so uncapped aging can cross a priority band (no
-- bounded-prefix starvation — plan rev2 BLOCK): effective = priority*$4 +
-- age_seconds*$5, deterministic id tie-break.
-- name: ClaimWaitingRoute :one
WITH picked AS (
    SELECT rr.id
    FROM route_requests rr
    WHERE rr.org_id = $1
      AND rr.status = 'waiting_match'
      AND rr.next_match_at <= now()
      AND rr.required_skills <@ $2::text[]
      AND $3::uuid <> ALL(rr.excluded_agent_ids)
    ORDER BY (rr.priority * $4::float8
              + EXTRACT(EPOCH FROM now() - rr.waiting_since) * $5::float8) DESC,
             rr.id ASC
    FOR UPDATE OF rr SKIP LOCKED
    LIMIT 1
)
UPDATE route_requests rr
SET status = 'offering',
    match_offer_token = gen_random_uuid(),
    match_attempt_seq = rr.match_attempt_seq + 1,
    offering_started_at = now(),
    updated_at = now()
FROM picked p
WHERE rr.id = p.id
RETURNING rr.*;

-- ReturnRouteToQueue requeues a route after a failed offer (slot/lease lost),
-- token-fenced so only the worker that claimed it can return it — a sweeper that
-- already re-tokened a stale offering wins.
-- name: ReturnRouteToQueue :execrows
UPDATE route_requests
SET status = 'waiting_match', next_match_at = now(),
    match_offer_token = NULL, offering_started_at = NULL, updated_at = now()
WHERE id = $1 AND org_id = $2 AND status = 'offering' AND match_offer_token = $3;

-- CommitMatchOffer flips a claimed route to 'waiting' (an offer was attached),
-- token-fenced so a superseded claim can't commit. 0 rows ⇒ the worker rolls
-- back its offer + releases the slot.
-- name: CommitMatchOffer :execrows
UPDATE route_requests
SET status = 'waiting', active_reservation_id = $4,
    current_reservation_id = $4, resume_cursor = $5,
    match_offer_token = NULL, offering_started_at = NULL, updated_at = now()
WHERE id = $1 AND org_id = $2 AND status = 'offering' AND match_offer_token = $3;

-- SweepStaleOffering reclaims routes stuck 'offering' (a worker crashed after the
-- claim, before/inside the offer tx). Re-tokens so the original worker's commit
-- (fenced on the old token) can't land. Cross-org via the raw pool.
-- name: SweepStaleOffering :execrows
UPDATE route_requests
SET status = 'waiting_match', next_match_at = now(),
    match_offer_token = gen_random_uuid(), offering_started_at = NULL, updated_at = now()
WHERE status = 'offering' AND offering_started_at < $1;

-- ClaimExpiredMatchRoutes flips queue-SLA-expired routes to 'running' (the route
-- run-lock) so the continuation worker can resume them with the no_candidate
-- fallback. Targets ONLY waiting_match (an offering route's reservation_timeout
-- wins — precedence). Cross-org via the raw pool, bounded batch.
-- name: ClaimExpiredMatchRoutes :many
UPDATE route_requests
SET status = 'running', updated_at = now()
WHERE id IN (
    SELECT id FROM route_requests
    WHERE status = 'waiting_match' AND match_deadline IS NOT NULL AND match_deadline <= now()
    ORDER BY match_deadline
    FOR UPDATE SKIP LOCKED
    LIMIT $1
)
RETURNING *;
