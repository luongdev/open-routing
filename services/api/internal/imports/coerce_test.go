// coerce_test.go — table-driven coverage for D5-01..D5-08 primitives.
//
// Tests live in `package imports` (not `_test`) so they can reach
// lowercase sentinel errors. Every TestParseX maps to one D5-NN
// decision per the verify gate in 05-04-PLAN.md.
package imports

import (
	"errors"
	"testing"
)

// TestParseBool walks the D5-02 loose-bool grammar plus a handful of
// rejection cases. ≥ 10 cases per the plan verify gate.
func TestParseBool(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    bool
		wantErr error
	}{
		{name: "lower_true", raw: "true", want: true},
		{name: "lower_false", raw: "false", want: false},
		{name: "upper_true", raw: "TRUE", want: true},
		{name: "upper_false", raw: "FALSE", want: false},
		{name: "one", raw: "1", want: true},
		{name: "zero", raw: "0", want: false},
		{name: "yes", raw: "yes", want: true},
		{name: "no", raw: "no", want: false},
		{name: "yes_with_whitespace", raw: " yes ", want: true},
		{name: "mixed_case_yes", raw: "Yes", want: true},
		{name: "unknown_value", raw: "maybe", wantErr: ErrInvalidBool},
		{name: "empty_string", raw: "", wantErr: ErrInvalidBool},
		{name: "stray_punctuation", raw: "true!", wantErr: ErrInvalidBool},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBool(tc.raw)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("parseBool(%q): want err %v, got %v", tc.raw, tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBool(%q): unexpected err %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("parseBool(%q): want %v, got %v", tc.raw, tc.want, got)
			}
		})
	}
}

// TestParseInt covers D5-07 truncation semantics + the rejection
// paths.
func TestParseInt(t *testing.T) {
	cases := []struct {
		name          string
		raw           string
		wantValue     int
		wantTruncated bool
		wantErr       error
	}{
		{name: "plain_int", raw: "7", wantValue: 7},
		{name: "whitespace_around_int", raw: " 7 ", wantValue: 7},
		{name: "exact_float", raw: "7.0", wantValue: 7},
		{name: "fractional_truncates", raw: "7.5", wantValue: 7, wantTruncated: true},
		{name: "negative_int", raw: "-3", wantValue: -3},
		{name: "negative_float_truncates_toward_zero", raw: "-3.7", wantValue: -3, wantTruncated: true},
		{name: "high_precision_fractional", raw: "3.999", wantValue: 3, wantTruncated: true},
		{name: "non_numeric", raw: "seven", wantErr: ErrInvalidInt},
		{name: "empty", raw: "", wantErr: ErrInvalidInt},
		{name: "only_whitespace", raw: "   ", wantErr: ErrInvalidInt},
		{name: "stray_letters_after_digit", raw: "7px", wantErr: ErrInvalidInt},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotValue, gotTrunc, err := parseInt(tc.raw)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("parseInt(%q): want err %v, got %v", tc.raw, tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseInt(%q): unexpected err %v", tc.raw, err)
			}
			if gotValue != tc.wantValue {
				t.Fatalf("parseInt(%q): want value %d, got %d", tc.raw, tc.wantValue, gotValue)
			}
			if gotTrunc != tc.wantTruncated {
				t.Fatalf("parseInt(%q): want truncated %v, got %v", tc.raw, tc.wantTruncated, gotTrunc)
			}
		})
	}
}

// TestSplitMulti covers D5-04 separator priority + edge cases.
func TestSplitMulti(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "pipe_three_parts", raw: "a|b|c", want: []string{"a", "b", "c"}},
		{name: "semicolon_two_parts", raw: "a;b", want: []string{"a", "b"}},
		{name: "comma_two_parts", raw: "a,b", want: []string{"a", "b"}},
		{name: "pipe_priority_over_semicolon", raw: "a|b;c", want: []string{"a", "b;c"}},
		{name: "semicolon_priority_over_comma", raw: "a;b,c", want: []string{"a", "b,c"}},
		{name: "empty_string", raw: "", want: nil},
		{name: "single_token", raw: "single", want: []string{"single"}},
		{name: "double_separator_drops_empty", raw: "a||b", want: []string{"a", "b"}},
		{name: "whitespace_pieces_trimmed", raw: "a | b | c", want: []string{"a", "b", "c"}},
		{name: "leading_trailing_separator_dropped", raw: "|a|b|", want: []string{"a", "b"}},
		{name: "whitespace_only_input", raw: "   ", want: nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := splitMulti(tc.raw)
			if !equalSlice(got, tc.want) {
				t.Fatalf("splitMulti(%q): want %v, got %v", tc.raw, tc.want, got)
			}
		})
	}
}

// TestValidateEnum exercises the D5-06 lowercase enum gate.
func TestValidateEnum(t *testing.T) {
	channelKinds := []string{"voice", "chat", "email"}
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{name: "lower_voice", raw: "voice", want: "voice"},
		{name: "title_case_voice", raw: "Voice", want: "voice"},
		{name: "upper_voice", raw: "VOICE", want: "voice"},
		{name: "whitespace_around_voice", raw: "  voice  ", want: "voice"},
		{name: "unknown_value", raw: "unknown", wantErr: ErrInvalidEnum},
		{name: "empty_string", raw: "", wantErr: ErrInvalidEnum},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateEnum(tc.raw, channelKinds)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("validateEnum(%q): want err %v, got %v", tc.raw, tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateEnum(%q): unexpected err %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("validateEnum(%q): want %q, got %q", tc.raw, tc.want, got)
			}
		})
	}
}

// TestValidateUTF8 covers D5-08 byte-level charset gate.
func TestValidateUTF8(t *testing.T) {
	t.Run("ascii_passes", func(t *testing.T) {
		if err := validateUTF8([]byte("code,name\nA,Alice\n")); err != nil {
			t.Fatalf("validateUTF8: unexpected err %v", err)
		}
	})
	t.Run("multibyte_passes", func(t *testing.T) {
		// "tư duy humanable" — uses Vietnamese tonal characters as a
		// real-world multi-byte UTF-8 fixture.
		if err := validateUTF8([]byte("tư duy humanable")); err != nil {
			t.Fatalf("validateUTF8: unexpected err %v", err)
		}
	})
	t.Run("invalid_byte_sequence_rejected", func(t *testing.T) {
		// 0xFF is never a valid UTF-8 starter byte (RFC 3629).
		if err := validateUTF8([]byte{0x41, 0xFF, 0x42}); !errors.Is(err, ErrCSVNotUTF8) {
			t.Fatalf("validateUTF8: want ErrCSVNotUTF8, got %v", err)
		}
	})
	t.Run("empty_passes", func(t *testing.T) {
		if err := validateUTF8(nil); err != nil {
			t.Fatalf("validateUTF8(nil): unexpected err %v", err)
		}
	})
}

// equalSlice compares two []string for content equality. Used because
// the stdlib slices package's Equal handles nil-vs-empty differently
// than we want here (we treat both as the same outcome).
func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
