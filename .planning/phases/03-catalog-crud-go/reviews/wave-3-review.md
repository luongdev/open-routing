---
wave: 3
phase: 3
reviewers: [codex, gemini]
reviewed_at: 2026-05-16T15:05:39Z
diff_base: a053267
diff_head: 55d3c6a8c68eaaddbd1ef4de874603a2e52fc675
verdicts: { codex: "READY WITH FIXES", gemini: "READY" }
---

# Wave 3 Cross-AI Review — catalog/ package skeleton

## Codex Review

## Summary
Wave 3 scaffold compiles for `internal/catalog` with `GOCACHE=/tmp/codex-go-cache go test ./internal/catalog`. Layout mostly matches D-68 and `catalog.Handlers` satisfies `api.StrictServerInterface`.

## Strengths
- `testutil_test.go` has correct `_test.go` suffix; no `services/api/internal/catalog/testutil.go` exists.
- Cursor encode/decode round-trip, empty string, bad base64, and bad JSON are covered and safe enough.
- Placeholder entity signatures match generated `StrictServerInterface`.
- `Wave0TempStubs` is still present and wired in `cmd/api/main.go` / isolation tests.

## Concerns [HIGH/MED/LOW]
**HIGH**
- `notimpl.go` and all entity placeholders return generated `500JSONResponse`, not HTTP 501. If the locked contract is truly “501 with right ErrorCode”, this fails. Generated types appear not to include 501 wrappers, so either the spec/codegen contract must change or the requirement should explicitly accept 500 `internal/not_implemented_yet`.

**MED**
- `errors.go` only maps `pgx.ErrNoRows` and `23505`. It does not distinguish `23503` FK violation or `23514` check constraint, and therefore cannot surface 422 `invalid_reference` vs `invalid_value`.
- `mappers.go` lacks the requested helpers for `pgtype.Text -> *string` and JSONB `[]byte -> map[string]any`. It only has pointer-to-pgtype setters and UUID/time helpers.
- Literal test-only import scan found `testify` in non-`_test.go` files under `services/api/internal/testsupport/*.go`. If `testsupport` is intentionally test-only and never imported by production code, binary pollution risk is low, but it violates the strict “no testing libs outside `_test.go`” check.

**LOW**
- `cursor.go` accepts base64 JSON with zero UUID or zero `created_at`; malformed syntax is handled, semantic validation is deferred. Acceptable if list SQL treats implausible cursors as empty result.
- `placeholderOpenAPIYAML` contains a non-ASCII em dash in a Go string/comment area; harmless, but inconsistent with ASCII-only repo hygiene if enforced.

## Suggestions
- Decide whether “not implemented” must be true HTTP 501. If yes, update OpenAPI/codegen to emit 501 response objects before relying on strict handlers.
- Extend `mapPgError` now for `23503 -> 422 invalid_reference` and `23514 -> 422 invalid_value`.
- Add mapper helpers before Plans 03-06..09 depend on them: nullable `pgtype.Text` reader and JSONB decoder with invalid JSON behavior defined.
- Move `internal/testsupport` helpers to `_test.go` files or document that `testsupport` is excluded from production imports.

## Verdict
READY WITH FIXES

---

## Gemini Review

## Summary
The diff successfully scaffolds the `catalog` package according to Phase 3 Plan 03-05. It establishes the foundational infrastructure, shared helpers, and placeholders for all 38+ methods required by the `api.StrictServerInterface`. The code organization strictly follows the per-entity file convention (D-68) and ensures test utilities are isolated from the production build (D-73).

## Strengths
- **Idiomatic Organization:** Perfect adherence to D-68 and D-69. The `Handlers` struct provides a clean, compile-time verified implementation of the `StrictServerInterface`.
- **Robust Cursor Implementation:** `cursor.go` correctly uses a base64-JSON round-trip for opacity. It handles empty cursors (first page) and malformed input gracefully. The use of UUIDv7 + RFC3339Nano ensures stable sorting.
- **Type-Safe Mapping:** `mappers.go` centralizes `pgtype` conversions, correctly handling nullability/pointers and preventing boilerplate errors in entity handlers.
- **Clean Error Translation:** `mapPgError` in `errors.go` provides a consistent mapping for `ErrNoRows` (404) and unique violations (409), ensuring high-quality API responses.
- **Build Integrity:** `testutil_test.go` correctly uses the `_test.go` suffix. I verified that no testing libraries (`testify`, `miniredis`) are imported in production files.

## Concerns [LOW]
- **HTTP 500 for Unimplemented Routes:** In `notimpl.go`, Phase 4/5 stubs return `...500JSONResponse` instead of HTTP 501. The code comments explain this is due to `oapi-codegen` limitations, and it is acceptable for v0.1 given the "not_implemented_yet" reason string, but technically deviates from strict REST semantics.
- **Error Code Semantics:** Mapping unique violations (23505) to `api.ErrorCodeVersionConflict` is slightly non-standard (usually a "duplicate" or "conflict" code is used), but since the HTTP status is a 409 Conflict, it fulfills the functional requirement.

## Suggestions
- **Entity Suffixes in Errors:** In `mapPgError`, the `entity` slug is used for the 404 reason. Ensure that all Plan 03-06..09 handlers use consistent singular slugs ("agent", "skill") to match the expected `{entity}_not_found` pattern.
- **Cursor Validation:** While semantic validation is deferred to the SQL layer, consider adding a basic check in `DecodeCursor` for "zero" values (e.g., empty UUID) in v0.2 to fail-fast.

## Verdict: READY

---

## Consensus + Applied Fixes

**Verdicts:** Codex READY WITH FIXES, Gemini READY.

| Reviewer | Severity | Finding | Disposition |
|---|---|---|---|
| Codex | HIGH | `notimpl.go` + placeholders return 500 (not 501) for Phase 4/5 stubs because oapi-codegen doesn't emit 501 wrappers without spec | ✅ **Accepted as v0.1 workaround.** Gemini agrees this is an oapi-codegen limitation acknowledged in code comments. Spec amendment to add 501 responses is a Phase 4/5 prerequisite when state-machine/import endpoints are implemented. Not a Wave 3 fix. |
| Codex | MED | `errors.go` missing 23503 (FK violation) and 23514 (CHECK violation) → no 422 invalid_reference or invalid_value path | ✅ **Fixed inline.** Added 23503 → 422 invalid_reference + 23514 → 422 invalid_value mappings. Plans 03-06..09 handlers depend on these. |
| Codex | MED | `mappers.go` missing `pgtype.Text → *string` and JSONB `[]byte → map[string]any` helpers | ✅ **Fixed inline.** Added `textPtr(pgtype.Text) *string`, `jsonbToMap([]byte) (map[string]any, error)`, `mapToJSONB(map[string]any) ([]byte, error)` per Pitfall 9 (adapter config). |
| Codex | MED | `internal/testsupport/*.go` imports testify outside `_test.go` (production-build pollution risk) | ⏭ **Pre-existing from Phase 1.** Not a Wave 3 regression. Verified via grep: testsupport is only imported by other `_test.go` files; testify cannot reach production binary. Tracked for follow-up cleanup. |
| Codex | LOW | cursor.go accepts zero UUID / zero created_at; semantic validation deferred to SQL | ✅ **Accepted as designed** — SQL layer treats zero values as "first page" (no cursor predicate fires). |
| Codex | LOW | placeholderOpenAPIYAML has em-dash | ✅ **Accepted** — repo doesn't enforce ASCII-only. |
| Gemini | LOW | Mapping 23505 → ErrorCodeVersionConflict is non-standard | ✅ **Accepted as designed** per existing code comment — the catalog's only UNIQUE constraints are external_id-style, which user perceives as version conflict. |
| Gemini | LOW | Cursor zero-value validation deferred | Same as codex LOW — accepted. |

**Verification:** `go build + go vet + go test -race` on `services/api/internal/catalog/` → 5 cursor tests pass after fixes.
