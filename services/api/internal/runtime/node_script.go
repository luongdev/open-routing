package runtime

// node_script.go — the v0.2 `script` escape hatch: a SANDBOXED Lua chunk over the
// variable bag. Deterministic + safe by construction:
//   - no IO/os libs opened (no file/network/os.time/os.execute) — only base,
//     table, string, math;
//   - math.random is seeded to a constant so a run replays identically;
//   - execution is bounded by a context deadline (a runaway loop is interrupted
//     and surfaces as a step error, never hangs the run).
// The chunk reads the bag via the global `vars` table and returns a value; the
// return is stored into save_as (when set). It cannot reach the network, the DB,
// or wall-clock time — exactly the determinism the simulator/replay relies on.

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	mrand "math/rand"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// maxLuaDepth bounds Lua→Go conversion so a self-referential or pathologically
// nested return value can't blow the Go stack (the deadline only fences VM
// instructions, not the post-call conversion). Cross-AI review BLOCK.
const maxLuaDepth = 64

const scriptDeadline = 200 * time.Millisecond

type scriptConfig struct {
	Code   string `json:"code"`
	SaveAs string `json:"save_as,omitempty"`
}

type scriptNode struct{ baseNode }

func (scriptNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[scriptConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	var issues []ValidationIssue
	if cfg.Code == "" {
		issues = append(issues, fieldIssue(n.ID, "code", IssueMissingField, "script requires code"))
	} else {
		// Parse-check the chunk at publish so a syntax error fails validation,
		// not mid-run. A throwaway state compiles without executing.
		l := lua.NewState(lua.Options{SkipOpenLibs: true})
		_, lerr := l.LoadString(cfg.Code)
		l.Close()
		if lerr != nil {
			issues = append(issues, fieldIssue(n.ID, "code", IssueInvalidConfig, fmt.Sprintf("lua syntax error: %v", lerr)))
		}
	}
	if cfg.SaveAs != "" && !validVarName(cfg.SaveAs) {
		issues = append(issues, fieldIssue(n.ID, "save_as", IssueInvalidConfig, "save_as must be an identifier (optionally dotted) and not a reserved word"))
	}
	return issues, nil
}

func (scriptNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[scriptConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

func (scriptNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[scriptConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}

	dctx, cancel := context.WithTimeout(ctx, scriptDeadline)
	defer cancel()

	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	L.SetContext(dctx)
	// Only pure libs — NO io/os/debug/package (no file, network, os.time).
	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		L.Push(L.NewFunction(lib.fn))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}
	// Close sandbox holes OpenBase leaves: dofile/loadfile read host files; print
	// writes to os.Stdout. Remove string.rep — a single Go-level rep(s, 1e9) call
	// allocates past the context deadline (it only fences between VM ops) → OOM
	// (cross-AI review BLOCK/HIGH).
	for _, g := range []string{"dofile", "loadfile", "print"} {
		L.SetGlobal(g, lua.LNil)
	}
	if strTbl, ok := L.GetGlobal("string").(*lua.LTable); ok {
		strTbl.RawSetString("rep", lua.LNil)
	}
	// Deterministic-but-not-fixed RNG: seed from the input bag so a run replays
	// identically (same input → same sequence) yet different interactions differ
	// (a constant seed made every run produce the same "random" — review HIGH).
	if mathTbl, ok := L.GetGlobal("math").(*lua.LTable); ok {
		seed := int64(1)
		if b, mErr := json.Marshal(ctx.Vars()); mErr == nil {
			h := fnv.New64a()
			_, _ = h.Write(b)
			seed = int64(h.Sum64())
		}
		rng := mrand.New(mrand.NewSource(seed))
		mathTbl.RawSetString("random", L.NewFunction(func(l *lua.LState) int {
			switch l.GetTop() {
			case 0:
				l.Push(lua.LNumber(rng.Float64()))
			case 1:
				l.Push(lua.LNumber(rng.Intn(l.CheckInt(1)) + 1))
			default:
				a, b := l.CheckInt(1), l.CheckInt(2)
				l.Push(lua.LNumber(rng.Intn(b-a+1) + a))
			}
			return 1
		}))
		mathTbl.RawSetString("randomseed", L.NewFunction(func(*lua.LState) int { return 0 }))
	}

	L.SetGlobal("vars", goToLua(L, ctx.Vars()))

	fn, lerr := L.LoadString(cfg.Code)
	if lerr != nil {
		return StepResult{}, fmt.Errorf("script: load: %w", lerr)
	}
	L.Push(fn)
	if perr := L.PCall(0, 1, nil); perr != nil {
		return StepResult{Failure: &RoutingFailure{Code: FailInvalidGraph, Message: "script error: " + perr.Error()}}, nil
	}
	result := luaToGo(L.Get(-1))
	L.Pop(1)

	out := map[string]any{"result": result}
	if cfg.SaveAs != "" {
		ctx.SetVar(cfg.SaveAs, result)
		out["save_as"] = cfg.SaveAs
	}
	ctx.Emit(string(step.Kind), out)
	return StepResult{Output: out}, nil
}

// goToLua converts a JSON-shaped Go value (the bag's value space) into a Lua
// value. Objects become string-keyed tables; arrays become 1-based tables.
func goToLua(L *lua.LState, v any) lua.LValue {
	switch t := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(t)
	case float64:
		return lua.LNumber(t)
	case int:
		return lua.LNumber(float64(t))
	case string:
		return lua.LString(t)
	case map[string]any:
		tbl := L.NewTable()
		for k, e := range t {
			tbl.RawSetString(k, goToLua(L, e))
		}
		return tbl
	case []any:
		tbl := L.NewTable()
		for i, e := range t {
			tbl.RawSetInt(i+1, goToLua(L, e))
		}
		return tbl
	default:
		// Fall back through JSON for any other shape (e.g. json.Number).
		b, err := json.Marshal(t)
		if err != nil {
			return lua.LNil
		}
		var any2 any
		if json.Unmarshal(b, &any2) != nil {
			return lua.LString(string(b))
		}
		return goToLua(L, any2)
	}
}

// luaToGo converts a Lua return value back to the bag's value space. A table with
// contiguous 1..n integer keys becomes a []any; otherwise a map[string]any.
// Bounded by maxLuaDepth so a cyclic/over-nested table can't overflow the stack.
func luaToGo(v lua.LValue) any { return luaToGoDepth(v, 0) }

func luaToGoDepth(v lua.LValue, depth int) any {
	if depth > maxLuaDepth {
		return nil
	}
	switch t := v.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(t)
	case lua.LNumber:
		return float64(t)
	case lua.LString:
		return string(t)
	case *lua.LTable:
		n := t.Len()
		isArray := n > 0
		count := 0
		t.ForEach(func(k, _ lua.LValue) {
			count++
			if _, ok := k.(lua.LNumber); !ok {
				isArray = false
			}
		})
		if isArray && count == n {
			arr := make([]any, 0, n)
			for i := 1; i <= n; i++ {
				arr = append(arr, luaToGoDepth(t.RawGetInt(i), depth+1))
			}
			return arr
		}
		m := make(map[string]any)
		t.ForEach(func(k, val lua.LValue) {
			m[k.String()] = luaToGoDepth(val, depth+1)
		})
		return m
	default:
		return nil
	}
}
