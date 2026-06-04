-- v0.3 W3 capacity slot queries. Slot state is derived from two columns:
--   free      = reservation_id IS NULL
--   pending   = reservation_id IS NOT NULL AND hold_expires_at IS NOT NULL
--   confirmed = reservation_id IS NOT NULL AND hold_expires_at IS NULL
-- The offer tx acquires a free slot (authoritative gate); accept confirms;
-- terminal releases; the timer sweeps PENDING only; the reconciler reclaims a
-- confirmed slot orphaned by a crash.

-- ProvisionCapacitySlots ensures slot_no 1..$3 exist for (agent, channel).
-- Idempotent: a re-provision at the same or lower capacity is a no-op. Shrinking
-- does NOT delete rows — AcquireCapacitySlot filters by the current cap so an
-- over-provisioned row from a past higher capacity is never handed out.
-- name: ProvisionCapacitySlots :exec
INSERT INTO agent_capacity_slots (org_id, agent_id, channel, slot_no)
SELECT $1, $2, $3, gs
FROM generate_series(1, $4::int) AS gs
ON CONFLICT (org_id, agent_id, channel, slot_no) DO NOTHING;

-- AcquireCapacitySlot claims one free slot (<= current capacity) under
-- FOR UPDATE SKIP LOCKED so concurrent acquirers never collide. ErrNoRows ⇒ at
-- capacity → the caller aborts the offer.
-- name: AcquireCapacitySlot :one
WITH picked AS (
    SELECT acs.org_id, acs.agent_id, acs.channel, acs.slot_no
    FROM agent_capacity_slots acs
    WHERE acs.org_id = $1 AND acs.agent_id = $2 AND acs.channel = $3
      AND acs.reservation_id IS NULL AND acs.slot_no <= $6::int
    ORDER BY acs.slot_no
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE agent_capacity_slots s
SET reservation_id = $4, hold_expires_at = $5, updated_at = NOW()
FROM picked p
WHERE s.org_id = p.org_id AND s.agent_id = p.agent_id
  AND s.channel = p.channel AND s.slot_no = p.slot_no
RETURNING s.slot_no;

-- ConfirmCapacitySlot promotes a pending hold to confirmed on accept. The
-- hold_expires_at > now() guard rejects an expired hold even if the sweep hasn't
-- run (defense-in-depth atop AcceptReservation's own expiry guard). 0 rows ⇒ the
-- slot was lost → accept conflict.
-- name: ConfirmCapacitySlot :execrows
UPDATE agent_capacity_slots
SET hold_expires_at = NULL, updated_at = NOW()
WHERE org_id = $1 AND reservation_id = $2 AND hold_expires_at > now();

-- ReleaseCapacitySlot frees a slot on a terminal transition. Idempotent: 0 rows
-- just means a sweep/reconcile freed it first — the caller treats that as benign.
-- name: ReleaseCapacitySlot :execrows
UPDATE agent_capacity_slots
SET reservation_id = NULL, hold_expires_at = NULL, updated_at = NOW()
WHERE org_id = $1 AND reservation_id = $2;

-- SweepExpiredCapacityHolds reclaims PENDING holds whose timer elapsed (RONA / a
-- producer crash between acquire and accept). Confirmed slots (hold_expires_at
-- NULL) are untouched. Sets BOTH columns NULL so a freed row is fully free.
-- name: SweepExpiredCapacityHolds :execrows
UPDATE agent_capacity_slots
SET reservation_id = NULL, hold_expires_at = NULL, updated_at = NOW()
WHERE hold_expires_at IS NOT NULL AND hold_expires_at < now();

-- ReconcileOrphanedSlots reclaims any slot (pending OR confirmed) whose
-- reservation is gone or already terminal — the safety net for a confirmed slot
-- orphaned by a gateway/agent crash that never sent a terminal command.
-- name: ReconcileOrphanedSlots :execrows
UPDATE agent_capacity_slots s
SET reservation_id = NULL, hold_expires_at = NULL, updated_at = NOW()
WHERE s.org_id = $1 AND s.reservation_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM reservations r
      WHERE r.org_id = s.org_id AND r.id = s.reservation_id
        AND r.state IN ('offered', 'accepted')
  );

-- ReconcileOrphanedSlotsAllOrgs is the cross-org sweep variant for the background
-- worker (run via the raw pool — no org filter, like the continuation worker).
-- It reclaims a slot whose reservation is gone or already TERMINAL. Note: a slot
-- whose reservation is still 'accepted' (an agent who crashed mid-call) is NOT
-- reclaimed here — that abandonment cleanup is presence-loss driven (W5 RONA),
-- built on the W3 lease.
-- name: ReconcileOrphanedSlotsAllOrgs :execrows
UPDATE agent_capacity_slots s
SET reservation_id = NULL, hold_expires_at = NULL, updated_at = NOW()
WHERE s.reservation_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM reservations r
      WHERE r.org_id = s.org_id AND r.id = s.reservation_id
        AND r.state IN ('offered', 'accepted')
  );

-- CountFreeCapacitySlots is how many free slots (<= current cap) the agent has
-- on the channel (used by tests; the live hint uses CountHeldCapacitySlots so it
-- doesn't depend on slots being pre-provisioned).
-- name: CountFreeCapacitySlots :one
SELECT COUNT(*)::int AS free
FROM agent_capacity_slots
WHERE org_id = $1 AND agent_id = $2 AND channel = $3
  AND reservation_id IS NULL AND slot_no <= $4::int;

-- CountHeldCapacityByAgents is the BULK hint read for the live candidate source:
-- held interactions per agent on a channel, for a set of agents — one query
-- instead of N (review HIGH: no per-candidate round-trip). Agents with 0 held
-- simply don't appear in the result.
-- name: CountHeldCapacityByAgents :many
SELECT agent_id, COUNT(*)::int AS held
FROM agent_capacity_slots
WHERE org_id = $1 AND channel = $2 AND reservation_id IS NOT NULL
  AND agent_id = ANY($3::uuid[])
GROUP BY agent_id;

-- CountHeldCapacitySlots is the candidate-source hint (NOT authoritative): how
-- many interactions the agent currently holds on the channel. under-capacity =
-- held < cap — provisioning-independent (an unprovisioned agent reads held=0, so
-- the hint includes them; the offer tx provisions+acquires authoritatively).
-- name: CountHeldCapacitySlots :one
SELECT COUNT(*)::int AS held
FROM agent_capacity_slots
WHERE org_id = $1 AND agent_id = $2 AND channel = $3
  AND reservation_id IS NOT NULL;
