package flowrt

import (
	"testing"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

func TestExecuteAgentCommand(t *testing.T) {
	f := newFixture(t)
	if f == nil {
		return
	}
	_, resID := f.seedRouteWithOffer(t)
	gr, _ := f.e.GetReservation(f.ctx, api.GetReservationRequestObject{Id: api.EntityIdPath(resID)})
	res := gr.(api.GetReservation200JSONResponse)
	orgID, agentID := uuid.UUID(res.OrgId), uuid.UUID(res.AgentId)

	// A live session for the call.
	sess := uuid.Must(uuid.NewV7())
	if _, err := f.q.CreateAgentSession(f.ctx, generated.CreateAgentSessionParams{
		OrgID: pgUUID(orgID), SessionID: pgUUID(sess), AgentID: pgUUID(agentID), GatewayID: "gw-test",
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// 1. Ownership CAS: a different agent cannot act on this reservation.
	other := uuid.Must(uuid.NewV7())
	if r, err := f.e.ExecuteAgentCommand(f.ctx, orgID, other, sess, uuid.Must(uuid.NewV7()), resID, CmdAccept, "h1"); err != nil || r.Status != "not_owner" {
		t.Fatalf("other-agent accept = %q (err %v), want not_owner", r.Status, err)
	}

	// 2. Accept by the owner.
	cmd := uuid.Must(uuid.NewV7())
	r, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, cmd, resID, CmdAccept, "h2")
	if err != nil || r.Status != "accepted" {
		t.Fatalf("accept = %q (err %v), want accepted", r.Status, err)
	}

	// 3. Idempotent: the SAME client_msg_id returns the cached result, no re-apply.
	r2, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, cmd, resID, CmdAccept, "h2")
	if err != nil || r2.Status != "accepted" {
		t.Fatalf("replay = %q (err %v), want cached accepted", r2.Status, err)
	}

	// 4. Reused client_msg_id with a DIFFERENT payload hash is rejected.
	if r3, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, cmd, resID, CmdAccept, "DIFFERENT"); err != nil || r3.Status != "reused_id" {
		t.Fatalf("reused id = %q (err %v), want reused_id", r3.Status, err)
	}

	// 5. Session revocation blocks further commands.
	if _, err := f.q.TerminateAgentSession(f.ctx, generated.TerminateAgentSessionParams{OrgID: pgUUID(orgID), SessionID: pgUUID(sess)}); err != nil {
		t.Fatalf("terminate session: %v", err)
	}
	if r4, err := f.e.ExecuteAgentCommand(f.ctx, orgID, agentID, sess, uuid.Must(uuid.NewV7()), resID, CmdComplete, "h3"); err != nil || r4.Status != "session_revoked" {
		t.Fatalf("after revoke = %q (err %v), want session_revoked", r4.Status, err)
	}
}
