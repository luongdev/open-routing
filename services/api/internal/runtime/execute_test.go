package runtime

import (
	"context"
	"testing"
	"time"
)

// branchGraph: trigger -> if_else("tier == gold") -true-> log -> end
//
//	-false-> end
func branchGraph(t *testing.T) CompiledPlan {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("c", NodeIfElse, ifElseConfig{Expr: "tier == gold"}),
			node("l", NodeLog, logConfig{Message: "vip"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "c"},
			{From: "c", To: "l", Label: "true"},
			{From: "c", To: "end", Label: "false"},
			{From: "l", To: "end"},
		},
	}
	if issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil); err != nil || len(issues) != 0 {
		t.Fatalf("branchGraph invalid: err=%v issues=%+v", err, issues)
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return plan
}

func stepIDs(tr Trace) []string {
	ids := make([]string, len(tr.Steps))
	for i, s := range tr.Steps {
		ids[i] = s.NodeID
	}
	return ids
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestExecutor_BranchTrue(t *testing.T) {
	ex := NewExecutor(DefaultRegistry())
	res, err := ex.Run(context.Background(), RealClock{}, branchGraph(t), map[string]any{"tier": "gold"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Trace.Outcome != "completed" {
		t.Fatalf("outcome = %q", res.Trace.Outcome)
	}
	if got := stepIDs(res.Trace); !eqStrings(got, []string{"t", "c", "l", "end"}) {
		t.Fatalf("path = %v, want [t c l end]", got)
	}
}

func TestExecutor_BranchFalse(t *testing.T) {
	ex := NewExecutor(DefaultRegistry())
	res, err := ex.Run(context.Background(), RealClock{}, branchGraph(t), map[string]any{"tier": "silver"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := stepIDs(res.Trace); !eqStrings(got, []string{"t", "c", "end"}) {
		t.Fatalf("path = %v, want [t c end]", got)
	}
}

func TestExecutor_DeterministicReplay(t *testing.T) {
	ex := NewExecutor(DefaultRegistry())
	plan := branchGraph(t)
	r1, _ := ex.Run(context.Background(), RealClock{}, plan, map[string]any{"tier": "gold"})
	r2, _ := ex.Run(context.Background(), RealClock{}, plan, map[string]any{"tier": "gold"})
	if !eqStrings(stepIDs(r1.Trace), stepIDs(r2.Trace)) {
		t.Fatal("same inputs produced different paths")
	}
}

func TestExecutor_SwitchCase(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("s", NodeSwitchCase, switchCaseConfig{Expr: "lang", Cases: []string{"es", "fr"}}),
			node("es", NodeLog, logConfig{Message: "es"}),
			node("def", NodeLog, logConfig{Message: "default"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "s"},
			{From: "s", To: "es", Label: "es"},
			{From: "s", To: "def", Label: "default"},
			{From: "es", To: "end"},
			{From: "def", To: "end"},
		},
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	ex := NewExecutor(DefaultRegistry())

	res, _ := ex.Run(context.Background(), RealClock{}, plan, map[string]any{"lang": "es"})
	if got := stepIDs(res.Trace); !eqStrings(got, []string{"t", "s", "es", "end"}) {
		t.Fatalf("es path = %v", got)
	}
	res, _ = ex.Run(context.Background(), RealClock{}, plan, map[string]any{"lang": "de"})
	if got := stepIDs(res.Trace); !eqStrings(got, []string{"t", "s", "def", "end"}) {
		t.Fatalf("default path = %v", got)
	}
}

func TestExecutor_WaitAutoResumeAdvancesClock(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("w", NodeWait, waitConfig{DurationMs: 5000}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "w"}, {From: "w", To: "end"}},
	}
	plan, _ := Compile(g, DefaultRegistry())
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewVirtualClock(start)
	ex := NewExecutor(DefaultRegistry(), WithAutoResume())
	res, err := ex.Run(context.Background(), clock, plan, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Trace.Outcome != "completed" {
		t.Fatalf("outcome = %q, want completed", res.Trace.Outcome)
	}
	if got := clock.Now().Sub(start); got != 5*time.Second {
		t.Fatalf("clock advanced %v, want 5s", got)
	}
}

func TestExecutor_WaitLiveSuspends(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("w", NodeWait, waitConfig{DurationMs: 5000}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "w"}, {From: "w", To: "end"}},
	}
	plan, _ := Compile(g, DefaultRegistry())
	ex := NewExecutor(DefaultRegistry()) // no auto-resume
	res, err := ex.Run(context.Background(), RealClock{}, plan, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Trace.Outcome != "suspended" || res.Suspension == nil {
		t.Fatalf("want suspended with suspension, got outcome=%q susp=%v", res.Trace.Outcome, res.Suspension)
	}
	last := res.Trace.Steps[len(res.Trace.Steps)-1]
	if last.NodeID != "w" || last.Status != "suspended" {
		t.Fatalf("last step = %+v, want w/suspended", last)
	}
}

func TestExecutor_CycleHitsMaxSteps(t *testing.T) {
	// A hand-built plan with a cycle (validation would reject it, but the
	// executor must not loop forever if a plan slips through).
	plan := CompiledPlan{
		FormatVersion: PlanFormatVersion,
		Entry:         "a",
		Steps: []PlanStep{
			{NodeID: "a", Kind: NodeLog, Compiled: []byte(`{}`)},
			{NodeID: "b", Kind: NodeLog, Compiled: []byte(`{}`)},
		},
		Edges: []CompiledEdge{{From: "a", To: "b"}, {From: "b", To: "a"}},
	}
	ex := NewExecutor(DefaultRegistry(), WithMaxSteps(10))
	res, err := ex.Run(context.Background(), RealClock{}, plan, nil)
	if err == nil {
		t.Fatal("expected max-steps error")
	}
	if res.Trace.Outcome != "failed" {
		t.Fatalf("outcome = %q, want failed", res.Trace.Outcome)
	}
}
