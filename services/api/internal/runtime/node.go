// Package runtime is the v0.2 routing execution library, shared by cmd/runtime
// (live execution + the time-driven worker) and cmd/api (in-process simulation).
//
// The node model is the extension seam: each executable node kind is a
// self-contained Node implementing Validate/Compile/Execute plus a UI
// Descriptor, registered once in a Registry. Validation, compilation, runtime,
// and the UI palette all iterate the registry generically — adding a node is
// one file plus one registration, never edits spread across every surface.
//
// Layer 1 ships the contract + registry + 12 registered stubs; Layer 3 fills the
// method bodies. Keeping the seam now is cheap; retrofitting it later is not.
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// NodeKind discriminates the v0.2 executable node subset.
type NodeKind string

const (
	NodeTrigger     NodeKind = "trigger"
	NodeIfElse      NodeKind = "if_else"
	NodeSwitchCase  NodeKind = "switch_case"
	NodeWait        NodeKind = "wait"
	NodeMatchSkill  NodeKind = "match_skill"
	NodeFilter      NodeKind = "filter"
	NodeRouteQueue  NodeKind = "route_queue"
	NodeReservation NodeKind = "reservation"
	NodeFallback    NodeKind = "fallback"
	NodeEffect      NodeKind = "effect"
	NodeLog         NodeKind = "log"
	NodeEnd         NodeKind = "end"
	NodeSetVar      NodeKind = "set_var"
	NodeCompute     NodeKind = "compute"
	// 3D-2 control-flow (region owners). The executor special-cases these via a
	// recursive region runner; their Execute stays the baseNode stub.
	NodeLoopFor   NodeKind = "loop_for"
	NodeLoopWhile NodeKind = "loop_while"
	NodeParallel  NodeKind = "parallel"
	NodeTryCatch  NodeKind = "try_catch"
)

// V02NodeKinds is the locked v0.2 subset, in palette order. Tests assert the
// registry covers exactly this set so a new node can't be half-added.
var V02NodeKinds = []NodeKind{
	NodeTrigger, NodeIfElse, NodeSwitchCase, NodeWait, NodeMatchSkill, NodeFilter,
	NodeRouteQueue, NodeReservation, NodeFallback, NodeEffect, NodeLog, NodeEnd,
	// 3D-1: deterministic data nodes (graduated from the "Soon" palette).
	NodeSetVar, NodeCompute,
	// 3D-2: control-flow region owners.
	NodeLoopFor, NodeLoopWhile, NodeParallel, NodeTryCatch,
}

// ControlKinds are the region-owning control-flow nodes the executor runs via
// the recursive region runner (NOT via Node.Execute).
var ControlKinds = map[NodeKind]bool{
	NodeLoopFor: true, NodeLoopWhile: true, NodeParallel: true, NodeTryCatch: true,
}

// GraphNode / GraphEdge are the parsed authoring graph. Config is the node's
// opaque per-kind settings (validated by the node's Validate).
type GraphNode struct {
	ID     string          `json:"id"`
	Kind   NodeKind        `json:"type"`
	Config json.RawMessage `json:"config,omitempty"`
	// Region (3D-2) is the branch region this node belongs to: "" = top level,
	// "<ownerID>" for a single-body region (loop/try), "<ownerID>#<i>" for
	// parallel branch i. Assigned by the builder; bounds the executor sub-walk.
	Region string `json:"region,omitempty"`
}

type GraphEdge struct {
	ID    string `json:"id,omitempty"`
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// ValidationIssue locates a problem at a node, edge, or field. The empty
// locators distinguish graph-level from node/edge/field-level issues.
type ValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	NodeID  string `json:"node_id,omitempty"`
	EdgeID  string `json:"edge_id,omitempty"`
	Field   string `json:"field,omitempty"`
}

// CatalogRefs is the reference-checking surface a node's Validate uses to
// confirm queues/skills/channels/adapters/break-reasons exist in the org.
// Layer 3 backs this with org-scoped catalog reads.
type CatalogRefs interface {
	HasQueue(code string) bool
	HasSkill(code string) bool
	HasChannel(code string) bool
	HasAdapter(code string) bool
	HasBreakReason(code string) bool
}

// PlanStep is one compiled, executable step. Compiled is the node's opaque
// compiled form; the runtime hands it back to Execute.
type PlanStep struct {
	NodeID   string          `json:"node_id"`
	Kind     NodeKind        `json:"kind"`
	Compiled json.RawMessage `json:"compiled,omitempty"`
	Region   string          `json:"region,omitempty"` // 3D-2: owning region, "" = top level
}

// ExecCtx is the per-step execution surface the runtime provides to Execute:
// the variable bag, the pinned clock, and event emission. Defined as an
// interface so simulation (virtual clock, scripted outcomes) and live execution
// supply different implementations behind one node contract.
//
// Now() MUST be used instead of time.Now() so a node is deterministic under the
// simulator's virtual clock. Layer 3 adds catalog-snapshot / reservation /
// effect accessors here — those are additive (they don't change the Node
// interface), so they are deferred until the execution model is built.
type ExecCtx interface {
	context.Context
	Now() time.Time
	Var(key string) (any, bool)
	SetVar(key string, val any)
	Emit(eventType string, payload any)

	// Routing surface (Wave 3 part 2). Candidates is the working candidate pool
	// threaded route_queue -> match_skill/filter -> reservation. Snapshot is the
	// pinned catalog+state read-set. Reserve resolves one offer (the driver owns
	// the clock advance on timeout). Empty/no-op for runs without a routing
	// driver (pure control-flow tests).
	Candidates() []Candidate
	SetCandidates(c []Candidate)
	Snapshot() *Snapshot
	Reserve(agentID string, timeout time.Duration) ReservationOutcome
}

// RoutingFailureCode is the typed taxonomy of routing failures. Mirrors the
// OpenAPI RoutingFailureCode enum so runtime and clients agree on the codes.
type RoutingFailureCode string

const (
	FailMissingPublishedFlow          RoutingFailureCode = "missing_published_flow"
	FailMissingCatalogReference       RoutingFailureCode = "missing_catalog_reference"
	FailNoEligibleCandidate           RoutingFailureCode = "no_eligible_candidate"
	FailMultipleActiveBindings        RoutingFailureCode = "multiple_active_bindings"
	FailInvalidGraph                  RoutingFailureCode = "invalid_graph"
	FailReservationTransitionConflict RoutingFailureCode = "reservation_transition_conflict"
	// 3D-2 control-flow failures (catchable by try_catch).
	FailLoopLimit        RoutingFailureCode = "loop_limit"
	FailInvalidLoopInput RoutingFailureCode = "invalid_loop_input"
)

// RoutingFailure is a typed terminal failure a node yields (recorded on the
// route request + trace).
type RoutingFailure struct {
	Code    RoutingFailureCode
	Message string
}

// Suspension parks a durable continuation: ResumeAt is the worker's due_at and
// Cursor is the execution position to resume from. A wait node returns one with
// ResumeAt = Now()+duration; a reservation offer returns one with ResumeAt =
// the offer timeout. Without this the SKIP-LOCKED worker would have no due_at.
type Suspension struct {
	ResumeAt time.Time
	Cursor   json.RawMessage
}

// StepResult is what Execute returns. Exactly one outcome is expected: follow
// an edge (by explicit Next node id, or by Port for a branch node), suspend
// (park a continuation due at ResumeAt), fail (typed), or terminate.
//
// Port selects which labelled out-edge to follow when a node has several
// (if_else -> "true"/"false", switch_case -> the case value). The executor
// resolves (node, Port) against the compiled edges. Next is the escape hatch
// for a node that names its successor directly; an empty Port + single out-edge
// means "follow the only edge".
//
// Output is recorded on the trace step (node-reported outputs for debugging);
// it does not affect routing.
type StepResult struct {
	Next       string          `json:"next,omitempty"`
	Port       string          `json:"port,omitempty"`
	Output     map[string]any  `json:"output,omitempty"`
	Terminal   bool            `json:"terminal,omitempty"`
	Suspension *Suspension     `json:"-"`
	Failure    *RoutingFailure `json:"-"`
}

// Descriptor is the UI/palette metadata for a node kind. The flow builder
// renders the palette and property panel from these, so a new node appears in
// the UI by registration alone.
type Descriptor struct {
	Kind     NodeKind `json:"kind"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Summary  string   `json:"summary"`
}

// Node is the per-kind contract. The whole point of v0.2's "narrow but easy to
// extend" subset: implement these four, register once.
type Node interface {
	Kind() NodeKind
	Descriptor() Descriptor
	// Validate takes a context and returns an error so reference checks that
	// touch the DB/cache surface (via refs) can fail distinctly from a
	// graph-level ValidationIssue (cross-AI review C13).
	Validate(ctx context.Context, n GraphNode, g *Graph, refs CatalogRefs) ([]ValidationIssue, error)
	Compile(n GraphNode, g *Graph) (PlanStep, error)
	Execute(ctx ExecCtx, step PlanStep) (StepResult, error)
}

// Registry holds the node kinds the runtime knows. Validation, compilation,
// runtime, and the UI palette all iterate it — no per-kind switch statements
// scattered across the codebase.
type Registry struct {
	nodes map[NodeKind]Node
	order []NodeKind
}

func NewRegistry() *Registry {
	return &Registry{nodes: make(map[NodeKind]Node)}
}

// Register adds a node, preserving registration order for stable palette output.
// Panics on a duplicate kind — a programming error caught at startup, not runtime.
func (r *Registry) Register(n Node) {
	k := n.Kind()
	if _, dup := r.nodes[k]; dup {
		panic(fmt.Sprintf("runtime: duplicate node registration for kind %q", k))
	}
	r.nodes[k] = n
	r.order = append(r.order, k)
}

// Lookup returns the node for a kind (ok=false if unknown).
func (r *Registry) Lookup(k NodeKind) (Node, bool) {
	n, ok := r.nodes[k]
	return n, ok
}

// Kinds returns the registered kinds in registration order.
func (r *Registry) Kinds() []NodeKind {
	out := make([]NodeKind, len(r.order))
	copy(out, r.order)
	return out
}

// Descriptors returns every node's UI descriptor in registration order — the
// data the flow-builder palette is generated from.
func (r *Registry) Descriptors() []Descriptor {
	out := make([]Descriptor, 0, len(r.order))
	for _, k := range r.order {
		out = append(out, r.nodes[k].Descriptor())
	}
	return out
}
