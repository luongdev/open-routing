// codecheck.go — Layer 1 + Layer 2 validation helpers for the `code`
// field on every catalog entity (D04_1-03, D04_1-19, D04_1-20).
//
// Why this file exists:
//   - oapi-codegen v2 does NOT auto-enforce OpenAPI `pattern` regexes
//     in the strict-server runtime (RESEARCH §Pitfall 1). The handler
//     is the single source of truth for code-format validation.
//   - Immutability is a business rule (D04_1-02) that cannot be
//     expressed in OpenAPI; it must compare a request field to the
//     stored DB value.
//   - Both checks repeat across 6 entity handlers. Centralizing them
//     keeps the per-entity handler bodies readable and the regex
//     compiles once at package init.
//
// Conceptual analog: services/api/internal/catalog/agent_skills.go
// `validateProficiencyRange` (D-74 carry-forward) — same idiom but
// extracted to a shared file because 6 entities call it.
package catalog

import "regexp"

// codeFormat is the canonical regex from D04_1-03: lowercase, starts
// with a letter, letters+digits+underscore only, 1-64 chars. Compiled
// once at package init (regexp.MustCompile panics on invalid pattern,
// surfacing config errors at startup rather than at first request).
//
// RE2 → no ReDoS risk on the bounded `{0,63}` quantifier.
var codeFormat = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// ValidateCodeFormat returns true iff s matches D04_1-03 strict format.
// Empty string returns false (D04_1-05 — empty code is invalid).
// Handlers map false → 400 invalid_body / reason="invalid_code_format".
//
// Exported in Phase 5 Wave 0 so internal/imports/ can reuse this regex
// without duplicating the D04_1-03 source-of-truth. Per-row CSV/JSON
// import validators consume it for the `code` field plus skill_code
// tokens in nested agent skills (D5-16, D5-17).
func ValidateCodeFormat(s string) bool {
	return codeFormat.MatchString(s)
}

// validateImmutableCode is the PATCH-only immutability gate (D04_1-20).
// Returns true iff requested is empty (the field was absent from the
// PATCH body) or equals stored. Handler maps false → 422 immutable_field
// / field="code". Empty `requested` means the PATCH did not touch the
// field; we accept it as no-op.
//
// Note: handlers MUST call ValidateCodeFormat(requested) BEFORE this so
// a malformed PATCH like `"code": "UPPER"` gets 400 invalid_body, not 422.
func validateImmutableCode(stored, requested string) bool {
	return requested == "" || requested == stored
}
