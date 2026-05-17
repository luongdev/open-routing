// agent_skills.go owns the CAT-03 skills-replace helpers extracted from
// agents.go (D-68 per-entity-file layout). The OpenAPI spec has no
// standalone /v1/agent_skills resource — these helpers exist only to be
// composed by CreateAgent + UpdateAgent in agents.go.
//
// Codex C4 iter 3 invariants:
//
//   - replaceAgentSkills accepts a caller-owned *generated.Queries bound
//     to an enclosing tx; the helper itself opens nothing. CreateAgent
//     and UpdateAgent in agents.go own the transactional lifecycle, so
//     the agent row write and the skills replace either both commit or
//     both roll back (Pitfall 5 atomicity).
//
//   - validateNoDuplicateSkills and validateProficiencyRange run as
//     pre-flight Layer 2 checks (D-74) BEFORE any DB call. Duplicate
//     skill_ids must surface as 422 invalid_value, never as 409 from the
//     UNIQUE constraint — the input itself is malformed (Wave 4 cross-AI
//     review).
package catalog

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// validateProficiencyRange enforces the 1..10 inclusive range on every
// AgentSkillAssignment. Plan 03-01 intentionally OMITS minimum/maximum
// from the OpenAPI proficiency field so oapi-codegen does not
// short-circuit with 400 — the handler owns the 422 invalid_value wire
// shape (ROADMAP Phase 3 criterion 4).
func validateProficiencyRange(assignments []api.AgentSkillAssignment) (badIdx int, ok bool) {
	for i, a := range assignments {
		if a.Proficiency < 1 || a.Proficiency > 10 {
			return i, false
		}
	}
	return 0, true
}

// validateNoDuplicateSkills rejects duplicate skill_ids in a single
// request body. Without this guard a duplicate falls through to
// InsertAgentSkill and hits the agent_skills UNIQUE constraint, which
// mapPgError translates to 409 version_conflict — the WRONG code for a
// malformed request body (Wave 4 cross-AI review).
func validateNoDuplicateSkills(assignments []api.AgentSkillAssignment) (dupIdx int, ok bool) {
	seen := make(map[uuid.UUID]struct{}, len(assignments))
	for i, a := range assignments {
		id := uuid.UUID(a.SkillId)
		if _, exists := seen[id]; exists {
			return i, false
		}
		seen[id] = struct{}{}
	}
	return 0, true
}

// replaceAgentSkills runs the D-76 cross-row probe + DELETE + N INSERTs
// against an EXISTING enclosing tx (Codex C4 iter 3). The helper does
// NOT open its own BeginTx — the caller (CreateAgent or UpdateAgent in
// agents.go) owns BeginTx + defer Rollback + Commit so the agent row
// write and the skills replace either both commit or both roll back.
//
// nil return → success. Non-nil result categorises the wire shape:
// ErrorCodeInternal → 500 (caller's defer Rollback handles cleanup);
// ErrorCodeInvalidReference → 422 (unknown / cross-org skill_id);
// other 422-class codes from mapPgError surface as 422 verbatim
// (defence-in-depth for FK + CHECK races between probe and INSERT).
func (h *Handlers) replaceAgentSkills(
	ctx context.Context,
	qtx *generated.Queries,
	orgID, agentID uuid.UUID,
	assignments []api.AgentSkillAssignment,
) *api.ErrorResponse {
	inputIDs := make([]uuid.UUID, 0, len(assignments))
	pgInputIDs := make([]pgtype.UUID, 0, len(assignments))
	for _, a := range assignments {
		id := uuid.UUID(a.SkillId)
		inputIDs = append(inputIDs, id)
		pgInputIDs = append(pgInputIDs, pgUUID(id))
	}

	present, err := qtx.SkillsPresentInOrg(ctx, generated.SkillsPresentInOrgParams{
		Column1: pgInputIDs,
		OrgID:   pgUUID(orgID),
	})
	if err != nil {
		return &api.ErrorResponse{
			Error:  api.ErrorCodeInternal,
			Reason: "skills_probe_failed",
		}
	}

	presentSet := make(map[uuid.UUID]struct{}, len(present))
	for _, p := range present {
		presentSet[uuid.UUID(p.Bytes)] = struct{}{}
	}
	for _, id := range inputIDs {
		if _, ok := presentSet[id]; !ok {
			return &api.ErrorResponse{
				Error:  api.ErrorCodeInvalidReference,
				Reason: fmt.Sprintf("unknown_skill_id:%s", id),
			}
		}
	}

	if _, err := qtx.DeleteAgentSkills(ctx, generated.DeleteAgentSkillsParams{
		AgentID: pgUUID(agentID),
		OrgID:   pgUUID(orgID),
	}); err != nil {
		return &api.ErrorResponse{
			Error:  api.ErrorCodeInternal,
			Reason: "delete_agent_skills_failed",
		}
	}

	for _, a := range assignments {
		if _, err := qtx.InsertAgentSkill(ctx, generated.InsertAgentSkillParams{
			AgentID:     pgUUID(agentID),
			SkillID:     pgUUID(uuid.UUID(a.SkillId)),
			OrgID:       pgUUID(orgID),
			Proficiency: mustInt32(a.Proficiency),
		}); err != nil {
			status, code, reason := mapPgError(err, "agent_skill")
			if status == 422 {
				return &api.ErrorResponse{Error: code, Reason: reason}
			}
			return &api.ErrorResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "insert_agent_skill_failed",
			}
		}
	}
	return nil
}
