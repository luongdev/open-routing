---
phase: 04-agent-state-machine-go
plan: 04
subsystem: api
tags: [go, clockwork, ttl, goroutine, state-machine, sweeper, wrapup]

requires:
  - plan: 04-03
    provides: "testutil_test.go helpers (seedAgentStateRow, newTestHandlers), Server struct + timersRegistry, Wave 2 handler bodies"

provides:
  - "scheduleWrapUpExpiry: per-agent clockwork.AfterFunc with pointer-equality delete (Gemini-HIGH-2 applied)"
  - "expireWrapUp: idempotent ExpireWrapUp UPDATE under db.WithBypass + 10s bounded ctx + cache.Del + audit log"
  - "startupSweep: synchronous before Start returns (D-95) — no stuck WrapUp survives restart"
  - "safetySweep + runSweepPastDue: 30s safety-net goroutine catches missed AfterFunc firings"
  - "Server.Start: synchronous startup sweep + safety goroutine spawn (replaces Wave 1 stub)"
  - "Server.Stop: ctx cancel + wg.Wait + timer iteration (replaces Wave 1 stub)"
  - "6 clockwork-driven TTL tests: FiresOnExpiry, StartupSweep PastDue/Future, SafetySweep, GracefulShutdown, RaceCondition"
  - "ExpireWrapUp SQL fix: engaged_channel = NULL added (STATE-05 schema invariant)"

affects: [04-05, 04-06]

tech-stack:
  added: []
  patterns:
    - "clockwork.FakeClock + real-time wrapup_until for deterministic timer tests (avoids FakeClock ↔ Postgres NOW() skew)"
    - "Pointer-equality delete in AfterFunc closure (Gemini-HIGH-2): var t clockwork.Timer + cur == t guard"
    - "db.WithBypass(ctx, 'wrapup_sweeper.*') on every sweeper-side query (Pitfall 8 mitigation)"
    - "context.WithTimeout(10s) on AfterFunc body (T-04-11 goroutine leak prevention)"
    - "runSweepPastDue extracted for direct test invocation (avoids ticker timing dependence)"

key-files:
  created:
    - "services/api/internal/state/ttl.go"
    - "services/api/internal/state/ttl_test.go"
  modified:
    - "services/api/internal/state/handlers.go — Start/Stop stubs replaced; jitterMaxMs const removed"
    - "services/api/internal/db/queries/agent_states.sql — ExpireWrapUp gains engaged_channel = NULL"
    - "services/api/internal/db/generated/agent_states.sql.go — regenerated from SQL fix"

key-decisions:
  - "FakeClock ↔ Postgres NOW() skew: use real-time (time.Now()) timestamps in DB seeds; FakeClock controls AfterFunc scheduling only"
  - "runSweepPastDue extracted as public method so SafetySweep test can call it directly without advancing the ticker"
  - "engaged_channel = NULL added to ExpireWrapUp SQL (Rule 2 auto-fix from Gemini peer review)"
  - "jitterMaxMs constant removed (D-87 deferred to v0.2 per Codex-HIGH-2 + Gemini-MED fix)"

metrics:
  duration: ~16 min
  completed: "2026-05-17T04:13:00Z"
  tasks: 2
  files: 5
---

# Phase 4 Plan 04: WrapUp TTL Goroutine Machinery Summary

**WrapUp TTL goroutine machinery with clockwork.AfterFunc per-agent timer registry, synchronous startup sweep (D-95), 30s safety-sweep goroutine, graceful Stop with wg.Wait + timer drain; 6 clockwork-driven tests pass race-clean**

## Performance

- **Duration:** ~16 min
- **Started:** 2026-05-17T03:57:41Z
- **Completed:** 2026-05-17T04:13:00Z
- **Tasks:** 2 (1 auto + 1 tdd)
- **Files modified:** 5 (2 new, 3 modified)

## Accomplishments

- `services/api/internal/state/ttl.go` created with 6 functions: `scheduleWrapUpExpiry`, `expireWrapUp`, `startupSweep`, `safetySweep`, `runSweepPastDue`
- `Server.Start/Stop` bodies in `handlers.go` replace Wave 1 stubs: synchronous startup sweep (D-95), safety goroutine via `wg.Add(1)/go safetySweep()`, Stop drains via `wg.Wait()` then iterates timers calling `t.Stop()`
- Gemini-HIGH-2 pointer-equality fix applied: `var t clockwork.Timer; t = s.clock.AfterFunc(...); if cur == t { delete(...) }`
- Pitfall 2 mitigation: `select { case <-s.ctx.Done(): return; default: }` in every AfterFunc body
- Pitfall 8 mitigation: `db.WithBypass(ctx, "wrapup_sweeper.*")` on every sweeper DB call
- T-04-11 mitigation: `context.WithTimeout(context.Background(), 10*time.Second)` wraps every AfterFunc DB call
- 6 TTL tests pass race-clean; GracefulShutdown proves Pitfall 2 mitigation holds
- Rule 2 auto-fix: `engaged_channel = NULL` added to `ExpireWrapUp` SQL (STATE-05 schema invariant — caught by Gemini peer review MED)

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create ttl.go + update handlers.go Start/Stop | 1aaaf69 | ttl.go (new), handlers.go |
| 2 | Write ttl_test.go 6 TTL tests | cd3b822 | ttl_test.go (new) |
| Deviation | Fix ExpireWrapUp engaged_channel=NULL (Rule 2) | b6c38e4 | agent_states.sql, agent_states.sql.go |

## Go Test Results

```
go test -race -count=1 -run "TestWrapUpTTL" ./internal/state/...
ok  github.com/luongdev/open-routing/services/api/internal/state   (6 passed)

go test -count=1 ./internal/state/... ./internal/domain/...
ok  github.com/luongdev/open-routing/services/api/internal/state   (22 passed)
ok  github.com/luongdev/open-routing/services/api/internal/domain  (11 passed)
33 total tests pass
```

## Acceptance Greps

All Wave 3 acceptance greps verified:

```
grep -E "var t clockwork.Timer" ttl.go              → FOUND (Gemini-HIGH-2)
grep -E "cur == t" ttl.go                           → FOUND (Gemini-HIGH-2)
grep -E "jitter|jitterMaxMs" ttl.go                 → 0 matches (Codex-HIGH-2)
grep -E "uuid\.New\(\)\.Time\(\)" ttl.go            → 0 matches (Gemini-MED)
grep -E "pgtype\.Text" ttl.go                       → 0 matches (Codex-HIGH-2)
grep -E "jackc/pgx/v5/pgtype" ttl.go                → 0 matches (import removed)
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] FakeClock ↔ Postgres NOW() skew in TTL tests**
- **Found during:** Task 2 (test execution)
- **Issue:** `clockwork.NewFakeClock()` starts at real current time. Tests seeded `wrapup_until = fakeClock.Now() + 60s`. After `fakeClock.Advance(61s)`, the AfterFunc fired but Postgres `NOW()` hadn't advanced in real time, so `wrapup_until < NOW()` predicate evaluated `false` → UPDATE found no rows → tests failed (3/6 failing).
- **Fix:** Tests use `time.Now()` (real time) for `wrapup_until` DB values so Postgres `NOW()` sees them correctly; FakeClock controls timer scheduling only. `TestWrapUpTTL_SafetySweep_FiresMissedTimer` calls `runSweepPastDue()` directly to avoid ticker timing dependence.
- **Files modified:** `ttl_test.go` (no production code change)
- **Committed in:** `cd3b822`

**2. [Rule 2 - Missing Critical Functionality] engaged_channel not cleared in ExpireWrapUp**
- **Found during:** Cross-AI peer review (Gemini MED concern)
- **Issue:** `ExpireWrapUp` SQL transitions status from `WrapUp` to `Ready/NotReady` but left `engaged_channel` non-null. STATE-05 requires `engaged_channel = NULL` in all states except `Engaged`.
- **Fix:** Added `engaged_channel = NULL` to `ExpireWrapUp` SQL; regenerated `agent_states.sql.go` via `task gen`.
- **Files modified:** `agent_states.sql`, `agent_states.sql.go`
- **Committed in:** `b6c38e4`

## Cross-AI Peer Review Results

**Codex verdict:** BLOCK — Both HIGH findings are out-of-scope for Wave 3 (Wave 4 handles production wiring in main.go + CreateAgent atomic INSERT per D-89/D-93). Not actionable in this plan.

**Gemini verdict:** READY WITH FIXES — 2 HIGH concerns (missing Engaged transition in matrix + engaged_channel wiped on PATCH) are Wave 2 scope. MED concern (engaged_channel not cleared in ExpireWrapUp) was Wave 3 scope and auto-fixed as Rule 2 deviation in commit `b6c38e4`.

**Assessment:** Both Codex HIGH findings refer to Wave 4 work (04-05-PLAN.md explicitly covers main.go ApiHandlers wiring and CreateAgent agent_states INSERT). The Gemini MED finding in Wave 3 scope was fixed. READY for Wave 4.

## Known Stubs

None — all 6 TTL functions are fully implemented. `jitter()` was explicitly deferred to v0.2 per D-87 (no v0.1 caller for system-initiated Engaged→WrapUp per D-82).

## Threat Flags

None — all 3 threats from plan's threat_model are mitigated:
- T-04-10 (SQLChecker bypass): `db.WithBypass(ctx, "wrapup_sweeper.*")` on every sweeper query
- T-04-11 (hung DB goroutine leak): `context.WithTimeout(10s)` wraps every AfterFunc DB call
- T-04-12 (TTL without audit trail): `slog.InfoContext(ctx, "state.ttl.expired", ...)` on every successful expiry

## Self-Check

### Created Files Check
- services/api/internal/state/ttl.go: FOUND
- services/api/internal/state/ttl_test.go: FOUND

### Commit Check
- 1aaaf69 (Task 1 — ttl.go + handlers.go): FOUND
- cd3b822 (Task 2 — ttl_test.go): FOUND
- b6c38e4 (Deviation — SQL fix): FOUND

## Self-Check: PASSED

---
*Phase: 04-agent-state-machine-go*
*Plan: 04*
*Completed: 2026-05-17*
