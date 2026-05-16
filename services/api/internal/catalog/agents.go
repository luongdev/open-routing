// agents.go — PLACEHOLDER entity file (Plan 03-05 scaffold).
//
// Plan 03-06 (Wave 4) REPLACES the bodies of all 5 methods below with
// the real implementations (CAT-01, CAT-03, CAT-08, CAT-09, CAT-10,
// CAT-11). Until then, every method returns the canonical
// 500 "not_implemented_yet" envelope.
//
// D-68: one .go per entity; D-69 catalog.Handlers IS the
// StrictServerInterface impl. The Wave 0 transitional stubs in
// services/api/internal/server/wave0_temp_stubs.go remain the real
// strict-server impl wired into main.go until Plan 03-10.
package catalog

import (
	"context"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// ListAgents — PLACEHOLDER replaced by Plan 03-06 (CAT-01, CAT-09, CAT-10).
func (h *Handlers) ListAgents(_ context.Context, _ api.ListAgentsRequestObject) (api.ListAgentsResponseObject, error) {
	return api.ListAgents500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// CreateAgent — PLACEHOLDER replaced by Plan 03-06 (CAT-01, CAT-03).
func (h *Handlers) CreateAgent(_ context.Context, _ api.CreateAgentRequestObject) (api.CreateAgentResponseObject, error) {
	return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// DeleteAgent — PLACEHOLDER replaced by Plan 03-06 (CAT-01, CAT-09).
func (h *Handlers) DeleteAgent(_ context.Context, _ api.DeleteAgentRequestObject) (api.DeleteAgentResponseObject, error) {
	return api.DeleteAgent500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// GetAgent — PLACEHOLDER replaced by Plan 03-06 (CAT-01, CAT-11).
func (h *Handlers) GetAgent(_ context.Context, _ api.GetAgentRequestObject) (api.GetAgentResponseObject, error) {
	return api.GetAgent500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// UpdateAgent — PLACEHOLDER replaced by Plan 03-06 (CAT-01, CAT-03, CAT-08).
func (h *Handlers) UpdateAgent(_ context.Context, _ api.UpdateAgentRequestObject) (api.UpdateAgentResponseObject, error) {
	return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}
