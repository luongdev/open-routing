package adapter

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockVoice is the v0.3 stand-in for a real voice adapter (LiveKit/SIP lands in
// v0.4 behind this same contract). It performs NO media: Deliver immediately
// drives accepted -> connecting -> established through the sink, and the terminal
// transition (completed / failed / disconnected / caller_abandoned) is driven by
// the engine or a test via the exposed methods. Deterministic: the handle is
// derived from the reservation id and events carry a stable correlation id.
type MockVoice struct {
	mu   sync.Mutex
	now  func() time.Time
	live map[Handle]EventSink // handle -> sink for later terminal events
}

// NewMockVoice builds a mock voice adapter. now defaults to time.Now when nil.
func NewMockVoice(now func() time.Time) *MockVoice {
	if now == nil {
		now = time.Now
	}
	return &MockVoice{now: now, live: map[Handle]EventSink{}}
}

func (m *MockVoice) Channel() string { return "voice" }

func (m *MockVoice) Deliver(ctx context.Context, a Assignment, sink EventSink) (Handle, error) {
	h := Handle("mock-voice:" + a.ReservationID)
	m.mu.Lock()
	m.live[h] = sink
	m.mu.Unlock()
	// No media — bring the assignment to "established" right away so the engine's
	// handling phase can run. Each step is idempotent under its correlation id.
	for _, t := range []EventType{EventAccepted, EventConnecting, EventEstablished} {
		if err := m.emit(ctx, sink, a.ReservationID, h, t, ""); err != nil {
			return h, err
		}
	}
	return h, nil
}

func (m *MockVoice) Release(_ context.Context, h Handle) error {
	m.mu.Lock()
	delete(m.live, h) // idempotent: deleting an absent key is a no-op
	m.mu.Unlock()
	return nil
}

// Complete / Fail / Disconnect / Abandon drive the terminal transition for a live
// handle (the engine calls these from real signals; tests call them directly). A
// terminal event releases the handle.
func (m *MockVoice) Complete(ctx context.Context, h Handle) error {
	return m.terminal(ctx, h, EventCompleted, "")
}
func (m *MockVoice) Fail(ctx context.Context, h Handle, reason string) error {
	return m.terminal(ctx, h, EventFailed, reason)
}
func (m *MockVoice) Disconnect(ctx context.Context, h Handle) error {
	return m.terminal(ctx, h, EventDisconnected, "agent_disconnected")
}
func (m *MockVoice) Abandon(ctx context.Context, h Handle) error {
	return m.terminal(ctx, h, EventCallerAbandoned, "caller_hung_up")
}

func (m *MockVoice) terminal(ctx context.Context, h Handle, t EventType, reason string) error {
	m.mu.Lock()
	sink, ok := m.live[h]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("adapter: unknown or already-terminated handle %q", h)
	}
	resID := string(h)[len("mock-voice:"):]
	if err := m.emit(ctx, sink, resID, h, t, reason); err != nil {
		return err
	}
	return m.Release(ctx, h)
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
