// Package server owns the chi router factory that fronts the open-routing
// API binary. The StrictServerInterface impl lives in
// services/api/internal/catalog (D-69 — catalog.Handlers IS the strict
// server); server.go assembles the LOCKED middleware chain and registers
// the spec routes against that handler via api.HandlerWithOptions.
//
// Route wiring (D-44, Phase 2):
//
//  1. chi stdlib Recoverer  — catches panics from every later middleware.
//  2. Custom UUIDv7 RequestID (D-28) — sets X-Request-Id before the
//     request body is touched.
//  3. Generated strict-server handler (api.HandlerFromMux) registers ALL
//     routes at root level from the OpenAPI spec — including bypass routes
//     (/healthz /readyz /openapi.yaml /docs) and all /v1/* catalog routes.
//  4. /metrics is registered separately (not in the spec, Phase 1 stub).
//  5. OrgContext is applied selectively (D-21) via a conditional middleware
//     inside the generated api.ChiServerOptions.Middlewares slice: the
//     middleware inspects r.URL.Path and calls OrgContext only for /v1/*
//     paths, leaving bypass routes reachable without X-Org-Id.
//  6. RequestIDInjectionMiddleware (B-1, D-35) wraps the strict pipeline
//     for every operation so error response bodies carry request_id from
//     ctx — matching the Phase 1 middleware.WriteError behavior that the
//     strict pipeline bypasses.
//
// After NewMux returns, cmd/api/main.go wraps the returned handler with
// the OTel HTTP root handler (Pattern S6 + Pitfall 5).
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	appmw "github.com/luongdev/open-routing/services/api/internal/middleware"
)

// importBodyLimit mirrors imports.ImportBodyLimit (D5-21 / IMP-07) — the
// 50 MB cap on org-scoped POST bodies. Duplicated here as a local const
// rather than reaching into the imports package because
// `internal/imports` depends on `internal/catalog`, and pulling imports
// into server.go would create a test-time cycle for
// `catalog/testutil_test.go` (package catalog) which already pulls in
// `server`. The literal is single-sourced via this comment + the
// Phase 5 spec lock; if the spec changes both constants flip together.
const importBodyLimit int64 = 50 << 20

// Deps bundles every runtime dependency the API mux needs. cmd/api/main.go
// constructs one of these (after wiring OTel, pool, redis, orgDB, cache,
// catalog) and hands it to NewMux. SpecBytes is the embedded openapi.yaml
// from api.GetSpec() (marshaled to YAML); StrictHandlers is the
// api.StrictServerInterface impl (production: catalog.Handlers from
// internal/catalog per D-69). Tests construct fakes that satisfy the
// interface.
type Deps struct {
	Pool           *pgxpool.Pool
	Redis          *redis.Client
	OrgDB          *db.OrgDB
	Config         *config.Config
	StrictHandlers api.StrictServerInterface // D-44, D-69 — production: catalog.Handlers.
	SpecBytes      []byte                    // D-45: embedded openapi.yaml bytes for /openapi.yaml
}

// NewMux constructs the chi router with the LOCKED middleware chain order.
// The generated api.HandlerFromMux registers all spec routes at the chi
// root level; OrgContext is applied selectively to /v1/* paths via a
// conditional MiddlewareFunc (D-21 bypass-list contract).
func NewMux(deps *Deps) http.Handler {
	r := chi.NewRouter()

	// (1) Recoverer first.
	r.Use(chimw.Recoverer)
	// (2) Our UUIDv7 RequestID — overrides chi's host/N-K counter (D-28).
	//     CRITICAL: must run BEFORE the strict pipeline so the
	//     RequestIDInjectionMiddleware StrictMiddlewareFunc can pull the
	//     id from ctx on the way out.
	r.Use(appmw.RequestID)

	// (3) /metrics: Phase 1 stub — NOT in the generated spec; registered
	//     separately as a bare chi route at root (D-21 bypass list).
	r.Get("/metrics", MetricsHandler())

	// (4) Generated strict-server pipeline. All spec routes (bypass paths
	//     AND /v1/* catalog routes) are registered by api.HandlerFromMux
	//     on the same chi root router, so the Recoverer + RequestID
	//     middleware above wraps every request including bypass probes.
	//
	//     RequestIDInjectionMiddleware (B-1, D-35) wraps the pipeline so
	//     every error response body carries request_id.
	//
	//     orgContextMiddleware applies OrgContext only for /v1/* requests,
	//     leaving /healthz, /readyz, /openapi.yaml, /docs reachable without
	//     X-Org-Id (D-21 bypass-list contract).
	strictPipeline := api.NewStrictHandler(deps.StrictHandlers, []api.StrictMiddlewareFunc{
		RequestIDInjectionMiddleware(),
	})
	// Register all spec routes on the chi root router. The conditional
	// orgContextMiddleware in ChiServerOptions.Middlewares applies OrgContext
	// only for /v1/* paths, leaving bypass routes (/healthz, /readyz,
	// /openapi.yaml, /docs) reachable without X-Org-Id (D-21).
	//
	//     Phase 5 (Plan 05-06): middleware.BodyLimit sits AFTER
	//     orgContextMiddleware per Open Q7 so cheap header-level
	//     rejections (invalid_org_id) short-circuit BEFORE the body
	//     wrap touches r.Body. The wrap remains BEFORE
	//     uuidv7PathParams so a malformed UUID and oversized body
	//     both surface through the same /v1/orgs/ surface in the
	//     order: org gate → size gate → path-shape gate →
	//     strict-server. middleware.BodyLimit is path-scoped to
	//     /v1/orgs/ via the second argument; non-/v1/orgs paths
	//     (e.g., /healthz) pass through untouched.
	api.HandlerWithOptions(strictPipeline, api.ChiServerOptions{
		BaseRouter: r,
		Middlewares: []api.MiddlewareFunc{
			orgContextMiddleware,
			appmw.BodyLimit(importBodyLimit, "/v1/orgs/"),
			uuidv7PathParamsMiddleware,
		},
	})

	return r
}

// orgContextMiddleware is a chi MiddlewareFunc that applies appmw.OrgContext
// only to /v1/* paths. Bypass routes (/healthz, /readyz, /openapi.yaml,
// /docs, /metrics) pass through without the X-Org-Id check.
//
// D-21 enforcement: this is the seam that implements the bypass-list
// contract in the Phase 2 strict-server architecture. The generated
// api.HandlerWithOptions registers all routes on a single chi router; we
// cannot use a Route("/v1", ...) closure because the generated handler
// does not expose a per-path middleware API. The path-prefix check is
// robust because the generated route table is spec-driven: any path that
// does NOT start with /v1/ is a bypass route by construction.
func orgContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			appmw.OrgContext(next).ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// uuidv7PathParamsMiddleware is a chi MiddlewareFunc that wraps
// appmw.UUIDv7PathParams and applies it only to /v1/* paths that have
// matched {id} parameters in the chi route context. Bypass routes and paths
// without {id} params are passed through untouched.
//
// Wired as the second entry in api.ChiServerOptions.Middlewares so it runs
// after orgContextMiddleware (which may short-circuit on missing X-Org-Id
// before we validate path UUIDs — correct order: auth gate first, then
// input validation).
//
// REVIEWS HIGH #3: without this middleware a UUIDv4 {id} path param reaches
// the handler, which queries the DB (no row found for a v4 id in a v7-keyed
// table), and the response is a 404. The 404 is indistinguishable from the
// FOUND-08 cross-org probe disposition, confusing incident response.
func uuidv7PathParamsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			appmw.UUIDv7PathParams(next).ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequestIDInjectionMiddleware is the StrictMiddlewareFunc that closes the
// D-35 gap for the strict-server pipeline (B-1). The strict pipeline
// bypasses middleware.WriteError, so without this wrapper any
// ErrorResponse the handler returns ships with an empty request_id — a
// regression from the Phase 1 envelope contract.
//
// The middleware runs AFTER the handler returns. It inspects the response
// object via a type switch; if it is one of the generated *JSONResponse
// types whose underlying schema is api.ErrorResponse and the RequestId
// field is nil, the middleware sets it to the UUIDv7 pulled from ctx (set
// by appmw.RequestID at the chi root). Successful responses pass through
// untouched.
//
// Exported so scaffold/handler_test.go can mount the same middleware in
// unit tests, proving the B-1 behavior at the package level.
//
// Type-switch based, NOT reflection based (B-1 acceptance check):
// the cases are compile-time checked — if a generated response type is
// renamed or removed, this file stops compiling rather than silently
// dropping request_id injection.
func RequestIDInjectionMiddleware() api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			response, err := next(ctx, w, r, request)
			if err != nil || response == nil {
				return response, err
			}
			id, ok := appmw.RequestIDFromContext(ctx)
			if !ok {
				return response, nil
			}
			return injectRequestIDIntoErrorResponse(response, id.String()), nil
		}
	}
}

// injectRequestIDIntoErrorResponse sets the RequestId field on any
// generated *JSONResponse type whose underlying schema is api.ErrorResponse.
// Phase 3 Wave 0 (Plan 03-01) deleted the Scaffold operation types when
// the openapi.yaml /v1/orgs/{org_id}/_scaffold paths were removed (D-77)
// and added new 409/422 wrappers for CreateChannel/UpdateChannel,
// UpdateAdapter, CreateAgent/UpdateAgent for invalid_reference +
// invalid_value (D-75 + ROADMAP CRIT 4 + Codex C2 iter 3).
//
// The 5 base error response types (BadRequestJSONResponse,
// InternalServerErrorJSONResponse, InvalidOrgIDJSONResponse,
// NotFoundJSONResponse, RequestEntityTooLargeJSONResponse) are direct
// aliases for api.ErrorResponse — their RequestId field can be set
// directly. The operation-specific types embed one of these base types
// or are flat aliases / structs with their own RequestId field.
//
// Implementation note: setting RequestId only when it is nil preserves any
// handler-set value (defensive — Wave 0 stubs intentionally leave
// RequestId nil; the middleware is the single injection point).
func injectRequestIDIntoErrorResponse(response any, id string) any {
	switch r := response.(type) {

	// ── Base types (direct ErrorResponse aliases) ──────────────────────
	case api.BadRequestJSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.BadRequestJSONResponse(v)
	case api.InternalServerErrorJSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.InternalServerErrorJSONResponse(v)
	case api.InvalidOrgIDJSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.InvalidOrgIDJSONResponse(v)
	case api.NotFoundJSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.NotFoundJSONResponse(v)
	case api.RequestEntityTooLargeJSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.RequestEntityTooLargeJSONResponse(v)
	case api.PatchAgentStatus422JSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.PatchAgentStatus422JSONResponse(v)
	case api.CreateChannel409JSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.CreateChannel409JSONResponse(v)
	case api.CreateChannel422JSONResponse:
		// Phase 3 D-75 + Codex C2 iter 3: flat ErrorResponse alias for
		// invalid_reference (default_queue_id FK miss).
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.CreateChannel422JSONResponse(v)
	case api.UpdateChannel422JSONResponse:
		// Phase 3 D-75: flat ErrorResponse alias for invalid_reference
		// (default_queue_id FK miss).
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.UpdateChannel422JSONResponse(v)
	case api.CreateAgent422JSONResponse:
		// Phase 3 Codex C2 iter 3: flat ErrorResponse alias covering BOTH
		// invalid_reference (skills[].skill_id FK miss) AND invalid_value
		// (skills[].proficiency outside 1-10) — the handler picks the
		// ErrorCode on the wrapped body; the wrapper TYPE is the same.
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.CreateAgent422JSONResponse(v)
	case api.CreateAgent409JSONResponse:
		return injectCreateAgent409RequestID(r, id)
	case api.UpdateAgent422JSONResponse:
		// Phase 3 D-75 + ROADMAP CRIT 4: flat ErrorResponse alias covering
		// BOTH invalid_reference (skills[].skill_id FK miss) AND invalid_value
		// (skills[].proficiency outside 1-10) — same union semantics as
		// CreateAgent422JSONResponse. Wave 3 (Plan 03-09) implements both
		// error paths in the catalog package.
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.UpdateAgent422JSONResponse(v)
	case api.UpdateChannel409JSONResponse:
		// Phase 3 OQ-1/A4: VersionConflict variant — has its own RequestId
		// field. Pattern matches UpdateAgent409 / UpdateQueue409.
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateAdapter409JSONResponse:
		// Phase 3 OQ-1/A4: VersionConflict variant — has its own RequestId
		// field. Pattern matches UpdateChannel409 / UpdateAgent409.
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateQueue409JSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.CreateQueue409JSONResponse(v)
	case api.CreateSkill409JSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.CreateSkill409JSONResponse(v)
	case api.CreateBreakReason409JSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.CreateBreakReason409JSONResponse(v)
	case api.BulkImportCatalog400JSONResponse:
		v := api.ErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.BulkImportCatalog400JSONResponse(v)

	// ── Adapter 500-stubs ─────────────────────────────────────────────
	case api.ListAdapters500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateAdapter500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteAdapter500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetAdapter500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateAdapter500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── Agent 500-stubs ────────────────────────────────────────────────
	case api.ListAgents500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateAgent500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteAgent500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetAgent500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateAgent500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetAgentStatus500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.PatchAgentStatus500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── BreakReason 500-stubs ─────────────────────────────────────────
	case api.ListBreakReasons500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateBreakReason500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteBreakReason500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetBreakReason500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateBreakReason500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── BulkImport 500-stub ───────────────────────────────────────────
	case api.BulkImportCatalog500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── Channel 500-stubs ─────────────────────────────────────────────
	case api.ListChannels500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateChannel500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteChannel500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetChannel500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateChannel500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── ImportJob 500-stub ────────────────────────────────────────────
	case api.GetImportJob500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── Queue 500-stubs ───────────────────────────────────────────────
	case api.ListQueues500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateQueue500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteQueue500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetQueue500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateQueue500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── Skill 500-stubs ───────────────────────────────────────────────
	case api.ListSkills500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateSkill500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteSkill500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetSkill500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateSkill500JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ─────────────────────────────────────────────────────────────────────────
	// Phase 3 real error paths — REVIEWS HIGH #2.
	// The original Phase 2 switch covered only 500-stubs. Phase 3 handlers will
	// return 400/404/409 types that were generated but missing from the switch.
	// Without coverage here, those error responses silently omit request_id.
	// ─────────────────────────────────────────────────────────────────────────

	// ── Adapter 400/404 ───────────────────────────────────────────────────
	case api.ListAdapters400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateAdapter400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteAdapter400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteAdapter404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetAdapter400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetAdapter404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateAdapter400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateAdapter404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── Agent 400/404/409 ─────────────────────────────────────────────────
	case api.ListAgents400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateAgent400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteAgent400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteAgent404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetAgent400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetAgent404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateAgent400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateAgent404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateAgent409JSONResponse:
		// VersionConflictErrorResponse variant — has its own RequestId field.
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── AgentStatus 400/404/409 ───────────────────────────────────────────
	case api.GetAgentStatus400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetAgentStatus404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.PatchAgentStatus400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.PatchAgentStatus404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.PatchAgentStatus409JSONResponse:
		// InvalidTransitionErrorResponse — has its own RequestId field.
		v := api.InvalidTransitionErrorResponse(r)
		v.RequestId = setIfNil(v.RequestId, id)
		return api.PatchAgentStatus409JSONResponse(v)

	// ── BreakReason 400/404/409 ───────────────────────────────────────────
	case api.ListBreakReasons400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateBreakReason400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteBreakReason400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteBreakReason404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetBreakReason400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetBreakReason404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateBreakReason400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateBreakReason404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateBreakReason409JSONResponse:
		// VersionConflictErrorResponse variant — has its own RequestId field.
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── BulkImport 413 ────────────────────────────────────────────────────
	case api.BulkImportCatalog413JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	// BulkImportCatalog422JSONResponse is BulkImportResult (no ErrorResponse/RequestId) — pass through.
	// GetReadyz503JSONResponse is ReadinessResponse (no RequestId) — pass through.

	// ── Channel 400/404 ───────────────────────────────────────────────────
	case api.ListChannels400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateChannel400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteChannel400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteChannel404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetChannel400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetChannel404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateChannel400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateChannel404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── ImportJob 400/404 ─────────────────────────────────────────────────
	case api.GetImportJob400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetImportJob404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── Queue 400/404/409 ─────────────────────────────────────────────────
	case api.ListQueues400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateQueue400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteQueue400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteQueue404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetQueue400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetQueue404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateQueue400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateQueue404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateQueue409JSONResponse:
		// VersionConflictErrorResponse variant — has its own RequestId field.
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	// ── Skill 400/404/409 ─────────────────────────────────────────────────
	case api.ListSkills400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.CreateSkill400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteSkill400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.DeleteSkill404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetSkill400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.GetSkill404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateSkill400JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateSkill404JSONResponse:
		r.RequestId = setIfNil(r.RequestId, id)
		return r
	case api.UpdateSkill409JSONResponse:
		// VersionConflictErrorResponse variant — has its own RequestId field.
		r.RequestId = setIfNil(r.RequestId, id)
		return r

	default:
		// Successful responses (200/201/204) and non-ErrorResponse shapes
		// pass through untouched.
		return response
	}
}

func injectCreateAgent409RequestID(r api.CreateAgent409JSONResponse, id string) api.CreateAgent409JSONResponse {
	raw, err := r.MarshalJSON()
	if err != nil {
		return r
	}

	var probe struct {
		Current json.RawMessage `json:"current"`
	}
	if err := json.Unmarshal(raw, &probe); err == nil && len(probe.Current) > 0 && string(probe.Current) != "null" {
		v, err := r.AsVersionConflictErrorResponse()
		if err != nil {
			return r
		}
		v.RequestId = setIfNil(v.RequestId, id)
		var out api.CreateAgent409JSONResponseBody
		_ = out.FromVersionConflictErrorResponse(v)
		return api.CreateAgent409JSONResponse(out)
	}

	v, err := r.AsErrorResponse()
	if err != nil {
		return r
	}
	v.RequestId = setIfNil(v.RequestId, id)
	var out api.CreateAgent409JSONResponseBody
	_ = out.FromErrorResponse(v)
	return api.CreateAgent409JSONResponse(out)
}

// setIfNil returns a pointer to id if current is nil, otherwise returns current.
// This preserves any handler-set value; the middleware is the fallback injection
// point, not the override point.
//
// api.UUIDv7 is a type alias for openapi_types.UUID which is uuid.UUID, so we
// use uuid.Parse to convert the string id.
func setIfNil(current *api.UUIDv7, id string) *api.UUIDv7 {
	if current != nil {
		return current
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil
	}
	v := api.UUIDv7(parsed)
	return &v
}
