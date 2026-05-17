// break_reasons.go — CAT-07 + CAT-08 + CAT-09 + CAT-10 + CAT-11. Wave 4
// entity modeled after agents.go/skills.go. Phase 04.1 added `code` (required)
// and `external_id` (optional, nullable) columns; the pre-04.1
// `UNIQUE(org_id, name)` was dropped in Plan 01 so the name-collision wire
// reason is replaced by the standard duplicate_code / duplicate_external_id
// constraint-name-driven mapping in errors.go (D04_1-21).
//
// Locked patterns inherited from agents.go:
//
//   - D-66 — version-checked UPDATE 0-row → GetBreakReasonByIdAnyVersion
//     in the same tx. 409 path also cache.Dels (D-56).
//   - D-55 — cache.Del after every mutation commit (and the 409 path).
//   - CAT-11 — GetBreakReason via cache.GetOrSet[api.BreakReason] under
//     `or:{orgID}:break_reasons:{id}` (D-58 entity slug verbatim,
//     underscore form to match the table name) with 60s TTL.
//   - CAT-10 — cursor + LIMIT N+1; ?include_disabled=true → IncludingDisabled.
//   - Phase 04.1 (D04_1-21): 23505 from UNIQUE(org_id, code) or
//     ix_break_reasons_org_external_id → 409 with duplicate_code /
//     duplicate_external_id (via mapPgError constraint-name introspect).
//     FLAT ErrorResponse wrapper (Pitfall 10).
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

// CreateBreakReason — POST /v1/orgs/{org_id}/break-reasons (CAT-07).
func (h *Handlers) CreateBreakReason(ctx context.Context, req api.CreateBreakReasonRequestObject) (api.CreateBreakReasonResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.CreateBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.CreateBreakReason400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}

	// Phase 04.1 Layer 1 (D04_1-19) — handler enforces D04_1-03 code regex
	// because oapi-codegen v2 does NOT auto-enforce the OpenAPI `pattern`.
	// break_reasons previously had no `code` column — Phase 04.1 makes it required.
	if !validateCodeFormat(req.Body.Code) {
		return api.CreateBreakReason400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error:  api.ErrorCodeInvalidBody,
			Reason: "invalid_code_format",
		}}, nil
	}

	id := uuid.Must(uuid.NewV7())
	enabled := derefOr(req.Body.Enabled, true)
	displayOrder, ok := int32Checked(req.Body.DisplayOrder)
	if !ok {
		return api.CreateBreakReason400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "display_order_out_of_range",
		}}, nil
	}

	q := generated.New(h.deps.OrgDB)
	row, err := q.InsertBreakReason(ctx, generated.InsertBreakReasonParams{
		ID:           pgUUID(id),
		OrgID:        pgUUID(orgID),
		Code:         req.Body.Code,
		ExternalID:   req.Body.ExternalId, // NEW for break_reasons in Phase 04.1 (column nullable).
		Name:         req.Body.Name,
		Routable:     req.Body.Routable,
		DisplayOrder: displayOrder,
		Enabled:      enabled,
	})
	if err != nil {
		status, code, reason := mapPgError(err, "break_reason")
		if status == 409 {
			// Phase 04.1 — pre-04.1 the 409 reason was hardcoded to a
			// fixed name-collision string because UNIQUE(org_id, name) was the
			// only constraint. Plan 01 dropped that constraint; the new
			// uniqueness is on (org_id, code) and the partial
			// (org_id, external_id). mapPgError now returns the
			// duplicate_code / duplicate_external_id reason directly via
			// constraint-name introspection (see errors.go D04_1-21).
			return api.CreateBreakReason409JSONResponse(api.ErrorResponse{
				Error: code, Reason: reason,
			}), nil
		}
		h.deps.Logger.ErrorContext(ctx, "create break_reason insert", "err", err)
		return api.CreateBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "insert_failed",
		}}, nil
	}

	// D-55 — DEL after commit, idempotent on cold cache.
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "break_reasons", id)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "break_reasons", id), "err", delErr)
	}

	return api.CreateBreakReason201JSONResponse(mapBreakReason(row)), nil
}

// GetBreakReason — GET /v1/orgs/{org_id}/break-reasons/{id} (CAT-07, CAT-11).
func (h *Handlers) GetBreakReason(ctx context.Context, req api.GetBreakReasonRequestObject) (api.GetBreakReasonResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	brID := uuid.UUID(req.Id)
	key := cache.Key(orgID, "break_reasons", brID)

	br, err := cache.GetOrSet[api.BreakReason](ctx, h.deps.Cache, key, cacheTTL,
		func(ctx context.Context) (api.BreakReason, error) {
			q := generated.New(h.deps.OrgDB)
			row, ferr := q.GetBreakReason(ctx, generated.GetBreakReasonParams{
				ID:    pgUUID(brID),
				OrgID: pgUUID(orgID),
			})
			if errors.Is(ferr, pgx.ErrNoRows) {
				return api.BreakReason{}, cache.ErrNotFound
			}
			if ferr != nil {
				return api.BreakReason{}, ferr
			}
			// Soft-deleted rows surface as 404 on detail (CAT-09).
			if !row.Enabled {
				return api.BreakReason{}, cache.ErrNotFound
			}
			return mapBreakReason(row), nil
		})

	switch {
	case errors.Is(err, cache.ErrNotFound):
		return api.GetBreakReason404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "break_reason_not_found",
		}}, nil
	case err != nil:
		h.deps.Logger.ErrorContext(ctx, "get break_reason", "err", err)
		return api.GetBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	return api.GetBreakReason200JSONResponse(br), nil
}

// ListBreakReasons — GET /v1/orgs/{org_id}/break-reasons (CAT-09, CAT-10).
// Note: spec uses cursor pagination ordered by (created_at, id) DESC. The
// display_order column drives the agent-status-picker UI sort path; the
// list-API default still uses cursor ordering for stable pagination.
func (h *Handlers) ListBreakReasons(ctx context.Context, req api.ListBreakReasonsRequestObject) (api.ListBreakReasonsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListBreakReasons500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}

	pageSize := defaultPageSize
	if req.Params.Limit != nil {
		pageSize = int(*req.Params.Limit)
		if pageSize < 1 || pageSize > maxPageSize {
			return api.ListBreakReasons400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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
		return api.ListBreakReasons400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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

	var rows []generated.BreakReason
	if includeDisabled {
		rs, e := q.ListBreakReasonsIncludingDisabled(ctx, generated.ListBreakReasonsIncludingDisabledParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list break_reasons (include_disabled)", "err", e)
			return api.ListBreakReasons500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	} else {
		rs, e := q.ListBreakReasons(ctx, generated.ListBreakReasonsParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list break_reasons", "err", e)
			return api.ListBreakReasons500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	}

	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	items := make([]api.BreakReason, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapBreakReason(r))
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		enc, encErr := EncodeCursor(last.CreatedAt.Time, apiUUID(last.ID))
		if encErr != nil {
			// has_more=true + nil next_cursor is an unrecoverable wire
			// shape for the client.
			h.deps.Logger.ErrorContext(ctx, "list break_reasons encode cursor", "err", encErr)
			return api.ListBreakReasons500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "cursor_encode_failed",
			}}, nil
		}
		nextCursor = &enc
	}
	return api.ListBreakReasons200JSONResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateBreakReason — PATCH /v1/orgs/{org_id}/break-reasons/{id}
// (CAT-07, CAT-08).
func (h *Handlers) UpdateBreakReason(ctx context.Context, req api.UpdateBreakReasonRequestObject) (api.UpdateBreakReasonResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.UpdateBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.UpdateBreakReason400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}
	brID := uuid.UUID(req.Id)

	// Phase 04.1 Layer 1 (D04_1-19) — fail fast on malformed code BEFORE any
	// DB call so a malformed PATCH gets 400, not 422.
	if req.Body.Code != nil {
		if !validateCodeFormat(*req.Body.Code) {
			return api.UpdateBreakReason400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
				Error:  api.ErrorCodeInvalidBody,
				Reason: "invalid_code_format",
			}}, nil
		}
	}

	// Single-table entity: no junction to coordinate (Wave 5 review —
	// dropped the BeginTx wrapper for consistency with queues/channels/
	// adapters). MVCC at the row level is sufficient for D-66.
	q := generated.New(h.deps.OrgDB)

	// Phase 04.1 Layer 2 (D04_1-20) — immutability gate. When req.Body.Code
	// is present, fetch the stored row first; reuse it in the 0-row branch
	// instead of a second probe. nil → preserve Phase 3 post-UPDATE flow.
	var stored *generated.BreakReason
	if req.Body.Code != nil {
		s, perr := q.GetBreakReasonByIdAnyVersion(ctx, generated.GetBreakReasonByIdAnyVersionParams{
			ID:    pgUUID(brID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(perr, pgx.ErrNoRows) {
			return api.UpdateBreakReason404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
				Error: api.ErrorCodeNotFound, Reason: "break_reason_not_found",
			}}, nil
		}
		if perr != nil {
			h.deps.Logger.ErrorContext(ctx, "load stored break_reason for immutability check", "err", perr)
			return api.UpdateBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "load_stored_failed",
			}}, nil
		}
		stored = &s
		if !validateImmutableCode(stored.Code, *req.Body.Code) {
			return api.UpdateBreakReason422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeImmutableField,
				Reason: "code",
			}), nil
		}
	}

	var displayOrder *int32
	if req.Body.DisplayOrder != nil {
		v, ok := int32Checked(*req.Body.DisplayOrder)
		if !ok {
			return api.UpdateBreakReason400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
				Error: api.ErrorCodeInvalidBody, Reason: "display_order_out_of_range",
			}}, nil
		}
		displayOrder = &v
	}

	expectedVersion, ok := int32Checked(req.Body.Version)
	if !ok {
		return api.UpdateBreakReason400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "version_out_of_range",
		}}, nil
	}
	row, err := q.UpdateBreakReason(ctx, generated.UpdateBreakReasonParams{
		ID:              pgUUID(brID),
		OrgID:           pgUUID(orgID),
		ExpectedVersion: expectedVersion,
		Name:            req.Body.Name,
		Routable:        req.Body.Routable,
		DisplayOrder:    displayOrder,
		Enabled:         req.Body.Enabled,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Phase 04.1: reuse `stored` if pre-UPDATE fetch already ran
		// (Branch 1: req.Body.Code != nil). Otherwise probe (Branch 2).
		var cur generated.BreakReason
		if stored != nil {
			cur = *stored
		} else {
			s2, perr := q.GetBreakReasonByIdAnyVersion(ctx, generated.GetBreakReasonByIdAnyVersionParams{
				ID:    pgUUID(brID),
				OrgID: pgUUID(orgID),
			})
			if errors.Is(perr, pgx.ErrNoRows) {
				return api.UpdateBreakReason404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
					Error: api.ErrorCodeNotFound, Reason: "break_reason_not_found",
				}}, nil
			}
			if perr != nil {
				h.deps.Logger.ErrorContext(ctx, "update break_reason disambiguate", "err", perr)
				return api.UpdateBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
					Error: api.ErrorCodeInternal, Reason: "disambiguation_failed",
				}}, nil
			}
			cur = s2
		}
		// D-56 — DEL on 409 so a stale cached value can't mask the conflict.
		if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "break_reasons", brID)); delErr != nil {
			h.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", "key", cache.Key(orgID, "break_reasons", brID), "err", delErr)
		}
		return api.UpdateBreakReason409JSONResponse{
			Current: mapBreakReason(cur),
			Error:   api.UpdateBreakReason409JSONResponseBodyErrorVersionConflict,
			Reason:  "version_mismatch",
		}, nil
	}
	if err != nil {
		status, code, reason := mapPgError(err, "break_reason")
		if status == 422 {
			return api.UpdateBreakReason422JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
		}
		h.deps.Logger.ErrorContext(ctx, "update break_reason", "err", err)
		return api.UpdateBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "break_reasons", brID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "break_reasons", brID), "err", delErr)
	}

	return api.UpdateBreakReason200JSONResponse(mapBreakReason(row)), nil
}

// DeleteBreakReason — DELETE /v1/orgs/{org_id}/break-reasons/{id}
// (CAT-07, CAT-09).
func (h *Handlers) DeleteBreakReason(ctx context.Context, req api.DeleteBreakReasonRequestObject) (api.DeleteBreakReasonResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.DeleteBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	brID := uuid.UUID(req.Id)

	q := generated.New(h.deps.OrgDB)
	tag, err := q.SoftDeleteBreakReason(ctx, generated.SoftDeleteBreakReasonParams{
		ID:    pgUUID(brID),
		OrgID: pgUUID(orgID),
	})
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "soft delete break_reason", "err", err)
		return api.DeleteBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	if tag == 0 {
		return api.DeleteBreakReason404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "break_reason_not_found",
		}}, nil
	}
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "break_reasons", brID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "break_reasons", brID), "err", delErr)
	}
	return api.DeleteBreakReason204Response{}, nil
}

func mapBreakReason(row generated.BreakReason) api.BreakReason {
	return api.BreakReason{
		Id:           api.UUIDv7(apiUUID(row.ID)),
		OrgId:        api.UUIDv7(apiUUID(row.OrgID)),
		Code:         row.Code,
		ExternalId:   row.ExternalID,
		Name:         row.Name,
		Routable:     row.Routable,
		DisplayOrder: int(row.DisplayOrder),
		Enabled:      row.Enabled,
		Version:      int(row.Version),
		CreatedAt:    ptrTime(row.CreatedAt),
		UpdatedAt:    ptrTime(row.UpdatedAt),
	}
}
