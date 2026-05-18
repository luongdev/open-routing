---
phase: 07-web-component-embed-bundle
plan: 06
subsystem: embed
tags: [lit, web-components, shadow-dom, routing]
requires: [07-01, 07-04, 07-05]
provides: [embed-element]
affects: [host-integration]
tech_stack:
  added: [openapi-fetch]
  patterns: [shadow-dom-events, router-adapter-injection, per-element-api-client]
key_files:
  created:
    - web/apps/embed/src/embed-element.ts
  modified:
    - web/apps/embed/package.json
    - web/packages/ui/src/components/shell/catalog-shell.ts
decisions:
  - "Construct HashRouterAdapter inside embed-element and assign to shell's routerAdapter property before flipping routingMode to 'hash' (Iteration-2 BLOCKER #1 typed-interface seam)"
  - "Apply Theme CSS variables via JSON attribute with prototype-pollution guard"
  - "Added openapi-fetch as a devDependency to apps/embed to resolve ApiClient Middleware types"
metrics:
  duration: "4m"
  completed_date: "2024-05-18"
---

# Phase 07 Plan 06: OpenRoutingCatalog Custom Element Summary

Implemented the core `<open-routing-catalog>` Custom Element which wraps the UI shell in Shadow DOM and provides the typed integration seam for hash routing.

## Completed Tasks

- Implemented `OpenRoutingCatalog` LitElement with reactive properties (`orgId`, `apiBaseUrl`, `theme`, `modules`, `locale`) using TC39 2023-11 decorators.
- Built cross-Shadow CustomEvents for `request-context` and `auth-expired`.
- Injected `HashRouterAdapter` instance into `<or-catalog-shell>` without `_routes` type casts, cleanly honoring the Iteration-2 typed-interface seam.
- Instantiated per-element `createApiClient` properly forwarding URL and orgId context.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed syntax errors in catalog-shell.ts**
- **Found during:** Task 1 (pnpm typecheck)
- **Issue:** Duplicate identifiers for `Routes` import and missing `PropertyValues` import in `@open-routing/ui`'s `catalog-shell.ts`.
- **Fix:** Removed duplicate `import type { Routes } from '@lit-labs/router';` and added `type PropertyValues` to `lit` imports.
- **Files modified:** `web/packages/ui/src/components/shell/catalog-shell.ts`
- **Commit:** `156e120`

**2. [Rule 3 - Blocker] Added openapi-fetch to catalog-embed dependencies**
- **Found during:** Task 1 (pnpm typecheck)
- **Issue:** Type definitions for `openapi-fetch`'s `Middleware` were missing in `catalog-embed` workspace because it wasn't listed as a dependency, preventing clean typecheck of `embed-element.ts`.
- **Fix:** Installed `openapi-fetch@^0.13.0` as a devDependency in `apps/embed`.
- **Files modified:** `web/apps/embed/package.json`, `web/pnpm-lock.yaml`
- **Commit:** `156e120`
## Self-Check: PASSED
