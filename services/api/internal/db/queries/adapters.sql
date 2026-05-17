-- Phase 3 Wave 1 catalog queries — CAT-06 adapters.
--
-- Adapter-specific shape (CAT-06):
--   * (Phase 04.1) `code TEXT NOT NULL` added — UNIQUE (org_id, code) is the
--     new primary user-facing identity; included in SELECT/INSERT/RETURNING.
--   * (Phase 04.1) `external_id TEXT NULL` added — adapters had neither pre-04.1.
--     Partial unique (org_id, external_id) WHERE external_id IS NOT NULL.
--   * config JSONB NOT NULL DEFAULT '{}' — sqlc emits []byte for pgx/v5;
--     handler marshals/unmarshals via encoding/json (Pitfall 9, H8).
--   * adapter_type TEXT NOT NULL.
--
-- See agents.sql for the shared per-entity rationale (D-62, D-65, D-66,
-- Codex C3 sparse-PATCH).
--
-- (Phase 04.1) UpdateAdapter does NOT mutate `code` — the param list excludes
--   it; the SET clause excludes it; the handler enforces immutability via
--   Layer 2. New per-entity GetAdapterByCode + UpsertAdapterByCode queries
--   authored for Phase 5 (Phase 04.1 does NOT invoke them).

-- name: InsertAdapter :one
INSERT INTO adapters (id, org_id, code, external_id, name, adapter_type, config, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, code, external_id, name, adapter_type, config, enabled, version, created_at, updated_at;

-- name: GetAdapter :one
SELECT id, org_id, code, external_id, name, adapter_type, config, enabled, version, created_at, updated_at
FROM adapters
WHERE id = $1 AND org_id = $2;

-- name: GetAdapterByIdAnyVersion :one
-- D-66 disambiguation probe.
SELECT id, org_id, code, external_id, name, adapter_type, config, enabled, version, created_at, updated_at
FROM adapters
WHERE id = $1 AND org_id = $2;

-- name: ListAdapters :many
SELECT id, org_id, code, external_id, name, adapter_type, config, enabled, version, created_at, updated_at
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
SELECT id, org_id, code, external_id, name, adapter_type, config, enabled, version, created_at, updated_at
FROM adapters
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateAdapter :one
-- D-66 + Codex C3 sparse-PATCH on external_id, name, adapter_type, config,
-- enabled. H7: adapters carries version per OQ-1/A4 spec amendment.
--
-- Phase 04.1 (D04_1-15): `code` is IMMUTABLE — present in RETURNING, absent
-- from SET clause and parameter list. external_id IS mutable (D04_1-07) —
-- NEW for adapters in Phase 04.1.
--
-- Phase 5 fix H2: empty-string-as-clear sentinel for external_id (see
-- agents.sql:UpdateAgent for the three-way rationale).
UPDATE adapters
SET external_id  = CASE
                     WHEN sqlc.narg('external_id')::text IS NULL THEN external_id
                     WHEN sqlc.narg('external_id')::text = ''    THEN NULL
                     ELSE sqlc.narg('external_id')::text
                   END,
    name         = COALESCE(sqlc.narg('name')::text,         name),
    adapter_type = COALESCE(sqlc.narg('adapter_type')::text, adapter_type),
    config       = COALESCE(sqlc.narg('config')::jsonb,      config),
    enabled      = COALESCE(sqlc.narg('enabled')::bool,      enabled),
    version = version + 1,
    updated_at   = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, code, external_id, name, adapter_type, config, enabled, version, created_at, updated_at;

-- name: SoftDeleteAdapter :execrows
UPDATE adapters
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;

-- name: GetAdapterByCode :one
-- Authored in Phase 04.1; invoked by Phase 5 upsert-by-code lookup (IMP-03).
-- Two-org isolation preserved: composite (org_id, code) match — cross-org
-- code probes return pgx.ErrNoRows (FOUND-08).
SELECT id, org_id, code, external_id, name, adapter_type, config, enabled, version, created_at, updated_at
FROM adapters
WHERE org_id = $1 AND code = $2;

-- name: UpsertAdapterByCode :one
-- Phase 5 bulk-import target (IMP-03). The ON CONFLICT path leaves code
-- untouched (it IS the conflict target). external_id can be (re)bound
-- on conflict. Phase 5 Wave 1 invokes this; Phase 04.1 just authors it.
INSERT INTO adapters (id, org_id, code, external_id, name, adapter_type, config, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (org_id, code) DO UPDATE
    SET external_id  = EXCLUDED.external_id,
        name         = EXCLUDED.name,
        adapter_type = EXCLUDED.adapter_type,
        config       = EXCLUDED.config,
        enabled      = EXCLUDED.enabled,
        version      = adapters.version + 1,
        updated_at   = NOW()
RETURNING id, org_id, code, external_id, name, adapter_type, config, enabled, version, created_at, updated_at;
