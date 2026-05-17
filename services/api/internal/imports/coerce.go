// coerce.go — typed preprocessing pipeline for D5-01..D5-08.
//
// Philosophy (RESEARCH §A1, user's "tư duy humanable"): every CSV cell
// passes through `trim → lower → split → parse` BEFORE validation, so
// per-entity validators only ever see well-shaped data. The pipeline is
// dispatched by the sqlc-generated Go type of the target column
// (resolved upstream in header.go's entityRegistry).
//
// Sentinel errors carry the wire-level reason codes that
// BulkImportFailedRow.reason emits. Callers wrap them with row/field
// metadata in chunk.go (Plan 05-05).
package imports

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Sentinel errors — one per D5-NN failure mode. The reason strings on
// the right are what eventually reach BulkImportFailedRow.reason after
// chunk.go wraps them with row/field metadata (Plan 05-05).
//
// Declared as standalone `var` statements (not grouped under
// `var ( ... )`) so the plan's grep gate `^var Err…` finds each one
// independently — and so future authors don't accidentally hide a new
// sentinel inside the group.

// ErrInvalidBool — D5-02. Cell did not match the loose bool grammar
// (true|false|TRUE|FALSE|1|0|yes|no after trim+lower).
var ErrInvalidBool = errors.New("invalid_bool")

// ErrMissingRequired — D5-03. Required-field column was empty.
// Triggered by the per-entity validator, not the coercer itself; the
// coerce.go pipeline is type-only.
var ErrMissingRequired = errors.New("missing_required")

// ErrInvalidEnum — D5-06. Cell value did not match the closed enum
// set after trim+lower.
var ErrInvalidEnum = errors.New("invalid_enum")

// ErrInvalidInt — D5-07. Cell could not be coerced to int via
// strconv.ParseFloat → math.Trunc.
var ErrInvalidInt = errors.New("invalid_int")

// ErrCSVNotUTF8 — D5-08. Byte sequence was not valid UTF-8 after
// BOM stripping.
var ErrCSVNotUTF8 = errors.New("csv_not_utf8")

// ErrInvalidSkillToken — D5-17. Skills cell token did not parse as
// `code:proficiency`.
var ErrInvalidSkillToken = errors.New("invalid_skill_token")

// ErrInvalidCodeFormat — D5-15 / D04_1-03. A `code` value (top-level
// or a skill_code token) failed the `^[a-z][a-z0-9_]{0,63}$` regex.
// catalog.ValidateCodeFormat is the single source of truth.
var ErrInvalidCodeFormat = errors.New("invalid_code_format")

// trimLower applies the D5-01 prefix transform: strip surrounding
// whitespace, then lowercase. Used by parseBool, validateEnum, and the
// skill-token parser. Not used for free-text fields (name, description)
// — those preserve admin-supplied casing.
func trimLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// parseBool is D5-02 loose bool coercion. Accepts the eight values
// admins commonly type when exporting from Excel / Google Sheets / hand:
// `true|false|TRUE|FALSE|1|0|yes|no` (case-insensitive after trimLower).
// Empty or unrecognised values → ErrInvalidBool.
func parseBool(raw string) (bool, error) {
	switch trimLower(raw) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	}
	return false, ErrInvalidBool
}

// parseInt is D5-07 tolerant int coercion via the float-truncate
// pipeline: strings.TrimSpace → strconv.ParseFloat → math.Trunc → int.
// Excel silently rewrites `7` to `7.0` when re-saving an .xlsx as CSV;
// strict strconv.Atoi would reject otherwise-valid columns.
//
// Returns (value, truncated, error). `truncated == true` when a non-zero
// fractional part was discarded (`7.5` → (7, true, nil)). The caller is
// responsible for emitting slog.Debug("import: int_truncation", ...) for
// forensics — this pure function does not have access to the package
// logger.
//
// Empty string → ErrInvalidInt. Non-numeric → ErrInvalidInt.
func parseInt(raw string) (int, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false, ErrInvalidInt
	}
	f, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false, ErrInvalidInt
	}
	truncated := math.Trunc(f)
	return int(truncated), truncated != f, nil
}

// splitMulti is D5-04 multi-value cell splitter. Priority: `|` > `;` > `,`.
// CSV's field-level `,` is already unescaped by csv.Reader before we see
// the cell value, so sub-splitting by `,` only happens inside the
// unescaped cell. Whitespace around each piece is trimmed and empty
// pieces are dropped.
//
// Returns []string of trimmed non-empty pieces. Empty input returns
// nil (not []string{""}).
func splitMulti(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	for _, sep := range []string{"|", ";", ","} {
		if strings.Contains(s, sep) {
			return splitTrim(s, sep)
		}
	}
	// Single token — no separator present at all.
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return []string{t}
}

// splitTrim is the inner helper used by splitMulti: split by sep, trim
// each piece, drop empties. Extracted so the priority logic in
// splitMulti stays a flat for-loop without nested string transforms.
func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// validateEnum is D5-06 closed-enum coercion. After trimLower the input
// must appear in `allowed`; otherwise ErrInvalidEnum. Returns the
// canonical lowercased value on success so downstream code never has to
// re-normalise.
func validateEnum(raw string, allowed []string) (string, error) {
	v := trimLower(raw)
	for _, a := range allowed {
		if v == a {
			return v, nil
		}
	}
	return "", ErrInvalidEnum
}

// validateUTF8 is D5-08 charset gate. After BOM stripping (parser_csv.go
// stripBOM), the remaining bytes must be valid UTF-8 — utf8.Valid is the
// stdlib check. Failure → ErrCSVNotUTF8. PITFALLS 5.1 lists charset
// auto-detect as the #1 silent-corruption source; v0.1 has no
// iso-8859-1 fallback.
func validateUTF8(b []byte) error {
	if !utf8.Valid(b) {
		return ErrCSVNotUTF8
	}
	return nil
}
