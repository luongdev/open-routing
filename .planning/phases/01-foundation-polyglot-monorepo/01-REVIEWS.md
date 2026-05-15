---
phase: 01
reviewers: [gemini, codex]
reviewed_at: 2026-05-15T12:25:00Z
plans_reviewed:
  - 01-01-PLAN.md (repo scaffold)
  - 01-02-PLAN.md (pnpm workspace)
  - 01-1a-PLAN.md (orgkey leaf)
  - 01-03-PLAN.md (orgDB + sqlc + cmd/migrate)
  - 01-04-PLAN.md (OTel + slog)
  - 01-05-PLAN.md (middleware)
  - 01-06-PLAN.md (HTTP server)
  - 01-07-PLAN.md (isolation tests)
  - 01-08-PLAN.md (CI workflow)
review_mode: post_execution
gemini_recommendation: APPROVE
codex_recommendation: REJECT
---

# Cross-AI Plan Review — Phase 1 (Post-Execution)

Two independent reviewers (Gemini 2.5 Pro + Codex GPT-5.5) audited the executed Phase 1 work against the phase goal and CONTEXT decisions. Their conclusions DIVERGE — Gemini approves; Codex rejects on three HIGH severity concerns.

---

## Gemini Review

### Summary
Phase 1 implementation successfully establishes a robust, polyglot monorepo foundation with a strict and provable multi-tenant isolation model. The codebase demonstrates high adherence to design constraints — particularly the `orgDB` wrapper, `chi` middleware chain order, and OTel instrumentation. The `testcontainers-go` integration alongside a rigorous isolation test suite provides strong confidence in security boundaries.

### Strengths
- **orgDB Enforcement:** AST-based SQL inspection via `pg_query_go` instead of brittle regex is excellent and resilient.
- **Middleware Safety:** `chi` chain places `Recoverer` before `RequestID` to catch panics (e.g., `uuid.Must` entropy exhaustion); bypass routes excluded from `OrgContext` closure.
- **OTel Init Ordering:** `cmd/api/main.go` wraps mux with `otelhttp` *after* router assembly — prevents silent trace propagation failures from RESEARCH.md pitfalls.
- **Testing Rigor:** `isolation_test.go` exercises the exact `server.NewMux` pipeline used in production, not handlers in a vacuum.

### Concerns
- **MEDIUM — `containsOrgIDColumnRef` String Matching Edge Case:** The SQL inspector relies on `strings.Contains(n.String(), 'sval:"org_id"')` over the protobuf representation. A query containing a literal string `'org_id'` (e.g., `SELECT 'org_id' AS alias FROM _scaffold`) would parse as an `A_Const` node with `sval:"org_id"`. **False-positive risk** — could trick the inspector into validating a query that lacks `WHERE org_id = $1`.
- **LOW — Deferred Operator Actions:** Live `docker compose` smoke + branch protection are standard environmental configurations.

### Suggestions
- Replace `n.String()` protobuf serialization matching with explicit AST node type-switch (`*pg_query.Node_ColumnRef`) to eliminate string-literal false-positives.
- Document a quick OTLP-exporter smoke against a local Jaeger container in dev onboarding.

### Recommendation
**APPROVE.** Phase meets all critical FOUND requirements. Deferred operator items are standard environmental configurations. Architectural boundaries — especially org isolation and context propagation — are sound, thoroughly tested, and ready to support Phase 2.

---

## Codex Review

### Summary
Implementation quality is solid for the current `_scaffold` path, but not enough to mark Phase 1 complete as a reusable org-isolation foundation. The HTTP isolation tests pass for the shipped handlers, yet the `orgDB` SQL inspector is too permissive and the CI gate is not reliable.

### Strengths
- Current scaffold queries correctly scoped: `WHERE org_id = $1/$2` in `services/api/internal/db/queries/scaffold.sql:7`.
- `X-Org-Id` middleware validates UUIDv7+ before setting context in `services/api/internal/middleware/orgcontext.go:48`.
- Two-org HTTP isolation suite uses real `server.NewMux` + Postgres container — passes locally.
- OTel init ordering correct: `InitOTel` before `server.NewMux` in `services/api/cmd/api/main.go:64`.
- Middleware chain sane: Recoverer → RequestID → `/v1` OrgContext in `services/api/internal/server/server.go:77`.

### Concerns
- **HIGH — SQL inspector not watertight.** For SELECT/UPDATE/DELETE, it only checks whether `org_id` appears *anywhere* in the AST/string form — not whether it is a scoped predicate tied to the target table and request org. `SELECT id, org_id FROM _scaffold` would pass despite no `WHERE org_id = ...`. See `services/api/internal/db/sqlcheck.go:98` and `:132`. **This is a cross-org leakage vector.**
- **HIGH — DDL/utility statements accepted by default in orgDB.** `DROP`, `TRUNCATE`, etc. should require bypass, not pass normal org-scoped DB access. See `services/api/internal/db/sqlcheck.go:108`.
- **HIGH — CI not currently trustworthy.** Local `golangci-lint run` fails because the pinned v1.63 lint binary is built with Go 1.23 while the module targets Go 1.25.10. CI pins the same line in `.github/workflows/ci.yml:39`.
- **MEDIUM — Testcontainer failures exit `0`** — the isolation suite can silently pass without running in CI if Docker/image startup fails. See `services/api/test/isolation/main_test.go:84`.
- **MEDIUM — FOUND-10 not literally met.** `docker-compose.yml` starts only Postgres + Redis (no API or Vite dev servers). `task dev` adds `air` but not pnpm. ROADMAP success criterion #1 says "Go API skeleton and the pnpm dev server stub". D-25 supersedes this but the requirement text wasn't updated. See `docker-compose.yml:1` and `Taskfile.yml:42`.
- **MEDIUM — OrgDB.QueryRow panics even in `ValidationError` mode**, contradicting the prod "return error" intent. See `services/api/internal/db/orgdb.go:87`.

### Suggestions
- Replace SQL checker with stricter AST validation: require target-table-scoped `org_id = $N` predicates for SELECT/UPDATE/DELETE, require INSERT `org_id` column, reject all utility/DDL unless bypass is present.
- Add negative tests for projection-only `org_id`, `ORDER BY org_id`, nested unrelated `org_id`, joins without request-org predicate, DDL through normal `OrgDB`.
- Make testcontainer setup fail-closed when `CI=true`; allow local skip only outside CI.
- Upgrade/pin `golangci-lint` to a version compatible with Go 1.25, or lower the module target.
- Fix the quoted `task --list` command in `.github/workflows/ci.yml:137` (executes as one invalid command string).
- Resolve FOUND-10 vs D-25 wording: either add compose services or update requirement text.

### Recommendation
**REJECT** for phase completion. Current scaffold behavior is isolated and core wiring is mostly good, but the foundation claim depends on `orgDB` being a strong reusable guard and CI being a reliable merge gate. Both have blocking gaps. Mark complete after SQL validation is tightened and CI/test gating is fail-closed.

---

## Consensus Summary

### Agreed Strengths (both reviewers)
- AST-based SQL inspection chosen over regex — correct architectural call.
- Middleware chain ordering (Recoverer → RequestID → `/v1` OrgContext) — sane and safe.
- OTel init ordering (InitOTel BEFORE chi router) — correct, prevents silent propagation failure.
- Two-org isolation suite uses the production `server.NewMux` pipeline — high-quality test.

### Agreed Concerns (both reviewers flagged SQL inspector permissiveness)
- **The `containsOrgIDColumnRef` predicate is too weak.** Gemini flagged false-positive (literal `'org_id'` string); Codex flagged false-negative (`SELECT id, org_id FROM _scaffold` passing without WHERE). Both stem from the same root cause: the inspector checks for `org_id` *presence* in the AST, not for `org_id` *as a scoped predicate* on the target table.
- Codex's false-negative framing is more dangerous — it's an actual cross-org data leak vector for any future query that projects `org_id` without filtering.

### Divergent Views — must resolve

| Concern | Gemini verdict | Codex verdict |
|---------|----------------|---------------|
| SQL inspector quality | MEDIUM (false-positive) | HIGH (false-negative — security gap) |
| DDL/utility statements via orgDB | not raised | HIGH (DROP/TRUNCATE bypass-free) |
| CI golangci-lint vs Go 1.25 | not raised | HIGH (CI gate unreliable) |
| Testcontainer fail-closed semantics | not raised | MEDIUM (silent CI pass on docker failure) |
| FOUND-10 literal compliance | not raised | MEDIUM (D-25 supersedes ROADMAP text — needs reconciliation) |
| Final recommendation | **APPROVE** | **REJECT** |

### Top Priority Action Items (synthesized)

If the user wants Phase 1 hardened before approving:

1. **Strengthen SQL inspector** (HIGH). Replace `containsOrgIDColumnRef` with explicit AST node-type walking that requires:
   - For SELECT/UPDATE/DELETE: a `WHERE` clause with an `org_id = $N` predicate bound to the target table
   - For INSERT: an `org_id` column in the column list
   - For DDL/utility (DROP/TRUNCATE/ALTER/CREATE): reject unless bypass marker present
   - Add negative tests for the 5 patterns Codex listed (projection-only, ORDER BY, nested, join-without-predicate, DDL via normal OrgDB)

2. **Fix CI lint compatibility** (HIGH). Either upgrade `golangci/golangci-lint-action` to a version with a Go 1.25-compatible binary, or downgrade `go.mod` to 1.23. Currently CI's golangci-lint job will fail on first push.

3. **Fail-closed testcontainer in CI** (MEDIUM). If `CI=true` and testcontainer setup fails, exit non-zero. Local dev can keep skip semantics.

4. **Resolve FOUND-10 wording** (MEDIUM). The ROADMAP text says "Go API skeleton and the pnpm dev server stub" but D-25 explicitly chose no API service in compose (devs run native via `air`) and no pnpm dev server. Update REQUIREMENTS.md FOUND-10 text to match D-25 reality, or add the compose services.

5. **Fix `task --list` quoting in ci.yml** (LOW). Currently invalid shell — would fail at runtime.

6. **OrgDB.QueryRow panic-vs-error consistency** (MEDIUM). In `ValidationError` mode, QueryRow should return an error like the other methods, not panic.

---

## Suggested Next Steps for User

**Option A — Accept Gemini's verdict and approve as-is.** The shipped scaffold's queries are correctly scoped (Gemini and Codex agree). The HIGH concerns Codex raised are about *future* queries the foundation needs to defend against — Phase 3 (catalog CRUD) is where they'd actually matter, and we'd catch them when those queries land.

**Option B — Accept Codex's verdict and run `/gsd-plan-phase 1 --gaps` to harden** the SQL inspector + CI lint + DDL guard + FOUND-10 wording before marking Phase 1 complete. Estimated: 1 small gap plan, ~2-4 tasks. Cost: extra wave of work, but Phase 1 becomes a stronger reusable foundation.

**Option C — Hybrid.** Approve Phase 1 (Gemini's path) AND open a Phase 1.5 / Phase 3 prerequisite ticket for the orgDB hardening that Codex flagged. This unblocks Phase 2 (OpenAPI contract) immediately while preserving Codex's findings as work items.

The orchestrator's view: Codex's HIGH-1 (SQL inspector false-negative on projection-only org_id) is technically correct but currently latent — no shipped query exhibits the pattern, and the planner deliberately scoped Phase 1's queries to be explicit. The HIGH-3 (golangci-lint Go 1.25 mismatch) is real and will fail on first PR. The HIGH-2 (DDL bypass-free) is real but DDL never flows through orgDB in normal operation (only via cmd/migrate which uses WithBypass). My recommendation: **Option C** — approve Phase 1, track the hardening items.
