// testutil_test.go — per-entity test scaffolding shared across the
// state suite (D-73). Plan 04-03 establishes the canonical pattern.
//
// Why `_test.go` suffix: the Go build tool excludes _test.go files from
// production builds, so miniredis, testify, and testcontainers never end
// up in the shipped binary (Codex C7 iter 3 — confirmed Wave 3 review
// found no testing libs in production files).
//
// Why `package state` (not `state_test`): the entity test files
// (agent_states_test.go) also use `package state` so they can call the
// unexported helpers below without re-exporting them. This is the
// idiomatic Go convention when test helpers are package-internal.
//
// Wiring:
//   - sharedPool is initialised by main_test.go's TestMain (one Postgres
//     testcontainer per `go test ./internal/state/...` invocation).
//   - newTestHandlers(t) constructs ONE state.Server + miniredis +
//     httptest server PER TEST so test isolation is total (no leaking
//     cache state, no leaking orgID).
//   - cleanStateTables(t, ctx, pool, orgID) DELETEs agent_states + catalog
//     tables before each test starts so the suite is order-independent.
//   - httpPOST / httpGET / httpPATCH marshal JSON, set X-Org-Id +
//     Content-Type, and return (response, body) for the caller to parse.
package state

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/flowrt"
	"github.com/luongdev/open-routing/services/api/internal/server"
)

// sharedPool is the package-level pgxpool reused across every test in the
// state package (D-73). Assigned in main_test.go's TestMain after the
// Postgres testcontainer is up + migrations applied.
// nil when -short is set or Docker is unavailable.
var sharedPool *pgxpool.Pool

// TestHandlers bundles the per-test fixture so test bodies can read
// orgID, hit the httptest server, and inspect the miniredis fixture
// directly to assert cache hit/miss state (D-86).
//
// The Pool field aliases sharedPool — kept on the struct so tests that
// also touch the DB directly (e.g., seed test data via INSERT) don't
// have to reach for the package-level var.
type TestHandlers struct {
	S         *Server
	Miniredis *miniredis.Miniredis
	Pool      *pgxpool.Pool
	OrgID     uuid.UUID
	HTTP      *httptest.Server
	rdb       *redis.Client
}

// stateOnlyHandlers wraps *Server for the two state endpoints and panics
// on all other StrictServerInterface methods (they are never called in
// state package tests).
//
// TEMP: Wave 2 test-only composite; Wave 4 replaces with cmd/api/main.go ApiHandlers.
type stateOnlyHandlers struct {
	*Server
	*flowrt.Endpoints
}

// Verify compile-time that stateOnlyHandlers satisfies StrictServerInterface.
var _ api.StrictServerInterface = stateOnlyHandlers{}

func (stateOnlyHandlers) GetDocs(_ context.Context, _ api.GetDocsRequestObject) (api.GetDocsResponseObject, error) {
	panic("state test: GetDocs not implemented")
}
func (stateOnlyHandlers) GetHealthz(_ context.Context, _ api.GetHealthzRequestObject) (api.GetHealthzResponseObject, error) {
	panic("state test: GetHealthz not implemented")
}
func (stateOnlyHandlers) GetOpenAPISpec(_ context.Context, _ api.GetOpenAPISpecRequestObject) (api.GetOpenAPISpecResponseObject, error) {
	panic("state test: GetOpenAPISpec not implemented")
}
func (stateOnlyHandlers) GetReadyz(_ context.Context, _ api.GetReadyzRequestObject) (api.GetReadyzResponseObject, error) {
	panic("state test: GetReadyz not implemented")
}
func (stateOnlyHandlers) ListAdapters(_ context.Context, _ api.ListAdaptersRequestObject) (api.ListAdaptersResponseObject, error) {
	panic("state test: ListAdapters not implemented")
}
func (stateOnlyHandlers) CreateFlow(_ context.Context, _ api.CreateFlowRequestObject) (api.CreateFlowResponseObject, error) {
	panic("state test: CreateFlow not implemented")
}
func (stateOnlyHandlers) GetFlow(_ context.Context, _ api.GetFlowRequestObject) (api.GetFlowResponseObject, error) {
	panic("state test: GetFlow not implemented")
}
func (stateOnlyHandlers) ListFlows(_ context.Context, _ api.ListFlowsRequestObject) (api.ListFlowsResponseObject, error) {
	panic("state test: ListFlows not implemented")
}
func (stateOnlyHandlers) UpdateFlow(_ context.Context, _ api.UpdateFlowRequestObject) (api.UpdateFlowResponseObject, error) {
	panic("state test: UpdateFlow not implemented")
}
func (stateOnlyHandlers) DeleteFlow(_ context.Context, _ api.DeleteFlowRequestObject) (api.DeleteFlowResponseObject, error) {
	panic("state test: DeleteFlow not implemented")
}
func (stateOnlyHandlers) CreateAdapter(_ context.Context, _ api.CreateAdapterRequestObject) (api.CreateAdapterResponseObject, error) {
	panic("state test: CreateAdapter not implemented")
}
func (stateOnlyHandlers) DeleteAdapter(_ context.Context, _ api.DeleteAdapterRequestObject) (api.DeleteAdapterResponseObject, error) {
	panic("state test: DeleteAdapter not implemented")
}
func (stateOnlyHandlers) GetAdapter(_ context.Context, _ api.GetAdapterRequestObject) (api.GetAdapterResponseObject, error) {
	panic("state test: GetAdapter not implemented")
}
func (stateOnlyHandlers) UpdateAdapter(_ context.Context, _ api.UpdateAdapterRequestObject) (api.UpdateAdapterResponseObject, error) {
	panic("state test: UpdateAdapter not implemented")
}
func (stateOnlyHandlers) ListAgents(_ context.Context, _ api.ListAgentsRequestObject) (api.ListAgentsResponseObject, error) {
	panic("state test: ListAgents not implemented")
}
func (stateOnlyHandlers) CreateAgent(_ context.Context, _ api.CreateAgentRequestObject) (api.CreateAgentResponseObject, error) {
	panic("state test: CreateAgent not implemented")
}
func (stateOnlyHandlers) DeleteAgent(_ context.Context, _ api.DeleteAgentRequestObject) (api.DeleteAgentResponseObject, error) {
	panic("state test: DeleteAgent not implemented")
}
func (stateOnlyHandlers) GetAgent(_ context.Context, _ api.GetAgentRequestObject) (api.GetAgentResponseObject, error) {
	panic("state test: GetAgent not implemented")
}
func (stateOnlyHandlers) UpdateAgent(_ context.Context, _ api.UpdateAgentRequestObject) (api.UpdateAgentResponseObject, error) {
	panic("state test: UpdateAgent not implemented")
}
func (stateOnlyHandlers) ListBreakReasons(_ context.Context, _ api.ListBreakReasonsRequestObject) (api.ListBreakReasonsResponseObject, error) {
	panic("state test: ListBreakReasons not implemented")
}
func (stateOnlyHandlers) CreateBreakReason(_ context.Context, _ api.CreateBreakReasonRequestObject) (api.CreateBreakReasonResponseObject, error) {
	panic("state test: CreateBreakReason not implemented")
}
func (stateOnlyHandlers) DeleteBreakReason(_ context.Context, _ api.DeleteBreakReasonRequestObject) (api.DeleteBreakReasonResponseObject, error) {
	panic("state test: DeleteBreakReason not implemented")
}
func (stateOnlyHandlers) GetBreakReason(_ context.Context, _ api.GetBreakReasonRequestObject) (api.GetBreakReasonResponseObject, error) {
	panic("state test: GetBreakReason not implemented")
}
func (stateOnlyHandlers) UpdateBreakReason(_ context.Context, _ api.UpdateBreakReasonRequestObject) (api.UpdateBreakReasonResponseObject, error) {
	panic("state test: UpdateBreakReason not implemented")
}
func (stateOnlyHandlers) BulkImportCatalog(_ context.Context, _ api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
	panic("state test: BulkImportCatalog not implemented")
}
func (stateOnlyHandlers) ListChannels(_ context.Context, _ api.ListChannelsRequestObject) (api.ListChannelsResponseObject, error) {
	panic("state test: ListChannels not implemented")
}
func (stateOnlyHandlers) CreateChannel(_ context.Context, _ api.CreateChannelRequestObject) (api.CreateChannelResponseObject, error) {
	panic("state test: CreateChannel not implemented")
}
func (stateOnlyHandlers) DeleteChannel(_ context.Context, _ api.DeleteChannelRequestObject) (api.DeleteChannelResponseObject, error) {
	panic("state test: DeleteChannel not implemented")
}
func (stateOnlyHandlers) GetChannel(_ context.Context, _ api.GetChannelRequestObject) (api.GetChannelResponseObject, error) {
	panic("state test: GetChannel not implemented")
}
func (stateOnlyHandlers) UpdateChannel(_ context.Context, _ api.UpdateChannelRequestObject) (api.UpdateChannelResponseObject, error) {
	panic("state test: UpdateChannel not implemented")
}
func (stateOnlyHandlers) GetImportJob(_ context.Context, _ api.GetImportJobRequestObject) (api.GetImportJobResponseObject, error) {
	panic("state test: GetImportJob not implemented")
}
func (stateOnlyHandlers) ListQueues(_ context.Context, _ api.ListQueuesRequestObject) (api.ListQueuesResponseObject, error) {
	panic("state test: ListQueues not implemented")
}
func (stateOnlyHandlers) CreateQueue(_ context.Context, _ api.CreateQueueRequestObject) (api.CreateQueueResponseObject, error) {
	panic("state test: CreateQueue not implemented")
}
func (stateOnlyHandlers) DeleteQueue(_ context.Context, _ api.DeleteQueueRequestObject) (api.DeleteQueueResponseObject, error) {
	panic("state test: DeleteQueue not implemented")
}
func (stateOnlyHandlers) GetQueue(_ context.Context, _ api.GetQueueRequestObject) (api.GetQueueResponseObject, error) {
	panic("state test: GetQueue not implemented")
}
func (stateOnlyHandlers) UpdateQueue(_ context.Context, _ api.UpdateQueueRequestObject) (api.UpdateQueueResponseObject, error) {
	panic("state test: UpdateQueue not implemented")
}
func (stateOnlyHandlers) ListSkills(_ context.Context, _ api.ListSkillsRequestObject) (api.ListSkillsResponseObject, error) {
	panic("state test: ListSkills not implemented")
}
func (stateOnlyHandlers) CreateSkill(_ context.Context, _ api.CreateSkillRequestObject) (api.CreateSkillResponseObject, error) {
	panic("state test: CreateSkill not implemented")
}
func (stateOnlyHandlers) DeleteSkill(_ context.Context, _ api.DeleteSkillRequestObject) (api.DeleteSkillResponseObject, error) {
	panic("state test: DeleteSkill not implemented")
}
func (stateOnlyHandlers) GetSkill(_ context.Context, _ api.GetSkillRequestObject) (api.GetSkillResponseObject, error) {
	panic("state test: GetSkill not implemented")
}
func (stateOnlyHandlers) UpdateSkill(_ context.Context, _ api.UpdateSkillRequestObject) (api.UpdateSkillResponseObject, error) {
	panic("state test: UpdateSkill not implemented")
}

// newTestHandlers constructs ONE fixture per test (D-73):
//   - one miniredis instance (fresh empty cache state)
//   - one OrgDB wrapping sharedPool with the SQLChecker in panic mode
//     (mirroring the FOUND-04 dev-test contract)
//   - one state.Server wired with miniredis + OrgDB + slog
//   - one httptest.Server fronting the PRODUCTION chi mux + middleware
//     chain (server.NewMux) so the test exercises the same code path
//     the binary ships
//
// Skips the calling test when sharedPool == nil (TestMain skipped
// bring-up due to Docker unavailability or -short).
func newTestHandlers(t testing.TB) *TestHandlers {
	t.Helper()
	if sharedPool == nil {
		t.Skip("state: sharedPool nil — Docker testcontainer unavailable (run without -short)")
		return nil
	}

	// 1. miniredis — fresh per test so cache state never leaks.
	mr := miniredis.RunT(t)

	// 2. *redis.Client pointing at miniredis. RunT wires Close() into
	//    t.Cleanup, so we don't double-Close here.
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	// 3. Cache wrapper — slog discard so test output stays clean.
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	c := cache.New(rdb, logger)

	// 4. OrgDB — ValidationPanic per FOUND-04 dev-test contract. Any
	//    sqlc query that fails the SQLChecker panics here, surfacing
	//    org-scoping regressions immediately.
	orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)

	// 5. state.Server — the StrictServerInterface impl under test.
	//    WithSweepInterval(100ms) compresses the safety sweep for test speed.
	s := New(Deps{
		OrgDB:  orgDB,
		Cache:  c,
		Logger: logger,
	}, WithSweepInterval(100*time.Millisecond))

	// 6. server.NewMux with a stateOnlyHandlers composite that routes the
	//    two state endpoints to s and panics on all others (catalog endpoints
	//    are not exercised here — Wave 4 production wiring uses real ApiHandlers).
	swagger, _ := api.GetSpec()
	specBytes, _ := yaml.Marshal(swagger)
	cfg := &config.Config{
		DatabaseURL:    "n/a",
		RedisURL:       "n/a",
		OTelExporter:   "stdout",
		ListenAddr:     ":0",
		ValidationMode: "panic",
	}
	mux := server.NewMux(&server.Deps{
		Pool:           sharedPool,
		Redis:          rdb,
		OrgDB:          orgDB,
		Config:         cfg,
		StrictHandlers: stateOnlyHandlers{Server: s, Endpoints: flowrt.New(flowrt.Deps{OrgDB: orgDB, Cache: c, Logger: logger})},
		SpecBytes:      specBytes,
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// 7. Per-test orgID. UUIDv7 because OrgContext middleware rejects
	//    UUIDv4 with 400 invalid_org_id/uuidv7_required (D-19).
	orgID := uuid.Must(uuid.NewV7())

	return &TestHandlers{
		S:         s,
		Miniredis: mr,
		Pool:      sharedPool,
		OrgID:     orgID,
		HTTP:      srv,
		rdb:       rdb,
	}
}

// cleanStateTables removes state + catalog rows for a single org.
// Per-org DELETE instead of TRUNCATE so parallel tests with distinct orgIDs
// do not stomp each other's fixtures (pre-existing race — FOUND-D-73 test
// isolation contract requires orgID-scoped cleanup in parallel suites).
func cleanStateTables(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) {
	t.Helper()
	if pool == nil {
		return
	}
	// No FK constraints across these tables (D-76, D-80) so order is free.
	for _, stmt := range []string{
		`DELETE FROM agent_states  WHERE org_id = $1`,
		`DELETE FROM agent_skills  WHERE org_id = $1`,
		`DELETE FROM break_reasons WHERE org_id = $1`,
		`DELETE FROM agents        WHERE org_id = $1`,
	} {
		_, err := pool.Exec(ctx, stmt, orgID)
		require.NoError(t, err, "cleanStateTables: DELETE failed for %s", stmt)
	}
}

// ---------------------------------------------------------------------------
// Seed helpers — raw INSERTs that bypass the HTTP layer (D-82 pattern).
// ---------------------------------------------------------------------------

// seedAgent inserts an agents row via direct INSERT (bypasses CreateAgent
// handler — keeps Wave 2 tests independent of Wave 4 composite wiring).
// Phase 04.1: InsertAgentParams gained a required `Code` field and `ExternalID`
// flipped to `*string` (the column is nullable post-04.1). Helper derives a
// regex-compliant code from the externalID slug for backwards compatibility.
func seedAgent(t testing.TB, pool *pgxpool.Pool, orgID, agentID uuid.UUID, externalID, name string) {
	t.Helper()
	q := generated.New(pool)
	ext := externalID
	_, err := q.InsertAgent(ctx(t), generated.InsertAgentParams{
		ID:         pgUUIDv(agentID),
		OrgID:      pgUUIDv(orgID),
		Code:       "emp_" + sanitizeForCode(externalID), // Phase 04.1: required.
		ExternalID: &ext,                                 // *string post-04.1.
		Name:       name,
		Email:      externalID + "@test.example",
		Enabled:    true,
	})
	require.NoError(t, err, "seedAgent: InsertAgent")

	// Seed initial agent_states row (status=Offline, version=1) so the
	// state package's GET/PATCH handlers can find the row (D-93 pattern).
	_, err = q.InsertAgentState(ctx(t), generated.InsertAgentStateParams{
		AgentID: pgUUIDv(agentID),
		OrgID:   pgUUIDv(orgID),
		Status:  "Offline",
	})
	require.NoError(t, err, "seedAgent: InsertAgentState")
}

// seedBreakReason inserts a break_reasons row directly. routable parameter
// controls the STATE-10 IsRoutable input.
// Phase 04.1: InsertBreakReasonParams gained a required `Code` field and a
// new optional `ExternalID *string` field (the column was added by Plan 01).
// Helper derives a regex-compliant code from the name.
func seedBreakReason(t testing.TB, pool *pgxpool.Pool, orgID, id uuid.UUID, name string, routable bool) {
	t.Helper()
	q := generated.New(pool)
	_, err := q.InsertBreakReason(ctx(t), generated.InsertBreakReasonParams{
		ID:           pgUUIDv(id),
		OrgID:        pgUUIDv(orgID),
		Code:         "break_" + sanitizeForCode(name), // Phase 04.1: required.
		ExternalID:   nil,                              // Phase 04.1: optional, NULL.
		Name:         name,
		Routable:     routable,
		DisplayOrder: 0,
		Enabled:      true,
	})
	require.NoError(t, err, "seedBreakReason: InsertBreakReason")
}

// SeedStateParams carries all columns for a direct agent_states INSERT.
// Bypasses the transition matrix so tests for Engaged/WrapUp states can
// set up their fixtures without orchestrating through PATCH (D-82 —
// system-initiated states have no v0.1 caller).
type SeedStateParams struct {
	AgentID              uuid.UUID
	OrgID                uuid.UUID
	Status               string
	EngagedChannel       *string
	BreakReasonID        *uuid.UUID
	PostInteractionState *string
	WrapupUntil          *time.Time
	StateVersion         int64
}

// seedAgentStateRow inserts an agent_states row directly. Bypasses the
// transition matrix so tests for Engaged/WrapUp states can set up their
// fixtures without orchestrating through PATCH (D-82 — system-initiated
// states have no v0.1 caller). RESEARCH §Testutil Deviations.
func seedAgentStateRow(t testing.TB, pool *pgxpool.Pool, p SeedStateParams) {
	t.Helper()
	version := p.StateVersion
	if version == 0 {
		version = 1
	}
	_, err := pool.Exec(ctx(t),
		`INSERT INTO agent_states
			(agent_id, org_id, status, engaged_channel, break_reason_id,
			 post_interaction_state, wrapup_until, state_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (agent_id) DO UPDATE
		   SET status                 = EXCLUDED.status,
		       engaged_channel        = EXCLUDED.engaged_channel,
		       break_reason_id        = EXCLUDED.break_reason_id,
		       post_interaction_state = EXCLUDED.post_interaction_state,
		       wrapup_until           = EXCLUDED.wrapup_until,
		       state_version          = EXCLUDED.state_version,
		       updated_at             = NOW()`,
		p.AgentID,
		p.OrgID,
		p.Status,
		p.EngagedChannel,
		p.BreakReasonID,
		p.PostInteractionState,
		p.WrapupUntil,
		version,
	)
	require.NoError(t, err, "seedAgentStateRow: INSERT")
}

// ctx returns a background context for seed helpers. Avoids threading t.Context
// which is not available in all Go versions.
func ctx(_ testing.TB) context.Context {
	return context.Background()
}

// pgUUIDv wraps uuid.UUID into pgtype.UUID for seed helper params.
func pgUUIDv(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func agentStatusPath(orgID, agentID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/agents/" + agentID.String() + "/status"
}

func httpPATCHStatus(t testing.TB, th *TestHandlers, agentID uuid.UUID, body any) (*http.Response, []byte) {
	t.Helper()
	return httpPATCH(t, th.HTTP, th.OrgID, agentStatusPath(th.OrgID, agentID), body)
}

func httpGETStatus(t testing.TB, th *TestHandlers, agentID uuid.UUID) (*http.Response, []byte) {
	t.Helper()
	return httpGET(t, th.HTTP, th.OrgID, agentStatusPath(th.OrgID, agentID), nil)
}

func httpGET(t testing.TB, srv *httptest.Server, orgID uuid.UUID, path string, q url.Values) (*http.Response, []byte) {
	t.Helper()
	return doJSON(t, srv, http.MethodGet, orgID, path, q, nil)
}

func httpPATCH(t testing.TB, srv *httptest.Server, orgID uuid.UUID, path string, body any) (*http.Response, []byte) {
	t.Helper()
	return doJSON(t, srv, http.MethodPatch, orgID, path, nil, body)
}

func doJSON(t testing.TB, srv *httptest.Server, method string, orgID uuid.UUID, path string, q url.Values, body any) (*http.Response, []byte) {
	t.Helper()

	u := srv.URL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err, "httpHelper: marshal body")
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, u, reqBody)
	require.NoError(t, err, "httpHelper: NewRequest")
	req.Header.Set("X-Org-Id", orgID.String())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := srv.Client().Do(req)
	require.NoError(t, err, "httpHelper: Do")
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "httpHelper: ReadAll")
	return resp, raw
}

// sanitizeForCode converts an arbitrary string into a valid `code` per the
// D04_1-03 regex (`^[a-z][a-z0-9_]{0,63}$`). Used by Phase 04.1 seed helpers
// so they can derive a unique code from a display name / external_id slug
// without hand-crafting it per call site. Truncates at 60 chars to keep the
// entire code <= 64 once a domain prefix is prepended.
func sanitizeForCode(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s) && len(out) < 60; i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+('a'-'A'))
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 || out[0] < 'a' || out[0] > 'z' {
		out = append([]byte{'r'}, out...)
	}
	return string(out)
}
