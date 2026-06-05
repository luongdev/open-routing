// Package wsgateway is the v0.3 W2 realtime transport: a WebSocket gateway that
// pushes durable outbox frames to a connected agent and carries the agent's
// idempotent accept/reject/complete commands to the runtime command service.
//
// It is transport-only (review scope): it does NOT decide who to offer (matcher =
// W4) or make an agent offerable (presence lease = W3). Identity is derived from
// the authenticated connection, never the envelope (review HIGH). Delivery is
// outbox-first: the relay reads COMMITTED agent_outbox rows in seq order and
// pushes; nothing is pushed from producer memory.
package wsgateway

// Inbound frame types (client -> server).
const (
	TypeHello     = "hello"     // {last_server_seq}: resume; replay outbox after last_server_seq
	TypeHeartbeat = "heartbeat" // keepalive; refreshes the session
	TypeAccept    = "reservation.accept"
	TypeReject    = "reservation.reject"
	TypeComplete  = "reservation.complete"
)

// Outbound frame types (server -> client).
const (
	TypeWelcome = "welcome" // {session_id}
	TypeAck     = "ack"     // {reply_to, status, reservation_id}
	TypeError   = "error"   // {reply_to, reason}
	// offer / assignment.* frames are relayed verbatim from agent_outbox.type.
)

// Inbound is a client frame. Identity (org/agent/session) is NOT read from here —
// it is bound to the authenticated connection — so a spoofed id is ignored.
type Inbound struct {
	ID          string `json:"id"` // client message id (idempotency key for commands)
	Type        string `json:"type"`
	LastSeq     int64  `json:"last_server_seq,omitempty"`
	Reservation string `json:"reservation_id,omitempty"`
	LeaseToken  string `json:"lease_token,omitempty"` // echoed from the offer frame; fences a stale command (D5)
}

// Outbound is a server frame. Relayed outbox frames set Seq + Payload; acks set
// ReplyTo + Status.
type Outbound struct {
	Type        string         `json:"type"`
	Seq         int64          `json:"server_seq,omitempty"`
	ReplyTo     string         `json:"reply_to,omitempty"`
	Status      string         `json:"status,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	Reservation string         `json:"reservation_id,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	Reason      string         `json:"reason,omitempty"`
}

// isCommand reports whether an inbound type is a reservation command.
func isCommand(t string) bool {
	return t == TypeAccept || t == TypeReject || t == TypeComplete
}
