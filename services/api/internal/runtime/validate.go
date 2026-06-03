package runtime

import (
	"context"
	"fmt"
)

// Graph-level validation issue codes (node/field-level codes live in
// node_kinds.go). The UI surfaces these verbatim, so they are stable.
const (
	IssueEmptyGraph       = "empty_graph"
	IssueDuplicateNodeID  = "duplicate_node_id"
	IssueNoTrigger        = "no_trigger"
	IssueMultipleTriggers = "multiple_triggers"
	IssueUnknownKind      = "unknown_node_kind"
	IssueDanglingEdge     = "dangling_edge"
	IssueUnreachableNode  = "unreachable_node"
	IssueNoTerminal       = "no_terminal"
	IssueNoPathToTerminal = "no_path_to_terminal"
)

// ValidateGraph runs structural checks over the authoring graph, then each
// node's own Validate. refs may be nil for structure-only validation (unit
// tests / no DB); reference checks are skipped in that case. The returned error
// is reserved for infrastructure failures inside a node's Validate (e.g. a DB
// read) — graph/field problems come back as issues, not errors.
//
// Issues are emitted in a deterministic order (graph-level, then per-node in
// graph order, then per-edge in graph order) so tests and the UI see stable
// output.
func ValidateGraph(ctx context.Context, g *Graph, reg *Registry, refs CatalogRefs) ([]ValidationIssue, error) {
	if g == nil || len(g.Nodes) == 0 {
		return []ValidationIssue{{Code: IssueEmptyGraph, Message: "graph has no nodes"}}, nil
	}

	var issues []ValidationIssue

	seen := make(map[string]bool, len(g.Nodes))
	triggers := 0
	hasEnd := false
	for _, n := range g.Nodes {
		if seen[n.ID] {
			issues = append(issues, ValidationIssue{Code: IssueDuplicateNodeID, Message: fmt.Sprintf("duplicate node id %q", n.ID), NodeID: n.ID})
			continue
		}
		seen[n.ID] = true
		switch n.Kind {
		case NodeTrigger:
			triggers++
		case NodeEnd:
			hasEnd = true
		}
	}

	switch {
	case triggers == 0:
		issues = append(issues, ValidationIssue{Code: IssueNoTrigger, Message: "graph has no trigger (entry) node"})
	case triggers > 1:
		issues = append(issues, ValidationIssue{Code: IssueMultipleTriggers, Message: fmt.Sprintf("graph has %d trigger nodes; exactly one is allowed", triggers)})
	}
	if !hasEnd {
		issues = append(issues, ValidationIssue{Code: IssueNoTerminal, Message: "graph has no end node"})
	}

	// Per-node: unknown kind, then the node's own Validate.
	for _, n := range g.Nodes {
		node, ok := reg.Lookup(n.Kind)
		if !ok {
			issues = append(issues, ValidationIssue{Code: IssueUnknownKind, Message: fmt.Sprintf("unknown node kind %q", n.Kind), NodeID: n.ID})
			continue
		}
		nodeIssues, err := node.Validate(ctx, n, g, refs)
		if err != nil {
			return nil, fmt.Errorf("runtime: validate node %q (%s): %w", n.ID, n.Kind, err)
		}
		issues = append(issues, nodeIssues...)
	}

	// Edges must reference existing nodes.
	for _, e := range g.Edges {
		if !seen[e.From] {
			issues = append(issues, ValidationIssue{Code: IssueDanglingEdge, Message: fmt.Sprintf("edge source %q is not a node", e.From), EdgeID: e.ID})
		}
		if !seen[e.To] {
			issues = append(issues, ValidationIssue{Code: IssueDanglingEdge, Message: fmt.Sprintf("edge target %q is not a node", e.To), EdgeID: e.ID})
		}
	}

	// Reachability from the single trigger. Only meaningful with exactly one
	// trigger and no dangling edges polluting the adjacency; skip otherwise so
	// we don't pile redundant errors onto an already-broken graph.
	if triggers == 1 && !hasDanglingEdge(issues) {
		entry := singleTrigger(g)
		reachable := reachableFrom(entry, g.Edges)
		canReachEnd := reverseReachableFromEnds(g)
		for _, n := range g.Nodes {
			if n.Kind == NodeTrigger {
				continue
			}
			switch {
			case !reachable[n.ID]:
				issues = append(issues, ValidationIssue{Code: IssueUnreachableNode, Message: fmt.Sprintf("node %q is not reachable from the trigger", n.ID), NodeID: n.ID})
			case n.Kind != NodeEnd && !canReachEnd[n.ID]:
				// Reachable but no path forward to an end — a dead-end sink or a
				// cycle with no exit. The bare `end`-exists check misses this.
				issues = append(issues, ValidationIssue{Code: IssueNoPathToTerminal, Message: fmt.Sprintf("node %q cannot reach an end node", n.ID), NodeID: n.ID})
			}
		}
	}

	return issues, nil
}

func hasDanglingEdge(issues []ValidationIssue) bool {
	for _, i := range issues {
		if i.Code == IssueDanglingEdge {
			return true
		}
	}
	return false
}

func singleTrigger(g *Graph) string {
	for _, n := range g.Nodes {
		if n.Kind == NodeTrigger {
			return n.ID
		}
	}
	return ""
}

// reverseReachableFromEnds returns the set of nodes that have at least one
// directed path to some end node (reverse BFS from every end over reversed
// edges). A reachable node absent from this set is a dead-end/cycle that can
// never terminate.
func reverseReachableFromEnds(g *Graph) map[string]bool {
	radj := make(map[string][]string)
	for _, e := range g.Edges {
		radj[e.To] = append(radj[e.To], e.From)
	}
	reached := map[string]bool{}
	var queue []string
	for _, n := range g.Nodes {
		if n.Kind == NodeEnd {
			reached[n.ID] = true
			queue = append(queue, n.ID)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, from := range radj[cur] {
			if !reached[from] {
				reached[from] = true
				queue = append(queue, from)
			}
		}
	}
	return reached
}

func reachableFrom(entry string, edges []GraphEdge) map[string]bool {
	adj := make(map[string][]string)
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	reached := map[string]bool{entry: true}
	queue := []string{entry}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, to := range adj[cur] {
			if !reached[to] {
				reached[to] = true
				queue = append(queue, to)
			}
		}
	}
	return reached
}
