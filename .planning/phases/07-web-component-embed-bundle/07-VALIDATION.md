---
phase: 7
slug: web-component-embed-bundle
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-18
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Sourced from `07-RESEARCH.md` §Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Frontend Unit Framework** | `vitest` 3.2.4 (existing in repo) — happy-dom environment |
| **Frontend E2E Framework** | `@playwright/test` 1.60.0 (existing in repo) — 3-project matrix |
| **Backend Test Framework** | Go `testing` + `stretchr/testify` (existing) |
| **Config files (Wave 0)** | `web/apps/embed/vitest.config.ts`, `web/apps/embed/playwright.config.ts`, `web/apps/embed/.size-limit.json` |
| **Quick run (embed unit)** | `pnpm --filter @open-routing/embed test` |
| **Quick run (Go CORS)** | `cd services/api && go test ./internal/middleware -run CORS -short` |
| **Full suite** | `pnpm -r test && pnpm --filter @open-routing/embed test:e2e && go test ./...` |
| **Bundle-size gate** | `pnpm --filter @open-routing/embed size` (size-limit + gzip) |
| **Estimated runtime (per task)** | ~5s unit / ~5–8s Go |
| **Estimated runtime (per wave)** | ~3–5 min (full e2e matrix) |

---

## Sampling Rate

- **After every task commit:** `pnpm --filter @open-routing/embed test` (component unit, ~5s) or `go test ./internal/middleware -run CORS -short` for Go tasks.
- **After every wave merge:** unit + Playwright matrix (`pnpm --filter @open-routing/embed test:e2e`) + Go CORS suite. Add `pnpm --filter @open-routing/embed size` once Wave 0 size-limit config exists.
- **Before `/gsd-verify-work`:** Full suite — backend isolation + frontend unit + Playwright matrix × 3 hosts + size-limit ≤70 KB gzipped + admin regression smoke (`pnpm --filter @open-routing/admin test:e2e smoke.spec.ts`) + `pnpm pack --dry-run` smoke.
- **Max feedback latency:** ~5s per task; ~5 min per wave.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 7-W0-01 | Wave 0 | 0 | infra | — | N/A | ci | `pnpm --filter @open-routing/embed test --run` (vitest config sanity) | ❌ W0 | ⬜ pending |
| 7-W0-02 | Wave 0 | 0 | infra | — | N/A | ci | `pnpm --filter @open-routing/embed test:e2e --list` (playwright config sanity) | ❌ W0 | ⬜ pending |
| 7-W0-03 | Wave 0 | 0 | CORS infra | T-7-CORS-01 | CORS preflight short-circuits before OrgContext | backend | `go test ./services/api/internal/middleware -run CORS -short` | ❌ W0 | ⬜ pending |
| 7-W0-04 | Wave 0 | 0 | EMBED-10 | — | Bundle budget gate exists | ci | `cat web/apps/embed/.size-limit.json && pnpm --filter @open-routing/embed size --json` | ❌ W0 | ⬜ pending |
| 7-EMBED-01-a | embed-build | 1 | EMBED-01 | — | Single ES module ≤70 KB gzip | ci | `pnpm --filter @open-routing/embed build && pnpm --filter @open-routing/embed size` | ❌ W0 | ⬜ pending |
| 7-EMBED-01-b | embed-build | 1 | EMBED-01 | — | `dist/embed.js` parses as a valid ES module (syntax-only check; bare-specifier dynamic imports for `@open-routing/ui` chunks + lit-localize cannot be resolved by Node so we do not actually import) | ci | `pnpm --filter @open-routing/embed build && pnpm exec esbuild --bundle=false --format=esm --log-level=silent web/apps/embed/dist/embed.js > /dev/null` | ❌ W0 | ⬜ pending |
| 7-EMBED-01-c | embed-build | 1 | EMBED-01 | — | Lazy chunks emit per-entity (≥6) | ci | `pnpm --filter @open-routing/embed build && ls web/apps/embed/dist/embed-*.js \| wc -l` | ❌ W0 | ⬜ pending |
| 7-EMBED-02-a | embed-element | 1 | EMBED-02 | T-7-02 | UUIDv7 `org-id` validated; error inline on malformed | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "invalid org-id"` | ❌ W1 | ⬜ pending |
| 7-EMBED-02-b | embed-element | 1 | EMBED-02 | T-7-02 | `createApiClient.getOrgId()` reads attribute live; X-Org-Id on every call | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "X-Org-Id"` | ❌ W1 | ⬜ pending |
| 7-EMBED-02-c | embed-element | 1 | EMBED-02 | T-7-02 | `org-id` attribute change re-validates without remount | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "attribute change"` | ❌ W1 | ⬜ pending |
| 7-EMBED-03-a | shell-lazy-routes | 1 | EMBED-03 | — | All 6 entities reachable inside embed via lazy routes | e2e | `pnpm --filter @open-routing/embed test:e2e embed-modules-filter.spec.ts` | ❌ W2 | ⬜ pending |
| 7-EMBED-03-b | shell-lazy-routes | 1 | EMBED-03 | — | Phase 6 admin smoke passes after shell refactor (D7-03 regression guard) | e2e | `pnpm --filter @open-routing/admin test:e2e smoke.spec.ts` | ✅ existing | ⬜ pending |
| 7-EMBED-04-a | embed-element | 2 | EMBED-04 | T-7-04 | Host styles don't bleed into embed | e2e | `pnpm --filter @open-routing/embed test:e2e embed-shadow-dom-isolation.spec.ts -t "host CSS does not leak in"` × 3 hosts | ❌ W2 | ⬜ pending |
| 7-EMBED-04-b | embed-element | 2 | EMBED-04 | T-7-04 | Embed styles don't leak out | e2e | `pnpm --filter @open-routing/embed test:e2e embed-shadow-dom-isolation.spec.ts -t "embed CSS does not escape"` × 3 hosts | ❌ W2 | ⬜ pending |
| 7-EMBED-04-c | shoelace-audit | 1 | EMBED-04 | T-7-04 | `<sl-dialog>` replacement — no portal escape | e2e | `pnpm --filter @open-routing/embed test:e2e embed-shadow-dom-isolation.spec.ts -t "no portal escape"` | ❌ W2 | ⬜ pending |
| 7-EMBED-05-a | embed-element | 1 | EMBED-05 | — | `theme` accepts JSON object; applies tokens on host | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "theme JSON parse"` | ❌ W1 | ⬜ pending |
| 7-EMBED-05-b | embed-element | 1 | EMBED-05 | — | `theme` accepts named string (`or-light`/`or-dark`/`or-brand`) | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "theme named string"` | ❌ W1 | ⬜ pending |
| 7-EMBED-05-c | embed-element | 1 | EMBED-05 | — | Malformed theme JSON falls back to `or-light` silently | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "malformed theme"` | ❌ W1 | ⬜ pending |
| 7-EMBED-06-a | shell-modules | 1 | EMBED-06 | — | `modules` attribute filters sidebar nav | e2e | `pnpm --filter @open-routing/embed test:e2e embed-modules-filter.spec.ts -t "modules filter"` × 3 hosts | ❌ W2 | ⬜ pending |
| 7-EMBED-06-b | shell-modules | 1 | EMBED-06 | — | Unlisted entity routes redirect to first allowed module | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "module restriction route"` | ❌ W2 | ⬜ pending |
| 7-EMBED-07-a | embed-element | 1 | EMBED-07 | — | `open-routing:request-context` fires once on mount with `composed:true` | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "request-context"` | ❌ W1 | ⬜ pending |
| 7-EMBED-07-b | embed-element | 1 | EMBED-07 | T-7-07 | `open-routing:auth-expired` fires on every 401 | unit | `pnpm --filter @open-routing/embed test embed-element.test.ts -t "auth-expired event"` | ❌ W1 | ⬜ pending |
| 7-EMBED-07-c | embed-element | 2 | EMBED-07 | T-7-07 | Both events cross Shadow DOM (composed:true behavior) | e2e | `pnpm --filter @open-routing/embed test:e2e embed-auth-expired.spec.ts -t "host receives event"` × 3 hosts | ❌ W2 | ⬜ pending |
| 7-EMBED-08-a | conflict-banner-reuse | 1 | EMBED-08 | — | 409 reload UX parity (`<or-conflict-banner>` reused) | unit | `pnpm --filter @open-routing/ui test conflict-banner.test.ts` | ✅ existing | ⬜ pending |
| 7-EMBED-09-a | npm-tarball | 3 | EMBED-09 | — | `pnpm pack --dry-run` produces valid tarball listing | ci | `pnpm --filter @open-routing/embed pack --dry-run` | ❌ W3 | ⬜ pending |
| 7-EMBED-09-b | npm-tarball | 3 | EMBED-09 | — | Tarball install in scratch dir succeeds | ci | CI shell script: `pnpm pack && pnpm install file:./*.tgz` in tmp dir | ❌ W3 | ⬜ pending |
| 7-EMBED-10-a | playwright-matrix | 2 | EMBED-10 | — | All 4 specs run across 3 host projects (12 runs) | e2e | `pnpm --filter @open-routing/embed test:e2e` | ❌ W2 | ⬜ pending |
| 7-EMBED-10-b | ci-jobs | 3 | EMBED-10 | — | Bundle size CI gate enforces ≤70 KB on every PR | ci | `andresz1/size-limit-action@v1` + `.size-limit.json` | ❌ W3 | ⬜ pending |
| 7-CORS-01 | cors-middleware | 1 | (CORS-1) | T-7-CORS-01 | go-chi/cors short-circuits OPTIONS preflight without `X-Org-Id` | backend | `cd services/api && go test ./internal/middleware -run CORS -short` | ❌ W0 | ⬜ pending |
| 7-CORS-02 | cors-middleware | 1 | (CORS-2) | T-7-CORS-02 | OPTIONS preflight to `/healthz` passes (bypass route) | backend | `cd services/api && go test ./test/isolation -run CORSPreflight -short` | ❌ W0 | ⬜ pending |
| 7-CORS-03 | cors-middleware | 1 | (CORS-3) | T-7-CORS-03 | Cross-origin GET with valid origin gets CORS headers | backend | `cd services/api && go test ./test/isolation -run CORSCrossOriginAllow -short` | ❌ W0 | ⬜ pending |
| 7-CORS-04 | cors-middleware | 1 | (CORS-4) | T-7-CORS-04 | Cross-origin GET with rejected origin gets no Allow-Origin header | backend | `cd services/api && go test ./test/isolation -run CORSCrossOriginReject -short` | ❌ W0 | ⬜ pending |
| 7-CORS-05 | cors-middleware | 1 | (CORS-5) | — | CORS middleware doesn't break Phase 1 isolation suite | backend | `cd services/api && go test ./test/isolation -short` (full Phase 1 suite) | ✅ existing | ⬜ pending |
| 7-D7-03-a | shell-routing-mode | 1 | (D7-03) | — | Shell `routingMode='hash'` triggers hash adapter; `'history'` does not | unit | `pnpm --filter @open-routing/ui test catalog-shell.test.ts -t "routingMode"` | ✅ existing (extend) | ⬜ pending |
| 7-D7-03-b | shell-routing-mode | 1 | (D7-03) | — | All 30+ lazy imports in shell resolve (no chunk-not-found errors) | e2e | implicit via `embed-modules-filter.spec.ts` + admin `smoke.spec.ts` | ✅ existing + new | ⬜ pending |
| 7-HASH-01 | hash-router-adapter | 1 | (Hash adapter) | — | `hashchange` event triggers `routes.goto()` with stripped hash | unit | `pnpm --filter @open-routing/embed test hash-router-adapter.test.ts` | ❌ W1 | ⬜ pending |
| 7-HASH-02 | hash-router-adapter | 1 | (Hash adapter) | — | Adapter ignores non-`open-routing/` prefixed hashes | unit | `pnpm --filter @open-routing/embed test hash-router-adapter.test.ts -t "ignore unrelated hash"` | ❌ W1 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

> Plan IDs are placeholders until the planner produces concrete plan slugs. Wave numbers track Wave 0 (infra) / 1 (build + element + shell refactor) / 2 (e2e specs + shadow DOM) / 3 (packaging + CI jobs). Threat Refs map to the threat model produced in PLAN.md per phase Security Threat Model gate.

---

## Wave 0 Requirements

These test-infrastructure artifacts MUST exist before Wave 1 implementation begins (otherwise per-task verification has no anchor):

- [ ] `web/apps/embed/vitest.config.ts` — Vitest config with `happy-dom` environment + `urlpattern-polyfill` setup
- [ ] `web/apps/embed/test-setup.ts` — Loads `urlpattern-polyfill` for hash-router URLPattern usage
- [ ] `web/apps/embed/playwright.config.ts` — 3-project matrix (react/vue/html); `webServer: 'vite preview'`
- [ ] `web/apps/embed/e2e/hosts/react/index.html` — Static host with importmap + React 18.3.1 from esm.sh
- [ ] `web/apps/embed/e2e/hosts/vue/index.html` — Static host with importmap + Vue 3.5.13 from esm.sh
- [ ] `web/apps/embed/e2e/hosts/html/index.html` — Plain HTML host
- [ ] `web/apps/embed/e2e/specs/embed-shadow-dom-isolation.spec.ts` — EMBED-04 assertion stubs
- [ ] `web/apps/embed/e2e/specs/embed-org-id-header.spec.ts` — EMBED-02 X-Org-Id assertion stubs
- [ ] `web/apps/embed/e2e/specs/embed-auth-expired.spec.ts` — EMBED-07 + 401 dispatch stubs
- [ ] `web/apps/embed/e2e/specs/embed-modules-filter.spec.ts` — EMBED-06 + EMBED-03 filter stubs
- [ ] `web/apps/embed/src/embed-element.test.ts` — Unit-test stub for `OpenRoutingCatalog` (happy-dom)
- [ ] `web/apps/embed/src/hash-router-adapter.test.ts` — Unit-test stub for hash adapter
- [ ] `web/apps/embed/src/locale.ts` — Type stub `export async function applyEmbedLocale(_raw: string): Promise<void> {}` — unblocks Plan 07-06 `tsc --noEmit` (TS2307 guard); overwritten by Plan 07-07
- [ ] `web/apps/embed/.size-limit.json` — Declarative 70 KB gzipped budget for `dist/embed.js`
- [ ] `services/api/internal/middleware/cors_test.go` — Unit tests for CORS middleware (origin allowlist, methods, headers)
- [ ] `services/api/test/isolation/cors_preflight_test.go` — Integration: OPTIONS preflight to bypass + `/v1`
- [ ] `services/api/test/isolation/cors_cross_origin_test.go` — Integration: cross-origin GET allow + reject
- [ ] `.github/workflows/ci.yml` — New jobs `web-bundle-size` + `web-e2e-embed`; Playwright browser cache

*No existing test infrastructure covers any of the EMBED-XX requirements — Phase 7 is a clean slate for embed-specific tests.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Browser-back/forward inside embed hash routes (`history.back()` from inside embed) | EMBED-03, D7-05 | Playwright covers `hashchange` programmatically but real keyboard back/forward needs human in DevTools | Mount embed in `http://localhost:4173/hosts/html/`, navigate agents → skills, hit browser back, confirm shell renders agents list with no chunk error |
| Theme JSON live edit via DevTools (set `theme=` attribute by hand, confirm CSS custom properties update) | EMBED-05 | Tests cover `theme` parse on mount; live attribute mutation in DevTools is a manual smoke | In DevTools console: `document.querySelector('open-routing-catalog').setAttribute('theme', JSON.stringify({...}))`; confirm `:host` custom-property values update |
| Real-world CDN-served bundle size (Brotli) vs CI gzip budget | EMBED-01 | size-limit measures gzip in CI; Brotli is what CDN actually serves (15–30% smaller per Cloudflare data) | After `pnpm --filter @open-routing/embed build`, run `brotli -c dist/embed.js \| wc -c` and confirm <60 KB |

---

## Validation Sign-Off

- [ ] All EMBED-XX requirements have an `<automated>` verify command or a Wave 0 dependency listed above
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify (planner must check during plan creation)
- [ ] Wave 0 covers all MISSING references (18 files listed)
- [ ] No `watch`-mode flags on per-task commands (vitest `--run`, playwright `--headed=false`)
- [ ] Feedback latency < 30s per task; < 5 min per wave
- [ ] `nyquist_compliant: true` set in frontmatter once Wave 0 lands and Plan IDs are wired in
- [ ] Threat refs (`T-7-NN`) cross-link with PLAN.md `<threat_model>` blocks

**Approval:** pending — flip `status: draft → ready` once planner wires real Plan IDs into the table above.
