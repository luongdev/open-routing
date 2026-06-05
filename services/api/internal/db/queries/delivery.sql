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

-- MarkDeliveryDelivered records the adapter handle + terminal-delivered state.
-- name: MarkDeliveryDelivered :execrows
UPDATE delivery_commands
SET status = 'delivered', handle = $3, claim_expires_at = NULL, updated_at = NOW()
WHERE id = $1 AND org_id = $2;

-- MarkDeliveryFailed releases the claim (so it is NOT retried — a delivery fault
-- is terminal for this attempt; the engine handles the failed delivery) + records
-- the error.
-- name: MarkDeliveryFailed :execrows
UPDATE delivery_commands
SET status = 'failed', last_error = $3, claim_expires_at = NULL, updated_at = NOW()
WHERE id = $1 AND org_id = $2;
