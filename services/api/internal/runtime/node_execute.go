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

func (waitNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[waitConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
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
