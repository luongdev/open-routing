package state

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

// agentEnabledCheck returns false when the agent row exists but is
// soft-deleted (enabled=false). A soft-deleted agent must produce 404 for
// all state operations — same surface as "agent does not exist" per FOUND-08.
func (s *Server) agentEnabledCheck(ctx context.Context, orgID, agentID uuid.UUID) (ok bool, err error) {
	q := generated.New(s.deps.OrgDB)
	agent, ferr := q.GetAgent(ctx, generated.GetAgentParams{
		ID:    pgUUID(agentID),
		OrgID: pgUUID(orgID),
	})
	if errors.Is(ferr, pgx.ErrNoRows) {
		return false, nil
	}
	if ferr != nil {
		return false, ferr
	}
	return agent.Enabled, nil
}

// GetAgentStatus serves the agent_states row through cache.GetOrSet with
// 60s TTL keyed `or:{orgId}:agent_state:{agent_id}` (D-86). Cache miss
// hits the DB via generated.New(orgDB).GetAgentStateByAgentId; ErrNoRows
// surfaces as cache.ErrNotFound → 404 (STATE-01 read-side).
func (s *Server) GetAgentStatus(ctx context.Context, req api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error:  api.ErrorCodeInternal,
			Reason: "missing_org_id_in_context",
		}}, nil
	}
	agentID := uuid.UUID(req.Id)

	// Soft-deleted agents return 404 (Gemini HIGH — treat enabled=false as
	// non-existent for state operations; preserves FOUND-08 surface).
	enabled, checkErr := s.agentEnabledCheck(ctx, orgID, agentID)
	if checkErr != nil {
		s.deps.Logger.ErrorContext(ctx, "get agent status: enabled check", "agent_id", agentID, "org_id", orgID, "err", checkErr)
		return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error:  api.ErrorCodeInternal,
			Reason: "agent_enabled_check_failed",
		}}, nil
	}
	if !enabled {
		return api.GetAgentStatus404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error:  api.ErrorCodeNotFound,
			Reason: "agent_state_not_found",
		}}, nil
	}

	key := s.cacheKeyFor(orgID, agentID)

	state, err := cache.GetOrSet[api.AgentState](ctx, s.deps.Cache, key, stateCacheTTL,
		func(ctx context.Context) (api.AgentState, error) {
			q := generated.New(s.deps.OrgDB)
			row, ferr := q.GetAgentStateByAgentId(ctx, generated.GetAgentStateByAgentIdParams{
				AgentID: pgUUID(agentID),
				OrgID:   pgUUID(orgID),
			})
			if errors.Is(ferr, pgx.ErrNoRows) {
				return api.AgentState{}, cache.ErrNotFound
			}
			if ferr != nil {
				return api.AgentState{}, fmt.Errorf("get agent state: %w", ferr)
			}
			return mapAgentState(row), nil
		})

	switch {
	case errors.Is(err, cache.ErrNotFound):
		return api.GetAgentStatus404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error:  api.ErrorCodeNotFound,
			Reason: "agent_state_not_found",
		}}, nil
	case err != nil:
		s.deps.Logger.ErrorContext(ctx, "get agent status", "agent_id", agentID, "org_id", orgID, "err", err)
		return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error:  api.ErrorCodeInternal,
			Reason: "agent_state_load_failed",
		}}, nil
	}
	return api.GetAgentStatus200JSONResponse(state), nil
}

// PatchAgentStatus implements PATCH /agents/{id}/status with the
// probe-then-matrix ordering required by Pitfall 3 and D-84/D-85/D-66.
// Control flow is: (1) orgID extract, (2) body validation, (3) break_reason
// probe FIRST regardless of force flag, (4) transition matrix OR force bypass,
// (5) tx + atomic UPDATE, (6) 0-row disambiguate, (7) commit, (8) cache.Del,
// (9) force WARN audit post-commit.
func (s *Server) PatchAgentStatus(ctx context.Context, req api.PatchAgentStatusRequestObject) (api.PatchAgentStatusResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.PatchAgentStatus400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "body_required",
		}}, nil
	}

	// Validate enums before any DB work. force=true with an invalid `to`
	// would otherwise reach the Postgres CHECK constraint and return 500 (23514).
	if !req.Body.To.Valid() {
		return api.PatchAgentStatus422JSONResponse(api.ErrorResponse{
			Error:  api.ErrorCodeInvalidValue,
			Reason: "invalid_value:to",
		}), nil
	}
	if req.Body.PostInteractionState != nil && !req.Body.PostInteractionState.Valid() {
		return api.PatchAgentStatus422JSONResponse(api.ErrorResponse{
			Error:  api.ErrorCodeInvalidValue,
			Reason: "invalid_value:post_interaction_state",
		}), nil
	}

	agentID := uuid.UUID(req.Id)
	forced := req.Body.Force != nil && *req.Body.Force
	targetStatus := req.Body.To

	// Soft-deleted agents return 404 (Gemini HIGH — treat enabled=false as
	// non-existent for state operations; preserves FOUND-08 surface).
	enabled, checkErr := s.agentEnabledCheck(ctx, orgID, agentID)
	if checkErr != nil {
		s.deps.Logger.ErrorContext(ctx, "patch agent status: enabled check", "agent_id", agentID, "org_id", orgID, "err", checkErr)
		return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "agent_enabled_check_failed",
		}}, nil
	}
	if !enabled {
		return api.PatchAgentStatus404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "agent_state_not_found",
		}}, nil
	}

	// Load current row FIRST so we have observed `from` for matrix
	// validation, 409 body, and the UpdateAgentStateStatus expected_from
	// predicate (D-85). Single SELECT cheaper than carrying the row
	// through the tx for D-66 disambiguate.
	q := generated.New(s.deps.OrgDB)
	current, err := q.GetAgentStateByAgentId(ctx, generated.GetAgentStateByAgentIdParams{
		AgentID: pgUUID(agentID),
		OrgID:   pgUUID(orgID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.PatchAgentStatus404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "agent_state_not_found",
		}}, nil
	}
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "patch agent status: load current", "agent_id", agentID, "org_id", orgID, "err", err)
		return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "agent_state_load_failed",
		}}, nil
	}
	observedFrom := api.AgentStatus(current.Status)

	// STEP 1 — BREAK_REASON PROBE (Pitfall 3 / T-04-06): runs FIRST,
	// independent of force flag. Cross-org or missing/disabled break_reason_id
	// returns 422 invalid_reference even when force=true (D-84 explicit).
	if targetStatus == api.AgentStatusBreak {
		if req.Body.BreakReasonId == nil {
			return api.PatchAgentStatus422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidValue,
				Reason: "break_reason_id_required_for_break",
			}), nil
		}
		breakReasonID := uuid.UUID(*req.Body.BreakReasonId)
		_, qErr := q.GetBreakReasonForState(ctx, generated.GetBreakReasonForStateParams{
			ID:    pgUUID(breakReasonID),
			OrgID: pgUUID(orgID),
		})
		if errors.Is(qErr, pgx.ErrNoRows) {
			return api.PatchAgentStatus422JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidReference,
				Reason: "break_reason_id_not_found_or_disabled",
			}), nil
		}
		if qErr != nil {
			s.deps.Logger.ErrorContext(ctx, "patch agent status: break_reason probe", "agent_id", agentID, "org_id", orgID, "err", qErr)
			return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "break_reason_probe_failed",
			}}, nil
		}
	}

	// STEP 2 — TRANSITION MATRIX or force bypass (D-84). force=true skips
	// the matrix entirely; matrix-validated edges still enforce
	// RequiresBreakReasonID via the probe above (Step 1 caught the nil case).
	if !forced {
		if _, vErr := validateTransition(observedFrom, targetStatus); vErr != nil {
			return api.PatchAgentStatus409JSONResponse(api.InvalidTransitionErrorResponse{
				Error: api.InvalidTransition,
				From:  observedFrom,
				To:    targetStatus,
			}), nil
		}
	}

	// STEP 3 — TX + ATOMIC UPDATE (D-66 adapted for D-85). Non-force path
	// uses UpdateAgentStateStatus with `expected_from = observedFrom`;
	// force path uses ForceUpdateAgentStateStatus (no status predicate).
	tx, txErr := s.deps.OrgDB.BeginTx(ctx)
	if txErr != nil {
		s.deps.Logger.ErrorContext(ctx, "patch agent status: begin tx", "err", txErr)
		return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "tx_begin_failed",
		}}, nil
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	var row generated.AgentState
	if forced {
		row, err = qtx.ForceUpdateAgentStateStatus(ctx, buildForceUpdateParams(agentID, orgID, observedFrom, req.Body))
	} else {
		row, err = qtx.UpdateAgentStateStatus(ctx, buildUpdateParams(agentID, orgID, observedFrom, req.Body))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		// 0 rows landed. D-66 disambiguate: either the row vanished (404)
		// or a concurrent PATCH shifted off observedFrom (409 invalid_transition).
		cur, perr := qtx.GetAgentStateByAgentId(ctx, generated.GetAgentStateByAgentIdParams{
			AgentID: pgUUID(agentID),
			OrgID:   pgUUID(orgID),
		})
		if errors.Is(perr, pgx.ErrNoRows) {
			return api.PatchAgentStatus404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
				Error: api.ErrorCodeNotFound, Reason: "agent_state_not_found",
			}}, nil
		}
		if perr != nil {
			s.deps.Logger.ErrorContext(ctx, "patch agent status: disambiguate", "err", perr)
			return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "disambiguate_failed",
			}}, nil
		}
		// D-56: 409 path also DELs cache so subsequent GET reloads fresh.
		if delErr := s.deps.Cache.Del(ctx, s.cacheKeyFor(orgID, agentID)); delErr != nil {
			s.deps.Logger.WarnContext(ctx, "cache del failed (409 path)", "key", s.cacheKeyFor(orgID, agentID), "err", delErr)
		}
		return api.PatchAgentStatus409JSONResponse(api.InvalidTransitionErrorResponse{
			Error: api.InvalidTransition,
			From:  api.AgentStatus(cur.Status),
			To:    targetStatus,
		}), nil
	}
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "patch agent status: update", "err", err)
		return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "update_failed",
		}}, nil
	}
	if err := tx.Commit(ctx); err != nil {
		s.deps.Logger.ErrorContext(ctx, "patch agent status: commit", "err", err)
		return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "commit_failed",
		}}, nil
	}

	// STEP 4 — Cache invalidation (D-55). Failure logs WARN; never 5xx.
	if delErr := s.deps.Cache.Del(ctx, s.cacheKeyFor(orgID, agentID)); delErr != nil {
		s.deps.Logger.WarnContext(ctx, "cache del failed", "key", s.cacheKeyFor(orgID, agentID), "err", delErr)
	}

	// STEP 5 — D-84 force audit. Emitted AFTER successful commit so the
	// audit log reflects an actual state change.
	if forced {
		s.deps.Logger.WarnContext(ctx, "state.force.applied",
			"agent_id", agentID,
			"from", observedFrom,
			"to", targetStatus,
			"org_id", orgID,
		)
	}

	return api.PatchAgentStatus200JSONResponse(mapAgentState(row)), nil
}

// buildUpdateParams translates the request body into UpdateAgentStateStatusParams.
// Engaged channel and wrapup_until are system-set only — not settable via
// agent-initiated PATCH per D-82.
//
// Cross-field invariant (Codex HIGH): break_reason_id is only written when
// to==Break; cleared on all other transitions. post_interaction_state is only
// written while current status==Engaged (the only state where STATE-06 applies);
// cleared when leaving Engaged.
func buildUpdateParams(agentID, orgID uuid.UUID, expectedFrom api.AgentStatus, body *api.PatchAgentStatusJSONRequestBody) generated.UpdateAgentStateStatusParams {
	p := generated.UpdateAgentStateStatusParams{
		AgentID:      pgUUID(agentID),
		OrgID:        pgUUID(orgID),
		ExpectedFrom: string(expectedFrom),
		ToStatus:     strPtr(string(body.To)),
	}
	if body.To == api.AgentStatusBreak && body.BreakReasonId != nil {
		p.BreakReasonID = pgtype.UUID{Bytes: uuid.UUID(*body.BreakReasonId), Valid: true}
	}
	if expectedFrom == api.AgentStatusEngaged && body.PostInteractionState != nil {
		p.PostInteractionState = strPtr(string(*body.PostInteractionState))
	}
	return p
}

// buildForceUpdateParams translates the request body into ForceUpdateAgentStateStatusParams.
//
// Cross-field invariant (Codex HIGH): same constraints as buildUpdateParams apply.
// Force bypasses the transition matrix, but not cross-field column semantics.
func buildForceUpdateParams(agentID, orgID uuid.UUID, currentStatus api.AgentStatus, body *api.PatchAgentStatusJSONRequestBody) generated.ForceUpdateAgentStateStatusParams {
	p := generated.ForceUpdateAgentStateStatusParams{
		AgentID:  pgUUID(agentID),
		OrgID:    pgUUID(orgID),
		ToStatus: strPtr(string(body.To)),
	}
	if body.To == api.AgentStatusBreak && body.BreakReasonId != nil {
		p.BreakReasonID = pgtype.UUID{Bytes: uuid.UUID(*body.BreakReasonId), Valid: true}
	}
	if currentStatus == api.AgentStatusEngaged && body.PostInteractionState != nil {
		p.PostInteractionState = strPtr(string(*body.PostInteractionState))
	}
	return p
}

func strPtr(s string) *string { return &s }
