# Roadmap: Open Routing — v0.1 Catalog Foundation

## Overview

v0.1 builds the catalog foundation that every subsequent Open Routing milestone depends on. Starting from a blank polyglot monorepo, seven phases deliver the complete catalog layer in strict dependency order: the Go/pnpm monorepo with org-isolation infrastructure first (isolation bugs compound into everything downstream), then the OpenAPI contract that types both the Go server and the TypeScript client before any endpoint is implemented, then full catalog CRUD (the six entities everything reads from), then the agent state machine (which guards against break_reasons from the catalog layer), then bulk import (which writes to the entity tables), then the shared Lit component library and standalone admin app, and finally the Web Component embed bundle. At the end of Phase 7, an org admin can configure routing data through a standalone Vite SPA, a host application can mount `<open-routing-catalog>` as a Web Component, and the agent-state foundation is live and correct before flows or runtime exist.

## Milestone

**v0.1 Catalog Foundation** — 53 requirements, 7 phases, Phase 1 is the current start.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundation & Polyglot Monorepo** - Bootstrap the Go + pnpm polyglot monorepo with org-isolation infrastructure: `services/api` Go skeleton with chi + pgx + slog + OTel, orgDB wrapper, PostgreSQL 17 + Redis via Docker Compose, golang-migrate, pnpm workspace stub, GitHub Actions CI, and the two-org isolation integration test proving zero data leakage.
- [ ] **Phase 2: OpenAPI Contract & Codegen** - Define `openapi/openapi.yaml` covering all v0.1 endpoints, wire oapi-codegen (Go server stubs) and openapi-typescript (TS client), and add a CI check that fails on any codegen drift.
- [ ] **Phase 3: Catalog CRUD (Go)** - sqlc queries and golang-migrate migrations for all 6 entities, chi handlers generated from the OpenAPI spec, soft-delete, version-locking with HTTP 409, cursor pagination, name search, and Redis cache for hot-path reads.
- [ ] **Phase 4: Agent State Machine (Go)** - Domain transition matrix in `services/api/internal/domain`, `agent_states` DB table, PATCH status endpoint with HTTP 409 on invalid transitions, Break→break_reason guard, post_interaction_state, server-owned WrapUp TTL goroutine, and IsRoutable helper.
- [ ] **Phase 5: Bulk Import (Go)** - POST import endpoint for all 6 entities, encoding/csv with BOM/CRLF handling, upsert ON CONFLICT, 207 partial success, import_jobs persistence, 50 MB/500-row cap, and schema versioning.
- [ ] **Phase 6: Shared UI Library & Standalone Admin** - `packages/ui` Lit + Shoelace components and generated TS client wrapper; `apps/admin` Vite SPA with CRUD screens for all 6 entities, 409 reload-prompt UX, and theme token support.
- [ ] **Phase 7: Web Component Embed Bundle** - `apps/embed` builds `<open-routing-catalog>` Custom Element with Shadow DOM CSS isolation, theme/modules attributes, auth-expired CustomEvent, and Playwright integration tests in React/Vue/HTML stub hosts with bundle size ≤ 70 KB gzipped.

## Phase Details

### Phase 1: Foundation & Polyglot Monorepo

**Goal**: A running, CI-gated polyglot monorepo where every piece of org-scoped infrastructure in Go is provably correct before any catalog entity or contract is defined.
**Depends on**: Nothing (first phase)
**Requirements**: FOUND-01, FOUND-02, FOUND-03, FOUND-04, FOUND-05, FOUND-06, FOUND-07, FOUND-08, FOUND-09, FOUND-10
**Success Criteria** (what must be TRUE):

  1. `docker compose up` brings up PostgreSQL 17, Redis, the Go API skeleton, and the pnpm dev server stub with a single command and no manual steps.
  2. Any HTTP request to the Go API that is missing or carries a malformed `X-Org-Id` header returns HTTP 400; a valid header propagates `org_id` through `context.Context` from middleware to handler to service to repository — no global state, no struct-field passing.
  3. The integration test suite seeds two orgs with identical `external_id` values, calls every scaffold endpoint, and asserts zero rows from org A appear in org B's responses — verified by the `orgDB` wrapper and `UNIQUE (org_id, external_id)` schema constraint.
  4. Every slog log line and OTel span carries an `org_id` attribute extracted from `context.Context`; the OTel SDK initializes before chi router setup.
  5. GitHub Actions CI runs `go vet`, `go test`, `golangci-lint`, frontend typecheck/lint/unit tests, and the two-org isolation suite on every pull request, blocking merge on any failure.

**Plans**: 8 plans
Plans:
**Wave 1**

- [x] 01-01-PLAN.md — Repo scaffolding & Wave-0 tooling (root files, docker-compose, first migration, go.mod, Taskfile, config loader)
- [x] 01-02-PLAN.md — pnpm workspace stub (apps/admin, apps/embed, packages/ui — empty index.ts per D-12)

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 01-03-PLAN.md — orgDB wrapper + SQL validator + bypass + sqlc gen + cmd/migrate (FOUND-04, FOUND-05, FOUND-06)
- [ ] 01-04-PLAN.md — OTel SDK init + slog TracingHandler (FOUND-07)
- [ ] 01-05-PLAN.md — Middleware: OrgContext + RequestID + httputil (FOUND-03, FOUND-05)

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 01-06-PLAN.md — HTTP server (chi mux + bypass routes + scaffold handlers + cmd/api) wiring all of Wave 2

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 01-07-PLAN.md — Two-org isolation test harness (testsupport + test/isolation/ — FOUND-08 gate)

**Wave 5** *(blocked on Wave 4 completion)*

- [ ] 01-08-PLAN.md — GitHub Actions CI workflow (FOUND-09)

**Branch**: `gsd/phase-01-foundation-polyglot-monorepo`

---

### Phase 2: OpenAPI Contract & Codegen

**Goal**: A single OpenAPI 3.1 spec covers every v0.1 endpoint and drives both the Go server stubs and the TypeScript client, so neither side can drift from the contract undetected.
**Depends on**: Phase 1
**Requirements**: CONTRACT-01, CONTRACT-02, CONTRACT-03, CONTRACT-04
**Success Criteria** (what must be TRUE):

  1. `openapi/openapi.yaml` defines every v0.1 REST endpoint, request/response schema, and error shape as a single valid OpenAPI 3.1 document that passes a spec linter in CI.
  2. Running `go generate ./...` regenerates Go handler interfaces, request/response types, and validators under `services/api/internal/api/` with no manual editing required.
  3. Running `pnpm gen:api` regenerates the typed TypeScript fetch client from the same spec, consumable by both `apps/admin` and `apps/embed` via `packages/ui` re-export.
  4. A CI job runs both codegen commands and fails the build if the resulting diff against committed code is non-empty — no silent drift between spec and generated artifacts.

**Plans**: TBD
**Branch**: `gsd/phase-02-openapi-contract`

---

### Phase 3: Catalog CRUD (Go)

**Goal**: An org admin can create, read, update, and soft-delete all six catalog entities through a fully validated REST API backed by Go handlers generated from the OpenAPI spec, with Redis caching on hot-path reads.
**Depends on**: Phase 2
**Requirements**: CAT-01, CAT-02, CAT-03, CAT-04, CAT-05, CAT-06, CAT-07, CAT-08, CAT-09, CAT-10, CAT-11
**Success Criteria** (what must be TRUE):

  1. An org admin can create, retrieve, update, and soft-delete each of the 6 entities (agents, skills, queues, channels, adapters, break_reasons) via REST; sending an update with `enabled=false` soft-deletes; soft-deleted rows are excluded from default list responses and visible only with `?include_disabled=true`.
  2. An update request carrying the wrong `version` value returns HTTP 409 with the current server-side record; two concurrent clients updating the same record do not silently overwrite each other.
  3. List endpoints support cursor-based pagination, `enabled` filtering, and case-insensitive `name` search; all three filters work independently and in combination.
  4. Agent-skill assignment accepts and enforces `proficiency` in the range 1–10; values outside that range return HTTP 422.
  5. Single-entity GET responses and status lookups are served from Redis under the key `or:{orgId}:{entity}:{id}` with a 60-second TTL; a write to any entity invalidates its cache key within the same request.

**Plans**: TBD
**Branch**: `gsd/phase-03-catalog-crud`

---

### Phase 4: Agent State Machine (Go)

**Goal**: The system enforces a strict, org-scoped agent status model where invalid transitions are rejected at the domain layer, WrapUp cannot get stuck on browser disconnect, and IsRoutable is correct for every Break sub-reason combination.
**Depends on**: Phase 3
**Requirements**: STATE-01, STATE-02, STATE-03, STATE-04, STATE-05, STATE-06, STATE-07, STATE-08, STATE-09, STATE-10
**Success Criteria** (what must be TRUE):

  1. `PATCH /agents/{id}/status` accepts every allowed transition (NotReady↔Ready, Ready→Break, Break→Ready, Break→NotReady, WrapUp→Ready, WrapUp→NotReady, and all system-initiated transitions) and rejects every other transition with HTTP 409 carrying `{"from", "to", "error": "invalid_transition"}`.
  2. A `Ready → Break` transition with a `break_reason_id` from a different org or a non-existent reason returns HTTP 422; a valid same-org reason succeeds and records the reason on the agent's state row.
  3. An agent in WrapUp whose browser is closed returns to their `post_interaction_state` (Ready or NotReady) automatically after `wrapup_until` expires, with no client action required — verified by killing the connection and waiting TTL+5s.
  4. `IsRoutable` in `services/api/internal/domain` returns `true` only when `status == Ready` OR (`status == Break` AND the associated break reason has `routable=true`); all four combinations are covered by unit tests.
  5. Every state mutation increments `state_version` monotonically; the `agent_states` table carries `status`, `engaged_channel`, `break_reason_id`, `post_interaction_state`, `state_version`, and `wrapup_until` from its initial migration.

**Plans**: TBD
**Branch**: `gsd/phase-04-agent-state-machine`

---

### Phase 5: Bulk Import (Go)

**Goal**: An org admin can seed or update any of the six catalog entities in bulk via a single synchronous import endpoint that handles JSON and CSV, tolerates partial row failures, and persists session results for polling.
**Depends on**: Phase 3
**Requirements**: IMP-01, IMP-02, IMP-03, IMP-04, IMP-05, IMP-06, IMP-07, IMP-08
**Success Criteria** (what must be TRUE):

  1. `POST /v1/orgs/{org_id}/catalog/import?entity={type}` accepts a JSON or CSV body for any of the 6 entity types and upserts rows keyed by `(org_id, external_id)`; running the same payload twice produces no duplicates.
  2. A CSV body containing a UTF-8 BOM, Windows CRLF line endings, and fields with embedded commas, quoted newlines, and escaped quotes is parsed without error or data corruption using Go's `encoding/csv` with explicit BOM handling.
  3. A batch where some rows fail and some succeed returns HTTP 207 with `{ succeeded: [ids], failed: [{row, field, message}] }`; valid rows are persisted even when the same batch contains invalid rows.
  4. A request body exceeding 50 MB or 500 rows returns HTTP 413 with a message pointing to the v0.2 async pathway; a CSV request missing `?schema_version=v0.1` returns HTTP 400 listing supported versions.
  5. `GET /v1/orgs/{org_id}/imports/{id}` returns the full import session (total_rows, succeeded_rows, failed_rows, errors JSONB) for any previously completed import, persisted in the `import_jobs` table.

**Plans**: TBD
**Branch**: `gsd/phase-05-bulk-import`

---

### Phase 6: Shared UI Library & Standalone Admin

**Goal**: A platform engineer can use the standalone admin SPA to configure all six catalog entities directly, and all Lit components and the generated TypeScript API client live in `packages/ui` ready to be consumed by the embed bundle.
**Depends on**: Phase 3, Phase 4
**Requirements**: ADMIN-01, ADMIN-02, ADMIN-03, ADMIN-04, ADMIN-05, ADMIN-06
**Success Criteria** (what must be TRUE):

  1. `apps/admin` builds as a Vite SPA (TypeScript + Lit + Shoelace) deployable as a static bundle; a platform engineer can open it, supply an `org_id` via URL path or stub login, and access full CRUD screens for all 6 catalog entities without any additional tooling.
  2. All API calls from the admin app include an `X-Org-Id` header derived from app state; `org_id` is never read from response bodies or injected via global variables.
  3. The generated TypeScript API client in `packages/ui` (re-exported from `openapi-typescript` output) is the only API access layer — TypeScript compilation fails if admin app code constructs raw fetch URLs bypassing the typed client.
  4. When a write returns HTTP 409, the admin app re-fetches the entity and surfaces a visible "Changed by someone else, reload?" affordance rather than silently discarding the user's edits.
  5. Shoelace theme tokens applied via CSS custom properties at the admin root switch theme variants at runtime without a page reload.

**Plans**: TBD
**Branch**: `gsd/phase-06-shared-ui-admin`
**UI hint**: yes

---

### Phase 7: Web Component Embed Bundle

**Goal**: A host application built in any framework can mount `<open-routing-catalog>` as a Web Component, get full catalog CRUD for all six entities inside a Shadow DOM boundary, and pass org context only through element attributes — never through globals.
**Depends on**: Phase 6
**Requirements**: EMBED-01, EMBED-02, EMBED-03, EMBED-04, EMBED-05, EMBED-06, EMBED-07, EMBED-08, EMBED-09, EMBED-10
**Success Criteria** (what must be TRUE):

  1. `apps/embed` builds a single ES module bundle that registers `<open-routing-catalog>` as a Custom Element; the gzipped bundle size is at or below 70 KB measured in CI on every build.
  2. A host app mounts `<open-routing-catalog org-id="..." api-base-url="..." theme="..." modules="...">` and every API call from inside the element carries `X-Org-Id` set from the `org-id` attribute — not from `window`, `localStorage`, or any ambient global.
  3. The Playwright stub host tests verify Shadow DOM CSS isolation in both directions (styles from outside do not bleed in; component styles do not bleed out) across all three host environments: React 18, Vue 3, and plain HTML.
  4. When the embed bundle receives an HTTP 401 response, it dispatches `open-routing:auth-expired` as a CustomEvent with `composed: true`, bubbling across the Shadow DOM boundary to the host document.
  5. The `modules` attribute filters navigation so only listed entity names appear; unlisted entities are absent from the route table and navigation — not just hidden by CSS.

**Plans**: TBD
**Branch**: `gsd/phase-07-web-component-embed`
**UI hint**: yes

---

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation & Polyglot Monorepo | 2/9 | In Progress|  |
| 2. OpenAPI Contract & Codegen | 0/TBD | Not started | - |
| 3. Catalog CRUD (Go) | 0/TBD | Not started | - |
| 4. Agent State Machine (Go) | 0/TBD | Not started | - |
| 5. Bulk Import (Go) | 0/TBD | Not started | - |
| 6. Shared UI Library & Standalone Admin | 0/TBD | Not started | - |
| 7. Web Component Embed Bundle | 0/TBD | Not started | - |

---

## Coverage Report

**Milestone:** v0.1 Catalog Foundation
**Requirements mapped:** 53 / 53

| Category | Requirements | Phase |
|----------|-------------|-------|
| Foundation & Multi-Org Isolation | FOUND-01 through FOUND-10 (10) | Phase 1 |
| API Contract (OpenAPI) | CONTRACT-01 through CONTRACT-04 (4) | Phase 2 |
| Catalog CRUD | CAT-01 through CAT-11 (11) | Phase 3 |
| Agent State Model | STATE-01 through STATE-10 (10) | Phase 4 |
| Bulk Import | IMP-01 through IMP-08 (8) | Phase 5 |
| Standalone Admin App | ADMIN-01 through ADMIN-06 (6) | Phase 6 |
| Web Component Embed Bundle | EMBED-01 through EMBED-10 (10) | Phase 7 |

**Unmapped:** 0

---

*Roadmap defined: 2026-05-15*
*Milestone: v0.1 Catalog Foundation*
*Stack: Go + chi + sqlc + pgx + golang-migrate + PostgreSQL 17 + Redis + OpenAPI 3.1 + Vite + Lit + Shoelace + Web Components*
*Phase numbering: sequential, starting at 1*
