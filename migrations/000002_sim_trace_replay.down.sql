DROP INDEX IF EXISTS idx_traces_sim_expiry;
DROP INDEX IF EXISTS idx_traces_org_flow_created;

ALTER TABLE traces
    DROP COLUMN IF EXISTS graph_hash,
    DROP COLUMN IF EXISTS read_set_snapshot,
    DROP COLUMN IF EXISTS simulation_input,
    DROP COLUMN IF EXISTS plan_format_version,
    DROP COLUMN IF EXISTS compiled_plan_snapshot,
    DROP COLUMN IF EXISTS flow_id;
