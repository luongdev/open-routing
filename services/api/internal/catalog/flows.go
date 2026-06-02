// flows.go — v0.2 Stage 1 flow draft CRUD (FLOW). Mirrors the catalog entity
// template (queues.go): orgDB-scoped queries, version-checked UPDATE for
// tx-free optimistic locking (D-66), soft delete (CAT-09), cursor pagination
// (D-63), and the Redis entity cache (CAT-11).
//
// graph is stored opaque (JSONB) here; structural validation lands in Stage 2.
// The 409 path is simpler than the catalog's: a flow has no external_id, so
// version_conflict is the only possible update conflict (no oneOf union).
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

// CreateFlow — POST /v1/orgs/{org_id}/flows.
func (h *Handlers) CreateFlow(ctx context.Context, req api.CreateFlowRequestObject) (api.CreateFlowResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.CreateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.CreateFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}
	if !ValidateCodeFormat(req.Body.Code) {
		return api.CreateFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "invalid_code_format",
		}}, nil
	}

	graphBytes, ok := newFlowGraph(req.Body.Graph)
	if !ok {
		return api.CreateFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "invalid_graph",
		}}, nil
	}

	id := uuid.Must(uuid.NewV7())
	q := generated.New(h.deps.OrgDB)
	row, err := q.InsertFlow(ctx, generated.InsertFlowParams{
		ID:      pgUUID(id),
		OrgID:   pgUUID(orgID),
		Code:    req.Body.Code,
		Name:    req.Body.Name,
		Graph:   graphBytes,
		Enabled: derefOr(req.Body.Enabled, true),
	})
	if err != nil {
		status, code, reason := MapPgError(err, "flow")
		if status == 409 {
			return api.CreateFlow409JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
		}
		h.deps.Logger.ErrorContext(ctx, "create flow insert", "err", err)
		return api.CreateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "flows", id)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "flows", id), "err", delErr)
	}

	flow, mapErr := mapFlow(row)
	if mapErr != nil {
		h.deps.Logger.ErrorContext(ctx, "map created flow", "err", mapErr)
		return api.CreateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "map_failed",
		}}, nil
	}
	return api.CreateFlow201JSONResponse(flow), nil
}

// GetFlow — GET /v1/orgs/{org_id}/flows/{id}.
func (h *Handlers) GetFlow(ctx context.Context, req api.GetFlowRequestObject) (api.GetFlowResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	flowID := uuid.UUID(req.Id)
	key := cache.Key(orgID, "flows", flowID)

	flow, err := cache.GetOrSet[api.Flow](ctx, h.deps.Cache, key, cacheTTL,
		func(ctx context.Context) (api.Flow, error) {
			q := generated.New(h.deps.OrgDB)
			row, ferr := q.GetFlow(ctx, generated.GetFlowParams{
				ID:    pgUUID(flowID),
				OrgID: pgUUID(orgID),
			})
			if errors.Is(ferr, pgx.ErrNoRows) {
				return api.Flow{}, cache.ErrNotFound
			}
			if ferr != nil {
				return api.Flow{}, ferr
			}
			// Detail GETs never reveal a soft-deleted row (CAT-09).
			if !row.Enabled {
				return api.Flow{}, cache.ErrNotFound
			}
			return mapFlow(row)
		})

	switch {
	case errors.Is(err, cache.ErrNotFound):
		return api.GetFlow404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "flow_not_found",
		}}, nil
	case err != nil:
		h.deps.Logger.ErrorContext(ctx, "get flow", "err", err)
		return api.GetFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	return api.GetFlow200JSONResponse(flow), nil
}

// ListFlows — GET /v1/orgs/{org_id}/flows.
func (h *Handlers) ListFlows(ctx context.Context, req api.ListFlowsRequestObject) (api.ListFlowsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListFlows500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}

	pageSize := defaultPageSize
	if req.Params.Limit != nil {
		pageSize = int(*req.Params.Limit)
		if pageSize < 1 || pageSize > maxPageSize {
			return api.ListFlows400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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
		return api.ListFlows400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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

	var rows []generated.Flow
	if includeDisabled {
		rs, e := q.ListFlowsIncludingDisabled(ctx, generated.ListFlowsIncludingDisabledParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list flows (include_disabled)", "err", e)
			return api.ListFlows500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	} else {
		rs, e := q.ListFlows(ctx, generated.ListFlowsParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list flows", "err", e)
			return api.ListFlows500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	}

	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	items := make([]api.Flow, 0, len(rows))
	for _, r := range rows {
		flow, mapErr := mapFlow(r)
		if mapErr != nil {
			h.deps.Logger.ErrorContext(ctx, "map flow row", "err", mapErr)
			return api.ListFlows500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "map_failed",
			}}, nil
		}
		items = append(items, flow)
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		enc, encErr := EncodeCursor(last.CreatedAt.Time, apiUUID(last.ID))
		if encErr != nil {
			h.deps.Logger.ErrorContext(ctx, "list flows encode cursor", "err", encErr)
			return api.ListFlows500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "cursor_encode_failed",
			}}, nil
		}
		nextCursor = &enc
	}
	return api.ListFlows200JSONResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateFlow — PATCH /v1/orgs/{org_id}/flows/{id}. D-66 version-checked UPDATE;
// `code` is immutable (422 on change).
func (h *Handlers) UpdateFlow(ctx context.Context, req api.UpdateFlowRequestObject) (api.UpdateFlowResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.UpdateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.UpdateFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}
	flowID := uuid.UUID(req.Id)

	if req.Body.Code != nil && !ValidateCodeFormat(*req.Body.Code) {
		return api.UpdateFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "invalid_code_format",
		}}, nil
	}

	var graphBytes []byte // nil → SQL NULL → COALESCE preserves the stored graph.
	if req.Body.Graph != nil {
		b, gErr := mapToJSONB(*req.Body.Graph)
		if gErr != nil {
			return api.UpdateFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
				Error: api.ErrorCodeInvalidBody, Reason: "invalid_graph",
			}}, nil
		}
		graphBytes = b
	}

	q := generated.New(h.deps.OrgDB)

	// Layer 2 immutability gate (D04_1-20): when code is present, fetch the
	// stored row first and reuse it on the 0-row branch instead of re-probing.
	if req.Body.Code != nil {
		s, perr := q.GetFlowByIdAnyVersion(ctx, generated.GetFlowByIdAnyVersionParams{
			ID:    pgUUID(flowID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(perr, pgx.ErrNoRows) {
			return api.UpdateFlow404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
				Error: api.ErrorCodeNotFound, Reason: "flow_not_found",
			}}, nil
		}
		if perr != nil {
			h.deps.Logger.ErrorContext(ctx, "load stored flow for immutability check", "err", perr)
			return api.UpdateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "load_stored_failed",
			}}, nil
		}
		if !validateImmutableCode(s.Code, *req.Body.Code) {
			return api.UpdateFlow422JSONResponse(api.ErrorResponse{
				Error: api.ErrorCodeImmutableField, Reason: "code",
			}), nil
		}
	}

	expectedVersion, ok := int32Checked(req.Body.Version)
	if !ok {
		return api.UpdateFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "version_out_of_range",
		}}, nil
	}

	row, err := q.UpdateFlow(ctx, generated.UpdateFlowParams{
		ID:              pgUUID(flowID),
		OrgID:           pgUUID(orgID),
		ExpectedVersion: expectedVersion,
		Name:            req.Body.Name,
		Graph:           graphBytes,
		Enabled:         req.Body.Enabled,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 0 rows = id-not-found OR version mismatch. Always re-probe for the true
		// current state: `stored` (from the immutability pre-check) can be stale if
		// a concurrent update landed between that probe and this UPDATE (TOCTOU),
		// which would make the 409 `current` diff lie.
		cur, perr := q.GetFlowByIdAnyVersion(ctx, generated.GetFlowByIdAnyVersionParams{
			ID:    pgUUID(flowID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(perr, pgx.ErrNoRows) {
			return api.UpdateFlow404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
				Error: api.ErrorCodeNotFound, Reason: "flow_not_found",
			}}, nil
		}
		if perr != nil {
			h.deps.Logger.ErrorContext(ctx, "update flow disambiguate", "err", perr)
			return api.UpdateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "disambiguation_failed",
			}}, nil
		}
		// D-56 — DEL on the 409 path so a stale cached value cannot mask the
		// conflict on the client's retry.
		if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "flows", flowID)); delErr != nil {
			h.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", "key", cache.Key(orgID, "flows", flowID), "err", delErr)
		}
		curFlow, mapErr := mapFlow(cur)
		if mapErr != nil {
			h.deps.Logger.ErrorContext(ctx, "map current flow (409)", "err", mapErr)
			return api.UpdateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "map_failed",
			}}, nil
		}
		return api.UpdateFlow409JSONResponse{
			Current: curFlow,
			Error:   api.UpdateFlow409JSONResponseBodyErrorVersionConflict,
			Reason:  "version_mismatch",
		}, nil
	}
	if err != nil {
		status, code, reason := MapPgError(err, "flow")
		if status == 422 {
			return api.UpdateFlow422JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
		}
		h.deps.Logger.ErrorContext(ctx, "update flow", "err", err)
		return api.UpdateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "flows", flowID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "flows", flowID), "err", delErr)
	}

	flow, mapErr := mapFlow(row)
	if mapErr != nil {
		h.deps.Logger.ErrorContext(ctx, "map updated flow", "err", mapErr)
		return api.UpdateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "map_failed",
		}}, nil
	}
	return api.UpdateFlow200JSONResponse(flow), nil
}

// DeleteFlow — DELETE /v1/orgs/{org_id}/flows/{id} (soft delete).
func (h *Handlers) DeleteFlow(ctx context.Context, req api.DeleteFlowRequestObject) (api.DeleteFlowResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.DeleteFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	flowID := uuid.UUID(req.Id)

	q := generated.New(h.deps.OrgDB)
	tag, err := q.SoftDeleteFlow(ctx, generated.SoftDeleteFlowParams{
		ID:    pgUUID(flowID),
		OrgID: pgUUID(orgID),
	})
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "soft delete flow", "err", err)
		return api.DeleteFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	if tag == 0 {
		return api.DeleteFlow404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "flow_not_found",
		}}, nil
	}
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "flows", flowID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "flows", flowID), "err", delErr)
	}
	return api.DeleteFlow204Response{}, nil
}

// newFlowGraph resolves the create-time graph: an omitted graph defaults to an
// empty object (the column is NOT NULL), a present graph is marshalled to JSONB.
func newFlowGraph(g *map[string]interface{}) ([]byte, bool) {
	if g == nil {
		return []byte("{}"), true
	}
	b, err := mapToJSONB(*g)
	if err != nil {
		return nil, false
	}
	return b, true
}

// mapFlow converts a sqlc-row Flow into the api.Flow DTO. graph is JSONB
// ([]byte) decoded via jsonbToMap; a decode failure is a 500 (we only ever
// write valid JSON, so a failure means storage corruption, not bad input).
func mapFlow(row generated.Flow) (api.Flow, error) {
	graph, err := jsonbToMap(row.Graph)
	if err != nil {
		return api.Flow{}, fmt.Errorf("jsonbToMap(row.Graph): %w", err)
	}
	return api.Flow{
		Id:        api.UUIDv7(apiUUID(row.ID)),
		OrgId:     api.UUIDv7(apiUUID(row.OrgID)),
		Code:      row.Code,
		Name:      row.Name,
		Graph:     graph,
		Enabled:   row.Enabled,
		Version:   int(row.Version),
		CreatedAt: ptrTime(row.CreatedAt),
		UpdatedAt: ptrTime(row.UpdatedAt),
	}, nil
}
