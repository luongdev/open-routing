package flowrt

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// capFixture gives an OrgDB (so capacity queries run through SQLChecker) + a ctx.
type capFixture struct {
	orgDB *db.OrgDB
	ctx   context.Context
	orgID uuid.UUID
	svc   *CapacityService
}

func newCapFixture(t *testing.T) *capFixture {
	t.Helper()
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
		return nil
	}
	orgID := uuid.Must(uuid.NewV7())
	return &capFixture{
		orgDB: db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic),
		ctx:   orgkey.SetOrgID(context.Background(), orgID),
		orgID: orgID,
		svc:   NewCapacityService(),
	}
}

// inTx runs fn against a committed OrgDB tx-bound *Queries.
func (f *capFixture) inTx(t *testing.T, fn func(q *generated.Queries) error) {
	t.Helper()
	tx, err := f.orgDB.BeginTx(f.ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(f.ctx) }()
	if err := fn(generated.New(tx)); err != nil {
		t.Fatalf("tx fn: %v", err)
	}
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestCapacity_AcquireToExhaustion(t *testing.T) {
	f := newCapFixture(t)
	if f == nil {
		return
	}
	agent := uuid.Must(uuid.NewV7())
	f.inTx(t, func(q *generated.Queries) error { return f.svc.ProvisionInTx(f.ctx, q, f.orgID, agent, "chat") })

	seen := map[int32]bool{}
	for i := int32(0); i < chatCapacity; i++ {
		f.inTx(t, func(q *generated.Queries) error {
			slot, ok, err := f.svc.AcquireInTx(f.ctx, q, f.orgID, agent, "chat", uuid.Must(uuid.NewV7()), time.Now().Add(time.Minute))
			if err != nil || !ok {
				t.Fatalf("acquire %d: ok=%v err=%v", i, ok, err)
			}
			if seen[slot] {
				t.Fatalf("slot %d handed out twice", slot)
			}
			seen[slot] = true
			return nil
		})
	}
	// One past capacity → ok=false.
	f.inTx(t, func(q *generated.Queries) error {
		_, ok, err := f.svc.AcquireInTx(f.ctx, q, f.orgID, agent, "chat", uuid.Must(uuid.NewV7()), time.Now().Add(time.Minute))
		if err != nil || ok {
			t.Fatalf("over-capacity acquire: ok=%v err=%v, want ok=false", ok, err)
		}
		return nil
	})
}

func TestCapacity_ConfirmReleaseAndExpiredHold(t *testing.T) {
	f := newCapFixture(t)
	if f == nil {
		return
	}
	agent := uuid.Must(uuid.NewV7())
	f.inTx(t, func(q *generated.Queries) error { return f.svc.ProvisionInTx(f.ctx, q, f.orgID, agent, "voice") })

	// Acquire + confirm + release frees the single voice slot.
	res := uuid.Must(uuid.NewV7())
	f.inTx(t, func(q *generated.Queries) error {
		_, ok, err := f.svc.AcquireInTx(f.ctx, q, f.orgID, agent, "voice", res, time.Now().Add(time.Minute))
		if err != nil || !ok {
			t.Fatalf("acquire: ok=%v err=%v", ok, err)
		}
		return nil
	})
	f.inTx(t, func(q *generated.Queries) error {
		ok, err := f.svc.ConfirmInTx(f.ctx, q, f.orgID, res)
		if err != nil || !ok {
			t.Fatalf("confirm: ok=%v err=%v", ok, err)
		}
		return nil
	})
	f.inTx(t, func(q *generated.Queries) error { return f.svc.ReleaseInTx(f.ctx, q, f.orgID, res) })
	// Released → re-acquireable.
	f.inTx(t, func(q *generated.Queries) error {
		_, ok, err := f.svc.AcquireInTx(f.ctx, q, f.orgID, agent, "voice", uuid.Must(uuid.NewV7()), time.Now().Add(time.Minute))
		if err != nil || !ok {
			t.Fatalf("re-acquire after release: ok=%v err=%v", ok, err)
		}
		return nil
	})

	// A hold that already expired cannot be confirmed (defense-in-depth).
	agent2 := uuid.Must(uuid.NewV7())
	res2 := uuid.Must(uuid.NewV7())
	f.inTx(t, func(q *generated.Queries) error { return f.svc.ProvisionInTx(f.ctx, q, f.orgID, agent2, "voice") })
	f.inTx(t, func(q *generated.Queries) error {
		_, ok, err := f.svc.AcquireInTx(f.ctx, q, f.orgID, agent2, "voice", res2, time.Now().Add(-time.Hour))
		if err != nil || !ok {
			t.Fatalf("acquire expired: ok=%v err=%v", ok, err)
		}
		return nil
	})
	f.inTx(t, func(q *generated.Queries) error {
		ok, err := f.svc.ConfirmInTx(f.ctx, q, f.orgID, res2)
		if err != nil || ok {
			t.Fatalf("confirm expired hold: ok=%v err=%v, want ok=false", ok, err)
		}
		return nil
	})
}

// TestCapacity_OnePerReservation: a second acquire for the same reservation_id
// violates the partial-unique index (guards a double-acquire).
func TestCapacity_OnePerReservation(t *testing.T) {
	f := newCapFixture(t)
	if f == nil {
		return
	}
	agent := uuid.Must(uuid.NewV7())
	res := uuid.Must(uuid.NewV7())
	f.inTx(t, func(q *generated.Queries) error { return f.svc.ProvisionInTx(f.ctx, q, f.orgID, agent, "chat") })
	f.inTx(t, func(q *generated.Queries) error {
		_, ok, err := f.svc.AcquireInTx(f.ctx, q, f.orgID, agent, "chat", res, time.Now().Add(time.Minute))
		if err != nil || !ok {
			t.Fatalf("first acquire: ok=%v err=%v", ok, err)
		}
		return nil
	})
	tx, _ := f.orgDB.BeginTx(f.ctx)
	defer func() { _ = tx.Rollback(f.ctx) }()
	if _, _, err := f.svc.AcquireInTx(f.ctx, generated.New(tx), f.orgID, agent, "chat", res, time.Now().Add(time.Minute)); err == nil {
		t.Fatal("second acquire for same reservation_id should violate unique index")
	}
}

func TestCapacity_ConcurrentAcquireExactlyCapacity(t *testing.T) {
	f := newCapFixture(t)
	if f == nil {
		return
	}
	agent := uuid.Must(uuid.NewV7())
	f.inTx(t, func(q *generated.Queries) error { return f.svc.ProvisionInTx(f.ctx, q, f.orgID, agent, "chat") })

	const producers = 12
	var wg sync.WaitGroup
	oks := make([]bool, producers)
	for i := 0; i < producers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tx, err := f.orgDB.BeginTx(f.ctx)
			if err != nil {
				return
			}
			defer func() { _ = tx.Rollback(f.ctx) }()
			_, ok, err := f.svc.AcquireInTx(f.ctx, generated.New(tx), f.orgID, agent, "chat", uuid.Must(uuid.NewV7()), time.Now().Add(time.Minute))
			if err == nil && ok {
				if cErr := tx.Commit(f.ctx); cErr == nil {
					oks[i] = true
				}
			}
		}(i)
	}
	wg.Wait()
	won := 0
	for _, ok := range oks {
		if ok {
			won++
		}
	}
	if won != int(chatCapacity) {
		t.Fatalf("%d producers won, want exactly %d (SKIP LOCKED capacity gate)", won, chatCapacity)
	}
}

// TestCapacity_SweepAndReconcile: the timer sweeps a PENDING expired hold but
// never a CONFIRMED slot; the reconciler reclaims a confirmed slot whose
// reservation went terminal.
func TestCapacity_SweepAndReconcile(t *testing.T) {
	f := newCapFixture(t)
	if f == nil {
		return
	}
	agent := uuid.Must(uuid.NewV7())
	f.inTx(t, func(q *generated.Queries) error { return f.svc.ProvisionInTx(f.ctx, q, f.orgID, agent, "chat") })

	pending := uuid.Must(uuid.NewV7()) // expired pending → swept
	confirmed := uuid.Must(uuid.NewV7())
	f.inTx(t, func(q *generated.Queries) error {
		if _, ok, err := f.svc.AcquireInTx(f.ctx, q, f.orgID, agent, "chat", pending, time.Now().Add(-time.Hour)); err != nil || !ok {
			t.Fatalf("acquire pending: ok=%v err=%v", ok, err)
		}
		if _, ok, err := f.svc.AcquireInTx(f.ctx, q, f.orgID, agent, "chat", confirmed, time.Now().Add(time.Minute)); err != nil || !ok {
			t.Fatalf("acquire confirmed: ok=%v err=%v", ok, err)
		}
		ok, err := f.svc.ConfirmInTx(f.ctx, q, f.orgID, confirmed)
		if err != nil || !ok {
			t.Fatalf("confirm: ok=%v err=%v", ok, err)
		}
		return nil
	})

	// Sweep runs cross-org via the raw pool (no org filter — like the worker).
	if _, err := generated.New(sharedPool).SweepExpiredCapacityHolds(f.ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	// The pending hold is freed; the confirmed slot still holds.
	if free := f.countFree(t, agent, "chat"); free != chatCapacity-1 {
		t.Fatalf("after sweep free=%d, want %d (pending freed, confirmed held)", free, chatCapacity-1)
	}

	// Reconcile reclaims the confirmed slot once its reservation is terminal.
	// No reservations row exists for `confirmed`, so NOT EXISTS(live) holds → freed.
	if _, err := generated.New(sharedPool).ReconcileOrphanedSlots(f.ctx, pgUUID(f.orgID)); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if free := f.countFree(t, agent, "chat"); free != chatCapacity {
		t.Fatalf("after reconcile free=%d, want %d (orphan confirmed slot reclaimed)", free, chatCapacity)
	}
}

func (f *capFixture) countFree(t *testing.T, agent uuid.UUID, channel string) int32 {
	t.Helper()
	free, err := generated.New(sharedPool).CountFreeCapacitySlots(f.ctx, generated.CountFreeCapacitySlotsParams{
		OrgID: pgUUID(f.orgID), AgentID: pgUUID(agent), Channel: channel, Column4: channelCapacity(channel),
	})
	if err != nil {
		t.Fatalf("count free: %v", err)
	}
	return free
}
