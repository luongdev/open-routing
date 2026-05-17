---
phase: 06-shared-ui-library-standalone-admin
plan: "05"
subsystem: web-ui
tags:
  - lit
  - agents
  - crud
  - wizard
  - conflict-banner
  - skills
  - wrapup
dependency_graph:
  requires:
    - "06-01"
    - "06-02"
    - "06-03"
    - "06-04"
  provides:
    - or-agent-list
    - or-agent-detail
    - or-agent-form
    - agents-index-barrel
    - components-index-wave2
  affects:
    - "06-06"
    - "06-07"
    - "06-08"
    - "06-09"
    - "06-10"
    - "06-11"
tech_stack:
  added:
    - "@lit/task — async state machine for agent-list API fetch"
    - "ajv standalone cast pattern for .errors property access"
    - "Web Animations API polyfills in test-setup (getAnimations + animate)"
  patterns:
    - "Lit Task args tuple: [orgId, search, cursor, includeDisabled, limit] as const"
    - "409 body consumed from error.current without re-GET (D6-03, Pitfall 9)"
    - "Typed-name delete confirm: _deleteConfirmName === entity.name (D6-V-40)"
    - "or-form-wizard.hideNav=true to suppress built-in nav when parent validates"
    - "ajv cast: (validate as {(d:unknown):boolean; errors:...}) for TS compatibility"
    - "external_id sent as '' (not null) to clear binding per D04_1-07"
key_files:
  created:
    - web/packages/ui/src/components/agents/agent-list.ts
    - web/packages/ui/src/components/agents/agent-list.test.ts
    - web/packages/ui/src/components/agents/agent-detail.ts
    - web/packages/ui/src/components/agents/agent-detail.test.ts
    - web/packages/ui/src/components/agents/agent-form.ts
    - web/packages/ui/src/components/agents/agent-form.test.ts
  modified:
    - web/packages/ui/src/components/agents/index.ts
    - web/packages/ui/src/components/index.ts
    - web/packages/ui/src/components/primitives/form-wizard.ts
    - web/packages/ui/src/test-setup.ts
decisions:
  - "ajv standalone validator .errors accessed via cast to avoid TS2339 (generators use @ts-nocheck, consumers cannot)"
  - "or-form-wizard gained hideNav property so agent-form can own its nav without duplicate buttons"
  - "external_id PATCH body uses '' sentinel (not null) per UpdateAgentRequest D04_1-07 contract"
  - "test-setup.ts gains Element.getAnimations + animate polyfills for Shoelace sl-dialog in happy-dom"
  - "Step 1 Next validates email format inline to prevent advancing with bad email (Gemini HIGH fix)"
metrics:
  duration: "19m"
  completed_date: "2026-05-18"
  tasks_completed: 4
  tests_added: 17
  files_created: 6
  files_modified: 4
---

# Phase 6 Plan 05: Agents Exemplar Entity Summary

**One-liner:** Complete Agents entity UI — Task-driven list with search/pagination, 2-column detail with 409 conflict banner from error.current + typed-name delete (D6-V-40) + skills sub-table + wrapup countdown, and 3-step wizard create with per-step validation.

## What Was Built

### or-agent-list (Task 1)
- `@lit/task` Task args tuple: `[orgId, search, cursor, includeDisabled, limit] as const`
- 300ms debounced name search; cursor + cursorStack reset on search/filter change (D6-V-11)
- Column set: code (mono render), name, email, enabled (check/x icon), updated_at (relative+tooltip)
- Empty states: no-search CTA and search-active with "Clear search" button
- `open-routing:navigate` dispatch on row click and "+ Create agent" button
- 4 tests: resolves rows, pending loading, row-click navigate, search query param

### or-agent-detail (Tasks 2a + 2b)
- 2-column layout: 640px form + flex-1 skills at ≥1280px; stacks vertically on mobile
- Code field: `<or-code-input .readonly=${true}>` always (D04_1-02 immutable post-create)
- Save disabled when `!_dirty || _saving`; enabled after any field change
- PATCH 2xx: entity refreshed from response body; Saved toast 3s auto-dismiss
- **409 conflict banner (SHOWCASE):** on 409, `_conflictServer = error.current` from error field — NO second GET (D6-03 + Pitfall 9). `<or-conflict-banner mode="crud">` shows server-vs-user diff. User can review+resubmit or discard.
- **Delete (D6-V-40):** sl-dialog with `sl-input` "Type agent name to confirm"; `[Delete]` disabled until `_deleteConfirmName === _entity?.name` (case-sensitive exact match)
- Enable/Disable: PATCH `{ enabled: bool, version }` + entity refresh from response
- Skills sub-table: proficiency sl-select (1-10); remove button; + Add skill search (300ms, GET /skills)
- Wrapup countdown: `setInterval` ticking `hh:mm:ss` when `wrapupUntil` property set
- 8 tests: 3 core (readonly code, dirty tracking, PATCH 2xx) + 5 extensions (409 no re-GET, typed-name delete, enable, skills populate, wrapup countdown)

### or-agent-form (Task 3)
- `<or-form-wizard .steps=${3} .hideNav=${true}>` — stepper shows Basics/Skills/Review
- Step 1 (Basics): or-code-input (required), name (required), email (optional, format validated in Next handler), external_id, enabled switch
- Step 1 Next: validates code regex + name required + email format before advancing
- Step 2 (Skills): optional skill assignment — same UI as detail sub-table
- Step 3 (Review): key-value summary + "Create agent" button → `_handleSubmit()`
- `validateCreateAgent` (ajv) called on final submit
- 409 duplicate_code → back to Step 1 with inline code error "This code is already in use."
- 201 → dispatches `open-routing:navigate` to `/orgs/{orgId}/agents/{newId}`
- 5 tests: stepper steps, Step 1 validation, POST with skills array, 409 back-to-step-1, success navigate

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TypeScript TS2339: ajv .errors property not on function type**
- **Found during:** Peer review (Codex HIGH)
- **Issue:** `validateUpdateAgent.errors` and `validateCreateAgent.errors` — TypeScript doesn't know ajv standalone validators attach `.errors` dynamically
- **Fix:** Cast validator to typed function interface `(d:unknown) => boolean & {errors: ...}`
- **Files modified:** agent-detail.ts, agent-form.ts
- **Commit:** d6942f7

**2. [Rule 1 - Bug] TypeScript TS2352: data cast from undefined in GET /skills**
- **Found during:** Peer review (Codex HIGH)
- **Issue:** `(data as { items?: unknown[] })?.items` — TypeScript rightfully rejects casting undefined
- **Fix:** Destructure through `(result as {data?: unknown}).data` first, then cast
- **Files modified:** agent-detail.ts, agent-form.ts
- **Commit:** d6942f7

**3. [Rule 1 - Bug] external_id PATCH contract: null sent instead of "" to clear**
- **Found during:** Peer review (Codex HIGH)
- **Issue:** `external_id: this._form.external_id || null` sends null which equals omission per UpdateAgentRequest spec (D04_1-07); clearing the field would not actually clear the binding
- **Fix:** Send `this._form.external_id` directly — empty string is the documented sentinel
- **Files modified:** agent-detail.ts
- **Commit:** d6942f7

**4. [Rule 2 - Missing Critical Functionality] or-form-wizard nav buttons duplicated**
- **Found during:** Peer review (Codex MED → structural correctness)
- **Issue:** or-form-wizard always renders its own Back/Next/Create buttons; agent-form also renders nav buttons inside step slots → duplicate buttons in UI; wizard's built-in "Create" fires wizard-completed which agent-form doesn't listen for → POST never fires on wizard button click
- **Fix:** Added `hideNav` property to or-form-wizard (boolean, attribute `hide-nav`); agent-form sets `.hideNav=${true}`
- **Files modified:** form-wizard.ts, agent-form.ts
- **Commit:** d6942f7

**5. [Rule 2 - Missing Critical Functionality] Email format validation missing in Step 1 Next**
- **Found during:** Peer review (Gemini HIGH)
- **Issue:** Users could advance to Step 2 with an invalid email; ajv would catch it only on final submit at Step 3, but the error would display on a step the user is no longer on
- **Fix:** Added email format regex check in `_handleNext` for Step 0
- **Files modified:** agent-form.ts
- **Commit:** d6942f7

**6. [Rule 3 - Blocking] happy-dom lacks Web Animations API — Shoelace sl-dialog fails**
- **Found during:** Task 2b test run
- **Issue:** `Element.getAnimations is not a function` (then `Element.animate is not a function`) as unhandled rejection from Shoelace's dialog animation code
- **Fix:** Polyfilled `Element.prototype.getAnimations` and `Element.prototype.animate` in test-setup.ts
- **Files modified:** test-setup.ts
- **Commit:** 74dde8b

**7. [Rule 3 - Blocking] Test timing: element must be mounted AFTER properties are set**
- **Found during:** Task 2a test run
- **Issue:** Tests set properties after `document.body.appendChild`, so `connectedCallback` fired before `orgId`/`entityId`/`client` were assigned; `_loadEntity()` was a no-op
- **Fix:** All detail tests now set properties before appending to DOM; `mountWithAgent` helper extracted
- **Files modified:** agent-detail.test.ts
- **Commit:** 74dde8b

**8. [Rule 3 - Blocking] Fast-forward merge needed to access phase-06 content**
- **Found during:** Plan start
- **Issue:** Worktree was at Wave 1 base (phase-05 tip); phase-06 content (06-04, primitives, validators) was on `gsd/phase-06-shared-ui-admin` branch
- **Fix:** `git merge --ff-only gsd/phase-06-shared-ui-admin` — fast-forward only, no merge commit
- **Files modified:** (merge — 90 files from Wave 1)
- **Commit:** a5ec796 (pre-existing Wave 1 tip)

## Threat Surface Scan

No new security-relevant surface beyond plan's threat model:
- T-06-05-01 (XSS via 409 current): Lit html`` auto-escapes all `serverValue` interpolations in or-conflict-banner — MITIGATED
- T-06-05-02 (org_id tampering): orgId sourced from router params; server enforces X-Org-Id — MITIGATED
- T-06-05-03 (form bypass): validateCreateAgent/validateUpdateAgent called before every POST/PATCH — MITIGATED
- T-06-05-04 (delete without confirm): `[Delete]` button `?disabled=${!_canDelete}` where `_canDelete = _deleteConfirmName === _entity?.name` — MITIGATED

## Known Stubs

None — agent components fetch real data via client.GET/POST/PATCH/DELETE; no hardcoded placeholders that flow to UI rendering.

## Replicability Pattern (Wave 3-4 guide)

To create Skills entity (Plan 06-07):
1. Copy agent-list.ts → skill-list.ts; replace `Agent` type, `/agents` path, column set
2. Copy agent-detail.ts → skill-detail.ts; remove skills sub-table section, keep 409/delete/enable-disable
3. Copy agent-form.ts → skill-form.ts; set steps to 1 (single-step with or-form-wizard default nav); remove Skills step
4. Update entity barrel (skills/index.ts) and components/index.ts
5. Copy and adapt the 17 test cases (4+8+5 → 4+3+2 for simpler entity)

Key patterns to preserve:
- `Task args() => [orgId, ...state] as const` — no stale closures
- `error.current` (not response.json()) for 409 body
- `_deleteConfirmName === entity.name` for delete confirm
- `client.GET/POST/PATCH/DELETE` only — never `fetch()`

## Self-Check

### Files Verified
- FOUND: web/packages/ui/src/components/agents/agent-list.ts
- FOUND: web/packages/ui/src/components/agents/agent-list.test.ts
- FOUND: web/packages/ui/src/components/agents/agent-detail.ts
- FOUND: web/packages/ui/src/components/agents/agent-detail.test.ts
- FOUND: web/packages/ui/src/components/agents/agent-form.ts
- FOUND: web/packages/ui/src/components/agents/agent-form.test.ts
- FOUND: web/packages/ui/src/components/agents/index.ts
- FOUND: web/packages/ui/src/components/index.ts

### Commits Verified
- ebb8b9d test(06-05): add failing tests for or-agent-list (RED)
- ff73802 feat(06-05): implement or-agent-list with Task-driven API + search + pagination
- 3ef9063 test(06-05): add failing tests for or-agent-detail Task 2a+2b (RED)
- 74dde8b feat(06-05): implement or-agent-detail — 2-col layout, 409 banner, delete/enable/disable, skills, wrapup
- 26408eb test(06-05): add failing tests for or-agent-form 3-step wizard (RED)
- 3d0ddc4 feat(06-05): implement or-agent-form 3-step wizard (Basics → Skills → Review)
- d6942f7 fix(06-05): address Codex + Gemini peer review blockers

### Test Results
- `pnpm --filter @open-routing/ui test` → 57 tests passed, exit 0
- `pnpm --filter @open-routing/ui typecheck` → 0 errors, exit 0
- Agent tests: 4 list + 8 detail + 5 form = 17 new tests (40 pre-existing preserved)

## Self-Check: PASSED
