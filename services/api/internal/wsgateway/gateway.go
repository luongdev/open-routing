package wsgateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
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

// CommandExecutor is the runtime command service (flowrt.Endpoints) — the gateway
// only transports; transitions run in the executor's own tx (review: gateway must
// not own reservation logic).
type CommandExecutor interface {
	ExecuteAgentCommand(ctx context.Context, orgID, agentID, sessionID, clientMsgID, resID uuid.UUID, kind flowrt.AgentCommandKind, reqHash string) (flowrt.AgentCommandResult, error)
}

type Deps struct {
	OrgDB     *db.OrgDB
	Cmd       CommandExecutor
	Presence  presence.Store // optional connection lease (nil ⇒ no presence tracking)
	Logger    *slog.Logger
	GatewayID string
}

type Gateway struct{ d Deps }

func New(d Deps) *Gateway {
	if d.GatewayID == "" {
		d.GatewayID = "gw"
	}
	return &Gateway{d: d}
}

const (
	relayInterval = 150 * time.Millisecond
	writeTimeout  = 5 * time.Second
	sendBuffer    = 64
	// readDeadline bounds an idle read: a client must send at least a heartbeat
	// within this window or the connection is torn down. Without it a half-open
	// socket (peer vanished, no FIN) pins the read goroutine and keeps a live
	// agent_sessions row forever (review MED-3). Clients should heartbeat well
	// inside this window.
	readDeadline = 60 * time.Second
)

// Handler upgrades the connection and runs it. Identity is derived from the
// authenticated request (org from OrgContext, agent from X-Agent-Id) — NOT the
// envelope (review HIGH).
func (g *Gateway) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := orgkey.OrgIDFromContext(r.Context())
		if !ok {
			http.Error(w, "missing org", http.StatusUnauthorized)
			return
		}
		agentID, err := uuid.Parse(r.Header.Get("X-Agent-Id"))
		if err != nil {
			http.Error(w, "missing or invalid X-Agent-Id", http.StatusBadRequest)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return // Accept already wrote the response
		}
		ctx := orgkey.SetOrgID(context.WithoutCancel(r.Context()), orgID)
		g.serve(ctx, conn, orgID, agentID)
	}
}

func (g *Gateway) serve(parent context.Context, conn *websocket.Conn, orgID, agentID uuid.UUID) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	q := generated.New(g.d.OrgDB)
	sessionID := uuid.Must(uuid.NewV7())
	if _, err := q.CreateAgentSession(ctx, generated.CreateAgentSessionParams{
		OrgID: pgUUID(orgID), SessionID: pgUUID(sessionID), AgentID: pgUUID(agentID), GatewayID: g.d.GatewayID,
	}); err != nil {
		_ = conn.Close(websocket.StatusInternalError, "session")
		return
	}
	g.renewPresence(ctx, orgID, agentID, sessionID)
	defer func() {
		// Best-effort terminate so a crashed/closed socket frees the session.
		_, _ = generated.New(g.d.OrgDB).TerminateAgentSession(context.WithoutCancel(ctx), generated.TerminateAgentSessionParams{OrgID: pgUUID(orgID), SessionID: pgUUID(sessionID)})
		g.dropPresence(context.WithoutCancel(ctx), orgID, agentID, sessionID)
		_ = conn.Close(websocket.StatusNormalClosure, "bye")
	}()

	send := make(chan Outbound, sendBuffer)
	// Single writer (coder/websocket forbids concurrent writers).
	go g.writer(ctx, cancel, conn, send)
	g.enqueue(ctx, cancel, send, Outbound{Type: TypeWelcome, SessionID: sessionID.String()})

	relayFrom := make(chan int64, 1) // hello sets the replay floor; default 0 (replay all)
	go g.relay(ctx, cancel, q, orgID, agentID, send, relayFrom)

	g.readLoop(ctx, cancel, conn, q, orgID, agentID, sessionID, send, relayFrom)
}

// writer drains the send channel to the socket; a write error cancels the conn.
func (g *Gateway) writer(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, send <-chan Outbound) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-send:
			b, _ := json.Marshal(msg)
			wctx, wcancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Write(wctx, websocket.MessageText, b)
			wcancel()
			if err != nil {
				cancel()
				return
			}
		}
	}
}

// enqueue applies backpressure: if the send buffer is full the slow consumer is
// closed (it reconnects + replays from server_seq) rather than blocking the relay.
func (g *Gateway) enqueue(ctx context.Context, cancel context.CancelFunc, send chan<- Outbound, msg Outbound) bool {
	select {
	case <-ctx.Done():
		return false
	case send <- msg:
		return true
	default:
		cancel() // overflow → drop the connection (review: close, rely on replay)
		return false
	}
}

// relay reads COMMITTED outbox rows in seq order and pushes them; lastSeq only
// advances after a successful enqueue (review: no skip on backpressure). The
// hello floor is drained inside the loop (not a one-shot select) so a hello that
// lands AFTER the no-hello fallback still raises the floor instead of being
// ignored — which would otherwise replay from 0 (review MED-4). max() guards
// against a late/duplicate hello rewinding past rows already sent.
func (g *Gateway) relay(ctx context.Context, cancel context.CancelFunc, q *generated.Queries, orgID, agentID uuid.UUID, send chan<- Outbound, relayFrom <-chan int64) {
	var lastSeq int64
	started := false // gate the first read on a hello handshake (or the fallback)
	fallback := time.NewTimer(time.Second)
	defer fallback.Stop()
	t := time.NewTicker(relayInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-relayFrom: // hello floor; honored even when late
			if f > lastSeq {
				lastSeq = f
			}
			started = true
		case <-fallback.C: // no hello → replay from lastSeq (0)
			started = true
		case <-t.C:
			if !started {
				continue
			}
			rows, err := q.ReadAgentOutboxSince(ctx, generated.ReadAgentOutboxSinceParams{
				OrgID: pgUUID(orgID), AgentID: pgUUID(agentID), ServerSeq: lastSeq, Limit: 100,
			})
			if err != nil {
				continue // transient; retry next tick
			}
			for _, row := range rows {
				var payload map[string]any
				_ = json.Unmarshal(row.Payload, &payload)
				frame := Outbound{Type: row.Type, Seq: row.ServerSeq, Payload: payload}
				if row.ReservationID.Valid {
					frame.Reservation = uuid.UUID(row.ReservationID.Bytes).String()
				}
				if !g.enqueue(ctx, cancel, send, frame) {
					return // overflow/closed
				}
				lastSeq = row.ServerSeq // advance only after enqueue
			}
		}
	}
}

func (g *Gateway) readLoop(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, q *generated.Queries, orgID, agentID, sessionID uuid.UUID, send chan<- Outbound, relayFrom chan<- int64) {
	helloSent := false
	for {
		rctx, rcancel := context.WithTimeout(ctx, readDeadline)
		typ, data, err := conn.Read(rctx)
		rcancel()
		if err != nil {
			cancel() // read error or idle timeout → tear down (frees the session)
			return
		}
		if typ != websocket.MessageText {
			continue
		}
		var in Inbound
		if json.Unmarshal(data, &in) != nil {
			continue
		}
		switch {
		case in.Type == TypeHello:
			if !helloSent {
				helloSent = true
				select {
				case relayFrom <- in.LastSeq:
				default:
				}
			}
		case in.Type == TypeHeartbeat:
			_, _ = q.TouchAgentSession(ctx, generated.TouchAgentSessionParams{OrgID: pgUUID(orgID), SessionID: pgUUID(sessionID)})
			g.renewPresence(ctx, orgID, agentID, sessionID)
		case isCommand(in.Type):
			g.handleCommand(ctx, cancel, send, orgID, agentID, sessionID, in)
		}
	}
}

func (g *Gateway) handleCommand(ctx context.Context, cancel context.CancelFunc, send chan<- Outbound, orgID, agentID, sessionID uuid.UUID, in Inbound) {
	ack := Outbound{Type: TypeAck, ReplyTo: in.ID}
	cmdID, err := uuid.Parse(in.ID)
	if err != nil {
		ack.Type, ack.Reason = TypeError, "bad message id"
		g.enqueue(ctx, cancel, send, ack)
		return
	}
	resID, err := uuid.Parse(in.Reservation)
	if err != nil {
		ack.Type, ack.Reason = TypeError, "bad reservation id"
		g.enqueue(ctx, cancel, send, ack)
		return
	}
	// request_hash binds the dedupe id to the payload so a reused id with a
	// different command/reservation is rejected.
	sum := sha256.Sum256([]byte(in.Type + "|" + in.Reservation))
	res, err := g.d.Cmd.ExecuteAgentCommand(ctx, orgID, agentID, sessionID, cmdID, resID, flowrt.AgentCommandKind(in.Type), hex.EncodeToString(sum[:]))
	if err != nil {
		ack.Type, ack.Reason = TypeError, "command failed"
		g.enqueue(ctx, cancel, send, ack)
		return
	}
	ack.Status, ack.Reservation = res.Status, res.ReservationID
	g.enqueue(ctx, cancel, send, ack)
}

// renewPresence/dropPresence are best-effort: a presence-store blip must never
// tear a live socket (the readDeadline already bounds a half-open conn; the DB
// agent_sessions row remains the audit trail). Drop is CAS-by-sessionID so a late
// teardown can't evict a fresh reconnect.
func (g *Gateway) renewPresence(ctx context.Context, orgID, agentID, sessionID uuid.UUID) {
	if g.d.Presence == nil {
		return
	}
	if err := g.d.Presence.Renew(ctx, orgID, agentID, sessionID.String()); err != nil && g.d.Logger != nil {
		g.d.Logger.WarnContext(ctx, "presence renew failed", "err", err, "agent_id", agentID)
	}
}

func (g *Gateway) dropPresence(ctx context.Context, orgID, agentID, sessionID uuid.UUID) {
	if g.d.Presence == nil {
		return
	}
	if err := g.d.Presence.Drop(ctx, orgID, agentID, sessionID.String()); err != nil && g.d.Logger != nil {
		g.d.Logger.WarnContext(ctx, "presence drop failed", "err", err, "agent_id", agentID)
	}
}

func pgUUID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }
