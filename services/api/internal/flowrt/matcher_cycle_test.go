package flowrt

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// newMatcherEndpoints builds a matcher-enabled Endpoints sharing the live
// fixture's OrgDB/org so seeding via lf.q is visible to the cycle.
func newMatcherEndpoints(lf *liveFixture) *Endpoints {
	return New(Deps{
		OrgDB: lf.e.deps.OrgDB, Cache: lf.e.deps.Cache, Logger: lf.e.deps.Logger,
		Presence: lf.mem, Capacity: NewCapacityService(), MatcherEnabled: true,
	})
}

func routeStatus(ctx context.Context, t *testing.T, org, routeID uuid.UUID) (string, bool) {
	t.Helper()
	var status string
	var hasRes bool
	if err := sharedPool.QueryRow(ctx,
		"SELECT status, current_reservation_id IS NOT NULL FROM route_requests WHERE id=$1 AND org_id=$2",
		routeID, org).Scan(&status, &hasRes); err != nil {
		t.Fatalf("read route status: %v", err)
	}
	return status, hasRes
}

func decisionCount(ctx context.Context, t *testing.T, org, routeID uuid.UUID, outcome string) int {
	t.Helper()
	var n int
	if err := sharedPool.QueryRow(ctx,
		"SELECT count(*) FROM route_decisions WHERE org_id=$1 AND route_request_id=$2 AND outcome=$3",
		org, routeID, outcome).Scan(&n); err != nil {
		t.Fatalf("count decisions: %v", err)
	}
	return n
}

// TestMatcher_PullOffersToConnectedAgent: a route that parked waiting_match (no
// agent at create) is pulled onto an agent the moment one is Ready + connected —
// the availability-driven half of the matcher. Asserts the full offer effect:
// reservation, capacity hold, route→waiting, and an `offered` decision audit row.
func TestMatcher_PullOffersToConnectedAgent(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3) // Ready but NOT connected at create
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
	if st, _ := routeStatus(lf.ctx, t, lf.orgID, routeID); st != "waiting_match" {
		t.Fatalf("route parked as %q, want waiting_match", st)
	}

	// The agent connects → the matcher pulls the waiting route onto it.
	agentID := lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(context.Background(), lf.orgID, agentID, "sess-1")
	n, err := me.RunMatchCycle(lf.ctx, lf.orgID, "matcher-1")
	if err != nil {
		t.Fatalf("match cycle: %v", err)
	}
	if n != 1 {
		t.Fatalf("cycle offered %d, want 1", n)
	}
	rs := lf.reservations(t, routeID)
	if len(rs) != 1 || rs[0].State != api.ReservationStateOffered || uuid.UUID(rs[0].AgentId) != agentID {
		t.Fatalf("reservations = %+v, want 1 offered to agent_a", rs)
	}
	if held := lf.heldVoice(t, agentID); held != 1 {
		t.Fatalf("held=%d after offer, want 1 (capacity slot held)", held)
	}
	if st, hasRes := routeStatus(lf.ctx, t, lf.orgID, routeID); st != "waiting" || !hasRes {
		t.Fatalf("route after offer = %q hasReservation=%v, want waiting + current_reservation_id set", st, hasRes)
	}
	if c := decisionCount(lf.ctx, t, lf.orgID, routeID, "offered"); c != 1 {
		t.Fatalf("offered decisions = %d, want 1 (audit row)", c)
	}
}

// TestMatcher_PullNoDoubleAssign: one waiting route, two connected Ready agents →
// the cycle offers it to exactly ONE (the claim flips status='offering' so the
// second agent's claim finds nothing).
func TestMatcher_PullNoDoubleAssign(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3)
	lf.seedReadyAgent(t, "agent_b", sid, 2)
	flowID := lf.seedFlow(t, "flow_q", simGraph(t))
	if _, err := me.PublishFlow(lf.ctx, api.PublishFlowRequestObject{
		Id: api.EntityIdPath(flowID), Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 1},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// Park the route (neither agent connected at create), then connect both.
	resp, err := me.CreateRouteRequest(lf.ctx, api.CreateRouteRequestRequestObject{
		Body: &api.CreateRouteRequest{Channel: "voice", EntryCode: "main"},
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	routeID := uuid.UUID(resp.(api.CreateRouteRequest201JSONResponse).Id)
	_ = lf.mem.Renew(context.Background(), lf.orgID, lf.agentID(t, "agent_a"), "sa")
	_ = lf.mem.Renew(context.Background(), lf.orgID, lf.agentID(t, "agent_b"), "sb")

	n, err := me.RunMatchCycle(lf.ctx, lf.orgID, "matcher-1")
	if err != nil {
		t.Fatalf("match cycle: %v", err)
	}
	if n != 1 {
		t.Fatalf("cycle offered %d, want exactly 1 (no double-assign)", n)
	}
	if rs := lf.reservations(t, routeID); len(rs) != 1 {
		t.Fatalf("route got %d reservations, want exactly 1", len(rs))
	}
}

// TestMatcher_AgingOvertakesPriority: an aged low-priority route outranks a fresh
// high-priority one when the age bonus crosses the priority band — the SQL-computed
// score (priority*W_p + age*W_age) prevents bounded-prefix starvation (plan rev2
// BLOCK; codex HIGH test: the winner is OUTSIDE the priority prefix).
func TestMatcher_AgingOvertakesPriority(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	sid := lf.seedSkillID(t, "skill_x")
	lf.seedReadyAgent(t, "agent_a", sid, 1)
	agentID := lf.agentID(t, "agent_a")
	_ = lf.mem.Renew(context.Background(), lf.orgID, agentID, "sess-1")
	q := generated.New(sharedPool)

	// Fresh, high priority (score = 5*100 + ~0 = 500).
	hi := seedRunningRoute(lf.ctx, t, lf.orgID)
	if _, err := q.EnqueueRouteForMatch(lf.ctx, generated.EnqueueRouteForMatchParams{
		RouteRequestID: pgUUID(hi), OrgID: pgUUID(lf.orgID), Priority: 5, RequiredSkills: []string{}, ResumeCursor: []byte("{}"),
	}); err != nil {
		t.Fatalf("enqueue hi: %v", err)
	}
	// Aged 600s, low priority (score = 0*100 + 600*1 = 600 > 500 → it wins).
	lo := seedRunningRoute(lf.ctx, t, lf.orgID)
	if _, err := q.EnqueueRouteForMatch(lf.ctx, generated.EnqueueRouteForMatchParams{
		RouteRequestID: pgUUID(lo), OrgID: pgUUID(lf.orgID), Priority: 0, RequiredSkills: []string{}, ResumeCursor: []byte("{}"),
	}); err != nil {
		t.Fatalf("enqueue lo: %v", err)
	}
	if _, err := sharedPool.Exec(lf.ctx,
		"UPDATE route_requests SET waiting_since = now() - interval '600 seconds' WHERE id=$1 AND org_id=$2", lo, lf.orgID); err != nil {
		t.Fatalf("age lo: %v", err)
	}

	if _, err := me.RunMatchCycle(lf.ctx, lf.orgID, "matcher-1"); err != nil {
		t.Fatalf("match cycle: %v", err)
	}
	if rs := lf.reservations(t, lo); len(rs) != 1 {
		t.Fatalf("aged low-priority route got %d offers, want 1 (aging overtakes)", len(rs))
	}
	if rs := lf.reservations(t, hi); len(rs) != 0 {
		t.Fatalf("fresh high-priority route got %d offers, want 0 (lost to the aged route)", len(rs))
	}
}

// TestMatcher_RONAMissedAgentExcluded: an agent in a RONA cooldown (missed, not
// yet expired) is omitted from the matcher's available pool; once the cooldown
// expires they are routable again — the matcher's READ side of RONA.
func TestMatcher_RONAMissedAgentExcluded(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedReadyAgent(t, "agent_a", sid, 3)
	agentID := lf.agentID(t, "agent_a")
	q := generated.New(sharedPool)
	params := generated.ListAvailableAgentsForMatchParams{OrgID: pgUUID(lf.orgID), Limit: 10}

	has := func() bool {
		rows, err := q.ListAvailableAgentsForMatch(lf.ctx, params)
		if err != nil {
			t.Fatalf("list available: %v", err)
		}
		for _, r := range rows {
			if apiUUID(r.AgentID) == agentID {
				return true
			}
		}
		return false
	}

	if !has() {
		t.Fatalf("Ready agent should be available before any RONA")
	}
	// Cooldown in the future → excluded.
	if _, err := q.MarkAgentMissed(lf.ctx, generated.MarkAgentMissedParams{
		OrgID: pgUUID(lf.orgID), AgentID: pgUUID(agentID),
		StateExpiresAt: ts(time.Now().Add(time.Minute)), LastReadyAt: ts(time.Now().Add(-time.Minute)),
	}); err != nil {
		t.Fatalf("mark missed: %v", err)
	}
	if has() {
		t.Fatalf("missed agent (cooldown active) must be excluded from the pool")
	}
	// Cooldown elapsed → routable again.
	if _, err := q.MarkAgentMissed(lf.ctx, generated.MarkAgentMissedParams{
		OrgID: pgUUID(lf.orgID), AgentID: pgUUID(agentID),
		StateExpiresAt: ts(time.Now().Add(-time.Second)), LastReadyAt: ts(time.Now().Add(-time.Minute)),
	}); err != nil {
		t.Fatalf("expire cooldown: %v", err)
	}
	if !has() {
		t.Fatalf("agent with an expired cooldown should be routable again")
	}
}

// TestMatcher_RunMatcherPullsPerOrg drives the full cmd/runtime tick (RunMatcher):
// a parked route + a connected Ready agent → the per-org pull offers it.
func TestMatcher_RunMatcherPullsPerOrg(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3)
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
	_ = lf.mem.Renew(context.Background(), lf.orgID, lf.agentID(t, "agent_a"), "sess-1")

	if _, err := me.RunMatcher(lf.ctx, sharedPool, "runtime-1", time.Now()); err != nil {
		t.Fatalf("run matcher: %v", err)
	}
	if rs := lf.reservations(t, routeID); len(rs) != 1 {
		t.Fatalf("after RunMatcher route got %d reservations, want 1 (per-org pull)", len(rs))
	}
}

// TestMatcher_RunMatcherSLAFallback: a route whose match_deadline has passed is
// given up by the SLA sweep → resumed with no_candidate → takes the reservation
// node's fallback port to a terminal end (no offer).
func TestMatcher_RunMatcherSLAFallback(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf)
	sid := lf.seedSkillID(t, "skill_es")
	lf.seedQueue(t, "queue_vip")
	lf.seedReadyAgent(t, "agent_a", sid, 3) // exists but never connects → stays parked
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
	if st, _ := routeStatus(lf.ctx, t, lf.orgID, routeID); st != "waiting_match" {
		t.Fatalf("route parked as %q, want waiting_match", st)
	}
	// Force the SLA deadline into the past.
	if _, err := sharedPool.Exec(lf.ctx,
		"UPDATE route_requests SET match_deadline = now() - interval '1 second' WHERE id=$1 AND org_id=$2", routeID, lf.orgID); err != nil {
		t.Fatalf("expire deadline: %v", err)
	}

	if _, err := me.RunMatcher(lf.ctx, sharedPool, "runtime-1", time.Now()); err != nil {
		t.Fatalf("run matcher: %v", err)
	}
	// The route ran the no_candidate fallback to its terminal end; no offer made.
	st, _ := routeStatus(lf.ctx, t, lf.orgID, routeID)
	if st != "completed" {
		t.Fatalf("SLA-expired route status = %q, want completed (no_candidate → fallback → end)", st)
	}
	if rs := lf.reservations(t, routeID); len(rs) != 0 {
		t.Fatalf("SLA-expired route got %d reservations, want 0 (gave up, no offer)", len(rs))
	}
}
