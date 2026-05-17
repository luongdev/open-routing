-- Phase 5 — Bulk import job persistence rollback.
--
-- Reverse of 000003_create_import_jobs.up.sql: drops the `import_jobs`
-- table. Per D-25 / D04_1-12 the v0.1 down migration is
-- documentation-only; production uses `task db:reset`.

BEGIN;
DROP TABLE IF EXISTS import_jobs;
COMMIT;
