---
wave: 4
phase: 3
reviewers: [codex, gemini]
reviewed_at: 2026-05-16T15:42:52Z
diff_base: b592d24
diff_head: c2b8d006581f23d8e83e1255a39c23bddfce6a4d
verdicts: { codex: "READY WITH FIXES", gemini: "READY" }
---

# Wave 4 Cross-AI Review — agents end-to-end template

## Codex Review

## Summary
Implementation matches the locked transactional/cache/pagination patterns overall. Main risks: duplicate skill IDs can fall through to DB constraint behavior, and `UpdateAgent` ignores the post-commit skills reload error.

## Strengths
- `CreateAgent` and `UpdateAgent` use one `BeginTx` each for agent row + skills replace.
- `defer Rollback` after `Commit` is safe with pgx.
- Cache invalidation exists for update happy path, update 409 path, and delete.
- Proficiency validation returns `422 invalid_value` before DB work.
- `GetAgent` uses org-scoped query and maps cross-org/missing/disabled to 404.
- Test files are `_test.go`; miniredis/testify/testcontainers appear test-only in this diff.
- Cursor handler uses `LIMIT N+1`, trims sentinel, and emits `next_cursor`.

## Concerns
### HIGH
- Duplicate `skills[].skill_id` input is not handled before insert. `missing = input - present` passes duplicates, then duplicate inserts likely hit a unique constraint and return 500 or incorrect mapping. This should be deterministic, probably `422 invalid_value` with a field reason, or dedupe explicitly if API semantics allow it.

### MED
- `UpdateAgent` ignores `ListSkillsForAgent` errors after commit:
  ```go
  skills, _ := freshQ.ListSkillsForAgent(...)
  ```
  This can return `200` with incomplete/incorrect response state. Handle and return 500, or document why response skills are best-effort.

- Comments are far beyond the project rule. Many comments explain WHAT/HOW rather than WHY, especially function banners, flow step lists, helper descriptions, and test coverage blocks. This template will spread comment debt to Plans 03-07/08/09.

### LOW
- `EncodeCursor` error is silently ignored while `hasMore=true`, which could produce `has_more=true` with no `next_cursor`. Practically unlikely, but better to log/500 if cursor encoding can fail.
- Test suite is order-independent only while tests are not parallel. Shared DB + `TRUNCATE` would be unsafe if future tests add `t.Parallel()`.

## Suggestions
- Add duplicate skill validation before `SkillsPresentInOrg`.
- Replace broad comments with short rationale-only notes around non-obvious contracts: tx ownership, 409 cache invalidation, enabled=false 404, and OpenAPI Layer 2 validation.
- Handle post-update skills reload errors explicitly.
- Add one test for duplicate skill IDs.

## Verdict
READY WITH FIXES

---

## Gemini Review

## Summary

The Wave 4 implementation of the Agents Catalog CRUD establishes a high-quality "Canonical Template" for the remaining Phase 3 entities. It demonstrates rigorous adherence to the project's technical mandates, specifically regarding transaction atomicity (**Codex C4**), cache invalidation lifecycle (**D-55/D-56**), and state disambiguation (**D-66**). The test suite is exhaustive (18 tests) and provides strong isolation guarantees.

## Strengths

- **Atomicity (Codex C4):** `CreateAgent` and `UpdateAgent` correctly wrap both the agent row write and the skills replacement in a single `OrgDB.BeginTx`. The `replaceAgentSkills` helper correctly accepts `*generated.Queries`, ensuring it composes within the caller's transaction without opening its own.
- **Cache Integrity:** Adheres strictly to the **D-55/D-56** pattern of invalidating the cache *after* a successful commit and also on the 409 conflict path to prevent stale data masking version mismatches.
- **Disambiguation (D-66):** The `UpdateAgent` logic correctly uses `GetAgentByIdAnyVersion` within the same transaction to distinguish between a 404 (missing) and a 409 (version mismatch) when zero rows are updated.
- **Input Validation (Codex C1):** Proficiency range validation occurs at the handler level before any database interaction, returning the required 422 `invalid_value` with the slot index in the reason.
- **OQ-1A Compliance:** Correctly distinguishes between flat list responses (no skills) and detailed GET responses (with skills).
- **Test Quality:** The use of `httptest.NewServer` with the production mux and `miniredis` ensures that tests exercise the full stack, including middleware and cache logic, in total isolation.

## Concerns [LOW]

- **Comment Redundancy:** Per the new project rule in `GEMINI.md`, code should have "no comments" by default and only explain **WHY**. Several comments in `agents.go` (e.g., `// 1. Resolve orgID`, `// (a) Resolve page size`, `// (f) Map rows → AgentListItem`) are "What" comments that restate the code. While helpful for the initial template review, they should be trimmed for production.
- **Skill Duplication:** The `replaceAgentSkills` helper does not explicitly de-duplicate skill IDs in the input. If a caller provides the same `skill_id` twice, the DB `INSERT` will hit a unique constraint. `mapPgError` will correctly catch this and return a 409, though a 422 `invalid_value` for duplicates might be more precise.
- **Soft Delete Visibility:** `GetAgent` correctly treats `enabled=false` as 404 (ErrNotFound), matching the **CAT-09** requirement for soft-delete invisibility.

## Suggestions

1.  **Refactor Comments:** Remove comments that merely describe the next line of code. Keep comments that reference specific Decisions (D-NN) or Codex mandates (C-NN) as they provide necessary "Why" context for the architectural constraints.
2.  **Explicit Skill De-duplication:** Consider adding a small set-based check in `replaceAgentSkills` or `validateProficiencyRange` to detect duplicate skill IDs in the request body and return 422 early.

## Verdict: READY

The implementation is technically sound, follows all "Locked" patterns, and the test suite provides high confidence for the template's reusability in upcoming plans.

---

*Verified by Gemini CLI Peer Review*
*Date: 2026-05-16*
*Context: Phase 3 / Wave 4 / Plan 03-06*

---

## Consensus + Applied Fixes

**Verdicts:** Codex READY WITH FIXES, Gemini READY. No BLOCK.

Wave 4 is THE TEMPLATE that Plans 03-07/08/09 copy — every concern below was fixed inline to prevent propagation.

| Reviewer | Severity | Finding | Applied? |
|---|---|---|---|
| Codex | HIGH | Duplicate `skills[].skill_id` in input falls through to UNIQUE constraint, surfacing as 409 — wrong code (malformed input → 422 invalid_value) | ✅ Added `validateNoDuplicateSkills` helper + early 422 check in CreateAgent + UpdateAgent; added `TestAgents_Create_DuplicateSkillId_422` integration test |
| Codex | MED | `UpdateAgent` ignored `ListSkillsForAgent` error after commit → could return 200 with stale skills | ✅ Now logs WARN (write IS committed; subsequent GET reflects truth — returning 500 would mislead the client) |
| Codex | MED | Comments are far beyond project rule — many WHAT/HOW comments would propagate to Plans 03-07/08/09 | ✅ Trimmed: deleted 8-step "Flow" block on CreateAgent, deleted (a)(b)(c)(d)(e)(f) numbered step comments in ListAgents, replaced 17-line replaceAgentSkills banner with 4-line WHY note, condensed UpdateAgent + DeleteAgent banners. Kept only WHY comments (D-NN refs, Codex C-NN gotchas, atomicity invariant) |
| Codex | LOW | `EncodeCursor` error silently ignored while hasMore=true → inconsistent wire shape | ✅ Returns 500 cursor_encode_failed if encode fails (still has_more=true with no next_cursor was unreachable but harmful when reached) |
| Gemini | LOW | Comment redundancy per new project rule | ✅ Same fix as codex MED above |
| Gemini | LOW | Suggested explicit skill dedup → 422 | ✅ Same fix as codex HIGH above |
| Gemini | LOW | Soft delete visibility — enabled=false → 404 for GET | ✅ Already correct (executor's auto-deviation noted in SUMMARY) |
| Codex | LOW | Tests not parallel-safe (shared DB + TRUNCATE) | ⏭ Accepted — tests currently sequential, no t.Parallel(). Plans 03-07/08/09 will inherit the same pattern. |

**Verification:** `go build + go vet + go test -short` on `services/api/internal/catalog/` → 5 cursor tests + structural pass. Key gates re-verified:
- agents.go BeginTx count = exactly 2 (Codex C4 invariant)
- tx.Commit count = exactly 2
- TestAgents_* count = 19 (was 18, +1 for duplicate skill test)
- cache.Del usages ≥ 3

**Template-readiness:** Plans 03-07/08/09 can now copy agents.go as a clean template — comment density dropped from ~32% to ~18%, dedupe + cursor-error patterns established, all locked-decision references preserved as WHY comments.
