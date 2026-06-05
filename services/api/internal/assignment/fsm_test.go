package assignment

import (
	"math/rand"
	"testing"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
)

// Table-driven: mirrors the state-transition table in the package doc, cell by
// cell — progress advances, skips, the duplicate/backward quarantines, terminals
// from every non-terminal state, and the post-terminal quarantine.
func TestDecideTable(t *testing.T) {
	cases := []struct {
		name       string
		cur        State
		ev         adapter.EventType
		wantNext   State
		wantAction Action
		wantQuar   bool
	}{
		{"pending→delivered", StatePending, adapter.EventDelivered, StateDelivered, ActionNone, false},
		{"pending→connecting (skip delivered)", StatePending, adapter.EventConnecting, StateConnecting, ActionNone, false},
		{"pending→established (skip both)", StatePending, adapter.EventEstablished, StateEstablished, ActionNone, false},
		{"delivered→established", StateDelivered, adapter.EventEstablished, StateEstablished, ActionNone, false},
		{"duplicate delivered", StateDelivered, adapter.EventDelivered, StateDelivered, ActionNone, true},
		{"backward established→connecting", StateEstablished, adapter.EventConnecting, StateEstablished, ActionNone, true},
		{"duplicate established", StateEstablished, adapter.EventEstablished, StateEstablished, ActionNone, true},

		{"complete from established", StateEstablished, adapter.EventCompleted, StateTerminal, ActionComplete, false},
		{"caller_abandoned while pending", StatePending, adapter.EventCallerLeft, StateTerminal, ActionTeardown, false},
		{"disconnect mid-call → reassign", StateEstablished, adapter.EventDisconnected, StateTerminal, ActionReassign, false},
		{"reject while delivered → reoffer", StateDelivered, adapter.EventRejected, StateTerminal, ActionReoffer, false},
		{"failed while connecting → teardown", StateConnecting, adapter.EventFailed, StateTerminal, ActionTeardown, false},
		{"cancelled → cancel_ack", StateEstablished, adapter.EventCancelled, StateTerminal, ActionCancelAck, false},

		{"post-terminal progress quarantined", StateTerminal, adapter.EventEstablished, StateTerminal, ActionNone, true},
		{"post-terminal second terminal quarantined", StateTerminal, adapter.EventCompleted, StateTerminal, ActionNone, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.cur, c.ev)
			if got.Next != c.wantNext || got.Action != c.wantAction || got.Quarantine != c.wantQuar {
				t.Fatalf("Decide(%s,%s) = {next:%s action:%s quar:%v} want {next:%s action:%s quar:%v}",
					c.cur, c.ev, got.Next, got.Action, got.Quarantine, c.wantNext, c.wantAction, c.wantQuar)
			}
		})
	}
}

var allEvents = []adapter.EventType{
	adapter.EventDelivered, adapter.EventConnecting, adapter.EventEstablished,
	adapter.EventCompleted, adapter.EventCancelled, adapter.EventFailed,
	adapter.EventRejected, adapter.EventDisconnected, adapter.EventCallerLeft,
}

// Property/fuzz: bombard the FSM with random event permutations and assert the
// invariants that protect against capacity leaks and stuck agents —
//   (1) progress is monotonic (never moves backward),
//   (2) exactly one terminal action is ever emitted (first-terminal-wins),
//   (3) after a terminal, the state is sticky and every event is quarantined,
//   (4) Decide is total (no panic) for every pair.
func TestDecideInvariantsFuzz(t *testing.T) {
	r := rand.New(rand.NewSource(0xA55159)) // fixed seed → reproducible
	for trial := 0; trial < 20000; trial++ {
		state := StatePending
		lastProgress := progressRank(StatePending)
		terminalCount := 0
		terminated := false
		n := r.Intn(12)
		for i := 0; i < n; i++ {
			ev := allEvents[r.Intn(len(allEvents))]
			d := Decide(state, ev)

			if terminated {
				if !d.Quarantine || d.Next != StateTerminal {
					t.Fatalf("post-terminal event %s not quarantined: %+v", ev, d)
				}
				continue
			}
			if d.Quarantine {
				if d.Next != state {
					t.Fatalf("quarantined event changed state %s→%s", state, d.Next)
				}
				continue
			}
			// Accepted event: either a terminal or a strict progress advance.
			if d.Next == StateTerminal {
				terminalCount++
				terminated = true
				if d.Action == ActionNone {
					t.Fatalf("terminal %s produced no action", ev)
				}
			} else {
				if progressRank(d.Next) <= lastProgress {
					t.Fatalf("non-monotonic progress %s→%s (event %s)", state, d.Next, ev)
				}
				lastProgress = progressRank(d.Next)
			}
			state = d.Next
		}
		if terminalCount > 1 {
			t.Fatalf("emitted %d terminals, want ≤1 (first-terminal-wins)", terminalCount)
		}
	}
}

// Every (state, event) pair yields a Decision (totality) and never advances a
// terminal back to a live state.
func TestDecideTotality(t *testing.T) {
	for _, s := range []State{StatePending, StateDelivered, StateConnecting, StateEstablished, StateTerminal} {
		for _, ev := range allEvents {
			d := Decide(s, ev)
			if s == StateTerminal && d.Next != StateTerminal {
				t.Fatalf("terminal state escaped via %s → %s", ev, d.Next)
			}
		}
	}
}
