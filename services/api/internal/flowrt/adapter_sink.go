package flowrt

import (
	"context"
	"errors"
	"time"

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
	return a, ok && a != nil // an explicit nil map value is "no adapter", not a panic
}

// adapterOpTimeout bounds a post-commit adapter call so a hung Deliver/Release
// can't pin the handler goroutine forever (real adapters; the mock is instant).
const adapterOpTimeout = 10 * time.Second

// deliverAssignment hands an accepted reservation to its channel adapter and binds
// the returned handle. Called POST-COMMIT (the accept tx is already durable), on a
// context DETACHED from the request (WithoutCancel) so a client disconnect at the
// commit boundary can't cancel the delivery and orphan the media (review MED). No
// adapter for the channel ⇒ no-op (the WS/HTTP test-double path still works).
//
// NOTE (v0.4): this post-commit handoff is not crash-durable — a process crash
// between the accept commit and Deliver leaves an accepted reservation with no
// delivery. Harmless for the v0.3 in-process mock (Deliver is synchronous, instant,
// and can't fail), but a real async adapter needs a durable delivery-outbox + a
// retry worker (like continuations). Tracked for v0.4 (cross-AI review BLOCK).
func (e *Endpoints) deliverAssignment(ctx context.Context, orgID, resID, routeID, agentID uuid.UUID, channel string) {
	ad, ok := e.adapterFor(channel)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), adapterOpTimeout)
	defer cancel()
	h, err := ad.Deliver(ctx, adapter.Assignment{
		ReservationID: resID.String(), RouteRequestID: routeID.String(), AgentID: agentID.String(), Channel: channel,
	}, e)
	if err != nil {
		// Delivery couldn't be set up → the agent + caller would be stranded on a
		// dead call. Tear the route down (cancel reservation, free slot, agent WrapUp)
		// rather than leave an accepted reservation with no live media (review HIGH).
		e.deps.Logger.ErrorContext(ctx, "adapter deliver failed → tearing down route", "reservation_id", resID, "channel", channel, "err", err)
		// Inline v3 path: the accept already committed, so there's no retry here —
		// best-effort teardown (failDelivery logs its own error). The durable-outbox
		// path checks this return; this caller intentionally does not.
		_ = e.failDelivery(ctx, orgID, routeID)
		return
	}
	octx := orgkey.SetOrgID(ctx, orgID)
	handle := string(h)
	rows, err := generated.New(e.deps.OrgDB).SetReservationAdapterHandle(octx, generated.SetReservationAdapterHandleParams{
		ID: pgUUID(resID), OrgID: pgUUID(orgID), AdapterHandle: &handle,
	})
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "bind adapter handle failed", "reservation_id", resID, "err", err)
		return
	}
	if rows == 0 {
		// The reservation went terminal during Deliver (a raced abandon/timeout): the
		// handle was never bound, so no teardown will Release it → release it now or
		// the adapter delivery leaks forever (review BLOCK).
		e.releaseAssignments(octx, channel, []string{handle}, adapter.ReleaseCancelled)
	}
}

// failDelivery tears a route down when its adapter delivery could not be set up.
// Its own tx; an already-terminal route (raced teardown) is a benign no-op. Returns
// an error so the durable-delivery worker can avoid marking a command failed when
// the teardown itself failed (else the route would be stranded 'accepted' while the
// command is 'failed' — cross-AI review MED split-brain). The v3 inline caller
// ignores the error (its accept already committed; a retry is not available there).
func (e *Endpoints) failDelivery(ctx context.Context, orgID, routeID uuid.UUID) error {
	octx := orgkey.SetOrgID(ctx, orgID)
	tx, err := e.deps.OrgDB.BeginTx(octx)
	if err != nil {
		e.deps.Logger.ErrorContext(octx, "fail-delivery begin tx", "route_id", routeID, "err", err)
		return err
	}
	defer func() { _ = tx.Rollback(octx) }()
	if _, _, err := e.teardownRouteTx(octx, generated.New(tx), orgID, routeID, "route.delivery_failed"); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // route already terminal — nothing to tear down
		}
		e.deps.Logger.ErrorContext(octx, "fail-delivery teardown", "route_id", routeID, "err", err)
		return err
	}
	return tx.Commit(octx)
}

// releaseAssignments tells the adapter to end each delivery the engine is tearing
// down (complete/cancel). Idempotent on the adapter side, so it's safe when the
// teardown was itself adapter-driven (the handle is already terminal). Post-commit
// on a detached context (review MED) so a request cancel can't leak the delivery.
func (e *Endpoints) releaseAssignments(ctx context.Context, channel string, handles []string, cause adapter.ReleaseCause) {
	ad, ok := e.adapterFor(channel)
	if !ok || len(handles) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), adapterOpTimeout)
	defer cancel()
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
//
// v0.3 scope: ALL adapter-originated terminals tear the route down. Finer policy
// (e.g. disconnect/reject → re-offer while the caller is still present) is the
// adapter-driven mid-handling REASSIGN policy deferred to v0.4.
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
	// Authority + staleness fence: act only on a terminal whose handle matches the
	// reservation's CURRENT bound handle AND whose reservation is still 'accepted'.
	// A superseded/forged terminal (wrong or empty handle) or one for a reservation
	// that already resolved (completed/cancelled/timeout) is ignored — a reservation
	// id alone does not authorize tearing a route down (review HIGH).
	if resv.State != "accepted" || resv.AdapterHandle == nil || *resv.AdapterHandle != string(ev.Handle) {
		return nil
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
