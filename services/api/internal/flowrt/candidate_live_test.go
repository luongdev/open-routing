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
	lt := reservationLeaseToken(lf.ctx, t, lf.orgID, resID)

	if r, err := lf.e.ExecuteAgentCommand(lf.ctx, lf.orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, lt, CmdAccept, "h1"); err != nil || r.Status != "accepted" {
		t.Fatalf("accept = %q (err %v), want accepted", r.Status, err)
	}
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("after accept held=%d, want 1 (slot held for the call)", held)
	}
	if r, err := lf.e.ExecuteAgentCommand(lf.ctx, lf.orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, lt, CmdComplete, "h2"); err != nil || r.Status != "completed" {
		t.Fatalf("complete = %q (err %v), want completed", r.Status, err)
	}
	if held := lf.heldVoice(t, agentID); held != 0 {
		t.Fatalf("after complete held=%d, want 0 (slot released)", held)
	}
}

// TestLive_NoAgentParksWaitingMatch: in matcher mode, a route with no available
// agent parks waiting_match (W4 enqueue) with the flow's required skills, instead
// of falling through to fallback.
func TestLive_NoAgentParksWaitingMatch(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := New(Deps{
		OrgDB: lf.e.deps.OrgDB, Cache: lf.e.deps.Cache, Logger: lf.e.deps.Logger,
		Presence: lf.mem, Capacity: NewCapacityService(), MatcherEnabled: true,
	})
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3) // exists but NOT connected → empty live pool
	flowID := lf.seedFlow(t, "flow_q", simGraph(t))
	if _, err := me.PublishFlow(lf.ctx, api.PublishFlowRequestObject{
		Id: api.EntityIdPath(flowID), Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 1},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	resp, err := me.CreateRouteRequest(lf.ctx, api.CreateRouteRequestRequestObject{
		Body: &api.CreateRouteRequest{Channel: "voice", EntryCode: "main"},
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	routeID := uuid.UUID(resp.(api.CreateRouteRequest201JSONResponse).Id)

	var status string
	var skills []string
	if err := sharedPool.QueryRow(lf.ctx,
		"SELECT status, required_skills FROM route_requests WHERE id=$1 AND org_id=$2", routeID, lf.orgID).Scan(&status, &skills); err != nil {
		t.Fatalf("read route: %v", err)
	}
	if status != "waiting_match" {
		t.Fatalf("route status = %q, want waiting_match (parked for matcher)", status)
	}
	if len(skills) != 1 || skills[0] != "skill_es" {
		t.Fatalf("required_skills = %v, want [skill_es]", skills)
	}
}

// TestLive_AbandonReleasesAndCancels: a caller hang-up tears the route down —
// the outstanding offer is cancelled, its capacity slot freed, the route goes
// cancelled, and a second abandon is a 409 (already terminal).
func TestLive_AbandonReleasesAndCancels(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	lf.seedLiveFlow(t)
	agentID := lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(context.Background(), lf.orgID, agentID, "sess-1")
	routeID := lf.createRoute(t)
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("pre-abandon held=%d, want 1", held)
	}

	resp, err := lf.e.AbandonRouteRequest(lf.ctx, api.AbandonRouteRequestRequestObject{Id: api.EntityIdPath(routeID)})
	if err != nil {
		t.Fatalf("abandon: %v", err)
	}
	ok, is := resp.(api.AbandonRouteRequest200JSONResponse)
	if !is || ok.Status != api.RouteRequestStatus("cancelled") {
		t.Fatalf("abandon resp = %T status, want 200 cancelled", resp)
	}
	if held := lf.heldVoice(t, agentID); held != 0 {
		t.Fatalf("post-abandon held=%d, want 0 (slot freed)", held)
	}
	if rs := lf.reservations(t, routeID); len(rs) != 1 || rs[0].State != api.ReservationStateCancelled {
		t.Fatalf("reservation = %+v, want 1 cancelled", rs)
	}
	// Second abandon → 409 (terminal).
	resp2, _ := lf.e.AbandonRouteRequest(lf.ctx, api.AbandonRouteRequestRequestObject{Id: api.EntityIdPath(routeID)})
	if _, is := resp2.(api.AbandonRouteRequest409JSONResponse); !is {
		t.Fatalf("second abandon = %T, want 409", resp2)
	}
}

func (lf *liveFixture) heldVoice(t *testing.T, agentID uuid.UUID) int32 {
	return lf.held(t, agentID, "voice")
}

func (lf *liveFixture) held(t *testing.T, agentID uuid.UUID, channel string) int32 {
	t.Helper()
	held, err := generated.New(sharedPool).CountHeldCapacitySlots(lf.ctx, generated.CountHeldCapacitySlotsParams{
		OrgID: pgUUID(lf.orgID), AgentID: pgUUID(agentID), Channel: channel,
	})
	if err != nil {
		t.Fatalf("count held: %v", err)
	}
	return held
}

// TestLive_ChatAllowsConcurrentReservationsPerAgent proves dropping the legacy
// per-agent capacity=1 reservation index lets chat=N hold multiple concurrent
// reservations for one agent (slots are the authoritative gate — review HIGH).
func TestLive_ChatAllowsConcurrentReservationsPerAgent(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3)
	flowID := lf.seedFlow(t, "flow_chat", simGraph(t))
	if _, err := lf.e.PublishFlow(lf.ctx, api.PublishFlowRequestObject{
		Id: api.EntityIdPath(flowID), Body: &api.PublishFlowRequest{Channel: "chat", EntryCode: "main", Version: 1},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	agentID := lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(context.Background(), lf.orgID, agentID, "sess-1")

	for i := 0; i < 2; i++ {
		resp, err := lf.e.CreateRouteRequest(lf.ctx, api.CreateRouteRequestRequestObject{
			Body: &api.CreateRouteRequest{Channel: "chat", EntryCode: "main"},
		})
		if err != nil {
			t.Fatalf("create chat route %d: %v", i, err)
		}
		routeID := uuid.UUID(resp.(api.CreateRouteRequest201JSONResponse).Id)
		if rs := lf.reservations(t, routeID); len(rs) != 1 {
			t.Fatalf("chat route %d got %d reservations, want 1 (same agent, chat=N)", i, len(rs))
		}
	}
	if held := lf.held(t, agentID, "chat"); held != 2 {
		t.Fatalf("chat held=%d, want 2 (concurrent reservations for one agent)", held)
	}
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
func (errStore) ConnectedMany(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]bool, error) {
	return nil, context.DeadlineExceeded
}
func (errStore) Drop(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
