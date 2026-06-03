// Package flowrt holds the v0.2 control-plane HTTP handlers for flow publish /
// validate / simulate / rollback and the runtime read surface (route requests,
// reservations, traces). These are the endpoints the flow UI (Layer 2) binds to.
//
// Method bodies are 501-style stubs in Layer 1 (foundation); Layer 3 fills them
// against internal/runtime + sqlc. The stubs deliberately do NOT touch the
// receiver, so a nil *Endpoints embedded in a test composite still answers
// (returns 500 not_implemented) instead of panicking — that keeps the
// state/server test fixtures from having to hand-list every method.
//
// The type is named Endpoints (not Handlers) so it embeds into the ApiHandlers
// composite under a field name distinct from catalog.Handlers / state.Server /
// imports.Importer (Pitfall 1 — no duplicate anonymous field).
package flowrt

import (
	"context"
	"log/slog"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db"
)

// Deps carries the dependencies Layer 3 will use. Stored via New so the wiring
// in cmd/api/main.go is already correct when the bodies land.
type Deps struct {
	OrgDB  *db.OrgDB
	Cache  *cache.Cache
	Logger *slog.Logger
}

type Endpoints struct {
	deps Deps
}

func New(deps Deps) *Endpoints { return &Endpoints{deps: deps} }

func notImpl() api.InternalServerErrorJSONResponse {
	return api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}
}

func (*Endpoints) ValidateFlow(_ context.Context, _ api.ValidateFlowRequestObject) (api.ValidateFlowResponseObject, error) {
	return api.ValidateFlow500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) SimulateFlow(_ context.Context, _ api.SimulateFlowRequestObject) (api.SimulateFlowResponseObject, error) {
	return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) PublishFlow(_ context.Context, _ api.PublishFlowRequestObject) (api.PublishFlowResponseObject, error) {
	return api.PublishFlow500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) RollbackFlow(_ context.Context, _ api.RollbackFlowRequestObject) (api.RollbackFlowResponseObject, error) {
	return api.RollbackFlow500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) ListFlowVersions(_ context.Context, _ api.ListFlowVersionsRequestObject) (api.ListFlowVersionsResponseObject, error) {
	return api.ListFlowVersions500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) GetFlowVersion(_ context.Context, _ api.GetFlowVersionRequestObject) (api.GetFlowVersionResponseObject, error) {
	return api.GetFlowVersion500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) CreateRouteRequest(_ context.Context, _ api.CreateRouteRequestRequestObject) (api.CreateRouteRequestResponseObject, error) {
	return api.CreateRouteRequest500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) GetRouteRequest(_ context.Context, _ api.GetRouteRequestRequestObject) (api.GetRouteRequestResponseObject, error) {
	return api.GetRouteRequest500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) GetRouteRequestTrace(_ context.Context, _ api.GetRouteRequestTraceRequestObject) (api.GetRouteRequestTraceResponseObject, error) {
	return api.GetRouteRequestTrace500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) GetReservation(_ context.Context, _ api.GetReservationRequestObject) (api.GetReservationResponseObject, error) {
	return api.GetReservation500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) AcceptReservation(_ context.Context, _ api.AcceptReservationRequestObject) (api.AcceptReservationResponseObject, error) {
	return api.AcceptReservation500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) RejectReservation(_ context.Context, _ api.RejectReservationRequestObject) (api.RejectReservationResponseObject, error) {
	return api.RejectReservation500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) GetTrace(_ context.Context, _ api.GetTraceRequestObject) (api.GetTraceResponseObject, error) {
	return api.GetTrace500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) ListRouteRequests(_ context.Context, _ api.ListRouteRequestsRequestObject) (api.ListRouteRequestsResponseObject, error) {
	return api.ListRouteRequests500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) ListRouteRequestReservations(_ context.Context, _ api.ListRouteRequestReservationsRequestObject) (api.ListRouteRequestReservationsResponseObject, error) {
	return api.ListRouteRequestReservations500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) CompleteReservation(_ context.Context, _ api.CompleteReservationRequestObject) (api.CompleteReservationResponseObject, error) {
	return api.CompleteReservation500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

func (*Endpoints) ListFlowEntryBindings(_ context.Context, _ api.ListFlowEntryBindingsRequestObject) (api.ListFlowEntryBindingsResponseObject, error) {
	return api.ListFlowEntryBindings500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
