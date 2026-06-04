-- name: InsertRouteRequest :one
INSERT INTO route_requests (
    id, org_id, channel, entry_code, flow_version_id, flow_code,
    interaction_input, status, failure_code, read_set_snapshot
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- AcquireRouteForRun is the route-level exclusive lock: it flips an idle route
-- to 'running' so exactly one process (API handler or worker) executes the flow
-- at a time. 0 rows returned ⇒ another process owns it or it was cancelled —
-- the caller MUST abort the resume.
-- name: AcquireRouteForRun :one
UPDATE route_requests
SET status = 'running', updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND status IN ('pending', 'waiting')
RETURNING *;

-- SuspendRoute parks the cursor and releases the run-lock (running -> waiting).
-- name: SuspendRoute :one
UPDATE route_requests
SET status = 'waiting', resume_cursor = $3, current_reservation_id = $4,
    run_seq = run_seq + 1, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND status = 'running'
RETURNING *;

-- FinishRoute terminates the route (completed/failed) and clears the cursor.
-- name: FinishRoute :one
UPDATE route_requests
SET status = $3, failure_code = $4, resume_cursor = NULL,
    current_reservation_id = NULL, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND status = 'running'
RETURNING *;

-- name: CancelRouteRequest :one
UPDATE route_requests
SET status = 'cancelled', resume_cursor = NULL, current_reservation_id = NULL,
    updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND status IN ('pending', 'waiting')
RETURNING *;

-- name: GetRouteRequest :one
SELECT * FROM route_requests WHERE id = $1 AND org_id = $2;

-- name: ListRouteRequests :many
SELECT * FROM route_requests
WHERE org_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2;
