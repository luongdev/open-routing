# Roadmap: Open Routing — v0.1 Catalog Foundation

## Overview

v0.1 builds the catalog foundation that every subsequent Open Routing milestone depends on. Starting from a blank polyglot monorepo, seven phases deliver the complete catalog layer in strict dependency order: the Go/pnpm monorepo with org-isolation infrastructure first (isolation bugs compound into everything downstream), then the OpenAPI contract that types both the Go server and the TypeScript client before any endpoint is implemented, then full catalog CRUD (the six entities everything reads from), then the agent state machine (which guards against break_reasons from the catalog layer), then bulk import (which writes to the entity tables), then the shared Lit component library and standalone admin app, and finally the Web Component embed bundle. At the end of Phase 7, an org admin can configure routing data through a standalone Vite SPA, a host application can mount `<open-routing-catalog>` as a Web Component, and the agent-state foundation is live and correct before flows or runtime exist.

## Milestone

**v0.1 Catalog Foundation** — 53 requirements, 7 phases, 5/7 phases complete (Phase 04.1 + Phase 5 shipped 2026-05-17); Phase 6 is current focus.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation & Polyglot Monorepo** - Bootstrap the Go + pnpm polyglot monorepo with org-isolation infrastructure: `services/api` Go skeleton with chi + pgx + slog + OTel, orgDB wrapper, PostgreSQL 17 + Redis via Docker Compose, golang-migrate, pnpm workspace stub, GitHub Actions CI, and the two-org isolation integration test proving zero data leakage. (completed 2026-05-15)
- [x] **Phase 2: OpenAPI Contract & Codegen** - Define `openapi/openapi.yaml` covering all v0.1 endpoints, wire oapi-codegen (Go server stubs) and openapi-typescript (TS client), and add a CI check that fails on any codegen drift. (completed 2026-05-16)
- [x] **Phase 3: Catalog CRUD (Go)** - sqlc queries and golang-migrate migrations for all 6 entities, chi handlers generated from the OpenAPI spec, soft-delete, version-locking with HTTP 409, cursor pagination, name search, and Redis cache for hot-path reads. (completed 2026-05-16)
- [x] **Phase 4: Agent State Machine (Go)** - Domain transition matrix in `services/api/internal/domain`, `agent_states` DB table, PATCH status endpoint with HTTP 409 on invalid transitions, Break→break_reason guard, post_interaction_state, server-owned WrapUp TTL goroutine, and IsRoutable helper. (completed 2026-05-17)
- [x] **Phase 04.1: Catalog Identity Normalization (INSERTED)** - Introduce universal user-facing `code` (TEXT NOT NULL, UNIQUE per org) on all 6 primary catalog entities. Demote `external_id` to optional with partial unique. Drop `break_reasons.UNIQUE (org_id, name)`. Updates migration 000002 (still editable per D-61), OpenAPI contract, sqlc queries, 6 CRUD handlers, and tests. Unblocks Phase 5 import keyed on `code`. (completed 2026-05-17)
- [x] **Phase 5: Bulk Import (Go)** - POST import endpoint for all 6 entities keyed on `code`, encoding/csv with BOM/CRLF handling, upsert ON CONFLICT, 207 partial success, import_jobs persistence, 50 MB/500-row cap, and schema versioning. (completed 2026-05-17)
- [x] **Phase 6: Shared UI Library & Standalone Admin** - `packages/ui` Lit + Shoelace components and generated TS client wrapper; `apps/admin` Vite SPA with CRUD screens for all 6 entities, 409 reload-prompt UX, and theme token support. (completed 2026-05-18)
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

- [x] 01-03-PLAN.md — orgDB wrapper + SQL validator + bypass + sqlc gen + cmd/migrate (FOUND-04, FOUND-05, FOUND-06)
- [x] 01-04-PLAN.md — OTel SDK init + slog TracingHandler (FOUND-07)
- [x] 01-05-PLAN.md — Middleware: OrgContext + RequestID + httputil (FOUND-03, FOUND-05)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-06-PLAN.md — HTTP server (chi mux + bypass routes + scaffold handlers + cmd/api) wiring all of Wave 2

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 01-07-PLAN.md — Two-org isolation test harness (testsupport + test/isolation/ — FOUND-08 gate)

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 01-08-PLAN.md — GitHub Actions CI workflow (FOUND-09)

**Branch**: `gsd/phase-01-foundation-polyglot-monorepo`

---

### Phase 2: OpenAPI Contract & Codegen

**Goal**: A single OpenAPI 3.0 spec covers every v0.1 endpoint and drives both the Go server stubs and the TypeScript client, so neither side can drift from the contract undetected. *(Note: originally locked as OAS 3.1; downgraded to OAS 3.0.0 during Plan 02-03 because oapi-codegen v2 does not support OAS 3.1 nullable type arrays. See 02-VERIFICATION.md "Deviation 1" for full rationale.)*
**Depends on**: Phase 1
**Requirements**: CONTRACT-01, CONTRACT-02, CONTRACT-03, CONTRACT-04
**Success Criteria** (what must be TRUE):

  1. `openapi/openapi.yaml` defines every v0.1 REST endpoint, request/response schema, and error shape as a single valid OpenAPI 3.0 document that passes a spec linter in CI.
  2. Running `go generate ./...` regenerates Go handler interfaces, request/response types, and validators under `services/api/internal/api/` with no manual editing required.
  3. Running `pnpm gen:api` regenerates the typed TypeScript fetch client from the same spec, consumable by both `apps/admin` and `apps/embed` via `packages/ui` re-export.
  4. A CI job runs both codegen commands and fails the build if the resulting diff against committed code is non-empty — no silent drift between spec and generated artifacts.

**Plans**: 6 plans
Plans:

**Wave 1**

- [x] 02-01-PLAN.md — UI sketches validating API shapes before spec is locked (D-33, D-34)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 02-02-PLAN.md — Author openapi/openapi.yaml + extend middleware.WriteError to embed request_id (CONTRACT-01, D-35, D-36, D-37)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 02-03-PLAN.md — Go codegen pipeline: tools.go + oapi-codegen.yaml + go:generate + lint exemption + Taskfile gen (CONTRACT-02, D-41, D-42, D-43)
- [x] 02-05-PLAN.md — TypeScript codegen distribution: deps + gen:api + generated.ts + client/errors/task/index (CONTRACT-03, D-38, D-39, D-40)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 02-04-PLAN.md — Scaffold strict-server migration + /openapi.yaml + /docs runtime serving + server wiring (CONTRACT-02, D-44, D-45, D-46)

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 02-06-PLAN.md — CI codegen-drift job: Redocly lint + task gen + git diff --exit-code (CONTRACT-04, D-47, D-48)

**Branch**: `gsd/phase-02-openapi-contract-codegen`

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

**Plans**: 10 plans
Plans:

**Wave 0**

- [x] 03-01-PLAN.md — OpenAPI spec amendment (invalid_reference + 422 + Channel/Adapter version + limit default) + scaffold deletion + codegen regen + Wave0TempStubs

**Wave 1** *(blocked on Wave 0 completion — three plans in parallel)*

- [x] 03-02-PLAN.md — Migration 000002 (7 catalog tables + indexes + universal version + agent_skills) + Taskfile db:reset
- [x] 03-03-PLAN.md — sqlc queries for 7 entities + OrgDB.BeginTx (OQ-5)
- [x] 03-04-PLAN.md — internal/cache/ package (CAT-11: GetOrSet[T] + singleflight + refresh-ahead + miniredis tests)

**Wave 2** *(blocked on Wave 1)*

- [x] 03-05-PLAN.md — internal/catalog/ package skeleton (Handlers + Deps + cursor + mappers + errors + notimpl + per-entity placeholders)

**Wave 3** *(blocked on Wave 2)*

- [x] 03-06-PLAN.md — Agents entity end-to-end (CAT-01, CAT-03, CAT-08, CAT-09, CAT-10, CAT-11) + main_test.go + testutil.go

**Wave 4** *(blocked on Wave 3 — three plans in parallel)*

- [x] 03-07-PLAN.md — Skills + BreakReasons CRUD (CAT-02, CAT-07)
- [x] 03-08-PLAN.md — Queues + Channels CRUD + D-76 cross-row FK validation (CAT-04, CAT-05)
- [x] 03-09-PLAN.md — Adapters CRUD (JSONB) + agent_skills helpers extraction (CAT-03, CAT-06)

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 03-10-PLAN.md — main.go wiring + bypass route handlers + cross-org isolation suite extension

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

**Plans**: 6 plans
Plans:

**Wave 0**

- [x] 04-01-PLAN.md — Spec amendment (force field) + agent_states migration append + tenantTables allowlist + codegen regen + clockwork install (D-78, D-92)

**Wave 1** *(blocked on Wave 0 completion)*

- [x] 04-02-PLAN.md — sqlc queries + IsRoutable domain pkg (STATE-10) + state pkg skeleton with Server type (Pitfall 1 rename) + transition matrix exhaustive walk (D-91)

**Wave 2** *(blocked on Wave 1)*

- [x] 04-03-PLAN.md — state.Server handler bodies: GetAgentStatus cache-read-through + PatchAgentStatus probe-then-matrix + force WARN + cache invalidation (D-66, D-84, D-86)

**Wave 3** *(blocked on Wave 1 — parallel with Wave 2)*

- [x] 04-04-PLAN.md — WrapUp TTL goroutine + clockwork.AfterFunc registry + 30s safety sweep + startup sweep + graceful Stop (D-81, D-95)

**Wave 4** *(blocked on Waves 2 + 3)*

- [x] 04-05-PLAN.md — ApiHandlers composite wiring in main.go + CreateAgent atomic agent_states INSERT + notimpl.go cleanup (D-89, D-93)

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 04-06-PLAN.md — D-94 cross-org isolation suite + 5 acceptance tests + cross-AI peer review (Codex + Gemini parallel per CLAUDE.md HARD RULE)

**Branch**: `gsd/phase-04-agent-state-machine-go`

---

### Phase 04.1: catalog-identity-normalization (INSERTED)

**Goal:** A single canonical user-facing identifier `code` exists on all 6 primary catalog entities (`agents`, `skills`, `queues`, `channels`, `adapters`, `break_reasons`). `code` is the upsert key for bulk import (Phase 5) and the cross-reference target for nested relationships. `external_id` is demoted to an optional integration-mapping field. After this phase, generic file imports and DSL references use `code`; v0.2 external sync can layer `external_source` onto the optional `external_id` without contract churn.
**Depends on:** Phase 4
**Requirements**: IDENT-01, IDENT-02, IDENT-03, IDENT-04, IDENT-05, IDENT-06, IDENT-07, IDENT-08
**Success Criteria** (what must be TRUE):

  1. Every primary catalog table has a `code TEXT NOT NULL` column with `UNIQUE (org_id, code)`; the existing `UNIQUE (org_id, external_id) NOT NULL` constraint is replaced by `external_id TEXT NULL` with partial unique `WHERE external_id IS NOT NULL` on `agents`, `skills`, `queues`, `channels`. `adapters` and `break_reasons` gain both new columns; `break_reasons.UNIQUE (org_id, name)` is dropped.
  2. `POST /v1/orgs/{org_id}/{entity}` with a body missing `code` returns HTTP 400; a body whose `code` collides with an existing row in the same org returns HTTP 409 with `ErrorCode=duplicate_code`. `PATCH` requests attempting to mutate `code` are rejected.
  3. `GET /v1/orgs/{org_id}/break_reasons/{id}` returns both `code` and `name`; renaming `name` does not change the URL or break any cross-reference; the entity remains addressable by its stable `code`.
  4. The two-org isolation test suite passes with `code`-keyed fixtures; every CRUD endpoint exercises both `code` collisions (409) and `external_id` collisions (409 distinct error code) within a single org and across orgs.
  5. `go generate ./...` and `pnpm gen:api` produce no diff against committed code (CONTRACT-04 maintained); OpenAPI 3.0 spec lint passes; `task gen` cleanly regenerates `server.gen.go` and `types.gen.go` with the new `code` field on all 6 entity schemas.
  6. Migration 000002 (still editable per D-61) is amended in place — no migration 003 is added; `task db:reset` rebuilds the schema cleanly from a Phase 1 baseline.

**Plans**: 6 plans
Plans:

**Wave 0** *(2 plans in parallel — different files, no cross-dep)*

- [ ] 04.1-01-PLAN.md — Migration 000002 amend (up + down): add code TEXT NOT NULL on 6 entities + UNIQUE(org_id, code) + demote external_id to TEXT NULL with partial unique + drop break_reasons.UNIQUE(org_id, name) + idempotent row_number backfill (IDENT-01, IDENT-02, IDENT-03, IDENT-07)
- [ ] 04.1-02-PLAN.md — OpenAPI yaml deltas: 3 new ErrorCode enum values (duplicate_code, duplicate_external_id, immutable_field) + Identity Model info.description link + code on 6 entity + 1 ListItem + 6 Create*Request + 6 Update*Request + new CreateAdapter 409 + new Update* 422 responses (IDENT-01..06)

**Wave 1** *(blocked on Wave 0)*

- [ ] 04.1-03-PLAN.md — sqlc query authoring (6 files: code in InsertX/ListX/UpdateX RETURNING; new GetXByCode + UpsertXByCode) + codecheck.go + codecheck_test.go (validateCodeFormat + validateImmutableCode helpers) + task gen regen of Go server stubs + sqlc structs + TS client (IDENT-04, IDENT-05)

**Wave 2** *(blocked on Wave 1)*

- [ ] 04.1-04-PLAN.md — Handler refactor: fix mapPgError constraint-name introspection in errors.go (fixes Phase 3 SIMPLICITY-REVIEW MED) + project Code in mappers.go + 6 entity handlers gain Layer 1 + Layer 2 + new 409/422 paths + CreateAdapter 409 wrapper + break_reasons drops name_collision (IDENT-01, IDENT-04, IDENT-06)

**Wave 3** *(blocked on Wave 2)*

- [ ] 04.1-05-PLAN.md — Per-entity test suites: 43 new tests across 6 _test.go files (6 standard per entity + TestBreakReasons_DuplicateName_NoConflict) + isolation fixtures rewrite to regex-compliant codes + TestCatalog_CrossOrgSameCode_BothSucceed + NEW migration_idempotent_test.go (IDENT-01..08)

**Wave 4** *(blocked on Wave 3 — sequential, includes human checkpoint)*

- [ ] 04.1-06-PLAN.md — Phase gate: task gen drift check + task test + task lint + task db:reset smoke + cross-AI peer review (Codex + Gemini in parallel per CLAUDE.md HARD RULE) synthesized into 04.1-REVIEW.md + Phase 5 amend checklist (D04_1-25) + human checkpoint approval (IDENT-01..08)

**Branch**: `gsd/phase-04-5-catalog-identity-normalization`
**Source review**: `.planning/phases/03-catalog-crud-go/03-CATALOG-IDENTITY-REVIEW.md` + `03-CATALOG-IDENTITY-REVIEW-RESPONSE.md` (cross-AI peer-review consensus)

### Phase 5: Bulk Import (Go)

**Goal**: An org admin can seed or update any of the six catalog entities in bulk via a single synchronous import endpoint that handles JSON and CSV, tolerates partial row failures, and persists session results for polling.
**Depends on**: Phase 3, Phase 04.1 (catalog identity contract — imports key on `code`, not `external_id`)
**Requirements**: IMP-01, IMP-02, IMP-03, IMP-04, IMP-05, IMP-06, IMP-07, IMP-08
**Success Criteria** (what must be TRUE):

  1. `POST /v1/orgs/{org_id}/catalog/import?entity={type}` accepts a JSON or CSV body for any of the 6 entity types and upserts rows keyed by `(org_id, code)`; running the same payload twice produces no duplicates. *(Updated post Phase 04.1 — was `(org_id, external_id)`.)*
  2. A CSV body containing a UTF-8 BOM, Windows CRLF line endings, and fields with embedded commas, quoted newlines, and escaped quotes is parsed without error or data corruption using Go's `encoding/csv` with explicit BOM handling.
  3. A batch where some rows fail and some succeed returns HTTP 207 with `{ succeeded: [ids], failed: [{row, field, message}] }`; valid rows are persisted even when the same batch contains invalid rows.
  4. A request body exceeding 50 MB or 500 rows returns HTTP 413 with a message pointing to the v0.2 async pathway; a CSV request missing `?schema_version=v0.1` returns HTTP 400 listing supported versions.
  5. `GET /v1/orgs/{org_id}/imports/{id}` returns the full import session (total_rows, succeeded_rows, failed_rows, errors JSONB) for any previously completed import, persisted in the `import_jobs` table.

**Plans**: 8 plans
Plans:

**Wave 0** *(2 plans in parallel — disjoint files)*

- [ ] 05-01-PLAN.md — OpenAPI yaml deltas (6 Import*Request + IdempotencyKeyHeader + ImportJob.status enum + BulkImportResult.idempotent_replay + description rewrites to code) + migration 000003 (import_jobs table + 3 indexes) + sqlcheck tenantTables["import_jobs"] + task gen no-drift commit (IMP-01, IMP-03, IMP-04, IMP-05, IMP-06, IMP-07, IMP-08)
- [ ] 05-02-PLAN.md — Export catalog.ValidateCodeFormat + catalog.MapPgError (rename pass across 7 catalog handler files) + (*OrgTx).BeginSavepoint(ctx) on internal/db/orgdb.go + middleware.BodyLimit (chi factory with Content-Length pre-flight + http.MaxBytesReader wrap) + ≥ 5 bodylimit_test cases (IMP-07)

**Wave 1** *(blocked on Wave 0)*

- [ ] 05-03-PLAN.md — sqlc queries: NEW import_jobs.sql (5 queries: InsertImportJob, FinaliseImportJob, GetImportJob, LookupImportJobByIdempotencyKey, SweepCrashedImportJobs) + APPEND MergeAgentSkill :exec to agent_skills.sql + APPEND InsertAgentStateOnConflictNothing :exec to agent_states.sql + APPEND ResolveSkillCodes :many to skills.sql + task gen regen (IMP-03, IMP-06)

**Wave 2** *(blocked on Wave 1)*

- [ ] 05-04-PLAN.md — internal/imports/ package scaffold: doc.go + handlers.go (Importer struct + Start/Stop lifecycle mirroring state.Server) + coerce.go (D5-01..D5-08 typed pipeline) + parser_json.go (reMarshalAs[T any] F1 helper) + parser_csv.go (stripBOM + newCSVReader) + header.go (entityRegistry + validateHeader) + mappers.go + errors.go (rowError + wrapPgError) + jobs.go (createJob + finaliseJob) + idempotency.go (lookupIdempotentReplay + rehydrateBulkImportResult) + sweep.go (safetySweep + startupSweep verbatim from state/ttl.go) + main_test.go + testutil_test.go + sweep_test.go (≥ 4 clockwork-backed cases) + exhaustive unit tests for coerce/parser_csv/parser_json/header (IMP-02, IMP-04, IMP-06, IMP-08)

**Wave 3** *(blocked on Wave 2)*

- [ ] 05-05-PLAN.md — Per-entity row processors: row_agent.go (Layer 1 + UpsertAgentByCode + InsertAgentStateOnConflictNothing + MergeAgentSkill loop) + row_skill.go + row_queue.go + row_channel.go (D-76 FK code-lookup probe) + row_adapter.go (JSONB passthrough) + row_break_reason.go + chunk.go (outerTx + per-row savepoint + ResolveSkillCodes batched once + POST-COMMIT cache.Del) + 7 paired _test.go (≥ 24 row processor tests + ≥ 5 chunk integration tests) (IMP-01, IMP-03, IMP-04, IMP-05)

**Wave 4** *(blocked on Wave 3)*

- [ ] 05-06-PLAN.md — Top-level handlers: handler_import.go (BulkImportCatalog + importJSON + importCSV + runImportPipeline) + handler_get_job.go (GetImportJob) + ≥ 11 unit dispatch tests + ApiHandlers composite update in cmd/api/main.go (third embed *imports.Importer; LIFO Start/Stop) + DELETE services/api/internal/catalog/notimpl.go + BodyLimit middleware injection in server.NewMux + 3 server chain-ordering tests (IMP-01, IMP-02, IMP-04, IMP-05, IMP-06, IMP-07, IMP-08)

**Wave 5** *(blocked on Wave 4)*

- [ ] 05-07-PLAN.md — Integration tests: testutil HTTP helpers extension + entity × format matrix (≥ 12 happy + ≥ 6 error) + windows-excel-agents.csv (UTF-8 BOM + CRLF + embedded quotes — Pitfall 1 mandatory) + invalid-utf8.csv + oversized-501-rows + idempotency_test (≥ 5 IdempotencyKey + ≥ 1 NoIdempotencyKey double-run) + jobs_test (≥ 3 GetImportJob + ≥ 2 FinaliseJob) + isolation/imports_test (cross-org probes incl. TestImport_CrossOrgSameCode_BothSucceed mirror of Phase 04.1) + isolation/imports_migration_idempotent_test (000003 replay) — total ≥ 35 new test cases (IMP-01..IMP-08)

**Wave 6** *(blocked on Wave 5 — includes human checkpoint)*

- [ ] 05-08-PLAN.md — Phase gate: task gen drift check + task test + task lint + task db:reset smoke + COMBINED cross-AI peer review (Codex + Gemini in parallel per CLAUDE.md HARD RULE — orchestrator override: covers BOTH Phase 04.1 + Phase 5 diff in a single pass to satisfy 04.1 Plan 06 deferred review) synthesized into 05-REVIEW.md + human checkpoint approval + STATE.md advancement (IMP-01..IMP-08)

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

**Plans**: 14 plans
Plans:
**Wave 1** *(scaffold + tooling)*

- [ ] 06-01-PLAN.md — Vite 8 + Lit 3 + decorator support; admin app package.json + tsconfigs; Vitest + happy-dom setup
- [ ] 06-02-PLAN.md — ajv validators codegen; theme CSS files; lit-localize skeleton; errors.ts extended; barrel exports
- [ ] 06-03-PLAN.md — or-catalog-shell (router + sidebar + theme toggle + locale toggle); or-org-picker
- [ ] 06-04-PLAN.md — Primitives: or-data-table, or-cursor-paginator, or-code-input, or-conflict-banner, or-form-wizard; CI extensions

**Wave 2** *(agents exemplar + shell wiring)*

- [ ] 06-05-PLAN.md — or-agent-list, or-agent-detail (skills sub-table), or-agent-form (3-step wizard)
- [ ] 06-06-PLAN.md — Shell agent routes wired + createApiClient bootstrap; Playwright smoke test (checkpoint)

**Wave 3** *(parallel: skills + queues + break-reasons)*

- [ ] 06-07-PLAN.md — or-skill-list, or-skill-detail, or-skill-form (single-step wizard); skill routes in shell
- [ ] 06-08-PLAN.md — or-queue-list (channel_types badges), or-queue-detail, or-queue-form; queue routes in shell
- [ ] 06-09-PLAN.md — or-break-reason-list (routable/display_order), or-break-reason-detail, or-break-reason-form; break-reason routes in shell

**Wave 4** *(parallel: channels + adapters)*

- [ ] 06-10-PLAN.md — or-queue-picker primitive; or-channel-list, or-channel-detail, or-channel-form (3-step wizard); channel routes in shell
- [ ] 06-11-PLAN.md — or-adapter-list (no config column), or-adapter-detail (JSONB textarea), or-adapter-form; adapter routes in shell

**Wave 5** *(status + import + final integration)*

- [ ] 06-12-PLAN.md — or-status-panel (5s polling, state machine, break picker, force flag); status route in shell
- [ ] 06-13-PLAN.md — or-import-page (3-step wizard + drop zone), or-import-result (207 stat cards + failure table); import routes in shell
- [ ] 06-14-PLAN.md — Export audit; CI drift gate + admin build smoke; end-to-end human verification (checkpoint)

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

**Plans**: 13 plans (+16 in Wave 0.0)
Plans:

**Wave 0.0** *(INSERTED 2026-05-18 — UI rewrite: drop Shoelace, adopt Frankenstyle v0.3.8 (Franken UI v3 rebrand) / Lit-based Hardened Web Components + Tailwind v4 + uk-* utility classes. Runs FIRST before Wave 0; existing Phase 7 work paused until W0.0 completes. Plan 07-03 becomes obsolete after W0.0-15.)*

- [ ] 07-w0-01-PLAN.md — Install `frankenstyle@^0.3.8` (npm) + Tailwind v4 in `packages/ui` and `apps/admin`; import `frankenstyle-kit.css` + `hwc-components.iife.js` bundle; verify uk-* classes resolve
- [x] 07-w0-02-PLAN.md — Ember palette via Tailwind v4 `@theme` directives (OKLCh light + dark) — `--primary`, `--background`, `--foreground`, `--ring` etc.; existing `--or-color-*` aliased to new vars in compat layer
- [ ] 07-w0-03-PLAN.md — `/playground` route in admin app: theme toggle (`<uk-theme-switcher>`), responsive grid, empty component slots; Ember reference screenshots saved to `apps/admin/src/playground/references/`
- [ ] 07-w0-10-PLAN.md — `or-button` Lit wrapper around `.uk-btn` (variants default/primary/secondary/ghost/destructive; sizes sm/md/lg; states hover/focus/disabled/loading); playground entry; visual diff vs Ember ref
- [ ] 07-w0-11-PLAN.md — `or-input` wrapper (.uk-input); update or-code-input to use or-input internally; playground entry covering all states
- [ ] 07-w0-12-PLAN.md — `or-select` wrapper (uk-select markup); playground entry
- [ ] 07-w0-13-PLAN.md — `or-card` wrapper (uk-card with header/body/footer slots); playground entry
- [ ] 07-w0-14-PLAN.md — `or-badge` wrapper (uk-label variants default/success/warning/destructive/info); playground entry
- [ ] 07-w0-15-PLAN.md — `or-dialog` wrapper around `<uk-modal>` (Franken UI focus trap built-in — supersedes Plan 07-03); update 7 detail components to use or-dialog; playground entry
- [ ] 07-w0-16-PLAN.md — `or-tabs` wrapper (uk-tab); playground entry
- [ ] 07-w0-17-PLAN.md — `or-icon` lucide SVG sprite (replaces sl-icon Bootstrap icons); playground entry showing 20 most-used icons
- [ ] 07-w0-18-PLAN.md — `or-switch` + `or-checkbox` wrappers (uk-switch + uk-checkbox); single plan covers both; playground entries
- [ ] 07-w0-19-PLAN.md — `or-table` wrapper (uk-table); update or-data-table to use or-table internally; playground entry
- [ ] 07-w0-20-PLAN.md — `or-toast` wrapper (uk-notification); `notifyError(err)` helper for API errors; playground entry with all 4 variants
- [ ] 07-w0-21-PLAN.md — `or-dropdown` wrapper (uk-dropdown); used for row-actions + user menu; playground entry
- [ ] 07-w0-22-PLAN.md — `or-sidebar` (refactor catalog-shell sidebar — section headers CLINICAL/OPERATIONS/ADMIN/SYSTEM; active state; collapse toggle); playground entry
- [ ] 07-w0-30-PLAN.md — Migration: search-and-replace all `sl-*` usages in 42 compound components → `or-*` or uk-classes; remove `@shoelace-style/shoelace` from dependencies; remove Vite include hacks; all existing tests still pass

**Wave 0** *(3 plans in parallel — disjoint files; scaffold + Go CORS + Shoelace audit — Plan 07-03 obsoleted by W0.0-15)*

- [x] 07-01-PLAN.md — Embed package scaffold: package.json + tsconfig + vite.config.ts (library mode) + vitest.config.ts + test-setup.ts + index.html + playwright.config.ts (3-host matrix) + .size-limit.json + 4 e2e spec stubs + 3 stub hosts (react/vue/html with esm.sh CDN imports) + 2 unit test stubs + .gitignore (EMBED-01, EMBED-09, EMBED-10)
- [x] 07-02-PLAN.md — Go CORS middleware (go-chi/cors v1.2.2): cors.go + AllowedOriginsFromEnv parser + 12 unit tests + cors_preflight_test.go + cors_cross_origin_test.go (Phase 1 isolation suite extension) + CORSAllowedOrigins config field + Taskfile dev env default (CORS_ALLOWED_ORIGINS=*) + chi chain insert BEFORE OrgContext (EMBED-02, EMBED-10)
- [ ] ~~07-03-PLAN.md~~ — OBSOLETE: superseded by 07-w0-15-PLAN.md (or-dialog wrapping uk-modal handles focus trap; standalone sl-dialog audit no longer needed)

**Wave 1** *(blocked on Wave 0)*

- [x] 07-04-PLAN.md — Shell refactor IN PLACE: convert ~30 static entity imports to lazy via _composedEnter helper that composes _orgRouteEnter guard + await import(); add routingMode reactive property (default history) + routerAdapter injection seam; 7 new shell unit tests (D7-01, D7-02 eager baseline, D7-03, D7-06, D7-10) (EMBED-01, EMBED-03, EMBED-06)
- [x] 07-05-PLAN.md — HashRouterAdapter class (~30-50 lines wrapping @lit-labs/router Routes) + 10+ unit tests; HASH_PREFIX='open-routing/' guard for Pitfall 9 (coexist with host hash routing); buildHash static helper (D7-05, D7-06) (EMBED-03, EMBED-06)
- [x] 07-13-PLAN.md — Docs + license: README.md (attribute API + event API + theme JSON + CORS + single-embed limitation + browser support + bundle size); LICENSE (MIT or match repo); CHANGELOG.md (0.1.0-pre.1 entry) (D7-15) (EMBED-09)

**Wave 2** *(blocked on Wave 1)*

- [x] 07-06-PLAN.md — OpenRoutingCatalog Custom Element class (~250 lines): @customElement decorator, 5 reactive properties with `accessor` keyword (TC39 2023-11), Shadow DOM mode open, UUIDv7 validation with inline error state (D7-08), theme JSON parse with prototype-pollution guard, per-element createApiClient + auth-expired onResponse middleware (D7-14), composed:true events for request-context (D7-13) + auth-expired, HashRouterAdapter injection into shell via routerAdapter prop (EMBED-02, EMBED-04, EMBED-05, EMBED-06, EMBED-07, EMBED-08)
- [x] 07-10-PLAN.md — Admin smoke regression: extend Phase 6 smoke.spec.ts with 2 new tests covering 4 lazy chunks (agents/skills/queues/break-reasons) + chunk-not-found console error filter (D7-03 mandate to regression-test admin) (EMBED-03)

**Wave 3** *(blocked on Wave 2)*

- [x] 07-07-PLAN.md — Library entry index.ts + locale.ts: side-effect import './embed-element.js' registers Custom Element + configureLocalization module-level bootstrap + applyEmbedLocale per-element helper; NO localStorage, NO navigator.language detection (CONTEXT <out_of_scope>) (EMBED-01, EMBED-02)

**Wave 4** *(blocked on Wave 3)*

- [x] 07-08-PLAN.md — Vitest unit suite for embed-element.test.ts (17+ cases across 4 describe blocks: org-id validation, theme parse, modules forwarding, request-context dispatch, auth-expired event with mock fetch); covers 7-EMBED-02-a/b/c, 7-EMBED-05-a/b/c, 7-EMBED-06-b, 7-EMBED-07-a/b (EMBED-02, EMBED-05, EMBED-06, EMBED-07)
- [ ] 07-09-PLAN.md — Playwright matrix (4 specs × 3 hosts = 30 runs): embed-shadow-dom-isolation (host CSS in + embed CSS out + no portal escape from <sl-dialog> replacement), embed-org-id-header (page.route intercept), embed-auth-expired (addInitScript + composed:true verified), embed-modules-filter (sidebar nav pierce); covers 7-EMBED-04-a/b/c, 7-EMBED-06-a, 7-EMBED-07-c, 7-EMBED-10-a (EMBED-02, EMBED-03, EMBED-04, EMBED-06, EMBED-07, EMBED-10)
- [ ] 07-12-PLAN.md — Final package.json shape + build smoke: pnpm build emits dist/embed.js valid ES module ≤ 70 KB gzipped + ≥ 6 lazy chunks; pnpm pack --dry-run + scratch-dir install smoke (Pitfall 5 pnpm#10195 mitigation); D7-15 npm tarball contract verified (EMBED-01, EMBED-09)

**Wave 5** *(blocked on Wave 4)*

- [ ] 07-11-PLAN.md — CI jobs (D-30 LOCKED required-status-checks): web-bundle-size (size-limit + andresz1/size-limit-action@v1 PR comment delta; hard fail > 70 KB) + web-e2e-embed (Playwright matrix + ms-playwright browser cache keyed on pnpm-lock.yaml + on-failure artifact upload); all actions pinned @v4/@v1 per ASVS V14.3 (EMBED-01, EMBED-10)

**Branch**: `gsd/phase-07-web-component-embed`
**UI hint**: yes

---

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7

**Overall:** 2 / 7 phases complete (29%).

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation & Polyglot Monorepo | 11/11 | Complete   | 2026-05-15 |
| 2. OpenAPI Contract & Codegen | 6/6 | Complete | 2026-05-16 |
| 3. Catalog CRUD (Go) | 10/10 | Complete   | 2026-05-16 |
| 4. Agent State Machine (Go) | 6/6 | Complete   | 2026-05-17 |
| 5. Bulk Import (Go) | 0/8 | Not started | - |
| 6. Shared UI Library & Standalone Admin | 0/14 | In progress | - |
| 7. Web Component Embed Bundle | 11/30 | In Progress|  |

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
*Stack: Go + chi + sqlc + pgx + golang-migrate + PostgreSQL 17 + Redis + OpenAPI 3.0 + Vite + Lit + Shoelace + Web Components*
*Phase numbering: sequential, starting at 1*
