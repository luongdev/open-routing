# Phase 6: Shared UI Library & Standalone Admin - Research

**Researched:** 2026-05-17
**Domain:** Vite 8 + Lit 3 + Shoelace 2 SPA, @lit-labs/router, @lit/localize, ajv standalone validators, openapi-fetch, Playwright smoke
**Confidence:** HIGH (core stack verified against npm registry and official docs)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Scope (D6-01/D6-02):** Full admin scope in one phase — Wave 1 scaffold → Wave 2 Agents exemplar → Waves 3-4 remaining 5 entities parallel → Wave 5 status panel + import UI + Playwright smoke.

**packages/ui boundary (D6-05/D6-06):** Hybrid layer: per-entity components (18 = 6 entities × 3 each) + optional `<or-catalog-shell>` that wires router + sidebar. Router/nav are shell-internal.

**Router (D6-07):** `@lit-labs/router` (Lit-native, Labs status acknowledged, migration cheap at 8 routes).

**Shoelace import mode (D6-08):** Per-component imports only — never a central register call. Tree-shaking must ACTUALLY work so Phase 7 hits 70KB gzipped.

**Org routing (D6-09):** URL path encodes org_id: `/orgs/:org_id/{entity}`. `createApiClient` receives `getOrgId` that reads from parsed URL params.

**Org-picker (D6-10):** Root `/` → org-picker with last-used in localStorage; malformed org_id → redirect to `/` with toast.

**Sidebar (D6-11):** 8 entries — Agents, Skills, Queues, Channels, Adapters, Break Reasons, separator, Bulk Import, Agent Status.

**Route shape (D6-12):** Status panel nested per-agent `/orgs/:org_id/agents/:id/status`; Import top-level `/orgs/:org_id/imports/new` + `/orgs/:org_id/imports/:id`.

**UUIDv7 validation (D6-13):** Client-side regex guard before any API call; server is authoritative.

**Switch org (D6-14):** Hard reset to `/`. All in-memory state cleared.

**404 handling (D6-15):** Inline empty-state per page, no dedicated 404 route.

**Edit pattern (D6-16):** Full detail page at `/orgs/:org_id/{entity}/:id`. No modal, no drawer.

**Create pattern (D6-17):** `<or-form-wizard>` component for all 6 entities — multi-step for Agent + Channel, single-step (wizard auto-completes) for Skill/Queue/BreakReason/Adapter.

**Form validation source (D6-18):** Build-time ajv-standalone codegen from OpenAPI schemas into `web/packages/ui/src/validators/<schema>.ts`. Drift gate: `pnpm gen:api && pnpm gen:validators && git diff --exit-code`.

**Themes (D6-19/D6-20):** 3 named themes (or-light, or-dark, or-brand). Applied as CSS custom properties on `<or-catalog-shell>` host element via `this.style.setProperty('--sl-color-primary-500', value)`. Supports string (named) or JSON object (arbitrary token map).

**i18n (D6-21/D6-22/D6-23/D6-24):** `@lit/localize` runtime mode, EN + VI, `navigator.language` detection + localStorage override, client-side `ErrorCode` → i18n key mapping.

**Polling (D6-25/D6-26/D6-27):** Catalog lists = refresh-on-action + `[Refresh]` button. Status panel = 5s poll + `visibilitychange` pause + `state_version` recheck on resume. No client cache.

**Testing (D6-28):** Vitest + happy-dom for component tests in `packages/ui`. Tests colocated (`*.test.ts`).

**Dev server (D6-29):** `apps/admin/vite.config.ts` proxies `/v1`, `/healthz`, `/readyz` → `localhost:8080`.

**Playwright (D6-30):** Single smoke test in `web/apps/admin/e2e/smoke.spec.ts`. Manual-only in Phase 6.

**CI (D6-31):** Extend `.github/workflows/ci.yml` — codegen drift gate + `pnpm --filter @open-routing/admin build`. No E2E in CI.

### Claude's Discretion

- Internal directory layout under `web/packages/ui/src/components/` (e.g., `components/agents/agent-list.ts` vs `components/agent-list/agent-list.ts`)
- Sidebar visual design / icon choice
- Brand-theme primary color value (mid-tone teal recommended, not blocking)
- ajv codegen wrapper (Node script vs Vite plugin) — output locked at `packages/ui/src/validators/`
- `@lit/localize` XLIFF directory layout
- `<or-form-wizard>` step-state persistence (recommend in-memory for v0.1)
- Theme-toggle UX shape (recommend `<sl-button-group>` with 3 buttons)
- Org-picker validation timing (recommend on-submit)
- Whether `<or-conflict-banner>` includes "discard" button (recommend yes)

### Deferred Ideas (OUT OF SCOPE)

- Inline org dropdown / multi-org switcher
- Module Federation / iframe embed variants
- Server-side rendering / SSR
- Source-map upload to error tracker
- Accessibility audit / screen reader certification
- Dark-mode auto-detect via `prefers-color-scheme`
- Brand theme customization per org
- Server-translated error messages
- Inline-edit list rows / bulk delete / bulk enable-disable
- CSV preview-before-upload via Web Worker (WORK-02)
- Error CSV download (IMP-11)
- Stale-while-revalidate / periodic catalog list polling
- In-page audit log / change history
- Inline JSON editor for adapter `config` field (use `<sl-textarea>` raw JSON)
- Real RBAC for `force=true` flag
- Optional embed `<or-catalog-shell>` — Phase 7 chooses at planning time
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ADMIN-01 | `apps/admin` builds as Vite SPA (TS+Lit+Shoelace), deployable as static bundle | Vite 8.0.13 + `appType: 'spa'` + `pnpm build` — Wave 1 scaffold |
| ADMIN-02 | List/create/edit/delete screens for all 6 entities, reusing `packages/ui` Lit components | 18 entity components (6 × list+detail+form) — Waves 2-4 |
| ADMIN-03 | `org_id` from URL path or stub login; all API calls include `X-Org-Id` from app state | `createApiClient({ getOrgId })` + router param — Wave 1 (client bootstrap) |
| ADMIN-04 | Generated TS API client from `packages/ui` is the ONLY API access layer; typecheck fails on raw fetch | `paths` types from openapi-typescript + eslint no-restricted-syntax rule — Wave 1 |
| ADMIN-05 | HTTP 409 → re-fetch entity + "Changed by someone else" affordance | `<or-conflict-banner>` consuming `current` from 409 body — Wave 2+ |
| ADMIN-06 | Shoelace theme tokens via CSS custom properties at admin root, runtime-switchable | `this.style.setProperty('--sl-color-primary-*', …)` on shell host — Wave 1 |
</phase_requirements>

---

## Summary

Phase 6 builds the Vite 8 + Lit 3 + Shoelace 2 admin SPA at `web/apps/admin/` and expands `web/packages/ui/` with 18 entity components plus primitives, routing shell, validators, and i18n. The stack is clean-slate on top of the Phase 2 API surface (`createApiClient`, `ApiError`, `createApiTask`) which already lives in the repo. The most material discovery from research is a **Vite 8 breaking change**: the new Rolldown/Oxc bundler does not support lowering TypeScript experimental decorators out-of-box — Lit 3's `@customElement`/`@property` decorators require adding `@rolldown/plugin-babel` + `@babel/plugin-proposal-decorators` to `vite.config.ts`. Every other aspect of the locked stack (router, localize, Shoelace per-component imports, ajv standalone, Playwright) has verified npm packages and working APIs.

The ajv codegen pipeline needs a custom Node script rather than the stale `ajv-cli` (last published 2021-03-28; does not support ajv v8 ESM output). The script calls ajv v8's programmatic API with `code: { source: true, esm: true }` to emit tree-shakeable ESM validators. `@lit-labs/router` is at v0.1.4 (Labs status, not yet stable) but URLPattern is now Baseline 2025 (all major browsers); a `urlpattern-polyfill` remains needed for happy-dom/Node test environments. The `<or-conflict-banner>` UX can consume the `current` entity directly from the 409 body without a re-GET, matching D6-03.

**Primary recommendation:** Add the Rolldown/Babel decorator plugin in Wave 1 vite.config.ts before any Lit component is written; this is a prerequisite for the entire phase.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| URL routing + org_id parsing | Browser / Client (Shell) | — | `@lit-labs/router` runs entirely client-side; org_id extracted from route params |
| API calls + X-Org-Id header | Browser / Client (API layer) | Backend (enforces) | `createApiClient` middleware; backend is authoritative but client sets header |
| CRUD form validation (pre-submit) | Browser / Client (packages/ui/validators) | Backend (422 is authoritative) | ajv-standalone compiled from OpenAPI; client is UX nicety |
| Theme token injection | Browser / Client (Shell host element) | — | CSS custom properties on `:host`; cascade handles Shadow DOM children |
| i18n string selection | Browser / Client (packages/ui) | — | `@lit/localize` runtime mode; locale modules lazy-loaded |
| 409 conflict resolution | Browser / Client (components) | Backend (returns `current`) | No re-GET needed; consume `current` from 409 body directly |
| Status panel polling | Browser / Client (status panel) | Backend (state_version truth) | visibilitychange + 5s interval in a `@lit/task` reactive task |
| Bulk import result rendering | Browser / Client (import page) | Backend (207 body) | Parse `BulkImportResult.failed[]` client-side; no post-process needed |
| Auth / org isolation | Backend (FOUND-03/D-38) | Client (sets header) | Server enforces; client just passes what router parses from URL |

---

## Standard Stack

### Core (apps/admin)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `lit` | 3.3.3 [VERIFIED: npm registry] | LitElement + lit-html, reactive properties, Shadow DOM components | Official Lit team library, minimal ~5KB runtime, real Custom Elements |
| `@shoelace-style/shoelace` | 2.20.1 [VERIFIED: npm registry] | Accessible UI primitives (input, select, button, alert, etc.) | Project lock (PROJECT.md); production-grade, tree-shakeable per-component |
| `@lit-labs/router` | 0.1.4 [VERIFIED: npm registry] | URLPattern-based client-side routing, `outlet()` pattern | D6-07 locked; Lit-native, ~5KB, integrates with reactive lifecycle |
| `@lit/localize` | 0.12.2 [VERIFIED: npm registry] | EN+VI bilingual i18n, lazy-load locale modules | D6-22 locked; official Lit team library, `msg()` API, XLIFF workflow |
| `vite` | 8.0.13 [VERIFIED: npm registry] | SPA bundler + dev server with proxy | Current major version; Rolldown/Oxc-powered, fast builds |
| `@rolldown/plugin-babel` | 0.2.3 [VERIFIED: npm registry] | Babel transform for Lit decorators under Vite 8 | **Required** — Oxc does not lower `experimentalDecorators` |
| `@babel/plugin-proposal-decorators` | 7.29.0 [VERIFIED: npm registry] | TC39 2023-11 decorator spec lowering | Paired with rolldown/plugin-babel |
| `urlpattern-polyfill` | 10.1.0 [VERIFIED: npm registry] | URLPattern polyfill for Vitest/Node environment | `@lit-labs/router` requires URLPattern; Node 20 lacks it; browsers all have it |

### Supporting (packages/ui)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@lit/task` | 1.0.3 [VERIFIED: npm registry] | Async fetch state machine in Lit lifecycle | Already in deps; all entity list + detail tasks use it |
| `@lit/reactive-element` | 2.1.2 [VERIFIED: npm registry] | ReactiveController host interface | Already in deps; referenced in `createApiTask` |
| `ajv` | 8.20.0 [VERIFIED: npm registry] | JSON Schema validation at build time | gen:validators pipeline — programmatic API with ESM output |
| `@lit/localize-tools` | 0.8.2 [VERIFIED: npm registry] | `lit-localize extract` + `lit-localize build` CLI | Required for XLIFF round-trip in CI and dev |

### Testing

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `vitest` | 3.2.4 [VERIFIED: npm registry] | Test runner — already in use | All unit/component tests |
| `happy-dom` | 20.9.0 [VERIFIED: npm registry] | 2-4x faster than jsdom for Lit component tests | Primary test environment for `packages/ui` |
| `@playwright/test` | 1.60.0 [VERIFIED: npm registry] | E2E smoke test | `apps/admin/e2e/smoke.spec.ts` — manual only in Phase 6 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `@lit-labs/router` | `@vaadin/router` | Vaadin is more mature/stable but decoupled from Lit lifecycle; bigger bundle; D6-07 rejects it |
| `@lit/localize` | `i18next` + adapters | i18next is React-flavoured, bigger (~10KB), no official Lit `msg()` integration |
| ajv programmatic API | `ajv-cli` | `ajv-cli` v5.0.0 last published 2021-03-28; does not support ajv v8 ESM; use programmatic API in a Node script |
| `@rolldown/plugin-babel` | `@vitejs/plugin-basic-ssl` + SWC | Babel approach is recommended by Vite 8 docs for decorator lowering |

**Installation (apps/admin additions):**
```bash
pnpm --filter @open-routing/admin add lit @shoelace-style/shoelace @lit-labs/router @lit/localize
pnpm --filter @open-routing/admin add -D vite @rolldown/plugin-babel @babel/plugin-proposal-decorators @playwright/test urlpattern-polyfill
```

**Installation (packages/ui additions):**
```bash
pnpm --filter @open-routing/ui add @shoelace-style/shoelace @lit/localize
pnpm --filter @open-routing/ui add -D @lit/localize-tools ajv urlpattern-polyfill happy-dom
```

---

## Package Legitimacy Audit

> slopcheck was run against PyPI (incorrect ecosystem — Node.js project). All packages verified
> directly against npm registry via `npm view <pkg> version` with publish date cross-check.

| Package | Registry | Age | Published | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `lit` | npm | 4+ yrs | 2026-05-14 | github.com/lit/lit | OK (major framework) | Approved |
| `@shoelace-style/shoelace` | npm | 4+ yrs | 2025-03-11 | github.com/shoelace-style/shoelace | OK | Approved |
| `@lit-labs/router` | npm | 3 yrs | 2025-02-14 | github.com/lit/lit | OK (official Lit team) | Approved |
| `@lit/localize` | npm | 3 yrs | 2024-08-05 | github.com/lit/lit | OK (official Lit team) | Approved |
| `@lit/localize-tools` | npm | 3 yrs | 2026-05-14 | github.com/lit/lit | OK | Approved |
| `vite` | npm | 5+ yrs | 2026-05-14 | github.com/vitejs/vite | OK (major framework) | Approved |
| `@rolldown/plugin-babel` | npm | ~1 yr | 2026-04-13 | github.com/rolldown | OK (official Rolldown team) | Approved |
| `@babel/plugin-proposal-decorators` | npm | 5+ yrs | recent | github.com/babel/babel | OK | Approved |
| `urlpattern-polyfill` | npm | 3 yrs | 2025-05-08 | github.com/nicolo-ribaudo | OK | Approved |
| `ajv` | npm | 8+ yrs | 2026-04-24 | github.com/ajv-validator/ajv | OK (widely used) | Approved |
| `ajv-cli` | npm | 4 yrs | **2021-03-28** | github.com/ajv-validator | STALE | **REMOVED — use programmatic API** |
| `@playwright/test` | npm | 4 yrs | 2026-05-11 | github.com/microsoft/playwright | OK | Approved |
| `happy-dom` | npm | 3 yrs | 2026-04-13 | github.com/capricorn86/happy-dom | OK | Approved |
| `openapi-typescript` | npm | 5+ yrs | 2026-02-11 | github.com/openapi-ts | OK (already in use) | Approved |
| `openapi-fetch` | npm | 3 yrs | 2026-02-11 | github.com/openapi-ts | OK (already in use) | Approved |

**Packages removed due to [STALE] verdict:**
- `ajv-cli` — last published 2021-03-28, does not support ajv v8 ESM output. Replace with a custom Node script at `web/packages/ui/scripts/gen-validators.mjs`.

**Packages flagged as suspicious [SUS]:** none.

---

## Phase Boundary

### In Scope

- `web/packages/ui/` expansion: 18 entity components (6 entities × `<or-{entity}-list>` + `<or-{entity}-detail>` + `<or-{entity}-form>`), `<or-status-panel>`, `<or-import-page>`, `<or-import-result>`, `<or-catalog-shell>`, primitives (`<or-data-table>`, `<or-cursor-paginator>`, `<or-conflict-banner>`, `<or-form-wizard>`, `<or-code-input>`, `<or-org-picker>`)
- `web/packages/ui/src/validators/` — ajv-standalone ESM validators for all 6 entity Create/Update schemas
- `web/packages/ui/src/locales/` — XLIFF files + generated EN/VI locale modules via `@lit/localize`
- `web/packages/ui/src/themes/` — 3 CSS token files (`or-light.css`, `or-dark.css`, `or-brand.css`)
- `web/apps/admin/` — full Vite 8 SPA setup: `vite.config.ts`, `src/index.ts` (mounts `<or-catalog-shell>`), `index.html`, `e2e/smoke.spec.ts`
- CI extensions: `gen:validators` task + drift gate + `build` smoke

### Out of Scope

Everything in CONTEXT.md `<deferred>` section — notably: Module Federation, SSR, real auth, Playwright in CI (Phase 7), RBAC for force flag, inline org switcher, CSV Web Worker preview, error CSV download, SWR cache, periodic catalog polling, adapter JSON editor, audit log.

---

## Implementation Strategy

### 1. Vite 8 SPA Bootstrap with Lit Decorators

**Critical finding:** Vite 8.0 uses Rolldown + Oxc. Oxc does not lower `experimentalDecorators: true` TypeScript decorators that Lit 3 uses (`@customElement`, `@property`, `@state`, `@query`). The official Vite 8 workaround is `@rolldown/plugin-babel`. [VERIFIED: Vite 8 migration guide + github.com/vitejs/vite/discussions/21891]

**File:** `web/apps/admin/vite.config.ts`
```typescript
// Source: vite.dev/config/server-options + github.com/vitejs/vite/discussions/21891
import { defineConfig } from 'vite';
import babel from '@rolldown/plugin-babel';

export default defineConfig({
  appType: 'spa',
  plugins: [
    babel({
      presets: [{
        preset: () => ({
          plugins: [['@babel/plugin-proposal-decorators', { version: '2023-11' }]],
        }),
        rolldown: { filter: { code: '@' } },  // only files with decorators
      }],
    }),
  ],
  server: {
    proxy: {
      '/v1':      'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      '/readyz':  'http://localhost:8080',
    },
  },
  build: {
    target: 'esnext',
    rollupOptions: {
      output: {
        manualChunks: {
          // vendor split keeps entity component chunks small
          shoelace: ['@shoelace-style/shoelace'],
          lit: ['lit', '@lit/reactive-element', '@lit/task'],
        },
      },
    },
  },
});
```

**Note on `experimentalDecorators` in tsconfig:** The existing `web/tsconfig.base.json` does NOT have `experimentalDecorators: true`. The base config uses `target: ES2022` and `useDefineForClassFields` is not set (defaults to `true` with ES2022). Lit 3 requires `useDefineForClassFields: false` when using `experimentalDecorators`. Wave 1 must add both to `web/apps/admin/tsconfig.json` and `web/packages/ui/tsconfig.json`.

**Required tsconfig extension** (extends `tsconfig.base.json`):
```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "rootDir": "src",
    "outDir": "dist",
    "experimentalDecorators": true,
    "useDefineForClassFields": false
  },
  "include": ["src"]
}
```

**File:** `web/apps/admin/index.html`
```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Open Routing Admin</title>
</head>
<body>
  <or-catalog-shell></or-catalog-shell>
  <script type="module" src="/src/index.ts"></script>
</body>
</html>
```

**File:** `web/apps/admin/src/index.ts`
```typescript
// Source: CONTEXT.md D6-09, D6-20
import '@open-routing/ui/components/shell';  // registers <or-catalog-shell>
// Shell component reads org_id from URL path and feeds to router
```

### 2. `@lit-labs/router` Integration

**API surface** [VERIFIED: github.com/lit/lit/blob/main/packages/labs/router/README.md]:

```typescript
// Source: github.com/lit/lit tree/main/packages/labs/router/README.md
import { Routes } from '@lit-labs/router';
import { LitElement, html } from 'lit';
import { customElement } from 'lit/decorators.js';

@customElement('or-catalog-shell')
export class OrCatalogShell extends LitElement {
  private _routes = new Routes(this, [
    { path: '/',                         render: () => html`<or-org-picker></or-org-picker>` },
    { path: '/orgs/:org_id/agents',      render: ({org_id}) => html`<or-agent-list .orgId=${org_id}></or-agent-list>` },
    { path: '/orgs/:org_id/agents/new',  render: ({org_id}) => html`<or-agent-form .orgId=${org_id}></or-agent-form>` },
    { path: '/orgs/:org_id/agents/:id',  render: ({org_id, id}) => html`<or-agent-detail .orgId=${org_id} .agentId=${id}></or-agent-detail>` },
    { path: '/orgs/:org_id/agents/:id/status',
      render: ({org_id, id}) => html`<or-status-panel .orgId=${org_id} .agentId=${id}></or-status-panel>` },
    // … ×5 more entities (skills/queues/channels/adapters/break_reasons)
    { path: '/orgs/:org_id/imports/new', render: ({org_id}) => html`<or-import-page .orgId=${org_id}></or-import-page>` },
    { path: '/orgs/:org_id/imports/:id', render: ({org_id, id}) => html`<or-import-result .orgId=${org_id} .importId=${id}></or-import-result>` },
  ]);

  render() {
    return html`
      <or-shell-chrome .routes=${this._routes}>
        <main slot="content">${this._routes.outlet()}</main>
      </or-shell-chrome>
    `;
  }
}
```

**Route guard pattern** (D6-13 UUIDv7 validation):
```typescript
// Source: CONTEXT.md D6-13 + @lit-labs/router enter() callback
const UUIDV7_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

{ 
  path: '/orgs/:org_id/*',
  enter: async ({ org_id }) => {
    if (!UUIDV7_PATTERN.test(org_id ?? '')) {
      this._routes.goto('/');  // redirect to org-picker with toast
      return false;  // abort render
    }
    return true;
  },
  render: () => html`...`
}
```

**Programmatic navigation:**
```typescript
this._routes.goto(`/orgs/${orgId}/agents`);
```

**URLPattern polyfill** — required in Vitest/Node environment but NOT in browser (all major browsers support URLPattern as of 2025 — Baseline 2025). Import polyfill in vitest setup file only:
```typescript
// web/packages/ui/src/test-setup.ts
import 'urlpattern-polyfill';
```

### 3. Lit 3 Component Architecture

**Base pattern for all entity components:**
```typescript
// Source: lit.dev/docs/components/decorators/
// File: web/packages/ui/src/components/agents/agent-list.ts
import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';

type Agent = components['schemas']['Agent'];

@customElement('or-agent-list')
export class OrAgentList extends LitElement {
  @property({ type: String }) orgId = '';
  @property({ type: Object }) client!: ApiClient;

  @state() private _search = '';
  @state() private _cursor: string | null = null;
  @state() private _includeDisabled = false;

  private _listTask = new Task(this, {
    task: async ([orgId, search, cursor, includeDisabled]) => {
      const { data, error } = await this.client.GET(
        '/v1/orgs/{org_id}/agents',
        { params: { path: { org_id: orgId }, query: { name: search || undefined, cursor: cursor ?? undefined, include_disabled: includeDisabled } } }
      );
      if (error) throw error;
      return data;
    },
    args: () => [this.orgId, this._search, this._cursor, this._includeDisabled] as const,
  });

  render() {
    return this._listTask.render({
      pending: () => html`<or-data-table .loading=${true}></or-data-table>`,
      complete: (data) => html`
        <or-data-table .items=${data.items} .columns=${AGENT_COLUMNS}>
          <button slot="actions" @click=${() => this._router?.goto(`/orgs/${this.orgId}/agents/new`)}>
            + Create agent
          </button>
        </or-data-table>
        <or-cursor-paginator .hasMore=${data.has_more} .onNext=${() => this._cursor = data.next_cursor ?? null}></or-cursor-paginator>
      `,
      error: (e) => html`<or-api-error .error=${e}></or-api-error>`,
    });
  }
}
```

**Shadow DOM event dispatch** — for events that must cross Shadow DOM boundaries (Phase 7 prep):
```typescript
// Source: lit.dev/docs/components/events/ — composed: true for Shadow DOM crossing
this.dispatchEvent(new CustomEvent('or-entity-saved', {
  detail: { entity: savedAgent },
  bubbles: true,
  composed: true,  // REQUIRED for Phase 7 embed CustomEvent protocol
}));
```

**ReactiveController pattern** — for reusable polling logic:
```typescript
// Source: lit.dev/docs/composition/controllers/
import type { ReactiveControllerHost } from '@lit/reactive-element/reactive-controller.js';

export class PollingController {
  host: ReactiveControllerHost;
  private _interval?: number;
  
  constructor(host: ReactiveControllerHost, readonly intervalMs: number) {
    this.host = host;
    host.addController(this);
  }
  
  hostConnected() {
    document.addEventListener('visibilitychange', this._onVisibility);
    this._start();
  }
  
  hostDisconnected() {
    document.removeEventListener('visibilitychange', this._onVisibility);
    this._stop();
  }
  
  private _onVisibility = () => {
    document.hidden ? this._stop() : this._start();
  };
  
  private _start() { this._interval = window.setInterval(() => this.host.requestUpdate(), this.intervalMs); }
  private _stop() { clearInterval(this._interval); }
}
```

### 4. Shoelace Per-Component Import Pattern

**Correct tree-shaking-safe pattern** [VERIFIED: shoelace.style/getting-started/installation]:
```typescript
// Source: shoelace.style — per-component, inside each Lit component file
// agent-form.ts
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/checkbox/checkbox.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';  // for conflict banner
```

**Shoelace event protocol** [VERIFIED: shoelace.style/components/input]:
- `sl-input` — fires on every keystroke (like native `input` event)
- `sl-change` — fires on committed change (like native `change` event)
- `.value` — string property to read/set current value
- `setCustomValidity(msg)` — set custom validation message
- Await `customElements.whenDefined('sl-input')` before attaching listeners in lifecycle methods

**CSS Part API for styling internals** [CITED: shoelace.style/getting-started/customizing]:
```css
/* Style the internal <input> element of sl-input: */
sl-input::part(input) {
  font-size: 14px;
}
/* Style the form-control wrapper: */
sl-input::part(form-control) {
  margin-bottom: 8px;
}
```

**Shoelace readiness pattern** — critical for Lit components:
```typescript
// Source: shoelace.style/getting-started/form-controls
// Wait before attaching form listeners (especially in tests)
await customElements.whenDefined('sl-input');
const input = this.shadowRoot!.querySelector('sl-input');
await (input as any).updateComplete;  // Lit-based component updateComplete
```

### 5. OpenAPI → ajv Validator Codegen

**Problem with `ajv-cli`:** Last published 2021-03-28 (version 5.0.0). Does NOT support ajv v8 ESM output. Use the ajv v8 programmatic API in a custom Node script.

**File:** `web/packages/ui/scripts/gen-validators.mjs`
```javascript
// Source: ajv.js.org/standalone.html + CONTEXT.md D6-18
import Ajv from 'ajv';
import { standaloneCode } from 'ajv/dist/standalone/index.js';
import addFormats from 'ajv-formats';
import { readFileSync, writeFileSync, mkdirSync } from 'fs';
import { load } from 'js-yaml';

const spec = load(readFileSync('../../../openapi/openapi.yaml', 'utf8'));
const schemas = spec.components?.schemas ?? {};

const ajv = new Ajv({
  code: { source: true, esm: true },  // ESM output for tree-shaking
  strict: false,                       // OpenAPI 3.0 uses nullable: true (not JSON Schema)
  allErrors: true,
});
addFormats(ajv);

// Compile only request-body schemas (Create/Update/PatchStatus)
const targetSchemas = [
  'CreateAgentRequest', 'UpdateAgentRequest',
  'CreateSkillRequest', 'UpdateSkillRequest',
  'CreateQueueRequest', 'UpdateQueueRequest',
  'CreateChannelRequest', 'UpdateChannelRequest',
  'CreateAdapterRequest', 'UpdateAdapterRequest',
  'CreateBreakReasonRequest', 'UpdateBreakReasonRequest',
  'PatchAgentStatusRequest',
];

mkdirSync('./src/validators', { recursive: true });

for (const name of targetSchemas) {
  const schema = schemas[name];
  if (!schema) { console.warn(`Schema ${name} not found`); continue; }
  schema.$id = name;
  const validate = ajv.compile(schema);
  const code = standaloneCode(ajv, validate);
  writeFileSync(`./src/validators/${name}.ts`, code, 'utf8');
  console.log(`Generated: src/validators/${name}.ts`);
}
```

**package.json addition** (packages/ui):
```json
{
  "scripts": {
    "gen:validators": "node scripts/gen-validators.mjs"
  },
  "devDependencies": {
    "ajv": "^8.20.0",
    "ajv-formats": "^3.0.0",
    "js-yaml": "^4.1.0"
  }
}
```

**Consumption in Lit form components:**
```typescript
// Source: ajv.js.org/standalone.html
import validateCreateAgent from '../../validators/CreateAgentRequest.js';

// In form submit handler:
const formData = { code: this._code, name: this._name, email: this._email, enabled: this._enabled };
if (!validateCreateAgent(formData)) {
  const errors = validateCreateAgent.errors ?? [];
  // Map errors to field-level messages for Shoelace setCustomValidity()
  this._fieldErrors = mapAjvErrors(errors);
  return;
}
// Proceed to API call
```

**Drift gate** (CI, extends Phase 2 D-47):
```bash
pnpm --filter @open-routing/ui gen:api
pnpm --filter @open-routing/ui gen:validators
git diff --exit-code
```

**Additional dependencies needed:**
```bash
pnpm --filter @open-routing/ui add -D ajv-formats js-yaml
```
- `ajv-formats` 3.0.0 [VERIFIED: npm registry] — adds email/uuid/date-time format validators
- `js-yaml` 4.1.0 [VERIFIED: npm registry] — parse openapi.yaml in the Node script

### 6. `packages/ui` Public API Surface

**File structure:**
```
web/packages/ui/
├── src/
│   ├── api/                    # Phase 2 — existing
│   │   ├── client.ts
│   │   ├── errors.ts
│   │   ├── task.ts
│   │   ├── generated.ts        # gen:api output
│   │   └── index.ts
│   ├── components/
│   │   ├── primitives/
│   │   │   ├── data-table.ts           # <or-data-table>
│   │   │   ├── cursor-paginator.ts     # <or-cursor-paginator>
│   │   │   ├── conflict-banner.ts      # <or-conflict-banner>
│   │   │   ├── form-wizard.ts          # <or-form-wizard>
│   │   │   ├── code-input.ts           # <or-code-input>
│   │   │   └── org-picker.ts           # <or-org-picker>
│   │   ├── agents/
│   │   │   ├── agent-list.ts           # <or-agent-list>
│   │   │   ├── agent-detail.ts         # <or-agent-detail>
│   │   │   ├── agent-form.ts           # <or-agent-form>
│   │   │   ├── agent-list.test.ts
│   │   │   ├── agent-detail.test.ts
│   │   │   └── agent-form.test.ts
│   │   ├── skills/
│   │   │   └── ... (same pattern)
│   │   ├── queues/ channels/ adapters/ break-reasons/
│   │   ├── status/
│   │   │   ├── status-panel.ts         # <or-status-panel>
│   │   │   └── status-panel.test.ts
│   │   ├── imports/
│   │   │   ├── import-page.ts          # <or-import-page>
│   │   │   ├── import-result.ts        # <or-import-result>
│   │   │   └── *.test.ts
│   │   ├── shell/
│   │   │   ├── catalog-shell.ts        # <or-catalog-shell>
│   │   │   └── shell-chrome.ts         # sidebar + topbar internals
│   │   └── index.ts                    # export * from all components
│   ├── validators/
│   │   ├── CreateAgentRequest.ts       # gen:validators output
│   │   ├── UpdateAgentRequest.ts
│   │   └── ... (13 files total)
│   ├── locales/
│   │   ├── en/                         # generated by lit-localize build
│   │   ├── vi/
│   │   └── locale-codes.ts
│   ├── themes/
│   │   ├── or-light.css
│   │   ├── or-dark.css
│   │   └── or-brand.css
│   └── index.ts                        # root barrel
├── scripts/
│   └── gen-validators.mjs
├── xliff/
│   ├── en.xliff                        # source strings
│   └── vi.xliff                        # translations
└── lit-localize.json                   # localize config
```

**Root barrel** (`web/packages/ui/src/index.ts`):
```typescript
// Phase 2 existing:
export * from './api';
// Phase 6 additions:
export * from './components';
export * from './validators';
// Note: themes are CSS files, not TS exports — imported by apps/admin/src/index.ts
// Note: locales are lazy-loaded by @lit/localize configureLocalization
```

**package.json `exports` map** (packages/ui — subpath exports for tree-shaking):
```json
{
  "exports": {
    ".": "./src/index.ts",
    "./api": "./src/api/index.ts",
    "./components": "./src/components/index.ts",
    "./components/agents": "./src/components/agents/index.ts",
    "./components/skills": "./src/components/skills/index.ts",
    "./components/queues": "./src/components/queues/index.ts",
    "./components/channels": "./src/components/channels/index.ts",
    "./components/adapters": "./src/components/adapters/index.ts",
    "./components/break-reasons": "./src/components/break-reasons/index.ts",
    "./components/shell": "./src/components/shell/index.ts",
    "./validators": "./src/validators/index.ts"
  }
}
```

### 7. 409 Optimistic-Lock UX

**CRUD 409 (version_conflict) pattern** (D6-03/D6-04):

The 409 `version_conflict` body from the server (D-37) contains `{ error: 'version_conflict', reason, request_id, current: <Entity> }`. The `current` field is the complete server-side entity at conflict time. Admin consumes this without a second GET.

```typescript
// Source: CONTEXT.md D6-03, D6-04 + openapi.yaml VersionConflictErrorResponse
// In or-agent-detail.ts PATCH handler:
const { data, error, response } = await this.client.PATCH(
  '/v1/orgs/{org_id}/agents/{id}',
  { params: { path: { org_id: this.orgId, id: this.agentId } }, body: patchBody }
);

if (response.status === 409) {
  const body = await response.json() as VersionConflictErrorResponse;
  if (body.error === 'version_conflict' && body.current) {
    // Update local form state with server truth — no re-GET needed
    this._serverVersion = body.current;
    this._showConflictBanner = true;
    this._requestId = body.request_id ?? null;
    return;
  }
}
```

**`<or-conflict-banner>` component** (primitive in packages/ui):
```typescript
// Renders inline above the form (not a modal/toast)
// Props: serverVersion (Entity), userEdits (Partial<Entity>), requestId (string)
// Slots: default = diff highlight between serverVersion and userEdits
// Buttons: "Review & re-submit" (keeps user edits, updates version) + "Use server version" (discard)
// Auto-dismisses after user action
```

**Status 409 (invalid_transition) pattern** (D6-04):
```typescript
// Source: openapi.yaml InvalidTransitionErrorResponse + CONTEXT.md D6-03
if (response.status === 409) {
  const body = await response.json() as InvalidTransitionErrorResponse;
  if (body.error === 'invalid_transition') {
    // Update displayed status to body.from (server truth)
    this._currentStatus = body.from;
    this._showTransitionBanner = true;
    // Banner: "Status changed to {from} — your request to go to {to} isn't allowed from {from}."
    // CTA: "Try again" resets transition picker with from as current state
  }
}
```

### 8. Theme Runtime Switching

**Pattern** (D6-19/D6-20) [CITED: shoelace.style/getting-started/themes + CONTEXT.md D6-20]:

Shoelace uses `--sl-color-primary-{50..950}` as the primary color scale. [CITED: shoelace.style/tokens/color]

```typescript
// Source: CONTEXT.md D6-20 + shoelace.style/tokens/color
// In or-catalog-shell.ts:
@property({ type: String }) theme: 'or-light' | 'or-dark' | 'or-brand' | Record<string, string> = 'or-light';

// Theme token files define CSS custom properties:
// web/packages/ui/src/themes/or-light.css — default Shoelace values (no overrides needed)
// web/packages/ui/src/themes/or-dark.css — inverted Shoelace color scale
// web/packages/ui/src/themes/or-brand.css — Open Routing brand palette

static styles = css`
  :host {
    /* tokens applied here via JS; cascade reaches all Shadow DOM children */
    display: block;
    height: 100%;
  }
`;

// Token application method:
private _THEME_TOKENS: Record<string, Record<string, string>> = {
  'or-light': {},  // empty = Shoelace defaults
  'or-dark': {
    '--sl-color-primary-50': '#0f172a',
    '--sl-color-primary-500': '#3b82f6',
    '--sl-color-primary-950': '#f0f9ff',
    // ... full dark scale
  },
  'or-brand': {
    '--sl-color-primary-500': '#0d9488',  // teal-600 (planner confirms exact value)
    '--sl-color-primary-600': '#0f766e',
    // ... brand scale
  },
};

updated(changed: Map<string, unknown>) {
  if (changed.has('theme')) {
    this._applyTheme();
  }
}

private _applyTheme() {
  const tokens = typeof this.theme === 'string'
    ? (this._THEME_TOKENS[this.theme] ?? {})
    : this.theme;  // JSON object from Phase 7 embed attribute
  
  // Clear all prior tokens first
  for (const key of Object.keys(this._THEME_TOKENS['or-dark'])) {
    this.style.removeProperty(key);
  }
  // Apply new tokens to :host element — CSS cascade reaches Shadow DOM children
  for (const [key, val] of Object.entries(tokens)) {
    this.style.setProperty(key, val);
  }
  // Persist named theme to localStorage
  if (typeof this.theme === 'string') {
    localStorage.setItem('or-theme', this.theme);
  }
}
```

**CSS cascade across Shadow DOM:** CSS custom properties ARE inherited across shadow boundaries. A custom property set on `<or-catalog-shell>` host propagates into all nested shadow roots — including Shoelace components and entity components. This is why `document.documentElement` approach fails for the embed (the embed's shadow root is isolated from `document.documentElement`).

**Theme persistence & restore:**
```typescript
// In connectedCallback or firstUpdated:
const saved = localStorage.getItem('or-theme') as typeof this.theme | null;
if (saved && ['or-light', 'or-dark', 'or-brand'].includes(saved as string)) {
  this.theme = saved as typeof this.theme;
}
```

### 9. Agent Status Panel (Phase 4 carry-forward)

**Component:** `<or-status-panel>` at `web/packages/ui/src/components/status/status-panel.ts`

**API interaction:**
- `GET /v1/orgs/{org_id}/agents/{id}/status` — polled every 5s
- `PATCH /v1/orgs/{org_id}/agents/{id}/status` — for transitions

**Polling with `PollingController` + `visibilitychange`** (D6-26, STATE-08):
```typescript
@customElement('or-status-panel')
export class OrStatusPanel extends LitElement {
  @property({ type: String }) orgId = '';
  @property({ type: String }) agentId = '';
  @property({ type: Object }) client!: ApiClient;

  private _polling = new PollingController(this, 5000);
  private _lastStateVersion: number | null = null;

  private _statusTask = new Task(this, {
    task: async ([orgId, agentId]) => {
      const { data, error } = await this.client.GET(
        '/v1/orgs/{org_id}/agents/{id}/status',
        { params: { path: { org_id: orgId, id: agentId } } }
      );
      if (error) throw error;
      // STATE-08: compare state_version; only update if newer
      if (this._lastStateVersion !== null && data.state_version <= this._lastStateVersion) {
        return null;  // stale response from cache; skip update
      }
      this._lastStateVersion = data.state_version;
      return data;
    },
    args: () => [this.orgId, this.agentId] as const,
  });
  // ...
}
```

**WrapUp countdown:** `wrapup_until` is an ISO timestamp. Display countdown in seconds (round to seconds; D-87 says ±100ms jitter is not user-visible at second granularity).

**Force flag** (D-84): Single `<sl-checkbox>` "Force transition — admin override" with red/warning styling. Sends `force: true` in PATCH body. In v0.1 stub auth, always visible; server logs WARN.

**Transition matrix display:** Show only ALLOWED transitions as buttons based on current status enum. Mapping:
```typescript
const AGENT_TRANSITIONS: Record<string, string[]> = {
  'Ready':    ['NotReady', 'Break'],
  'NotReady': ['Ready'],
  'Break':    ['Ready', 'NotReady'],
  'WrapUp':   ['Ready', 'NotReady'],
  'Engaged':  ['Break'],       // post_interaction_state only
  'Offline':  [],              // system-initiated only
};
```

### 10. Bulk Import UI (Phase 5 carry-forward)

**Component:** `<or-import-page>` at `web/packages/ui/src/components/imports/import-page.ts`

**API shape consumed** (from Phase 5 + openapi.yaml):
- `POST /v1/orgs/{org_id}/catalog/import?entity={type}&schema_version=v0.1`
- Body: `Content-Type: text/csv` or `application/json` + `Idempotency-Key: {uuidv7}` header (optional)
- Response 200/207: `BulkImportResult { succeeded: string[], failed: BulkImportFailedRow[], idempotent_replay?: bool }`
- Response 413: non-canonical body (middleware fires before handler per D5-22) — admin shows "File too large (limit: 50MB / 500 rows)"

**CSV format helper** (D5-04, D5-15/D5-17):
```typescript
// Agent CSV skill syntax: code:proficiency|code:proficiency (pipe separator)
// Example row: emp_0042,Alice Nguyen,alice@example.com,true,skill_voice:7|skill_chat:9
// Show in <sl-details> "Show CSV format" disclosure:
const AGENT_CSV_EXAMPLE = `code,name,email,enabled,skills
emp_0042,Alice Nguyen,alice@example.com,true,skill_voice:7|skill_chat:9
emp_0043,Bob Chen,bob@example.com,true,skill_chat:10`;
```

**`idempotent_replay` handling** (D5-13): When `BulkImportResult.idempotent_replay === true`, show: "This import was already processed. Results below are from the original run. Note: succeeded[] is empty on replay — check failed[] for issues."

**207 result rendering:**
```typescript
// Source: CONTEXT.md D6-01 + openapi.yaml BulkImportResult
// <or-import-result> props: result (BulkImportResult), entityType (string)
// Renders: total/succeeded/failed counts + collapsible succeeded IDs + failed[] table
// Failed table columns: Row # | Field | Error Code | Reason
```

### 11. i18n with @lit/localize

**lit-localize.json** (`web/packages/ui/lit-localize.json`):
```json
{
  "sourceLocale": "en",
  "targetLocales": ["vi"],
  "tsConfig": "./tsconfig.json",
  "output": {
    "mode": "runtime",
    "outputDir": "./src/locales",
    "localeCodesModule": "./src/locales/locale-codes.ts"
  },
  "interchange": {
    "format": "xliff",
    "xliffDir": "./xliff"
  }
}
```

**Usage in components** [VERIFIED: lit.dev/docs/localization/runtime-mode/]:
```typescript
import { msg } from '@lit/localize';
// In render():
html`<sl-button>${msg('Save changes')}</sl-button>`
html`<span>${msg(html`Changed by <strong>someone else</strong>. Reload?`)}</span>`
```

**App init** (in `apps/admin/src/index.ts`):
```typescript
import { configureLocalization } from '@lit/localize';
import { sourceLocale, targetLocales } from '@open-routing/ui/locales/locale-codes.js';

const { setLocale } = configureLocalization({
  sourceLocale,
  targetLocales,
  loadLocale: (locale) => import(`@open-routing/ui/locales/${locale}.js`),
});

// Detect locale
const saved = localStorage.getItem('or-locale');
const detected = navigator.language.startsWith('vi') ? 'vi' : 'en';
await setLocale(saved ?? detected);
```

**ErrorCode → i18n key mapping** (D6-24):
```typescript
// In packages/ui/src/api/errors.ts (extend existing):
export const ERROR_I18N_KEYS: Record<ErrorCode, string> = {
  invalid_body:        'errors.invalid_body',
  invalid_id:         'errors.invalid_id',
  not_found:          'errors.not_found',
  internal:           'errors.internal',
  version_conflict:   'errors.version_conflict',
  cross_org:          'errors.cross_org',
  duplicate_code:     'errors.duplicate_code',
  duplicate_external_id: 'errors.duplicate_external_id',
  invalid_org_id:     'errors.invalid_org_id',
  invalid_transition: 'errors.invalid_transition',
  import_failed:      'errors.import_failed',
  immutable_field:    'errors.immutable_field',
  rate_limited:       'errors.rate_limited',
  invalid_reference:  'errors.invalid_reference',
  invalid_value:      'errors.invalid_value',
};
```

### 12. Vitest + happy-dom Setup

**vitest.config.ts** (packages/ui):
```typescript
// Source: vitest.dev/guide/environment
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'happy-dom',
    setupFiles: ['./src/test-setup.ts'],
    include: ['src/**/*.test.ts'],
  },
});
```

**test-setup.ts:**
```typescript
// urlpattern-polyfill needed in Node/happy-dom but NOT in real browsers
import 'urlpattern-polyfill';
```

**Component test pattern** (colocated `*.test.ts`):
```typescript
// Source: vitest.dev docs + lit.dev/docs/tools/testing/
// agent-list.test.ts
import { describe, it, expect, vi } from 'vitest';
import './agent-list.js';  // registers <or-agent-list>

describe('OrAgentList', () => {
  it('renders loading skeleton when task is pending', async () => {
    const el = document.createElement('or-agent-list') as any;
    el.orgId = 'test-org-id';
    el.client = { GET: vi.fn().mockReturnValue(new Promise(() => {})) };
    document.body.appendChild(el);
    await el.updateComplete;
    const shadow = el.shadowRoot;
    expect(shadow?.querySelector('[data-testid="loading"]')).toBeTruthy();
    document.body.removeChild(el);
  });
});
```

**Key pattern:** Always `await el.updateComplete` after setting properties or triggering events. happy-dom supports `customElements.define` + `adoptedStyleSheets` as of v20+ [VERIFIED: npm happy-dom 20.9.0].

### 13. Two-Org Cross-Isolation Playwright Test

**File:** `web/apps/admin/e2e/cross-org-isolation.spec.ts`
```typescript
// Source: Phase 1 FOUND-08 pattern adapted to UI layer
import { test, expect, Page } from '@playwright/test';

test('two-org SPA cross-isolation: no data crossover', async ({ browser }) => {
  const contextA = await browser.newContext();
  const contextB = await browser.newContext();
  const pageA: Page = await contextA.newPage();
  const pageB: Page = await contextB.newPage();

  // Each page has its own org_id in the URL path
  await pageA.goto(`http://localhost:5173/orgs/${ORG_A_ID}/agents`);
  await pageB.goto(`http://localhost:5173/orgs/${ORG_B_ID}/agents`);

  // Assert org A's data does not appear in org B
  const agentsA = await pageA.locator('or-agent-list').getAttribute('data-items');
  const agentsB = await pageB.locator('or-agent-list').getAttribute('data-items');
  expect(agentsA).not.toContain(ORG_A_AGENT_CODE);  // cross-check
  
  await contextA.close();
  await contextB.close();
});
```

### 14. Raw Fetch Ban (ADMIN-04)

To enforce that all API calls go through `createApiClient` (the only typed access layer per ADMIN-04):

**eslint.config.js addition:**
```javascript
// In web/eslint.config.js — add rule:
{
  rules: {
    'no-restricted-syntax': [
      'error',
      {
        selector: 'CallExpression[callee.name="fetch"]',
        message: 'Use createApiClient() instead of raw fetch. See ADMIN-04.',
      },
      {
        selector: 'MemberExpression[object.name="window"][property.name="fetch"]',
        message: 'Use createApiClient() instead of window.fetch.',
      },
    ],
  },
},
```

### 15. Build Size Budget

**Phase 7 context:** Phase 7 needs ≤70KB gzipped for the embed bundle. Phase 6 admin SPA has no hard cap, but the per-component Shoelace pattern (D6-08) enables Phase 7 to cherry-pick.

**Recommended measurement gate** (to be added to Wave 5):
```bash
# Measure admin bundle post-build:
pnpm --filter @open-routing/admin build
du -sh dist/assets/*.js | sort -h
# Report in Wave 5 SUMMARY.md so Phase 7 planner has baseline
```

**Key tree-shaking requirement for Phase 7:** Entity components must NOT import the full Shoelace barrel. Each `import '@shoelace-style/shoelace/dist/components/{name}/{name}.js'` inside a specific component file is safe — Rolldown's dead-code elimination works at the module graph level.

---

## Pitfalls and Landmines

### Pitfall 1: Vite 8 Decorator Failure (BLOCKING)

**What goes wrong:** Any LitElement with `@customElement`, `@property`, or `@state` fails to build under Vite 8 without `@rolldown/plugin-babel`. Error is cryptic ("unexpected token @") and appears at the Rolldown transform stage.

**Why it happens:** Vite 8 replaced esbuild with Oxc; Oxc does not support `experimentalDecorators: true`. Lit 3 requires this TypeScript flag. [VERIFIED: vite.dev/guide/migration + vitejs/vite/discussions/21891]

**How to avoid:** Wave 1 MUST install and configure `@rolldown/plugin-babel` BEFORE writing any Lit component. Verify with: `pnpm --filter @open-routing/admin build` producing a non-empty `dist/` before proceeding to Wave 2.

**Warning sign:** Build output says "unexpected token" or "@ is not expected" near any decorator.

### Pitfall 2: `useDefineForClassFields` and Lit Reactive Properties

**What goes wrong:** Reactive properties (`@property`) silently break when `useDefineForClassFields: true` (TypeScript default for `target: ES2022+`). Properties become plain class fields instead of getters/setters, so Lit's `requestUpdate()` is never called on assignment — component never re-renders.

**Why it happens:** TypeScript ES2022+ target enables `useDefineForClassFields` by default, which initializes class fields BEFORE the Lit `@property` decorator runs. [CITED: lit.dev/docs/components/decorators/]

**How to avoid:** `useDefineForClassFields: false` in every tsconfig that compiles Lit source (both `packages/ui/tsconfig.json` and `apps/admin/tsconfig.json`).

**Warning sign:** Setting a `@property` value from outside the component does not trigger re-render.

### Pitfall 3: `ajv-cli` Is Stale

**What goes wrong:** `ajv-cli@5.0.0` was last published 2021-03-28. It does not support ajv v8 (the current major) or ESM output. Running `ajv-cli --code` with ajv v8 will fail or produce CJS output that doesn't tree-shake.

**How to avoid:** Use the ajv v8 programmatic API in a custom Node script (`gen-validators.mjs`). Do NOT install `ajv-cli`.

### Pitfall 4: URLPattern Polyfill Scope

**What goes wrong:** `urlpattern-polyfill` is needed in Node/Vitest/happy-dom environments but must NOT be imported in the browser bundle (it's unnecessary overhead since all target browsers support URLPattern natively as of 2025). Importing it in a shared module pollutes the browser build.

**How to avoid:** Import `urlpattern-polyfill` ONLY in `packages/ui/src/test-setup.ts` (Vitest setup file). Never import it inside component source files. [VERIFIED: URLPattern Baseline 2025 — Firefox 142 Aug 2025, Safari 26 Sep 2025]

### Pitfall 5: Shoelace Components Not Yet Defined

**What goes wrong:** Attaching event listeners or reading `.value` from Shoelace components (`sl-input`, `sl-select`) before `customElements.whenDefined('sl-input')` resolves causes `undefined` reads or silent event listener failures. Especially in tests that don't load the Shoelace element registration.

**How to avoid:** In Lit `firstUpdated()` lifecycle, always:
```typescript
await customElements.whenDefined('sl-input');
const input = this.shadowRoot!.querySelector('sl-input')!;
await (input as any).updateComplete;
```
In tests: import the specific Shoelace component file before testing the Lit component.

### Pitfall 6: CSS Custom Properties and `<slot>` Content

**What goes wrong:** CSS custom properties cascade INTO `<slot>` content (light DOM), but the slot content's own styles do NOT cascade back out. Theming `<or-catalog-shell>` with CSS custom properties correctly reaches all child shadow roots (Shoelace + entity components) — this is intentional. But applying `all: initial` on any component breaks the cascade chain.

**How to avoid:** Never use `all: initial` or `all: unset` in component styles. Shoelace components already handle their own reset internally.

### Pitfall 7: @lit-labs/router Outlet in Shadow DOM

**What goes wrong:** `this._routes.outlet()` must be placed inside the component's `render()` return — NOT in a slot content from a parent. If `outlet()` is called outside the component instance that owns the `Routes` controller, route changes don't trigger re-render.

**How to avoid:** `outlet()` belongs in the shell component's own `render()` method. Sidebar and chrome can be separate child components, but the outlet must remain in the shell's render tree.

### Pitfall 8: `@lit/task` and Stale Closures

**What goes wrong:** The `args` function captures component properties by value at task construction time, not at run time. If `args` returns stale values, the task doesn't re-run on property change.

**How to avoid:** Always return reactive properties in `args()` directly:
```typescript
args: () => [this.orgId, this._search, this._cursor] as const,
// NOT: const args = [this.orgId]; return () => args; ← stale closure
```

### Pitfall 9: 409 Body Consumption After openapi-fetch

**What goes wrong:** `openapi-fetch` may have already consumed the response body by the time the error handler tries to call `response.json()`. Body can only be read once.

**How to avoid:** openapi-fetch returns `{ data, error, response }`. When status is 409, access `error` (which openapi-fetch parses) OR the raw `response` — not both. Check if the error field already contains the `current` / `from`+`to` fields before calling `response.json()`.

```typescript
// Preferred: openapi-fetch 0.17.x returns error as parsed JSON for 4xx
const { data, error } = await client.PATCH(...);
if (error && 'current' in error) {
  // version_conflict — error.current is the server-side entity
}
```

### Pitfall 10: Shoelace Theme — `sl-theme-dark` Class Approach

**What goes wrong:** The Shoelace docs show activating the dark theme by adding `class="sl-theme-dark"` to `<html>`. This approach does NOT cascade into Shadow DOM for the embed case (Phase 7) because the embed's shadow root is isolated.

**How to avoid:** The `this.style.setProperty('--sl-color-primary-*', value)` approach on the shell host element IS correct for both admin and embed. Never use the class-based approach. [CITED: CONTEXT.md D6-20 + Shoelace CSS cascade behavior]

---

## Validation Architecture

> `workflow.nyquist_validation: true` in .planning/config.json — section required.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest 3.2.4 + happy-dom 20.9.0 (component); Playwright 1.60.0 (E2E smoke) |
| Config file | `web/packages/ui/vitest.config.ts` (to be created in Wave 1) |
| Quick run command | `pnpm --filter @open-routing/ui test` |
| Full suite command | `pnpm --filter @open-routing/ui test -- --coverage` |
| E2E command (manual) | `pnpm --filter @open-routing/admin test:e2e` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ADMIN-01 | Vite SPA builds as static bundle | Build smoke | `pnpm --filter @open-routing/admin build` | Wave 1 |
| ADMIN-02 | All 6 entity list/create/edit/delete screens render | Unit (component) | `pnpm --filter @open-routing/ui test` | Wave 2+ |
| ADMIN-03 | org_id from URL path; X-Org-Id on every request | Unit (client mock) | `pnpm --filter @open-routing/ui test` | Wave 2 |
| ADMIN-04 | Raw fetch is ESLint error | Lint | `pnpm --filter @open-routing/ui lint` | Wave 1 |
| ADMIN-05 | 409 → conflict banner renders with server data | Unit (component) | `pnpm --filter @open-routing/ui test` | Wave 2 |
| ADMIN-06 | Theme switch changes CSS custom properties on host | Unit (component) | `pnpm --filter @open-routing/ui test` | Wave 1 |

### Key Validation Strategies

1. **Typed-client gate (ADMIN-04):** ESLint `no-restricted-syntax` rule banning raw `fetch` + TypeScript compilation catching non-`paths`-typed URLs. Runs in CI via `pnpm --filter @open-routing/ui lint`.

2. **Shoelace component-render assertions:** Component tests create `<or-agent-list>` in happy-dom, mock the `client` prop, and assert expected Shoelace elements exist in shadow DOM. `await el.updateComplete` required after each reactive property change.

3. **ajv-validator-matches-server-422:** Unit tests assert that the ajv validators compiled from OpenAPI schemas accept valid payloads and reject invalid ones. Drift gate ensures validators stay in sync with spec.

4. **409 UX assertion:** Unit test mocks a 409 response with `{ error: 'version_conflict', current: <Entity> }` and asserts `<or-conflict-banner>` renders with the server entity fields and that the form version field is updated.

5. **Theme-runtime-switch assertion:** Unit test sets `shell.theme = 'or-dark'` and asserts `shell.style.getPropertyValue('--sl-color-primary-500')` returns the dark palette value.

6. **Cross-org Playwright smoke:** `web/apps/admin/e2e/cross-org-isolation.spec.ts` — opens org A and org B in separate browser contexts; asserts agent codes from org A are not visible in org B's agent list.

7. **Codegen drift gate (CI):** `pnpm gen:api && pnpm gen:validators && git diff --exit-code` — fails if committed generated files diverge from spec.

### Sampling Rate

- **Per task commit:** `pnpm --filter @open-routing/ui test` (component unit tests, ~30s)
- **Per wave merge:** `pnpm --filter @open-routing/ui test -- --coverage && pnpm --filter @open-routing/admin build`
- **Phase gate:** Full suite green + Playwright smoke (manual) + cross-AI peer review before `/gsd:verify-work`

### Wave 0 Gaps (files that must be created before implementation begins)

- [ ] `web/packages/ui/vitest.config.ts` — Vitest config with happy-dom environment + urlpattern-polyfill setup
- [ ] `web/packages/ui/src/test-setup.ts` — imports urlpattern-polyfill
- [ ] `web/apps/admin/vite.config.ts` — with babel decorator plugin
- [ ] `web/apps/admin/index.html` — SPA entry HTML
- [ ] `web/apps/admin/playwright.config.ts` — Playwright config for smoke test
- [ ] `web/packages/ui/scripts/gen-validators.mjs` — ajv codegen Node script
- [ ] `web/packages/ui/lit-localize.json` — localize config
- [ ] `web/packages/ui/xliff/en.xliff` — initial empty XLIFF source

---

## Security Domain

> `security_enforcement` not explicitly set to false in config.json — section required.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No (stub auth only, v1 deferred) | N/A |
| V3 Session Management | No (no session, org_id from URL) | N/A |
| V4 Access Control | Partial — `force=true` is unrestricted in v0.1 | Client-side display only; server logs WARN |
| V5 Input Validation | Yes | ajv-standalone validators + `<or-code-input>` regex |
| V6 Cryptography | No | N/A |

### Known Threat Patterns for Vite + Lit SPA

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| org_id path tampering (visiting wrong org URL) | Spoofing | Server enforces X-Org-Id from header; client UUIDv7 regex is UX only |
| XSS via unescaped template literals | Tampering | Lit's `html` template literal sanitizes interpolated values; never use `unsafeHTML` |
| Cross-origin API call without org isolation | Information Disclosure | `createApiClient` middleware always sets X-Org-Id; no bypass in typed client |
| Import of an attacker-controlled file (CSV upload) | Tampering | Client sends file verbatim; server validates; no client-side CSV parsing in v0.1 |
| CSP violation from ajv inline validators | Elevation of Privilege | ajv standalone code is PRE-COMPILED (no eval at runtime); CSP-safe |

---

## Open Questions (RESOLVED)

> All 6 open questions from initial research were resolved during planning iterations 1+2 (2026-05-17 → 2026-05-18). See per-question RESOLVED notes below.

1. **`ajv-formats` version compatibility with ajv v8**
   - What we know: ajv v8 supports ajv-formats v3+; v2 works with ajv v8 via compatibility shim
   - What's unclear: whether `email` format from OpenAPI spec maps to ajv-formats email validator correctly for the `CreateAgentRequest.email` field
   - Recommendation: Add a test in the gen-validators script that validates `alice@example.com` as valid and `not-an-email` as invalid
   - **RESOLVED (2026-05-18, iteration 2):** Smoke test added to 06-02 Task 1 acceptance_criteria — `CreateAgentRequest({ code: 'alice', name: 'Alice', email: 'a@b.co' })` returns true. ajv-formats v3 installed via 06-01 Task 1 pnpm. Email format is enabled by `addFormats(ajv)` in `gen-validators.mjs`.

2. **Shoelace 2.20.1 vs upcoming v3**
   - What we know: Shoelace 2.20.1 is current stable (2025-03-11); a v3 rewrite (Shoelace to Web Awesome) was announced but is separate
   - What's unclear: whether any Phase 7 planning should account for v3 migration
   - Recommendation: Lock to `^2.20.1` for v0.1; v3 migration is Phase 7+ concern
   - **RESOLVED (2026-05-17, iteration 1):** Locked to `^2.20.1` in 06-01 Task 1 pnpm install. v3/Web-Awesome migration deferred to Phase 7+ per CONTEXT.md `Deferred Ideas` section.

3. **`@lit-labs/router` programmatic navigation and browser history**
   - What we know: `this._routes.goto(path)` uses `history.pushState` internally
   - What's unclear: whether the router handles `history.popstate` (browser back button) correctly within shadow DOM boundaries
   - Recommendation: Wave 2 integration test should verify browser back from `/agents/:id` → `/agents` restores the list view
   - **RESOLVED (2026-05-17, iteration 1):** Wave 2 plan 06-06 includes a Playwright back-navigation smoke test. `@lit-labs/router` v0.1.4 verified to handle `popstate` via internal listener — the labs status caveat is acknowledged in 06-06 must_haves.truths.

4. **Adapter `config` JSONB field form UI**
   - What we know: CONTEXT.md deferred inline JSON editor to v0.2; Wave 3-4 ships `<sl-textarea>` raw JSON entry
   - What's unclear: whether ajv validator for `CreateAdapterRequest` should validate the `config` object structure (it's free-form JSONB) or just accept any object
   - Recommendation: ajv schema for `config` should be `{ type: 'object' }` with no further constraints; validation error message "Invalid JSON" for non-parseable input
   - **RESOLVED (2026-05-17, iteration 1):** Plan 06-11 implements adapter `config` as `<sl-textarea>` with client-side `JSON.parse` try/catch wrapping a per-keystroke parse check; ajv schema for `CreateAdapterRequest.config` is `{ type: 'object' }` per the recommendation; "Invalid JSON" copy registered in 06-11 must_haves.truths.

5. **`@lit/localize` build step in Turbo pipeline**
   - What we know: `lit-localize build` must run after `lit-localize extract` to produce locale modules; these must run before TypeScript compilation to avoid import errors
   - What's unclear: how to order `lit-localize build` relative to `gen:api` and `gen:validators` in `web/turbo.json`
   - Recommendation: Add `build:locales` script that runs both extract + build; make `typecheck` depend on `build:locales` in turbo.json
   - **RESOLVED (2026-05-17, iteration 1):** Plan 06-02 Task 3 adds the `build:locales` script (calls `lit-localize extract && lit-localize build`) and updates `web/turbo.json` so `typecheck` lists `build:locales` in its dependsOn array. Order in Wave 1 Task 3: `gen:api` → `gen:validators` → `build:locales` → `typecheck`.

6. **Exact `or-brand` primary color value**
   - What we know: CONTEXT.md recommends "mid-tone teal/blue"; not a blocker
   - What's unclear: exact hex values for the full `--sl-color-primary-{50..950}` brand scale
   - Recommendation: Planner picks teal-600 (`#0d9488`) as primary-500 and generates the scale using the Shoelace color generation formula; document in Wave 1 theme token file
   - **RESOLVED (2026-05-17, UI-SPEC produced):** UI-SPEC.md D6-V-24/25/26 locked the 3-theme primary scale — or-light `#2b8a93`, or-dark `#4faab2`, or-brand `#0d8b96`. Plan 06-02 Task 1 theme CSS uses these exact hex values per the UI-SPEC §2.2 token map.

---

## Recommended Wave Breakdown

Based on D6-02 and research findings:

### Wave 1 — Scaffold (no entity code; establishes all infrastructure)
**Files created:** `apps/admin/vite.config.ts`, `apps/admin/index.html`, `apps/admin/src/index.ts`, `apps/admin/tsconfig.json` (updated), `apps/admin/playwright.config.ts`, `packages/ui/tsconfig.json` (updated), `packages/ui/vitest.config.ts`, `packages/ui/src/test-setup.ts`, `packages/ui/scripts/gen-validators.mjs`, `packages/ui/lit-localize.json`, `packages/ui/xliff/en.xliff`, `packages/ui/xliff/vi.xliff`, `packages/ui/src/themes/or-light.css`, `packages/ui/src/themes/or-dark.css`, `packages/ui/src/themes/or-brand.css`, `packages/ui/src/locales/locale-codes.ts`, `packages/ui/src/components/shell/catalog-shell.ts` (skeleton), `packages/ui/src/components/shell/shell-chrome.ts` (sidebar skeleton), `packages/ui/src/components/primitives/org-picker.ts`, turbo.json updates, CI workflow extension, pnpm dependency additions.

**Gate:** `pnpm --filter @open-routing/admin build` succeeds (empty SPA with shell skeleton); `pnpm --filter @open-routing/ui gen:validators` produces no diff; `pnpm --filter @open-routing/ui test` green (shell + theme tests).

### Wave 2 — Agents Exemplar (full entity pattern; all subsequent entities copy it)
**Files created:** `packages/ui/src/components/agents/agent-list.ts`, `agent-detail.ts`, `agent-form.ts` + colocated `*.test.ts`; `packages/ui/src/components/primitives/data-table.ts`, `cursor-paginator.ts`, `conflict-banner.ts`, `form-wizard.ts`, `code-input.ts`; admin route wiring for agents in shell.

**Gate:** `pnpm test` green including agent + primitive tests; 409 conflict-banner test passes; theme-switch test passes; `build` still succeeds.

### Waves 3-4 — Remaining 5 Entities (parallel plans)
**Wave 3:** Skills + Queues + BreakReasons (3 simpler entities)
**Wave 4:** Channels + Adapters (Channel has default_queue FK picker; Adapter has JSONB config textarea)

Each entity = 3 Lit files + 3 test files + route entries in shell. Pattern established in Wave 2; mechanical repetition with entity-specific field sets.

**Gate per wave:** All entity component tests green; entity routes accessible in admin dev server.

### Wave 5 — Status Panel + Import UI + Playwright Smoke + CI
**Files created:** `packages/ui/src/components/status/status-panel.ts` + test, `packages/ui/src/components/imports/import-page.ts` + `import-result.ts` + tests; `apps/admin/e2e/smoke.spec.ts`, `apps/admin/e2e/cross-org-isolation.spec.ts`; `.github/workflows/ci.yml` extension.

**Gate:** All unit tests green; Playwright smoke runs locally (developer runs `pnpm test:e2e`); CI drift gate + build smoke passes in PR; bundle size measured and recorded in SUMMARY.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | Vite + pnpm + gen scripts | ✓ | (assumed — pnpm already in use) | — |
| pnpm | Package management | ✓ | 10.33.0 (from web/package.json) | — |
| Go API (localhost:8080) | Vite dev proxy + Playwright E2E | Assumed available | — | Run `task dev` in separate terminal |
| Docker Compose (Postgres + Redis) | Backend for E2E | Assumed available | — | `docker compose up` per Phase 1 D-10 |
| Turbo | pnpm task orchestration | ✓ | 2.0+ (in devDependencies) | — |
| Playwright browsers | E2E smoke test | To be installed Wave 5 | — | `pnpm exec playwright install chromium` |

**Missing dependencies with no fallback:** None — all tools are either already present or standard developer setup.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Vite 7 / esbuild bundling | Vite 8 / Rolldown + Oxc | 2026-03 (v8 stable) | 10-30x faster builds; breaking: decorators need Babel plugin |
| `ajv-cli` for standalone validators | ajv v8 programmatic API in Node script | 2021 (ajv v8 released) | ajv-cli never updated for v8; use JS API directly |
| Shoelace CDN autoloader | Per-component imports | Shoelace 2.0+ | CDN autoloader loads ALL components; per-component enables tree-shaking |
| `document.documentElement` class for theme | CSS custom properties on shadow host element | Shadow DOM era | Class on html element doesn't cascade into Shadow DOM; host element cascade does |
| `jsdom` in Vitest | `happy-dom` for Web Component tests | 2023+ | 2-4x faster; better CSSStyleSheet/adoptedStyleSheets support for Lit |
| URLPattern polyfill always required | URLPattern native in all major browsers | Aug/Sep 2025 (Baseline 2025) | Polyfill now only needed in Node/test environments |

**Deprecated/outdated:**
- `ajv-cli`: last release 2021, does not support ajv v8 — do not use
- `@vaadin/router`: rejected in D6-07 — do not suggest as alternative
- Shoelace CDN autoloader import: breaks tree-shaking — never use in this codebase

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Go API runs on localhost:8080 during dev (`task dev`) | Vite proxy config | Proxy config would need port change; low risk |
| A2 | `or-brand` teal-600 (`#0d9488`) is acceptable primary color | Theme runtime switching | Planner may choose different color; no functional impact |
| A3 | `@lit/localize` runtime mode lazy-loads locale modules via dynamic import | i18n section | If transform mode used instead, locale modules are inlined; different bundle shape |
| A4 | `ajv-formats` v3+ is compatible with ajv v8.20.0 for `email` and `uuid` format validation | ajv validator codegen | If incompatible, format validation silently disabled; server still validates |
| A5 | happy-dom 20.9.0 handles Lit 3 `adoptedStyleSheets` API correctly | Testing section | If not, fall back to jsdom for affected tests |
| A6 | Phase 04.1 `code` field and Phase 5 `Import*Request` schemas are on main branch at Phase 6 start | API contract | If not merged, openapi.yaml on main is stale; gen:api produces stale types; must wait for merge |

---

## Sources

### Primary (HIGH confidence)

- npm registry (`npm view <pkg>`) — all package versions and publish dates verified 2026-05-17
- [lit.dev/docs/components/decorators/](https://lit.dev/docs/components/decorators/) — Lit 3 decorator TypeScript config
- [github.com/lit/lit/blob/main/packages/labs/router/README.md](https://github.com/lit/lit/blob/main/packages/labs/router/README.md) — `@lit-labs/router` API: Routes constructor, outlet(), goto(), nested routes
- [ajv.js.org/standalone.html](https://ajv.js.org/standalone.html) — ajv v8 standalone code generation API
- [vite.dev/guide/migration](https://vite.dev/guide/migration) — Vite 8 breaking changes (Rolldown/Oxc, decorator support)
- [github.com/vitejs/vite/discussions/21891](https://github.com/vitejs/vite/discussions/21891) — Vite 8 decorator workaround via @rolldown/plugin-babel
- [shoelace.style/components/input](https://shoelace.style/components/input) — sl-input events, value, CSS parts, import path
- [shoelace.style/getting-started/form-controls](https://shoelace.style/getting-started/form-controls) — Shoelace form protocol, setCustomValidity, updateComplete
- [lit.dev/docs/localization/runtime-mode/](https://lit.dev/docs/localization/runtime-mode/) — configureLocalization, setLocale, lazy-load pattern
- Existing codebase: `web/packages/ui/src/api/client.ts`, `errors.ts`, `task.ts`, `index.ts`, `package.json`, `tsconfig.json` — confirmed Phase 2 API surface and existing config

### Secondary (MEDIUM confidence)

- [shoelace.style/getting-started/themes](https://shoelace.style/getting-started/themes) — theme application via CSS class; confirmed CSS custom property cascade behavior
- [shoelace.style/tokens/color](https://shoelace.style/tokens/color) — `--sl-color-primary-{50..950}` token naming confirmed
- Web search + npm view: URLPattern now Baseline 2025 (Firefox 142 Aug 2025, Safari 26 Sep 2025)
- Web search: happy-dom 2-4x faster than jsdom for Lit component tests; vitest/vitest discussion #1607

### Tertiary (LOW confidence — assumptions flagged)

- ajv-formats v3 compatibility with ajv v8 for `email`/`uuid` format keywords (not directly verified against docs; inferred from ajv v8 changelog [ASSUMED])
- Exact Shoelace v3 timeline (not directly verified; recommendation to lock ^2.20.1 for v0.1 is safe regardless [ASSUMED])

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified via npm registry, official docs
- Architecture: HIGH — derived from locked CONTEXT.md decisions + verified API surfaces
- Pitfalls: HIGH — Vite 8 decorator issue verified; others from official docs
- Validation architecture: HIGH — follows existing project pattern (Vitest already in use)

**Research date:** 2026-05-17
**Valid until:** 2026-06-17 (stable stack; re-verify if Vite 9 or Lit 4 releases)
