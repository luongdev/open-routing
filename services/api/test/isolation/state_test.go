// state_test.go carries the D-94 cross-org isolation suite for the agent state
// machine endpoints. Four probes verify FOUND-08 holds for GET/PATCH /status:
//   - GET cross-org → 404
//   - PATCH cross-org agent → 404
//   - PATCH cross-org break_reason_id → 422 invalid_reference
//   - PATCH with force=true + cross-org break_reason_id → still 422 (Pitfall 3 gate)
//
// One additional probe verifies D-93 atomic seeding: CreateAgent immediately
// seeds an agent_states row so GET /status returns 200 (not 404) after creation.
//
// Uses helpers from isolation_test.go (baseURL, freshOrg, requireContainer,
// postEntity) and catalog_test.go (postEntity). State-specific HTTP helpers
// (getStatusEndpoint, patchStatusReturnCode) are defined here.
package isolation_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// getStatusEndpoint issues GET /v1/orgs/{orgID}/agents/{agentID}/status and
// returns the HTTP status code. The X-Org-Id header is set to orgID so the
// OrgContext middleware accepts the request.
func getStatusEndpoint(t *testing.T, urlBase string, orgID, agentID uuid.UUID) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/v1/orgs/%s/agents/%s/status", urlBase, orgID, agentID),
		nil)
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// patchStatusReturnCode issues PATCH /v1/orgs/{orgID}/agents/{agentID}/status
// and returns (statusCode, raw body). The X-Org-Id header is set to orgID.
func patchStatusReturnCode(t *testing.T, urlBase string, orgID, agentID uuid.UUID, body any) (int, []byte) {
	t.Helper()
	buf, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch,
		fmt.Sprintf("%s/v1/orgs/%s/agents/%s/status", urlBase, orgID, agentID),
		bytes.NewReader(buf))
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}

// TestState_GetStatusCrossOrg_404 — D-94(a). GET /agents/{id}/status from
// orgB when the agent belongs to orgA must return 404. Neither the
// agent_states row nor the agent row should be visible across org boundaries
// (FOUND-08 invariant).
func TestState_GetStatusCrossOrg_404(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)

	// CreateAgent atomically seeds agent_states (D-93) so the state row exists.
	codeA, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
		"external_id": "iso-state-a", "name": "A", "email": "a@example.com",
	})
	require.Equal(t, http.StatusCreated, codeA)

	// Same-org GET: proves agent_states row was seeded.
	require.Equal(t, http.StatusOK, getStatusEndpoint(t, baseURL(), orgA, agentA),
		"same-org GET must return 200 (D-93 seeding gate)")

	// Cross-org GET: orgB MUST NOT observe orgA's state row.
	require.Equal(t, http.StatusNotFound, getStatusEndpoint(t, baseURL(), orgB, agentA),
		"cross-org GET must return 404 (FOUND-08)")
}

// TestState_PatchStatusCrossOrgAgent_404 — D-94(d). PATCH /agents/{id}/status
// where the agent belongs to orgA but the request carries orgB's context must
// return 404 (agent invisible across org boundary — FOUND-08).
func TestState_PatchStatusCrossOrgAgent_404(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)

	codeA, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
		"external_id": "iso-state-pcr-a", "name": "A", "email": "a@example.com",
	})
	require.Equal(t, http.StatusCreated, codeA)

	// PATCH agent owned by orgA, but X-Org-Id set to orgB — must 404.
	code, _ := patchStatusReturnCode(t, baseURL(), orgB, agentA, map[string]any{
		"to": "Ready",
	})
	require.Equal(t, http.StatusNotFound, code,
		"PATCH against orgA agent from orgB context must return 404 (FOUND-08)")
}

// TestState_PatchStatusCrossOrgBreakReason_422 — D-94(b). PATCH to Break
// using a break_reason_id that belongs to orgB while the request is for
// orgA's agent must return 422 invalid_reference. The cross-org probe fires
// before any matrix or DB UPDATE logic (Pitfall 3 — probe runs first).
//
// Setup sequence:
//  1. Seed orgA's agent (starts in Offline per D-93).
//  2. Force-PATCH to NotReady so we can reach Ready via matrix.
//  3. Matrix-PATCH to Ready.
//  4. PATCH to Break with orgB's break_reason_id → 422.
func TestState_PatchStatusCrossOrgBreakReason_422(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)

	codeA, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
		"external_id": "iso-state-pco-a", "name": "A", "email": "a@example.com",
	})
	require.Equal(t, http.StatusCreated, codeA)

	// Seed break_reason in orgB.
	codeB, brB := postEntity(t, baseURL(), "break-reasons", orgB, map[string]any{
		"name": "OtherOrgBreak", "routable": false, "display_order": 0,
	})
	require.Equal(t, http.StatusCreated, codeB)

	// Offline is a system-only initial state; force-PATCH to NotReady to
	// enter the agent-initiated matrix (D-82 — Offline→NotReady is system-only).
	code, _ := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
		"to": "NotReady", "force": true,
	})
	require.Equal(t, http.StatusOK, code, "force Offline→NotReady must succeed")

	// Matrix-PATCH NotReady→Ready (valid agent-initiated edge).
	code, _ = patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{"to": "Ready"})
	require.Equal(t, http.StatusOK, code, "NotReady→Ready matrix edge must succeed")

	// PATCH Ready→Break with orgB's break_reason_id — must 422 invalid_reference.
	code, raw := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
		"to": "Break", "break_reason_id": brB.String(),
	})
	require.Equal(t, http.StatusUnprocessableEntity, code,
		"cross-org break_reason_id must return 422 invalid_reference (D-94b), body=%s", string(raw))

	var errBody api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &errBody))
	require.Equal(t, api.ErrorCodeInvalidReference, errBody.Error)
}

// TestState_ForceDoesNotBypassCrossOrgBreakReason_422 — D-94(c) + Pitfall 3
// regression gate. force=true bypasses the TRANSITION MATRIX only; the
// cross-row break_reason probe MUST still run and reject orgB's break_reason_id.
//
// This test intentionally issues the PATCH from the agent's initial Offline
// state — the probe fires BEFORE any matrix check, so Offline is irrelevant.
// If this test ever fails, force=true is leaking across org boundaries.
func TestState_ForceDoesNotBypassCrossOrgBreakReason_422(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)

	codeA, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
		"external_id": "iso-state-fcb-a", "name": "A", "email": "a@example.com",
	})
	require.Equal(t, http.StatusCreated, codeA)

	codeB, brB := postEntity(t, baseURL(), "break-reasons", orgB, map[string]any{
		"name": "OtherOrgBreakForce", "routable": false, "display_order": 0,
	})
	require.Equal(t, http.StatusCreated, codeB)

	// Even with force=true the cross-row probe fires before matrix bypass.
	code, raw := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
		"to": "Break", "break_reason_id": brB.String(), "force": true,
	})
	require.Equal(t, http.StatusUnprocessableEntity, code,
		"force=true MUST NOT bypass cross-org break_reason probe (Pitfall 3), body=%s", string(raw))

	var errBody api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &errBody))
	require.Equal(t, api.ErrorCodeInvalidReference, errBody.Error,
		"expected invalid_reference even with force=true")
}

// TestCreateAgentSeedsState — D-93 + Pitfall 6 regression gate. CreateAgent
// MUST atomically INSERT into agent_states in the same transaction. A
// subsequent GET /status must return 200 (not 404). If this test fails,
// the atomic seeding path in catalog/agents.go was broken.
func TestCreateAgentSeedsState(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	org := freshOrg(t)

	code, agentID := postEntity(t, baseURL(), "agents", org, map[string]any{
		"external_id": "iso-state-cas", "name": "Seeded", "email": "s@example.com",
	})
	require.Equal(t, http.StatusCreated, code)

	// GET /status immediately after CreateAgent must succeed because the
	// agent_states INSERT is in the same tx as the agents INSERT (D-93).
	// 404 here means either the INSERT was skipped or ran outside the tx.
	require.Equal(t, http.StatusOK, getStatusEndpoint(t, baseURL(), org, agentID),
		"GET /status immediately after CreateAgent must succeed (D-93 atomic seeding)")
}
