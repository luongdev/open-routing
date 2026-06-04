package runtime

// node_side_effects.go — the v0.2 "record/mock" side-effect node family: channel
// output (voice/chat/email), outbound integration (http/webhook), and agent-state
// intents. v0.2 makes NO live external call (mirrors the effect node): each node
// records its intent on the trace and continues down its single `done` edge, so a
// flow that uses them stays deterministic and replayable. They share one
// implementation because they share one execution shape — record + continue —
// differing only in which config fields they accept (sideEffectSpecs).
//
// Interactive/input nodes (get_dtmf, prompt_text, manual_approval, …) are NOT
// here: they suspend for an inbound value, a different execution shape that needs
// resume-with-value plumbing (deferred). `script` needs a sandbox (deferred).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/luongdev/open-routing/services/api/internal/runtime/expr"
)

// sideEffectSpec declares a record/mock node's config contract. Title/Category/
// Summary live in defaultDescriptors (single palette source); this table is the
// validation rules only.
type sideEffectSpec struct {
	required    []string            // present + non-empty
	enums       map[string][]string // string field -> allowed values (checked only when present)
	positiveNum []string            // numeric fields that must be > 0 when present
}

// sideEffectSpecs is keyed by kind; the zero value (no rules) is valid for nodes
// whose every field is optional (typing_indicator, bot_handoff).
var sideEffectSpecs = map[NodeKind]sideEffectSpec{
	NodeTTSSpeak:     {required: []string{"text"}},
	NodePlayPrompt:   {required: []string{"prompt"}},
	NodeTransferCall: {required: []string{"destination"}},
	NodeHangup:       {},
	NodeSendMessage:  {required: []string{"text"}},
	NodeQuickReplies: {required: []string{"text"}},
	NodeTypingIndic:  {},
	NodeAttachFile:   {required: []string{"url"}},
	NodeBotHandoff:   {},
	NodeSendTemplate: {required: []string{"template"}},
	NodeHTTPRequest:  {required: []string{"url"}, enums: map[string][]string{"method": {"GET", "POST", "PUT", "PATCH", "DELETE"}}},
	NodeWebhook:      {required: []string{"url"}},
	NodeSetAgentState: {
		required: []string{"state"},
		enums:    map[string][]string{"state": {"Ready", "NotReady", "Break", "WrapUp", "Offline"}},
	},
	NodeWrapupTimer: {required: []string{"duration_sec"}, positiveNum: []string{"duration_sec"}},
}

// sideEffectKinds is the family's order — appended verbatim to V02NodeKinds and
// registered in this order so the registry-order test stays green.
var sideEffectKinds = []NodeKind{
	NodeTTSSpeak, NodePlayPrompt, NodeTransferCall, NodeHangup,
	NodeSendMessage, NodeQuickReplies, NodeTypingIndic, NodeAttachFile, NodeBotHandoff,
	NodeSendTemplate,
	NodeHTTPRequest, NodeWebhook,
	NodeSetAgentState, NodeWrapupTimer,
}

type sideEffectNode struct{ baseNode }

func (s sideEffectNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeSideEffectConfig(n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	spec := sideEffectSpecs[s.desc.Kind]
	var issues []ValidationIssue
	for _, f := range spec.required {
		if isEmptyField(cfg[f]) {
			issues = append(issues, fieldIssue(n.ID, f, IssueMissingField, fmt.Sprintf("%s requires %q", n.Kind, f)))
		}
	}
	for field, allowed := range spec.enums {
		v, present := cfg[field]
		if !present || isEmptyField(v) {
			continue
		}
		sv, _ := v.(string)
		if !containsStr(allowed, sv) {
			issues = append(issues, fieldIssue(n.ID, field, IssueInvalidConfig, fmt.Sprintf("%s must be one of %s", field, strings.Join(allowed, ", "))))
		}
	}
	for _, field := range spec.positiveNum {
		v, present := cfg[field]
		if !present {
			continue
		}
		if f, ok := toFloatField(v); !ok || f <= 0 {
			issues = append(issues, fieldIssue(n.ID, field, IssueInvalidConfig, fmt.Sprintf("%s must be a positive number", field)))
		}
	}
	// Response capture: save_as must be a usable variable name; a mock_response
	// with no ${...} must be valid JSON (with ${...} it can only be checked once
	// the vars are known, so we defer to the interpolation parse below).
	if saveAs, ok := cfg[seFieldSaveAs].(string); ok && strings.TrimSpace(saveAs) != "" && !validVarName(saveAs) {
		issues = append(issues, fieldIssue(n.ID, seFieldSaveAs, IssueInvalidConfig, "save_as must be an identifier (optionally dotted) and not a reserved word"))
	}
	if mr, ok := cfg[seFieldMockResponse].(string); ok && strings.TrimSpace(mr) != "" && !strings.Contains(mr, "${") {
		var v any
		if err := json.Unmarshal([]byte(mr), &v); err != nil {
			issues = append(issues, fieldIssue(n.ID, seFieldMockResponse, IssueInvalidConfig, "mock_response must be valid JSON"))
		}
	}
	// Any ${...} expression embedded in a string field is parsed at publish time
	// so a bad interpolation (e.g. ${customer.) is caught before it runs.
	for field, v := range cfg {
		if str, ok := v.(string); ok {
			for _, m := range interpExprRe.FindAllStringSubmatch(str, -1) {
				inner := strings.TrimSpace(m[1])
				if inner == "" {
					continue
				}
				if node, perr := expr.Parse(inner); perr != nil {
					issues = append(issues, fieldIssue(n.ID, field, IssueInvalidExpr, perr.Msg))
				} else if err := expr.Check(node); err != nil {
					issues = append(issues, fieldIssue(n.ID, field, IssueInvalidExpr, err.Error()))
				}
			}
		}
	}
	return issues, nil
}

func (sideEffectNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeSideEffectConfig(n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	// json.Marshal sorts map keys, so the compiled config is canonical regardless
	// of author key order.
	return compileConfig(n, cfg)
}

func (s sideEffectNode) Execute(ctx ExecCtx, step PlanStep) (StepResult, error) {
	cfg, err := decodeSideEffectConfig(step.Compiled)
	if err != nil {
		return StepResult{}, err
	}
	// Record the RESOLVED config: ${expr} placeholders in string fields are
	// interpolated against the var bag so the trace shows exactly what would be
	// sent (e.g. the request body with customer values substituted).
	out := map[string]any{"node": string(step.Kind), "mode": "recorded"}
	for k, v := range cfg {
		if k == seFieldSaveAs || k == seFieldMockResponse {
			continue // response-capture fields handled below, not echoed raw
		}
		if str, ok := v.(string); ok {
			out[k] = interpolateStr(str, ctx)
		} else {
			out[k] = v
		}
	}
	// Response capture: store the (mock, v0.2) response into save_as so downstream
	// nodes navigate it through the expr engine — nested objects via
	// resp.user.name, arrays via resp.items.0.id, length via arr.len(resp.items).
	if saveAs, _ := cfg[seFieldSaveAs].(string); strings.TrimSpace(saveAs) != "" {
		resp := parseMockResponse(cfg[seFieldMockResponse], ctx)
		ctx.SetVar(saveAs, resp)
		out["save_as"] = saveAs
		out["response"] = resp
	}
	ctx.Emit(string(step.Kind), out)
	return StepResult{Output: out}, nil
}

const (
	seFieldSaveAs       = "save_as"
	seFieldMockResponse = "mock_response"
)

// parseMockResponse turns the node's mock_response config into a navigable value:
// the string is interpolated (${var}) then JSON-decoded, so an object/array
// becomes map[string]any/[]any the expr engine can index. Non-JSON stays a plain
// string; absent/blank yields nil.
func parseMockResponse(raw any, ctx ExecCtx) any {
	str, ok := raw.(string)
	if !ok {
		return raw
	}
	s := strings.TrimSpace(interpolateStr(str, ctx))
	if s == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

// interpExprRe matches a ${expr} interpolation placeholder. Non-greedy body so
// adjacent placeholders in one string don't merge.
var interpExprRe = regexp.MustCompile(`\$\{([^}]*)\}`)

// interpolateStr replaces each ${expr} in s with the evaluated expression,
// stringified. An unparseable/erroring expr is left as the literal placeholder
// (Validate already rejects bad exprs at publish, so this is a runtime safety
// net, not the primary check).
func interpolateStr(s string, ctx ExecCtx) string {
	if !strings.Contains(s, "${") {
		return s
	}
	return interpExprRe.ReplaceAllStringFunc(s, func(m string) string {
		inner := strings.TrimSpace(m[2 : len(m)-1])
		if inner == "" {
			return m
		}
		v, err := evalValue(inner, ctx)
		if err != nil {
			return m
		}
		return stringifyVal(v)
	})
}

// stringifyVal renders an interpolated value: strings verbatim, whole floats
// without a trailing .0, bools as true/false, everything else as compact JSON.
func stringifyVal(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func decodeSideEffectConfig(raw json.RawMessage) (map[string]any, error) {
	cfg := map[string]any{}
	if len(bytes.TrimSpace(raw)) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// isEmptyField treats nil and a blank/whitespace string as "not provided"; any
// other value (number, bool, array, object) counts as present.
func isEmptyField(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// toFloatField coerces a JSON-decoded value to a float (config numbers decode to
// float64; json.Number tolerated if a decoder used UseNumber).
func toFloatField(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}
