package runtime

import (
	"context"
	"fmt"
	"time"
)

// execState is the concrete ExecCtx for one run: a variable bag, the run's
// clock, and an event sink. The executor owns the clock directly (to Advance it
// on auto-resume); nodes see only the ExecCtx surface.
type execState struct {
	context.Context
	clock      Clock
	vars       map[string]any
	events     []EmittedEvent
	candidates []Candidate
	snapshot   *Snapshot
	driver     RoutingDriver
}

type EmittedEvent struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

func (s *execState) Now() time.Time           { return s.clock.Now() }
func (s *execState) Var(k string) (any, bool) { v, ok := s.vars[k]; return v, ok }
func (s *execState) SetVar(k string, v any)   { s.vars[k] = v }
func (s *execState) Emit(t string, p any) {
	s.events = append(s.events, EmittedEvent{Type: t, Payload: p})
}
func (s *execState) Candidates() []Candidate     { return s.candidates }
func (s *execState) SetCandidates(c []Candidate) { s.candidates = c }
func (s *execState) Snapshot() *Snapshot         { return s.snapshot }
func (s *execState) Reserve(agentID string, timeout time.Duration) ReservationOutcome {
	if s.driver == nil {
		return ResvRejected
	}
	return s.driver.Reserve(s.clock, agentID, timeout)
}

// TraceStep is one executed node's record: routing decision (Port), the node's
// reported Output, timing, and status (ok / failed / suspended).
type TraceStep struct {
	NodeID     string         `json:"node_id"`
	Kind       NodeKind       `json:"kind"`
	Status     string         `json:"status"`
	Port       string         `json:"port,omitempty"`
	DurationMs float64        `json:"duration_ms"`
	Output     map[string]any `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// Trace is the ordered step record plus the terminal outcome.
type Trace struct {
	Steps       []TraceStep `json:"steps"`
	Outcome     string      `json:"outcome"` // completed | failed | suspended
	FailureCode string      `json:"failure_code,omitempty"`
}

// RunResult bundles the trace with the final variable bag and emitted events.
// Suspension is set (and Trace.Outcome == "suspended") when a live run parks at
// a wait/reservation node instead of running to completion.
type RunResult struct {
	Trace      Trace
	Vars       map[string]any
	Events     []EmittedEvent
	Suspension *Suspension
}

// Executor walks a compiled plan. autoResume controls wait/suspension handling:
// in simulation it advances the (virtual) clock past the delay and continues;
// live it parks and returns the Suspension for the continuation worker.
type Executor struct {
	reg        *Registry
	autoResume bool
	maxSteps   int
	snapshot   *Snapshot
	driver     RoutingDriver
}

type ExecutorOption func(*Executor)

// WithAutoResume makes the executor advance the clock through wait/suspension
// nodes and keep going — used by the simulator. Live execution leaves it off so
// the continuation worker owns resumption. NOTE: this advances `wait` only; a
// reservation `timeout` advance is owned by the RoutingDriver, not here, so
// virtual time is never double-counted.
func WithAutoResume() ExecutorOption { return func(e *Executor) { e.autoResume = true } }

// WithRouting supplies the pinned snapshot and the reservation driver for a
// routing run (simulation now; live in 3c). Without it, routing nodes see an
// empty pool and a reject-only driver.
func WithRouting(snapshot *Snapshot, driver RoutingDriver) ExecutorOption {
	return func(e *Executor) { e.snapshot = snapshot; e.driver = driver }
}

// WithMaxSteps bounds the walk; the default guards against a cycle the
// validator did not (defensively) reject.
func WithMaxSteps(n int) ExecutorOption { return func(e *Executor) { e.maxSteps = n } }

func NewExecutor(reg *Registry, opts ...ExecutorOption) *Executor {
	ex := &Executor{reg: reg, maxSteps: 1000}
	for _, o := range opts {
		o(ex)
	}
	return ex
}

// Run executes plan against the given clock and initial variables. ctx carries
// cancellation; the returned RunResult always includes whatever trace was built
// before a terminal/failure/suspension/error.
func (ex *Executor) Run(ctx context.Context, clock Clock, plan CompiledPlan, input map[string]any) (RunResult, error) {
	vars := make(map[string]any, len(input))
	for k, v := range input {
		vars[k] = v
	}
	state := &execState{Context: ctx, clock: clock, vars: vars, snapshot: ex.snapshot, driver: ex.driver}

	stepByID := make(map[string]PlanStep, len(plan.Steps))
	for _, s := range plan.Steps {
		stepByID[s.NodeID] = s
	}
	edgesByFrom := make(map[string][]CompiledEdge)
	for _, e := range plan.Edges {
		edgesByFrom[e.From] = append(edgesByFrom[e.From], e)
	}

	res := RunResult{Vars: vars}
	cur := plan.Entry

	for i := 0; i < ex.maxSteps; i++ {
		if err := ctx.Err(); err != nil {
			return ex.finish(&res, state, "failed", "", err)
		}
		step, ok := stepByID[cur]
		if !ok {
			return ex.finish(&res, state, "failed", "", fmt.Errorf("runtime: plan has no step for node %q", cur))
		}
		node, ok := ex.reg.Lookup(step.Kind)
		if !ok {
			return ex.finish(&res, state, "failed", "", fmt.Errorf("runtime: no registered node for kind %q", step.Kind))
		}

		t0 := clock.Now()
		out, err := node.Execute(state, step)
		ts := TraceStep{
			NodeID:     cur,
			Kind:       step.Kind,
			Port:       out.Port,
			Output:     out.Output,
			DurationMs: float64(clock.Now().Sub(t0).Microseconds()) / 1000,
		}
		if err != nil {
			ts.Status, ts.Error = "failed", err.Error()
			res.Trace.Steps = append(res.Trace.Steps, ts)
			return ex.finish(&res, state, "failed", "", err)
		}
		if out.Failure != nil {
			ts.Status, ts.Error = "failed", out.Failure.Message
			res.Trace.Steps = append(res.Trace.Steps, ts)
			return ex.finish(&res, state, "failed", string(out.Failure.Code), nil)
		}
		if out.Suspension != nil && !ex.autoResume {
			ts.Status = "suspended"
			res.Trace.Steps = append(res.Trace.Steps, ts)
			res.Suspension = out.Suspension
			return ex.finish(&res, state, "suspended", "", nil)
		}
		if out.Suspension != nil {
			// Simulation: advance the virtual clock past the delay and continue.
			if d := out.Suspension.ResumeAt.Sub(clock.Now()); d > 0 {
				clock.Advance(d)
			}
		}
		ts.Status = "ok"
		res.Trace.Steps = append(res.Trace.Steps, ts)

		if out.Terminal {
			return ex.finish(&res, state, "completed", "", nil)
		}

		nextID, ok := resolveNext(cur, out, edgesByFrom)
		if !ok {
			// No out-edge and not terminal: the path ran off the end of the
			// graph. Treat as completion rather than error (validation already
			// requires an end node; this is the graceful tail).
			return ex.finish(&res, state, "completed", "", nil)
		}
		cur = nextID
	}
	return ex.finish(&res, state, "failed", "", fmt.Errorf("runtime: exceeded max steps (%d) — possible cycle", ex.maxSteps))
}

func (ex *Executor) finish(res *RunResult, state *execState, outcome, failureCode string, err error) (RunResult, error) {
	res.Trace.Outcome = outcome
	res.Trace.FailureCode = failureCode
	res.Events = state.events
	return *res, err
}

// resolveNext picks the successor node: an explicit Next wins; otherwise the
// out-edge whose Port matches out.Port; otherwise the sole out-edge when Port is
// empty. Ambiguous (multiple edges, no/unknown port) or dead-end returns false.
func resolveNext(from string, out StepResult, edgesByFrom map[string][]CompiledEdge) (string, bool) {
	if out.Next != "" {
		return out.Next, true
	}
	outs := edgesByFrom[from]
	if len(outs) == 0 {
		return "", false
	}
	if out.Port != "" {
		for _, e := range outs {
			if e.Port == out.Port {
				return e.To, true
			}
		}
		return "", false
	}
	if len(outs) == 1 {
		return outs[0].To, true
	}
	return "", false
}
