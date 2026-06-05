package flowrt

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
)

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
