# Stack Research

**Domain:** Embeddable multi-channel routing platform (ACD-like), B2B, multi-org, control-plane + MFE
**Milestone scope:** v0.1 — Catalog Foundation only. No flows, no runtime, no adapter execution.
**Researched:** 2026-05-15
**Confidence:** HIGH (all versions verified via npm registry; architecture recommendations verified against official docs and current ecosystem state)

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Node.js | 22.x LTS | Runtime | Fastify v5 requires Node 20/22; 22.x is current LTS with longest support window — choose it over 20 now |
| TypeScript | 6.0.3 | Language | Latest stable; full ESM support; `strict` mode mandatory from day 1 for multi-org correctness |
| Fastify | 5.8.5 | REST management API server | ~70k req/s vs Express ~15k; native JSON schema validation; plugin encapsulation maps directly to the control-plane/runtime split; gRPC coexistence via ConnectRPC plugin is first-class |
| Drizzle ORM | 0.45.2 | PostgreSQL data access | SQL-first: schema is TypeScript, queries compile to near-raw SQL — zero magic between you and org_id WHERE clauses; no runtime query build surprises; migration output is readable SQL files you can audit |
| drizzle-kit | 0.31.10 | Schema migrations | Generates deterministic SQL migrations; stores applied state in `__drizzle_migrations`; works without a running server |
| postgres (pg driver) | 3.4.9 | PostgreSQL wire driver | The `postgres` package (not `pg`) — async iterator streaming, TypeScript-native, works with Drizzle ORM's postgres dialect; better ergonomics for bulk import streaming |
| React | 19.2.6 | MFE catalog UI | Latest stable; concurrent features; Server Components irrelevant here (pure client MFE) |
| Vite | 8.0.13 | MFE build tooling | Fastest cold start and HMR; `@module-federation/vite` 1.15.4 provides MF 2.x runtime for host embedding |
| Zod | 4.4.3 | Schema validation (API + import) | 14x faster than Zod 3; required for CSV/JSON import row validation and for request body contracts in Fastify route schemas |
| pnpm | 11.1.2 | Package manager + workspace engine | Content-addressable store; strict phantom-dep prevention; workspace protocol links packages without publishing |
| Turborepo | — (managed via pnpm) | Monorepo task orchestration | Fast cache-aware task runner; low config; right size for a 2-3 package split; NOT Nx (see Alternatives) |
| Vitest | 4.1.6 | Testing (all packages) | Same config as Vite; covers unit + integration; Fastify's `inject()` works inside Vitest without adapter glue |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@connectrpc/connect-fastify` | 2.1.1 | gRPC/Connect plugin for Fastify | v0.1: do NOT install yet. Wire in v1 when runtime-engine split needs internal gRPC. Fastify plugin exists; path is clear. |
| `@module-federation/vite` | 1.15.4 | Module Federation 2.x for Vite | v0.1: install in the MFE app package. Exposes the catalog UI as a federated remote that host apps load at runtime. |
| `@tanstack/react-query` | 5.100.10 | Server-state management in MFE | All catalog CRUD calls go through React Query; handles caching, background refresh, optimistic updates. |
| `@tanstack/react-table` | 8.21.3 | Data table in MFE | Catalog lists (agents, queues, skills etc) are tables with sort/filter/pagination. Headless — pairs with shadcn/ui. |
| `shadcn/ui` (components) | current | UI components for MFE | Unstyled-by-default Radix primitives + Tailwind; B2B product teams can override theme tokens. No version pin needed — components are copy-paste, not a dependency. |
| `tailwindcss` | 4.x | Styling | CSS-first v4; works with shadcn/ui; theme variables exposed as CSS custom properties for host-app token override. |
| `fast-csv` | 5.0.7 | CSV streaming parser (server) | Node.js streaming, TypeScript-native, handles malformed input gracefully. Use on import endpoint to pipe file stream → row iterator → Zod validation → DB insert. |
| `@opentelemetry/sdk-node` | 0.218.0 | OTel SDK for API server | Vendor-neutral traces + metrics; auto-instruments Fastify and pg driver. Required from v0.1 — retrofit is painful. |
| `@opentelemetry/auto-instrumentations-node` | 0.76.0 | Auto-instrument common libs | Captures HTTP, pg, dns spans with zero per-call code changes. |
| `pino` | bundled in Fastify | Structured logging | Fastify ships pino; use its default JSON logger — do not add Winston or any other logger. Pino serializers handle org_id context injection. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `tsx` | TypeScript execution for scripts/migrations | `npm view tsx version` → 4.22.0; replaces ts-node; no separate compile step for seed scripts and drizzle-kit |
| `eslint` (v9 flat config) | Lint backend + frontend | Use `@typescript-eslint` ruleset; enforce `no-unsafe-*` rules; flat config works across monorepo packages from repo root |
| `prettier` | Formatting | Single `.prettierrc` at repo root; pnpm workspace script runs it across all packages |
| `docker compose` | Local PostgreSQL | Spin up versioned Postgres 16 with a `open_routing_dev` database; no cloud dependency in local dev |
| GitHub Actions | CI | Single workflow; pnpm cache; Turborepo remote cache optional (add later); matrix: lint → test → build |

---

## Monorepo Structure

Use **pnpm workspaces + Turborepo**. Three top-level packages for v0.1; structure anticipates the v1 split without creating it yet.

```
open-routing/
├── pnpm-workspace.yaml
├── turbo.json
├── package.json          (root — workspace scripts, shared eslint/prettier)
├── apps/
│   └── catalog-ui/       (React 19 + Vite + Module Federation — the MFE)
├── packages/
│   ├── api/              (Fastify REST management API — control-plane)
│   └── db/               (Drizzle schema + migrations + seed scripts — shared by api, future runtime-engine)
```

**Rationale for this split:**
- `packages/db` is the key separation: when v1 adds the runtime-engine, it imports `@open-routing/db` for schema types and the Drizzle client instance. The runtime-engine never re-declares schema.
- `packages/api` maps to the control-plane. The future runtime-engine goes in `packages/engine` (not built in v0.1 but the slot is obvious).
- `apps/catalog-ui` is the embeddable MFE. It does NOT import from `packages/db` directly — all data access goes through the REST API. This keeps the MFE deployable independently.
- `turbo.json` defines the task pipeline (`build` depends on `^build`; `test` depends on `db#generate`).

**Do NOT start with a polyrepo.** The catalog schema, API types, and MFE shared constants will cross-reference from day one. A polyrepo forces npm publish cycles during early development when schema is still changing.

---

## Database Layer: Multi-Org Pattern

### Schema enforcement pattern: App-layer (Drizzle) + database constraint — NOT RLS

**Decision: app-layer enforcement via Drizzle helper, NOT PostgreSQL RLS.**

Reasoning specific to this domain:

1. **Connection pooling incompatibility.** RLS with `SET LOCAL rls.org_id` requires session-mode PgBouncer (not transaction-mode). Transaction-mode pooling — which is standard for ACD-scale connection counts — resets session variables between requests. This makes RLS unreliable unless you control pooler config, which embedded deployments (product teams deploying Open Routing) may not.

2. **gRPC runtime path.** When the runtime-engine is added in v1, it runs internal gRPC calls that bypass the Fastify middleware where you'd set the RLS session variable. App-layer enforcement in a Drizzle client helper is explicit and survives the control-plane/runtime split.

3. **Debugging.** RLS policy failures in PgBouncer environments are silent — the query returns empty rows, not an error. In an ACD-like system this would be catastrophic (routing decisions silently skipping agents from another org). App-layer explicit WHERE is loud and traceable.

**Pattern:**

```typescript
// packages/db/src/client.ts
export function orgDb(orgId: string) {
  // Returns a Drizzle client scoped to orgId.
  // All query helpers here append .where(eq(table.orgId, orgId)).
  // No raw .query.table.findMany() without orgId is exposed from this module.
}
```

Every table that is org-scoped carries `org_id uuid NOT NULL REFERENCES orgs(id)` as a column. A check constraint `CHECK (org_id IS NOT NULL)` prevents accidental inserts. Drizzle schema enforces the column presence at compile time.

**Add a database-level mitigation:** Create a Postgres role `api_user` that has no direct table grants — only grants through a schema. This reduces the blast radius of a missed WHERE but does not replace the app-layer pattern.

### Migrations

Use `drizzle-kit generate` + `drizzle-kit migrate` (applied on startup via a startup script, not at runtime per-request). Migration files are committed to `packages/db/migrations/`. Never auto-apply in test; tests run against a fresh schema via `drizzle-kit push` to an isolated test database.

---

## MFE Embed Strategy

### Recommendation: Module Federation 2.x (`@module-federation/vite`)

**Not single-spa. Not Web Components.**

Why Module Federation over single-spa for this domain:
- Single-spa requires a root config wrapper in the host app. Product teams embedding Open Routing cannot restructure their app router to accommodate it. MF exposes a named remote (`catalog-ui`) that the host loads with a single `<script>` tag and a dynamic `import()`.
- Web Components are framework-agnostic but add Shadow DOM friction: host-app theme tokens don't pierce the shadow boundary without CSS part exposure. The catalog UI needs to accept theme tokens from the host (the requirement says "theme tokens and module hide/show controls"). MF + CSS custom properties is the cleaner path.
- MF 2.x stable (April 2026) supports Vite natively via `@module-federation/vite`. Dynamic TypeScript type generation means the host app gets type hints for what the remote exposes.

**What the MFE exposes:**
```typescript
// catalog-ui/src/federation.ts (exposed as MF remote entry)
export { CatalogShell } from './CatalogShell'
// CatalogShell accepts: orgId, apiBaseUrl, theme tokens
```

**What host apps do:**
```html
<!-- In host app's HTML -->
<script src="https://your-cdn.example.com/catalog-ui/remoteEntry.js"></script>
```
```typescript
const { CatalogShell } = await import('catalog-ui/CatalogShell')
```

**v0.1 scope:** Ship a Vite dev server with MF remote configured. Document the host integration pattern. The production CDN deployment is a v1 concern.

---

## CSV/JSON Bulk Import Pattern

### Endpoint design

`POST /v1/orgs/{orgId}/catalog/import`
- `Content-Type: multipart/form-data` with a file field (CSV or JSON auto-detected)
- Returns `202 Accepted` + `importId` immediately
- Result polled at `GET /v1/orgs/{orgId}/catalog/imports/{importId}`

**Why async:** CSV files from external ACD dumps can be 50k+ rows. Synchronous import blocks HTTP workers and gives no progress visibility.

### Implementation pattern

```
Stream (fast-csv or JSON.parse stream)
  → Batch collector (100 rows)
  → Zod schema validation per row
  → Drizzle upsert (ON CONFLICT DO UPDATE — idempotency via external_id column)
  → ImportResult accumulator (succeeded[], failed[{row, error}])
```

**Idempotency:** Every importable entity carries an optional `external_id` column (varchar, nullable). Import uses `INSERT ... ON CONFLICT (org_id, external_id) DO UPDATE` — re-running the same CSV file is safe.

**Partial failure:** Import continues on row errors. Failed rows are accumulated in the ImportResult record. The import does NOT abort on first error (it is NOT an all-or-nothing transaction). This is the correct pattern for ACD catalog seeding where "import 9,900 of 10,000 agents" is acceptable.

**Validation library:** Zod 4 with per-entity schemas (e.g., `AgentImportRowSchema`). Zod 4's 14x parse speed improvement makes per-row validation cost negligible.

---

## Testing / CI / Observability Scaffolding

### Testing layers

| Layer | Tool | Scope |
|-------|------|-------|
| Unit | Vitest 4.x | Domain logic, Zod schemas, import parsers |
| Integration (API) | Vitest + Fastify `inject()` | Route handlers with real DB (test-scoped schema) |
| Integration (DB) | Vitest + Drizzle against Postgres | Migration smoke test, org_id constraint enforcement |
| E2E (deferred) | Playwright — not in v0.1 | MFE flows, deferred to v0.2 |

**Test DB strategy:** In CI, spin up Postgres via `services:` in GitHub Actions workflow. Use `drizzle-kit push` (not `migrate`) against the test DB to apply schema without migration history. Each test suite creates org fixtures; `afterEach` deletes by `org_id` (not truncate — respects FK cascade and is faster than full table truncate in parallel test runs).

### CI pipeline (GitHub Actions)

```yaml
jobs:
  ci:
    steps:
      - pnpm install --frozen-lockfile
      - turbo run lint
      - turbo run typecheck
      - turbo run test          # includes API integration tests
      - turbo run build
```

Turborepo caches `typecheck` and `build` between runs (remote cache optional). Test jobs do NOT cache — always fresh against real DB.

### Observability

Initialize OpenTelemetry in `packages/api/src/telemetry.ts` — loaded before any Fastify plugin registration. This is the "instrument first" requirement: retrofitting OTel after plugins are registered misses auto-instrumented spans.

```typescript
// telemetry.ts — loaded via --import flag in Node startup
import { NodeSDK } from '@opentelemetry/sdk-node'
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node'

const sdk = new NodeSDK({ instrumentations: [getNodeAutoInstrumentations()] })
sdk.start()
```

**Metrics exports:** In v0.1, use console/OTLP exporter to a local collector (docker compose service). In v1, swap exporter config via env var — no code change needed.

**Structured logging:** Fastify's pino logger with `serializers` that inject `org_id` from `request.orgId` on every log line. Never log without org context on a multi-org system.

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Backend framework | Fastify 5 | NestJS | NestJS adds class decorators, DI container, module system — appropriate for large teams with explicit layering rules; overkill for a 2-3 package monorepo starting with catalog CRUD. Fastify's plugin system provides equivalent encapsulation without reflection metadata. |
| Backend framework | Fastify 5 | Express 5 | Express 5 stable released 2024; still callback-model without native schema validation; requires body-parser, express-validator, etc. separately. Fastify includes all of this and is 4-5x faster. |
| ORM | Drizzle | Prisma | Prisma's query engine is a separate Rust binary — adds 50-100ms cold start latency and complicates Docker images. Prisma's generated client is opaque; debugging a missed `org_id` scope in generated queries is harder than in Drizzle's explicit query builder. |
| ORM | Drizzle | TypeORM | TypeORM is in maintenance mode; decorator-based schema is TypeScript-unfriendly in strict mode; active-record pattern fights the hexagonal architecture that routing platforms need. |
| MFE strategy | Module Federation 2.x | single-spa | single-spa requires the host to adopt a routing adapter; product teams cannot restructure their apps for one embedded widget. MF is purely additive — add a script tag, import the remote. |
| MFE strategy | Module Federation 2.x | Web Components | Web Components + Shadow DOM blocks CSS custom property inheritance for theme tokens; no TypeScript contract between host and component props. MF exposes typed React components. |
| Monorepo tooling | pnpm + Turborepo | Nx | Nx's power (affected computation, generators, distributed CI) is valuable at 20+ packages. For 3 packages, the configuration overhead and opinionated project structure slows initial velocity. Turborepo's `turbo.json` is 20 lines. Migrate to Nx if the package count crosses 10. |
| Monorepo tooling | pnpm + Turborepo | npm/yarn workspaces alone | No task caching; no dependency-aware task ordering; impossible to parallelise lint/test/build correctly without a task runner. |
| Validation | Zod 4 | Joi | Joi has no TypeScript inference — schema and TypeScript type must be maintained separately. Zod derives the type. In a multi-entity catalog with 6 entity schemas, the maintenance cost is significant. |
| Validation | Zod 4 | Yup | Same problem as Joi; weaker TypeScript inference; slower than Zod 4. |
| CSV parsing | fast-csv | papaparse | papaparse is browser-primary; its Node.js streaming support is secondary. fast-csv is purpose-built for Node.js streams, TypeScript-native, and integrates cleanly with async iterators used in the import pipeline. |
| gRPC (v1) | ConnectRPC + connect-fastify | @grpc/grpc-js | ConnectRPC serves gRPC, gRPC-Web, and Connect protocol from the same Fastify server — no proxy required. Browser clients (if needed) and Node clients share the same generated TypeScript client. `@grpc/grpc-js` is lower-level and requires a separate gRPC port + proxy layer. |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Prisma (in v0.1+) | Rust query engine binary adds cold-start latency and Docker image complexity; opaque generated client obscures `org_id` scoping bugs | Drizzle ORM — SQL-first, no binary |
| TypeORM | Maintenance mode; active-record pattern fights explicit org scoping; `@Entity` decorators don't survive `experimentalDecorators: false` in TS 5+ | Drizzle ORM |
| PostgreSQL RLS as primary isolation | RLS + PgBouncer transaction mode (standard for ACD scale) = silent empty rows instead of errors; gRPC runtime path bypasses Fastify middleware where RLS context would be set | App-layer `orgDb(orgId)` helper wrapping all Drizzle queries |
| NestJS for v0.1 | Reflection metadata, DI container, and module system add config surface without adding value for a 3-package monorepo in early development | Fastify with encapsulated plugins |
| single-spa | Requires host app to adopt single-spa routing adapter; not compatible with "drop-in embed" requirement for product teams | Module Federation 2.x |
| Winston logger | Slower than pino; not JSON-structured by default; Fastify ships pino — use it | pino (via Fastify's built-in logger) |
| Express | 4-5x slower than Fastify; no native JSON schema validation; requires many separate middleware packages | Fastify 5 |
| Lerna | Deprecated/maintenance mode for release orchestration; use Changesets if/when release coordination is needed | pnpm workspaces + Changesets (add Changesets when first public package ships) |
| Global `SET rls.org_id` without `SET LOCAL` | `SET` persists across connection pool sessions; leaks org context to next request | Never use `SET`; use `SET LOCAL` inside a transaction — and even then, prefer app-layer enforcement |

---

## v0.1 vs v1 Boundary

| Concern | v0.1 (do now) | v1 (defer — don't block) |
|---------|---------------|--------------------------|
| REST API | Full CRUD for 6 entities + import | — |
| MFE | Catalog config UI as MF remote | iframe embed variant |
| DB schema | All 6 catalog entities + agent_status | Outbox table, events table |
| Auth | Stub: trusted `X-Org-Id` header | Real auth/RBAC, SSO |
| gRPC | Not installed; path clear via connect-fastify | Internal runtime API |
| Runtime engine | Not present | `packages/engine` workspace package |
| Observability | OTel SDK + pino initialized | Proper collector + Grafana stack |
| Testing | Unit + API integration | E2E (Playwright) |
| ConnectRPC | Not installed | Install `@connectrpc/connect-fastify` |
| Redis | Not needed | Hot-path cache for routing decisions |

---

## Version Compatibility Summary

| Package | Version | Compatible With |
|---------|---------|-----------------|
| fastify | 5.8.5 | Node.js 20, 22 |
| drizzle-orm | 0.45.2 | drizzle-kit 0.31.10; postgres 3.x |
| drizzle-kit | 0.31.10 | drizzle-orm 0.45.x |
| postgres (driver) | 3.4.9 | Node.js 18+; works with drizzle-orm postgres dialect |
| zod | 4.4.3 | TypeScript 5.x; note: Zod 4 has breaking API changes from Zod 3 — do not mix |
| react | 19.2.6 | vite 8.x; `@module-federation/vite` 1.15.4 |
| @module-federation/vite | 1.15.4 | vite 8.x; MF 2.x runtime; not compatible with MF 1.x (webpack 5 native) |
| @tanstack/react-query | 5.100.10 | React 18+, React 19 — v5 is a full rewrite from v4; do not use v4 |
| vitest | 4.1.6 | vite 8.x (must match major); works with Fastify inject() |
| @connectrpc/connect-fastify | 2.1.1 | fastify 5.x; Protobuf ES 2.x |
| @opentelemetry/sdk-node | 0.218.0 | Node.js 18+; alpha stability warning: OTel JS is pre-1.0 for metrics/logs but traces are stable |
| turborepo | latest (pnpm dlx) | pnpm 9+; no install required — use `pnpm dlx turbo` or add as devDep |

---

## Integration Points (API ↔ DB ↔ MFE)

```
catalog-ui (Vite + MF remote)
  ↓ REST fetch to apiBaseUrl
packages/api (Fastify)
  ↓ X-Org-Id header → request.orgId
  ↓ orgDb(request.orgId) → scoped Drizzle client
packages/db (Drizzle schema + migrations)
  ↓ postgres driver
PostgreSQL 16 (shared DB, org_id on all org-scoped tables)
```

The `apiBaseUrl` is injected into the MFE `CatalogShell` component as a prop — the MFE does not hard-code the API location. This allows product teams to deploy Open Routing behind their own reverse proxy.

---

## Sources

- Fastify v5 LTS docs — https://fastify.dev/docs/v5.1.x/Reference/LTS/ (Node.js 20/22 support confirmed)
- Fastify testing guide v5 — https://fastify.dev/docs/v5.1.x/Guides/Testing/ (inject() pattern confirmed)
- npm registry (verified via `npm view` 2026-05-15): fastify@5.8.5, drizzle-orm@0.45.2, drizzle-kit@0.31.10, zod@4.4.3, react@19.2.6, vite@8.0.13, typescript@6.0.3, vitest@4.1.6, postgres@3.4.9, fast-csv@5.0.7, @tanstack/react-query@5.100.10, @tanstack/react-table@8.21.3, @module-federation/vite@1.15.4, @connectrpc/connect-fastify@2.1.1, @opentelemetry/sdk-node@0.218.0, pnpm@11.1.2, tsx@4.22.0
- Drizzle ORM latest releases — https://orm.drizzle.team/docs/latest-releases (MEDIUM — page showed older stable; npm registry confirmed 0.45.2 as current)
- Module Federation 2.0 stable release — https://www.infoq.com/news/2026/04/module-federation-2-stable/ (April 2026; Vite, Rspack, webpack support confirmed)
- Zod v4 release — https://zod.dev/v4 and InfoQ coverage (August 2025 stable; 4.4.3 on npm)
- PostgreSQL RLS vs app-layer — AWS blog, permit.io, simplyblock.io (connection pooling pitfall confirmed across multiple sources)
- Turborepo vs Nx comparison — daily.dev, DEV Community (March 2026; Turborepo recommended for <10 package repos)
- ConnectRPC Fastify plugin — https://connectrpc.com/docs/node/server-plugins/ (Fastify plugin confirmed; @connectrpc/connect-fastify@2.1.1)

---

*Stack research for: Open Routing v0.1 Catalog Foundation*
*Researched: 2026-05-15*
