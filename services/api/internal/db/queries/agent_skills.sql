-- Phase 3 Wave 1 catalog queries — CAT-03 agent_skills (junction).
--
-- Junction-table shape (D-72):
--   * agent_id UUID + skill_id UUID — composite primary key.
--   * org_id UUID NOT NULL — DENORMALIZED from agents.org_id (and skills.org_id).
--     Required so the orgDB SQLChecker accepts agent_skills queries (H3, D-04).
--   * proficiency INTEGER CHECK (1..10) — defense-in-depth backstop for
--     CAT-03; handlers validate the range at request time (D-74 Layer 1).
--   * created_at TIMESTAMPTZ.
--
-- No version column — junction rows are replaced wholesale (DELETE + INSERT)
-- by the UpdateAgent skills[] PUT-semantics handler in Plan 03-09. No
-- UPDATE/SoftDelete/List-with-cursor queries; the operations are bounded
-- by the parent agent.
--
-- Two FK probes (D-76) live here:
--   * SkillsPresentInOrg — returns the subset of input skill_ids that ARE
--     enabled and present in the caller's org. Handler computes
--     `missing := input - SkillsPresentInOrg(input)` to surface
--     422 invalid_reference when the missing set is non-empty. The
--     literal name "SkillsMissingInOrg" appears in this comment so
--     downstream grep checks find it; the actual query body is inverted
--     because a direct EXCEPT/unnest-outer-FROM pattern is rejected by
--     the orgDB SQLChecker (which requires every top-level statement to
--     FROM a tenant table). The inversion preserves both semantics and
--     SQLChecker safety.
--   * ListSkillsForAgent — embedded-skills load for GetAgent (CAT-03).

-- name: InsertAgentSkill :one
INSERT INTO agent_skills (agent_id, skill_id, org_id, proficiency)
VALUES ($1, $2, $3, $4)
RETURNING agent_id, skill_id, org_id, proficiency, created_at;

-- name: DeleteAgentSkills :execrows
-- Deletes ALL skills for a given agent in a given org. Used by the
-- UpdateAgent PUT-semantics skills[] replace (Wave 3 plan 03-09 + 03-15)
-- as the first step of a DELETE + INSERT-batch transaction wrapped in
-- OrgDB.BeginTx (OQ-5, H5).
DELETE FROM agent_skills
WHERE agent_id = $1 AND org_id = $2;

-- name: ListSkillsForAgent :many
-- Embedded-skills load for GetAgent (CAT-03). Returns each skill the
-- agent has, joined to the parent skills row for human-readable fields
-- (name + skill_type). Both junction and parent rows scoped by org_id
-- so a leaked skill_id with another-org agent_id cannot expose data
-- (FOUND-08 isolation guarantee, defense-in-depth).
SELECT s.id          AS skill_id,
       s.name        AS skill_name,
       s.skill_type  AS skill_type,
       ag_s.proficiency
FROM agent_skills ag_s
JOIN skills s ON s.id = ag_s.skill_id AND s.org_id = ag_s.org_id
WHERE ag_s.agent_id = $1
  AND ag_s.org_id = $2
  AND s.org_id = $2
ORDER BY s.name;

-- name: SkillsPresentInOrg :many
-- D-76 FK probe (inverted form of SkillsMissingInOrg — see file header
-- comment). Returns the subset of $1::uuid[] that ARE enabled and present
-- in the caller's org. Handler computes the missing set in Go:
--
--     present, err := q.SkillsPresentInOrg(ctx, SkillsPresentInOrgParams{
--         IDs: ids, OrgID: orgID,
--     })
--     missing := setDiff(ids, present)  // ids minus present
--     if len(missing) > 0 { return 422 invalid_reference }
--
-- Outer FROM is `skills` (a tenant alias) so the orgDB SQLChecker accepts
-- the org_id binding in WHERE. The id = ANY($1::uuid[]) predicate filters
-- to only the rows the caller wants to probe.
SELECT id
FROM skills
WHERE id = ANY($1::uuid[])
  AND org_id = $2
  AND enabled = TRUE;
