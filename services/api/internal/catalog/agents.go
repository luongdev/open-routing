// agents.go — CAT-01 + CAT-03 + CAT-08 + CAT-09 + CAT-10 + CAT-11
// reference template for every other CRUD entity in the catalog package.
//
// Invariants (WHY, not what):
//
//   - Codex C1/C2 — proficiency 1..10 validated at the handler BEFORE any
//     DB call so out-of-range surfaces as 422 invalid_value. Plan 03-01
//     OMITS minimum/maximum from the OpenAPI proficiency field so
//     oapi-codegen does NOT short-circuit with 400.
//
//   - Codex C4 — the agent row write and skills replace share one tx
//     (CreateAgent + UpdateAgent only). The h.replaceAgentSkills method
//     in agent_skills.go composes inside that tx via the passed-in qtx,
//     so a failed INSERT rolls back the agent row write too. Exactly two
//     transactional call sites in this file are intentional — adding a
//     third would break Pitfall 5 atomicity.
//
//   - D-66 — UPDATE returning 0 rows is ambiguous between 404 and 409.
//     GetAgentByIdAnyVersion runs inside the same tx so the probe sees
//     the rolled-back state, then D-56 flushes the cache on the 409 path
//     to prevent a stale read masking the conflict on the client retry.
//
//   - D-55 — every mutation calls cache.Del AFTER commit; a Del failure
//     logs warn but never downgrades the response (the cache TTLs out).
//
//   - D-76 — cross-row FK probe uses SkillsPresentInOrg (subset present +
//     enabled in caller's org). The Wave 2 inversion preserves the
//     SQLChecker top-level FROM tenant constraint.
//
//   - CAT-11 — GetAgent runs through cache.GetOrSet[api.Agent] with 60s
//     TTL. ErrNotFound from the loader maps to 404 WITHOUT caching the
//     miss (D-54).
//
//   - CAT-10 — cursor pagination uses (created_at, id) + LIMIT N+1
//     sentinel (D-63). Defaults 25 / max 100 (D-67) enforced handler-side
//     because the spec deliberately omits min/max on LimitQuery.
//     ?include_disabled toggles to a separate prepared statement (D-65)
//     so the planner caches both shapes.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

const (
	// cacheTTL — CAT-11 / D-59 per-entity cache lifetime. Refresh-ahead
	// fires inside cache.GetOrSet when PTTL falls below 10s (D-53).
	cacheTTL = 60 * time.Second

	// defaultPageSize — D-67. Returned when ?limit is omitted.
	defaultPageSize = 25

	// maxPageSize — D-67. ?limit values >100 → 400 invalid_body.
	maxPageSize = 100
)

// CreateAgent — POST /v1/orgs/{org_id}/agents (CAT-01, CAT-03).
func (h *Handlers) CreateAgent(ctx context.Context, req api.CreateAgentRequestObject) (api.CreateAgentResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.CreateAgent400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}

	// Phase 04.1 Layer 1 (D04_1-19, RESEARCH §Pitfall 1) — oapi-codegen v2 does
	// NOT enforce OpenAPI `pattern` regexes. The handler is the single source
	// of truth for code-format validation. Fires BEFORE proficiency / any DB
	// call so a malformed code surfaces as 400 invalid_body cleanly.
	if !ValidateCodeFormat(req.Body.Code) {
		return api.CreateAgent400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error:  api.ErrorCodeInvalidBody,
			Reason: "invalid_code_format",
		}}, nil
	}

	// Codex C1 iter 3 — Layer 2 proficiency check BEFORE any DB call.
	if req.Body.Skills != nil {
		if badIdx, valid := validateProficiencyRange(*req.Body.Skills); !valid {
			return api.CreateAgent422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidValue,
				Reason: fmt.Sprintf("skills[%d].proficiency must be 1-10", badIdx),
			}), nil
		}
		if dupIdx, ok := validateNoDuplicateSkills(*req.Body.Skills); !ok {
			return api.CreateAgent422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidValue,
				Reason: fmt.Sprintf("skills[%d].skill_id is a duplicate", dupIdx),
			}), nil
		}
	}

	id := uuid.Must(uuid.NewV7())
	enabled := derefOr(req.Body.Enabled, true)

	// Codex C4 iter 3 — single tx for agent INSERT + optional skills replace.
	tx, err := h.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "create agent begin tx", "err", err)
		return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "begin_tx_failed",
		}}, nil
	}
	defer func() { _ = tx.Rollback(ctx) }() // safe after commit — pgx ignores.

	qtx := generated.New(tx)
	row, err := qtx.InsertAgent(ctx, generated.InsertAgentParams{
		ID:         pgUUID(id),
		OrgID:      pgUUID(orgID),
		Code:       req.Body.Code,
		ExternalID: req.Body.ExternalId, // *string post-04.1 (column nullable).
		Name:       req.Body.Name,
		Email:      string(req.Body.Email),
		Enabled:    enabled,
	})
	if err != nil {
		status, code, reason := MapPgError(err, "agent")
		switch status {
		case 409:
			// Pitfall 2 — CreateAgent409 is a oneOf union. Build via the
			// FromErrorResponse helper, NOT a struct literal.
			var body api.CreateAgent409JSONResponseBody
			_ = body.FromErrorResponse(api.ErrorResponse{Error: code, Reason: reason})
			return api.CreateAgent409JSONResponse(body), nil
		case 422:
			// 23503 FK violation (rare for agents — table has no FK) or
			// 23514 CHECK → 422 invalid_reference / invalid_value.
			return api.CreateAgent422JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
		}
		h.deps.Logger.ErrorContext(ctx, "create agent insert", "err", err)
		return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	// D-93: seed agent_states row inside the SAME tx so agent + state
	// commit/rollback atomically (Codex C4 invariant). InsertAgentState
	// MUST go through qtx (not a fresh generated.New on the pool) or the
	// state row commits independently of the agent row and breaks the
	// invariant on failure paths (Pitfall 10).
	//
	// Phase 5 reminder: bulk-import (CSV/JSON upsert into agents) MUST also
	// seed agent_states for every NEW agent it inserts. Use
	// `ON CONFLICT (agent_id) DO NOTHING` so re-imports don't regress the
	// state machine (Pitfall 6 / Hazard 7 in 04-PATTERNS.md).
	if _, sErr := qtx.InsertAgentState(ctx, generated.InsertAgentStateParams{
		AgentID: pgUUID(id),
		OrgID:   pgUUID(orgID),
		Status:  string(api.AgentStatusOffline),
	}); sErr != nil {
		h.deps.Logger.ErrorContext(ctx, "create agent state row", "agent_id", id, "org_id", orgID, "err", sErr)
		return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error:  api.ErrorCodeInternal,
			Reason: "agent_state_insert_failed",
		}}, nil
	}

	// Skills replace inside the SAME tx (Codex C4 — atomicity). Failure
	// here rolls back BOTH the agent insert and the skills writes.
	if req.Body.Skills != nil && len(*req.Body.Skills) > 0 {
		if errResp := h.replaceAgentSkills(ctx, qtx, orgID, id, *req.Body.Skills); errResp != nil {
			if errResp.Error == api.ErrorCodeInternal {
				// 500 — caller's defer Rollback handles cleanup.
				h.deps.Logger.ErrorContext(ctx, "create agent skills replace", "reason", errResp.Reason)
				return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
					Error: errResp.Error, Reason: errResp.Reason,
				}}, nil
			}
			// 422 invalid_reference / invalid_value — tx auto-rolls back.
			return api.CreateAgent422JSONResponse(*errResp), nil
		}
	}

	if err := tx.Commit(ctx); err != nil {
		h.deps.Logger.ErrorContext(ctx, "create agent commit", "err", err)
		return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "commit_failed",
		}}, nil
	}

	// D-55 — cache DEL after commit. The cache is empty at this point
	// (a fresh row was never cached), but DEL is idempotent and keeping
	// the call site uniform across create/update/delete prevents drift.
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "agents", id)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "agents", id), "err", delErr)
	}

	return api.CreateAgent201JSONResponse(mapAgent(row, nil)), nil
}

// GetAgent — GET /v1/orgs/{org_id}/agents/{id} (CAT-01, CAT-11).
// Wraps the loader in cache.GetOrSet[api.Agent] (60s TTL). Loader maps
// pgx.ErrNoRows → cache.ErrNotFound so the cache never writes an empty
// entry (D-54).
func (h *Handlers) GetAgent(ctx context.Context, req api.GetAgentRequestObject) (api.GetAgentResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	agentID := uuid.UUID(req.Id)
	key := cache.Key(orgID, "agents", agentID)

	agent, err := cache.GetOrSet[api.Agent](ctx, h.deps.Cache, key, cacheTTL,
		func(ctx context.Context) (api.Agent, error) {
			q := generated.New(h.deps.OrgDB)
			row, ferr := q.GetAgent(ctx, generated.GetAgentParams{
				ID:    pgUUID(agentID),
				OrgID: pgUUID(orgID),
			})
			if errors.Is(ferr, pgx.ErrNoRows) {
				return api.Agent{}, cache.ErrNotFound
			}
			if ferr != nil {
				return api.Agent{}, ferr
			}
			// Soft-deleted rows surface as 404 from the detail endpoint
			// (CAT-09 contract: DELETE → 204; GET → 404). The SQL query
			// returns the row regardless of `enabled` so the handler
			// applies the filter — matches the list-default behaviour
			// (D-65). ?include_disabled is a LIST-only knob; detail GETs
			// never reveal a soft-deleted row (Rule 2 — soft-delete
			// invisibility is a correctness requirement, not a feature).
			if !row.Enabled {
				return api.Agent{}, cache.ErrNotFound
			}
			skills, sErr := q.ListSkillsForAgent(ctx, generated.ListSkillsForAgentParams{
				AgentID: pgUUID(agentID),
				OrgID:   pgUUID(orgID),
			})
			if sErr != nil {
				return api.Agent{}, sErr
			}
			return mapAgent(row, skills), nil
		})

	switch {
	case errors.Is(err, cache.ErrNotFound):
		return api.GetAgent404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "agent_not_found",
		}}, nil
	case err != nil:
		h.deps.Logger.ErrorContext(ctx, "get agent", "err", err)
		return api.GetAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	return api.GetAgent200JSONResponse(agent), nil
}

// ListAgents — GET /v1/orgs/{org_id}/agents (CAT-09, CAT-10).
// Cursor (created_at, id) + LIMIT N+1 sentinel for has_more (D-63).
// ?include_disabled=true → ListAgentsIncludingDisabled prepared statement
// (D-65 — two queries, not one with conditional WHERE).
// Default limit 25 / max 100 (D-67) validated handler-side because the
// spec intentionally lets the handler own the 400 wire shape.
func (h *Handlers) ListAgents(ctx context.Context, req api.ListAgentsRequestObject) (api.ListAgentsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListAgents500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}

	pageSize := defaultPageSize
	if req.Params.Limit != nil {
		pageSize = int(*req.Params.Limit)
		if pageSize < 1 || pageSize > maxPageSize {
			return api.ListAgents400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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
		return api.ListAgents400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
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

	var rows []generated.Agent
	if includeDisabled {
		rs, e := q.ListAgentsIncludingDisabled(ctx, generated.ListAgentsIncludingDisabledParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list agents (include_disabled)", "err", e)
			return api.ListAgents500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	} else {
		rs, e := q.ListAgents(ctx, generated.ListAgentsParams{
			OrgID:      pgUUID(orgID),
			Limit:      limit,
			CursorAt:   cursorTime(cur),
			CursorID:   cursorID(cur),
			NameFilter: nameFilter,
		})
		if e != nil {
			h.deps.Logger.ErrorContext(ctx, "list agents", "err", e)
			return api.ListAgents500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "list_failed",
			}}, nil
		}
		rows = rs
	}

	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	// OQ-1A — list items are FLAT, no embedded skills.
	items := make([]api.AgentListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapAgentListItem(r))
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		enc, encErr := EncodeCursor(last.CreatedAt.Time, apiUUID(last.ID))
		if encErr != nil {
			// Surface as 500: hasMore=true + missing next_cursor would
			// be an inconsistent wire shape clients can't recover from.
			h.deps.Logger.ErrorContext(ctx, "list agents encode cursor", "err", encErr)
			return api.ListAgents500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "cursor_encode_failed",
			}}, nil
		}
		nextCursor = &enc
	}
	return api.ListAgents200JSONResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateAgent — PATCH /v1/orgs/{org_id}/agents/{id} (CAT-01, CAT-03, CAT-08).
// Codex C4: the version-checked UPDATE and optional skills replace share
// one tx — a failed skills replace rolls the agent row write back too.
// D-66: 0 rows from the version-checked UPDATE is ambiguous between 404
// (no row) and 409 (version mismatch). GetAgentByIdAnyVersion runs in the
// same tx so the probe sees the rolled-back state; the 409 path flushes
// the cache (D-56) so a stale read can't mask the conflict on retry.
func (h *Handlers) UpdateAgent(ctx context.Context, req api.UpdateAgentRequestObject) (api.UpdateAgentResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.UpdateAgent400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}
	agentID := uuid.UUID(req.Id)

	// Phase 04.1 Layer 1 (D04_1-19) — fail fast on malformed code BEFORE the
	// transaction begins so a malformed PATCH gets 400, not 422. Layer 2
	// (immutability) fires inside the tx after fetching the stored row.
	if req.Body.Code != nil {
		if !ValidateCodeFormat(*req.Body.Code) {
			return api.UpdateAgent400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
				Error:  api.ErrorCodeInvalidBody,
				Reason: "invalid_code_format",
			}}, nil
		}
	}

	// Codex C1 iter 3 — Layer 2 proficiency check BEFORE any DB call.
	if req.Body.Skills != nil {
		if badIdx, valid := validateProficiencyRange(*req.Body.Skills); !valid {
			return api.UpdateAgent422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidValue,
				Reason: fmt.Sprintf("skills[%d].proficiency must be 1-10", badIdx),
			}), nil
		}
		if dupIdx, ok := validateNoDuplicateSkills(*req.Body.Skills); !ok {
			return api.UpdateAgent422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidValue,
				Reason: fmt.Sprintf("skills[%d].skill_id is a duplicate", dupIdx),
			}), nil
		}
	}

	// Codex C4 iter 3 — single tx for agent UPDATE + skills replace.
	tx, err := h.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "update agent begin tx", "err", err)
		return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "begin_tx_failed",
		}}, nil
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := generated.New(tx)

	// Phase 04.1 Layer 2 (D04_1-20, RESEARCH §Common Operation 5) — immutability
	// gate. When req.Body.Code != nil we fetch the stored row pre-UPDATE so we
	// (a) hold a tx-local view while we compare it to the requested code and
	// (b) reuse that same row in the post-UPDATE 0-row branch instead of a
	// second GetAgentByIdAnyVersion probe. `stored` is nil when the PATCH
	// body omitted `code` (Branch 2) — in that case the existing Phase 3
	// post-UPDATE flow handles 404/409 disambiguation unchanged.
	var stored *generated.Agent
	if req.Body.Code != nil {
		s, perr := qtx.GetAgentByIdAnyVersion(ctx, generated.GetAgentByIdAnyVersionParams{
			ID:    pgUUID(agentID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(perr, pgx.ErrNoRows) {
			return api.UpdateAgent404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
				Error: api.ErrorCodeNotFound, Reason: "agent_not_found",
			}}, nil
		}
		if perr != nil {
			h.deps.Logger.ErrorContext(ctx, "load stored agent for immutability check", "err", perr)
			return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "load_stored_failed",
			}}, nil
		}
		stored = &s
		if !validateImmutableCode(stored.Code, *req.Body.Code) {
			return api.UpdateAgent422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeImmutableField,
				Reason: "code",
			}), nil
		}
	}

	// Codex C3 — pass nil for omitted fields; COALESCE preserves the
	// existing column. Email is *openapi_types.Email aliased to *string.
	var emailStr *string
	if req.Body.Email != nil {
		s := string(*req.Body.Email)
		emailStr = &s
	}

	// Atomic version-checked UPDATE (D-66).
	expectedVersion, ok := int32Checked(req.Body.Version)
	if !ok {
		return api.UpdateAgent400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "version_out_of_range",
		}}, nil
	}
	row, err := qtx.UpdateAgent(ctx, generated.UpdateAgentParams{
		ID:              pgUUID(agentID),
		OrgID:           pgUUID(orgID),
		ExpectedVersion: expectedVersion,
		// Phase 5 fix H2: pass external_id through. The SQL CASE expression
		// treats: nil → preserve (omitted), empty string → NULL (clear),
		// non-empty → new value. Pre-fix the handler omitted ExternalID
		// entirely so the SQL COALESCE always preserved the existing value
		// and clients literally could not PATCH external_id.
		ExternalID:      req.Body.ExternalId,
		Name:            req.Body.Name,
		Email:           emailStr,
		Enabled:         req.Body.Enabled,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 0 rows — disambiguate 404 vs 409 inside the same tx.
		//
		// Phase 04.1 (D04_1-20): when `stored != nil` we already proved the
		// row exists via the pre-UPDATE Layer 2 fetch — 0 rows here means
		// version mismatch. Skip the redundant GetAgentByIdAnyVersion probe.
		// Otherwise (Branch 2, Code absent from body), run the existing
		// post-UPDATE probe so the 404/409 distinction is unchanged from
		// Phase 3.
		var cur generated.Agent
		if stored != nil {
			cur = *stored
		} else {
			s2, perr := qtx.GetAgentByIdAnyVersion(ctx, generated.GetAgentByIdAnyVersionParams{
				ID:    pgUUID(agentID),
				OrgID: pgUUID(orgID),
			})
			if errors.Is(perr, pgx.ErrNoRows) {
				return api.UpdateAgent404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
					Error: api.ErrorCodeNotFound, Reason: "agent_not_found",
				}}, nil
			}
			if perr != nil {
				h.deps.Logger.ErrorContext(ctx, "update agent disambiguate", "err", perr)
				return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
					Error: api.ErrorCodeInternal, Reason: "disambiguation_failed",
				}}, nil
			}
			cur = s2
		}
		// 409 — defer Rollback runs (no UPDATE landed). D-56 cache DEL
		// so a stale cached value can never mask the conflict.
		if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "agents", agentID)); delErr != nil {
			h.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", "key", cache.Key(orgID, "agents", agentID), "err", delErr)
		}
		// Phase 5 fix H1 — UpdateAgent409 is now a oneOf union to share the
		// HTTP status code between version_conflict (CAT-08) and
		// duplicate_external_id (PATCH-time UNIQUE collision). The version-
		// conflict branch builds via FromUpdateAgent409JSONResponseBody1
		// (the allOf VersionConflictErrorResponse + Current member).
		var body api.UpdateAgent409JSONResponseBody
		_ = body.FromUpdateAgent409JSONResponseBody1(api.UpdateAgent409JSONResponseBody1{
			Current: mapAgent(cur, nil),
			Error:   api.UpdateAgent409JSONResponseBody1ErrorVersionConflict,
			Reason:  "version_mismatch",
		})
		return api.UpdateAgent409JSONResponse(body), nil
	}
	if err != nil {
		// Phase 5 fix H1 — PATCH-time 23505 (duplicate_external_id) MUST
		// surface as 409, not 500. MapPgError returns status=409 for
		// duplicate_external_id (per Phase 04.1 D04_1-21 constraint-name
		// introspect); pre-fix the handler only branched on 422 and let 409
		// fall through to 500. The new branch builds the ErrorResponse
		// member of the oneOf union.
		status, code, reason := MapPgError(err, "agent")
		switch status {
		case 422:
			return api.UpdateAgent422JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
		case 409:
			var body api.UpdateAgent409JSONResponseBody
			_ = body.FromErrorResponse(api.ErrorResponse{Error: code, Reason: reason})
			return api.UpdateAgent409JSONResponse(body), nil
		}
		h.deps.Logger.ErrorContext(ctx, "update agent", "err", err)
		return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	// Happy path UPDATE returned a row — skills replace inside the SAME
	// tx (Codex C4). Failure rolls back the agent UPDATE too.
	if req.Body.Skills != nil {
		if errResp := h.replaceAgentSkills(ctx, qtx, orgID, agentID, *req.Body.Skills); errResp != nil {
			if errResp.Error == api.ErrorCodeInternal {
				h.deps.Logger.ErrorContext(ctx, "update agent skills replace", "reason", errResp.Reason)
				return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
					Error: errResp.Error, Reason: errResp.Reason,
				}}, nil
			}
			return api.UpdateAgent422JSONResponse(*errResp), nil
		}
	}

	if err := tx.Commit(ctx); err != nil {
		h.deps.Logger.ErrorContext(ctx, "update agent commit", "err", err)
		return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "commit_failed",
		}}, nil
	}

	// D-55 — cache DEL after commit (best-effort).
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "agents", agentID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "agents", agentID), "err", delErr)
	}

	freshQ := generated.New(h.deps.OrgDB)
	skills, listErr := freshQ.ListSkillsForAgent(ctx, generated.ListSkillsForAgentParams{
		AgentID: pgUUID(agentID),
		OrgID:   pgUUID(orgID),
	})
	if listErr != nil {
		// Tx is already committed — the agent + its skills are in their
		// new state. Returning 500 here would mislead the client into
		// thinking the write failed. Log + return the agent row with
		// the skills slice we have (possibly empty); the next GET will
		// fetch the truth.
		h.deps.Logger.WarnContext(ctx, "update agent: reload skills failed (write committed)",
			"agent_id", agentID, "err", listErr)
	}
	return api.UpdateAgent200JSONResponse(mapAgent(row, skills)), nil
}

// DeleteAgent — DELETE /v1/orgs/{org_id}/agents/{id} (CAT-01, CAT-09).
// Soft delete: WHERE enabled=TRUE so re-delete returns 0 rows → 404
// (per D-65). Cache DEL after the row flip (D-55).
func (h *Handlers) DeleteAgent(ctx context.Context, req api.DeleteAgentRequestObject) (api.DeleteAgentResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.DeleteAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	agentID := uuid.UUID(req.Id)

	q := generated.New(h.deps.OrgDB)
	tag, err := q.SoftDeleteAgent(ctx, generated.SoftDeleteAgentParams{
		ID:    pgUUID(agentID),
		OrgID: pgUUID(orgID),
	})
	if err != nil {
		h.deps.Logger.ErrorContext(ctx, "soft delete agent", "err", err)
		return api.DeleteAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "internal",
		}}, nil
	}
	if tag == 0 {
		// Row missing OR already disabled — both map to 404 (D-65).
		return api.DeleteAgent404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "agent_not_found",
		}}, nil
	}
	// D-55 — cache DEL after the row flip.
	if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "agents", agentID)); delErr != nil {
		h.deps.Logger.WarnContext(ctx, "cache del failed", "key", cache.Key(orgID, "agents", agentID), "err", delErr)
	}
	return api.DeleteAgent204Response{}, nil
}

// mapAgent converts a sqlc-row Agent + optional ListSkillsForAgentRow
// slice into the api.Agent DTO (CAT-01). skills==nil → the response
// omits the Skills field; list responses pass nil so the FLAT
// AgentListItem is used (OQ-1A). Detail GETs pass the loaded slice.
func mapAgent(row generated.Agent, skills []generated.ListSkillsForAgentRow) api.Agent {
	a := api.Agent{
		Id:         api.UUIDv7(apiUUID(row.ID)),
		OrgId:      api.UUIDv7(apiUUID(row.OrgID)),
		Code:       row.Code,
		ExternalId: row.ExternalID,
		Name:       row.Name,
		Email:      openapi_types.Email(row.Email),
		Enabled:    row.Enabled,
		Version:    int(row.Version),
		CreatedAt:  ptrTime(row.CreatedAt),
		UpdatedAt:  ptrTime(row.UpdatedAt),
	}
	if skills != nil {
		list := make([]api.AgentSkillAssignment, 0, len(skills))
		for _, s := range skills {
			name := s.SkillName
			list = append(list, api.AgentSkillAssignment{
				SkillId:     api.UUIDv7(apiUUID(s.SkillID)),
				Proficiency: int(s.Proficiency),
				Name:        &name,
			})
		}
		a.Skills = &list
	}
	return a
}

// mapAgentListItem produces the FLAT list-response item. Skills are
// intentionally omitted (OQ-1A — detail responses include skills, list
// responses are flat for performance + bandwidth).
func mapAgentListItem(row generated.Agent) api.AgentListItem {
	return api.AgentListItem{
		Id:         api.UUIDv7(apiUUID(row.ID)),
		OrgId:      api.UUIDv7(apiUUID(row.OrgID)),
		Code:       row.Code,
		ExternalId: row.ExternalID,
		Name:       row.Name,
		Email:      openapi_types.Email(row.Email),
		Enabled:    row.Enabled,
		Version:    int(row.Version),
		CreatedAt:  ptrTime(row.CreatedAt),
		UpdatedAt:  ptrTime(row.UpdatedAt),
	}
}

// ptrTime — pgtype.Timestamptz → *time.Time. Returns nil when the
// column was SQL NULL (Valid == false). The catalog schema never
// produces NULL for created_at/updated_at but the helper stays
// defensive.
func ptrTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// cursorTime / cursorID — translate a *Cursor (nil = first page) into
// the pgtype.Timestamptz / pgtype.UUID the sqlc.narg cursor params
// expect. Valid=false maps to SQL NULL → the IS NULL branch of the
// COALESCE/OR predicate fires (Pitfall 3a — narg ensures the null
// branch short-circuits).
func cursorTime(cur *Cursor) pgtype.Timestamptz {
	if cur == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: cur.CreatedAt, Valid: true}
}

func cursorID(cur *Cursor) pgtype.UUID {
	if cur == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: cur.ID, Valid: true}
}
