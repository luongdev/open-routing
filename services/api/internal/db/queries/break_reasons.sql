-- Phase 3 Wave 1 catalog queries — CAT-07 break_reasons.
--
-- Break-reason-specific shape (CAT-07):
--   * NO external_id — UNIQUE is (org_id, name).
--   * routable BOOLEAN NOT NULL DEFAULT FALSE.
--   * display_order INTEGER NOT NULL DEFAULT 0 — drives the ordered list
--     view used by the agent-status picker.
--
-- See agents.sql for the shared per-entity rationale.

-- name: InsertBreakReason :one
INSERT INTO break_reasons (id, org_id, name, routable, display_order, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, name, routable, display_order, enabled, version, created_at, updated_at;

-- name: GetBreakReason :one
SELECT id, org_id, name, routable, display_order, enabled, version, created_at, updated_at
FROM break_reasons
WHERE id = $1 AND org_id = $2;

-- name: GetBreakReasonByIdAnyVersion :one
-- D-66 disambiguation probe.
SELECT id, org_id, name, routable, display_order, enabled, version, created_at, updated_at
FROM break_reasons
WHERE id = $1 AND org_id = $2;

-- name: ListBreakReasons :many
SELECT id, org_id, name, routable, display_order, enabled, version, created_at, updated_at
FROM break_reasons
WHERE org_id = $1
  AND enabled = TRUE
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: ListBreakReasonsIncludingDisabled :many
SELECT id, org_id, name, routable, display_order, enabled, version, created_at, updated_at
FROM break_reasons
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateBreakReason :one
-- D-66 + Codex C3 sparse-PATCH on name, routable, display_order, enabled.
UPDATE break_reasons
SET name          = COALESCE(sqlc.narg('name')::text,          name),
    routable      = COALESCE(sqlc.narg('routable')::bool,      routable),
    display_order = COALESCE(sqlc.narg('display_order')::int,  display_order),
    enabled       = COALESCE(sqlc.narg('enabled')::bool,       enabled),
    version = version + 1,
    updated_at    = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, name, routable, display_order, enabled, version, created_at, updated_at;

-- name: SoftDeleteBreakReason :execrows
UPDATE break_reasons
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;
