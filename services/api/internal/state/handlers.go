package state

import (
	"context"
	"fmt"
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
	// jitterMaxMs: D-87 ±100ms thundering-herd guard deferred to v0.2 —
	// no v0.1 caller for system-initiated Engaged→WrapUp (D-82).
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

// Start is called by cmd/api/main.go before serving. Runs a synchronous
// startup sweep (D-95 — guarantees no stuck WrapUp survives a restart)
// BEFORE spawning the 30s safety-sweep goroutine. Idempotent via startMu.
func (s *Server) Start(ctx context.Context) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if s.started {
		return nil
	}
	s.started = true
	s.ctx, s.cancel = context.WithCancel(ctx)

	// D-95: startup sweep is synchronous so callers see a consistent
	// state immediately after Start returns; failure aborts startup.
	if err := s.startupSweep(s.ctx); err != nil {
		s.started = false
		s.cancel()
		return fmt.Errorf("state.Server: startup sweep: %w", err)
	}

	s.wg.Add(1)
	go s.safetySweep()

	return nil
}

// Stop cancels the internal ctx, waits for the safety-sweep goroutine to
// drain, and calls Timer.Stop on every pending AfterFunc handle. Releasing
// startMu BEFORE wg.Wait prevents a deadlock if a concurrent Stop call
// arrives while we are waiting (the `started=false` flip is atomic under
// the lock). Idempotent.
func (s *Server) Stop() {
	s.startMu.Lock()
	if !s.started {
		s.startMu.Unlock()
		return
	}
	s.started = false
	s.startMu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	s.wg.Wait()

	s.timers.mu.Lock()
	for id, t := range s.timers.t {
		t.Stop()
		delete(s.timers.t, id)
	}
	s.timers.mu.Unlock()
}

// cacheKeyFor centralises the agent_state cache key namespace (D-86).
// Singular "agent_state" (NOT plural) per Pitfall 7. Every cache.GetOrSet
// + cache.Del in agent_states.go routes through here.
func (s *Server) cacheKeyFor(orgID, agentID uuid.UUID) string {
	return cache.Key(orgID, "agent_state", agentID)
}
