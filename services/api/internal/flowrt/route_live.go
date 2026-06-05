package flowrt

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
)

// mapRouteRequest converts a DB row to the API DTO. TraceId is left nil — the
// trace references the route (not vice-versa); GetRouteRequestTrace resolves it.
func mapRouteRequest(r generated.RouteRequest) api.RouteRequest {
	out := api.RouteRequest{
		Id:        api.UUIDv7(apiUUID(r.ID)),
		OrgId:     api.UUIDv7(apiUUID(r.OrgID)),
		Channel:   r.Channel,
		EntryCode: r.EntryCode,
		Status:    api.RouteRequestStatus(r.Status),
		CreatedAt: ptrTime(r.CreatedAt),
		UpdatedAt: ptrTime(r.UpdatedAt),
	}
	if r.FlowVersionID.Valid {
		v := api.UUIDv7(apiUUID(r.FlowVersionID))
		out.FlowVersionId = &v
	}
	out.FlowCode = r.FlowCode
	if r.FailureCode != nil {
		fc := api.RoutingFailureCode(*r.FailureCode)
		out.FailureCode = &fc
	}
	return out
}

func mapReservation(r generated.Reservation) api.Reservation {
	out := api.Reservation{
		Id:             api.UUIDv7(apiUUID(r.ID)),
		OrgId:          api.UUIDv7(apiUUID(r.OrgID)),
		RouteRequestId: api.UUIDv7(apiUUID(r.RouteRequestID)),
		AgentId:        api.UUIDv7(apiUUID(r.AgentID)),
		State:          api.ReservationState(r.State),
		Attempt:        int(r.Attempt),
		Reason:         r.Reason,
		CreatedAt:      ptrTime(r.CreatedAt),
		UpdatedAt:      ptrTime(r.UpdatedAt),
	}
	if r.OfferedAt.Valid {
		out.OfferedAt = r.OfferedAt.Time
	}
	if r.ExpiresAt.Valid {
		out.ExpiresAt = r.ExpiresAt.Time
	}
	out.ResolvedAt = ptrTime(r.ResolvedAt)
	return out
}

func (e *Endpoints) GetRouteRequest(ctx context.Context, req api.GetRouteRequestRequestObject) (api.GetRouteRequestResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetRouteRequest500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	row, err := generated.New(e.deps.OrgDB).GetRouteRequest(ctx, generated.GetRouteRequestParams{ID: pgUUID(uuid.UUID(req.Id)), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.GetRouteRequest404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Error: api.ErrorCodeNotFound, Reason: "route_request_not_found"}}, nil
	}
	if err != nil {
		return api.GetRouteRequest500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "load_failed"}}, nil
	}
	return api.GetRouteRequest200JSONResponse(mapRouteRequest(row)), nil
}

func (e *Endpoints) ListRouteRequests(ctx context.Context, req api.ListRouteRequestsRequestObject) (api.ListRouteRequestsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListRouteRequests500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	limit := 25
	if req.Params.Limit != nil && *req.Params.Limit > 0 {
		limit = int(*req.Params.Limit)
	}
	// Fetch one extra to compute has_more without a cursor (cursor paging is
	// additive — v0.2 returns a bounded first page).
	rows, err := generated.New(e.deps.OrgDB).ListRouteRequests(ctx, generated.ListRouteRequestsParams{OrgID: pgUUID(orgID), Limit: int32(limit + 1)}) //nolint:gosec // limit is bounded (<=100) by the handler
	if err != nil {
		return api.ListRouteRequests500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "list_failed"}}, nil
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]api.RouteRequest, len(rows))
	for i, r := range rows {
		items[i] = mapRouteRequest(r)
	}
	return api.ListRouteRequests200JSONResponse{Items: items, HasMore: hasMore}, nil
}

func (e *Endpoints) GetReservation(ctx context.Context, req api.GetReservationRequestObject) (api.GetReservationResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	row, err := generated.New(e.deps.OrgDB).GetReservation(ctx, generated.GetReservationParams{ID: pgUUID(uuid.UUID(req.Id)), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.GetReservation404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Error: api.ErrorCodeNotFound, Reason: "reservation_not_found"}}, nil
	}
	if err != nil {
		return api.GetReservation500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "load_failed"}}, nil
	}
	return api.GetReservation200JSONResponse(mapReservation(row)), nil
}

func (e *Endpoints) ListRouteRequestReservations(ctx context.Context, req api.ListRouteRequestReservationsRequestObject) (api.ListRouteRequestReservationsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListRouteRequestReservations500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	rows, err := generated.New(e.deps.OrgDB).ListReservationsByRoute(ctx, generated.ListReservationsByRouteParams{OrgID: pgUUID(orgID), RouteRequestID: pgUUID(uuid.UUID(req.Id))})
	if err != nil {
		return api.ListRouteRequestReservations500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "list_failed"}}, nil
	}
	items := make([]api.Reservation, len(rows))
	for i, r := range rows {
		items[i] = mapReservation(r)
	}
	return api.ListRouteRequestReservations200JSONResponse{Items: items}, nil
}

func mapRuntimeEvent(r generated.RuntimeEvent) api.RuntimeEvent {
	out := api.RuntimeEvent{
		Id:        api.UUIDv7(apiUUID(r.ID)),
		Source:    r.Source,
		Type:      r.Type,
		CreatedAt: ptrTime(r.CreatedAt),
	}
	if r.RouteRequestID.Valid {
		v := api.UUIDv7(apiUUID(r.RouteRequestID))
		out.RouteRequestId = &v
	}
	if r.CorrelationID.Valid {
		v := api.UUIDv7(apiUUID(r.CorrelationID))
		out.CorrelationId = &v
	}
	if len(r.Payload) > 0 {
		var p map[string]any
		if json.Unmarshal(r.Payload, &p) == nil {
			out.Payload = &p
		}
	}
	return out
}

func (e *Endpoints) ListRouteRequestEvents(ctx context.Context, req api.ListRouteRequestEventsRequestObject) (api.ListRouteRequestEventsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListRouteRequestEvents500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	rows, err := generated.New(e.deps.OrgDB).ListRuntimeEventsByRoute(ctx, generated.ListRuntimeEventsByRouteParams{OrgID: pgUUID(orgID), RouteRequestID: pgUUID(uuid.UUID(req.Id))})
	if err != nil {
		return api.ListRouteRequestEvents500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "list_failed"}}, nil
	}
	items := make([]api.RuntimeEvent, len(rows))
	for i, r := range rows {
		items[i] = mapRuntimeEvent(r)
	}
	return api.ListRouteRequestEvents200JSONResponse{Items: items}, nil
}

func (e *Endpoints) GetRouteRequestTrace(ctx context.Context, req api.GetRouteRequestTraceRequestObject) (api.GetRouteRequestTraceResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetRouteRequestTrace500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	row, err := generated.New(e.deps.OrgDB).GetTraceByRoute(ctx, generated.GetTraceByRouteParams{OrgID: pgUUID(orgID), RouteRequestID: pgUUID(uuid.UUID(req.Id))})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.GetRouteRequestTrace404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Error: api.ErrorCodeNotFound, Reason: "trace_not_found"}}, nil
	}
	if err != nil {
		return api.GetRouteRequestTrace500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "load_failed"}}, nil
	}
	tr, err := rowToAPITrace(row)
	if err != nil {
		return api.GetRouteRequestTrace500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "trace_decode_failed"}}, nil
	}
	return api.GetRouteRequestTrace200JSONResponse(tr), nil
}

func (e *Endpoints) ListFlowEntryBindings(ctx context.Context, _ api.ListFlowEntryBindingsRequestObject) (api.ListFlowEntryBindingsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListFlowEntryBindings500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	rows, err := generated.New(e.deps.OrgDB).ListActiveBindings(ctx, pgUUID(orgID))
	if err != nil {
		return api.ListFlowEntryBindings500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "list_failed"}}, nil
	}
	items := make([]api.FlowEntryBinding, len(rows))
	for i, r := range rows {
		items[i] = mapBinding(r)
	}
	return api.ListFlowEntryBindings200JSONResponse{Items: items}, nil
}

// liveOfferer implements runtime.Offerer for a live route run: it writes an
// `offered` reservation row inside the route's tx. Each INSERT is wrapped in a
// SAVEPOINT so a 23505 (agent already active elsewhere, or attempt collision)
// rolls back just that offer — not the whole route tx — and the reservation
// node skips to the next candidate (ok=false). The runtime works in agent
// CODES; the offerer resolves code→uuid via GetAgentByCode.
type liveOfferer struct {
	ctx      context.Context
	tx       *db.OrgTx
	orgID    uuid.UUID
	routeID  uuid.UUID
	channel  string             // for capacity (voice=1, chat=N)
	cap      *CapacityService   // nil ⇒ simulation mode (no capacity holds)
	excluded map[uuid.UUID]bool // agents already offered on this route (resume re-offer)
	attempt  int
	lastRes  uuid.UUID
	lastExp  time.Time
	offered  bool
}

func (o *liveOfferer) Offer(agentCode string, timeout time.Duration) (string, bool, error) {
	agent, err := generated.New(o.tx).GetAgentByCode(o.ctx, generated.GetAgentByCodeParams{Code: agentCode, OrgID: pgUUID(o.orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil // agent vanished since the snapshot → skip
	}
	if err != nil {
		return "", false, err
	}
	if o.excluded[apiUUID(agent.ID)] {
		return "", false, nil // already offered this agent on this route → skip
	}
	exp := time.Now().Add(timeout)
	// Each candidate's offer is wrapped in a savepoint so a capacity skip or a
	// unique-violation (agent busy) rolls back just this attempt and the reservation
	// node moves to the next candidate — the parent route tx survives.
	sp, err := o.tx.BeginSavepoint(o.ctx)
	if err != nil {
		return "", false, err
	}
	spq := generated.New(sp)
	att, capOK, aErr := attachOffer(o.ctx, spq, o.cap, o.orgID, o.routeID, apiUUID(agent.ID), o.channel, int32(o.attempt+1), exp) //nolint:gosec // attempt bounded by max_attempts
	if aErr != nil {
		_ = sp.Rollback(o.ctx)
		// Agent busy on this route, or a vanished agent → skip this candidate, not a
		// route failure; any other error aborts.
		if errors.Is(aErr, errOfferBusy) || errors.Is(aErr, errAgentVanished) {
			return "", false, nil
		}
		return "", false, aErr
	}
	if !capOK {
		// Roll back the savepoint FIRST, THEN write the capacity_lost audit on the
		// parent tx: sp and o.tx share one connection, so ROLLBACK TO SAVEPOINT would
		// also revert a row inserted before it — recording after the rollback is what
		// actually persists the decision (cross-AI strict review HIGH).
		_ = sp.Rollback(o.ctx)
		recordInlineDecision(o.ctx, generated.New(o.tx), o.orgID, o.routeID, o.channel, agentCode, apiUUID(agent.ID), "capacity_lost")
		return "", false, nil
	}
	recordInlineDecision(o.ctx, spq, o.orgID, o.routeID, o.channel, agentCode, apiUUID(agent.ID), "offered")
	if err := sp.Commit(o.ctx); err != nil {
		return "", false, err
	}
	o.attempt++
	o.lastRes, o.lastExp, o.offered = att.resID, exp, true
	return att.resID.String(), true, nil
}

// GetRoutingStats returns the org's live matcher/queue snapshot for the ops view.
func (e *Endpoints) GetRoutingStats(ctx context.Context, _ api.GetRoutingStatsRequestObject) (api.GetRoutingStatsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetRoutingStats500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	q := generated.New(e.deps.OrgDB)
	qs, err := q.GetRoutingQueueStats(ctx, pgUUID(orgID))
	if err != nil {
		return api.GetRoutingStats500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "stats_failed"}}, nil
	}
	held, err := q.CountHeldSlotsForOrg(ctx, pgUUID(orgID))
	if err != nil {
		return api.GetRoutingStats500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "occupancy_failed"}}, nil
	}
	return api.GetRoutingStats200JSONResponse{
		WaitingMatch:         int(qs.WaitingMatch),
		Offering:             int(qs.Offering),
		WaitingOffer:         int(qs.WaitingOffer),
		OldestWaitingSeconds: int(qs.OldestWaitingSeconds),
		HeldSlots:            int(held),
	}, nil
}

// AbandonRouteRequest tears down a route whose caller hung up: cancel the route
// (any non-terminal state) + cancel its outstanding offered reservations, freeing
// each agent's capacity hold. One tx so the route terminates and slots free
// atomically. 409 if the route is already terminal.
func (e *Endpoints) AbandonRouteRequest(ctx context.Context, req api.AbandonRouteRequestRequestObject) (api.AbandonRouteRequestResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.AbandonRouteRequest500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	routeID := uuid.UUID(req.Id)
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return api.AbandonRouteRequest500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "tx_begin_failed"}}, nil
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	row, handles, err := e.teardownRouteTx(ctx, qtx, orgID, routeID, "route.abandoned")
	if errors.Is(err, pgx.ErrNoRows) {
		// 0 rows: not found vs already-terminal.
		if _, gErr := qtx.GetRouteRequest(ctx, generated.GetRouteRequestParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID)}); errors.Is(gErr, pgx.ErrNoRows) {
			return api.AbandonRouteRequest404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Error: api.ErrorCodeNotFound, Reason: "route_request_not_found"}}, nil
		}
		return api.AbandonRouteRequest409JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidTransition, Reason: "route_already_terminal"}), nil
	}
	if err != nil {
		return api.AbandonRouteRequest500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "abandon_failed"}}, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return api.AbandonRouteRequest500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "commit_failed"}}, nil
	}
	// Post-commit: tell the adapter to tear down each live delivery (idempotent —
	// a no-op if the adapter already ended it, e.g. this abandon WAS adapter-driven).
	e.releaseAssignments(ctx, row.Channel, handles, adapter.ReleaseCancelled)
	return api.AbandonRouteRequest200JSONResponse(mapRouteRequest(row)), nil
}

// teardownRouteTx cancels a non-terminal route + its live (offered/accepted)
// reservations, frees each capacity slot, moves accepted-call agents to WrapUp,
// and emits eventType. Returns the cancelled route row and the adapter handles of
// the cancelled deliveries (for the caller to Release post-commit). ErrNoRows ⇒
// the route was already terminal (caller maps to 409/no-op) — naturally idempotent,
// which is why an adapter terminal can re-drive it harmlessly.
func (e *Endpoints) teardownRouteTx(ctx context.Context, qtx *generated.Queries, orgID, routeID uuid.UUID, eventType string) (generated.RouteRequest, []string, error) {
	row, err := qtx.AbandonRoute(ctx, generated.AbandonRouteParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID)})
	if err != nil {
		return generated.RouteRequest{}, nil, err
	}
	// Read live reservations BEFORE cancelling so we know which were 'accepted' (a
	// live call → the agent goes to WrapUp) and capture their adapter handles.
	live, err := qtx.ListReservationsByRoute(ctx, generated.ListReservationsByRouteParams{OrgID: pgUUID(orgID), RouteRequestID: pgUUID(routeID)})
	if err != nil {
		return generated.RouteRequest{}, nil, err
	}
	cancelled, err := qtx.CancelLiveReservationsForRoute(ctx, generated.CancelLiveReservationsForRouteParams{OrgID: pgUUID(orgID), RouteRequestID: pgUUID(routeID)})
	if err != nil {
		return generated.RouteRequest{}, nil, err
	}
	if e.deps.Capacity != nil {
		for _, r := range cancelled {
			if rErr := e.deps.Capacity.ReleaseInTx(ctx, qtx, orgID, apiUUID(r.ID)); rErr != nil {
				return generated.RouteRequest{}, nil, rErr
			}
		}
	}
	wrapUp := string(api.AgentStatusWrapUp)
	until := pgtype.Timestamptz{Time: time.Now().Add(wrapUpSeconds * time.Second), Valid: true}
	var handles []string
	for _, r := range live {
		// Only the offered/accepted reservations are the ones CancelLiveReservations
		// just terminated — collect adapter handles from those, never from a prior
		// completed/rejected reservation (which must not get a ReleaseCancelled).
		if r.State != "offered" && r.State != "accepted" {
			continue
		}
		if r.AdapterHandle != nil && *r.AdapterHandle != "" {
			handles = append(handles, *r.AdapterHandle)
		}
		if r.State != "accepted" {
			continue
		}
		if _, err := qtx.UpdateAgentStateStatus(ctx, generated.UpdateAgentStateStatusParams{
			AgentID: r.AgentID, OrgID: pgUUID(orgID), ToStatus: &wrapUp, ExpectedFrom: string(api.AgentStatusEngaged), WrapupUntil: until,
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return generated.RouteRequest{}, nil, err
		}
	}
	e.appendEvent(ctx, qtx, orgID, routeID, eventType, map[string]any{"cancelled_reservations": len(cancelled)})
	return row, handles, nil
}

// recordInlineDecision writes an interaction_offer route_decisions row for the
// inline (interaction-driven) offer path, mirroring the matcher's availability_pull
// audit. Best-effort: a missing audit row must not fail the offer.
func recordInlineDecision(ctx context.Context, q *generated.Queries, orgID, routeID uuid.UUID, channel, agentCode string, agentID uuid.UUID, outcome string) {
	detail, _ := json.Marshal(map[string]any{"agent_code": agentCode, "channel": channel, "source": "inline"})
	_ = q.InsertRouteDecision(ctx, generated.InsertRouteDecisionParams{
		ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(orgID), RouteRequestID: pgUUID(routeID),
		DecisionType: "interaction_offer", MatcherInstance: "inline", Channel: channel,
		SelectedAgentID: pgUUID(agentID), Outcome: outcome, Detail: detail,
	})
}

func (e *Endpoints) appendEvent(ctx context.Context, q *generated.Queries, orgID, routeID uuid.UUID, typ string, payload map[string]any) {
	p := []byte("{}")
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			p = b
		}
	}
	_, _ = q.AppendRuntimeEvent(ctx, generated.AppendRuntimeEventParams{
		ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(orgID),
		RouteRequestID: pgUUID(routeID), Source: "runtime", Type: typ, Payload: p,
	})
}

func crErr(reason string) api.CreateRouteRequestResponseObject {
	return api.CreateRouteRequest500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: reason}}
}

// CreateRouteRequest runs a live route: resolve the active published flow for
// (channel, entry_code), then in ONE tx insert the route, run the executor live
// (offer→suspend at a reservation), persist the trace + events, and either park
// the route (waiting + cursor + reservation_timeout continuation) or finish it.
func (e *Endpoints) CreateRouteRequest(ctx context.Context, req api.CreateRouteRequestRequestObject) (api.CreateRouteRequestResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return crErr("missing_org_id_in_context"), nil
	}
	if req.Body == nil {
		return crErr("missing_body"), nil
	}
	body := *req.Body
	q := generated.New(e.deps.OrgDB)
	routeID := uuid.Must(uuid.NewV7())
	input := map[string]any{}
	if body.InteractionInput != nil {
		input = *body.InteractionInput
	}
	inputJSON, _ := json.Marshal(input)

	binding, err := q.GetActiveBinding(ctx, generated.GetActiveBindingParams{OrgID: pgUUID(orgID), Channel: body.Channel, EntryCode: body.EntryCode})
	if errors.Is(err, pgx.ErrNoRows) {
		// Typed failure: no published flow bound to this entry point.
		fc := string(runtime.FailMissingPublishedFlow)
		row, iErr := q.InsertRouteRequest(ctx, generated.InsertRouteRequestParams{
			ID: pgUUID(routeID), OrgID: pgUUID(orgID), Channel: body.Channel, EntryCode: body.EntryCode,
			InteractionInput: inputJSON, Status: "failed", FailureCode: &fc,
		})
		if iErr != nil {
			return crErr("insert_failed"), nil
		}
		return api.CreateRouteRequest201JSONResponse(mapRouteRequest(row)), nil
	}
	if err != nil {
		return crErr("binding_lookup_failed"), nil
	}

	fv, err := q.GetFlowVersion(ctx, generated.GetFlowVersionParams{ID: binding.FlowVersionID, OrgID: pgUUID(orgID)})
	if err != nil {
		return crErr("flow_version_load_failed"), nil
	}
	graph, gErr := parseGraph(fv.Graph)
	if gErr != nil {
		return crErr("graph_parse_failed"), nil
	}
	plan, cErr := runtime.Compile(graph, e.reg)
	if cErr != nil {
		return crErr("compile_failed"), nil
	}
	snapshot, sErr := e.routingSnapshot(ctx, orgID, body.Channel, graph)
	if sErr != nil {
		return crErr("snapshot_failed"), nil
	}
	snapJSON, _ := json.Marshal(snapshot)

	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return crErr("tx_begin_failed"), nil
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	flowCode := fv.FlowCode
	if _, err := qtx.InsertRouteRequest(ctx, generated.InsertRouteRequestParams{
		ID: pgUUID(routeID), OrgID: pgUUID(orgID), Channel: body.Channel, EntryCode: body.EntryCode,
		FlowVersionID: binding.FlowVersionID, FlowCode: &flowCode,
		InteractionInput: inputJSON, Status: "running", ReadSetSnapshot: snapJSON,
	}); err != nil {
		return crErr("insert_failed"), nil
	}

	offerer := &liveOfferer{ctx: ctx, tx: tx, orgID: orgID, routeID: routeID, channel: body.Channel, cap: e.deps.Capacity}
	opts := []runtime.ExecutorOption{runtime.WithRouting(snapshot, nil), runtime.WithOfferer(offerer)}
	if e.matcherMode() {
		opts = append(opts, runtime.WithMatcher())
	}
	ex := runtime.NewExecutor(e.reg, opts...)
	decStart := time.Now()
	res, rErr := ex.Run(ctx, runtime.NewVirtualClock(time.Now().UTC()), plan, input)
	observeRouteDecision(e.deps.Logger, "create", time.Since(decStart))
	if rErr != nil {
		return crErr("execute_failed"), nil
	}
	e.appendEvent(ctx, qtx, orgID, routeID, "route.created", nil)
	if err := e.persistRunResult(ctx, qtx, orgID, routeID, fv, snapJSON, offerer, res); err != nil {
		return crErr("persist_failed"), nil
	}

	if err := tx.Commit(ctx); err != nil {
		return crErr("commit_failed"), nil
	}
	row, err := q.GetRouteRequest(ctx, generated.GetRouteRequestParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID)})
	if err != nil {
		return crErr("reload_failed"), nil
	}
	return api.CreateRouteRequest201JSONResponse(mapRouteRequest(row)), nil
}
