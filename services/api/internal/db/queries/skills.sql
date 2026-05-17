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
--
-- (Phase 04.1) `code TEXT NOT NULL` added; included in SELECT/INSERT/RETURNING.
--   UpdateX does NOT mutate `code` — the param list excludes it; the SET
--   clause excludes it; the handler enforces immutability via Layer 2.
--   New per-entity GetSkillByCode + UpsertSkillByCode queries authored for
--   Phase 5 (Phase 04.1 does NOT invoke them).

-- name: InsertSkill :one
INSERT INTO skills (id, org_id, code, external_id, name, description, skill_type, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, code, external_id, name, description, skill_type, enabled, version, created_at, updated_at;

-- name: GetSkill :one
SELECT id, org_id, code, external_id, name, description, skill_type, enabled, version, created_at, updated_at
FROM skills
WHERE id = $1 AND org_id = $2;

-- name: GetSkillByIdAnyVersion :one
-- D-66 disambiguation probe (404-vs-409 after a 0-row UpdateSkill).
SELECT id, org_id, code, external_id, name, description, skill_type, enabled, version, created_at, updated_at
FROM skills
WHERE id = $1 AND org_id = $2;

-- name: ListSkills :many
-- Default-list path (D-65): enabled = TRUE.
SELECT id, org_id, code, external_id, name, description, skill_type, enabled, version, created_at, updated_at
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
SELECT id, org_id, code, external_id, name, description, skill_type, enabled, version, created_at, updated_at
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
-- on mutable columns (Codex C3 iter 3): external_id, name, description,
-- skill_type, enabled.
--
-- Phase 04.1 (D04_1-15): `code` is IMMUTABLE — present in RETURNING, absent
-- from SET clause and parameter list. external_id IS mutable (D04_1-07).
UPDATE skills
SET external_id = COALESCE(sqlc.narg('external_id')::text, external_id),
    name        = COALESCE(sqlc.narg('name')::text,        name),
    description = COALESCE(sqlc.narg('description')::text, description),
    skill_type  = COALESCE(sqlc.narg('skill_type')::text,  skill_type),
    enabled     = COALESCE(sqlc.narg('enabled')::bool,     enabled),
    version = version + 1,
    updated_at  = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, code, external_id, name, description, skill_type, enabled, version, created_at, updated_at;

-- name: SoftDeleteSkill :execrows
-- Idempotent soft delete (CAT-09, D-65).
UPDATE skills
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;

-- name: GetSkillByCode :one
-- Authored in Phase 04.1; invoked by Phase 5 upsert-by-code lookup (IMP-03).
-- Two-org isolation preserved: composite (org_id, code) match — cross-org
-- code probes return pgx.ErrNoRows (FOUND-08).
SELECT id, org_id, code, external_id, name, description, skill_type, enabled, version, created_at, updated_at
FROM skills
WHERE org_id = $1 AND code = $2;

-- name: UpsertSkillByCode :one
-- Phase 5 bulk-import target (IMP-03). The ON CONFLICT path leaves code
-- untouched (it IS the conflict target). external_id can be (re)bound
-- on conflict. Phase 5 Wave 1 invokes this; Phase 04.1 just authors it.
INSERT INTO skills (id, org_id, code, external_id, name, description, skill_type, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (org_id, code) DO UPDATE
    SET external_id = EXCLUDED.external_id,
        name        = EXCLUDED.name,
        description = EXCLUDED.description,
        skill_type  = EXCLUDED.skill_type,
        enabled     = EXCLUDED.enabled,
        version     = skills.version + 1,
        updated_at  = NOW()
RETURNING id, org_id, code, external_id, name, description, skill_type, enabled, version, created_at, updated_at;

-- name: ResolveSkillCodes :many
-- D5-16 + D5-17 (Phase 5 bulk import, post-04.1): batch lookup of skill
-- UUIDs by `code`. Used by the agent row processor to resolve nested
-- skill references once per chunk via Phase 04.1's `code` column.
-- Returns (id, code) pairs; missing codes are absent from the result
-- set — handler distinguishes "unknown skill" by set difference
-- (mirror agent_skills.go SkillsPresentInOrg miss-detection pattern
-- from the file header comment in agent_skills.sql).
--
-- RESEARCH Pitfall 6 (N+1 lookup avoidance) is the trap this query
-- exists to eliminate. 50 rows × 3 skills each = 150 round trips
-- without batching; one ANY($2::text[]) call collapses it to 1 round
-- trip per chunk. The handler's row processor builds the input slice
-- by uniqifying all skill_codes referenced across the 50-row chunk
-- (D5-16 JSON shape `skills: [{skill_code, proficiency}, ...]` and
-- D5-17 CSV shape `skill_voice:7|skill_chat:9`).
--
-- SQLChecker compliance (D-02): top-level FROM skills (a tenant alias
-- per sqlcheck.go tenantTables map line 35) + literal `org_id = $1` in
-- WHERE — preflight is green. The org_id leads parameter ordering to
-- match the Phase 04.1 convention (GetSkillByCode line 99,
-- UpsertSkillByCode line 105 both put org_id as $1).
SELECT id, code
FROM skills
WHERE org_id = $1 AND code = ANY($2::text[]);
