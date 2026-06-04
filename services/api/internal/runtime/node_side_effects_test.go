package runtime

import (
	"context"
	"testing"
)

// validateSE runs a single side-effect node's Validate with nil refs (record/mock
// nodes have no catalog references) and returns the issue codes.
func validateSE(t *testing.T, kind NodeKind, cfg any) []string {
	t.Helper()
	n, ok := DefaultRegistry().Lookup(kind)
	if !ok {
		t.Fatalf("kind %q not registered", kind)
	}
	issues, err := n.Validate(context.Background(), node("x", kind, cfg), nil, nil)
	if err != nil {
		t.Fatalf("validate %q: %v", kind, err)
	}
	codes := make([]string, len(issues))
	for i, is := range issues {
		codes[i] = is.Code
	}
	return codes
}

func hasCode(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

func TestSideEffect_RequiredField(t *testing.T) {
	if codes := validateSE(t, NodeSendMessage, map[string]any{}); !hasCode(codes, IssueMissingField) {
		t.Fatalf("send_message without text: codes=%v, want missing_required_field", codes)
	}
	if codes := validateSE(t, NodeSendMessage, map[string]any{"text": "hi"}); len(codes) != 0 {
		t.Fatalf("send_message with text: codes=%v, want none", codes)
	}
	// Blank/whitespace counts as absent.
	if codes := validateSE(t, NodeTTSSpeak, map[string]any{"text": "   "}); !hasCode(codes, IssueMissingField) {
		t.Fatalf("tts_speak blank text: codes=%v, want missing_required_field", codes)
	}
}

func TestSideEffect_EnumAndPositiveNum(t *testing.T) {
	if codes := validateSE(t, NodeHTTPRequest, map[string]any{"url": "https://x", "method": "FETCH"}); !hasCode(codes, IssueInvalidConfig) {
		t.Fatalf("http_request bad method: codes=%v, want invalid_config", codes)
	}
	if codes := validateSE(t, NodeHTTPRequest, map[string]any{"url": "https://x", "method": "POST"}); len(codes) != 0 {
		t.Fatalf("http_request good method: codes=%v, want none", codes)
	}
	if codes := validateSE(t, NodeSetAgentState, map[string]any{"state": "Banana"}); !hasCode(codes, IssueInvalidConfig) {
		t.Fatalf("set_agent_state bad state: codes=%v, want invalid_config", codes)
	}
	if codes := validateSE(t, NodeWrapupTimer, map[string]any{"duration_sec": 0}); !hasCode(codes, IssueInvalidConfig) {
		t.Fatalf("wrapup_timer zero duration: codes=%v, want invalid_config", codes)
	}
	if codes := validateSE(t, NodeWrapupTimer, map[string]any{"duration_sec": 30}); len(codes) != 0 {
		t.Fatalf("wrapup_timer positive duration: codes=%v, want none", codes)
	}
}

func TestSideEffect_BadInterpolationExpr(t *testing.T) {
	// A closed-but-malformed ${...} expression is caught at validate time.
	if codes := validateSE(t, NodeSendMessage, map[string]any{"text": "Hi ${1 +}"}); !hasCode(codes, IssueInvalidExpr) {
		t.Fatalf("send_message bad interp: codes=%v, want invalid_expression", codes)
	}
	// An unclosed ${ is harmless literal text, not an error.
	if codes := validateSE(t, NodeSendMessage, map[string]any{"text": "cost is ${100"}); len(codes) != 0 {
		t.Fatalf("send_message unclosed brace: codes=%v, want none (literal)", codes)
	}
	// A well-formed one validates clean.
	if codes := validateSE(t, NodeSendMessage, map[string]any{"text": "Hi ${customer.name}"}); len(codes) != 0 {
		t.Fatalf("send_message good interp: codes=%v, want none", codes)
	}
}

// ${expr} placeholders in string fields are resolved against the var bag and the
// resolved value is what the node records on the trace.
func TestSideEffect_InterpolatesAtExecute(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("h", NodeHTTPRequest, map[string]any{
				"method": "POST",
				"url":    "https://api/users/${customer.id}",
				"body":   `{"tier":"${customer.tier}","n":${count}}`,
			}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{{From: "t", To: "h"}, {From: "h", To: "end"}},
	}
	if issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil); err != nil || len(issues) != 0 {
		t.Fatalf("graph invalid: err=%v issues=%+v", err, issues)
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	res, err := NewExecutor(DefaultRegistry()).Run(context.Background(), RealClock{}, plan,
		map[string]any{"customer": map[string]any{"id": "c7", "tier": "gold"}, "count": 3})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var httpStep *TraceStep
	for i := range res.Trace.Steps {
		if res.Trace.Steps[i].NodeID == "h" {
			httpStep = &res.Trace.Steps[i]
		}
	}
	if httpStep == nil {
		t.Fatal("no http step in trace")
	}
	if got := httpStep.Output["url"]; got != "https://api/users/c7" {
		t.Fatalf("url = %v, want https://api/users/c7", got)
	}
	if got := httpStep.Output["body"]; got != `{"tier":"gold","n":3}` {
		t.Fatalf("body = %v, want interpolated JSON with gold/3", got)
	}
}

// http_request stores its mock response into save_as; a downstream compute then
// navigates the captured value — array index, nested object, and arr.len.
func TestSideEffect_CaptureResponseAndNavigate(t *testing.T) {
	mock := `{"items":[{"id":"a1","name":"Alice"},{"id":"b2","name":"Bob"}],"page":{"total":2}}`
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("h", NodeHTTPRequest, map[string]any{"method": "GET", "url": "https://api/x", "save_as": "resp", "mock_response": mock}),
			node("c1", NodeCompute, map[string]any{"expr": "resp.items.0.name", "var": "firstName"}),
			node("c2", NodeCompute, map[string]any{"expr": "arr.len(resp.items)", "var": "n"}),
			node("c3", NodeCompute, map[string]any{"expr": "resp.page.total", "var": "total"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "h"}, {From: "h", To: "c1"}, {From: "c1", To: "c2"}, {From: "c2", To: "c3"}, {From: "c3", To: "end"},
		},
	}
	if issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil); err != nil || len(issues) != 0 {
		t.Fatalf("graph invalid: err=%v issues=%+v", err, issues)
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	res, err := NewExecutor(DefaultRegistry()).Run(context.Background(), RealClock{}, plan, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Trace.Outcome != "completed" {
		t.Fatalf("outcome = %q", res.Trace.Outcome)
	}
	if got := res.Vars["firstName"]; got != "Alice" {
		t.Fatalf("resp.items.0.name = %v, want Alice", got)
	}
	if got, _ := toFloatField(res.Vars["n"]); got != 2 {
		t.Fatalf("arr.len(resp.items) = %v, want 2", res.Vars["n"])
	}
	if got, _ := toFloatField(res.Vars["total"]); got != 2 {
		t.Fatalf("resp.page.total = %v, want 2", res.Vars["total"])
	}
}

func TestSideEffect_CaptureValidation(t *testing.T) {
	if codes := validateSE(t, NodeHTTPRequest, map[string]any{"url": "https://x", "save_as": "1bad"}); !hasCode(codes, IssueInvalidConfig) {
		t.Fatalf("bad save_as: codes=%v, want invalid_config", codes)
	}
	if codes := validateSE(t, NodeHTTPRequest, map[string]any{"url": "https://x", "save_as": "resp", "mock_response": "{not json"}); !hasCode(codes, IssueInvalidConfig) {
		t.Fatalf("bad mock_response JSON: codes=%v, want invalid_config", codes)
	}
}

// A flow using side-effect nodes runs to completion, recording each as a trace
// step (record/mock — no suspension, single done edge).
func TestSideEffect_RecordsAndContinues(t *testing.T) {
	g := &Graph{
		Nodes: []GraphNode{
			node("t", NodeTrigger, nil),
			node("msg", NodeSendMessage, map[string]any{"text": "hello"}),
			node("tts", NodeTTSSpeak, map[string]any{"text": "welcome"}),
			node("end", NodeEnd, nil),
		},
		Edges: []GraphEdge{
			{From: "t", To: "msg"},
			{From: "msg", To: "tts"},
			{From: "tts", To: "end"},
		},
	}
	if issues, err := ValidateGraph(context.Background(), g, DefaultRegistry(), nil); err != nil || len(issues) != 0 {
		t.Fatalf("graph invalid: err=%v issues=%+v", err, issues)
	}
	plan, err := Compile(g, DefaultRegistry())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	res, err := NewExecutor(DefaultRegistry()).Run(context.Background(), RealClock{}, plan, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Trace.Outcome != "completed" {
		t.Fatalf("outcome = %q, want completed", res.Trace.Outcome)
	}
	if got := stepIDs(res.Trace); !eqStrings(got, []string{"t", "msg", "tts", "end"}) {
		t.Fatalf("path = %v, want [t msg tts end]", got)
	}
}
