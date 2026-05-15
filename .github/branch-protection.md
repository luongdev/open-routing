# Branch Protection — open-routing

Per CONTEXT.md D-31, every PR merging to `main` must pass all 6 CI jobs defined in
`.github/workflows/ci.yml`. GitHub does not honor required-checks configuration that
lives in code — it must be enabled in the repo's Settings > Branches > Branch
protection rules UI. This file is the canonical record of which checks should be
required.

> **ASVS V14.3 (CI/build pipeline integrity):** The action references in
> `.github/workflows/ci.yml` are pinned to specific major versions (no `@latest`).
> Branch protection is the operational control that prevents bypass of those
> pinned, audited checks; this document is the operational hand-off.

## Required status checks for `main`

1. `go-vet`
2. `go-lint`
3. `go-test`
4. `web-typecheck`
5. `web-lint`
6. `migration-drift`

All 6 jobs run in parallel from `.github/workflows/ci.yml` on every pull request
and push to `main`. The `concurrency.cancel-in-progress` block cancels in-flight
runs when a new commit is pushed to the same ref so stale-code merges are
impossible.

> The CI workflow also runs a 7th job, `compose-config` (W-7 sanity per Phase 1
> plan-phase iteration 2), which validates `docker-compose.yml` and `Taskfile.yml`
> parse cleanly. It is **not** in the required-status-checks list because D-30
> locks the required set to the original 6. Adding it as a 7th required check
> later is a one-line UI edit.

## Manual setup steps (one-time, by a repo admin)

1. Open `https://github.com/<owner>/<repo>/settings/branches`
2. Click "Add classic branch protection rule" (or edit the existing rule for `main`)
3. Branch name pattern: `main`
4. Check **"Require status checks to pass before merging"**
5. Check **"Require branches to be up to date before merging"**
6. In the "Status checks that are required" search box, type each of the 6 job
   names above and select them. The names appear in the picker only **after at
   least one CI run has executed against the repo** — GitHub indexes them on
   first run. If a name is missing from the picker, push a feature branch first
   and re-open this page.
7. Check **"Require a pull request before merging"** so the CODEOWNERS file
   (`.github/CODEOWNERS`) takes effect for reviewer routing.
8. Optional: enable **"Require review from Code Owners"** to make CODEOWNERS
   review mandatory instead of merely advisory.
9. Optional but recommended: enable **"Require linear history"** (no merge
   commits). Forces squash or rebase merges; keeps the main branch readable.
10. Optional: **"Restrict who can push to matching branches"** — gates direct
    push and force-push if desired.
11. Save.

## Verifying the configuration

Open any pull request. The "Checks" tab should show all 6 jobs queued or
completed. The merge button is greyed out until all 6 succeed. Failing a job
blocks the merge with the message "X / 6 required status checks have completed."

To re-verify the required-set after edits:

```bash
gh api "repos/<owner>/<repo>/branches/main/protection/required_status_checks" \
  --jq '.contexts[]'
```

The output should list exactly:

```
go-vet
go-lint
go-test
web-typecheck
web-lint
migration-drift
```

## Coverage threshold

Phase 1 does **not** enforce a code coverage minimum per D-31. Defer until Phase 3
has real catalog test surface area. Adding a coverage gate later: add a 7th job
(`coverage`) to `.github/workflows/ci.yml` and a 7th required check in the UI.

## Related docs

- `.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md` §D-30, D-31
- `.planning/phases/01-foundation-polyglot-monorepo/01-RESEARCH.md` §"Pattern 5"
- `.github/workflows/ci.yml` — the workflow definition itself
- `.github/CODEOWNERS` — code-review routing (works only when step 7 above is enabled)
