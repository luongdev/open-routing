package flowrt

import (
	"testing"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
)

func (f *fixture) seedSkillID(t *testing.T, code string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := f.q.InsertSkill(f.ctx, generated.InsertSkillParams{
		ID: pgUUID(id), OrgID: pgUUID(f.orgID), Code: code, Name: code, SkillType: "binary", Enabled: true,
	}); err != nil {
		t.Fatalf("seed skill: %v", err)
	}
	return id
}

func (f *fixture) seedReadyAgent(t *testing.T, code string, skillID uuid.UUID, prof int) {
	t.Helper()
	aid := uuid.Must(uuid.NewV7())
	if _, err := f.q.InsertAgent(f.ctx, generated.InsertAgentParams{
		ID: pgUUID(aid), OrgID: pgUUID(f.orgID), Code: code, Name: code, Email: code + "@x.test", Enabled: true,
	}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	if _, err := f.q.InsertAgentState(f.ctx, generated.InsertAgentStateParams{
		AgentID: pgUUID(aid), OrgID: pgUUID(f.orgID), Status: "Ready",
	}); err != nil {
		t.Fatalf("seed agent state: %v", err)
	}
	if _, err := f.q.InsertAgentSkill(f.ctx, generated.InsertAgentSkillParams{
		AgentID: pgUUID(aid), SkillID: pgUUID(skillID), OrgID: pgUUID(f.orgID), Proficiency: int32(prof),
	}); err != nil {
		t.Fatalf("seed agent skill: %v", err)
	}
}

// simGraph: trigger -> route_queue -> match_skill -> reservation
//
//	reservation -accepted->     end
//	reservation -timeout->      fb -> end
//	reservation -no_candidate-> fb
func simGraph(t *testing.T) runtime.Graph {
	return runtime.Graph{
		Nodes: []runtime.GraphNode{
			{ID: "t", Kind: runtime.NodeTrigger},
			{ID: "q", Kind: runtime.NodeRouteQueue, Config: cfg(t, map[string]string{"queue": "queue_vip"})},
			{ID: "s", Kind: runtime.NodeMatchSkill, Config: cfg(t, map[string]any{"skill": "skill_es", "min_proficiency": 1})},
			{ID: "r", Kind: runtime.NodeReservation, Config: cfg(t, map[string]any{"timeout_sec": 5, "max_attempts": 2})},
			{ID: "fb", Kind: runtime.NodeFallback},
			{ID: "end", Kind: runtime.NodeEnd},
		},
		Edges: []runtime.GraphEdge{
			{ID: "e1", From: "t", To: "q"},
			{ID: "e2", From: "q", To: "s"},
			{ID: "e3", From: "s", To: "r"},
			{ID: "e4", From: "r", To: "end", Label: "accepted"},
			{ID: "e5", From: "r", To: "fb", Label: "timeout"},
			{ID: "e6", From: "r", To: "fb", Label: "no_candidate"},
			{ID: "e7", From: "fb", To: "end"},
		},
	}
}

func (f *fixture) countByOrg(t *testing.T, table string) int {
	t.Helper()
	var n int
	if err := sharedPool.QueryRow(f.ctx, "SELECT count(*) FROM "+table+" WHERE org_id=$1", f.orgID).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func stepPort(steps []api.TraceStep, nodeID string) string {
	for _, s := range steps {
		if s.NodeId == nodeID && s.Port != nil {
			return *s.Port
		}
	}
	return ""
}

func TestSimulateFlow_RoutesAcceptedPersistsAndDoesNotMutateLiveState(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	sid := f.seedSkillID(t, "skill_es")
	f.seedQueue(t, "queue_vip")
	f.seedReadyAgent(t, "agent_a", sid, 3)
	id := f.seedFlow(t, "flow_sim", simGraph(t))

	resp, err := f.e.SimulateFlow(f.ctx, api.SimulateFlowRequestObject{
		Id: api.EntityIdPath(id),
		Body: &api.SimulateFlowRequest{
			InteractionInput:            map[string]any{},
			ScriptedReservationOutcomes: &[]api.SimulateScriptedReservationOutcome{{Outcome: api.Accepted}},
		},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	r, ok := resp.(api.SimulateFlow200JSONResponse)
	if !ok {
		t.Fatalf("want 200, got %T", resp)
	}
	if r.VirtualClockStart.IsZero() {
		t.Fatal("expected a resolved virtual_clock_start")
	}
	if got := stepPort(r.Trace.Steps, "r"); got != "accepted" {
		t.Fatalf("reservation port = %q, want accepted (steps=%+v)", got, r.Trace.Steps)
	}
	if r.Trace.FlowId == nil || uuid.UUID(*r.Trace.FlowId) != id {
		t.Fatalf("trace flow_id mismatch: %v", r.Trace.FlowId)
	}

	// Persisted exactly one simulation trace; ZERO live mutations.
	if n := f.countByOrg(t, "traces"); n != 1 {
		t.Fatalf("traces rows = %d, want 1", n)
	}
	for _, tbl := range []string{"reservations", "route_requests", "runtime_events", "continuations"} {
		if n := f.countByOrg(t, tbl); n != 0 {
			t.Fatalf("%s rows = %d, want 0 (sim must not mutate live state)", tbl, n)
		}
	}
}

func TestSimulateFlow_InvalidGraph422(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	// No catalog seeded → queue/skill refs dangle → invalid.
	id := f.seedFlow(t, "flow_bad", simGraph(t))
	resp, _ := f.e.SimulateFlow(f.ctx, api.SimulateFlowRequestObject{
		Id:   api.EntityIdPath(id),
		Body: &api.SimulateFlowRequest{InteractionInput: map[string]any{}},
	})
	if _, ok := resp.(api.SimulateFlow422JSONResponse); !ok {
		t.Fatalf("want 422 for invalid graph, got %T", resp)
	}
	if n := f.countByOrg(t, "traces"); n != 0 {
		t.Fatalf("invalid sim must not persist a trace, got %d", n)
	}
}

func TestTraces_ListAndGet_OrgIsolated(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	sid := f.seedSkillID(t, "skill_es")
	f.seedQueue(t, "queue_vip")
	f.seedReadyAgent(t, "agent_a", sid, 3)
	id := f.seedFlow(t, "flow_sim", simGraph(t))
	resp, _ := f.e.SimulateFlow(f.ctx, api.SimulateFlowRequestObject{
		Id:   api.EntityIdPath(id),
		Body: &api.SimulateFlowRequest{InteractionInput: map[string]any{}, ScriptedReservationOutcomes: &[]api.SimulateScriptedReservationOutcome{{Outcome: api.Accepted}}},
	})
	traceID := uuid.UUID(resp.(api.SimulateFlow200JSONResponse).Trace.Id)

	// List returns it for the owning org.
	lr, _ := f.e.ListFlowTraces(f.ctx, api.ListFlowTracesRequestObject{Id: api.EntityIdPath(id)})
	if l, ok := lr.(api.ListFlowTraces200JSONResponse); !ok || len(l.Traces) != 1 {
		t.Fatalf("list: want 1 trace, got %T %v", lr, lr)
	}

	// Get from a DIFFERENT org → 404 (org isolation).
	otherCtx := orgkey.SetOrgID(f.ctx, uuid.Must(uuid.NewV7()))
	gr, _ := f.e.GetTrace(otherCtx, api.GetTraceRequestObject{Id: api.EntityIdPath(traceID)})
	if _, ok := gr.(api.GetTrace404JSONResponse); !ok {
		t.Fatalf("cross-org GetTrace: want 404, got %T", gr)
	}
	// Owning org can get it.
	gr2, _ := f.e.GetTrace(f.ctx, api.GetTraceRequestObject{Id: api.EntityIdPath(traceID)})
	if _, ok := gr2.(api.GetTrace200JSONResponse); !ok {
		t.Fatalf("owner GetTrace: want 200, got %T", gr2)
	}
}
