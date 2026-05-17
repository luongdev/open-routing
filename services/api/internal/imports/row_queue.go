// row_queue.go — per-row processor for entity=queues (D5-15 / IMP-03).
//
// Flat shape (no FK, no nested arrays). channel_types is a []string
// that the CSV path's coerceChannelTypesCell splits via splitMulti
// (D5-04 priority `|` > `;` > `,`); the JSON path receives the array
// directly. Per-token enum validation (D5-06 — voice/chat/email) is
// the server contract's job at the DB CHECK level — the row processor
// does not enumerate the closed set here; an invalid token reaches
// the DB and surfaces as 23514 → invalid_value via MapPgError.
//
// Empty channel_types is a server-contract violation (the OpenAPI
// schema marks the field required); an empty array triggers a 23514
// CHECK if the schema enforces non-empty, or surfaces as a per-row
// failure via the UpsertQueueByCode call when the column's
// constraints are otherwise asserted. v0.1 simply lets the DB-level
// CHECK or NOT NULL produce the failure.
package imports

import (
	"context"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/catalog"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

type queueRowProc struct {
	handlers *Importer
}

func (p *queueRowProc) process(
	ctx context.Context,
	sp *db.OrgTx,
	orgID uuid.UUID,
	r parsedRow,
	_ *chunkSkillResolver,
) (succeededRow, *rowError) {
	typed, err := reMarshalAs[api.ImportQueueRequest](r.raw)
	if err != nil {
		return succeededRow{}, &rowError{Field: "", Reason: "invalid_json_row"}
	}
	if !catalog.ValidateCodeFormat(typed.Code) {
		return succeededRow{}, &rowError{Field: "code", Reason: "invalid_code_format"}
	}

	// priority + acw_sec are admin-supplied ints; OpenAPI schema bounds
	// them within int32 range (CSV coerce + JSON Decode both reject
	// overflow); safe int → int32.
	priority := int32(derefInt(typed.Priority, 0)) //nolint:gosec // schema-bounded int32 range
	acwSec := int32(derefInt(typed.AcwSec, 0))     //nolint:gosec // schema-bounded int32 range

	id := uuid.Must(uuid.NewV7())
	qtx := generated.New(sp)
	row, upErr := qtx.UpsertQueueByCode(ctx, generated.UpsertQueueByCodeParams{
		ID:           pgUUID(id),
		OrgID:        pgUUID(orgID),
		Code:         typed.Code,
		ExternalID:   typed.ExternalId,
		Name:         typed.Name,
		ChannelTypes: typed.ChannelTypes,
		Priority:     priority,
		AcwSec:       acwSec,
		Enabled:      derefBool(typed.Enabled, true),
	})
	if upErr != nil {
		return succeededRow{}, wrapPgError(upErr, "queue")
	}
	return succeededRow{
		id:     uuid.UUID(row.ID.Bytes),
		entity: api.Queues,
		lineNo: r.lineNo,
	}, nil
}

// derefInt returns *p or fallback. Distinct from derefBool to avoid
// generic-instantiation overhead; both helpers live alongside the row
// processors where they are exclusively used.
//
//nolint:unused // Used by row_queue.go and row_break_reason.go.
func derefInt(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}
