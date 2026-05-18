# Phase 7: Web Component Embed Bundle - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-18
**Phase:** 7-Web Component Embed Bundle
**Areas discussed:** Reuse strategy, Shadow DOM routing, CI infra (bundle gate + Playwright), Embed public contract

---

## Reuse strategy

### Question 1: How should `<open-routing-catalog>` mount the catalog UI inside Shadow DOM?

| Option | Description | Selected |
|--------|-------------|----------|
| Hybrid — shell + lazy routes | Mount `<or-catalog-shell>` but convert entity routes from static to dynamic `import()`. Baseline ~25 KB; entity chunks load on demand. ~50 lines + Vite manualChunks. Stays one shell for admin + embed. | ✓ |
| Mount shell as-is | Wrap shell verbatim. Zero refactor. Bundle ~110 KB before gzip; relies on Shoelace+Lit pre-gzip ratio to squeak under 70 KB. Blows budget on next entity. | |
| Compose entities directly | Embed implements its own thin top-level routing. Imports only entities in `modules=`. Diverges from admin UX; biggest maintenance cost. | |

**User's choice:** Hybrid — shell + lazy routes
**Notes:** Phase 6 D6-05 left this open for Phase 7 planning; hybrid is the only path that single-source-of-truth shell AND hits 70 KB budget.

### Question 2: What stays eager (always in the baseline bundle) vs lazy-loaded per route?

| Option | Description | Selected |
|--------|-------------|----------|
| Eager: shell + router + theme + primitives + auth-banner; Lazy: 6 entities + status + import | Baseline = chrome user always sees (~25 KB gzipped budget). 8 lazy chunks. Matches admin UX after first paint. | ✓ |
| Eager: shell + router + theme only; Lazy: everything else | Smallest baseline (~15 KB). Risk: primitives reload-fetch on first list render. More waterfall. | |
| Predictive: shell + router + theme + primitives + first entity from `modules`; Lazy: rest | Clever but fragile; module order isn't a UX promise. | |

**User's choice:** Eager baseline includes primitives + auth-banner; entity-level lazy split.

### Question 3: Shell currently statically imports all entity components in `catalog-shell.ts` — refactor in place, or introduce a shell variant?

| Option | Description | Selected |
|--------|-------------|----------|
| Refactor `catalog-shell.ts` in place — same shell for admin + embed | Convert ~30 static imports to dynamic. Admin gets lazy loading for free. Single source of truth. ~50 lines + admin smoke regression check. | ✓ |
| Add `catalog-shell-lazy.ts` variant | New file with dynamic imports. Zero admin regression risk. Duplicate ~600 lines, drift risk between eager and lazy versions. | |
| Build-time Vite manualChunks magic | Keep catalog-shell.ts unchanged; rely on Vite chunking heuristics. Brittle; likely to fail tree-shaking. | |

**User's choice:** Refactor in place.

### Question 4: Shoelace dialogs/toasts portal to `document.body` — they ESCAPE the embed's Shadow DOM. Handling?

| Option | Description | Selected |
|--------|-------------|----------|
| Inline alerts only — no toast/dialog in v0.1 | Replace any remaining `<sl-dialog>/<sl-alert>` toast usage with inline Lit alerts. `<or-conflict-banner>` already inline (D6-04). Zero portal escape risk. | ✓ |
| Keep Shoelace dialogs; document 'admin reset required' | Toasts inherit host CSS. Technically breaks EMBED-04. Ugly. | |
| Patch Shoelace to mount portals in shadowRoot | Some Shoelace components accept portal target overrides; not all. Per-component audit + custom forks. High risk. | |

**User's choice:** Inline alerts only.

---

## Shadow DOM routing

### Question 1: How should the embed's internal router interact with the host page URL?

| Option | Description | Selected |
|--------|-------------|----------|
| Hash routing inside embed | `#open-routing/orgs/{id}/agents`. Host pathname untouched. Bookmarkable; browser back/forward works. ~30 lines hash-adapter. | ✓ |
| Memory routing — no URL sync | State in component instance only. Zero collision risk. Not bookmarkable; browser back exits embed. | |
| history + required `base-path` attribute | Bookmarkable but host must reserve a path prefix. Highest host integration burden. | |
| Configurable via `routing` attribute | Maximum flexibility; 3× Playwright matrix. Adds attribute parsing + 3 router adapters. | |

**User's choice:** Hash routing inside embed.

### Question 2: Admin uses history mode. Should hash become embed-only, or unify both apps on hash?

| Option | Description | Selected |
|--------|-------------|----------|
| Embed-only hash; admin stays on history | Shell accepts `routing-mode` prop (default 'history'). Each app keeps native UX. ~30 lines new code, no admin regression. | ✓ |
| Unify both apps on hash | Admin URLs become `/#/orgs/{id}/agents`. Consistent codepath; admin UX regression. | |
| Admin history; embed memory + `open-routing:route-change` events | Cleanest separation; highest host integration burden; unbookmarkable inside embed. | |

**User's choice:** Embed-only hash mode.

### Question 3: Multiple embed instances on same page (e.g., split-pane comparison) — how to handle hash collision?

| Option | Description | Selected |
|--------|-------------|----------|
| Document 'single embed per page' v0.1 limitation; last write wins | Realistic v0.1 trade-off. v0.2 can add per-instance hash-prefix. | ✓ |
| Per-instance hash-prefix derived from element id | Works for multi-embed; uglier URLs; complex; marginal v0.1 benefit. | |
| Second-onward instances auto-downgrade to memory routing | Magic behavior; hard to debug. | |

**User's choice:** Document single-embed limitation; multi-instance deferred to v0.2.

### Question 4: Shell redirects malformed `:org_id` route params to `/` (org-picker). Embed behaviour on malformed org-id ATTRIBUTE?

| Option | Description | Selected |
|--------|-------------|----------|
| Render error state inside Shadow DOM; do NOT fall back to org-picker | EMBED-02 mandates attribute-only org. Picker would surprise host. | ✓ |
| Fall back to `<or-org-picker>` (admin parity) | Picker writes to local state hidden from host. Misleading for integrators. | |
| Dispatch `open-routing:invalid-org-id` event; render nothing | Cleanest separation; bad UX if host doesn't wire listener. | |

**User's choice:** Error state inside Shadow DOM.

---

## CI infra (bundle gate + Playwright)

### Question 1: How should the 70 KB gzipped budget be measured and enforced in CI?

| Option | Description | Selected |
|--------|-------------|----------|
| size-limit declarative budget + GH Action PR comment | `.size-limit.json` config; `pnpm size`; `andresz1/size-limit-action` posts PR delta. Hard fail on bust. Trusted ecosystem (date-fns, redux, Preact). | ✓ |
| Custom Node gzip script | `zlib.gzipSync(readFileSync('dist/embed.js')).length`. ~15 lines. No external dep, no PR-comment niceties. | |
| Multi-budget gzip + brotli + raw | More signals; brotli is closer to CDN-served reality. More places to break build. | |

**User's choice:** size-limit declarative budget.

### Question 2: Does the 70 KB budget apply to eager baseline only, or eager+all-lazy chunks combined?

| Option | Description | Selected |
|--------|-------------|----------|
| Eager baseline ≤70 KB; lazy chunks tracked with soft budgets (~15 KB) | Interprets EMBED-01 'single ES module' as eager entry. Per-entity chunks logged not blocked. | ✓ |
| Eager + sum of all lazy chunks ≤70 KB | Stricter total cap. Higher CI-failure rate on new entities. | |
| Eager ≤70 KB; lazy chunks untracked | Simplest gate. Loses signal — a bloated entity wouldn't fail. | |

**User's choice:** Eager baseline ≤70 KB; lazy chunks soft-tracked.

### Question 3: Playwright stub host matrix — where do stubs live, how do they serve the embed bundle?

| Option | Description | Selected |
|--------|-------------|----------|
| `apps/embed/e2e/hosts/{react,vue,html}/` — static HTML; React/Vue from esm.sh CDN; bundle from Vite preview | Lightweight: 3 static HTML files, no React/Vue devDeps in monorepo. Single Playwright project with 3 spec files. | ✓ |
| Full Vite SPAs per host (separate packages) | Realistic browser environment. Adds 3 packages + ~200MB node_modules. Heaviest infra. | |
| Top-level `web/e2e-hosts/{react,vue,html}/` shared across phases | Premature generalization for v0.1's single embed surface. | |

**User's choice:** apps/embed/e2e/hosts/ static-HTML approach.

### Question 4: Phase 7 promotes Playwright to CI per D6-31. What's the failure budget for the cross-host matrix?

| Option | Description | Selected |
|--------|-------------|----------|
| `retries: 1` per spec; same EMBED-10 assertions across all 3 hosts; flaky failure blocks merge | Absorbs CDN flakes; forces fix-or-skip on real regressions. | ✓ |
| `0 retries`; per-host required-status-check (3 separate green checks) | Strict; most signal; most noise on CDN flakes. | |
| `retries: 2`; mark host failures as non-blocking (warning only) | Loosest. Defeats the matrix purpose. | |

**User's choice:** 1 retry per spec; all 3 hosts gate merge.

---

## Embed public contract

### Question 1: `open-routing:request-context` fires on mount — purpose and payload?

| Option | Description | Selected |
|--------|-------------|----------|
| Announce-only; `detail = { orgId, apiBaseUrl, theme, modules }` | Host listens for diagnostics. v1 auth can add reply-via-custom-event without breaking. Matches research/SUMMARY.md line 199. | ✓ |
| Negotiable; host can `preventDefault()` to block API calls until context provided | More powerful for v1 SSO. Bigger surface; more states to test. Premature. | |
| Fire-and-forget with NO payload | Loses signal; embed becomes a black box for host telemetry. | |

**User's choice:** Announce-only.

### Question 2: `open-routing:auth-expired` on 401 — trigger semantics, payload, UI behavior?

| Option | Description | Selected |
|--------|-------------|----------|
| Fire on EVERY 401; `detail = { statusCode, requestId, path }`; embed renders inline 'Session expired' state, continues fetching | Each 401 dispatches; host owns rate-limiting + recovery; inline UI shows request_id (D6-24 parity). | ✓ |
| Fire ONCE per mount on first 401; suppress later; embed locks UI until remount | Less event noise; forces host integration. More complex for v0.1 stub auth. | |
| Fire on every 401; no UI change; host owns 100% of recovery | Minimal in-embed change. Worst UX for hosts that forget to wire listener. | |

**User's choice:** Every 401; inline state; continues fetching.

### Question 3: Distribution — how is `@open-routing/catalog-embed` shipped in v0.1?

| Option | Description | Selected |
|--------|-------------|----------|
| Build artifact + npm-ready `package.json`; DO NOT publish | `pnpm pack --dry-run` verified in CI. Tarball install for early integrators. Real publishing deferred to v1. | ✓ |
| Publish to public npm registry | Real package, real version. Premature for v0.1 (no customers, brand exposure). | |
| Publish to GitHub Packages (private) | Authenticated install via GH token. Non-trivial; can come later. | |
| Self-host on a CDN | `<script type="module" src="https://cdn.../embed.js">`. Adds ops surface; jsDelivr auto-syncs npm but needs publish first. | |

**User's choice:** Build artifact only; no publishing in v0.1.

### Question 4: CORS — who owns it in v0.1?

| Option | Description | Selected |
|--------|-------------|----------|
| Go API ships CORS middleware (go-chi/cors); allowlist via env var; dev default `*` | Removes a host-deployment step for early integrators. New `internal/middleware/cors.go`. Phase 1 isolation suite extended. | ✓ |
| Host is responsible — must reverse-proxy `/v1/*` to API on same origin | Cleaner architecture; harder onboarding; embed always uses relative URLs. | |
| Document both paths in README; ship neither | Blocks Playwright matrix (stub hosts need working CORS or proxy). | |

**User's choice:** Go API CORS middleware.

---

## Claude's Discretion

User explicitly accepted Claude-discretion on these:
- Exact internal layout of `apps/embed/src/` (boundary between `index.ts` and `embed-element.ts`).
- size-limit GH Action choice (`andresz1/` vs alternatives).
- esm.sh version pinning strategy (exact patch vs major) in stub hosts.
- Hash-adapter implementation detail (URLPattern parse vs hand-rolled regex).
- Auth-expired inline banner copy + dismiss UX.
- Whether `<or-conflict-banner>` and auth-banner share a base component.
- README example payload for `theme=` attribute.
- Shadow DOM mode (open vs closed) — recommend `open`.
- Playwright spec file layout (one file per assertion type vs one per host).
- Source-map emission policy in production build.

## Deferred Ideas

These came up but explicitly belong outside Phase 7:
- Per-instance hash-prefix attribute for multi-embed pages (v0.2).
- history-mode routing for embed with base-path attribute.
- Real npm/CDN publishing of `@open-routing/catalog-embed` (v1).
- Unified hash routing across admin + embed.
- 3-mode configurable routing attribute.
- Negotiable open-routing:request-context contract.
- First-401-only auth-expired with UI lock.
- GitHub Packages private publishing path.
- Per-org brand theme customization (carry-forward from D6-19 deferral).
- Server-side rendering of `<open-routing-catalog>`.
- Operator-facing CORS allowlist UI per org.
- Embed-side localStorage / sessionStorage usage.
- Bundle-analyzer (rollup-plugin-visualizer) PR artifact.
- Brotli + raw-minified bundle budgets (gzip-only in v0.1).
