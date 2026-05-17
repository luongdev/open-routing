// header.go — strict D5-05 header validation + per-entity column
// registry.
//
// The registry is a data table (matrix-as-data, mirror of
// state/transitions.go) keyed by api.ImportEntityType. Each entry
// enumerates every column the entity's CSV may contain, marks
// required vs optional, and pins the coercion callback the chunk
// loop (Plan 05-05) will invoke for that column.
//
// validateHeader implements D5-05 strict-batch policy:
//
//   - Missing required column → batch-level 400 (D5-05). All required
//     columns must appear; case-sensitive (`email` ≠ `Email`).
//   - Unknown column → batch-level 400 (D5-05). Forward-compat with
//     v0.2 column adds: a v0.2 column arriving with a v0.1 server
//     produces a clear error rather than silently dropping data.
package imports

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/catalog"
)

// columnSpec describes a single CSV column for a single entity. The
// coerce callback signature is `(string) (any, error)` so the chunk
// loop can hold a heterogeneous result slice without per-column
// type-switch boilerplate; Plan 05-05's row processor reads the typed
// result back via the spec's name.
type columnSpec struct {
	name     string
	required bool
	coerce   func(string) (any, error)
}

// entityColumnRegistry is the ordered list of columns for one entity.
// Order matters for the data layout (it pins the CSV header order
// admins should follow), but validateHeader is order-tolerant — it
// checks set membership only.
type entityColumnRegistry []columnSpec

// entityRegistry pins the column registry for every entity that
// Phase 5 imports. The columns mirror each entity's Import*Request
// schema (api/types.gen.go lines 835-944) one-for-one:
//
//   - All `code` columns: required, route through coerceCode
//     (delegates to catalog.ValidateCodeFormat).
//   - All `external_id` columns: optional, route through coerceString
//     (D04_1-07 — integration-mapping metadata, mutable).
//   - All `name` columns: required, free-text via coerceString.
//   - All `enabled` columns: optional, route through coerceBool
//     (D5-02 loose bool).
//   - Agents: skills column via coerceSkillsCell (D5-17 pipe-syntax).
//   - Channels: default_queue_code via coerceCode (optional FK target).
//   - Queues: channel_types via splitMulti + coerceChannelTypesCell.
//   - Adapters: config via coerceJSONBObject (free-form JSONB).
//
// Wave 3 row processors consume entityRegistry via header.go's
// dispatchRow helper (also in Wave 3). This wave only ships the
// registry shape + validator.
var entityRegistry = map[api.ImportEntityType]entityColumnRegistry{
	api.Agents: {
		{name: "code", required: true, coerce: coerceCode},
		{name: "external_id", required: false, coerce: coerceString},
		{name: "name", required: true, coerce: coerceString},
		{name: "email", required: true, coerce: coerceEmail},
		{name: "enabled", required: false, coerce: coerceBool},
		{name: "skills", required: false, coerce: coerceSkillsCell},
	},
	api.Skills: {
		{name: "code", required: true, coerce: coerceCode},
		{name: "external_id", required: false, coerce: coerceString},
		{name: "name", required: true, coerce: coerceString},
		{name: "skill_type", required: true, coerce: coerceString},
		{name: "description", required: false, coerce: coerceString},
		{name: "enabled", required: false, coerce: coerceBool},
	},
	api.Queues: {
		{name: "code", required: true, coerce: coerceCode},
		{name: "external_id", required: false, coerce: coerceString},
		{name: "name", required: true, coerce: coerceString},
		{name: "channel_types", required: true, coerce: coerceChannelTypesCell},
		{name: "priority", required: false, coerce: coerceInt},
		{name: "acw_sec", required: false, coerce: coerceInt},
		{name: "enabled", required: false, coerce: coerceBool},
	},
	api.Channels: {
		{name: "code", required: true, coerce: coerceCode},
		{name: "external_id", required: false, coerce: coerceString},
		{name: "name", required: true, coerce: coerceString},
		{name: "channel_type", required: true, coerce: coerceString},
		{name: "default_queue_code", required: false, coerce: coerceCode},
		{name: "enabled", required: false, coerce: coerceBool},
	},
	api.Adapters: {
		{name: "code", required: true, coerce: coerceCode},
		{name: "external_id", required: false, coerce: coerceString},
		{name: "name", required: true, coerce: coerceString},
		{name: "adapter_type", required: true, coerce: coerceString},
		{name: "config", required: false, coerce: coerceJSONBObject},
		{name: "enabled", required: false, coerce: coerceBool},
	},
	api.BreakReasons: {
		{name: "code", required: true, coerce: coerceCode},
		{name: "external_id", required: false, coerce: coerceString},
		{name: "name", required: true, coerce: coerceString},
		{name: "routable", required: false, coerce: coerceBool},
		{name: "display_order", required: false, coerce: coerceInt},
		{name: "enabled", required: false, coerce: coerceBool},
	},
}

// validateHeader implements D5-05 strict-batch policy. Returns two
// slices:
//
//   - missing: required column names that did NOT appear in `header`.
//   - unknown: column names in `header` that are NOT in entityRegistry.
//
// Both slices nil ⇒ header is valid. Either non-nil ⇒ batch-level
// 400 (chunk loop never runs).
//
// Comparison is case-sensitive (admin must spell column names exactly
// as documented). Duplicate columns in `header` are NOT detected here
// — csv.Reader's FieldsPerRecord=0 latches the first record's count
// but does not enforce uniqueness; if needed, Wave 3 can add a
// duplicate-column pass.
//
// An unknown entity type returns ([], header) (every column is
// unknown) — the handler should have validated entity before reaching
// here, but defensive behaviour avoids a nil-map panic.
func validateHeader(entity api.ImportEntityType, header []string) (missing, unknown []string) {
	registry, ok := entityRegistry[entity]
	if !ok {
		// Unknown entity — every column is unknown. Defensive only;
		// handler validates entity before calling validateHeader.
		return nil, append([]string(nil), header...)
	}

	known := make(map[string]bool, len(registry))
	required := make(map[string]bool)
	for _, spec := range registry {
		known[spec.name] = true
		if spec.required {
			required[spec.name] = true
		}
	}

	seen := make(map[string]bool, len(header))
	for _, h := range header {
		seen[h] = true
		if !known[h] {
			unknown = append(unknown, h)
		}
	}
	for _, spec := range registry {
		if spec.required && !seen[spec.name] {
			missing = append(missing, spec.name)
		}
	}
	return missing, unknown
}

// ---------------------------------------------------------------------------
// Coercion callbacks — bridges between coerce.go primitives and the
// entityRegistry's `(string) (any, error)` signature.
// ---------------------------------------------------------------------------

// coerceCode validates a `code` value through catalog.ValidateCodeFormat
// (D04_1-03 regex, exported in Plan 05-02). Returns the trimmed value
// on success; ErrInvalidCodeFormat on failure. Trim is necessary because
// CSV cells often have stray whitespace from spreadsheet pasting.
func coerceCode(raw string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	if !catalog.ValidateCodeFormat(trimmed) {
		return nil, ErrInvalidCodeFormat
	}
	return trimmed, nil
}

// coerceString is the pass-through. Returns the input verbatim (no
// trim, no lowercase) so free-text fields preserve admin-supplied
// casing. Phase 5 has no v0.1 string field that fails coercion — the
// per-entity validator handles length / regex if any.
func coerceString(raw string) (any, error) {
	return raw, nil
}

// coerceEmail is intentionally lenient at the coerce layer — the
// generated openapi_types.Email type does its own RFC validation when
// the typed value is constructed. We only trim whitespace.
func coerceEmail(raw string) (any, error) {
	return strings.TrimSpace(raw), nil
}

// coerceBool delegates to parseBool (D5-02).
func coerceBool(raw string) (any, error) {
	v, err := parseBool(raw)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// coerceInt delegates to parseInt (D5-07). Truncation is silently
// applied; emitting the slog.Debug forensic event is the chunk loop's
// responsibility (it has access to the package logger).
func coerceInt(raw string) (any, error) {
	v, _, err := parseInt(raw)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// SkillToken is the parsed shape of one `code:proficiency` pair in
// the CSV `skills` cell (D5-17). The Wave 3 row processor consumes
// []SkillToken when mapping agent_skills.
type SkillToken struct {
	SkillCode   string
	Proficiency int
}

// coerceSkillsCell parses D5-17 syntax: `code:prof|code:prof` (D5-04
// separator priority `|` > `;` > `,`). Returns []SkillToken on success.
// Failures (no colon, non-int proficiency, invalid code format) return
// the matching sentinel error. The empty cell returns an empty slice
// + nil error (admin chose to import an agent with no skills).
func coerceSkillsCell(raw string) (any, error) {
	parts := splitMulti(raw)
	if len(parts) == 0 {
		return []SkillToken{}, nil
	}
	out := make([]SkillToken, 0, len(parts))
	for _, p := range parts {
		colon := strings.IndexByte(p, ':')
		if colon < 0 {
			return nil, ErrInvalidSkillToken
		}
		code := strings.TrimSpace(p[:colon])
		profRaw := strings.TrimSpace(p[colon+1:])
		if !catalog.ValidateCodeFormat(code) {
			return nil, ErrInvalidCodeFormat
		}
		prof, perr := strconv.Atoi(profRaw)
		if perr != nil {
			return nil, ErrInvalidSkillToken
		}
		out = append(out, SkillToken{SkillCode: code, Proficiency: prof})
	}
	return out, nil
}

// coerceChannelTypesCell is the queue.channel_types splitter. Pipe-
// delimited list (D5-04). Each token is lowercased for D5-06 enum-
// match downstream; this function only splits + trims + lowercases.
// Per-entity validator (Wave 3) confirms each token is in the closed
// ChannelType enum set.
func coerceChannelTypesCell(raw string) (any, error) {
	parts := splitMulti(raw)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.ToLower(p))
	}
	return out, nil
}

// coerceJSONBObject is the adapter.config splitter. Free-form JSONB
// passed through verbatim (D-72 carry-forward — no schema). Empty
// string → nil (NULL column).
//
// Phase 5 fix M3: validate the JSON object shape AT the coerce step so
// a malformed config cell surfaces as `{field: "config", reason:
// "invalid_json"}` rather than as a generic `invalid_json_row` later
// in the row-processor's reMarshalAs round-trip. Pre-fix the coerce
// step accepted any non-empty string and the JSON parse happened in
// materialiseTypedRaw, which dropped the failure on the floor and let
// the row processor surface an empty-field generic error.
//
// We return the parsed map[string]interface{} on success so the
// downstream materialiseTypedRaw layer is a pure passthrough — no
// second JSON decode, no second failure mode.
func coerceJSONBObject(raw string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &m); err != nil {
		return nil, ErrInvalidJSON
	}
	return m, nil
}
