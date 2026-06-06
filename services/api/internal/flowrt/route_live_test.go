package flowrt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// Seed a route_request + one offered reservation directly, then exercise the
// read endpoints (mapping + org isolation). The live write path (W3-3 handlers)
// lands separately; these reads are usable now.
func TestRouteReads_GetListMapping(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	routeID := uuid.Must(uuid.NewV7())
	fvID := uuid.Must(uuid.NewV7())
	flowCode := "flow_x"
	if _, err := f.q.InsertRouteRequest(f.ctx, generated.InsertRouteRequestParams{
		ID: pgUUID(routeID), OrgID: pgUUID(f.orgID), Channel: "voice", EntryCode: "ivr_main",
		FlowVersionID: pgUUID(fvID), FlowCode: &flowCode,
		InteractionInput: []byte(`{}`), Status: "waiting", // CHECK: non-pending needs a pinned version
	}); err != nil {
		t.Fatalf("seed route: %v", err)
	}
	resID := uuid.Must(uuid.NewV7())
	agentID := uuid.Must(uuid.NewV7())
	if _, err := f.q.InsertReservationOffer(f.ctx, generated.InsertReservationOfferParams{
		ID: pgUUID(resID), OrgID: pgUUID(f.orgID), RouteRequestID: pgUUID(routeID),
		AgentID: pgUUID(agentID), Attempt: 1,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Second), Valid: true},
	}); err != nil {
		t.Fatalf("seed reservation: %v", err)
	}

	// GetRouteRequest
	gr, err := f.e.GetRouteRequest(f.ctx, api.GetRouteRequestRequestObject{Id: api.EntityIdPath(routeID)})
	if err != nil {
		t.Fatalf("get route: %v", err)
	}
	got, ok := gr.(api.GetRouteRequest200JSONResponse)
	if !ok {
		t.Fatalf("want 200, got %T", gr)
	}
	if got.Channel != "voice" || got.Status != api.RouteRequestStatus("waiting") {
		t.Fatalf("route mapping wrong: %+v", got)
	}

	// ListRouteRequests
	lr, _ := f.e.ListRouteRequests(f.ctx, api.ListRouteRequestsRequestObject{})
	ll, ok := lr.(api.ListRouteRequests200JSONResponse)
	if !ok || len(ll.Items) != 1 {
		t.Fatalf("list route: want 1, got %T %v", lr, lr)
	}

	// GetReservation
	rr, err := f.e.GetReservation(f.ctx, api.GetReservationRequestObject{Id: api.EntityIdPath(resID)})
	if err != nil {
		t.Fatalf("get reservation: %v", err)
	}
	rget, ok := rr.(api.GetReservation200JSONResponse)
	if !ok {
		t.Fatalf("want 200, got %T", rr)
	}
	if rget.State != api.ReservationState("offered") || rget.Attempt != 1 {
		t.Fatalf("reservation mapping wrong: %+v", rget)
	}

	// ListRouteRequestReservations
	lrr, _ := f.e.ListRouteRequestReservations(f.ctx, api.ListRouteRequestReservationsRequestObject{Id: api.EntityIdPath(routeID)})
	lres, ok := lrr.(api.ListRouteRequestReservations200JSONResponse)
	if !ok || len(lres.Items) != 1 {
		t.Fatalf("list reservations: want 1, got %T", lrr)
	}

	// 404 on an unknown route
	nf, _ := f.e.GetRouteRequest(f.ctx, api.GetRouteRequestRequestObject{Id: api.EntityIdPath(uuid.Must(uuid.NewV7()))})
	if _, ok := nf.(api.GetRouteRequest404JSONResponse); !ok {
		t.Fatalf("want 404 for unknown route, got %T", nf)
	}
}

func TestListRouteRequestEvents(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	routeID := uuid.Must(uuid.NewV7())
	fvID := uuid.Must(uuid.NewV7())
	flowCode := "flow_ev"
	if _, err := f.q.InsertRouteRequest(f.ctx, generated.InsertRouteRequestParams{
		ID: pgUUID(routeID), OrgID: pgUUID(f.orgID), Channel: "voice", EntryCode: "main",
		FlowVersionID: pgUUID(fvID), FlowCode: &flowCode, InteractionInput: []byte(`{}`), Status: "waiting",
	}); err != nil {
		t.Fatalf("seed route: %v", err)
	}
	resID := uuid.Must(uuid.NewV7())
	f.e.appendEvent(f.ctx, f.q, f.orgID, routeID, "route.created", nil)
	f.e.appendEvent(f.ctx, f.q, f.orgID, routeID, "reservation.offered", map[string]any{"reservation_id": resID.String()})
	// Event on a different route must not bleed into this route's log.
	otherRoute := uuid.Must(uuid.NewV7())
	if _, err := f.q.InsertRouteRequest(f.ctx, generated.InsertRouteRequestParams{
		ID: pgUUID(otherRoute), OrgID: pgUUID(f.orgID), Channel: "voice", EntryCode: "main",
		FlowVersionID: pgUUID(fvID), FlowCode: &flowCode, InteractionInput: []byte(`{}`), Status: "waiting",
	}); err != nil {
		t.Fatalf("seed other route: %v", err)
	}
	f.e.appendEvent(f.ctx, f.q, f.orgID, otherRoute, "route.created", nil)

	resp, err := f.e.ListRouteRequestEvents(f.ctx, api.ListRouteRequestEventsRequestObject{Id: api.EntityIdPath(routeID)})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	ok200, ok := resp.(api.ListRouteRequestEvents200JSONResponse)
	if !ok {
		t.Fatalf("want 200, got %T", resp)
	}
	if len(ok200.Items) != 2 {
		t.Fatalf("events = %d, want 2 (org/route isolation)", len(ok200.Items))
	}
	if ok200.Items[0].Type != "route.created" || ok200.Items[1].Type != "reservation.offered" {
		t.Fatalf("event order wrong: %q, %q", ok200.Items[0].Type, ok200.Items[1].Type)
	}
	if ok200.Items[0].Source != "runtime" {
		t.Fatalf("source = %q, want runtime", ok200.Items[0].Source)
	}
	if ok200.Items[1].Payload == nil || (*ok200.Items[1].Payload)["reservation_id"] != resID.String() {
		t.Fatalf("payload not decoded: %+v", ok200.Items[1].Payload)
	}

	// Unknown route → empty list (not an error).
	empty, _ := f.e.ListRouteRequestEvents(f.ctx, api.ListRouteRequestEventsRequestObject{Id: api.EntityIdPath(uuid.Must(uuid.NewV7()))})
	if e, ok := empty.(api.ListRouteRequestEvents200JSONResponse); !ok || len(e.Items) != 0 {
		t.Fatalf("want empty list for unknown route, got %T %v", empty, empty)
	}
}

func TestListAgentReservations(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	fvID := uuid.Must(uuid.NewV7())
	flowCode := "flow_ar"
	exp := pgtype.Timestamptz{Time: time.Now().Add(30 * time.Second), Valid: true}
	// ux_reservations_route_active allows only one live reservation per route
	// (sequential offers), so each agent's offer is on its OWN route.
	mk := func(agent uuid.UUID) uuid.UUID {
		routeID := uuid.Must(uuid.NewV7())
		if _, err := f.q.InsertRouteRequest(f.ctx, generated.InsertRouteRequestParams{
			ID: pgUUID(routeID), OrgID: pgUUID(f.orgID), Channel: "voice", EntryCode: "main",
			FlowVersionID: pgUUID(fvID), FlowCode: &flowCode, InteractionInput: []byte(`{}`), Status: "waiting",
		}); err != nil {
			t.Fatalf("seed route: %v", err)
		}
		id := uuid.Must(uuid.NewV7())
		if _, err := f.q.InsertReservationOffer(f.ctx, generated.InsertReservationOfferParams{
			ID: pgUUID(id), OrgID: pgUUID(f.orgID), RouteRequestID: pgUUID(routeID), AgentID: pgUUID(agent), Attempt: 1, ExpiresAt: exp,
		}); err != nil {
			t.Fatalf("seed reservation: %v", err)
		}
		return id
	}
	agentA := uuid.Must(uuid.NewV7())
	agentB := uuid.Must(uuid.NewV7())
	aRes := mk(agentA)
	mk(agentB)

	// agentA has one live (offered) reservation.
	resp, _ := f.e.ListAgentReservations(f.ctx, api.ListAgentReservationsRequestObject{Id: api.EntityIdPath(agentA)})
	ok, isOK := resp.(api.ListAgentReservations200JSONResponse)
	if !isOK || len(ok.Items) != 1 || uuid.UUID(ok.Items[0].Id) != aRes {
		t.Fatalf("agentA live = %+v, want 1 (the offered one)", resp)
	}

	// Reject agentA's offer → it's no longer live; the endpoint returns empty.
	reason := "no"
	if _, err := f.q.RejectReservation(f.ctx, generated.RejectReservationParams{ID: pgUUID(aRes), OrgID: pgUUID(f.orgID), Reason: &reason}); err != nil {
		t.Fatalf("reject: %v", err)
	}
	resp2, _ := f.e.ListAgentReservations(f.ctx, api.ListAgentReservationsRequestObject{Id: api.EntityIdPath(agentA)})
	if r := resp2.(api.ListAgentReservations200JSONResponse); len(r.Items) != 0 {
		t.Fatalf("agentA after reject = %d live, want 0", len(r.Items))
	}
	// agentB still has its offered one (org/agent isolation).
	resp3, _ := f.e.ListAgentReservations(f.ctx, api.ListAgentReservationsRequestObject{Id: api.EntityIdPath(agentB)})
	if r := resp3.(api.ListAgentReservations200JSONResponse); len(r.Items) != 1 {
		t.Fatalf("agentB live = %d, want 1", len(r.Items))
	}
}

// CreateRouteRequest on a published flow with a Ready agent OFFERS the top
// candidate and parks the route (waiting + offered reservation + timeout
// continuation + persisted runtime trace).
func TestCreateRouteRequest_OffersAndSuspends(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
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
	rr, ok := resp.(api.CreateRouteRequest201JSONResponse)
	if !ok {
		t.Fatalf("want 201, got %T", resp)
	}
	if rr.Status != api.RouteRequestStatus("waiting") {
		t.Fatalf("status = %q, want waiting", rr.Status)
	}
	routeID := uuid.UUID(rr.Id)

	// Exactly one OFFERED reservation for the Ready agent.
	if n := f.countByOrg(t, "reservations"); n != 1 {
		t.Fatalf("reservations = %d, want 1", n)
	}
	if n := f.countByOrg(t, "continuations"); n != 1 {
		t.Fatalf("continuations = %d, want 1 (reservation_timeout parked)", n)
	}
	if n := f.countByOrg(t, "traces"); n != 1 {
		t.Fatalf("traces = %d, want 1 (runtime trace)", n)
	}
	lrr, _ := f.e.ListRouteRequestReservations(f.ctx, api.ListRouteRequestReservationsRequestObject{Id: api.EntityIdPath(routeID)})
	lres := lrr.(api.ListRouteRequestReservations200JSONResponse)
	if len(lres.Items) != 1 || lres.Items[0].State != api.ReservationState("offered") {
		t.Fatalf("want 1 offered reservation, got %+v", lres.Items)
	}
}

// No active binding for the entry → a typed-failure route (not a 500).
func TestCreateRouteRequest_NoPublishedFlowFails(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	resp, err := f.e.CreateRouteRequest(f.ctx, api.CreateRouteRequestRequestObject{
		Body: &api.CreateRouteRequest{Channel: "sms", EntryCode: "nope"},
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	rr, ok := resp.(api.CreateRouteRequest201JSONResponse)
	if !ok {
		t.Fatalf("want 201, got %T", resp)
	}
	if rr.Status != api.RouteRequestStatus("failed") || rr.FailureCode == nil || *rr.FailureCode != api.RoutingFailureCode("missing_published_flow") {
		t.Fatalf("want failed/missing_published_flow, got status=%q code=%v", rr.Status, rr.FailureCode)
	}
}
