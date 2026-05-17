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
--
-- (Phase 04.1) `code TEXT NOT NULL` added; included in SELECT/INSERT/RETURNING.
--   UpdateChannel does NOT mutate `code` — the param list excludes it; the
--   SET clause excludes it; the handler enforces immutability via Layer 2.
--   New per-entity GetChannelByCode + UpsertChannelByCode queries authored
--   for Phase 5 (Phase 04.1 does NOT invoke them).

-- name: InsertChannel :one
INSERT INTO channels (id, org_id, code, external_id, name, channel_type, default_queue_id, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, code, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at;

-- name: GetChannel :one
SELECT id, org_id, code, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at
FROM channels
WHERE id = $1 AND org_id = $2;

-- name: GetChannelByIdAnyVersion :one
-- D-66 disambiguation probe.
SELECT id, org_id, code, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at
FROM channels
WHERE id = $1 AND org_id = $2;

-- name: ListChannels :many
SELECT id, org_id, code, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at
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
SELECT id, org_id, code, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at
FROM channels
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateChannel :one
-- D-66 + Codex C3 sparse-PATCH on external_id, name, channel_type,
-- default_queue_id, enabled. H7: channels carries version per OQ-1/A4
-- spec amendment.
--
-- Phase 04.1 (D04_1-15): `code` is IMMUTABLE — present in RETURNING, absent
-- from SET clause and parameter list. external_id IS mutable (D04_1-07).
UPDATE channels
SET external_id      = COALESCE(sqlc.narg('external_id')::text,      external_id),
    name             = COALESCE(sqlc.narg('name')::text,             name),
    channel_type     = COALESCE(sqlc.narg('channel_type')::text,     channel_type),
    default_queue_id = COALESCE(sqlc.narg('default_queue_id')::uuid, default_queue_id),
    enabled          = COALESCE(sqlc.narg('enabled')::bool,          enabled),
    version = version + 1,
    updated_at       = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, code, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at;

-- name: SoftDeleteChannel :execrows
UPDATE channels
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;

-- name: GetChannelByCode :one
-- Authored in Phase 04.1; invoked by Phase 5 upsert-by-code lookup (IMP-03).
-- Two-org isolation preserved: composite (org_id, code) match — cross-org
-- code probes return pgx.ErrNoRows (FOUND-08).
SELECT id, org_id, code, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at
FROM channels
WHERE org_id = $1 AND code = $2;

-- name: UpsertChannelByCode :one
-- Phase 5 bulk-import target (IMP-03). The ON CONFLICT path leaves code
-- untouched (it IS the conflict target). external_id can be (re)bound
-- on conflict. Phase 5 Wave 1 invokes this; Phase 04.1 just authors it.
INSERT INTO channels (id, org_id, code, external_id, name, channel_type, default_queue_id, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (org_id, code) DO UPDATE
    SET external_id      = EXCLUDED.external_id,
        name             = EXCLUDED.name,
        channel_type     = EXCLUDED.channel_type,
        default_queue_id = EXCLUDED.default_queue_id,
        enabled          = EXCLUDED.enabled,
        version          = channels.version + 1,
        updated_at       = NOW()
RETURNING id, org_id, code, external_id, name, channel_type, default_queue_id, enabled, version, created_at, updated_at;
