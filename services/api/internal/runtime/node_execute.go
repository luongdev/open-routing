package runtime

import "time"

// node_execute.go holds the Execute bodies for the deterministic control/data
// nodes (Wave 3 part 1). The routing nodes — match_skill, route_queue, filter,
// reservation — keep the baseNode "not implemented" Execute until the candidate
// + reservation subsystem lands (Wave 3 part 2); their Validate/Compile are
// already real. step.Compiled is the node's own compiled config (written by
// Compile), so a decode error here is a programming bug, surfaced as a step
// error rather than a silent skip.

func omitEmpty(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	return m
}

func (triggerNode) Execute(_ ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[triggerConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	out := map[string]any{}
	if cfg.Channel != "" {
		out["channel"] = cfg.Channel
	}
	if cfg.EntryCode != "" {
		out["entry_code"] = cfg.EntryCode
	}
	return StepResult{Output: omitEmpty(out)}, nil
}

func (ifElseNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[ifElseConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	result, err := evalBool(cfg.Expr, ctx)
	if err != nil {
		return StepResult{}, err
	}
	port := "false"
	if result {
		port = "true"
	}
	return StepResult{Port: port, Output: map[string]any{"expr": cfg.Expr, "result": result}}, nil
}

func (switchCaseNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[switchCaseConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	val, err := evalString(cfg.Expr, ctx)
	if err != nil {
		return StepResult{}, err
	}
	port := "default"
	for _, c := range cfg.Cases {
		if c == val {
			port = val
			break
		}
	}
	return StepResult{Port: port, Output: map[string]any{"value": val}}, nil
}

func (filterNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[filterConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	// v0.2: filter is a predicate gate — evaluate and record the result; the
	// candidate-pool narrowing it informs lands with the assignment subsystem
	// (Wave 3 part 2). Routing is linear (the single out edge).
	passed, err := evalBool(cfg.Expr, ctx)
	if err != nil {
		return StepResult{}, err
	}
	return StepResult{Output: map[string]any{"expr": cfg.Expr, "passed": passed}}, nil
}

func (routeQueueNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[routeQueueConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	snap := ctx.Snapshot()
	if snap == nil {
		return StepResult{Failure: &RoutingFailure{Code: FailMissingCatalogReference, Message: "no snapshot for route_queue"}}, nil
	}
	pool, ok := snap.QueueCandidates[cfg.Queue]
	if !ok {
		return StepResult{Failure: &RoutingFailure{Code: FailMissingCatalogReference, Message: "queue " + cfg.Queue + " not in snapshot"}}, nil
	}
	ctx.SetCandidates(pool)
	return StepResult{Output: map[string]any{"queue": cfg.Queue, "candidates": len(pool)}}, nil
}

func (matchSkillNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[matchSkillConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	required := []RequiredSkill{{Code: cfg.Skill, MinProficiency: cfg.MinProficiency}}
	ranked := RankCandidates(ctx.Candidates(), required)
	ctx.SetCandidates(ranked)
	// An empty pool is NOT a terminal failure: it flows downstream to the
	// reservation node, which yields its `no_candidate` port (→ fallback). A
	// terminal failure here would make the no_candidate routing path unreachable
	// (cross-AI review BLOCK).
	return StepResult{Output: map[string]any{"skill": cfg.Skill, "min_proficiency": cfg.MinProficiency, "candidates": len(ranked)}}, nil
}

// reservationMaxAttempts clamps max_attempts: omitted/0 -> 1, capped at 10
// (validate already rejects <0 and >10; this is defense-in-depth at exec time).
func reservationMaxAttempts(n int) int {
	if n <= 0 {
		return 1
	}
	if n > 10 {
		return 10
	}
	return n
}

func (reservationNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[reservationConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	timeout := time.Duration(cfg.TimeoutSec) * time.Second

	// LIVE: offer the top candidate and SUSPEND; resume on the agent's action.
	if ctx.LiveRouting() {
		return liveReservation(ctx, step, timeout)
	}

	// A per-node scripted outcome pins this reservation's result port directly
	// (the simulator's per-node branch control) — skip the offer loop.
	if port, ok := ctx.ScriptedOutcome(step.NodeID); ok {
		return StepResult{Port: port, Output: map[string]any{"scripted": port}}, nil
	}
	maxAttempts := reservationMaxAttempts(cfg.MaxAttempts)
	// Working copy of the ranked pool; a rejected/timed-out candidate is removed
	// before the next offer (driver advances the clock on timeout).
	pool := append([]Candidate(nil), ctx.Candidates()...)
	attempts := 0
	lastTimedOut := false
	for attempts < maxAttempts && len(pool) > 0 {
		cand := pool[0]
		attempts++
		switch ctx.Reserve(cand.AgentID, timeout) {
		case ResvAccepted:
			return StepResult{Port: "accepted", Output: map[string]any{"agent_id": cand.AgentID, "attempts": attempts}}, nil
		case ResvTimeout:
			lastTimedOut = true
			pool = pool[1:]
		default: // rejected
			lastTimedOut = false
			pool = pool[1:]
		}
	}
	// Pool/attempts exhausted: timeout port if the last offer timed out (so the
	// flow can branch on "nobody answered in time"), else no_candidate.
	port := "no_candidate"
	if lastTimedOut {
		port = "timeout"
	}
	return StepResult{Port: port, Output: map[string]any{"attempts": attempts}}, nil
}

// liveReservation runs the reservation node in LIVE mode: an accept signal ends
// the offer (accepted port); otherwise it offers the top OFFERABLE candidate and
// SUSPENDS, or — when none can be offered — yields timeout/no_candidate. The
// candidate pool already excludes agents already offered on this route (flowrt
// rebuilds the segment snapshot each resume), so there is no per-attempt cursor.
func liveReservation(ctx ExecCtx, step PlanStep, timeout time.Duration) (StepResult, error) {
	sig, resuming := ctx.ResumeSignal(step.NodeID)
	if resuming && sig == "accepted" {
		return StepResult{Port: "accepted", Output: map[string]any{"outcome": "accepted"}}, nil
	}
	// The matcher (or, in the inline path, the SLA deadline) gives up by resuming
	// with no_candidate → fall through to the fallback path.
	if resuming && sig == "no_candidate" {
		return StepResult{Port: "no_candidate", Output: map[string]any{"outcome": "no_candidate"}}, nil
	}
	for _, c := range ctx.Candidates() {
		resID, ok, err := ctx.Offer(c.AgentID, timeout)
		if err != nil {
			return StepResult{}, err
		}
		if ok {
			return StepResult{
				Suspension: &Suspension{ResumeAt: ctx.Now().Add(timeout)},
				Output:     map[string]any{"offered_agent": c.AgentID, "reservation_id": resID},
			}, nil
		}
	}
	// No candidate available now. In MATCHER mode the route waits in the queue —
	// the matcher offers when an agent frees, the SLA sweep bounds the wait — so
	// park (waiting_match) instead of taking the no_candidate/timeout port.
	if ctx.MatcherMode() {
		return StepResult{
			Suspension: &Suspension{WaitForMatch: true},
			Output:     map[string]any{"queued": true},
		}, nil
	}
	// Inline (W3) / sim: a `timeout` resume means the prior offer elapsed
	// unanswered → distinguish "nobody answered" from "nobody eligible".
	port := "no_candidate"
	if resuming && sig == "timeout" {
		port = "timeout"
	}
	return StepResult{Port: port, Output: map[string]any{"attempts": 0}}, nil
}

func (waitNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[waitConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	// LIVE resume: the worker fired this wait's continuation (time elapsed) →
	// continue past it instead of re-suspending. (Sim auto-resumes the clock and
	// never sets a resume signal, so it still suspends-then-advances as before.)
	if _, resuming := ctx.ResumeSignal(step.NodeID); resuming {
		return StepResult{Output: map[string]any{"resumed": true}}, nil
	}
	resumeAt := ctx.Now().Add(time.Duration(cfg.DurationMs) * time.Millisecond)
	return StepResult{
		Suspension: &Suspension{ResumeAt: resumeAt},
		Output:     map[string]any{"duration_ms": cfg.DurationMs},
	}, nil
}

func (logNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[logConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	level := cfg.Level
	if level == "" {
		level = "info"
	}
	ctx.Emit("log", map[string]any{"level": level, "message": cfg.Message})
	return StepResult{Output: map[string]any{"level": level, "message": cfg.Message}}, nil
}

func (effectNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[effectConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	// v0.2 effects are record/mock only — no live external call. The trace
	// records the intent so simulation can show what *would* fire.
	ctx.Emit("effect", map[string]any{"adapter": cfg.Adapter, "action": cfg.Action, "mode": "recorded"})
	return StepResult{Output: map[string]any{"adapter": cfg.Adapter, "action": cfg.Action, "mode": "recorded"}}, nil
}

func (fallbackNode) Execute(_ ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[fallbackConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	out := map[string]any{}
	if cfg.Reason != "" {
		out["reason"] = cfg.Reason
	}
	return StepResult{Output: omitEmpty(out)}, nil
}

func (setVarNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[setVarConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	val, err := evalValue(cfg.ValueExpr, ctx)
	if err != nil {
		return StepResult{}, err
	}
	ctx.SetVar(cfg.Name, val)
	return StepResult{Output: map[string]any{"name": cfg.Name, "value": val}}, nil
}

func (computeNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[computeConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	val, err := evalValue(cfg.Expr, ctx)
	if err != nil {
		return StepResult{}, err
	}
	out := map[string]any{"result": val}
	if cfg.Var != "" {
		ctx.SetVar(cfg.Var, val)
		out["var"] = cfg.Var
	}
	return StepResult{Output: out}, nil
}

func (endNode) Execute(_ ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[endConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	out := map[string]any{}
	if cfg.Outcome != "" {
		out["outcome"] = cfg.Outcome
	}
	return StepResult{Terminal: true, Output: omitEmpty(out)}, nil
}
