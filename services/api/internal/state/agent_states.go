package state

import (
	"context"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// GetAgentStatus is the StrictServerInterface implementation for
// GET /v1/orgs/{org_id}/agents/{id}/status. Wave 1 ships a 501 stub
// so the state package can be wired into the composite ApiHandlers
// before Wave 2 implements real cache+DB load. Wave 2 (04-03-PLAN.md)
// replaces this body entirely.
func (s *Server) GetAgentStatus(_ context.Context, _ api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error) {
	return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
		Error:  api.ErrorCodeInternal,
		Reason: "wave_1_stub_GetAgentStatus_pending_wave_2_implementation",
	}}, nil
}

// PatchAgentStatus is the StrictServerInterface implementation for
// PATCH /v1/orgs/{org_id}/agents/{id}/status. Wave 1 ships a 501 stub
// so the state package can be wired into the composite ApiHandlers
// before Wave 2 implements: (a) cross-row break_reason probe, (b)
// transition validator OR force bypass, (c) atomic UPDATE + 0-row
// disambiguate, (d) cache.Del after commit. Wave 2 (04-03-PLAN.md)
// replaces this body entirely.
func (s *Server) PatchAgentStatus(_ context.Context, _ api.PatchAgentStatusRequestObject) (api.PatchAgentStatusResponseObject, error) {
	return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
		Error:  api.ErrorCodeInternal,
		Reason: "wave_1_stub_PatchAgentStatus_pending_wave_2_implementation",
	}}, nil
}
