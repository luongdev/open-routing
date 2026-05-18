---
phase: 06-shared-ui-library-standalone-admin
plan: "02"
subsystem: packages/ui (Wave 1 infrastructure)
tags:
  - ajv-standalone
  - codegen
  - validators
  - themes
  - i18n
  - import-api
  - barrel
  - ADMIN-04

dependency_graph:
  requires: []
  provides:
    - web/packages/ui/src/validators/ (13 ajv ESM validators)
    - web/packages/ui/src/themes/ (or-light/or-dark/or-brand CSS + TS token maps)
    - web/packages/ui/src/api/import.ts (createImporter — ADMIN-04 gate)
    - web/packages/ui/src/api/errors.ts (extended ErrorCodes + ERROR_I18N_KEYS)
    - web/packages/ui/lit-localize.json + xliff/ seeds
    - web/packages/ui/src/locales/locale-codes.ts
    - web/packages/ui/src/components/ stub barrels
  affects:
    - Any plan that imports from @open-routing/ui validators subpath
    - Any plan that imports from @open-routing/ui themes subpath
    - Plan 06-13 (import page uses createImporter)
    - All Wave 2+ entity component plans (stub barrels must be expanded)

tech_stack:
  added:
    - ajv@^8.20.0 (dev) — programmatic ESM validator codegen
    - ajv-formats@^3.0.0 (dev) — email/uuid/date-time format validators
    - js-yaml@^4.1.0 (dev) — parse openapi.yaml in Node scripts
  patterns:
    - ajv v8 programmatic standalone codegen (NOT ajv-cli — stale/dead)
    - normalizeNullable() pre-processor converts OpenAPI nullable:true → JSON Schema type:[X,"null"]
    - @ts-nocheck on generated validator files (ajv output is JS-style, not TS-typed)
    - createImporter factory pattern mirrors createApiClient (injectable deps for testing)
    - ImportError class wraps status + body (ApiError is a type alias, not a class)
    - ESLint ignores src/validators/** (same as generated.ts pattern)

key_files:
  created:
    - web/packages/ui/scripts/gen-validators.mjs
    - web/packages/ui/src/validators/CreateAgentRequest.ts (+ 12 others)
    - web/packages/ui/src/validators/index.ts
    - web/packages/ui/src/themes/or-light.css
    - web/packages/ui/src/themes/or-dark.css
    - web/packages/ui/src/themes/or-brand.css
    - web/packages/ui/src/themes/index.ts
    - web/packages/ui/src/api/import.ts
    - web/packages/ui/src/api/import.test.ts
    - web/packages/ui/lit-localize.json
    - web/packages/ui/xliff/en.xliff
    - web/packages/ui/xliff/vi.xliff
    - web/packages/ui/src/locales/locale-codes.ts
    - web/packages/ui/src/components/index.ts (+ 8 subdirectory stubs)
  modified:
    - web/packages/ui/src/api/errors.ts (added 5 ErrorCodes + ERROR_I18N_KEYS map)
    - web/packages/ui/src/api/errors.test.ts (updated count expectation 10→15)
    - web/packages/ui/src/api/index.ts (added ERROR_I18N_KEYS, createImporter exports)
    - web/packages/ui/src/index.ts (extended barrel with components/validators/api/import)
    - web/packages/ui/package.json (gen:validators script + devDeps + exports map)
    - web/eslint.config.js (added src/validators/** to ignores)
    - web/pnpm-lock.yaml (updated for new devDeps)

decisions:
  - "createImporter returns BulkImportResult for 200, 207, AND 422 per openapi.yaml spec (Codex cross-AI finding)"
  - "ajv standalone output uses @ts-nocheck + eslint ignore — generated JS-style code is correct at runtime"
  - "normalizeNullable() pre-processes OpenAPI schemas before addSchema() to fix nullable:true validation"
  - "ImportError class created locally; ApiError is type-only alias and cannot be constructed"
  - "import.ts URL uses encodeURIComponent(orgId) + trailing-slash stripping for defensive URL construction"

metrics:
  duration: ~70 minutes
  completed: "2026-05-17T18:38:00Z"
  tasks_completed: 3
  files_created: 27
  files_modified: 7
  commits: 5
  tests_added: 10
  tests_total_passing: 20
---

# Phase 6 Plan 02: AJV Standalone Codegen + Themes + Lit-Localize + createImporter Summary

ajv v8 programmatic standalone codegen (13 ESM validators) + 3 theme CSS token files (or-light/or-dark/or-brand) + createImporter typed wrapper (ADMIN-04 gate) + lit-localize i18n config/XLIFF seeds + ErrorCodes extension + packages/ui barrel expansion

## Objective

Set up Wave 1 infrastructure that all later plans depend on: ajv-standalone validator codegen from OpenAPI, Shoelace theme tokens, @lit/localize i18n config, typed bulk-import API wrapper (ADMIN-04 compliance), and extended packages/ui public API.

## What Was Built

### Task 1: AJV Validator Codegen + Theme CSS Files

**scripts/gen-validators.mjs** — ajv v8 programmatic standalone codegen pipeline:
- Uses the programmatic API (NOT ajv-cli — stale/dead as of 2021)
- Seeds ALL `#/components/schemas/*` entries via `ajv.addSchema()` BEFORE compiling any target schema — prevents MissingRefError on $ref-bearing schemas (CreateAgentRequest → AgentSkillAssignment, CreateChannelRequest → ChannelType)
- `normalizeNullable()` pre-processor converts OpenAPI 3.0 `nullable: true` to JSON Schema `type: [X, "null"]` — fixes validation of nullable fields like `external_id`, `default_queue_id`, `break_reason_id`
- Generates 13 ESM validators + `src/validators/index.ts` barrel
- Each generated file gets `// @ts-nocheck` header (ajv output is JS-style code)

**Theme CSS files** (exact values from UI-SPEC §2.2):
- `or-light.css` — `--sl-color-primary-500: #2b8a93` (mid-tone teal, hue=185, sat=43%)
- `or-dark.css` — `--sl-color-primary-500: #4faab2` (lighter teal for dark surfaces)
- `or-brand.css` — `--sl-color-primary-500: #0d8b96` (deeper brand teal, hue=190, sat=60%)
- `src/themes/index.ts` — TypeScript token maps (orLight, orDark, orBrand) + ThemeName/Theme types for `<or-catalog-shell>._applyTheme()`

### Task 2: createImporter Typed Wrapper — ADMIN-04 Compliance (TDD)

**src/api/import.ts** — single gateway for POST /v1/orgs/{orgId}/catalog/import:
- `createImporter(deps)` factory returns bound `importCatalog(args)` function
- X-Org-Id header from `deps.getOrgId()` (invoked per-request, not at construction)
- Content-Type from `args.contentType` (text/csv or application/json)
- `schema_version` query param appended for CSV + schemaVersion only
- `Idempotency-Key` header set only when `args.idempotencyKey` present (D5-13)
- Returns BulkImportResult for 200 (success), 207 (partial), AND 422 (all-failed)
- Throws ImportError(status, body) for 400, 413, 500+
- `fetchImpl` injectable for unit testing
- URL construction: `encodeURIComponent(orgId)` + trailing-slash stripping

**src/api/import.test.ts** — 10 tests covering full HTTP contract:
- X-Org-Id injection, Content-Type for CSV/JSON, schema_version gate
- Idempotency-Key set/omitted, 200/207/422 success, 413/400 error throws

### Task 3: i18n Config, XLIFF Seeds, ErrorCodes Extension, Barrel

- `lit-localize.json` — runtime mode, sourceLocale: en, targetLocales: [vi], XLIFF interchange
- `xliff/en.xliff` + `xliff/vi.xliff` — initial empty XLIFF 1.2 seeds
- `src/locales/locale-codes.ts` — sourceLocale/targetLocales constants
- `errors.ts` — 5 new error codes: DUPLICATE_CODE, DUPLICATE_EXTERNAL_ID, IMMUTABLE_FIELD, INVALID_REFERENCE, INVALID_VALUE (Phase 04.1 / D6-24)
- `errors.ts` — ERROR_I18N_KEYS map covering all 15 ErrorCode values
- `src/index.ts` — barrel extended with `components`, `validators`, `api/import` exports
- `package.json` — `gen:validators` script + 14 subpath exports entries for tree-shaking
- `src/components/` — stub index.ts files for agents, skills, queues, channels, adapters, break-reasons, shell, primitives (populated by Wave 2+ plans)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ESLint ignores for generated validators**
- **Found during:** Task 3 (typecheck step)
- **Issue:** ajv standalone output files have `// @ts-nocheck` + JS-style code that fails ESLint
- **Fix:** Added `packages/ui/src/validators/**` to `web/eslint.config.js` ignores (same pattern as generated.ts)
- **Files modified:** `web/eslint.config.js`
- **Commit:** 092e0f5

**2. [Rule 1 - Bug] TypeScript errors in generated validator files**
- **Found during:** Task 3 (typecheck step)
- **Issue:** ajv standalone code uses `any` parameters and union-incompatible error shapes
- **Fix:** Prepend `// @ts-nocheck` to each generated file in gen-validators.mjs
- **Files modified:** `gen-validators.mjs` + regenerated 13 validator files
- **Commit:** efa8d61 → regenerated in 092e0f5

**3. [Rule 1 - Bug] src/components/index.ts not a module**
- **Found during:** Task 3 (typecheck step)
- **Issue:** Empty file with only a comment is not a TypeScript module
- **Fix:** Added `export {};` to make it a valid TS module
- **Files modified:** `src/components/index.ts`
- **Commit:** efa8d61

**4. [Rule 1 - Bug] errors.test.ts hardcoded count expected 10 codes, now has 15**
- **Found during:** Task 3 (test run)
- **Issue:** Test expected exactly 10 ErrorCodes; Task 3 added 5 more
- **Fix:** Updated test description and expected count from 10→15
- **Files modified:** `src/api/errors.test.ts`
- **Commit:** efa8d61

### Cross-AI Review Findings Addressed (2026-05-18)

**5. [Rule 1 - Bug] 422 contract drift (Codex HIGH)**
- **Issue:** createImporter threw ImportError for 422, but openapi.yaml specifies 422 returns BulkImportResult (all-rows-failed case)
- **Fix:** Added `resp.status === 422` to the BulkImportResult return path; added test for 422 case
- **Commit:** 092e0f5

**6. [Rule 1 - Bug] Nullable allOf fields rejected by validators (Codex HIGH / Gemini MED)**
- **Issue:** `nullable: true` in OpenAPI 3.0 not honored by ajv; null values for fields like `external_id`, `default_queue_id` would fail validation
- **Fix:** Added `normalizeNullable()` pre-processor in gen-validators.mjs that converts `nullable: true + type: X → type: [X, "null"]` before schema registration
- **Commit:** 092e0f5

**7. [Rule 2 - Security] Defensive URL construction (Gemini MED/LOW)**
- **Issue:** baseURL with trailing slash produces double-slash; orgId not URL-encoded
- **Fix:** `deps.baseURL.replace(/\/$/, '')` + `encodeURIComponent(orgId)` in createImporter
- **Commit:** 092e0f5

## Known Stubs

The following are intentional stubs that will be populated by later plans:

- `src/components/index.ts` and all 8 subdirectory index.ts files: empty stubs; Wave 2+ plans implement entity components
- `xliff/en.xliff` and `xliff/vi.xliff`: empty XLIFF seeds; populated by `lit-localize extract` when msg() calls exist in components

These stubs do NOT prevent this plan's goal from being achieved. The barrel exports `export * from './components'` which re-exports nothing initially — this is correct and intentional.

## Threat Flags

None. No new network endpoints or auth paths introduced. The `createImporter` gateway is the planned API surface for POST /catalog/import — not a new trust boundary introduction.

## Self-Check: PASSED

Files verified:
- [x] web/packages/ui/scripts/gen-validators.mjs exists
- [x] web/packages/ui/src/validators/ contains 14 files (13 validators + index.ts)
- [x] web/packages/ui/src/themes/or-light.css contains --sl-color-primary-500: #2b8a93
- [x] web/packages/ui/src/themes/or-dark.css contains --sl-color-primary-500: #4faab2
- [x] web/packages/ui/src/themes/or-brand.css contains --sl-color-primary-500: #0d8b96
- [x] web/packages/ui/src/api/import.ts exports createImporter
- [x] web/packages/ui/lit-localize.json exists
- [x] web/packages/ui/xliff/en.xliff and vi.xliff exist
- [x] web/packages/ui/src/api/errors.ts contains DUPLICATE_CODE and ERROR_I18N_KEYS
- [x] 20 tests pass (19 existing + 10 import tests, with errors.test count updated)
- [x] typecheck exits 0

Commits verified:
- [x] 6b5939c — Task 1 (ajv codegen + themes)
- [x] 6279821 — Task 2 RED (failing import tests)
- [x] 1c48c54 — Task 2 GREEN (import.ts implementation)
- [x] efa8d61 — Task 3 (i18n + ErrorCodes + barrel)
- [x] 092e0f5 — Cross-AI review fixes (nullable, 422, ESLint, URL)
