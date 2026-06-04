package runtime

import "time"

// Snapshot is the point-in-time catalog+state read-set a run executes against.
// Built ONCE per run (under a REPEATABLE READ tx for live; supplied directly for
// simulation), so the run is deterministic and replayable. All candidate slices
// are pre-sorted by RankCandidates' tie-break so iteration order is stable.
type Snapshot struct {
	// QueueCandidates maps a queue code to its candidate pool. route_queue sets
	// the working pool from this; match_skill/filter then narrow it.
	QueueCandidates map[string][]Candidate `json:"queue_candidates"`
}

// ReservationOutcome is the result of a single offer. The reservation node loops
// offers and maps the terminal outcome to an output port.
type ReservationOutcome string

const (
	ResvAccepted ReservationOutcome = "accepted"
	ResvRejected ReservationOutcome = "rejected"
	ResvTimeout  ReservationOutcome = "timeout"
)

// RoutingDriver resolves a reservation offer. It owns the clock so a `timeout`
// outcome advances virtual time by the offer timeout (the executor must NOT
// double-advance — see WithAutoResume, which advances `wait` only). The sim
// driver consumes scripted outcomes deterministically; the live driver (3c)
// writes a real reservation and parks a continuation.
type RoutingDriver interface {
	Reserve(clock Clock, agentID string, timeout time.Duration) ReservationOutcome
}

// Offerer makes a single LIVE reservation offer for a route run, writing the
// reservation row (flowrt) and returning its id. ok=false means the agent could
// not be offered (busy / ineligible — e.g. a 23505 on the agent-active partial
// unique index) so the reservation node skips to the next candidate. Unlike the
// sim RoutingDriver, the offer does NOT resolve synchronously: the reservation
// node suspends after a successful offer and resumes on the agent's action.
type Offerer interface {
	Offer(agentID string, timeout time.Duration) (reservationID string, ok bool, err error)
}

// scriptedDriver drives reservations from a pre-authored list, consumed IN
// ORDER (agent_id on a scripted entry is informational in v0.2). An exhausted
// script yields `rejected` so the reservation node keeps offering until the pool
// or max_attempts runs out (→ no_candidate). A `timeout` advances the clock.
type scriptedDriver struct {
	outcomes []ReservationOutcome
	i        int
}

// NewScriptedDriver builds a sim reservation driver from ordered outcomes.
func NewScriptedDriver(outcomes []ReservationOutcome) *scriptedDriver {
	return &scriptedDriver{outcomes: outcomes}
}

func (d *scriptedDriver) Reserve(clock Clock, _ string, timeout time.Duration) ReservationOutcome {
	oc := ResvRejected
	if d.i < len(d.outcomes) {
		oc = d.outcomes[d.i]
		d.i++
	}
	if oc == ResvTimeout {
		clock.Advance(timeout) // driver-owned advance; the executor does not.
	}
	return oc
}
