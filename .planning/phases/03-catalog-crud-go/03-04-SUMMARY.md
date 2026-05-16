---
phase: 03-catalog-crud-go
plan: 04
subsystem: cache
tags: [cache, redis, go-redis, singleflight, generics, refresh-ahead, miniredis, testify]

# Dependency graph
requires:
  - phase: 03-catalog-crud-go
    provides: "Wave 0 added golang.org/x/sync v0.20.0 (singleflight) + github.com/alicebob/miniredis/v2 v2.38.0 (test-only) to services/api/go.mod"
provides:
  - "services/api/internal/cache.Cache — Redis read-through cache with Get/Set/Del semantics"
  - "services/api/internal/cache.GetOrSet[T any] — generic, type-safe, singleflight-deduped, refresh-ahead read-through helper"
  - "services/api/internal/cache.Key(orgID, entity, id) — D-58 canonical cache key constructor"
  - "services/api/internal/cache.ErrNotFound — sentinel for no-negative-caching propagation (D-54)"
  - "Reusable cache infra for Phase 4 (agent state lookups) and any future hot-path read entity"
affects: [03-catalog-crud-go (Wave 2: catalog handlers consume GetOrSet/Del/Key), 04-agent-state-machine (status lookups), 05-bulk-import (potential session reads)]

# Tech tracking
tech-stack:
  added:
    - "golang.org/x/sync/singleflight (already in go.mod via Wave 0; first use in this plan)"
    - "github.com/alicebob/miniredis/v2 (already in go.mod via Wave 0; first use in this plan)"
  patterns:
    - "Generic read-through cache: cache.GetOrSet[T any](ctx, c, key, ttl, load) (T, error)"
    - "Refresh-ahead via async goroutine using context.WithoutCancel to preserve trace_id + org_id"
    - "Process-local singleflight dedup of concurrent identical misses (Group.Do per key + refresh:<key>)"
    - "slog observability with 'cache' message + 'key' + 'outcome' (hit|miss|refresh|del|error) attrs"
    - "Best-effort cache semantics: Redis errors logged via slog but never propagated when loader can serve"

key-files:
  created:
    - services/api/internal/cache/cache.go
    - services/api/internal/cache/cache_test.go
    - services/api/internal/cache/doc.go
  modified: []

key-decisions:
  - "doc.go kept as a separate front-door quick-reference (caller examples) while cache.go header carries the D-49..D-60 contract. Plan said doc.go is optional; we kept it because the orchestrator success_criteria required the file."
  - "No REFACTOR commit cut — GREEN code has consistent slog attr keys ('key', 'outcome', 'err'), no duplication, no dead code. Plan explicitly permits skipping REFACTOR when no changes are needed."
  - "Refresh-ahead implementation inlined inside GetOrSet[T]'s body (closure captures T) rather than refactored into a *Cache method, which would have lost the generic type. Matches plan's <interfaces> shape verbatim."

patterns-established:
  - "Pattern: GetOrSet[T any] generic — caller passes a typed loader func(ctx) (T, error); JSON marshal/unmarshal is internal; ErrNotFound from loader propagates without caching the zero-value."
  - "Pattern: refresh-ahead at PTTL < 10s — goroutine uses context.WithoutCancel(ctx) so request cancel doesn't kill the refresh (Pitfall 4 guard preserves trace_id + org_id in slog correlation)."
  - "Pattern: singleflight dedup under both miss-key and refresh:<key> so concurrent refreshers collapse to one DB load."
  - "Pattern: cache.Del returns underlying Redis error so callers can log+continue (D-55) — a DEL failure NEVER converts a successful DB write into a 5xx."

requirements-completed: [CAT-11]

# Metrics
duration: 18min
completed: 2026-05-16
---

# Phase 3 Plan 04: internal/cache pkg — Redis-backed hot-path cache Summary

**Type-generic Redis read-through cache with singleflight miss-dedup and refresh-ahead at PTTL < 10s, shipping services/api/internal/cache.GetOrSet[T any] / Key / Del / ErrNotFound for CAT-11.**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-05-16T13:50:00Z (approx)
- **Completed:** 2026-05-16T14:08:35Z
- **Tasks:** 1 (TDD pattern → 2 commits: RED + GREEN)
- **Files modified:** 3 new (cache.go, cache_test.go, doc.go); 0 existing files touched

## Accomplishments

- **Type-generic cache.GetOrSet[T any]** with read-through semantics, JSON codec (D-60), and exact D-51 signature.
- **Singleflight-deduped miss path** (D-52) verified by 32-goroutine concurrent test — exactly ONE loader invocation regardless of concurrency.
- **Refresh-ahead at PTTL < 10s** (D-53) verified by miniredis FastForward test — async goroutine fires via context.WithoutCancel so trace_id + org_id remain attached.
- **No negative caching guard** (D-54) — loader returning ErrNotFound propagates WITHOUT writing the zero value; miniredis.Exists verifies absence.
- **Best-effort Del** (D-55) — propagates underlying Redis error so callers can log+continue per the v0.1 invalidation contract.
- **Reusable across phases** — single shared *Cache services all entity types via Go generics; Phase 4 will use it for agent status, Phase 5 may use it for import sessions.

## Task Commits

Task 1 was decomposed per TDD discipline into RED + GREEN:

1. **Task 1 (RED): Failing tests** — `6c5d352` (test) — 8 test functions against undefined cache.Cache/New/Key/GetOrSet/Del/ErrNotFound symbols. Verified RED by running `go test ./internal/cache/...` and observing 11 "undefined: cache.*" compile errors.

2. **Task 1 (GREEN): Implementation** — `9ca7817` (feat) — cache.go (238 lines) + doc.go (31 lines). All 8 tests pass under `-race`. Verified with the full plan acceptance gate:
   - `go build ./internal/cache/...` exit 0
   - `go vet ./internal/cache/...` exit 0
   - `go test -count=1 -race ./internal/cache/...` exit 0 — `8 passed in 1 packages` in 3.04s

3. **REFACTOR: skipped** — GREEN code has consistent slog attr ordering (`"key"`, `"outcome"`, `"op"`, `"err"`), no duplication, no dead code, no overlong functions. Plan explicitly allows skipping REFACTOR when no improvement is warranted.

**Plan metadata commit:** _(pending — this SUMMARY commit)_

## Files Created/Modified

- `services/api/internal/cache/cache.go` (238 lines) — Exported `Cache`, `New`, `Key`, `GetOrSet[T any]`, `(*Cache).Del`, `ErrNotFound`. Internal `refreshThreshold = 10*time.Second` constant. Implements D-49..D-60 in full.
- `services/api/internal/cache/cache_test.go` (212 lines) — 8 test functions covering the locked contract (see below).
- `services/api/internal/cache/doc.go` (31 lines) — User-facing quickref with code examples; cache.go header carries the D-49..D-60 contract reference.

## Test Inventory (services/api/internal/cache/cache_test.go)

Eight tests, all passing under `-race`:

| # | Test | Behavior asserted | Contract pin |
|---|------|-------------------|--------------|
| 1 | `TestKey_Format` | `cache.Key("orgA", "agents", "ag-1")` produces `or:orgA:agents:ag-1` verbatim. Also verifies the UUID-string + entity-slug case. | D-58 (key format) |
| 2 | `TestGetOrSet_Miss_LoadsAndSets` | First call with cold cache invokes the loader exactly once (atomic counter), returns the loaded value, and writes the JSON payload to Redis under the requested key. | Miss path; D-59 (60s TTL); D-60 (JSON) |
| 3 | `TestGetOrSet_Hit_NoLoad` | After pre-warm, second call returns the cached value, loader counter stays at 0. Uses a different loader return value to prove the cached value wins (not a race). | Hit path; D-50 (one shared cache, generics) |
| 4 | `TestGetOrSet_ErrNotFound_NotCached` | Loader returning `cache.ErrNotFound` propagates via `errors.Is`; miniredis `s.Exists(key)` returns false — the empty zero value is NOT written. | D-54 (no negative caching) |
| 5 | `TestGetOrSet_RefreshAhead` | Pre-warm with 60s TTL, `s.FastForward(51*time.Second)` to drop PTTL < 10s. Next call returns cached value immediately AND fires async refresh — `require.Eventually(loads >= 2, 2s)`. | D-53 (refresh-ahead + context.WithoutCancel) |
| 6 | `TestGetOrSet_Singleflight_Dedups` | 32 concurrent goroutines call GetOrSet for the same cold key; loader sleeps 20ms to hold singleflight open. After `wg.Wait()`, loader counter is exactly 1. | D-52 (singleflight dedup) |
| 7 | `TestDel` | Pre-warm key, then `c.Del(ctx, key)` returns nil; `s.Exists(key)` returns false. | D-55 invalidation contract (happy path) |
| 8 | `TestDel_RedisErrorPropagates` | `s.Close()` simulates Redis outage; `c.Del(ctx, "or:any:key")` returns a non-nil error. Callers log+continue per D-55. | D-55 invalidation contract (error path) |

### Raw test output (`go test -v -count=1 -race ./internal/cache/...`)

```
=== RUN   TestKey_Format
--- PASS: TestKey_Format (0.00s)
=== RUN   TestGetOrSet_Miss_LoadsAndSets
--- PASS: TestGetOrSet_Miss_LoadsAndSets (0.00s)
=== RUN   TestGetOrSet_Hit_NoLoad
--- PASS: TestGetOrSet_Hit_NoLoad (0.00s)
=== RUN   TestGetOrSet_ErrNotFound_NotCached
--- PASS: TestGetOrSet_ErrNotFound_NotCached (0.00s)
=== RUN   TestGetOrSet_RefreshAhead
--- PASS: TestGetOrSet_RefreshAhead (0.01s)
=== RUN   TestGetOrSet_Singleflight_Dedups
--- PASS: TestGetOrSet_Singleflight_Dedups (0.03s)
=== RUN   TestDel
--- PASS: TestDel (0.00s)
=== RUN   TestDel_RedisErrorPropagates
[redis dial-failure log lines from go-redis pool retry — expected during simulated outage]
--- PASS: TestDel_RedisErrorPropagates (1.71s)
PASS
ok  	github.com/luongdev/open-routing/services/api/internal/cache	3.033s
```

Note: `TestDel_RedisErrorPropagates` emits 4 lines of "redis: connection pool: failed to dial after 5 attempts" — this is go-redis's internal retry logger printing the simulated outage we intentionally created with `s.Close()`. The test PASSES; the output is expected.

## Decisions Made

1. **Inlined refresh-ahead inside GetOrSet[T]** — first draft factored the refresh into a `(*Cache).maybeRefreshAhead` method, but that loses the generic type parameter T (methods can't have type parameters separate from their receiver in Go 1.25). Re-inlined per the plan's `<interfaces>` shape verbatim. The closure captures T from the outer function so the JSON Marshal/Set sequence is type-safe.

2. **doc.go kept (not merged into cache.go)** — Plan said it was optional, but the orchestrator's `<success_criteria>` explicitly required `services/api/internal/cache/doc.go exists with package doc`. We split responsibilities: cache.go header holds the D-49..D-60 locked-contract block; doc.go holds the user-facing quickref with copy-pasteable code example. Both files carry `// Package cache` doc comments; `go vet` reports no duplicate-doc warning.

3. **No REFACTOR commit** — Code is already minimal: 1 file, 1 generic function with clear hit/refresh-ahead/miss branches, 1 method (Del), 1 constructor (New), 1 helper (Key), 1 sentinel (ErrNotFound). Skipping REFACTOR is explicitly permitted by the plan's TDD pattern ("REFACTOR (if needed)... commit only if changes").

## Deviations from Plan

None — plan executed exactly as written.

- Plan-specified interface signatures matched verbatim (GetOrSet[T any] with `(ctx, c, key, ttl, load)` argument order matching D-51).
- Plan-specified slog attribute keys matched (`"key"`, `"outcome"` with values `hit|miss|refresh|del|error`).
- Plan-specified test count matched (8 — Key format + 6 GetOrSet variants + 2 Del).
- Plan-specified hazards all addressed: refresh goroutine uses `context.WithoutCancel(ctx)` (Pitfall 4), ErrNotFound never writes to Redis (D-54 — pinned by TestGetOrSet_ErrNotFound_NotCached's `require.False(s.Exists)`), Set failure on miss is logged-not-propagated (D-55 spirit), generic type assertion handled defensively with typed error.

No CLAUDE.md project file exists; user-global CLAUDE.md instructions (RTK + GitHub assignee) are unrelated to this Go implementation work.

## Issues Encountered

1. **go.mod requires go 1.25.0; system Go is 1.23.12** — Resolved transparently by Go's `GOTOOLCHAIN=auto` setting. Running `go build` from within `services/api/` auto-downloaded go1.25.10 toolchain and the build proceeded normally. No plan deviation needed.

2. **TestDel_RedisErrorPropagates is slow (~1.7s)** — go-redis's internal retry loop tries to dial the closed miniredis 5 times with backoff before returning the failure to our `c.Del` call. This is go-redis behavior we can't control without injecting a custom dialer; the test passes correctly. Total test suite still runs in ~3s.

## Plan Acceptance Criteria — Verification

All checks from the plan's `<acceptance_criteria>` block:

```
test -f services/api/internal/cache/cache.go         : OK
test -f services/api/internal/cache/cache_test.go    : OK
test -f services/api/internal/cache/doc.go           : OK
grep "package cache" cache.go                        : 1
grep "func GetOrSet[T any]" cache.go                 : 1
grep "var ErrNotFound" cache.go                      : 1
grep "func New(" cache.go                            : 1
grep "func Key(" cache.go                            : 1
grep "func (c *Cache) Del(" cache.go                 : 1
grep "context.WithoutCancel" cache.go                : 4
grep "singleflight" cache.go                         : 10
grep "refreshThreshold" cache.go                     : 6
grep slog outcome attrs                              : 13 (>=4 required)
grep "miniredis.RunT" cache_test.go                  : 1
grep "s.FastForward" cache_test.go                   : 1
grep "^func Test" cache_test.go                      : 8 (>=7 required)
grep "require.False(t, s.Exists(key)" cache_test.go  : 2 (>=1 required)
cache.go line count                                  : 238 (>=100 required)
cache_test.go line count                             : 212 (>=200 required)
cd services/api && go build ./internal/cache/...     : exit 0
cd services/api && go vet ./internal/cache/...       : exit 0
cd services/api && go test -count=1 -race ./...      : exit 0 (8/8 pass)
```

All checks PASS.

## Threat Model — Mitigations Applied

From plan `<threat_model>`:

- **T-3-12 (cross-org leak via wrong key)** — `cache.Key(orgID, ...)` enforces orgID as the FIRST segment; type-genericity means catalog handlers can't accidentally omit it. Cross-org probe coverage lives in Plan 03-10's isolation test extension (out of scope for this plan).
- **T-3-13 (negative cache poisoning)** — Mitigated by D-54 implementation + `TestGetOrSet_ErrNotFound_NotCached` regression guard.
- **T-3-14 (cache stampede)** — Mitigated by D-52 singleflight (TestGetOrSet_Singleflight_Dedups) + D-53 refresh-ahead (TestGetOrSet_RefreshAhead).
- **T-3-15 (Redis outage -> 5xx)** — Mitigated by best-effort error handling on Get/PTTL/Set (logged, fall through to loader). Del propagates error but callers log+continue.
- **T-3-16 (refresh ctx panic)** — Mitigated by `context.WithoutCancel(ctx)` which preserves ctx values; `TestGetOrSet_RefreshAhead` exercises the goroutine path.

## Known Stubs

None — every exported symbol has a working implementation; tests prove behavior end-to-end. The cache package compiles and operates in isolation. Wave 2 (Plan 03-05) wires it into `catalog.Deps`; until then no caller exists by design.

## Threat Flags

None — this plan adds no new network endpoint or trust-boundary file. The cache layer sits behind in-process handler code; it has no external entry point. Org isolation lives in the key construction (D-58) which is exercised by `TestKey_Format`.

## User Setup Required

None.

## Next Phase Readiness

- **Wave 2 (Plans 03-05 → 03-09, sequential)** can now wire `cache.New(rdb, logger)` in `main.go` and pass `*cache.Cache` into `catalog.Deps`. The plan's locked signatures (`GetOrSet[T any]`, `Key`, `Del`, `ErrNotFound`) match the Wave 2 consumer patterns documented in 03-PATTERNS.md.
- **Phase 4 (Agent State Machine)** can reuse this package for status lookups with key `or:{orgId}:agent_state:{agent_id}` — no further infra needed.
- **No blockers** for downstream plans.

## Self-Check: PASSED

Verified files exist and commit hashes are reachable:

- `services/api/internal/cache/cache.go` — FOUND (238 lines)
- `services/api/internal/cache/cache_test.go` — FOUND (212 lines)
- `services/api/internal/cache/doc.go` — FOUND (31 lines)
- `.planning/phases/03-catalog-crud-go/03-04-SUMMARY.md` — FOUND (this file)
- Commit `6c5d352` (RED) — REACHABLE in `git log --oneline`
- Commit `9ca7817` (GREEN) — REACHABLE in `git log --oneline`

## TDD Gate Compliance

This plan uses the TDD pattern (tdd="true" on Task 1). Gate sequence verified in git log:

1. `test(03-04): add failing tests for cache package (RED)` — `6c5d352` ✓
2. `feat(03-04): implement cache package (GREEN)` — `9ca7817` ✓
3. `refactor(03-04): ...` — skipped (no changes required; permitted by TDD pattern)

RED + GREEN gates present in the expected order.

---
*Phase: 03-catalog-crud-go*
*Plan: 04 (Wave 1, Plan C — parallel to 03-02 migration + 03-03 sqlc)*
*Completed: 2026-05-16*
