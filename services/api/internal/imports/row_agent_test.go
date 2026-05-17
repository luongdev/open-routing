// row_agent_test.go — testcontainers Postgres integration suite for
// the agent row processor.
//
// Test inventory (≥ 8 cases per plan):
//
//   - HappyPath_FirstImport — new agent + 2 skills land; state row seeded.
//   - Reimport_PreservesState — pre-existing Ready state is not regressed.
//   - InvalidOwnCodeFormat_PerRow — Layer 1 reject on agent.code.
//   - NestedSkillCode_InvalidFormat_PerRow — Pitfall 9 — Layer 1 reject
//     on skills[i].skill_code BEFORE ResolveSkillCodes runs.
//   - UnknownSkillCode_PerRow — resolver missing → unknown_skill.
//   - SkillMerge_PreservesExisting — D5-18 MERGE / D5-19 import wins.
//   - DuplicateExternalID_PerRow — 23505 on partial unique index.
//   - AgentStateSeedFails_RowFailedNotPartial — seeded row + bad state
//     would corrupt state machine; SAVEPOINT rolls back fully.
package imports

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// fetchAgentStateStatus returns the persisted agent_states.status for
// (agent_id, org_id). Returns "" if absent.
func fetchAgentStateStatus(t testing.TB, ctx context.Context, th *TestImports, agentID uuid.UUID) string {
	t.Helper()
	var s string
	row := th.Pool.QueryRow(ctx,
		`SELECT status FROM agent_states WHERE agent_id = $1 AND org_id = $2`,
		agentID, th.OrgID,
	)
	if err := row.Scan(&s); err != nil {
		return ""
	}
	return s
}

func TestProcessRow_Agent_HappyPath_FirstImport(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	seedSkillRaw(t, ctx, th, "skill_voice", "Voice", "core")
	seedSkillRaw(t, ctx, th, "skill_chat", "Chat", "core")

	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRowWithSkills("alice", "Alice", "alice@example.com",
			[]map[string]interface{}{
				{"skill_code": "skill_voice", "proficiency": 7},
				{"skill_code": "skill_chat", "proficiency": 5},
			})},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)

	// Agent row + agent_states row + 2 agent_skills rows all landed.
	require.Equal(t, 1, fetchAgentCountByCode(t, ctx, th, "alice"))
	require.Equal(t, "Offline", fetchAgentStateStatus(t, ctx, th, succeeded[0].id))
	voice, chat := fetchAgentSkillProficiencies(t, ctx, th, "alice")
	require.Equal(t, 7, voice)
	require.Equal(t, 5, chat)
}

func TestProcessRow_Agent_Reimport_PreservesState(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed an agent with status=Ready.
	id := uuid.Must(uuid.NewV7())
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO agents (id, org_id, code, name, email, enabled, version, created_at, updated_at)
		 VALUES ($1, $2, 'alice', 'Alice Old', 'alice-old@example.com', TRUE, 1, NOW(), NOW())`,
		id, th.OrgID,
	)
	require.NoError(t, err)
	_, err = th.Pool.Exec(ctx,
		`INSERT INTO agent_states (agent_id, org_id, status, state_version, updated_at)
		 VALUES ($1, $2, 'Ready', 1, NOW())`,
		id, th.OrgID,
	)
	require.NoError(t, err)

	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRow("alice", "Alice New", "alice-new@example.com")},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)

	// Hazard 7 / Pitfall 3 — re-imports MUST NOT regress state to Offline.
	require.Equal(t, "Ready", fetchAgentStateStatus(t, ctx, th, id),
		"re-import must preserve Ready; ON CONFLICT DO NOTHING is the safeguard")
}

func TestProcessRow_Agent_InvalidOwnCodeFormat_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRow("INVALID UPPER", "Bob", "bob@example.com")},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.NotNil(t, failed[0].Field)
	require.Equal(t, "code", *failed[0].Field)
	require.Equal(t, "invalid_code_format", failed[0].Reason)
}

func TestProcessRow_Agent_NestedSkillCode_InvalidFormat_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRowWithSkills("alice", "Alice", "alice@example.com",
			[]map[string]interface{}{
				{"skill_code": "INVALID UPPER", "proficiency": 5},
			})},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.NotNil(t, failed[0].Field)
	require.Equal(t, "skills[0].skill_code", *failed[0].Field)
	require.Equal(t, "invalid_code_format", failed[0].Reason,
		"Pitfall 9 — Layer 1 must fire BEFORE ResolveSkillCodes")
}

func TestProcessRow_Agent_UnknownSkillCode_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Deliberately do NOT pre-seed any skills — resolver returns empty.
	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRowWithSkills("alice", "Alice", "alice@example.com",
			[]map[string]interface{}{
				{"skill_code": "skill_nonexistent", "proficiency": 5},
			})},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.NotNil(t, failed[0].Field)
	require.Equal(t, "skills[0].skill_code", *failed[0].Field)
	require.Equal(t, "unknown_skill", failed[0].Reason)
	// Agent must NOT have landed.
	require.Equal(t, 0, fetchAgentCountByCode(t, ctx, th, "alice"),
		"savepoint must roll back fully when an unknown skill is referenced")
}

func TestProcessRow_Agent_SkillMerge_PreservesExisting(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed agent + two skills + the skill_chat assignment with
	// proficiency=3. Import provides skill_voice only with prof=9.
	agentID := seedAgentRaw(t, ctx, th, "alice", "Alice Old", "alice-old@example.com")
	voiceID := seedSkillRaw(t, ctx, th, "skill_voice", "Voice", "core")
	chatID := seedSkillRaw(t, ctx, th, "skill_chat", "Chat", "core")

	_, err := th.Pool.Exec(ctx,
		`INSERT INTO agent_skills (agent_id, skill_id, org_id, proficiency)
		 VALUES ($1, $2, $3, 3)`,
		agentID, chatID, th.OrgID,
	)
	require.NoError(t, err)

	// Import a row that includes only skill_voice with prof=9.
	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRowWithSkills("alice", "Alice New", "alice-new@example.com",
			[]map[string]interface{}{
				{"skill_code": "skill_voice", "proficiency": 9},
			})},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)

	// D5-18 — skill_chat MUST still be present (NOT deleted by the
	// import which lacked it). D5-19 — skill_voice must have prof=9
	// (the new value; import wins).
	voice, chat := fetchAgentSkillProficiencies(t, ctx, th, "alice")
	require.Equal(t, 9, voice, "D5-19 import wins on proficiency conflict")
	require.Equal(t, 3, chat, "D5-18 skill_chat (not in payload) must be preserved")

	// Reference the seeded UUIDs to defend against future schema changes.
	_, _ = voiceID, chatID
}

func TestProcessRow_Agent_DuplicateExternalID_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Seed an agent with external_id="HR-EMP-001".
	id := uuid.Must(uuid.NewV7())
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO agents (id, org_id, code, external_id, name, email, enabled, version, created_at, updated_at)
		 VALUES ($1, $2, 'alice', 'HR-EMP-001', 'Alice', 'alice@example.com', TRUE, 1, NOW(), NOW())`,
		id, th.OrgID,
	)
	require.NoError(t, err)
	_, err = th.Pool.Exec(ctx,
		`INSERT INTO agent_states (agent_id, org_id, status, state_version, updated_at)
		 VALUES ($1, $2, 'Offline', 1, NOW())`,
		id, th.OrgID,
	)
	require.NoError(t, err)

	// Now import a DIFFERENT code carrying the same external_id.
	externalID := "HR-EMP-001"
	rows := []parsedRow{
		{lineNo: 1, raw: map[string]interface{}{
			"code":        "bob",
			"external_id": externalID,
			"name":        "Bob",
			"email":       "bob@example.com",
		}},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.Contains(t, failed[0].Reason, "duplicate_external_id",
		"Phase 04.1 partial unique on (org_id, external_id) must surface duplicate_external_id via MapPgError")
}

// TestProcessRow_Agent_AgentStateSeedFails_RowFailedNotPartial — Phase 5
// fix L2. Pre-fix this test was a t.Skip with rationale that the
// agent_states CHECK constraint made the failure path hard to force in
// integration. The L2 fix-up installs a TEMPORARY CHECK constraint on
// agent_states that DENIES status='Offline' for the test's duration,
// runs the import (which always inserts status='Offline' per
// row_agent.go Step 6), asserts the row fails with reason=
// agent_state_seed_failed, and unconditionally drops the temporary
// constraint at the end (defer t.Cleanup).
//
// Why not a mock at the unit layer: the agent row processor is
// transactional — agent upsert + state seed share one savepoint, and
// a state-seed failure must roll the agent insert back. Asserting that
// behaviour requires a real DB; a pure unit test cannot exercise it.
func TestProcessRow_Agent_AgentStateSeedFails_RowFailedNotPartial(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Install a TEMPORARY CHECK constraint that forbids 'Offline' —
	// the agent row processor hardcodes status='Offline' on every
	// fresh state seed, so this guarantees InsertAgentStateOnConflictNothing
	// will fail with a CHECK violation (23514).
	//
	// We use a NOT VALID + VALIDATE pair to ensure the constraint
	// applies to subsequent INSERTs while still letting any already-
	// present rows from prior tests remain. The Cleanup drops it
	// before the next test runs.
	_, err := th.Pool.Exec(ctx,
		`ALTER TABLE agent_states ADD CONSTRAINT chk_l2_forbid_offline
		 CHECK (status <> 'Offline') NOT VALID`)
	require.NoError(t, err, "L2: install temporary CHECK constraint")
	t.Cleanup(func() {
		_, dropErr := th.Pool.Exec(context.Background(),
			`ALTER TABLE agent_states DROP CONSTRAINT chk_l2_forbid_offline`)
		require.NoError(t, dropErr, "L2: drop temporary CHECK constraint")
	})

	// Import a fresh agent. The processor will:
	//   1. Upsert agents — succeeds.
	//   2. InsertAgentStateOnConflictNothing(status='Offline') — fires the
	//      temporary CHECK constraint, returns 23514.
	//   3. Row processor returns *rowError{reason: "agent_state_seed_failed"}.
	//   4. Chunk loop ROLLBACK TO savepoint — both agent + state writes
	//      unwind together (atomicity proven by the post-test row count).
	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRow("emp_l2_a", "Alice L2", "alice-l2@example.com")},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents,
		&agentRowProc{handlers: th.I}, rows)

	require.Len(t, succeeded, 0,
		"L2: state-seed CHECK violation must NOT leave the row in succeeded[]")
	require.Len(t, failed, 1)
	require.Equal(t, "agent_state_seed_failed", failed[0].Reason,
		"L2: row processor must surface agent_state_seed_failed reason")

	// Atomicity guarantee — the savepoint ROLLBACK TO unwinds the agent
	// insert too. Pre-existing rows are untouched by cleanImportTables.
	var agentRowCount int
	require.NoError(t, th.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agents WHERE org_id = $1 AND code = 'emp_l2_a'`,
		th.OrgID).Scan(&agentRowCount))
	require.Equal(t, 0, agentRowCount,
		"L2: savepoint atomicity — agent insert must roll back when state seed fails")
}
