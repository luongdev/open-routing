# Phase 5: Bulk Import (Go) — Pattern Map

**Mapped:** 2026-05-17
**Files analyzed:** 33 (24 NEW, 7 MODIFIED, 2 auto-gen)
**Analogs found:** 31 / 33 (2 pure-new have no analog — coercer + JSON re-marshal helper)
**Padded phase:** 05

> **CRITICAL FOR PLANNER:** Phase 5 is greenfield code stitched into a heavily-templated codebase. The vast majority of files have an exact analog from Phase 3 / Phase 4 / Phase 04.1. Only two files (`coerce.go`, `parser_json.go` re-marshal helper) are pure-new; everything else copies an existing shape with surgical deltas. The locked Phase 04.1 contract (`code` is the upsert key, `external_id` is optional integration mapping per D04_1-01 / D04_1-07) governs every FK reference in the new `Import*Request` schemas.
>
> Three architecture-shaping constraints from RESEARCH §Architectural Findings are MANDATORY:
> - **F1** — strict-server eagerly decodes JSON; Phase 5 accepts this in v0.1 (memory bounded by `MaxBytesReader`).
> - **F2** — oapi-codegen v2 has NO `AsX/FromX` for `oneOf` request bodies; keep `items: {}` + `?entity=` dispatch.
> - **F3** — stubs live at `services/api/internal/catalog/notimpl.go:28,34` (NOT `server/stubs.go`); add a third embed `*imports.Importer` to `ApiHandlers` in `cmd/api/main.go:178`.
>
> Three Phase 3 / Phase 04.1 invariants govern every Phase 5 file:
> - **D-68 one-file-per-entity** — each of the 6 entity row-processors lives in its own `row_<entity>.go`.
> - **D-72 per-entity `_test.go`** — paired test file per source file.
> - **D-74 two-layer validation** — Layer 1 (format/regex) at handler boundary; Layer 2 (business rule) right before DB call.

## File Classification

### NEW Files (24)

| File | Role | Data Flow | Closest Analog | Match Quality |
|------|------|-----------|----------------|---------------|
| `migrations/000003_create_import_jobs.up.sql` | migration | schema-additive | `migrations/000002_catalog_v0_1.up.sql` (agent_states block lines 320-360) | exact |
| `migrations/000003_create_import_jobs.down.sql` | migration | schema rollback | `migrations/000002_catalog_v0_1.down.sql` | exact |
| `services/api/internal/db/queries/import_jobs.sql` | sqlc query | CRUD + sweep | `services/api/internal/db/queries/agent_states.sql` (ExpireWrapUp + ListExpiringWrapUps for sweeper) | exact |
| `services/api/internal/middleware/bodylimit.go` | middleware | request-response (size-gate) | `services/api/internal/middleware/uuidv7path.go` | exact (path-scoped chi MiddlewareFunc) |
| `services/api/internal/middleware/bodylimit_test.go` | unit test | request-response | `services/api/internal/middleware/uuidv7path_test.go` | exact |
| `services/api/internal/imports/doc.go` | package scaffold | package-doc | `services/api/internal/state/handlers.go` (header doc block) | role-match |
| `services/api/internal/imports/handlers.go` | package scaffold (Deps + New + Start/Stop) | request-response + bg-goroutine | `services/api/internal/state/handlers.go` (lines 1-164) | exact |
| `services/api/internal/imports/handler_import.go` | controller (BulkImportCatalog) | request-response + tx + streaming | `services/api/internal/state/agent_states.go` `PatchAgentStatus` (lines 107-346) | role-match (composite: orgID extract + body validation + tx + idempotency check) |
| `services/api/internal/imports/handler_get_job.go` | controller (GetImportJob) | request-response + cache-miss-only | `services/api/internal/state/agent_states.go` `GetAgentStatus` (lines 40-99) | exact |
| `services/api/internal/imports/coerce.go` | utility (typed pipeline) | pure-transform | none in-tree (RESEARCH §Pattern 1, D5-01..D5-08) — closest structural cousin is `services/api/internal/catalog/codecheck.go` (pure-function file) | no-analog |
| `services/api/internal/imports/parser_json.go` | utility (re-marshal helper) | pure-transform + per-row decode | none in-tree (RESEARCH §Pattern 4, F1 accommodation) | no-analog |
| `services/api/internal/imports/parser_csv.go` | utility (BOM strip + utf8 validate + csv.Reader factory) | streaming read | none in-tree (RESEARCH §Pattern 3, D5-08); structural cousin `services/api/internal/catalog/cursor.go` | role-match (pure-utility file with package-init compile + unit tests) |
| `services/api/internal/imports/header.go` | utility (per-entity column registry) | pure-transform (config) | `services/api/internal/state/transitions.go` (matrix-as-data pattern) | role-match (per-entity static registry) |
| `services/api/internal/imports/row_agent.go` | controller (per-entity row processor) | CRUD + nested-skill resolve + tx | `services/api/internal/catalog/agents.go` `CreateAgent` (lines 71-207) | exact (orgID + Layer 1 + qtx + UpsertAgentByCode + InsertAgentStateOnConflictNothing + MergeAgentSkill loop) |
| `services/api/internal/imports/row_skill.go` | controller (per-entity row processor) | CRUD | `services/api/internal/catalog/skills.go` `CreateSkill` (analog of `agents.CreateAgent`) | exact |
| `services/api/internal/imports/row_queue.go` | controller (per-entity row processor) | CRUD | `services/api/internal/catalog/queues.go` `CreateQueue` | exact |
| `services/api/internal/imports/row_channel.go` | controller (per-entity row processor) | CRUD + FK code-lookup probe | `services/api/internal/catalog/channels.go` `CreateChannel` (D-76 FK probe at lines 55-73) | exact |
| `services/api/internal/imports/row_adapter.go` | controller (per-entity row processor) | CRUD + JSONB passthrough | `services/api/internal/catalog/adapters.go` `CreateAdapter` | exact |
| `services/api/internal/imports/row_break_reason.go` | controller (per-entity row processor) | CRUD | `services/api/internal/catalog/break_reasons.go` `CreateBreakReason` | exact |
| `services/api/internal/imports/chunk.go` | utility (chunked-savepoint orchestrator) | CRUD + tx + savepoint | `services/api/internal/catalog/agents.go` `CreateAgent` tx pattern (lines 114-197) + RESEARCH §Pattern 5 | role-match (savepoint per row INSIDE outer tx) |
| `services/api/internal/imports/jobs.go` | utility (import_jobs lifecycle) | CRUD | `services/api/internal/catalog/agents.go` `UpdateAgent` 0-row disambiguate (lines 384-440) — same INSERT-then-UPDATE-after-work shape | role-match |
| `services/api/internal/imports/idempotency.go` | utility (replay lookup) | CRUD (read-only) | `services/api/internal/state/agent_states.go` `GetAgentStatus` cache-loader pattern (lines 40-99) | role-match (single DB lookup → rehydrate) |
| `services/api/internal/imports/sweep.go` | bg-goroutine (crash-recovery sweep) | event-driven + bg-poll | `services/api/internal/state/ttl.go` `safetySweep` + `runSweepPastDue` + `startupSweep` (lines 132-196) | exact (Phase 4 STATE-07 pattern — `clockwork.Clock` + `time.Ticker` + `db.WithBypass` + WaitGroup) |
| `services/api/internal/imports/errors.go` | utility (per-row error mapping) | pure-transform | `services/api/internal/catalog/errors.go` `mapPgError` (REUSED; Phase 5 wraps verdict into `BulkImportFailedRow`) | role-match (Phase 5 calls existing `mapPgError`, adds row-failure shape) |
| `services/api/internal/imports/mappers.go` | utility (sqlc-row → wire-shape) | pure-transform | `services/api/internal/state/mappers.go` (1.7KB analog) | exact |
| `services/api/internal/imports/handlers_test.go` | integration test (entity × format matrix) | request-response | `services/api/internal/state/agent_states_test.go` (816 lines) | exact |
| `services/api/internal/imports/coerce_test.go` | unit test (pure pipeline) | pure-transform | `services/api/internal/catalog/codecheck_test.go` (pure-function table-driven) | role-match |
| `services/api/internal/imports/parser_csv_test.go` | unit test (BOM/CRLF/embedded quotes) | streaming | `services/api/internal/state/transitions_test.go` (table-driven pure unit) | role-match |
| `services/api/internal/imports/parser_json_test.go` | unit test | pure-transform | same as coerce_test.go | role-match |
| `services/api/internal/imports/header_test.go` | unit test (column registry) | pure-transform | same as coerce_test.go | role-match |
| `services/api/internal/imports/chunk_test.go` | integration test (testcontainers) | CRUD + tx + savepoint | `services/api/internal/catalog/agents_test.go` UpdateAgent version-conflict test (around line 600) | role-match |
| `services/api/internal/imports/jobs_test.go` | integration test (testcontainers) | CRUD | `services/api/internal/state/ttl_test.go` (clockwork-backed integration) | role-match |
| `services/api/internal/imports/idempotency_test.go` | integration test (testcontainers) | CRUD | same as jobs_test.go | role-match |
| `services/api/internal/imports/sweep_test.go` | integration test (clockwork) | event-driven | `services/api/internal/state/ttl_test.go` (FakeClock + Advance + Eventually pattern) | exact |
| `services/api/internal/imports/main_test.go` | scaffold (TestMain bring-up) | scaffold | `services/api/internal/state/main_test.go` (4.7KB) | exact (verbatim re-use; only package path changes) |
| `services/api/internal/imports/testutil_test.go` | shared test fixtures | scaffold | `services/api/internal/state/testutil_test.go` (20.6KB) | exact (composite ApiHandlers wiring + `cleanImportTables` per-org DELETE) |
| `services/api/internal/imports/testdata/agents-basic.{json,csv}` | golden test data | data | none (new convention; RESEARCH §Validation Architecture) | no-analog |
| `services/api/internal/imports/testdata/windows-excel-agents.csv` | golden test data (UTF-8 BOM + CRLF) | data | none (REQUIRED — Pitfall 1) | no-analog |
| `services/api/test/isolation/imports_test.go` | integration test (cross-org probes) | request-response | `services/api/test/isolation/state_test.go` (9.1KB) + `services/api/test/isolation/catalog_test.go` `TestCatalog_CrossOrgSameCode_BothSucceed` (line 375) | exact |
| `services/api/test/isolation/imports_migration_idempotent_test.go` | integration test (migration replay) | DB migration | `services/api/test/isolation/migration_idempotent_test.go` (existing — Phase 04.1 ship) | exact |

### MODIFIED Files (7)

| File | Modification Type | Closest Analog | Match Quality |
|------|------------------|----------------|---------------|
| `openapi/openapi.yaml` | spec amendment (D-75 in-place) | itself (Phase 04.1's 41-yaml-delta amend pattern) | exact |
| `services/api/internal/catalog/notimpl.go` | DELETE BulkImportCatalog + GetImportJob stubs | itself (Phase 4 already removed `GetAgentStatus` + `PatchAgentStatus` stubs from this same file) | self-deletion |
| `services/api/internal/db/queries/agent_states.sql` | APPEND `InsertAgentStateOnConflictNothing :exec` | itself (existing `InsertAgentState` at lines 1-7) | self-extension |
| `services/api/internal/db/queries/agent_skills.sql` | APPEND `MergeAgentSkill :exec` | itself (existing `InsertAgentSkill` at lines 29-32 — closest shape; gets ON CONFLICT clause) | self-extension |
| `services/api/internal/db/queries/skills.sql` | APPEND `ResolveSkillCodes :many` | `agent_skills.sql` `SkillsPresentInOrg :many` (lines 74-78) — same `ANY($1::uuid[]) + org_id = $2` shape | role-match |
| `services/api/internal/db/sqlcheck.go` | extend `tenantTables` allowlist | itself (Phase 4 added `"agent_states": {}` at line 41) | self-extension |
| `services/api/internal/db/orgdb.go` | APPEND `(*OrgTx).BeginSavepoint(ctx)` method | itself (existing `(*OrgDB).BeginTx` at line 223 — same return-shape pattern but on Tx) | self-extension |
| `services/api/cmd/api/main.go` | wire `*imports.Importer` into `ApiHandlers` + Start/Stop crash sweep | itself (lines 150-194 — Phase 4 added `*state.Server`) | self-extension |

### Codegen Output Files (DO NOT hand-edit — `task gen` regenerates)

| File | Mechanism |
|------|-----------|
| `services/api/internal/api/server.gen.go` | oapi-codegen — `BulkImportCatalogParams` gains `IdempotencyKey *uuid` |
| `services/api/internal/api/types.gen.go` | oapi-codegen — emits new `ImportAgentRequest`, `ImportSkillRequest`, `ImportQueueRequest`, `ImportChannelRequest`, `ImportAdapterRequest`, `ImportBreakReasonRequest`; `ImportJob.Status` typed enum constants; `BulkImportResult.IdempotentReplay *bool`; `BulkImportCatalogJSONBody` stays `[]interface{}` per §F2 |
| `services/api/internal/api/spec.gen.go` | oapi-codegen — embeds regenerated yaml bytes |
| `services/api/internal/db/generated/*.sql.go` | sqlc — emits 5 new `import_jobs.sql` methods + `MergeAgentSkill` + `ResolveSkillCodes` + `InsertAgentStateOnConflictNothing` + `BeginSavepoint` is hand-written on OrgTx (NOT codegen) |
| `web/packages/ui/src/api/generated.ts` | openapi-typescript via `pnpm gen:api` |

---

## Pattern Assignments

### `migrations/000003_create_import_jobs.up.sql` (NEW migration)

**Analog:** `migrations/000002_catalog_v0_1.up.sql` — specifically the Phase 4 `agent_states` table append block. NEW file (not amending 000002) per RESEARCH §Open Question Q4 recommendation: `import_jobs` is a Phase 5-owned table, not catalog identity; the editable-migration policy (D-61) applies to 000002 only.

**BEGIN/COMMIT envelope** (matches 000002 lines 19, 214):

```sql
BEGIN;

-- IMP-06 / Phase 5: import_jobs persistence. One row per POST to
-- /catalog/import. Held in pending state during processing; finalised
-- to completed/failed after the last chunk commits. The 24h sweep
-- (D5-11) flips orphaned pending → failed.
CREATE TABLE import_jobs (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    entity_type     TEXT NOT NULL
                      CHECK (entity_type IN ('agents','skills','queues','channels','adapters','break_reasons')),
    status          TEXT NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','completed','failed')),
    total_rows      INTEGER NOT NULL DEFAULT 0,
    succeeded_rows  INTEGER NOT NULL DEFAULT 0,
    failed_rows     INTEGER NOT NULL DEFAULT 0,
    errors          JSONB,
    idempotency_key TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ix_import_jobs_org_created
    ON import_jobs (org_id, created_at DESC, id DESC);

-- D5-11 crash sweep partial index: scan only pending rows
CREATE INDEX ix_import_jobs_pending_updated
    ON import_jobs (updated_at)
    WHERE status = 'pending';

-- D5-13 idempotency partial UNIQUE: enforce only when key is present
CREATE UNIQUE INDEX uq_import_jobs_org_idempotency
    ON import_jobs (org_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

COMMIT;
```

**Pattern hazards:**

- **No `enabled` column** — `import_jobs` is event-history, not a soft-deletable entity (mirror `agent_states` deviation per Phase 4 PATTERNS line 1009).
- **No `version` column** — counters monotonically increase; no optimistic concurrency. `state_version` on agent_states is BIGINT for the same "fires often" reason; `import_jobs` counters are INTEGER because each row is touched at most 3 times (INSERT → finalise → maybe sweep).
- **NO FK to anything** — D-80 invariant carry-forward (app-layer probes only).
- **`org_id` is denormalized** (no FK) so SQLChecker (D-02) sees the column on every query.

**Critical sweep query alignment** — `errors` JSONB shape must match `BulkImportResult.failed[]` exactly (D5-12). The sweep query writes:

```sql
errors = '[{"row":0,"error":"import_failed","reason":"server_crash"}]'::jsonb
```

— row 0 is the synthetic sentinel (RESEARCH §Pattern 7 sweep SQL).

---

### `migrations/000003_create_import_jobs.down.sql` (NEW migration rollback)

**Analog:** `migrations/000002_catalog_v0_1.down.sql` (BEGIN; DROP TABLE; COMMIT pattern).

```sql
BEGIN;
DROP TABLE IF EXISTS import_jobs;
COMMIT;
```

Per D-25 / D04_1-12, v0.1 down migration is documentation-only; production uses `task db:reset`. No need for column-level rollback.

---

### `services/api/internal/db/queries/import_jobs.sql` (NEW sqlc query file)

**Analogs (composite):**

- **`InsertImportJob`** ← `agents.sql` `InsertAgent` (lines 29-32): basic `:one` INSERT with explicit `(id, org_id, ...)` for SQLChecker.
- **`FinaliseImportJob`** ← `agents.sql` `UpdateAgent` (lines 98-108): version-checked `UPDATE` (but Phase 5 doesn't version — just `WHERE id = $5 AND org_id = $6`).
- **`GetImportJob`** ← `agents.sql` `GetAgent` (lines 34-39): `:one` SELECT by (id, org_id).
- **`LookupImportJobByIdempotencyKey`** ← same shape as `GetAgent` but WHERE clause swaps id → idempotency_key.
- **`SweepCrashedImportJobs`** ← `agent_states.sql` `ListExpiringWrapUps` (lines 69-78) + `ExpireWrapUp` (lines 80-106) composite: returns affected rows for logging.

**Verbatim sweep pattern** (from agent_states.sql lines 69-78, adapted for import_jobs):

```sql
-- name: SweepCrashedImportJobs :many
-- D5-11 crash-recovery sweep: flip pending → failed for jobs > 24h old.
-- Phase 5 sweep goroutine wraps ctx with db.WithBypass(ctx, "import_crash_sweep")
-- (mirror state/ttl.go:179 "wrapup_sweeper.safety" pattern). The SELECT
-- mentions org_id so SQLChecker (D-02) accepts the org-agnostic UPDATE.
UPDATE import_jobs
SET status = 'failed',
    errors = '[{"row":0,"error":"import_failed","reason":"server_crash"}]'::jsonb,
    updated_at = NOW()
WHERE status = 'pending'
  AND updated_at < NOW() - INTERVAL '24 hours'
RETURNING id, org_id;
```

**Full query inventory (5 queries):**

```sql
-- name: InsertImportJob :one
INSERT INTO import_jobs (
    id, org_id, entity_type, status, total_rows,
    succeeded_rows, failed_rows, errors, idempotency_key
)
VALUES ($1, $2, $3, 'pending', $4, 0, 0, NULL, sqlc.narg('idempotency_key'))
RETURNING id, org_id, entity_type, status, total_rows, succeeded_rows,
          failed_rows, errors, idempotency_key, created_at, updated_at;

-- name: FinaliseImportJob :one
UPDATE import_jobs
SET status = $1,
    succeeded_rows = $2,
    failed_rows = $3,
    errors = $4,
    updated_at = NOW()
WHERE id = $5 AND org_id = $6
RETURNING id, org_id, entity_type, status, total_rows, succeeded_rows,
          failed_rows, errors, idempotency_key, created_at, updated_at;

-- name: GetImportJob :one
SELECT id, org_id, entity_type, status, total_rows, succeeded_rows,
       failed_rows, errors, idempotency_key, created_at, updated_at
FROM import_jobs
WHERE id = $1 AND org_id = $2;

-- name: LookupImportJobByIdempotencyKey :one
SELECT id, org_id, entity_type, status, total_rows, succeeded_rows,
       failed_rows, errors, idempotency_key, created_at, updated_at
FROM import_jobs
WHERE org_id = $1 AND idempotency_key = $2;

-- name: SweepCrashedImportJobs :many
-- (see verbatim above)
```

**Pattern hazards** (Phase 3 H3 + Phase 4 ListExpiringWrapUps comment):

- Every query mentions `org_id` in WHERE/INSERT/UPDATE (D-02 SQLChecker).
- `SweepCrashedImportJobs` is the lone exception — RETURNING mentions org_id which satisfies SQLChecker because the predicate is org-agnostic by intent and the goroutine wraps with `db.WithBypass(ctx, "import_crash_sweep")`.
- `idempotency_key` uses `sqlc.narg` so `NULL` is the cardinality for "no key passed."

---

### `services/api/internal/db/queries/agent_skills.sql` (MODIFIED — append MergeAgentSkill)

**Analog:** `agent_skills.sql` itself (3.5KB existing file). Append AFTER `SkillsPresentInOrg :many` (last query at line 79).

**Reference existing `InsertAgentSkill` (lines 29-32) — Phase 5's MergeAgentSkill extends with ON CONFLICT:**

```sql
-- name: InsertAgentSkill :one
INSERT INTO agent_skills (agent_id, skill_id, org_id, proficiency)
VALUES ($1, $2, $3, $4)
RETURNING agent_id, skill_id, org_id, proficiency, created_at;
```

**Phase 5 addition (verbatim from RESEARCH §Pattern 7 lines 836-848):**

```sql
-- name: MergeAgentSkill :exec
-- D5-18 + D5-19: skill assignment MERGE on import (post-04.1).
-- ON CONFLICT (agent_id, skill_id) DO UPDATE so existing assignments get
-- the new proficiency from the import (D5-19 import wins). Existing skills
-- NOT in this payload are LEFT INTACT (D5-18 PATCH-like) — achieved by
-- simply NOT issuing DELETE statements (do NOT call replaceAgentSkills
-- from Phase 3's catalog/agent_skills.go:74 which uses DELETE-ALL + INSERT-N).
INSERT INTO agent_skills (agent_id, skill_id, org_id, proficiency)
VALUES ($1, $2, $3, $4)
ON CONFLICT (agent_id, skill_id) DO UPDATE
SET proficiency = EXCLUDED.proficiency;
```

**Pattern hazards:**

- `:exec` (NOT `:one`) — Phase 5 doesn't need the row back; ON CONFLICT UPDATE makes the RETURNING ambiguous about which version landed. Mirror agent_states.sql `ExpireWrapUp` which returns the post-state.
- `org_id` is in the params for SQLChecker even though `(agent_id, skill_id)` already implies organization scope through the schema FK semantics. D-02 doesn't negotiate.

---

### `services/api/internal/db/queries/skills.sql` (MODIFIED — append ResolveSkillCodes)

**Analog:** `agent_skills.sql` `SkillsPresentInOrg :many` (lines 74-78) — same `ANY($::[]) + org_id = $` shape.

```sql
-- (SkillsPresentInOrg verbatim, for reference:)
SELECT id
FROM skills
WHERE id = ANY($1::uuid[])
  AND org_id = $2
  AND enabled = TRUE;
```

**Phase 5 addition (verbatim from RESEARCH §Pattern 7 lines 822-833):**

```sql
-- name: ResolveSkillCodes :many
-- D5-16 + D5-17: batch lookup of skill UUIDs by `code` (post-04.1).
-- Used by the agent row processor to resolve nested skill references
-- once per chunk. Returns id + code pairs. Missing codes are absent
-- from the result; handler distinguishes "unknown skill" by set difference
-- (mirror agent_skills.go:88 SkillsPresentInOrg miss-detection pattern).
SELECT id, code
FROM skills
WHERE org_id = $1 AND code = ANY($2::text[]);
```

**Critical pattern from `SkillsPresentInOrg`'s header comment (lines 60-73):**

> Outer FROM is `skills` (a tenant alias) so the orgDB SQLChecker accepts the org_id binding in WHERE. The id = ANY($1::uuid[]) predicate filters to only the rows the caller wants to probe.

Phase 5 mirrors verbatim — `ResolveSkillCodes` is a tenant-aliased top-level SELECT with `org_id = $1` in WHERE.

---

### `services/api/internal/db/queries/agent_states.sql` (MODIFIED — append InsertAgentStateOnConflictNothing)

**Analog:** `agent_states.sql` itself — existing `InsertAgentState :one` (lines 1-7).

**Reference existing pattern:**

```sql
-- name: InsertAgentState :one
-- D-93: called from catalog.CreateAgent inside the existing OrgTx so
-- agent + agent_state INSERTs commit atomically (Codex C4 pattern).
INSERT INTO agent_states (agent_id, org_id, status, state_version)
VALUES ($1, $2, $3, 1)
RETURNING agent_id, org_id, status, engaged_channel, break_reason_id,
         post_interaction_state, wrapup_until, state_version, updated_at;
```

**Phase 5 addition (per RESEARCH §Pattern 7 lines 850-859 and Pitfall 3 / Phase 4 Hazard 7):**

```sql
-- name: InsertAgentStateOnConflictNothing :exec
-- Phase 5 import: seed state for NEW agents only; re-imports MUST NOT
-- regress the state machine to Offline (Pitfall 3 / Phase 4 Hazard 7).
-- Differs from InsertAgentState (Phase 4, line 1) which is unconditional —
-- adding ON CONFLICT to the existing query would change Phase 4 CreateAgent
-- semantics. Phase 5 authors this as a separate sqlc query.
INSERT INTO agent_states (agent_id, org_id, status, state_version)
VALUES ($1, $2, $3, 1)
ON CONFLICT (agent_id) DO NOTHING;
```

**Pattern hazard:** `:exec` (not `:one`) — on conflict, no row is returned. The agent row processor doesn't need to read back state; it only ensures the row exists.

---

### `services/api/internal/db/sqlcheck.go` (MODIFIED — add import_jobs to tenantTables)

**Analog:** itself — `tenantTables` map at lines 32-42 (verified — Phase 4 added `"agent_states": {}` at line 41).

**Existing pattern (sqlcheck.go around line 32-42):**

```go
var tenantTables = map[string]struct{}{
    "_scaffold":     {}, // legacy Phase 1
    "agents":        {}, // CAT-01
    "agent_skills":  {}, // CAT-03
    "agent_states":  {}, // STATE-01 (Phase 4)
    "skills":        {}, // CAT-02
    "queues":        {}, // CAT-04
    "channels":      {}, // CAT-05
    "adapters":      {}, // CAT-06
    "break_reasons": {}, // CAT-07
}
```

**Phase 5 addition (one line, alphabetical insert order helps grep):**

```go
    "import_jobs":   {}, // IMP-06 (Phase 5; denormalized org_id per RESEARCH §Pattern 8)
```

**Pattern hazard** (Phase 4 Pitfall 4 carry-forward / RESEARCH Pitfall 2): without this addition, the FIRST sqlc-generated query against `import_jobs` panics in dev/test with `ErrSQLMissingOrgFilter`. Wave 0 task description MUST list this as a step adjacent to the migration apply.

---

### `services/api/internal/db/orgdb.go` (MODIFIED — append BeginSavepoint)

**Analog:** itself — existing `(*OrgDB).BeginTx` at line 223:

```go
// BeginTx starts a transaction inheriting the parent OrgDB's checker and
// mode. The returned OrgTx satisfies generated.DBTX so handler code can
// pass it to generated.New(tx) for sqlc-typed transactional queries
// (OQ-5, H5 — used by the agent_skills full-replace in Plan 03-09).
// Default pgx.TxOptions (read-write, default isolation) — handlers
// needing read-only or stricter isolation should add an Options-accepting
// overload in v0.2.
func (o *OrgDB) BeginTx(ctx context.Context) (*OrgTx, error) {
    tx, err := o.pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        return nil, fmt.Errorf("orgdb: begin tx: %w", err)
    }
    return &OrgTx{tx: tx, checker: o.checker, mode: o.mode}, nil
}
```

**Phase 5 addition (verbatim from RESEARCH §Pattern 5 lines 619-628):**

```go
// BeginSavepoint starts a savepoint on the current Tx and returns a new
// *OrgTx wrapping the child. SQLChecker preflight is preserved on every
// Exec/Query/QueryRow against the returned child Tx. The pgx Tx.Begin
// method internally implements SAVEPOINT semantics on a non-top-level
// Tx (verified — pkg.go.dev/github.com/jackc/pgx/v5).
//
// Phase 5 chunk loop: outerTx.BeginSavepoint(ctx) per row inside a chunk
// of 50, then sp.Commit (RELEASE) on success or sp.Rollback (ROLLBACK TO)
// on per-row failure (D5-09).
func (t *OrgTx) BeginSavepoint(ctx context.Context) (*OrgTx, error) {
    child, err := t.tx.Begin(ctx)
    if err != nil {
        return nil, fmt.Errorf("orgtx: begin savepoint: %w", err)
    }
    return &OrgTx{tx: child, checker: t.checker, mode: t.mode}, nil
}
```

**Pattern hazards:**

- The returned `*OrgTx` carries the SAME checker pointer — preflight semantics are identical to the parent Tx.
- pgx automatically generates unique SAVEPOINT names internally — Phase 5 does NOT issue raw `SAVEPOINT row_N` strings.
- Calling `BeginSavepoint` on a Tx that has already been committed/rolled-back returns a `tx closed` error from pgx (RESEARCH Pitfall 8).

---

### `services/api/internal/middleware/bodylimit.go` (NEW middleware)

**Analog:** `services/api/internal/middleware/uuidv7path.go` (2.7KB; structural cousin — same shape: path-prefix gate + `next.ServeHTTP` plumbing + early return on rejection via `WriteError`).

**Reference pattern from existing `requestid.go` (lines 1-50) — chi MiddlewareFunc shape:**

```go
// Source: services/api/internal/middleware/requestid.go
package middleware

import "net/http"

func RequestID() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // body...
            next.ServeHTTP(w, r)
        })
    }
}
```

**Phase 5 verbatim from RESEARCH §Pattern 2 lines 489-516** (already in research file as code excerpt):

```go
// services/api/internal/middleware/bodylimit.go (NEW)
package middleware

import (
    "net/http"
    "strconv"
    "strings"
)

// BodyLimit returns a chi MiddlewareFunc that rejects requests whose
// declared Content-Length exceeds maxBytes and wraps r.Body in
// http.MaxBytesReader so mid-stream over-limit also surfaces as
// *http.MaxBytesError (D5-21). pathPrefix scopes the middleware to a
// single URL so other endpoints retain unlimited bodies.
func BodyLimit(maxBytes int64, pathPrefix string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !strings.HasPrefix(r.URL.Path, pathPrefix) {
                next.ServeHTTP(w, r)
                return
            }
            // (a) Content-Length pre-flight (when present and trusted)
            if cl := r.Header.Get("Content-Length"); cl != "" {
                if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > maxBytes {
                    WriteError(r.Context(), w, http.StatusRequestEntityTooLarge,
                        "invalid_body", "request_too_large_use_async_pathway")
                    return
                }
            }
            // (b) Wrap r.Body so mid-stream over-limit also fails.
            r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
            next.ServeHTTP(w, r)
        })
    }
}
```

**Pattern hazards** (RESEARCH §Pattern 2 + Pitfall 5):

- **Use the existing `WriteError(ctx, w, status, code, reason)` from `httputil.go`** — do NOT invent a parallel error emitter. The 413 wire shape uses `error="invalid_body", reason="request_too_large_use_async_pathway"` (research locked the reason string).
- **Path-scoped via prefix** — Open Q7 recommendation: chain order is `RequestID → OrgContext → BodyLimit("/v1/orgs/")`. Cheap rejection (400 invalid headers) fires BEFORE body read.
- **Mid-stream MaxBytesError** — when the handler reads `req.Body` and gets `*http.MaxBytesError`, it maps to 413 via:
  ```go
  var maxBytesErr *http.MaxBytesError
  if errors.As(decodeErr, &maxBytesErr) { return 413 }
  ```

---

### `services/api/internal/middleware/bodylimit_test.go` (NEW unit test)

**Analog:** `services/api/internal/middleware/uuidv7path_test.go` (5.4KB) — same shape: chi router + httptest.NewServer + table-driven assertions.

**Test cases to cover** (RESEARCH §Validation Architecture table):

| Test | Expected | Reference |
|------|----------|-----------|
| `TestBodyLimit_ContentLengthOverMax_413_BeforeBodyRead` | 413; body never read | D5-21 pre-flight |
| `TestBodyLimit_LyingContentLength_413_MidStream` | 413 with *MaxBytesError | Pitfall 5 |
| `TestBodyLimit_PathMismatch_Passthrough` | upstream handler runs | path-scoped invariant |
| `TestBodyLimit_AtExactlyMaxBytes_Passes` | 200 | boundary case |
| `TestBodyLimit_OverMaxBytesByOneByte_413` | 413 | boundary case |

---

### `services/api/internal/imports/handlers.go` (NEW — package scaffold + Server type + lifecycle)

**Analog:** `services/api/internal/state/handlers.go` (164 lines — verbatim template).

**CRITICAL NAMING:** Per RESEARCH §F3 Open Q5, the struct CANNOT be named `Handlers` (conflict with `catalog.Handlers`) NOR `Server` (conflict with `state.Server`). **Recommendation: `imports.Importer`** — verb-noun, matches `state.Server` semantic of "owns a long-running responsibility." Composite literal becomes:

```go
type ApiHandlers struct {
    *catalog.Handlers
    *state.Server
    *imports.Importer  // NEW; field name auto-derived from type name → unique
}
```

**Reference pattern (verbatim from state/handlers.go lines 1-164):**

```go
package imports

import (
    "context"
    "fmt"
    "log/slog"
    "sync"
    "time"

    "github.com/jonboulle/clockwork"

    "github.com/luongdev/open-routing/services/api/internal/cache"
    "github.com/luongdev/open-routing/services/api/internal/db"
)

const (
    defaultSweepInterval = 1 * time.Hour  // D5-11 recommendation
    importBodyLimit      = 50 << 20       // D5-21 50 MB
    importRowLimit       = 500            // D5-22
    chunkSize            = 50             // D5-09
)

// Deps bundles required runtime dependencies. All fields REQUIRED;
// New does NOT validate non-nil — a zero field panics at first
// dereference (D-71 hybrid-constructor contract).
type Deps struct {
    OrgDB   *db.OrgDB
    Cache   *cache.Cache
    Logger  *slog.Logger
}

type Option func(*Importer)

func WithClock(c clockwork.Clock) Option         { return func(s *Importer) { s.clock = c } }
func WithSweepInterval(d time.Duration) Option   { return func(s *Importer) { s.sweepInterval = d } }

// Importer implements the SUBSET of api.StrictServerInterface for the
// two bulk-import endpoints (BulkImportCatalog + GetImportJob). The full
// interface is satisfied by the *ApiHandlers composite in cmd/api/main.go
// (RESEARCH §F3 Open Q5 — Importer is the unambiguous third embed).
//
// Pitfall (Phase 4 lesson): Server vs Handlers naming collides with the
// existing two embedded types. Importer is the chosen disambiguator.
type Importer struct {
    deps          Deps
    clock         clockwork.Clock
    sweepInterval time.Duration

    ctx     context.Context
    cancel  context.CancelFunc
    wg      sync.WaitGroup
    started bool
    startMu sync.Mutex
}

// New constructs an *Importer. Required deps validate at first dereference;
// optional knobs apply via Option funcs. Defaults: real clock, 1h sweep
// interval (D5-11).
func New(deps Deps, opts ...Option) *Importer {
    s := &Importer{
        deps:          deps,
        clock:         clockwork.NewRealClock(),
        sweepInterval: defaultSweepInterval,
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Start is called by cmd/api/main.go before serving. Mirror Phase 4
// state.Server.Start: synchronous startup sweep (D5-11 — guarantees no
// stuck pending job survives a restart) BEFORE spawning the 1h safety-sweep
// goroutine. Idempotent via startMu.
func (s *Importer) Start(ctx context.Context) error {
    s.startMu.Lock()
    defer s.startMu.Unlock()
    if s.started {
        return nil
    }
    s.started = true
    s.ctx, s.cancel = context.WithCancel(ctx)

    if err := s.startupSweep(s.ctx); err != nil {
        s.started = false
        s.cancel()
        return fmt.Errorf("imports.Importer: startup sweep: %w", err)
    }

    s.wg.Add(1)
    go s.safetySweep()

    return nil
}

// Stop cancels the internal ctx, waits for the safety-sweep goroutine to
// drain. Mirror state.Server.Stop without the timer-registry plumbing
// (Phase 5 has no per-row AfterFunc timers — only the global sweep ticker).
// Idempotent.
func (s *Importer) Stop() {
    s.startMu.Lock()
    if !s.started {
        s.startMu.Unlock()
        return
    }
    s.started = false
    s.startMu.Unlock()

    if s.cancel != nil {
        s.cancel()
    }
    s.wg.Wait()
}
```

**Deviations from state/handlers.go analog:**

- **No `timersRegistry`** — Phase 5's sweep is global (UPDATE … WHERE updated_at < NOW() - 24h), no per-row timers.
- **No `WrapUpDuration` in Deps** — Phase 5's 24h TTL is hardcoded (matches D5-11 lock).
- **Naming:** `Importer` (not `Server`) — resolves the embed-collision per RESEARCH §F3.

---

### `services/api/internal/imports/handler_import.go` (NEW — BulkImportCatalog)

**Analog:** `services/api/internal/state/agent_states.go` `PatchAgentStatus` (lines 107-346) for the orgID-extract + body-validate + tx flow. Also `services/api/internal/catalog/agents.go` `CreateAgent` (lines 71-207) for the qtx-and-cache-del pattern.

**Top-level dispatch flow (verbatim from RESEARCH §Pattern 1 + Architecture Diagram):**

```go
package imports

import (
    "context"
    "errors"
    "net/http"

    "github.com/google/uuid"

    "github.com/luongdev/open-routing/services/api/internal/api"
    "github.com/luongdev/open-routing/services/api/internal/db/generated"
    "github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// BulkImportCatalog — IMP-* synchronous import endpoint. Dispatch by
// ?entity= (typed enum from oapi-codegen) and Content-Type (JSON via
// req.JSONBody, CSV via req.Body io.Reader). 6-step flow per RESEARCH
// §Pattern 1 + §Architecture Diagram.
func (s *Importer) BulkImportCatalog(ctx context.Context, req api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        return api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
        }}, nil
    }

    // STEP 1: Validate ?entity= (typed enum from oapi-codegen — Valid() method).
    if !req.Params.Entity.Valid() {
        return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
            Error: api.ErrorCodeInvalidBody, Reason: "unsupported_entity",
        }), nil
    }

    // STEP 2: Idempotency-Key replay BEFORE any work (D5-13).
    if req.Params.IdempotencyKey != nil {
        if replay, found, err := s.lookupIdempotentReplay(ctx, orgID, *req.Params.IdempotencyKey); err != nil {
            s.deps.Logger.ErrorContext(ctx, "import.idempotency.lookup", "err", err)
            return api.BulkImportCatalog500JSONResponse{...}, nil
        } else if found {
            return replay, nil
        }
    }

    // STEP 3: Dispatch parser by which body field oapi-codegen populated.
    //   - req.JSONBody != nil  → JSON (eager-decoded per §F1; memory bounded by D5-21)
    //   - req.Body != nil      → CSV io.Reader (still MaxBytes-wrapped)
    //   - else                  → 415 (strict-server should auto-reject; defensive 400 here)
    switch {
    case req.JSONBody != nil:
        return s.importJSON(ctx, orgID, req.Params.Entity, *req.JSONBody, req)
    case req.Body != nil:
        if req.Params.SchemaVersion == nil || *req.Params.SchemaVersion != "v0.1" {
            return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
                Error:  api.ErrorCodeInvalidBody,
                Reason: "unsupported_schema_version_supported_versions=v0.1",
            }), nil
        }
        return s.importCSV(ctx, orgID, req.Params.Entity, req.Body, req)
    default:
        return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
            Error: api.ErrorCodeInvalidBody, Reason: "unsupported_content_type",
        }), nil
    }
}
```

**Inner `processRows` orchestrator (chunked savepoint per D5-09):**

```go
// processRows runs the chunked-savepoint pipeline over a row iterator.
// Used by BOTH importJSON and importCSV via a common callback shape so
// the chunk loop doesn't care whether the source was JSON or CSV.
//
// Mirror catalog/agents.go:114-197 tx pattern (outer Begin + defer
// Rollback + per-row qtx). Phase 5 adds per-row SAVEPOINT via the new
// (*OrgTx).BeginSavepoint method.
func (s *Importer) processRows(ctx context.Context, orgID uuid.UUID, entity api.ImportEntityType, rowCount int, rowAt func(idx int) (parsedRow, error)) (api.BulkImportCatalogResponseObject, error) {
    // ... per RESEARCH §Pattern 5 chunk loop body lines 632-707 ...
}
```

**Pattern hazards (Anti-Patterns from RESEARCH §Pattern 5 + Pitfall 4):**

- **DO NOT call `generated.New(s.deps.OrgDB)` inside the chunk loop** — every per-row query MUST go through the savepoint Tx (`generated.New(savepointTx)`). Mirror `catalog/agents.go:124` (`qtx := generated.New(tx)`).
- **DO NOT call `cache.Del` per-row inside the savepoint** — buffer successful UUIDs in a local `succeededInChunk` slice; call `cache.Del` ONLY after `outerTx.Commit()` succeeds. Mirror Phase 3's post-commit pattern (`catalog/agents.go:202`).
- **DO NOT mint entity UUIDs ahead of UpsertXByCode** — the supplied `$1=id` is the new-row UUID; ON CONFLICT path discards it and returns the existing row's id. `import_jobs.id` is the only thing minted via `uuid.NewV7()` at handler entry.

---

### `services/api/internal/imports/handler_get_job.go` (NEW — GetImportJob)

**Analog:** `services/api/internal/state/agent_states.go` `GetAgentStatus` (lines 40-99) — same single-row lookup + 404 mapping, but NO cache (import_jobs results are read-once after completion; no GetOrSet wrap).

```go
// GetImportJob — IMP-06 read-side. No cache (vs catalog detail GETs)
// because import_jobs is monotonic event data and reads are rare.
// FOUND-08 isolation: cross-org probe returns pgx.ErrNoRows → 404.
func (s *Importer) GetImportJob(ctx context.Context, req api.GetImportJobRequestObject) (api.GetImportJobResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        return api.GetImportJob500JSONResponse{...}, nil
    }

    jobID := uuid.UUID(req.Id)
    q := generated.New(s.deps.OrgDB)
    row, err := q.GetImportJob(ctx, generated.GetImportJobParams{
        ID:    pgUUID(jobID),
        OrgID: pgUUID(orgID),
    })
    if errors.Is(err, pgx.ErrNoRows) {
        return api.GetImportJob404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{
            Error: api.ErrorCodeNotFound, Reason: "import_job_not_found",
        }}, nil
    }
    if err != nil {
        s.deps.Logger.ErrorContext(ctx, "get import job", "err", err)
        return api.GetImportJob500JSONResponse{...}, nil
    }
    return api.GetImportJob200JSONResponse(mapImportJob(row)), nil
}
```

**Pattern hazard:** No `enabledCheck` like state/agent_states.go — import_jobs has no soft-delete; the only filter is `(id, org_id)`. FOUND-08 holds because the WHERE clause is composite-org-scoped.

---

### `services/api/internal/imports/coerce.go` (NEW — typed pipeline)

**Analog:** **None in-tree** — pure-new code per RESEARCH §Pattern 1 (D5-01..D5-08). Structural cousin: `services/api/internal/catalog/codecheck.go` (47 lines of pure functions with regex/string validators). Phase 5's `coerce.go` follows the same shape: package-private helpers + table-driven unit tests.

**Reference structural shape from `codecheck.go` (verbatim file header for the new `coerce.go` to mirror):**

```go
// codecheck.go — Layer 1 + Layer 2 validation helpers ...
package catalog

import "regexp"

var codeFormat = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func validateCodeFormat(s string) bool {
    return codeFormat.MatchString(s)
}
```

**Phase 5 `coerce.go` signatures (per D5-01..D5-08):**

```go
// coerce.go — CSV cell + JSON value type-coercion pipeline (D5-01..D5-08).
// Every cell goes through trim → lower → split → parse before validation.
// One central dispatcher; per-type coercer; per-entity validator runs on
// post-coercion typed values.
//
// Why this file exists:
//   - User constraint (CONTEXT.md <specifics>): "khi biết kiểu trong db thì
//     nên quyết định trước là có trim, lower, split, ... trước không rồi mới
//     process tiếp. Như vậy đỡ được khối lỗi."
//   - Shrinks the "weird input" surface so per-entity validators only see
//     well-shaped data and focus on business rules.
//
// Conceptual analog: services/api/internal/catalog/codecheck.go (pure
// validation helpers — same shape, different domain).
package imports

import (
    "errors"
    "math"
    "strconv"
    "strings"
    "unicode/utf8"
)

// ErrCoerce... sentinels — one per D5 decision so handler maps to the
// exact per-row reason string.
var (
    ErrInvalidBool         = errors.New("invalid_bool")          // D5-02
    ErrMissingRequired     = errors.New("missing_required")      // D5-03
    ErrInvalidEnum         = errors.New("invalid_enum")          // D5-06
    ErrInvalidInt          = errors.New("invalid_int")           // D5-07
    ErrCSVNotUTF8          = errors.New("csv_not_utf8")          // D5-08
)

// trimLower applies the universal D5-01 prefix transform (trim → lower).
// Returned string is empty when the input was blank (D5-03 — empty cells = null/skip).
func trimLower(s string) string {
    return strings.ToLower(strings.TrimSpace(s))
}

// parseBool applies D5-02 loose bool coercion. Accepts
//   true|false|TRUE|FALSE|1|0|yes|no (case-insensitive).
// Any other value → ErrInvalidBool.
func parseBool(raw string) (bool, error) {
    s := trimLower(raw)
    switch s {
    case "true", "1", "yes":
        return true, nil
    case "false", "0", "no":
        return false, nil
    default:
        return false, ErrInvalidBool
    }
}

// parseInt applies D5-07 tolerant int parsing. Pipeline:
//   trim → strconv.ParseFloat → math.Trunc → int.
// "7", " 7 ", "7.0", "7.5" all coerce to 7 (truncation, never rounding).
// "seven" → ErrInvalidInt.
//
// Emits slog.Debug("import: int_truncation", "raw", raw, "value", n)
// when a non-zero fractional part is discarded (specifics line "tolerant
// int parsing for Excel"); caller passes (row, field) for forensics.
func parseInt(raw string) (int, bool, error) {
    s := strings.TrimSpace(raw)
    f, err := strconv.ParseFloat(s, 64)
    if err != nil {
        return 0, false, ErrInvalidInt
    }
    truncated := math.Trunc(f)
    return int(truncated), truncated != f, nil
}

// splitMulti applies D5-04 multi-value separator priority (| > ; > ,).
// CSV field-level , unescaping happens first (RFC 4180); this function
// operates on the unescaped cell value.
func splitMulti(s string) []string {
    if strings.Contains(s, "|") {
        return splitTrim(s, "|")
    }
    if strings.Contains(s, ";") {
        return splitTrim(s, ";")
    }
    return splitTrim(s, ",")
}

// validateEnum applies D5-06 case-insensitive enum match. After trimLower,
// compare against the supplied closed set. No-match → ErrInvalidEnum.
func validateEnum(raw string, allowed []string) (string, error) { ... }

// validateUTF8 applies D5-08 strict UTF-8 enforcement after BOM strip.
// Returns ErrCSVNotUTF8 on invalid sequences.
func validateUTF8(b []byte) error {
    if !utf8.Valid(b) {
        return ErrCSVNotUTF8
    }
    return nil
}
```

**Pattern hazard:** The pipeline functions return sentinel errors (NOT `*RowError` — that's the orchestrator's job). The chunk loop wraps `err` into `BulkImportFailedRow{Field: X, Reason: err.Error()}` after catching it.

---

### `services/api/internal/imports/parser_csv.go` (NEW — BOM strip + utf8 validate + csv.Reader factory)

**Analog:** None in-tree (RESEARCH §Pattern 3 — verbatim). Structural cousin: `services/api/internal/catalog/cursor.go` (97-line pure-utility file with package-init compile + a paired unit test). Phase 5's `parser_csv.go` follows the same shape.

**Reference verbatim from RESEARCH §Pattern 3 lines 524-562:**

```go
// services/api/internal/imports/parser_csv.go (NEW)
package imports

import (
    "bufio"
    "encoding/csv"
    "errors"
    "io"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// stripBOM peeks 3 bytes and consumes them iff they match the BOM.
// Peek is non-destructive when no BOM is present
// [CITED: pkg.go.dev/bufio#Reader.Peek].
//
// Pitfall 1 (RESEARCH §Pitfall 1): Excel-on-Windows saves CSV with a
// UTF-8 BOM. Without stripping it, the first cell of the header row
// reads as "\xEF\xBB\xBFcode" and header validation fails with a
// confusing "missing required column code" error.
func stripBOM(r io.Reader) (*bufio.Reader, error) {
    br := bufio.NewReader(r)
    head, err := br.Peek(3)
    if err != nil && !errors.Is(err, io.EOF) {
        return nil, err
    }
    if len(head) == 3 && head[0] == utf8BOM[0] && head[1] == utf8BOM[1] && head[2] == utf8BOM[2] {
        _, _ = br.Discard(3)
    }
    return br, nil
}

// newCSVReader returns a csv.Reader configured for D5-* compliance:
//   - LazyQuotes=false     (strict RFC 4180)
//   - FieldsPerRecord=0    (latches to first row's column count)
//   - TrimLeadingSpace=false  (coerce.go owns trimming)
// csv.Reader auto-converts CRLF → LF on output [CITED: pkg.go.dev/encoding/csv].
func newCSVReader(r io.Reader) *csv.Reader {
    cr := csv.NewReader(r)
    cr.LazyQuotes = false
    cr.FieldsPerRecord = 0
    cr.TrimLeadingSpace = false
    return cr
}
```

**Pattern hazard:** `bufio.Reader.Peek(3)` is non-destructive on `*http.MaxBytesReader`-wrapped readers (RESEARCH A4 verified). Phase 5 calls `stripBOM(req.Body)` AFTER the middleware has already wrapped the body.

---

### `services/api/internal/imports/parser_json.go` (NEW — re-marshal helper)

**Analog:** **None in-tree** — pure-new per RESEARCH §F1 accommodation. The strict-server eagerly decodes `[]interface{}`; Phase 5 re-marshals each element into the right typed `Import*Request` via marshal-then-unmarshal.

**Verbatim from RESEARCH §Pattern 4 lines 586-597:**

```go
// parser_json.go — re-marshal helper for the F1-accommodation pattern.
// The strict-server has already eager-decoded req.JSONBody to []interface{}
// by the time the handler runs (§F1). Per-row, we marshal the interface{}
// element back to bytes and unmarshal into the typed struct. Per-row
// failures are isolated to that row.
package imports

import (
    "bytes"
    "encoding/json"
    "fmt"
)

// reMarshalAs is the type-parameterized round-trip helper. Usage:
//   typed, err := reMarshalAs[api.ImportAgentRequest](rawItem)
// On per-row decode error, return as a rowError to the chunk loop.
func reMarshalAs[T any](item interface{}) (T, error) {
    var zero T
    raw, err := json.Marshal(item)
    if err != nil {
        return zero, fmt.Errorf("re-marshal: %w", err)
    }
    var out T
    if err := json.Unmarshal(raw, &out); err != nil {
        return zero, fmt.Errorf("re-unmarshal: %w", err)
    }
    return out, nil
}
```

**Pattern hazard:** Generic type parameter `T` requires Go 1.18+ (verified — Phase 1 D-19 carries-forward Go 1.25). The instantiation `reMarshalAs[api.ImportAgentRequest](rawItem)` happens once per row.

---

### `services/api/internal/imports/header.go` (NEW — per-entity column registry)

**Analog:** `services/api/internal/state/transitions.go` (matrix-as-data pattern). Phase 5's `header.go` similarly encodes a per-entity registry of required + optional column names + typed-column dispatch.

**Reference shape from `state/transitions.go`** (matrix encoding):

```go
// transitions.go (verbatim from state package)
type TransitionRule struct {
    RequiresBreakReasonID bool
    AgentInitiated        bool
}

var matrix = map[api.AgentStatus]map[api.AgentStatus]TransitionRule{
    api.AgentStatusNotReady: { api.AgentStatusReady: {AgentInitiated: true} },
    api.AgentStatusReady:    { api.AgentStatusNotReady: {AgentInitiated: true}, ... },
    // ...
}
```

**Phase 5 `header.go` shape (D5-05 strict header policy, per-entity registry):**

```go
// header.go — per-entity column registry (D5-05). Required + optional
// column names + dispatch to typed coercion. Strict header policy: missing
// required → 400 missing_columns; unknown → 400 unknown_columns. Header
// validation fires BEFORE the first data row is read.
package imports

import "github.com/luongdev/open-routing/services/api/internal/api"

// columnSpec describes one column of an entity's CSV import shape.
type columnSpec struct {
    name     string  // CSV header text
    required bool
    coerce   func(string) (any, error)  // dispatches to coerce.go
}

// entityRegistry is the source of truth for what columns each entity
// accepts on CSV import. Mirror state/transitions.go matrix pattern.
var entityRegistry = map[api.ImportEntityType][]columnSpec{
    api.Agents: {
        {name: "code", required: true, coerce: coerceCode},
        {name: "external_id", required: false, coerce: coerceString},
        {name: "name", required: true, coerce: coerceString},
        {name: "email", required: true, coerce: coerceEmail},
        {name: "enabled", required: false, coerce: coerceBool},
        {name: "skills", required: false, coerce: coerceSkillsCell},  // "code:prof|code:prof"
    },
    api.Skills: { ... },
    api.Queues: { ... },
    // ... 6 entities total
}

// validateHeader is the D5-05 strict batch-level check. Returns
// (missing, unknown) sets; either non-empty → 400 with the appropriate reason.
func validateHeader(entity api.ImportEntityType, header []string) (missing, unknown []string) { ... }
```

**Pattern hazard:** The dispatch function `coerceSkillsCell` for agents handles the `"code:prof|code:prof"` syntax (D5-17). Other entities' multi-value cells use the same `splitMulti` from coerce.go.

---

### `services/api/internal/imports/row_agent.go` (NEW — per-entity row processor)

**Analog:** `services/api/internal/catalog/agents.go` `CreateAgent` (lines 71-207) — the canonical "Layer 1 → qtx → UpsertByCode → InsertAgentState → cache.Del" flow. Phase 5's row processor inverts Create's outer-tx ownership (the chunk loop owns the outer tx; the row processor receives the savepoint tx).

**Reference qtx + 23505 + agent_state pattern (catalog/agents.go lines 124-174):**

```go
qtx := generated.New(tx)
row, err := qtx.InsertAgent(ctx, generated.InsertAgentParams{
    ID:         pgUUID(id),
    OrgID:      pgUUID(orgID),
    Code:       req.Body.Code,
    ExternalID: req.Body.ExternalId,
    Name:       req.Body.Name,
    Email:      string(req.Body.Email),
    Enabled:    enabled,
})
if err != nil {
    status, code, reason := mapPgError(err, "agent")
    switch status {
    case 409:
        var body api.CreateAgent409JSONResponseBody
        _ = body.FromErrorResponse(api.ErrorResponse{Error: code, Reason: reason})
        return api.CreateAgent409JSONResponse(body), nil
    case 422:
        return api.CreateAgent422JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
    }
    h.deps.Logger.ErrorContext(ctx, "create agent insert", "err", err)
    return api.CreateAgent500JSONResponse{...}, nil
}

// D-93: seed agent_states row inside the SAME tx.
if _, sErr := qtx.InsertAgentState(ctx, generated.InsertAgentStateParams{
    AgentID: pgUUID(id),
    OrgID:   pgUUID(orgID),
    Status:  string(api.AgentStatusOffline),
}); sErr != nil {
    return api.CreateAgent500JSONResponse{...}, nil
}
```

**Phase 5 verbatim from RESEARCH §Pattern 6 lines 715-815** (key deltas):

- Replace `qtx.InsertAgent` → `qtx.UpsertAgentByCode` (Phase 04.1 query, ON CONFLICT (org_id, code) DO UPDATE).
- Replace `qtx.InsertAgentState` → `qtx.InsertAgentStateOnConflictNothing` (Phase 5 new query — re-imports MUST NOT regress state).
- Add nested skill_code validation via `validateCodeFormat` from `catalog/codecheck.go:33` (REUSED) per Pitfall 9.
- Call `qtx.MergeAgentSkill` (Phase 5 new query) for each skill in the import payload — D5-18 PATCH-like merge, NOT DELETE-ALL like `replaceAgentSkills` (RESEARCH Pitfall 10).
- Wrap any 409/422/500 verdict from `mapPgError` into a `*rowError` (chunk loop converts to `BulkImportFailedRow`).

**Critical reuse note:** Phase 04.1's `validateCodeFormat` lives in package `catalog`. Phase 5's `row_agent.go` is in package `imports`. To reuse: either (a) export Phase 04.1's helper (`ValidateCodeFormat` with capital V — package-API change) OR (b) duplicate the regex constant in `imports/coerce.go` (simpler). Recommendation: **Option (a) export** — RESEARCH §A11/A15 lists this as a known reuse case. Plan task: rename `catalog.validateCodeFormat` → `catalog.ValidateCodeFormat` and update the 6 catalog Create/Update handlers' call sites accordingly (mechanical s/validateCodeFormat/ValidateCodeFormat/g).

---

### `services/api/internal/imports/row_skill.go`, `row_queue.go`, `row_channel.go`, `row_adapter.go`, `row_break_reason.go` (NEW × 5 — per-entity processors)

**Analog:** `services/api/internal/catalog/<entity>.go` `Create<Entity>` for each (Phase 3 + 04.1 templates verified). All 5 follow the same pattern:

1. `reMarshalAs[api.Import<Entity>Request](r.raw)` — typed decode.
2. `validateCodeFormat(typed.Code)` — Layer 1 (REUSED from catalog package).
3. (channels only) Layer 1 code-format validation on `default_queue_code` + D-76 FK code-lookup probe via Phase 04.1's `GetQueueByCode`.
4. `qtx := generated.New(sp)` — bind to savepoint Tx.
5. `qtx.Upsert<Entity>ByCode(ctx, params)` — Phase 04.1 query.
6. On error: `status, code, reason := mapPgError(err, "<entity>")` — REUSED from `catalog/errors.go:48`.
7. Return `succeededRow{id: row.ID, entity: <type>, lineNo: r.lineNo}` OR `*rowError{Field, Reason}`.

**Per-entity nuances:**

| Entity | Special |
|--------|---------|
| `skill` | flat (no nested arrays); flat 409 wrapper (mirror `Create<Skill>409JSONResponse` shape) |
| `queue` | `channel_types TEXT[]` needs cell-level split per D5-04; flat 409 |
| `channel` | FK code-lookup probe for `default_queue_code` per D-76; flat 409 |
| `adapter` | JSONB `config` passthrough (no validation); NEW 409 path per Phase 04.1 D04_1-13 (`Create<Adapter>409` was added in Phase 04.1) |
| `break_reason` | flat (no nested); flat 409 (Phase 04.1 dropped `UNIQUE(org_id, name)` → 409 now keyed on `code` only) |

**Pattern hazard:** Phase 04.1 already authored all 6 `Upsert<Entity>ByCode` queries. Phase 5 invokes them — does NOT author parallel queries. Verified line citations in RESEARCH §Sources:
- `services/api/internal/db/queries/agents.sql:127`
- `services/api/internal/db/queries/skills.sql:101`
- `services/api/internal/db/queries/queues.sql` (per equivalent)
- `services/api/internal/db/queries/channels.sql` (per equivalent)
- `services/api/internal/db/queries/adapters.sql:92`
- `services/api/internal/db/queries/break_reasons.sql:113`

---

### `services/api/internal/imports/chunk.go` (NEW — chunked savepoint orchestrator)

**Analog:** `services/api/internal/catalog/agents.go` `CreateAgent` tx pattern (lines 114-197) — adapted to per-row savepoint via the new `(*OrgTx).BeginSavepoint` method.

**Verbatim from RESEARCH §Pattern 5 lines 633-706** (key invariants):

```go
package imports

import (
    "context"

    "github.com/google/uuid"

    "github.com/luongdev/open-routing/services/api/internal/api"
    "github.com/luongdev/open-routing/services/api/internal/cache"
)

func (s *Importer) processChunk(ctx context.Context, orgID uuid.UUID, rowProc rowProcessor, rows []parsedRow) (succeeded []succeededRow, failed []api.BulkImportFailedRow) {
    outerTx, err := s.deps.OrgDB.BeginTx(ctx)
    if err != nil {
        for _, r := range rows {
            failed = append(failed, api.BulkImportFailedRow{
                Row: r.lineNo, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: "tx_begin_failed",
            })
        }
        return
    }
    defer func() { _ = outerTx.Rollback(ctx) }()  // safe after commit (pgx ignores)

    // Resolve skill codes ONCE for the chunk (D5-16 / D5-17 batch optimization).
    // Mirror catalog/agent_skills.go:88 SkillsPresentInOrg batch pattern.
    skillResolver, err := s.resolveChunkSkillCodes(ctx, outerTx, orgID, rows)
    if err != nil { /* mark whole chunk failed */ return }

    for _, r := range rows {
        sp, spErr := outerTx.BeginSavepoint(ctx)
        if spErr != nil {
            failed = append(failed, api.BulkImportFailedRow{
                Row: r.lineNo, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: "savepoint_begin_failed",
            })
            continue
        }
        out, procErr := rowProc.process(ctx, sp, orgID, r, skillResolver)
        if procErr != nil {
            _ = sp.Rollback(ctx)  // ROLLBACK TO savepoint — per-row failure
            failed = append(failed, api.BulkImportFailedRow{
                Row: r.lineNo, Field: procErr.Field, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: procErr.Reason,
            })
            continue
        }
        if err := sp.Commit(ctx); err != nil {  // RELEASE savepoint — per-row success
            failed = append(failed, api.BulkImportFailedRow{...})
            continue
        }
        succeeded = append(succeeded, out)
    }

    if err := outerTx.Commit(ctx); err != nil {
        // Chunk commit failure — all "succeeded" rows in this chunk are lost.
        // Transfer them into failed[] with reason "chunk_commit_failed".
        for _, sr := range succeeded {
            failed = append(failed, api.BulkImportFailedRow{
                Row: sr.lineNo, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: "chunk_commit_failed",
            })
        }
        return nil, failed
    }

    // POST-COMMIT cache invalidation. ALWAYS after outerTx.Commit succeeds
    // (mirror catalog/agents.go:202 pattern; RESEARCH Pitfall 4).
    for _, sr := range succeeded {
        cacheKey := cache.Key(orgID, string(sr.entity), sr.id)
        if delErr := s.deps.Cache.Del(ctx, cacheKey); delErr != nil {
            s.deps.Logger.WarnContext(ctx, "import.cache_del_failed",
                "key", cacheKey, "err", delErr)
        }
    }
    return succeeded, failed
}
```

**Anti-patterns to enforce in code review (RESEARCH §Anti-Patterns lines 1060-1071):**

- DO NOT call `generated.New(s.deps.OrgDB)` inside the chunk loop.
- DO NOT issue per-row skill code lookups (batch once via `ResolveSkillCodes`).
- DO NOT call `cache.Del` per-row inside the chunk loop.
- DO NOT mint UUIDv7 for entity rows ahead of `UpsertXByCode`.
- DO NOT skip `InsertAgentStateOnConflictNothing` for new agents.
- DO NOT call `replaceAgentSkills` (use `MergeAgentSkill`).

---

### `services/api/internal/imports/jobs.go` (NEW — import_jobs lifecycle)

**Analog:** `services/api/internal/catalog/agents.go` `UpdateAgent` 0-row disambiguate pattern (lines 384-440) for the "INSERT row → do work → UPDATE row" shape. Job INSERT is its own short tx (D5-10) so admin GET can see the job even mid-import.

**Phase 5 shape:**

```go
// jobs.go — import_jobs lifecycle (D5-10). Each handler call:
//   1. createJob: INSERT status='pending' in its own short tx BEFORE chunk 1
//   2. process chunks (chunk.go owns)
//   3. finaliseJob: UPDATE counters + errors JSONB + status='completed'|'failed'

func (s *Importer) createJob(ctx context.Context, orgID uuid.UUID, entity api.ImportEntityType, totalRows int, idempotencyKey *string) (uuid.UUID, error) {
    id := uuid.Must(uuid.NewV7())
    q := generated.New(s.deps.OrgDB)
    _, err := q.InsertImportJob(ctx, generated.InsertImportJobParams{
        ID:             pgUUID(id),
        OrgID:          pgUUID(orgID),
        EntityType:     string(entity),
        TotalRows:      int32(totalRows),
        IdempotencyKey: idempotencyKey,  // sqlc.narg → *string
    })
    return id, err
}

func (s *Importer) finaliseJob(ctx context.Context, id, orgID uuid.UUID, status string, succeeded, failed int, errorsJSON []byte) error {
    q := generated.New(s.deps.OrgDB)
    _, err := q.FinaliseImportJob(ctx, generated.FinaliseImportJobParams{
        Status:         status,
        SucceededRows:  int32(succeeded),
        FailedRows:     int32(failed),
        Errors:         errorsJSON,  // []byte → JSONB
        ID:             pgUUID(id),
        OrgID:          pgUUID(orgID),
    })
    return err
}
```

**Pattern hazard:** `errors` JSONB shape MUST mirror `BulkImportResult.failed[]` exactly (D5-12). Serialize via `json.Marshal([]api.BulkImportFailedRow)` before passing to `FinaliseImportJob`. No timestamps, no raw_value, no attempt count.

---

### `services/api/internal/imports/idempotency.go` (NEW — replay lookup)

**Analog:** `services/api/internal/state/agent_states.go` `GetAgentStatus` single-row read pattern (lines 40-99). Phase 5 reuses the lookup shape but no cache (idempotency lookups are rare).

**Verbatim from RESEARCH §Pattern 10 lines 1029-1056:**

```go
// idempotency.go — D5-13 replay lookup. Hit returns rehydrated
// BulkImportResult with idempotent_replay=true.
func (s *Importer) lookupIdempotentReplay(ctx context.Context, orgID uuid.UUID, key openapi_types.UUID) (api.BulkImportCatalogResponseObject, bool, error) {
    q := generated.New(s.deps.OrgDB)
    prior, err := q.LookupImportJobByIdempotencyKey(ctx, generated.LookupImportJobByIdempotencyKeyParams{
        OrgID:          pgUUID(orgID),
        IdempotencyKey: key.String(),  // sqlc emits as string column
    })
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, false, nil
    }
    if err != nil {
        return nil, false, err
    }
    return rehydrateBulkImportResult(prior, true), true, nil
}

// rehydrateBulkImportResult builds a 200 response from a persisted job.
// D5-13 KNOWN LIMITATION: succeeded[] cannot be reconstructed from counters
// (only total_rows, succeeded_rows, failed_rows + errors JSONB are stored).
// Returns succeeded: [], idempotent_replay: true so callers detect replay.
func rehydrateBulkImportResult(row generated.ImportJob, replay bool) api.BulkImportCatalog200JSONResponse {
    var failed []api.BulkImportFailedRow
    if row.Errors.Valid {
        _ = json.Unmarshal(row.Errors.Bytes, &failed)
    }
    return api.BulkImportCatalog200JSONResponse(api.BulkImportResult{
        Succeeded:        []api.UUIDv7{},
        Failed:           failed,
        IdempotentReplay: &replay,
    })
}
```

**Pattern hazard:** D5-13 KNOWN LIMITATION is intentional. Engineer impulse to "fix" by adding `succeeded_ids JSONB` at the last minute is the RESEARCH Pitfall 7 warning. Deferred to v0.2.

---

### `services/api/internal/imports/sweep.go` (NEW — crash-recovery goroutine)

**Analog:** `services/api/internal/state/ttl.go` `safetySweep` + `runSweepPastDue` + `startupSweep` (lines 132-196) — verbatim Phase 4 STATE-07 pattern. The Phase 5 sweep is structurally identical but operates on `import_jobs` instead of `agent_states`.

**Reference verbatim Phase 4 pattern** (state/ttl.go lines 156-196):

```go
// safetySweep runs on a clock.NewTicker cadence ...
func (s *Server) safetySweep() {
    defer s.wg.Done()
    ticker := s.clock.NewTicker(s.sweepInterval)
    defer ticker.Stop()
    for {
        select {
        case <-s.ctx.Done():
            return
        case <-ticker.Chan():
            s.runSweepPastDue()
        }
    }
}

func (s *Server) runSweepPastDue() {
    ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
    defer cancel()
    ctx = db.WithBypass(ctx, "wrapup_sweeper.safety")
    q := generated.New(s.deps.OrgDB)
    rows, err := q.ListExpiringWrapUps(ctx)
    // ...
}
```

**Phase 5 verbatim adaptation (RESEARCH §Pattern 9 lines 976-1009):**

```go
// sweep.go — D5-11 crash-recovery sweep. Mirror state/ttl.go STATE-07
// pattern. clockwork.Clock injection → testable via FakeClock.

func (s *Importer) safetySweep() {
    defer s.wg.Done()
    ticker := s.clock.NewTicker(s.sweepInterval)
    defer ticker.Stop()
    for {
        select {
        case <-s.ctx.Done():
            return
        case <-ticker.Chan():
            s.runSweepPastDue()
        }
    }
}

func (s *Importer) runSweepPastDue() {
    ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
    defer cancel()
    ctx = db.WithBypass(ctx, "import_crash_sweep")  // mirror "wrapup_sweeper.safety"

    q := generated.New(s.deps.OrgDB)
    rows, err := q.SweepCrashedImportJobs(ctx)
    if err != nil {
        s.deps.Logger.WarnContext(ctx, "import.crash_sweep.failed", "err", err)
        return
    }
    for _, r := range rows {
        s.deps.Logger.WarnContext(ctx, "import.crash_sweep",
            "event", "import_crash_sweep",          // D5-11 locked event name
            "import_job_id", uuid.UUID(r.ID.Bytes),
            "org_id", uuid.UUID(r.OrgID.Bytes),
        )
    }
}

// startupSweep — D5-11 synchronous startup sweep, mirror state/ttl.go:132.
// Called from Start(ctx) BEFORE the safety-sweep goroutine spawns so a
// crash window's pending rows flip to failed IMMEDIATELY on process boot.
func (s *Importer) startupSweep(ctx context.Context) error {
    ctx = db.WithBypass(ctx, "import_crash_sweep.startup")  // distinct event for audit
    q := generated.New(s.deps.OrgDB)
    rows, err := q.SweepCrashedImportJobs(ctx)
    if err != nil {
        return fmt.Errorf("import startup sweep: %w", err)
    }
    for _, r := range rows {
        s.deps.Logger.WarnContext(ctx, "import.crash_sweep.startup", "import_job_id", uuid.UUID(r.ID.Bytes), "org_id", uuid.UUID(r.OrgID.Bytes))
    }
    return nil
}
```

**Pattern hazards (verbatim from state/ttl.go + Phase 4 PATTERNS Pitfall 2 / 8):**

- **`db.WithBypass(ctx, "import_crash_sweep")`** — SQLChecker (D-04) requires either an org_id in WHERE or a bypass marker. The org-agnostic sweep MUST use bypass; reason string `"import_crash_sweep"` is locked (D5-11).
- **Single-replica only** — STATE.md blocker for both Phase 4 WrapUp and Phase 5 import crash sweep. v1 multi-replica needs distributed lock (Postgres advisory lock or Redis SETNX). Phase 5 inherits the existing blocker; no new mitigation.
- **Synchronous startup sweep BEFORE ticker spawn** — Phase 4 D-95 pattern (mirror `state/ttl.go:132 startupSweep`). Reason: a crash during import leaves a pending row that should flip to failed IMMEDIATELY on next process boot, not 1 hour later.
- **`time.Ticker` not `time.Tick`** — explicit Stop() in defer (clockwork's Ticker preserves the signature).

---

### `services/api/internal/imports/errors.go` (NEW — per-row error mapping)

**Analog:** `services/api/internal/catalog/errors.go` (93 lines) — REUSED (no duplication). Phase 5 imports `catalog.MapPgError` (export required; rename pass) OR keeps the helper in `catalog` package and call across packages.

**Phase 5 file content (thin wrapper):**

```go
// errors.go — per-row error wrapping. The constraint-name introspection
// (catalog/errors.go:48 mapPgError) is REUSED; Phase 5 wraps the verdict
// into the row-level shape expected by BulkImportFailedRow.
package imports

import (
    "github.com/luongdev/open-routing/services/api/internal/catalog"
)

// rowError is the chunk loop's per-row failure shape. Field is empty
// string when the error is row-level (e.g. tx failure), non-empty when
// field-level (e.g. "skills[2].skill_code").
type rowError struct {
    Field  string
    Reason string
}

// wrapPgError converts the catalog mapPgError triple into a rowError.
// Drops the HTTP status (Phase 5 is row-level; no HTTP per row).
func wrapPgError(err error, entity string) *rowError {
    _, code, reason := catalog.MapPgError(err, entity)  // exported in 04.1 amend
    return &rowError{
        Field:  "",
        Reason: string(code) + ":" + reason,
    }
}
```

**Reuse requirement:** Either export `catalog.mapPgError` → `catalog.MapPgError` (mechanical s/// across 6 catalog handlers; cheap), OR inline the same constraint-name introspection in `imports/errors.go` (duplicates the constraint-name string suffixes — Phase 04.1 D04_1-21 — and creates drift risk).

**Recommendation: export.** RESEARCH §Pattern 6 + §Don't Hand-Roll table explicitly says "reuse `mapPgError` from `services/api/internal/catalog/errors.go:48`."

---

### `services/api/internal/imports/mappers.go` (NEW — sqlc row → wire shape)

**Analog:** `services/api/internal/state/mappers.go` (1.7KB) — same shape: pure functions that convert sqlc `generated.X` row types to OpenAPI `api.X` wire types.

**Phase 5 content (one mapper):**

```go
// mappers.go — sqlc row → OpenAPI wire shape. One mapper per response type.
package imports

import (
    "encoding/json"

    "github.com/luongdev/open-routing/services/api/internal/api"
    "github.com/luongdev/open-routing/services/api/internal/db/generated"
)

func mapImportJob(row generated.ImportJob) api.ImportJob {
    var errs *[]api.BulkImportFailedRow
    if row.Errors.Valid {
        var parsed []api.BulkImportFailedRow
        if err := json.Unmarshal(row.Errors.Bytes, &parsed); err == nil {
            errs = &parsed
        }
    }
    return api.ImportJob{
        Id:            api.UUIDv7(row.ID.Bytes),
        OrgId:         api.UUIDv7(row.OrgID.Bytes),
        EntityType:    api.ImportEntityType(row.EntityType),
        Status:        api.ImportJobStatus(row.Status),
        TotalRows:     int(row.TotalRows),
        SucceededRows: int(row.SucceededRows),
        FailedRows:    int(row.FailedRows),
        Errors:        errs,
        // ... CreatedAt + UpdatedAt with pointer wrap (mirror state/mappers.go)
    }
}
```

---

### `services/api/internal/imports/handlers_test.go` (NEW — handler integration tests)

**Analog:** `services/api/internal/state/agent_states_test.go` (816 lines — verbatim template for the integration-test shape: httptest server + http.POST/GET helpers + miniredis + testcontainers postgres).

**Reference helper shape (state/agent_states_test.go has `agentStatusPath`, `patchStatus`, etc.):**

```go
// Phase 5 helpers to add to testutil_test.go:
func importPath(orgID uuid.UUID) string {
    return "/v1/orgs/" + orgID.String() + "/catalog/import"
}

func importJobPath(orgID, jobID uuid.UUID) string {
    return "/v1/orgs/" + orgID.String() + "/imports/" + jobID.String()
}

func postImportJSON(t testing.TB, th *TestImports, entity api.ImportEntityType, rows []any) (*http.Response, []byte) {
    body, _ := json.Marshal(rows)
    req := httptest.NewRequest(http.MethodPost,
        importPath(th.OrgID)+"?entity="+string(entity), bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Org-Id", th.OrgID.String())
    return th.HTTP.Client().Do(req)
}

func postImportCSV(t testing.TB, th *TestImports, entity api.ImportEntityType, csvBody []byte) (*http.Response, []byte) {
    req := httptest.NewRequest(http.MethodPost,
        importPath(th.OrgID)+"?entity="+string(entity)+"&schema_version=v0.1", bytes.NewReader(csvBody))
    req.Header.Set("Content-Type", "text/csv")
    req.Header.Set("X-Org-Id", th.OrgID.String())
    return th.HTTP.Client().Do(req)
}
```

**Coverage matrix (RESEARCH §Validation Architecture):**

| Test | Type | Cov |
|------|------|-----|
| `TestBulkImport_JSON_Agents_HappyPath_200` | integration | IMP-01 |
| `TestBulkImport_CSV_Agents_HappyPath_200` | integration | IMP-01 |
| `TestBulkImport_CSV_<5 more entities>` × 5 | integration | IMP-01 |
| `TestBulkImport_WindowsExcel_BOM_CRLF_200` | integration | IMP-02 (Pitfall 1) |
| `TestBulkImport_InvalidUTF8_400_csv_not_utf8` | integration | IMP-02 / D5-08 |
| `TestBulkImport_Idempotent_DoubleRun_NoDuplicate` | integration | IMP-03 (Phase 04.1 invariant) |
| `TestBulkImport_FailedRow_Structure_207` | integration | IMP-04 / D5-12 |
| `TestBulkImport_HTTPStatus_200_207_422` | integration | IMP-05 |
| `TestBulkImport_GetImportJob_RoundTrip` | integration | IMP-06 |
| `TestBulkImport_Oversize_50MB_413` | integration | IMP-07 |
| `TestBulkImport_501Rows_413` | integration | D5-22 |
| `TestBulkImport_SchemaVersion_Mismatch_400` | integration | IMP-08 |
| `TestBulkImport_IdempotencyKey_Replay_Returns_idempotent_replay_true` | integration | D5-13 |
| `TestBulkImport_AgentImport_NestedSkillCode_InvalidFormat_PerRow` | integration | Pitfall 9 |
| `TestBulkImport_AgentImport_SkillMerge_PreservesExisting` | integration | D5-18 / Pitfall 10 |
| `TestBulkImport_SeedsAgentStatesForNewAgents` | integration | Hazard 7 / Pitfall 3 |
| `TestBulkImport_Reimport_PreservesAgentStateMachine` | integration | Hazard 7 |

---

### `services/api/internal/imports/testutil_test.go` (NEW — shared fixtures)

**Analog:** `services/api/internal/state/testutil_test.go` (20.6KB — verbatim template). Same fixture struct, same composite-ApiHandlers wiring, same `cleanXTables` per-org DELETE (NOT global TRUNCATE — Phase 4 inheritance).

**Key adaptation from `state/testutil_test.go` (search hits confirmed lines 277-294):**

```go
// cleanStateTables removes state + catalog rows for a single org.
// Per-org DELETE instead of TRUNCATE so parallel tests with distinct
// orgIDs don't trample each other.
func cleanStateTables(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) {
    statements := []string{
        `DELETE FROM agent_states  WHERE org_id = $1`,
        `DELETE FROM agent_skills  WHERE org_id = $1`,
        `DELETE FROM break_reasons WHERE org_id = $1`,
        `DELETE FROM agents        WHERE org_id = $1`,
        // ...
    }
    // ...
}
```

**Phase 5 `cleanImportTables` (mirror; add `import_jobs` first):**

```go
func cleanImportTables(t testing.TB, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) {
    statements := []string{
        `DELETE FROM import_jobs   WHERE org_id = $1`,
        `DELETE FROM agent_states  WHERE org_id = $1`,
        `DELETE FROM agent_skills  WHERE org_id = $1`,
        `DELETE FROM break_reasons WHERE org_id = $1`,
        `DELETE FROM channels      WHERE org_id = $1`,
        `DELETE FROM queues        WHERE org_id = $1`,
        `DELETE FROM adapters      WHERE org_id = $1`,
        `DELETE FROM skills        WHERE org_id = $1`,
        `DELETE FROM agents        WHERE org_id = $1`,
    }
    for _, stmt := range statements {
        _, err := pool.Exec(ctx, stmt, orgID)
        require.NoError(t, err, "cleanImportTables: DELETE failed for %s", stmt)
    }
}
```

**Composite ApiHandlers wiring (mirror state/testutil_test.go lines 67-158):**

```go
type TestImports struct {
    H         *Importer
    Miniredis *miniredis.Miniredis
    Pool      *pgxpool.Pool
    OrgID     uuid.UUID
    HTTP      *httptest.Server
    rdb       *redis.Client
}

func newTestImports(t testing.TB) *TestImports {
    t.Helper()
    if sharedPool == nil {
        t.Skip("imports: sharedPool nil — Docker testcontainer unavailable")
        return nil
    }
    // ... build *Importer via imports.New(imports.Deps{...})
    // ... build composite ApiHandlers with catalog + state + imports
    // ... server.NewMux with composite as StrictHandlers + BodyLimit middleware
    // ... httptest.NewServer(mux)
    // ...
}
```

---

### `services/api/internal/imports/main_test.go` (NEW — TestMain bring-up)

**Analog:** `services/api/internal/state/main_test.go` (4.7KB — verbatim re-use; only package path changes).

The TestMain provisions the testcontainer postgres, runs migrations (now includes 000003), opens pgxpool → assigns to `sharedPool` package-level var. `m.Run()` then proceeds. Migration path stays `../../../../migrations` (Phase 5 doesn't touch the migration path constant).

---

### `services/api/internal/imports/sweep_test.go` (NEW — clockwork-backed)

**Analog:** `services/api/internal/state/ttl_test.go` (15.3KB) — same shape: `clockwork.NewFakeClock()` + `WithClock` option + `fakeClock.Advance(d)` + `require.Eventually(t, ..., 2*time.Second, 50*time.Millisecond, ...)` pattern.

**Test cases:**

- `TestImportSweep_PendingOver24h_FlipsToFailed` — seed a pending row with `updated_at = NOW() - 25h`, call `Start(ctx)` (synchronous startup sweep), assert row transitions to failed BEFORE Start returns.
- `TestImportSweep_PendingUnder24h_NoOp` — seed pending row with `updated_at = NOW() - 23h`, advance fakeClock by 1.5h, assert sweep does NOT flip (boundary case).
- `TestImportSweep_StopCancelsTicker` — start, advance fakeClock past tick interval, Stop(), assert no further SweepCrashedImportJobs calls (Phase 4 Pitfall 2 carry-forward).

---

### `services/api/test/isolation/imports_test.go` (NEW — cross-org probes)

**Analog:** `services/api/test/isolation/state_test.go` (9.1KB — Phase 4 cross-org probes) + `services/api/test/isolation/catalog_test.go` `TestCatalog_CrossOrgSameCode_BothSucceed` (line 375).

**Reuse existing helpers** (do NOT duplicate; defined in `catalog_test.go` lines 35-102 + isolation_test.go):
- `postEntity(t, urlBase, entityPath, orgID, body)`
- `getEntityStatus(t, urlBase, entityPath, orgID, id)`
- `freshOrg(t)`
- `baseURL()`
- `requireContainer(t)`

**Phase 5 test cases (RESEARCH §Validation Architecture FOUND-08 + IMP-03):**

```go
// (a) Cross-org: orgA POST import, orgB GET /imports/{id} → 404
func TestImports_GetImportJobCrossOrg_404(t *testing.T) { ... }

// (b) Cross-org same-code: both orgs import code=emp_001; both succeed
// (mirror TestCatalog_CrossOrgSameCode_BothSucceed at catalog_test.go:375)
func TestImports_CrossOrgSameCode_BothSucceed(t *testing.T) { ... }

// (c) Cross-org break_reason_id (channel default_queue_code) → 422 invalid_reference
//     (mirror state/state_test.go TestState_ForceDoesNotBypassCrossOrgBreakReason)
func TestImports_CrossOrg_FK_invalid_reference(t *testing.T) { ... }
```

---

### `services/api/test/isolation/imports_migration_idempotent_test.go` (NEW — migration replay)

**Analog:** `services/api/test/isolation/migration_idempotent_test.go` (existing Phase 04.1 file at 4.5KB; verified shape lines 1-124). Phase 5 mirrors the pattern: shared testcontainer + INSERT seed → re-execute migration step → assert 0 rows mutated.

**Phase 5 test:**

```go
// TestMigration_ImportJobs_Idempotent — Phase 5 inheritance from D04_1-11
// pattern. The 000003_create_import_jobs migration must be idempotent.
// Re-running CREATE TABLE IF NOT EXISTS + the partial-index DDL produces
// zero schema changes when run twice. NB: golang-migrate short-circuits
// because schema_migrations already has the version; this test asserts
// the SQL itself is replay-safe.
func TestMigration_ImportJobs_Idempotent(t *testing.T) { ... }
```

---

### `openapi/openapi.yaml` (MODIFIED — spec amendment)

**Analog:** itself — D-75 in-place amendment pattern; Phase 04.1's 41-yaml-delta amend is the closest precedent.

**Modifications required (D5-25, D5-26, D5-27 + RESEARCH §Summary Finding 6):**

| # | Delta | Approx Location |
|---|-------|-----------------|
| 1 | Update line 3067 description: "Upsert semantics: ... keyed by (org_id, **code**)" (was external_id) | inline comment on BulkImportCatalog operation |
| 2 | Update line 3212 Import tag description: external_id → code | tags section |
| 3 | Keep `items: {}` at line 3096 (per RESEARCH §F2 — oneOf infeasible); add 6 new request schemas in `components.schemas` | request body retains `items: {}`, schemas land separately |
| 4 | Add `ImportAgentRequest`, `ImportSkillRequest`, `ImportQueueRequest`, `ImportChannelRequest`, `ImportAdapterRequest`, `ImportBreakReasonRequest` schemas | components.schemas |
| 5 | `ImportAgentRequest.skills: [{skill_code, proficiency}]` (was `{skill_external_id, proficiency}`) | nested array per D5-16 |
| 6 | Each Import*Request has `*_code` field with regex `^[a-z][a-z0-9_]{0,63}$` for FK references (mirror Phase 04.1 D04_1-03 pattern) | schema property |
| 7 | Each Import*Request retains optional `external_id` field (integration-mapping, mutable per D04_1-07) | schema property |
| 8 | `ImportJob.status` enum: `[pending, completed, failed]` (required) | D5-26 |
| 9 | Optional `Idempotency-Key` header parameter on BulkImportCatalog operation | D5-27; mirror existing header param shape |
| 10 | `BulkImportResult.idempotent_replay: { type: boolean, default: false, nullable: true }` | D5-27 |

**Reference shape from Phase 04.1's `CreateAgentRequest` (verified Phase 04.1-PATTERNS lines 254-275):**

```yaml
CreateAgentRequest:
  type: object
  required:
    - code
    - name
    - email
  properties:
    code:
      type: string
      pattern: '^[a-z][a-z0-9_]{0,63}$'
      example: "emp_0042"
    external_id:
      type: string
      nullable: true
      description: Optional external-system mapping. See entity schema.
      example: "HR-EMP-0042"
    name:        { type: string }
    email:       { type: string, format: email }
    enabled:     { type: boolean, default: true }
    skills:      { type: array, ... }
```

**Phase 5 `ImportAgentRequest` (mirror with `*_code` FKs instead of UUIDs):**

```yaml
ImportAgentRequest:
  type: object
  required:
    - code
    - name
    - email
  properties:
    code: { type: string, pattern: '^[a-z][a-z0-9_]{0,63}$' }
    external_id: { type: string, nullable: true }
    name:  { type: string }
    email: { type: string, format: email }
    enabled: { type: boolean, default: true }
    skills:
      type: array
      items:
        type: object
        required: [skill_code, proficiency]
        properties:
          skill_code: { type: string, pattern: '^[a-z][a-z0-9_]{0,63}$' }
          proficiency: { type: integer, minimum: 1, maximum: 10 }
```

**Pattern hazard** (Phase 04.1 D04_1-23 carry-forward / RESEARCH §Pitfall 5): after `task gen`, the planner MUST diff `server.gen.go` against `injectRequestIDIntoErrorResponse` (in `services/api/internal/server/server.go`) and add any new `*JSONResponse` case BEFORE running `TestRequestIDInjection_Exhaustiveness`. Phase 5 likely introduces NO new error response types (BulkImportCatalog already has 200/207/400/413/422/500); GetImportJob already has 200/400/404/500. Verified A7 in RESEARCH but planner MUST re-confirm post-gen.

---

### `services/api/internal/catalog/notimpl.go` (MODIFIED — delete two stubs)

**Analog:** itself — Phase 4 already deleted `GetAgentStatus` + `PatchAgentStatus` stubs from the same file (verified file content lines 1-37).

**Current file shape** (verified — 37 lines total):

```go
// notimpl.go — 501-equivalent stubs for endpoints owned by Phase 5
// (bulk import).
//
// Phase 4 deleted: GetAgentStatus + PatchAgentStatus 501 stubs.
// state.Server owns these methods; cmd/api/main.go composes
// *catalog.Handlers + *state.Server into ApiHandlers (D-89).
//
// Remaining stubs are Phase 5 (bulk import) — kept until Phase 5 lands.
package catalog

import (
    "context"
    "github.com/luongdev/open-routing/services/api/internal/api"
)

func notImplementedBody() api.InternalServerErrorJSONResponse {
    return api.InternalServerErrorJSONResponse{
        Error:  api.ErrorCodeInternal,
        Reason: "not_implemented_yet",
    }
}

func (h *Handlers) BulkImportCatalog(_ context.Context, _ api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
    return api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

func (h *Handlers) GetImportJob(_ context.Context, _ api.GetImportJobRequestObject) (api.GetImportJobResponseObject, error) {
    return api.GetImportJob500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}
```

**Phase 5 action:** DELETE the entire file. After Phase 5 wires `*imports.Importer` into `ApiHandlers`, the composite satisfies the full `StrictServerInterface` and `catalog.Handlers` no longer needs the stubs. The `notImplementedBody()` helper is also deleted (no other callers — RESEARCH §F3 verified).

**Pattern hazard:** The `var _ api.StrictServerInterface = (*Handlers)(nil)` assertion in `catalog/handlers.go` line 92 is ALREADY GONE (Phase 4 moved it to `cmd/api/main.go:189` on `*ApiHandlers`). Phase 5 does NOT need to touch the assertion location.

---

### `services/api/cmd/api/main.go` (MODIFIED — wire imports.Importer)

**Analog:** itself (245 lines verified). Phase 4 added `*state.Server` at lines 150-194; Phase 5 adds `*imports.Importer` as a third embed.

**Current verified pattern** (main.go lines 150-194):

```go
// (8.5) state.Server — D-89 composite activation.
stateServer := state.New(state.Deps{
    OrgDB:  orgDB,
    Cache:  catalogCache,
    Logger: slog.Default(),
}, state.WithClock(clockwork.NewRealClock()))

if err := stateServer.Start(ctx); err != nil {
    slog.ErrorContext(ctx, "state server start", "err", err)
    return 1
}
defer stateServer.Stop()

type ApiHandlers struct {
    *catalog.Handlers
    *state.Server
}
var _ api.StrictServerInterface = (*ApiHandlers)(nil)

apiHandlers := &ApiHandlers{
    Handlers: catalogHandlers,
    Server:   stateServer,
}
```

**Phase 5 additions (RESEARCH §F3 Open Q5 — third embed via `imports.Importer`):**

```go
// (8.6) imports.Importer — Phase 5 composite activation (RESEARCH §F3).
importer := imports.New(imports.Deps{
    OrgDB:  orgDB,
    Cache:  catalogCache,        // SAME cache instance — single Redis client
    Logger: slog.Default(),
}, imports.WithClock(clockwork.NewRealClock()))

// Synchronous startup sweep — guarantees no stuck pending job survives a
// restart (D5-11). Failure aborts startup so kubernetes restarts with
// a fresh attempt.
if err := importer.Start(ctx); err != nil {
    slog.ErrorContext(ctx, "imports importer start", "err", err)
    return 1
}
// Shutdown order (LIFO): http.Server.Shutdown drains in-flight HTTP
// requests; THEN importer.Stop drains crash-sweep ticker; THEN
// stateServer.Stop drains wrapup sweeper; THEN pool.Close.
defer importer.Stop()

type ApiHandlers struct {
    *catalog.Handlers
    *state.Server
    *imports.Importer  // PHASE 5 ADDITION (third embed; no naming collision)
}
var _ api.StrictServerInterface = (*ApiHandlers)(nil)

apiHandlers := &ApiHandlers{
    Handlers: catalogHandlers,
    Server:   stateServer,
    Importer: importer,
}

// (9) chi mux + Phase 5 body-limit middleware wired at the import path.
mux := server.NewMux(&server.Deps{
    Pool:           pool,
    Redis:          rdb,
    OrgDB:          orgDB,
    Config:         cfg,
    StrictHandlers: apiHandlers,
    SpecBytes:      specBytes,
    // Phase 5: BodyLimit middleware scoped to /v1/orgs/*/catalog/import
    // (see middleware.BodyLimit; wiring goes inside server.NewMux's
    // mux.Use chain — coordinate with server.go).
})
```

**Pattern hazards:**

- **Shutdown order** — three defers (importer.Stop, stateServer.Stop, pool.Close). The order matters: HTTP server first (drain requests), then importer (drain crash sweep), then state (drain wrapup sweeper), then pool. Phase 4's analog order is preserved; Phase 5 inserts importer.Stop BEFORE stateServer.Stop.
- **Single cache instance** — `catalogCache` is shared across all three composites; cache keys are namespaced per entity (`or:{orgId}:agents:{id}` vs `or:{orgId}:agent_state:{id}`).
- **Compile-time assertion** — `var _ api.StrictServerInterface = (*ApiHandlers)(nil)` will fail to compile if the spec gains a new endpoint without an implementation, OR if one of the three embedded types loses a method. This is the canonical Phase 5 ship signal.
- **`server.NewMux` middleware injection** — Phase 5's `BodyLimit("/v1/orgs/", 50<<20)` middleware needs to be added to the chain INSIDE `server.NewMux` (in `services/api/internal/server/server.go`); the planner's Wave 0 task should append it. Order: `RequestID → OrgContext → BodyLimit("/v1/orgs/...")` (RESEARCH Open Q7).

---

## Shared Patterns

### Cross-Cutting Pattern 1: Org context extraction (every handler entry)

**Source:** `services/api/internal/catalog/agents.go` lines 72-77, repeated in every handler.

**Apply to:** `BulkImportCatalog`, `GetImportJob` in `imports/handler_*.go`.

```go
orgID, ok := orgkey.OrgIDFromContext(ctx)
if !ok {
    return api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
        Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
    }}, nil
}
```

This contract holds because `OrgContext` middleware sets `org_id` in ctx for every `/v1/orgs/{org_id}/...` route. Failure here means the middleware was bypassed — always 500, never 4xx.

### Cross-Cutting Pattern 2: D-74 Two-Layer Validation (Phase 04.1 carry-forward)

**Source:** RESEARCH §Pattern 2 + Phase 04.1 Layer 1 + Layer 2 flow.

**Apply to:** Every row processor in `imports/row_<entity>.go`.

- **Layer 1 (format/regex):** call `catalog.ValidateCodeFormat(typed.Code)` (exported in Phase 5 amend) BEFORE any DB call. False → row-level `invalid_code_format`.
- **Layer 1 (nested):** for agent imports, ALSO validate each `sk.SkillCode` in the nested `skills[]` BEFORE calling `ResolveSkillCodes`. Pitfall 9 — failing to do this gives "unknown_skill" instead of "invalid_code_format".
- **Layer 2 (business rule):** not directly applicable to imports (immutability checks come into play on UPDATE-not-INSERT-paths). But the per-row processor still consults `mapPgError` after `UpsertXByCode` to map `23505 (org_id, code)` → `duplicate_code` vs `23505 (org_id, external_id)` → `duplicate_external_id`.

### Cross-Cutting Pattern 3: D-68 One-File-Per-Entity (Phase 3 carry-forward)

**Source:** `services/api/internal/catalog/{agents,skills,queues,channels,adapters,break_reasons}.go` — 6 files (verified via ls).

**Apply to:** `imports/row_<entity>.go` × 6.

Each row processor file contains exactly one `<entity>RowProc` struct with a `process(ctx, sp, orgID, r, skillResolver) (succeededRow, *rowError)` method. No mixing of entity logic across files.

### Cross-Cutting Pattern 4: D-72 Per-Entity `_test.go` (Phase 3 carry-forward)

**Source:** `services/api/internal/catalog/{entity}_test.go` × 6 files.

**Apply to:** `imports/row_<entity>_test.go` × 6 (one paired test per row processor). The end-to-end `handlers_test.go` exercises the full pipeline; per-entity tests exercise validation + coercion at the row-processor level.

### Cross-Cutting Pattern 5: Cache Invalidation Key Format

**Source:** D-58 + Phase 3 `cache.Key(orgID, "agents", id)` in `catalog/agents.go:202`.

**Apply to:** Phase 5 POST-COMMIT cache invalidation in `imports/chunk.go`.

**Key format:** `or:{orgId}:{entity_table_name}:{entity_id}` — Phase 5 invalidates the EXISTING catalog cache keys so subsequent catalog GETs reload fresh rows. Uses the PLURAL entity name (`agents`, NOT singular `agent_state`-style namespace).

```go
cacheKey := cache.Key(orgID, string(sr.entity), sr.id)
// where sr.entity is api.Agents | api.Skills | ... — already the plural table name
if delErr := s.deps.Cache.Del(ctx, cacheKey); delErr != nil { ... }
```

### Cross-Cutting Pattern 6: Bypass Pattern for Org-Agnostic Goroutine

**Source:** Phase 4 `state/ttl.go:179` (`db.WithBypass(ctx, "wrapup_sweeper.safety")`) + Phase 1 D-04.

**Apply to:** `imports/sweep.go` — `db.WithBypass(ctx, "import_crash_sweep")` for the ticker tick path; `db.WithBypass(ctx, "import_crash_sweep.startup")` for the synchronous startup sweep. Distinct event strings keep the audit log greppable.

### Cross-Cutting Pattern 7: Error Envelope

**Source:** Phase 2 D-35 — `{error, reason, request_id}` for batch-level errors.

**Apply to:**
- 400/413/415/500 paths: use `middleware.WriteError(ctx, w, status, code, reason)` — strict-server's generated wrappers actually take a `JSONResponse` struct; Phase 5 builds it as `api.BulkImportCatalog<status>JSONResponse{ErrorResponse{Error: api.ErrorCodeX, Reason: "..."}}`. The wire shape ends up identical to `WriteError`.
- 200/207/422 success-ish paths: emit `BulkImportResult` directly via `api.BulkImportCatalog<status>JSONResponse(api.BulkImportResult{...})`.
- Per-row errors in `failed[]`: `api.BulkImportFailedRow{Row: N, Field: F, Error: api.BulkImportFailedRowErrorImportFailed, Reason: "..."}` — separate from the batch envelope per Phase 2 OQ-2A.

---

## No Analog Found

| File | Role | Data Flow | Reason | Recommendation |
|------|------|-----------|--------|----------------|
| `services/api/internal/imports/coerce.go` | utility | pure-transform | No in-tree CSV/JSON cell coercion code. Phase 5 is the first synthesizer of typed coercion pipelines. | Use RESEARCH §Pattern 1 + the D5-01..D5-08 decisions as the locked spec; structural cousin `catalog/codecheck.go` (pure-function file) for shape. |
| `services/api/internal/imports/parser_json.go` (re-marshal helper) | utility | pure-transform | Pure-new code per RESEARCH §F1 accommodation; no existing helper in the tree marshals-then-unmarshals an interface{} for type re-decoding. | Use RESEARCH §Pattern 4 verbatim. |

---

## Metadata

**Codegen Output Files (regenerated by `task gen`):**

| File | Source |
|------|--------|
| `services/api/internal/api/server.gen.go` | `openapi/openapi.yaml` |
| `services/api/internal/api/types.gen.go` | `openapi/openapi.yaml` (emits 6 new Import*Request types, ImportJobStatus enum, BulkImportResult.IdempotentReplay pointer, BulkImportCatalogParams.IdempotencyKey pointer) |
| `services/api/internal/api/spec.gen.go` | `openapi/openapi.yaml` (embedded yaml bytes) |
| `services/api/internal/db/generated/import_jobs.sql.go` | sqlc reads `db/queries/import_jobs.sql` |
| `services/api/internal/db/generated/skills.sql.go` | sqlc reads updated `db/queries/skills.sql` (adds `ResolveSkillCodes`) |
| `services/api/internal/db/generated/agent_skills.sql.go` | sqlc reads updated `db/queries/agent_skills.sql` (adds `MergeAgentSkill`) |
| `services/api/internal/db/generated/agent_states.sql.go` | sqlc reads updated `db/queries/agent_states.sql` (adds `InsertAgentStateOnConflictNothing`) |
| `web/packages/ui/src/api/generated.ts` | openapi-typescript via `pnpm gen:api` |

**Analog search scope:**

- `services/api/internal/catalog/` (6 entity handlers + helpers — Phase 3 + Phase 04.1 templates)
- `services/api/internal/state/` (Phase 4 state machine — closest goroutine + cache + handler pattern)
- `services/api/internal/middleware/` (Phase 1 middleware shapes for BodyLimit)
- `services/api/internal/db/orgdb.go` (BeginTx + BeginSavepoint extension target)
- `services/api/internal/db/queries/` (8 existing sqlc query files — Phase 5 appends 3)
- `services/api/test/isolation/` (cross-org probe templates)
- `migrations/000002_catalog_v0_1.{up,down}.sql` (migration shape; Phase 5 creates new 000003)

**Files scanned:** 23 source files + 8 sqlc query files + 2 migrations + 1 yaml spec = 34 files.

**Pattern extraction date:** 2026-05-17.
