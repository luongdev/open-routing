// break_reasons.go — PLACEHOLDER entity file (Plan 03-05 scaffold).
//
// Plan 03-07 (Wave 4) REPLACES the bodies of all 5 methods below with
// the real implementations (CAT-07, CAT-08, CAT-09, CAT-10).
package catalog

import (
	"context"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// ListBreakReasons — PLACEHOLDER replaced by Plan 03-07 (CAT-07, CAT-09, CAT-10).
func (h *Handlers) ListBreakReasons(_ context.Context, _ api.ListBreakReasonsRequestObject) (api.ListBreakReasonsResponseObject, error) {
	return api.ListBreakReasons500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// CreateBreakReason — PLACEHOLDER replaced by Plan 03-07 (CAT-07).
func (h *Handlers) CreateBreakReason(_ context.Context, _ api.CreateBreakReasonRequestObject) (api.CreateBreakReasonResponseObject, error) {
	return api.CreateBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// DeleteBreakReason — PLACEHOLDER replaced by Plan 03-07 (CAT-07, CAT-09).
func (h *Handlers) DeleteBreakReason(_ context.Context, _ api.DeleteBreakReasonRequestObject) (api.DeleteBreakReasonResponseObject, error) {
	return api.DeleteBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// GetBreakReason — PLACEHOLDER replaced by Plan 03-07 (CAT-07, CAT-11).
func (h *Handlers) GetBreakReason(_ context.Context, _ api.GetBreakReasonRequestObject) (api.GetBreakReasonResponseObject, error) {
	return api.GetBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// UpdateBreakReason — PLACEHOLDER replaced by Plan 03-07 (CAT-07, CAT-08).
func (h *Handlers) UpdateBreakReason(_ context.Context, _ api.UpdateBreakReasonRequestObject) (api.UpdateBreakReasonResponseObject, error) {
	return api.UpdateBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}
