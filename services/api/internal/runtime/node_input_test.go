package runtime

import (
	"context"
	"testing"
)

// runInput simulates trigger → <input node> → (one end per branch port), pinning
// a scripted captured value (or none), and returns the trace + final vars.
func runInput(t *testing.T, kind NodeKind, cfg map[string]any, ports []string, scripted map[string]any) (Trace, map[string]any) {
	t.Helper()
	nodes := []GraphNode{node("t", NodeTrigger, nil), node("in", kind, cfg)}
	edges := []GraphEdge{{From: "t", To: "in"}}
	for _, p := range ports {
		end := "end_" + p
		nodes = append(nodes, node(end, NodeEnd, map[string]any{"outcome": p}))
		edges = append(edges, GraphEdge{From: "in", To: end, Label: p})
	}
	g := &Graph{Nodes: nodes, Edges: edges}
	if issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil); err != nil || len(issues) != 0 {
		t.Fatalf("graph invalid: err=%v issues=%+v", err, issues)
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	tr, err := Simulate(context.Background(), DefaultRegistry(), plan, SimInput{ScriptedInputs: scripted})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	// Recover the final vars by re-running the executor directly (Simulate returns
	// only the trace); assert via the terminal end outcome + trace below instead.
	return tr, nil
}

// endOutcome returns the outcome recorded by the end node the run reached.
func endOutcome(tr Trace) string {
	for i := len(tr.Steps) - 1; i >= 0; i-- {
		if tr.Steps[i].Kind == NodeEnd {
			if o, ok := tr.Steps[i].Output["outcome"].(string); ok {
				return o
			}
		}
	}
	return ""
}

func inputStepPort(tr Trace, nodeID string) (string, map[string]any) {
	for _, s := range tr.Steps {
		if s.NodeID == nodeID {
			return s.Port, s.Output
		}
	}
	return "", nil
}

func TestInput_ScriptedValueBranchesAndCaptures(t *testing.T) {
	// get_dtmf with a pinned value → captured branch, digits stored.
	tr, _ := runInput(t, NodeGetDTMF, map[string]any{"save_as": "digits"}, []string{"captured", "timeout"}, map[string]any{"in": "1234"})
	if got := endOutcome(tr); got != "captured" {
		t.Fatalf("get_dtmf reached %q end, want captured", got)
	}
	if port, out := inputStepPort(tr, "in"); port != "captured" || out["captured"] != "1234" {
		t.Fatalf("get_dtmf step port=%q out=%+v", port, out)
	}

	// manual_approval: "rejected" value → rejected branch.
	tr, _ = runInput(t, NodeManualApproval, nil, []string{"approved", "rejected", "timeout"}, map[string]any{"in": "rejected"})
	if got := endOutcome(tr); got != "rejected" {
		t.Fatalf("manual_approval reached %q, want rejected", got)
	}
	// manual_approval: "approved" value → approved branch.
	tr, _ = runInput(t, NodeManualApproval, nil, []string{"approved", "rejected", "timeout"}, map[string]any{"in": "approved"})
	if got := endOutcome(tr); got != "approved" {
		t.Fatalf("manual_approval reached %q, want approved", got)
	}

	// detect_speech: empty value → no_match branch.
	tr, _ = runInput(t, NodeDetectSpeech, nil, []string{"recognized", "no_match", "timeout"}, map[string]any{"in": ""})
	if got := endOutcome(tr); got != "no_match" {
		t.Fatalf("detect_speech empty reached %q, want no_match", got)
	}
}

func TestInput_NoValueTimesOut(t *testing.T) {
	// No scripted value → suspend → sim fast-forwards → timeout branch.
	tr, _ := runInput(t, NodePromptText, map[string]any{"timeout_sec": 10}, []string{"captured", "timeout"}, nil)
	if tr.Outcome != "completed" {
		t.Fatalf("outcome = %q, want completed", tr.Outcome)
	}
	if got := endOutcome(tr); got != "timeout" {
		t.Fatalf("prompt_text unanswered reached %q, want timeout", got)
	}
}

func TestInput_Validate(t *testing.T) {
	n, _ := DefaultRegistry().Lookup(NodeGetDTMF)
	check := func(cfg any) []string {
		issues, err := n.Validate(context.Background(), node("x", NodeGetDTMF, cfg), nil, nil)
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		codes := make([]string, len(issues))
		for i, is := range issues {
			codes[i] = is.Code
		}
		return codes
	}
	if codes := check(map[string]any{"save_as": "1bad"}); !hasCode(codes, IssueInvalidConfig) {
		t.Fatalf("bad save_as: %v, want invalid_config", codes)
	}
	if codes := check(map[string]any{"timeout_sec": -5}); !hasCode(codes, IssueInvalidConfig) {
		t.Fatalf("negative timeout: %v, want invalid_config", codes)
	}
	if codes := check(map[string]any{"prompt": "Press 1 ${1 +}"}); !hasCode(codes, IssueInvalidExpr) {
		t.Fatalf("bad prompt expr: %v, want invalid_expression", codes)
	}
	if codes := check(map[string]any{"save_as": "digits", "timeout_sec": 5, "prompt": "Press ${menu.key}"}); len(codes) != 0 {
		t.Fatalf("valid config: %v, want none", codes)
	}
}
