// notimpl.go — 501-equivalent stubs for endpoints owned by Phase 4
// (agent state machine) and Phase 5 (bulk import).
//
// Why these stubs live in the catalog package: `catalog.Handlers` IS the
// StrictServerInterface implementer (D-69), so it MUST provide a method
// for every operation in the spec — including the ones Phase 3 doesn't
// own. Until Phases 4/5 land, the catalog handler returns a 500-shaped
// "not_implemented_yet" body that flows through RequestIDInjectionMiddleware.
//
// Why 500 and not 501: oapi-codegen v2.7 does not emit a 501 wrapper for
// these operations. The shape is intentionally generic 500 — clients
// receive {"error":"internal","reason":"not_implemented_yet",
// "request_id":"..."} until Phase 4/5 ship.
//
// Forward-compat (D-70): when Phase 4 lands, it introduces
// services/api/internal/state/ with state.Handlers implementing ONLY
// these methods. A top-level ApiHandlers struct embeds catalog.Handlers
// + state.Handlers + imports.Handlers; Go's method-set resolution picks
// state.Handlers's real implementation over catalog.Handlers's stub
// once both exist. Phase 4 deletes GetAgentStatus + PatchAgentStatus
// from this file; Phase 5 deletes BulkImportCatalog + GetImportJob.
package catalog

import (
	"context"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// notImplementedBody is the canonical 500 "not_implemented_yet" envelope.
// Shared by both notimpl.go (Phase 4/5) and the per-entity placeholder
// files (Plans 03-06..03-09 replace).
func notImplementedBody() api.InternalServerErrorJSONResponse {
	return api.InternalServerErrorJSONResponse{
		Error:  api.ErrorCodeInternal,
		Reason: "not_implemented_yet",
	}
}

// GetAgentStatus — STATE-* (Phase 4). Real implementation in
// services/api/internal/state/ when Phase 4 lands.
func (h *Handlers) GetAgentStatus(_ context.Context, _ api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error) {
	return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// PatchAgentStatus — STATE-* (Phase 4). Real implementation in
// services/api/internal/state/ when Phase 4 lands.
func (h *Handlers) PatchAgentStatus(_ context.Context, _ api.PatchAgentStatusRequestObject) (api.PatchAgentStatusResponseObject, error) {
	return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// BulkImportCatalog — IMP-* (Phase 5). Real implementation in
// services/api/internal/imports/ when Phase 5 lands.
func (h *Handlers) BulkImportCatalog(_ context.Context, _ api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
	return api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// GetImportJob — IMP-* (Phase 5). Real implementation in
// services/api/internal/imports/ when Phase 5 lands.
func (h *Handlers) GetImportJob(_ context.Context, _ api.GetImportJobRequestObject) (api.GetImportJobResponseObject, error) {
	return api.GetImportJob500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}
