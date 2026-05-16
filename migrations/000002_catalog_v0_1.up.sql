-- Phase 3 — Catalog v0.1 schema (D-61 single editable migration).
--
-- This migration is EDITABLE in place during v0.1. Phase 4 will extend it
-- with agent_states columns + state-transition CHECK; Phase 5 will add
-- import_jobs. Once v0.1 ships to a real environment, freeze this migration
-- and ratchet forward with 003+.
--
-- Conventions enforced by this file:
--   * Every catalog table carries org_id UUID NOT NULL (multi-org isolation D-04).
--   * Every catalog table carries enabled BOOLEAN NOT NULL DEFAULT TRUE (soft-delete CAT-09).
--   * Every catalog table carries version INTEGER NOT NULL DEFAULT 1 (optimistic
--     locking CAT-08, universal — including channels + adapters per OQ-1/A4).
--   * Composite DESC index (org_id, created_at DESC, id DESC) powers cursor
--     pagination (D-63).
--   * Partial functional index (org_id, lower(name) text_pattern_ops) WHERE
--     enabled = TRUE powers case-insensitive prefix name search (D-64).
--   * NO cross-entity FK constraints (D-76) — app-layer validation only in v0.1.

BEGIN;

-- D-77: Phase 1 scaffold is replaced by the catalog. Drop first so `task
-- db:reset` rebuilds cleanly and migrate-up from a Phase 1 baseline doesn't
-- leave _scaffold behind.
DROP TABLE IF EXISTS _scaffold;

-- ---------------------------------------------------------------------------
-- CAT-01: agents
-- ---------------------------------------------------------------------------
CREATE TABLE agents (
    id            UUID PRIMARY KEY,
    org_id        UUID NOT NULL,
    external_id   TEXT NOT NULL,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    version       INTEGER NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, external_id)
);
CREATE INDEX ix_agents_org_created ON agents (org_id, created_at DESC, id DESC);
-- Partial functional index; supports prefix LIKE only — substring '%foo%' will
-- still seq-scan. Acceptable at v0.1 scale (<1K rows/org/entity). Trigram /
-- pg_trgm GIN deferred to v0.2 (D-64, Pitfall 7).
CREATE INDEX ix_agents_org_name ON agents (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

-- ---------------------------------------------------------------------------
-- CAT-02: skills
-- ---------------------------------------------------------------------------
CREATE TABLE skills (
    id            UUID PRIMARY KEY,
    org_id        UUID NOT NULL,
    external_id   TEXT NOT NULL,
    name          TEXT NOT NULL,
    description   TEXT,
    skill_type    TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    version       INTEGER NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, external_id)
);
CREATE INDEX ix_skills_org_created ON skills (org_id, created_at DESC, id DESC);
-- Partial functional index; supports prefix LIKE only — substring '%foo%' will
-- still seq-scan. Acceptable at v0.1 scale (D-64, Pitfall 7).
CREATE INDEX ix_skills_org_name ON skills (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

-- ---------------------------------------------------------------------------
-- CAT-04: queues
-- ---------------------------------------------------------------------------
CREATE TABLE queues (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    external_id     TEXT NOT NULL,
    name            TEXT NOT NULL,
    channel_types   TEXT[] NOT NULL DEFAULT '{}',
    priority        INTEGER NOT NULL DEFAULT 0,
    acw_sec         INTEGER NOT NULL DEFAULT 0,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, external_id)
);
CREATE INDEX ix_queues_org_created ON queues (org_id, created_at DESC, id DESC);
-- Partial functional index; supports prefix LIKE only (D-64, Pitfall 7).
CREATE INDEX ix_queues_org_name ON queues (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

-- ---------------------------------------------------------------------------
-- CAT-05: channels (OQ-1/A4: version column added for CAT-08 parity)
-- ---------------------------------------------------------------------------
-- channels.default_queue_id is nullable; NO db-level FK in v0.1 —
-- cross-row validation lives in the catalog handler at the application
-- layer (D-76). v0.2 may revisit FK CASCADE semantics once soft-delete
-- behavior is locked.
CREATE TABLE channels (
    id                 UUID PRIMARY KEY,
    org_id             UUID NOT NULL,
    external_id        TEXT NOT NULL,
    name               TEXT NOT NULL,
    channel_type       TEXT NOT NULL,
    default_queue_id   UUID,
    enabled            BOOLEAN NOT NULL DEFAULT TRUE,
    version            INTEGER NOT NULL DEFAULT 1,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, external_id)
);
CREATE INDEX ix_channels_org_created ON channels (org_id, created_at DESC, id DESC);
-- Partial functional index; supports prefix LIKE only (D-64, Pitfall 7).
CREATE INDEX ix_channels_org_name ON channels (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

-- ---------------------------------------------------------------------------
-- CAT-06: adapters (NO external_id per spec; version added for CAT-08 parity)
-- ---------------------------------------------------------------------------
-- adapter.config is free-form JSONB (CAT-06). Unbounded write surface is
-- accepted at v0.1 (stub auth); production hardening (http.MaxBytesReader,
-- JSON depth limits) deferred. No UNIQUE constraint beyond PRIMARY KEY(id) —
-- adapter has no external_id (verified Phase 2 spec).
CREATE TABLE adapters (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    name            TEXT NOT NULL,
    adapter_type    TEXT NOT NULL,
    config          JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ix_adapters_org_created ON adapters (org_id, created_at DESC, id DESC);
-- Partial functional index; supports prefix LIKE only (D-64, Pitfall 7).
CREATE INDEX ix_adapters_org_name ON adapters (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

-- ---------------------------------------------------------------------------
-- CAT-07: break_reasons (NO external_id; name is user-facing unique key;
-- display_order is the ordering field used by the agent-status UI)
-- ---------------------------------------------------------------------------
CREATE TABLE break_reasons (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    name            TEXT NOT NULL,
    routable        BOOLEAN NOT NULL DEFAULT FALSE,
    display_order   INTEGER NOT NULL DEFAULT 0,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, name)
);
CREATE INDEX ix_break_reasons_org_created ON break_reasons (org_id, created_at DESC, id DESC);
-- display_order index supports the ordered list view used by the agent-status
-- picker; ASC ordering is intentional (lower display_order = higher in list).
CREATE INDEX ix_break_reasons_org_display ON break_reasons (org_id, display_order, id) WHERE enabled = TRUE;

-- ---------------------------------------------------------------------------
-- CAT-03: agent_skills join
-- ---------------------------------------------------------------------------
-- agent_skills.org_id is denormalized from agents.org_id to satisfy the
-- orgDB SQLChecker, which requires every agent_skills query mention org_id
-- (Hazard H3, D-04 carry-forward). Without this column the static analyzer
-- rejects every sqlc-generated agent_skills query at first generation.
--
-- proficiency CHECK is the defense-in-depth backstop for CAT-03; the handler
-- validates the range at request time and returns 422 invalid_body before the
-- INSERT reaches the database (D-74 Layer 1). The CHECK guarantees that no
-- direct DB write (admin shell, future bulk import) can bypass the range.
CREATE TABLE agent_skills (
    agent_id        UUID NOT NULL,
    skill_id        UUID NOT NULL,
    org_id          UUID NOT NULL,
    proficiency     INTEGER NOT NULL CHECK (proficiency BETWEEN 1 AND 10),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, skill_id)
);
CREATE INDEX ix_agent_skills_org_agent ON agent_skills (org_id, agent_id);
CREATE INDEX ix_agent_skills_org_skill ON agent_skills (org_id, skill_id);

COMMIT;
