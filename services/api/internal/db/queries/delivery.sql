-- v0.4 durable delivery outbox (Wave 1). The accept path commits one row per
-- reservation in the same tx as the reservation flip; a runtime drain worker
-- claims due rows, calls the channel adapter, and records the handle.

-- AppendDeliveryCommand enqueues a delivery for an accepted reservation. ON
-- CONFLICT DO NOTHING on (org_id, reservation_id) makes a retried/deduped accept
-- idempotent — never two rooms for one reservation. 0 rows ⇒ already enqueued.
-- name: AppendDeliveryCommand :execrows
INSERT INTO delivery_commands (id, org_id, reservation_id, route_request_id, agent_id, channel, interaction)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (org_id, reservation_id) DO NOTHING;

-- ClaimDueDeliveryCommands leases a batch of pending (or crashed-claim-expired)
-- rows for THIS worker, cross-org on the raw pool. FOR UPDATE SKIP LOCKED so
-- replicas don't collide; the lease lets a dead worker's row be reclaimed after
-- claim_expires_at. attempt_count bumps so a poison row can be capped later.
-- name: ClaimDueDeliveryCommands :many
UPDATE delivery_commands
SET status = 'pending', claimed_at = $1, claim_expires_at = $2, claimed_by = $3,
    attempt_count = attempt_count + 1, updated_at = $1
WHERE id IN (
    SELECT id FROM delivery_commands
    WHERE status = 'pending' AND (claim_expires_at IS NULL OR claim_expires_at < $1)
    ORDER BY created_at
    LIMIT $4
    FOR UPDATE SKIP LOCKED
)
RETURNING id, org_id, reservation_id, route_request_id, agent_id, channel, interaction, attempt_count;

-- MarkDeliveryDelivered records the adapter handle + delivered state, FENCED on
-- claimed_by + still-pending: a worker whose lease expired and was re-claimed by
-- a peer gets 0 rows here, so it cannot overwrite the newer worker's handle or
-- finalize a row it no longer owns (cross-AI review BLOCK — multi-replica safety).
-- name: MarkDeliveryDelivered :execrows
UPDATE delivery_commands
SET status = 'delivered', handle = $3, claim_expires_at = NULL, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND status = 'pending' AND claimed_by = $4;

-- MarkDeliveryFailed declares the delivery failed, same claimed_by fence. 0 rows ⇒
-- the claim was lost; the caller must NOT proceed to tear the route down.
-- name: MarkDeliveryFailed :execrows
UPDATE delivery_commands
SET status = 'failed', last_error = $3, claim_expires_at = NULL, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND status = 'pending' AND claimed_by = $4;
