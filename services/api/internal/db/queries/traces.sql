-- name: InsertTrace :one
INSERT INTO traces (
    id, org_id, kind, flow_id, flow_version_id, route_request_id,
    steps, outcome,
    compiled_plan_snapshot, plan_format_version, simulation_input, read_set_snapshot, graph_hash
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetTrace :one
SELECT * FROM traces WHERE id = $1 AND org_id = $2;

-- name: ListFlowTraces :many
SELECT * FROM traces
WHERE org_id = $1 AND flow_id = $2
ORDER BY created_at DESC
LIMIT $3;
