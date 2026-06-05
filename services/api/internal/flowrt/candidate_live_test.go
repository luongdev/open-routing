package flowrt

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/presence"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
)

// simGraphWithWait is simGraph with a wait node AFTER the accepted port, so the
// route stays 'waiting' (non-terminal) post-accept — the window where an
// adapter-driven mid-call teardown (caller hangup) is meaningful.
func simGraphWithWait(t *testing.T) runtime.Graph {
	return runtime.Graph{
		Nodes: []runtime.GraphNode{
			{ID: "t", Kind: runtime.NodeTrigger},
			{ID: "q", Kind: runtime.NodeRouteQueue, Config: cfg(t, map[string]string{"queue": "queue_vip"})},
			{ID: "s", Kind: runtime.NodeMatchSkill, Config: cfg(t, map[string]any{"skill": "skill_es", "min_proficiency": 1})},
			{ID: "r", Kind: runtime.NodeReservation, Config: cfg(t, map[string]any{"timeout_sec": 5, "max_attempts": 2})},
			{ID: "w", Kind: runtime.NodeWait, Config: cfg(t, map[string]any{"duration_ms": 60000})},
			{ID: "fb", Kind: runtime.NodeFallback},
			{ID: "end", Kind: runtime.NodeEnd},
		},
		Edges: []runtime.GraphEdge{
			{ID: "e1", From: "t", To: "q"}, {ID: "e2", From: "q", To: "s"}, {ID: "e3", From: "s", To: "r"},
			{ID: "e4", From: "r", To: "w", Label: "accepted"},
			{ID: "e5", From: "r", To: "fb", Label: "timeout"},
			{ID: "e6", From: "r", To: "fb", Label: "no_candidate"},
			{ID: "e7", From: "w", To: "end"}, {ID: "e8", From: "fb", To: "end"},
		},
	}
}

// TestLive_AdapterCallerAbandonTearsDown: after an accept hands the assignment to
// the channel adapter, an ADAPTER-ORIGINATED caller hangup (MockVoice.Abandon →
// caller_abandoned event) drives the engine to tear the route down — cancel the
// reservation, free the slot, move the agent to WrapUp. This is goal-5's "adapter
// events drive the reservation lifecycle".
func TestLive_AdapterCallerAbandonTearsDown(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	mv := adapter.NewMockVoice(nil)
	me := New(Deps{
		OrgDB: lf.e.deps.OrgDB, Cache: lf.e.deps.Cache, Logger: lf.e.deps.Logger,
		Presence: lf.mem, Capacity: NewCapacityService(),
		Adapters: map[string]adapter.ChannelAdapter{"voice": mv},
	})
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3)
	flowID := lf.seedFlow(t, "flow_wait", simGraphWithWait(t))
	if _, err := me.PublishFlow(lf.ctx, api.PublishFlowRequestObject{
		Id: api.EntityIdPath(flowID), Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 1},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	agentID := lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(context.Background(), lf.orgID, agentID, "sess-1")

	resp, err := me.CreateRouteRequest(lf.ctx, api.CreateRouteRequestRequestObject{Body: &api.CreateRouteRequest{Channel: "voice", EntryCode: "main"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	routeID := uuid.UUID(resp.(api.CreateRouteRequest201JSONResponse).Id)
	rs, _ := me.ListRouteRequestReservations(lf.ctx, api.ListRouteRequestReservationsRequestObject{Id: api.EntityIdPath(routeID)})
	items := rs.(api.ListRouteRequestReservations200JSONResponse).Items
	if len(items) != 1 {
		t.Fatalf("reservations = %d, want 1 offered", len(items))
	}
	resID := uuid.UUID(items[0].Id)

	if _, err := me.AcceptReservation(lf.ctx, api.AcceptReservationRequestObject{Id: api.EntityIdPath(resID)}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("after accept held=%d, want 1", held)
	}
	// The adapter handle was bound on accept (Deliver).
	var handle string
	if err := sharedPool.QueryRow(lf.ctx, "SELECT adapter_handle FROM reservations WHERE id=$1", resID).Scan(&handle); err != nil || handle == "" {
		t.Fatalf("adapter_handle=%q err=%v, want a bound handle", handle, err)
	}

	// Caller hangs up — the adapter fires caller_abandoned into the engine sink.
	mv.Abandon(context.Background(), adapter.Handle(handle))

	st, _ := routeStatus(lf.ctx, t, lf.orgID, routeID)
	if st != "cancelled" {
		t.Fatalf("route after caller_abandoned = %q, want cancelled", st)
	}
	if held := lf.heldVoice(t, agentID); held != 0 {
		t.Fatalf("after caller_abandoned held=%d, want 0 (slot freed)", held)
	}
	var resState, agentStatus string
	_ = sharedPool.QueryRow(lf.ctx, "SELECT state FROM reservations WHERE id=$1", resID).Scan(&resState)
	_ = sharedPool.QueryRow(lf.ctx, "SELECT status FROM agent_states WHERE agent_id=$1 AND org_id=$2", agentID, lf.orgID).Scan(&agentStatus)
	if resState != "cancelled" || agentStatus != "WrapUp" {
		t.Fatalf("after teardown reservation=%q agent=%q, want cancelled + WrapUp", resState, agentStatus)
	}
}

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

// TestLive_InlineOfferWritesDecision: the interaction-driven (inline) offer path
// writes an interaction_offer route_decisions row, matching the matcher's audit.
func TestLive_InlineOfferWritesDecision(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	lf.seedLiveFlow(t)
	agentID := lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(context.Background(), lf.orgID, agentID, "sess-1")
	routeID := lf.createRoute(t)
	var n int
	if err := sharedPool.QueryRow(lf.ctx,
		"SELECT count(*) FROM route_decisions WHERE org_id=$1 AND route_request_id=$2 AND decision_type='interaction_offer' AND outcome='offered'",
		lf.orgID, routeID).Scan(&n); err != nil {
		t.Fatalf("count decisions: %v", err)
	}
	if n != 1 {
		t.Fatalf("interaction_offer decisions = %d, want 1 (inline audit parity)", n)
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

// TestLive_AbandonEndsAcceptedCall: abandoning a route with an in-progress
// ACCEPTED call cancels the reservation, releases its confirmed slot, and moves
// the agent to WrapUp — no capacity leak, no stranded-Engaged agent.
func TestLive_AbandonEndsAcceptedCall(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedReadyAgent(t, "agent_a", sid, 3)
	agentID := lf.agentID(t, "agent_a")
	ctx := lf.ctx
	// Agent on a live call (Engaged); a 'waiting' route with an accepted reservation
	// and a confirmed slot (reservation_id set, hold_expires_at NULL).
	if _, err := sharedPool.Exec(ctx, "UPDATE agent_states SET status='Engaged' WHERE agent_id=$1 AND org_id=$2", agentID, lf.orgID); err != nil {
		t.Fatalf("engage: %v", err)
	}
	routeID := uuid.Must(uuid.NewV7())
	if _, err := sharedPool.Exec(ctx,
		"INSERT INTO route_requests (id,org_id,channel,entry_code,status,flow_version_id,flow_code) VALUES ($1,$2,'voice','main','waiting',$3,'f')",
		routeID, lf.orgID, uuid.Must(uuid.NewV7())); err != nil {
		t.Fatalf("seed route: %v", err)
	}
	resID := uuid.Must(uuid.NewV7())
	if _, err := sharedPool.Exec(ctx,
		"INSERT INTO reservations (id,org_id,route_request_id,agent_id,state,expires_at) VALUES ($1,$2,$3,$4,'accepted',now()+interval '1 hour')",
		resID, lf.orgID, routeID, agentID); err != nil {
		t.Fatalf("seed reservation: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		"INSERT INTO agent_capacity_slots (org_id,agent_id,channel,slot_no,reservation_id,hold_expires_at) VALUES ($1,$2,'voice',1,$3,NULL)",
		lf.orgID, agentID, resID); err != nil {
		t.Fatalf("seed slot: %v", err)
	}
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("pre-abandon held=%d, want 1 (confirmed slot)", held)
	}

	resp, err := lf.e.AbandonRouteRequest(ctx, api.AbandonRouteRequestRequestObject{Id: api.EntityIdPath(routeID)})
	if err != nil {
		t.Fatalf("abandon: %v", err)
	}
	if _, is := resp.(api.AbandonRouteRequest200JSONResponse); !is {
		t.Fatalf("abandon resp = %T, want 200", resp)
	}
	if held := lf.heldVoice(t, agentID); held != 0 {
		t.Fatalf("post-abandon held=%d, want 0 (confirmed slot released)", held)
	}
	var resState, agentStatus string
	_ = sharedPool.QueryRow(ctx, "SELECT state FROM reservations WHERE id=$1", resID).Scan(&resState)
	_ = sharedPool.QueryRow(ctx, "SELECT status FROM agent_states WHERE agent_id=$1 AND org_id=$2", agentID, lf.orgID).Scan(&agentStatus)
	if resState != "cancelled" {
		t.Fatalf("reservation state=%q, want cancelled", resState)
	}
	if agentStatus != "WrapUp" {
		t.Fatalf("agent status=%q, want WrapUp (after-call work, not stranded Engaged)", agentStatus)
	}
}

// TestLive_InlineCapacityLostAuditPersists drives liveOfferer directly against an
// at-capacity agent (the snapshot pre-filter normally hides them; this is the
// TOCTOU-race branch) and asserts the capacity_lost decision SURVIVES the per-offer
// savepoint rollback — the audit must be recorded on the parent tx AFTER the
// rollback, not before (strict review HIGH: sp + parent share one connection).
func TestLive_InlineCapacityLostAuditPersists(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedReadyAgent(t, "agent_a", sid, 3)
	agentID := lf.agentID(t, "agent_a")
	// Occupy agent_a's only voice slot (confirmed) → at capacity.
	if _, err := sharedPool.Exec(lf.ctx,
		"INSERT INTO agent_capacity_slots (org_id,agent_id,channel,slot_no,reservation_id,hold_expires_at) VALUES ($1,$2,'voice',1,$3,NULL)",
		lf.orgID, agentID, uuid.Must(uuid.NewV7())); err != nil {
		t.Fatalf("seed held slot: %v", err)
	}
	routeID := seedRunningRoute(lf.ctx, t, lf.orgID)

	tx, err := lf.e.deps.OrgDB.BeginTx(lf.ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	off := &liveOfferer{ctx: lf.ctx, tx: tx, orgID: lf.orgID, routeID: routeID, channel: "voice", cap: NewCapacityService()}
	resID, ok, oErr := off.Offer("agent_a", 30*time.Second)
	if oErr != nil || ok || resID != "" {
		_ = tx.Rollback(lf.ctx)
		t.Fatalf("offer to at-capacity agent = (%q,%v,%v), want ('',false,nil)", resID, ok, oErr)
	}
	if err := tx.Commit(lf.ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var n int
	if err := sharedPool.QueryRow(lf.ctx,
		"SELECT count(*) FROM route_decisions WHERE org_id=$1 AND route_request_id=$2 AND decision_type='interaction_offer' AND outcome='capacity_lost'",
		lf.orgID, routeID).Scan(&n); err != nil {
		t.Fatalf("count decisions: %v", err)
	}
	if n != 1 {
		t.Fatalf("capacity_lost decisions = %d, want 1 (must survive the savepoint rollback)", n)
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
