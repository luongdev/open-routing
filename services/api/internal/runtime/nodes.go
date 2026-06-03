package runtime

import (
	"context"
	"fmt"
)

// stubNode is the Layer 1 placeholder implementation of Node: a real Descriptor
// (so the UI palette is complete now) with Validate/Compile/Execute stubbed.
// Layer 3 replaces each kind with a purpose-built type; the registry seam and
// the contract do not change when it does.
type stubNode struct {
	desc Descriptor
}

func (s stubNode) Kind() NodeKind         { return s.desc.Kind }
func (s stubNode) Descriptor() Descriptor { return s.desc }

func (s stubNode) Validate(_ context.Context, _ GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	return nil, nil
}

func (s stubNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	return PlanStep{NodeID: n.ID, Kind: s.desc.Kind}, nil
}

func (s stubNode) Execute(_ ExecCtx, _ PlanStep) (StepResult, error) {
	return StepResult{}, fmt.Errorf("runtime: node %q execute not implemented (Layer 3)", s.desc.Kind)
}

// defaultDescriptors is the v0.2 palette, in V02NodeKinds order. The flow
// builder renders directly from this, so adding a kind here (plus its entry in
// V02NodeKinds) is all it takes to surface a new node in the UI.
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
}

// DefaultRegistry returns a registry populated with the v0.2 node subset.
// Construct once per process (cmd/runtime) or per simulation (cmd/api).
func DefaultRegistry() *Registry {
	r := NewRegistry()
	for _, d := range defaultDescriptors {
		r.Register(stubNode{desc: d})
	}
	return r
}
