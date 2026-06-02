# Design: v0.2 Core Routing Runtime Foundation

## Approach

v0.2 should formalize the smallest end-to-end routing slice that proves Open Routing's core behavior without drifting into media, real adapters, or full auth. The slice is: create a flow draft, validate it, simulate it, publish it, execute the published version for a synthetic interaction, create a reservation, drive system-owned agent state transitions, and persist a trace that the UI can inspect.

The existing catalog, agent-state, OpenAPI, Go service, and shared Lit UI patterns remain the base. The vNext playground screens are visual references, not current behavior.

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
- Use deterministic execution for tests and simulator runs by pinning flow version, catalog snapshot, interaction input, and initial state snapshot.
- Keep playground vNext components as design references until they are backed by real contracts.
- Treat realtime push as an explicit rescope from the old GSD v0.2 notes unless live routing monitor behavior is confirmed in Wave 0.
- Keep capability validation out of v0.2 unless a capabilities entity is explicitly added.
- Use REST/OpenAPI for control-plane and admin-facing flow/runtime operations in v0.2; internal gRPC remains deferred until runtime/adapter boundaries need process separation.

## Gray Areas

These items should be discussed before implementation plans lock:

**Runtime process boundary**

Default assumption: implement runtime as a separate Go package inside `services/api` first, with interfaces that can move to a separate service. A separate `cmd/runtime` can be added once hot-path or deployment requirements demand it.

Alternative: create a separate runtime service immediately. That is cleaner architecturally but increases local dev, CI, migrations, and API surface complexity.

**Realtime delivery**

Default assumption: persist events and traces first; expose read APIs and use polling for UI. Add SSE only if v0.2 needs live routing monitor behavior.

Alternative: ship SSE with v0.2. This gives better runtime visibility but pulls in browser-worker and fan-out decisions earlier.

**DSL depth**

Default assumption: graph JSON is canonical; DSL is export/import/review only.

Alternative: build a real DSL editor. That is likely too broad for the first runtime milestone.

**Mock adapter scope**

Default assumption: runtime emits adapter commands and mock adapters acknowledge them in tests.

Alternative: build production-shaped voice/chat/email mock adapters with full conformance now. This may be better for v1, but it can dominate v0.2 if included too early.

**Auth and RBAC**

Default assumption: keep v0.1 stub org context for v0.2 runtime foundation, but never rely on it as a production security claim.

Alternative: do AUTH first. That is cleaner for external use but delays core routing proof.

## Validation Strategy

- Contract tests for flow APIs and generated clients.
- Domain tests for graph validation, compiler output, deterministic execution, and reservation transitions.
- Isolation tests proving runtime events, reservations, traces, and flow records are org-scoped.
- Agent-state tests for runtime-owned system transitions and WrapUp expiry.
- UI component tests replacing mock data with API-backed fixtures.
- End-to-end smoke: publish a flow, simulate it, execute a route request, create/resolve a reservation, inspect trace output.
