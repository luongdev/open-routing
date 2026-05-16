package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// noopStrictStub returns 500 not_implemented for every catalog method and
// serves the minimal Healthz response. Used here so the mux-routing tests
// stay self-contained (no testcontainers, no cache, no DB) — what they
// assert is middleware chain behaviour, not handler semantics. The real
// integration tests live in test/isolation against the production
// catalog.Handlers wiring.
type noopStrictStub struct{}

func (noopStrictStub) GetHealthz(context.Context, api.GetHealthzRequestObject) (api.GetHealthzResponseObject, error) {
	return api.GetHealthz200JSONResponse(api.HealthResponse{Status: api.Alive}), nil
}

func notImpl() api.InternalServerErrorJSONResponse {
	return api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "not_implemented_in_test_stub"}
}

func (noopStrictStub) GetDocs(context.Context, api.GetDocsRequestObject) (api.GetDocsResponseObject, error) {
	return api.GetDocs200TexthtmlResponse{}, nil
}
func (noopStrictStub) GetOpenAPISpec(context.Context, api.GetOpenAPISpecRequestObject) (api.GetOpenAPISpecResponseObject, error) {
	return api.GetOpenAPISpec200ApplicationyamlResponse{}, nil
}
func (noopStrictStub) GetReadyz(context.Context, api.GetReadyzRequestObject) (api.GetReadyzResponseObject, error) {
	return api.GetReadyz200JSONResponse(api.ReadinessResponse{Status: api.ReadinessResponseStatusOk}), nil
}
func (noopStrictStub) ListAdapters(context.Context, api.ListAdaptersRequestObject) (api.ListAdaptersResponseObject, error) {
	return api.ListAdapters500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) CreateAdapter(context.Context, api.CreateAdapterRequestObject) (api.CreateAdapterResponseObject, error) {
	return api.CreateAdapter500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) DeleteAdapter(context.Context, api.DeleteAdapterRequestObject) (api.DeleteAdapterResponseObject, error) {
	return api.DeleteAdapter500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) GetAdapter(context.Context, api.GetAdapterRequestObject) (api.GetAdapterResponseObject, error) {
	return api.GetAdapter500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) UpdateAdapter(context.Context, api.UpdateAdapterRequestObject) (api.UpdateAdapterResponseObject, error) {
	return api.UpdateAdapter500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) ListAgents(context.Context, api.ListAgentsRequestObject) (api.ListAgentsResponseObject, error) {
	return api.ListAgents500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) CreateAgent(context.Context, api.CreateAgentRequestObject) (api.CreateAgentResponseObject, error) {
	return api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) DeleteAgent(context.Context, api.DeleteAgentRequestObject) (api.DeleteAgentResponseObject, error) {
	return api.DeleteAgent500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) GetAgent(context.Context, api.GetAgentRequestObject) (api.GetAgentResponseObject, error) {
	return api.GetAgent500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) UpdateAgent(context.Context, api.UpdateAgentRequestObject) (api.UpdateAgentResponseObject, error) {
	return api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) GetAgentStatus(context.Context, api.GetAgentStatusRequestObject) (api.GetAgentStatusResponseObject, error) {
	return api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) PatchAgentStatus(context.Context, api.PatchAgentStatusRequestObject) (api.PatchAgentStatusResponseObject, error) {
	return api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) ListBreakReasons(context.Context, api.ListBreakReasonsRequestObject) (api.ListBreakReasonsResponseObject, error) {
	return api.ListBreakReasons500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) CreateBreakReason(context.Context, api.CreateBreakReasonRequestObject) (api.CreateBreakReasonResponseObject, error) {
	return api.CreateBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) DeleteBreakReason(context.Context, api.DeleteBreakReasonRequestObject) (api.DeleteBreakReasonResponseObject, error) {
	return api.DeleteBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) GetBreakReason(context.Context, api.GetBreakReasonRequestObject) (api.GetBreakReasonResponseObject, error) {
	return api.GetBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) UpdateBreakReason(context.Context, api.UpdateBreakReasonRequestObject) (api.UpdateBreakReasonResponseObject, error) {
	return api.UpdateBreakReason500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) BulkImportCatalog(context.Context, api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
	return api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) ListChannels(context.Context, api.ListChannelsRequestObject) (api.ListChannelsResponseObject, error) {
	return api.ListChannels500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) CreateChannel(context.Context, api.CreateChannelRequestObject) (api.CreateChannelResponseObject, error) {
	return api.CreateChannel500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) DeleteChannel(context.Context, api.DeleteChannelRequestObject) (api.DeleteChannelResponseObject, error) {
	return api.DeleteChannel500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) GetChannel(context.Context, api.GetChannelRequestObject) (api.GetChannelResponseObject, error) {
	return api.GetChannel500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) UpdateChannel(context.Context, api.UpdateChannelRequestObject) (api.UpdateChannelResponseObject, error) {
	return api.UpdateChannel500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) GetImportJob(context.Context, api.GetImportJobRequestObject) (api.GetImportJobResponseObject, error) {
	return api.GetImportJob500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) ListQueues(context.Context, api.ListQueuesRequestObject) (api.ListQueuesResponseObject, error) {
	return api.ListQueues500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) CreateQueue(context.Context, api.CreateQueueRequestObject) (api.CreateQueueResponseObject, error) {
	return api.CreateQueue500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) DeleteQueue(context.Context, api.DeleteQueueRequestObject) (api.DeleteQueueResponseObject, error) {
	return api.DeleteQueue500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) GetQueue(context.Context, api.GetQueueRequestObject) (api.GetQueueResponseObject, error) {
	return api.GetQueue500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) UpdateQueue(context.Context, api.UpdateQueueRequestObject) (api.UpdateQueueResponseObject, error) {
	return api.UpdateQueue500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) ListSkills(context.Context, api.ListSkillsRequestObject) (api.ListSkillsResponseObject, error) {
	return api.ListSkills500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) CreateSkill(context.Context, api.CreateSkillRequestObject) (api.CreateSkillResponseObject, error) {
	return api.CreateSkill500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) DeleteSkill(context.Context, api.DeleteSkillRequestObject) (api.DeleteSkillResponseObject, error) {
	return api.DeleteSkill500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) GetSkill(context.Context, api.GetSkillRequestObject) (api.GetSkillResponseObject, error) {
	return api.GetSkill500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}
func (noopStrictStub) UpdateSkill(context.Context, api.UpdateSkillRequestObject) (api.UpdateSkillResponseObject, error) {
	return api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: notImpl()}, nil
}

var _ api.StrictServerInterface = noopStrictStub{}

// TestNewMux_HealthzAccessibleWithoutOrgHeader pins the LOCKED middleware
// chain in NewMux: /healthz must reach the GetHealthz handler with no
// X-Org-Id header — D-21 bypass-list contract. A regression that pushed
// /healthz under OrgContext would return 400 here and fail.
func TestNewMux_HealthzAccessibleWithoutOrgHeader(t *testing.T) {
	t.Parallel()
	deps := &Deps{StrictHandlers: noopStrictStub{}}
	mux := NewMux(deps)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "healthz must succeed without X-Org-Id (D-21)")
	require.Contains(t, rec.Body.String(), `"status":"alive"`, "healthz body shape (D-17)")
	require.NotEmpty(t, rec.Header().Get("X-Request-Id"), "RequestID middleware must populate X-Request-Id (D-28)")
}

// TestNewMux_MetricsAccessibleWithoutOrgHeader covers the /metrics bypass
// path with the same harness — /metrics is mounted as a bare chi route
// outside the strict-server pipeline, so it must still bypass OrgContext.
func TestNewMux_MetricsAccessibleWithoutOrgHeader(t *testing.T) {
	t.Parallel()
	deps := &Deps{StrictHandlers: noopStrictStub{}}
	mux := NewMux(deps)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

// TestNewMux_V1RequiresOrgHeader proves /v1/* routes pass through the
// orgContextMiddleware gate — missing X-Org-Id returns 400 invalid_org_id
// / missing_header before reaching the strict server. Spoofing mitigation
// for T-1-01.
func TestNewMux_V1RequiresOrgHeader(t *testing.T) {
	t.Parallel()
	deps := &Deps{StrictHandlers: noopStrictStub{}}
	mux := NewMux(deps)

	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0e/agents", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, "v1 routes must require X-Org-Id")
	require.Contains(t, rec.Body.String(), `"error":"invalid_org_id"`, "rejection code must be invalid_org_id")
	require.Contains(t, rec.Body.String(), `"reason":"missing_header"`, "rejection reason must be missing_header")
}
