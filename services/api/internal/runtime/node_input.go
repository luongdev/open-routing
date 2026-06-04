package runtime

// node_input.go — the v0.2 interactive-input node family: nodes that pause for
// an inbound value (IVR digits, free text, an external signal, a human approval,
// recognized speech, a survey score), store it into a variable, and branch on it.
//
// One execution shape, three resolutions:
//   - SIM: the simulator pins a captured value per node id (scripted_effect_outputs
//     → ScriptedInput). The node stores it and takes the branch the value implies.
//   - SIM, no pinned value: the node SUSPENDS; auto-resume fast-forwards the clock
//     and takes the timeout branch (deterministic "nobody answered").
//   - LIVE: the node SUSPENDS (parks a continuation due at the timeout). v0.2 has
//     no submit-value API, so the worker's timer fires and it takes the timeout
//     branch; a submit-value resume that injects the captured value is v0.3.

import (
	"context"
	"strings"
	"time"

	"github.com/luongdev/open-routing/services/api/internal/runtime/expr"
)

// inputSpec declares an input node's branch ports and how a captured value maps
// to one. timeoutPort is taken when the wait elapses with no value.
type inputSpec struct {
	defaultVar  string
	timeoutPort string
	// portFor maps a captured value to its branch port. Total over any value.
	portFor func(val any) string
}

const defaultInputTimeoutSec = 30

var inputSpecs = map[NodeKind]inputSpec{
	NodeGetDTMF:    {defaultVar: "digits", timeoutPort: "timeout", portFor: func(any) string { return "captured" }},
	NodePromptText: {defaultVar: "text", timeoutPort: "timeout", portFor: func(any) string { return "captured" }},
	NodeWaitSignal: {defaultVar: "signal", timeoutPort: "timeout", portFor: func(any) string { return "received" }},
	NodeManualApproval: {defaultVar: "decision", timeoutPort: "timeout", portFor: func(v any) string {
		// Anything other than an explicit approval reads as a rejection.
		if s, ok := v.(string); ok && (s == "approved" || s == "approve" || s == "true") {
			return "approved"
		}
		if b, ok := v.(bool); ok && b {
			return "approved"
		}
		return "rejected"
	}},
	NodeDetectSpeech: {defaultVar: "speech", timeoutPort: "timeout", portFor: func(v any) string {
		if isEmptyField(v) {
			return "no_match"
		}
		return "recognized"
	}},
	NodeCSATSurvey: {defaultVar: "csat", timeoutPort: "timeout", portFor: func(any) string { return "done" }},
	NodeNPSSurvey:  {defaultVar: "nps", timeoutPort: "timeout", portFor: func(any) string { return "done" }},
}

// inputKinds is the family's registration/V02NodeKinds order.
var inputKinds = []NodeKind{
	NodeGetDTMF, NodePromptText, NodeWaitSignal, NodeManualApproval, NodeDetectSpeech,
	NodeCSATSurvey, NodeNPSSurvey,
}

type inputConfig struct {
	TimeoutSec int    `json:"timeout_sec,omitempty"`
	SaveAs     string `json:"save_as,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
}

type inputNode struct{ baseNode }

func (inputNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[inputConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	var issues []ValidationIssue
	if cfg.TimeoutSec < 0 {
		issues = append(issues, fieldIssue(n.ID, "timeout_sec", IssueInvalidConfig, "timeout_sec cannot be negative"))
	}
	if cfg.SaveAs != "" && !validVarName(cfg.SaveAs) {
		issues = append(issues, fieldIssue(n.ID, "save_as", IssueInvalidConfig, "save_as must be an identifier (optionally dotted) and not a reserved word"))
	}
	issues = append(issues, checkInterpExprs(n.ID, "prompt", cfg.Prompt)...)
	return issues, nil
}

func (inputNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[inputConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

func (s inputNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeConfig[inputConfig](step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	spec := inputSpecs[s.desc.Kind]
	saveAs := cfg.SaveAs
	if saveAs == "" {
		saveAs = spec.defaultVar
	}

	// SIM (or a future live submit): a captured value was pinned → store + branch.
	if val, ok := ctx.ScriptedInput(step.NodeID); ok {
		ctx.SetVar(saveAs, val)
		port := spec.portFor(val)
		ctx.Emit(string(step.Kind), map[string]any{"captured": val, "save_as": saveAs, "port": port})
		return StepResult{Port: port, Output: map[string]any{"captured": val, "save_as": saveAs, "port": port}}, nil
	}

	// LIVE resume: the worker's timer fired (no value submitted in v0.2) → timeout.
	if _, resuming := ctx.ResumeSignal(step.NodeID); resuming {
		return StepResult{Port: spec.timeoutPort, Output: map[string]any{"port": spec.timeoutPort, "timed_out": true}}, nil
	}

	// First encounter, no value: SUSPEND until the timeout. Live parks a
	// continuation; sim auto-resume fast-forwards and takes the timeout Port.
	timeoutSec := cfg.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = defaultInputTimeoutSec
	}
	return StepResult{
		Suspension: &Suspension{ResumeAt: ctx.Now().Add(time.Duration(timeoutSec) * time.Second)},
		Port:       spec.timeoutPort,
		Output:     map[string]any{"awaiting": string(s.desc.Kind), "save_as": saveAs, "prompt": interpolateStr(cfg.Prompt, ctx)},
	}, nil
}

// checkInterpExprs validates ${expr} placeholders embedded in a string field at
// publish time (same rule as the side-effect family).
func checkInterpExprs(nodeID, field, s string) []ValidationIssue {
	var issues []ValidationIssue
	for _, m := range interpExprRe.FindAllStringSubmatch(s, -1) {
		inner := strings.TrimSpace(m[1])
		if inner == "" {
			continue
		}
		if node, perr := expr.Parse(inner); perr != nil {
			issues = append(issues, fieldIssue(nodeID, field, IssueInvalidExpr, perr.Msg))
		} else if err := expr.Check(node); err != nil {
			issues = append(issues, fieldIssue(nodeID, field, IssueInvalidExpr, err.Error()))
		}
	}
	return issues
}
