-- name: ListRoutableCandidates :many
-- The simulation/routing candidate pool: enabled agents in Ready state, with
-- each skill's code + proficiency and the instant they became available
-- (agent_states.updated_at, used for the longest-available tie-break). One row
-- per (agent, skill); a skill-less agent yields one row with NULL skill.
-- Every tenant table is org-filtered in the top-level WHERE (SQLChecker requires
-- a WHERE org_id ColumnRef per tenant alias, not a JOIN-ON one); the LEFT-joined
-- tables use `OR ... IS NULL` so a skill-less agent is not dropped.
SELECT a.code           AS agent_code,
       sk.code          AS skill_code,
       ags.proficiency  AS proficiency,
       ast.updated_at   AS available_since
FROM agents a
JOIN agent_states ast      ON ast.agent_id = a.id
LEFT JOIN agent_skills ags ON ags.agent_id = a.id
LEFT JOIN skills sk        ON sk.id = ags.skill_id
WHERE a.org_id = $1
  AND ast.org_id = $1
  AND (ags.org_id = $1 OR ags.org_id IS NULL)
  AND (sk.org_id = $1 OR sk.org_id IS NULL)
  AND a.enabled = TRUE
  AND ast.status = 'Ready'
ORDER BY a.code, sk.code;
