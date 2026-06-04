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
	resID := uuid.Must(uuid.NewV7())
	exp := time.Now().Add(timeout)
	sp, err := o.tx.BeginSavepoint(o.ctx)
	if err != nil {
		return "", false, err
	}
	_, err = generated.New(sp).InsertReservationOffer(o.ctx, generated.InsertReservationOfferParams{
		ID: pgUUID(resID), OrgID: pgUUID(o.orgID), RouteRequestID: pgUUID(o.routeID),
		AgentID: agent.ID, Attempt: int32(o.attempt + 1), //nolint:gosec // attempt is bounded (<=10) by max_attempts
		ExpiresAt: pgtype.Timestamptz{Time: exp, Valid: true},
	})
	if isUniqueViolation(err) {
		_ = sp.Rollback(o.ctx) // busy/ineligible → undo this offer, try next candidate
		return "", false, nil
	}
	if err != nil {
		_ = sp.Rollback(o.ctx)
		return "", false, err
	}
	if err := sp.Commit(o.ctx); err != nil {
		return "", false, err
	}
	o.attempt++
	o.lastRes, o.lastExp, o.offered = resID, exp, true
	return resID.String(), true, nil
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
	snapshot, sErr := e.buildSnapshot(ctx, pgUUID(orgID), graph)
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

	offerer := &liveOfferer{ctx: ctx, tx: tx, orgID: orgID, routeID: routeID}
	ex := runtime.NewExecutor(e.reg, runtime.WithRouting(snapshot, nil), runtime.WithOfferer(offerer))
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
