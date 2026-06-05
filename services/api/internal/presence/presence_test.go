package presence

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestMemStore_RenewConnectedDropCAS(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()
	org, agent := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	if c, _ := m.Connected(ctx, org, agent); c {
		t.Fatal("not connected before renew")
	}
	_ = m.Renew(ctx, org, agent, "sess-1")
	if c, _ := m.Connected(ctx, org, agent); !c {
		t.Fatal("connected after renew")
	}
	// A stale session must NOT evict a live lease (CAS).
	_ = m.Drop(ctx, org, agent, "stale")
	if c, _ := m.Connected(ctx, org, agent); !c {
		t.Fatal("stale Drop must be a no-op")
	}
	// The owning session evicts.
	_ = m.Drop(ctx, org, agent, "sess-1")
	if c, _ := m.Connected(ctx, org, agent); c {
		t.Fatal("owning Drop must evict")
	}
	// Expire simulates a TTL lapse.
	_ = m.Renew(ctx, org, agent, "sess-2")
	m.Expire(org, agent)
	if c, _ := m.Connected(ctx, org, agent); c {
		t.Fatal("expired lease must read disconnected")
	}
}

func TestRedisStore_TTLAndDropCAS(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ctx := context.Background()
	store := NewRedisStore(rdb, 90*time.Second)
	org, agent := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	if err := store.Renew(ctx, org, agent, "sess-1"); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if c, err := store.Connected(ctx, org, agent); err != nil || !c {
		t.Fatalf("connected = %v (err %v)", c, err)
	}

	// A reconnect takes over the lease with a new token.
	if err := store.Renew(ctx, org, agent, "sess-2"); err != nil {
		t.Fatalf("renew2: %v", err)
	}
	// The OLD session's Drop is a no-op (CAS mismatch).
	if err := store.Drop(ctx, org, agent, "sess-1"); err != nil {
		t.Fatalf("stale drop: %v", err)
	}
	if c, _ := store.Connected(ctx, org, agent); !c {
		t.Fatal("stale Drop evicted a newer lease (CAS broken)")
	}
	// The current session drops it.
	if err := store.Drop(ctx, org, agent, "sess-2"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if c, _ := store.Connected(ctx, org, agent); c {
		t.Fatal("owning Drop must evict")
	}

	// TTL expiry → disconnected.
	if err := store.Renew(ctx, org, agent, "sess-3"); err != nil {
		t.Fatalf("renew3: %v", err)
	}
	s.FastForward(91 * time.Second)
	if c, _ := store.Connected(ctx, org, agent); c {
		t.Fatal("lease must expire after TTL")
	}
}

func TestRedisStore_ConnectedManyBatch(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	ctx := context.Background()
	store := NewRedisStore(rdb, time.Minute)
	org := uuid.Must(uuid.NewV7())
	a, b, c := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	_ = store.Renew(ctx, org, a, "s")
	_ = store.Renew(ctx, org, c, "s")

	got, err := store.ConnectedMany(ctx, org, []uuid.UUID{a, b, c})
	if err != nil {
		t.Fatalf("connected many: %v", err)
	}
	if !got[a] || got[b] || !got[c] {
		t.Fatalf("ConnectedMany = %v, want a=true b=false c=true", got)
	}
	// Error propagates (infra), not "all disconnected".
	s.Close()
	if _, err := store.ConnectedMany(ctx, org, []uuid.UUID{a}); err == nil {
		t.Fatal("ConnectedMany must propagate an unreachable-store error")
	}
}

func TestRedisStore_ConnectedErrorPropagates(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	store := NewRedisStore(rdb, time.Minute)
	s.Close() // server gone → Connected must error, NOT return false
	if _, err := store.Connected(context.Background(), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())); err == nil {
		t.Fatal("Connected must propagate an unreachable-store error (fail-closed as infra error)")
	}
}
