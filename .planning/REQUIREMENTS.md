# Requirements: Open Routing — v0.1 Catalog Foundation

**Defined:** 2026-05-15
**Milestone:** v0.1 Catalog Foundation
**Core Value:** Product teams can define, simulate, debug, publish, and embed powerful routing flows quickly without Open Routing becoming a media platform, agent desktop, CRM, or ticketing system.

> v0.1 ships the catalog data model + agent status machine + a standalone admin app + a Web Component embed bundle. No flows, no routing execution, no real adapters. Everything in v0.1 is foundational — every later milestone (flows, runtime, simulator) depends on this layer being correct.

**Locked stack (see PROJECT.md Key Decisions for rationale):**
- Backend: Go + chi + sqlc + pgx + golang-migrate + slog
- Database: PostgreSQL 17 + Redis
- API contract: OpenAPI 3.1 → oapi-codegen (Go) + openapi-typescript (TS client)
- Frontend: Vite + Lit + Shoelace + TypeScript
- Embedding: Web Components (Custom Elements + Shadow DOM)
- Repo: polyglot monorepo (Go + pnpm workspaces)

## v0.1 Requirements

### Foundation & Multi-Org Isolation

- [ ] **FOUND-01**: Repository is structured as a polyglot monorepo with `services/api` (Go module), `web/` (pnpm workspace containing `apps/admin`, `apps/embed`, `packages/ui`), `openapi/openapi.yaml`, and `migrations/` at the root.
- [ ] **FOUND-02**: PostgreSQL 17 schema includes `org_id UUID NOT NULL` on every org-scoped table; golang-migrate migration files enforce this constraint.
- [ ] **FOUND-03**: API extracts `org_id` exclusively from the `X-Org-Id` request header via chi middleware; requests missing or with malformed `org_id` return HTTP 400 and never read `org_id` from request body or query parameters.
- [ ] **FOUND-04**: All database access flows through an `orgDB` wrapper in `services/api/internal/db` that automatically injects `WHERE org_id = $N` on every query; service handlers receive only `orgDB` instances, never raw `pgx` pools.
- [ ] **FOUND-05**: Request `org_id` propagates through the call chain via Go `context.Context` from middleware → handler → service → repository; no global state, no struct field passing.
- [ ] **FOUND-06**: Every catalog table enforces `UNIQUE (org_id, external_id)` as a composite constraint; no table has a global `UNIQUE (external_id)`.
- [ ] **FOUND-07**: OpenTelemetry Go SDK initializes before chi router setup; every `slog` log line and OTel span carries an `org_id` attribute extracted from `context.Context`.
- [ ] **FOUND-08**: Integration test suite seeds two orgs with overlapping `external_id` values and exercises every CRUD endpoint, proving zero cross-org data leakage; tests run with `pgx` against a real Postgres 17 container.
- [ ] **FOUND-09**: GitHub Actions CI runs `go vet`, `go test`, `golangci-lint`, frontend typecheck/lint/unit tests, and the two-org isolation suite on every pull request; merge blocked on any failure.
- [ ] **FOUND-10**: Docker Compose brings up PostgreSQL 17, Redis, the Go API, and the Vite dev servers for local development with a single command (`docker compose up`).

### API Contract (OpenAPI)

- [ ] **CONTRACT-01**: `openapi/openapi.yaml` defines every v0.1 REST endpoint, request/response schema, and error shape as a single OpenAPI 3.1 document.
- [ ] **CONTRACT-02**: Go server stubs (handler interfaces, request/response types, validators) are generated from `openapi/openapi.yaml` via `oapi-codegen` and checked into the repository under `services/api/internal/api/`.
- [ ] **CONTRACT-03**: TypeScript client (typed fetch wrappers, schemas) is generated from `openapi/openapi.yaml` via `openapi-typescript` and consumed by both `apps/admin` and `apps/embed` via `packages/ui` re-export.
- [ ] **CONTRACT-04**: A CI check fails the build if the committed generated code diverges from what the spec would produce (`go generate ./...` + `pnpm gen:api` produce no diff).

### Catalog CRUD

- [ ] **CAT-01**: Org admin can create, read, update, and soft-delete `agents` (`id, external_id, name, email, enabled, version`) via REST API.
- [ ] **CAT-02**: Org admin can create, read, update, and soft-delete `skills` (`id, external_id, name, description, skill_type, enabled, version`) via REST API.
- [ ] **CAT-03**: Org admin can assign and unassign skills to/from agents with a `proficiency` integer (validated 1–10) on each `agent_skills` join row.
- [ ] **CAT-04**: Org admin can create, read, update, and soft-delete `queues` (`id, external_id, name, channel_types, priority, acw_sec, enabled, version`) via REST API.
- [ ] **CAT-05**: Org admin can create, read, update, and soft-delete `channels` (`id, external_id, name, channel_type, default_queue_id, enabled`) via REST API.
- [ ] **CAT-06**: Org admin can create, read, update, and soft-delete `adapters` (`id, name, adapter_type, config JSONB, enabled`) via REST API; `config` is a free-form JSONB blob with no vendor-specific fixed columns.
- [ ] **CAT-07**: Org admin can create, read, update, and soft-delete `break_reasons` (`id, name, routable BOOLEAN, display_order, enabled`) via REST API.
- [ ] **CAT-08**: All update endpoints require a `version` field on the request body; version mismatch returns HTTP 409 with the current server-side record.
- [ ] **CAT-09**: Soft-deleted entities (`enabled=false`) are excluded from default list responses; an explicit `?include_disabled=true` query parameter is required to surface them.
- [ ] **CAT-10**: List endpoints support cursor-based pagination, filtering by `enabled`, and a case-insensitive `name` search.
- [ ] **CAT-11**: Hot-path read queries (single-entity GETs, status lookups) cache through Redis with a TTL of 60s and a cache key namespaced as `or:{orgId}:{entity}:{id}`; cache invalidates on write within the same request.

### Agent State Model

- [ ] **STATE-01**: System persists an `agent_states` row per agent with `status ∈ {Ready, NotReady, Break, Engaged, WrapUp, Offline}`.
- [ ] **STATE-02**: `PATCH /agents/{id}/status` accepts agent-initiated transitions: NotReady↔Ready, Ready→Break, Break→Ready, Break→NotReady, WrapUp→Ready, WrapUp→NotReady.
- [ ] **STATE-03**: Transitions not in the allowed matrix (e.g. Engaged→NotReady) return HTTP 409 with body `{"from", "to", "error": "invalid_transition"}`.
- [ ] **STATE-04**: `Ready → Break` transitions require a `break_reason_id` that exists in the same org's `break_reasons` table; missing or cross-org reason ids return HTTP 422.
- [ ] **STATE-05**: `Engaged` state carries an `engaged_channel ∈ {voice, chat, email}` value; system-initiated `Ready → Engaged` transitions require `engaged_channel` as a parameter.
- [ ] **STATE-06**: Agent can set `post_interaction_state ∈ {ready, not_ready}` while Engaged; the system applies it as the target of the next post-WrapUp transition.
- [ ] **STATE-07**: WrapUp has a server-owned TTL (`wrapup_until` column); a Go background goroutine fires the WrapUp → `post_interaction_state` transition on expiry, independent of client connectivity.
- [ ] **STATE-08**: Every state mutation increments a monotonic `state_version` integer used by the UI to detect stale events.
- [ ] **STATE-09**: System-initiated `Offline → NotReady` fires on login event; system-initiated `* → Offline` fires on logout or session timeout.
- [ ] **STATE-10**: `IsRoutable(state AgentState) bool` helper in `services/api/internal/domain` returns `true` only when `status == Ready` OR (`status == Break` AND the associated break reason has `routable=true`).

### Bulk Import

- [ ] **IMP-01**: Org admin can `POST /v1/orgs/{org_id}/catalog/import?entity={entity_type}` with a JSON or CSV body for any of the 6 catalog entities.
- [ ] **IMP-02**: CSV parser normalizes Byte Order Marks, accepts CRLF and LF line endings, and handles quoted fields with embedded commas, newlines, and quotes (Go `encoding/csv` with explicit BOM handling).
- [ ] **IMP-03**: Import upserts rows keyed by `(org_id, external_id)` via `INSERT ... ON CONFLICT (org_id, external_id) DO UPDATE`; existing rows update fields, new rows insert; the same input run twice produces no duplicates.
- [ ] **IMP-04**: Failed rows return structured errors with row number, field name, and a human-readable message; valid rows in the same batch still succeed.
- [ ] **IMP-05**: Import endpoint returns HTTP 207 Multi-Status with body `{ succeeded: [ids], failed: [{row, field, message}] }` when any rows fail; HTTP 200 when all succeed.
- [ ] **IMP-06**: Import sessions persist in `import_jobs` (id, org_id, entity_type, total_rows, succeeded_rows, failed_rows, errors JSONB, created_at); `GET /v1/orgs/{org_id}/imports/{id}` returns the session result.
- [ ] **IMP-07**: Import body is capped at 50 MB and 500 rows; oversized requests return HTTP 413 with a message pointing to the v0.2 async pathway.
- [ ] **IMP-08**: CSV requests require `?schema_version=v0.1`; mismatched versions return HTTP 400 with a list of supported versions.

### Standalone Admin App

- [ ] **ADMIN-01**: `apps/admin` builds as a Vite SPA written in TypeScript + Lit + Shoelace, deployable as a static bundle to any web host.
- [ ] **ADMIN-02**: Admin app provides list, create, edit, and delete screens for all 6 catalog entities (agents, skills, queues, channels, adapters, break_reasons), reusing Lit components from `packages/ui`.
- [ ] **ADMIN-03**: Admin app reads `org_id` from URL path or a configured stub login screen and stores it in app state; all API calls include `X-Org-Id` derived from app state.
- [ ] **ADMIN-04**: Admin app uses the generated TypeScript API client from `packages/ui` (re-exporting from `openapi-typescript` output); types stay in sync with the OpenAPI spec automatically.
- [ ] **ADMIN-05**: Admin app handles HTTP 409 optimistic-lock failures by re-fetching the entity and surfacing a "Changed by someone else, reload?" affordance.
- [ ] **ADMIN-06**: Admin app applies Shoelace theme tokens via CSS custom properties at the root; theme variants are switchable at runtime.

### Web Component Embed Bundle

- [ ] **EMBED-01**: `apps/embed` builds as a single ES module bundle that registers `<open-routing-catalog>` as a Custom Element on import; bundle size target is ≤ 70 KB gzipped (Lit runtime + Shoelace tree-shaken + app code).
- [ ] **EMBED-02**: Host app mounts the element as `<open-routing-catalog org-id="..." api-base-url="..." theme="..." modules="..."></open-routing-catalog>`; `org-id` flows to all REST calls via the `X-Org-Id` header (never via `window` globals or `localStorage`).
- [ ] **EMBED-03**: Embed renders list + create + edit + delete screens for all 6 catalog entities, reusing the same Lit components from `packages/ui` as `apps/admin`.
- [ ] **EMBED-04**: Embed wraps all rendering inside Shadow DOM; no CSS bleeds in or out; host page styles are unaffected even when host uses an aggressive CSS reset.
- [ ] **EMBED-05**: Embed accepts the `theme` attribute as a JSON-encoded set of CSS custom property tokens (colors, typography, spacing) and applies them on the Shadow DOM host node.
- [ ] **EMBED-06**: Embed accepts the `modules` attribute as a comma-separated entity name list; only listed entities appear in navigation and routes (e.g. `modules="agents,skills,queues"` hides channels/adapters/break_reasons).
- [ ] **EMBED-07**: Embed dispatches `open-routing:request-context` CustomEvent on mount and `open-routing:auth-expired` on HTTP 401 (contract reserved for v1 auth integration); both events bubble across the Shadow DOM boundary via `composed: true`.
- [ ] **EMBED-08**: Embed handles HTTP 409 optimistic-lock failures with the same "Changed by someone else, reload?" affordance as the standalone admin app.
- [ ] **EMBED-09**: Embed bundle is published as `@open-routing/catalog-embed` on the npm registry (or served from a CDN URL) so host apps can `import 'https://cdn.../catalog-embed.js'` or `import '@open-routing/catalog-embed'`.
- [ ] **EMBED-10**: Integration test loads the embed inside a Playwright stub host page (React 18, Vue 3, and plain HTML variants) and verifies (a) Shadow DOM CSS isolation in both directions, (b) `org-id` propagation on every API call, (c) `open-routing:auth-expired` fires on 401, (d) bundle gzipped size is under 70 KB.

## Future Requirements (v0.2+)

Tracked but deliberately deferred from v0.1.

### Flow Authoring

- **FLOW-01**: Visual flow builder UI with graph as source of truth.
- **FLOW-02**: DSL surface for flow import/export/review.
- **FLOW-03**: Flow publish governance (validate → simulate → publish → rollback).
- **FLOW-04**: Compile published flow graph into cached executable plan.

### Routing Runtime

- **RT-01**: Runtime engine executes compiled flow plans against catalog snapshots.
- **RT-02**: Route decision p95 < 50 ms target.
- **RT-03**: Reservation lifecycle (offer/accept/timeout/retry).
- **RT-04**: State projections, append-only events, PostgreSQL outbox.
- **RT-05**: Internal gRPC API for runtime and adapter calls (Buf / Connect-Go alongside OpenAPI).

### Real Adapters

- **ADP-01**: Adapter SDK contract (state, capabilities, commands, events).
- **ADP-02**: Mock voice adapter with conformance test suite.
- **ADP-03**: Mock chat adapter with conformance test suite.
- **ADP-04**: Mock email adapter with conformance test suite.

### Simulator & Debug

- **SIM-01**: Deterministic simulator with synthetic and replayed interaction input.
- **SIM-02**: Effect-node record/mock during debug and replay.
- **SIM-03**: Trace viewer with deterministic replay.

### Production-Grade Auth & RBAC

- **AUTH-01**: Tenant API key + admin token (replacing stub `X-Org-Id` header).
- **AUTH-02**: Standalone login + embedded SSO support.
- **AUTH-03**: Role-based access control on catalog endpoints.

### Additional Embedding Surfaces

- **EMB-EXT-01**: Iframe-based embedding alongside Web Components.
- **EMB-EXT-02**: Module Federation 2.x remote for React-native host integration (if customer demand justifies).

### Realtime & Workers

- **RT-PUSH-01**: Server-Sent Events (SSE) channel for agent state push.
- **RT-PUSH-02**: Redis pub/sub fan-out for multi-replica deployments.
- **WORK-01**: Shared Worker deduping SSE connections across multi-tab.
- **WORK-02**: Web Worker for client-side CSV parse + preview before upload.

### Async Bulk Import

- **IMP-09**: Async import path for > 500 rows or > 50 MB (Redis-backed job queue, e.g. `riverqueue/river`).
- **IMP-10**: Dry-run import mode.
- **IMP-11**: Error CSV download for failed rows.

### Audit & Catalog Extensions

- **AUD-01**: Org-facing audit log for routing, flow publish, config, and runtime traces.
- **CAT-EXT-01**: Capabilities entity (channel/adapter capabilities).
- **CAT-EXT-02**: Routing strategies entity.
- **CAT-EXT-03**: Flow templates entity.
- **CAT-EXT-04**: Live external-system catalog sync (one provider as proof).

## Out of Scope (v0.1)

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Real media / call control | Project boundary: Open Routing bridges to channel systems but does not hold calls, streams, chats, or email sessions. |
| Agent desktop UI | Out of scope per PROJECT.md; agent work surfaces belong to host products. |
| WFM, CRM, ticketing | External systems integrated through adapters or host apps, not built in v0.1. |
| Iframe and Module Federation embedding | Web Components is the only v0.1 embed surface; iframe and MF deferred. |
| Real auth / SSO / RBAC | v0.1 ships stub auth (`X-Org-Id` header from trusted host); full auth deferred. |
| Flow execution & runtime engine | Catalog foundation only; runtime arrives after data model is stable. |
| Real adapter execution | v0.1 stores adapter registry rows only; SDK contract and execution deferred. |
| Async bulk import | v0.1 sync only, <500 rows; async path deferred to v0.2. |
| Multi-region active-active | Architecture must not block it, but not delivered in v0.1. |
| PostgreSQL RLS in default migrations | App-layer `orgDB` is primary defense; RLS deferred to operator-controlled hardening (PgBouncer transaction-mode incompatibility). |
| Capabilities / strategies / flow-templates entities | Deferred; catalog v0.1 covers only the 6 entities needed for foundational routing data. |
| Live external-system catalog sync | Sync deferred; v0.1 ships bulk import only. |
| Realtime push (SSE/WebSocket) | REST polling only in v0.1; push channels arrive with the runtime engine. |
| Browser workers (service / shared / web) | No offline, no push notifications, no SSE dedup, no client-side CSV preview in v0.1. |
| gRPC internal API | REST + OpenAPI only in v0.1; gRPC (Buf/Connect) is a runtime-engine concern. |
| Audit logging | Deferred to the milestone that ships the runtime engine. |
| React-based UI / Module Federation / shadcn aesthetic | Replaced by Lit + Shoelace Web Components in v0.1; see Key Decisions in PROJECT.md. |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| FOUND-01 | Phase 1 | Pending |
| FOUND-02 | Phase 1 | Pending |
| FOUND-03 | Phase 1 | Pending |
| FOUND-04 | Phase 1 | Pending |
| FOUND-05 | Phase 1 | Pending |
| FOUND-06 | Phase 1 | Pending |
| FOUND-07 | Phase 1 | Pending |
| FOUND-08 | Phase 1 | Pending |
| FOUND-09 | Phase 1 | Pending |
| FOUND-10 | Phase 1 | Pending |
| CONTRACT-01 | Phase 2 | Pending |
| CONTRACT-02 | Phase 2 | Pending |
| CONTRACT-03 | Phase 2 | Pending |
| CONTRACT-04 | Phase 2 | Pending |
| CAT-01 | Phase 3 | Pending |
| CAT-02 | Phase 3 | Pending |
| CAT-03 | Phase 3 | Pending |
| CAT-04 | Phase 3 | Pending |
| CAT-05 | Phase 3 | Pending |
| CAT-06 | Phase 3 | Pending |
| CAT-07 | Phase 3 | Pending |
| CAT-08 | Phase 3 | Pending |
| CAT-09 | Phase 3 | Pending |
| CAT-10 | Phase 3 | Pending |
| CAT-11 | Phase 3 | Pending |
| STATE-01 | Phase 4 | Pending |
| STATE-02 | Phase 4 | Pending |
| STATE-03 | Phase 4 | Pending |
| STATE-04 | Phase 4 | Pending |
| STATE-05 | Phase 4 | Pending |
| STATE-06 | Phase 4 | Pending |
| STATE-07 | Phase 4 | Pending |
| STATE-08 | Phase 4 | Pending |
| STATE-09 | Phase 4 | Pending |
| STATE-10 | Phase 4 | Pending |
| IMP-01 | Phase 5 | Pending |
| IMP-02 | Phase 5 | Pending |
| IMP-03 | Phase 5 | Pending |
| IMP-04 | Phase 5 | Pending |
| IMP-05 | Phase 5 | Pending |
| IMP-06 | Phase 5 | Pending |
| IMP-07 | Phase 5 | Pending |
| IMP-08 | Phase 5 | Pending |
| ADMIN-01 | Phase 6 | Pending |
| ADMIN-02 | Phase 6 | Pending |
| ADMIN-03 | Phase 6 | Pending |
| ADMIN-04 | Phase 6 | Pending |
| ADMIN-05 | Phase 6 | Pending |
| ADMIN-06 | Phase 6 | Pending |
| EMBED-01 | Phase 7 | Pending |
| EMBED-02 | Phase 7 | Pending |
| EMBED-03 | Phase 7 | Pending |
| EMBED-04 | Phase 7 | Pending |
| EMBED-05 | Phase 7 | Pending |
| EMBED-06 | Phase 7 | Pending |
| EMBED-07 | Phase 7 | Pending |
| EMBED-08 | Phase 7 | Pending |
| EMBED-09 | Phase 7 | Pending |
| EMBED-10 | Phase 7 | Pending |

**Coverage:**
- v0.1 requirements: 53 total
- Mapped to phases: 53
- Unmapped: 0

---
*Requirements defined: 2026-05-15*
*Last updated: 2026-05-15 — stack pivot (Go + Lit + Shoelace + Web Components), 53 requirements; traceability populated across 7 phases*
