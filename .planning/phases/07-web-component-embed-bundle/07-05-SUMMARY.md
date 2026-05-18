---
phase: 07-web-component-embed-bundle
plan: 05
subsystem: web/apps/embed
tags: [embed, routing, hash, lit, adapter]
dependency_graph:
  requires: [07-01]
  provides: [07-06]
  affects: [web/apps/embed]
tech_stack:
  added: [hash-routing]
  patterns: [structural-typing]
key_files:
  created:
    - web/apps/embed/src/hash-router-adapter.ts
  modified:
    - web/apps/embed/src/hash-router-adapter.test.ts
    - web/apps/embed/package.json
decisions:
  - Added @lit-labs/router as a devDependency to satisfy type check for structural typing without importing UI package.
metrics:
  duration_minutes: 5
  tasks_completed: 2/2
  files_changed: 4
  test_coverage_added: 11 tests
---

# Phase 07 Plan 05: HashRouterAdapter Implementation Summary

Implemented the `HashRouterAdapter` for the embed custom element, which allows the embed shell to navigate via URL hashes (`#open-routing/...`) without interfering with host-level anchors. The implementation acts as a structural match for `CatalogShellRouterAdapter`, guaranteeing zero compile-time dependencies on `@open-routing/ui` to keep wave-1 execution parallel-safe.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocker] Fixed missing type dependency for @lit-labs/router**
- **Found during:** Task 1 & 2 typecheck
- **Issue:** TypeScript threw `Cannot find module '@lit-labs/router' or its corresponding type declarations` because the embed package lacked the types dependency.
- **Fix:** Added `@lit-labs/router` as a `devDependency` to `web/apps/embed/package.json` to allow structural matching.
- **Files modified:** `web/apps/embed/package.json`, `web/pnpm-lock.yaml`
- **Commit:** c710ccc

**2. [Rule 1 - Bug] Fixed test failing due to mocked state leakage**
- **Found during:** Task 2 unit tests
- **Issue:** "stop() clears the Routes reference" test failed because `adapter.start(routes)` calls `goto('/')` during initialization, which falsely triggered assertions after `stop()` was called without clearing mocks.
- **Fix:** Added `(routes.goto as ReturnType<typeof vi.fn>).mockClear();` directly after `adapter.start(routes)`.
- **Files modified:** `web/apps/embed/src/hash-router-adapter.test.ts`
- **Commit:** 11913c0

## Threat Flags
None.

## Known Stubs
None.
