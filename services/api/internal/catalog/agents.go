// agents.go — CAT-01 + CAT-03 + CAT-08 + CAT-09 + CAT-10 + CAT-11 first
// complete CRUD entity. Plan 03-06 replaces the Plan 03-05 placeholders
// with the real handler bodies that establish the TEMPLATE every other
// entity copies in Plans 03-07/08/09.
//
// Locked patterns codified here (Codex iter 3 amendments):
//
//   - Codex C1 — Layer 2 proficiency validation runs BEFORE any DB call.
//     Out-of-range → 422 invalid_value (NOT 400). Plan 03-01 deliberately
//     OMITS minimum/maximum from AgentSkillAssignment.proficiency in the
//     OpenAPI spec so oapi-codegen does NOT short-circuit with 400; the
//     handler owns the wire shape. validateProficiencyRange returns the
//     offending index so the response can include
//     `skills[N].proficiency` in the Reason text.
//
//   - Codex C2 — CreateAgent422JSONResponse path covers BOTH FK miss
//     (unknown skill_id → invalid_reference) AND proficiency boundary
//     (out-of-range → invalid_value). CreateAgent uses the same
//     validation+tx pattern as UpdateAgent.
//
//   - Codex C4 — atomic UPDATE/INSERT + skills replace in a SINGLE
//     OrgDB.BeginTx. Both CreateAgent and UpdateAgent open exactly one
//     tx that wraps the agent row write AND the optional skills replace;
//     defer Rollback ensures full atomicity. The inline replaceAgentSkills
//     helper accepts qtx and composes inside the caller's tx — it does
//     NOT own its own BeginTx (Plan 03-09 will lift the helper into
//     agent_skills.go with the same shape).
//
//   - D-66 — UPDATE returns 0 rows → handler issues GetAgentByIdAnyVersion
//     (inside the same tx, so the probe observes the rollback) to
//     disambiguate 404 (no row) vs 409 (version mismatch). Per D-56, the
//     409 path also calls cache.Del so a stale cached value can never
//     mask a version conflict.
//
//   - D-55 — every mutation calls h.deps.Cache.Del AFTER tx.Commit. Cache
//     deletion failure logs a warn but does NOT downgrade the response.
//
//   - D-76 — cross-row FK probe uses SkillsPresentInOrg (returns the
//     subset of input that exists + enabled in caller's org) and the
//     handler computes the set difference `missing = input − present`.
//     The Wave 2 query is intentionally INVERTED so the SQLChecker
//     accepts the top-level FROM tenant table.
//
//   - CAT-11 — GetAgent goes through cache.GetOrSet[api.Agent] with key
//     `or:{orgID}:agents:{id}` and 60s TTL. ErrNotFound from the loader
//     propagates as 404 WITHOUT caching the empty value (D-54).
//
//   - CAT-10 — cursor pagination via DecodeCursor/EncodeCursor (D-63)
//     with LIMIT N+1 sentinel pattern. defaultPageSize=25, maxPageSize=100
//     (D-67). ?include_disabled=true switches to ListAgentsIncludingDisabled
//     (D-65 two-query split, plan-cacheable).
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

// ---------------------------------------------------------------------------
// CreateAgent (POST /v1/orgs/{org_id}/agents)  CAT-01, CAT-03
// ---------------------------------------------------------------------------
//
// Flow:
//
//  1. Resolve orgID from ctx (never from URL path — FOUND-05).
//  2. Codex C1: validate any skills[].proficiency in 1-10 BEFORE any DB
//     call. Out-of-range → 422 invalid_value.
//  3. Mint UUIDv7 server-side (D-19 — IDs originate at the write boundary).
//  4. Codex C4: BeginTx wrapping BOTH the agent INSERT AND the optional
//     skills replace. Defer Rollback covers every error branch.
//  5. InsertAgent via qtx. 23505 (unique violation) → 409 with the
//     CreateAgent409JSONResponseBody.FromErrorResponse union builder
//     (Pitfall 2). Other pgx errors → 500.
//  6. If skills[] non-empty: D-76 probe via SkillsPresentInOrg + set
//     difference. Missing IDs → 422 invalid_reference (CreateAgent422,
//     Codex C2). DELETE + N INSERT via qtx (composes inside caller tx).
//  7. tx.Commit then cache.Del (best-effort per D-55) — the cache may
//     be empty at this point, but DEL is idempotent and the consistent
//     pattern keeps the contract identical across create/update/delete.
//  8. Return 201 with the mapped Agent DTO (skills omitted in 201
//     response per the spec — list responses are flat).
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

	// Codex C1 iter 3 — Layer 2 proficiency check BEFORE any DB call.
	if req.Body.Skills != nil {
		if badIdx, valid := validateProficiencyRange(*req.Body.Skills); !valid {
			return api.CreateAgent422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidValue,
				Reason: fmt.Sprintf("skills[%d].proficiency must be 1-10", badIdx),
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
		ExternalID: req.Body.ExternalId,
		Name:       req.Body.Name,
		Email:      string(req.Body.Email),
		Enabled:    enabled,
	})
	if err != nil {
		status, code, reason := mapPgError(err, "agent")
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

	// Skills replace inside the SAME tx (Codex C4 — atomicity). Failure
	// here rolls back BOTH the agent insert and the skills writes.
	if req.Body.Skills != nil && len(*req.Body.Skills) > 0 {
		if errResp := replaceAgentSkills(ctx, qtx, orgID, id, *req.Body.Skills); errResp != nil {
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

// ---------------------------------------------------------------------------
// GetAgent (GET /v1/orgs/{org_id}/agents/{id})  CAT-01, CAT-11
// ---------------------------------------------------------------------------
//
// CAT-11 — wraps the loader in cache.GetOrSet[api.Agent] with a 60s TTL.
// The loader fetches the agent row + embedded skills (OQ-1A — detail
// GETs include skills, list responses do not). ErrNoRows from sqlc
// becomes cache.ErrNotFound so the cache does NOT write an empty entry
// (D-54).
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

// ---------------------------------------------------------------------------
// ListAgents (GET /v1/orgs/{org_id}/agents)  CAT-01, CAT-09, CAT-10
// ---------------------------------------------------------------------------
//
// Cursor pagination (CAT-10, D-63): cursor encodes (created_at, id).
// LIMIT N+1 sentinel signals has_more without a separate COUNT query.
// Default page size 25; max 100 (D-67) — handler validates because the
// OpenAPI spec intentionally lets the handler own the 400 wire shape.
//
// ?include_disabled=true (CAT-09): D-65 split — different prepared
// statement so PostgreSQL plans each path optimally.
//
// Name filter (OQ-4B): case-insensitive ILIKE %name%. Optional.
func (h *Handlers) ListAgents(ctx context.Context, req api.ListAgentsRequestObject) (api.ListAgentsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListAgents500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}

	// (a) Resolve page size (D-67).
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

	// (b) Decode cursor — empty string = first page (DecodeCursor returns
	//     nil, nil).
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

	// (c) Optional name filter.
	var nameFilter *string
	if req.Params.Name != nil {
		s := string(*req.Params.Name)
		nameFilter = &s
	}

	// (d) Dispatch to the appropriate sqlc query (D-65 two-query split).
	q := generated.New(h.deps.OrgDB)
	includeDisabled := derefOr(req.Params.IncludeDisabled, false)
	limit := int32(pageSize + 1) // N+1 sentinel.

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

	// (e) N+1 sentinel pagination.
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	// (f) Map rows → AgentListItem (OQ-1A — list items are FLAT, no
	//     embedded skills).
	items := make([]api.AgentListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapAgentListItem(r))
	}

	// (g) Build next_cursor from the last returned row.
	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		enc, encErr := EncodeCursor(last.CreatedAt.Time, apiUUID(last.ID))
		if encErr == nil {
			nextCursor = &enc
		}
	}
	return api.ListAgents200JSONResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// ---------------------------------------------------------------------------
// UpdateAgent (PATCH /v1/orgs/{org_id}/agents/{id})  CAT-01, CAT-03, CAT-08
// ---------------------------------------------------------------------------
//
// Codex C4 iter 3 — BeginTx wraps the version-checked UPDATE AND the
// optional skills replace in a SINGLE tx. If the skills replace fails,
// the agent row UPDATE rolls back along with it.
//
// D-66 — UPDATE returns 0 rows means EITHER the row doesn't exist (404)
// OR the version mismatched (409). Disambiguate via
// GetAgentByIdAnyVersion run inside the same tx (so the probe observes
// the rollback state). 409 path also cache.Del's per D-56 to avoid a
// stale cached value masking the conflict.
//
// Codex C1 — proficiency 1-10 check before any DB call.
// Codex C3 — sqlc UpdateAgentParams uses COALESCE for sparse-PATCH so
// the handler passes nil for omitted fields and the column is preserved.
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

	// Codex C1 iter 3 — Layer 2 proficiency check BEFORE any DB call.
	if req.Body.Skills != nil {
		if badIdx, valid := validateProficiencyRange(*req.Body.Skills); !valid {
			return api.UpdateAgent422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidValue,
				Reason: fmt.Sprintf("skills[%d].proficiency must be 1-10", badIdx),
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

	// Codex C3 — pass nil for omitted fields; COALESCE preserves the
	// existing column. Email is *openapi_types.Email aliased to *string.
	var emailStr *string
	if req.Body.Email != nil {
		s := string(*req.Body.Email)
		emailStr = &s
	}

	// Atomic version-checked UPDATE (D-66).
	row, err := qtx.UpdateAgent(ctx, generated.UpdateAgentParams{
		ID:              pgUUID(agentID),
		OrgID:           pgUUID(orgID),
		ExpectedVersion: int32(req.Body.Version),
		Name:            req.Body.Name,
		Email:           emailStr,
		Enabled:         req.Body.Enabled,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 0 rows — disambiguate 404 vs 409 inside the same tx.
		cur, perr := qtx.GetAgentByIdAnyVersion(ctx, generated.GetAgentByIdAnyVersionParams{
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
		// 409 — defer Rollback runs (no UPDATE landed). D-56 cache DEL
		// so a stale cached value can never mask the conflict.
		if delErr := h.deps.Cache.Del(ctx, cache.Key(orgID, "agents", agentID)); delErr != nil {
			h.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", "key", cache.Key(orgID, "agents", agentID), "err", delErr)
		}
		return api.UpdateAgent409JSONResponse{
			Current: mapAgent(cur, nil),
			Error:   api.UpdateAgent409JSONResponseBodyErrorVersionConflict,
			Reason:  "version_mismatch",
		}, nil
	}
	if err != nil {
		status, code, reason := mapPgError(err, "agent")
		if status == 422 {
			return api.UpdateAgent422JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
		}
		h.deps.Logger.ErrorContext(ctx, "update agent", "err", err)
		return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: code, Reason: reason,
		}}, nil
	}

	// Happy path UPDATE returned a row — skills replace inside the SAME
	// tx (Codex C4). Failure rolls back the agent UPDATE too.
	if req.Body.Skills != nil {
		if errResp := replaceAgentSkills(ctx, qtx, orgID, agentID, *req.Body.Skills); errResp != nil {
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

	// Load fresh skills for the response so the PATCH echo includes the
	// post-state (separate query — the tx has committed, so a new pool
	// connection sees the new state via MVCC).
	freshQ := generated.New(h.deps.OrgDB)
	skills, _ := freshQ.ListSkillsForAgent(ctx, generated.ListSkillsForAgentParams{
		AgentID: pgUUID(agentID),
		OrgID:   pgUUID(orgID),
	})
	return api.UpdateAgent200JSONResponse(mapAgent(row, skills)), nil
}

// ---------------------------------------------------------------------------
// DeleteAgent (DELETE /v1/orgs/{org_id}/agents/{id})  CAT-01, CAT-09
// ---------------------------------------------------------------------------
//
// Soft delete (CAT-09): SoftDeleteAgent flips enabled=false WHERE
// enabled=TRUE. Re-deleting an already-disabled row returns 0 rows →
// 404 (the documented idempotent-disabled contract from D-65).
//
// Cache DEL after the row is gone (D-55).
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

// ---------------------------------------------------------------------------
// Helpers — inlined here for Plan 03-06. Plan 03-09 will extract
// replaceAgentSkills + validateProficiencyRange into agent_skills.go.
// ---------------------------------------------------------------------------

// validateProficiencyRange — Codex C1 iter 3. Layer 2 validation of
// proficiency 1-10. Returns the index of the first out-of-range entry
// (badIdx) and ok=false; otherwise (0, true).
//
// The OpenAPI spec for AgentSkillAssignment.proficiency intentionally
// OMITS minimum/maximum so the oapi-codegen Layer 1 validator does NOT
// short-circuit out-of-range values with HTTP 400. This handler-level
// check fires the 422 invalid_value response the contract requires.
// Caller embeds the index in the Reason string: `skills[N].proficiency
// must be 1-10`.
func validateProficiencyRange(assignments []api.AgentSkillAssignment) (badIdx int, ok bool) {
	for i, a := range assignments {
		if a.Proficiency < 1 || a.Proficiency > 10 {
			return i, false
		}
	}
	return 0, true
}

// replaceAgentSkills — Codex C4 iter 3. PUT-semantics full-replace of
// the agent_skills join rows for one (agentID, orgID) pair.
//
// CRITICAL: this helper does NOT open its own BeginTx. It accepts the
// caller's *generated.Queries (from generated.New(tx)) and composes
// inside the enclosing tx. The exact-2 BeginTx gate in Plan 03-06's
// acceptance criteria depends on this — see the awk gate that scans
// for "BeginTx" inside this function's body.
//
// Returns:
//
//   - nil → success; caller proceeds to commit.
//   - *api.ErrorResponse with Error==ErrorCodeInternal → internal
//     failure; caller maps to 500 + tx auto-rollback.
//   - *api.ErrorResponse with Error==ErrorCodeInvalidReference (D-76
//     unknown skill_id) or ErrorCodeInvalidValue (DB CHECK backstop
//     via 23514) → 422 response; caller wraps in CreateAgent422 or
//     UpdateAgent422.
//
// Steps:
//
//  1. D-76 cross-row FK probe via SkillsPresentInOrg (returns the
//     subset of input that exists + enabled in caller's org). Handler
//     computes the SET DIFFERENCE: `missing = input − present`.
//
//  2. DELETE every existing agent_skills row for (agentID, orgID).
//     Idempotent — :execrows return is unused.
//
//  3. INSERT one row per assignment via qtx.InsertAgentSkill. Failures
//     pass through mapPgError; the caller's defer Rollback covers any
//     partial state.
//
// Plan 03-09 will lift this helper into agent_skills.go with the SAME
// signature so the awk gate stays satisfied and the test surface
// doesn't drift.
func replaceAgentSkills(
	ctx context.Context,
	qtx *generated.Queries,
	orgID, agentID uuid.UUID,
	assignments []api.AgentSkillAssignment,
) *api.ErrorResponse {
	// 1. Cross-row FK probe (D-76). Build the []pgtype.UUID slice the
	//    sqlc-generated param expects.
	inputIDs := make([]uuid.UUID, 0, len(assignments))
	pgInputIDs := make([]pgtype.UUID, 0, len(assignments))
	for _, a := range assignments {
		id := uuid.UUID(a.SkillId)
		inputIDs = append(inputIDs, id)
		pgInputIDs = append(pgInputIDs, pgUUID(id))
	}

	present, err := qtx.SkillsPresentInOrg(ctx, generated.SkillsPresentInOrgParams{
		Column1: pgInputIDs,
		OrgID:   pgUUID(orgID),
	})
	if err != nil {
		return &api.ErrorResponse{
			Error:  api.ErrorCodeInternal,
			Reason: "skills_probe_failed",
		}
	}

	// Set difference: missing = inputIDs − present. Build a map of the
	// present IDs (the SkillsPresentInOrg query returns a slice of
	// pgtype.UUID — each Bytes is the 16-byte UUID).
	presentSet := make(map[uuid.UUID]struct{}, len(present))
	for _, p := range present {
		presentSet[uuid.UUID(p.Bytes)] = struct{}{}
	}
	for _, id := range inputIDs {
		if _, ok := presentSet[id]; !ok {
			return &api.ErrorResponse{
				Error:  api.ErrorCodeInvalidReference,
				Reason: fmt.Sprintf("unknown_skill_id:%s", id),
			}
		}
	}

	// 2. DELETE existing rows for (agentID, orgID). Idempotent.
	if _, err := qtx.DeleteAgentSkills(ctx, generated.DeleteAgentSkillsParams{
		AgentID: pgUUID(agentID),
		OrgID:   pgUUID(orgID),
	}); err != nil {
		return &api.ErrorResponse{
			Error:  api.ErrorCodeInternal,
			Reason: "delete_agent_skills_failed",
		}
	}

	// 3. INSERT each assignment. Any failure rolls the tx back via the
	//    caller's defer.
	for _, a := range assignments {
		if _, err := qtx.InsertAgentSkill(ctx, generated.InsertAgentSkillParams{
			AgentID:     pgUUID(agentID),
			SkillID:     pgUUID(uuid.UUID(a.SkillId)),
			OrgID:       pgUUID(orgID),
			Proficiency: int32(a.Proficiency),
		}); err != nil {
			// 23503 FK race (skill disabled between probe and insert) →
			// 422 invalid_reference. 23514 CHECK (DB-level proficiency
			// 1-10 backstop) → 422 invalid_value. Other errors → 500.
			status, code, reason := mapPgError(err, "agent_skill")
			if status == 422 {
				return &api.ErrorResponse{Error: code, Reason: reason}
			}
			return &api.ErrorResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "insert_agent_skill_failed",
			}
		}
	}
	return nil
}

// mapAgent converts a sqlc-row Agent + optional ListSkillsForAgentRow
// slice into the api.Agent DTO (CAT-01). skills==nil → the response
// omits the Skills field; list responses pass nil so the FLAT
// AgentListItem is used (OQ-1A). Detail GETs pass the loaded slice.
func mapAgent(row generated.Agent, skills []generated.ListSkillsForAgentRow) api.Agent {
	a := api.Agent{
		Id:         api.UUIDv7(apiUUID(row.ID)),
		OrgId:      api.UUIDv7(apiUUID(row.OrgID)),
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
