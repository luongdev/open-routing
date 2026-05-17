---
phase: 05-bulk-import-go
plan: 09
subsystem: bulk-import + catalog-identity
tags: [fixup, cross-ai-review-block, h1-h2-h3, m1-m2-m3-m4-m5, l1-l2-l3-l4, phase-04.1-carry-forward]

# Dependency graph
requires:
  - phase: 05-bulk-import-go
    provides: Plans 05-01..05-08 implementation + cross-AI review BLOCK verdict (REVIEW.md)
  - phase: 04.1-catalog-identity-normalization
    provides: Code identity contract (3 of 12 findings rooted in Phase 04.1 — H1, H2, M6)
provides:
  - Phase 5 + Phase 04.1 BLOCK clearance (3 HIGH + 5 MED + 4 LOW addressed)
  - PATCH duplicate_external_id → 409 (H1; 6 catalog Update handlers)
  - PATCH external_id empty-string clear semantic (H2; 6 SQL queries + OpenAPI doc)
  - Bulk-import finaliseJob failure → 500 (H3; data-committed-audit-corrupt signal)
  - Savepoint failure abort + chunk-atomicity preservation (M1+M2)
  - Adapter config field-level error precision (M3)
  - Agent skill proficiency 1..10 validation pre-DB (M4)
  - Idempotency replay status code mirrors original (M5; 200/207/422)
  - Documentation refresh + test-coverage gaps (L1, L2, L3, L4)
affects: [phase-04.1-merge, phase-5-merge, cross-ai-re-review]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "OpenAPI Update*409 oneOf union for VersionConflict + duplicate_external_id (mirror of Create*409)"
    - "SQL CASE expression three-way semantic for nullable PATCH columns (preserve / clear / set)"
    - "Test-only override hook on Importer (WithFinaliseOverride) for audit-failure injection"
    - "Temporary CHECK constraint pattern for forcing DB-layer failures in integration tests"
    - "Atomic-fix commit splitting: one commit per finding ID where possible, combined commits where the diff is inherently intertwined (H1+H2 share OpenAPI schemas + handlers; M1+M2 share chunk-loop logic)"

key-files:
  created:
    - .planning/phases/05-bulk-import-go/05-09-SUMMARY.md
  modified:
    - openapi/openapi.yaml (Update*409 → oneOf x6, Update*Request external_id description x6, L4 PUT semantics block)
    - services/api/internal/api/types.gen.go (regen — Update*409JSONResponseBody union types)
    - services/api/internal/api/server.gen.go (regen — Update*409 union encoding)
    - services/api/internal/api/spec.gen.go (regen)
    - services/api/internal/catalog/agents.go (409 branch + ExternalID passthrough + union body)
    - services/api/internal/catalog/skills.go (409 branch + ExternalID passthrough + union body)
    - services/api/internal/catalog/queues.go (409 branch + ExternalID passthrough + union body)
    - services/api/internal/catalog/channels.go (409 branch + ExternalID passthrough + union body)
    - services/api/internal/catalog/adapters.go (409 branch + ExternalID passthrough + union body)
    - services/api/internal/catalog/break_reasons.go (409 branch + ExternalID passthrough + union body)
    - services/api/internal/catalog/agents_test.go (TestAgents_PatchDuplicateExternalId + TestAgents_PatchClearExternalId)
    - services/api/internal/catalog/skills_test.go (corresponding H1+H2 tests)
    - services/api/internal/catalog/queues_test.go (corresponding H1+H2 tests)
    - services/api/internal/catalog/channels_test.go (corresponding H1+H2 tests)
    - services/api/internal/catalog/adapters_test.go (corresponding H1+H2 tests)
    - services/api/internal/catalog/break_reasons_test.go (corresponding H1+H2 tests)
    - services/api/internal/db/queries/agents.sql (CASE expression for external_id)
    - services/api/internal/db/queries/skills.sql (CASE expression for external_id)
    - services/api/internal/db/queries/queues.sql (CASE expression for external_id)
    - services/api/internal/db/queries/channels.sql (CASE expression for external_id)
    - services/api/internal/db/queries/adapters.sql (CASE expression for external_id)
    - services/api/internal/db/queries/break_reasons.sql (CASE expression for external_id)
    - services/api/internal/db/generated/agents.sql.go (sqlc regen)
    - services/api/internal/db/generated/skills.sql.go (sqlc regen)
    - services/api/internal/db/generated/queues.sql.go (sqlc regen)
    - services/api/internal/db/generated/channels.sql.go (sqlc regen)
    - services/api/internal/db/generated/adapters.sql.go (sqlc regen)
    - services/api/internal/db/generated/break_reasons.sql.go (sqlc regen)
    - services/api/internal/server/server.go (6 new injectUpdate*409RequestID helpers + switch cases)
    - services/api/internal/server/request_id_exhaustiveness_test.go (6 Update*409 fixture + assertion updates)
    - services/api/internal/imports/handler_import.go (H3 finaliseJob → 500 + M3 config passthrough)
    - services/api/internal/imports/handlers.go (WithFinaliseOverride option + finaliseOverride field)
    - services/api/internal/imports/jobs.go (H3 override hook honour)
    - services/api/internal/imports/chunk.go (M1+M2 savepoint failure abort)
    - services/api/internal/imports/coerce.go (L3 int32 bounds + M3 ErrInvalidJSON sentinel)
    - services/api/internal/imports/header.go (M3 coerceJSONBObject parses JSON pre-emit)
    - services/api/internal/imports/row_agent.go (M4 proficiency 1..10 validation + L4 PUT semantics doc)
    - services/api/internal/imports/doc.go (L1 file map refresh)
    - services/api/internal/imports/idempotency.go (M5 status-mirror replay)
    - services/api/internal/imports/handlers_test.go (TestBulkImport_FinaliseJobFails_Returns500 + TestBulkImport_AgentImport_ProficiencyOutOfRange_FieldLevelError + TestBulkImport_CSV_Adapters_MalformedConfig_FieldLevelError)
    - services/api/internal/imports/idempotency_test.go (M5 207/422 replay tests + adjusted existing 422 case)
    - services/api/internal/imports/handler_import_test.go (M5 assertion type fixed)
    - services/api/internal/imports/row_agent_test.go (L2 implement skipped test via temporary CHECK constraint)
    - services/api/internal/imports/coerce_test.go (L3 6 new int32-boundary sub-cases)
    - web/packages/ui/src/api/generated.ts (openapi-typescript regen)

key-decisions:
  - "H1+H2 atomic-commit decision: ship in a single commit. The two findings touch the same 6 catalog handlers + the same OpenAPI Update*Request/Response schemas; splitting would require fragile cherry-picking of generated code that interleaves between the two changes. The commit message attributes each finding distinctly so cross-AI re-review can map fixes to findings."
  - "H2 implementation: empty-string `\"\"` sentinel for external_id clear (NOT a tri-state JSON null). oapi-codegen v2 cannot distinguish JSON null from omitted-field at decode time (both render as nil *string); the documented v0.1 way to clear is to send `\"\"`. v0.2 candidate: tri-state wrapper that allows literal null."
  - "H3 implementation: return 500 from finaliseJob failure rather than retry-and-return-success. The data IS committed at the DB level (per-chunk outer tx ran), so retry would double-apply unless the idempotency-key was supplied. A 500 with reason=finalise_job_failed_data_committed_audit_corrupt is honest about the inconsistent state and prompts the admin to verify via GET /imports/{id}."
  - "M1+M2 atomic-commit decision: combined into one commit because they share the same chunk-atomicity invariant and the fix code paths are symmetrical (BEGIN-fail flush + RELEASE-fail flush)."
  - "M3 implementation: parse JSON at coerce time, not in materialiseTypedRaw. Moves the error attribution to the field-spec layer where the column name is in scope, eliminates a double-parse, and aligns with the D5-08 field-precision contract that the other coerce primitives already honour."
  - "M5 implementation: lookupIdempotentReplay return type widened from BulkImportCatalog200JSONResponse to BulkImportCatalogResponseObject. This is the only Phase 5 fix that changes a function signature; existing callers ([replay, nil] pattern in handler_import.go) work unchanged because Go's interface return-type widening is transparent at the call site."
  - "L2 implementation: temporary CHECK constraint (ADD CONSTRAINT ... NOT VALID + DROP CONSTRAINT in t.Cleanup) — forces the state-seed failure path in integration without destructive schema mutations. NOT VALID skips re-validation of pre-existing rows."
  - "L4 documentation-only fix: source-code semantic IS PUT (EXCLUDED.column always overwrites). The plan considered making it MERGE but that's a v0.2 candidate (D5-DEFER) — for v0.1 the fix is to document the actual behaviour clearly in three places (OpenAPI YAML comment, row_agent.go file header, doc.go advisory section)."
  - "False-positive rejection: 2 of the 11 Gemini MED findings (break_reasons cache key + queue priority default) were verified spurious in the REVIEW.md synthesis; this plan does NOT touch those code paths. The 3 Gemini false-positives (incl. the reclassified MED→LOW M6 stale-Current-snapshot) are deferred to v0.2 per the REVIEW.md action-items section."

patterns-established:
  - "Atomic-commit per finding (H1, H2 combined as joint commit due to schema entanglement; M1+M2 combined as symmetric chunk-loop fix; H3/M3/M4/M5/L1/L2/L3/L4 each their own commit)"
  - "OpenAPI Update*409 oneOf union to share a single HTTP status across multiple semantic causes (version_conflict vs duplicate_external_id) — applied to all 6 catalog entities so the pattern is uniform"
  - "Empty-string sentinel for nullable PATCH-clear (oapi-codegen v2 limitation workaround; documented as v0.1 contract with v0.2 tri-state upgrade path)"
  - "Test-only override hook field on Importer for integration-level error injection (WithFinaliseOverride pattern — same package field access, no exported setter)"
  - "Temporary CHECK constraint for integration-level DB-layer failure forcing (ALTER TABLE ... NOT VALID + DROP CONSTRAINT in t.Cleanup) — non-destructive substitute for schema-corruption integration tests"

requirements-completed: []

# Metrics
duration: ~55min
completed: 2026-05-17
---

# Phase 5 Plan 09: Cross-AI Review BLOCK Clearance Summary

**Addresses all 12 confirmed cross-AI review findings (3 HIGH + 5 MED + 4 LOW) flagged by Codex + Gemini parallel review on the combined Phase 04.1 + Phase 5 diff (REVIEW.md). Both reviewers independently returned BLOCK; the fix-up commits below clear the BLOCK ahead of the cross-AI re-review and the Phase 04.1 + Phase 5 PR-group merge.**

## Performance

- **Duration:** ~55min total
- **Started:** 2026-05-17 (immediately after Plan 05-08 returned BLOCK)
- **Completed:** 2026-05-17
- **Commits:** 10 (one per finding where feasible; H1+H2 combined; M1+M2 combined)
- **Test impact:** 705 passing across 17 packages with `-race -count=1` (was 690 pre-fix; +15 from H1+H2 entity matrix tests + H3/M3/M4/M5/L2/L3 cases)
- **Lint impact:** `task lint` exits 0 (no new lint issues introduced)
- **Codegen drift:** `task gen && task gen && git diff --exit-code` clean (zero-drift on second invocation)

## Accomplishments

### HIGH (3 findings, 2 commits)

**H1 — PATCH duplicate-external-id → 500 instead of 409** (commit `eaf429f`)
- Site: 6 catalog Update handlers (agents.go:537, skills.go:363, queues.go:395, channels.go:408, adapters.go:407, break_reasons.go:381).
- Root cause: MapPgError returned status=409 for `duplicate_external_id` (per Phase 04.1 D04_1-21 constraint-name introspect), but Update handlers only branched on `status == 422` — the 409 fell through to the 500 default, poisoning 5xx metrics for a client-correctable conflict. Wave 5 fix in Phase 04.1 was applied to Create handlers only; Update handlers were missed.
- Fix:
  - OpenAPI Update*409 schemas extended from `allOf [VersionConflict, Current]` to `oneOf [ErrorResponse, allOf [VersionConflict, Current]]` (mirrors Create*409). Six schemas updated.
  - Generated types regenerated: `Update*409JSONResponseBody` is now a oneOf union with `FromErrorResponse` + `FromUpdate*409JSONResponseBody1` constructors.
  - 6 Update handlers updated: VersionConflict path now uses the union helper; new `case 409` branch on MapPgError result builds the ErrorResponse member for duplicate_external_id.
  - 6 new server.go injectUpdate*409RequestID helpers (probe `current` field to disambiguate union member, inject request_id, re-encode).
  - server/request_id_exhaustiveness_test.go: 6 union-shape fixtures + 6 As*Body1 assertion paths.
  - 6 new catalog/*_test.go tests asserting PATCH dup external_id → 409 + ErrorCode = duplicate_external_id.

**H2 — PATCH external_id cannot clear binding** (same commit `eaf429f`)
- Site: 6 UPDATE SQL queries + 6 Update handlers.
- Root cause: pre-fix SQL used `COALESCE(sqlc.narg('external_id')::text, external_id)`. oapi-codegen renders both omitted and `null` as nil pointer → SQL parameter is always NULL → COALESCE always preserves the existing value. Direct contract drift introduced by Phase 04.1's "pass null to clear" semantic; combined with Update handlers that omitted ExternalID from sqlc params entirely, clients literally could not clear the column.
- Fix:
  - OpenAPI Update*Request.external_id description rewritten: pass `""` to clear (sentinel for v0.1; v0.2 candidate is tri-state).
  - 6 UPDATE SQL queries rewritten with three-way CASE:
    ```sql
    external_id = CASE
      WHEN sqlc.narg('external_id')::text IS NULL THEN external_id  -- omit → preserve
      WHEN sqlc.narg('external_id')::text = ''    THEN NULL          -- "" → clear
      ELSE sqlc.narg('external_id')::text                            -- new value
    END
    ```
  - 6 Update handlers now pass `ExternalID: req.Body.ExternalId` through to sqlc params (pre-fix the field was omitted entirely).
  - 6 new catalog/*_test.go tests asserting PATCH external_id="" → 200 with the column nulled.

**H3 — finaliseJob failure swallowed → admin sees stale "pending"** (commit `9d5db0f`)
- Site: services/api/internal/imports/handler_import.go:446.
- Root cause: pre-fix handler logged warn on finaliseJob error and returned the 200/207 success result anyway. Three downstream consequences poisoned the audit trail (pending status persisted, idempotency replay returned empty, 24h sweep overwrote with server_crash).
- Fix: return BulkImportCatalog500JSONResponse with reason `finalise_job_failed_data_committed_audit_corrupt`. Test override hook `WithFinaliseOverride` added to Importer so integration tests can inject the failure without destructive schema mutations.

### MED (5 findings, 4 commits)

**M1 — Savepoint RELEASE failure not rolled back** (commit `e0f08ed`)
**M2 — Savepoint BEGIN failure cascades silently** (same commit `e0f08ed`)
- Combined fix: both failure paths now abort the chunk by flushing previously-succeeded rows + the failing row + the remaining tail to `failed[]` with the matching sentinel reason. Returns `(nil, failed)` so the caller doesn't double-count.

**M3 — Adapter config CSV malformed → generic field** (commit `66e2b46`)
- Site: services/api/internal/imports/header.go:256 + handler_import.go:574.
- Fix: parse the JSON object at the coerce step (`coerceJSONBObject` in header.go) so the error surfaces as `{field: "config", reason: "invalid_json"}` via the standard columnSpec.coerce error path. materialiseTypedRaw's "config" branch becomes a passthrough — the cell is already a parsed map when we arrive there.

**M4 — Agent skill proficiency 1..10 not validated pre-DB** (commit `21e8302`)
- Site: services/api/internal/imports/row_agent.go (skills loop).
- Fix: range check 1..10 inline in row_agent.go's Step 3 (right after the skill_code regex). Out-of-range surfaces as `{field: "skills[i].proficiency", reason: "invalid_value"}`. The misleading "0..100" comment on the nolint:gosec annotation is also corrected to "1..10".

**M5 — Idempotency replay always returns 200** (commit `99909a7`)
- Site: services/api/internal/imports/idempotency.go.
- Fix: `lookupIdempotentReplay` return type widened from `BulkImportCatalog200JSONResponse` to `BulkImportCatalogResponseObject` (the interface). `rehydrateBulkImportResult` chooses 200 / 207 / 422 based on persisted succeeded_rows + failed_rows.

### LOW (4 findings, 4 commits)

**L1 — doc.go stale Wave 3/4 comments** (commit `b93e70f`)
- Renamed section heading from "Wave-by-wave file map" (forward-looking) to "Landed file map (Phase 5 Plans 04..07 + 09 fix-up)" (descriptive). Added per-fix-up annotations (H3, M1+M2, M3, M4, M5) so the file map reflects the current state.

**L2 — row_agent_test.go t.Skip on agent_state_seed_failed** (commit `aa03c25`)
- Implemented with a temporary CHECK constraint pattern (ADD CONSTRAINT ... NOT VALID + DROP CONSTRAINT in t.Cleanup). The constraint forbids `status='Offline'` for the test's duration; the row processor always inserts `status='Offline'`, so InsertAgentStateOnConflictNothing fails with 23514. The test asserts the rowError reason AND that the agent row was rolled back (savepoint atomicity).

**L3 — parseInt int32 bounds check** (commit `04614a2`)
- coerce.go parseInt now rejects values outside `[math.MinInt32, math.MaxInt32]` with `ErrInvalidInt`. Six new sub-cases in TestParseInt covering the boundaries.

**L4 — CSV-omitted-column PUT semantics documentation** (commit `508e8fa`)
- Documentation refresh in three places: OpenAPI YAML comment block above ImportXxxRequest schemas, row_agent.go file header, doc.go advisory section (added in L1 commit). No source-code semantic change.

## Task Commits

Each finding's fix as its own atomic commit (H1+H2 and M1+M2 combined where the diff is inherently intertwined):

| Commit | Finding | Title |
|--------|---------|-------|
| `eaf429f` | H1+H2 | `fix(04.1): PATCH external_id 409 mapping + empty-string clear (H1+H2)` |
| `9d5db0f` | H3 | `fix(05): finaliseJob failure returns 500 instead of 200 (H3)` |
| `e0f08ed` | M1+M2 | `fix(05): abort chunk on savepoint failure (M1+M2)` |
| `66e2b46` | M3 | `fix(05): malformed adapter config cell field-level error (M3)` |
| `21e8302` | M4 | `fix(05): validate proficiency 1..10 pre-DB in agent import (M4)` |
| `99909a7` | M5 | `fix(05): idempotency replay matches original HTTP status (M5)` |
| `b93e70f` | L1 | `docs(05): refresh imports/doc.go post-Wave-completion (L1)` |
| `aa03c25` | L2 | `test(05): implement skipped agent_state_seed path (L2)` |
| `04614a2` | L3 | `fix(05): coerce parseInt int32 bounds check (L3)` |
| `508e8fa` | L4 | `docs(05): document CSV-omitted-column PUT semantics (L4)` |

Plan metadata commit (this SUMMARY) follows.

## Files Created/Modified

See the YAML frontmatter `key-files` for the canonical list. Summary:

- **Created (1):** `.planning/phases/05-bulk-import-go/05-09-SUMMARY.md`
- **Modified (40 source + regen files)** spanning openapi.yaml, 6 catalog handlers + tests, 6 SQL queries + sqlc-regen, server.go + tests, 8 imports/*.go + 5 imports/*_test.go files, 3 api/*.gen.go regen, generated.ts regen.

## Decisions Made

See YAML `key-decisions` block. Highlights:

- **D09-01: H1+H2 ship as one commit** because the two findings touch the same 6 catalog handlers + the same OpenAPI Update*Request/Response schemas. Splitting would require fragile generated-code cherry-pick.
- **D09-02: H2 implementation = empty-string sentinel** (not tri-state). oapi-codegen v2 cannot distinguish JSON null from omitted-field; v0.1 documented contract is `""` clears. v0.2 candidate: tri-state wrapper.
- **D09-03: H3 returns 500** rather than retry-and-return-success. The data IS committed at the DB level (per-chunk outer tx ran), so retry would double-apply without Idempotency-Key. 500 with reason `finalise_job_failed_data_committed_audit_corrupt` is honest about the state.
- **D09-04: M5 lookupIdempotentReplay return-type widening** from `BulkImportCatalog200JSONResponse` to the interface. This is the only signature change in this plan; callers (handler_import.go:118) work unchanged because Go interface return-type widening is transparent.
- **D09-05: L2 implementation = temporary CHECK constraint** rather than DB mock. ADD CONSTRAINT ... NOT VALID + DROP in t.Cleanup is non-destructive and exercises the real savepoint atomicity (which is the load-bearing invariant the test verifies).
- **D09-06: L4 is documentation-only**. The source-code semantic is PUT (EXCLUDED.column always overwrites); making it MERGE is a v0.2 candidate (D5-DEFER). For v0.1 the fix is clear docs in three places.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] pnpm install in web/**
- Found during: H1+H2 work (first `task gen` invocation).
- Issue: Worktree shipped without `web/node_modules`; openapi-typescript missing.
- Fix: `cd web && pnpm install --frozen-lockfile`. No commit needed (node_modules is gitignored).
- Same pattern as Plan 08.

**2. [Rule 1 — Test-only] Pre-existing test assumed replay was always 200 (M5 fallout)**
- Found during: M5 work, after the fix flipped pre-seeded all-failed rows to 422.
- Affected: TestBulkImport_IdempotencyKey_HitReturnsPriorFailures (idempotency_test.go) and TestBulkImportCatalog_IdempotencyHit_ReturnsReplay (handler_import_test.go).
- Fix: updated the assertions to expect 422 (matching the new M5 semantic). Both tests' pre-seeded rows had succeeded=0/failed=N which is the all-failed case → 422 per M5.
- Tracked in M5 commit (`99909a7`) — explicitly documented in the commit message.

**3. [Rule 1 — Test-only] api.Agent{} fails MarshalJSON regex on empty Email**
- Found during: H1 work (request_id_exhaustiveness_test.go fixture build).
- Issue: api.Agent has an openapi_types.Email field that runs regex validation in MarshalJSON. The empty zero value fails the regex.
- Fix: supply `api.Agent{Email: "stub@example.com"}` in the test fixture so the union round-trip succeeds. Same for Skill / Queue / Channel / Adapter / BreakReason fixtures.
- Tracked in H1+H2 commit (`eaf429f`).

**Total deviations:** 3 auto-fixed (1 Rule 3 env setup; 2 Rule 1 test-only adjustments necessitated by the contract changes).

## Issues Encountered

**Combined commits for tightly-coupled findings.** The plan asked for atomic commits per finding, but H1+H2 share the same 6 catalog handlers + the same OpenAPI Update*Request/Response schemas (oapi-codegen regen interleaves between them in types.gen.go and server.gen.go). M1+M2 share the same savepoint-failure abort code path. For both pairs, the cleanest approach was a combined commit with a message that distinguishes the two findings — see commit `eaf429f` and `e0f08ed` for the dual-attribution format. Other findings (H3, M3-M5, L1-L4) each got their own atomic commit.

**Test fixture regression discovery during M5.** The pre-existing idempotency tests assumed every replay returned 200. After M5 flipped the contract, those tests started failing (correctly — they were probing the buggy behaviour). Updating both `idempotency_test.go:143` and `handler_import_test.go:312` was a small extra scope inside the M5 commit; the commit message calls it out explicitly so the changes are attributable to the contract fix, not a flaky test.

## User Setup Required

None — this plan is purely fix-up + tests. No external service configuration changes.

## Open Issues

### REVIEW.md findings NOT addressed by this plan

Per REVIEW.md § Composite Findings, this plan addresses 12 of the 12 confirmed concerns (3 HIGH + 5 MED + 4 LOW). The two **rejected** findings are not fixed because they are verified false-positives:

- **Gemini MED #1 (cache key break_reasons):** `string(api.BreakReasons)` evaluates to `"break_reasons"` (underscored), not `"break-reasons"` (hyphenated). The cache key matches the catalog handler's pattern. Verified spurious in REVIEW.md.
- **Gemini MED #3 (queue priority default 5):** OpenAPI has `example: 5` NOT `default: 5` on `priority`. The handler's `derefInt(typed.Priority, 0)` is consistent with no documented default. Verified spurious in REVIEW.md.

Additionally, **Gemini reclassified-to-MED M6 (409 Current is stale snapshot)** was deemed optional in REVIEW.md (Gemini's HTTP-status claim was wrong; the underlying staleness concern affects only the 409 response body's `current` field freshness, not the status code itself). Deferred to a v0.2 plan per REVIEW.md action-items.

### Threat surface flags

None detected. This plan adds:
- **A new test-only override hook** (`WithFinaliseOverride`) on Importer — but the field is unexported (`finaliseOverride`), production wiring leaves it nil, and the override is package-internal so external callers cannot reach it. No new attack surface.
- **A temporary CHECK constraint pattern** in row_agent_test.go's L2 test — runs against the test DB only, scope-bounded by `t.Cleanup` to drop the constraint before the next test.

## Wave 6 follow-up / Phase 6 readiness

**Phase 6 (Standalone Admin & Shared UI) is now unblocked.** The Phase 04.1 + Phase 5 PR group can merge to main pending the cross-AI re-review pass on this plan's fix-up commits (the orchestrator's Plan 09 spawn objective said: "After fixes, run `go test -race -count=1 ./...` + `go vet ./...` + `task lint` + `task gen && git diff --exit-code` to confirm green. The orchestrator will re-run cross-AI review on the fixup diff after this plan returns.").

The re-review must confirm:
- Both reviewers' BLOCK verdicts cleared (or downgraded to READY / READY-WITH-FIXES).
- No NEW HIGH/MED findings introduced by the fix-up commits themselves.

---

*Phase: 05-bulk-import-go*
*Plan: 09 (fix-up)*
*Completed: 2026-05-17*

## Self-Check: PASSED

Files referenced in this SUMMARY all exist and are tracked:

- `.planning/phases/05-bulk-import-go/05-09-SUMMARY.md` — committed by THIS commit.
- `openapi/openapi.yaml` — modified in `eaf429f`, `508e8fa`.
- `services/api/internal/api/types.gen.go` — regen modified in `eaf429f`.
- `services/api/internal/catalog/{agents,skills,queues,channels,adapters,break_reasons}.go` — modified in `eaf429f`.
- `services/api/internal/catalog/{agents,skills,queues,channels,adapters,break_reasons}_test.go` — modified in `eaf429f`.
- `services/api/internal/db/queries/{agents,skills,queues,channels,adapters,break_reasons}.sql` — modified in `eaf429f`.
- `services/api/internal/db/generated/{agents,skills,queues,channels,adapters,break_reasons}.sql.go` — regen in `eaf429f`.
- `services/api/internal/server/server.go` + `request_id_exhaustiveness_test.go` — modified in `eaf429f`.
- `services/api/internal/imports/handler_import.go` — modified in `9d5db0f` (H3) + `66e2b46` (M3).
- `services/api/internal/imports/handlers.go` + `jobs.go` + `handlers_test.go` — H3 fix `9d5db0f`.
- `services/api/internal/imports/chunk.go` — M1+M2 fix `e0f08ed`.
- `services/api/internal/imports/coerce.go` — M3 + L3 (`66e2b46`, `04614a2`).
- `services/api/internal/imports/header.go` — M3 fix `66e2b46`.
- `services/api/internal/imports/row_agent.go` — M4 + L4 (`21e8302`, `508e8fa`).
- `services/api/internal/imports/idempotency.go` + `idempotency_test.go` + `handler_import_test.go` — M5 fix `99909a7`.
- `services/api/internal/imports/doc.go` — L1 refresh `b93e70f`.
- `services/api/internal/imports/row_agent_test.go` — L2 fix `aa03c25`.
- `services/api/internal/imports/coerce_test.go` — L3 cases `04614a2`.

Commits referenced exist on this worktree branch:
- `eaf429f` (H1+H2) — present.
- `9d5db0f` (H3) — present.
- `e0f08ed` (M1+M2) — present.
- `66e2b46` (M3) — present.
- `21e8302` (M4) — present.
- `99909a7` (M5) — present.
- `b93e70f` (L1) — present.
- `aa03c25` (L2) — present.
- `04614a2` (L3) — present.
- `508e8fa` (L4) — present.

Validation gates PASSED:
- `task gen` × 2 — clean, no drift.
- `go test -race -count=1 ./...` — 705 passing across 17 packages.
- `go vet ./...` — no issues.
- `task lint` — 3 successful, 3 total.
- `git diff --exit-code` — clean.
