Warning: 256-color support not detected. Using a terminal with at least 256-color support is recommended for a better visual experience.
Ripgrep is not available. Falling back to GrepTool.
Error stating path example.com",
    })
    require.Equal(t, http.StatusCreated, code)
    require.Equal(t, http.StatusOK,
        getEntityStatus(t, baseURL(), "agents/"+agentA.String()+"/status", orgA, uuid.Nil))
    require.Equal(t, http.StatusNotFound,
        getEntityStatus(t, baseURL(), "agents/"+agentA.String()+"/status", orgB, uuid.Nil))
}
```

**Note:** RESEARCH §Cross-org isolation tests flagged that the existing `getEntityStatus` helper takes (urlBase, entityPath, orgID, id). For `/agents/{id}/status` you can stuff the path into entityPath as `"agents/<uuid>/status"` and pass `uuid.Nil` as the id — the helper concatenates `path+id` if id != uuid.Nil. **Better:** add a new helper `getStatusEndpoint(t, urlBase, orgID, agentID)` that builds `/v1/orgs/{orgID}/agents/{agentID}/status` cleanly. The new helper goes into `state_test.go` (NOT into `catalog_test.go`).

**(c) is the critical Pitfall 3 test:**

```go
// (c) PATCH /status with force=true does NOT bypass cross-org break_reason_id check.
func TestState_ForceDoesNotBypassCrossOrgBreakReason(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    orgA, orgB := freshOrg(t), freshOrg(t)

    // Seed: agent in A, break_reason in B (cross-org).
    _, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{...})
    _, brB := postEntity(t, baseURL(), "break-reasons", orgB, map[string]any{
        "name": "OtherOrgBreak", "routable": false, "display_order": 0,
    })

    // PATCH agentA's status to Break with brB + force=true.
    // Expected: 422 invalid_reference (force bypasses matrix, NOT probes).
    code := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
        "to": "Break", "break_reason_id": brB.String(), "force": true,
    })
    require.Equal(t, http.StatusUnprocessableEntity, code,
        "force=true: ENAMETOOLONG: name too long, stat '/Users/luong/workspace/dev/open-solutions/example.com",
    })
    require.Equal(t, http.StatusCreated, code)
    require.Equal(t, http.StatusOK,
        getEntityStatus(t, baseURL(), "agents/"+agentA.String()+"/status", orgA, uuid.Nil))
    require.Equal(t, http.StatusNotFound,
        getEntityStatus(t, baseURL(), "agents/"+agentA.String()+"/status", orgB, uuid.Nil))
}
```

**Note:** RESEARCH §Cross-org isolation tests flagged that the existing `getEntityStatus` helper takes (urlBase, entityPath, orgID, id). For `/agents/{id}/status` you can stuff the path into entityPath as `"agents/<uuid>/status"` and pass `uuid.Nil` as the id — the helper concatenates `path+id` if id != uuid.Nil. **Better:** add a new helper `getStatusEndpoint(t, urlBase, orgID, agentID)` that builds `/v1/orgs/{orgID}/agents/{agentID}/status` cleanly. The new helper goes into `state_test.go` (NOT into `catalog_test.go`).

**(c) is the critical Pitfall 3 test:**

```go
/ (c) PATCH /status with force=true does NOT bypass cross-org break_reason_id check.
func TestState_ForceDoesNotBypassCrossOrgBreakReason(t *testing.T) {
    requireContainer(t)
    t.Parallel()
    orgA, orgB := freshOrg(t), freshOrg(t)

    / Seed: agent in A, break_reason in B (cross-org).
    _, agentA := postEntity(t, baseURL(), "agents", orgA, map[string]any{...})
    _, brB := postEntity(t, baseURL(), "break-reasons", orgB, map[string]any{
        "name": "OtherOrgBreak", "routable": false, "display_order": 0,
    })

    / PATCH agentA's status to Break with brB + force=true.
    / Expected: 422 invalid_reference (force bypasses matrix, NOT probes).
    code := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
        "to": "Break", "break_reason_id": brB.String(), "force": true,
    })
    require.Equal(t, http.StatusUnprocessableEntity, code,
        "force=true'
Error stating path example.com",
    })
    require.Equal(t, http.StatusCreated, codeA)
    codeB, brB := postEntity(t, baseURL(), "break-reasons", orgB, map[string]any{
        "name": "OtherOrgBreakForce", "routable": false, "display_order": 0,
    })
    require.Equal(t, http.StatusCreated, codeB)

    // Even with force=true + valid matrix bypass — cross-row probe still fires.
    code, raw := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
        "to": "Break", "break_reason_id": brB.String(), "force": true,
    })
    require.Equal(t, http.StatusUnprocessableEntity, code,
        "force=true: ENAMETOOLONG: name too long, stat '/Users/luong/workspace/dev/open-solutions/example.com",
    })
    require.Equal(t, http.StatusCreated, codeA)
    codeB, brB := postEntity(t, baseURL(), "break-reasons", orgB, map[string]any{
        "name": "OtherOrgBreakForce", "routable": false, "display_order": 0,
    })
    require.Equal(t, http.StatusCreated, codeB)

    / Even with force=true + valid matrix bypass — cross-row probe still fires.
    code, raw := patchStatusReturnCode(t, baseURL(), orgA, agentA, map[string]any{
        "to": "Break", "break_reason_id": brB.String(), "force": true,
    })
    require.Equal(t, http.StatusUnprocessableEntity, code,
        "force=true'
## Summary
The Phase 4 planning artifacts are exceptionally thorough, demonstrating deep alignment with the Phase 3 foundation and the project's strict architectural mandates (No FKs, WHY-not-WHAT comments, and cross-org isolation). The wave decomposition correctly handles the transition from pure-domain logic to production wiring and end-to-end acceptance. However, two critical behavioral bugs in the TTL/Registry design and a significant casing mismatch in the schema must be addressed to prevent runtime panics and database constraint violations.

## Strengths
- **Renaming to `state.Server` (Pitfall 1):** Proactively identifying the anonymous embedding collision between `catalog.Handlers` and `state.Handlers` and renaming the latter to `Server` eliminates a major integration hurdle in Wave 4.
- **Probe-then-Matrix Ordering (Pitfall 3):** Explicitly ordering the handler logic to run cross-row probes before the transition matrix ensures that `force=true` cannot be used to leak or manipulate cross-org `break_reason_id` values.
- **Clockwork Integration (D-81):** Using `jonboulle/clockwork` instead of archived alternatives allows for deterministic testing of the high-performance `time.AfterFunc` logic.
- **SQLChecker Lifecycle:** Waves 0 and 1 correctly register the new table and ensure every query (including the sweeper bypass path) mentions `org_id` to satisfy the security validator.
- **Acceptance Mapping:** Every ROADMAP success criterion is mapped to a named integration test, ensuring a high-confidence ship.

## Concerns

### [HIGH]: `PostInteractionState` Case Mismatch
The `agent_states` schema (D-78) and OpenAPI spec define `PostInteractionState` values as lowercase (`ready`, `not_ready`), but the target `status` column requires PascalCase (`Ready`, `NotReady`). The `ExpireWrapUp` SQL in Wave 1 (`SET status = COALESCE(post_interaction_state, 'NotReady')`) will fail the `status` CHECK constraint if a lowercase value is applied.
*   **Impact:** Database `check_violation` (23514) on every WrapUp expiration.

### [HIGH]: Timer Registry Corruption / Stop() Leak
The `scheduleWrapUpExpiry` callback in `04-04-PLAN.md` deletes the map entry by key (`delete(s.timers.t, agentID)`) without verifying if the entry still belongs to the specific timer that fired. If a timer is replaced (e.g., rapid state changes or v0.2 extensions), an old callback firing will delete the *new* timer's handle from the map.
*   **Impact:** `Server.Stop()` will fail to cancel the active successor timer, leading to background goroutines attempting to hit the DB after the pool is closed.

### [MED]: `jitter()` Implementation Panic
The `jitter()` function in `04-04-PLAN.md` uses `int64(uuid.New().Time())`. `uuid.New()` produces a v4 (random) UUID; calling `.Time()` on a random UUID causes a panic in the `github.com/google/uuid` library (it is only valid for v1/v2/v6/v7).
*   **Impact:** Process panic on system transitions (even if deferred to v0.2, the code exists in v0.1).

### [MED]: Dead Code / Scope Creep
As noted by the `gsd-plan-checker`, `jitter()` is unreachable in v0.1 because no production path sets `wrapup_until` (D-82). Similarly, `_ = pgtype.Text` in `ttl.go` is a draft-guard that should be pruned before commit.

## Suggestions
1.  **Align Casing:** Update `openapi/openapi.yaml` and the `agent_states` migration (Wave 0) to use PascalCase for `PostInteractionState` (`Ready`, `NotReady`) to match the `AgentStatus` enum.
2.  **Safeguard Registry Deletion:** In the `AfterFunc` closure, capture the timer handle and only delete from the map if it matches the current value:
    ```go
    var t clockwork.Timer
    t = s.clock.AfterFunc(d, func() {
        // ... (expire logic)
        s.timers.mu.Lock()
        if s.timers.t[agentID] == t { delete(s.timers.t, agentID) }
        s.timers.mu.Unlock()
    })
    s.timers.t[agentID] = t
    ```
3.  **Fix Jitter Entropy:** Use `math/rand` or `uuid.Must(uuid.NewV7()).Time()` to avoid the v4 `.Time()` panic.
4.  **Prune Draft Guards:** Remove the `_ = pgtype.Text` and `jitter()` helpers if they are not strictly required for v0.1 tests, or mark them clearly as "Schema Ready" with a functional test.

## Verdict: READY WITH FIXES
