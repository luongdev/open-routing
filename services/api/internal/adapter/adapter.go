// Package adapter defines the v0.3 CHANNEL-NEUTRAL assignment contract: how an
// accepted assignment is delivered to an agent and how its lifecycle is reported
// back to the routing engine. It is deliberately NOT a media contract — the
// delivery Handle is opaque to the engine, so a real voice adapter (LiveKit/SIP,
// v0.4) and non-media channels (chat, email) implement the same interface.
//
// Lifecycle (each event idempotent under CorrelationID so a redelivered adapter
// callback is safe):
//
//	Deliver -> accepted -> connecting -> established -> completed
//	                                              \-> failed
//	                                              \-> disconnected
//	            (any time)                        \-> caller_abandoned
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
	EventAccepted        EventType = "accepted"         // the agent took the offer; delivery begins
	EventConnecting      EventType = "connecting"       // adapter is bridging agent <-> interaction
	EventEstablished     EventType = "established"      // both parties connected; handling started
	EventCompleted       EventType = "completed"        // handling finished normally -> WrapUp
	EventFailed          EventType = "failed"           // adapter-side failure (typed handling failure)
	EventDisconnected    EventType = "disconnected"     // the agent dropped mid-handling
	EventCallerAbandoned EventType = "caller_abandoned" // the customer hung up / left
)

// Terminal reports whether an event ends the assignment (no further events).
func (t EventType) Terminal() bool {
	switch t {
	case EventCompleted, EventFailed, EventDisconnected, EventCallerAbandoned:
		return true
	}
	return false
}

// Assignment is the unit handed to an adapter for delivery. The engine fills it
// from the accepted reservation; the adapter needs only enough to bridge.
type Assignment struct {
	ReservationID  string
	RouteRequestID string
	AgentID        string
	Channel        string
	Interaction    map[string]any
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
// onto the reservation lifecycle (accepted/completed/failed/...) idempotently by
// CorrelationID. Defined here so adapters depend only on the contract.
type EventSink interface {
	OnAssignmentEvent(ctx context.Context, ev AssignmentEvent) error
}

// ChannelAdapter delivers an accepted assignment to an agent and reports its
// lifecycle to the EventSink. One adapter per channel kind; Channel() is its key.
type ChannelAdapter interface {
	// Channel is the channel kind this adapter serves (e.g. "voice").
	Channel() string
	// Deliver bridges the assignment to the agent and returns an opaque handle.
	// It reports lifecycle via the sink (synchronously or later). A non-nil error
	// is a delivery setup failure (no handle, no events).
	Deliver(ctx context.Context, a Assignment, sink EventSink) (Handle, error)
	// Release tears down a delivery (normal completion, reassign, or abandonment).
	// Idempotent: releasing an unknown/already-released handle is not an error.
	Release(ctx context.Context, h Handle) error
}
