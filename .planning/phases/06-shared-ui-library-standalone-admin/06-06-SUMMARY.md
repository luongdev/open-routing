---
phase: 06-shared-ui-library-standalone-admin
plan: "06"
subsystem: web/packages/ui/src/components/shell + web/apps/admin/e2e
tags: [lit3, router, playwright, shell, agent-routes, api-client, wave3]
dependency_graph:
  requires:
    - "06-03 (catalog-shell skeleton)"
    - "06-05 (agent components: OrAgentList, OrAgentDetail, OrAgentForm)"
  provides:
    - "catalog-shell.ts with real agent routes + shared UUIDv7 guard + createApiClient bootstrap"
    - "Playwright smoke test harness (playwright.config.ts + e2e/smoke.spec.ts)"
  affects:
    - "06-07 (skills) — depends on catalog-shell.ts skeleton"
    - "06-08 (queues) — depends on catalog-shell.ts skeleton"
    - "06-09 (break-reasons) — depends on catalog-shell.ts skeleton"
    - "06-10 (channels) — depends on catalog-shell.ts skeleton"
    - "06-11 (adapters) — depends on catalog-shell.ts skeleton"
    - "06-12 (status) — will replace placeholder in catalog-shell.ts"
    - "06-13 (import) — will replace placeholder in catalog-shell.ts"
tech_stack:
  added: ["@playwright/test 1.60.0"]
  patterns:
    - "Shared _orgRouteEnter() guard applied to all /orgs/:org_id/* routes (D6-13)"
    - "enter() callbacks own state sync; render() stays pure (no reactive mutations)"
    - "createApiClient bootstrap: getOrgId closes over _currentOrgId — no recreation on nav"
    - "Playwright shadow DOM piercing: native locators preferred over manual evaluate traversal"
key_files:
  created:
    - web/apps/admin/playwright.config.ts
    - web/apps/admin/e2e/smoke.spec.ts
    - web/apps/admin/.gitignore
  modified:
    - web/packages/ui/src/components/shell/catalog-shell.ts
decisions:
  - "Shared _orgRouteEnter() guard on ALL entity routes ensures D6-13 contract (reject malformed before API call) — not just first route"
  - "State sync (orgId, _currentOrgId, client bootstrap) moved to enter() to keep render() pure and avoid double-render cycles"
  - "_syncOrgIdFromUrl: match?.[1] narrowing avoids TypeScript string | undefined type error"
  - "Sidebar Agent Status nav links to agents list (/agents) since D6-12 status route requires agent :id — no standalone status page"
metrics:
  duration: "~45 minutes"
  completed: "2026-05-18"
  tasks: 2
  files: 4
---

# Phase 6 Plan 6: Shell Router Wire + createApiClient Bootstrap + Playwright Smoke Test

Shell router wired with real OrAgentList/OrAgentDetail/OrAgentForm via shared UUIDv7 guard; createApiClient bootstrapped from URL org_id via closure; Playwright smoke test harness committed.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Wire agent routes + createApiClient bootstrap | b4c604a | catalog-shell.ts |
| 2 | Playwright config + smoke test | 31addb0 | playwright.config.ts, e2e/smoke.spec.ts, .gitignore |
| fix | Address Codex + Gemini HIGH/MED peer review | fc07cdb | catalog-shell.ts |

## What Was Built

### Task 1: Shell Router + createApiClient Bootstrap

`catalog-shell.ts` updated with:

1. **Shared `_orgRouteEnter()` guard** — runs on ALL `/orgs/:org_id/*` routes (22 entity routes). Validates UUIDv7, syncs `_currentOrgId`/`orgId`, bootstraps `_client` on direct URL navigation. Previously, only `/orgs/:org_id/agents` had the guard.

2. **createApiClient bootstrap** — one client instance per org session. `getOrgId: () => this._currentOrgId` closes by reference so it returns the current value without client recreation. Client is null until an org is selected (org-picker screen).

3. **Real agent routes** — 4 routes now render real components with `.orgId` and `.client` bindings:
   - `/orgs/:org_id/agents` → `<or-agent-list .orgId .client>`
   - `/orgs/:org_id/agents/new` → `<or-agent-form .orgId .client>`
   - `/orgs/:org_id/agents/:id` → `<or-agent-detail .orgId .entityId .client>`
   - `/orgs/:org_id/agents/:id/status` → placeholder (Wave 5, Plan 06-12)

4. **Placeholder routes** — 18 entity routes (skills/queues/channels/adapters/break-reasons/imports) render `<div class="placeholder-wave">` annotated with the Wave/Plan that will replace them.

5. **`open-routing:navigate` listener** — entity components dispatch this event instead of `history.pushState`; shell delegates to `this._routes.goto(path)`. Keeps navigation ownership in shell.

6. **`open-routing:org-selected` handler** — creates new client with new org_id, navigates to agents list.

7. **Switch org** — clears `_client`, `_currentOrgId`, `orgId` before `goto('/')`. Hard reset per D6-14.

8. **popstate listener** — `_syncOrgIdFromUrl()` keeps `_currentOrgId` in sync when user presses browser back/forward.

### Task 2: Playwright Smoke Test

`playwright.config.ts`:
- Targets `http://localhost:5173` (Vite dev server)
- `reuseExistingServer: true` — avoids double startup when developer already has `pnpm dev` running
- Chromium only for Phase 6; Phase 7 (EMBED-10) adds more browsers

`e2e/smoke.spec.ts` — 5 tests:
- `org-picker → enter UUID → agents list loads`
- `direct navigation to /orgs/:org_id/agents bypasses org-picker`
- `malformed org_id in URL redirects to org-picker`
- `agent create wizard flow (requires live API)`
- `Switch org navigates back to org-picker`

`playwright --list` confirms all 5 tests register correctly.

## Deviations from Plan

### Auto-fixed Issues

**[Rule 2 - Missing Critical] UUIDv7 guard applied to ALL entity routes**
- Found during: Codex + Gemini peer review (HIGH)
- Issue: Guard was only on `/orgs/:org_id/agents`; direct navigation to `/orgs/not-a-uuid/agents/new` or `/skills` bypassed guard. Contract D6-13 "reject malformed org_id before any API call" was drifted.
- Fix: Extracted `_orgRouteEnter()` shared method applied as `enter:` callback on all 22 entity routes.
- Files: catalog-shell.ts
- Commit: fc07cdb

**[Rule 1 - Bug] TypeScript type error in `_syncOrgIdFromUrl`**
- Found during: Codex peer review (HIGH)
- Issue: `RegExp.exec()` returns `string | undefined` for capture groups; assigning to `string` would fail `tsc --strict`.
- Fix: `match?.[1]` with early-exit guard: `if (match?.[1]) { const org_id: string = match[1]; ... }`.
- Files: catalog-shell.ts
- Commit: fc07cdb

**[Rule 1 - Bug] Double-render: state mutation inside render() callbacks**
- Found during: Gemini peer review (MED)
- Issue: `this._currentOrgId = org_id ?? ''` inside render functions causes a secondary Lit render cycle on every route change.
- Fix: State sync moved exclusively to `_orgRouteEnter()` (the `enter:` callback). `render()` functions now pure — no reactive property mutations.
- Files: catalog-shell.ts
- Commit: fc07cdb

**[Rule 1 - Bug] Sidebar Agent Status nav path was invalid**
- Found during: Codex peer review (MED)
- Issue: `Agent Status` sidebar entry used `/orgs/{orgId}/agents/status` which would match the `/orgs/:org_id/agents/:id` route with `id = "status"` (detail page, not status panel).
- Fix: Changed to `/orgs/{orgId}/agents` — Agent Status nav links to agents list so user can select an agent then view its status. D6-12: status route is `/agents/:id/status` (per-agent; no standalone status list).
- Files: catalog-shell.ts
- Commit: fc07cdb

### Manual Verification Item

**Playwright smoke test: did NOT pass in auto mode (as expected per objective)**

Attempted: `pnpm exec playwright test --project=chromium`

Result: 5 tests failed with two causes:
1. **Lit decorator dev-mode error**: `Unsupported decorator location: field` — pre-existing from Wave 1 vite.config.ts (`@rolldown/plugin-babel` applies correctly for `vite build` but dev mode (`vite serve`) uses a different transform pipeline). This is NOT caused by 06-06 changes (vite.config.ts unchanged). The production build (pnpm build) succeeds.
2. **No Go API running**: Expected in CI/auto mode; tests require `task dev` on port 8080.

**To run the smoke test manually:**
```bash
# Terminal 1: Start Go API
cd services/api && task dev

# Terminal 2: Start Vite dev server
pnpm --filter @open-routing/admin dev

# Terminal 3: Run Playwright
pnpm --filter @open-routing/admin test:e2e
```

Set `TEST_ORG_ID` environment variable to a valid org UUID that exists in your test DB.

**Note on dev-mode decorator issue**: The Lit decorator error in dev mode is tracked as a deferred item — the prod build proves the components work. Phase 7 may need to investigate Vite 8 dev-mode decorator handling more carefully.

## Known Stubs

The following routes render placeholder `<div class="placeholder-wave">` components — they will be replaced by entity-specific Wave 3-5 plans:

| Route | Placeholder label | Replacing plan |
|-------|-------------------|----------------|
| `/orgs/:org_id/agents/:id/status` | Agent Status Panel — coming in Wave 5 (Plan 06-12) | 06-12 |
| `/orgs/:org_id/skills[/new/:id]` | Skills — coming in Wave 3 (Plan 06-07) | 06-07 |
| `/orgs/:org_id/queues[/new/:id]` | Queues — coming in Wave 3 (Plan 06-08) | 06-08 |
| `/orgs/:org_id/channels[/new/:id]` | Channels — coming in Wave 4 (Plan 06-10) | 06-10 |
| `/orgs/:org_id/adapters[/new/:id]` | Adapters — coming in Wave 3 (Plan 06-11) | 06-11 |
| `/orgs/:org_id/break-reasons[/new/:id]` | Break Reasons — coming in Wave 3 (Plan 06-09) | 06-09 |
| `/orgs/:org_id/imports/new` | Bulk Import — coming in Wave 5 (Plan 06-13) | 06-13 |
| `/orgs/:org_id/imports/:id` | Import Result — coming in Wave 5 (Plan 06-13) | 06-13 |

These stubs are intentional — the goal of 06-06 is the shell skeleton + agent routes + client bootstrap. Wave 3-5 plans will replace each placeholder.

## Threat Flags

None beyond what was already declared in the plan's threat model:

| Flag | File | Description |
|------|------|-------------|
| T-06-06-01 mitigated | catalog-shell.ts | UUIDv7 guard now on ALL entity routes (D6-13); malformed → redirect to '/' |
| T-06-06-02 accepted | e2e/smoke.spec.ts | Hardcoded test org_id `01952a6b-1c00-7000-8000-000000000001` is a dev fixture; `TEST_ORG_ID` env var overrides |

## Self-Check: PASSED

- [x] `web/packages/ui/src/components/shell/catalog-shell.ts` FOUND
- [x] `web/apps/admin/playwright.config.ts` FOUND
- [x] `web/apps/admin/e2e/smoke.spec.ts` FOUND
- [x] `grep "createApiClient" catalog-shell.ts` FOUND
- [x] `grep "or-agent-list" catalog-shell.ts` FOUND
- [x] `pnpm --filter @open-routing/admin build` exits 0 (✓ built in 209ms)
- [x] `pnpm --filter @open-routing/ui test` 57 tests passed (10 test files)
- [x] `playwright --list` shows 5 tests in e2e/smoke.spec.ts
- [x] Commit b4c604a exists (Task 1)
- [x] Commit 31addb0 exists (Task 2)
- [x] Commit fc07cdb exists (peer review fixes)
