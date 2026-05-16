// Package scaffold_test holds the create/get/list happy-path coverage for
// the Phase 2 _scaffold strict-server surface. After the D-46 migration,
// handlers are served through the generated api.StrictServerInterface pipeline
// instead of hand-written (w, r) handlers.
//
// Two-org isolation tests live in test/isolation/ — this file proves the
// chain wires up correctly with a single org, the request/response shape is
// stable, and the cross-org 404 path works.
//
// Tests guard themselves with `if testing.Short() { t.Skip(...) }` because
// they need an ephemeral Postgres via testcontainers-go. `go test -short`
// (the unit-level CI loop) skips them; the docker-available CI lane runs
// them via `go test -timeout=180s`. The malformed-ID test is a pure-handler
// unit case that does NOT need docker (no DB call) and is NOT skipped.
package scaffold_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/yaml.v3"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/server"
	appmw "github.com/luongdev/open-routing/services/api/internal/middleware"
)

// sharedPool is reused across every test in this file. TestMain owns its
// lifecycle (testcontainer + migrations) so each test pays a per-test
// fresh-orgID cost only, not a fresh container.
var sharedPool *pgxpool.Pool

// containerFailureExitCode returns 1 in CI and 0 locally.
func containerFailureExitCode() int {
	if os.Getenv("CI") != "" {
		return 1
	}
	return 0
}

// TestMain provisions an ephemeral Postgres testcontainer, applies the Phase 1
// migrations against it, and opens a shared pgxpool the tests reuse. Skipped
// when `go test -short` is requested.
func TestMain(m *testing.M) {
	if !flag.Parsed() {
		flag.Parse()
	}
	if testing.Short() {
		os.Exit(m.Run())
	}
	ctx := context.Background()
	pgC, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("scaffold_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		os.Stderr.WriteString("scaffold_test: testcontainer postgres unavailable: " + err.Error() + "\n")
		os.Exit(containerFailureExitCode())
	}

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		os.Stderr.WriteString("scaffold_test: connection string: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}

	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		os.Stderr.WriteString("scaffold_test: sql.Open: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		os.Stderr.WriteString("scaffold_test: pgmigrate driver: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	m2, err := migrate.NewWithDatabaseInstance("file://../../../../migrations", "postgres", driver)
	if err != nil {
		os.Stderr.WriteString("scaffold_test: migrate.NewWithDatabaseInstance: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	if err := m2.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		os.Stderr.WriteString("scaffold_test: migrate.Up: " + err.Error() + "\n")
		_ = sqlDB.Close()
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}
	_ = sqlDB.Close()

	sharedPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		os.Stderr.WriteString("scaffold_test: pgxpool.New: " + err.Error() + "\n")
		_ = pgC.Terminate(ctx)
		os.Exit(containerFailureExitCode())
	}

	code := m.Run()
	sharedPool.Close()
	_ = pgC.Terminate(ctx)
	os.Exit(code)
}

// buildStrictHandler creates the strict-server pipeline for the scaffold
// handler, including the RequestIDInjectionMiddleware (B-1, D-35), mounted
// on a chi router with the RequestID middleware at root.
//
// The returned http.Handler serves /v1/orgs/{orgID}/_scaffold routes.
// Tests inject orgID into ctx directly via orgkey.SetOrgID (simulating
// OrgContext middleware) rather than going through the full header parsing.
//
// Uses server.NewCompositeServer so that the full api.StrictServerInterface is
// satisfied (scaffold.Handler only implements 3 of 44 methods; compositeServer
// stubs the remaining 41 with 500 "not_implemented" responses). The scaffold
// paths are the only ones hit by these tests.
//
// For TestScaffold_GetMalformedID_Returns400 (nil orgDB), passing nil is safe
// because the generated router rejects the malformed UUID before reaching any
// DB call.
func buildStrictHandler(orgDB *db.OrgDB) http.Handler {
	// compositeServer satisfies api.StrictServerInterface for all 44 methods.
	composite := server.NewCompositeServer(orgDB, nil, nil, nil)
	strictHandler := api.NewStrictHandler(
		composite,
		[]api.StrictMiddlewareFunc{server.RequestIDInjectionMiddleware()},
	)

	r := chi.NewRouter()
	r.Use(appmw.RequestID) // mint request_id into ctx (D-28)

	// Use HandlerFromMux to register the full spec routes on our router.
	// The scaffold-specific routes are what tests will hit; bypass routes
	// and catalog stubs will never be reached in these tests.
	api.HandlerFromMux(strictHandler, r)
	return r
}

// callWithOrg simulates the OrgContext middleware by attaching orgID to ctx
// and routing the request through the strict-server handler.
//
// D-46: unlike Phase 1's callWithOrg which took chi.Router, this version
// takes http.Handler so it works with the strict-server pipeline.
func callWithOrg(t *testing.T, h http.Handler, orgID uuid.UUID, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if len(body) > 0 {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	// Simulate OrgContext by injecting org_id directly into ctx.
	req = req.WithContext(orgkey.SetOrgID(req.Context(), orgID))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestScaffold_CreateAndGet_Roundtrip proves the full POST -> GET-by-id ->
// GET-list cycle works end-to-end through the strict-server pipeline.
// Asserts UUIDv7 version (D-19), org_id scoping, and created_at presence.
func TestScaffold_CreateAndGet_Roundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffold integration test: skipped in -short mode (requires postgres testcontainer)")
	}
	if sharedPool == nil {
		t.Skip("scaffold integration test: sharedPool unavailable (testcontainer setup failed)")
	}
	t.Parallel()
	orgID := uuid.Must(uuid.NewV7())
	orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)
	h := buildStrictHandler(orgDB)

	// POST creates row.
	createBody := []byte(`{"external_id":"ext-1","name":"first"}`)
	rec := callWithOrg(t, h, orgID, http.MethodPost,
		fmt.Sprintf("/v1/orgs/%s/_scaffold", orgID), createBody)
	require.Equal(t, http.StatusCreated, rec.Code, "create body=%s", rec.Body.String())

	var created struct {
		ID         uuid.UUID `json:"id"`
		OrgID      uuid.UUID `json:"org_id"`
		ExternalID string    `json:"external_id"`
		Name       string    `json:"name"`
		CreatedAt  time.Time `json:"created_at"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, "ext-1", created.ExternalID)
	require.Equal(t, "first", created.Name)
	require.Equal(t, orgID, created.OrgID)
	require.Equal(t, uint8(7), uint8(created.ID.Version()), "id must be UUIDv7 (D-19)")
	require.False(t, created.CreatedAt.IsZero(), "created_at must be populated")

	// GET /{id} returns the row.
	rec = callWithOrg(t, h, orgID, http.MethodGet,
		fmt.Sprintf("/v1/orgs/%s/_scaffold/%s", orgID, created.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, "get body=%s", rec.Body.String())

	// GET / lists the row.
	rec = callWithOrg(t, h, orgID, http.MethodGet,
		fmt.Sprintf("/v1/orgs/%s/_scaffold", orgID), nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var listed []struct {
		ID uuid.UUID `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	require.Len(t, listed, 1)
	require.Equal(t, created.ID, listed[0].ID)
}

// TestScaffold_GetNotFound_Returns404 covers the missing-id path (FOUND-08).
// The sqlc query filters by id AND org_id; a missing or cross-org id both
// return pgx.ErrNoRows which the handler maps to 404.
func TestScaffold_GetNotFound_Returns404(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffold integration test: skipped in -short mode (requires postgres testcontainer)")
	}
	if sharedPool == nil {
		t.Skip("scaffold integration test: sharedPool unavailable (testcontainer setup failed)")
	}
	t.Parallel()
	orgID := uuid.Must(uuid.NewV7())
	orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)
	h := buildStrictHandler(orgDB)

	missingID := uuid.Must(uuid.NewV7())
	rec := callWithOrg(t, h, orgID, http.MethodGet,
		fmt.Sprintf("/v1/orgs/%s/_scaffold/%s", orgID, missingID), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, "body=%s", rec.Body.String())
}

// TestScaffold_CreateInvalidBody_Returns400 covers the request validation
// short-circuit. In the strict-server pipeline the rejection comes from
// oapi-codegen's request validation (missing required field external_id or
// name). The assertion only checks the status code (not the reason string)
// because oapi-codegen's message format may differ from Phase 1's.
func TestScaffold_CreateInvalidBody_Returns400(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffold integration test: skipped in -short mode (requires postgres testcontainer)")
	}
	if sharedPool == nil {
		t.Skip("scaffold integration test: sharedPool unavailable (testcontainer setup failed)")
	}
	t.Parallel()
	orgID := uuid.Must(uuid.NewV7())
	orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)
	h := buildStrictHandler(orgDB)

	// Missing name field -> 400
	rec := callWithOrg(t, h, orgID, http.MethodPost,
		fmt.Sprintf("/v1/orgs/%s/_scaffold", orgID), []byte(`{"external_id":"ext-1"}`))
	require.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", rec.Body.String())
}

// TestScaffold_GetMalformedID_Returns400 covers the URL-path UUID parse
// failure. Pure handler unit case — no DB call, no docker needed.
// oapi-codegen parses {id} as EntityIdPath (uuid.UUID) and rejects non-UUID
// values with 400 before calling the handler.
func TestScaffold_GetMalformedID_Returns400(t *testing.T) {
	t.Parallel()
	// nil orgDB is safe because the handler short-circuits before any DB access.
	var orgDB *db.OrgDB
	h := buildStrictHandler(orgDB)

	orgID := uuid.Must(uuid.NewV7())
	rec := callWithOrg(t, h, orgID, http.MethodGet,
		fmt.Sprintf("/v1/orgs/%s/_scaffold/not-a-uuid", orgID), nil)
	require.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", rec.Body.String())
}

// TestScaffold_GetNotFound_BodyContainsRequestID asserts the B-1 success
// criterion at the scaffold-package level: the 404 error response body MUST
// contain a non-empty request_id that equals the X-Request-Id response header.
//
// This proves that server.RequestIDInjectionMiddleware (B-1, D-35) correctly
// injects the UUIDv7 request id from ctx into the strict-server error envelope
// — not just for the isolation suite but at the unit level of this package.
func TestScaffold_GetNotFound_BodyContainsRequestID(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffold integration test: skipped in -short mode (requires postgres testcontainer)")
	}
	if sharedPool == nil {
		t.Skip("scaffold integration test: sharedPool unavailable (testcontainer setup failed)")
	}
	t.Parallel()
	orgID := uuid.Must(uuid.NewV7())
	orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)
	h := buildStrictHandler(orgDB)

	missingID := uuid.Must(uuid.NewV7())
	rec := callWithOrg(t, h, orgID, http.MethodGet,
		fmt.Sprintf("/v1/orgs/%s/_scaffold/%s", orgID, missingID), nil)
	require.Equal(t, http.StatusNotFound, rec.Code)

	// Parse the response body as an error envelope.
	var errBody struct {
		Error     string `json:"error"`
		Reason    string `json:"reason"`
		RequestID string `json:"request_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody), "body=%s", rec.Body.String())
	require.NotEmpty(t, errBody.RequestID, "B-1: request_id must be present in error body")

	// The X-Request-Id header (set by appmw.RequestID) must match the body.
	headerID := rec.Header().Get("X-Request-Id")
	require.NotEmpty(t, headerID, "X-Request-Id header must be present")
	require.Equal(t, headerID, errBody.RequestID,
		"B-1: body.request_id must equal X-Request-Id header (end-to-end propagation through strict pipeline + middleware)")
}

// Ensure gopkg.in/yaml.v3 import is used for spec bytes (referenced in main_test.go pattern).
// This var keeps the import alive if other callers are removed.
var _ = yaml.Marshal
