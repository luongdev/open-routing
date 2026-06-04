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
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
)

const wrapUpSeconds = 30

// persistRunResult records the trace segment + events for one executor run and
// either parks the route (offered → waiting + cursor + reservation_timeout
// continuation) or finishes it (completed/failed). Runs inside the route tx.
func (e *Endpoints) persistRunResult(ctx context.Context, qtx *generated.Queries, orgID, routeID uuid.UUID, fv generated.FlowVersion, snapJSON []byte, offerer *liveOfferer, res runtime.RunResult) error {
	steps := mapTraceSteps(res.Trace)
	stepsJSON, _ := json.Marshal(steps)
	outcome := res.Trace.Outcome
	pfv := int32(runtime.PlanFormatVersion)
	if _, err := qtx.InsertTrace(ctx, generated.InsertTraceParams{
		ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(orgID), Kind: "runtime",
		FlowID: fv.FlowID, FlowVersionID: fv.ID, RouteRequestID: pgUUID(routeID),
		Steps: stepsJSON, Outcome: &outcome, PlanFormatVersion: &pfv, ReadSetSnapshot: snapJSON,
	}); err != nil {
		return err
	}
	switch {
	case res.Suspension != nil:
		cur := runtime.ResumeCursor{Version: 1, NodeID: res.SuspendedNodeID, Vars: res.Vars}
		curJSON, _ := json.Marshal(cur)
		if _, err := qtx.SuspendRoute(ctx, generated.SuspendRouteParams{
			ID: pgUUID(routeID), OrgID: pgUUID(orgID), ResumeCursor: curJSON, CurrentReservationID: pgUUID(offerer.lastRes),
		}); err != nil {
			return err
		}
		if _, err := qtx.InsertContinuation(ctx, generated.InsertContinuationParams{
			ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(orgID), Kind: "reservation_timeout",
			RouteRequestID: pgUUID(routeID), ReservationID: pgUUID(offerer.lastRes),
			FlowVersionID: fv.ID, Cursor: []byte("{}"),
			DueAt: pgtype.Timestamptz{Time: offerer.lastExp, Valid: true},
		}); err != nil {
			return err
		}
		e.appendEvent(ctx, qtx, orgID, routeID, "reservation.offered", map[string]any{"reservation_id": offerer.lastRes.String()})
	case outcome == "failed":
		fc := res.Trace.FailureCode
		var fcp *string
		if fc != "" {
			fcp = &fc
		}
		if _, err := qtx.FinishRoute(ctx, generated.FinishRouteParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID), Status: "failed", FailureCode: fcp}); err != nil {
			return err
		}
		e.appendEvent(ctx, qtx, orgID, routeID, "route.failed", nil)
	default:
		if _, err := qtx.FinishRoute(ctx, generated.FinishRouteParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID), Status: "completed"}); err != nil {
			return err
		}
		e.appendEvent(ctx, qtx, orgID, routeID, "route.completed", nil)
	}
	return nil
}

// resumeRoute re-runs the parked flow with `signal` (must hold the route lock).
// It re-reads the live candidate pool (route_queue/match_skill re-run) and the
// offerer excludes agents already offered on this route, so a reject/timeout
// re-offers a fresh agent.
func (e *Endpoints) resumeRoute(ctx context.Context, tx *db.OrgTx, qtx *generated.Queries, orgID uuid.UUID, route generated.RouteRequest, signal string) error {
	fv, err := qtx.GetFlowVersion(ctx, generated.GetFlowVersionParams{ID: route.FlowVersionID, OrgID: pgUUID(orgID)})
	if err != nil {
		return err
	}
	graph, err := parseGraph(fv.Graph)
	if err != nil {
		return err
	}
	plan, err := runtime.Compile(graph, e.reg)
	if err != nil {
		return err
	}
	snapshot, err := e.buildSnapshot(ctx, pgUUID(orgID), graph)
	if err != nil {
		return err
	}
	snapJSON, _ := json.Marshal(snapshot)
	var cur runtime.ResumeCursor
	if len(route.ResumeCursor) > 0 {
		_ = json.Unmarshal(route.ResumeCursor, &cur)
	}
	routeID := apiUUID(route.ID)

	resvs, err := qtx.ListReservationsByRoute(ctx, generated.ListReservationsByRouteParams{OrgID: pgUUID(orgID), RouteRequestID: route.ID})
	if err != nil {
		return err
	}
	excluded := map[uuid.UUID]bool{}
	maxAttempt := 0
	for _, r := range resvs {
		excluded[apiUUID(r.AgentID)] = true
		if int(r.Attempt) > maxAttempt {
			maxAttempt = int(r.Attempt)
		}
	}
	offerer := &liveOfferer{ctx: ctx, tx: tx, orgID: orgID, routeID: routeID, excluded: excluded, attempt: maxAttempt}
	ex := runtime.NewExecutor(e.reg, runtime.WithRouting(snapshot, nil), runtime.WithOfferer(offerer))
	res, err := ex.RunResume(ctx, runtime.NewVirtualClock(time.Now().UTC()), plan, cur.NodeID, signal, cur.Vars)
	if err != nil {
		return err
	}
	return e.persistRunResult(ctx, qtx, orgID, routeID, fv, snapJSON, offerer, res)
}

// resolveReservation is the shared accept/reject body: acquire the route lock,
// apply the guarded reservation transition, then (for accept) engage the agent
// and resume the flow. 0 rows on either guard → 409 conflict.
func (e *Endpoints) lifecycleConflict() api.AcceptReservationResponseObject {
	return api.AcceptReservation409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "reservation_transition_conflict"})
}

func (e *Endpoints) AcceptReservation(ctx context.Context, req api.AcceptReservationRequestObject) (api.AcceptReservationResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.AcceptReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	resID := uuid.UUID(req.Id)
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return api.AcceptReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "tx_begin_failed"}}, nil
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	resv, err := qtx.GetReservation(ctx, generated.GetReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.AcceptReservation404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Error: api.ErrorCodeNotFound, Reason: "reservation_not_found"}}, nil
	}
	if err != nil {
		return api.AcceptReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "load_failed"}}, nil
	}
	route, err := qtx.AcquireRouteForRun(ctx, generated.AcquireRouteForRunParams{ID: resv.RouteRequestID, OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return e.lifecycleConflict(), nil // route not waiting/pending → someone else owns it
	}
	if err != nil {
		return api.AcceptReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "route_lock_failed"}}, nil
	}
	acc, err := qtx.AcceptReservation(ctx, generated.AcceptReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return e.lifecycleConflict(), nil // already resolved (rejected/timeout/raced)
	}
	if err != nil {
		return api.AcceptReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "accept_failed"}}, nil
	}
	// Agent Ready→Engaged (best-effort: a stale agent state must not roll back the
	// accept — the reservation is the assignment's source of truth).
	engaged := string(api.AgentStatusEngaged)
	ch := route.Channel
	_, _ = qtx.UpdateAgentStateStatus(ctx, generated.UpdateAgentStateStatusParams{
		AgentID: resv.AgentID, OrgID: pgUUID(orgID), ToStatus: &engaged, ExpectedFrom: string(api.AgentStatusReady), EngagedChannel: &ch,
	})
	routeID := apiUUID(route.ID)
	e.appendEvent(ctx, qtx, orgID, routeID, "reservation.accepted", map[string]any{"reservation_id": resID.String()})
	e.appendEvent(ctx, qtx, orgID, routeID, "agent.engaged", map[string]any{"agent_id": apiUUID(resv.AgentID).String()})

	if err := e.resumeRoute(ctx, tx, qtx, orgID, route, "accepted"); err != nil {
		return api.AcceptReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "resume_failed"}}, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return api.AcceptReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "commit_failed"}}, nil
	}
	return api.AcceptReservation200JSONResponse(mapReservation(acc)), nil
}

func (e *Endpoints) RejectReservation(ctx context.Context, req api.RejectReservationRequestObject) (api.RejectReservationResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.RejectReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	resID := uuid.UUID(req.Id)
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return api.RejectReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "tx_begin_failed"}}, nil
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	resv, err := qtx.GetReservation(ctx, generated.GetReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.RejectReservation404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Error: api.ErrorCodeNotFound, Reason: "reservation_not_found"}}, nil
	}
	if err != nil {
		return api.RejectReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "load_failed"}}, nil
	}
	route, err := qtx.AcquireRouteForRun(ctx, generated.AcquireRouteForRunParams{ID: resv.RouteRequestID, OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.RejectReservation409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "reservation_transition_conflict"}), nil
	}
	if err != nil {
		return api.RejectReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "route_lock_failed"}}, nil
	}
	reason := "rejected_by_agent"
	if _, err := qtx.RejectReservation(ctx, generated.RejectReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID), Reason: &reason}); errors.Is(err, pgx.ErrNoRows) {
		return api.RejectReservation409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "reservation_transition_conflict"}), nil
	} else if err != nil {
		return api.RejectReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "reject_failed"}}, nil
	}
	routeID := apiUUID(route.ID)
	e.appendEvent(ctx, qtx, orgID, routeID, "reservation.rejected", map[string]any{"reservation_id": resID.String()})
	if err := e.resumeRoute(ctx, tx, qtx, orgID, route, "rejected"); err != nil {
		return api.RejectReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "resume_failed"}}, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return api.RejectReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "commit_failed"}}, nil
	}
	got, _ := generated.New(e.deps.OrgDB).GetReservation(ctx, generated.GetReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID)})
	return api.RejectReservation200JSONResponse(mapReservation(got)), nil
}

// CompleteReservation ends the assignment: reservation accepted→completed and
// the agent Engaged→WrapUp. It does NOT drive the route flow — route completion
// comes from the executor reaching a terminal (cross-AI review).
func (e *Endpoints) CompleteReservation(ctx context.Context, req api.CompleteReservationRequestObject) (api.CompleteReservationResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.CompleteReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	resID := uuid.UUID(req.Id)
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return api.CompleteReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "tx_begin_failed"}}, nil
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	comp, err := qtx.CompleteReservation(ctx, generated.CompleteReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		// Either not found or not in 'accepted' — disambiguate to 404 vs 409.
		if _, gErr := qtx.GetReservation(ctx, generated.GetReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID)}); errors.Is(gErr, pgx.ErrNoRows) {
			return api.CompleteReservation404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Error: api.ErrorCodeNotFound, Reason: "reservation_not_found"}}, nil
		}
		return api.CompleteReservation409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "reservation_transition_conflict"}), nil
	}
	if err != nil {
		return api.CompleteReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "complete_failed"}}, nil
	}
	wrapUp := string(api.AgentStatusWrapUp)
	until := pgtype.Timestamptz{Time: time.Now().Add(wrapUpSeconds * time.Second), Valid: true}
	_, _ = qtx.UpdateAgentStateStatus(ctx, generated.UpdateAgentStateStatusParams{
		AgentID: comp.AgentID, OrgID: pgUUID(orgID), ToStatus: &wrapUp, ExpectedFrom: string(api.AgentStatusEngaged), WrapupUntil: until,
	})
	routeID := apiUUID(comp.RouteRequestID)
	e.appendEvent(ctx, qtx, orgID, routeID, "reservation.completed", map[string]any{"reservation_id": resID.String()})
	e.appendEvent(ctx, qtx, orgID, routeID, "agent.wrapup", map[string]any{"agent_id": apiUUID(comp.AgentID).String()})
	if err := tx.Commit(ctx); err != nil {
		return api.CompleteReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "commit_failed"}}, nil
	}
	return api.CompleteReservation200JSONResponse(mapReservation(comp)), nil
}
