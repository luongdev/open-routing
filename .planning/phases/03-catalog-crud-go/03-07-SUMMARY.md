---
phase: 03-catalog-crud-go
plan: 07
subsystem: services/api/internal/catalog
tags: [cat-02, cat-07, cat-08, cat-09, cat-10, cat-11, skills, break_reasons, wave-4]
dependency_graph:
  requires:
    - 03-06 (agents canonical template — Plan 03-07 clones agents.go shape verbatim)
    - 03-03 (sqlc-generated queries for skills + break_reasons)
    - 03-05 (placeholder scaffold + cache + cursor + mappers helpers)
  provides:
    - "Real CRUD handler bodies for 2 simple catalog entities (skills + break_reasons) — 10 of 30 strict-server methods now implemented"
    - "Replicated agents.go template — confirms the pattern composes for entities with no join table, no FK fields, and (for break_reasons) no external_id"
  affects:
    - 03-08 (queues+channels): same template shape applies, plus channels gets a FK probe (default_queue_id)
    - 03-09 (adapters+agent_skills): same template plus JSONB and join semantics
    - 03-10 (final wiring + isolation suite): cache slugs `"skills"` and `"break_reasons"` are now locked, isolation tests in 03-10 will inspect these exact slugs
tech-stack:
  added: []
  patterns:
    - "Wave 4 template clone — minor entity variant: NO BeginTx in CreateX (skills/break_reasons have no join), simpler than agents.go"
    - "BeginTx in UpdateX retained — D-66 disambiguation contract requires the probe to observe the rollback, so the tx shape stays uniform"
    - "Direct `*string` pass-through for nullable description — sqlc generated `Description *string` (emit_pointers_for_null_types: true) renders pgTextPtr unnecessary for InsertSkillParams/UpdateSkillParams"
    - "23505 UNIQUE(org_id, name) on break_reasons → 500 with reason=name_collision (spec omits 409 response type for CreateBreakReason; clients branch on error code per D-36)"
key-files:
  created:
    - services/api/internal/catalog/skills_test.go (422 LOC; 14 test funcs, 14 t.Run subcases inside TestSkills_LimitOutOfRange)
    - services/api/internal/catalog/break_reasons_test.go (430 LOC; 14 test funcs)
  modified:
    - services/api/internal/catalog/skills.go (placeholder → 384 LOC implementation, 5 handler bodies + mapSkill)
    - services/api/internal/catalog/break_reasons.go (placeholder → 378 LOC implementation, 5 handler bodies + mapBreakReason)
decisions:
  - "Skills `Description` is sqlc-generated as `*string` (not `pgtype.Text`), so pgTextPtr helper is not used; direct *string pass-through round-trips NULL correctly through pgx (Deviation Rule 3 — see below)"
  - "CreateBreakReason has no 409 response type in the OpenAPI spec; 23505 collisions therefore surface as 500 with the canonical name_collision reason; clients branch on the error code (D-36)"
  - "Cache slug for break_reasons is `'break_reasons'` (underscore) per D-58 verbatim — Plan 03-10 isolation tests will pin this exact string"
metrics:
  duration_minutes: 17
  completed_at: 2026-05-16T15:55:17Z
  tasks_completed: 2
  files_changed: 4
  loc_added: 1578
  loc_removed: 36
  tests_added: 28 (top-level) + 2 (subtests) = 30 test cases
---

# Phase 03 Plan 07: Skills + BreakReasons CRUD Summary

Implemented end-to-end CRUD handlers for the two simple catalog entities (skills + break_reasons) using the agents.go template established in Plan 03-06; both files compile, vet clean, and pass all 30 integration tests under `-race` in ~1.27 seconds total.

## Tasks Completed

| # | Task                                                              | Commit    | Files                                           |
|---|-------------------------------------------------------------------|-----------|-------------------------------------------------|
| 1 | Implement skills.go (5 handlers + mapSkill) + skills_test.go      | `d95be5d` | skills.go, skills_test.go                       |
| 2 | Implement break_reasons.go + break_reasons_test.go                | `afb461b` | break_reasons.go, break_reasons_test.go         |

## Test Inventory

### skills_test.go (14 top-level + 2 subtests = 16 test cases)

| Test                                       | CAT-* Coverage |
|--------------------------------------------|----------------|
| `TestSkills_CreateThenGet`                 | CAT-02 + CAT-11 (cache fill) |
| `TestSkills_GetMissing`                    | CAT-02 404 |
| `TestSkills_VersionConflict`               | CAT-08 (D-66 disambiguation) |
| `TestSkills_SoftDelete`                    | CAT-09 (D-65 idempotent-disabled) |
| `TestSkills_SoftDelete_IncludeDisabled`    | CAT-09 (include_disabled toggle) |
| `TestSkills_Cursor`                        | CAT-10 (pagination + N+1) |
| `TestSkills_NameSearch`                    | CAT-10 (ILIKE %name%) |
| `TestSkills_CacheInvalidationOnUpdate`     | CAT-11 + D-55 |
| `TestSkills_CacheInvalidationOnDelete`     | CAT-11 + D-55 |
| `TestSkills_DescriptionNullable`           | CAT-02 (nullable Description round-trip) |
| `TestSkills_LimitOutOfRange` (zero+too_high) | handler-level 400 invalid_body |
| `TestSkills_BadCursor`                     | DecodeCursor → 400 bad_cursor |
| `TestSkills_CrossOrgGet404`                | FOUND-08 isolation canary |
| `TestSkills_ExternalIdCollision`           | CAT-02 (23505 UNIQUE(org_id, external_id) → 409) |

### break_reasons_test.go (14 test cases)

| Test                                              | CAT-* Coverage |
|---------------------------------------------------|----------------|
| `TestBreakReasons_CreateThenGet`                  | CAT-07 + CAT-11 |
| `TestBreakReasons_GetMissing`                     | CAT-07 404 |
| `TestBreakReasons_VersionConflict`                | CAT-08 (D-66 disambiguation) |
| `TestBreakReasons_SoftDelete`                     | CAT-09 |
| `TestBreakReasons_SoftDelete_IncludeDisabled`     | CAT-09 |
| `TestBreakReasons_Cursor`                         | CAT-10 |
| `TestBreakReasons_NameSearch`                     | CAT-10 |
| `TestBreakReasons_CacheInvalidationOnUpdate`      | CAT-11 + D-55 |
| `TestBreakReasons_CacheInvalidationOnDelete`      | CAT-11 + D-55 |
| `TestBreakReasons_RoutableFlag`                   | CAT-07 (routable bool round-trip via POST + PATCH) |
| `TestBreakReasons_DisplayOrder`                   | CAT-07 (display_order round-trip; list ordering by (created_at, id) DESC per CAT-10) |
| `TestBreakReasons_NameUniqueCollision`            | CAT-07 (23505 UNIQUE(org_id, name) → name_collision reason) |
| `TestBreakReasons_CrossOrgGet404`                 | FOUND-08 |
| `TestBreakReasons_BadCursor`                      | 400 bad_cursor |

**Total: 30 test cases pass under `go test -count=1 -race -timeout 240s -run "TestSkills|TestBreakReasons" ./internal/catalog/...`.**

## Diff Scope

```
services/api/internal/catalog/skills.go              | placeholder → 384 LOC (5 handlers + mapSkill)
services/api/internal/catalog/skills_test.go         | NEW         → 422 LOC (14 test funcs)
services/api/internal/catalog/break_reasons.go       | placeholder → 378 LOC (5 handlers + mapBreakReason)
services/api/internal/catalog/break_reasons_test.go  | NEW         → 430 LOC (14 test funcs)
```

No changes to any other file. No modifications to STATE.md or ROADMAP.md (per parallel-executor contract; the orchestrator handles state mutations after wave merge).

## Pattern Adherence (agents.go template)

| Invariant                                      | skills.go | break_reasons.go |
|------------------------------------------------|-----------|------------------|
| `cache.GetOrSet[api.T]` in Get handler         | ✅ (1)    | ✅ (1)           |
| `cache.Key(orgID, "<slug>", id)` references    | ✅ (9)    | ✅ (9)           |
| `h.deps.Cache.Del()` post-commit + 409 + delete | ✅ (4)    | ✅ (4)           |
| `Get<Entity>ByIdAnyVersion` D-66 disambiguation | ✅ (3)    | ✅ (2)           |
| `mapX(row generated.T) api.T`                  | ✅        | ✅               |
| BeginTx in UpdateX                              | ✅ (1)    | ✅ (1)           |
| BeginTx NOT in CreateX (no join table)         | ✅        | ✅               |
| Soft-delete invisibility (CAT-09): GET on enabled=false → 404 | ✅ | ✅ |
| `ExternalID` field absent (break_reasons-only) | n/a       | ✅ (0 references) |

## Deviations from Plan

Plan 03-07 included a single deviation discovered during implementation. No checkpoint required (Rule 1/Rule 3 auto-fix scope).

### Auto-fixed Issues

**1. [Rule 3 - Blocking] sqlc Description field is `*string`, not `pgtype.Text`**

- **Found during:** Task 1 (skills.go implementation)
- **Issue:** Plan acceptance criterion required `grep -F "pgTextPtr(req.Body.Description)" services/api/internal/catalog/skills.go` to return ≥ 1. But the actual sqlc-generated `InsertSkillParams.Description` and `UpdateSkillParams.Description` types are `*string` (because the sqlc config has `emit_pointers_for_null_types: true` — confirmed in `internal/db/generated/skills.sql.go` lines 85 and 280). The `pgTextPtr(*string) pgtype.Text` helper would produce the WRONG type for these params.
- **Fix:** Pass `req.Body.Description` (already `*string`) directly into the params. The nullable round-trip is preserved: pgx writes SQL NULL when the pointer is nil, and the mapper (`mapSkill`) reads `row.Description != nil` to decide whether to emit a non-nil DTO field. The semantics required by the plan ("description nullable round-trip") are fully preserved — only the helper-function name in the acceptance grep is unattainable.
- **Files modified:** `services/api/internal/catalog/skills.go` (the entire description-handling path is direct `*string` rather than via pgTextPtr).
- **Test coverage:** `TestSkills_DescriptionNullable` exercises POST without description → GET returns nil → PATCH adds description → GET returns the value. Test passes.

**2. [Rule 3 - Blocking] CreateBreakReason has no 409 response type in the spec**

- **Found during:** Task 2 (break_reasons.go implementation).
- **Issue:** A `grep -nE "type CreateBreakReason[0-9]" internal/api/server.gen.go` shows ONLY 201/400/500 responses defined — no 409 response type exists for this operation. But the 23505 UNIQUE(org_id, name) collision MUST surface to clients somehow per CAT-07.
- **Fix:** The handler routes the 409-status branch of `mapPgError` to `CreateBreakReason500JSONResponse` with `Error: api.ErrorCodeVersionConflict, Reason: "name_collision"`. The HTTP status is therefore 500 in this single edge case, but per D-36 ("clients branch on error code, not HTTP status"), the wire-shape contract is preserved. The acceptance test `TestBreakReasons_NameUniqueCollision` documents this behavior verbatim. If the spec is later amended to add a 409 type, this branch flips trivially.
- **Files modified:** `services/api/internal/catalog/break_reasons.go` (CreateBreakReason 409 branch comment explains the rationale).

No Rule 4 architectural changes were needed.

## Threat Surface Scan

Reviewed all 4 changed files; no new threat surface beyond what was already enumerated in Plan 03-07's `<threat_model>`:
- T-3-28 (wrong cache slug) — mitigated: slugs `"skills"` and `"break_reasons"` are hard-coded string literals; acceptance greps pin them; Plan 03-10 isolation suite will re-verify.
- T-3-29 (UNIQUE violation 500-vs-409) — accepted with documentation: for break_reasons, the spec only defines a 500 response, so 23505 surfaces as 500 with the canonical `name_collision` reason (D-36 branch path).
- T-3-30 (60s cache staleness window) — accepted (v0.1); D-55 + D-56 ensure DEL on writes including the 409 path.

No new threat flags to record.

## Self-Check: PASSED

Verified:
- `services/api/internal/catalog/skills.go` exists (384 LOC).
- `services/api/internal/catalog/skills_test.go` exists (422 LOC).
- `services/api/internal/catalog/break_reasons.go` exists (378 LOC).
- `services/api/internal/catalog/break_reasons_test.go` exists (430 LOC).
- Commit `d95be5d` exists in `git log --oneline -3` (Task 1).
- Commit `afb461b` exists in `git log --oneline -3` (Task 2).
- `go build + go vet + go test -race -run "TestSkills|TestBreakReasons" ./internal/catalog/...` exits 0 with 30 passing tests.

## Open Items

None. The plan's success criteria are fully met. Wave 4 templated entities (agents + skills + break_reasons) now establish three independent confirmations of the canonical CRUD shape; Plan 03-08 (queues + channels with FK probe) and Plan 03-09 (adapters with JSONB + agent_skills with join semantics) can proceed with high confidence in the template stability.
