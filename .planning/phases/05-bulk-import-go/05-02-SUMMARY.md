---
phase: 05-bulk-import-go
plan: 02
subsystem: infra
tags: [middleware, body-limit, savepoint, pgx, http, exports, refactor]

requires:
  - phase: 04.1-catalog-identity-normalization
    provides: validateCodeFormat (D04_1-19) + mapPgError (D04_1-21) helpers in services/api/internal/catalog/
  - phase: 01-foundation-polyglot-monorepo
    provides: middleware/{requestid.go,httputil.go,uuidv7path.go} + db/orgdb.go (OrgTx + SQLChecker) + testsupport/{postgres,migrate}.go
provides:
  - catalog.ValidateCodeFormat (exported regex helper for D04_1-03 reuse by internal/imports/)
  - catalog.MapPgError (exported pgconn error → response triple for 23505/23503/23514 introspection by internal/imports/)
  - (*OrgTx).BeginSavepoint(ctx) chunk-scoped savepoint helper for D5-09 chunked-savepoint pattern
  - middleware.BodyLimit(maxBytes, pathPrefix) path-scoped Content-Length pre-flight + http.MaxBytesReader wrap (D5-21)
affects:
  - 05-04 (Wave 2 — coerce/parser pipeline reads through wrapped r.Body so 50 MB cap survives JSON eager-decode)
  - 05-05 (Wave 3 — chunk loop calls outerTx.BeginSavepoint per row inside a 50-row chunk)
  - 05-06 (Wave 4 — wires BodyLimit into server.NewMux's chi middleware chain BEHIND RequestID + OrgContext per Open Q7)
  - 05-07 (Wave 5 — entity row processors call catalog.ValidateCodeFormat + catalog.MapPgError directly)

tech-stack:
  added: []  # zero new external Go modules — net/http + strings stdlib only.
  patterns:
    - "Path-scoped chi MiddlewareFunc factory (BodyLimit) — extends the requestid.go / uuidv7path.go shape."
    - "r.ContentLength (stdlib-parsed long) over raw header string — aligns with Go stdlib idiom + avoids httptest.NewRequest header-projection gotcha."
    - "Per-test testcontainer (testsupport.StartPostgres) for db-package integration tests — avoids introducing TestMain into a package that previously had only unit tests."
    - "SAVEPOINT carries the SAME *SQLChecker pointer into the child *OrgTx — preflight semantics are preserved across the tx → savepoint hop (D-02 invariant survives)."

key-files:
  created:
    - "services/api/internal/db/orgdb_savepoint_test.go — 4 testcontainer integration tests for BeginSavepoint semantics + SQLChecker pointer preservation + tx-closed error wrapping."
    - "services/api/internal/middleware/bodylimit.go — D5-21 path-scoped middleware factory (BodyLimit)."
    - "services/api/internal/middleware/bodylimit_test.go — 7 unit tests (plan required ≥5) covering pre-flight, lying Content-Length, path-mismatch passthrough, boundary cases (+1/exact), chunked-transfer wrap, zero-byte body."
  modified:
    - "services/api/internal/catalog/codecheck.go — validateCodeFormat → ValidateCodeFormat (exported)."
    - "services/api/internal/catalog/errors.go — mapPgError → MapPgError (exported)."
    - "services/api/internal/db/orgdb.go — append (*OrgTx).BeginSavepoint(ctx)."
    - "services/api/internal/catalog/{adapters,agents,agent_skills,break_reasons,channels,queues,skills}.go — call-site rename + comment updates."
    - "services/api/internal/catalog/{codecheck,agents,agent_skills,break_reasons,channels,queues,skills}_test.go — comment + call-site rename to exported helpers."

key-decisions:
  - "Test database for BeginSavepoint integration tests targets the skills table (smallest 000002 catalog table) rather than _scaffold (dropped by 000002) or the unborn import_jobs table (Plan 05-01 deliverable)."
  - "Middleware uses r.ContentLength (parsed long) rather than r.Header.Get(\"Content-Length\") (raw string) — robust against httptest.NewRequest's header-not-projected behaviour and aligns with net/http stdlib idiom."
  - "Per-test testcontainer (testsupport.StartPostgres) rather than a TestMain shared pool — the db package previously had only unit tests; adding TestMain for 4 plumbing-validation tests would over-engineer for ~minimal speed savings."

patterns-established:
  - "Path-scoped chi MiddlewareFunc factory: function returning a function returning http.Handler, with a pathPrefix gate that lets non-matching requests fall through untouched. Reusable by future body-size or rate-limit middleware."
  - "Exported helpers in catalog package signal cross-package consumption: comment notes 'Exported in Phase 5 Wave 0 so internal/imports/ can reuse...' makes the intent grep-able for future maintainers."
  - "SAVEPOINT preserving SQLChecker pointer: when BeginSavepoint copies t.checker + t.mode into the child *OrgTx, the org_id filter validator survives unchanged across the chain. Pattern extends naturally to nested savepoints (Phase 5 v0.1 doesn't need them but the code is symmetric)."

requirements-completed:
  - IMP-07

# Metrics
duration: 1h 0m
completed: 2026-05-17
---

# Phase 5 Plan 02: Wave 0 Cross-Cutting Plumbing Summary

**Catalog helpers exported + (*OrgTx).BeginSavepoint added + path-scoped BodyLimit middleware with 7 unit tests — Phase 5 Wave 1+ unblocked**

## Performance

- **Duration:** ~1h 0m (commit timestamps `efc98a5` → `140e8a4`)
- **Started:** 2026-05-17T~10:55Z (Task 1 commit)
- **Completed:** 2026-05-17T11:55Z
- **Tasks:** 3
- **Files modified:** 17 (14 catalog, 1 db, 2 middleware) + 3 new files

## Accomplishments

- **Task 1 — Catalog helper exports.** Renamed `catalog.validateCodeFormat` → `ValidateCodeFormat` and `catalog.mapPgError` → `MapPgError` across `codecheck.go`, `errors.go`, the 7 handler `.go` files, and their `_test.go` counterparts. Mechanical lexical rename — zero behaviour change. The package-private regex variable `codeFormat` and the private helper `validateImmutableCode` deliberately stay lowercase (neither needs cross-package consumption). Phase 5's `internal/imports/` can now reuse the D04_1-03 regex + the D04_1-21 constraint-name introspection without duplication.
- **Task 2 — `(*OrgTx).BeginSavepoint(ctx)`.** Added the chunk-scoped savepoint helper that pgx v5 implements internally via the SAVEPOINT/RELEASE/ROLLBACK TO SAVEPOINT sequence on a non-top-level Tx. The returned child `*OrgTx` carries the SAME `*SQLChecker` pointer + `ValidationMode` as the parent, so the org_id-filter preflight survives the parent → child hop intact (D5-09 contract; verified by integration test).
- **Task 3 — Path-scoped `BodyLimit` middleware (D5-21).** New `internal/middleware/bodylimit.go` with the factory `BodyLimit(maxBytes int64, pathPrefix string) func(http.Handler) http.Handler`. Pre-flights `r.ContentLength` against `maxBytes` (short-circuiting 413 with the locked `{error: invalid_body, reason: request_too_large_use_async_pathway, request_id: <uuid>}` envelope), then wraps `r.Body` in `http.MaxBytesReader` so chunked-transfer / lying-Content-Length still trip a typed `*http.MaxBytesError` for downstream handler `errors.As` → 413 mapping. 7 unit tests cover every documented branch.

## Task Commits

Each task was committed atomically:

1. **Task 1: Export catalog helpers** — `efc98a5` (refactor) — 16 files changed, +64/-50
2. **Task 2: BeginSavepoint + integration tests** — `9b9f45f` (feat) — 2 files changed, +255/-0
3. **Task 3: BodyLimit middleware + unit tests** — `140e8a4` (feat) — 2 files changed, +375/-0

## Files Created/Modified

### Created (3)

- `services/api/internal/db/orgdb_savepoint_test.go` — 4 integration tests for `(*OrgTx).BeginSavepoint` (rollback isolates per-row failure, release lands savepoint write, preserves SQLChecker preflight, on-closed-tx returns wrapped error). Uses `testsupport.StartPostgres` per-test container; `-short` skips cleanly.
- `services/api/internal/middleware/bodylimit.go` — `BodyLimit(maxBytes, pathPrefix)` chi MiddlewareFunc factory.
- `services/api/internal/middleware/bodylimit_test.go` — 7 unit tests covering pre-flight + wrap branches + boundary cases.

### Modified — catalog package rename (16 files, all `s/lowercase/Capitalized/`)

- `services/api/internal/catalog/codecheck.go` — function rename + doc comment update (`Exported in Phase 5 Wave 0 so internal/imports/ can reuse...`).
- `services/api/internal/catalog/codecheck_test.go` — 2 call-site renames + 1 doc-line update.
- `services/api/internal/catalog/errors.go` — function rename + doc comment update.
- `services/api/internal/catalog/adapters.go` — 2× `validateCodeFormat` + 2× `mapPgError` + 1× comment.
- `services/api/internal/catalog/agents.go` — 2× `validateCodeFormat` + 2× `mapPgError`.
- `services/api/internal/catalog/agent_skills.go` — 1× `mapPgError` + 2× comment.
- `services/api/internal/catalog/break_reasons.go` — 2× `validateCodeFormat` + 2× `mapPgError` + 1× comment.
- `services/api/internal/catalog/channels.go` — 2× `validateCodeFormat` + 2× `mapPgError`.
- `services/api/internal/catalog/queues.go` — 2× `validateCodeFormat` + 2× `mapPgError`.
- `services/api/internal/catalog/skills.go` — 2× `validateCodeFormat` + 2× `mapPgError` + 1× comment.
- `services/api/internal/catalog/agents_test.go` — 2× comment update.
- `services/api/internal/catalog/agent_skills_test.go` — 1× comment update.
- `services/api/internal/catalog/break_reasons_test.go` — 1× comment update.
- `services/api/internal/catalog/channels_test.go` — 2× comment update.
- `services/api/internal/catalog/queues_test.go` — 2× comment update.
- `services/api/internal/catalog/skills_test.go` — 2× comment update.

### Modified — orgdb.go (1 file)

- `services/api/internal/db/orgdb.go` — appended `(*OrgTx).BeginSavepoint(ctx) (*OrgTx, error)` after the existing `BeginTx`. Implementation: `child, err := t.tx.Begin(ctx); return &OrgTx{tx: child, checker: t.checker, mode: t.mode}, err` (with `orgtx: begin savepoint:` error-wrap prefix).

## Diff Summary — BeginSavepoint method body

```go
// BeginSavepoint starts a savepoint on the current Tx and returns a new
// *OrgTx wrapping the child. SQLChecker preflight is preserved on every
// Exec/Query/QueryRow against the returned child Tx. The pgx Tx.Begin
// method internally implements SAVEPOINT semantics on a non-top-level
// Tx (verified — pkg.go.dev/github.com/jackc/pgx/v5).
//
// Phase 5 chunk loop (D5-09): outerTx.BeginSavepoint(ctx) per row inside
// a chunk of 50, then sp.Commit (RELEASE) on success or sp.Rollback
// (ROLLBACK TO) on per-row failure. The child carries the SAME checker
// pointer + ValidationMode — preflight semantics are identical to the
// parent Tx.
//
// pgx auto-generates SAVEPOINT names internally; callers do NOT issue raw
// `SAVEPOINT row_N` strings. Calling BeginSavepoint on a Tx that has
// already been committed/rolled-back returns a `tx is closed` error from
// pgx (RESEARCH Pitfall 8).
func (t *OrgTx) BeginSavepoint(ctx context.Context) (*OrgTx, error) {
    child, err := t.tx.Begin(ctx)
    if err != nil {
        return nil, fmt.Errorf("orgtx: begin savepoint: %w", err)
    }
    return &OrgTx{tx: child, checker: t.checker, mode: t.mode}, nil
}
```

## Test Inventory — bodylimit_test.go (7 cases; plan required ≥5)

| # | Test | Branch covered | Assertion |
|---|------|----------------|-----------|
| 1 | `TestBodyLimit_ContentLengthOverMax_413_BeforeBodyRead` | pre-flight short-circuit | 413; downstream handler NEVER runs; wire shape `{error: invalid_body, reason: request_too_large_use_async_pathway}` |
| 2 | `TestBodyLimit_LyingContentLength_413_MidStream` | wrap-side `MaxBytesError` | downstream handler reads, gets `*http.MaxBytesError` via `errors.As`, emits 413 |
| 3 | `TestBodyLimit_PathMismatch_Passthrough` | path-scope invariant | `/healthz` with 1000-byte body passes through uncapped |
| 4 | `TestBodyLimit_AtExactlyMaxBytes_Passes` | boundary case (cap == body) | 200; downstream reads exactly `maxBytes` bytes |
| 5 | `TestBodyLimit_OverMaxBytesByOneByte_413` | boundary case (cap + 1) | 413; locks `>` vs `>=` comparison |
| 6 | `TestBodyLimit_NoContentLength_StillWraps` | chunked-transfer scenario | pre-flight silently passes (no `Content-Length`), wrap catches over-limit body, downstream emits 413 |
| 7 | `TestBodyLimit_ZeroByteBody_Passes` | trivial bottom edge | 200; empty body passes |

## Test Inventory — orgdb_savepoint_test.go (4 integration tests)

| # | Test | Scenario | Assertion |
|---|------|----------|-----------|
| 1 | `TestOrgTx_BeginSavepoint_RollbackIsolatesPerRowFailure` | outer INSERT + savepoint INSERT + savepoint Rollback + outer Commit | Verify Tx sees exactly 1 row (outer survives, savepoint INSERT rolled back) |
| 2 | `TestOrgTx_BeginSavepoint_ReleaseLandsSavepointWrite` | outer INSERT + savepoint INSERT + savepoint Commit (RELEASE) + outer Commit | Verify Tx sees both rows |
| 3 | `TestOrgTx_BeginSavepoint_PreservesSQLCheckerPreflight` | Child Tx executes unscoped UPDATE (no `org_id`) | Returns `ErrSQLMissingOrgFilter` — proves SQLChecker pointer carried into child |
| 4 | `TestOrgTx_BeginSavepoint_OnClosedTx_ReturnsError` | Roll back outer Tx, then call BeginSavepoint on it | Returns wrapped `orgtx: begin savepoint: tx is closed` error |

All 4 tests skip cleanly under `go test -short`. Against docker all 4 pass (~3-4 second total run including container bring-up amortised per test).

## Verification

### Plan acceptance gates (paste of grep results)

```text
=== Gate 1: ValidateCodeFormat exported ===
1 (services/api/internal/catalog/codecheck.go:33: func ValidateCodeFormat)

=== Gate 2: MapPgError exported ===
1 (services/api/internal/catalog/errors.go:52: func MapPgError)

=== Gate 3: no lowercase validateCodeFormat remains ===
OK: no occurrences

=== Gate 4: no lowercase mapPgError remains ===
OK: no occurrences

=== Gate 5: BeginSavepoint method added ===
1 (services/api/internal/db/orgdb.go:234: func (t *OrgTx) BeginSavepoint)

=== Gate 6: BodyLimit middleware added ===
1 (services/api/internal/middleware/bodylimit.go:65: func BodyLimit)

=== Gate 7: ≥5 TestBodyLimit_ cases ===
7
```

### Build + test commands

```text
$ cd services/api && go build ./...
Go build: Success

$ cd services/api && go vet ./...
Go vet: No issues found

$ cd services/api && go test -count=1 -short ./internal/catalog/... ./internal/db/... ./internal/middleware/...
Go test: 112 passed in 5 packages

$ cd services/api && go test -count=1 -timeout 180s -run 'TestOrgTx_BeginSavepoint' ./internal/db/...
Go test: 4 passed in 3 packages (savepoint integration tests against docker)
```

## Decisions Made

1. **Skills table as savepoint-test sentinel.** Used the `skills` catalog table (smallest 000002 table that survived Phase 04.1's `_scaffold` drop) rather than `_scaffold` (dropped) or `import_jobs` (Plan 05-01 deliverable, not yet landed because Plan 05-01 runs in parallel with this plan in Wave 0). Trade-off: tests carry a tiny knowledge of skills schema (5 NOT-NULL columns) but compile without depending on any other plan in Wave 0.
2. **`r.ContentLength` over `r.Header.Get("Content-Length")`.** Initial draft used the header string parsed via `strconv.ParseInt`. Switched to `r.ContentLength` (Go's parsed long) after observing that `httptest.NewRequest(..., bytes.NewReader(...))` does NOT project Content-Length back to the header even when `req.ContentLength` is set. `r.ContentLength` is also the canonical net/http idiom — the runtime populates it from the header at request-parse time, and reading the parsed long avoids one more string→int conversion at every request. Doc comment notes the choice.
3. **Per-test testcontainer for orgdb savepoint suite.** The `db` package previously had only unit tests (nil pool + ValidationPanic/Error suites). Adding a TestMain shared-pool pattern just for 4 plumbing-validation tests would be over-engineering. The `testsupport.StartPostgres(t)` per-test pattern is the canonical alternative and is already used by `services/api/test/isolation/*_test.go` for one-shot scenarios.

## Deviations from Plan

None — plan executed exactly as written.

The plan task descriptions were sufficiently detailed (verbatim code excerpts for both the BeginSavepoint method body and the BodyLimit factory, with explicit hazard call-outs) that no additional code beyond what was specified was needed. The only minor judgement calls (Decisions 1-3 above) are sentinel-table choice + header-vs-parsed-long source + test-infra granularity, all of which were explicitly listed as "planner picks" or fell within the discretion granted by the plan's "If existing test file pattern requires shared setup ... follow it. Otherwise, the test can use a one-shot container; this is plumbing-validation, not behaviour-critical." note.

## Issues Encountered

1. **Initial savepoint test used `_scaffold` table — table no longer exists.** First test draft INSERTed into `_scaffold`, which was dropped by migration 000002 during Phase 04.1. Discovered when the savepoint tests failed against docker with `relation "_scaffold" does not exist (SQLSTATE 42P01)`. Resolved by rewriting INSERTs against the `skills` table (smallest catalog table with `org_id`) — included in the same commit (`9b9f45f`), no separate fix commit needed.
2. **Initial bodylimit test failed because `httptest.NewRequest` doesn't project Content-Length to the header.** Test set `req.ContentLength = 100` but the middleware read `r.Header.Get("Content-Length")` which returned `""`, so the pre-flight branch never fired and pre-flight tests returned 200 instead of 413. Resolved by switching the middleware to read `r.ContentLength` (parsed long) directly, which is both more robust and more idiomatic — see Decision 2.

Both issues were caught and resolved before commit. Neither required a separate fix commit.

## User Setup Required

None — no external service configuration required. All work is internal-only Go code changes within `services/api/`.

## Next Phase Readiness

**Wave 0 deliverables for Phase 5 are complete from this plan's side.** The exported `catalog.ValidateCodeFormat` + `catalog.MapPgError` symbols are ready for `internal/imports/` to consume in Wave 1/3. `(*OrgTx).BeginSavepoint(ctx)` is ready for the chunk-loop orchestrator in Wave 3. `middleware.BodyLimit` is ready for wiring into `server.NewMux` in Wave 4 (per Plan 05-06).

**Note for Plan 05-06 (Wave 4):** when wiring `BodyLimit` into the chi chain, ensure the order is `RequestID → OrgContext → BodyLimit("/v1/orgs/", 50<<20)` so cheap header rejection (400 invalid `X-Org-Id`) fires BEFORE any body bytes are wrapped (Open Q7, encoded in the bodylimit.go doc comment).

**No blockers for downstream Phase 5 waves.**

## Self-Check: PASSED

Verified each claim before declaring done:

1. **Files exist on disk:**
   - `services/api/internal/catalog/codecheck.go` — FOUND (ValidateCodeFormat exported)
   - `services/api/internal/catalog/errors.go` — FOUND (MapPgError exported)
   - `services/api/internal/db/orgdb.go` — FOUND (BeginSavepoint method added)
   - `services/api/internal/db/orgdb_savepoint_test.go` — FOUND (new test file)
   - `services/api/internal/middleware/bodylimit.go` — FOUND (new middleware)
   - `services/api/internal/middleware/bodylimit_test.go` — FOUND (new test file)

2. **Commits exist in git log:**
   - `efc98a5` — FOUND (Task 1: catalog helper exports)
   - `9b9f45f` — FOUND (Task 2: BeginSavepoint + integration tests)
   - `140e8a4` — FOUND (Task 3: BodyLimit middleware + unit tests)

3. **All 7 plan acceptance gates pass.** (See verification section above.)

4. **All tests green:**
   - `go test -count=1 -short ./internal/catalog/... ./internal/db/... ./internal/middleware/...` → 112 passed
   - `go test -count=1 -timeout 180s -run 'TestOrgTx_BeginSavepoint' ./internal/db/...` → 4 passed (docker available)

---

*Phase: 05-bulk-import-go*
*Plan: 02*
*Completed: 2026-05-17*
