-- Phase 5 — Bulk import job persistence (IMP-06).
--
-- Adds the `import_jobs` table that records one row per POST to
-- /v1/orgs/{org_id}/catalog/import. The row is INSERTed with
-- status='pending' before the first chunk opens (D5-10), then UPDATEd to
-- 'completed' (or 'failed') after the last chunk commits. The 24h
-- crash-recovery sweep (D5-11) flips stranded pending rows to 'failed'
-- with a synthetic error entry.
--
-- New migration (NOT amending 000002) per Phase 5 RESEARCH §Open Q4:
-- import_jobs is a Phase-5-owned table that does NOT belong to the
-- catalog v0.1 identity schema; the editable-migration policy (D-61)
-- applies to 000002 only.
--
-- Conventions:
--   * org_id UUID NOT NULL is DENORMALIZED on this table so SQLChecker
--     (D-02) sees the column on every query. Phase 5 row processors
--     route through OrgDB (D-01); the sweep goroutine uses
--     `db.WithBypass(ctx, "import_crash_sweep")` per Phase 4 STATE-07.
--   * NO FK to any catalog table (D-80 invariant — app-layer probes only).
--   * NO `enabled` column — `import_jobs` is event-history, not a
--     soft-deletable entity (mirror Phase 4 agent_states deviation).
--   * NO `version` column — counters monotonically increase; each row is
--     touched at most 3 times (INSERT → finalise → maybe sweep).
--   * `idempotency_key TEXT NULL` with partial UNIQUE on (org_id,
--     idempotency_key) WHERE NOT NULL (D5-13) — enforces uniqueness only
--     when the client supplied the header.

BEGIN;

CREATE TABLE import_jobs (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    entity_type     TEXT NOT NULL
                      CHECK (entity_type IN ('agents','skills','queues','channels','adapters','break_reasons')),
    status          TEXT NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','completed','failed')),
    total_rows      INTEGER NOT NULL DEFAULT 0,
    succeeded_rows  INTEGER NOT NULL DEFAULT 0,
    failed_rows     INTEGER NOT NULL DEFAULT 0,
    errors          JSONB,
    idempotency_key TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Admin list (forward-compat). Cursor pagination convention (D-63).
CREATE INDEX ix_import_jobs_org_created
    ON import_jobs (org_id, created_at DESC, id DESC);

-- D5-11 crash-recovery sweep partial index — scans only pending rows so
-- the sweep query is O(pending) rather than O(total). On a healthy
-- system pending row count is bounded by concurrent imports.
CREATE INDEX ix_import_jobs_pending_updated
    ON import_jobs (updated_at)
    WHERE status = 'pending';

-- D5-13 idempotency partial UNIQUE — enforces (org_id, idempotency_key)
-- only when the client passed the header. NULL idempotency_key rows
-- are NOT covered by this constraint and may coexist freely.
CREATE UNIQUE INDEX uq_import_jobs_org_idempotency
    ON import_jobs (org_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

COMMIT;
