// Package state holds the agent state-machine implementation
// (STATE-01..STATE-10).
//
// Pitfall 1: the exported handler type is `Server` (NOT `Handlers`)
// because *catalog.Handlers + *state.Handlers fail to compile when
// anonymous-embedded into the Wave 4 ApiHandlers composite (Go rejects
// "duplicate field name").
package state

import (
	"errors"
	"fmt"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// AgentStatus aliases the wire-level enum so transitions.go is the
// single source of truth for the matrix without forking a parallel type.
type AgentStatus = api.AgentStatus

// TransitionRule captures per-edge requirements (STATE-04, STATE-05).
type TransitionRule struct {
	RequiresBreakReasonID  bool
	RequiresEngagedChannel bool
	AgentInitiated         bool
}

// matrix encodes the 7 agent-initiated edges allowed by PATCH /status
// (STATE-02). System-initiated edges deferred per D-82:
//   - Ready→Engaged + Engaged→WrapUp  → v0.2 runtime engine
//   - *→Offline + Offline→NotReady    → AUTH phase login/logout events
var matrix = map[AgentStatus]map[AgentStatus]TransitionRule{
	api.AgentStatusNotReady: {
		api.AgentStatusReady: {AgentInitiated: true},
	},
	api.AgentStatusReady: {
		api.AgentStatusNotReady: {AgentInitiated: true},
		api.AgentStatusBreak:    {RequiresBreakReasonID: true, AgentInitiated: true},
	},
	api.AgentStatusBreak: {
		api.AgentStatusReady:    {AgentInitiated: true},
		api.AgentStatusNotReady: {AgentInitiated: true},
	},
	api.AgentStatusWrapUp: {
		api.AgentStatusReady:    {AgentInitiated: true},
		api.AgentStatusNotReady: {AgentInitiated: true},
	},
}

// ErrInvalidTransition is the sentinel wrapped by validateTransition for
// any pair not in the matrix. Callers translate to
// PatchAgentStatus409JSONResponse(InvalidTransitionErrorResponse{...})
// via errors.Is.
var ErrInvalidTransition = errors.New("state: invalid transition")

// validateTransition returns the per-edge rule when (from, to) is in
// the matrix, ErrInvalidTransition wrapped with from/to otherwise.
// D-84 force=true bypasses this validator — caller branches around it.
func validateTransition(from, to AgentStatus) (TransitionRule, error) {
	next, ok := matrix[from]
	if !ok {
		return TransitionRule{}, fmt.Errorf("%w: from=%s to=%s", ErrInvalidTransition, from, to)
	}
	rule, ok := next[to]
	if !ok {
		return TransitionRule{}, fmt.Errorf("%w: from=%s to=%s", ErrInvalidTransition, from, to)
	}
	return rule, nil
}

// allStatuses returns every defined AgentStatus for the exhaustive test
// walk. Package-internal — production code never iterates the whole enum.
func allStatuses() []AgentStatus {
	return []AgentStatus{
		api.AgentStatusNotReady,
		api.AgentStatusReady,
		api.AgentStatusBreak,
		api.AgentStatusEngaged,
		api.AgentStatusWrapUp,
		api.AgentStatusOffline,
	}
}
