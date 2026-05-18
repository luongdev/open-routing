# Phase 7: Web Component Embed Bundle - Pattern Map

**Mapped:** 2026-05-18
**Files analyzed:** 25 (new + modified files; embed app + shell refactor + Go CORS + CI)
**Analogs found:** 18 / 25 strong analogs (Phase 6 + Phase 1 codebase); 7 are new territory (no codebase analog, research-derived)

---

## File Classification

### Wave 0 — Build Scaffold + Test Infrastructure

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/apps/embed/package.json` (modify) | package-json | — | `web/apps/admin/package.json` | exact (role-match) |
| `web/apps/embed/vite.config.ts` | build-config | — | `web/apps/admin/vite.config.ts` | partial (library mode = new) |
| `web/apps/embed/tsconfig.json` (extend) | build-config | — | `web/apps/embed/tsconfig.json` (existing skeleton) | exact |
| `web/apps/embed/vitest.config.ts` | build-config | — | `web/packages/ui/vitest.config.ts` | exact |
| `web/apps/embed/test-setup.ts` | build-config | — | `web/packages/ui/src/test-setup.ts` | exact |
| `web/apps/embed/playwright.config.ts` | build-config | — | `web/apps/admin/playwright.config.ts` | partial (multi-project = new) |
| `web/apps/embed/.size-limit.json` | build-config | — | NO ANALOG — research-derived | — |
| `web/apps/embed/index.html` (dev) | build-config | — | `web/apps/admin/index.html` | role-match |

### Wave 1 — Custom Element + Hash Router

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/apps/embed/src/index.ts` (modify) | custom-element | request-response | `web/apps/admin/src/index.ts` | exact (locale bootstrap) |
| `web/apps/embed/src/embed-element.ts` | custom-element | event-driven | `web/packages/ui/src/components/shell/catalog-shell.ts` + `conflict-banner.ts` | partial (LitElement + composed events) |
| `web/apps/embed/src/hash-router-adapter.ts` | router-adapter | event-driven | NO ANALOG — research-derived | — |
| `web/apps/embed/src/embed-element.test.ts` | vitest-spec | — | `web/packages/ui/src/components/shell/catalog-shell.test.ts` + `agents/agent-list.test.ts` | exact |
| `web/apps/embed/src/hash-router-adapter.test.ts` | vitest-spec | — | `web/packages/ui/src/components/shell/catalog-shell.test.ts` | role-match |

### Wave 2 — Shell Refactor (in-place, packages/ui)

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/packages/ui/src/components/shell/catalog-shell.ts` (MODIFY in place) | lit-component | request-response | `web/packages/ui/src/components/shell/catalog-shell.ts` (self — refactor target) | exact |
| `web/packages/ui/src/components/shell/catalog-shell.test.ts` (extend) | vitest-spec | — | `web/packages/ui/src/components/shell/catalog-shell.test.ts` | exact |
| `<sl-dialog>` audit replacements (8 files in `packages/ui/src/components/*/`) | lit-component | event-driven | `web/packages/ui/src/components/primitives/conflict-banner.ts` (inline alert pattern) | role-match |

### Wave 2 — Playwright Stub Hosts + Specs

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/apps/embed/e2e/hosts/react/index.html` | playwright-spec | — | NO ANALOG — research-derived (importmap stub) | — |
| `web/apps/embed/e2e/hosts/vue/index.html` | playwright-spec | — | NO ANALOG — research-derived | — |
| `web/apps/embed/e2e/hosts/html/index.html` | playwright-spec | — | NO ANALOG — research-derived | — |
| `web/apps/embed/e2e/specs/embed-shadow-dom-isolation.spec.ts` | playwright-spec | event-driven | `web/apps/admin/e2e/smoke.spec.ts` | partial (shadow piercing pattern) |
| `web/apps/embed/e2e/specs/embed-org-id-header.spec.ts` | playwright-spec | request-response | `web/apps/admin/e2e/smoke.spec.ts` | partial (page.route intercept = new) |
| `web/apps/embed/e2e/specs/embed-auth-expired.spec.ts` | playwright-spec | event-driven | `web/apps/admin/e2e/smoke.spec.ts` | partial |
| `web/apps/embed/e2e/specs/embed-modules-filter.spec.ts` | playwright-spec | request-response | `web/apps/admin/e2e/smoke.spec.ts` | partial |

### Wave 0/3 — Backend (Go CORS)

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `services/api/internal/middleware/cors.go` | go-middleware | request-response | `services/api/internal/middleware/orgcontext.go` + `bodylimit.go` | exact (chi MiddlewareFunc factory) |
| `services/api/internal/middleware/cors_test.go` | go-test | — | `services/api/internal/middleware/orgcontext_test.go` + `bodylimit_test.go` | exact |
| `services/api/internal/server/server.go` (MODIFY chain) | go-middleware | request-response | `services/api/internal/server/server.go` (self — modify NewMux chain) | exact |
| `services/api/internal/config/config.go` (MODIFY add env) | go-middleware | — | `services/api/internal/config/config.go` (self — extend) | exact |
| `services/api/test/isolation/cors_preflight_test.go` | go-test | request-response | `services/api/test/isolation/isolation_test.go` | exact |
| `services/api/test/isolation/cors_cross_origin_test.go` | go-test | request-response | `services/api/test/isolation/isolation_test.go` | exact |

### Wave 3 — CI + Distribution

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `.github/workflows/ci.yml` (NEW file) | ci-yaml | — | NO ANALOG — no ci.yml exists yet in repo | — |
| `web/apps/embed/README.md` | readme | — | NO ANALOG (no admin README); research-derived | — |
| `web/apps/embed/LICENSE` | readme | — | NO ANALOG — research-derived | — |
| `web/apps/embed/CHANGELOG.md` | readme | — | NO ANALOG — research-derived | — |

---

## Pattern Assignments

### `web/apps/embed/package.json` (modify — fill in scripts + deps)

**Analog:** `web/apps/admin/package.json` (full file, 30 lines)

**Pattern excerpt** (admin/package.json lines 1-30):
```json
{
  "name": "@open-routing/admin",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "typecheck": "tsc --noEmit",
    "lint": "eslint src --max-warnings 0",
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:e2e": "playwright test"
  },
  "dependencies": {
    "@lit-labs/router": "^0.1.4",
    "@lit/localize": "^0.12.2",
    "@open-routing/ui": "workspace:*",
    "@shoelace-style/shoelace": "^2.20.1",
    "lit": "^3.3.3"
  },
  "devDependencies": {
    "@babel/plugin-proposal-decorators": "^7.29.0",
    "@playwright/test": "^1.60.0",
    "@rolldown/plugin-babel": "^0.2.3",
    "urlpattern-polyfill": "^10.1.0",
    "vite": "^8.0.13",
    "vitest": "^3.2.4"
  }
}
```

**Deviation note:** Embed differs in 5 ways: (1) `name: "@open-routing/catalog-embed"` (publishable name per D7-15); (2) `version: "0.1.0-pre.1"` (real version, not `0.0.0`); (3) add `main`, `exports`, `files`, `sideEffects: ["./dist/embed.js"]` per D7-15 npm tarball contract; (4) add `size`, `pack:dry` scripts; (5) add `size-limit` + `@size-limit/preset-app` + `happy-dom` to devDependencies. **Critical:** `@open-routing/ui` MUST be moved to `devDependencies` (not `dependencies`) so Vite library mode bundles it instead of externalizing (Pitfall 8). Strip `@lit-labs/router`, `@lit/localize`, `@shoelace-style/shoelace` from `dependencies` — they come transitively through bundled `@open-routing/ui`.

---

### `web/apps/embed/vite.config.ts` (NEW — library mode)

**Analog:** `web/apps/admin/vite.config.ts` (lines 1-30, full file)

**Babel decorator config from admin** (lines 1-29):
```typescript
import { defineConfig } from 'vite';

export default defineConfig({
  appType: 'spa',
  server: {
    proxy: {
      '/v1': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      '/readyz': 'http://localhost:8080',
    },
  },
  build: {
    target: 'esnext',
    rollupOptions: {
      output: {
        // Rolldown (Vite 8) requires manualChunks as a function, not an object.
        manualChunks: (id: string) => {
          if (id.includes('@shoelace-style/shoelace')) return 'shoelace';
          if (
            id.includes('/node_modules/lit/') ||
            id.includes('/node_modules/@lit/reactive-element/') ||
            id.includes('/node_modules/@lit/task/')
          )
            return 'lit';
        },
      },
    },
  },
});
```

**Deviation note:** Embed is library mode, not SPA. Differences: (1) `appType: 'spa'` → REMOVED (library mode); (2) no `server.proxy` (embed has no dev server proxy — Playwright stub hosts mock the API via `page.route`); (3) **add** `@rolldown/plugin-babel` plugin for TC39 2023-11 decorators (admin's vite.config.ts is missing this; embed uses the plugin pattern from RESEARCH §1); (4) **add** `build.lib: { entry, formats: ['es'], fileName: () => 'embed.js' }` for single-ES-module output; (5) **add** `build.sourcemap: true` (external `.map` files per CONTEXT Discretion); (6) **add** `build.cssCodeSplit: false`; (7) **drop** the `manualChunks` function — natural dynamic-import code splitting per route handles chunking (Pitfall 2: don't define manualChunks for library mode). Pattern source: RESEARCH §1.

**Reuse pattern X:** Babel decorator preset config (admin pioneered, embed copies). **Differs in Y way:** Embed swaps SPA mode for library mode entry config.

---

### `web/apps/embed/tsconfig.json` (extend existing skeleton)

**Analog:** `web/apps/embed/tsconfig.json` (existing, 9 lines) + `web/tsconfig.base.json`

**Existing skeleton** (full file):
```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "rootDir": "src",
    "outDir": "dist"
  },
  "include": ["src"]
}
```

**Base config** (`web/tsconfig.base.json`, full file):
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

**Deviation note:** Phase 6 admin and packages/ui both extend base with `experimentalDecorators: false` + `useDefineForClassFields: true` (TC39 2023-11 standard decorators with `accessor` keyword, NOT legacy). Embed uses identical compiler options. The existing skeleton needs `experimentalDecorators` and `useDefineForClassFields` carefully set so Lit `accessor`-based decorator pattern works (RESEARCH §2 note: "the `accessor` keyword IS REQUIRED on every `@property` field"). **Reuse pattern X.**

---

### `web/apps/embed/vitest.config.ts` (NEW)

**Analog:** `web/packages/ui/vitest.config.ts` (full file, 12 lines)

**Pattern excerpt** (full file):
```typescript
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'happy-dom',
    setupFiles: ['./src/test-setup.ts'],
    include: ['src/**/*.test.ts'],
    reporters: ['verbose'],
    globals: false,
  },
});
```

**Deviation note:** Same shape. Embed's setupFiles path is `./test-setup.ts` (root) per RESEARCH §Code Examples, not `./src/test-setup.ts`. The polyfill content is the same (urlpattern + Animation). **Reuse pattern X.**

---

### `web/apps/embed/test-setup.ts` (NEW)

**Analog:** `web/packages/ui/src/test-setup.ts` (full file, 30 lines)

**Pattern excerpt** (lines 1-29):
```typescript
// urlpattern-polyfill is needed in Node/happy-dom test environments for @lit-labs/router
// route parsing. All major browsers support URLPattern natively as of 2025 (Baseline 2025).
// Do NOT import this in any component source file — only in test setup.
import 'urlpattern-polyfill';

// Shoelace uses the Web Animations API (Element.getAnimations, Element.animate)
// for dialog/overlay transitions. happy-dom does not implement these.
if (typeof Element !== 'undefined') {
  if (!Element.prototype.getAnimations) {
    Element.prototype.getAnimations = function () { return []; };
  }
  if (!Element.prototype.animate) {
    Element.prototype.animate = function () {
      return { finished: Promise.resolve(), cancel: () => {}, finish: () => {},
               play: () => {}, pause: () => {}, reverse: () => {},
               addEventListener: () => {}, removeEventListener: () => {},
             } as unknown as Animation;
    };
  }
}
```

**Deviation note:** Embed file should be identical for v0.1 (D7-04 mandates inline alerts, so animate-shim coverage for Shoelace dialogs is still needed for the **shell's** transitive component tests). **Reuse pattern X verbatim.**

---

### `web/apps/embed/playwright.config.ts` (NEW — 3-project matrix)

**Analog:** `web/apps/admin/playwright.config.ts` (full file, 37 lines)

**Pattern excerpt** (lines 8-37):
```typescript
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.ts',
  timeout: 30_000,
  retries: 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  webServer: {
    command: 'pnpm --filter @open-routing/admin dev',
    url: 'http://localhost:5173',
    reuseExistingServer: true,
    timeout: 30_000,
  },
});
```

**Deviation note:** Embed scales `projects` to 3 entries (`react`, `vue`, `html`) each pointing at a different baseURL path (`/hosts/react/`, `/hosts/vue/`, `/hosts/html/`) per RESEARCH §9. Embed changes 4 ways: (1) `retries: 1` per D7-12 CDN-flake tolerance; (2) `testMatch: 'specs/*.spec.ts'`; (3) `baseURL: 'http://localhost:4173'` (Vite preview port, not dev port); (4) `webServer.command: 'pnpm --filter @open-routing/embed preview'` + `timeout: 60_000` (Vite build before preview). **Reuse pattern X (config skeleton); differs in projects matrix + preview-server command.**

---

### `web/apps/embed/.size-limit.json` (NEW — declarative budget)

**Analog:** NO ANALOG — first usage of size-limit in repo. Reference RESEARCH §Code Examples + D7-09.

**Pattern to write** (from RESEARCH §Code Examples, lines 951-975):
```json
[
  {
    "name": "embed.js (eager baseline)",
    "path": "dist/embed.js",
    "limit": "70 KB",
    "gzip": true
  },
  {
    "name": "embed-agents-*.js (lazy chunk)",
    "path": "dist/embed-agents-*.js",
    "limit": "15 KB",
    "gzip": true,
    "running": false
  }
]
```

**Deviation note:** **New territory — research-derived.** D7-10 separates hard-gate eager baseline (70 KB) from soft-budget lazy chunks (15 KB each, logged but not blocking). Implementation: per RESEARCH §Code Examples §note, set lazy-chunk `limit: "20 KB"` to give headroom OR emit a separate `.size-limit-soft.json` for non-required CI. Recommend: ONE `.size-limit.json` with eager-only hard gate; lazy chunks logged via PR comment from same action but in a separate JSON entry that operates with `running: false` to skip runtime perf check.

---

### `web/apps/embed/index.html` (dev entry)

**Analog:** `web/apps/admin/index.html` (full file, 12 lines)

**Pattern excerpt** (full file):
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

**Deviation note:** Embed dev HTML mounts `<open-routing-catalog org-id="..." api-base-url="..." theme="or-light">` instead of `<or-catalog-shell>` for `vite dev` testing. Production build does NOT emit this HTML (library mode only emits dist/embed.js). **Reuse pattern X (dev-only); embed never ships HTML.**

---

### `web/apps/embed/src/index.ts` (modify — registration + locale bootstrap)

**Analog:** `web/apps/admin/src/index.ts` (full file, 30 lines)

**Pattern excerpt** (lines 1-29):
```typescript
import { configureLocalization } from '@lit/localize';
import { sourceLocale, targetLocales } from '@open-routing/ui/locales/locale-codes.js';
import '@open-routing/ui/components/shell'; // registers <or-catalog-shell>

type AppLocale = typeof sourceLocale | (typeof targetLocales)[number];
const VALID_LOCALES: ReadonlySet<string> = new Set([sourceLocale, ...targetLocales]);

const { setLocale } = configureLocalization({
  sourceLocale,
  targetLocales,
  loadLocale: (locale: string) =>
    import(`@open-routing/ui/locales/${locale}.js`) as unknown as ReturnType<
      Parameters<typeof configureLocalization>[0]['loadLocale']
    >,
});

const saved = localStorage.getItem('or-locale');
const detected: AppLocale = navigator.language.startsWith('vi') ? 'vi' : 'en';
const locale: AppLocale =
  saved !== null && VALID_LOCALES.has(saved) ? (saved as AppLocale) : detected;
await setLocale(locale);
```

**Deviation note:** Embed reuses the `configureLocalization` shape but with 3 changes: (1) **no `localStorage` access** — locale comes from `<open-routing-catalog locale="en|vi">` attribute (D6-23 carry-forward doesn't apply per CONTEXT.md `<out_of_scope>`); (2) **add** `import './embed-element.js'` to register the Custom Element class; (3) initial `setLocale()` call happens before module export resolution completes, so embed-element reads from a Reactive Property at `connectedCallback` rather than at module load. **Reuse pattern X (configureLocalization shape); differs in attribute-driven locale + Custom Element registration.**

---

### `web/apps/embed/src/embed-element.ts` (NEW — LitElement Custom Element)

**Analog (multi):** `web/packages/ui/src/components/shell/catalog-shell.ts` (lines 122-243 — shell LitElement + reactive properties + connectedCallback) AND `web/packages/ui/src/components/primitives/conflict-banner.ts` (lines 200-214 — composed CustomEvent dispatch pattern for inline alert).

**Shell LitElement structure** (catalog-shell.ts lines 122-210):
```typescript
@customElement('or-catalog-shell')
export class OrCatalogShell extends LitElement {
  static override styles = css`
    :host { display: flex; flex-direction: column; height: 100vh; }
    /* ... */
  `;

  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String }) accessor theme: ThemeName = 'or-light';
  @property({ type: String }) accessor modules = '';
  @property({ type: String }) accessor locale: 'en' | 'vi' = 'en';

  @state() private accessor _currentOrgId = '';
  @state() private accessor _client: ApiClient | null = null;
  /* ... */
}
```

**`accessor` decorator pattern** — REQUIRED for TC39 2023-11 standard decorators under Vite 8 + Rolldown. Phase 6 shell proves this works in production. Drop `accessor` and reactivity silently breaks.

**Composed CustomEvent dispatch** (conflict-banner.ts lines 206-214):
```typescript
private _dispatch(action: 'review' | 'discard'): void {
  this.dispatchEvent(
    new CustomEvent('open-routing:conflict-acknowledged', {
      detail: { action },
      bubbles: true,
      composed: true,  // crosses Shadow DOM for Phase 7
    })
  );
}
```

**Shell theme-on-host pattern** (catalog-shell.ts lines 467-481):
```typescript
override updated(changed: Map<string, unknown>): void {
  if (changed.has('theme')) {
    this._applyTheme();
  }
}

private _applyTheme(): void {
  for (const key of ALL_TOKEN_KEYS) {
    this.style.removeProperty(key);
  }
  const tokens = _THEME_TOKENS[this.theme] ?? {};
  for (const [key, val] of Object.entries(tokens)) {
    this.style.setProperty(key, val);
  }
}
```

**UUIDv7 validation** (catalog-shell.ts line 72):
```typescript
const UUIDV7_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
```

**Deviation note:** Embed combines THREE patterns into one element: (1) LitElement attribute-reactive properties from shell; (2) inline-banner state + composed CustomEvent dispatch from conflict-banner; (3) UUIDv7 regex validation from shell (same constant, copied verbatim per D7-08). Embed adds 3 NEW patterns from RESEARCH §2 (no codebase analog): (a) `_validateAndBuildClient()` calls `createApiClient` factory directly from element instance (per-element client lifecycle); (b) `_dispatchRequestContext()` on `connectedCallback` (announce-only, fires once — D7-13); (c) `onResponse` middleware on the client to intercept 401 + dispatch `open-routing:auth-expired` (D7-14, RESEARCH §7). **Reuse 3 patterns from Phase 6; new territory = client-per-element lifecycle + 401 middleware wiring.**

---

### `web/apps/embed/src/hash-router-adapter.ts` (NEW)

**Analog:** NO ANALOG — research-derived. Reference RESEARCH §4.

**Pattern to write** (from RESEARCH §4, lines 525-552):
```typescript
import type { Routes } from '@lit-labs/router';

export class HashRouterAdapter {
  private _onHashChange: () => void;

  constructor(private _routes: Routes) {
    this._onHashChange = () => this._navigate();
  }

  start(): void {
    window.addEventListener('hashchange', this._onHashChange);
    this._navigate();
  }

  stop(): void {
    window.removeEventListener('hashchange', this._onHashChange);
  }

  private _navigate(): void {
    const hash = window.location.hash.slice(1) || '/';
    this._routes.goto(hash);
  }
}
```

**Deviation note:** **New territory — research-derived.** Closest in-codebase reference is the shell's own `popstate` listener (catalog-shell.ts lines 413-414, 429-431): same shape (DOM event subscribe in connect, unsubscribe in disconnect, navigate on event), different event (`hashchange` vs `popstate`) and different parse target (`location.hash.slice(1)` vs `location.pathname`). Add hash-prefix guard per RESEARCH §Pitfall 9 — adapter ignores hashes that don't start with `open-routing/` so it coexists with host hash routing.

---

### `web/apps/embed/src/embed-element.test.ts` (NEW)

**Analog:** `web/packages/ui/src/components/shell/catalog-shell.test.ts` (lines 1-60) + `web/packages/ui/src/components/agents/agent-list.test.ts` (lines 1-80)

**Shell test structure** (catalog-shell.test.ts lines 1-30):
```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './catalog-shell.js';

describe('OrCatalogShell', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-catalog-shell');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) {
      el.parentNode.removeChild(el);
    }
  });

  it('sets --sl-color-primary-500 to #4faab2 when theme is or-dark', async () => {
    (el as any).theme = 'or-dark';
    await (el as any).updateComplete;
    expect((el as HTMLElement).style.getPropertyValue('--sl-color-primary-500')).toBe('#4faab2');
  });
});
```

**Agent-list async-state test** (agent-list.test.ts lines 19-52):
```typescript
describe('OrAgentList', () => {
  let el: HTMLElement;
  beforeEach(() => { el = document.createElement('or-agent-list'); document.body.appendChild(el); });
  afterEach(() => { if (el.parentNode) el.parentNode.removeChild(el); });

  it('renders or-data-table with rows when client.GET resolves successfully', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_AGENT], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;
    /* ... */
  });
});
```

**Deviation note:** Embed-element tests follow the same `beforeEach/afterEach` mount-unmount pattern, but assert different things: (1) invalid `org-id` → renders inline `<div class="error">` (D7-08); (2) `open-routing:request-context` fires on `connectedCallback` with `composed: true, bubbles: true` (D7-13); (3) `open-routing:auth-expired` fires on every 401 with `requestId, path` in detail (D7-14); (4) theme JSON parse + forward to shell (D7-15, RESEARCH §5); (5) attribute change re-validates without remount. **Reuse pattern X (vitest mount-unmount + `await el.updateComplete` lifecycle); differs in event-dispatch assertions instead of render assertions.**

---

### `web/packages/ui/src/components/shell/catalog-shell.ts` (MODIFY in place — lazy routes + routingMode)

**Analog:** `web/packages/ui/src/components/shell/catalog-shell.ts` (self — refactor target, 588 lines)

**Existing static-import block** (lines 24-69) — convert ALL to lazy:
```typescript
// Agent components (Wave 2, Plan 06-05) — wired to real routes in Plan 06-06.
import '../agents/agent-list.js';
import '../agents/agent-detail.js';
import '../agents/agent-form.js';
/* ... 27 more imports ... */
import '../imports/import-page.js';
import '../imports/import-result.js';
```

**Existing route definition** (lines 252-257) — wrap `enter` to compose guard + lazy import:
```typescript
{
  path: '/orgs/:org_id/agents',
  enter: this._orgRouteEnter,
  render: ({ org_id }: Record<string, string | undefined>) =>
    html`<or-agent-list .orgId=${org_id ?? ''} .client=${this._client!}></or-agent-list>`,
},
```

**Refactored pattern** (from RESEARCH §3, lines 482-491):
```typescript
private _agentListRoute = {
  path: '/orgs/:org_id/agents',
  enter: async (params: Record<string, string | undefined>) => {
    if (!(await this._orgRouteEnter(params))) return false;
    await import('../agents/agent-list.js');
    return true;
  },
  render: ({ org_id }: Record<string, string | undefined>) =>
    html`<or-agent-list .orgId=${org_id ?? ''} .client=${this._client!}></or-agent-list>`,
};
```

**Add routingMode reactive property + typed routerAdapter injection seam** (RESEARCH §3, lines 498-510; Iteration-2 BLOCKER #1 refactor):
```typescript
import type { Routes } from '@lit-labs/router';

// Iteration-2 BLOCKER #1 — typed seam for hash-routing injection.
// Defined in packages/ui so consumers (apps/embed) implement against
// the contract without packages/ui needing a static dep on
// @open-routing/embed. Symmetric coupling: shell hands Routes via
// adapter.start(routes); adapter never reaches into shell.
export interface CatalogShellRouterAdapter {
  start(routes: Routes): void;
  stop(): void;
}

@property({ type: String, attribute: 'routing-mode' })
accessor routingMode: 'history' | 'hash' = 'history';

// Iteration-2 BLOCKER #1 — public injection property. Consumers
// assign BEFORE flipping routingMode='hash'. attribute: false because
// adapter is a class instance, not a string attribute.
@property({ attribute: false })
accessor routerAdapter: CatalogShellRouterAdapter | undefined = undefined;

// In firstUpdated (NOT connectedCallback — Routes instance must be
// fully constructed at this point):
override firstUpdated(_changed: PropertyValues): void {
  if (this.routingMode === 'hash' && this.routerAdapter) {
    this.routerAdapter.start(this._routes);
  }
}

// In disconnectedCallback:
this.routerAdapter?.stop();
```

The adapter (HashRouterAdapter in apps/embed) is structurally
compatible with `CatalogShellRouterAdapter` — no `implements` clause
in the adapter is required (TS verifies shape at consumer's
assignment site).

```typescript
// In apps/embed/src/embed-element.ts (Plan 07-06):
override firstUpdated(_changed: PropertyValues): void {
  const shellEl = this._shellEl;
  if (!shellEl || this._hashAdapter) return;
  this._hashAdapter = new HashRouterAdapter();
  shellEl.routerAdapter = this._hashAdapter;     // assign BEFORE flip
  shellEl.routingMode = 'hash';                   // flip triggers shell.firstUpdated
}
```

**Deviation note:** **Self-refactor** — same file, ~30 lines deleted + new `lazyRoute()` helper inline. Critical constraint per D7-03: must keep history mode behavior intact for admin smoke (Plan 06-06). Per Pitfall 7: `routingMode` is set-once at construction; warn if changed after `connectedCallback`. **No external analog; this is the largest in-place modification of Phase 7.**

---

### `<sl-dialog>` Audit Replacement (8 files in `packages/ui/src/components/*/`)

**Analog:** `web/packages/ui/src/components/primitives/conflict-banner.ts` (full file, 334 lines — inline alert pattern)

**Inline-alert state pattern** (conflict-banner.ts lines 196-204):
```typescript
override connectedCallback(): void {
  super.connectedCallback();
  // Set aria-live on the host so screen readers announce the banner immediately.
  this.setAttribute('aria-live', 'assertive');
  this.setAttribute('role', 'alert');
}
```

**Inline-banner render** (conflict-banner.ts lines 251-285):
```typescript
private _renderCrudMode() {
  return html`
    <div class="conflict-banner">
      <div class="banner-header">
        <sl-icon class="banner-icon" name="exclamation-triangle"></sl-icon>
        <div>
          <p class="banner-heading">This was changed elsewhere</p>
          <p class="banner-body">Your edits are below — review the diff and re-submit, or discard
            your changes and load the server's version.</p>
        </div>
      </div>
      ${when(this.showDiff, () => this._renderCrudDiff())}
      <div class="action-row">
        <sl-button variant="primary" size="small" @click=${() => this._dispatch('review')}>
          Review and re-submit
        </sl-button>
        <sl-button variant="text" size="small" @click=${() => this._dispatch('discard')}>
          Discard my changes
        </sl-button>
      </div>
    </div>
  `;
}
```

**Deviation note:** Conflict-banner is the **existing v0.1 inline alert template**. The 8 files using `<sl-dialog>` (adapter-detail, agent-detail, break-reason-detail, channel-detail, queue-detail, skill-detail [2], status-panel per RESEARCH §Runtime State Inventory) each need a confirm-panel inline replacement that follows the same shape: aria-live host, inline div, action-row buttons, CustomEvent dispatch on action click. Per D7-04, focus-trap inside nested Shadow DOM is broken for `<sl-dialog>` (shoelace#709) — replace, don't accept. **Reuse pattern X verbatim; new instances per file.**

---

### Playwright Stub Hosts — `web/apps/embed/e2e/hosts/{react,vue,html}/index.html`

**Analog:** NO ANALOG — research-derived. Reference RESEARCH §9.

**Pattern to write** (RESEARCH §9, lines 705-741, React stub):
```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>React host (embed test)</title>
  <script type="importmap">
    {
      "imports": {
        "react": "https://esm.sh/react@18.3.1",
        "react-dom/client": "https://esm.sh/react-dom@18.3.1/client"
      }
    }
  </script>
  <style>
    * { all: revert; box-sizing: border-box; }
    body { font-family: serif; background: lime; color: red; }
  </style>
</head>
<body>
  <h1 style="font-size: 200%; color: purple;">Host CSS poison</h1>
  <div id="root"></div>
  <script type="module" src="/embed.js"></script>
  <script type="module">
    import React from 'react';
    import { createRoot } from 'react-dom/client';
    const App = () => React.createElement('open-routing-catalog', {
      'org-id': '01952a6b-1c00-7000-8000-000000000001',
      'api-base-url': 'http://localhost:4173',
      theme: 'or-light',
      modules: 'agents,skills',
    });
    createRoot(document.getElementById('root')).render(React.createElement(App));
  </script>
</body>
</html>
```

**Deviation note:** **New territory — research-derived.** Vue stub uses `vue@3.5.13` via esm.sh; HTML stub omits the framework script + creates the element directly. All 3 hosts share the aggressive `* { all: revert }` host CSS reset to make EMBED-04 Shadow DOM isolation assertions meaningful. CDN flake mitigated by D7-12 `retries: 1`.

---

### Playwright Specs — `e2e/specs/embed-*.spec.ts` (4 files)

**Analog:** `web/apps/admin/e2e/smoke.spec.ts` (lines 1-100 — shadow piercing pattern + test.describe block)

**Test imports + describe** (smoke.spec.ts lines 16-30):
```typescript
import { test, expect } from '@playwright/test';

const TEST_ORG_ID =
  process.env.TEST_ORG_ID ?? '01952a6b-1c00-7000-8000-000000000001';

test.describe('Admin SPA smoke test', () => {
  test('org-picker → enter UUID → agents list loads', async ({ page }) => {
    await page.goto('/');
    const orgPickerHeading = page.locator('or-org-picker').first();
    await expect(orgPickerHeading).toBeAttached({ timeout: 10_000 });
    /* ... */
  });
});
```

**Shadow piercing pattern** (smoke.spec.ts lines 41-72):
```typescript
const orgPicker = page.locator('or-catalog-shell or-org-picker');
await expect(orgPicker).toBeAttached({ timeout: 5_000 });

// Access shadow DOM via evaluate
await page.evaluate((orgId: string) => {
  const shell = document.querySelector('or-catalog-shell');
  const shellShadow = shell?.shadowRoot;
  const picker = shellShadow?.querySelector('or-org-picker');
  const pickerShadow = picker?.shadowRoot;
  const nativeInput = pickerShadow?.querySelector('input');
  if (nativeInput) {
    nativeInput.value = orgId;
    nativeInput.dispatchEvent(new Event('input', { bubbles: true, composed: true }));
  }
}, TEST_ORG_ID);
```

**Deviation note:** Embed specs reuse the admin smoke shape but pierce a DEEPER shadow root chain: `<open-routing-catalog>.shadowRoot → <or-catalog-shell>.shadowRoot → <or-agent-list>.shadowRoot → ...`. Each spec asserts a different EMBED-XX requirement:
- `embed-shadow-dom-isolation.spec.ts` — assert host `body` CSS doesn't bleed into shadow root; assert embed `:host` styles don't escape (EMBED-04, RESEARCH §VA Wave 0)
- `embed-org-id-header.spec.ts` — `page.route('**/v1/**')` intercepts API; assert `X-Org-Id: 01952a6b-...` header on every request (EMBED-02)
- `embed-auth-expired.spec.ts` — `page.route` mocks 401 + `request_id: 'test-rid-123'`; assert `open-routing:auth-expired` event fires on host listener (EMBED-07, D7-14)
- `embed-modules-filter.spec.ts` — pierce shell shadow root, assert sidebar `<nav>` only shows entries in `modules=` attribute (EMBED-06)

**Reuse pattern X (Playwright structure + Shadow DOM piercing via `page.evaluate`); new territory = `page.route` API mocking.**

---

### `services/api/internal/middleware/cors.go` (NEW)

**Analog:** `services/api/internal/middleware/orgcontext.go` (lines 48-79 — factory shape) AND `services/api/internal/middleware/bodylimit.go` (lines 38-60 — factory-returning-middleware pattern)

**OrgContext middleware factory** (orgcontext.go lines 48-79):
```go
func OrgContext(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        raw := r.Header.Get("X-Org-Id")
        if raw == "" {
            WriteError(ctx, w, http.StatusBadRequest, "invalid_org_id", "missing_header")
            return
        }
        id, err := uuid.Parse(raw)
        if err != nil {
            WriteError(ctx, w, http.StatusBadRequest, "invalid_org_id", "malformed_uuid")
            return
        }
        if id.Version() < 7 {
            WriteError(ctx, w, http.StatusBadRequest, "invalid_org_id", "uuidv7_required")
            return
        }
        ctx = orgkey.SetOrgID(r.Context(), id)
        span := trace.SpanFromContext(ctx)
        span.SetAttributes(attribute.String("org_id", id.String()))
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**BodyLimit factory-returning-middleware** (bodylimit.go lines 44-60):
```go
// BodyLimit returns a chi MiddlewareFunc that rejects requests whose
// declared Content-Length exceeds maxBytes ...
// The factory shape (function returning a middleware) matches chi
// MiddlewareFunc and the existing requestid.go / uuidv7path.go shapes.
```

**Pattern to write** (from RESEARCH §Code Examples + D7-16):
```go
package middleware

import (
    "net/http"
    "strings"
    "github.com/go-chi/cors"
)

// AllowedOriginsFromEnv parses CORS_ALLOWED_ORIGINS into a list of origins.
// Empty input returns empty list (reject all); dev sets "*".
func AllowedOriginsFromEnv(raw string) []string {
    if raw == "" { return []string{} }
    parts := strings.Split(raw, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        if trimmed := strings.TrimSpace(p); trimmed != "" {
            out = append(out, trimmed)
        }
    }
    return out
}

// NewCORS returns a chi MiddlewareFunc that handles CORS preflight + headers
// per D7-16. cors.Handler short-circuits OPTIONS preflight automatically
// (returns 200 OK + headers, does NOT call next.ServeHTTP) — verified in
// github.com/go-chi/cors source. This is why ordering in the chain is safe
// BEFORE OrgContext.
func NewCORS(allowedOrigins []string) func(http.Handler) http.Handler {
    return cors.Handler(cors.Options{
        AllowedOrigins:   allowedOrigins,
        AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"X-Org-Id", "Content-Type", "Idempotency-Key", "Accept-Language"},
        ExposedHeaders:   []string{"X-Request-Id"},
        AllowCredentials: false,
        MaxAge:           300,
    })
}
```

**Deviation note:** **Reuse pattern X (chi MiddlewareFunc factory shape) verbatim from orgcontext/bodylimit pattern.** New territory = the `cors.Options` struct field set is library-specific. Critical: D7-16 places CORS BEFORE OrgContext in the chain. **Differs in Y way:** no `WriteError` use (cors library writes its own preflight 200); no ctx mutation (cors doesn't touch ctx); factory returns library-built handler instead of hand-rolled.

---

### `services/api/internal/middleware/cors_test.go` (NEW)

**Analog:** `services/api/internal/middleware/orgcontext_test.go` (full file, 162 lines) AND `services/api/internal/middleware/bodylimit_test.go` (lines 1-70 — test header)

**OrgContext test structure** (orgcontext_test.go lines 15-46):
```go
func failNextHandler(t *testing.T) http.Handler {
    t.Helper()
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        t.Fatalf("next handler must NOT be invoked when OrgContext rejects the request")
    })
}

func decodeErrorBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
    t.Helper()
    require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
    var body map[string]string
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
    return body
}

func TestOrgContext_MissingHeader400(t *testing.T) {
    t.Parallel()
    h := OrgContext(failNextHandler(t))
    req := httptest.NewRequest(http.MethodGet, "/v1/orgs/x/_scaffold", nil)
    rec := httptest.NewRecorder()
    h.ServeHTTP(rec, req)
    require.Equal(t, http.StatusBadRequest, rec.Code)
    body := decodeErrorBody(t, rec)
    require.Equal(t, "invalid_org_id", body["error"])
    require.Equal(t, "missing_header", body["reason"])
}
```

**Deviation note:** **Reuse pattern X verbatim.** Embed CORS test asserts: (1) OPTIONS preflight without `X-Org-Id` returns 200 (short-circuit happens BEFORE next); (2) `Access-Control-Allow-Origin` header matches `Origin` request header when allowed; (3) rejected origin gets no `Access-Control-Allow-Origin`; (4) `AllowedOriginsFromEnv` parses comma-separated origins, trims whitespace, treats `""` as empty list. Each test follows the table-driven `httptest.NewRequest` + `httptest.NewRecorder` shape. Both `failNextHandler` and `decodeErrorBody` helpers reusable verbatim. **Differs in Y way:** no `WriteError` reason-string assertions (CORS lib doesn't emit them).

---

### `services/api/internal/server/server.go` (MODIFY — insert CORS into chain)

**Analog:** `services/api/internal/server/server.go` (self — existing NewMux at lines 77-131)

**Existing chain** (server.go lines 77-90):
```go
func NewMux(deps *Deps) http.Handler {
    r := chi.NewRouter()

    // (1) Recoverer first.
    r.Use(chimw.Recoverer)
    // (2) Our UUIDv7 RequestID — overrides chi's host/N-K counter (D-28).
    //     CRITICAL: must run BEFORE the strict pipeline so the
    //     RequestIDInjectionMiddleware StrictMiddlewareFunc can pull the
    //     id from ctx on the way out.
    r.Use(appmw.RequestID)

    // (3) /metrics: Phase 1 stub — NOT in the generated spec; registered
    //     separately as a bare chi route at root (D-21 bypass list).
    r.Get("/metrics", MetricsHandler())
    /* ... */
}
```

**Modification pattern** (from RESEARCH §Code Examples, lines 939-944):
```go
// Insert AFTER Recoverer + RequestID, BEFORE strict pipeline.
r.Use(chimw.Recoverer)
r.Use(appmw.RequestID)
r.Use(appmw.NewCORS(deps.Config.CORSAllowedOrigins))  // NEW for Phase 7
r.Get("/metrics", MetricsHandler())
```

**Deviation note:** **Self-modification.** Add ONE `r.Use()` line after `appmw.RequestID`, BEFORE the `r.Get("/metrics", ...)` and `api.HandlerWithOptions(...)` strict pipeline. Critical insight from RESEARCH (line 100-103, 946): `cors.Handler` short-circuits OPTIONS automatically — inserting at the chi-root level handles bypass routes AND /v1 routes uniformly without touching the existing `orgContextMiddleware` conditional. **Reuse pattern X (chain insertion); differs in Y way:** chain order is the actual change (CORS BEFORE both OrgContext-via-strict-pipeline AND BodyLimit-via-ChiServerOptions).

---

### `services/api/internal/config/config.go` (MODIFY — add CORSAllowedOrigins)

**Analog:** `services/api/internal/config/config.go` (self — existing Config struct)

**Existing struct** (used in main_test.go isolation suite, line 187-193):
```go
cfg := &config.Config{
    DatabaseURL:    "n/a",
    RedisURL:       "n/a",
    OTelExporter:   "stdout",
    ListenAddr:     ":0",
    ValidationMode: "panic",
}
```

**Deviation note:** Add field `CORSAllowedOrigins []string` to `config.Config` struct. Add env-var parser entry `CORS_ALLOWED_ORIGINS` in the config Load function (split comma-separated, default empty list, dev Taskfile sets `"*"`). **Reuse pattern X (env-var → typed field); differs in Y way:** list-typed field (most existing fields are scalar strings).

---

### `services/api/test/isolation/cors_preflight_test.go` (NEW)

**Analog:** `services/api/test/isolation/isolation_test.go` (lines 26-95 — helper functions + test shape)

**Helpers** (isolation_test.go lines 26-52):
```go
func baseURL() string {
    return sharedSrv.URL
}

func freshOrg(t *testing.T) uuid.UUID {
    t.Helper()
    return uuid.Must(uuid.NewV7())
}

func requireContainer(t *testing.T) {
    t.Helper()
    if testing.Short() {
        t.Skip("isolation: requires postgres testcontainer; -short set")
    }
    if sharedPool == nil || sharedSrv == nil {
        t.Skip("isolation: shared testcontainer/server unavailable")
    }
}
```

**Test pattern** (isolation_test.go lines 60-67):
```go
func TestOrgContext_MissingHeader400(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    resp, body := testsupport.DoBare(t, baseURL(), http.MethodGet,
        "/v1/orgs/"+uuid.Must(uuid.NewV7()).String()+"/agents", nil)
    require.Equal(t, http.StatusBadRequest, resp.StatusCode)
    require.Contains(t, string(body), `"reason":"missing_header"`)
}
```

**Deviation note:** **Reuse pattern X (requireContainer guard + `testsupport.DoBare` HTTP helper + `t.Parallel()` + table-driven assertions).** CORS preflight test asserts: (1) OPTIONS to `/v1/orgs/{id}/agents` with `Origin: https://example.com` returns 200 (NOT 400 missing_header) — CORS short-circuits before OrgContext runs; (2) OPTIONS to bypass route `/healthz` returns 200 with CORS headers; (3) `Access-Control-Allow-Methods` header lists GET, POST, PATCH, DELETE, OPTIONS; (4) `Access-Control-Allow-Headers` lists X-Org-Id, Content-Type, Idempotency-Key, Accept-Language. **Differs in Y way:** uses `http.MethodOptions` instead of `http.MethodGet`; assertions check response headers instead of body fields.

---

### `services/api/test/isolation/cors_cross_origin_test.go` (NEW)

**Analog:** `services/api/test/isolation/isolation_test.go` (lines 60-95 — same helpers)

**Deviation note:** Cross-origin GET test mirrors the preflight test shape but asserts the actual request response: (1) GET `/v1/orgs/{id}/agents` with `Origin: https://allowed.example.com` + valid `X-Org-Id` returns 200 + `Access-Control-Allow-Origin: https://allowed.example.com`; (2) same request with `Origin: https://rejected.example.com` returns 200 (body served) but NO `Access-Control-Allow-Origin` header (browser will block the response, not the server); (3) `X-Request-Id` header is exposed via `Access-Control-Expose-Headers` so admin/embed can read it on 4xx responses. **Reuse pattern X verbatim.**

---

### `.github/workflows/ci.yml` (NEW file — CI does not exist yet)

**Analog:** NO ANALOG — repository has no `.github/workflows/*.yml`. Reference RESEARCH §VA + D7-09 + D7-12.

**Pattern to write** (research-derived, GitHub Actions canonical shape):
```yaml
name: CI

on:
  pull_request:
  push:
    branches: [main]

jobs:
  web-typecheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with: { node-version: 22, cache: pnpm }
      - run: pnpm install --frozen-lockfile
      - run: pnpm -F '*' typecheck

  web-bundle-size:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with: { node-version: 22, cache: pnpm }
      - run: pnpm install --frozen-lockfile
      - run: pnpm --filter @open-routing/embed build
      - uses: andresz1/size-limit-action@v1
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          directory: web/apps/embed

  web-e2e-embed:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with: { node-version: 22, cache: pnpm }
      - run: pnpm install --frozen-lockfile
      - uses: actions/cache@v4
        with:
          path: ~/.cache/ms-playwright
          key: playwright-${{ hashFiles('web/pnpm-lock.yaml') }}
      - run: pnpm --filter @open-routing/embed exec playwright install --with-deps chromium
      - run: pnpm --filter @open-routing/embed build
      - run: pnpm --filter @open-routing/embed test:e2e
```

**Deviation note:** **New territory — no analog (repo has no CI yaml).** Per CONTEXT.md `<canonical_refs>` D-30 LOCKED, both `web-bundle-size` and `web-e2e-embed` become required-status-checks. Per RESEARCH §Code Examples (Playwright section), cache `~/.cache/ms-playwright` keyed on `pnpm-lock.yaml` hash saves ~90s/run. Per D7-09, use `andresz1/size-limit-action@v1` (NOT the GitHub Action default — andresz1 has the PR-comment-with-delta feature).

---

### `web/apps/embed/README.md`, `LICENSE`, `CHANGELOG.md`

**Analog:** NO ANALOG (no admin README, no LICENSE file in repo so far).

**Deviation note:** **New territory — research-derived.** Per D7-15 + RESEARCH §Implementation Strategy 8: README documents attribute API (org-id, api-base-url, theme, modules, locale), event API (open-routing:request-context, open-routing:auth-expired), theme JSON shape (5-10 most-impactful keys from RESEARCH §5, lines 568-578), CORS setup, single-embed-per-page limitation (D7-07), esm.sh CDN usage example. CHANGELOG follows standard "0.1.0-pre.1 - 2026-05-18" format. LICENSE is whatever project-level license is (need to confirm with user; recommend MIT). No code excerpts — pure documentation.

---

## Shared Patterns

### Lit `accessor` Decorator for TC39 2023-11 Standard Decorators

**Source:** `web/packages/ui/src/components/shell/catalog-shell.ts` (lines 204-207) + RESEARCH §2 note (line 452)

**Apply to:** Every `@property` and `@state` declaration in embed-element.ts, hash-router-adapter.ts (none), and the shell refactor.

```typescript
@property({ type: String, attribute: 'org-id' }) accessor orgId = '';
@property({ type: String }) accessor theme: ThemeName = 'or-light';
@state() private accessor _currentOrgId = '';
@state() private accessor _client: ApiClient | null = null;
```

**Critical:** Phase 6 admin proves this works in production under Vite 8 + Rolldown + `@rolldown/plugin-babel`. Drop `accessor` and reactivity silently breaks (no compile error). The `useDefineForClassFields: true` in tsconfig + the babel decorator preset together make this work.

---

### Composed CustomEvent Dispatch (Cross Shadow DOM)

**Source:** `web/packages/ui/src/components/primitives/conflict-banner.ts` (lines 206-213) + RESEARCH §Pitfall 4

**Apply to:** `embed-element.ts` for `open-routing:request-context` (D7-13) and `open-routing:auth-expired` (D7-14). Internal events (shell ↔ entity components inside one shadow root) do NOT need `composed: true`.

```typescript
this.dispatchEvent(new CustomEvent('open-routing:auth-expired', {
  detail: { statusCode: 401, requestId, path },
  bubbles: true,
  composed: true,  // REQUIRED — without this, event stops at shadowRoot
}));
```

**Pitfall:** `CustomEvent` defaults are `bubbles: false, composed: false`. Forgetting `composed: true` is the most common bug per RESEARCH §Pitfall 4 — host listener attached to `document` never fires.

---

### UUIDv7 Validation Regex (Client-Side)

**Source:** `web/packages/ui/src/components/shell/catalog-shell.ts` (line 72) — D6-13 carry-forward

**Apply to:** `embed-element.ts` (D7-08 — malformed `org-id` → inline error state).

```typescript
const UUIDV7_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
```

**Server is authoritative** (Phase 1 D-20 / orgcontext.go); client-side regex is UX nicety only.

---

### Chi MiddlewareFunc Factory Shape (Go)

**Source:** `services/api/internal/middleware/orgcontext.go` (line 48) + `services/api/internal/middleware/bodylimit.go` (line 44) + `requestid.go` (line 32)

**Apply to:** `cors.go` `NewCORS()` factory.

```go
func XxxMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ... pre-next logic ...
        next.ServeHTTP(w, r)
        // ... post-next logic (optional) ...
    })
}
```

**Factory-returning-middleware variant** (when middleware needs config — bodylimit pattern):

```go
func XxxMiddleware(config XxxConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // use config + next
        })
    }
}
```

CORS uses the factory variant (config = `cors.Options{...}` constructed from env-var list).

---

### Go Middleware Test — httptest.NewRequest + Recorder

**Source:** `services/api/internal/middleware/orgcontext_test.go` (full file)

**Apply to:** `cors_test.go` (unit tests) + `cors_preflight_test.go` / `cors_cross_origin_test.go` (integration tests via shared isolation_test server).

```go
func TestXxx_Yyy(t *testing.T) {
    t.Parallel()
    h := XxxMiddleware(failNextHandler(t))   // or noopOK handler for accept tests
    req := httptest.NewRequest(http.MethodGet, "/v1/orgs/...", nil)
    req.Header.Set("Origin", "https://example.com")
    rec := httptest.NewRecorder()
    h.ServeHTTP(rec, req)
    require.Equal(t, http.StatusOK, rec.Code)
    require.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}
```

**Helpers reusable verbatim:** `failNextHandler(t)`, `decodeErrorBody(t, rec)`.

---

### Vitest Component Test Lifecycle

**Source:** `web/packages/ui/src/components/shell/catalog-shell.test.ts` (lines 1-18) + `agents/agent-list.test.ts` (lines 19-30)

**Apply to:** `embed-element.test.ts`, `hash-router-adapter.test.ts`, every shell refactor test.

```typescript
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import './target-component.js';  // registers <or-target> custom element

describe('OrTarget', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-target');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('does X', async () => {
    (el as any).propName = 'value';
    await (el as any).updateComplete;  // REQUIRED — await Lit render cycle
    // assertions
  });
});
```

**Rule:** Always `await el.updateComplete` after every property set or event dispatch. Import the component module (registration side effect), not the class.

---

### Theme-on-Host CSS Property Cascade (D6-20)

**Source:** `web/packages/ui/src/components/shell/catalog-shell.ts` (lines 467-481)

**Apply to:** `embed-element.ts` for forwarding `theme=` JSON to shell (NOT directly applying — shell does the actual `this.style.setProperty()`).

```typescript
override updated(changed: Map<string, unknown>): void {
  if (changed.has('theme')) this._applyTheme();
}

private _applyTheme(): void {
  for (const key of ALL_TOKEN_KEYS) {
    this.style.removeProperty(key);
  }
  const tokens = _THEME_TOKENS[this.theme] ?? {};
  for (const [key, val] of Object.entries(tokens)) {
    this.style.setProperty(key, val);
  }
}
```

**Embed flow:** parse `theme=` attribute (string OR JSON), forward to shell's `theme` reactive property; shell's `_applyTheme` does the actual injection. **Don't duplicate** the token-key iteration in embed-element.

---

### openapi-fetch `createApiClient` Per-Element Lifecycle

**Source:** `web/packages/ui/src/api/client.ts` (full file, lines 45-65) — D-38

**Apply to:** `embed-element.ts` — each `<open-routing-catalog>` instance builds its own client with `getOrgId: () => this.orgId`.

```typescript
this._client = createApiClient({
  baseURL: this.apiBaseUrl,
  getOrgId: () => this.orgId,
});
```

**Critical:** `getOrgId` is invoked PER REQUEST (not at construction) — embed instances can return different values across element life as `org-id` attribute changes. RESEARCH §2 + Pitfall 3 note: when wiring auth-expired middleware AFTER client creation via `.use(authExpiredMiddleware)`, order doesn't matter because X-Org-Id uses `onRequest` and auth-expired uses `onResponse` — independent hooks.

---

## No Analog Found

Files with no close match in the codebase (planner should use RESEARCH.md patterns instead):

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `web/apps/embed/vite.config.ts` (library mode portion) | build-config | — | First library-mode build in repo; admin is `appType: 'spa'` |
| `web/apps/embed/.size-limit.json` | build-config | — | First size-limit usage in repo (D7-09) |
| `web/apps/embed/src/hash-router-adapter.ts` | router-adapter | event-driven | First hash-routing code in repo (admin uses history mode only) |
| `web/apps/embed/e2e/hosts/{react,vue,html}/index.html` | playwright-spec | — | First esm.sh CDN stub-host pattern in repo |
| `web/apps/embed/e2e/specs/embed-org-id-header.spec.ts` (page.route mocking) | playwright-spec | request-response | First `page.route` API-intercept usage in repo |
| `.github/workflows/ci.yml` | ci-yaml | — | No GitHub Actions yaml exists in repo |
| `web/apps/embed/README.md`, `LICENSE`, `CHANGELOG.md` | readme | — | No admin README; no LICENSE in repo |

---

## Phase Boundary Notes

**Files NOT modified by Phase 7 (per CONTEXT.md `<out_of_scope>`):**
- Phase 6 entity components (`agents/`, `skills/`, `queues/`, `channels/`, `adapters/`, `break-reasons/`) — converted from static-imported to dynamic-imported by shell refactor, but the **component files themselves** are NOT edited
- API client (`packages/ui/src/api/client.ts`) — embed reuses Phase 2 D-38 factory verbatim
- Phase 1 OrgContext middleware — embed's CORS sits BEFORE it in chain; OrgContext code unchanged
- Phase 1-5 isolation test suite — Phase 7 ADDS two new test files (`cors_preflight_test.go`, `cors_cross_origin_test.go`) but does NOT modify the existing 7 test files

**Critical refactor target — `<sl-dialog>` audit per RESEARCH §Runtime State Inventory:**
- 8 files use `<sl-dialog>`: adapter-detail, agent-detail, break-reason-detail, channel-detail, queue-detail, skill-detail (×2), status-panel
- Each replaced with inline confirm panel per `conflict-banner.ts` template (aria-live host + inline div + action-row)
- These files ARE modified by Phase 7 even though they're "Phase 6 components" — D7-04 mandate

---

## Metadata

**Analog search scope:**
- `web/apps/admin/` (Phase 6 admin SPA)
- `web/apps/embed/` (existing skeleton)
- `web/packages/ui/src/` (Phase 6 shell + components + primitives + api + themes + locales)
- `services/api/internal/middleware/` (Phase 1 chi middleware)
- `services/api/test/isolation/` (Phase 1+ integration tests)
- `services/api/internal/server/server.go` (chi NewMux chain)
- `web/tsconfig.base.json`, `web/turbo.json`, `web/packages/ui/vitest.config.ts`

**Files scanned (analogs read):** 18
- web/apps/admin: package.json, vite.config.ts, playwright.config.ts, src/index.ts, e2e/smoke.spec.ts, index.html
- web/apps/embed: package.json (skeleton), tsconfig.json, src/index.ts (skeleton)
- web/packages/ui: package.json, vitest.config.ts, src/test-setup.ts, src/api/client.ts, src/components/shell/catalog-shell.ts (lines 1-481), src/components/shell/catalog-shell.test.ts (lines 1-60), src/components/primitives/conflict-banner.ts (full), src/components/primitives/conflict-banner.test.ts (lines 1-50), src/components/agents/agent-list.ts (lines 1-60), src/components/agents/agent-list.test.ts (lines 1-80)
- services/api: internal/middleware/orgcontext.go (full), orgcontext_test.go (full), bodylimit.go (lines 1-60), bodylimit_test.go (lines 1-70), requestid.go (lines 1-50), uuidv7path.go (lines 1-60), internal/server/server.go (lines 1-152), test/isolation/isolation_test.go (lines 1-95), test/isolation/main_test.go (lines 1-276)

**Pattern extraction date:** 2026-05-18

**Key takeaway for planner:** Phase 7 is a **composition phase** — every primitive exists. Roughly 70% of files reuse a Phase 6 / Phase 1 analog verbatim or with minor changes; 30% are research-derived new territory (Vite library mode, hash adapter, size-limit, esm.sh stub hosts, page.route mocking, CI yaml). The most pattern-dense file is `embed-element.ts`, which **stacks three Phase 6 patterns** (shell LitElement reactivity + conflict-banner composed CustomEvent + UUIDv7 validation) and adds one new pattern (per-element client lifecycle with `getOrgId` closure over `this.orgId`).
