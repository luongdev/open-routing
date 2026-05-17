// row_break_reason.go — per-row processor for entity=break_reasons
// (D5-15 / IMP-03 / IDENT-03).
//
// Flat shape. Phase 04.1 dropped the legacy UNIQUE(org_id, name)
// constraint — multiple break_reasons with the same `name` SAME `org`
// are now legal as long as their `code` values differ. The 409 path
// keys on (org_id, code) or partial (org_id, external_id) only.
//
// Processor pipeline:
//
//   1. reMarshalAs into ImportBreakReasonRequest.
//   2. Layer 1 — validateCodeFormat on `code`.
//   3. qtx.UpsertBreakReasonByCode.
//   4. Wrap pg errors via wrapPgError.
//   5. Return succeededRow on success.
//
// `routable` and `display_order` are optional with sensible defaults
// (routable defaults true — most break reasons are routable; admin
// must opt out for "Lunch" / "Training"; display_order defaults 0).
package imports

import (
	"context"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/catalog"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

type breakReasonRowProc struct {
	handlers *Importer
}

func (p *breakReasonRowProc) process(
	ctx context.Context,
	sp *db.OrgTx,
	orgID uuid.UUID,
	r parsedRow,
	_ *chunkSkillResolver,
) (succeededRow, *rowError) {
	typed, err := reMarshalAs[api.ImportBreakReasonRequest](r.raw)
	if err != nil {
		return succeededRow{}, &rowError{Field: "", Reason: "invalid_json_row"}
	}
	if !catalog.ValidateCodeFormat(typed.Code) {
		return succeededRow{}, &rowError{Field: "code", Reason: "invalid_code_format"}
	}

	routable := derefBool(typed.Routable, true)
	displayOrder := int32(derefInt(typed.DisplayOrder, 0))

	id := uuid.Must(uuid.NewV7())
	qtx := generated.New(sp)
	row, upErr := qtx.UpsertBreakReasonByCode(ctx, generated.UpsertBreakReasonByCodeParams{
		ID:           pgUUID(id),
		OrgID:        pgUUID(orgID),
		Code:         typed.Code,
		ExternalID:   typed.ExternalId,
		Name:         typed.Name,
		Routable:     routable,
		DisplayOrder: displayOrder,
		Enabled:      derefBool(typed.Enabled, true),
	})
	if upErr != nil {
		return succeededRow{}, wrapPgError(upErr, "break_reason")
	}
	return succeededRow{
		id:     uuid.UUID(row.ID.Bytes),
		entity: api.BreakReasons,
		lineNo: r.lineNo,
	}, nil
}
