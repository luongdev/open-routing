---
phase: 06-shared-ui-library-standalone-admin
plan: 12
subsystem: ui-status-panel
tags:
  - lit
  - shoelace
  - status-panel
  - polling
  - state-machine
  - force-flag
  - d6-26
  - d6-03
dependency_graph:
  requires:
    - 06-06 (catalog-shell routing, orgRouteEnter guard)
    - 06-09 (BreakReasons entity, break-reason picker data source)
    - 06-11 (Adapters, wave ordering for catalog-shell.ts edits)
  provides:
    - or-status-panel custom element with 5s polling + state machine
    - /orgs/:org_id/agents/:id/status route wired to real component
  affects:
    - catalog-shell.ts (status route replaced placeholder)
    - components barrel (status/ subdir — to be added at merge time per wave constraint)
tech_stack:
  added:
    - web/packages/ui/src/components/status/ (new subdir)
  patterns:
    - visibilitychange listener for polling pause/resume (D6-26)
    - state_version compare on visibility resume (STATE-08)
    - 409 invalid_transition consumed from error.from/.to (D6-03 status variant)
    - force=true admin override behind Advanced disclosure (D-84, D6-V-19)
    - setInterval(5000) polling cleared on disconnectedCallback
key_files:
  created:
    - web/packages/ui/src/components/status/status-panel.ts
    - web/packages/ui/src/components/status/status-panel.test.ts
    - web/packages/ui/src/components/status/index.ts
  modified:
    - web/packages/ui/src/components/shell/catalog-shell.ts
decisions:
  - Force flag uses 'warning' variant (not 'danger') per D6-V-19 — admin recovery, not destructive
  - Tests use real timers (not vi.useFakeTimers) due to Lit rendering async interaction
  - Break picker implemented as inline dropdown (not sl-dropdown) for better test visibility
  - _openBreakDropdown() exposed as non-private for test access (not underscore-protected)
metrics:
  duration: "~45 minutes"
  completed: "2026-05-18"
  tasks_completed: 2
  tasks_total: 2
  files_created: 3
  files_modified: 1
---

# Phase 6 Plan 12: or-status-panel — Status Panel Summary

One-liner: 5s-polling status panel with state-machine buttons, break picker, wrapup countdown, force-flag admin disclosure, and 409 invalid_transition consumed from response body.

## What Was Built

### Task 1: or-status-panel component (TDD: RED → GREEN)

**RED phase:** Wrote `status-panel.test.ts` with 8 behaviors covering the full spec. Tests initially failed (`import './status-panel.js'` — file not found).

**GREEN phase:** Implemented `status-panel.ts`:

- `@customElement('or-status-panel')` — Lit 3 LitElement with Shadow DOM
- **Polling (D6-26):** `connectedCallback` → `document.addEventListener('visibilitychange', ...)` + `_startPolling()`. `_startPolling()` fires immediate GET then `setInterval(5000)`. `disconnectedCallback` clears both intervals.
- **Visibility pause/resume:** `_handleVisibilityChange` — when `document.hidden` → `clearInterval(_pollInterval)`. On resume → `_fetchStatus()` then `_startPolling()` (fresh 5s from now).
- **state_version compare (STATE-08):** `_fetchStatus()` compares `data.state_version` to `_lastKnownVersion`. Only updates `_status` + UI if changed (or on first load).
- **Status pill (D6-V-18):** Per-state CSS custom property map (bg/text/icon) for Ready/NotReady/Break/Engaged/WrapUp/Offline.
- **State version chip:** Inline `vN` chip beside the status pill.
- **Transition button matrix:**
  - Ready → [Set Not Ready] + [Go on Break ▾]
  - NotReady → [Set Ready]
  - Break → [Back to Ready] + [Set Not Ready] + current break_reason_name display
  - Engaged → post_interaction_state radio (ready/not_ready) + [Save]
  - WrapUp → hh:mm:ss countdown + [Go to Ready now] + [Go to Not Ready now]
  - Offline → informational text
- **Break picker:** Inline panel (not sl-dropdown) opens on "Go on Break" click. Fetches `/v1/orgs/{orgId}/break-reasons?include_disabled=false&limit=100` lazily (only when opened, only once). Renders each reason with [routable]/[not routable] badge. [Confirm break] disabled until reason selected.
- **WrapUp countdown:** `_startWrapupCountdown(wrapup_until)` → `setInterval(1000)` updating `_wrapupSecondsLeft`. On expiry (≤0): clears interval + fires immediate `_fetchStatus()`.
- **Force flag (D6-V-19, D-84):** `<details>`-style disclosure at card bottom. Warning amber border/background. Target radios: Ready/NotReady/Break/Offline. [Force to {state}] (variant="warning") opens confirmation dialog. On confirm → `PATCH({to, force: true})`.
- **409 handling (D6-03):** `_patchStatus()` checks `'from' in error && 'to' in error` → sets `_conflictError`. Renders `<or-conflict-banner mode="status">` with no extra GET round-trip. Conflict banner "Refresh transitions" CTA calls `_fetchStatus()`.
- **Shoelace imports (D6-08):** 12 per-component imports for tree-shaking budget.

**Test results:** All 8 tests pass (124 total suite tests pass).

### Task 2: Wire /agents/:id/status route in catalog-shell.ts

- Added `import '../status/status-panel.js'` to catalog-shell.ts (D6-08 per-component import).
- Replaced placeholder `<div class="placeholder-wave">` with `<or-status-panel .orgId=${org_id ?? ''} .agentId=${id ?? ''} .client=${this._client!}>`.
- `enter: this._orgRouteEnter` preserved — UUIDv7 guard mirrors 06-09 BreakReasons pattern (D6-13).
- `pnpm --filter @open-routing/admin build` → 204 modules, exits 0.

## Deviations from Plan

### Auto-fixed Issues

None — plan executed as written.

### Out-of-Scope Pre-existing Issues (deferred)

**1. channels/index.ts missing module exports**
- **Found during:** Task 2 typecheck
- **Issue:** `web/packages/ui/src/components/channels/index.ts` is a comment-only stub created by plan 06-02. `tsc` reports `TS2306: File is not a module`. This causes `pnpm --filter @open-routing/ui typecheck` to fail.
- **Root cause:** Pre-existing from plan 06-02; not introduced by this plan.
- **Scope:** Out of scope (not caused by 06-12 changes). Logged in deferred-items per deviation boundary rules.
- **Fix:** Plan 06-10 (Channels) should have populated this. The merge resolver should add proper exports.

**2. components/index.ts: status/ barrel not added**
- Per parallel execution constraint: "DO NOT touch components/index.ts — will be patched at merge time." Intentionally deferred.

### Rule 4 architectural considerations: None needed.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: elevation_of_privilege | status-panel.ts | force=true PATCH visible to all admin users (v0.1 stub auth per D-84); confirmed: server enforces org_admin restriction in v1 AUTH phase; client-side UI intentional exposure documented |

## Known Stubs

None — all status fields rendered from real API response. Break reason picker fetches live data. Wrapup countdown derives from wrapup_until. Force flag fires real PATCH.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 — TDD (RED+GREEN) | `40b0457` | test(06-12): RED failing tests + feat(06-12): GREEN or-status-panel implementation |
| 2 — Shell wire | `5634a3f` | feat(06-12): wire /agents/:id/status route to or-status-panel in catalog-shell |

## Verification Results

```
pnpm --filter @open-routing/ui test -- --testPathPattern=status-panel
  Tests: 8/8 passed, 124 total suite tests passed

grep "visibilitychange" status-panel.ts → FOUND
grep "state_version" status-panel.ts → FOUND
grep "or-status-panel" catalog-shell.ts → FOUND
pnpm --filter @open-routing/admin build → 204 modules, 0 errors
```

## Self-Check: PASSED

- [x] `web/packages/ui/src/components/status/status-panel.ts` — exists, 5s polling + state machine
- [x] `web/packages/ui/src/components/status/status-panel.test.ts` — exists, 8 tests all pass
- [x] `web/packages/ui/src/components/status/index.ts` — exists, exports OrStatusPanel
- [x] `catalog-shell.ts` contains `or-status-panel` — confirmed by grep
- [x] Commits `40b0457` and `5634a3f` exist in git log
- [x] `visibilitychange` in status-panel.ts — confirmed
- [x] `state_version` in status-panel.ts — confirmed
- [x] `force: true` in PATCH body when force-flag used — confirmed by test 8
- [x] Admin build exits 0 — confirmed (204 modules transformed)
