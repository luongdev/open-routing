// adapters.go — PLACEHOLDER entity file (Plan 03-05 scaffold).
//
// Plan 03-09 (Wave 5) REPLACES the bodies of all 5 methods below with
// the real implementations (CAT-06, CAT-08, CAT-09, CAT-10).
package catalog

import (
	"context"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// ListAdapters — PLACEHOLDER replaced by Plan 03-09 (CAT-06, CAT-09, CAT-10).
func (h *Handlers) ListAdapters(_ context.Context, _ api.ListAdaptersRequestObject) (api.ListAdaptersResponseObject, error) {
	return api.ListAdapters500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// CreateAdapter — PLACEHOLDER replaced by Plan 03-09 (CAT-06).
func (h *Handlers) CreateAdapter(_ context.Context, _ api.CreateAdapterRequestObject) (api.CreateAdapterResponseObject, error) {
	return api.CreateAdapter500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// DeleteAdapter — PLACEHOLDER replaced by Plan 03-09 (CAT-06, CAT-09).
func (h *Handlers) DeleteAdapter(_ context.Context, _ api.DeleteAdapterRequestObject) (api.DeleteAdapterResponseObject, error) {
	return api.DeleteAdapter500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// GetAdapter — PLACEHOLDER replaced by Plan 03-09 (CAT-06, CAT-11).
func (h *Handlers) GetAdapter(_ context.Context, _ api.GetAdapterRequestObject) (api.GetAdapterResponseObject, error) {
	return api.GetAdapter500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// UpdateAdapter — PLACEHOLDER replaced by Plan 03-09 (CAT-06, CAT-08).
func (h *Handlers) UpdateAdapter(_ context.Context, _ api.UpdateAdapterRequestObject) (api.UpdateAdapterResponseObject, error) {
	return api.UpdateAdapter500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}
