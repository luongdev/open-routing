# Phase 6: Shared UI Library & Standalone Admin - Context

**Gathered:** 2026-05-17
**Status:** Ready for planning

> **Decision-ID convention:** Phase-local prefix `D6-NN`. Continues the
> per-phase prefix convention used by Phase 04.1 (`D04_1-NN`) and Phase 5
> (`D5-NN`) — keeps Phase 6's UI/SPA decisions distinct from the continuous
> Phase 1-4 backend sequence (`D-01..D-95`) and from any future numbering
> for Phase 7 (anticipated `D7-NN`).

<domain>
## Phase Boundary

Ship the v0.1 standalone admin SPA + shared component library that Phase 7's
Web Component embed bundle will reuse. The admin SPA is a Vite + Lit 3 +
Shoelace TypeScript build at `web/apps/admin/`; the shared component library
lives at `web/packages/ui/` and exports a hybrid surface — per-entity Lit
components (`<or-agent-list>`, `<or-agent-form>`, `<or-status-panel>`, ...)
**plus** an optional `<or-catalog-shell>` page-level shell that wires
`@lit-labs/router` + sidebar navigation. Both admin and (in Phase 7) embed
can compose either layer.

The admin delivers full CRUD over the 6 catalog entities (agents, skills,
queues, channels, adapters, break_reasons), plus the agent status panel
(PATCH `/agents/{id}/status` with admin `force` flag from Phase 4 D-84)
and the bulk import UI (POST `/catalog/import` + GET `/imports/{id}` from
Phase 5). Themes (light + dark + brand) are switchable at runtime via CSS
custom properties applied on the shell host element — the same mechanism
Phase 7 will use to consume the `theme=<JSON>` Custom Element attribute.

**In scope:**
- **`web/packages/ui/` expansion** beyond the existing Phase 2 API surface
  (`createApiClient`, `ApiError`/`ErrorCodes`, `createApiTask`):
  - Per-entity components: `<or-agent-list>`, `<or-agent-detail>`,
    `<or-agent-form>` (×6 entities = 18 components); each registers its
    own Shoelace deps (`import '@shoelace-style/shoelace/dist/components/input/input.js'`)
    to preserve tree-shaking for Phase 7's 70KB gzipped bundle budget.
  - Status panel component: `<or-status-panel>` (per Sketch 3 — current
    status display + transition buttons + break reason picker +
    post_interaction_state radio + wrapup countdown + force-flag affordance
    for admin recovery).
  - Bulk import components: `<or-import-page>` + `<or-import-result>`
    (per Sketch 2 — entity picker + file upload + 207
    `BulkImportResult.failed[]` table + 413 oversized message + CSV
    `code:prof|code:prof` format helper).
  - Optional page-level `<or-catalog-shell modules="..." theme="..." org-id="...">`
    that includes `@lit-labs/router`, sidebar with 8 nav entries (6 catalog
    + Bulk Import + Agent Status placeholder), top bar with theme toggle +
    locale toggle + "Switch org" link.
  - Primitives: `<or-data-table>`, `<or-cursor-paginator>`,
    `<or-conflict-banner>` (the 409 auto-update inline banner from D6-04),
    `<or-form-wizard>` (multi-step wizard for complex create flows), and
    `<or-code-input>` (validates Phase 04.1 regex `^[a-z][a-z0-9_]{0,63}$`).
  - **OpenAPI-driven validators**: a Vite-time codegen step (mirrors
    `gen:api`) compiles `components.schemas.*` to ajv-standalone modules
    at `web/packages/ui/src/validators/`. Per-entity validators are
    tree-shakeable; admin/embed import only what's used. Drift gate
    (CI codegen check) mirrors Phase 2 D-47.
  - i18n via `@lit/localize`: 2 locales (EN + VI), lazy-loaded modules
    per locale, `navigator.language` detection + localStorage override.
  - Theme tokens: 3 named themes (`or-light`, `or-dark`, `or-brand`),
    applied as CSS custom properties on `<or-catalog-shell>` host element
    (mirrors Shoelace's own theme convention).
- **`web/apps/admin/` Vite SPA**:
  - `vite.config.ts` with dev proxy (`/v1` + `/healthz` + `/readyz` →
    `http://localhost:8080`) so admin runs same-origin against the Go
    `task dev` server (no CORS in dev).
  - Single entry `apps/admin/src/index.ts` that registers
    `<or-catalog-shell>` against `document.body`. Org_id sourced from URL
    path `/orgs/:org_id/...`; root `/` shows org-picker (UUIDv7 input,
    last-used pre-filled from localStorage).
  - Routes registered with `@lit-labs/router` inside the shell:
    - `/` → org-picker
    - `/orgs/:org_id/agents` (list)
    - `/orgs/:org_id/agents/new` (multi-step wizard create)
    - `/orgs/:org_id/agents/:id` (detail/edit, full page)
    - `/orgs/:org_id/agents/:id/status` (status panel)
    - `/orgs/:org_id/skills`, `/orgs/:org_id/skills/new`, `/orgs/:org_id/skills/:id`
    - … (×6 entities, same shape — wizard only for Agent + Channel; others
      use single-step form via the same `<or-form-wizard>` component)
    - `/orgs/:org_id/imports/new` (POST flow)
    - `/orgs/:org_id/imports/:id` (GET historical result)
  - org_id validation: client-side UUIDv7 regex (in `packages/ui`) +
    server-side reject (FOUND-03 + middleware.UUIDv7PathParams).
  - 404 from API → inline empty-state per page (matches Sketch 1 error
    state pattern).
- **Tests**: Vitest + happy-dom for component tests beside source
  (`*.test.ts` in `packages/ui/src/components/`). Admin SPA gets a
  single Playwright smoke test (manual run; not yet in CI) covering
  `/` → org-picker → `/orgs/:id/agents` → create agent → see in list.
  Playwright harness is reused by Phase 7's EMBED-10 integration tests.
- **CI**: extend `.github/workflows/ci.yml` with codegen drift gate
  (`pnpm gen:api && pnpm gen:validators && git diff --exit-code`) and
  `pnpm --filter @open-routing/admin build` production-build check.
  E2E Playwright NOT in CI for this phase (manual-only); Phase 7 adds it.

**Out of scope:**
- Phase 7's `apps/embed` Web Component bundle (separate phase). Phase 6
  designs `packages/ui` for embed reuse but does NOT build the embed
  Custom Element wrapper.
- Real auth — stub `X-Org-Id` header only; no login form, no session
  expiry handling beyond Phase 7's EMBED-07 `open-routing:auth-expired`
  contract reservation.
- Server-side rendering / SSR — Vite SPA only, client-side rendering.
- Audit log UI — no surface in v0.1 (audit deferred to runtime milestone).
- Real-time push (SSE/WebSocket) — REST polling only per PROJECT.md.
  Status panel polls 5s; pauses on `document.hidden`; rechecks
  `state_version` (STATE-08) on resume.
- Catalog list polling — refresh-on-action only (no periodic poll). User
  has `[Refresh]` button per list.
- Async bulk import UI (IMP-09 deferred to v0.2) — admin shows 413 with
  the documented v0.2 async pathway message.
- Bulk import error CSV download (IMP-11 deferred to v0.2).
- Multi-org dropdown / inline org switcher — Phase 6 ships only the
  hard-reset-to-org-picker pattern.
- Server-translated error messages — `reason` field stays English from
  the server; admin client-side maps `ErrorCode` enum → i18n key for
  display (`reason` shown in a collapsed "Technical details" section).
- Accessibility audit / screen reader certification — Shoelace defaults
  cover the baseline; explicit a11y testing deferred.
- Source-map upload to error tracker — frontend errors → console only
  in v0.1 (observability beyond `request_id` display deferred).
- Production deployment automation — Phase 6 ships build artifacts; ops
  team handles static hosting separately.

</domain>

<decisions>
## Implementation Decisions

### Scope Boundary (Area 1)

- **D6-01:** **Full admin scope in this phase.** Ship 6 entity CRUD +
  agent status panel (PATCH /status with force flag) + bulk import UI.
  Matches Phase 5's downstream expectation (Phase 5 CONTEXT D5-17/D5-18
  explicitly says "Phase 6 admin will add a bulk-import screen") and uses
  the four UI sketches already produced in Phase 2 (02-UI-SKETCHES.md).
- **D6-02:** **Wave-based layered split.** Wave 1 establishes the scaffold
  (Vite config, packages/ui primitives, codegen for validators, i18n
  setup, theme tokens, `<or-catalog-shell>` skeleton). Wave 2 builds the
  shared CRUD framework + Agents as exemplar entity (validates the
  entity-component pattern end-to-end). Waves 3-4 parallelize the
  remaining 5 entities. Wave 5 ships the status panel + import UI +
  Playwright smoke test. Mirrors Phase 3's "establish patterns first,
  then parallelize" approach.
- **D6-03:** **Distinguish CRUD 409 from status 409 semantically; same
  underlying mechanism.** CRUD `409 version_conflict` (CAT-08) → consume
  the `current: <Entity>` field from the response body (D-37) and update
  local form state in place. Status PATCH `409 invalid_transition`
  (STATE-03) → consume the `from`/`to` fields from the response body
  (D-91) and update local status display. NEITHER triggers a second
  GET round-trip — server already returned the truth in the 4xx body.
- **D6-04:** **Auto-update + inline banner UX.** After the 4xx body is
  consumed (D6-03), render `<or-conflict-banner>` inline above the form
  / status panel:
  - CRUD: "This was changed elsewhere. Your edits are below — review and
    re-submit?" with a side-by-side diff highlight of the new server
    value vs the user's pending edits.
  - Status: "Status changed to `{from}` — your request to go to `{to}`
    isn't allowed from `{from}`." with a "Try again" CTA that resets the
    transition picker against the new `from`.
  Both banners auto-dismiss after the user takes action (re-submits or
  clicks "Got it"). Toast / modal patterns rejected.

### `packages/ui` Reuse Boundary (Area 2)

- **D6-05:** **Hybrid layer model.** `packages/ui` exports BOTH per-entity
  Lit components (`<or-agent-list>`, `<or-agent-detail>`, `<or-agent-form>`,
  ×6 entities; plus `<or-status-panel>`, `<or-import-page>`,
  `<or-import-result>`) AND an optional page-level
  `<or-catalog-shell modules="..." theme="..." org-id="...">` that
  composes them with built-in routing + sidebar + chrome. Admin mounts
  the shell; Phase 7 embed can either mount the shell (heavy reuse) or
  compose entity components directly (lean wrapper). Entity components
  do NOT depend on the shell — they take props/attributes and render.
- **D6-06:** **Router + nav live inside the shell.** `@lit-labs/router`
  + the sidebar Lit components are private internals of
  `<or-catalog-shell>`. Apps that mount the shell get the chrome free.
  Apps that compose entity components directly bring their own router
  and skip the dep tree. Trade-off: shell takes opinions on routing/nav
  pattern; lean composition is the escape hatch.
- **D6-07:** **`@lit-labs/router`** for the router library. Lit-native
  (same maintainers as Lit 3), smallest bundle (~5KB), tightest
  integration with `outlet()` pattern. Labs status acknowledged: API
  could shift in a minor release; v0.1's 8-route scope makes future
  migration cheap (~30 lines of router config). Rejected: @vaadin/router
  (more mature but decoupled, slightly bigger) and hand-rolled
  (reinvents browser-back / hash-mode-in-Shadow-DOM edge cases).
- **D6-08:** **Per-component Shoelace imports.** Each Lit component
  imports the specific Shoelace components it uses directly
  (`import '@shoelace-style/shoelace/dist/components/input/input.js'`
  inside `<or-agent-form>`). Lit defines elements idempotently; double
  imports are free. Tree-shaking ACTUALLY works — if embed mounts only
  `<or-agent-list>`, only `<sl-button>` + `<sl-input>` + `<sl-table>` get
  bundled. Required to hit Phase 7's 70KB gzipped budget. Rejected:
  central `registerShoelace()` (breaks tree-shaking) and Shoelace
  autoloader CDN (incompatible with Shadow DOM).

### Routing + Auth Bootstrap (Area 3)

- **D6-09:** **URL path encodes org_id.** Routes are
  `/orgs/:org_id/agents`, `/orgs/:org_id/agents/:id`, etc. Router parses
  org_id from URL and feeds it into `createApiClient({ getOrgId: () =>
  currentOrgId })`. Bookmarkable; multi-tab supports different orgs in
  different tabs. Admin uses URL; Phase 7 embed will use the `org-id`
  HTML attribute — same `createApiClient` factory, different source
  feeding `getOrgId`.
- **D6-10:** **Root `/` shows org-picker with last-used pre-filled.**
  `<or-org-picker>` is a minimal page: UUIDv7 input + "Continue" button.
  Persists last-used org_id in localStorage; pre-fills on reload of `/`.
  Unknown routes (and routes off of an invalid org_id) redirect to `/`
  with a toast explaining what happened.
- **D6-11:** **Sidebar with 8 nav entries.** Left sidebar lists:
  Agents · Skills · Queues · Channels · Adapters · Break Reasons · ── ·
  Bulk Import · Agent Status (entry-point shows "select an agent"
  empty state when no agent_id; lists last 5 agents accessed). Active
  entry highlighted. Top bar shows truncated org_id + theme toggle +
  locale toggle + "Switch org" link. Phase 7 embed will use the
  `modules="..."` filter to hide unlisted entries — sidebar entries
  honour the filter when present.
- **D6-12:** **Mixed routing for status panel + import.** Status panel
  is nested per-agent: `/orgs/:org_id/agents/:id/status`. Import is
  top-level: `/orgs/:org_id/imports/new` (POST flow) +
  `/orgs/:org_id/imports/:id` (GET historical result). Matches OpenAPI
  shape: PATCH `/agents/{id}/status` is per-agent; POST `/catalog/import`
  is org-level.
- **D6-13:** **Both client-side and server-side UUIDv7 validation.**
  Client: `<or-catalog-shell>` validates `:org_id` route param against
  UUIDv7 regex before any API call; malformed → redirect to `/` with
  toast. Server: middleware.UUIDv7PathParams rejects malformed path
  params with HTTP 400/invalid_id (FOUND-03 carry-forward). Both
  layers — client is UX nicety, server is the source of truth.
- **D6-14:** **"Switch org" → hard reset.** Click "Switch org" or
  manually change `:org_id` in URL → navigate to `/`. Org-picker shows
  (with new candidate value pre-filled if URL contained one). All
  in-memory state cleared. Simple, matches single-tab-single-org mental
  model. Inline org dropdown rejected as out-of-scope for v0.1.
- **D6-15:** **404 from API → inline empty-state per page.** Detail
  page renders its normal shell with a centered empty state:
  "Agent not found in this org. [Back to agents]". Status panel:
  "No status found for this agent." Each page owns its 404 UI; no
  dedicated 404 route. Matches Sketch 1's error-state pattern.

### CRUD Form + 409 + Theme (Area 4)

- **D6-16:** **Edit pattern = full detail page.** Row click on a list
  navigates to `/orgs/:org_id/{entity}/:id`. Detail page renders the
  read+edit form with Save / Cancel buttons. Matches Sketch 1 visual
  contract. Bookmarkable, deep-linkable, room for the skills sub-table
  on Agent + (in a later phase) audit log entries. Rejected: modal
  overlay (cramped for complex forms) and slide-over drawer (heavier
  visual treatment for no win).
- **D6-17:** **Create pattern = multi-step wizard for complex entities,
  single-step form for simple ones.** Same `<or-form-wizard>` component
  for all 6 entities:
  - **Multi-step:** Agent (Step 1 basics: code/name/email/enabled →
    Step 2 skills: assign skills + proficiency → Step 3 review &
    create); Channel (Step 1 basics: code/name/channel_type/enabled →
    Step 2 default_queue picker → Step 3 review).
  - **Single-step:** Skill, Queue, BreakReason, Adapter (the wizard
    component auto-completes step 1 → review for entities with simple
    field sets). Consistent UX across entities, right-sized per entity
    complexity. URL: `/orgs/:org_id/{entity}/new`. On successful POST →
    redirect to `/orgs/:org_id/{entity}/:newId`.
- **D6-18:** **Form validation source = OpenAPI codegen via ajv.**
  Build-time codegen step parses `openapi/openapi.yaml`
  `components.schemas.*`, compiles to ajv-standalone validators
  (`ajv-cli --code`). Output: `web/packages/ui/src/validators/<schema>.ts`
  per schema, tree-shakeable. Runtime cost: ~0 (compiled). Single source
  of truth: the OpenAPI spec. Drift gate (CI): `pnpm gen:api &&
  pnpm gen:validators && git diff --exit-code`. Per-keystroke validation
  in forms uses the same compiled validator. Server stays authoritative;
  client is UX nicety. Rejected: hand-written Zod (drift risk + extra
  ~6KB), HTML5 Constraint Validation API alone (no shared source of
  truth across pattern/min/max).
- **D6-19:** **3 named themes: `or-light`, `or-dark`, `or-brand`.**
  Each theme = a CSS file defining `--sl-color-primary-*` overrides
  (mapping to Shoelace's CSS variable namespace) plus a couple of
  Open-Routing-specific layout tokens. Light is the default; toggle
  cycles light → dark → brand → light in the top bar. Persisted in
  localStorage; switching is class-swap or property-set, never a reload.
  Brand theme uses Open Routing primary color (to be decided by planner
  — recommend mid-tone teal/blue to differ from Shoelace's default
  primary; not a blocker).
- **D6-20:** **Theme injected on `<or-catalog-shell>` host element.**
  Shell has a `theme` reactive property accepting either a string
  (named theme: `'or-light' | 'or-dark' | 'or-brand'`) or a JSON
  object (arbitrary CSS-custom-property map). When set, the shell
  iterates the resolved token map and calls
  `this.style.setProperty('--sl-color-primary-500', value)` etc. on
  its own host element. CSS cascade carries the tokens into all entity
  components and Shoelace components inside the shell. Same mechanism
  works for admin (string from toggle) AND Phase 7 embed (JSON from
  `theme=` attribute on `<open-routing-catalog>`, parsed and forwarded
  to the shell). Rejected: `document.documentElement` class (doesn't
  cascade into Shadow DOM — fork between admin and embed) and Lit
  context provider (verbose vs CSS cascade).

### i18n / Locale (Area 5)

- **D6-21:** **EN + VI bilingual from v0.1.** Top bar has a locale
  toggle (next to theme toggle). Both string sets ship in the admin
  bundle (lazy-loaded by locale via `@lit/localize`'s
  `configureLocalization`).
- **D6-22:** **`@lit/localize`** as the i18n library. Official Lit
  team library, `msg(html`Hello ${name}`)` API, XLIFF extract +
  per-locale compile workflow (`lit-localize extract` / `lit-localize
  build`). Lazy-load locale modules. ~3KB runtime. Type-safe via
  `msg` overloads. Rejected: hand-rolled key-value map (no pluralisation /
  datetime formatting) and Format.js (~10KB, React-flavoured wrap-cost).
- **D6-23:** **Default locale = `navigator.language` detect; user
  override persisted to localStorage.** First visit: read
  `navigator.language`; if it starts with `'vi'` → VI, otherwise → EN.
  Toggle stores explicit choice in localStorage; subsequent visits read
  localStorage first. Server is not in the loop (stub auth only).
- **D6-24:** **Server error messages translated client-side via
  `ErrorCode` enum.** Closed enum on the wire (Phase 2 D-36:
  `invalid_body | not_found | duplicate_code | duplicate_external_id |
  immutable_field | version_conflict | invalid_transition | invalid_org_id
  | invalid_id | import_failed | cross_org | rate_limited | internal`).
  Client maps each code → i18n key (`errors.invalid_body`,
  `errors.duplicate_code`, ...) and shows the translated message.
  Server's `reason` field (English detail) renders in a collapsed
  "Technical details" disclosure under the friendly translated message.
  `request_id` always visible (for support). Server-side translation
  rejected as out of scope for v0.1.

### Polling + Freshness (Area 6)

- **D6-25:** **Catalog list pages: refresh-on-action only.** List fetch
  fires on component mount and on user action (search input change,
  paginate, return-from-edit). NO periodic polling on lists — catalog
  data changes slowly (admins don't create agents every second), polling
  burns bandwidth and server load for no win. Each list has a
  `[Refresh]` button for explicit reload. Aligns with PROJECT.md REST
  polling philosophy.
- **D6-26:** **Status panel: poll 5s when visible; pause when hidden;
  state_version check on resume.** `<or-status-panel>` registers a
  `visibilitychange` listener. When `document.hidden === true`, the
  `@lit/task` polling task cancels. When tab becomes visible again,
  fire one immediate GET; compare returned `state_version` (STATE-08)
  to last known. If newer → update UI; resume 5s timer. Saves bandwidth
  for backgrounded tabs without stale UX on tab-return. Multi-tab safe
  (each tab polls independently; PROJECT.md acknowledged no shared
  worker dedup in v0.1).
- **D6-27:** **No client-side cache layer.** Each component mount
  refetches via `@lit/task`. Phase 3's Redis cache (CAT-11) makes GET
  responses < 100ms on hot path; single source of truth = server. No
  module-level cache, no sessionStorage persist. Trade-off: tab-back
  navigation re-fetches; acceptable given the Redis layer. Stale-state
  confusion risk removed.

### Testing + Dev Experience (Area 7)

- **D6-28:** **Vitest + happy-dom** for component tests in
  `packages/ui`. Vitest is already the test runner (current API client
  tests in `web/packages/ui/src/api/client.test.ts`). Add `environment:
  'happy-dom'` to `vitest.config.ts`. happy-dom is ~2-3× faster than
  jsdom and handles modern Web Component APIs (CSSStyleSheet, adopted
  styles) better. Tests live beside source: `*.test.ts` colocated with
  the component file (matches Phase 3 D-72 backend convention).
- **D6-29:** **Vite dev server proxies `/v1` + `/healthz` + `/readyz`
  to `localhost:8080`.** `apps/admin/vite.config.ts` config:
  ```ts
  server: { proxy: {
    '/v1':      'http://localhost:8080',
    '/healthz': 'http://localhost:8080',
    '/readyz':  'http://localhost:8080',
  }}
  ```
  Admin fetches relative URLs (`/v1/orgs/...`) — same-origin in dev,
  no CORS needed. Backend `task dev` (Phase 1 D-25 air hot-reload) runs
  on 8080; `pnpm --filter @open-routing/admin dev` runs Vite on 5173.
  Two terminals; both have hot reload.
- **D6-30:** **Playwright smoke test in Phase 6; matrix runs in Phase 7.**
  Single test at `web/apps/admin/e2e/smoke.spec.ts`: navigate to `/`,
  enter test org_id, navigate to `/orgs/:org_id/agents`, click [+ Create
  Agent], complete wizard, assert agent appears in list. Manual-only
  in Phase 6 (developer runs `pnpm test:e2e` locally). The Playwright
  harness (config, fixtures, page objects) is reused by Phase 7's
  EMBED-10 React/Vue/HTML stub host matrix. Avoids two separate E2E
  infrastructures.
- **D6-31:** **CI extension: codegen drift + production build only.**
  `.github/workflows/ci.yml` Phase 6 additions:
  - `pnpm --filter @open-routing/ui gen:api` (regenerate TS types from
    openapi.yaml)
  - `pnpm --filter @open-routing/ui gen:validators` (regenerate ajv
    validators from openapi.yaml)
  - `git diff --exit-code` (drift gate for both)
  - `pnpm --filter @open-routing/admin build` (production Vite build
    smoke — catches Lit decorator / SSR misconfig early)
  E2E Playwright NOT in CI for Phase 6 (developer runs locally). Phase 7
  promotes Playwright to CI as part of EMBED-10. Aligns with FOUND-09
  CI shape; one extra `pnpm` step under the existing
  `frontend-typecheck-lint-unit` job.

### Claude's Discretion

- Exact internal directory layout under `web/packages/ui/src/components/`
  (e.g., `components/entities/agents/agent-list.ts` vs
  `components/agent-list/agent-list.ts`) — planner picks; should be
  consistent across all 6 entities.
- Sidebar visual design / icon choice — planner picks Shoelace icons or
  Material; brand-theme primary color value to be settled in plan
  (mid-tone teal/blue recommended but not blocking).
- Exact ajv codegen wrapper script (Node script vs Vite plugin) —
  planner picks; output location locked at
  `packages/ui/src/validators/`.
- @lit/localize directory layout for XLIFF files (`web/packages/ui/xliff/`
  vs `web/packages/ui/src/locales/`) — planner picks; recommend a single
  shared XLIFF directory since both admin and (Phase 7) embed share the
  same string set via `packages/ui`.
- `<or-form-wizard>` step-state persistence — in-memory only (lost on
  navigation away) vs sessionStorage (survives accidental reload)?
  Recommend in-memory for v0.1; revisit if user reports lost work.
- Theme-toggle UX shape (`<sl-button-group>` with three buttons vs a
  single button that cycles vs an `<sl-select>`) — planner picks;
  three-button group is most discoverable.
- Org-picker validation timing (debounce keystroke vs on-blur vs on-submit)
  — planner picks; on-submit is simplest and matches the modal feel.
- Whether `<or-conflict-banner>` includes a "discard local edits" button
  alongside "review & re-submit" — planner picks based on Sketch 1 + Phase
  3 RESPONSE thinking; recommend including discard since admins sometimes
  realize the server's truth is fine.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level locks (PROJECT.md, REQUIREMENTS.md, ROADMAP.md, STATE.md)
- `.planning/PROJECT.md` — Key Decisions table (locked stack: Vite + Lit
  + Shoelace + TypeScript; embed = Web Components + Shadow DOM; REST
  polling only; browser workers deferred). Note "Performance is part of
  the product value" applies indirectly to admin (snappy CRUD).
- `.planning/REQUIREMENTS.md` §Standalone Admin App — **ADMIN-01 through
  ADMIN-06**. Acceptance criteria: Vite SPA static bundle (ADMIN-01),
  CRUD screens for 6 entities (ADMIN-02), org_id from URL or stub login +
  X-Org-Id header (ADMIN-03), generated TS client only (ADMIN-04), 409
  reload-prompt UX (ADMIN-05), Shoelace theme tokens runtime-switchable
  (ADMIN-06).
- `.planning/REQUIREMENTS.md` §Web Component Embed Bundle — **EMBED-01
  through EMBED-10** (Phase 7). Phase 6's `packages/ui` design choices
  must enable these: 70KB gzipped (EMBED-01), `org-id` attribute
  threading (EMBED-02), entity reuse from `packages/ui` (EMBED-03),
  Shadow DOM CSS isolation (EMBED-04), `theme=<JSON>` attribute
  (EMBED-05), `modules=` attribute filter (EMBED-06), 409 reload-prompt
  parity with admin (EMBED-08).
- `.planning/ROADMAP.md` §Phase 6 — Goal statement + 5 success criteria.
- `.planning/STATE.md` §Blockers/Concerns — relevant: "Phase 7 host
  integration test harness must be set up at Phase 7 start" (Phase 6
  Playwright smoke is the early establishment of that harness per D6-30).

### Phase 2 UI sketches (LOCKED — visual contract for Phase 6 screens)
- `.planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md` —
  Four ASCII sketches with API-shape implications:
  - **Sketch 1: Agent detail/edit** — N:M skills embedded on detail,
    PATCH replaces skills set, 409 version_conflict UX. Drives D6-16.
  - **Sketch 2: Bulk import result** — Entity picker, file upload, 207
    `BulkImportResult` rendering with `failed[]` table, 413 oversized
    message. Drives `<or-import-page>`/`<or-import-result>` shapes.
  - **Sketch 3: Agent status panel** — Per-state transitions,
    `engaged_channel` display, `wrapup_until` countdown,
    `post_interaction_state` radio, 409 invalid_transition UX. Drives
    `<or-status-panel>` shape + D6-04 status banner.
  - **Sketch 4: Catalog list (generic)** — Cursor pagination,
    include_disabled toggle, name search, row context menu. Drives
    `<or-data-table>` + `<or-cursor-paginator>` primitives.

### Phase 1 carry-forward (LOCKED)
- `.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md` —
  D-01..D-31 locked. Especially:
  - **D-19, D-20:** UUIDv7 everywhere. Admin client-side UUIDv7 regex
    must match the same `[0-7][0-9a-f]{3}` version byte the server
    enforces.
  - **D-21:** Middleware bypass list `{/healthz, /readyz, /metrics,
    /openapi.yaml, /docs}` — admin Vite proxy must list `/v1` + the
    bypass routes it actually calls; `getOrgId` is harmless on bypass
    because they're not in the openapi-typescript `paths` map.
  - **D-25:** `task dev` air hot-reload runs Go API natively on
    `localhost:8080`. Admin Vite proxy targets this.
  - **FOUND-09:** CI shape — `frontend-typecheck-lint-unit` job already
    exists; Phase 6 D6-31 extends it.

### Phase 2 carry-forward (LOCKED contract — admin client consumes it)
- `.planning/phases/02-openapi-contract-codegen/02-CONTEXT.md` —
  D-32..D-48. Especially:
  - **D-35, D-36:** Canonical error envelope `{error, reason,
    request_id}`. Closed `ErrorCode` enum. Admin maps error→i18n key
    per D6-24; request_id always visible.
  - **D-37:** Special-case 4xx response shapes (`current` on 409
    version_conflict, `from/to` on 409 invalid_transition). Admin
    consumes these directly per D6-03/D6-04.
  - **D-38:** `createApiClient({ baseURL, getOrgId, fetch? })` factory.
    Admin instantiates one client at boot from URL-path org_id (D6-09);
    Phase 7 embed will instantiate per-element with `getOrgId` reading
    the host attribute.
  - **D-39:** `ApiError`, `ErrorCodes`, `isApiError`, `parseApiError`
    in `packages/ui/src/api/errors.ts`. Admin reuses; ajv-codegen
    validators (D6-18) extend the same module.
  - **D-40:** Barrel `web/packages/ui/src/index.ts` re-exports the API
    module. Phase 6 EXTENDS the barrel with component exports +
    validators (`export * from './components';` and
    `export * from './validators';`).

### Phase 3 carry-forward (LOCKED — admin renders these entity shapes)
- `.planning/phases/03-catalog-crud-go/03-CONTEXT.md` — D-49..D-77 plus
  A6/A7 amendments. Drives entity-aware admin behaviour:
  - Cursor pagination opaque base64, `next_cursor` + `has_more`.
    Drives `<or-cursor-paginator>`.
  - `include_disabled` filter default false. Drives list-page query
    state.
  - Case-insensitive substring `name` search (resolved in Phase 2
    OQ-4B → ILIKE). Drives list-page search input.
  - Soft-delete = `enabled=false` via UPDATE; row-context-menu
    [Disable] vs [Delete] map to PATCH `enabled=false` vs DELETE.
  - 409 `version_conflict` body carries `current: <Entity>` (D-37).
    Drives D6-03/D6-04 inline banner.

### Phase 04.1 carry-forward (LOCKED — universal `code` identifier)
- `/Users/luong/workspace/dev/open-solutions/.claude/worktrees/frosty-khayyam-9c3d54/.planning/phases/04.1-catalog-identity-normalization/04.1-CONTEXT.md`
  (lives in Phase 5 worktree until merged; treat as authoritative for
  Phase 6 planning) — D04_1-01..26. Drives admin UI rendering of `code`:
  - **D04_1-01:** `code` is the universal user-facing identifier on all
    6 entities. Composite `UNIQUE (org_id, code)`. Admin list columns
    show `code` prominently (Sketch 4 still says `external_id` — admin
    UI should show both columns, `code` primary).
  - **D04_1-02:** `code` is IMMUTABLE post-create. Update form must
    render `code` as read-only with a tooltip "Code cannot be changed.
    Use the rename endpoint (v0.2) or recreate."
  - **D04_1-03:** Regex `^[a-z][a-z0-9_]{0,63}$`. `<or-code-input>`
    enforces same regex client-side; PATTERN attribute + setCustomValidity.
  - **D04_1-16:** 409 `duplicate_code` distinct from 409
    `duplicate_external_id`; admin maps both → i18n keys per D6-24.
  - **D04_1-20:** 422 `immutable_field` for PATCH `code` mutation
    attempts. Admin form prevents this client-side; defensive error
    handling still required for direct API users.

### Phase 4 carry-forward (LOCKED — agent status panel consumes this)
- `.planning/phases/04-agent-state-machine-go/04-CONTEXT.md` —
  D-78..D-95. Drives `<or-status-panel>`:
  - **D-84:** `force: bool` field on `PatchAgentStatusRequest`. Admin
    UI surfaces force toggle (single checkbox "Force transition — admin
    override" with red border emphasis) for operational recovery; cross-
    org break_reason still rejected. Logged WARN at server. v0.1 stub
    auth: every admin sees the toggle; v1 AUTH phase restricts to
    `org_admin` role.
  - **D-85:** No `expected_state_version` on PATCH; concurrent agent
    PATCHes surface as transition-matrix 409. Admin handles via D6-04
    status banner.
  - **D-87:** `wrapup_until` includes ±100ms jitter. Admin countdown
    display rounds to seconds (no jitter visible to admin).
  - **D-91:** Transition matrix encoded as `map[AgentStatus]
    map[AgentStatus]TransitionRule`. Admin transition buttons should
    mirror the matrix (show only allowed transitions per current
    status); spec amendment D-92 added the field, openapi.yaml gives
    the enum.

### Phase 5 carry-forward (LOCKED — bulk import UI consumes this)
- `/Users/luong/workspace/dev/open-solutions/.claude/worktrees/frosty-khayyam-9c3d54/.planning/phases/05-bulk-import-go/05-CONTEXT.md`
  (lives in Phase 5 worktree until merged) — D5-01..D5-27. Drives
  `<or-import-page>` + `<or-import-result>`:
  - **D5-04:** Multi-value separator priority `|` > `;` > `,`.
    `<or-import-page>` CSV format helper must document this; provides
    a "Show example" disclosure.
  - **D5-15..D5-17:** Import schemas reference FKs by `code`. CSV agent
    skills syntax `code:proficiency|code:proficiency`. Format helper
    must show this exact syntax with examples (`skill_voice:7|skill_chat:9`).
  - **D5-18:** Skill merge semantics on agent update via import
    (PATCH-like, not PUT). Admin UI copy: "Importing existing agents
    MERGES skills — to remove a skill, use the agent detail edit page."
  - **D5-13:** Optional `Idempotency-Key` header (client UUIDv7).
    Admin import page can optionally generate one (`<sl-checkbox>`
    "Make this import retry-safe"); v0.1 known limitation: replay
    response has `succeeded: []` per D5-13 (admin UI surfaces
    `idempotent_replay: true` field with helpful message).
  - **D5-22:** 500-row cap fail-fast at row 501 (server-side). Admin
    UI shows file row count before upload via a Web Worker? Out of
    scope per PROJECT.md (browser workers deferred); v0.1 admin lets
    the server fail and renders the 413 message.
  - **D5-25:** `Import*Request` schemas (oneOf concern noted by Phase
    5 research). Admin sends `?entity=` query + body matching the
    request schema for that entity; ajv validators (D6-18) cover this.

### External standards (READ for Lit/Shoelace/Vite/ajv idioms)
- [Lit 3 documentation](https://lit.dev/docs/) — ReactiveElement, decorators
  (Vite + TS works without legacy decorators), template syntax.
- [`@lit-labs/router`](https://github.com/lit/lit/tree/main/packages/labs/router)
  — `URLPattern`-based routing, `outlet()` template.
- [`@lit/localize`](https://lit.dev/docs/localization/overview/) —
  `msg()` API, lit-localize CLI, runtime mode (`init` style).
- [`@lit/task`](https://lit.dev/docs/data/task/) — already in
  `packages/ui` deps; manages async fetch state via reactive args.
- [Shoelace components](https://shoelace.style/) — `<sl-button>`,
  `<sl-input>`, `<sl-select>`, `<sl-drawer>`, `<sl-alert>` (for the
  conflict banner per D6-04), `<sl-button-group>` (for theme toggle).
- [Shoelace theming](https://shoelace.style/getting-started/themes/) —
  CSS custom property convention `--sl-color-*`, dark theme file pattern.
- [Vite SSR + dev proxy config](https://vitejs.dev/config/server-options.html#server-proxy).
- [`ajv` standalone compilation](https://ajv.js.org/standalone.html) —
  `ajv-cli --code` workflow for OpenAPI-driven validators.
- [`openapi-typescript`](https://openapi-ts.dev/) — already in
  `packages/ui` deps for the typed fetch client.
- [`openapi-fetch` middleware](https://openapi-ts.dev/openapi-fetch/middleware-auth/)
  — already used in `createApiClient` for `X-Org-Id` header (Phase 2).
- [happy-dom](https://github.com/capricorn86/happy-dom) — Vitest
  environment for fast Web Component tests.
- [Playwright](https://playwright.dev/) — smoke test in Phase 6;
  full matrix in Phase 7.
- [MDN: `visibilitychange` event](https://developer.mozilla.org/en-US/docs/Web/API/Document/visibilitychange_event)
  — drives D6-26 polling pause/resume.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (already in `web/packages/ui/` from Phase 2)
- **`createApiClient(config)`** at `web/packages/ui/src/api/client.ts` —
  factory taking `{ baseURL, getOrgId, fetch? }`; emits `X-Org-Id`
  header per request via openapi-fetch Middleware. Admin instantiates
  one client at app boot, where `getOrgId` reads from the URL-path
  org_id captured by the router (D6-09).
- **`ApiError`, `ErrorCodes`, `isApiError`, `parseApiError`** at
  `web/packages/ui/src/api/errors.ts` — closed enum + type guards.
  Admin imports `ErrorCodes` into the i18n error-key map (D6-24).
  ajv-codegen validators (D6-18) extend this module with the per-schema
  `validate{Schema}` functions emitted alongside.
- **`createApiTask(host, { task, args })`** at
  `web/packages/ui/src/api/task.ts` — wraps `@lit/task` Task with
  passthrough. List components use this with args `[orgId, search,
  cursor, includeDisabled]` so re-runs auto-fire on filter change.
  Status panel polling task uses this with periodic re-runs (D6-26).
- **Generated `paths`, `components`, `operations`** at
  `web/packages/ui/src/api/generated.ts` (from `pnpm gen:api`) — types
  for every request/response shape. Admin imports
  `components['schemas']['Agent']` etc. directly for form state.

### Established Patterns
- **Per-component file co-location for tests** (mirrors backend D-72) —
  `agent-list.test.ts` beside `agent-list.ts`. Vitest + happy-dom
  (D6-28).
- **Barrel re-export** (Phase 2 D-40) — `web/packages/ui/src/index.ts`
  re-exports everything consumed by admin/embed. Phase 6 extends with
  `export * from './components';` and `export * from './validators';`.
- **openapi-fetch per-request middleware** (Phase 2 D-38) — `X-Org-Id`
  injection example. Same Middleware pattern works for future locale
  header / authentication tokens.
- **`pnpm gen:api` Turbo task** in `web/turbo.json` — Phase 6 adds
  `gen:validators` alongside; same drift-gate semantics (D6-31).

### Integration Points
- **Phase 1 Go API (`localhost:8080`)** — Vite dev server proxies `/v1`
  to it (D6-29). `task dev` air-reload + `pnpm dev` Vite-reload run
  side-by-side; admin developers run both terminals.
- **Phase 2 OpenAPI spec** (`openapi/openapi.yaml`) — source of truth
  for TS types (`gen:api`) + ajv validators (`gen:validators`). Drift
  gate in CI prevents committed types/validators from diverging from
  the spec.
- **Phase 3 catalog entities** — admin renders 6 entity CRUD against
  these endpoints. Phase 4's `agent_states` row pre-exists for every
  agent (created atomically per D-93); admin status panel always finds
  a row to render.
- **Phase 4 agent state machine** — `<or-status-panel>` is the UI
  surface for the transition matrix (D-91). Admin force flag (D-84)
  surfaces here.
- **Phase 04.1 + Phase 5 (pending merge from `frosty-khayyam-9c3d54`
  worktree)** — Phase 6 plan must wait for these to land on `main`,
  OR plan against the openapi.yaml at the merge tip. The 6 `Import*Request`
  schemas (D5-25) and the `code` field on all entities (D04_1-13) are
  in that worktree. Researcher / planner: cross-reference `openapi.yaml`
  contents at planning time, not at this CONTEXT.md authoring time.
- **Phase 7 embed bundle** — DOWNSTREAM consumer of `packages/ui`. Phase
  6 design decisions (D6-05 hybrid layer, D6-08 per-component Shoelace
  imports, D6-20 theme-on-shell-host) are explicitly shaped by Phase 7's
  70KB gzipped bundle target and theme/modules attribute contract.

### Phase 6 hits clean-slate code (`web/apps/admin/` + new `packages/ui` components)
- `web/apps/admin/src/index.ts` is currently `export {};` — empty
  scaffold from Phase 1 D-12. Phase 6 fills this in.
- `web/apps/admin/package.json` declares `typecheck` + `lint` only;
  Phase 6 adds `dev`, `build`, `preview`, `test`, `test:e2e` scripts
  + Vite + Lit + Shoelace + @lit-labs/router + @lit/localize +
  happy-dom + Playwright deps.
- `web/packages/ui/package.json` declares `typecheck` + `lint` + `test`
  + `gen:api`; Phase 6 adds `gen:validators` (ajv-cli) and Shoelace
  per-component deps.
- No `apps/admin/src/components/`, no `packages/ui/src/components/`,
  no `packages/ui/src/validators/` yet — fresh build per CONTEXT.md.

</code_context>

<specifics>
## Specific Ideas

- **"Wizard cho chuyên nghiệp"** — user explicitly asked for a multi-step
  wizard create flow (Area 4, Create pattern). Implemented as a single
  `<or-form-wizard>` component used by all 6 entities; complex entities
  (Agent, Channel) use multiple steps, simple ones use a single review
  step. Drove D6-17.
- **"Tư duy humanable" (carried from Phase 04.1)** — admin UI surfaces
  `code` prominently and treats it as the human-friendly identifier;
  UUIDs shown only when necessary (e.g., truncated in row context).
  Form validators enforce the same regex as the server (D6-18 +
  D04_1-03 alignment).
- **"State cũng quan trọng — phải load ngay trạng thái chuẩn"** — user
  reaction during 409 discussion. Drove D6-03: rather than separate
  reload-prompt UX, both 409s consume the truth from the response body
  in-place (`current` for CRUD, `from`/`to` for status). No extra GET
  round-trip needed.
- **"Đang dùng Lit thì tại sao gợi ý vaadin/router?"** — user pushed
  back on the initial vaadin/router recommendation. Switched to
  `@lit-labs/router` as the Lit-native option (D6-07). Labs status
  acknowledged as a small risk; 8 routes in v0.1 makes migration cheap
  if the API shifts.
- **EN + VI bilingual UI from v0.1** — user is Vietnamese-speaking
  and asked for Vietnamese explanations multiple times during the
  discussion. Decided to ship VI alongside EN from day one rather than
  English-first then translate later. @lit/localize handles extraction
  + per-locale compile.
- **Theme-on-shell-host pattern** — required Vietnamese explanation to
  surface the underlying reason (Shadow DOM CSS isolation in Phase 7
  blocks `document.documentElement` cascade). Once explained, decision
  was clear (D6-20).
- **CI E2E deferred to Phase 7** — user explicitly chose "chỉ thêm
  codegen drift + build" (D6-31), keeping Phase 6 CI lean. Playwright
  is built (smoke test exists) and developers run it locally; promotion
  to CI happens with Phase 7's EMBED-10 multi-host matrix.

</specifics>

<deferred>
## Deferred Ideas

These came up during discussion (or were considered) but explicitly pushed
out of Phase 6 scope.

- **Inline org dropdown / multi-org switcher** — D6-14 locks the
  hard-reset-to-org-picker pattern. v0.2 / v1 can introduce a recent-orgs
  `<sl-select>` once real auth gives a notion of "orgs you have access to".
- **Module Federation / iframe embed variants** — PROJECT.md scope.
  v1+ only if customer demand justifies; Phase 6 / 7 only ship Web
  Components.
- **Server-side rendering / SSR** — Vite SPA is client-only in v0.1.
  Useful for embed-in-marketing-page scenarios; deferred.
- **Source-map upload to error tracker (Sentry / Bugsnag)** — frontend
  errors → console + request_id display in v0.1. Centralized error
  reporting deferred until ops adopts a tracker.
- **Accessibility audit / screen reader certification** — Shoelace
  defaults give a reasonable baseline; explicit a11y testing (axe-core,
  manual screen reader passes) deferred.
- **Dark-mode auto-detect via `prefers-color-scheme`** — D6-19 ships
  three named themes with an explicit toggle. Auto-detect from system
  preference deferred; can be added as a fourth option `or-auto` later.
- **Brand theme customization per org** — v0.1 ships a hard-coded
  `or-brand` palette. Per-org theme overrides (driven by an `orgs.theme`
  column?) deferred to v0.2 with the broader theming story.
- **Server-translated error messages** — D6-24 does client-side
  ErrorCode → i18n key. v0.2 could add `Accept-Language` server-side
  if reasons need translation.
- **Inline-edit list rows** — for high-volume edits (e.g., adjusting
  20 break_reasons display_orders). Phase 6 ships full-detail-page
  edit only. v0.2 can add an "Edit in place" affordance if admin
  feedback requests it.
- **Bulk delete / bulk enable-disable** — list pages don't have
  multi-select in Phase 6. v0.2 if needed.
- **CSV preview-before-upload via Web Worker** — WORK-02 deferred per
  PROJECT.md. v0.1 admin just uploads; server validates.
- **Error CSV download (IMP-11)** — Phase 5 deferral. Admin import
  result page in Phase 6 shows `failed[]` table on screen; download
  CSV button is v0.2.
- **Stale-while-revalidate** — D6-27 locks always-refetch-on-mount.
  SWR can be added in v0.2 if Redis cache pressure becomes a concern.
- **Periodic catalog list polling** — D6-25 locks refresh-on-action
  only. Periodic polling deferred (or never; SSE in v1 supersedes).
- **In-page audit log / change history** — no surface in v0.1.
- **Inline JSON editor for adapter `config` field** — Adapter entity's
  free-form JSONB `config` (CAT-06). Phase 6 ships a plain `<sl-textarea>`
  for raw JSON entry with ajv `validate JSON` check on submit. Rich
  CodeMirror-like editor deferred to v0.2.
- **Real RBAC for `force=true` flag** (Phase 4 D-84 carry-forward) —
  v0.1 stub auth surfaces the toggle to every admin; logged WARN at
  server. v1 AUTH phase restricts to `org_admin` role and surfaces a
  permission error.
- **Optional embed `<or-catalog-shell>` consumption from Phase 7** —
  Phase 7 chooses at planning time between mounting the shell (heaviest
  reuse) and composing entity components directly (leanest bundle).
  Phase 6 designs for both; no Phase 6 commitment.

### Reviewed Todos (not folded)
None — STATE.md's lone pending todo ("Phase 3 simplification cleanup")
belongs in Phase 3 follow-up, not Phase 6.

</deferred>

---

*Phase: 6-Shared UI Library & Standalone Admin*
*Context gathered: 2026-05-17*
*Next: `/gsd-plan-phase 6` (research → plan → verify). Researcher should
verify openapi.yaml on `main` includes Phase 04.1 (`code` field) and
Phase 5 (`Import*Request` schemas) before producing RESEARCH.md.*
