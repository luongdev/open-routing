package flowrt

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// driveToParkedCall is driveToAcceptedCall with a flow that PARKS after accept (a
// wait node), so the route stays non-terminal during the call — the realistic
// live-call shape reassignment operates on (vs simGraph which completes on accept).
func driveToParkedCall(t *testing.T, lf *liveFixture, me *Endpoints) (routeID, resID, agentID, agentBID uuid.UUID) {
	t.Helper()
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3)
	// A second Ready+skilled agent, NOT connected at create (so agent_a wins the
	// first offer). The reassignment re-match test connects it after a drop.
	lf.seedReadyAgent(t, "agent_b", sid, 3)
	agentBID = lf.agentID(t, "agent_b")
	flowID := lf.seedFlow(t, "flow_reassign", simGraphWithWait(t))
	if _, err := me.PublishFlow(lf.ctx, api.PublishFlowRequestObject{
		Id: api.EntityIdPath(flowID), Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 1},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	resp, err := me.CreateRouteRequest(lf.ctx, api.CreateRouteRequestRequestObject{Body: &api.CreateRouteRequest{Channel: "voice", EntryCode: "main"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	routeID = uuid.UUID(resp.(api.CreateRouteRequest201JSONResponse).Id)
	agentID = lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(lf.ctx, lf.orgID, agentID, "sess-1")
	if n, err := me.RunMatchCycle(lf.ctx, lf.orgID, "matcher-1"); err != nil || n != 1 {
		t.Fatalf("match cycle n=%d err=%v, want 1 offer", n, err)
	}
	rs := lf.reservations(t, routeID)
	resID = uuid.UUID(rs[0].Id)
	lt := reservationLeaseToken(lf.ctx, t, lf.orgID, resID)
	sess := uuid.Must(uuid.NewV7())
	if _, err := lf.q.CreateAgentSession(lf.ctx, generated.CreateAgentSessionParams{
		OrgID: pgUUID(lf.orgID), SessionID: pgUUID(sess), AgentID: pgUUID(agentID), GatewayID: "gw",
	}); err != nil {
		t.Fatalf("session: %v", err)
	}
	if r, err := me.ExecuteAgentCommand(lf.ctx, lf.orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, lt, CmdAccept, "h1"); err != nil || r.Status != "accepted" {
		t.Fatalf("accept = %q (err %v), want accepted", r.Status, err)
	}
	if st, _ := routeStatus(lf.ctx, t, lf.orgID, routeID); st != "waiting" {
		t.Fatalf("route after accept = %q, want waiting (parked at the call node)", st)
	}
	return routeID, resID, agentID, agentBID
}

func reassignCount(t *testing.T, org, routeID uuid.UUID) int {
	t.Helper()
	var n int
	if err := sharedPool.QueryRow(context.Background(),
		"SELECT reassign_count FROM route_requests WHERE id=$1 AND org_id=$2", routeID, org).Scan(&n); err != nil {
		t.Fatalf("read reassign_count: %v", err)
	}
	return n
}

// bindHandle binds an adapter handle on the accepted reservation so the sink's
// ownership fence (handle-match) passes.
func bindHandle(t *testing.T, lf *liveFixture, resID uuid.UUID, handle string) {
	t.Helper()
	if _, err := lf.q.SetReservationAdapterHandle(lf.ctx, generated.SetReservationAdapterHandleParams{
		ID: pgUUID(resID), OrgID: pgUUID(lf.orgID), AdapterHandle: &handle,
	}); err != nil {
		t.Fatalf("bind handle: %v", err)
	}
}

// TestReassign_DisconnectRequeues: an adapter `disconnected` on a live accepted
// call re-queues the interaction (waiting_match, reassign_count++), cancels the
// dropped reservation (reason 'reassigned'), and frees the slot — instead of
// abandoning the route (the v0.4 W4 reclaim→reassign path; wires the W0 FSM).
func TestReassign_DisconnectRequeues(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	routeID, resID, agentID, _ := driveToParkedCall(t, lf, me)
	bindHandle(t, lf, resID, "room-x")

	if err := me.OnAssignmentEvent(lf.ctx, adapter.AssignmentEvent{
		Type: adapter.EventDisconnected, Handle: "room-x", ReservationID: resID.String(),
	}); err != nil {
		t.Fatalf("OnAssignmentEvent: %v", err)
	}

	if st, _ := routeStatus(lf.ctx, t, lf.orgID, routeID); st != "waiting_match" {
		t.Fatalf("route = %q, want waiting_match (re-queued)", st)
	}
	if c := reassignCount(t, lf.orgID, routeID); c != 1 {
		t.Fatalf("reassign_count = %d, want 1", c)
	}
	if st := reservationState(t, lf.orgID, resID); st != "cancelled" {
		t.Fatalf("dropped reservation = %q, want cancelled", st)
	}
	if r := reservationReason(t, lf.orgID, resID); r != "reassigned" {
		t.Fatalf("reservation reason = %q, want reassigned", r)
	}
	if held := lf.heldVoice(t, agentID); held != 0 {
		t.Fatalf("held=%d after reassign, want 0 (slot freed)", held)
	}
	if !routeHasEvent(t, lf.orgID, routeID, "route.reassigned") {
		t.Fatal("missing route.reassigned event")
	}
}

// TestReassign_RematchesToAnotherAgent: after a drop re-queues the interaction, a
// matcher cycle offers it to a DIFFERENT connected agent (the dropped one is
// excluded). (Driving the replacement accept fully through the call is the
// documented W3 gap — the re-matched agent currently resumes the post-accept wait
// cursor, not the reservation node.)
func TestReassign_RematchesToAnotherAgent(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	routeID, resID, agentA, agentB := driveToParkedCall(t, lf, me)
	bindHandle(t, lf, resID, "room-r")

	if err := me.OnAssignmentEvent(lf.ctx, adapter.AssignmentEvent{
		Type: adapter.EventDisconnected, Handle: "room-r", ReservationID: resID.String(),
	}); err != nil {
		t.Fatalf("OnAssignmentEvent: %v", err)
	}
	// agent_b connects + the matcher re-offers the re-queued route to it.
	_ = lf.mem.Renew(lf.ctx, lf.orgID, agentB, "sess-b")
	if n, err := me.RunMatchCycle(lf.ctx, lf.orgID, "matcher-2"); err != nil || n != 1 {
		t.Fatalf("re-match cycle n=%d err=%v, want 1 offer to the new agent", n, err)
	}
	// A fresh offered reservation exists for agent_b (not agent_a).
	var offered bool
	for _, r := range lf.reservations(t, routeID) {
		if r.State == api.ReservationStateOffered {
			if uuid.UUID(r.AgentId) != agentB {
				t.Fatalf("re-offer went to %v, want the new agent %v (not dropped %v)", uuid.UUID(r.AgentId), agentB, agentA)
			}
			offered = true
		}
	}
	if !offered {
		t.Fatal("no fresh offer after reassign re-match")
	}
}

// TestReassign_ExhaustedAbandons: once the hop cap is hit, a further drop abandons
// the route instead of re-queueing forever.
func TestReassign_ExhaustedAbandons(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	routeID, resID, _, _ := driveToParkedCall(t, lf, me)
	bindHandle(t, lf, resID, "room-y")
	// Pin the route at the hop cap so the next drop gives up.
	if _, err := sharedPool.Exec(lf.ctx,
		"UPDATE route_requests SET reassign_count=$1 WHERE id=$2 AND org_id=$3", maxReassignHops, routeID, lf.orgID); err != nil {
		t.Fatalf("seed reassign_count: %v", err)
	}

	if err := me.OnAssignmentEvent(lf.ctx, adapter.AssignmentEvent{
		Type: adapter.EventDisconnected, Handle: "room-y", ReservationID: resID.String(),
	}); err != nil {
		t.Fatalf("OnAssignmentEvent: %v", err)
	}
	if st, _ := routeStatus(lf.ctx, t, lf.orgID, routeID); st != "cancelled" {
		t.Fatalf("route = %q, want cancelled (reassign exhausted → abandon)", st)
	}
	if !routeHasEvent(t, lf.orgID, routeID, "route.reassign_ended") {
		t.Fatal("missing route.reassign_ended event")
	}
}

// TestReassign_CallerAbandonStillTearsDown: caller_abandoned (the interaction is
// over) still abandons — only AGENT drops reassign.
func TestReassign_CallerAbandonStillTearsDown(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	routeID, resID, _, _ := driveToParkedCall(t, lf, me)
	bindHandle(t, lf, resID, "room-z")

	if err := me.OnAssignmentEvent(lf.ctx, adapter.AssignmentEvent{
		Type: adapter.EventCallerLeft, Handle: "room-z", ReservationID: resID.String(),
	}); err != nil {
		t.Fatalf("OnAssignmentEvent: %v", err)
	}
	if st, _ := routeStatus(lf.ctx, t, lf.orgID, routeID); st != "cancelled" {
		t.Fatalf("route = %q, want cancelled (caller abandoned → teardown, not reassign)", st)
	}
	if c := reassignCount(t, lf.orgID, routeID); c != 0 {
		t.Fatalf("reassign_count = %d, want 0 (caller abandon must not reassign)", c)
	}
}
