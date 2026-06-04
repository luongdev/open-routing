package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// fakeOfferer records offers; agents in `busy` can't be offered (ok=false), so
// the reservation node skips to the next candidate.
type fakeOfferer struct {
	busy    map[string]bool
	offered []string
	n       int
}

func (f *fakeOfferer) Offer(agentID string, _ time.Duration) (string, bool, error) {
	if f.busy[agentID] {
		return "", false, nil
	}
	f.n++
	f.offered = append(f.offered, agentID)
	return fmt.Sprintf("res-%d", f.n), true, nil
}

func liveExec(t *testing.T, off Offerer) *Executor {
	t.Helper()
	return NewExecutor(DefaultRegistry(), WithRouting(simSnapshot(), nil), WithOfferer(off))
}

var liveT0 = time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)

// A fresh live run offers the top-ranked candidate and SUSPENDS at the
// reservation node (it does NOT resolve synchronously like sim).
func TestLive_OffersTopAndSuspends(t *testing.T) {
	off := &fakeOfferer{}
	res, err := liveExec(t, off).Run(context.Background(), NewVirtualClock(liveT0), routingPlan(t), nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Suspension == nil || res.Trace.Outcome != "suspended" {
		t.Fatalf("expected suspended, got outcome=%q susp=%v", res.Trace.Outcome, res.Suspension)
	}
	if res.SuspendedNodeID != "rsv" {
		t.Fatalf("suspended node = %q, want rsv", res.SuspendedNodeID)
	}
	if len(off.offered) != 1 || off.offered[0] != "a" {
		t.Fatalf("offered = %v, want [a] (top-ranked)", off.offered)
	}
}

// Resuming with `accepted` takes the accepted port and runs to completion.
func TestLive_ResumeAcceptedCompletes(t *testing.T) {
	off := &fakeOfferer{}
	ex := liveExec(t, off)
	first, _ := ex.Run(context.Background(), NewVirtualClock(liveT0), routingPlan(t), nil)
	cur := ResumeCursor{Version: 1, NodeID: first.SuspendedNodeID, Vars: first.Vars}
	res, err := ex.RunFrom(context.Background(), NewVirtualClock(liveT0), routingPlan(t), cur, "accepted", nil)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.Trace.Outcome != "completed" {
		t.Fatalf("outcome = %q, want completed", res.Trace.Outcome)
	}
	if portOf(res.Trace, "rsv") != "accepted" {
		t.Fatalf("rsv port = %q, want accepted", portOf(res.Trace, "rsv"))
	}
	for _, s := range res.Trace.Steps {
		if s.NodeID == "fb" {
			t.Fatalf("took fallback on an accepted resume: %v", stepIDs(res.Trace))
		}
	}
}

// Resuming with `rejected` offers the NEXT freshly-read candidate and suspends
// again (flowrt supplies the pool excluding the already-offered agent).
func TestLive_ResumeRejectedOffersNext(t *testing.T) {
	off := &fakeOfferer{}
	ex := liveExec(t, off)
	first, _ := ex.Run(context.Background(), NewVirtualClock(liveT0), routingPlan(t), nil) // offered "a"
	cur := ResumeCursor{Version: 1, NodeID: first.SuspendedNodeID, Vars: first.Vars}
	// flowrt re-read excluding "a" → only "b" remains
	pool := []Candidate{{AgentID: "b", Proficiency: map[string]int{"sales": 2}}}
	res, err := ex.RunFrom(context.Background(), NewVirtualClock(liveT0), routingPlan(t), cur, "rejected", pool)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.Suspension == nil || res.Trace.Outcome != "suspended" {
		t.Fatalf("expected re-suspend on next offer, got %q", res.Trace.Outcome)
	}
	if len(off.offered) != 2 || off.offered[1] != "b" {
		t.Fatalf("offered = %v, want [a b]", off.offered)
	}
}

// Resuming with `timeout` and an exhausted pool yields the timeout port.
func TestLive_ResumeTimeoutExhaustedToTimeoutPort(t *testing.T) {
	off := &fakeOfferer{}
	ex := liveExec(t, off)
	first, _ := ex.Run(context.Background(), NewVirtualClock(liveT0), routingPlan(t), nil)
	cur := ResumeCursor{Version: 1, NodeID: first.SuspendedNodeID, Vars: first.Vars}
	res, err := ex.RunFrom(context.Background(), NewVirtualClock(liveT0), routingPlan(t), cur, "timeout", nil) // no candidates left
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if portOf(res.Trace, "rsv") != "timeout" {
		t.Fatalf("rsv port = %q, want timeout", portOf(res.Trace, "rsv"))
	}
	took := false
	for _, s := range res.Trace.Steps {
		if s.NodeID == "fb" {
			took = true
		}
	}
	if !took {
		t.Fatalf("expected fallback after timeout: %v", stepIDs(res.Trace))
	}
}

// A busy top candidate (offer ok=false) is skipped; the next is offered.
func TestLive_SkipsBusyCandidate(t *testing.T) {
	off := &fakeOfferer{busy: map[string]bool{"a": true}}
	res, err := liveExec(t, off).Run(context.Background(), NewVirtualClock(liveT0), routingPlan(t), nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Suspension == nil {
		t.Fatalf("expected suspend after skipping busy 'a'")
	}
	if len(off.offered) != 1 || off.offered[0] != "b" {
		t.Fatalf("offered = %v, want [b] (a busy, skipped)", off.offered)
	}
}
