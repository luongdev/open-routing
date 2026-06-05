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

-- name: CancelOfferedReservationsForRoute :many
UPDATE reservations
SET state = 'cancelled', resolved_at = NOW(), updated_at = NOW()
WHERE org_id = $1 AND route_request_id = $2 AND state = 'offered'
RETURNING *;

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
