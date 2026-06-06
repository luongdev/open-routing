// Package adapter defines the v0.3 CHANNEL-NEUTRAL assignment contract: how an
// accepted assignment is delivered to an agent and how its lifecycle is reported
// back to the routing engine. It is deliberately NOT a media contract — the
// delivery Handle is opaque to the engine, so a real voice adapter (LiveKit/SIP,
// v0.4) and non-media channels (chat, email) implement the same interface.
//
// Lifecycle (each event idempotent under CorrelationID; the engine applies each
// at most once). After Deliver the adapter drives the setup sequence and reports
// EVERY transition — including any terminal — through the EventSink:
//
//	Deliver -> delivered -> connecting -> established -> completed
//	   any non-terminal state -> { failed | rejected | disconnected | caller_abandoned | cancelled }
//
// Two terminal sources:
//   - ADAPTER-ORIGINATED (failed, rejected, disconnected, caller_abandoned): the
//     outside world (media stack / caller / agent device). The engine OBSERVES
//     these via the sink.
//   - ENGINE-INITIATED (completed, cancelled): the engine ends the delivery via
//     Release(cause); the adapter tears down and emits the matching terminal so
//     both sources converge on one observable terminal per assignment.
//
// Sink contract: OnAssignmentEvent is called in lifecycle order per handle. A
// returned error means the engine did not durably accept the event; the adapter
// SHOULD retry (the engine is idempotent by CorrelationID). The engine MUST treat
// any terminal as final and ignore later events for the same handle.
package adapter

import (
	"context"
	"time"
)

// Handle is an opaque, adapter-owned reference to an in-flight delivery. The
// engine stores it verbatim and hands it back to Release — it never inspects it
// (no media/room assumptions), so chat/email fit the same contract.
type Handle string

// EventType is the channel-neutral assignment lifecycle.
type EventType string

const (
	// EventDelivered: the adapter began delivering to the agent (NOT the matcher's
	// reservation acceptance, which already happened — this is delivery-start).
	EventDelivered    EventType = "delivered"
	EventConnecting   EventType = "connecting"   // bridging agent <-> interaction
	EventEstablished  EventType = "established"  // both parties connected; handling started
	EventCompleted    EventType = "completed"    // engine-initiated normal end -> WrapUp
	EventCancelled    EventType = "cancelled"    // engine-initiated teardown (reassign / cancel)
	EventFailed       EventType = "failed"       // adapter-side fault (typed handling failure)
	EventRejected     EventType = "rejected"     // the agent device declined (distinct from a fault)
	EventDisconnected EventType = "disconnected" // the agent dropped mid-handling
	EventCallerLeft   EventType = "caller_abandoned"
)

// Terminal reports whether an event ends the assignment (no further events). Any
// of these may occur from ANY non-terminal state (setup can fail before
// established; a caller can abandon while it rings).
func (t EventType) Terminal() bool {
	switch t {
	case EventCompleted, EventCancelled, EventFailed, EventRejected, EventDisconnected, EventCallerLeft:
		return true
	}
	return false
}

// ReleaseCause is why the engine is ending a delivery (engine-initiated). It maps
// to the terminal event the adapter emits on Release.
type ReleaseCause string

const (
	ReleaseCompleted ReleaseCause = "completed" // agent finished handling normally
	ReleaseCancelled ReleaseCause = "cancelled" // reassign / cancel / caller-gone cleanup
)

func (c ReleaseCause) event() EventType {
	if c == ReleaseCompleted {
		return EventCompleted
	}
	return EventCancelled
}

// Assignment is the unit handed to an adapter for delivery. The engine fills it
// from the reservation the matcher accepted; the adapter needs only enough to
// bridge.
type Assignment struct {
	ReservationID  string
	RouteRequestID string
	AgentID        string
	Channel        string
	Interaction    map[string]any
	// IdempotencyKey is the durable delivery-attempt id. Deliver MUST be idempotent
	// on it (a redelivered command with the same key maps to the SAME media session,
	// never a second room) so the at-least-once delivery outbox can't leak sessions
	// across retries/crashes (cross-AI review BLOCK).
	IdempotencyKey string
}

// AssignmentEvent is one lifecycle transition reported by an adapter. CorrelationID
// dedupes redelivered callbacks (the engine applies each at most once); Reason
// carries a terminal cause (e.g. the failure/abandon reason).
type AssignmentEvent struct {
	Type          EventType
	Handle        Handle
	ReservationID string
	CorrelationID string
	Reason        string
	At            time.Time
}

// EventSink receives adapter events. The engine implements this and maps each
// onto the reservation lifecycle idempotently by CorrelationID. Defined here so
// adapters depend only on the contract.
type EventSink interface {
	OnAssignmentEvent(ctx context.Context, ev AssignmentEvent) error
}

// ChannelAdapter delivers an accepted assignment to an agent and reports its
// lifecycle to the EventSink. One adapter per channel kind; Channel() is its key.
type ChannelAdapter interface {
	// Channel is the channel kind this adapter serves (e.g. "voice").
	Channel() string
	// Deliver registers a delivery and returns its opaque handle; the lifecycle
	// (delivered -> ... -> terminal, including any setup failure) is reported via
	// the sink, ordered per handle. A non-nil error means delivery could not be
	// registered AT ALL — no handle, no events. Once a handle is returned, every
	// terminal path is observable as a terminal sink event.
	Deliver(ctx context.Context, a Assignment, sink EventSink) (Handle, error)
	// Release ends a delivery the ENGINE initiated (cause) and emits the matching
	// terminal event exactly once. Idempotent: releasing an already-terminal or
	// unknown handle is a no-op (nil), so a redelivered teardown is safe.
	Release(ctx context.Context, h Handle, cause ReleaseCause) error
}
