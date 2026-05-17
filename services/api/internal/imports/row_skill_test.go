// row_skill_test.go — testcontainers integration suite for the
// skill row processor (flat shape; no FK, no nested arrays).
//
// Test inventory (≥ 4 cases per plan):
//
//   - HappyPath_FirstImport — new skill lands.
//   - InvalidCodeFormat_PerRow — Layer 1 reject.
//   - Reimport_UpdatesViaUpsertPath — same code → ON CONFLICT UPDATE.
//   - DuplicateExternalID_PerRow — partial unique index violation.
package imports

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// rawSkillRow builds a JSON-shaped map for ImportSkillRequest. The
// `external_id` and `description` columns are optional.
func rawSkillRow(code, name, skillType string) map[string]interface{} {
	return map[string]interface{}{
		"code":       code,
		"name":       name,
		"skill_type": skillType,
	}
}

func fetchSkillByCode(t testing.TB, ctx context.Context, th *TestImports, code string) (int, string) {
	t.Helper()
	var n int
	var name string
	err := th.Pool.QueryRow(ctx,
		`SELECT count(*), COALESCE(MAX(name), '') FROM skills WHERE org_id = $1 AND code = $2`,
		th.OrgID, code,
	).Scan(&n, &name)
	require.NoError(t, err)
	return n, name
}

func TestProcessRow_Skill_HappyPath_FirstImport(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawSkillRow("skill_voice", "Voice Skill", "core")},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Skills, &skillRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)
	n, name := fetchSkillByCode(t, ctx, th, "skill_voice")
	require.Equal(t, 1, n)
	require.Equal(t, "Voice Skill", name)
}

func TestProcessRow_Skill_InvalidCodeFormat_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawSkillRow("INVALID UPPER", "Bad Skill", "core")},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Skills, &skillRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.NotNil(t, failed[0].Field)
	require.Equal(t, "code", *failed[0].Field)
	require.Equal(t, "invalid_code_format", failed[0].Reason)
}

func TestProcessRow_Skill_Reimport_UpdatesViaUpsertPath(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed the skill with the old name.
	_ = seedSkillRaw(t, ctx, th, "skill_voice", "Voice Old", "core")

	// Re-import with a new name (same code).
	rows := []parsedRow{
		{lineNo: 1, raw: rawSkillRow("skill_voice", "Voice New", "core")},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Skills, &skillRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1, "re-import is an UPDATE path; must succeed")
	require.Len(t, failed, 0)
	n, name := fetchSkillByCode(t, ctx, th, "skill_voice")
	require.Equal(t, 1, n)
	require.Equal(t, "Voice New", name, "ON CONFLICT UPDATE must update the name")
}

func TestProcessRow_Skill_DuplicateExternalID_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed a skill with external_id="HR-SKILL-001".
	id := uuid.Must(uuid.NewV7())
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO skills (id, org_id, code, external_id, name, skill_type, enabled, version, created_at, updated_at)
		 VALUES ($1, $2, 'skill_voice', 'HR-SKILL-001', 'Voice', 'core', TRUE, 1, NOW(), NOW())`,
		id, th.OrgID,
	)
	require.NoError(t, err)

	externalID := "HR-SKILL-001"
	rows := []parsedRow{
		{lineNo: 1, raw: map[string]interface{}{
			"code":        "skill_chat",
			"external_id": externalID,
			"name":        "Chat",
			"skill_type":  "core",
		}},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Skills, &skillRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.Contains(t, failed[0].Reason, "duplicate_external_id")
}
