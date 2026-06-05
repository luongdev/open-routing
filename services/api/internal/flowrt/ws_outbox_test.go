package flowrt

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// seedAgent inserts the agents row the outbox lock is keyed off (LockAgentOutboxSeq
// derives its advisory key from the agent row — review HIGH-2).
func seedAgent(ctx context.Context, t *testing.T, org, agent uuid.UUID) {
	t.Helper()
	if _, err := generated.New(sharedPool).InsertAgent(ctx, generated.InsertAgentParams{
		ID: pgUUID(agent), OrgID: pgUUID(org), Code: "a-" + agent.String()[:8], Name: "t", Email: "t@t", Enabled: true,
	}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
}

// appendOutbox runs the lock+append in one tx (mirrors how the producer must).
func appendOutbox(ctx context.Context, org, agent uuid.UUID, eventKey, typ string) (generated.AgentOutbox, error) {
	var zero generated.AgentOutbox
	tx, err := sharedPool.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := generated.New(tx)
	locked, err := q.LockAgentOutboxSeq(ctx, generated.LockAgentOutboxSeqParams{OrgID: pgUUID(org), ID: pgUUID(agent)})
	if err != nil {
		return zero, err
	}
	if locked != 1 {
		return zero, fmt.Errorf("agent not present: outbox lock acquired 0 rows") // review HIGH: absent agent → no lock
	}
	row, err := q.AppendAgentOutbox(ctx, generated.AppendAgentOutboxParams{
		OrgID: pgUUID(org), AgentID: pgUUID(agent), EventKey: eventKey, Type: typ, Payload: []byte(`{}`),
	})
	if err != nil {
		return zero, err
	}
	return row, tx.Commit(ctx)
}

func TestWSOutbox_PerAgentSeqRaceSafe(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	ctx := context.Background()
	org, agent := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	seedAgent(ctx, t, org, agent)

	// 20 concurrent producers — the per-agent advisory lock must serialize seq
	// allocation so we get 1..20 with no duplicate (a PK violation) and no gap.
	const n = 20
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = appendOutbox(ctx, org, agent, "evt-"+uuid.Must(uuid.NewV7()).String(), "offer")
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("append %d: %v", i, e)
		}
	}
	rows, err := generated.New(sharedPool).ReadAgentOutboxSince(ctx, generated.ReadAgentOutboxSinceParams{
		OrgID: pgUUID(org), AgentID: pgUUID(agent), ServerSeq: 0, Limit: 100,
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(rows) != n {
		t.Fatalf("got %d rows, want %d", len(rows), n)
	}
	for i, r := range rows {
		if r.ServerSeq != int64(i+1) {
			t.Fatalf("row %d has seq %d, want %d (gap/dup)", i, r.ServerSeq, i+1)
		}
	}
}

func TestWSOutbox_EventKeyIdempotent(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	ctx := context.Background()
	org, agent := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	seedAgent(ctx, t, org, agent)
	if _, err := appendOutbox(ctx, org, agent, "dup-key", "offer"); err != nil {
		t.Fatalf("first append: %v", err)
	}
	// Same event_key again → ON CONFLICT DO NOTHING → no row (ErrNoRows), not a dup.
	_, err := appendOutbox(ctx, org, agent, "dup-key", "offer")
	if err != pgx.ErrNoRows {
		t.Fatalf("re-append same key: err=%v, want ErrNoRows", err)
	}
	rows, _ := generated.New(sharedPool).ReadAgentOutboxSince(ctx, generated.ReadAgentOutboxSinceParams{OrgID: pgUUID(org), AgentID: pgUUID(agent), ServerSeq: 0, Limit: 10})
	if len(rows) != 1 {
		t.Fatalf("duplicate event_key created %d rows, want 1", len(rows))
	}
}

// TestWSOutbox_LockAbsentAgentReturnsZero pins review HIGH: when no agents row
// exists the lock acquires nothing and reports 0 rows, so the W4 producer can
// detect agent_not_found instead of appending under a silent no-op lock.
func TestWSOutbox_LockAbsentAgentReturnsZero(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	ctx := context.Background()
	org, agent := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()) // NOT seeded
	tx, err := sharedPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	locked, err := generated.New(tx).LockAgentOutboxSeq(ctx, generated.LockAgentOutboxSeqParams{OrgID: pgUUID(org), ID: pgUUID(agent)})
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	if locked != 0 {
		t.Fatalf("absent-agent lock acquired %d rows, want 0", locked)
	}
}

func TestWSCommandDedupe(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	ctx := context.Background()
	q := generated.New(sharedPool)
	org, agent, cmd := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	row, err := q.BeginCommand(ctx, generated.BeginCommandParams{
		OrgID: pgUUID(org), AgentID: pgUUID(agent), ClientMsgID: pgUUID(cmd), CommandType: "reservation.accept", RequestHash: "h1",
	})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if row.Status != "pending" {
		t.Fatalf("status = %q", row.Status)
	}
	// Re-claim the same client_msg_id → ON CONFLICT DO NOTHING → ErrNoRows (a retry).
	if _, err := q.BeginCommand(ctx, generated.BeginCommandParams{
		OrgID: pgUUID(org), AgentID: pgUUID(agent), ClientMsgID: pgUUID(cmd), CommandType: "reservation.accept", RequestHash: "h1",
	}); err != pgx.ErrNoRows {
		t.Fatalf("duplicate begin: err=%v, want ErrNoRows", err)
	}
	res := []byte(`{"status":"accepted"}`)
	if n, _ := q.FinishCommand(ctx, generated.FinishCommandParams{OrgID: pgUUID(org), AgentID: pgUUID(agent), ClientMsgID: pgUUID(cmd), Result: res}); n != 1 {
		t.Fatalf("finish affected %d rows", n)
	}
}
