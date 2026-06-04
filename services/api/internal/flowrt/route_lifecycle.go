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
		// A reservation offer parks a reservation_timeout (named reservation, due
		// at the offer expiry); a wait parks a wait continuation (no reservation,
		// due at the wait's resume time). Mixing them up left a wait route stuck.
		if offerer.offered {
			susp, err := qtx.SuspendRoute(ctx, generated.SuspendRouteParams{
				ID: pgUUID(routeID), OrgID: pgUUID(orgID), ResumeCursor: curJSON, CurrentReservationID: pgUUID(offerer.lastRes),
			})
			if err != nil {
				return err
			}
			if _, err := qtx.InsertContinuation(ctx, generated.InsertContinuationParams{
				ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(orgID), Kind: "reservation_timeout",
				RouteRequestID: pgUUID(routeID), ReservationID: pgUUID(offerer.lastRes),
				FlowVersionID: fv.ID, Cursor: []byte("{}"), RunSeq: susp.RunSeq,
				DueAt: pgtype.Timestamptz{Time: offerer.lastExp, Valid: true},
			}); err != nil {
				return err
			}
			e.appendEvent(ctx, qtx, orgID, routeID, "reservation.offered", map[string]any{"reservation_id": offerer.lastRes.String()})
		} else {
			// wait suspension: no reservation, resume at the timer's due time.
			susp, err := qtx.SuspendRoute(ctx, generated.SuspendRouteParams{
				ID: pgUUID(routeID), OrgID: pgUUID(orgID), ResumeCursor: curJSON, CurrentReservationID: pgtype.UUID{},
			})
			if err != nil {
				return err
			}
			// RunSeq pins the route's seq at suspend so a stale timer that fires
			// after a later suspend is no-op'd by the fenced acquire (review B1).
			if _, err := qtx.InsertContinuation(ctx, generated.InsertContinuationParams{
				ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(orgID), Kind: "wait",
				RouteRequestID: pgUUID(routeID), FlowVersionID: fv.ID, Cursor: []byte("{}"), RunSeq: susp.RunSeq,
				DueAt: pgtype.Timestamptz{Time: res.Suspension.ResumeAt, Valid: true},
			}); err != nil {
				return err
			}
			e.appendEvent(ctx, qtx, orgID, routeID, "flow.waiting", nil)
		}
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
// errNotInputCursor: a live input submit targeted a route whose parked cursor is
// not an interactive-input node (re-review H3) → the handler maps it to 409.
var errNotInputCursor = errors.New("flowrt: route is not parked at an input node")

func (e *Endpoints) resumeRoute(ctx context.Context, tx *db.OrgTx, qtx *generated.Queries, orgID uuid.UUID, route generated.RouteRequest, signal string) error {
	return e.resumeRouteWith(ctx, tx, qtx, orgID, route, signal, nil)
}

// resumeRouteWith is resumeRoute plus per-node captured input values: a live
// submit to an interactive-input node passes {nodeID: value} so that node takes
// its captured branch on the re-run instead of timing out. nil for agent/timer
// resumes.
func (e *Endpoints) resumeRouteWith(ctx context.Context, tx *db.OrgTx, qtx *generated.Queries, orgID uuid.UUID, route generated.RouteRequest, signal string, scriptedInputs map[string]any) error {
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
	snapshot, err := e.routingSnapshot(ctx, orgID, route.Channel, graph)
	if err != nil {
		return err
	}
	snapJSON, _ := json.Marshal(snapshot)
	var cur runtime.ResumeCursor
	if len(route.ResumeCursor) > 0 {
		_ = json.Unmarshal(route.ResumeCursor, &cur)
	}
	// A live input submit (scriptedInputs set) may only target an interactive-input
	// cursor — never a parked wait/reservation node, which would otherwise resume
	// early on the injected value (re-review H3).
	if len(scriptedInputs) > 0 && !runtime.IsInteractiveInputKind(plan.StepKind(cur.NodeID)) {
		return errNotInputCursor
	}
	// Replay from the route's ORIGINAL interaction_input (not the post-suspension
	// vars) so re-running from entry is deterministic — set_var/compute before the
	// reservation recompute cleanly instead of double-applying already-mutated
	// state (cross-AI review HIGH).
	input := map[string]any{}
	if len(route.InteractionInput) > 0 {
		_ = json.Unmarshal(route.InteractionInput, &input)
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
	offerer := &liveOfferer{ctx: ctx, tx: tx, orgID: orgID, routeID: routeID, channel: route.Channel, cap: e.deps.Capacity, excluded: excluded, attempt: maxAttempt}
	ex := runtime.NewExecutor(e.reg, runtime.WithRouting(snapshot, nil), runtime.WithOfferer(offerer), runtime.WithScriptedInputs(scriptedInputs))
	decStart := time.Now()
	res, err := ex.RunResume(ctx, runtime.NewVirtualClock(time.Now().UTC()), plan, cur.NodeID, signal, input)
	observeRouteDecision(e.deps.Logger, "resume", time.Since(decStart))
	if err != nil {
		return err
	}
	return e.persistRunResult(ctx, qtx, orgID, routeID, fv, snapJSON, offerer, res)
}

// SubmitRouteInput answers a route parked at an interactive-input node: it locks
// the route, injects the captured value for the target node (the body's node_id
// or the resume cursor), and resumes — the input node takes its captured branch
// instead of timing out. 409 when the route is not waiting.
func (e *Endpoints) SubmitRouteInput(ctx context.Context, req api.SubmitRouteInputRequestObject) (api.SubmitRouteInputResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.SubmitRouteInput500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	if req.Body == nil || req.Body.Value == nil {
		return api.SubmitRouteInput409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "missing_value"}), nil
	}
	routeID := uuid.UUID(req.Id)
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return api.SubmitRouteInput500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "tx_begin_failed"}}, nil
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	// Lock the route: pending/waiting → running. 0 rows ⇒ not waiting (already
	// resumed, completed, or raced) → 409.
	route, err := qtx.AcquireRouteForRun(ctx, generated.AcquireRouteForRunParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.SubmitRouteInput409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "route_not_waiting"}), nil
	}
	if err != nil {
		return api.SubmitRouteInput500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "route_lock_failed"}}, nil
	}
	// A route parked on a reservation offer (current_reservation_id set) is
	// answered via accept/reject, NOT this input endpoint — refuse so an input
	// submit can't hijack a reservation wait (cross-AI review BLOCK).
	if route.CurrentReservationID.Valid {
		return api.SubmitRouteInput409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "route_waiting_on_reservation"}), nil
	}

	// The value is always applied to the node the route is parked at (the resume
	// cursor). A body node_id, if present, MUST match it — a non-cursor node_id
	// would otherwise be silently dropped on the entry-replay and let the actually
	// parked input time out (review H4).
	var cur runtime.ResumeCursor
	if len(route.ResumeCursor) > 0 {
		_ = json.Unmarshal(route.ResumeCursor, &cur)
	}
	target := cur.NodeID
	if target == "" {
		return api.SubmitRouteInput409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "no_parked_input_node"}), nil
	}
	if req.Body.NodeId != nil && *req.Body.NodeId != target {
		return api.SubmitRouteInput409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "node_id_not_parked"}), nil
	}

	e.appendEvent(ctx, qtx, orgID, routeID, "route.input_submitted", map[string]any{"node_id": target})
	if err := e.resumeRouteWith(ctx, tx, qtx, orgID, route, "", map[string]any{target: req.Body.Value}); err != nil {
		if errors.Is(err, errNotInputCursor) {
			return api.SubmitRouteInput409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "not_an_input_node"}), nil
		}
		return api.SubmitRouteInput500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "resume_failed"}}, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return api.SubmitRouteInput500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "commit_failed"}}, nil
	}
	row, err := generated.New(e.deps.OrgDB).GetRouteRequest(ctx, generated.GetRouteRequestParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID)})
	if err != nil {
		return api.SubmitRouteInput500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "reload_failed"}}, nil
	}
	return api.SubmitRouteInput200JSONResponse(mapRouteRequest(row)), nil
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
	// The route must still be parked on THIS reservation; a stale (older-attempt)
	// accept whose route has moved to a newer offer/wait is a 409 (review H3).
	if !route.CurrentReservationID.Valid || uuid.UUID(route.CurrentReservationID.Bytes) != resID {
		return e.lifecycleConflict(), nil
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
	// Best-effort: a stale agent state (guard miss → ErrNoRows) must not roll back
	// the accept, but a REAL DB error must surface — it has poisoned the tx, so
	// swallowing it would only fail the next query opaquely (review M7).
	if _, err := qtx.UpdateAgentStateStatus(ctx, generated.UpdateAgentStateStatusParams{
		AgentID: resv.AgentID, OrgID: pgUUID(orgID), ToStatus: &engaged, ExpectedFrom: string(api.AgentStatusReady), EngagedChannel: &ch,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return api.AcceptReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "agent_state_failed"}}, nil
	}
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
	// Route must still be parked on THIS reservation (review H3).
	if !route.CurrentReservationID.Valid || uuid.UUID(route.CurrentReservationID.Bytes) != resID {
		return api.RejectReservation409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "reservation_transition_conflict"}), nil
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
	// Best-effort, but surface a real DB error (review M7).
	if _, err := qtx.UpdateAgentStateStatus(ctx, generated.UpdateAgentStateStatusParams{
		AgentID: comp.AgentID, OrgID: pgUUID(orgID), ToStatus: &wrapUp, ExpectedFrom: string(api.AgentStatusEngaged), WrapupUntil: until,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return api.CompleteReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "agent_state_failed"}}, nil
	}
	routeID := apiUUID(comp.RouteRequestID)
	e.appendEvent(ctx, qtx, orgID, routeID, "reservation.completed", map[string]any{"reservation_id": resID.String()})
	e.appendEvent(ctx, qtx, orgID, routeID, "agent.wrapup", map[string]any{"agent_id": apiUUID(comp.AgentID).String()})
	if err := tx.Commit(ctx); err != nil {
		return api.CompleteReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "commit_failed"}}, nil
	}
	return api.CompleteReservation200JSONResponse(mapReservation(comp)), nil
}
