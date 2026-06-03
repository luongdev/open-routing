package runtime

import (
	"context"
	"testing"
	"time"
)

// routingPlan: trigger -> route_queue(q1) -> match_skill(sales) -> reservation
//   reservation -accepted->     end(routed)
//   reservation -timeout->      fb -> end
//   reservation -no_candidate-> fb
func routingPlan(t *testing.T) CompiledPlan {
	t.Helper()
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("rq", NodeRouteQueue, routeQueueConfig{Queue: "q1"}),
			node("ms", NodeMatchSkill, matchSkillConfig{Skill: "sales", MinProficiency: 1}),
			node("rsv", NodeReservation, reservationConfig{TimeoutSec: 5, MaxAttempts: 2}),
			node("fb", NodeFallback, fallbackConfig{Reason: "no agent"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "rq"},
			{From: "rq", To: "ms"},
			{From: "ms", To: "rsv"},
			{From: "rsv", To: "end", Label: "accepted"},
			{From: "rsv", To: "fb", Label: "timeout"},
			{From: "rsv", To: "fb", Label: "no_candidate"},
			{From: "fb", To: "end"},
		},
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return plan
}

func simSnapshot() *Snapshot {
	return &Snapshot{QueueCandidates: map[string][]Candidate{
		"q1": {
			{AgentID: "a", Proficiency: map[string]int{"sales": 3}},
			{AgentID: "b", Proficiency: map[string]int{"sales": 2}},
		},
	}}
}

func portOf(tr Trace, nodeID string) string {
	for _, s := range tr.Steps {
		if s.NodeID == nodeID {
			return s.Port
		}
	}
	return ""
}

func TestSimulate_ReservationAccepted(t *testing.T) {
	tr, err := Simulate(context.Background(), DefaultRegistry(), routingPlan(t), SimInput{
		Snapshot:         simSnapshot(),
		ScriptedOutcomes: []ReservationOutcome{ResvAccepted},
		ClockStart:       time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if got := portOf(tr, "rsv"); got != "accepted" {
		t.Fatalf("reservation port = %q, want accepted", got)
	}
	if tr.Outcome != "completed" {
		t.Fatalf("outcome = %q, want completed", tr.Outcome)
	}
	// reached end via the accepted edge, NOT the fallback
	for _, s := range tr.Steps {
		if s.NodeID == "fb" {
			t.Fatalf("took fallback on an accepted reservation: %v", stepIDs(tr))
		}
	}
}

func TestSimulate_ReservationTimeoutToFallback(t *testing.T) {
	start := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	tr, err := Simulate(context.Background(), DefaultRegistry(), routingPlan(t), SimInput{
		Snapshot:         simSnapshot(),
		ScriptedOutcomes: []ReservationOutcome{ResvTimeout, ResvTimeout}, // both agents time out
		ClockStart:       start,
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if got := portOf(tr, "rsv"); got != "timeout" {
		t.Fatalf("reservation port = %q, want timeout", got)
	}
	took := false
	for _, s := range tr.Steps {
		if s.NodeID == "fb" {
			took = true
		}
	}
	if !took {
		t.Fatalf("expected fallback after both offers timed out: %v", stepIDs(tr))
	}
}

func TestSimulate_Deterministic(t *testing.T) {
	in := SimInput{
		Snapshot:         simSnapshot(),
		ScriptedOutcomes: []ReservationOutcome{ResvRejected, ResvAccepted},
		ClockStart:       time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC),
	}
	a, _ := Simulate(context.Background(), DefaultRegistry(), routingPlan(t), in)
	b, _ := Simulate(context.Background(), DefaultRegistry(), routingPlan(t), in)
	if !eqStrings(stepIDs(a), stepIDs(b)) {
		t.Fatalf("non-deterministic step path: %v vs %v", stepIDs(a), stepIDs(b))
	}
	// rejected a -> accepted b => second agent accepted
	if portOf(a, "rsv") != "accepted" {
		t.Fatalf("expected accepted on 2nd offer, got %q", portOf(a, "rsv"))
	}
}
