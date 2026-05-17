// row_agent.go — per-row processor for entity=agents (D5-15..D5-19).
//
// The agent row processor is the most complex of the six:
//
//   1. reMarshalAs into the typed ImportAgentRequest.
//   2. Layer 1 validation of the agent's own `code` (D04_1-03 regex).
//   3. Layer 1 validation of EACH nested skill_code (Pitfall 9 — must
//      fire BEFORE ResolveSkillCodes else a malformed code reaches the
//      DB lookup as a query parameter).
//   4. Skill UUID resolution via the chunk-level resolver (one DB
//      round-trip per chunk; per-row code → uuid lookup is O(1) map
//      reads). Missing codes → unknown_skill at the first failing index.
//   5. qtx.UpsertAgentByCode (Phase 04.1 — INSERT ... ON CONFLICT
//      (org_id, code) DO UPDATE ... RETURNING). The savepoint Tx is
//      pre-routed by the chunk loop; this processor NEVER opens its
//      own tx.
//   6. qtx.InsertAgentStateOnConflictNothing (Plan 05-03; Hazard 7 —
//      seeds state for new agents only; re-imports preserve whatever
//      state Phase 4's state machine has reached).
//   7. qtx.MergeAgentSkill per skill in the payload (Plan 05-03;
//      D5-18 PATCH-like merge — existing skills NOT in payload are
//      LEFT INTACT; D5-19 import wins on proficiency conflicts).
//   8. Return succeededRow{} with the row's UUID + lineNo + agents
//      entity tag so chunk.go's post-commit cache.Del invalidates the
//      right `or:{orgID}:agents:{id}` key.
//
// Anti-pattern enforcement (RESEARCH Pitfall 10 + Anti-Patterns list):
//
//   - NEVER call catalog.replaceAgentSkills (PUT semantic). D5-18
//     mandates MERGE; this file uses qtx.MergeAgentSkill exclusively.
//   - NEVER call generated.New(s.deps.OrgDB) — every per-row query
//     goes through generated.New(sp) so the savepoint can ROLLBACK
//     TO on failure without dragging in unrelated writes.
package imports

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/catalog"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// agentRowProc is the rowProcessor implementation for entity=agents.
// Constructed once per chunk by chunk.processChunk's dispatch and
// reused across the 50-row loop; the struct itself carries no
// per-row state.
//
// `handlers` captures the *Importer so the processor can reach
// s.deps.Logger for per-row warn events that the chunk loop's tx-level
// logging would otherwise hide.
type agentRowProc struct {
	handlers *Importer
}

// process implements the rowProcessor contract. See file header for
// the 8-step pipeline.
func (p *agentRowProc) process(
	ctx context.Context,
	sp *db.OrgTx,
	orgID uuid.UUID,
	r parsedRow,
	resolver *chunkSkillResolver,
) (succeededRow, *rowError) {
	// Step 1: typed decode. The strict-server eager-decode put each
	// row's []interface{} item into r.raw; we re-marshal into the
	// typed shape. A decode failure means the row's JSON shape did
	// not match ImportAgentRequest (e.g., wrong types on a field).
	typed, err := reMarshalAs[api.ImportAgentRequest](r.raw)
	if err != nil {
		return succeededRow{}, &rowError{Field: "", Reason: "invalid_json_row"}
	}

	// Step 2: Layer 1 — own code regex (D04_1-03 / Pitfall 9). Must
	// fire BEFORE any DB lookup so a malformed CSV/JSON row never
	// reaches the UpsertAgentByCode call (and never wastes a chunk
	// savepoint slot on a guaranteed failure).
	if !catalog.ValidateCodeFormat(typed.Code) {
		return succeededRow{}, &rowError{Field: "code", Reason: "invalid_code_format"}
	}

	// Step 3: Layer 1 — nested skill_code regex (Pitfall 9). Iterate
	// in payload order so the field index in the error message
	// matches the admin's input position. Stop at the first failing
	// skill — admins re-import after fixing the first invalid token.
	if typed.Skills != nil {
		for i, sk := range *typed.Skills {
			if !catalog.ValidateCodeFormat(sk.SkillCode) {
				return succeededRow{}, &rowError{
					Field:  fmt.Sprintf("skills[%d].skill_code", i),
					Reason: "invalid_code_format",
				}
			}
		}
	}

	// Step 4: Skill UUID resolution via the chunk-level resolver.
	// Collect codes in payload order so missing[0] indexes the FIRST
	// unknown skill in the agent's own payload, NOT the first in the
	// chunk-wide set.
	codes := agentSkillCodes(typed)
	var skillUUIDs map[string]uuid.UUID
	if len(codes) > 0 {
		if resolver == nil {
			// Defensive — chunk.processChunk SHOULD pass a non-nil
			// resolver for the Agents entity. A nil resolver means the
			// chunk loop is bypassed (e.g., test calling process
			// directly without resolveChunkSkillCodes). Surface as a
			// row-level failure so the test fails cleanly.
			return succeededRow{}, &rowError{Field: "", Reason: "skill_resolver_missing"}
		}
		uuids, missing := resolver.resolveAll(codes)
		if len(missing) > 0 {
			return succeededRow{}, &rowError{
				Field:  fmt.Sprintf("skills[%d].skill_code", missing[0]),
				Reason: "unknown_skill",
			}
		}
		skillUUIDs = uuids
	}

	// Step 5: Upsert the agent. UUIDv7 minted up front — discarded by
	// the ON CONFLICT path (existing row's id flows through RETURNING).
	id := uuid.Must(uuid.NewV7())
	qtx := generated.New(sp)
	row, upErr := qtx.UpsertAgentByCode(ctx, generated.UpsertAgentByCodeParams{
		ID:         pgUUID(id),
		OrgID:      pgUUID(orgID),
		Code:       typed.Code,
		ExternalID: typed.ExternalId,
		Name:       typed.Name,
		Email:      string(typed.Email),
		Enabled:    derefBool(typed.Enabled, true),
	})
	if upErr != nil {
		return succeededRow{}, wrapPgError(upErr, "agent")
	}

	// Step 6: Seed agent_states for NEW agents only (Hazard 7). The
	// ON CONFLICT DO NOTHING ensures re-imports preserve whatever
	// status the state machine has reached (Ready / Engaged / WrapUp).
	// Failures here propagate as row_failed — the savepoint will
	// ROLLBACK TO so neither the agent row nor the state row land.
	if stErr := qtx.InsertAgentStateOnConflictNothing(ctx, generated.InsertAgentStateOnConflictNothingParams{
		AgentID: row.ID,
		OrgID:   pgUUID(orgID),
		Status:  string(api.AgentStatusOffline),
	}); stErr != nil {
		p.handlers.deps.Logger.WarnContext(ctx, "import.agent.state_seed_failed",
			"agent_id", uuid.UUID(row.ID.Bytes), "err", stErr)
		return succeededRow{}, &rowError{Field: "", Reason: "agent_state_seed_failed"}
	}

	// Step 7: Merge agent_skills (D5-18 + D5-19). Existing skills
	// NOT in the payload are left intact (no DELETE issued); skills
	// IN the payload get their proficiency overwritten if previously
	// set (ON CONFLICT (agent_id, skill_id) DO UPDATE SET proficiency).
	if typed.Skills != nil && len(*typed.Skills) > 0 {
		for i, sk := range *typed.Skills {
			skillID, ok := skillUUIDs[sk.SkillCode]
			if !ok {
				// Unreachable in practice — resolver.resolveAll already
				// reported missing codes above. Defensive: if a skill
				// passed validation but the map lookup misses, surface
				// the row-level failure with the payload index.
				return succeededRow{}, &rowError{
					Field:  fmt.Sprintf("skills[%d].skill_code", i),
					Reason: "unknown_skill",
				}
			}
			if mErr := qtx.MergeAgentSkill(ctx, generated.MergeAgentSkillParams{
				AgentID:     row.ID,
				SkillID:     pgUUID(skillID),
				OrgID:       pgUUID(orgID),
				Proficiency: int32(sk.Proficiency),
			}); mErr != nil {
				p.handlers.deps.Logger.WarnContext(ctx, "import.agent.merge_skill_failed",
					"agent_id", uuid.UUID(row.ID.Bytes),
					"skill_code", sk.SkillCode, "err", mErr)
				return succeededRow{}, &rowError{
					Field:  fmt.Sprintf("skills[%d]", i),
					Reason: "merge_skill_failed",
				}
			}
		}
	}

	// Step 8: success. Capture the canonical id from RETURNING so
	// post-commit cache.Del targets the correct row (the ON CONFLICT
	// UPDATE path returns the EXISTING row's id, not the freshly
	// minted UUIDv7).
	return succeededRow{
		id:     uuid.UUID(row.ID.Bytes),
		entity: api.Agents,
		lineNo: r.lineNo,
	}, nil
}

// agentSkillCodes returns the skill_code values from typed.Skills in
// payload order. Nil/empty Skills → nil slice (resolveAll handles the
// empty case). Kept as a pure helper so tests can build the resolver
// directly without re-decoding the request body.
func agentSkillCodes(typed api.ImportAgentRequest) []string {
	if typed.Skills == nil || len(*typed.Skills) == 0 {
		return nil
	}
	out := make([]string, 0, len(*typed.Skills))
	for _, sk := range *typed.Skills {
		out = append(out, sk.SkillCode)
	}
	return out
}
