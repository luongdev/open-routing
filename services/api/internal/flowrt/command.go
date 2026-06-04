package flowrt

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// command.go is the v0.3 W2 live-path command service: an agent's WS
// accept/reject/complete runs idempotently in ONE transaction that owns the
// dedupe row AND the reservation transition (review BLOCK-1 — the WS layer must
// NOT wrap the tx-owning HTTP handlers). It adds two things the test-double HTTP
// path doesn't need: per-command session-revocation + agent-ownership CAS (an
// agent can't accept another agent's offered reservation by guessing the id —
// review HIGH). The HTTP handlers stay as-is for sim/CI/Route-Tester.

type AgentCommandKind string

const (
	CmdAccept   AgentCommandKind = "reservation.accept"
	CmdReject   AgentCommandKind = "reservation.reject"
	CmdComplete AgentCommandKind = "reservation.complete"
)

// AgentCommandResult is the canonical, idempotently-stored outcome of a command.
// Status is a stable string the gateway maps to an ack frame.
type AgentCommandResult struct {
	Status        string `json:"status"` // accepted|rejected|completed|conflict|not_found|not_owner|session_revoked|reused_id
	ReservationID string `json:"reservation_id"`
}

// ExecuteAgentCommand runs one agent command at-most-once. A redelivered
// client_msg_id returns the original stored result without re-running; a reused id
// with a different payload hash is rejected. A "conflict"/"not_owner" outcome is a
// committed result (so retries are stable), not a rolled-back error; only real
// infra failures return a non-nil error.
func (e *Endpoints) ExecuteAgentCommand(ctx context.Context, orgID, agentID, sessionID, clientMsgID, resID uuid.UUID, kind AgentCommandKind, reqHash string) (AgentCommandResult, error) {
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return AgentCommandResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	// Dedupe claim FIRST so a redelivered command returns its stored result even
	// after the session was revoked (review MED-5: the session gate is for NEW
	// commands only — a cached replay must not flip to session_revoked).
	// ErrNoRows ⇒ this client_msg_id already exists → return its stored result
	// (waiting under FOR UPDATE if a concurrent duplicate is still running).
	if _, err := qtx.BeginCommand(ctx, generated.BeginCommandParams{
		OrgID: pgUUID(orgID), AgentID: pgUUID(agentID), ClientMsgID: pgUUID(clientMsgID),
		CommandType: string(kind), RequestHash: reqHash,
	}); errors.Is(err, pgx.ErrNoRows) {
		existing, gErr := qtx.GetCommandForUpdate(ctx, generated.GetCommandForUpdateParams{OrgID: pgUUID(orgID), AgentID: pgUUID(agentID), ClientMsgID: pgUUID(clientMsgID)})
		if gErr != nil {
			return AgentCommandResult{}, gErr
		}
		if existing.RequestHash != reqHash {
			return AgentCommandResult{Status: "reused_id"}, nil // same id, different payload
		}
		var res AgentCommandResult
		if len(existing.Result) > 0 {
			_ = json.Unmarshal(existing.Result, &res)
		}
		return res, tx.Commit(ctx) // commit to release the FOR UPDATE lock
	} else if err != nil {
		return AgentCommandResult{}, err
	}

	// New command: the session must be live (revocation is cluster-visible —
	// review HIGH). On revocation we return without FinishCommand; the deferred
	// rollback drops the just-claimed dedupe row so a later reconnect can retry.
	if _, err := qtx.IsAgentSessionLive(ctx, generated.IsAgentSessionLiveParams{OrgID: pgUUID(orgID), SessionID: pgUUID(sessionID)}); errors.Is(err, pgx.ErrNoRows) {
		return AgentCommandResult{Status: "session_revoked"}, nil
	} else if err != nil {
		return AgentCommandResult{}, err
	}

	res, runErr := e.runTransition(ctx, tx, qtx, orgID, agentID, resID, kind)
	if runErr != nil {
		return AgentCommandResult{}, runErr // infra error → rollback (dedupe row gone)
	}

	resJSON, _ := json.Marshal(res)
	if _, err := qtx.FinishCommand(ctx, generated.FinishCommandParams{OrgID: pgUUID(orgID), AgentID: pgUUID(agentID), ClientMsgID: pgUUID(clientMsgID), Result: resJSON}); err != nil {
		return AgentCommandResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AgentCommandResult{}, err
	}
	return res, nil
}

// runTransition applies the reservation state change within the command tx. A
// domain conflict (lost race, wrong owner, wrong state) is a committed result
// (returned status), not an error; only infra failures return an error.
func (e *Endpoints) runTransition(ctx context.Context, tx *db.OrgTx, qtx *generated.Queries, orgID, agentID, resID uuid.UUID, kind AgentCommandKind) (AgentCommandResult, error) {
	out := AgentCommandResult{ReservationID: resID.String()}
	resv, err := qtx.GetReservation(ctx, generated.GetReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		out.Status = "not_found"
		return out, nil
	}
	if err != nil {
		return out, err
	}
	// Agent-ownership CAS: only the offered agent may act on this reservation.
	if uuid.UUID(resv.AgentID.Bytes) != agentID {
		out.Status = "not_owner"
		return out, nil
	}

	switch kind {
	case CmdComplete:
		comp, cErr := qtx.CompleteReservation(ctx, generated.CompleteReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID)})
		if errors.Is(cErr, pgx.ErrNoRows) {
			out.Status = "conflict"
			return out, nil
		}
		if cErr != nil {
			return out, cErr
		}
		if e.deps.Capacity != nil { // free the slot held for the live interaction
			if rErr := e.deps.Capacity.ReleaseInTx(ctx, qtx, orgID, resID); rErr != nil {
				return out, rErr
			}
		}
		wrapUp := string(api.AgentStatusWrapUp)
		until := pgtype.Timestamptz{Time: time.Now().Add(wrapUpSeconds * time.Second), Valid: true}
		if _, err := qtx.UpdateAgentStateStatus(ctx, generated.UpdateAgentStateStatusParams{
			AgentID: comp.AgentID, OrgID: pgUUID(orgID), ToStatus: &wrapUp, ExpectedFrom: string(api.AgentStatusEngaged), WrapupUntil: until,
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
		routeID := apiUUID(comp.RouteRequestID)
		e.appendEvent(ctx, qtx, orgID, routeID, "reservation.completed", map[string]any{"reservation_id": resID.String()})
		e.appendEvent(ctx, qtx, orgID, routeID, "agent.wrapup", map[string]any{"agent_id": agentID.String()})
		out.Status = "completed"
		return out, nil
	case CmdAccept, CmdReject:
		// AcquireRouteForRun flips the route to status='running' (the run-lock).
		// resumeRoute releases it — but only on the success path. A domain
		// conflict AFTER the flip (wrong current reservation, or a lost
		// Accept/Reject CAS) returns early WITHOUT resumeRoute, which would
		// strand the route 'running' forever. Wrap acquire+transition in a
		// savepoint: roll it back on conflict so the run-lock flip is undone,
		// while the parent tx keeps the committed dedupe row + conflict result
		// (review BLOCK-1).
		sp, spErr := tx.BeginSavepoint(ctx)
		if spErr != nil {
			return out, spErr
		}
		qsp := generated.New(sp)
		res, txErr := e.runRouteTransition(ctx, sp, qsp, orgID, agentID, resID, kind, resv)
		if txErr != nil {
			_ = sp.Rollback(ctx)
			return out, txErr
		}
		if res.Status == "conflict" {
			if err := sp.Rollback(ctx); err != nil {
				return out, err
			}
			return res, nil
		}
		if err := sp.Commit(ctx); err != nil {
			return out, err
		}
		return res, nil
	default:
		out.Status = "conflict"
		return out, nil
	}
}

// runRouteTransition acquires the route run-lock and applies an accept/reject. It
// runs inside a savepoint owned by the caller so a domain-conflict early return
// can be rolled back, undoing the run-lock flip (review BLOCK-1).
func (e *Endpoints) runRouteTransition(ctx context.Context, tx *db.OrgTx, qtx *generated.Queries, orgID, agentID, resID uuid.UUID, kind AgentCommandKind, resv generated.Reservation) (AgentCommandResult, error) {
	out := AgentCommandResult{ReservationID: resID.String()}
	route, rErr := qtx.AcquireRouteForRun(ctx, generated.AcquireRouteForRunParams{ID: resv.RouteRequestID, OrgID: pgUUID(orgID)})
	if errors.Is(rErr, pgx.ErrNoRows) {
		out.Status = "conflict"
		return out, nil
	}
	if rErr != nil {
		return out, rErr
	}
	if !route.CurrentReservationID.Valid || uuid.UUID(route.CurrentReservationID.Bytes) != resID {
		out.Status = "conflict"
		return out, nil
	}
	if kind == CmdAccept {
		return e.applyAccept(ctx, tx, qtx, orgID, agentID, resID, route)
	}
	return e.applyReject(ctx, tx, qtx, orgID, resID, route)
}

func (e *Endpoints) applyAccept(ctx context.Context, tx *db.OrgTx, qtx *generated.Queries, orgID, agentID, resID uuid.UUID, route generated.RouteRequest) (AgentCommandResult, error) {
	out := AgentCommandResult{ReservationID: resID.String()}
	if _, err := qtx.AcceptReservation(ctx, generated.AcceptReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID)}); errors.Is(err, pgx.ErrNoRows) {
		out.Status = "conflict"
		return out, nil
	} else if err != nil {
		return out, err
	}
	// Promote the capacity hold to confirmed (gated on the accept above). A lost
	// slot ⇒ conflict; the surrounding savepoint rolls the accept back.
	if e.deps.Capacity != nil {
		if ok, cErr := e.deps.Capacity.ConfirmInTx(ctx, qtx, orgID, resID); cErr != nil {
			return out, cErr
		} else if !ok {
			out.Status = "conflict"
			return out, nil
		}
	}
	engaged := string(api.AgentStatusEngaged)
	ch := route.Channel
	if _, err := qtx.UpdateAgentStateStatus(ctx, generated.UpdateAgentStateStatusParams{
		AgentID: pgUUID(agentID), OrgID: pgUUID(orgID), ToStatus: &engaged, ExpectedFrom: string(api.AgentStatusReady), EngagedChannel: &ch,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	routeID := apiUUID(route.ID)
	e.appendEvent(ctx, qtx, orgID, routeID, "reservation.accepted", map[string]any{"reservation_id": resID.String()})
	e.appendEvent(ctx, qtx, orgID, routeID, "agent.engaged", map[string]any{"agent_id": agentID.String()})
	if err := e.resumeRoute(ctx, tx, qtx, orgID, route, "accepted"); err != nil {
		return out, err
	}
	out.Status = "accepted"
	return out, nil
}

func (e *Endpoints) applyReject(ctx context.Context, tx *db.OrgTx, qtx *generated.Queries, orgID, resID uuid.UUID, route generated.RouteRequest) (AgentCommandResult, error) {
	out := AgentCommandResult{ReservationID: resID.String()}
	reason := "rejected_by_agent"
	if _, err := qtx.RejectReservation(ctx, generated.RejectReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID), Reason: &reason}); errors.Is(err, pgx.ErrNoRows) {
		out.Status = "conflict"
		return out, nil
	} else if err != nil {
		return out, err
	}
	if e.deps.Capacity != nil { // free the slot (gated on the reject above)
		if cErr := e.deps.Capacity.ReleaseInTx(ctx, qtx, orgID, resID); cErr != nil {
			return out, cErr
		}
	}
	routeID := apiUUID(route.ID)
	e.appendEvent(ctx, qtx, orgID, routeID, "reservation.rejected", map[string]any{"reservation_id": resID.String()})
	if err := e.resumeRoute(ctx, tx, qtx, orgID, route, "rejected"); err != nil {
		return out, err
	}
	out.Status = "rejected"
	return out, nil
}
