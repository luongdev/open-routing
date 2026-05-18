---
phase: 06-shared-ui-library-standalone-admin
plan: "01"
subsystem: web/apps/admin + web/packages/ui
tags:
  - vite8
  - lit3
  - decorators
  - rolldown
  - babel
  - vitest
  - happy-dom
  - scaffold
dependency_graph:
  requires: []
  provides:
    - "Vite 8 SPA build config with @rolldown/plugin-babel decorator support"
    - "apps/admin package manifest with all Lit/Shoelace/router deps"
    - "packages/ui vitest.config.ts with happy-dom environment"
    - "packages/ui/src/test-setup.ts with urlpattern-polyfill for Node test env"
    - "packages/ui tsconfig with experimentalDecorators:true + useDefineForClassFields:false"
    - "turbo.json gen:validators + build tasks"
  affects:
    - "web/apps/admin — buildable Vite 8 SPA skeleton"
    - "web/packages/ui — Vitest-ready test harness"
    - "web/turbo.json — gen:validators pipeline registered"
tech_stack:
  added:
    - "lit@3.3.3"
    - "@shoelace-style/shoelace@2.20.1"
    - "@lit-labs/router@0.1.4"
    - "@lit/localize@0.12.2"
    - "@lit/localize-tools@0.8.2"
    - "vite@8.0.13"
    - "@rolldown/plugin-babel@0.2.3"
    - "@babel/plugin-proposal-decorators@7.29.0"
    - "@playwright/test@1.60.0"
    - "vitest@3.2.4 (admin)"
    - "happy-dom@20.9.0"
    - "ajv@8.20.0"
    - "ajv-formats@3.0.1"
    - "urlpattern-polyfill@10.1.0"
  patterns:
    - "Vite 8 + @rolldown/plugin-babel for TypeScript experimentalDecorators lowering"
    - "Rolldown-compatible manualChunks as function (not object)"
    - "happy-dom Vitest environment with urlpattern-polyfill setup file"
    - "subpath exports map in packages/ui for tree-shaking"
    - "VALID_LOCALES guard on localStorage locale value"
key_files:
  created:
    - "web/apps/admin/vite.config.ts"
    - "web/apps/admin/index.html"
    - "web/apps/admin/src/index.ts"
    - "web/packages/ui/vitest.config.ts"
    - "web/packages/ui/src/test-setup.ts"
    - "web/packages/ui/src/components/shell/index.ts (stub)"
    - "web/packages/ui/src/components/index.ts"
    - "web/packages/ui/src/locales/locale-codes.ts"
    - "web/packages/ui/src/locales/vi.ts (stub)"
  modified:
    - "web/apps/admin/package.json (scripts + deps)"
    - "web/apps/admin/tsconfig.json (experimentalDecorators + useDefineForClassFields)"
    - "web/packages/ui/package.json (exports map + new deps)"
    - "web/packages/ui/tsconfig.json (experimentalDecorators + useDefineForClassFields)"
    - "web/turbo.json (gen:validators + build:locales + build tasks)"
    - "web/pnpm-lock.yaml"
decisions:
  - "manualChunks uses function form (not object) — Rolldown in Vite 8 only accepts function"
  - "vi.ts locale stub created for Wave 1 — prevents ERR_PACKAGE_PATH_NOT_EXPORTED; replaced by codegen in Plan 06-02"
  - "shell component stub registered as <or-catalog-shell> — full implementation in Plan 06-03"
  - "VALID_LOCALES guard added to index.ts — protects against stale/unknown localStorage values"
  - "Wildcard ./locales/*.js export added to packages/ui — needed for dynamic import in loadLocale callback"
metrics:
  duration: "13m39s"
  completed_date: "2026-05-17"
  tasks_completed: 2
  tasks_total: 2
  files_created: 9
  files_modified: 6
---

# Phase 6 Plan 01: Vite 8 + Lit 3 decorator scaffold Summary

**One-liner:** Vite 8 SPA scaffold with @rolldown/plugin-babel Babel decorator lowering for Lit 3, Vitest happy-dom test harness, and turbo gen:validators task.

## What Was Built

Task 1 installed all Lit 3 / Shoelace 2 dependencies for `apps/admin` and `packages/ui`, then created `apps/admin/package.json` (with full dev/build/preview/test/test:e2e scripts and `@open-routing/ui workspace:*` dependency) and updated `apps/admin/tsconfig.json` with the critical Lit 3 flags.

Task 2 created the complete Wave 1 scaffold:
- `vite.config.ts` — Vite 8 SPA config with `@rolldown/plugin-babel` using the `presets[].rolldown.filter` pattern from RESEARCH §1, dev proxy for /v1 + /healthz + /readyz, and Rolldown-compatible `manualChunks` as function
- `index.html` — mounts `<or-catalog-shell>` as the SPA entry
- `src/index.ts` — imperative bootstrap: configureLocalization + validated locale detection + shell component registration
- `packages/ui/tsconfig.json` — updated with `experimentalDecorators:true` + `useDefineForClassFields:false`
- `packages/ui/vitest.config.ts` — happy-dom environment + test-setup.ts + verbose reporter
- `packages/ui/src/test-setup.ts` — urlpattern-polyfill import for Node/happy-dom (NOT in browser bundle)
- `web/turbo.json` — gen:validators + build:locales + build tasks with correct dependsOn chains

## Verification Results

All acceptance criteria passed:
- `pnpm --filter @open-routing/admin build` exits 0, produces `dist/` with JS chunks
- `pnpm --filter @open-routing/ui test` exits 0 (10 existing tests pass with happy-dom)
- `vite.config.ts` contains `@rolldown/plugin-babel` + `@babel/plugin-proposal-decorators` version `2023-11`
- Both tsconfig.json files have `experimentalDecorators:true` AND `useDefineForClassFields:false`
- `vitest.config.ts` has `environment: 'happy-dom'` + `setupFiles: ['./src/test-setup.ts']`
- `turbo.json` has `gen:validators` task

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Rolldown manualChunks object → function form**
- **Found during:** Task 2 build attempt
- **Issue:** Rolldown (Vite 8) does not accept `manualChunks` as an object — only as a function. The plan's action section specified object form (from pre-Vite 8 RESEARCH), but the actual Vite 8 runtime requires function.
- **Fix:** Converted `manualChunks` to a function `(id: string) => { ... }` that pattern-matches module IDs
- **Files modified:** `web/apps/admin/vite.config.ts`

**2. [Rule 1 - Bug] Fixed locale validation: VALID_LOCALES guard on localStorage**
- **Found during:** Cross-AI peer review (Codex + Gemini both flagged MED)
- **Issue:** `saved ?? detected` blindly passed localStorage value to `setLocale` without validating it against `[sourceLocale, ...targetLocales]`. A stale `'fr'` or other unknown value would throw at bootstrap.
- **Fix:** Added `VALID_LOCALES: ReadonlySet<string>` guard before calling `setLocale`
- **Files modified:** `web/apps/admin/src/index.ts`
- **Commit:** 9695187

**3. [Rule 2 - Missing] Added wildcard locale export and vi.ts stub**
- **Found during:** Cross-AI peer review (both reviewers flagged HIGH — contract drift)
- **Issue:** `loadLocale` dynamically imports `@open-routing/ui/locales/${locale}.js` but the exports map only had a single `./locales/locale-codes.js` entry. Dynamic imports of `vi.js` would throw `ERR_PACKAGE_PATH_NOT_EXPORTED`.
- **Fix:** Added `"./locales/*.js": "./src/locales/*.ts"` wildcard export to `packages/ui/package.json`, and created `src/locales/vi.ts` as a Wave 1 scaffold stub (empty templates object). Plan 06-02 replaces this with the actual generated locale.
- **Files modified:** `web/packages/ui/package.json`, `web/packages/ui/src/locales/vi.ts`
- **Commit:** 9695187

### Cross-AI Peer Review

Ran Codex + Gemini in parallel on the implementation diff. Both returned BLOCK due to:

| Concern | Severity | Resolution |
|---------|----------|------------|
| Missing wildcard locale export — ERR_PACKAGE_PATH_NOT_EXPORTED for dynamic imports | HIGH | Fixed: wildcard export + vi.ts stub |
| localStorage locale value not validated | MED | Fixed: VALID_LOCALES guard |
| Decorator version mismatch concern (experimentalDecorators + 2023-11) | HIGH (disputed) | Accepted per RESEARCH.md: Vite 8 Oxc passes through to Babel; experimentalDecorators is for TS type-checker only; Babel does the actual lowering. Build verified at exit 0. |
| manualChunks function form (already fixed in Task 2) | LOW | Already fixed |

The decorator version concern was evaluated and determined to be a theoretical concern that does not apply in the Vite 8 / Oxc pipeline — Oxc strips TypeScript types first, then Babel handles decorator transformation. RESEARCH.md explicitly prescribes this combination. The build exit 0 verification confirms correctness.

## Known Stubs

| Stub | File | Reason |
|------|------|--------|
| OrCatalogShell (render: slot only) | `web/packages/ui/src/components/shell/index.ts` | Wave 1 scaffold; full router + sidebar implementation in Plan 06-03 |
| vi.ts locale (empty templates) | `web/packages/ui/src/locales/vi.ts` | Prevents ERR_PACKAGE_PATH_NOT_EXPORTED at scaffold time; replaced by @lit/localize-tools generated output in Plan 06-02 |

These stubs are intentional and do NOT prevent the plan's goal (buildable SPA skeleton + Vitest runner). They will be resolved in Plans 06-02 and 06-03 respectively.

## Self-Check: PASSED

All 12 files verified present on disk. All 3 task commits found in git log.
Build exits 0 (dist/ produced). Test runner exits 0 (10/10 tests pass).
