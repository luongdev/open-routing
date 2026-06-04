# Tasks: Define v0.2 Core Routing Runtime Foundation

## 1. Milestone Confirmation

Wave 0 decisions are locked in `acceptance.md` (2026-06-02). Remaining Wave 0 work:

- [x] Confirm all user-owned decision gates in `acceptance.md`.
- [ ] Disambiguate draft optimistic-lock revision from immutable published-version naming.
- [ ] Sketch REST/OpenAPI response shapes for flows, publish, simulation, route requests, reservations, and traces.
- [ ] Re-run Claude and agy/Gemini review on the locked Wave 0 decisions — notably the changed runtime-boundary split (`cmd/runtime`) and the DB-only reservation guard — before implementation.

## 2. Contracts and Data Model

- [ ] Add REST/OpenAPI paths and schemas for flow drafts, flow versions, publish, rollback, simulation, admin-facing route requests, reservations, and traces.
- [ ] Add migrations for flow definitions, flow versions, publish records, flow entry bindings, runtime events, durable continuations, reservations, route requests, and traces.
- [ ] Add a database-backed invariant for at most one active published flow binding per org, channel, and entry code.
- [ ] Define typed routing-failure taxonomy for missing published flow, unavailable catalog reference, no eligible candidate, binding invariant violation, invalid graph, stale draft version, and reservation transition conflict.
- [ ] Add sqlc queries and generated Go/TypeScript clients.
- [ ] Ensure every new repository query flows through `orgDB`/SQLChecker enforcement unless it has an explicit audited bypass.
- [ ] Add drift gates for OpenAPI, sqlc, and TypeScript client generation.
- [ ] Preserve org-scoped isolation on every new table and endpoint.

## 3. Flow Authoring and Publish

- [ ] Implement flow draft CRUD with code identity and optimistic versioning.
- [ ] Implement graph validation for shape, node links, graph export consistency, catalog references, and adapter references.
- [ ] Implement compile-to-plan output for the v0.2 node subset: `trigger`, `if_else`, `switch_case`, `wait`, `match_skill`, `filter`, `route_queue`, `reservation`, `fallback`, `effect`, `log`, and `end`.
- [ ] Add compiled-plan format versioning or an equivalent compatibility contract for already-published plans.
- [ ] Define per-node execution contracts for inputs, outputs, branch semantics, and validation errors.
- [ ] Build a node registry: each node provides `Validate`/`Compile`/`Execute` plus a UI descriptor, and validation, compilation, runtime, and the UI palette iterate the registry generically so adding a node is one file plus one registration.
- [ ] Implement flow entry binding so a route request resolves exactly one active published version.
- [ ] Implement publish/rollback activation so route requests cannot observe multiple active versions for one binding.
- [ ] Implement publish with immutable version records.
- [ ] Implement rollback to a previous published version.
- [ ] Add tests for invalid graphs, stale writes, cross-org references, publish, and rollback.

## 4. Runtime Foundation

- [ ] Scaffold the `cmd/runtime` process wrapping the shared `internal/runtime` library; keep simulation runnable in-process from `cmd/api`.
- [ ] Implement deterministic runtime execution against pinned flow version, catalog snapshot, interaction input, and state snapshot.
- [ ] Implement skill-based candidate selection (eligibility by required skills, proficiency rank, longest-available tie-break) with a stable final tie-break key for determinism.
- [ ] Add a virtual clock or injectable clock source for deterministic `wait` and reservation-timeout replay.
- [ ] Implement durable continuation storage and a `cmd/runtime` worker claiming due rows via `FOR UPDATE SKIP LOCKED` for live `wait` and reservation-timeout resume; migrate v0.1 WrapUp expiry onto the same claim pattern so multiple runtime replicas stay safe.
- [ ] Ensure continuation records store pinned flow version, execution cursor, and due-time metadata.
- [ ] Implement route request creation and execution result persistence.
- [ ] Implement reservation lifecycle states: offered, accepted, rejected, timeout, cancelled, completed.
- [ ] Implement sequential reservation offers unless Wave 0 explicitly scopes broadcast offers.
- [ ] Implement the legal reservation transition table and tests for invalid transitions.
- [ ] Enforce one active offered/accepted reservation per agent.
- [ ] Enforce one accepted reservation per route request with idempotent or rejected stale accepts.
- [ ] Enforce guarded reservation transitions from offered to accepted, rejected, timeout, or cancelled.
- [ ] Implement cancellation trigger handling for cancelled reservations.
- [ ] Implement retry as a runtime action that creates a new offered reservation attempt.
- [ ] Wire runtime-owned `Ready -> Engaged` and `Engaged -> WrapUp` transitions through the existing agent-state rules.
- [ ] Persist append-only runtime events and derived trace records.
- [ ] Add latency measurement and a report-only p95 under 50 ms route-decision budget.
- [ ] Add tests for unavailable catalog references in published flows.

## 5. Simulator and Trace Viewer

- [ ] Implement simulation endpoint using synthetic inputs without mutating live reservations or agent state.
- [ ] Persist or return deterministic simulation traces.
- [ ] Support record/mock behavior for effect nodes in simulation.
- [ ] Ensure simulation cannot mutate live reservation or agent-state tables.
- [ ] Capture minimal catalog/state read sets for replay instead of full catalog dumps.
- [ ] Record continuation fire times, virtual-clock advances, reservation outcome signals, and candidate tie-break inputs as replay inputs.
- [ ] Replace vNext trace viewer mock data with API-backed trace fixtures.
- [ ] Add tests proving replay stability for the same flow version, catalog snapshot, input, and initial state.

## 6. UI and Embed

- [ ] Convert `or-flow-list` from playground preview to API-backed shared UI component.
- [ ] Convert `or-flow-builder` to load, validate, simulate, and publish real flow drafts.
- [ ] Convert `or-trace-viewer` to load real runtime or simulation traces.
- [ ] Add flow routes to standalone admin.
- [ ] Expose the flow builder as an embeddable Web Component (Shadow-DOM-safe canvas) under the v0.1 trusted-host org model, and validate it runs embedded, not only standalone.
- [ ] Keep playground entries as fixture-backed design references for regression screenshots.
- [ ] Add UI states for empty lists and API errors in flow list, builder, simulator, and trace viewer.

## 7. CI and Closure

- [ ] Add backend runtime unit and integration test jobs.
- [ ] Add frontend flow UI typecheck, unit, and admin build gates.
- [ ] Add end-to-end smoke for publish, simulate, execute, reserve, and trace.
- [ ] (v0.3) Add the desktop/mobile Playwright screenshot matrix and UAT gate for flow list, builder, simulator mode, and trace viewer.
- [ ] Verify business, output, UI, and review gates in `acceptance.md`.
- [ ] Run `openspec validate --strict` or the repo-local equivalent when an OpenSpec validator is available.
- [ ] Run cross-AI peer review on the full v0.2 implementation diff.
- [ ] Merge accepted behavior into `openspec/specs/` and archive this change when v0.2 ships.
