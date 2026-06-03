package expr

import (
	"testing"
	"time"
)

func env(vars map[string]any) MapEnv {
	return MapEnv{Vars: vars, Clock: time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)}
}

func TestEvalBool_Comparisons(t *testing.T) {
	e := env(map[string]any{"tier": "gold", "age": float64(30), "vip": true})
	cases := []struct {
		src  string
		want bool
	}{
		{`tier == gold`, true},
		{`tier == "silver"`, false},
		{`tier != silver`, true},
		{`age > 18`, true},
		{`age <= 30`, true},
		{`age < 30`, false},
		{`vip`, true},
		{`missing`, false},
		{``, false},
	}
	for _, c := range cases {
		got, err := EvalBool(c.src, e)
		if err != nil {
			t.Fatalf("EvalBool(%q): %v", c.src, err)
		}
		if got != c.want {
			t.Errorf("EvalBool(%q) = %v, want %v", c.src, got, c.want)
		}
	}
}

func TestEvalBool_BooleanLogicPrecedenceAndShortCircuit(t *testing.T) {
	e := env(map[string]any{"a": true, "b": false, "c": true})
	cases := []struct {
		src  string
		want bool
	}{
		{`a AND b`, false},
		{`a OR b`, true},
		{`a AND b OR c`, true},   // (a AND b) OR c
		{`a AND (b OR c)`, true}, // grouping
		{`NOT b`, true},
		{`NOT a == b`, true},             // NOT (a == b): a(true)==b(false) → false → NOT → true
		{`b AND undefinedfn.x()`, false}, // short-circuit: RHS not evaluated, no error
	}
	for _, c := range cases {
		got, err := EvalBool(c.src, e)
		if err != nil {
			t.Fatalf("EvalBool(%q): %v", c.src, err)
		}
		if got != c.want {
			t.Errorf("EvalBool(%q) = %v, want %v", c.src, got, c.want)
		}
	}
}

func TestParse_RejectsChainedComparison(t *testing.T) {
	if _, err := Parse(`a < b < c`); err == nil {
		t.Fatal("expected chained-comparison parse error")
	}
	for _, bad := range []string{`a ==`, `(a AND`, `a == == b`, `"unterminated`} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("expected parse error for %q", bad)
		}
	}
}

func TestEval_Functions(t *testing.T) {
	e := env(map[string]any{
		"name":  "  Alice ",
		"score": float64(-4),
		"items": []any{"a", "b", "c"},
		"cust":  map[string]any{"tier": "gold"},
	})
	cases := []struct {
		src  string
		want bool
	}{
		{`str.upper(str.trim(name)) == ALICE`, true},
		{`str.contains(name, "lic")`, true},
		{`num.abs(score) == 4`, true},
		{`num.max(1, 5, 3) == 5`, true},
		{`num.sum(items) == 0`, false}, // strings → not numbers → error path tested below
		{`arr.len(items) == 3`, true},
		{`arr.contains(items, "b")`, true},
		{`cust.tier == gold`, true}, // dotted into nested map
	}
	for _, c := range cases {
		got, err := EvalBool(c.src, e)
		if c.src == `num.sum(items) == 0` {
			if err == nil {
				t.Errorf("expected error for num.sum over strings")
			}
			continue
		}
		if err != nil {
			t.Fatalf("EvalBool(%q): %v", c.src, err)
		}
		if got != c.want {
			t.Errorf("EvalBool(%q) = %v, want %v", c.src, got, c.want)
		}
	}
}

func TestEval_UnknownFunctionAndArity(t *testing.T) {
	e := env(nil)
	if _, err := EvalBool(`foo.bar(1)`, e); err == nil {
		t.Error("expected unknown-function error")
	}
	if _, err := EvalBool(`num.abs(1, 2)`, e); err == nil {
		t.Error("expected arity error")
	}
}

func TestCheck_RejectsBadCallsStatically(t *testing.T) {
	bad := []string{
		`foo.bar(1)`,      // unknown function
		`num.abs(1, 2)`,   // too many args
		`num.abs()`,       // too few args
		`num.min()`,       // variadic with zero args
		`vip AND foo.x(1)`, // nested unknown function
	}
	for _, src := range bad {
		node, perr := Parse(src)
		if perr != nil {
			t.Fatalf("Parse(%q) unexpected parse error: %v", src, perr)
		}
		if err := Check(node); err == nil {
			t.Errorf("Check(%q) = nil, want semantic error", src)
		}
	}
	for _, src := range []string{`num.abs(1)`, `num.min(1, 2, 3)`, `str.upper(name) == X`, `vip`} {
		node, perr := Parse(src)
		if perr != nil {
			t.Fatalf("Parse(%q) unexpected parse error: %v", src, perr)
		}
		if err := Check(node); err != nil {
			t.Errorf("Check(%q) = %v, want nil", src, err)
		}
	}
}

func TestEvalString_SwitchValue(t *testing.T) {
	e := env(map[string]any{"lang": "es"})
	got, err := EvalString(`lang`, e)
	if err != nil || got != "es" {
		t.Fatalf("EvalString(lang) = %q, %v", got, err)
	}
	got, _ = EvalString(`str.upper(lang)`, e)
	if got != "ES" {
		t.Fatalf("EvalString(str.upper(lang)) = %q", got)
	}
}

func TestPath_NestedAndPanicSafe(t *testing.T) {
	root := map[string]any{
		"a": map[string]any{"b": []any{map[string]any{"c": "deep"}}},
	}
	if v, ok := Path(root, "a.b.0.c"); !ok || v != "deep" {
		t.Fatalf("Path nested = %v, %v", v, ok)
	}
	for _, miss := range []string{"a.x", "a.b.9.c", "a.b.-1", "a.b.0.c.d", "nope"} {
		if _, ok := Path(root, miss); ok {
			t.Errorf("Path(%q) should miss gracefully", miss)
		}
	}
}

func TestEval_DateDeterministic(t *testing.T) {
	e := env(nil) // clock = 2026-06-03 UTC
	got, err := EvalBool(`date.year(date.now()) == 2026`, e)
	if err != nil || !got {
		t.Fatalf("date.year(date.now()) == 2026 → %v, %v", got, err)
	}
}

func TestCatalog_StableAndComplete(t *testing.T) {
	cat := Catalog()
	if len(cat) < 20 {
		t.Fatalf("catalog has %d functions, expected the broad library", len(cat))
	}
	for i, f := range cat {
		if f.Signature == "" || f.Ns == "" || f.Name == "" {
			t.Errorf("function %d incomplete: %+v", i, f)
		}
		if i > 0 {
			prev := cat[i-1]
			if prev.Ns > f.Ns || (prev.Ns == f.Ns && prev.Name > f.Name) {
				t.Errorf("catalog not sorted at %d: %s.%s after %s.%s", i, f.Ns, f.Name, prev.Ns, prev.Name)
			}
		}
	}
}
