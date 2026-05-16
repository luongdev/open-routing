---
wave: 2
phase: 3
reviewers: [codex, gemini]
reviewed_at: 2026-05-16T14:48:57Z
diff_base: 049b443
diff_head: eb21b146f6aa5accdcc655f932b1e7090ed59c36
verdicts: { codex: "READY WITH FIXES", gemini: "READY" }
---

# Wave 2 Cross-AI Review — sqlc queries + OrgDB.BeginTx

## Codex Review

## Summary
Wave 2 mostly honors the locked SQL/query-shape decisions: per-entity query files exist, list variants are differentiated, versioned updates are atomic, `OrgTx` matches `generated.DBTX`, and SQL args are parameterized. Main unresolved risk is that the `SkillsPresentInOrg` inversion is only documented here; handler-side set-difference is not present in this diff, so end-to-end missing-skill semantics cannot be verified yet.

## Strengths
- All 6 entity `UPDATE` queries use `COALESCE(sqlc.narg(...), column)` on mutable fields.
- Version checks are atomic: `UPDATE ... WHERE id ... AND org_id ... AND version = expected RETURNING ...`.
- Default list queries include `enabled = TRUE`; `IncludingDisabled` variants omit it.
- `QueueExistsAndEnabledInOrg` and `SkillsPresentInOrg` keep tenant tables in the outer `FROM`, matching validator constraints.
- `OrgTx` implements `Exec`, `Query`, `QueryRow` with `generated.DBTX` compile-time assertion.
- Placeholder generated/query files were removed and replaced with catalog generated files.

## Concerns
- [MED] `services/api/internal/db/queries/agent_skills.sql:57` — `SkillsPresentInOrg` now returns present IDs, but no handler code is in this diff to prove callers compute `input - present`. If Wave 3 handlers still expect missing IDs, invalid references will invert behavior.
- [LOW] `services/api/internal/db/queries/agents.sql:101` and equivalent soft-delete queries — soft delete uses `AND enabled = TRUE`, so re-deleting an already-disabled row returns 0. That is only idempotent if the handler intentionally maps already-deleted to 404; it is not idempotent-success semantics.
- [LOW] `services/api/internal/db/sqlcheck.go:17` — allowlist expansion preserves scaffold coverage, but policy remains fail-open for future tenant tables not added to `tenantTables`. Existing scaffold-era tests should still pass because `_scaffold` remains allowlisted.

## Suggestions
- Add a Wave 3 handler/unit test proving `SkillsPresentInOrg` set-difference returns 422 for missing/disabled/cross-org skill IDs.
- Clarify soft-delete API contract: repeated delete returns 404 vs 204. Current SQL supports 404.
- Consider a regression test that enumerates expected generated query files and fails if scaffold artifacts reappear.

## Verdict
READY WITH FIXES

---

## Gemini Review

## Summary
The Wave 2 diff successfully implements the SQL layer for the Catalog CRUD foundation, strictly adhering to the architectural constraints defined in Phase 3. The implementation provides a clean, per-entity query structure with robust support for sparse-PATCH semantics via `COALESCE`, atomic version-controlled updates, and efficient composite-cursor pagination. Notably, the introduction of `OrgTx` ensures that the multi-tenancy security invariants (enforced by the `SQLChecker`) are preserved even within transactional boundaries, which is critical for the complex handlers planned for Wave 3.

## Strengths
- **Security Integrity:** The extraction and reuse of `preflightSQL` logic across both `OrgDB` and `OrgTx` ensures a consistent and non-bypassable multi-tenancy validation layer.
- **Sparse-PATCH Implementation:** The use of `sqlc.narg` combined with `COALESCE` in `UPDATE` queries provides a clean and idiomatic way to handle partial updates without zeroing out omitted fields.
- **D-66 Pattern Adherence:** The inclusion of `Get{Entity}ByIdAnyVersion` probes across all entities provides the necessary disambiguation logic for handlers to distinguish between 404 (Not Found) and 409 (Conflict) responses.
- **Precise Pagination:** The composite cursor predicate `(created_at, id) < (...)` correctly implements stable pagination for descending sort orders, ensuring the tie-breaker `id` works as intended.
- **FK Probe Safety:** The "inversion" of `SkillsPresentInOrg` effectively balances the need for cross-table validation with the strict requirements of the `SQLChecker` by ensuring tenant tables are always at the top-level `FROM`.

## Concerns
- [LOW] **Immutable External IDs:** `external_id` is excluded from all `UPDATE` queries. This is consistent with it being a stable integration key, but any future requirement to rotate these IDs will require manual SQL updates.
- [LOW] **Junction Denormalization:** `agent_skills` denormalizes `org_id`. While required for the `SQLChecker` to function, it places the burden of ensuring `org_id` consistency on the handler layer (correctly noted in the comments).

## Suggestions
- **Set Difference Utility:** For the upcoming handler implementation of `SkillsPresentInOrg`, consider adding a generic `setDiff` utility in a shared package to keep handler code lean as more junction tables are introduced.
- **Soft-Delete Idempotency:** The `SoftDelete` queries correctly use `WHERE enabled = TRUE`. Ensure the handler layer consistently maps `0 rows affected` to a `404 Not Found` to maintain the API contract.

## Verdict
READY

---

## Consensus + Disposition

**Verdicts:** Codex READY WITH FIXES, Gemini READY. No HIGH concerns. No BLOCK.

| Reviewer | Severity | Finding | Disposition |
|---|---|---|---|
| Codex | MED | `SkillsPresentInOrg` inversion needs handler-side set-difference (caller computes `input - present`) | ⏭ **Forward-tracked for Wave 5 / Plan 03-09 agent_skills handler.** Wave 2 only added the query; handler logic lives in Plan 03-09. Plan 03-09's `replaceAgentSkills` helper already plans to compute missing IDs from the present set (per iter 3 plan revisions). Tracked. |
| Codex | LOW | Soft-delete `WHERE enabled = TRUE` makes re-delete return 0 → handler 404, not idempotent-success | ✅ **Accepted as designed.** ROADMAP CRIT 1 doesn't require idempotent-success; 404 on already-deleted is the documented contract per D-65. |
| Codex | LOW | SQLChecker allowlist policy is fail-open for future tenant tables not added | ✅ **Accepted as v0.1 design.** Defense-in-depth concern; revisit when Phase 4 adds `agent_states` table. |
| Gemini | LOW | `external_id` immutable in UPDATE queries — stable integration key, can't be rotated | ✅ **Accepted as designed.** external_id is a stable IDP-side identifier; rotation requires manual SQL. |
| Gemini | LOW | `agent_skills` denormalized org_id places consistency burden on handler | ✅ **Accepted (D-72 H3 locked).** Handler must set org_id from agent's org_id when inserting; trivial in the replace pattern. |

**Wave 2 dispositions are all "accepted" or "forward-tracked"; no immediate fixes needed.** Proceeding to Wave 3 (catalog skeleton).

**Verification:** post-merge `go build ./... && go vet ./... && go test -count=1 -short ./internal/db/...` exits 0 (58 tests pass — orgdb_test.go: TestOrgDB_BeginTx_PreservesValidator + 5 sub-tests).
