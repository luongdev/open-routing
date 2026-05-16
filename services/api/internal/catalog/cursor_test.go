// cursor_test.go — RED-phase tests for EncodeCursor/DecodeCursor (D-63).
//
// Cursor encoding is the only piece of cross-cutting helper logic worth
// unit-testing in isolation: it's a pure function with no DB, no Redis, no
// network. Mappers + errors are tested transitively via the per-entity
// integration tests (Plan 03-06..03-09).
package catalog

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestCursorRoundTrip — happy-path: encode (createdAt, id) then decode
// returns the same id + a CreatedAt within microsecond precision. D-63
// formats CreatedAt as RFC3339Nano via json.Marshal — Go's time.Time JSON
// codec preserves nanoseconds, but the assertion uses WithinDuration with
// time.Microsecond tolerance for safety against future codec tweaks.
func TestCursorRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 5, 16, 9, 30, 45, 123456789, time.UTC)
	id := uuid.Must(uuid.NewV7())

	s, err := EncodeCursor(createdAt, id)
	require.NoError(t, err)
	require.NotEmpty(t, s)

	cur, err := DecodeCursor(s)
	require.NoError(t, err)
	require.NotNil(t, cur)
	require.Equal(t, id, cur.ID)
	require.WithinDuration(t, createdAt, cur.CreatedAt, time.Microsecond)
}

// TestDecodeCursor_Empty — empty string signals "first page"; returns
// (nil, nil) so callers can branch without comparing to a sentinel.
func TestDecodeCursor_Empty(t *testing.T) {
	cur, err := DecodeCursor("")
	require.NoError(t, err)
	require.Nil(t, cur)
}

// TestDecodeCursor_Malformed — non-base64 input returns errBadCursor.
// Callers map errBadCursor to 400 invalid_body.
func TestDecodeCursor_Malformed(t *testing.T) {
	_, err := DecodeCursor("not-base64!!!")
	require.ErrorIs(t, err, errBadCursor)
}

// TestDecodeCursor_BadJSON — valid base64 of "not-json" decodes the
// outer layer but the inner json.Unmarshal fails; returns errBadCursor.
// base64("not-json") = "bm90LWpzb24=".
func TestDecodeCursor_BadJSON(t *testing.T) {
	s := "bm90LWpzb24=" // base64("not-json")
	_, err := DecodeCursor(s)
	require.ErrorIs(t, err, errBadCursor)
}

// TestEncodeCursor_Stable — same input twice returns identical output.
// Cursors must be deterministic so list endpoints can be replayed.
func TestEncodeCursor_Stable(t *testing.T) {
	createdAt := time.Date(2026, 5, 16, 9, 30, 45, 0, time.UTC)
	id := uuid.MustParse("01900000-0000-7000-8000-000000000001")

	s1, err1 := EncodeCursor(createdAt, id)
	s2, err2 := EncodeCursor(createdAt, id)

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.Equal(t, s1, s2)
}
