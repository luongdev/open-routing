// mappers.go — pure conversion helpers between sqlc-generated types
// (pgtype.X) and stdlib / api-package types. No I/O, no error returns,
// no domain logic — these are the "glue" functions every entity handler
// calls when building sqlc params or DTO responses.
//
// Why this file exists: pgx/v5's pgtype types use `{Bytes/Time/String,
// Valid bool}` records to express nullability. Mapping between
// pgtype.UUID and google/uuid.UUID (or pgtype.Timestamptz and *time.Time)
// for every column would be repetitive and easy to get wrong (e.g.,
// forgetting Valid=true on a non-null column). Centralising the
// conversions keeps the per-entity handler bodies readable.
package catalog

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// pgUUID wraps a google/uuid value for sqlc params. Always Valid=true —
// the catalog schema has no nullable UUID PRIMARY KEYs; callers needing
// to pass NULL should build pgtype.UUID{Valid: false} directly.
func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// apiUUID converts an sqlc-row pgtype.UUID back to google/uuid. The Valid
// flag is NOT checked — sqlc emits Valid=true for non-null columns, and
// the catalog schema has NO nullable UUID PRIMARY KEYs. If a future
// schema adds a nullable UUID column, callers must dispatch on
// pgUUID.Valid before calling apiUUID.
func apiUUID(p pgtype.UUID) uuid.UUID {
	return uuid.UUID(p.Bytes)
}

// derefOr returns *p or fallback when p is nil. Used for paging defaults
// (e.g., page_size fallback to 25 when the client omits the parameter).
func derefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

const (
	minPGInt4 = -1 << 31
	maxPGInt4 = 1<<31 - 1
)

func int32Checked(v int) (int32, bool) {
	if v < minPGInt4 || v > maxPGInt4 {
		return 0, false
	}
	return int32(v), true // #nosec G115 -- explicit range check above.
}

func mustInt32(v int) int32 {
	out, ok := int32Checked(v)
	if !ok {
		panic("catalog int32 conversion outside checked range")
	}
	return out
}

// jsonbToMap unmarshals a sqlc-row JSONB column ([]byte) into
// map[string]any per Pitfall 9 (Adapter.Config). NULL bytes (empty
// slice from a nullable JSONB or pgtype.Bytes Valid=false branch) →
// nil map. Decode errors are logged by the caller and surface to the
// client as 500 internal — callers should NOT have non-JSON bytes in
// JSONB columns (the migration declares JSONB, and Postgres rejects
// invalid JSON at INSERT time). Wave 3 codex review iter 1.
func jsonbToMap(b []byte) (map[string]any, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("jsonb decode: %w", err)
	}
	return m, nil
}

// mapToJSONB marshals a map[string]any to JSON bytes for an INSERT/
// UPDATE on a JSONB column. Nil map → empty bytes (Postgres stores
// SQL NULL). Wave 3 codex review iter 1.
func mapToJSONB(m map[string]any) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("jsonb encode: %w", err)
	}
	return b, nil
}
