-- Phase 3 Wave 1 catalog queries — CAT-02 skills.
--
-- Same per-entity layout as agents.sql (D-62). Fields specific to skills:
--   * description TEXT NULL — sqlc emits pgtype.Text for non-null types,
--     and with emit_pointers_for_null_types: true the field becomes *string
--     for nullable columns. Handler passes a string pointer; pgx writes NULL
--     when the pointer is nil.
--   * skill_type TEXT NOT NULL.
--
-- See agents.sql for the COALESCE / sparse-PATCH / D-66 / D-65 / Pitfall 3
-- rationale that applies identically here.
--
-- Spec semantics: PATCH `description = null` is INDISTINGUISHABLE from
-- omission at the oapi-codegen pointer-type layer (both produce nil). The
-- COALESCE pattern preserves the current value when the handler passes
-- nil — a caller CANNOT clear a previously-set description back to NULL
-- through PATCH in v0.1. A dedicated "unset description" endpoint or a
-- distinguishable sentinel (e.g., empty string) is deferred to v0.2.

-- name: InsertSkill :one
INSERT INTO skills (id, org_id, external_id, name, description, skill_type, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, org_id, external_id, name, description, skill_type, enabled, version, created_at, updated_at;

-- name: GetSkill :one
SELECT id, org_id, external_id, name, description, skill_type, enabled, version, created_at, updated_at
FROM skills
WHERE id = $1 AND org_id = $2;

-- name: GetSkillByIdAnyVersion :one
-- D-66 disambiguation probe (404-vs-409 after a 0-row UpdateSkill).
SELECT id, org_id, external_id, name, description, skill_type, enabled, version, created_at, updated_at
FROM skills
WHERE id = $1 AND org_id = $2;

-- name: ListSkills :many
-- Default-list path (D-65): enabled = TRUE.
SELECT id, org_id, external_id, name, description, skill_type, enabled, version, created_at, updated_at
FROM skills
WHERE org_id = $1
  AND enabled = TRUE
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: ListSkillsIncludingDisabled :many
-- ?include_disabled=true (CAT-09) path.
SELECT id, org_id, external_id, name, description, skill_type, enabled, version, created_at, updated_at
FROM skills
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateSkill :one
-- D-66 atomic version-checked UPDATE. Sparse-PATCH semantics via COALESCE
-- on mutable columns (Codex C3 iter 3): name, description, skill_type, enabled.
UPDATE skills
SET name        = COALESCE(sqlc.narg('name')::text,        name),
    description = COALESCE(sqlc.narg('description')::text, description),
    skill_type  = COALESCE(sqlc.narg('skill_type')::text,  skill_type),
    enabled     = COALESCE(sqlc.narg('enabled')::bool,     enabled),
    version = version + 1,
    updated_at  = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, external_id, name, description, skill_type, enabled, version, created_at, updated_at;

-- name: SoftDeleteSkill :execrows
-- Idempotent soft delete (CAT-09, D-65).
UPDATE skills
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;
