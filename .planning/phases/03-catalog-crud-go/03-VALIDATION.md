---
phase: 3
slug: catalog-crud-go
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-16
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `03-RESEARCH.md § Validation Architecture` + Phase 1/2 carry-forward.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib) + `github.com/stretchr/testify` (assert/require already in go.mod) |
| **Config file** | None — `go test` config lives in package directories; `services/api/test/isolation/main_test.go` holds shared testcontainer setup |
| **Quick run command** | `cd services/api && go test ./internal/catalog/...` |
| **Full suite command** | `cd services/api && go test ./...` (includes `test/isolation/` cross-org probes + miniredis cache tests) |
| **Estimated runtime** | ~30s catalog unit + miniredis tests; ~90s with isolation testcontainers (Postgres bootstrap dominates) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/catalog/{entity}/...` for the entity touched (quick path, no testcontainers).
- **After every plan wave:** Run `go test ./...` (full suite including cross-org isolation).
- **Before `/gsd-verify-work`:** Full suite must be green AND `task gen` must show zero drift (codegen-drift CI green).
- **Max feedback latency:** ~30 s for the quick path; ~90 s for the full suite.

---

## Per-Task Verification Map

Filled by the planner after PLAN.md generation. The plan-checker enforces that every task carries either an `<automated>` block or a Wave 0 dependency.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD     | TBD  | TBD  | CAT-01..11  | T-3-* (TBD) | TBD            | unit / integration | TBD          | ❌ W0        | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/api/internal/cache/cache_test.go` — miniredis-backed unit tests for `GetOrSet[T any]`, `Del`, refresh-ahead, singleflight (covers CAT-11)
- [ ] `services/api/internal/catalog/testutil.go` — shared per-entity test setup (Postgres testcontainer + miniredis wiring; D-73)
- [ ] `services/api/internal/catalog/{entity}_test.go` — one file per entity (agents, skills, queues, channels, adapters, break_reasons, agent_skills); covers happy path + CAT-08 version conflict + CAT-09 soft-delete + CAT-10 cursor/filter/name search + CAT-11 cache hit/miss
- [ ] `services/api/test/isolation/catalog_test.go` — cross-org probe test extending FOUND-08 suite with 6 entity GETs + 1 agent_skills probe (must return 404, never 200)
- [ ] `services/api/internal/server/request_id_exhaustiveness_test.go` — extend 121 sub-tests to cover the new `InvalidReferenceErrorResponse` wrapper introduced by spec amendment D-75 (Phase 2 D-44 carry-forward; this test will FAIL the build if forgotten)
- [ ] `services/api/internal/server/server.go` — add new error wrapper types to the `injectRequestIDIntoErrorResponse` type switch (driven by the exhaustiveness test)
- [ ] go.mod — add `github.com/alicebob/miniredis/v2` (test-only) and confirm `golang.org/x/sync` is a direct dep (currently indirect)
- [ ] CI workflow Go toolchain — confirm `go-version: '1.25.x'` in `.github/workflows/*.yml` (local Go is 1.23 but go.mod targets 1.25 per researcher A-block); fix if mismatched

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Codegen-drift in CI catches the openapi.yaml amendment (`invalid_reference` enum + 422 responses) | D-75 spec amendment | The drift CI job runs only in GitHub Actions PR pipeline | Open PR after Wave 0 commits; verify `codegen-drift` CI job is green and that `services/api/internal/api/*.gen.go` + `web/packages/ui/src/api/generated.ts` were regenerated in the same commit |
| Real Redis end-to-end smoke (miniredis covers protocol but not eviction edges) | CAT-11 | miniredis omits some eviction + replication quirks | After Wave 4 lands, run `docker compose up redis` locally, point `REDIS_URL` at it, hit `GET /v1/agents/{id}` twice within 60s and confirm second response time drops; verify slog "outcome=hit" attr is logged |
| `task db:reset` dev workflow still works after editing migration 002 (D-61) | D-61 single editable migration | Requires a real Postgres + the user's local task runner | Run `task db:reset` after any migration 002 edit; expect drop + create + migrate up to succeed |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90 s for full suite
- [ ] `nyquist_compliant: true` set in frontmatter once planner finishes filling Per-Task table

**Approval:** pending
