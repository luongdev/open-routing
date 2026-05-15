---
phase: 01-foundation-polyglot-monorepo
plan: 05
subsystem: api
tags: [chi, middleware, uuid-v7, otel, slog, http, multi-org, x-org-id, request-id]

requires:
  - phase: 01-foundation-polyglot-monorepo (plan 01)
    provides: services/api Go module + chi v5 + google/uuid + OTel v1.43 deps
  - phase: 01-foundation-polyglot-monorepo (plan 1a / 03)
    provides: services/api/internal/db/orgkey package (SetOrgID + OrgIDFromContext)

provides:
  - OrgContext middleware — X-Org-Id parser with UUIDv7+ validation and OTel span injection (FOUND-03)
  - RequestID middleware — per-request UUIDv7 generation with X-Request-Id response header (D-28)
  - WriteError / WriteJSON HTTP response helpers (Shared Pattern S3)
  - Locked four reject reasons emitted by OrgContext (missing_header, malformed_uuid, uuidv7_required)
  - Trust-boundary structure that is JWT-swap-ready for v1 AUTH-01 (T-1-03)

affects:
  - Plan 06 (server.go) — imports RequestID + OrgContext, wires bypass routes outside /v1
  - Phase 3 catalog handlers — import WriteError / WriteJSON for response shape
  - Phase 4 agent state — relies on OrgIDFromContext after OrgContext stored it
  - v1 AUTH-01 — replaces header parse with JWT-claim parse; downstream consumers untouched

tech-stack:
  added:
    - go.opentelemetry.io/otel/attribute (span attribute injection)
    - go.opentelemetry.io/otel/trace (SpanFromContext for active-span access)
  patterns:
    - "S3: structured JSON error response { error, reason } (locked shape)"
    - "S5: OTel span attribute injection via trace.SpanFromContext(ctx).SetAttributes"
    - "S1: unexported empty-struct ctx-key type for request_id (requestIDCtxKey)"
    - "Trust-boundary middleware shape — parse external input → validate → SetOrgID → next (JWT-swap-ready)"

key-files:
  created:
    - services/api/internal/middleware/httputil.go
    - services/api/internal/middleware/requestid.go
    - services/api/internal/middleware/requestid_test.go
    - services/api/internal/middleware/orgcontext.go
    - services/api/internal/middleware/orgcontext_test.go
  modified: []

key-decisions:
  - "Empty string X-Org-Id treated as missing_header (not malformed_uuid) — explicit edge case added to tests"
  - "uuid.Nil (Version()==0) rejected as uuidv7_required, exercising the Version() < 7 branch with a parseable input"
  - "span := trace.SpanFromContext(ctx); span.SetAttributes(...) — no nil-check; SpanFromContext returns a no-op span when no tracer is configured (works correctly under httptest)"
  - "RequestID stores uuid.UUID (not string) in ctx — type safety identical to org_id storage"

patterns-established:
  - "Pattern S3 implementation: WriteError emits {error,reason} with Content-Type: application/json — used by both middleware rejections and (Plan 06+) handler errors"
  - "Locked reject reason strings — missing_header / malformed_uuid / uuidv7_required — are stable client-facing identifiers; never consolidate into a generic bad_request"
  - "Test pattern: t.Parallel() + fresh uuid.NewV7() per test (S8) + failNextHandler helper to assert middleware short-circuit"

requirements-completed:
  - FOUND-03
  - FOUND-05

duration: 12min
completed: 2026-05-15
---

# Phase 1 Plan 05: X-Org-Id Middleware + RequestID + httputil Summary

**chi-compatible OrgContext middleware that parses X-Org-Id, rejects non-UUIDv7 input with locked error codes, stores uuid.UUID in ctx, and tags the active OTel span — plus a UUIDv7 RequestID middleware and the canonical {error,reason} response helpers.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-05-15T10:30:00Z (approx; spawn time)
- **Completed:** 2026-05-15T10:42:18Z
- **Tasks:** 3
- **Files created:** 5 (3 source, 2 test)
- **Test count:** 10 (3 RequestID + 7 OrgContext, all `t.Parallel()`)

## Accomplishments

- **OrgContext middleware** parses `X-Org-Id`, enforces `id.Version() >= 7` (D-19, D-20), stores `uuid.UUID` in ctx via `orgkey.SetOrgID`, and injects `org_id` on the active OTel span (FOUND-07).
- **RequestID middleware** generates `uuid.NewV7()` per request, stores it in ctx behind an unexported `requestIDCtxKey{}` (S1), and echoes via `X-Request-Id` response header (D-28).
- **httputil helpers** (`WriteError`, `WriteJSON`) implement Shared Pattern S3's `{error, reason}` shape — exported for Plan 06 handlers to reuse.
- **10 tests pass** without testcontainers (pure `httptest` + stub `next` handler).
- **Single canonical writer assertion** verified: `orgkey.SetOrgID` is the only `context.WithValue(ctx, orgIDKey{}, ...)` call site in the repo; `requestIDCtxKey{}` lives only in `requestid.go`.

## Task Commits

Each task was committed atomically on the worktree branch `worktree-agent-afc350f194123a4e5`:

1. **Task 1: Create httputil.go (WriteError + WriteJSON helpers)** — `d53216b` (feat)
2. **Task 2: Implement RequestID middleware (UUIDv7 generation)** — `9328268` (feat)
3. **Task 3: Implement OrgContext middleware (X-Org-Id + span attribute)** — `4fed549` (feat)

## Exported API Surface (consumed by Plan 06)

```go
package middleware

// Middleware (chi-compatible http.Handler wrappers)
func RequestID(next http.Handler) http.Handler
func OrgContext(next http.Handler) http.Handler

// Context helpers
func RequestIDFromContext(ctx context.Context) (uuid.UUID, bool)

// Response helpers (exported for handler use)
func WriteError(w http.ResponseWriter, status int, code, reason string)
func WriteJSON(w http.ResponseWriter, status int, body any)
```

The org_id ctx-helpers (`orgkey.SetOrgID` / `orgkey.OrgIDFromContext`) live in `internal/db/orgkey/` from Plan 03 — this plan only writes to that key via `orgkey.SetOrgID`.

## Locked Reject Reasons (FOUND-03 / D-20)

The four 400-rejection paths emit JSON bodies with the locked reason strings below. Clients (admin UI, future SDKs) branch on `reason`.

| Trigger | Body |
|---|---|
| Missing `X-Org-Id` header (or empty string) | `{"error":"invalid_org_id","reason":"missing_header"}` |
| `X-Org-Id` not parseable as UUID | `{"error":"invalid_org_id","reason":"malformed_uuid"}` |
| `X-Org-Id` parses but `Version() < 7` (incl. v1/v4/Nil) | `{"error":"invalid_org_id","reason":"uuidv7_required"}` |

Test coverage: `TestOrgContext_MissingHeader400`, `TestOrgContext_EmptyStringHeader400`, `TestOrgContext_MalformedHeader400`, `TestOrgContext_UUIDv4Rejected400`, `TestOrgContext_NilUUIDRejected400` — all assert status, both body fields, and that `next` is never invoked.

## Files Created/Modified

- **`services/api/internal/middleware/httputil.go`** — `WriteError`/`WriteJSON` + canonical `errorBody{Error, Reason}` struct. ~70 LoC.
- **`services/api/internal/middleware/requestid.go`** — `RequestID` middleware + `RequestIDFromContext` + unexported `requestIDCtxKey{}`. ~48 LoC.
- **`services/api/internal/middleware/requestid_test.go`** — 3 tests (UUIDv7 version assertion, per-request uniqueness, missing-ctx contract). ~80 LoC.
- **`services/api/internal/middleware/orgcontext.go`** — `OrgContext` middleware: parse → validate Version() ≥ 7 → `orgkey.SetOrgID` → `span.SetAttributes`. ~78 LoC.
- **`services/api/internal/middleware/orgcontext_test.go`** — 7 tests (5 reject paths + 2 happy paths). ~162 LoC.

## Decisions Made

- **Empty string `X-Org-Id` ≡ missing**, not malformed. `req.Header.Set("X-Org-Id", "")` produces `Get() == ""`, which trips the first branch and emits `missing_header`. Test `TestOrgContext_EmptyStringHeader400` locks this behavior to prevent later refactors from emitting `malformed_uuid` instead.
- **uuid.Nil tested as `uuidv7_required`** — Nil parses cleanly via `uuid.Parse("00000000-0000-0000-0000-000000000000")` but has `Version() == 0`, so it must trip the version branch (not the parse branch). The dedicated `TestOrgContext_NilUUIDRejected400` test proves the rejection lands on the right reason. This guards against future refactors that might short-circuit Nil before the version check.
- **`span.SetAttributes` called without nil-check** — `trace.SpanFromContext(ctx)` is documented to always return a non-nil `trace.Span`; when no tracer is configured (the httptest case), it returns a no-op span whose `SetAttributes` is a safe no-op. This avoids a defensive `if span != nil` that the plan's example contains — the plan's nil-check was overcautious; the actual contract is no-op span.
- **RequestID stores `uuid.UUID` (not string)** in ctx, mirroring the org_id ctx-storage convention. `RequestIDFromContext` returns the typed value so log handlers / Plan 06 handlers don't have to re-parse.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Defensive nil-check tightened] Removed `if span != nil` guard around `SetAttributes`**
- **Found during:** Task 3 (OrgContext implementation)
- **Issue:** The plan's example wraps `span.SetAttributes(...)` inside `if span != nil { ... }`. Per OpenTelemetry Go SDK contract, `trace.SpanFromContext` is guaranteed to return a non-nil `Span` — when no tracer is registered it returns a no-op span. The nil-check is dead code and confuses readers into thinking the call site can fail.
- **Fix:** Implemented without the nil-check. Documented the no-op-span contract in the OrgContext doc comment so future maintainers understand why no guard is needed.
- **Files modified:** `services/api/internal/middleware/orgcontext.go`
- **Verification:** All 10 tests pass, including the happy path that doesn't configure an OTel tracer — confirms the no-op span's SetAttributes does not panic.
- **Committed in:** `4fed549`

**2. [Rule 2 — Missing critical coverage] Added uuid.Nil reject test**
- **Found during:** Task 3 (writing orgcontext_test.go)
- **Issue:** Plan listed 4 reject cases (missing, malformed, v4, empty). `uuid.Nil` is a distinct edge case: it parses cleanly so `uuid.Parse` succeeds — only `Version() < 7` rejects it. Without an explicit test, a refactor that adds an `id == uuid.Nil` short-circuit before the version check could emit `malformed_uuid` instead of `uuidv7_required`, breaking the locked reason contract.
- **Fix:** Added `TestOrgContext_NilUUIDRejected400` exercising `uuid.Nil.String()` (`"00000000-0000-0000-0000-000000000000"`) and asserting `reason: "uuidv7_required"`.
- **Files modified:** `services/api/internal/middleware/orgcontext_test.go`
- **Verification:** Test passes (`go test -count=1 -run TestOrgContext_NilUUID ./internal/middleware/...`).
- **Committed in:** `4fed549`

**3. [Rule 2 — Documentation gap] Added RequestIDFromContext missing-ctx test**
- **Found during:** Task 2 (requestid_test.go)
- **Issue:** Plan specified 2 RequestID tests but did not cover the contract for callers that run OUTSIDE the middleware (bypass routes like `/healthz`, raw test code that bypasses the middleware). Without a test, a refactor could change the return from `(uuid.Nil, false)` to a generated UUID or panic, breaking callers that legitimately check `ok` first.
- **Fix:** Added `TestRequestIDFromContext_MissingReturnsFalse` asserting `(uuid.Nil, false)` on a bare ctx.
- **Files modified:** `services/api/internal/middleware/requestid_test.go`
- **Verification:** Test passes.
- **Committed in:** `9328268`

---

**Total deviations:** 3 auto-fixed (1 Rule 1 nil-check tightening, 2 Rule 2 test coverage additions)
**Impact on plan:** All three deviations harden the contract that this plan promises to Plan 06 and beyond. No scope creep — same files, no new dependencies, no architectural change. Net change vs plan: -1 dead `if span != nil` guard, +2 test cases.

## Issues Encountered

None. The plan was extremely well-specified — exact signatures, exact reject reasons, exact import paths, exact test patterns. Implementation was largely transcription with the documented deviations above.

## Verification Evidence

```text
$ cd services/api && go vet ./...
$ go build ./internal/middleware/...
$ go test -count=1 -v ./internal/middleware/...
=== RUN   TestOrgContext_MissingHeader400      --- PASS
=== RUN   TestOrgContext_MalformedHeader400    --- PASS
=== RUN   TestOrgContext_UUIDv4Rejected400     --- PASS
=== RUN   TestOrgContext_NilUUIDRejected400    --- PASS
=== RUN   TestOrgContext_EmptyStringHeader400  --- PASS
=== RUN   TestOrgContext_ValidUUIDv7_StoresInContext  --- PASS
=== RUN   TestOrgContext_FreshUUIDv7PerRequest --- PASS
=== RUN   TestRequestID_StoresUUIDv7InContextAndHeader --- PASS
=== RUN   TestRequestID_GeneratesUniqueIDsPerRequest --- PASS
=== RUN   TestRequestIDFromContext_MissingReturnsFalse --- PASS
PASS
ok    github.com/luongdev/open-routing/services/api/internal/middleware  0.201s
```

### Canonical writer audit (per plan's output spec)

```text
$ grep -rn "SetOrgID" services/api/internal/
internal/middleware/orgcontext.go:65:    ctx := orgkey.SetOrgID(r.Context(), id)
internal/db/orgkey/orgkey.go:24:func SetOrgID(ctx context.Context, id uuid.UUID) context.Context {
```

`orgkey.SetOrgID` defined once. Called from exactly one site: `OrgContext`. Single canonical writer confirmed.

```text
$ grep -rn "orgIDKey" services/api/internal/
internal/db/orgkey/orgkey.go:20:type orgIDKey struct{}
internal/db/orgkey/orgkey.go:25:    return context.WithValue(ctx, orgIDKey{}, id)
internal/db/orgkey/orgkey.go:32:    id, ok := ctx.Value(orgIDKey{}).(uuid.UUID)
```

`orgIDKey` defined and used only inside the orgkey package. No collisions.

## Next Plan Readiness (Plan 06: server.go)

Plan 06 can now wire the middleware chain. Recommended order (per PATTERNS.md S6):

```go
mux := chi.NewRouter()
mux.Use(chimw.Recoverer)            // chi stdlib
mux.Use(middleware.RequestID)       // <- this plan
// Bypass routes (D-21) — NOT inside /v1
mux.Get("/healthz", health.Live)
mux.Get("/readyz",  health.Ready(deps))
mux.Handle("/metrics", promhttp.Handler())
mux.Route("/v1", func(v1 chi.Router) {
    v1.Use(middleware.OrgContext)   // <- this plan; scoped to /v1 only
    v1.Mount("/orgs/{org_id}/_scaffold", scaffold.Routes(deps))
})
rootHandler := otelhttp.NewHandler(mux, "open-routing-api")
http.ListenAndServe(addr, rootHandler)
```

Critical ordering invariants for Plan 06:
1. `Recoverer` must run before `RequestID` so a panic in id generation (entropy exhaustion, effectively never) is caught and returns 500.
2. `OrgContext` must live inside `mux.Route("/v1", ...)`, never as a global `mux.Use`. Doing the latter would block `/healthz`, breaking D-21.
3. `otelhttp.NewHandler` must wrap the mux AFTER all `mux.Use` calls (Pitfall 5 / S6) — otherwise the span doesn't exist when `OrgContext.SetAttributes` runs.

## Self-Check: PASSED

- File: `services/api/internal/middleware/httputil.go` → FOUND
- File: `services/api/internal/middleware/requestid.go` → FOUND
- File: `services/api/internal/middleware/requestid_test.go` → FOUND
- File: `services/api/internal/middleware/orgcontext.go` → FOUND
- File: `services/api/internal/middleware/orgcontext_test.go` → FOUND
- Commit: `d53216b` (Task 1 — httputil) → FOUND
- Commit: `9328268` (Task 2 — RequestID) → FOUND
- Commit: `4fed549` (Task 3 — OrgContext) → FOUND
- `go vet ./...` → clean
- `go test -count=1 ./internal/middleware/...` → 10/10 PASS
- Canonical writer audit → 1 writer (`orgkey.SetOrgID`), 1 call site (`OrgContext`)

---
*Phase: 01-foundation-polyglot-monorepo*
*Plan: 05 — X-Org-Id middleware + RequestID + httputil*
*Completed: 2026-05-15*
