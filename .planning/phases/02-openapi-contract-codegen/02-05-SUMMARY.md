---
phase: 02-openapi-contract-codegen
plan: "05"
subsystem: ts-client-distribution
tags: [openapi-typescript, openapi-fetch, lit, codegen, typed-client, error-helpers, vitest, tdd]
dependency_graph:
  requires:
    - openapi/openapi.yaml (from plan 02-02, downgraded to OAS 3.0 in plan 02-03)
    - web/packages/ui/package.json (minimal skeleton from Phase 1)
  provides:
    - web/packages/ui/src/api/generated.ts (openapi-typescript output, drift-gated)
    - web/packages/ui/src/api/client.ts (createApiClient factory, D-38)
    - web/packages/ui/src/api/errors.ts (isApiError + parseApiError + ErrorCodes, D-39)
    - web/packages/ui/src/api/task.ts (createApiTask Lit-aware helper, D-39)
    - web/packages/ui/src/api/index.ts (barrel re-export, D-40)
    - web/packages/ui/src/index.ts (root barrel, @open-routing/ui entry)
  affects:
    - web/eslint.config.js (generated.ts lint exemption added)
    - web/packages/ui/package.json (4 runtime deps + 3 devDeps + test + gen:api scripts)
    - web/turbo.json (gen:api task added)
    - web/pnpm-lock.yaml (lockfile updated)
    - Plan 06 (CI drift gate uses pnpm -F @open-routing/ui gen:api)
    - Phase 6 (admin SPA consumes createApiClient + createApiTask from @open-routing/ui)
    - Phase 7 (embed consumes createApiClient + types; can skip task.ts)
tech_stack:
  added:
    - "openapi-typescript 7.13.0 — CLI codegen for openapi/openapi.yaml → generated.ts"
    - "openapi-fetch 0.13.8 — runtime typed fetch wrapper paired with openapi-typescript"
    - "@lit/task 1.0.3 — Lit reactive task controller (D-39 Lit-aware helper)"
    - "lit 3.3.3 — Lit runtime (Lit ecosystem peer)"
    - "@lit/reactive-element 2.1.2 — ReactiveControllerHost type (direct dep for task.ts import)"
    - "vitest 3.2.4 — unit test runner"
    - "@vitest/coverage-v8 3.2.4 — coverage adapter"
  patterns:
    - "Factory client (D-38): createApiClient returns openapi-fetch Client<paths>, getOrgId per request via Middleware"
    - "Closed enum (D-36): ErrorCodes as const record, 10 values, no open strings"
    - "Pattern S1: generated.ts committed to git, drift-gated by Plan 06"
    - "Pattern S1: generated.ts excluded from eslint via workspace eslint.config.js ignores"
    - "W-4: test fixtures use /v1/orgs/{org_id}/agents (stable CAT-01 surface), not _scaffold (D-46 deletion target)"
key_files:
  created:
    - web/packages/ui/src/api/generated.ts
    - web/packages/ui/src/api/client.ts
    - web/packages/ui/src/api/errors.ts
    - web/packages/ui/src/api/task.ts
    - web/packages/ui/src/api/index.ts
    - web/packages/ui/src/api/client.test.ts
    - web/packages/ui/src/api/errors.test.ts
  modified:
    - web/packages/ui/src/index.ts
    - web/packages/ui/package.json
    - web/turbo.json
    - web/eslint.config.js
    - web/pnpm-lock.yaml
decisions:
  - "@lit/reactive-element added as direct dep: ReactiveControllerHost is not in lit's public exports; @lit/task's type imports it from @lit/reactive-element/reactive-controller.js directly"
  - "Empty baseURL ('') invalid in Node.js test env: used http://localhost for the custom-fetch override test (Test 3)"
  - "eslint exemption at workspace root (web/eslint.config.js) rather than per-package: workspace config already existed; per-package eslint.config.js not needed"
  - "_scaffold mention in client.test.ts comments is documentation (per plan spec), not fixture usage; W-4 is satisfied — test assertions use STABLE_TEST_PATH=/v1/orgs/{org_id}/agents exclusively"
metrics:
  duration: "approx 20 minutes"
  completed: "2026-05-16"
  tasks_completed: 2
  files_created: 7
  files_modified: 5
---

# Phase 2 Plan 05: TypeScript Client Distribution Summary

**One-liner:** openapi-typescript 7.13.0 generates 3083-line typed generated.ts from openapi.yaml (deterministic); createApiClient factory (D-38) + ErrorCodes/isApiError/parseApiError (D-39, 10 codes) + createApiTask Lit helper (D-39) + barrel exports deliver the full D-40 module layout with 10 passing unit tests.

## What Was Built

### Task 1 — Install deps, gen:api script, codegen, turbo task

Extended `web/packages/ui/package.json` with:
- Runtime deps: `openapi-fetch ^0.13.0`, `@lit/task ^1.0.0`, `lit ^3.0.0`, `@lit/reactive-element ^2.1.2`
- DevDeps: `openapi-typescript ^7.0.0`, `vitest ^3.0.0`, `@vitest/coverage-v8 ^3.0.0`
- Scripts: `gen:api` + `test`
- Exports entry: `{ ".": "./src/index.ts" }`

Extended `web/turbo.json` with `gen:api` task (inputs: openapi.yaml, outputs: generated.ts).

Ran `pnpm -F @open-routing/ui gen:api` → produced `web/packages/ui/src/api/generated.ts`:
- **Line count:** 3083 lines
- **Exports:** `paths` (interface, 19 path entries), `components` (interface, all schemas), `operations` (interface, 41 operationIds)
- **Key path verified:** `"/v1/orgs/{org_id}/agents"` present at line 153

**Determinism verified:** Second `pnpm -F @open-routing/ui gen:api` run produced empty `git diff --exit-code` (exit 0).

**Resolved pinned versions in pnpm-lock.yaml:**
| Package | Resolved Version |
|---------|-----------------|
| openapi-typescript | 7.13.0 |
| openapi-fetch | 0.13.8 |
| @lit/task | 1.0.3 |
| lit | 3.3.3 |
| @lit/reactive-element | 2.1.2 |
| vitest | 3.2.4 |
| @vitest/coverage-v8 | 3.2.4 |

### Task 2 — API module files + barrel + ESLint + unit tests

**D-40 module layout delivered verbatim:**

```
web/packages/ui/src/api/
├── generated.ts   3083 lines  openapi-typescript output
├── client.ts        64 lines  createApiClient factory (D-38)
├── errors.ts        75 lines  isApiError + parseApiError + ErrorCodes (D-36 x10)
├── task.ts          37 lines  createApiTask Lit-aware helper (D-39)
└── index.ts         13 lines  barrel re-export
```

`web/packages/ui/src/index.ts` updated to `export * from './api'` — consumers now import via `@open-routing/ui` directly.

**ESLint exemption:** Added `packages/ui/src/api/generated.ts` to ignores in workspace-level `web/eslint.config.js` (Pattern S1 — generated code discipline). Per-package eslint.config.js not needed since workspace config already existed.

**Unit tests (vitest 3.2.4):**

| File | Tests | Description |
|------|-------|-------------|
| client.test.ts | 3 | createApiClient: getOrgId per request, baseURL, custom fetch override |
| errors.test.ts | 7 | isApiError (3 cases), parseApiError (3 cases), ErrorCodes count (1) |
| **Total** | **10** | All pass |

**W-4 fixture path:** `client.test.ts` uses `STABLE_TEST_PATH = '/v1/orgs/{org_id}/agents'` (CAT-01 stable surface). The `_scaffold` path referenced only in doc comments explaining why it's NOT used — Phase 3's D-46 deletion cannot break these tests.

**TS feature in generated.ts requiring tsconfig adjustment:** None. openapi-typescript 7.13.0 outputs standard TypeScript compatible with tsconfig.base.json's strict settings.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Install deps, gen:api script, run codegen, commit generated.ts | c1fefad | web/packages/ui/package.json, web/pnpm-lock.yaml, web/turbo.json, web/packages/ui/src/api/generated.ts |
| 2 | Author client.ts + errors.ts + task.ts + index.ts + root barrel + eslint exemption + unit tests | cdb3b3c | web/packages/ui/src/api/{client,errors,task,index}.ts, web/packages/ui/src/{api/client.test.ts,api/errors.test.ts,index.ts}, web/eslint.config.js, web/packages/ui/package.json, web/pnpm-lock.yaml |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ReactiveControllerHost not in lit's public exports**
- **Found during:** Task 2 typecheck — `Cannot find module '@lit/reactive-element/reactive-controller.js'`
- **Issue:** The plan spec says `import type { ReactiveControllerHost } from 'lit'` but `ReactiveControllerHost` is not exported from the `lit` package root. The `@lit/task` library's own types import it from `@lit/reactive-element/reactive-controller.js` directly.
- **Fix:** Added `@lit/reactive-element ^2.1.2` as a direct dep in package.json. Import in task.ts uses `from '@lit/reactive-element/reactive-controller.js'` (matching how `@lit/task` itself resolves it).
- **Files modified:** `web/packages/ui/package.json`, `web/pnpm-lock.yaml`
- **Commit:** cdb3b3c

**2. [Rule 1 - Bug] Empty baseURL invalid in Node.js test environment**
- **Found during:** Task 2 test run — `TypeError: Failed to parse URL from /v1/orgs/...`
- **Issue:** The plan's Test 3 passes `baseURL: ''`. openapi-fetch constructs a `new URL(path, baseUrl)` — with empty baseUrl and a relative path, this is an invalid URL in Node.js (no default origin unlike a browser).
- **Fix:** Changed Test 3 `baseURL: ''` to `baseURL: 'http://localhost'`. The test still validates that the custom fetch override is called and the global fetch is NOT called — the behavior assertion is unchanged.
- **Files modified:** `web/packages/ui/src/api/client.test.ts`
- **Commit:** cdb3b3c

**3. [Rule 1 - Bug] Unused parameter in test stub function**
- **Found during:** Task 2 lint run — `'_input' is defined but never used`
- **Issue:** Test 3's `stubFetch` declared `_input: Request` parameter but it was unused (the test only asserts on `stubFetch` call count, not on the request).
- **Fix:** Removed the typed parameter from the arrow function (the function still receives the argument, it just doesn't bind it to a name).
- **Files modified:** `web/packages/ui/src/api/client.test.ts`
- **Commit:** cdb3b3c

## Threat Flags

None. The files created introduce no new network endpoints, no new auth paths, no file access patterns, and no schema changes at trust boundaries. The trust surface changes are:

- New npm packages (supply chain): All classified TRUSTED in plan threat register. Exact versions pinned in pnpm-lock.yaml. `pnpm install --frozen-lockfile` enforces lockfile in CI.
- `generated.ts`: Committed to git; Plan 06 drift gate enforces no-diff.
- `createApiClient`: Factory only — no singleton, no global state. `getOrgId` override is type-safe.

## Known Stubs

None. All five module files are fully implemented and wired:
1. `generated.ts` — live codegen output, deterministic
2. `client.ts` — factory with X-Org-Id middleware, per-request getOrgId
3. `errors.ts` — all 3 helpers + 10 ErrorCodes
4. `task.ts` — wraps @lit/task
5. `index.ts` — barrel re-exporting everything above

## Self-Check: PASSED

- [x] `generated.ts` exists: `web/packages/ui/src/api/generated.ts`
- [x] Line count >= 200: 3083 lines
- [x] `paths` exported: line 6
- [x] `components` exported: line 696
- [x] `operations` exported: line 1498
- [x] `/v1/orgs/{org_id}/agents` in generated.ts: line 153
- [x] `client.ts` exists with createApiClient, getOrgId, baseURL
- [x] `errors.ts` exists with isApiError, parseApiError, ErrorCodes (10 values)
- [x] `task.ts` exists with createApiTask
- [x] `api/index.ts` re-exports from client, errors, task, generated
- [x] `src/index.ts` has `export * from './api'`
- [x] `web/eslint.config.js` has `generated.ts` in ignores
- [x] Task 1 commit `c1fefad` exists in git log
- [x] Task 2 commit `cdb3b3c` exists in git log
- [x] `pnpm -F @open-routing/ui gen:api` deterministic (second run empty diff)
- [x] `pnpm -F @open-routing/ui typecheck` exits 0
- [x] `pnpm -F @open-routing/ui lint` exits 0
- [x] `pnpm -F @open-routing/ui test` exits 0 (10/10 tests pass)
- [x] `pnpm -F '*' typecheck` exits 0 (admin + embed unaffected)
- [x] `pnpm -F '*' lint` exits 0
- [x] W-4: client.test.ts uses STABLE_TEST_PATH=/v1/orgs/{org_id}/agents (not _scaffold)
- [x] ErrorCodes has exactly 10 D-36 codes
