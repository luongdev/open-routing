package flowrt

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
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
	rows, err := generated.New(e.deps.OrgDB).ListRouteRequests(ctx, generated.ListRouteRequestsParams{OrgID: pgUUID(orgID), Limit: int32(limit + 1)})
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
