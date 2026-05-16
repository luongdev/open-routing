---
phase: 03-catalog-crud-go
plan: 10
subsystem: api
tags: [go, chi, oapi-codegen, pgx, redis, miniredis, testcontainers, openapi, foundatio-08, multi-org]

requires:
  - phase: 01-foundation-polyglot-monorepo
    provides: org-db wrapper, RequestID middleware, OrgContext middleware, SQLChecker, testcontainers harness
  - phase: 02-openapi-contract-codegen
    provides: oapi-codegen strict-server, openapi 3.0 spec, error envelope shape, type-safe response wrappers
  - phase: 03-catalog-crud-go (Waves 0-3)
    provides: catalog.Handlers with all 6 entity CRUD (CAT-01..06), agent_skills join, cursor pagination, optimistic locking, cache layer with refresh-ahead + singleflight
provides:
  - Production wiring of cache.New + catalog.New in cmd/api/main.go (Wave0TempStubs deleted)
  - Real bypass-route handlers (Healthz/Readyz/OpenAPISpec/Docs) on catalog.Handlers
  - Cache.Ping(ctx) for readyz probe
  - Cross-org isolation suite covering every CAT-* entity + 2 FK cross-org probes (D-76)
  - Empirical proof of FOUND-08 zero-leakage across all 6 catalog tables
affects: [phase-04-agent-state-machine, phase-05-bulk-import, phase-07-host-integration]

tech-stack:
  added:
    - sync.Once for spec-YAML marshal memoisation in bypass.go
  patterns:
    - "Catalog handlers as the single StrictServerInterface impl (D-69) — bypass methods + CRUD methods share one handler value"
    - "Bypass handlers use Pool directly (not orgDB) — bypass routes have no X-Org-Id, so SQLChecker's unscoped-query rejection would block them"
    - "miniredis fallback in test/isolation/main_test.go when REDIS_URL is unset — catalog.Handlers always has a working Cache dep"
    - "Helper-driven cross-org probes: postEntity/getEntityStatus/listEntityIDs centralise the auth header + URL shape so per-entity tests stay 5-10 lines"

key-files:
  created:
    - services/api/internal/catalog/bypass.go
    - services/api/internal/catalog/docs.html (relocated from internal/server/)
    - services/api/test/isolation/catalog_test.go
  modified:
    - services/api/cmd/api/main.go
    - services/api/internal/cache/cache.go
    - services/api/internal/catalog/handlers.go
    - services/api/internal/catalog/testutil_test.go
    - services/api/internal/server/server.go
    - services/api/internal/server/server_test.go
    - services/api/internal/server/health.go
    - services/api/internal/server/health_test.go
    - services/api/internal/testsupport/doc.go
    - services/api/internal/testsupport/httpclient.go
    - services/api/test/isolation/main_test.go
    - services/api/test/isolation/isolation_test.go
  deleted:
    - services/api/internal/server/wave0_temp_stubs.go (Wave 0 transitional stubs)
    - services/api/internal/server/openapi.go (dead factory — logic in bypass.go)
    - services/api/internal/server/openapi_test.go (covered by isolation suite)
    - services/api/internal/server/docs.html (moved to internal/catalog/)
    - services/api/internal/catalog/bypass_placeholders.go (replaced by bypass.go)
    - services/api/internal/testsupport/seed.go (scaffold helpers — referenced types deleted)
    - services/api/test/isolation/schema_test.go (queries dropped _scaffold table)

key-decisions:
  - "Bypass handlers live on catalog.Handlers (not a separate infrastructure package) — D-69 single dispatch surface"
  - "GetOpenAPISpec memoises yaml.Marshal via sync.Once — the LB health-check path cannot afford to re-marshal on every hit"
  - "miniredis fallback in test/isolation/main_test.go so the suite never depends on a developer's local REDIS_URL"
  - "noopStrictStub kept in server_test.go (not via importing catalog) — mux-routing tests stay self-contained at 0 testcontainer cost"

patterns-established:
  - "Pattern: when a handler error path has no spec-defined wrapper (e.g. GetOpenAPISpec 500), return (nil, err) and let the strict-server default emit 500 with request_id injection"
  - "Pattern: cross-org probe = create-in-orgA + GET-in-orgA-200 + GET-in-orgB-404 (no other assertions needed — the SQLChecker guarantees the rest)"
  - "Pattern: cross-org FK probe = orgB-owned row id in orgA-owned create body → 422 invalid_reference (D-76)"
  - "Pattern: list-isolation probe asserts every returned row's org_id field matches the caller (catches per-row leaks even when row count is right)"

requirements-completed: [CAT-01, CAT-02, CAT-03, CAT-04, CAT-05, CAT-06, CAT-07, CAT-08, CAT-09, CAT-10, CAT-11]

duration: 1h 5m
completed: 2026-05-16
---

# Phase 3 Plan 10: Final Wiring + Cross-Org Isolation Suite

**Production binary now boots with catalog.Handlers as the single StrictServerInterface impl; bypass routes serve real bodies; every catalog table has a passing cross-org probe (9 new tests) proving FOUND-08 zero-leakage.**

## Performance

- **Duration:** ~1h 5m
- **Started:** 2026-05-16T15:30:00Z (approx — based on first commit)
- **Completed:** 2026-05-16T16:37:00Z
- **Tasks:** 3 of 3
- **Files modified/deleted:** 21 (8 created/modified, 7 deleted, 6 minor cleanups)
- **Net code churn:** +780 / -1159 lines

## Accomplishments

- **`cmd/api/main.go` wires the real catalog handlers.** `cache.New + catalog.New` replace the Wave 0 transitional stubs. The binary now exercises the exact same code path the unit + integration suites have been validating since Plan 03-05.
- **Bypass routes ship real bodies on catalog.Handlers.** `/healthz` is dependency-free (LB liveness), `/readyz` pings pgxpool + Cache + reads `schema_migrations` (503 if anything degraded), `/openapi.yaml` serves the embedded spec via memoised yaml.Marshal, `/docs` returns the Scalar viewer HTML. All four exist as methods on `*catalog.Handlers` so the strict-server pipeline is the single dispatch surface (D-69).
- **Cross-org isolation suite extended to every CAT-* entity** (9 new tests in `test/isolation/catalog_test.go`):
  - 6 single-entity probes (agents, skills, queues, channels, adapters, break-reasons)
  - 2 FK cross-org probes (channels.default_queue_id, agent_skills.skill_id) — proves D-76 holds end-to-end
  - 1 list-isolation probe — asserts orgA's list contains exactly orgA rows AND every row's org_id field matches the caller
- **Wave 0 transitional artefacts deleted.** No `Wave0TempStubs` reference remains in the working tree. `bypass_placeholders.go` deleted. `schema_test.go` (queried the dropped `_scaffold` table) deleted. `testsupport/seed.go` (scaffold helpers) deleted.
- **Empirical proof Phase 3 is complete.** `go test -race -count=1 -timeout 600s ./...` exits 0 with 340 tests across 14 packages green; `go build ./...` and `go vet ./...` clean.

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire cache.New + catalog.New into cmd/api/main.go; delete Wave0TempStubs** — `42e27ca` (feat)
2. **Task 2: Real bypass handlers + Cache.Ping for readyz probe** — `0aa0160` (feat)
3. **Task 3: Cross-org isolation suite for every catalog entity** — `44cc8ac` (test)

## Files Created/Modified

### Created

- `services/api/internal/catalog/bypass.go` — Real Healthz/Readyz/OpenAPISpec/Docs handlers, plus a memoised yaml.Marshal of the embedded spec.
- `services/api/internal/catalog/docs.html` — Scalar API Reference viewer (relocated from `internal/server/` to live next to the `//go:embed` directive in `bypass.go`).
- `services/api/test/isolation/catalog_test.go` — 9 cross-org probe tests for every CAT-* entity + 2 FK probes + 1 list-isolation probe.

### Modified

- `services/api/cmd/api/main.go` — Wiring now `cache.New(rdb, slog.Default())` + `catalog.New(catalog.Deps{...})` → `server.NewMux(StrictHandlers: catalogHandlers)`. Wiring order numbers in the package comment updated.
- `services/api/internal/cache/cache.go` — `Cache.Ping(ctx) error` added (one-line wrap around `c.rdb.Ping(ctx).Err()`); used by GetReadyz.
- `services/api/internal/server/server.go` — Top-of-file package doc updated to describe the catalog dispatch surface; `Deps.StrictHandlers` comment now references production catalog.Handlers (not Wave 0 stubs).
- `services/api/internal/server/server_test.go` — Local `noopStrictStub` satisfying `api.StrictServerInterface` so mux-routing tests stay self-contained (no DB, no Redis, no testcontainers).
- `services/api/internal/server/health.go` — Trimmed to just `MetricsHandler` (LiveHandler / ReadyzHandler factories were dead code — bodies now live in catalog/bypass.go).
- `services/api/internal/server/health_test.go` — Trimmed to just the MetricsHandler test.
- `services/api/internal/catalog/handlers.go` — Header comment updated (no longer references Wave 0 stubs as a transitional state).
- `services/api/internal/catalog/testutil_test.go` — Stale Wave 0 comment removed.
- `services/api/internal/testsupport/doc.go` + `httpclient.go` — Trimmed to the `DoBare` helper only (scaffold-era helpers had referenced deleted types).
- `services/api/test/isolation/main_test.go` — Wires `cache.New + catalog.New` (replaces deleted `server.NewWave0TempStubs`); miniredis fallback when REDIS_URL is unset.
- `services/api/test/isolation/isolation_test.go` — Scaffold-era cases 1-3 + 11 DROPPED (superseded by `catalog_test.go`). Middleware cases (4-7 + 10 + B-1) kept with `/agents` URLs replacing `/_scaffold`. Case B-1 now asserts the real 404 from catalog.GetAgent against a missing UUIDv7.

### Deleted

- `services/api/internal/server/wave0_temp_stubs.go` — Wave 0 transitional StrictServerInterface impl (299 LOC).
- `services/api/internal/server/openapi.go` + `openapi_test.go` + `docs.html` — Dead factories; logic now in catalog/bypass.go.
- `services/api/internal/catalog/bypass_placeholders.go` — Replaced by real bypass.go.
- `services/api/internal/testsupport/seed.go` — Scaffold seed helpers referenced deleted types.
- `services/api/test/isolation/schema_test.go` — Queried `_scaffold` table which migration 002 drops.

## Decisions Made

- **GetOpenAPISpec 500 path returns `(nil, err)`** — the spec defines only a 200 response for `/openapi.yaml`, so the strict-server contract has no 500 wrapper. Returning a non-nil error triggers the strict-server default error handler which emits 500 with the request_id middleware still active. (vs. inventing a wrapper type or panicking — both worse choices.)
- **`server_test.go` uses a local 38-method `noopStrictStub` instead of importing catalog.** Keeps the mux-routing unit tests at zero testcontainer cost; the catalog→server test wiring (testutil_test.go) already proves the catalog.Handlers integration works at the integration tier.
- **miniredis fallback in `test/isolation/main_test.go`.** The isolation suite never relies on a developer's local REDIS_URL — when REDIS_URL is unset, miniredis.Run() spins up an in-process Redis-compatible server that lives for the test binary's lifetime. catalog.Handlers always has a working Cache dep, so `/readyz` returns 200 (not 503), and the CAT-11 cache path is exercised end-to-end.
- **docs.html relocated to `internal/catalog/`.** `//go:embed docs.html` requires the file in the same directory as the embed directive; moving it from `internal/server/` to `internal/catalog/` ships the asset where it's consumed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Replaced custom `marshalOnce` type with `sync.Once`**
- **Found during:** Task 2 (initial draft of bypass.go)
- **Issue:** First draft rolled its own `marshalOnce` type with a non-thread-safe `done bool`. Multiple concurrent /openapi.yaml requests would race on the spec-marshal closure.
- **Fix:** Replaced with `sync.Once` from stdlib.
- **Files modified:** services/api/internal/catalog/bypass.go
- **Verification:** `go vet ./...` + `go test -race -count=1` exit 0 across 340 tests.
- **Committed in:** 0aa0160 (Task 2 commit, before the commit was finalised)

**2. [Rule 3 - Blocking] test/isolation/main_test.go updated in Task 1 (not deferred to Task 3)**
- **Found during:** End of Task 1 (`go vet ./...` failed with `undefined: server.NewWave0TempStubs`)
- **Issue:** Deleting `wave0_temp_stubs.go` in Task 1 broke `test/isolation/main_test.go` which referenced `server.NewWave0TempStubs`. Plan scoped main_test.go update to Task 3, but the build had to stay green at the Task 1 commit boundary.
- **Fix:** Updated `main_test.go` in Task 1 to use `catalog.New + cache.New + miniredis` fallback. Task 3 only adds `catalog_test.go` + rewrites `isolation_test.go`.
- **Files modified:** services/api/test/isolation/main_test.go (Task 1 commit)
- **Verification:** `go build + go vet + go test -short -race ./...` exit 0 at end of Task 1.
- **Committed in:** 42e27ca (Task 1 commit)

**3. [Rule 1 - Bug] server_test.go required a StrictServerInterface impl after Wave0TempStubs deletion**
- **Found during:** Task 1 cleanup
- **Issue:** Three tests in `services/api/internal/server/server_test.go` called `NewWave0TempStubs(nil, nil, nil)` to get a strict server for mux-routing tests. After the deletion, these tests fail to compile.
- **Fix:** Defined a local `noopStrictStub` type implementing the 38 StrictServerInterface methods (real GetHealthz body, 500 not_implemented_in_test_stub for everything else). Used in all three tests via `&Deps{StrictHandlers: noopStrictStub{}}`.
- **Files modified:** services/api/internal/server/server_test.go (Task 1 commit)
- **Verification:** All three mux-routing tests pass.
- **Committed in:** 42e27ca (Task 1 commit)

**4. [Rule 1 - Bug] LiveHandler/ReadyzHandler/OpenAPISpecHandler/DocsHandler were dead code**
- **Found during:** Task 2 (reading internal/server/openapi.go + health.go to find reusable helpers)
- **Issue:** Plan-described "may need to inline the logic since the existing server/ helpers may be methods on a deleted struct" — actually the factory functions in openapi.go and health.go were never called from NewMux (the strict server handles bypass routes via the spec-driven registration in api.HandlerWithOptions). They were leftover dead code.
- **Fix:** Deleted `LiveHandler`, `ReadyzHandler`, `OpenAPISpecHandler`, `DocsHandler` and their tests. Kept `MetricsHandler` (still wired at `r.Get("/metrics", MetricsHandler())` in server.go because /metrics is NOT in the OpenAPI spec). Moved docs.html from internal/server/ to internal/catalog/ for the embed directive.
- **Files modified:** services/api/internal/server/openapi.go (DELETED), openapi_test.go (DELETED), health.go (trimmed), health_test.go (trimmed), docs.html (MOVED)
- **Verification:** `grep -rn` confirms no remaining callers; all tests still pass.
- **Committed in:** 42e27ca (Task 1 commit — bundled with the main.go wiring cleanup)

**5. [Rule 1 - Bug] testsupport scaffold helpers referenced deleted types**
- **Found during:** Task 3 (reviewing testsupport for cleanup)
- **Issue:** `testsupport.PostScaffold`, `ListScaffolds`, `GetScaffoldStatus`, `SeedScaffold`, `ScaffoldRow`, `ScaffoldSeed` were defined in `httpclient.go` and `seed.go` but had zero callers (the isolation_test.go scaffold-era cases were rewritten in Task 3 to use catalog endpoints directly).
- **Fix:** Trimmed `httpclient.go` to just the `DoBare` helper (still used). Deleted `seed.go`. Updated `doc.go` to describe the trimmed surface.
- **Files modified:** services/api/internal/testsupport/{doc.go, httpclient.go, seed.go (DELETED)}
- **Verification:** `grep -rn "testsupport\." --include="*.go" .` confirms only `DoBare` is referenced post-cleanup.
- **Committed in:** 44cc8ac (Task 3 commit)

---

**Total deviations:** 5 auto-fixed (1 bug, 1 blocking, 3 dead-code cleanups)
**Impact on plan:** All deviations stay within Rule 1-3 (no architectural changes; no Rule 4 escalations). The dead-code deletions reduce LOC by ~430 net while preserving all behavior. No scope creep — every change directly serves the plan's "final wiring + cleanup" objective.

## Issues Encountered

- **None.** The plan was thorough; the only friction was that some plan-described "Wave 0 stub" descriptions assumed structure (e.g., `LiveHandler` factories being callable, GetOpenAPISpec having a 500 wrapper) that didn't match the actual generated code. These resolved into the deviations documented above.

## User Setup Required

None — no external service configuration changed in this plan. The production binary still requires `DATABASE_URL` and `REDIS_URL` from Phase 1 D-08 config; no new env vars added.

## Next Phase Readiness

**Phase 3 is COMPLETE.** Phase 4 (Agent State Machine) is unblocked:

- `catalog.Handlers` now ships every CAT-* method as real (no more 500 not_implemented_yet on the catalog hot path).
- `notimpl.go` still 500-stubs `GetAgentStatus`, `PatchAgentStatus`, `BulkImportCatalog`, `GetImportJob` — these are Phase 4 (STATE-*) and Phase 5 (IMP-*) surface and will land per D-70 forward-compat (catalog.Handlers embeds state.Handlers + imports.Handlers when those phases ship).
- The cross-org isolation harness is now entity-aware — Phase 4 will add `TestState_CrossOrg` for agent_states (STATE-* equivalent of FOUND-08).

**No blockers carried into Phase 4.** State pending in `.planning/STATE.md`:
- WrapUp TTL multi-replica concern (still open per Phase 3 RESEARCH — flag before v1).
- `postInteractionState` default configurability (Phase 4 product decision).

## Self-Check: PASSED

All created files exist on disk; all deleted files are absent; all three commit hashes are reachable from `git log --oneline --all`.
- Created: `bypass.go`, `docs.html` (in catalog/), `catalog_test.go` — all present.
- Deleted: `wave0_temp_stubs.go`, `openapi.go`, `openapi_test.go`, `bypass_placeholders.go`, `seed.go`, `schema_test.go` — all confirmed absent.
- Commits: `42e27ca`, `0aa0160`, `44cc8ac` — all reachable.
