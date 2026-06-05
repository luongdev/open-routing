package flowrt

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// adapter_sink.go wires the channel-adapter contract to the engine: on accept the
// engine hands the assignment to the matching adapter (Deliver) and stores its
// handle; the engine implements EventSink so adapter-originated terminals (caller
// hangup, agent disconnect) drive the reservation/route teardown. v0.3 ships a mock
// voice adapter; real LiveKit/SIP media lands behind the same contract in v0.4.

func (e *Endpoints) adapterFor(channel string) (adapter.ChannelAdapter, bool) {
	a, ok := e.deps.Adapters[channel]
	return a, ok
}

// deliverAssignment hands an accepted reservation to its channel adapter and binds
// the returned handle. Called POST-COMMIT (the accept tx is already durable): the
// mock adapter emits delivered→connecting→established synchronously through the
// sink, and each of those opens its own tx — running them outside the accept tx
// avoids reentrancy. No adapter for the channel ⇒ no-op (the WS/HTTP test-double
// path still works).
func (e *Endpoints) deliverAssignment(ctx context.Context, orgID, resID, routeID, agentID uuid.UUID, channel string) {
	ad, ok := e.adapterFor(channel)
	if !ok {
		return
	}
	h, err := ad.Deliver(ctx, adapter.Assignment{
		ReservationID: resID.String(), RouteRequestID: routeID.String(), AgentID: agentID.String(), Channel: channel,
	}, e)
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "adapter deliver failed", "reservation_id", resID, "channel", channel, "err", err)
		return
	}
	octx := orgkey.SetOrgID(ctx, orgID)
	handle := string(h)
	if _, err := generated.New(e.deps.OrgDB).SetReservationAdapterHandle(octx, generated.SetReservationAdapterHandleParams{
		ID: pgUUID(resID), OrgID: pgUUID(orgID), AdapterHandle: &handle,
	}); err != nil {
		e.deps.Logger.ErrorContext(ctx, "bind adapter handle failed", "reservation_id", resID, "err", err)
	}
}

// releaseAssignments tells the adapter to end each delivery the engine is tearing
// down (complete/cancel). Idempotent on the adapter side, so it's safe when the
// teardown was itself adapter-driven (the handle is already terminal). Post-commit.
func (e *Endpoints) releaseAssignments(ctx context.Context, channel string, handles []string, cause adapter.ReleaseCause) {
	ad, ok := e.adapterFor(channel)
	if !ok {
		return
	}
	for _, h := range handles {
		if h == "" {
			continue
		}
		if err := ad.Release(ctx, adapter.Handle(h), cause); err != nil {
			e.deps.Logger.ErrorContext(ctx, "adapter release failed", "handle", h, "err", err)
		}
	}
}

// OnAssignmentEvent maps a channel-adapter lifecycle event onto the reservation.
// Non-terminal events (delivered/connecting/established) are informational.
// Engine-initiated terminals (completed/cancelled) already transitioned the
// reservation, so they are no-ops. An ADAPTER-ORIGINATED terminal
// (caller_abandoned / disconnected / failed / rejected) tears down the route — the
// outside world ended the interaction. Idempotent by construction: teardownRouteTx
// is a no-op on an already-terminal route (the contract lets adapters retry).
func (e *Endpoints) OnAssignmentEvent(ctx context.Context, ev adapter.AssignmentEvent) error {
	if !ev.Type.Terminal() || ev.Type == adapter.EventCompleted || ev.Type == adapter.EventCancelled {
		return nil
	}
	resID, err := uuid.Parse(ev.ReservationID)
	if err != nil {
		return nil // malformed id — nothing to drive
	}
	// Resolve org+route from the reservation id alone (events carry no org).
	bctx := db.WithBypass(ctx, "adapter_event")
	resv, err := generated.New(e.deps.OrgDB).GetReservationByID(bctx, pgUUID(resID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // unknown reservation
	}
	if err != nil {
		return err // transient — the adapter retries
	}
	orgID := apiUUID(resv.OrgID)
	routeID := apiUUID(resv.RouteRequestID)
	octx := orgkey.SetOrgID(ctx, orgID)
	tx, err := e.deps.OrgDB.BeginTx(octx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(octx) }()
	qtx := generated.New(tx)
	if _, _, tErr := e.teardownRouteTx(octx, qtx, orgID, routeID, "route."+string(ev.Type)); tErr != nil {
		if errors.Is(tErr, pgx.ErrNoRows) {
			return nil // route already terminal — idempotent
		}
		return tErr
	}
	return tx.Commit(octx)
}
