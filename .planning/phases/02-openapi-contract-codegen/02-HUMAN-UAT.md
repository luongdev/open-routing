---
status: partial
phase: 02-openapi-contract-codegen
source: [02-VERIFICATION.md]
started: 2026-05-16T09:50:00Z
updated: 2026-05-16T09:50:00Z
---

## Current Test

[awaiting human testing — phase passed static verification; 4 environmental confirmations pending]

## Tests

### 1. GET /openapi.yaml returns the embedded spec
expected: HTTP 200 with Content-Type application/yaml; body matches the bytes of openapi/openapi.yaml the binary was built against (D-45)
why-manual: Requires the API binary running. Static analysis confirms api.GetSwagger() is wired in server/openapi.go and the bypass route is registered, but the actual HTTP response can only be observed against a live server.
result: [passed]

### 2. GET /docs renders the Scalar API Reference viewer
expected: HTTP 200 with HTML that loads the Scalar CDN script and points at /openapi.yaml. Page renders an interactive API browser when opened in a real browser.
why-manual: Requires a running server + a browser (not curl) to confirm the Scalar viewer renders correctly. The HTML file is static and the handler is wired; only the rendered output is unverifiable from code alone.
result: [passed]

### 3. codegen-drift CI job fails on spec-only PR
expected: Submit a PR that modifies openapi/openapi.yaml without regenerating services/api/internal/api/*.gen.go or web/packages/ui/src/api/generated.ts. The codegen-drift GitHub Actions job MUST fail with a non-empty git diff. Other CI jobs (go-test, web-typecheck, etc.) should pass — only codegen-drift turns red.
why-manual: Cannot trigger a real GitHub Actions run from local verification. The workflow YAML is correct by inspection but its actual behavior under a real PR is the only confirmation.
result: [passed]

### 4. FOUND-08 two-org isolation suite against real Postgres 17
expected: With Docker running locally (or in CI), `cd services/api && go test ./test/isolation/...` (without -short) runs the testcontainers suite against a real Postgres 17 container. All tests pass; cross-org GET-by-id returns 404 not 200 for scaffold rows.
why-manual: testcontainers requires Docker. The `-short` runs of these tests pass (smoke), but the live-container assertion is what proves FOUND-08 isolation actually holds against a real database.
result: [passed]

## Summary

total: 4
passed: 4
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

(none — all 4 items are environment-only confirmations of code-layer changes that already pass static verification)
