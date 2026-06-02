---
phase: 07-web-component-embed-bundle
plan: 01
subsystem: embed
tags: [setup, vite, vitest, playwright, size-limit]
dependency_graph:
  requires: []
  provides: [embed-build-pipeline, embed-test-framework]
  affects: [web/apps/embed]
tech_stack:
  added: [vite-lib, playwright, happy-dom]
  patterns: [library-mode, shadow-dom-testing, esm-sh-hosts]
key_files:
  created:
    - web/apps/embed/package.json
    - web/apps/embed/vite.config.ts
    - web/apps/embed/vitest.config.ts
    - web/apps/embed/playwright.config.ts
    - web/apps/embed/.size-limit.json
    - web/apps/embed/e2e/hosts/react/index.html
    - web/apps/embed/e2e/hosts/vue/index.html
    - web/apps/embed/e2e/hosts/html/index.html
  modified:
    - web/.gitignore
decisions:
  - id: D7-15
    title: "Embed package npm tarball shape"
    status: implemented
  - id: D7-01
    title: "Vite library mode for embed"
    status: implemented
  - id: D7-09
    title: "70KB gzipped size budget"
    status: implemented
  - id: D7-11
    title: "3-host Playwright stub matrix"
    status: implemented
metrics:
  duration: 5
  completed_date: "2025-02-23"
---

# Phase 07 Plan 01: Wave 0 build + test scaffold Summary

Set up the full build and test infrastructure for `@open-routing/catalog-embed`. 

The setup establishes Vite library mode (to bundle the workspace dependency), configures Vitest with Happy DOM for unit testing, declares a 70 KB size-limit budget, and wires Playwright with a 3-project matrix (React, Vue, HTML) using stub hosts loaded via esm.sh. Stub spec files were created to hold pending tests for embedding features.

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

All files generated correctly and all tests verified running and passing (as stubs). No threat flags added.
