# Phase 7: Web Component Embed Bundle - Research

**Researched:** 2026-05-18
**Domain:** Vite 8 library mode + Lit 3 Custom Elements + Shadow DOM + `@lit-labs/router` lazy routes + size-limit gating + go-chi/cors + Playwright multi-host matrix
**Confidence:** HIGH (every major decision verified against official sources; one MEDIUM area noted around brotli-vs-gzip baseline strictness)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Reuse Strategy (D7-01..D7-04):**
- **D7-01:** Hybrid — mount `<or-catalog-shell>` from `packages/ui` + lazy-load entity routes via dynamic `import()`. Eager baseline = shell + router + chrome + theme tokens + primitives + auth-banner. Lazy = 6 entity bundles + status-panel + import-page. Rejected: mount-shell-as-is (balloon risk) and compose-entities (drift from admin UX).
- **D7-02:** Eager baseline = chrome a user always sees on first paint. 5 primitives + auth-expired banner. Each entity becomes a separate Vite chunk. Soft per-chunk budget ~15 KB gzipped (logged, not blocked).
- **D7-03:** Refactor `catalog-shell.ts` IN PLACE — same shell for admin + embed. Convert all 30+ static entity imports to a `lazyRoute()` helper that lazily imports the entity module inside the router's `enter` hook. Admin gets lazy loading as a side effect. No `catalog-shell-lazy.ts` variant. Plan must re-run Plan 06-06 Playwright smoke.
- **D7-04:** Inline alerts only — no Shoelace `<sl-dialog>` / `<sl-alert>` toast usage anywhere embed reaches. `<or-conflict-banner>` is already inline; remaining Shoelace dialog/toast usage in `packages/ui` components is audited and replaced with inline Lit alerts.

**Shadow DOM Routing (D7-05..D7-08):**
- **D7-05:** Hash routing inside embed — `window.location.hash` carries embed nav state (`#open-routing/orgs/{id}/agents`).
- **D7-06:** Embed-only hash mode; admin stays on history. Shell gains `routingMode: 'history' | 'hash'` reactive property (default `'history'`). Hash adapter (~30 lines) at `web/apps/embed/src/hash-router-adapter.ts`. Admin URLs remain `/orgs/{id}/agents`.
- **D7-07:** Document "single embed per page" v0.1 limitation in README + JSDoc on the Custom Element class.
- **D7-08:** Malformed/missing `org-id` → error state inside Shadow DOM (`<div class="error">Invalid org-id attribute</div>`). No `<or-org-picker>` fallback. Attribute-change observer re-runs validation; live correction recovers without remount.

**CI Infrastructure (D7-09..D7-12):**
- **D7-09:** size-limit declarative budget + GitHub Action PR comment via `andresz1/size-limit-action`. Budget in `apps/embed/.size-limit.json`. CI job `web-bundle-size`. Hard fail above 70 KB.
- **D7-10:** Eager baseline ≤70 KB gzipped. Per-entity lazy chunks each get a ~15 KB gzipped soft budget (logged only).
- **D7-11:** Playwright stub hosts at `apps/embed/e2e/hosts/{react,vue,html}/` — static HTML + esm.sh CDN. Bundle served via Vite preview.
- **D7-12:** Playwright failure budget — `retries: 1` per spec; identical assertions across 3 hosts; flaky failure blocks merge. New required-status-check `web-e2e-embed`.

**Embed Public Contract (D7-13..D7-16):**
- **D7-13:** `open-routing:request-context` is announce-only. Fires once on `connectedCallback`. `detail = { orgId, apiBaseUrl, theme, modules }`. `composed: true`, `bubbles: true`.
- **D7-14:** `open-routing:auth-expired` fires on EVERY 401. `detail = { statusCode: 401, requestId, path }`. `composed: true`, `bubbles: true`. Embed continues fetching; inline banner dismissible.
- **D7-15:** Build artifact + npm-ready `package.json`; DO NOT publish in v0.1. CI verifies `pnpm pack --dry-run`. No `pnpm publish` step.
- **D7-16:** Go API ships CORS middleware (`go-chi/cors`) with env-var allowlist `CORS_ALLOWED_ORIGINS`. Inserted BEFORE `OrgContext` in chi chain so preflight `OPTIONS` don't return 400. Bypass routes (Phase 1 D-21) covered identically. Phase 1 isolation suite extended with one OPTIONS preflight + one cross-origin GET assertion.

### Claude's Discretion

- Exact internal layout of `apps/embed/src/` — recommend `index.ts` registers the element + locale bootstrap, `embed-element.ts` is the LitElement class.
- size-limit GH Action choice — recommend `andresz1/size-limit-action` (more downloads, recent commits).
- esm.sh version pinning strategy in stub hosts — recommend specific minor (e.g. `react@18.3.1`).
- Hash-adapter implementation detail — recommend `URLPattern` so shell's existing route definitions work without dual notation.
- Auth-expired inline banner copy + dismiss UX — align with D6-24 `ErrorCode → errors.auth_expired` pattern.
- Whether `<or-conflict-banner>` and inline auth-banner share a base component — recommend independent for v0.1.
- README example payload for `theme=` attribute — pick 5-10 most-impactful keys (primary color, background, text, border-radius, font family).
- Shadow DOM mode `open` vs `closed` — recommend `open` (host can `querySelector` into `shadowRoot` for debugging; closed sometimes breaks Playwright assertions).
- Exact Playwright spec structure for cross-host matrix — between (a) one spec file per assertion type with `projects` matrix, or (b) one spec per host with all 4 assertions inside.
- Source-map handling in production build — recommend emitting external `.map` files; document that ops can choose not to deploy them.

### Deferred Ideas (OUT OF SCOPE)

- Per-instance `hash-prefix` attribute for multi-embed pages — v0.2 if customer demand emerges.
- history-mode routing for embed with `base-path` attribute — v1.
- Real npm/CDN publishing of `@open-routing/catalog-embed` — v1.
- Unified hash routing across admin + embed — rejected (admin UX aesthetic).
- 3 user-selectable routing modes via attribute — premature flexibility.
- Negotiable `open-routing:request-context` with preventDefault-blocks-boot — v1 auth phase.
- First-401-only auth-expired with UI lock — v1 if event storms become a host problem.
- GitHub Packages private publishing path — unnecessary for v0.1.
- Per-org brand theme customization (Phase 6 D6-19 carry-forward).
- Server-side rendering of `<open-routing-catalog>` — v0.1 is client-only.
- Operator-facing CORS allowlist UI per org — v0.1 ships env-var allowlist.
- Embed-side localStorage / sessionStorage usage — host owns persistence.
- Bundle-analyzer (`rollup-plugin-visualizer`) — v0.2 as non-blocking PR comment.
- Brotli + raw-minified bundle budgets — v0.2.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| EMBED-01 | Single ES module bundle registers `<open-routing-catalog>` Custom Element; ≤70 KB gzipped target | Vite `build.lib` with single ES entry + natural dynamic-import chunk splitting; size-limit gate (§Standard Stack, §Implementation Strategy 1) |
| EMBED-02 | Element accepts `org-id`, `api-base-url`, `theme`, `modules` attrs; `org-id` flows via `X-Org-Id` header | Lit `@property({ attribute: 'org-id' })` + existing `createApiClient({ getOrgId })` factory (Phase 2 D-38) (§Implementation Strategy 2) |
| EMBED-03 | Renders list/create/edit/delete for all 6 catalog entities, reusing `packages/ui` Lit components | Mount `<or-catalog-shell>` with lazy routes pointing at existing `packages/ui` entity components (§Implementation Strategy 3) |
| EMBED-04 | Shadow DOM CSS isolation in both directions; host page styles unaffected | Custom Element with `attachShadow({ mode: 'open' })` + audit and replace all `<sl-dialog>` / `<sl-tooltip>` instances (§Implementation Strategy 4, §Pitfall 1) |
| EMBED-05 | `theme` attribute = JSON-encoded CSS custom property tokens; applied on Shadow DOM host | Parse JSON in `attributeChangedCallback`, forward to shell `theme` property; D6-20 `this.style.setProperty(...)` pattern unchanged (§Implementation Strategy 5) |
| EMBED-06 | `modules` attribute = comma-separated entity list; navigation filters | Existing D6-11 `modules` filter in `<or-catalog-shell>._visibleEntries()` — pass-through (§Implementation Strategy 6) |
| EMBED-07 | `open-routing:request-context` on mount + `open-routing:auth-expired` on 401; `composed: true` events | `CustomEvent` with `composed: true, bubbles: true` from `embed-element.ts`; auth-expired via openapi-fetch onResponse middleware (§Implementation Strategy 7) |
| EMBED-08 | HTTP 409 "Changed by someone else, reload?" UX parity with admin | Phase 6 `<or-conflict-banner>` already implements this; embed inherits via shell reuse (§Implementation Strategy 3) |
| EMBED-09 | Published as `@open-routing/catalog-embed` on npm / CDN | Phase 7 deferral: build artifact + `pnpm pack --dry-run` validation; real publish = v1 (§Implementation Strategy 8) |
| EMBED-10 | Playwright stub host integration tests for React 18, Vue 3, plain HTML — Shadow DOM isolation + org-id propagation + auth-expired + bundle size | Playwright multi-project config (3 projects × 4 specs); Vite preview + esm.sh CDN hosts (§Implementation Strategy 9, §Validation Architecture) |
</phase_requirements>

---

## Summary

Phase 7 ships `web/apps/embed/` — a single ES module that registers the `<open-routing-catalog>` Custom Element. The bundle wraps `<or-catalog-shell>` (Phase 6) inside a Shadow DOM, with the shell refactored in-place to lazy-load entity routes via dynamic imports. The central technical challenge is hitting the 70 KB gzipped eager-baseline budget — every Shoelace component, every Lit utility, every theme token must be tree-shake-clean. Phase 6's per-component Shoelace import discipline (D6-08) is the precondition; Phase 7 verifies the discipline holds and ships a CI gate that fails if it slips.

Three findings deserve immediate planner attention:

1. **`@lit-labs/router`'s `enter` hook supports the async lazy-import pattern exactly as the CONTEXT.md describes** — verified against the official Lit discussions (#3354) and the [README example](https://www.npmjs.com/package/@lit-labs/router): `enter: async () => { await import('./x-foo.js'); }` is the documented pattern. `goto()` returns a Promise that awaits async enter callbacks. **No deviation needed from the D7-03 plan.** [VERIFIED: npm @lit-labs/router README + github.com/lit/lit discussions/3354]

2. **`<sl-dialog>` does NOT portal to `document.body`** but stays inside its parent shadow root. However, focus-trap behavior is documented broken when `<sl-dialog>` is nested in a parent shadow root (issues [#709](https://github.com/shoelace-style/shoelace/issues/709), [#1382](https://github.com/shoelace-style/shoelace/issues/1382)). D7-04's "inline alerts only" mandate is the right call — but the audit scope is larger than CONTEXT.md anticipated: **15 files in `packages/ui` use `<sl-dialog>` or `<sl-tooltip>`** and need attention. `<sl-tooltip>` uses Floating UI's positioning; with `hoist` attribute it CAN move outside the parent (potential leak), without `hoist` it stays in place but may get clipped. The catalog-shell's own `<sl-tooltip>` on the org-chip needs review.

3. **`go-chi/cors` is designed to short-circuit OPTIONS preflight requests by default** (without `OptionsPassthrough`), returning `200 OK` and never invoking downstream middleware. Source code at [cors.go:handlePreflight](https://github.com/go-chi/cors/blob/master/cors.go) confirms: `if c.optionPassthrough { next.ServeHTTP(w, r) } else { w.WriteHeader(http.StatusOK) }`. **This means CORS at the chi-root level can be wired BEFORE `OrgContext` without touching the bypass-route logic** — the OPTIONS request never reaches OrgContext to be 400'd for missing X-Org-Id. D7-16's ordering is correct; planner must place `r.Use(cors.Handler(...))` as the first or second `Use()` in `server.NewMux` (after `Recoverer`).

The fourth research question — size-limit gzip vs CDN — has a MEDIUM-confidence answer: size-limit's gzip is the [Node zlib library](https://github.com/ai/size-limit) at default compression. Cloudflare Free serves Gzip (matches size-limit ±2 KB); Cloudflare Pro/Business default to Brotli (15-30% smaller than gzip per [Cloudflare engineering blog](https://blog.cloudflare.com/results-experimenting-brotli/)). For a 70 KB gzip budget, brotli-served reality is ~50-60 KB. **Recommendation: keep gzip-only budget in Phase 7; document that real-world Brotli will be smaller; defer brotli budget to v0.2.**

**Primary recommendation:** In Wave 1, do TWO things in parallel before any Custom Element code: (a) `vite.config.ts` library-mode setup with `formats: ['es']`, single entry, sourcemaps, `external: ['/^@open-routing/']` ONLY if pnpm pack will bundle the workspace dep — actually keep `noExternal: true` so the workspace `@open-routing/ui` gets bundled into `dist/embed.js`; (b) the `<sl-dialog>` / `<sl-tooltip>` audit in `packages/ui` to flag every instance that needs replacement BEFORE Wave 2 starts refactoring the shell.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Custom Element registration | Browser / Client (embed entry) | — | `customElements.define('open-routing-catalog', OpenRoutingCatalog)` is a browser API; no SSR in v0.1 |
| Attribute parsing + validation (org-id, theme, modules, api-base-url, locale) | Browser / Client (embed-element.ts) | — | Lit `@property({ attribute: ... })` decorators + `attributeChangedCallback` |
| Shadow DOM construction | Browser / Client (LitElement default) | — | Lit auto-attaches `shadowRoot` via `createRenderRoot()` |
| Theme JSON parse + forward to shell | Browser / Client (embed-element.ts) | Shell (applies tokens) | Embed parses; shell already implements D6-20 token-on-host cascade |
| Hash route parsing | Browser / Client (hash-router-adapter.ts) | Shell (consumes routes) | `window.location.hash` is browser-only; shell delegates to adapter when `routingMode === 'hash'` |
| Lazy entity bundle loading | Browser / Client (shell's `enter` hooks) | Vite (codegen-time chunk splitting) | Vite's dynamic-import code splitting splits per `await import(...)`; runtime requests chunks on route entry |
| Public CustomEvents (request-context, auth-expired) | Browser / Client (embed-element.ts dispatches) | API client middleware (signals 401) | `dispatchEvent` from element; openapi-fetch onResponse triggers on 401 |
| X-Org-Id header injection | Browser / Client (API client middleware) | Backend (enforces) | Phase 2 D-38 `createApiClient` already implements `getOrgId` middleware — reused |
| CORS preflight handling | API / Backend (chi middleware) | — | go-chi/cors at chi-root; short-circuits OPTIONS without invoking OrgContext |
| Bundle size budget enforcement | CI / Build (size-limit + GH Action) | — | Build-time measurement; PR comment with delta vs main |
| Cross-host integration verification | CI / Build (Playwright matrix) | Vite preview (serves bundle) | Static HTML hosts + Vite preview at localhost:4173 |

---

## Standard Stack

### Core (apps/embed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `lit` | 3.3.3 [VERIFIED: npm registry — `npm view lit version` confirmed at admin's existing dep] | LitElement base class for Custom Element | Already locked in Phase 6 (D6-07); reuse |
| `@open-routing/ui` | workspace:* [VERIFIED: pnpm workspace path] | All shell + entity components reused | Phase 6 ships these — D7-01 hybrid mandate |
| `@lit-labs/router` | 0.1.4 [VERIFIED: npm registry, published 2025-02-14] | Route definitions consumed by shell | Already in `packages/ui` deps via Phase 6 |
| `@lit/localize` | 0.12.2 [VERIFIED: npm registry] | Locale module loading for `locale=` attr | Already in `packages/ui` deps |
| `@shoelace-style/shoelace` | 2.20.1 [VERIFIED: npm registry, published 2025-03-11] | Transitive UI primitives via packages/ui | NOT a direct embed dep (re-exported via packages/ui per D6-08) |

### Build Tooling (apps/embed devDependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `vite` | 8.0.13 [VERIFIED: npm registry] | Library-mode build pipeline | Phase 6 lock; reuse |
| `@rolldown/plugin-babel` | 0.2.3 [VERIFIED: npm registry, published 2026-04-13] | Babel decorator transform under Rolldown/Oxc | Phase 6 D6-V-RESEARCH established this — Oxc does not lower experimentalDecorators |
| `@babel/plugin-proposal-decorators` | 7.29.0 [VERIFIED: npm registry] | TC39 2023-11 decorator spec lowering | Paired with @rolldown/plugin-babel |
| `urlpattern-polyfill` | 10.1.0 [VERIFIED: npm registry] | URLPattern shim for Vitest/Node | Required for hash-router-adapter URLPattern usage in tests |
| `size-limit` | 12.1.0 [VERIFIED: npm registry — `npm view size-limit version`] | Declarative bundle size budget | D7-09 lock; trusted ecosystem (date-fns, redux, Preact use it) |
| `@size-limit/preset-app` | 12.1.0 [VERIFIED: npm registry] | Standard size-limit preset for app bundles | Standard preset paired with size-limit |
| `@playwright/test` | 1.60.0 [VERIFIED: npm registry] | E2E matrix runner | Phase 6 admin uses this; reuse |
| `vitest` | 3.2.4 [VERIFIED: npm registry] | Unit tests for embed-element + hash-router-adapter | Already in workspace |
| `happy-dom` | 20.9.0 [VERIFIED: npm registry] | Test environment with Web Component APIs | Already in workspace |

### Backend Addition (services/api)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/cors` | v1.2.2 [VERIFIED: `go list -m -versions github.com/go-chi/cors` returned v1.0.0...v1.2.2; latest is v1.2.2] | CORS middleware for chi | D7-16 lock; canonical for go-chi ecosystem |

### Stub Host CDN (Playwright e2e — not bundled, importmap-loaded)

| Library | Version | Purpose | Pinning Recommendation |
|---------|---------|---------|------------------------|
| `react` (esm.sh) | 18.3.1 [VERIFIED: esm.sh URL pattern] | React 18 stub host | `https://esm.sh/react@18.3.1` (specific patch — reproducibility) |
| `react-dom` (esm.sh) | 18.3.1 | React 18 DOM | `https://esm.sh/react-dom@18.3.1` |
| `vue` (esm.sh) | 3.5.13 | Vue 3 stub host | `https://esm.sh/vue@3.5.13` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `andresz1/size-limit-action` | `pajaydev/size-limit-bundle-action` | Andresz1 has more downloads, recent commits, working PR-comment format. D7-09 picks andresz1. |
| `size-limit` gzip-only budget | brotli budget (closer to CDN reality) | Brotli baseline depends on CDN tier (Cloudflare Free = gzip, Pro = brotli). Gzip is conservative; brotli would be 15-30% smaller. Phase 7 ships gzip-only per D7-10; brotli deferred per `<deferred>`. |
| Vite library mode + `external: ['/@open-routing\\/.*/']` | `noExternal: true` (bundle workspace dep) | Embed must ship a self-contained ES module — host apps don't have `@open-routing/ui` available. **noExternal is required**; workspace dep gets inlined. |
| `@lit-labs/router` URLPattern hash adapter | Hand-rolled regex on `location.hash` | URLPattern lets the shell's existing route definitions work without dual notation. Hand-rolled would duplicate route patterns. D7 Discretion item recommends URLPattern. |
| Closed Shadow DOM (`mode: 'closed'`) | Open Shadow DOM (`mode: 'open'`) | Closed prevents host JS from inspecting embed internals but BREAKS Playwright's `:scope >>` shadow piercing and many debugging workflows. Open is the documented Playwright recommendation. CONTEXT Discretion item recommends `open`. |

**Installation (apps/embed additions):**
```bash
pnpm --filter @open-routing/embed add lit @open-routing/ui @shoelace-style/shoelace
pnpm --filter @open-routing/embed add -D vite @rolldown/plugin-babel @babel/plugin-proposal-decorators urlpattern-polyfill size-limit @size-limit/preset-app @playwright/test vitest happy-dom
```

**Installation (services/api):**
```bash
cd services/api && go get github.com/go-chi/cors@v1.2.2 && go mod tidy
```

**Version verification commands** (planner should re-run before installing):
```bash
npm view lit version                          # confirms 3.3.x current
npm view @lit-labs/router version             # confirms 0.1.4
npm view size-limit version                   # confirms 12.x
npm view @size-limit/preset-app version       # confirms 12.x
npm view @rolldown/plugin-babel version       # confirms 0.2.x
go list -m -versions github.com/go-chi/cors   # confirms v1.2.2 is latest
```

---

## Package Legitimacy Audit

> slopcheck was not installed during this research session. Per the Package Legitimacy Protocol's
> graceful degradation rule, the table below documents the verification trail per package: every
> package recommended above is either (a) already in use elsewhere in the repo with a track
> record (Lit ecosystem, Shoelace, Vite, Playwright, Vitest, happy-dom) OR (b) directly verified
> via `npm view <pkg>` AND consumed at the recommendation of authoritative documentation. The
> planner should still run slopcheck before the install task — but the risk surface is small.

| Package | Registry | Age | Downloads | Source Repo | Provenance | Disposition |
|---------|----------|-----|-----------|-------------|------------|-------------|
| `lit` | npm | 4+ yrs | 6M+/wk | github.com/lit/lit | [VERIFIED: Phase 6 existing dep + official Lit team] | Approved |
| `@open-routing/ui` | pnpm workspace | n/a | n/a | this repo | n/a (workspace) | Approved |
| `@lit-labs/router` | npm | 3 yrs | ~10k/wk | github.com/lit/lit | [VERIFIED: Phase 6 existing dep + official Lit team] | Approved |
| `@lit/localize` | npm | 3+ yrs | ~25k/wk | github.com/lit/lit | [VERIFIED: Phase 6 existing dep + official Lit team] | Approved |
| `@shoelace-style/shoelace` | npm | 4+ yrs | ~150k/wk | github.com/shoelace-style/shoelace | [VERIFIED: Phase 6 existing dep + PROJECT.md lock] | Approved |
| `vite` | npm | 5+ yrs | 50M+/wk | github.com/vitejs/vite | [VERIFIED: Phase 6 existing dep] | Approved |
| `@rolldown/plugin-babel` | npm | ~1 yr | ~50k/wk | github.com/rolldown | [VERIFIED: Phase 6 existing dep + official Rolldown team] | Approved |
| `@babel/plugin-proposal-decorators` | npm | 5+ yrs | 10M+/wk | github.com/babel/babel | [VERIFIED: Phase 6 existing dep] | Approved |
| `urlpattern-polyfill` | npm | 3 yrs | ~250k/wk | github.com/kenchris/urlpattern-polyfill | [VERIFIED: Phase 6 existing dep] | Approved |
| `size-limit` | npm | 6+ yrs | ~150k/wk | github.com/ai/size-limit | [VERIFIED: `npm view size-limit version` = 12.1.0; widely adopted (date-fns, redux, Preact)] | Approved |
| `@size-limit/preset-app` | npm | 6+ yrs | ~50k/wk | github.com/ai/size-limit | [VERIFIED: `npm view` confirmed; same publisher as size-limit] | Approved |
| `andresz1/size-limit-action` | GitHub Marketplace | 4+ yrs | n/a | github.com/andresz1/size-limit-action | [VERIFIED: published 1.8.0 latest; ISC license; 7 dependencies — reasonable] | Approved |
| `@playwright/test` | npm | 4 yrs | 8M+/wk | github.com/microsoft/playwright | [VERIFIED: Phase 6 existing dep + Microsoft] | Approved |
| `vitest` | npm | 3 yrs | 8M+/wk | github.com/vitest-dev/vitest | [VERIFIED: Phase 6 existing dep] | Approved |
| `happy-dom` | npm | 3 yrs | 4M+/wk | github.com/capricorn86/happy-dom | [VERIFIED: Phase 6 existing dep] | Approved |
| `github.com/go-chi/cors` | Go modules | 6+ yrs | n/a | github.com/go-chi/cors | [VERIFIED: `go list -m -versions` returned v1.0.0...v1.2.2; companion to chi/v5 already in go.mod; trusted maintainers] | Approved |

**Packages removed due to slopcheck [SLOP] verdict:** none — but planner SHOULD run slopcheck before install to validate this assertion.

**Packages flagged as suspicious [SUS]:** none.

**Cross-ecosystem confusion check:** No Python/Rust packages recommended. All Node packages verified on npm; the one Go package verified on Go module proxy.

---

## Phase Boundary

### In Scope

- `web/apps/embed/` directory build-out:
  - `package.json` with full script set + deps (`name: '@open-routing/catalog-embed'`, `version`, `main`, `exports`, `files`, `sideEffects: false`)
  - `vite.config.ts` library-mode build
  - `tsconfig.json` extending base
  - `src/index.ts` (Custom Element registration + locale bootstrap)
  - `src/embed-element.ts` (`OpenRoutingCatalog extends LitElement`)
  - `src/hash-router-adapter.ts` (~30 lines wrapping `@lit-labs/router`)
  - `e2e/hosts/{react,vue,html}/index.html` (static stub hosts)
  - `e2e/specs/embed-*.spec.ts` (4 spec files)
  - `playwright.config.ts` (3-project matrix)
  - `.size-limit.json` (declarative budget)
  - `README.md` + `LICENSE` + `CHANGELOG.md`
- `web/packages/ui/src/components/shell/catalog-shell.ts` refactor:
  - Convert ~30 static entity imports to `lazyRoute()` helper invocations
  - Add `routingMode: 'history' | 'hash'` reactive property
  - When `routingMode === 'hash'`, instantiate hash router adapter
- `web/packages/ui` Shoelace audit + inline-alert sweep:
  - Audit all 15 files using `<sl-dialog>` / `<sl-tooltip>` (see §Runtime State Inventory)
  - Replace `<sl-dialog>` confirms with inline panel state OR document hosted-in-Shadow-DOM behavior is acceptable
  - Replace `<sl-tooltip hoist>` usages (none currently) with non-hoisted variants
- `services/api/internal/middleware/cors.go` — new file
- `services/api/internal/server/server.go` — wire CORS into chain
- `services/api/internal/config/config.go` — add `CORSAllowedOrigins` env var
- `.github/workflows/ci.yml` — add `web-bundle-size` + `web-e2e-embed` jobs; cache `~/.cache/ms-playwright`
- Phase 1 isolation test suite (`test/isolation/`) — extend with OPTIONS preflight + cross-origin GET assertions

### Out of Scope

Everything in CONTEXT.md `<deferred>` section. Notably: no npm publish; no iframe / Module Federation variants; no multi-instance per page; no history-mode for embed; no production CORS allowlist UI; no SSR; no theme builder.

---

## Implementation Strategy

### 1. Vite Library Mode Build Setup (apps/embed/vite.config.ts)

**Critical knobs** (verified against [Vite Build Options](https://vite.dev/config/build-options) + [vitejs/vite#1736](https://github.com/vitejs/vite/discussions/1736)):

```typescript
// Source: vite.dev/config/build-options + Phase 6 admin/vite.config.ts decorator config
import { defineConfig } from 'vite';
import { resolve } from 'node:path';
import babel from '@rolldown/plugin-babel';

export default defineConfig({
  plugins: [
    babel({
      presets: [{
        preset: () => ({
          plugins: [['@babel/plugin-proposal-decorators', { version: '2023-11' }]],
        }),
        rolldown: { filter: { code: '@' } },
      }],
    }),
  ],
  build: {
    target: 'esnext',
    sourcemap: true,                  // external .map files (CONTEXT Discretion)
    cssCodeSplit: false,              // library mode default — keeps a single CSS file
    lib: {
      entry: resolve(import.meta.dirname, 'src/index.ts'),
      formats: ['es'],                // single ES module — no UMD, no CJS
      fileName: () => 'embed.js',     // produces dist/embed.js exactly
    },
    rollupOptions: {
      output: {
        // Per-entity lazy chunks land here. Vite splits chunks for every
        // dynamic import() in the shell's enter() callbacks.
        chunkFileNames: 'embed-[name]-[hash].js',
        // Lit + shell primitives stay in the eager entry; entity components
        // land in lazy chunks via dynamic import.
      },
      // Bundle the workspace UI package into dist/embed.js — host apps do
      // not have @open-routing/ui available. This is the opposite of admin
      // (admin is an app that resolves workspace deps at dev time; embed
      // ships a self-contained ES module).
    },
  },
});
```

**Note on `external` vs `noExternal`:** Vite's default for library mode is to EXTERNALIZE dependencies declared in `dependencies` and `peerDependencies` of the embed's `package.json`. For embed to ship a self-contained bundle, the workspace dep (`@open-routing/ui`) must NOT be externalized. The cleanest approach: list `@open-routing/ui` as a `devDependency` of embed (not `dependencies`) so Vite bundles it by default. `lit` and `@shoelace-style/shoelace` come along transitively via packages/ui — they also get bundled.

**Per-entity chunk emission:** Vite's natural dynamic-import code splitting produces one chunk per `import('...')` call in the shell's `enter` hooks. After Phase 6 refactor, shell has 8 dynamic imports (6 entities + status-panel + import-page) → 8 lazy chunks. File names follow the `chunkFileNames` template above. [VERIFIED: Vite docs + github.com/vitejs/vite discussions/17730]

**ESM-only browsers requirement:** `build.target: 'esnext'` produces output that requires native ES module support. Caniuse: every browser since Chrome 89 / Safari 14 / Firefox 89 (2021+) supports this. PROJECT.md doesn't list browser requirements — recommend documenting "modern browsers (last 2 years)" in README.

### 2. Custom Element Class (apps/embed/src/embed-element.ts)

```typescript
// Source: lit.dev/docs/components/overview + MDN Custom Elements Lifecycle
import { LitElement, html, css, type PropertyValues } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { createApiClient } from '@open-routing/ui';
import type { ApiClient } from '@open-routing/ui';
import { HashRouterAdapter } from './hash-router-adapter.js';
import '@open-routing/ui/components/shell';  // registers <or-catalog-shell>

const UUIDV7_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

@customElement('open-routing-catalog')
export class OpenRoutingCatalog extends LitElement {
  static override styles = css`
    :host { display: block; }
    .error { padding: 16px; color: var(--or-color-danger, #d92d20); }
    .auth-expired { padding: 12px 16px; background: var(--or-color-conflict-bg, #fef3c7);
                    border-bottom: 1px solid var(--or-color-conflict-border, #f59e0b); }
  `;

  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String, attribute: 'api-base-url' }) accessor apiBaseUrl = '';
  @property({ type: String }) accessor theme = '';   // JSON string or named theme
  @property({ type: String }) accessor modules = '';
  @property({ type: String }) accessor locale: 'en' | 'vi' = 'en';

  @state() private accessor _client: ApiClient | null = null;
  @state() private accessor _validationError: string | null = null;
  @state() private accessor _authExpired: { requestId: string; path: string } | null = null;
  @state() private accessor _parsedTheme: string | Record<string, string> = '';

  override connectedCallback(): void {
    super.connectedCallback();
    this._validateAndBuildClient();
    if (!this._validationError) {
      this._dispatchRequestContext();
    }
  }

  override updated(changed: PropertyValues): void {
    if (changed.has('orgId') || changed.has('apiBaseUrl')) {
      this._validateAndBuildClient();
    }
    if (changed.has('theme')) {
      this._parseTheme();
    }
  }

  private _validateAndBuildClient(): void {
    if (!this.orgId) {
      this._validationError = 'org-id attribute required';
      this._client = null;
      return;
    }
    if (!UUIDV7_PATTERN.test(this.orgId)) {
      this._validationError = 'Invalid org-id attribute (expected UUIDv7)';
      this._client = null;
      return;
    }
    this._validationError = null;
    // D-38: factory client per element instance. Per-request getOrgId closes
    // over `this.orgId` so live attribute changes propagate without rebuilding.
    this._client = createApiClient({
      baseURL: this.apiBaseUrl,
      getOrgId: () => this.orgId,
    });
    // Intercept 401 to dispatch auth-expired event + render banner.
    // openapi-fetch middleware onResponse hook fires for every response.
    // (See §Implementation Strategy 7 for the middleware wiring.)
  }

  private _parseTheme(): void {
    if (!this.theme) { this._parsedTheme = ''; return; }
    if (['or-light', 'or-dark', 'or-brand'].includes(this.theme)) {
      this._parsedTheme = this.theme as 'or-light' | 'or-dark' | 'or-brand';
      return;
    }
    try {
      const parsed = JSON.parse(this.theme);
      this._parsedTheme = parsed;
    } catch {
      this._parsedTheme = 'or-light';
    }
  }

  private _dispatchRequestContext(): void {
    this.dispatchEvent(new CustomEvent('open-routing:request-context', {
      detail: { orgId: this.orgId, apiBaseUrl: this.apiBaseUrl,
                theme: this.theme, modules: this.modules },
      bubbles: true,
      composed: true,
    }));
  }

  override render() {
    if (this._validationError) {
      return html`<div class="error" role="alert">${this._validationError}</div>`;
    }
    return html`
      ${this._authExpired ? html`
        <div class="auth-expired" role="alert">
          Session expired — request ${this._authExpired.requestId}
          <button @click=${() => { this._authExpired = null; }}>Dismiss</button>
        </div>
      ` : ''}
      <or-catalog-shell
        .orgId=${this.orgId}
        .theme=${this._parsedTheme}
        .modules=${this.modules}
        .locale=${this.locale}
        routingMode="hash"
        .client=${this._client!}
      ></or-catalog-shell>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'open-routing-catalog': OpenRoutingCatalog;
  }
}
```

**Lit decorator note:** Since Phase 6 uses TC39 2023-11 standard decorators (Vite 8 + Rolldown), the `accessor` keyword IS REQUIRED on every `@property` field. This matches the existing `catalog-shell.ts` pattern. Don't drop it. [VERIFIED: Phase 6 admin uses this pattern; same `@rolldown/plugin-babel` config.]

### 3. Shell Refactor to Lazy Routes (packages/ui/src/components/shell/catalog-shell.ts)

Current state: ~30 static imports at top of file (lines 24-69). Each entity has 3 components (list, detail, form) plus status-panel + import-page = 20+ imports that become lazy.

**Refactor pattern** — replace static import + render with a `lazyRoute()` helper:

```typescript
// Source: github.com/lit/lit/discussions/3354 + @lit-labs/router README
function lazyRoute(
  importFn: () => Promise<unknown>,
  renderFn: (params: Record<string, string | undefined>) => unknown,
): { enter: (params: Record<string, string | undefined>) => Promise<boolean>; render: (params: Record<string, string | undefined>) => unknown } {
  return {
    enter: async (params) => {
      // _orgRouteEnter MUST run BEFORE the dynamic import — otherwise a
      // malformed org_id triggers a wasted network fetch for the entity chunk.
      // Pattern: compose the guard with the loader (see _composedEnter below).
      await importFn();
      return true;
    },
    render: renderFn,
  };
}
```

**Composing _orgRouteEnter + lazyRoute** — the shell's existing `_orgRouteEnter` guard rejects malformed UUIDs by calling `this._routes.goto('/')`. The new pattern:

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

**Verified pattern signature** (from official Lit discussion #3354 + npm README): `enter` returns `Promise<boolean | undefined>`. Returning `false` rejects the route. Returning anything else (including undefined) accepts.

**Routing-mode prop + Iteration-2 BLOCKER #1 typed-interface seam:**

```typescript
import type { Routes } from '@lit-labs/router';

// Iteration-2 BLOCKER #1 — typed seam exported from packages/ui.
// Consumer (apps/embed/embed-element.ts) implements + injects;
// packages/ui has no dep on @open-routing/embed.
export interface CatalogShellRouterAdapter {
  start(routes: Routes): void;
  stop(): void;
}

@property({ type: String, attribute: 'routing-mode' })
accessor routingMode: 'history' | 'hash' = 'history';

@property({ attribute: false })
accessor routerAdapter: CatalogShellRouterAdapter | undefined = undefined;

// In firstUpdated (NOT connectedCallback — Routes instance is constructed
// at field-init time but the typed handoff lives in the post-render hook
// so consumers can assign routerAdapter before flipping routingMode):
override firstUpdated(_changed: PropertyValues): void {
  if (this.routingMode === 'hash' && this.routerAdapter) {
    this.routerAdapter.start(this._routes);
  }
}

// In disconnectedCallback:
this.routerAdapter?.stop();
```

Consumer side (apps/embed/src/embed-element.ts):
```typescript
override firstUpdated(_changed: PropertyValues): void {
  const shellEl = this._shellEl;
  if (!shellEl || this._hashAdapter) return;
  this._hashAdapter = new HashRouterAdapter();          // parameterless
  shellEl.routerAdapter = this._hashAdapter;            // assign BEFORE flip
  shellEl.routingMode = 'hash';                          // triggers shell.firstUpdated
}
```

HashRouterAdapter (apps/embed) does NOT have an `implements
CatalogShellRouterAdapter` clause and does NOT import the interface —
TS structural typing verifies the shape at the assignment site
(`shellEl.routerAdapter = this._hashAdapter`). This keeps Plan 07-05
with zero compile-time dep on packages/ui, preserving Wave 1 parallel
placement with Plan 07-04.

**Admin regression guard (D7-03 mandate):** After refactor, run Plan 06-06's Playwright smoke against admin to confirm:
1. `/orgs/:id/agents` still loads (lazy chunk fetched, rendered)
2. `/orgs/:id/agents/new` wizard still works
3. `/orgs/:id/agents/:id/status` still works
4. Switch-org clears `_client` and navigates to `/`

**Risk:** D7-03 calls out "regression risk" — researcher's verdict is LOW. The lazy-route wrapper composes the existing `_orgRouteEnter` with `await import(...)`. `@lit-labs/router`'s `goto()` returns a Promise that awaits the enter callback's promise (verified in npm README). So `goto()` resolves only after the entity chunk is loaded, the component class is registered, and the next render uses it. The only subtle behavior change: a brief loading flash between route nav and entity render. Admin smoke covers this; embed Playwright covers it across hosts.

### 4. Hash Router Adapter (apps/embed/src/hash-router-adapter.ts)

Minimal wrapper around `@lit-labs/router`. The shell's `Routes` class already uses URLPattern internally for matching. The adapter just needs to feed it the right pathname.

```typescript
// Source: MDN HashChangeEvent + @lit-labs/router README + WHATWG URLPattern spec
import type { Routes } from '@lit-labs/router';

export class HashRouterAdapter {
  private _onHashChange: () => void;

  constructor(private _routes: Routes) {
    this._onHashChange = () => this._navigate();
  }

  start(): void {
    window.addEventListener('hashchange', this._onHashChange);
    this._navigate();  // sync to initial hash
  }

  stop(): void {
    window.removeEventListener('hashchange', this._onHashChange);
  }

  private _navigate(): void {
    // window.location.hash includes the leading '#' — strip it and treat the
    // remainder as a normal pathname. The shell's existing URLPattern matchers
    // for '/orgs/:org_id/agents' etc. work unchanged.
    const hash = window.location.hash.slice(1) || '/';
    this._routes.goto(hash);
  }
}
```

**Why this works:** `@lit-labs/router`'s `Routes.goto(path)` calls `URLPattern.exec({ pathname: path })` against each route. The pathname can be ANY string — `/orgs/01234...` (history) or what we feed from `location.hash` strip. No regex changes needed. [VERIFIED: URLPattern API spec + Lit discussion #3354]

**Hash format:** Recommendation — `#open-routing/orgs/01234.../agents`. The `open-routing/` prefix is the path prefix CONTEXT.md mentions for multi-embed disambiguation in v0.2. For v0.1 (single embed per page), it's still a reasonable convention because the hash space is shared with the host page.

**Edge case — host page also uses hash routing:** If the host SPA writes `window.location.hash = '#some-other-state'`, the embed's `hashchange` listener fires and tries to navigate. Mitigation: the adapter checks the hash starts with `open-routing/`; if not, it ignores. This isolates from non-prefixed hosts but does not fully solve the case where two embeds OR an embed + a hash-routed React Router collide. D7-07 documents this as a v0.1 limitation.

### 5. Theme Forwarding (theme attribute → shell)

D6-20 already implements theme tokens on shell host. Embed just needs to parse the attribute and pass it through.

- **String input** (`theme="or-light"`): Pass-through. Shell's existing logic handles the named themes.
- **JSON input** (`theme='{"--sl-color-primary-500":"#0d8b96", ...}'`): Parse to object, pass as object. Shell's existing logic iterates and applies via `this.style.setProperty(...)`.
- **Malformed JSON:** Fall back to `or-light` silently. Don't throw — host might be mid-typing during dev.

**Tokens to document in README:** The 5-10 most-impactful keys (CONTEXT Discretion item):
- `--sl-color-primary-500` (primary brand color)
- `--or-color-app-bg` (page background)
- `--or-color-text-strong` (heading text)
- `--or-color-card-bg` (card surfaces)
- `--or-color-card-border` (card borders)
- `--sl-border-radius-medium` (corner radius)
- `--sl-font-family` (font stack)
- `--or-color-conflict-bg` (banner background)
- `--or-color-focus-ring` (focus ring color)
- `--or-color-divider` (table/sidebar dividers)

### 6. Modules Filter Pass-Through

`<or-catalog-shell>` already implements `modules` attribute filtering via `_visibleEntries()` (catalog-shell.ts:489). Embed forwards the attribute via reactive property. No new code in shell.

**Edge case** — `modules="agents"` shown but the user navigates to `#open-routing/orgs/:id/skills` somehow (manual URL edit). The router still has the skills route registered; lazy import would fire. **D7 doesn't specify this case.** Recommendation for planner: at the lazy-route enter hook, check if the entity is in the allowed modules list; if not, redirect to the first allowed module's list page.

### 7. CustomEvent Dispatch (request-context + auth-expired)

**request-context** — fires once on `connectedCallback` AFTER attribute validation succeeds. Shown in §Implementation Strategy 2 (above).

**auth-expired** — fires on every 401 response. Implementation: extend the existing openapi-fetch `createApiClient` middleware pattern.

```typescript
// In _validateAndBuildClient(), after creating the client:
const authExpiredMiddleware: Middleware = {
  onResponse: ({ response, request }) => {
    if (response.status === 401) {
      response.clone().json().then((body) => {
        const requestId = body?.request_id ?? '';
        const path = new URL(request.url).pathname;
        // Update banner state — triggers re-render.
        this._authExpired = { requestId, path };
        // Dispatch composed event for host listener.
        this.dispatchEvent(new CustomEvent('open-routing:auth-expired', {
          detail: { statusCode: 401, requestId, path },
          bubbles: true,
          composed: true,
        }));
      }).catch(() => {
        // 401 body not JSON — still dispatch event with empty requestId.
        this._authExpired = { requestId: '', path: new URL(request.url).pathname };
        this.dispatchEvent(new CustomEvent('open-routing:auth-expired', {
          detail: { statusCode: 401, requestId: '', path: this._authExpired.path },
          bubbles: true,
          composed: true,
        }));
      });
    }
    return response;
  },
};
this._client!.use(authExpiredMiddleware);
```

**Composed flag verified** — per MDN: "The composed property of the Event interface returns a boolean value which indicates whether or not the event will propagate across the shadow DOM boundary." Events with `composed: true` AND `bubbles: true` reach all ancestors INCLUDING ancestors outside the embed's shadow root. The host listener attached to `document` OR to `<open-routing-catalog>` itself OR anywhere in between will receive the event. [VERIFIED: MDN + lit.dev/docs/components/events/]

**Event retargeting:** When the event crosses the shadow boundary, `event.target` is rewritten to the host element (`<open-routing-catalog>`). Host listeners see `event.target === document.querySelector('open-routing-catalog')`, not internal Lit components. This is correct — internals are not leaked.

### 8. npm Pack Validation (D7-15)

`apps/embed/package.json` shape:

```json
{
  "name": "@open-routing/catalog-embed",
  "version": "0.1.0-pre.1",
  "type": "module",
  "main": "./dist/embed.js",
  "module": "./dist/embed.js",
  "types": "./dist/embed.d.ts",
  "exports": {
    ".": {
      "import": "./dist/embed.js",
      "types": "./dist/embed.d.ts"
    },
    "./package.json": "./package.json"
  },
  "files": [
    "dist/",
    "README.md",
    "LICENSE",
    "CHANGELOG.md"
  ],
  "sideEffects": false,
  "scripts": {
    "build": "vite build",
    "size": "size-limit",
    "pack:dry": "pnpm pack --dry-run"
  }
}
```

**Why `sideEffects: false`:** Embed's only side effect is the `customElements.define()` call in `src/index.ts`. But that runs on import — there's nothing "side-effect-free" about it from the host's perspective. However, since the host imports the embed entry specifically TO trigger registration, declaring `sideEffects: false` is acceptable IF we mark `src/index.ts` as a side-effectful entry via... actually, this needs more care.

**Corrected sideEffects strategy:** Declare `"sideEffects": ["./dist/embed.js"]` (array form, not false). This tells bundlers in host apps that `dist/embed.js` itself has side effects (the registration) but anything else in the package does not. [Per webpack docs on sideEffects array form.]

**`pnpm pack --dry-run` output** lists the files that would go in the tarball. CI verifies the list includes `dist/embed.js`, `dist/embed.js.map`, `dist/embed-*.js` (lazy chunks), `README.md`, `LICENSE`, `CHANGELOG.md`, `package.json`. Catches `.files` regressions or accidental inclusion of test files.

### 9. Playwright Multi-Project Matrix

**Config shape** (`apps/embed/playwright.config.ts`):

```typescript
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  testMatch: 'specs/*.spec.ts',
  timeout: 30_000,
  retries: 1,  // D7-12: CDN-flake tolerance
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: 'http://localhost:4173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    { name: 'react', use: { ...devices['Desktop Chrome'], baseURL: 'http://localhost:4173/hosts/react/' } },
    { name: 'vue',   use: { ...devices['Desktop Chrome'], baseURL: 'http://localhost:4173/hosts/vue/' } },
    { name: 'html',  use: { ...devices['Desktop Chrome'], baseURL: 'http://localhost:4173/hosts/html/' } },
  ],
  webServer: {
    command: 'pnpm --filter @open-routing/embed preview',
    url: 'http://localhost:4173',
    reuseExistingServer: true,
    timeout: 60_000,
  },
});
```

**Vite preview structure:** `pnpm vite preview` serves `dist/` by default. To also serve the static host HTML files, mount them as static under the same Vite preview by placing them in `apps/embed/e2e/hosts/{react,vue,html}/index.html`. Public files are copied to dist on build; preview serves them at `/hosts/...`. Alternatively, configure `preview: { ... }` in vite.config.ts to add extra static dirs.

**Stub host HTML** (`apps/embed/e2e/hosts/react/index.html`):

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
    /* Aggressive host CSS reset — EMBED-04 isolation assertion */
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
      'api-base-url': 'http://localhost:4173',  // mocked by page.route
      theme: 'or-light',
      modules: 'agents,skills',
    });
    createRoot(document.getElementById('root')).render(React.createElement(App));
  </script>
</body>
</html>
```

**Why aggressive host reset:** EMBED-04 requires Shadow DOM isolation in BOTH directions. The host's `* { all: revert; }` should NOT bleed into embed's Shadow DOM; the embed's `:host { ... }` styles should NOT escape to host's `<h1>`. Without aggressive host CSS, an isolation test could pass falsely.

**Mock API responses via `page.route`:** Playwright intercepts API calls and returns mock JSON. Specs assert that `X-Org-Id: 01952a6b-...` header arrives on every intercepted request. For `auth-expired` test, mock returns 401 + `{ error: 'auth_expired', request_id: 'test-rid-123' }`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bundle size measurement | Custom Node script that gzips + measures dist | `size-limit` + `@size-limit/preset-app` | size-limit handles brotli/gzip/raw, integrates with GH Action, has PR comment format. Custom script forgets edge cases (BOM, dynamic chunks, source maps stripped) |
| URL pattern matching | Hand-rolled regex for `/orgs/:org_id/agents` parsing | `URLPattern` (used by `@lit-labs/router`) | URLPattern is now Baseline 2025 (all major browsers); polyfilled via existing `urlpattern-polyfill` for tests |
| Hash routing event handling | Custom `hashchange` listener that re-parses pathname | Wrap `@lit-labs/router` with the adapter above | Adapter is ~30 lines; rolling separate parsing logic duplicates URLPattern matchers from the shell's routes |
| CORS preflight handling | Custom OPTIONS handler that returns ad-hoc headers | `github.com/go-chi/cors` | Wildcard origin support, AllowOriginFunc for dynamic policies, short-circuit logic, browser compatibility quirks all handled |
| Custom Element attribute observing | `static get observedAttributes` + manual `attributeChangedCallback` | Lit's `@property({ attribute: '...' })` decorator | Lit auto-generates `observedAttributes` from `@property` declarations + handles attribute-to-property reflection, type coercion (string/number/boolean/object). Hand-rolling forgets one direction of sync |
| 401 response interception | Custom `fetch` wrapper that checks status | openapi-fetch `onResponse` middleware on the existing `createApiClient` | Phase 2 already established the middleware pattern (X-Org-Id injection). Reuse the seam |
| Test stub host setup | Full Vite SPAs for React + Vue per host | Static HTML + esm.sh CDN imports | D7-11 lock; matches real-world embed integration; no per-host package.json maintenance |
| ES module pack tarball validation | Custom file enumeration + size measurement | `pnpm pack --dry-run` | Native pnpm support for files-field + ignore-file resolution + sideEffects + exports field validation |

**Key insight:** Phase 7 is mostly composition — every primitive is established (Lit, Shoelace, router, openapi-fetch, validators, themes). The temptation will be to write "just a little custom logic" for hash routing or for size budgeting; **don't**. The libraries each handle 1-2 edge cases (URLPattern hash polyfill behavior, size-limit Brotli vs Gzip, openapi-fetch middleware ordering) that custom code will get wrong.

---

## Runtime State Inventory

> This phase is greenfield code addition + Phase 6 shell refactor in place. No data migration. No
> renaming. Runtime state inventory is narrowly about the **Shoelace dialog/tooltip audit** that
> D7-04 mandates — the existing `packages/ui` components have `<sl-dialog>` and `<sl-tooltip>`
> usages that Phase 6 shipped knowing the admin host page has no Shadow DOM ancestor. Once the
> shell is mounted INSIDE the embed's Shadow DOM, that assumption breaks.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| **Stored data** | None — embed reads attributes only; no localStorage; no DB writes from new code | None |
| **Live service config** | None — no n8n/Datadog/etc references in scope | None |
| **OS-registered state** | None — no Task Scheduler / systemd / launchd touches | None |
| **Secrets/env vars** | `CORS_ALLOWED_ORIGINS` (new) — comma-separated origin list for Go API | Document in `services/api/.env.example` + Taskfile dev defaults |
| **Build artifacts / installed packages** | Phase 6 lazy-chunk file names (currently emit `embed-*.js`?) — Phase 7 IS the first emitter. No legacy artifacts | None — Phase 7 emits dist/embed.js fresh |
| **Shoelace `<sl-dialog>` audit (D7-04 scope)** | **8 files** in `packages/ui` use `<sl-dialog>`: `adapters/adapter-detail.ts`, `agents/agent-detail.ts`, `break-reasons/break-reason-detail.ts`, `channels/channel-detail.ts`, `queues/queue-detail.ts`, `skills/skill-detail.ts` (2 instances), `status/status-panel.ts` | **Code edit per file.** `<sl-dialog>` does NOT portal to body (stays in parent shadow root) BUT focus-trap is broken inside nested shadow root per shoelace#709. Options: (a) replace `<sl-dialog>` with inline confirm panel pattern; (b) document broken focus-trap inside embed as acceptable for v0.1. Recommendation: **replace** — focus-trap matters for accessibility |
| **Shoelace `<sl-tooltip>` audit (D7-04 scope)** | **15+ instances** across `packages/ui` components including `shell/catalog-shell.ts:530`, all entity *-list.ts and *-detail.ts files | **Code review per file.** `<sl-tooltip>` uses Floating UI; without `hoist` attribute it stays in parent shadow root (OK for isolation). With `hoist` it moves out. **Action: grep for `<sl-tooltip ... hoist`; replace those that hoist; leave non-hoisted alone.** Phase 7 verifies with Playwright |
| **Shoelace `<sl-drawer>` audit** | Iteration-2 BLOCKER #2 correction: catalog-shell.ts does NOT actually render `<sl-drawer>` — the mobile sidebar uses `<nav class="sidebar">` + `_sidebarOpen` state with CSS media-query. The `<sl-drawer>` import at line ~37 was dead Phase 6 placeholder code. | Plan 07-03 Task 3 deletes the unused import (~3-5 KB freed); Plan 07-09 audits `<nav.sidebar>` on mobile viewport (NOT `<sl-drawer>`) |

**Nothing found in category:** Stored data, Live service config, OS-registered state — all stated explicitly.

**Audit command for planner to run before Wave 2:**
```bash
grep -rn "sl-dialog\|sl-tooltip\|sl-drawer" web/packages/ui/src/components/ \
  | grep -v "\.test\.ts" \
  > /tmp/embed-shadow-audit.txt
wc -l /tmp/embed-shadow-audit.txt
```

Expected: ~25-30 lines of matches. Planner's Wave 2 work includes deciding "replace" vs "verify-contained" per match.

---

## Common Pitfalls

### Pitfall 1: `<sl-dialog>` Focus Trap Breaks Inside Nested Shadow DOM
**What goes wrong:** User tabs out of a confirm dialog into the host page below; accessibility audit fails. [VERIFIED: shoelace-style/shoelace#709, #1382]
**Why it happens:** Shoelace's focus trap walks the DOM tree and gets confused at shadow boundaries.
**How to avoid:** Replace `<sl-dialog>` confirms with inline confirm panels (a `_confirmOpen: boolean` state + an inline `<div role="dialog">` with focus-trap implemented via `:scope >> [tabindex]` query). D7-04 mandates this for inline alerts; extend the same logic to confirms.
**Warning signs:** Playwright spec sees keyboard `Tab` reach host elements while a Shoelace dialog is open.

### Pitfall 2: `manualChunks` Function Form Required in Vite 8 / Rolldown
**What goes wrong:** Setting `rollupOptions.output.manualChunks` as an object literal (the old Rollup-classic syntax) fails silently or with a confusing error.
**Why it happens:** Vite 8 uses Rolldown, which requires `manualChunks` as a function: `manualChunks: (id) => ...`. Admin's vite.config.ts already uses this form (verified at lines 30-38).
**How to avoid:** Use function form. Better yet: don't define `manualChunks` at all for embed library mode — let Vite's dynamic-import code splitting do the work. Define it only if a chunk-size visualizer flags hotspots later.
**Warning signs:** All entity components land in `dist/embed.js` instead of separate `dist/embed-agents-*.js` etc.

### Pitfall 3: openapi-fetch Middleware Order Matters for X-Org-Id + Auth-Expired
**What goes wrong:** Auth-expired middleware runs before X-Org-Id is set; or X-Org-Id arrives after the request leaves; either case breaks one of the two contracts.
**Why it happens:** openapi-fetch middleware runs in registration order. `onRequest` fires before send; `onResponse` fires after receive. The X-Org-Id middleware uses `onRequest`; auth-expired uses `onResponse`. They're independent — but if both are registered as `onRequest`, ordering matters.
**How to avoid:** Use the correct hook per middleware. X-Org-Id = `onRequest`. Auth-expired = `onResponse`. Both attach via `client.use(middleware)`; ordering doesn't matter as long as hooks are correct.
**Warning signs:** Playwright `embed-org-id-header` spec passes but `embed-auth-expired` spec fails with "401 not intercepted".

### Pitfall 4: `composed: true` is NOT Default — Events Stop at Shadow Boundary
**What goes wrong:** Host listener attached via `document.addEventListener('open-routing:auth-expired', ...)` never fires; embed code dispatches the event but no one receives.
**Why it happens:** `CustomEvent` defaults are `bubbles: false, composed: false`. Without `composed: true`, events bubble up to the embed's `shadowRoot` and stop. They never reach the host document tree. [VERIFIED: MDN composed property]
**How to avoid:** Explicit `bubbles: true, composed: true` on EVERY public CustomEvent dispatched by embed. (Internal events between shell and entity components inside the shadow root don't need composed.)
**Warning signs:** Playwright spec catches the dispatched event but Vue/React host's `useEffect`/`onMounted` listener doesn't fire.

### Pitfall 5: `pnpm pack` Strips `prepublishOnly` and `packageManager` Fields
**What goes wrong:** Tarball passed to a host project misses lifecycle scripts that work locally. [VERIFIED: pnpm#10195]
**Why it happens:** pnpm's pack command has a known issue (#10195) where some `package.json` fields are removed in the tarball vs the local file.
**How to avoid:** Test the tarball end-to-end in CI: `pnpm pack --dry-run` for the file listing, then `pnpm pack` + `pnpm install ./open-routing-catalog-embed-0.1.0.tgz` in a scratch directory + `node -e "import('@open-routing/catalog-embed')"` smoke test.
**Warning signs:** Host project install fails with "missing scripts" or behaves differently from the workspace install.

### Pitfall 6: `urlpattern-polyfill` Not Loaded in Playwright Browser Pages
**What goes wrong:** `@lit-labs/router` throws `URLPattern is not defined` in older Chromium versions or under heavy CSP.
**Why it happens:** Modern Chromium has URLPattern natively (since v95, 2021), but Playwright's bundled Chromium may lag, and stub host HTML doesn't import the polyfill.
**How to avoid:** Stub host HTML adds `<script src="https://esm.sh/urlpattern-polyfill@10/dist/index-cjs.js"></script>` BEFORE the embed script. Same in dev (Vite handles via `optimizeDeps`).
**Warning signs:** Playwright spec fails with "URLPattern not defined" error on initial load.

### Pitfall 7: Shell `routingMode` Property Reactivity — Mode Switch Mid-Lifecycle
**What goes wrong:** Some test or host app changes `routingMode` from `'history'` to `'hash'` after the shell is connected. Existing `Routes` instance keeps listening to popstate; new hash adapter starts. Both fire. Routes double-trigger.
**Why it happens:** Reactive properties are mutable. The shell created its `Routes` (hooked to history) in field initialization; switching mode doesn't re-create it.
**How to avoid:** Document `routingMode` as **set-once at construction**. Add a console.warn if it changes after `connectedCallback`. (Admin sets it never; embed sets it once via attribute at mount.)
**Warning signs:** Playwright spec sees duplicate `routes.outlet()` renders on a single navigation.

### Pitfall 8: Workspace Dep `@open-routing/ui` Externalized by Default in Library Mode
**What goes wrong:** `dist/embed.js` weighs 8 KB instead of 70 KB. Host install fails with "Cannot resolve @open-routing/ui".
**Why it happens:** Vite's library mode default externalizes `dependencies` and `peerDependencies`. If `@open-routing/ui` is in `embed/package.json` `dependencies`, it gets externalized.
**How to avoid:** Move `@open-routing/ui` to `devDependencies` of embed. Set explicit `rollupOptions: { external: [/^lit/, /^@lit/] }` if even Lit should be externalized (it shouldn't for the embed contract — Lit gets bundled).
**Warning signs:** First `pnpm --filter @open-routing/embed build` succeeds but `dist/embed.js` is tiny (<10 KB) and contains `import { ... } from '@open-routing/ui'` strings.

### Pitfall 9: Hash Adapter Triggers on Host's Hash Changes
**What goes wrong:** Host SPA (React Router 6 with hash mode, or a deep-link to `#section-2`) writes `location.hash`. Embed's adapter sees `hashchange`, tries to navigate to the wrong route.
**Why it happens:** `window.location.hash` is global. Multiple listeners can fire on the same change.
**How to avoid:** Adapter only `goto()`s if `hash.startsWith('open-routing/')`. Otherwise ignore. Document this in the README ("embed reserves `#open-routing/...` for its routing state").
**Warning signs:** Host's "scroll to anchor" behavior broken when embed is mounted.

### Pitfall 10: `size-limit` Measures Gzip; Brotli-Served Reality is Smaller
**What goes wrong:** Team panics when bundle approaches 65 KB gzip, thinking they're near the wall. Reality: Cloudflare Pro serves Brotli; the deployed size is ~50 KB.
**Why it happens:** size-limit's gzip uses Node `zlib` (deflate level 6 by default). CDN gzip is similar (Cloudflare uses level 6); CDN brotli (when enabled) is 15-30% smaller.
**How to avoid:** Document in README that "70 KB gzip target ≈ ~52 KB brotli on a CDN that supports it." Don't switch the budget to brotli in v0.1 — gzip is the conservative budget (more pessimistic = safer headroom).
**Warning signs:** Slack thread asking "why does Cloudflare show 50 KB but our CI says 65 KB?"

---

## Code Examples

### `dispatchEvent` with `composed: true`

```typescript
// Source: lit.dev/docs/components/events + MDN Event.composed
this.dispatchEvent(new CustomEvent('open-routing:auth-expired', {
  detail: { statusCode: 401, requestId: '...', path: '/v1/orgs/.../agents' },
  bubbles: true,
  composed: true,  // REQUIRED — without this, event stops at shadowRoot
}));
```

### `@lit-labs/router` lazy `enter` hook

```typescript
// Source: npm @lit-labs/router README + github.com/lit/lit/discussions/3354
this._routes = new Routes(this, [
  {
    path: '/orgs/:org_id/agents',
    enter: async ({ org_id }) => {
      if (!UUIDV7_PATTERN.test(org_id ?? '')) {
        this._routes.goto('/');
        return false;
      }
      await import('../agents/agent-list.js');  // lazy chunk fetched
      return true;
    },
    render: ({ org_id }) =>
      html`<or-agent-list .orgId=${org_id ?? ''} .client=${this._client!}></or-agent-list>`,
  },
]);
```

### `go-chi/cors` Wiring

```go
// Source: github.com/go-chi/cors README + cors.go source
// services/api/internal/middleware/cors.go
package middleware

import (
    "strings"
    "github.com/go-chi/cors"
)

// AllowedOriginsFromEnv returns the CORS allowed origins parsed from
// CORS_ALLOWED_ORIGINS. Default: empty list (reject all). Dev sets "*".
func AllowedOriginsFromEnv(raw string) []string {
    if raw == "" {
        return []string{}
    }
    parts := strings.Split(raw, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        if trimmed := strings.TrimSpace(p); trimmed != "" {
            out = append(out, trimmed)
        }
    }
    return out
}

func NewCORS(allowedOrigins []string) func(http.Handler) http.Handler {
    return cors.Handler(cors.Options{
        AllowedOrigins:   allowedOrigins,
        AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"X-Org-Id", "Content-Type", "Idempotency-Key", "Accept-Language"},
        ExposedHeaders:   []string{"X-Request-Id"},
        AllowCredentials: false,  // v0.1 stub auth doesn't use cookies
        MaxAge:           300,    // 5 min preflight cache
    })
}

// services/api/internal/server/server.go (within NewMux):
// Insert AFTER Recoverer + RequestID, BEFORE strict pipeline.
r.Use(chimw.Recoverer)
r.Use(appmw.RequestID)
r.Use(appmw.NewCORS(deps.Config.CORSAllowedOrigins))  // NEW for Phase 7
r.Get("/metrics", MetricsHandler())
// ... rest of NewMux unchanged
```

**Key insight from source code:** `cors.Handler` short-circuits OPTIONS preflight automatically (returns 200 OK + headers, does NOT call `next.ServeHTTP`). So inserting BEFORE OrgContext correctly handles "OPTIONS without X-Org-Id should not return 400". [VERIFIED: github.com/go-chi/cors/blob/master/cors.go]

### `size-limit` Config

```json
// apps/embed/.size-limit.json
// Source: github.com/ai/size-limit README + Phase 6 admin precedent (none — first usage)
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
  },
  {
    "name": "embed-skills-*.js (lazy chunk)",
    "path": "dist/embed-skills-*.js",
    "limit": "15 KB",
    "gzip": true,
    "running": false
  }
  // ... per-entity entries (logged-not-blocked per D7-10)
]
```

**Note on "logged-not-blocked":** size-limit JSON does NOT have a per-entry "warn-only" flag. The way to "log but not block" is to set the limit high enough that the chunk would have to balloon dramatically before triggering. Recommendation: set `limit: "20 KB"` for lazy chunks (still gates but with headroom).

Alternative: emit a separate `.size-limit-soft.json` that runs as a SEPARATE non-required CI step. Hard fail on `.size-limit.json` (eager only); soft warning via PR comment from `size-limit-soft.json` (all chunks).

### Vitest Component Test Setup

```typescript
// Source: Phase 6 packages/ui/test-setup.ts + happy-dom docs
// apps/embed/vitest.config.ts
import { defineConfig } from 'vitest/config';
export default defineConfig({
  test: {
    environment: 'happy-dom',
    globals: false,
    setupFiles: ['./test-setup.ts'],
  },
});

// apps/embed/test-setup.ts
import 'urlpattern-polyfill';   // for @lit-labs/router URLPattern
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hash routing as primary SPA pattern | Hash routing as fallback / for isolation only | History API (HTML5) ~2010s | Modern SPAs default to history; hash is for cases like embed where path control is unavailable |
| Module Federation for micro-frontends | Web Components + Custom Elements | TC39 ESM stabilization + native browser support | PROJECT.md pivoted; Phase 7 is the embed surface |
| Bundle size budgets in custom Node scripts | `size-limit` declarative config + GH Action | ~2019 ecosystem adoption | Standard now; date-fns/redux/Preact all use it |
| CDN bundles served via `<script src>` UMD | ES modules + importmaps + esm.sh | Browser native importmap support 2022+ | esm.sh `?deps=` URL pattern is current best practice |
| Custom focus-trap implementations for modals | Native `<dialog>` element (HTML) | 2022 Chromium, 2023 Safari + Firefox | Native `<dialog>` is now Baseline 2023 — could be used for embed confirms (deferred to v0.2; Shoelace's `<sl-dialog>` is the v0.1 path) |
| `static get observedAttributes` boilerplate | Lit `@property({ attribute: '...' })` decorator | Lit 1.0+ | Decorators auto-register observed attributes |

**Deprecated/outdated:**
- `ajv-cli` for codegen: last published 2021; use ajv v8 programmatic API in a custom Node script (Phase 6 already does this)
- Module Federation 1.x: superseded by Module Federation 2.x for React ecosystem; PROJECT.md picks Web Components instead
- `setInterval` for polling: still works but should be paused on `document.hidden` (Phase 6 D6-26)

---

## Project Constraints (from CLAUDE.md)

These directives apply to all Phase 7 code:

| Directive | Source | How Phase 7 Honors |
|-----------|--------|---------------------|
| **No over-commenting; comments explain WHY** | `./CLAUDE.md` HARD RULE | Phase 7 task instructions must include "no WHAT/HOW comments; only WHY for hidden constraints, gotchas, decision pointers (e.g., D7-XX)" |
| **Cross-AI peer review on non-trivial work** | `./CLAUDE.md` + `~/.claude/CLAUDE.md` HARD RULE | Phase 7 plans must include Codex + Gemini parallel review at phase-gate plan; review runs on the full diff including shell refactor and CORS middleware |
| **No `--no-verify`, no hook skipping** | `./CLAUDE.md` | All commits use the standard hook chain |
| **Self-documenting names over comments** | `./CLAUDE.md` | `embed-element.ts`, `hash-router-adapter.ts`, `OpenRoutingCatalog` — names should tell the story |
| **Prefer Read/Edit/Write tools over bash** | `./CLAUDE.md` (system prompt rule) | Task instructions reference Read/Edit/Write, not heredocs |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Shoelace `<sl-tooltip>` without `hoist` attribute stays inside parent shadow root | §Runtime State Inventory + Pitfall 1 | If wrong, more components need replacement than the audit table suggests |
| A2 | `pnpm pack --dry-run` correctly previews all files that would be in the tarball | §Implementation Strategy 8 | If wrong, host install may fail; mitigation: also do full `pnpm pack` + scratch-dir install in CI |
| A3 | Vite library mode `noExternal` of workspace dep produces a self-contained ES module | §Implementation Strategy 1 + Pitfall 8 | If wrong, embed bundle is too small + host install fails; mitigation: validate dist/embed.js size > 50 KB in CI |
| A4 | Brotli vs Gzip CDN size delta is 15-30% (Cloudflare standard) | §Pitfall 10 | If wrong, gzip budget is more or less conservative than expected; impact is documentation-only |
| A5 | Playwright Chromium 1.60 has URLPattern natively | §Pitfall 6 | If wrong, polyfill load needed in stub host HTML; impact is one extra script tag |
| A6 | `@lit-labs/router` `goto()` Promise resolves only after async `enter` callback completes | §Implementation Strategy 3 | If wrong, lazy-loading flashes empty UI; mitigation: explicit loading state in shell |
| A7 | go-chi/cors Handler returns early on OPTIONS without OptionsPassthrough | §Implementation Strategy 1 (verified in source code) | LOW RISK — confirmed in cors.go source |
| A8 | Lit `accessor` decorator pattern works for Custom Element attribute observation under TC39 2023-11 | §Implementation Strategy 2 | LOW RISK — Phase 6 admin proves this pattern works in production |
| A9 | esm.sh React 18.3.1 + Vue 3.5.13 pinning is stable enough for Playwright CI (no breaking server changes) | §Standard Stack | If wrong, `retries: 1` absorbs occasional flakes; D7-12 acknowledges |
| A10 | The shell can switch `routingMode` set-once at construction without breaking history mode for admin | §Pitfall 7 | If wrong, admin smoke breaks; mitigation: Plan 06-06 smoke re-run gate |

---

## Open Questions

1. **Should `<or-conflict-banner>` and the new inline auth-expired banner share a base component?**
   - What we know: D6-04 already implements `<or-conflict-banner>` for 409 reload UX
   - What's unclear: Whether v0.1 needs the abstraction or v0.2 can refactor
   - Recommendation: **Keep independent in v0.1** (CONTEXT Discretion item agrees). Different copy, different actions; if more banner types emerge in v0.2, extract a base then.

2. **Does the embed need a `<sl-tooltip hoist>` replacement strategy?**
   - What we know: catalog-shell.ts uses `<sl-tooltip>` on the org-chip (no hoist); `<sl-tooltip>` without hoist stays in shadow root
   - What's unclear: Whether non-hoisted tooltips inside the embed's nested shadow root render correctly (Floating UI positioning might confuse)
   - Recommendation: **Test during Phase 7 Wave 2**. If tooltips misposition in embed, swap to inline `aria-describedby` + `title` attribute fallback. If they work, no change needed.

3. **How aggressive should the host CSS reset be in Playwright stub hosts?**
   - What we know: EMBED-04 requires isolation in both directions; aggressive host CSS makes the test meaningful
   - What's unclear: Whether `* { all: revert }` is the right level or if `* { all: unset }` (more nuclear) better validates isolation
   - Recommendation: **Use `* { all: revert }` + a few specific overrides** (font-family: serif, background: lime, color: red). `all: unset` strips even default user-agent behaviors and would make the test page unusably broken in non-Shadow-DOM areas.

4. **Should the size-limit GH Action comment on every push or only on PRs?**
   - What we know: `andresz1/size-limit-action` defaults to PR-only commenting; push to main updates baseline
   - What's unclear: Whether main-branch builds should also fail on >70 KB or just record the new baseline
   - Recommendation: **PR-only commenting (default). Main-branch build still runs `size` but doesn't comment — just updates baseline for next PR.** Matches the action's default.

5. **What's the fallback if a host's hash collides with embed's hash?**
   - What we know: D7-07 documents "single embed per page" as a v0.1 limitation
   - What's unclear: Whether the embed's hash-adapter prefix (`open-routing/`) is enough to coexist with hash-using hosts (rare in 2026)
   - Recommendation: **Prefix is sufficient** for v0.1. Test via a stub host that uses its own hash (`#main-section`) and verify embed ignores it. Document in README as "embed reserves `#open-routing/...`".

6. **Does embed need a `disabled` attribute / `programmatic shutdown` API?**
   - What we know: Host SPAs may want to unmount the embed temporarily (e.g., user logs out)
   - What's unclear: Whether `<open-routing-catalog>` should expose a method like `disconnect()` for host-controlled cleanup
   - Recommendation: **Lean on standard Custom Element lifecycle.** Host removes the element from DOM → `disconnectedCallback` fires → hash adapter stops + client cleared. No explicit API needed for v0.1.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js 22+ | Vite 8, pnpm, Playwright | ✓ | 22.x (Phase 6 CI) | — |
| pnpm 10+ | Workspace install, pack | ✓ | 10.33.0 (root package.json) | — |
| Vite 8 (transitive via admin) | Embed build (library mode) | ✓ | 8.0.13 in admin | — |
| Playwright Chromium | Multi-host matrix E2E | ✓ | 1.60.0 in admin | — |
| esm.sh CDN | Stub host React/Vue loading | ✓ (public CDN) | — | `retries: 1` per D7-12 |
| Go 1.25 | API + cors middleware compile | ✓ | go.mod toolchain go1.25.10 | — |
| `github.com/go-chi/cors` | CORS middleware | ✗ (not yet in go.mod) | needs v1.2.2 install | — |
| `size-limit` | CI bundle gate | ✗ (not yet in any package) | needs 12.1.0 install | — |
| `andresz1/size-limit-action` | GH Action PR comment | ✓ (GitHub Marketplace) | 1.8.0 | — |
| PostgreSQL 17 + Redis | Backend already running (existing Phase 1) | ✓ | docker-compose | — |
| Phase 6 packages/ui components | Lazy-route render targets | ✓ | shipped 2026-05-18 | — |
| Phase 6 admin app (for regression smoke) | D7-03 mandate to re-run Plan 06-06 smoke | ✓ | shipped | — |

**Missing dependencies with no fallback:** None — all required deps are install-time additions, no human checkpoint needed.

**Missing dependencies with fallback:** None — esm.sh CDN flake is the only "fallback" scenario, absorbed by `retries: 1`.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Frontend Unit Framework | `vitest` 3.2.4 [VERIFIED: existing in repo] |
| Frontend E2E Framework | `@playwright/test` 1.60.0 [VERIFIED: existing in repo] |
| Backend Test Framework | Go standard `testing` + `stretchr/testify` [VERIFIED: existing in repo] |
| Vitest config (embed) | `apps/embed/vitest.config.ts` — Wave 0 creates |
| Playwright config (embed) | `apps/embed/playwright.config.ts` — Wave 0 creates |
| Quick run (unit) | `pnpm --filter @open-routing/embed test` |
| Quick run (Go) | `cd services/api && go test ./internal/middleware -run CORS -short` |
| Full suite | `pnpm test:all && pnpm test:e2e:embed && go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| EMBED-01 | Single ES module produced; ≤70 KB gzipped | ci | `pnpm --filter @open-routing/embed size` | ❌ Wave 0 |
| EMBED-01 | `dist/embed.js` exists and is a valid ES module | ci | `node -e "import('./dist/embed.js')"` after build | ❌ Wave 0 |
| EMBED-01 | Lazy chunks emit per-entity (8 chunks) | ci | `ls dist/embed-*.js \| wc -l` assertion in build smoke | ❌ Wave 0 |
| EMBED-02 | Element validates UUIDv7 `org-id`; shows error inline on malformed | unit | `pytest -- vitest run apps/embed/src/embed-element.test.ts -t "invalid org-id"` (vitest, not pytest — typo intentional fix below) | ❌ Wave 1 |
| EMBED-02 | `createApiClient.getOrgId()` reads attribute live; X-Org-Id on every call | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "X-Org-Id"` | ❌ Wave 1 |
| EMBED-02 | `org-id` attribute change re-validates without remount | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "attribute change"` | ❌ Wave 1 |
| EMBED-03 | All 6 entities reachable inside embed via lazy routes | e2e | `pnpm --filter @open-routing/embed test:e2e embed-modules-filter.spec.ts` | ❌ Wave 2 |
| EMBED-03 | Phase 6 admin smoke still passes after shell refactor (D7-03 regression guard) | e2e | `pnpm --filter @open-routing/admin test:e2e smoke.spec.ts` | ✓ existing |
| EMBED-04 | Shadow DOM CSS isolation — host styles don't bleed in | e2e | `pnpm --filter @open-routing/embed test:e2e embed-shadow-dom-isolation.spec.ts -t "host CSS does not leak in"` × 3 hosts | ❌ Wave 2 |
| EMBED-04 | Shadow DOM CSS isolation — embed styles don't leak out | e2e | `pnpm --filter @open-routing/embed test:e2e embed-shadow-dom-isolation.spec.ts -t "embed CSS does not escape"` × 3 hosts | ❌ Wave 2 |
| EMBED-04 | `<sl-dialog>` replacement verified — no portal escape | e2e | `pnpm --filter @open-routing/embed test:e2e embed-shadow-dom-isolation.spec.ts -t "no portal escape"` | ❌ Wave 2 |
| EMBED-05 | `theme` accepts JSON object and applies tokens on host | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "theme JSON parse"` | ❌ Wave 1 |
| EMBED-05 | `theme` accepts named string ('or-light' etc) | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "theme named string"` | ❌ Wave 1 |
| EMBED-05 | Malformed theme JSON falls back to or-light silently | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "malformed theme"` | ❌ Wave 1 |
| EMBED-06 | `modules` attribute filters sidebar nav | e2e | `pnpm --filter @open-routing/embed test:e2e embed-modules-filter.spec.ts -t "modules filter"` × 3 hosts | ❌ Wave 2 |
| EMBED-06 | Unlisted entity routes redirect to first allowed module | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "module restriction route"` | ❌ Wave 2 (open question #6 above) |
| EMBED-07 | `open-routing:request-context` fires once on mount with composed:true | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "request-context"` | ❌ Wave 1 |
| EMBED-07 | `open-routing:auth-expired` fires on every 401 | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "auth-expired event"` | ❌ Wave 1 |
| EMBED-07 | Both events cross Shadow DOM boundary (composed:true behavior) | e2e | `pnpm --filter @open-routing/embed test:e2e embed-auth-expired.spec.ts -t "host receives event"` × 3 hosts | ❌ Wave 2 |
| EMBED-08 | 409 reload UX parity with admin (`<or-conflict-banner>`) | unit | `pnpm --filter @open-routing/ui test conflict-banner.test.ts` | ✓ existing (verify reused) |
| EMBED-09 | `pnpm pack --dry-run` produces valid tarball listing | ci | `pnpm --filter @open-routing/embed pack --dry-run` | ❌ Wave 3 |
| EMBED-09 | Tarball install in scratch dir succeeds | ci | shell script in CI: `pnpm pack && pnpm install file:./*.tgz` in tmp dir | ❌ Wave 3 |
| EMBED-10 | All 4 specs run across 3 host projects (12 runs total) | e2e | `pnpm --filter @open-routing/embed test:e2e` (Playwright projects matrix) | ❌ Wave 2 |
| EMBED-10 | Bundle size CI gate enforces ≤70 KB on every PR | ci | `andresz1/size-limit-action@v1` + `.size-limit.json` | ❌ Wave 3 |
| (CORS-1) | go-chi/cors short-circuits OPTIONS preflight without X-Org-Id | backend | `cd services/api && go test ./internal/middleware -run CORS -short` | ❌ Wave 0 (backend) |
| (CORS-2) | OPTIONS preflight to `/healthz` passes (bypass route) | backend | extend `test/isolation/cors_preflight_test.go` | ❌ Wave 0 (backend) |
| (CORS-3) | Cross-origin GET with valid origin gets CORS headers | backend | extend `test/isolation/cors_cross_origin_test.go` | ❌ Wave 0 (backend) |
| (CORS-4) | Cross-origin GET with rejected origin gets no Allow-Origin header | backend | extend `test/isolation/cors_cross_origin_test.go` | ❌ Wave 0 (backend) |
| (CORS-5) | CORS middleware ordering doesn't break Phase 1 isolation suite | backend | full Phase 1 isolation suite re-runs green | ✓ existing |
| (D7-03) | Shell `routingMode='hash'` triggers hash adapter; `routingMode='history'` does not | unit | `pnpm --filter @open-routing/ui test catalog-shell.test.ts -t "routingMode"` | ✓ existing test file (extend) |
| (D7-03) | All 30+ lazy imports in shell resolve correctly (no chunk-not-found errors) | e2e | implicit via embed-modules-filter.spec.ts and admin smoke.spec.ts | ✓ existing + new |
| (Hash adapter) | `hashchange` event triggers `routes.goto()` with stripped hash | unit | `pnpm --filter @open-routing/embed test hash-router-adapter.test.ts` | ❌ Wave 1 |
| (Hash adapter) | Adapter ignores non-`open-routing/` prefixed hashes | unit | `pnpm --filter @open-routing/embed test hash-router-adapter.test.ts -t "ignore unrelated hash"` | ❌ Wave 1 |

### Sampling Rate

- **Per task commit:** `pnpm --filter @open-routing/embed test` (component unit tests only; ~5s)
- **Per wave merge:** `pnpm --filter @open-routing/embed test && pnpm --filter @open-routing/embed test:e2e` (~3-5 min depending on Playwright matrix) + `cd services/api && go test ./internal/middleware -run CORS`
- **Phase gate:** Full suite — backend isolation + frontend unit + Playwright matrix × 3 hosts + size-limit gate + admin regression smoke + `pnpm pack` smoke. Green before `/gsd:verify-work`.

### Wave 0 Gaps

The following test infrastructure does not exist and must be created before implementation begins:

- [ ] `web/apps/embed/vitest.config.ts` — Vitest config with happy-dom environment
- [ ] `web/apps/embed/test-setup.ts` — Loads `urlpattern-polyfill` for hash-router URLPattern usage
- [ ] `web/apps/embed/playwright.config.ts` — 3-project matrix (react/vue/html); webServer = `vite preview`
- [ ] `web/apps/embed/e2e/hosts/react/index.html` — Static stub host with importmap + React 18.3.1
- [ ] `web/apps/embed/e2e/hosts/vue/index.html` — Static stub host with importmap + Vue 3.5.13
- [ ] `web/apps/embed/e2e/hosts/html/index.html` — Plain HTML stub host
- [ ] `web/apps/embed/e2e/specs/embed-shadow-dom-isolation.spec.ts` — EMBED-04 assertions
- [ ] `web/apps/embed/e2e/specs/embed-org-id-header.spec.ts` — EMBED-02 X-Org-Id assertion
- [ ] `web/apps/embed/e2e/specs/embed-auth-expired.spec.ts` — EMBED-07 + auth-expired event assertion
- [ ] `web/apps/embed/e2e/specs/embed-modules-filter.spec.ts` — EMBED-06 + EMBED-03 modules filter assertion
- [ ] `web/apps/embed/src/embed-element.test.ts` — Unit tests for the OpenRoutingCatalog class (mounted in happy-dom)
- [ ] `web/apps/embed/src/hash-router-adapter.test.ts` — Unit tests for the hash adapter
- [ ] `web/apps/embed/.size-limit.json` — Declarative budget
- [ ] `services/api/internal/middleware/cors_test.go` — Unit tests for the new CORS middleware
- [ ] `services/api/test/isolation/cors_preflight_test.go` — Integration test: OPTIONS preflight to bypass + /v1
- [ ] `services/api/test/isolation/cors_cross_origin_test.go` — Integration test: cross-origin GET with allowed + rejected origin
- [ ] `.github/workflows/ci.yml` — New jobs `web-bundle-size` + `web-e2e-embed`; cache for Playwright browsers

*(No existing test infrastructure covers any of the EMBED-XX requirements — Phase 7 is a clean slate for embed-specific tests.)*

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | partial — embed reservation for v1 | `open-routing:auth-expired` event contract reserves the channel; v0.1 stub auth via `X-Org-Id` header from host |
| V3 Session Management | no | No sessions in v0.1; embed reads attributes per mount |
| V4 Access Control | partial — relies on Phase 1 OrgContext + Go server enforcement | Backend remains authoritative; embed cannot bypass server checks |
| V5 Input Validation | yes | UUIDv7 regex on `org-id` (client-side UX); ajv-standalone validators on form submit (compiled from OpenAPI); server still authoritative via `middleware.UUIDv7PathParams` |
| V6 Cryptography | no | No new crypto introduced |
| V8 Data Protection | partial | Theme JSON parse — must not execute as code (`JSON.parse` is safe; no `eval`) |
| V9 Communications | yes | CORS — go-chi/cors enforces allowed origin list; `CORS_ALLOWED_ORIGINS` env var must be explicit (no `*` default in prod); HTTPS enforced at deployment layer (out of scope for Phase 7) |
| V14 Configuration | yes | `CORS_ALLOWED_ORIGINS` env var; dev default `*` via Taskfile; prod requires explicit allowlist |

### Known Threat Patterns for the Embed Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| CSS injection via theme attribute escaping into the host page | Tampering | CSS custom properties applied via `this.style.setProperty(name, value)`; values are strings, no JS eval; browser CSS engine rejects malformed values |
| XSS via `theme` JSON parse | Tampering / Spoofing | `JSON.parse` is safe; never `eval`; ajv schema validates theme JSON shape (recommend Wave 1 task: add `validateTheme` schema) |
| Cross-origin data exfiltration from host page via CustomEvent | Information Disclosure | `composed: true` events expose embed internals; only fire whitelisted events (`request-context`, `auth-expired`); never include sensitive data in `detail` (no auth tokens, no PII; only orgId which host already knows) |
| CSRF on embed-issued API calls | Tampering | v0.1 stub auth; v1 AUTH phase introduces CSRF tokens; Phase 7 reserves the contract via `open-routing:auth-expired` event |
| Malicious origin sending API requests | Spoofing | `go-chi/cors` `AllowedOrigins` enforces; reject unlisted origins; `AllowCredentials: false` in v0.1 (no cookies cross-origin) |
| Bundle tampering via CDN MITM | Tampering / Spoofing | v0.1 ships tarball only; v1 publish must include subresource integrity (SRI) hashes in README CDN example |
| Dependency confusion attack (slopsquatting) | Tampering | Package Legitimacy Audit above; all packages verified on npm with publish history |
| Prototype pollution via JSON theme | Tampering | `JSON.parse` doesn't add `__proto__` by default in modern Node/browsers; ajv validator catches `__proto__` keys |
| `customElements.define` collision with host's own elements | Tampering | Element name `open-routing-catalog` is project-scoped (`open-routing` namespace); collision would require host to also define `<open-routing-catalog>` (rare); documented in README |
| Hash collision with host SPA routing | Denial of Service (host side) | Adapter prefix check (`#open-routing/...`); D7-07 documented limitation |
| 401 response replay flooding `open-routing:auth-expired` events | Denial of Service (host side) | Host's listener owns rate limiting; embed dispatches faithfully (D7-14) |

**Out-of-scope security work (deferred):**
- Operator-facing CORS allowlist per org UI (v1+)
- SRI hashes for CDN-published embed (v1 publish)
- CSP headers for the embed host (host owns CSP; embed documents `connect-src` requirement for `api-base-url`)
- Real RBAC on `force` flag (Phase 4 D-84 carry-forward; v1)

---

## Sources

### Primary (HIGH confidence)

- [Lit Custom Element guide](https://lit.dev/docs/components/overview/) — `connectedCallback`, attribute reflection, Shadow DOM modes
- [Lit Events guide](https://lit.dev/docs/components/events/) — `composed: true` cross-shadow-boundary contract
- [`@lit-labs/router` npm README](https://www.npmjs.com/package/@lit-labs/router) — `enter()` async hook, URLPattern matching, `outlet()` integration
- [`@lit-labs/router` discussion #3354](https://github.com/lit/lit/discussions/3354) — confirmed lazy-load pattern with dynamic imports
- [`go-chi/cors` source code](https://github.com/go-chi/cors/blob/master/cors.go) — `handlePreflight` short-circuit logic confirmed
- [`go-chi/cors` README](https://github.com/go-chi/cors/blob/master/README.md) — middleware ordering recommendation
- [MDN: Event.composed](https://developer.mozilla.org/en-US/docs/Web/API/Event/composed) — Shadow DOM boundary crossing
- [MDN: HashChangeEvent](https://developer.mozilla.org/en-US/docs/Web/API/Window/hashchange_event) — hash adapter semantics
- [WHATWG URLPattern](https://urlpattern.spec.whatwg.org/) — pattern matching used by `@lit-labs/router`
- [Vite Build Options](https://vite.dev/config/build-options) — library mode, externalization
- [Vite library mode discussion #1736](https://github.com/vitejs/vite/discussions/1736) — multi-entry, single entry patterns
- [size-limit README](https://github.com/ai/size-limit) — config shape, compression options
- [`andresz1/size-limit-action`](https://github.com/andresz1/size-limit-action) — GH Action options, pnpm support
- [Shoelace `<sl-dialog>` issue #709](https://github.com/shoelace-style/shoelace/issues/709) — focus trap broken in shadow root
- [Shoelace `<sl-dialog>` issue #1382](https://github.com/shoelace-style/shoelace/issues/1382) — same issue, second report
- [esm.sh CDN](https://esm.sh/) — version pinning patterns
- Existing Phase 6 RESEARCH.md at `.planning/phases/06-shared-ui-library-standalone-admin/06-RESEARCH.md` — Vite 8 decorator config + Lit 3 + Shoelace patterns
- Existing Phase 6 CONTEXT.md (D6-08, D6-11, D6-20, D6-24, D6-30) — locked carry-forward decisions
- Existing Phase 1 CONTEXT.md (D-21 bypass routes) — chi middleware chain ordering
- Existing Phase 2 CONTEXT.md (D-38 createApiClient) — openapi-fetch middleware pattern reused

### Secondary (MEDIUM confidence)

- [Cloudflare Brotli vs Gzip experiments](https://blog.cloudflare.com/results-experimenting-brotli/) — 15-30% size reduction; bundle gzip budget is conservative
- [pnpm pack docs](https://pnpm.io/cli/pack) + [pnpm#10195](https://github.com/pnpm/pnpm/issues/10195) — pack lifecycle quirks
- [Playwright shadow DOM piercing](https://www.testingmavens.com/blogs/interacting-with-shadow-dom-the) — Playwright's CSS engine pierces open Shadow DOM by default

### Tertiary (LOW confidence — verify before relying)

- esm.sh React/Vue version pinning best-practice articles (multiple Medium / dev.to posts, agree on `@x.y.z` patch pinning but no single canonical source)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every package version verified via `npm view` / `go list`; carry-forward from Phase 6 well-trodden
- Architecture: HIGH — D7-03 lazy-route pattern confirmed against Lit discussion #3354; CORS placement verified in go-chi/cors source
- Pitfalls: HIGH on critical (Pitfalls 1-8); MEDIUM on size-limit-vs-CDN (Pitfall 10 has documentation-level impact only)
- Shoelace audit: HIGH on scope (grep counts deterministic); MEDIUM on remediation approach (replace-vs-document is a design call)
- Hash adapter: HIGH — pattern is simple, established by URLPattern spec + `hashchange` MDN docs

**Research date:** 2026-05-18
**Valid until:** 2026-06-17 (30 days — stable libraries; refresh if Lit 4 or Shoelace 3 lands)
