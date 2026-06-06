package flowrt

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/metrics"
)

func metricsReclaims() int64 { return metrics.MatcherReclaims.Value() }

// driveToAcceptedCall runs the live loop up to an accepted reservation holding a
// confirmed capacity slot, returning the route + reservation + agent ids and the
// session the agent accepted on. Mirrors the first half of TestE2E_*.
func driveToAcceptedCall(t *testing.T, lf *liveFixture, me *Endpoints) (routeID, resID, agentID, sessionID uuid.UUID) {
	t.Helper()
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3)
	flowID := lf.seedFlow(t, "flow_reclaim", simGraph(t))
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
	if len(rs) != 1 {
		t.Fatalf("want 1 reservation, got %d", len(rs))
	}
	resID = uuid.UUID(rs[0].Id)

	lt := reservationLeaseToken(lf.ctx, t, lf.orgID, resID)
	sessionID = uuid.Must(uuid.NewV7())
	if _, err := lf.q.CreateAgentSession(lf.ctx, generated.CreateAgentSessionParams{
		OrgID: pgUUID(lf.orgID), SessionID: pgUUID(sessionID), AgentID: pgUUID(agentID), GatewayID: "gw",
	}); err != nil {
		t.Fatalf("session: %v", err)
	}
	if r, err := me.ExecuteAgentCommand(lf.ctx, lf.orgID, agentID, sessionID, uuid.Must(uuid.NewV7()), resID, lt, CmdAccept, "h1"); err != nil || r.Status != "accepted" {
		t.Fatalf("accept = %q (err %v), want accepted", r.Status, err)
	}
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("after accept held=%d, want 1", held)
	}
	return routeID, resID, agentID, sessionID
}

func ageSession(t *testing.T, org, sessionID uuid.UUID) {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(),
		"UPDATE agent_sessions SET last_seen_at = NOW() - INTERVAL '10 minutes' WHERE org_id=$1 AND session_id=$2",
		org, sessionID); err != nil {
		t.Fatalf("age session: %v", err)
	}
}

// TestReclaim_FreshSessionNotReclaimed: an agent on a live call whose session is
// still fresh (recent heartbeat) must NOT be reclaimed — the slot stays held.
func TestReclaim_FreshSessionNotReclaimed(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	_, resID, agentID, _ := driveToAcceptedCall(t, lf, me)

	if _, err := me.RunMatcher(lf.ctx, sharedPool, "rt", time.Now()); err != nil {
		t.Fatalf("RunMatcher: %v", err)
	}
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("fresh-session call reclaimed: held=%d, want 1", held)
	}
	if st := reservationState(t, lf.orgID, resID); st != "accepted" {
		t.Fatalf("reservation = %q, want still accepted", st)
	}
}

// TestReclaim_VanishedAgentFreesSlot: once the agent has been unseen past the
// grace, the matcher tick reclaims the stuck slot — reservation cancelled
// (agent_lost), capacity freed, agent moved out of Engaged, event recorded.
func TestReclaim_VanishedAgentFreesSlot(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	routeID, resID, agentID, sessionID := driveToAcceptedCall(t, lf, me)

	// Agent vanishes: its only session goes stale beyond reclaimGrace.
	ageSession(t, lf.orgID, sessionID)
	lf.mem.Expire(lf.orgID, agentID)

	before := metricsReclaims()
	if _, err := me.RunMatcher(lf.ctx, sharedPool, "rt", time.Now()); err != nil {
		t.Fatalf("RunMatcher: %v", err)
	}

	if held := lf.heldVoice(t, agentID); held != 0 {
		t.Fatalf("after reclaim held=%d, want 0 (slot freed)", held)
	}
	if st := reservationState(t, lf.orgID, resID); st != "cancelled" {
		t.Fatalf("reservation = %q, want cancelled", st)
	}
	if reason := reservationReason(t, lf.orgID, resID); reason != "agent_lost" {
		t.Fatalf("reservation reason = %q, want agent_lost", reason)
	}
	var agentStatus string
	_ = sharedPool.QueryRow(lf.ctx, "SELECT status FROM agent_states WHERE agent_id=$1 AND org_id=$2", agentID, lf.orgID).Scan(&agentStatus)
	if agentStatus != "WrapUp" {
		t.Fatalf("agent status=%q, want WrapUp", agentStatus)
	}
	if !routeHasEvent(t, lf.orgID, routeID, "route.agent_lost") {
		t.Fatal("missing route.agent_lost event")
	}
	if got := metricsReclaims(); got <= before {
		t.Fatalf("reclaim metric not incremented: %d → %d", before, got)
	}

	// Idempotent: a second tick finds nothing to reclaim.
	if _, err := me.RunMatcher(lf.ctx, sharedPool, "rt", time.Now()); err != nil {
		t.Fatalf("second RunMatcher: %v", err)
	}
	if held := lf.heldVoice(t, agentID); held != 0 {
		t.Fatalf("second tick changed held to %d, want 0", held)
	}
}

func reservationState(t *testing.T, org, resID uuid.UUID) string {
	t.Helper()
	var st string
	if err := sharedPool.QueryRow(context.Background(), "SELECT state FROM reservations WHERE id=$1 AND org_id=$2", resID, org).Scan(&st); err != nil {
		t.Fatalf("read reservation state: %v", err)
	}
	return st
}

func reservationReason(t *testing.T, org, resID uuid.UUID) string {
	t.Helper()
	var reason *string
	if err := sharedPool.QueryRow(context.Background(), "SELECT reason FROM reservations WHERE id=$1 AND org_id=$2", resID, org).Scan(&reason); err != nil {
		t.Fatalf("read reservation reason: %v", err)
	}
	if reason == nil {
		return ""
	}
	return *reason
}

func routeHasEvent(t *testing.T, org, routeID uuid.UUID, typ string) bool {
	t.Helper()
	var n int
	if err := sharedPool.QueryRow(context.Background(),
		"SELECT count(*) FROM runtime_events WHERE org_id=$1 AND route_request_id=$2 AND type=$3", org, routeID, typ).Scan(&n); err != nil {
		t.Fatalf("count events: %v", err)
	}
	return n > 0
}
