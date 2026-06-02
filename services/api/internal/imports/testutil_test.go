// testutil_test.go — per-test scaffolding shared across the imports
// suite (D-73 carry-forward from state/testutil_test.go).
//
// Wave 2 (Plan 05-04) shipped the bare minimum:
//
//   - sharedPool aliasing for sweep_test.go.
//   - cleanImportTables for per-org DELETE before each test (mirrors
//     state.cleanStateTables; per-org instead of TRUNCATE so parallel
//     tests with distinct orgIDs cannot trample each other).
//   - newTestImports for building an Importer wired against
//     sharedPool with the SQLChecker in panic mode (FOUND-04 dev-test
//     contract) and a discarded logger (test output stays clean).
//
// Wave 5 (Plan 05-07 — THIS WAVE) extends with:
//
//   - Composite ApiHandlers wiring (catalog.Handlers + state.Server +
//     imports.Importer) so handler-level tests exercise the full chi mux
//     middleware chain (BodyLimit included).
//   - httptest.Server fronting server.NewMux — every test request flows
//     through the production code path.
//   - HTTP helpers (postImportJSON, postImportCSV, getImportJob,
//     loadTestData, etc.) so test bodies focus on assertions.
//
// Why `_test.go` suffix: the Go build tool excludes _test.go files
// from production builds, so miniredis and testcontainers never end
// up in the shipped binary.
//
// Why `package imports`: this lets test bodies in sweep_test.go reach
// unexported methods (runSweepPastDue, etc.) without re-exporting
// them.
package imports

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonboulle/clockwork"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/catalog"
	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/flowrt"
	"github.com/luongdev/open-routing/services/api/internal/server"
	"github.com/luongdev/open-routing/services/api/internal/state"
)

// sharedPool is the package-level pgxpool reused across every test in
// the imports package (D-73). Assigned in main_test.go's TestMain
// after the Postgres testcontainer is up + migrations applied.
// nil when -short is set or Docker is unavailable.
var sharedPool *pgxpool.Pool

// TestImports bundles the per-test fixture so test bodies can read
// orgID, the Importer under test, and the pool for raw-SQL row
// seeding (sweep_test.go uses raw INSERT to seed past-due pending
// rows — the sqlc query InsertImportJob hardcodes status='pending'
// which is fine for one path but not for the "past-due timestamp"
// shape sweep_test needs).
//
// The Pool field aliases sharedPool — kept on the struct so test
// bodies can write `th.Pool.Exec(...)` without reaching for the
// package-level var.
//
// Wave 5 additions:
//   - HTTP is the httptest.Server fronting the production chi mux.
//     Handler integration tests POST/GET through it so the full
//     middleware chain (RequestID + orgContext + BodyLimit +
//     strict-server) runs.
//   - rdb keeps a handle on the *redis.Client so test bodies can
//     inspect cache invalidation behaviour.
type TestImports struct {
	I       *Importer
	OrgDB   *db.OrgDB
	Pool    *pgxpool.Pool
	OrgID   uuid.UUID
	FakeClk clockwork.FakeClock
	Logger  *slog.Logger
	HTTP    *httptest.Server // Wave 5: full mux for handler integration tests.
	rdb     *redis.Client
	cleanup func()
}

// importsApiHandlers mirrors the production three-embed ApiHandlers
// composite from cmd/api/main.go. Used by newTestImports so handler-
// level integration tests exercise the full strict-server interface
// (catalog endpoints used for FK probe seeding via UpsertXByCode,
// state endpoints used for agent_states verification, imports
// endpoints under test).
//
// Compile-time: var _ api.StrictServerInterface ... lives at the
// package level via the import of the api package; the composite
// shape proves itself by failing to build if a method goes missing.
type importsApiHandlers struct {
	*catalog.Handlers
	*state.Server
	*Importer
	*flowrt.Endpoints
}

// newTestImports constructs ONE Importer per test (D-73):
//   - one OrgDB wrapping sharedPool with the SQLChecker in panic
//     mode (FOUND-04 dev-test contract — any sqlc query that fails
//     the SQLChecker panics here, surfacing org-scoping regressions
//     immediately).
//   - one cache instance backed by miniredis (per-test fresh state,
//     no leaks between tests — D-49 spirit).
//   - one clockwork.FakeClock for deterministic ticker advancement.
//   - one Importer with the above deps + a 1h sweep interval (Wave 2
//     sweep tests advance the fake clock manually; the test harness
//     never relies on real-time elapsing).
//   - Wave 5 addition: httptest.Server fronting server.NewMux so
//     handler-level integration tests exercise the full middleware
//     chain.
//
// Skips the calling test when sharedPool == nil (TestMain skipped
// bring-up due to Docker unavailability or -short).
func newTestImports(t testing.TB) *TestImports {
	t.Helper()
	if sharedPool == nil {
		t.Skip("imports: sharedPool nil — Docker testcontainer unavailable (run without -short)")
		return nil
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	// miniredis + cache — Wave 2 sweep tests do not exercise the
	// cache (sweep only writes import_jobs), but newTestImports
	// constructs one anyway so Wave 5 handler tests inherit the
	// fixture shape with no re-wiring.
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	c := cache.New(rdb, logger)

	orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)

	fakeClock := clockwork.NewFakeClock()
	imp := New(Deps{
		OrgDB:  orgDB,
		Cache:  c,
		Logger: logger,
	}, WithClock(fakeClock), WithSweepInterval(1*time.Hour))

	// Catalog handlers — Wave 5 handler integration tests need real
	// catalog.Handlers + state.Server to satisfy the full
	// StrictServerInterface assertion in server.NewMux. They are
	// constructed but never called directly by import tests; only
	// the imports.Importer methods are exercised.
	catalogHandlers := catalog.New(catalog.Deps{
		OrgDB:  orgDB,
		Pool:   sharedPool,
		Cache:  c,
		Logger: logger,
	})
	stateServer := state.New(state.Deps{
		OrgDB:  orgDB,
		Cache:  c,
		Logger: logger,
	}, state.WithClock(clockwork.NewRealClock()))
	// state.Server.Start runs the synchronous startup sweep; not
	// required for the import tests but keeps the fixture honest.
	require.NoError(t, stateServer.Start(context.Background()),
		"newTestImports: stateServer.Start")
	t.Cleanup(stateServer.Stop)

	handlers := &importsApiHandlers{
		Handlers:  catalogHandlers,
		Server:    stateServer,
		Importer:  imp,
		Endpoints: flowrt.New(flowrt.Deps{OrgDB: orgDB, Cache: c, Logger: logger}),
	}

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
		StrictHandlers: handlers,
		SpecBytes:      specBytes,
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Per-test orgID. UUIDv7 because production OrgContext middleware
	// rejects UUIDv4 with 400 invalid_org_id/uuidv7_required (D-19).
	orgID := uuid.Must(uuid.NewV7())

	th := &TestImports{
		I:       imp,
		OrgDB:   orgDB,
		Pool:    sharedPool,
		OrgID:   orgID,
		FakeClk: fakeClock,
		Logger:  logger,
		HTTP:    srv,
		rdb:     rdb,
		cleanup: func() {
			// Importer.Stop is idempotent so unconditional cleanup is
			// safe even when the test never called Start.
			imp.Stop()
		},
	}
	t.Cleanup(th.cleanup)
	return th
}

// cleanImportTables removes import_jobs (and the catalog + state
// tables that Wave 5 handler tests will touch) for a single org.
// Per-org DELETE instead of TRUNCATE so parallel tests with distinct
// orgIDs do not stomp each other's fixtures (D-73 isolation contract).
//
// Order matches state/testutil_test.go's cleanStateTables: import_jobs
// first (it has no FKs), then the 6 catalog tables + agent_states +
// agent_skills (Wave 5 will need these clean for handler tests).
//
// No FK constraints across these tables (D-76, D-80) so order is
// technically free, but keeping it consistent helps when reading
// query plans.
func cleanImportTables(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) {
	t.Helper()
	if pool == nil {
		return
	}
	for _, stmt := range []string{
		`DELETE FROM import_jobs   WHERE org_id = $1`,
		`DELETE FROM agent_states  WHERE org_id = $1`,
		`DELETE FROM agent_skills  WHERE org_id = $1`,
		`DELETE FROM agents        WHERE org_id = $1`,
		`DELETE FROM skills        WHERE org_id = $1`,
		`DELETE FROM queues        WHERE org_id = $1`,
		`DELETE FROM channels      WHERE org_id = $1`,
		`DELETE FROM adapters      WHERE org_id = $1`,
		`DELETE FROM break_reasons WHERE org_id = $1`,
	} {
		_, err := pool.Exec(ctx, stmt, orgID)
		require.NoError(t, err, "cleanImportTables: DELETE failed for %s", stmt)
	}
}

// seedPendingImportJob inserts an import_jobs row directly via raw
// SQL with a caller-specified updated_at — necessary because the sqlc
// InsertImportJob query lets Postgres NOW() populate updated_at, and
// sweep_test needs to seed rows whose updated_at is in the past.
//
// Returns the minted job ID for later assertion.
func seedPendingImportJob(
	t testing.TB,
	ctx context.Context,
	pool *pgxpool.Pool,
	orgID uuid.UUID,
	entityType string,
	updatedAt time.Time,
) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx,
		`INSERT INTO import_jobs
			(id, org_id, entity_type, status, total_rows, succeeded_rows, failed_rows, errors, idempotency_key, created_at, updated_at)
		 VALUES ($1, $2, $3, 'pending', 0, 0, 0, NULL, NULL, $4, $4)`,
		id, orgID, entityType, updatedAt,
	)
	require.NoError(t, err, "seedPendingImportJob: INSERT")
	return id
}

// fetchImportJobStatus returns (status, errorsJSONB) for a given
// (orgID, jobID). Used by sweep_test.go to assert post-sweep state.
func fetchImportJobStatus(
	t testing.TB,
	ctx context.Context,
	pool *pgxpool.Pool,
	orgID, jobID uuid.UUID,
) (string, []byte) {
	t.Helper()
	row := pool.QueryRow(ctx,
		`SELECT status, errors FROM import_jobs WHERE id = $1 AND org_id = $2`,
		jobID, orgID,
	)
	var status string
	var errorsRaw []byte
	require.NoError(t, row.Scan(&status, &errorsRaw), "fetchImportJobStatus: scan")
	return status, errorsRaw
}

// ---------------------------------------------------------------------------
// Wave 5 HTTP helpers — exercise the full chi mux through httptest.
// ---------------------------------------------------------------------------

// importPath returns the canonical bulk-import URL path. The orgID is
// embedded in the path AND emitted as X-Org-Id header (D-20) — the
// middleware enforces the path/header match (D-21).
func importPath(orgID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/catalog/import"
}

// importJobPath returns the GET-import-job URL path.
func importJobPath(orgID, jobID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/imports/" + jobID.String()
}

// postImportJSON marshals rows into a JSON array and POSTs to the
// bulk-import endpoint. Returns the response + body. Uses the test's
// OrgID for both the URL and X-Org-Id header.
func postImportJSON(
	t testing.TB,
	th *TestImports,
	entity api.ImportEntityType,
	rows []interface{},
) (*http.Response, []byte) {
	t.Helper()
	raw, err := json.Marshal(rows)
	require.NoError(t, err, "postImportJSON: marshal")
	return doImportRequest(t, th, http.MethodPost,
		importPath(th.OrgID)+"?entity="+string(entity),
		"application/json", bytes.NewReader(raw), nil)
}

// postImportJSONWithIdempotencyKey is the same as postImportJSON but
// adds the Idempotency-Key header (D5-13).
func postImportJSONWithIdempotencyKey(
	t testing.TB,
	th *TestImports,
	entity api.ImportEntityType,
	rows []interface{},
	key uuid.UUID,
) (*http.Response, []byte) {
	t.Helper()
	raw, err := json.Marshal(rows)
	require.NoError(t, err, "postImportJSONWithIdempotencyKey: marshal")
	return doImportRequest(t, th, http.MethodPost,
		importPath(th.OrgID)+"?entity="+string(entity),
		"application/json", bytes.NewReader(raw),
		map[string]string{"Idempotency-Key": key.String()})
}

// postImportCSV POSTs a CSV body to the bulk-import endpoint with
// Content-Type: text/csv + the mandatory ?schema_version=v0.1 query
// param (IMP-08).
func postImportCSV(
	t testing.TB,
	th *TestImports,
	entity api.ImportEntityType,
	csvBody []byte,
) (*http.Response, []byte) {
	t.Helper()
	return doImportRequest(t, th, http.MethodPost,
		importPath(th.OrgID)+"?entity="+string(entity)+"&schema_version=v0.1",
		"text/csv", bytes.NewReader(csvBody), nil)
}

// postImportCSVNoSchemaVersion is the same as postImportCSV but omits
// the ?schema_version query param — used to assert IMP-08 rejects the
// request with 400.
func postImportCSVNoSchemaVersion(
	t testing.TB,
	th *TestImports,
	entity api.ImportEntityType,
	csvBody []byte,
) (*http.Response, []byte) {
	t.Helper()
	return doImportRequest(t, th, http.MethodPost,
		importPath(th.OrgID)+"?entity="+string(entity),
		"text/csv", bytes.NewReader(csvBody), nil)
}

// postImportCSVWithSchemaVersion lets callers explicitly choose a
// schema_version (so the test can assert v0.2 → 400).
func postImportCSVWithSchemaVersion(
	t testing.TB,
	th *TestImports,
	entity api.ImportEntityType,
	csvBody []byte,
	schemaVersion string,
) (*http.Response, []byte) {
	t.Helper()
	return doImportRequest(t, th, http.MethodPost,
		importPath(th.OrgID)+"?entity="+string(entity)+"&schema_version="+schemaVersion,
		"text/csv", bytes.NewReader(csvBody), nil)
}

// getImportJob GETs an import_jobs row by id.
func getImportJob(
	t testing.TB,
	th *TestImports,
	jobID uuid.UUID,
) (*http.Response, []byte) {
	t.Helper()
	return doImportRequest(t, th, http.MethodGet,
		importJobPath(th.OrgID, jobID), "", nil, nil)
}

// getImportJobAsOrg is like getImportJob but sends X-Org-Id of a
// DIFFERENT org (orgB) to probe cross-org isolation (FOUND-08).
//
//nolint:unused // retained for future cross-org isolation tests;
// the analog in test/isolation/imports_test.go covers the current
// FOUND-08 surface and uses a different signature.
func getImportJobAsOrg(
	t testing.TB,
	th *TestImports,
	orgID, jobID uuid.UUID,
) (*http.Response, []byte) {
	t.Helper()
	u := th.HTTP.URL + importJobPath(orgID, jobID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	require.NoError(t, err, "getImportJobAsOrg: NewRequest")
	req.Header.Set("X-Org-Id", orgID.String())
	resp, err := th.HTTP.Client().Do(req)
	require.NoError(t, err, "getImportJobAsOrg: Do")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "getImportJobAsOrg: ReadAll")
	return resp, body
}

// doImportRequest is the shared backbone of every Wave 5 HTTP helper.
// Sets X-Org-Id from th.OrgID, optional Content-Type, optional extra
// headers, and reads/returns the body so the caller can defer-close
// without juggling resp.Body lifetimes.
func doImportRequest(
	t testing.TB,
	th *TestImports,
	method, path, contentType string,
	body io.Reader,
	extraHeaders map[string]string,
) (*http.Response, []byte) {
	t.Helper()
	u := th.HTTP.URL + path
	req, err := http.NewRequestWithContext(context.Background(), method, u, body)
	require.NoError(t, err, "doImportRequest: NewRequest")
	req.Header.Set("X-Org-Id", th.OrgID.String())
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	resp, err := th.HTTP.Client().Do(req)
	require.NoError(t, err, "doImportRequest: Do")
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "doImportRequest: ReadAll")
	return resp, raw
}

// loadTestData reads a file from internal/imports/testdata/ relative
// to the test binary's working directory (which Go sets to the
// package directory).
func loadTestData(t testing.TB, filename string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", filename))
	require.NoError(t, err, "loadTestData: read %s", filename)
	return b
}

// decodeJSONArray parses a JSON array file into []interface{} so the
// resulting slice can be passed straight to postImportJSON (which
// re-marshals into the request body).
func decodeJSONArray(t testing.TB, raw []byte) []interface{} {
	t.Helper()
	var out []interface{}
	require.NoError(t, json.Unmarshal(raw, &out),
		"decodeJSONArray: parse; raw=%s", string(raw))
	return out
}

// decodeBulkImportResult parses a 200/207/422 response body into a
// BulkImportResult value.
func decodeBulkImportResult(t testing.TB, body []byte) api.BulkImportResult {
	t.Helper()
	var r api.BulkImportResult
	require.NoError(t, json.Unmarshal(body, &r),
		"decodeBulkImportResult: unmarshal; body=%s", string(body))
	return r
}

// decodeImportJob parses a 200 GET response body into an ImportJob value.
func decodeImportJob(t testing.TB, body []byte) api.ImportJob {
	t.Helper()
	var j api.ImportJob
	require.NoError(t, json.Unmarshal(body, &j),
		"decodeImportJob: unmarshal; body=%s", string(body))
	return j
}

// extractFirstSucceededID returns the first succeeded[] UUID from a
// 200/207 response. Asserts at least one row succeeded.
//
//nolint:unused // retained for future tests that need the persisted
// UUID for follow-up GET assertions; current Plan 05-07 tests assert
// shape via decodeBulkImportResult directly.
func extractFirstSucceededID(t testing.TB, body []byte) uuid.UUID {
	t.Helper()
	r := decodeBulkImportResult(t, body)
	require.NotEmpty(t, r.Succeeded, "extractFirstSucceededID: succeeded[] empty; body=%s", string(body))
	return uuid.UUID(r.Succeeded[0])
}

// ---------------------------------------------------------------------------
// Wave 5 seed helpers — bypass the HTTP layer for pre-test setup so
// tests can verify Phase 5 merge semantics (D5-18) and FK probes
// (D-76) without coupling to the catalog handler API.
// ---------------------------------------------------------------------------

// seedSkillForImports inserts a skills row directly via raw SQL so
// agent-with-skills tests have a referent before POSTing.
func seedSkillForImports(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, code, name, skillType string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx,
		`INSERT INTO skills (id, org_id, code, name, skill_type, enabled)
		 VALUES ($1, $2, $3, $4, $5, true)`,
		id, orgID, code, name, skillType,
	)
	require.NoError(t, err, "seedSkillForImports: INSERT code=%s", code)
	return id
}

// seedQueueForImports inserts a queues row directly via raw SQL so
// channel-import tests can probe the default_queue_code FK (D-76).
func seedQueueForImports(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, code, name string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx,
		`INSERT INTO queues (id, org_id, code, name, channel_types, priority, acw_sec, enabled)
		 VALUES ($1, $2, $3, $4, '{voice,chat}', 1, 30, true)`,
		id, orgID, code, name,
	)
	require.NoError(t, err, "seedQueueForImports: INSERT code=%s", code)
	return id
}

// seedAgentForImports inserts an agents row + agent_states row directly
// via raw SQL. Used by re-import tests to assert state-machine
// preservation on subsequent imports (Hazard 7 / Pitfall 3).
func seedAgentForImports(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, code, name, email, initialStatus string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx,
		`INSERT INTO agents (id, org_id, code, name, email, enabled)
		 VALUES ($1, $2, $3, $4, $5, true)`,
		id, orgID, code, name, email,
	)
	require.NoError(t, err, "seedAgentForImports: INSERT agent code=%s", code)
	_, err = pool.Exec(ctx,
		`INSERT INTO agent_states (agent_id, org_id, status, state_version)
		 VALUES ($1, $2, $3, 1)`,
		id, orgID, initialStatus,
	)
	require.NoError(t, err, "seedAgentForImports: INSERT agent_state code=%s", code)
	return id
}

// seedAgentSkillAssignment binds an agent to a skill at a proficiency.
// Used by the skill-merge test to seed an existing assignment that the
// import should LEAVE INTACT (D5-18 MERGE semantics).
func seedAgentSkillAssignment(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID, agentID, skillID uuid.UUID, proficiency int) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO agent_skills (agent_id, skill_id, org_id, proficiency)
		 VALUES ($1, $2, $3, $4)`,
		agentID, skillID, orgID, proficiency,
	)
	require.NoError(t, err, "seedAgentSkillAssignment: INSERT")
}

// fetchAgentByCode returns (id, name) for an agent by (orgID, code) for
// post-import verification.
func fetchAgentByCode(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, code string) (uuid.UUID, string) {
	t.Helper()
	var id uuid.UUID
	var name string
	err := pool.QueryRow(ctx,
		`SELECT id, name FROM agents WHERE org_id = $1 AND code = $2`,
		orgID, code,
	).Scan(&id, &name)
	require.NoError(t, err, "fetchAgentByCode: %s", code)
	return id, name
}

// fetchAgentStateStatusByOrg returns the status of the agent_states row
// for (orgID, agentID). Wave 5 variant taking pool+orgID explicitly so
// cross-org isolation tests (in test/isolation/) and same-org handler
// tests can share the same primitive. The Wave 3 row_agent_test.go
// helper fetchAgentStateStatus(th, agentID) covers the th-bound path.
func fetchAgentStateStatusByOrg(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID, agentID uuid.UUID) string {
	t.Helper()
	var status string
	err := pool.QueryRow(ctx,
		`SELECT status FROM agent_states WHERE org_id = $1 AND agent_id = $2`,
		orgID, agentID,
	).Scan(&status)
	require.NoError(t, err, "fetchAgentStateStatusByOrg")
	return status
}

// fetchAgentSkillCount returns the count of agent_skills rows for an
// agent. Used by the skill-merge test to assert no rows were dropped.
func fetchAgentSkillCount(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID, agentID uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(ctx,
		`SELECT count(*) FROM agent_skills WHERE org_id = $1 AND agent_id = $2`,
		orgID, agentID,
	).Scan(&count)
	require.NoError(t, err, "fetchAgentSkillCount")
	return count
}

// fetchAgentSkillProficiency returns the proficiency for a specific
// (agent, skill) pair. Used by the import-wins-on-conflict test (D5-19).
func fetchAgentSkillProficiency(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID, agentID, skillID uuid.UUID) int {
	t.Helper()
	var prof int
	err := pool.QueryRow(ctx,
		`SELECT proficiency FROM agent_skills WHERE org_id = $1 AND agent_id = $2 AND skill_id = $3`,
		orgID, agentID, skillID,
	).Scan(&prof)
	require.NoError(t, err, "fetchAgentSkillProficiency")
	return prof
}

// fetchImportJob returns the persisted import_jobs row for assertions.
func fetchImportJob(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID, jobID uuid.UUID) (string, int, int, int, []byte) {
	t.Helper()
	var status string
	var total, succeeded, failed int
	var errorsRaw []byte
	err := pool.QueryRow(ctx,
		`SELECT status, total_rows, succeeded_rows, failed_rows, errors
		   FROM import_jobs WHERE id = $1 AND org_id = $2`,
		jobID, orgID,
	).Scan(&status, &total, &succeeded, &failed, &errorsRaw)
	require.NoError(t, err, "fetchImportJob")
	return status, total, succeeded, failed, errorsRaw
}

// countImportJobsByOrg returns the number of import_jobs rows for an
// org. Used by the no-idempotency-key double-run test to verify TWO
// rows exist (vs idempotent replay's single row).
func countImportJobsByOrg(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(ctx,
		`SELECT count(*) FROM import_jobs WHERE org_id = $1`,
		orgID,
	).Scan(&count)
	require.NoError(t, err, "countImportJobsByOrg")
	return count
}

// extractJobIDFromCreatedRow extracts the last import_jobs.id created
// for an org. Used by some tests when the response body does not carry
// the job ID (e.g., the v0.1 BulkImportResult does not expose `id`).
func extractLatestJobID(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT id FROM import_jobs WHERE org_id = $1
		   ORDER BY created_at DESC, id DESC LIMIT 1`,
		orgID,
	).Scan(&id)
	require.NoError(t, err, "extractLatestJobID")
	return id
}

// _ = fmt.Sprint keep-alive to defend against an inadvertent fmt import drop.
var _ = fmt.Sprint

// _ = openapi_types.UUID(uuid.Nil) keep-alive for the openapi types import
// (used by tests that build BulkImportCatalogParams directly).
var _ = openapi_types.UUID(uuid.Nil)
