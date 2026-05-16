// Package catalog implements the v0.1 catalog CRUD layer (CAT-01..CAT-11).
//
// The package follows D-68 / D-69 / D-71:
//   - D-68: one .go per entity in the same package (`catalog`). The 6
//     CRUD entities each own a file (agents.go, skills.go, queues.go,
//     channels.go, adapters.go, break_reasons.go) plus the agent_skills
//     join (agent_skills.go).
//   - D-69: a single `*Handlers` value implements api.StrictServerInterface
//     for every catalog method — there is NO composite server. Plans
//     03-06..03-09 REPLACE the placeholder bodies created here with the
//     real implementations.
//   - D-71: hybrid constructor — required deps in a typed struct
//     (compile-error if a field is missing), optional knobs (Clock) via
//     functional Options.
//
// Plans 03-06..03-09 fill in per-entity handlers; Plan 03-10 wires
// catalog.New(deps) into main.go as the production StrictServerInterface.
//
// Phase 4 (agent state-machine) and Phase 5 (bulk import) endpoints are
// 501-stubbed via notimpl.go and remain stubbed until those phases land —
// D-70 forward-compat via struct embedding swaps them in later.
package catalog

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db"
)

// Deps bundles the required runtime dependencies for the catalog handler
// set. Every field is REQUIRED; New does NOT validate non-nil — a zero
// field panics at first call that dereferences it. This is the D-71
// "hybrid constructor" required-fields side.
type Deps struct {
	// OrgDB is the per-request *db.OrgDB factory used by every handler
	// to obtain an org-scoped connection. The catalog handler pulls
	// orgDB from request context (set by middleware) rather than holding
	// a single OrgDB on the struct, but Deps carries OrgDB so tests +
	// future helpers can construct query objects from the pool.
	OrgDB *db.OrgDB

	// Pool is the pgxpool used for read-only queries that don't need
	// the OrgDB injection layer (e.g., testutil bring-up, schema
	// migrations probe). Production handlers use OrgDB.
	Pool *pgxpool.Pool

	// Cache is the Redis-backed entity cache (CAT-11). Handlers call
	// cache.GetOrSet for GETs and cache.Del for writes; list endpoints
	// (CAT-09 / CAT-10) bypass the cache per D-49.
	Cache *cache.Cache

	// Logger is the slog logger threaded into every handler for
	// per-request structured logging (D-57 cache-observability via
	// slog attrs).
	Logger *slog.Logger
}

// Option mutates a *Handlers after construction. In v0.1 only WithClock
// is exposed (D-71) — future tunables (cache TTL override, page-size
// default override) ride this seam without breaking the New() signature.
type Option func(*Handlers)

// WithClock overrides the clock used for created_at / updated_at minting
// and UUIDv7 timestamp tests. Default is time.Now. Tests inject a fixed
// clock to make UUIDv7 ordering + created_at assertions deterministic.
func WithClock(c func() time.Time) Option {
	return func(h *Handlers) { h.clock = c }
}

// Handlers implements api.StrictServerInterface for every catalog
// endpoint. Per-method bodies live in the entity-specific files; this
// file holds only the constructor and shared state.
//
// Fields are unexported — the package-internal entity files reach in
// directly (e.g., h.deps.Cache, h.clock()), so the API surface stays
// small (`New`, `WithClock`, plus the StrictServerInterface methods).
type Handlers struct {
	deps  Deps
	clock func() time.Time
}

// Compile-time guarantee that *Handlers satisfies the generated
// StrictServerInterface. If this line fails to compile, the package is
// missing a method that the OpenAPI spec defines — look at the diff
// between server.gen.go's StrictServerInterface and the catalog
// package's method set.
var _ api.StrictServerInterface = (*Handlers)(nil)

// New constructs a *Handlers value. Deps fields MUST be non-nil — New
// does not validate, but the first method call that dereferences a nil
// field will panic. Optional knobs apply via Option funcs (D-71).
//
// Usage (Plan 03-10 wiring):
//
//	h := catalog.New(catalog.Deps{
//	    OrgDB:  orgDB,
//	    Pool:   pool,
//	    Cache:  c,
//	    Logger: logger,
//	})
//	mux := api.HandlerWithOptions(
//	    api.NewStrictHandler(h, []api.StrictMiddlewareFunc{
//	        server.RequestIDInjectionMiddleware(),
//	    }),
//	    api.ChiServerOptions{},
//	)
func New(deps Deps, opts ...Option) *Handlers {
	h := &Handlers{deps: deps, clock: time.Now}
	for _, opt := range opts {
		opt(h)
	}
	return h
}
