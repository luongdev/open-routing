# Research Summary: Open Routing v0.1 Catalog Foundation

**Project:** Open Routing — v0.1 Catalog Foundation
**Domain:** Embeddable multi-channel ACD-style routing platform (B2B, multi-org)
**Researched:** 2026-05-15
**Confidence:** HIGH (domain findings); SUPERSEDED (stack recommendations)

> ⚠ **Stack pivot — 2026-05-15 (post-research):** Original research recommended Node.js + Fastify + Drizzle + React + Module Federation. User locked an alternative stack: **Go + chi + sqlc + pgx + PostgreSQL 17 + Redis + Vite + Lit + Shoelace + Web Components + OpenAPI 3.1 contract**. See PROJECT.md Key Decisions for full rationale. The **domain findings** in this document (8-vendor agent state analysis, catalog entity shapes, pitfall taxonomy, MFE-specific pitfalls that now apply to Web Components/Shadow DOM, bulk import patterns) remain authoritative. The **specific framework/library recommendations** (Fastify/Drizzle/React/Module Federation/Zod sections) are superseded and should NOT be used to plan implementation — translate the pattern (e.g. "OrgScopedDb wrapper") to the locked stack (e.g. "Go `orgDB` wrapper around `pgx.Pool`").

---

## What We Are Building (1-page recap)

Open Routing is a routing brain and configuration surface that product teams embed into their own applications. It is not a contact-center suite, not a media platform, and not an agent desktop. v0.1 ships the catalog layer — the foundational data model that every subsequent milestone (flows, runtime, simulator) depends on.

**v0.1 ships exactly:**
- A normalized catalog of 6 entities: agents, skills, queues, channels, adapters, break_reasons
- A validated agent status model with 3 agent-toggleable states and 3 system-set states, plus configurable break sub-reasons with a per-reason `routable: bool` flag
- Multi-org isolation from the first line of code: shared PostgreSQL with `org_id` on every org-scoped table, enforced at app layer
- A REST management API for full CRUD on all 6 entities, plus JSON/CSV bulk import (sync, partial-success)
- An MFE-embeddable Catalog config UI delivered as a Module Federation 2.x remote

**v0.1 deliberately excludes:** flows, runtime engine, routing decisions, real adapter execution, standalone auth/SSO, real-time agent presence, WFM, CRM, agent desktop, iframe embed variant.

---

## Resolved Stack Conflicts

### Backend Framework: Fastify 5.8.5 (not Hono)

STACK.md recommends Fastify 5.8.5; ARCHITECTURE.md recommends Hono. **Fastify wins** for these reasons:

1. **gRPC coexistence path:** `@connectrpc/connect-fastify` 2.1.1 is a first-class production-tested Fastify plugin. Adding the v1 runtime gRPC API is `npm install + fastify.register` — no architecture change. Hono's gRPC story requires a separate adapter layer.
2. **OpenTelemetry maturity:** `@opentelemetry/auto-instrumentations-node` instruments Fastify HTTP automatically. Hono requires manual span configuration.
3. **Plugin encapsulation:** Fastify's `fastify.register(plugin, { prefix })` maps directly to the `packages/api` package boundary and the future control-plane/runtime split. Each domain (agents, skills, queues) is a registered plugin.
4. **Performance sufficient:** Fastify at ~70k req/s vs Hono ~140k req/s. The catalog API is not the routing hot path. p95 < 50ms is a runtime-engine target for v1, not a catalog CRUD target.

**Why not Hono:** Hono's edge-runtime portability (Cloudflare Workers, Deno Deploy) is irrelevant — Open Routing's control-plane runs Node.js 22.x behind a reverse proxy. Revisit if edge deployment becomes a target at the runtime-engine milestone.

### MFE Plugin: `@module-federation/vite` 1.15.4 (not `@originjs/vite-plugin-federation`)

`@module-federation/vite` 1.15.4 is the official Module Federation 2.x implementation (April 2026 GA) with improved shared dependency resolution. `@originjs/vite-plugin-federation` implements MF 1.x semantics and lacks the MF 2.x runtime improvements that make `singleton: true` reliable across version mismatches.

### Multi-Org RLS Posture: App-Layer Primary, RLS Optional

STACK.md says "NOT RLS" (PgBouncer transaction-mode incompatibility). ARCHITECTURE.md wants belt-and-suspenders. **Reconciled:**

- **Primary defense (mandatory): app-layer `OrgScopedDb` wrapper.** Works in all deployment modes including PgBouncer transaction-mode.
- **RLS optional:** Operators with session-mode PgBouncer or direct connection pools can add RLS as hardening. It must NOT be assumed present in application code.
- **Why app-layer is primary:** RLS failures in PgBouncer transaction mode return silent empty rows, not errors. Silent empty rows in routing decisions are catastrophic. App-layer failures are loud (exceptions, DB constraint violations).
- **v0.1 decision:** Ship app-layer enforcement only. Document RLS as an optional operator hardening step. No RLS in the v0.1 migration scripts.

---

## Locked Stack

| Technology | Version | Purpose |
|------------|---------|---------|
| Node.js | 22.x LTS | Runtime |
| TypeScript | 6.0.3 | Language — `strict` mode mandatory |
| Fastify | 5.8.5 | REST management API |
| Drizzle ORM | 0.45.2 | PostgreSQL data access |
| drizzle-kit | 0.31.10 | Schema migrations |
| postgres (driver) | 3.4.9 | PostgreSQL wire driver |
| React | 19.2.6 | MFE catalog UI |
| Vite | 8.0.13 | MFE build tooling |
| `@module-federation/vite` | 1.15.4 | Module Federation 2.x remote |
| Zod | 4.4.3 | Schema validation (API + import) |
| pnpm | 11.1.2 | Package manager |
| Turborepo | latest | Monorepo task orchestration |
| Vitest | 4.1.6 | Testing |
| fast-csv | 5.0.7 | CSV streaming parser (BOM + CRLF required) |
| `@opentelemetry/sdk-node` | 0.218.0 | OTel SDK — initialize before plugins |
| pino | bundled in Fastify | Structured logging |
| shadcn/ui + tailwindcss 4.x | current / 4.x | MFE UI + styling (configure `prefix: 'or-'`) |

**Deferred to v1:** `@connectrpc/connect-fastify`, Redis, XState, pg-boss (async jobs).

---

## Monorepo Layout

```
open-routing/
├── apps/
│   ├── api/              # Fastify REST management API (control-plane)
│   └── catalog-ui/       # React 19 + Vite + Module Federation remote
├── packages/
│   ├── db/               # Drizzle schema + migrations + OrgScopedDb wrapper
│   ├── domain/           # Pure TypeScript entities, state machine (no I/O)
│   ├── services/         # Business logic, framework-agnostic
│   └── shared-types/     # Zod API schemas + DTOs (consumed by api/ AND catalog-ui/)
├── pnpm-workspace.yaml
└── turbo.json
```

**Key boundary rules:** `catalog-ui` never imports from `packages/db` or `packages/services`. Services receive only `OrgScopedDb`, never raw `DrizzleDb`. `packages/domain` has zero framework dependencies (ready for a future `apps/runtime`).

---

## Agent State Model (Finalized)

### States

| State | Actor | Routing Behavior |
|-------|-------|-----------------|
| `Ready` | Agent-toggleable | Routable |
| `NotReady` | Agent-toggleable | Not routable (no sub-reasons) |
| `Break` | Agent-toggleable | Per-reason `routable: bool` — most granular model across all 8 vendors surveyed |
| `Engaged` | System-set | Not routable for new contacts |
| `WrapUp` | System-set | Configurable; server owns TTL |
| `Offline` | System-set | Not routable |

### `AgentStatusRecord` Shape

```typescript
interface AgentStatusRecord {
  agentId:              string;
  orgId:                string;
  status:               AgentStatus;
  engagedChannel:       'voice' | 'chat' | 'email' | null;  // null unless Engaged
  breakReasonId:        string | null;                       // null unless Break
  postInteractionState: 'ready' | 'not_ready';               // deferred intent during Engaged
  stateVersion:         number;                              // monotonic — UI rejects stale events
  updatedAt:            Date;
  updatedBy:            'agent' | 'system';
}
```

**Why composite `engagedChannel` (not flat enum):** Adding a channel type in v1 adds one string to the union. A flat enum (`EngagedVoice | EngagedChat | EngagedEmail`) requires DB migration, API contract change, and transition table update per channel.

**Why `postInteractionState`:** Agent clicks "Go NotReady" while Engaged. Without this field, concurrent `Engaged → WrapUp` (system) and `→ NotReady` (agent) silently discard the agent's intent. `postInteractionState` preserves intent; the system reads it when transitioning out of WrapUp.

**Why `stateVersion`:** Multi-tab agents with divergent state views can generate conflicting updates. `stateVersion` lets the UI detect stale state without requiring single-active-session enforcement in v0.1.

**WrapUp TTL:** Server-owned. `wrapup_until` stored in DB. Background scheduler fires `WrapUp → Ready|NotReady` on expiry, independent of client connectivity. Client countdown is UX-only.

### Transition Matrix (anything not listed = HTTP 409)

| From | To | Actor | Guard |
|------|----|-------|-------|
| NotReady | Ready | agent | — |
| Ready | NotReady | agent | — |
| Ready | Break | agent | `breakReasonId` exists in org's `break_reasons` |
| Break | Ready | agent | — |
| Break | NotReady | agent | — |
| WrapUp | Ready | agent | — |
| WrapUp | NotReady | agent | — |
| Ready | Engaged | system | `engagedChannel` provided |
| Engaged | WrapUp | system | — |
| WrapUp | Offline | system | — |
| Ready/NotReady/Break | Offline | system | — |
| Offline | NotReady | system | login event |

---

## 6 Catalog Entities — Minimal v0.1 Field Shapes

All entities share these constraints: `org_id UUID NOT NULL`, `UNIQUE (org_id, external_id)` (never global), `version BIGINT NOT NULL DEFAULT 0` (optimistic locking), `enabled BOOLEAN NOT NULL DEFAULT true`.

**agents:** `id, org_id, external_id, name, email, enabled, version, created_at, updated_at`

**skills:** `id, org_id, external_id, name, description, skill_type, enabled, version, created_at`

**agent_skills (join):** `agent_id FK, skill_id FK, proficiency SMALLINT NOT NULL CHECK (1..10)` — integer only, 10=expert. Named tiers are v2.

**queues:** `id, org_id, external_id, name, description, channel_types TEXT[], priority INT, acw_sec INT, enabled, version, created_at`

**channels:** `id, org_id, external_id, name, channel_type VARCHAR, default_queue_id FK nullable, enabled, created_at`

**adapters:** `id, org_id, name, adapter_type VARCHAR, config JSONB NOT NULL, enabled, created_at` — `config` is JSONB; no vendor-specific fixed columns. Adapter is a leaf node (no FK back to queues — no cycles).

**break_reasons:** `id, org_id, name, routable BOOLEAN NOT NULL DEFAULT false, display_order INT, enabled, created_at`

---

## Multi-Org Enforcement Stack

```
HTTP Request (X-Org-Id: <uuid>)
  → Fastify org-context middleware: validates header, stores in AsyncLocalStorage, stamps audit log
  → Fastify route handler: Zod parse → call service
  → Service: receives OrgScopedDb (never raw DrizzleDb)
  → OrgScopedDb: reads orgId from AsyncLocalStorage, appends WHERE org_id = ? to every query
  → PostgreSQL: org_id NOT NULL constraint hard-fails on any missed insert
```

---

## MFE Topology

```
Host App → renders <CatalogShell orgId="..." apiBaseUrl="..." theme={tokens} modules={visible} />
  ↓ Module Federation 2.x remote load
catalog-ui (apps/catalog-ui)
  Exposes: ./CatalogShell
  OrgContext: orgId from prop (never window globals)
  Emits: routing:request-context on every mount
  Emits: routing:auth-expired on 401 (contract for v1 auth)
  API client: all requests include x-org-id header from OrgContext
  ↓ REST to apiBaseUrl
apps/api (Fastify)
```

Auth upgrade path (v0.1 → v1): add `authToken?: string` prop. No structural MFE changes.

---

## Bulk Import (v0.1 Scope)

Sync, <500 rows, partial success (207). `fast-csv` with BOM stripping + CRLF normalization. Upsert: `INSERT ... ON CONFLICT (org_id, external_id) DO UPDATE`. Failed rows include row number + field + message. Import sessions persist in `import_jobs` table. CSV format versioned from day one (`?schema_version=v0.1`). v1 adds: async pg-boss jobs for >500 rows, dry-run mode, error CSV download.

---

## Top 10 Pitfalls and Prevention

1. **Cache key missing `org_id` prefix** — Silent cross-org cache poisoning. Use `cacheKey(orgId, ...segments)` that throws if `orgId` is falsy. CI test: warm cache for org A, read as org B, assert empty.

2. **RLS with PgBouncer transaction mode = silent empty rows** — Never rely on RLS as primary guard. App-layer `OrgScopedDb` is mandatory. RLS is optional operator hardening.

3. **`UNIQUE (external_id)` without `org_id`** — Org B's import fails on IDs already used by org A. Schema constraint must be `UNIQUE (org_id, external_id)`. Test: same `external_id` values for two orgs must produce 6 rows, not 3.

4. **Agent stuck in WrapUp on browser close** — WrapUp TTL must be server-owned. `wrapup_until` in DB; background scheduler fires transition. Test: kill browser during WrapUp, wait TTL+5s, assert transition.

5. **Race: agent toggles NotReady while Engaged** — `postInteractionState` field resolves this. Toggle during Engaged writes to `postInteractionState`, not `status`. System reads it when transitioning from WrapUp.

6. **Module Federation duplicate React (version skew)** — `singleton: true, requiredVersion: deps.react` in both host and remote. Async bootstrap entry (never `eager: true`). Test in a real host+remote build, not just local dev.

7. **`org_id` missing from async job payload** — Worker thread does not inherit `AsyncLocalStorage`. Serialize `org_id` into every job payload. Worker sets `OrgContext` from `job.orgId` as first action.

8. **`org_id` read from request body** — Attacker sets any `org_id`. `org_id` comes ONLY from `X-Org-Id` header. Body `org_id` is ignored and logged as a security warning.

9. **Invalid state transitions allowed** — Explicit `ALLOWED_TRANSITIONS` matrix in `packages/domain`. Anything not listed returns HTTP 409 `{ "from": "engaged", "to": "not_ready", "error": "invalid_transition" }`.

10. **CSS bleed (Tailwind class collision)** — Configure `prefix: 'or-'` in `tailwind.config.js`. All classes become `.or-flex`, `.or-text-sm`. Screenshot-test MFE inside a host with aggressive CSS reset.

---

## Implications for Roadmap: Suggested Phases

### Phase 1: Monorepo Foundation + Org Isolation Scaffold

**Rationale:** Org isolation bugs compound into every entity and every test written after them. Getting this wrong early means rework in every phase.

**Delivers:** pnpm+Turborepo workspace, `packages/db` with `OrgScopedDb` scaffold, `apps/api` with Fastify + org-context middleware (AsyncLocalStorage) + CORS for `X-Org-Id`, `packages/domain` skeleton, GitHub Actions CI, Docker Compose PostgreSQL 16, OTel initialized, integration test proving two-org isolation.

**Avoids pitfalls:** Cache key without `org_id`, `org_id` from request body, admin endpoints bypassing org middleware, test fixtures bleeding between orgs.

### Phase 2: Catalog CRUD (6 Entities)

**Rationale:** All 6 entities must exist before state machine (Phase 3) can validate Break guards, before import (Phase 4) can write rows, and before MFE (Phase 5) can call real endpoints. `shared-types` drives both API and MFE typed client.

**Delivers:** Drizzle schema migrations for all 6 entities with correct constraints (`org_id NOT NULL`, `version BIGINT`, `UNIQUE (org_id, external_id)`, `proficiency SMALLINT CHECK (1..10)`, `config JSONB`), full REST CRUD, service layer with soft-delete policy, optimistic locking (PUT requires `version`, 409 on conflict), two-org isolation test for each entity.

**Avoids pitfalls:** Global `external_id` uniqueness, missing `version` column, orphaned soft-delete assignments, split-brain proficiency, adapter vendor-specific columns, catalog cyclic FK.

### Phase 3: Agent State Machine

**Rationale:** `break_reasons` (Phase 2) must exist for the Break guard. State machine is pure domain logic, testable without MFE or import.

**Delivers:** `packages/domain/src/agent-state/` (states, transitions, state machine), `agent_states` DB table with `engaged_channel, break_reason_id, post_interaction_state, state_version, wrapup_until`, `AgentStateService`, `PATCH /agents/{id}/status` with HTTP 409 for invalid transitions, server-side WrapUp TTL scheduler, compound index `(org_id, status)`.

**Avoids pitfalls:** Direct Engaged → NotReady, WrapUp stuck on disconnect, concurrent system/agent race, `routable` flag ignored in routing queries, flat Engaged enum, XState before runtime actors.

### Phase 4: Bulk Import

**Rationale:** Import writes to entity tables (Phase 2) and may include initial agent status (Phase 3). Both must be stable.

**Delivers:** `POST /v1/orgs/{orgId}/catalog/import` (multipart), `fast-csv` with BOM+CRLF normalization, Zod per-entity import row schemas, upsert `ON CONFLICT (org_id, external_id)`, 207 Multi-Status partial success, `import_jobs` table (persisted sessions), `GET /import/{id}` polling endpoint, CSV format versioned (`?schema_version=v0.1`), `windows-excel.csv` test fixture in `testdata/`, 50MB body size limit documented.

**Avoids pitfalls:** BOM+CRLF parser failures, partial success not communicated, sessions not persisted, global `external_id` constraint, single-transaction timeout, schema drift on saved templates.

### Phase 5: Catalog MFE

**Rationale:** MFE is a consumer of the API. Substantive work starts only once API endpoints, state model, and import are stable.

**Delivers:** `apps/catalog-ui` with MF 2.x remote (`./CatalogShell`), `OrgContext` (prop-based, emits `routing:request-context` on mount), `routing:auth-expired` event contract, typed fetch client against `shared-types`, CRUD UI for all 6 entities (`@tanstack/react-query` + `@tanstack/react-table` + shadcn/ui), Tailwind `prefix: 'or-'`, theme token injection via CSS custom properties, module visibility flags, async bootstrap entry (no `eager: true`), integration test in stub host with CSS reset.

**Avoids pitfalls:** `eager: true` deadlock, duplicate React instances, CSS bleed, `org_id` lost on navigation, auth token expiry contract not established, `org_id` via window globals.

### Phase Ordering Rationale

- Foundation first: isolation bugs propagate into every entity and test.
- Catalog before state machine: `break_reasons` is a catalog entity; the Break guard validates against it.
- Catalog before import: entity tables must have correct schemas before import workers write to them.
- Catalog before MFE: `shared-types` Zod schemas are the API contract; MFE typed client is generated from them.
- MFE last: it is a consumer. Scaffolding in Phase 1; substantive work after API shapes stabilize.

### Research Flags

All 5 phases use well-documented patterns — no additional research-phase needed during planning. The one integration validation gap is **Phase 5**: test against a real framework-level host app (not just Vite test harness) to verify MF singleton resolution and CSS isolation before marking the phase complete.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All versions npm-verified 2026-05-15; framework choices cross-validated against official docs |
| Features | HIGH | Agent state model validated against 8 vendor sources with explicit API docs; bulk import patterns verified against HubSpot/Salesforce/Tyk |
| Architecture | HIGH | Org isolation pattern confirmed across AWS, Crunchy Data, permit.io, simplyblock.io; monorepo layout is standard pnpm+Turborepo |
| Pitfalls | HIGH (multi-org, MFE, import); MEDIUM (state machine race conditions) | Multi-org and MFE pitfalls are confirmed in post-mortem literature; state machine races inferred from Cisco UCCX forum reports |

**Overall: HIGH**

### Gaps to Address During Implementation

1. **WrapUp TTL in multi-replica:** `setInterval` works in v0.1 single process. In v1 multi-replica, WrapUp expiry needs a distributed lock or a single-writer pattern. Flag before v1 multi-replica deployment.
2. **`postInteractionState` default configurability:** Research does not answer whether orgs can configure the default to `not_ready`. Flag for product decision before Phase 3.
3. **Phase 5 host integration test harness:** A minimal Playwright + host shell must be set up at the start of Phase 5, before component work begins.
4. **Import format versioning communication:** The `?schema_version=v0.1` approach is specified but the customer notification mechanism for version changes is not. Flag before v0.2 ships with schema changes.

---

**Ready for roadmap: yes**
