package runtime

import (
	"context"
	"testing"
	"time"
)

// routingPlan: trigger -> route_queue(q1) -> match_skill(sales) -> reservation
//
//	reservation -accepted->     end(routed)
//	reservation -timeout->      fb -> end
//	reservation -no_candidate-> fb
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

func TestSimulate_PerNodeScriptedOutcome(t *testing.T) {
	// A per-node scripted outcome pins the reservation's port directly (no
	// candidates needed), bypassing the offer loop — the per-node sim control.
	for _, port := range []string{"accepted", "timeout", "no_candidate"} {
		tr, err := Simulate(context.Background(), DefaultRegistry(), routingPlan(t), SimInput{
			Snapshot:     &Snapshot{QueueCandidates: map[string][]Candidate{"q1": {}}}, // empty pool
			NodeOutcomes: map[string]string{"rsv": port},
			ClockStart:   time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("simulate: %v", err)
		}
		if got := portOf(tr, "rsv"); got != port {
			t.Fatalf("scripted node outcome %q → reservation port %q", port, got)
		}
	}
}

func TestSimulate_EmptyPoolToNoCandidateFallback(t *testing.T) {
	// match_skill on an empty pool must NOT fail terminally — it flows to the
	// reservation, which yields no_candidate -> fallback (review BLOCK).
	tr, err := Simulate(context.Background(), DefaultRegistry(), routingPlan(t), SimInput{
		Snapshot:   &Snapshot{QueueCandidates: map[string][]Candidate{"q1": {}}},
		ClockStart: time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if got := portOf(tr, "rsv"); got != "no_candidate" {
		t.Fatalf("reservation port = %q, want no_candidate (steps=%v)", got, stepIDs(tr))
	}
	took := false
	for _, s := range tr.Steps {
		if s.NodeID == "fb" {
			took = true
		}
	}
	if !took {
		t.Fatalf("expected fallback on empty pool: %v", stepIDs(tr))
	}
}

func TestSimulate_SetVarAndComputeFeedDownstream(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("sv", NodeSetVar, setVarConfig{Name: "tier", ValueExpr: "gold"}),
			node("cp", NodeCompute, computeConfig{Expr: "num.abs(-5)", Var: "score"}),
			node("iff", NodeIfElse, ifElseConfig{Expr: "tier == gold AND score > 3"}),
			node("y", NodeLog, logConfig{Message: "vip"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "sv"}, {From: "sv", To: "cp"}, {From: "cp", To: "iff"},
			{From: "iff", To: "y", Label: "true"}, {From: "iff", To: "end", Label: "false"}, {From: "y", To: "end"},
		},
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	tr, err := Simulate(context.Background(), DefaultRegistry(), plan, SimInput{
		ClockStart: time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	// set_var(tier=gold) + compute(score=abs(-5)=5) → if_else(tier==gold AND score>3) true.
	if got := portOf(tr, "iff"); got != "true" {
		t.Fatalf("if_else port = %q, want true (steps=%v)", got, stepIDs(tr))
	}
	took := false
	for _, s := range tr.Steps {
		if s.NodeID == "y" {
			took = true
		}
	}
	if !took {
		t.Fatalf("expected the true branch (log node) to run: %v", stepIDs(tr))
	}
}

func TestSetVarCompute_Validate(t *testing.T) {
	reg := DefaultRegistry()
	bad := &Graph{Nodes: []GraphNode{
		node("t", NodeTrigger, nil),
		node("sv", NodeSetVar, setVarConfig{Name: "", ValueExpr: "gold"}),       // empty name
		node("sv2", NodeSetVar, setVarConfig{Name: "x", ValueExpr: "num.abs("}), // bad expr
		node("cp", NodeCompute, computeConfig{Expr: "1 +", Var: "1bad"}),        // bad expr + bad var
		node("end", NodeEnd, nil),
	}, Edges: []GraphEdge{{From: "t", To: "sv"}, {From: "sv", To: "sv2"}, {From: "sv2", To: "cp"}, {From: "cp", To: "end"}}}
	issues, err := ValidateGraph(context.Background(), bad, reg, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	wantFields := map[string]bool{"name": false, "value_expr": false, "expr": false, "var": false}
	for _, is := range issues {
		if _, ok := wantFields[is.Field]; ok {
			wantFields[is.Field] = true
		}
	}
	for f, seen := range wantFields {
		if !seen {
			t.Errorf("expected a validation issue on field %q; issues=%+v", f, issues)
		}
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
