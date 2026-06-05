// Package adapterwebhook is the v0.4 Wave 2 inbound assignment-event webhook: the
// network path a real, out-of-process channel adapter (LiveKit/SIP bridge) uses to
// report assignment lifecycle events back to the engine — the over-the-wire twin
// of the in-process adapter.EventSink.
//
// Authenticity is an HMAC-SHA256 signature over "timestamp.body" with a shared
// secret (Stripe-style), plus a replay window on the timestamp. The engine's
// EventSink already carries the ownership fence (the terminal is applied only when
// the event's handle matches the reservation's CURRENT bound handle and the
// reservation is still live) and is idempotent by handle — so a forged or replayed
// terminal for a stale handle is a no-op even past the signature check (defense in
// depth, cross-AI review HIGH).
package adapterwebhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
)

const (
	maxBody      = 1 << 16         // 64 KiB — an assignment event is tiny
	replayWindow = 5 * time.Minute // reject a signature older than this (replay guard)
	futureSkew   = 1 * time.Minute // tolerate this much forward clock skew, no more
	sigHeader    = "X-Or-Signature"
	tsHeader     = "X-Or-Timestamp"
)

// eventPayload is the webhook wire shape, mapped onto adapter.AssignmentEvent. The
// wire format lives here (not on the core struct) so the contract is the webhook's
// concern, not the adapter interface's.
type eventPayload struct {
	Type          string `json:"type"`
	Handle        string `json:"handle"`
	ReservationID string `json:"reservation_id"`
	CorrelationID string `json:"correlation_id"`
	Reason        string `json:"reason,omitempty"`
}

// Handler verifies the signature + replay window, then hands the event to the
// sink. secret MUST be non-empty (the caller only mounts the route when it is).
// now is injectable for tests; nil ⇒ time.Now.
func Handler(sink adapter.EventSink, secret string, logger *slog.Logger, now func() time.Time) http.HandlerFunc {
	if now == nil {
		now = time.Now
	}
	key := []byte(secret)
	return func(w http.ResponseWriter, r *http.Request) {
		// MaxBytesReader REJECTS an oversized body (it doesn't truncate) — so we never
		// verify the HMAC over a truncated prefix of a larger real body (review MED).
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large or unreadable", http.StatusRequestEntityTooLarge)
			return
		}
		tsStr := r.Header.Get(tsHeader)
		tsec, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			http.Error(w, "bad timestamp", http.StatusBadRequest)
			return
		}
		// Reject a stale OR a meaningfully future-dated timestamp (only a small clock
		// skew is tolerated forward), bounding replay of a captured request even with
		// a valid signature (review MED).
		delta := now().Unix() - tsec // >0 = past, <0 = future
		if delta > int64(replayWindow.Seconds()) || delta < -int64(futureSkew.Seconds()) {
			http.Error(w, "stale or future timestamp", http.StatusUnauthorized)
			return
		}
		// HMAC over "timestamp.body" so the timestamp can't be tampered independently.
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(tsStr))
		mac.Write([]byte("."))
		mac.Write(body)
		want := hex.EncodeToString(mac.Sum(nil))
		got := r.Header.Get(sigHeader)
		if !hmac.Equal([]byte(want), []byte(got)) {
			http.Error(w, "bad signature", http.StatusUnauthorized)
			return
		}

		var p eventPayload
		if json.Unmarshal(body, &p) != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		ev := adapter.AssignmentEvent{
			Type:          adapter.EventType(p.Type),
			Handle:        adapter.Handle(p.Handle),
			ReservationID: p.ReservationID,
			CorrelationID: p.CorrelationID,
			Reason:        p.Reason,
			At:            now(),
		}
		// The sink owns the ownership fence + idempotency; a non-nil error is
		// transient (adapter should retry) → 500, else 200.
		if err := sink.OnAssignmentEvent(r.Context(), ev); err != nil {
			if logger != nil {
				logger.ErrorContext(r.Context(), "assignment webhook sink error", "handle", p.Handle, "type", p.Type, "err", err)
			}
			http.Error(w, "sink error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// Sign computes the header values a caller (the remote adapter / a test) sends. It
// is the canonical client side of the scheme, exported so the remote-mock adapter
// and tests can't drift from the verifier.
func Sign(secret string, ts time.Time, body []byte) (timestamp, signature string) {
	timestamp = strconv.FormatInt(ts.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return timestamp, hex.EncodeToString(mac.Sum(nil))
}
