package runtime

import (
	"fmt"
	"time"
)

const defaultLoopMaxIter = 1000

// runControl dispatches a control node to its region-running logic. It returns
// the output port to follow (done/catch) on normal completion, or a walkResult
// carrying a bubbling outcome (terminated / suspended / domain failure) that the
// caller must propagate. A Go error is non-catchable (terminates the run). idx
// is the control node's own trace index, used to attribute a control-ORIGIN
// failure (loop_limit / invalid_loop_input) to the node itself.
func (r *runner) runControl(step PlanStep, idx int) (string, walkResult, error) {
	switch step.Kind {
	case NodeLoopFor:
		return r.runLoopFor(step, idx)
	case NodeLoopWhile:
		return r.runLoopWhile(step, idx)
	case NodeParallel:
		return r.runParallel(step)
	case NodeTryCatch:
		return r.runTryCatch(step)
	default:
		return "", walkResult{}, nil
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

func (r *runner) runLoopFor(step PlanStep, idx int) (string, walkResult, error) {
	cfg, err := decodeConfig[loopForConfig](step.Compiled)
	if err != nil {
		return "", walkResult{}, err
	}
	region, err := r.bodyRegion(step)
	if err != nil {
		return "", walkResult{}, err
	}
	val, err := evalValue(cfg.ArrayExpr, r.state) // expr-eval error → non-catchable
	if err != nil {
		return "", walkResult{}, err
	}
	var items []any
	switch v := val.(type) {
	case nil:
		items = nil // missing/nil → zero iterations
	case []any:
		items = v
	default:
		r.failIndex = idx // control-origin failure: the loop node itself
		return "", walkResult{fail: &RoutingFailure{Code: FailInvalidLoopInput, Message: "loop_for array expression did not evaluate to an array"}}, nil
	}
	maxIter := cfg.MaxIter
	if maxIter <= 0 {
		maxIter = defaultLoopMaxIter
	}
	if len(items) > maxIter {
		r.failIndex = idx
		return "", walkResult{fail: &RoutingFailure{Code: FailLoopLimit, Message: "loop_for array exceeds max_iter"}}, nil
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
			return "", walkResult{}, err
		}
		// terminated / suspended / fail all abort remaining iterations and bubble
		// (failIndex was set by the failing leaf, not overwritten here).
		if wr.terminated || wr.suspended != nil || wr.fail != nil {
			return "", wr, nil
		}
	}
	return "done", walkResult{}, nil
}

func (r *runner) runLoopWhile(step PlanStep, idx int) (string, walkResult, error) {
	cfg, err := decodeConfig[loopWhileConfig](step.Compiled)
	if err != nil {
		return "", walkResult{}, err
	}
	region, err := r.bodyRegion(step)
	if err != nil {
		return "", walkResult{}, err
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
			return "", walkResult{}, err
		}
		if !cond {
			return "done", walkResult{}, nil
		}
		if iter >= maxIter {
			r.failIndex = idx
			return "", walkResult{fail: &RoutingFailure{Code: FailLoopLimit, Message: "loop_while exceeded max_iter"}}, nil
		}
		save := r.iter
		ii := iter
		r.iter = &ii
		wr, err := r.walk(region.ID, region.Entry)
		r.iter = save
		if err != nil {
			return "", walkResult{}, err
		}
		if wr.terminated || wr.suspended != nil || wr.fail != nil {
			return "", wr, nil
		}
	}
}

func (r *runner) runParallel(step PlanStep) (string, walkResult, error) {
	branches := r.branchesByOwner[step.NodeID]
	if len(branches) == 0 {
		return "", walkResult{}, fmt.Errorf("runtime: parallel node %q has no branch regions", step.NodeID)
	}
	snapshot := cloneVars(r.state.vars)
	merged := cloneVars(snapshot)
	saveCands := r.state.candidates
	saveWrites := r.state.writes
	wroteUnion := map[string]bool{}
	// Branches are concurrent: each runs on its OWN clock starting at the
	// parallel's entry time (a VirtualClock only moves forward, so the shared
	// clock cannot be rewound between branches), and the join advances the
	// original clock once to the LONGEST branch — not the summed delays.
	origClock := r.clock
	t0 := origClock.Now()
	var maxAdvance time.Duration

	restore := func() {
		r.state.vars = merged
		r.state.candidates = saveCands
		r.state.writes = saveWrites
		r.clock = origClock
		r.state.clock = origClock
		origClock.Advance(maxAdvance)
	}

	for _, br := range branches {
		// Each branch runs against an isolated deep copy of the bag + candidate
		// pool + clock; only keys it actually WRITES (tracked, not value-diffed)
		// merge back, last-writer-wins in branch order.
		r.state.vars = cloneVars(snapshot)
		r.state.candidates = append([]Candidate(nil), saveCands...)
		r.state.writes = map[string]bool{}
		bc := NewVirtualClock(t0)
		r.clock = bc
		r.state.clock = bc
		eventsLen := len(r.state.events)
		save := r.branch
		bi := br.BranchIndex
		r.branch = &bi
		wr, err := r.walk(br.ID, br.Entry)
		r.branch = save
		branchWrites := r.state.writes
		if err != nil {
			restore()
			return "", walkResult{}, err
		}
		if wr.fail != nil {
			// A failed (catchable) branch is rolled back — its emitted effects
			// must not persist as if committed.
			r.state.events = r.state.events[:eventsLen]
			restore()
			return "", wr, nil
		}
		if wr.terminated || wr.suspended != nil {
			restore()
			return "", wr, nil
		}
		if adv := bc.Now().Sub(t0); adv > maxAdvance {
			maxAdvance = adv
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
	return "done", walkResult{}, nil
}

func (r *runner) runTryCatch(step PlanStep) (string, walkResult, error) {
	cfg, err := decodeConfig[tryCatchConfig](step.Compiled)
	if err != nil {
		return "", walkResult{}, err
	}
	region, err := r.bodyRegion(step)
	if err != nil {
		return "", walkResult{}, err
	}
	wr, err := r.walk(region.ID, region.Entry)
	if err != nil {
		return "", walkResult{}, err // infra error — NOT caught
	}
	// A terminal end or a parked wait is not a catchable failure — propagate.
	if wr.terminated || wr.suspended != nil {
		return "", wr, nil
	}
	if wr.fail != nil {
		errorVar := cfg.ErrorVar
		if errorVar == "" {
			errorVar = "error"
		}
		r.state.SetVar(errorVar, map[string]any{"code": string(wr.fail.Code), "message": wr.fail.Message})
		// Mark the exact failing step (tracked during the walk — the bubbling leaf,
		// not the control node that relayed it).
		if r.failIndex >= 0 && r.failIndex < len(r.res.Trace.Steps) {
			r.res.Trace.Steps[r.failIndex].Caught = true
		}
		return "catch", walkResult{}, nil
	}
	return "done", walkResult{}, nil
}
