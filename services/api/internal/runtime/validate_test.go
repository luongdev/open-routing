package runtime

import (
	"context"
	"encoding/json"
	"testing"
)

// fakeRefs is a structure-only CatalogRefs for validation tests: a code exists
// iff it is in the corresponding set.
type fakeRefs struct {
	queues, skills, channels, adapters, breaks map[string]bool
}

func (f fakeRefs) has(m map[string]bool, c string) bool { return m[c] }
func (f fakeRefs) HasQueue(c string) bool               { return f.has(f.queues, c) }
func (f fakeRefs) HasSkill(c string) bool               { return f.has(f.skills, c) }
func (f fakeRefs) HasChannel(c string) bool             { return f.has(f.channels, c) }
func (f fakeRefs) HasAdapter(c string) bool             { return f.has(f.adapters, c) }
func (f fakeRefs) HasBreakReason(c string) bool         { return f.has(f.breaks, c) }

func node(id string, kind NodeKind, cfg any) GraphNode {
	var raw json.RawMessage
	if cfg != nil {
		b, _ := json.Marshal(cfg)
		raw = b
	}
	return GraphNode{ID: id, Kind: kind, Config: raw}
}

func codes(issues []ValidationIssue) map[string]int {
	m := map[string]int{}
	for _, i := range issues {
		m[i.Code]++
	}
	return m
}

func validGraph() *Graph {
	return &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("q", NodeRouteQueue, routeQueueConfig{Queue: "queue_vip"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{ID: "e1", From: "t", To: "q"},
			{ID: "e2", From: "q", To: "end"},
		},
	}
}

func TestValidateGraph_HappyPath(t *testing.T) {
	refs := fakeRefs{queues: map[string]bool{"queue_vip": true}}
	issues, err := ValidateGraph(context.Background(), validGraph(), DefaultRegistry(), refs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
}

func TestValidateGraph_EmptyGraph(t *testing.T) {
	issues, _ := ValidateGraph(context.Background(), &Graph{}, DefaultRegistry(), nil)
	if codes(issues)[IssueEmptyGraph] != 1 {
		t.Fatalf("want empty_graph, got %+v", issues)
	}
}

func TestValidateGraph_StructuralProblems(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("a", NodeIfElse, ifElseConfig{Expr: "x > 1"}), // no trigger, no end
			node("a", NodeLog, logConfig{}),                    // duplicate id
			node("orphan", NodeFilter, filterConfig{Expr: "true"}),
		},
		Edges: []GraphEdge{
			{ID: "bad", From: "a", To: "ghost"}, // dangling target
		},
	}
	issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := codes(issues)
	for _, want := range []string{IssueNoTrigger, IssueNoTerminal, IssueDuplicateNodeID, IssueDanglingEdge} {
		if c[want] == 0 {
			t.Errorf("expected issue %q, got %+v", want, issues)
		}
	}
	// Reachability is skipped when edges dangle, so no unreachable_node noise here.
	if c[IssueUnreachableNode] != 0 {
		t.Errorf("did not expect unreachable_node while edges dangle: %+v", issues)
	}
}

func TestValidateGraph_UnreachableNode(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("end", NodeEnd, nil),
			node("island", NodeLog, logConfig{}),
		},
		Edges: []GraphEdge{{ID: "e1", From: "t", To: "end"}},
	}
	issues, _ := ValidateGraph(context.Background(), g, DefaultRegistry(), nil)
	c := codes(issues)
	if c[IssueUnreachableNode] != 1 {
		t.Fatalf("want one unreachable_node, got %+v", issues)
	}
}

func TestValidateGraph_MultipleTriggers(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{node("t1", NodeTrigger, nil), node("t2", NodeTrigger, nil), node("e", NodeEnd, nil)},
		Edges: []GraphEdge{{From: "t1", To: "e"}, {From: "t2", To: "e"}},
	}
	issues, _ := ValidateGraph(context.Background(), g, DefaultRegistry(), nil)
	if codes(issues)[IssueMultipleTriggers] != 1 {
		t.Fatalf("want multiple_triggers, got %+v", issues)
	}
}

func TestValidateGraph_MissingCatalogReference(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("q", NodeRouteQueue, routeQueueConfig{Queue: "ghost_queue"}),
			node("s", NodeMatchSkill, matchSkillConfig{Skill: "ghost_skill"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "q"}, {From: "q", To: "s"}, {From: "s", To: "end"}},
	}
	// Empty refs: nothing exists.
	issues, _ := ValidateGraph(context.Background(), g, DefaultRegistry(), fakeRefs{})
	if codes(issues)[IssueMissingCatalog] != 2 {
		t.Fatalf("want two missing_catalog_reference, got %+v", issues)
	}
}

func TestValidateGraph_NilRefsSkipsReferenceChecks(t *testing.T) {
	// Same graph, nil refs → reference checks skipped, so it validates clean.
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("q", NodeRouteQueue, routeQueueConfig{Queue: "anything"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "q"}, {From: "q", To: "end"}},
	}
	issues, _ := ValidateGraph(context.Background(), g, DefaultRegistry(), nil)
	if len(issues) != 0 {
		t.Fatalf("nil refs should skip reference checks, got %+v", issues)
	}
}

func TestValidateGraph_FieldErrors(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("w", NodeWait, waitConfig{DurationMs: 0}),               // invalid
			node("i", NodeIfElse, ifElseConfig{Expr: ""}),                // missing
			node("r", NodeReservation, reservationConfig{TimeoutSec: 0}), // invalid
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "w"}, {From: "w", To: "i"}, {From: "i", To: "r"}, {From: "r", To: "end"}},
	}
	issues, _ := ValidateGraph(context.Background(), g, DefaultRegistry(), nil)
	c := codes(issues)
	if c[IssueInvalidConfig] < 2 || c[IssueMissingField] < 1 {
		t.Fatalf("want invalid_config x2 + missing_required_field, got %+v", issues)
	}
}

func TestValidateGraph_MalformedConfig(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			{ID: "bad", Kind: NodeWait, Config: json.RawMessage(`{"duration_ms": "not a number"}`)},
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "bad"}, {From: "bad", To: "end"}},
	}
	issues, _ := ValidateGraph(context.Background(), g, DefaultRegistry(), nil)
	if codes(issues)[IssueMalformedConfig] != 1 {
		t.Fatalf("want malformed_config, got %+v", issues)
	}
}

func TestCompile_ProducesDeterministicPlan(t *testing.T) {
	reg := DefaultRegistry()
	g := validGraph()
	plan, err := Compile(g, reg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if plan.FormatVersion != PlanFormatVersion {
		t.Errorf("format version = %d, want %d", plan.FormatVersion, PlanFormatVersion)
	}
	if plan.Entry != "t" {
		t.Errorf("entry = %q, want t", plan.Entry)
	}
	if len(plan.Steps) != 3 || len(plan.Edges) != 2 {
		t.Fatalf("steps=%d edges=%d, want 3/2", len(plan.Steps), len(plan.Edges))
	}
	// Compile is deterministic: same input, byte-identical plan.
	b1, _ := json.Marshal(plan)
	plan2, _ := Compile(g, reg)
	b2, _ := json.Marshal(plan2)
	if string(b1) != string(b2) {
		t.Fatal("compile is not deterministic")
	}
}

func TestValidateAndCompile_InvalidGraphYieldsIssuesNotPlan(t *testing.T) {
	g := &Graph{Nodes: []GraphNode{node("q", NodeRouteQueue, routeQueueConfig{Queue: "x"})}} // no trigger/end
	issues, plan, err := ValidateAndCompile(context.Background(), g, DefaultRegistry(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan != nil {
		t.Fatal("expected no plan for invalid graph")
	}
	if len(issues) == 0 {
		t.Fatal("expected issues for invalid graph")
	}
}

func codesOf(issues []ValidationIssue) map[string]bool {
	m := map[string]bool{}
	for _, i := range issues {
		m[i.Code] = true
	}
	return m
}

func TestValidate_InvalidNodeID(t *testing.T) {
	g := &Graph{Nodes: []GraphNode{node("", NodeTrigger, nil), node("a#b", NodeEnd, nil)}}
	issues, _ := ValidateGraph(context.Background(), g, DefaultRegistry(), nil)
	if !codesOf(issues)[IssueInvalidNodeID] {
		t.Fatalf("want invalid_node_id, got %+v", issues)
	}
}

func TestValidate_SuspendInRegion(t *testing.T) {
	// A reservation inside a loop body region is rejected (review B3); a wait there
	// is allowed (sim clock-join feature).
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("lp", NodeLoopFor, loopForConfig{ArrayExpr: "items"}),
			func() GraphNode { n := node("r", NodeReservation, reservationConfig{TimeoutSec: 5}); n.Region = "lp"; return n }(),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "lp"}, {From: "lp", To: "r", Label: "body"}, {From: "lp", To: "end", Label: "done"}},
	}
	if !codesOf(mustIssues(t, g))[IssueSuspendInRegion] {
		t.Fatalf("want suspend_in_region for reservation in region")
	}
}

func TestValidate_PortCardinality(t *testing.T) {
	// if_else missing the "false" port → invalid_port_cardinality.
	g := &Graph{
		Nodes: []GraphNode{node("t", NodeTrigger, nil), node("c", NodeIfElse, ifElseConfig{Expr: "x"}), node("end", NodeEnd, nil)},
		Edges: []GraphEdge{{From: "t", To: "c"}, {From: "c", To: "end", Label: "true"}},
	}
	if !codesOf(mustIssues(t, g))[IssuePortCardinality] {
		t.Fatalf("want invalid_port_cardinality for if_else missing 'false'")
	}
	// duplicate label.
	g2 := &Graph{
		Nodes: []GraphNode{node("t", NodeTrigger, nil), node("c", NodeIfElse, ifElseConfig{Expr: "x"}), node("e1", NodeEnd, nil), node("e2", NodeEnd, nil)},
		Edges: []GraphEdge{{From: "t", To: "c"}, {From: "c", To: "e1", Label: "true"}, {From: "c", To: "e2", Label: "true"}, {From: "c", To: "e1", Label: "false"}},
	}
	if !codesOf(mustIssues(t, g2))[IssuePortCardinality] {
		t.Fatalf("want invalid_port_cardinality for duplicate 'true'")
	}
}

func mustIssues(t *testing.T, g *Graph) []ValidationIssue {
	t.Helper()
	issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil)
	if err != nil {
		t.Fatalf("validate err: %v", err)
	}
	return issues
}

func TestValidate_SwitchCasePorts(t *testing.T) {
	// switch_case must wire every declared case + default; a missing case port is
	// invalid_port_cardinality (re-review BLOCK).
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("s", NodeSwitchCase, switchCaseConfig{Expr: "tier", Cases: []string{"gold", "silver"}}),
			node("eg", NodeEnd, nil), node("ed", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "s"},
			{From: "s", To: "eg", Label: "gold"},
			{From: "s", To: "ed", Label: "default"},
			// "silver" port intentionally unwired.
		},
	}
	if !codesOf(mustIssues(t, g))[IssuePortCardinality] {
		t.Fatalf("want invalid_port_cardinality for switch_case missing 'silver'")
	}
}
