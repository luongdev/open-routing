# Tasks: Define v0.2 Core Routing Runtime Foundation

## 1. Milestone Confirmation

- [ ] Confirm v0.2 scope with product owner: runtime foundation plus API-backed flow UI, not UI-only prototype.
- [ ] Resolve runtime process boundary: same Go API process first vs separate runtime service now.
- [ ] Resolve realtime scope as an explicit rescope from GSD notes: polling trace/read APIs vs SSE in v0.2.
- [ ] Resolve DSL depth: graph JSON import/export/review vs full DSL editor.
- [ ] Resolve mock adapter scope: adapter-command tests only vs production-shaped mock adapters.

## 2. Contracts and Data Model

- [ ] Add REST/OpenAPI paths and schemas for flow drafts, flow versions, publish, rollback, simulation, admin-facing route requests, reservations, and traces.
- [ ] Add migrations for flow definitions, flow versions, publish records, runtime events, reservations, route requests, and traces.
- [ ] Add sqlc queries and generated Go/TypeScript clients.
- [ ] Ensure every new repository query flows through `orgDB`/SQLChecker enforcement unless it has an explicit audited bypass.
- [ ] Add drift gates for OpenAPI, sqlc, and TypeScript client generation.
- [ ] Preserve org-scoped isolation on every new table and endpoint.

## 3. Flow Authoring and Publish

- [ ] Implement flow draft CRUD with code identity and optimistic versioning.
- [ ] Implement graph validation for shape, node links, graph export consistency, catalog references, and adapter registry references.
- [ ] Implement compile-to-plan output for the v0.2 node subset: `trigger`, `if_else`, `switch_case`, `wait`, `match_skill`, `filter`, `route_queue`, `reservation`, `fallback`, `effect`, `log`, and `end`.
- [ ] Implement publish with immutable version records.
- [ ] Implement rollback to a previous published version.
- [ ] Add tests for invalid graphs, stale writes, cross-org references, publish, and rollback.

## 4. Runtime Foundation

- [ ] Implement deterministic runtime execution against pinned flow version, catalog snapshot, interaction input, and state snapshot.
- [ ] Implement route request creation and execution result persistence.
- [ ] Implement reservation lifecycle states: offered, accepted, rejected, timeout, cancelled, completed.
- [ ] Implement retry as a runtime action that creates a new offered reservation attempt.
- [ ] Wire runtime-owned `Ready -> Engaged` and `Engaged -> WrapUp` transitions through the existing agent-state rules.
- [ ] Persist append-only runtime events and derived trace records.
- [ ] Add latency measurement and a report-only p95 route-decision budget.

## 5. Simulator and Trace Viewer

- [ ] Implement simulation endpoint using synthetic inputs without mutating live reservations or agent state.
- [ ] Persist or return deterministic simulation traces.
- [ ] Support record/mock behavior for effect nodes in simulation.
- [ ] Replace vNext trace viewer mock data with API-backed trace fixtures.
- [ ] Add tests proving replay stability for the same flow version, catalog snapshot, input, and initial state.

## 6. UI and Embed

- [ ] Convert `or-flow-list` from playground preview to API-backed shared UI component.
- [ ] Convert `or-flow-builder` to load, validate, simulate, and publish real flow drafts.
- [ ] Convert `or-trace-viewer` to load real runtime or simulation traces.
- [ ] Add flow routes to standalone admin.
- [ ] Decide whether flow routes are exposed through `<open-routing-catalog>` modules in v0.2 or remain admin-only until embed scope is confirmed.
- [ ] Keep playground entries as fixture-backed design references for regression screenshots.

## 7. CI and Closure

- [ ] Add backend runtime unit and integration test jobs.
- [ ] Add frontend flow UI typecheck, unit, and admin build gates.
- [ ] Add end-to-end smoke for publish, simulate, execute, reserve, and trace.
- [ ] Run cross-AI peer review on the full v0.2 implementation diff.
- [ ] Merge accepted behavior into `openspec/specs/` and archive this change when v0.2 ships.
