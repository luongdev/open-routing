package flowrt

import (
	"testing"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// seedRouteWithOffer publishes simGraph, creates a route (which offers the Ready
// agent), and returns the route id + the offered reservation id.
func (f *fixture) seedRouteWithOffer(t *testing.T) (routeID, resID uuid.UUID) {
	t.Helper()
	sid := f.seedSkillID(t, "skill_es")
	f.seedQueue(t, "queue_vip")
	f.seedReadyAgent(t, "agent_a", sid, 3)
	flowID := f.seedFlow(t, "flow_live", simGraph(t))
	if _, err := f.e.PublishFlow(f.ctx, api.PublishFlowRequestObject{
		Id: api.EntityIdPath(flowID), Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 1},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	resp, err := f.e.CreateRouteRequest(f.ctx, api.CreateRouteRequestRequestObject{
		Body: &api.CreateRouteRequest{Channel: "voice", EntryCode: "main"},
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	routeID = uuid.UUID(resp.(api.CreateRouteRequest201JSONResponse).Id)
	lrr, _ := f.e.ListRouteRequestReservations(f.ctx, api.ListRouteRequestReservationsRequestObject{Id: api.EntityIdPath(routeID)})
	items := lrr.(api.ListRouteRequestReservations200JSONResponse).Items
	if len(items) != 1 {
		t.Fatalf("want 1 offered reservation, got %d", len(items))
	}
	return routeID, uuid.UUID(items[0].Id)
}

func (f *fixture) agentStatus(t *testing.T, code string) string {
	t.Helper()
	var status string
	if err := sharedPool.QueryRow(f.ctx,
		"SELECT ast.status FROM agent_states ast JOIN agents a ON a.id=ast.agent_id WHERE a.code=$1 AND ast.org_id=$2",
		code, f.orgID).Scan(&status); err != nil {
		t.Fatalf("agent status: %v", err)
	}
	return status
}

func TestAcceptReservation_EngagesAndCompletesRoute(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	routeID, resID := f.seedRouteWithOffer(t)

	resp, err := f.e.AcceptReservation(f.ctx, api.AcceptReservationRequestObject{Id: api.EntityIdPath(resID)})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	acc, ok := resp.(api.AcceptReservation200JSONResponse)
	if !ok {
		t.Fatalf("want 200, got %T", resp)
	}
	if acc.State != api.ReservationState("accepted") {
		t.Fatalf("reservation state = %q, want accepted", acc.State)
	}
	// accepted → end → route completed.
	gr, _ := f.e.GetRouteRequest(f.ctx, api.GetRouteRequestRequestObject{Id: api.EntityIdPath(routeID)})
	if gr.(api.GetRouteRequest200JSONResponse).Status != api.RouteRequestStatus("completed") {
		t.Fatalf("route status = %q, want completed", gr.(api.GetRouteRequest200JSONResponse).Status)
	}
	if s := f.agentStatus(t, "agent_a"); s != "Engaged" {
		t.Fatalf("agent status = %q, want Engaged", s)
	}
}

func TestAcceptReservation_SecondAcceptConflicts(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	_, resID := f.seedRouteWithOffer(t)
	if _, err := f.e.AcceptReservation(f.ctx, api.AcceptReservationRequestObject{Id: api.EntityIdPath(resID)}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	// Second accept: route no longer waiting (completed) → 409.
	resp, _ := f.e.AcceptReservation(f.ctx, api.AcceptReservationRequestObject{Id: api.EntityIdPath(resID)})
	if _, ok := resp.(api.AcceptReservation409JSONResponse); !ok {
		t.Fatalf("want 409 on second accept, got %T", resp)
	}
}

func TestRejectReservation_ReoffersThenNoCandidate(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	routeID, resID := f.seedRouteWithOffer(t)
	resp, err := f.e.RejectReservation(f.ctx, api.RejectReservationRequestObject{Id: api.EntityIdPath(resID)})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	rj := resp.(api.RejectReservation200JSONResponse)
	if rj.State != api.ReservationState("rejected") {
		t.Fatalf("reservation state = %q, want rejected", rj.State)
	}
	// Only agent_a exists and is now excluded → no_candidate → fallback → end →
	// route completed.
	gr, _ := f.e.GetRouteRequest(f.ctx, api.GetRouteRequestRequestObject{Id: api.EntityIdPath(routeID)})
	if gr.(api.GetRouteRequest200JSONResponse).Status != api.RouteRequestStatus("completed") {
		t.Fatalf("route status = %q, want completed (no_candidate fallback)", gr.(api.GetRouteRequest200JSONResponse).Status)
	}
}

func TestCompleteReservation_WrapsUpAgent(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	_, resID := f.seedRouteWithOffer(t)
	if _, err := f.e.AcceptReservation(f.ctx, api.AcceptReservationRequestObject{Id: api.EntityIdPath(resID)}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	resp, err := f.e.CompleteReservation(f.ctx, api.CompleteReservationRequestObject{Id: api.EntityIdPath(resID)})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	cmp, ok := resp.(api.CompleteReservation200JSONResponse)
	if !ok {
		t.Fatalf("want 200, got %T", resp)
	}
	if cmp.State != api.ReservationState("completed") {
		t.Fatalf("reservation state = %q, want completed", cmp.State)
	}
	if s := f.agentStatus(t, "agent_a"); s != "WrapUp" {
		t.Fatalf("agent status = %q, want WrapUp", s)
	}
}
