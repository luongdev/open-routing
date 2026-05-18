---
phase: 06-shared-ui-library-standalone-admin
plan: "04"
subsystem: shared-ui-primitives
tags:
  - lit3
  - shoelace
  - web-components
  - primitives
  - ci-drift-gate
  - tdd

dependency_graph:
  requires:
    - "06-03: OrCatalogShell + OrOrgPicker (base primitives pattern established)"
    - "06-02: ajv validators, themes tokens (design system tokens used by primitives)"
    - "06-01: Vite + Lit 3 + Shoelace scaffold, vitest + happy-dom test infra"
  provides:
    - "OrDataTable: reusable table for all 6 entity list pages (Wave 2+)"
    - "OrCursorPaginator: cursor-based pagination for all entity list pages"
    - "OrCodeInput: validated code field with D04_1-03 regex enforcement"
    - "OrConflictBanner: inline 409 UX for CRUD + status 409 handling"
    - "OrFormWizard: multi-step wizard for Agent/Channel create; single-step for others"
    - "CI: validators-drift gate + admin-build-smoke ensure codegen integrity"
  affects:
    - "06-05 through 06-10: All entity components import these primitives"
    - "06-11 through 06-14: Import/status components use or-data-table + or-conflict-banner"

tech_stack:
  added:
    - "or-data-table: Lit 3 LitElement, Shoelace dropdown/menu/menu-item/spinner/icon"
    - "or-cursor-paginator: Shoelace button/select/option"
    - "or-code-input: Shoelace input/icon, CODE_PATTERN /^[a-z][a-z0-9_]{0,63}$/"
    - "or-conflict-banner: Shoelace button/icon/details, JSON.stringify deep diff"
    - "or-form-wizard: Shoelace button/icon, slot-based step content"
    - "CI: validators-drift job + admin-build-smoke job in .github/workflows/ci.yml"
  patterns:
    - "TDD (RED/GREEN): failing tests committed first, implementations second"
    - "Per-component Shoelace imports (D6-08) for tree-shaking"
    - "aria-live=assertive on or-conflict-banner host (connectedCallback)"
    - "static override styles = css`` (TS4114 compliance across all Lit components)"
    - "JSON.stringify deep comparison for conflict-banner diff (avoids reference-eq false diffs)"
    - "Non-null assertion pattern (nextBtn!, prevBtn!) over unsafe casts in tests"

key_files:
  created:
    - "web/packages/ui/src/components/primitives/data-table.ts"
    - "web/packages/ui/src/components/primitives/data-table.test.ts"
    - "web/packages/ui/src/components/primitives/cursor-paginator.ts"
    - "web/packages/ui/src/components/primitives/code-input.ts"
    - "web/packages/ui/src/components/primitives/conflict-banner.ts"
    - "web/packages/ui/src/components/primitives/conflict-banner.test.ts"
    - "web/packages/ui/src/components/primitives/form-wizard.ts"
  modified:
    - "web/packages/ui/src/components/primitives/index.ts (appended 5 new exports)"
    - ".github/workflows/ci.yml (added validators-drift + admin-build-smoke jobs)"
    - "web/packages/ui/src/components/primitives/org-picker.ts (static override styles, localStorage try/catch)"
    - "web/packages/ui/src/components/shell/catalog-shell.ts (static override styles)"

decisions:
  - "CODE_PATTERN uses /^[a-z][a-z0-9_]{0,63}$/ per D04_1-03 (authoritative source); disallows hyphens"
  - "or-cursor-paginator emits cursor: null for 'next' direction — parent resolves actual next_cursor from API response; this is intentional contract (paginator doesn't own API state)"
  - "or-conflict-banner uses JSON.stringify deep comparison after Codex review flagged shallow ref-eq"
  - "or-form-wizard uses slot-based step content (slot=step-{key}) for clean separation of concerns"
  - "validators-drift CI job runs gen:api first (types must be current before validator regen)"

metrics:
  duration: "~25 minutes"
  completed: "2026-05-18"
  tasks_completed: 2
  files_created: 7
  files_modified: 4
  tests_added: 12
  tests_total: 40
  commits: 3
---

# Phase 6 Plan 04: Shared Primitive Lit Components + CI Drift Gate Summary

All 5 shared primitive components created and tested. CI extended with validator codegen drift gate and admin production build smoke. 40 tests pass (28 pre-existing + 12 new). TypeScript typecheck: 0 errors.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | or-data-table + or-cursor-paginator + or-code-input | f24d810 | data-table.ts, cursor-paginator.ts, code-input.ts, data-table.test.ts, index.ts |
| 2 | or-conflict-banner + or-form-wizard + CI drift gate | 4bb543f | conflict-banner.ts, form-wizard.ts, conflict-banner.test.ts, index.ts, ci.yml |
| fix | Peer review: static override styles, deep comparison, test types, localStorage guard | bd8c8ec | All 5 new primitives + org-picker.ts + catalog-shell.ts |

## What Was Built

### or-data-table
Reusable data table for all 6 entity list pages. Renders a `<table>` with sticky `<thead>` and `<tbody>`. Each row dispatches `or-row-click` CustomEvent with `detail.row`. Context menu `[⋮]` per row uses `sl-dropdown` + `sl-menu` with Edit/Disable/Enable/Delete actions (dispatches `or-row-action`). Loading state renders `data-testid="loading"` skeleton rows. All CSS uses `--or-color-*` design tokens.

### or-cursor-paginator
Cursor-based pagination control. Previous button disabled when `cursorStack` is empty; Next button disabled when `hasMore` is false. Limit picker (10/25/50/100) via `sl-select`. Dispatches `or-page-changed` CustomEvent with `{ cursor, direction, limit }`. The parent component manages cursor state; paginator emits the intent.

### or-code-input
Validated code field wrapping `sl-input`. Enforces `CODE_PATTERN = /^[a-z][a-z0-9_]{0,63}$/` (Phase 04.1 D04_1-03). Public `validate()` method sets `setCustomValidity` on the inner `sl-input`. Read-only mode (D04_1-02): disabled input + lock icon + "Code cannot be changed after create." message. Empty value only fails when `required=true`.

### or-conflict-banner
Inline 409 conflict resolution UI per D6-03/D6-04. CRUD mode: amber banner with per-field server-vs-user diff table (JSON.stringify deep comparison), "Review and re-submit" + "Discard my changes" buttons. Status mode: from/to state copy, "Refresh transitions" button. Dispatches `open-routing:conflict-acknowledged` with `{ action: 'review' | 'discard' }`. `aria-live="assertive"` set in `connectedCallback` for screen-reader announcement. XSS-safe: Lit html template auto-escapes all serverValue/userValue interpolations (T-06-04-01 mitigated).

### or-form-wizard
Multi-step wizard for entity create flows (D6-17). Single-step detection: `steps.length <= 1` hides the stepper visual. Multi-step: 24px step circles with connector lines; completed (filled primary + check icon), current (primary border + number), future (muted border + number). Slot-based step content: `<slot name="step-{key}">`. Dispatches `open-routing:wizard-step-changed` and `open-routing:wizard-completed`.

### CI Drift Gate (D6-31)
Two new CI jobs:
- **validators-drift**: runs `gen:api` then `gen:validators` then `git diff --exit-code` on generated files. Fails PR if committed validators diverge from the spec.
- **admin-build-smoke**: runs `pnpm --filter @open-routing/admin build`. Catches Lit decorator misconfigs and import resolution failures before deployment.

## Test Coverage

| File | Tests | What's covered |
|------|-------|----------------|
| data-table.test.ts | 3 | tbody with one row, loading skeleton, row-click event |
| data-table.test.ts | 2 | cursor-paginator: next disabled when hasMore=false, prev emits direction=prev |
| data-table.test.ts | 2 | code-input: validate() returns true/false for valid/invalid values |
| conflict-banner.test.ts | 5 | crud banner renders, diff shows changed field, status from/to, review event, discard event |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Missing `override` on `static styles` in all Lit components**
- **Found during:** Codex peer review (typecheck failure TS4114)
- **Issue:** TypeScript `strict` mode requires `override` keyword on inherited class members. `static styles = css\`\`` overrides `LitElement.styles` but was missing the `override` modifier.
- **Fix:** Added `override` to `static styles` in all 5 new primitives + org-picker.ts (Phase 03) + catalog-shell.ts (Phase 03)
- **Files modified:** data-table.ts, cursor-paginator.ts, code-input.ts, conflict-banner.ts, form-wizard.ts, org-picker.ts, catalog-shell.ts
- **Commit:** bd8c8ec

**2. [Rule 1 - Bug] TypeScript null-cast errors in test files**
- **Found during:** Codex peer review (typecheck failure TS2352)
- **Issue:** `(btn as HTMLButtonElement)` when `btn` is `Element | null` — TypeScript correctly rejects this conversion because neither overlaps with null.
- **Fix:** Changed `let nextBtn: Element | null` to `let nextBtn: HTMLElement | null`, cast in the forEach with `btn as HTMLElement`, and used non-null assertion `nextBtn!.click()` after `expect(nextBtn).toBeTruthy()` guard.
- **Files modified:** data-table.test.ts, conflict-banner.test.ts
- **Commit:** bd8c8ec

**3. [Rule 1 - Bug] Shallow reference comparison in conflict-banner diff**
- **Found during:** Codex + Gemini peer review (MED concern)
- **Issue:** `serverValue[key] !== userValue[key]` uses JavaScript reference inequality, which gives false positives for nested objects/arrays (e.g., adapter `config` field) even when they're structurally identical.
- **Fix:** Changed to `JSON.stringify(this.serverValue[key]) !== JSON.stringify(this.userValue[key])` for deterministic deep comparison of primitive entity fields.
- **Files modified:** conflict-banner.ts
- **Commit:** bd8c8ec

**4. [Rule 2 - Missing functionality] OrOrgPicker.updated() missing localStorage try/catch**
- **Found during:** Codex peer review (MED concern)
- **Issue:** `firstUpdated()` wraps localStorage in try/catch but `updated()` did not, creating inconsistency. In cross-origin embed contexts `localStorage` can throw SecurityError on any access.
- **Fix:** Wrapped `localStorage.getItem(LS_KEY)` in `updated()` with try/catch consistent with `firstUpdated()`.
- **Files modified:** org-picker.ts
- **Commit:** bd8c8ec

**5. [Rule 2 - Missing functionality] OrCodeInput.validate() fails empty value when required=false**
- **Found during:** Codex peer review (LOW concern)
- **Issue:** `CODE_PATTERN.test('')` returns false, so calling `validate()` on an empty field would flag an error even when `required=false`.
- **Fix:** Added early return for `!this.value && !this.required` — resets to `_validationState = null` (untouched) instead of setting invalid.
- **Files modified:** code-input.ts
- **Commit:** bd8c8ec

**6. [Rule 1 - Bug] Redundant ternary in form-wizard**
- **Found during:** Gemini peer review (LOW concern)
- **Issue:** `const finalLabel = this.steps.length > 0 ? 'Create' : 'Create'` — both branches identical.
- **Fix:** Simplified to `const finalLabel = 'Create'`.
- **Files modified:** form-wizard.ts
- **Commit:** bd8c8ec

### Non-Issues Raised (Gemini hallucination)

Gemini flagged HIGH concern about `@state()` being corrupted to `@.planning/STATE.md()`. This was false — the actual code has proper `@state()` decorators. The diff Gemini reviewed included the gsd/phase-06 merge commit with Phase 03 files, and Gemini appears to have hallucinated the corruption. No action required.

### Within-Scope Pre-existing Fixes

`org-picker.ts` and `catalog-shell.ts` (Phase 03 files) had the same `static styles` TS4114 error. These are in scope because they were blocking `pnpm typecheck` which is a CI gate for all primitives. Fixed in the same commit as the new primitives.

## Self-Check

### Files Created Exist
- [x] data-table.ts
- [x] cursor-paginator.ts
- [x] code-input.ts
- [x] conflict-banner.ts
- [x] form-wizard.ts
- [x] data-table.test.ts
- [x] conflict-banner.test.ts

### Commits Exist
- [x] f24d810 (Task 1: or-data-table + or-cursor-paginator + or-code-input)
- [x] 4bb543f (Task 2: or-conflict-banner + or-form-wizard + CI drift gate)
- [x] bd8c8ec (fix: peer review blockers)

### Forbidden Files Untouched
- [x] .planning/STATE.md — not modified
- [x] .planning/ROADMAP.md — not modified
- [x] web/packages/ui/src/themes/index.ts — not modified
- [x] web/packages/ui/src/components/shell/index.ts — not modified
- [x] .planning/phases/06-shared-ui-library-standalone-admin/06-VALIDATION.md — not modified

## Self-Check: PASSED
