// codecheck_test.go — RED-phase tests for validateCodeFormat /
// validateImmutableCode (D04_1-03, D04_1-19, D04_1-20).
//
// Pure-function tests; no DB, no httptest, no testcontainer; runs under
// `go test -short`. Mirrors cursor_test.go in shape (stdlib testing +
// testify/require) per the PATTERNS.md analog (services/api/internal/
// catalog/codecheck_test.go locked test cases).
//
// Phase 04.1 Plan 03 Task 1 — TDD style. The implementation in
// codecheck.go lands in a separate GREEN commit immediately after.
package catalog

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateCodeFormat_Valid locks D04_1-03 examples (regex
// `^[a-z][a-z0-9_]{0,63}$`). Boundary cases: 1-char minimum + 64-char
// maximum + every entity prefix.
func TestValidateCodeFormat_Valid(t *testing.T) {
	cases := []string{
		"emp_001", "skill_voice_tier1", "queue_billing",
		"channel_voice_primary", "adapter_freeswitch_dc1", "break_lunch",
		"a",                          // 1-char minimum
		"a" + strings.Repeat("0", 63), // 64-char maximum
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			require.True(t, validateCodeFormat(c), "valid code %q must pass", c)
		})
	}
}

// TestValidateCodeFormat_Invalid locks D04_1-03 rejection set + D04_1-05
// (empty is invalid). The `over_64_chars` fixture uses a printable form
// (PATTERNS.md alternate) so subtest output stays grep-able under failure.
func TestValidateCodeFormat_Invalid(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"upper_start", "UPPER"},
		{"mixed_case", "Emp_001"},
		{"digit_start", "1emp"},
		{"underscore_start", "_emp"},
		{"hyphen", "emp-001"},
		{"space", "emp 001"},
		{"leading_space", " emp_001"},
		{"trailing_space", "emp_001 "},
		{"over_64_chars", "a" + strings.Repeat("0", 64)}, // 65 chars total
		{"unicode", "empléyé"},
		{"period", "emp.001"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.False(t, validateCodeFormat(c.in), "invalid code %q must fail", c.in)
		})
	}
}

// TestValidateImmutableCode locks the D04_1-20 PATCH semantics. Empty
// requested means "field absent from PATCH body" → no-op succeed. Case-
// different is still a violation (no auto-lowercase per D04_1-Deferred).
func TestValidateImmutableCode(t *testing.T) {
	require.True(t, validateImmutableCode("emp_001", ""),
		"empty requested (absent in PATCH) = no-op")
	require.True(t, validateImmutableCode("emp_001", "emp_001"),
		"same value = no-op")
	require.False(t, validateImmutableCode("emp_001", "emp_002"),
		"different value = immutability violation")
	require.False(t, validateImmutableCode("emp_001", "EMP_001"),
		"case-different is still different (no auto-lowercase)")
}
