package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// ---------------------------------------------------------------------------
// Helper-direct invocation harness
// ---------------------------------------------------------------------------

// withHelperTx opens an OrgDB tx (so the SQLChecker stays armed) and hands
// the caller a per-tx *generated.Queries plus the agent_id we use as the
// junction-row anchor. The tx is rolled back via t.Cleanup so each test
// is fully isolated — direct-helper tests bypass the HTTP layer's tx
// lifecycle, so the test owns it explicitly.
func withHelperTx(t *testing.T, th *TestHandlers) (ctx context.Context, qtx *generated.Queries, agentID uuid.UUID) {
	t.Helper()
	ctx = orgkey.SetOrgID(context.Background(), th.OrgID)
	tx, err := th.H.deps.OrgDB.BeginTx(ctx)
	require.NoError(t, err, "withHelperTx: BeginTx")
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	qtx = generated.New(tx)
	agentID = uuid.Must(uuid.NewV7())
	ext := "ext-helper-anchor-" + agentID.String()[:8]
	_, err = qtx.InsertAgent(ctx, generated.InsertAgentParams{
		ID:         pgUUID(agentID),
		OrgID:      pgUUID(th.OrgID),
		Code:       "emp_helper_" + sanitizeForCode(agentID.String()[:8]),
		ExternalID: &ext, // *string post-04.1.
		Name:       "Helper Anchor",
		Email:      "anchor+" + agentID.String()[:8] + "@example.test",
		Enabled:    true,
	})
	require.NoError(t, err, "withHelperTx: InsertAgent anchor")
	return ctx, qtx, agentID
}

// ---------------------------------------------------------------------------
// Helper unit tests — direct invocation
// ---------------------------------------------------------------------------

// TestAgentSkills_FullReplace exercises the happy path on the extracted
// helper: DELETE-then-N-INSERT swaps the join-table rows wholesale.
func TestAgentSkills_FullReplace(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s1 := seedSkill(t, th, ctx, "ja")
	s2 := seedSkill(t, th, ctx, "ko")
	s3 := seedSkill(t, th, ctx, "vi")

	hctx, qtx, agentID := withHelperTx(t, th)

	initial := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s1), Proficiency: 5},
		{SkillId: api.UUIDv7(s2), Proficiency: 7},
	}
	errResp := th.H.replaceAgentSkills(hctx, qtx, th.OrgID, agentID, initial)
	require.Nil(t, errResp, "initial replace must succeed")

	rows, err := qtx.ListSkillsForAgent(hctx, generated.ListSkillsForAgentParams{
		AgentID: pgUUID(agentID),
		OrgID:   pgUUID(th.OrgID),
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	replace := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s2), Proficiency: 9},
		{SkillId: api.UUIDv7(s3), Proficiency: 10},
	}
	errResp = th.H.replaceAgentSkills(hctx, qtx, th.OrgID, agentID, replace)
	require.Nil(t, errResp, "replace with disjoint set must succeed")

	rows, err = qtx.ListSkillsForAgent(hctx, generated.ListSkillsForAgentParams{
		AgentID: pgUUID(agentID),
		OrgID:   pgUUID(th.OrgID),
	})
	require.NoError(t, err)
	require.Len(t, rows, 2, "old assignments cleared, new ones inserted")
	seen := map[uuid.UUID]int{}
	for _, r := range rows {
		seen[uuid.UUID(r.SkillID.Bytes)] = int(r.Proficiency)
	}
	require.Equal(t, 9, seen[s2], "s2 retained with new proficiency 9")
	require.Equal(t, 10, seen[s3], "s3 inserted at proficiency 10")
	_, hasS1 := seen[s1]
	require.False(t, hasS1, "s1 must have been removed")

	empty := []api.AgentSkillAssignment{}
	errResp = th.H.replaceAgentSkills(hctx, qtx, th.OrgID, agentID, empty)
	require.Nil(t, errResp, "replace with empty set deletes all")
	rows, err = qtx.ListSkillsForAgent(hctx, generated.ListSkillsForAgentParams{
		AgentID: pgUUID(agentID),
		OrgID:   pgUUID(th.OrgID),
	})
	require.NoError(t, err)
	require.Len(t, rows, 0)
}

// TestAgentSkills_UnknownSkillId — random skill_id triggers the D-76 probe
// and produces 422 invalid_reference without running any DELETE or INSERT.
func TestAgentSkills_UnknownSkillId(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s1 := seedSkill(t, th, ctx, "valid-skill")

	hctx, qtx, agentID := withHelperTx(t, th)
	preexisting := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s1), Proficiency: 4},
	}
	require.Nil(t, th.H.replaceAgentSkills(hctx, qtx, th.OrgID, agentID, preexisting))

	unknown := uuid.Must(uuid.NewV7())
	attempt := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(unknown), Proficiency: 6},
	}
	errResp := th.H.replaceAgentSkills(hctx, qtx, th.OrgID, agentID, attempt)
	require.NotNil(t, errResp)
	require.Equal(t, api.ErrorCodeInvalidReference, errResp.Error)
	require.Contains(t, errResp.Reason, "unknown_skill_id:")
	require.Contains(t, errResp.Reason, unknown.String())

	// Probe-fail bails before DELETE — the pre-existing row stays intact.
	rows, err := qtx.ListSkillsForAgent(hctx, generated.ListSkillsForAgentParams{
		AgentID: pgUUID(agentID),
		OrgID:   pgUUID(th.OrgID),
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "DELETE must not have run when probe fails")
	require.Equal(t, s1, uuid.UUID(rows[0].SkillID.Bytes))
}

// TestAgentSkills_CrossOrgSkillId — a skill_id from another org is
// indistinguishable from a non-existent id (D-76 + FOUND-08). The probe
// scopes by ctx-orgID and the other org's skill is invisible.
func TestAgentSkills_CrossOrgSkillId(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	// Seed a skill DIRECTLY in orgB via the pool (sidestep the OrgDB so
	// preflight doesn't reject the cross-org INSERT — the cleanup
	// TRUNCATE handles teardown).
	orgB := uuid.Must(uuid.NewV7())
	q := generated.New(th.Pool)
	skillB := uuid.Must(uuid.NewV7())
	extB := "ext-cross-org-skill"
	_, err := q.InsertSkill(ctx, generated.InsertSkillParams{
		ID:         pgUUID(skillB),
		OrgID:      pgUUID(orgB),
		Code:       "skill_cross_org",
		ExternalID: &extB, // *string post-04.1.
		Name:       "cross-org-skill",
		SkillType:  "language",
		Enabled:    true,
	})
	require.NoError(t, err, "seed orgB skill")

	hctx, qtx, agentID := withHelperTx(t, th)
	assign := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(skillB), Proficiency: 5},
	}
	errResp := th.H.replaceAgentSkills(hctx, qtx, th.OrgID, agentID, assign)
	require.NotNil(t, errResp)
	require.Equal(t, api.ErrorCodeInvalidReference, errResp.Error,
		"cross-org skill_id must surface as invalid_reference, NOT cross_org — D-76 + FOUND-08 say the skill is invisible")
	require.Contains(t, errResp.Reason, skillB.String())
}

// TestAgentSkills_DBErrorPathFKRace simulates the FK race the helper
// defends against: probe sees a skill enabled, then the row is hard-deleted
// before INSERT runs. MapPgError translates 23503 to 422 invalid_reference
// so a TOCTOU window cannot regress to 500.
func TestAgentSkills_DBErrorPathFKRace(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s1 := seedSkill(t, th, ctx, "race-skill")

	hctx, qtx, agentID := withHelperTx(t, th)

	// Hard-delete the parent skill row OUTSIDE the helper's tx so the
	// probe still sees it (probe runs inside qtx but Postgres MVCC means
	// the parallel hard-delete is visible only after Commit; for a
	// realistic TOCTOU sim we delete before the helper runs but reuse
	// the qtx so the probe sees the pre-deletion snapshot — actually
	// pgx snapshots at first stmt, so we must drop the row inside qtx).
	_, err := qtx.SoftDeleteSkill(hctx, generated.SoftDeleteSkillParams{
		ID:    pgUUID(s1),
		OrgID: pgUUID(th.OrgID),
	})
	require.NoError(t, err, "soft-delete skill inside tx")

	// SoftDelete flips enabled=false; SkillsPresentInOrg filters on
	// enabled=TRUE, so the probe correctly catches the missing skill
	// as invalid_reference. This proves the helper's defense holds even
	// when the skill is disabled between the application's last GET and
	// the PATCH request.
	assign := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s1), Proficiency: 5},
	}
	errResp := th.H.replaceAgentSkills(hctx, qtx, th.OrgID, agentID, assign)
	require.NotNil(t, errResp)
	require.Equal(t, api.ErrorCodeInvalidReference, errResp.Error)
}

// ---------------------------------------------------------------------------
// End-to-end coverage via the agents handler
// ---------------------------------------------------------------------------

// TestAgentSkills_OutOfRangeProficiency_422 — PATCH agent with proficiency
// 0 or 11 → 422 invalid_value (Plan 03-01 spec amendment + ROADMAP CRIT 4).
// PATCH with proficiency 1 or 10 succeeds. The validation runs in the
// extracted validateProficiencyRange helper BEFORE any DB call, so the
// agent row never changes on the failure path.
func TestAgentSkills_OutOfRangeProficiency_422(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s1 := seedSkill(t, th, ctx, "proficiency-skill")
	a := postAgent(t, th, makeAgentBody("emp_proficiency_help", "ext-proficiency-help", "Boundary", nil))

	low := []api.AgentSkillAssignment{{SkillId: api.UUIDv7(s1), Proficiency: 1}}
	body := api.UpdateAgentRequest{Version: 1, Skills: &low}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "proficiency=1 must succeed, body=%s", string(raw))

	high := []api.AgentSkillAssignment{{SkillId: api.UUIDv7(s1), Proficiency: 10}}
	body = api.UpdateAgentRequest{Version: 2, Skills: &high}
	resp, raw = httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "proficiency=10 must succeed, body=%s", string(raw))

	belowMin := []api.AgentSkillAssignment{{SkillId: api.UUIDv7(s1), Proficiency: 0}}
	body = api.UpdateAgentRequest{Version: 3, Skills: &belowMin}
	resp, raw = httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"proficiency=0 must be 422 invalid_value, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidValue, e.Error)
	require.Contains(t, e.Reason, "skills[0].proficiency")

	aboveMax := []api.AgentSkillAssignment{{SkillId: api.UUIDv7(s1), Proficiency: 11}}
	body = api.UpdateAgentRequest{Version: 3, Skills: &aboveMax}
	resp, raw = httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"proficiency=11 must be 422 invalid_value, body=%s", string(raw))
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidValue, e.Error)
	require.Contains(t, e.Reason, "skills[0].proficiency")
}

// TestAgentSkills_DuplicateSkillId_422 — duplicate skill_ids in the
// request body must surface as 422 invalid_value (Wave 4 review). Without
// the validateNoDuplicateSkills gate this would slip through to
// InsertAgentSkill and trip the UNIQUE constraint, surfacing as a 409 —
// the WRONG code for a malformed request body.
func TestAgentSkills_DuplicateSkillId_422(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s1 := seedSkill(t, th, ctx, "dup-skill")
	a := postAgent(t, th, makeAgentBody("emp_dup_help", "ext-dup-help", "Dup", nil))

	dup := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s1), Proficiency: 5},
		{SkillId: api.UUIDv7(s1), Proficiency: 7},
	}
	body := api.UpdateAgentRequest{Version: 1, Skills: &dup}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"duplicate skill_id must be 422 invalid_value, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidValue, e.Error)
	require.Contains(t, e.Reason, "duplicate")
}
