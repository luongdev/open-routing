# Implementation Plan: v0.2 Core Routing Runtime Foundation

## Goal

Ship a narrow, end-to-end routing foundation: author a flow, validate it, simulate it, publish it, execute a route request, create and resolve a reservation, drive runtime-owned agent-state transitions, and inspect the resulting trace.

## Working Assumptions

- Runtime starts as an internal Go package in `services/api`.
- UI is API-backed in standalone admin first.
- Embed flow routes are deferred unless explicitly confirmed.
- Realtime push is deliberately rescoped from old GSD v0.2 notes; traces and state are read through REST/polling unless Wave 0 confirms SSE.
- DSL is export/import/review from graph JSON, not a full editor.
- Adapter behavior is proved through adapter-command contracts and test doubles.
- Reservation states are `offered`, `accepted`, `rejected`, `timeout`, `cancelled`, and `completed`; retry is an action that creates another offer.

## Wave 0: Scope Lock And Contract Sketch

Purpose: remove gray-area ambiguity before implementation starts.

Deliverables:

- Confirm the working assumptions above.
- Confirm the v0.2 executable node subset without expanding beyond the research list.
- Sketch REST/OpenAPI response shapes for flows, publish, simulation, admin-facing route requests, reservations, and traces.
- Decide naming for tables and public API nouns.
- Decide whether flow UI is admin-only or included in embed modules.

Gates:

- OpenSpec proposal/design/tasks updated with confirmed decisions.
- Cross-AI review of the locked scope before code changes.

## Wave 1: Data Model And Codegen

Purpose: create the durable contract without business logic drift.

Deliverables:

- Migrations for flow drafts, flow versions, publish records, runtime events, route requests, reservations, traces, and catalog snapshots.
- sqlc queries for all new tables.
- OpenAPI schemas and paths.
- Regenerated Go and TypeScript clients.
- orgDB/SQLChecker coverage for every new query.

Gates:

- `task gen` produces no diff after committed generated files.
- Backend tests prove org isolation for every new table.
- OpenAPI and sqlc drift gates pass.

## Wave 2: Flow Draft, Validation, Compile, Publish

Purpose: make graph authoring and publish governance real before runtime execution.

Deliverables:

- Flow draft CRUD with code identity and optimistic versioning.
- Graph validator for node shape, edges, orphan nodes, cycles where disallowed, and catalog references.
- Compiler from graph to a deterministic v0.2 executable plan.
- Publish and rollback endpoints.
- Immutable published versions.

Gates:

- Invalid graphs fail with node/edge/field-level errors.
- Cross-org catalog references fail.
- Publish requires validation.
- Rollback activates an older published version and records the version used.

## Wave 3: Runtime And Reservations

Purpose: execute published plans and prove assignment lifecycle.

Deliverables:

- Runtime package with deterministic execution inputs.
- Route request endpoint and execution result persistence.
- Reservation lifecycle states and transition APIs/internal handlers.
- Runtime-owned `Ready -> Engaged` and `Engaged -> WrapUp` through existing state rules.
- Runtime event writes with canonical envelope.
- p95 route-decision measurement fixture and report.

Gates:

- Route request with no published flow returns typed failure.
- Successful route creates reservation and trace.
- Reservation timeout follows fallback path.
- Test-double adapter or simulator can accept, reject, or let offers time out.
- Accepted reservation moves agent to Engaged.
- Completion moves agent to WrapUp and server-owned expiry still works.
- Performance report is produced in CI.

## Wave 4: Simulation And Trace Replay

Purpose: make debug deterministic and useful before UI polish.

Deliverables:

- Simulation endpoint that does not mutate live reservations or agent state.
- Trace shape with ordered steps, inputs, outputs, timings, catalog refs, effect status, and errors.
- Record/mock behavior for effect nodes.
- Saved test cases or replay fixtures for deterministic traces.

Gates:

- Same pinned inputs produce the same trace.
- Live effects are disabled by default in simulation.
- Failing effect trace explains failure without leaking secrets or raw unsafe payloads.

## Wave 5: Admin UI Graduation

Purpose: move vNext screens from fixture preview to API-backed product surface.

Deliverables:

- API-backed `or-flow-list`.
- API-backed `or-flow-builder` with validation, simulation, publish, and rollback actions.
- API-backed `or-trace-viewer`.
- Admin routes for flows and traces.
- Playground entries retained as fixture-backed visual regression surfaces.

Gates:

- UI tests cover loading, validation errors, publish, simulation, and trace display.
- `pnpm --filter @open-routing/ui typecheck` passes.
- `pnpm --filter @open-routing/admin build` passes.

## Wave 6: CI, Review, And Archive

Purpose: make the milestone shippable and archive the OpenSpec change.

Deliverables:

- Backend runtime unit/integration gates.
- Frontend flow UI gates.
- End-to-end smoke for publish, simulate, execute, reserve, complete, trace.
- Branch-protection docs updated for v0.2 checks.
- Cross-AI review recorded.
- Delta specs merged into current `openspec/specs/`.
- Change archived under `openspec/changes/archive/YYYY-MM-DD-define-v0-2-core-routing-runtime-foundation/`.

Gates:

- `git diff --check`.
- Relevant Go tests pass.
- Relevant pnpm typecheck/build/test gates pass.
- Cross-AI review has no blockers.

## Explicit Deferrals

- Real auth/RBAC.
- Full DSL editor.
- SSE and browser workers, unless Wave 0 explicitly confirms live routing monitor scope.
- Production-shaped voice/chat/email mock adapters.
- Real adapter integrations.
- Async bulk import.
- Iframe or Module Federation embedding.
