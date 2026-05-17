-- Phase 3 Wave 1 catalog queries — CAT-01 agents.
--
-- Layout per D-62 (one file per entity). The query set per entity is
-- canonical (Insert / Get / GetByIdAnyVersion / List / ListIncludingDisabled
-- / Update / SoftDelete) so handler code in Wave 3 can be generated from
-- a shared template.
--
-- Hazard reminders:
--   * H3 — every query mentions org_id in WHERE/INSERT/UPDATE/DELETE so
--     the orgDB SQLChecker accepts it (D-02).
--   * Pitfall 3a — cursor + name params use sqlc.narg('cursor_at') so the
--     IS NULL branch fires; positional $N would resolve to a non-pointer
--     pgtype field and the null branch would never short-circuit.
--   * Pitfall 3b (Codex C3 iter 3) — UpdateAgent uses COALESCE(sqlc.narg
--     ('col'), col) for sparse-PATCH semantics. A PATCH that sends only
--     {version, name} must NOT zero-out email/enabled; the handler passes
--     NULL for unset fields and COALESCE preserves the current value.
--   * D-66 — UPDATE returns the row via RETURNING; 0 rows means either
--     row missing (404) or version mismatched (409). Handler issues
--     GetAgentByIdAnyVersion to disambiguate.
--   * D-65 — two list variants: default omits soft-deleted rows; the
--     IncludingDisabled variant powers ?include_disabled=true.
--   * (Phase 04.1) `code TEXT NOT NULL` added; included in SELECT/INSERT/RETURNING.
--     UpdateX does NOT mutate `code` — the param list excludes it; the SET
--     clause excludes it; the handler enforces immutability via Layer 2.
--     New per-entity GetXByCode + UpsertXByCode queries authored for Phase 5
--     (Phase 04.1 does NOT invoke them).

-- name: InsertAgent :one
INSERT INTO agents (id, org_id, code, external_id, name, email, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, org_id, code, external_id, name, email, enabled, version, created_at, updated_at;

-- name: GetAgent :one
-- Single-row lookup by (id, org_id). Returns pgx.ErrNoRows when the row
-- does not exist or belongs to another org (FOUND-08 isolation guarantee).
SELECT id, org_id, code, external_id, name, email, enabled, version, created_at, updated_at
FROM agents
WHERE id = $1 AND org_id = $2;

-- name: GetAgentByIdAnyVersion :one
-- D-66 disambiguation probe. UpdateAgent returns 0 rows when either the
-- row does not exist (404) or the row exists with a different version
-- (409). Handler calls this after a 0-row UPDATE to choose the response
-- code. Same body as GetAgent — separate name keeps the intent grep-able
-- in the codebase.
SELECT id, org_id, code, external_id, name, email, enabled, version, created_at, updated_at
FROM agents
WHERE id = $1 AND org_id = $2;

-- name: ListAgents :many
-- Default-list path (D-65): enabled = TRUE filter applied unconditionally.
-- Cursor pagination per D-63: ordered by (created_at DESC, id DESC) so the
-- composite key is monotonically decreasing and stable under deletes (the
-- cursor anchor row may be soft-deleted but cursor still resolves rows
-- older than it). Page size limit caller-provided (handler enforces 1..100
-- per D-67) and includes the N+1 sentinel.
SELECT id, org_id, code, external_id, name, email, enabled, version, created_at, updated_at
FROM agents
WHERE org_id = $1
  AND enabled = TRUE
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: ListAgentsIncludingDisabled :many
-- ?include_disabled=true (CAT-09) path: same shape as ListAgents but
-- omits the enabled = TRUE filter so soft-deleted rows surface to the
-- caller. Plan-cacheable as a distinct prepared statement per D-65.
SELECT id, org_id, code, external_id, name, email, enabled, version, created_at, updated_at
FROM agents
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateAgent :one
-- Atomic version-checked UPDATE with sparse-PATCH semantics (D-66, Codex
-- C3 iter 3). COALESCE(sqlc.narg('field'), field) preserves the current
-- column value when the handler passes NULL. Required because PATCH may
-- omit optional fields and oapi-codegen renders omitted pointer types as
-- nil → sqlc renders NULL → without COALESCE the UPDATE would zero out
-- email/enabled. 0 rows returned → handler issues GetAgentByIdAnyVersion
-- to choose 404 (no row) vs 409 (version mismatched).
--
-- Phase 04.1 (D04_1-15): `code` is IMMUTABLE — it appears in RETURNING
-- (so handlers can render it on the response) but NOT in the SET clause
-- nor the parameter list. The handler enforces request-side immutability
-- via Layer 2 (validateImmutableCode); the absence here is the structural
-- backstop so the COALESCE sparse-PATCH pattern cannot silently mutate
-- code. external_id IS mutable (D04_1-07).
UPDATE agents
SET external_id = COALESCE(sqlc.narg('external_id')::text, external_id),
    name        = COALESCE(sqlc.narg('name')::text,        name),
    email       = COALESCE(sqlc.narg('email')::text,       email),
    enabled     = COALESCE(sqlc.narg('enabled')::bool,     enabled),
    version = version + 1,
    updated_at  = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, code, external_id, name, email, enabled, version, created_at, updated_at;

-- name: SoftDeleteAgent :execrows
-- Idempotent soft delete (D-65, CAT-09). WHERE enabled = TRUE means
-- re-deleting an already-soft-deleted row returns 0 rows → handler maps
-- 0 rows to 404 Not Found. Bumps updated_at so the audit timestamp
-- reflects the soft-delete moment.
UPDATE agents
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;

-- name: GetAgentByCode :one
-- Authored in Phase 04.1; invoked by Phase 5 upsert-by-code lookup (IMP-03).
-- Two-org isolation preserved: composite (org_id, code) match — cross-org
-- code probes return pgx.ErrNoRows (FOUND-08).
SELECT id, org_id, code, external_id, name, email, enabled, version, created_at, updated_at
FROM agents
WHERE org_id = $1 AND code = $2;

-- name: UpsertAgentByCode :one
-- Phase 5 bulk-import target (IMP-03). The ON CONFLICT path leaves code
-- untouched (it IS the conflict target). external_id can be (re)bound
-- on conflict. Phase 5 Wave 1 invokes this; Phase 04.1 just authors it.
INSERT INTO agents (id, org_id, code, external_id, name, email, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (org_id, code) DO UPDATE
    SET external_id = EXCLUDED.external_id,
        name        = EXCLUDED.name,
        email       = EXCLUDED.email,
        enabled     = EXCLUDED.enabled,
        version     = agents.version + 1,
        updated_at  = NOW()
RETURNING id, org_id, code, external_id, name, email, enabled, version, created_at, updated_at;
