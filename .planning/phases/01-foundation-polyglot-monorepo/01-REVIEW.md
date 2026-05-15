---
phase: 01-foundation-polyglot-monorepo
reviewed: 2026-05-15T13:03:15Z
depth: standard
files_reviewed: 63
files_reviewed_list:
  - .editorconfig
  - .env.example
  - .gitattributes
  - .github/CODEOWNERS
  - .github/branch-protection.md
  - .github/workflows/ci.yml
  - README.md
  - Taskfile.yml
  - docker-compose.yml
  - migrations/000001_create_scaffold.down.sql
  - migrations/000001_create_scaffold.up.sql
  - openapi/.gitkeep
  - services/api/.air.toml
  - services/api/.golangci.yml
  - services/api/cmd/api/main.go
  - services/api/cmd/migrate/main.go
  - services/api/go.mod
  - services/api/go.sum
  - services/api/internal/config/config.go
  - services/api/internal/db/bypass.go
  - services/api/internal/db/generated/db.go
  - services/api/internal/db/generated/models.go
  - services/api/internal/db/generated/scaffold.sql.go
  - services/api/internal/db/orgdb.go
  - services/api/internal/db/orgdb_test.go
  - services/api/internal/db/orgkey/orgkey.go
  - services/api/internal/db/orgkey/orgkey_test.go
  - services/api/internal/db/pool.go
  - services/api/internal/db/queries/scaffold.sql
  - services/api/internal/db/sqlcheck.go
  - services/api/internal/db/sqlcheck_test.go
  - services/api/internal/middleware/httputil.go
  - services/api/internal/middleware/orgcontext.go
  - services/api/internal/middleware/orgcontext_test.go
  - services/api/internal/middleware/requestid.go
  - services/api/internal/middleware/requestid_test.go
  - services/api/internal/scaffold/handler.go
  - services/api/internal/scaffold/handler_test.go
  - services/api/internal/server/health.go
  - services/api/internal/server/health_test.go
  - services/api/internal/server/server.go
  - services/api/internal/server/server_test.go
  - services/api/scripts/smoke.sh
  - services/api/sqlc.yaml
  - services/api/test/isolation/main_test.go
  - services/api/tools.go
  - web/.gitignore
  - web/.npmrc
  - web/apps/admin/package.json
  - web/apps/admin/src/index.ts
  - web/apps/admin/tsconfig.json
  - web/apps/embed/package.json
  - web/apps/embed/src/index.ts
  - web/apps/embed/tsconfig.json
  - web/eslint.config.js
  - web/package.json
  - web/packages/ui/package.json
  - web/packages/ui/src/index.ts
  - web/packages/ui/tsconfig.json
  - web/pnpm-lock.yaml
  - web/pnpm-workspace.yaml
  - web/tsconfig.base.json
  - web/turbo.json
findings:
  critical: 1
  warning: 3
  info: 0
  total: 4
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-05-15T13:03:15Z
**Depth:** standard
**Files Reviewed:** 63
**Status:** issues_found

## Summary

Reviewed the Phase 01 source/config scope resolved from SUMMARY artifacts. The earlier broad SQL-inspector problems are mostly fixed: projection-only org_id and DDL now reject, and QueryRow returns an errRow in ValidationError mode. Remaining blockers are narrower: SQLChecker still accepts unsafe queries when org_id is present only in a nested subquery or only on one alias in a multi-tenant join. CI/local lint reproducibility and test-failure signaling also need cleanup.

Verification run during review:
- `go test -short ./...` in `services/api`: passed, 45 tests.
- `go test -race -timeout=10m ./...` in `services/api`: passed, 60 tests.
- `go vet ./...` in `services/api`: passed.
- `pnpm -F '*' lint` in `web`: passed.
- `pnpm -F '*' typecheck` in `web`: passed via `rtk proxy`.
- `docker compose config`: passed.
- `golangci-lint run`: failed locally because installed `golangci-lint` is built with Go 1.23 while repo targets Go 1.25.10.

## Critical Issues

### CR-01: SQLChecker Counts Nested org_id As Outer Query Scoping

**File:** `services/api/internal/db/sqlcheck.go:169`
**Issue:** `whereContainsOrgIDRef` treats an `org_id` reference inside a `SubLink` as satisfying the parent statement's org filter. That allows unsafe outer queries such as:

```sql
SELECT id, org_id, external_id
FROM _scaffold
WHERE EXISTS (
  SELECT 1 FROM _scaffold s2 WHERE s2.org_id = $1
);
```

If the subquery has at least one row for the caller's org, the outer `_scaffold` scan is unscoped and can return rows from every org. The same "any org_id anywhere in WHERE is enough" shape also fails for joins where only one tenant table alias is scoped but another alias is selected.

**Fix:** Do not let subquery predicates satisfy the enclosing statement. Classify each statement scope separately: collect tenant table aliases from the top-level `FROM`/join tree, then require a top-level predicate binding each tenant alias to `org_id`. Add rejection tests for subquery-only and partially scoped join cases.

## Warnings

### WR-01: Scaffold Integration TestMain Exits 0 On Setup Failures

**File:** `services/api/internal/scaffold/handler_test.go:75`
**Issue:** The scaffold integration suite exits with status 0 when the Postgres testcontainer, connection string, migration setup, or pgxpool setup fails. In CI, `go test -race -timeout=10m ./...` can therefore pass without exercising this package's integration assertions if Docker/image pull/migration setup breaks. The isolation suite already has a CI-aware `ciExitCode`; this suite does not.

**Fix:** Mirror `services/api/test/isolation/main_test.go` and exit 1 when `CI` is set. Keep exit 0 only for local docker-less runs.

### WR-02: golangci-lint Version Is Ambient And Currently Fails Against Go 1.25.10

**File:** `Taskfile.yml:60`
**Issue:** `task lint` calls whichever `golangci-lint` binary is on PATH, while the repo targets `toolchain go1.25.10` in `services/api/go.mod`. On this machine, `golangci-lint version` reports `v1.63.4 built with go1.23.4`, and `golangci-lint run` fails before loading config. The CI workflow also leaves the action's linter version implicit, so the lint gate is not reproducible over time.

**Fix:** Pin a linter version that supports the repo Go toolchain and install/use that exact version in both CI and local tasks. For example, set `version:` in `golangci/golangci-lint-action@v6` and add a Taskfile/bootstrap path that installs the same version.

### WR-03: Migration Binary Requires Redis Configuration It Does Not Use

**File:** `services/api/cmd/migrate/main.go:24`
**Issue:** `cmd/migrate` calls `config.MustLoad()`, which requires both `DATABASE_URL` and `REDIS_URL`, but the migration runner only uses `DatabaseURL`. That can block migrations in environments where Postgres must be migrated before Redis is provisioned or configured, even though Redis is irrelevant to schema migration.

**Fix:** Add a migration-specific config loader that requires only `DATABASE_URL`, or split `config.Load` into shared base fields and API-only dependency fields. Keep the API binary's Redis requirement unchanged.

---

_Reviewed: 2026-05-15T13:03:15Z_
_Reviewer: the agent (gsd-code-reviewer inline)_
_Depth: standard_
