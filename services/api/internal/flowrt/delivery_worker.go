package flowrt

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// delivery_worker.go is the v0.4 Wave 1 durable delivery drain: it closes the v0.3
// gap where the post-commit in-process Deliver could be lost on a crash between
// accept-commit and the adapter handoff. The accept path now commits a
// delivery_commands row in the SAME tx as the reservation flip; this worker (run
// on the cmd/runtime tick) claims due rows, calls the channel adapter, binds the
// returned handle, and marks the row — at-least-once, idempotent at the adapter by
// the command id (delivery_attempt_id).

const deliveryClaimLease = 30 * time.Second

// maxDeliveryAttempts bounds Deliver retries: a transient adapter outage (adapter
// down) should NOT tear the route down on the first failure — the command is left
// for the claim lease to expire and retry. Only after this many attempts is the
// delivery declared failed and the route torn down (cross-AI review: don't
// endlessly claim+fail, but don't abandon a call over a blip either).
const maxDeliveryAttempts = 5

func (e *Endpoints) deliveryBatch() int32 {
	if e.deps.MatcherBatch > 0 {
		return int32(e.deps.MatcherBatch) //nolint:gosec // operator-configured, small
	}
	return matcherBatchDefault
}

// DrainDeliveries claims and dispatches a batch of due delivery commands. Returns
// the number delivered. Cross-org (raw pool); each delivery re-scopes under its
// org. Safe across replicas (FOR UPDATE SKIP LOCKED + claim lease).
func (e *Endpoints) DrainDeliveries(ctx context.Context, pool *pgxpool.Pool, workerID string, now time.Time) (int, error) {
	rows, err := generated.New(pool).ClaimDueDeliveryCommands(ctx, generated.ClaimDueDeliveryCommandsParams{
		ClaimedAt:      ts(now),
		ClaimExpiresAt: ts(now.Add(deliveryClaimLease)),
		ClaimedBy:      &workerID,
		Limit:          e.deliveryBatch(),
	})
	if err != nil {
		return 0, err
	}
	delivered := 0
	for _, r := range rows {
		if e.dispatchDelivery(ctx, r, workerID) {
			delivered++
		}
	}
	return delivered, nil
}

// dispatchDelivery runs ONE claimed command: Deliver → (atomically, lease-fenced)
// bind the handle + mark delivered. workerID is the claim owner — the finalize
// fences on it so a worker whose lease was re-claimed by a peer cannot clobber the
// newer handle (cross-AI review BLOCK, multi-replica). On a Deliver fault it
// retries up to the cap, then fails + tears the route down. Returns true on a
// successful delivery this tick.
func (e *Endpoints) dispatchDelivery(ctx context.Context, cmd generated.ClaimDueDeliveryCommandsRow, workerID string) bool {
	orgID := apiUUID(cmd.OrgID)
	resID := apiUUID(cmd.ReservationID)
	routeID := apiUUID(cmd.RouteRequestID)
	ad, ok := e.adapterFor(cmd.Channel)
	if !ok {
		// Outbox is on but no adapter serves this channel — a misconfiguration. Do NOT
		// silently mark delivered (that strands the call with media-less + no signal):
		// fail the delivery + tear the route down so it surfaces (cross-AI review HIGH).
		octx := orgkey.SetOrgID(ctx, orgID)
		msg := "no adapter for channel " + cmd.Channel
		if rows, _ := generated.New(e.deps.OrgDB).MarkDeliveryFailed(octx, generated.MarkDeliveryFailedParams{ID: cmd.ID, OrgID: cmd.OrgID, LastError: &msg, ClaimedBy: &workerID}); rows > 0 {
			_ = e.failDelivery(octx, orgID, routeID)
		}
		e.deps.Logger.ErrorContext(octx, "delivery has no adapter for channel", "channel", cmd.Channel, "reservation_id", resID)
		return false
	}
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), adapterOpTimeout)
	defer cancel()
	octx := orgkey.SetOrgID(dctx, orgID)
	var interaction map[string]any
	_ = json.Unmarshal(cmd.Interaction, &interaction)

	h, err := ad.Deliver(dctx, adapter.Assignment{
		ReservationID: resID.String(), RouteRequestID: routeID.String(),
		AgentID: apiUUID(cmd.AgentID).String(), Channel: cmd.Channel,
		Interaction: interaction, IdempotencyKey: apiUUID(cmd.ID).String(),
	}, e)
	if err != nil {
		// Retry-with-cap: a transient adapter outage shouldn't tear the route down on
		// the first failure. Leave the row claimed so the lease expires and a later
		// tick retries (Deliver is idempotent by IdempotencyKey). Only past the cap
		// fail — and tear down FIRST, marking failed only if the teardown succeeded, so
		// a teardown failure can't strand the route 'accepted' (review MED).
		if cmd.AttemptCount < maxDeliveryAttempts {
			e.deps.Logger.WarnContext(octx, "delivery deliver failed → will retry", "reservation_id", resID, "channel", cmd.Channel, "attempt", cmd.AttemptCount, "err", err)
			return false
		}
		if tErr := e.failDelivery(octx, orgID, routeID); tErr != nil {
			return false // teardown failed → leave pending; retry both next tick
		}
		errMsg := err.Error()
		_, _ = generated.New(e.deps.OrgDB).MarkDeliveryFailed(octx, generated.MarkDeliveryFailedParams{ID: cmd.ID, OrgID: cmd.OrgID, LastError: &errMsg, ClaimedBy: &workerID})
		e.deps.Logger.ErrorContext(octx, "delivery deliver failed past retry cap → route torn down", "reservation_id", resID, "channel", cmd.Channel, "attempts", cmd.AttemptCount, "err", err)
		return false
	}
	handle := string(h)

	// Finalize atomically + lease-fenced in ONE tx: mark delivered (fenced on
	// claimed_by) THEN bind the handle. A stale worker gets 0 rows on the mark and
	// releases its orphan room instead of clobbering the peer's handle.
	tx, terr := e.deps.OrgDB.BeginTx(octx)
	if terr != nil {
		e.deps.Logger.ErrorContext(octx, "delivery finalize begin tx", "err", terr)
		return false
	}
	defer func() { _ = tx.Rollback(octx) }()
	qtx := generated.New(tx)
	marked, merr := qtx.MarkDeliveryDelivered(octx, generated.MarkDeliveryDeliveredParams{ID: cmd.ID, OrgID: cmd.OrgID, Handle: &handle, ClaimedBy: &workerID})
	if merr != nil {
		e.deps.Logger.ErrorContext(octx, "mark delivered failed", "reservation_id", resID, "err", merr)
		return false // retry; Deliver is idempotent by key
	}
	if marked == 0 {
		// Lost the claim (a peer re-claimed + finalized): our Deliver created an orphan
		// room → release it best-effort; the peer owns the binding.
		if relErr := ad.Release(octx, h, adapter.ReleaseCancelled); relErr != nil {
			e.deps.Logger.ErrorContext(octx, "release orphan (lost claim)", "handle", handle, "err", relErr)
		}
		return false
	}
	bound, berr := qtx.SetReservationAdapterHandle(octx, generated.SetReservationAdapterHandleParams{ID: pgUUID(resID), OrgID: cmd.OrgID, AdapterHandle: &handle})
	if berr != nil {
		e.deps.Logger.ErrorContext(octx, "bind adapter handle failed", "reservation_id", resID, "err", berr)
		return false // rollback (mark undone) → retry
	}
	if cErr := tx.Commit(octx); cErr != nil {
		e.deps.Logger.ErrorContext(octx, "delivery finalize commit", "err", cErr)
		return false
	}
	if bound == 0 {
		// Reservation went terminal during Deliver → the handle is an orphan the
		// teardown didn't know about; release it best-effort.
		if relErr := ad.Release(octx, h, adapter.ReleaseCancelled); relErr != nil {
			e.deps.Logger.ErrorContext(octx, "release orphan (reservation terminal)", "handle", handle, "err", relErr)
		}
	}
	return true
}

// enqueueDelivery commits a durable delivery command in the caller's tx (the same
// tx as the accept), so the delivery survives a crash before the adapter handoff.
// Idempotent on (org, reservation): a deduped/retried accept enqueues at most once.
func (e *Endpoints) enqueueDelivery(ctx context.Context, q *generated.Queries, orgID, resID, routeID, agentID uuid.UUID, channel string, interaction []byte) error {
	if interaction == nil {
		interaction = []byte("{}")
	}
	_, err := q.AppendDeliveryCommand(ctx, generated.AppendDeliveryCommandParams{
		ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(orgID), ReservationID: pgUUID(resID),
		RouteRequestID: pgUUID(routeID), AgentID: pgUUID(agentID), Channel: channel, Interaction: interaction,
	})
	return err
}
