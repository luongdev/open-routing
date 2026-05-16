// skills.go — CAT-02 + CAT-08 + CAT-09 + CAT-10 + CAT-11. Wave 4 entity
// modeled after agents.go (Plan 03-06 template). Skills has NO join
// table, NO FK fields, NO Layer 2 input validation — only a nullable
// `description` and the standard CRUD+cache+version-lock shape.
//
// Locked patterns inherited from agents.go:
//
//   - D-66 — version-checked UPDATE 0-row → GetSkillByIdAnyVersion
//     inside the same tx to disambiguate 404 (no row) vs 409 (version
//     mismatch). 409 path also cache.Dels (D-56) so a stale cached
//     value can never mask a conflict on retry.
//   - D-55 — every mutation calls cache.Del AFTER tx.Commit.
//   - CAT-11 — GetSkill goes through cache.GetOrSet[api.Skill] with
//     `or:{orgID}:skills:{id}` and 60s TTL. ErrNotFound propagates
//     without caching the empty value (D-54).
//   - CAT-10 — cursor + LIMIT N+1 sentinel; ?include_disabled=true uses
//     ListSkillsIncludingDisabled (D-65).
//   - CreateSkill409 is FLAT ErrorResponse (Pitfall 2 — only CreateAgent409
//     is the oneOf union).
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

// CreateSkill — POST /v1/orgs/{org_id}/skills (CAT-02).
func (h *Handlers) CreateSkill(ctx context.Context, req api.CreateSkillRequestObject) (api.CreateSkillResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.CreateSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.CreateSkill400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}

	id := uuid.Must(uuid.NewV7())
	enabled := derefOr(req.Body.Enabled, true)

	q := generated.New(h.deps.OrgDB)
	row, err := q.InsertSkill(ctx, generated.InsertSkillParams{
		ID:          pgUUID(id),
		OrgID:       pgUUID(orgID),
		ExternalID:  req.Body.ExternalId,
		Name:        req.Body.Name,
		Description: req.Body.Description,
		SkillType:   req.Body.SkillType,
		Enabled:     enabled,
	})
	if err != nil {
		status, code, reason := mapPgError(err, "skill")
		switch status {
		case 409:
			// CreateSkill409 is a FLAT ErrorResponse (Pitfall 2).
			return api.CreateSkill409JSONResponse(api.ErrorResponse{
				Error: code, Reason: "external_id_collision",
			}), nil
		}
		h.deps.Logger.ErrorContext(ctx, "create skill insert", "err", err)
		return api.CreateSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	// D-55 — DEL after a successful create. Cache is cold here, but the
	// uniform call site prevents drift between create/update/delete paths.
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "skills", id)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "skills", id), "err", delErr)
	}

	return api.CreateSkill201JSONResponse(mapSkill(row)), nil
}

// GetSkill — GET /v1/orgs/{org_id}/skills/{id} (CAT-02, CAT-11).
func (h *Handlers) GetSkill(ctx context.Context, req api.GetSkillRequestObject) (api.GetSkillResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	skillID := uuid.UUID(req.Id)
	key := cache.Key(orgID, "skills", skillID)

	skill, err := cache.GetOrSet[api.Skill](ctx, h.deps.Cache, key, cacheTTL,
		func(ctx context.Context) (api.Skill, error) {
			q := generated.New(h.deps.OrgDB)
			row, ferr := q.GetSkill(ctx, generated.GetSkillParams{
				ID:    pgUUID(skillID),
				OrgID: pgUUID(orgID),
			})
			if errors.Is(ferr, pgx.ErrNoRows) {
				return api.Skill{}, cache.ErrNotFound
			}
			if ferr != nil {
				return api.Skill{}, ferr
			}
			// Soft-deleted rows surface as 404 on detail (CAT-09 contract —
			// DELETE → 204; GET → 404). ?include_disabled is a LIST-only knob.
			if !row.Enabled {
				return api.Skill{}, cache.ErrNotFound
			}
			return mapSkill(row), nil
		})

	switch {
	case errors.Is(err, cache.ErrNotFound):
		return api.GetSkill404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "skill_not_found",
		}}, nil
	case err != nil:
		h.deps.Logger.ErrorContext(ctx, "get skill", "err", err)
		return api.GetSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	return api.GetSkill200JSONResponse(skill), nil
}

// ListSkills — GET /v1/orgs/{org_id}/skills (CAT-09, CAT-10).
func (h *Handlers) ListSkills(ctx context.Context, req api.ListSkillsRequestObject) (api.ListSkillsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListSkills500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}

	pageSize := defaultPageSize
	if req.Params.Limit != nil {
		pageSize = int(*req.Params.Limit)
		if pageSize < 1 || pageSize > maxPageSize {
			return api.ListSkills400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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
		return api.ListSkills400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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
	limit := int32(pageSize + 1) // N+1 sentinel

	var rows []generated.Skill
	if includeDisabled {
		rs, e := q.ListSkillsIncludingDisabled(ctx, generated.ListSkillsIncludingDisabledParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list skills (include_disabled)", "err", e)
			return api.ListSkills500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	} else {
		rs, e := q.ListSkills(ctx, generated.ListSkillsParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list skills", "err", e)
			return api.ListSkills500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	}

	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	items := make([]api.Skill, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapSkill(r))
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		enc, encErr := EncodeCursor(last.CreatedAt.Time, apiUUID(last.ID))
		if encErr != nil {
			// has_more=true + missing next_cursor would be an inconsistent
			// wire shape clients can't recover from.
			h.deps.Logger.ErrorContext(ctx, "list skills encode cursor", "err", encErr)
			return api.ListSkills500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "cursor_encode_failed",
			}}, nil
		}
		nextCursor = &enc
	}
	return api.ListSkills200JSONResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateSkill — PATCH /v1/orgs/{org_id}/skills/{id} (CAT-02, CAT-08).
// D-66: 0-row version-checked UPDATE → disambiguate via GetSkillByIdAnyVersion
// in the same tx. 409 path also cache.Dels (D-56).
func (h *Handlers) UpdateSkill(ctx context.Context, req api.UpdateSkillRequestObject) (api.UpdateSkillResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.UpdateSkill400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}
	skillID := uuid.UUID(req.Id)

	// Skills has no join table — the tx wraps only the row UPDATE + the
	// disambiguation probe. Shape stays identical to agents.go for the
	// disambiguation contract (D-66 invariant: probe sees the rollback).
	tx, err := h.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "update skill begin tx", "err", err)
		return api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "begin_tx_failed",
		}}, nil
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := generated.New(tx)
	row, err := qtx.UpdateSkill(ctx, generated.UpdateSkillParams{
		ID:              pgUUID(skillID),
		OrgID:           pgUUID(orgID),
		ExpectedVersion: int32(req.Body.Version),
		Name:            req.Body.Name,
		Description:     req.Body.Description,
		SkillType:       req.Body.SkillType,
		Enabled:         req.Body.Enabled,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		cur, perr := qtx.GetSkillByIdAnyVersion(ctx, generated.GetSkillByIdAnyVersionParams{
			ID:    pgUUID(skillID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(perr, pgx.ErrNoRows) {
			return api.UpdateSkill404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
				Error: api.ErrorCodeNotFound, Reason: "skill_not_found",
			}}, nil
		}
		if perr != nil {
			h.deps.Logger.ErrorContext(ctx, "update skill disambiguate", "err", perr)
			return api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "disambiguation_failed",
			}}, nil
		}
		// 409 — D-56 cache DEL so a stale value can't mask the conflict.
		if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "skills", skillID)); delErr != nil {
			h.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", "key", cache.Key(orgID, "skills", skillID), "err", delErr)
		}
		return api.UpdateSkill409JSONResponse{
			Current: mapSkill(cur),
			Error:   api.VersionConflict,
			Reason:  "version_mismatch",
		}, nil
	}
	if err != nil {
		status, code, reason := mapPgError(err, "skill")
		if status == 422 {
			return api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: code, Reason: reason,
			}}, nil
		}
		h.deps.Logger.ErrorContext(ctx, "update skill", "err", err)
		return api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	if err := tx.Commit(ctx); err != nil {
		h.deps.Logger.ErrorContext(ctx, "update skill commit", "err", err)
		return api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "commit_failed",
		}}, nil
	}

	// D-55 — DEL after commit (best-effort).
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "skills", skillID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "skills", skillID), "err", delErr)
	}

	return api.UpdateSkill200JSONResponse(mapSkill(row)), nil
}

// DeleteSkill — DELETE /v1/orgs/{org_id}/skills/{id} (CAT-02, CAT-09).
func (h *Handlers) DeleteSkill(ctx context.Context, req api.DeleteSkillRequestObject) (api.DeleteSkillResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.DeleteSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	skillID := uuid.UUID(req.Id)

	q := generated.New(h.deps.OrgDB)
	tag, err := q.SoftDeleteSkill(ctx, generated.SoftDeleteSkillParams{
		ID:    pgUUID(skillID),
		OrgID: pgUUID(orgID),
	})
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "soft delete skill", "err", err)
		return api.DeleteSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	if tag == 0 {
		// Row missing OR already disabled — both → 404 (D-65).
		return api.DeleteSkill404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "skill_not_found",
		}}, nil
	}
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "skills", skillID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "skills", skillID), "err", delErr)
	}
	return api.DeleteSkill204Response{}, nil
}

// mapSkill — sqlc-row → api.Skill DTO. Reads the nullable `description`
// column via textPtr (Wave 3 helper).
func mapSkill(row generated.Skill) api.Skill {
	var desc *string
	if row.Description != nil {
		d := *row.Description
		desc = &d
	}
	return api.Skill{
		Id:          api.UUIDv7(apiUUID(row.ID)),
		OrgId:       api.UUIDv7(apiUUID(row.OrgID)),
		ExternalId:  row.ExternalID,
		Name:        row.Name,
		Description: desc,
		SkillType:   row.SkillType,
		Enabled:     row.Enabled,
		Version:     int(row.Version),
		CreatedAt:   ptrTime(row.CreatedAt),
		UpdatedAt:   ptrTime(row.UpdatedAt),
	}
}
