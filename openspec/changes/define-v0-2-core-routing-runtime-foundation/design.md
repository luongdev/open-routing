# Design: v0.2 Core Routing Runtime Foundation

## Approach

v0.2 should formalize the smallest end-to-end routing slice that proves Open Routing's core behavior without drifting into media, real adapters, or full auth. The slice is: create a flow draft, validate it, simulate it, publish it, execute the published version for a synthetic interaction, create a reservation, drive system-owned agent state transitions, and persist a trace that the UI can inspect.

The existing catalog, agent-state, OpenAPI, Go service, and shared Lit UI patterns remain the base. The vNext playground screens are visual references, not current behavior.

OpenSpec is the planning contract, not the quality gate by itself. `acceptance.md` defines the user-owned decisions, business gates, output gates, UI gates, and review gates that must be satisfied before v0.2 can archive.

## Existing Inputs

- `.planning/PROJECT.md`: product boundary, flow-centered future, runtime split, publish governance, deterministic debug.
- `.planning/REQUIREMENTS.md`: FLOW, RT, ADP, SIM, AUTH, RT-PUSH, and AUD future requirement groups.
- `openspec/specs/agent-state/spec.md`: current state model and WrapUp expiry behavior.
- `openspec/specs/catalog-management/spec.md`: catalog identity and cache behavior.
- `web/packages/ui/src/components/vnext/`: mock flow list, flow builder, and trace viewer.

## Milestone Shape

v0.2 should be a foundation milestone, not full v1 runtime. It should introduce enough persistent data, API contract, runtime logic, and UI wiring to make flow execution observable and testable.

Recommended scope:

- Flow drafts and published versions.
- Graph validation with catalog reference checks.
- Simulation against synthetic interaction input and fixed catalog/state snapshots.
- Runtime execution of the v0.2 node subset: `trigger`, `if_else`, `switch_case`, `wait`, `match_skill`, `filter`, `route_queue`, `reservation`, `fallback`, `effect`, `log`, and `end`.
- Reservation lifecycle states: `offered`, `accepted`, `rejected`, `timeout`, `cancelled`, and `completed`. Retry is a runtime action that creates a new offered reservation attempt, not a persisted reservation state.
- Runtime-owned `Ready -> Engaged` and `Engaged -> WrapUp`.
- Trace records with step input, output, timing, selected catalog refs, and effect status.
- API-backed flow list, flow builder, simulator mode, and trace viewer.

## Key Decisions

- Keep graph as the canonical authoring model. DSL can be import/export/review unless explicitly expanded.
- Keep media and channel execution outside core; runtime outputs adapter commands and reservations only.
- Use mock or in-memory adapter behavior only to prove the adapter boundary.
- Keep org scoping explicit through existing org context patterns.
- Persist runtime events append-only; v0.2 can use direct trace/read tables before adding broader derived projections.
- Use deterministic execution for tests and simulator runs by pinning flow version, catalog snapshot, interaction input, initial state snapshot, virtual clock, reservation outcome signals, and effect outputs.
- Keep playground vNext components as design references until they are backed by real contracts.
- Treat realtime push as an explicit rescope from the old GSD v0.2 notes unless live routing monitor behavior is confirmed in Wave 0.
- Keep capability validation out of v0.2 unless a capabilities entity is explicitly added.
- Treat adapter references as references to the existing v0.1 adapter catalog rows unless Wave 0 explicitly adds a separate runtime registry.
- Treat v0.2 runtime events as debug/trace infrastructure, not the org-facing audit retention contract.
- Use REST/OpenAPI for control-plane and admin-facing flow/runtime operations in v0.2. Run the runtime as a separate `cmd/runtime` process that shares the `internal/runtime` library and the database with `cmd/api`; inter-plane gRPC is deferred until a synchronous low-latency hot path needs it.
- Bind published flows to route entry points by org, channel, and entry code so route requests resolve exactly one active published version.
- The data model must enforce at most one active published binding per org, channel, and entry code.
- Successful publish and rollback must transactionally leave one active published binding for their target org, channel, and entry code.
- Pin the published flow version at route-request start. Rollback affects only new route requests; in-flight executions finish on their pinned version.
- Use durable DB-backed continuations for live `wait` nodes and reservation timeouts. Continuation records carry the pinned flow version and execution cursor. The virtual clock is only for simulation and replay.
- Reservation-timeout continuations and server-owned WrapUp expiry should share compatible claiming semantics so timer-driven handlers cannot double-fire conflicting agent-state changes.
- Enforce reservation integrity with database-backed constraints and transactional state updates: one active offered/accepted reservation per agent, and one accepted reservation per route request.
- Guard reservation outcome transitions with current-state checks so accept, reject, timeout, and cancellation races cannot double-resolve the same offer.
- Store the minimal catalog/state read set required for replay instead of dumping the full catalog into every trace.
- Store continuation fire times, virtual-clock advances, reservation outcome signals, and candidate tie-break inputs as part of replay inputs.
- Keep `effect` nodes in record/mock mode for v0.2 unless Wave 0 explicitly scopes live external side effects.
- Control-plane (`cmd/api`) owns CRUD, publish/rollback, and simulation; simulation runs in-process against the shared `internal/runtime` library so it needs no inter-process call. Live route execution and all time-driven work run in `cmd/runtime`.
- Consolidate time-driven server work (live `wait`, reservation timeouts, and v0.1 WrapUp expiry) in the runtime worker, claiming due rows with `FOR UPDATE SKIP LOCKED`. This replaces the current single-replica `time.Ticker` sweeper pattern (`internal/state/ttl.go`, `internal/imports/sweep.go`) and lets multiple runtime replicas run safely.
- v0.2 candidate selection is skill-based: filter to agents that satisfy the flow's required skills, rank by proficiency, tie-break by longest-available, then a stable final tie-break key for determinism. A weighted scoring/strategy engine is deferred.
- Reservation integrity is DB-only for v0.2: a partial unique index plus guarded single-row CAS updates are both the source of truth and the cross-replica lock. No Redis lock — an offer is held for several seconds, so a TTL lock would expire mid-offer (double-book) or ghost-lock on crash, and it cannot be atomic with the reservation state in Postgres. A Redis fast-path lease may be added later only above the DB guard, never as the source of truth.
- Each executable node is a self-contained unit implementing a `Validate`/`Compile`/`Execute` contract plus a UI descriptor, registered in a node registry that validation, compilation, runtime, and the UI palette iterate generically. Adding a node is one file plus one registration, not edits across every surface.
- The `route_request` record is the interaction spine for v0.2: reservations, runtime-owned agent-state transitions, runtime events, and traces all reference it. With no real adapters yet, accept/complete signals come from the test-double adapter or simulator.
- The flow builder is embeddable as a Web Component from v0.2 because Shadow-DOM constraints (event retargeting, measurement, drag/connect behavior on the graph canvas) are load-bearing architecture, not polish that can be retrofitted later.

## Resolved Trade-offs (Wave 0, locked 2026-06-02)

These were the major gray areas; `acceptance.md` holds the canonical decision table. Resolutions:

**Runtime process boundary** — Decided: separate `cmd/runtime` from day one, because the config plane (admin CRUD, low QPS, human-driven) and the runtime plane (route decisions, p95<50 ms, machine-driven) have different scaling, deploy, and failure profiles, matching the project's "split control-plane and runtime-engine from the start" principle. Cost is kept low by sharing one Go module and the `internal/runtime` library and deferring gRPC; the two planes share the database for v0.2.

**Realtime delivery** — Decided: persist events and traces, expose read APIs, and poll. SSE/workers deferred to v0.3+.

**DSL depth** — Decided: graph JSON is canonical; DSL is export/import/review only.

**Mock adapter scope** — Decided: runtime emits adapter commands; test doubles acknowledge them. No production-shaped adapters in v0.2.

**Auth and RBAC** — Decided: keep the v0.1 stub org context. v0.2 is internal/trusted-host only and is never a production security claim.

**Live delayed continuations** — Decided: durable due-at continuation rows, claimed transactionally with `FOR UPDATE SKIP LOCKED` by the runtime worker. No in-memory-only timers; no external job queue.

**Reservation locking** — Decided: DB-only guard (Option A) — partial unique constraints plus guarded single-row CAS transitions are the source of truth and the cross-replica lock. A Redis lock is explicitly rejected as the source of truth: an offer is held for several seconds, so a TTL lock would either expire mid-offer (double-book) or ghost-lock on crash, and it cannot be made atomic with the reservation state in Postgres. A Redis fast-path lease may be added later above the DB guard if measured contention justifies it.

**Candidate selection** — Decided: skill-based eligibility, proficiency rank, longest-available tie-break, stable final tie-break key. Weighted scoring engine deferred.

**Node extensibility** — Decided: a node registry plus per-node `Validate`/`Compile`/`Execute` contract and UI descriptor, so the narrow v0.2 subset can grow by O(1) per node during iteration.

## Validation Strategy

- Contract tests for flow APIs and generated clients.
- Domain tests for graph validation, compiler output, deterministic execution, and reservation transitions.
- Domain tests for typed routing failures, including missing published flow, unavailable catalog reference, and reservation transition conflict.
- Isolation tests proving runtime events, reservations, traces, and flow records are org-scoped.
- Agent-state tests for runtime-owned system transitions and WrapUp expiry.
- UI component tests replacing mock data with API-backed fixtures.
- End-to-end smoke: publish a flow, simulate it, execute a route request, create/resolve a reservation, inspect trace output.
