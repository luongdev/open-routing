# Review Log: v0.2 Core Routing Runtime Foundation

## 2026-06-02: agy/Gemini review

Status: Approved / no blockers after Wave 0 scope review.

Key feedback accepted:

- Keep runtime in the Go API process for v0.2.
- Keep realtime as REST/read APIs and polling first.
- Use durable DB-backed continuations for live waits and reservation timeouts.
- Enforce reservation integrity with database constraints and transactional updates.
- Keep the executable node subset tight, especially `effect` and `wait`.
- Store deterministic replay snapshots efficiently as a minimal read set.
- Prefer sharing continuation infrastructure with server-owned WrapUp expiry where practical.

Response:

- `acceptance.md`, `design.md`, `plan.md`, and runtime specs now require durable continuations, guarded reservation transitions, minimal replay read sets, and record/mock `effect` behavior unless explicitly changed in Wave 0.
- Full reviewer output is captured in `gemini-review.md`.

## 2026-06-02: Claude review

Initial blockers:

- Reservation double-booking and atomic claim behavior were underspecified.
- Live wait and reservation-timeout continuation behavior was not production-safe enough.
- Flow entry binding did not specify how route requests resolve a published version.

Response:

- Runtime specs now require DB-backed reservation integrity, durable delayed continuations, and exact flow binding resolution by org, channel, and entry code.
- Design and plan now pin in-flight route requests to the published version selected at route-request start.

Second-pass findings:

- Accept-vs-timeout race needed an explicit guarded transition.
- Active published flow binding needed a publish-time uniqueness invariant.
- `cancelled` reservation state needed a scenario and trigger.
- Replay of live time-driven runs needed recorded continuation fire times.
- `effect` mode needed its own user-owned decision gate.

Response:

- `routing-runtime/spec.md` now requires guarded offered-to-terminal transitions, cancelled reservation behavior, and replay inputs for continuation fire times.
- `flow-authoring/spec.md` now requires an at-most-one active binding invariant and one active binding after successful publish or rollback.
- `acceptance.md`, `plan.md`, and `tasks.md` now include these as milestone gates and implementation tasks.

Third-pass blockers:

- User-owned decision gates were not recordable because the table had no status field.
- Reservation lifecycle lacked an explicit legal-transition table, and `offered -> completed` was incorrectly implied.
- Replay determinism did not record reservation outcome signals or candidate tie-break inputs.
- Runtime behavior was undefined when a published flow references a now-unavailable catalog row.
- "Exactly one active binding" was mixed with a DB invariant that can only enforce at-most-one.

Response:

- `acceptance.md` now has `Status` and `Decided on` columns, with all decisions pending user confirmation.
- `routing-runtime/spec.md` now defines legal reservation transitions, guarded competing terminal outcomes, missing catalog-reference behavior, deterministic candidate ordering, trace equality, and reservation outcome replay inputs.
- `flow-authoring/spec.md`, `acceptance.md`, `design.md`, `plan.md`, and `tasks.md` now distinguish at-most-one binding invariants from successful publish/rollback postconditions.

## 2026-06-02: Final re-review

Status: Approved / no blockers from both Claude and agy/Gemini.

Final reviewer notes addressed:

- v0.2 now states sequential offers first; broadcast offers are explicitly deferred unless Wave 0 changes the decision.
- `routing-runtime/spec.md` now includes completion of accepted reservations through the test-double adapter or simulator.
- `tasks.md` now includes OpenSpec structural validation when a validator is available.
- Wave 0 tasks now enumerate audit scope and auth/RBAC, matching the decision table.
- `design.md` now calls out compatible claiming semantics between reservation-timeout continuations and server-owned WrapUp expiry.
- `plan.md` now defers external adapter delivery idempotency for duplicate real interactions until real adapters are scoped.

## 2026-06-02: Wave 0 decision lock (user)

All decision gates in `acceptance.md` are locked. Two rows changed from the reviewed recommendation; the rest accepted on default.

Changed:

- **Runtime boundary** → separate `cmd/runtime` process (not internal package). Config plane and runtime plane scale, deploy, and fail differently, matching PROJECT.md "split control-plane and runtime-engine from the start." Cost kept low: shared Go module + `internal/runtime` library, shared DB, gRPC deferred. Simulation runs in-process in `cmd/api`. Bonus: the runtime worker consolidates time-driven work (waits, reservation timeouts, WrapUp expiry) onto a `FOR UPDATE SKIP LOCKED` claim, retiring the v0.1 single-replica `time.Ticker` sweeper limitation.
- **Embed exposure** → flow builder must be embeddable as a Web Component from v0.2 (Shadow-DOM-correct canvas), so the embed is dogfooded as the test surface; full embed/UI polish deferred to v0.3.

Accepted with refinement:

- **Reservation integrity** → DB-only guard (partial unique index + guarded single-row CAS). Redis distributed lock rejected as source of truth: a multi-second offer + TTL lock means expire-mid-offer (double-book) or ghost-lock on crash, and it cannot be atomic with reservation state in Postgres. The reservation row + partial unique index *is* the cross-replica lock. Redis fast-path lease may sit above the DB guard later if measured contention justifies it.
- **Candidate selection** (new decision, previously unspecified) → skill-based eligibility + proficiency rank + longest-available tie-break + stable final tie-break key; weighted scoring engine deferred.
- **Node subset** → narrow 12-node set, but a node registry + per-node `Validate`/`Compile`/`Execute` contract + UI descriptor is mandatory so the set grows by one file per node during iteration.
- **Reservation offer policy** split out from integrity → sequential offers first, broadcast deferred.
- **Flow entry binding** → closed as accepted (org + channel + entry code); it was incorrectly left `pending` while the whole spec was already built on it.

Scope:

- v0.2 keeps the hard distributed-systems correctness (durable continuations, guarded reservations, deterministic replay). UI-graduation polish (full Playwright matrix, UAT sign-off) and the perf-gate hard-fail are deferred to v0.3.

Pending:

- Re-run Claude + agy/Gemini review on the changed runtime-boundary split and DB-only reservation guard before implementation (tracked in `tasks.md`).

## 2026-06-02: Layer 1 cross-AI review (schema + flows + SQLChecker + caching)

Reviewers: codex (gpt-5.5) + agy/Gemini, on the consolidated migration, flows handler, sqlc queries, SQLChecker, OpenAPI flow contract, and the caching strategy. Both converged on the same high-severity issues. All fixed; full Go suite green.

Findings + responses:

- **CRITICAL — reservation route guard (codex).** `WHERE state='accepted'` allowed a second accept after `accepted→completed` and permitted concurrent offers per route (violating sequential-offer). → `ux_reservations_route_active` partial unique on `(org_id, route_request_id) WHERE state IN ('offered','accepted','completed')`; added `ux_reservations_route_attempt` unique `(org_id, route_request_id, attempt)` and `CHECK (attempt > 0)`.
- **CRITICAL — SQLChecker subquery/CTE/INSERT…SELECT bypass (both).** The validator inspected only top-level statements; a nested SELECT from a tenant table without org_id passed. → Rewrote `classify` to recurse (`validateNode`/`validateSelectStmt`); INSERT…SELECT sources validated. Added accept/reject tests (scoped vs unscoped subquery/CTE/INSERT…SELECT).
- **HIGH — continuation typed subject (codex).** `reservation_timeout`/`wrapup_expiry` lacked their own subject; a stale timer could resolve a newer offer. → Added `reservation_id`, `agent_id`, and a per-`kind` CHECK.
- **HIGH — continuation crash-recovery lease (both).** A worker crash after claim stranded rows. → Added `claim_expires_at`/`claimed_by`/`attempt_count`/`last_error`; the due index now covers `claimed` so expired leases are reclaimable.
- **HIGH — flow_versions immutability not enforced (codex).** Cache is "never invalidate" but the DB allowed UPDATE/DELETE. → BEFORE UPDATE/DELETE trigger rejects mutation.
- **MEDIUM — route_requests pinned version (codex).** → CHECK that a started request has `flow_version_id`; `flow_code`/`flow_version_id` set together.
- **MEDIUM — GetFlowByCode resolves soft-deleted (agy).** → `AND enabled = TRUE`.
- **MEDIUM — 409 stale `current` TOCTOU (both).** UpdateFlow reused the pre-flight `stored`. → Always re-probe on conflict.

Deferred (noted, not blockers):
- Substring `LIKE` bypassing the `text_pattern_ops` index (agy) — accepted v0.1 trade-off (D-64); revisit with pg_trgm in a perf pass.
- Speculative `(org_id, agent_id, created_at)` reservations history index (agy) — no v0.2 query needs it; add when agent-history/reporting lands.

Collateral: the Stage-1 `api.VersionConflict` enum rename + flow-method additions had broken `state`/`server` test fixtures — updated to the fully-qualified enum constant + flow stubs.

## 2026-06-02: Layer 1 cross-AI review #2 (runtime contract + node seam + skeleton)

Reviewers: codex (gpt-5.5, 14 findings) + agy/Gemini (7 findings) on the new Layer 1 surface — the OpenAPI runtime contract, `internal/runtime` node seam, `flowrt` stub handlers, `cmd/runtime` scaffold, and the composite wiring. Both converged on the runtime seam + the nil-embed footgun as the highest severity. All accepted findings fixed; full Go suite green.

Fixed:

- **CRITICAL — runtime seam (A1/C1/C13).** `ExecCtx` had no clock and `StepResult.Suspend bool` couldn't yield a wakeup time, so a `wait`/timeout node could be neither deterministic nor able to tell the worker its `due_at`. → Added `ExecCtx.Now() time.Time`; replaced `Suspend bool` with typed `Suspension{ResumeAt, Cursor}` + a typed `Failure *RoutingFailure`; `Validate` now takes `context.Context` and returns `error`; added a `RoutingFailureCode` Go type. (Deeper execution accessors — catalog snapshot, reservation/effect — are additive to `ExecCtx` and deferred to Layer 3 without changing the `Node` interface.)
- **CRITICAL/MEDIUM — nil-embed footgun (A2/C12).** Embedding a nil `*flowrt.Endpoints` works only while stubs ignore the receiver; Layer 3 deref would panic. → All composites now construct non-nil `flowrt.New(...)` (real deps where available; zero deps for the middleware-only server fixture).
- **HIGH — binding contract (A3/C5/C6).** → Added `FlowEntryBinding` schema, `GET /bindings`, publish/rollback now return `FlowPublishResult {version, binding}`, and an optional `expected_current_flow_version_id` guard against concurrent publish/rollback clobber.
- **HIGH — typed taxonomy (A5/C8).** → `RoutingFailureCode` enum referenced by `RouteRequest.failure_code`; `TraceStep.status` is now an enum.
- **HIGH — spine traversal (A6/C2/C3).** → `GET /route-requests` (paginated, status/channel filter), `GET /route-requests/{id}/reservations`, and a `trace_id` link on `RouteRequest`.
- **HIGH — reservation lifecycle (C10).** → `POST /reservations/{id}/complete` (test-double/simulator completion signal).
- **HIGH — cmd/runtime infra (A4).** → Scaffold now runs the cmd/api early-startup sequence (config, OTel, slog TracingHandler, pgx pool) the SKIP-LOCKED worker will need.
- **HIGH — publish race (C4).** → `PublishFlowRequest.version` is required. **LOW (C14):** `info.version` → 0.2.0.
- **HIGH (partial) — simulate determinism (C7).** → Added `virtual_clock_start` + typed `scripted_reservation_outcomes`; remaining determinism inputs (full catalog/state snapshot, tie-break) refine in Layer 6.

Deferred (with rationale):

- **C9 typed graph schema** — kept `graph` opaque at the API. The UI authoring graph (node layout/positions etc.) is a superset of the runtime execution graph; coupling the contract to `node.go`'s execution shape now would either lose UI data or force a mismatch. The server validates structure and returns the typed `FlowValidationResult`; the runtime extracts its typed `Graph` at compile. Revisit if a shared graph contract proves necessary.
- **C11 WrapUp-sweeper ownership** — stays in `cmd/api` until the `cmd/runtime` SKIP-LOCKED worker lands in Layer 5; moving it now would leave a window with no expiry owner.
- **A7 rollback by `to_version_number`** — kept; it's unique per `(org, flow_code)` and more ergonomic for the UI, and C6's `expected_current_flow_version_id` covers the integrity concern.
