// queues.go — CAT-04 + CAT-08 + CAT-09 + CAT-10 + CAT-11. Mirrors the
// canonical agents.go template; queues has no join table so the BeginTx
// dance is dropped and only the version-checked UPDATE retains tx-free
// atomicity (D-66).
//
// Queue-specific shapes:
//
//   - channel_types TEXT[] — sqlc emits []string. The api DTO uses
//     []api.ChannelType which is a string alias, so the conversion is a
//     straight cast through a temporary slice both directions.
//
//   - priority + acw_sec — int32 in sqlc, int in api. Straight cast.
//
//   - No 422 wire shape on CreateQueue/UpdateQueue: the queue schema has
//     no FK to other catalog tables and no CHECK constraints, so a 23503/
//     23514 pg error here is a true 500 (schema drift, not a malformed
//     input the client can correct).
package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// CreateQueue — POST /v1/orgs/{org_id}/queues (CAT-04).
func (h *Handlers) CreateQueue(ctx context.Context, req api.CreateQueueRequestObject) (api.CreateQueueResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.CreateQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.CreateQueue400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}

	id := uuid.Must(uuid.NewV7())
	ctStrings := make([]string, 0, len(req.Body.ChannelTypes))
	for _, ct := range req.Body.ChannelTypes {
		ctStrings = append(ctStrings, string(ct))
	}
	priority, ok := int32Checked(req.Body.Priority)
	if !ok {
		return api.CreateQueue400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "priority_out_of_range",
		}}, nil
	}
	acwSec, ok := int32Checked(req.Body.AcwSec)
	if !ok {
		return api.CreateQueue400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "acw_sec_out_of_range",
		}}, nil
	}

	q := generated.New(h.deps.OrgDB)
	row, err := q.InsertQueue(ctx, generated.InsertQueueParams{
		ID:           pgUUID(id),
		OrgID:        pgUUID(orgID),
		ExternalID:   req.Body.ExternalId,
		Name:         req.Body.Name,
		ChannelTypes: ctStrings,
		Priority:     priority,
		AcwSec:       acwSec,
		Enabled:      derefOr(req.Body.Enabled, true),
	})
	if err != nil {
		status, code, reason := mapPgError(err, "queue")
		if status == 409 {
			return api.CreateQueue409JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
		}
		h.deps.Logger.ErrorContext(ctx, "create queue insert", "err", err)
		return api.CreateQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	// D-55 — DEL kept uniform across mutations even though the cache is
	// empty for a freshly-minted id.
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "queues", id)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "queues", id), "err", delErr)
	}

	return api.CreateQueue201JSONResponse(mapQueue(row)), nil
}

// GetQueue — GET /v1/orgs/{org_id}/queues/{id} (CAT-04, CAT-11).
func (h *Handlers) GetQueue(ctx context.Context, req api.GetQueueRequestObject) (api.GetQueueResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	queueID := uuid.UUID(req.Id)
	key := cache.Key(orgID, "queues", queueID)

	queue, err := cache.GetOrSet[api.Queue](ctx, h.deps.Cache, key, cacheTTL,
		func(ctx context.Context) (api.Queue, error) {
			q := generated.New(h.deps.OrgDB)
			row, ferr := q.GetQueue(ctx, generated.GetQueueParams{
				ID:    pgUUID(queueID),
				OrgID: pgUUID(orgID),
			})
			if errors.Is(ferr, pgx.ErrNoRows) {
				return api.Queue{}, cache.ErrNotFound
			}
			if ferr != nil {
				return api.Queue{}, ferr
			}
			// Detail GETs never reveal a soft-deleted row (CAT-09).
			if !row.Enabled {
				return api.Queue{}, cache.ErrNotFound
			}
			return mapQueue(row), nil
		})

	switch {
	case errors.Is(err, cache.ErrNotFound):
		return api.GetQueue404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "queue_not_found",
		}}, nil
	case err != nil:
		h.deps.Logger.ErrorContext(ctx, "get queue", "err", err)
		return api.GetQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	return api.GetQueue200JSONResponse(queue), nil
}

// ListQueues — GET /v1/orgs/{org_id}/queues (CAT-09, CAT-10).
func (h *Handlers) ListQueues(ctx context.Context, req api.ListQueuesRequestObject) (api.ListQueuesResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListQueues500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}

	pageSize := defaultPageSize
	if req.Params.Limit != nil {
		pageSize = int(*req.Params.Limit)
		if pageSize < 1 || pageSize > maxPageSize {
			return api.ListQueues400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
				Error:  api.ErrorCodeInvalidBody,
				Reason: fmt.Sprintf("limit_out_of_range:%d", pageSize),
			}}, nil
		}
	}

	var cursorStr string
	if req.Params.Cursor != nil {
		cursorStr = *req.Params.Cursor
	}
	cur, err := DecodeCursor(cursorStr)
	if err != nil {
		return api.ListQueues400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "bad_cursor",
		}}, nil
	}

	var nameFilter *string
	if req.Params.Name != nil {
		s := string(*req.Params.Name)
		nameFilter = &s
	}

	q := generated.New(h.deps.OrgDB)
	includeDisabled := derefOr(req.Params.IncludeDisabled, false)
	limit := mustInt32(pageSize + 1) // N+1 sentinel

	var rows []generated.Queue
	if includeDisabled {
		rs, e := q.ListQueuesIncludingDisabled(ctx, generated.ListQueuesIncludingDisabledParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list queues (include_disabled)", "err", e)
			return api.ListQueues500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	} else {
		rs, e := q.ListQueues(ctx, generated.ListQueuesParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list queues", "err", e)
			return api.ListQueues500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	}

	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	items := make([]api.Queue, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapQueue(r))
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		enc, encErr := EncodeCursor(last.CreatedAt.Time, apiUUID(last.ID))
		if encErr != nil {
			h.deps.Logger.ErrorContext(ctx, "list queues encode cursor", "err", encErr)
			return api.ListQueues500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "cursor_encode_failed",
			}}, nil
		}
		nextCursor = &enc
	}
	return api.ListQueues200JSONResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateQueue — PATCH /v1/orgs/{org_id}/queues/{id} (CAT-04, CAT-08).
// D-66 disambiguation in the same orgDB session so the probe sees the
// state the version-checked UPDATE just observed.
func (h *Handlers) UpdateQueue(ctx context.Context, req api.UpdateQueueRequestObject) (api.UpdateQueueResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.UpdateQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.UpdateQueue400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}
	queueID := uuid.UUID(req.Id)

	// Codex C3 sparse PATCH: pass nil → COALESCE preserves the column.
	// For channel_types the slice itself is the omission signal — sqlc
	// treats a nil []string as SQL NULL on the parameter side.
	var ctStrings []string
	if req.Body.ChannelTypes != nil {
		ctStrings = make([]string, 0, len(*req.Body.ChannelTypes))
		for _, ct := range *req.Body.ChannelTypes {
			ctStrings = append(ctStrings, string(ct))
		}
	}

	var priority *int32
	if req.Body.Priority != nil {
		v, ok := int32Checked(*req.Body.Priority)
		if !ok {
			return api.UpdateQueue400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
				Error: api.ErrorCodeInvalidBody, Reason: "priority_out_of_range",
			}}, nil
		}
		priority = &v
	}
	var acwSec *int32
	if req.Body.AcwSec != nil {
		v, ok := int32Checked(*req.Body.AcwSec)
		if !ok {
			return api.UpdateQueue400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
				Error: api.ErrorCodeInvalidBody, Reason: "acw_sec_out_of_range",
			}}, nil
		}
		acwSec = &v
	}

	q := generated.New(h.deps.OrgDB)
	expectedVersion, ok := int32Checked(req.Body.Version)
	if !ok {
		return api.UpdateQueue400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "version_out_of_range",
		}}, nil
	}
	row, err := q.UpdateQueue(ctx, generated.UpdateQueueParams{
		ID:              pgUUID(queueID),
		OrgID:           pgUUID(orgID),
		ExpectedVersion: expectedVersion,
		Name:            req.Body.Name,
		ChannelTypes:    ctStrings,
		Priority:        priority,
		AcwSec:          acwSec,
		Enabled:         req.Body.Enabled,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		cur, perr := q.GetQueueByIdAnyVersion(ctx, generated.GetQueueByIdAnyVersionParams{
			ID:    pgUUID(queueID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(perr, pgx.ErrNoRows) {
			return api.UpdateQueue404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
				Error: api.ErrorCodeNotFound, Reason: "queue_not_found",
			}}, nil
		}
		if perr != nil {
			h.deps.Logger.ErrorContext(ctx, "update queue disambiguate", "err", perr)
			return api.UpdateQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "disambiguation_failed",
			}}, nil
		}
		// D-56 — DEL on the 409 path so a stale cached value cannot mask
		// the conflict on the client's retry.
		if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "queues", queueID)); delErr != nil {
			h.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", "key", cache.Key(orgID, "queues", queueID), "err", delErr)
		}
		return api.UpdateQueue409JSONResponse{
			Current: mapQueue(cur),
			Error:   api.UpdateQueue409JSONResponseBodyErrorVersionConflict,
			Reason:  "version_mismatch",
		}, nil
	}
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "update queue", "err", err)
		return api.UpdateQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}

	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "queues", queueID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "queues", queueID), "err", delErr)
	}
	return api.UpdateQueue200JSONResponse(mapQueue(row)), nil
}

// DeleteQueue — DELETE /v1/orgs/{org_id}/queues/{id} (CAT-04, CAT-09).
func (h *Handlers) DeleteQueue(ctx context.Context, req api.DeleteQueueRequestObject) (api.DeleteQueueResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.DeleteQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	queueID := uuid.UUID(req.Id)

	q := generated.New(h.deps.OrgDB)
	tag, err := q.SoftDeleteQueue(ctx, generated.SoftDeleteQueueParams{
		ID:    pgUUID(queueID),
		OrgID: pgUUID(orgID),
	})
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "soft delete queue", "err", err)
		return api.DeleteQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	if tag == 0 {
		return api.DeleteQueue404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "queue_not_found",
		}}, nil
	}
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "queues", queueID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "queues", queueID), "err", delErr)
	}
	return api.DeleteQueue204Response{}, nil
}

// mapQueue converts a sqlc-row Queue into the api.Queue DTO. channel_types
// is the only non-trivial conversion: sqlc emits []string for TEXT[] and
// the api layer requires []api.ChannelType (string alias).
func mapQueue(row generated.Queue) api.Queue {
	cts := make([]api.ChannelType, 0, len(row.ChannelTypes))
	for _, ct := range row.ChannelTypes {
		cts = append(cts, api.ChannelType(ct))
	}
	return api.Queue{
		Id:           api.UUIDv7(apiUUID(row.ID)),
		OrgId:        api.UUIDv7(apiUUID(row.OrgID)),
		ExternalId:   row.ExternalID,
		Name:         row.Name,
		ChannelTypes: cts,
		Priority:     int(row.Priority),
		AcwSec:       int(row.AcwSec),
		Enabled:      row.Enabled,
		Version:      int(row.Version),
		CreatedAt:    ptrTime(row.CreatedAt),
		UpdatedAt:    ptrTime(row.UpdatedAt),
	}
}
