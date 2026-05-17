---
phase: 05-bulk-import-go
plan: 03
subsystem: api
tags:
  - sqlc
  - codegen
  - import-jobs
  - agent-skills
  - agent-states
  - skills

# Dependency graph
requires:
  - phase: 05-bulk-import-go-plan-01
    provides: "migration 000003 import_jobs schema + sqlcheck tenantTables[\"import_jobs\"] entry + types.gen.go Import*Request schemas"
  - phase: 05-bulk-import-go-plan-02
    provides: "(*OrgTx).BeginSavepoint, catalog.ValidateCodeFormat/MapPgError exports, BodyLimit middleware — Plan 05-03 itself does not consume these but downstream plans do"
  - phase: 04.1-catalog-identity-normalization-plan-03
    provides: "UpsertXByCode + GetXByCode queries for all 6 catalog entities — Plan 05-03 invocations are appended by Plan 05-04+ (handler), not authored here"
  - phase: 04-agent-state-machine-go
    provides: "InsertAgentState :one (unconditional) — Plan 05-03 must NOT modify; authors InsertAgentStateOnConflictNothing as a separate query"
provides:
  - "import_jobs.sql with 5 queries (InsertImportJob, FinaliseImportJob, GetImportJob, LookupImportJobByIdempotencyKey, SweepCrashedImportJobs) — covers IMP-06 job lifecycle + D5-11 crash-recovery sweep + D5-13 idempotency replay"
  - "agent_skills.sql: MergeAgentSkill :exec — D5-18 PATCH-like merge + D5-19 import-value-wins for Phase 5 row processor; companion to existing InsertAgentSkill (untouched)"
  - "agent_states.sql: InsertAgentStateOnConflictNothing :exec — Phase 4 Hazard 7 carry-forward; seeds state for NEW agents on import while preserving the state machine on re-import"
  - "skills.sql: ResolveSkillCodes :many — batched (id, code) lookup for D5-16/17 chunk-level skill resolution (collapses N+1 to 1 round trip per 50-row chunk)"
  - "Generated *Queries methods: InsertImportJob, FinaliseImportJob, GetImportJob, LookupImportJobByIdempotencyKey, SweepCrashedImportJobs, MergeAgentSkill, InsertAgentStateOnConflictNothing, ResolveSkillCodes — 8 new typed methods on *Queries"
affects:
  - "Plan 05-04+ (Wave 2 — imports package scaffold) consumes InsertImportJob/FinaliseImportJob/GetImportJob/LookupImportJobByIdempotencyKey to wire the BulkImportCatalog + GetImportJob handlers"
  - "Plan 05-05+ (Wave 3 — per-entity row processors) consumes MergeAgentSkill (D5-18) + ResolveSkillCodes (D5-16/17) + InsertAgentStateOnConflictNothing (Phase 4 Hazard 7) for the agent row processor"
  - "Plan 05-04+ sweep.go consumes SweepCrashedImportJobs inside db.WithBypass(ctx, \"import_crash_sweep\") block (D5-11)"

# Tech tracking
tech-stack:
  added: []  # zero new external Go modules — sqlc v1.31.1 already pinned; queries use stdlib-equivalent pgx types
  patterns:
    - "Lone org-agnostic UPDATE under WithBypass: SweepCrashedImportJobs is the SOLE query in the entire codebase whose WHERE omits org_id; SQLChecker (D-02) rejects it in normal context, the goroutine call site uses db.WithBypass per Phase 4 STATE-07. Sets the precedent for any future cross-org maintenance sweep."
    - "Co-existing :one and :exec INSERTs against the same table: Phase 4's InsertAgentState :one (unconditional, used by CreateAgent) and Phase 5's InsertAgentStateOnConflictNothing :exec (idempotent, used by import) live side-by-side in agent_states.sql. The :one variant is left unchanged so Phase 4 callers are unaffected (T-05-03-04 mitigation). Pattern reusable when a new caller needs idempotent semantics for an existing INSERT without disturbing the original consumers."
    - "Batched lookup via ANY($N::text[]): ResolveSkillCodes mirrors the SkillsPresentInOrg ANY($N::uuid[]) shape but keyed by `code` (Phase 04.1's universal identifier) — N+1 mitigation pattern is type-parameter-symmetric, the SQL just swaps the array element type."

key-files:
  created:
    - "services/api/internal/db/queries/import_jobs.sql (124 lines, 5 named queries)"
    - "services/api/internal/db/generated/import_jobs.sql.go (sqlc-generated, ~245 lines, 5 methods on *Queries + 5 Params + 1 Row type, 1 query const each)"
    - ".planning/phases/05-bulk-import-go/05-03-SUMMARY.md"
  modified:
    - "services/api/internal/db/queries/agent_skills.sql (+30 lines; MergeAgentSkill :exec appended after SkillsPresentInOrg)"
    - "services/api/internal/db/queries/agent_states.sql (+27 lines; InsertAgentStateOnConflictNothing :exec appended after ExpireWrapUp)"
    - "services/api/internal/db/queries/skills.sql (+26 lines; ResolveSkillCodes :many appended after UpsertSkillByCode)"
    - "services/api/internal/db/generated/agent_skills.sql.go (regen, +50 lines; MergeAgentSkill method + Params)"
    - "services/api/internal/db/generated/agent_states.sql.go (regen, +38 lines; InsertAgentStateOnConflictNothing method + Params)"
    - "services/api/internal/db/generated/skills.sql.go (regen, +57 lines; ResolveSkillCodes method + Params + Row)"

key-decisions:
  - "Author 8 new queries across 4 sql files, leveraging Phase 04.1's already-shipped UpsertXByCode/GetXByCode primitives — Plan 05-03 does NOT duplicate or modify those; downstream waves invoke them"
  - "SweepCrashedImportJobs body is the lone org-agnostic UPDATE in the codebase, RETURNING (id, org_id) so the sweep goroutine can structured-log per-org deltas; the goroutine wraps the call site with db.WithBypass(ctx, \"import_crash_sweep\") (wired in Plan 05-04, not here)"
  - "InsertAgentStateOnConflictNothing is a NEW query, NOT a modification of InsertAgentState :one (T-05-03-04 mitigation) — preserves Phase 4 CreateAgent atomic-rollback assertion"
  - "MergeAgentSkill is :exec (not :one) because the ON CONFLICT UPDATE makes the RETURNING value ambiguous between INSERT and UPDATE paths; the row processor does not need the row back"
  - "ResolveSkillCodes parameter ordering follows Phase 04.1 convention (org_id leads at $1, batch array at $2)"
  - "sqlc.yaml uses a directory glob `queries: \"internal/db/queries\"` (NOT a per-file list); the plan's Task 1 step 2 'append import_jobs.sql to queries list' is a no-op in this repo configuration — the new file is auto-discovered"

patterns-established:
  - "Idempotent INSERT companion to a non-idempotent INSERT: pair `Insert<Entity> :one` (unconditional, used by entity-create flows) with `Insert<Entity>OnConflictNothing :exec` (idempotent, used by import flows). Both go into the same .sql file; sqlc emits two distinct methods. Eliminates the temptation to add ON CONFLICT to the existing :one and accidentally change Phase 4 semantics."
  - "Two-phase Idempotency-Key replay: LookupImportJobByIdempotencyKey FIRST (replay path on hit), InsertImportJob with sqlc.narg('idempotency_key') second (new row on miss). The partial UNIQUE on (org_id, idempotency_key) WHERE idempotency_key IS NOT NULL serves as a defensive backstop for the unlikely TOCTOU race; per D5-13, the lookup-first path is the primary flow."

requirements-completed:
  - IMP-03
  - IMP-06

# Metrics
duration: 6 min
completed: 2026-05-17
---

# Phase 5 Plan 03: SQL Query Surface for Bulk Import Summary

**8 new sqlc queries authored across 4 files (import_jobs.sql NEW + agent_skills.sql / agent_states.sql / skills.sql each appended one query) + clean sqlc regeneration with two-run idempotency proof. Phase 5 row processors and handler glue (Plans 05-04 onwards) now have typed `*Queries` methods to invoke.**

## Performance

- **Duration:** ~6 min (commit timestamps `b8fad42` → `9bdb57a`)
- **Started:** 2026-05-17T12:00:35Z
- **Completed:** 2026-05-17T12:06:56Z
- **Tasks:** 4 (all autonomous)
- **Files modified:** 7 (1 SQL created, 3 SQL appended, 3 generated regen + 1 generated created)

## Accomplishments

- **Task 1 — import_jobs.sql (NEW).** Authored 5 lifecycle queries: InsertImportJob (status='pending' INSERT with sqlc.narg('idempotency_key') for nullable D5-13 partial UNIQUE), FinaliseImportJob (terminal counters + errors JSONB UPDATE without version check per D-66 carry-forward since counters are monotonic), GetImportJob (composite (id, org_id) preserving FOUND-08 isolation), LookupImportJobByIdempotencyKey (replay path before InsertImportJob per D5-13 two-phase pattern), SweepCrashedImportJobs (the LONE org-agnostic UPDATE in the codebase — flips stranded pending rows >24h to failed with synthetic server_crash error, RETURNING (id, org_id) for sweep telemetry). File header documents the lifecycle, the unique WithBypass exception, the idempotency flow, and FOUND-08 isolation reminders.
- **Task 2 — MergeAgentSkill + InsertAgentStateOnConflictNothing.** Appended `MergeAgentSkill :exec` to agent_skills.sql (ON CONFLICT (agent_id, skill_id) DO UPDATE SET proficiency = EXCLUDED.proficiency — D5-18 PATCH-like merge + D5-19 import-value-wins). Appended `InsertAgentStateOnConflictNothing :exec` to agent_states.sql (ON CONFLICT (agent_id) DO NOTHING — Phase 4 Hazard 7 carry-forward + T-05-03-04 mitigation by authoring as a SEPARATE query, leaving the original Phase 4 `InsertAgentState :one` unconditional and untouched). Both are `:exec` because their ON CONFLICT paths make RETURNING semantically ambiguous and the row processors don't need rows back.
- **Task 3 — ResolveSkillCodes.** Appended `ResolveSkillCodes :many` to skills.sql for D5-16 (JSON nested-skills) + D5-17 (CSV `code:prof|code:prof`) chunk-level batched lookup. Returns (id, code) so the handler can compute the unknown-skill set by Go-side difference (mirror agent_skills.SkillsPresentInOrg miss-detection pattern). Collapses 150 round trips (50 rows × 3 skills) into 1 per chunk — RESEARCH Pitfall 6 mitigation. SQLChecker (D-02) compliant: tenant-aliased FROM skills + literal `org_id = $1` in WHERE; parameter ordering matches Phase 04.1 (org_id leads at $1, code-array at $2).
- **Task 4 — Codegen regen.** `task gen` invokes sqlc + oapi-codegen + openapi-typescript. sqlc emitted `import_jobs.sql.go` (NEW) with 5 typed methods on `*Queries`, regen'd `agent_skills.sql.go` / `agent_states.sql.go` / `skills.sql.go` adding one method each. **Idempotency proof:** two consecutive `task gen` runs produce zero new diff after the first. `go vet ./...` clean; `go build ./...` clean; `go test -count=1 -short ./internal/db/...` = 58 passed in 3 packages.

## Task Commits

Each task committed atomically:

1. **Task 1: Author import_jobs.sql with 5 queries** — `b8fad42` (feat) — 1 file, +124 lines
2. **Task 2: Append MergeAgentSkill + InsertAgentStateOnConflictNothing** — `972e7ad` (feat) — 2 files, +58 lines
3. **Task 3: Append ResolveSkillCodes to skills.sql** — `e318ddb` (feat) — 1 file, +26 lines
4. **Task 4: Regenerate sqlc bindings** — `9bdb57a` (chore) — 4 files, +429 lines (1 new + 3 modified)

_No final metadata commit — orchestrator owns STATE.md / ROADMAP.md updates per `<parallel_execution>` directive._

## Files Created/Modified

### Created

- `services/api/internal/db/queries/import_jobs.sql` (124 lines, 5 named queries with heavy header + per-query doc comments).
- `services/api/internal/db/generated/import_jobs.sql.go` (sqlc-generated; 5 methods on `*Queries`):
  - `InsertImportJob(ctx, arg InsertImportJobParams) (ImportJob, error)` — Params: `{ID, OrgID pgtype.UUID; EntityType string; TotalRows int32; IdempotencyKey *string}` (sqlc.narg → nullable pointer).
  - `FinaliseImportJob(ctx, arg FinaliseImportJobParams) (ImportJob, error)` — Params: `{Status string; SucceededRows, FailedRows int32; Errors []byte; ID, OrgID pgtype.UUID}`.
  - `GetImportJob(ctx, arg GetImportJobParams) (ImportJob, error)` — Params: `{ID, OrgID pgtype.UUID}`.
  - `LookupImportJobByIdempotencyKey(ctx, arg LookupImportJobByIdempotencyKeyParams) (ImportJob, error)` — Params: `{OrgID pgtype.UUID; IdempotencyKey string}`.
  - `SweepCrashedImportJobs(ctx) ([]SweepCrashedImportJobsRow, error)` — Row: `{ID, OrgID pgtype.UUID}`.

### Modified

- `services/api/internal/db/queries/agent_skills.sql` (+30 lines) — `MergeAgentSkill :exec` appended after `SkillsPresentInOrg :many`. Heavy doc comment explaining the deliberate divergence from PUT-style replaceAgentSkills (Phase 3 catalog/agent_skills.go:74) and the import-value-wins rationale (D5-19).
- `services/api/internal/db/queries/agent_states.sql` (+27 lines) — `InsertAgentStateOnConflictNothing :exec` appended after `ExpireWrapUp :one`. Doc comment makes the "SEPARATE query, NOT modification of InsertAgentState :one" intent grep-able for future maintainers (T-05-03-04 mitigation).
- `services/api/internal/db/queries/skills.sql` (+26 lines) — `ResolveSkillCodes :many` appended after `UpsertSkillByCode :one`. Doc comment notes the SQLChecker compliance shape and the N+1 mitigation arithmetic.
- `services/api/internal/db/generated/agent_skills.sql.go` (+50 lines) — `MergeAgentSkill(ctx, arg MergeAgentSkillParams) error` method + Params `{AgentID, SkillID, OrgID pgtype.UUID; Proficiency int32}`.
- `services/api/internal/db/generated/agent_states.sql.go` (+38 lines) — `InsertAgentStateOnConflictNothing(ctx, arg InsertAgentStateOnConflictNothingParams) error` method + Params `{AgentID, OrgID pgtype.UUID; Status string}`. Phase 4's `InsertAgentState :one` method is BIT-IDENTICAL to its pre-Plan-05-03 version.
- `services/api/internal/db/generated/skills.sql.go` (+57 lines) — `ResolveSkillCodes(ctx, arg ResolveSkillCodesParams) ([]ResolveSkillCodesRow, error)` method + Params `{OrgID pgtype.UUID; Column2 []string}` + Row `{ID pgtype.UUID; Code string}`.

## ImportJob Model Field Types (from db/generated/models.go, unchanged since Plan 05-01)

| Field            | Go type              | Source column                  | Notes                                                      |
| ---------------- | -------------------- | ------------------------------ | ---------------------------------------------------------- |
| `ID`             | `pgtype.UUID`        | id UUID PRIMARY KEY            | UUIDv7 minted at handler entry per D-19                    |
| `OrgID`          | `pgtype.UUID`        | org_id UUID NOT NULL           | Denormalised per RESEARCH §Pattern 8 (SQLChecker visible)  |
| `EntityType`     | `string`             | entity_type TEXT NOT NULL      | CHECK IN (agents, skills, queues, channels, adapters, break_reasons) |
| `Status`         | `string`             | status TEXT NOT NULL           | CHECK IN (pending, completed, failed); DEFAULT 'pending'   |
| `TotalRows`      | `int32`              | total_rows INTEGER NOT NULL    | DEFAULT 0; set on INSERT to caller's row count             |
| `SucceededRows`  | `int32`              | succeeded_rows INTEGER NOT NULL| DEFAULT 0; monotonically grows during chunk loop           |
| `FailedRows`     | `int32`              | failed_rows INTEGER NOT NULL   | DEFAULT 0; monotonically grows during chunk loop           |
| `Errors`         | `[]byte`             | errors JSONB                   | sqlc maps JSONB to `[]byte`; shape mirrors BulkImportResult.failed[] per D5-12 |
| `IdempotencyKey` | `*string`            | idempotency_key TEXT NULL      | Pointer because nullable + sqlc.narg → emit_pointers_for_null_types: true |
| `CreatedAt`      | `pgtype.Timestamptz` | created_at TIMESTAMPTZ NOT NULL | DEFAULT NOW() at INSERT                                    |
| `UpdatedAt`      | `pgtype.Timestamptz` | updated_at TIMESTAMPTZ NOT NULL | NOW() bumped on finalise + sweep                           |

## Decisions Made

1. **`SweepCrashedImportJobs` body is the lone org-agnostic UPDATE.** Authored with WHERE clause referencing only `status` + `updated_at`, RETURNING `(id, org_id)` so the Plan 05-04 sweep goroutine can structured-log per-org sweep deltas. The orgDB SQLChecker (D-02) explicitly rejects queries lacking `org_id = $N` in WHERE — the goroutine wraps its single call site with `db.WithBypass(ctx, "import_crash_sweep")` (Phase 4 STATE-07 pattern). The unique-exception comment in the query header makes this grep-able (`grep -n 'UNIQUE SQLChecker EXCEPTION' import_jobs.sql`).
2. **`InsertAgentStateOnConflictNothing` is a SEPARATE query, NOT a modification of `InsertAgentState`.** Adding `ON CONFLICT (agent_id) DO NOTHING` to the existing Phase 4 query would silently change `CreateAgent` semantics — Phase 4's atomic-rollback assertion at `services/api/internal/catalog/agents.go` relies on `InsertAgentState` raising 23505 when called twice for the same agent (which can only happen if the outer transaction's agent INSERT raced). T-05-03-04 mitigation: leave Phase 4 untouched, author Phase 5's variant alongside.
3. **`MergeAgentSkill` is `:exec` not `:one`.** ON CONFLICT UPDATE makes RETURNING semantically ambiguous about whether the INSERT or UPDATE path landed. Phase 5's row processor consumes `error` only; it doesn't need the post-state row back. Mirror: agent_states.sql `ExpireWrapUp` which uses `:one` but only because it deliberately returns the post-transition state for the sweep goroutine's cache invalidation — MergeAgentSkill has no comparable side-effect that needs the row.
4. **`MergeAgentSkill` parameters match `InsertAgentSkill` ordering** (agent_id, skill_id, org_id, proficiency). Symmetry minimises caller confusion; the row processor can swap one for the other based on whether it's import (Merge) or PUT-replace (Insert + DeleteAgentSkills).
5. **`ResolveSkillCodes` returns `(id, code)` not just `id`.** The handler computes the unknown-set by Go-side difference of input codes vs returned codes — having the code in the result tuple lets the handler build the map without round-tripping the input slice. Compare with `SkillsPresentInOrg` which returns only `id` because the handler-side difference is `input_ids - present_ids` (no need for the code there).
6. **sqlc.yaml uses a directory glob, not a per-file list.** The plan's Task 1 step 2 said "append `queries/import_jobs.sql` to the `queries:` list" — but `services/api/sqlc.yaml` is configured with `queries: "internal/db/queries"` (a directory, not an array). New `.sql` files are auto-discovered, so no yaml edit is needed. The plan text reflects a different convention; this repo uses the simpler glob form.
7. **No final metadata commit per `<parallel_execution>` directive.** The orchestrator owns STATE.md / ROADMAP.md updates; this executor commits per-task feat/chore commits only.

## F2 Invariant Confirmation (BulkImportCatalogJSONBody)

After Task 4's full `task gen` regen, the F2 invariant from Plan 05-01 is preserved:

```text
$ grep -n 'BulkImportCatalogJSONBody' services/api/internal/api/types.gen.go
1447:// BulkImportCatalogJSONBody defines parameters for BulkImportCatalog.
1448:type BulkImportCatalogJSONBody = []interface{}
1538:type BulkImportCatalogJSONRequestBody = BulkImportCatalogJSONBody
```

The request body remains parametric `[]interface{}` (per F2 / RESEARCH); handler dispatches per-row by `?entity=` and re-marshals each element into the typed Import*Request struct in Plan 05-05+.

## Phase 4 InsertAgentState Unchanged (T-05-03-04 Mitigation)

```text
$ grep -A 4 '^-- name: InsertAgentState :one' services/api/internal/db/queries/agent_states.sql
-- name: InsertAgentState :one
-- D-93: called from catalog.CreateAgent inside the existing OrgTx so
-- agent + agent_state INSERTs commit atomically (Codex C4 pattern).
INSERT INTO agent_states (agent_id, org_id, status, state_version)
VALUES ($1, $2, $3, 1)

$ grep -c 'func (q \*Queries) InsertAgentState\b' services/api/internal/db/generated/agent_states.sql.go
1
```

The :one variant exists with its original signature, unconditional behaviour, and Phase 4 doc comment. Phase 5's :exec variant lives alongside as a separate method (T-05-03-04 mitigation).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Restored pnpm workspace `node_modules` before second `task gen` invocation**
- **Found during:** Task 4 (first `task gen` failed with `sh: openapi-typescript: command not found`).
- **Issue:** The worktree was freshly created and `web/node_modules` was not present. The `task gen` chain runs `cd web && pnpm -F @open-routing/ui gen:api` which requires `openapi-typescript` from the local `node_modules/.bin`. Without it, the typescript regen step fails and the overall `task gen` exits non-zero. This is the same issue Plan 05-01's executor encountered and documented.
- **Fix:** Ran `cd web && pnpm install --frozen-lockfile` from the worktree root — 380 packages restored from the existing lockfile, no new dependencies, no lockfile drift. Subsequent `task gen` runs succeed and are idempotent.
- **Files modified:** none — pnpm install does not modify the repo (node_modules is gitignored; pnpm-lock.yaml is unchanged).
- **Verification:** Two consecutive `task gen` runs produce identical output (idempotency proof).
- **Committed in:** N/A — this is environment setup, not a code change.

### Documented gap, not deviation

**2. sqlc.yaml convention vs plan instruction**
- **Plan Task 1 step 2:** "Append `queries/import_jobs.sql` to the `services/api/sqlc.yaml` `queries:` list."
- **Reality:** `services/api/sqlc.yaml` has `queries: "internal/db/queries"` (a directory glob, not a list). sqlc auto-discovers all `.sql` files in that directory.
- **Decision:** No yaml change needed — the new `import_jobs.sql` is auto-discovered. sqlc generate succeeds and emits `import_jobs.sql.go` from the first run.
- **Documented in:** Decision 6 above + this section. No action required of downstream agents.

---

**Total deviations:** 1 auto-fixed (Rule 3 — environment setup, not code). 1 documented convention gap (plan said "append to list", repo uses glob).
**Impact on plan:** Zero behavioural drift. Plan acceptance gates all met as written.

## Acceptance Gates (grep results pasted)

```text
=== import_jobs.sql contains 5 named queries (each appears exactly once) ===
InsertImportJob :one: 1
FinaliseImportJob :one: 1
GetImportJob :one: 1
LookupImportJobByIdempotencyKey :one: 1
SweepCrashedImportJobs :many: 1

=== agent_skills.sql gained MergeAgentSkill :exec ===
MergeAgentSkill :exec: 1
ON CONFLICT (agent_id, skill_id) DO UPDATE: 2 (1 in doc comment + 1 in SQL — the
  documented merge semantic is present in both prose and code; acceptance met)

=== agent_states.sql: InsertAgentStateOnConflictNothing :exec added, InsertAgentState :one preserved ===
InsertAgentStateOnConflictNothing :exec: 1
InsertAgentState :one: 1 (Phase 4 — UNCHANGED)
ON CONFLICT (agent_id) DO NOTHING: 1

=== skills.sql gained ResolveSkillCodes :many ===
ResolveSkillCodes :many: 1
code = ANY: 1

=== 8 new generated *Queries methods present ===
func (q *Queries) InsertImportJob: 1
func (q *Queries) FinaliseImportJob: 1
func (q *Queries) GetImportJob: 1
func (q *Queries) LookupImportJobByIdempotencyKey: 1
func (q *Queries) SweepCrashedImportJobs: 1
func (q *Queries) MergeAgentSkill: 1
func (q *Queries) InsertAgentStateOnConflictNothing: 1
func (q *Queries) ResolveSkillCodes: 1

=== ImportJob model present in db/generated/models.go (auto-emitted by Wave 0) ===
type ImportJob struct: 1

=== ResolveSkillCodesRow returns {id, code} as expected ===
type ResolveSkillCodesRow struct {
    ID   pgtype.UUID `json:"id"`
    Code string      `json:"code"`
}

=== F2 invariant retained: BulkImportCatalogJSONBody = []interface{} ===
1448:type BulkImportCatalogJSONBody = []interface{}

=== task gen idempotency (sha-stable across two consecutive runs) ===
Run 1: sqlc + oapi-codegen + openapi-typescript → 3 modified + 1 new generated files
Run 2: same chain → ZERO new diff (git status --short returns the same modified
       files as Run 1; no further mutation)
=> IDEMPOTENT_CODEGEN_PASS

=== Build + vet + test ===
cd services/api && go vet ./...                               → "No issues found"
cd services/api && go build ./...                             → "Success" (exit 0)
cd services/api && go test -count=1 -short ./internal/db/...  → 58 passed in 3 packages
```

## Self-Check: PASSED

All files claimed in this SUMMARY exist on disk:

- `services/api/internal/db/queries/import_jobs.sql` — FOUND (124 lines)
- `services/api/internal/db/queries/agent_skills.sql` — FOUND (modified, +30 lines)
- `services/api/internal/db/queries/agent_states.sql` — FOUND (modified, +27 lines)
- `services/api/internal/db/queries/skills.sql` — FOUND (modified, +26 lines)
- `services/api/internal/db/generated/import_jobs.sql.go` — FOUND (NEW, ~245 lines)
- `services/api/internal/db/generated/agent_skills.sql.go` — FOUND (regen, +50 lines)
- `services/api/internal/db/generated/agent_states.sql.go` — FOUND (regen, +38 lines)
- `services/api/internal/db/generated/skills.sql.go` — FOUND (regen, +57 lines)
- `.planning/phases/05-bulk-import-go/05-03-SUMMARY.md` — FOUND (this file)

All commits claimed in this SUMMARY exist in git log:

- `b8fad42` (Task 1: import_jobs.sql NEW) — FOUND
- `972e7ad` (Task 2: MergeAgentSkill + InsertAgentStateOnConflictNothing) — FOUND
- `e318ddb` (Task 3: ResolveSkillCodes) — FOUND
- `9bdb57a` (Task 4: sqlc regen) — FOUND

All success_criteria from the orchestrator prompt verified:

- [x] All 4 tasks executed and committed atomically
- [x] No modifications to .planning/STATE.md or .planning/ROADMAP.md (orchestrator owns those)
- [x] `services/api/internal/db/queries/import_jobs.sql` exists with 5 queries
- [x] `services/api/internal/db/queries/agent_skills.sql` appended with `MergeAgentSkill :exec`
- [x] `services/api/internal/db/queries/skills.sql` appended with `ResolveSkillCodes :many`
- [x] `services/api/internal/db/queries/agent_states.sql` appended with `InsertAgentStateOnConflictNothing :exec` (note: orchestrator prompt mentioned `agents.sql` for this query but the plan frontmatter + PATTERNS.md authoritatively place it in `agent_states.sql`; the table is `agent_states` not `agents` so this is the structurally correct location — followed plan authority)
- [x] `task gen` runs successfully; second run produces zero new diff (idempotency proof)
- [x] `services/api/internal/db/generated/import_jobs.sql.go` (NEW) contains all 5 import_jobs query methods
- [x] `services/api/internal/db/generated/agent_skills.sql.go` contains MergeAgentSkill
- [x] `services/api/internal/db/generated/skills.sql.go` contains ResolveSkillCodes (returns []ResolveSkillCodesRow with {id, code} fields)
- [x] `cd services/api && go build ./...` exits 0
- [x] `cd services/api && go vet ./...` exits 0
- [x] All grep gates from PLAN.md acceptance criteria pass (results pasted in "Acceptance Gates" section above)

## Notes for Plan 05-04+ (Downstream Waves)

- **Wave 2 (imports package scaffold):** instantiate `*generated.Queries` against an `*OrgDB` and invoke `q.InsertImportJob` / `q.FinaliseImportJob` / `q.GetImportJob` / `q.LookupImportJobByIdempotencyKey` in the handler.
- **Wave 3 (per-entity row processors):** call `q.MergeAgentSkill` in the agent row processor for D5-18 merge; call `q.ResolveSkillCodes` once per 50-row chunk for D5-16/17 batched lookup; call `q.InsertAgentStateOnConflictNothing` after `q.UpsertAgentByCode` for every new agent (Phase 4 Hazard 7).
- **Plan 05-04 sweep.go:** instantiate `*generated.Queries` against `*OrgDB`, wrap the sweep goroutine's per-tick context with `db.WithBypass(ctx, "import_crash_sweep")` per Phase 4 STATE-07, then call `q.SweepCrashedImportJobs(ctx)`. For each returned `(id, org_id)`, structured-log at `slog.Warn` with `event=import_crash_sweep`, `import_job_id=...`, `org_id=...`.
- **Threat surface scan:** none new beyond plan's `<threat_model>` (T-05-03-01..06). The surface introduced (5 import_jobs queries + 3 cross-entity queries) is enumerated in the plan; SweepCrashedImportJobs is the highest-risk addition and its WithBypass gating is the documented mitigation.
- **Open follow-ups for orchestrator:**
  - STATE.md / ROADMAP.md updates — owned by orchestrator per `<parallel_execution>` directive.
  - REQUIREMENTS.md will be marked complete for IMP-03 and IMP-06 by the orchestrator's `requirements mark-complete` call. **Note:** IMP-03 and IMP-06 are not fully complete until the handler ships in Plans 05-04/05-05; reasonable to defer the mark-complete to phase boundary.

---
*Phase: 05-bulk-import-go*
*Plan: 03*
*Completed: 2026-05-17*
