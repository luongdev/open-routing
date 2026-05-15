---
phase: 01-foundation-polyglot-monorepo
plan: 02
subsystem: infra
tags: [pnpm, turborepo, typescript, eslint, monorepo, workspace, web]

# Dependency graph
requires:
  - phase: 01-foundation-polyglot-monorepo
    provides: D-12 minimal pnpm scaffold scope; D-13 no-Vite/Lit/Shoelace constraint; FOUND-01 web/ directory shape
provides:
  - pnpm workspace at web/ resolving @open-routing/admin, @open-routing/embed, @open-routing/ui
  - turbo typecheck + lint pipelines (web/turbo.json) ready for Plan 08 CI to call
  - shared strict tsconfig.base.json that per-package tsconfigs extend
  - ESLint v9 flat config covering all three packages
  - committed pnpm-lock.yaml enabling --frozen-lockfile installs in CI
affects:
  - 01-08 (CI workflow — web-typecheck + web-lint jobs)
  - 06-* (admin + ui packages will fill these stubs with Vite + Lit + Shoelace)
  - 07-* (embed package will fill its stub with Web Component bundle)

# Tech tracking
tech-stack:
  added:
    - pnpm@10.33.0 (workspace manager, locked via packageManager field)
    - turbo@2.9.14 (task orchestrator inside web/ only per D-09)
    - typescript@5.9.3 (strict, noUncheckedIndexedAccess, isolatedModules, verbatimModuleSyntax)
    - eslint@9.39.4 (flat config, ESM)
    - "@typescript-eslint/parser@8.59.3"
    - "@typescript-eslint/eslint-plugin@8.59.3"
    - prettier@3.8.3
  patterns:
    - "Workspace shape locked: apps/* + packages/* globs; per-package npm names locked to @open-routing/<name>"
    - "Each sub-package ships exactly three files (package.json + tsconfig.json + src/index.ts) per D-12"
    - "Empty module idiom: src/index.ts is `export {};` (D-12 forbids real impl in Phase 1; satisfies isolatedModules)"
    - "Per-package tsconfig extends ../../tsconfig.base.json (depth identical for apps/* and packages/*)"
    - "Turborepo pipelines: typecheck depends on ^typecheck for topological order (admin/embed will later depend on ui)"

key-files:
  created:
    - web/pnpm-workspace.yaml
    - web/package.json
    - web/.npmrc
    - web/turbo.json
    - web/tsconfig.base.json
    - web/eslint.config.js
    - web/.gitignore
    - web/apps/admin/package.json
    - web/apps/admin/tsconfig.json
    - web/apps/admin/src/index.ts
    - web/apps/embed/package.json
    - web/apps/embed/tsconfig.json
    - web/apps/embed/src/index.ts
    - web/packages/ui/package.json
    - web/packages/ui/tsconfig.json
    - web/packages/ui/src/index.ts
    - web/pnpm-lock.yaml
  modified: []

key-decisions:
  - "Locked pnpm@10.33.0 via packageManager field — corepack picks it up automatically; CI never picks a stray version"
  - "ESLint flat config in ESM form (matches `\"type\": \"module\"` in root package.json) with project: false — type-aware lint would be overhead against empty stubs; switch on in Phase 6/7"
  - "Added `lib: [\"ES2022\", \"DOM\", \"DOM.Iterable\"]` to tsconfig.base.json so the future Lit/Shoelace code (Phase 6/7) gets DOM globals without modifying base"
  - "shamefully-hoist=false retains strict pnpm isolated node_modules (prevents phantom-dep risk)"
  - "Used turbo `dependsOn: [\"^typecheck\"]` so when admin -> ui workspace dep lands in Phase 6, ui typechecks before admin without further config"

patterns-established:
  - "Pattern: pnpm workspace member glob — `apps/*` for end-products (admin, embed), `packages/*` for shared libraries (ui)"
  - "Pattern: per-package scripts are uniform (typecheck = tsc --noEmit; lint = eslint src --max-warnings 0) so Turborepo can drive them all"
  - "Pattern: root delegates to turbo (`turbo typecheck`, `turbo lint`); turbo dispatches per-package; no direct workspace iteration"

requirements-completed: [FOUND-01]

# Metrics
duration: 3m 8s
completed: 2026-05-15
---

# Phase 01 Plan 02: pnpm Workspace Stub Summary

**Bootstrapped the pnpm workspace at `web/` with three empty-stub packages (`@open-routing/admin`, `@open-routing/embed`, `@open-routing/ui`), turbo typecheck + lint pipelines, ESLint v9 flat config, and a committed pnpm-lock.yaml — D-12/D-13 compliant scaffold ready for Plan 08 CI gating.**

## Performance

- **Duration:** 3m 8s
- **Started:** 2026-05-15T10:13:31Z
- **Completed:** 2026-05-15T10:16:39Z
- **Tasks:** 3
- **Files created:** 17 (7 workspace root + 9 sub-package + 1 lockfile)
- **Files modified:** 0

## Accomplishments

- pnpm workspace resolves three packages with locked names `@open-routing/admin`, `@open-routing/embed`, `@open-routing/ui` (D-12 / FOUND-01)
- `cd web && pnpm install --frozen-lockfile` exits 0 — proves Plan 08's CI workflow will pass
- `cd web && pnpm typecheck` exits 0 — all 3 packages succeed via turbo (admin, ui, embed)
- `cd web && pnpm lint` exits 0 — ESLint reports "No issues found" across all 3 packages
- 111 transitive packages locked in `pnpm-lock.yaml` (committed; required for CI reproducibility)
- Zero `lit`, `@shoelace-style/shoelace`, or `vite` references anywhere in `web/` (D-13 honored — verified by grep in Task 2 verify command)
- Turborepo confined to `web/turbo.json` only — Go side never touches turbo (D-09)

## Task Commits

Each task committed atomically (3 commits total):

1. **Task 1: Create pnpm workspace root files** — `9cdead4` (feat)
   - 7 files: pnpm-workspace.yaml, package.json, .npmrc, turbo.json, tsconfig.base.json, eslint.config.js, .gitignore
2. **Task 2: Create three sub-packages (admin, embed, ui) with empty index.ts stubs** — `8d45e49` (feat)
   - 9 files: 3 package.json + 3 tsconfig.json + 3 src/index.ts (each `export {};`)
3. **Task 3: Run pnpm install, generate lockfile, validate pnpm typecheck + pnpm lint succeed** — `826d1c4` (chore)
   - 1 file: pnpm-lock.yaml (1005 lines, 111 packages resolved)

## Files Created/Modified

### Workspace root (Task 1)
- `web/pnpm-workspace.yaml` — Declares `apps/*` + `packages/*` workspace member globs
- `web/package.json` — `open-routing-web` root: ESM, `packageManager: pnpm@10.33.0`, turbo/typescript/eslint/prettier devDeps, scripts delegating to turbo
- `web/.npmrc` — `auto-install-peers=true`, `shamefully-hoist=false` (strict pnpm isolated layout)
- `web/turbo.json` — `typecheck` (with `^typecheck` topo order) + `lint` tasks
- `web/tsconfig.base.json` — strict + noUncheckedIndexedAccess + isolatedModules + verbatimModuleSyntax + DOM lib
- `web/eslint.config.js` — flat config (ESM) using `@typescript-eslint/parser` + `@typescript-eslint/eslint-plugin`, `project: false`
- `web/.gitignore` — `node_modules/`, `.turbo/`, `dist/`, `*.tsbuildinfo`, `.DS_Store`

### Sub-packages (Task 2) — 3 packages × 3 files each
- `web/apps/admin/package.json` — `@open-routing/admin@0.0.0`, private ESM, scripts `typecheck` + `lint`
- `web/apps/admin/tsconfig.json` — extends `../../tsconfig.base.json`, rootDir/outDir/include set
- `web/apps/admin/src/index.ts` — `export {};` (D-12: index.ts exports nothing)
- `web/apps/embed/package.json` — `@open-routing/embed@0.0.0` (same shape as admin)
- `web/apps/embed/tsconfig.json` — byte-identical to admin tsconfig
- `web/apps/embed/src/index.ts` — `export {};`
- `web/packages/ui/package.json` — `@open-routing/ui@0.0.0` (same shape as admin)
- `web/packages/ui/tsconfig.json` — same `extends: "../../tsconfig.base.json"` (depth matches apps/*)
- `web/packages/ui/src/index.ts` — `export {};`

### Lockfile (Task 3)
- `web/pnpm-lock.yaml` — 1005 lines; 111 packages installed under `web/node_modules/.pnpm/`

## Decisions Made

### Resolved devDependency versions (from `pnpm install` against the locked semver ranges)

| Package | Specifier | Resolved |
|---------|-----------|----------|
| turbo | `^2.0.0` | `2.9.14` |
| typescript | `^5.6.0` | `5.9.3` |
| eslint | `^9.0.0` | `9.39.4` |
| @typescript-eslint/parser | `^8.0.0` | `8.59.3` |
| @typescript-eslint/eslint-plugin | `^8.0.0` | `8.59.3` |
| prettier | `^3.0.0` | `3.8.3` |

(Full transitive closure in `web/pnpm-lock.yaml`.)

### Verification output (last lines of each command run from `web/`)

- `pnpm install --frozen-lockfile`: `Already up to date / Done in 196ms using pnpm v10.33.0`
- `pnpm typecheck`: `Tasks: 3 successful, 3 total / Cached: 3 cached, 3 total / Time: 32ms >>> FULL TURBO`
- `pnpm lint`: `ESLint: No issues found` — all 3 turbo tasks succeed (verified with `pnpm exec turbo lint --force`: 3 successful, 3 total)

### D-13 compliance check

`grep -i "lit\|shoelace\|vite" web/**/package.json` returns zero hits. Per-package devDeps are absent (only the root pulls in turbo/typescript/eslint/prettier). No Vite config files exist. No Lit components exist. Stubs are pure `export {};` only.

### Peer-dependency warnings

None observed during `pnpm install`. `auto-install-peers=true` in `.npmrc` resolved every peer cleanly.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required for this plan. All tooling is local (pnpm + turbo + tsc + eslint).

## Next Phase Readiness

- **Plan 08 (CI workflow):** All inputs ready. `web-typecheck` job can call `cd web && pnpm install --frozen-lockfile && pnpm typecheck`. `web-lint` job can call `cd web && pnpm install --frozen-lockfile && pnpm lint`. Lockfile committed.
- **Phase 6 (admin + ui):** Stubs in place. Phase 6 only adds Vite/Lit/Shoelace dependencies into the existing `package.json` files and fills `src/index.ts` — no workspace restructure needed. The `@open-routing/admin -> @open-routing/ui` workspace edge will be added with `"@open-routing/ui": "workspace:*"`.
- **Phase 7 (embed):** Stub in place. Phase 7 fills `web/apps/embed/src/index.ts` with the Web Component bundle and adds Vite + Lit deps to `web/apps/embed/package.json`.

## Self-Check: PASSED

**Files created (all 17 present):**
- web/pnpm-workspace.yaml — FOUND
- web/package.json — FOUND
- web/.npmrc — FOUND
- web/turbo.json — FOUND
- web/tsconfig.base.json — FOUND
- web/eslint.config.js — FOUND
- web/.gitignore — FOUND
- web/apps/admin/package.json — FOUND
- web/apps/admin/tsconfig.json — FOUND
- web/apps/admin/src/index.ts — FOUND
- web/apps/embed/package.json — FOUND
- web/apps/embed/tsconfig.json — FOUND
- web/apps/embed/src/index.ts — FOUND
- web/packages/ui/package.json — FOUND
- web/packages/ui/tsconfig.json — FOUND
- web/packages/ui/src/index.ts — FOUND
- web/pnpm-lock.yaml — FOUND

**Commits (all 3 present in git log):**
- 9cdead4 — FOUND
- 8d45e49 — FOUND
- 826d1c4 — FOUND

**Verification commands re-run (from `web/`):**
- `pnpm install --frozen-lockfile` — exit 0
- `pnpm typecheck` — exit 0 (3 successful)
- `pnpm lint` — exit 0 (No issues found)

---
*Phase: 01-foundation-polyglot-monorepo*
*Completed: 2026-05-15*
