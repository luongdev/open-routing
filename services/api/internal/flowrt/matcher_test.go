package flowrt

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
)

// seedRunningRoute inserts a minimal route_requests row in 'running' so the
// matcher queries can act on it (no flow execution needed for the SQL tests).
func seedRunningRoute(ctx context.Context, t *testing.T, org uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	fv := uuid.Must(uuid.NewV7())
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO route_requests (id, org_id, channel, entry_code, status, flow_version_id, flow_code)
		 VALUES ($1,$2,'voice','main','running',$3,'flow_x')`, id, org, fv); err != nil {
		t.Fatalf("seed route: %v", err)
	}
	return id
}

func TestMatcher_EnqueueThenClaim(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	ctx := context.Background()
	q := generated.New(sharedPool)
	org := uuid.Must(uuid.NewV7())
	routeID := seedRunningRoute(ctx, t, org)

	// running → waiting_match with the queue/skills context.
	if _, err := q.EnqueueRouteForMatch(ctx, generated.EnqueueRouteForMatchParams{RouteRequestID: pgUUID(routeID), OrgID: pgUUID(org), Priority: 5,
		RequiredSkills: []string{"skill_es"}, ResumeCursor: []byte("{}"),
		MatchDeadline: ts(time.Now().Add(2 * time.Minute)),
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// An eligible agent (has skill_es) claims it → status flips to offering.
	claimed, err := q.ClaimWaitingRoute(ctx, generated.ClaimWaitingRouteParams{
		OrgID: pgUUID(org), Column2: []string{"skill_es", "skill_x"},
		Column3: pgUUID(uuid.Must(uuid.NewV7())), Column4: 100, Column5: 1,
	})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed.Status != "offering" || !claimed.MatchOfferToken.Valid {
		t.Fatalf("claimed status=%q token.valid=%v, want offering + a token", claimed.Status, claimed.MatchOfferToken.Valid)
	}

	// An agent WITHOUT the skill cannot claim a fresh route.
	other := seedRunningRoute(ctx, t, org)
	_, _ = q.EnqueueRouteForMatch(ctx, generated.EnqueueRouteForMatchParams{RouteRequestID: pgUUID(other), OrgID: pgUUID(org), RequiredSkills: []string{"skill_es"}, ResumeCursor: []byte("{}")})
	if _, err := q.ClaimWaitingRoute(ctx, generated.ClaimWaitingRouteParams{
		OrgID: pgUUID(org), Column2: []string{"skill_fr"}, Column3: pgUUID(uuid.Must(uuid.NewV7())), Column4: 100, Column5: 1,
	}); err != pgx.ErrNoRows {
		t.Fatalf("ineligible claim err=%v, want ErrNoRows", err)
	}
}

// TestMatcher_SLAExpiredNotClaimable: a route past its match_deadline is NOT
// claimable by the matcher (it belongs to the SLA sweep → fallback), but IS
// reclaimed by ClaimExpiredMatchRoutes.
func TestMatcher_SLAExpiredNotClaimable(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	ctx := context.Background()
	q := generated.New(sharedPool)
	org := uuid.Must(uuid.NewV7())
	routeID := seedRunningRoute(ctx, t, org)
	if _, err := q.EnqueueRouteForMatch(ctx, generated.EnqueueRouteForMatchParams{RouteRequestID: pgUUID(routeID), OrgID: pgUUID(org), RequiredSkills: []string{}, ResumeCursor: []byte("{}"),
		MatchDeadline: ts(time.Now().Add(-time.Minute)), // already past SLA
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// The matcher must NOT claim it.
	if _, err := q.ClaimWaitingRoute(ctx, generated.ClaimWaitingRouteParams{
		OrgID: pgUUID(org), Column2: []string{}, Column3: pgUUID(uuid.Must(uuid.NewV7())), Column4: 100, Column5: 1,
	}); err != pgx.ErrNoRows {
		t.Fatalf("claim of SLA-expired route err=%v, want ErrNoRows", err)
	}
	// The deadline sweep discovers it (cross-org list) and the fenced acquire flips
	// it to 'running' for the no_candidate fallback — exactly once.
	rows, err := q.ListExpiredMatchRoutes(ctx, 10)
	if err != nil {
		t.Fatalf("list expired: %v", err)
	}
	found := false
	for _, r := range rows {
		if apiUUID(r.ID) == routeID {
			found = true
		}
	}
	if !found {
		t.Fatalf("SLA-expired route not discovered by the deadline sweep")
	}
	got, err := q.AcquireExpiredMatchRouteForRun(ctx, generated.AcquireExpiredMatchRouteForRunParams{ID: pgUUID(routeID), OrgID: pgUUID(org)})
	if err != nil {
		t.Fatalf("acquire expired: %v", err)
	}
	if got.Status != "running" {
		t.Fatalf("acquired SLA-expired route status=%q, want running", got.Status)
	}
	// Second acquire (a racing replica) gets nothing — the status guard fences it.
	if _, err := q.AcquireExpiredMatchRouteForRun(ctx, generated.AcquireExpiredMatchRouteForRunParams{ID: pgUUID(routeID), OrgID: pgUUID(org)}); err != pgx.ErrNoRows {
		t.Fatalf("second acquire err=%v, want ErrNoRows (fenced)", err)
	}
}

// TestMatcher_EnqueueExcludesRejectedAgents: a route re-enqueued after an agent
// rejected/timed-out won't be re-offered to that agent by the matcher (RONA
// distinct-agent — excluded_agent_ids synced from reservations on enqueue).
func TestMatcher_EnqueueExcludesRejectedAgents(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	ctx := context.Background()
	q := generated.New(sharedPool)
	org := uuid.Must(uuid.NewV7())
	routeID := seedRunningRoute(ctx, t, org)
	missed := uuid.Must(uuid.NewV7())
	// A prior offer to `missed` that timed out.
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO reservations (id, org_id, route_request_id, agent_id, state, expires_at)
		 VALUES ($1,$2,$3,$4,'timeout', now())`, uuid.Must(uuid.NewV7()), org, routeID, missed); err != nil {
		t.Fatalf("seed reservation: %v", err)
	}
	if _, err := q.EnqueueRouteForMatch(ctx, generated.EnqueueRouteForMatchParams{
		RouteRequestID: pgUUID(routeID), OrgID: pgUUID(org), RequiredSkills: []string{}, ResumeCursor: []byte("{}"),
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// The missed agent must NOT be able to claim it.
	if _, err := q.ClaimWaitingRoute(ctx, generated.ClaimWaitingRouteParams{
		OrgID: pgUUID(org), Column2: []string{}, Column3: pgUUID(missed), Column4: 100, Column5: 1,
	}); err != pgx.ErrNoRows {
		t.Fatalf("excluded (timed-out) agent claim err=%v, want ErrNoRows", err)
	}
	// A different agent CAN.
	if _, err := q.ClaimWaitingRoute(ctx, generated.ClaimWaitingRouteParams{
		OrgID: pgUUID(org), Column2: []string{}, Column3: pgUUID(uuid.Must(uuid.NewV7())), Column4: 100, Column5: 1,
	}); err != nil {
		t.Fatalf("fresh agent claim: %v", err)
	}
}

// TestMatcher_ConcurrentClaimExactlyOne: many agents racing for ONE waiting route
// → exactly one claims it (FOR UPDATE OF rr SKIP LOCKED + the status flip).
func TestMatcher_ConcurrentClaimExactlyOne(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	ctx := context.Background()
	org := uuid.Must(uuid.NewV7())
	routeID := seedRunningRoute(ctx, t, org)
	if _, err := generated.New(sharedPool).EnqueueRouteForMatch(ctx, generated.EnqueueRouteForMatchParams{RouteRequestID: pgUUID(routeID), OrgID: pgUUID(org), RequiredSkills: []string{}, ResumeCursor: []byte("{}")}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	const agents = 12
	var wg sync.WaitGroup
	won := make([]bool, agents)
	for i := 0; i < agents; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tx, err := sharedPool.Begin(ctx)
			if err != nil {
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()
			_, err = generated.New(tx).ClaimWaitingRoute(ctx, generated.ClaimWaitingRouteParams{
				OrgID: pgUUID(org), Column2: []string{}, Column3: pgUUID(uuid.Must(uuid.NewV7())), Column4: 100, Column5: 1,
			})
			if err == nil {
				if cErr := tx.Commit(ctx); cErr == nil {
					won[i] = true
				}
			}
		}(i)
	}
	wg.Wait()
	n := 0
	for _, w := range won {
		if w {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d agents claimed the route, want exactly 1 (fenced claim)", n)
	}
}

func TestRequiredSkillsFromTrace(t *testing.T) {
	// Only the match_skill steps that ACTUALLY ran contribute (path-accurate) —
	// a skill on a NOT-taken branch must not over-constrain the matcher.
	tr := runtime.Trace{Steps: []runtime.TraceStep{
		{NodeID: "t", Kind: runtime.NodeTrigger},
		{NodeID: "s1", Kind: runtime.NodeMatchSkill, Output: map[string]any{"skill": "skill_es"}},
		{NodeID: "s1b", Kind: runtime.NodeMatchSkill, Output: map[string]any{"skill": "skill_es"}}, // dup
		{NodeID: "r", Kind: runtime.NodeReservation},
	}}
	got := requiredSkillsFromTrace(tr)
	if len(got) != 1 || got[0] != "skill_es" {
		t.Fatalf("required skills = %v, want [skill_es] (deduped, path-only)", got)
	}
}
