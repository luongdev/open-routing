---
phase: 3
reviewers: [gemini, codex]
reviewed_at: 2026-05-16T12:57:50Z
plans_reviewed:
  - 03-01-PLAN.md
  - 03-02-PLAN.md
  - 03-03-PLAN.md
  - 03-04-PLAN.md
  - 03-05-PLAN.md
  - 03-06-PLAN.md
  - 03-07-PLAN.md
  - 03-08-PLAN.md
  - 03-09-PLAN.md
  - 03-10-PLAN.md
---

# Cross-AI Plan Review — Phase 3 (Catalog CRUD, Go)

## Gemini Review

This review covers implementation plans **03-01** through **03-10** for Phase 3: Catalog CRUD (Go).

## Phase-Level Assessment
Phase 3 is the most significant backend milestone for v0.1, transitioning the project from an infrastructure-only scaffold to a functional data management platform. The plans demonstrate a deep understanding of the established `orgDB` isolation patterns and the strict-server contract. 

The strategy of using **Agents** as the canonical template (Plan 03-06) before fanning out to other entities is an excellent de-risking move. The inclusion of a generic, singleflight-protected cache layer and a transactional `OrgTx` wrapper shows a high level of engineering maturity for a v0.1 foundation.

---

## Individual Plan Reviews

### 03-01: Wave 0 Foundations + Spec Amendment
*   **Summary**: bootstraps the phase by amending the OpenAPI spec to support 422 error codes, adds versioning to all entities, deletes the scaffold, and manages dependencies.
*   **Strengths**: Correctly identifies the need to remove spec-level `min/max` constraints on proficiency to allow the handler to return 422 `invalid_value` instead of a 400 `invalid_body`. early cleanup of scaffold code prevents drift.
*   **Concerns**: [LOW] Dependency on `miniredis` version. The plan handles the slopcheck false-positive well with a human-verify task.
*   **Suggestions**: None.
*   **Risk**: LOW.

### 03-02: Migration 002 (Catalog Tables)
*   **Summary**: Defines the single editable migration for v0.1 catalog tables, indexes, and soft-delete columns.
*   **Strengths**: Adheres strictly to the "one migration for v0.1" rule. Correct use of partial functional indexes for name search. Critical denormalization of `org_id` on `agent_skills` is present.
*   **Concerns**: [LOW] Partial index on `lower(name)` doesn't support substring search, but this is explicitly documented as a v0.1 trade-off.
*   **Risk**: LOW.

### 03-03: sqlc Queries + OrgDB.BeginTx
*   **Summary**: Authoring sqlc query files and extending `OrgDB` with a transactional wrapper that preserves the SQL validator.
*   **Strengths**: The `OrgTx` implementation is a highlight; it ensures that even multi-statement transactions cannot bypass the `org_id` isolation guard (FOUND-04). Use of `sqlc.narg` for cursors correctly handles nullable inputs.
*   **Concerns**: [MEDIUM] `unnest($1::uuid[])` can sometimes lead to type mismatches in sqlc if not typed exactly as the driver expects (`[]pgtype.UUID`).
*   **Risk**: LOW.

### 03-04: internal/cache/ Package
*   **Summary**: Implements a generic, singleflight-backed Redis cache with refresh-ahead logic.
*   **Strengths**: Use of `context.WithoutCancel` for refresh goroutines prevents context-deadline-induced cache stampedes. Clean separation of concerns.
*   **Concerns**: None.
*   **Risk**: LOW.

### 03-05: Catalog Skeleton
*   **Summary**: Scaffolds the catalog package with all 38 methods to satisfy the generated interface.
*   **Strengths**: the `var _ api.StrictServerInterface = (*Handlers)(nil)` guarantee provides immediate feedback if a method is missing. The `notImplemented` helper prevents build breakage during entity fan-out.
*   **Concerns**: [LOW] High file count (11 files) but necessary for the per-entity layout (D-68).
*   **Risk**: LOW.

### 03-06: Agents End-to-End (Template)
*   **Summary**: Implements the first complete entity (Agents) with caching, versioning, and N:M join logic.
*   **Strengths**: Solves the `CreateAgent409` union type pitfall. Implements the complex `replaceAgentSkills` logic transactionally.
*   **Concerns**: [MEDIUM] The plan notes "best-effort" rollback for skills in `CreateAgent`. Since `BeginTx` is available from Plan 03-03, the Agent creation and skill assignment should be wrapped in a single transaction from the start.
*   **Risk**: MEDIUM. This is the most logic-heavy plan in the phase.

### 03-07: Skills + BreakReasons
*   **Summary**: Implements two simple CRUD entities following the agent template.
*   **Strengths**: Minimal logic drift from template. Correctly omits `external_id` for BreakReasons per spec.
*   **Risk**: LOW.

### 03-08: Queues + Channels (FK Probes)
*   **Summary**: Implements Queues and Channels, exercising the D-76 cross-row validation pattern.
*   **Strengths**: Test coverage for cross-org FK probes (`default_queue_id`) is a vital verification of the system's security posture.
*   **Risk**: LOW.

### 03-09: Adapters + agent_skills Helpers
*   **Summary**: Implements Adapters (JSONB) and extracts shared join helpers.
*   **Strengths**: Properly handles the `[]byte` vs `map[string]any` mapping for JSONB. Refactoring `agents.go` to use extracted helpers improves maintainability.
*   **Risk**: LOW.

### 03-10: Main Wiring + Isolation Suite
*   **Summary**: Final assembly, bypass route implementation, and extension of the isolation test suite.
*   **Strengths**: Exhaustive cross-org tests for every entity (6 entities + 2 FK probes) provides high confidence in FOUND-08 compliance.
*   **Risk**: LOW.

---

## Key Performance Indicators (Phase 3)

| Focus Area | Status | Notes |
| :--- | :--- | :--- |
| **Multi-Org Isolation** | ✅ EXCELLENT | Every query filtered; every probe org-scoped; isolation suite extended to all entities. |
| **Consistency** | ✅ EXCELLENT | Template approach ensures all 6 CRUDs behave identically regarding versioning/caching/pagination. |
| **Validation** | ✅ STRONG | Two-layer validation (schema + cross-row) is correctly implemented; 422 response wrappers correctly wired. |
| **Concurrency** | ✅ STRONG | Atomic UPDATE with RETURNING is used everywhere. 409 disambiguation logic is robust. |
| **Environment** | ⚠️ CAUTION | System Go 1.23 vs go.mod 1.25. Agent must use the provided Taskfile/Docker lanes to ensure build success. |

## Suggestions for Execution
1.  **Transactional Create**: In Plan 03-06, wrap `InsertAgent` and `replaceAgentSkills` in a single transaction using the new `OrgDB.BeginTx` method to ensure atomicity during creation.
2.  **sqlc Type Alignment**: After Task 1 of Plan 03-03, immediately run `task gen` and verify the `[]pgtype.UUID` mapping for the `unnest` parameters before proceeding to handler implementation.
3.  **Bypass Wrapper Names**: In Plan 03-10, take extra care with the `application/x-yaml` response wrapper name, as `oapi-codegen` naming for non-JSON types is often unintuitive.

---

## Overall Verdict

**READY TO EXECUTE**

The plans are highly granular, technically accurate, and perfectly aligned with the architectural mandates of the project. The phase provides a solid foundation for the subsequent Agent State Machine (Phase 4) and Bulk Import (Phase 5) workstreams.

---

## Codex Review

# Plan Review — Phase 3 Catalog CRUD

Đã đối chiếu các plan với context D-49..D-77 và RTK.md. Nhận định chung: kế hoạch có cấu trúc tốt, nhưng hiện có nhiều blocker thực thi thật, chủ yếu ở OpenAPI/handler mismatch, PATCH semantics, transaction boundary, wave dependency, và thời điểm xóa scaffold.

## 03-01 — OpenAPI + Scaffold Deletion

**Summary** — Ý tưởng Wave 0 đúng: sửa spec, regen codegen, xóa scaffold. Nhưng plan này đang cố giữ build xanh bằng temp stubs trong khi test/isolation vẫn còn phụ thuộc scaffold, nên rủi ro compile/vet rất cao.

**Strengths**
- Nhận diện đúng `invalid_reference`, `invalid_value`, Channel/Adapter `version`, `limit` default 25.
- Có drift gate `task gen && git diff`.
- Có request-id exhaustiveness awareness.

**Concerns**
- [HIGH] Xóa scaffold/generated scaffold ở 03-01 nhưng port isolation/testsupport tới 03-10; `go vet ./...` nhiều khả năng fail do test refs cũ.
- [HIGH] CreateAgent có `skills[]` trong later plans nhưng 03-01 không thêm `CreateAgent422JSONResponse`.
- [HIGH] `invalid_value` response shape chưa nhất quán với `ErrorResponse`; later plan dùng `api.Error{Code, Field, Message}` có thể không tồn tại.
- [MEDIUM] `Wave0TempStubs` là workaround lớn, dễ lệch với StrictServerInterface và không nằm rõ trong frontmatter.
- [MEDIUM] D-69 nói 501 stubs, plan dùng 500.

**Suggestions**
- Port/delete scaffold tests cùng 03-01, hoặc hoãn scaffold deletion tới khi catalog skeleton build được.
- Thêm 422 cho `CreateAgent` nếu CreateAgent nhận `skills[]`.
- Chốt lại schema `ErrorResponse`: nếu cần `field/message`, thêm vào OpenAPI ngay trong 03-01.
- Ưu tiên giữ old stubs tới 03-05 thay vì tạo `Wave0TempStubs`.

**Risk Assessment** — HIGH: dễ fail build ngay sau Wave 0.

## 03-02 — Migration

**Summary** — Migration bao quát đủ bảng chính, nhưng có path inconsistency và thiếu index quan trọng cho `break_reasons` name search.

**Strengths**
- Một migration v0.1 đúng D-61.
- Universal `version`, `enabled`, cursor index, `agent_skills.org_id`.
- Không thêm FK DB, đúng D-76.

**Concerns**
- [HIGH] Migration path mâu thuẫn giữa context (`services/api/internal/db/migrations`) và plan (`migrations/`).
- [HIGH] `break_reasons` thiếu partial functional name index dù CAT-10 áp dụng mọi entity.
- [MEDIUM] `task db:reset` URL `${DATABASE_URL%\?*}/postgres...` có thể tạo URL sai.
- [MEDIUM] `agent_skills` PK không gồm `org_id`; UUID global thường ổn nhưng kém rõ org-boundary.
- [LOW] Không có trigger `updated_at`; handler phải luôn set đúng.

**Suggestions**
- Chốt duy nhất migration directory trước khi execute.
- Thêm `ix_break_reasons_org_name`.
- Viết `db:reset` bằng URL parser hoặc `docker compose exec postgres psql -d postgres`.
- Cân nhắc PK `(org_id, agent_id, skill_id)`.

**Risk Assessment** — MEDIUM/HIGH: schema gần đúng, nhưng path/index/db reset có thể làm Wave 1 fail.

## 03-03 — sqlc + OrgDB.BeginTx

**Summary** — Query coverage tốt, nhưng Wave dependency và PATCH/update design chưa chắc đúng.

**Strengths**
- Per-entity query files đúng D-62.
- D-66 UPDATE + fallback SELECT được chuẩn hóa.
- `OrgDB.BeginTx` là cần thiết cho CAT-03.

**Concerns**
- [HIGH] Plan nói song song với 03-02 nhưng `sqlc generate` cần migration 002 trên disk.
- [HIGH] UPDATE query nhận full concrete fields; handlers later dùng zero fallback cho missing PATCH fields, gây data loss.
- [MEDIUM] `sqlc.narg(...)` trộn với positional `LIMIT $2` dễ sinh params khó đoán.
- [MEDIUM] `db` package import `generated` chỉ để assert DBTX làm tăng coupling.
- [LOW] Acceptance grep “mọi query mention org_id” yếu, không chứng minh SQLChecker pass.

**Suggestions**
- Cho 03-03 depend trực tiếp 03-02, hoặc split 03-02 Task 1 thành Wave 0.5.
- Chọn PATCH-safe SQL: `COALESCE(sqlc.narg(...), column)` hoặc handler read-merge-update.
- Dùng named args cho `limit`, `cursor_id`, `name_filter`.
- Test SQLChecker bằng generated queries thực tế, không chỉ grep.

**Risk Assessment** — HIGH: dependency và PATCH semantics là blocker.

## 03-04 — Cache Package

**Summary** — Cache plan bám D-49..D-60 khá tốt. Rủi ro chủ yếu là chi tiết implementation nhỏ và goroutine lifecycle.

**Strengths**
- `GetOrSet[T]`, `singleflight`, no negative cache, refresh-ahead đúng quyết định.
- Test plan đủ: hit/miss/not-found/refresh/del/singleflight.
- Key format `or:{org}:{entity}:{id}` rõ.

**Concerns**
- [MEDIUM] Refresh goroutine dùng `WithoutCancel` nhưng không có timeout riêng; có thể treo nếu loader/Redis treo.
- [LOW] Code mẫu có nguy cơ lỗi scope biến `jerr` nếu copy máy móc.
- [LOW] Không guard nil `Cache`, nil `rdb`, nil `logger`.

**Suggestions**
- Trong refresh goroutine dùng `context.WithTimeout(context.WithoutCancel(ctx), 5s/10s)`.
- Thêm nil checks trong `New`.
- Thêm test corrupted JSON → loader reload.

**Risk Assessment** — LOW/MEDIUM: thiết kế ổn, cần harden implementation.

## 03-05 — Catalog Skeleton

**Summary** — Skeleton giúp chia việc tốt, nhưng test utilities đang đặt sai file type và bypass ownership chưa sạch.

**Strengths**
- `Handlers` implements `StrictServerInterface` compile-time.
- File layout theo D-68.
- Cursor helper/test tốt.

**Concerns**
- [HIGH] `testutil.go` không phải `_test.go` nhưng later import `miniredis`, `testing`, `testify`; sẽ thành production build deps.
- [MEDIUM] Catalog package gánh `/healthz`, `/docs`, `/openapi.yaml`; kiến trúc hơi lệch.
- [MEDIUM] Placeholder 500 thay vì 501 lệch D-69.
- [LOW] StrictServerInterface method count 38 cần verify sau regen, không hardcode.

**Suggestions**
- Đổi `testutil.go` thành `testutil_test.go`.
- Hoặc giữ bypass ở server layer/top-level handler wrapper thay vì catalog.
- Nếu muốn 501 thật, thêm 501 response vào OpenAPI cho deferred endpoints.

**Risk Assessment** — MEDIUM/HIGH: production/test boundary sai là blocker nhỏ nhưng dễ sửa.

## 03-06 — Agents

**Summary** — Agents là template quan trọng nhưng hiện có lỗi semantics lớn: PATCH overwrite, transaction boundary, CreateAgent skills atomicity, và proficiency 400/422 mismatch.

**Strengths**
- Bao phủ GET cache, D-66 409, soft-delete, cursor, cross-org.
- Có ý thức `CreateAgent409` union.
- Có agent_skills full replace concept.

**Concerns**
- [HIGH] PATCH dùng `derefOr(..., zero)` sẽ xóa name/email/enabled nếu field absent.
- [HIGH] Agent UPDATE và skills replace không cùng transaction; có thể update agent thành công rồi skills fail.
- [HIGH] CreateAgent + skills không atomic và cần `CreateAgent422` wrapper.
- [HIGH] Test proficiency out-of-range kỳ vọng 400, mâu thuẫn user decision/03-01/03-09 là 422 `invalid_value`.
- [MEDIUM] Create write không consistently `cache.Del` dù D-49/D-55 nói write nào cũng invalidate.
- [MEDIUM] Skills reload after update bỏ qua error.

**Suggestions**
- Sửa PATCH semantics trước khi copy sang entity khác.
- Wrap `UpdateAgent` row update + skills delete/insert trong một `OrgTx`.
- Hoặc bỏ skills khỏi CreateAgent; nếu giữ, validate + insert agent + skills trong một tx.
- Chuyển proficiency validation vào 03-06 hoặc bỏ test tới 03-09, nhưng final phải là 422.

**Risk Assessment** — HIGH: không nên dùng làm template khi còn data-loss và atomicity bug.

## 03-07 — Skills + Break Reasons

**Summary** — CRUD đơn giản và phù hợp để copy template, nhưng sẽ inherit lỗi PATCH từ agents nếu không sửa trước.

**Strengths**
- Scope hợp lý: hai entity không FK.
- Có nullable `description`, `routable`, collision tests.
- Cache slugs đúng, đặc biệt `break_reasons`.

**Concerns**
- [HIGH] Inherits destructive PATCH update pattern.
- [MEDIUM] `break_reasons` ordering chưa chốt: display_order hay cursor order.
- [MEDIUM] `CreateBreakReason409`/`CreateSkill409` wrapper phải verify thực tế; spec có thể chưa có create 409.
- [LOW] `description` nil vs empty string semantics cần test rõ.

**Suggestions**
- Sau khi sửa generic PATCH pattern, copy sang plan này.
- Chốt list ordering: CAT-10 cursor default nên là `(created_at,id)`; display_order sort nếu cần thì thêm explicit param/future task.
- Verify generated create conflict wrappers trước khi plan hóa code shape.

**Risk Assessment** — MEDIUM/HIGH: đơn giản nhưng phụ thuộc template đang lỗi.

## 03-08 — Queues + Channels

**Summary** — Channels FK validation đúng hướng, nhưng PATCH preservation và nullable `default_queue_id` cần thiết kế rõ.

**Strengths**
- Bundling queues/channels hợp lý do FK relationship.
- Có test 422 cho missing/disabled/cross-org queue.
- `channel_types[]` round-trip được nêu rõ.

**Concerns**
- [HIGH] Inherits destructive PATCH; absent `default_queue_id` có thể bị set NULL/zero sai.
- [MEDIUM] Channel 422 wrappers phụ thuộc 03-01; cần verify exact generated types.
- [MEDIUM] TOCTOU được accept, nhưng cần comment + future remediation rõ.
- [LOW] Không thấy length/maxItems guard cho `channel_types`.

**Suggestions**
- PATCH update phải distinguish absent vs explicit null vs value.
- Add tests: PATCH channel name only preserves `default_queue_id`; PATCH default_queue_id null clears it only if spec allows.
- Keep disabled/cross-org queue tests as required.

**Risk Assessment** — MEDIUM/HIGH: FK idea tốt, update semantics cần sửa trước.

## 03-09 — Adapters + Agent Skills

**Summary** — JSONB adapter work và agent_skills extraction hợp lý, nhưng dependencies và proficiency rollout chưa sạch.

**Strengths**
- Nhận diện đúng JSONB `[]byte` ↔ `map[string]any`.
- Agent skills helpers cô lập tốt.
- Có tests cho cross-org skill and full replace.

**Concerns**
- [HIGH] 03-09 tests POST skills nhưng plan không depend 03-07; Wave 4 parallel sẽ fail nếu skills vẫn placeholder.
- [HIGH] Proficiency 422 validation tới quá muộn so với 03-06 CAT-03 claims/tests.
- [HIGH] Adapter PATCH preserve config nhưng các field khác vẫn có nguy cơ zero-overwrite.
- [MEDIUM] `invalid_value` response construction dùng `api.Error` không chắc khớp generated schema.
- [MEDIUM] “AtomicityOnInsertFailure” test không thật sự test mid-tx rollback; chỉ test pre-validation.
- [LOW] JSON number deep equality cần xử lý `float64`.

**Suggestions**
- Add `depends_on: [03-07]` hoặc seed skills trực tiếp qua DB.
- Move proficiency validation into 03-06, hoặc make 03-06 explicitly partial and remove conflicting tests.
- Apply same PATCH-safe merge pattern to Adapter fields.
- Add a true tx rollback test via injectable tx/failing query if feasible.

**Risk Assessment** — HIGH: dependency and CAT-03 behavior mismatch block parallel execution.

## 03-10 — Wiring + Isolation

**Summary** — Final wiring intent đúng, nhưng bypass implementation and scaffold-test timing need cleanup.

**Strengths**
- Removes temp stubs and wires real catalog.
- Adds cross-org probes for all 6 entities plus FK probes.
- Final full-suite/drift gates are appropriate.

**Concerns**
- [HIGH] `/openapi.yaml` via `api.GetSpec()` + `yaml.Marshal` may not equal embedded original spec; also adds dependency. Better serve raw `SpecBytes`.
- [HIGH] Scaffold test cleanup is too late if 03-01 already requires `go vet ./...`.
- [MEDIUM] Catalog owning docs/health is architectural leakage.
- [MEDIUM] miniredis process in TestMain needs cleanup.
- [LOW] `task ci` final gate okay, but earlier waves should not claim `go vet ./...` before scaffold tests are ported.

**Suggestions**
- Pass `SpecBytes` into catalog deps or keep bypass in server layer.
- Move scaffold isolation/testsupport cleanup to 03-01.
- Keep `catalog_test.go` probes; add assertion response body error code for 422 FK tests.

**Risk Assessment** — MEDIUM/HIGH: final state can be good, but sequencing is currently broken.

## Phase-Level Review

**Summary** — The phase plan is comprehensive and has strong acceptance criteria, but it is not ready to execute. The biggest issue is not missing coverage; it is inconsistent contracts across OpenAPI, generated wrappers, handler behavior, and wave order.

**Strengths**
- Strong decomposition by entity and reusable patterns.
- Good attention to CAT-08, CAT-09, CAT-10, CAT-11.
- Cache design is pragmatic and testable.
- Cross-org isolation is treated as a first-class acceptance gate.
- D-76 app-level FK validation is consistently intended.

**Concerns**
- [HIGH] OpenAPI/handler/test mismatch for `skills[].proficiency`: final desired behavior is 422 `invalid_value`, but 03-06 still expects 400.
- [HIGH] `CreateAgent` with skills needs 422 spec support or should not accept skills.
- [HIGH] PATCH semantics currently risk overwriting omitted fields with zero values across every entity.
- [HIGH] Agent update + skills replace must be one transaction; current plan splits them.
- [HIGH] Wave parallelism is wrong: 03-03 needs migration file from 03-02; 03-09 needs skills endpoint from 03-07.
- [HIGH] Scaffold deletion happens before scaffold-dependent tests are ported.
- [HIGH] Test-only helper files must be `_test.go` or build-tagged.
- [MEDIUM] Migration directory/path must be normalized before execution.
- [MEDIUM] Bypass serving should reuse raw spec bytes, not reconstructed YAML.

**Suggestions**
1. Replan blockers first: OpenAPI 422 contract, PATCH-safe update semantics, agent+skills transaction, and wave dependencies.
2. Move scaffold test cleanup into 03-01 or delay scaffold deletion until 03-10.
3. Make `testutil.go` → `testutil_test.go`; keep miniredis/testify out of production build.
4. Add `CreateAgent422`, fix 03-06 proficiency expectation to 422, and validate proficiency before any DB call.
5. Normalize migration path and add `break_reasons` name-search index.
6. Rewave: `03-02 -> 03-03`; `03-07 -> 03-09`; keep 03-08 parallel with 03-07 if no file overlap.

**Risk Assessment** — HIGH: plan quality is high in detail, but several contract and sequencing bugs would waste execution time or produce incorrect behavior.

## Overall Verdict

BLOCK — REPLAN — fix OpenAPI/422 consistency, PATCH semantics, agent_skills transaction boundary, scaffold-test sequencing, and wave dependencies before execution.

---

## Consensus Summary

Gemini and Codex agree the **plan structure is high quality** — granular tasks, strong acceptance criteria, comprehensive cross-org isolation gates, correct architectural intent (orgDB, cache, optimistic concurrency, soft-delete, cursor pagination). They diverge sharply on **readiness to execute**.

**Verdicts:**
- **Gemini:** `READY TO EXECUTE` — minor suggestions only.
- **Codex:** `BLOCK — REPLAN` — 7 HIGH-severity concerns spanning OpenAPI contract, PATCH semantics, transaction boundaries, wave dependencies, and scaffold-test sequencing.

### Agreed Strengths
- Use of Agents as canonical template before fanning out (Plan 03-06).
- `OrgTx` + SQLChecker preservation across transactions (Plan 03-03, FOUND-04).
- Atomic UPDATE with version + RETURNING (D-66).
- Two-layer validation: oapi-codegen Layer 1 schema + handler-level Layer 2 cross-row (D-74, D-76).
- Single editable migration v0.1 (D-61).
- Universal `version` + `enabled` + cursor-friendly composite indexes.
- Cache design with `cache.GetOrSet[T]`, singleflight, refresh-ahead.

### Agreed Concerns
- **Migration path consistency** — Codex flags `migrations/` vs CONTEXT-cited `services/api/internal/db/migrations/`. Gemini doesn't flag but a path mismatch will break the test bootstrap.
- **System Go version vs go.mod** — Gemini notes 1.23 (system) vs 1.25 (go.mod); use Taskfile/Docker lanes.

### Codex-Only HIGH Concerns (must triage)

| # | Concern | Severity | Verification needed |
|---|---------|----------|---------------------|
| C1 | 03-06 may still expect 400 for out-of-range proficiency (after revision iter 2 changed to 422 elsewhere) | HIGH | grep Plan 03-06 |
| C2 | CreateAgent with `skills[]` needs 422 spec support; 03-01 added 422 to UpdateAgent only | HIGH | grep Plan 03-01 + spec for CreateAgent422 |
| C3 | PATCH semantics: omitted fields could overwrite with zero values (no COALESCE in UPDATE) | HIGH | grep Plan 03-03 query files for COALESCE |
| C4 | Agent UPDATE + skills replace not in single transaction (only skills replace is) | HIGH | inspect Plan 03-06 + 03-09 |
| C5 | Wave parallelism wrong: 03-03 depends_on 03-01 only, but 03-03 reads the migration file written by 03-02 | HIGH | confirmed — revision iter 1 over-corrected |
| C6 | 03-01 deletes scaffold but scaffold-dependent tests in `services/api/test/isolation/` aren't ported until 03-10 → `go vet` will fail mid-phase | HIGH | grep isolation tests for scaffold refs |
| C7 | `testutil.go` is production-build visible; pulls miniredis/testify into prod binary unless renamed `_test.go` or build-tagged | HIGH | inspect Plan 03-05 + 03-06 file paths |

### Divergent Views
- Gemini called 03-01's slopcheck checkpoint elegant; Codex worried `Wave0TempStubs` is too large a workaround.
- Gemini OK with 03-09 verdict; Codex said it's a HIGH-risk dependency mismatch.
- Gemini OK with wave parallelism; Codex says it's broken.

### Recommended Next Step

Run a **targeted revision pass** addressing the 7 Codex HIGHs (after verifying each against the actual plan text — Codex sometimes hallucinates context, while Gemini sometimes glosses over wiring details). Then re-run plan-checker. The structural strengths both reviewers identified should be preserved.

Alternatively, **accept calculated risk** on the milder concerns and proceed to execute — but C3 (PATCH zero-overwrite), C5 (03-03 wave dep), and C6 (scaffold-test sequencing) are likely to cause real execution failures.
