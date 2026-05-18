---
phase: 07-web-component-embed-bundle
plan: 10
subsystem: admin-e2e
tags:
  - testing
  - playwright
  - regression-guard
dependency_graph:
  requires: ["07-04"]
  provides: ["D7-03-regression-guard"]
  affects: ["web/apps/admin/e2e/smoke.spec.ts"]
tech_stack:
  added: []
  patterns:
    - playwright-e2e
key_files:
  created: []
  modified:
    - web/apps/admin/e2e/smoke.spec.ts
metrics:
  duration: 120
  completed_tasks: 1
---

# Phase 07 Plan 10: Admin Smoke Test Extension Summary

Admin smoke extended with regression tests covering multiple lazy chunks (agents, skills, queues, break-reasons) with chunk-not-found console error guards.

## Work Completed

1.  **Extended Admin Smoke Test**:
    *   Added `shell navigates to a second lazy route (skills) without chunk-not-found error` to verify that a second lazy chunk loads successfully after the initial agent list chunk.
    *   Added `shell switches between 3 entity routes in sequence (lazy chunk caching)` to assert that traversing `agents` -> `queues` -> `break-reasons` does not trigger chunk-loading errors and uses cached promises correctly.
    *   Added console error filters to explicitly fail the test if Vite dynamic import errors (`Failed to fetch dynamically imported module`) are encountered.

## Deviations from Plan

None - plan executed exactly as written.

## Threat Flags

None - this was purely a test extension adding test coverage.

## Known Stubs

None introduced.