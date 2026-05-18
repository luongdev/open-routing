# Phase 7: Web Component Embed Bundle - Context

**Gathered:** 2026-05-18
**Status:** Ready for planning

> **Decision-ID convention:** Phase-local prefix `D7-NN`. Continues the
> per-phase prefix convention used by Phase 04.1 (`D04_1-NN`), Phase 5
> (`D5-NN`), and Phase 6 (`D6-NN`). Backend Phase 1-4 used the continuous
> `D-01..D-95` sequence; Phase 7 keeps its UI/embed decisions in a
> dedicated namespace so cross-phase reads stay disambiguated.

<domain>
## Phase Boundary

Ship `web/apps/embed/` — a single ES module bundle that registers
`<open-routing-catalog>` as a Custom Element. Host apps in any framework
(React 18, Vue 3, plain HTML) mount the tag, pass `org-id`, `api-base-url`,
`theme`, `modules` attributes, and get the full Phase 6 catalog admin UX
isolated inside a Shadow DOM root. Bundle gzipped baseline ≤70 KB,
enforced in CI. Playwright matrix verifies Shadow DOM CSS isolation in
both directions, `X-Org-Id` propagation, `open-routing:auth-expired` 401
dispatch, and bundle-size budget across all three host frameworks.

The embed is a hybrid wrapper around `<or-catalog-shell>` from
`packages/ui` (Phase 6 D6-05). Phase 7's central refactor is
**converting the shell's static entity imports to dynamic `import()`
per route**, splitting the eager baseline (shell + router + theme +
primitives + auth-banner) from per-entity lazy chunks. Same shell powers
admin (history routing) and embed (hash routing) via a new routing-mode
prop on the shell; embed wrapper provides a thin hash-adapter on top of
`@lit-labs/router`.

**In scope:**
- **`web/apps/embed/` build pipeline:**
  - `package.json` with `dev`, `build`, `preview`, `test`, `test:e2e`,
    `size` scripts. Deps: `lit`, `@open-routing/ui` (workspace), no
    Shoelace (re-exported through ui).
  - `vite.config.ts` set up as a **library build** (`build.lib`) emitting
    a single ES module `dist/embed.js` + sourcemaps + per-entity lazy
    chunks. Same `@rolldown/plugin-babel` decorators config as
    `apps/admin` (D-78-style). `build.target: 'esnext'`.
    Excludes Vite dev-proxy (host serves API).
  - Entry `web/apps/embed/src/index.ts` registers the `<open-routing-catalog>`
    Custom Element + locale bootstrap (mirrors admin's
    `configureLocalization` shape but reads locale from element
    attribute instead of localStorage).
  - `web/apps/embed/src/embed-element.ts` — the Custom Element class
    (`OpenRoutingCatalog extends LitElement`). Owns attribute parsing,
    Shadow DOM construction, `<or-catalog-shell>` mount with
    `routing="hash"`, `open-routing:request-context` / `:auth-expired`
    event dispatch, error state for malformed `org-id`.
  - `web/apps/embed/src/hash-router-adapter.ts` — hash-mode wrapper
    around `@lit-labs/router` (~30 lines: parse `window.location.hash`,
    listen for `hashchange`, expose the same `goto()` / outlet API the
    shell uses).
- **`packages/ui` refactor (shared shell + lazy routes):**
  - Convert `web/packages/ui/src/components/shell/catalog-shell.ts`
    entity imports from static (`import '../agents/agent-list.js'`) to
    dynamic — wrapped in a small `lazyRoute(() => import('../agents/agent-list.js'))`
    helper that ties into `@lit-labs/router`'s `enter` hook to
    `await import()` then `render` the component.
  - Shell gains a `routingMode: 'history' | 'hash'` reactive property.
    Default `'history'` (admin's current behavior unchanged). Embed sets
    `'hash'`. The hash-router-adapter is imported only when
    `routingMode === 'hash'`.
  - Eager bundle = shell + router + chrome + theme tokens + primitives
    (`or-data-table`, `or-cursor-paginator`, `or-code-input`,
    `or-conflict-banner`, `or-form-wizard`) + auth-expired banner.
  - Lazy chunks = 6 entity bundles (agents / skills / queues / channels /
    adapters / break-reasons) + status-panel + import-page. Vite emits
    `embed-agents-{hash}.js` etc. via natural code-splitting on dynamic
    imports.
  - Existing admin tests get a regression pass — lazy loading must not
    break Plan 06-06 Playwright smoke; admin's first-paint may improve.
- **Embed-specific Shadow DOM behaviour:**
  - **Inline alerts only** — no `<sl-dialog>` / `<sl-alert>` toast usage
    inside embedded code paths (those portal to `document.body` and
    escape Shadow DOM). `<or-conflict-banner>` already inline (D6-04);
    any remaining Shoelace dialog/toast usage in shell/components is
    swept into inline Lit alerts inside their Shadow DOMs.
  - **Auth-expired UI state**: when `open-routing:auth-expired` fires,
    embed renders an inline banner at the shell top with the
    `request_id` (D6-24 parity) — embed does NOT pause API requests,
    host owns recovery. Banner dismisses on user click.
  - **Malformed/missing `org-id` attribute**: embed renders error state
    *inside* Shadow DOM (`<div>Invalid org-id attribute</div>` — no
    org-picker fallback). EMBED-02 mandates org-id from attribute, so
    embed never renders `<or-org-picker>` from D6-10 shell layout.
- **Public API surface (HTML attributes on `<open-routing-catalog>`):**
  - `org-id` (required, UUIDv7 — same regex as D6-13)
  - `api-base-url` (required; absolute URL accepted; relative
    accepted with documented "host reverse-proxy" caveat)
  - `theme` (optional; JSON object string of CSS custom-property tokens
    OR named theme `or-light` / `or-dark` / `or-brand`; mirrors D6-20
    shell behavior)
  - `modules` (optional; comma-separated entity keys; same filter as
    D6-11)
  - `locale` (optional; `en` | `vi`; D6-21 parity)
- **Public events** (`composed: true`, bubbling — cross Shadow DOM):
  - `open-routing:request-context` fired ONCE on `connectedCallback`,
    `detail = { orgId, apiBaseUrl, theme, modules }`. Announce-only
    contract for v0.1; reserves the channel for v1 auth handshake.
  - `open-routing:auth-expired` fired on EVERY 401 response (not just
    first), `detail = { statusCode: 401, requestId, path }`. Embed
    continues fetching; host decides recovery.
- **Go API CORS middleware** (`services/api/internal/middleware/cors.go`):
  - `github.com/go-chi/cors` pinned in `go.mod`.
  - Allowed origins from `CORS_ALLOWED_ORIGINS` env var (comma-separated;
    `*` in dev, explicit list in prod).
  - Allowed headers: `X-Org-Id`, `Content-Type`, `Idempotency-Key`,
    `Accept-Language`.
  - Allowed methods: `GET, POST, PATCH, DELETE, OPTIONS`.
  - Inserted into chi chain BEFORE `OrgContext` so preflight `OPTIONS`
    requests don't require `X-Org-Id`.
  - Bypass routes (`/healthz`, `/readyz`, `/metrics`, `/openapi.yaml`,
    `/docs` — Phase 1 D-21) covered identically.
  - Existing Phase 1 isolation test suite extended with one OPTIONS
    preflight assertion + one cross-origin GET assertion.
- **Playwright stub-host matrix** (`web/apps/embed/e2e/`):
  - `e2e/hosts/react/index.html` — static HTML; `<script type="importmap">`
    pinning `react@18`, `react-dom@18` from `esm.sh`; `<script type="module"
    src="/dist/embed.js">`; minimal React component embedding
    `<open-routing-catalog>` via ref.
  - `e2e/hosts/vue/index.html` — Vue 3 via esm.sh; same pattern.
  - `e2e/hosts/html/index.html` — plain HTML; embed mounted directly,
    optionally inside a `<div>` with aggressive CSS reset.
  - `playwright.config.ts` reuses Phase 6 D6-30 structure; adds 3
    projects (`react`, `vue`, `html`) all hitting `http://localhost:4173`
    (Vite preview serving the embed dist + host static files).
    `retries: 1` (CDN-flake tolerance).
  - Specs at `e2e/specs/embed-shadow-dom-isolation.spec.ts`,
    `embed-org-id-header.spec.ts`, `embed-auth-expired.spec.ts`,
    `embed-modules-filter.spec.ts`. Each spec runs against all 3
    `projects`.
- **CI updates (`.github/workflows/ci.yml`):**
  - **New job `web-bundle-size`**: builds `apps/embed`, runs
    `pnpm --filter @open-routing/embed size` (size-limit). GH Action
    comment on PR shows delta vs `main`. Hard fail on >70 KB gzipped
    eager + warning on lazy-chunk regression. Required-status-check
    (D-30 carry-forward).
  - **New job `web-e2e-embed`**: builds embed, runs Vite preview, runs
    Playwright matrix (3 hosts × 4 specs = 12 test runs). 1 retry/spec.
    Required-status-check.
  - **Extend existing `web-typecheck` + `web-lint`** with `apps/embed`
    (already covered by `pnpm -F '*'` patterns).
  - **Extend `codegen-drift`**: no new gen step; embed consumes
    `packages/ui` generated types transitively.
  - GH Actions browser cache: cache `~/.cache/ms-playwright` keyed on
    playwright version (saves 90s/run).
- **Distribution prep (NOT publishing in v0.1):**
  - `apps/embed/package.json` set up with proper `name`, `version`,
    `main`, `exports`, `files`, `sideEffects` so `pnpm pack` produces a
    valid npm tarball. Verified in CI via `pnpm pack --dry-run`.
  - `apps/embed/README.md` covers: install via tarball, attribute API,
    event API, theme JSON shape, CORS setup, single-embed-per-page
    limitation, esm.sh CDN usage example.
  - `LICENSE`, `CHANGELOG.md` initial entry.
  - No `pnpm publish` step in CI. Real publishing deferred to v1
    alongside real auth + first customer integration.

**Out of scope:**
- **Actual npm/CDN publishing** — Phase 7 ships a tarball-ready package;
  uploading to a registry is v1 work alongside real customers.
- **Iframe embed variant + Module Federation variant** — PROJECT.md
  out-of-scope; v1+ only if demand justifies.
- **Multi-instance per page** — `<open-routing-catalog>` mounted twice
  on the same page is documented as unsupported in v0.1 (shared hash
  state, last write wins). v0.2 can add `hash-prefix` attribute if
  customer demand emerges.
- **history-mode routing for embed** — embed locked to hash; history
  needs host's `base-path` cooperation and isn't worth the integration
  burden for v0.1.
- **Production CORS allowlist UX/config** — Phase 7 ships the
  middleware + env-var allowlist; an operator-facing UI for managing
  allowed origins per org is v1 / runtime-milestone.
- **Server-side rendering of embed** — Custom Elements are
  client-rendered only. Hosts wanting SSR shells render a placeholder
  and hydrate on `connectedCallback`.
- **Theme builder UI / per-org brand theme** — Phase 6 D6-19 deferral
  carries forward. Embed accepts arbitrary JSON theme; no in-product
  authoring tool.
- **Locale-detection from host's `Accept-Language` / navigator** — embed
  reads `locale` attribute only (host-controlled). Admin's
  `navigator.language` detection (D6-23) does not apply.
- **Embed-side persistent state (localStorage / sessionStorage)** —
  embed reads attributes per mount; no localStorage usage. Host owns
  persistence; saves cross-origin embed-isolation concerns.
- **Real auth-expired recovery flow** — embed dispatches the event +
  shows inline banner; host owns sign-in/refresh-token logic. v1 AUTH
  phase wires real recovery.

</domain>

<decisions>
## Implementation Decisions

### Reuse Strategy (Area 1)

- **D7-01:** **Hybrid: mount `<or-catalog-shell>` + lazy-load entity
  routes via dynamic `import()`.** Baseline eager = shell + router +
  chrome + theme tokens + primitives + auth-banner. Lazy = 6 entity
  bundles + status-panel + import-page. Phase 6 D6-05 left this open;
  hybrid is the only path that hits 70 KB gzipped baseline while keeping
  shell single-source-of-truth across admin + embed. Rejected:
  mount-shell-as-is (risks bundle balloon on new entity), compose-entities
  (drift from admin UX shape, 200+ lines new router code).
- **D7-02:** **Eager baseline = chrome a user always sees on first
  paint.** Concretely: shell + router-adapter + theme machinery + 5
  primitives (`or-data-table`, `or-cursor-paginator`, `or-code-input`,
  `or-conflict-banner`, `or-form-wizard`) + auth-expired banner. Each
  entity becomes a separate Vite chunk via natural dynamic-import code
  splitting. Soft per-chunk budget ~15 KB gzipped (logged, not blocked).
- **D7-03:** **Refactor `catalog-shell.ts` IN PLACE — same shell for
  admin + embed.** Convert ~30 static entity imports to a `lazyRoute()`
  helper that lazily imports the entity module inside the router's
  `enter` hook. Admin gets lazy loading as a side effect (small UX win).
  No `catalog-shell-lazy.ts` variant; no duplicate test suites. Plan must
  re-run Plan 06-06 Playwright smoke to ensure admin first-paint still
  navigates correctly.
- **D7-04:** **Inline alerts only — no Shoelace `<sl-dialog>` /
  `<sl-alert>` toast usage anywhere embed reaches.** Shoelace portals
  attach to `document.body` and escape Shadow DOM (EMBED-04 violation
  risk). `<or-conflict-banner>` is already inline (D6-04); any remaining
  Shoelace dialog/toast usage in `packages/ui` components is audited
  and replaced with inline Lit alerts. Slightly less polish than
  slide-out toasts; matches admin's existing inline-banner UX.

### Shadow DOM Routing (Area 2)

- **D7-05:** **Hash routing inside embed** — `window.location.hash`
  carries embed nav state (`#open-routing/orgs/{id}/agents`). Host
  `pathname` untouched; host SPAs that own `window.history` don't
  collide. Bookmarkable inside embed; browser back/forward works.
  Conflicts only if host also uses hash routing (rare in 2026 — React
  Router 6 + Vue Router 4 default to history mode).
- **D7-06:** **Embed-only hash mode; admin stays on history.** Shell
  gains a `routingMode: 'history' | 'hash'` reactive property (default
  `'history'`). Embed's `<open-routing-catalog>` sets `routingMode="hash"`
  on the shell instance it mounts. Hash adapter (~30 lines) lives at
  `web/apps/embed/src/hash-router-adapter.ts` — wraps `@lit-labs/router`
  to parse `location.hash` instead of `location.pathname` and listen
  for `hashchange` events. Admin URLs remain pretty (`/orgs/{id}/agents`).
- **D7-07:** **Document "single embed per page" v0.1 limitation.**
  `window.location.hash` is global — two embed instances on the same
  page would fight for it (last write wins). Documented in
  `apps/embed/README.md` + JSDoc on the Custom Element class. Realistic
  v0.1 scope (no compelling 2-embed use case). v0.2 can add per-instance
  `hash-prefix="foo"` attribute → `#foo/orgs/...` if customer demand
  emerges.
- **D7-08:** **Malformed/missing `org-id` attribute → error state
  inside Shadow DOM.** Embed validates `org-id` against UUIDv7 regex
  (same as D6-13) at `connectedCallback`. Invalid: render
  `<div class="error">Invalid org-id attribute</div>` inside Shadow DOM
  with brief help text; do NOT fall back to D6-10 `<or-org-picker>`
  (EMBED-02 mandates attribute-only org binding). Missing: same error
  text "org-id attribute required". Attribute-change observer re-runs
  validation; live correction recovers without remount.

### CI Infrastructure (Area 3)

- **D7-09:** **size-limit declarative budget + GitHub Action PR
  comment.** `apps/embed` devDeps add `size-limit` + `@size-limit/preset-app`.
  Budget in `apps/embed/.size-limit.json`:
  `[{ "name": "embed.js (eager baseline)", "path": "dist/embed.js",
  "limit": "70 KB", "gzip": true }]`. CI job `web-bundle-size` runs
  `pnpm --filter @open-routing/embed size`. `andresz1/size-limit-action`
  posts PR comment with delta vs `main`. Hard fail above 70 KB. Trusted
  ecosystem (date-fns, redux, Preact use it).
- **D7-10:** **Eager baseline ≤70 KB; lazy chunks tracked with soft
  budgets.** EMBED-01 "single ES module bundle" interpreted as the
  eager entry that registers the Custom Element + renders shell shell.
  Per-entity lazy chunks each get a ~15 KB gzipped soft budget — size-limit
  logs delta but does NOT fail CI. Allows entity chunks to grow within
  reason as features mature; prevents one bloated entity doubling
  perceived load without silent regression.
- **D7-11:** **Playwright stub hosts at `apps/embed/e2e/hosts/{react,vue,html}/`
  — static HTML + esm.sh CDN; bundle served via Vite preview.** Each
  host is a static `index.html` with `<script type="importmap">` pinning
  React/Vue versions from `esm.sh`. No React/Vue devDeps in monorepo;
  no separate apps/embed-e2e-react packages. `pnpm --filter
  @open-routing/embed test:e2e` runs `vite preview` (serves
  `apps/embed/dist/` + the host HTML files) then Playwright matrix.
  Lightweight; mirrors real-world host integration (CDN-loaded
  framework + script-tag embed).
- **D7-12:** **Playwright failure budget: `retries: 1` per spec; same
  EMBED-10 assertions across all 3 hosts; flaky failure blocks merge.**
  All 3 hosts (React 18, Vue 3, plain HTML) run identical assertions:
  (a) Shadow DOM CSS isolation in both directions — host CSS reset
  doesn't bleed into embed, embed's `:host` styles don't escape; (b)
  `X-Org-Id` header set from `org-id` attribute on every API call
  (intercept via Playwright `page.route`); (c) `open-routing:auth-expired`
  fires on host-page listener when API returns 401 (intercept + mock
  401 response); (d) gzipped bundle ≤70 KB (covered by
  `web-bundle-size` in earlier CI step). `retries: 1` absorbs esm.sh
  CDN flakes. New required-status-check `web-e2e-embed`.

### Embed Public Contract (Area 4)

- **D7-13:** **`open-routing:request-context` is announce-only.** Fires
  once on `connectedCallback` after attributes are validated. `detail =
  { orgId, apiBaseUrl, theme, modules }`. `composed: true`, `bubbles:
  true`. No expected response; host listens for diagnostics, logging,
  or to wire its own context. v1 auth phase can add response-via-custom-event
  pattern without breaking the announce-only contract. Rejected:
  negotiable / `preventDefault`-blocks-boot (premature; v0.1 stub auth
  doesn't need it).
- **D7-14:** **`open-routing:auth-expired` fires on EVERY 401.**
  `detail = { statusCode: 401, requestId, path }`. `composed: true`,
  `bubbles: true`. Embed continues fetching subsequent requests; UI
  shows inline banner `Session expired — request {requestId}` at shell
  top (dismissible). Host owns recovery (refresh token, redirect to
  sign-in, etc.). Rate-limiting noisy event handlers is host's concern.
  v0.1 stub auth never triggers this in practice; contract reserved for
  v1.
- **D7-15:** **Build artifact + npm-ready `package.json`; DO NOT publish
  in v0.1.** Phase 7 produces `apps/embed/dist/embed.js` + a complete
  `apps/embed/package.json` (`name: @open-routing/catalog-embed`, `version`,
  `main`, `exports`, `files`, `sideEffects: false`). CI verifies `pnpm
  pack --dry-run` produces a valid tarball. No `pnpm publish` step.
  Documentation gives early integrators `pnpm pack` + tarball install
  recipe. Real npm publishing deferred to v1 alongside real auth + first
  customer.
- **D7-16:** **Go API ships CORS middleware (go-chi/cors) with env-var
  allowlist.** New file `services/api/internal/middleware/cors.go`.
  `CORS_ALLOWED_ORIGINS` env var (comma-separated; `*` default in dev
  via Taskfile, empty in prod requires explicit list). Allowed headers:
  `X-Org-Id, Content-Type, Idempotency-Key, Accept-Language`. Allowed
  methods: `GET, POST, PATCH, DELETE, OPTIONS`. CORS middleware inserted
  BEFORE `OrgContext` in chi chain so preflight `OPTIONS` (without
  `X-Org-Id`) don't return 400. Bypass routes (Phase 1 D-21) covered.
  Phase 1 isolation suite extended with one OPTIONS preflight + one
  cross-origin GET assertion.

### Claude's Discretion
- Exact internal layout of `apps/embed/src/` (e.g., `embed-element.ts`
  vs `index.ts` boundary, where the locale bootstrap lives) — planner
  picks; recommend `index.ts` registers the element + locale bootstrap,
  `embed-element.ts` is the LitElement class.
- size-limit GH Action choice (`andresz1/size-limit-action` vs
  `pajaydev/size-limit-bundle-action`) — planner picks; recommend
  `andresz1/` (more downloads, more recent commits).
- esm.sh version pinning strategy in stub hosts (exact patch `18.3.1` vs
  major `18`) — planner picks; recommend pinning to a specific minor
  for reproducibility but staying within a major.
- Hash-adapter implementation detail: `URLPattern` parse on `hashchange`
  vs hand-rolled regex — planner picks; recommend `URLPattern` so the
  shell's existing route definitions work without dual notation.
- Auth-expired inline banner copy + dismiss UX — planner picks; align
  with D6-24 ErrorCode-to-i18n-key pattern (`errors.auth_expired`).
- Whether `<or-conflict-banner>` and inline auth-banner share a base
  component or stay independent — planner picks; recommend independent
  for v0.1 (different copy, different actions; deduplicate in v0.2 if
  more banner types emerge).
- README example payload for `theme=` attribute (which token keys to
  showcase) — planner picks 5-10 most-impactful keys (primary color,
  background, text, border-radius, font family).
- Whether the embed's Custom Element shadow DOM is `open` or `closed`
  mode — recommend `open` (host can `querySelector` into shadowRoot for
  debugging); closed sometimes breaks Playwright assertions.
- Exact Playwright spec structure for cross-host matrix — planner picks
  between (a) one spec file per assertion type with `projects` matrix,
  or (b) one spec per host with all 4 assertions inside.
- Source-map handling in production build — planner picks; recommend
  emitting external `.map` files (separate from `embed.js`), document
  that ops can choose not to deploy them.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level locks
- `.planning/PROJECT.md` — Key Decisions table (locked stack: Vite +
  Lit + Shoelace + Web Components + Shadow DOM; REST polling only;
  browser workers deferred). "Performance is part of the product value"
  applies (embed first-paint must feel snappy).
- `.planning/REQUIREMENTS.md` §Web Component Embed Bundle — **EMBED-01
  through EMBED-10**. Acceptance criteria: single ES module ≤70 KB
  gzipped (EMBED-01), org-id from attribute (EMBED-02), 6-entity CRUD
  reuse (EMBED-03), Shadow DOM CSS isolation both directions (EMBED-04),
  theme=JSON tokens (EMBED-05), modules= filter (EMBED-06), composed:true
  events (EMBED-07), 409 reload UX (EMBED-08), npm/CDN publishing
  contract (EMBED-09 — Phase 7 ships build artifact, defers publish),
  Playwright matrix across React/Vue/HTML (EMBED-10).
- `.planning/ROADMAP.md` §Phase 7 — Goal statement + 5 success criteria.
- `.planning/STATE.md` §Blockers/Concerns — "Phase 7 host integration
  test harness must be set up at Phase 7 start" (D7-11 fulfils).

### Phase 6 carry-forward (LOCKED — shell + packages/ui)
- `.planning/phases/06-shared-ui-library-standalone-admin/06-CONTEXT.md`
  — D6-01..D6-31. Especially:
  - **D6-05** hybrid layer model — Phase 7 picks shell-mount-with-lazy
    (per D7-01).
  - **D6-08** per-component Shoelace imports — precondition for D7-09
    70 KB budget.
  - **D6-11** sidebar honours `modules="..."` filter — already
    implements EMBED-06.
  - **D6-13** UUIDv7 validation on `:org_id` — shared with D7-08
    attribute validation.
  - **D6-20** theme tokens applied on shell HOST element via
    `this.style.setProperty(...)` — EMBED-05 ready. Theme JSON parsed
    in the embed element and forwarded to shell.
  - **D6-24** ErrorCode → i18n key mapping — auth-expired banner copy
    aligns.
  - **D6-26** status panel `visibilitychange` pause — works inside
    Shadow DOM (events propagate normally).
  - **D6-30** Playwright harness — Phase 7 reuses Phase 6
    `web/apps/admin/e2e/` config shape + extends to multi-host matrix.
  - **D6-31** CI E2E deferred to Phase 7 — D7-11 / D7-12 deliver.

### Phase 2 UI sketches (LOCKED — visual contract)
- `.planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md` —
  Sketches 1-4 (agent detail, bulk import, status panel, generic
  list). All four screens reachable inside embed once entity chunks
  lazy-load. Same Lit components as admin; no embed-specific UI tweaks
  in v0.1.

### Phase 1 carry-forward (LOCKED — server-side infra)
- `.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md` —
  D-01..D-31. Especially:
  - **D-21** middleware bypass list (`/healthz`, `/readyz`, `/metrics`,
    `/openapi.yaml`, `/docs`) — CORS middleware must apply identically
    to bypass routes (D7-16).
  - **D-19, D-20** UUIDv7 regex — D7-08 reuses.
  - **D-30 LOCKED** required-status-check enforcement — D7-09
    (`web-bundle-size`) + D7-12 (`web-e2e-embed`) are new required
    checks.

### Phase 2 carry-forward (LOCKED — API contract)
- `.planning/phases/02-openapi-contract-codegen/02-CONTEXT.md` —
  D-32..D-48. Especially:
  - **D-35, D-36** error envelope `{error, reason, request_id}` —
    embed auth-expired banner displays `requestId` from the 401
    response.
  - **D-38** `createApiClient({ baseURL, getOrgId, fetch? })` factory
    — embed instantiates per-element; `getOrgId` reads attribute,
    `baseURL` reads `api-base-url` attribute.

### Phase 04.1 + 5 carry-forward (LOCKED — entity shapes)
- `.planning/phases/04.1-catalog-identity-normalization/04.1-CONTEXT.md`
  — universal `code` field, validation regex, immutability. Embed
  inherits via packages/ui components.
- `.planning/phases/05-bulk-import-go/05-CONTEXT.md` — import schemas,
  CSV/JSON shapes, idempotency. Embed's lazy `import-page` chunk
  consumes.

### Research
- `.planning/research/SUMMARY.md` §MFE Topology + §Top 10 Pitfalls —
  pre-pivot research mentioned Module Federation; translate patterns
  to Web Components (Shadow DOM CSS isolation = "Tailwind prefix"
  analog from pitfall 10). The blocker around "host integration test
  harness must be set up at Phase 5 start" was post-pivot remapped to
  Phase 7 (per STATE.md Blockers/Concerns).

### External standards (READ for embed-specific idioms)
- [Lit Custom Element guide](https://lit.dev/docs/components/overview/)
  — `connectedCallback`, attribute reflection, Shadow DOM modes.
- [`@lit-labs/router` README](https://github.com/lit/lit/tree/main/packages/labs/router)
  — `Routes` class, `outlet()`, `URLPattern` route definitions, `enter`
  hook (D7-03 lazy-route entry point).
- [size-limit docs](https://github.com/ai/size-limit) — `.size-limit.json`
  config shape, gzip vs brotli measurement, `@size-limit/preset-app`
  preset.
- [`andresz1/size-limit-action`](https://github.com/andresz1/size-limit-action)
  — GH Action that posts PR comments with size deltas.
- [`go-chi/cors`](https://github.com/go-chi/cors) — middleware shape,
  `cors.Options{AllowedOrigins, AllowedHeaders, AllowedMethods,
  MaxAge}`.
- [Playwright importmap testing](https://playwright.dev/docs/test-fixtures)
  — running specs against statically-served HTML with esm.sh CDN
  imports.
- [esm.sh](https://esm.sh/) — `https://esm.sh/react@18.3.1` URL pattern.
- [MDN: HashChangeEvent](https://developer.mozilla.org/en-US/docs/Web/API/HashChangeEvent)
  — `hashchange` event semantics for D7-06 hash router adapter.
- [WHATWG URLPattern spec](https://urlpattern.spec.whatwg.org/) — used
  by `@lit-labs/router`; available natively in Chrome 95+, polyfilled
  by admin's `urlpattern-polyfill` dep (embed reuses).
- [MDN: CustomEvent composed flag](https://developer.mozilla.org/en-US/docs/Web/API/Event/composed)
  — D7-13/D7-14 mandatory.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (Phase 6 shipped 2026-05-18)
- **`<or-catalog-shell>`** at
  `web/packages/ui/src/components/shell/catalog-shell.ts` (21.4 KB
  source) — top-level shell with @lit-labs/router (history mode),
  sidebar nav with `modules` filter, theme-on-host CSS property
  injection (D6-20), client-side UUIDv7 route param validation. Phase 7
  refactors entity imports here for lazy loading + adds `routingMode`
  prop.
- **Entity components** at `web/packages/ui/src/components/{agents,
  skills,queues,channels,adapters,break-reasons}/` — full CRUD trios
  (list / detail / form). 18 files total. Phase 7 makes them lazy
  imports from shell.
- **Status panel** at `web/packages/ui/src/components/status/
  status-panel.ts` — pauses on `document.hidden` (D6-26); works inside
  Shadow DOM.
- **Import flow** at `web/packages/ui/src/components/imports/
  import-page.ts` + `import-result.ts` — Phase 7 makes lazy.
- **Primitives** at `web/packages/ui/src/components/primitives/` —
  `or-data-table`, `or-cursor-paginator`, `or-code-input`,
  `or-conflict-banner`, `or-form-wizard`, `or-org-picker` (admin-only —
  embed does not mount it). All eager in Phase 7 baseline.
- **Theme tokens** at `web/packages/ui/src/themes/index.ts` —
  `orLight`, `orDark`, `orBrand`, `ALL_TOKEN_KEYS`. Embed parses
  `theme=` attribute (named string or JSON) and applies via the same
  D6-20 mechanism.
- **Locale bootstrap** at `web/apps/admin/src/index.ts` —
  `configureLocalization` with `sourceLocale` + `targetLocales` from
  `@open-routing/ui/locales/locale-codes.js`. Embed mirrors but reads
  locale from element attribute.
- **API client factory** `createApiClient` at
  `web/packages/ui/src/api/client.ts` — `{ baseURL, getOrgId }` factory.
  Embed instantiates per-element from attributes.
- **Validators** at `web/packages/ui/src/validators/` — ajv-compiled
  from OpenAPI; embed consumes transitively through entity components.

### Established Patterns
- **`@rolldown/plugin-babel` decorators config** (Phase 6 D-78 carryover
  in admin's `vite.config.ts`) — embed's `vite.config.ts` reuses same
  Babel preset config for `@customElement`/`@property` decoration.
- **Per-component Shoelace imports** (D6-08) — entity components
  already do this; precondition for the 70 KB budget. No central
  Shoelace registry.
- **Turbo task dependencies** in `web/turbo.json` — `build` depends on
  `gen:api`, `gen:validators`, `^typecheck`. Phase 7 adds `apps/embed`
  to the workspace; turbo auto-orchestrates.
- **`pnpm -F '*'`** patterns in CI (`web-typecheck`, `web-lint`) —
  `apps/embed` participates automatically once `package.json` is
  populated.
- **Playwright config shape** at `web/apps/admin/playwright.config.ts`
  — Phase 7 adapts for multi-project (3 hosts) matrix.

### Integration Points
- **Phase 6 shell static imports** → Phase 7 refactors to dynamic
  `import()` per route. Must not regress admin smoke (Plan 06-06).
- **Go API CORS gap** — currently no CORS middleware. Embed
  cross-origin calls require one. D7-16 ships it.
- **Vite Library Mode** — embed builds differently from admin (admin
  is `appType: 'spa'`; embed needs `build.lib` with single ES entry +
  manualChunks for lazy chunks). New `vite.config.ts` in `apps/embed/`.
- **size-limit + GH Action** — first dep ecosystem touch in Phase 7;
  zero existing usage in repo.
- **esm.sh CDN dependency in Playwright tests** — first time CI hits
  an external CDN. Acceptable risk; `retries: 1` absorbs flakes.
- **`urlpattern-polyfill`** at `web/apps/admin/package.json` devDeps —
  embed reuses this polyfill via shell dependency.

### Phase 7 hits clean-slate code in `web/apps/embed/`
- `web/apps/embed/src/index.ts` is currently `export {};` (1.5 K
  comment + empty export). Phase 7 fills in.
- `web/apps/embed/package.json` only declares `typecheck` + `lint`
  scripts. Phase 7 adds `dev`, `build`, `preview`, `test`, `test:e2e`,
  `size` + deps (`lit`, workspace `@open-routing/ui`, dev:
  `vite`, `@rolldown/plugin-babel`, `vitest`, `@playwright/test`,
  `size-limit`, `@size-limit/preset-app`).
- No `web/apps/embed/vite.config.ts`, no `e2e/`, no `dist/` — fresh.

</code_context>

<specifics>
## Specific Ideas

- **"Hybrid mount-shell + lazy routes"** — user picked the
  Phase-6-anticipated path (D6-05 left this open). Drives D7-01..D7-03.
  Key insight: the ONLY way to single-source-of-truth shell across
  admin and embed AND hit 70 KB is to make entity imports dynamic at
  the shell level — not at the wrapper level.
- **Hash routing inside embed, history stays for admin** — user
  picked the routing-mode prop on shell rather than unifying both apps
  on hash. Preserves admin URL aesthetics; isolates embed from host
  router. Drove D7-06.
- **"Single embed per page" documented limitation** — user accepted
  the realistic v0.1 trade-off (window.location.hash is global, no
  per-instance prefix yet). Drove D7-07 + deferred-ideas entry.
- **Error state inside Shadow DOM on bad org-id** — user explicitly
  rejected falling back to org-picker, preserving EMBED-02
  "attribute-only" contract. Drove D7-08.
- **size-limit declarative gate** — user picked the trusted ecosystem
  option over custom Node script. Drove D7-09.
- **Eager baseline budget; soft per-chunk budgets** — user interpreted
  EMBED-01 strictly (single ES module = eager entry only). Lazy chunks
  get logged-not-blocked budgets. Drove D7-10.
- **Static HTML + esm.sh CDN stub hosts** — user picked the lightweight
  option over full Vite SPAs per framework. Drove D7-11.
- **`retries: 1` per spec + identical EMBED-10 assertions across 3
  hosts** — user picked a balanced flake-tolerance over strict-0-retry
  and warn-only-failures. Drove D7-12.
- **announce-only request-context, every-401 auth-expired** — user
  rejected negotiable / preventDefault contracts as premature for v0.1
  stub auth. Drove D7-13 + D7-14.
- **No npm publish in v0.1; tarball-ready package metadata only** —
  user accepted the deferral to v1 (no real customers yet, premature
  brand exposure). Drove D7-15.
- **Go API ships CORS middleware (go-chi/cors)** — user picked the
  "API owns CORS" path over host-reverse-proxy-only documentation.
  Drove D7-16; adds first Go middleware change in Phase 7.

</specifics>

<deferred>
## Deferred Ideas

These came up during discussion but explicitly belong outside Phase 7.

- **Per-instance hash-prefix attribute for multi-embed pages** —
  `<open-routing-catalog hash-prefix="foo">` enabling `#foo/orgs/...`.
  v0.2 if customer demand for 2-embed split-pane comparison emerges.
- **history-mode routing for embed with base-path attribute** — would
  let embed coexist with host URL space cleanly. Higher host
  integration burden than v0.1 wants. Reconsider in v1 if customer
  hosts have flexible routing.
- **Real npm/CDN publishing of `@open-routing/catalog-embed`** — v1
  alongside real auth + first customer integration. Phase 7 ships
  tarball-installable artifact only.
- **Unified hash routing across admin + embed** — would simplify shell
  routingMode logic; rejected for admin UX aesthetic.
- **history + memory + hash routing as 3 user-selectable modes via
  attribute** — premature flexibility for v0.1. Hash is the default
  and only mode in v0.1.
- **Negotiable open-routing:request-context with preventDefault-blocks
  -boot** — reserved for v1 auth phase if real SSO needs handshake.
- **First-401-only auth-expired with UI lock** — rejected for v0.1
  (continues fetching, host owns recovery). Reconsider in v1 if event
  storms become a host problem.
- **GitHub Packages private publishing path** — workable for design
  partners but unnecessary for v0.1 internal use.
- **Per-org brand theme customization (orgs.theme column)** — Phase 6
  D6-19 deferral carries forward.
- **Server-side rendering of `<open-routing-catalog>`** — Custom
  Elements are client-rendered only in v0.1. Hosts wanting SSR
  placeholder + hydrate on connectedCallback.
- **Operator-facing CORS allowlist UI per org** — v0.1 ships env-var
  allowlist; runtime-milestone may add per-org UI control.
- **Embed-side localStorage / sessionStorage usage** — v0.1 reads
  attributes only; no embed persistent state. Host owns persistence.
- **Bundle-analyzer (rollup-plugin-visualizer) publish to PR artifact**
  — useful debugging aid; v0.1 ships size-limit gate without graphical
  visualization. v0.2 can add as a non-blocking PR comment.
- **Brotli + raw-minified bundle budgets** — Phase 7 ships gzip-only.
  Brotli (the actual CDN-served size in 2026) and raw can be added to
  size-limit JSON as additional logged budgets in v0.2.

### Reviewed Todos (not folded)
None — `cross_reference_todos` returned 0 matches for Phase 7. The
sole STATE.md pending todo (Phase 3 simplification cleanup) belongs in
Phase 3 follow-up, not Phase 7.

</deferred>

---

*Phase: 7-Web Component Embed Bundle*
*Context gathered: 2026-05-18*
*Next: `/gsd-plan-phase 7` (research → plan → verify). Researcher
should verify (a) the `routingMode` prop refactor preserves Plan 06-06
admin smoke behavior, (b) `@lit-labs/router` `enter` hook supports the
`async () => { await import(); return component; }` pattern for D7-03,
(c) size-limit's gzip measurement matches CDN-served byte sizes within
a reasonable margin, and (d) `go-chi/cors` preflight ordering relative
to the existing Phase 1 chi middleware chain.*
