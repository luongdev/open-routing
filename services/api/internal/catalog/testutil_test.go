// testutil_test.go — per-entity test scaffolding shared across the
// catalog suite (D-73). Plan 03-06 fleshes out the Plan 03-05 skeleton
// per the canonical pattern that Plans 03-07/08/09 will copy.
//
// Why `_test.go` suffix: the Go build tool excludes _test.go files from
// production builds, so miniredis, testify, and testcontainers never end
// up in the shipped binary (Codex C7 iter 3 — confirmed Wave 3 review
// found no testing libs in production files).
//
// Why `package catalog` (not `catalog_test`): the entity test files
// (agents_test.go) also use `package catalog` so they can call the
// unexported helpers below without re-exporting them. This is the
// idiomatic Go convention when test helpers are package-internal.
//
// Wiring:
//   - sharedPool is initialised by main_test.go's TestMain (one Postgres
//     testcontainer per `go test ./internal/catalog/...` invocation).
//   - newTestHandlers(t) constructs ONE catalog.Handlers + miniredis +
//     httptest server PER TEST so test isolation is total (no leaking
//     cache state, no leaking orgID).
//   - cleanCatalogTables(t, ctx) TRUNCATEs every catalog table CASCADE
//     before each test starts so the suite is order-independent.
//   - httpPOST / httpGET / httpPATCH / httpDELETE marshal JSON, set
//     X-Org-Id + Content-Type, and return (response, body) for the
//     caller to parse per its expected shape.
package catalog

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

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/server"
)

// sharedPool is the package-level pgxpool reused across every entity
// _test.go in the catalog package (D-73). Assigned in main_test.go's
// TestMain after the Postgres testcontainer is up + migrations applied.
// nil when -short is set or Docker is unavailable.
var sharedPool *pgxpool.Pool

// TestHandlers bundles the per-test fixture so test bodies can read
// orgID, hit the httptest server, and inspect the miniredis fixture
// directly to assert cache hit/miss state (CAT-11).
//
// The Pool field aliases sharedPool — kept on the struct so tests that
// also touch the DB directly (e.g., seed test data via INSERT) don't
// have to reach for the package-level var.
type TestHandlers struct {
	H         *Handlers
	Miniredis *miniredis.Miniredis
	Pool      *pgxpool.Pool
	OrgID     uuid.UUID
	HTTP      *httptest.Server
	// rdb is the *redis.Client wired into h.deps.Cache. Tests that need
	// to call rdb.Del / rdb.Get directly (rare) can reach for this; the
	// preferred assertion surface is Miniredis.Exists / Miniredis.Get.
	rdb *redis.Client
}

// newTestHandlers constructs ONE fixture per test (D-73):
//   - one miniredis instance (fresh empty cache state)
//   - one OrgDB wrapping sharedPool with the SQLChecker in panic mode
//     (mirroring the FOUND-04 dev-test contract — a missing org_id at
//     the SQL layer panics immediately rather than silently passing)
//   - one catalog.Handlers wired with miniredis + OrgDB + slog
//   - one httptest.Server fronting the PRODUCTION chi mux + middleware
//     chain (server.NewMux) so the test exercises the same code path
//     the binary ships
//
// Skips the calling test when sharedPool == nil (TestMain skipped
// bring-up due to Docker unavailability or -short).
func newTestHandlers(t testing.TB) *TestHandlers {
	t.Helper()
	if sharedPool == nil {
		t.Skip("catalog: sharedPool nil — Docker testcontainer unavailable (run without -short)")
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

	// 5. catalog.Handlers — the StrictServerInterface impl under test.
	h := New(Deps{
		OrgDB:  orgDB,
		Pool:   sharedPool,
		Cache:  c,
		Logger: logger,
	})

	// 6. server.NewMux with catalog.Handlers as the StrictHandlers field
	//    (Plan 03-06 wires the real impl in for testing while production
	//    still uses Wave0TempStubs until Plan 03-10 rolls the swap into
	//    cmd/api/main.go). Build the spec bytes the same way main.go
	//    does — from the embedded spec.
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
		StrictHandlers: h, // catalog.Handlers satisfies api.StrictServerInterface (D-69).
		SpecBytes:      specBytes,
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// 7. Per-test orgID. UUIDv7 because OrgContext middleware rejects
	//    UUIDv4 with 400 invalid_org_id/uuidv7_required (D-19).
	orgID := uuid.Must(uuid.NewV7())

	return &TestHandlers{
		H:         h,
		Miniredis: mr,
		Pool:      sharedPool,
		OrgID:     orgID,
		HTTP:      srv,
		rdb:       rdb,
	}
}

// cleanCatalogTables TRUNCATEs every catalog table CASCADE before a
// test starts so the suite is order-independent. The CASCADE clause
// drops dependent rows in the junction table (agent_skills) without
// requiring the caller to know the FK graph.
//
// The DELETE order matches the migration's REFERENCES graph; with
// CASCADE it's redundant but documents the dependency direction for
// future readers.
func cleanCatalogTables(t testing.TB, ctx context.Context) {
	t.Helper()
	if sharedPool == nil {
		return
	}
	// Single round-trip; CASCADE handles the FK graph.
	_, err := sharedPool.Exec(ctx, `TRUNCATE TABLE agent_skills, break_reasons, adapters, channels, queues, skills, agents CASCADE`)
	require.NoError(t, err, "cleanCatalogTables: TRUNCATE failed")
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------
// Each helper marshals the request, sets X-Org-Id + Content-Type,
// dispatches the request, reads the response body, and returns
// (*http.Response, []byte) so the caller can parse per its expected
// shape (typically `json.Unmarshal(body, &want)` where want matches
// the operation-specific response wrapper from api/server.gen.go).
//
// The X-Org-Id header is set from the orgID arg (NOT from a struct
// field) so individual tests can hit the same httptest server with
// a DIFFERENT org for cross-org isolation probes (FOUND-08).

func httpPOST(t testing.TB, srv *httptest.Server, orgID uuid.UUID, path string, body any) (*http.Response, []byte) {
	t.Helper()
	return doJSON(t, srv, http.MethodPost, orgID, path, nil, body)
}

func httpGET(t testing.TB, srv *httptest.Server, orgID uuid.UUID, path string, q url.Values) (*http.Response, []byte) {
	t.Helper()
	return doJSON(t, srv, http.MethodGet, orgID, path, q, nil)
}

func httpPATCH(t testing.TB, srv *httptest.Server, orgID uuid.UUID, path string, body any) (*http.Response, []byte) {
	t.Helper()
	return doJSON(t, srv, http.MethodPatch, orgID, path, nil, body)
}

func httpDELETE(t testing.TB, srv *httptest.Server, orgID uuid.UUID, path string) (*http.Response, []byte) {
	t.Helper()
	return doJSON(t, srv, http.MethodDelete, orgID, path, nil, nil)
}

// doJSON is the shared body of the four helpers above. Marshals the
// body to JSON when non-nil, appends q to the URL if non-empty, sets
// X-Org-Id + Content-Type, dispatches via srv.Client(), and reads the
// response body. Returns (*http.Response, []byte) — caller is
// responsible for parsing the body per its expected shape.
//
// The httptest server's Client() returns an *http.Client whose Transport
// is wired to dial the test server directly; net/http and DNS are
// bypassed. The response body is fully read + closed so callers can
// inspect body bytes without leaking the connection.
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
