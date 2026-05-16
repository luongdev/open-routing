// Package server: wave0_temp_stubs.go is a TEMPORARY (Phase 3 Wave 0 only)
// StrictServerInterface implementation that bridges the gap between the
// deleted Phase 2 scaffold composite (`stubs.go` / `NewCompositeServer`, D-77)
// and Wave 2's `catalog.Handlers`.
//
// Scope:
//   - Keeps the four bypass-path methods (GetHealthz, GetReadyz, GetOpenAPISpec,
//     GetDocs) functional so /healthz, /readyz, /openapi.yaml, /docs continue
//     to serve their canonical bodies even between Wave 0 and Wave 2. Without
//     this, deploys at the end of Wave 0 would 501 on the LB liveness probe.
//   - Returns HTTP 500 with `error=internal, reason=not_implemented_yet` for
//     every other StrictServerInterface method. The 500-shaped body flows
//     through RequestIDInjectionMiddleware so request_id is populated.
//
// Lifetime: deleted by Plan 03-08 (Wave 2) when `catalog.Handlers`
// implements StrictServerInterface and the catalog package owns the
// catalog CRUD methods. The bypass-path methods migrate to a tiny
// `infrastructure.Handlers` package at that point.
//
// Why a struct, not the generated `Unimplemented`: the generated
// `Unimplemented` satisfies `ServerInterface` (chi-style), not
// `StrictServerInterface`. Strict-server bodies must return typed
// `*ResponseObject` values, not 501 status codes via w.WriteHeader.
package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// Wave0TempStubs implements api.StrictServerInterface. The four bypass
// methods (GetHealthz, GetReadyz, GetOpenAPISpec, GetDocs) are real; all
// other methods return a typed 500 "not_implemented_yet" response.
//
// Constructed by NewWave0TempStubs(pool, rdb, specBytes). The pool +
// redis are needed by GetReadyz; specBytes is the embedded openapi.yaml.
type Wave0TempStubs struct {
	pool      *pgxpool.Pool
	rdb       *redis.Client
	specBytes []byte
}

// NewWave0TempStubs returns the Wave 0 transitional strict-server impl.
// cmd/api/main.go and test/isolation/main_test.go both call this.
func NewWave0TempStubs(pool *pgxpool.Pool, rdb *redis.Client, specBytes []byte) api.StrictServerInterface {
	return &Wave0TempStubs{
		pool:      pool,
		rdb:       rdb,
		specBytes: specBytes,
	}
}

// notImplemented constructs the canonical 500 "not_implemented_yet" body
// shared by every non-bypass method. The plan acceptance criterion
// (Wave 0 build-fix) only requires this for catalog/state-machine/import
// methods; Wave 2 replaces every call with a real handler.
func notImplemented() api.InternalServerErrorJSONResponse {
	return api.InternalServerErrorJSONResponse{
		Error:  api.ErrorCodeInternal,
		Reason: "not_implemented_yet",
	}
}

// ── Bypass-path methods (real implementations) ───────────────────────────

// GetDocs serves the Scalar API Reference viewer HTML.
func (s *Wave0TempStubs) GetDocs(_ context.Context, _ api.GetDocsRequestObject) (api.GetDocsResponseObject, error) {
	return api.GetDocs200TexthtmlResponse{
		Body:          bytes.NewReader(docsHTML),
		ContentLength: int64(len(docsHTML)),
	}, nil
}

// GetHealthz returns 200 {"status":"alive"} unconditionally.
func (s *Wave0TempStubs) GetHealthz(_ context.Context, _ api.GetHealthzRequestObject) (api.GetHealthzResponseObject, error) {
	return api.GetHealthz200JSONResponse(api.HealthResponse{Status: api.Alive}), nil
}

// GetOpenAPISpec serves the embedded openapi.yaml bytes.
func (s *Wave0TempStubs) GetOpenAPISpec(_ context.Context, _ api.GetOpenAPISpecRequestObject) (api.GetOpenAPISpecResponseObject, error) {
	return api.GetOpenAPISpec200ApplicationyamlResponse{
		Body:          bytes.NewReader(s.specBytes),
		ContentLength: int64(len(s.specBytes)),
	}, nil
}

// GetReadyz pings pgxpool + Redis and reads schema_migrations.
func (s *Wave0TempStubs) GetReadyz(_ context.Context, _ api.GetReadyzRequestObject) (api.GetReadyzResponseObject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dbCheck := api.ReadinessCheckOk
	redisCheck := api.ReadinessCheckOk
	status := api.ReadinessResponseStatusOk
	code := http.StatusOK

	if err := s.pool.Ping(ctx); err != nil {
		dbCheck = api.ReadinessCheckDegraded
		status = api.ReadinessResponseStatusDegraded
		code = http.StatusServiceUnavailable
	}

	if s.rdb != nil {
		if err := s.rdb.Ping(ctx).Err(); err != nil {
			redisCheck = api.ReadinessCheckDegraded
			status = api.ReadinessResponseStatusDegraded
			code = http.StatusServiceUnavailable
		}
	} else {
		redisCheck = api.ReadinessCheckDegraded
		status = api.ReadinessResponseStatusDegraded
		code = http.StatusServiceUnavailable
	}

	var ver int
	var dirty bool
	if err := s.pool.QueryRow(ctx,
		`SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1`,
	).Scan(&ver, &dirty); err != nil {
		slog.Error("readyz: schema_migrations query", "err", err)
		status = api.ReadinessResponseStatusDegraded
		code = http.StatusServiceUnavailable
	} else if dirty {
		status = api.ReadinessResponseStatusDegraded
		code = http.StatusServiceUnavailable
	}

	resp := api.ReadinessResponse{
		Status: status,
		Checks: struct {
			Db         api.ReadinessCheck `json:"db"`
			Migrations struct {
				Version int `json:"version"`
			} `json:"migrations"`
			Redis api.ReadinessCheck `json:"redis"`
		}{
			Db:    dbCheck,
			Redis: redisCheck,
			Migrations: struct {
				Version int `json:"version"`
			}{Version: ver},
		},
	}

	if code == http.StatusServiceUnavailable {
		return api.GetReadyz503JSONResponse(resp), nil
	}
	return api.GetReadyz200JSONResponse(resp), nil
}

// ── 500 "not_implemented_yet" stubs ──────────────────────────────────────
// Each StrictServerInterface method below returns a typed 500 response so
// the RequestIDInjectionMiddleware (B-1, D-35) can inject request_id into
// the body. Wave 2 (Plan 03-08) and Wave 3 (Plans 03-06..03-09) replace
// these with real handlers.

func (s *Wave0TempStubs) ListAdapters(_ context.Context, _ api.ListAdaptersRequestObject) (api.ListAdaptersResponseObject, error) {
	return api.ListAdapters500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) CreateAdapter(_ context.Context, _ api.CreateAdapterRequestObject) (api.CreateAdapterResponseObject, error) {
	return api.CreateAdapter500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) DeleteAdapter(_ context.Context, _ api.DeleteAdapterRequestObject) (api.DeleteAdapterResponseObject, error) {
	return api.DeleteAdapter500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) GetAdapter(_ context.Context, _ api.GetAdapterRequestObject) (api.GetAdapterResponseObject, error) {
	return api.GetAdapter500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) UpdateAdapter(_ context.Context, _ api.UpdateAdapterRequestObject) (api.UpdateAdapterResponseObject, error) {
	return api.UpdateAdapter500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) ListAgents(_ context.Context, _ api.ListAgentsRequestObject) (api.ListAgentsResponseObject, error) {
	return api.ListAgents500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) CreateAgent(_ context.Context, _ api.CreateAgentRequestObject) (api.CreateAgentResponseObject, error) {
	return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) DeleteAgent(_ context.Context, _ api.DeleteAgentRequestObject) (api.DeleteAgentResponseObject, error) {
	return api.DeleteAgent500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) GetAgent(_ context.Context, _ api.GetAgentRequestObject) (api.GetAgentResponseObject, error) {
	return api.GetAgent500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) UpdateAgent(_ context.Context, _ api.UpdateAgentRequestObject) (api.UpdateAgentResponseObject, error) {
	return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) GetAgentStatus(_ context.Context, _ api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error) {
	return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) PatchAgentStatus(_ context.Context, _ api.PatchAgentStatusRequestObject) (api.PatchAgentStatusResponseObject, error) {
	return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) ListBreakReasons(_ context.Context, _ api.ListBreakReasonsRequestObject) (api.ListBreakReasonsResponseObject, error) {
	return api.ListBreakReasons500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) CreateBreakReason(_ context.Context, _ api.CreateBreakReasonRequestObject) (api.CreateBreakReasonResponseObject, error) {
	return api.CreateBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) DeleteBreakReason(_ context.Context, _ api.DeleteBreakReasonRequestObject) (api.DeleteBreakReasonResponseObject, error) {
	return api.DeleteBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) GetBreakReason(_ context.Context, _ api.GetBreakReasonRequestObject) (api.GetBreakReasonResponseObject, error) {
	return api.GetBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) UpdateBreakReason(_ context.Context, _ api.UpdateBreakReasonRequestObject) (api.UpdateBreakReasonResponseObject, error) {
	return api.UpdateBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) BulkImportCatalog(_ context.Context, _ api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
	return api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) ListChannels(_ context.Context, _ api.ListChannelsRequestObject) (api.ListChannelsResponseObject, error) {
	return api.ListChannels500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) CreateChannel(_ context.Context, _ api.CreateChannelRequestObject) (api.CreateChannelResponseObject, error) {
	return api.CreateChannel500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) DeleteChannel(_ context.Context, _ api.DeleteChannelRequestObject) (api.DeleteChannelResponseObject, error) {
	return api.DeleteChannel500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) GetChannel(_ context.Context, _ api.GetChannelRequestObject) (api.GetChannelResponseObject, error) {
	return api.GetChannel500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) UpdateChannel(_ context.Context, _ api.UpdateChannelRequestObject) (api.UpdateChannelResponseObject, error) {
	return api.UpdateChannel500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) GetImportJob(_ context.Context, _ api.GetImportJobRequestObject) (api.GetImportJobResponseObject, error) {
	return api.GetImportJob500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) ListQueues(_ context.Context, _ api.ListQueuesRequestObject) (api.ListQueuesResponseObject, error) {
	return api.ListQueues500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) CreateQueue(_ context.Context, _ api.CreateQueueRequestObject) (api.CreateQueueResponseObject, error) {
	return api.CreateQueue500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) DeleteQueue(_ context.Context, _ api.DeleteQueueRequestObject) (api.DeleteQueueResponseObject, error) {
	return api.DeleteQueue500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) GetQueue(_ context.Context, _ api.GetQueueRequestObject) (api.GetQueueResponseObject, error) {
	return api.GetQueue500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) UpdateQueue(_ context.Context, _ api.UpdateQueueRequestObject) (api.UpdateQueueResponseObject, error) {
	return api.UpdateQueue500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) ListSkills(_ context.Context, _ api.ListSkillsRequestObject) (api.ListSkillsResponseObject, error) {
	return api.ListSkills500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) CreateSkill(_ context.Context, _ api.CreateSkillRequestObject) (api.CreateSkillResponseObject, error) {
	return api.CreateSkill500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) DeleteSkill(_ context.Context, _ api.DeleteSkillRequestObject) (api.DeleteSkillResponseObject, error) {
	return api.DeleteSkill500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) GetSkill(_ context.Context, _ api.GetSkillRequestObject) (api.GetSkillResponseObject, error) {
	return api.GetSkill500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}

func (s *Wave0TempStubs) UpdateSkill(_ context.Context, _ api.UpdateSkillRequestObject) (api.UpdateSkillResponseObject, error) {
	return api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: notImplemented()}, nil
}
