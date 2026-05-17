---
phase: 3
review_type: simplicity
reviewed_at: 2026-05-16
scope: handwritten Phase 3 code plus inherited Phase 2 server glue
verdict: FIX EARLY (targeted)
---

# Phase 3 Simplicity Review

Purpose: track code that is technically working but already too expensive to maintain. Fix these before Phase 3 grows more handlers on top.

Verification run:

- `cd services/api && go test ./...` passed on 2026-05-16.

## Cross-AI Review Addendum

Reviewed by:

- Gemini CLI 0.41.2
- Claude Code CLI 2.1.142

Round 2 consensus:

- Keep `SQLChecker` as runtime defense-in-depth; add build-time audit only as a complement.
- Do not replace request-id injection with runtime reflection. If this is simplified later, use generated code/metadata or keep the explicit switch.
- Fix `mapPgError` conflict reasons early.
- Prune transitional stubs/placeholders as soon as real catalog wiring lands.
- Bound or otherwise harden `SQLChecker`'s cache.
- Reduce future GSD plan implementation-code bloat; keep source comments that explain stable decisions/invariants.

## Findings

### MEDIUM: request_id injection is overfit to generated response types

Evidence:

- `services/api/internal/server/server.go` has a long `injectRequestIDIntoErrorResponse` type switch over generated response wrapper types.
- `services/api/internal/server/request_id_exhaustiveness_test.go` mirrors the same generated type matrix in tests.

Problem:

- A tiny cross-cutting concern now requires editing production code and tests whenever OpenAPI adds/removes an error response type.
- The test mostly verifies that the manual list matches another manual list.
- This has already become hundreds of lines for `request_id` alone.

Early fix:

- Keep the explicit switch for now because it gives compile-time coverage over generated response wrappers.
- If this grows further, simplify by generating the switch from OpenAPI/codegen metadata. Avoid runtime reflection unless the team explicitly accepts the tradeoff.
- Keep one end-to-end HTTP test proving body `request_id` equals `X-Request-Id`.

### HIGH: temporary strict-server stubs and catalog placeholders overlap

Evidence:

- `services/api/internal/server/wave0_temp_stubs.go` implements every strict-server method as transitional 500 stubs.
- `services/api/internal/catalog/*` also contains placeholder entity methods and bypass placeholders.

Problem:

- There are now multiple "not implemented" layers that can drift independently.
- Route wiring depends on knowing which placeholder is actually active.
- This creates fake progress: code compiles, but multiple endpoints are still intentionally non-functional.

Early fix:

- Do not let `Wave0TempStubs` survive past the first catalog handler wiring step.
- Keep exactly one strict-server implementation in the final Phase 3 branch.
- Move real bypass handlers once, then delete placeholder bypass responses.

### MEDIUM: plans contain too much implementation code

Evidence:

- Phase 3 plan files are thousands of lines and include large code blocks.
- Several source comments reference plan/wave IDs instead of explaining stable runtime behavior.

Problem:

- Planner writes pseudo-code, executor rewrites real code, then future maintainers inherit both the source and the planning narrative.
- This increases token cost and makes the code look more complicated than the product behavior.

Early fix:

- Update GSD planning rule: `PLAN.md` should contain contracts, file map, edge cases, and tests. Full implementation code belongs in patches or execute phase only.
- Remove plan-history comments from source when the code graduates from placeholder to production.

### MEDIUM: SQLChecker is complex but still a partial guarantee

Evidence:

- `services/api/internal/db/sqlcheck.go` parses SQL AST at runtime and maintains its own tenant table allowlist.

Problem:

- The checker proves `org_id` appears in the statement shape, not that it is compared to the current org value.
- New tenant tables fail open unless added to `tenantTables`.
- This is a lot of machinery for a guard that still needs query review and tests.

Early fix:

- Keep runtime preflight as defense-in-depth, but add a build/test-time sqlc query audit as complementary coverage.
- Generate/derive tenant table coverage from migrations or make unknown tables fail closed.
- Bound the verdict cache or explicitly prove only static sqlc query strings reach it.
- Keep handler isolation tests as the real cross-org proof.

### LOW/MEDIUM: cache API is more generic than the product needs

Evidence:

- `cache.Key(orgID, entity, id any)` accepts any type.
- `cache.New` documents nil dependency panics instead of rejecting them.

Problem:

- The API is flexible in ways callers should not use.
- Bad keys or nil deps fail late.

Early fix:

- Prefer a narrower signature (`fmt.Stringer` or explicit supported types) over unconstrained `any`.
- Validate `New` dependencies or provide a no-op cache explicitly for tests/dev.

### MEDIUM: pg error mapping hides entity-specific conflicts

Evidence:

- `catalog.mapPgError` maps every `23505` unique violation to `external_id_collision`.
- `break_reasons` has no `external_id`; its uniqueness is `(org_id, name)`.

Problem:

- The shared helper is simple at the call site but wrong for at least one entity.
- Future handlers may return misleading reasons.

Early fix:

- Map conflict reasons by constraint name or pass the expected entity conflict reason from the handler.

### LOW: comments carry too much planning history

Evidence:

- Many source files include plan IDs, wave IDs, decision IDs, and review iteration notes.

Problem:

- Maintainers need behavior, invariants, and risk notes. They should not need to parse the planning timeline.

Early fix:

- Keep decision IDs in `.planning`.
- Keep source comments only where they explain runtime behavior that is not obvious from code.

## Recommended Fix Order

1. Tighten `mapPgError` before writing real CRUD handlers.
2. Delete transitional stubs/placeholders as soon as final catalog wiring exists.
3. Bound or harden `SQLChecker`'s cache and add build-time sqlc query audit as complementary coverage.
4. Tighten cache key/deps before handlers depend on cache everywhere.
5. Keep request-id type switch for now; only replace with generated code if it grows further.
6. Add GSD planning rule: no full implementation code in plans unless emitted as a patch artifact.
