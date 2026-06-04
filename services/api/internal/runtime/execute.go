package runtime

import (
	"context"
	"time"
)

// execState is the concrete ExecCtx for one run: a variable bag, the run's
// clock, and an event sink. The executor owns the clock directly (to Advance it
// on auto-resume); nodes see only the ExecCtx surface.
type execState struct {
	context.Context
	clock          Clock
	vars           map[string]any
	events         []EmittedEvent
	candidates     []Candidate
	snapshot       *Snapshot
	driver         RoutingDriver
	scopes         []map[string]any  // 3D-2 loop control-var scopes (innermost last)
	nodeOutcomes   map[string]string // per-node scripted reservation result port
	scriptedInputs map[string]any    // per-node captured input value (interactive-input nodes)
	// writes, when non-nil, records the keys SetVar touches. parallel turns this
	// on per branch so the merge keys actual WRITES (last-writer-wins) — a value
	// diff drops a branch that rewrites a key back to its snapshot value.
	writes map[string]bool
	// Live routing (Wave 3): offerer makes durable offers; resumeAt/resumeSignal
	// carry the inbound signal to the node a RunFrom re-enters at (one-shot).
	offerer      Offerer
	resumeAt     string
	resumeSignal string
}

type EmittedEvent struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

func (s *execState) Now() time.Time { return s.clock.Now() }
func (s *execState) Var(k string) (any, bool) {
	// Loop control scopes (item/index) shadow the root bag, innermost first.
	for i := len(s.scopes) - 1; i >= 0; i-- {
		if v, ok := s.scopes[i][k]; ok {
			return v, true
		}
	}
	v, ok := s.vars[k]
	return v, ok
}

// SetVar writes to the ROOT bag (not a loop scope) so accumulator patterns
// persist across iterations (3D-2 spec §4).
func (s *execState) SetVar(k string, v any) {
	s.vars[k] = v
	if s.writes != nil {
		s.writes[k] = true
	}
}

// Vars returns a merged snapshot: root bag overlaid by each loop scope
// (innermost last wins), matching Var's lookup order.
func (s *execState) Vars() map[string]any {
	out := make(map[string]any, len(s.vars))
	for k, v := range s.vars {
		out[k] = v
	}
	for _, scope := range s.scopes {
		for k, v := range scope {
			out[k] = v
		}
	}
	return out
}

func (s *execState) pushScope(m map[string]any) { s.scopes = append(s.scopes, m) }
func (s *execState) popScope()                  { s.scopes = s.scopes[:len(s.scopes)-1] }
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
func (s *execState) ScriptedOutcome(nodeID string) (string, bool) {
	p, ok := s.nodeOutcomes[nodeID]
	return p, ok
}
func (s *execState) ScriptedInput(nodeID string) (any, bool) {
	v, ok := s.scriptedInputs[nodeID]
	return v, ok
}
func (s *execState) LiveRouting() bool { return s.offerer != nil }
func (s *execState) Offer(agentID string, timeout time.Duration) (string, bool, error) {
	if s.offerer == nil {
		return "", false, nil
	}
	return s.offerer.Offer(agentID, timeout)
}
func (s *execState) ResumeSignal(nodeID string) (string, bool) {
	if s.resumeAt != "" && s.resumeAt == nodeID {
		sig := s.resumeSignal
		s.resumeAt, s.resumeSignal = "", "" // one-shot
		return sig, true
	}
	return "", false
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
	// 3D-2 region nesting (omitempty so flat traces are unchanged).
	Region    string `json:"region,omitempty"`    // owning region id
	Iteration *int   `json:"iteration,omitempty"` // loop iteration index
	Branch    *int   `json:"branch,omitempty"`    // parallel branch index
	Caught    bool   `json:"caught,omitempty"`    // a domain failure caught by try_catch
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
	// SuspendedNodeID is the node the run parked at (live routing); flowrt builds
	// the durable ResumeCursor from it + Vars.
	SuspendedNodeID string
}

// ResumeCursor is the durable execution position for a live route (persisted on
// route_requests.resume_cursor). v0.2 resumes only TOP-LEVEL wait/reservation
// nodes; region re-entry is out of scope.
type ResumeCursor struct {
	Version int            `json:"v"`
	NodeID  string         `json:"node_id"`
	Vars    map[string]any `json:"vars"`
}

// Executor walks a compiled plan. autoResume controls wait/suspension handling:
// in simulation it advances the (virtual) clock past the delay and continues;
// live it parks and returns the Suspension for the continuation worker.
type Executor struct {
	reg            *Registry
	autoResume     bool
	maxSteps       int
	snapshot       *Snapshot
	driver         RoutingDriver
	nodeOutcomes   map[string]string
	scriptedInputs map[string]any
	offerer        Offerer
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

// WithNodeOutcomes pins per-reservation-node result ports (node id → port).
func WithNodeOutcomes(m map[string]string) ExecutorOption {
	return func(e *Executor) { e.nodeOutcomes = m }
}

// WithScriptedInputs pins per-node captured input values (node id → value) so
// interactive-input nodes resolve their branch deterministically in simulation.
func WithScriptedInputs(m map[string]any) ExecutorOption {
	return func(e *Executor) { e.scriptedInputs = m }
}

// WithOfferer makes the run LIVE: reservation nodes offer the top candidate via
// the Offerer and SUSPEND, instead of the sim's synchronous offer loop.
func WithOfferer(o Offerer) ExecutorOption { return func(e *Executor) { e.offerer = o } }

// WithMaxSteps bounds the walk; the default guards against a cycle the
// validator did not (defensively) reject.
func WithMaxSteps(n int) ExecutorOption { return func(e *Executor) { e.maxSteps = n } }

// defaultMaxSteps is the cycle guard. It must sit well above defaultLoopMaxIter
// (1000): a single loop_for over a max-size array already executes 1000+ body
// steps, so a 1000-step cap would spuriously trip on a legal loop.
const defaultMaxSteps = 100000

func NewExecutor(reg *Registry, opts ...ExecutorOption) *Executor {
	ex := &Executor{reg: reg, maxSteps: defaultMaxSteps}
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
	state := &execState{Context: ctx, clock: clock, vars: vars, snapshot: ex.snapshot, driver: ex.driver, nodeOutcomes: ex.nodeOutcomes, scriptedInputs: ex.scriptedInputs, offerer: ex.offerer}
	return ex.drive(state, clock, plan, plan.Entry)
}

// RunFrom resumes a parked live route at cursor.NodeID with the agent action /
// timer signal (accepted/rejected/timeout; "" for a plain wait resume). The node
// it re-enters reads the signal via ResumeSignal and continues instead of
// re-suspending. v0.2 supports TOP-LEVEL wait/reservation cursors only.
// candidates seeds the working pool for a reservation resume: the walk re-enters
// AT the reservation node, so route_queue/match_skill do not re-run — flowrt
// supplies the freshly re-read eligible agents (excluding those already offered
// on this route). nil for a plain wait resume.
func (ex *Executor) RunFrom(ctx context.Context, clock Clock, plan CompiledPlan, cur ResumeCursor, signal string, candidates []Candidate) (RunResult, error) {
	vars := make(map[string]any, len(cur.Vars))
	for k, v := range cur.Vars {
		vars[k] = v
	}
	state := &execState{Context: ctx, clock: clock, vars: vars, candidates: candidates, snapshot: ex.snapshot, driver: ex.driver, nodeOutcomes: ex.nodeOutcomes, scriptedInputs: ex.scriptedInputs, offerer: ex.offerer, resumeAt: cur.NodeID, resumeSignal: signal}
	return ex.drive(state, clock, plan, cur.NodeID)
}

// RunResume re-runs a live flow from its ENTRY, but with the parked node
// (resumeNodeID) primed to consume `signal` (accepted/rejected/timeout) instead
// of re-suspending. Re-running from the top lets route_queue/match_skill rebuild
// the ranked candidate pool a reservation re-offer needs (the offerer excludes
// already-offered agents), and a wait whose timer fired continues past. Pure
// read nodes re-run harmlessly; v0.2 has no live side-effects before the park.
func (ex *Executor) RunResume(ctx context.Context, clock Clock, plan CompiledPlan, resumeNodeID, signal string, vars map[string]any) (RunResult, error) {
	v := make(map[string]any, len(vars))
	for k, val := range vars {
		v[k] = val
	}
	state := &execState{Context: ctx, clock: clock, vars: v, snapshot: ex.snapshot, driver: ex.driver, nodeOutcomes: ex.nodeOutcomes, scriptedInputs: ex.scriptedInputs, offerer: ex.offerer, resumeAt: resumeNodeID, resumeSignal: signal}
	return ex.drive(state, clock, plan, plan.Entry)
}

func (ex *Executor) drive(state *execState, clock Clock, plan CompiledPlan, entry string) (RunResult, error) {
	res := RunResult{Vars: state.vars}
	// The region runner walks top-level flow (regionID "") and recurses into
	// control-node body regions. A flat (region-free) plan walks the same way.
	rt := newRunner(ex, state, clock, plan, &res)
	wr, err := rt.walk("", entry)
	switch {
	case err != nil:
		return ex.finish(&res, state, "failed", "", err)
	case wr.suspended != nil:
		res.Suspension = wr.suspended
		res.SuspendedNodeID = rt.suspendedNodeID
		return ex.finish(&res, state, "suspended", "", nil)
	case wr.fail != nil: // an uncaught domain failure at top level
		return ex.finish(&res, state, "failed", string(wr.fail.Code), nil)
	default: // terminal end or ran off the end of the graph
		return ex.finish(&res, state, "completed", "", nil)
	}
}

func (ex *Executor) finish(res *RunResult, state *execState, outcome, failureCode string, err error) (RunResult, error) {
	res.Trace.Outcome = outcome
	res.Trace.FailureCode = failureCode
	res.Events = state.events
	// parallel reassigns state.vars to the merged bag, so the map Run captured at
	// the start is stale — resync from the live state.
	res.Vars = state.vars
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
