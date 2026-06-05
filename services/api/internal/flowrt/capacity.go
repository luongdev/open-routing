package flowrt

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// capacity.go is the v0.3 W3 DB-solid capacity gate. Each concurrent interaction
// an agent can hold on a channel is a ROW in agent_capacity_slots; the offer tx
// acquires a free slot under FOR UPDATE SKIP LOCKED (the authoritative gate —
// the candidate-source free-count is only a hint). Voice=1, chat=N. The service
// is stateless and operates on a caller-supplied tx-bound *Queries so an acquire
// composes with the route offer tx / its savepoint.

// channelCapacity is the per-channel concurrent-interaction capacity. v0.3 is
// voice-first (1); chat is N. This is the single source of truth for both
// provisioning and acquire-time filtering. TODO(v0.4): per-agent capacity config
// (D4 deferral) — until then a channel-code heuristic.
func channelCapacity(channel string) int32 {
	switch channel {
	case "chat":
		return chatCapacity
	default:
		return 1 // voice and everything else: one live interaction at a time
	}
}

const chatCapacity = 4

type CapacityService struct{}

func NewCapacityService() *CapacityService { return &CapacityService{} }

// ProvisionInTx ensures the agent's slot rows for the channel exist at the
// current capacity. Idempotent; shrinking is handled at acquire time (slot_no
// filter), not by deleting rows.
func (CapacityService) ProvisionInTx(ctx context.Context, q *generated.Queries, orgID, agentID uuid.UUID, channel string) error {
	return q.ProvisionCapacitySlots(ctx, generated.ProvisionCapacitySlotsParams{
		OrgID: pgUUID(orgID), AgentID: pgUUID(agentID), Channel: channel, Column4: channelCapacity(channel),
	})
}

// AcquireInTx claims a free slot for (agent, channel) as a PENDING hold expiring
// at holdUntil. Returns ok=false when the agent is at capacity (the caller aborts
// the offer). Errors are infra-only.
func (CapacityService) AcquireInTx(ctx context.Context, q *generated.Queries, orgID, agentID uuid.UUID, channel string, resID uuid.UUID, holdUntil time.Time) (int32, bool, error) {
	slot, err := q.AcquireCapacitySlot(ctx, generated.AcquireCapacitySlotParams{
		OrgID:         pgUUID(orgID),
		AgentID:       pgUUID(agentID),
		Channel:       channel,
		ReservationID: pgUUID(resID),
		HoldExpiresAt: ts(holdUntil),
		Column6:       channelCapacity(channel),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil // at capacity
	}
	if err != nil {
		return 0, false, err
	}
	return slot, true, nil
}

// ConfirmInTx promotes the reservation's pending hold to confirmed on accept.
// ok=false ⇒ the slot was lost (expired/swept) → the caller raises an accept
// conflict.
func (CapacityService) ConfirmInTx(ctx context.Context, q *generated.Queries, orgID, resID uuid.UUID) (bool, error) {
	n, err := q.ConfirmCapacitySlot(ctx, generated.ConfirmCapacitySlotParams{OrgID: pgUUID(orgID), ReservationID: pgUUID(resID)})
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// ReleaseInTx frees the reservation's slot on a terminal transition. Idempotent:
// 0 rows just means a sweep/reconcile freed it first — not an error.
func (CapacityService) ReleaseInTx(ctx context.Context, q *generated.Queries, orgID, resID uuid.UUID) error {
	_, err := q.ReleaseCapacitySlot(ctx, generated.ReleaseCapacitySlotParams{OrgID: pgUUID(orgID), ReservationID: pgUUID(resID)})
	return err
}
