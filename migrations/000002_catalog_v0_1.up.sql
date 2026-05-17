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
--   * (Phase 04.1) Every primary catalog table carries `code TEXT NOT NULL`
--     with `UNIQUE (org_id, code)`; `external_id` is demoted to `TEXT NULL`
--     with a partial unique index `ix_<entity>_org_external_id ... WHERE
--     external_id IS NOT NULL`. break_reasons.UNIQUE(org_id, name) is
--     dropped (name becomes mutable display label). Backfill at the end of
--     this file derives code from external_id (or name for adapters and
--     break_reasons) and dedupes collisions via row_number window
--     (idempotent — re-runnable). Backfill is a defensive no-op under
--     fresh `task db:reset` (code is NOT NULL inline); present for
--     stale-DB rollforward safety per D-61. See
--     .planning/phases/04.1-catalog-identity-normalization/.

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
    code          TEXT NOT NULL,
    external_id   TEXT NULL,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    version       INTEGER NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, code)
);
CREATE INDEX ix_agents_org_created ON agents (org_id, created_at DESC, id DESC);
-- Partial functional index; supports prefix LIKE only — substring '%foo%' will
-- still seq-scan. Acceptable at v0.1 scale (<1K rows/org/entity). Trigram /
-- pg_trgm GIN deferred to v0.2 (D-64, Pitfall 7).
CREATE INDEX ix_agents_org_name ON agents (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_agents_org_external_id ON agents (org_id, external_id) WHERE external_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- CAT-02: skills
-- ---------------------------------------------------------------------------
CREATE TABLE skills (
    id            UUID PRIMARY KEY,
    org_id        UUID NOT NULL,
    code          TEXT NOT NULL,
    external_id   TEXT NULL,
    name          TEXT NOT NULL,
    description   TEXT,
    skill_type    TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    version       INTEGER NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, code)
);
CREATE INDEX ix_skills_org_created ON skills (org_id, created_at DESC, id DESC);
-- Partial functional index; supports prefix LIKE only — substring '%foo%' will
-- still seq-scan. Acceptable at v0.1 scale (D-64, Pitfall 7).
CREATE INDEX ix_skills_org_name ON skills (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_skills_org_external_id ON skills (org_id, external_id) WHERE external_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- CAT-04: queues
-- ---------------------------------------------------------------------------
CREATE TABLE queues (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    code          TEXT NOT NULL,
    external_id   TEXT NULL,
    name            TEXT NOT NULL,
    channel_types   TEXT[] NOT NULL DEFAULT '{}',
    priority        INTEGER NOT NULL DEFAULT 0,
    acw_sec         INTEGER NOT NULL DEFAULT 0,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, code)
);
CREATE INDEX ix_queues_org_created ON queues (org_id, created_at DESC, id DESC);
-- Partial functional index; supports prefix LIKE only (D-64, Pitfall 7).
CREATE INDEX ix_queues_org_name ON queues (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_queues_org_external_id ON queues (org_id, external_id) WHERE external_id IS NOT NULL;

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
    code          TEXT NOT NULL,
    external_id   TEXT NULL,
    name               TEXT NOT NULL,
    channel_type       TEXT NOT NULL,
    default_queue_id   UUID,
    enabled            BOOLEAN NOT NULL DEFAULT TRUE,
    version            INTEGER NOT NULL DEFAULT 1,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, code)
);
CREATE INDEX ix_channels_org_created ON channels (org_id, created_at DESC, id DESC);
-- Partial functional index; supports prefix LIKE only (D-64, Pitfall 7).
CREATE INDEX ix_channels_org_name ON channels (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_channels_org_external_id ON channels (org_id, external_id) WHERE external_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- CAT-06: adapters (Phase 04.1: gains code + external_id per D04_1-01 / D04_1-06)
-- ---------------------------------------------------------------------------
-- adapter.config is free-form JSONB (CAT-06). Unbounded write surface is
-- accepted at v0.1 (stub auth); production hardening (http.MaxBytesReader,
-- JSON depth limits) deferred. Phase 04.1 adds BOTH code (required) and
-- external_id (optional) — composite UNIQUE (org_id, code) auto-name
-- `adapters_org_id_code_key`; partial unique on external_id WHERE NOT NULL.
CREATE TABLE adapters (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    code          TEXT NOT NULL,
    external_id   TEXT NULL,
    name            TEXT NOT NULL,
    adapter_type    TEXT NOT NULL,
    config          JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, code)
);
CREATE INDEX ix_adapters_org_created ON adapters (org_id, created_at DESC, id DESC);
-- Partial functional index; supports prefix LIKE only (D-64, Pitfall 7).
CREATE INDEX ix_adapters_org_name ON adapters (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_adapters_org_external_id ON adapters (org_id, external_id) WHERE external_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- CAT-07: break_reasons (Phase 04.1: code+external_id added; UNIQUE(org_id,name) dropped)
-- ---------------------------------------------------------------------------
-- Phase 04.1 (D04_1-03 + IDENT-03): break_reasons gains code (required) and
-- external_id (optional). The historical UNIQUE (org_id, name) constraint is
-- DROPPED — name becomes a mutable display label. display_order is the
-- ordering field used by the agent-status UI.
CREATE TABLE break_reasons (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    code          TEXT NOT NULL,
    external_id   TEXT NULL,
    name            TEXT NOT NULL,
    routable        BOOLEAN NOT NULL DEFAULT FALSE,
    display_order   INTEGER NOT NULL DEFAULT 0,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, code)
);
CREATE INDEX ix_break_reasons_org_created ON break_reasons (org_id, created_at DESC, id DESC);
-- display_order index supports the ordered list view used by the agent-status
-- picker; ASC ordering is intentional (lower display_order = higher in list).
CREATE INDEX ix_break_reasons_org_display ON break_reasons (org_id, display_order, id) WHERE enabled = TRUE;
-- Partial functional name-search index (D-64 universal application; Wave 1 codex review).
-- Supports prefix LIKE for ILIKE name filter on the break_reasons list endpoint (CAT-10).
CREATE INDEX ix_break_reasons_org_name ON break_reasons (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_break_reasons_org_external_id ON break_reasons (org_id, external_id) WHERE external_id IS NOT NULL;

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

-- ---------------------------------------------------------------------------
-- STATE-01..STATE-10: agent_states (Phase 4 — D-78)
-- ---------------------------------------------------------------------------
-- One row per agent. PK = agent_id. NO FK (D-80 — app-layer probes only).
-- TEXT + CHECK encoding for status/engaged_channel/post_interaction_state
-- (D-79). Adding values = ALTER constraint, not ALTER TYPE on PG14-.
-- org_id is denormalized so SQLChecker (Phase 1 D-04) sees the column on
-- every sqlc-generated query; the sweeper goroutine bypasses ctx-injection
-- and relies on this column in the SQL WHERE clauses for org auditability.
CREATE TABLE agent_states (
    agent_id                UUID PRIMARY KEY,
    org_id                  UUID NOT NULL,
    status                  TEXT NOT NULL
                              CHECK (status IN ('Ready','NotReady','Break','Engaged','WrapUp','Offline')),
    engaged_channel         TEXT
                              CHECK (engaged_channel IS NULL OR engaged_channel IN ('voice','chat','email')),
    break_reason_id         UUID,
    post_interaction_state  TEXT
                              CHECK (post_interaction_state IS NULL OR post_interaction_state IN ('ready','not_ready')),
    wrapup_until            TIMESTAMPTZ,
    state_version           BIGINT NOT NULL DEFAULT 1,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ix_agent_states_org_status ON agent_states (org_id, status);

-- Partial index — sweeper hot path is `WHERE status='WrapUp' AND wrapup_until < NOW()`.
-- Partial index ensures the planner reads only the small subset of rows in WrapUp.
CREATE INDEX ix_agent_states_wrapup_until
    ON agent_states (wrapup_until)
    WHERE status = 'WrapUp';

-- ---------------------------------------------------------------------------
-- Phase 04.1 backfill (D04_1-10, D04_1-11): populate code from external_id
-- (agents/skills/queues/channels) or name (adapters/break_reasons), then
-- dedupe with row_number window. Idempotent — re-running on deduped data
-- is a no-op.
--
-- IMPORTANT — execution semantics under `task db:reset` vs stale-DB rollforward:
--   * Under `task db:reset` (fresh): the `code TEXT NOT NULL` column was
--     created inline; no row exists with NULL or empty code. The
--     `WHERE code IS NULL OR code = ''` predicate matches zero rows.
--     Backfill is a defensive no-op on fresh task db:reset. This is intentional.
--   * Under stale-DB rollforward (a dev applied a pre-Phase-04.1 version of
--     000002 locally before this amendment; D-61 editable-migration policy):
--     the dev runs `task db:reset` to pick up the amendment; this branch
--     never executes. The backfill ONLY fires if a custom migrate-up path
--     is invoked that adds the `code` column without populating it
--     (currently no such path exists in v0.1; this is forward-compatibility).
--   * The PATTERNS.md two-step pattern (`TEXT NULL → UPDATE → SET NOT NULL`)
--     is the production-rollforward-correct equivalent and SHOULD be used
--     if a future production migration adds `code` to existing rows.
--     v0.1 uses the inline-NOT-NULL form because pre-v0.1-ship reality (A2):
--     no production rows exist yet. See RESEARCH §Pitfall 5.
-- ---------------------------------------------------------------------------

-- agents: backfill code from external_id, then dedupe via row_number window.
-- Backfill is no-op on fresh task db:reset (code is NOT NULL inline);
-- present for stale-DB rollforward safety per D-61.
-- Prefix `r_` if external_id does not start with a letter (D04_1-03 strict regex).
UPDATE agents
SET code = (CASE WHEN substring(external_id from 1 for 1) ~ '^[a-zA-Z]$' THEN '' ELSE 'r_' END)
           || lower(regexp_replace(external_id, '[^a-zA-Z0-9_]', '_', 'g'))
WHERE code IS NULL OR code = '';

WITH dups AS (
    SELECT id,
           row_number() OVER (PARTITION BY org_id, code ORDER BY created_at, id) AS rn
    FROM agents
)
UPDATE agents a
SET code = a.code || '_' || dups.rn::text
FROM dups
WHERE a.id = dups.id AND dups.rn > 1;

-- skills: backfill code from external_id, then dedupe via row_number window.
-- Backfill is no-op on fresh task db:reset (code is NOT NULL inline);
-- present for stale-DB rollforward safety per D-61.
-- Prefix `r_` if external_id does not start with a letter (D04_1-03 strict regex).
UPDATE skills
SET code = (CASE WHEN substring(external_id from 1 for 1) ~ '^[a-zA-Z]$' THEN '' ELSE 'r_' END)
           || lower(regexp_replace(external_id, '[^a-zA-Z0-9_]', '_', 'g'))
WHERE code IS NULL OR code = '';

WITH dups AS (
    SELECT id,
           row_number() OVER (PARTITION BY org_id, code ORDER BY created_at, id) AS rn
    FROM skills
)
UPDATE skills s
SET code = s.code || '_' || dups.rn::text
FROM dups
WHERE s.id = dups.id AND dups.rn > 1;

-- queues: backfill code from external_id, then dedupe via row_number window.
-- Backfill is no-op on fresh task db:reset (code is NOT NULL inline);
-- present for stale-DB rollforward safety per D-61.
-- Prefix `r_` if external_id does not start with a letter (D04_1-03 strict regex).
UPDATE queues
SET code = (CASE WHEN substring(external_id from 1 for 1) ~ '^[a-zA-Z]$' THEN '' ELSE 'r_' END)
           || lower(regexp_replace(external_id, '[^a-zA-Z0-9_]', '_', 'g'))
WHERE code IS NULL OR code = '';

WITH dups AS (
    SELECT id,
           row_number() OVER (PARTITION BY org_id, code ORDER BY created_at, id) AS rn
    FROM queues
)
UPDATE queues q
SET code = q.code || '_' || dups.rn::text
FROM dups
WHERE q.id = dups.id AND dups.rn > 1;

-- channels: backfill code from external_id, then dedupe via row_number window.
-- Backfill is no-op on fresh task db:reset (code is NOT NULL inline);
-- present for stale-DB rollforward safety per D-61.
-- Prefix `r_` if external_id does not start with a letter (D04_1-03 strict regex).
UPDATE channels
SET code = (CASE WHEN substring(external_id from 1 for 1) ~ '^[a-zA-Z]$' THEN '' ELSE 'r_' END)
           || lower(regexp_replace(external_id, '[^a-zA-Z0-9_]', '_', 'g'))
WHERE code IS NULL OR code = '';

WITH dups AS (
    SELECT id,
           row_number() OVER (PARTITION BY org_id, code ORDER BY created_at, id) AS rn
    FROM channels
)
UPDATE channels c
SET code = c.code || '_' || dups.rn::text
FROM dups
WHERE c.id = dups.id AND dups.rn > 1;

-- adapters: backfill code from name (no pre-existing external_id), then dedupe.
-- Backfill is no-op on fresh task db:reset (code is NOT NULL inline);
-- present for stale-DB rollforward safety per D-61.
-- Prefix `r_` if name does not start with a letter (D04_1-03 strict regex).
UPDATE adapters
SET code = (CASE WHEN substring(name from 1 for 1) ~ '^[a-zA-Z]$' THEN '' ELSE 'r_' END)
           || lower(regexp_replace(name, '[^a-zA-Z0-9_]', '_', 'g'))
WHERE code IS NULL OR code = '';

WITH dups AS (
    SELECT id,
           row_number() OVER (PARTITION BY org_id, code ORDER BY created_at, id) AS rn
    FROM adapters
)
UPDATE adapters ad
SET code = ad.code || '_' || dups.rn::text
FROM dups
WHERE ad.id = dups.id AND dups.rn > 1;

-- break_reasons: backfill code from name (no pre-existing external_id), then dedupe.
-- Backfill is no-op on fresh task db:reset (code is NOT NULL inline);
-- present for stale-DB rollforward safety per D-61.
-- Prefix `r_` if name does not start with a letter (D04_1-03 strict regex).
UPDATE break_reasons
SET code = (CASE WHEN substring(name from 1 for 1) ~ '^[a-zA-Z]$' THEN '' ELSE 'r_' END)
           || lower(regexp_replace(name, '[^a-zA-Z0-9_]', '_', 'g'))
WHERE code IS NULL OR code = '';

WITH dups AS (
    SELECT id,
           row_number() OVER (PARTITION BY org_id, code ORDER BY created_at, id) AS rn
    FROM break_reasons
)
UPDATE break_reasons br
SET code = br.code || '_' || dups.rn::text
FROM dups
WHERE br.id = dups.id AND dups.rn > 1;

COMMIT;
