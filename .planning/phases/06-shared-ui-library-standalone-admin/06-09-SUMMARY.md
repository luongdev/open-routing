---
phase: 06-shared-ui-library-standalone-admin
plan: "09"
subsystem: web-ui
tags:
  - lit
  - break-reasons
  - crud
  - routable
  - display_order
  - conflict-banner
  - single-step-wizard
dependency_graph:
  requires:
    - "06-01"
    - "06-02"
    - "06-03"
    - "06-04"
    - "06-05"
  provides:
    - or-break-reason-list
    - or-break-reason-detail
    - or-break-reason-form
    - break-reasons-barrel
    - catalog-shell-break-reason-routes
  affects:
    - "06-12"
tech_stack:
  added: []
  patterns:
    - "routable column header uses TemplateResult label (sl-tooltip) via widened OrDataTableColumn.label type"
    - "409 body consumed from error.current without re-GET (D6-03, Pitfall 9)"
    - "409 discard/review updates _entity from _conflictServer to avoid repeat 409 on re-submit"
    - "Single-step wizard: WIZARD_STEPS=[{key:'basics'}] with hideNav=true; wizard stepper hidden"
    - "display_order as sl-input type=number min=0 step=1 required; '' sentinel for empty state"
    - "routable default=true in form; bound via ?checked=${formData.routable}"
    - "Shell routes use attribute-form bindings (org-id, entity-id) without client prop (06-06 adds client)"
key_files:
  created:
    - web/packages/ui/src/components/break-reasons/break-reason-list.ts
    - web/packages/ui/src/components/break-reasons/break-reason-list.test.ts
    - web/packages/ui/src/components/break-reasons/break-reason-detail.ts
    - web/packages/ui/src/components/break-reasons/break-reason-detail.test.ts
    - web/packages/ui/src/components/break-reasons/break-reason-form.ts
    - web/packages/ui/src/components/break-reasons/break-reason-form.test.ts
  modified:
    - web/packages/ui/src/components/break-reasons/index.ts
    - web/packages/ui/src/components/shell/catalog-shell.ts
    - web/packages/ui/src/components/primitives/data-table.ts
decisions:
  - "OrDataTableColumn.label widened to string | TemplateResult to support sl-tooltip header for routable column (Codex HIGH fix)"
  - "409 conflict handler updates _entity from _conflictServer on both discard and review actions to prevent repeat 409"
  - "Shell routes use attribute-form bindings rather than property bindings since _client not yet in shell (06-06 responsibility)"
  - "display_order form state uses number | '' type to distinguish empty (invalid) from 0 (valid)"
metrics:
  duration: "28m"
  completed_date: "2026-05-18"
  tasks_completed: 2
  tests_added: 15
  files_created: 6
  files_modified: 3
---

# Phase 6 Plan 09: BreakReasons Entity (List + Detail + Form) Summary

**One-liner:** BreakReason CRUD with routable boolean column (✓/✗ + tooltip) and display_order integer, single-step wizard, 409 via error.current (D6-03), ajv validators.

## What Was Built

### or-break-reason-list (Task 1)
- `@lit/task` Task args tuple: `[orgId, search, cursor, includeDisabled, limit] as const`
- Column set per UI-SPEC §5.3 D6-V-10: code (mono), name, routable (check-lg/x-lg icons), display_order (right-aligned), enabled, updated_at (relative+tooltip)
- **Routable column header**: `TemplateResult` label containing `<sl-tooltip content="Routable: agent can still receive interactions while on break">` per D6-V-10
- **Display order column**: right-aligned via `style="text-align:right;display:block"` in render
- Empty state: "No break reasons yet" / "Define why agents go on break. Each reason shows in the break picker." per UI-SPEC §11
- Search-active empty: "No break reasons found matching '{search}'" + Clear search button
- Row click and CTA button dispatch `open-routing:navigate` to correct break-reason paths
- 6 tests: empty state, routable icons (check-lg/x-lg via column render), tooltip template label, display_order right-align, row-click navigate, CTA navigate

### or-break-reason-detail (Task 2)
- Code field: `<or-code-input .value=${br.code} .readonly=${true}>` always (D04_1-02 immutable post-create)
- Routable field: `<sl-switch>` with helper "When on, agents on this break can still receive routed interactions."
- Display order: `<sl-input type="number" min="0" step="1">` with helper "Lower values appear first in the break picker."
- **409 conflict banner**: on 409, `_conflictServer = error.current` (D6-03 + Pitfall 9: never call response.json()). `<or-conflict-banner mode="crud">` inline.
- **409 version fix**: `_handleConflictAcknowledged` updates `_entity` from `_conflictServer` for both discard and review actions — prevents repeat 409 on re-submit.
- Delete: sl-dialog with typed-name confirm (D6-V-40 pattern from agent exemplar)
- Footer metadata: version · updated (relative) · created (date)
- 5 tests: code readonly (D04_1-02), routable sl-switch + helper text, display_order number input + helper, 409 sets _conflictServer from error.current without second GET (D6-03), save disabled until dirty

### or-break-reason-form (Task 2)
- Single-step wizard: `WIZARD_STEPS=[{key:'basics', label:'Basics'}]` with `hideNav=true`; stepper shows as hidden header
- Fields per UI-SPEC §5.5 D6-V-16: code (or-code-input required), name (required), external_id (optional), routable (sl-switch, **default checked=true** per spec), display_order (sl-input type=number required), enabled (sl-switch default true)
- `validateCreateBreakReason` (ajv) called on submit; display_order empty check before ajv
- 409 duplicate_code → inline code error "This code is already in use."
- 201 → dispatches `open-routing:navigate` to `/orgs/{orgId}/break-reasons/{id}`
- 4 tests: single-step wizard, routable default=true, display_order required validation, successful POST navigate

### catalog-shell.ts (Task 2)
- Imports `or-break-reason-list.js`, `or-break-reason-detail.js`, `or-break-reason-form.js`
- Routes wired: `/orgs/:org_id/break-reasons` → `<or-break-reason-list>`, `/new` → `<or-break-reason-form>`, `/:id` → `<or-break-reason-detail>`
- Uses attribute-form bindings (`org-id=`, `entity-id=`) since shell does not yet have `_client` property (06-06 will wire the client prop)

### break-reasons/index.ts
- Exports: `OrBreakReasonList`, `OrBreakReasonDetail`, `OrBreakReasonForm`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] OrDataTableColumn.label typed as string; routable column uses TemplateResult**
- **Found during:** Codex cross-AI review (HIGH)
- **Issue:** `OrDataTableColumn.label: string` but routable header uses `html\`<sl-tooltip...>\`` (TemplateResult). TypeScript TS2322 would fail.
- **Fix:** Widened `label` to `string | TemplateResult` in data-table.ts. Lit's template engine renders both types correctly.
- **Files modified:** web/packages/ui/src/components/primitives/data-table.ts
- **Commit:** ff930c7

**2. [Rule 1 - Bug] 409 discard/review keeps stale _entity version causing repeat 409**
- **Found during:** Codex cross-AI review (HIGH)
- **Issue:** `_handleConflictAcknowledged` reset `_formData` from `_entity` but never updated `_entity` from `_conflictServer`. Re-submitting after review would PATCH with `version: oldVersion`, causing another 409.
- **Fix:** On both discard and review, update `_entity = _conflictServer` before resetting form or clearing banner.
- **Files modified:** web/packages/ui/src/components/break-reasons/break-reason-detail.ts
- **Commit:** ff930c7

**3. [Rule 1 - Bug] TS1345: void expression tested for truthiness in test**
- **Found during:** TypeScript check
- **Issue:** `expect(x).toContain('a') || expect(x).toContain('b')` — `expect().toContain()` returns void; TypeScript rejects using void in boolean context.
- **Fix:** Rewritten as `expect(x.includes('a') || x.includes('b')).toBe(true)`
- **Files modified:** web/packages/ui/src/components/break-reasons/break-reason-detail.test.ts
- **Commit:** ff930c7

**4. [Rule 3 - Blocking] Branch fast-forward needed to access phase-06 content**
- **Found during:** Plan start
- **Issue:** Worktree was at main branch tip (pre-phase-06); `gsd/phase-06-shared-ui-admin` had all Wave 1-2 content (agents, primitives, validators).
- **Fix:** `git merge --ff-only gsd/phase-06-shared-ui-admin` — fast-forward only, no merge commit.
- **Commit:** 3fe616f (pre-existing Wave 3 base tip)

## Cross-AI Peer Review

**Codex:** 3 HIGH concerns raised — all addressed in fix(06-09) commit ff930c7:
- TS2322 on TemplateResult label → widened OrDataTableColumn.label type
- 409 stale version on re-submit → _entity updated from _conflictServer in conflict handler
- TS1345 void in boolean → rewritten to boolean assertion

**Gemini:** CLI output was empty (UI rendering noise only, no review content produced)

**Verdict:** All Codex HIGH concerns addressed. READY.

## Known Stubs

The catalog-shell.ts break-reason routes do not pass a `.client` property to the components (since the shell has no `_client` property yet). This means break-reason pages will not make API calls until 06-06 adds the client wiring. This is a known limitation — the routes are wired correctly and the components are implemented; the missing client will be resolved by plan 06-06.

## Threat Surface Scan

No new surface beyond plan's threat model:
- T-06-09-01 (routable tampering): ajv validateCreateBreakReason enforces `boolean` type — MITIGATED
- T-06-09-02 (negative display_order): sl-input `min="0"` + ajv `minimum:0` constraint — MITIGATED in form; detail accepts any integer per UpdateBreakReasonRequest (server validates)
- T-06-09-03 (409 current exposes state): org-scoped, admin already has read access — ACCEPTED

## Self-Check

### Files Verified
- FOUND: web/packages/ui/src/components/break-reasons/break-reason-list.ts
- FOUND: web/packages/ui/src/components/break-reasons/break-reason-list.test.ts
- FOUND: web/packages/ui/src/components/break-reasons/break-reason-detail.ts
- FOUND: web/packages/ui/src/components/break-reasons/break-reason-detail.test.ts
- FOUND: web/packages/ui/src/components/break-reasons/break-reason-form.ts
- FOUND: web/packages/ui/src/components/break-reasons/break-reason-form.test.ts
- FOUND: web/packages/ui/src/components/break-reasons/index.ts
- FOUND: web/packages/ui/src/components/shell/catalog-shell.ts (modified)
- FOUND: web/packages/ui/src/components/primitives/data-table.ts (modified)

### Commits Verified
- 0ce0168 test(06-09): add failing tests for or-break-reason-list (RED)
- ab95700 feat(06-09): implement or-break-reason-list with routable badge + display_order
- 9b8649d test(06-09): add failing tests for or-break-reason-detail and or-break-reason-form (RED)
- cae7565 feat(06-09): implement or-break-reason-detail + or-break-reason-form + shell routes
- ff930c7 fix(06-09): address Codex peer review blockers

### Test Results
- `pnpm --filter @open-routing/ui test` → 72 tests passed, exit 0
- `pnpm --filter @open-routing/admin build` → ✓ built in 195ms, exit 0
- Break-reason tests: 6 list + 5 detail + 4 form = 15 new tests (57 pre-existing preserved)

## Self-Check: PASSED
