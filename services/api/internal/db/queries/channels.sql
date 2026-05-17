-- Phase 3 Wave 1 catalog queries — CAT-05 channels.
--
-- Channel-specific fields:
--   * channel_type TEXT NOT NULL.
--   * default_queue_id UUID NULL (nullable; FK semantics deferred to v0.2
--     per D-76; application-layer validation only).
--
-- Spec semantics: PATCH default_queue_id = null is INDISTINGUISHABLE from
-- omission at the oapi-codegen pointer-type layer (both produce nil). The
-- COALESCE pattern preserves the current value when the handler passes nil.
-- A dedicated "unset default_queue_id" endpoint is deferred to v0.2.

-- name: InsertChannel :one
INSERT INTO channels (id, org_id, external_id, name, channel_type, default_queue_id, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, org_id, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at;

-- name: GetChannel :one
-- Keep unfiltered by version/enabled: update handlers reuse this for D-66
-- 404-vs-409 disambiguation after a version-checked UPDATE returns 0 rows.
SELECT id, org_id, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at
FROM channels
WHERE id = $1 AND org_id = $2;

-- name: ListChannels :many
SELECT id, org_id, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at
FROM channels
WHERE org_id = $1
  AND enabled = TRUE
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: ListChannelsIncludingDisabled :many
SELECT id, org_id, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at
FROM channels
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateChannel :one
-- D-66 + Codex C3 sparse-PATCH on name, channel_type, default_queue_id,
-- enabled. H7: channels carries version per OQ-1/A4 spec amendment.
UPDATE channels
SET name             = COALESCE(sqlc.narg('name')::text,             name),
    channel_type     = COALESCE(sqlc.narg('channel_type')::text,     channel_type),
    default_queue_id = COALESCE(sqlc.narg('default_queue_id')::uuid, default_queue_id),
    enabled          = COALESCE(sqlc.narg('enabled')::bool,          enabled),
    version = version + 1,
    updated_at       = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at;

-- name: SoftDeleteChannel :execrows
UPDATE channels
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;
