// Package assignment is the v0.4 Wave 0 engine-side assignment state machine: the
// single authority that decides, for one assignment (one delivery_attempt /
// media_session), how an incoming adapter lifecycle event transitions the
// assignment and what the engine must do — and which events to QUARANTINE.
//
// It exists because real, out-of-process media makes lifecycle events arrive
// late, duplicated, out of order, skipped (only a terminal, no `established`), or
// after a terminal (a stale room's webhook landing during a new reassignment
// hop). v0.3's reservation+correlation fences cannot absorb that; this FSM can,
// deterministically and identity-free, so it is exhaustively table- and
// property-testable before any DB, webhook, or media code is written
// (cross-AI round-1 #1 risk: lifecycle identity confusion / non-monotonic events).
//
// State-transition table (cur × event → decision):
//
//	cur \ event   delivered    connecting   established   <terminal>     (post-terminal)
//	pending       →delivered   →connecting  →established  →terminal A     —
//	delivered     QUAR dup     →connecting  →established  →terminal A     —
//	connecting    QUAR stale   QUAR dup     →established  →terminal A     —
//	established   QUAR stale   QUAR stale   QUAR dup      →terminal A     —
//	terminal      QUAR post    QUAR post    QUAR post     QUAR post       QUAR post
//
// Progress is monotonic: an event whose progress rank is ≤ the current state is a
// duplicate or a backward (stale/out-of-order) event and is QUARANTINED. A skipped
// progress event simply advances (pending → established is fine — "missed events").
// A terminal is accepted from ANY non-terminal state; once terminal, EVERY further
// event (including another terminal) is quarantined — first-terminal-wins.
package assignment

import "github.com/luongdev/open-routing/services/api/internal/adapter"

// State is the lifecycle state of one assignment.
type State string

const (
	StatePending     State = "pending"     // delivery intent created; nothing delivered yet
	StateDelivered   State = "delivered"   // adapter began delivering to the agent
	StateConnecting  State = "connecting"  // bridging agent <-> interaction
	StateEstablished State = "established" // both legs connected; handling started
	StateTerminal    State = "terminal"    // ended (reason in Decision.Reason)
)

// Action is what the engine must do as a result of an accepted event. Progress
// events carry ActionNone (just record/notify); each terminal maps to one action.
type Action string

const (
	ActionNone      Action = ""           // zero value: accepted progress advance / quarantine — record only
	ActionComplete  Action = "complete"   // normal end → reservation complete + WrapUp
	ActionReassign  Action = "reassign"   // agent dropped mid-call → re-queue to another agent
	ActionReoffer   Action = "reoffer"    // agent device declined → re-offer per matcher
	ActionTeardown  Action = "teardown"   // caller gone / adapter fault → tear the route down
	ActionCancelAck Action = "cancel_ack" // engine-initiated cancel; just observe the terminal
)

// Decision is the FSM output for one (state, event) pair.
type Decision struct {
	Next       State  // the resulting state (== cur when quarantined)
	Action     Action // engine action (ActionNone for progress / quarantine)
	Quarantine bool   // true ⇒ ignore this event (late / duplicate / out-of-order / post-terminal)
	Reason     string // terminal cause, or the quarantine reason
}

// progressRank orders the non-terminal lifecycle; terminal/unknown are not ranked.
func progressRank(s State) int {
	switch s {
	case StatePending:
		return 0
	case StateDelivered:
		return 1
	case StateConnecting:
		return 2
	case StateEstablished:
		return 3
	}
	return -1
}

// eventProgress maps a progress event to its rank + the state it advances to. A
// non-progress (terminal/unknown) event returns rank 0.
func eventProgress(e adapter.EventType) (rank int, to State) {
	switch e {
	case adapter.EventDelivered:
		return 1, StateDelivered
	case adapter.EventConnecting:
		return 2, StateConnecting
	case adapter.EventEstablished:
		return 3, StateEstablished
	}
	return 0, ""
}

// terminalAction maps a terminal event to the engine action it drives.
func terminalAction(e adapter.EventType) Action {
	switch e {
	case adapter.EventCompleted:
		return ActionComplete
	case adapter.EventCancelled:
		return ActionCancelAck
	case adapter.EventRejected:
		return ActionReoffer
	case adapter.EventDisconnected:
		return ActionReassign
	case adapter.EventFailed, adapter.EventCallerLeft:
		return ActionTeardown
	}
	return ActionTeardown
}

// Decide returns the transition for receiving ev while in cur. It is pure and
// total — every (state, event) pair yields a Decision, never a panic.
func Decide(cur State, ev adapter.EventType) Decision {
	// First-terminal-wins: once terminal, nothing else applies.
	if cur == StateTerminal {
		return Decision{Next: StateTerminal, Quarantine: true, Reason: "post_terminal"}
	}
	if ev.Terminal() {
		return Decision{Next: StateTerminal, Action: terminalAction(ev), Reason: string(ev)}
	}
	rank, to := eventProgress(ev)
	if rank == 0 {
		return Decision{Next: cur, Quarantine: true, Reason: "unknown_event"}
	}
	// Monotonic progress: a rank at or below the current state is a duplicate or a
	// backward (stale / out-of-order) event.
	if rank <= progressRank(cur) {
		return Decision{Next: cur, Quarantine: true, Reason: "stale_or_duplicate_progress"}
	}
	return Decision{Next: to, Action: ActionNone}
}
