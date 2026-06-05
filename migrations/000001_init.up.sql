-- Open Routing — consolidated pre-release schema (single editable migration).
--
-- v0.2 is pre-release, so the former 000001–000005 are squashed into this one
-- file (catalog + agent state + bulk import + flow authoring + runtime
-- foundation). Re-apply via `task db:reset`. Split into a frozen baseline +
-- forward migrations only when the first real environment ships.
--
-- Invariants enforced here:
--   * org_id UUID NOT NULL on every row (multi-org isolation, D-04).
--   * Soft delete via `enabled` + optimistic `version` on mutable catalog/flow
--     entities (CAT-08, CAT-09). Append-only/event tables omit both.
--   * (org_id, created_at DESC, id DESC) cursor index (D-63); partial
--     name-search index where applicable (D-64).
--   * NO cross-entity FK (D-76) — every reference is checked at the app layer.
--   * Locked v0.2 runtime invariants live as partial unique indexes:
--     at-most-one active published binding per (org, channel, entry_code);
--     one active offered|accepted reservation per agent; one accepted
--     reservation per route request (reservation integrity Option A — the
--     source of truth and cross-replica lock, no Redis).

BEGIN;

-- ===========================================================================
-- Catalog (CAT-01..CAT-07)
-- ===========================================================================
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
CREATE INDEX ix_agents_org_name ON agents (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_agents_org_external_id ON agents (org_id, external_id) WHERE external_id IS NOT NULL;

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
CREATE INDEX ix_skills_org_name ON skills (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_skills_org_external_id ON skills (org_id, external_id) WHERE external_id IS NOT NULL;

CREATE TABLE queues (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    code            TEXT NOT NULL,
    external_id     TEXT NULL,
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
CREATE INDEX ix_queues_org_name ON queues (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_queues_org_external_id ON queues (org_id, external_id) WHERE external_id IS NOT NULL;

-- channels.default_queue_id is a nullable app-layer reference (no FK, D-76).
CREATE TABLE channels (
    id                 UUID PRIMARY KEY,
    org_id             UUID NOT NULL,
    code               TEXT NOT NULL,
    external_id        TEXT NULL,
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
CREATE INDEX ix_channels_org_name ON channels (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_channels_org_external_id ON channels (org_id, external_id) WHERE external_id IS NOT NULL;

CREATE TABLE adapters (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    code            TEXT NOT NULL,
    external_id     TEXT NULL,
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
CREATE INDEX ix_adapters_org_name ON adapters (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_adapters_org_external_id ON adapters (org_id, external_id) WHERE external_id IS NOT NULL;

CREATE TABLE break_reasons (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    code            TEXT NOT NULL,
    external_id     TEXT NULL,
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
CREATE INDEX ix_break_reasons_org_display ON break_reasons (org_id, display_order, id) WHERE enabled = TRUE;
CREATE INDEX ix_break_reasons_org_name ON break_reasons (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;
CREATE UNIQUE INDEX ix_break_reasons_org_external_id ON break_reasons (org_id, external_id) WHERE external_id IS NOT NULL;

-- agent_skills join: org_id denormalized so the SQLChecker sees it on every
-- query (D-72); proficiency CHECK is the defense-in-depth backstop (D-74).
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

-- ===========================================================================
-- Agent state (STATE-01..STATE-10). One row per agent. org_id denormalized so
-- the server-owned WrapUp-expiry sweeper can run org-agnostic (D-78).
-- ===========================================================================
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
CREATE INDEX ix_agent_states_wrapup_until ON agent_states (wrapup_until) WHERE status = 'WrapUp';

-- ===========================================================================
-- Bulk import jobs (IMP-06). Event-history row, no enabled/version.
-- ===========================================================================
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
CREATE INDEX ix_import_jobs_org_created ON import_jobs (org_id, created_at DESC, id DESC);
CREATE INDEX ix_import_jobs_pending_updated ON import_jobs (updated_at) WHERE status = 'pending';
CREATE UNIQUE INDEX uq_import_jobs_org_idempotency ON import_jobs (org_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

-- ===========================================================================
-- Flow authoring (FLOW). graph JSONB is the canonical authored artifact;
-- `version` is the optimistic-lock revision of the DRAFT (distinct from the
-- immutable published flow_versions below).
-- ===========================================================================
CREATE TABLE flows (
    id            UUID PRIMARY KEY,
    org_id        UUID NOT NULL,
    code          TEXT NOT NULL,
    name          TEXT NOT NULL,
    graph         JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    version       INTEGER NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, code)
);
CREATE INDEX ix_flows_org_created ON flows (org_id, created_at DESC, id DESC);
CREATE INDEX ix_flows_org_name ON flows (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE;

-- Immutable published versions (write-once; version_number monotonic per
-- (org_id, flow_code) so it survives draft deletion). Carries the compiled
-- plan + its format version for the already-published compatibility contract.
CREATE TABLE flow_versions (
    id                  UUID PRIMARY KEY,
    org_id              UUID NOT NULL,
    flow_id             UUID NOT NULL,
    flow_code           TEXT NOT NULL,
    version_number      INTEGER NOT NULL,
    graph               JSONB NOT NULL,
    compiled_plan       JSONB NOT NULL,
    plan_format_version INTEGER NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, flow_code, version_number)
);
CREATE INDEX ix_flow_versions_org_flow ON flow_versions (org_id, flow_code, version_number DESC);

-- flow_versions is write-once and its compiled_plan is cached "never invalidate";
-- enforce immutability at the DB so a stray UPDATE/DELETE can't desync that cache.
CREATE FUNCTION reject_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'table % is immutable (insert-only)', TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER flow_versions_immutable BEFORE UPDATE OR DELETE ON flow_versions
    FOR EACH ROW EXECUTE FUNCTION reject_mutation();

-- Route entry → active published version. Partial unique index enforces the
-- at-most-one-active invariant; "exactly one after publish/rollback" is the
-- transactional postcondition of those ops (a DB index can only enforce
-- at-most-one).
CREATE TABLE flow_entry_bindings (
    id                UUID PRIMARY KEY,
    org_id            UUID NOT NULL,
    channel           TEXT NOT NULL,
    entry_code        TEXT NOT NULL,
    flow_version_id   UUID NOT NULL,
    flow_code         TEXT NOT NULL,
    active            BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX ux_flow_bindings_active ON flow_entry_bindings (org_id, channel, entry_code) WHERE active = TRUE;
CREATE INDEX ix_flow_bindings_lookup ON flow_entry_bindings (org_id, channel, entry_code);

-- ===========================================================================
-- Runtime (RT). route_requests is the interaction spine; reservations,
-- continuations, runtime_events, traces all reference it.
-- ===========================================================================
CREATE TABLE route_requests (
    id                UUID PRIMARY KEY,
    org_id            UUID NOT NULL,
    channel           TEXT NOT NULL,
    entry_code        TEXT NOT NULL,
    flow_version_id   UUID,
    flow_code         TEXT,
    interaction_input JSONB NOT NULL DEFAULT '{}'::jsonb,
    status            TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','running','waiting','waiting_match','offering','completed','failed','cancelled')),
    failure_code      TEXT,
    read_set_snapshot JSONB,
    -- Wave 4 matcher (queue + bidirectional pull). A route with no available
    -- agent parks 'waiting_match'; the matcher claims it ('offering', token-fenced)
    -- and attaches an offer. match_offer_token fences every claim/requeue/recover
    -- so a stalled worker can't clobber a fresh offer.
    queue_id            UUID,
    priority            INTEGER NOT NULL DEFAULT 0,
    required_skills     TEXT[] NOT NULL DEFAULT '{}',
    waiting_since       TIMESTAMPTZ,
    next_match_at       TIMESTAMPTZ,
    match_deadline      TIMESTAMPTZ,
    match_attempt_seq   INTEGER NOT NULL DEFAULT 0,
    match_offer_token   UUID,
    offering_started_at TIMESTAMPTZ,
    active_reservation_id UUID,
    excluded_agent_ids  UUID[] NOT NULL DEFAULT '{}',
    -- Live-run execution lock + resume position (Wave 3). The route_requests row
    -- is the per-route exclusive lock: a process flips status->running before
    -- running the executor, parks resume_cursor on suspend, and releases it.
    -- Cursor lives here (not on continuations) to avoid the API<->worker deadlock
    -- when an accept races a reservation timeout. run_seq fences cursor staleness.
    resume_cursor          JSONB,
    current_reservation_id UUID,
    run_seq                INTEGER NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- A request that has started executing must have a pinned version; flow_code
    -- and flow_version_id are set together at pin time.
    CHECK (status IN ('pending','failed','cancelled') OR flow_version_id IS NOT NULL),
    CHECK ((flow_version_id IS NULL) = (flow_code IS NULL))
);
CREATE INDEX ix_route_requests_org_created ON route_requests (org_id, created_at DESC, id DESC);
-- Wave 4 matcher access paths: the availability-pull ranking prefix, the SLA
-- deadline sweep, the stale-offering recovery sweep, and skill eligibility.
CREATE INDEX ix_route_requests_pull
    ON route_requests (org_id, queue_id, next_match_at, priority DESC, waiting_since ASC)
    WHERE status = 'waiting_match';
CREATE INDEX ix_route_requests_match_deadline
    ON route_requests (org_id, match_deadline) WHERE status = 'waiting_match';
CREATE INDEX ix_route_requests_offering
    ON route_requests (org_id, offering_started_at) WHERE status = 'offering';
CREATE INDEX ix_route_requests_required_skills ON route_requests USING GIN (required_skills);

-- Reservation lifecycle. Retry = a NEW offered row (attempt+1), not a state.
-- The two partial unique indexes ARE the double-booking / double-acceptance
-- guards (DB-only Option A). The offered-expiry index powers the timeout sweep.
CREATE TABLE reservations (
    id                UUID PRIMARY KEY,
    org_id            UUID NOT NULL,
    route_request_id  UUID NOT NULL,
    agent_id          UUID NOT NULL,
    state             TEXT NOT NULL DEFAULT 'offered'
                        CHECK (state IN ('offered','accepted','rejected','timeout','cancelled','completed')),
    attempt           INTEGER NOT NULL DEFAULT 1 CHECK (attempt > 0),
    offered_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at        TIMESTAMPTZ NOT NULL,
    resolved_at       TIMESTAMPTZ,
    reason            TEXT,
    -- Wave 4 D5 fencing: an accept/reject/complete must match the lease_token +
    -- agent_session_id bound at offer time so a stale command for a superseded
    -- (re-offered) reservation can't resolve it.
    lease_token       UUID,
    agent_session_id  UUID,
    -- v0.3 W6: the channel adapter's opaque delivery handle, bound on accept when
    -- the engine hands the assignment to the adapter (Deliver). Release on complete
    -- + adapter-originated terminals (caller_abandoned, disconnect) key off it.
    adapter_handle    TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- NOTE: a per-(org,agent) "one active reservation" unique index used to live here
-- (v0.2 capacity=1). v0.3 W3 makes capacity DB-solid via agent_capacity_slots
-- (per-(agent,channel) slot rows, FOR UPDATE SKIP LOCKED at offer time) — the
-- authoritative gate. A global per-agent unique index would cap chat=N at 1 and
-- block cross-channel routing, so it was removed (cross-AI review HIGH).
-- One active-or-won reservation per route: enforces the sequential-offer policy
-- (no two simultaneous offers) AND blocks a second accept after accepted->completed.
-- (An agent is free again after completing, so 'completed' is NOT in the agent guard above.)
CREATE UNIQUE INDEX ux_reservations_route_active ON reservations (org_id, route_request_id) WHERE state IN ('offered','accepted','completed');
-- Retry attempts are distinct rows; attempt numbers are unique per route.
CREATE UNIQUE INDEX ux_reservations_route_attempt ON reservations (org_id, route_request_id, attempt);
CREATE INDEX ix_reservations_route ON reservations (org_id, route_request_id);
CREATE INDEX ix_reservations_offered_expiry ON reservations (expires_at) WHERE state = 'offered';

-- Durable delayed work for live wait nodes, reservation timeouts, and the
-- migrated WrapUp expiry. The worker claims due rows with FOR UPDATE SKIP
-- LOCKED so multiple runtime replicas run safely.
CREATE TABLE continuations (
    id                UUID PRIMARY KEY,
    org_id            UUID NOT NULL,
    kind              TEXT NOT NULL
                        CHECK (kind IN ('wait','reservation_timeout','wrapup_expiry')),
    route_request_id  UUID,
    reservation_id    UUID,
    agent_id          UUID,
    flow_version_id   UUID,
    cursor            JSONB NOT NULL DEFAULT '{}'::jsonb,
    due_at            TIMESTAMPTZ NOT NULL,
    status            TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','claimed','done','cancelled')),
    -- Lease for crash recovery: a claimed row whose worker died is reclaimable
    -- once claim_expires_at passes (the claim picks pending OR claimed-and-expired).
    claimed_at        TIMESTAMPTZ,
    claim_expires_at  TIMESTAMPTZ,
    claimed_by        TEXT,
    attempt_count     INTEGER NOT NULL DEFAULT 0,
    -- run_seq fences a stale wait continuation: it pins the route's run_seq at
    -- suspend time, so a timer that fires after the route already advanced (a
    -- newer suspend bumped route_requests.run_seq) is detected and no-op'd
    -- instead of resuming the wrong cursor (v0.2 review B1).
    run_seq           INTEGER NOT NULL DEFAULT 0,
    last_error        TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Each kind carries its own typed subject so a stale timer fires against the
    -- exact target — a reservation_timeout names the specific reservation attempt,
    -- not just the route, so it can't resolve a newer offer.
    CHECK (
        (kind = 'wait' AND route_request_id IS NOT NULL)
        OR (kind = 'reservation_timeout' AND reservation_id IS NOT NULL)
        OR (kind = 'wrapup_expiry' AND agent_id IS NOT NULL)
    )
);
-- Covers both pending and claimed so the worker can reclaim expired leases.
CREATE INDEX ix_continuations_due ON continuations (due_at) WHERE status IN ('pending','claimed');
CREATE INDEX ix_continuations_org ON continuations (org_id, created_at DESC, id DESC);

-- Append-only canonical event envelope (outbox-first). Immutable facts —
-- no version/enabled/updated_at. Distinct from the org audit retention contract.
CREATE TABLE runtime_events (
    id                UUID PRIMARY KEY,
    org_id            UUID NOT NULL,
    route_request_id  UUID,
    source            TEXT NOT NULL,
    type              TEXT NOT NULL,
    correlation_id    UUID,
    payload           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ix_runtime_events_route ON runtime_events (org_id, route_request_id, created_at);
CREATE INDEX ix_runtime_events_org_created ON runtime_events (org_id, created_at DESC, id DESC);

-- Trace read records (runtime + simulation). Direct read table (ordered steps
-- as JSONB) before any broader derived projection.
CREATE TABLE traces (
    id                UUID PRIMARY KEY,
    org_id            UUID NOT NULL,
    route_request_id  UUID,
    kind              TEXT NOT NULL DEFAULT 'runtime'
                        CHECK (kind IN ('runtime','simulation')),
    flow_version_id   UUID,
    steps             JSONB NOT NULL DEFAULT '[]'::jsonb,
    outcome           TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ix_traces_route ON traces (org_id, route_request_id);
CREATE INDEX ix_traces_org_created ON traces (org_id, created_at DESC, id DESC);

-- v0.3 W2: realtime transport (WS gateway). All durable so a dropped socket never
-- loses an offer or double-applies a command (outbox-first delivery).

-- agent_outbox is the ONLY outbound delivery source. server_seq is per-agent
-- monotonic, allocated under a per-agent advisory lock INSIDE the producing tx
-- (NOT a global IDENTITY, whose commit-order gap would skip rows on reconnect).
-- The relay reads only committed rows in seq order; event_key makes a reconnect
-- re-derivation idempotent.
CREATE TABLE agent_outbox (
    org_id            UUID NOT NULL,
    agent_id          UUID NOT NULL,
    server_seq        BIGINT NOT NULL,
    event_key         TEXT NOT NULL,
    type              TEXT NOT NULL,
    reservation_id    UUID,
    payload           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, agent_id, server_seq),
    UNIQUE (org_id, agent_id, event_key)
);

-- ws_command_dedupe: an agent command applies at most once. status+result let a
-- redelivered command return the original ack without re-running; request_hash
-- rejects a reused client_msg_id carrying a different payload.
CREATE TABLE ws_command_dedupe (
    org_id            UUID NOT NULL,
    agent_id          UUID NOT NULL,
    client_msg_id     UUID NOT NULL,
    command_type      TEXT NOT NULL,
    request_hash      TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','done')),
    result            JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, agent_id, client_msg_id)
);

-- agent_sessions: cluster-visible session inventory for per-command revocation.
CREATE TABLE agent_sessions (
    org_id            UUID NOT NULL,
    session_id        UUID NOT NULL,
    agent_id          UUID NOT NULL,
    gateway_id        TEXT NOT NULL,
    connected_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    terminated_at     TIMESTAMPTZ,
    PRIMARY KEY (org_id, session_id)
);
CREATE INDEX ix_agent_sessions_agent ON agent_sessions (org_id, agent_id) WHERE terminated_at IS NULL;

-- agent_capacity_slots (v0.3 W3): DB-solid per-(agent,channel) capacity. One row
-- per concurrent interaction the agent can hold on a channel (voice=1, chat=N).
-- Slot state is derived: free = reservation_id NULL; pending = reservation_id
-- set AND hold_expires_at set (sweepable); confirmed = reservation_id set AND
-- hold_expires_at NULL (held for a live call, never swept by the timer). The
-- offer tx acquires a free slot under FOR UPDATE SKIP LOCKED — the authoritative
-- capacity gate (the candidate-source free-count is only a hint).
CREATE TABLE agent_capacity_slots (
    org_id            UUID NOT NULL,
    agent_id          UUID NOT NULL,
    channel           TEXT NOT NULL,
    slot_no           INT NOT NULL,
    reservation_id    UUID,
    hold_expires_at   TIMESTAMPTZ,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, agent_id, channel, slot_no)
);
CREATE INDEX ix_agent_capacity_slots_sweep
    ON agent_capacity_slots (hold_expires_at) WHERE hold_expires_at IS NOT NULL;
-- A reservation holds at most one slot anywhere (guards a double-acquire). Keyed
-- on reservation_id, NOT agent — so it does NOT impose capacity=1.
CREATE UNIQUE INDEX ux_agent_capacity_slots_one_per_reservation
    ON agent_capacity_slots (org_id, reservation_id) WHERE reservation_id IS NOT NULL;

-- route_decisions (v0.3 W4, design D9): one row per matcher decision — the audit
-- trail that makes a live routing decision fully explainable (who was considered,
-- who was excluded and why, the ranking, the outcome).
CREATE TABLE route_decisions (
    id                UUID PRIMARY KEY,
    org_id            UUID NOT NULL,
    route_request_id  UUID NOT NULL,
    decision_type     TEXT NOT NULL
                        CHECK (decision_type IN ('interaction_offer','availability_pull','retry','sweep')),
    decision_version  INTEGER NOT NULL DEFAULT 1,
    matcher_instance  TEXT NOT NULL,
    channel           TEXT NOT NULL,
    queue_id          UUID,
    selected_agent_id UUID,
    selected_slot_no  INTEGER,
    outcome           TEXT NOT NULL
                        CHECK (outcome IN ('offered','no_candidate','capacity_lost','lease_lost','route_lost')),
    reason            TEXT,
    detail            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ix_route_decisions_route ON route_decisions (org_id, route_request_id, created_at DESC);
CREATE INDEX ix_route_decisions_agent ON route_decisions (org_id, selected_agent_id, created_at DESC) WHERE selected_agent_id IS NOT NULL;

-- agent_routing_state (v0.3 W4 RONA): kept separate from the agents catalog row
-- (which the catalog package maps with column-exact queries) so adding mutable
-- routing state doesn't churn that package. A missed offer marks the agent
-- non-routable until the TTL or an explicit Ready clears it; last_ready_at fences
-- a late RONA write from clobbering a Ready that arrived after the ring.
CREATE TABLE agent_routing_state (
    org_id            UUID NOT NULL,
    agent_id          UUID NOT NULL,
    routing_state     TEXT NOT NULL DEFAULT 'routable'
                        CHECK (routing_state IN ('routable','missed')),
    state_expires_at  TIMESTAMPTZ,
    last_ready_at     TIMESTAMPTZ,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, agent_id)
);

COMMIT;
