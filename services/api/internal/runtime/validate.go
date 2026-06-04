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
	// 3D-2 region (control-flow) structural codes.
	IssueRegionBoundary = "region_boundary_crossing"
	IssueRegionEntry    = "region_entry_invalid"
	IssueRegionBranches = "region_branches_invalid"
	IssueEndInRegion    = "end_in_region"
	IssueLoopVarShadow  = "loop_var_shadow"
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

	// 3D-2 region structure (body edges, boundaries, branch shape, loop vars).
	issues = append(issues, validateRegions(g)...)

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
			case n.Region != "":
				// A region node exits by returning to its owner (no in-region
				// successor), not by reaching a top-level end — the region runner
				// owns its termination, so the global end-reachability check
				// doesn't apply.
			case n.Kind != NodeEnd && !canReachEnd[n.ID]:
				// Reachable but no path forward to an end — a dead-end sink or a
				// cycle with no exit. The bare `end`-exists check misses this.
				issues = append(issues, ValidationIssue{Code: IssueNoPathToTerminal, Message: fmt.Sprintf("node %q cannot reach an end node", n.ID), NodeID: n.ID})
			}
		}
	}

	return issues, nil
}

// validateRegions enforces the 3D-2 region contract: body edges declare a child
// region whose entry must be a member; flow edges may not cross a region
// boundary; `end` is top-level only; parallel branch indices are contiguous; and
// a set_var/compute may not shadow an enclosing loop's item/index var.
func validateRegions(g *Graph) []ValidationIssue {
	var issues []ValidationIssue
	regionOf := make(map[string]string, len(g.Nodes))
	kindOf := make(map[string]NodeKind, len(g.Nodes))
	for _, n := range g.Nodes {
		regionOf[n.ID] = n.Region
		kindOf[n.ID] = n.Kind
		if n.Kind == NodeEnd && n.Region != "" {
			issues = append(issues, ValidationIssue{Code: IssueEndInRegion, Message: fmt.Sprintf("end node %q cannot be inside a region", n.ID), NodeID: n.ID})
		}
	}

	ridsByOwner := make(map[string][]string)
	for _, e := range g.Edges {
		rid, _ := regionID(e.From, e.Label)
		if rid == "" {
			continue // a flow edge
		}
		ridsByOwner[e.From] = append(ridsByOwner[e.From], rid)
		if !ControlKinds[kindOf[e.From]] {
			issues = append(issues, ValidationIssue{Code: IssueRegionEntry, Message: fmt.Sprintf("node %q declares a body region but is not a control node", e.From), NodeID: e.From})
		}
		if regionOf[e.To] != rid {
			issues = append(issues, ValidationIssue{Code: IssueRegionEntry, Message: fmt.Sprintf("region %q entry %q is not a member of that region", rid, e.To), NodeID: e.To})
		}
	}

	for owner, rids := range ridsByOwner {
		switch kindOf[owner] {
		case NodeParallel:
			// branch ids must be owner#0..owner#N-1 (contiguous, no dup).
			want := make(map[string]bool, len(rids))
			for i := range rids {
				want[fmt.Sprintf("%s#%d", owner, i)] = true
			}
			ok := len(rids) > 0
			seenR := map[string]bool{}
			for _, r := range rids {
				if seenR[r] || !want[r] {
					ok = false
				}
				seenR[r] = true
			}
			if !ok {
				issues = append(issues, ValidationIssue{Code: IssueRegionBranches, Message: fmt.Sprintf("parallel %q branches must be body:0..body:%d (contiguous, unique)", owner, len(rids)-1), NodeID: owner})
			}
		default: // loop_for / loop_while / try_catch: exactly one body region == owner
			if len(rids) != 1 || rids[0] != owner {
				issues = append(issues, ValidationIssue{Code: IssueRegionBranches, Message: fmt.Sprintf("%s %q must declare exactly one body region (label \"body\")", kindOf[owner], owner), NodeID: owner})
			}
		}
	}

	for _, e := range g.Edges {
		if rid, _ := regionID(e.From, e.Label); rid != "" {
			continue // body edge — the only legal boundary crossing
		}
		if regionOf[e.From] != regionOf[e.To] {
			issues = append(issues, ValidationIssue{Code: IssueRegionBoundary, Message: fmt.Sprintf("flow edge %q→%q crosses a region boundary", e.From, e.To), EdgeID: e.ID})
		}
	}

	issues = append(issues, validateLoopVarShadow(g, regionOf)...)
	return issues
}

// validateLoopVarShadow rejects a set_var/compute whose target equals the
// item/index var of the loop that owns its region (direct membership; nested
// owners are a 3D-2 follow-on).
func validateLoopVarShadow(g *Graph, regionOf map[string]string) []ValidationIssue {
	loopVars := map[string]map[string]bool{} // regionID -> {item,index}
	for _, n := range g.Nodes {
		if n.Kind != NodeLoopFor {
			continue
		}
		cfg, err := decodeConfig[loopForConfig](n.Config)
		if err != nil {
			continue
		}
		item, index := cfg.ItemVar, cfg.IndexVar
		if item == "" {
			item = "item"
		}
		if index == "" {
			index = "index"
		}
		loopVars[n.ID] = map[string]bool{item: true, index: true}
	}
	var issues []ValidationIssue
	for _, n := range g.Nodes {
		vars := loopVars[regionOf[n.ID]]
		if vars == nil {
			continue
		}
		var name string
		switch n.Kind {
		case NodeSetVar:
			if cfg, err := decodeConfig[setVarConfig](n.Config); err == nil {
				name = cfg.Name
			}
		case NodeCompute:
			if cfg, err := decodeConfig[computeConfig](n.Config); err == nil {
				name = cfg.Var
			}
		}
		if name != "" && vars[name] {
			issues = append(issues, ValidationIssue{Code: IssueLoopVarShadow, Message: fmt.Sprintf("node %q writes %q, which shadows the enclosing loop's variable", n.ID, name), NodeID: n.ID})
		}
	}
	return issues
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
