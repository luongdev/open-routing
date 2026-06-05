package wsgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/flowrt"
	"github.com/luongdev/open-routing/services/api/internal/presence"
)

// fakeCmd records the last command and returns a canned result (the real
// dedupe/transition is covered by flowrt's command_test).
type fakeCmd struct {
	last      string
	lastLease string
}

func (f *fakeCmd) ExecuteAgentCommand(_ context.Context, _, _, _, _, resID uuid.UUID, leaseToken string, kind flowrt.AgentCommandKind, _ string) (flowrt.AgentCommandResult, error) {
	f.last = string(kind)
	f.lastLease = leaseToken
	return flowrt.AgentCommandResult{Status: "accepted", ReservationID: resID.String()}, nil
}

func dialGateway(t *testing.T, orgID, agentID uuid.UUID, cmd CommandExecutor) (*websocket.Conn, func()) {
	t.Helper()
	g := New(Deps{OrgDB: db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic), Cmd: cmd})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Handler()(w, r.WithContext(orgkey.SetOrgID(r.Context(), orgID)))
	})
	srv := httptest.NewServer(h)
	conn, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/agent/ws",
		&websocket.DialOptions{HTTPHeader: http.Header{"X-Agent-Id": {agentID.String()}}})
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}
	return conn, func() { _ = conn.Close(websocket.StatusNormalClosure, ""); srv.Close() }
}

func readFrame(t *testing.T, conn *websocket.Conn) Outbound {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var out Outbound
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func writeFrame(t *testing.T, conn *websocket.Conn, in Inbound) {
	t.Helper()
	b, _ := json.Marshal(in)
	if err := conn.Write(context.Background(), websocket.MessageText, b); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func pg(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

func TestGateway_WelcomeRelayAndCommand(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	org, agent := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	fc := &fakeCmd{}
	conn, done := dialGateway(t, org, agent, fc)
	defer done()

	// 1. Welcome with a session id.
	if w := readFrame(t, conn); w.Type != TypeWelcome || w.SessionID == "" {
		t.Fatalf("welcome = %+v", w)
	}
	// hello to start the relay from seq 0.
	writeFrame(t, conn, Inbound{Type: TypeHello, LastSeq: 0})

	// 2. Enqueue an outbox offer → the relay must push it.
	ctx := context.Background()
	if _, err := generated.New(sharedPool).InsertAgent(ctx, generated.InsertAgentParams{
		ID: pg(agent), OrgID: pg(org), Code: "a-" + agent.String()[:8], Name: "t", Email: "t@t", Enabled: true,
	}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	tx, _ := sharedPool.Begin(ctx)
	q := generated.New(tx)
	if locked, err := q.LockAgentOutboxSeq(ctx, generated.LockAgentOutboxSeqParams{OrgID: pg(org), ID: pg(agent)}); err != nil || locked != 1 {
		t.Fatalf("lock outbox seq: locked=%d err=%v", locked, err)
	}
	resID := uuid.Must(uuid.NewV7())
	if _, err := q.AppendAgentOutbox(ctx, generated.AppendAgentOutboxParams{
		OrgID: pg(org), AgentID: pg(agent), EventKey: "offer-1", Type: "offer", ReservationID: pg(resID), Payload: []byte(`{"x":1}`),
	}); err != nil {
		t.Fatalf("append outbox: %v", err)
	}
	_ = tx.Commit(ctx)

	if f := readFrame(t, conn); f.Type != "offer" || f.Seq != 1 || f.Reservation != resID.String() {
		t.Fatalf("relayed frame = %+v", f)
	}

	// 3. A command → ack carrying the executor's result.
	cmdID := uuid.Must(uuid.NewV7())
	writeFrame(t, conn, Inbound{ID: cmdID.String(), Type: TypeAccept, Reservation: resID.String()})
	ack := readFrame(t, conn)
	if ack.Type != TypeAck || ack.ReplyTo != cmdID.String() || ack.Status != "accepted" {
		t.Fatalf("ack = %+v", ack)
	}
	if fc.last != string(flowrt.CmdAccept) {
		t.Fatalf("executor saw %q, want reservation.accept", fc.last)
	}
}

// TestGateway_OfferLeaseRoundTrips proves the full transport loop the reference
// client relies on: a durable 'reservation.offer' frame carries the lease_token in
// its payload, the agent echoes it on accept, and the gateway forwards exactly
// that token to the command service (the D5 fence input).
func TestGateway_OfferLeaseRoundTrips(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	org, agent := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	fc := &fakeCmd{}
	conn, done := dialGateway(t, org, agent, fc)
	defer done()
	if w := readFrame(t, conn); w.Type != TypeWelcome {
		t.Fatalf("welcome = %+v", w)
	}
	writeFrame(t, conn, Inbound{Type: TypeHello, LastSeq: 0})

	ctx := context.Background()
	if _, err := generated.New(sharedPool).InsertAgent(ctx, generated.InsertAgentParams{
		ID: pg(agent), OrgID: pg(org), Code: "a-" + agent.String()[:8], Name: "t", Email: "t@t", Enabled: true,
	}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	// A durable offer frame carrying the lease_token (as the offer paths produce it).
	resID := uuid.Must(uuid.NewV7())
	lease := uuid.Must(uuid.NewV7()).String()
	tx, _ := sharedPool.Begin(ctx)
	q := generated.New(tx)
	if _, err := q.LockAgentOutboxSeq(ctx, generated.LockAgentOutboxSeqParams{OrgID: pg(org), ID: pg(agent)}); err != nil {
		t.Fatalf("lock seq: %v", err)
	}
	if _, err := q.AppendAgentOutbox(ctx, generated.AppendAgentOutboxParams{
		OrgID: pg(org), AgentID: pg(agent), EventKey: "offer:" + resID.String(), Type: "reservation.offer",
		ReservationID: pg(resID), Payload: []byte(`{"reservation_id":"` + resID.String() + `","lease_token":"` + lease + `"}`),
	}); err != nil {
		t.Fatalf("append offer: %v", err)
	}
	_ = tx.Commit(ctx)

	f := readFrame(t, conn)
	if f.Type != "reservation.offer" || f.Payload["lease_token"] != lease {
		t.Fatalf("offer frame = %+v, want lease_token %s", f, lease)
	}
	// The agent echoes the lease from the payload on accept.
	cmdID := uuid.Must(uuid.NewV7())
	writeFrame(t, conn, Inbound{ID: cmdID.String(), Type: TypeAccept, Reservation: resID.String(), LeaseToken: f.Payload["lease_token"].(string)})
	if ack := readFrame(t, conn); ack.Type != TypeAck || ack.Status != "accepted" {
		t.Fatalf("ack = %+v", ack)
	}
	if fc.lastLease != lease {
		t.Fatalf("command saw lease %q, want the offered %q (gateway must forward the echoed token)", fc.lastLease, lease)
	}
}

// TestGateway_ReconnectReplaysOffer: an agent that drops mid-offer and reconnects
// re-receives the still-pending offer from the durable outbox (hello replays from
// seq 0) — the blip-reconnect-keeps-offer guarantee. A hello past the seq skips it.
func TestGateway_ReconnectReplaysOffer(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	org, agent := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	g := New(Deps{OrgDB: db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic), Cmd: &fakeCmd{}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Handler()(w, r.WithContext(orgkey.SetOrgID(r.Context(), org)))
	}))
	defer srv.Close()
	dial := func() *websocket.Conn {
		c, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/agent/ws",
			&websocket.DialOptions{HTTPHeader: http.Header{"X-Agent-Id": {agent.String()}}})
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return c
	}

	ctx := context.Background()
	if _, err := generated.New(sharedPool).InsertAgent(ctx, generated.InsertAgentParams{
		ID: pg(agent), OrgID: pg(org), Code: "a-" + agent.String()[:8], Name: "t", Email: "t@t", Enabled: true,
	}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	resID := uuid.Must(uuid.NewV7())
	tx, _ := sharedPool.Begin(ctx)
	q := generated.New(tx)
	_, _ = q.LockAgentOutboxSeq(ctx, generated.LockAgentOutboxSeqParams{OrgID: pg(org), ID: pg(agent)})
	if _, err := q.AppendAgentOutbox(ctx, generated.AppendAgentOutboxParams{
		OrgID: pg(org), AgentID: pg(agent), EventKey: "offer:" + resID.String(), Type: "reservation.offer", ReservationID: pg(resID), Payload: []byte(`{}`),
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	_ = tx.Commit(ctx)

	// First connection receives the offer, then "drops".
	c1 := dial()
	_ = readFrame(t, c1) // welcome
	writeFrame(t, c1, Inbound{Type: TypeHello, LastSeq: 0})
	if f := readFrame(t, c1); f.Type != "reservation.offer" || f.Reservation != resID.String() {
		t.Fatalf("first delivery = %+v", f)
	}
	_ = c1.Close(websocket.StatusAbnormalClosure, "blip")

	// Reconnect, replay from 0 → the still-pending offer is re-delivered.
	c2 := dial()
	defer func() { _ = c2.Close(websocket.StatusNormalClosure, "") }()
	_ = readFrame(t, c2) // welcome
	writeFrame(t, c2, Inbound{Type: TypeHello, LastSeq: 0})
	if f := readFrame(t, c2); f.Type != "reservation.offer" || f.Reservation != resID.String() {
		t.Fatalf("reconnect replay = %+v, want the pending offer re-delivered", f)
	}
}

func TestGateway_DrivesPresence(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	org, agent := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	mem := presence.NewMemStore()
	g := New(Deps{OrgDB: db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic), Cmd: &fakeCmd{}, Presence: mem})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Handler()(w, r.WithContext(orgkey.SetOrgID(r.Context(), org)))
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	conn, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/agent/ws",
		&websocket.DialOptions{HTTPHeader: http.Header{"X-Agent-Id": {agent.String()}}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = readFrame(t, conn) // welcome

	// Connect → lease present.
	if c, _ := mem.Connected(context.Background(), org, agent); !c {
		t.Fatal("connect must set the presence lease")
	}
	// Close → CAS drop releases it.
	_ = conn.Close(websocket.StatusNormalClosure, "")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c, _ := mem.Connected(context.Background(), org, agent); !c {
			return // dropped
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("close must drop the presence lease")
}

func TestGateway_RejectsMissingAgent(t *testing.T) {
	if sharedPool == nil {
		t.Skip("no testcontainer pool")
	}
	g := New(Deps{OrgDB: db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic), Cmd: &fakeCmd{}})
	org := uuid.Must(uuid.NewV7())
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Handler()(w, r.WithContext(orgkey.SetOrgID(r.Context(), org)))
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	// No X-Agent-Id → 400, the upgrade is refused.
	_, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/agent/ws", nil)
	if err == nil {
		t.Fatal("dial without X-Agent-Id should fail")
	}
}

func TestGateway_ConnSlotCapPerOrg(t *testing.T) {
	g := New(Deps{MaxConnsPerOrg: 2})
	orgA := uuid.Must(uuid.NewV7())
	orgB := uuid.Must(uuid.NewV7())

	if !g.acquireSlot(orgA) || !g.acquireSlot(orgA) {
		t.Fatal("first two slots for orgA must be granted")
	}
	if g.acquireSlot(orgA) {
		t.Fatal("third slot for orgA must be rejected (cap=2)")
	}
	// A different org has its own budget.
	if !g.acquireSlot(orgB) {
		t.Fatal("orgB must not be affected by orgA's cap")
	}
	// Releasing frees a slot; the map entry is dropped at zero.
	g.releaseSlot(orgA)
	if !g.acquireSlot(orgA) {
		t.Fatal("a released slot must be reusable")
	}
	g.releaseSlot(orgA)
	g.releaseSlot(orgA)
	g.releaseSlot(orgA) // extra release must not underflow
	g.mu.Lock()
	_, present := g.conns[orgA]
	g.mu.Unlock()
	if present {
		t.Fatal("orgA entry must be deleted once it returns to zero")
	}
}

func TestGateway_ConnCapUnlimitedWhenZero(t *testing.T) {
	g := New(Deps{MaxConnsPerOrg: 0})
	org := uuid.Must(uuid.NewV7())
	for i := 0; i < 1000; i++ {
		if !g.acquireSlot(org) {
			t.Fatalf("cap=0 means unlimited; rejected at %d", i)
		}
	}
}
