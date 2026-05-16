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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// pgUUID wraps a google/uuid value for sqlc params. Always Valid=true —
// the catalog schema has no nullable UUID PRIMARY KEYs; callers needing
// to pass NULL should build pgtype.UUID{Valid: false} directly.
func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// pgUUIDFromBytes wraps a raw 16-byte array. Used when sqlc params expose
// a [16]byte rather than uuid.UUID (rare; included for completeness).
func pgUUIDFromBytes(b [16]byte) pgtype.UUID {
	return pgtype.UUID{Bytes: b, Valid: true}
}

// apiUUID converts an sqlc-row pgtype.UUID back to google/uuid. The Valid
// flag is NOT checked — sqlc emits Valid=true for non-null columns, and
// the catalog schema has NO nullable UUID PRIMARY KEYs. If a future
// schema adds a nullable UUID column, callers must dispatch on
// pgUUID.Valid before calling apiUUID.
func apiUUID(p pgtype.UUID) uuid.UUID {
	return uuid.UUID(p.Bytes)
}

// pgTimestamptzPtr wraps an optional time pointer. Nil → Valid=false
// (SQL NULL). Used for cursor parameters where the absence of a cursor
// signals "first page" and the sqlc-named-arg becomes NULL.
func pgTimestamptzPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// pgTextPtr wraps an optional string pointer. Nil → Valid=false. Used
// for nullable name-search and optional-string filter params.
func pgTextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// deref returns *p or the zero value of T when p is nil. Used to flatten
// UpdateXRequest pointer fields (e.g., *string) to concrete values when
// the sqlc param is non-pointer. The caller is responsible for branching
// on nil BEFORE invoking the update if absent-means-leave-unchanged
// semantics matter — deref's zero-value fallback intentionally drops
// information about the nil case.
func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// derefOr returns *p or fallback when p is nil. Used for paging defaults
// (e.g., page_size fallback to 25 when the client omits the parameter).
func derefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}
