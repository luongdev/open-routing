// Command runtime is the v0.2 routing runtime plane — a separate process from
// cmd/api (the control plane). It owns live route execution and all time-driven
// work (live `wait` resumes, reservation timeouts, WrapUp expiry), claiming due
// continuations with FOR UPDATE SKIP LOCKED so multiple replicas run safely.
//
// Layer 1 ships the process scaffold with the SAME early-startup sequence as
// cmd/api (config, OTel, structured logging, pgx pool) plus the shared node
// registry, then blocks until shutdown. Layer 3/5 fill in the DB-backed claim
// loop and executor over the registry; both reuse internal/runtime, which
// cmd/api also imports for in-process simulation, so there is one execution
// implementation across both planes.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
	"github.com/luongdev/open-routing/services/api/internal/telemetry"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load", "err", err)
		return 1
	}

	shutdownOTel, err := telemetry.InitOTel(ctx, cfg)
	if err != nil {
		slog.Error("otel init", "err", err)
		return 1
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownOTel(shutdownCtx)
	}()

	slog.SetDefault(slog.New(telemetry.NewTracingHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))))

	// The continuation worker claims due rows over this pool with FOR UPDATE
	// SKIP LOCKED; it is the runtime plane's source of truth, so the pool is
	// foundational here even though the claim loop itself lands in Layer 3/5.
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.ErrorContext(ctx, "db pool init", "err", err)
		return 1
	}
	defer pool.Close()

	reg := runtime.DefaultRegistry()
	slog.InfoContext(ctx, "open-routing runtime starting (worker stub — Layer 3/5 fill the SKIP LOCKED claim loop)",
		"node_kinds", len(reg.Kinds()))

	// TODO(Layer 3/5): build OrgDB over pool, the continuation worker
	// (due_at + FOR UPDATE SKIP LOCKED), and the executor over reg.
	<-ctx.Done()
	slog.InfoContext(ctx, "open-routing runtime shutting down")
	return 0
}
