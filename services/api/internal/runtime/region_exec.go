package runtime

import (
	"fmt"
	"sort"
	"time"
)

// runner executes a compiled plan with region awareness (3D-2). The top-level
// flow is walk("", entry); a control node dispatches to runControl, which runs
// its body region(s) as bounded recursive sub-walks. Flat (region-free) plans
// run through the same walk over top-level flow edges, so the prior behavior is
// preserved.
type runner struct {
	ex              *Executor
	state           *execState
	clock           Clock
	stepByID        map[string]PlanStep
	flowByFrom      map[string][]CompiledEdge   // FLOW edges only
	regionByID      map[string]CompiledRegion   // single-body region by owner/id
	branchesByOwner map[string][]CompiledRegion // parallel owner -> branches (by index)
	res             *RunResult
	steps           int
	iter            *int // current loop iteration, for the trace
	branch          *int // current parallel branch, for the trace
}

type walkResult struct {
	fail       *RoutingFailure // a domain failure bubbling up
	terminated bool            // hit a terminal end (top-level completion)
	suspended  *Suspension     // live suspension (autoResume off)
}

func newRunner(ex *Executor, state *execState, clock Clock, plan CompiledPlan, res *RunResult) *runner {
	r := &runner{
		ex: ex, state: state, clock: clock, res: res,
		stepByID:        make(map[string]PlanStep, len(plan.Steps)),
		flowByFrom:      make(map[string][]CompiledEdge),
		regionByID:      make(map[string]CompiledRegion),
		branchesByOwner: make(map[string][]CompiledRegion),
	}
	for _, s := range plan.Steps {
		r.stepByID[s.NodeID] = s
	}
	for _, e := range plan.Edges {
		if e.Kind == EdgeBody {
			continue // body edges are declarations, not runtime flow
		}
		r.flowByFrom[e.From] = append(r.flowByFrom[e.From], e)
	}
	for _, rg := range plan.Regions {
		r.regionByID[rg.ID] = rg
		r.branchesByOwner[rg.OwnerID] = append(r.branchesByOwner[rg.OwnerID], rg)
	}
	for owner := range r.branchesByOwner {
		brs := r.branchesByOwner[owner]
		sort.Slice(brs, func(i, j int) bool { return brs[i].BranchIndex < brs[j].BranchIndex })
	}
	return r
}

func (r *runner) record(step PlanStep, status, port, errMsg string, output map[string]any, durMs float64) int {
	ts := TraceStep{
		NodeID: step.NodeID, Kind: step.Kind, Status: status, Port: port,
		Output: output, Error: errMsg, DurationMs: durMs, Region: step.Region,
	}
	if r.iter != nil {
		i := *r.iter
		ts.Iteration = &i
	}
	if r.branch != nil {
		b := *r.branch
		ts.Branch = &b
	}
	r.res.Trace.Steps = append(r.res.Trace.Steps, ts)
	return len(r.res.Trace.Steps) - 1
}

// walk runs nodes within regionID from `entry`, following FLOW edges, until a
// node fails (bubbles), a terminal end (top-level done), a live suspension, or
// no in-region successor (region/level done).
func (r *runner) walk(regionID, entry string) (walkResult, error) {
	cur := entry
	for cur != "" {
		r.steps++
		if r.steps > r.ex.maxSteps {
			return walkResult{}, fmt.Errorf("runtime: exceeded max steps (%d) — possible cycle", r.ex.maxSteps)
		}
		if err := r.state.Err(); err != nil {
			return walkResult{}, err
		}
		step, ok := r.stepByID[cur]
		if !ok {
			return walkResult{}, fmt.Errorf("runtime: plan has no step for node %q", cur)
		}

		var port string
		var fail *RoutingFailure
		if ControlKinds[step.Kind] {
			// Record the control node first (placeholder), then run its body so
			// body steps appear AFTER it; backfill its port/status.
			idx := r.record(step, "ok", "", "", nil, 0)
			p, f, err := r.runControl(step)
			if err != nil {
				return walkResult{}, err
			}
			r.res.Trace.Steps[idx].Port = p
			if f != nil {
				r.res.Trace.Steps[idx].Status = "failed"
				r.res.Trace.Steps[idx].Error = f.Message
			}
			port, fail = p, f
		} else {
			out, susp, err := r.execNode(step)
			if err != nil {
				return walkResult{}, err
			}
			if susp != nil {
				return walkResult{suspended: susp}, nil
			}
			if out.Terminal {
				return walkResult{terminated: true}, nil
			}
			port, fail = out.Port, out.Failure
		}

		if fail != nil {
			return walkResult{fail: fail}, nil
		}
		next, ok := resolveNext(cur, StepResult{Port: port}, r.flowByFrom)
		if !ok {
			return walkResult{}, nil // region/level done
		}
		cur = next
	}
	return walkResult{}, nil
}

// execNode runs a non-control node and records its trace step. A returned
// *Suspension (autoResume off) signals the caller to park.
func (r *runner) execNode(step PlanStep) (StepResult, *Suspension, error) {
	node, ok := r.ex.reg.Lookup(step.Kind)
	if !ok {
		return StepResult{}, nil, fmt.Errorf("runtime: no registered node for kind %q", step.Kind)
	}
	t0 := time.Now() // wall-clock CPU time (NOT the virtual clock)
	out, err := node.Execute(r.state, step)
	dur := float64(time.Since(t0).Microseconds()) / 1000
	if err != nil {
		r.record(step, "failed", out.Port, err.Error(), out.Output, dur)
		return StepResult{}, nil, err
	}
	if out.Suspension != nil && !r.ex.autoResume {
		r.record(step, "suspended", out.Port, "", out.Output, dur)
		return StepResult{}, out.Suspension, nil
	}
	if out.Suspension != nil {
		if d := out.Suspension.ResumeAt.Sub(r.clock.Now()); d > 0 {
			r.clock.Advance(d)
		}
	}
	status := "ok"
	if out.Failure != nil {
		status = "failed"
	}
	errMsg := ""
	if out.Failure != nil {
		errMsg = out.Failure.Message
	}
	r.record(step, status, out.Port, errMsg, out.Output, dur)
	return out, nil, nil
}

func cloneVars(m map[string]any) map[string]any {
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
