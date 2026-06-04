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
