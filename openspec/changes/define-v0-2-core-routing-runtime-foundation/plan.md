# Implementation Plan: v0.2 Core Routing Runtime Foundation

## Goal

Ship a narrow, end-to-end routing foundation: author a flow, validate it, simulate it, publish it, execute a route request, create and resolve a reservation, drive runtime-owned agent-state transitions, and inspect the resulting trace.

## Working Assumptions

- Runtime runs as a separate `cmd/runtime` process sharing the `internal/runtime` library and database with `cmd/api`; simulation runs in-process in `cmd/api`; inter-plane gRPC deferred.
- UI is API-backed in standalone admin first.
- Flow builder is embeddable as a Web Component from v0.2 (Shadow-DOM-correct), under the v0.1 trusted-host org model; full embed polish deferred to v0.3.
- Realtime push is deliberately rescoped from old GSD v0.2 notes; traces and state are read through REST/polling. SSE deferred to v0.3+.
- DSL is export/import/review from graph JSON, not a full editor.
- Adapter behavior is proved through adapter-command contracts and test doubles.
- Reservation states are `offered`, `accepted`, `rejected`, `timeout`, `cancelled`, and `completed`; retry is an action that creates another offer.
- v0.2 is sequential-offer first; broadcast offers are deferred unless Wave 0 explicitly scopes them.
- Simulation and replay use a virtual clock for `wait`, reservation timeout behavior, and recorded reservation outcome signals.
- Runtime events are debug/trace infrastructure in v0.2, not the audit retention contract.
- Flow entry binding is by org, channel, and entry code unless Wave 0 changes it.
- The data model enforces at most one active published binding per org, channel, and entry code; successful publish and rollback leave one active binding for their target.
- In-flight route requests finish on the published flow version they pinned at start, even if rollback happens.
- Reservation integrity is DB-only: a partial unique index plus guarded single-row CAS transitions (Option A). No Redis lock in v0.2; a Redis fast-path lease may be added later only above the DB guard.
- Reservation outcomes use guarded current-state transitions so accept, reject, timeout, and cancellation races cannot double-resolve.
- Live wait and timeout behavior uses durable continuations, not volatile timers only.
- Candidate selection is skill-based: eligibility by required skills, rank by proficiency, longest-available tie-break, stable final tie-break key. Weighted scoring deferred.
- Each node implements a `Validate`/`Compile`/`Execute` contract plus a UI descriptor registered in a node registry, so the subset can grow by one file per node during iteration.
- Time-driven work (live `wait`, reservation timeouts, WrapUp expiry) is consolidated in the `cmd/runtime` worker, claimed via `FOR UPDATE SKIP LOCKED`, replacing the v0.1 single-replica ticker sweeper.
- `route_request` is the v0.2 interaction spine; reservations, agent-state transitions, runtime events, and traces reference it.

## Hard Gates

- Wave 0 cannot close until the user-owned decision gates in `acceptance.md` are confirmed.
- Implementation cannot start from playground UI alone; runtime/API contracts must be planned first.
- v0.2 cannot archive until business, output, UI, and review gates in `acceptance.md` pass.

## Wave 0: Scope Lock And Contract Sketch

Purpose: remove gray-area ambiguity before implementation starts.

Deliverables:

- Confirm the user-owned decision gates from `acceptance.md`.
- Confirm the v0.2 executable node subset without expanding beyond the research list.
- Sketch REST/OpenAPI response shapes for flows, publish, simulation, admin-facing route requests, reservations, and traces.
- Decide naming for tables and public API nouns.
- Disambiguate draft optimistic-lock revision from immutable published version naming.
- Scope the Shadow-DOM embed work for the flow builder (decided: embeddable from v0.2).
- Adapter references validate existing v0.1 catalog rows (decided).

Gates:

- OpenSpec proposal/design/tasks updated with confirmed decisions.
- `acceptance.md` decision gates updated with accepted or changed decisions.
- Cross-AI review of the locked scope before code changes.

## Wave 1: Data Model And Codegen

Purpose: create the durable contract without business logic drift.

Deliverables:

- Migrations for flow drafts, flow versions, publish records, flow entry bindings, runtime events, durable continuations, route requests, reservations, traces, and replay read-set snapshots.
- Database-backed invariant for at most one active published flow binding per org, channel, and entry code.
- Partial unique indexes for reservation integrity: one active offered/accepted reservation per agent, and one accepted reservation per route request.
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
- Per-node `Validate`/`Compile`/`Execute` contract plus UI descriptor, registered in a node registry that validation, compilation, runtime, and the UI palette iterate generically.
- Compiler from graph to a deterministic v0.2 executable plan.
- Compiled-plan format version field or equivalent compatibility contract.
- Flow entry-point binding by channel and entry code.
- Publish and rollback endpoints.
- Immutable published versions.
- Transactional activation of published bindings so runtime cannot observe two active versions for one binding.

Gates:

- Invalid graphs fail with node/edge/field-level errors.
- Cross-org catalog references fail.
- Publish requires validation.
- Rollback activates an older published version and records the version used.
- In-flight route requests keep running on their pinned published version after rollback.

## Wave 3: Runtime And Reservations

Purpose: execute published plans and prove assignment lifecycle.

Deliverables:

- Shared `internal/runtime` library with deterministic execution inputs, plus a separate `cmd/runtime` entrypoint that wraps it; `cmd/api` imports the same library for in-process simulation.
- Skill-based candidate selection: eligibility by required skills, proficiency rank, longest-available tie-break, and a stable final tie-break key.
- Virtual clock support for wait nodes, timeout paths, simulation, and replay.
- Durable continuation storage and a `cmd/runtime` worker that claims due rows with `FOR UPDATE SKIP LOCKED` for live wait nodes and reservation timeouts; v0.1 WrapUp expiry migrates onto this claim pattern so multiple runtime replicas stay safe.
- Continuation records carry pinned flow version, execution cursor, and due-time metadata.
- Reservation-timeout continuation claiming stays compatible with server-owned WrapUp expiry semantics.
- Route request endpoint and execution result persistence.
- Reservation lifecycle states and transition APIs/internal handlers.
- Explicit legal-transition table for reservation states.
- Database-backed reservation integrity for one active reservation per agent and one accepted reservation per route request.
- Guarded reservation transitions from offered to accepted, rejected, timeout, or cancelled.
- Runtime-owned `Ready -> Engaged` and `Engaged -> WrapUp` through existing state rules.
- Runtime event writes with canonical envelope.
- p95 route-decision measurement fixture and report against the project target of p95 under 50 ms.

Gates:

- Route request with no published flow returns typed failure.
- Successful route creates reservation and trace.
- Published flow with unavailable catalog reference returns a typed failure or follows an explicit fallback path.
- Reservation timeout follows fallback path.
- Test-double adapter or simulator can accept, reject, or let offers time out.
- Route request cancellation records cancelled reservations without moving agents to Engaged.
- Concurrent offers cannot double-book one agent.
- Racing accepts cannot produce two accepted reservations for one route request.
- Accept-vs-timeout races have exactly one winning transition.
- Accepted reservation moves agent to Engaged.
- Completion moves agent to WrapUp and server-owned expiry still works.
- Performance report is produced in CI.

## Wave 4: Simulation And Trace Replay

Purpose: make debug deterministic and useful before UI polish.

Deliverables:

- Simulation endpoint that does not mutate live reservations or agent state.
- Trace shape with ordered steps, inputs, outputs, timings, catalog refs, effect status, and errors.
- Record/mock behavior for effect nodes.
- Minimal read-set snapshot strategy for deterministic replay without full catalog dumps.
- Recorded continuation fire times, virtual-clock advances, reservation outcome signals, and deterministic candidate tie-break inputs for replay.
- Saved test cases or replay fixtures for deterministic traces.

Gates:

- Same pinned inputs produce the same trace.
- Live effects are disabled by default in simulation.
- Simulation cannot mutate live reservation or agent-state tables.
- Failing effect trace explains failure without leaking secrets or raw unsafe payloads.

## Wave 5: Admin UI Graduation

Purpose: move vNext screens from fixture preview to API-backed product surface.

Deliverables:

- API-backed `or-flow-list`.
- API-backed `or-flow-builder` with validation, simulation, publish, and rollback actions.
- `or-flow-builder` validated running embedded as a Web Component (Shadow-DOM-safe canvas), not only standalone.
- API-backed `or-trace-viewer`.
- Admin routes for flows and traces.
- Playground entries retained as fixture-backed visual regression surfaces.

Deferred to v0.3 (UI-graduation polish): desktop/mobile Playwright screenshot matrix and user UAT checkpoint across all four surfaces.

Gates:

- UI tests cover loading, validation errors, publish, simulation, and trace display.
- UI tests cover empty states and API-error states for flow list and builder.
- `or-flow-builder` functions inside a Shadow-DOM host without canvas/event regressions.
- No mock-data leakage on production admin routes.
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
- OpenSpec structural validation passes when a repo-local or installed CLI is available.

## Explicit Deferrals

- Real auth/RBAC.
- Full DSL editor.
- SSE and browser workers (v0.3+).
- Production-shaped voice/chat/email mock adapters.
- Real adapter integrations.
- Broadcast reservation offers.
- External adapter delivery idempotency for duplicate real interactions.
- Post-accept cancellation unless Wave 0 explicitly scopes it.
- Async bulk import.
- Iframe or Module Federation embedding.
- Redis reservation fast-path lease (revisit if measured contention justifies it).
- Weighted scoring / route-strategy engine.
- Inter-plane gRPC between `cmd/api` and `cmd/runtime`.
- UI-graduation polish — full desktop/mobile Playwright matrix and UAT sign-off (v0.3).
- Perf-gate hard-fail on p95 < 50 ms (report-only in v0.2; hard-fail in v0.3).
