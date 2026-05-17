---
phase: 06-shared-ui-library-standalone-admin
plan: "07"
subsystem: web-ui
tags:
  - lit
  - skills
  - crud
  - simple-entity
  - conflict-banner
  - single-step-form
dependency_graph:
  requires:
    - "06-01"
    - "06-02"
    - "06-03"
    - "06-04"
    - "06-05"
  provides:
    - or-skill-list
    - or-skill-detail
    - or-skill-form
    - skills-index-barrel
  affects:
    - "06-06"
    - "06-12"
    - "06-14"
tech_stack:
  added: []
  patterns:
    - "Lit Task args tuple: [orgId, search, cursor, includeDisabled, limit] as const"
    - "409 body consumed from error.current without re-GET (D6-03, Pitfall 9)"
    - "409 discard reverts _entity + _form to error.current (server's latest state)"
    - "409 overwrite bumps _entity.version from error.current to prevent re-409"
    - ".value property binding (not value= attribute) for Shoelace programmatic resets"
    - "description: _form.description (empty-string sentinel '' clears; null === omission)"
    - "Single-step wizard (steps.length=1): or-form-wizard hides stepper auto (D6-17)"
    - "ajv cast: (validate as {(d:unknown):boolean; errors:...}) for TS compatibility"
key_files:
  created:
    - web/packages/ui/src/components/skills/skill-list.ts
    - web/packages/ui/src/components/skills/skill-list.test.ts
    - web/packages/ui/src/components/skills/skill-detail.ts
    - web/packages/ui/src/components/skills/skill-detail.test.ts
    - web/packages/ui/src/components/skills/skill-form.ts
    - web/packages/ui/src/components/skills/skill-form.test.ts
  modified:
    - web/packages/ui/src/components/skills/index.ts
decisions:
  - "catalog-shell.ts skill routes NOT wired (parallel constraint: 06-06 owns shell this wave); barrel exports complete for shell consumption"
  - "Lit .value property binding used over value= attribute for reliable Shoelace programmatic resets after 409 discard"
  - "409 conflict handler: discard reverts to error.current (server state), not stale _entity; overwrite bumps version to prevent re-409"
  - "description cleared by empty-string '' (not null) per UpdateSkillRequest D04_1-07 sentinel contract"
metrics:
  duration: "26m"
  completed_date: "2026-05-18"
  tasks_completed: 2
  tests_added: 15
  files_created: 6
  files_modified: 1
---

# Phase 6 Plan 07: Skills Entity CRUD Components Summary

**One-liner:** Skills entity UI — Task-driven list with search/pagination, single-column detail with 409 conflict banner from error.current (no re-GET, D6-03) + typed-name delete, and single-step wizard create (D6-17) with ajv validateCreateSkill.

## What Was Built

### or-skill-list (Task 1)

- `@lit/task` Task args tuple: `[orgId, search, cursor, includeDisabled, limit] as const`
- 300ms debounced name search; cursor + cursorStack reset on search/filter change (D6-V-11)
- Column set per UI-SPEC §5.3 D6-V-10: code (mono), name, skill_type (chip), enabled (check/x icon), updated_at (relative+tooltip)
- skill_type chip rendered as inline `<span>` with `--or-color-skeleton-base` background and `border-radius:4px`
- Empty states: no-search CTA "No skills yet" and search-active "No skills found matching '{_search}'"
- `open-routing:navigate` dispatch on row click (`/orgs/{orgId}/skills/{id}`) and "+ Create skill" CTA
- Shoelace per-component imports (D6-08): input, checkbox, icon-button, button, icon, alert, tooltip, spinner, dropdown, menu, menu-item
- 6 tests: empty state, 3-row render, cursor reset, row-click navigate, search empty, GET with name param

### or-skill-detail (Task 2)

- Single-column layout (Skills is the simple entity per D6-17 — no skills sub-table, no wrapup countdown)
- Code field: `<or-code-input .readonly=${true}>` always (D04_1-02 immutable post-create)
- Form fields in spec order (UI-SPEC §5.4): code (RO), name, external_id, description (textarea), skill_type, enabled
- All form inputs use `.value` property binding (not `value=` attribute) for reliable Shoelace programmatic resets
- Save disabled when `!_dirty || _saving`; enabled after any field change
- PATCH 2xx: entity refreshed from response body; Saved toast 3s auto-dismiss
- **409 conflict banner (SHOWCASE):** on 409, `_conflictServer = error.current` — NO second GET (D6-03 + Pitfall 9). `<or-conflict-banner mode="crud">` shows diff.
  - Discard: reverts `_entity` AND `_form` to `error.current` (not stale local `_entity`)
  - Review/overwrite: bumps `_entity.version` to `error.current.version` to prevent guaranteed re-409
- **Delete (D6-V-40):** sl-dialog with typed skill name confirm; `[Delete]` disabled until `_deleteConfirmName === _entity?.name`
- Cancel with dirty form: `sl-dialog` "Discard your changes?" with Keep editing / Discard buttons
- Footer metadata bar (D6-V-14): `version N · updated {relative tooltip} · created {YYYY-MM-DD}`
- Enable/Disable: PATCH `{ enabled: bool, version }` + entity refresh from response
- description sent as `_form.description` (not `null`): empty string `""` is the clear sentinel per D04_1-07
- 5 tests: code readonly, dirty tracking, 409 no re-GET (SHOWCASE), cancel dirty dialog, PATCH 2xx

### or-skill-form (Task 3 via Task 2)

- `<or-form-wizard .steps=${WIZARD_STEPS} .hideNav=${true}>` — single step `[{key:'basics', label:'Basics'}]`
- When `steps.length === 1`, wizard hides stepper per D6-17 simple entity pattern
- Header: "Create skill" (page-title CSS class)
- Form fields (UI-SPEC §5.5 D6-V-16): code (or-code-input), name (required), external_id, description (textarea), skill_type (required), enabled (default true)
- All inputs use `.value` property binding for Shoelace reactivity
- `validateCreateSkill` (ajv standalone cast) called before POST
- 409 duplicate_code → inline error on code field "This code is already in use."
- 201 → dispatches `open-routing:navigate` to `/orgs/${orgId}/skills/${newId}`
- 5xx → danger alert with Retry
- 4 tests: single-step header, ajv before POST (code pattern), 409 inline error, success navigate

### skills/index.ts (barrel)

- Exports `OrSkillList`, `OrSkillDetail`, `OrSkillForm` — pre-wired into `components/index.ts` by orchestrator

## Deviations from Plan

### Skipped (Blocked by Parallel Constraint)

**1. catalog-shell.ts skill route wiring — SKIPPED (06-06 owns shell)**
- **Found during:** Task 2 pre-execution
- **Issue:** Plan Task 2 specified updating `catalog-shell.ts` to replace skill placeholder routes with real components. However, the parallel execution constraint explicitly states: "DO NOT touch shell/catalog-shell.ts (06-06 owns that this wave)."
- **Resolution:** Skills barrel (`skills/index.ts`) exports all three components. The shell wiring will be performed by Plan 06-06 or the merge orchestrator using the barrel export.
- **Impact:** No functional impact — the barrel is complete; the shell placeholder routes remain as stubs until 06-06 wires them.

### Auto-fixed Issues (from Peer Review)

**2. [Rule 1 - Bug] Lit property binding: value= attribute vs .value property**
- **Found during:** Gemini HIGH peer review
- **Issue:** `value=${...}` binds to the HTML attribute, not the DOM property. Shoelace inputs prioritize internal state over attribute changes, so programmatic resets (e.g., after 409 discard) don't visually update the input.
- **Fix:** Replaced all `value=` with `.value=` on `sl-input`, `sl-textarea` in skill-detail.ts and skill-form.ts
- **Files modified:** skill-detail.ts, skill-form.ts
- **Commit:** 9d5d39b

**3. [Rule 1 - Bug] 409 discard handler reverted to stale _entity instead of error.current**
- **Found during:** Gemini HIGH + Codex HIGH peer review
- **Issue:** `_handleConflictAcknowledged` on discard reverted `_form` to `this._entity` (stale local state before the 409), not to `error.current` (server's latest state). This would leave user in a stale state that would 409 again.
- **Fix:** Discard now sets `_entity = error.current` and rebuilds `_form` from server data
- **Files modified:** skill-detail.ts
- **Commit:** 9d5d39b

**4. [Rule 1 - Bug] 409 "review/overwrite" did not bump entity version**
- **Found during:** Gemini HIGH + Codex HIGH peer review
- **Issue:** After choosing to review the conflict (user's edits preserved), the next PATCH reused the same stale version, guaranteeing another 409.
- **Fix:** On "review" action, bump `_entity.version` to `error.current.version` before clearing the banner
- **Files modified:** skill-detail.ts
- **Commit:** 9d5d39b

**5. [Rule 1 - Bug] description: null clears field but null === omission in UpdateSkillRequest**
- **Found during:** Gemini HIGH peer review
- **Issue:** `description: this._form.description || null` sends `null` when field is empty. Per D04_1-07 contract, `null === omission` in PATCH bodies, making it impossible to clear an existing description.
- **Fix:** Send `description: this._form.description` directly — empty string `""` is the clear sentinel
- **Files modified:** skill-detail.ts
- **Commit:** 9d5d39b

## Threat Surface Scan

No new security-relevant surface beyond plan's threat model:
- T-06-07-01 (skill_type freeform): validateCreateSkill / validateUpdateSkill client-side; server validates against allowed enum — MITIGATED
- T-06-07-02 (409 error.current in DOM): Lit html`` auto-escapes all serverValue interpolations in or-conflict-banner — MITIGATED
- T-06-07-03 (or-skill-form without valid org_id): Shell UUIDv7 route guard (Plan 06-03) rejects malformed org_ids before component renders — MITIGATED

## Known Stubs

- `catalog-shell.ts` skill routes: placeholder `html\`<div data-route="skills">...\`` — intentional per parallel execution constraint; 06-06 owns wiring.
- Empty index.ts barrels for queues, break-reasons, adapters, channels (pre-wired by orchestrator; parallel Wave 3/4 plans 06-08/09/10/11 will populate).

## Deferred Items

- TypeScript TS2306 errors from empty barrels in queues/break-reasons/adapters/channels index.ts files — pre-existing orchestrator pre-wire, owned by plans 06-08/09/10/11. Logged to deferred-items.
- MED: `duplicate_external_id` 409 error code not specially handled (Codex MED). Falls to generic alert. Acceptable for v0.1 — the field is optional and the user can correct it from the alert.

## Replicability Pattern (Wave 3 guide for 06-08/09/10/11)

To create Queues/BreakReasons/Adapters/Channels:
1. Copy skill-list.ts → entity-list.ts; replace `Skill` type, `/skills` path, column set
2. Copy skill-detail.ts → entity-detail.ts; adjust field set and form fields
3. Copy skill-form.ts → entity-form.ts; adjust field set (all use single-step D6-17 except Channel)
4. Update entity barrel (index.ts) and components/skills/index.ts → new entity dir
5. Key patterns to preserve:
   - `.value` property binding on sl-input/sl-textarea (not `value=` attribute)
   - `error.current` for 409 (not response.json())
   - `discard` → revert to server data; `overwrite` → bump version
   - `external_id: _form.external_id` (not `|| null`) — empty string sentinel clears

## Self-Check

### Files Verified

- FOUND: web/packages/ui/src/components/skills/skill-list.ts
- FOUND: web/packages/ui/src/components/skills/skill-list.test.ts
- FOUND: web/packages/ui/src/components/skills/skill-detail.ts
- FOUND: web/packages/ui/src/components/skills/skill-detail.test.ts
- FOUND: web/packages/ui/src/components/skills/skill-form.ts
- FOUND: web/packages/ui/src/components/skills/skill-form.test.ts
- FOUND: web/packages/ui/src/components/skills/index.ts

### Commits Verified

- 132e681 test(06-07): add failing tests for or-skill-list (RED)
- 7a0ef2f feat(06-07): implement or-skill-list with Task-driven API + search + pagination
- 3dec698 test(06-07): add failing tests for or-skill-detail + or-skill-form (RED)
- 271af6f feat(06-07): implement or-skill-detail and or-skill-form (single-step wizard)
- 9d5d39b fix(06-07): address Gemini peer review blockers for skills components

### Test Results

- `pnpm --filter @open-routing/ui test` → 72 tests passed (57 baseline + 15 new), exit 0
- Skills-specific: 6 list + 5 detail + 4 form = 15 new tests
- TypeScript: 0 errors in skills files (TS2306 errors are in OTHER entity empty barrels, pre-wired by orchestrator, out-of-scope)

## Self-Check: PASSED
