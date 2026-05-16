---
phase: 02-openapi-contract-codegen
reviewed: 2026-05-15T15:18:14Z
depth: deep
files_reviewed: 7
files_reviewed_list:
  - openapi/openapi.yaml
  - services/api/internal/middleware/httputil.go
  - services/api/internal/middleware/httputil_test.go
  - services/api/internal/middleware/orgcontext.go
  - services/api/internal/scaffold/handler.go
  - web/package.json
  - web/pnpm-lock.yaml
findings:
  critical: 1
  warning: 1
  info: 0
  total: 2
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-05-15T15:18:14Z
**Depth:** deep
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Reviewed the Phase 02 source scope resolved from SUMMARY artifacts. The Go middleware changes are internally consistent and pass tests, and the OpenAPI file passes Redocly structural lint. The blocking issue is contract drift: the spec's error enum and auth-failure responses do not match the already-tested OrgContext runtime behavior.

## Fix Disposition

- **CR-01 fixed**: `openapi/openapi.yaml` now uses `invalid_org_id`, removes v0.1 `401 Unauthorized` org-header responses, documents org-header rejection as HTTP 400, and keeps future real auth separate.
- **WR-01 fixed**: `_scaffold/{id}` now uses `operationId: GetScaffoldById`, matching the strict-server migration plan.

Verification run during review:
- `cd services/api && go test -race -count=1 ./internal/middleware/...`: passed.
- `cd services/api && go test -race -count=1 -short ./internal/scaffold/...`: passed.
- `cd services/api && go test -short ./...`: passed.
- `cd web && pnpm install --frozen-lockfile --ignore-scripts`: passed.
- `cd web && pnpm exec redocly lint ../openapi/openapi.yaml`: passed with 5 expected warnings.

## Critical Issues

### CR-01: OpenAPI Error Contract Does Not Match OrgContext Runtime

**File:** `openapi/openapi.yaml:98`

**Issue:** The `ErrorCode` enum omits `invalid_org_id` and instead includes `missing_org_id`, while the actual locked runtime contract emits `400 {"error":"invalid_org_id"}` for missing, malformed, empty, UUIDv4, and nil `X-Org-Id` headers. The runtime behavior is enforced in `services/api/internal/middleware/orgcontext.go:53` and existing tests in `services/api/internal/middleware/orgcontext_test.go` / `services/api/internal/server/server_test.go`.

The same drift appears in the reusable `Unauthorized` response (`openapi/openapi.yaml:1411`) and protected path responses such as `CreateScaffold` (`openapi/openapi.yaml:1581`): the spec documents `401` / `missing_org_id`, but the server returns `400` / `invalid_org_id`. Generated clients will branch on a closed enum that excludes a real response code, and API consumers will handle the wrong HTTP status for org-header rejection.

**Fix:** Align the spec with the locked runtime contract unless intentionally changing runtime/tests:
- Add `invalid_org_id` to `ErrorCode`.
- Remove or stop using `missing_org_id` unless runtime is changed to emit it.
- Replace protected-route `401 Unauthorized` org-header failures with a `400` response that documents `invalid_org_id` reasons: `missing_header`, `malformed_uuid`, `uuidv7_required`.
- Keep real `401` for future JWT auth only if a separate auth response is introduced.

## Warnings

### WR-01: Scaffold GET OperationId Conflicts With Downstream Strict-Server Plan

**File:** `openapi/openapi.yaml:1617`

**Issue:** The `_scaffold/{id}` operation is named `GetScaffold`, but the Phase 02 strict-server migration plan hard-codes the generated API names as `GetScaffoldById` for handler methods, request/response object types, and the request-id injection type switch. If this spec is used as-is, oapi-codegen will generate `GetScaffold...` symbols, while the next plan expects `GetScaffoldById...` symbols. That creates avoidable compile churn or a failed acceptance grep in the scaffold migration.

**Fix:** Pick one name and make the spec and plan agree before running Go codegen. If the plan's current generated-name expectations stand, rename the operationId to `GetScaffoldById`. If `GetScaffold` is preferred for consistency with `GetAgent`, update the strict-server plan snippets and acceptance checks before execution.

---

_Reviewed: 2026-05-15T15:18:14Z_
_Reviewer: Codex inline (`gsd-code-review` workflow)_
_Depth: deep_
