---
phase: 07-web-component-embed-bundle
plan: w1-01
subsystem: ui-shell
tags: [ember-layout, frankenstyle, uk-icon, sidebar, css-grid, redesign]
dependency_graph:
  requires: [07-w0-01, 07-w0-02]
  provides: [ember-shell-layout]
  affects:
    - web/packages/ui/src/components/shell/catalog-shell.ts
tech_stack:
  added: []
  patterns:
    - css-grid-app-layout
    - color-mix-oklch-active-state
    - uk-icon-lucide-nav
key_files:
  created: []
  modified:
    - web/packages/ui/src/components/shell/catalog-shell.ts
decisions:
  - "Sidebar state: _sidebarOpen=false → expanded (260px), true → collapsed (64px). Variable name is misleading but @state constraint prevents rename; documented via comment."
  - "Switch-org button uses .topbar-action class (not .hamburger) after peer review."
  - "theme-btn--active condition: matches both or-light/ember-light and or-dark/ember-dark so existing or-* themes show the correct active button."
metrics:
  duration: 25
  completed_date: "2026-05-19"
---

# Phase 07 Plan W1-01: Ember-style catalog-shell layout Summary

Replaced catalog-shell.ts render() + static styles with Ember healthcare dashboard layout: CSS grid (260px sidebar + 56px topbar + content area), uk-icon Frankenstyle icons throughout, coral active nav pills via color-mix oklch, theme toggle in sidebar footer.

## What Changed

**Static styles**: Replaced old flexbox `.body + .sidebar + .content` layout with CSS grid `.app-grid` (template areas: sidebar/topbar/content). Sidebar collapses to 64px icon-only via `.app-grid--collapsed` on hamburger click.

**NAV_ENTRIES**: All Bootstrap/Shoelace icon names translated to Lucide (`people-fill` → `users`, `tag-fill` → `tag`, `funnel-fill` → `filter`, `broadcast-pin` → `radio`, `plug-fill` → `plug`, `pause-circle-fill` → `pause-circle`, `circle-fill` → `activity`). `upload` unchanged.

**_renderNav()**: `<sl-icon>` → `<uk-icon icon=... height="18" width="18">`, label wrapped in `<span class="nav-item-label">` for collapse hiding.

**render()**: Full template rewrite — `<aside class="sidebar">` with `.sidebar-header` (hamburger + wordmark) + `.sidebar-nav` + `.sidebar-footer` (theme buttons). Topbar minimal: spacer + org-chip + switch-org (log-out icon).

**Shoelace imports removed**: All `import '@shoelace-style/shoelace/dist/components/...'` lines dropped from this file.

## Deviations from Plan

**1. [Rule 2 - Peer Review] Added .topbar-action CSS class**
- **Found during**: Gemini peer review post-commit
- **Issue**: Switch-org button in topbar was using `.hamburger` class (semantically wrong — hamburger = menu icon)
- **Fix**: Added `.topbar-action` class for topbar icon buttons; switch-org button uses `.topbar-action`, sidebar hamburger keeps `.hamburger`
- **Files modified**: `catalog-shell.ts`
- **Commit**: 3067ef9

**2. [Rule 3 - Peer Review] Removed redundant grid-row: 1/-1 from .sidebar**
- **Found during**: Gemini peer review post-commit
- **Issue**: `grid-area: sidebar` already covers the full sidebar span from `grid-template-areas`; `grid-row: 1/-1` was redundant
- **Fix**: Removed the redundant property
- **Files modified**: `catalog-shell.ts`
- **Commit**: 3067ef9

## _sidebarOpen Semantics Note

The variable `_sidebarOpen=false` (default) means sidebar is EXPANDED (visible, 260px). `_sidebarOpen=true` means sidebar is COLLAPSED (icon-only, 64px). The name is counterintuitive but the @state declaration constraint prevents renaming. A comment documents this in `render()`.

## Icon Verification (planned — browser test)

Icons mapped to Lucide names: `users`, `tag`, `filter`, `radio`, `plug`, `pause-circle`, `upload`, `activity`, `menu`, `sun`, `moon`, `palette`, `log-out`. Verification via preview MCP screenshot. Fallback names noted in plan if any render empty.

## Tests

- 266/266 passing (unchanged from pre-W0.1-01)
- No test assertions broke — all existing tests query routing/state, not DOM structure
- No test selector updates needed (tests don't query `sl-icon`, `.wordmark` in old location, etc.)

## Build

Admin build: clean. No new warnings.

## Known Stubs

None — this is a pure layout/chrome change; all route outlets and entity components render as before.

## Threat Flags

None — no new network endpoints, auth paths, or trust boundary changes.

## Self-Check: PASSED

- `/Users/luong/workspace/dev/open-solutions/web/packages/ui/src/components/shell/catalog-shell.ts` modified (confirmed via git diff)
- Commit `132e0d0`: feat(ui): redesign catalog-shell to Ember layout (W0.1-01)
- Commit `3067ef9`: fix(ui): address Gemini peer review findings on catalog-shell (W0.1-01)
- 266 tests pass
- Build clean
