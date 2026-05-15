---
status: partial
phase: 01-foundation-polyglot-monorepo
source: [01-VERIFICATION.md]
started: 2026-05-15T11:50:00Z
updated: 2026-05-15T11:50:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Live `docker compose up` single-command bring-up (FOUND-10)
expected: Fresh clone → `cp .env.example .env` → `docker compose up -d` → `curl localhost:8080/readyz` returns `{"status":"ok","checks":{"db":"ok","redis":"ok",...}}` within 30s
why-manual: Local environment has foreign containers (`open-routing-postgres-1` postgres:16-alpine, `open-routing-redis-1`, both 3+ days uptime) occupying 5432/6379. Executor declined to stop them per auto-mode rule 5 (no destruction of shared systems). Operator must either (a) stop the foreign containers and re-run `docker compose up -d` from this repo, or (b) accept this as deferred until the next clean machine.
result: [pending]

### 2. CI workflow first-PR execution (FOUND-09)
expected: First push to a PR branch triggers `.github/workflows/ci.yml` and all 7 jobs (go-vet, go-lint, go-test, web-typecheck, web-lint, migration-drift, compose-config) report green within ~5 minutes
why-manual: Cannot execute GitHub Actions runner locally. Plan 01-08 SUMMARY explicitly defers this ("first PR push is the live proof"). Specific concern flagged by verifier: local `golangci-lint v1.63.4` was built with Go 1.23 and cannot lint a Go 1.25 target; CI uses `golangci/golangci-lint-action@v6` which may resolve a newer binary — needs the live run to confirm.
result: [pending]

### 3. GitHub branch-protection enablement on `main` (D-31)
expected: Repo admin opens GitHub → Settings → Branches → Add branch protection rule for `main`; enables the 6 required status checks documented in `.github/branch-protection.md` (go-vet, go-lint, go-test, web-typecheck, web-lint, migration-drift); enables "Require a pull request before merging" + "Require status checks to pass" + dismiss stale reviews on push.
why-manual: GitHub's branch-protection model lives entirely in the web UI; no GitHub Actions / API workflow can self-enable it (would require a privileged token that bypasses the very protections it's setting). The branch-protection.md file is the documentation; the UI click-through is the authoritative enforcement.
result: [pending]

### 4. Live OTLP exporter against a real collector (FOUND-07 prod path)
expected: Run `OTEL_EXPORTER=otlp OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:14268/api/traces task dev` against a local Jaeger / Tempo / honeycomb-collector; trigger one `/healthz` request; confirm a span with `service.name=open-routing-api` arrives at the collector
why-manual: Phase 1 unit tests exercise the stdout exporter only. The OTLP HTTP exporter is wired (telemetry/otel.go switches on `OTEL_EXPORTER` env var) but no live collector is part of the Phase 1 infrastructure (D-15: no Jaeger/collector in docker-compose). Validating the prod path requires an external collector that ships in a later phase or is provisioned by the operator.
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps

(none — all 4 items are operator hand-offs, not code-layer gaps)
