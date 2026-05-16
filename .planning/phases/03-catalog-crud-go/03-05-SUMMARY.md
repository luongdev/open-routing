---
phase: 03-catalog-crud-go
plan: 05
subsystem: api
tags: [go, chi, oapi-codegen, strict-server, catalog, scaffold, cursor, pgtype]

# Dependency graph
requires:
  - phase: 03-catalog-crud-go (intra-phase)
    provides: openapi.yaml regen with 38-method StrictServerInterface (Plan 03-01), sqlc-generated types + OrgDB (Plans 03-02/03), cache.Cache (Plan 03-04)
provides:
  - "services/api/internal/catalog/ package — Handlers struct + Deps + New + WithClock"
  - "Compile-time guarantee var _ api.StrictServerInterface = (*Handlers)(nil)"
  - "Cross-cutting helpers: cursor.go (D-63 encode/decode), mappers.go (pgtype <-> stdlib), errors.go (pgx error -> http triple)"
  - "Phase 4/5 stubs in notimpl.go (4 methods); bypass placeholders (4 methods); 30 per-entity placeholders across 6 files"
  - "Reserved agent_skills.go for Plan 03-09; testutil_test.go skeleton for Plan 03-06"
affects: [03-06, 03-07, 03-08, 03-09, 03-10, 04-state-machine, 05-bulk-import]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D-68 one .go per entity in single catalog package — agents/skills/queues/channels/adapters/break_reasons/agent_skills"
    - "D-69 catalog.Handlers IS the StrictServerInterface impl — no composite server"
    - "D-71 hybrid constructor: New(Deps, ...Option) with required-fields struct + functional options"
    - "D-73 testutil_test.go (not testutil.go) — _test.go suffix gates miniredis/testify out of production binary"
    - "Placeholder bodies use api.X500JSONResponse{InternalServerErrorJSONResponse: {Error: internal, Reason: not_implemented_yet}}"

key-files:
  created:
    - services/api/internal/catalog/handlers.go
    - services/api/internal/catalog/cursor.go
    - services/api/internal/catalog/cursor_test.go
    - services/api/internal/catalog/mappers.go
    - services/api/internal/catalog/errors.go
    - services/api/internal/catalog/notimpl.go
    - services/api/internal/catalog/bypass_placeholders.go
    - services/api/internal/catalog/agents.go
    - services/api/internal/catalog/skills.go
    - services/api/internal/catalog/queues.go
    - services/api/internal/catalog/channels.go
    - services/api/internal/catalog/adapters.go
    - services/api/internal/catalog/break_reasons.go
    - services/api/internal/catalog/agent_skills.go
    - services/api/internal/catalog/testutil_test.go
  modified: []

key-decisions:
  - "Placeholder bodies in entity files (not a shared placeholder file) so Plans 03-06..03-09 REPLACE one file each without inter-plan file conflicts"
  - "Bypass routes (GetDocs, GetHealthz, GetOpenAPISpec, GetReadyz) get minimal 200 placeholders rather than 500 stubs — bypass operations have no 500 wrapper in the generated code (verified via grep)"
  - "Wave 0 server.Wave0TempStubs stays alive as the binary's strict-server impl; Plan 03-10 wires catalog.New(deps) into main.go in a single drop-in swap"
  - "agent_skills.go ships as a package-decl-only file (no methods) — the spec has no standalone /v1/agent_skills resource; CAT-03 lives inside UpdateAgent"

patterns-established:
  - "Compile-time StrictServerInterface guarantee via var _ api.StrictServerInterface = (*Handlers)(nil) — adding a spec method without a catalog impl fails the build"
  - "notImplementedBody() shared helper returns canonical 500 envelope so every placeholder/stub uses the same body"
  - "Cursor encoding uses base64.StdEncoding(json.Marshal(Cursor{CreatedAt, ID})) — opaque, deterministic, replayable"
  - "mapPgError(err, entity) returns (httpStatus, ErrorCode, reason) triple — response-type-agnostic, reusable across all 6 entities"

requirements-completed: []  # Infrastructure-only plan; CAT-* coverage owned by Plans 03-06..09

# Metrics
duration: 18min
completed: 2026-05-16
---

# Phase 3 Plan 05: internal/catalog/ scaffold — Deps + cross-cutting helpers + 7 entity placeholders Summary

**Lays down the `services/api/internal/catalog/` package skeleton — `Handlers` struct with `Deps` + `WithClock` constructor, 5 cursor tests passing, and a compile-time `var _ api.StrictServerInterface = (*Handlers)(nil)` guarantee that proves all 38 spec methods have catalog-package implementations (4 bypass + 30 CRUD placeholders + 4 Phase 4/5 stubs).**

## Performance

- **Duration:** ~18 min (estimated from commit timestamps)
- **Started:** 2026-05-16T14:42Z (approx — first commit after worktree base reset)
- **Completed:** 2026-05-16T14:59:41Z
- **Tasks:** 2 (Task 1 = cross-cutting helpers; Task 2 = handlers + placeholders + testutil)
- **Files created:** 15 (4 cross-cutting + 1 main + 1 notimpl + 1 bypass + 7 entity + 1 testutil)
- **Lines of Go added:** 802 across 15 files

## Accomplishments

- New `services/api/internal/catalog/` package compiles clean (`go build ./internal/catalog/...` exit 0).
- `go vet ./internal/catalog/...` clean.
- `go test -race ./internal/catalog/...` passes 5 cursor tests (round-trip with microsecond precision, empty, bad base64, bad inner JSON, determinism).
- The compile-time assertion `var _ api.StrictServerInterface = (*Handlers)(nil)` proves catalog.Handlers implements every one of the 38 methods in the generated StrictServerInterface.
- Plans 03-06..09 can now run in parallel: each owns one entity file (agents.go, skills.go, etc.) and replaces only its file's bodies without colliding with sibling plans (D-68 layout).
- Cross-cutting helpers (cursor, mappers, errors) are response-type-agnostic so all entity plans reuse them verbatim.

## Task Commits

Each task was committed atomically:

1. **Task 1: cross-cutting helpers (cursor + mappers + errors + cursor_test)** — `430788c` (feat, TDD)
2. **Task 2: handlers.go + notimpl.go + 7 entity placeholders + bypass + testutil_test.go** — `42de24e` (feat)

_Plan metadata commit (this SUMMARY.md) follows below._

## Files Created/Modified

### Cross-cutting helpers (Task 1)

- `services/api/internal/catalog/cursor.go` (74 LOC) — `EncodeCursor`, `DecodeCursor`, `Cursor` struct, `errBadCursor` sentinel. D-63 format: `base64(JSON{created_at:RFC3339Nano, id:UUIDv7})`. Empty input returns `(nil, nil)`; malformed returns `errBadCursor`.
- `services/api/internal/catalog/cursor_test.go` (73 LOC) — 5 tests: round-trip, empty, malformed base64, bad inner JSON, determinism. Uses `require.WithinDuration` with `time.Microsecond` for JSON time-precision tolerance.
- `services/api/internal/catalog/mappers.go` (83 LOC) — `pgUUID`, `pgUUIDFromBytes`, `apiUUID`, `pgTimestamptzPtr`, `pgTextPtr`, `deref[T any]`, `derefOr[T any]`. Pure functions, no I/O.
- `services/api/internal/catalog/errors.go` (47 LOC) — `mapPgError(err, entity)` returning `(httpStatus, ErrorCode, reason)`. Translates `pgx.ErrNoRows` → 404 not_found, 23505 (UNIQUE) → 409 version_conflict, default → 500 internal.

### Package skeleton (Task 2)

- `services/api/internal/catalog/handlers.go` (118 LOC) — `Deps` struct (OrgDB, Pool, Cache, Logger — required), `Option` functional-option type, `WithClock` (the only v0.1 knob per D-71), `Handlers` struct (`deps`, `clock`), `New(Deps, ...Option) *Handlers`, compile-time `var _ api.StrictServerInterface = (*Handlers)(nil)` guard.
- `services/api/internal/catalog/notimpl.go` (62 LOC) — `notImplementedBody()` shared helper + 4 method stubs: `GetAgentStatus`, `PatchAgentStatus` (Phase 4), `BulkImportCatalog`, `GetImportJob` (Phase 5). Each returns operation-specific 500 wrapper with `reason="not_implemented_yet"`.
- `services/api/internal/catalog/bypass_placeholders.go` (85 LOC) — 4 minimal-200 placeholders for `GetDocs` (placeholder HTML), `GetHealthz` (real `{status:alive}`), `GetOpenAPISpec` (placeholder YAML), `GetReadyz` (synthetic `status:ok`). Plan 03-10 replaces with the real bodies (currently in `server.Wave0TempStubs`).

### Per-entity placeholder files (Task 2)

- `services/api/internal/catalog/agents.go` (43 LOC) — 5 methods (`ListAgents`, `CreateAgent`, `DeleteAgent`, `GetAgent`, `UpdateAgent`). Replaced by Plan 03-06.
- `services/api/internal/catalog/skills.go` (36 LOC) — 5 methods. Replaced by Plan 03-07.
- `services/api/internal/catalog/queues.go` (36 LOC) — 5 methods. Replaced by Plan 03-08.
- `services/api/internal/catalog/channels.go` (36 LOC) — 5 methods. Replaced by Plan 03-08.
- `services/api/internal/catalog/adapters.go` (36 LOC) — 5 methods. Replaced by Plan 03-09.
- `services/api/internal/catalog/break_reasons.go` (36 LOC) — 5 methods. Replaced by Plan 03-07.
- `services/api/internal/catalog/agent_skills.go` (13 LOC) — reserved package-decl-only file. Plan 03-09 fills with agent_skills join helpers; the OpenAPI spec exposes no standalone `/v1/agent_skills` resource.
- `services/api/internal/catalog/testutil_test.go` (24 LOC) — `_test.go` suffix (D-73) so miniredis/testify never enter production binary. Currently declares only `var sharedPool *pgxpool.Pool`; Plan 03-06 grows `newTestHandlers` + the testcontainer `TestMain`.

### Total

- 15 files, 802 LOC. Method count breakdown: 30 entity placeholders + 4 notimpl stubs + 4 bypass placeholders = 38 = exactly the StrictServerInterface method count after the Phase 1 scaffold removal landed in Plan 03-01.

## Verification

```
$ cd services/api
$ go build ./internal/catalog/...
   (exit 0)
$ go vet ./internal/catalog/...
   (exit 0)
$ go test -count=1 -race -v ./internal/catalog/...
=== RUN   TestCursorRoundTrip
--- PASS: TestCursorRoundTrip (0.00s)
=== RUN   TestDecodeCursor_Empty
--- PASS: TestDecodeCursor_Empty (0.00s)
=== RUN   TestDecodeCursor_Malformed
--- PASS: TestDecodeCursor_Malformed (0.00s)
=== RUN   TestDecodeCursor_BadJSON
--- PASS: TestDecodeCursor_BadJSON (0.00s)
=== RUN   TestEncodeCursor_Stable
--- PASS: TestEncodeCursor_Stable (0.00s)
PASS
ok      github.com/luongdev/open-routing/services/api/internal/catalog 1.522s
```

StrictServerInterface method count verification:

```
$ awk '/^type StrictServerInterface interface \{/,/^\}/' services/api/internal/api/server.gen.go | grep -cE '^\t[A-Z][a-zA-Z]+\(ctx'
38
$ grep -cE '^func \(h \*Handlers\) ' services/api/internal/catalog/*.go | grep -v cursor_test | grep -v mappers | grep -v errors
agents.go:5   adapters.go:5   break_reasons.go:5   bypass_placeholders.go:4
channels.go:5  notimpl.go:4   queues.go:5  skills.go:5
(0 elsewhere; total = 5+5+5+5+5+5+4+4 = 38)
```

Full-repo sanity:

```
$ go build ./...
   (exit 0)
$ go vet ./...
   (exit 0)
```

## Decisions Made

- **Single placeholder body shape across all 38 methods.** Every CRUD + notimpl placeholder returns `api.{Op}500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}` with `reason="not_implemented_yet"`. Bypass placeholders are the only exception — they return their 200 wrappers (text/html, yaml, JSON) because bypass operations don't have 500 wrappers in the generated code.
- **`notImplementedBody()` lives in notimpl.go.** Both notimpl.go (Phase 4/5 stubs) and the per-entity placeholder files import it. When Plan 03-06..09 fill in real bodies, the helper stays — Phase 4/5 endpoints still call it from notimpl.go.
- **agent_skills.go ships as package-decl-only.** The OpenAPI spec has no standalone `/v1/agent_skills` resource — CAT-03's skills-array semantics live inside `UpdateAgent`. The file exists in this scaffold to claim ownership of the filename so Plan 03-09 can REPLACE it without filename collision with a sibling plan.
- **testutil_test.go is the bare minimum.** Plan 03-05 ships only `var sharedPool *pgxpool.Pool` so the package compiles + cursor_test.go runs. Plan 03-06 grows `newTestHandlers`, `cleanCatalogTables`, http helpers, and the top-level `main_test.go` `TestMain` that brings up the pg testcontainer. Splitting this scope keeps Plan 03-05 small and merge-clean.
- **Wave 0 `server.Wave0TempStubs` is NOT deleted.** Plan 03-05 only creates the catalog package — main.go still wires Wave0TempStubs as the strict-server impl. Plan 03-10 performs the single drop-in swap (`catalog.New(deps)` in place of `NewWave0TempStubs(...)`) after every entity plan has filled in real bodies.

## Deviations from Plan

None — plan executed exactly as written. Two minor judgement calls made on Claude-discretion items, documented above under Decisions Made:

- Plan body left flexibility on agent_skills.go content (empty vs. helper-stubs). Chose empty per the plan's own preference ("agent_skills.go ... created empty / with only a package decl").
- Plan body left flexibility on bypass placeholders. Chose minimal real 200 bodies (HTML/YAML/JSON placeholders) rather than 500 stubs because bypass routes lack 500 wrappers in the generated code. Plan 03-10 swaps to the real bypass implementations.

## Issues Encountered

- **CLAUDE.md GitHub assignee rule misapplied to `git commit`.** The user's global rule mandates `--assignee mpt-luongld` on `gh issue/pr create`; first commit attempt incorrectly passed the flag to `git commit`, which doesn't accept it. Re-ran without the flag. No code impact — the rule still applies to any future `gh` invocation made from this PC.
- **Cursor test runner output silencing.** The Bash tool's wrapper summarised `go test -v` to "1 passed in 1 packages" instead of streaming per-test PASS lines. Used `rtk proxy go test` to get raw test output for verification. Final test run (above) shows all 5 cursor tests passing.

## User Setup Required

None — no external service configuration required for this plan. The catalog package compiles with zero runtime dependencies; tests use stdlib only (cursor tests). Plan 03-06 introduces the Postgres testcontainer requirement.

## Next Phase Readiness

- **Plans 03-06..09 unblocked.** Each can REPLACE its single entity file (agents.go / skills.go+break_reasons.go / queues.go+channels.go / adapters.go+agent_skills.go) with real implementations without conflicting on shared files.
- **Wave 0 Wave0TempStubs remains the production strict-server impl** until Plan 03-10 wires `catalog.New(deps)` in main.go.
- **No blockers.** All compile + vet + cursor-test gates green.

## Self-Check: PASSED

- [x] `services/api/internal/catalog/cursor.go` — FOUND
- [x] `services/api/internal/catalog/cursor_test.go` — FOUND
- [x] `services/api/internal/catalog/mappers.go` — FOUND
- [x] `services/api/internal/catalog/errors.go` — FOUND
- [x] `services/api/internal/catalog/handlers.go` — FOUND
- [x] `services/api/internal/catalog/notimpl.go` — FOUND
- [x] `services/api/internal/catalog/bypass_placeholders.go` — FOUND
- [x] `services/api/internal/catalog/agents.go` — FOUND
- [x] `services/api/internal/catalog/skills.go` — FOUND
- [x] `services/api/internal/catalog/queues.go` — FOUND
- [x] `services/api/internal/catalog/channels.go` — FOUND
- [x] `services/api/internal/catalog/adapters.go` — FOUND
- [x] `services/api/internal/catalog/break_reasons.go` — FOUND
- [x] `services/api/internal/catalog/agent_skills.go` — FOUND
- [x] `services/api/internal/catalog/testutil_test.go` — FOUND
- [x] Commit `430788c` (Task 1) — FOUND in git log
- [x] Commit `42de24e` (Task 2) — FOUND in git log
- [x] StrictServerInterface count 38 = catalog method count 38

---

*Phase: 03-catalog-crud-go*
*Plan: 05*
*Completed: 2026-05-16*
