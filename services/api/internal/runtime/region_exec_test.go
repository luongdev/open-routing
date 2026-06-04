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

// A branch rewriting a key back to its SNAPSHOT value must still win when it is
// the last writer — value-diff merging would drop it (review BLOCK). Branch 0
// sets z=2, branch 1 sets z=1 (== the pre-parallel snapshot); the merge must
// reflect branch 1 (last-writer-wins), so the downstream z==1 test is true.
func TestRunParallel_WriteTrackingNotValueDiff(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("zinit", NodeSetVar, setVarConfig{Name: "z", ValueExpr: "1"}),
			node("par", NodeParallel, parallelConfig{}),
			inRegion(node("b0", NodeSetVar, setVarConfig{Name: "z", ValueExpr: "2"}), "par#0"),
			inRegion(node("b1", NodeSetVar, setVarConfig{Name: "z", ValueExpr: "1"}), "par#1"),
			node("iff", NodeIfElse, ifElseConfig{Expr: "z == 1"}),
			node("yes", NodeLog, logConfig{Message: "z is 1"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "zinit"}, {From: "zinit", To: "par"},
			{From: "par", To: "b0", Label: "body:0"},
			{From: "par", To: "b1", Label: "body:1"},
			{From: "par", To: "iff", Label: "done"},
			{From: "iff", To: "yes", Label: "true"}, {From: "iff", To: "end", Label: "false"}, {From: "yes", To: "end"},
		},
	}
	tr, err := simRun(t, g, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := portOf(tr, "iff"); got != "true" {
		t.Fatalf("z==1 → %q, want true (last writer set z back to the snapshot value; steps=%v)", got, stepIDs(tr))
	}
}

// A parallel branch must run against an ISOLATED bag — branch 1 must not see
// branch 0's write. a starts 0; branch 0 sets a=1; branch 1 copies a→b. If the
// branches share a bag, b would be 1; isolated, branch 1 reads the snapshot
// (a=0) so b==0 downstream.
func TestRunParallel_BranchesAreIsolated(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("ainit", NodeSetVar, setVarConfig{Name: "a", ValueExpr: "0"}),
			node("par", NodeParallel, parallelConfig{}),
			inRegion(node("b0", NodeSetVar, setVarConfig{Name: "a", ValueExpr: "1"}), "par#0"),
			inRegion(node("b1", NodeSetVar, setVarConfig{Name: "b", ValueExpr: "a"}), "par#1"),
			node("iff", NodeIfElse, ifElseConfig{Expr: "b == 0"}),
			node("yes", NodeLog, logConfig{Message: "b is 0"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "ainit"}, {From: "ainit", To: "par"},
			{From: "par", To: "b0", Label: "body:0"},
			{From: "par", To: "b1", Label: "body:1"},
			{From: "par", To: "iff", Label: "done"},
			{From: "iff", To: "yes", Label: "true"}, {From: "iff", To: "end", Label: "false"}, {From: "yes", To: "end"},
		},
	}
	tr, err := simRun(t, g, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := portOf(tr, "iff"); got != "true" {
		t.Fatalf("branch 1 saw branch 0's write: b==0 → %q, want true", got)
	}
}

// loop_while must evaluate the condition BEFORE the limit check, so a loop that
// terminates naturally on its maxIter-th test completes (not loop_limit). With
// max_iter=1 the body flips the flag on the single allowed iteration; the next
// cond test is false → done (the limit-first ordering tripped loop_limit here).
func TestRunLoopWhile_CompletesAtExactlyMaxIter(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("init", NodeSetVar, setVarConfig{Name: "run", ValueExpr: "true"}),
			node("lw", NodeLoopWhile, loopWhileConfig{CondExpr: "run", MaxIter: 1}),
			inRegion(node("stop", NodeSetVar, setVarConfig{Name: "run", ValueExpr: "false"}), "lw"),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "init"}, {From: "init", To: "lw"}, {From: "lw", To: "stop", Label: "body"}, {From: "lw", To: "end", Label: "done"}},
	}
	tr, err := simRun(t, g, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if tr.Outcome != "completed" {
		t.Fatalf("outcome=%q code=%q, want completed (cond-before-limit)", tr.Outcome, tr.FailureCode)
	}
	if got := countKind(tr, NodeSetVar); got != 2 {
		t.Fatalf("set_var ran %d times, want 2 (init + one iteration)", got)
	}
}

// try_catch marks the EXACT failing step as caught — the second body step here
// fails, the first (a log) must NOT be flagged.
func TestRunTryCatch_MarksTheFailingLeaf(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("tc", NodeTryCatch, tryCatchConfig{}),
			inRegion(node("ok", NodeLog, logConfig{Message: "before"}), "tc"),
			inRegion(node("boom", NodeRouteQueue, routeQueueConfig{Queue: "ghost"}), "tc"),
			node("recover", NodeLog, logConfig{Message: "recovered"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "tc"},
			{From: "tc", To: "ok", Label: "body"}, {From: "ok", To: "boom"},
			{From: "tc", To: "recover", Label: "catch"},
			{From: "tc", To: "end", Label: "done"},
			{From: "recover", To: "end"},
		},
	}
	tr, err := simRun(t, g, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, s := range tr.Steps {
		if s.NodeID == "ok" && s.Caught {
			t.Fatalf("the non-failing log step was marked caught")
		}
		if s.NodeID == "boom" && !s.Caught {
			t.Fatalf("the failing route_queue step was not marked caught")
		}
	}
}

func TestDeepClone_IsolatesNestedContainers(t *testing.T) {
	orig := map[string]any{
		"m": map[string]any{"k": 1},
		"s": []any{1, 2},
	}
	c := cloneVars(orig)
	orig["m"].(map[string]any)["k"] = 99
	orig["s"].([]any)[0] = 99
	if got := c["m"].(map[string]any)["k"]; got != 1 {
		t.Fatalf("nested map shared: clone m.k = %v, want 1", got)
	}
	if got := c["s"].([]any)[0]; got != 1 {
		t.Fatalf("nested slice shared: clone s[0] = %v, want 1", got)
	}
}

// RunResult.Vars must reflect the parallel-merged bag, not the stale map Run
// captured before parallel reassigned state.vars (review MED).
func TestRun_VarsReflectParallelMerge(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("par", NodeParallel, parallelConfig{}),
			inRegion(node("b0", NodeSetVar, setVarConfig{Name: "merged", ValueExpr: "7"}), "par#0"),
			inRegion(node("b1", NodeSetVar, setVarConfig{Name: "other", ValueExpr: "9"}), "par#1"),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "par"},
			{From: "par", To: "b0", Label: "body:0"},
			{From: "par", To: "b1", Label: "body:1"},
			{From: "par", To: "end", Label: "done"},
		},
	}
	if issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil); err != nil || len(issues) != 0 {
		t.Fatalf("invalid: %v %+v", err, issues)
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	ex := NewExecutor(DefaultRegistry(), WithAutoResume())
	res, err := ex.Run(context.Background(), NewVirtualClock(time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)), plan, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Vars["merged"] != float64(7) || res.Vars["other"] != float64(9) {
		t.Fatalf("RunResult.Vars stale after parallel: merged=%v other=%v", res.Vars["merged"], res.Vars["other"])
	}
}

// A failure that bubbles through a control node (loop body leaf → loop → outer
// try_catch) must mark the LEAF as caught, not the relaying control node
// (review: failIndex overwrite).
func TestRunTryCatch_MarksLeafBubbledThroughLoop(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("tc", NodeTryCatch, tryCatchConfig{}),
			inRegion(node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "items", MaxIter: 10}), "tc"),
			inRegion(node("boom", NodeRouteQueue, routeQueueConfig{Queue: "ghost"}), "lf"),
			node("recover", NodeLog, logConfig{Message: "recovered"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "tc"},
			{From: "tc", To: "lf", Label: "body"},
			{From: "lf", To: "boom", Label: "body"},
			{From: "tc", To: "recover", Label: "catch"},
			{From: "tc", To: "end", Label: "done"},
			{From: "recover", To: "end"},
		},
	}
	tr, err := simRun(t, g, map[string]any{"items": []any{"a"}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if tr.Outcome != "completed" {
		t.Fatalf("outcome=%q, want completed (caught)", tr.Outcome)
	}
	for _, s := range tr.Steps {
		if s.NodeID == "boom" && !s.Caught {
			t.Fatalf("the failing leaf was not marked caught")
		}
		if s.NodeID == "lf" && s.Caught {
			t.Fatalf("the relaying loop_for node was wrongly marked caught")
		}
	}
}

// A terminal end inside a region is rejected at validation — which is why the
// walkResult.terminated propagation through control nodes is defense-in-depth,
// not a reachable path. Lock that validation guard so the invariant holds.
func TestValidate_RejectsEndInsideRegion(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "items", MaxIter: 10}),
			inRegion(node("bodyend", NodeEnd, nil), "lf"),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "lf"},
			{From: "lf", To: "bodyend", Label: "body"},
			{From: "lf", To: "end", Label: "done"},
		},
	}
	issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	found := false
	for _, is := range issues {
		if is.Code == "end_in_region" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected end_in_region issue, got %+v", issues)
	}
}

// parallel branches are CONCURRENT: a wait in each branch starts from the same
// virtual time, so the join advances the clock to the LONGEST branch, not the
// sum (reviewers HIGH: shared clock).
func TestRunParallel_ClockJoinsAtMaxNotSum(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("par", NodeParallel, parallelConfig{}),
			inRegion(node("w0", NodeWait, waitConfig{DurationMs: 10000}), "par#0"),
			inRegion(node("w1", NodeWait, waitConfig{DurationMs: 4000}), "par#1"),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "par"},
			{From: "par", To: "w0", Label: "body:0"},
			{From: "par", To: "w1", Label: "body:1"},
			{From: "par", To: "end", Label: "done"},
		},
	}
	if issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil); err != nil || len(issues) != 0 {
		t.Fatalf("invalid: %v %+v", err, issues)
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	t0 := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	clk := NewVirtualClock(t0)
	ex := NewExecutor(DefaultRegistry(), WithAutoResume())
	if _, err := ex.Run(context.Background(), clk, plan, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := clk.Now().Sub(t0); got != 10*time.Second {
		t.Fatalf("virtual time after parallel = %s, want 10s (max branch, not the 14s sum)", got)
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
