package flowrt

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// lease.go is the v0.3 W4/D5 offer-lease binding: every offer mints a fresh
// lease_token (a secret the agent echoes back on accept/reject so a stale command
// for a superseded offer fails closed) and is delivered as a durable agent_outbox
// frame. agent_session_id records which connection got the offer (best-effort).

// bindOfferLease mints the per-offer lease token and resolves the agent's live
// session (NULL if none — the token alone still fences commands). Runs in the
// offer tx.
func bindOfferLease(ctx context.Context, qtx *generated.Queries, orgID, agentID uuid.UUID) (uuid.UUID, pgtype.UUID, error) {
	leaseToken := uuid.Must(uuid.NewV7())
	sid, err := qtx.GetLiveAgentSession(ctx, generated.GetLiveAgentSessionParams{OrgID: pgUUID(orgID), AgentID: pgUUID(agentID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return leaseToken, pgtype.UUID{}, nil
	}
	if err != nil {
		return uuid.Nil, pgtype.UUID{}, err
	}
	return leaseToken, sid, nil
}

// errAgentVanished signals the agent row disappeared between candidate selection
// and the offer frame — the offer cannot be delivered, so the caller must abandon
// it (skip the candidate / drop the claim) rather than commit a frame-less offer
// that would silently sit until it times out (cross-AI review HIGH).
var errAgentVanished = errors.New("flowrt: agent row gone — cannot deliver offer frame")

// enqueueOfferFrame writes the durable 'reservation.offer' outbox frame the agent
// WS relay delivers, carrying the reservation id + lease_token to echo back. The
// per-agent seq lock (LockAgentOutboxSeq) makes server_seq race-free; a 0-row lock
// means the agent row vanished → errAgentVanished so the offer is abandoned, never
// committed undeliverable. A duplicate event_key (ErrNoRows from the append) is an
// idempotent re-enqueue, not an error.
func enqueueOfferFrame(ctx context.Context, qtx *generated.Queries, orgID, agentID, resID, leaseToken uuid.UUID, expiresAt time.Time) error {
	locked, err := qtx.LockAgentOutboxSeq(ctx, generated.LockAgentOutboxSeqParams{OrgID: pgUUID(orgID), ID: pgUUID(agentID)})
	if err != nil {
		return err
	}
	if locked == 0 {
		return errAgentVanished
	}
	payload, _ := json.Marshal(map[string]any{
		"reservation_id": resID.String(),
		"lease_token":    leaseToken.String(),
		"expires_at":     expiresAt.UTC().Format(time.RFC3339Nano),
	})
	if _, err := qtx.AppendAgentOutbox(ctx, generated.AppendAgentOutboxParams{
		OrgID: pgUUID(orgID), AgentID: pgUUID(agentID),
		EventKey: "offer:" + resID.String(), Type: "reservation.offer",
		ReservationID: pgUUID(resID), Payload: payload,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}
