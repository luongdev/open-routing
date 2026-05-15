---
phase: 01-foundation-polyglot-monorepo
plan: "09"
subsystem: orgdb-sql-inspector
tags: [security, sql-checker, orgdb, gap-closure]
dependency_graph:
  requires: [01-03]
  provides: [hardened-sql-inspector, errrow-queryrow]
  affects: [services/api/internal/db]
tech_stack:
  added: []
  patterns:
    - WHERE-predicate AST walk via pg_query_go v6 ColumnRef/BoolExpr/AExpr/SubLink
    - errRow adapter implementing pgx.Row for ValidationError mode
key_files:
  created:
    - services/api/internal/db/orgdb_test.go
  modified:
    - services/api/internal/db/sqlcheck.go
    - services/api/internal/db/sqlcheck_test.go
    - services/api/internal/db/orgdb.go
decisions:
  - "Walk only the WHERE predicate subtree (not full AST) to prevent false positives from projection/ORDER BY/JOIN ON/literal org_id references"
  - "DDL default case returns ErrSQLMissingOrgFilter (fail-closed) — bypass short-circuits before inspector so DDL reaching the inspector means bypass is absent"
  - "errRow implements pgx.Row returning the stored preflight error at Scan time — avoids panic in ValidationError mode while satisfying the pgx.Row interface"
metrics:
  duration: "~2 minutes"
  completed: "2026-05-15T12:54:00Z"
  tasks_completed: 4
  files_changed: 4
requirements_closed: [FOUND-04]
---

# Phase 01 Plan 09: orgDB SQL Inspector Hardening Summary

**One-liner:** WHERE-predicate AST walk replacing strings.Contains; errRow adapter for non-panicking QueryRow in ValidationError mode.

## What Was Accomplished

Three reviewer-flagged vulnerabilities (Codex HIGH-1, HIGH-2, HIGH-3-partial; Gemini MEDIUM) in the orgDB SQL inspector were closed:

**Task 1 — sqlcheck.go: WHERE-predicate walk**

Replaced `containsOrgIDColumnRef` (which called `strings.Contains` on the full serialised protobuf AST text) with two typed functions:

- `isOrgIDColumnRef(*pg_query.Node_ColumnRef) bool` — checks the last field of the ColumnRef against "org_id", correctly handling table-qualified refs like `a.org_id`.
- `whereContainsOrgIDRef(*pg_query.Node) bool` — recursively walks only the WHERE clause subtree (ColumnRef, BoolExpr AND/OR/NOT, AExpr comparison, SubLink subquery WHERE), returning true only when org_id appears as a WHERE predicate.

The `classify()` function was restructured to extract the WHERE clause from each statement type via a nested type switch, and the `default` case was changed to return `ErrSQLMissingOrgFilter` (fail-closed DDL rejection). The now-unused `"strings"` import was removed.

**Task 2 — sqlcheck_test.go: hardened test coverage**

Updated `TestSQLChecker_MustContainOrgFilter`:
- Moved `DDL_AcceptedAsBypass` from accept to reject (DDL rejected by inspector; bypass short-circuits before)
- Moved `JoinedOrgID` (renamed `JoinWithoutWhereOrgID`) from accept to reject (org_id in JOIN ON only, not WHERE)
- Added `JoinWithWhereOrgID` to accept (join with WHERE org_id predicate — correct)
- Added 5 new reject cases:
  - `ProjectionOnlyOrgID`: `SELECT id, org_id FROM _scaffold` — SELECT-list ref, not WHERE
  - `OrderByOrgID`: `SELECT id FROM _scaffold ORDER BY org_id` — ORDER BY ref, not WHERE
  - `LiteralOrgIDString`: `SELECT 'org_id' AS alias FROM _scaffold` — A_Const.sval, not ColumnRef
  - `DropTable`: `DROP TABLE _scaffold` — DDL without bypass
  - `TruncateTable`: `TRUNCATE _scaffold` — DDL without bypass

**Task 3 — orgdb.go + orgdb_test.go: errRow and non-panicking QueryRow**

Added `errRow` struct implementing `pgx.Row` (Scan returns stored error). Updated `QueryRow` to panic only in `ValidationPanic` mode and return `errRow` in `ValidationError` mode. Created `orgdb_test.go` with `TestOrgDB_QueryRow_ValidationError_NoOrg` proving no panic on missing-ctx-org_id with nil pool.

**Task 4 — Full suite regression**

`go test -count=1 -race ./internal/db/...`: 23 tests passed across 3 packages. `go vet ./...`: no issues.

## Commits

| Hash      | Message                                                                |
|-----------|------------------------------------------------------------------------|
| `3b7cbf9` | fix(sqlcheck): require org_id in WHERE clause; reject DDL without bypass |
| `02fde35` | fix(orgdb): QueryRow returns errRow in ValidationError mode instead of panic |

## Files Modified

| File | Change |
|------|--------|
| `services/api/internal/db/sqlcheck.go` | Replace containsOrgIDColumnRef with isOrgIDColumnRef + whereContainsOrgIDRef; remove "strings" import; DDL default case rejects |
| `services/api/internal/db/sqlcheck_test.go` | Move 2 cases to reject, add 5 new reject cases, add JoinWithWhereOrgID accept case |
| `services/api/internal/db/orgdb.go` | Add errRow type; QueryRow panics only in ValidationPanic, returns errRow in ValidationError |
| `services/api/internal/db/orgdb_test.go` | Created — TestOrgDB_QueryRow_ValidationError_NoOrg |

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes introduced.

## Must-Haves Verification

| Truth | Status |
|-------|--------|
| `SELECT id, org_id FROM _scaffold` rejected (no WHERE predicate) | PASS — ProjectionOnlyOrgID in reject list, test passes |
| `SELECT 'org_id' AS alias FROM _scaffold` rejected (literal not ColumnRef) | PASS — LiteralOrgIDString in reject list, test passes |
| `DROP TABLE _scaffold` rejected (DDL without bypass) | PASS — DropTable in reject list, test passes |
| `TRUNCATE _scaffold` rejected (DDL without bypass) | PASS — TruncateTable in reject list, test passes |
| OrgDB.QueryRow in ValidationError mode returns typed error from Scan, no panic | PASS — errRow returned; TestOrgDB_QueryRow_ValidationError_NoOrg passes |
| All existing accept cases still pass | PASS — 23 tests pass with -race |
| `go test -count=1 ./internal/db/...` exits 0 | PASS |
| `go vet ./...` exits 0 | PASS |
| `strings.Contains` not used in sqlcheck.go | PASS — "strings" import removed |
| DDL statements not accepted by default in classify() | PASS — default case returns ErrSQLMissingOrgFilter |
| QueryRow does not panic in ValidationError mode | PASS |

## Self-Check: PASSED

- `services/api/internal/db/sqlcheck.go`: exists and modified
- `services/api/internal/db/sqlcheck_test.go`: exists and modified
- `services/api/internal/db/orgdb.go`: exists and modified
- `services/api/internal/db/orgdb_test.go`: created
- Commit `3b7cbf9`: exists in git log
- Commit `02fde35`: exists in git log
