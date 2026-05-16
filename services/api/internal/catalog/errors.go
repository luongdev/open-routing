// errors.go — pgx/pgconn error → strict-server response triple.
//
// Every catalog handler that touches the DB calls mapPgError on the
// non-happy-path return to translate pgx errors into the locked
// (httpStatus, ErrorCode, reason) triple, then wraps the triple in the
// operation-specific *JSONResponse type.
//
// Translations:
//   - pgx.ErrNoRows          → 404 / not_found        / "{entity}_not_found"
//   - pgconn 23505 (UNIQUE)  → 409 / version_conflict / "external_id_collision"
//   - everything else        → 500 / internal         / "internal"
//
// Note: 23505 is mapped to version_conflict because the catalog schema's
// only application-level uniqueness constraints are external_id-style
// (e.g., agent.external_id), which the user perceives as a duplicate-
// id error rather than a generic violation. v0.2 may add 23503 (FK) +
// 23514 (CHECK) translations when the catalog grows cross-table FKs.
package catalog

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// mapPgError translates a sqlc/pgx-returned error into the canonical
// (httpStatus, ErrorCode, reason) triple. The handler then wraps the
// triple in the operation-specific *JSONResponse type — this function
// stays response-type-agnostic so it can be reused across all 6 entities.
//
// `entity` is the short slug ("agent", "skill", etc.) used to construct
// the 404 reason ("{entity}_not_found"). It is NOT used for any other
// branch — keeping the call sites uniform.
func mapPgError(err error, entity string) (int, api.ErrorCode, string) {
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound, api.ErrorCodeNotFound, entity + "_not_found"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return http.StatusConflict, api.ErrorCodeVersionConflict, "external_id_collision"
	}
	return http.StatusInternalServerError, api.ErrorCodeInternal, "internal"
}
