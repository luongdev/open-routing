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
    -- Sync the distinct-agent RONA history from THIS route's resolved offers so
    -- the matcher (ClaimWaitingRoute: $agent <> ALL(excluded_agent_ids)) won't
    -- re-ring an agent who already rejected/timed out — the in-run `excluded` map
    -- only fences the inline re-run, not the later SQL claim (cross-AI review
    -- HIGH). Authoritative recompute (empty on the first enqueue).
    excluded_agent_ids = COALESCE(
        (SELECT array_agg(DISTINCT r.agent_id) FROM reservations r
         WHERE r.org_id = $2 AND r.route_request_id = $1 AND r.state IN ('rejected', 'timeout')),
        '{}'),
    match_offer_token = NULL,
    offering_started_at = NULL,
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
      -- Don't claim a route past its queue SLA — it belongs to the deadline sweep
      -- (→ fallback), not to a new offer (cross-AI review HIGH: claim-vs-expire race).
      AND (rr.match_deadline IS NULL OR rr.match_deadline > now())
      AND rr.required_skills <@ $2::text[]
      AND $3::uuid <> ALL(rr.excluded_agent_ids)
      -- NOTE: queue/channel are NOT eligibility filters in v0.3 — there is no
      -- agent↔queue membership yet, so the candidate pool is skill-based and all
      -- queues share it. When membership lands, filter served queues here.
    ORDER BY (rr.priority * $4::float8
              + EXTRACT(EPOCH FROM now() - rr.waiting_since) * $5::float8) DESC,
             rr.id ASC
    FOR UPDATE OF rr SKIP LOCKED
    LIMIT 1
)
UPDATE route_requests rr
SET status = 'offering',
    match_offer_token = gen_random_uuid(),
    offering_started_at = now(),
    updated_at = now()
FROM picked p
-- rr.org_id = $1 is redundant for correctness (the CTE already scoped org_id and
-- id is the PK) but REQUIRED: SQLChecker rejects an org-scoped UPDATE whose
-- top-level WHERE has no org_id ColumnRef (cross-AI review BLOCK).
WHERE rr.id = p.id AND rr.org_id = $1
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
-- token-fenced so a superseded claim can't commit. ErrNoRows ⇒ the worker rolls
-- back its offer + releases the slot. Bumps run_seq (a new suspension at this
-- offer) so a stale reservation_timeout from a prior attempt is fenced out by
-- AcquireRouteForRunAtSeq — same invariant SuspendRoute upholds on the inline path.
-- match_attempt_seq is incremented HERE (not at claim) so it counts ACTUAL offers
-- to distinct agents, not claims that lost the capacity/lease gate — otherwise a
-- few at-capacity agents would burn a route's attempt budget (cross-AI review BLOCK).
-- name: CommitMatchOffer :one
UPDATE route_requests
SET status = 'waiting', active_reservation_id = $4,
    current_reservation_id = $4, resume_cursor = $5,
    run_seq = run_seq + 1,
    match_attempt_seq = match_attempt_seq + 1,
    match_offer_token = NULL, offering_started_at = NULL, updated_at = now()
WHERE id = $1 AND org_id = $2 AND status = 'offering' AND match_offer_token = $3
RETURNING *;

-- SweepStaleOffering reclaims routes stuck 'offering' (a worker crashed after the
-- claim, before/inside the offer tx). Re-tokens so the original worker's commit
-- (fenced on the old token) can't land. Cross-org via the raw pool.
-- name: SweepStaleOffering :execrows
UPDATE route_requests
SET status = 'waiting_match', next_match_at = now(),
    match_offer_token = gen_random_uuid(), offering_started_at = NULL, updated_at = now()
WHERE status = 'offering' AND offering_started_at < $1;

-- ListOrgsWithWaitingMatch returns the distinct orgs that currently have a route
-- parked in the queue, so the cross-org matcher driver (cmd/runtime) only spins a
-- per-org cycle for orgs with pending demand. Cross-org via the raw pool (no org
-- filter — like the sweeps). Bounded so a pathological org count can't unbound the
-- tick.
-- name: ListOrgsWithWaitingMatch :many
SELECT DISTINCT org_id FROM route_requests
WHERE status = 'waiting_match' AND next_match_at <= now()
LIMIT $1;

-- ListAvailableAgentsForMatch is the matcher's availability side: enabled agents
-- in Ready state that are routable (no RONA hold, or an expired one) — each with
-- their enabled skill codes aggregated so the caller can claim a skill-eligible
-- route. Presence (the connection lease) is filtered in Go, not here (Redis).
-- Bounded batch. Org-scoped: every tenant alias carries org_id in the WHERE
-- (SQLChecker), and the LEFT-joined skill rows use `OR ... IS NULL` so a
-- skill-less agent is not dropped (it can still serve a no-skill route).
-- name: ListAvailableAgentsForMatch :many
SELECT a.id AS agent_id,
       a.code AS agent_code,
       COALESCE(array_remove(array_agg(DISTINCT sk.code), NULL), '{}')::text[] AS skills
-- org_id is repeated in each LEFT JOIN ON (defense-in-depth, cross-AI review HIGH)
-- AND in the top-level WHERE: the ON keeps the join itself org-correct, the WHERE
-- ColumnRef satisfies SQLChecker (which ignores ON-clause org refs). agent_id /
-- skill_id are org-unique so this is belt-and-suspenders, not a behavior change.
FROM agents a
JOIN agent_states ast ON ast.agent_id = a.id AND ast.org_id = a.org_id
LEFT JOIN agent_routing_state ars ON ars.agent_id = a.id AND ars.org_id = a.org_id
LEFT JOIN agent_skills ags ON ags.agent_id = a.id AND ags.org_id = a.org_id
LEFT JOIN skills sk ON sk.id = ags.skill_id AND sk.org_id = a.org_id AND sk.enabled = TRUE
WHERE a.org_id = $1
  AND ast.org_id = $1
  AND (ars.org_id = $1 OR ars.org_id IS NULL)
  AND (ags.org_id = $1 OR ags.org_id IS NULL)
  AND (sk.org_id = $1 OR sk.org_id IS NULL)
  AND a.enabled = TRUE
  AND ast.status = 'Ready'
  AND (ars.agent_id IS NULL
       OR ars.routing_state = 'routable'
       OR (ars.state_expires_at IS NOT NULL AND ars.state_expires_at <= now()))
GROUP BY a.id, a.code
-- random() (not a.code) so successive ticks SAMPLE different agents: a fixed
-- alphabetical LIMIT would let the first N at-capacity agents starve the N+1th who
-- actually has a free slot (capacity is gated in Go, not this query — review HIGH).
ORDER BY random()
LIMIT $2;

-- MarkAgentMissed puts an agent into a short RONA cooldown after an offer to them
-- timed out (didn't answer) so the matcher doesn't immediately re-ring them for a
-- DIFFERENT route. The Ready-race fence ($4 = the missed offer's offered_at): a
-- conflicting row is only downgraded to 'missed' when the agent has NOT gone Ready
-- since that offer (last_ready_at NULL or older than the offer) — so a Ready that
-- arrived after the ring is not clobbered (cross-AI review HIGH R-readyfence). The
-- READ side (ListAvailableAgentsForMatch) treats an expired cooldown as routable,
-- so recovery is automatic; an explicit Ready clears it once the state machine
-- writes last_ready_at. 0 rows ⇒ the fence held (the agent re-readied) — benign.
-- name: MarkAgentMissed :execrows
INSERT INTO agent_routing_state (org_id, agent_id, routing_state, state_expires_at, last_ready_at)
VALUES ($1, $2, 'missed', $3, NULL)
ON CONFLICT (org_id, agent_id) DO UPDATE
SET routing_state = 'missed', state_expires_at = $3, updated_at = now()
WHERE agent_routing_state.org_id = $1
  AND (agent_routing_state.last_ready_at IS NULL
       OR agent_routing_state.last_ready_at <= $4);

-- InsertRouteDecision records one matcher decision (the D9 audit trail): who was
-- selected/considered, the outcome, and a JSONB detail blob (ranking, excluded,
-- eligibility). Written on every offer and every pre-offer failure.
-- name: InsertRouteDecision :exec
INSERT INTO route_decisions (
    id, org_id, route_request_id, decision_type, matcher_instance, channel,
    queue_id, selected_agent_id, selected_slot_no, outcome, reason, detail
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- ListExpiredMatchRoutes finds queue-SLA-expired routes (waiting_match past their
-- match_deadline) the matcher must give up on → no_candidate fallback. Read-only
-- discovery; the per-route fenced AcquireExpiredMatchRouteForRun does the
-- exactly-once flip+resume so 'running' is never committed without a driver. Only
-- waiting_match (an offering route's reservation_timeout wins — precedence).
-- Cross-org via the raw pool, bounded batch.
-- name: ListExpiredMatchRoutes :many
SELECT id, org_id FROM route_requests
WHERE status = 'waiting_match' AND match_deadline IS NOT NULL AND match_deadline <= now()
ORDER BY match_deadline
LIMIT $1;

-- AcquireExpiredMatchRouteForRun is the org-scoped, fenced flip the SLA fallback
-- runs INSIDE its resume tx: waiting_match-past-deadline → running, atomically with
-- the no_candidate resume + persist (so a crash rolls back to waiting_match, never
-- a stuck 'running'). ErrNoRows ⇒ another worker/tick already took it (the status
-- guard gives exactly-once across replicas — no SKIP LOCKED needed).
-- name: AcquireExpiredMatchRouteForRun :one
UPDATE route_requests
SET status = 'running',
    match_offer_token = NULL, offering_started_at = NULL, updated_at = now()
WHERE id = $1 AND org_id = $2 AND status = 'waiting_match'
  AND match_deadline IS NOT NULL AND match_deadline <= now()
RETURNING *;
