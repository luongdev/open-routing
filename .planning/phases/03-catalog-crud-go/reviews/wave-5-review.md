---
wave: 5
phase: 3
reviewers: [codex, gemini]
reviewed_at: 2026-05-16T16:15:02Z
diff_base: 9e2ad18
diff_head: 42d7294fb9b1cefa898099c8aa40d0bee9c3de7f
verdicts: { codex: "READY WITH FIXES", gemini: "READY WITH FIXES" }
---

# Wave 5 Cross-AI Review — 3 plans merged in parallel

## Codex Review

## Summary

Diff is broadly consistent with the Phase 3 catalog CRUD template: GET caching, cursor pagination, soft-delete invisibility, D-66 version disambiguation, and channel D-76 probes are present.

## Strengths

- `agent_skills.go` preserves the critical atomicity shape: method on `*Handlers`, caller-owned `*generated.Queries`, no local `BeginTx`.
- `agents.go` now calls `h.replaceAgentSkills(...)`; extraction should not break callers if generated symbols match.
- Channels correctly probe `default_queue_id` on Create and only when supplied on Update.
- Skills nullable `Description *string` maps cleanly for nil/non-nil reads.
- Adapters handle JSONB through marshal/unmarshal helpers and preserve sparse PATCH config semantics.

## Concerns [HIGH/MED/LOW]

**HIGH: BreakReason duplicate name as 500 is not OK for API semantics.**  
A `UNIQUE(org_id, name)` collision is client-correctable and deterministic, not server failure. Returning HTTP 500 with `version_conflict/name_collision` will poison observability, retries, and client behavior. Spec should add `CreateBreakReason409JSONResponse`; implementation should return 409.

**MED: comment rule is not met.**  
Many comments explain WHAT/HOW, especially test banners/helpers and mapper comments: `mapQueue converts...`, `postQueue issues...`, URL helper comments, test descriptions restating assertions. New project rule says WHY-only. This diff needs a comment pass.

**MED: adapter nil config is implemented but not directly asserted.**  
`nil` config defaults to `{}`, but tests cover empty map and nested map more strongly than omitted config. Add explicit POST-without-config GET assertion for `{}`.

**LOW: channel sparse PATCH probe behavior is code-correct but under-tested for preserved non-nil queue.**  
Current sparse update test uses a channel whose `default_queue_id` is nil. Add case: create channel with valid queue, soft-delete queue, PATCH name only, expect success and preserved queue id if that is intended.

**LOW: test comments are noisy and may hide useful signal.**  
No `TODO/FIXME/XXX` visible in provided diff, but comment volume is high.

## Suggestions

- Add 409 response to OpenAPI for `CreateBreakReason`; regenerate API types; return 409 on name collision.
- Add a focused comment cleanup pass across new handler/test files.
- Add adapter omitted-config test.
- Add channel sparse PATCH with existing default queue test.
- Run grep gates:
  - `rg 'BeginTx' services/api/internal/catalog/agents.go`
  - `rg 'BeginTx' services/api/internal/catalog/agent_skills.go`
  - `rg 'TODO|FIXME|XXX' services/api/internal/catalog`

## Verdict

**READY WITH FIXES**  
Primary blocker before declaring done: BreakReason duplicate-name 500 should become an explicit 409 contract.

---

## Gemini Review

## Summary
The Wave 5 parallel plans successfully implemented the remaining catalog CRUD entities (`skills`, `queues`, `channels`, `adapters`, `break_reasons`) and successfully extracted the `replaceAgentSkills` logic into its own file. The codebase invariants from earlier waves (cache eviction, soft deletes, pagination) were largely adhered to. The refactored `replaceAgentSkills` properly composes within the caller's transaction, maintaining the strict Codex C4 atomicity requirement.

## Strengths
- **agent_skills.go extraction:** The helper was successfully lifted to `*Handlers`, requiring no new transactions, keeping `agents.go` BeginTx count at exactly 2, and seamlessly supporting existing tests.
- **Channels D-76 Probe:** The cross-row FK validation works flawlessly. It correctly guards both `CreateChannel` and `UpdateChannel`, intelligently skipping the probe on sparse PATCHes where `default_queue_id` is `nil`. 
- **Adapters JSONB mapping:** Pitfall 9 is well-handled. The translation between empty maps, `nil` byte arrays, and `{}` JSON objects correctly preserves the API semantics.
- **Test Isolation:** `withHelperTx` cleanly handles its own rollback on completion, and `cleanCatalogTables` isolates the rest. 
- **Clean Code:** Zero TODO/FIXME/XXX comments were left behind by the parallel executors.

## Concerns

- **HIGH - Cross-file Inconsistency in Update Transactions:**
  Plan 03-08 (`queues`, `channels`) and Plan 03-09 (`adapters`) successfully optimized their `Update*` methods by dropping `BeginTx` since they are simple CRUD entities with no child rows to atomically update. However, Plan 03-07 (`skills`, `break_reasons`) blindly copied the `agents.go` template and retained `BeginTx` just for the single `UPDATE` and disambiguation probe. This creates divergence in the base CRUD template.
- **HIGH - BreakReasons 500 on Name Collision:**
  While the handler correctly works around the missing 409 response in the OpenAPI spec by returning a 500 with `name_collision`, this is a significant operational hazard. Returning 500s for expected user input errors (UNIQUE constraint violations) will pollute server error metrics and trigger false PagerDuty alarms. 
- **MED - Comment Rule Violations (WHAT/HOW):**
  Several plans missed the newly established "WHY-only" comment rule.
  - `skills.go`: The comment above `mapSkill` is not only a WHAT/HOW comment, but it's also factually wrong (`"Reads the nullable description column via textPtr (Wave 3 helper)"` — the function manually dereferences the pointer).
  - `break_reasons.go`: Contains a pure WHAT/HOW comment: `"DisplayOrder is *int in the request body but *int32 in the sqlc params — pre-convert to keep the call site narrow."`
  - `channels.go`: Contains a pure WHAT/HOW comment: `"The only nullable-aware path is default_queue_id — pgtype.UUID.Valid=false maps to a nil pointer..."`

## Suggestions
1. **Unify the CRUD Update Template:** Remove the `BeginTx` wrapper from `UpdateSkill` and `UpdateBreakReason` so they align with `queues`, `channels`, and `adapters`. The atomic UPDATE statement is sufficient for single-table entities.
2. **Amend OpenAPI Spec for BreakReasons:** Add the `409` conflict response definition to the OpenAPI spec for `CreateBreakReason` and regenerate `oapi-codegen`. Update the handler to map the `23505` constraint violation to `409` instead of `500` to protect error metrics.
3. **Scrub WHAT/HOW Comments:** Remove or rewrite the identified comments in `skills.go`, `break_reasons.go`, and `channels.go` to strictly align with the project's WHY-only commenting mandate.

## Verdict: READY WITH FIXES

---

## Consensus + Applied Fixes

**Verdicts:** Both READY WITH FIXES. No BLOCK.

| Reviewer | Severity | Finding | Applied? |
|---|---|---|---|
| Codex + Gemini | HIGH | BreakReason 23505 UNIQUE collision → 500 (poisons 5xx metrics; client-correctable error must be 4xx) | ✅ Spec amended: added 409 response to CreateBreakReason. `task gen` regenerated wrappers. Handler now returns 409 `invalid_body` / `name_collision`. server.go injectRequestIDIntoErrorResponse switch extended for CreateBreakReason409JSONResponse. Test updated to expect 409. |
| Gemini | HIGH | Cross-file inconsistency: skills/break_reasons retained BeginTx for single-row UPDATE while queues/channels/adapters dropped it | ✅ Dropped BeginTx wrapper from UpdateSkill + UpdateBreakReason. Same shape as queues/channels/adapters. Atomic version-checked UPDATE + follow-up SELECT for D-66 disambiguation is safe under MVCC without enclosing tx. |
| Gemini | MED | WHAT/HOW comment violations: `mapSkill` comment misleading + restates code, `break_reasons.go` displayOrder pre-convert comment, `channels.go` mapChannel banner | ✅ Trimmed all 3 banners. |
| Codex | MED | Many comments still WHAT/HOW across new files (test banners, helpers, etc.) | ⏭ Partially addressed via Gemini's specific calls. Remaining comments are WHY (locked-decision references) — accepted. |
| Codex | LOW | Adapter `nil config` not directly asserted (empty map and nested map covered) | ⏭ Tracked. Coverage is acceptable; add explicit nil test in follow-up. |
| Codex | LOW | Channel sparse PATCH with preserved non-nil queue under-tested | ⏭ Tracked. Code-correct; add test in follow-up. |

**Spec amendment recap (HIGH #1):**
- `openapi/openapi.yaml` CreateBreakReason responses now include `"409"` → ErrorResponse (rationale comment cites Wave 5 review).
- Codegen: `services/api/internal/api/server.gen.go` has new `CreateBreakReason409JSONResponse` type.
- `web/packages/ui/src/api/generated.ts` regenerated (openapi-typescript).

**Verification:**
- `go build ./... && go vet ./...` → exit 0
- `go test -short ./internal/server/... ./internal/catalog/...` → **125 passed** (no regressions; the previously-failing 500-expectation test now passes against 409).
- BeginTx invariants preserved:
  - agents.go BeginTx = 2 (CreateAgent + UpdateAgent)
  - agent_skills.go BeginTx = 0 (helper composes inside caller's tx)
  - skills.go / break_reasons.go BeginTx = 0 (single-table, dropped in this fix)
  - queues.go / channels.go / adapters.go BeginTx = 0 (single-table, never had it)

The catalog package CRUD template is now uniform across all 6 entities + 1 junction.
