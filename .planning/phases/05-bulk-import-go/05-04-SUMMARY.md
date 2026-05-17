---
phase: 05-bulk-import-go
plan: 04
subsystem: api
tags:
  - imports-package
  - go
  - clockwork
  - sweep
  - coercion
  - csv
  - json
  - idempotency

# Dependency graph
requires:
  - phase: 05-bulk-import-go/01
    provides: |
      OpenAPI Import*Request schemas (ImportAgentRequest /
      ImportSkillRequest / …), ImportJob wire shape with status enum,
      Idempotency-Key header param, BulkImportResult.IdempotentReplay
      field, BulkImportCatalogJSONBody = []interface{} (F2 invariant)
  - phase: 05-bulk-import-go/02
    provides: |
      catalog.ValidateCodeFormat + catalog.MapPgError exports,
      *OrgTx.BeginSavepoint, http.MaxBytesReader middleware, sqlcheck
      tenantTables += "import_jobs"
  - phase: 05-bulk-import-go/03
    provides: |
      sqlc-generated InsertImportJob / FinaliseImportJob / GetImportJob
      / LookupImportJobByIdempotencyKey / SweepCrashedImportJobs +
      MergeAgentSkill / ResolveSkillCodes / InsertAgentStateOnConflictNothing.
      Migration 000003_create_import_jobs.
  - phase: 04-agent-state-machine-go/04
    provides: |
      state.Server lifecycle template (handlers.go / ttl.go /
      main_test.go / testutil_test.go patterns mirrored verbatim,
      clockwork.Clock seam, per-org DELETE test isolation, slog
      bypass-audit shape)
  - phase: 04.1-catalog-identity-normalization/03
    provides: |
      UpsertXByCode + GetXByCode primitives + D04_1-03 code regex
      `^[a-z][a-z0-9_]{0,63}$`
provides:
  - "services/api/internal/imports/ package skeleton (19 files)"
  - "imports.Importer struct with Start/Stop lifecycle (synchronous startup sweep before goroutine spawn; idempotent)"
  - "Typed coercion pipeline D5-01..D5-08: trimLower, parseBool, parseInt, splitMulti, validateEnum, validateUTF8"
  - "7 import-specific sentinel errors (ErrInvalidBool, ErrMissingRequired, ErrInvalidEnum, ErrInvalidInt, ErrCSVNotUTF8, ErrInvalidSkillToken, ErrInvalidCodeFormat)"
  - "reMarshalAs[T any] generic helper (F1 accommodation for strict-server eager-decode)"
  - "stripBOM + newCSVReader (strict-quote / LazyQuotes=false / FieldsPerRecord=0)"
  - "entityRegistry per-entity column registry (6 entities × N columns) + validateHeader (D5-05 strict batch policy)"
  - "coerceSkillsCell parser for D5-17 `code:prof|code:prof` syntax"
  - "mapImportJob sqlc-row → api.ImportJob wire conversion (mappers.go)"
  - "rowError + wrapPgError thin adapter over catalog.MapPgError export"
  - "(*Importer).createJob + finaliseJob lifecycle helpers (D5-10)"
  - "(*Importer).lookupIdempotentReplay + rehydrateBulkImportResult (D5-13 replay, KNOWN LIMITATION: succeeded[] empty)"
  - "safetySweep + runSweepPastDue + startupSweep mirror of state/ttl.go (D5-11; SOLE org-agnostic SQLChecker carve-out via db.WithBypass)"
  - "9 clockwork-backed sweep tests + 34 table-driven unit tests"
affects:
  - "05-05 — Wave 3 row processors + chunk loop (consumes coerceXxx callbacks, entityRegistry, wrapPgError, SkillToken type)"
  - "05-06 — Wave 4 handler wiring (consumes Importer.BulkImportCatalog / GetImportJob method bodies — added there)"
  - "05-07 — Wave 5 integration tests (consumes testutil_test.go scaffolding; populates testdata/)"

# Tech tracking
tech-stack:
  added:
    - "github.com/jonboulle/clockwork (promoted indirect → direct in services/api/go.mod)"
  patterns:
    - "Single-owner background-goroutine pattern (Importer struct mirrors state.Server lifecycle verbatim — startMu + ctx + cancel + wg)"
    - "Matrix-as-data column registry (entityRegistry map[api.ImportEntityType]entityColumnRegistry — mirrors state/transitions.go)"
    - "Generic reMarshalAs[T] helper accommodating F1 eager-decode pattern"
    - "SOLE org-agnostic SQL carve-out via two distinct db.WithBypass markers (`import_crash_sweep` + `import_crash_sweep.startup`) for audit-log distinction"
    - "Sentinel-error-per-D5-decision pattern (one var ErrXxx per failure mode, all standalone declarations so plan grep gates find them)"
    - "Test fixture per-org DELETE cleanup (cleanImportTables mirrors state.cleanStateTables — D-73 isolation)"

key-files:
  created:
    - "services/api/internal/imports/doc.go — package documentation + D5-NN invariant index"
    - "services/api/internal/imports/handlers.go — Importer struct + Deps + Option + New + Start + Stop"
    - "services/api/internal/imports/coerce.go — typed pipeline + 7 sentinel errors"
    - "services/api/internal/imports/coerce_test.go — 35 table-driven cases"
    - "services/api/internal/imports/parser_json.go — reMarshalAs[T] generic helper"
    - "services/api/internal/imports/parser_json_test.go — 5 round-trip cases"
    - "services/api/internal/imports/parser_csv.go — stripBOM + newCSVReader"
    - "services/api/internal/imports/parser_csv_test.go — 10 BOM+CRLF+quote cases"
    - "services/api/internal/imports/header.go — entityRegistry + validateHeader + coerceXxx callbacks"
    - "services/api/internal/imports/header_test.go — 14 strict-batch + skill-token cases"
    - "services/api/internal/imports/mappers.go — mapImportJob + pgUUID + ptrToTime"
    - "services/api/internal/imports/errors.go — rowError + wrapPgError"
    - "services/api/internal/imports/jobs.go — createJob + finaliseJob"
    - "services/api/internal/imports/idempotency.go — lookupIdempotentReplay + rehydrateBulkImportResult"
    - "services/api/internal/imports/sweep.go — safetySweep + runSweepPastDue + startupSweep"
    - "services/api/internal/imports/sweep_test.go — 9 clockwork-backed sweep cases"
    - "services/api/internal/imports/main_test.go — TestMain + testcontainer + migrations"
    - "services/api/internal/imports/testutil_test.go — TestImports fixture + cleanImportTables + raw-SQL seed helpers"
    - "services/api/internal/imports/testdata/.gitkeep — Wave 5 placeholder"
  modified:
    - "services/api/go.mod — clockwork promoted indirect → direct"

key-decisions:
  - "Importer struct name (NOT Server, NOT Handlers) resolves F3 / Open Q5 — Go disallows duplicate anonymous-field type names in the cmd/api ApiHandlers composite (catalog.Handlers + state.Server + imports.Importer)"
  - "Placeholder safetySweep + startupSweep stubs lived briefly in handlers.go between Task 1 and Task 4 so each task commits a standalone-buildable diff; Task 4 deleted them when sweep.go shipped the real methods"
  - "Sentinel errors declared as standalone `var ErrX = …` lines (not grouped `var (…)`) so the plan's grep gate `^var ErrInvalidBool` resolves verbatim and future authors don't accidentally hide a sentinel in a group"
  - "wrapPgError emits `<code>:<constraint-detail>` (e.g. `duplicate_code:agents`) — never echoes raw row content (T-05-04-02 mitigation + OQ-2A lock on BulkImportFailedRow.reason)"
  - "rehydrateBulkImportResult enforces D5-13 KNOWN LIMITATION: Succeeded[] is always empty + IdempotentReplay=true; documented in idempotency.go header comment"
  - "Both bypass markers (`import_crash_sweep` + `import_crash_sweep.startup`) are locked at the call sites; orgdb.preflight emits each one as a distinct slog event so the audit log distinguishes 1h ticks from startup-time calls"

patterns-established:
  - "Pattern: TestMain bring-up — every internal/* package using testcontainers gets its own TestMain with a unique DB name (state_test → catalog_test → imports_test) so logs are grep-able per package"
  - "Pattern: TestImports fixture composition — Importer + OrgDB (panic mode) + miniredis-backed cache + fakeClock per test, with t.Cleanup wiring Stop()"
  - "Pattern: Raw-SQL seed helpers (seedPendingImportJob) for cases where sqlc-generated query hardcodes a column value the test needs to control (status='pending' + custom updated_at)"

requirements-completed:
  - IMP-02
  - IMP-04
  - IMP-06
  - IMP-08

# Metrics
duration: 16min
completed: 2026-05-17
---

# Phase 5 Plan 04: Imports Package Skeleton + Sweep Summary

**Bulk-import scaffolding shipped — 19-file `services/api/internal/imports/` package with typed coercion pipeline, BOM-safe CSV reader, F1-accommodation re-marshal helper, idempotency replay path, and a clockwork-backed crash-recovery sweep mirroring Phase 4's state.Server pattern verbatim.**

## Performance

- **Duration:** 16 min
- **Started:** 2026-05-17 (first commit: `f6a3d03`)
- **Completed:** 2026-05-17 (final commit: `cd3f623`)
- **Tasks:** 4 (plus one go.mod tidy chore)
- **Files created:** 19 (1 doc + 14 source + 4 test scaffolding)
- **Files modified:** 1 (go.mod — clockwork indirect → direct)

## Accomplishments

- 19 new files in `services/api/internal/imports/` — package compiles, vets clean, and passes its full unit test suite under `-short` (34 PASS + 9 SKIP) and against a real Postgres testcontainer (43 PASS).
- `imports.Importer` struct ships with the full Start/Stop lifecycle: synchronous `startupSweep` before goroutine spawn, idempotent under `startMu`, drains cleanly on `Stop` (matches state.Server contract).
- Typed coercion pipeline encoding D5-01..D5-08 + D5-15 + D5-17 invariants: `trimLower`, `parseBool` (loose grammar), `parseInt` (truncating-float pipeline), `splitMulti` (priority `|` > `;` > `,`), `validateEnum` (case-insensitive lowercase), `validateUTF8`, plus `coerceSkillsCell` parser for the `code:prof|code:prof` agent-skills syntax.
- `reMarshalAs[T any]` generic helper accommodates the F1 strict-server eager-decode pattern (request body decoded into `[]interface{}` before handler runs; we re-marshal per row into the typed `Import*Request` shape).
- Strict header validator + per-entity column registry (`entityRegistry` map keyed by `api.ImportEntityType`) covering all 6 entities × N columns each, mirrored 1:1 from `Import*Request` schemas in `api/types.gen.go`.
- D5-11 crash-recovery sweep: `safetySweep` (1h ticker goroutine) + `runSweepPastDue` (per-tick body with bounded 30s ctx) + `startupSweep` (synchronous startup-time flavour), all routing through `db.WithBypass` so the SOLE org-agnostic query in the codebase has the expected audit-trail markers.
- 9 sweep tests pass against a real Postgres testcontainer (verified locally with OrbStack docker). Test inventory: 44 total test functions across the package.

## Task Commits

Each task was committed atomically:

1. **Task 1: Scaffold imports package skeleton (Importer + lifecycle)** — `f6a3d03` (feat)
2. **Task 2: Author coerce + parsers + header registry + table tests** — `6cd08dc` (feat)
3. **Task 3: Author mappers + errors + jobs + idempotency** — `6b9205f` (feat)
4. **Task 4: Author sweep + tests + main_test scaffolding (D5-11)** — `a7f44a6` (feat)

**Plan metadata commit:** `cd3f623` (chore — go.mod clockwork tidy)

## Files Created/Modified

### Created (19 files)

| File | Lines | Purpose |
|------|------:|---------|
| `services/api/internal/imports/doc.go` | 102 | Package doc + D5-NN invariant index + wave file map |
| `services/api/internal/imports/handlers.go` | 156 | Importer struct + Deps + Option + New + Start + Stop |
| `services/api/internal/imports/coerce.go` | 165 | Typed pipeline + 7 sentinel errors |
| `services/api/internal/imports/coerce_test.go` | 198 | 35 table-driven cases |
| `services/api/internal/imports/parser_json.go` | 53 | reMarshalAs[T] generic helper |
| `services/api/internal/imports/parser_json_test.go` | 107 | 5 round-trip cases |
| `services/api/internal/imports/parser_csv.go` | 81 | stripBOM + newCSVReader |
| `services/api/internal/imports/parser_csv_test.go` | 134 | 10 BOM+CRLF+quote cases |
| `services/api/internal/imports/header.go` | 240 | entityRegistry + validateHeader + coerce callbacks |
| `services/api/internal/imports/header_test.go` | 191 | 14 strict-batch + skill-token cases |
| `services/api/internal/imports/mappers.go` | 82 | mapImportJob + pgUUID + ptrToTime |
| `services/api/internal/imports/errors.go` | 47 | rowError + wrapPgError |
| `services/api/internal/imports/jobs.go` | 81 | createJob + finaliseJob |
| `services/api/internal/imports/idempotency.go` | 95 | lookupIdempotentReplay + rehydrate |
| `services/api/internal/imports/sweep.go` | 109 | safetySweep + runSweepPastDue + startupSweep |
| `services/api/internal/imports/sweep_test.go` | 277 | 9 clockwork-backed sweep cases |
| `services/api/internal/imports/main_test.go` | 124 | TestMain + testcontainer + migrations |
| `services/api/internal/imports/testutil_test.go` | 175 | TestImports fixture + cleanImportTables + seed |
| `services/api/internal/imports/testdata/.gitkeep` | 8 | Wave 5 placeholder marker |

### Modified (1 file)

| File | Change |
|------|--------|
| `services/api/go.mod` | `github.com/jonboulle/clockwork v0.4.0` promoted from indirect to direct (consumed by handlers.go + sweep.go + testutil_test.go) |

## Plan Acceptance Gates

| Gate | Status | Notes |
|------|--------|-------|
| 19 new files in `services/api/internal/imports/` | PASS | `find services/api/internal/imports -type f \| wc -l` = 19 |
| `imports.Importer` struct present (NOT Server) | PASS | `grep -c '^type Importer struct' services/api/internal/imports/handlers.go` = 1 |
| `reMarshalAs[T any]` generic helper | PASS | `grep -c '^func reMarshalAs\[T any\]' services/api/internal/imports/parser_json.go` = 1 |
| `sweep.go` mirrors `state/ttl.go` pattern (clockwork + startup + ticker + WithBypass) | PASS | Both bypass markers locked: `import_crash_sweep` + `import_crash_sweep.startup` |
| `cd services/api && go build ./...` | PASS | Full project builds cleanly |
| `cd services/api && go vet ./...` | PASS | No vet issues across all packages |
| `cd services/api && go test -count=1 -short ./internal/imports/...` | PASS | 34 PASS + 9 SKIP (sweep tests need testcontainer) |
| Sweep tests (≥ 4 cases) pass under real testcontainer | PASS | 9 PASS (StartCancellation / PendingOver24h_OnStart / PendingUnder24h_NoOp / PendingOver24h_OnTick / ServerCrashErrorShape / RunSweepPastDue / CrossOrgIsolation / HandlesEmptyTable / StopBeforeStart) |
| All grep gates from PLAN.md acceptance criteria | PASS | See "Verification Gate Output" below |

### Verification Gate Output

```text
$ grep -c '^package imports' services/api/internal/imports/doc.go                   = 1
$ grep -c '^type Importer struct' services/api/internal/imports/handlers.go        = 1
$ grep -c '^type Deps struct' services/api/internal/imports/handlers.go            = 1
$ grep -c '^func New(' services/api/internal/imports/handlers.go                   = 1
$ grep -c '^func (s \*Importer) Start' services/api/internal/imports/handlers.go   = 1
$ grep -c '^func (s \*Importer) Stop' services/api/internal/imports/handlers.go    = 1
$ grep -c 'defaultSweepInterval' services/api/internal/imports/handlers.go         = 5

$ grep -c '^var ErrInvalidBool ' services/api/internal/imports/coerce.go           = 1
$ grep -c '^func parseBool' services/api/internal/imports/coerce.go                = 1
$ grep -c '^func parseInt' services/api/internal/imports/coerce.go                 = 1
$ grep -c '^func splitMulti' services/api/internal/imports/coerce.go               = 1
$ grep -c '^func validateEnum' services/api/internal/imports/coerce.go             = 1
$ grep -c '^func validateUTF8' services/api/internal/imports/coerce.go             = 1
$ grep -c '^func reMarshalAs\[T any\]' services/api/internal/imports/parser_json.go = 1
$ grep -c '^func stripBOM' services/api/internal/imports/parser_csv.go             = 1
$ grep -c '^func newCSVReader' services/api/internal/imports/parser_csv.go         = 1
$ grep -c '^var entityRegistry ' services/api/internal/imports/header.go           = 1
$ grep -c '^func validateHeader' services/api/internal/imports/header.go           = 1

$ grep -c '^func mapImportJob' services/api/internal/imports/mappers.go            = 1
$ grep -c '^func wrapPgError' services/api/internal/imports/errors.go              = 1
$ grep -c '^func (s \*Importer) createJob' services/api/internal/imports/jobs.go   = 1
$ grep -c '^func (s \*Importer) finaliseJob' services/api/internal/imports/jobs.go = 1
$ grep -c '^func (s \*Importer) lookupIdempotentReplay' services/api/internal/imports/idempotency.go = 1
$ grep -c '^func rehydrateBulkImportResult' services/api/internal/imports/idempotency.go = 1

$ grep -c '^func (s \*Importer) safetySweep' services/api/internal/imports/sweep.go     = 1
$ grep -c '^func (s \*Importer) runSweepPastDue' services/api/internal/imports/sweep.go = 1
$ grep -c '^func (s \*Importer) startupSweep' services/api/internal/imports/sweep.go    = 1
$ grep -c 'db.WithBypass(ctx, "import_crash_sweep")' services/api/internal/imports/sweep.go         = 1
$ grep -c 'db.WithBypass(ctx, "import_crash_sweep.startup")' services/api/internal/imports/sweep.go = 1
$ grep -c '^func TestImportSweep_' services/api/internal/imports/sweep_test.go     = 9

$ cd services/api && go vet ./internal/imports/...   → No issues found
$ cd services/api && go build ./internal/imports/... → Success
$ cd services/api && go test -count=1 -short ./internal/imports/... → 34 PASS + 9 SKIP
```

## Decisions Made

- **Importer struct name** resolves F3 / Open Q5: Go disallows two anonymous embedded fields with the same type name in a struct. The cmd/api `ApiHandlers` composite already embeds `*catalog.Handlers` and `*state.Server` (D-89). Adding `*imports.Server` would collide with `*state.Server`. Renaming the Phase 5 struct to `Importer` keeps Wave 4 wiring trivial — `imports.Importer` is unambiguous and reads better at the call site than `Server`.
- **Placeholder methods strategy:** Task 1's handlers.go briefly carried no-op `startupSweep` + `safetySweep` stubs so each task commits a standalone-buildable diff (Task 1 verify gate runs `go vet`). Task 4 deleted the placeholders when sweep.go shipped the real implementations. The intermediate state is documented in the Task 1 commit message and the `//nolint:unused` markers on the stubs.
- **Sentinel-error declaration style:** Each `var ErrXxx = errors.New(...)` declared as a standalone line (not grouped `var ( ... )`). The plan's verify gate uses `grep -c '^var ErrInvalidBool '` which would not find a grouped declaration. Standalone declarations also make it harder to accidentally hide a new sentinel inside a group when future authors edit the file.
- **rehydrateBulkImportResult known limitation (D5-13):** `Succeeded` is always `[]` + `IdempotentReplay=true` on replay because the persisted `import_jobs` row stores counters only. Documented prominently in `idempotency.go` header comment. v0.2 deferred work: persist `succeeded_ids JSONB` so replays can return the full UUID set.
- **wrapPgError reason format:** `<code>:<constraint-detail>` (e.g. `duplicate_code:agents`). This preserves both the machine-readable code (closed enum from `api.ErrorCode`) and the constraint-detail half (whatever `catalog.MapPgError` emitted) in a single `reason` string field without re-echoing raw row content. T-05-04-02 mitigation; OQ-2A lock on `BulkImportFailedRow.reason`.
- **Both bypass markers locked at call sites:** `import_crash_sweep` (1h tick) and `import_crash_sweep.startup` (synchronous startup) emit distinct `slog.Warn(event=orgdb_bypass, reason=...)` audit events. The orgdb-preflight log distinguishes the two paths cleanly.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Sweep test asserted JSON substring without accounting for Postgres JSONB whitespace normalization**

- **Found during:** Task 4 (`TestImportSweep_ServerCrashErrorShape` first run)
- **Issue:** Test asserted `require.Contains(t, s, "\"row\":0")` against the persisted JSONB payload, but Postgres re-formats JSONB output with whitespace after `:` and `,` (`"row": 0` not `"row":0`).
- **Fix:** Replaced substring assertions with `json.Unmarshal` into a typed `map[string]any` shape, then `require.Equal` on each field. More robust against future Postgres formatting changes and gives clearer assertion errors when the contract drifts.
- **Files modified:** `services/api/internal/imports/sweep_test.go`
- **Verification:** Test passes against real Postgres testcontainer (verified via `rtk proxy go test -run 'TestImportSweep_ServerCrashErrorShape' -v`).
- **Committed in:** `a7f44a6` (Task 4 commit)

**2. [Rule 3 - Blocking] go.mod required `clockwork` promotion to direct dependency**

- **Found during:** Task 4 cleanup (post-`go mod tidy`)
- **Issue:** `clockwork` was previously a transitive dep (via Phase 4's `state` package); Phase 5's `imports` package now consumes it directly (handlers.go + sweep.go + testutil_test.go).
- **Fix:** Ran `go mod tidy`; result was a single line move in go.mod (indirect → direct). No version change.
- **Files modified:** `services/api/go.mod`
- **Verification:** `go build ./...` + `go vet ./...` clean across the full project.
- **Committed in:** `cd3f623` (chore commit; isolated from Task 4 feat commit for clarity)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking).
**Impact on plan:** Both fixes were minimal and necessary. The sweep-test JSON normalization fix improves the test's robustness against driver formatting drift. The go.mod tidy reflects an inevitable consequence of the new direct consumer.

## Issues Encountered

None beyond the two auto-fixes documented above. The plan's task ordering accommodated the placeholder-stub strategy needed by Go's "no duplicate method on receiver" rule. All other tasks executed exactly as written.

## Testutil Deferrals for Wave 5 (Plan 05-07)

`testutil_test.go` ships the minimal fixture Wave 2 needs:

- `TestImports{ I, OrgDB, Pool, OrgID, FakeClk, Logger }`
- `newTestImports(t)` — constructor.
- `cleanImportTables(t, ctx, pool, orgID)` — per-org DELETE.
- `seedPendingImportJob` + `fetchImportJobStatus` — raw-SQL helpers for sweep tests.

**Wave 5 will extend with:**

- Composite `*ApiHandlers` wiring (catalog.Handlers + state.Server + imports.Importer) — Wave 4 (Plan 05-06) adds the BulkImportCatalog / GetImportJob method bodies first.
- `httptest.Server` fronting `server.NewMux` so handler-level integration tests exercise the full middleware chain.
- HTTP helpers (`httpPOST`, `httpPOSTCSV`, `httpGET`) — pattern mirrors state/testutil_test.go's helpers.
- `testdata/` population: `windows-excel-agents.csv` (UTF-8 BOM + CRLF — PITFALLS 5.1 NON-NEGOTIABLE), `invalid-code-format.csv`, and the per-entity matrix.

## Threat Flags

None — Plan 05-04 adds no new network surface (no new endpoints, no new auth paths). The sweep goroutine's `db.WithBypass` is the SOLE org-agnostic SQL carve-out and the plan's threat register T-05-04-01..T-05-04-SC already covers it.

## Next Phase Readiness

- **Wave 3 (Plan 05-05 — row processors + chunk loop):** ready. All scaffolding consumers — `coerceXxx` callbacks, `entityRegistry`, `SkillToken`, `wrapPgError`, `createJob`/`finaliseJob` — are exported (or package-internal but reachable from Wave 3 files since they share `package imports`).
- **Wave 4 (Plan 05-06 — handler wiring):** ready. `Importer.BulkImportCatalog` and `Importer.GetImportJob` method bodies will compose `lookupIdempotentReplay` + `createJob` + chunk loop (from Wave 3) + `finaliseJob`. `mapImportJob` is ready for `GetImportJob`.
- **Wave 5 (Plan 05-07 — integration tests):** ready. `main_test.go` brings up testcontainer Postgres; `testutil_test.go` provides the per-test fixture; `testdata/` directory exists awaiting CSV/JSON files.
- **No blockers.** The `catalog/notimpl.go` stubs for `BulkImportCatalog` + `GetImportJob` remain in place — Plan 05-06 will delete them and wire the real `Importer` methods into the `ApiHandlers` composite.

## Self-Check

| Claim | Status |
|-------|--------|
| 19 files present in `services/api/internal/imports/` | FOUND |
| Commit `f6a3d03` (Task 1 — scaffold) | FOUND |
| Commit `6cd08dc` (Task 2 — coerce+parsers+header) | FOUND |
| Commit `6b9205f` (Task 3 — mappers+errors+jobs+idempotency) | FOUND |
| Commit `a7f44a6` (Task 4 — sweep+tests) | FOUND |
| Commit `cd3f623` (go.mod tidy) | FOUND |
| Importer struct in handlers.go | FOUND |
| safetySweep / runSweepPastDue / startupSweep in sweep.go | FOUND |
| `db.WithBypass(ctx, "import_crash_sweep")` in sweep.go | FOUND |
| `db.WithBypass(ctx, "import_crash_sweep.startup")` in sweep.go | FOUND |
| `reMarshalAs[T any]` in parser_json.go | FOUND |
| 9 TestImportSweep_ functions in sweep_test.go | FOUND |
| go vet clean | FOUND |
| go build clean | FOUND |
| go test -short clean (34 PASS + 9 SKIP) | FOUND |
| go test full clean against testcontainer (43 PASS) | FOUND |

## Self-Check: PASSED

---
*Phase: 05-bulk-import-go*
*Completed: 2026-05-17*
