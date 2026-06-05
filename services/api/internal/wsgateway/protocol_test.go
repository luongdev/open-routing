package wsgateway

import (
	"encoding/json"
	"testing"
)

// The agent WS protocol is a CONTRACT: the reference client (cmd/refagent) and
// any external agent client depend on these exact JSON field names + frame
// shapes. These golden tests fail loudly if a struct tag or frame type changes,
// so a wire-breaking edit can't land silently. (Behavioral fixtures — reconnect
// replay, dedupe, timeout, stale-offer — are covered by gateway_test.go and the
// flowrt matcher/command tests; this file locks the schema those rely on.)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestProtocol_InboundFramesGolden(t *testing.T) {
	cases := []struct {
		name string
		in   Inbound
		want string
	}{
		{"hello", Inbound{Type: TypeHello, LastSeq: 42}, `{"id":"","type":"hello","last_server_seq":42}`},
		{"heartbeat", Inbound{Type: TypeHeartbeat}, `{"id":"","type":"heartbeat"}`},
		{"accept", Inbound{ID: "m1", Type: TypeAccept, Reservation: "r1", LeaseToken: "lt1"},
			`{"id":"m1","type":"reservation.accept","reservation_id":"r1","lease_token":"lt1"}`},
		{"reject", Inbound{ID: "m2", Type: TypeReject, Reservation: "r1", LeaseToken: "lt1"},
			`{"id":"m2","type":"reservation.reject","reservation_id":"r1","lease_token":"lt1"}`},
		{"complete", Inbound{ID: "m3", Type: TypeComplete, Reservation: "r1", LeaseToken: "lt1"},
			`{"id":"m3","type":"reservation.complete","reservation_id":"r1","lease_token":"lt1"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mustJSON(t, c.in); got != c.want {
				t.Fatalf("wire schema drift:\n got %s\nwant %s", got, c.want)
			}
		})
	}
}

func TestProtocol_OutboundFramesGolden(t *testing.T) {
	cases := []struct {
		name string
		out  Outbound
		want string
	}{
		{"welcome", Outbound{Type: TypeWelcome, SessionID: "s1"}, `{"type":"welcome","session_id":"s1"}`},
		{"ack", Outbound{Type: TypeAck, ReplyTo: "m1", Status: "accepted", Reservation: "r1"},
			`{"type":"ack","reply_to":"m1","status":"accepted","reservation_id":"r1"}`},
		{"error", Outbound{Type: TypeError, ReplyTo: "m1", Reason: "lease_mismatch"},
			`{"type":"error","reply_to":"m1","reason":"lease_mismatch"}`},
		// Offer frames are relayed verbatim from agent_outbox: type + server_seq +
		// reservation_id + a payload carrying the lease_token the agent must echo.
		{"offer", Outbound{Type: "reservation.offer", Seq: 7, Reservation: "r1", Payload: map[string]any{"lease_token": "lt1"}},
			`{"type":"reservation.offer","server_seq":7,"reservation_id":"r1","payload":{"lease_token":"lt1"}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mustJSON(t, c.out); got != c.want {
				t.Fatalf("wire schema drift:\n got %s\nwant %s", got, c.want)
			}
		})
	}
}

// A spoofed identity in an inbound frame must be unrepresentable: Inbound carries
// NO org/agent/session fields (identity is bound to the authenticated connection,
// review HIGH). Unknown fields are ignored, never promoted to identity.
func TestProtocol_InboundIgnoresUnknownAndIdentityFields(t *testing.T) {
	raw := `{"id":"m1","type":"reservation.accept","reservation_id":"r1","lease_token":"lt1","org_id":"evil","agent_id":"evil","session_id":"evil","extra":123}`
	var in Inbound
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.ID != "m1" || in.Type != TypeAccept || in.Reservation != "r1" || in.LeaseToken != "lt1" {
		t.Fatalf("known fields mis-parsed: %+v", in)
	}
	// Re-marshalling must not leak any org/agent/session field — the struct can't
	// hold one, so the spoof is dropped on the floor.
	if out := mustJSON(t, in); out != `{"id":"m1","type":"reservation.accept","reservation_id":"r1","lease_token":"lt1"}` {
		t.Fatalf("identity field leaked into envelope: %s", out)
	}
}

func TestProtocol_CommandClassification(t *testing.T) {
	for _, c := range []string{TypeAccept, TypeReject, TypeComplete} {
		if !isCommand(c) {
			t.Fatalf("%q must be a command", c)
		}
	}
	for _, c := range []string{TypeHello, TypeHeartbeat, TypeWelcome, TypeAck, TypeError, "reservation.offer", "garbage"} {
		if isCommand(c) {
			t.Fatalf("%q must NOT be a command", c)
		}
	}
}
