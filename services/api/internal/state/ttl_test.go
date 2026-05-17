package state

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// loadAgentStateRow fetches the live agent_states row from the DB for
// assertion in TTL tests. Uses db.WithBypass so the sweeper-context
// pattern matches production; pool is passed as DBTX to accept both
// pgxpool.Pool and OrgDB.
func loadAgentStateRow(t testing.TB, pool generated.DBTX, orgID, agentID uuid.UUID) generated.AgentState {
	t.Helper()
	ctx := db.WithBypass(context.Background(), "test.loadAgentStateRow")
	q := generated.New(pool)
	row, err := q.GetAgentStateByAgentId(ctx, generated.GetAgentStateByAgentIdParams{
		AgentID: pgUUIDv(agentID),
		OrgID:   pgUUIDv(orgID),
	})
	require.NoError(t, err, "loadAgentStateRow: GetAgentStateByAgentId failed")
	return row
}

// TestWrapUpTTL_FiresOnExpiry asserts STATE-07: scheduleWrapUpExpiry fires
// the idempotent ExpireWrapUp UPDATE via clockwork.AfterFunc. To avoid the
// FakeClock ↔ Postgres NOW() skew, the wrapup_until is set just slightly
// in the real future so Postgres' WHERE wrapup_until < NOW() becomes true
// as soon as real time advances; fakeClock controls when the AfterFunc fires.
func TestWrapUpTTL_FiresOnExpiry(t *testing.T) {
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	fakeClock := clockwork.NewFakeClock()
	s := New(Deps{
		OrgDB:  th.S.deps.OrgDB,
		Cache:  th.S.deps.Cache,
		Logger: th.S.deps.Logger,
	}, WithClock(fakeClock), WithSweepInterval(1*time.Hour))
	require.NoError(t, s.Start(ctx))
	defer s.Stop()

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-ttl-fire", "TTL Fire Agent")

	// wrapup_until is just 50ms in the REAL future so Postgres' NOW()
	// predicate (wrapup_until < NOW()) becomes true almost immediately
	// without relying on FakeClock advancing real time.
	wrapupUntil := time.Now().Add(50 * time.Millisecond)
	fakeTriggerUntil := fakeClock.Now().Add(200 * time.Millisecond)
	pis := "ready"
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:              agentID,
		OrgID:                th.OrgID,
		Status:               "WrapUp",
		WrapupUntil:          &wrapupUntil,
		PostInteractionState: &pis,
		StateVersion:         3,
	})

	// Register the AfterFunc with fakeClock. The timer fires when
	// fakeClock.Advance exceeds the fake duration.
	s.scheduleWrapUpExpiry(agentID, th.OrgID, fakeTriggerUntil)

	// Wait for real-time wrapup_until to pass (so Postgres NOW() gate opens).
	time.Sleep(100 * time.Millisecond)

	// Advance FakeClock past fakeTriggerUntil to fire the AfterFunc.
	fakeClock.Advance(300 * time.Millisecond)

	require.Eventually(t, func() bool {
		row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
		return row.Status == "Ready" && row.StateVersion == 4 && !row.WrapupUntil.Valid
	}, 5*time.Second, 50*time.Millisecond, "WrapUp timer never fired or row not transitioned")
}

// TestWrapUpTTL_StartupSweep_PastDue_FiresImmediately asserts D-95:
// Start returns only AFTER synchronously expiring past-due WrapUp rows.
// Directly after Start returns the row must already be transitioned.
func TestWrapUpTTL_StartupSweep_PastDue_FiresImmediately(t *testing.T) {
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	fakeClock := clockwork.NewFakeClock()
	s := New(Deps{
		OrgDB:  th.S.deps.OrgDB,
		Cache:  th.S.deps.Cache,
		Logger: th.S.deps.Logger,
	}, WithClock(fakeClock), WithSweepInterval(1*time.Hour))

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-ttl-pastdue", "Past Due Agent")

	// Seed with wrapup_until 1 hour in the real past so both FakeClock.Now()
	// and Postgres NOW() see it as expired.
	pastDue := time.Now().Add(-1 * time.Hour)
	pis := "ready"
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:              agentID,
		OrgID:                th.OrgID,
		Status:               "WrapUp",
		WrapupUntil:          &pastDue,
		PostInteractionState: &pis,
		StateVersion:         2,
	})

	// D-95: Start is synchronous — startup sweep fires immediately and
	// the row is already transitioned before Start returns.
	require.NoError(t, s.Start(ctx))
	defer s.Stop()

	row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
	require.Equal(t, "Ready", row.Status,
		"past-due WrapUp must be expired synchronously during Start")
	require.Equal(t, int64(3), row.StateVersion,
		"state_version must increment exactly once")
	require.False(t, row.WrapupUntil.Valid,
		"wrapup_until must be cleared after expiry")
}

func TestWrapUpTTL_ExpireWrapUp_UsesAndClearsPostInteractionState(t *testing.T) {
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	s := New(Deps{
		OrgDB:  th.S.deps.OrgDB,
		Cache:  th.S.deps.Cache,
		Logger: th.S.deps.Logger,
	})

	cases := []struct {
		name       string
		pis        string
		wantStatus string
	}{
		{"ready", "ready", "Ready"},
		{"not_ready", "not_ready", "NotReady"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			agentID := uuid.Must(uuid.NewV7())
			channel := "voice"
			pastDue := time.Now().Add(-1 * time.Hour)
			seedAgent(t, th.Pool, th.OrgID, agentID, "ext-ttl-pis-"+c.name, "TTL PIS Agent "+c.name)
			seedAgentStateRow(t, th.Pool, SeedStateParams{
				AgentID:              agentID,
				OrgID:                th.OrgID,
				Status:               "WrapUp",
				EngagedChannel:       &channel,
				WrapupUntil:          &pastDue,
				PostInteractionState: &c.pis,
				StateVersion:         1,
			})

			s.expireWrapUp(ctx, agentID, th.OrgID)

			row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
			require.Equal(t, c.wantStatus, row.Status)
			require.Nil(t, row.PostInteractionState)
			require.False(t, row.WrapupUntil.Valid)
			require.Nil(t, row.EngagedChannel)
		})
	}
}

// TestWrapUpTTL_StartupSweep_FutureSchedules asserts that Start schedules
// AfterFunc timers for WrapUp rows with future wrapup_until (does NOT
// fire immediately), then the timer fires after enough clock time passes.
func TestWrapUpTTL_StartupSweep_FutureSchedules(t *testing.T) {
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	fakeClock := clockwork.NewFakeClock()
	s := New(Deps{
		OrgDB:  th.S.deps.OrgDB,
		Cache:  th.S.deps.Cache,
		Logger: th.S.deps.Logger,
	}, WithClock(fakeClock), WithSweepInterval(1*time.Hour))

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-ttl-future", "Future Schedule Agent")

	// wrapup_until is 200ms in the real future so Postgres' NOW() will
	// see it as expired after we sleep, and fakeClock controls when
	// the AfterFunc fires (via startupSweep's scheduleWrapUpExpiry).
	futureWrapup := time.Now().Add(200 * time.Millisecond)
	fakeFutureUntil := fakeClock.Now().Add(500 * time.Millisecond)

	// Seed using the FAKE time for the wrapup_until column — startupSweep
	// compares r.WrapupUntil.Time.After(s.clock.Now()); we need it to be
	// in the FAKE future so startupSweep schedules (not fires immediately).
	// We then also need Postgres NOW() predicate to fire — use real future.
	//
	// Compromise: seed with REAL future timestamp but override fakeClock
	// to be BEFORE the real timestamp so startupSweep takes the schedule path.
	_ = fakeFutureUntil // suppress unused warning

	pis := "not_ready"
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:              agentID,
		OrgID:                th.OrgID,
		Status:               "WrapUp",
		WrapupUntil:          &futureWrapup,
		PostInteractionState: &pis,
		StateVersion:         5,
	})

	require.NoError(t, s.Start(ctx))
	defer s.Stop()

	// Immediately after Start, row still in WrapUp (future timestamp,
	// startupSweep scheduled an AfterFunc but did not fire immediately).
	rowImmediate := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
	require.Equal(t, "WrapUp", rowImmediate.Status,
		"future WrapUp must NOT be expired immediately by startupSweep")

	// Wait for real-time wrapup_until to pass (Postgres NOW() gate opens).
	time.Sleep(300 * time.Millisecond)

	// Advance FakeClock past the scheduled timer duration so AfterFunc fires.
	// startupSweep called scheduleWrapUpExpiry(agentID, orgID, futureWrapup)
	// with remaining = futureWrapup - fakeClock.Now() ≈ 200ms (real duration
	// at the time Start was called). Advance by 1s covers any variance.
	fakeClock.Advance(1 * time.Second)

	require.Eventually(t, func() bool {
		row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
		return row.Status == "NotReady" && !row.WrapupUntil.Valid
	}, 5*time.Second, 50*time.Millisecond,
		"future WrapUp timer must fire after clock advances past wrapup_until")
}

// TestWrapUpTTL_SafetySweep_FiresMissedTimer verifies the 30s safety-net
// ticker catches a past-due WrapUp row. We call runSweepPastDue directly
// to avoid the FakeClock ↔ real-ticker ordering complexity.
func TestWrapUpTTL_SafetySweep_FiresMissedTimer(t *testing.T) {
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	fakeClock := clockwork.NewFakeClock()
	s := New(Deps{
		OrgDB:  th.S.deps.OrgDB,
		Cache:  th.S.deps.Cache,
		Logger: th.S.deps.Logger,
	}, WithClock(fakeClock), WithSweepInterval(1*time.Hour))

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-ttl-missed", "Missed Timer Agent")

	// Seed past-due in both real time and FakeClock time.
	pastDue := time.Now().Add(-2 * time.Hour)
	pis := "ready"
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:              agentID,
		OrgID:                th.OrgID,
		Status:               "WrapUp",
		WrapupUntil:          &pastDue,
		PostInteractionState: &pis,
		StateVersion:         7,
	})

	// Start without scheduling a timer (simulates a missed AfterFunc).
	// startupSweep will fire the past-due row immediately, so we need
	// to re-seed it after Start to simulate a row that missed its timer.
	// Call runSweepPastDue directly — this exercises the safety-sweep
	// body without depending on the ticker advancing in real time.
	require.NoError(t, s.Start(ctx))
	defer s.Stop()

	// Re-seed after startupSweep fired and reset the row (simulate a
	// new WrapUp row that arrives after startup but misses its AfterFunc).
	wrapupMissed := time.Now().Add(-30 * time.Minute)
	pis2 := "ready"
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:              agentID,
		OrgID:                th.OrgID,
		Status:               "WrapUp",
		WrapupUntil:          &wrapupMissed,
		PostInteractionState: &pis2,
		StateVersion:         8,
	})

	// Directly invoke the safety-sweep body. This is equivalent to what
	// happens when the 30s ticker fires after an AfterFunc miss.
	s.runSweepPastDue()

	row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
	require.Equal(t, "Ready", row.Status,
		"safety sweep body must fire past-due WrapUp row")
	require.False(t, row.WrapupUntil.Valid,
		"wrapup_until must be cleared after expiry")
}

// TestWrapUpTTL_GracefulShutdown_NoFireAfterStop is the Pitfall 2 regression
// gate. Stop must drain pending timers and cancel ctx BEFORE any AfterFunc
// body issues the DB UPDATE.
func TestWrapUpTTL_GracefulShutdown_NoFireAfterStop(t *testing.T) {
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	fakeClock := clockwork.NewFakeClock()
	s := New(Deps{
		OrgDB:  th.S.deps.OrgDB,
		Cache:  th.S.deps.Cache,
		Logger: th.S.deps.Logger,
	}, WithClock(fakeClock), WithSweepInterval(1*time.Hour))
	require.NoError(t, s.Start(ctx))

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-ttl-shutdown", "Shutdown Agent")

	// wrapup_until in the real future — if the timer fires after Stop(),
	// the UPDATE would still fail Postgres' NOW() predicate. We set it
	// in the NEAR future so that if the timer does fire, it would succeed.
	wrapupUntil := time.Now().Add(50 * time.Millisecond)
	fakeTriggerUntil := fakeClock.Now().Add(200 * time.Millisecond)
	pis := "ready"
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:              agentID,
		OrgID:                th.OrgID,
		Status:               "WrapUp",
		WrapupUntil:          &wrapupUntil,
		PostInteractionState: &pis,
		StateVersion:         3,
	})

	s.scheduleWrapUpExpiry(agentID, th.OrgID, fakeTriggerUntil)

	// Stop drains timers + cancels ctx BEFORE advancing the clock.
	// Timer.Stop() called in Stop() must cancel the pending AfterFunc.
	s.Stop()

	// Sleep past the real wrapup_until so Postgres' NOW() predicate
	// WOULD succeed if the timer fires.
	time.Sleep(100 * time.Millisecond)

	// Advance past fake trigger to fire (if timer survived Stop).
	fakeClock.Advance(300 * time.Millisecond)

	// Give any stray goroutine a chance to land (the only legitimate
	// time.Sleep in TTL tests — we wait to PROVE no event fires, not to
	// OBSERVE an event).
	time.Sleep(150 * time.Millisecond)

	row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
	require.Equal(t, "WrapUp", row.Status,
		"Stop() must prevent post-Stop AfterFunc firings from reaching the DB")
	require.Equal(t, int64(3), row.StateVersion,
		"no UPDATE should have landed after Stop")
}

// TestWrapUpTTL_RaceCondition_IdempotentMultiFire asserts D-81 idempotency:
// concurrent expireWrapUp calls must produce exactly one state change
// (state_version bumps by exactly 1, not N).
func TestWrapUpTTL_RaceCondition_IdempotentMultiFire(t *testing.T) {
	th := newTestHandlers(t)
	require.NotNil(t, th)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cleanStateTables(t, ctx, th.Pool, th.OrgID)

	s := New(Deps{
		OrgDB:  th.S.deps.OrgDB,
		Cache:  th.S.deps.Cache,
		Logger: th.S.deps.Logger,
	})
	require.NoError(t, s.Start(ctx))
	defer s.Stop()

	agentID := uuid.Must(uuid.NewV7())
	seedAgent(t, th.Pool, th.OrgID, agentID, "ext-ttl-race", "Race Condition Agent")
	pastDue := time.Now().Add(-1 * time.Hour)
	pis := "ready"
	seedAgentStateRow(t, th.Pool, SeedStateParams{
		AgentID:              agentID,
		OrgID:                th.OrgID,
		Status:               "WrapUp",
		WrapupUntil:          &pastDue,
		PostInteractionState: &pis,
		StateVersion:         3,
	})

	// Fire from 10 concurrent goroutines — only one UPDATE predicate
	// (status='WrapUp' AND wrapup_until < NOW()) wins per D-81.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			callCtx, callCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer callCancel()
			s.expireWrapUp(callCtx, agentID, th.OrgID)
		}()
	}
	wg.Wait()

	row := loadAgentStateRow(t, th.Pool, th.OrgID, agentID)
	require.Equal(t, "Ready", row.Status,
		"expected transition to Ready (post_interaction_state=ready)")
	require.Equal(t, int64(4), row.StateVersion,
		"idempotent UPDATE must yield exactly one version bump (not 13)")
}
