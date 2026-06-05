// Command api runs the open-routing HTTP API.
//
// Wiring order is LOCKED by FOUND-07 / Pattern S6 / Pitfall 5:
//
//  1. Context with SIGINT/SIGTERM cancellation
//  2. config.Load  — 12-factor env loader
//  3. telemetry.InitOTel — FIRST runtime call, BEFORE chi.NewRouter is touched
//     so the global tracer provider is set before any consumer reads it.
//  4. slog default: TracingHandler wrapping JSONHandler so every log line
//     carries trace_id + span_id + org_id (D-29).
//  5. db.NewPool — pgxpool with retry-with-backoff (D-26).
//  6. redis.NewClient via URL parse.
//  7. db.NewOrgDB — shared instance handed to handlers.
//  8. cache.New + catalog.New — catalog.Handlers satisfies
//     api.StrictServerInterface (D-69) and is passed into server.NewMux
//     as Deps.StrictHandlers (Plan 03-10 replaces the Wave 0 stubs).
//  9. server.NewMux — chi mux with the LOCKED middleware chain (Recoverer
//     -> RequestID -> bypass routes -> /v1 Route(OrgContext + catalog)).
//  10. otelhttp.NewHandler wraps the mux AFTER NewMux completes — Pattern S6
//     requires the wrap is AFTER every mux.Use call so the request span
//     exists when OrgContext sets org_id span attribute.
//  11. http.Server.ListenAndServe in a goroutine; graceful shutdown on
//     ctx cancel via srv.Shutdown with a 10s timeout.
//
// Anti-pattern guard: this main MUST NOT call migrate.NewWithDatabaseInstance
// or m.Up. Per D-11 the API never auto-runs migrations — cmd/migrate owns
// that surface. SUMMARY.md asserts this with `! grep -q 'm.Up()'`.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"gopkg.in/yaml.v3"

	"github.com/jonboulle/clockwork"
	"github.com/redis/go-redis/v9"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/catalog"
	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/flowrt"
	"github.com/luongdev/open-routing/services/api/internal/imports"
	"github.com/luongdev/open-routing/services/api/internal/presence"
	"github.com/luongdev/open-routing/services/api/internal/server"
	"github.com/luongdev/open-routing/services/api/internal/state"
	"github.com/luongdev/open-routing/services/api/internal/telemetry"
	"github.com/luongdev/open-routing/services/api/internal/wsgateway"
)

func main() {
	os.Exit(run())
}

func run() int {
	// (0) Signal-aware context. signal.NotifyContext cancels ctx on
	// SIGINT/SIGTERM, which the goroutine on srv.ListenAndServe and the
	// graceful-shutdown block both observe.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// (1) Config first — every later step depends on it. We use slog
	// stdout directly here because OTel is not initialized yet (the
	// TracingHandler wraps the JSON handler only after step 3).
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load", "err", err)
		return 1
	}

	// (2) OTel SDK BEFORE chi router setup (FOUND-07).
	shutdownOTel, err := telemetry.InitOTel(ctx, cfg)
	if err != nil {
		slog.Error("otel init", "err", err)
		return 1
	}
	defer func() {
		// 5s budget for span flush so we don't block shutdown forever.
		// stdouttrace.WithSyncer (dev/CI) returns immediately; otlp
		// batcher needs the timeout to flush pending spans.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownOTel(shutdownCtx)
	}()

	// (3) slog with TracingHandler — every log line from this point on
	// carries trace_id + span_id + (when in-org) org_id (D-29).
	handler := telemetry.NewTracingHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(slog.New(handler))
	slog.InfoContext(ctx, "open-routing api starting", "listen_addr", cfg.ListenAddr)

	// (4) Database pool — retries with backoff against the dsn.
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.ErrorContext(ctx, "db pool init", "err", err)
		return 1
	}
	defer pool.Close()

	// (5) Redis client. ParseURL handles "redis://host:port/db" and
	// "rediss://..." for TLS.
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.ErrorContext(ctx, "redis url parse", "err", err)
		return 1
	}
	rdb := redis.NewClient(redisOpts)
	defer func() { _ = rdb.Close() }()

	// (6) Shared OrgDB. Validation mode comes from cfg (D-02). Dev/test
	// default is "panic" (config default); prod sets ORGDB_VALIDATION_MODE=error
	// so a regression returns a typed error instead of panicking the process.
	validationMode := db.ValidationPanic
	if cfg.ValidationMode == "error" {
		validationMode = db.ValidationError
	}
	orgDB := db.NewOrgDB(pool, db.NewSQLChecker(), validationMode)

	// (7) Spec bytes from the embedded generated package (D-45). The binary
	// always serves the spec it was built against — no stale-file risk.
	swagger, err := api.GetSpec()
	if err != nil {
		slog.ErrorContext(ctx, "spec load", "err", err)
		return 1
	}
	specBytes, err := yaml.Marshal(swagger)
	if err != nil {
		slog.ErrorContext(ctx, "spec marshal", "err", err)
		return 1
	}

	// (8) Cache + catalog handlers (D-69). catalog.Handlers IS the
	// StrictServerInterface impl — there is no composite server. The cache
	// is Redis-backed (CAT-11) and read-through via singleflight (D-52).
	catalogCache := cache.New(rdb, slog.Default())
	catalogHandlers := catalog.New(catalog.Deps{
		OrgDB:  orgDB,
		Pool:   pool,
		Cache:  catalogCache,
		Logger: slog.Default(),
	})

	// (8.5) state.Server — D-89 composite activation. Phase 4 introduces a
	// separate state-machine package; main.go merges it with catalog via
	// anonymous embedding. Server (NOT Handlers) per Pitfall 1.
	stateServer := state.New(state.Deps{
		OrgDB:  orgDB,
		Cache:  catalogCache,
		Logger: slog.Default(),
	}, state.WithClock(clockwork.NewRealClock()))

	// Synchronous startup sweep — guarantees no stuck WrapUp survives a
	// restart (D-95). Failure aborts process startup so kubernetes restarts
	// with a fresh attempt.
	if err := stateServer.Start(ctx); err != nil {
		slog.ErrorContext(ctx, "state server start", "err", err)
		return 1
	}
	// Shutdown order (LIFO) — note the additional importer.Stop step
	// introduced below for Phase 5:
	//   1. http.Server.Shutdown drains in-flight HTTP requests
	//   2. importer.Stop drains the import crash-recovery sweep goroutine
	//      (Phase 5 D5-11) — must precede pool.Close because the sweep
	//      issues UPDATEs against the still-open pool
	//   3. stateServer.Stop drains sweeper goroutine + cancels AfterFunc timers
	//      (must precede pool.Close — sweeper UPDATEs need the pool open)
	//   4. rdb.Close + pool.Close + telemetry shutdown
	defer stateServer.Stop()

	// (8.6) imports.Importer — Phase 5 composite activation (RESEARCH §F3).
	// Named Importer (not Server) to avoid the Go duplicate-anonymous-field
	// constraint with *state.Server in the ApiHandlers composite below.
	// Reuses the same catalogCache instance so import write paths hit the
	// same per-entity Redis keys as the catalog CRUD path (cache.Del per
	// succeeded row inside processChunk's post-commit hook).
	importer := imports.New(imports.Deps{
		OrgDB:  orgDB,
		Cache:  catalogCache,
		Logger: slog.Default(),
	}, imports.WithClock(clockwork.NewRealClock()))

	// Synchronous startup sweep — guarantees no stuck pending import job
	// survives a restart (D5-11). Failure aborts process startup so
	// kubernetes restarts with a fresh attempt (matches stateServer
	// startup contract above).
	if err := importer.Start(ctx); err != nil {
		slog.ErrorContext(ctx, "imports importer start", "err", err)
		return 1
	}
	// Deferred LIFO position: registered AFTER stateServer.Stop's defer,
	// so importer.Stop runs FIRST during shutdown (LIFO ordering of
	// deferred calls). Both must precede pool.Close because both sweepers
	// issue UPDATEs against the open pool.
	defer importer.Stop()

	// ApiHandlers composite — D-70 forward-compat seam activated for Phase 4,
	// extended in Phase 5 (D-89). Three-embed shape:
	//
	//   - catalog.Handlers contributes 41 methods (CRUD for 6 entities + scaffold).
	//   - state.Server contributes 2 agent-state status methods.
	//   - imports.Importer contributes BulkImportCatalog + GetImportJob
	//     (Phase 5 — Plan 05-06 added the method bodies).
	//   - flowrt.Endpoints contributes the v0.2 flow publish/validate/simulate/
	//     rollback + runtime read endpoints (501 stubs until Layer 3).
	//
	// All embed sets are disjoint — Go's method-set resolution merges them
	// cleanly. Pitfall 1 is avoided by giving each type a distinct name
	// (Handlers / Server / Importer / Endpoints; per RESEARCH §F3).
	// v0.3 W3: one presence store + capacity service shared by the route engine
	// (offerability + capacity holds) and the WS gateway (lease renew/drop) so a
	// gateway heartbeat is visible to the matcher's Connected check.
	presenceStore := presence.NewRedisStore(rdb, 0)
	capacitySvc := flowrt.NewCapacityService()
	flowrtEndpoints := flowrt.New(flowrt.Deps{
		OrgDB:          orgDB,
		Cache:          catalogCache,
		Presence:       presenceStore,
		Capacity:       capacitySvc,
		Logger:         slog.Default(),
		MatcherEnabled: cfg.MatcherEnabled,
		// v0.3 mock voice adapter (real LiveKit/SIP media = v0.4 behind this contract):
		// accept hands the assignment here; adapter terminals drive reservation teardown.
		Adapters: map[string]adapter.ChannelAdapter{"voice": adapter.NewMockVoice(nil)},
	})
	type ApiHandlers struct {
		*catalog.Handlers
		*state.Server
		*imports.Importer
		*flowrt.Endpoints
	}
	// Compile-time guarantee that the COMPOSITE satisfies the full
	// StrictServerInterface. If this line fails to compile, either:
	//  (a) the OpenAPI spec gained an endpoint with no implementation, OR
	//  (b) one of the embedded types lost a method.
	// Phase 5 ALSO depends on this assertion: deleting catalog/notimpl.go
	// removed catalog.Handlers's BulkImportCatalog + GetImportJob 501 stubs,
	// and the composite now picks them up from imports.Importer instead.
	var _ api.StrictServerInterface = (*ApiHandlers)(nil)

	apiHandlers := &ApiHandlers{
		Handlers:  catalogHandlers,
		Server:    stateServer,
		Importer:  importer,
		Endpoints: flowrtEndpoints,
	}

	// v0.3 W2/W3: agent WebSocket gateway (transport over the runtime command
	// service) + W3 connection-lease presence (Redis-primary).
	wsGateway := wsgateway.New(wsgateway.Deps{
		OrgDB:          orgDB,
		Cmd:            flowrtEndpoints,
		Presence:       presenceStore,
		Logger:         slog.Default(),
		MaxConnsPerOrg: cfg.WSMaxConnsPerOrg,
	})

	// (9) chi mux with locked chain (D-44 strict-server wiring).
	mux := server.NewMux(&server.Deps{
		Pool:           pool,
		Redis:          rdb,
		OrgDB:          orgDB,
		Config:         cfg,
		StrictHandlers: apiHandlers,
		SpecBytes:      specBytes,
		WSHandler:      wsGateway.Handler(),
	})

	// (10) OTel HTTP wrap AFTER NewMux returns (Pattern S6 — wrap is after
	// every mux.Use). The wrap creates a root server span on every
	// request, including /healthz/readyz/metrics (Plan 07 case 7 will
	// confirm via TestBypassPaths_NoHeaderRequired).
	rootHandler := otelhttp.NewHandler(mux, "open-routing-api")

	// (11) HTTP server.
	// ReadHeaderTimeout guards against slowloris-style header drip;
	// 10s is the conventional default for an internal API.
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           rootHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// (12) Listen + serve in a goroutine so main can wait on ctx.Done
	// and trigger graceful shutdown when the signal arrives.
	go func() {
		slog.InfoContext(ctx, "listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "listen and serve", "err", err)
			// Cancel ctx so the main goroutine's <-ctx.Done() unblocks and
			// the shutdown sequence runs (mux + pool + redis + otel).
			stop()
		}
	}()

	<-ctx.Done()
	slog.InfoContext(context.Background(), "shutting down")

	// 10s is enough for in-flight requests to drain while staying under
	// the typical kubernetes terminationGracePeriodSeconds (30s).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, "http shutdown", "err", err)
	}
	return 0
}
