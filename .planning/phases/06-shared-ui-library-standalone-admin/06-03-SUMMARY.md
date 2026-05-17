---
phase: 06-shared-ui-library-standalone-admin
plan: 03
subsystem: web/packages/ui
tags: [lit, shoelace, router, theme, org-picker, shell, tdd]
dependency_graph:
  requires:
    - "06-01: vitest/vite scaffold (deps installed; tsconfig decorator flags)"
    - "06-02: themes/index.ts token maps (plan 02 installs themes CSS; this plan creates JS maps)"
  provides:
    - "<or-catalog-shell> registered as custom element with @lit-labs/router + theme switching"
    - "<or-org-picker> registered with UUIDv7 validation + localStorage persist"
    - "web/packages/ui/src/themes/index.ts with orLight/orDark/orBrand token maps"
    - "web/packages/ui/src/components/shell/index.ts exports OrCatalogShell"
    - "web/packages/ui/src/components/primitives/index.ts exports OrOrgPicker (seeded for Plan 06-04)"
    - "web/packages/ui/vitest.config.ts happy-dom test runner"
    - "web/packages/ui/src/test-setup.ts urlpattern-polyfill"
  affects:
    - "06-04: primitives/index.ts file will be appended with additional exports"
    - "06-05..06-11: entity routes in catalog-shell will be filled with real components"
    - "Phase 7: or-catalog-shell theme=<JSON> attribute forward-compat via this.style.setProperty"
tech_stack:
  added:
    - "@lit-labs/router@0.1.4"
    - "@shoelace-style/shoelace@2.20.1"
    - "@lit/localize@0.12.2"
    - "happy-dom@20.9.0"
    - "urlpattern-polyfill@10.1.0"
    - "ajv@8.20.0"
    - "@lit/localize-tools@0.8.2"
  patterns:
    - "Lit 3 @customElement + @property + @state decorator pattern (D6-08)"
    - "Theme via this.style.setProperty on host element (D6-20) — NOT document.documentElement"
    - "@lit-labs/router Routes with enter() guard and render() per route"
    - "UUIDv7 regex client-side guard on org route entry (D6-13)"
    - "TDD: RED (failing test commit) → GREEN (implementation commit) per plan"
key_files:
  created:
    - "web/packages/ui/src/components/shell/catalog-shell.ts"
    - "web/packages/ui/src/components/shell/shell-chrome.ts"
    - "web/packages/ui/src/components/shell/catalog-shell.test.ts"
    - "web/packages/ui/src/components/shell/index.ts"
    - "web/packages/ui/src/components/primitives/org-picker.ts"
    - "web/packages/ui/src/components/primitives/org-picker.test.ts"
    - "web/packages/ui/src/components/primitives/index.ts"
    - "web/packages/ui/src/themes/index.ts"
    - "web/packages/ui/src/test-setup.ts"
    - "web/packages/ui/vitest.config.ts"
  modified:
    - "web/packages/ui/package.json (added @lit-labs/router, @shoelace-style/shoelace, happy-dom, urlpattern-polyfill, ajv, @lit/localize, @lit/localize-tools)"
    - "web/packages/ui/tsconfig.json (added experimentalDecorators:true, useDefineForClassFields:false)"
    - "web/pnpm-lock.yaml"
decisions:
  - "Removed standalone wildcard route guard (/orgs/:org_id/* with no render) — @lit-labs/router matches first-wins; a guard with no render blocks all child routes. UUIDv7 enter() moved to first concrete org route (/orgs/:org_id/agents)"
  - "org-picker uses native <input> instead of <sl-input> for direct value access in tests without Shoelace readiness wait"
  - "Themes implemented as JS Record<string, string> token maps (not CSS files) so _applyTheme() can iterate and call this.style.setProperty per D6-20"
  - "ALL_TOKEN_KEYS union exported from themes/index.ts so shell can clear all prior tokens on theme switch"
  - "localStorage wrapped in try/catch throughout (SecurityError in cross-origin Phase 7 embed contexts)"
metrics:
  duration: "~45 minutes"
  completed: "2026-05-18"
  tasks_completed: 2
  files_created: 10
  files_modified: 3
---

# Phase 6 Plan 03: Catalog Shell + Org Picker Summary

**One-liner:** OrCatalogShell with @lit-labs/router + 3-theme CSS custom property switching on host element; OrOrgPicker with UUIDv7 on-submit validation and localStorage persist.

## What Was Built

### Task 1: OrCatalogShell

- `@customElement('or-catalog-shell')` registered as a Lit 3 Web Component
- `@lit-labs/router` Routes with 22 routes across 6 entities + imports + root org-picker
- 8-entry sidebar per D6-11: Agents, Skills, Queues, Channels, Adapters, Break Reasons, divider, Bulk Import, Agent Status
- `modules` property filters sidebar entries (always shows: divider, imports, status)
- Theme switching via `_applyTheme()`: clears all prior tokens, applies new token map to `this.style` (D6-20)
- Themes: `orLight`, `orDark`, `orBrand` — all CSS custom property maps from `src/themes/index.ts`
- localStorage restore on connect + persist on theme change (both guarded in try/catch)
- Handles `open-routing:org-selected` event to navigate to /orgs/:orgId/agents
- Top bar: hamburger, wordmark, org chip with clipboard copy, theme toggle buttons, Switch org button

### Task 2: OrOrgPicker

- `@customElement('or-org-picker')` registered as a Lit 3 Web Component
- UUIDv7 validation `UUIDV7_PATTERN` fires ON SUBMIT only, not on keystroke (D6-V-09)
- Pre-fills from `localStorage['or-last-org-id']`, falls back to `lastUsedOrgId` attribute
- Dispatches `open-routing:org-selected` with `bubbles: true, composed: true`
- Shows "Last used" affordance when localStorage value differs from current input
- 480px card layout per UI-SPEC §5.2 with native `<input>` for direct value access
- localStorage operations guarded in try/catch for cross-origin embed safety

### Infrastructure

- `src/themes/index.ts`: Full token maps for `orLight`, `orDark`, `orBrand` + `ALL_TOKEN_KEYS`
- `vitest.config.ts`: happy-dom environment + `src/test-setup.ts` setup file
- `src/test-setup.ts`: `import 'urlpattern-polyfill'` for Node test environment
- All 6 packages.json entries added; tsconfig updated with `experimentalDecorators: true`

## Test Results

```
Test Files  4 passed (4)
     Tests  18 passed (18)
           ↳ 7 pre-existing API tests
           ↳ 4 catalog-shell tests (theme dark/brand, sidebar 8-entries, modules filter)
           ↳ 4 org-picker tests (event dispatch, validation error, pre-fill, localStorage)
           ↳ 3 pre-existing API client tests
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Wildcard router guard without render swallows concrete routes**
- **Found during:** Cross-AI peer review (Codex HIGH + Gemini HIGH)
- **Issue:** Plan specified `/orgs/:org_id/*` with `enter()` but no `render()`. In @lit-labs/router, routes match first-wins; a wildcard placed before concrete routes would match first and render nothing (outlet returns undefined).
- **Fix:** Removed standalone wildcard guard route. UUIDv7 `enter()` callback moved to the `/orgs/:org_id/agents` route (first concrete org route). Wave 2+ can add guards to individual routes as needed.
- **Files modified:** `catalog-shell.ts`
- **Commit:** cc1e787, 12789b1

**2. [Rule 2 - Missing critical] or-org-picker not imported/registered in shell**
- **Found during:** Cross-AI peer review (Codex HIGH)
- **Issue:** Root route renders `html\`<or-org-picker>\`` but the element was not imported, so it would render as an unknown element.
- **Fix:** Added `import '../primitives/org-picker.js'` in catalog-shell.ts
- **Files modified:** `catalog-shell.ts`
- **Commit:** 12789b1

**3. [Rule 2 - Missing critical] org-selected event not handled by shell**
- **Found during:** Cross-AI peer review (Codex HIGH)
- **Issue:** OrOrgPicker dispatches `open-routing:org-selected` but shell never listened for it or navigated.
- **Fix:** Added `addEventListener('open-routing:org-selected', this._handleOrgSelected)` in `connectedCallback`. Handler sets `this.orgId` and calls `this._routes.goto('/orgs/:orgId/agents')`.
- **Files modified:** `catalog-shell.ts`
- **Commit:** 12789b1

**4. [Rule 2 - Missing] localStorage unguarded against SecurityError**
- **Found during:** Cross-AI peer review (Codex MED + Gemini MED)
- **Issue:** Direct `localStorage.getItem/setItem` calls throw `SecurityError` in cross-origin iframe contexts (Phase 7 embed use case).
- **Fix:** Wrapped all localStorage calls in try/catch in both `catalog-shell.ts` and `org-picker.ts`
- **Files modified:** `catalog-shell.ts`, `org-picker.ts`
- **Commit:** 12789b1

**5. [Rule 1 - Bug] Comments contained 'document.documentElement' text**
- **Found during:** Plan verification check (`grep -c "document.documentElement" returns 0`)
- **Issue:** 3 comments referenced the forbidden string as a "NOT this" example, causing the grep count check to fail.
- **Fix:** Rewrote comments to not include the literal string.
- **Files modified:** `catalog-shell.ts`
- **Commit:** cc1e787

### Plan Divergences (by design)

- **org-picker uses native `<input>` instead of `<sl-input>`:** Native input allows direct `.value` access in tests without waiting for Shoelace custom element registration (`customElements.whenDefined('sl-input')`). Shoelace spinner is still used for the submitting state indicator.
- **themes/index.ts created here:** Plan 06-02 creates CSS files (`or-light.css`, etc.); this plan creates the JS token map for `_applyTheme()`. Both can coexist — CSS files are for static import; JS maps enable runtime switching.

## Cross-AI Review Results

Both Codex and Gemini ran on the diff. 4 HIGH concerns + 1 MED concern were addressed (auto-fixed per Rules 1-2). No BLOCK remains after fixes.

**Final verdict:** READY WITH FIXES → READY after applying the 4 fixes.

## Known Stubs

The shell routes for all entities render placeholder `<div data-route="...">` elements. These are intentional skeletons per the plan spec — Wave 2+ plans will replace them with real components.

- `web/packages/ui/src/components/shell/catalog-shell.ts` lines 196-310: All entity route renders are stub divs.

## Threat Flags

None. All surfaces match the plan's threat model:
- URL path org_id: client UUIDv7 regex is UX nicety; server X-Org-Id middleware is authoritative (T-06-03-01)
- localStorage org_id: same-origin only; no secret (T-06-03-03)
- Lit html`` templates: auto-escaped; no unsafeHTML usage (T-06-03-02)

## Self-Check: PASSED

All files found, all 6 task commits verified, 18/18 tests passing.
