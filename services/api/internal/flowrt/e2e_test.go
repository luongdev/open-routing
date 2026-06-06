package flowrt

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// TestE2E_MatcherPullToComplete exercises the whole live loop with real
// Endpoints: a route parks waiting_match (no agent at create) → the matcher pulls
// it onto a newly-connected Ready agent (durable offer + lease + held slot) → the
// agent accepts echoing the lease (the D5 fence passes) → the flow runs to a
// terminal → the agent completes → the capacity slot frees and the agent goes
// WrapUp. This is the matcher→command→capacity chain no single unit test covers.
func TestE2E_MatcherPullToComplete(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3) // Ready but NOT connected at create → parks
	flowID := lf.seedFlow(t, "flow_e2e", simGraph(t))
	if _, err := me.PublishFlow(lf.ctx, api.PublishFlowRequestObject{
		Id: api.EntityIdPath(flowID), Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 1},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// 1. Create with no live agent → waiting_match.
	resp, err := me.CreateRouteRequest(lf.ctx, api.CreateRouteRequestRequestObject{Body: &api.CreateRouteRequest{Channel: "voice", EntryCode: "main"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	routeID := uuid.UUID(resp.(api.CreateRouteRequest201JSONResponse).Id)
	if st, _ := routeStatus(lf.ctx, t, lf.orgID, routeID); st != "waiting_match" {
		t.Fatalf("parked as %q, want waiting_match", st)
	}

	// 2. Agent connects → the matcher pulls the route onto it.
	agentID := lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(context.Background(), lf.orgID, agentID, "sess-1")
	if n, err := me.RunMatchCycle(lf.ctx, lf.orgID, "matcher-1"); err != nil || n != 1 {
		t.Fatalf("match cycle n=%d err=%v, want 1 offer", n, err)
	}
	rs := lf.reservations(t, routeID)
	if len(rs) != 1 || rs[0].State != api.ReservationStateOffered {
		t.Fatalf("reservations = %+v, want 1 offered", rs)
	}
	resID := uuid.UUID(rs[0].Id)
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("after offer held=%d, want 1", held)
	}

	// 3. Accept over the command path, echoing the offer's lease_token.
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
	// simGraph: accepted → end → the route runs to a terminal; the slot is confirmed.
	if st, _ := routeStatus(lf.ctx, t, lf.orgID, routeID); st != "completed" {
		t.Fatalf("route after accept = %q, want completed (accepted→end)", st)
	}
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("after accept held=%d, want 1 (confirmed for the call)", held)
	}

	// 4. Complete → slot freed, agent WrapUp.
	if r, err := me.ExecuteAgentCommand(lf.ctx, lf.orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, lt, CmdComplete, "h2"); err != nil || r.Status != "completed" {
		t.Fatalf("complete = %q (err %v), want completed", r.Status, err)
	}
	if held := lf.heldVoice(t, agentID); held != 0 {
		t.Fatalf("after complete held=%d, want 0 (slot released)", held)
	}
	var agentStatus string
	_ = sharedPool.QueryRow(lf.ctx, "SELECT status FROM agent_states WHERE agent_id=$1 AND org_id=$2", agentID, lf.orgID).Scan(&agentStatus)
	if agentStatus != "WrapUp" {
		t.Fatalf("agent status=%q, want WrapUp", agentStatus)
	}
}
