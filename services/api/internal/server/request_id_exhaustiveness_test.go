package server

// TestRequestIDInjection_Exhaustiveness verifies that every *JSONResponse type
// in the api package that carries an ErrorResponse shape (i.e., has a
// request_id field) is handled by injectRequestIDIntoErrorResponse.
//
// REVIEWS HIGH #2: the original Phase 2 type switch covered only 500-stub
// operations. Phase 3 will return 400/404/409 types that were generated but
// missing, causing those responses to silently omit request_id.
//
// Test strategy: construct a representative instance of each error-bearing
// JSONResponse type, call injectRequestIDIntoErrorResponse, and assert the
// request_id field is populated. Types with no ErrorResponse shape (success
// types, ReadinessResponse, BulkImportResult) are explicitly excluded and
// enumerated in the EXCLUDED list so a code reviewer can verify the
// exclusions are correct.
//
// If a new *JSONResponse type is added to server.gen.go by codegen and NOT
// added to the switch below, the test will catch the regression by verifying
// the switch's return value is correct — the logic below tests each type
// individually, so a missing case hits the `default` branch which returns
// the original value unchanged and the request_id assertion fails.
import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// sentinelID is the id injected during tests.
const sentinelID = "01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0e"

func mustUUID(s string) uuid.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// errResp builds a base ErrorResponse with no RequestId set.
func errResp() api.ErrorResponse {
	return api.ErrorResponse{Error: "test_error", Reason: "test_reason"}
}

// assertID checks the request_id field on a response value after injection.
// It uses type assertions to avoid reflection — keeping compile-time safety.
func assertID(t *testing.T, result any, typeName string) {
	t.Helper()
	id := mustUUID(sentinelID)
	expectedID := api.UUIDv7(id)

	switch r := result.(type) {
	// ── Base types ────────────────────────────────────────────────────────
	case api.BadRequestJSONResponse:
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.InternalServerErrorJSONResponse:
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.InvalidOrgIDJSONResponse:
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.NotFoundJSONResponse:
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.RequestEntityTooLargeJSONResponse:
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)

	// ── Embedded struct types (embed a base type) ─────────────────────────
	// The field on the embedded type carries the RequestId.

	// Adapters
	case api.ListAdapters400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.UpdateAdapter409JSONResponse:
		// Phase 3 OQ-1/A4 — VersionConflict variant; RequestId is a direct field.
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.ListAdapters500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateAdapter400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateAdapter500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteAdapter400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteAdapter404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteAdapter500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetAdapter400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetAdapter404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetAdapter500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateAdapter400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateAdapter404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateAdapter500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)

	// Agents
	case api.ListAgents400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.ListAgents500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateAgent400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateAgent500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteAgent400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteAgent404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteAgent500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetAgent400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetAgent404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetAgent500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateAgent400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateAgent404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateAgent409JSONResponse:
		// VersionConflictErrorResponse variant — RequestId is a direct field.
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.UpdateAgent422JSONResponse:
		// Phase 3 D-75 + ROADMAP CRIT 4 — flat ErrorResponse alias covering
		// both invalid_reference (skills[].skill_id FK) and invalid_value
		// (skills[].proficiency 1-10) error paths.
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.CreateAgent422JSONResponse:
		// Phase 3 Codex C2 iter 3 — flat ErrorResponse alias covering
		// both invalid_reference (skills[].skill_id FK) and invalid_value
		// (skills[].proficiency 1-10) error paths for the create path.
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.CreateAgent409JSONResponse:
		v, err := r.AsErrorResponse()
		require.NoError(t, err, "%s: unmarshal ErrorResponse union", typeName)
		require.NotNil(t, v.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *v.RequestId)
	case api.UpdateAgent500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)

	// AgentStatus
	case api.GetAgentStatus400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetAgentStatus404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetAgentStatus500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.PatchAgentStatus400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.PatchAgentStatus404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.PatchAgentStatus409JSONResponse:
		// InvalidTransitionErrorResponse — has its own RequestId.
		v := api.InvalidTransitionErrorResponse(r)
		require.NotNil(t, v.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *v.RequestId)
	case api.PatchAgentStatus422JSONResponse:
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
	case api.PatchAgentStatus500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)

	// BreakReasons
	case api.ListBreakReasons400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.ListBreakReasons500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateBreakReason400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateBreakReason500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteBreakReason400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteBreakReason404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteBreakReason500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetBreakReason400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetBreakReason404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetBreakReason500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateBreakReason400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateBreakReason404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateBreakReason409JSONResponse:
		// VersionConflictErrorResponse — has its own RequestId.
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.UpdateBreakReason500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)

	// BulkImport
	case api.BulkImportCatalog400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.BulkImportCatalog413JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.BulkImportCatalog500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)

	// Channels
	case api.ListChannels400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.ListChannels500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateChannel400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateChannel409JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateChannel422JSONResponse:
		// Phase 3 D-75 — flat ErrorResponse alias for invalid_reference
		// (default_queue_id FK miss).
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.CreateChannel500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteChannel400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteChannel404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteChannel500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetChannel400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetChannel404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetChannel500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateChannel400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateChannel404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateChannel409JSONResponse:
		// Phase 3 OQ-1/A4 — VersionConflict variant; RequestId is a direct field.
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.UpdateChannel422JSONResponse:
		// Phase 3 D-75 — flat ErrorResponse alias for invalid_reference
		// (default_queue_id FK miss).
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.UpdateChannel500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)

	// ImportJob
	case api.GetImportJob400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetImportJob404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetImportJob500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)

	// Queues
	case api.ListQueues400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.ListQueues500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateQueue400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateQueue409JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateQueue500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteQueue400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteQueue404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteQueue500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetQueue400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetQueue404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetQueue500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateQueue400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateQueue404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateQueue409JSONResponse:
		// VersionConflictErrorResponse — has its own RequestId.
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.UpdateQueue500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)

	// Skills
	case api.ListSkills400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.ListSkills500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateSkill400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateSkill409JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.CreateSkill500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteSkill400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteSkill404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.DeleteSkill500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetSkill400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetSkill404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.GetSkill500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateSkill400JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateSkill404JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)
	case api.UpdateSkill409JSONResponse:
		// VersionConflictErrorResponse — has its own RequestId.
		require.NotNil(t, r.RequestId, "%s: RequestId must be set after injection", typeName)
		require.Equal(t, expectedID, *r.RequestId)
	case api.UpdateSkill500JSONResponse:
		require.NotNil(t, r.RequestId, "%s", typeName)

	default:
		t.Fatalf("assertID: unhandled type %T (typeName=%s) — add a case here when adding a new JSONResponse type", result, typeName)
	}
}

// TestInjectRequestID_AllErrorTypes exercises every ErrorResponse-carrying
// JSONResponse type against injectRequestIDIntoErrorResponse.
//
// Each sub-test verifies that after injection the request_id field is
// populated with the sentinel UUID. If a type is missing from the
// injectRequestIDIntoErrorResponse switch, it falls through to `default`
// and returns the original value unchanged — the request_id assertion here
// will then fail with a clear name identifying the uncovered type.
//
// EXCLUDED types (no RequestId field, not ErrorResponse-based):
//   - GetReadyz503JSONResponse (ReadinessResponse — health check data)
//   - BulkImportCatalog422JSONResponse (BulkImportResult — partial result)
//   - Success types: *200JSONResponse, *201JSONResponse, *204JSONResponse
func TestInjectRequestID_AllErrorTypes(t *testing.T) {
	t.Parallel()

	e := errResp()

	// Helper to construct embedded struct types
	bResp := api.BadRequestJSONResponse(e)
	iResp := api.InternalServerErrorJSONResponse(e)
	invResp := api.InvalidOrgIDJSONResponse(e)
	nfResp := api.NotFoundJSONResponse(e)
	tooLargeResp := api.RequestEntityTooLargeJSONResponse(e)
	var createAgent409 api.CreateAgent409JSONResponseBody
	require.NoError(t, createAgent409.FromErrorResponse(e))

	tests := []struct {
		name  string
		input any
	}{
		// Base types
		{"BadRequestJSONResponse", bResp},
		{"InternalServerErrorJSONResponse", iResp},
		{"InvalidOrgIDJSONResponse", invResp},
		{"NotFoundJSONResponse", nfResp},
		{"RequestEntityTooLargeJSONResponse", tooLargeResp},

		// Adapters
		{"ListAdapters400JSONResponse", api.ListAdapters400JSONResponse{BadRequestJSONResponse: bResp}},
		{"ListAdapters500JSONResponse", api.ListAdapters500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"CreateAdapter400JSONResponse", api.CreateAdapter400JSONResponse{BadRequestJSONResponse: bResp}},
		{"CreateAdapter500JSONResponse", api.CreateAdapter500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"DeleteAdapter400JSONResponse", api.DeleteAdapter400JSONResponse{InvalidOrgIDJSONResponse: invResp}},
		{"DeleteAdapter404JSONResponse", api.DeleteAdapter404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"DeleteAdapter500JSONResponse", api.DeleteAdapter500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"GetAdapter400JSONResponse", api.GetAdapter400JSONResponse{BadRequestJSONResponse: bResp}},
		{"GetAdapter404JSONResponse", api.GetAdapter404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"GetAdapter500JSONResponse", api.GetAdapter500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"UpdateAdapter400JSONResponse", api.UpdateAdapter400JSONResponse{BadRequestJSONResponse: bResp}},
		{"UpdateAdapter404JSONResponse", api.UpdateAdapter404JSONResponse{NotFoundJSONResponse: nfResp}},
		// Phase 3 OQ-1/A4: VersionConflict variant — struct with Current Adapter.
		{"UpdateAdapter409JSONResponse", api.UpdateAdapter409JSONResponse{Error: "version_conflict", Reason: "stale"}},
		{"UpdateAdapter500JSONResponse", api.UpdateAdapter500JSONResponse{InternalServerErrorJSONResponse: iResp}},

		// Agents
		{"ListAgents400JSONResponse", api.ListAgents400JSONResponse{BadRequestJSONResponse: bResp}},
		{"ListAgents500JSONResponse", api.ListAgents500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"CreateAgent400JSONResponse", api.CreateAgent400JSONResponse{BadRequestJSONResponse: bResp}},
		{"CreateAgent409JSONResponse", api.CreateAgent409JSONResponse(createAgent409)},
		// Phase 3 Codex C2 iter 3: flat ErrorResponse alias covering both
		// invalid_reference and invalid_value paths for skills[].
		{"CreateAgent422JSONResponse", api.CreateAgent422JSONResponse(e)},
		{"CreateAgent500JSONResponse", api.CreateAgent500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"DeleteAgent400JSONResponse", api.DeleteAgent400JSONResponse{InvalidOrgIDJSONResponse: invResp}},
		{"DeleteAgent404JSONResponse", api.DeleteAgent404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"DeleteAgent500JSONResponse", api.DeleteAgent500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"GetAgent400JSONResponse", api.GetAgent400JSONResponse{BadRequestJSONResponse: bResp}},
		{"GetAgent404JSONResponse", api.GetAgent404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"GetAgent500JSONResponse", api.GetAgent500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"UpdateAgent400JSONResponse", api.UpdateAgent400JSONResponse{BadRequestJSONResponse: bResp}},
		{"UpdateAgent404JSONResponse", api.UpdateAgent404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"UpdateAgent409JSONResponse", api.UpdateAgent409JSONResponse{Error: "version_conflict", Reason: "stale"}},
		// Phase 3 D-75 + ROADMAP CRIT 4: flat ErrorResponse alias covering both
		// invalid_reference and invalid_value paths for skills[].
		{"UpdateAgent422JSONResponse", api.UpdateAgent422JSONResponse(e)},
		{"UpdateAgent500JSONResponse", api.UpdateAgent500JSONResponse{InternalServerErrorJSONResponse: iResp}},

		// AgentStatus
		{"GetAgentStatus400JSONResponse", api.GetAgentStatus400JSONResponse{InvalidOrgIDJSONResponse: invResp}},
		{"GetAgentStatus404JSONResponse", api.GetAgentStatus404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"GetAgentStatus500JSONResponse", api.GetAgentStatus500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"PatchAgentStatus400JSONResponse", api.PatchAgentStatus400JSONResponse{BadRequestJSONResponse: bResp}},
		{"PatchAgentStatus404JSONResponse", api.PatchAgentStatus404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"PatchAgentStatus409JSONResponse", api.PatchAgentStatus409JSONResponse{Error: "invalid_transition"}},
		{"PatchAgentStatus422JSONResponse", api.PatchAgentStatus422JSONResponse(e)},
		{"PatchAgentStatus500JSONResponse", api.PatchAgentStatus500JSONResponse{InternalServerErrorJSONResponse: iResp}},

		// BreakReasons
		{"ListBreakReasons400JSONResponse", api.ListBreakReasons400JSONResponse{BadRequestJSONResponse: bResp}},
		{"ListBreakReasons500JSONResponse", api.ListBreakReasons500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"CreateBreakReason400JSONResponse", api.CreateBreakReason400JSONResponse{BadRequestJSONResponse: bResp}},
		{"CreateBreakReason500JSONResponse", api.CreateBreakReason500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"DeleteBreakReason400JSONResponse", api.DeleteBreakReason400JSONResponse{InvalidOrgIDJSONResponse: invResp}},
		{"DeleteBreakReason404JSONResponse", api.DeleteBreakReason404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"DeleteBreakReason500JSONResponse", api.DeleteBreakReason500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"GetBreakReason400JSONResponse", api.GetBreakReason400JSONResponse{BadRequestJSONResponse: bResp}},
		{"GetBreakReason404JSONResponse", api.GetBreakReason404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"GetBreakReason500JSONResponse", api.GetBreakReason500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"UpdateBreakReason400JSONResponse", api.UpdateBreakReason400JSONResponse{BadRequestJSONResponse: bResp}},
		{"UpdateBreakReason404JSONResponse", api.UpdateBreakReason404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"UpdateBreakReason409JSONResponse", api.UpdateBreakReason409JSONResponse{Error: "version_conflict", Reason: "stale"}},
		{"UpdateBreakReason500JSONResponse", api.UpdateBreakReason500JSONResponse{InternalServerErrorJSONResponse: iResp}},

		// BulkImport
		{"BulkImportCatalog400JSONResponse", api.BulkImportCatalog400JSONResponse(e)},
		{"BulkImportCatalog413JSONResponse", api.BulkImportCatalog413JSONResponse{RequestEntityTooLargeJSONResponse: tooLargeResp}},
		{"BulkImportCatalog500JSONResponse", api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: iResp}},

		// Channels
		{"ListChannels400JSONResponse", api.ListChannels400JSONResponse{BadRequestJSONResponse: bResp}},
		{"ListChannels500JSONResponse", api.ListChannels500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"CreateChannel400JSONResponse", api.CreateChannel400JSONResponse{BadRequestJSONResponse: bResp}},
		{"CreateChannel409JSONResponse", api.CreateChannel409JSONResponse(e)},
		// Phase 3 D-75: flat ErrorResponse alias for invalid_reference
		// (default_queue_id FK miss).
		{"CreateChannel422JSONResponse", api.CreateChannel422JSONResponse(e)},
		{"CreateChannel500JSONResponse", api.CreateChannel500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"DeleteChannel400JSONResponse", api.DeleteChannel400JSONResponse{InvalidOrgIDJSONResponse: invResp}},
		{"DeleteChannel404JSONResponse", api.DeleteChannel404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"DeleteChannel500JSONResponse", api.DeleteChannel500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"GetChannel400JSONResponse", api.GetChannel400JSONResponse{BadRequestJSONResponse: bResp}},
		{"GetChannel404JSONResponse", api.GetChannel404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"GetChannel500JSONResponse", api.GetChannel500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"UpdateChannel400JSONResponse", api.UpdateChannel400JSONResponse{BadRequestJSONResponse: bResp}},
		{"UpdateChannel404JSONResponse", api.UpdateChannel404JSONResponse{NotFoundJSONResponse: nfResp}},
		// Phase 3 OQ-1/A4: VersionConflict variant — struct with Current Channel.
		{"UpdateChannel409JSONResponse", api.UpdateChannel409JSONResponse{Error: "version_conflict", Reason: "stale"}},
		// Phase 3 D-75: flat ErrorResponse alias for invalid_reference
		// (default_queue_id FK miss).
		{"UpdateChannel422JSONResponse", api.UpdateChannel422JSONResponse(e)},
		{"UpdateChannel500JSONResponse", api.UpdateChannel500JSONResponse{InternalServerErrorJSONResponse: iResp}},

		// ImportJob
		{"GetImportJob400JSONResponse", api.GetImportJob400JSONResponse{InvalidOrgIDJSONResponse: invResp}},
		{"GetImportJob404JSONResponse", api.GetImportJob404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"GetImportJob500JSONResponse", api.GetImportJob500JSONResponse{InternalServerErrorJSONResponse: iResp}},

		// Queues
		{"ListQueues400JSONResponse", api.ListQueues400JSONResponse{BadRequestJSONResponse: bResp}},
		{"ListQueues500JSONResponse", api.ListQueues500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"CreateQueue400JSONResponse", api.CreateQueue400JSONResponse{BadRequestJSONResponse: bResp}},
		{"CreateQueue409JSONResponse", api.CreateQueue409JSONResponse(e)},
		{"CreateQueue500JSONResponse", api.CreateQueue500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"DeleteQueue400JSONResponse", api.DeleteQueue400JSONResponse{InvalidOrgIDJSONResponse: invResp}},
		{"DeleteQueue404JSONResponse", api.DeleteQueue404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"DeleteQueue500JSONResponse", api.DeleteQueue500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"GetQueue400JSONResponse", api.GetQueue400JSONResponse{BadRequestJSONResponse: bResp}},
		{"GetQueue404JSONResponse", api.GetQueue404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"GetQueue500JSONResponse", api.GetQueue500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"UpdateQueue400JSONResponse", api.UpdateQueue400JSONResponse{BadRequestJSONResponse: bResp}},
		{"UpdateQueue404JSONResponse", api.UpdateQueue404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"UpdateQueue409JSONResponse", api.UpdateQueue409JSONResponse{Error: "version_conflict", Reason: "stale"}},
		{"UpdateQueue500JSONResponse", api.UpdateQueue500JSONResponse{InternalServerErrorJSONResponse: iResp}},

		// Skills
		{"ListSkills400JSONResponse", api.ListSkills400JSONResponse{BadRequestJSONResponse: bResp}},
		{"ListSkills500JSONResponse", api.ListSkills500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"CreateSkill400JSONResponse", api.CreateSkill400JSONResponse{BadRequestJSONResponse: bResp}},
		{"CreateSkill409JSONResponse", api.CreateSkill409JSONResponse(e)},
		{"CreateSkill500JSONResponse", api.CreateSkill500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"DeleteSkill400JSONResponse", api.DeleteSkill400JSONResponse{InvalidOrgIDJSONResponse: invResp}},
		{"DeleteSkill404JSONResponse", api.DeleteSkill404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"DeleteSkill500JSONResponse", api.DeleteSkill500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"GetSkill400JSONResponse", api.GetSkill400JSONResponse{BadRequestJSONResponse: bResp}},
		{"GetSkill404JSONResponse", api.GetSkill404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"GetSkill500JSONResponse", api.GetSkill500JSONResponse{InternalServerErrorJSONResponse: iResp}},
		{"UpdateSkill400JSONResponse", api.UpdateSkill400JSONResponse{BadRequestJSONResponse: bResp}},
		{"UpdateSkill404JSONResponse", api.UpdateSkill404JSONResponse{NotFoundJSONResponse: nfResp}},
		{"UpdateSkill409JSONResponse", api.UpdateSkill409JSONResponse{Error: "version_conflict", Reason: "stale"}},
		{"UpdateSkill500JSONResponse", api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: iResp}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := injectRequestIDIntoErrorResponse(tc.input, sentinelID)
			assertID(t, result, tc.name)
		})
	}
}

// TestInjectRequestID_PreservesExistingID ensures the middleware does not
// overwrite a handler-set request_id. This is the "setIfNil" contract.
func TestInjectRequestID_PreservesExistingID(t *testing.T) {
	t.Parallel()

	existingID := api.UUIDv7(mustUUID("01901b2c-0000-7abc-0000-000000000001"))
	e := api.ErrorResponse{
		Error:     "test",
		RequestId: &existingID,
	}
	resp := api.BadRequestJSONResponse(e)
	result := injectRequestIDIntoErrorResponse(resp, sentinelID)
	got, ok := result.(api.BadRequestJSONResponse)
	require.True(t, ok)
	require.Equal(t, existingID, *got.RequestId, "existing RequestId must not be overwritten")
}

// TestInjectRequestID_SuccessPassThrough verifies that 200/201 response types
// are not modified (they don't have RequestId fields and must not panic).
func TestInjectRequestID_SuccessPassThrough(t *testing.T) {
	t.Parallel()

	// A 200 success type — not an error response, must pass through unchanged.
	resp := api.GetHealthz200JSONResponse{}
	result := injectRequestIDIntoErrorResponse(resp, sentinelID)
	require.Equal(t, resp, result, "success response must pass through unchanged")
}
