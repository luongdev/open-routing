package flowrt

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// reservationLeaseToken reads the lease_token bound on a reservation at offer
// time — the secret a WS command must echo to pass the D5 lease fence.
func reservationLeaseToken(ctx context.Context, t *testing.T, org, resID uuid.UUID) string {
	t.Helper()
	var lt uuid.UUID
	if err := sharedPool.QueryRow(ctx,
		"SELECT lease_token FROM reservations WHERE id=$1 AND org_id=$2", resID, org).Scan(&lt); err != nil {
		t.Fatalf("read lease_token: %v", err)
	}
	return lt.String()
}

func TestExecuteAgentCommand(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	_, resID := f.seedRouteWithOffer(t)
	gr, _ := f.e.GetReservation(f.ctx, api.GetReservationRequestObject{Id: api.EntityIdPath(resID)})
	res := gr.(api.GetReservation200JSONResponse)
	orgID, agentID := uuid.UUID(res.OrgId), uuid.UUID(res.AgentId)
	lt := reservationLeaseToken(f.ctx, t, orgID, resID) // the offer-bound token the agent echoes

	// A live session for the call.
	sess := uuid.Must(uuid.NewV7())
	if _, err := f.q.CreateAgentSession(f.ctx, generated.CreateAgentSessionParams{
		OrgID: pgUUID(orgID), SessionID: pgUUID(sess), AgentID: pgUUID(agentID), GatewayID: "gw-test",
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// 1. Ownership CAS: a different agent cannot act on this reservation.
	other := uuid.Must(uuid.NewV7())
	if r, err := f.e.ExecuteAgentCommand(f.ctx, orgID, other, sess, uuid.Must(uuid.NewV7()), resID, lt, CmdAccept, "h1"); err != nil || r.Status != "not_owner" {
		t.Fatalf("other-agent accept = %q (err %v), want not_owner", r.Status, err)
	}

	// 2. Accept by the owner.
	cmd := uuid.Must(uuid.NewV7())
	r, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, cmd, resID, lt, CmdAccept, "h2")
	if err != nil || r.Status != "accepted" {
		t.Fatalf("accept = %q (err %v), want accepted", r.Status, err)
	}

	// 3. Idempotent: the SAME client_msg_id returns the cached result, no re-apply.
	r2, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, cmd, resID, lt, CmdAccept, "h2")
	if err != nil || r2.Status != "accepted" {
		t.Fatalf("replay = %q (err %v), want cached accepted", r2.Status, err)
	}

	// 4. Reused client_msg_id with a DIFFERENT payload hash is rejected.
	if r3, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, cmd, resID, lt, CmdAccept, "DIFFERENT"); err != nil || r3.Status != "reused_id" {
		t.Fatalf("reused id = %q (err %v), want reused_id", r3.Status, err)
	}

	// 5. A cached result survives session revocation: a redelivered command
	// must return its stored result, NOT session_revoked (review MED-5 — the
	// dedupe read precedes the session gate). Replay the accepted command (cmd)
	// after terminating the session.
	if _, err := f.q.TerminateAgentSession(f.ctx, generated.TerminateAgentSessionParams{OrgID: pgUUID(orgID), SessionID: pgUUID(sess)}); err != nil {
		t.Fatalf("terminate session: %v", err)
	}
	if rc, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, cmd, resID, lt, CmdAccept, "h2"); err != nil || rc.Status != "accepted" {
		t.Fatalf("cached replay after revoke = %q (err %v), want accepted", rc.Status, err)
	}

	// 6. A NEW command on a revoked session is blocked.
	if r4, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, lt, CmdComplete, "h3"); err != nil || r4.Status != "session_revoked" {
		t.Fatalf("after revoke = %q (err %v), want session_revoked", r4.Status, err)
	}
}

// TestExecuteAgentCommand_LeaseFence: a WS command must echo the offer's
// lease_token; a wrong or absent token fails closed (D5 — a stale command for a
// superseded offer can't resolve it).
func TestExecuteAgentCommand_LeaseFence(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	_, resID := f.seedRouteWithOffer(t)
	gr, _ := f.e.GetReservation(f.ctx, api.GetReservationRequestObject{Id: api.EntityIdPath(resID)})
	res := gr.(api.GetReservation200JSONResponse)
	orgID, agentID := uuid.UUID(res.OrgId), uuid.UUID(res.AgentId)
	sess := uuid.Must(uuid.NewV7())
	if _, err := f.q.CreateAgentSession(f.ctx, generated.CreateAgentSessionParams{
		OrgID: pgUUID(orgID), SessionID: pgUUID(sess), AgentID: pgUUID(agentID), GatewayID: "gw-test",
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	wrong := uuid.Must(uuid.NewV7()).String()
	if r, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, wrong, CmdAccept, "h1"); err != nil || r.Status != "lease_mismatch" {
		t.Fatalf("wrong-lease accept = %q (err %v), want lease_mismatch", r.Status, err)
	}
	if r, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, "", CmdAccept, "h2"); err != nil || r.Status != "lease_mismatch" {
		t.Fatalf("empty-lease accept = %q (err %v), want lease_mismatch", r.Status, err)
	}
	lt := reservationLeaseToken(f.ctx, t, orgID, resID)
	if r, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, lt, CmdAccept, "h3"); err != nil || r.Status != "accepted" {
		t.Fatalf("correct-lease accept = %q (err %v), want accepted", r.Status, err)
	}
}

// TestExecuteAgentCommand_PostAcquireConflictReleasesRoute pins review BLOCK-1: a
// domain conflict AFTER AcquireRouteForRun flips the route to 'running' must not
// strand it there. Here the offer is force-expired so AcceptReservation returns
// ErrNoRows (conflict) only after the run-lock is taken; the savepoint rollback
// must restore the route to 'waiting'.
func TestExecuteAgentCommand_PostAcquireConflictReleasesRoute(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	routeID, resID := f.seedRouteWithOffer(t)
	gr, _ := f.e.GetReservation(f.ctx, api.GetReservationRequestObject{Id: api.EntityIdPath(resID)})
	res := gr.(api.GetReservation200JSONResponse)
	orgID, agentID := uuid.UUID(res.OrgId), uuid.UUID(res.AgentId)
	lt := reservationLeaseToken(f.ctx, t, orgID, resID)

	sess := uuid.Must(uuid.NewV7())
	if _, err := f.q.CreateAgentSession(f.ctx, generated.CreateAgentSessionParams{
		OrgID: pgUUID(orgID), SessionID: pgUUID(sess), AgentID: pgUUID(agentID), GatewayID: "gw-test",
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Force the offer past expiry while leaving state='offered' and the route's
	// current_reservation_id pointing at it — so the conflict lands AFTER
	// AcquireRouteForRun, on the AcceptReservation expires_at guard.
	if _, err := sharedPool.Exec(f.ctx,
		"UPDATE reservations SET expires_at = NOW() - interval '1 hour' WHERE id=$1 AND org_id=$2", resID, orgID); err != nil {
		t.Fatalf("expire reservation: %v", err)
	}

	r, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, lt, CmdAccept, "hx")
	if err != nil || r.Status != "conflict" {
		t.Fatalf("expired accept = %q (err %v), want conflict", r.Status, err)
	}

	var status string
	if err := sharedPool.QueryRow(f.ctx, "SELECT status FROM route_requests WHERE id=$1 AND org_id=$2", routeID, orgID).Scan(&status); err != nil {
		t.Fatalf("route status: %v", err)
	}
	if status != "waiting" {
		t.Fatalf("route status = %q after post-acquire conflict, want 'waiting' (BLOCK-1: the run-lock flip must be rolled back, not stranded)", status)
	}
}
