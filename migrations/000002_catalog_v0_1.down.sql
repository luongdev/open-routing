-- Phase 3 — Catalog v0.1 schema rollback.
--
-- Reverse of 000002_catalog_v0_1.up.sql: drops the 7 catalog tables and
-- RECREATES _scaffold so `migrate down 1` from a fully-applied state lands
-- back at the Phase 1 baseline (RESEARCH Common Operation 3 lines 980-998).
--
-- Drop order is reverse-dependency: agent_skills first (would carry the join
-- references if FKs existed), then the entity tables. No DB FKs exist (D-76)
-- so the order is documentation-only — every DROP would succeed in isolation.

BEGIN;

-- Phase 4 — agent_states (D-78) reverse. Drops first because it was
-- appended LAST in the up migration; no FK dependencies exist (D-80) so
-- ordering is documentation-only.
DROP TABLE IF EXISTS agent_states;

DROP TABLE IF EXISTS agent_skills;
DROP TABLE IF EXISTS break_reasons;
DROP TABLE IF EXISTS adapters;
DROP TABLE IF EXISTS channels;
DROP TABLE IF EXISTS queues;
DROP TABLE IF EXISTS skills;
DROP TABLE IF EXISTS agents;

-- Recreate _scaffold for rollback safety. Schema MUST match
-- 000001_create_scaffold.up.sql so `migrate down 1` from 002 == state after
-- `migrate up 1` from a clean DB.
CREATE TABLE _scaffold (
    id          UUID PRIMARY KEY,
    org_id      UUID NOT NULL,
    external_id TEXT NOT NULL,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, external_id)
);

CREATE INDEX idx_scaffold_org_id ON _scaffold (org_id);

COMMIT;
