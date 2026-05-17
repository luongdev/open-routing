package imports

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db"
)

// Locked numeric constants for the Phase 5 import endpoints. Captured
// here so the chi-mux setup (server.NewMux) and the chunk orchestrator
// (Plan 05-05) can reference a single source of truth.
const (
	// defaultSweepInterval — D5-11 crash-recovery cadence. 1 hour
	// balances freshness vs DB load (a stuck import takes 24 h to
	// surface as failed regardless of cadence; tick frequency only
	// controls how soon after that point the sweep observes it).
	defaultSweepInterval = 1 * time.Hour

	// ImportBodyLimit — D5-21 50 MB cap. Exported so the chi-mux
	// setup in server.go can wrap r.Body in http.MaxBytesReader
	// BEFORE any parser sees it. PITFALLS 5.3 reverse-proxy
	// alignment is documented in 05-CONTEXT.md (out of scope for
	// v0.1 docker-compose).
	ImportBodyLimit = 50 << 20

	// importRowLimit — D5-22 500-row hard cap. The streaming parsers
	// (Wave 3) fail-fast at row 501 with HTTP 413 instead of reading
	// to the end and counting. Locked here so the row processors and
	// the handlers reference one constant.
	importRowLimit = 500

	// chunkSize — D5-09 batched-savepoint chunk size. 500 ÷ 50 = at
	// most 10 chunks per request; each chunk opens one transaction,
	// applies 50 savepoints, then commits. Per-row failures rollback
	// the savepoint; chunk failures rollback the chunk. Wave 3 owns
	// the orchestrator (chunk.go).
	chunkSize = 50
)

// Deps bundles the runtime dependencies the Importer needs. Every field
// is REQUIRED; New does NOT validate non-nil at construction time. A nil
// field panics at first dereference with a clear method name in the stack
// trace (D-71 hybrid-constructor contract). Mirrors state.Deps; no
// per-org config knobs in v0.1.
type Deps struct {
	OrgDB  *db.OrgDB
	Cache  *cache.Cache
	Logger *slog.Logger
}

// Option is the functional-option type accepted by New. Mirrors
// state.Option exactly.
type Option func(*Importer)

// WithClock injects a clockwork.Clock implementation. Production wiring
// in cmd/api/main.go passes clockwork.NewRealClock(); tests use
// clockwork.NewFakeClock() so the safety-sweep ticker fires
// deterministically on fakeClock.Advance(d) — see sweep_test.go.
func WithClock(c clockwork.Clock) Option { return func(s *Importer) { s.clock = c } }

// WithSweepInterval overrides the defaultSweepInterval (1h) for tests.
// Production wiring never calls this — the cadence is locked at D5-11.
func WithSweepInterval(d time.Duration) Option { return func(s *Importer) { s.sweepInterval = d } }

// Importer implements the SUBSET of api.StrictServerInterface dealing
// with bulk-import operations (BulkImportCatalog + GetImportJob; Wave 4
// adds the method bodies). The full interface is satisfied by the
// *ApiHandlers composite in cmd/api/main.go (D-89 carry-forward); Wave 4
// will add `*imports.Importer` as a third anonymous field.
//
// Pitfall 1 / RESEARCH §F3 Open Q5: the type is [Importer] (NOT Server,
// NOT Handlers). Go rejects an ApiHandlers struct embedding two anonymous
// fields with the same type name; catalog already contributes Handlers
// and state contributes Server. Naming the Phase 5 struct Importer keeps
// Wave 4 wiring trivial.
type Importer struct {
	deps          Deps
	clock         clockwork.Clock
	sweepInterval time.Duration

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
	startMu sync.Mutex
}

// New constructs an *Importer. Required Deps validate at first
// dereference (D-71); optional knobs apply via Option funcs. Defaults:
// real clock, defaultSweepInterval (1h).
//
// No per-row AfterFunc timers exist in this package — only the global
// safety-sweep ticker — so there is no timers registry (contrast with
// state.Server which schedules per-agent AfterFunc handles for WrapUp).
func New(deps Deps, opts ...Option) *Importer {
	s := &Importer{
		deps:          deps,
		clock:         clockwork.NewRealClock(),
		sweepInterval: defaultSweepInterval,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start launches the background safety-sweep goroutine after running a
// synchronous startup sweep (D-95 / D5-11). The startup sweep runs BEFORE
// the ticker so callers see a consistent state immediately after Start
// returns; a failed startup aborts the Start call so cmd/api/main.go can
// fail the process and let kubernetes restart with a fresh attempt.
//
// Idempotent under startMu. A second Start while already-started is a
// silent no-op (matches state.Server.Start contract).
func (s *Importer) Start(ctx context.Context) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if s.started {
		return nil
	}
	s.started = true
	s.ctx, s.cancel = context.WithCancel(ctx)

	// D5-11 / D-95: startup sweep is synchronous — failure aborts
	// Start so the deployer learns about a misconfigured pool or
	// migration drift before the binary starts serving requests.
	if err := s.startupSweep(s.ctx); err != nil {
		s.started = false
		s.cancel()
		return fmt.Errorf("imports.Importer: startup sweep: %w", err)
	}

	s.wg.Add(1)
	go s.safetySweep()

	return nil
}

// Stop cancels the internal ctx and waits for the safety-sweep goroutine
// to drain. Idempotent. cmd/api/main.go calls Stop as part of the LIFO
// shutdown sequence:
//
//  1. http.Server.Shutdown drains in-flight requests.
//  2. stateServer.Stop drains state-machine WrapUp timers.
//  3. importer.Stop drains the import sweep goroutine.
//  4. rdb.Close + pool.Close + telemetry shutdown.
//
// Steps 2/3 run BEFORE pool.Close because their sweeps issue UPDATEs
// against the still-open pool.
func (s *Importer) Stop() {
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
}

// ---------------------------------------------------------------------------
// Sweep placeholders — REMOVED IN TASK 4 (sweep.go).
//
// These exist only so Plan 05-04 Task 1 can build standalone before
// Task 4 ships sweep.go. Task 4 deletes both stubs from handlers.go
// and ships the real implementations (safetySweep, runSweepPastDue,
// startupSweep) in sweep.go. Removing the placeholders is part of
// Task 4's diff; do NOT leave them in once sweep.go exists.
// ---------------------------------------------------------------------------

// startupSweep is the placeholder for Task 4's synchronous startup
// flip-stranded-pending-rows query. Currently a no-op so handlers.go
// compiles in isolation.
func (s *Importer) startupSweep(_ context.Context) error { return nil } //nolint:unused // replaced by sweep.go in Task 4

// safetySweep is the placeholder for Task 4's 1h-tick goroutine body.
// Currently exits immediately so wg.Done is called and Stop() drains
// cleanly even with the placeholder in place.
func (s *Importer) safetySweep() { defer s.wg.Done() } //nolint:unused // replaced by sweep.go in Task 4

