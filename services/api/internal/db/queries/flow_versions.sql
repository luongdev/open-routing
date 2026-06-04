-- v0.2 Wave 2 — immutable published flow versions (FLOW). Write-once: the
-- BEFORE UPDATE/DELETE trigger (migration 000001) rejects mutation, so there is
-- no Update/Delete query by design. version_number is monotonic per
-- (org_id, flow_code) and survives draft deletion.

-- name: NextFlowVersionNumber :one
-- COALESCE(MAX,0)+1 per (org_id, flow_code). Two racing publishes can compute
-- the same number; the UNIQUE(org_id, flow_code, version_number) then rejects
-- the loser with 23505 (mapped to 409) rather than silently overwriting.
SELECT COALESCE(MAX(version_number), 0) + 1 AS next_version
FROM flow_versions
WHERE org_id = $1 AND flow_code = $2;

-- name: InsertFlowVersion :one
INSERT INTO flow_versions (id, org_id, flow_id, flow_code, version_number, graph, compiled_plan, plan_format_version)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, flow_id, flow_code, version_number, graph, compiled_plan, plan_format_version, created_at;

-- name: GetFlowVersion :one
SELECT id, org_id, flow_id, flow_code, version_number, graph, compiled_plan, plan_format_version, created_at
FROM flow_versions
WHERE id = $1 AND org_id = $2;

-- name: GetFlowVersionByNumber :one
-- Rollback target lookup: resolve (flow_code, version_number) to its immutable row.
SELECT id, org_id, flow_id, flow_code, version_number, graph, compiled_plan, plan_format_version, created_at
FROM flow_versions
WHERE org_id = $1 AND flow_code = $2 AND version_number = $3;

-- name: ListFlowVersionsByCode :many
SELECT id, org_id, flow_id, flow_code, version_number, graph, compiled_plan, plan_format_version, created_at
FROM flow_versions
WHERE org_id = $1 AND flow_code = $2
ORDER BY version_number DESC
LIMIT $3;
