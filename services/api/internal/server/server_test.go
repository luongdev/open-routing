package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

// ─────────────────────────────────────────────────────────────────────
// Phase 5 (Plan 05-06) — BodyLimit middleware chain-ordering tests.
//
// The plan locks the order via Open Q7: RequestID → orgContextMiddleware
// → BodyLimit → uuidv7PathParamsMiddleware → strict-server. The three
// tests below pin distinct halves of that contract:
//
//   - oversize body to /v1/orgs/ → 413 from the middleware (request
//     never reaches the strict handler).
//   - under-limit body to /v1/orgs/ → reaches the strict handler. For
//     the noopStrictStub that means 500 not_implemented — critically
//     NOT 413.
//   - oversize body to /healthz → middleware is path-scoped to
//     /v1/orgs/; healthz still responds normally (200 alive).
// ─────────────────────────────────────────────────────────────────────

// validOrgID is a fixed UUIDv7 used by the BodyLimit chain tests so the
// orgContextMiddleware accepts the request and we can verify what the
// BodyLimit middleware does AFTER the org gate succeeds.
const validOrgID = "01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0e"

// oversizeBody builds a synthetic body slightly larger than the locked
// 50 MB cap. The middleware reads r.ContentLength (the parsed long, not
// the header) so we use httptest.NewRequest's auto-detected
// Content-Length from the bytes.Reader length — well above 50<<20. We
// reference the package-internal importBodyLimit (server.go) so any
// future cap revision flips both the production wire and this test
// together.
func oversizeBody() []byte {
	return bytes.Repeat([]byte("A"), int(importBodyLimit)+1024)
}

// TestServerMuxChain_BodyLimit_BlocksOversizeBody — POST > 50 MB body to
// the bulk-import path; middleware returns 413 with the canonical
// "request_too_large_use_async_pathway" reason BEFORE the strict-server
// pipeline runs.
func TestServerMuxChain_BodyLimit_BlocksOversizeBody(t *testing.T) {
	t.Parallel()
	deps := &Deps{StrictHandlers: noopStrictStub{}}
	mux := NewMux(deps)

	body := oversizeBody()
	req := httptest.NewRequest(http.MethodPost,
		"/v1/orgs/"+validOrgID+"/catalog/import?entity=agents",
		bytes.NewReader(body))
	req.Header.Set("X-Org-Id", validOrgID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code,
		"oversize body must trigger 413 via BodyLimit middleware")
	require.Contains(t, rec.Body.String(), `"error":"invalid_body"`,
		"middleware error code is invalid_body")
	require.Contains(t, rec.Body.String(), "request_too_large_use_async_pathway",
		"middleware reason points to IMP-09 async pathway")
}

// TestServerMuxChain_BodyLimit_PassesUnderLimit — POST a small body to
// the same path; middleware lets it through; the noopStrictStub returns
// 500 not_implemented_in_test_stub. The critical assertion is the
// response is NOT 413 — under-limit bodies make it past BodyLimit.
func TestServerMuxChain_BodyLimit_PassesUnderLimit(t *testing.T) {
	t.Parallel()
	deps := &Deps{StrictHandlers: noopStrictStub{}}
	mux := NewMux(deps)

	// 1 KB body — well under 50 MB.
	body := bytes.Repeat([]byte("a"), 1024)
	req := httptest.NewRequest(http.MethodPost,
		"/v1/orgs/"+validOrgID+"/catalog/import?entity=agents",
		bytes.NewReader(body))
	req.Header.Set("X-Org-Id", validOrgID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.NotEqual(t, http.StatusRequestEntityTooLarge, rec.Code,
		"under-limit body MUST NOT trigger 413 (got %d)", rec.Code)
	// The noopStrictStub's BulkImportCatalog returns 500 not_implemented
	// for the test fixture. The middleware passed it through; that is
	// what we are asserting.
	require.True(t,
		rec.Code == http.StatusInternalServerError ||
			rec.Code == http.StatusBadRequest ||
			rec.Code == http.StatusOK,
		"under-limit body must reach the strict handler (got %d, body=%q)",
		rec.Code, rec.Body.String())
}

// TestServerMuxChain_BodyLimit_BypassesHealthz — POST an oversize body to
// /healthz; the middleware is path-scoped to /v1/orgs/ so it MUST NOT
// gate /healthz. Asserts the path-prefix scope is honoured. (The bypass
// route may return 200, 404, or 405 depending on the route registration;
// the assertion is "not 413".)
func TestServerMuxChain_BodyLimit_BypassesHealthz(t *testing.T) {
	t.Parallel()
	deps := &Deps{StrictHandlers: noopStrictStub{}}
	mux := NewMux(deps)

	body := oversizeBody()
	req := httptest.NewRequest(http.MethodPost, "/healthz",
		bytes.NewReader(body))
	// No X-Org-Id needed for /healthz (D-21 bypass-list).
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.NotEqual(t, http.StatusRequestEntityTooLarge, rec.Code,
		"healthz must NOT be gated by the /v1/orgs/-scoped BodyLimit "+
			"middleware (got %d, body=%q)",
		rec.Code, rec.Body.String())
	// Defensive: the response must NOT contain the BodyLimit middleware's
	// canonical reason string either, in case some future test framework
	// returns 200 with an error envelope.
	require.False(t,
		strings.Contains(rec.Body.String(), "request_too_large_use_async_pathway"),
		"healthz response must not carry the BodyLimit 413 reason")
}
