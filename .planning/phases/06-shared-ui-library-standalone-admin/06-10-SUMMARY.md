---
phase: 06-shared-ui-library-standalone-admin
plan: "10"
subsystem: web-ui
tags:
  - lit
  - channels
  - crud
  - wizard-3-step
  - queue-picker
  - nullable-uuid
  - conflict-banner
  - routable
dependency_graph:
  requires:
    - "06-01"
    - "06-02"
    - "06-03"
    - "06-04"
    - "06-05"
    - "06-08"
    - "06-09"
  provides:
    - or-queue-picker
    - or-channel-list
    - or-channel-detail
    - or-channel-form
    - channels-barrel
    - catalog-shell-channel-routes
  affects:
    - "06-12"
    - "06-13"
tech_stack:
  added: []
  patterns:
    - "or-queue-picker: @lit/task with args [orgId, search] for debounced async select; emits or-queue-picker-change with {queueId: string | null}"
    - "queue-picker tagName guard: _handleChange filters sl-change events by target.tagName to prevent sl-input bubble contamination"
    - "nullable UUID ajv bypass: validationBody excludes null default_queue_id to work around allOf+nullable UUID bug in generated validators"
    - "3-step wizard: or-form-wizard with steps=[Basics, Default queue, Review], hideNav=true, parent owns navigation"
    - "default_queue_id truncation: null → '—'; non-null → first 8 chars + '…' + sl-tooltip with full UUID"
    - "external_id normalization: PATCH body uses `external_id || undefined` (consistent with create form)"
    - "enable/disable error surfacing: _handleEnable/_handleDisable set _apiError on PATCH failure"
    - "409 dual-update: _entity = error.current in addition to _conflictServer (version staleness prevention)"
key_files:
  created:
    - web/packages/ui/src/components/primitives/queue-picker.ts
    - web/packages/ui/src/components/primitives/queue-picker.test.ts
    - web/packages/ui/src/components/channels/channel-list.ts
    - web/packages/ui/src/components/channels/channel-list.test.ts
    - web/packages/ui/src/components/channels/channel-detail.ts
    - web/packages/ui/src/components/channels/channel-detail.test.ts
    - web/packages/ui/src/components/channels/channel-form.ts
    - web/packages/ui/src/components/channels/channel-form.test.ts
  modified:
    - web/packages/ui/src/components/channels/index.ts
    - web/packages/ui/src/components/primitives/index.ts
    - web/packages/ui/src/components/shell/catalog-shell.ts
decisions:
  - "ajv allOf+nullable UUID bug: validationBody excludes default_queue_id when null; POST/PATCH body includes it explicitly (null serialized to clear)"
  - "queue-picker tagName guard: _handleChange short-circuits when e.target is not sl-select (Gemini HIGH bubble contamination fix)"
  - "break-reason routes upgraded from attribute bindings to property bindings (Codex HIGH: missing .client prop)"
  - "enable/disable errors surfaced via _apiError (Gemini MED: was silently swallowed)"
  - "external_id coerced to undefined when empty in PATCH body (Gemini MED: consistency with create form)"
metrics:
  duration: "32m"
  completed_date: "2026-05-17"
  tasks_completed: 2
  tests_added: 16
  files_created: 8
  files_modified: 3
---

# Phase 6 Plan 10: Channels Entity (List + Detail + 3-Step Wizard + Queue Picker) Summary

**One-liner:** Channels CRUD with 3-step wizard (Basics → Default queue → Review), async or-queue-picker primitive with debounced search + null clear, 409 via error.current (D6-03), and channel routes wired in catalog-shell with UUIDv7 enter guard.

## What Was Built

### or-queue-picker primitive (Task 1)

- `@lit/task` Task with `args: () => [this.orgId, this._search]` — re-fetches on orgId or search change
- GET `/v1/orgs/{orgId}/queues` with `name=_search` (omitted if empty), `limit=25`
- sl-select with "(none)" option (clears to null), sl-divider, then per-queue sl-option elements
- 300ms debounced search input at top (sl-input slot, stops click propagation to avoid dropdown close)
- `_handleChange` filters by `e.target.tagName.toLowerCase() === 'sl-select'` — prevents sl-input sl-change bubble contamination (Gemini HIGH fix)
- Emits `or-queue-picker-change` CustomEvent `{queueId: string | null}` — null when "(none)" selected
- "Load more" sl-button visible when `_hasMore === true` (cursor-based pagination)
- T-06-10-04 mitigated: limit=25 + 300ms debounce prevents keystroke-per-request DoS

### or-channel-list (Task 1)

- `@lit/task` Task args tuple: `[orgId, search, cursor, includeDisabled, limit] as const`
- Column set per UI-SPEC §5.3 D6-V-10 Channels: code (mono), name, channel_type (plain text), default_queue_id, enabled (check/x icons), updated_at (relative+tooltip), [⋮] context menu
- **default_queue_id column**: null/undefined → `<span>—</span>`; non-null → `<sl-tooltip content="${fullUUID}"><code>${first8}…</code></sl-tooltip>`
- Empty state "No channels yet" + "Channels connect your routing system to external communication platforms." + [+ Create channel]
- Search-active empty: "No channels found matching '...'" + Clear search button
- Row click and CTA dispatch `open-routing:navigate`
- 4 tests: empty state, default_queue_id truncate+tooltip render function, null em-dash, row-click navigate

### or-channel-detail (Task 2)

- GET `/v1/orgs/{orgId}/channels/{entityId}` on `connectedCallback`
- code → `<or-code-input .value=${entity.code} .readonly=${true}>` (D04_1-02 immutable post-create)
- channel_type → `<sl-select>` single with voice/chat/email options (T-06-10-01 enum enforcement)
- default_queue_id → `<or-queue-picker .orgId .client .value>` with `@or-queue-picker-change` handler setting null on clear
- **409 conflict**: `_conflictServer = error.current` AND `_entity = error.current` (no re-GET D6-03; dual-update prevents version staleness on re-submit)
- `<or-conflict-banner mode="crud">` renders inline
- `validateUpdateChannel` ajv on save; null bypass for allOf+nullable UUID quirk
- Enable/Disable errors surfaced via `_apiError` (Gemini MED fix)
- external_id normalized to `undefined` when empty in PATCH body (Gemini MED consistency fix)
- Typed-name delete dialog (D6-V-40 pattern)
- 4 tests: code readonly, channel_type 3 options, or-queue-picker null clear, 409 no-re-GET + banner

### or-channel-form (Task 2)

- 3-step wizard: `WIZARD_STEPS=[{key:'basics',label:'Basics'},{key:'default-queue',label:'Default queue'},{key:'review',label:'Review'}]`
- `or-form-wizard.hideNav=true` — parent owns navigation per agent pattern
- **Step 1 Basics**: code (or-code-input required), name (sl-input required), channel_type (sl-select required voice/chat/email), external_id (optional), enabled (sl-switch default true)
- Step 1 Next validates: code pattern, name non-empty, channel_type non-empty; stays on step 0 with errors
- **Step 2 Default queue**: `<or-queue-picker>` optional — advances unconditionally (null allowed)
- **Step 3 Review**: key-value summary of all fields + [← Back] + [Create channel]
- ajv validationBody excludes null default_queue_id (allOf+nullable UUID bypass); POST body includes it explicitly
- 409 duplicate_code → step 0 with inline code error; 201 → navigate to detail
- 4 tests: 3-step wizard labels, Step 1 validation stays on step 0, Step 2 optional, Review POST

### catalog-shell.ts (Task 2)

- Imports channel-list.js, channel-detail.js, channel-form.js
- Routes wired with property bindings + `enter: this._orgRouteEnter` on all 3:
  - `/orgs/:org_id/channels` → `<or-channel-list .orgId .client>`
  - `/orgs/:org_id/channels/new` → `<or-channel-form .orgId .client>`
  - `/orgs/:org_id/channels/:id` → `<or-channel-detail .orgId .entityId .client>`
- Also fixed break-reason routes (Codex HIGH): upgraded from attribute bindings to property bindings with `.client=${this._client!}`

### channels/index.ts

Exports: `OrChannelList`, `OrChannelDetail`, `OrChannelForm`

### primitives/index.ts

Appended: `export { OrQueuePicker } from './queue-picker.js'` (existing exports preserved)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ajv allOf+nullable UUID validator fails null default_queue_id**
- **Found during:** Task 1+2 GREEN phase
- **Issue:** Generated `CreateChannelRequest.ts` validator has a bug: when `default_queue_id` is null, the allOf's `UUIDv7` schema `else { "must be string" }` fires. Passing null to `validateFn` returns false.
- **Fix:** Built two separate objects: `validationBody` (excludes null default_queue_id) for ajv check; full `body` (includes explicit null) for actual POST/PATCH.
- **Files modified:** channel-form.ts, channel-detail.ts
- **Commits:** 3209c9c, b62643f

**2. [Rule 2 - Missing] Codex HIGH: break-reason routes missing .client property**
- **Found during:** Cross-AI peer review (Codex BLOCK)
- **Issue:** Break-reason routes in catalog-shell.ts used attribute bindings `org-id=` without `.client` property. Components call `this.client.GET/POST/PATCH` at runtime but receive undefined.
- **Fix:** Converted all 3 break-reason routes to property bindings `.orgId`, `.client`, `.entityId` matching the channel route pattern.
- **Files modified:** catalog-shell.ts
- **Commit:** 2898aed

**3. [Rule 1 - Bug] Gemini HIGH: sl-input sl-change bubbles to sl-select handler**
- **Found during:** Cross-AI peer review (Gemini HIGH)
- **Issue:** In queue-picker, sl-input inside sl-select can fire sl-change that bubbles up to the `@sl-change` on sl-select. `_handleChange` would then grab `input.value` (search string) instead of a UUID.
- **Fix:** Added tagName guard: `if (target.tagName.toLowerCase() !== 'sl-select') return;`
- **Files modified:** queue-picker.ts, queue-picker.test.ts
- **Commit:** b62643f

**4. [Rule 2 - Missing] Gemini MED: enable/disable errors silently swallowed**
- **Found during:** Cross-AI peer review (Gemini MED)
- **Issue:** `_handleEnable` and `_handleDisable` had no error handling — PATCH failures were invisible to the user.
- **Fix:** On PATCH error, set `_apiError` with the reason.
- **Files modified:** channel-detail.ts
- **Commit:** b62643f

**5. [Rule 2 - Missing] Gemini MED: external_id inconsistency in PATCH body**
- **Found during:** Cross-AI peer review (Gemini MED)
- **Issue:** channel-detail PATCH sent `external_id: ""` (empty string) while channel-form POST sent `external_id: undefined`. Backend may reject empty string with 422 minLength.
- **Fix:** PATCH body now uses `external_id: this._formData.external_id || undefined`.
- **Files modified:** channel-detail.ts
- **Commit:** b62643f

## Cross-AI Peer Review

**Codex:** BLOCK → addressed in fix(06-10) commit 2898aed
- HIGH: break-reason routes missing .client → fixed (property bindings)
- Other concerns pre-existing (Skills/Queues placeholders, loadLocale typing, ESLint) — out of scope for 06-10

**Gemini:** READY WITH FIXES → addressed in fix(06-10) commit b62643f
- HIGH: queue-picker sl-change bubble contamination → tagName guard added
- MED: enable/disable silent swallow → _apiError surfaced
- MED: external_id empty string inconsistency → normalized to undefined

**Verdict after fixes:** READY

## Known Stubs

None — all 4 components fetch real data via client.GET/POST/PATCH/DELETE; no hardcoded placeholders that flow to UI rendering. The queue-picker fetches live queue data; channels fetch live channel data.

## Threat Surface Scan

No new surface beyond plan's threat model:
- T-06-10-01 (channel_type enum injection): ajv validateCreateChannel/validateUpdateChannel enforces enum; server validates independently; 422 on invalid value — MITIGATED
- T-06-10-02 (cross-org queue UUID): Server enforces X-Org-Id middleware; client has no cross-org access — ACCEPTED (server-side control)
- T-06-10-03 (or-queue-picker exposes queue data): Queues are org-scoped data visible to admin — ACCEPTED
- T-06-10-04 (unbounded search): limit=25 + 300ms debounce — MITIGATED
- T-06-10-SC (no new npm installs): uses packages from Plan 06-01 legitimacy audit — ACCEPTED

## Self-Check

### Files Verified
- FOUND: web/packages/ui/src/components/primitives/queue-picker.ts
- FOUND: web/packages/ui/src/components/primitives/queue-picker.test.ts
- FOUND: web/packages/ui/src/components/channels/channel-list.ts
- FOUND: web/packages/ui/src/components/channels/channel-list.test.ts
- FOUND: web/packages/ui/src/components/channels/channel-detail.ts
- FOUND: web/packages/ui/src/components/channels/channel-detail.test.ts
- FOUND: web/packages/ui/src/components/channels/channel-form.ts
- FOUND: web/packages/ui/src/components/channels/channel-form.test.ts
- FOUND: web/packages/ui/src/components/channels/index.ts (modified)
- FOUND: web/packages/ui/src/components/primitives/index.ts (modified)
- FOUND: web/packages/ui/src/components/shell/catalog-shell.ts (modified)

### Commits Verified
- 82ffd14 test(06-10): add failing tests for or-queue-picker + or-channel-list (RED)
- 440658b feat(06-10): implement or-queue-picker + or-channel-list (GREEN)
- 9571fc8 test(06-10): add failing tests for or-channel-detail + or-channel-form (RED)
- 3209c9c feat(06-10): implement or-channel-detail + or-channel-form + shell routing (GREEN)
- 2898aed fix(06-10): address Codex HIGH — wire .client to break-reason shell routes
- b62643f fix(06-10): address Gemini HIGH + MED peer review findings

### Test Results
- `pnpm --filter @open-routing/ui test` → 132 tests passed, exit 0
- `pnpm --filter @open-routing/admin build` → 201 modules transformed, exit 0 (248KB bundle)
- New tests: 4 queue-picker + 4 channel-list + 4 channel-detail + 4 channel-form = 16 new tests (116 pre-existing preserved)
- Grep: "or-channel-list" in catalog-shell.ts → PASS
- Grep: "or-queue-picker" in channel-detail.ts → PASS
- Grep: "validateUpdateChannel" in channel-detail.ts → PASS

## Self-Check: PASSED
