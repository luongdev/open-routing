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

// CreateAdapter — POST /v1/orgs/{org_id}/adapters (CAT-06).
func (h *Handlers) CreateAdapter(ctx context.Context, req api.CreateAdapterRequestObject) (api.CreateAdapterResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.CreateAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.CreateAdapter400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}

	// Pitfall 9 — sqlc emits []byte for JSONB; round-trip via stdlib json.
	// Empty/nil map serialises to "{}" so Postgres stores a valid JSONB
	// object and GET returns {} (NOT null) — matches the spec
	// additionalProperties:true semantics.
	var cfg map[string]any
	if req.Body.Config != nil {
		cfg = *req.Body.Config
	} else {
		cfg = map[string]any{}
	}
	cfgBytes, err := mapToJSONB(cfg)
	if err != nil {
		return api.CreateAdapter400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "config_marshal_failed",
		}}, nil
	}
	if cfgBytes == nil {
		cfgBytes = []byte("{}")
	}

	id := uuid.Must(uuid.NewV7())
	enabled := derefOr(req.Body.Enabled, true)

	q := generated.New(h.deps.OrgDB)
	row, err := q.InsertAdapter(ctx, generated.InsertAdapterParams{
		ID:          pgUUID(id),
		OrgID:       pgUUID(orgID),
		Name:        req.Body.Name,
		AdapterType: req.Body.AdapterType,
		Config:      cfgBytes,
		Enabled:     enabled,
	})
	if err != nil {
		status, code, reason := mapPgError(err, "adapter")
		if status == 422 {
			return api.CreateAdapter400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
				Error: code, Reason: reason,
			}}, nil
		}
		h.deps.Logger.ErrorContext(ctx, "create adapter insert", "err", err)
		return api.CreateAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	// D-55 — DEL after the write so a stale cached value can never mask
	// the freshly minted row. Idempotent against an empty cache.
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "adapters", id)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "adapters", id), "err", delErr)
	}

	out, mErr := mapAdapter(row)
	if mErr != nil {
		h.deps.Logger.ErrorContext(ctx, "create adapter map", "err", mErr)
		return api.CreateAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "config_unmarshal_failed",
		}}, nil
	}
	return api.CreateAdapter201JSONResponse(out), nil
}

// GetAdapter — GET /v1/orgs/{org_id}/adapters/{id} (CAT-06, CAT-11).
// Loader maps pgx.ErrNoRows → cache.ErrNotFound so the cache never writes
// an empty entry (D-54). Soft-deleted rows (enabled=false) surface as 404
// from the detail endpoint per CAT-09.
func (h *Handlers) GetAdapter(ctx context.Context, req api.GetAdapterRequestObject) (api.GetAdapterResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	adapterID := uuid.UUID(req.Id)
	key := cache.Key(orgID, "adapters", adapterID)

	adapter, err := cache.GetOrSet[api.Adapter](ctx, h.deps.Cache, key, cacheTTL,
		func(ctx context.Context) (api.Adapter, error) {
			q := generated.New(h.deps.OrgDB)
			row, ferr := q.GetAdapter(ctx, generated.GetAdapterParams{
				ID:    pgUUID(adapterID),
				OrgID: pgUUID(orgID),
			})
			if errors.Is(ferr, pgx.ErrNoRows) {
				return api.Adapter{}, cache.ErrNotFound
			}
			if ferr != nil {
				return api.Adapter{}, ferr
			}
			// Rule 2 — soft-delete invisibility from detail endpoints (CAT-09
			// contract: DELETE → 204; subsequent GET → 404). ?include_disabled
			// is a LIST-only knob.
			if !row.Enabled {
				return api.Adapter{}, cache.ErrNotFound
			}
			return mapAdapter(row)
		})

	switch {
	case errors.Is(err, cache.ErrNotFound):
		return api.GetAdapter404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "adapter_not_found",
		}}, nil
	case err != nil:
		h.deps.Logger.ErrorContext(ctx, "get adapter", "err", err)
		return api.GetAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	return api.GetAdapter200JSONResponse(adapter), nil
}

// ListAdapters — GET /v1/orgs/{org_id}/adapters (CAT-06, CAT-09, CAT-10).
func (h *Handlers) ListAdapters(ctx context.Context, req api.ListAdaptersRequestObject) (api.ListAdaptersResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListAdapters500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}

	pageSize := defaultPageSize
	if req.Params.Limit != nil {
		pageSize = int(*req.Params.Limit)
		if pageSize < 1 || pageSize > maxPageSize {
			return api.ListAdapters400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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
		return api.ListAdapters400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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
	limit := mustInt32(pageSize + 1) // N+1 sentinel for has_more.

	var rows []generated.Adapter
	if includeDisabled {
		rs, e := q.ListAdaptersIncludingDisabled(ctx, generated.ListAdaptersIncludingDisabledParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list adapters (include_disabled)", "err", e)
			return api.ListAdapters500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	} else {
		rs, e := q.ListAdapters(ctx, generated.ListAdaptersParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list adapters", "err", e)
			return api.ListAdapters500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	}

	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	items := make([]api.Adapter, 0, len(rows))
	for _, r := range rows {
		dto, mErr := mapAdapter(r)
		if mErr != nil {
			h.deps.Logger.ErrorContext(ctx, "list adapters map", "err", mErr, "adapter_id", uuid.UUID(r.ID.Bytes))
			return api.ListAdapters500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "config_unmarshal_failed",
			}}, nil
		}
		items = append(items, dto)
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		enc, encErr := EncodeCursor(last.CreatedAt.Time, apiUUID(last.ID))
		if encErr != nil {
			h.deps.Logger.ErrorContext(ctx, "list adapters encode cursor", "err", encErr)
			return api.ListAdapters500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "cursor_encode_failed",
			}}, nil
		}
		nextCursor = &enc
	}
	return api.ListAdapters200JSONResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateAdapter — PATCH /v1/orgs/{org_id}/adapters/{id} (CAT-06, CAT-08).
// D-66: 0 rows from version-checked UPDATE = 404 (no row) or 409 (version
// mismatch). Disambiguation probe runs via GetAdapterByIdAnyVersion.
func (h *Handlers) UpdateAdapter(ctx context.Context, req api.UpdateAdapterRequestObject) (api.UpdateAdapterResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.UpdateAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.UpdateAdapter400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}
	adapterID := uuid.UUID(req.Id)

	// Pitfall 9 — JSONB sparse PATCH semantics. The sqlc UPDATE uses
	// COALESCE($3::jsonb, config), so passing NULL (nil bytes) preserves
	// the existing column. Empty map → "{}" bytes (replaces with empty
	// object, NOT NULL). Non-nil map → marshalled JSON.
	var cfgBytes []byte
	if req.Body.Config != nil {
		b, mErr := mapToJSONB(*req.Body.Config)
		if mErr != nil {
			return api.UpdateAdapter400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
				Error: api.ErrorCodeInvalidBody, Reason: "config_marshal_failed",
			}}, nil
		}
		if b == nil {
			b = []byte("{}")
		}
		cfgBytes = b
	}

	q := generated.New(h.deps.OrgDB)
	expectedVersion, ok := int32Checked(req.Body.Version)
	if !ok {
		return api.UpdateAdapter400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "version_out_of_range",
		}}, nil
	}
	row, err := q.UpdateAdapter(ctx, generated.UpdateAdapterParams{
		ID:              pgUUID(adapterID),
		OrgID:           pgUUID(orgID),
		ExpectedVersion: expectedVersion,
		Name:            req.Body.Name,
		AdapterType:     req.Body.AdapterType,
		Config:          cfgBytes,
		Enabled:         req.Body.Enabled,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		cur, perr := q.GetAdapterByIdAnyVersion(ctx, generated.GetAdapterByIdAnyVersionParams{
			ID:    pgUUID(adapterID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(perr, pgx.ErrNoRows) {
			return api.UpdateAdapter404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
				Error: api.ErrorCodeNotFound, Reason: "adapter_not_found",
			}}, nil
		}
		if perr != nil {
			h.deps.Logger.ErrorContext(ctx, "update adapter disambiguate", "err", perr)
			return api.UpdateAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "disambiguation_failed",
			}}, nil
		}
		// D-56 — flush cache on 409 so a stale read can never mask the
		// conflict on the client's retry path.
		if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "adapters", adapterID)); delErr != nil {
			h.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", "key", cache.Key(orgID, "adapters", adapterID), "err", delErr)
		}
		curDTO, mErr := mapAdapter(cur)
		if mErr != nil {
			h.deps.Logger.ErrorContext(ctx, "update adapter 409 map", "err", mErr)
			return api.UpdateAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "config_unmarshal_failed",
			}}, nil
		}
		return api.UpdateAdapter409JSONResponse{
			Current: curDTO,
			Error:   api.UpdateAdapter409JSONResponseBodyErrorVersionConflict,
			Reason:  "version_mismatch",
		}, nil
	}
	if err != nil {
		status, code, reason := mapPgError(err, "adapter")
		if status == 422 {
			return api.UpdateAdapter400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
				Error: code, Reason: reason,
			}}, nil
		}
		h.deps.Logger.ErrorContext(ctx, "update adapter", "err", err)
		return api.UpdateAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	// D-55 — DEL after the write so the next GET sees fresh state.
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "adapters", adapterID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "adapters", adapterID), "err", delErr)
	}

	out, mErr := mapAdapter(row)
	if mErr != nil {
		h.deps.Logger.ErrorContext(ctx, "update adapter map", "err", mErr)
		return api.UpdateAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "config_unmarshal_failed",
		}}, nil
	}
	return api.UpdateAdapter200JSONResponse(out), nil
}

// DeleteAdapter — DELETE /v1/orgs/{org_id}/adapters/{id} (CAT-06, CAT-09).
func (h *Handlers) DeleteAdapter(ctx context.Context, req api.DeleteAdapterRequestObject) (api.DeleteAdapterResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.DeleteAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	adapterID := uuid.UUID(req.Id)

	q := generated.New(h.deps.OrgDB)
	tag, err := q.SoftDeleteAdapter(ctx, generated.SoftDeleteAdapterParams{
		ID:    pgUUID(adapterID),
		OrgID: pgUUID(orgID),
	})
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "soft delete adapter", "err", err)
		return api.DeleteAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	if tag == 0 {
		return api.DeleteAdapter404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "adapter_not_found",
		}}, nil
	}
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "adapters", adapterID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "adapters", adapterID), "err", delErr)
	}
	return api.DeleteAdapter204Response{}, nil
}

// mapAdapter converts a sqlc-row Adapter into the api.Adapter DTO.
// Pitfall 9 — Config column is []byte (JSONB); decode via jsonbToMap. Nil
// bytes / empty map decode to a fresh empty map so the response always
// contains a JSON object (NOT null) per the additionalProperties:true
// spec semantics. json.Unmarshal performed inside the loader propagates
// non-JSON bytes to the caller as 500 — Postgres guarantees JSONB columns
// are valid JSON, so this branch is unreachable in healthy production.
func mapAdapter(row generated.Adapter) (api.Adapter, error) {
	cfg, err := jsonbToMap(row.Config)
	if err != nil {
		return api.Adapter{}, fmt.Errorf("json.Unmarshal(row.Config): %w", err)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return api.Adapter{
		Id:          api.UUIDv7(apiUUID(row.ID)),
		OrgId:       api.UUIDv7(apiUUID(row.OrgID)),
		Code:        row.Code,
		ExternalId:  row.ExternalID,
		Name:        row.Name,
		AdapterType: row.AdapterType,
		Config:      &cfg,
		Enabled:     row.Enabled,
		Version:     int(row.Version),
		CreatedAt:   ptrTime(row.CreatedAt),
		UpdatedAt:   ptrTime(row.UpdatedAt),
	}, nil
}
