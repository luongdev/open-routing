// queues.go — PLACEHOLDER entity file (Plan 03-05 scaffold).
//
// Plan 03-08 (Wave 4) REPLACES the bodies of all 5 methods below with
// the real implementations (CAT-04, CAT-08, CAT-09, CAT-10).
package catalog

import (
	"context"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// ListQueues — PLACEHOLDER replaced by Plan 03-08 (CAT-04, CAT-09, CAT-10).
func (h *Handlers) ListQueues(_ context.Context, _ api.ListQueuesRequestObject) (api.ListQueuesResponseObject, error) {
	return api.ListQueues500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// CreateQueue — PLACEHOLDER replaced by Plan 03-08 (CAT-04).
func (h *Handlers) CreateQueue(_ context.Context, _ api.CreateQueueRequestObject) (api.CreateQueueResponseObject, error) {
	return api.CreateQueue500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// DeleteQueue — PLACEHOLDER replaced by Plan 03-08 (CAT-04, CAT-09).
func (h *Handlers) DeleteQueue(_ context.Context, _ api.DeleteQueueRequestObject) (api.DeleteQueueResponseObject, error) {
	return api.DeleteQueue500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// GetQueue — PLACEHOLDER replaced by Plan 03-08 (CAT-04, CAT-11).
func (h *Handlers) GetQueue(_ context.Context, _ api.GetQueueRequestObject) (api.GetQueueResponseObject, error) {
	return api.GetQueue500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// UpdateQueue — PLACEHOLDER replaced by Plan 03-08 (CAT-04, CAT-08).
func (h *Handlers) UpdateQueue(_ context.Context, _ api.UpdateQueueRequestObject) (api.UpdateQueueResponseObject, error) {
	return api.UpdateQueue500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}
