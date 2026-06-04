-- name: AppendRuntimeEvent :one
INSERT INTO runtime_events (
    id, org_id, route_request_id, source, type, correlation_id, payload
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListRuntimeEventsByRoute :many
SELECT * FROM runtime_events
WHERE org_id = $1 AND route_request_id = $2
ORDER BY created_at ASC, id ASC;
