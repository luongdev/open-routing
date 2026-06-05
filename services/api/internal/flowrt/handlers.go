// Package flowrt holds the v0.2 control-plane HTTP handlers for flow validate /
// publish / rollback / simulate and the runtime read surface (route requests,
// reservations, traces). These are the endpoints the flow UI binds to.
//
// Wave 2 implements validate, publish, rollback, and the version read surface
// against internal/runtime + sqlc. Simulate and the route-request/reservation/
// trace surface remain 501 stubs until Wave 3/4. The stubs deliberately do NOT
// touch the receiver, so a nil *Endpoints embedded in a test composite still
// answers (returns 500 not_implemented) instead of panicking.
//
// The type is named Endpoints (not Handlers) so it embeds into the ApiHandlers
// composite under a field name distinct from catalog.Handlers / state.Server /
// imports.Importer (Pitfall 1 — no duplicate anonymous field).
package flowrt

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/presence"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
)

const maxVersionsPerList = 100

// Deps carries the handler dependencies. reg is the shared node registry,
// constructed once in New and reused for validate/compile across requests.
type Deps struct {
	OrgDB *db.OrgDB
	Cache *cache.Cache
	// Presence + Capacity make routing LIVE (offerability from a connection lease
	// + DB-solid capacity). When nil the route path runs in simulation mode
	// (buildSnapshot, no capacity holds). A real binary (cmd/api) always wires
	// both — see routingSnapshot (review BLOCK: no silent live→sim fallback).
	Presence presence.Store
	Capacity *CapacityService
	// MatcherEnabled turns on the W4 queue model: a route with no available agent
	// parks waiting_match for the matcher instead of falling through to fallback.
	// Off until the matcher loop (cmd/runtime, W4 Stage 3) is wired, else parked
	// routes would never be pulled.
	MatcherEnabled bool
	// MatcherBatch bounds how many agents/orgs/expired routes one matcher tick
	// processes. 0 ⇒ matcherBatchDefault. A full batch is logged (no silent caps);
	// the remainder is picked up next tick.
	MatcherBatch int
	// Adapters is the channel→ChannelAdapter registry. On accept the engine hands
	// the assignment to the matching adapter (Deliver) and maps its lifecycle events
	// back onto the reservation; nil/absent ⇒ no media delivery (the WS/HTTP
	// test-double path still drives accept/complete directly).
	Adapters map[string]adapter.ChannelAdapter
	Logger   *slog.Logger
}

// matcherMode reports whether to run reservations in W4 queue mode.
func (e *Endpoints) matcherMode() bool {
	return e.deps.MatcherEnabled && e.deps.Capacity != nil && e.deps.Presence != nil
}

type Endpoints struct {
	deps Deps
	reg  *runtime.Registry
}

func New(deps Deps) *Endpoints { return &Endpoints{deps: deps, reg: runtime.DefaultRegistry()} }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ValidateFlow — POST /v1/orgs/{org_id}/flows/{id}/validate. Structural + node +
// catalog-reference validation of the draft graph. Always 200 with a result
// (valid flag + issues); 404 only if the draft is missing.
func (e *Endpoints) ValidateFlow(ctx context.Context, req api.ValidateFlowRequestObject) (api.ValidateFlowResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ValidateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	q := generated.New(e.deps.OrgDB)
	flow, err := q.GetFlowByIdAnyVersion(ctx, generated.GetFlowByIdAnyVersionParams{
		ID: pgUUID(uuid.UUID(req.Id)), OrgID: pgUUID(orgID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.ValidateFlow404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "flow_not_found",
		}}, nil
	}
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "validate: load flow", "err", err)
		return api.ValidateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "load_failed",
		}}, nil
	}

	graph, gErr := parseGraph(flow.Graph)
	if gErr != nil {
		e.deps.Logger.ErrorContext(ctx, "validate: parse graph", "err", gErr)
		return api.ValidateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "graph_parse_failed",
		}}, nil
	}

	refs := newDBRefs(ctx, q, pgUUID(orgID))
	issues, vErr := runtime.ValidateGraph(ctx, graph, e.reg, refs)
	if vErr != nil {
		e.deps.Logger.ErrorContext(ctx, "validate: graph", "err", vErr)
		return api.ValidateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "validation_failed",
		}}, nil
	}
	if refs.Err() != nil {
		e.deps.Logger.ErrorContext(ctx, "validate: catalog refs", "err", refs.Err())
		return api.ValidateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "catalog_ref_failed",
		}}, nil
	}

	return api.ValidateFlow200JSONResponse(api.FlowValidationResult{
		Valid:  len(issues) == 0,
		Issues: toAPIIssues(issues),
	}), nil
}

// PublishFlow — POST /v1/orgs/{org_id}/flows/{id}/publish. Validates + compiles
// the draft, writes an immutable flow_version, and transactionally activates
// the (channel, entry_code) binding. Returns 422 with issues if the graph is
// invalid, 409 on optimistic-concurrency conflict.
func (e *Endpoints) PublishFlow(ctx context.Context, req api.PublishFlowRequestObject) (api.PublishFlowResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.PublishFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.PublishFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}

	q := generated.New(e.deps.OrgDB)
	flow, err := q.GetFlowByIdAnyVersion(ctx, generated.GetFlowByIdAnyVersionParams{
		ID: pgUUID(uuid.UUID(req.Id)), OrgID: pgUUID(orgID),
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !flow.Enabled) {
		return api.PublishFlow404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "flow_not_found",
		}}, nil
	}
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "publish: load flow", "err", err)
		return api.PublishFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "load_failed",
		}}, nil
	}
	// Optimistic concurrency against the draft being published.
	if int(flow.Version) != req.Body.Version {
		return api.PublishFlow409JSONResponse(api.ErrorResponse{
			Error: api.ErrorCodeVersionConflict, Reason: "draft_version_mismatch",
		}), nil
	}

	graph, gErr := parseGraph(flow.Graph)
	if gErr != nil {
		e.deps.Logger.ErrorContext(ctx, "publish: parse graph", "err", gErr)
		return api.PublishFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "graph_parse_failed",
		}}, nil
	}
	refs := newDBRefs(ctx, q, pgUUID(orgID))
	issues, plan, cErr := runtime.ValidateAndCompile(ctx, graph, e.reg, refs)
	if cErr != nil {
		e.deps.Logger.ErrorContext(ctx, "publish: validate/compile", "err", cErr)
		return api.PublishFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "compile_failed",
		}}, nil
	}
	if refs.Err() != nil {
		e.deps.Logger.ErrorContext(ctx, "publish: catalog refs", "err", refs.Err())
		return api.PublishFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "catalog_ref_failed",
		}}, nil
	}
	if len(issues) > 0 {
		return api.PublishFlow422JSONResponse(api.FlowValidationResult{Valid: false, Issues: toAPIIssues(issues)}), nil
	}

	planBytes, mErr := json.Marshal(plan)
	if mErr != nil {
		e.deps.Logger.ErrorContext(ctx, "publish: marshal plan", "err", mErr)
		return api.PublishFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "plan_marshal_failed",
		}}, nil
	}

	version, binding, pErr := e.activate(ctx, orgID, activation{
		flowID:       flow.ID,
		flowCode:     flow.Code,
		draftVersion: req.Body.Version,
		channel:      req.Body.Channel,
		entryCode:    req.Body.EntryCode,
		expected:     req.Body.ExpectedCurrentFlowVersionId,
		graph:        flow.Graph,
		plan:         planBytes,
	})
	switch {
	case errors.Is(pErr, errDraftConflict):
		return api.PublishFlow409JSONResponse(api.ErrorResponse{
			Error: api.ErrorCodeVersionConflict, Reason: "draft_version_mismatch",
		}), nil
	case errors.Is(pErr, errBindingConflict):
		return api.PublishFlow409JSONResponse(api.ErrorResponse{
			Error: api.ErrorCodeVersionConflict, Reason: "binding_changed",
		}), nil
	case pErr != nil:
		e.deps.Logger.ErrorContext(ctx, "publish: activate", "err", pErr)
		return api.PublishFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "publish_failed",
		}}, nil
	}

	ver, vmErr := mapFlowVersion(version)
	if vmErr != nil {
		e.deps.Logger.ErrorContext(ctx, "publish: map version", "err", vmErr)
		return api.PublishFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "map_failed",
		}}, nil
	}
	return api.PublishFlow201JSONResponse(api.FlowPublishResult{Version: ver, Binding: mapBinding(binding)}), nil
}

// RollbackFlow — POST /v1/orgs/{org_id}/flows/{id}/rollback. Re-activates an
// existing published version for the (channel, entry_code) binding. No new
// version is written.
func (e *Endpoints) RollbackFlow(ctx context.Context, req api.RollbackFlowRequestObject) (api.RollbackFlowResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.RollbackFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	if req.Body == nil {
		return api.RollbackFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "missing_body",
		}}, nil
	}
	// version_number is INT4 in PG; reject out-of-range before the int32 cast in
	// rollback() so a wrap can't resolve to a real version (cross-AI HIGH-3).
	if req.Body.ToVersionNumber < 1 || req.Body.ToVersionNumber > math.MaxInt32 {
		return api.RollbackFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeInvalidBody, Reason: "to_version_number_out_of_range",
		}}, nil
	}

	q := generated.New(e.deps.OrgDB)
	flow, err := q.GetFlowByIdAnyVersion(ctx, generated.GetFlowByIdAnyVersionParams{
		ID: pgUUID(uuid.UUID(req.Id)), OrgID: pgUUID(orgID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.RollbackFlow404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "flow_not_found",
		}}, nil
	}
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "rollback: load flow", "err", err)
		return api.RollbackFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "load_failed",
		}}, nil
	}

	version, binding, rErr := e.rollback(ctx, orgID, rollbackTarget{
		flowCode:      flow.Code,
		channel:       req.Body.Channel,
		entryCode:     req.Body.EntryCode,
		versionNumber: req.Body.ToVersionNumber,
		expected:      req.Body.ExpectedCurrentFlowVersionId,
	})
	switch {
	case errors.Is(rErr, errVersionNotFound):
		return api.RollbackFlow404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "flow_version_not_found",
		}}, nil
	case errors.Is(rErr, errBindingConflict):
		return api.RollbackFlow400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Error: api.ErrorCodeVersionConflict, Reason: "binding_changed",
		}}, nil
	case rErr != nil:
		e.deps.Logger.ErrorContext(ctx, "rollback: activate", "err", rErr)
		return api.RollbackFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "rollback_failed",
		}}, nil
	}

	ver, vmErr := mapFlowVersion(version)
	if vmErr != nil {
		e.deps.Logger.ErrorContext(ctx, "rollback: map version", "err", vmErr)
		return api.RollbackFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "map_failed",
		}}, nil
	}
	return api.RollbackFlow200JSONResponse(api.FlowPublishResult{Version: ver, Binding: mapBinding(binding)}), nil
}

// ListFlowVersions — GET /v1/orgs/{org_id}/flows/{id}/versions.
func (e *Endpoints) ListFlowVersions(ctx context.Context, req api.ListFlowVersionsRequestObject) (api.ListFlowVersionsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListFlowVersions500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	q := generated.New(e.deps.OrgDB)
	flow, err := q.GetFlowByIdAnyVersion(ctx, generated.GetFlowByIdAnyVersionParams{
		ID: pgUUID(uuid.UUID(req.Id)), OrgID: pgUUID(orgID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.ListFlowVersions404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "flow_not_found",
		}}, nil
	}
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "list versions: load flow", "err", err)
		return api.ListFlowVersions500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "load_failed",
		}}, nil
	}
	rows, err := q.ListFlowVersionsByCode(ctx, generated.ListFlowVersionsByCodeParams{
		OrgID: pgUUID(orgID), FlowCode: flow.Code, Limit: maxVersionsPerList,
	})
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "list versions", "err", err)
		return api.ListFlowVersions500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "list_failed",
		}}, nil
	}
	items := make([]api.FlowVersion, 0, len(rows))
	for _, r := range rows {
		v, mErr := mapFlowVersion(r)
		if mErr != nil {
			e.deps.Logger.ErrorContext(ctx, "list versions: map", "err", mErr)
			return api.ListFlowVersions500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error: api.ErrorCodeInternal, Reason: "map_failed",
			}}, nil
		}
		items = append(items, v)
	}
	return api.ListFlowVersions200JSONResponse{Items: items}, nil
}

// GetFlowVersion — GET /v1/orgs/{org_id}/flow-versions/{id} (id = flow_version id).
func (e *Endpoints) GetFlowVersion(ctx context.Context, req api.GetFlowVersionRequestObject) (api.GetFlowVersionResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetFlowVersion500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	q := generated.New(e.deps.OrgDB)
	row, err := q.GetFlowVersion(ctx, generated.GetFlowVersionParams{
		ID: pgUUID(uuid.UUID(req.Id)), OrgID: pgUUID(orgID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.GetFlowVersion404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
			Error: api.ErrorCodeNotFound, Reason: "flow_version_not_found",
		}}, nil
	}
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "get version", "err", err)
		return api.GetFlowVersion500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "load_failed",
		}}, nil
	}
	v, mErr := mapFlowVersion(row)
	if mErr != nil {
		e.deps.Logger.ErrorContext(ctx, "get version: map", "err", mErr)
		return api.GetFlowVersion500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "map_failed",
		}}, nil
	}
	return api.GetFlowVersion200JSONResponse(v), nil
}

// ---- Wave 3/4 stubs ----

// SimulateFlow + ListFlowTraces live in trace_sim.go (3b).

// CreateRouteRequest + reads live in route_live.go; the reservation lifecycle
// (accept/reject/complete) lives in route_lifecycle.go.

func (e *Endpoints) GetTrace(ctx context.Context, req api.GetTraceRequestObject) (api.GetTraceResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetTrace500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	row, err := generated.New(e.deps.OrgDB).GetTrace(ctx, generated.GetTraceParams{ID: pgUUID(uuid.UUID(req.Id)), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.GetTrace404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Error: api.ErrorCodeNotFound, Reason: "trace_not_found"}}, nil
	}
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "get trace", "err", err)
		return api.GetTrace500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "load_failed"}}, nil
	}
	t, mErr := rowToAPITrace(row)
	if mErr != nil {
		return api.GetTrace500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "trace_map_failed"}}, nil
	}
	return api.GetTrace200JSONResponse(t), nil
}
