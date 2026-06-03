package runtime

import (
	"strings"
	"time"

	"github.com/luongdev/open-routing/services/api/internal/runtime/expr"
)

// exprEnv adapts the runtime ExecCtx to the expression engine's Env: variable
// lookup with dotted-path resolution (a flat dotted key wins; otherwise the
// first segment is read via ExecCtx.Var and the rest walked through nested
// maps/arrays) plus the deterministic clock for date.*.
type exprEnv struct{ ctx ExecCtx }

func (e exprEnv) Now() time.Time { return e.ctx.Now() }

func (e exprEnv) Lookup(path string) (any, bool) {
	if v, ok := e.ctx.Var(path); ok {
		return v, true
	}
	segs := strings.Split(path, ".")
	cur, ok := e.ctx.Var(segs[0])
	if !ok {
		return nil, false
	}
	for _, seg := range segs[1:] {
		cur, ok = expr.Index(cur, seg)
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// evalBool / evalString are the runtime's entry points into the engine.
func evalBool(src string, ctx ExecCtx) (bool, error)     { return expr.EvalBool(src, exprEnv{ctx}) }
func evalString(src string, ctx ExecCtx) (string, error) { return expr.EvalString(src, exprEnv{ctx}) }
