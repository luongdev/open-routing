---
phase: 07-web-component-embed-bundle
plan: 07-08
status: complete
started: 2026-05-18
completed: 2026-05-18
key-files:
  created:
    - .planning/phases/07-web-component-embed-bundle/07-08-SUMMARY.md
  modified:
    - web/apps/embed/src/embed-element.test.ts
    - web/apps/embed/tsconfig.json
---

# 07-08: Add Web Component Vitest Suite

**Goal:** Author unit tests for `<open-routing-catalog>` to cover EMBED-02, EMBED-05, EMBED-06, and EMBED-07 mandates.

## Execution
- Authored 21 test cases in `web/apps/embed/src/embed-element.test.ts`.
- Validated `org-id` attribute requirement and rejection of invalid UUIDv7.
- Validated theme attribute parsing and fallback.
- Validated modules attribute parsing and default route injection in the Shell.
- Validated `open-routing:request-context` emission logic.
- Validated `open-routing:auth-expired` and `409 Conflict` event interception and UI injection.
- Modified `web/apps/embed/tsconfig.json` to enable `experimentalDecorators: true` so tests can compile correctly.

## Deviations
None.