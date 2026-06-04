package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// baseNode supplies the kind/descriptor plumbing and a default Execute that
// reports "not implemented (Wave 3)". The 12 concrete node types embed it and
// override Validate/Compile; Execute bodies land with the runtime executor.
type baseNode struct{ desc Descriptor }

func (b baseNode) Kind() NodeKind         { return b.desc.Kind }
func (b baseNode) Descriptor() Descriptor { return b.desc }

func (b baseNode) Execute(_ ExecCtx, _ PlanStep) (StepResult, error) {
	return StepResult{}, fmt.Errorf("runtime: node %q execute not implemented (Wave 3)", b.desc.Kind)
}

// defaultDescriptors is the v0.2 palette, in V02NodeKinds order. The flow
// builder renders directly from this, so adding a kind here (plus its entry in
// V02NodeKinds and a registration below) is all it takes to surface a new node.
var defaultDescriptors = []Descriptor{
	{Kind: NodeTrigger, Title: "Trigger", Category: "entry", Summary: "Flow entry point for a route request."},
	{Kind: NodeIfElse, Title: "If / Else", Category: "branch", Summary: "Two-way conditional branch."},
	{Kind: NodeSwitchCase, Title: "Switch", Category: "branch", Summary: "Multi-way branch on a value."},
	{Kind: NodeWait, Title: "Wait", Category: "control", Summary: "Pause for a duration via a durable continuation."},
	{Kind: NodeMatchSkill, Title: "Match Skill", Category: "routing", Summary: "Filter candidates by required skill and proficiency."},
	{Kind: NodeFilter, Title: "Filter", Category: "routing", Summary: "Filter candidates by a predicate."},
	{Kind: NodeRouteQueue, Title: "Route to Queue", Category: "routing", Summary: "Select a queue's candidate pool."},
	{Kind: NodeReservation, Title: "Reservation", Category: "routing", Summary: "Offer the interaction to a selected agent."},
	{Kind: NodeFallback, Title: "Fallback", Category: "control", Summary: "Path taken when routing fails or times out."},
	{Kind: NodeEffect, Title: "Effect", Category: "action", Summary: "Record/mock side effect (no live external call in v0.2)."},
	{Kind: NodeLog, Title: "Log", Category: "action", Summary: "Emit a trace log entry."},
	{Kind: NodeEnd, Title: "End", Category: "exit", Summary: "Terminate the flow."},
	{Kind: NodeSetVar, Title: "Set Variable", Category: "data", Summary: "Assign an expression result to a variable."},
	{Kind: NodeCompute, Title: "Compute", Category: "data", Summary: "Evaluate an expression (optionally into a variable)."},
	{Kind: NodeLoopFor, Title: "For Each", Category: "control", Summary: "Iterate a body region over an array."},
	{Kind: NodeLoopWhile, Title: "While", Category: "control", Summary: "Repeat a body region while a condition holds."},
	{Kind: NodeParallel, Title: "Parallel", Category: "control", Summary: "Run branch regions (deterministic fan-out / join)."},
	{Kind: NodeTryCatch, Title: "Try / Catch", Category: "control", Summary: "Run a body region under an error boundary."},
	// 3D-3: record/mock side effects. v0.2 records the intent on the trace and
	// continues — no live external call (see node_side_effects.go).
	{Kind: NodeTTSSpeak, Title: "TTS Speak", Category: "voice", Summary: "Synthesize speech to the caller (record/mock in v0.2)."},
	{Kind: NodePlayPrompt, Title: "Play Prompt", Category: "voice", Summary: "Play a pre-recorded audio prompt (record/mock in v0.2)."},
	{Kind: NodeTransferCall, Title: "Transfer Call", Category: "voice", Summary: "Bridge the call to an external destination (record/mock)."},
	{Kind: NodeHangup, Title: "Hangup", Category: "voice", Summary: "Record a hangup intent (wire to an end node)."},
	{Kind: NodeSendMessage, Title: "Send Message", Category: "chat", Summary: "Send an outbound chat message (record/mock)."},
	{Kind: NodeQuickReplies, Title: "Quick Replies", Category: "chat", Summary: "Offer quick-reply chips (record/mock)."},
	{Kind: NodeTypingIndic, Title: "Typing Indicator", Category: "chat", Summary: "Show a typing indicator (record/mock)."},
	{Kind: NodeAttachFile, Title: "Attach File", Category: "chat", Summary: "Send a file to the customer (record/mock)."},
	{Kind: NodeBotHandoff, Title: "Bot Handoff", Category: "chat", Summary: "Escalate from bot to a human agent (record/mock)."},
	{Kind: NodeSendTemplate, Title: "Send Template", Category: "email", Summary: "Render + send a templated email (record/mock)."},
	{Kind: NodeHTTPRequest, Title: "HTTP Request", Category: "integration", Summary: "Outbound HTTP request (record/mock in v0.2)."},
	{Kind: NodeWebhook, Title: "Webhook", Category: "integration", Summary: "Fire-and-forget outbound webhook (record/mock)."},
	{Kind: NodeSetAgentState, Title: "Set Agent State", Category: "routing", Summary: "Record an agent presence-change intent."},
	{Kind: NodeWrapupTimer, Title: "Wrap-up Timer", Category: "routing", Summary: "Record a server-owned ACW countdown intent."},
	// 3D-4: interactive input. Pause for a value, store it, branch on it
	// (see node_input.go). Sim resolves from a scripted value; live times out.
	{Kind: NodeGetDTMF, Title: "Get DTMF", Category: "input", Summary: "Capture IVR digits (captured / timeout)."},
	{Kind: NodePromptText, Title: "Prompt Text", Category: "input", Summary: "Capture free text (captured / timeout)."},
	{Kind: NodeWaitSignal, Title: "Wait for Signal", Category: "input", Summary: "Pause until an external signal (received / timeout)."},
	{Kind: NodeManualApproval, Title: "Manual Approval", Category: "input", Summary: "Human gate (approved / rejected / timeout)."},
	{Kind: NodeDetectSpeech, Title: "Detect Speech", Category: "input", Summary: "Recognize spoken intent (recognized / no_match / timeout)."},
	{Kind: NodeCSATSurvey, Title: "CSAT Survey", Category: "input", Summary: "1-5 satisfaction rating (done / timeout)."},
	{Kind: NodeNPSSurvey, Title: "NPS Survey", Category: "input", Summary: "0-10 Net Promoter Score (done / timeout)."},
	// 3D-5: sandboxed Lua script over the var bag.
	{Kind: NodeScript, Title: "Script", Category: "data", Summary: "Sandboxed Lua over the variable bag (no IO; result → save_as)."},
}

var descriptorByKind = func() map[NodeKind]Descriptor {
	m := make(map[NodeKind]Descriptor, len(defaultDescriptors))
	for _, d := range defaultDescriptors {
		m[d.Kind] = d
	}
	return m
}()

func base(k NodeKind) baseNode { return baseNode{desc: descriptorByKind[k]} }

// DefaultRegistry returns a registry populated with the v0.2 node subset.
// Construct once per process (cmd/runtime) or per simulation/validation (cmd/api).
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(triggerNode{base(NodeTrigger)})
	r.Register(ifElseNode{base(NodeIfElse)})
	r.Register(switchCaseNode{base(NodeSwitchCase)})
	r.Register(waitNode{base(NodeWait)})
	r.Register(matchSkillNode{base(NodeMatchSkill)})
	r.Register(filterNode{base(NodeFilter)})
	r.Register(routeQueueNode{base(NodeRouteQueue)})
	r.Register(reservationNode{base(NodeReservation)})
	r.Register(fallbackNode{base(NodeFallback)})
	r.Register(effectNode{base(NodeEffect)})
	r.Register(logNode{base(NodeLog)})
	r.Register(endNode{base(NodeEnd)})
	r.Register(setVarNode{base(NodeSetVar)})
	r.Register(computeNode{base(NodeCompute)})
	r.Register(loopForNode{base(NodeLoopFor)})
	r.Register(loopWhileNode{base(NodeLoopWhile)})
	r.Register(parallelNode{base(NodeParallel)})
	r.Register(tryCatchNode{base(NodeTryCatch)})
	// 3D-3 side effects: one type, registered per kind in sideEffectKinds order
	// (must match the V02NodeKinds tail so the registry-order test holds).
	for _, k := range sideEffectKinds {
		r.Register(sideEffectNode{base(k)})
	}
	// 3D-4 interactive input: one type per kind, in inputKinds order.
	for _, k := range inputKinds {
		r.Register(inputNode{base(k)})
	}
	// 3D-5 sandboxed script.
	r.Register(scriptNode{base(NodeScript)})
	return r
}

// decodeConfig parses a node's opaque config into a typed struct. An absent
// config decodes to the zero value (each node decides whether that is valid).
// Unknown fields are tolerated for forward-compat — a node only validates the
// fields it knows.
func decodeConfig[T any](raw json.RawMessage) (T, error) {
	var cfg T
	if len(bytes.TrimSpace(raw)) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// compileConfig re-marshals a node's normalized config into the PlanStep. It
// round-trips through the typed struct so the compiled plan stores a canonical
// shape, not the raw author bytes.
func compileConfig[T any](n GraphNode, cfg T) (PlanStep, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return PlanStep{}, fmt.Errorf("runtime: compile %q: %w", n.Kind, err)
	}
	return PlanStep{NodeID: n.ID, Kind: n.Kind, Compiled: b}, nil
}

// fieldIssue is a node/field-level validation issue helper.
func fieldIssue(nodeID, field, code, msg string) ValidationIssue {
	return ValidationIssue{Code: code, Message: msg, NodeID: nodeID, Field: field}
}
