// row_adapter.go — per-row processor for entity=adapters
// (D-72 / D04_1-13 / IMP-03).
//
// Adapter shape: flat top-level + JSONB `config` passthrough. The
// processor:
//
//   1. reMarshalAs into ImportAdapterRequest.
//   2. Layer 1 validateCodeFormat.
//   3. Marshal Config (*map[string]interface{}) into []byte for the
//      JSONB column. Nil/empty → "{}" so Postgres stores a valid
//      object (matches catalog/adapters.go CreateAdapter behaviour).
//   4. qtx.UpsertAdapterByCode.
//   5. Wrap pg errors via wrapPgError.
//
// 409 path (D04_1-13): Phase 04.1 added the 409 wrapper because
// adapters previously had no UNIQUE constraint. Phase 5 imports
// reach the same 409 path on duplicate_code / duplicate_external_id;
// wrapPgError translates to row-level reason verbatim (catalog
// MapPgError owns the introspection).
package imports

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/catalog"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

type adapterRowProc struct {
	handlers *Importer
}

func (p *adapterRowProc) process(
	ctx context.Context,
	sp *db.OrgTx,
	orgID uuid.UUID,
	r parsedRow,
	_ *chunkSkillResolver,
) (succeededRow, *rowError) {
	typed, err := reMarshalAs[api.ImportAdapterRequest](r.raw)
	if err != nil {
		return succeededRow{}, &rowError{Field: "", Reason: "invalid_json_row"}
	}
	if !catalog.ValidateCodeFormat(typed.Code) {
		return succeededRow{}, &rowError{Field: "code", Reason: "invalid_code_format"}
	}

	// Marshal Config to JSONB bytes. Nil / empty map → "{}" so the
	// column stores a valid JSONB object (matches catalog/adapters.go
	// CreateAdapter behaviour at lines 41-59).
	var cfgBytes []byte
	if typed.Config != nil {
		b, mErr := json.Marshal(*typed.Config)
		if mErr != nil {
			return succeededRow{}, &rowError{Field: "config", Reason: "config_marshal_failed"}
		}
		cfgBytes = b
	}
	if len(cfgBytes) == 0 {
		cfgBytes = []byte("{}")
	}

	id := uuid.Must(uuid.NewV7())
	qtx := generated.New(sp)
	row, upErr := qtx.UpsertAdapterByCode(ctx, generated.UpsertAdapterByCodeParams{
		ID:          pgUUID(id),
		OrgID:       pgUUID(orgID),
		Code:        typed.Code,
		ExternalID:  typed.ExternalId,
		Name:        typed.Name,
		AdapterType: typed.AdapterType,
		Config:      cfgBytes,
		Enabled:     derefBool(typed.Enabled, true),
	})
	if upErr != nil {
		return succeededRow{}, wrapPgError(upErr, "adapter")
	}
	return succeededRow{
		id:     uuid.UUID(row.ID.Bytes),
		entity: api.Adapters,
		lineNo: r.lineNo,
	}, nil
}
