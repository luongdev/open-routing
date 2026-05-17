# Phase 6: Shared UI Library & Standalone Admin - Pattern Map

**Mapped:** 2026-05-17
**Files analyzed:** 55 (new/modified files across all 5 waves)
**Analogs found:** 8 / 55 (from existing codebase); remaining 47 are new patterns referenced from RESEARCH.md

---

## File Classification

### Wave 1 — Scaffold

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/apps/admin/package.json` | package manifest | — | `web/packages/ui/package.json` | role-match |
| `web/apps/admin/tsconfig.json` | config | — | `web/tsconfig.base.json` | role-match |
| `web/apps/admin/index.html` | scaffold | — | NO ANALOG — new pattern | — |
| `web/apps/admin/vite.config.ts` | build config | — | NO ANALOG — new pattern | — |
| `web/apps/admin/playwright.config.ts` | build config | — | NO ANALOG — new pattern | — |
| `web/apps/admin/src/index.ts` | scaffold | request-response | `web/packages/ui/src/index.ts` | partial |
| `web/packages/ui/tsconfig.json` | config | — | `web/tsconfig.base.json` | role-match |
| `web/packages/ui/vitest.config.ts` | config | — | NO ANALOG — new pattern | — |
| `web/packages/ui/src/test-setup.ts` | config | — | NO ANALOG — new pattern | — |
| `web/packages/ui/scripts/gen-validators.mjs` | utility | batch | NO ANALOG — new pattern | — |
| `web/packages/ui/lit-localize.json` | config | — | NO ANALOG — new pattern | — |
| `web/packages/ui/xliff/en.xliff` | config | — | NO ANALOG — new pattern | — |
| `web/packages/ui/xliff/vi.xliff` | config | — | NO ANALOG — new pattern | — |
| `web/packages/ui/src/themes/or-light.css` | theme | — | NO ANALOG — new pattern | — |
| `web/packages/ui/src/themes/or-dark.css` | theme | — | NO ANALOG — new pattern | — |
| `web/packages/ui/src/themes/or-brand.css` | theme | — | NO ANALOG — new pattern | — |
| `web/packages/ui/src/locales/locale-codes.ts` | config | — | NO ANALOG — new pattern | — |
| `web/packages/ui/src/components/shell/catalog-shell.ts` | Lit component | request-response | NO ANALOG — new pattern | — |
| `web/packages/ui/src/components/shell/shell-chrome.ts` | Lit component | request-response | NO ANALOG — new pattern | — |
| `web/packages/ui/src/components/primitives/org-picker.ts` | Lit component | request-response | NO ANALOG — new pattern | — |
| `web/turbo.json` (modify) | config | — | `web/turbo.json` (existing) | exact |
| `web/packages/ui/package.json` (modify) | package manifest | — | `web/packages/ui/package.json` (existing) | exact |
| `.github/workflows/ci.yml` (modify) | config | — | NO ANALOG — extend existing | — |

### Wave 2 — Agents Exemplar + Primitives

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/packages/ui/src/components/agents/agent-list.ts` | Lit component | CRUD | `web/packages/ui/src/api/task.ts` | partial (async task pattern) |
| `web/packages/ui/src/components/agents/agent-detail.ts` | Lit component | CRUD | `web/packages/ui/src/api/client.ts` | partial (client usage) |
| `web/packages/ui/src/components/agents/agent-form.ts` | form | CRUD | `web/packages/ui/src/api/errors.ts` | partial (error handling) |
| `web/packages/ui/src/components/agents/agent-list.test.ts` | test | — | `web/packages/ui/src/api/client.test.ts` | role-match |
| `web/packages/ui/src/components/agents/agent-detail.test.ts` | test | — | `web/packages/ui/src/api/client.test.ts` | role-match |
| `web/packages/ui/src/components/agents/agent-form.test.ts` | test | — | `web/packages/ui/src/api/client.test.ts` | role-match |
| `web/packages/ui/src/components/primitives/data-table.ts` | Lit component | CRUD | NO ANALOG — new pattern | — |
| `web/packages/ui/src/components/primitives/cursor-paginator.ts` | Lit component | CRUD | NO ANALOG — new pattern | — |
| `web/packages/ui/src/components/primitives/conflict-banner.ts` | Lit component | event-driven | NO ANALOG — new pattern | — |
| `web/packages/ui/src/components/primitives/form-wizard.ts` | Lit component | CRUD | NO ANALOG — new pattern | — |
| `web/packages/ui/src/components/primitives/code-input.ts` | Lit component | event-driven | NO ANALOG — new pattern | — |

### Waves 3-4 — Remaining 5 Entities (pattern-copy of Wave 2 agents)

Each of the 5 entities (skills, queues, channels, adapters, break-reasons) produces 3 Lit components + 3 test files = 15 Lit components + 15 tests. All copy the agent pattern from Wave 2.

| Entity Pattern (×5) | Role | Data Flow | Closest Analog | Match Quality |
|---------------------|------|-----------|----------------|---------------|
| `src/components/{entity}/{entity}-list.ts` | Lit component | CRUD | `agents/agent-list.ts` (Wave 2) | exact |
| `src/components/{entity}/{entity}-detail.ts` | Lit component | CRUD | `agents/agent-detail.ts` (Wave 2) | exact |
| `src/components/{entity}/{entity}-form.ts` | form | CRUD | `agents/agent-form.ts` (Wave 2) | exact |
| `src/components/{entity}/{entity}-*.test.ts` | test | — | `agents/agent-*.test.ts` (Wave 2) | exact |

### Wave 5 — Status Panel + Import UI + E2E + CI

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/packages/ui/src/components/status/status-panel.ts` | Lit component | event-driven | `web/packages/ui/src/api/task.ts` | partial (Task polling) |
| `web/packages/ui/src/components/status/status-panel.test.ts` | test | — | `web/packages/ui/src/api/client.test.ts` | role-match |
| `web/packages/ui/src/components/imports/import-page.ts` | Lit component | file-I/O | NO ANALOG — new pattern | — |
| `web/packages/ui/src/components/imports/import-result.ts` | Lit component | CRUD | NO ANALOG — new pattern | — |
| `web/packages/ui/src/components/imports/import-page.test.ts` | test | — | `web/packages/ui/src/api/client.test.ts` | role-match |
| `web/apps/admin/e2e/smoke.spec.ts` | test | — | NO ANALOG — new pattern | — |
| `web/apps/admin/e2e/cross-org-isolation.spec.ts` | test | — | NO ANALOG — new pattern | — |

### Generated Files (codegen output — not hand-written)

| Generated File | Role | Analog |
|---------------|------|--------|
| `web/packages/ui/src/validators/CreateAgentRequest.ts` (×13 schemas) | validator | NO ANALOG — ajv standalone output |
| `web/packages/ui/src/locales/en/*.ts` | locale | NO ANALOG — lit-localize build output |
| `web/packages/ui/src/locales/vi/*.ts` | locale | NO ANALOG — lit-localize build output |

---

## Pattern Assignments

### `web/apps/admin/package.json` (package manifest)

**Analog:** `web/packages/ui/package.json`

**Import structure pattern** (lines 1-26, full file):
```json
{
  "name": "@open-routing/ui",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "exports": { ".": "./src/index.ts" },
  "scripts": {
    "typecheck": "tsc --noEmit",
    "lint": "eslint src --max-warnings 0",
    "test": "vitest run",
    "gen:api": "openapi-typescript ../../../openapi/openapi.yaml -o src/api/generated.ts"
  }
}
```

**Deviation note:** Admin package adds `dev`, `build`, `preview`, `test:e2e` scripts + Vite/Lit/Shoelace/Playwright deps. No `gen:*` scripts (those live in `packages/ui`). Name is `@open-routing/admin`.

---

### `web/apps/admin/tsconfig.json` and `web/packages/ui/tsconfig.json` (config)

**Analog:** `web/tsconfig.base.json`

**Base config pattern** (full file, 16 lines):
```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "verbatimModuleSyntax": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"]
  }
}
```

**Deviation note:** Both Lit tsconfigs extend base and add `experimentalDecorators: true` + `useDefineForClassFields: false` — REQUIRED for Lit 3 reactive properties to work (RESEARCH.md Pitfall 2). Without these, `@property` decorators silently break reactivity.

**Required extension** (from RESEARCH.md §1):
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

---

### `web/apps/admin/vite.config.ts` (build config)

**Analog:** NO ANALOG — new pattern. Reference RESEARCH.md §1.

**Pattern to copy** (from RESEARCH.md §1):
```typescript
import { defineConfig } from 'vite';
import babel from '@rolldown/plugin-babel';

export default defineConfig({
  appType: 'spa',
  plugins: [
    babel({
      presets: [{ preset: () => ({
        plugins: [['@babel/plugin-proposal-decorators', { version: '2023-11' }]],
      }), rolldown: { filter: { code: '@' } } }],
    }),
  ],
  server: {
    proxy: {
      '/v1':      'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      '/readyz':  'http://localhost:8080',
    },
  },
  build: { target: 'esnext' },
});
```

**Deviation note:** The `@rolldown/plugin-babel` plugin is REQUIRED — Vite 8 Oxc does not lower TypeScript `experimentalDecorators`. Skip it and every `@customElement`/`@property` decorator fails with "unexpected token @".

---

### `web/apps/admin/src/index.ts` (scaffold / SPA entry)

**Analog:** `web/packages/ui/src/index.ts` (lines 1-4) — barrel pattern

**Existing barrel** (full file):
```typescript
// Root barrel — re-exports the API module per D-40.
export * from './api';
```

**Deviation note:** Admin entry is NOT a barrel — it's an imperative bootstrap: import locale setup, import shell component registration, and call `configureLocalization`. Pattern from RESEARCH.md §2 / §11.

**Pattern to write** (from RESEARCH.md §11):
```typescript
import { configureLocalization } from '@lit/localize';
import { sourceLocale, targetLocales } from '@open-routing/ui/locales/locale-codes.js';
import '@open-routing/ui/components/shell';  // registers <or-catalog-shell>

const { setLocale } = configureLocalization({
  sourceLocale, targetLocales,
  loadLocale: (locale) => import(`@open-routing/ui/locales/${locale}.js`),
});
const saved = localStorage.getItem('or-locale');
const detected = navigator.language.startsWith('vi') ? 'vi' : 'en';
await setLocale(saved ?? detected);
```

---

### `web/turbo.json` (modify — add `gen:validators`)

**Analog:** `web/turbo.json` (existing, full file):
```json
{
  "$schema": "https://turbo.build/schema.json",
  "tasks": {
    "typecheck": { "dependsOn": ["^typecheck"], "outputs": [] },
    "lint":      { "outputs": [] },
    "gen:api":   { "inputs": ["../../../openapi/openapi.yaml"], "outputs": ["src/api/generated.ts"] }
  }
}
```

**Deviation note:** Add `gen:validators` task with same `inputs` as `gen:api` but `outputs: ["src/validators/*.ts"]`. Also add `build:locales` and `build` tasks for admin. `typecheck` should depend on `^gen:api` and `^gen:validators`.

---

### `web/packages/ui/src/index.ts` (modify — extend barrel)

**Analog:** `web/packages/ui/src/index.ts` (existing, full file):
```typescript
// Root barrel — re-exports the API module per D-40. Phase 6 will add
// Lit component exports alongside this line.
export * from './api';
```

**Deviation note:** Phase 6 adds `export * from './components';` and `export * from './validators';`. Themes are CSS files (not TS exports); locales are lazy-loaded — neither gets a barrel export.

---

### `web/packages/ui/src/api/errors.ts` (modify — add `ERROR_I18N_KEYS`)

**Analog:** `web/packages/ui/src/api/errors.ts` (existing, full file)

**Existing error pattern** (lines 24-35):
```typescript
export const ErrorCodes = {
  INVALID_BODY: 'invalid_body',
  NOT_FOUND:    'not_found',
  VERSION_CONFLICT: 'version_conflict',
  INVALID_TRANSITION: 'invalid_transition',
  // ... 10 total
} as const;
export type ErrorCode = (typeof ErrorCodes)[keyof typeof ErrorCodes];
```

**Deviation note:** Phase 6 appends `ERROR_I18N_KEYS: Record<ErrorCode, string>` map (D6-24). Also adds `duplicate_code`, `duplicate_external_id`, `immutable_field`, `invalid_reference`, `invalid_value` to `ErrorCodes` — these come from Phase 04.1 and must match the updated openapi.yaml on main.

---

### `web/packages/ui/src/api/client.ts` (no change — reference for components)

**Analog:** `web/packages/ui/src/api/client.ts` (full file — read above)

**Key pattern for components** (lines 45-65):
```typescript
export function createApiClient(config: CreateApiClientConfig): ApiClient {
  const orgIdMiddleware: Middleware = {
    onRequest({ request }) {
      request.headers.set('X-Org-Id', config.getOrgId());
      return request;
    },
  };
  const client = createClient<paths>(clientOptions);
  client.use(orgIdMiddleware);
  return client;
}
```

**Deviation note:** Components receive `client` as a `@property` — they never construct it. The shell constructs one client at boot with `getOrgId: () => currentOrgId` from the router params (D6-09).

---

### Entity List Component — `agents/agent-list.ts` (Wave 2 exemplar; Waves 3-4 copy)

**Analog:** `web/packages/ui/src/api/task.ts` (async Task pattern) + `client.ts` (client usage)

**`createApiTask` pattern** (task.ts lines 20-34):
```typescript
export function createApiTask<TArgs extends readonly unknown[], TData>(
  host: ReactiveControllerHost,
  config: {
    task: (args: TArgs, signal: AbortSignal) => Promise<TData>;
    args: () => TArgs;
  },
): Task<TArgs, TData> {
  return new Task<TArgs, TData>(host, {
    task: (args, options) => config.task(args, options.signal),
    args: config.args,
  });
}
```

**Full entity list pattern** (from RESEARCH.md §3):
```typescript
import { LitElement, html } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';

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
      complete: (data) => html`<or-data-table .items=${data.items}></or-data-table>`,
      error:   (e)    => html`<p role="alert">${(e as Error).message}</p>`,
    });
  }
}
```

**Deviation note for Waves 3-4:** Replace `agents` with entity name (`skills`, `queues`, etc.), API path, and field set. Skills/Queues/BreakReasons/Adapters skip the `@state() _includeDisabled` filter unless the entity has `enabled` field. Copy Shoelace imports from the specific fields used.

---

### Entity Detail Component — `agents/agent-detail.ts`

**Analog:** `web/packages/ui/src/api/errors.ts` (error handling pattern)

**409 conflict handling pattern** (from RESEARCH.md §7 + errors.ts isApiError lines 51-59):
```typescript
// In PATCH submit handler:
const { data, error } = await this.client.PATCH('/v1/orgs/{org_id}/agents/{id}', { ... });
if (error && 'current' in error) {
  // version_conflict — error.current is the full server entity (no re-GET needed)
  this._serverVersion = (error as any).current;
  this._showConflictBanner = true;
  return;
}
if (error) {
  this._apiError = error;
  return;
}
```

**`isApiError` pattern** (errors.ts lines 51-59):
```typescript
export function isApiError(value: unknown): value is ApiError {
  return typeof value === 'object' && value !== null
    && 'error' in value && typeof (value as { error: unknown }).error === 'string'
    && 'reason' in value && typeof (value as { reason: unknown }).reason === 'string';
}
```

**Deviation note:** Detail page handles 3 flows: read (GET on mount), edit (PATCH on save), delete (DELETE on confirm). `@state()` tracks `_editing: boolean`. `code` field is rendered read-only with tooltip per D04_1-02.

---

### Entity Form Component — `agents/agent-form.ts`

**Analog:** NO ANALOG for the Shoelace/ajv form pattern. Reference RESEARCH.md §4 (Shoelace) + §5 (ajv).

**Shoelace per-component import pattern** (RESEARCH.md §4):
```typescript
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/checkbox/checkbox.js';
```

**ajv validator consumption** (RESEARCH.md §5):
```typescript
import validateCreateAgent from '../../validators/CreateAgentRequest.js';

// In submit handler:
if (!validateCreateAgent(formData)) {
  this._fieldErrors = mapAjvErrors(validateCreateAgent.errors ?? []);
  return;
}
// Proceed to client.POST(...)
```

**Deviation note:** Agent form is multi-step (Step 1: basics, Step 2: skills assignment, Step 3: review). Other entities use single-step. All use `<or-form-wizard>` which auto-completes single-step flows. `<or-code-input>` used for the `code` field with regex `^[a-z][a-z0-9_]{0,63}$` (D04_1-03).

---

### Component Tests — `agents/agent-list.test.ts` (exemplar)

**Analog:** `web/packages/ui/src/api/client.test.ts` (Vitest structure)

**Test file structure** (client.test.ts lines 1-15):
```typescript
import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from './client';

const STABLE_TEST_PATH = '/v1/orgs/{org_id}/agents' as const;
const TEST_ORG_ID = '01935b00-0000-7000-8000-000000000099';

describe('createApiClient', () => {
  it('invokes getOrgId per request, not at construction', async () => {
    const getOrgId = vi.fn(() => '...');
    const stubFetch = vi.fn(async () => new Response(...));
    // ...
  });
});
```

**Component test pattern** (from RESEARCH.md §12):
```typescript
import { describe, it, expect, vi } from 'vitest';
import './agent-list.js';  // registers <or-agent-list>

describe('OrAgentList', () => {
  it('renders loading skeleton when task is pending', async () => {
    const el = document.createElement('or-agent-list') as any;
    el.orgId = 'test-org-id';
    el.client = { GET: vi.fn().mockReturnValue(new Promise(() => {})) };
    document.body.appendChild(el);
    await el.updateComplete;  // REQUIRED — await Lit render cycle
    expect(el.shadowRoot?.querySelector('[data-testid="loading"]')).toBeTruthy();
    document.body.removeChild(el);
  });
});
```

**Deviation note:** Component tests use `happy-dom` environment (from vitest.config.ts) and `await el.updateComplete` after every property/event change. Client tests use default environment (Node). Never call `document.createElement` in client.test.ts style tests.

---

### `web/packages/ui/src/components/shell/catalog-shell.ts` (Lit component, router)

**Analog:** NO ANALOG — new pattern. Reference RESEARCH.md §2 for `@lit-labs/router`.

**Router integration pattern** (RESEARCH.md §2):
```typescript
import { Routes } from '@lit-labs/router';
import { LitElement, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';

@customElement('or-catalog-shell')
export class OrCatalogShell extends LitElement {
  @property({ type: String }) theme: 'or-light' | 'or-dark' | 'or-brand' = 'or-light';
  @property({ type: String, attribute: 'org-id' }) orgId = '';

  private _routes = new Routes(this, [
    { path: '/', render: () => html`<or-org-picker></or-org-picker>` },
    { path: '/orgs/:org_id/agents', render: ({ org_id }) => html`<or-agent-list .orgId=${org_id}></or-agent-list>` },
    // ... remaining routes
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

**UUIDv7 route guard** (RESEARCH.md §2):
```typescript
const UUIDV7_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
// enter callback on /orgs/:org_id/* route:
enter: async ({ org_id }) => {
  if (!UUIDV7_PATTERN.test(org_id ?? '')) { this._routes.goto('/'); return false; }
  return true;
},
```

**Theme apply pattern** (RESEARCH.md §8):
```typescript
updated(changed: Map<string, unknown>) {
  if (changed.has('theme')) this._applyTheme();
}
private _applyTheme() {
  const tokens = typeof this.theme === 'string' ? (this._THEME_TOKENS[this.theme] ?? {}) : this.theme;
  for (const [key, val] of Object.entries(tokens)) {
    this.style.setProperty(key, val);
  }
}
```

**Deviation note:** Shell is the ONLY component that constructs `Routes` — `outlet()` MUST stay in shell's `render()`. Sidebar can be a separate `<or-shell-chrome>` component but outlet stays in shell.

---

### `web/packages/ui/src/components/status/status-panel.ts` (event-driven polling)

**Analog:** `web/packages/ui/src/api/task.ts` (Task lifecycle) — partial match

**Polling controller pattern** (RESEARCH.md §3):
```typescript
export class PollingController {
  constructor(host: ReactiveControllerHost, readonly intervalMs: number) {
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
}
```

**createApiTask pattern reused** (task.ts lines 20-34 — existing):
```typescript
return new Task<TArgs, TData>(host, {
  task: (args, options) => config.task(args, options.signal),
  args: config.args,
});
```

**409 status transition pattern** (RESEARCH.md §7):
```typescript
if (error && 'error' in error && (error as any).error === 'invalid_transition') {
  this._currentStatus = (error as any).from;
  this._showTransitionBanner = true;
  // Banner: "Status changed to {from} — your request to go to {to} isn't allowed."
}
```

**Deviation note:** Status panel polls every 5s using `PollingController` + `Task`. Polling PAUSES on `document.hidden` (D6-26). On resume, compare `state_version` (STATE-08) before updating UI. Force-flag checkbox (`force: bool`, D-84) always visible in v0.1.

---

### `web/packages/ui/scripts/gen-validators.mjs` (utility, batch)

**Analog:** NO ANALOG — new pattern. Reference RESEARCH.md §5.

**Core pattern** (RESEARCH.md §5):
```javascript
import Ajv from 'ajv';
import { standaloneCode } from 'ajv/dist/standalone/index.js';
import addFormats from 'ajv-formats';

const ajv = new Ajv({ code: { source: true, esm: true }, strict: false, allErrors: true });
addFormats(ajv);
const validate = ajv.compile(schema);
const code = standaloneCode(ajv, validate);
writeFileSync(`./src/validators/${name}.ts`, code, 'utf8');
```

**Deviation note:** Use `ajv v8` programmatic API — NOT `ajv-cli` (stale, last release 2021). Compile only request-body schemas (13 total: 6×Create + 6×Update + PatchAgentStatusRequest).

---

## Shared Patterns

### Async Task Pattern
**Source:** `web/packages/ui/src/api/task.ts` (full file, lines 1-34)
**Apply to:** All entity list/detail components, status panel polling task

Key rule: `args` arrow function must return reactive properties DIRECTLY — never capture in a variable (RESEARCH.md Pitfall 8):
```typescript
// Correct:
args: () => [this.orgId, this._search, this._cursor] as const,
// Wrong (stale closure):
// const args = [this.orgId]; return () => args;
```

### Error Handling
**Source:** `web/packages/ui/src/api/errors.ts` (lines 51-59 — `isApiError` + lines 73-80 — `parseApiError`)
**Apply to:** All entity detail/form components, status panel, import page

Pattern: check `error && 'current' in error` for version_conflict; check `'error' in error && error.error === 'invalid_transition'` for status 409. Do NOT call `response.json()` if openapi-fetch already parsed `error` (RESEARCH.md Pitfall 9).

### X-Org-Id Client Middleware
**Source:** `web/packages/ui/src/api/client.ts` (lines 45-65)
**Apply to:** Shell (constructs client); all entity components receive `client` as `@property`

Components never call `createApiClient` — they receive it as a prop. Shell creates one client with `getOrgId` reading current route params.

### Shoelace Per-Component Import
**Source:** RESEARCH.md §4 (no codebase analog — new pattern for Phase 6)
**Apply to:** Every Lit component that uses Shoelace elements

Rule: each component imports ONLY what it uses (`import '@shoelace-style/shoelace/dist/components/input/input.js'`). Never import the barrel `@shoelace-style/shoelace`. Violation breaks the Phase 7 70KB gzipped budget.

### i18n String Wrapping
**Source:** RESEARCH.md §11 (no codebase analog)
**Apply to:** All component `render()` methods for user-visible strings

Pattern: wrap every user-facing string with `msg()` from `@lit/localize`:
```typescript
import { msg } from '@lit/localize';
html`<sl-button>${msg('Save changes')}</sl-button>`
```

### Vitest Component Test Lifecycle
**Source:** `web/packages/ui/src/api/client.test.ts` (lines 1-3, 15-19) + RESEARCH.md §12
**Apply to:** All `*.test.ts` component tests in `packages/ui/src/components/`

Rule: always `await el.updateComplete` after property set or event dispatch. Import the component registration file (not the class) so the element is registered in happy-dom's custom element registry.

---

## No Analog Found

Files with no close match in the codebase (planner should use RESEARCH.md patterns instead):

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `web/apps/admin/vite.config.ts` | build config | — | No Vite config exists in repo; first SPA build setup |
| `web/apps/admin/index.html` | scaffold | — | No HTML entry files; first browser entry point |
| `web/apps/admin/playwright.config.ts` | config | — | No E2E test infrastructure exists |
| `web/apps/admin/e2e/smoke.spec.ts` | test | — | No Playwright tests exist; no analog in Go backend |
| `web/packages/ui/src/components/shell/catalog-shell.ts` | Lit component | request-response | No router or shell component in codebase |
| `web/packages/ui/src/components/primitives/*.ts` (6 files) | Lit component | — | No Lit components exist anywhere in codebase |
| `web/packages/ui/src/themes/*.css` (3 files) | theme | — | No CSS theme files exist |
| `web/packages/ui/scripts/gen-validators.mjs` | utility | batch | No codegen scripts beyond `gen:api` (which is a CLI invocation, not a custom script) |
| `web/packages/ui/lit-localize.json` | config | — | No i18n infrastructure exists |
| `web/packages/ui/src/components/imports/import-page.ts` | Lit component | file-I/O | No file upload UI exists anywhere |
| `web/packages/ui/src/components/status/status-panel.ts` | Lit component | event-driven | No polling UI component exists; only a polling backend concept |

---

## Metadata

**Analog search scope:** `web/packages/ui/src/`, `web/apps/admin/src/`, `web/turbo.json`, `web/tsconfig.base.json`
**Files scanned:** 9 existing files (all files in Phase 2 codebase)
**Analogs from codebase:** 8 analog targets found; all from Phase 2 API surface
**Pattern extraction date:** 2026-05-17

**Key takeaway for planner:** Phase 6 is the first significant frontend phase. The codebase's Phase 2 contribution is entirely the API layer (`client.ts`, `errors.ts`, `task.ts`). All Lit component patterns, Vite config, Shoelace integration, ajv codegen, and i18n must be sourced from RESEARCH.md — not the codebase. The three Phase 2 files ARE real analogs for: (1) typed HTTP client usage in components, (2) error handling/type-guard patterns, (3) async Task lifecycle that entity components extend.
