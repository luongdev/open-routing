package expr

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Env is the evaluation surface: a variable lookup (dotted paths) + a clock for
// date.* (the runtime passes its virtual/real clock, so eval is deterministic).
type Env interface {
	Lookup(path string) (any, bool)
	Now() time.Time
}

// EvalBool parses + evaluates src to a boolean (truthy). Empty/whitespace → false
// (a missing condition is a validation error, surfaced earlier — never a panic).
func EvalBool(src string, env Env) (bool, error) {
	if strings.TrimSpace(src) == "" {
		return false, nil
	}
	v, err := evalSrc(src, env)
	if err != nil {
		return false, err
	}
	return truthy(v), nil
}

// EvalString parses + evaluates src to its string form (switch_case matching).
func EvalString(src string, env Env) (string, error) {
	if strings.TrimSpace(src) == "" {
		return "", nil
	}
	v, err := evalSrc(src, env)
	if err != nil {
		return "", err
	}
	return toStr(v), nil
}

// EvalValue parses + evaluates src to its typed value (set_var / compute). The
// internal `unresolved` sentinel (an undefined bareword) is unwrapped to a plain
// string so it never leaks into stored variables or the trace.
func EvalValue(src string, env Env) (any, error) {
	if strings.TrimSpace(src) == "" {
		return "", nil
	}
	v, err := evalSrc(src, env)
	if err != nil {
		return nil, err
	}
	if u, ok := v.(unresolved); ok {
		return string(u), nil
	}
	return v, nil
}

func evalSrc(src string, env Env) (any, error) {
	n, perr := Parse(src)
	if perr != nil {
		return nil, perr
	}
	return Eval(n, env)
}

// Eval walks the AST. A missing variable resolves to nil (falsey), matching the
// original bare-var semantics. AND/OR short-circuit.
func Eval(n Node, env Env) (any, error) {
	switch x := n.(type) {
	case LitNode:
		return x.Val, nil
	case VarNode:
		if v, ok := env.Lookup(x.Path); ok {
			return v, nil
		}
		return unresolved(x.Path), nil
	case UnaryNode: // NOT
		v, err := Eval(x.X, env)
		if err != nil {
			return nil, err
		}
		return !truthy(v), nil
	case BinaryNode:
		switch x.Op {
		case "AND":
			l, err := Eval(x.L, env)
			if err != nil {
				return nil, err
			}
			if !truthy(l) {
				return false, nil
			}
			r, err := Eval(x.R, env)
			if err != nil {
				return nil, err
			}
			return truthy(r), nil
		case "OR":
			l, err := Eval(x.L, env)
			if err != nil {
				return nil, err
			}
			if truthy(l) {
				return true, nil
			}
			r, err := Eval(x.R, env)
			if err != nil {
				return nil, err
			}
			return truthy(r), nil
		default: // comparison
			l, err := Eval(x.L, env)
			if err != nil {
				return nil, err
			}
			r, err := Eval(x.R, env)
			if err != nil {
				return nil, err
			}
			return compare(l, r, x.Op)
		}
	case CallNode:
		fn, ok := Lookup(x.Ns, x.Name)
		if !ok {
			return nil, fmt.Errorf("unknown function %s.%s", x.Ns, x.Name)
		}
		args := make([]any, len(x.Args))
		for i, a := range x.Args {
			v, err := Eval(a, env)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}
		if fn.Arity >= 0 && len(args) != fn.Arity {
			return nil, fmt.Errorf("%s.%s expects %d argument(s), got %d", x.Ns, x.Name, fn.Arity, len(args))
		}
		return fn.Call(env.Now(), args)
	}
	return nil, fmt.Errorf("eval: unsupported node %T", n)
}

// ---- Dotted-path resolution (panic-safe) ----

// MapEnv is an in-memory Env for tests and simple callers. Lookup tries a flat
// dotted key first, then walks nested maps/arrays.
type MapEnv struct {
	Vars  map[string]any
	Clock time.Time
}

func (m MapEnv) Now() time.Time { return m.Clock }
func (m MapEnv) Lookup(path string) (any, bool) {
	if v, ok := m.Vars[path]; ok {
		return v, true
	}
	return Path(m.Vars, path)
}

// Path walks a dotted path from a root value into nested map[string]any / []any.
// Any miss / out-of-range / negative index / nil / type mismatch returns
// (nil,false) — it never panics.
func Path(root any, dotted string) (any, bool) {
	cur := root
	for _, seg := range strings.Split(dotted, ".") {
		var ok bool
		cur, ok = Index(cur, seg)
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// Index does one resolution step: map key or array index.
func Index(v any, seg string) (any, bool) {
	switch c := v.(type) {
	case map[string]any:
		x, ok := c[seg]
		return x, ok
	case []any:
		i, err := strconv.Atoi(seg)
		if err != nil || i < 0 || i >= len(c) {
			return nil, false
		}
		return c[i], true
	default:
		return nil, false
	}
}
