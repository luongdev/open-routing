package state

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"

	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db"
)

const (
	defaultSweepInterval = 30 * time.Second // D-81 safety sweep cadence
	defaultWrapUpDur     = 60 * time.Second // v0.1 hardcoded; per-org override deferred
	stateCacheTTL        = 60 * time.Second // D-86 matches catalog cache TTL
	jitterMaxMs          = 100              // D-87 ±100ms thundering-herd guard
)

// Deps bundles required runtime dependencies. Every field REQUIRED;
// New does NOT validate non-nil — a zero field panics at first
// dereference (D-71 hybrid-constructor contract).
type Deps struct {
	OrgDB          *db.OrgDB
	Cache          *cache.Cache
	Logger         *slog.Logger
	WrapUpDuration time.Duration // 0 → defaultWrapUpDur
}

type Option func(*Server)

// WithClock injects a clockwork.Clock implementation. Tests use
// clockwork.NewFakeClock() so AfterFunc fires deterministically on
// fakeClock.Advance(d).
func WithClock(c clockwork.Clock) Option { return func(s *Server) { s.clock = c } }

// WithSweepInterval overrides the 30s safety-sweep cadence for tests.
func WithSweepInterval(d time.Duration) Option { return func(s *Server) { s.sweepInterval = d } }

// Server implements the SUBSET of api.StrictServerInterface dealing with
// agent-state operations. The full interface is satisfied by the
// *ApiHandlers composite in cmd/api/main.go (D-89 — Wave 4 wires this).
//
// Pitfall 1: the type is `Server` (NOT `Handlers`) because Go rejects
// `type ApiHandlers struct { *catalog.Handlers; *state.Handlers }`
// with "duplicate field name." Renaming here keeps Wave 4 wiring trivial.
type Server struct {
	deps          Deps
	clock         clockwork.Clock
	sweepInterval time.Duration

	timers *timersRegistry

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
	startMu sync.Mutex
}

// timersRegistry is the in-memory AfterFunc handle map keyed by
// agent_id. RWMutex over sync.Map per Claude's discretion #2: Stop()
// iterates the whole map; mutex makes the iteration semantics explicit.
// Wave 3 (04-04-PLAN.md) implements registry methods + sweeper.
type timersRegistry struct {
	mu sync.RWMutex
	t  map[uuid.UUID]clockwork.Timer
}

// New constructs a *Server. Required deps validate at first dereference;
// optional knobs apply via Option funcs. Defaults: real clock, 30s
// sweep interval, 60s WrapUp duration.
func New(deps Deps, opts ...Option) *Server {
	s := &Server{
		deps:          deps,
		clock:         clockwork.NewRealClock(),
		sweepInterval: defaultSweepInterval,
		timers:        &timersRegistry{t: make(map[uuid.UUID]clockwork.Timer)},
	}
	if deps.WrapUpDuration == 0 {
		s.deps.WrapUpDuration = defaultWrapUpDur
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start is the lifecycle hook called by cmd/api/main.go before serving.
// Wave 1 stubs to no-op; Wave 3 implements synchronous startup sweep +
// safety-sweep goroutine. Idempotent via startMu + started.
func (s *Server) Start(ctx context.Context) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if s.started {
		return nil
	}
	s.started = true
	s.ctx, s.cancel = context.WithCancel(ctx)
	return nil
}

// Stop is called via defer in main.go shutdown path. Wave 3 cancels ctx,
// waits for sweeper drain, cancels timers. Idempotent.
func (s *Server) Stop() {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if !s.started {
		return
	}
	s.started = false
	if s.cancel != nil {
		s.cancel()
	}
}

// cacheKeyFor centralises the agent_state cache key namespace (D-86).
// Singular "agent_state" (NOT plural) per Pitfall 7. Every cache.GetOrSet
// + cache.Del in agent_states.go routes through here.
func (s *Server) cacheKeyFor(orgID, agentID uuid.UUID) string {
	return cache.Key(orgID, "agent_state", agentID)
}
