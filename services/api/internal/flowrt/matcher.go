package flowrt

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// matcher.go is the v0.3 W4 availability-driven pull: for each available agent
// (Ready + leased-connected + routable + under-capacity) claim the best-ranked
// eligible waiting route and attach an offer. The interaction-driven side
// (enqueue on no available agent) lives in route_lifecycle.go; this is the other
// half of the bidirectional matcher.
//
// Ranking is computed in SQL (ClaimWaitingRoute) so uncapped aging can cross a
// priority band — a sufficiently-aged low-priority route overtakes a fresh
// high-priority one (no bounded-prefix starvation, plan rev2 BLOCK). The weights:
// effective = priority*W_p + age_seconds*W_age, deterministic id tie-break.
const (
	matcherAgentBatch    = 100
	matcherOrgBatch      = 100
	matcherExpiredBatch  = 100
	matcherWeightPrio    = 100.0
	matcherWeightAge     = 1.0
	matcherOfferExpiry   = 20 * time.Second // RONA ring window before the timeout sweep re-queues
	staleOfferingTimeout = 30 * time.Second // an 'offering' route older than this lost its worker → re-token
)

// RunMatcher is the cross-org matcher tick (cmd/runtime, alongside the
// continuation worker): (1) re-token routes stuck 'offering' from a crashed
// worker, (2) give up on SLA-expired waiting_match routes → no_candidate
// fallback, (3) pull-offer per org with pending demand. The sweeps run on the raw
// pool (cross-org); per-org/per-route work re-scopes through the OrgDB. Returns
// offers attached this tick.
func (e *Endpoints) RunMatcher(ctx context.Context, pool *pgxpool.Pool, matcherInstance string, now time.Time) (int, error) {
	rawq := generated.New(pool)

	// 1. Stale-offering recovery: re-token so the crashed worker's CommitMatchOffer
	//    (fenced on the old token) can't land; the route returns to waiting_match.
	if _, err := rawq.SweepStaleOffering(ctx, ts(now.Add(-staleOfferingTimeout))); err != nil {
		return 0, err
	}

	// 2. SLA deadline: a route nobody could match before its match_deadline gives up
	//    → resume the reservation node's no_candidate port (fallback). Per route in
	//    its own org-scoped tx so the run-lock flip + resume are atomic.
	expired, err := rawq.ListExpiredMatchRoutes(ctx, matcherExpiredBatch)
	if err != nil {
		return 0, err
	}
	for _, r := range expired {
		if fErr := e.fallbackExpiredRoute(ctx, apiUUID(r.OrgID), apiUUID(r.ID)); fErr != nil {
			e.deps.Logger.ErrorContext(ctx, "matcher SLA fallback failed", "org_id", apiUUID(r.OrgID), "route_id", apiUUID(r.ID), "err", fErr)
		}
	}

	// 3. Availability-driven pull, per org with a waiting route.
	orgs, err := rawq.ListOrgsWithWaitingMatch(ctx, matcherOrgBatch)
	if err != nil {
		return 0, err
	}
	offered := 0
	for _, o := range orgs {
		n, cErr := e.RunMatchCycle(ctx, apiUUID(o), matcherInstance)
		if cErr != nil {
			e.deps.Logger.ErrorContext(ctx, "match cycle failed", "org_id", apiUUID(o), "err", cErr)
			continue
		}
		offered += n
	}
	return offered, nil
}

// fallbackExpiredRoute flips ONE SLA-expired route to running (fenced) and resumes
// it with no_candidate, atomically. A losing race / already-handled route is a
// benign no-op (ErrNoRows).
func (e *Endpoints) fallbackExpiredRoute(ctx context.Context, orgID, routeID uuid.UUID) error {
	ctx = orgkey.SetOrgID(ctx, orgID)
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)
	route, err := qtx.AcquireExpiredMatchRouteForRun(ctx, generated.AcquireExpiredMatchRouteForRunParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	e.appendEvent(ctx, qtx, orgID, routeID, "route.match_deadline_expired", nil)
	// no_candidate is consumed by liveReservation BEFORE its matcher-mode park, so
	// the route takes the reservation node's no_candidate port → fallback → terminal.
	if err := e.resumeRoute(ctx, tx, qtx, orgID, route, "no_candidate"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RunMatchCycle runs one availability-driven pass for a single org: list the
// available agents, then claim+offer the best eligible waiting route to each.
// Returns the number of offers attached. Org-scoped — the caller (cmd/runtime)
// drives it per org with a waiting_match queue. Presence (the connection lease)
// is fail-closed: a presence error skips the cycle rather than offering to an
// agent who may be gone. No wall-clock param: ranking/claims use Postgres now(),
// and each offer's hold expiry is stamped from time.Now() at the offer itself
// (not cycle start) so a long batch can't backdate later holds (review HIGH).
func (e *Endpoints) RunMatchCycle(ctx context.Context, orgID uuid.UUID, matcherInstance string) (int, error) {
	ctx = orgkey.SetOrgID(ctx, orgID)
	q := generated.New(e.deps.OrgDB)
	agents, err := q.ListAvailableAgentsForMatch(ctx, generated.ListAvailableAgentsForMatchParams{
		OrgID: pgUUID(orgID), Limit: matcherAgentBatch,
	})
	if err != nil {
		return 0, err
	}
	if len(agents) == 0 {
		return 0, nil
	}
	if len(agents) == matcherAgentBatch {
		// No silent caps: a full batch means more available agents than we considered
		// this tick — the rest are picked up next tick (longest-idle ordering is stable).
		e.deps.Logger.WarnContext(ctx, "matcher agent batch truncated", "org_id", orgID, "cap", matcherAgentBatch)
	}

	// Filter to leased-connected agents (the live offerability gate). An offer to a
	// disconnected agent would ring nobody and burn a RONA attempt.
	connected := map[uuid.UUID]bool{}
	if e.deps.Presence != nil {
		ids := make([]uuid.UUID, len(agents))
		for i, a := range agents {
			ids[i] = apiUUID(a.AgentID)
		}
		connected, err = e.deps.Presence.ConnectedMany(ctx, orgID, ids)
		if err != nil {
			return 0, err
		}
	}

	offered := 0
	for _, a := range agents {
		agentID := apiUUID(a.AgentID)
		if e.deps.Presence != nil && !connected[agentID] {
			continue
		}
		ok, mErr := e.tryOfferToAgent(ctx, orgID, matcherInstance, agentID, a.AgentCode, a.Skills)
		if mErr != nil {
			// One agent's offer failure (infra) must not abort the whole cycle — log
			// and move on so other agents still get matched this tick.
			e.deps.Logger.ErrorContext(ctx, "matcher offer failed", "org_id", orgID, "agent_id", agentID, "err", mErr)
			continue
		}
		if ok {
			offered++
		}
	}
	return offered, nil
}

// tryOfferToAgent claims the best eligible waiting route for one agent and
// attaches an offer, all in one tx (claim → slot → reservation → commit →
// continuation → audit). The lock order is GLOBAL — route-claim (FOR UPDATE OF
// rr) first, then the capacity slot (SKIP LOCKED), then the reservation — the
// same order the inline offer and accept paths use, so the matcher can't deadlock
// against them (plan rev2 R-lockorder).
func (e *Endpoints) tryOfferToAgent(ctx context.Context, orgID uuid.UUID, matcherInstance string, agentID uuid.UUID, agentCode string, skills []string) (bool, error) {
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	claimed, err := qtx.ClaimWaitingRoute(ctx, generated.ClaimWaitingRouteParams{
		OrgID: pgUUID(orgID), Column2: skills, Column3: pgUUID(agentID),
		Column4: matcherWeightPrio, Column5: matcherWeightAge,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // no eligible waiting route for this agent
	}
	if err != nil {
		return false, err
	}
	routeID := apiUUID(claimed.ID)
	token := claimed.MatchOfferToken

	resID := uuid.Must(uuid.NewV7())
	// Stamp the hold from NOW (this offer), not the cycle start — a batch that takes
	// seconds must not backdate later agents' holds into the past (review HIGH).
	exp := time.Now().Add(matcherOfferExpiry)
	// Clamp the offer (and its timeout) to the queue SLA — an offer must never
	// outlive the deadline at which the SLA sweep gives up to fallback (R-clamp).
	// ClaimWaitingRoute already excluded routes past match_deadline, so exp > now.
	if claimed.MatchDeadline.Valid && claimed.MatchDeadline.Time.Before(exp) {
		exp = claimed.MatchDeadline.Time
	}

	var slotNo *int32
	if e.deps.Capacity != nil {
		if err := e.deps.Capacity.ProvisionInTx(ctx, qtx, orgID, agentID, claimed.Channel); err != nil {
			return false, err
		}
		slot, ok, aErr := e.deps.Capacity.AcquireInTx(ctx, qtx, orgID, agentID, claimed.Channel, resID, exp)
		if aErr != nil {
			return false, aErr
		}
		if !ok {
			// At capacity — return the route to the queue (token-fenced) and record
			// the miss. Committing the requeue (vs rolling back) keeps the audit row.
			return false, e.requeueAfterFailedOffer(ctx, tx, qtx, orgID, matcherInstance, claimed, agentID, "capacity_lost")
		}
		slotNo = &slot
	}

	leaseToken, sessionID, err := bindOfferLease(ctx, qtx, orgID, agentID)
	if err != nil {
		return false, err
	}
	if _, err := qtx.InsertReservationOffer(ctx, generated.InsertReservationOfferParams{
		ID: pgUUID(resID), OrgID: pgUUID(orgID), RouteRequestID: pgUUID(routeID),
		// +1: the claim no longer bumps match_attempt_seq (CommitMatchOffer does, on
		// success only), so this offer's attempt number is the next one.
		AgentID: pgUUID(agentID), Attempt: claimed.MatchAttemptSeq + 1,
		ExpiresAt:  pgtype.Timestamptz{Time: exp, Valid: true},
		LeaseToken: pgUUID(leaseToken), AgentSessionID: sessionID,
	}); err != nil {
		return false, err
	}

	// Token-fenced commit: a sweep that re-queued this offering route between the
	// claim and here would have changed the token → ErrNoRows → roll back (the slot
	// + reservation go with it) and record route_lost.
	committed, err := qtx.CommitMatchOffer(ctx, generated.CommitMatchOfferParams{
		ID: pgUUID(routeID), OrgID: pgUUID(orgID), MatchOfferToken: token,
		ActiveReservationID: pgUUID(resID), ResumeCursor: claimed.ResumeCursor,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_ = tx.Rollback(ctx)
		return false, e.recordDecisionOwnTx(ctx, orgID, matcherInstance, claimed, agentID, "route_lost")
	}
	if err != nil {
		return false, err
	}

	// Arm the reservation timeout (RONA). run_seq is taken from the just-committed
	// row so a stale timer from a prior attempt is fenced by AcquireRouteForRunAtSeq.
	if _, err := qtx.InsertContinuation(ctx, generated.InsertContinuationParams{
		ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(orgID), Kind: "reservation_timeout",
		RouteRequestID: pgUUID(routeID), ReservationID: pgUUID(resID),
		FlowVersionID: claimed.FlowVersionID, Cursor: []byte("{}"), RunSeq: committed.RunSeq,
		DueAt: pgtype.Timestamptz{Time: exp, Valid: true},
	}); err != nil {
		return false, err
	}

	if err := enqueueOfferFrame(ctx, qtx, orgID, agentID, resID, leaseToken, exp); err != nil {
		if errors.Is(err, errAgentVanished) {
			return false, nil // agent gone since the claim → drop the offer (tx rolls back → route stays waiting_match)
		}
		return false, err
	}
	if err := e.recordDecision(ctx, qtx, orgID, matcherInstance, claimed, agentID, "offered", agentCode, skills, slotNo); err != nil {
		return false, err
	}
	e.appendEvent(ctx, qtx, orgID, routeID, "reservation.offered", map[string]any{
		"reservation_id": resID.String(), "agent_id": agentID.String(), "source": "matcher",
	})
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// requeueAfterFailedOffer returns a claimed route to the queue inside the SAME tx
// (the claim's offering flip is undone by the token-fenced ReturnRouteToQueue) and
// commits, so the failure is auditable. Used for pre-commit failures where the tx
// has no reservation/slot yet to roll back (capacity at cap).
func (e *Endpoints) requeueAfterFailedOffer(ctx context.Context, tx *db.OrgTx, qtx *generated.Queries, orgID uuid.UUID, matcherInstance string, claimed generated.RouteRequest, agentID uuid.UUID, outcome string) error {
	if _, err := qtx.ReturnRouteToQueue(ctx, generated.ReturnRouteToQueueParams{
		ID: claimed.ID, OrgID: pgUUID(orgID), MatchOfferToken: claimed.MatchOfferToken,
	}); err != nil {
		return err
	}
	if err := e.recordDecision(ctx, qtx, orgID, matcherInstance, claimed, agentID, outcome, "", nil, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// recordDecisionOwnTx writes a decision audit row in a fresh tx — for the case
// where the offer tx already rolled back (route_lost) so its qtx is unusable.
func (e *Endpoints) recordDecisionOwnTx(ctx context.Context, orgID uuid.UUID, matcherInstance string, claimed generated.RouteRequest, agentID uuid.UUID, outcome string) error {
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := e.recordDecision(ctx, generated.New(tx), orgID, matcherInstance, claimed, agentID, outcome, "", nil, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (e *Endpoints) recordDecision(ctx context.Context, qtx *generated.Queries, orgID uuid.UUID, matcherInstance string, claimed generated.RouteRequest, agentID uuid.UUID, outcome, agentCode string, skills []string, slotNo *int32) error {
	detail, _ := json.Marshal(map[string]any{
		"agent_code":      agentCode,
		"agent_skills":    skills,
		"required_skills": claimed.RequiredSkills,
		"priority":        claimed.Priority,
		"weight_priority": matcherWeightPrio,
		"weight_age":      matcherWeightAge,
	})
	return qtx.InsertRouteDecision(ctx, generated.InsertRouteDecisionParams{
		ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(orgID), RouteRequestID: claimed.ID,
		DecisionType: "availability_pull", MatcherInstance: matcherInstance, Channel: claimed.Channel,
		QueueID: claimed.QueueID, SelectedAgentID: pgUUID(agentID), SelectedSlotNo: slotNo, Outcome: outcome, Detail: detail,
	})
}
