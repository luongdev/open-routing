-- Phase 3 Wave 1 catalog queries — CAT-04 queues.
--
-- Queue-specific fields:
--   * channel_types TEXT[] — sqlc emits []string for pgx/v5 (Pitfall: empty
--     arrays must default to {} via the migration NOT NULL DEFAULT '{}'::TEXT[]).
--   * priority INTEGER, acw_sec INTEGER — non-null, defaults via migration.
--
-- Plus the FK probe used by channels.default_queue_id validation (D-76).

-- name: InsertQueue :one
INSERT INTO queues (id, org_id, external_id, name, channel_types, priority, acw_sec, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at;

-- name: GetQueue :one
-- Keep unfiltered by version/enabled: update handlers reuse this for D-66
-- 404-vs-409 disambiguation after a version-checked UPDATE returns 0 rows.
SELECT id, org_id, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at
FROM queues
WHERE id = $1 AND org_id = $2;

-- name: ListQueues :many
SELECT id, org_id, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at
FROM queues
WHERE org_id = $1
  AND enabled = TRUE
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: ListQueuesIncludingDisabled :many
SELECT id, org_id, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at
FROM queues
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateQueue :one
-- Sparse-PATCH semantics (Codex C3) on name, channel_types, priority,
-- acw_sec, enabled. D-66 atomic version-checked UPDATE.
UPDATE queues
SET name          = COALESCE(sqlc.narg('name')::text,           name),
    channel_types = COALESCE(sqlc.narg('channel_types')::text[], channel_types),
    priority      = COALESCE(sqlc.narg('priority')::int,         priority),
    acw_sec       = COALESCE(sqlc.narg('acw_sec')::int,          acw_sec),
    enabled       = COALESCE(sqlc.narg('enabled')::bool,         enabled),
    version = version + 1,
    updated_at    = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at;

-- name: SoftDeleteQueue :execrows
UPDATE queues
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;

-- name: QueueExistsAndEnabledInOrg :one
-- D-76 FK probe used by CreateChannel / UpdateChannel to validate
-- channels.default_queue_id references an enabled queue in the same org.
-- Returns one row (the constant 1) when the queue exists and is enabled;
-- returns zero rows (handler maps pgx.ErrNoRows → 422 invalid_reference)
-- otherwise. Structured as a top-level SELECT FROM queues so the orgDB
-- SQLChecker sees a tenant alias.
SELECT 1::int AS exists_in_org
FROM queues
WHERE id = $1 AND org_id = $2 AND enabled = TRUE
LIMIT 1;
