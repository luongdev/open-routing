package state

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

func TestTransitionMatrix_Exhaustive(t *testing.T) {
	allowed := map[[2]api.AgentStatus]TransitionRule{
		{api.AgentStatusNotReady, api.AgentStatusReady}: {AgentInitiated: true},
		{api.AgentStatusReady, api.AgentStatusNotReady}: {AgentInitiated: true},
		{api.AgentStatusReady, api.AgentStatusBreak}:    {RequiresBreakReasonID: true, AgentInitiated: true},
		{api.AgentStatusBreak, api.AgentStatusReady}:    {AgentInitiated: true},
		{api.AgentStatusBreak, api.AgentStatusNotReady}: {AgentInitiated: true},
		{api.AgentStatusWrapUp, api.AgentStatusReady}:   {AgentInitiated: true},
		{api.AgentStatusWrapUp, api.AgentStatusNotReady}: {AgentInitiated: true},
	}
	require.Len(t, allowed, 7, "matrix must encode exactly 7 agent-initiated edges (STATE-02)")

	statuses := allStatuses()
	require.Len(t, statuses, 6)

	for _, from := range statuses {
		for _, to := range statuses {
			rule, err := validateTransition(from, to)
			if wantRule, ok := allowed[[2]api.AgentStatus{from, to}]; ok {
				require.NoError(t, err, "expected matrix to allow %s→%s", from, to)
				require.Equal(t, wantRule, rule, "rule mismatch for %s→%s", from, to)
			} else {
				require.ErrorIs(t, err, ErrInvalidTransition, "expected matrix to reject %s→%s", from, to)
			}
		}
	}
}

func TestMatrix_SystemOnlyTransitionsAbsent(t *testing.T) {
	systemOnly := [][2]api.AgentStatus{
		{api.AgentStatusReady, api.AgentStatusEngaged},
		{api.AgentStatusEngaged, api.AgentStatusWrapUp},
		{api.AgentStatusReady, api.AgentStatusOffline},
		{api.AgentStatusBreak, api.AgentStatusOffline},
		{api.AgentStatusEngaged, api.AgentStatusOffline},
		{api.AgentStatusWrapUp, api.AgentStatusOffline},
		{api.AgentStatusOffline, api.AgentStatusNotReady},
	}
	for _, pair := range systemOnly {
		_, err := validateTransition(pair[0], pair[1])
		require.True(t, errors.Is(err, ErrInvalidTransition),
			"system-only %s→%s must NOT be in v0.1 matrix (D-82)", pair[0], pair[1])
	}
}

func TestMatrix_EngagedDeferred(t *testing.T) {
	// STATE-05 schema readiness: Ready→Engaged is system-only (v0.2).
	_, err := validateTransition(api.AgentStatusReady, api.AgentStatusEngaged)
	require.ErrorIs(t, err, ErrInvalidTransition)
}
