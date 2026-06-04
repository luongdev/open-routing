package adapter

import (
	"context"
	"testing"
	"time"
)

// recSink records events and de-dupes by CorrelationID (mirrors how the engine
// will apply each at most once).
type recSink struct {
	events []AssignmentEvent
	seen   map[string]bool
}

func newRecSink() *recSink { return &recSink{seen: map[string]bool{}} }

func (s *recSink) OnAssignmentEvent(_ context.Context, ev AssignmentEvent) error {
	if s.seen[ev.CorrelationID] {
		return nil // idempotent: already applied
	}
	s.seen[ev.CorrelationID] = true
	s.events = append(s.events, ev)
	return nil
}

func (s *recSink) types() []EventType {
	out := make([]EventType, len(s.events))
	for i, e := range s.events {
		out[i] = e.Type
	}
	return out
}

func fixedNow() func() time.Time {
	t := time.Unix(1700000000, 0).UTC()
	return func() time.Time { return t }
}

func eqTypes(a []EventType, b ...EventType) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func deliver(t *testing.T) (*MockVoice, *recSink, Handle) {
	t.Helper()
	m := NewMockVoice(fixedNow())
	sink := newRecSink()
	h, err := m.Deliver(context.Background(), Assignment{ReservationID: "r1", AgentID: "a1", Channel: "voice"}, sink)
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if h != "mock-voice:r1" {
		t.Fatalf("handle = %q, want mock-voice:r1", h)
	}
	return m, sink, h
}

func TestMockVoice_HappyPath(t *testing.T) {
	m, sink, h := deliver(t)
	if !eqTypes(sink.types(), EventAccepted, EventConnecting, EventEstablished) {
		t.Fatalf("after deliver: %v", sink.types())
	}
	if err := m.Complete(context.Background(), h); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if !eqTypes(sink.types(), EventAccepted, EventConnecting, EventEstablished, EventCompleted) {
		t.Fatalf("after complete: %v", sink.types())
	}
}

func TestMockVoice_TerminalVariants(t *testing.T) {
	cases := []struct {
		name string
		do   func(*MockVoice, context.Context, Handle) error
		want EventType
	}{
		{"failed", func(m *MockVoice, c context.Context, h Handle) error { return m.Fail(c, h, "boom") }, EventFailed},
		{"disconnected", func(m *MockVoice, c context.Context, h Handle) error { return m.Disconnect(c, h) }, EventDisconnected},
		{"caller_abandoned", func(m *MockVoice, c context.Context, h Handle) error { return m.Abandon(c, h) }, EventCallerAbandoned},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, sink, h := deliver(t)
			if err := tc.do(m, context.Background(), h); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			last := sink.events[len(sink.events)-1]
			if last.Type != tc.want {
				t.Fatalf("last = %q, want %q", last.Type, tc.want)
			}
			if !last.Type.Terminal() {
				t.Fatalf("%q should be terminal", last.Type)
			}
			// A second terminal call fails: the handle was released.
			if err := tc.do(m, context.Background(), h); err == nil {
				t.Fatalf("second %s on released handle should error", tc.name)
			}
		})
	}
}

func TestMockVoice_FailedCarriesReason(t *testing.T) {
	m, sink, h := deliver(t)
	_ = m.Fail(context.Background(), h, "adapter exploded")
	last := sink.events[len(sink.events)-1]
	if last.Reason != "adapter exploded" {
		t.Fatalf("reason = %q", last.Reason)
	}
}

// A redelivered event (same CorrelationID) is applied at most once.
func TestMockVoice_Idempotent(t *testing.T) {
	_, sink, _ := deliver(t)
	// Replay the established event verbatim.
	est := sink.events[2]
	_ = sink.OnAssignmentEvent(context.Background(), est)
	if got := len(sink.events); got != 3 {
		t.Fatalf("duplicate applied: %d events, want 3", got)
	}
}

// Release is idempotent and the handle stays opaque to callers.
func TestMockVoice_ReleaseIdempotent(t *testing.T) {
	m, _, h := deliver(t)
	if err := m.Release(context.Background(), h); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := m.Release(context.Background(), h); err != nil {
		t.Fatalf("second release should be a no-op: %v", err)
	}
}
