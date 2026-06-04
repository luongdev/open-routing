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

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/flowrt"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
	"github.com/luongdev/open-routing/services/api/internal/telemetry"
)

const (
	workerTick  = 1 * time.Second
	workerLease = 30 * time.Second
	workerBatch = 50
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

	validationMode := db.ValidationPanic
	if cfg.ValidationMode == "error" {
		validationMode = db.ValidationError
	}
	orgDB := db.NewOrgDB(pool, db.NewSQLChecker(), validationMode)
	endpoints := flowrt.New(flowrt.Deps{
		OrgDB:  orgDB,
		Logger: slog.Default(),
	})
	workerID := "runtime-" + uuid.Must(uuid.NewV7()).String()
	slog.InfoContext(ctx, "open-routing runtime starting (continuation worker)",
		"node_kinds", len(reg.Kinds()), "worker_id", workerID, "tick", workerTick)

	// Continuation worker: on each tick claim due reservation-timeout / wait /
	// wrapup-expiry rows (FOR UPDATE SKIP LOCKED) and resolve them. Multiple
	// replicas run safely (disjoint claims + claimed_by fencing).
	tick := time.NewTicker(workerTick)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "open-routing runtime shutting down")
			return 0
		case <-tick.C:
			n, err := endpoints.ProcessDueContinuations(ctx, pool, workerID, time.Now(), workerLease, workerBatch)
			if err != nil {
				slog.ErrorContext(ctx, "continuation tick failed", "err", err)
			} else if n > 0 {
				slog.InfoContext(ctx, "processed continuations", "count", n)
			}
		}
	}
}
