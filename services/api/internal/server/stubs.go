// Package server: stubs.go holds the compositeServer that satisfies
// api.StrictServerInterface by delegating Scaffold methods to
// scaffold.Handler, delegating bypass-path methods (GetHealthz, GetReadyz,
// GetOpenAPISpec, GetDocs) to inline implementations, and returning
// HTTP 500 with reason "not_implemented" for every other operation.
//
// Phase 3 replaces the 501-stub implementations with real handlers as
// catalog endpoints are implemented. The bypass-path methods are permanent
// (GetDocs and GetOpenAPISpec are delegated to the openapi.go helpers;
// GetHealthz and GetReadyz use the health.go logic).
//
// The 500-stub style is deliberately encoded as:
//
//	<Op>500JSONResponse{InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal,
//	    Reason: "not_implemented"}}
//
// so server.RequestIDInjectionMiddleware (B-1, D-35) populates request_id
// in the response body — proving the gap is closed for every non-Scaffold
// operation today, not just the three migrated paths.
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
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/scaffold"
)

// compositeServer implements api.StrictServerInterface. Scaffold methods
// are promoted from scaffold.Handler. Bypass-path methods delegate to the
// helpers in health.go and openapi.go. All catalog/import/agent-state
// methods return a typed 500 "not_implemented" response so the strict
// pipeline's RequestIDInjectionMiddleware can inject request_id even for
// unimplemented routes.
type compositeServer struct {
	*scaffold.Handler
	pool     *pgxpool.Pool
	rdb      *redis.Client
	specBytes []byte
}

// NewCompositeServer wires the scaffold handler and health deps.
// Phase 3 will add fields for each entity's handler and replace the 501
// stubs with real methods.
func NewCompositeServer(orgDB *db.OrgDB, pool *pgxpool.Pool, rdb *redis.Client, specBytes []byte) api.StrictServerInterface {
	return &compositeServer{
		Handler:   scaffold.NewHandler(orgDB),
		pool:      pool,
		rdb:       rdb,
		specBytes: specBytes,
	}
}

// GetDocs implements the GET /docs bypass route (D-45). Returns the Scalar
// API Reference viewer HTML embedded via openapi.go's //go:embed directive.
func (s *compositeServer) GetDocs(_ context.Context, _ api.GetDocsRequestObject) (api.GetDocsResponseObject, error) {
	return api.GetDocs200TexthtmlResponse{
		Body:          bytes.NewReader(docsHTML),
		ContentLength: int64(len(docsHTML)),
	}, nil
}

// GetHealthz implements the GET /healthz liveness probe (D-17, D-21).
// Returns 200 {"status":"alive"} unconditionally — same contract as the
// LiveHandler() used in Phase 1.
func (s *compositeServer) GetHealthz(_ context.Context, _ api.GetHealthzRequestObject) (api.GetHealthzResponseObject, error) {
	return api.GetHealthz200JSONResponse(api.HealthResponse{
		Status: api.Alive,
	}), nil
}

// GetOpenAPISpec implements the GET /openapi.yaml bypass route (D-45).
// Returns the embedded spec bytes served from the same binary that handles
// requests — no stale-spec risk.
func (s *compositeServer) GetOpenAPISpec(_ context.Context, _ api.GetOpenAPISpecRequestObject) (api.GetOpenAPISpecResponseObject, error) {
	return api.GetOpenAPISpec200ApplicationyamlResponse{
		Body:          bytes.NewReader(s.specBytes),
		ContentLength: int64(len(s.specBytes)),
	}, nil
}

// GetReadyz implements the GET /readyz readiness probe (D-17, D-21). Pings
// pgxpool, Redis, and reads the latest schema_migrations row. Returns 200
// when all checks pass, 503 when any check fails.
func (s *compositeServer) GetReadyz(_ context.Context, _ api.GetReadyzRequestObject) (api.GetReadyzResponseObject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dbCheck := api.ReadinessCheckOk
	redisCheck := api.ReadinessCheckOk
	status := api.ReadinessResponseStatusOk
	code := http.StatusOK

	// (1) Postgres ping.
	if err := s.pool.Ping(ctx); err != nil {
		dbCheck = api.ReadinessCheckDegraded
		status = api.ReadinessResponseStatusDegraded
		code = http.StatusServiceUnavailable
	}

	// (2) Redis ping.
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

	// (3) Latest schema_migrations row.
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

// ---- 501 stubs for catalog/import/agent-state operations ----
// Phase 3 replaces these with real implementations.

func (s *compositeServer) ListAdapters(_ context.Context, _ api.ListAdaptersRequestObject) (api.ListAdaptersResponseObject, error) {
	return api.ListAdapters500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) CreateAdapter(_ context.Context, _ api.CreateAdapterRequestObject) (api.CreateAdapterResponseObject, error) {
	return api.CreateAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) DeleteAdapter(_ context.Context, _ api.DeleteAdapterRequestObject) (api.DeleteAdapterResponseObject, error) {
	return api.DeleteAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) GetAdapter(_ context.Context, _ api.GetAdapterRequestObject) (api.GetAdapterResponseObject, error) {
	return api.GetAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) UpdateAdapter(_ context.Context, _ api.UpdateAdapterRequestObject) (api.UpdateAdapterResponseObject, error) {
	return api.UpdateAdapter500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) ListAgents(_ context.Context, _ api.ListAgentsRequestObject) (api.ListAgentsResponseObject, error) {
	return api.ListAgents500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) CreateAgent(_ context.Context, _ api.CreateAgentRequestObject) (api.CreateAgentResponseObject, error) {
	return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) DeleteAgent(_ context.Context, _ api.DeleteAgentRequestObject) (api.DeleteAgentResponseObject, error) {
	return api.DeleteAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) GetAgent(_ context.Context, _ api.GetAgentRequestObject) (api.GetAgentResponseObject, error) {
	return api.GetAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) UpdateAgent(_ context.Context, _ api.UpdateAgentRequestObject) (api.UpdateAgentResponseObject, error) {
	return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) GetAgentStatus(_ context.Context, _ api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error) {
	return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) PatchAgentStatus(_ context.Context, _ api.PatchAgentStatusRequestObject) (api.PatchAgentStatusResponseObject, error) {
	return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) ListBreakReasons(_ context.Context, _ api.ListBreakReasonsRequestObject) (api.ListBreakReasonsResponseObject, error) {
	return api.ListBreakReasons500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) CreateBreakReason(_ context.Context, _ api.CreateBreakReasonRequestObject) (api.CreateBreakReasonResponseObject, error) {
	return api.CreateBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) DeleteBreakReason(_ context.Context, _ api.DeleteBreakReasonRequestObject) (api.DeleteBreakReasonResponseObject, error) {
	return api.DeleteBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) GetBreakReason(_ context.Context, _ api.GetBreakReasonRequestObject) (api.GetBreakReasonResponseObject, error) {
	return api.GetBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) UpdateBreakReason(_ context.Context, _ api.UpdateBreakReasonRequestObject) (api.UpdateBreakReasonResponseObject, error) {
	return api.UpdateBreakReason500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) BulkImportCatalog(_ context.Context, _ api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
	return api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) ListChannels(_ context.Context, _ api.ListChannelsRequestObject) (api.ListChannelsResponseObject, error) {
	return api.ListChannels500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) CreateChannel(_ context.Context, _ api.CreateChannelRequestObject) (api.CreateChannelResponseObject, error) {
	return api.CreateChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) DeleteChannel(_ context.Context, _ api.DeleteChannelRequestObject) (api.DeleteChannelResponseObject, error) {
	return api.DeleteChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) GetChannel(_ context.Context, _ api.GetChannelRequestObject) (api.GetChannelResponseObject, error) {
	return api.GetChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) UpdateChannel(_ context.Context, _ api.UpdateChannelRequestObject) (api.UpdateChannelResponseObject, error) {
	return api.UpdateChannel500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) GetImportJob(_ context.Context, _ api.GetImportJobRequestObject) (api.GetImportJobResponseObject, error) {
	return api.GetImportJob500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) ListQueues(_ context.Context, _ api.ListQueuesRequestObject) (api.ListQueuesResponseObject, error) {
	return api.ListQueues500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) CreateQueue(_ context.Context, _ api.CreateQueueRequestObject) (api.CreateQueueResponseObject, error) {
	return api.CreateQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) DeleteQueue(_ context.Context, _ api.DeleteQueueRequestObject) (api.DeleteQueueResponseObject, error) {
	return api.DeleteQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) GetQueue(_ context.Context, _ api.GetQueueRequestObject) (api.GetQueueResponseObject, error) {
	return api.GetQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) UpdateQueue(_ context.Context, _ api.UpdateQueueRequestObject) (api.UpdateQueueResponseObject, error) {
	return api.UpdateQueue500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) ListSkills(_ context.Context, _ api.ListSkillsRequestObject) (api.ListSkillsResponseObject, error) {
	return api.ListSkills500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) CreateSkill(_ context.Context, _ api.CreateSkillRequestObject) (api.CreateSkillResponseObject, error) {
	return api.CreateSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) DeleteSkill(_ context.Context, _ api.DeleteSkillRequestObject) (api.DeleteSkillResponseObject, error) {
	return api.DeleteSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) GetSkill(_ context.Context, _ api.GetSkillRequestObject) (api.GetSkillResponseObject, error) {
	return api.GetSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}

func (s *compositeServer) UpdateSkill(_ context.Context, _ api.UpdateSkillRequestObject) (api.UpdateSkillResponseObject, error) {
	return api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented"}}, nil
}
