// notimpl.go — 501-equivalent stubs for endpoints owned by Phase 5
// (bulk import).
//
// Phase 4 deleted: GetAgentStatus + PatchAgentStatus 501 stubs.
// state.Server owns these methods; cmd/api/main.go composes
// *catalog.Handlers + *state.Server into ApiHandlers (D-89).
//
// Remaining stubs are Phase 5 (bulk import) — kept until Phase 5 lands.
package catalog

import (
	"context"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// notImplementedBody is the canonical 500 "not_implemented_yet" envelope.
// Used by Phase 5 stubs below until Phase 5 ships and deletes this file.
func notImplementedBody() api.InternalServerErrorJSONResponse {
	return api.InternalServerErrorJSONResponse{
		Error:  api.ErrorCodeInternal,
		Reason: "not_implemented_yet",
	}
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
