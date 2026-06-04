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
)

// fakeCmd records the last command and returns a canned result (the real
// dedupe/transition is covered by flowrt's command_test).
type fakeCmd struct{ last string }

func (f *fakeCmd) ExecuteAgentCommand(_ context.Context, _, _, _, _, resID uuid.UUID, kind flowrt.AgentCommandKind, _ string) (flowrt.AgentCommandResult, error) {
	f.last = string(kind)
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
