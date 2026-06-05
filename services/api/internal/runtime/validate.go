package runtime

import (
	"context"
	"fmt"
	"strings"
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
	IssueInvalidNodeID  = "invalid_node_id"
	IssueSuspendInRegion = "suspend_in_region"
	IssuePortCardinality = "invalid_port_cardinality"
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
		// Node id must be non-empty and free of '#' — '#' collides with the region
		// owner#index naming, and an empty id breaks compile (review H10).
		if n.ID == "" || strings.Contains(n.ID, "#") {
			issues = append(issues, ValidationIssue{Code: IssueInvalidNodeID, Message: fmt.Sprintf("node id %q must be non-empty and must not contain '#'", n.ID), NodeID: n.ID})
			continue
		}
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

	// 3D hardening: suspending nodes are top-level only; branch ports must be sane.
	issues = append(issues, validateSuspendScope(g)...)
	issues = append(issues, validatePorts(g)...)

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

	// body edges declare child regions; collect branch indices per owner (in
	// edge order) + entry-membership / control-owner checks.
	branchesByOwner := make(map[string][]int)
	for _, e := range g.Edges {
		rid, branch := regionID(e.From, e.Label)
		if rid == "" {
			continue // a flow edge
		}
		branchesByOwner[e.From] = append(branchesByOwner[e.From], branch)
		if !ControlKinds[kindOf[e.From]] {
			issues = append(issues, ValidationIssue{Code: IssueRegionEntry, Message: fmt.Sprintf("node %q declares a body region but is not a control node", e.From), NodeID: e.From})
		}
		if regionOf[e.To] != rid {
			issues = append(issues, ValidationIssue{Code: IssueRegionEntry, Message: fmt.Sprintf("region %q entry %q is not a member of that region", rid, e.To), NodeID: e.To})
		}
	}

	// Validate EVERY control node in graph order (deterministic) — incl. owners
	// with ZERO body edges, which would otherwise compile with no region.
	for _, n := range g.Nodes {
		if !ControlKinds[n.Kind] {
			continue
		}
		idxs := branchesByOwner[n.ID]
		if n.Kind == NodeParallel {
			seen := map[int]bool{}
			ok := len(idxs) > 0
			for _, i := range idxs {
				if seen[i] {
					ok = false
				}
				seen[i] = true
			}
			for i := 0; i < len(idxs); i++ {
				if !seen[i] {
					ok = false
				}
			}
			if !ok {
				issues = append(issues, ValidationIssue{Code: IssueRegionBranches, Message: fmt.Sprintf("parallel %q needs body:0..body:N branches (contiguous, unique, at least one)", n.ID), NodeID: n.ID})
			}
		} else if len(idxs) != 1 || idxs[0] != 0 {
			// loop_for / loop_while / try_catch: exactly one "body" (branch 0).
			issues = append(issues, ValidationIssue{Code: IssueRegionBranches, Message: fmt.Sprintf("%s %q must declare exactly one body region (edge label \"body\")", n.Kind, n.ID), NodeID: n.ID})
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

	issues = append(issues, validateRegionExits(g, regionOf)...)
	issues = append(issues, validateLoopVarShadow(g, regionOf)...)
	return issues
}

// validateRegionExits ensures every region member can reach a region EXIT (a
// member with no in-region flow successor — control returns to the owner there).
// Without this, a body region with an internal cycle and no fallthrough compiles
// clean and the region runner spins to the max-step guard.
func validateRegionExits(g *Graph, regionOf map[string]string) []ValidationIssue {
	// in-region flow adjacency (forward) per region.
	type radj struct{ succ map[string][]string }
	byRegion := map[string]*radj{}
	memberCount := map[string]int{}
	for _, n := range g.Nodes {
		if r := regionOf[n.ID]; r != "" {
			memberCount[r]++
			if byRegion[r] == nil {
				byRegion[r] = &radj{succ: map[string][]string{}}
			}
			if _, ok := byRegion[r].succ[n.ID]; !ok {
				byRegion[r].succ[n.ID] = nil // ensure the node is a key in its region adjacency
			}
		}
	}
	for _, e := range g.Edges {
		if rid, _ := regionID(e.From, e.Label); rid != "" {
			continue // body edge
		}
		if r := regionOf[e.From]; r != "" && r == regionOf[e.To] {
			byRegion[r].succ[e.From] = append(byRegion[r].succ[e.From], e.To)
		}
	}
	// Compute exit-reachability ONCE per region (not per node — that was O(N²)).
	reachByRegion := map[string]map[string]bool{}
	for r, adj := range byRegion {
		rev := map[string][]string{}
		var q []string
		reach := map[string]bool{}
		for m, ss := range adj.succ {
			if len(ss) == 0 { // a region exit
				reach[m] = true
				q = append(q, m)
			}
			for _, s := range ss {
				rev[s] = append(rev[s], m)
			}
		}
		for len(q) > 0 {
			cur := q[0]
			q = q[1:]
			for _, p := range rev[cur] {
				if !reach[p] {
					reach[p] = true
					q = append(q, p)
				}
			}
		}
		reachByRegion[r] = reach
	}
	var issues []ValidationIssue
	for _, n := range g.Nodes { // graph order → deterministic
		r := regionOf[n.ID]
		if r == "" {
			continue
		}
		if !reachByRegion[r][n.ID] {
			issues = append(issues, ValidationIssue{Code: IssueNoPathToTerminal, Message: fmt.Sprintf("node %q cannot reach a region exit (the region never returns to its owner)", n.ID), NodeID: n.ID})
		}
	}
	return issues
}

// ownerOf returns the owner node id of a branch region ("<owner>" or
// "<owner>#<i>"). Node ids never contain '#'.
func ownerOf(region string) string {
	if region == "" {
		return ""
	}
	if i := strings.IndexByte(region, '#'); i >= 0 {
		return region[:i]
	}
	return region
}

// validateLoopVarShadow rejects a set_var/compute whose target equals the
// item/index var of ANY enclosing loop (walks the region ancestor chain, so a
// nested-region write can't silently shadow an outer loop var).
func validateLoopVarShadow(g *Graph, regionOf map[string]string) []ValidationIssue {
	loopVars := map[string][2]string{} // loop node id -> {item, index}
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
		loopVars[n.ID] = [2]string{item, index}
	}
	// enclosing(nodeID) → set of loop vars in scope (own region + all ancestors).
	enclosing := func(nodeID string) map[string]bool {
		out := map[string]bool{}
		for r := regionOf[nodeID]; r != ""; r = regionOf[ownerOf(r)] {
			if v, ok := loopVars[ownerOf(r)]; ok {
				out[v[0]] = true
				out[v[1]] = true
			}
		}
		return out
	}
	var issues []ValidationIssue
	for _, n := range g.Nodes {
		// Every name a node BINDS — set_var/compute targets AND a nested control
		// node's own vars (loop item/index, caught error). All can shadow an
		// enclosing loop's var (agy review HIGH).
		var names []string
		switch n.Kind {
		case NodeSetVar:
			if cfg, err := decodeConfig[setVarConfig](n.Config); err == nil && cfg.Name != "" {
				names = append(names, cfg.Name)
			}
		case NodeCompute:
			if cfg, err := decodeConfig[computeConfig](n.Config); err == nil && cfg.Var != "" {
				names = append(names, cfg.Var)
			}
		case NodeLoopFor:
			if v := loopVars[n.ID]; v != [2]string{} {
				names = append(names, v[0], v[1])
			}
		case NodeTryCatch:
			if cfg, err := decodeConfig[tryCatchConfig](n.Config); err == nil {
				ev := cfg.ErrorVar
				if ev == "" {
					ev = "error"
				}
				names = append(names, ev)
			}
		}
		encl := enclosing(n.ID)
		for _, name := range names {
			if encl[name] {
				issues = append(issues, ValidationIssue{Code: IssueLoopVarShadow, Message: fmt.Sprintf("node %q binds %q, which shadows an enclosing loop's variable", n.ID, name), NodeID: n.ID})
				break
			}
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

// regionForbiddenKinds need an EXTERNAL party (an agent accept, a caller's input)
// plus a durable resume cursor. v0.2 resumes only TOP-LEVEL cursors, so one inside
// a loop/parallel/try region would resume with corrupted scope — reject it
// (review B3). `wait` is intentionally NOT here: it auto-resumes deterministically
// in simulation (the parallel clock-join feature) and needs no external party.
var regionForbiddenKinds = map[NodeKind]bool{
	NodeReservation: true,
	NodeGetDTMF:     true, NodePromptText: true, NodeWaitSignal: true,
	NodeManualApproval: true, NodeDetectSpeech: true, NodeCSATSurvey: true, NodeNPSSurvey: true,
}

func validateSuspendScope(g *Graph) []ValidationIssue {
	var issues []ValidationIssue
	for _, n := range g.Nodes {
		if n.Region != "" && regionForbiddenKinds[n.Kind] {
			issues = append(issues, ValidationIssue{
				Code: IssueSuspendInRegion, NodeID: n.ID,
				Message: fmt.Sprintf("%s %q cannot be inside a loop/parallel/try region (v0.2 resumes top-level only)", n.Kind, n.ID),
			})
		}
	}
	return issues
}

// fixedOutPorts are the kinds whose out-edge labels are a known fixed set. Every
// out-edge of such a node must use a declared port, no port may be wired twice,
// and (for the binary/branch kinds) all ports must be present — otherwise a graph
// that should branch silently "completes" at runtime when it takes an unwired
// port (review B4). switch_case ports are dynamic (case values) and control nodes
// own their body/done/catch edges (validateRegions), so both are excluded here.
var fixedOutPorts = map[NodeKind][]string{
	NodeIfElse:         {"true", "false"},
	NodeReservation:    {"accepted", "timeout", "no_candidate"},
	NodeGetDTMF:        {"captured", "timeout"},
	NodePromptText:     {"captured", "timeout"},
	NodeWaitSignal:     {"received", "timeout"},
	NodeManualApproval: {"approved", "rejected", "timeout"},
	NodeDetectSpeech:   {"recognized", "no_match", "timeout"},
	NodeCSATSurvey:     {"done", "timeout"},
	NodeNPSSurvey:      {"done", "timeout"},
}

func validatePorts(g *Graph) []ValidationIssue {
	var issues []ValidationIssue
	kindOf := make(map[string]NodeKind, len(g.Nodes))
	for _, n := range g.Nodes {
		kindOf[n.ID] = n.Kind
	}
	// out-edge labels per source node.
	labels := make(map[string][]string)
	for _, e := range g.Edges {
		labels[e.From] = append(labels[e.From], e.Label)
	}
	for _, n := range g.Nodes {
		ls := labels[n.ID]
		// Duplicate labels (incl. duplicate unlabeled) are ambiguous for any node.
		seen := map[string]int{}
		for _, l := range ls {
			seen[l]++
		}
		for l, c := range seen {
			if c > 1 {
				disp := l
				if disp == "" {
					disp = "(unlabeled)"
				}
				issues = append(issues, ValidationIssue{Code: IssuePortCardinality, NodeID: n.ID,
					Message: fmt.Sprintf("node %q has %d out-edges labelled %s; labels must be unique", n.ID, c, disp)})
			}
		}
		var ports []string
		var fixed bool
		if n.Kind == NodeSwitchCase {
			// switch_case ports are dynamic: its declared cases + the built-in
			// default. Validate against that so a missing/typo'd case edge is caught
			// (re-review BLOCK) rather than silently completing at runtime.
			cfg, _ := decodeConfig[switchCaseConfig](n.Config)
			seenC := map[string]bool{}
			for _, c := range cfg.Cases {
				c = strings.TrimSpace(c)
				if c != "" && c != "default" && !seenC[c] {
					seenC[c] = true
					ports = append(ports, c)
				}
			}
			ports = append(ports, "default")
			fixed = true
		} else {
			ports, fixed = fixedOutPorts[n.Kind]
		}
		if !fixed {
			continue
		}
		allowed := map[string]bool{}
		for _, p := range ports {
			allowed[p] = true
		}
		present := map[string]bool{}
		for _, l := range ls {
			present[l] = true
			if !allowed[l] {
				issues = append(issues, ValidationIssue{Code: IssuePortCardinality, NodeID: n.ID,
					Message: fmt.Sprintf("node %q (%s) has an out-edge with unknown port %q", n.ID, n.Kind, l)})
			}
		}
		for _, p := range ports {
			if !present[p] {
				issues = append(issues, ValidationIssue{Code: IssuePortCardinality, NodeID: n.ID,
					Message: fmt.Sprintf("node %q (%s) is missing an out-edge for port %q", n.ID, n.Kind, p)})
			}
		}
	}
	return issues
}
