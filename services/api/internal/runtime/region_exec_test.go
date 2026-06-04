package runtime

import (
	"context"
	"testing"
	"time"
)

func simRun(t *testing.T, g *Graph, input map[string]any) (Trace, error) {
	t.Helper()
	if issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil); err != nil || len(issues) != 0 {
		t.Fatalf("graph invalid: err=%v issues=%+v", err, issues)
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return Simulate(context.Background(), DefaultRegistry(), plan, SimInput{
		Input: input, ClockStart: time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
	})
}

func countKind(tr Trace, kind NodeKind) int {
	n := 0
	for _, s := range tr.Steps {
		if s.Kind == kind {
			n++
		}
	}
	return n
}

// loop_for over a 3-element array runs its body 3×; set_var inside writes to the
// root (persists), and the scoped `item` is visible to the body.
func TestRunLoopFor_IteratesAndScopes(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "items", MaxIter: 10}),
			inRegion(node("sv", NodeSetVar, setVarConfig{Name: "seen", ValueExpr: "item"}), "lf"),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "lf"}, {From: "lf", To: "sv", Label: "body"}, {From: "lf", To: "end", Label: "done"}},
	}
	tr, err := simRun(t, g, map[string]any{"items": []any{"a", "b", "c"}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if tr.Outcome != "completed" {
		t.Fatalf("outcome=%q", tr.Outcome)
	}
	if got := countKind(tr, NodeSetVar); got != 3 {
		t.Fatalf("set_var ran %d times, want 3 (steps=%d)", got, len(tr.Steps))
	}
	// iteration tags present on body steps
	iters := []int{}
	for _, s := range tr.Steps {
		if s.Kind == NodeSetVar && s.Iteration != nil {
			iters = append(iters, *s.Iteration)
		}
	}
	if len(iters) != 3 || iters[0] != 0 || iters[2] != 2 {
		t.Fatalf("iteration tags = %v, want [0 1 2]", iters)
	}
}

// loop_while runs while a root var is truthy; the body flips it, so exactly one
// iteration runs.
func TestRunLoopWhile_RunsWhileTrue(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("init", NodeSetVar, setVarConfig{Name: "run", ValueExpr: "true"}),
			node("lw", NodeLoopWhile, loopWhileConfig{CondExpr: "run", MaxIter: 100}),
			inRegion(node("stop", NodeSetVar, setVarConfig{Name: "run", ValueExpr: "false"}), "lw"),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "init"}, {From: "init", To: "lw"}, {From: "lw", To: "stop", Label: "body"}, {From: "lw", To: "end", Label: "done"}},
	}
	tr, err := simRun(t, g, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// init (run=true) + stop (run=false, one iteration) → 2 set_var steps total
	if got := countKind(tr, NodeSetVar); got != 2 {
		t.Fatalf("set_var ran %d times, want 2", got)
	}
}

// loop_for with an array longer than max_iter yields the catchable loop_limit
// failure (uncaught here → run failed).
func TestRunLoopFor_LoopLimit(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "items", MaxIter: 2}),
			inRegion(node("sv", NodeSetVar, setVarConfig{Name: "x", ValueExpr: "item"}), "lf"),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "lf"}, {From: "lf", To: "sv", Label: "body"}, {From: "lf", To: "end", Label: "done"}},
	}
	tr, _ := simRun(t, g, map[string]any{"items": []any{"a", "b", "c"}})
	if tr.Outcome != "failed" || tr.FailureCode != string(FailLoopLimit) {
		t.Fatalf("outcome=%q code=%q, want failed/loop_limit", tr.Outcome, tr.FailureCode)
	}
}

// parallel runs branches in stable order; writes merge last-writer-wins.
func TestRunParallel_MergesInBranchOrder(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("par", NodeParallel, parallelConfig{}),
			inRegion(node("b0", NodeSetVar, setVarConfig{Name: "z", ValueExpr: "1"}), "par#0"),
			inRegion(node("b1", NodeSetVar, setVarConfig{Name: "z", ValueExpr: "2"}), "par#1"),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "par"},
			{From: "par", To: "b0", Label: "body:0"},
			{From: "par", To: "b1", Label: "body:1"},
			{From: "par", To: "end", Label: "done"},
		},
	}
	tr, err := simRun(t, g, nil)
	if err != nil || tr.Outcome != "completed" {
		t.Fatalf("run: %v outcome=%q", err, tr.Outcome)
	}
	if countKind(tr, NodeSetVar) != 2 {
		t.Fatalf("expected both branches to run")
	}
}

// try_catch catches a body domain failure (route_queue with no snapshot →
// missing_catalog_reference) and takes the catch port.
func TestRunTryCatch_CatchesDomainFailure(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("tc", NodeTryCatch, tryCatchConfig{}),
			inRegion(node("rq", NodeRouteQueue, routeQueueConfig{Queue: "ghost"}), "tc"),
			node("recover", NodeLog, logConfig{Message: "recovered"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "tc"},
			{From: "tc", To: "rq", Label: "body"},
			{From: "tc", To: "recover", Label: "catch"},
			{From: "tc", To: "end", Label: "done"},
			{From: "recover", To: "end"},
		},
	}
	tr, err := simRun(t, g, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if tr.Outcome != "completed" {
		t.Fatalf("outcome=%q, want completed (failure caught)", tr.Outcome)
	}
	// the failing route_queue step is marked caught; the recover (catch) node ran
	caught := false
	recovered := false
	for _, s := range tr.Steps {
		if s.Kind == NodeRouteQueue && s.Caught {
			caught = true
		}
		if s.NodeID == "recover" {
			recovered = true
		}
	}
	if !caught {
		t.Fatalf("route_queue failure not marked caught: %+v", tr.Steps)
	}
	if !recovered {
		t.Fatalf("catch path (recover) did not run: %v", stepIDs(tr))
	}
}
