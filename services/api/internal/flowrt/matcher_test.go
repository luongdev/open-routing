package flowrt

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/luongdev/open-routing/services/api/internal/db/generated"
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
	if _, err := q.EnqueueRouteForMatch(ctx, generated.EnqueueRouteForMatchParams{
		ID: pgUUID(routeID), OrgID: pgUUID(org), Priority: 5,
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
	_, _ = q.EnqueueRouteForMatch(ctx, generated.EnqueueRouteForMatchParams{ID: pgUUID(other), OrgID: pgUUID(org), RequiredSkills: []string{"skill_es"}, ResumeCursor: []byte("{}")})
	if _, err := q.ClaimWaitingRoute(ctx, generated.ClaimWaitingRouteParams{
		OrgID: pgUUID(org), Column2: []string{"skill_fr"}, Column3: pgUUID(uuid.Must(uuid.NewV7())), Column4: 100, Column5: 1,
	}); err != pgx.ErrNoRows {
		t.Fatalf("ineligible claim err=%v, want ErrNoRows", err)
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
	if _, err := generated.New(sharedPool).EnqueueRouteForMatch(ctx, generated.EnqueueRouteForMatchParams{
		ID: pgUUID(routeID), OrgID: pgUUID(org), RequiredSkills: []string{}, ResumeCursor: []byte("{}"),
	}); err != nil {
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
