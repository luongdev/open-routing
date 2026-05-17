---
phase: 05-bulk-import-go
plan: 07
subsystem: api
tags:
  - integration-tests
  - testdata
  - isolation
  - migration-idempotent
  - pitfall-1
  - found-08

# Dependency graph
requires:
  - phase: 05-bulk-import-go/06
    provides: |
      Importer.BulkImportCatalog + Importer.GetImportJob handlers wired
      into ApiHandlers three-embed composite; middleware.BodyLimit in
      server.NewMux chain; catalog/notimpl.go gone. The Wave 5 tests
      exercise this full pipeline end-to-end via httptest.Server.
  - phase: 05-bulk-import-go/05
    provides: |
      processChunk + 6 per-entity rowProcessor impls. Wave 5 tests
      that perform real DB writes (handlers_test.go entity x format
      matrix) route through these.
  - phase: 05-bulk-import-go/04
    provides: |
      testutil_test.go skeleton (sharedPool + cleanImportTables +
      seedPendingImportJob); doc.go invariant index; idempotency.go
      lookup + rehydrate; jobs.go createJob + finaliseJob.
  - phase: 04.1-catalog-identity-normalization/05
    provides: |
      test/isolation/main_test.go + isolation_test.go + catalog_test.go
      patterns (TestCatalog_CrossOrgSameCode_BothSucceed at line 375 —
      mirrored verbatim for the import path); migration_idempotent_test.go
      DDL-replay pattern.
  - phase: 04-agent-state-machine-go/03
    provides: |
      state/testutil_test.go HTTP-helper + mux-bringup template (mirrored
      in Wave 5's testutil_test.go extension).

provides:
  - "internal/imports/testutil_test.go extension: importsApiHandlers three-embed composite + httptest.Server + 9 HTTP helpers (postImportJSON, postImportJSONWithIdempotencyKey, postImportCSV, postImportCSVNoSchemaVersion, postImportCSVWithSchemaVersion, getImportJob, getImportJobAsOrg, loadTestData, decodeBulkImportResult, decodeImportJob, decodeJSONArray, extractFirstSucceededID) + 9 seed/fetch primitives (seedSkillForImports, seedQueueForImports, seedAgentForImports, seedAgentSkillAssignment, fetchAgentByCode, fetchAgentStateStatusByOrg, fetchAgentSkillCount, fetchAgentSkillProficiency, fetchImportJob, countImportJobsByOrg, extractLatestJobID)"
  - "internal/imports/handlers_test.go: 25 entity x format matrix integration tests (12 happy paths + 13 error/merge/IDENT-03 cases)"
  - "internal/imports/handlers_csv_test.go: 7 CSV-spec edge tests (BOM + CRLF + RFC-4180 round-trip + invalid UTF-8 + oversize 501 rows + oversize 50 MB body + schema_version mismatch/missing + nested quoted newline)"
  - "internal/imports/idempotency_test.go: 6 idempotency tests (5 IdempotencyKey cases including cross-org + 1 no-key double-run dedup)"
  - "internal/imports/jobs_test.go: 5 import_jobs lifecycle tests (3 GetImportJob + 2 FinaliseJob)"
  - "internal/imports/testdata/: 12 CSV + 2 JSON golden fixtures (agents-basic, agents-with-skills, skills-basic, queues-basic, channels-basic, adapters-basic, break_reasons-basic, partial-success, full-failure, invalid-code-format, windows-excel-agents [BOM + CRLF + RFC-4180 quote escape], invalid-utf8 [invalid 2-byte UTF-8 sequence])"
  - "test/isolation/imports_test.go: 4 FOUND-08 cross-org probes (CrossOrgGetJob_404 + CrossOrgSameCode_BothSucceed [D04_1-24 mirror for import path] + CrossOrg_Channel_DefaultQueueCode_invalid_reference [D-76 cross-org FK probe] + CrossOrgSameIdempotencyKey_BothProceed [composite UNIQUE on import_jobs])"
  - "test/isolation/imports_migration_idempotent_test.go: 1 DDL-replay idempotency smoke (asserts SQLSTATE 42P07 on every CREATE TABLE / CREATE INDEX replay + pre/post pg_catalog snapshot byte-equivalence)"
  - "Auto-fix in handler_import.go: materialiseTypedRaw now decodes CSV `config` cells (JSON-encoded string) into a typed map[string]interface{} so reMarshalAs[ImportAdapterRequest] sees the right shape (pre-existing bug surfaced by Plan 05-07 Task 1's TestBulkImport_CSV_Adapters_HappyPath_200)"
  - ".gitattributes binary flag for windows-excel-agents.csv + invalid-utf8.csv (prevents repo-level `eol=lf` rule from stripping CRLF + BOM and corrupting the Pitfall-1 invariant on checkout)"

affects:
  - "05-08 — Phase 5 phase-gate plan now has its full automated test surface; the gate confirms `go test -count=1 -short ./...` (352 PASS) + `go test -count=1 ./internal/imports/... ./test/isolation/...` (testcontainer: 138 PASS + 1 SKIP + 32 PASS = 170 PASS + 1 SKIP)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Three-embed composite ApiHandlers in test fixtures matches production (D-89 evolved through Phase 3 -> Phase 4 -> Phase 5); the imports/testutil_test.go bring-up is byte-for-byte identical to test/isolation/main_test.go's mux construction so both surfaces exercise the same chi mux + middleware chain"
  - "Migration idempotency smoke via DDL replay + pg_catalog snapshot diff (mirrors Phase 04.1's backfill-CTE test pattern but adapted for DDL-only migration 000003)"
  - "Cross-org test pattern lifted from Phase 04.1 catalog_test.go TestCatalog_CrossOrgSameCode_BothSucceed (line 375) — the import path inherits the same invariant via UpsertXByCode"
  - "Pitfall-1 golden file authored byte-by-byte via a one-off Go helper that prepends 0xEF 0xBB 0xBF + writes CRLF line endings + uses RFC 4180 quote escape; .gitattributes flags the file as binary so eol=lf normalisation never strips the CRLF"
  - "Failed-row shape regression test (TestBulkImport_FailedRow_Structure_207) iterates the Failed[] slice and asserts each entry has non-zero Row + ErrorCode=import_failed + non-empty Reason — locks the wire shape per IMP-04 / D5-12"

key-files:
  created:
    - "services/api/internal/imports/handlers_test.go — 25 entity x format matrix tests (~640 lines)"
    - "services/api/internal/imports/handlers_csv_test.go — 7 CSV-spec edge tests (~220 lines)"
    - "services/api/internal/imports/idempotency_test.go — 6 idempotency replay tests (~290 lines)"
    - "services/api/internal/imports/jobs_test.go — 5 import_jobs lifecycle tests (~170 lines)"
    - "services/api/internal/imports/testdata/agents-basic.{json,csv}"
    - "services/api/internal/imports/testdata/agents-with-skills.{json,csv}"
    - "services/api/internal/imports/testdata/skills-basic.csv"
    - "services/api/internal/imports/testdata/queues-basic.csv"
    - "services/api/internal/imports/testdata/channels-basic.csv"
    - "services/api/internal/imports/testdata/adapters-basic.csv"
    - "services/api/internal/imports/testdata/break_reasons-basic.csv"
    - "services/api/internal/imports/testdata/partial-success.csv"
    - "services/api/internal/imports/testdata/full-failure.csv"
    - "services/api/internal/imports/testdata/invalid-code-format.csv"
    - "services/api/internal/imports/testdata/windows-excel-agents.csv (BOM + CRLF + RFC-4180 quote escape + embedded comma)"
    - "services/api/internal/imports/testdata/invalid-utf8.csv (0xC3 0x28 invalid 2-byte sequence)"
    - "services/api/test/isolation/imports_test.go — 4 FOUND-08 cross-org probes (~290 lines)"
    - "services/api/test/isolation/imports_migration_idempotent_test.go — 1 migration replay test (~230 lines)"
  modified:
    - "services/api/internal/imports/testutil_test.go — extended with importsApiHandlers composite + httptest.Server + 9 HTTP helpers + 9 seed/fetch primitives"
    - "services/api/internal/imports/handler_import.go — materialiseTypedRaw `config` case decodes JSON-encoded string into map[string]interface{} (auto-fix Rule 1)"
    - ".gitattributes — windows-excel-agents.csv + invalid-utf8.csv flagged as binary so eol=lf doesn't strip CRLF / invalid bytes"
  deleted:
    - "services/api/internal/imports/testdata/.gitkeep — placeholder replaced by 14 real testdata files"

key-decisions:
  - "Three-embed composite in importsApiHandlers (testutil_test.go) mirrors cmd/api/main.go's ApiHandlers exactly so handler integration tests exercise the production middleware chain (BodyLimit + orgContext + RequestID). The catalog.Handlers + state.Server fields are constructed but never directly called by import tests; only their existence is required for the StrictServerInterface compile-time assertion."
  - "materialiseTypedRaw bridges CSV cell maps -> typed JSON shape for reMarshalAs[T]. Adapter config column required special handling because ImportAdapterRequest.Config is *map[string]interface{} but coerceJSONBObject returns string. Added a `config` case that json.Unmarshal's the trimmed string into a map; falls through to omit the field on empty/nil. This was a pre-existing bug (Rule 1 auto-fix) caught by Wave 5's TestBulkImport_CSV_Adapters_HappyPath_200 — would have shipped silently because no Wave 4 dispatch test exercised the adapter CSV path end-to-end."
  - ".gitattributes binary flag for windows-excel-agents.csv + invalid-utf8.csv is non-negotiable. The repo's `* text=auto eol=lf` rule (line 1) would silently strip the CRLF from windows-excel-agents.csv on every checkout, breaking Pitfall-1 round-trip after the first contributor pulls. Documented the rationale in .gitattributes inline comments."
  - "Migration idempotency test uses pg_catalog snapshot diff (columns + indexes + CHECK constraints) rather than naive 'is the table still there' check. golang-migrate already prevents accidental replay via schema_migrations; the test catches schema DRIFT — a future migration author who edits 000003 in-place AFTER it ships would land here with a snapshot mismatch."
  - "FOUND-08 cross-org isolation test for the channel default_queue_code FK probe (TestImport_CrossOrg_Channel_DefaultQueueCode_invalid_reference) asserts orgB cannot reach orgA's queue rows via the import path. This is the D-76 same-org FK probe pattern adapted for cross-org: GetQueueByCode runs inside the savepoint Tx with the importing org's org_id; orgA's queue is filtered out; the channel row fails with per-row reason=invalid_reference."
  - "POST-style helper bytes.NewReader replaced the custom *bytesReader / *eofErr that I initially wrote in idempotency_test.go. The simpler std-lib path is correct and shorter; the custom-type approach would have created subtle EOF-sentinel issues with errors.Is."

patterns-established:
  - "Pattern: HTTP-fixture extension across phase waves — Wave 2 testutil_test.go shipped the bare Importer fixture; Wave 5 extended (not rewrote) it with the httptest.Server + HTTP helpers. Future phase waves follow the same forward-only extension contract."
  - "Pattern: cross-org test placement — per-entity tests in test/isolation/<subsystem>_test.go (one file per phase subsystem); middleware-layer tests in isolation_test.go; migration-idempotency tests in <subsystem>_migration_idempotent_test.go. Phase 5 establishes the file-per-subsystem convention."
  - "Pattern: golden testdata file naming — `<entity>-<scenario>.{csv,json}` keyed by entity then scenario (agents-basic, agents-with-skills, partial-success, full-failure, etc.). The `windows-excel-agents.csv` name signals 'Excel-exported BOM + CRLF golden' so future authors don't accidentally remove the BOM."

requirements-completed:
  - IMP-01
  - IMP-02
  - IMP-03
  - IMP-04
  - IMP-05
  - IMP-06
  - IMP-07
  - IMP-08

# Metrics
duration: 75min
completed: 2026-05-17
---

# Phase 5 Plan 07: Integration Tests + Testdata + Cross-Org Isolation + Migration Idempotency Summary

**Wave 5 of the bulk-import phase ships the full integration test surface — 48 new test functions (25 entity x format matrix + 7 CSV-spec edges + 6 idempotency + 5 import_jobs lifecycle + 4 cross-org isolation + 1 migration idempotency) backed by 14 testdata golden files (12 CSV + 2 JSON), with the Pitfall-1 mandatory `windows-excel-agents.csv` byte-for-byte authored (UTF-8 BOM + CRLF + RFC-4180 embedded quote/comma) and .gitattributes-locked against the repo's `eol=lf` normalisation rule.**

## Performance

- **Duration:** ~75 min
- **Started:** 2026-05-17 20:30 UTC
- **Completed:** 2026-05-17 21:25 UTC
- **Tasks:** 4 commits + this SUMMARY
- **Source files created:** 18 (4 _test.go + 14 testdata fixtures)
- **Source files modified:** 3 (testutil_test.go extension, handler_import.go materialiseTypedRaw fix, .gitattributes binary flag)
- **Files deleted:** 1 (testdata/.gitkeep)

## Accomplishments

- **48 new integration test functions** spread across 6 test files cover the full Wave 5 acceptance matrix:
  - **handlers_test.go (25):** entity x format matrix (12 happy paths) + 13 error/merge/IDENT-03 cases including IMP-04 failed[] structure shape, IMP-05 207/422 dispatch, Hazard 7 agent_states seed, Pitfall 3 re-import state preservation, D5-18 skill merge, D5-16 unknown_skill per-row, D04_1-05 IDENT-03 same-name break_reasons.
  - **handlers_csv_test.go (7):** Pitfall-1 BOM+CRLF+RFC-4180 round-trip with byte-level pre-assert, D5-08 invalid UTF-8 -> 400, D5-22 501-row -> 413, D5-21 50 MB body -> 413, IMP-08 schema_version mismatch+missing -> 400, RFC 4180 quoted-newline round-trip.
  - **idempotency_test.go (6):** D5-13 miss-proceeds, hit-replay (succeeded[]=[] KNOWN LIMITATION), pre-seeded prior failures rehydration, different-body-same-key still replays, cross-org same-key both proceed, no-key double-run -> 2 jobs 1 unique agent set (IMP-03).
  - **jobs_test.go (5):** GetImportJob round-trip, partial-failure errors[] rehydration (D5-12 zero-transformation), 404 not-found, finaliseJob counter updates on happy + partial.
  - **isolation/imports_test.go (4):** FOUND-08 cross-org GetImportJob -> 404, D04_1-24 cross-org same-code both succeed (mirrors Phase 04.1 line 375), D-76 cross-org channel FK probe -> invalid_reference, composite UNIQUE on import_jobs lets cross-org same-key both proceed.
  - **isolation/imports_migration_idempotent_test.go (1):** DDL-replay smoke asserts SQLSTATE 42P07 on every CREATE TABLE / INDEX replay + pg_catalog snapshot byte-equivalence pre/post.

- **14 testdata golden files** authored: 7 entity happy-paths (1 JSON + 1 CSV for agents incl. skills variants) + 4 scenario files (partial-success, full-failure, invalid-code-format) + 2 Pitfall-1 files (windows-excel-agents.csv with BOM+CRLF+RFC-4180 escape, invalid-utf8.csv with the 0xC3 0x28 invalid 2-byte sequence). All non-Pitfall-1 CSVs validated to have NO BOM (T-05-07-01).
- **Auto-fix (Rule 1):** `materialiseTypedRaw` in handler_import.go now decodes CSV `config` cells from JSON-encoded string -> `map[string]interface{}` so the row processor sees the typed shape. Pre-existing bug surfaced by `TestBulkImport_CSV_Adapters_HappyPath_200` — would have shipped silently because no Wave 4 dispatch test exercised the adapter CSV path end-to-end.
- **.gitattributes binary flag** for windows-excel-agents.csv + invalid-utf8.csv prevents the repo's `* text=auto eol=lf` rule from silently stripping CRLF or normalising invalid bytes on checkout. Documented inline.

## Task Commits

Each task was committed atomically:

1. **Task 1 (testutil HTTP harness + matrix tests + testdata):** `2f1ddfe` — test(05-07): add entity x format matrix + HTTP harness + testdata
2. **Task 2 (Pitfall-1 golden + CSV-spec edge tests):** `27d6909` — test(05-07): add Pitfall-1 windows-excel golden + UTF-8/oversize/schema tests
3. **Task 3 (idempotency + jobs lifecycle tests):** `97a605e` — test(05-07): add idempotency replay + import_jobs lifecycle tests
4. **Task 4 (cross-org isolation + migration idempotency):** `cacaaaf` — test(05-07): add cross-org isolation + migration 000003 idempotency

The orchestrator/parent will add the SUMMARY commit alongside.

## Files Created

| File | Lines | Purpose |
|------|------:|---------|
| `services/api/internal/imports/handlers_test.go` | 642 | 25 entity x format matrix integration tests |
| `services/api/internal/imports/handlers_csv_test.go` | 222 | 7 CSV-spec edge tests (BOM + UTF-8 + oversize + schema_version + quoted newline) |
| `services/api/internal/imports/idempotency_test.go` | 295 | 6 Idempotency-Key replay tests |
| `services/api/internal/imports/jobs_test.go` | 173 | 5 import_jobs lifecycle tests |
| `services/api/test/isolation/imports_test.go` | 291 | 4 FOUND-08 cross-org probes |
| `services/api/test/isolation/imports_migration_idempotent_test.go` | 230 | 1 migration 000003 DDL-replay idempotency test |
| **14 testdata golden files** | — | see Testdata Manifest below |

## Files Modified

| File | Change |
|------|--------|
| `services/api/internal/imports/testutil_test.go` | Extended with importsApiHandlers three-embed composite + httptest.Server + 9 HTTP helpers + 9 seed/fetch primitives. Net +500 lines. |
| `services/api/internal/imports/handler_import.go` | materialiseTypedRaw `config` case decodes JSON-encoded string -> map for reMarshalAs[ImportAdapterRequest] (Rule 1 auto-fix). +20 lines. |
| `.gitattributes` | Added binary flag for windows-excel-agents.csv + invalid-utf8.csv. +9 lines. |

## Files Deleted

| File | Reason |
|------|--------|
| `services/api/internal/imports/testdata/.gitkeep` | 8 lines — placeholder. Replaced by 14 real testdata files. |

## Testdata Manifest

| File | Size (bytes) | BOM | CRLF | Purpose |
|------|------:|-----|-----:|---------|
| `agents-basic.csv` | 156 | no | 0 | Happy-path 3 agents (header + 3 LF-terminated rows) |
| `agents-basic.json` | 285 | n/a | n/a | Happy-path 3 agents (JSON array) |
| `agents-with-skills.csv` | 147 | no | 0 | 2 agents with `skills` column `code:prof|code:prof` (D5-17) |
| `agents-with-skills.json` | 366 | n/a | n/a | 2 agents with nested `skills[]` (D5-16) |
| `skills-basic.csv` | 100 | no | 0 | 3 skills (skill_voice, skill_chat, skill_email) |
| `queues-basic.csv` | 110 | no | 0 | 2 queues with pipe-delimited `channel_types` |
| `channels-basic.csv` | 123 | no | 0 | 2 channels referencing default_queue_code=queue_main |
| `adapters-basic.csv` | 192 | no | 0 | 2 adapters with RFC-4180-escaped JSON Config |
| `break_reasons-basic.csv` | 111 | no | 0 | 3 break_reasons, 2 sharing name "Lunch" (IDENT-03 probe) |
| `partial-success.csv` | 172 | no | 0 | 3 valid + 1 invalid (uppercase code) -> 207 |
| `full-failure.csv` | 127 | no | 0 | 3 invalid (all uppercase) -> 422 |
| `invalid-code-format.csv` | 94 | no | 0 | 1 valid + 1 uppercase -> 207 + per-row reason |
| `windows-excel-agents.csv` | 175 | **YES (0xEF 0xBB 0xBF)** | **4** | **Pitfall-1 MANDATORY:** BOM + CRLF + RFC-4180 quote escape + embedded comma |
| `invalid-utf8.csv` | 27 | no | 0 | header valid + row 1 contains 0xC3 0x28 invalid 2-byte sequence |

**BOM verification:** `head -c 3 services/api/internal/imports/testdata/windows-excel-agents.csv | xxd -p` returns `efbbbf` (PASS). All other CSV files have `no_BOM` (T-05-07-01 mitigation satisfied).

**CRLF verification:** `grep -c $'\r$' services/api/internal/imports/testdata/windows-excel-agents.csv` returns 4 (header + 3 data rows; PASS). All other CSV files have 0 CRLF (LF-only; PASS).

## Test Count Breakdown by Area

| Area | File | Count | Coverage |
|------|------|------:|----------|
| Entity x Format Matrix (happy) | handlers_test.go | 14 | 6 entities x {JSON, CSV} + 2 agents-with-skills variants |
| Entity x Format Matrix (error) | handlers_test.go | 11 | partial-success 207, all-fail 422, all-succeed 200, invalid_code_format per-row, missing-column 400, unknown-column 400, unknown-entity 400, Hazard 7 state seed, Pitfall 3 re-import preservation, D5-18 skill merge, D5-16 unknown_skill |
| CSV-spec edge | handlers_csv_test.go | 7 | Pitfall-1 BOM+CRLF+RFC-4180, invalid UTF-8 400, 501-row 413, 50 MB body 413, schema_version mismatch 400, schema_version missing 400, quoted newline round-trip |
| Idempotency | idempotency_test.go | 6 | miss-proceeds, hit-replay-no-new-work, hit-prior-failures, diff-body-same-key, cross-org-same-key, no-key-double-run |
| import_jobs lifecycle | jobs_test.go | 5 | GetImportJob round-trip + partial-failure errors + 404 + finaliseJob happy + finaliseJob partial |
| Cross-org isolation | test/isolation/imports_test.go | 4 | CrossOrgGetJob 404 + CrossOrgSameCode BothSucceed + CrossOrg Channel FK invalid_reference + CrossOrgSameIdempotencyKey BothProceed |
| Migration idempotency | test/isolation/imports_migration_idempotent_test.go | 1 | TestMigration_ImportJobs_Idempotent (DDL-replay + pg_catalog snapshot diff) |
| **TOTAL** | — | **48** | — |

## Per-IMP Requirement Coverage Matrix

| Requirement | Test Functions |
|-------------|----------------|
| **IMP-01** (POST /catalog/import per-entity) | TestBulkImport_{JSON,CSV}_{Agents,Skills,Queues,Channels,Adapters,BreakReasons}_HappyPath_200 (12) |
| **IMP-02** (UTF-8 + BOM + CRLF) | TestBulkImport_WindowsExcel_BOM_CRLF_200, TestBulkImport_InvalidUTF8_400 |
| **IMP-03** (Idempotent dedup) | TestBulkImport_NoIdempotencyKey_DoubleRunCreatesTwoJobs_NoDupesByCode, TestImport_CrossOrgSameCode_BothSucceed |
| **IMP-04** (failed[] structure) | TestBulkImport_FailedRow_Structure_207, TestBulkImport_InvalidCodeFormat_PerRow, TestBulkImport_AgentImport_UnknownSkillCode_PerRow |
| **IMP-05** (HTTP 200/207/422) | TestBulkImport_HTTPStatus_AllSucceed_200, TestBulkImport_HTTPStatus_AllFail_422, TestBulkImport_FailedRow_Structure_207 (207) |
| **IMP-06** (import_jobs persistence + GET) | TestBulkImport_GetImportJob_RoundTrip, TestBulkImport_GetImportJob_PartialFailure_ReturnsErrors, TestBulkImport_GetImportJob_NotFound_404, TestImport_CrossOrgGetJob_404 |
| **IMP-07** (50 MB + 500 row cap -> 413) | TestBulkImport_Oversize501Rows_413, TestBulkImport_Oversize50MBBody_413 |
| **IMP-08** (schema_version) | TestBulkImport_SchemaVersionMismatch_400, TestBulkImport_SchemaVersionMissing_400 |

Every IMP requirement has at least one passing automated test.

## D5-NN Decision Coverage

| Decision | Test Function(s) |
|----------|------------------|
| D5-08 (UTF-8 only) | TestBulkImport_InvalidUTF8_400 |
| D5-10 (createJob before chunk + finaliseJob after) | TestBulkImport_FinaliseJob_OnAllSucceed_UpdatesStatus, TestBulkImport_FinaliseJob_OnPartial_UpdatesCounters |
| D5-12 (errors JSONB shape = failed[] verbatim) | TestBulkImport_GetImportJob_PartialFailure_ReturnsErrors, TestBulkImport_IdempotencyKey_HitReturnsPriorFailures |
| D5-13 (Idempotency-Key replay) | All 5 TestBulkImport_IdempotencyKey_* cases |
| D5-15..D5-19 (skills nested + merge + import wins) | TestBulkImport_{JSON,CSV}_Agents_WithSkills_200, TestBulkImport_AgentImport_SkillMerge_PreservesExisting, TestBulkImport_AgentImport_UnknownSkillCode_PerRow |
| D5-21 (50 MB BodyLimit middleware) | TestBulkImport_Oversize50MBBody_413 |
| D5-22 (500-row streaming cap) | TestBulkImport_Oversize501Rows_413 |
| D04_1-05 (IDENT-03 same-name break_reasons) | TestBulkImport_{JSON,CSV}_BreakReasons_HappyPath_200 (two `name="Lunch"` rows succeed) |
| D04_1-24 (cross-org same-code) | TestImport_CrossOrgSameCode_BothSucceed |
| D-76 (FK probe in savepoint Tx) | TestImport_CrossOrg_Channel_DefaultQueueCode_invalid_reference |
| FOUND-08 (cross-org 404) | TestImport_CrossOrgGetJob_404 |
| Hazard 7 (agent_states seed ON CONFLICT DO NOTHING) | TestBulkImport_SeedsAgentStatesForNewAgents, TestBulkImport_Reimport_PreservesAgentStateMachine |
| Pitfall 1 (BOM + CRLF + Excel) | TestBulkImport_WindowsExcel_BOM_CRLF_200 |
| Pitfall 3 (re-import preserves state) | TestBulkImport_Reimport_PreservesAgentStateMachine |
| Pitfall 4 (cache.Del POST-COMMIT only) | Inherited from Wave 3 `TestProcessChunk_PostCommit_CacheDel_OnlyAfterSuccess` |

## Plan Acceptance Gates

| Gate | Status | Evidence |
|------|--------|----------|
| handlers_test.go ≥ 16 happy-path tests | PASS | grep count = 25 |
| testdata CSVs ≥ 8 | PASS | `ls .../testdata/*.csv` = 12 |
| testdata JSONs ≥ 2 | PASS | `ls .../testdata/*.json` = 2 |
| postImportJSON in testutil_test.go | PASS | grep count = 8 (including 1 doc-line; ≥1 required) |
| postImportCSV in testutil_test.go | PASS | grep count = 7 (≥1 required) |
| windows-excel-agents.csv has BOM | PASS | `head -c 3 ... \| xxd -p` = `efbbbf` |
| windows-excel-agents.csv has CRLF | PASS | `grep -c $'\r$' ...` = 4 |
| TestBulkImport_WindowsExcel_ ≥ 1 | PASS | grep count = 1 |
| TestBulkImport_InvalidUTF8_ ≥ 1 | PASS | grep count = 1 |
| TestBulkImport_Oversize ≥ 2 | PASS | grep count = 2 |
| TestBulkImport_SchemaVersion ≥ 2 | PASS | grep count = 2 |
| TestBulkImport_IdempotencyKey_ ≥ 5 | PASS | grep count = 5 |
| TestBulkImport_NoIdempotencyKey_ ≥ 1 | PASS | grep count = 1 |
| TestBulkImport_GetImportJob_ ≥ 3 | PASS | grep count = 3 |
| TestBulkImport_FinaliseJob_ ≥ 2 | PASS | grep count = 2 |
| TestImport_CrossOrg ≥ 3 | PASS | grep count = 4 |
| TestImport_CrossOrgSameCode_BothSucceed = 1 | PASS | grep count = 1 |
| TestMigration_ImportJobs_Idempotent = 1 | PASS | grep count = 1 |
| go vet ./internal/imports/... clean | PASS | `go vet` returns "No issues found" |
| go build ./... clean | PASS | exit 0 |
| go test -count=1 -short ./... 352 PASS | PASS | All 17 packages pass under -short |
| go test -count=1 ./internal/imports/... (testcontainer) | PASS | 138 PASS + 1 SKIP (the pre-existing AgentStateSeedFails skip) |
| go test -count=1 ./test/isolation/... | PASS | 32 PASS |
| go test -race -count=1 ./... | PASS | 17 packages OK (race detector clean) |
| Total new test functions ≥ 35 | PASS | 48 new tests added (target was ≥ 35) |

### Verification Gate Output

```text
$ grep -c '^func TestBulkImport_' services/api/internal/imports/handlers_test.go         = 25
$ grep -c '^func TestBulkImport_WindowsExcel_' services/api/internal/imports/handlers_csv_test.go = 1
$ grep -c '^func TestBulkImport_InvalidUTF8_' services/api/internal/imports/handlers_csv_test.go  = 1
$ grep -c '^func TestBulkImport_Oversize' services/api/internal/imports/handlers_csv_test.go      = 2
$ grep -c '^func TestBulkImport_SchemaVersion' services/api/internal/imports/handlers_csv_test.go = 2
$ grep -c '^func TestBulkImport_IdempotencyKey_' services/api/internal/imports/idempotency_test.go = 5
$ grep -c '^func TestBulkImport_NoIdempotencyKey_' services/api/internal/imports/idempotency_test.go = 1
$ grep -c '^func TestBulkImport_GetImportJob_' services/api/internal/imports/jobs_test.go = 3
$ grep -c '^func TestBulkImport_FinaliseJob_' services/api/internal/imports/jobs_test.go = 2
$ grep -c '^func TestImport_CrossOrg' services/api/test/isolation/imports_test.go = 4
$ grep -c '^func TestMigration_ImportJobs_Idempotent' services/api/test/isolation/imports_migration_idempotent_test.go = 1

$ head -c 3 services/api/internal/imports/testdata/windows-excel-agents.csv | xxd -p = efbbbf
$ grep -c $'\r$' services/api/internal/imports/testdata/windows-excel-agents.csv = 4
$ ls services/api/internal/imports/testdata/*.csv | wc -l = 12
$ ls services/api/internal/imports/testdata/*.json | wc -l = 2

$ cd services/api && go vet ./... = No issues found
$ cd services/api && go build ./... = Success
$ cd services/api && go test -count=1 -short ./... = 352 PASS in 17 packages
$ cd services/api && go test -count=1 ./internal/imports/... = 138 PASS + 1 SKIP (pre-existing AgentStateSeedFails)
$ cd services/api && go test -count=1 ./test/isolation/... = 32 PASS
$ cd services/api && go test -race -count=1 ./... = 17 packages OK (43s wall, race detector clean)
```

## Decisions Made

- **Three-embed composite in importsApiHandlers (testutil_test.go).** Mirrors cmd/api/main.go's production ApiHandlers exactly. The catalog.Handlers + state.Server fields are constructed but never directly called by import tests; only their existence is required to satisfy StrictServerInterface compile-time. The state.Server is `Start`'d so its sweeper goroutine doesn't accidentally interfere with import sweeps in parallel tests.
- **materialiseTypedRaw `config` case (Rule 1 auto-fix).** Pre-existing bug surfaced by Wave 5's adapter CSV test. ImportAdapterRequest.Config is `*map[string]interface{}` but coerceJSONBObject returns the trimmed JSON string. The fix decodes the string into a map via json.Unmarshal before placing into the materialised raw map; the row processor's reMarshalAs[ImportAdapterRequest] then sees the right shape. Defensive fallback: on decode failure, leave as string and let the row processor surface invalid_json_row.
- **.gitattributes binary flag for windows-excel-agents.csv.** Non-negotiable per Pitfall 1. The repo's `* text=auto eol=lf` rule (line 1) would silently strip the CRLF on every checkout, breaking Pitfall-1 round-trip after the first contributor pulls. The same flag applies to invalid-utf8.csv so the invalid 0xC3 0x28 byte sequence is not "fixed" by git's text-detection.
- **Migration idempotency via pg_catalog snapshot diff.** golang-migrate already prevents accidental double-application via schema_migrations; the test's value is catching schema DRIFT. A future author who edits 000003 in-place AFTER it ships would trigger a snapshot mismatch here. The replay-each-DDL-statement-individually approach (vs. one replay of the whole file) lets the test pin the EXPECTED SQLSTATE per statement so a regression where a CREATE TABLE silently succeeds (e.g., someone added IF NOT EXISTS) is also caught.
- **bytes.NewReader replaces a custom io.Reader / EOF in idempotency_test.go.** Initial draft used a homemade *bytesReader + *eofErr; the std-lib path is correct and shorter. The custom-type approach risked subtle errors.Is issues with the std-lib's io.EOF sentinel.
- **Single shared httptest.Server per test (not per request).** newTestImports constructs one server with one mux; every test in the imports package goes through it. This matches state/testutil_test.go's pattern and avoids the per-request server allocation overhead. Per-test cleanup (t.Cleanup(srv.Close)) prevents goroutine leaks.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] materialiseTypedRaw did not decode CSV `config` strings into typed maps**

- **Found during:** Task 1 (TestBulkImport_CSV_Adapters_HappyPath_200 first run)
- **Issue:** Pre-existing bug from Plan 05-06's materialiseTypedRaw. The `skills` column was special-cased to rewrite []SkillToken into the typed JSON shape, but `config` (adapters-only) was left as a passthrough. coerceJSONBObject returns the trimmed JSON string; ImportAdapterRequest.Config is `*map[string]interface{}`. reMarshalAs[T] couldn't decode the string into the map, producing per-row reason="invalid_json_row" for every adapter CSV row.
- **Fix:** Added `case "config"` to the materialiseTypedRaw switch. When the cell value is a non-empty string, json.Unmarshal into `map[string]interface{}` and place into the materialised map. On decode failure, leave as string and let the row processor surface invalid_json_row (defensive). On nil/empty string, OMIT the field (matches the JSONB column's NULL/empty-object behaviour at the DB layer; row processor's nil Config defaults to "{}").
- **Files modified:** `services/api/internal/imports/handler_import.go`
- **Verification:** `TestBulkImport_CSV_Adapters_HappyPath_200` passes; all 25 entity x format matrix tests pass.
- **Committed in:** `2f1ddfe` (Task 1).

**2. [Rule 3 - Blocking] .gitattributes `* text=auto eol=lf` would strip CRLF from windows-excel-agents.csv**

- **Found during:** Task 2 (git add reported "warning: CRLF will be replaced by LF the next time Git touches it")
- **Issue:** Repo-level .gitattributes forces eol=lf on all text files. The Pitfall-1 windows-excel-agents.csv MUST retain CRLF + BOM byte-for-byte; the first contributor to checkout the file post-commit would lose the CRLF, silently breaking TestBulkImport_WindowsExcel_BOM_CRLF_200.
- **Fix:** Added two lines to .gitattributes flagging both windows-excel-agents.csv and invalid-utf8.csv as `binary`. git rm --cached + re-add cycled the index so the new attribute took effect for both files.
- **Files modified:** `.gitattributes`
- **Verification:** `git add` no longer prints the CRLF warning; `head -c 3 ... | xxd -p` returns efbbbf after re-add (confirmed BOM still present).
- **Committed in:** `27d6909` (Task 2).

**3. [Rule 1 - Bug] Initial idempotency_test.go used a custom io.Reader / EOF type**

- **Found during:** Task 3 first draft (review of my own first-pass code before commit)
- **Issue:** I initially wrote `mustReader(b []byte) *bytesReader` + a custom `*eofErr` so I wouldn't have to import "io" in idempotency_test.go. This is incorrect — the custom EOF type is not the std-lib's io.EOF sentinel, which errors.Is checks for in many code paths.
- **Fix:** Replaced with `bytes.NewReader(rawB)` (already had bytes imported via the package-wide imports). Removed the custom types.
- **Files modified:** idempotency_test.go (before commit)
- **Verification:** `go vet` clean; `go test -count=1 -run 'TestBulkImport_IdempotencyKey_'` 5 PASS.
- **Committed in:** `97a605e` (Task 3) — the fix was inline before commit, so no separate fix-up commit.

---

**Total deviations:** 3 auto-fixed (1 pre-existing bug in materialiseTypedRaw, 1 blocking .gitattributes config, 1 self-corrected before commit). No architectural changes; no Rule 4 escalations.

## Issues Encountered

Beyond the three auto-fixes above, none. The Wave 4 dispatch tests + Wave 3 row processor tests + Wave 2 sweep tests had already exercised the production code paths; Wave 5's contribution is the entity x format matrix coverage end-to-end and the cross-org / migration-idempotency proofs.

## Threat Mitigation Confirmation

| Threat | Mitigation | Verified |
|--------|------------|----------|
| T-05-07-01 (testdata files with editor-applied BOM) | BOM check per CSV file in testdata manifest above. Only windows-excel-agents.csv has BOM; all others are confirmed no_BOM. | `for f in *.csv; do head -c 3 "$f" \| xxd -p; done` |
| T-05-07-02 (cross-org shared sharedPool race) | Every test uses `freshOrg(t)` -> per-test orgID + `cleanImportTables(orgID)` in defer. Per-org DELETE is parallel-safe (Phase 4 D-73 invariant). | All 32 isolation tests pass under default parallelism. |
| T-05-07-03 (migration idempotency masking DDL bug) | pg_catalog snapshot diff captures columns + indexes + CHECK constraints; replay each DDL individually and assert EXPECTED SQLSTATE 42P07 per statement. | TestMigration_ImportJobs_Idempotent passes against real PG 17 testcontainer. |
| T-05-07-04 (51 MB body test exhausting CI memory) | Body built via `strings.Repeat(row, repeats)` inside the test body; allocated once per test invocation; GC'd after the test exits. ~50 MB peak. | TestBulkImport_Oversize50MBBody_413 passes in < 1s wall time. |
| T-05-07-05 (tests bypassing BodyLimit middleware) | Every integration test goes through httptest.NewServer(server.NewMux(...)) — the production chi mux with BodyLimit injected. No test reaches the Importer directly. | Plan 05-06 server_test.go already pinned this; Wave 5's Oversize tests transitively re-confirm. |
| T-05-07-06 (PII in testdata) | All testdata uses synthetic codes (emp_001, br_lunch_1, etc.) + synthetic names + @example.com emails. No real customer data. | Visual review of all 14 testdata files. |
| T-05-07-SC (test setup downloading testdata at runtime) | All testdata is checked into the repo. The 501-row oversize test generates the body PROGRAMMATICALLY inside the test, not fetched. | TestBulkImport_Oversize501Rows_413 inlines bytes.Buffer + fmt.Fprintf. |

## Threat Flags

None — Plan 05-07 adds no new network surface or trust boundary. The new tests exercise existing endpoints via the production chi mux.

## Next Phase Readiness

- **Phase 5 phase-gate (Plan 05-08):** ready. The full automated test surface is green. The phase-gate plan can now confirm:
  - `cd services/api && go test -count=1 -short ./...` -> 352 PASS in 17 packages.
  - `cd services/api && go test -count=1 ./internal/imports/... ./test/isolation/...` -> 170 PASS + 1 SKIP (testcontainer; the SKIP is Wave 3's documented AgentStateSeedFails).
  - `cd services/api && go test -race -count=1 ./...` -> 17 packages OK in ~43s wall (race detector clean).
- **Phase 6 (Standalone Admin):** ready to consume. The full BulkImportCatalog + GetImportJob endpoints work end-to-end with the locked wire shape. The Phase 6 admin UI can POST CSV/JSON to /v1/orgs/{org_id}/catalog/import?entity={agent|skill|...} and render the BulkImportResult.failed[] entries as a downloadable error table; the GetImportJob endpoint supports the polling-after-POST shape (v0.1 sync; v0.2 async path is already wire-compatible via the status enum).
- **No blockers.** All locked invariants have automated regression coverage; Pitfall 1 is byte-for-byte protected via .gitattributes; cross-org isolation has 4 dedicated probes; migration 000003 has a DDL-replay snapshot test.

## Self-Check

| Claim | Status |
|-------|--------|
| 6 new _test.go files in services/api/internal/imports/ + services/api/test/isolation/ | FOUND |
| 14 testdata files in services/api/internal/imports/testdata/ | FOUND |
| Commit `2f1ddfe` (Task 1 — entity x format matrix + HTTP harness + testdata) | FOUND |
| Commit `27d6909` (Task 2 — Pitfall-1 windows-excel + CSV-spec edges) | FOUND |
| Commit `97a605e` (Task 3 — idempotency + jobs lifecycle) | FOUND |
| Commit `cacaaaf` (Task 4 — cross-org isolation + migration idempotency) | FOUND |
| handlers_test.go has 25 TestBulkImport_ functions | FOUND |
| handlers_csv_test.go has 7 TestBulkImport_ functions | FOUND |
| idempotency_test.go has 6 TestBulkImport_ functions | FOUND |
| jobs_test.go has 5 TestBulkImport_ functions | FOUND |
| test/isolation/imports_test.go has 4 TestImport_CrossOrg_* functions | FOUND |
| test/isolation/imports_migration_idempotent_test.go has TestMigration_ImportJobs_Idempotent | FOUND |
| windows-excel-agents.csv has BOM (0xEF 0xBB 0xBF) | FOUND |
| windows-excel-agents.csv has CRLF (4 line endings) | FOUND |
| .gitattributes flags both windows-excel-agents.csv + invalid-utf8.csv as binary | FOUND |
| materialiseTypedRaw `config` case decodes JSON-encoded string -> map | FOUND |
| go vet ./internal/imports/... clean | FOUND |
| go vet ./test/isolation/... clean | FOUND |
| go build ./... clean | FOUND |
| go test -count=1 -short ./... -> 352 PASS in 17 packages | FOUND |
| go test -count=1 ./internal/imports/... -> 138 PASS + 1 SKIP | FOUND |
| go test -count=1 ./test/isolation/... -> 32 PASS | FOUND |
| go test -race -count=1 ./... -> 17 packages OK | FOUND |
| Total NEW test functions in this plan = 48 (≥35 required) | FOUND |

## Self-Check: PASSED

---
*Phase: 05-bulk-import-go*
*Completed: 2026-05-17*
