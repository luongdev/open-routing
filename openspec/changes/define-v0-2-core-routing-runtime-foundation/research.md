# Research: v0.2 Core Routing Runtime Foundation

## Sources Reviewed

- `.planning/PROJECT.md`: product boundary, graph-first flow model, deterministic debug, outbox-first storage, runtime split.
- `.planning/REQUIREMENTS.md`: future requirements for FLOW, RT, ADP, SIM, AUTH, RT-PUSH, WORK, IMP, AUD, and catalog extensions.
- `.planning/research/SUMMARY.md`: domain summary, agent state implications, roadmap implications, and known implementation gaps.
- `.planning/research/FEATURES.md`: vendor-derived agent state and reservation lifecycle evidence.
- `.planning/research/PITFALLS.md`: multi-org, state-machine, async context, and routing candidate hazards.
- `.planning/phases/04-agent-state-machine-go/04-CONTEXT.md`: deferred runtime transitions and agent-state invariants.
- `web/packages/ui/src/components/vnext/`: flow list, flow builder, simulator, and trace viewer previews.

## Findings

### 1. v0.2 should not be UI-only

The vNext playground already communicates a product direction, but the code marks it as preview/mock-only. Treating it as the milestone would leave the core product value unproven. The next milestone should make the backend/runtime contract real and then graduate the UI onto that contract.

Recommendation: v0.2 should ship a narrow runtime foundation plus API-backed flow workspace, not only a polished flow-builder prototype.

### 2. Runtime can start inside the existing Go API boundary

The project has already locked Go, chi, sqlc, pgx, OpenAPI, and app-layer org isolation. Phase 4 prepared system-owned state transitions but deliberately deferred their runtime caller. A separate runtime process is directionally correct long term, but requiring it in v0.2 adds deployment and CI complexity before the runtime contract is proven.

Recommendation: implement runtime as an internal Go package first, with explicit interfaces for future extraction. Do not add a separate `cmd/runtime` until there is evidence that process separation is needed.

### 3. Existing agent-state model is ready for runtime ownership

Phase 4 shipped `Engaged`, `WrapUp`, `engaged_channel`, `post_interaction_state`, `wrapup_until`, and server-owned WrapUp expiry. It explicitly deferred `Ready -> Engaged` and `Engaged -> WrapUp` to the runtime engine. The correct v0.2 work is to wire runtime-owned transitions through the existing state guard, not bypass it.

Recommendation: runtime should call state transition helpers, preserve invalid-transition safety, and record state-transition failures in the route trace.

### 4. Reservation lifecycle is the correct routing unit

Vendor research shows assignment is not a one-shot "selected agent" result. Twilio TaskRouter-style reservation states and other ACD systems use an offer/accept/timeout/fallback model. The vNext playground also uses a `reservation` node with accepted, rejected, and timeout outputs.

Recommendation: v0.2 should model route assignment as reservations with explicit lifecycle states: offered, accepted, rejected, timeout, cancelled, completed. Retry is a runtime action that creates a new offered reservation attempt, not a persisted reservation state. Runtime outcomes and traces should reference reservation IDs.

### 5. Flow node scope must be cut down

The playground taxonomy includes broad workflow nodes: scripts, HTTP requests, channel-specific voice/chat/email actions, surveys, state nodes, and side effects. Shipping all of that in v0.2 would turn a foundation milestone into a full workflow engine.

Recommendation: v0.2 should support this executable subset:

- `trigger`
- `if_else`
- `switch_case`
- `wait`
- `match_skill`
- `filter`
- `route_queue`
- `reservation`
- `fallback`
- `effect` in record/mock mode
- `log`
- `end`

Everything else can stay visible only as playground/design vocabulary or future node taxonomy.

### 6. Determinism requires snapshots and effect controls

The project requirement is deterministic debug: the same flow version, catalog snapshot, interaction input, and state snapshot produce the same trace. The vNext trace data already models step inputs/outputs and effect mock notes.

Recommendation: simulation and replay must pin flow version, catalog snapshot, interaction input, initial state snapshot, and effect mode. The snapshot contract must store enough catalog and state values to replay without consulting mutable current catalog rows or Redis cache. Live effects should be disabled by default in simulation.

### 7. Outbox-first is enough for v0.2; SSE can wait

The project wants runtime events, state/event streaming, and future SSE/workers. Older GSD notes pointed SSE/workers at v0.2, but v0.2 can prove runtime behavior with durable events, trace APIs, and polling. Adding SSE now would pull in fan-out, browser worker, reconnect, and multi-tab decisions before the route engine is stable.

Recommendation: persist runtime events and traces first. Treat SSE/workers as a deliberate v0.2 rescope unless routing monitor live behavior becomes a confirmed requirement in Wave 0.

### 8. Keep auth and real adapters out of v0.2

The v0.1 stub org context remains a known limitation. Real auth/RBAC and production-shaped adapters are important, but either one can dominate the milestone. The runtime foundation can still be validated using org-scoped tests and mock adapter acknowledgements.

Recommendation: do not make AUTH or full voice/chat/email mock adapters part of v0.2 acceptance. Use adapter-command contracts plus test doubles.

## Recommended Decisions For User Confirmation

- v0.2 scope: runtime foundation plus API-backed flow workspace.
- Runtime boundary: internal Go package first, extract later.
- Realtime: durable events plus read APIs first, no SSE by default.
- DSL: graph JSON import/export/review, no full DSL editor.
- Adapter scope: adapter command contract plus test doubles, no full mock adapters yet.
- UI exposure: standalone admin first; embed exposure only after module/host impact is explicitly scoped.

## Risks

- The node subset may still be too broad if each node receives production-grade UI and validation in the first pass.
- Runtime event schema can become audit schema by accident; audit needs org-facing retention and display decisions that may be broader than trace debugging.
- Existing no-FK posture means every runtime reference check must be app-layer and test-backed.
- Performance target needs a realistic fixture budget; v0.2 should report p95 first instead of failing builds on an unstable benchmark.
