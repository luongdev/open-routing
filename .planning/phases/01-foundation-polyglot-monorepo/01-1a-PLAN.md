---
phase: 01-foundation-polyglot-monorepo
plan: 1a
type: execute
wave: 1
depends_on:
  - 01-01
files_modified:
  - services/api/internal/db/orgkey/orgkey.go
  - services/api/internal/db/orgkey/orgkey_test.go
autonomous: true
requirements:
  - FOUND-05

must_haves:
  truths:
    - "orgkey package holds the canonical context key for request-scoped org_id (FOUND-05) — both internal/db (Plan 03) and internal/middleware (Plan 05) import this leaf"
    - "orgkey.SetOrgID stores uuid.UUID (not string) per D-20"
    - "orgkey.OrgIDFromContext returns (uuid.UUID, bool) — presence flag signals missing key"
    - "orgkey has zero internal-package dependencies; only stdlib `context` + `github.com/google/uuid` — breaks the middleware ↔ db import cycle (Plan 03 §import_cycle_note)"
    - "Plans 03, 04, 05 all depend on this leaf package; extracting to Wave 1 unblocks Wave 2 parallelism"
  artifacts:
    - path: "services/api/internal/db/orgkey/orgkey.go"
      provides: "Canonical orgIDKey ctx-key + SetOrgID/OrgIDFromContext helpers"
      contains: "type orgIDKey struct{}"
    - path: "services/api/internal/db/orgkey/orgkey_test.go"
      provides: "Unit test proving Set then Get returns the same UUID; missing key returns (uuid.Nil, false)"
      contains: "TestSetThenGet_Roundtrip"
  key_links:
    - from: "services/api/internal/db/orgkey/orgkey.go SetOrgID"
      to: "context.WithValue with orgIDKey{} key"
      via: "unexported empty struct prevents ctx-key collision (S1)"
      pattern: "context\\.WithValue\\(ctx, orgIDKey\\{\\}"
    - from: "services/api/internal/db/orgkey/orgkey.go OrgIDFromContext"
      to: "ctx.Value(orgIDKey{}).(uuid.UUID)"
      via: "type-asserted retrieval with presence flag"
      pattern: "ctx\\.Value\\(orgIDKey\\{\\}\\)"
---

<objective>
Extract the canonical `org_id` context-key package into a Wave 1 leaf so Plans 03 (db / orgDB), 04 (telemetry / slog handler), and 05 (middleware / OrgContext) can all import it without creating a `db ↔ middleware` bidirectional cycle. The package contains exactly two exported helpers and one unexported key type — ~30 lines of code plus a focused unit test.

Purpose: Satisfies FOUND-05 (org_id propagates only through `context.Context`). The single source of truth for the ctx-key lives here so the wrapper validator (Plan 03), the slog tracing handler (Plan 04), and the HTTP middleware (Plan 05) all read/write the same key — no duplicate definitions, no ctx-key collisions.

Output: `services/api/internal/db/orgkey/orgkey.go` plus a unit test. `go vet ./internal/db/orgkey/...` passes; `go test ./internal/db/orgkey/...` passes. The package is importable by every Wave 2 plan via `github.com/luongdev/open-routing/services/api/internal/db/orgkey`.

This plan exists per checker B-2 to extract this leaf out of Plan 03 (which originally owned it). Moving to Wave 1 makes Wave 2 (Plans 03, 04, 05) truly parallel by removing their shared serialization point.
</objective>

<execution_context>
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/workflows/execute-plan.md
@/Users/luong/workspace/dev/open-solutions/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md
@.planning/phases/01-foundation-polyglot-monorepo/01-RESEARCH.md
@.planning/phases/01-foundation-polyglot-monorepo/01-PATTERNS.md
@.planning/phases/01-foundation-polyglot-monorepo/01-01-SUMMARY.md

<interfaces>
<!-- Locked contracts this plan creates that Wave 2 plans (03, 04, 05) import -->

Package orgkey at services/api/internal/db/orgkey/

Exports:
  // Unexported key type (S1 — empty struct prevents collisions across packages)
  type orgIDKey struct{}

  // SetOrgID returns a copy of ctx carrying the org_id.
  // Always store as uuid.UUID (not string) per D-20.
  func SetOrgID(ctx context.Context, id uuid.UUID) context.Context

  // OrgIDFromContext returns the stored org_id and a presence flag.
  // Returns (uuid.Nil, false) when ctx has no org_id (pre-middleware, bypass paths).
  func OrgIDFromContext(ctx context.Context) (uuid.UUID, bool)

Imports (allowed):
  context              (stdlib)
  github.com/google/uuid  v1.6.0  (from Plan 01's go.mod)

Imports (FORBIDDEN — would create cycles):
  github.com/luongdev/open-routing/services/api/internal/db          (Plan 03 imports orgkey, not the other way around)
  github.com/luongdev/open-routing/services/api/internal/middleware  (Plan 05 imports orgkey, not the other way around)
  Any other internal/* package — orgkey is a leaf with no internal-package dependencies
</interfaces>

<import_cycle_note>
The middleware package (Plan 05) reads/writes org_id in ctx. The db package (Plan 03) ALSO reads org_id in ctx (preflight check). If both packages owned a `SetOrgID`/`OrgIDFromContext` pair, the dependency graph would be bidirectional:
  - middleware → db (no — middleware doesn't call DB)
  - db → middleware (to read org_id) ← this would be the actual coupling

Original Research §Pattern 2 imports `"github.com/luongdev/open-routing/services/api/internal/middleware"` from inside `orgdb.go` (line 498). That works but tightly couples db to middleware and forces orgdb_test.go to import middleware too.

**Resolution (this plan owns it):** Extract the ctx-key into the tiny leaf package `internal/db/orgkey/` so BOTH `internal/db` and `internal/middleware` import `orgkey`, but neither imports the other. This is the canonical Go solution to the bidirectional ctx-key problem and keeps the dependency graph acyclic.

Moving this leaf to Wave 1 (instead of inside Plan 03) lets Plans 03, 04, 05 all start in Wave 2 without serializing on Plan 03's completion of the orgkey subdirectory.
</import_cycle_note>
</context>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: Create orgkey leaf package + unit test</name>
  <files>services/api/internal/db/orgkey/orgkey.go, services/api/internal/db/orgkey/orgkey_test.go</files>
  <read_first>
    - .planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md §D-20 (org_id in ctx is uuid.UUID, not string)
    - .planning/phases/01-foundation-polyglot-monorepo/01-PATTERNS.md §Shared Pattern S1 (unexported empty struct ctx keys, lines 910-921)
    - .planning/phases/01-foundation-polyglot-monorepo/01-RESEARCH.md §Pattern 1 §orgIDCtxKey (lines 432-446)
    - <import_cycle_note> in this plan's context block — explains why orgkey is a separate package
    - services/api/go.mod (Plan 01 Task 5) — confirms `github.com/google/uuid v1.6.0` is on the dep list
  </read_first>
  <action>
    Create the canonical home of the org_id ctx key. Two files: the package and its unit test.

    File 1 — `services/api/internal/db/orgkey/orgkey.go`. Content:
    ```go
    // Package orgkey holds the canonical context key for the request-scoped org_id.
    // Both internal/db and internal/middleware import this package to avoid a
    // bidirectional import cycle. This is a leaf package — it has zero
    // internal-package dependencies and depends only on `context` (stdlib) and
    // `github.com/google/uuid`.
    //
    // The unexported empty-struct key type (S1) prevents ctx-key collisions across
    // packages: any other package that wants to write the same ctx key would have
    // to import orgkey, which means there is exactly one canonical writer.
    package orgkey

    import (
        "context"

        "github.com/google/uuid"
    )

    // orgIDKey is the unexported context key type. Empty struct + unexported name
    // makes this package the only legal source for the key (S1).
    type orgIDKey struct{}

    // SetOrgID returns a copy of ctx carrying the org_id. Always store as
    // uuid.UUID (not string) per D-20 — gives downstream consumers type safety.
    func SetOrgID(ctx context.Context, id uuid.UUID) context.Context {
        return context.WithValue(ctx, orgIDKey{}, id)
    }

    // OrgIDFromContext returns the stored org_id and a presence flag.
    // Returns (uuid.Nil, false) when ctx has no org_id (pre-middleware code
    // paths, bypass paths like /healthz, or tests that did not call SetOrgID).
    func OrgIDFromContext(ctx context.Context) (uuid.UUID, bool) {
        id, ok := ctx.Value(orgIDKey{}).(uuid.UUID)
        return id, ok
    }
    ```

    File 2 — `services/api/internal/db/orgkey/orgkey_test.go`. Two focused tests:
    ```go
    package orgkey

    import (
        "context"
        "testing"

        "github.com/google/uuid"
    )

    // TestSetThenGet_Roundtrip proves that SetOrgID followed by OrgIDFromContext
    // returns the exact same uuid.UUID value with ok=true.
    func TestSetThenGet_Roundtrip(t *testing.T) {
        t.Parallel()
        want := uuid.Must(uuid.NewV7())
        ctx := SetOrgID(context.Background(), want)

        got, ok := OrgIDFromContext(ctx)
        if !ok {
            t.Fatalf("expected ok=true, got false")
        }
        if got != want {
            t.Fatalf("orgkey roundtrip mismatch: want=%s got=%s", want, got)
        }
    }

    // TestOrgIDFromContext_MissingKey proves that a ctx without SetOrgID
    // returns (uuid.Nil, false). This is the contract bypass paths and
    // pre-middleware code paths rely on (slog TracingHandler in Plan 04
    // uses the bool to decide whether to attach the org_id log field).
    func TestOrgIDFromContext_MissingKey(t *testing.T) {
        t.Parallel()
        got, ok := OrgIDFromContext(context.Background())
        if ok {
            t.Fatalf("expected ok=false for naked ctx, got true with id=%s", got)
        }
        if got != uuid.Nil {
            t.Fatalf("expected uuid.Nil when missing, got %s", got)
        }
    }

    // TestSetOrgID_ParentNotMutated proves SetOrgID returns a derived ctx
    // (parent is unchanged) — standard context.WithValue semantics.
    func TestSetOrgID_ParentNotMutated(t *testing.T) {
        t.Parallel()
        parent := context.Background()
        _ = SetOrgID(parent, uuid.Must(uuid.NewV7()))
        if _, ok := OrgIDFromContext(parent); ok {
            t.Fatalf("parent ctx must NOT carry org_id after SetOrgID returns a derived ctx")
        }
    }
    ```

    Anti-pattern guards:
    - Do NOT import any `internal/*` package here. The whole point of this plan is to keep `orgkey` a dependency-free leaf so Wave 2 plans can import it freely.
    - Do NOT add a `SetOrgIDString(ctx context.Context, s string) ...` helper. D-20 requires uuid.UUID at the type level; allowing string input invites the OrgContext middleware (Plan 05) to skip the `Version() >= 7` check.
    - Do NOT export `orgIDKey`. The point of the empty-struct + unexported pattern is that no other package can write the same key. Exporting it defeats the collision-prevention guarantee.

    Build verification: `cd services/api && go vet ./internal/db/orgkey/... && go test -count=1 ./internal/db/orgkey/...`. Both must pass — orgkey has zero dependencies on other Plan 01 / Plan 1a code beyond `github.com/google/uuid v1.6.0` already in go.mod (Plan 01 Task 5).
  </action>
  <verify>
    <automated>test -f services/api/internal/db/orgkey/orgkey.go &amp;&amp; test -f services/api/internal/db/orgkey/orgkey_test.go &amp;&amp; grep -q '^package orgkey$' services/api/internal/db/orgkey/orgkey.go &amp;&amp; grep -q 'type orgIDKey struct{}' services/api/internal/db/orgkey/orgkey.go &amp;&amp; grep -q 'func SetOrgID(ctx context.Context, id uuid.UUID) context.Context' services/api/internal/db/orgkey/orgkey.go &amp;&amp; grep -q 'func OrgIDFromContext(ctx context.Context) (uuid.UUID, bool)' services/api/internal/db/orgkey/orgkey.go &amp;&amp; grep -q 'context.WithValue(ctx, orgIDKey{}, id)' services/api/internal/db/orgkey/orgkey.go &amp;&amp; ! grep -qE 'github\.com/luongdev/open-routing/services/api/internal/(db|middleware|telemetry|server|scaffold|config)' services/api/internal/db/orgkey/orgkey.go &amp;&amp; grep -q 'TestSetThenGet_Roundtrip' services/api/internal/db/orgkey/orgkey_test.go &amp;&amp; grep -q 'TestOrgIDFromContext_MissingKey' services/api/internal/db/orgkey/orgkey_test.go &amp;&amp; (cd services/api &amp;&amp; go vet ./internal/db/orgkey/...) &amp;&amp; (cd services/api &amp;&amp; go test -count=1 ./internal/db/orgkey/...)</automated>
  </verify>
  <done>orgkey package compiles with zero internal-package imports; SetOrgID stores uuid.UUID via empty-struct ctx-key; OrgIDFromContext returns (uuid.UUID, bool); 3 unit tests pass</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| orgkey package ↔ all consumers | orgkey is the single canonical writer of the org_id ctx key. Any package wanting to read org_id MUST call `OrgIDFromContext`; any package wanting to write MUST call `SetOrgID`. The unexported `orgIDKey` type makes alternative implementations impossible without copying the package |
| context.Context ↔ values | ctx values are weakly typed in Go; the type-assertion `ctx.Value(orgIDKey{}).(uuid.UUID)` is the type-safety boundary that catches misuse |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-1-CTXKEY | Tampering | orgkey.go orgIDKey | mitigate | Unexported empty struct + unique package path means no other Go code can produce the same key value. A package that imports orgkey can only write via SetOrgID (which guarantees uuid.UUID type). A package that does NOT import orgkey cannot write the key at all. Shared Pattern S1 verified |
| T-1-TYPE-CONFUSION | Tampering | orgkey.go SetOrgID | mitigate | Signature accepts only `uuid.UUID` — callers cannot inject a string by mistake. D-20 enforcement is type-level here, not runtime |
| T-1-NIL-READ | Information disclosure | OrgIDFromContext | accept | When ctx has no org_id, returns (uuid.Nil, false). Consumers (Plan 04 TracingHandler, Plan 03 orgDB preflight, Plan 06 handlers) MUST check the bool. Tests cover the missing case to keep this contract honest |
</threat_model>

<verification>
After the single task completes:

```bash
cd services/api && go vet ./internal/db/orgkey/...
cd services/api && go build ./internal/db/orgkey/...
cd services/api && go test -count=1 ./internal/db/orgkey/...
```

Expected: vet clean, build clean, 3 tests pass in milliseconds (no testcontainer, no network).

Cycle check (must produce zero output):
```bash
grep -rE 'github\.com/luongdev/open-routing/services/api/internal/(db|middleware|telemetry|server|scaffold|config)' services/api/internal/db/orgkey/
```

Reverse check (Plans 03, 04, 05 should import orgkey but not duplicate the key):
```bash
# Run this AFTER Wave 2 plans complete — sanity check only.
grep -rE 'type\s+orgIDKey\s+struct' services/api/internal/ | grep -v 'internal/db/orgkey/'
# Expected: zero lines (only orgkey owns the key type).
```
</verification>

<success_criteria>
- `services/api/internal/db/orgkey/orgkey.go` exists with `package orgkey`, the unexported `orgIDKey` empty struct, and exported `SetOrgID` / `OrgIDFromContext` matching the locked interface
- The file imports only `context` (stdlib) and `github.com/google/uuid` — no internal package imports
- `services/api/internal/db/orgkey/orgkey_test.go` has 3 tests: roundtrip, missing-key, parent-not-mutated
- `go vet`, `go build`, `go test -count=1` are all green for `./internal/db/orgkey/...`
- No other file in `services/api/internal/` declares its own `orgIDKey` type (single source of truth)
- Plans 03, 04, 05 can import this package without creating any cycle — verified by Plans' own build steps in Wave 2
</success_criteria>

<output>
Create `.planning/phases/01-foundation-polyglot-monorepo/01-1a-SUMMARY.md` capturing:
- Confirmation that orgkey compiles standalone with zero internal-package imports
- The 3 test results
- Note for Wave 2 plans (03, 04, 05): they import `github.com/luongdev/open-routing/services/api/internal/db/orgkey` and consume `SetOrgID` / `OrgIDFromContext` directly — no need to define their own ctx key
- A grep result confirming no duplicate `orgIDKey` definitions exist elsewhere in the repo
</output>
