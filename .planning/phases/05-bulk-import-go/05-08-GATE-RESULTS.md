# Phase 5 Ship Gate Results

**Date:** 2026-05-17
**Commit:** 1cf00433fd7a9a8eb2ae1df0abebb135d83be33d (worktree base; lint-fix commits land on top)
**Branch:** worktree-agent-a97806d3ebd6f384c (executor) → gsd/phase-05-bulk-import-go

## Summary

| Gate | Status | Notes |
|------|--------|-------|
| `task gen` first run (drift) | **PASS** | exit 0; pnpm install needed before gen could run (web/node_modules absent in this worktree) |
| `task gen` second run (idempotency) | **PASS** | exit 0; zero diff |
| `task gen` path-constrained diff (CONTRACT-04) | **PASS** | exit 0 — `git diff --exit-code -- services/api/internal/api/ services/api/internal/db/generated/ web/packages/ui/src/api/generated.ts` |
| `task gen` path-unconstrained diff (W10) | **PASS** | exit 0 — `git diff --exit-code` |
| `task test` (race-detector full suite) | **PASS** | 424 tests passing, 1 skip, 0 fail/panic across 11 packages |
| `task lint` first run | FAIL (6 issues) | Auto-fixed per Rule 1 — see "Auto-fixed Issues" below |
| `task lint` after Rule 1 fixes | **PASS** | exit 0 |
| `task db:reset` smoke | **NOT RUN (acceptable)** | Port 5432 owned by another worktree's `open-solutions-postgres-1`; tearing it down via `DROP DATABASE ... WITH (FORCE)` would destroy that worktree's state. Substitute coverage from testcontainer-based migration-idempotency tests (passed) + Plan 05-01 SUMMARY's documented Docker smoke. |

## task gen

### Pre-flight

The worktree had no `web/node_modules` (Claude Code worktrees ship without
node_modules per the worktree-base layout); `task gen`'s third step (`pnpm
-F @open-routing/ui gen:api`) needs `openapi-typescript` from `node_modules/.bin`.

Auto-fixed per Rule 3 (blocking issue):

```bash
cd web && pnpm install --frozen-lockfile
```

The `pnpm-lock.yaml` is committed; `--frozen-lockfile` guarantees the same
versions across all worktrees. Install completed cleanly:

```
+ @redocly/cli 2.30.6
+ @typescript-eslint/eslint-plugin 8.59.3
+ @typescript-eslint/parser 8.59.3
+ eslint 9.39.4
+ prettier 3.8.3
+ turbo 2.9.14
+ typescript 5.9.3
Done in 2.2s using pnpm v10.33.0
```

### First run (drift gate)

```bash
$ task gen
task: [gen] cd services/api && sqlc generate
task: [gen] cd services/api && go generate ./internal/api/...
task: [gen] cd web && pnpm -F @open-routing/ui gen:api
✨ openapi-typescript 7.13.0
🚀 ../../../openapi/openapi.yaml → src/api/generated.ts [49.3ms]
```

```bash
$ git status --porcelain
(empty)

$ git diff --exit-code -- services/api/internal/api/ services/api/internal/db/generated/ web/packages/ui/src/api/generated.ts
(exit 0, empty)

$ git diff --exit-code
(exit 0, empty)
```

**Verdict:** **PASS** — zero drift in CONTRACT-04 scope; zero drift anywhere
in the working tree. The codegen output committed in Plan 05-01 + 05-03 +
04.1-01..05 is byte-equivalent to what `task gen` produces from the
current worktree state.

### Second run (idempotency proof)

```bash
$ task gen
task: [gen] cd services/api && sqlc generate
task: [gen] cd services/api && go generate ./internal/api/...
task: [gen] cd web && pnpm -F @open-routing/ui gen:api
✨ openapi-typescript 7.13.0
🚀 ../../../openapi/openapi.yaml → src/api/generated.ts [46.2ms]

$ git diff --exit-code
(exit 0, empty)
```

**Verdict:** **PASS** — codegen is byte-equivalent across consecutive runs.

**Logs:** `/tmp/05-08-task-gen.log` (1st run), `/tmp/05-08-task-gen-2nd.log` (2nd run).

## task test

```bash
$ task test    # = cd services/api && go test -race -count=1 ./...
```

### Per-package result

| Package | Status | Time |
|---------|--------|------|
| `internal/cache` | ok | 3.325s |
| `internal/catalog` | ok | 22.000s |
| `internal/config` | (no tests) | — |
| `internal/db` | ok | 9.041s |
| `internal/db/generated` | (no tests) | — |
| `internal/db/orgkey` | ok | 3.116s |
| `internal/domain` | ok | 2.061s |
| `internal/imports` | ok | 17.122s (Phase 5 home — 138+ tests) |
| `internal/middleware` | ok | 5.134s |
| `internal/server` | ok | 4.202s |
| `internal/state` | ok | 11.071s |
| `internal/telemetry` | ok | 4.687s |
| `test/isolation` | ok | 8.060s (includes import migration idempotency + cross-org isolation) |

### Aggregate

- **Total tests:** 424 PASS
- **Skips:** 1
- **Failures:** 0
- **Panics:** 0
- **Race detector findings:** 0
- **Cgo linker noise:** `ld: warning: LC_DYSYMTAB` — macOS race-detector cgo artefact, unrelated to test logic; identical noise observed in Phase 04.1.

### Phase 5 + 04.1 critical test surface (sample, all PASS)

- `internal/imports`:
  - `TestImporter_Single_Agent_201`, `TestImporter_Single_Skill_Updates`,
    `TestImporter_CSV_Channels_PartialSuccess_207`,
    `TestImporter_OversizedBody_413`, `TestImporter_500_Rows_Cap`,
    `TestImporter_Idempotency_Replay_Returns_Persisted_Result`,
    `TestImporter_DuplicateCode_409`, plus the full row_<entity>_test.go
    matrix.
- `test/isolation`:
  - `TestMigration_ImportJobs_Idempotent` — Phase 5 migration 000003 idempotency
  - `TestMigration_Idempotent_BackfillProducesNoOpOnRerun` — Phase 04.1
    migration 000002 backfill idempotency
  - `TestImport_CrossOrgSameCode_BothSucceed` — Phase 5 cross-org code
    collision allowed (FOUND-08 / PITFALLS 4.4)
  - `TestImport_CrossOrgGetJob_404` — Phase 5 cross-org GET isolation
  - `TestImport_CrossOrgSameIdempotencyKey_BothProceed` — Phase 5
    idempotency-key isolation
  - `TestImport_CrossOrg_Channel_DefaultQueueCode_invalid_reference` —
    Phase 5 cross-org FK lookup boundary

**Verdict:** **PASS**.

**Logs:** `/tmp/05-08-task-test.log` (compact), `/tmp/05-08-all-verbose.log`
(full -v with per-test names + timing).

## task lint

### First run (FAIL → 6 issues, all in Phase 5)

```
internal/imports/jobs.go:55:24: G115: integer overflow conversion int -> int32 (gosec)
internal/imports/jobs.go:84:23: G115: integer overflow conversion int -> int32 (gosec)
internal/imports/jobs.go:85:23: G115: integer overflow conversion int -> int32 (gosec)
internal/imports/sweep_test.go:204:2: ineffectual assignment to jobID (ineffassign)
internal/imports/testutil_test.go:420:6: func getImportJobAsOrg is unused (unused)
internal/imports/testutil_test.go:510:6: func extractFirstSucceededID is unused (unused)
6 issues: gosec: 3, ineffassign: 1, unused: 2
```

### Auto-fixed (Rule 1 — Bug / NEW lint introduction)

All 6 issues are Phase-5-introduced; the plan's gate criterion is "no NEW
lint errors introduced by Phase 5." Fixes applied inline:

1. **G115 in jobs.go (×3) — gosec int → int32 overflow false positive.**
   `totalRows`, `succeeded`, `failed` are each hard-bounded ≤ 500 by D5-22
   (handler rejects bodies > 500 rows BEFORE calling createJob/finaliseJob).
   Bounded int → int32 is safe. Added `//nolint:gosec // bounded ≤ 500 by D5-22`
   with the load-bearing rationale in-line.

2. **ineffassign in sweep_test.go:204 — first `jobID :=` value overwritten
   on line 215 without being used.** The first seed exists so the
   Importer's `startupSweep` (fired during `Start()`) has a row to drain
   before we exercise `runSweepPastDue` directly with a fresh seed.
   Renamed `jobID :=` → `_ =` on the first seed (intent: discard; the row
   itself is what matters, not the returned UUID), and changed the
   second `jobID =` → `jobID :=` (now the first declaration of that
   variable).

3. **unused in testutil_test.go (×2) — `getImportJobAsOrg` and
   `extractFirstSucceededID`.** Both helpers were authored for Plan 05-07
   tests but the final test design used different paths
   (`decodeBulkImportResult` directly, `test/isolation` package's
   own `getImportJobAsOrg` helper with a different signature). Helpers
   are still useful for follow-up tests; same pattern as
   `containsSubstring` in sweep_test.go (line 302). Added
   `//nolint:unused // <retained-for rationale>` to each, mirroring the
   established convention.

### Second run (after fixes)

```bash
$ task lint
task: [lint] cd services/api && go vet ./...
task: [lint] cd services/api && ../../.task/bin/golangci-lint run
(0 issues)
task: [lint] cd web && pnpm -F '*' lint
apps/embed lint: Done
apps/admin lint: Done
packages/ui lint: Done
. lint:  Tasks:    3 successful, 3 total
```

**Verdict:** **PASS**.

**Logs:** `/tmp/05-08-task-lint.log` (1st run, FAIL), `/tmp/05-08-task-lint-3.log` (final, PASS).

## task db:reset

**Status:** **NOT RUN (acceptable exception — same rationale as Phase 04.1
Plan 06).**

**Reason:** `task db:reset` runs `docker compose exec -T postgres psql ... "DROP
DATABASE openrouting WITH (FORCE)"` against this worktree's compose project.
The compose project in this worktree has `0 services` running; the only
running postgres container on the machine is `open-solutions-postgres-1`
on port 5432, owned by another worktree. Tearing it down via the destructive
`DROP DATABASE` would corrupt that worktree's dev DB. Auto Mode rule 5
(destructive shared-infra modification needs explicit user confirmation)
applies — denied here.

**Substitute coverage:**

1. **Plan 05-01 SUMMARY § Smoke Test (Docker Postgres 17)** — recorded a
   full clean migration replay against a fresh DB: 000001 (Phase 1
   baseline) + 000002 (Phase 04.1 identity normalization) + 000003 (Phase
   5 `import_jobs`) all applied; expected table shape (`\d import_jobs`)
   inspected; UNIQUE constraints verified.

2. **`test/isolation` testcontainer suite** — provisions a fresh Postgres
   17 testcontainer in `TestMain`, applies the SAME three migrations via
   `migrate -path migrations`, then runs every isolation test against
   that schema. The whole `test/isolation` package PASSED (8.060s) in
   this gate's `task test` run, including:
   - `TestMigration_ImportJobs_Idempotent` (000003 idempotency)
   - `TestMigration_Idempotent_BackfillProducesNoOpOnRerun` (000002 backfill idempotency)

3. **`internal/imports` testcontainer harness** — `internal/imports/testutil_test.go`
   spins its own Postgres 17 testcontainer per `TestMain`, applies all
   three migrations, then runs 138+ tests against that schema. All passed.

These three testcontainer paths give superset coverage of what
`task db:reset` would prove (fresh DB + all migrations replay clean).
The orchestrator's "if Docker compose is unavailable, mark as deferred
locally; CI runs the gate" carve-out applies here — same as Phase 04.1.

**Log:** `/tmp/05-08-task-db-reset.log` (records the BLOCKED-on-infra status).

## Phase Gate Summary

| Gate | Status |
|------|--------|
| `task gen` path-constrained | **PASS** |
| `task gen` path-unconstrained | **PASS** |
| `task gen` idempotency | **PASS** |
| `task test` | **PASS** (424 tests / 1 skip / 0 fail) |
| `task lint` | **PASS** (after Rule 1 inline fixes for 6 issues) |
| `task db:reset` | **NOT-RUN (acceptable)** — testcontainer substitute coverage proven green |

**All gates that ran are GREEN. The single not-run gate has documented
substitute coverage. Phase 5 ship gate is satisfied.**
