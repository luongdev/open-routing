// Package domain holds pure functions with no I/O, no DB, no cache, no
// network. Consumers (state.Server, future v0.2 routing engine) load
// required scalars (e.g., break_reasons.routable) before calling.
package domain

import "github.com/luongdev/open-routing/services/api/internal/api"

type AgentStateInputs struct {
	Status              string
	BreakReasonRoutable bool
}

// IsRoutable returns true iff the agent is currently eligible to take
// new interactions: Ready (always) or Break+routable=true (STATE-10 +
// ROADMAP Phase 4 success criterion #4). String comparison against
// api.AgentStatus* constants avoids type coupling so callers can pass
// raw DB-row strings without conversion.
func IsRoutable(in AgentStateInputs) bool {
	if in.Status == string(api.AgentStatusReady) {
		return true
	}
	if in.Status == string(api.AgentStatusBreak) && in.BreakReasonRoutable {
		return true
	}
	return false
}
