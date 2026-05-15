# Pitfalls Research

**Domain:** Embeddable multi-tenant ACD-like routing platform — v0.1 Catalog Foundation
**Researched:** 2026-05-15
**Confidence:** HIGH (multi-org isolation, MFE, bulk import), MEDIUM (agent state machine — vendor specifics not publicly documented)

---

## 1. Multi-Org Isolation Pitfalls

### Pitfall 1.1: Cache Key Omits `org_id` — Cross-Org Cache Poisoning

**What goes wrong:**
A cache key is built from only the resource identifier (e.g. `agent:{agent_id}`) without the `org_id`. Org A's agent list is cached under `agents:list`. Org B requests the same route seconds later and gets Org A's data served from cache before any DB query fires. The breach is invisible in logs because the HTTP 200 comes from the cache layer, not the DB. The telecom post-mortem pattern: one missing tenant filter exposes 100M customers' records for hours before anyone notices, because no error is raised — the response just contains the wrong org's data.

**Why it happens:**
Caching is often added as a performance afterthought. The developer writes `cache.get("agents:list")` before adding `org_id` scoping. Unit tests pass because tests use a single org context. Integration tests don't assert that Org B cannot see Org A's cached data.

**Warning signs:**
- Cache helpers that accept only a `resource_type` and `resource_id`, not `org_id`
- A `getCacheKey(resourceType, id)` utility function that has no `orgId` parameter
- Cache invalidation on write uses `DELETE agents:*` rather than `DELETE org:{org_id}:agents:*`
- No CI test that creates two orgs, caches data for org 1, then reads as org 2 and asserts empty/error

**How to avoid:**
- Enforce: every cache key MUST begin with `org:{org_id}:`. Lint rule or wrapper function — `cacheKey(orgId, ...segments)` — that makes omitting `org_id` a compile-time error.
- Code pattern: `const key = cacheKey(ctx.orgId, 'agents', 'list')` where `cacheKey` throws if `orgId` is falsy.
- Integration test: `GET /agents` as org_a, warm the cache, then `GET /agents` with `X-Org-ID: org_b` and assert the response does not contain org_a agents.

**Phase to address:** v0.1 Phase 1 (catalog CRUD + org isolation). Must be in place before the first cacheable endpoint ships.

---

### Pitfall 1.2: Background Job Runs in Wrong Org Context

**What goes wrong:**
An async job (e.g., post-import enrichment, audit log flush, cache warm) is enqueued from an HTTP request that carries `org_id`. The job framework uses thread-local or request-scoped context. When the job executes on a worker thread, the `org_id` is absent — either it reverts to null, to a system default, or worst, inherits the previous job's org context still sitting in the thread-local. The job then queries without an org filter and reads or writes across org boundaries. The failure is silent: no exception, no error log, just wrong data landing in the wrong org's dataset.

**Why it happens:**
`@Async` / `CompletableFuture` / message queue consumers do not inherit caller context variables. Developers propagate HTTP context via `ThreadLocal` and forget that worker threads have a separate lifecycle. The bug only surfaces in production where many orgs share a worker pool.

**Warning signs:**
- Job payload schema does not include `org_id` as a mandatory field
- Worker initialization code reads `org_id` from a `ThreadLocal` without a fallback check
- Job handler uses `getCurrentOrgId()` that reads thread-local rather than job payload
- No test that enqueues a job, executes it in a clean worker thread, and asserts correct org scoping

**How to avoid:**
- **Serialize `org_id` into every job payload.** The job payload is the source of truth for org context — never thread-local.
- Worker start: first thing is `OrgContext.set(job.orgId)`. If `job.orgId` is null, reject the job with a dead-letter entry and alert.
- Code pattern:
  ```
  // BAD
  queue.enqueue(ImportJob { importId })
  // GOOD
  queue.enqueue(ImportJob { orgId: ctx.orgId, importId })
  ```
- Test: spin up a worker pool with 4 threads, enqueue jobs for org_a and org_b in alternating order, assert each job's DB writes carry the correct `org_id`.

**Phase to address:** v0.1 Phase 1 (org isolation scaffolding). Any async work introduced in the bulk import feature must follow this pattern from day one.

---

### Pitfall 1.3: Bulk Import Writes Cross-Org Rows

**What goes wrong:**
The bulk import endpoint accepts a JSON/CSV payload and iterates rows, inserting each into the DB. The `org_id` is read from the HTTP header at the start of the request but is not threaded into the row insertion loop. A developer copies a DB insert helper that doesn't require `org_id` because it was originally for internal seeding. The result: every imported agent, skill, or queue row has `org_id = NULL` or inherits the last value that was in scope, which may be a different org if connection pooling reuses a session variable.

**Why it happens:**
Bulk insert helpers are often extracted for reuse and the reuse site assumes the caller will set `org_id`. In a loop over 5,000 rows, the omission touches every row simultaneously, making recovery an expensive re-import or a manual DB patch under customer pressure.

**Warning signs:**
- `insertAgent(agentData)` function signature without an `orgId` parameter
- The import controller extracts `org_id` from the header at the top of the function but the inner loop calls `db.insert(row)` not `db.insert({ ...row, org_id: orgId })`
- No DB-level NOT NULL constraint on `org_id` columns (allowing silent NULL inserts)
- Test fixtures that insert rows without `org_id`

**How to avoid:**
- `org_id` must be NOT NULL with no default in the DB schema. NULL inserts will hard-fail with a DB constraint error immediately.
- All insert/upsert functions require `orgId` as the **first** parameter, not an optional field buried in a data bag.
- Integration test: POST to the bulk import endpoint as org_a, then query `SELECT * FROM agents WHERE org_id IS NULL` and assert zero rows.
- Code review checklist item: "Does every INSERT/UPSERT include `org_id` explicitly?"

**Phase to address:** v0.1 Phase 2 (bulk import). Schema constraint must land in Phase 1 migration.

---

### Pitfall 1.4: Admin/Management Endpoints Leak Across Orgs

**What goes wrong:**
An endpoint like `GET /admin/agents?email=foo@bar.com` is built for internal support tooling during development. It searches across all orgs for debugging convenience. The endpoint is never formally secured or removed. A B2B customer's API integration team discovers the endpoint, passes their own `X-Org-ID` header but the endpoint ignores it, and receives agent records for all orgs. This is OWASP BOLA (Broken Object Level Authorization) applied at the platform level.

**Why it happens:**
Development convenience endpoints skip the org-scoping middleware because the developer is testing cross-org visibility. The endpoint never goes through the same code path as production API routes. In a stub-auth v0.1, the org_id header is trusted but not enforced at every layer.

**Warning signs:**
- Any route that queries without a `WHERE org_id = ?` clause
- Routes under `/admin/` or `/internal/` that don't share the same org-scoping middleware as `/api/`
- A test suite that tests admin endpoints without asserting org isolation
- Search endpoints that accept a bare identifier (email, phone, external_id) and return results across all orgs

**How to avoid:**
- Apply org-scoping middleware to ALL routes including admin, not just the public API. The middleware should be registered at the router root, not per-route.
- Any endpoint that intentionally needs cross-org access (e.g., a future super-admin) must be explicitly marked and protected by a separate elevated-privilege check, distinct from the org header.
- Test: call every management endpoint with two different `X-Org-ID` values and assert results are non-overlapping.

**Phase to address:** v0.1 Phase 1 (org isolation middleware). Must be enforced before any management endpoint is accessible.

---

### Pitfall 1.5: Test Fixtures Bleed Between Orgs in CI

**What goes wrong:**
Test helpers create a "default test org" and populate it with agents, skills, and queues. A second test that creates its own org asserts a count like `expect(agents.length).toBe(3)` but the count is 5 because the default org's fixtures leaked into the query scope. The test passes or fails non-deterministically based on test execution order. Over time, tests are written to match the observed count rather than the correct isolated count, turning CI into a false-confidence machine.

**Why it happens:**
Shared test database with shared fixtures. Tests don't clean up between runs or use sequential transaction rollbacks that don't always commit cleanly. The `org_id` filter on assertions is missing because the developer assumed the test org is isolated by name, not by enforced `org_id` scoping.

**Warning signs:**
- Test setup creates a `DEFAULT_ORG_ID = 1` used across many test files
- Queries in test assertions don't include `org_id` in the WHERE clause
- Test teardown is `DELETE FROM agents` (all orgs) rather than `DELETE FROM agents WHERE org_id = ?`
- Flaky test counts that pass alone but fail in parallel or full CI runs

**How to avoid:**
- Each test (or test suite) gets a randomly-generated `org_id` created fresh and destroyed after. Never share org IDs across tests.
- All test assertion queries explicitly include `AND org_id = $testOrgId`.
- Test teardown deletes only rows with `org_id = $testOrgId`, not truncate-all.
- CI runs tests in parallel with different org IDs to expose sharing bugs early.

**Phase to address:** v0.1 Phase 1 (test infrastructure). Establish the pattern before the first test is written for catalog CRUD.

---

## 2. Agent State Machine Pitfalls

### Pitfall 2.1: Stuck-in-WrapUp — Agent Never Returns to Ready After Browser Close

**What goes wrong:**
An agent is in WrapUp (system-set, post-interaction cleanup period). The agent closes the browser tab or their network drops before the WrapUp timer fires. The server-side state remains `WrapUp` indefinitely because the timer callback is bound to the HTTP session or the WebSocket connection. When the connection dies, the callback is cancelled. The agent is invisible to the router — not Offline (which would be fine), but WrapUp, which the system treats as "about to be Ready." The routing engine holds a reservation slot for this agent indefinitely. This pattern appears in Cisco UCCX forums as "agent stuck not-ready after wrap-up" and is a known operational headache in web-based ACD systems where WrapUp completion depended on client-side confirmation.

**Why it happens:**
WrapUp timer is implemented as a client-side countdown that sends a "WrapUp complete" signal to the server. If the client disappears, no signal arrives. The server has no timeout watchdog because the assumption was "WrapUp always completes through the UI."

**Warning signs:**
- WrapUp timer logic lives in the frontend rather than in a server-side scheduler
- State transition from `WrapUp → Ready` requires an explicit client API call
- No server-side TTL or watchdog on the `WrapUp` state
- Monitoring shows agents in `WrapUp` for >10 minutes with no active interaction

**How to avoid:**
- WrapUp is a **server-owned** state with a server-side TTL. The server sets a deadline: `wrapup_until = now() + config.wrapup_duration`. A background scheduler checks for expired WrapUp records and transitions them to `Ready` (or `NotReady` if the agent's last toggle was NotReady).
- The client may display a countdown as a UX convenience, but the authoritative transition must happen server-side on deadline expiry, independent of client connectivity.
- Transition: `WrapUp → Ready` fires if no prior `NotReady` toggle existed. `WrapUp → NotReady` fires if agent had toggled NotReady before the WrapUp began (intent-preserving).
- Test: start WrapUp, kill the WebSocket connection, wait for `wrapup_duration + epsilon`, assert the state is `Ready` or `NotReady` (not still `WrapUp`).

**Phase to address:** v0.1 Phase 3 (agent state model). Must be designed server-first before any UI is built.

---

### Pitfall 2.2: Race Between System State (Engaged) and User Toggle (NotReady)

**What goes wrong:**
An agent is `Engaged` (handling an interaction). The agent clicks "Go NotReady" in the UI to queue a break after the current interaction ends. At the same moment, the routing engine decides the interaction has ended and sends a `Transition: Engaged → WrapUp` event. Two state transitions arrive at the server almost simultaneously: `SetNotReady(agent, reason)` from the user and `InteractionEnded(interaction)` from the system. If processed out of order or without locking: the NotReady is applied first (state = NotReady), then WrapUp is applied (state = WrapUp), and the user's NotReady intent is silently discarded. The agent comes out of WrapUp as `Ready` and immediately receives another call they thought they had blocked.

**Why it happens:**
State machine does not track "pending intent" separately from "current state." User toggles are applied immediately to the state column rather than being recorded as a deferred intent that survives system transitions.

**Warning signs:**
- State table has a single `status` column with no `pending_status` or `intent` column
- State transitions are applied with a bare `UPDATE agents SET status = ? WHERE id = ?` without optimistic concurrency
- No test that fires a user toggle and a system transition in the same 50ms window and asserts the final state respects intent
- Agent state transitions are processed in a shared async queue without per-agent ordering

**How to avoid:**
- Model "after-interaction intent" as a separate field: `post_interaction_state ENUM('Ready', 'NotReady') DEFAULT 'Ready'`.
- When the system transitions out of `Engaged` (to `WrapUp` or directly to `Ready`), it reads `post_interaction_state` to determine where to land after WrapUp.
- User's "Go NotReady" during `Engaged` writes to `post_interaction_state`, not to `status`. UI reflects: "You'll go NotReady after this interaction."
- All state transitions for a given agent are serialized through a per-agent mutex or a single-writer pattern (one goroutine/actor per agent).

**Phase to address:** v0.1 Phase 3 (agent state model). Design the state table before CRUD, not after.

---

### Pitfall 2.3: Break Reason `routable` Flag Ignored or Inverted

**What goes wrong:**
An org configures a break reason "Training" with `routable: true` (agents on training break can still receive some interaction types). Another break reason "Lunch" has `routable: false`. The routing engine queries `WHERE status = 'Break' AND routable = true` to find routable agents on Break. A bug in the break reason join produces a cartesian product, or the flag is inverted in an ORM serialization (boolean stored as 0/1, deserialized as `routable = !value`), causing `routable: true` agents to be excluded and `routable: false` agents to be included. Agents on lunch receive calls; agents in training sit idle.

**Why it happens:**
The `routable` flag lives on the `break_reasons` table, not on the agent state row. Routing queries must join through `agent_state → break_reason → routable`. The join is non-obvious and gets omitted in quick fixes. Boolean column serialization bugs (1/0 vs true/false) are common across ORM layers.

**Warning signs:**
- Routing queries against agent state do not join `break_reasons`
- `break_reasons.routable` is stored as `INTEGER` (0/1) without an explicit boolean cast in the ORM model
- No test that creates a `routable: false` break reason, puts an agent on that break, and asserts they do not appear in routing candidate queries
- No test for the `routable: true` break reason path

**How to avoid:**
- Schema: `break_reasons.routable BOOLEAN NOT NULL DEFAULT FALSE` — fail loudly on missing values.
- Routing candidate query: `JOIN break_reasons br ON agent_state.break_reason_id = br.id WHERE (agent_state.status != 'Break' OR br.routable = true)`.
- Test matrix: 2x2 — Break+routable, Break+!routable, NotBreak+routable, NotBreak+!routable. Assert routing candidate inclusion for each.
- Code review rule: any query that reads `agent_state.status` for routing purposes must be reviewed against the break_reason join.

**Phase to address:** v0.1 Phase 3 (agent state model + routing candidate query design).

---

### Pitfall 2.4: Invalid Transitions Allowed — Engaged → NotReady Directly

**What goes wrong:**
An API endpoint `PUT /agents/{id}/status` accepts any target state and applies it without validating the transition. A client sends `{ status: "NotReady", reason: "Lunch" }` while the agent is `Engaged`. The system accepts it and marks the agent NotReady mid-call. The routing engine, seeing NotReady, ceases tracking the agent's current interaction. The interaction is stranded — no routing system is monitoring it. When the interaction ends, there's no WrapUp triggered, no state cleanup, and the agent is stuck as `NotReady` with an orphaned interaction record.

**Why it happens:**
CRUD-first thinking: `PUT /agents/{id}/status` is built as a generic update without modeling allowed transitions. The state machine is implicit in documentation, not enforced in code.

**Warning signs:**
- `PUT /agents/{id}/status` accepts any `status` value without checking current state
- No transition table/matrix in code — allowed transitions are in a wiki page or in a developer's head
- Test coverage for state endpoints does not test invalid transitions
- `Engaged → NotReady` returns HTTP 200 without error

**How to avoid:**
- Implement an explicit transition guard: `allowedTransitions = { Ready: [...], NotReady: [...], Break: [...], WrapUp: [...], Engaged: [WrapUp, Offline] }`. Transitions not in the list return HTTP 409 with `{"error": "invalid_transition", "from": "Engaged", "to": "NotReady"}`.
- Agent-toggle endpoints (Ready, NotReady, Break) must check: if current state is system-set (Engaged, WrapUp), the toggle is deferred (write to `post_interaction_state`), not applied immediately.
- Test: PUT NotReady while Engaged → assert HTTP 409. PUT NotReady while Engaged, then system transitions to WrapUp → assert `post_interaction_state` = NotReady and final state after WrapUp = NotReady.

**Phase to address:** v0.1 Phase 3 (agent state model). The transition guard must be in the domain layer, not the HTTP layer.

---

### Pitfall 2.5: Multi-Tab Agent — Two Browsers, Two State Sources

**What goes wrong:**
An agent opens the config UI in two browser tabs (common during onboarding or when switching from laptop to desktop). Tab A sets status to `Ready`. Tab B, using a stale WebSocket or a new one, also initializes with a local state from its own `GET /agents/{id}/status` call 200ms later and renders `Offline` (the persisted state before Tab A's update was written to DB). Tab B sends `SetReady` again — which is fine, it's idempotent. But Tab B then loses focus and remains open. The agent goes on a call (Engaged). Tab B still shows `Ready` in its local state. The agent uses Tab B to click "Go NotReady" because they think they're Ready. Tab B sends `SetNotReady` to the server. The server accepts it and writes `post_interaction_state: NotReady`. The interaction ends, WrapUp fires, and the agent lands in NotReady — unexpectedly, from their perspective on Tab A.

**Why it happens:**
Each browser tab maintains local UI state without a server-push subscription. Tabs diverge silently. There's no enforcement of "only one active session" per agent in a catalog-only v0.1 context.

**Warning signs:**
- Agent state UI is initialized from a one-shot REST GET with no subsequent subscription for state changes
- No `last_seen_session_id` or similar mechanism to detect stale sessions
- No warning to the agent when multiple tabs are detected
- WebSocket events from state changes are not broadcast back to all sessions for the same agent

**How to avoid:**
- For v0.1 (stub auth, no real-time enforcement): document the risk and add a UI warning if `localStorage` or a session cookie detects multiple tabs via the `BroadcastChannel` API.
- Server-side: every state-change response should include a monotonic `state_version`. The UI rejects or re-fetches if the state it holds is older than the version in a push event.
- Future: enforce single-active-session per agent identity, rejecting or invalidating older sessions on new login.

**Phase to address:** v0.1 Phase 3 (agent state model). The `state_version` field must be in the state schema from day one; retrofitting it after relationships are built is expensive.

---

## 3. MFE Embedding Pitfalls

### Pitfall 3.1: Module Federation Version Skew — Duplicate React, Broken Hooks

**What goes wrong:**
The host application bundles React 18.2.0. The Open Routing MFE remote bundles React 18.3.0. Both declare `react` as a shared singleton in Module Federation config. The version range `^18` in one and `18.x` in the other resolves differently across webpack versions, resulting in two separate React instances being loaded. `useState` and `useEffect` hooks fail with "hooks called outside of a React tree" or "invalid hook call" runtime errors — appearing only in production where the host and remote are built independently, not in local development where both share the same build output.

**Why it happens:**
Shared singleton configuration is subtle. The `requiredVersion` field must be set precisely. If one side omits `singleton: true` or the version ranges don't overlap after semver resolution, webpack loads two copies. This is a documented webpack Module Federation bug that appeared repeatedly in real production deployments.

**Warning signs:**
- Host and MFE have different `react` or `react-dom` versions in their `package.json`
- The shared config in `webpack.config.js` / `rspack.config.js` does not specify `singleton: true` and `requiredVersion`
- Local dev works fine; staging or production breaks with hooks errors
- `window.__REACT_DEVTOOLS_GLOBAL_HOOK__._renderers` shows more than one renderer

**How to avoid:**
- In `ModuleFederationPlugin` config for both host and remote: `shared: { react: { singleton: true, requiredVersion: deps.react }, 'react-dom': { singleton: true, requiredVersion: deps['react-dom'] } }`.
- Pin the exact same React version in both host and remote `package.json` for v0.1.
- CI check: a script that compares `package.json` `react` and `react-dom` versions between the host repo and the MFE repo and fails if they diverge.
- Use the async bootstrap pattern (`import('./bootstrap')` as the main entry) to ensure shared dependencies resolve before any component code runs.

**Phase to address:** v0.1 Phase 4 (MFE embed setup). Must be validated end-to-end in a real host-remote build before claiming "embed works."

---

### Pitfall 3.2: Module Federation Eager-Load Deadlock

**What goes wrong:**
Libraries marked `eager: true` in the shared config are expected to load synchronously. But when a remote module is imported dynamically, webpack creates a pending promise for the remote's container. If the eagerly-loaded library is also shared with the remote and the remote's promise hasn't resolved yet, the eager lib's initialization hangs — it waits for the promise that depends on it. The result is a white screen with no error; the module's `.get()` function remains pending forever. This is a documented webpack bug (issue #15829) that affects builds where host and remote share overlapping `eager` dependencies.

**Why it happens:**
Circular dependency between eager loading (synchronous requirement) and remote module resolution (asynchronous promise). The bug is in webpack's Module Federation bootstrap timing, not fixable at the application level without workarounds.

**Warning signs:**
- Both host and remote declare the same library with `eager: true` in shared config
- Application renders nothing on first load in production (white screen) but works in dev
- Network tab shows no failed requests — the JS loads but nothing executes
- Console shows no errors, only silence

**How to avoid:**
- Do not use `eager: true` for shared dependencies in v0.1. Use the async bootstrap pattern instead: wrap the entire application entry in a dynamic import (`import('./bootstrap').then(...)`) and let webpack manage chunk loading order.
- The bootstrap file is the `index.js` that calls `ReactDOM.render(...)`. The entry point is just `import('./bootstrap')`.
- Test: load the MFE in a host container in a headless browser (Playwright/Puppeteer) and assert the root component renders within 3 seconds. This catches white-screen deadlocks that unit tests miss.

**Phase to address:** v0.1 Phase 4 (MFE embed setup). Catch before demo, not after.

---

### Pitfall 3.3: `org_id` Context Lost on Navigation or Full Page Reload

**What goes wrong:**
The host application passes `org_id` to the MFE via a prop or a custom event at mount time. The MFE stores `org_id` in React state (not persistent storage). The user navigates away and back using the browser's back/forward buttons, triggering a full component unmount/remount. The MFE remounts, but the host does not re-send the `org_id` because it assumes "mount" is a one-time event. The MFE renders in an empty org context, hitting `GET /agents` without an `X-Org-ID` header. The server returns 400 or an empty catalog. The user sees a broken config panel.

**Why it happens:**
The interface contract between host and MFE is informal — "the host passes org_id on mount" — but not hardened against remount, navigation, or SPA route changes. Hosts built on different frameworks (Angular, vanilla JS) have different lifecycle events that may not trigger the MFE's mount props again.

**Warning signs:**
- The MFE reads `org_id` from a prop that is only passed once at initial render
- The MFE does not emit an "I need org context" event when it has no `org_id`
- No test that unmounts and remounts the MFE and asserts the org context is re-established
- Integration tests only test initial load, not navigation round-trips

**How to avoid:**
- The MFE must request its own context on every mount: `window.dispatchEvent(new CustomEvent('routing:request-context'))` and listen for a `routing:context` event from the host.
- If no context is received within 500ms of mount, the MFE renders an explicit "Not configured — host must provide org context" error state, not a broken or empty UI.
- The MFE should also read from a fallback: `window.__ROUTING_CONFIG__` (set by host) or `sessionStorage.getItem('org_id')` (set by the MFE on first successful context receipt).
- Test: mount MFE, receive context, unmount, remount without re-sending context, assert the MFE displays the error state (not a broken/empty catalog).

**Phase to address:** v0.1 Phase 4 (MFE embed + host integration contract).

---

### Pitfall 3.4: Auth Token Expiry Inside MFE with No Refresh Path

**What goes wrong:**
The host passes a JWT or session token to the MFE at mount. The token expires after 1 hour (standard OAuth access token lifetime). The user is actively working in the MFE catalog UI. At 60 minutes, the next API call returns HTTP 401. The MFE's error handler shows "Unauthorized" or a blank error state. The user has no way to refresh because the MFE has no login flow — authentication belongs to the host. In stub-auth v0.1 this is deferred, but the interface contract must be designed now or retrofitting it is expensive.

**Why it happens:**
Auth is treated as a one-time setup concern. The token refresh lifecycle belongs to the host but the MFE makes the actual API calls. The responsibility boundary is unclear: who detects 401, who triggers refresh, who notifies whom?

**Warning signs:**
- The MFE stores the auth token in a module-level variable with no expiry tracking
- 401 responses are caught by a global error handler that shows "Please log in" with a login button (impossible inside an embedded panel)
- No event is emitted when the MFE receives a 401 for the host to intercept
- Token expiry is not tested (tests use tokens that never expire)

**How to avoid:**
- Define the contract now: the MFE emits `routing:auth-expired` when it receives a 401. The host listens for this event, refreshes the token, and re-sends it via `routing:set-token`. The MFE retries the failed request.
- In v0.1 with stub auth: implement the event protocol with a no-op host handler. The contract is established but the host handler is a stub that logs a warning.
- Token storage: the MFE should receive tokens via events, not store them in module globals. The host is the token owner.
- Test: simulate a 401 response from the mock server; assert the MFE emits `routing:auth-expired` and does not show a login screen.

**Phase to address:** v0.1 Phase 4 (MFE embed). Design the contract in v0.1 even though full auth is deferred.

---

### Pitfall 3.5: CSS / Style Bleeding Between Host and MFE

**What goes wrong:**
The host application uses a global CSS reset that sets `* { box-sizing: border-box; margin: 0; padding: 0; }` or aggressive element selectors like `button { background: transparent; border: none; }`. These rules leak into the MFE's rendered DOM because the MFE is not in a Shadow DOM and its styles are injected into the shared document `<head>`. The MFE's buttons lose their borders, its grid layout breaks, and its form inputs are unstyled. The inverse also occurs: the MFE's styles bleed into the host's components if the MFE uses utility classes (e.g., Tailwind without a prefix) that conflict with host class names.

**Why it happens:**
CSS in the browser global scope has no module boundary. `style-loader` injects all CSS into `<head>` regardless of where the component is rendered. Hosts with aggressive CSS resets are the default in enterprise design systems (Material UI, Bootstrap, Ant Design all use resets).

**Warning signs:**
- The MFE uses Tailwind without a configured prefix (`prefix: 'or-'` in `tailwind.config.js`)
- The MFE uses `style-loader` without a Shadow DOM container
- Integration test renders the MFE inside a host that has a CSS reset and does not visually test button/input rendering
- MFE looks correct in standalone dev but broken inside the host

**How to avoid:**
- Tailwind: configure `prefix: 'or-'` (Open Routing prefix) so all utility classes are `or-flex`, `or-text-sm`, etc. Zero collision with host utilities.
- For component-level styles: use CSS Modules or `@layer` scoping. Never use bare element selectors in MFE stylesheets.
- Shadow DOM: for maximum isolation, render the MFE root into a `<div>` inside a Shadow DOM. Custom element wrapping handles this. This prevents both bleed-in and bleed-out.
- Defensive baseline: wrap the MFE's root container in a `div` with `all: revert` to reset host styles, then apply MFE-specific styles from a known baseline.
- Integration test: render the MFE inside a host with a known aggressive CSS reset and screenshot-test that primary UI elements (buttons, inputs, tables) are visually correct.

**Phase to address:** v0.1 Phase 4 (MFE embed). Catch in the first integration test against a real host shell, not in unit tests.

---

### Pitfall 3.6: Cross-Origin Issues When Host and MFE Run on Different Domains

**What goes wrong:**
In production, the host runs on `app.customer.com` and the MFE is served from `cdn.openrouting.io`. The MFE's webpack build uses the public URL from its own origin. Module Federation's remote entry URL is hardcoded to `https://cdn.openrouting.io/remoteEntry.js`. The browser loads this cross-origin script — which works. But the MFE then makes API calls to `https://api.openrouting.io` from within the host origin context. CORS headers on the API must allow the host origin. If the CORS policy only allows `openrouting.io` origins, the API calls from `app.customer.com` are blocked by the browser with no useful error to the end user.

**Why it happens:**
CORS is configured during initial development where host and MFE share the same origin. Cross-origin production deployment exposes the gap. The MFE developer does not know which host origins to allow; the list is open-ended for an embeddable product.

**Warning signs:**
- API CORS config hardcodes a list of allowed origins rather than validating against a DB-stored list of registered host domains
- CORS headers are added to the backend after the first cross-origin test failure, not from the start
- No integration test that runs the MFE from a different origin than the API

**How to avoid:**
- API must accept `Origin` from any registered host domain. Store `allowed_origins` per org in the DB or config. On request, look up the `Origin` header against the registered list and set `Access-Control-Allow-Origin` dynamically.
- For v0.1 (stub auth): accept all origins with `Access-Control-Allow-Origin: *` and document that this is a stub to be replaced with per-org origin validation in the auth milestone.
- Set `Access-Control-Allow-Headers: X-Org-ID, Content-Type, Authorization` explicitly — the `X-Org-ID` custom header triggers CORS preflight.
- Test: make API calls from a test page served on `localhost:4000` to an API on `localhost:3000` and assert the CORS headers are correct.

**Phase to address:** v0.1 Phase 4 (MFE embed) and Phase 1 (API setup). CORS for `X-Org-ID` must be configured from the first API endpoint.

---

## 4. Catalog Model Pitfalls

### Pitfall 4.1: Soft Delete Ambiguity — Referenced Entity Gets Deleted Silently

**What goes wrong:**
An agent is soft-deleted (`deleted_at IS NOT NULL`). The agent has active assignments to three queues and two skills. The soft-delete operation succeeds without cascading. Queries for queue members still return the soft-deleted agent because the join doesn't filter on `deleted_at`. The routing engine may offer calls to an agent who is "deleted" from the catalog perspective but still present in queue membership tables. Alternatively, if the join does filter `deleted_at`, the agent silently drops out of queue assignments, reducing queue capacity without any audit event.

**Why it happens:**
Soft delete is added to the agent table but the foreign key relationships (agent_queue_memberships, agent_skill_assignments) are not updated to be consistent. There's no decision about whether soft-deleting an agent should cascade to their assignments or block the deletion.

**Warning signs:**
- `deleted_at` column exists on `agents` but not on `agent_queue_memberships` or `agent_skills`
- Queries for queue members do not filter `WHERE a.deleted_at IS NULL`
- No DB-level enforcement: soft-deleted agents can be referenced in new assignments
- The soft-delete endpoint returns 200 without checking for active queue memberships

**How to avoid:**
- Define the delete policy explicitly in the schema documentation: "Soft-deleting an agent MUST first remove all queue memberships and skill assignments (with audit events for each). If the agent is Engaged, the delete is rejected with HTTP 409."
- Enforce via application logic: the delete operation is a multi-step transaction: deassign queues, deassign skills, soft-delete agent, emit audit events.
- DB: add a trigger or application check that prevents inserting into `agent_queue_memberships` or `agent_skills` where `agent_id` references a soft-deleted agent.
- Test: soft-delete an agent, then query all queue memberships and assert the agent does not appear. Query routing candidates and assert the agent is absent.

**Phase to address:** v0.1 Phase 2 (catalog CRUD). Must be specified in the schema before the first migration is written.

---

### Pitfall 4.2: Skill Proficiency Model Overload

**What goes wrong:**
The initial schema defines `skill_assignments.proficiency INTEGER CHECK (1..10)`. By v0.2, some orgs want to use named tiers ("Beginner", "Intermediate", "Expert"). A migration adds `proficiency_label VARCHAR`. Now some rows have `proficiency = 7, proficiency_label = NULL` and others have `proficiency = NULL, proficiency_label = 'Expert'`. Routing logic queries `WHERE proficiency >= 6` — it silently excludes all "Expert" agents whose numeric proficiency is NULL. Routers that use label-based matching silently exclude all numeric-only agents. The model is split-brained.

**Why it happens:**
The proficiency model is designed to be "flexible" without committing to a schema contract. Two features are bolted together into one column family without a single canonical representation.

**Warning signs:**
- `proficiency` is defined as a raw integer with no enum/reference table
- No `skill_tiers` reference table exists
- Application logic compares proficiency with both numeric operators (`>=`) and string equality (`= 'Expert'`) in different code paths
- Tests use `proficiency = 5` in some places and `proficiency_label = 'Advanced'` in others

**How to avoid:**
- Pick one representation and stick with it for v0.1. Recommendation: integer 1–10. If named tiers are needed later, they map to integer ranges via a config table (`skill_tier_ranges`) rather than by changing the column type.
- Define `proficiency SMALLINT NOT NULL CHECK (proficiency BETWEEN 1 AND 10)` with a NOT NULL constraint. No nullable proficiency in v0.1.
- Document the decision: "Proficiency is always an integer 1–10 in v0.1. Named tiers are a future org-config feature that maps names to integer ranges. Routing always uses integers."

**Phase to address:** v0.1 Phase 2 (catalog schema). Lock the model before CRUD is built.

---

### Pitfall 4.3: Mass Updates Without Optimistic Locking (Concurrent Edit Overwrite)

**What goes wrong:**
Two admin users in the same org both open the agent list at the same time (10:00:00). Admin A edits agent "Bob" and submits at 10:00:45. Admin B, who had the same snapshot from 10:00:00, edits the same agent "Bob" and submits at 10:01:00. Admin B's PUT request reads the current version (without a version check), applies their delta, and overwrites Admin A's changes silently. Admin A's update — which set Bob's skills — is lost with no error, no warning, and no audit trail indicating a conflict.

**Why it happens:**
REST PUT endpoints are built with "last write wins" semantics. No `version` or `updated_at` field is returned in GET responses or required in PUT requests. The ORM issues a blind UPDATE without a version check.

**Warning signs:**
- GET `/agents/{id}` response does not include a `version` or `etag` field
- PUT `/agents/{id}` does not accept or require a `version` field
- `UPDATE agents SET ... WHERE id = ?` does not include `AND version = ?`
- No test that simulates two concurrent edits and asserts the second one is rejected

**How to avoid:**
- Add `version BIGINT NOT NULL DEFAULT 0` to all catalog entity tables.
- GET returns `"version": 3`. PUT requires `"version": 3` in the request body. The SQL: `UPDATE agents SET ..., version = version + 1 WHERE id = ? AND version = ? RETURNING version`. If `0 rows affected`, return HTTP 409 `{"error": "conflict", "message": "Record was modified by another user. Reload and retry."}`.
- For bulk import: mass upsert operations skip per-row version checks (bulk imports are seeding operations, not concurrent edits). Document this explicitly.

**Phase to address:** v0.1 Phase 2 (catalog CRUD). Version column in the first schema migration.

---

### Pitfall 4.4: `external_id` Collisions During Bulk Import

**What goes wrong:**
Org A exports their agents from their CRM with `external_id` = the CRM's primary key (integers: 1001, 1002, 1003). Org B does the same from a different CRM instance, also with `external_id` 1001, 1002, 1003. The `unique` constraint on `external_id` is defined without `org_id` in the index. Org A's import succeeds. Org B's import fails with a "duplicate key" error on row 1 — even though their `external_id` 1001 is in a completely different org. The import fails entirely if `ALL_OR_NOTHING` mode is selected. Alternatively, if the unique index is per-org but the upsert logic looks up by `external_id` alone without `org_id`, it finds Org A's record and silently updates Org A's agent with Org B's data.

**Why it happens:**
`external_id` is treated as a globally unique identifier. The unique index is `UNIQUE (external_id)` rather than `UNIQUE (org_id, external_id)`. The upsert lookup is `WHERE external_id = ?` without `AND org_id = ?`.

**Warning signs:**
- Database migration has `CREATE UNIQUE INDEX ON agents (external_id)` without `org_id`
- Upsert SQL: `INSERT INTO agents ... ON CONFLICT (external_id) DO UPDATE ...` without `org_id` in the conflict target
- Test uses only a single org for bulk import testing
- Multi-org import test does not exist

**How to avoid:**
- Schema: `UNIQUE (org_id, external_id)` — this is the correct and only valid uniqueness scope.
- Upsert: `INSERT INTO agents (...) ON CONFLICT (org_id, external_id) DO UPDATE SET ...`.
- Test: import the same `external_id` values for two different orgs and assert both succeed with separate records.

**Phase to address:** v0.1 Phase 2 (schema) and Phase 3 (bulk import). Schema must be correct before the import feature is built.

---

### Pitfall 4.5: Queue / Channel / Adapter Cyclic Reference in Catalog

**What goes wrong:**
The catalog schema allows: a `queue` references a `channel`. A `channel` references an `adapter`. An `adapter` references a `queue` for overflow routing. The cycle is `queue → channel → adapter → queue`. Validation logic that traverses these references enters an infinite loop. Serialization for the flow compiler (future milestone) hits a stack overflow when resolving catalog dependencies. In v0.1 where these are just registry rows, the cycle is silent until the validator or compiler is built.

**Why it happens:**
Each reference is added incrementally as features are designed. No one maps the full entity relationship graph before building CRUD. The cycle becomes apparent only when traversal code is written.

**Warning signs:**
- No entity relationship diagram exists for the catalog schema
- Foreign key graph has not been checked for cycles
- `adapters` table has a `queue_id` column (suggesting back-reference to queue)
- No validation that prevents circular references at insert/update time

**How to avoid:**
- Draw the entity reference graph before writing any schema. For v0.1 adapter entity: it is a registry row only (no SDK, no execution). The adapter must NOT reference a queue — that would create a cycle. Document: "Adapter is a leaf node in the catalog graph in v0.1."
- If overflow queue needs to be referenced, it must be on the `flow` or `routing_strategy` entity, not on the adapter catalog row.
- Future: add a graph cycle validator to the catalog publish step.

**Phase to address:** v0.1 Phase 2 (catalog schema design). Block cycle-introducing FKs in the migration review.

---

## 5. Bulk Import Pitfalls

### Pitfall 5.1: Encoding / BOM / CRLF Surprises

**What goes wrong:**
A customer exports their agent list from Microsoft Excel on Windows. Excel saves as "UTF-8 with BOM" and uses CRLF line endings. The import parser reads the first column name as `\xEF\xBB\xBFname` (the BOM bytes prepended to "name"). The parser fails to find the `name` column and errors with "Required column 'name' not found." Alternatively, the CRLF parser only strips `\n` as the row terminator, leaving `\r` as the last character of every field value. The agent name "Alice\r" does not match the validation pattern for agent names and fails. These are silent, non-obvious errors that confuse non-technical users who believe their CSV is "just a spreadsheet."

**Why it happens:**
CSV is not a standard. Python's `csv` module and Node.js's common parsers handle BOM and CRLF differently. The parser is chosen during development using developer-created test files (created on macOS or Linux, UTF-8, LF), which never expose these Windows-Excel gotchas.

**Warning signs:**
- The test CSV files in the repo are created on macOS or Linux
- The import parser is not configured with `utf-8-sig` (BOM-stripping) mode or equivalent
- No test uses an actual Excel-exported file with BOM+CRLF
- Error messages from encoding failures reference internal parser details, not user-friendly column names

**How to avoid:**
- Use a CSV parser configured with: (1) BOM stripping (`utf-8-sig` or `strip_bom: true`), (2) CRLF normalization (accept both `\r\n` and `\n`), (3) UTF-8 as the only accepted encoding (reject and report cleanly if charset detection fails).
- Test suite must include a file named `windows-excel.csv` that is a real Excel-exported file (UTF-8 BOM, CRLF). This file lives in `testdata/` and is never modified.
- User-facing error messages map column-not-found errors to: "The file appears to have encoding issues. Try saving as 'CSV UTF-8 (no BOM)' from Excel."

**Phase to address:** v0.1 Phase 2 (bulk import). Test file must be created before parser code is written.

---

### Pitfall 5.2: Partial-Success Semantics Not Communicated to User

**What goes wrong:**
The import endpoint processes 1,000 rows. 950 succeed. 50 fail validation (invalid email, duplicate skill name, etc.). The server returns HTTP 200 with `{ "imported": 950, "failed": 50 }`. The user sees the number but has no way to know which rows failed, why they failed, or how to fix them. They re-export from their CRM, manually scan 1,000 rows looking for the 50 that broke, and contact support. Support cannot help without the import session details, which are not stored.

**Why it happens:**
The endpoint is built to return aggregate counts as the "MVP" for communication. Per-row error details are considered a "nice to have" for v2. The import session is not persisted, so there's no way to retrieve errors after the response is returned.

**Warning signs:**
- Import response body contains only `{ "total": N, "succeeded": M, "failed": K }` with no row-level detail
- Import sessions are not stored in the DB
- No endpoint like `GET /imports/{id}/errors` exists
- The user documentation for bulk import says "check the returned count"

**How to avoid:**
- Import sessions must be persisted. On upload, create an `import_sessions` record. On completion, record per-row results with the row index, the original row data (or a key field), and the error message.
- Return: `{ "importId": "...", "succeeded": 950, "failed": 50, "errors_url": "/imports/{id}/errors" }`.
- The errors endpoint returns a downloadable report (JSON or CSV) with one row per failure, including the row number, the problematic field, and the validation message.
- HTTP status: use 207 Multi-Status when there are partial failures, not 200.

**Phase to address:** v0.1 Phase 2 (bulk import). `import_sessions` table in the schema migration; error storage in the first import implementation.

---

### Pitfall 5.3: Large File Upload Times Out at Proxy / CDN Layer

**What goes wrong:**
A customer tries to import 50,000 agents (a realistic catalog for a mid-size contact center). The CSV is ~8MB. The reverse proxy (Nginx, Cloudflare, AWS ALB) has a default body size limit of 1MB or a 30-second read timeout. The upload is cut off mid-stream. The server receives a partial file and either fails with a parser error (unexpected EOF) or — worse — silently processes the truncated file, importing only the agents in the first portion of the file. The customer believes the full import succeeded.

**Why it happens:**
Proxy limits are configuration defaults that developers rarely override in development (where there's no proxy). The limits are discovered only when a customer tries a large real-world import.

**Warning signs:**
- Nginx config does not explicitly set `client_max_body_size`
- ALB or Cloudflare timeout settings have not been reviewed for the import endpoint
- The max import file size is not documented or enforced by the API
- The API reads the file body with a streaming parser but does not validate total bytes received against a `Content-Length` header

**How to avoid:**
- Set an explicit and documented maximum file size: `50MB` for v0.1. Enforce at the API layer before parsing: `if content_length > MAX_BYTES: return 413`.
- Configure the reverse proxy explicitly: Nginx `client_max_body_size 55m; proxy_read_timeout 300s;` for the import endpoint.
- For very large files: accept the file upload as a multipart upload, store to S3/object storage, return a job ID, and process asynchronously. The synchronous path is for files under 5MB; larger files get async processing.
- After async processing, the `import_sessions` table records the outcome and the user polls `GET /imports/{id}` for status.
- Document: "Maximum synchronous import: 5MB (~5,000 rows). Larger imports are processed asynchronously; allow up to 5 minutes for completion."

**Phase to address:** v0.1 Phase 2 (bulk import). Proxy configuration must be tested with a real large file before the feature is marked done.

---

### Pitfall 5.4: Schema Drift Between v0.1 and v0.2 Breaks Saved Imports

**What goes wrong:**
A customer saves their import CSV as a template for monthly catalog refreshes. In v0.1, the CSV format has columns `name, email, skill_ids, queue_ids`. In v0.2, the schema evolves: `skill_ids` is replaced by `skill_names` (to support name-based lookup), and `queue_ids` is split into `primary_queue` and `overflow_queue`. The customer's saved template fails silently: `skill_ids` is an unknown column (ignored), so agents import with no skills. The customer doesn't notice until routing failures surface a week later.

**Why it happens:**
The CSV format is not versioned. There's no schema version header in the file. The parser silently ignores unknown columns instead of warning the user. No deprecation notice is given when column names change.

**Warning signs:**
- CSV import format has no `schema_version` field or column
- The parser silently ignores unrecognized column names
- No migration strategy exists for saved import templates when schema changes
- The import format is documented informally, not as a versioned contract

**How to avoid:**
- Version the import format from v0.1: include a `schema_version` header or accept it as a query param (`?schema_version=v0.1`).
- The parser emits a WARNING (not an error) for unrecognized columns — include the column name in the import report: "Column 'skill_ids' not recognized in schema v0.2. Use 'skill_names'. Rows with this column are imported without skills."
- For v0.2 schema changes: support reading v0.1 column names with a compatibility translation layer for at least one release cycle.
- Publish a changelog for the import format as a first-class document, not a footnote in the API changelog.

**Phase to address:** v0.1 Phase 2 (bulk import). Versioning must be in the first import implementation. Retrofitting versioning onto an unversioned format requires all existing customers to update templates simultaneously.

---

## 6. Domain Scope Anti-Patterns (Open Routing Boundary Violations)

These are explicitly called out because PROJECT.md's "Out of Scope" list is the guard rail. Each item below maps to a real drift pattern seen in ACD platform development.

### Anti-Pattern 6.1: Drifting into Agent Desktop Territory

**What goes wrong:**
The catalog UI starts showing agent "availability" as a live green/yellow/red dot. Then a request comes to show the agent's active interaction count. Then someone adds a "Force Ready" button for supervisors. Suddenly the catalog config panel is a lite agent desktop — real-time presence, interaction counts, supervisor controls. The embed contract becomes heavy (real-time WebSocket required, media state required). Customers who embedded the catalog panel now have a dependency on a full real-time infrastructure.

**PROJECT.md says:** "Agent desktop in v1 - agent work surfaces belong to host products or downstream systems."

**Warning signs:**
- A `presence` field or `active_interactions_count` appears in the catalog API response
- The catalog UI renders a colored status dot that updates in real-time
- A "Force State Change" action appears in the catalog UI for supervisor use
- The MFE requires a WebSocket connection at mount time

**How to avoid:**
- The catalog API returns the **configured** state model (status labels, break reasons with routable flag) — not the **live** agent state.
- Live agent state belongs to the runtime/monitoring plane (future milestone), not the catalog.
- Code review gate: any PR that adds a real-time data feed to the catalog endpoints is a scope violation. Reject.
- If a stakeholder requests live presence in the catalog UI: document it as a feature request for the monitoring plane, link to the PROJECT.md boundary, defer.

**Phase to address:** v0.1 Phase 3 and 4. Establish the boundary in the API contract and the MFE's data flow before any stakeholder sees a live demo with colored dots.

---

### Anti-Pattern 6.2: Embedding Media / Call Control Logic in the State Machine

**What goes wrong:**
The agent state machine is designed to handle `Voice`, `Chat`, `Email` as sub-states of `Engaged`. A developer adds logic: "When Engaged(Voice), automatically set routable = false for all channels." Then "When Engaged(Voice), send a SIP HOLD command to the adapter." The state machine now directly calls the adapter SDK and knows about SIP commands. The routing engine has become a voice bridge controller. Future adapter changes (switching from SIP to WebRTC) require changes in the state machine.

**PROJECT.md says:** "Real media/call control in v1 - Open Routing bridges to channel systems but does not hold calls, streams, chats, or email sessions itself." and "Keep media holding, call control, chat execution, email execution, and channel-specific session ownership outside core."

**Warning signs:**
- State machine code imports or calls adapter-specific APIs directly
- `Engaged` state sub-states have channel-specific business logic (SIP, WebRTC, SMTP references)
- Channel-specific conditions appear in routing queries ("if channel == Voice and state == Engaged, then...")
- The state machine emits adapter commands directly rather than emitting routing events for adapters to react to

**How to avoid:**
- The state machine emits neutral routing events: `AgentBecameEngaged(agentId, interactionId, channelType)`. Adapters react to these events and issue their own channel-specific commands.
- The routing core knows `channelType` as a label (Voice, Chat, Email) for routing decisions only. It does not contain channel-specific logic.
- Test: the state machine test suite must not import any adapter module. If it does, it's a boundary violation.
- Code review gate: any import of a channel-specific library (SIP, WebRTC, SMTP) inside the routing core package is an automatic reject.

**Phase to address:** v0.1 Phase 3 (agent state model). The event emission pattern must be established before any channel-specific sub-states are added.

---

### Anti-Pattern 6.3: Coupling Catalog Persistence to a Specific Channel Adapter

**What goes wrong:**
The `adapters` catalog table is designed with FreeSWITCH-specific columns: `freeswitch_host`, `freeswitch_port`, `esl_password`. A second adapter type (Twilio, LiveKit) cannot be stored because the schema is FreeSWITCH-shaped. The catalog cannot serve as a generic adapter registry. When v0.2 adds a second adapter type, a schema migration is required that breaks existing adapter rows.

**PROJECT.md says:** "Adapter entity is a registry row only — no SDK contract, no execution in v0.1."

**Warning signs:**
- `adapters` table has columns named after a specific vendor or protocol
- The adapter entity has a hard-coded `type` enum with only one value
- Adapter configuration is stored as fixed columns rather than as a JSONB config blob
- There's no `adapter_type` discriminator column

**How to avoid:**
- Schema for v0.1: `adapters(id, org_id, name, adapter_type VARCHAR, config JSONB, created_at, updated_at, deleted_at)`. The `config` JSONB is adapter-type-specific. The catalog stores what the adapter *is* (type + config shape), not how to run it.
- `adapter_type` is a string enum validated against a registered list (`voice_mock`, `chat_mock`, `email_mock` for v0.1). Adding v0.2 types requires only adding to the enum validation, not a schema migration.
- Test: create two adapters with different `adapter_type` values and different `config` shapes. Assert both store and retrieve correctly.

**Phase to address:** v0.1 Phase 2 (catalog schema). JSONB config is the decision to make before the first adapter migration is written.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip `version` column on catalog entities | Simpler CRUD | Concurrent edits silently overwrite; impossible to retrofit safely after data exists | Never — add it in the first migration |
| Single org in all unit tests | Faster test setup | Multi-org bugs pass CI; found in production by customers | Never for isolation-critical paths |
| WrapUp timer in client only | Simpler server | Agents stuck in WrapUp on browser close; routing starvation | Never — server must own the timer |
| Hard-code CORS to `*` | Zero config | Must be tightened before GA; often forgotten and ships to production | v0.1 stub only — document the gap |
| Omit `schema_version` from CSV import | Simpler parser | Schema changes break all saved templates silently | Never — version from day one |
| Store adapter config as fixed columns | Easier queries | Breaks when a second adapter type is added; requires migration | Never — use JSONB from day one |
| Eager: true in Module Federation | One less async chunk | White-screen deadlock in production when host and remote share deps | Never — use async bootstrap |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Redis cache | Key missing `org_id` prefix | `cacheKey(orgId, resourceType, id)` wrapper that throws if orgId is falsy |
| PostgreSQL bulk insert | Omitting `org_id` from INSERT statement | `org_id NOT NULL` schema constraint that hard-fails on omission |
| Module Federation host | Passing `org_id` only at mount time | MFE emits `routing:request-context` on every mount; host re-sends on every mount |
| WebSocket disconnect | WrapUp timer bound to WebSocket lifecycle | Server-side deadline stored in DB; background scheduler fires transitions |
| CSV parser (Node.js) | Default parser fails on BOM+CRLF | `csv-parse` with `bom: true` option and `record_delimiter: ['\n', '\r\n']` |
| Reverse proxy (Nginx) | Default 1MB body limit rejects large imports | Explicit `client_max_body_size 55m` on the import route |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Routing candidate query without index on `(org_id, status)` | p95 routing decision > 50ms target | Compound index `(org_id, status, routable)` on agent state table | ~1,000 agents per org |
| Bulk import in a single DB transaction | Import times out at the proxy layer | Batch in groups of 500; each batch is its own transaction with partial-success tracking | ~5,000 rows |
| `import_sessions` table with full row data for every import row | Storage blows up with heavy import usage | Store only the row index + error message for failed rows; successful rows not stored | ~10 imports/day at 10K rows each |
| Soft-delete with no index on `deleted_at` | Slow catalog queries as soft-deleted rows accumulate | Partial index: `CREATE INDEX ... WHERE deleted_at IS NULL` | ~100K soft-deleted rows |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| `org_id` read from request body instead of trusted header | Attacker sets any `org_id` in the body and reads other orgs' data | `org_id` comes ONLY from the trusted `X-Org-ID` header; body `org_id` is ignored and logged as a warning |
| `PUT /agents/{id}/status` accepts any status without transition validation | Agent forced to invalid states (e.g. Engaged without an interaction) | Explicit transition guard returns HTTP 409 for disallowed transitions |
| Admin endpoints not behind org-scoping middleware | Any org's admin user reads cross-org data | Org-scoping middleware applied at router root, not per-route |
| `external_id` unique constraint without `org_id` | Org B's import fails due to Org A's `external_id`; or Org B's data silently overwrites Org A's | `UNIQUE (org_id, external_id)` — always |
| Break reason `routable` flag not validated on update | Org sets `routable = true` on a break reason that should be non-routable, bypassing routing controls | No validation issue here — the flag is an org configuration; the bug is the routing query ignoring it, not a security issue |

---

## "Looks Done But Isn't" Checklist

- [ ] **Org isolation:** Verify that `GET /agents` with `X-Org-ID: org_b` after caching data for `org_a` returns an empty list, not org_a's agents.
- [ ] **WrapUp timeout:** Verify that closing the browser during WrapUp and waiting `wrapup_duration + 5s` results in the agent transitioning to Ready (not stuck in WrapUp). Check the DB directly.
- [ ] **Bulk import errors:** Verify that importing a 10-row CSV with 3 invalid rows returns HTTP 207, stores the session, and provides a downloadable error report with row-level detail.
- [ ] **Module Federation embed:** Verify that the MFE renders correctly inside a host shell that has a CSS reset (`* { margin: 0; padding: 0; }`) and a different React patch version.
- [ ] **State transition guard:** Verify that `PUT /agents/{id}/status` with `{ "status": "NotReady" }` while the agent is `Engaged` returns HTTP 409 (not HTTP 200).
- [ ] **External ID isolation:** Verify that importing 3 agents with `external_id` 1, 2, 3 for org_a, then importing 3 agents with the same `external_id` 1, 2, 3 for org_b, results in 6 total rows with no cross-org collision.
- [ ] **Break reason routable:** Verify that an agent on Break with a `routable: false` reason does not appear in the routing candidate query result.
- [ ] **Soft delete cascade:** Verify that soft-deleting an agent removes all queue memberships and skill assignments and emits audit events for each removal.
- [ ] **CSV BOM+CRLF:** Verify that importing a Windows-Excel-exported CSV (BOM + CRLF) succeeds without encoding errors.
- [ ] **CORS with custom header:** Verify that `OPTIONS /agents` from a different origin returns `Access-Control-Allow-Headers: X-Org-ID`.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Cross-org cache poisoning discovered in production | HIGH | Flush entire cache, identify affected orgs from access logs, notify affected customers, add org_id to cache key, re-deploy |
| Background job wrote rows to wrong org | HIGH | Query all rows created by the job without correct org_id, identify true owner from job payload logs, move rows (update org_id in transaction), emit corrective audit events, post-mortem |
| Bulk import wrote 5,000 rows without org_id (all NULL) | MEDIUM | Identify the import session from logs, use the import file to determine the intended org_id, UPDATE all NULL rows in a transaction, add NOT NULL constraint to prevent recurrence |
| Agent stuck in WrapUp (operational) | LOW | `UPDATE agent_states SET status = 'Ready' WHERE id = ? AND status = 'WrapUp'` with manual audit event; then fix the server-side timer |
| Module Federation white-screen in production | MEDIUM | Roll back to previous MFE bundle version; remove `eager: true` from shared config; add async bootstrap; re-deploy |
| CSV schema drift breaks customer import templates | MEDIUM | Add compat layer for old column names; send notification to affected customers with migration guide |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Cache key missing org_id | Phase 1 (org isolation scaffolding) | Integration test: two-org cache isolation |
| Background job missing org context | Phase 1 (job infrastructure) | Test: job executes on clean thread with org_id from payload only |
| Bulk import cross-org rows | Phase 1 (schema) + Phase 2 (import) | DB constraint test: null org_id hard-fails |
| Admin endpoints leak cross-org | Phase 1 (router middleware) | Test: every endpoint with two org_ids returns non-overlapping results |
| Test fixture bleeding | Phase 1 (test infrastructure) | CI: tests run in parallel with unique org_ids |
| WrapUp stuck on disconnect | Phase 3 (agent state model) | Test: kill WebSocket during WrapUp, assert auto-transition |
| Engaged/NotReady race | Phase 3 (agent state model) | Concurrent-request test: two requests in 50ms window |
| Break reason routable ignored | Phase 3 (agent state model) | Routing candidate query test: 2x2 matrix |
| Invalid state transitions | Phase 3 (agent state model) | Test: every disallowed transition returns HTTP 409 |
| Multi-tab state divergence | Phase 3 (agent state model) | Test: remount without context, assert error state |
| Module Federation version skew | Phase 4 (MFE embed) | CI check: compare React versions host vs remote |
| MFE eager-load deadlock | Phase 4 (MFE embed) | Playwright: load MFE in host shell, assert renders in <3s |
| org_id lost on navigation | Phase 4 (MFE embed) | Test: unmount + remount without re-sending context |
| Auth token expiry in MFE | Phase 4 (MFE embed) | Test: mock 401, assert routing:auth-expired event emitted |
| CSS bleed between host/MFE | Phase 4 (MFE embed) | Screenshot test in host with aggressive CSS reset |
| CORS missing X-Org-ID header | Phase 1 (API) + Phase 4 (embed) | CORS preflight test from different origin |
| Soft delete orphans references | Phase 2 (catalog schema) | Test: delete agent, check queue membership table |
| Skill proficiency model split | Phase 2 (catalog schema) | Schema constraint: NOT NULL, CHECK 1..10 |
| Mass update no optimistic lock | Phase 2 (catalog CRUD) | Concurrent PUT test, assert HTTP 409 on conflict |
| external_id collision cross-org | Phase 2 (schema) | Test: same external_id, two orgs, both succeed |
| Catalog cyclic references | Phase 2 (schema design) | Entity graph review in migration PR |
| CSV BOM+CRLF encoding | Phase 2 (bulk import) | Test file: windows-excel.csv in testdata/ |
| Partial success not communicated | Phase 2 (bulk import) | Test: 10-row import with 3 failures returns 207 + error report |
| Large file proxy timeout | Phase 2 (bulk import) | Load test: upload 8MB file through proxy, assert success |
| Schema drift breaks templates | Phase 2 (bulk import) | schema_version in parser from day one |
| Agent desktop feature drift | Phase 3 + 4 (ongoing) | Code review gate: no real-time presence in catalog API |
| Media logic in state machine | Phase 3 (ongoing) | Test: state machine package has no adapter imports |
| Catalog coupled to adapter vendor | Phase 2 (schema) | adapters.config is JSONB, no vendor-specific columns |

---

## Sources

- Borabastab, "Six Shades of Multi-Tenant Mayhem" (Medium, 2025) — cache poisoning, background job context loss patterns
- OWASP Multi-Tenant Security Cheat Sheet — BOLA, privilege escalation, IDOR in multi-tenant APIs
- Redis.io, "Data Isolation in Multi-Tenant SaaS" — cache key design patterns
- Webpack GitHub issue #15829, "Module Federation eager shared libs break" — eager-load deadlock reproduction
- Webpack GitHub discussion #12747, "Module federation not working with eager shared libs" — root cause analysis
- Module Federation examples repo, issue #4410, "Circular Dependency Causes .get() to Remain Pending" — pending promise deadlock
- Cisco UCCX community, "Agent state stuck not-ready after wrap-up" — WrapUp stuck pattern in ACD
- Cisco Webex Contact Center, "Troubleshoot extended WrapUp timer" — server vs client timer ownership
- Genesys Cloud Resource Center, "Routing status" — routing status ↔ agent state relationship
- McKenzie, Patrick. "Design and Implementation of CSV/Excel Upload for SaaS" (Kalzumeus, 2015) — partial success, async processing, encoding pitfalls
- OneUptime Blog, "How to Handle Partial Success in Bulk API Operations" (2026) — HTTP 207, per-row error structures
- Directus GitHub issue #12970, "UTF-8 BOM CSV encoding causes import to malfunction" — real production BOM bug
- Redmine issue #44051, "CSV import fails with CRLF on Windows" — CRLF + BOM combination failure mode
- SplitForge Blog, "CSV Wrong Line Breaks? Fix CRLF vs LF Errors" — CRLF parser behavior
- Gearset Docs, "Resolving DUPLICATE_EXTERNAL_ID errors" — Salesforce upsert external_id collision pattern
- AgnitoStudio Blog, "Preventing Cross-Tenant Data Leakage" — multi-tenant isolation defense-in-depth
- Spring @Async context propagation (javathinking.com) — background thread context loss in async frameworks
- LogRocket, "Solving micro-frontend challenges with Module Federation" — version skew, CSS bleed
- Tailwind CSS Discussion #19578, "Scoping Tailwind in MFE without Shadow DOM" — CSS bleeding prevention
- Open Routing PROJECT.md — Out of Scope constraints and boundary definitions

---

*Pitfalls research for: Open Routing v0.1 Catalog Foundation — embeddable ACD-like routing platform*
*Researched: 2026-05-15*
