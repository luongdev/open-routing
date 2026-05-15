-- name: InsertScaffold :one
INSERT INTO _scaffold (id, org_id, external_id, name)
VALUES ($1, $2, $3, $4)
RETURNING id, org_id, external_id, name, created_at;

-- name: GetScaffoldByID :one
SELECT id, org_id, external_id, name, created_at
FROM _scaffold
WHERE id = $1 AND org_id = $2;

-- name: ListScaffolds :many
SELECT id, org_id, external_id, name, created_at
FROM _scaffold
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT 100;
