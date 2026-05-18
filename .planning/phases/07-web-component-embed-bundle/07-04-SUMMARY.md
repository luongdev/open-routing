---
phase: 07-web-component-embed-bundle
plan: 07-04
status: complete
started: 2026-05-18
completed: 2026-05-18
key-files:
  created:
    - .planning/phases/07-web-component-embed-bundle/07-04-SUMMARY.md
  modified:
    - web/packages/ui/src/components/shell/catalog-shell.ts
    - web/packages/ui/src/components/shell/catalog-shell.test.ts
    - web/packages/ui/src/components/shell/index.ts
---

# 07-04: Refactor catalog-shell for lazy routing and Hash Adapter Seam

**Goal:** Remove static entity imports from `catalog-shell.ts` and replace them with dynamic imports via a `_composedEnter` helper. Add `routingMode` and a typed `routerAdapter` injection seam.

## Execution
- Converted all 20+ static entity route definitions to use `lazyRoute` logic (`_composedEnter`).
- Exported `CatalogShellRouterAdapter` interface and re-exported it in `index.ts`.
- Added `routingMode: 'history' | 'hash'` and `routerAdapter` properties to the shell.
- Implemented lifecycle methods (`firstUpdated`, `disconnectedCallback`) to start/stop the adapter, handing it the internal Routes object.
- Warn when `routingMode` is mutated after mount.
- Extended unit tests to cover routing properties and lifecycle methods.

## Deviations
None.