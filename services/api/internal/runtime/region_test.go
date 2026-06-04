package runtime

import (
	"context"
	"testing"
)

func inRegion(n GraphNode, region string) GraphNode { n.Region = region; return n }

// loopGraph: trigger -> loop_for(lf) --body--> body1(log) ; lf --done--> end.
func loopGraph() *Graph {
	return &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "items", MaxIter: 10}),
			inRegion(node("body1", NodeLog, logConfig{Message: "x"}), "lf"),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "lf"},
			{ID: "b", From: "lf", To: "body1", Label: "body"},
			{From: "lf", To: "end", Label: "done"},
		},
	}
}

func TestCompile_BuildsRegions(t *testing.T) {
	plan, err := Compile(loopGraph(), DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(plan.Regions) != 1 {
		t.Fatalf("regions = %d, want 1: %+v", len(plan.Regions), plan.Regions)
	}
	r := plan.Regions[0]
	if r.ID != "lf" || r.OwnerID != "lf" || r.Entry != "body1" || len(r.StepIDs) != 1 || r.StepIDs[0] != "body1" {
		t.Fatalf("region = %+v", r)
	}
	// the body edge is classified, not a flow edge
	var bodyKind, doneKind EdgeKind
	for _, e := range plan.Edges {
		if e.From == "lf" && e.To == "body1" {
			bodyKind = e.Kind
		}
		if e.From == "lf" && e.To == "end" {
			doneKind = e.Kind
		}
	}
	if bodyKind != EdgeBody {
		t.Errorf("lf->body1 kind = %q, want body", bodyKind)
	}
	if doneKind != EdgeFlow {
		t.Errorf("lf->end kind = %q, want flow", doneKind)
	}
	// the body step carries its region
	for _, s := range plan.Steps {
		if s.NodeID == "body1" && s.Region != "lf" {
			t.Errorf("body1 step region = %q, want lf", s.Region)
		}
	}
}

func TestCompile_RegionValid(t *testing.T) {
	issues, err := ValidateGraph(context.Background(), loopGraph(), DefaultRegistry(), nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected a clean loop graph, got: %+v", issues)
	}
}

func TestValidateRegions_CatchesViolations(t *testing.T) {
	cases := []struct {
		name string
		g    *Graph
		code string
	}{
		{"end in region", &Graph{
			Nodes: []GraphNode{node("t", NodeTrigger, nil), node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "x", MaxIter: 1}),
				inRegion(node("e2", NodeEnd, nil), "lf"), node("end", NodeEnd, nil)},
			Edges: []GraphEdge{{From: "t", To: "lf"}, {From: "lf", To: "e2", Label: "body"}, {From: "lf", To: "end", Label: "done"}},
		}, IssueEndInRegion},
		{"boundary crossing", &Graph{
			Nodes: []GraphNode{node("t", NodeTrigger, nil), node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "x", MaxIter: 1}),
				inRegion(node("body1", NodeLog, logConfig{Message: "x"}), "lf"), node("end", NodeEnd, nil)},
			// flow edge from a region node to a top-level node — crosses boundary
			Edges: []GraphEdge{{From: "t", To: "lf"}, {From: "lf", To: "body1", Label: "body"}, {ID: "x", From: "body1", To: "end"}, {From: "lf", To: "end", Label: "done"}},
		}, IssueRegionBoundary},
		{"entry not member", &Graph{
			Nodes: []GraphNode{node("t", NodeTrigger, nil), node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "x", MaxIter: 1}),
				node("body1", NodeLog, logConfig{Message: "x"}), node("end", NodeEnd, nil)}, // body1 has NO region
			Edges: []GraphEdge{{From: "t", To: "lf"}, {From: "lf", To: "body1", Label: "body"}, {From: "lf", To: "end", Label: "done"}},
		}, IssueRegionEntry},
		{"loop var shadow", &Graph{
			Nodes: []GraphNode{node("t", NodeTrigger, nil), node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "x", MaxIter: 1}),
				inRegion(node("sv", NodeSetVar, setVarConfig{Name: "item", ValueExpr: "1"}), "lf"), node("end", NodeEnd, nil)},
			Edges: []GraphEdge{{From: "t", To: "lf"}, {From: "lf", To: "sv", Label: "body"}, {From: "lf", To: "end", Label: "done"}},
		}, IssueLoopVarShadow},
		{"zero-body control owner", &Graph{
			// loop_for with NO body edge — must be flagged, not compile clean.
			Nodes: []GraphNode{node("t", NodeTrigger, nil), node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "x", MaxIter: 1}), node("end", NodeEnd, nil)},
			Edges: []GraphEdge{{From: "t", To: "lf"}, {From: "lf", To: "end", Label: "done"}},
		}, IssueRegionBranches},
		{"nested loop reuses outer loop var", &Graph{
			// inner loop_for (in outer's region) reuses item_var "x" → shadows.
			Nodes: []GraphNode{node("t", NodeTrigger, nil),
				node("lf1", NodeLoopFor, loopForConfig{ArrayExpr: "a", MaxIter: 5, ItemVar: "x"}),
				inRegion(node("lf2", NodeLoopFor, loopForConfig{ArrayExpr: "b", MaxIter: 5, ItemVar: "x"}), "lf1"),
				inRegion(node("body", NodeLog, logConfig{Message: "y"}), "lf2"),
				node("end", NodeEnd, nil)},
			Edges: []GraphEdge{{From: "t", To: "lf1"}, {From: "lf1", To: "lf2", Label: "body"}, {From: "lf2", To: "body", Label: "body"}, {From: "lf1", To: "end", Label: "done"}},
		}, IssueLoopVarShadow},
		{"region internal cycle (no exit)", &Graph{
			// two body nodes pointing at each other → no in-region exit.
			Nodes: []GraphNode{node("t", NodeTrigger, nil), node("lf", NodeLoopFor, loopForConfig{ArrayExpr: "x", MaxIter: 1}),
				inRegion(node("a", NodeLog, logConfig{Message: "a"}), "lf"), inRegion(node("b", NodeLog, logConfig{Message: "b"}), "lf"), node("end", NodeEnd, nil)},
			Edges: []GraphEdge{{From: "t", To: "lf"}, {From: "lf", To: "a", Label: "body"}, {From: "a", To: "b"}, {From: "b", To: "a"}, {From: "lf", To: "end", Label: "done"}},
		}, IssueNoPathToTerminal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues, _ := ValidateGraph(context.Background(), c.g, DefaultRegistry(), nil)
			if codes(issues)[c.code] == 0 {
				t.Fatalf("expected issue %q; got %+v", c.code, issues)
			}
		})
	}
}
