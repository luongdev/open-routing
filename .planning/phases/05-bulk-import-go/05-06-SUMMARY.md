---
phase: 05-bulk-import-go
plan: 06
subsystem: api
tags:
  - handlers
  - wiring
  - middleware
  - notimpl-delete
  - composite-third-embed

# Dependency graph
requires:
  - phase: 05-bulk-import-go/04
    provides: |
      imports.Importer struct + Deps + lifecycle (Start/Stop);
      lookupIdempotentReplay + rehydrateBulkImportResult (D5-13);
      createJob + finaliseJob (D5-10);
      reMarshalAs[T any] generic helper (F1 accommodation);
      stripBOM + newCSVReader + validateUTF8;
      entityRegistry + validateHeader (D5-05 strict batch);
      mapImportJob + wrapPgError + pgUUID helpers.
  - phase: 05-bulk-import-go/05
    provides: |
      chunk.processChunk (D5-09 chunked-savepoint orchestrator);
      6 per-entity rowProcessor impls (agentRowProc / skillRowProc /
      queueRowProc / channelRowProc / adapterRowProc /
      breakReasonRowProc);
      parsedRow + succeededRow + rowProcessor interface;
      coerceRow's downstream sink (cells map → typed JSON via
      materialiseTypedRaw, added here in this plan).
  - phase: 04-agent-state-machine-go/04
    provides: |
      ApiHandlers composite three-embed pattern (catalog.Handlers +
      state.Server — Phase 5 adds the third *imports.Importer embed);
      D-89 forward-compat seam.
  - phase: 05-bulk-import-go/02
    provides: |
      middleware.BodyLimit factory (50<<20 path-scoped to /v1/orgs/);
      injection point in server.NewMux's MiddlewareFunc slice.

provides:
  - "(*Importer).BulkImportCatalog method — full Pattern 1 dispatch (orgID extract → entity Valid → Idempotency-Key replay → Content-Type dispatch → importJSON / importCSV → runImportPipeline → status decision)"
  - "(*Importer).GetImportJob method — single-row read with FOUND-08 cross-org isolation via composite (id, org_id) WHERE clause"
  - "importJSON helper — eager-decode path with 500-row defence-in-depth (D5-22)"
  - "importCSV helper — stripBOM + UTF-8 validate + header validate + streaming-row loop with row 501 → 413 (D5-22) + *http.MaxBytesError mapping to 413 (D5-21 mid-stream guard)"
  - "runImportPipeline orchestrator — createJob → chunk loop → finaliseJob → status decision (200 / 207 / 422 / 200-empty)"
  - "rowProcessorFor switch — entity → per-entity rowProcessor dispatch (6 entities)"
  - "coerceRow helper — CSV per-cell coercion via entityRegistry callbacks; first failure short-circuits row"
  - "materialiseTypedRaw helper — bridges CSV cells map → JSON-shaped typed Import*Request shape consumed by reMarshalAs[T] in row processors"
  - "ApiHandlers three-embed composite — catalog.Handlers + state.Server + imports.Importer (D-89 / F3 Open Q5)"
  - "importer.Start + importer.Stop wired into cmd/api/main.go's LIFO shutdown chain ahead of stateServer.Stop"
  - "middleware.BodyLimit injection in server.NewMux's chi MiddlewareFunc slice — order: RequestID → orgContext → BodyLimit → uuidv7PathParams → strict-server (Open Q7)"
  - "21 new test functions: 14 TestBulkImportCatalog_ dispatch cases + 4 TestGetImportJob_ cases + 3 TestServerMuxChain_BodyLimit chain-ordering cases"
  - "DELETED: services/api/internal/catalog/notimpl.go (37 lines — BulkImportCatalog + GetImportJob 501 stubs replaced by *imports.Importer embed)"

affects:
  - "05-07 — Wave 5 integration tests (testutil_test.go composite + httptest harness now exercise the full chi mux → BulkImportCatalog / GetImportJob pipeline; testdata/ population still pending)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Three-embed composite ApiHandlers (D-89 evolved across phases: Phase 4 added *state.Server; Phase 5 added *imports.Importer — F3 Open Q5)"
    - "Local const + value-mirror pattern for cross-package literals when a hard import would create a cycle (importBodyLimit in server.go mirrors imports.ImportBodyLimit by value)"
    - "noopImporter test stub pattern — catalog/testutil_test.go local stub satisfies StrictServerInterface without taking a dependency on internal/imports (which already imports internal/catalog)"
    - "Pattern 1 status decision matrix encoded as a 4-way switch over (len(succeeded), len(failed)) at the end of runImportPipeline — 200 / 207 / 422 / 200-empty"
    - "*http.MaxBytesError handling via errors.As at every io.Reader read site in the CSV path; the BodyLimit middleware bounds Content-Length but cannot peek inside io.Reader use, so the handler is the canonical 413 emitter"

key-files:
  created:
    - "services/api/internal/imports/handler_import.go — BulkImportCatalog method + importJSON + importCSV + runImportPipeline + rowProcessorFor + coerceRow + materialiseTypedRaw + buildHeaderReason (~600 lines)"
    - "services/api/internal/imports/handler_import_test.go — 14 TestBulkImportCatalog_ dispatch test cases (~400 lines)"
    - "services/api/internal/imports/handler_get_job.go — GetImportJob method (~90 lines)"
    - "services/api/internal/imports/handler_get_job_test.go — 4 TestGetImportJob_ cases (~155 lines)"
  modified:
    - "services/api/internal/imports/chunk.go — added parsedRow.coerceFailure field for the CSV per-cell coercion failure pre-filter (chunk loop surfaces these as per-row failures BEFORE entering a savepoint)"
    - "services/api/cmd/api/main.go — added third *imports.Importer embed in the ApiHandlers composite; importer.Start synchronous boot; defer importer.Stop in LIFO position (importer drains BEFORE stateServer before pool.Close)"
    - "services/api/internal/server/server.go — middleware.BodyLimit (50<<20, '/v1/orgs/') injected into chi MiddlewareFunc slice between orgContextMiddleware and uuidv7PathParams; package-local importBodyLimit const sourced to avoid package cycle"
    - "services/api/internal/server/server_test.go — 3 new TestServerMuxChain_BodyLimit_* tests pin oversize-block / under-limit-pass / healthz-bypass"
    - "services/api/test/isolation/main_test.go — production-matching three-embed composite; importer.Stop appended to LIFO cleanup"
    - "services/api/internal/catalog/testutil_test.go — local noopImporter stub satisfies StrictServerInterface (cannot embed *imports.Importer due to package cycle: imports depends on catalog)"
  deleted:
    - "services/api/internal/catalog/notimpl.go — 37 lines; BulkImportCatalog + GetImportJob 501 stubs replaced by *imports.Importer embed in cmd/api/main.go composite"

key-decisions:
  - "Phase 5 ApiHandlers composite three-embed pattern locked: ApiHandlers{*catalog.Handlers, *state.Server, *imports.Importer} (RESEARCH §F3 Open Q5). Naming Importer (not Server) avoids Go's duplicate-anonymous-field constraint with *state.Server."
  - "BodyLimit cap referenced via package-local importBodyLimit const in server.go, NOT via imports.ImportBodyLimit. The hard import would create a test-time package cycle (catalog/testutil_test.go is `package catalog` → server → imports → catalog). Cap is single-sourced via the const comment + Phase 5 spec lock (50 MB IMP-07)."
  - "Catalog test fixture uses a local noopImporter stub instead of embedding *imports.Importer. Same package-cycle constraint: internal/imports already depends on internal/catalog. Catalog tests never exercise the import endpoints (those tests live in internal/imports/ + test/isolation/)."
  - "parsedRow gains a coerceFailure *rowError field so CSV per-cell coercion errors (D5-01..D5-08 sentinels) surface as per-row failures BEFORE entering a savepoint. Chunk loop filters these out so they never enter the savepoint scope; row processors never see a row that already failed coercion."
  - "materialiseTypedRaw bridges the CSV cells map and the row-processor reMarshalAs[T any] step by re-shaping coerced cells (esp. []SkillToken → [{skill_code, proficiency}]) into the JSON-friendly shape that ImportAgentRequest expects. Single code path for JSON + CSV downstream."
  - "Idempotency replay short-circuits BEFORE STEP 3 (Content-Type dispatch) per D5-13 / Pattern 1 step 2 — no parsing work for replayed requests."
  - "Status decision encoded as a 4-way switch in runImportPipeline: empty rows → 200 empty; all-succeed → 200; partial → 207; all-fail → 422 (D-37 + Open Q6)."

patterns-established:
  - "Pattern: BulkImportCatalog dispatch flow (RESEARCH §Pattern 1) — every step verbatim from research → research is now the canonical handler reference for v0.2 dispatch additions"
  - "Pattern: rowProcessorFor switch — entity-keyed dispatch table replaces a per-row reflection or switch; new entities in v0.2 add one case + one row_<entity>.go file"
  - "Pattern: package-local mirror const for cross-package literals when a hard import would create a cycle (importBodyLimit in server.go)"

requirements-completed:
  - IMP-01
  - IMP-02
  - IMP-04
  - IMP-05
  - IMP-06
  - IMP-07
  - IMP-08

# Metrics
duration: 19min
completed: 2026-05-17
---

# Phase 5 Plan 06: Top-Level Handlers + ApiHandlers Wiring + BodyLimit Middleware Summary

**Wave 4 of the bulk-import phase ships the controller surface — `BulkImportCatalog` and `GetImportJob` methods on `*imports.Importer`, wired into the `cmd/api/main.go` three-embed `ApiHandlers` composite, with `middleware.BodyLimit` injected into the `server.NewMux` chain and the obsolete `catalog/notimpl.go` deleted.**

## Performance

- **Duration:** ~19 min
- **Started:** 2026-05-17 13:14:36 UTC (commit `5ac82ba` base)
- **Completed:** 2026-05-17 13:34:18 UTC (final commit: `105be4e`)
- **Tasks:** 4 commits + this SUMMARY
- **Files created:** 4 (handler_import.go + handler_import_test.go + handler_get_job.go + handler_get_job_test.go)
- **Files modified:** 6 (chunk.go + cmd/api/main.go + server.go + server_test.go + isolation/main_test.go + catalog/testutil_test.go)
- **Files deleted:** 1 (catalog/notimpl.go)

## Accomplishments

- `(*Importer).BulkImportCatalog` implements the full RESEARCH §Pattern 1 dispatch flow:
  1. Cross-Cutting Pattern 1 — orgID extraction via `orgkey.OrgIDFromContext`.
  2. `?entity=` validation via `api.ImportEntityType.Valid()`.
  3. Idempotency-Key replay lookup BEFORE any work (D5-13 / Pattern 1 step 2).
  4. Content-Type dispatch — `req.JSONBody` → JSON path; `req.Body` + `?schema_version=v0.1` → CSV path; else → 400 `unsupported_content_type`.
  5. `importJSON` enforces 500-row cap (D5-22 defence-in-depth).
  6. `importCSV` runs stripBOM → ReadAll (bounded by upstream BodyLimit) → UTF-8 validate (D5-08) → header validate (D5-05) → streaming row loop with row 501 → 413; `*http.MaxBytesError` catches map to 413 (D5-21 mid-stream guard).
  7. `runImportPipeline`: createJob (D5-10 step 1) → row processor dispatch → chunk loop (50 rows per chunk via processChunk) → finaliseJob (D5-10 step 3) → status decision (200 / 207 / 422 / 200-empty per D-37 + Open Q6).
- `(*Importer).GetImportJob` implements the single-row read with FOUND-08 cross-org isolation: composite `(id, org_id)` WHERE clause in the sqlc query → `pgx.ErrNoRows` on either unknown ID or cross-org probe → 404 `import_job_not_found` (never 403).
- `cmd/api/main.go` ships the three-embed `ApiHandlers` composite: `*catalog.Handlers + *state.Server + *imports.Importer`. The compile-time `var _ api.StrictServerInterface = (*ApiHandlers)(nil)` assertion now holds with the `catalog/notimpl.go` stubs removed.
- `importer.Start` runs synchronously at boot; failure aborts startup. `defer importer.Stop` is registered AFTER `defer stateServer.Stop` so the LIFO defer order drains imports BEFORE state BEFORE the pool closes.
- `server.NewMux` middleware chain has `middleware.BodyLimit(50<<20, "/v1/orgs/")` injected between `orgContextMiddleware` and `uuidv7PathParamsMiddleware` per Open Q7 (cheap header rejection short-circuits BEFORE the body wrap touches r.Body).
- `services/api/internal/catalog/notimpl.go` is **DELETED**. The BulkImportCatalog + GetImportJob 501 stubs are replaced by the `*imports.Importer` embed in the composite.
- **21 new test functions** ship in this wave:
  - 14 `TestBulkImportCatalog_` dispatch cases (≥ 8 required).
  - 4 `TestGetImportJob_` cases (≥ 3 required).
  - 3 `TestServerMuxChain_BodyLimit_` chain-ordering cases (≥ 3 required).

## Task Commits

Each task was committed atomically:

1. **Task 1 (handler_import.go + tests):** `17a2871` — feat(05-06): add BulkImportCatalog handler + dispatch tests
2. **Task 2 (handler_get_job.go + tests):** `1c1609b` — feat(05-06): add GetImportJob handler + cross-org isolation tests
3. **Task 3 (composite wiring + notimpl.go delete):** `91089ec` — feat(05-06): wire *imports.Importer into ApiHandlers + delete notimpl.go
4. **Task 4 (BodyLimit + server_test.go):** `105be4e` — feat(05-06): inject middleware.BodyLimit into NewMux chain + tests

The orchestrator/parent will add the SUMMARY commit alongside.

## Files Created

| File | Lines | Purpose |
|------|------:|---------|
| `services/api/internal/imports/handler_import.go` | ~600 | BulkImportCatalog + importJSON + importCSV + runImportPipeline + rowProcessorFor + coerceRow + materialiseTypedRaw + buildHeaderReason |
| `services/api/internal/imports/handler_import_test.go` | ~400 | 14 TestBulkImportCatalog_ dispatch cases |
| `services/api/internal/imports/handler_get_job.go` | ~90 | GetImportJob method (orgID extract → DB read → 200/404/500 with FOUND-08 isolation) |
| `services/api/internal/imports/handler_get_job_test.go` | ~155 | 4 TestGetImportJob_ cases (MissingOrgID / Found_FullShape / NotFound / CrossOrg) |

## Files Modified

| File | Change |
|------|--------|
| `services/api/internal/imports/chunk.go` | Added `parsedRow.coerceFailure *rowError` field (5 lines) so CSV per-cell coercion errors surface as per-row failures BEFORE entering a savepoint |
| `services/api/cmd/api/main.go` | Added third embed `*imports.Importer` in `ApiHandlers`; importer.Start + defer importer.Stop in LIFO position (importer drains BEFORE stateServer); 25 lines added |
| `services/api/internal/server/server.go` | Injected `middleware.BodyLimit(importBodyLimit, "/v1/orgs/")` in chi MiddlewareFunc slice; added package-local `importBodyLimit int64 = 50 << 20` const to avoid package cycle; 12 lines added |
| `services/api/internal/server/server_test.go` | Added 3 TestServerMuxChain_BodyLimit_ chain-ordering tests; ~85 lines added |
| `services/api/test/isolation/main_test.go` | Updated `apiHandlers` composite to match production three-embed shape; added importer.Start + importer.Stop in LIFO cleanup; 12 lines added |
| `services/api/internal/catalog/testutil_test.go` | Added local `noopImporter` stub (cannot embed *imports.Importer due to package cycle: imports → catalog); updated `testApiHandlers` to embed noopImporter; 32 lines added |

## Files Deleted

| File | Reason |
|------|--------|
| `services/api/internal/catalog/notimpl.go` | 37 lines — BulkImportCatalog + GetImportJob 501 stubs replaced by *imports.Importer embed in cmd/api/main.go composite. The composite's compile-time `var _ api.StrictServerInterface = (*ApiHandlers)(nil)` assertion still holds. |

## Status Decision Matrix (Pattern 1 Step 9 / D-37 / Open Q6)

| succeeded | failed | HTTP status | Response type | Notes |
|-----------|-------:|-------------|---------------|-------|
| 0 | 0 | 200 | BulkImportCatalog200JSONResponse | Empty input — vacuous success (Open Q6 — "zero-rows-after-header is vacuously a success") |
| > 0 | 0 | 200 | BulkImportCatalog200JSONResponse | All-succeed |
| > 0 | > 0 | 207 | BulkImportCatalog207JSONResponse | Partial — multi-status |
| 0 | > 0 | 422 | BulkImportCatalog422JSONResponse | All-fail |

## ApiHandlers Composite Shape (After Plan 05-06)

```go
type ApiHandlers struct {
    *catalog.Handlers   // Phase 3 — 41 methods (CRUD for 6 entities + scaffold)
    *state.Server       // Phase 4 — 2 agent-state status methods (D-89)
    *imports.Importer   // Phase 5 — BulkImportCatalog + GetImportJob (F3 / Open Q5)
}
var _ api.StrictServerInterface = (*ApiHandlers)(nil)
```

The compile-time `StrictServerInterface` assertion succeeds because the three embed sets are disjoint and together cover the full interface. With `catalog/notimpl.go` deleted, catalog.Handlers alone no longer satisfies the interface — Phase 5 promotes the composite to the canonical wiring point.

## BodyLimit Middleware Chain Position (server.NewMux)

```
Recoverer (chi) → RequestID → /metrics route (bare chi) → strict-server pipeline:
    orgContextMiddleware            (gates X-Org-Id BEFORE body wrap)
        → middleware.BodyLimit      (50MB cap, path-scoped /v1/orgs/)
            → uuidv7PathParamsMiddleware
                → strict-server (RequestIDInjectionMiddleware wraps the strict handler)
```

Order rationale per Open Q7: cheap header rejection (`invalid_org_id`) short-circuits BEFORE any body wrap; the 413 emission from the middleware carries the request_id from the chi RequestID middleware that ran above it.

## Plan Acceptance Gates

| Gate | Status | Evidence |
|------|--------|----------|
| handler_import.go exists | PASS | `test -f services/api/internal/imports/handler_import.go` |
| handler_import_test.go exists | PASS | `test -f services/api/internal/imports/handler_import_test.go` |
| `^func (s \*Importer) BulkImportCatalog` count == 1 | PASS | grep == 1 |
| `lookupIdempotentReplay` refs in handler_import.go ≥ 1 | PASS | grep == 1 (call site) |
| `stripBOM` refs in handler_import.go ≥ 1 | PASS | grep == 3 (call + doc) |
| `newCSVReader` refs in handler_import.go ≥ 1 | PASS | grep == 2 |
| `validateHeader` refs in handler_import.go ≥ 1 | PASS | grep == 4 |
| `createJob` refs in handler_import.go ≥ 1 | PASS | grep == 4 |
| `processChunk` refs in handler_import.go ≥ 1 | PASS | grep == 5 |
| `finaliseJob` refs in handler_import.go ≥ 1 | PASS | grep == 4 |
| `*http.MaxBytesError` refs in handler_import.go ≥ 1 | PASS | grep == 6 |
| `^func TestBulkImportCatalog_` count ≥ 8 | PASS | grep == 14 |
| handler_get_job.go exists | PASS | `test -f services/api/internal/imports/handler_get_job.go` |
| handler_get_job_test.go exists | PASS | `test -f services/api/internal/imports/handler_get_job_test.go` |
| `^func (s \*Importer) GetImportJob` count == 1 | PASS | grep == 1 |
| `GetImportJob404JSONResponse` refs ≥ 1 | PASS | grep == 1 |
| `^func TestGetImportJob_` count ≥ 3 | PASS | grep == 4 |
| `*imports.Importer` in main.go ≥ 1 | PASS | grep == 1 |
| `importer.Start` in main.go ≥ 1 | PASS | grep == 1 |
| `defer importer.Stop` in main.go ≥ 1 | PASS | grep == 1 |
| `Importer: importer` in main.go ≥ 1 | PASS | grep == 1 |
| `services/api/internal/catalog/notimpl.go` GONE | PASS | `! test -f .../notimpl.go` |
| `middleware.BodyLimit` in server.go ≥ 1 | PASS | grep == 2 (comments referencing the call) |
| `imports.ImportBodyLimit \| 50<<20` in server.go ≥ 1 | PASS | grep == 1 (`50 << 20` const in importBodyLimit) |
| `TestServerMuxChain_BodyLimit` count ≥ 3 | PASS | grep == 3 |
| `cd services/api && go build ./...` exits 0 | PASS | exit 0 |
| `cd services/api && go vet ./...` exits 0 | PASS | exit 0 |
| `cd services/api && go test -count=1 -short ./internal/imports/...` exits 0 | PASS | 90 PASS in 1 package |
| `cd services/api && go test -count=1 -short ./internal/catalog/...` exits 0 | PASS | 28 PASS in 1 package |
| `cd services/api && go test -count=1 -short ./internal/state/...` exits 0 | PASS | 4 PASS in 1 package |
| `cd services/api && go test -count=1 -short ./internal/server/...` exits 0 | PASS | 121 PASS in 1 package |
| `cd services/api && go test -count=1 -short ./...` exits 0 | PASS | 352 PASS in 17 packages |
| New tests pass against testcontainer | PASS | 21 PASS (14 BulkImportCatalog + 4 GetImportJob + 3 BodyLimit) |

### Verification Gate Output

```text
$ grep -c '^func (s \*Importer) BulkImportCatalog' services/api/internal/imports/handler_import.go = 1
$ grep -c 'lookupIdempotentReplay' services/api/internal/imports/handler_import.go              = 1
$ grep -c 'stripBOM' services/api/internal/imports/handler_import.go                            = 3
$ grep -c 'newCSVReader' services/api/internal/imports/handler_import.go                        = 2
$ grep -c 'validateHeader' services/api/internal/imports/handler_import.go                      = 4
$ grep -c 'createJob' services/api/internal/imports/handler_import.go                           = 4
$ grep -c 'processChunk' services/api/internal/imports/handler_import.go                        = 5
$ grep -c 'finaliseJob' services/api/internal/imports/handler_import.go                         = 4
$ grep -c '\*http.MaxBytesError' services/api/internal/imports/handler_import.go                = 6
$ grep -c '^func TestBulkImportCatalog_' services/api/internal/imports/handler_import_test.go   = 14

$ grep -c '^func (s \*Importer) GetImportJob' services/api/internal/imports/handler_get_job.go  = 1
$ grep -c 'GetImportJob404JSONResponse' services/api/internal/imports/handler_get_job.go        = 1
$ grep -c '^func TestGetImportJob_' services/api/internal/imports/handler_get_job_test.go       = 4

$ grep -c '\*imports.Importer' services/api/cmd/api/main.go                                     = 1
$ grep -c 'importer.Start' services/api/cmd/api/main.go                                         = 1
$ grep -c 'defer importer.Stop' services/api/cmd/api/main.go                                    = 1
$ grep -c 'Importer: importer' services/api/cmd/api/main.go                                     = 1
$ test ! -f services/api/internal/catalog/notimpl.go && echo GONE                               = GONE

$ grep -c 'middleware.BodyLimit' services/api/internal/server/server.go                         = 2
$ grep -cE 'imports\.ImportBodyLimit|50<<20' services/api/internal/server/server.go             = 1
$ grep -c '^func TestServerMuxChain_BodyLimit' services/api/internal/server/server_test.go      = 3

$ cd services/api && go build ./...                                                             → Success
$ cd services/api && go vet ./...                                                               → No issues found
$ cd services/api && go test -count=1 -short ./...                                              → 352 PASS in 17 packages
$ cd services/api && go test -count=1 -run 'TestBulkImportCatalog_|TestGetImportJob_|TestServerMuxChain_BodyLimit' ./... → 21 PASS in 17 packages
```

## Decisions Made

- **Three-embed composite locks D-89 evolution.** Phase 3 used catalog.Handlers as the strict-server impl; Phase 4 added *state.Server (introducing the composite); Phase 5 adds *imports.Importer. The composite is now the canonical wiring point — catalog/notimpl.go is gone, and any future stub-and-replace cycle for a new subsystem (e.g., a v0.2 routing engine) follows the same three-embed-plus-rename pattern. RESEARCH §F3 Open Q5 locked the Importer name to avoid the Go duplicate-anonymous-field constraint with state.Server.
- **Package-local mirror const for BodyLimit cap (server.go).** The first wiring attempt imported `imports.ImportBodyLimit` directly into server.go. This created a package cycle at TEST BUILD time because `services/api/internal/catalog/testutil_test.go` is in `package catalog` and uses `server` — and the new server → imports → catalog chain back-references catalog. Fix: introduced `importBodyLimit int64 = 50 << 20` as a package-local const in server.go that mirrors `imports.ImportBodyLimit` by value, single-sourced via comment cross-reference + the Phase 5 spec lock (IMP-07).
- **Catalog test fixture uses a local noopImporter stub.** Same package-cycle constraint as above — internal/imports already depends on internal/catalog (errors.go imports catalog.MapPgError; header.go imports catalog.ValidateCodeFormat). Catalog tests that need a complete strict-server composite (for the mux they front httptest with) define a local `noopImporter` struct with the two import methods returning 500 not_implemented. The actual import endpoint behaviour is tested in `internal/imports/handler_*_test.go` and (Wave 5) `test/isolation/imports_test.go`.
- **parsedRow gains a coerceFailure field.** The Wave 3 chunk loop expects all rows in its slice to be processable; CSV per-cell coercion errors (D5-01..D5-08) should NOT enter a savepoint because they are not DB-level failures. The handler's pre-chunk filter loop pulls failed-coercion rows aside and emits per-row BulkImportFailedRow entries directly; only post-coercion rows reach `processChunk`. This keeps the chunk loop's invariant clean: every row in a chunk has a savepoint attempt, success or rollback.
- **materialiseTypedRaw bridges CSV → typed JSON shape.** The CSV path produces a `cells map[string]any` per row; the row processors call `reMarshalAs[T any]` expecting JSON-friendly types. The materialiseTypedRaw helper reshapes the cells map into the typed JSON form (esp. `[]SkillToken → [{skill_code, proficiency}]`), so the row processors see the same shape regardless of whether the source was JSON or CSV. Single chunk-loop code path.
- **Status decision matrix encoded at the response boundary.** The 4-way switch in `runImportPipeline` (empty / all-succeed / partial / all-fail) is the canonical decision point. Test fixtures pin all 4 branches.
- **Idempotency replay short-circuits BEFORE parser dispatch.** D5-13 / Pattern 1 step 2 — the handler's idempotency check fires immediately after entity validation, BEFORE any Content-Type dispatch. Replayed requests do zero parser work; the persisted job's BulkImportResult is returned with `idempotent_replay=true`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] catalog/testutil_test.go ApiHandlers composite required noopImporter stub**

- **Found during:** Task 3 (after catalog/notimpl.go deleted, `go vet ./...` reported the catalog test fixture's composite no longer satisfied StrictServerInterface).
- **Issue:** `services/api/internal/catalog/testutil_test.go` is `package catalog` (so it can reach the package's unexported helpers from sibling _test.go files). Adding the *imports.Importer embed would create a build-time package cycle since `internal/imports` already depends on `internal/catalog` (errors.go imports catalog.MapPgError; header.go imports catalog.ValidateCodeFormat). The cycle is `catalog/testutil_test.go (package catalog) → imports → catalog`.
- **Fix:** Defined a local `noopImporter` struct in catalog/testutil_test.go with the two import methods (BulkImportCatalog + GetImportJob) returning 500 `not_implemented_in_catalog_test_fixture`. Updated the local `testApiHandlers` composite to embed `noopImporter` instead of `*imports.Importer`. Production composite in cmd/api/main.go is unaffected.
- **Files modified:** `services/api/internal/catalog/testutil_test.go`
- **Verification:** `go vet ./...` clean; `go test -count=1 -short ./internal/catalog/...` → 28 PASS.
- **Committed in:** `91089ec` (Task 3 commit).

**2. [Rule 3 - Blocking] test/isolation/main_test.go ApiHandlers composite required matching production update**

- **Found during:** Task 3 (same `go vet ./...` run as above).
- **Issue:** `services/api/test/isolation/main_test.go` is `package isolation_test` (external). It owns its own test mux + httptest.Server that mirrors the production wiring. After catalog/notimpl.go's deletion, the isolation suite's `apiHandlers` composite no longer satisfied StrictServerInterface either.
- **Fix:** Added `imports.Importer` import; updated `apiHandlers` composite to the three-embed shape `{*catalog.Handlers, *state.Server, *imports.Importer}`; wired `imports.New(deps).Start(ctx)` synchronously at TestMain bring-up; added `importer.Stop()` to the LIFO cleanup sequence. No cycle here because the package is `isolation_test` (external test package).
- **Files modified:** `services/api/test/isolation/main_test.go`
- **Verification:** `go vet ./...` clean; full project test build green.
- **Committed in:** `91089ec` (Task 3 commit).

**3. [Rule 3 - Blocking] server.go's imports.ImportBodyLimit reference created a test-time package cycle**

- **Found during:** Task 4 first verification run (`go test -count=1 -short ./...` after first wiring attempt).
- **Issue:** Initial wiring used `appmw.BodyLimit(imports.ImportBodyLimit, "/v1/orgs/")` in server.go. Result: `go test` reported `import cycle not allowed in test: catalog → server → imports → catalog` because catalog/testutil_test.go (package catalog) pulls in server.go, which now pulls in imports, which pulls in catalog.
- **Fix:** Introduced `importBodyLimit int64 = 50 << 20` as a package-local const in server.go, mirroring `imports.ImportBodyLimit` by value. The const carries a comment cross-referencing the Phase 5 spec lock (IMP-07) so the single-source contract is preserved at the doc level even though the literal is duplicated. Removed the `imports` import from server.go and server_test.go.
- **Files modified:** `services/api/internal/server/server.go`, `services/api/internal/server/server_test.go`
- **Verification:** `go test -count=1 -short ./...` → 352 PASS in 17 packages; `go vet ./...` clean.
- **Committed in:** `105be4e` (Task 4 commit).

---

**Total deviations:** 3 auto-fixed (all Rule 3 blocking — test-time package cycle issues stemming from the imports → catalog dependency direction established in Plan 05-04). All resolved inline without architectural changes; the underlying composite + middleware patterns ship as planned.

## Issues Encountered

None beyond the three auto-fixes above. The handler control flow followed RESEARCH §Pattern 1 verbatim; the chunk loop integration was a clean caller into the Wave 3 processChunk; the composite update followed Phase 4's D-89 pattern.

## Threat Mitigation Confirmation

| Threat | Mitigation | Verified |
|--------|------------|----------|
| T-05-06-01 (DoS — body-size at handler level) | BodyLimit middleware wired in server.go + handler-side `*http.MaxBytesError` catches in importCSV | TestServerMuxChain_BodyLimit_BlocksOversizeBody passes; `*http.MaxBytesError` grep count = 6 in handler_import.go |
| T-05-06-02 (Info Disclosure — cross-org GetImportJob probe) | Composite `(id, org_id)` WHERE in sqlc query; cross-org returns pgx.ErrNoRows → 404 | TestGetImportJob_CrossOrg_404 passes against real Postgres testcontainer |
| T-05-06-03 (Tampering — idempotency replay returning stale data) | Replay returns PERSISTED prior result with idempotent_replay=true; D5-13 KNOWN LIMITATION documented (succeeded[] empty on replay) | TestBulkImportCatalog_IdempotencyHit_ReturnsReplay asserts both replay flag and empty succeeded[] |
| T-05-06-04 (Tampering — empty CSV body misclassified as 422) | runImportPipeline's empty-rows guard returns 200 with empty arrays (Open Q6) | TestBulkImportCatalog_ZeroRowsAfterHeader_200_EmptyArrays + TestBulkImportCatalog_EmptyCSVBody_200_EmptyArrays pass |
| T-05-06-05 (Info Disclosure — catalog/notimpl.go leaking 500 stubs after Phase 5 ships) | File DELETED in Task 3; composite ApiHandlers (catalog + state + imports) satisfies the full StrictServerInterface | catalog/notimpl.go is GONE; compile-time `var _ api.StrictServerInterface = (*ApiHandlers)(nil)` assertion holds |
| T-05-06-06 (DoS — BodyLimit applied too broadly) | 50 MB cap is non-restrictive for CRUD bodies; documented as v0.2 candidate for tighter per-path caps | TestServerMuxChain_BodyLimit_PassesUnderLimit confirms small bodies pass; TestServerMuxChain_BodyLimit_BypassesHealthz confirms bypass routes are untouched |
| T-05-06-07 (Tampering — Importer Start failure leaving zombie ctx) | Start is idempotent via startMu; failure resets started=false + cancels ctx; main.go returns 1 on Start error so kubernetes restarts | Plan 05-04 already verified via TestImportSweep_StartCancellation |
| T-05-06-SC (Tampering — imports package importing catalog circular dep risk) | Confirmed one-way: imports → catalog (errors.go MapPgError + header.go ValidateCodeFormat). Catalog does NOT import imports; the test composites work around via local stub + main_test.go external package | Compile cycle would have surfaced as `import cycle not allowed`; explicit fixes for catalog/testutil_test.go and server.go cleared all transitive cycles |

## Next Phase Readiness

- **Wave 5 (Plan 05-07 — integration tests):** ready. The full request → mux → BulkImportCatalog → chunk loop → finaliseJob → response pipeline is wired. testutil_test.go in internal/imports/ already composes the Importer fixture; Wave 5 will extend it with httptest harness + testdata/ population (`windows-excel-agents.csv`, the entity × format matrix). The composite in test/isolation/main_test.go is now identical to production, so Wave 5's cross-org isolation tests for BulkImportCatalog + GetImportJob have a direct path.
- **No blockers.** Phase 5 strict-server surface is complete at the dispatch level; only integration tests remain.

## Self-Check

| Claim | Status |
|-------|--------|
| 4 files created in services/api/internal/imports/ (2 handlers + 2 tests) | FOUND |
| 6 files modified (chunk.go + main.go + server.go + server_test.go + isolation/main_test.go + catalog/testutil_test.go) | FOUND |
| 1 file deleted (services/api/internal/catalog/notimpl.go) | FOUND |
| Commit `17a2871` (Task 1 — handler_import.go + tests) | FOUND |
| Commit `1c1609b` (Task 2 — handler_get_job.go + tests) | FOUND |
| Commit `91089ec` (Task 3 — composite wiring + notimpl.go delete) | FOUND |
| Commit `105be4e` (Task 4 — BodyLimit + server_test.go) | FOUND |
| BulkImportCatalog method on *Importer | FOUND |
| GetImportJob method on *Importer | FOUND |
| ApiHandlers three-embed composite in cmd/api/main.go | FOUND |
| importer.Start + defer importer.Stop in cmd/api/main.go | FOUND |
| catalog/notimpl.go DELETED | FOUND |
| middleware.BodyLimit injection in server.go (string literal grep) | FOUND |
| TestBulkImportCatalog_ count = 14 (≥ 8 required) | FOUND |
| TestGetImportJob_ count = 4 (≥ 3 required) | FOUND |
| TestServerMuxChain_BodyLimit count = 3 (≥ 3 required) | FOUND |
| go build ./... clean | FOUND |
| go vet ./... clean | FOUND |
| go test -count=1 -short ./... 352 PASS in 17 packages | FOUND |
| StrictServerInterface compile-time assertion still holds | FOUND |
| New tests pass against testcontainer (21 total) | FOUND |

## Self-Check: PASSED

---
*Phase: 05-bulk-import-go*
*Completed: 2026-05-17*
