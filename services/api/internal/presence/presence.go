// Package presence is the v0.3 W3 agent connection lease. An agent is
// *offerable* only while connected; connection is a TTL'd lease the gateway
// renews on every heartbeat (Redis-primary — a crashed gateway's lease simply
// expires, so liveness is cluster-visible without a DB write per beat). The DB
// (agent_sessions) remains the audit trail; a gateway-start Redis rebuild from
// live sessions is a documented follow-up (design.md D2), not W3.
//
// The lease VALUE is the session id (a per-connection token) so Drop is a
// compare-and-delete: a stale teardown cannot evict a newer reconnect's lease
// (review HIGH). The live candidate source treats a Connected error as an infra
// error that parks the route (retryable) — NOT as "disconnected" — so a Redis
// outage never silently empties the pool (review HIGH).
package presence

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// DefaultTTL is the lease lifetime: a client must heartbeat well inside it. ~3×
// a 30s heartbeat budget so one missed beat does not drop a live agent.
const DefaultTTL = 90 // seconds

// Store is the connection-lease store. All methods are safe for concurrent use.
type Store interface {
	// Renew sets/extends the agent's lease, stamping it with this connection's
	// sessionID (the CAS token for Drop).
	Renew(ctx context.Context, org, agent uuid.UUID, sessionID string) error
	// Connected reports whether the agent currently holds a live lease. An error
	// means the store is unreachable — callers MUST treat that as an infra error,
	// never as "disconnected".
	Connected(ctx context.Context, org, agent uuid.UUID) (bool, error)
	// ConnectedMany is the batch form (one round-trip) for the live candidate
	// source. Returns a map keyed by agent; an absent/false entry = not connected.
	// Same error contract as Connected (infra error, never "disconnected").
	ConnectedMany(ctx context.Context, org uuid.UUID, agents []uuid.UUID) (map[uuid.UUID]bool, error)
	// Drop releases the lease ONLY if it still belongs to sessionID (compare-and-
	// delete) so a late teardown can't evict a fresh reconnect.
	Drop(ctx context.Context, org, agent uuid.UUID, sessionID string) error
}

func key(org, agent uuid.UUID) string {
	return "presence:" + org.String() + ":" + agent.String()
}

// MemStore is an in-process Store for tests. It models the CAS-on-drop semantics
// exactly; TTL expiry is simulated explicitly via Expire (deterministic — no
// sleeps).
type MemStore struct {
	mu      sync.Mutex
	entries map[string]string // key → sessionID
}

func NewMemStore() *MemStore { return &MemStore{entries: map[string]string{}} }

func (m *MemStore) Renew(_ context.Context, org, agent uuid.UUID, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key(org, agent)] = sessionID
	return nil
}

func (m *MemStore) Connected(_ context.Context, org, agent uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.entries[key(org, agent)]
	return ok, nil
}

func (m *MemStore) ConnectedMany(_ context.Context, org uuid.UUID, agents []uuid.UUID) (map[uuid.UUID]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[uuid.UUID]bool, len(agents))
	for _, a := range agents {
		_, ok := m.entries[key(org, a)]
		out[a] = ok
	}
	return out, nil
}

func (m *MemStore) Drop(_ context.Context, org, agent uuid.UUID, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(org, agent)
	if m.entries[k] == sessionID { // CAS: only the owning session may evict
		delete(m.entries, k)
	}
	return nil
}

// Expire simulates a TTL lapse for tests.
func (m *MemStore) Expire(org, agent uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key(org, agent))
}
