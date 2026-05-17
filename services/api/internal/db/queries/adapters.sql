-- Phase 3 Wave 1 catalog queries — CAT-06 adapters.
--
-- Adapter-specific shape (CAT-06):
--   * NO external_id — adapter UNIQUE is just PRIMARY KEY (id).
--   * config JSONB NOT NULL DEFAULT '{}' — sqlc emits []byte for pgx/v5;
--     handler marshals/unmarshals via encoding/json (Pitfall 9, H8).
--   * adapter_type TEXT NOT NULL.
--
-- See agents.sql for the shared per-entity rationale (D-62, D-65, D-66,
-- Codex C3 sparse-PATCH).

-- name: InsertAdapter :one
INSERT INTO adapters (id, org_id, name, adapter_type, config, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, name, adapter_type, config, enabled, version, created_at, updated_at;

-- name: GetAdapter :one
-- Keep unfiltered by version/enabled: update handlers reuse this for D-66
-- 404-vs-409 disambiguation after a version-checked UPDATE returns 0 rows.
SELECT id, org_id, name, adapter_type, config, enabled, version, created_at, updated_at
FROM adapters
WHERE id = $1 AND org_id = $2;

-- name: ListAdapters :many
SELECT id, org_id, name, adapter_type, config, enabled, version, created_at, updated_at
FROM adapters
WHERE org_id = $1
  AND enabled = TRUE
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: ListAdaptersIncludingDisabled :many
SELECT id, org_id, name, adapter_type, config, enabled, version, created_at, updated_at
FROM adapters
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateAdapter :one
-- D-66 + Codex C3 sparse-PATCH on name, adapter_type, config, enabled.
-- H7: adapters carries version per OQ-1/A4 spec amendment.
UPDATE adapters
SET name         = COALESCE(sqlc.narg('name')::text,         name),
    adapter_type = COALESCE(sqlc.narg('adapter_type')::text, adapter_type),
    config       = COALESCE(sqlc.narg('config')::jsonb,      config),
    enabled      = COALESCE(sqlc.narg('enabled')::bool,      enabled),
    version = version + 1,
    updated_at   = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, name, adapter_type, config, enabled, version, created_at, updated_at;

-- name: SoftDeleteAdapter :execrows
UPDATE adapters
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;
