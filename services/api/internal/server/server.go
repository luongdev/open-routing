// Package server owns the chi router factory that fronts the open-routing
// API binary. health.go in this package provides the bypass-path handlers;
// server.go assembles the LOCKED middleware chain and the route table.
//
// The chain order is contract: Recoverer first (catches panics from
// every later middleware including RequestID), then RequestID at root
// (UUIDv7 per D-28), then the bypass routes (/healthz /readyz /metrics —
// D-21), then the /v1 sub-router scoped with OrgContext (D-21 scopes
// OrgContext to inside the Route closure so the bypass paths stay open).
// Pattern S6 + Pitfall 5 require the OTel HTTP root wrap to happen AFTER
// all mux.Use calls — that wrap lives in cmd/api/main.go, never inside
// NewMux.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	appmw "github.com/luongdev/open-routing/services/api/internal/middleware"
	"github.com/luongdev/open-routing/services/api/internal/scaffold"
)

// Deps bundles every runtime dependency the API mux needs. cmd/api/main.go
// constructs one of these (after wiring OTel, pool, redis, orgDB) and hands
// it to NewMux. The Deps struct is the seam between binary-level wiring and
// router-level wiring — tests can supply fakes here (Plan 07's integration
// suite supplies a real testcontainer pool + redis).
type Deps struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	OrgDB  *db.OrgDB
	Config *config.Config
}

// NewMux constructs the chi router with the LOCKED middleware chain order:
//
//  1. chi stdlib Recoverer  — catches panics from every later middleware
//     and from handlers themselves; converts them to 500 responses.
//     MUST be first so it surrounds RequestID (uuid.Must can panic on
//     entropy exhaustion — Recoverer is the safety net).
//
//  2. custom RequestID — UUIDv7 per D-28; sets the X-Request-Id response
//     header before the request body is touched, so even panicking
//     handlers produce a correlatable response.
//
//  3. Bypass routes at root:
//     GET /healthz   liveness, always 200
//     GET /readyz    deep readiness (pings pgxpool + redis + schema_migrations)
//     GET /metrics   Phase 1 stub (Open Question #4)
//     Per D-21 these MUST live outside the /v1 closure so they are reachable
//     without an X-Org-Id header. The chi.Route closure scope is the
//     enforcement seam — never call r.Use(appmw.OrgContext) at root.
//
//  4. /v1 sub-router with OrgContext inside the Route closure (D-21):
//     POST /v1/orgs/{org_id}/_scaffold
//     GET  /v1/orgs/{org_id}/_scaffold
//     GET  /v1/orgs/{org_id}/_scaffold/{id}
//     The {org_id} URL parameter is intentionally unread by handlers —
//     handlers read the authoritative org_id from ctx (FOUND-05). The URL
//     parameter exists for REST-friendly URLs and to make logs/traces
//     filterable.
//
// After NewMux returns, cmd/api/main.go wraps the returned handler with
// the OTel HTTP root handler so EVERY request (including bypass paths)
// carries a root server span. Pattern S6 + Pitfall 5: the wrap is AFTER all
// mux.Use() calls so the span exists when middleware runs and the org_id
// attribute injection in OrgContext lands on the correct span.
func NewMux(deps *Deps) http.Handler {
	r := chi.NewRouter()

	// (1) Recoverer first.
	r.Use(chimw.Recoverer)
	// (2) Our UUIDv7 RequestID — overrides chi's host/N-K counter (D-28).
	r.Use(appmw.RequestID)

	// (3) Bypass routes — D-21 says they MUST NOT carry OrgContext.
	r.Get("/healthz", LiveHandler())
	r.Get("/readyz", ReadyzHandler(deps.Pool, deps.Redis))
	r.Get("/metrics", MetricsHandler())

	// (4) /v1 sub-router. Per chi v5 semantics, middleware added via
	// v1.Use applies ONLY inside this Route block, so /healthz et al
	// stay reachable without a header (D-21).
	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(appmw.OrgContext)
		v1.Mount("/orgs/{org_id}/_scaffold", scaffold.Routes(deps.OrgDB))
	})

	return r
}
