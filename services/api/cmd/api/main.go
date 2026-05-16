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
//  8. server.NewMux — chi mux with the LOCKED middleware chain (Recoverer
//     -> RequestID -> bypass routes -> /v1 Route(OrgContext + scaffold)).
//  9. otelhttp.NewHandler wraps the mux AFTER NewMux completes — Pattern S6
//     requires the wrap is AFTER every mux.Use call so the request span
//     exists when OrgContext sets org_id span attribute.
//  10. http.Server.ListenAndServe in a goroutine; graceful shutdown on
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

	"github.com/redis/go-redis/v9"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/server"
	"github.com/luongdev/open-routing/services/api/internal/telemetry"
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

	// (8) Composite server: scaffold impl + bypass-path handlers + 501 stubs.
	strictServer := server.NewCompositeServer(orgDB, pool, rdb, specBytes)

	// (9) chi mux with locked chain (D-44 strict-server wiring).
	mux := server.NewMux(&server.Deps{
		Pool:           pool,
		Redis:          rdb,
		OrgDB:          orgDB,
		Config:         cfg,
		StrictHandlers: strictServer,
		SpecBytes:      specBytes,
	})

	// (8) OTel HTTP wrap AFTER NewMux returns (Pattern S6 — wrap is after
	// every mux.Use). The wrap creates a root server span on every
	// request, including /healthz/readyz/metrics (Plan 07 case 7 will
	// confirm via TestBypassPaths_NoHeaderRequired).
	rootHandler := otelhttp.NewHandler(mux, "open-routing-api")

	// (9) HTTP server.
	// ReadHeaderTimeout guards against slowloris-style header drip;
	// 10s is the conventional default for an internal API.
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           rootHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// (10) Listen + serve in a goroutine so main can wait on ctx.Done
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
