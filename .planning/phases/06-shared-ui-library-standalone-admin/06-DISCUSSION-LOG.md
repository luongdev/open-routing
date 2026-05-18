# Phase 6: Shared UI Library & Standalone Admin - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-17
**Phase:** 6-Shared UI Library & Standalone Admin
**Areas discussed:** Scope boundary; packages/ui reuse boundary; Routing + auth bootstrap; CRUD form + 409 + theme; i18n / locale; Polling + freshness; Testing + dev experience

---

## Scope boundary

### Q1: What ships in Phase 6 — strict ADMIN-01..06 or full admin (incl. status panel + import UI)?

| Option | Description | Selected |
|--------|-------------|----------|
| CRUD + Status + Import | Full admin: 6 entity CRUD + agent status panel (sketch 3) + bulk import UI (sketch 2). Matches Phase 5's downstream expectation; uses existing sketches. | ✓ |
| CRUD only (literal ADMIN-*) | Strict ADMIN-01..06 scope. Defer status panel + import UI to later phase. | |
| CRUD + Status (defer import UI) | 6 CRUD + status panel; defer bulk import UI. Admins curl import endpoint in v0.1. | |

**User's choice:** CRUD + Status + Import
**Notes:** Phase 5 CONTEXT explicitly says "Phase 6 admin will add a bulk-import screen" (D5-17/D5-18); user accepted that downstream expectation.

### Q2: How to split the work across plans/waves?

| Option | Description | Selected |
|--------|-------------|----------|
| Wave-based by layer | Wave 1 scaffold; Wave 2 shared framework + agents exemplar; Waves 3-4 parallel entities; Wave 5 status + import. Mirrors Phase 3. | ✓ |
| Vertical slices per entity | Each entity end-to-end before moving on. Simpler graph but no early shared abstractions. | |
| You decide | Planner picks split. | |

**User's choice:** Wave-based by layer

### Q3: Should CRUD 409 and status 409 share one UX or distinguish?

| Option | Description | Selected |
|--------|-------------|----------|
| Distinguish: only CRUD uses reload-prompt | CRUD = 'Changed elsewhere, reload?'; status = inline transition error. | (reframed) |
| Unified single UX | Both 409s show same affordance. Loses semantic nuance. | |
| Skip status 409 handling | Status 409 renders raw envelope; polish in Phase 7+. | |

**User's choice:** "Chưa rõ lắm. Trạng thái cũng quan trọng lắm đấy, nếu đổi lỗi conflict, phải load ngay trạng thái chuẩn về cho user chứ?"
**Notes:** Reframed: both 409 bodies already carry server truth (`current` for CRUD per D-37; `from`/`to` for status per D-91). No extra GET needed — consume body, update local state directly. Distinguish at the banner copy level, not the mechanism level.

### Q4: What does the user see after the 409 body is consumed?

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-update + inline banner | Apply server truth immediately; show inline banner with diff. | ✓ |
| Modal blocker | Modal forces acknowledgment. More disruptive. | |
| Toast notification only | Lightweight toast; easy to miss. | |

**User's choice:** Auto-update + inline banner

---

## packages/ui reuse boundary

### Q1: Page-level shell, entity components, or generic primitives?

| Option | Description | Selected |
|--------|-------------|----------|
| Page-level shells | <or-catalog-shell> with everything; admin/embed render one tag. Maximum reuse, locks UX opinions. | |
| Entity components only | Per-entity components; each app owns router/nav. | |
| Generic primitives only | Just primitives; entity screens written per app. | |
| Hybrid | Entity components + optional <or-catalog-shell>. Best of both. | ✓ |

**User's choice:** "Chưa hiểu lắm? Tư vấn thêm xem nào" → after explanation: Hybrid
**Notes:** Required a longer concrete walk-through of each option. Hybrid chosen because admin can use the shell while Phase 7 keeps the freedom to compose entity components directly to hit the 70KB budget.

### Q2: Where do router + nav dependencies live?

| Option | Description | Selected |
|--------|-------------|----------|
| Router/nav inside the shell | packages/ui owns lightweight router + nav. Apps that mount shell get chrome free. | ✓ |
| Router as peer dep + opt-in shell | Apps install router; lean default. | |
| Shell lives in apps/admin (not packages/ui) | apps/admin and apps/embed each define their own shell. | |

**User's choice:** Router/nav inside the shell

### Q3: Which router library?

| Option | Description | Selected |
|--------|-------------|----------|
| @vaadin/router | ~7KB, mature, decoupled from Lit. | |
| @lit-labs/router | ~5KB, Lit-native. | ✓ |
| Hand-rolled minimal | ~1KB custom; reinvents browser-back edge cases. | |

**User's choice:** "Đang dùng lit thì tại sao lại gợi ý cái vaadin/router thế? Nó có gì hay à?" → after explanation: @lit-labs/router
**Notes:** User pushed back on the initial @vaadin/router recommendation. Switched to @lit-labs/router for Lit-stack purity; Labs status acknowledged.

### Q4: How does packages/ui register Shoelace components?

| Option | Description | Selected |
|--------|-------------|----------|
| Each component registers its own | Per-file imports of needed `<sl-*>` components. Tree-shaking works. | ✓ |
| packages/ui central register | Shared setup function; pulls all Shoelace components — breaks tree-shaking. | |
| Shoelace autoload | Incompatible with Shadow DOM. Reject. | |

**User's choice:** Each component registers its own

---

## Routing + auth bootstrap

### Q1: How does admin supply org_id?

| Option | Description | Selected |
|--------|-------------|----------|
| URL path | Routes `/orgs/:org_id/...`; bookmarkable; multi-tab multi-org. | ✓ |
| Stub login screen | Form at `/login`; org_id in app state; routes without org. | |
| Both | Stub login then URL. | |

**User's choice:** URL path

### Q2: Behavior at `/` (root)?

| Option | Description | Selected |
|--------|-------------|----------|
| Org-picker fallback | `/` shows UUIDv7 input; last-used pre-filled from localStorage. | ✓ |
| Dev-only default org_id | Hardcoded dev org redirect. | |
| Configurable VITE_DEFAULT_ORG_ID env | Build-time default. | |

**User's choice:** Org-picker fallback

### Q3: Nav structure?

| Option | Description | Selected |
|--------|-------------|----------|
| Sidebar with 8 entries | 6 catalog + Bulk Import + Agent Status entry-point. | ✓ |
| Top tabs | Horizontal tabs; breaks at 8+. | |
| Sidebar grouped | Grouped 'Catalog' + 'Operations'. | |

**User's choice:** Sidebar with 8 entries

### Q4: Status panel + import URL shape?

| Option | Description | Selected |
|--------|-------------|----------|
| Mixed | Status nested under agent; import top-level. Matches OpenAPI shape. | ✓ |
| Top-level for both | Flat structure. Easier sidebar wiring; loses URL context. | |
| Status nested, import top-level without /new | Single import page handles both upload and result via state. | |

**User's choice:** Mixed

### Q5: org_id validation timing — client, server, or both?

| Option | Description | Selected |
|--------|-------------|----------|
| Pre-validate UUIDv7 client-side | Regex check before API call. | |
| Let API reject | Server returns 400/invalid_org_id. | |
| Both | Client UX nicety + server source of truth. | ✓ |

**User's choice:** Both

### Q6: Org-switcher behavior?

| Option | Description | Selected |
|--------|-------------|----------|
| Hard reset to org-picker | Navigate to `/`; in-memory state cleared. | ✓ |
| URL change triggers reset | Router invalidates and refetches; no prompt. | |
| Inline org dropdown | `<sl-select>` recent orgs; one-click switch. | |

**User's choice:** Hard reset to org-picker

### Q7: 404 deep-link behavior?

| Option | Description | Selected |
|--------|-------------|----------|
| Inline empty-state per page | Each page owns its 404 UI. | ✓ |
| Dedicated /404 route | Centralized UI; loses context. | |
| Modal + back-redirect | Auto-navigate after 2s; intrusive. | |

**User's choice:** Inline empty-state per page

---

## CRUD form + 409 + theme

### Q1: Edit pattern — modal, drawer, or full detail page?

| Option | Description | Selected |
|--------|-------------|----------|
| Full detail page | Row click navigates to `/orgs/:org_id/{entity}/:id`. Matches Sketch 1. | ✓ |
| Modal overlay | List context preserved; cramped for complex forms. | |
| Slide-over drawer | Compromise; bookmarkable via query param. | |

**User's choice:** Full detail page

### Q2: Create flow — dedicated /new, modal, or inline?

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated /new route | `/orgs/:org_id/{entity}/new`. Same detail-page component in create mode. | ✓ (with wizard) |
| Modal on top of list | List preserved; small create form only. | |
| Inline at top of list | Quick add for simple entities. | |

**User's choice:** "Option 1, nhưng nên làm dạng wizard cho chuyên nghiệp"
**Notes:** User wants a wizard pattern for create. Refined in Q3.

### Q3: Wizard scope — all entities, complex only, or just Agent?

| Option | Description | Selected |
|--------|-------------|----------|
| Wizard only for complex entities | Multi-step for Agent + Channel; single-step for others; same component. | ✓ |
| Wizard for all 6 entities | Every Create has 2+ steps. Most consistent; adds clicks for simple cases. | |
| Wizard only for Agent | Only Agent Create is multi-step; others use detail-page form. | |

**User's choice:** Wizard only for complex entities

### Q4: Form validation source?

| Option | Description | Selected |
|--------|-------------|----------|
| HTML5 Constraint Validation API | Native `pattern`, `min`, `max`, `required`. Zero dep. | |
| Codegen validator from OpenAPI | Single source of truth; ajv standalone. | ✓ |
| Hand-written Zod schemas | Familiar DX; ~6KB; drift risk. | |

**User's choice:** Codegen validator from OpenAPI

### Q5: Codegen approach for OpenAPI validators?

| Option | Description | Selected |
|--------|-------------|----------|
| ajv + json-schema-ref-parser | Build-time script; ajv-cli --code. Tree-shakeable. | ✓ |
| Custom Vite plugin | Hand-written walker; tighter control. | |
| openapi-fetch built-in validation | Middleware-level; doesn't cover per-keystroke. | |

**User's choice:** ajv + json-schema-ref-parser

### Q6: Theme scope?

| Option | Description | Selected |
|--------|-------------|----------|
| Light + dark + brand variant | Three themes; toggle persists in localStorage. | ✓ |
| Shoelace defaults only | Built-in light + dark. Cheapest. | |
| Full custom token system | Complete OR token set; largest design effort. | |

**User's choice:** Light + dark + 1 brand variant

### Q7: Theme injection point?

| Option | Description | Selected |
|--------|-------------|----------|
| Apply on <or-catalog-shell> host | CSS vars on shell host element; cascades to children. Works for admin (string) and Phase 7 embed (JSON attribute). | ✓ |
| Apply on document.documentElement | Class on `<html>`. Shadow DOM blocks cascade in Phase 7. Forks. | |
| Lit context provider | Idiomatic; verbose. | |

**User's choice:** "Chưa rõ lợi ích lắm, tư vấn đi. Nói tiếng Việt cho hiểu rõ" → after Vietnamese explanation: Apply on <or-catalog-shell> host
**Notes:** Required Vietnamese explanation that Shadow DOM CSS isolation (EMBED-04) blocks `document.documentElement` cascade in Phase 7. Once explained, decision was clear.

---

## i18n / locale

### Q1: Language scope for v0.1?

| Option | Description | Selected |
|--------|-------------|----------|
| English-only v0.1 | Ship EN; extract to keys for easy translation later. | |
| Song ngữ EN + VI (toggle) | Both string sets ship; toggle in top bar. | ✓ |
| VI-only v0.1 | Vietnamese first; English deferred. | |

**User's choice:** Song ngữ EN + VI (toggle)

### Q2: i18n library?

| Option | Description | Selected |
|--------|-------------|----------|
| @lit/localize | Official Lit team; XLIFF workflow; ~3KB; type-safe. | ✓ |
| Simple key-value map | Hand-rolled JSON file; no pluralisation. | |
| Format.js (react-intl-style) | ICU MessageFormat; ~10KB; React-flavoured. | |

**User's choice:** @lit/localize

### Q3: Default locale + persistence?

| Option | Description | Selected |
|--------|-------------|----------|
| navigator.language detect + localStorage override | Auto-detect first; toggle persists choice. | ✓ |
| Mặc định EN | Always start English; user toggles. | |
| Mặc định VI | Always start VI; English users toggle. | |

**User's choice:** navigator.language detect + localStorage override

### Q4: Server error message handling?

| Option | Description | Selected |
|--------|-------------|----------|
| ErrorCode → client-side translation | Map closed ErrorCode enum to i18n keys; show translated message; `reason` in collapsed details. | ✓ |
| Display raw reason | Show server English; simplest but worst UX for VI users. | |
| Server-translated via Accept-Language | Spec change; out of v0.1 scope. | |

**User's choice:** ErrorCode → client-side translation

---

## Polling + freshness

### Q1: Catalog list polling?

| Option | Description | Selected |
|--------|-------------|----------|
| Refresh-on-action only | Mount + user-action fetches; explicit [Refresh] button. | ✓ |
| Poll 30s when visible | Periodic refresh; pause when hidden. | |
| Stale-while-revalidate | Show cached, fetch background. | |

**User's choice:** Refresh-on-action only

### Q2: Agent status panel polling when tab hidden?

| Option | Description | Selected |
|--------|-------------|----------|
| Pause when hidden | visibilitychange listener; resume on visible. | |
| Always poll | 5s regardless of visibility. | |
| Pause + state_version check on resume | Pause; on resume fire immediate GET and compare state_version. | ✓ |

**User's choice:** Pause khi hidden + kiểm tra state_version khi visible

### Q3: Client-side cache strategy?

| Option | Description | Selected |
|--------|-------------|----------|
| Always refetch on mount | Default @lit/task behavior. Single source of truth. | ✓ |
| Cache 30s in-memory | Module-level Map with TTL; invalidate on edit. | |
| Cache in sessionStorage | Persist across reload; stale risk. | |

**User's choice:** Always refetch on mount

---

## Testing + dev experience

### Q1: Component test runner environment?

| Option | Description | Selected |
|--------|-------------|----------|
| Vitest + happy-dom | Fast; handles modern Web Component APIs. | ✓ |
| Vitest + jsdom | Mature; slower; some Shadow DOM edge cases. | |
| @web/test-runner (real browser) | High fidelity; complex setup; slow. | |

**User's choice:** Vitest + happy-dom

### Q2: Vite dev server config?

| Option | Description | Selected |
|--------|-------------|----------|
| Vite proxy `/v1` → localhost:8080 | Same-origin; no CORS in dev. | ✓ |
| CORS dev allowance | Go server enables CORS; full URL fetch. | |
| Sub-path deploy match prod | Build admin to `/admin/*`; proxy `/v1`. | |

**User's choice:** Vite proxy `/v1` → localhost:8080

### Q3: E2E tests for Phase 6?

| Option | Description | Selected |
|--------|-------------|----------|
| Playwright smoke in Phase 6 | Single test; reuses harness for Phase 7. | ✓ |
| Defer all E2E to Phase 7 | Vitest component tests only in Phase 6. | |
| Manual UAT only | Component tests + manual demo. | |

**User's choice:** Bắt đầu Playwright smoke trong Phase 6

### Q4: CI pipeline additions for Phase 6?

| Option | Description | Selected |
|--------|-------------|----------|
| Add Playwright smoke + ajv codegen check | Full pipeline including E2E in CI. | |
| Codegen drift + production build only | Drift gate + Vite build smoke; E2E manual. | ✓ |
| No CI changes | Rely on existing FOUND-09 typecheck/lint/unit. | |

**User's choice:** Chỉ thêm codegen drift + build

---

## Claude's Discretion

Items where Claude has flexibility (deferred to planner):
- Exact internal directory layout under `web/packages/ui/src/components/`
- Sidebar visual design / icon choice + brand-theme primary color value
- ajv codegen wrapper script (Node script vs Vite plugin)
- @lit/localize XLIFF directory layout
- `<or-form-wizard>` step-state persistence shape
- Theme-toggle UX shape (`<sl-button-group>` vs cycle button vs `<sl-select>`)
- Org-picker validation timing (debounce vs on-blur vs on-submit)
- Whether `<or-conflict-banner>` includes a "discard local edits" button

## Deferred Ideas

Listed in CONTEXT.md `<deferred>` section. Highlights:
- Inline org dropdown / multi-org switcher (v0.2)
- SSR (deferred)
- Source-map upload to error tracker (deferred)
- Accessibility audit / screen reader certification (deferred)
- `prefers-color-scheme` auto-detect for themes
- Per-org brand theme customization (v0.2)
- Server-translated error messages
- Inline-edit list rows
- Bulk delete / bulk enable-disable
- CSV preview-before-upload via Web Worker (WORK-02 — PROJECT.md deferral)
- Error CSV download (IMP-11 — Phase 5 deferral)
- Stale-while-revalidate
- Periodic catalog list polling
- In-page audit log / change history
- Rich CodeMirror-like JSON editor for Adapter config field
- Real RBAC for force=true flag (Phase 4 D-84 carry-forward)
