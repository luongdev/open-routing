-- 3b "make Simulate real": persist simulation traces with everything needed to
-- replay them deterministically, independent of the (mutable) draft they ran
-- against. 000001 is already applied on running envs, so this is a forward
-- migration (never edit 000001).

ALTER TABLE traces
    ADD COLUMN flow_id                UUID,
    -- The EXACT executable plan the run used; replay never reads the live draft.
    ADD COLUMN compiled_plan_snapshot JSONB,
    ADD COLUMN plan_format_version    INT,
    -- Full request input: interaction_input, channel, entry_code, resolved
    -- virtual_clock_start, scripted_reservation_outcomes, scripted_effect_outputs.
    ADD COLUMN simulation_input       JSONB,
    -- The catalog+state read-set captured under a REPEATABLE READ snapshot.
    ADD COLUMN read_set_snapshot      JSONB,
    -- Integrity/search key only (NOT sufficient for replay on its own).
    ADD COLUMN graph_hash             TEXT;

-- List-by-flow (newest first) + retention key.
CREATE INDEX idx_traces_org_flow_created ON traces (org_id, flow_id, created_at DESC);

-- Retention sweep target: simulation traces age out; runtime traces are kept.
CREATE INDEX idx_traces_sim_expiry ON traces (created_at) WHERE kind = 'simulation';
