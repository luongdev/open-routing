package flowrt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
)

// failingVoice is a voice adapter whose Deliver always errors — to exercise the
// drain worker's retry-with-cap (an adapter that's "down").
type failingVoice struct{ calls int }

func (f *failingVoice) Channel() string { return "voice" }
func (f *failingVoice) Deliver(context.Context, adapter.Assignment, adapter.EventSink) (adapter.Handle, error) {
	f.calls++
	return "", errors.New("adapter down")
}
func (f *failingVoice) Release(context.Context, adapter.Handle, adapter.ReleaseCause) error { return nil }

// TestDrainDeliveries_RetryCap: a down adapter is retried (command stays pending,
// route not torn down) up to maxDeliveryAttempts, then the delivery is marked
// failed — bounded, not endless, and not abandoning the call on a blip.
func TestDrainDeliveries_RetryCap(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	fail := &failingVoice{}
	oe := New(Deps{
		OrgDB: lf.e.deps.OrgDB, Cache: lf.e.deps.Cache, Logger: lf.e.deps.Logger,
		Presence: lf.mem, Capacity: NewCapacityService(), MatcherEnabled: true,
		Adapters: map[string]adapter.ChannelAdapter{"voice": fail}, DeliveryOutbox: true,
	})
	_, resID, _, _ := driveToAcceptedCall(t, lf, oe)

	// Drain with an advancing clock so each tick's claim lease has expired → re-claim.
	base := time.Now()
	for i := 0; i < maxDeliveryAttempts-1; i++ {
		if _, err := oe.DrainDeliveries(lf.ctx, sharedPool, "w1", base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("drain %d: %v", i, err)
		}
		if st := deliveryStatus(t, lf.orgID, resID); st != "pending" {
			t.Fatalf("after attempt %d status=%q, want pending (still retrying)", i+1, st)
		}
	}
	// The capped attempt declares it failed.
	if _, err := oe.DrainDeliveries(lf.ctx, sharedPool, "w1", base.Add(time.Duration(maxDeliveryAttempts)*time.Minute)); err != nil {
		t.Fatalf("final drain: %v", err)
	}
	if st := deliveryStatus(t, lf.orgID, resID); st != "failed" {
		t.Fatalf("after %d attempts status=%q, want failed", maxDeliveryAttempts, st)
	}
	if fail.calls < maxDeliveryAttempts {
		t.Fatalf("Deliver called %d times, want >= %d", fail.calls, maxDeliveryAttempts)
	}
}

// TestDrainDeliveries_BindsHandle: an accepted reservation with a durable
// delivery_command is delivered by the drain worker — the adapter handle is bound
// on the reservation, the command is marked delivered, the producer is idempotent
// (UNIQUE per reservation), and a second drain finds nothing.
func TestDrainDeliveries_BindsHandle(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	me := newMatcherEndpoints(lf) // no adapter → accept leaves adapter_handle null
	routeID, resID, agentID, _ := driveToAcceptedCall(t, lf, me)

	// A delivery-capable Endpoints with the mock voice adapter (the drain worker's
	// role on cmd/runtime).
	de := New(Deps{
		OrgDB: lf.e.deps.OrgDB, Logger: lf.e.deps.Logger, Capacity: NewCapacityService(),
		Adapters: map[string]adapter.ChannelAdapter{"voice": adapter.NewMockVoice(nil)},
	})

	if err := de.enqueueDelivery(lf.ctx, lf.q, lf.orgID, resID, routeID, agentID, "voice", nil); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// Idempotent producer: a retried accept must NOT enqueue a second command.
	if err := de.enqueueDelivery(lf.ctx, lf.q, lf.orgID, resID, routeID, agentID, "voice", nil); err != nil {
		t.Fatalf("enqueue (2nd): %v", err)
	}
	if c := deliveryCount(t, lf.orgID, resID); c != 1 {
		t.Fatalf("delivery_commands for reservation = %d, want 1 (idempotent)", c)
	}

	n, err := de.DrainDeliveries(lf.ctx, sharedPool, "w1", time.Now())
	if err != nil || n != 1 {
		t.Fatalf("drain n=%d err=%v, want 1", n, err)
	}
	if h := reservationHandle(t, lf.orgID, resID); h == "" {
		t.Fatal("adapter handle not bound on the reservation after drain")
	}
	if st := deliveryStatus(t, lf.orgID, resID); st != "delivered" {
		t.Fatalf("delivery_command status = %q, want delivered", st)
	}

	// Nothing left pending → a second drain is a no-op (no redelivery).
	if n2, _ := de.DrainDeliveries(lf.ctx, sharedPool, "w1", time.Now()); n2 != 0 {
		t.Fatalf("second drain delivered %d, want 0", n2)
	}
}

// TestAcceptEnqueuesDelivery_WhenOutboxOn: with DeliveryOutbox on, accept commits
// a delivery_command in the accept tx and does NOT deliver in-process (handle stays
// null until the drain worker runs). Proves the gated wiring end to end.
func TestAcceptEnqueuesDelivery_WhenOutboxOn(t *testing.T) {
	lf := newLiveFixture(t)
	if lf == nil {
		return
	}
	oe := New(Deps{
		OrgDB: lf.e.deps.OrgDB, Cache: lf.e.deps.Cache, Logger: lf.e.deps.Logger,
		Presence: lf.mem, Capacity: NewCapacityService(), MatcherEnabled: true,
		Adapters: map[string]adapter.ChannelAdapter{"voice": adapter.NewMockVoice(nil)},
		DeliveryOutbox: true,
	})
	_, resID, agentID, _ := driveToAcceptedCall(t, lf, oe)

	// Accept enqueued a durable command but did NOT deliver in-process yet.
	if c := deliveryCount(t, lf.orgID, resID); c != 1 {
		t.Fatalf("delivery_commands after accept = %d, want 1 (enqueued, not in-process)", c)
	}
	if h := reservationHandle(t, lf.orgID, resID); h != "" {
		t.Fatalf("handle bound in-process %q — outbox path must defer to the drain worker", h)
	}
	_ = agentID

	// The drain worker delivers it.
	if n, err := oe.DrainDeliveries(lf.ctx, sharedPool, "w1", time.Now()); err != nil || n != 1 {
		t.Fatalf("drain n=%d err=%v, want 1", n, err)
	}
	if h := reservationHandle(t, lf.orgID, resID); h == "" {
		t.Fatal("handle not bound after drain")
	}
}

func deliveryCount(t *testing.T, org, resID uuid.UUID) int {
	t.Helper()
	var n int
	_ = sharedPool.QueryRow(context.Background(),
		"SELECT count(*) FROM delivery_commands WHERE org_id=$1 AND reservation_id=$2", org, resID).Scan(&n)
	return n
}

func deliveryStatus(t *testing.T, org, resID uuid.UUID) string {
	t.Helper()
	var s string
	_ = sharedPool.QueryRow(context.Background(),
		"SELECT status FROM delivery_commands WHERE org_id=$1 AND reservation_id=$2", org, resID).Scan(&s)
	return s
}

func reservationHandle(t *testing.T, org, resID uuid.UUID) string {
	t.Helper()
	var h *string
	_ = sharedPool.QueryRow(context.Background(),
		"SELECT adapter_handle FROM reservations WHERE org_id=$1 AND id=$2", org, resID).Scan(&h)
	if h == nil {
		return ""
	}
	return *h
}
