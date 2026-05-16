---
phase: 02-openapi-contract-codegen
plan: 06
subsystem: infra
tags: [github-actions, redocly, oapi-codegen, openapi-typescript, ci, drift-gate, CONTRACT-04]

# Dependency graph
requires:
  - phase: 02-03
    provides: Go oapi-codegen pipeline — services/api/internal/api/*.gen.go via go generate ./internal/api/...
  - phase: 02-04
    provides: strict-server wiring into NewMux; /openapi.yaml + /docs runtime handlers
  - phase: 02-05
    provides: TS client distribution layer — web/packages/ui/src/api/generated.ts via pnpm gen:api

provides:
  - codegen-drift GitHub Actions job that enforces CONTRACT-04 on every PR and push to main
  - Redocly recommended lint gate (D-48) on openapi/openapi.yaml BEFORE any codegen runs
  - git diff --exit-code gate covering services/api/internal/api/ AND web/packages/ui/src/api/generated.ts
  - All Phase 2 codegen surface enforced under CI — no spec change can merge without regenerating both Go + TS

affects: [phase-03, phase-04, phase-05, phase-06, phase-07]

# Tech tracking
tech-stack:
  added:
    - "@redocly/cli@1.25.0 (npx-invoked in CI — no package.json dep)"
  patterns:
    - "Redocly lint BEFORE codegen (D-48 order lock) — spec must be valid before regen runs"
    - "Separate CI job for drift detection (D-47) — drift failure is its own red check, not buried in go-test or web-typecheck"
    - "task gen as canonical codegen entry point in CI — matches local developer workflow"
    - "git diff --exit-code with explicit -- path separator to scope diff to codegen outputs"

key-files:
  created: []
  modified:
    - .github/workflows/ci.yml

key-decisions:
  - "Redocly pinned to @1.25.0 (not @latest) per ASVS V14.3 no-@latest rule already in ci.yml header"
  - "npx --yes flag used for Redocly CLI invocation to suppress interactive install prompt in CI headless environment"
  - "codegen-drift job has no needs: dependency on other jobs — runs in parallel for independent red check signal (D-47)"
  - "Branch-protection marking codegen-drift as a required check is a separate operator step in GitHub repo Settings > Branches — not part of this plan"

patterns-established:
  - "Separate drift-gate job pattern: lint spec → regen → diff --exit-code (reusable for any future codegen pipeline)"

requirements-completed:
  - CONTRACT-04

# Metrics
duration: 8min
completed: 2026-05-16
---

# Phase 02 Plan 06: Codegen Drift CI Gate Summary

**codegen-drift GitHub Actions job enforcing CONTRACT-04: Redocly lint on openapi.yaml then go+TS codegen then git diff --exit-code gate**

## Performance

- **Duration:** 8 min
- **Started:** 2026-05-16T02:24:00Z
- **Completed:** 2026-05-16T02:32:25Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Added `codegen-drift` job to `.github/workflows/ci.yml` as the 8th top-level job after `compose-config`
- Redocly recommended lint runs BEFORE codegen (D-48 order locked): any spec lint failure blocks the PR before codegen is attempted
- `task gen` invoked via `$(go env GOPATH)/bin/task gen` — runs sqlc + `go generate ./internal/api/...` + `pnpm -F @open-routing/ui gen:api`
- `git diff --exit-code -- services/api/internal/api/ web/packages/ui/src/api/generated.ts` fails the job on any drift (CONTRACT-04)
- All action versions pinned: `actions/checkout@v4`, `actions/setup-go@v5`, `pnpm/action-setup@v4`, `actions/setup-node@v4`; `@redocly/cli@1.25.0` pinned (never @latest)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add codegen-drift job to .github/workflows/ci.yml** - `3be759f` (feat)

**Plan metadata:** (committed after self-check below)

## Output Spec Confirmations (per plan `<output>` block)

1. **Redocly lint runs BEFORE codegen (D-48):** Confirmed — step `redocly lint openapi.yaml` appears before `install task CLI` and `task gen` in the job's step list. `awk` line-order check passes.
2. **Exact Redocly CLI version pinned:** `@redocly/cli@1.25.0` — pinned to specific patch version per ASVS V14.3.
3. **Two paths covered by git diff verbatim:**
   - `services/api/internal/api/`
   - `web/packages/ui/src/api/generated.ts`
4. **Branch-protection configuration note:** Marking `codegen-drift` as a required check is a **separate operator step** in GitHub repo Settings > Branches > Branch protection rules. The job appears as a pass/fail check immediately on the next PR but does not block merge until an operator adds it to the required-status-checks list.

## Files Created/Modified

- `.github/workflows/ci.yml` — Added 55-line `codegen-drift` job block after `compose-config`

## Decisions Made

- Pinned `@redocly/cli` to `1.25.0` (the current stable 1.x release at execution time). The plan noted "pin to latest stable 1.x at execution time" — verified at https://www.npmjs.com/package/@redocly/cli; 1.25.0 is the current stable release.
- Used `npx --yes` to suppress the interactive install prompt that npx shows in CI headless environments (without `--yes`, npx would pause for confirmation).
- The `codegen-drift` job has NO `needs:` entry — it is fully independent, running in parallel with all other 7 jobs. This gives the drift failure its own red check signal (D-47 explicit requirement).

## Deviations from Plan

None - plan executed exactly as written. The YAML excerpt in the plan's `<action>` block was used verbatim.

## Issues Encountered

None. The existing ci.yml patterns (setup-go, pnpm/node, task CLI install) mapped directly to the new job with no resolution needed.

## Known Stubs

None. This plan delivers a CI workflow job, not application code. No stubs or placeholder values.

## Threat Flags

None. The new job introduces no new network endpoints, auth paths, file access patterns, or schema changes. The supply chain threats (T-02-SC-CLI, T-02-SC-TASK) documented in the plan's `<threat_model>` are already addressed by pinning action and tool versions.

## User Setup Required

**Optional operator action (not blocking):** To make the `codegen-drift` check required on PRs, add it to the branch protection rule:

1. Go to GitHub repo Settings > Branches > Branch protection rules for `main`
2. Under "Require status checks to pass before merging", search for `codegen-drift`
3. Add it to the required checks list

Until this is done, the job runs and shows as pass/fail but does not block merge.

## Next Phase Readiness

All 6 Phase 2 plans are now complete:
- **02-01:** UI sketches (contract validation artifacts)
- **02-02:** OpenAPI 3.1 spec authored (CONTRACT-01)
- **02-03:** Go strict-server codegen pipeline (CONTRACT-02)
- **02-04:** Strict-server wired into NewMux + /openapi.yaml + /docs (CONTRACT-03)
- **02-05:** TS client distribution layer (openapi-typescript + openapi-fetch + Lit helper)
- **02-06:** CI drift gate (CONTRACT-04) — this plan

Phase 2 moves to verification. The `codegen-drift` job will be green on the next PR that carries the clean committed generated files from Plans 03 + 05.

---
*Phase: 02-openapi-contract-codegen*
*Completed: 2026-05-16*
