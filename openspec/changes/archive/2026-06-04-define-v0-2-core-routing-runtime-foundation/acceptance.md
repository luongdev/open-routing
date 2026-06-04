# Acceptance Gates: v0.2 Core Routing Runtime Foundation

## Purpose

OpenSpec does not guarantee correct shipping by itself. This change is only useful if it defines decision ownership, business acceptance, output acceptance, UI acceptance, verification gates, and review gates.

Wave 0 decisions are now **locked as of 2026-06-02**. Implementation may proceed on these; reopening any row requires a fresh cross-AI review and explicit user sign-off.

## User-Owned Decision Gates

All rows are resolved. `accepted` = locked on the recommended default; `changed` = locked on a different choice than the original recommendation. Rationale lives in `design.md`.

| Decision | Locked choice | Status | Decided |
|----------|---------------|--------|---------|
| v0.2 scope | Runtime foundation + API-backed flow workspace; UI-graduation polish (full Playwright matrix, UAT sign-off) and the perf-gate hard-fail are deferred to v0.3 | accepted | 2026-06-02 |
| Runtime boundary | Separate `cmd/runtime` process sharing an `internal/runtime` library; simulation runs in-process in `cmd/api`; inter-plane gRPC deferred | changed | 2026-06-02 |
| Realtime | REST/read APIs + polling; SSE deferred to v0.3+ | accepted | 2026-06-02 |
| DSL depth | Graph JSON export/import/review only | accepted | 2026-06-02 |
| Adapter scope | Adapter-command contract + test doubles | accepted | 2026-06-02 |
| Embed exposure | Flow builder is embeddable from v0.2 (Shadow-DOM-correct), under the v0.1 trusted-host org model; full embed polish deferred | changed | 2026-06-02 |
| Audit scope | Runtime events are trace/debug infrastructure, not the org-facing audit retention contract | accepted | 2026-06-02 |
| Auth/RBAC | Keep v0.1 stub org context; v0.2 is internal/trusted-host only and never a production security claim | accepted | 2026-06-02 |
| Adapter reference model | Validate existing v0.1 adapter catalog rows only | accepted | 2026-06-02 |
| Flow entry binding | Bind published flows by org, channel, and entry code | accepted | 2026-06-02 |
| Live delayed continuation | Durable DB-backed continuations claimed via `FOR UPDATE SKIP LOCKED` in the runtime worker | accepted | 2026-06-02 |
| Reservation integrity | DB-only guard: partial unique index + guarded single-row CAS transitions (Option A). Redis fast-path deferred until measured contention justifies it | accepted | 2026-06-02 |
| Reservation offer policy | Sequential offers first; broadcast offers deferred | accepted | 2026-06-02 |
| Candidate selection | Skill-based eligibility + proficiency rank + longest-available tie-break + stable final tie-break key; weighted scoring engine deferred | accepted | 2026-06-02 |
| Node subset | Narrow 12-node subset; each node implements a `Validate`/`Compile`/`Execute` contract plus a UI descriptor, registered in a node registry so adding a node is O(1) | accepted | 2026-06-02 |
| Effect mode | Record/mock effect nodes only | accepted | 2026-06-02 |

## Business Acceptance Gates

The milestone is not shippable unless these product outcomes are demonstrably true:

- A user can create a flow draft, validate it, simulate it, publish it, execute it, and inspect the trace.
- The data model enforces at most one active published flow binding per org, channel, and entry code.
- Successful publish and rollback leave one active published flow binding for their target org, channel, and entry code.
- A published route can select an eligible agent using catalog data, skill/proficiency data, queue data, agent state, and break-reason routability.
- Assignment is modeled as a reservation lifecycle, not a one-shot selected agent.
- The same agent cannot hold two active offered/accepted reservations at once.
- The same route request cannot be accepted by two agents.
- Reservation accept is idempotent for the winning reservation and rejects or no-ops stale competing accepts.
- Reservation accept, timeout, reject, and cancellation are guarded by current-state transitions so only one terminal path wins.
- Reservation timeout follows a fallback path and records why.
- Reservation cancellation records the cancellation trigger and does not move the agent to Engaged.
- Live wait nodes and reservation timeouts resume through durable continuations, not browser state or volatile timers only.
- Accepted reservation moves the agent to Engaged through existing agent-state rules.
- Completion moves the agent to WrapUp and server-owned WrapUp expiry still works.
- Simulation and replay are deterministic for fixed flow version, catalog snapshot, interaction input, state snapshot, virtual clock, reservation outcome signals, and effect records.
- Runtime output stays inside Open Routing's boundary: no media holding, no call control, no CRM/ticketing ownership, and no agent desktop behavior.
- Multi-org isolation is proven for flows, published versions, route requests, reservations, runtime events, and traces.

## Output Acceptance Gates

The milestone is not shippable unless the produced contracts and artifacts are coherent:

- OpenAPI paths and schemas exist for flow drafts, publish, rollback, simulation, route requests, reservations, and traces.
- Go and TypeScript generated clients match the OpenAPI contract with no drift.
- Migrations and sqlc queries cover every new table.
- Every new repository query flows through `orgDB`/SQLChecker enforcement unless it has an explicit audited bypass.
- Runtime events use one canonical envelope.
- Typed routing failures have a canonical taxonomy for missing published flow, missing catalog reference, no eligible candidate, multiple active bindings, stale draft version, invalid graph, and reservation transition conflict.
- Trace records include ordered steps, node references, inputs, outputs, status, timing, selected catalog refs, effect status, and errors.
- Route-decision p95 is reported against a stable fixture using the project target of p95 under 50 ms. Failing the build on this budget is deferred until the fixture is stable enough to be meaningful.
- Route execution snapshots store the minimal read set needed for deterministic replay rather than a full catalog dump for every trace.
- Replay inputs include continuation fire times and virtual-clock advances for live time-driven paths.

## UI Acceptance Gates

The milestone is not shippable unless the flow UI is real product UI, not playground-only mock UI.

Required in v0.2:

- Production admin routes do not read from `playground-mock-data`.
- Flow list loads API data and clearly shows draft, published, and archived states.
- Flow builder can load a real draft, edit graph data, validate, simulate, publish, and rollback.
- Flow builder runs correctly when embedded as a Web Component (Shadow-DOM-safe canvas), not only in the standalone admin.
- Validation errors map back to the relevant node, edge, or field.
- Empty states and API-error states are implemented for flow list and builder.
- Simulator mode shows pinned inputs, run state, variable bag, effect mode, and deterministic replay output.
- Trace viewer loads real runtime or simulation traces and explains the route path without leaking cross-org records.

Deferred to v0.3 (UI-graduation polish):

- Empty/error-state coverage extended to simulator and trace viewer.
- Desktop and mobile Playwright screenshots for flow list, flow builder, simulator mode, and trace viewer.
- Screenshot review for text fit, overlap, responsive layout, action affordances, and visual consistency with the v0.1 catalog UI.
- User UAT sign-off across all four surfaces.

## Review Gates

- Claude and agy/Gemini review the locked Wave 0 decisions before implementation.
- Claude and agy/Gemini review the implementation diff before v0.2 archive.
- Review results and responses are recorded in this change folder.
- Any blocker from either reviewer must be fixed or explicitly waived by the user before archive.
