---
phase: 01-foundation-polyglot-monorepo
plan: 08
subsystem: infra
tags: [github-actions, ci, branch-protection, codeowners, golangci-lint, pnpm, testcontainers, golang-migrate]

# Dependency graph
requires:
  - phase: 01-foundation-polyglot-monorepo
    provides: services/api/go.mod + go.sum (Plan 01), web/pnpm-lock.yaml + scaffold (Plan 02), Taskfile.yml (Plan 03), services/api/.golangci.yml (Plan 04), migrations/000001_create_scaffold.{up,down}.sql (Plan 01), docker-compose.yml (Plan 03), testcontainers-go isolation suite (Plan 07)
provides:
  - .github/workflows/ci.yml — 7 parallel jobs (6 D-30 required + 1 W-7 sanity) on every PR and push to main
  - .github/branch-protection.md — canonical doc for the manual GitHub UI configuration of required status checks (D-31)
  - .github/CODEOWNERS — wildcard routing of all reviews to @mpt-luongld
affects: [all-future-phases, phase-2-openapi, phase-3-catalog, phase-4-state-machine, phase-5-bulk-import, phase-6-admin-ui, phase-7-embed]

# Tech tracking
tech-stack:
  added:
    - actions/checkout@v4
    - actions/setup-go@v5
    - actions/setup-node@v4
    - pnpm/action-setup@v4 (version 10)
    - golangci/golangci-lint-action@v6 (linter version v1.63)
    - golang-migrate CLI v4.19.1 (Postgres tag)
    - go-task v3.40.0 (used by compose-config job)
    - postgres:17 GitHub Actions service container (migration-drift job only)
  patterns:
    - Per-job version pinning (no @latest anywhere — ASVS V14.3)
    - concurrency.cancel-in-progress for ref-scoped run replacement
    - Service container for raw migration apply (avoids testcontainers overhead where Go code path is not exercised)
    - Required-status-check enforcement as a UI-only control (documented in branch-protection.md), not in code

key-files:
  created:
    - .github/workflows/ci.yml
    - .github/branch-protection.md
    - .github/CODEOWNERS
  modified: []

key-decisions:
  - "compose-config job is NOT in the D-31 required-status-check set (D-30 locks the original 6); it runs on every push as an early-warning sanity check, intentionally non-blocking until promoted via UI edit"
  - "go-task pinned to v3.40.0 for the compose-config job's `task --list` step (not @latest — supply-chain hygiene)"
  - "CODEOWNERS routes to @mpt-luongld (resolved via `gh api user`), not @luongdev (the local git config user.name); the active GitHub account is the binding identity"
  - "migration-drift uses a GH Actions service container per Research §line 920 — D-06 same-code-path constraint applies only to suites that exercise Go code; raw `migrate up` against ephemeral Postgres is acceptable because it tests the SQL files independently"

patterns-established:
  - "Pattern: All GitHub Actions `uses:` references pinned to specific major versions (`@v4`, `@v5`, `@v6`) or specific patch versions (`@v4.19.1`, `@v3.40.0`). Future workflow additions MUST follow this convention."
  - "Pattern: Required-status-checks documented in `.github/branch-protection.md` because GitHub does not honor a `required-checks.yaml` in-code config; the doc is the canonical record and the UI is the enforcement point."
  - "Pattern: A non-required CI job (compose-config) lives alongside the 6 required jobs to surface regressions early without blocking merges; promote to required via a one-line UI edit when the team is confident."

requirements-completed:
  - FOUND-09
  - FOUND-10

# Metrics
duration: 4m 11s
completed: 2026-05-15
---

# Phase 1 Plan 8: GitHub Actions CI Gate + Branch Protection Documentation Summary

**Polyglot CI gate with 6 required parallel jobs (go-vet, go-lint, go-test with race detector and testcontainers-go isolation, web-typecheck, web-lint, migration-drift via postgres:17 service container) plus a non-blocking compose-config sanity job — all actions pinned, concurrency cancels stale runs, and branch-protection enforcement documented for the operator.**

## Performance

- **Duration:** 4m 11s
- **Started:** 2026-05-15T11:33:50Z
- **Completed:** 2026-05-15T11:38:01Z
- **Tasks:** 4 (3 source + 1 verification-only)
- **Files created:** 3

## Accomplishments

- **FOUND-09 satisfied at the code layer.** `.github/workflows/ci.yml` runs all 6 D-30-locked jobs on every PR and every push to main. The `go-test` job runs `go test -race -timeout=10m ./...` from `services/api/`, which transitively exercises the testcontainers-go isolation suite created in Plan 07 (`./test/isolation/...`). GitHub Actions Linux runners ship with Docker preinstalled, so testcontainers-go works without any setup-docker action.
- **FOUND-10 sanity automated.** Added a 7th `compose-config` job (W-7 add from Phase 1 plan-phase iteration 2) that runs `docker compose config > /dev/null` and `task --list > /dev/null` on every push. This catches regressions in `docker-compose.yml` or `Taskfile.yml` before they break a developer's machine. It is intentionally NOT in the required-status-check set — promoting it is a one-line UI edit when the team agrees.
- **D-31 operator hand-off documented.** `.github/branch-protection.md` enumerates the 6 required checks, walks the repo admin through the GitHub Settings > Branches UI setup steps, and includes a `gh api` snippet to re-verify the required-set after edits. The doc is the canonical record because required-status-checks cannot be encoded in repo code.
- **Code-review routing wired.** `.github/CODEOWNERS` has a wildcard rule routing everything to `@mpt-luongld` (the active GitHub account resolved via `gh api user`, distinct from the local `git config user.name = luongdev`). Effective only when branch-protection's "Require review from Code Owners" is enabled — documented in branch-protection.md.
- **Supply-chain hygiene enforced.** Every `uses:` reference in `ci.yml` is pinned (counted: 17 references, 0 `@latest`). The migrate CLI install pins `@v4.19.1`; go-task pins `@v3.40.0`. Verified by `! grep -qE 'uses:.*@latest' .github/workflows/ci.yml`.

## Task Commits

Each task was committed atomically on `worktree-agent-ab094ce703e6b2df0`:

1. **Task 1: Author .github/workflows/ci.yml** — `2e98d75` (feat)
2. **Task 2: Author .github/branch-protection.md** — `9df2938` (docs)
3. **Task 3: Author .github/CODEOWNERS** — `96aa16d` (chore)
4. **Task 4: Validate ci.yml syntax** — no commit (verification-only; results captured below)

Final metadata commit for SUMMARY: pending (orchestrator owns STATE/ROADMAP per worktree contract).

## Files Created/Modified

### Created

- **`.github/workflows/ci.yml`** (137 lines) — CI workflow with 7 jobs:
  - `go-vet`: `go vet ./...` in `services/api`, with `setup-go@v5` + cache-dependency-path on `services/api/go.sum`
  - `go-lint`: `golangci/golangci-lint-action@v6` working-directory `services/api`, version `v1.63`
  - `go-test`: `go test -race -timeout=10m ./...` — covers `internal/...` AND `test/isolation/...` (testcontainers suite, FOUND-08 in CI)
  - `web-typecheck`: `pnpm install --frozen-lockfile` then `pnpm -F '*' typecheck` from `web/`
  - `web-lint`: `pnpm install --frozen-lockfile` then `pnpm -F '*' lint` from `web/`
  - `migration-drift`: postgres:17 service container, installs `migrate@v4.19.1`, runs `migrate -path migrations up` then `migrate version`
  - `compose-config`: runs `docker compose config > /dev/null` and `task --list > /dev/null` (validates Taskfile.yml + docker-compose.yml syntax)
  - Top-level: `name: ci`, triggers on `pull_request` and `push: branches: [main]`, `concurrency: group: ci-${{ github.ref }} cancel-in-progress: true`
- **`.github/branch-protection.md`** (91 lines) — Documents the 6 required status checks for `main`, walks through the manual GitHub UI setup steps (Settings > Branches > Branch protection rules), explains why the 7th `compose-config` job is intentionally not required, links back to D-30 / D-31 in CONTEXT.md.
- **`.github/CODEOWNERS`** (12 lines) — Wildcard rule routing all paths to `@mpt-luongld`.

### Modified

None.

## Decisions Made

1. **Pinned `go-task` to `v3.40.0`** (not the plan's `@latest` placeholder). The plan's Task 1 action block showed `go install github.com/go-task/task/v3/cmd/task@latest` for the compose-config job. That violates the plan's own Pitfall 8 (NEVER `@latest`) and would have failed the verify-block check `! grep -qE 'uses:.*@latest'` — although `@latest` in a `run:` block isn't caught by that grep, the policy intent (supply-chain hygiene per ASVS V14.3) clearly applies. Pinned to `v3.40.0` (a recent stable release as of 2026-05-15). This is a Rule 2 deviation (missing critical functionality: version pinning for supply-chain integrity).
2. **CODEOWNERS handle resolved at runtime.** Plan Task 3 said "default to `@luongdev` and surface uncertainty"; I ran `gh api user --jq '.login'` which returned `mpt-luongld`, matching the user's CLAUDE.md PC rule. Used the resolved handle directly — no uncertainty to surface.
3. **YAML validated via Python `yaml.safe_load` (plus BaseLoader cross-check).** `actionlint` is not on this machine; auto-mode classifier denied `go install ...@latest` to fetch it. Fell back to Python YAML parsing — both `safe_load` (which coerces `on` → `True` per YAML 1.1) and `BaseLoader` (which does not) confirm the file is well-formed and contains the 7 jobs. GitHub Actions uses YAML 1.2 and handles the unquoted `on:` correctly, so this is non-blocking.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Supply-Chain Hygiene] Pinned `go-task` install version from `@latest` to `v3.40.0`**

- **Found during:** Task 1 (CI workflow authoring)
- **Issue:** Plan's `compose-config` job spec had `go install github.com/go-task/task/v3/cmd/task@latest`. The plan's own ASVS V14.3 directive and Pitfall 8 forbid `@latest`. The verify grep `! grep -qE 'uses:.*@latest'` wouldn't have caught this because `@latest` appears inside a `run:` block (not a `uses:`), but the supply-chain risk is identical (a compromised future release of `go-task` would silently execute in CI).
- **Fix:** Changed to `go install github.com/go-task/task/v3/cmd/task@v3.40.0` — a specific recent stable version.
- **Files modified:** `.github/workflows/ci.yml`
- **Verification:** `grep -c "uses:.*@latest" .github/workflows/ci.yml` → 0; `grep "task@" .github/workflows/ci.yml` → `task@v3.40.0`. No further `@latest` references anywhere in the file (`grep -c "@latest" .github/workflows/ci.yml` → 0).
- **Committed in:** `2e98d75` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 Rule 2 missing-critical-functionality)
**Impact on plan:** Strictly tightening — eliminated a contradiction between the plan's policy (no `@latest`) and one of its own example commands. No scope creep; no behavior change beyond removing supply-chain ambiguity.

## Issues Encountered

- **`actionlint` not installed locally; install denied by auto-mode classifier.** Fell back to `python3 yaml.safe_load` + a discrete static-check script (job-count >= 8, uses-count >= 8, no `@latest`) per Task 4's Option C. The classifier reason was sound: installing an unpinned `@latest` Go binary from a third-party repo to validate a file that was already statically valid is an unnecessary supply-chain risk. The Python+grep verification is sufficient for structural correctness; actionlint's semantic checks (expression syntax, action-input typing) will be exercised live on the first GitHub push, when GitHub's own action parser runs.
- **`on:` parsed as Python boolean `True`.** Known PyYAML 1.1 quirk — `safe_load` coerces unquoted `on` to `True`. Verified via `yaml.BaseLoader` that the actual textual content is `on: { pull_request: ..., push: ... }`. GitHub Actions parses YAML 1.2 and handles this correctly without quoting. Non-blocking.

## Validation Outcome

Task 4 results:

- `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"` → exit 0
- `! grep -qE 'uses:.*@latest' .github/workflows/ci.yml` → 0 hits, passes
- `! grep -q '@latest' .github/workflows/ci.yml` → 0 hits anywhere in the file
- Top-level job count (regex `^\s{2}[a-z-]+:\s*$`) → 8 (the 7 jobs + the `concurrency` block matches the regex)
- `uses:` reference count → 17 (each job has 2–4 action references)
- Required jobs from D-30 (`go-vet`, `go-lint`, `go-test`, `web-typecheck`, `web-lint`, `migration-drift`) → all 6 present
- `compose-config` job → present, runs `docker compose config > /dev/null` and `task --list > /dev/null`
- BaseLoader deep parse → all 7 jobs have `runs-on: ubuntu-latest` and a non-empty `steps:` array
- Action pinning → all 17 `uses:` references pinned to specific major versions; 0 unpinned

> **Limitation (per parent agent's instruction):** Structural validation only. The first PR push will be the live proof that GitHub Actions accepts the workflow and runs the 6 jobs in parallel. Expected cold-run wall-clock: 5–8 minutes (Go module fetch + pnpm install + Postgres service warmup + go-test with race detector).

## Self-Check: PASSED

- `.github/workflows/ci.yml` → FOUND
- `.github/branch-protection.md` → FOUND
- `.github/CODEOWNERS` → FOUND
- Commit `2e98d75` (Task 1 ci.yml) → FOUND in `git log --oneline`
- Commit `9df2938` (Task 2 branch-protection.md) → FOUND in `git log --oneline`
- Commit `96aa16d` (Task 3 CODEOWNERS) → FOUND in `git log --oneline`

## User Setup Required

None at the code level. **Operational follow-up (one-time, by a repo admin)** is documented in `.github/branch-protection.md`:

1. Push this branch to GitHub so the CI workflow runs at least once and GitHub indexes the job names.
2. Open `https://github.com/<owner>/<repo>/settings/branches` and add a protection rule for `main` with the 6 required status checks listed (and optionally "Require review from Code Owners" to activate CODEOWNERS).
3. Verify with the `gh api` snippet in `branch-protection.md`.

This is an operational control, not a code control — by design per D-31.

## Threat Flags

None. The threat model in `01-08-PLAN.md` (5 STRIDE entries: T-1-SC, T-1-08, T-1-CACHE-PREVENT, T-1-SERVICE-CONTAINER, T-1-CONCURRENCY) covers the full surface this plan introduces. No new endpoints, no new auth paths, no new file-access patterns, no schema changes — the plan only adds CI orchestration metadata.

## Next Phase Readiness

- **FOUND-09 locked.** Every PR going forward exercises the full Phase 1 surface (Go lint+test+race+isolation, web typecheck+lint, migration-drift). Phase 2 (OpenAPI Contract & Codegen) can add `oapi-codegen` to `task gen` and the existing `go-test` job will pick up generated code automatically.
- **FOUND-10 sanity automated.** Phase 2 changes to `docker-compose.yml` or `Taskfile.yml` will be caught by the compose-config job on push.
- **Phase 1 milestone status.** This plan is the last in Phase 1's wave 5 (per CONTEXT.md ordering). With Plans 01–07 already complete and 08 now landed, Phase 1 fully satisfies FOUND-01 through FOUND-10 once branch protection is enabled in the UI.
- **One operator action gates Phase 1's external completion.** Until a repo admin enables the 6 required checks per `.github/branch-protection.md`, merges can still bypass CI failures. The doc makes this explicit and provides a `gh api` re-verification path.

---
*Phase: 01-foundation-polyglot-monorepo*
*Plan: 08*
*Completed: 2026-05-15*
