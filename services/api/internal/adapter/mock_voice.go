package adapter

import (
	"context"
	"strings"
	"sync"
	"time"
)

const mockVoicePrefix = "mock-voice:"

// MockVoice is the v0.3 stand-in for a real voice adapter (LiveKit/SIP lands in
// v0.4 behind this same contract). It performs NO media: Deliver drives
// delivered -> connecting -> established through the sink, and a terminal
// transition is driven either by the engine (Release) or — to simulate the
// outside world in tests — by Fail/Disconnect/Abandon/Reject. Deterministic: the
// handle derives from the reservation id and events carry a stable correlation id.
//
// Concurrency: every terminal transition is a compare-and-set under the mutex
// (review fix #2/#6 — exactly one terminal per handle, redelivery is a no-op);
// the Deliver setup loop re-checks liveness before each emit (review fix #3/#5).
type MockVoice struct {
	mu   sync.Mutex
	now  func() time.Time
	live map[Handle]*mvState
}

type mvState struct {
	sink       EventSink
	terminated bool
}

// NewMockVoice builds a mock voice adapter. now defaults to time.Now when nil.
func NewMockVoice(now func() time.Time) *MockVoice {
	if now == nil {
		now = time.Now
	}
	return &MockVoice{now: now, live: map[Handle]*mvState{}}
}

func (m *MockVoice) Channel() string { return "voice" }

func (m *MockVoice) Deliver(ctx context.Context, a Assignment, sink EventSink) (Handle, error) {
	h := Handle(mockVoicePrefix + a.ReservationID)
	m.mu.Lock()
	m.live[h] = &mvState{sink: sink}
	m.mu.Unlock()
	// No media — bring the assignment to "established" right away. Re-check
	// liveness before each emit so a terminal that races in mid-setup stops the
	// sequence (no post-terminal events).
	for _, t := range []EventType{EventDelivered, EventConnecting, EventEstablished} {
		if !m.isLive(h) {
			break
		}
		_ = m.emit(ctx, sink, a.ReservationID, h, t, "")
	}
	return h, nil
}

// Release ends a delivery the engine initiated (ReleaseCompleted / ReleaseCancelled)
// and emits the matching terminal. Idempotent: an already-terminal or unknown
// handle is a no-op.
func (m *MockVoice) Release(ctx context.Context, h Handle, cause ReleaseCause) error {
	return m.fireTerminal(ctx, h, cause.event(), "")
}

// Fail / Disconnect / Abandon / Reject simulate ADAPTER-ORIGINATED terminals (the
// outside world) for tests. All idempotent no-ops on an already-terminal handle;
// a sink error leaves the handle live so the next call re-fires (retry contract).
func (m *MockVoice) Fail(ctx context.Context, h Handle, reason string) {
	_ = m.fireTerminal(ctx, h, EventFailed, reason)
}
func (m *MockVoice) Disconnect(ctx context.Context, h Handle) {
	_ = m.fireTerminal(ctx, h, EventDisconnected, "agent_disconnected")
}
func (m *MockVoice) Abandon(ctx context.Context, h Handle) {
	_ = m.fireTerminal(ctx, h, EventCallerLeft, "caller_hung_up")
}
func (m *MockVoice) Reject(ctx context.Context, h Handle) {
	_ = m.fireTerminal(ctx, h, EventRejected, "declined_by_device")
}

func (m *MockVoice) isLive(h Handle) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.live[h]
	return ok && !s.terminated
}

// fireTerminal compare-and-sets the terminated tombstone under the lock so only
// one caller wins, then emits the terminal event OUTSIDE the lock (the sink may
// re-enter the adapter — e.g. the engine calling Release — so holding the lock
// across emit would deadlock).
func (m *MockVoice) fireTerminal(ctx context.Context, h Handle, t EventType, reason string) error {
	m.mu.Lock()
	s, ok := m.live[h]
	if !ok || s.terminated {
		m.mu.Unlock()
		return nil
	}
	s.terminated = true
	sink := s.sink
	m.mu.Unlock()
	if err := m.emit(ctx, sink, strings.TrimPrefix(string(h), mockVoicePrefix), h, t, reason); err != nil {
		// The engine did not durably accept the terminal → roll back the tombstone
		// so a retry re-fires it (adapter retry contract). The CAS still prevents a
		// double-fire on success.
		m.mu.Lock()
		if s2, ok := m.live[h]; ok {
			s2.terminated = false
		}
		m.mu.Unlock()
		return err
	}
	return nil
}

func (m *MockVoice) emit(ctx context.Context, sink EventSink, resID string, h Handle, t EventType, reason string) error {
	return sink.OnAssignmentEvent(ctx, AssignmentEvent{
		Type:          t,
		Handle:        h,
		ReservationID: resID,
		CorrelationID: string(h) + ":" + string(t), // stable: each event-type once per handle
		Reason:        reason,
		At:            m.now(),
	})
}
