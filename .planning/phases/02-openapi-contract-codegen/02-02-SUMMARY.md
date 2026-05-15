---
phase: 02-openapi-contract-codegen
plan: "02"
subsystem: middleware + api-contract
tags: [openapi, tdd, middleware, error-envelope, request-id, catalog, agent-state, bulk-import]
dependency_graph:
  requires:
    - .planning/phases/02-openapi-contract-codegen/02-CONTEXT.md
    - .planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md
    - .planning/phases/02-openapi-contract-codegen/02-01-SUMMARY.md
    - services/api/internal/middleware/httputil.go
    - services/api/internal/middleware/requestid.go
    - .planning/REQUIREMENTS.md
  provides:
    - openapi/openapi.yaml (complete v0.1 API surface)
    - services/api/internal/middleware/httputil.go (extended WriteError)
    - services/api/internal/middleware/httputil_test.go (3 new tests)
  affects:
    - services/api/internal/middleware/orgcontext.go (3 call sites updated)
    - services/api/internal/scaffold/handler.go (9 call sites updated)
    - Phase 3 (implements against generated stubs from openapi.yaml)
    - Phase 4 (implements agent state endpoints from openapi.yaml)
    - Phase 5 (implements bulk import from openapi.yaml)
tech_stack:
  added:
    - "@redocly/cli devDependency (web/package.json) — spec lint CI"
  patterns:
    - TDD RED→GREEN for middleware signature change (D-35)
    - OAS 3.1 nullable syntax: anyOf + type 'null' (not nullable: true)
    - Closed enum ErrorCode with omitempty request_id in error envelope
    - allOf PaginatedList reuse across 6 entity list endpoints
    - anyOf for nullable $ref properties (OAS 3.1 compliant)
    - security: [] on bypass routes (no OrgHeader requirement)
key_files:
  created:
    - openapi/openapi.yaml
    - services/api/internal/middleware/httputil_test.go
  modified:
    - services/api/internal/middleware/httputil.go
    - services/api/internal/middleware/orgcontext.go
    - services/api/internal/scaffold/handler.go
    - web/package.json
    - web/pnpm-lock.yaml
decisions:
  - "WriteError signature: context.Context is first param; request_id embedded via RequestIDFromContext with omitempty"
  - "OAS 3.1 nullable: use anyOf + type 'null', not nullable: true (Redocly recommended rejects the latter)"
  - "Bypass route security: security: [] explicitly on healthz/readyz/openapi.yaml/docs operations"
  - "@redocly/cli added as web devDependency for D-48 local lint parity"
metrics:
  duration: "approx 25 minutes"
  completed: "2026-05-15"
  tasks_completed: 2
  files_created: 3
  files_modified: 5
---

# Phase 2 Plan 02: WriteError ctx extension + v0.1 OpenAPI Spec Summary

**One-liner:** TDD extension of WriteError to embed UUIDv7 request_id from ctx (D-35) plus authoring the complete 2882-line v0.1 OpenAPI 3.1 spec covering all catalog CRUD, agent state, bulk import, and infrastructure endpoints (CONTRACT-01).

## What Was Built

### Task 1 — WriteError ctx + request_id (TDD)

Extended `middleware.WriteError` to carry `context.Context` as first argument and embed the
per-request UUIDv7 from `RequestIDFromContext(ctx)` into every error response body as
`"request_id"` (D-35). The field uses `omitempty` — absent when ctx has no request id
(bypass routes, pre-middleware rejections).

Three new tests covering:
1. `TestWriteError_EmbedsRequestID_WhenCtxHasOne` — validates the happy path
2. `TestWriteError_OmitsRequestID_WhenCtxLacksOne` — validates omit behavior
3. `TestWriteJSON_Unchanged` — regression guard for WriteJSON

TDD protocol followed: RED commit (`1935cdd`) with build-failing tests, GREEN commit
(`0684dc3`) with implementation.

**Call site coverage (12 total):**
- `orgcontext.go`: 3 sites — missing `ctx` introduced a `ctx :=` declaration before the first
  rejection branch; the later `ctx := orgkey.SetOrgID(...)` was changed to `ctx =` (assignment)
  to avoid redeclaration compile error
- `scaffold/handler.go`: 9 sites — all three handlers already had `ctx := r.Context()` at the
  top, so each call site received `ctx` as the new first argument

Build verification: `go build ./...` and `go vet ./...` pass. All 71 tests in 12 packages pass.

### Task 2 — Full v0.1 OpenAPI 3.1 Spec

`openapi/openapi.yaml` — 2882 lines, single-file, Redocly recommended lint: **0 errors, 5 warnings** (all expected warnings — localhost server URL and no 4XX on bypass routes).

**Coverage:**

| Category | Items |
|----------|-------|
| Entity schemas | 7: Agent, Skill, Queue, Channel, Adapter, BreakReason, Scaffold |
| Entity list schemas | AgentListItem (flat, no skills — OQ-1A) |
| Error schemas | ErrorCode (10 values), ErrorResponse, VersionConflictErrorResponse, InvalidTransitionErrorResponse |
| D-37 special schemas | VersionConflictErrorResponse, InvalidTransitionErrorResponse, BulkImportResult |
| State schemas | AgentStatus (6 values), PostInteractionState, AgentState, PatchAgentStatusRequest |
| Import schemas | ImportEntityType, BulkImportFailedRow, BulkImportResult, ImportJob |
| Pagination | PaginatedList with allOf per-entity override |
| Parameters | OrgIdPath, EntityIdPath, CursorQuery, LimitQuery, IncludeDisabledQuery, NameSearchQuery, EntityTypeQuery, SchemaVersionQuery, ImportJobIdPath |
| Reusable responses | BadRequest, Unauthorized, NotFound, RequestEntityTooLarge, InternalServerError |
| Infrastructure paths | /healthz, /readyz, /openapi.yaml, /docs |
| Scaffold paths | POST /v1/orgs/{org_id}/_scaffold, GET /v1/orgs/{org_id}/_scaffold, GET /v1/orgs/{org_id}/_scaffold/{id} |
| Catalog CRUD paths | 5 paths × ~4 operations each = ~20 operations (agents, skills, queues, channels, adapters, break-reasons) |
| Agent state paths | GET + PATCH /v1/orgs/{org_id}/agents/{id}/status |
| Import paths | POST /v1/orgs/{org_id}/catalog/import, GET /v1/orgs/{org_id}/imports/{import_id} |

**D-36 ErrorCode enum (all 10 on own lines):**
- `invalid_body`
- `invalid_id`
- `not_found`
- `internal`
- `version_conflict`
- `cross_org`
- `missing_org_id`
- `invalid_transition`
- `import_failed`
- `rate_limited`

**D-37 special-case response shapes implemented:**
1. **CAT-08 HTTP 409 version mismatch** — `VersionConflictErrorResponse` with `current: <EntitySchema>` via `allOf` on per-entity PATCH responses
2. **STATE-03 HTTP 409 invalid transition** — `InvalidTransitionErrorResponse` with `from` and `to` AgentStatus fields
3. **IMP-05 HTTP 207 multi-status** — `BulkImportResult` (separate schema, not an error envelope) returned on POST /catalog/import at HTTP 200, 207, and 422

**FOUND-08 OrgIdPath description** — parameter description explicitly notes the path `{org_id}` is decorative; the OrgContext middleware reads the header exclusively; cross-org probe returns 404 not 403.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | WriteError ctx + request_id tests — RED gate | 1935cdd | services/api/internal/middleware/httputil_test.go |
| 1 (GREEN) | WriteError accepts ctx, embeds request_id (D-35) | 0684dc3 | services/api/internal/middleware/httputil.go, orgcontext.go, scaffold/handler.go |
| 2 | Author full v0.1 OpenAPI 3.1 spec | 6985c5c | openapi/openapi.yaml, web/package.json, web/pnpm-lock.yaml |

## TDD Gate Compliance

- RED gate commit `1935cdd`: `test(02-02): WriteError ctx + request_id field — RED`
- GREEN gate commit `0684dc3`: `feat(02-02): WriteError accepts ctx and embeds request_id (D-35)`
- No REFACTOR step needed — implementation was clean on first pass.

Both gate commits exist and are ordered correctly in git log.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed `ctx :=` redeclaration in orgcontext.go**
- **Found during:** Task 1 GREEN implementation
- **Issue:** Adding `ctx := r.Context()` before the first rejection branch created a
  compile error because the existing code later had `ctx := orgkey.SetOrgID(r.Context(), id)`.
  Go does not allow re-declaring the same variable with `:=` in the same scope.
- **Fix:** Changed the later `ctx :=` to `ctx =` (plain assignment).
- **Files modified:** `services/api/internal/middleware/orgcontext.go`
- **Commit:** 0684dc3

**2. [Rule 1 - Bug] Fixed OAS 3.1 nullable syntax**
- **Found during:** Task 2 — Redocly lint (13 errors)
- **Issue:** Used `nullable: true` (OpenAPI 3.0 syntax) throughout the spec. Redocly
  rejects `nullable` as an unrecognized property in OAS 3.1 (JSON Schema 2020-12 alignment).
- **Fix:** Replaced `nullable: true` on primitive types with `type: [string, "null"]`;
  replaced `nullable: true` next to `$ref` with `anyOf: [{$ref}, {type: "null"}]`.
- **Affected schemas:** PaginatedList.next_cursor, AgentState.wrapup_until,
  Skill.description (3 occurrences), Adapter config (2 occurrences),
  BulkImportFailedRow.field, ImportJob.errors, Channel.default_queue_id (3 occurrences),
  AgentState.engaged_channel/break_reason_id/post_interaction_state,
  PatchAgentStatusRequest.break_reason_id/post_interaction_state
- **Commit:** 6985c5c (fixed in same commit, before committing)

**3. [Rule 2 - Missing critical functionality] Added `security: []` on bypass routes**
- **Found during:** Task 2 — Redocly lint (4 errors: security-defined rule)
- **Issue:** `/healthz`, `/readyz`, `/openapi.yaml`, `/docs` operations had no `security`
  field. Redocly's recommended ruleset requires every operation to declare its security
  requirements explicitly — either inherit from global or use `security: []` for public.
- **Fix:** Added `security: []` to all 4 bypass operations.
- **Commit:** 6985c5c (fixed in same commit, before committing)

**4. [Rule 2 - Missing critical functionality] Added @redocly/cli devDependency**
- **Found during:** Task 2 — no Redocly binary available for lint verification
- **Issue:** D-48 requires `redocly lint` to run in CI; the tool was not in any package.json.
- **Fix:** `pnpm add -D @redocly/cli -w` in the web workspace root. Committed
  `web/package.json` and `web/pnpm-lock.yaml` with the OpenAPI spec commit.
- **Commit:** 6985c5c

## Threat Flags

None. This plan:
- Modified an internal error helper (middleware layer only — no new network surface)
- Added a planning artifact (openapi.yaml) — defines the surface for later phases but
  introduces no runtime code in this plan
- No new auth paths, no new file access patterns, no schema changes at trust boundaries

Consistent with the threat register dispositions for Phase 02.

## Known Stubs

None. Both deliverables are complete:
1. `WriteError` with `request_id` embedding — fully wired, tested, all call sites updated
2. `openapi/openapi.yaml` — complete spec, no placeholder paths, all schemas defined

## Self-Check: PASSED

- [x] `httputil_test.go` exists: `services/api/internal/middleware/httputil_test.go`
- [x] 3 tests present: TestWriteError_EmbedsRequestID_WhenCtxHasOne, TestWriteError_OmitsRequestID_WhenCtxLacksOne, TestWriteJSON_Unchanged
- [x] RED commit `1935cdd` exists in git log
- [x] GREEN commit `0684dc3` exists in git log
- [x] All 71 tests pass (`go test ./...`)
- [x] `go vet ./...` clean
- [x] `openapi/openapi.yaml` exists: `openapi/openapi.yaml`
- [x] Line count >= 800: 2882 lines
- [x] Opens with `openapi: 3.1.0`
- [x] All 10 ErrorCode values on own lines (grep confirmed)
- [x] All 7 entity schemas present (Agent, Skill, Queue, Channel, Adapter, BreakReason, Scaffold)
- [x] Scaffold schema has required `org_id`
- [x] D-37 shapes: VersionConflictErrorResponse (6 refs), InvalidTransitionErrorResponse (3 refs), BulkImportResult (6 refs)
- [x] FOUND-08 OrgIdPath description present
- [x] Redocly recommended lint: 0 errors, 5 warnings (all expected)
- [x] OpenAPI spec commit `6985c5c` exists in git log
