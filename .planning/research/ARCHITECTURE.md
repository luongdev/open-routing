# Architecture Research

**Domain:** Embeddable multi-channel routing platform — v0.1 Catalog Foundation
**Researched:** 2026-05-15
**Confidence:** HIGH (stack decisions), MEDIUM (MFE topology detail), HIGH (org isolation pattern)

---

## 1. Layered Architecture and Module Structure

### Recommendation: Hono + pnpm workspace monorepo, single deployable with internal module boundaries

**Why Hono over NestJS:** NestJS adds 500-2000ms cold starts, enforces a specific DI container,
and makes it hard to later extract the runtime-engine as a separate process without major refactoring.
Hono is ultrafast (~14 kB), runtime-portable, and produces thin application code that is easy to
split at a future boundary. For a team building toward a control-plane/runtime-engine split, Hono's
module-per-concern style maps naturally to future service extraction — the NestJS module system
creates lock-in that would need to be unwound.

**Why pnpm workspace + Turborepo over a NestJS built-in monorepo:** The project must co-host a
TypeScript API, shared domain types, and a React MFE. NestJS's built-in monorepo is Node-only.
pnpm + Turborepo gives build caching, cross-package type sharing, and supports heterogeneous apps
(backend, MFE) in one repo with no tool lock-in.

### Repository Layout

```
open-routing/
├── apps/
│   ├── api/                        # Hono HTTP server — control-plane entry point in v0.1
│   │   ├── src/
│   │   │   ├── main.ts             # Hono app factory, middleware chain, route mount
│   │   │   ├── middleware/
│   │   │   │   ├── org-context.ts  # Extract org_id header → AsyncLocalStorage
│   │   │   │   ├── audit-log.ts    # Log every request with org_id, resource, action
│   │   │   │   └── error-handler.ts
│   │   │   ├── routes/
│   │   │   │   ├── agents.ts
│   │   │   │   ├── skills.ts
│   │   │   │   ├── queues.ts
│   │   │   │   ├── channels.ts
│   │   │   │   ├── adapters.ts
│   │   │   │   ├── break-reasons.ts
│   │   │   │   └── import.ts       # Bulk import endpoint
│   │   │   └── openapi.ts          # Zod → OpenAPI schema generation
│   │   └── package.json
│   │
│   └── catalog-ui/                 # React MFE — Vite + Module Federation remote
│       ├── src/
│       │   ├── bootstrap.ts        # MFE entry (deferred import for Module Federation)
│       │   ├── App.tsx
│       │   ├── components/
│       │   │   ├── agents/
│       │   │   ├── skills/
│       │   │   ├── queues/
│       │   │   ├── channels/
│       │   │   ├── adapters/
│       │   │   └── break-reasons/
│       │   ├── context/
│       │   │   └── OrgContext.tsx  # Receives org_id from host via props/context
│       │   └── api/
│       │       └── client.ts       # Typed fetch client, injects org_id header
│       └── vite.config.ts          # Module Federation plugin config
│
├── packages/
│   ├── domain/                     # Pure TypeScript — no framework dependencies
│   │   ├── src/
│   │   │   ├── catalog/
│   │   │   │   ├── agent.ts        # Agent entity, types
│   │   │   │   ├── skill.ts
│   │   │   │   ├── queue.ts
│   │   │   │   ├── channel.ts
│   │   │   │   ├── adapter.ts
│   │   │   │   └── break-reason.ts
│   │   │   ├── agent-state/
│   │   │   │   ├── state-machine.ts  # Transition table + guard functions
│   │   │   │   ├── states.ts         # AgentStatus enum
│   │   │   │   └── transitions.ts    # Allowed transition matrix
│   │   │   └── org.ts
│   │   └── package.json
│   │
│   ├── db/                         # Database layer — Drizzle schema + OrgScopedDb
│   │   ├── src/
│   │   │   ├── schema/
│   │   │   │   ├── agents.ts
│   │   │   │   ├── skills.ts
│   │   │   │   ├── queues.ts
│   │   │   │   ├── channels.ts
│   │   │   │   ├── adapters.ts
│   │   │   │   ├── break-reasons.ts
│   │   │   │   ├── agent-states.ts
│   │   │   │   └── import-jobs.ts
│   │   │   ├── org-scoped-db.ts   # OrgScopedDb wrapper — enforces org_id on all queries
│   │   │   ├── migrations/
│   │   │   └── client.ts          # Drizzle + pg pool init
│   │   └── package.json
│   │
│   ├── services/                   # Business logic — framework-agnostic
│   │   ├── src/
│   │   │   ├── catalog/
│   │   │   │   ├── agent-service.ts
│   │   │   │   ├── skill-service.ts
│   │   │   │   ├── queue-service.ts
│   │   │   │   ├── channel-service.ts
│   │   │   │   ├── adapter-service.ts
│   │   │   │   └── break-reason-service.ts
│   │   │   ├── agent-state-service.ts  # State transition enforcement
│   │   │   └── import-service.ts       # Bulk import orchestration
│   │   └── package.json
│   │
│   └── shared-types/               # API contracts, DTOs — consumed by both api/ and catalog-ui/
│       ├── src/
│       │   ├── api/
│       │   │   ├── agents.ts       # Request/response Zod schemas
│       │   │   ├── skills.ts
│       │   │   ├── queues.ts
│       │   │   ├── channels.ts
│       │   │   ├── adapters.ts
│       │   │   ├── break-reasons.ts
│       │   │   └── import.ts
│       │   └── index.ts
│       └── package.json
│
├── pnpm-workspace.yaml
├── turbo.json
└── package.json
```

### Layer Responsibilities

| Layer | Location | Responsibility |
|-------|----------|----------------|
| HTTP/REST | `apps/api/src/routes/` | Request parsing, Zod validation, response shaping |
| Middleware | `apps/api/src/middleware/` | org_id extraction, audit logging, error normalization |
| Service | `packages/services/src/` | Business rules, state machine transitions, orchestration |
| Repository | `packages/db/src/org-scoped-db.ts` | Query execution, org_id enforcement, persistence |
| Domain | `packages/domain/src/` | Entities, value objects, state machine rules (no I/O) |
| Schema | `packages/db/src/schema/` | Drizzle table definitions, relations, migrations |

### Future Split Preparation

The control-plane/runtime-engine split is NOT implemented in v0.1, but the structure above enables it:
- `apps/api` becomes `apps/control-plane` — CRUD, import, MFE serving
- `apps/runtime` is added as a separate Hono app that imports `packages/services/` and `packages/domain/`
- `packages/domain/` and `packages/db/` are shared between both apps
- No circular dependencies, no cross-app imports — each app is its own deployable

---

## 2. Multi-Org Data Isolation

### Recommendation: Three-layer belt-and-suspenders — HTTP middleware + OrgScopedDb + PostgreSQL RLS

#### Layer 1: HTTP Middleware extracts and propagates org_id

Use `AsyncLocalStorage` to carry `org_id` for the entire request lifetime. This avoids threading `org_id` through every function argument and makes it available anywhere in the call stack without prop-drilling.

```typescript
// apps/api/src/middleware/org-context.ts
import { AsyncLocalStorage } from 'node:async_hooks';

export const orgStore = new AsyncLocalStorage<{ orgId: string }>();

export const orgContextMiddleware: MiddlewareHandler = async (c, next) => {
  const orgId = c.req.header('x-org-id');
  if (!orgId) return c.json({ error: 'x-org-id header required' }, 400);

  // Stub auth: trust the header in v0.1. Replace with JWT claim extraction in v1.
  await orgStore.run({ orgId }, async () => {
    // Audit: every request is stamped before any service call
    await auditLog({ orgId, path: c.req.path, method: c.req.method });
    await next();
  });
};
```

#### Layer 2: OrgScopedDb — app-layer enforcement on every query

Drizzle ORM has no built-in query interceptor for multi-tenancy. The recommended pattern is a typed wrapper class that automatically injects `org_id` into `WHERE` clauses for all select/update/delete operations and into `VALUES` for inserts.

```typescript
// packages/db/src/org-scoped-db.ts
import { orgStore } from '@open-routing/api/middleware/org-context';

export class OrgScopedDb {
  constructor(private readonly db: DrizzleDb) {}

  private get orgId(): string {
    const store = orgStore.getStore();
    if (!store) throw new Error('OrgScopedDb used outside org context — programming error');
    return store.orgId;
  }

  agents() {
    return {
      findMany: (opts?: FindManyOpts) =>
        this.db.select().from(agentsTable)
          .where(and(eq(agentsTable.orgId, this.orgId), opts?.where)),

      insert: (data: NewAgent) =>
        this.db.insert(agentsTable).values({ ...data, orgId: this.orgId }),

      update: (id: string, data: Partial<Agent>) =>
        this.db.update(agentsTable)
          .set(data)
          .where(and(eq(agentsTable.id, id), eq(agentsTable.orgId, this.orgId))),

      delete: (id: string) =>
        this.db.delete(agentsTable)
          .where(and(eq(agentsTable.id, id), eq(agentsTable.orgId, this.orgId))),
    };
  }
  // ... same pattern for each entity
}
```

Services receive `OrgScopedDb`, not raw `DrizzleDb`. Raw `DrizzleDb` is never exported from `packages/db/` for application use — only for migrations and internal tests.

#### Layer 3: PostgreSQL RLS — database-level backstop

RLS catches bugs in application code (missing OrgScopedDb wrapper, raw query, misconfigured route). It is configured per table with a session variable set at connection time.

```sql
-- Set once per connection (in connection pool `connect` hook)
-- Transaction-scoped to avoid leakage across pooled connections
SET LOCAL app.current_org_id = '<uuid>';

-- Policy per table (example for agents)
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;

CREATE POLICY agents_org_isolation
  ON agents
  USING (org_id = NULLIF(current_setting('app.current_org_id', TRUE), '')::uuid);
```

The API uses a dedicated `app_user` role — NOT the table owner — so RLS policies always apply. The pool `connect` callback runs `SET LOCAL app.current_org_id` from `AsyncLocalStorage` before releasing the connection for query use.

RLS failure results in 0 rows returned, not an error, so application tests must assert record counts — not just absence of errors.

#### Audit Trail

Every cross-org access attempt is detectable via two mechanisms:
1. **Request-level audit log** — middleware stamps every request with `org_id`, `path`, `method`, `timestamp` to a `audit_log` table before any service call.
2. **RLS policy violation log** — if a query returns 0 rows where data is expected, the service layer raises an `OrgIsolationViolation` error, which is logged with full `org_id` and resource context.

```typescript
// packages/services/src/catalog/agent-service.ts
async findById(id: string): Promise<Agent> {
  const result = await this.db.agents().findFirst(id);
  if (!result) {
    // Could be legitimately missing, or could be RLS blocking cross-org access.
    // Log for audit — cannot distinguish at app layer without additional query.
    logger.warn({ orgId: this.db.orgId, agentId: id }, 'agent_not_found_or_org_blocked');
    throw new NotFoundError('agent', id);
  }
  return result;
}
```

#### Test Strategy for No-Leakage Proof

```
For each entity (6 total):
  1. Seed org A with 3 records, org B with 2 records
  2. Make API request with x-org-id: orgA
  3. Assert response contains exactly 3 records, all with orgId === orgA
  4. Assert no record from org B appears in response
  5. Attempt direct DB update of org B record via org A's OrgScopedDb — assert 0 rows affected
  6. Bypass OrgScopedDb and query raw DB as app_user — assert RLS blocks cross-org rows
```

Tests live in `packages/db/src/__tests__/org-isolation.test.ts` and use a real PostgreSQL instance (Testcontainers or a local Docker DB) — no mocking, RLS must be exercised.

---

## 3. Agent State Machine Placement

### Recommendation: Pure TypeScript transition table in `packages/domain/` — no XState in v0.1

**Why not XState:** XState is designed for stateful, long-lived actor instances. Agent state in v0.1 is stored in PostgreSQL and reconstructed per-request. Running an XState machine would require either in-memory actor persistence (stateful process, not stateless-scalable) or serializing/deserializing full machine snapshots to the DB on every transition. Both add complexity without benefit for catalog-foundation-only scope. XState becomes compelling in v1 when the runtime-engine handles live interaction routing and needs persisted actor state. Park it for then.

**Why pure transition table:** The agent state is a small, well-defined set of states with a fixed set of allowed transitions. A transition table encoded as a plain TypeScript record is:
- Easy to test exhaustively with a matrix test
- Readable by anyone without XState knowledge
- Trivially serializable to/from DB enum columns
- Ready to be replaced by XState in v1 with no structural changes to callers

#### State Design

```typescript
// packages/domain/src/agent-state/states.ts

export enum AgentStatus {
  // Agent-toggleable
  Ready     = 'ready',
  NotReady  = 'not_ready',
  Break     = 'break',

  // System-set only
  Engaged   = 'engaged',
  WrapUp    = 'wrap_up',
  Offline   = 'offline',
}

// Engaged sub-state: composite column, not a separate enum.
// Avoids a 3-level nested enum; sub-state is only meaningful when status === Engaged.
export type EngagedChannel = 'voice' | 'chat' | 'email';

// Full agent status record stored in DB
export interface AgentStatusRecord {
  agentId:        string;
  orgId:          string;
  status:         AgentStatus;
  engagedChannel: EngagedChannel | null;  // non-null only when status === Engaged
  breakReasonId:  string | null;           // FK to break_reasons, non-null only when status === Break
  updatedAt:      Date;
  updatedBy:      'agent' | 'system';     // audit field
}
```

**Why composite (status + engagedChannel) not a single flat enum:** A flat enum would require
`EngagedVoice | EngagedChat | EngagedEmail` plus all other states — 8 values for 3 channel types.
Adding a channel type later means adding enum variants across DB migrations, API contracts, and
transition tables. The composite model (`status: Engaged, engagedChannel: 'voice'`) adds a channel
by inserting one value into the `EngagedChannel` union — DB column unchanged.

#### Transition Table

```typescript
// packages/domain/src/agent-state/transitions.ts

type Transition = {
  from:   AgentStatus;
  to:     AgentStatus;
  actor:  'agent' | 'system';
  guard?: (ctx: TransitionContext) => boolean;
};

interface TransitionContext {
  breakReasonId?: string;
  breakReasonExists?: boolean;   // validated before applying
  engagedChannel?: EngagedChannel;
}

// Explicit allowed transitions — anything not listed is forbidden
export const ALLOWED_TRANSITIONS: Transition[] = [
  // Agent-toggleable
  { from: AgentStatus.NotReady,  to: AgentStatus.Ready,   actor: 'agent' },
  { from: AgentStatus.Ready,     to: AgentStatus.NotReady, actor: 'agent' },
  { from: AgentStatus.Ready,     to: AgentStatus.Break,    actor: 'agent',
    guard: (ctx) => !!ctx.breakReasonId && !!ctx.breakReasonExists },
  { from: AgentStatus.Break,     to: AgentStatus.Ready,    actor: 'agent' },
  { from: AgentStatus.Break,     to: AgentStatus.NotReady, actor: 'agent' },
  { from: AgentStatus.WrapUp,    to: AgentStatus.Ready,    actor: 'agent' },
  { from: AgentStatus.WrapUp,    to: AgentStatus.NotReady, actor: 'agent' },

  // System-set
  { from: AgentStatus.Ready,     to: AgentStatus.Engaged,  actor: 'system',
    guard: (ctx) => !!ctx.engagedChannel },
  { from: AgentStatus.Engaged,   to: AgentStatus.WrapUp,   actor: 'system' },
  { from: AgentStatus.WrapUp,    to: AgentStatus.Offline,  actor: 'system' },
  { from: AgentStatus.Ready,     to: AgentStatus.Offline,  actor: 'system' },
  { from: AgentStatus.NotReady,  to: AgentStatus.Offline,  actor: 'system' },
  { from: AgentStatus.Break,     to: AgentStatus.Offline,  actor: 'system' },
  { from: AgentStatus.Offline,   to: AgentStatus.NotReady, actor: 'system' }, // login
];
```

#### How break_reasons Hook In

`break_reasons` is a catalog entity (per-org, CRUD managed). When an agent requests `Break`:
1. `agent-state-service.ts` receives the `breakReasonId` in the transition request.
2. It validates the `breakReasonId` exists in `break_reasons` for the org via `OrgScopedDb`.
3. The guard on the `Ready → Break` transition receives `breakReasonExists: true`.
4. The transition is applied; `AgentStatusRecord.breakReasonId` is set.
5. The `routable` flag on the `break_reason` is NOT stored on the status record — it is read
   from the catalog at routing decision time. This keeps state records normalized.

#### DB Schema for Agent State

```sql
-- In packages/db/src/schema/agent-states.ts (Drizzle)
-- status and engaged_channel stored as text (mapped to TypeScript enum at ORM layer)
-- breakReasonId is FK into break_reasons — referential integrity enforced
CREATE TABLE agent_states (
  agent_id        UUID NOT NULL REFERENCES agents(id),
  org_id          UUID NOT NULL,
  status          TEXT NOT NULL,
  engaged_channel TEXT,            -- null unless status = 'engaged'
  break_reason_id UUID REFERENCES break_reasons(id),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by      TEXT NOT NULL,   -- 'agent' | 'system'
  PRIMARY KEY (agent_id)
);
```

---

## 4. MFE Embed Architecture

### Recommendation: Module Federation 2.0 remote (catalog-ui) consumed by host — Vite + @module-federation/vite

#### Topology

```
Host Application (product team's app — not in this repo)
├── Mounts: <CatalogConfigUI orgId="..." theme={tokens} modules={visibleModules} />
│
└── Loads remote: catalog-ui@[version]
    ├── Exposes: ./CatalogConfigUI   (React component, default export)
    ├── Remote URL: https://cdn.example.com/catalog-ui/remoteEntry.js
    └── Shared: react (singleton), react-dom (singleton), react-router-dom (singleton)

catalog-ui (this repo: apps/catalog-ui)
├── Entry: bootstrap.ts (deferred import — required by Module Federation runtime)
├── Exposes: { './CatalogConfigUI': './src/CatalogConfigUI.tsx' }
└── API calls: x-org-id header injected from OrgContext
```

#### Vite Configuration

```typescript
// apps/catalog-ui/vite.config.ts
import { defineConfig } from 'vite';
import federation from '@module-federation/vite';

export default defineConfig({
  plugins: [
    federation({
      name: 'catalog_ui',
      filename: 'remoteEntry.js',
      exposes: {
        './CatalogConfigUI': './src/CatalogConfigUI.tsx',
      },
      shared: {
        react:         { singleton: true, requiredVersion: '~18.2.0' },
        'react-dom':   { singleton: true, requiredVersion: '~18.2.0' },
      },
    }),
  ],
});
```

#### org_id Flow: Host → MFE → API

```
Host app holds authenticated session (JWT or session cookie)
  │
  ├─→ Reads orgId from session/JWT claim
  ├─→ Renders <CatalogConfigUI orgId={orgId} theme={tokens} modules={visibleModules} />
  │
CatalogConfigUI React component (in catalog-ui remote)
  │
  ├─→ Receives orgId as React prop (NOT from window globals, NOT from localStorage)
  ├─→ Stores in OrgContext (React Context)
  │
API client (apps/catalog-ui/src/api/client.ts)
  │
  ├─→ Reads orgId from OrgContext via useContext
  └─→ Sets header: 'x-org-id': orgId on all requests
```

**Why props, not globals:** `window.__ORG_ID__` creates implicit coupling and breaks when multiple
MFEs or iframes share a page. Props are explicit, type-checked, and auditable. The host always
controls what `orgId` is passed — consistent with the stub-auth model where the host is trusted.

#### Theme Tokens and Module Controls

```typescript
// Prop contract for the exposed component
interface CatalogConfigUIProps {
  orgId:   string;
  theme?: {
    // CSS custom properties passed as object — applied to a wrapping div
    '--or-primary':     string;  // e.g. '#0052CC'
    '--or-text':        string;
    '--or-surface':     string;
    [key: string]:      string;
  };
  modules?: {
    // Explicit per-module visibility — unlisted modules show by default
    agents?:        boolean;
    skills?:        boolean;
    queues?:        boolean;
    channels?:      boolean;
    adapters?:      boolean;
    breakReasons?:  boolean;
  };
}
```

Theme tokens are applied as inline CSS custom properties on the root wrapper element —
no global stylesheet mutation. Module visibility flags control conditional rendering.
Both are props (not context or config files) so the host has declarative control at mount time.

#### Auth Context in v0.1 (Stub Auth)

The host passes `orgId` as a prop. The API trusts the `x-org-id` header. No token or signature in
v0.1. The upgrade path in v1:
- Host passes a short-lived JWT (signed by auth service) as `authToken` prop.
- API middleware verifies JWT signature and extracts `orgId` from the claim instead of trusting the header.
- No structural changes to the MFE component contract — just `authToken` prop added alongside `orgId`.

---

## 5. Bulk Import Architecture

### Recommendation: Async with pg-boss job queue, DB-backed, idempotent by import job ID

**Why not synchronous:** CSV files from external system dumps can be thousands of rows. Parsing,
validating, and persisting in a single HTTP request risks timeouts, re-upload on retry, and double
inserts. Even for "small files only" in v0.1, building async from the start means the polling
endpoint is available and tested before v1 when imports may be larger.

**Why pg-boss over Redis/BullMQ:** pg-boss runs on the same PostgreSQL instance already required for
catalog data. No additional Redis dependency. Uses `SKIP LOCKED` for exactly-once delivery. Job
records are durable — not lost on process restart. Aligns with the project's PostgreSQL-first
philosophy (outbox before Kafka/NATS).

#### Import Flow

```
Client                    API                       pg-boss         Worker
  │                        │                           │               │
  ├─ POST /import ─────────→│                           │               │
  │  (multipart: file)     │                           │               │
  │                        ├─ validate file mime/size  │               │
  │                        ├─ generate importJobId     │               │
  │                        ├─ store raw file in DB ────→│               │
  │                        ├─ enqueue job ─────────────→│               │
  │                        │                           ├─ notify ──────→│
  ←─ 202 { importJobId } ──┤                           │               │
  │                        │                           │  parse CSV/JSON│
  │                        │                           │  validate rows │
  │                        │                           │  upsert records│
  │                        │                           │  update status │
  ├─ GET /import/:id ───────→│                           │               │
  ←─ 200 { status, counts} ─┤                           │               │
```

#### Import Job Table

```sql
CREATE TABLE import_jobs (
  id              UUID PRIMARY KEY,
  org_id          UUID NOT NULL,
  status          TEXT NOT NULL DEFAULT 'queued',   -- queued|processing|done|failed
  entity_type     TEXT NOT NULL,                    -- 'agents'|'skills'|...
  format          TEXT NOT NULL,                    -- 'json'|'csv'
  total_rows      INT,
  success_rows    INT,
  failure_rows    INT,
  errors          JSONB,                            -- [{row, field, message}]
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at    TIMESTAMPTZ
);
```

#### Idempotency and Partial Failure

- **Idempotency key:** The `importJobId` is generated by the API on upload and returned to the
  caller. Re-uploading the same file produces a new job — by design. Callers who want idempotent
  re-runs supply an `importJobId` query param on POST; if a job with that ID exists and is done,
  the API returns the existing result without re-processing.

- **Partial failure semantics:** Each row is processed in its own Drizzle transaction. A failed row
  (validation error, duplicate key) is recorded in `errors` JSONB and counted in `failure_rows`.
  Processing continues for remaining rows. A job is marked `done` even when `failure_rows > 0` —
  the caller inspects `errors` to decide whether to fix and re-import.

- **Row upsert strategy:** Import uses `INSERT ... ON CONFLICT (org_id, external_id) DO UPDATE` so
  re-importing a corrected file is safe. `external_id` (e.g., employee ID in the source system) is
  stored on import-eligible entities as a nullable indexed column.

---

## 6. Build Order and Phase Boundaries

### Phase Dependencies

```
Phase 1: Foundation
  └─ Monorepo scaffold, CI, DB migrations (Drizzle + PostgreSQL), OrgScopedDb, RLS policies

Phase 2: Catalog CRUD
  └─ Depends on: Phase 1
  └─ 6 entity schemas, Hono routes, OrgScopedDb wrappers, service layer, Zod validation
  └─ Cannot start MFE until API shapes are defined (shared-types drives both)

Phase 3: Agent State Model
  └─ Depends on: Phase 2 (break_reasons catalog entity must exist for guard validation)
  └─ Transition table, AgentStatusRecord schema, state service, agent-state routes

Phase 4: Bulk Import
  └─ Depends on: Phase 2 (entities must exist to receive imported data)
  └─ Depends on: Phase 3 (agent import may include initial status)
  └─ pg-boss setup, import-service, worker, import-jobs table, POST/GET /import

Phase 5: Catalog UI (MFE)
  └─ Depends on: Phase 2 (needs real API endpoints to call)
  └─ Module Federation config, OrgContext, API client, entity CRUD components

Phase 6: Integration and Hardening
  └─ Depends on: All prior phases
  └─ End-to-end org isolation tests (Testcontainers), audit log validation,
     MFE embedding test in a stub host app, bulk import partial-failure tests
```

### Rationale for Ordering

- **Phase 1 before everything:** OrgScopedDb and RLS are foundational — every phase that touches
  data depends on them being correct. Getting isolation wrong early means rework in every entity.

- **Phase 2 before Phase 3:** `break_reasons` is a catalog entity. The state machine guard that
  validates a Break transition references it. Implementing the state machine before its catalog
  dependency exists means either mocking or incomplete guard logic.

- **Phase 2 before Phase 5:** `shared-types` package defines API request/response shapes. The MFE
  API client is typed against these. Build the API contract first, then build the UI against it.

- **Phase 4 after Phase 2-3:** Import writes to entity tables. Those tables must have their full
  schemas (including agent_states) before import workers are implemented.

- **Phase 5 last among core features:** The MFE is a consumer of the API, not a producer. It can
  be scaffolded in Phase 1 but substantive UI work starts only once API endpoints are stable.

- **Phase 6 is non-optional:** Org isolation correctness and the embed contract are load-bearing
  for v1. Hardening is a phase, not a last-minute activity.

---

## Data Flow

### Standard Catalog CRUD Request

```
HTTP Request (x-org-id: <uuid>)
  ↓
org-context middleware
  → validates header present
  → orgStore.run({ orgId }) wraps the rest of the chain
  → audit-log stamps request
  ↓
Hono route handler
  → Zod parse request body
  → call service function
  ↓
Service (packages/services/)
  → receives OrgScopedDb
  → business rule validation
  → call OrgScopedDb method
  ↓
OrgScopedDb (packages/db/)
  → reads orgId from AsyncLocalStorage
  → constructs Drizzle query with WHERE org_id = ?
  → (RLS also enforces at DB level)
  ↓
PostgreSQL
  ← returns rows filtered by org_id (double-filtered: app + RLS)
  ↓
Response (200 with entity or 404)
```

### Agent State Transition Request

```
PATCH /agents/:id/status { status: 'break', breakReasonId: '<uuid>' }
  ↓
org-context middleware (as above)
  ↓
agent-state route handler
  → parse body with Zod
  ↓
AgentStateService
  → load current status from agent_states via OrgScopedDb
  → validate breakReasonId exists in break_reasons (OrgScopedDb)
  → call StateMachine.transition(current, target, 'agent', context)
  → StateMachine.transition checks ALLOWED_TRANSITIONS matrix + guard
  → if allowed: persist new status to agent_states
  → if forbidden: throw InvalidTransitionError (→ 422)
  ↓
Response (200 { status, breakReasonId } or 422 { error })
```

---

## Scaling Considerations

| Scale | Architecture Adjustment |
|-------|------------------------|
| v0.1 single org pilot | Single Hono process, pg-boss worker in-process, no cache |
| v1 multi-org production | Extract runtime-engine to separate Hono app, add Redis for hot-path projection cache, pg-boss worker stays in-process or extracts |
| v1+ high routing load | Route decisions hit Redis projection cache (target: p95 < 50ms), PostgreSQL for catalog reads, outbox → NATS/Kafka for event fan-out |

The v0.1 structure does not require any of the v1 scaling changes — but it does not block them either. The control-plane/runtime split is enabled by the package boundaries above: `packages/domain/` and `packages/db/` have no Hono dependency and can be imported by a future `apps/runtime/` process without modification.

---

## Anti-Patterns

### Anti-Pattern 1: Raw DrizzleDb in service layer

**What people do:** Import and use `drizzle(pool)` directly in services, adding `where(eq(table.orgId, orgId))` manually each time.
**Why it's wrong:** Forgetting the clause on one query silently leaks data across orgs. Manual org_id threading is error-prone.
**Do this instead:** Services receive only `OrgScopedDb`. `DrizzleDb` is internal to `packages/db/`. The compiler enforces the boundary.

### Anti-Pattern 2: org_id in JWT payload only — no middleware extraction

**What people do:** Parse the JWT in each route handler to extract org_id.
**Why it's wrong:** Forgetting in one handler creates an unscoped route. Inconsistent extraction is audited inconsistently.
**Do this instead:** Middleware extracts org_id once, stores in AsyncLocalStorage, and every downstream consumer reads from the store. Missing extraction is a 400 at middleware, not a data leak.

### Anti-Pattern 3: XState for catalog agent state in v0.1

**What people do:** Add XState to model agent state machine because "it's the right tool".
**Why it's wrong:** XState requires in-process actor instances. Catalog-only v0.1 is stateless API — there is no long-lived agent session to attach an actor to. XState adds a dependency and serialization complexity without providing benefit until the runtime-engine exists.
**Do this instead:** Transition table in `packages/domain/` — replaces trivially with XState in v1 when actors live in the runtime-engine.

### Anti-Pattern 4: Synchronous bulk import in single HTTP handler

**What people do:** Parse the entire CSV, validate all rows, then INSERT in one transaction, returning 200 or 422.
**Why it's wrong:** Large files time out. A single validation failure in row 847 forces the user to fix and re-upload the entire file. Transactional rollback on partial failure discards valid rows.
**Do this instead:** Async pg-boss job with row-level partial failure semantics. Return 202 immediately, poll for status.

### Anti-Pattern 5: Flat enum for Engaged sub-states

**What people do:** Define `EngagedVoice | EngagedChat | EngagedEmail` as separate top-level status values.
**Why it's wrong:** Adding a channel in v1 (e.g., social) requires a new DB enum variant, migration, API contract change, and transition table update.
**Do this instead:** Composite model — `status: 'engaged'` + `engagedChannel: 'voice' | 'chat' | 'email'`. Adding a channel adds one string to the union; schema and transition rules unchanged.

---

## Integration Points

### External Boundaries

| Boundary | v0.1 Shape | v1 Upgrade Path |
|----------|-----------|-----------------|
| Host app → catalog-ui MFE | Module Federation remote, props: orgId + theme + modules | Add authToken prop, verify JWT in API middleware |
| catalog-ui → API | REST over HTTPS, x-org-id header (stub auth) | Bearer token, org_id from JWT claim |
| API → PostgreSQL | Drizzle + node-postgres, RLS session variable | Unchanged — add read replica for projection queries |
| API → pg-boss | In-process worker | Extract to separate worker process if import load warrants |
| API → runtime-engine (v1) | Not in v0.1 | gRPC internal API via protobuf contracts |

### Internal Package Boundaries

| Boundary | Communication | Rule |
|----------|---------------|------|
| `apps/api` ↔ `packages/services` | Direct import | services must not import from apps/ |
| `apps/catalog-ui` ↔ `packages/shared-types` | Direct import | UI consumes types only — no service logic |
| `packages/services` ↔ `packages/db` | Direct import | services depend on db, not reverse |
| `packages/services` ↔ `packages/domain` | Direct import | services use domain entities and state machine |
| `packages/db` ↔ `packages/domain` | Direct import | db schema types derive from domain entities |

---

## Sources

- [Module Federation Shared Dependencies — module-federation.io](https://module-federation.io/configure/shared)
- [Row-Level Security for Tenants in Postgres — Crunchy Data](https://www.crunchydata.com/blog/row-level-security-for-tenants-in-postgres)
- [Multi-tenant data isolation with PostgreSQL RLS — AWS](https://aws.amazon.com/blogs/database/multi-tenant-data-isolation-with-postgresql-row-level-security/)
- [Drizzle ORM tenantId enforcement discussion](https://github.com/drizzle-team/drizzle-orm/discussions/1539)
- [pg-boss PostgreSQL job queue — GitHub](https://github.com/timgit/pg-boss)
- [Fastify request context (AsyncLocalStorage)](https://github.com/fastify/fastify-request-context)
- [Hono Context Storage Middleware](https://hono.dev/docs/middleware/builtin/context-storage)
- [NestJS vs Fastify vs Hono — Encore](https://encore.dev/articles/nestjs-vs-fastify-vs-hono)
- [Agent States — Cisco DevNet Contact Center Express](https://developer.cisco.com/docs/contact-center-express/what-is-an-agent-state/)
- [XState v5 — statelyai/xstate](https://github.com/statelyai/xstate)
- [pnpm Workspaces](https://pnpm.io/workspaces)
- [Turborepo structuring a repository](https://turborepo.dev/docs/crafting-your-repository/structuring-a-repository)
- [Webpack Module Federation — webpack.js.org](https://webpack.js.org/concepts/module-federation/)

---
*Architecture research for: Open Routing v0.1 Catalog Foundation*
*Researched: 2026-05-15*
