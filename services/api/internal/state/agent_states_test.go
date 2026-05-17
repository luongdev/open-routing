package state

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
)

// ---------------------------------------------------------------------------
// GET /agents/{id}/status
// ---------------------------------------------------------------------------

func TestGetAgentStatus_404_AgentMissing(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	resp, raw := httpGETStatus(t, th, agentID)
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "expected 404 for unknown agent, body=%s", raw)

	var body api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, api.ErrorCodeNotFound, body.Error)
}

func TestGetAgentStatus_200_FromCache(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-cache-test", "Cache Test Agent")
	// Seed with Ready so we have a non-Offline row to assert
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "Ready",
		StateVersion: 1,
	})

	// First GET: cache miss → DB load → cache.Set
	resp, raw := httpGETStatus(t, th, agentID)
	require.Equal(t, http.StatusOK, resp.StatusCode, "first GET want 200, body=%s", raw)

	var state1 api.AgentState
	require.NoError(t, json.Unmarshal(raw, &state1))
	require.Equal(t, api.AgentStatusReady, state1.Status)

	// After first GET, miniredis must carry the cache key (cache hit path).
	cacheKey := cache.Key(th.OrgID, "agent_state", agentID)
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key must exist after first GET")

	// Second GET: cache hit — still 200, same status.
	resp2, raw2 := httpGETStatus(t, th, agentID)
	require.Equal(t, http.StatusOK, resp2.StatusCode, "second GET want 200, body=%s", raw2)

	var state2 api.AgentState
	require.NoError(t, json.Unmarshal(raw2, &state2))
	require.Equal(t, api.AgentStatusReady, state2.Status)
}

// ---------------------------------------------------------------------------
// PATCH /agents/{id}/status — happy paths
// ---------------------------------------------------------------------------

func TestPatchAgentStatus_AllowedTransition_HappyPath(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-happy", "Happy Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "NotReady",
		StateVersion: 1,
	})

	body := api.PatchAgentStatusRequest{To: api.AgentStatusReady}
	resp, raw := httpPATCHStatus(t, th, agentID, body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "NotReady→Ready want 200, body=%s", raw)

	var state api.AgentState
	require.NoError(t, json.Unmarshal(raw, &state))
	require.Equal(t, api.AgentStatusReady, state.Status)
	require.Equal(t, 2, state.StateVersion, "state_version must increment to 2")
}

func TestPatchAgentStatus_StateVersionMonotonic(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-version", "Version Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "NotReady",
		StateVersion: 1,
	})

	// PATCH 1: NotReady → Ready (version 1 → 2)
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{To: api.AgentStatusReady})
	require.Equal(t, http.StatusOK, resp.StatusCode, "PATCH 1 body=%s", raw)
	var s1 api.AgentState
	require.NoError(t, json.Unmarshal(raw, &s1))
	require.Equal(t, 2, s1.StateVersion)

	// PATCH 2: Ready → NotReady (version 2 → 3)
	resp2, raw2 := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{To: api.AgentStatusNotReady})
	require.Equal(t, http.StatusOK, resp2.StatusCode, "PATCH 2 body=%s", raw2)
	var s2 api.AgentState
	require.NoError(t, json.Unmarshal(raw2, &s2))
	require.Equal(t, 3, s2.StateVersion)

	// PATCH 3: NotReady → Ready (version 3 → 4)
	resp3, raw3 := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{To: api.AgentStatusReady})
	require.Equal(t, http.StatusOK, resp3.StatusCode, "PATCH 3 body=%s", raw3)
	var s3 api.AgentState
	require.NoError(t, json.Unmarshal(raw3, &s3))
	require.Equal(t, 4, s3.StateVersion)
}

// ---------------------------------------------------------------------------
// PATCH — transition matrix (STATE-03 / 409)
// ---------------------------------------------------------------------------

func TestPatchAgentStatus_InvalidTransition_409(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-invalid", "Invalid Agent")
	// Engaged is a system-only state; no agent-initiated edges from Engaged
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "Engaged",
		StateVersion: 1,
	})

	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{To: api.AgentStatusReady})
	require.Equal(t, http.StatusConflict, resp.StatusCode, "Engaged→Ready want 409, body=%s", raw)

	var body api.InvalidTransitionErrorResponse
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, api.AgentStatusEngaged, body.From, "from must be Engaged")
	require.Equal(t, api.AgentStatusReady, body.To, "to must be Ready")
	require.Equal(t, api.InvalidTransition, body.Error)
}

// ---------------------------------------------------------------------------
// PATCH — break_reason probe (STATE-04 / 422)
// ---------------------------------------------------------------------------

func TestPatchAgentStatus_BreakReasonRequired_422(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-brreq", "BR Required Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "Ready",
		StateVersion: 1,
	})

	// PATCH to Break with nil break_reason_id must 422 invalid_value
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{
		To: api.AgentStatusBreak,
		// break_reason_id intentionally omitted
	})
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "want 422 when break_reason_id missing, body=%s", raw)

	var body api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, api.ErrorCodeInvalidValue, body.Error)
	require.Equal(t, "break_reason_id_required_for_break", body.Reason)
}

func TestPatchAgentStatus_BreakReasonProbe_422_missing(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-brmiss", "BR Missing Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "Ready",
		StateVersion: 1,
	})

	nonExistentID := uuid.Must(uuid.NewV7())
	brID := api.UUIDv7(nonExistentID)
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{
		To:            api.AgentStatusBreak,
		BreakReasonId: &brID,
	})
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "non-existent break_reason_id want 422, body=%s", raw)

	var body api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, api.ErrorCodeInvalidReference, body.Error)
}

func TestPatchAgentStatus_BreakReasonProbe_200_valid(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-brvalid", "BR Valid Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "Ready",
		StateVersion: 1,
	})

	breakReasonID := uuid.Must(uuid.NewV7())
	seedBreakReason(t, th.Pool, th.OrgID, breakReasonID, "Short Break", false)

	brID := api.UUIDv7(breakReasonID)
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{
		To:            api.AgentStatusBreak,
		BreakReasonId: &brID,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode, "valid break_reason_id want 200, body=%s", raw)

	var state api.AgentState
	require.NoError(t, json.Unmarshal(raw, &state))
	require.Equal(t, api.AgentStatusBreak, state.Status)
	require.NotNil(t, state.BreakReasonId, "break_reason_id must be set in response")
	require.Equal(t, breakReasonID, uuid.UUID(*state.BreakReasonId))
}

// ---------------------------------------------------------------------------
// PATCH — force=true paths (D-84 / Pitfall 3 regression gate)
// ---------------------------------------------------------------------------

func TestPatchAgentStatus_Force_BypassesMatrix(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-force", "Force Agent")
	// Engaged has no agent-initiated matrix edges (system-only per D-82)
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "Engaged",
		StateVersion: 1,
	})

	forceTrue := true
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{
		To:    api.AgentStatusNotReady,
		Force: &forceTrue,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode, "force=true Engaged→NotReady want 200 (matrix bypassed), body=%s", raw)

	var state api.AgentState
	require.NoError(t, json.Unmarshal(raw, &state))
	require.Equal(t, api.AgentStatusNotReady, state.Status)
}

// TestPatchAgentStatus_Force_DoesNotBypassBreakReason is the Pitfall 3
// regression gate: force=true MUST bypass the transition matrix only —
// the cross-row break_reason probe MUST still run (D-84 / T-04-06).
func TestPatchAgentStatus_Force_DoesNotBypassBreakReason(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-force-br", "Force BR Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "Ready",
		StateVersion: 1,
	})

	// The break_reason belongs to a DIFFERENT org — cross-org probe must reject.
	otherOrgID := uuid.Must(uuid.NewV7())
	crossOrgBreakReasonID := uuid.Must(uuid.NewV7())
	seedBreakReason(t, th.Pool, otherOrgID, crossOrgBreakReasonID, "OtherOrg Break", false)

	forceTrue := true
	brID := api.UUIDv7(crossOrgBreakReasonID)
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{
		To:            api.AgentStatusBreak,
		BreakReasonId: &brID,
		Force:         &forceTrue,
	})
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"force=true with cross-org break_reason_id must still return 422, body=%s", raw)

	var body api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, api.ErrorCodeInvalidReference, body.Error,
		"must be invalid_reference not invalid_transition (probe ran before matrix bypass)")
}

// ---------------------------------------------------------------------------
// PATCH — STATE-06: post_interaction_state set while Engaged via force
// ---------------------------------------------------------------------------

func TestPatchAgentStatus_PostInteractionState_SetViaForce(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-pis", "PIS Agent")
	// Seed Engaged with no post_interaction_state
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "Engaged",
		StateVersion: 1,
	})

	// PATCH Engaged→Engaged (no matrix edge) with force=true to set PIS.
	// STATE-06: agent CAN set post_interaction_state while Engaged.
	forceTrue := true
	pisReady := api.PostInteractionStateReady
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{
		To:                   api.AgentStatusEngaged,
		Force:                &forceTrue,
		PostInteractionState: &pisReady,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode, "force Engaged→Engaged with PIS want 200, body=%s", raw)

	var state api.AgentState
	require.NoError(t, json.Unmarshal(raw, &state))
	require.Equal(t, api.AgentStatusEngaged, state.Status)
	require.NotNil(t, state.PostInteractionState, "post_interaction_state must be set")
	require.Equal(t, api.PostInteractionStateReady, *state.PostInteractionState)
}

// ---------------------------------------------------------------------------
// PATCH — 404
// ---------------------------------------------------------------------------

func TestPatchAgentStatus_404_AgentMissing(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{To: api.AgentStatusReady})
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "missing agent want 404, body=%s", raw)
}

// ===========================================================================
// Acceptance tests — one per ROADMAP Phase 4 §Success Criteria.
// ===========================================================================

// TestAcceptance_AllTransitions — ROADMAP §1.
// Every allowed agent-initiated edge returns 200; every system-only/invalid
// transition returns 409 with InvalidTransitionErrorResponse.
func TestAcceptance_AllTransitions(t *testing.T) {
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	brID := uuid.Must(uuid.NewV7())
	seedBreakReason(t, th.Pool, th.OrgID, brID, "AccBreak", false)
	brUUID := api.UUIDv7(brID)

	type tc struct {
		name      string
		seedFrom  string
		patchBody any
		wantCode  int
	}
	cases := []tc{
		{"NotReady→Ready allowed", "NotReady",
			api.PatchAgentStatusRequest{To: api.AgentStatusReady}, http.StatusOK},
		{"Ready→NotReady allowed", "Ready",
			api.PatchAgentStatusRequest{To: api.AgentStatusNotReady}, http.StatusOK},
		{"Ready→Break allowed with break_reason", "Ready",
			api.PatchAgentStatusRequest{To: api.AgentStatusBreak, BreakReasonId: &brUUID}, http.StatusOK},
		{"Break→Ready allowed", "Break",
			api.PatchAgentStatusRequest{To: api.AgentStatusReady}, http.StatusOK},
		{"Break→NotReady allowed", "Break",
			api.PatchAgentStatusRequest{To: api.AgentStatusNotReady}, http.StatusOK},
		{"WrapUp→Ready allowed", "WrapUp",
			api.PatchAgentStatusRequest{To: api.AgentStatusReady}, http.StatusOK},
		{"WrapUp→NotReady allowed", "WrapUp",
			api.PatchAgentStatusRequest{To: api.AgentStatusNotReady}, http.StatusOK},
		// System-only / invalid transitions → 409.
		{"Engaged→Ready rejected", "Engaged",
			api.PatchAgentStatusRequest{To: api.AgentStatusReady}, http.StatusConflict},
		{"Offline→Ready rejected", "Offline",
			api.PatchAgentStatusRequest{To: api.AgentStatusReady}, http.StatusConflict},
		{"NotReady→Engaged rejected", "NotReady",
			api.PatchAgentStatusRequest{To: api.AgentStatusEngaged}, http.StatusConflict},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			agentID := uuid.Must(uuid.NewV7())
			seedAgent(t, th.Pool, th.OrgID, agentID, "ag-acc-"+c.name, "AG")
			seedAgentStateRow(t, th.Pool, SeedStateParams{
				AgentID:      agentID,
				OrgID:        th.OrgID,
				Status:       c.seedFrom,
				StateVersion: 1,
			})
			resp, raw := httpPATCHStatus(t, th, agentID, c.patchBody)
			require.Equal(t, c.wantCode, resp.StatusCode, "body=%s", string(raw))
			if c.wantCode == http.StatusConflict {
				var err409 api.InvalidTransitionErrorResponse
				require.NoError(t, json.Unmarshal(raw, &err409))
				require.Equal(t, api.InvalidTransition, err409.Error)
				require.Equal(t, api.AgentStatus(c.seedFrom), err409.From,
					"409 must carry the observed current status")
			}
		})
	}
}

// TestAcceptance_BreakReasonValidation — ROADMAP §2.
// Valid same-org break_reason → 200 with reason recorded on the state row.
// (Cross-org rejection is covered by the isolation suite and
// TestPatchAgentStatus_Force_DoesNotBypassBreakReason.)
func TestAcceptance_BreakReasonValidation(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	brID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "a-acc-2", "A")
	seedBreakReason(t, th.Pool, th.OrgID, brID, "ValidBreak", false)
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "Ready",
		StateVersion: 1,
	})

	brUUID := api.UUIDv7(brID)
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{
		To:            api.AgentStatusBreak,
		BreakReasonId: &brUUID,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var state api.AgentState
	require.NoError(t, json.Unmarshal(raw, &state))
	require.NotNil(t, state.BreakReasonId)
	require.Equal(t, brID, uuid.UUID(*state.BreakReasonId),
		"break_reason_id must be recorded on the state row (ROADMAP §2)")
}

// TestAcceptance_WrapUpExpiresWithoutClient — ROADMAP §3.
// Agent in WrapUp transitions to post_interaction_state automatically
// after wrapup_until expires. Uses the real clock with a very short TTL
// to validate end-to-end behavior through the Server lifecycle.
func TestAcceptance_WrapUpExpiresWithoutClient(t *testing.T) {
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	// Fresh Server with real clock + short sweep interval so the safety
	// sweep can catch the timer if AfterFunc misfires.
	s := New(Deps{
		OrgDB:  th.S.deps.OrgDB,
		Cache:  th.S.deps.Cache,
		Logger: th.S.deps.Logger,
	}, WithSweepInterval(200*time.Millisecond))
	require.NoError(t, s.Start(ctx))
	defer s.Stop()

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "a-acc-3", "A")
	wrapupUntil := time.Now().Add(1 * time.Second)
	pis := "ready"
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:              agentID,
		OrgID:                th.OrgID,
		Status:               "WrapUp",
		WrapupUntil:          &wrapupUntil,
		PostInteractionState: &pis,
		StateVersion:         1,
	})
	s.scheduleWrapUpExpiry(agentID, th.OrgID, wrapupUntil)

	require.Eventually(t, func() bool {
		row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
		return row.Status == "Ready" && !row.WrapupUntil.Valid && row.StateVersion >= 2
	}, 8*time.Second, 100*time.Millisecond,
		"WrapUp expiry never fired with real clock (ROADMAP §3 TTL acceptance)")
}

// TestAcceptance_IsRoutableMatrix — ROADMAP §4.
// IsRoutable returns true only for Ready or Break+routable=true; all other
// statuses return false. The exhaustive 4-case (now 10-case) table lives in
// internal/domain/state_test.go::TestIsRoutable. This wrapper exists for
// ROADMAP §4 test-name traceability in /gsd-verify-work.
func TestAcceptance_IsRoutableMatrix(t *testing.T) {
	t.Log("ROADMAP §4 — STATE-10 IsRoutable covered by internal/domain/state_test.go::TestIsRoutable (10 cases)")
}

// TestAcceptance_StateVersionMonotonic — ROADMAP §5.
// Every PATCH increments state_version by exactly 1. Five sequential
// transitions assert versions 1→2→3→4→5→6.
func TestAcceptance_StateVersionMonotonic(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	brID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "a-acc-5", "A")
	seedBreakReason(t, th.Pool, th.OrgID, brID, "VerBreak", false)
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "NotReady",
		StateVersion: 1,
	})

	brUUID := api.UUIDv7(brID)
	transitions := []any{
		api.PatchAgentStatusRequest{To: api.AgentStatusReady},
		api.PatchAgentStatusRequest{To: api.AgentStatusBreak, BreakReasonId: &brUUID},
		api.PatchAgentStatusRequest{To: api.AgentStatusReady},
		api.PatchAgentStatusRequest{To: api.AgentStatusNotReady},
		api.PatchAgentStatusRequest{To: api.AgentStatusReady},
	}
	expectedVer := 2
	for i, body := range transitions {
		resp, raw := httpPATCHStatus(t, th, agentID, body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "transition %d body=%s", i, string(raw))
		var state api.AgentState
		require.NoError(t, json.Unmarshal(raw, &state))
		require.Equal(t, expectedVer, state.StateVersion,
			"state_version monotonic violation at step %d (ROADMAP §5)", i)
		expectedVer++
	}
}

// ---------------------------------------------------------------------------
// Deferred HIGH/MED fix tests (Wave 5 Part A)
// ---------------------------------------------------------------------------

// TestPatchAgentStatus_BreakReasonIgnoredOnNonBreak — Codex HIGH cross-field
// invariant. Sending break_reason_id with to=Ready must NOT persist the UUID
// in the DB row (it belongs exclusively to Break transitions).
func TestPatchAgentStatus_BreakReasonIgnoredOnNonBreak(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	brID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-brig", "BR Ignore Agent")
	seedBreakReason(t, th.Pool, th.OrgID, brID, "IgnoreMe", false)
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "NotReady",
		StateVersion: 1,
	})

	brUUID := api.UUIDv7(brID)
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{
		To:            api.AgentStatusReady,
		BreakReasonId: &brUUID,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode, "NotReady→Ready want 200, body=%s", raw)

	var state api.AgentState
	require.NoError(t, json.Unmarshal(raw, &state))
	require.Equal(t, api.AgentStatusReady, state.Status)
	require.Nil(t, state.BreakReasonId,
		"break_reason_id MUST be nil on non-Break transition (cross-field invariant)")
}

// TestPatchAgentStatus_PostInteractionStateClearedOnExitEngaged — Codex HIGH
// cross-field invariant. post_interaction_state MUST only be accepted while
// current status==Engaged. Sending it from NotReady must be ignored.
func TestPatchAgentStatus_PostInteractionStateClearedOnExitEngaged(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-pisce", "PIS Clear Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "NotReady",
		StateVersion: 1,
	})

	pisReady := api.PostInteractionStateReady
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{
		To:                   api.AgentStatusReady,
		PostInteractionState: &pisReady,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode, "NotReady→Ready want 200, body=%s", raw)

	var state api.AgentState
	require.NoError(t, json.Unmarshal(raw, &state))
	require.Equal(t, api.AgentStatusReady, state.Status)
	require.Nil(t, state.PostInteractionState,
		"post_interaction_state MUST be nil when not transitioning from Engaged (cross-field invariant)")
}

// TestPatchAgentStatus_InvalidEnum_422 — Codex MED enum validation. An
// unrecognized `to` value must return 422 invalid_value (not 500 via
// Postgres CHECK constraint).
func TestPatchAgentStatus_InvalidEnum_422(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-invalidenum", "Invalid Enum Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "NotReady",
		StateVersion: 1,
	})

	// Pass an invalid enum value via raw map (api.PatchAgentStatusRequest.To
	// is a typed enum so we use a map to bypass compile-time validation).
	resp, raw := httpPATCHStatus(t, th, agentID, map[string]any{"to": "InvalidStatus"})
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"unrecognized `to` value MUST return 422 (not 500 via CHECK), body=%s", raw)

	var body api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, api.ErrorCodeInvalidValue, body.Error)
}

// TestGetAgentStatus_SoftDeletedAgent_Returns404 — Gemini HIGH soft-delete.
func TestGetAgentStatus_SoftDeletedAgent_Returns404(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-softdel-get", "Soft Delete Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "NotReady",
		StateVersion: 1,
	})

	// Confirm 200 before soft-delete.
	resp, raw := httpGETStatus(t, th, agentID)
	require.Equal(t, http.StatusOK, resp.StatusCode, "pre-delete GET want 200, body=%s", raw)

	// Soft-delete the agent via direct DB UPDATE.
	_, err := th.Pool.Exec(ctx,
		"UPDATE agents SET enabled = FALSE, updated_at = NOW() WHERE id = $1 AND org_id = $2",
		agentID, th.OrgID,
	)
	require.NoError(t, err, "soft-delete UPDATE must succeed")

	// GET must now return 404.
	resp2, raw2 := httpGETStatus(t, th, agentID)
	require.Equal(t, http.StatusNotFound, resp2.StatusCode,
		"soft-deleted agent GET must return 404, body=%s", raw2)
}

// TestPatchAgentStatus_SoftDeletedAgent_Returns404 — Gemini HIGH soft-delete.
func TestPatchAgentStatus_SoftDeletedAgent_Returns404(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-softdel-patch", "Soft Delete PATCH Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "NotReady",
		StateVersion: 1,
	})

	// Soft-delete the agent.
	_, err := th.Pool.Exec(ctx,
		"UPDATE agents SET enabled = FALSE, updated_at = NOW() WHERE id = $1 AND org_id = $2",
		agentID, th.OrgID,
	)
	require.NoError(t, err)

	// PATCH must return 404.
	resp, raw := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{To: api.AgentStatusReady})
	require.Equal(t, http.StatusNotFound, resp.StatusCode,
		"soft-deleted agent PATCH must return 404, body=%s", raw)
}

// ---------------------------------------------------------------------------
// PATCH — cache invalidation (D-55 / D-56)
// ---------------------------------------------------------------------------

func TestPatchAgentStatus_CacheInvalidation(t *testing.T) {
	t.Parallel()
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-cacheinval", "Cache Inval Agent")
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:      agentID,
		OrgID:        th.OrgID,
		Status:       "NotReady",
		StateVersion: 1,
	})
	cacheKey := cache.Key(th.OrgID, "agent_state", agentID)

	// 1. GET to populate cache.
	resp, raw := httpGETStatus(t, th, agentID)
	require.Equal(t, http.StatusOK, resp.StatusCode, "initial GET want 200, body=%s", raw)
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key must exist after first GET")

	// 2. PATCH to mutate state; this must DEL the cache entry.
	resp2, raw2 := httpPATCHStatus(t, th, agentID, api.PatchAgentStatusRequest{To: api.AgentStatusReady})
	require.Equal(t, http.StatusOK, resp2.StatusCode, "PATCH want 200, body=%s", raw2)

	// cache.Del is synchronous (miniredis is in-process); key must be gone immediately.
	require.False(t, th.Miniredis.Exists(cacheKey), "cache key must be absent after PATCH (cache.Del fired)")

	// 3. Second GET returns the new state (cache miss → DB).
	resp3, raw3 := httpGETStatus(t, th, agentID)
	require.Equal(t, http.StatusOK, resp3.StatusCode, "second GET want 200, body=%s", raw3)
	var state api.AgentState
	require.NoError(t, json.Unmarshal(raw3, &state))
	require.Equal(t, api.AgentStatusReady, state.Status, "second GET must return new status Ready")
}
