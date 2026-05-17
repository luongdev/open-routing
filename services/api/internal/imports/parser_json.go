// parser_json.go — F1 accommodation helper.
//
// The strict-server eagerly decodes the application/json request body
// into BulkImportCatalogJSONRequestBody (which is type-aliased to
// []interface{} per F2 — see api/types.gen.go:1448). By the time the
// handler runs, req.JSONBody points at an already-decoded slice of
// untyped values.
//
// Per-row, we need to coerce each []interface{} element into the
// typed Import*Request shape (api.ImportAgentRequest, etc.). The
// cheapest round-trip that preserves field-tag semantics is to
// json.Marshal the interface{} item back to bytes, then json.Unmarshal
// into the typed target. Failures isolate to the single row — the
// chunk loop captures them in BulkImportFailedRow.
//
// Memory bound (RESEARCH §A6 + threat T-05-04-04): the strict-server
// decoder is gated by http.MaxBytesReader(50 MB) via the D5-21
// middleware, so the worst-case live []interface{} sits inside the
// 50 MB → ~300 MB working set ceiling. Per-row re-marshal allocates
// O(row size) transient bytes; the call site cap of 500 rows (D5-22)
// keeps the cumulative transient memory bounded.
package imports

import (
	"encoding/json"
	"fmt"
)

// reMarshalAs is the type-parameterised round-trip helper. Usage:
//
//	typed, err := reMarshalAs[api.ImportAgentRequest](rawItem)
//	if err != nil {
//	    // record per-row failure with field="", reason from err
//	}
//
// The two-step Marshal+Unmarshal isolates per-row schema errors. A
// row whose JSON shape does not match T (missing required field —
// detected only when T uses non-pointer fields; type mismatch on a
// present field) returns the underlying json.Unmarshal error wrapped
// for context.
//
// A row whose interface{} value cannot be re-marshalled (e.g. a
// function value, which the eager decoder cannot produce but a
// programmatic caller could synthesise) returns the Marshal error
// wrapped for context. Both wraps preserve errors.Is identity.
func reMarshalAs[T any](item interface{}) (T, error) {
	var zero T
	raw, err := json.Marshal(item)
	if err != nil {
		return zero, fmt.Errorf("re-marshal: %w", err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, fmt.Errorf("re-unmarshal: %w", err)
	}
	return out, nil
}
