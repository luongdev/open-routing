---
phase: 07-web-component-embed-bundle
plan: 02
subsystem: api-middleware
tags: [cors, security, integration, embed]
dependency_graph:
  requires: [07-01]
  provides: [CORS_MIDDLEWARE]
  affects: [server.NewMux, API_CORS_HEADERS]
tech_stack:
  added:
    - github.com/go-chi/cors@v1.2.2
  patterns:
    - go-chi middleware short-circuiting
key_files:
  created:
    - services/api/internal/middleware/cors.go
    - services/api/internal/middleware/cors_test.go
    - services/api/test/isolation/cors_preflight_test.go
    - services/api/test/isolation/cors_cross_origin_test.go
  modified:
    - services/api/internal/config/config.go
    - services/api/internal/server/server.go
    - Taskfile.yml
    - services/api/test/isolation/main_test.go
decisions_made:
  - "D7-16: Placed CORS middleware BEFORE OrgContext in the chi router to allow OPTIONS preflight requests to short-circuit without requiring an X-Org-Id header."
  - "Security: CORSAllowedOrigins relies on explicit allowlist (CORS_ALLOWED_ORIGINS env) with an empty default in prod; dev defaults to '*' via Taskfile."
metrics:
  duration: 10m
  completed_date: "2024-05-18T12:00:00Z"
---

# Phase 07 Plan 02: CORS Middleware Setup Summary

Implementation of the Go CORS middleware (D7-16) to ensure the embed bundle's cross-origin fetches receive the appropriate CORS headers.

## Key Changes
- **CORS Middleware & Parser**: Integrated `go-chi/cors` and created `AllowedOriginsFromEnv` for parsing a comma-separated list of allowed origins.
- **Routing Integration**: Inserted the `NewCORS` middleware into the chi router pipeline (`server.NewMux`) *before* the strict `OrgContext` enforcement. This guarantees that `OPTIONS` preflight checks short-circuit correctly with HTTP 200 rather than being rejected with `400 invalid_org_id`.
- **Integration Tests**: Extended the Phase 1 isolation suite with `TestCORSPreflight_*` and `TestCORSCrossOrigin_*` test cases to validate that the short-circuit behavior works as expected without regressing the project's core cross-org isolation model.
- **Configuration updates**: Added `CORSAllowedOrigins` to the `Config` struct, and modified `Taskfile.yml` to supply `CORS_ALLOWED_ORIGINS=*` during local development.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed testsupport.DoBare invocation in integration tests**
- **Found during:** Task 2 verification
- **Issue:** The `testsupport.DoBare` API has a signature requiring exactly 5 arguments, with headers strictly passed as a `map[string]string`. The initial implementation incorrectly supplied `nil` followed by a request manipulation closure (6 arguments total), failing compilation.
- **Fix:** Replaced the closure configuration with a `map[string]string` containing `Origin` and other required CORS headers.
- **Files modified:** `services/api/test/isolation/cors_cross_origin_test.go`, `services/api/test/isolation/cors_preflight_test.go`
- **Commit:** 3de8a2a

## Self-Check: PASSED
- cb242d6: feat(07-02): add CORS middleware and env parser
- 3de8a2a: feat(07-02): wire CORS into NewMux and add isolation tests