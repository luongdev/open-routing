# Open Routing

## What This Is

Open Routing is an embeddable, multi-channel routing platform for products that need ACD-like routing without owning media, CRM, ticketing, or channel systems. It provides flow-based routing, catalog-driven configuration, tenant-facing audit, and adapter contracts for bridging systems such as voice, chat, email, and later real channel providers.

The primary users are product teams embedding routing and configuration surfaces into their own applications. Platform and solution engineering teams can also run it directly to configure flows, catalogs, adapters, and routing behavior.

## Core Value

Product teams can define, simulate, debug, publish, and embed powerful routing flows quickly without Open Routing becoming a media platform, agent desktop, CRM, or ticketing system.

## Requirements

### Validated

(None yet - ship to validate)

### Active

- [ ] Provide a visual flow builder where the UI graph is the source of truth.
- [ ] Compile published flow graphs into cached executable plans with strict validation.
- [ ] Support an advanced DSL surface for import, export, review, and power-user editing.
- [ ] Execute full orchestration flows: conditions, scoring, priority, skill matching, queue selection, route decisions, adapter commands, wait/timeout, retry, fallback, and webhook/API enrichment.
- [ ] Support deterministic core execution for debug: same flow version, catalog snapshot, interaction input, and state snapshot produce the same trace.
- [ ] Record or mock effect-node outputs during debug and replay.
- [ ] Provide a simulator using real catalog data plus synthetic or replayed interaction/state/event input.
- [ ] Support routing for voice, chat, and email in v1 through production-shaped mock adapters.
- [ ] Provide an adapter SDK contract before real adapter implementations.
- [ ] Keep routing core focused on open routing: route decisions, queue logic, reservation lifecycle, state projections, and adapter commands.
- [ ] Keep media holding, call control, chat execution, email execution, and channel-specific session ownership outside core.
- [ ] Keep a normalized catalog for agents, skills, queues, channels, adapters, capabilities, routing strategies, and flow templates.
- [ ] Allow catalog data to be managed directly in Open Routing or synced/imported from external systems.
- [ ] Support route strategies selected by flow, with strategy-engine scoring and a future extension point for custom algorithms.
- [ ] Support flow-defined fallback policies for routing failures.
- [ ] Expose embedded configuration surfaces through both iframe and micro frontend integration.
- [ ] Provide UI surfaces for flow builder/debugger, catalog config, routing monitor, trace viewer, and adapter config.
- [ ] Support theme tokens and module hide/show controls for embedded configuration surfaces.
- [ ] Provide REST management APIs for UI, catalog, flow, publish, adapter config, and admin operations.
- [ ] Provide internal gRPC APIs for runtime, adapters, routing decisions, and state/event streaming.
- [ ] Use a canonical event envelope for runtime and adapter events.
- [ ] Store events append-only, maintain fast projections for routing reads, and use short, clear names for tables, APIs, and concepts.
- [ ] Use PostgreSQL outbox first, with a future bridge to Kafka or NATS.
- [ ] Enforce tenant isolation from the start with shared database tables carrying `tenant_id` and app-layer enforcement.
- [ ] Split control-plane and runtime-engine from the start without decomposing into many services.
- [ ] Meet route-decision latency target of p95 under 50 ms.
- [ ] Support stateless runtime scaling, horizontal scale, database/outbox foundation, Redis/cache for hot path, and no active-active multi-region requirement in v1.
- [ ] Provide tenant-facing audit across routing, flow publish, config changes, and runtime traces.
- [ ] Support flow publish governance: validate, simulate, publish, and rollback to the previous published version.
- [ ] Position the project as both a deployable routing platform and a modular SDK/module set.

### Out of Scope

- Real media/call control in v1 - Open Routing bridges to channel systems but does not hold calls, streams, chats, or email sessions itself.
- Agent desktop in v1 - agent work surfaces belong to host products or downstream systems.
- WFM, CRM, and ticketing - these are external systems integrated through adapters or host applications.
- Real production adapters in v1 - v1 proves the adapter contract with production-shaped mock implementations; real FreeSWITCH, LiveKit, and other bridges come in later milestones.
- Active-active multi-region in v1 - architecture should not block it, but it is not a v1 delivery target.
- Full standalone plus embedded SSO/auth/RBAC in v1 - auth/RBAC must be designed for both modes, but full standalone and embedded SSO can land in a later milestone.

## Context

Open Routing is inspired by ACD systems but should not become a full contact-center suite. It is the routing brain and configuration surface that can be embedded into other products. Channels include voice, chat, and email in v1, with social comments, feeds, and additional messaging channels later.

The flow system is the center of the product. The visual graph is canonical. Publishing compiles the graph into an executable plan and caches it for runtime. Validation must be strict because invalid graph/runtime mismatch would break routing. The DSL is required, but it is an advanced surface rather than the primary authoring path.

Debugging is not done against a live customer interaction. It uses a simulator with real catalog records plus synthetic or replayed interaction/state/event input. Effect nodes can be recorded or mocked so replay stays stable.

The core owns routing behavior, queue logic, priority handling through flows, state projections, reservations, and fallback orchestration. Adapters own channel execution and normalize state/capabilities into the Open Routing model.

The product should feel simple despite powerful behavior. The UI should reduce clicks through templates, presets, and focused configuration surfaces. Naming should stay short and concrete; avoid verbose table, API, and concept names.

## Constraints

- **Boundary**: Open Routing is routing-only, not media, CRM, ticketing, WFM, or agent desktop - prevents the product from drifting into a full contact-center suite.
- **Flow Model**: UI graph is canonical and compiles to cached executable plans - supports visual authoring while keeping runtime fast.
- **Validation**: Flow publish must strictly validate graph, DSL/export, runtime plan, catalog references, and adapter capability references - prevents broken published flows.
- **Debug**: Core execution must be deterministic under fixed flow, catalog, interaction, and state snapshots - makes trace replay and simulator debugging trustworthy.
- **Channels**: v1 covers voice, chat, and email through mock adapters - proves multi-channel shape without overbuilding real bridges too early.
- **Adapter Boundary**: Adapters normalize state/capabilities and execute channel-specific commands - keeps channel systems outside core while giving routing enough state to decide.
- **API**: REST for management, gRPC for internal runtime/adapter APIs, event envelope for events - separates UI/admin ergonomics from runtime contracts.
- **Storage**: Append-only events, fast projections, PostgreSQL outbox first - supports replay/debug now and event-bus bridge later.
- **Tenancy**: Shared database with `tenant_id` on all tenant-scoped tables and app-layer enforcement - tenant isolation is required from the start.
- **Runtime**: Split control-plane and runtime-engine from the start - keeps hot path isolated without creating too many services.
- **Performance**: Route decision p95 under 50 ms - routing must stay viable for ACD-style workloads.
- **Embedding**: Support iframe and micro frontend integration - host products need flexible embed options.
- **Audit**: Tenant-facing audit is required - product teams and tenants need visibility into routing decisions and config changes.
- **Naming**: Prefer short, concrete names - avoid long meaningless table, API, and domain names.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Product name is Open Routing | Clear positioning around routing rather than a full ACD/contact-center suite | - Pending |
| Primary user is product teams, with platform/solution engineers as secondary users | The product is meant to be embedded into other applications while remaining usable directly | - Pending |
| UI graph is the source of truth for flows | Visual builder is the main product surface; DSL is advanced | - Pending |
| Published flows compile into cached executable plans | Keeps runtime fast and separates authoring from execution | - Pending |
| Flow publish uses validate, simulate, publish, rollback | Prevents broken routing changes and gives a safe recovery path | - Pending |
| Flow engine owns full orchestration | Routing needs enrichment, scoring, adapter commands, wait/timeout, retry, and fallback, not just a single decision | - Pending |
| Core uses deterministic execution with record/mock for effects during debug | Makes simulation and replay reliable without touching real interactions | - Pending |
| Route strategy is chosen by flow and scored by a strategy engine | Keeps v1 simple while leaving room for custom algorithms later | - Pending |
| Core includes queue logic but adapters own holding/execution | Allows routing decisions without making core own media or channel sessions | - Pending |
| Routing output supports reservation lifecycle | ACD assignment is offer/accept/timeout/retry, not only a one-shot target | - Pending |
| Adapter SDK contract comes before real adapters | Proves integration boundaries before coupling to FreeSWITCH, LiveKit, or other providers | - Pending |
| v1 uses voice, chat, and email mock adapters | Demonstrates multi-channel routing without real bridge complexity | - Pending |
| Catalog is normalized inside Open Routing | Routing needs consistent agents, skills, queues, channels, adapters, capabilities, strategies, and templates | - Pending |
| Priority and SLA are determined by flow | Keeps prioritization flexible and explicit in customer logic | - Pending |
| REST management API plus internal gRPC runtime API | UI/admin operations and runtime/adapter operations have different needs | - Pending |
| PostgreSQL outbox first, Kafka/NATS bridge later | Keeps v1 operationally simpler while preserving event-driven growth path | - Pending |
| Shared database with `tenant_id` and app-layer enforcement | Tenant isolation is needed from day one without per-tenant database overhead | - Pending |
| Split control-plane and runtime-engine from the start | Protects routing hot path without decomposing into many services | - Pending |
| Route decision target is p95 under 50 ms | Performance is part of the product value, not a later optimization | - Pending |
| Embedded UI supports iframe and micro frontend | Host products need both isolation and native integration options | - Pending |
| v1 excludes media/call control, agent desktop, WFM, CRM, ticketing, and production adapters | Keeps the project inside open-routing boundaries | - Pending |
| Auth/RBAC supports standalone and embedded SSO later | Needed long term, but full implementation can wait until a later milestone | - Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `$gsd-transition`):
1. Requirements invalidated? -> Move to Out of Scope with reason
2. Requirements validated? -> Move to Validated with phase reference
3. New requirements emerged? -> Add to Active
4. Decisions to log? -> Add to Key Decisions
5. "What This Is" still accurate? -> Update if drifted

**After each milestone** (via `$gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check - still the right priority?
3. Audit Out of Scope - reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-05-15 after initialization*
