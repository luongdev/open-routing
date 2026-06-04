package runtime

import "fmt"

const defaultLoopMaxIter = 1000

// runControl dispatches a control node to its region-running logic, returning
// the output port to follow (done/catch), a domain failure to bubble, or a live
// suspension parked inside the body. A Go error is non-catchable (propagates to
// terminate the run).
func (r *runner) runControl(step PlanStep) (string, *RoutingFailure, *Suspension, error) {
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
		return "", nil, nil, nil
	}
}

// bodyRegion returns the single body region a loop/try owns, erroring if the
// plan has none (validation should have rejected that — defense in depth).
func (r *runner) bodyRegion(step PlanStep) (CompiledRegion, error) {
	region, ok := r.regionByID[step.NodeID]
	if !ok {
		return CompiledRegion{}, fmt.Errorf("runtime: control node %q (%s) has no body region", step.NodeID, step.Kind)
	}
	return region, nil
}

func (r *runner) runLoopFor(step PlanStep) (string, *RoutingFailure, *Suspension, error) {
	cfg, err := decodeConfig[loopForConfig](step.Compiled)
	if err != nil {
		return "", nil, nil, err
	}
	region, err := r.bodyRegion(step)
	if err != nil {
		return "", nil, nil, err
	}
	val, err := evalValue(cfg.ArrayExpr, r.state) // expr-eval error → non-catchable
	if err != nil {
		return "", nil, nil, err
	}
	var items []any
	switch v := val.(type) {
	case nil:
		items = nil // missing/nil → zero iterations
	case []any:
		items = v
	default:
		return "", &RoutingFailure{Code: FailInvalidLoopInput, Message: "loop_for array expression did not evaluate to an array"}, nil, nil
	}
	maxIter := cfg.MaxIter
	if maxIter <= 0 {
		maxIter = defaultLoopMaxIter
	}
	if len(items) > maxIter {
		return "", &RoutingFailure{Code: FailLoopLimit, Message: "loop_for array exceeds max_iter"}, nil, nil
	}
	itemVar, indexVar := cfg.ItemVar, cfg.IndexVar
	if itemVar == "" {
		itemVar = "item"
	}
	if indexVar == "" {
		indexVar = "index"
	}
	for i, item := range items {
		r.state.pushScope(map[string]any{itemVar: item, indexVar: i})
		save := r.iter
		ii := i
		r.iter = &ii
		wr, err := r.walk(region.ID, region.Entry)
		r.iter = save
		r.state.popScope()
		if err != nil {
			return "", nil, nil, err
		}
		if wr.suspended != nil {
			return "", nil, wr.suspended, nil
		}
		if wr.fail != nil {
			return "", wr.fail, nil, nil // abort remaining iterations, bubble
		}
	}
	return "done", nil, nil, nil
}

func (r *runner) runLoopWhile(step PlanStep) (string, *RoutingFailure, *Suspension, error) {
	cfg, err := decodeConfig[loopWhileConfig](step.Compiled)
	if err != nil {
		return "", nil, nil, err
	}
	region, err := r.bodyRegion(step)
	if err != nil {
		return "", nil, nil, err
	}
	maxIter := cfg.MaxIter
	if maxIter <= 0 {
		maxIter = defaultLoopMaxIter
	}
	for iter := 0; ; iter++ {
		// Evaluate the condition BEFORE the limit check: a loop that terminates
		// naturally on its maxIter-th test must complete, not trip loop_limit.
		cond, err := evalBool(cfg.CondExpr, r.state)
		if err != nil {
			return "", nil, nil, err
		}
		if !cond {
			return "done", nil, nil, nil
		}
		if iter >= maxIter {
			return "", &RoutingFailure{Code: FailLoopLimit, Message: "loop_while exceeded max_iter"}, nil, nil
		}
		save := r.iter
		ii := iter
		r.iter = &ii
		wr, err := r.walk(region.ID, region.Entry)
		r.iter = save
		if err != nil {
			return "", nil, nil, err
		}
		if wr.suspended != nil {
			return "", nil, wr.suspended, nil
		}
		if wr.fail != nil {
			return "", wr.fail, nil, nil
		}
	}
}

func (r *runner) runParallel(step PlanStep) (string, *RoutingFailure, *Suspension, error) {
	branches := r.branchesByOwner[step.NodeID]
	if len(branches) == 0 {
		return "", nil, nil, fmt.Errorf("runtime: parallel node %q has no branch regions", step.NodeID)
	}
	snapshot := cloneVars(r.state.vars)
	merged := cloneVars(snapshot)
	saveCands := r.state.candidates
	saveWrites := r.state.writes
	wroteUnion := map[string]bool{}

	restore := func() {
		r.state.vars = merged
		r.state.candidates = saveCands
		r.state.writes = saveWrites
	}

	for _, br := range branches {
		// Each branch runs against an isolated deep copy of the bag + candidate
		// pool; only keys it actually WRITES (tracked, not value-diffed) are
		// merged back, last-writer-wins in branch order.
		r.state.vars = cloneVars(snapshot)
		r.state.candidates = append([]Candidate(nil), saveCands...)
		r.state.writes = map[string]bool{}
		save := r.branch
		bi := br.BranchIndex
		r.branch = &bi
		wr, err := r.walk(br.ID, br.Entry)
		r.branch = save
		branchWrites := r.state.writes
		if err != nil {
			restore()
			return "", nil, nil, err
		}
		if wr.suspended != nil {
			restore()
			return "", nil, wr.suspended, nil
		}
		if wr.fail != nil {
			restore()
			return "", wr.fail, nil, nil
		}
		for k := range branchWrites {
			merged[k] = deepClone(r.state.vars[k])
			wroteUnion[k] = true
		}
	}
	restore()
	// Propagate this region's writes outward so an enclosing parallel's merge
	// (nested parallel) also sees them.
	if r.state.writes != nil {
		for k := range wroteUnion {
			r.state.writes[k] = true
		}
	}
	return "done", nil, nil, nil
}

func (r *runner) runTryCatch(step PlanStep) (string, *RoutingFailure, *Suspension, error) {
	cfg, err := decodeConfig[tryCatchConfig](step.Compiled)
	if err != nil {
		return "", nil, nil, err
	}
	region, err := r.bodyRegion(step)
	if err != nil {
		return "", nil, nil, err
	}
	wr, err := r.walk(region.ID, region.Entry)
	if err != nil {
		return "", nil, nil, err // infra error — NOT caught
	}
	if wr.suspended != nil {
		return "", nil, wr.suspended, nil // a parked wait is not a catchable failure
	}
	if wr.fail != nil {
		errorVar := cfg.ErrorVar
		if errorVar == "" {
			errorVar = "error"
		}
		r.state.SetVar(errorVar, map[string]any{"code": string(wr.fail.Code), "message": wr.fail.Message})
		// Mark the exact failing step (tracked during the walk), not a backward
		// scan that could mark an earlier already-caught failure.
		if r.failIndex >= 0 && r.failIndex < len(r.res.Trace.Steps) {
			r.res.Trace.Steps[r.failIndex].Caught = true
		}
		return "catch", nil, nil, nil
	}
	return "done", nil, nil, nil
}
