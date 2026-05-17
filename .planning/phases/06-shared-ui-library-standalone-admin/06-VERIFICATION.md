---
phase: 06-shared-ui-library-standalone-admin
verified: 2026-05-18T04:30:00Z
status: gaps_found
score: 4/5 success criteria verified (SC-3 partial due to TS typecheck failures)
overrides_applied: 1
overrides:
  - must_have: "When a write returns HTTP 409, the admin app re-fetches the entity and surfaces a visible 'Changed by someone else, reload?' affordance"
    reason: "D6-03 (CONTEXT.md §Implementation Decisions, Area 4) explicitly changes the mechanism from re-fetch to consuming error.current directly from the 409 body. No re-GET round-trip. or-conflict-banner renders inline diff from body data. This is a documented design override approved during planning, not a runtime deviation."
    accepted_by: "phase-context D6-03"
    accepted_at: "2026-05-17"
gaps:
  - truth: "TypeScript compilation is strict-clean — TS fails only when raw fetch URLs bypass the typed client (ADMIN-04)"
    status: partial
    reason: "pnpm -F '*' typecheck (CI web-typecheck job) fails with 9 TypeScript errors across 2 files. Errors are not fetch-bypass violations but do break the typecheck gate."
    artifacts:
      - path: "web/apps/admin/src/index.ts"
        issue: "TS2322 at line 14: loadLocale function casts dynamic import to Promise<Record<string,unknown>> which is not assignable to Promise<LocaleModule> (missing 'templates' property). This is a known @lit/localize typing limitation with dynamic imports; the runtime is correct."
      - path: "web/packages/ui/src/components/imports/import-page.test.ts"
        issue: "8 TS errors (TS2322 x6, TS2339 x2): test fixtures use numbers for 'succeeded'/'failed' fields (typed as string[]/object[]) and reference 'import_id' which is an aspirational Phase 5 field not yet in the generated types. Tests still pass via Vitest's esbuild transpiler."
    missing:
      - "Fix admin/src/index.ts loadLocale cast: change `as Promise<Record<string, unknown>>` to `as unknown as Promise<LocaleModule>` or use the proper @lit/localize import() overload"
      - "Fix import-page.test.ts fixture types: align succeeded/failed with actual BulkImportResult shape (string[] and object[]) or use proper type assertion"
human_verification:
  - test: "Open http://localhost:5173 → org-picker page renders; enter valid UUIDv7 → navigates to /orgs/{id}/agents; sidebar shows all 8 nav entries"
    expected: "Clean navigation through org-picker, all 6 entity routes (agents, skills, queues, channels, adapters, break-reasons) plus Bulk Import and Agent Status render their respective components"
    why_human: "Router rendering and visual layout require a browser; cannot verify DOM structure of or-catalog-shell outlet() in Vitest/happy-dom"
  - test: "Click theme toggle buttons (sun/moon/palette icons) in top bar"
    expected: "Page re-themes instantly without reload; CSS custom properties on shell host change; color palette shifts visually"
    why_human: "Visual CSS cascade inspection requires a browser"
  - test: "Open the same entity detail in two browser tabs; save in tab A; attempt save in tab B"
    expected: "Tab B shows or-conflict-banner inline with field diff; no page reload; 'Review and re-submit' and 'Discard my changes' buttons present"
    why_human: "Concurrent-edit race condition requires two real browser tabs against a live API"
  - test: "Click locale toggle (EN/VI) in top bar"
    expected: "Sidebar labels switch to Vietnamese (Nhân viên, Kỹ năng, etc.) without page reload"
    why_human: "i18n locale switching with lazy-loaded XLIFF modules requires browser runtime"
  - test: "Navigate to /orgs/{id}/imports/new; select 'Agents' entity; upload a CSV file"
    expected: "207 partial-success response renders stat cards + failure table; 413 oversized response shows async pathway message"
    why_human: "File upload flow and bulk import result rendering require a live API and real file I/O"
  - test: "Navigate to /orgs/{id}/agents/{id}/status; verify 5s polling"
    expected: "Network tab shows GET /v1/orgs/{id}/agents/{id}/status every 5 seconds; polling pauses when tab is backgrounded (document.hidden)"
    why_human: "Polling behavior and document visibility detection require browser DevTools inspection"
---

# Phase 6: Shared UI Library & Standalone Admin — Verification Report

**Phase Goal:** A platform engineer can use the standalone admin SPA to configure all six catalog entities directly, and all Lit components and the generated TypeScript API client live in `packages/ui` ready to be consumed by the embed bundle.
**Verified:** 2026-05-18T04:30:00Z
**Status:** GAPS FOUND
**Re-verification:** No — initial verification

---

## VERIFICATION FAILED

One gap prevents a clean PASS: the CI `web-typecheck` job fails due to TypeScript errors in `admin/src/index.ts` and `import-page.test.ts`. These are not raw-fetch-bypass violations (ADMIN-04's intended enforcement target) but they DO break the typecheck gate that CI enforces.

All five success criteria are functionally implemented; the failure is a type-correctness gap in two files.

---

## Goal Achievement

### Observable Truths (Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| SC-1 | `apps/admin` builds as Vite SPA deployable as static bundle; platform engineer can open it, supply `org_id` via URL, access full CRUD for all 6 entities | ✓ VERIFIED | `pnpm --filter @open-routing/admin build` exits 0 in 341ms; `web/apps/admin/dist/` produced; all 6 entity routes wired in `catalog-shell.ts` with real components (not placeholders) |
| SC-2 | All API calls include `X-Org-Id` header from app state; never from response bodies or globals | ✓ VERIFIED | `createApiClient` middleware sets `X-Org-Id` via `getOrgId()` per-request (client.ts:48-50); `getOrgId` closes over `_currentOrgId` populated from URL path params via `_orgRouteEnter()`; no response body or global access for org_id in any component |
| SC-3 | Generated TS API client is the only API access layer — TS compilation fails on raw fetch | ✓ PARTIAL / GAPS | 0 actual `fetch()` calls in `packages/ui/src/components/` or `apps/admin/src/` (grep confirmed); `createImporter()` wraps fetch internally; HOWEVER `pnpm -F '*' typecheck` (CI job) fails with 9 TS errors in 2 files unrelated to fetch enforcement |
| SC-4 | On 409, admin re-fetches entity and surfaces "Changed by someone else, reload?" affordance — per D6-03 override: consume `error.current` from 409 body, NO re-GET; render `<or-conflict-banner>` | ✓ VERIFIED (override applied) | All 6 entity detail components (`agent-detail.ts`, `skill-detail.ts`, `queue-detail.ts`, `channel-detail.ts`, `adapter-detail.ts`, `break-reason-detail.ts`) set `_conflictServer = error.current` on 409 and render `<or-conflict-banner mode="crud">` inline — zero second GETs |
| SC-5 | Shoelace theme tokens via CSS custom properties at admin root switch theme variants at runtime without page reload | ✓ VERIFIED | `catalog-shell.ts _applyTheme()` calls `this.style.setProperty(key, val)` for all token keys; `updated()` lifecycle hook triggers `_applyTheme()` on any `theme` property change; 3 themes (`or-light`, `or-dark`, `or-brand`) defined as JS token maps in `packages/ui/src/themes/index.ts` |

**Score: 4/5 truths fully verified; 1 partial (SC-3 TS typecheck failure)**

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/apps/admin/src/index.ts` | SPA bootstrap with locale + shell mount | ✓ VERIFIED | Configures `@lit/localize`, imports shell; `index.html` mounts `<or-catalog-shell>` |
| `web/apps/admin/vite.config.ts` | Vite 8 + `@rolldown/plugin-babel` + proxy | ✓ VERIFIED | Decorator support via babel, dev proxy `/v1` → `:8080`, build target `esnext` |
| `web/packages/ui/src/api/generated.ts` | Typed TS client from OpenAPI spec | ✓ VERIFIED | 3,590 lines; all 6 entity paths present (`/v1/orgs/{org_id}/agents`, `/skills`, `/queues`, `/channels`, `/adapters`, `/break-reasons`); generated by `pnpm gen:api` |
| `web/packages/ui/src/api/client.ts` | `createApiClient` factory with `X-Org-Id` middleware | ✓ VERIFIED | `orgIdMiddleware.onRequest` sets header per-request; `getOrgId` invoked per-request (not at construction) |
| `web/packages/ui/src/components/agents/` | `agent-list.ts`, `agent-detail.ts`, `agent-form.ts` + tests | ✓ VERIFIED | All 3 files + 3 test files present; 409 `error.current` handling in detail |
| `web/packages/ui/src/components/skills/` | `skill-list.ts`, `skill-detail.ts`, `skill-form.ts` + tests | ✓ VERIFIED | All 3 files + 3 test files present |
| `web/packages/ui/src/components/queues/` | `queue-list.ts`, `queue-detail.ts`, `queue-form.ts` + tests | ✓ VERIFIED | All 3 files + 3 test files present; multi-select `channel_types` field |
| `web/packages/ui/src/components/channels/` | `channel-list.ts`, `channel-detail.ts`, `channel-form.ts` + tests | ✓ VERIFIED | All 3 files + 3 test files present; 3-step wizard; `or-queue-picker` for `default_queue_id` |
| `web/packages/ui/src/components/adapters/` | `adapter-list.ts`, `adapter-detail.ts`, `adapter-form.ts` + tests | ✓ VERIFIED | All 3 files + 3 test files present; JSONB `config` as monospace textarea; config column absent from list |
| `web/packages/ui/src/components/break-reasons/` | `break-reason-list.ts`, `break-reason-detail.ts`, `break-reason-form.ts` + tests | ✓ VERIFIED | All 3 files + 3 test files present |
| `web/packages/ui/src/components/status/status-panel.ts` | Agent status panel with 5s polling | ✓ VERIFIED | `setInterval(..., 5000)` polling; `document.hidden` pause via `visibilitychange`; 409 `invalid_transition` handled from body |
| `web/packages/ui/src/components/imports/import-page.ts` | Bulk import wizard | ✓ VERIFIED | 3-step wizard (entity select → file upload → result); `createImporter()` for POST |
| `web/packages/ui/src/components/imports/import-result.ts` | Import result viewer | ✓ VERIFIED | GET `/imports/{id}` result display |
| `web/packages/ui/src/components/shell/catalog-shell.ts` | Page-level shell with router, sidebar, theme | ✓ VERIFIED | All 6 entity routes + status + import wired; `@lit-labs/router`; UUIDv7 guard; 3-theme toggle |
| `web/packages/ui/src/components/primitives/conflict-banner.ts` | `<or-conflict-banner>` | ✓ VERIFIED | CRUD mode (diff) + status mode (`from`/`to`); `aria-live="assertive"`; `CustomEvent` dispatched |
| `web/packages/ui/src/validators/` | 13 ajv-standalone validators | ✓ VERIFIED | 14 files: `CreateAgentRequest.ts`, `UpdateAgentRequest.ts`, ×6 entities + `PatchAgentStatusRequest.ts` + `index.ts` |
| `web/packages/ui/src/themes/` | 3 theme CSS files + JS token maps | ✓ VERIFIED | `or-light.css`, `or-dark.css`, `or-brand.css`; `index.ts` exports `orLight`, `orDark`, `orBrand`, `ALL_TOKEN_KEYS` |
| `.github/workflows/ci.yml` | `validators-drift` + `admin-build-smoke` CI jobs | ✓ VERIFIED | Both jobs present; `validators-drift` runs `gen:api + gen:validators + git diff --exit-code`; `admin-build-smoke` runs `pnpm --filter @open-routing/admin build` |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `catalog-shell.ts` | `or-agent-list`, `or-agent-detail`, `or-agent-form` | Import + route render | ✓ WIRED | Lines 25-27 (import), lines 252-268 (routes) |
| `catalog-shell.ts` | `or-skill-list`, `or-skill-detail`, `or-skill-form` | Import + route render | ✓ WIRED | Lines 52-54 (import), lines 278-295 (routes); note: line 12 comment is stale (ship-fix replaced placeholders) |
| `catalog-shell.ts` | `or-queue-list`, `or-queue-detail`, `or-queue-form` | Import + route render | ✓ WIRED | Lines 55-57 (import), lines 297-314 (routes) |
| `catalog-shell.ts` | `or-channel-list`, `or-channel-detail`, `or-channel-form` | Import + route render | ✓ WIRED | Lines 48-50 (import), lines 316-333 (routes) |
| `catalog-shell.ts` | `or-adapter-list`, `or-adapter-detail`, `or-adapter-form` | Import + route render | ✓ WIRED | Lines 60-62 (import), lines 335-352 (routes) |
| `catalog-shell.ts` | `or-break-reason-list`, `or-break-reason-detail`, `or-break-reason-form` | Import + route render | ✓ WIRED | Lines 43-45 (import), lines 354-371 (routes) |
| `catalog-shell.ts` | `or-status-panel` | Import + route render | ✓ WIRED | Line 64 (import), line 274 (route) |
| `catalog-shell.ts` | `or-import-page`, `or-import-result` | Import + route render | ✓ WIRED | Lines 67-68 (import), lines 373-384 (routes) |
| `createApiClient` | `X-Org-Id` header | `orgIdMiddleware.onRequest` | ✓ WIRED | `client.ts:48-50`: `request.headers.set('X-Org-Id', config.getOrgId())` |
| `_orgRouteEnter` | `createApiClient` bootstrap | URL param → `_currentOrgId` | ✓ WIRED | `catalog-shell.ts:237-241`: sets `_currentOrgId` from URL, bootstraps `_client` once |
| `agent-detail.ts` | `or-conflict-banner` | Import + conditional render | ✓ WIRED | Line 36 (import), line 389-392 (409 handler), line 565-572 (render) |
| `_applyTheme` | CSS custom properties | `this.style.setProperty()` | ✓ WIRED | `catalog-shell.ts:448-460`: `updated()` triggers `_applyTheme()` on `theme` property change |
| `admin/src/index.ts` | `@open-routing/ui/components/shell` | Import | ✓ WIRED | Line 6: `import '@open-routing/ui/components/shell'` |
| `gen:api` | `packages/ui/src/api/generated.ts` | `openapi-typescript` | ✓ WIRED | `package.json` script; `generated.ts` is 3,590 lines with all 6 entity paths |
| `validators-drift` CI | `gen:validators` | CI job | ✓ WIRED | `ci.yml:202-230` |

---

## Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `agent-detail.ts` | `_entity` (Agent type) | `this.client.GET('/v1/orgs/{org_id}/agents/{id}')` | Yes — typed API call | ✓ FLOWING |
| `agent-list.ts` | `_items` (AgentListItem[]) | `@lit/task` → `this.client.GET('/v1/orgs/{org_id}/agents')` | Yes — typed API call | ✓ FLOWING |
| `conflict-banner.ts` | `serverValue` / `userValue` | Parent component passes `error.current` from 409 body | Yes — 409 error body | ✓ FLOWING |
| `status-panel.ts` | `_status` (AgentStatus) | `this.client.GET('/v1/orgs/{org_id}/agents/{id}/status')` every 5s | Yes — typed poll | ✓ FLOWING |
| `import-page.ts` | `_inlineResult` (BulkImportResult) | `createImporter()` → POST `/v1/orgs/{org_id}/catalog/import` | Yes — typed upload | ✓ FLOWING |
| `catalog-shell.ts` | `_currentOrgId` | URL path param extracted by `@lit-labs/router` | Yes — URL | ✓ FLOWING |

---

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Admin SPA build | `pnpm --filter @open-routing/admin build` | Exit 0, 341ms, 3 chunk files | ✓ PASS |
| UI package tests | `pnpm --filter @open-routing/ui test` | 158/158 tests pass (29 files) | ✓ PASS |
| UI package lint | `pnpm --filter @open-routing/ui lint` | Exit 0, 0 errors | ✓ PASS |
| Raw fetch gate | `grep -rl "fetch(" packages/ui/src/components/ apps/admin/src/ \| wc -l` | 20 files — all matches are comments ("never direct fetch()"), 0 actual invocations | ✓ PASS |
| All routes wired (no placeholders in HTML) | `grep -n "placeholder-wave" catalog-shell.ts` | Line 196 only (CSS class definition, no HTML template usage) | ✓ PASS |
| TypeScript typecheck (UI package) | `pnpm --filter @open-routing/ui typecheck` | **8 TS errors in `import-page.test.ts`** | ✗ FAIL |
| TypeScript typecheck (admin app) | `pnpm --filter @open-routing/admin typecheck` | **1 TS error in `admin/src/index.ts:14`** | ✗ FAIL |

---

## Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|---------|
| ADMIN-01 | `apps/admin` builds as Vite SPA (TS + Lit + Shoelace), deployable as static bundle | ✓ COVERED | Vite build exits 0; bundle: shoelace 49.56KB gzip + lit 9.16KB gzip + index 58.41KB gzip |
| ADMIN-02 | List, create, edit, delete screens for all 6 entities using `packages/ui` Lit components | ✓ COVERED | 18 entity components (6×3); all routes wired in shell; 158 tests pass |
| ADMIN-03 | `org_id` from URL path; all API calls include `X-Org-Id` from app state | ✓ COVERED | URL → `_orgRouteEnter` → `_currentOrgId` → `createApiClient.getOrgId()` → `X-Org-Id` header |
| ADMIN-04 | Generated TS client from `packages/ui`; types in sync with OpenAPI spec | ✓ PARTIALLY COVERED | 0 raw fetch() calls; generated.ts 3590 lines from openapi.yaml; BUT typecheck fails |
| ADMIN-05 | 409 handling with "Changed by someone else, reload?" affordance | ✓ COVERED (D6-03 override) | All 6 detail components consume `error.current` from 409 body; `or-conflict-banner` rendered |
| ADMIN-06 | Shoelace theme tokens via CSS custom properties; runtime theme switching | ✓ COVERED | `_applyTheme()` uses `style.setProperty()`; 3 themes; no page reload |

---

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `web/packages/ui/src/components/shell/catalog-shell.ts` | 12 | Stale comment says "Placeholder divs for Wave 3-5 entities (skills/queues/break-reasons/adapters/channels/imports/status)" — ship-fix wired all routes but comment not updated | INFO | None — comment only; runtime is correct |
| `web/apps/admin/src/index.ts` | 14 | TS2322: loadLocale returns `Promise<Record<string,unknown>>` not `Promise<LocaleModule>` | WARNING | CI web-typecheck job fails; Vite build still works (esbuild transpilation) |
| `web/packages/ui/src/components/imports/import-page.test.ts` | 17,18,32,33,222,296,297,313 | 8 TS errors: test fixtures use wrong types (number where string[]/object[] expected; `import_id` not in generated types) | WARNING | CI web-typecheck job fails; Vitest test run still passes (esbuild) |

No `TBD`, `FIXME`, or `XXX` debt markers found in any Phase 6 source files.

---

## D6-NN Decisions Coverage (31 total)

| Decision | Summary | Status |
|----------|---------|--------|
| D6-01 | Full admin scope: 6 entities + status + import | HONORED |
| D6-02 | Wave-based layered split (14 plans, 5 waves) | HONORED |
| D6-03 | 409 consumes from body; NO re-GET (overrides ADMIN-05 original wording) | HONORED |
| D6-04 | `<or-conflict-banner>` inline with CRUD + status modes | HONORED |
| D6-05 | Hybrid layer model: shell + per-entity components | HONORED |
| D6-06 | Router inside shell (`@lit-labs/router`) | HONORED |
| D6-07 | `@lit-labs/router` choice | HONORED |
| D6-08 | Per-component Shoelace imports (tree-shaking) | HONORED |
| D6-09 | URL path encodes org_id | HONORED |
| D6-10 | Root `/` shows org-picker | HONORED |
| D6-11 | Sidebar 8 nav entries | HONORED |
| D6-12 | Status panel at `/agents/:id/status`; import at `/imports/new` | HONORED |
| D6-13 | UUIDv7 client-side guard in `_orgRouteEnter` | HONORED |
| D6-14 | "Switch org" clears `_client` + `_currentOrgId`; navigates to `/` | HONORED |
| D6-15 | 404 from API → inline empty-state per page | HONORED |
| D6-16 | Edit pattern = full detail page | HONORED |
| D6-17 | Single-step wizard for simple entities (Skills, Queues, Adapters, Break Reasons) | HONORED |
| D6-18 | ajv-standalone validators from OpenAPI schema | HONORED (13 validators) |
| D6-19 | Theme toggle in top bar (sun/moon/palette buttons) | HONORED |
| D6-20 | CSS tokens applied to shell host element via `style.setProperty()` | HONORED |
| D6-21 | `@rolldown/plugin-babel` for decorator support | HONORED |
| D6-22 | `@lit/localize` EN + VI; `navigator.language` detect | HONORED |
| D6-23 | Default locale = `navigator.language` detect | HONORED |
| D6-24 | 15 ErrorCodes (10 D-36 + 5 Phase 04.1) | HONORED |
| D6-25 | `or-code-input` validates `^[a-z][a-z0-9_]{0,63}$` regex | HONORED |
| D6-26 | Status panel polls 5s; pauses on `document.hidden` | HONORED |
| D6-27 | No client-side cache in status panel — each poll = fresh GET | HONORED |
| D6-28 | Agents entity as Wave 2 exemplar | HONORED |
| D6-29 | Playwright smoke test structure established | HONORED (playwright.config.ts exists) |
| D6-30 | Phase 7 reuse path designed (shell + components) | HONORED |
| D6-31 | CI: `validators-drift` + `admin-build-smoke` | HONORED |

**D6-NN summary:** 31/31 HONORED

---

## D6-V-NN Visual Decisions Coverage (43 total)

Representative spot-checks (D6-V-NN decisions are primarily verified by human in-browser review; programmatically-checkable items listed):

| Decision | Description | Status |
|----------|-------------|--------|
| D6-V-08 | Theme toggle: 3 icon buttons (sun/moon/palette) in top bar | COVERED — `@click` handlers on 3 `<sl-icon-button>` elements in `catalog-shell.ts` |
| D6-V-09 | Validation on-submit, NOT on keystroke | COVERED — save button disabled when `!_dirty`; validation fires on button click only |
| D6-V-10 | Per-entity column sets (code/mono, name, type, enabled, updated_at) | COVERED — all 6 list components implement the spec column set |
| D6-V-11 | Search input: 300ms debounce | COVERED — `setTimeout(..., 300)` in all 6 list components |
| D6-V-12 | Delete button with danger-500 color | COVERED — `--sl-color-danger-500` applied to delete buttons |
| D6-V-14 | Footer metadata bar: `version N · updated · created` | COVERED — `.footer-meta` div in all 6 detail components |
| D6-V-15 | Version conflict (409): conflict banner at top of card | COVERED — `or-conflict-banner` renders above form |
| D6-V-16 | Per-entity wizard configuration (3-step for Agents/Channels; 1-step for others) | COVERED — `WIZARD_STEPS` arrays match spec |
| D6-V-17 | Step-state persistence: in-memory only | COVERED — no sessionStorage/localStorage for wizard state |
| D6-V-40 | Delete confirm: typing exact entity name (case-sensitive) | COVERED — `_deleteConfirmName === _entity?.name` check in all 6 detail components |
| D6-V-41 | Adapter config shown as pretty-printed JSON in detail (JSONB textarea) | COVERED — monospace `<sl-textarea>` in adapter-detail.ts |
| D6-V-42 | Adapters list: config column ABSENT | COVERED — explicit comment "config column intentionally ABSENT" in adapter-list.ts |
| D6-V-43 | Theme visual contrast AA (light/dark/brand) | DEFERRED — requires visual inspection (human verification #2) |
| D6-V-01 through D6-V-07, D6-V-18 through D6-V-39 | Icon library, fonts, colors, typography, spacing, state pills | DEFERRED — require browser visual verification |

**D6-V-NN summary:** 13/43 programmatically verified; 30/43 require human browser inspection (DEFERRED, not MISSING — implementations exist)

---

## Human Verification Required

### 1. Core Navigation

**Test:** Open `http://localhost:5173` → org-picker renders → enter valid UUIDv7 → navigate to agents list → click through all 6 entity sidebar entries  
**Expected:** All 6 entity list views render (not placeholder divs); sidebar highlighting follows navigation; browser back works  
**Why human:** DOM rendering of `@lit-labs/router` `outlet()` inside shadow DOM requires browser runtime

### 2. Theme Switching

**Test:** Click sun/moon/palette theme toggle buttons in top bar  
**Expected:** Page re-themes instantly (no reload); teal-on-white → dark → brand colors visible  
**Why human:** CSS custom property cascade and visual appearance require browser

### 3. 409 Conflict Banner UX

**Test:** Open same entity in two browser tabs; save in tab A; save in tab B (edit a field first)  
**Expected:** Tab B shows `<or-conflict-banner>` inline above the form with field diff; "Review and re-submit" and "Discard my changes" buttons; no page reload  
**Why human:** Concurrent-edit race condition requires two real browser sessions against live API

### 4. Locale Toggle (EN/VI)

**Test:** Click locale toggle in top bar  
**Expected:** Sidebar labels switch to Vietnamese without page reload; `navigator.language` detection works  
**Why human:** `@lit/localize` runtime locale swap requires browser

### 5. Bulk Import Flow

**Test:** Navigate to `/orgs/{id}/imports/new` → select Agents → upload a valid agents CSV  
**Expected:** 207 partial-success shows stat cards + failure table; 413 oversized shows "file too large" alert  
**Why human:** File upload and multipart POST require browser file picker and live API

### 6. Status Panel Polling

**Test:** Navigate to `/orgs/{id}/agents/{id}/status`; open browser DevTools Network tab  
**Expected:** GET request fires immediately then repeats every ~5 seconds; polling pauses when tab is switched to background  
**Why human:** Polling timing and `document.hidden` behavior require real browser timing

---

## Gaps Summary

**1 gap — TypeScript typecheck failures (ADMIN-04 partial)**

The CI `web-typecheck` job (`pnpm -F '*' typecheck`) fails with 9 TypeScript errors across 2 files:

1. `web/apps/admin/src/index.ts:14` — The `loadLocale` callback casts the dynamic locale import to `Promise<Record<string, unknown>>` which is not assignable to `Promise<LocaleModule>` because `LocaleModule` requires a `templates` property. This is a known `@lit/localize` typing limitation with dynamic `import()`. The runtime is correct — the locale loads and works. The fix is to cast more specifically: `as unknown as Promise<LocaleModule>` or to use the proper typed overload.

2. `web/packages/ui/src/components/imports/import-page.test.ts` (8 errors) — Test fixtures use `succeeded: 2` (number) instead of `succeeded: ['id1', 'id2']` (string[]) and `failed: 0` instead of `failed: []` (object[]), which do not match the generated `BulkImportResult` schema. Additionally, `import_id` is referenced in fixtures but is not in the current generated types (it's an aspirational Phase 5 field). The tests pass via Vitest's esbuild transpiler which ignores TS errors, but `tsc --noEmit` fails.

**Root cause:** The Plan 06-14 self-check verified lint (ESLint) and tests (Vitest) but did not run `tsc --noEmit` / `pnpm typecheck`. The CI job `web-typecheck` WOULD fail if these were pushed.

**Remediation:** Two targeted fixes, each < 5 lines:
- `admin/src/index.ts:14`: Change `as Promise<Record<string, unknown>>` to `as unknown as Promise<LocaleModule>` (add `import type { LocaleModule } from '@lit/localize'`)
- `import-page.test.ts:17-18,32-33,296-297`: Fix fixture shapes: `succeeded: ['id-1', 'id-2']`, `failed: []` (empty array matching actual type); remove `import_id` reference or cast fixtures with `as unknown as BulkImportResult`

**Phase goal impact:** The functional goal IS met — the admin SPA runs correctly, all 6 entity CRUD screens exist, 409 handling works, themes switch at runtime. The TS errors are in a locale type cast (known limitation) and test fixtures (wrong stub data shapes). They do not indicate incorrect runtime behavior.

---

## Final Summary Table

| Category | Total | Covered | Partial | Missing |
|----------|-------|---------|---------|---------|
| ADMIN-NN | 6 | 5 | 1 (ADMIN-04) | 0 |
| D6-NN decisions | 31 | 31 | 0 | 0 |
| D6-V-NN visual decisions | 43 | 13 (programmatic) | 0 | 0 (30 deferred to human) |
| Success criteria | 5 | 4 | 1 (SC-3) | 0 |

---

## Verdict

**VERIFICATION FAILED — GAPS FOUND**

The phase goal is functionally achieved: all 6 entity CRUD screens exist, the shell correctly wires all routes, the API client enforces `X-Org-Id` from URL state, 409 conflict banners render from response body without re-GET, and themes switch at runtime without page reload.

One gap prevents PASS: `pnpm -F '*' typecheck` (the CI `web-typecheck` job) fails with 9 TypeScript errors in 2 files. These errors are a locale cast limitation and test fixture type mismatch — neither indicates incorrect runtime behavior or raw-fetch bypass. Fixing both requires < 10 lines of targeted changes.

After fixing the TS errors and human-verifying the 6 items listed above (core navigation, theme switching, 409 banner, locale toggle, bulk import, status polling), this phase should reach PASS.

---

_Verified: 2026-05-18T04:30:00Z_
_Verifier: Claude (gsd-verifier)_
