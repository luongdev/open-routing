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
