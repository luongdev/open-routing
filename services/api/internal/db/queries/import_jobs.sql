-- Phase 5 Wave 1 — IMP-06 import_jobs CRUD + sweep queries.
--
-- Schema: migrations/000003_create_import_jobs.up.sql (denormalised org_id;
-- partial UNIQUE on (org_id, idempotency_key) WHERE idempotency_key IS NOT
-- NULL; partial sweep index on (updated_at) WHERE status='pending'; NO FK
-- to any catalog table per D-80 — app-layer probes only).
--
-- Lifecycle (D5-10):
--   1) InsertImportJob — handler INSERTs row with status='pending',
--      total_rows=N, idempotency_key (nullable narg). Happens BEFORE the
--      first chunk opens so admin GET sees the job mid-import.
--   2) Per-chunk savepoint loop runs (D5-09) — does NOT touch import_jobs.
--   3) FinaliseImportJob — handler UPDATEs status='completed'|'failed',
--      succeeded_rows, failed_rows, errors JSONB after last chunk commits.
--   4) GetImportJob — admin GET endpoint reads the finalised row.
--   5) LookupImportJobByIdempotencyKey — replay path for Idempotency-Key
--      hit (D5-13). Handler checks this BEFORE InsertImportJob.
--   6) SweepCrashedImportJobs — 1h-tick background goroutine (D5-11)
--      flips stranded pending rows (> 24h) to failed with a synthetic
--      server_crash error entry. ORG-AGNOSTIC by intent (sweep crosses
--      all orgs); the goroutine wraps ctx with
--      `db.WithBypass(ctx, "import_crash_sweep")` per Phase 4 STATE-07.
--
-- Hazard reminders:
--   * H3 (D-02) — every query mentions org_id in WHERE/INSERT/UPDATE so
--     the orgDB SQLChecker accepts it (tenantTables entry added in Plan
--     05-01 Wave 0; sqlcheck.go line 42). SweepCrashedImportJobs is the
--     LONE org-agnostic query in the entire codebase; it requires
--     WithBypass at the call site (Plan 05-04 sweep.go wires this).
--   * D-66 carry-forward — FinaliseImportJob does NOT use a version
--     check; the counter columns are monotonically increasing rather
--     than optimistic-locked, and the row is touched at most 3 times
--     (insert → finalise → maybe sweep). Cross-row mid-import races are
--     impossible because the handler holds the only writer goroutine
--     until finalise returns.
--   * Pitfall 6 (RESEARCH §F4) — Idempotency-Key flow is:
--     LookupImportJobByIdempotencyKey FIRST (replay if hit) →
--     InsertImportJob (new row) — the 23505 raised by the partial
--     UNIQUE on (org_id, idempotency_key) is a defensive backstop only.
--   * FOUND-08 isolation — GetImportJob's composite (id, org_id) WHERE
--     means a cross-org GET returns pgx.ErrNoRows; handler maps to 404
--     (NOT 403) so the existence of another org's job is never leaked.

-- name: InsertImportJob :one
-- D5-10 lifecycle step 1: row created BEFORE chunk 1 opens. idempotency_key
-- uses sqlc.narg so NULL is the cardinality for "no header passed" — the
-- partial UNIQUE on (org_id, idempotency_key) WHERE idempotency_key IS NOT
-- NULL ignores these rows so multiple unkeyed imports may coexist.
INSERT INTO import_jobs (
    id, org_id, entity_type, status, total_rows,
    succeeded_rows, failed_rows, errors, idempotency_key
)
VALUES ($1, $2, $3, 'pending', $4, 0, 0, NULL, sqlc.narg('idempotency_key'))
RETURNING id, org_id, entity_type, status, total_rows, succeeded_rows,
          failed_rows, errors, idempotency_key, created_at, updated_at;

-- name: FinaliseImportJob :one
-- D5-10 lifecycle step 3: writes terminal status + counters + errors JSONB
-- after the last chunk commits. No version check (D-66 carry-forward —
-- counters are monotonic, not optimistic-locked). WHERE composite (id,
-- org_id) preserves FOUND-08 isolation.
UPDATE import_jobs
SET status = $1,
    succeeded_rows = $2,
    failed_rows = $3,
    errors = $4,
    updated_at = NOW()
WHERE id = $5 AND org_id = $6
RETURNING id, org_id, entity_type, status, total_rows, succeeded_rows,
          failed_rows, errors, idempotency_key, created_at, updated_at;

-- name: GetImportJob :one
-- D5-10 lifecycle step 4: admin GET endpoint (GET
-- /v1/orgs/{org_id}/imports/{id}). Returns pgx.ErrNoRows when the row
-- does not exist OR belongs to another org (FOUND-08 isolation guarantee
-- — composite (id, org_id) match prevents cross-org lookup leakage).
SELECT id, org_id, entity_type, status, total_rows, succeeded_rows,
       failed_rows, errors, idempotency_key, created_at, updated_at
FROM import_jobs
WHERE id = $1 AND org_id = $2;

-- name: LookupImportJobByIdempotencyKey :one
-- D5-13 replay path: handler probes this BEFORE InsertImportJob when the
-- client passed Idempotency-Key. Hit → re-serialise the persisted job's
-- BulkImportResult (with idempotent_replay=true, succeeded=[] per the
-- v0.1 known limitation in D5-13). Miss → pgx.ErrNoRows → handler
-- proceeds with a fresh InsertImportJob persisting the key.
--
-- The 23505 raised by the partial UNIQUE on (org_id, idempotency_key) is
-- a defensive backstop for a TOCTOU race between this lookup and the
-- subsequent INSERT (extremely unlikely; admin operating two parallel
-- POSTs with the same UUIDv7 key within microseconds).
SELECT id, org_id, entity_type, status, total_rows, succeeded_rows,
       failed_rows, errors, idempotency_key, created_at, updated_at
FROM import_jobs
WHERE org_id = $1 AND idempotency_key = $2;

-- name: SweepCrashedImportJobs :many
-- D5-11 crash-recovery sweep: flips stranded pending rows (> 24h since
-- last touch) to failed with a synthetic server_crash error entry. The
-- sweep is INTENTIONALLY org-agnostic — one goroutine ticks across all
-- orgs every hour (mirror Phase 4 STATE-07 ListExpiringWrapUps pattern).
--
-- *** UNIQUE SQLChecker EXCEPTION ***
-- This is the SOLE query in the codebase whose WHERE clause omits
-- org_id. The orgDB SQLChecker (D-02) WOULD REJECT the query in normal
-- contexts. Plan 05-04 sweep.go wraps the call site with
-- `db.WithBypass(ctx, "import_crash_sweep")` per Phase 4 STATE-07
-- (state/ttl.go:179 "wrapup_sweeper.safety" pattern) — only that single
-- call site passes preflight. RETURNING includes org_id so the goroutine
-- can structured-log the per-org sweep deltas at slog.Warn level for
-- forensics (event=import_crash_sweep, mirror STATE-07 telemetry).
--
-- The partial sweep index `ix_import_jobs_pending_updated (updated_at)
-- WHERE status='pending'` (migration 000003) keeps this UPDATE O(pending)
-- rather than O(total) — on a healthy system pending rows are bounded by
-- concurrent imports.
UPDATE import_jobs
SET status = 'failed',
    errors = '[{"row":0,"error":"import_failed","reason":"server_crash"}]'::jsonb,
    updated_at = NOW()
WHERE status = 'pending'
  AND updated_at < NOW() - INTERVAL '24 hours'
RETURNING id, org_id;
