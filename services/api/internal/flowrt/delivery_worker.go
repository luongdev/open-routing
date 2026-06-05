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
		if e.dispatchDelivery(ctx, r) {
			delivered++
		}
	}
	return delivered, nil
}

// dispatchDelivery runs ONE claimed command: Deliver → bind handle + mark
// delivered; on a Deliver fault, mark failed + tear the route down (the agent +
// caller would otherwise be stranded on a dead call). Mirrors deliverAssignment's
// handling but is driven by the durable row instead of an inline post-commit call.
func (e *Endpoints) dispatchDelivery(ctx context.Context, cmd generated.ClaimDueDeliveryCommandsRow) bool {
	orgID := apiUUID(cmd.OrgID)
	resID := apiUUID(cmd.ReservationID)
	routeID := apiUUID(cmd.RouteRequestID)
	ad, ok := e.adapterFor(cmd.Channel)
	if !ok {
		// No adapter for the channel (e.g. the HTTP/WS test-double path) — nothing to
		// deliver; mark it delivered so it isn't re-claimed forever.
		_, _ = generated.New(e.deps.OrgDB).MarkDeliveryDelivered(orgkey.SetOrgID(ctx, orgID), generated.MarkDeliveryDeliveredParams{ID: cmd.ID, OrgID: cmd.OrgID})
		return false
	}
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), adapterOpTimeout)
	defer cancel()
	var interaction map[string]any
	_ = json.Unmarshal(cmd.Interaction, &interaction)

	h, err := ad.Deliver(dctx, adapter.Assignment{
		ReservationID: resID.String(), RouteRequestID: routeID.String(),
		AgentID: apiUUID(cmd.AgentID).String(), Channel: cmd.Channel, Interaction: interaction,
	}, e)
	octx := orgkey.SetOrgID(dctx, orgID)
	if err != nil {
		errMsg := err.Error()
		_, _ = generated.New(e.deps.OrgDB).MarkDeliveryFailed(octx, generated.MarkDeliveryFailedParams{ID: cmd.ID, OrgID: cmd.OrgID, LastError: &errMsg})
		e.deps.Logger.ErrorContext(octx, "delivery deliver failed → tearing down route", "reservation_id", resID, "channel", cmd.Channel, "err", err)
		e.failDelivery(octx, orgID, routeID)
		return false
	}
	handle := string(h)
	q := generated.New(e.deps.OrgDB)
	bound, err := q.SetReservationAdapterHandle(octx, generated.SetReservationAdapterHandleParams{ID: pgUUID(resID), OrgID: cmd.OrgID, AdapterHandle: &handle})
	if err != nil {
		// Leave the command claimed; the lease expires and a later tick retries the
		// bind. Deliver is idempotent by the command id, so a redeliver maps to the
		// same handle.
		e.deps.Logger.ErrorContext(octx, "bind adapter handle failed", "reservation_id", resID, "err", err)
		return false
	}
	if bound == 0 {
		// The reservation went terminal during Deliver (raced abandon/timeout): the
		// handle was never bound, so release the orphan now or it leaks forever.
		e.releaseAssignments(octx, cmd.Channel, []string{handle}, adapter.ReleaseCancelled)
	}
	_, _ = q.MarkDeliveryDelivered(octx, generated.MarkDeliveryDeliveredParams{ID: cmd.ID, OrgID: cmd.OrgID, Handle: &handle})
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
