package adapter

import (
	"context"
	"testing"
	"time"
)

// recSink records events and de-dupes by CorrelationID (mirrors how the engine
// applies each at most once).
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
	if !eqTypes(sink.types(), EventDelivered, EventConnecting, EventEstablished) {
		t.Fatalf("after deliver: %v", sink.types())
	}
	if err := m.Release(context.Background(), h, ReleaseCompleted); err != nil {
		t.Fatalf("release: %v", err)
	}
	if !eqTypes(sink.types(), EventDelivered, EventConnecting, EventEstablished, EventCompleted) {
		t.Fatalf("after complete: %v", sink.types())
	}
}

func TestMockVoice_EngineCancel(t *testing.T) {
	m, sink, h := deliver(t)
	_ = m.Release(context.Background(), h, ReleaseCancelled)
	last := sink.events[len(sink.events)-1]
	if last.Type != EventCancelled || !last.Type.Terminal() {
		t.Fatalf("cancel terminal = %q", last.Type)
	}
}

func TestMockVoice_AdapterOriginatedTerminals(t *testing.T) {
	cases := []struct {
		name string
		do   func(*MockVoice, context.Context, Handle)
		want EventType
	}{
		{"failed", func(m *MockVoice, c context.Context, h Handle) { m.Fail(c, h, "boom") }, EventFailed},
		{"disconnected", func(m *MockVoice, c context.Context, h Handle) { m.Disconnect(c, h) }, EventDisconnected},
		{"caller_abandoned", func(m *MockVoice, c context.Context, h Handle) { m.Abandon(c, h) }, EventCallerLeft},
		{"rejected", func(m *MockVoice, c context.Context, h Handle) { m.Reject(c, h) }, EventRejected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, sink, h := deliver(t)
			tc.do(m, context.Background(), h)
			last := sink.events[len(sink.events)-1]
			if last.Type != tc.want || !last.Type.Terminal() {
				t.Fatalf("last = %q, want terminal %q", last.Type, tc.want)
			}
			// A second terminal of ANY kind is an idempotent no-op (no new event).
			n := len(sink.events)
			tc.do(m, context.Background(), h)
			_ = m.Release(context.Background(), h, ReleaseCompleted)
			if len(sink.events) != n {
				t.Fatalf("second terminal produced extra events: %v", sink.types())
			}
		})
	}
}

func TestMockVoice_FailedCarriesReason(t *testing.T) {
	m, sink, h := deliver(t)
	m.Fail(context.Background(), h, "adapter exploded")
	last := sink.events[len(sink.events)-1]
	if last.Reason != "adapter exploded" {
		t.Fatalf("reason = %q", last.Reason)
	}
}

// A redelivered event (same CorrelationID) is applied at most once by the sink.
func TestMockVoice_Idempotent(t *testing.T) {
	_, sink, _ := deliver(t)
	est := sink.events[2]
	_ = sink.OnAssignmentEvent(context.Background(), est)
	if got := len(sink.events); got != 3 {
		t.Fatalf("duplicate applied: %d events, want 3", got)
	}
}

// Release on an unknown handle is a no-op (not an error).
func TestMockVoice_ReleaseUnknown(t *testing.T) {
	m := NewMockVoice(fixedNow())
	if err := m.Release(context.Background(), "mock-voice:nope", ReleaseCompleted); err != nil {
		t.Fatalf("release unknown should be a no-op: %v", err)
	}
}

// A terminal that races mid-setup stops the setup sequence (no post-terminal
// events): inject a terminal from inside the sink on `connecting`.
func TestMockVoice_NoPostTerminalEvents(t *testing.T) {
	m := NewMockVoice(fixedNow())
	sink := newRecSink()
	racer := sinkFunc(func(ctx context.Context, ev AssignmentEvent) error {
		_ = sink.OnAssignmentEvent(ctx, ev)
		if ev.Type == EventConnecting {
			m.Abandon(ctx, ev.Handle) // outside world ends it mid-setup
		}
		return nil
	})
	_, _ = m.Deliver(context.Background(), Assignment{ReservationID: "r9"}, racer)
	// established must NOT appear after caller_abandoned.
	for i, e := range sink.events {
		if e.Type == EventEstablished {
			for _, prior := range sink.events[:i] {
				if prior.Type.Terminal() {
					t.Fatalf("established emitted after a terminal: %v", sink.types())
				}
			}
		}
	}
}

type sinkFunc func(context.Context, AssignmentEvent) error

func (f sinkFunc) OnAssignmentEvent(ctx context.Context, ev AssignmentEvent) error { return f(ctx, ev) }
