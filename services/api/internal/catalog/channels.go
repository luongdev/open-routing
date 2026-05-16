// channels.go — PLACEHOLDER entity file (Plan 03-05 scaffold).
//
// Plan 03-08 (Wave 4) REPLACES the bodies of all 5 methods below with
// the real implementations (CAT-05, CAT-08, CAT-09, CAT-10).
package catalog

import (
	"context"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// ListChannels — PLACEHOLDER replaced by Plan 03-08 (CAT-05, CAT-09, CAT-10).
func (h *Handlers) ListChannels(_ context.Context, _ api.ListChannelsRequestObject) (api.ListChannelsResponseObject, error) {
	return api.ListChannels500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// CreateChannel — PLACEHOLDER replaced by Plan 03-08 (CAT-05).
func (h *Handlers) CreateChannel(_ context.Context, _ api.CreateChannelRequestObject) (api.CreateChannelResponseObject, error) {
	return api.CreateChannel500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// DeleteChannel — PLACEHOLDER replaced by Plan 03-08 (CAT-05, CAT-09).
func (h *Handlers) DeleteChannel(_ context.Context, _ api.DeleteChannelRequestObject) (api.DeleteChannelResponseObject, error) {
	return api.DeleteChannel500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// GetChannel — PLACEHOLDER replaced by Plan 03-08 (CAT-05, CAT-11).
func (h *Handlers) GetChannel(_ context.Context, _ api.GetChannelRequestObject) (api.GetChannelResponseObject, error) {
	return api.GetChannel500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// UpdateChannel — PLACEHOLDER replaced by Plan 03-08 (CAT-05, CAT-08).
func (h *Handlers) UpdateChannel(_ context.Context, _ api.UpdateChannelRequestObject) (api.UpdateChannelResponseObject, error) {
	return api.UpdateChannel500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}
