package flowrt

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/presence"
)

// liveFixture wraps the base fixture with a live Endpoints (presence + capacity)
// sharing the same OrgDB/org so seeding via f.q is visible to the live path.
type liveFixture struct {
	*fixture
	mem *presence.MemStore
	e   *Endpoints
}

func newLiveFixture(t *testing.T) *liveFixture {
	t.Helper()
	f := newFixture(t)
	if f == nil {
		return nil
	}
	mem := presence.NewMemStore()
	return &liveFixture{
		fixture: f,
		mem:     mem,
		e: New(Deps{
			OrgDB: f.e.deps.OrgDB, Cache: f.e.deps.Cache, Logger: f.e.deps.Logger,
			Presence: mem, Capacity: NewCapacityService(),
		}),
	}
}

func (lf *liveFixture) agentID(t *testing.T, code string) uuid.UUID {
	t.Helper()
	row, err := lf.q.GetAgentByCode(lf.ctx, generated.GetAgentByCodeParams{Code: code, OrgID: pgUUID(lf.orgID)})
	if err != nil {
		t.Fatalf("agent by code %q: %v", code, err)
	}
	return apiUUID(row.ID)
}

func (lf *liveFixture) seedLiveFlow(t *testing.T) {
	t.Helper()
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3)
	flowID := lf.seedFlow(t, "flow_live", simGraph(t))
	if _, err := lf.e.PublishFlow(lf.ctx, api.PublishFlowRequestObject{
		Id: api.EntityIdPath(flowID), Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 1},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

func (lf *liveFixture) createRoute(t *testing.T) uuid.UUID {
	t.Helper()
	resp, err := lf.e.CreateRouteRequest(lf.ctx, api.CreateRouteRequestRequestObject{
		Body: &api.CreateRouteRequest{Channel: "voice", EntryCode: "main"},
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	ok, is := resp.(api.CreateRouteRequest201JSONResponse)
	if !is {
		t.Fatalf("create route response = %T, want 201", resp)
	}
	return uuid.UUID(ok.Id)
}

func (lf *liveFixture) reservations(t *testing.T, routeID uuid.UUID) []api.Reservation {
	t.Helper()
	lrr, _ := lf.e.ListRouteRequestReservations(lf.ctx, api.ListRouteRequestReservationsRequestObject{Id: api.EntityIdPath(routeID)})
	return lrr.(api.ListRouteRequestReservations200JSONResponse).Items
}

// A Ready+eligible agent who is NOT connected is omitted from the live pool, so
// no offer is made (route falls through to fallback).
func TestLive_DisconnectedAgentNotOffered(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	lf.seedLiveFlow(t)
	routeID := lf.createRoute(t)
	if rs := lf.reservations(t, routeID); len(rs) != 0 {
		t.Fatalf("disconnected agent got %d reservations, want 0", len(rs))
	}
}

// A connected agent is offerable: an offer is created and a capacity slot held.
func TestLive_ConnectedAgentOfferedAndSlotHeld(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	lf.seedLiveFlow(t)
	agentID := lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(context.Background(), lf.orgID, agentID, "sess-1")

	routeID := lf.createRoute(t)
	rs := lf.reservations(t, routeID)
	if len(rs) != 1 || rs[0].State != api.ReservationState("offered") {
		t.Fatalf("connected agent reservations = %+v, want 1 offered", rs)
	}
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("held voice slots = %d, want 1 (offer holds a slot)", held)
	}
}

// A connected agent already at capacity (voice=1) is omitted.
func TestLive_AtCapacityAgentNotOffered(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	lf.seedLiveFlow(t)
	agentID := lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(context.Background(), lf.orgID, agentID, "sess-1")

	// Occupy the single voice slot with an unrelated hold.
	lf.inTxLive(t, func(q *generated.Queries) error {
		if err := lf.e.deps.Capacity.ProvisionInTx(lf.ctx, q, lf.orgID, agentID, "voice"); err != nil {
			return err
		}
		_, _, err := lf.e.deps.Capacity.AcquireInTx(lf.ctx, q, lf.orgID, agentID, "voice", uuid.Must(uuid.NewV7()), time.Now().Add(time.Minute))
		return err
	})

	routeID := lf.createRoute(t)
	if rs := lf.reservations(t, routeID); len(rs) != 0 {
		t.Fatalf("at-capacity agent got %d reservations, want 0", len(rs))
	}
}

// A presence-store error parks the route (snapshot_failed) — it must NOT be read
// as "disconnected" nor fall back to a DB snapshot (review HIGH).
func TestLive_PresenceErrorParksRoute(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	lf.seedLiveFlow(t)
	bad := New(Deps{
		OrgDB: lf.e.deps.OrgDB, Cache: lf.e.deps.Cache, Logger: lf.e.deps.Logger,
		Presence: errStore{}, Capacity: NewCapacityService(),
	})
	resp, err := bad.CreateRouteRequest(lf.ctx, api.CreateRouteRequestRequestObject{
		Body: &api.CreateRouteRequest{Channel: "voice", EntryCode: "main"},
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	e500, ok := resp.(api.CreateRouteRequest500JSONResponse)
	if !ok || e500.Reason != "snapshot_failed" {
		t.Fatalf("presence error → %T (%v), want 500 snapshot_failed", resp, resp)
	}
}

// Accept confirms the held slot (it stays held for the live call); complete
// releases it — the full capacity lifecycle through the live command service.
func TestLive_AcceptConfirmsAndCompleteReleases(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	lf.seedLiveFlow(t)
	agentID := lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(context.Background(), lf.orgID, agentID, "sess-1")
	routeID := lf.createRoute(t)
	resID := uuid.UUID(lf.reservations(t, routeID)[0].Id)

	sess := uuid.Must(uuid.NewV7())
	if _, err := lf.q.CreateAgentSession(lf.ctx, generated.CreateAgentSessionParams{
		OrgID: pgUUID(lf.orgID), SessionID: pgUUID(sess), AgentID: pgUUID(agentID), GatewayID: "gw-test",
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if r, err := lf.e.ExecuteAgentCommand(lf.ctx, lf.orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, CmdAccept, "h1"); err != nil || r.Status != "accepted" {
		t.Fatalf("accept = %q (err %v), want accepted", r.Status, err)
	}
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("after accept held=%d, want 1 (slot held for the call)", held)
	}
	if r, err := lf.e.ExecuteAgentCommand(lf.ctx, lf.orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, CmdComplete, "h2"); err != nil || r.Status != "completed" {
		t.Fatalf("complete = %q (err %v), want completed", r.Status, err)
	}
	if held := lf.heldVoice(t, agentID); held != 0 {
		t.Fatalf("after complete held=%d, want 0 (slot released)", held)
	}
}

func (lf *liveFixture) heldVoice(t *testing.T, agentID uuid.UUID) int32 {
	t.Helper()
	held, err := generated.New(sharedPool).CountHeldCapacitySlots(lf.ctx, generated.CountHeldCapacitySlotsParams{
		OrgID: pgUUID(lf.orgID), AgentID: pgUUID(agentID), Channel: "voice",
	})
	if err != nil {
		t.Fatalf("count held: %v", err)
	}
	return held
}

func (lf *liveFixture) inTxLive(t *testing.T, fn func(q *generated.Queries) error) {
	t.Helper()
	tx, err := lf.e.deps.OrgDB.BeginTx(lf.ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(lf.ctx) }()
	if err := fn(generated.New(tx)); err != nil {
		t.Fatalf("tx: %v", err)
	}
	if err := tx.Commit(lf.ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// errStore is a presence.Store whose Connected always errors.
type errStore struct{}

func (errStore) Renew(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
func (errStore) Connected(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, context.DeadlineExceeded
}
func (errStore) Drop(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
