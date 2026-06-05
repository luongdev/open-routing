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
	"github.com/redis/go-redis/v9"

	"github.com/luongdev/open-routing/services/api/internal/config"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/adapter"
	"github.com/luongdev/open-routing/services/api/internal/flowrt"
	"github.com/luongdev/open-routing/services/api/internal/presence"
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

	// v0.3 W3: the runtime resumes live routes (timeout → re-offer), so it needs
	// the SAME live signals as the API — presence (so a re-offer filters by the
	// connection lease) + capacity (release on timeout, acquire on re-offer).
	redisOpts, rErr := redis.ParseURL(cfg.RedisURL)
	if rErr != nil {
		slog.ErrorContext(ctx, "redis url parse", "err", rErr)
		return 1
	}
	rdb := redis.NewClient(redisOpts)
	defer func() { _ = rdb.Close() }()
	endpoints := flowrt.New(flowrt.Deps{
		OrgDB:          orgDB,
		Presence:       presence.NewRedisStore(rdb, 0),
		Capacity:       flowrt.NewCapacityService(),
		Logger:         slog.Default(),
		MatcherEnabled: cfg.MatcherEnabled,
		DeliveryOutbox: cfg.DeliveryOutbox,
		// The matcher tick (here) reclaims a slot when an agent vanishes mid-call,
		// which must Release the channel adapter delivery — so the runtime needs the
		// same adapter wiring as cmd/api (cross-AI review MED). NOTE: the in-process
		// MockVoice keeps handles per-process, so a runtime reclaim of a handle the
		// api process created is a graceful no-op until real (out-of-process) media
		// lands in v0.4; the contract call is correct either way.
		Adapters: map[string]adapter.ChannelAdapter{"voice": adapter.NewMockVoice(nil)},
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
			// v0.3 W3: reclaim leaked capacity slots (expired pending holds +
			// terminal-reservation orphans) cross-org on the same tick.
			if freed, sErr := endpoints.SweepCapacity(ctx, pool); sErr != nil {
				slog.ErrorContext(ctx, "capacity sweep failed", "err", sErr)
			} else if freed > 0 {
				slog.InfoContext(ctx, "reclaimed capacity slots", "count", freed)
			}
			// v0.3 W4: the matcher tick — stale-offering recovery, SLA-deadline
			// fallback, and the availability-driven pull. Gated so the queue/matcher
			// model can be rolled out independently of the offer-now path.
			if cfg.MatcherEnabled {
				if offered, mErr := endpoints.RunMatcher(ctx, pool, workerID, time.Now()); mErr != nil {
					slog.ErrorContext(ctx, "matcher tick failed", "err", mErr)
				} else if offered > 0 {
					slog.InfoContext(ctx, "matcher offered routes", "count", offered)
				}
			}
			// v0.4 W1: drain the durable delivery outbox — hand accepted assignments
			// to the channel adapter. Gated; the in-process post-commit Deliver path
			// stays the default until DELIVERY_OUTBOX_ENABLED is flipped.
			if cfg.DeliveryOutbox {
				if delivered, dErr := endpoints.DrainDeliveries(ctx, pool, workerID, time.Now()); dErr != nil {
					slog.ErrorContext(ctx, "delivery drain failed", "err", dErr)
				} else if delivered > 0 {
					slog.InfoContext(ctx, "drained deliveries", "count", delivered)
				}
			}
		}
	}
}
