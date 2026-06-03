package runtime

import (
	"context"
	"fmt"
)

// PlanFormatVersion is the compiled-plan schema version. It is stamped into
// every CompiledPlan and persisted on flow_versions.plan_format_version so the
// runtime can refuse (or migrate) a plan compiled by an incompatible version.
// Bump it on any breaking change to PlanStep / CompiledPlan / CompiledEdge.
const PlanFormatVersion = 1

// CompiledEdge is one resolved edge of the executable plan. Port is the source
// node's output label (e.g. "true"/"false" on if_else); empty for a single
// linear out.
type CompiledEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Port string `json:"port,omitempty"`
}

// CompiledPlan is the deterministic executable form persisted on a published
// flow_version. Entry is the trigger node id; Steps are the compiled nodes in
// graph order; Edges is the routing topology the executor follows.
type CompiledPlan struct {
	FormatVersion int            `json:"format_version"`
	Entry         string         `json:"entry"`
	Steps         []PlanStep     `json:"steps"`
	Edges         []CompiledEdge `json:"edges"`
}

// Compile turns a graph into an executable plan. The caller MUST run
// ValidateGraph first (and confirm no issues) — Compile assumes a structurally
// valid graph and fails hard on anything it cannot compile rather than emitting
// validation issues.
func Compile(g *Graph, reg *Registry) (CompiledPlan, error) {
	if g == nil || len(g.Nodes) == 0 {
		return CompiledPlan{}, fmt.Errorf("runtime: compile empty graph")
	}

	entry := singleTrigger(g)
	if entry == "" {
		return CompiledPlan{}, fmt.Errorf("runtime: compile graph without a trigger")
	}

	steps := make([]PlanStep, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		node, ok := reg.Lookup(n.Kind)
		if !ok {
			return CompiledPlan{}, fmt.Errorf("runtime: compile unknown node kind %q (node %q)", n.Kind, n.ID)
		}
		step, err := node.Compile(n, g)
		if err != nil {
			return CompiledPlan{}, fmt.Errorf("runtime: compile node %q (%s): %w", n.ID, n.Kind, err)
		}
		steps = append(steps, step)
	}

	edges := make([]CompiledEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		edges = append(edges, CompiledEdge{From: e.From, To: e.To, Port: e.Label})
	}

	return CompiledPlan{
		FormatVersion: PlanFormatVersion,
		Entry:         entry,
		Steps:         steps,
		Edges:         edges,
	}, nil
}

// ValidateAndCompile is the publish-path convenience: validate, and only on a
// clean result compile. It returns the issues (empty == valid) so the caller
// can return 422 with the issues, or proceed to persist the plan.
func ValidateAndCompile(ctx context.Context, g *Graph, reg *Registry, refs CatalogRefs) ([]ValidationIssue, *CompiledPlan, error) {
	issues, err := ValidateGraph(ctx, g, reg, refs)
	if err != nil {
		return nil, nil, err
	}
	if len(issues) > 0 {
		return issues, nil, nil
	}
	plan, err := Compile(g, reg)
	if err != nil {
		return nil, nil, err
	}
	return nil, &plan, nil
}
