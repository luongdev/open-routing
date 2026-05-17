// sweep_test.go — clockwork-backed coverage for D5-11 crash-recovery
// sweep (Plan 05-04 Wave 2).
//
// Mirrors state/ttl_test.go pattern:
//   - fakeClock controls when the safety-sweep ticker fires (real time
//     never elapses in tests).
//   - Postgres NOW() is the gate for the SweepCrashedImportJobs WHERE
//     predicate — seeded rows use updated_at = NOW() - INTERVAL '25 hours'
//     (real timestamp) so Postgres sees them as expired regardless of
//     fakeClock.
//   - require.Eventually polls for the post-sweep row state.
//
// All tests skip under `-short` (newTestImports handles the skip via
// sharedPool == nil). Wave 5 will exercise the handler-level integration
// tests; this wave covers the goroutine lifecycle + SQL contract.
package imports

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestImportSweep_StartCancellation asserts Start/Stop lifecycle drains
// cleanly even when no past-due rows exist. Skipped under -short via
// newTestImports' sharedPool nil check.
//
// Pattern source: state/ttl_test.go does the same Start+Stop+wg.Wait
// dance for its sweep goroutine.
func TestImportSweep_StartCancellation(t *testing.T) {
	th := newTestImports(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Start — startupSweep runs synchronously with no past-due rows;
	// returns nil. safetySweep goroutine spawns and blocks on
	// ticker/ctx.
	require.NoError(t, th.I.Start(ctx))

	// Stop — cancels internal ctx; wg.Wait drains the goroutine.
	// Should return within ~ms (no DB work outstanding).
	done := make(chan struct{})
	go func() {
		th.I.Stop()
		close(done)
	}()
	select {
	case <-done:
		// Goroutine exited.
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not drain safetySweep goroutine within 2s")
	}

	// Idempotent: second Stop returns immediately.
	th.I.Stop()
}

// TestImportSweep_PendingOver24h_FlipsToFailed_OnStart asserts D5-11
// synchronous startup sweep flips past-due pending rows BEFORE Start
// returns. The row's updated_at is in the real past so Postgres NOW()
// satisfies the WHERE predicate.
func TestImportSweep_PendingOver24h_FlipsToFailed_OnStart(t *testing.T) {
	th := newTestImports(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Seed a past-due pending row: updated_at = 25h ago.
	past := time.Now().Add(-25 * time.Hour)
	jobID := seedPendingImportJob(t, ctx, th.Pool, th.OrgID, "agents", past)

	// Start: startupSweep is synchronous so the row is flipped before
	// Start returns.
	require.NoError(t, th.I.Start(ctx))
	defer th.I.Stop()

	status, errorsJSON := fetchImportJobStatus(t, ctx, th.Pool, th.OrgID, jobID)
	require.Equal(t, "failed", status,
		"past-due WrapUp must be flipped to failed by startupSweep")
	require.Contains(t, string(errorsJSON), "server_crash",
		"errors JSONB must include the synthetic server_crash entry")
}

// TestImportSweep_PendingUnder24h_NoOp asserts a row whose updated_at
// is within the 24h TTL is NOT flipped — neither by startupSweep nor by
// a tick advancement.
func TestImportSweep_PendingUnder24h_NoOp(t *testing.T) {
	th := newTestImports(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Seed a not-yet-past-due pending row: updated_at = 23h ago.
	recent := time.Now().Add(-23 * time.Hour)
	jobID := seedPendingImportJob(t, ctx, th.Pool, th.OrgID, "agents", recent)

	require.NoError(t, th.I.Start(ctx))
	defer th.I.Stop()

	// Advance fakeClock past one tick (default sweepInterval is 1h in
	// the test fixture). The sweep tick fires runSweepPastDue, which
	// queries the same WHERE — and still sees the row as not past-due.
	th.FakeClk.Advance(90 * time.Minute)

	// Poll briefly to let the ticker goroutine react.
	time.Sleep(200 * time.Millisecond)

	status, _ := fetchImportJobStatus(t, ctx, th.Pool, th.OrgID, jobID)
	require.Equal(t, "pending", status,
		"row within 24h TTL must remain pending")
}

// TestImportSweep_PendingOver24h_FlipsToFailed_OnTick asserts the
// safety sweep ticker (after Start has spawned the goroutine) flips a
// pending row that becomes past-due AFTER startup. We seed the row
// post-Start so startupSweep does not see it, then advance fakeClock
// past one tick interval and poll for the flip.
func TestImportSweep_PendingOver24h_FlipsToFailed_OnTick(t *testing.T) {
	th := newTestImports(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanImportTables(t, ctx, th.Pool, th.OrgID)

	require.NoError(t, th.I.Start(ctx))
	defer th.I.Stop()

	// Seed AFTER Start so startupSweep ran on an empty table. The
	// safetySweep goroutine is now blocked on the 1h ticker.
	past := time.Now().Add(-25 * time.Hour)
	jobID := seedPendingImportJob(t, ctx, th.Pool, th.OrgID, "agents", past)

	// Pre-condition: row is still pending.
	status, _ := fetchImportJobStatus(t, ctx, th.Pool, th.OrgID, jobID)
	require.Equal(t, "pending", status)

	// Advance fakeClock past one tick — runSweepPastDue runs in the
	// safetySweep goroutine.
	th.FakeClk.Advance(2 * time.Hour)

	// Poll for the flip. Postgres NOW() and goroutine scheduling
	// introduce small delays; require.Eventually with a generous
	// timeout absorbs them.
	require.Eventually(t, func() bool {
		s, _ := fetchImportJobStatus(t, ctx, th.Pool, th.OrgID, jobID)
		return s == "failed"
	}, 5*time.Second, 50*time.Millisecond,
		"safety sweep tick must flip past-due pending row")
}

// TestImportSweep_ServerCrashErrorShape asserts the persisted errors
// JSONB conforms to the locked D5-11 shape: one synthetic entry with
// {row: 0, error: import_failed, reason: server_crash}. This is the
// shape the wire layer rehydrates via mapImportJob.
func TestImportSweep_ServerCrashErrorShape(t *testing.T) {
	th := newTestImports(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanImportTables(t, ctx, th.Pool, th.OrgID)

	past := time.Now().Add(-25 * time.Hour)
	jobID := seedPendingImportJob(t, ctx, th.Pool, th.OrgID, "agents", past)

	require.NoError(t, th.I.Start(ctx))
	defer th.I.Stop()

	_, errorsJSON := fetchImportJobStatus(t, ctx, th.Pool, th.OrgID, jobID)
	// Decode the persisted JSONB and assert the locked D5-11 shape.
	// Substring matching is fragile because Postgres re-formats JSONB
	// (inserts whitespace after `:` and `,`); decoding gives us a
	// stable contract regardless of formatting.
	var failed []map[string]any
	require.NoError(t, json.Unmarshal(errorsJSON, &failed),
		"errors JSONB must be a decodable array")
	require.Len(t, failed, 1, "must be exactly one synthetic crash entry")
	require.EqualValues(t, 0, failed[0]["row"], "row must be 0")
	require.Equal(t, "import_failed", failed[0]["error"], "error must be import_failed")
	require.Equal(t, "server_crash", failed[0]["reason"], "reason must be server_crash")
}

// TestImportSweep_RunSweepPastDue_DirectInvocation calls
// runSweepPastDue directly (without spinning the ticker) — useful
// because it doesn't depend on fakeClock advancement timing. Mirrors
// state/ttl_test.go's TestWrapUpTTL_SafetySweep_FiresMissedTimer
// pattern.
func TestImportSweep_RunSweepPastDue_DirectInvocation(t *testing.T) {
	th := newTestImports(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanImportTables(t, ctx, th.Pool, th.OrgID)

	past := time.Now().Add(-25 * time.Hour)
	// Seed an initial past-due row so the Importer's startupSweep has
	// something to find (drains the startup-side path before we exercise
	// the per-tick runSweepPastDue directly). The seeded ID is not
	// inspected; the row is re-cleaned and re-seeded below.
	_ = seedPendingImportJob(t, ctx, th.Pool, th.OrgID, "agents", past)

	// Start the Importer so its internal ctx is initialised (required
	// by runSweepPastDue's context.WithTimeout(s.ctx, ...) call).
	require.NoError(t, th.I.Start(ctx))
	defer th.I.Stop()

	// Re-seed: startupSweep already fired on this row when Start was
	// called. Insert a fresh past-due row to exercise runSweepPastDue
	// directly.
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	jobID := seedPendingImportJob(t, ctx, th.Pool, th.OrgID, "agents", past)

	// Invoke the per-tick body directly.
	th.I.runSweepPastDue()

	status, _ := fetchImportJobStatus(t, ctx, th.Pool, th.OrgID, jobID)
	require.Equal(t, "failed", status,
		"runSweepPastDue direct invocation must flip past-due row")
}

// TestImportSweep_CrossOrgIsolation asserts the sweep flips rows
// regardless of org_id (D5-11 SOLE org-agnostic query carve-out).
// FOUND-08 isolation does NOT apply to the sweep — the goroutine is
// process-wide; the WithBypass marker is the audit-trail signal that
// authorises the cross-org reach.
func TestImportSweep_CrossOrgIsolation(t *testing.T) {
	th := newTestImports(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Two distinct orgs — both seeded with past-due rows.
	orgA := th.OrgID
	orgB := uuid.Must(uuid.NewV7())
	cleanImportTables(t, ctx, th.Pool, orgA)
	cleanImportTables(t, ctx, th.Pool, orgB)

	past := time.Now().Add(-25 * time.Hour)
	jobA := seedPendingImportJob(t, ctx, th.Pool, orgA, "agents", past)
	jobB := seedPendingImportJob(t, ctx, th.Pool, orgB, "skills", past)

	require.NoError(t, th.I.Start(ctx))
	defer th.I.Stop()

	statusA, _ := fetchImportJobStatus(t, ctx, th.Pool, orgA, jobA)
	statusB, _ := fetchImportJobStatus(t, ctx, th.Pool, orgB, jobB)
	require.Equal(t, "failed", statusA, "sweep must flip orgA's past-due row")
	require.Equal(t, "failed", statusB, "sweep must flip orgB's past-due row")
}

// TestImportSweep_HandlesEmptyTable asserts startupSweep is a clean
// no-op against an empty import_jobs table. The 0-row case fits the
// plan's coverage requirement (D5-11 happy-path null).
func TestImportSweep_HandlesEmptyTable(t *testing.T) {
	th := newTestImports(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// No rows inserted. startupSweep must succeed cleanly and Start
	// must return nil so cmd/api/main.go's `if err := imp.Start ...`
	// keeps proceeding.
	require.NoError(t, th.I.Start(ctx))
	defer th.I.Stop()
}

// TestImportSweep_StopBeforeStart_NoOp asserts Stop on an Importer that
// was never Started returns cleanly (idempotent — matches state.Server
// contract). Captures Pitfall: a startup-failure path that called
// cancel() then Stop() must not double-deref s.cancel.
func TestImportSweep_StopBeforeStart_NoOp(t *testing.T) {
	th := newTestImports(t)
	require.NotNil(t, th)

	// Stop without prior Start. Should be a no-op via startMu guard.
	done := make(chan struct{})
	go func() {
		th.I.Stop()
		close(done)
	}()
	select {
	case <-done:
		// expected
	case <-time.After(1 * time.Second):
		t.Fatal("Stop-before-Start did not return within 1s")
	}
}

// containsSubstring is a small readability helper for the JSON shape
// assertions where we want a specific substring but don't want to
// commit to JSON-decode semantics in a sweep test.
//
//nolint:unused // available for future sweep_test additions; current
// tests use require.Contains which is more idiomatic.
func containsSubstring(s, sub string) bool {
	return strings.Contains(s, sub)
}
