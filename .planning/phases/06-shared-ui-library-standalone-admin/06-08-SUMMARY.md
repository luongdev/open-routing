---
phase: 06-shared-ui-library-standalone-admin
plan: "08"
subsystem: web-ui
tags:
  - lit
  - queues
  - crud
  - channel-types
  - badges
  - multi-select
  - conflict-banner
  - integer-fields
dependency_graph:
  requires:
    - "06-01"
    - "06-02"
    - "06-03"
    - "06-04"
    - "06-05"
  provides:
    - or-queue-list
    - or-queue-detail
    - or-queue-form
    - queues-index-barrel
  affects:
    - "06-10"
tech_stack:
  added: []
  patterns:
    - "channel_types array → one sl-badge per type (variant=neutral pill) in list view"
    - "acw_sec column renders as '{N}s' format string"
    - "priority column right-aligned via inline style on span"
    - "sl-select multiple for channel_types multi-select in detail + form"
    - "parseInt(..., 10) with NaN fallback for integer fields (priority, acw_sec)"
    - "409 error.current → both _conflictServer AND _entity updated (version staleness fix)"
    - "validateCreateQueue/validateUpdateQueue ajv cast for .errors access"
    - "Single-step wizard: or-form-wizard.hideNav=true with WIZARD_STEPS=[{basics}]"
    - "channel_types empty-array guard before ajv: explicit error before submit"
key_files:
  created:
    - web/packages/ui/src/components/queues/queue-list.ts
    - web/packages/ui/src/components/queues/queue-list.test.ts
    - web/packages/ui/src/components/queues/queue-detail.ts
    - web/packages/ui/src/components/queues/queue-detail.test.ts
    - web/packages/ui/src/components/queues/queue-form.ts
    - web/packages/ui/src/components/queues/queue-form.test.ts
  modified:
    - web/packages/ui/src/components/queues/index.ts
decisions:
  - "409 fix: _entity updated from error.current on conflict (not just _conflictServer) so re-submit PATCH uses server's current version"
  - "Delete error surfacing: _handleDelete sets _apiError on failure (was silently swallowed)"
  - "catalog-shell.ts NOT modified — 06-06 owns that file per parallel execution contract"
  - "channel_types pre-check before ajv in queue-form._handleSubmit catches empty array before ajv sees it"
metrics:
  duration: "27m"
  completed_date: "2026-05-18"
  tasks_completed: 2
  tests_added: 14
  files_created: 6
  files_modified: 1
---

# Phase 6 Plan 08: Queues Entity CRUD Components Summary

**One-liner:** Queues entity UI — Task-driven list with channel_types badges (sl-badge per type) + acw_sec "{N}s" format, single-column detail with 409 conflict banner from updated error.current + typed-name delete, and single-step wizard create form with channel_types multi-select (minItems=1).

## What Was Built

### or-queue-list (Task 1)

- `@lit/task` Task args tuple: `[orgId, search, cursor, includeDisabled, limit] as const`
- Column set: code (mono), name, channel_types (one `sl-badge variant="neutral" pill` per type), priority (right-aligned), acw_sec ("{N}s"), enabled (check/x icon), updated_at (relative+tooltip), context menu
- `_handleDisable(_handleEnable)`: PATCH enabled flag + version; refreshes list task
- Empty state "No queues yet" with description text; search-active "No queues found matching '...'"
- Per-component Shoelace imports including `badge.js` (D6-08)
- 6 tests: empty state, channel_types data, acw_sec format, row-click navigate, create button, disable PATCH

### or-queue-detail (Task 2)

- Single-column form: code (or-code-input readonly), name, external_id, channel_types (sl-select multiple), priority (number step=1), acw_sec (number step=1), enabled
- **409 conflict (SHOWCASE):** on 409, `_conflictServer = error.current` AND `_entity = error.current` — dual update ensures re-submit PATCH uses server version (Codex HIGH fix). `<or-conflict-banner mode="crud">` renders diff.
- `validateUpdateQueue` (ajv cast) blocks save if channel_types.length === 0 (minItems=1)
- **Delete surfacing:** `_handleDelete` sets `_apiError` on failure (Gemini MED fix)
- **Typed-name delete:** `_deleteConfirmName === _entity.name` required (D6-V-40)
- Dirty tracking; Saved toast 3s; Enable/Disable top-bar buttons
- 4 tests: code readonly, channel_types sl-select multiple with 3 sl-option, validateUpdateQueue rejects empty, 409 no re-GET + conflict banner

### or-queue-form (Task 2)

- Single-step wizard: `WIZARD_STEPS=[{key:'basics', label:'Basics'}]`; `or-form-wizard.hideNav=true` (D6-17)
- Fields: code (or-code-input), name, external_id, channel_types (sl-select multiple required), priority (number step=1), acw_sec (number step=1 + "seconds" helper), enabled
- Empty channel_types check fires before ajv: immediate error "Select at least one channel type."
- `validateCreateQueue` (ajv cast) on full submit
- 409 `duplicate_code` → inline code error
- 201 → navigate to `/orgs/{orgId}/queues/{newId}`
- 4 tests: single-step wizard Basics, channel_types required, number inputs step=1, POST → navigate

### queues/index.ts

Updated to export `OrQueueList`, `OrQueueDetail`, `OrQueueForm` (was empty stub).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Codex HIGH: 409 version staleness on re-submit**
- **Found during:** Cross-AI peer review (Codex BLOCK)
- **Issue:** On 409, only `_conflictServer` was set from `error.current`; `_entity.version` remained stale. "Review and re-submit" would PATCH with the old version again, producing another 409 conflict infinitely.
- **Fix:** On 409, update `_entity = error.current` in addition to `_conflictServer = error.current`. Next PATCH body uses `_entity.version` which is now the server's current version.
- **Files modified:** queue-detail.ts
- **Commit:** 719c5e5

**2. [Rule 2 - Missing] Gemini MED: DELETE error silently swallowed**
- **Found during:** Cross-AI peer review (Gemini MED)
- **Issue:** `_handleDelete` navigated on success but had no error handler. Referential integrity failures or other DELETE errors left the dialog open silently.
- **Fix:** On DELETE error, close dialog and set `_apiError` with the server reason.
- **Files modified:** queue-detail.ts
- **Commit:** 719c5e5

### Intentional Deviation from Plan Tasks

**3. catalog-shell.ts NOT modified (plan task 2 listed it)**
- **Reason:** Parallel execution constraint: "DO NOT touch shell/catalog-shell.ts (06-06 owns that)". The queue routes remain as placeholder divs until 06-06 wires them. This is not a bug — it is the intended parallel execution boundary.
- **Impact:** Queue routes in shell not yet wired; addressed by 06-06.

## Threat Surface Scan

No new security-relevant surface beyond plan's threat model:
- T-06-08-01 (channel_types tamper): `validateCreateQueue/validateUpdateQueue` enforces enum+minItems; server validates independently — MITIGATED
- T-06-08-02 (negative integer injection): `sl-input type="number" min="0"` + server-side 422 — MITIGATED
- T-06-08-03 (409 information disclosure): org-scoped; X-Org-Id enforced by server middleware — ACCEPTED
- T-06-08-SC (no new npm installs): uses packages from Plan 06-01 legitimacy audit — ACCEPTED

## Known Stubs

None — all three queue components fetch real data via client.GET/POST/PATCH/DELETE; no hardcoded placeholders that flow to UI rendering.

## Self-Check

### Files Verified

- FOUND: web/packages/ui/src/components/queues/queue-list.ts
- FOUND: web/packages/ui/src/components/queues/queue-list.test.ts
- FOUND: web/packages/ui/src/components/queues/queue-detail.ts
- FOUND: web/packages/ui/src/components/queues/queue-detail.test.ts
- FOUND: web/packages/ui/src/components/queues/queue-form.ts
- FOUND: web/packages/ui/src/components/queues/queue-form.test.ts
- FOUND: web/packages/ui/src/components/queues/index.ts

### Commits Verified

- a21d954 test(06-08): add failing tests for or-queue-list (RED)
- f6b7cb1 feat(06-08): implement or-queue-list with channel_types badges, acw_sec format, priority column
- e498138 test(06-08): add failing tests for or-queue-detail and or-queue-form (RED)
- 89e8266 feat(06-08): implement or-queue-detail and or-queue-form (channel_types multi-select, integer fields)
- 719c5e5 fix(06-08): address Codex HIGH + Gemini MED peer review findings

### Test Results

- `pnpm --filter @open-routing/ui test` → 71 tests passed, exit 0
- `pnpm --filter @open-routing/admin build` → 128 modules transformed, exit 0
- Queue tests: 6 list + 4 detail + 4 form = 14 new tests (57 pre-existing preserved)

## Self-Check: PASSED
