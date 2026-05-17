-- Phase 3 Wave 1 catalog queries — CAT-04 queues.
--
-- Queue-specific fields:
--   * channel_types TEXT[] — sqlc emits []string for pgx/v5 (Pitfall: empty
--     arrays must default to {} via the migration NOT NULL DEFAULT '{}'::TEXT[]).
--   * priority INTEGER, acw_sec INTEGER — non-null, defaults via migration.
--
-- Plus the FK probe used by channels.default_queue_id validation (D-76).
--
-- (Phase 04.1) `code TEXT NOT NULL` added; included in SELECT/INSERT/RETURNING.
--   UpdateQueue does NOT mutate `code` — the param list excludes it; the SET
--   clause excludes it; the handler enforces immutability via Layer 2.
--   New per-entity GetQueueByCode + UpsertQueueByCode queries authored for
--   Phase 5 (Phase 04.1 does NOT invoke them).

-- name: InsertQueue :one
INSERT INTO queues (id, org_id, code, external_id, name, channel_types, priority, acw_sec, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, org_id, code, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at;

-- name: GetQueue :one
SELECT id, org_id, code, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at
FROM queues
WHERE id = $1 AND org_id = $2;

-- name: GetQueueByIdAnyVersion :one
-- D-66 disambiguation probe.
SELECT id, org_id, code, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at
FROM queues
WHERE id = $1 AND org_id = $2;

-- name: ListQueues :many
SELECT id, org_id, code, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at
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
SELECT id, org_id, code, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at
FROM queues
WHERE org_id = $1
  AND (sqlc.narg('cursor_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
  AND (sqlc.narg('name_filter')::text IS NULL
       OR lower(name) LIKE '%' || lower(sqlc.narg('name_filter')::text) || '%')
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateQueue :one
-- Sparse-PATCH semantics (Codex C3) on external_id, name, channel_types,
-- priority, acw_sec, enabled. D-66 atomic version-checked UPDATE.
--
-- Phase 04.1 (D04_1-15): `code` is IMMUTABLE — present in RETURNING, absent
-- from SET clause and parameter list. external_id IS mutable (D04_1-07).
UPDATE queues
SET external_id   = COALESCE(sqlc.narg('external_id')::text,    external_id),
    name          = COALESCE(sqlc.narg('name')::text,           name),
    channel_types = COALESCE(sqlc.narg('channel_types')::text[], channel_types),
    priority      = COALESCE(sqlc.narg('priority')::int,         priority),
    acw_sec       = COALESCE(sqlc.narg('acw_sec')::int,          acw_sec),
    enabled       = COALESCE(sqlc.narg('enabled')::bool,         enabled),
    version = version + 1,
    updated_at    = NOW()
WHERE id = sqlc.arg('id')
  AND org_id = sqlc.arg('org_id')
  AND version = sqlc.arg('expected_version')
RETURNING id, org_id, code, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at;

-- name: SoftDeleteQueue :execrows
UPDATE queues
SET enabled = FALSE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND enabled = TRUE;

-- name: GetQueueByCode :one
-- Authored in Phase 04.1; invoked by Phase 5 upsert-by-code lookup (IMP-03).
-- Two-org isolation preserved: composite (org_id, code) match — cross-org
-- code probes return pgx.ErrNoRows (FOUND-08).
SELECT id, org_id, code, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at
FROM queues
WHERE org_id = $1 AND code = $2;

-- name: UpsertQueueByCode :one
-- Phase 5 bulk-import target (IMP-03). The ON CONFLICT path leaves code
-- untouched (it IS the conflict target). external_id can be (re)bound
-- on conflict. Phase 5 Wave 1 invokes this; Phase 04.1 just authors it.
INSERT INTO queues (id, org_id, code, external_id, name, channel_types, priority, acw_sec, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (org_id, code) DO UPDATE
    SET external_id   = EXCLUDED.external_id,
        name          = EXCLUDED.name,
        channel_types = EXCLUDED.channel_types,
        priority      = EXCLUDED.priority,
        acw_sec       = EXCLUDED.acw_sec,
        enabled       = EXCLUDED.enabled,
        version       = queues.version + 1,
        updated_at    = NOW()
RETURNING id, org_id, code, external_id, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at;

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
