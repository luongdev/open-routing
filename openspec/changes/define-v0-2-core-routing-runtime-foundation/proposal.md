# Define v0.2 Core Routing Runtime Foundation

## Why

v0.1 shipped the catalog foundation and Web Component catalog embed. The next useful milestone needs to turn the post-v0.1 product intent from `.planning/PROJECT.md` and the vNext playground previews into an implementation-grade OpenSpec change.

The playground already has flow list, flow builder with simulator mode, and trace viewer mock screens, but those screens are design previews only. v0.2 should formalize the backend/runtime contracts they need before treating the UI as shipped behavior.

## What Changes

**Milestone planning**

- From: v0.1 is archived and the next milestone is not formalized.
- To: v0.2 is proposed as the Core Routing Runtime Foundation milestone.
- Reason: New work needs an active OpenSpec change instead of resuming historical GSD rows.
- Impact: Implementation can be planned and reviewed against proposal, design, tasks, and delta specs.

**Flow authoring**

- From: flow authoring exists only as product intent and playground mock UI.
- To: v0.2 defines persisted flow drafts, immutable published versions, graph validation, simulation, publish, and rollback behavior.
- Reason: Runtime execution cannot be reliable until graph identity, versioning, and publish governance are explicit.
- Impact: The vNext flow list/builder can graduate from mock data to API-backed surfaces when the contract lands.

**Runtime execution**

- From: Open Routing has catalog data and agent state, but no runtime caller for routing decisions.
- To: v0.2 defines a deterministic runtime foundation that executes compiled flow plans against catalog snapshots and interaction input.
- Reason: The product's core value is routing behavior, not only catalog configuration.
- Impact: Runtime routes are testable before real adapters or media integrations exist.

**Reservation and state transitions**

- From: `Ready -> Engaged` and `Engaged -> WrapUp` are schema-ready but have no runtime owner.
- To: v0.2 makes the runtime the owner of offer/reservation lifecycle and system-initiated state transitions.
- Reason: ACD routing needs offered reservations, accept/reject/timeout outcomes, retry actions, and post-interaction cleanup, not a one-shot selected agent.
- Impact: Existing agent-state invariants are exercised by real runtime paths.

**Trace and simulator mode**

- From: trace viewer and simulator are playground previews backed by mock trace data.
- To: v0.2 defines deterministic simulation and trace records for routing runs.
- Reason: Product teams need to explain and replay routing outcomes before trusting published flows.
- Impact: Debug output becomes a first-class contract instead of a UI-only artifact.

## Non-Goals

- Real media holding, call control, chat execution, or email execution.
- Real FreeSWITCH, LiveKit, CRM, ticketing, or WFM adapters.
- Full standalone plus embedded SSO/RBAC.
- Async bulk import, error CSV download, and import dry-run unless separately scoped.
- Iframe or Module Federation embedding.
- Full power-user DSL editor if graph JSON export/import is enough for v0.2.

## Source Planning

Migrated from `.planning/PROJECT.md`, `.planning/REQUIREMENTS.md` future requirements, `.planning/research/`, Phase 4 runtime deferrals, and the vNext playground components under `web/packages/ui/src/components/vnext/`.

## Decision Gates

Wave 0 decisions are locked as of 2026-06-02; implementation may proceed. The canonical decision table, status fields, business gates, output gates, UI gates, and review gates live in `acceptance.md`. Notable changes from the original recommendations: the runtime runs as a separate `cmd/runtime` process, and the flow builder is embeddable as a Web Component from v0.2.
