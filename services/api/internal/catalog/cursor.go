// cursor.go — opaque cursor encode/decode for catalog list endpoints (D-63).
//
// Cursor format: base64(JSON{"created_at": RFC3339Nano, "id": UUIDv7}).
// Client-opaque, server-decodable. The composite (created_at, id) is stable
// under inserts because UUIDv7 carries a millisecond-precision timestamp,
// so cursor ordering aligns with insertion order even across same-ms
// timestamps (the id tiebreaker is monotonic within a single process).
//
// Cursors are NOT signed in v0.1 — clients can forge them, but queries
// still org-scope (T-3-18 in 03-05 PLAN.md), so the worst-case probe
// enumerates the caller's own org. HMAC signing deferred to v0.2 hardening.
package catalog

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// errBadCursor signals an undecodable / malformed cursor. Handlers map
// this to 400 invalid_body (the strict-server pipeline never sees a
// non-cursor-shaped string because the schema rejects non-string types
// at decode time).
var errBadCursor = errors.New("catalog: bad cursor")

// Cursor is the decoded payload. The JSON tags match the wire shape
// verbatim — do NOT rename the fields without updating any persisted
// cursor a client might still hold (cursors are ephemeral, but external
// integrations may cache them in workflow state).
type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// EncodeCursor packs (createdAt, id) into a base64-encoded JSON string.
// The result is suitable for the `?cursor=` query parameter.
//
// Returns the empty string + nil error never — encoding is deterministic
// and json.Marshal of {time.Time, uuid.UUID} cannot fail in practice
// (uuid.UUID has a stable MarshalJSON; time.Time always serialises to
// RFC3339Nano). The error return preserves the option to swap in HMAC
// signing later without a breaking API change.
func EncodeCursor(createdAt time.Time, id uuid.UUID) (string, error) {
	b, err := json.Marshal(Cursor{CreatedAt: createdAt, ID: id})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeCursor inverts EncodeCursor. The empty string returns (nil, nil)
// — the first-page case where no cursor is supplied. Any decode failure
// (base64 or JSON) returns errBadCursor; callers map to 400 invalid_body.
//
// DecodeCursor does NOT validate semantic plausibility (e.g., createdAt
// way in the future). The downstream sqlc query handles that — out-of-
// range cursors simply return no rows.
func DecodeCursor(s string) (*Cursor, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, errBadCursor
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, errBadCursor
	}
	return &c, nil
}
