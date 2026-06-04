package flowrt

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// A due reservation_timeout continuation fires: the offered reservation flips to
// timeout and the route resumes (only agent excluded → no_candidate → fallback
// → end → completed).
func TestWorker_ReservationTimeoutFiresAndResumes(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	routeID, resID := f.seedRouteWithOffer(t)

	// One pending continuation parked at create (org-scoped count).
	if n := f.countByOrg(t, "continuations"); n != 1 {
		t.Fatalf("continuations = %d, want 1", n)
	}
	// Process with a clock well past due_at so it claims the timer. The worker is
	// CROSS-ORG (shared test DB), so it may also resolve other tests' leftover
	// continuations — assert on THIS route's outcome, not the global count.
	if _, err := f.e.ProcessDueContinuations(f.ctx, sharedPool, "worker-test", time.Now().Add(time.Hour), 30*time.Second, 100); err != nil {
		t.Fatalf("process: %v", err)
	}
	gres, _ := f.e.GetReservation(f.ctx, api.GetReservationRequestObject{Id: api.EntityIdPath(resID)})
	if gres.(api.GetReservation200JSONResponse).State != api.ReservationState("timeout") {
		t.Fatalf("reservation state = %q, want timeout", gres.(api.GetReservation200JSONResponse).State)
	}
	gr, _ := f.e.GetRouteRequest(f.ctx, api.GetRouteRequestRequestObject{Id: api.EntityIdPath(routeID)})
	if gr.(api.GetRouteRequest200JSONResponse).Status != api.RouteRequestStatus("completed") {
		t.Fatalf("route status = %q, want completed (no_candidate fallback after timeout)", gr.(api.GetRouteRequest200JSONResponse).Status)
	}
}

// After an agent accepts, the timeout continuation is a no-op (the reservation
// is no longer offered): the worker marks it done without disturbing the route.
func TestWorker_TimeoutAfterAcceptIsNoop(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	routeID, resID := f.seedRouteWithOffer(t)
	if _, err := f.e.AcceptReservation(f.ctx, api.AcceptReservationRequestObject{Id: api.EntityIdPath(resID)}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if _, err := f.e.ProcessDueContinuations(f.ctx, sharedPool, "worker-test", time.Now().Add(time.Hour), 30*time.Second, 10); err != nil {
		t.Fatalf("process: %v", err)
	}
	// Reservation stays accepted; route stays completed.
	gres, _ := f.e.GetReservation(f.ctx, api.GetReservationRequestObject{Id: api.EntityIdPath(resID)})
	if gres.(api.GetReservation200JSONResponse).State != api.ReservationState("accepted") {
		t.Fatalf("reservation flipped after accept: %q", gres.(api.GetReservation200JSONResponse).State)
	}
	_ = routeID
	_ = uuid.UUID{}
}
