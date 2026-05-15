---
phase: 01-foundation-polyglot-monorepo
plan: 1a
subsystem: infra
tags: [go, context, uuid, ctx-key, leaf-package]

# Dependency graph
requires:
  - phase: 01-foundation-polyglot-monorepo
    provides: "Plan 01 — services/api/go.mod with github.com/google/uuid v1.6.0 + module path github.com/luongdev/open-routing/services/api"
provides:
  - "Canonical orgkey leaf package at services/api/internal/db/orgkey with SetOrgID / OrgIDFromContext helpers and unexported orgIDKey type"
  - "Single source of truth for the org_id context key — eliminates the db ↔ middleware bidirectional import cycle by making both depend on this leaf"
  - "Type-safe context propagation of org_id as uuid.UUID (not string) for downstream consumers"
affects:
  - "Plan 03 (internal/db / orgDB) — imports orgkey for ctx preflight check"
  - "Plan 04 (internal/telemetry / slog TracingHandler) — imports orgkey for log-field injection"
  - "Plan 05 (internal/middleware / OrgContext) — imports orgkey for SetOrgID after header parsing"
  - "Plan 06 (handlers) — imports orgkey via OrgIDFromContext for org-scoped business logic"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pattern S1: unexported empty-struct context-key (collision-proof, single legal writer)"
    - "Pattern S2: uuid.UUID stored in ctx (not string) per D-20"

key-files:
  created:
    - services/api/internal/db/orgkey/orgkey.go
    - services/api/internal/db/orgkey/orgkey_test.go
  modified: []

key-decisions:
  - "orgkey lives at services/api/internal/db/orgkey (under internal/db, not internal/middleware) — co-locates with the package that owns the DB-side preflight (Plan 03)"
  - "Three tests instead of two (roundtrip, missing-key, parent-not-mutated) — covers the standard context.WithValue copy-on-write semantics so future refactors cannot silently mutate parents"

patterns-established:
  - "Pattern S1 (Context-key isolation): orgkey is the canonical implementation — unexported orgIDKey struct{}; SetOrgID + OrgIDFromContext are the only legal access paths"
  - "Leaf-package pattern for breaking import cycles: shared context-keys belong in their own dependency-free leaf so both sides of a potential cycle import the leaf"

requirements-completed: [FOUND-05]

# Metrics
duration: 2min
completed: 2026-05-15
---

# Phase 01 Plan 1a: Orgkey Leaf Package Summary

**Canonical context-key package (orgkey.SetOrgID / orgkey.OrgIDFromContext) extracted as a Wave 1 leaf to break the db ↔ middleware import cycle and unblock Wave 2 parallelism.**

## Performance

- **Duration:** 2 min (96 seconds)
- **Started:** 2026-05-15T10:32:37Z
- **Completed:** 2026-05-15T10:34:13Z
- **Tasks:** 1
- **Files modified:** 2 (both created)

## Accomplishments

- Created `services/api/internal/db/orgkey/orgkey.go` defining the canonical unexported `orgIDKey struct{}` context-key plus `SetOrgID` (returns derived ctx) and `OrgIDFromContext` (returns `(uuid.UUID, bool)` with presence flag).
- Created `services/api/internal/db/orgkey/orgkey_test.go` with 3 focused unit tests: roundtrip, missing-key, parent-not-mutated — all `t.Parallel()`-safe.
- Verified zero internal-package dependencies: package imports only `context` (stdlib) and `github.com/google/uuid v1.6.0`. The forbidden-imports grep over `services/api/internal/db/orgkey/` returns empty.
- Verified single source of truth: `grep -rE 'type\s+orgIDKey\s+struct' services/api/internal/` returns only this new file.
- `go vet`, `go build`, `go test -count=1` all green for `./internal/db/orgkey/...`.

## Task Commits

1. **Task 1: Create orgkey leaf package + unit test** — `e9f2a52` (feat)

**Plan metadata commit:** (this SUMMARY commit, see below)

## Files Created/Modified

- `services/api/internal/db/orgkey/orgkey.go` — Package orgkey. Holds unexported `orgIDKey struct{}` context-key type plus `SetOrgID(ctx, uuid.UUID) context.Context` and `OrgIDFromContext(ctx) (uuid.UUID, bool)`. Zero internal-package imports.
- `services/api/internal/db/orgkey/orgkey_test.go` — Three unit tests covering: `TestSetThenGet_Roundtrip` (set returns matching value with ok=true), `TestOrgIDFromContext_MissingKey` (naked ctx returns `(uuid.Nil, false)`), `TestSetOrgID_ParentNotMutated` (`context.WithValue` copy-on-write semantics preserved).

## Decisions Made

- **Three tests instead of the two the plan strictly requires.** The plan-supplied `<action>` block included `TestSetOrgID_ParentNotMutated` as the third case, and it's a cheap parallel-safe test that catches future refactor regressions of the `context.WithValue` copy-on-write contract. Both required tests (`TestSetThenGet_Roundtrip`, `TestOrgIDFromContext_MissingKey`) are present and named exactly as the verify-grep expects.
- **Followed the plan's exact code as written** — the package source matches the locked interface in `<interfaces>` and the `<action>` block character-for-character (gofmt-equivalent formatting). No deviations.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## Verification Outputs

**Build verification (per plan's `<verify>` block):**

```bash
# 1. File existence + grep contracts
test -f services/api/internal/db/orgkey/orgkey.go                # PASS
test -f services/api/internal/db/orgkey/orgkey_test.go           # PASS
grep -q '^package orgkey$'                                       # PASS
grep -q 'type orgIDKey struct{}'                                 # PASS
grep -q 'func SetOrgID(ctx context.Context, id uuid.UUID) context.Context'  # PASS
grep -q 'func OrgIDFromContext(ctx context.Context) (uuid.UUID, bool)'      # PASS
grep -q 'context.WithValue(ctx, orgIDKey{}, id)'                            # PASS
! grep -qE 'internal/(db|middleware|telemetry|server|scaffold|config)'      # PASS (no forbidden imports)
grep -q 'TestSetThenGet_Roundtrip'                                          # PASS
grep -q 'TestOrgIDFromContext_MissingKey'                                   # PASS

# 2. Go toolchain (from services/api/)
go vet ./internal/db/orgkey/...                                  # PASS (no issues)
go build ./internal/db/orgkey/...                                # PASS
go test -count=1 ./internal/db/orgkey/...                        # PASS (3 tests)
```

**Cycle-detection check:**

```bash
grep -rE 'github\.com/luongdev/open-routing/services/api/internal/(db|middleware|telemetry|server|scaffold|config)' \
  services/api/internal/db/orgkey/
# Output: (empty — confirmed zero internal-package imports)
```

**Single source of truth check:**

```bash
grep -rE 'type\s+orgIDKey\s+struct' services/api/internal/
# Output:
# services/api/internal/db/orgkey/orgkey.go:type orgIDKey struct{}
# (exactly one match — only orgkey owns the key type)
```

## Wave 2 Consumption Note

Plans 03, 04, 05 (Wave 2) consume this package as follows:

- **Import path:** `github.com/luongdev/open-routing/services/api/internal/db/orgkey`
- **Plan 03 (internal/db / orgDB):** Replace any internal `orgIDKey` definition with `orgkey.OrgIDFromContext(ctx)` for the preflight check; expect `(uuid.Nil, false)` when missing and surface a typed error.
- **Plan 04 (internal/telemetry / TracingHandler):** Use `orgkey.OrgIDFromContext(ctx)` in `Handle` to decide whether to attach `slog.String("org_id", id.String())` to the log record.
- **Plan 05 (internal/middleware / OrgContext):** After parsing and validating the `X-Org-Id` header, call `ctx = orgkey.SetOrgID(ctx, id)` and pass the derived ctx via `r.WithContext(ctx)` to `next.ServeHTTP`.
- **Plan 06 (handlers):** Handlers read the org_id from ctx via `orgkey.OrgIDFromContext` whenever they need to scope business logic.

## Next Phase Readiness

- Wave 2 plans (03, 04, 05) are unblocked: they can begin in parallel since none of them needs to wait for another to define the ctx-key.
- The forbidden-imports grep is the regression guard — any future PR that adds an `internal/*` import to this leaf will fail the cycle check.
- Plan 03 must remove the original co-located `orgIDKey` definition (per its `<import_cycle_note>`) and switch to importing this package.

---
*Phase: 01-foundation-polyglot-monorepo*
*Plan: 1a*
*Completed: 2026-05-15*
