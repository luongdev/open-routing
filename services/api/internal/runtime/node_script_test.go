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

// Cross-AI review BLOCKs: dofile/loadfile (file read) and string.rep (OOM) are
// removed from the sandbox; calling them fails rather than escaping/allocating.
func TestScript_SandboxHolesClosed(t *testing.T) {
	for _, code := range []string{`return dofile("/etc/passwd")`, `return loadfile("/etc/passwd")`, `return string.rep("a", 100)`} {
		res := runScript(t, code, nil)
		if res.Trace.Outcome != "failed" {
			t.Fatalf("sandbox hole open for %q (outcome %q)", code, res.Trace.Outcome)
		}
	}
}

// load/loadstring/collectgarbage are removed from the sandbox (review B2).
func TestScript_DynamicCodeBlocked(t *testing.T) {
	for _, code := range []string{`return load("return 1")()`, `return loadstring("return 1")()`, `collectgarbage(); return 1`} {
		res := runScript(t, code, nil)
		if res.Trace.Outcome != "failed" {
			t.Fatalf("sandbox should block %q (outcome %q)", code, res.Trace.Outcome)
		}
	}
}

// Non-finite numbers (NaN/Inf) don't leak into the var bag (review H9).
func TestScript_NonFiniteRejected(t *testing.T) {
	res := runScript(t, `return 0/0`, nil) // NaN
	if res.Trace.Outcome != "completed" {
		t.Fatalf("0/0 outcome = %q, want completed", res.Trace.Outcome)
	}
	if res.Vars["out"] != nil {
		t.Fatalf("NaN leaked into var: %v", res.Vars["out"])
	}
}

// pairs(vars) is deterministic across runs (sorted-key insertion, review H8).
func TestScript_DeterministicPairs(t *testing.T) {
	code := `local s = ""; for k,_ in pairs(vars) do s = s .. k .. "," end; return s`
	input := map[string]any{"zebra": 1, "alpha": 2, "mike": 3, "bravo": 4}
	a := runScript(t, code, input)
	b := runScript(t, code, input)
	if a.Vars["out"] != b.Vars["out"] {
		t.Fatalf("pairs order nondeterministic: %v vs %v", a.Vars["out"], b.Vars["out"])
	}
}

// A self-referential table can't overflow the Go stack during Lua→Go conversion.
func TestScript_CyclicTableBounded(t *testing.T) {
	res := runScript(t, `local t = {}; t.self = t; return t`, nil)
	// Either completes (depth-capped conversion) or fails — must NOT crash/hang.
	if res.Trace.Outcome != "completed" && res.Trace.Outcome != "failed" {
		t.Fatalf("cyclic table: unexpected outcome %q", res.Trace.Outcome)
	}
}

// Same input replays identically; different input yields a different RNG stream.
func TestScript_PerRunSeed(t *testing.T) {
	a := runScript(t, `return math.random(1,1000000)`, map[string]any{"k": "x"})
	a2 := runScript(t, `return math.random(1,1000000)`, map[string]any{"k": "x"})
	b := runScript(t, `return math.random(1,1000000)`, map[string]any{"k": "y"})
	if a.Vars["out"] != a2.Vars["out"] {
		t.Fatalf("same input not replay-stable: %v vs %v", a.Vars["out"], a2.Vars["out"])
	}
	if a.Vars["out"] == b.Vars["out"] {
		t.Fatalf("different input produced same RNG value %v — seed not input-derived", a.Vars["out"])
	}
}

// string.format and string.rep are removed (allocator DoS; SetMx is unsafe).
func TestScript_AllocatorsBlocked(t *testing.T) {
	for _, code := range []string{`return string.rep("a", 10)`, `return string.format("%d", 1)`} {
		if runScript(t, code, nil).Trace.Outcome != "failed" {
			t.Fatalf("expected %q to fail (allocator removed)", code)
		}
	}
}
