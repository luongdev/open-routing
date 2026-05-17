---
phase: 5
slug: bulk-import-go
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-17
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Extracted from `05-RESEARCH.md` §Validation Architecture (Nyquist Dimension 8e).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` + `github.com/stretchr/testify/require` + `testcontainers-go` (Phase 1+ pattern) |
| **Config file** | none — `services/api/go.mod` only |
| **Quick run command** | `task test:quick` → `cd services/api && go test -count=1 -short ./internal/imports/...` |
| **Full suite command** | `task test` → `cd services/api && go test -race -count=1 ./...` |
| **Estimated runtime** | ~30s quick / ~90-120s full (testcontainers + race detector) |
| **Skipped under -short** | `services/api/test/isolation/...` and integration tests requiring Postgres |

---

## Sampling Rate

- **After every task commit:** `task test:quick` (covers `internal/imports/` unit tests, sub-30-second target)
- **After every wave merge:** `task test` (full suite including testcontainers Postgres; ~1-2 minutes given Phase 4's full suite ran sub-2min on similar scope)
- **Before `/gsd-verify-work`:** Full suite green + `task gen` produces no diff + isolation suite passes + `task lint` clean
- **Max feedback latency:** ~30 seconds per task commit, ~120 seconds per wave merge

---

## Per-Task Verification Map

| Req ID | Behavior | Test Type | Automated Command | File Exists | Status |
|--------|----------|-----------|-------------------|-------------|--------|
| IMP-01 | POST /catalog/import accepts JSON for each of 6 entities | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_JSON_<entity>` | ❌ Wave 5 | ⬜ pending |
| IMP-01 | POST /catalog/import accepts CSV for each of 6 entities | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_CSV_<entity>` | ❌ Wave 5 | ⬜ pending |
| IMP-02 | UTF-8 BOM stripped; CRLF handled; embedded quotes/commas/newlines decoded | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_WindowsExcel_BOM_CRLF` (`testdata/windows-excel-agents.csv`) | ❌ Wave 5 | ⬜ pending |
| IMP-02 | Invalid UTF-8 → 400 `csv_not_utf8` | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_InvalidUTF8_400` | ❌ Wave 5 | ⬜ pending |
| IMP-03 | **[POST-04.1]** Upsert keyed by `(org_id, code)` via Phase 04.1's `UpsertXByCode`; same payload twice → no duplicates | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_Idempotent_DoubleRun_NoDuplicate` | ❌ Wave 5 | ⬜ pending |
| IMP-03 | **[POST-04.1]** Cross-org same-code-different-rows isolation (mirror `TestCatalog_CrossOrgSameCode_BothSucceed`) | integration | `go test -count=1 ./test/isolation/ -run TestImport_CrossOrgSameCode_BothSucceed` | ❌ Wave 5 | ⬜ pending |
| IMP-04 | Failed rows return structured errors with row + field + message | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_FailedRow_Structure` | ❌ Wave 5 | ⬜ pending |
| IMP-05 | 200 all succeed; 207 partial; 422 all fail | integration | `go test -count=1 ./internal/imports/ -run "TestBulkImport_HTTPStatus_(200\|207\|422)"` | ❌ Wave 5 | ⬜ pending |
| IMP-06 | import_jobs persisted; GET /imports/{id} returns it | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_GetImportJob_RoundTrip` | ❌ Wave 5 | ⬜ pending |
| IMP-07 | 50 MB → 413; 500 rows → 413 | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_Oversize_413` | ❌ Wave 5 | ⬜ pending |
| IMP-08 | CSV requires `?schema_version=v0.1`; mismatch → 400 | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_SchemaVersion_Mismatch_400` | ❌ Wave 5 | ⬜ pending |
| D5-01 | Typed coercion pipeline | unit | `go test -count=1 ./internal/imports/ -run "TestCoerce_(Bool\|Int\|Multi)"` | ❌ Wave 2 | ⬜ pending |
| D5-09 | Chunked-savepoint: per-row failure rollback; chunk-level commit | integration (testcontainers Postgres) | `go test -count=1 ./internal/imports/ -run TestChunk_SavepointRollback` | ❌ Wave 3 | ⬜ pending |
| D5-11 | Crash sweep flips pending → failed after 24h | unit (clockwork) | `go test -count=1 ./internal/imports/ -run TestSweep_PendingOver24h_FlipsToFailed` | ❌ Wave 5 | ⬜ pending |
| D5-13 | Idempotency-Key replay returns prior result; new key proceeds; replay has `idempotent_replay=true` | integration | `go test -count=1 ./internal/imports/ -run "TestIdempotencyKey_(Replay\|MissProceed)"` | ❌ Wave 4 | ⬜ pending |
| D5-15 | **[POST-04.1]** Per-entity Import*Request; FK by `code`; unknown skill_code → unknown_skill | integration | `go test -count=1 ./internal/imports/ -run TestAgentImport_UnknownSkillCode` | ❌ Wave 3 | ⬜ pending |
| D5-15 | **[POST-04.1]** Invalid code format on entity's own `code` → per-row `invalid_code_format` | integration | `go test -count=1 ./internal/imports/ -run TestAgentImport_InvalidCodeFormat_400OrPerRow` | ❌ Wave 3 | ⬜ pending |
| D5-16/17 | **[POST-04.1]** Invalid nested `skill_code` format → per-row `invalid_code_format` BEFORE skill resolution | integration | `go test -count=1 ./internal/imports/ -run TestAgentImport_NestedSkillCode_InvalidFormat_PerRow` | ❌ Wave 3 | ⬜ pending |
| D5-18 | Skill MERGE on update; existing skill not in import retained | integration | `go test -count=1 ./internal/imports/ -run TestAgentImport_SkillMerge_PreservesExisting` | ❌ Wave 3 | ⬜ pending |
| Hazard 7 | New agent import seeds agent_states row (ON CONFLICT DO NOTHING) | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_SeedsAgentStatesForNewAgents` | ❌ Wave 3 | ⬜ pending |
| Hazard 7 (re-import) | Re-importing an agent does NOT reset agent_states to Offline | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_Reimport_PreservesAgentStateMachine` | ❌ Wave 3 | ⬜ pending |
| FOUND-08 | Cross-org isolation: orgA imports, orgB GET /imports/{id} → 404 | integration | `go test -count=1 ./test/isolation/ -run TestImport_CrossOrg_404` | ❌ Wave 5 | ⬜ pending |
| FOUND-08 | **[POST-04.1]** Migration idempotency smoke (mirror `migration_idempotent_test.go`) for new 000003 | integration | `go test -count=1 ./test/isolation/ -run TestMigration_ImportJobs_Idempotent` | ❌ Wave 5 | ⬜ pending |
| Pitfall 4 (cache) | Per-entity `cache.Del` fires only AFTER chunk commit | unit (miniredis) | `go test -count=1 ./internal/imports/ -run TestChunk_CacheInvalidation_PostCommitOnly` | ❌ Wave 3 | ⬜ pending |
| Pitfall 5 (Content-Length lie) | 100MB body with Content-Length:1000 → 413 via `MaxBytesError` | integration | `go test -count=1 ./internal/middleware/ -run TestBodyLimit_LyingContentLength_413` | ❌ Wave 1 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Files NEW to Wave 0 (stub or create before downstream waves can implement):

- [ ] `services/api/internal/imports/coerce_test.go` — covers D5-01 / D5-02 / D5-04 / D5-06 / D5-07 / D5-08
- [ ] `services/api/internal/imports/parser_csv_test.go` — covers IMP-02 / D5-08 / Pitfall 1
- [ ] `services/api/internal/imports/parser_json_test.go` — covers D5-23 (eager-decode-acceptable-with-MaxBytes)
- [ ] `services/api/internal/imports/header_test.go` — covers D5-05 strict header policy
- [ ] `services/api/internal/imports/chunk_test.go` — covers D5-09 / D5-19 / Hazard 7 / Pitfall 10
- [ ] `services/api/internal/imports/jobs_test.go` — covers IMP-06 / D5-10
- [ ] `services/api/internal/imports/idempotency_test.go` — covers D5-13
- [ ] `services/api/internal/imports/sweep_test.go` — covers D5-11 (clockwork-backed)
- [ ] `services/api/internal/imports/handlers_test.go` — entity × format matrix (≥ 12 cases: 6 entities × 2 formats)
- [ ] `services/api/internal/imports/testutil_test.go` — shared httptest harness
- [ ] `services/api/internal/imports/testdata/windows-excel-agents.csv` — UTF-8 BOM + CRLF golden file (Pitfall 1 NON-NEGOTIABLE)
- [ ] `services/api/internal/imports/testdata/<entity>-<scenario>.{json,csv}` — at least 4 scenarios per entity
- [ ] `services/api/internal/imports/testdata/invalid-code-format.csv` — **[POST-04.1]** drives `invalid_code_format` per-row error
- [ ] `services/api/internal/middleware/bodylimit_test.go` — covers D5-21 / Pitfall 5

**No framework install required** — Go stdlib + testify + testcontainers-go already in `services/api/go.mod`.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Excel-exported CSV smoke (real Excel binary) | IMP-02 / Pitfall 1 | Requires Excel binary; CI uses pre-generated golden CSV | Export a sample sheet from Excel as CSV; verify import succeeds; commit the golden file under `testdata/` |
| `task db:reset` smoke after 000003 migration | IMP-06 / D5-09 | Requires local Docker Postgres; CI uses testcontainers | `task db:reset && migrate up; psql -c "\\d+ import_jobs"` — verify `import_jobs` table present with idempotency_key unique index |

---

## Validation Sign-Off

- [ ] All 25 verification-map rows have automated commands OR Wave 0 stubs scheduled
- [ ] Sampling continuity: every IMP-XX requirement has at least one automated test
- [ ] Wave 0 covers all NEW test scaffold files (`coerce_test.go`, `parser_csv_test.go`, etc.)
- [ ] No watch-mode flags in commands (all use `-count=1` for cache bypass)
- [ ] Feedback latency < 30s per task commit, < 120s per wave merge
- [ ] Phase gate includes `task gen && git diff --exit-code` (CONTRACT-04 codegen-drift guard preserved)
- [ ] `nyquist_compliant: true` set in frontmatter after Wave 0 ships

**Approval:** pending — set to `approved 2026-05-XX` once Phase 5 plan-checker re-verifies post-VALIDATION.md add.

---

## Notes (Nyquist Dimension 11 — Research Resolution)

All 7 Open Questions in `05-RESEARCH.md` §Open Questions (RESOLVED) have a `Recommendation:` line that the Phase 5 planner accepted verbatim. Resolution mapping:

| Q | Recommendation | Plan applying it |
|---|---------------|------------------|
| Q1: BulkImportResult.idempotent_replay field | nullable:true, default:false, readOnly:true | 05-01 DELTA 5 |
| Q2: Crash-sweep WithBypass audit volume | accept bypass; revisit if v0.2 multi-org noise | 05-04 (sweep.go) |
| Q3: Inline vs separate openapi components | inline | 05-01 |
| Q4: Migration 000002 amend vs new 000003 | new 000003 | 05-01 Task 2 |
| Q5: ApiHandlers Server-selector ambiguity | `imports.Importer` | 05-04 Task 1, 05-06 Task 3 |
| Q6: 422 vs 200 on empty-rows-after-header | 200 with empty arrays | 05-04 (header.go), 05-07 (test) |
| Q7: bodyLimit middleware ordering | orgContextMiddleware BEFORE BodyLimit | 05-06 Task 4 |
