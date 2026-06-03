-- v0.2 Wave 2 — flow entry bindings (FLOW). One ACTIVE binding per
-- (org_id, channel, entry_code) is enforced by the partial unique index
-- ux_flow_bindings_active. Publish/rollback both run deactivate-then-insert in
-- one transaction, so the postcondition is exactly one active binding.

-- name: GetActiveBinding :one
SELECT id, org_id, channel, entry_code, flow_version_id, flow_code, active, created_at, updated_at
FROM flow_entry_bindings
WHERE org_id = $1 AND channel = $2 AND entry_code = $3 AND active = TRUE;

-- name: DeactivateActiveBinding :execrows
-- Clears the current active binding (if any) ahead of inserting the new one,
-- inside the publish/rollback transaction — keeps the partial unique index satisfied.
UPDATE flow_entry_bindings
SET active = FALSE, updated_at = NOW()
WHERE org_id = $1 AND channel = $2 AND entry_code = $3 AND active = TRUE;

-- name: InsertActiveBinding :one
INSERT INTO flow_entry_bindings (id, org_id, channel, entry_code, flow_version_id, flow_code, active)
VALUES ($1, $2, $3, $4, $5, $6, TRUE)
RETURNING id, org_id, channel, entry_code, flow_version_id, flow_code, active, created_at, updated_at;
