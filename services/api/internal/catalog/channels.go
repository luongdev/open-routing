// channels.go — CAT-05 + CAT-08 + CAT-09 + CAT-10 + CAT-11. This is the
// first entity to exercise the D-76 cross-row FK validation pattern:
// CreateChannel and UpdateChannel call QueueExistsAndEnabledInOrg
// BEFORE writing so the 422 invalid_reference fires before any state
// mutation.
//
// Locked patterns from agents.go carry over (cache.GetOrSet on GET,
// D-66 disambiguation on UPDATE, D-55/D-56 cache DEL, soft-delete →
// 404 on detail GET). The D-76 probe-then-write hazard window (queue
// soft-deleted between probe and INSERT) is acceptable in v0.1 — the
// next GET surfaces the now-disabled queue and admins reconcile.
//
// Org-scope safety (FOUND-08): the probe runs through orgDB and is
// gated on `org_id = caller's orgID + enabled = TRUE`. A queue in a
// different org therefore returns ErrNoRows and the response is the
// SAME 422 invalid_reference as a missing queue — clients cannot
// distinguish "queue is in another org" from "queue doesn't exist".
package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// CreateChannel — POST /v1/orgs/{org_id}/channels (CAT-05).
// D-76 probe runs BEFORE InsertChannel so an unknown queue id never
// leaves an inconsistent row in the table.
func (h *Handlers) CreateChannel(ctx context.Context, req api.CreateChannelRequestObject) (api.CreateChannelResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.CreateChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.CreateChannel400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}

	q := generated.New(h.deps.OrgDB)

	// D-76 cross-row FK probe. Missing/disabled/cross-org queue all
	// surface as pgx.ErrNoRows here (FOUND-08 — no oracle for which).
	if req.Body.DefaultQueueId != nil {
		queueID := uuid.UUID(*req.Body.DefaultQueueId)
		_, qErr := q.QueueExistsAndEnabledInOrg(ctx, generated.QueueExistsAndEnabledInOrgParams{
			ID:    pgUUID(queueID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(qErr, pgx.ErrNoRows) {
			return api.CreateChannel422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidReference,
				Reason: "default_queue_id_not_found_or_disabled",
			}), nil
		}
		if qErr != nil {
			h.deps.Logger.ErrorContext(ctx, "queue exists probe", "err", qErr)
			return api.CreateChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "queue_probe_failed",
			}}, nil
		}
	}

	id := uuid.Must(uuid.NewV7())
	defQID := pgtype.UUID{Valid: false}
	if req.Body.DefaultQueueId != nil {
		defQID = pgUUID(uuid.UUID(*req.Body.DefaultQueueId))
	}

	row, err := q.InsertChannel(ctx, generated.InsertChannelParams{
		ID:             pgUUID(id),
		OrgID:          pgUUID(orgID),
		ExternalID:     req.Body.ExternalId,
		Name:           req.Body.Name,
		ChannelType:    string(req.Body.ChannelType),
		DefaultQueueID: defQID,
		Enabled:        derefOr(req.Body.Enabled, true),
	})
	if err != nil {
		status, code, reason := mapPgError(err, "channel")
		switch status {
		case 409:
			return api.CreateChannel409JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
		case 422:
			// D-76 probe-write race: queue disabled between probe and
			// INSERT. Schema-level FK fires (23503) → 422 invalid_reference.
			return api.CreateChannel422JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
		}
		h.deps.Logger.ErrorContext(ctx, "create channel insert", "err", err)
		return api.CreateChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "channels", id)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "channels", id), "err", delErr)
	}
	return api.CreateChannel201JSONResponse(mapChannel(row)), nil
}

// GetChannel — GET /v1/orgs/{org_id}/channels/{id} (CAT-05, CAT-11).
func (h *Handlers) GetChannel(ctx context.Context, req api.GetChannelRequestObject) (api.GetChannelResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	channelID := uuid.UUID(req.Id)
	key := cache.Key(orgID, "channels", channelID)

	channel, err := cache.GetOrSet[api.Channel](ctx, h.deps.Cache, key, cacheTTL,
		func(ctx context.Context) (api.Channel, error) {
			q := generated.New(h.deps.OrgDB)
			row, ferr := q.GetChannel(ctx, generated.GetChannelParams{
				ID:    pgUUID(channelID),
				OrgID: pgUUID(orgID),
			})
			if errors.Is(ferr, pgx.ErrNoRows) {
				return api.Channel{}, cache.ErrNotFound
			}
			if ferr != nil {
				return api.Channel{}, ferr
			}
			// Detail GETs hide soft-deleted rows (CAT-09).
			if !row.Enabled {
				return api.Channel{}, cache.ErrNotFound
			}
			return mapChannel(row), nil
		})

	switch {
	case errors.Is(err, cache.ErrNotFound):
		return api.GetChannel404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "channel_not_found",
		}}, nil
	case err != nil:
		h.deps.Logger.ErrorContext(ctx, "get channel", "err", err)
		return api.GetChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	return api.GetChannel200JSONResponse(channel), nil
}

// ListChannels — GET /v1/orgs/{org_id}/channels (CAT-09, CAT-10).
func (h *Handlers) ListChannels(ctx context.Context, req api.ListChannelsRequestObject) (api.ListChannelsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListChannels500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}

	pageSize := defaultPageSize
	if req.Params.Limit != nil {
		pageSize = int(*req.Params.Limit)
		if pageSize < 1 || pageSize > maxPageSize {
			return api.ListChannels400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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
		return api.ListChannels400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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
	limit := mustInt32(pageSize + 1)

	var rows []generated.Channel
	if includeDisabled {
		rs, e := q.ListChannelsIncludingDisabled(ctx, generated.ListChannelsIncludingDisabledParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list channels (include_disabled)", "err", e)
			return api.ListChannels500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	} else {
		rs, e := q.ListChannels(ctx, generated.ListChannelsParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list channels", "err", e)
			return api.ListChannels500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	}

	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	items := make([]api.Channel, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapChannel(r))
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		enc, encErr := EncodeCursor(last.CreatedAt.Time, apiUUID(last.ID))
		if encErr != nil {
			h.deps.Logger.ErrorContext(ctx, "list channels encode cursor", "err", encErr)
			return api.ListChannels500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "cursor_encode_failed",
			}}, nil
		}
		nextCursor = &enc
	}
	return api.ListChannels200JSONResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateChannel — PATCH /v1/orgs/{org_id}/channels/{id} (CAT-05, CAT-08).
// D-76 probe runs only when default_queue_id is supplied (sparse PATCH —
// omitting the field preserves the existing value via COALESCE).
func (h *Handlers) UpdateChannel(ctx context.Context, req api.UpdateChannelRequestObject) (api.UpdateChannelResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.UpdateChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.UpdateChannel400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}
	channelID := uuid.UUID(req.Id)
	q := generated.New(h.deps.OrgDB)

	// D-76 — same probe as CreateChannel. Only validate when the client
	// is changing default_queue_id (nil = preserve existing — no probe).
	if req.Body.DefaultQueueId != nil {
		queueID := uuid.UUID(*req.Body.DefaultQueueId)
		_, qErr := q.QueueExistsAndEnabledInOrg(ctx, generated.QueueExistsAndEnabledInOrgParams{
			ID:    pgUUID(queueID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(qErr, pgx.ErrNoRows) {
			return api.UpdateChannel422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidReference,
				Reason: "default_queue_id_not_found_or_disabled",
			}), nil
		}
		if qErr != nil {
			h.deps.Logger.ErrorContext(ctx, "queue exists probe", "err", qErr)
			return api.UpdateChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "queue_probe_failed",
			}}, nil
		}
	}

	var channelTypeStr *string
	if req.Body.ChannelType != nil {
		s := string(*req.Body.ChannelType)
		channelTypeStr = &s
	}

	defQID := pgtype.UUID{Valid: false}
	if req.Body.DefaultQueueId != nil {
		defQID = pgUUID(uuid.UUID(*req.Body.DefaultQueueId))
	}
	expectedVersion, ok := int32Checked(req.Body.Version)
	if !ok {
		return api.UpdateChannel400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "version_out_of_range",
		}}, nil
	}

	row, err := q.UpdateChannel(ctx, generated.UpdateChannelParams{
		ID:              pgUUID(channelID),
		OrgID:           pgUUID(orgID),
		ExpectedVersion: expectedVersion,
		Name:            req.Body.Name,
		ChannelType:     channelTypeStr,
		DefaultQueueID:  defQID,
		Enabled:         req.Body.Enabled,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		cur, perr := q.GetChannelByIdAnyVersion(ctx, generated.GetChannelByIdAnyVersionParams{
			ID:    pgUUID(channelID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(perr, pgx.ErrNoRows) {
			return api.UpdateChannel404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
				Error: api.ErrorCodeNotFound, Reason: "channel_not_found",
			}}, nil
		}
		if perr != nil {
			h.deps.Logger.ErrorContext(ctx, "update channel disambiguate", "err", perr)
			return api.UpdateChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "disambiguation_failed",
			}}, nil
		}
		if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "channels", channelID)); delErr != nil {
			h.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", "key", cache.Key(orgID, "channels", channelID), "err", delErr)
		}
		return api.UpdateChannel409JSONResponse{
			Current: mapChannel(cur),
			Error:   api.UpdateChannel409JSONResponseBodyErrorVersionConflict,
			Reason:  "version_mismatch",
		}, nil
	}
	if err != nil {
		status, code, reason := mapPgError(err, "channel")
		if status == 422 {
			return api.UpdateChannel422JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
		}
		h.deps.Logger.ErrorContext(ctx, "update channel", "err", err)
		return api.UpdateChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "channels", channelID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "channels", channelID), "err", delErr)
	}
	return api.UpdateChannel200JSONResponse(mapChannel(row)), nil
}

// DeleteChannel — DELETE /v1/orgs/{org_id}/channels/{id} (CAT-05, CAT-09).
func (h *Handlers) DeleteChannel(ctx context.Context, req api.DeleteChannelRequestObject) (api.DeleteChannelResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.DeleteChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	channelID := uuid.UUID(req.Id)

	q := generated.New(h.deps.OrgDB)
	tag, err := q.SoftDeleteChannel(ctx, generated.SoftDeleteChannelParams{
		ID:    pgUUID(channelID),
		OrgID: pgUUID(orgID),
	})
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "soft delete channel", "err", err)
		return api.DeleteChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	if tag == 0 {
		return api.DeleteChannel404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "channel_not_found",
		}}, nil
	}
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "channels", channelID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "channels", channelID), "err", delErr)
	}
	return api.DeleteChannel204Response{}, nil
}

func mapChannel(row generated.Channel) api.Channel {
	var defQID *api.UUIDv7
	if row.DefaultQueueID.Valid {
		v := api.UUIDv7(apiUUID(row.DefaultQueueID))
		defQID = &v
	}
	return api.Channel{
		Id:             api.UUIDv7(apiUUID(row.ID)),
		OrgId:          api.UUIDv7(apiUUID(row.OrgID)),
		ExternalId:     row.ExternalID,
		Name:           row.Name,
		ChannelType:    api.ChannelType(row.ChannelType),
		DefaultQueueId: defQID,
		Enabled:        row.Enabled,
		Version:        int(row.Version),
		CreatedAt:      ptrTime(row.CreatedAt),
		UpdatedAt:      ptrTime(row.UpdatedAt),
	}
}
