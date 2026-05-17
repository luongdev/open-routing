# Phase 5 (+ Phase 04.1) Combined Cross-AI Review

**Date:** 2026-05-17
**Diff base:** 286b3eb61a73cd4c992d8ffbf72d5d26f7ab1097 (origin/main merge-base)
**Diff HEAD:** 93b146223f290713029538a0ad695a6dcbf2976b (Phase 5 Plan 05-08 Task 1 lint-fix commit)
**Branch:** worktree-agent-a97806d3ebd6f384c → gsd/phase-05-bulk-import-go
**Diff size:** 41,235 total lines / 16,386 code-only lines (91 files; planning/.md, generated codegen output, pnpm-lock, go.sum, testdata excluded for length budget)
**Commits since merge-base:** 90 (Phase 04.1 Plans 01..06 + Phase 5 Plans 01..08 + merge commits)
**Reviewers:** Codex (gpt-5.5 fell back to CLI default — gpt-5.5/high not available; CLI default ≈ gpt-5.1) + Gemini (gemini-3.1-pro-preview) — both run in parallel per CLAUDE.md HARD RULE invocation pattern.
**Per:** CLAUDE.md HARD RULE + orchestrator combined-review override (review Phase 04.1 + Phase 5 in a single pass; satisfies Phase 04.1 Plan 06's deferred peer review).

## Combined Review

## Scope

This review covers BOTH Phase 04.1 (catalog identity normalization — universal `code` contract, `external_id` demoted to optional, migration 000002 in-place amend, sqlc query authoring for 6 entities, OpenAPI contract update + 36 new test cases) AND Phase 5 (bulk import — JSON/CSV ingest endpoint, savepoint-chunk orchestration, idempotency-key replay, 24h crash sweep, migration 000003 `import_jobs`, internal/imports/ package with 138+ tests).

Phase 04.1's Plan 06 explicitly deferred its cross-AI review to this combined pass (user instruction "review cả thể" — review everything together). The combined approach is the orchestrator's directive (see Phase 04.1 04.1-REVIEW.md § Deferral Notice).

### Diff capacity workaround

The full origin/main..HEAD diff is 41,235 lines / 2.4 MB — exceeds both Codex's and Gemini's 1 MB stdin context cap. Codex's first invocation (full diff) failed with `Error: turn/start failed: Input exceeds the maximum length of 1048576 characters`. The review was re-run against a **code-only** trimmed diff (16,386 lines / 681 KB) that excludes:

- `.planning/**` (45 files of planning markdown)
- `web/packages/ui/src/api/generated.ts` (openapi-typescript generated)
- `services/api/internal/api/{server,types,spec}.gen.go` (oapi-codegen generated)
- `services/api/internal/db/generated/**` (sqlc generated)
- `services/api/internal/imports/testdata/**` (fixture data)
- `go.sum`, `pnpm-lock.yaml` (dependency lock manifests)

**Trade-off:** the trimmed diff still includes `openapi/openapi.yaml` (the source of truth for the contract), all migrations, all hand-written Go code, all SQL queries, and all hand-written tests. The exclusions are reproducible-from-source artefacts that the reviewers should not need to inspect directly to reason about correctness; this matches industry practice for AI-assisted code review. The reviewers were informed about the exclusion in the prompt header.

## Codex Review

**Note:** gpt-5.5/reasoning-high failed; Codex CLI default model (gpt-5.1) ran the review.

### Codex Summary (verbatim)

> Diff có hướng đúng nhưng chưa READY. Có lỗi contract/PATCH rõ ràng và một số rủi ro transaction/idempotency làm sai hành vi production.

(Translation: "The diff is heading the right way but is not yet READY. There are clear PATCH/contract bugs and several transaction/idempotency risks that produce wrong production behaviour.")

### Codex Concerns (verbatim)

#### HIGH

- `services/api/internal/catalog/agents.go:537`, `skills.go:363`, `queues.go:395`, `channels.go:408`, `adapters.go:407`, `break_reasons.go:381`
  PATCH `external_id` duplicate sẽ bị map thành 500 thay vì 409. `MapPgError` trả `status=409` cho `duplicate_external_id`, nhưng các Update handler chỉ xử lý `status == 422`, còn lại rơi vào 500. Impact: client-correctable uniqueness conflict làm bẩn 5xx metrics và lệch OpenAPI/error contract.

- `services/api/internal/db/queries/agents.sql:96`, `skills.sql:72`, `queues.sql:59`, `channels.sql:62`, `adapters.sql:64`, `break_reasons.sql:64`
  OpenAPI nói PATCH `external_id: null` sẽ clear binding, nhưng SQL dùng `COALESCE(sqlc.narg('external_id')::text, external_id)`, nên `NULL` luôn preserve giá trị cũ. Với oapi-codegen optional nullable thường cũng không phân biệt omit/null bằng `*string`. Impact: PATCH semantics contract drift; client không thể clear `external_id`.

- `services/api/internal/imports/handler_import.go:415`
  `finaliseJob` failure chỉ log rồi vẫn trả success/partial result. Nếu finalise fail, job còn `pending` và sau 24h sweep có thể đánh dấu `failed/server_crash` dù import đã landed. Impact: persisted audit/idempotency replay sai, GET job sai trạng thái, retry với same idempotency key có thể replay pending/empty counters.

#### MED

- `services/api/internal/imports/chunk.go:303`
  `sp.Commit` failure được ghi failed nhưng không rollback savepoint/outer tx. Nếu `RELEASE SAVEPOINT` fail, transaction thường ở trạng thái không đáng tin; loop vẫn tiếp tục và outer commit có thể tạo kết quả khó đoán. Nên rollback/abort chunk và mark chunk failed.

- `services/api/internal/imports/chunk.go:273`
  `BeginSavepoint` failure chỉ mark row failed rồi tiếp tục. Nếu outer tx đã closed/aborted, toàn bộ chunk còn lại nên fail nhanh và rollback outer tx. Hiện tại có thể tạo nhiều failure sai nguyên nhân và vẫn thử commit outer tx.

- `services/api/internal/imports/handler_import.go:454`
  Partial success terminal status đang là `completed`; only all-fail là `failed`. Nếu status enum chỉ có pending/completed/failed thì đây có thể chấp nhận, nhưng cần đảm bảo downstream hiểu `failed_rows > 0` là partial. Nếu "failed" nghĩa "any row failed", hiện contract drift.

- `services/api/internal/imports/header.go:256`, `handler_import.go:574`
  `config` CSV malformed JSON không fail ở coercion; `materialiseTypedRaw` để string lọt xuống và row processor trả `invalid_json_row` field rỗng. Impact: lỗi field-level kém chính xác, không chỉ ra `config`.

- `services/api/internal/imports/row_agent.go:188`
  Comment nói proficiency bounded `0..100`, trong khi OpenAPI import schema là `minimum: 1, maximum: 10`. Không thấy validation explicit trước `MergeAgentSkill`; nếu DB CHECK là 1..10 thì lỗi sẽ thành generic `merge_skill_failed` thay vì field `skills[i].proficiency`. Contract row-level error yếu.

#### LOW

- `.gitattributes:4`
  Comment khá dài trong metadata file; không ảnh hưởng runtime.

- `services/api/internal/imports/doc.go`
  File map comment đã lỗi thời: nói Wave 3/4 "will add" dù các file đã có trong diff. Không ảnh hưởng runtime nhưng gây nhiễu bảo trì.

- `services/api/internal/imports/row_agent_test.go:265`
  Test bị `t.Skip` cho path `agent_state_seed_failed`; nếu path này quan trọng, nên thay bằng unit/mock hoặc bỏ test skip để tránh false sense of coverage.

### Codex Suggestions (verbatim)

- Chuẩn hóa `MapPgError` handling cho Update handlers: `409 -> Update*409JSONResponse`, `422 -> Update*422JSONResponse`, còn lại 500.
- Nếu cần PATCH clear nullable fields, dùng tri-state type hoặc generated nullable wrapper, rồi SQL tách `external_id_set` và `external_id_value`.
- Với chunk transaction, coi savepoint begin/commit failure là chunk-level failure: rollback outer tx, chuyển succeeded trong chunk sang failed.
- Nếu `finaliseJob` fail, nên trả 500 hoặc ít nhất không trả idempotent-safe success.
- Thêm tests: PATCH duplicate external_id, PATCH external_id null clear, malformed adapter CSV config, proficiency out of range.

### Codex Verdict (verbatim)

> Verdict: BLOCK

## Gemini Review

**Reviewer:** gemini-3.1-pro-preview (primary model SUCCESS — no fallback).

### Gemini Summary (verbatim)

> The Phase 04.1 and Phase 5 changes introduce a comprehensive new universal identity contract (`code`) and a robust bulk import orchestrator with savepoint-based chunking. The bulk import pipeline correctly manages transaction boundaries, CSV/JSON dispatch, nested skill merging, and idempotency key constraints.
>
> However, the refactored HTTP PATCH handlers introduce a critical optimistic concurrency regression and broken sparse-PATCH capabilities for clearing the newly added `external_id` column. Furthermore, a cache key mismatch and omitted status code logic in the idempotency replay path must be addressed.

### Gemini Concerns (verbatim)

#### HIGH
- **Broken Sparse PATCH Semantics for `external_id`** (Multiple files: `services/api/internal/db/queries/*.sql`)
  The `UPDATE` queries map `external_id` using `COALESCE(sqlc.narg('external_id')::text, external_id)`. The OpenAPI contract states that clients should "pass JSON null to clear the binding". However, `oapi-codegen` unmarshals `null` as a `nil` pointer, which translates to a SQL `NULL` parameter. `COALESCE(NULL, external_id)` evaluates to the *existing* value, completely ignoring the `null` payload and making it impossible to clear an `external_id`.
- **Concurrency Bug in PATCH 404/409 Disambiguation** (Multiple files: e.g., `services/api/internal/catalog/agents.go:499`)
  To enforce `code` immutability, the `UpdateX` handlers fetch a pre-update `stored` row. If the subsequent `UpdateX` query returns `0 rows` due to a concurrent transaction updating the version, the handler reuses the pre-update `stored` row to check if `expectedVersion == cur.Version`. Because it reuses the stale version rather than fetching the fresh DB state, the check incorrectly passes, resulting in a `404 Not Found` rather than the correct `409 Conflict`.

#### MED
- **Cache Invalidation Key Mismatch for Break Reasons** (`services/api/internal/imports/chunk.go:379`)
  The chunk loop calls `cache.Key(orgID, string(sr.entity), sr.id)`. For break reasons, `string(api.BreakReasons)` evaluates to `"break-reasons"` (hyphenated). The catalog handler sets the cache key as `"break_reasons"` (underscored). The post-commit invalidation will completely miss bulk-imported break reasons.
- **Idempotency Replay Discards Original HTTP Status** (`services/api/internal/imports/idempotency.go:85`)
  The `rehydrateBulkImportResult` function is hardcoded to return `api.BulkImportCatalog200JSONResponse`. Replaying a job that originally failed as a `422 Unprocessable Entity` (all fail) or `207 Multi-Status` (partial fail) will inaccurately return an HTTP `200 OK`, masking the semantic failure of the job from the client.
- **Contract Drift on Queue Priority Default** (`services/api/internal/imports/row_queue.go:52`)
  The queue row processor defaults `priority` using `derefInt(typed.Priority, 0)`. The OpenAPI contract explicitly enforces `{ type: integer, default: 5 }` for `priority`. Imported CSV queues omitting the priority column will be assigned 0 instead of the documented 5.

#### LOW
- **Missing Bounds Checking in CSV Parsing** (`services/api/internal/imports/coerce.go:94`)
  `parseInt` relies on `strconv.ParseFloat` and `math.Trunc` before blindly casting to `int`. Extremely large numeric strings (e.g., `999999999999`) will silently overflow or truncate when cast downstream to `int32`, resulting in silently corrupted `priority` or `acw_sec` values.
- **Bulk Import Destructive Upsert for Omitted Columns** (`services/api/internal/imports/row_*.go`)
  Because bulk import maps all columns to `EXCLUDED.*` during upserts, it functions as a full `PUT`. If an admin imports a CSV missing the `external_id` column to simply fix a typo in a `name`, the upsert will overwrite the existing `external_id` with `NULL`. While technically correct for an upsert, it risks catastrophic metadata loss for admins expecting PATCH semantics.

### Gemini Suggestions (verbatim)
- **Sparse PATCH Fix:** Abandon `COALESCE` for optional fields that need explicit nullification. Refactor the `UpdateX` query parameters to use `pgtype.Text` so you can explicitly distinguish between an absent field (`Valid: false`) and a nulled field (`Valid: true, String: ""`), or dynamically construct the `SET` clause in the handlers based on request payload presence.
- **Fix 404/409 Disambiguation:** Remove the `cur = *stored` shortcut inside the `ErrNoRows` fallback branch. Always issue a fresh `GetXByIdAnyVersion` query if `UpdateX` returns `0 rows` so the handler can evaluate the actual post-update state.
- **Cache Key Normalization:** In `chunk.go`, safely map `string(sr.entity)` by applying `strings.ReplaceAll(..., "-", "_")` before injecting it into `cache.Key()`.
- **Status Code on Replay:** Plumb logic into `rehydrateBulkImportResult` to evaluate `job.SucceededRows` and `job.FailedRows` and construct the matching `200`, `207`, or `422` response wrapper rather than defaulting to `200`.
- **Queue Default Alignment:** Update `derefInt(typed.Priority, 0)` to `derefInt(typed.Priority, 5)`.
- **Integer Boundaries:** Enforce `int32` bounds checking (`math.MinInt32` and `math.MaxInt32`) inside `parseInt` before casting.

### Gemini Verdict (verbatim)

> Verdict: BLOCK

## Synthesis

### Verification of HIGH concerns (executor cross-checked each finding against the actual code)

#### Codex HIGH #1 — PATCH 23505 → 500 instead of 409 (CONFIRMED REAL)

All 6 Update handlers in `services/api/internal/catalog/{agents,skills,queues,channels,adapters,break_reasons}.go` use this exact pattern at the MapPgError site (e.g., `agents.go:540`):

```go
status, code, reason := MapPgError(err, "agent")
if status == 422 {
    return api.UpdateAgent422JSONResponse(api.ErrorResponse{Error: code, Reason: reason}), nil
}
h.deps.Logger.ErrorContext(ctx, "update agent", "err", err)
return api.UpdateAgent500JSONResponse{...code, reason}, nil
```

`MapPgError` returns `status=409` for both `duplicate_code` and `duplicate_external_id` (constraint-name introspect), but the handler only branches on `status==422`. Status 409 falls through to the 500 path, returning a client-correctable conflict as a server error. This is the **same pattern of bug** Phase 04.1 Wave 5 already caught in CREATE handlers (per `channels.go:670` docstring "Wave 5 cross-AI review: 23505 surfaced as 500 was poisoning 5xx metrics for a client-correctable error") — the Wave 5 fix was applied to CREATE but never propagated to UPDATE.

**Impact:** PATCH operations that supply a duplicate `external_id` (or, after a successful rename when v0.2 lifts code immutability, a duplicate `code`) return 500 instead of 409. Client retry logic that distinguishes server vs client errors will mis-handle this. 5xx alert metrics get poisoned.

**Severity:** HIGH — affects all 6 Update endpoints.

#### Codex HIGH #2 / Gemini HIGH #1 — PATCH `external_id: null` cannot clear (CONFIRMED REAL)

Both reviewers flagged the same SQL pattern:

```sql
UPDATE agents
SET external_id = COALESCE(sqlc.narg('external_id')::text, external_id), ...
```

OpenAPI documents (Update*Request schema):

> Mutable external-system mapping (D04_1-07). Pass a string to (re)bind to an external row; **pass JSON null to clear the binding**; omit the field to leave unchanged.

The SQL COALESCE pattern preserves the existing value whenever the SQL parameter is NULL. Since oapi-codegen renders both omitted-field and explicit-`null` as nil pointer (`*string == nil`), there is NO way for sqlc to distinguish the two states. **Clients can never clear `external_id` via PATCH.**

The pre-existing SQL comment (in agents.sql:88) acknowledges this trade-off for `email`/`enabled` ("PATCH may omit optional fields and oapi-codegen renders omitted pointer types as nil → sqlc renders NULL → without COALESCE the UPDATE would zero out email/enabled"). But for `external_id`, which Phase 04.1 made nullable AND documented as null-clearable, this is a contract drift introduced by Phase 04.1.

**Impact:** Admins cannot remove a binding to a stale external system without going via direct DB write.

**Severity:** HIGH — direct OpenAPI contract drift for all 6 catalog Update endpoints; introduced by Phase 04.1.

#### Codex HIGH #3 — `finaliseJob` failure swallowed (CONFIRMED REAL)

In `internal/imports/handler_import.go:415`:

```go
if finErr := s.finaliseJob(ctx, jobID, orgID, terminalStatus, ...); finErr != nil {
    // Log but proceed: the import landed at the DB level; only the
    // audit row failed. Returning the result preserves the admin's
    // ability to see what succeeded; the sweep goroutine will mark
    // the pending row failed after 24h if the row ever surfaces.
    s.deps.Logger.WarnContext(ctx, "import.finalise_job_failed", "err", finErr, "job_id", jobID)
}
```

The handler then returns 200/207 with the actual succeeded UUIDs. But the persisted `import_jobs` row stays in `status='pending'`. Three downstream consequences:

1. **GET /imports/{id}** returns `status: pending` with `succeeded_rows=0, failed_rows=0` — admin sees "pending" for an import that actually landed.
2. **Idempotency replay** (D5-13) reads the pending row; `rehydrateBulkImportResult` returns empty succeeded[] + empty failed[] with status 200, masking the real outcome.
3. **24h crash sweep** flips the pending row to `status='failed'` with `errors=[{row:0, error:'import_failed', reason:'server_crash'}]` — overwriting the actual outcome with a synthetic crash entry.

**Impact:** Persistent audit-trail corruption + idempotency-replay corruption. Admin observability completely diverges from DB reality.

**Severity:** HIGH — every finalise failure (e.g., connection drop after chunk commits) produces this corruption.

#### Gemini HIGH #2 — Concurrency bug in PATCH 404/409 disambiguation (PARTIALLY VALID — reclassified MED)

Gemini's specific claim ("the check incorrectly passes, resulting in a 404 Not Found rather than the correct 409 Conflict") is **wrong on close reading**. The disambiguation branch at `agents.go:498`:

```go
if errors.Is(err, pgx.ErrNoRows) {
    var cur generated.Agent
    if stored != nil {
        cur = *stored   // unconditionally enters the 409 path; no version check
    } else {
        // ... fresh GetByIdAnyVersion probe
    }
    return api.UpdateAgent409JSONResponse{Current: mapAgent(cur, nil), ...}, nil
}
```

There is NO `expectedVersion == cur.Version` check in this branch. It UNCONDITIONALLY returns 409 when `stored != nil` (which is true when the pre-UPDATE Layer 2 fetch found the row). This is correct — if `stored` proved the row existed pre-UPDATE and the UPDATE returned 0 rows with WHERE clause `(id, org_id, version=expected_version)`, the cause MUST be a version mismatch (UPDATE has no other filter).

**Re-evaluated severity:** Gemini's underlying concern is real but more subtle: the 409 response includes `Current: mapAgent(cur)` where `cur` is the pre-UPDATE snapshot, not the latest DB state. Between the pre-fetch and the UPDATE, another transaction may have advanced the row past its current state. The client receiving 409 might be shown a `current` snapshot that is itself already stale.

**Severity:** MED (downgraded from Gemini's HIGH) — affects 409 response payload freshness but not the HTTP status code itself. Recommendation: Gemini's "always GetByIdAnyVersion when 0 rows" suggestion is over-conservative — but adding a fresh probe specifically for the `Current` field when `stored != nil` would make the response state authoritative.

### Verification of MED concerns

#### Codex MED #1 — Savepoint commit failure not rolled back (LIKELY REAL)

`chunk.go:303` after `sp.Commit` failure marks the row as failed but the outer tx continues. If `RELEASE SAVEPOINT` fails, the tx state is undefined per PG semantics. The current behaviour might commit garbage on outer commit. **Severity:** MED — rare path (RELEASE rarely fails in practice) but the recovery is unsound.

#### Codex MED #2 — `BeginSavepoint` failure cascades silently (LIKELY REAL)

`chunk.go:273` similar issue. If outer tx is already aborted (e.g., deadlock killed it externally), the failed-row append keeps going but every subsequent savepoint will also fail. The first savepoint failure should abort the chunk.

#### Codex MED #3 — Terminal status `completed` even when failed_rows > 0 (CONTRACT-DEFINED)

`ImportJob.status` enum is `pending | completed | failed`. The plan (D5-26) explicitly states v0.1 returns `completed` or `failed`. The convention is:
- `completed` = the job ran to completion (some rows may have failed at the row level).
- `failed` = the job did not produce any succeeded rows.

This is plan-locked, not a bug. **Severity:** LOW (re-classified) — the semantic is documented; partial-success is signalled via `failed_rows > 0` on the persisted row, not via the status enum.

#### Codex MED #4 — Adapter `config` JSON malformed falls through to row_failure with empty field (LIKELY REAL)

CSV `config` cell may carry stringified JSON; if malformed it produces a generic `invalid_json_row` without field specificity. **Severity:** MED — field-level error precision is weaker than other rows; admins debugging a failed adapter import lose the "which field?" signal.

#### Codex MED #5 — Proficiency range not validated before MergeAgentSkill (CONFIRMED REAL)

`row_agent.go:181` calls `MergeAgentSkill` with `int32(sk.Proficiency)` and NO upstream range check. The DB has `CHECK (proficiency BETWEEN 1 AND 10)` (000002 migration). Out-of-range value triggers 23514, mapped to `merge_skill_failed` generic — field is `skills[i]` not `skills[i].proficiency`. The catalog package has `validateProficiencyRange` (`internal/catalog/agent_skills.go:32`) but it's NOT called from imports.

**Severity:** MED — field-level error precision is weaker; out-of-range values still rejected by DB CHECK; but `merge_skill_failed` masks the actual cause.

Also note: my own lint-fix comment in row_agent.go:181 says "bounded 0..100 by validator" — that's WRONG; OpenAPI specifies `1..10`. This needs correction whether or not the validation is added.

#### Gemini MED #1 — Cache key mismatch break_reasons (FALSE POSITIVE)

`string(api.BreakReasons)` evaluates to `"break_reasons"` (underscore) per `types.gen.go:160`. Gemini confused this with the URL slug `/break-reasons` (hyphen). Cache key `cache.Key(orgID, "break_reasons", id)` matches catalog handler's pattern. **Verdict:** No bug; reject this finding.

#### Gemini MED #2 — Idempotency replay always returns 200 (CONFIRMED REAL)

`idempotency.go:89` — `rehydrateBulkImportResult` is hardcoded to return `BulkImportCatalog200JSONResponse`. A replay of a job that originally returned 207 (partial) or 422 (all-failed) misrepresents the outcome.

**Severity:** MED — contract divergence on replay. The original D5-13 design said "returns the persisted prior result" — implicitly including the status code.

#### Gemini MED #3 — Queue priority default 0 vs OpenAPI default 5 (FALSE POSITIVE)

OpenAPI `priority` has `example: 5` but **NO `default: 5`**. `derefInt(typed.Priority, 0)` is consistent with no documented default. **Verdict:** Not a contract drift.

### Verification of LOW concerns

- **Codex LOW #1** (.gitattributes verbose comment): LOW; cosmetic. Reject — long comment is intentional for downstream agents.
- **Codex LOW #2** (doc.go stale "will add"): LOW; minor doc nit; can be addressed in post-merge cleanup.
- **Codex LOW #3** (`t.Skip` on `agent_state_seed_failed`): LOW; the test is a placeholder; the actual seed-failure path is exercised by unit tests via mock. Acceptable but worth documenting.
- **Gemini LOW #1** (parseInt int32 overflow): LOW-MED; the gosec G115 false-positive I annotated in Task 1 lint-fix actually has a real underlying concern — extreme numeric strings ARE accepted by the coerce layer. The DB layer (INT4 column) rejects > 2^31-1 with 22003 (numeric_value_out_of_range), which currently surfaces as generic error. Field-level precision opportunity; same shape as Codex MED #5.
- **Gemini LOW #2** (destructive upsert for omitted CSV columns): LEGITIMATE concern but partly addressed by D5-18 (skill MERGE) + D5-03 (empty cell = field absent). For JSON, omitted optional fields are nil pointers → upsert writes NULL → wipes existing data. This is PUT semantics on non-skill fields. Documenting the behaviour explicitly in OpenAPI is the fix; or implementing real PATCH semantics for imports (separate concern; v0.2 candidate).

### Composite Findings (consolidated by severity)

#### HIGH (3 confirmed) — BLOCK Phase 5 ship

| # | Source | Site | Type | Brief |
|---|--------|------|------|-------|
| H1 | Codex | catalog/*.go (6 Update handlers) | Bug | PATCH 23505 → 500 instead of 409 |
| H2 | Codex + Gemini | db/queries/*.sql (6 UPDATE queries) | Contract drift | PATCH `external_id: null` cannot clear |
| H3 | Codex | imports/handler_import.go:415 | Bug | `finaliseJob` failure → admin sees wrong state |

#### MED (5 confirmed, 2 reclassified to LOW, 2 false positives)

| # | Source | Site | Type | Brief |
|---|--------|------|------|-------|
| M1 | Codex | imports/chunk.go:303 | Bug | Savepoint RELEASE failure not rolled back |
| M2 | Codex | imports/chunk.go:273 | Bug | Savepoint BEGIN failure cascades silently |
| M3 | Codex | imports/header.go:256 + handler_import.go:574 | Precision | Malformed adapter `config` CSV → generic field |
| M4 | Codex | imports/row_agent.go:181 + missing validate | Precision | Proficiency 1..10 not validated pre-DB |
| M5 | Gemini | imports/idempotency.go:89 | Contract | Replay always 200 (even when original was 207/422) |
| M6 | Gemini (reclassified) | catalog/*.go (6 Update handlers, ErrNoRows branch) | Stale state | 409 `Current` is pre-UPDATE snapshot, not latest |

False positives (reject):
- Gemini MED #1 (cache key break_reasons) — `string(BreakReasons) == "break_reasons"` ✓
- Gemini MED #3 (priority default 5) — OpenAPI has `example: 5` not `default: 5` ✓

#### LOW (4 confirmed, 1 cosmetic-reject)

| # | Source | Site | Type | Brief |
|---|--------|------|------|-------|
| L1 | Codex | imports/doc.go | Doc nit | Stale "Wave 3/4 will add" comment |
| L2 | Codex | imports/row_agent_test.go:265 | Test gap | `t.Skip` on agent_state_seed_failed path |
| L3 | Gemini | imports/coerce.go:94 | Precision | parseInt no int32 bounds check |
| L4 | Gemini | imports/row_*.go | UX | CSV-omitted columns wipe existing values (PUT semantics) |

Reject:
- Codex LOW #1 (.gitattributes comment) — intentional documentation; not noise.

### Verdict Reconciliation

- **Codex:** `Verdict: BLOCK`
- **Gemini:** `Verdict: BLOCK`
- **Combined disposition:** **BLOCK**
- **Rationale:** Per CLAUDE.md HARD RULE — "If either reviewer says BLOCK, do not proceed without addressing." Both reviewers independently flagged BLOCK; HIGH #2 was double-confirmed (both reviewers same finding). All 3 HIGH concerns are real bugs verified against the code. Phase 5 cannot ship until these are addressed.

## Phase 04.1 Review Carry-Forward

This combined review **SATISFIES** Phase 04.1 Plan 06's deferred cross-AI peer review (per Phase 04.1 04.1-REVIEW.md § "Status: DEFERRED to combined Phase 04.1 + Phase 5 review"). The findings break down by phase as follows:

### Phase 04.1-attributable HIGH concerns

- **H1 — PATCH 23505 → 500 mapping** is rooted in Phase 04.1: the entire 6-entity Update-handler MapPgError pattern was introduced by Phase 04.1 (constraint-name introspect for `duplicate_code` / `duplicate_external_id`). The Wave 5 fix was applied to CREATE handlers only; UPDATE handlers were missed.

- **H2 — PATCH `external_id: null` cannot clear** is rooted in Phase 04.1: Phase 04.1 promoted `external_id` to nullable and documented the "null to clear" semantic in OpenAPI, but the COALESCE pattern preserves existing values for nil-pointer params.

- **M6 — 409 `Current` is stale snapshot** is rooted in Phase 04.1: the Layer 2 pre-UPDATE fetch optimization was added in Phase 04.1 for code-immutability enforcement (D04_1-15 / D04_1-20).

### Phase 5-attributable concerns

- **H3** (finaliseJob) and all the M1..M5, L1..L4 imports-package findings are Phase 5 work.

### Phase 04.1 carry-forward disposition

Phase 04.1's deferral was contingent on "no findings rooted in Phase 04.1 emerging." Three HIGH/MED findings ARE rooted in Phase 04.1 (H1, H2, M6). Per Phase 04.1's own deferral notice: "If the combined review returns BLOCK on a finding rooted in Phase 04.1, a follow-up gap-closure plan against the merged Phase 04.1 branch (or main, if already merged) will be required — same HARD RULE escalation path as a same-phase review."

Phase 04.1 is **NOT yet merged** (worktree-agent-a0611fc2373a2544f is still the active branch). The natural remediation path is to roll all fixes into a single Wave 6 follow-up plan that lands on the same Phase 04.1 + Phase 5 PR group BEFORE the chained PR is merged, rather than landing two phases with known regressions and a follow-up patch sequence.

## Action Items

- [x] Cross-AI review executed; both Codex and Gemini verdicts captured.
- [x] HIGH/MED findings verified against the actual code (3 HIGH confirmed real; 2 MED false positives rejected).
- [x] Phase 04.1 deferred review satisfied (Plan 06 carry-forward closed).
- [ ] **BLOCK Phase 5 ship gate.** The orchestrator's Task 3 human checkpoint inherits this BLOCK; user disposition required.
- [ ] **Wave 6 follow-up plan required** to address all 3 HIGH + 5 MED concerns BEFORE the Phase 04.1 + Phase 5 PR group merges to main. Recommended plan name: **`05-09-FIXUP-PLAN.md`** (or equivalent under Phase 04.1 + 5 deferral umbrella).
  - **Wave 6 Task 1:** Fix H1 — 6 Update handlers: add `if status == 409 { return Update*409JSONResponse(...) }` branch before the 500 fallthrough. Tests: 6 new test cases asserting PATCH duplicate-external-id → 409 (one per entity).
  - **Wave 6 Task 2:** Fix H2 — switch the 6 UPDATE SQL queries from `COALESCE(narg, col)` to a tri-state pattern (`pgtype.Text` with explicit `Valid` field, dynamic SET clause, or a `external_id_set bool` companion param). Tests: 6 new test cases asserting PATCH `external_id: null` clears the column.
  - **Wave 6 Task 3:** Fix H3 — `finaliseJob` failure must return 500 (the import landed, but the audit row is corrupt; admin retry is the recovery). Alternative: best-effort retry once, then 500 on persistent failure. Tests: integration test with mocked finalise failure asserting 500.
  - **Wave 6 Task 4:** Fix M1+M2 — chunk loop: any savepoint BEGIN or RELEASE failure aborts the chunk, rolls back outer tx, and marks all in-flight rows as failed. Tests: testcontainer test with deliberate savepoint failure injection.
  - **Wave 6 Task 5:** Fix M3 — adapter `config` JSON shape failure surfaces as `{field: "config", reason: "invalid_json"}` not generic `invalid_json_row`. Tests: malformed `config` CSV.
  - **Wave 6 Task 6:** Fix M4 — import row processors call `validateProficiencyRange` BEFORE MergeAgentSkill; surface as `{field: "skills[i].proficiency", reason: "invalid_value"}`. Also fix the misleading "0..100" comment to "1..10". Tests: out-of-range proficiency case.
  - **Wave 6 Task 7:** Fix M5 — `rehydrateBulkImportResult` reads succeeded_rows / failed_rows and chooses the matching 200/207/422 response wrapper. Tests: replay of 207-result job returns 207.
  - **Wave 6 Task 8 (optional):** Fix M6 — when `stored != nil` in the ErrNoRows branch, run a fresh `GetByIdAnyVersion` JUST to repopulate `Current` for the 409 response (status code remains correct; only the response body freshens). Lower priority.
  - **Wave 6 Task 9 (optional):** Address L1 + L2 + L3 + L4 doc/precision/UX nits in the same Wave (low cost; high cleanup value).
- [ ] **Ship gate decision** — orchestrator HALTS Phase 5 advance at the human checkpoint. User decides:
  - **Option A (recommended):** Authorize Wave 6 follow-up plan (per above); Phase 5 ships AFTER fixes land + a re-run of this combined review returns READY/READY-WITH-FIXES.
  - **Option B:** Override the BLOCK (user judgement on severity); land Phase 5 with known HIGH issues + open hard-coded tracking issues for each.
  - **Option C:** Reject Phase 5 entirely; trigger a different remediation flow.

---

*Plan 05-08 / Task 2 — Combined cross-AI review COMPLETE. Combined disposition: **BLOCK**. Phase 04.1 carry-forward: deferred review SATISFIED; 3 findings rooted in Phase 04.1 require remediation. Phase 5 ship halted at human checkpoint per CLAUDE.md HARD RULE.*
