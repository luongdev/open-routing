-- InsertReservationOffer binds the lease at offer time: lease_token (a fresh
-- per-offer secret echoed by the agent on accept/reject — survives reconnect) and
-- agent_session_id (the session the offer was delivered to, best-effort/NULLable).
-- A stale command for a superseded offer fails the lease fence (D5 R-fence).
-- name: InsertReservationOffer :one
INSERT INTO reservations (
    id, org_id, route_request_id, agent_id, state, attempt, offered_at, expires_at,
    lease_token, agent_session_id
) VALUES ($1, $2, $3, $4, 'offered', $5, NOW(), $6, $7, $8)
RETURNING *;

-- Guarded transitions: the WHERE state clause is the concurrency authority. 0
-- rows ⇒ the offer was already resolved (accept-vs-timeout race loser) — the
-- caller treats it as a no-op / 409.
-- name: AcceptReservation :one
-- expires_at guard (review H2): an offer whose timeout already elapsed cannot be
-- accepted even if its timeout continuation hasn't fired yet. clock_timestamp()
-- (not NOW()/tx-start) so a long-running tx can't accept past real expiry.
UPDATE reservations
SET state = 'accepted', resolved_at = NOW(), updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND state = 'offered' AND expires_at > clock_timestamp()
RETURNING *;

-- name: RejectReservation :one
UPDATE reservations
SET state = 'rejected', resolved_at = NOW(), reason = $3, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND state = 'offered'
RETURNING *;

-- name: TimeoutReservation :one
UPDATE reservations
SET state = 'timeout', resolved_at = NOW(), updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND state = 'offered'
RETURNING *;

-- name: CompleteReservation :one
UPDATE reservations
SET state = 'completed', resolved_at = NOW(), updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND state = 'accepted'
RETURNING *;

-- ListReclaimableAcceptedReservations finds accepted reservations (a live call)
-- whose agent has not been seen on ANY session since $1 (now - reclaim grace) —
-- i.e. the agent vanished mid-call and the capacity slot is stuck held. Grace on
-- last_seen_at (not terminated_at) so a brief WS blip+reconnect refreshes a new
-- session before the cutoff and is NOT reclaimed (the W5 "no reject on a blip"
-- principle, extended to confirmed calls). Cross-org (raw pool); the per-route
-- reclaim re-scopes + re-fences under org.
-- name: ListReclaimableAcceptedReservations :many
SELECT r.id, r.org_id, r.route_request_id, r.agent_id
FROM reservations r
WHERE r.state = 'accepted'
  AND NOT EXISTS (
    SELECT 1 FROM agent_sessions s
    WHERE s.org_id = r.org_id AND s.agent_id = r.agent_id AND s.last_seen_at > $1
  )
ORDER BY r.updated_at ASC
LIMIT $2;

-- ReclaimStaleAcceptedReservation is the atomic per-call fence + flip: cancel the
-- reservation (freeing its capacity slot) ONLY if it is still accepted AND the
-- agent is still unseen since $3. The conditional UPDATE row-locks, so two racing
-- reclaims or an agent that reconnected (its new session's last_seen_at > $3)
-- yield 0 rows ⇒ no-op. RETURNING carries what the post-flip cleanup needs (the
-- route to maybe tear down, the agent to move out of Engaged, the adapter handle
-- to release). Reclaim is reservation-scoped, not route-scoped, because a route
-- can be terminal while its accepted reservation still holds the live call's slot.
-- name: ReclaimStaleAcceptedReservation :one
UPDATE reservations r
SET state = 'cancelled', reason = 'agent_lost', resolved_at = NOW(), updated_at = NOW()
WHERE r.id = $1 AND r.org_id = $2 AND r.state = 'accepted'
  AND NOT EXISTS (
    SELECT 1 FROM agent_sessions s
    WHERE s.org_id = r.org_id AND s.agent_id = r.agent_id AND s.last_seen_at > $3
  )
RETURNING r.route_request_id, r.agent_id, r.adapter_handle;

-- name: CancelOfferedReservationsForRoute :many
UPDATE reservations
SET state = 'cancelled', resolved_at = NOW(), updated_at = NOW()
WHERE org_id = $1 AND route_request_id = $2 AND state = 'offered'
RETURNING *;

-- CancelLiveReservationsForRoute terminalizes BOTH an outstanding offer AND an
-- in-progress accepted call on a caller-abandon teardown, so neither leaks its
-- capacity slot. The handler reads prior states (ListReservationsByRoute) before
-- calling this, to free each slot and move an accepted call's agent into WrapUp
-- (cross-AI review HIGH).
-- name: CancelLiveReservationsForRoute :many
UPDATE reservations
SET state = 'cancelled', resolved_at = NOW(), updated_at = NOW()
WHERE org_id = $1 AND route_request_id = $2 AND state IN ('offered', 'accepted')
RETURNING *;

-- SetReservationAdapterHandle binds the adapter's opaque delivery handle on an
-- accepted reservation (so Release/adapter-event mapping can find it). Guarded on
-- 'accepted' so it can't attach to a reservation that already went terminal.
-- name: SetReservationAdapterHandle :execrows
UPDATE reservations
SET adapter_handle = $3, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND state = 'accepted';

-- GetReservationByID looks up a reservation by id ALONE (no org filter) — for the
-- channel-adapter event sink, whose AssignmentEvent carries only the reservation
-- id. Run cross-org via db.WithBypass (the row's org_id then scopes the teardown).
-- name: GetReservationByID :one
SELECT * FROM reservations WHERE id = $1;

-- name: GetReservation :one
SELECT * FROM reservations WHERE id = $1 AND org_id = $2;

-- name: ListReservationsByRoute :many
SELECT * FROM reservations
WHERE org_id = $1 AND route_request_id = $2
ORDER BY attempt ASC;

-- ListOfferedAgentsForRoute returns agents already offered to on this route
-- (any state) so the next offer excludes them — sequential-offer policy.
-- name: ListOfferedAgentsForRoute :many
SELECT DISTINCT agent_id FROM reservations
WHERE org_id = $1 AND route_request_id = $2;

-- ListAgentLiveReservations returns an agent's currently actionable reservations
-- (offered = ringing, accepted = on a call) for a polling agent console — newest
-- first. Org-scoped. The WS gateway is the real-time path; this is the browser
-- console's REST/poll fallback (no custom-header WS auth needed).
-- name: ListAgentLiveReservations :many
SELECT * FROM reservations
WHERE org_id = $1 AND agent_id = $2 AND state IN ('offered', 'accepted')
ORDER BY created_at DESC;
