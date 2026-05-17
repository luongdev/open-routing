package state

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jonboulle/clockwork"

	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// scheduleWrapUpExpiry registers (or replaces) a per-agent AfterFunc timer.
// Concurrent firings (timer + safety-sweep + v0.1 multi-replica) race the
// UPDATE's `WHERE status='WrapUp' AND wrapup_until < NOW()` predicate;
// only the first wins (D-81).
//
// Called from startupSweep for rows returning after process restart, and
// from PATCH handlers once v0.2 system fires Engaged→WrapUp (D-82 deferred).
//
// Gemini-HIGH-2 fix: capture the Timer handle in a local before installing
// it so the AfterFunc closure can delete by POINTER EQUALITY after firing —
// without pointer-check a successor schedule (rapid state changes, WrapUp
// extension) causes the old AfterFunc body to delete the new timer's entry
// on fire, leaving Stop() unable to cancel it → goroutine leak after Stop.
func (s *Server) scheduleWrapUpExpiry(agentID, orgID uuid.UUID, until time.Time) {
	remaining := until.Sub(s.clock.Now())
	if remaining < 0 {
		remaining = 0
	}

	s.timers.mu.Lock()
	defer s.timers.mu.Unlock()

	if prior, ok := s.timers.t[agentID]; ok {
		prior.Stop()
	}

	var t clockwork.Timer
	t = s.clock.AfterFunc(remaining, func() {
		// Pitfall 2: respect ctx.Done() before any DB work so Stop()
		// drains cleanly even when AfterFunc fires concurrently with Stop.
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Bounded ctx so a hung DB call never leaks the goroutine
		// indefinitely (Threat T-04-11).
		expireCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.expireWrapUp(expireCtx, agentID, orgID)

		s.timers.mu.Lock()
		if cur, ok := s.timers.t[agentID]; ok && cur == t {
			delete(s.timers.t, agentID)
		}
		s.timers.mu.Unlock()
	})
	s.timers.t[agentID] = t
}

// expireWrapUp executes the idempotent ExpireWrapUp UPDATE. Safe to call
// from AfterFunc, safety sweep, and startup sweep. 0 rows = NO-OP (already
// expired by another firing or status changed — D-81 idempotency).
//
// Pitfall 8: sweeper has no ctx-injected org_id. db.WithBypass marks the
// ctx for SQLChecker audit (Phase 1 D-04). The UPDATE itself still
// mentions org_id in WHERE (denormalized agent_states.org_id column per
// D-78) for query auditability + Postgres plan stability.
func (s *Server) expireWrapUp(ctx context.Context, agentID, orgID uuid.UUID) {
	ctx = db.WithBypass(ctx, "wrapup_sweeper")
	q := generated.New(s.deps.OrgDB)
	row, err := q.ExpireWrapUp(ctx, generated.ExpireWrapUpParams{
		AgentID: pgUUID(agentID),
		OrgID:   pgUUID(orgID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Idempotent: another concurrent firing already expired this row,
		// or the row's status changed between schedule and fire (D-81).
		s.deps.Logger.DebugContext(ctx, "state.ttl.expire.noop",
			"agent_id", agentID, "org_id", orgID)
		return
	}
	if err != nil {
		s.deps.Logger.WarnContext(ctx, "state.ttl.expire.failed",
			"agent_id", agentID, "org_id", orgID, "err", err)
		return
	}

	// Cache invalidation. Failure logs WARN; cache heals via 60s TTL (D-86).
	cacheKey := s.cacheKeyFor(orgID, agentID)
	if delErr := s.deps.Cache.Del(ctx, cacheKey); delErr != nil {
		s.deps.Logger.WarnContext(ctx, "state.ttl.cache_del_failed",
			"key", cacheKey, "err", delErr)
	}

	// T-04-12: emit structured audit event on every successful expiry.
	s.deps.Logger.InfoContext(ctx, "state.ttl.expired",
		"agent_id", agentID,
		"org_id", orgID,
		"new_status", row.Status,
		"state_version", row.StateVersion,
	)
}

// startupSweep runs synchronously inside Start(ctx). It loads every WrapUp
// row across all orgs, fires-immediately for past-due rows, and schedules
// future timers for the rest. Per D-95, the sweep blocks Start from
// returning so no stuck WrapUp survives a process restart.
func (s *Server) startupSweep(ctx context.Context) error {
	ctx = db.WithBypass(ctx, "wrapup_sweeper.startup")
	q := generated.New(s.deps.OrgDB)
	rows, err := q.ListExpiringWrapUps(ctx)
	if err != nil {
		return fmt.Errorf("list expiring wrapups: %w", err)
	}

	now := s.clock.Now()
	for _, r := range rows {
		agentID := uuid.UUID(r.AgentID.Bytes)
		orgID := uuid.UUID(r.OrgID.Bytes)

		if !r.WrapupUntil.Valid || !r.WrapupUntil.Time.After(now) {
			// Past-due or null wrapup_until (defensive: status=WrapUp
			// should always have a timestamp, but expireWrapUp is idempotent).
			s.expireWrapUp(ctx, agentID, orgID)
			continue
		}
		s.scheduleWrapUpExpiry(agentID, orgID, r.WrapupUntil.Time)
	}
	return nil
}

// safetySweep runs on a clock.NewTicker cadence (default 30s, overridable
// via WithSweepInterval). On each tick it re-lists WrapUp rows and
// fires-immediately for any past-due row the AfterFunc timer missed
// (panic, drift, clock skew). Idempotent UPDATE protects against double-firing.
func (s *Server) safetySweep() {
	defer s.wg.Done()
	ticker := s.clock.NewTicker(s.sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.Chan():
			s.runSweepPastDue()
		}
	}
}

// runSweepPastDue is the per-tick body extracted so tests can call it
// deterministically without spinning the ticker.
func (s *Server) runSweepPastDue() {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()
	ctx = db.WithBypass(ctx, "wrapup_sweeper.safety")
	q := generated.New(s.deps.OrgDB)
	rows, err := q.ListExpiringWrapUps(ctx)
	if err != nil {
		s.deps.Logger.WarnContext(ctx, "state.ttl.safety_sweep.list_failed", "err", err)
		return
	}

	now := s.clock.Now()
	for _, r := range rows {
		if !r.WrapupUntil.Valid || r.WrapupUntil.Time.After(now) {
			continue
		}
		agentID := uuid.UUID(r.AgentID.Bytes)
		orgID := uuid.UUID(r.OrgID.Bytes)
		s.expireWrapUp(ctx, agentID, orgID)
	}
}
