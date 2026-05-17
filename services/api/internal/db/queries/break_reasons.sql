-- Phase 3 Wave 1 catalog queries — CAT-07 break_reasons.
--
-- Break-reason-specific shape (CAT-07):
--   * (Phase 04.1) `code TEXT NOT NULL` added — UNIQUE (org_id, code) is the
--     new primary user-facing identity (D04_1-09 dropped the pre-04.1
--     UNIQUE(org_id, name) constraint). Included in SELECT/INSERT/RETURNING.
--   * (Phase 04.1) `external_id TEXT NULL` added — break_reasons had neither
--     pre-04.1. Partial unique (org_id, external_id) WHERE external_id IS NOT NULL.
--   * routable BOOLEAN NOT NULL DEFAULT FALSE.
--   * display_order INTEGER NOT NULL DEFAULT 0 — drives the ordered list
--     view used by the agent-status picker.
--
-- See agents.sql for the shared per-entity rationale.
--
-- (Phase 04.1) UpdateBreakReason does NOT mutate `code` — the param list
--   excludes it; the SET clause excludes it; the handler enforces immutability
--   via Layer 2. New per-entity GetBreakReasonByCode + UpsertBreakReasonByCode
--   queries authored for Phase 5 (Phase 04.1 does NOT invoke them).

-- name: InsertBreakReason :one
INSERT INTO break_reasons (id, org_id, code, external_id, name, routable, display_order, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, code, external_id, name, routable, display_order, enabled, version, created_at, updated_at;

-- name: GetBreakReason :one
SELECT id, org_id, code, external_id, name, routable, display_order, enabled, version, created_at, updated_at
FROM break_reasons
WHERE id = $1 AND org_id = $2;

-- name: GetBreakReasonByIdAnyVersion :one
-- D-66 disambiguation probe.
SELECT id, org_id, code, external_id, name, routable, display_order, enabled, version, created_at, updated_at
FROM break_reasons
WHERE id = $1 AND org_id = $2;

-- name: ListBreakReasons :many
SELECT id, org_id, code, external_id, name, routable, display_order, enabled, version, created_at, updated_at
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
SELECT id, org_id, code, external_id, name, routable, display_order, enabled, version, created_at, updated_at
FROM break_reasons
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateBreakReason :one
-- D-66 + Codex C3 sparse-PATCH on external_id, name, routable,
-- display_order, enabled.
--
-- Phase 04.1 (D04_1-15): `code` is IMMUTABLE — present in RETURNING, absent
-- from SET clause and parameter list. external_id IS mutable (D04_1-07) —
-- NEW for break_reasons in Phase 04.1.
--
-- Phase 5 fix H2: empty-string-as-clear sentinel for external_id (see
-- agents.sql:UpdateAgent for the three-way rationale).
UPDATE break_reasons
SET external_id   = CASE
                      WHEN sqlc.narg('external_id')::text IS NULL THEN external_id
                      WHEN sqlc.narg('external_id')::text = ''    THEN NULL
                      ELSE sqlc.narg('external_id')::text
                    END,
    name          = COALESCE(sqlc.narg('name')::text,          name),
    routable      = COALESCE(sqlc.narg('routable')::bool,      routable),
    display_order = COALESCE(sqlc.narg('display_order')::int,  display_order),
    enabled       = COALESCE(sqlc.narg('enabled')::bool,       enabled),
    version = version + 1,
    updated_at    = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, code, external_id, name, routable, display_order, enabled, version, created_at, updated_at;

-- name: SoftDeleteBreakReason :execrows
UPDATE break_reasons
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;

-- name: GetBreakReasonForState :one
-- D-76 cross-row probe — used by PatchAgentStatus (STATE-04 422
-- invalid_reference path) and IsRoutable callers (STATE-10). Returns
-- the routable flag when the reason exists + is enabled in the caller's
-- org; pgx.ErrNoRows otherwise. Same-org enforcement is the FOUND-08
-- invariant (cross-org reason ids return ErrNoRows → handler maps to
-- 422 invalid_reference per D-75). Combined probe per RESEARCH
-- §Pattern 5: one query covers BOTH STATE-04 existence check AND
-- STATE-10 routable scalar; keeps the sqlc surface minimal.
--
-- Phase 04.1 preserves this query verbatim: it returns the scalar
-- `routable bool` and is consumed by services/api/internal/state/
-- agent_states.go (outside the catalog handler wave boundary). Adding
-- code/external_id to the SELECT projection would change the emitted
-- return type from `bool` to a struct and break that caller — out of
-- scope for Plan 03 per D04_1-DEFER-03 (agent_states wave boundary).
SELECT routable
FROM break_reasons
WHERE id = $1 AND org_id = $2 AND enabled = TRUE
LIMIT 1;

-- name: GetBreakReasonByCode :one
-- Authored in Phase 04.1; invoked by Phase 5 upsert-by-code lookup (IMP-03).
-- Two-org isolation preserved: composite (org_id, code) match — cross-org
-- code probes return pgx.ErrNoRows (FOUND-08).
SELECT id, org_id, code, external_id, name, routable, display_order, enabled, version, created_at, updated_at
FROM break_reasons
WHERE org_id = $1 AND code = $2;

-- name: UpsertBreakReasonByCode :one
-- Phase 5 bulk-import target (IMP-03). The ON CONFLICT path leaves code
-- untouched (it IS the conflict target). external_id can be (re)bound
-- on conflict. Phase 5 Wave 1 invokes this; Phase 04.1 just authors it.
INSERT INTO break_reasons (id, org_id, code, external_id, name, routable, display_order, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (org_id, code) DO UPDATE
    SET external_id   = EXCLUDED.external_id,
        name          = EXCLUDED.name,
        routable      = EXCLUDED.routable,
        display_order = EXCLUDED.display_order,
        enabled       = EXCLUDED.enabled,
        version       = break_reasons.version + 1,
        updated_at    = NOW()
RETURNING id, org_id, code, external_id, name, routable, display_order, enabled, version, created_at, updated_at;
