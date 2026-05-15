---
phase: 1
slug: foundation-polyglot-monorepo
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-15
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.23.x) + `pnpm typecheck` / `pnpm lint` (TypeScript 5.x) |
| **Config file** | `services/api/go.mod`, `web/pnpm-workspace.yaml`, `web/turbo.json` |
| **Quick run command** | `task test:quick` (unit tests only, no testcontainers) |
| **Full suite command** | `task ci` (Go vet + lint + unit + integration + frontend typecheck/lint) |
| **Estimated runtime** | ~25s quick / ~3 min full (testcontainers cold-start dominates) |

---

## Sampling Rate

- **After every task commit:** Run `task test:quick`
- **After every plan wave:** Run `task ci`
- **Before `/gsd-verify-work`:** `task ci` must be green
- **Max feedback latency:** 30s for quick run

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 01-01-W0 | 01 | 0 | — | — | Wave-0 infra | install | `go install github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.x` | ❌ W0 | ⬜ pending |
| 01-01-W0 | 01 | 0 | — | — | Wave-0 infra | install | `go install github.com/go-task/task/v3/cmd/task@latest` | ❌ W0 | ⬜ pending |
| 01-FOUND-01 | 01 | 1 | FOUND-01 | — | Polyglot monorepo layout | structural | `test -d services/api && test -d web && test -d openapi && test -d migrations` | ❌ W0 | ⬜ pending |
| 01-FOUND-02 | 01 | 1 | FOUND-02 | T-1-02 | `org_id UUID NOT NULL` on every table | sql-check | `grep -E 'org_id UUID NOT NULL' migrations/*.up.sql` | ❌ W0 | ⬜ pending |
| 01-FOUND-03 | 05 | 2 | FOUND-03 | T-1-03 | Malformed/missing X-Org-Id → 400 | integration | `go test ./services/api/internal/middleware -run TestOrgIDExtraction` | ❌ W0 | ⬜ pending |
| 01-FOUND-04 | 03 | 2 | FOUND-04 | T-1-04 | orgDB rejects SQL without org_id filter | unit | `go test ./services/api/internal/orgdb -run TestValidatorRejects` | ❌ W0 | ⬜ pending |
| 01-FOUND-05 | 03 | 2 | FOUND-05 | — | org_id propagation via context.Context | unit | `go test ./services/api/internal/orgdb -run TestContextPropagation` | ❌ W0 | ⬜ pending |
| 01-FOUND-06 | 01 (schema) + 07 (test) | 1 (schema) + 4 (test) | FOUND-06 | T-1-02 | `UNIQUE (org_id, external_id)` constraint | sql-check | `grep 'UNIQUE (org_id, external_id)' migrations/*.up.sql` | ❌ W0 | ⬜ pending |
| 01-FOUND-07 | 04 | 2 | FOUND-07 | — | OTel SDK init before chi mux | unit | `go test ./services/api/internal/observability -run TestOTelInitOrder` | ❌ W0 | ⬜ pending |
| 01-FOUND-07b | 04 | 2 | FOUND-07 | — | slog + span carry org_id | integration | `go test ./services/api/internal/observability -run TestSpanOrgIDAttribute` | ❌ W0 | ⬜ pending |
| 01-FOUND-08 | 07 | 4 | FOUND-08 | T-1-08 | Two-org zero leakage (HTTP chain) | integration | `go test -tags=integration ./services/api/test/isolation -run TestTwoOrgIsolation -timeout 120s` | ❌ W0 | ⬜ pending |
| 01-FOUND-09a | 08 | 5 | FOUND-09 | — | CI runs `go vet` `go test` `golangci-lint` | ci-check | `act -j go-tests` or `.github/workflows/ci.yml` validation | ❌ W0 | ⬜ pending |
| 01-FOUND-09b | 08 | 5 | FOUND-09 | — | CI runs frontend typecheck/lint | ci-check | `cd web && pnpm typecheck && pnpm lint` | ❌ W0 | ⬜ pending |
| 01-FOUND-10 | 01 (compose) + 08 (CI sanity) | 1 + 5 | FOUND-10 | — | `docker compose up` single-command bring-up | smoke | `docker compose up -d && docker compose ps --status running` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/api/go.mod` — initialized with module path `github.com/open-routing/open-solutions/services/api`
- [ ] `migrate` CLI installed (`go install github.com/golang-migrate/migrate/v4/cmd/migrate/...@v4.18.x` with `postgres` build tag)
- [ ] `task` CLI installed (`brew install go-task/tap/go-task` or `go install`)
- [ ] `golangci-lint` v1.63.x installed (project already uses `golangci-lint 1.63.4` per researcher env-probe)
- [ ] `sqlc` v1.31.x installed (project already uses `sqlc 1.31.1`)
- [ ] `services/api/test/isolation/main_test.go` — testcontainers `TestMain` setup writing the shared Postgres connection string for all integration tests
- [ ] `services/api/test/isolation/helpers.go` — `seedOrgs(t, db, n)` + `httpReq(t, mux, org, method, path, body)` helpers
- [ ] `services/api/internal/orgdb/orgdb_test.go` — table-driven SQL validator tests (statements with/without `org_id` filter, INSERT/UPDATE/DELETE variants)

*If none of these exist at Phase 1 start (greenfield repo), all are Wave 0 tasks in the plan.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `docker compose up` single-command UX | FOUND-10 | Subjective "no manual steps" requires human confirmation that no `.env` editing or schema preload is required | 1. Fresh clone the repo; 2. `cp .env.example .env`; 3. `docker compose up -d`; 4. `curl localhost:8080/readyz` returns `{"status":"ok",...}` within 30s |
| OTLP exporter wiring (prod path) | FOUND-07 | Requires real OTLP endpoint to fully verify; stdout exporter covers Phase 1 unit/integration tests | Set `OTEL_EXPORTER=otlp` + `OTEL_EXPORTER_OTLP_ENDPOINT` against a local Jaeger; trigger one `/healthz` request; confirm span arrives. **Documented as deferred to Phase 2+ verify.** |
| Bypass slog WARN visibility | D-05 | Requires a developer to run `task migrate-up` and visually confirm a `slog.Warn event=orgdb_bypass` line in stdout | Run `task migrate-up`; tail logs; confirm exactly one bypass event per migration step. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter once Wave 0 complete

**Approval:** pending
