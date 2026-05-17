---
phase: 05-bulk-import-go
plan: 01
subsystem: api
tags:
  - openapi
  - codegen
  - migration
  - sqlcheck
  - oapi-codegen
  - sqlc
  - openapi-typescript

# Dependency graph
requires:
  - phase: 04.1-catalog-identity-normalization
    provides: "code is universal identifier (D04_1-01); UpsertXByCode + GetXByCode queries; validateCodeFormat helper; mapPgError 23505 constraint introspection"
  - phase: 02-openapi-contract-codegen
    provides: "BulkImportCatalog + GetImportJob strict-server stubs; oapi-codegen v2 strict-server flavor; CI codegen-drift gate"
  - phase: 04-agent-state-machine-go
    provides: "Phase 4 STATE-07 background-goroutine pattern (time.Ticker + WithBypass sweep) — reused by Phase 5 D5-11 crash-recovery sweep"
provides:
  - "openapi.yaml v0.1.5: 6 Import*Request schemas keyed on code; Idempotency-Key header parameter; ImportJob.status enum (pending|completed|failed); BulkImportResult.idempotent_replay; operation+tag descriptions rewritten to reference (org_id, code) per D04_1-01"
  - "migration 000003_create_import_jobs.{up,down}.sql with 3 indexes (org+created admin list, partial pending sweep, partial UNIQUE idempotency); no FK constraints (D-80); denormalised org_id for SQLChecker"
  - "sqlcheck.go tenantTables[\"import_jobs\"] allowlist entry (IMP-06; sweep goroutine uses WithBypass)"
  - "regenerated codegen: types.gen.go (6 ImportXRequest + ImportJobStatus + IdempotentReplay); server.gen.go (BulkImportCatalogParams.IdempotencyKey); spec.gen.go; db/generated/models.go (ImportJob struct from sqlc); web/packages/ui/src/api/generated.ts (TS shapes)"
  - "oapi-codegen.types.yaml: skip-prune=true so the 6 Import*Request schemas survive pruning (they are not $ref'd from any request body per F2 invariant; client typed access requires them)"
affects:
  - "Phase 5 Plan 02+ (sqlc queries for import_jobs + agent_skills MergeAgentSkill + skills ResolveSkillCodes — must reference the schemas + tenantTables entry landed here)"
  - "Phase 5 Plan 03+ (handler implementation — consumes ImportJobStatus enum, BulkImportCatalogParams.IdempotencyKey, ImportAgentRequest.skills nested shape)"
  - "Phase 6 (Admin UI bulk-import screen — consumes generated.ts ImportXRequest TS types + idempotent_replay flag)"

# Tech tracking
tech-stack:
  added: []  # zero new external dependencies — all tools (sqlc/oapi-codegen/openapi-typescript/golang-migrate) inherited from prior phases
  patterns:
    - "skip-prune toggle in oapi-codegen.types.yaml — preserve schemas that are defined for client typed access but NOT $ref'd from the request body (F2 invariant accommodation)"
    - "x-import-row-types extension on requestBody — list of related Import*Request schema names; documentation-only (doesn't keep schemas alive — that requires skip-prune)"
    - "partial UNIQUE index on (org_id, idempotency_key) WHERE NOT NULL — enforce uniqueness only when client supplied the header; NULL rows coexist freely"
    - "partial sweep index on (updated_at) WHERE status='pending' — sweep query scans only the work, not the whole table"
    - "denormalised org_id on Phase 5-owned tables — SQLChecker (D-02) sees the column on every query without FK probe; sweep goroutine uses db.WithBypass per STATE-07"

key-files:
  created:
    - "migrations/000003_create_import_jobs.up.sql"
    - "migrations/000003_create_import_jobs.down.sql"
    - ".planning/phases/05-bulk-import-go/05-01-SUMMARY.md"
  modified:
    - "openapi/openapi.yaml (10 deltas: 6 schemas, IdempotencyKeyHeader param, idempotent_replay, ImportJob.status, requestBody x-import-row-types extension, description rewrites, items: {} retained)"
    - "services/api/internal/db/sqlcheck.go (+1 tenantTables entry)"
    - "services/api/internal/api/oapi-codegen.types.yaml (skip-prune: false → true)"
    - "services/api/internal/api/types.gen.go (regen — 6 Import*Request + ImportJobStatus + IdempotentReplay)"
    - "services/api/internal/api/server.gen.go (regen — IdempotencyKey wired)"
    - "services/api/internal/api/spec.gen.go (regen — embedded yaml)"
    - "services/api/internal/db/generated/models.go (regen — sqlc emits ImportJob struct)"
    - "web/packages/ui/src/api/generated.ts (regen — TS shapes for new types)"

key-decisions:
  - "Keep items: {} on application/json request body — F2 invariant (oapi-codegen v2 cannot accessor-emit oneOf request bodies, issue #1620); handler dispatches per-row by ?entity= and re-marshals each interface{} into the typed struct"
  - "Flip skip-prune to true in oapi-codegen.types.yaml — the 6 Import*Request schemas are NOT $ref'd anywhere (because items: {} is parametric) and would be pruned otherwise"
  - "New migration 000003 (NOT amending 000002) per RESEARCH Q4 — import_jobs is Phase 5-owned, not catalog identity"
  - "x-import-row-types yaml extension on requestBody — documentation-only listing of the 6 related Import*Request schemas; supplements (does not replace) skip-prune"

patterns-established:
  - "Phase-additive migration sequence — Phase 5 starts the new-file (NOT amend) pattern; 000002 remains editable per D-61 until v0.1 ships, 000003+ are frozen forward-only"
  - "Idempotency-Key as reusable components.parameters entry — future write endpoints (D5-13 forward-compat) reference $ref: \"#/components/parameters/IdempotencyKeyHeader\""

requirements-completed:
  - IMP-01
  - IMP-03
  - IMP-04
  - IMP-05
  - IMP-06
  - IMP-07
  - IMP-08

# Metrics
duration: 9 min
completed: 2026-05-17
---

# Phase 5 Plan 01: OpenAPI deltas + migration 000003 + codegen regen Summary

**Phase 5 Wave 0 contract: 6 Import*Request schemas keyed on `code`, Idempotency-Key header, ImportJob.status enum, BulkImportResult.idempotent_replay, import_jobs migration 000003 with partial indexes, and regenerated Go+TS codegen — single coherent commit chain so every downstream wave inherits the contract.**

## Performance

- **Duration:** 9 min
- **Started:** 2026-05-17T11:34:04Z
- **Completed:** 2026-05-17T11:42:32Z
- **Tasks:** 3 (all autonomous)
- **Files modified:** 10 (3 created, 7 modified — including 5 regen outputs)

## Accomplishments

- OpenAPI spec carries 6 new `Import*Request` schemas (alphabetised: Adapter/Agent/BreakReason/Channel/Queue/Skill) with `code` regex `^[a-z][a-z0-9_]{0,63}$` per Phase 04.1 D04_1-03, optional `external_id` integration-mapping per D04_1-07, nested `skills[]` on `ImportAgentRequest` with MERGE semantics (D5-18).
- `Idempotency-Key` (RFC 9562 UUIDv7) header parameter added under `components.parameters.IdempotencyKeyHeader`, referenced from the `BulkImportCatalog` operation; oapi-codegen materialised `BulkImportCatalogParams.IdempotencyKey *IdempotencyKeyHeader` and bind logic in the wrapper.
- `ImportJob.status` enum (required) and `BulkImportResult.idempotent_replay` boolean (nullable, default false, readOnly) plumbed through Go types (`ImportJobStatus` constants Pending/Completed/Failed; `IdempotentReplay *bool`) and TypeScript shapes.
- Migration `000003_create_import_jobs.{up,down}.sql` creates `import_jobs` with denormalised `org_id`, status CHECK list, counter columns, errors JSONB, idempotency_key TEXT NULL, three indexes (admin-list cursor, partial pending-sweep, partial UNIQUE idempotency). sqlc auto-detected the new table and emitted `ImportJob` struct in `db/generated/models.go`.
- `sqlcheck.go` tenantTables map registers `"import_jobs"` with IMP-06 comment — every future sqlc query against the table will be preflighted for `org_id = $N`.
- `task gen` is idempotent: two consecutive runs produce sha256-identical outputs for all 5 generated files; CI codegen-drift gate (`git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/`) returns 0 after the final commit.
- F2 invariant preserved: `BulkImportCatalogJSONBody = []interface{}` (the request body remains `items: {}` per RESEARCH §F2 — oapi-codegen v2 cannot accessor-emit `oneOf` request bodies; handler dispatches per-row by `?entity=`).
- Zero new external dependencies — every tool (sqlc, oapi-codegen, openapi-typescript, golang-migrate) is inherited from prior phases. No package-legitimacy gate triggered.

## Task Commits

Each task was committed atomically:

1. **Task 1: Amend openapi.yaml with 10 deltas** — `d0a57ab` (feat)
2. **Task 2: Create migration 000003 + sqlcheck tenantTables entry** — `bed566d` (feat)
3. **Task 3: Run task gen + commit codegen drift (with Rule 3 fix to oapi-codegen.types.yaml)** — `277f3e5` (chore)

_No final metadata commit — orchestrator owns STATE.md / ROADMAP.md updates per `<parallel_execution>` directive._

## Files Created/Modified

### Created

- `migrations/000003_create_import_jobs.up.sql` (65 lines) — `import_jobs` table + 3 indexes. Schema: `id UUID PK`, `org_id UUID NOT NULL` (denormalised), `entity_type TEXT NOT NULL CHECK IN (agents|skills|queues|channels|adapters|break_reasons)`, `status TEXT NOT NULL DEFAULT 'pending' CHECK IN (pending|completed|failed)`, `total_rows INTEGER NOT NULL DEFAULT 0`, `succeeded_rows INTEGER NOT NULL DEFAULT 0`, `failed_rows INTEGER NOT NULL DEFAULT 0`, `errors JSONB`, `idempotency_key TEXT`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`. Indexes: `ix_import_jobs_org_created (org_id, created_at DESC, id DESC)`, `ix_import_jobs_pending_updated (updated_at) WHERE status='pending'` (partial — sweep), `uq_import_jobs_org_idempotency (org_id, idempotency_key) WHERE idempotency_key IS NOT NULL` (partial UNIQUE — idempotency).
- `migrations/000003_create_import_jobs.down.sql` (9 lines) — symmetric `DROP TABLE IF EXISTS import_jobs` inside `BEGIN…COMMIT`; v0.1 documentation-only per D-25 / D04_1-12.

### Modified

- `openapi/openapi.yaml` (+196 lines, –6 lines): 10 yaml deltas (counted by feature):
  - **DELTA 1** (line 3067-area): operation description rewritten — "Upsert semantics: keyed by `(org_id, code)`" + new "Idempotency" paragraph documenting D5-13 known limitation around `succeeded[]` reconstruction.
  - **DELTA 2** (line 3209-area): `Import` tag description rewritten — `(org_id, code)` not `(org_id, external_id)`.
  - **DELTA 3** (line ~3270-area): `requestBody.content.application/json.schema` keeps `type: array, items: {}, maxItems: 500` (F2 invariant) but description now references the 6 Import*Request schemas and the `?entity=` dispatch contract.
  - **DELTA 4** (line ~3254-area): operation-level `parameters:` block added with `$ref: "#/components/parameters/IdempotencyKeyHeader"`.
  - **DELTA 5** (line 1520-area): `BulkImportResult.idempotent_replay` property added (boolean, default false, nullable, readOnly).
  - **DELTA 6** (line 1556-area): `ImportJob.status` enum added to `required` list + property defined (enum: pending|completed|failed).
  - **DELTA 7** (line 1564-area): 6 new `Import*Request` schemas inserted alphabetically in `components.schemas` (Adapter, Agent, BreakReason, Channel, Queue, Skill). Each has `code` regex pattern, optional `external_id`, type-appropriate properties. `ImportAgentRequest` nested `skills: [{skill_code, proficiency 1-10}]`.
  - **DELTA 8** (line ~3257-area): `x-import-row-types` yaml extension on `requestBody` listing the 6 schema names — documentation-only (does not affect codegen pruning; skip-prune toggle in Task 3 handles that).
  - **DELTA 9** (line 1786-area): `IdempotencyKeyHeader` parameter added under `components.parameters` (alphabetical: between `EntityIdPath` and `ImportJobIdPath`).
  - **DELTA 10**: structural yaml sanity — `python -c "yaml.safe_load(...)"` validates 19 paths / 44 schemas / 10 parameters; no spec lint regressions.
- `services/api/internal/db/sqlcheck.go` (+1 line): tenantTables map adds `"import_jobs": {}, // IMP-06 (Phase 5; denormalized org_id per RESEARCH §Pattern 8 — sweeper bypasses ctx-org and relies on this column)`. Appended after the Phase 4 `agent_states` line (chronological CAT-NN / STATE-NN / IMP-NN convention matching existing file order, not strict alphabetical — same pattern Phase 4 used).
- `services/api/internal/api/oapi-codegen.types.yaml` (+9 lines, –1 line): `skip-prune: false → true` with header comment explaining the Phase-5 rationale. Without skip-prune, the 6 `Import*Request` schemas would be removed because nothing `$ref`s them (the request body is parametric `items: {}` per F2).
- `services/api/internal/api/types.gen.go` (+103 lines, –17 lines): regen. New types: `ImportAgentRequest`, `ImportSkillRequest`, `ImportQueueRequest`, `ImportChannelRequest`, `ImportAdapterRequest`, `ImportBreakReasonRequest`, `ImportJobStatus` (typed enum + constants Pending/Completed/Failed + `Valid()` method). `ImportJob.Status` field added. `BulkImportResult.IdempotentReplay *bool` pointer. `BulkImportCatalogParams.IdempotencyKey *IdempotencyKeyHeader` pointer. **F2 invariant retained**: `BulkImportCatalogJSONBody = []interface{}`.
- `services/api/internal/api/server.gen.go` (+21 lines): regen. Wrapper now binds the `Idempotency-Key` header into `params.IdempotencyKey` using `runtime.BindStyledParameterWithOptions("simple", …)`.
- `services/api/internal/api/spec.gen.go` (~530 lines diff): regen — embedded yaml bytes refresh.
- `services/api/internal/db/generated/models.go` (+14 lines): sqlc emits `type ImportJob struct { ID, OrgID, EntityType, Status, TotalRows, SucceededRows, FailedRows, Errors []byte, IdempotencyKey *string, CreatedAt, UpdatedAt }` (auto-detected from new migration).
- `web/packages/ui/src/api/generated.ts` (+131 lines, –0 lines): TS shapes for `ImportAgentRequest`, `ImportSkillRequest`, `ImportQueueRequest`, `ImportChannelRequest`, `ImportAdapterRequest`, `ImportBreakReasonRequest`; `IdempotencyKeyHeader` parameter referenced; `BulkImportResult.idempotent_replay` nullable boolean; `ImportJob.status` string enum.

## Decisions Made

1. **Keep `items: {}` on application/json request body** (F2 invariant per RESEARCH). Adding `oneOf: [6 schemas]` was tempting but oapi-codegen v2 does not emit `AsX/FromX` accessors for `oneOf` request bodies (verified — oapi-codegen issue #1620). The handler dispatches per-row by `?entity=` and re-marshals each `interface{}` element into the right typed struct in Plan 03+.
2. **Flip `skip-prune: false → true` in oapi-codegen.types.yaml** (Rule 3 auto-fix during Task 3). With the previous setting, oapi-codegen pruned the 6 Import*Request schemas because nothing `$ref`s them. The 6 schemas exist purely for client typed access (the server treats incoming rows as `interface{}`). `skip-prune=true` keeps every `components.schemas` entry. Inline yaml header comment documents the Phase-5 rationale so future maintainers don't flip it back.
3. **New migration 000003 (NOT amending 000002)** per RESEARCH Q4 / 04.1-PHASE5-AMEND-CHECKLIST. 000002 remains editable per D-61 only for catalog-identity concerns (Phase 04.1 owns it); `import_jobs` is Phase 5-owned. This also unblocks Phase 04.1's eventual `task db:reset` golden flow — 000003 is forward-only.
4. **Partial UNIQUE on `(org_id, idempotency_key) WHERE idempotency_key IS NOT NULL`** (D5-13). NULL idempotency_key rows coexist freely — only client-supplied keys are uniqueness-enforced. Postgres partial-index semantics make this the cleanest path; the alternative (composite UNIQUE on (org_id, idempotency_key) without the partial filter) would treat each `NULL` as distinct but still allocate a btree slot.
5. **Denormalised `org_id` on `import_jobs`** (no FK; D-80 invariant). SQLChecker (D-02) requires `org_id = $N` in every DML; the sweep goroutine uses `db.WithBypass(ctx, "import_crash_sweep")` per Phase 4 STATE-07 to mark the org-agnostic UPDATE.
6. **`Import*Request` schemas placed alphabetically in components.schemas** (Adapter < Agent < BreakReason < Channel < Queue < Skill). The existing `Bulk*` and `Import*` schemas are not strictly alphabetised either, but the new block is internally consistent — easier to skim than chronological.
7. **`Import*Request.skills` shape on JSON** (D5-16): nested array `[{skill_code, proficiency: integer 1–10}]`. CSV side (Plan 03+) uses the `code:prof|code:prof` cell convention per D5-17; the JSON shape is the primary contract.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Flipped `skip-prune: false → true` in oapi-codegen.types.yaml**
- **Found during:** Task 3 (first `task gen` run after spec edits)
- **Issue:** oapi-codegen pruned all 6 Import*Request schemas from `types.gen.go` because nothing `$ref`s them. The plan acceptance gate (`grep -c 'type ImportAgentRequest ' services/api/internal/api/types.gen.go | awk '{ if ($1 >= 1) exit 0; else exit 1 }'`) returned 0 / fail. Without these types, Plan 03+ cannot type-decode incoming rows.
- **Fix:** Set `skip-prune: true` in `services/api/internal/api/oapi-codegen.types.yaml` with a header comment explaining the F2-driven rationale. Re-ran `task gen` — all 6 types now present.
- **Files modified:** `services/api/internal/api/oapi-codegen.types.yaml`
- **Verification:**
  - `grep -c "type Import.*Request " services/api/internal/api/types.gen.go` → 6 (one per entity).
  - `grep 'BulkImportCatalogJSONBody = \[\]interface{}' services/api/internal/api/types.gen.go` → 1 (F2 invariant retained).
  - `go vet ./...` → pass; `go build ./...` → pass.
  - Two consecutive `task gen` runs produce sha256-identical outputs.
- **Committed in:** `277f3e5` (Task 3 commit — bundled with the regen output)

**2. [Rule 2 - Missing Critical] Added `x-import-row-types` yaml extension on `requestBody`**
- **Found during:** Task 1 (DELTA 8 in plan was "add a no-op `x-import-row-types` extension")
- **Issue:** The plan listed this as DELTA 8 (so technically not a deviation — but worth documenting because it's the kind of thing easy to miss). The extension is documentation-only; oapi-codegen does NOT honour `x-*` extensions for pruning (verified — the schemas were still pruned until skip-prune was flipped).
- **Fix:** Inserted `x-import-row-types: [ImportAgentRequest, ImportSkillRequest, ImportQueueRequest, ImportChannelRequest, ImportAdapterRequest, ImportBreakReasonRequest]` on the `requestBody` object. Future tools that introspect the spec (e.g., custom client generators, doc generators) can use this list.
- **Files modified:** `openapi/openapi.yaml`
- **Verification:** Inline in the spec at the `requestBody` for `BulkImportCatalog`; yaml syntactic sanity check passes.
- **Committed in:** `d0a57ab` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 Rule 3 blocking — codegen pruning).
**Impact on plan:** Necessary for Plan 03+ to compile. No scope creep. The `oapi-codegen.types.yaml` change is documented inline so future agents understand why skip-prune is on.

## Issues Encountered

- **pnpm workspace had `node_modules` missing** when first running `task gen`. Ran `pnpm install --frozen-lockfile` from the `web/` directory; 380 packages restored from the existing lockfile. No new dependencies, no lockfile drift. Subsequent `task gen` runs succeed without re-install.
- **Plan's verify command referenced a stale worktree path** (`/Users/luong/workspace/dev/open-solutions/.claude/worktrees/frosty-khayyam-9c3d54`). The actual worktree spawned for this execution is `agent-aa68b0a763c9a9c58`. Used the live `pwd` everywhere; the structural intent of every verify command is preserved.
- **`grep -c '^        IdempotencyKeyHeader:'` mismatch in plan verify**: the plan's verify expression had 8-space indentation for `IdempotencyKeyHeader`, but `components.parameters` items live at 4-space indent in this codebase. Used `^    IdempotencyKeyHeader:` (correct yaml indentation) — count = 1. Acceptance is met in spirit.

## Acceptance Gates (grep results pasted)

```
=== (org_id, code) appears in openapi.yaml ===
10                                                      # exceeds ≥1 requirement

=== 6 schemas in yaml + Go + TS ===
ImportAgentRequest: yaml=1, go=1, ts=1
ImportSkillRequest: yaml=1, go=1, ts=1
ImportQueueRequest: yaml=1, go=1, ts=1
ImportChannelRequest: yaml=1, go=1, ts=1
ImportAdapterRequest: yaml=1, go=1, ts=1
ImportBreakReasonRequest: yaml=1, go=1, ts=1

=== IdempotencyKeyHeader chain ===
openapi.yaml components.parameters: 1
types.gen.go IdempotencyKeyHeader refs: 3 (type alias + bind + param field)

=== ImportJobStatus chain ===
types.gen.go ImportJobStatus refs: 9 (type + 3 enum consts + Valid() + ImportJob.Status field + comment)
types.gen.go IdempotentReplay refs: 2 (field decl + comment)

=== F2 invariant retained ===
items: {} in openapi.yaml: 2 (line 239 — pre-existing _scaffold-era; line 3283 — BulkImportCatalog)
type BulkImportCatalogJSONBody = []interface{}: present

=== import_jobs in sqlcheck tenantTables ===
sqlcheck.go line 42: "import_jobs":   {}, // IMP-06 (Phase 5; …)

=== import_jobs migration up — table + 3 indexes ===
CREATE TABLE import_jobs (
CREATE INDEX ix_import_jobs_org_created
CREATE INDEX ix_import_jobs_pending_updated
CREATE UNIQUE INDEX uq_import_jobs_org_idempotency

=== sqlc auto-emit ===
db/generated/models.go: type ImportJob struct { … } present (sqlc auto-detected new migration)

=== Idempotency-Key wired in server.gen.go ===
server.gen.go IdempotencyKey: 3 refs (declaration, bind, assign)

=== task gen idempotency (sha256 stable across two runs) ===
IDEMPOTENT_CODEGEN_PASS

=== Final CI drift gate ===
git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/ services/api/internal/db/generated/
exit code: 0 (CI codegen-drift gate green)
```

## Self-Check

- [x] Plan verify command (Task 1) returns `OPENAPI_DELTAS_OK`.
- [x] Plan verify command (Task 2) returns `MIGRATION_AND_SQLCHECK_OK`.
- [x] Plan verify command (Task 3) returns `CODEGEN_OK` + `git diff --exit-code` returns 0.
- [x] `go vet ./...` returns 0; `go build ./...` returns 0; `go test -count=1 -short -run='^$' ./...` compile pass.
- [x] `task gen` idempotency: sha256sum stable across two consecutive runs.
- [x] F2 invariant: `BulkImportCatalogJSONBody = []interface{}` retained.
- [x] All 7 frontmatter requirements (IMP-01, IMP-03..08) traceable to schemas/migration/sqlcheck/codegen outputs.
- [x] Three commits exist in git log: `d0a57ab` (feat openapi), `bed566d` (feat migration+sqlcheck), `277f3e5` (chore codegen+config).

## Next Plan Readiness

- **Plan 02 (sqlc queries for import_jobs + agent_skills MergeAgentSkill + skills ResolveSkillCodes)** is unblocked. The migration is in place; sqlc auto-detected the new table; sqlcheck.go allowlist is set. Plan 02 only needs to author the `.sql` query files in `services/api/internal/db/queries/` and re-run `sqlc generate`.
- **Plan 03+ (handler implementation)** can rely on `ImportJobStatus` enum, `BulkImportCatalogParams.IdempotencyKey *IdempotencyKeyHeader`, `BulkImportResult.IdempotentReplay *bool`, and the 6 `Import*Request` Go types being available in `services/api/internal/api/types.gen.go`.
- **Threat surface scan:** none new beyond plan's `<threat_model>` — the surface introduced (parametric `application/json` body, Idempotency-Key header, `idempotency_key` partial UNIQUE) is already enumerated in T-05-01-01..05. No `threat_flag:` entries warranted.
- **Open follow-ups for plan-phase orchestrator:**
  - STATE.md / ROADMAP.md updates — owned by orchestrator (this executor did not write either per `<parallel_execution>` directive).
  - REQUIREMENTS.md will be marked complete for IMP-01, IMP-03..08 by the orchestrator's `requirements mark-complete` call. **Note:** strictly speaking, IMP-01..08 require the *handler* to ship — Wave 0 only delivers the contract scaffolding. Reasonable to consider these "in progress" until Plan 03+ ships the implementation; the orchestrator may choose to mark them complete only at the phase boundary rather than per-plan.

## Self-Check: PASSED

All files claimed in this SUMMARY exist on disk:
- migrations/000003_create_import_jobs.up.sql — FOUND
- migrations/000003_create_import_jobs.down.sql — FOUND
- services/api/internal/db/sqlcheck.go — FOUND
- services/api/internal/api/oapi-codegen.types.yaml — FOUND
- services/api/internal/api/types.gen.go — FOUND
- services/api/internal/api/server.gen.go — FOUND
- services/api/internal/api/spec.gen.go — FOUND
- services/api/internal/db/generated/models.go — FOUND
- web/packages/ui/src/api/generated.ts — FOUND
- openapi/openapi.yaml — FOUND
- .planning/phases/05-bulk-import-go/05-01-SUMMARY.md — FOUND

All commits claimed in this SUMMARY exist in git log:
- d0a57ab (Task 1: openapi.yaml deltas) — FOUND
- bed566d (Task 2: migration 000003 + sqlcheck) — FOUND
- 277f3e5 (Task 3: codegen regen + skip-prune fix) — FOUND

---
*Phase: 05-bulk-import-go*
*Completed: 2026-05-17*
