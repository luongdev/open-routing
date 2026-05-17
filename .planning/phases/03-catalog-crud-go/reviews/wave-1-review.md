---
wave: 1
phase: 3
reviewers: [codex, gemini]
reviewed_at: 2026-05-16T14:18:22Z
diff_base: 6a8f089
diff_head: e5844ddc4220e6dc6380bd2e55922502f328e67b
verdicts: { codex: "READY WITH FIXES", gemini: "READY WITH FIXES" }
---

# Wave 1 Cross-AI Review — Migration 002 + internal/cache pkg

## Codex Review

## Summary
Wave 1 phần lớn đúng hướng: migration tạo đúng 7 bảng, không có FK cross-catalog, có `org_id` trên `agent_skills`, có proficiency CHECK, cache package tách riêng và miss-path dùng singleflight. Tuy nhiên còn vi phạm locked contract ở schema và thiếu vài edge-test/cache concurrency guard.

## Strengths
- `_scaffold` được drop trong up và recreate trong down đúng D-77.
- Không thấy DB FK cross-catalog, phù hợp D-76.
- `agent_skills.org_id` có mặt, proficiency có `CHECK (proficiency BETWEEN 1 AND 10)`.
- Cache có `rdb + sf + logger`, key namespace đúng với string input, hit path không đi qua singleflight.
- Refresh-ahead dùng `context.WithoutCancel`.

## Concerns
- [HIGH] `agent_skills` thiếu `version INTEGER NOT NULL DEFAULT 1`, vi phạm CAT-08 “every catalog table”. `migrations/000002_catalog_v0_1.up.sql:165`
- [MED] `break_reasons` thiếu D-64 partial functional name-search index. Hiện chỉ có cursor index và display-order index, chưa có `(org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE`. `migrations/000002_catalog_v0_1.up.sql:148`
- [MED] `db:reset` dễ fail khi DB còn active connections và không bật `ON_ERROR_STOP`; `DROP DATABASE` fail có thể để reset ở trạng thái không rõ. `Taskfile.yml:44`
- [MED] Miss-path singleflight dùng request `ctx` của caller đầu tiên cho shared DB load; nếu caller đó cancel, toàn bộ waiters cùng key nhận lỗi dù request của họ còn sống. `services/api/internal/cache/cache.go:174`
- [LOW] Test “error/typegen” chưa đủ: có Del Redis error, nhưng chưa cover Get/Set Redis failure fallback, corrupt JSON fallback, và generic type mismatch/key collision. `services/api/internal/cache/cache_test.go`
- [LOW] `Key(... any)` với `%s` có thể tạo `%!s(...)` cho type không phải string/Stringer, trong khi comment nói nhận `pgtype.UUID`. `services/api/internal/cache/cache.go:91`

## Suggestions
- Add `version` to `agent_skills`; consider `updated_at` too if proficiency updates use optimistic locking.
- Add `ix_break_reasons_org_name`.
- Make `db:reset` use `psql -v ON_ERROR_STOP=1` and terminate active connections or `DROP DATABASE ... WITH (FORCE)` if supported.
- For miss dedup, consider a detached bounded context for the shared load, or explicitly document cancellation semantics.
- Add cache tests for Redis get/set outage fallback, corrupt cached JSON fallback, and wrong generic type for same key.

## Verdict
READY WITH FIXES

---

## Gemini Review

## Summary
The migration and cache package are solidly implemented, honoring the vast majority of the locked decisions. The single-file migration pattern is clean, the tables include the required multi-org isolation guarantees (`org_id`), and the cache deduplicates read-through misses effectively while safely backgrounding refresh-aheads. There are a few edge cases related to connection concurrency during database resets and goroutine lifecycle in the cache background refreshes that should be ironed out.

## Strengths
- Precise adherence to locked rules (D-49 through D-77) including singleflight deduplication and single-file editable migration structure.
- Excellent cache test coverage spanning concurrency, time-travel refresh logic, and error handling.
- Caching logic is elegant; separating singleflight `key` from `refresh:key` isolates foreground and background requests perfectly.
- Clean rollback implementation via `_scaffold` reconstruction in `down.sql`.

## Concerns
- [MED] `Taskfile.yml:46`: `DROP DATABASE IF EXISTS openrouting` will fail if the API (`air`) is running and actively connected to the database. Since you are using PostgreSQL 17, you should append `WITH (FORCE)` to safely terminate active connections before dropping.
- [MED] `services/api/internal/cache/cache.go:138`: The async refresh goroutine uses `context.WithoutCancel(ctx)` but does not apply a new timeout. If the database `load()` hangs, this goroutine will leak indefinitely. Wrap it in a timeout (e.g., `bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)`) and ensure `cancel()` is deferred.
- [LOW] `migrations/000002_catalog_v0_1.up.sql:155`: The `break_reasons` table is missing the partial functional index for case-insensitive name search (`text_pattern_ops`) that was applied to all other entities. If the admin UI filters break reasons by name, it will perform a sequential scan.

## Suggestions
- The `agent_skills` join table lacks a `version` column and `updated_at` timestamp. While it's a join table, if the `proficiency` value is intended to be mutable in the future, you may want to add these columns for optimistic locking parity with the rest of the catalog (CAT-08).

## Verdict
READY WITH FIXES

---

## Consensus + Applied Fixes

**Verdicts:** both reviewers READY WITH FIXES — no BLOCK.

**Fixes applied this turn:**
| Reviewer | Severity | Finding | Applied? |
|---|---|---|---|
| Codex | HIGH | `agent_skills` missing `version` column | ❌ Skipped — false positive. agent_skills is a junction table with full-replace semantics (DELETE+INSERT in Plan 03-09), not UPDATE; Plan 03-02 must_haves only specify denormalized `org_id` + proficiency CHECK for agent_skills. The 6 catalog entities (agents/skills/queues/channels/adapters/break_reasons) carry version per CAT-08. |
| Codex | MED | `break_reasons` missing partial functional name index | ✅ Added `ix_break_reasons_org_name` (D-64 universal) |
| Codex/Gemini | MED | `db:reset` doesn't handle active connections / no ON_ERROR_STOP | ✅ Added `WITH (FORCE)` + `ON_ERROR_STOP=1` |
| Gemini | MED | Cache refresh goroutine has no timeout — leak risk on hung load | ✅ Added `refreshTimeout = 10s` constant + `context.WithTimeout` wrap + `defer bgCancel()` |
| Codex | LOW | `cache.Key(...)` `%s` produces `%!s(...)` for non-Stringer types | ✅ Changed to `%v` (defensive against bad caller usage) |
| Codex | MED | Singleflight uses first caller's ctx — if cancelled, waiters error | ⏭ Deferred. Real concern but complex to fix without redesigning the sf wrap. Tracked for Phase 4/5 hardening pass. |
| Codex | LOW | Cache test coverage gaps (Redis outage fallback, corrupt JSON, wrong generic type) | ⏭ Deferred. Defense-in-depth; main paths covered. Tracked for hardening pass. |

**Verification:** `go build + go vet + go test -race` on `services/api/internal/cache/` exits 0 with 8 tests passing after fixes.
