-- v0.2 Stage 1 — flow draft queries (FLOW). Mirrors the catalog template
-- (queues.sql): version-checked UPDATE for tx-free optimistic locking (D-66),
-- sparse-PATCH via sqlc.narg COALESCE, soft-delete via enabled (CAT-09),
-- cursor pagination via (created_at, id) tuple compare (D-63).
--
-- `code` is immutable post-create: absent from the UpdateFlow SET clause and
-- param list; the handler enforces it via the same Layer-2 gate as catalog.
-- `graph` is opaque JSONB here — structural validation lands in Stage 2.

-- name: InsertFlow :one
INSERT INTO flows (id, org_id, code, name, graph, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, code, name, graph, enabled, version, created_at, updated_at;

-- name: GetFlow :one
SELECT id, org_id, code, name, graph, enabled, version, created_at, updated_at
FROM flows
WHERE id = $1 AND org_id = $2;

-- name: GetFlowByIdAnyVersion :one
-- D-66 disambiguation probe used by the UpdateFlow 0-row / immutability paths.
SELECT id, org_id, code, name, graph, enabled, version, created_at, updated_at
FROM flows
WHERE id = $1 AND org_id = $2;

-- name: GetFlowByCode :one
-- Composite (org_id, code) match — cross-org code probes return pgx.ErrNoRows.
-- The Stage 3 publish-by-code lookup must not resolve a soft-deleted draft, so
-- enabled = TRUE is part of the match (a disabled flow returns pgx.ErrNoRows).
SELECT id, org_id, code, name, graph, enabled, version, created_at, updated_at
FROM flows
WHERE org_id = $1 AND code = $2 AND enabled = TRUE;

-- name: ListFlows :many
SELECT id, org_id, code, name, graph, enabled, version, created_at, updated_at
FROM flows
WHERE org_id = $1
  AND enabled = TRUE
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: ListFlowsIncludingDisabled :many
SELECT id, org_id, code, name, graph, enabled, version, created_at, updated_at
FROM flows
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateFlow :one
-- Sparse PATCH on name, graph, enabled. D-66 atomic version-checked UPDATE.
-- `code` is IMMUTABLE — present in RETURNING, absent from SET + param list.
UPDATE flows
SET name       = COALESCE(sqlc.narg('name')::text,    name),
    graph      = COALESCE(sqlc.narg('graph')::jsonb,  graph),
    enabled    = COALESCE(sqlc.narg('enabled')::bool, enabled),
    version    = version + 1,
    updated_at = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, code, name, graph, enabled, version, created_at, updated_at;

-- name: LockFlowForPublish :one
-- SELECT ... FOR UPDATE inside the publish tx: the draft `version` is read +
-- the row locked so a concurrent UpdateFlow cannot bump the version (and change
-- the graph) between the handler's validate/compile and the version insert.
-- version mismatch under the lock => 409 (cross-AI HIGH-2). version bumps on
-- every graph PATCH, so a matching version guarantees the compiled graph is
-- still current.
SELECT version, enabled
FROM flows
WHERE id = $1 AND org_id = $2
FOR UPDATE;

-- name: SoftDeleteFlow :execrows
UPDATE flows
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;
