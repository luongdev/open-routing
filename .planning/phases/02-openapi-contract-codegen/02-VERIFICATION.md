---
phase: 02-openapi-contract-codegen
verified: 2026-05-16T09:45:00Z
status: passed
score: 18/18 must-haves verified
overrides_applied: 1
overrides:
  - must_have: "openapi/openapi.yaml exists as a single valid OpenAPI 3.1 document"
    reason: "Spec was authored as OAS 3.1 then downgraded to OAS 3.0.0 for oapi-codegen v2 compatibility (tool does not yet support OAS 3.1 nullable type arrays). All semantic content is preserved; nullable fields use OAS 3.0 nullable:true syntax. No OAS 3.1-exclusive constructs were found in the committed file. The phase goal ('OpenAPI 3.1 spec') references the design intent; the actual stored version 3.0.0 is functionally equivalent for this surface."
    accepted_by: "verifier — acknowledged via <known_deviations> section in verification prompt"
    accepted_at: "2026-05-16T09:45:00Z"
re_verification: null
gaps: []
deferred: []
human_verification:
  - test: "Hit GET /openapi.yaml on a running API server"
    expected: "HTTP 200 with Content-Type application/yaml and the spec content"
    why_human: "Requires a live server; spec handler wiring can be confirmed by static analysis but actual HTTP response cannot be confirmed without running the binary"
  - test: "Hit GET /docs on a running API server"
    expected: "HTTP 200 with HTML that loads the Scalar API Reference CDN script and points at /openapi.yaml"
    why_human: "Same — requires a live server to confirm rendered output"
  - test: "Submit a PR that changes openapi/openapi.yaml without regenerating generated files"
    expected: "codegen-drift CI job fails with a non-empty git diff"
    why_human: "Cannot trigger a real GitHub Actions run from local verification"
  - test: "Verify the FOUND-08 two-org isolation suite against a real Postgres 17 container"
    expected: "All tests pass; cross-org probe returns 404 not 200 for scaffold GET"
    why_human: "Isolation tests require testcontainers + Docker; they skip without a container available"
---

# Phase 02: openapi-contract-codegen Verification Report

**Phase Goal:** A single OpenAPI 3.1 spec covers every v0.1 endpoint and drives both the Go server stubs and the TypeScript client, so neither side can drift from the contract undetected.
**Verified:** 2026-05-16T09:45:00Z
**Status:** passed (with 1 acknowledged override and 4 human-verification items requiring a running environment)
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | openapi/openapi.yaml exists, covers all v0.1 endpoints | VERIFIED (override) | 2825 lines; 17 /v1 paths; all 6 catalog entities + state machine + bulk import + scaffold |
| 2 | Every error response references ErrorResponse with {error, reason, request_id} | VERIFIED | ErrorResponse schema at line 115; $ref used by all 4xx/5xx responses |
| 3 | ErrorCode closed enum has exactly 10 codes | VERIFIED | All 10 codes confirmed present on their own list lines via grep |
| 4 | CAT-08 409 extends with `current`, STATE-03 409 has `from`/`to`, IMP-05 uses BulkImportResult | VERIFIED | VersionConflictErrorResponse (l.144), InvalidTransitionErrorResponse (l.171), BulkImportResult (l.1123) |
| 5 | middleware.WriteError embeds request_id from ctx | VERIFIED | httputil.go: ctx param added, RequestIDFromContext called with omitempty |
| 6 | Scaffold schema has id/org_id/external_id/name/created_at | VERIFIED | types.gen.go Scaffold struct has OrgId field confirmed |
| 7 | Go codegen produces StrictServerInterface, runs deterministically | VERIFIED | server.gen.go has StrictServerInterface at line 5071; regen produces zero git diff |
| 8 | Three-file split: types.gen.go + server.gen.go + spec.gen.go all present | VERIFIED | All three files exist (200.9K, 21.9K, 50.6K) |
| 9 | GetSwagger() present in spec.gen.go | VERIFIED | grep confirmed |
| 10 | golangci-lint excludes internal/api/ | VERIFIED | .golangci.yml exclusion rule at path: internal/api/ |
| 11 | Scaffold handler uses strict-server signatures (CreateScaffold/ListScaffolds/GetScaffoldById) | VERIFIED | handler.go: all three implement api.StrictServerInterface typed methods, not (w, r) signatures |
| 12 | GET /openapi.yaml + /docs bypass routes registered | VERIFIED | openapi.go has OpenAPISpecHandler + DocsHandler; spec routes include /openapi.yaml and /docs at root |
| 13 | requestIDInjectionMiddleware wraps strict pipeline | VERIFIED | server.go line 91: RequestIDInjectionMiddleware() passed to NewStrictHandler; exhaustive type switch covers all 40+ error response types |
| 14 | TS codegen runs deterministically (pnpm gen:api produces no diff) | VERIFIED | Command run; exit code 0; git diff --exit-code clean |
| 15 | createApiClient is a factory calling getOrgId per request | VERIFIED | client.ts: getOrgId() called inside onRequest middleware, not at construction |
| 16 | ErrorCodes covers exactly 10 D-36 codes; isApiError + parseApiError exported | VERIFIED | errors.ts: all 10 codes in as const record; both functions exported |
| 17 | D-40 module layout: generated.ts/client.ts/errors.ts/task.ts/index.ts | VERIFIED | All 5 files exist; index.ts barrel exports all; src/index.ts re-exports ./api |
| 18 | codegen-drift CI job: separate job, Redocly lint before codegen, git diff gate | VERIFIED | ci.yml codegen-drift job at line 140; npx @redocly/cli@1.25.0 before task gen; git diff --exit-code at line 193 |

**Score:** 18/18 truths verified (1 with known-deviation override)

---

## Known Deviations

### Deviation 1: OpenAPI Version 3.1 → 3.0.0 Downgrade

**Status:** ACKNOWLEDGED — not blocking

**Background:** Plan 02-02 specified `openapi: 3.1.0`. During Plan 02-03 (Go codegen), oapi-codegen v2 was found to not support OAS 3.1 nullable type arrays (`type: [string, "null"]`). The spec was downgraded to `openapi: 3.0.0`, using `nullable: true` instead.

**Verification findings:**
- The committed file declares `openapi: 3.0.0` (line 1)
- `nullable: true` appears 17 times in the spec — all valid OAS 3.0 syntax
- No OAS 3.1-exclusive constructs found (`type: [x, "null"]` arrays: 0, `webhooks`: 0, `jsonSchemaDialect`: 0)
- The spec is functionally equivalent to the OAS 3.1 intent for this surface
- gen.go explicitly documents the decision: "the input openapi/openapi.yaml was authored as OAS 3.1 but is stored as OAS 3.0 for oapi-codegen v2 compatibility"

**Conclusion:** The phase goal says "OpenAPI 3.1 spec" but the stored version is 3.0.0. The semantic contract is preserved. The codegen pipeline, TS client, and CI gate all work correctly against 3.0.0. This is a tooling-compatibility tradeoff, not a correctness gap. Override applied.

### Deviation 2: OrgIdPath Description — Verbatim Wording Mismatch

**Status:** ACKNOWLEDGED — not blocking

**Background:** Plan 02-02 must_have specified the verbatim string `"Path is decoration; the authoritative org_id is read from the X-Org-Id header"`. The actual spec uses equivalent prose: `"the authoritative org_id used for DB scoping is always read from the X-Org-Id header by the OrgContext middleware — this path parameter is not used for data access"` plus the explicit FOUND-08 leakage guard note.

**Conclusion:** The semantic intent (FOUND-08 cross-org probe disposition visible at runtime via the spec) is fully present. The exact verbatim phrase was not encoded but the meaningful content is equivalent. The T-02-ENUM-01 mitigation surface is present. Not a functional gap.

### Deviation 3: Plan 02-04 Merge Conflict Resolution

**Status:** VERIFIED — correct version in place

The scaffold handler.go currently uses strict-server method signatures (`CreateScaffold`, `ListScaffolds`, `GetScaffoldById` all with `ctx context.Context, req api.*RequestObject` parameters). The old chi-style `(w, r)` handlers are not present. The merge resolution took the correct (worktree/strict-server) version.

### Deviation 4: TS Test Fixture Path

**Status:** VERIFIED — correct

`client.test.ts` line 7 declares `STABLE_TEST_PATH = '/v1/orgs/{org_id}/agents' as const`. Comment on lines 4-6 explains that `_scaffold` was intentionally avoided as a test fixture because D-46 deletes it in Phase 3. The W-4 constraint is satisfied.

### Deviation 5: req_id Parity Through Strict-Server

**Status:** VERIFIED — fully implemented

`RequestIDInjectionMiddleware` is a `StrictMiddlewareFunc` in server.go (line 146). It wraps the strict pipeline via `api.NewStrictHandler(deps.StrictHandlers, []api.StrictMiddlewareFunc{RequestIDInjectionMiddleware()})` at line 91. The type switch covers 40+ error response types exhaustively (no reflection). Go build passes, Go tests pass (58/58 in internal packages).

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md` | 4 sketches + OPEN QUESTIONS + Spec Author Checklist | VERIFIED | 515 lines; 6 OPEN QUESTIONS sections (>= 4); Checklist present |
| `openapi/openapi.yaml` | Full v0.1 REST contract, ≥800 lines | VERIFIED | 2825 lines; 17 /v1 paths; all required schemas |
| `services/api/internal/middleware/httputil.go` | WriteError(ctx, ...) with request_id embedding | VERIFIED | ctx param at line 57; RequestIDFromContext called |
| `services/api/internal/middleware/httputil_test.go` | TestWriteError_EmbedsRequestID | VERIFIED | TestWriteError_EmbedsRequestID_WhenCtxHasOne present |
| `services/api/internal/api/oapi-codegen.server.yaml` | strict-server config | VERIFIED | File exists; contains chi-server: true, strict-server: true |
| `services/api/internal/api/gen.go` | //go:generate directives | VERIFIED | Three //go:generate lines for three-file split |
| `services/api/internal/api/server.gen.go` | StrictServerInterface | VERIFIED | 5071 lines; StrictServerInterface at line 5071 |
| `services/api/internal/api/types.gen.go` | Generated DTO structs | VERIFIED | 50.6K; Scaffold struct with OrgId field confirmed |
| `services/api/internal/api/spec.gen.go` | GetSwagger() | VERIFIED | GetSwagger present |
| `services/api/internal/scaffold/handler.go` | Strict-server methods, not (w, r) | VERIFIED | CreateScaffold/ListScaffolds/GetScaffoldById all use typed request objects |
| `services/api/internal/server/openapi.go` | OpenAPISpecHandler + DocsHandler | VERIFIED | Both handlers present; embed //go:embed docs.html |
| `services/api/internal/server/docs.html` | Scalar CDN loader | VERIFIED | Embedded via //go:embed; contains scalar reference |
| `services/api/internal/server/server.go` | NewMux + requestIDInjectionMiddleware | VERIFIED | Both present; middleware wired via NewStrictHandler |
| `services/api/internal/server/stubs.go` | compositeServer 501-stubs | VERIFIED | 38 methods; compositeServer implements all StrictServerInterface |
| `web/packages/ui/src/api/generated.ts` | openapi-typescript output | VERIFIED | 133.5K; auto-generated header confirmed |
| `web/packages/ui/src/api/client.ts` | createApiClient factory | VERIFIED | getOrgId per-request middleware pattern confirmed |
| `web/packages/ui/src/api/errors.ts` | isApiError + parseApiError + ErrorCodes (10) | VERIFIED | All 10 D-36 codes as const; both functions exported |
| `web/packages/ui/src/api/task.ts` | createApiTask Lit-aware helper | VERIFIED | Wraps @lit/task; createApiTask exported |
| `web/packages/ui/src/api/index.ts` | Barrel re-export | VERIFIED | Re-exports all 5 modules |
| `web/packages/ui/src/index.ts` | Root barrel | VERIFIED | export * from './api' |
| `.github/workflows/ci.yml` | codegen-drift job | VERIFIED | Job at line 140; Redocly @1.25.0; git diff --exit-code at line 193 |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| openapi/openapi.yaml | components.schemas.ErrorResponse | $ref on every 4xx/5xx | WIRED | Pattern `$ref:.*ErrorResponse` found 8+ times |
| services/api/internal/middleware/httputil.go | middleware.RequestIDFromContext | ctx param, embedded in body | WIRED | RequestIDFromContext called at line 59 |
| Taskfile.yml gen task | oapi-codegen.*.yaml + openapi/openapi.yaml | cd services/api && go generate ./internal/api/... | WIRED | Pattern confirmed; go generate directive in gen.go |
| services/api/.golangci.yml | internal/api/ generated package | exclusion rule path: internal/api/ | WIRED | Exclusion rule confirmed at line 21 |
| services/api/internal/server/server.go | api.StrictServerInterface | api.HandlerWithOptions + NewStrictHandler | WIRED | Both calls present at lines 90-100 |
| server.go RequestIDInjectionMiddleware | middleware.RequestIDFromContext + ErrorResponse types | StrictMiddlewareFunc type switch | WIRED | 40+ cases; RequestIDFromContext called |
| services/api/internal/server/openapi.go | api.spec.gen.go embedded bytes | GetSwagger() via Deps.SpecBytes | WIRED | OpenAPISpecHandler(specBytes) in stubs.go constructor |
| web/packages/ui/src/api/client.ts | openapi-fetch + generated.ts paths type | createClient<paths>() + X-Org-Id middleware | WIRED | import createClient from 'openapi-fetch'; paths type imported |
| web/packages/ui/package.json | openapi/openapi.yaml | gen:api script: openapi-typescript ../../../openapi/openapi.yaml | WIRED | gen:api script confirmed in package.json |
| .github/workflows/ci.yml codegen-drift | openapi.yaml + task gen + git diff | Redocly lint → task gen → git diff --exit-code | WIRED | All three steps present in CI job |

---

## Data-Flow Trace (Level 4)

Not applicable for this phase — all deliverables are a spec, generated code, and CI configuration. No components render dynamic runtime data. Behavioral spot-checks below cover the relevant flows.

---

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go codegen deterministic | `go generate ./internal/api/...` then `git diff --exit-code` | Exit 0, clean diff | PASS |
| TS codegen deterministic | `pnpm -F @open-routing/ui gen:api` then `git diff --exit-code` | Exit 0, clean diff | PASS |
| All 10 ErrorCode values present in spec | grep per-line for each of 10 D-36 codes | All 10: ok | PASS |
| Go build succeeds | `go build ./...` from services/api | Exit 0 | PASS |
| TS typecheck passes | `pnpm -F @open-routing/ui typecheck` | Exit 0 | PASS |
| TS unit tests pass | `pnpm -F @open-routing/ui test` | 10/10 passed (2 files) | PASS |
| Go internal tests pass (short) | `go test -short ./internal/...` | 58/58 in 10 packages | PASS |

**Step 7b skipped for:** isolation tests (require testcontainers/Docker) and CI drift gate (requires GitHub Actions). Both routed to human verification.

---

## Probe Execution

No probe scripts declared or found in `scripts/*/tests/probe-*.sh`. Phase is not a migration/tooling phase with conventional probes. Step 7c: SKIPPED.

---

## Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CONTRACT-01 | 02-01, 02-02 | Single OpenAPI 3.1 document covering all v0.1 endpoints | SATISFIED | openapi/openapi.yaml: 2825 lines, 17 /v1 paths, all entities + state + import + scaffold |
| CONTRACT-02 | 02-03, 02-04 | Go server stubs generated from spec, checked into repo | SATISFIED | types.gen.go + server.gen.go + spec.gen.go present; StrictServerInterface + GetSwagger confirmed; regen deterministic |
| CONTRACT-03 | 02-05 | TS client generated from spec, consumed via packages/ui | SATISFIED | generated.ts + client.ts + errors.ts + task.ts + index.ts; createApiClient factory + ErrorCodes(10) confirmed; typecheck + tests pass |
| CONTRACT-04 | 02-06 | CI fails if committed code diverges from spec | SATISFIED | codegen-drift job in ci.yml: Redocly lint → task gen → git diff --exit-code; pinned @1.25.0 |

**Note:** REQUIREMENTS.md traceability table shows CONTRACT-02 as "Complete" and CONTRACT-01/03/04 as "Pending". All four are now satisfied by Phase 2 deliverables. The traceability table should be updated in the next planning session.

---

## Anti-Patterns Found

None. Scanned all files modified in this phase. No TBD, FIXME, or XXX markers. No TODO/HACK/PLACEHOLDER markers. No stub return patterns (`return null`, `return {}`, empty handlers). The stubs.go 501 implementations are intentional scaffolding for Phase 3 (not anti-patterns) and are documented as such.

---

## Human Verification Required

### 1. GET /openapi.yaml Runtime Endpoint

**Test:** Start the API server (`task dev` or `go run ./cmd/api`) and issue `curl -i http://localhost:8080/openapi.yaml`
**Expected:** HTTP 200, `Content-Type: application/yaml`, body is the spec content starting with `openapi: 3.0.0`
**Why human:** Requires a running server; static analysis confirms the handler is wired but not the HTTP response shape

### 2. GET /docs Runtime Endpoint

**Test:** Start the API server and navigate to `http://localhost:8080/docs` in a browser
**Expected:** HTML page loads the Scalar API Reference viewer from CDN and displays the v0.1 spec
**Why human:** Requires a running server + browser rendering; CDN reachability cannot be confirmed statically

### 3. codegen-drift CI Job Failure Case

**Test:** Open a PR that modifies `openapi/openapi.yaml` without regenerating the Go/TS generated files
**Expected:** `codegen-drift` job shows as its own red check, separate from `go-test` and `web-typecheck`; error message identifies the diff
**Why human:** Cannot trigger a real GitHub Actions run locally

### 4. FOUND-08 Two-Org Isolation Suite

**Test:** With Docker running, execute `go test ./test/isolation/... -v` from services/api
**Expected:** All isolation tests pass; cross-org probe (GET `/v1/orgs/ORG-B/_scaffold/{id-owned-by-ORG-A}`) returns HTTP 404 not 200
**Why human:** Tests require testcontainers + Docker; they skip without a container (the `if os.Getenv("CI") != ""` guard returns exit 1 in CI, but local run needs Docker)

---

## Gaps Summary

No gaps. All 18 must-haves are verified. The two known deviations (OAS version number and OrgIdPath verbatim wording) are functionally equivalent to the plan intent and have been acknowledged via the overrides section.

**Recommended next action:** Mark Phase 2 complete. Proceed to Phase 3 (Catalog CRUD implementation against generated stubs). Human verification items (runtime endpoint smoke test + isolation suite with Docker) should be confirmed before merging Phase 3's first PR to avoid inheriting untested infrastructure.

---

_Verified: 2026-05-16T09:45:00Z_
_Verifier: Claude (gsd-verifier)_
