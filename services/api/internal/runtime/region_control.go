package runtime

import "reflect"

const defaultLoopMaxIter = 1000

// runControl dispatches a control node to its region-running logic, returning
// the output port to follow (done/catch) or a domain failure to bubble. A Go
// error is non-catchable (propagates to terminate the run).
func (r *runner) runControl(step PlanStep) (string, *RoutingFailure, error) {
	switch step.Kind {
	case NodeLoopFor:
		return r.runLoopFor(step)
	case NodeLoopWhile:
		return r.runLoopWhile(step)
	case NodeParallel:
		return r.runParallel(step)
	case NodeTryCatch:
		return r.runTryCatch(step)
	default:
		return "", nil, nil
	}
}

func (r *runner) runLoopFor(step PlanStep) (string, *RoutingFailure, error) {
	cfg, err := decodeConfig[loopForConfig](step.Compiled)
	if err != nil {
		return "", nil, err
	}
	val, err := evalValue(cfg.ArrayExpr, r.state) // expr-eval error → non-catchable
	if err != nil {
		return "", nil, err
	}
	var items []any
	switch v := val.(type) {
	case nil:
		items = nil // missing/nil → zero iterations
	case []any:
		items = v
	default:
		return "", &RoutingFailure{Code: FailInvalidLoopInput, Message: "loop_for array expression did not evaluate to an array"}, nil
	}
	maxIter := cfg.MaxIter
	if maxIter <= 0 {
		maxIter = defaultLoopMaxIter
	}
	if len(items) > maxIter {
		return "", &RoutingFailure{Code: FailLoopLimit, Message: "loop_for array exceeds max_iter"}, nil
	}
	itemVar, indexVar := cfg.ItemVar, cfg.IndexVar
	if itemVar == "" {
		itemVar = "item"
	}
	if indexVar == "" {
		indexVar = "index"
	}
	region := r.regionByID[step.NodeID]
	for i, item := range items {
		r.state.pushScope(map[string]any{itemVar: item, indexVar: i})
		save := r.iter
		ii := i
		r.iter = &ii
		wr, err := r.walk(region.ID, region.Entry)
		r.iter = save
		r.state.popScope()
		if err != nil {
			return "", nil, err
		}
		if wr.fail != nil {
			return "", wr.fail, nil // abort remaining iterations, bubble
		}
	}
	return "done", nil, nil
}

func (r *runner) runLoopWhile(step PlanStep) (string, *RoutingFailure, error) {
	cfg, err := decodeConfig[loopWhileConfig](step.Compiled)
	if err != nil {
		return "", nil, err
	}
	region := r.regionByID[step.NodeID]
	for iter := 0; ; iter++ {
		if iter >= cfg.MaxIter {
			return "", &RoutingFailure{Code: FailLoopLimit, Message: "loop_while exceeded max_iter"}, nil
		}
		cond, err := evalBool(cfg.CondExpr, r.state)
		if err != nil {
			return "", nil, err
		}
		if !cond {
			return "done", nil, nil
		}
		save := r.iter
		ii := iter
		r.iter = &ii
		wr, err := r.walk(region.ID, region.Entry)
		r.iter = save
		if err != nil {
			return "", nil, err
		}
		if wr.fail != nil {
			return "", wr.fail, nil
		}
	}
}

func (r *runner) runParallel(step PlanStep) (string, *RoutingFailure, error) {
	branches := r.branchesByOwner[step.NodeID]
	snapshot := cloneVars(r.state.vars)
	merged := cloneVars(snapshot)
	for _, br := range branches {
		// Each branch runs against a snapshot of the bag (isolation); writes are
		// merged in branch order (last-writer-wins) on join.
		r.state.vars = cloneVars(snapshot)
		save := r.branch
		bi := br.BranchIndex
		r.branch = &bi
		wr, err := r.walk(br.ID, br.Entry)
		r.branch = save
		if err != nil {
			r.state.vars = merged
			return "", nil, err
		}
		if wr.fail != nil {
			r.state.vars = merged
			return "", wr.fail, nil
		}
		for k, v := range r.state.vars {
			if !reflect.DeepEqual(snapshot[k], v) {
				merged[k] = v
			}
		}
	}
	r.state.vars = merged
	return "done", nil, nil
}

func (r *runner) runTryCatch(step PlanStep) (string, *RoutingFailure, error) {
	cfg, err := decodeConfig[tryCatchConfig](step.Compiled)
	if err != nil {
		return "", nil, err
	}
	region := r.regionByID[step.NodeID]
	wr, err := r.walk(region.ID, region.Entry)
	if err != nil {
		return "", nil, err // infra error — NOT caught
	}
	if wr.fail != nil {
		errorVar := cfg.ErrorVar
		if errorVar == "" {
			errorVar = "error"
		}
		r.state.SetVar(errorVar, map[string]any{"code": string(wr.fail.Code), "message": wr.fail.Message})
		// Mark the failing body step as caught (status stays "failed", but the
		// run continued via catch).
		for i := len(r.res.Trace.Steps) - 1; i >= 0; i-- {
			if r.res.Trace.Steps[i].Status == "failed" {
				r.res.Trace.Steps[i].Caught = true
				break
			}
		}
		return "catch", nil, nil
	}
	return "done", nil, nil
}
