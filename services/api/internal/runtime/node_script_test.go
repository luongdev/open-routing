package runtime

import (
	"context"
	"testing"
)

func runScript(t *testing.T, code string, input map[string]any) RunResult {
	t.Helper()
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("s", NodeScript, map[string]any{"code": code, "save_as": "out"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "s"}, {From: "s", To: "end"}},
	}
	if issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil); err != nil || len(issues) != 0 {
		t.Fatalf("graph invalid: err=%v issues=%+v", err, issues)
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	res, err := NewExecutor(DefaultRegistry()).Run(context.Background(), RealClock{}, plan, input)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return res
}

func TestScript_ComputesOverVars(t *testing.T) {
	res := runScript(t, `return vars.a + vars.b * 2`, map[string]any{"a": float64(3), "b": float64(5)})
	if got, _ := toFloatField(res.Vars["out"]); got != 13 {
		t.Fatalf("out = %v, want 13", res.Vars["out"])
	}
}

func TestScript_ReturnsObjectAndArray(t *testing.T) {
	res := runScript(t, `return { name = vars.customer.name, tags = {"a","b"} }`,
		map[string]any{"customer": map[string]any{"name": "Lan"}})
	m, ok := res.Vars["out"].(map[string]any)
	if !ok {
		t.Fatalf("out is not a map: %T %v", res.Vars["out"], res.Vars["out"])
	}
	if m["name"] != "Lan" {
		t.Fatalf("out.name = %v, want Lan", m["name"])
	}
	tags, ok := m["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Fatalf("out.tags = %v, want [a b]", m["tags"])
	}
}

func TestScript_Deterministic(t *testing.T) {
	// Seeded RNG → identical result across runs (replay stability).
	a := runScript(t, `return math.random()`, nil)
	b := runScript(t, `return math.random()`, nil)
	if a.Vars["out"] != b.Vars["out"] {
		t.Fatalf("non-deterministic: %v vs %v", a.Vars["out"], b.Vars["out"])
	}
}

func TestScript_RuntimeErrorIsDomainFailure(t *testing.T) {
	// error() in the chunk → step failure, not a hung/panicked run.
	res := runScript(t, `error("boom")`, nil)
	if res.Trace.Outcome != "failed" {
		t.Fatalf("outcome = %q, want failed", res.Trace.Outcome)
	}
}

func TestScript_Validate(t *testing.T) {
	n, _ := DefaultRegistry().Lookup(NodeScript)
	check := func(cfg any) []string {
		issues, err := n.Validate(context.Background(), node("x", NodeScript, cfg), nil, nil)
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		codes := make([]string, len(issues))
		for i, is := range issues {
			codes[i] = is.Code
		}
		return codes
	}
	if codes := check(map[string]any{"code": ""}); !hasCode(codes, IssueMissingField) {
		t.Fatalf("empty code: %v, want missing_required_field", codes)
	}
	if codes := check(map[string]any{"code": "return 1 +"}); !hasCode(codes, IssueInvalidConfig) {
		t.Fatalf("syntax error: %v, want invalid_config", codes)
	}
	if codes := check(map[string]any{"code": "return vars.a", "save_as": "out"}); len(codes) != 0 {
		t.Fatalf("valid: %v, want none", codes)
	}
}

// The sandbox does NOT expose os/io — a script touching them fails (no ambient
// authority), proving network/file/clock access is unavailable.
func TestScript_SandboxNoOSIO(t *testing.T) {
	res := runScript(t, `return os.time()`, nil)
	if res.Trace.Outcome != "failed" {
		t.Fatalf("os.time() should fail in the sandbox; outcome = %q", res.Trace.Outcome)
	}
}
