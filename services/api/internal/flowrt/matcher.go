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

	"github.com/luongdev/open-routing/services/api/internal/adapter"
	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/metrics"
)

// matcher.go is the availability-driven pull: for each available agent (Ready +
// leased-connected + routable + under-capacity) claim the best-ranked eligible
// waiting route and attach an offer. The interaction-driven side (enqueue on no
// available agent) lives in route_lifecycle.go; this is the other half of the
// bidirectional matcher.
//
// Ranking is computed in SQL (ClaimWaitingRoute) so uncapped aging can cross a
// priority band — a sufficiently-aged low-priority route overtakes a fresh
// high-priority one (no bounded-prefix starvation). The weights:
// effective = priority*W_p + age_seconds*W_age, deterministic id tie-break.
const (
	matcherBatchDefault  = 100
	matcherWeightPrio    = 100.0
	matcherWeightAge     = 1.0
	matcherOfferExpiry   = 20 * time.Second // RONA ring window before the timeout sweep re-queues
	staleOfferingTimeout = 30 * time.Second // an 'offering' route older than this lost its worker → re-token
	// reclaimGrace is how long an agent may go unseen (no session heartbeat) while
	// holding an accepted call before the slot is reclaimed and the call torn down.
	// Aligned with presence.DefaultTTL (90s) so a normal heartbeat cadence + one
	// missed beat never trips it — only a genuine mid-call vanish does.
	reclaimGrace = 90 * time.Second
	// maxReassignHops bounds how many times one interaction is re-matched after a
	// mid-call agent drop before the flow's no_candidate fallback gives up — so a
	// serially-dropping call can't loop forever.
	maxReassignHops      = 3
	reassignMatchTimeout = 30 * time.Second // SLA window for a reassignment hop to find a new agent
)

// matcherBatch is the per-tick cap on agents/orgs/expired routes, configurable via
// Deps.MatcherBatch (0 ⇒ default). A full batch is logged, not silently dropped.
func (e *Endpoints) matcherBatch() int32 {
	if e.deps.MatcherBatch > 0 {
		return int32(e.deps.MatcherBatch) //nolint:gosec // operator-configured, small
	}
	return matcherBatchDefault
}

// RunMatcher is the cross-org matcher tick (cmd/runtime, alongside the
// continuation worker): (1) re-token routes stuck 'offering' from a crashed
// worker, (2) give up on SLA-expired waiting_match routes → no_candidate
// fallback, (3) pull-offer per org with pending demand. The sweeps run on the raw
// pool (cross-org); per-org/per-route work re-scopes through the OrgDB. Returns
// offers attached this tick.
func (e *Endpoints) RunMatcher(ctx context.Context, pool *pgxpool.Pool, matcherInstance string, now time.Time) (int, error) {
	rawq := generated.New(pool)

	// 0. Confirmed-slot reclaim: an agent who vanished mid-call (unseen on any
	//    session past reclaimGrace) leaves a reservation stuck 'accepted' holding a
	//    capacity slot forever. Tear the route down + free the slot so capacity and
	//    the interaction aren't lost. Auto-reassign to another agent is adapter/
	//    media-coupled (v0.4) — v0.3 abandons the dropped call. The cutoff captured
	//    here is reused by each per-route fence so a reconnect after this point wins.
	reclaimCutoff := ts(now.Add(-reclaimGrace))
	stale, err := rawq.ListReclaimableAcceptedReservations(ctx, generated.ListReclaimableAcceptedReservationsParams{
		LastSeenAt: reclaimCutoff, Limit: e.matcherBatch(),
	})
	if err != nil {
		return 0, err
	}
	for _, s := range stale {
		if rErr := e.reclaimAbandonedCall(ctx, apiUUID(s.OrgID), apiUUID(s.RouteRequestID), apiUUID(s.ID), reclaimCutoff); rErr != nil {
			e.deps.Logger.ErrorContext(ctx, "confirmed-slot reclaim failed", "org_id", apiUUID(s.OrgID), "route_id", apiUUID(s.RouteRequestID), "reservation_id", apiUUID(s.ID), "err", rErr)
		}
	}

	// 1. Stale-offering recovery: re-token so the crashed worker's CommitMatchOffer
	//    (fenced on the old token) can't land; the route returns to waiting_match.
	if _, err := rawq.SweepStaleOffering(ctx, ts(now.Add(-staleOfferingTimeout))); err != nil {
		return 0, err
	}

	// 2. SLA deadline: a route nobody could match before its match_deadline gives up
	//    → resume the reservation node's no_candidate port (fallback). Per route in
	//    its own org-scoped tx so the run-lock flip + resume are atomic.
	expired, err := rawq.ListExpiredMatchRoutes(ctx, e.matcherBatch())
	if err != nil {
		return 0, err
	}
	for _, r := range expired {
		if fErr := e.fallbackExpiredRoute(ctx, apiUUID(r.OrgID), apiUUID(r.ID)); fErr != nil {
			e.deps.Logger.ErrorContext(ctx, "matcher SLA fallback failed", "org_id", apiUUID(r.OrgID), "route_id", apiUUID(r.ID), "err", fErr)
		}
	}

	// 3. Availability-driven pull, per org with a waiting route.
	orgs, err := rawq.ListOrgsWithWaitingMatch(ctx, e.matcherBatch())
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

// reclaimAbandonedCall frees the stuck capacity slot of ONE call whose agent
// vanished. The fenced flip (still-accepted AND still-unseen) makes it a no-op if
// the agent reconnected or the call completed between discovery and here, so a
// momentary blip never kills a live call. It is reservation-scoped, not route-
// scoped: a route can already be terminal while its accepted reservation still
// holds the live call's slot, so we release the slot + move the (gone) agent out
// of Engaged regardless of route state, and additionally tear the route down only
// if it is still live (a parked call-handling route can't continue agent-less).
// Auto-reassign to another agent is adapter/media-coupled → v0.4.
func (e *Endpoints) reclaimAbandonedCall(ctx context.Context, orgID, routeID, resID uuid.UUID, cutoff pgtype.Timestamptz) error {
	ctx = orgkey.SetOrgID(ctx, orgID)
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	// Lock the parent route row FIRST (top-down order: route → reservation →
	// agent_state) so a concurrent caller-abandon teardown on the same route can't
	// AB-BA deadlock us. This only locks — the conditional reservation flip below
	// is still the authority on whether to reclaim (cross-AI review HIGH).
	locked, err := qtx.LockRouteForReclaim(ctx, generated.LockRouteForReclaimParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // route vanished
	}
	if err != nil {
		return err
	}

	rec, err := qtx.ReclaimStaleAcceptedReservation(ctx, generated.ReclaimStaleAcceptedReservationParams{
		ID: pgUUID(resID), OrgID: pgUUID(orgID), LastSeenAt: cutoff,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // reservation resolved or agent returned → no-op
	}
	if err != nil {
		return err
	}

	if e.deps.Capacity != nil {
		if rErr := e.deps.Capacity.ReleaseInTx(ctx, qtx, orgID, resID); rErr != nil {
			return rErr
		}
	}
	wrapUp := string(api.AgentStatusWrapUp)
	until := pgtype.Timestamptz{Time: time.Now().Add(wrapUpSeconds * time.Second), Valid: true}
	if _, err := qtx.UpdateAgentStateStatus(ctx, generated.UpdateAgentStateStatusParams{
		AgentID: rec.AgentID, OrgID: pgUUID(orgID), ToStatus: &wrapUp, ExpectedFrom: string(api.AgentStatusEngaged), WrapupUntil: until,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// Tear the route down if still live (the call can't continue agent-less). The
	// route row is already locked above, so this adds no new lock-order edge; a
	// terminal route is a benign no-op (ErrNoRows). Channel for the adapter release
	// comes from the lock regardless of route state.
	if _, aErr := qtx.AbandonRoute(ctx, generated.AbandonRouteParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID)}); aErr != nil && !errors.Is(aErr, pgx.ErrNoRows) {
		return aErr
	}
	channel := locked.Channel
	e.appendEvent(ctx, qtx, orgID, routeID, "route.agent_lost", map[string]any{
		"reservation_id": resID.String(), "agent_id": apiUUID(rec.AgentID).String(),
	})
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	var handles []string
	if rec.AdapterHandle != nil && *rec.AdapterHandle != "" {
		handles = []string{*rec.AdapterHandle}
	}
	e.releaseAssignments(ctx, channel, handles, adapter.ReleaseCancelled)
	metrics.MatcherReclaims.Inc()
	return nil
}

// reassignRoute handles a mid-call agent drop (adapter `disconnected`/`rejected`):
// cancel the dropped accepted reservation, free its slot, move the dropped agent
// out of Engaged, then — if under the hop cap — re-queue the interaction to
// waiting_match for a fresh match to ANOTHER agent (the matcher excludes everyone
// who already failed this interaction); past the cap, give up and tear the route
// down. The dropped call's media handle is released post-commit. This completes the
// v0.3 reclaim path: reclaim WITHOUT abandon — re-queue instead (v0.4 W4).
func (e *Endpoints) reassignRoute(ctx context.Context, orgID, routeID, resID uuid.UUID) error {
	octx := orgkey.SetOrgID(ctx, orgID)
	tx, err := e.deps.OrgDB.BeginTx(octx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(octx) }()
	qtx := generated.New(tx)

	// Fence + cancel the dropped reservation (still 'accepted' → the live call). 0
	// rows ⇒ it already resolved (raced complete) → no-op.
	rec, err := qtx.ReassignStaleReservation(octx, generated.ReassignStaleReservationParams{ID: pgUUID(resID), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if e.deps.Capacity != nil {
		if rErr := e.deps.Capacity.ReleaseInTx(octx, qtx, orgID, resID); rErr != nil {
			return rErr
		}
	}
	// Move the dropped agent out of Engaged (it's disconnected; presence already
	// excludes it from re-offers — this just clears its state).
	wrapUp := string(api.AgentStatusWrapUp)
	until := pgtype.Timestamptz{Time: time.Now().Add(wrapUpSeconds * time.Second), Valid: true}
	if _, uErr := qtx.UpdateAgentStateStatus(octx, generated.UpdateAgentStateStatusParams{
		AgentID: rec.AgentID, OrgID: pgUUID(orgID), ToStatus: &wrapUp, ExpectedFrom: string(api.AgentStatusEngaged), WrapupUntil: until,
	}); uErr != nil && !errors.Is(uErr, pgx.ErrNoRows) {
		return uErr
	}

	route, err := qtx.GetRouteRequest(octx, generated.GetRouteRequestParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID)})
	if err != nil {
		return err
	}
	event := "route.reassigned"
	if route.ReassignCount >= maxReassignHops {
		// Exhausted: give up — abandon the route (the slot is already freed above).
		if _, aErr := qtx.AbandonRoute(octx, generated.AbandonRouteParams{ID: pgUUID(routeID), OrgID: pgUUID(orgID)}); aErr != nil && !errors.Is(aErr, pgx.ErrNoRows) {
			return aErr
		}
		event = "route.reassign_exhausted"
	} else if _, rErr := qtx.ReassignRouteForMatch(octx, generated.ReassignRouteForMatchParams{
		RouteRequestID: pgUUID(routeID), OrgID: pgUUID(orgID), MatchDeadline: ts(time.Now().Add(reassignMatchTimeout)),
	}); rErr != nil && !errors.Is(rErr, pgx.ErrNoRows) {
		return rErr
	}
	e.appendEvent(octx, qtx, orgID, routeID, event, map[string]any{
		"agent_id": apiUUID(rec.AgentID).String(), "reassign_count": int(route.ReassignCount),
	})
	if err := tx.Commit(octx); err != nil {
		return err
	}
	if rec.AdapterHandle != nil && *rec.AdapterHandle != "" {
		e.releaseAssignments(octx, route.Channel, []string{*rec.AdapterHandle}, adapter.ReleaseCancelled)
	}
	metrics.MatcherReassigns.Inc()
	return nil
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
		OrgID: pgUUID(orgID), Limit: e.matcherBatch(),
	})
	if err != nil {
		return 0, err
	}
	if len(agents) == 0 {
		return 0, nil
	}
	if len(agents) == int(e.matcherBatch()) {
		// No silent caps: a full batch means more available agents than we considered
		// this tick — the rest are picked up next tick (longest-idle ordering is stable).
		e.deps.Logger.WarnContext(ctx, "matcher agent batch truncated", "org_id", orgID, "cap", e.matcherBatch())
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
// against them.
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

	// Stamp the hold from NOW (this offer), not the cycle start — a batch that takes
	// seconds must not backdate later agents' holds into the past (review HIGH).
	// Clamp to the queue SLA: an offer must never outlive the deadline at which the
	// SLA sweep gives up to fallback. ClaimWaitingRoute already excluded routes past
	// match_deadline, so exp > now.
	exp := time.Now().Add(matcherOfferExpiry)
	if claimed.MatchDeadline.Valid && claimed.MatchDeadline.Time.Before(exp) {
		exp = claimed.MatchDeadline.Time
	}

	// Attempt = claimed.MatchAttemptSeq+1: the claim no longer bumps the seq
	// (CommitMatchOffer does, on success only), so this is the next attempt number.
	att, capOK, err := attachOffer(ctx, qtx, e.deps.Capacity, orgID, routeID, agentID, claimed.Channel, claimed.MatchAttemptSeq+1, exp)
	if err != nil {
		// Agent vanished or already reserved on this route → drop the offer; the
		// deferred tx rollback undoes the claim so the route stays waiting_match.
		if errors.Is(err, errAgentVanished) || errors.Is(err, errOfferBusy) {
			return false, nil
		}
		return false, err
	}
	if !capOK {
		// At capacity — return the route to the queue (token-fenced) + audit the miss.
		// Committing the requeue (vs rolling back) keeps the decision row.
		return false, e.requeueAfterFailedOffer(ctx, tx, qtx, orgID, matcherInstance, claimed, agentID, "capacity_lost")
	}

	// Token-fenced commit: a sweep that re-queued this offering route between the
	// claim and here would have changed the token → ErrNoRows → roll back (the slot
	// + reservation + frame go with it) and record route_lost.
	committed, err := qtx.CommitMatchOffer(ctx, generated.CommitMatchOfferParams{
		ID: pgUUID(routeID), OrgID: pgUUID(orgID), MatchOfferToken: token,
		ActiveReservationID: pgUUID(att.resID), ResumeCursor: claimed.ResumeCursor,
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
		RouteRequestID: pgUUID(routeID), ReservationID: pgUUID(att.resID),
		FlowVersionID: claimed.FlowVersionID, Cursor: []byte("{}"), RunSeq: committed.RunSeq,
		DueAt: ts(exp),
	}); err != nil {
		return false, err
	}

	if err := e.recordDecision(ctx, qtx, orgID, matcherInstance, claimed, agentID, "offered", agentCode, skills, att.slotNo); err != nil {
		return false, err
	}
	e.appendEvent(ctx, qtx, orgID, routeID, "reservation.offered", map[string]any{
		"reservation_id": att.resID.String(), "agent_id": agentID.String(), "source": "matcher",
	})
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	metrics.MatcherOffers.Inc()
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
