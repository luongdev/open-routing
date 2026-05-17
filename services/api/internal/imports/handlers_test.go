// handlers_test.go — Wave 5 integration tests for the bulk-import
// endpoint (Plan 05-07). Every test in this file goes through the
// full chi mux: RequestID → orgContext → BodyLimit → strict-server →
// (*Importer).BulkImportCatalog / GetImportJob → chunk loop → DB.
//
// Coverage areas in this file:
//   - Entity × format matrix: 6 entities × {JSON, CSV} happy paths
//     (≥ 12 tests prefixed TestBulkImport_JSON_<Entity>_HappyPath_200
//     and TestBulkImport_CSV_<Entity>_HappyPath_200).
//   - Per-row failure shapes: partial-success (207) + all-fail (422)
//     + invalid_code_format (per-row Reason).
//   - Header policy: missing-column (400), unknown-column (400),
//     unsupported entity (400).
//   - Agent-specific: skill merge preservation (D5-18), agent_states
//     re-import preservation (Hazard 7), unknown skill_code per-row.
//
// CSV-spec edge tests (BOM + CRLF + invalid UTF-8 + oversize +
// schema_version) live in handlers_csv_test.go (Task 2).
// Idempotency tests live in idempotency_test.go (Task 3). import_jobs
// lifecycle tests live in jobs_test.go (Task 3). Cross-org isolation
// + migration idempotency tests live in test/isolation/ (Task 4).
package imports

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// errFinaliseInjected — sentinel used by Phase 5 fix H3 test injection
// to force finaliseJob to fail. Defined as a package-level var so the
// test's override closure can return the same identity-comparable
// error on every call (none of the production code branches on
// identity, but the sentinel keeps the test reason readable).
var errFinaliseInjected = errors.New("finalise injected failure")

// ---------------------------------------------------------------------------
// Agents — JSON + CSV happy paths.
// ---------------------------------------------------------------------------

// TestBulkImport_JSON_Agents_HappyPath_200 — POST testdata/agents-basic.json
// → 200 + 3 succeeded UUIDs + 0 failed + the import_jobs row carries
// status=completed.
func TestBulkImport_JSON_Agents_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := decodeJSONArray(t, loadTestData(t, "agents-basic.json"))
	resp, body := postImportJSON(t, th, api.Agents, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))

	r := decodeBulkImportResult(t, body)
	require.Len(t, r.Succeeded, 3, "agents-basic.json has 3 rows; body=%s", string(body))
	require.Empty(t, r.Failed)

	// Verify import_jobs lifecycle row exists with terminal status.
	jobID := extractLatestJobID(t, ctx, th.Pool, th.OrgID)
	status, total, succ, fail, _ := fetchImportJob(t, ctx, th.Pool, th.OrgID, jobID)
	require.Equal(t, "completed", status)
	require.Equal(t, 3, total)
	require.Equal(t, 3, succ)
	require.Equal(t, 0, fail)
}

// TestBulkImport_CSV_Agents_HappyPath_200 — POST testdata/agents-basic.csv
// → 200 + 3 succeeded + 0 failed.
func TestBulkImport_CSV_Agents_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.Agents, loadTestData(t, "agents-basic.csv"))
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	r := decodeBulkImportResult(t, body)
	require.Len(t, r.Succeeded, 3)
	require.Empty(t, r.Failed)
}

// TestBulkImport_JSON_Agents_WithSkills_200 — pre-seed 3 skills via raw SQL;
// POST agents-with-skills.json → 200 + each agent's agent_skills join row
// is written (D5-15..D5-19).
func TestBulkImport_JSON_Agents_WithSkills_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed the 3 skills referenced by agents-with-skills.json.
	seedSkillForImports(t, ctx, th.Pool, th.OrgID, "skill_voice", "Voice", "language")
	seedSkillForImports(t, ctx, th.Pool, th.OrgID, "skill_chat", "Chat", "language")
	seedSkillForImports(t, ctx, th.Pool, th.OrgID, "skill_email", "Email", "language")

	rows := decodeJSONArray(t, loadTestData(t, "agents-with-skills.json"))
	resp, body := postImportJSON(t, th, api.Agents, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	r := decodeBulkImportResult(t, body)
	require.Len(t, r.Succeeded, 2, "two agents imported; body=%s", string(body))
	require.Empty(t, r.Failed)

	// Verify agent_skills join rows exist for emp_skilled_001 (2) +
	// emp_skilled_002 (1).
	aID, _ := fetchAgentByCode(t, ctx, th.Pool, th.OrgID, "emp_skilled_001")
	bID, _ := fetchAgentByCode(t, ctx, th.Pool, th.OrgID, "emp_skilled_002")
	require.Equal(t, 2, fetchAgentSkillCount(t, ctx, th.Pool, th.OrgID, aID))
	require.Equal(t, 1, fetchAgentSkillCount(t, ctx, th.Pool, th.OrgID, bID))
}

// TestBulkImport_CSV_Agents_WithSkills_200 — same as the JSON variant but
// using the D5-17 CSV `skills` column syntax `code:prof|code:prof`.
func TestBulkImport_CSV_Agents_WithSkills_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	seedSkillForImports(t, ctx, th.Pool, th.OrgID, "skill_voice", "Voice", "language")
	seedSkillForImports(t, ctx, th.Pool, th.OrgID, "skill_chat", "Chat", "language")
	seedSkillForImports(t, ctx, th.Pool, th.OrgID, "skill_email", "Email", "language")

	resp, body := postImportCSV(t, th, api.Agents, loadTestData(t, "agents-with-skills.csv"))
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	r := decodeBulkImportResult(t, body)
	require.Len(t, r.Succeeded, 2)
	require.Empty(t, r.Failed)

	aID, _ := fetchAgentByCode(t, ctx, th.Pool, th.OrgID, "emp_skilled_001")
	require.Equal(t, 2, fetchAgentSkillCount(t, ctx, th.Pool, th.OrgID, aID))
}

// ---------------------------------------------------------------------------
// Skills — JSON + CSV happy paths.
// ---------------------------------------------------------------------------

func TestBulkImport_JSON_Skills_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []interface{}{
		map[string]interface{}{"code": "skill_voice", "name": "Voice", "skill_type": "language"},
		map[string]interface{}{"code": "skill_chat", "name": "Chat", "skill_type": "language"},
		map[string]interface{}{"code": "skill_email", "name": "Email", "skill_type": "language"},
	}
	resp, body := postImportJSON(t, th, api.Skills, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	require.Len(t, decodeBulkImportResult(t, body).Succeeded, 3)
}

func TestBulkImport_CSV_Skills_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.Skills, loadTestData(t, "skills-basic.csv"))
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	require.Len(t, decodeBulkImportResult(t, body).Succeeded, 3)
}

// ---------------------------------------------------------------------------
// Queues — JSON + CSV happy paths.
// ---------------------------------------------------------------------------

func TestBulkImport_JSON_Queues_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	pri := 1
	acw := 30
	rows := []interface{}{
		map[string]interface{}{
			"code": "queue_main", "name": "Main",
			"channel_types": []string{"voice", "chat"},
			"priority":      pri, "acw_sec": acw,
		},
		map[string]interface{}{
			"code": "queue_vip", "name": "VIP",
			"channel_types": []string{"voice"},
			"priority":      5, "acw_sec": 60,
		},
	}
	resp, body := postImportJSON(t, th, api.Queues, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	require.Len(t, decodeBulkImportResult(t, body).Succeeded, 2)
}

func TestBulkImport_CSV_Queues_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.Queues, loadTestData(t, "queues-basic.csv"))
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	require.Len(t, decodeBulkImportResult(t, body).Succeeded, 2)
}

// ---------------------------------------------------------------------------
// Channels — JSON + CSV happy paths (FK code-lookup via default_queue_code).
// ---------------------------------------------------------------------------

func TestBulkImport_JSON_Channels_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed queue queue_main so the channel's default_queue_code FK
	// probe (D-76) finds a referent.
	seedQueueForImports(t, ctx, th.Pool, th.OrgID, "queue_main", "Main")

	dqc := "queue_main"
	rows := []interface{}{
		map[string]interface{}{
			"code": "ch_voice_main", "name": "Main Voice",
			"channel_type": "voice", "default_queue_code": dqc,
		},
		map[string]interface{}{
			"code": "ch_chat_main", "name": "Main Chat",
			"channel_type": "chat", "default_queue_code": dqc,
		},
	}
	resp, body := postImportJSON(t, th, api.Channels, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	require.Len(t, decodeBulkImportResult(t, body).Succeeded, 2)
}

func TestBulkImport_CSV_Channels_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	seedQueueForImports(t, ctx, th.Pool, th.OrgID, "queue_main", "Main")

	resp, body := postImportCSV(t, th, api.Channels, loadTestData(t, "channels-basic.csv"))
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	require.Len(t, decodeBulkImportResult(t, body).Succeeded, 2)
}

// ---------------------------------------------------------------------------
// Adapters — JSON + CSV happy paths (Config JSONB passthrough).
// ---------------------------------------------------------------------------

func TestBulkImport_JSON_Adapters_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []interface{}{
		map[string]interface{}{
			"code": "adapter_twilio", "name": "Twilio", "adapter_type": "voice",
			"config": map[string]interface{}{"endpoint": "https://twilio.example"},
		},
		map[string]interface{}{
			"code": "adapter_zendesk", "name": "Zendesk", "adapter_type": "chat",
			"config": map[string]interface{}{"endpoint": "https://zendesk.example"},
		},
	}
	resp, body := postImportJSON(t, th, api.Adapters, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	require.Len(t, decodeBulkImportResult(t, body).Succeeded, 2)
}

func TestBulkImport_CSV_Adapters_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.Adapters, loadTestData(t, "adapters-basic.csv"))
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	require.Len(t, decodeBulkImportResult(t, body).Succeeded, 2)
}

// ---------------------------------------------------------------------------
// BreakReasons — JSON + CSV happy paths. IMPORTANT: two rows have the
// SAME name "Lunch" but DIFFERENT codes — Phase 04.1 IDENT-03 (D04_1-05)
// dropped UNIQUE(org_id, name); both MUST land successfully.
// ---------------------------------------------------------------------------

func TestBulkImport_JSON_BreakReasons_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []interface{}{
		map[string]interface{}{"code": "br_lunch_1", "name": "Lunch"},
		map[string]interface{}{"code": "br_lunch_2", "name": "Lunch"},
		map[string]interface{}{"code": "br_training", "name": "Training"},
	}
	resp, body := postImportJSON(t, th, api.BreakReasons, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	require.Len(t, decodeBulkImportResult(t, body).Succeeded, 3,
		"IDENT-03 mirror: two br_lunch_* with same name 'Lunch' MUST both succeed; body=%s",
		string(body))
}

func TestBulkImport_CSV_BreakReasons_HappyPath_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.BreakReasons, loadTestData(t, "break_reasons-basic.csv"))
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
	require.Len(t, decodeBulkImportResult(t, body).Succeeded, 3,
		"IDENT-03 mirror via CSV; body=%s", string(body))
}

// ---------------------------------------------------------------------------
// Error-path tests — partial success (207), all-fail (422), header policy.
// ---------------------------------------------------------------------------

// TestBulkImport_FailedRow_Structure_207 — POST partial-success.csv → 207
// with structured Failed[] entries (D5-12 / IMP-04 + IMP-05).
func TestBulkImport_FailedRow_Structure_207(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.Agents, loadTestData(t, "partial-success.csv"))
	require.Equal(t, http.StatusMultiStatus, resp.StatusCode,
		"partial-success.csv has 3 valid + 1 invalid; body=%s", string(body))
	r := decodeBulkImportResult(t, body)
	require.NotEmpty(t, r.Succeeded, "at least one row must succeed; body=%s", string(body))
	require.NotEmpty(t, r.Failed, "at least one row must fail; body=%s", string(body))

	// Validate the failed[] structure shape (IMP-04 / D5-12).
	for i, f := range r.Failed {
		require.NotZero(t, f.Row, "failed[%d].row must be 1-based non-zero; got %+v", i, f)
		require.Equal(t, api.BulkImportFailedRowErrorImportFailed, f.Error,
			"failed[%d].error MUST be import_failed (closed enum); got %q", i, f.Error)
		require.NotEmpty(t, f.Reason, "failed[%d].reason MUST be present; got %+v", i, f)
	}
}

// TestBulkImport_HTTPStatus_AllFail_422 — POST full-failure.csv → 422.
func TestBulkImport_HTTPStatus_AllFail_422(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.Agents, loadTestData(t, "full-failure.csv"))
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"all rows invalid → 422; body=%s", string(body))
	r := decodeBulkImportResult(t, body)
	require.Empty(t, r.Succeeded)
	require.Len(t, r.Failed, 3)
}

// TestBulkImport_HTTPStatus_AllSucceed_200 — happy-path 3 valid rows → 200.
// Companion to AllFail_422 to assert the 200/422 dispatch matrix end-to-end.
func TestBulkImport_HTTPStatus_AllSucceed_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.Agents, loadTestData(t, "agents-basic.csv"))
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))
}

// TestBulkImport_InvalidCodeFormat_PerRow — POST invalid-code-format.csv
// (1 uppercase + 1 valid) → 207 with Failed[0].Field="code" +
// Reason="invalid_code_format".
func TestBulkImport_InvalidCodeFormat_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, body := postImportCSV(t, th, api.Agents, loadTestData(t, "invalid-code-format.csv"))
	require.Equal(t, http.StatusMultiStatus, resp.StatusCode,
		"1 invalid + 1 valid = 207; body=%s", string(body))
	r := decodeBulkImportResult(t, body)
	require.Len(t, r.Succeeded, 1)
	require.Len(t, r.Failed, 1)
	require.Equal(t, "invalid_code_format", r.Failed[0].Reason)
	require.NotNil(t, r.Failed[0].Field, "field must populate on per-row failure")
	require.Equal(t, "code", *r.Failed[0].Field)
}

// TestBulkImport_MissingRequiredColumn_400 — CSV header missing required
// `name` column → 400 invalid_header (D5-05).
func TestBulkImport_MissingRequiredColumn_400(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	csvBody := []byte("code,email\nemp_x,x@example.com\n")
	resp, body := postImportCSV(t, th, api.Agents, csvBody)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(body))
	require.Contains(t, string(body), "invalid_header")
	require.Contains(t, string(body), "name")
}

// TestBulkImport_UnknownColumn_400 — CSV header carries `salary` (not in
// the agents registry) → 400 invalid_header (D5-05 forward-compat).
func TestBulkImport_UnknownColumn_400(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	csvBody := []byte("code,name,email,salary\nemp_x,X,x@example.com,100\n")
	resp, body := postImportCSV(t, th, api.Agents, csvBody)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(body))
	require.Contains(t, string(body), "invalid_header")
	require.Contains(t, string(body), "salary")
}

// TestBulkImport_UnknownEntity_400 — entity=widgets → 400 unsupported_entity.
func TestBulkImport_UnknownEntity_400(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}

	// Build the URL with entity=widgets directly (the helper hardcodes
	// the entity query param).
	u := th.HTTP.URL + importPath(th.OrgID) + "?entity=widgets"
	req, err := http.NewRequest(http.MethodPost, u, nil)
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", th.OrgID.String())
	req.Header.Set("Content-Type", "application/json")
	resp, err := th.HTTP.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	// Strict-server rejects unknown enum values at decode time with 400.
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Agent-specific edge tests — Hazard 7, Pitfall 3, D5-18 merge, D5-16
// unknown skill_code.
// ---------------------------------------------------------------------------

// TestBulkImport_SeedsAgentStatesForNewAgents — Hazard 7. POST 3 agents;
// each must have a corresponding agent_states row with status='Offline'
// + state_version=1.
func TestBulkImport_SeedsAgentStatesForNewAgents(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, _ := postImportCSV(t, th, api.Agents, loadTestData(t, "agents-basic.csv"))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	for _, code := range []string{"emp_001", "emp_002", "emp_003"} {
		agentID, _ := fetchAgentByCode(t, ctx, th.Pool, th.OrgID, code)
		status := fetchAgentStateStatusByOrg(t, ctx, th.Pool, th.OrgID, agentID)
		require.Equal(t, "Offline", status,
			"Hazard 7: new agents must have agent_states.status=Offline; code=%s", code)
	}
}

// TestBulkImport_Reimport_PreservesAgentStateMachine — Pitfall 3.
// Pre-seed an agent at Ready; POST the same code; assert state stays Ready
// (ON CONFLICT DO NOTHING on agent_states; D5-18 PATCH-like agents path).
func TestBulkImport_Reimport_PreservesAgentStateMachine(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed an agent already at Ready state.
	preID := seedAgentForImports(t, ctx, th.Pool, th.OrgID,
		"emp_reimport", "Original Name", "orig@example.com", "Ready")
	require.Equal(t, "Ready", fetchAgentStateStatusByOrg(t, ctx, th.Pool, th.OrgID, preID))

	// Import the same code (UpsertAgentByCode will UPDATE — Phase 04.1).
	rows := []interface{}{
		map[string]interface{}{
			"code": "emp_reimport", "name": "Updated Name",
			"email": "updated@example.com",
		},
	}
	resp, body := postImportJSON(t, th, api.Agents, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))

	// The agent row id is preserved (UpsertAgentByCode returns the
	// existing UUID via ON CONFLICT … RETURNING). The state must remain
	// Ready (Pitfall 3 / Hazard 7).
	id, name := fetchAgentByCode(t, ctx, th.Pool, th.OrgID, "emp_reimport")
	require.Equal(t, preID, id, "code-keyed upsert must preserve the agent UUID")
	require.Equal(t, "Updated Name", name, "name must update via UPDATE branch")
	require.Equal(t, "Ready",
		fetchAgentStateStatusByOrg(t, ctx, th.Pool, th.OrgID, id),
		"Pitfall 3: re-import must NOT regress agent_states.status to Offline")
}

// TestBulkImport_AgentImport_SkillMerge_PreservesExisting — D5-18 + D5-19.
// Pre-seed an agent with 2 skills (skill_voice@5 + skill_chat@5); POST an
// import row that mentions ONLY skill_voice at proficiency 9; assert:
//   - the existing skill_chat row is still present (PATCH-like merge)
//   - the skill_voice proficiency is now 9 (import wins on conflict)
func TestBulkImport_AgentImport_SkillMerge_PreservesExisting(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	voiceID := seedSkillForImports(t, ctx, th.Pool, th.OrgID, "skill_voice", "Voice", "language")
	chatID := seedSkillForImports(t, ctx, th.Pool, th.OrgID, "skill_chat", "Chat", "language")
	agentID := seedAgentForImports(t, ctx, th.Pool, th.OrgID,
		"emp_merge", "Original Name", "merge@example.com", "Offline")
	seedAgentSkillAssignment(t, ctx, th.Pool, th.OrgID, agentID, voiceID, 5)
	seedAgentSkillAssignment(t, ctx, th.Pool, th.OrgID, agentID, chatID, 5)

	rows := []interface{}{
		map[string]interface{}{
			"code": "emp_merge", "name": "Merged Name", "email": "merge@example.com",
			"skills": []interface{}{
				map[string]interface{}{"skill_code": "skill_voice", "proficiency": 9},
			},
		},
	}
	resp, body := postImportJSON(t, th, api.Agents, rows)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(body))

	// MERGE semantics: skill_chat MUST still exist.
	require.Equal(t, 2, fetchAgentSkillCount(t, ctx, th.Pool, th.OrgID, agentID),
		"D5-18 MERGE: skill_chat assignment must NOT be deleted by an import that omits it")
	// D5-19 import wins: skill_voice proficiency now 9.
	require.Equal(t, 9, fetchAgentSkillProficiency(t, ctx, th.Pool, th.OrgID, agentID, voiceID),
		"D5-19: import value wins on (agent_id, skill_id) proficiency conflict")
	require.Equal(t, 5, fetchAgentSkillProficiency(t, ctx, th.Pool, th.OrgID, agentID, chatID),
		"D5-18 MERGE: skill_chat proficiency unchanged (not in payload)")
}

// TestBulkImport_AgentImport_UnknownSkillCode_PerRow — D5-16.
// POST agents-with-skills.json with a skill that was NOT pre-seeded.
// Expect 422 (all-fail since the only row fails) + per-row reason
// "unknown_skill".
func TestBulkImport_AgentImport_UnknownSkillCode_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed skill_voice but NOT skill_chat — so the first agent fails.
	seedSkillForImports(t, ctx, th.Pool, th.OrgID, "skill_voice", "Voice", "language")

	rows := []interface{}{
		map[string]interface{}{
			"code": "emp_unknown_skill", "name": "X", "email": "x@example.com",
			"skills": []interface{}{
				map[string]interface{}{"skill_code": "skill_does_not_exist", "proficiency": 5},
			},
		},
	}
	resp, body := postImportJSON(t, th, api.Agents, rows)
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"all rows failed → 422; body=%s", string(body))
	r := decodeBulkImportResult(t, body)
	require.Len(t, r.Failed, 1)
	require.Equal(t, "unknown_skill", r.Failed[0].Reason)
	require.NotNil(t, r.Failed[0].Field)
	require.Contains(t, *r.Failed[0].Field, "skills[",
		"unknown skill failure must report the skills[N].skill_code path")
}

// TestBulkImport_FinaliseJobFails_Returns500 — Phase 5 fix H3.
// When `finaliseJob` (the audit-row UPDATE that flips status=pending →
// completed/failed) errors out, the handler MUST return HTTP 500
// signalling that the import data did commit but the persisted audit
// row stayed in `pending`. Pre-fix the handler logged warn and returned
// the success/partial result anyway, poisoning the audit trail (GET /
// imports returned status=pending with zero counters, idempotency
// replay returned empty result, the 24h sweep overwrote with
// server_crash). Test seam: WithFinaliseOverride lets us force the
// error without destructive schema mutations.
func TestBulkImport_FinaliseJobFails_Returns500(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Inject the failure hook. Same package → direct field access (no
	// exported setter needed; the override field is intentionally
	// unexported so production wiring cannot set it).
	th.I.finaliseOverride = func(
		_ context.Context,
		_, _ uuid.UUID,
		_ string,
		_, _ int,
		_ []byte,
	) error {
		return errFinaliseInjected
	}

	rows := []interface{}{
		map[string]interface{}{"code": "emp_h3_fin", "name": "Alice", "email": "alice@example.com"},
	}
	resp, body := postImportJSON(t, th, api.Agents, rows)
	require.Equalf(t, http.StatusInternalServerError, resp.StatusCode,
		"H3: finaliseJob failure must surface as 500, not 200; body=%s", string(body))

	// The data did land — verify the agent row exists despite the audit
	// failure (this is what makes H3 nuanced: the import IS committed,
	// only the audit step failed). The status code surfaces the
	// inconsistency so admin can verify via GET /imports/{id}.
	var agentRows int
	err := th.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agents WHERE org_id = $1 AND code = 'emp_h3_fin'`,
		th.OrgID).Scan(&agentRows)
	require.NoError(t, err)
	require.Equal(t, 1, agentRows,
		"H3: per-row chunk commit happened before finaliseJob; agent row must exist")
}
