---
phase: 06
slug: shared-ui-library-standalone-admin
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-17
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `06-RESEARCH.md` Validation Architecture (§Validation Architecture).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 3.2.4 + happy-dom 20.9.0 (component); Playwright 1.60.0 (E2E smoke) |
| **Config file** | `web/packages/ui/vitest.config.ts` (Wave 1 installs); `web/apps/admin/playwright.config.ts` (Wave 5 installs) |
| **Quick run command** | `pnpm --filter @open-routing/ui test` |
| **Full suite command** | `pnpm --filter @open-routing/ui test -- --coverage && pnpm --filter @open-routing/admin build` |
| **E2E smoke command** | `pnpm --filter @open-routing/admin test:e2e` |
| **Estimated runtime** | ~30s quick · ~90s full · ~45s E2E |

---

## Sampling Rate

- **After every task commit:** Run `pnpm --filter @open-routing/ui test` (component unit tests; ~30s)
- **After every plan wave:** Run `pnpm --filter @open-routing/ui test -- --coverage && pnpm --filter @open-routing/admin build`
- **Before `/gsd-verify-work`:** Full suite green + Playwright cross-org smoke + ajv/codegen drift gate green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Populated by the planner during PLAN.md generation. Each plan task whose
> `<acceptance_criteria>` matches a phase requirement MUST appear here, citing
> the automated command from "Test Infrastructure".

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 06-XX-YY | XX | N | ADMIN-NN | — | — | unit/build/lint | `pnpm --filter @open-routing/ui test` | ❌ W0/W1 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

> Files that must exist before any unit test can run. The planner injects
> these as Wave 1 scaffold tasks in 06-01-PLAN.md (Wave 1 also serves as
> Wave 0 for this phase — there is no separate pre-wave installation step
> per D6-02).

- [ ] `web/packages/ui/vitest.config.ts` — Vitest config with `happy-dom` environment + `setupFiles: ['./src/test-setup.ts']`
- [ ] `web/packages/ui/src/test-setup.ts` — imports `urlpattern-polyfill` (Node-side polyfill for `@lit-labs/router` route parsing under Vitest; browser is Baseline 2025 — no runtime polyfill needed)
- [ ] `web/apps/admin/vite.config.ts` — Vite 8 config with `@rolldown/plugin-babel` + `@babel/plugin-proposal-decorators` for Lit 3 `@customElement` decorator support
- [ ] `web/apps/admin/index.html` — SPA entry HTML with mount point for `<or-catalog-shell>`
- [ ] `web/apps/admin/playwright.config.ts` — Playwright config; reuses existing CI Chromium browser
- [ ] `web/packages/ui/scripts/gen-validators.mjs` — ajv codegen Node script (ajv-cli is dead; programmatic API required — see RESEARCH §B.2)
- [ ] `web/packages/ui/lit-localize.json` — `@lit/localize` config (sourceLocale: `en`, targetLocales: `[vi]`, mode: `runtime`)
- [ ] `web/packages/ui/xliff/en.xliff` — initial empty XLIFF source for extract pipeline
- [ ] `tsconfig.base.json` / `web/tsconfig.json` — set `experimentalDecorators: true`, `useDefineForClassFields: false`, `verbatimModuleSyntax: true` per Lit 3 + Vite 8 requirements (RESEARCH §A.4)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Theme visual contrast (AA) across all 3 themes (or-light / or-dark / or-brand) | ADMIN-06 + D6-V-43 | Color contrast assertion needs a designer's eye; automated chroma checks miss perceived contrast on data-dense screens | Open list page in each theme, sweep tab through every row + status badge; spot-check destructive button + conflict banner contrast |
| 409 conflict banner UX (CRUD + status) — copy reads cleanly and the user knows what to do next | ADMIN-05 + D6-V-15 | Word-choice quality is judgmental | Force a concurrent edit by opening the same entity in two tabs; save in tab A; save in tab B; verify the banner explains what to do in plain English |
| Bulk import partial success layout (207) — failed rows table is scannable when 100+ rows fail | ADMIN-02 + IMP-03 | Information density / scan-ability is a UX judgment | Generate a 50-row CSV with 25 deliberate failures; upload; verify the failure table is usable without horizontal scroll on a 1440-wide laptop |
| Sidebar collapses cleanly to drawer at ≤1024px | UI-SPEC §9 | Responsive layout judgment | Resize browser from 1920 → 800; verify no flash-of-broken-layout |

---

## Validation Sign-Off

- [ ] All tasks in 06-*-PLAN.md have an `<automated>` verify command OR a Wave 1 dependency listed above
- [ ] Sampling continuity: no 3 consecutive tasks without an automated verify
- [ ] Wave 0/Wave 1 covers all MISSING references (test infra, codegen, theme tokens, i18n)
- [ ] No watch-mode flags in automated commands (`--watch=false` if framework defaults to watch)
- [ ] Feedback latency < 30s for quick run
- [ ] `nyquist_compliant: true` set in frontmatter once the per-task map is complete and green

**Approval:** pending
