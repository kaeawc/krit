package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

// TestAppendFindingJSON_MatchesJSONMarshal pins the byte-identical
// contract: appendFindingJSON(dst, f) must produce exactly the same
// bytes json.Marshal(f) would produce, for every realistic finding
// shape we emit. Any drift here is a CLI / IDE / downstream-consumer
// schema break, so the comparison is byte-exact rather than fuzzy.
func TestAppendFindingJSON_MatchesJSONMarshal(t *testing.T) {
	startByte := 42
	endByte := 100
	zeroStart := 0
	zeroEnd := 0
	tests := []struct {
		name    string
		finding JSONFinding
	}{
		{
			name: "minimal-warning",
			finding: JSONFinding{
				File: "Foo.kt", Line: 1, Column: 1,
				RuleSet: "style", Rule: "MaxLineLength",
				Severity: "warning", Message: "Line too long.",
				Confidence: 0.75,
			},
		},
		{
			name: "with-byte-offsets",
			finding: JSONFinding{
				File: "src/main/kotlin/A.kt", Line: 42, Column: 7,
				StartByte: &startByte, EndByte: &endByte,
				RuleSet: "potential-bugs", Rule: "UnsafeCall",
				Severity: "warning", Message: "Unsafe.",
				Confidence: 0.85,
			},
		},
		{
			name: "zero-byte-offsets",
			finding: JSONFinding{
				File: "A.kt", Line: 1, Column: 1,
				StartByte: &zeroStart, EndByte: &zeroEnd,
				RuleSet: "style", Rule: "X", Severity: "warning",
				Message: "msg", Confidence: 0.75,
			},
		},
		{
			name: "fixable-with-level",
			finding: JSONFinding{
				File: "B.kt", Line: 3, Column: 5,
				RuleSet: "style", Rule: "TrailingComma",
				Severity: "warning", Message: "Missing.",
				Fixable: true, FixLevel: "cosmetic",
				Confidence: 0.9, Effort: "trivial",
			},
		},
		{
			name: "confidence-zero-omitted",
			finding: JSONFinding{
				File: "C.kt", Line: 1, Column: 1,
				RuleSet: "complexity", Rule: "LongMethod",
				Severity: "warning", Message: "Too long.",
			},
		},
		{
			name: "error-severity",
			finding: JSONFinding{
				File: "D.kt", Line: 9, Column: 4,
				RuleSet: "security", Rule: "InsecureTrustManager",
				Severity: "error", Message: "Unsafe TM.",
				Confidence: 0.95,
			},
		},
		{
			name: "all-fields-set",
			finding: JSONFinding{
				File: "E.kt", Line: 7, Column: 3,
				StartByte: &startByte, EndByte: &endByte,
				RuleSet: "potential-bugs", Rule: "X",
				Severity: "warning", Message: "msg",
				Fixable: true, FixLevel: "semantic",
				Confidence: 0.85, Effort: "local",
			},
		},
		{
			name: "string-with-quote-escape",
			finding: JSONFinding{
				File: `path/with "quote".kt`, Line: 1, Column: 1,
				RuleSet: "style", Rule: "X", Severity: "warning",
				Message:    `He said "hello" and \\ escaped.`,
				Confidence: 0.75,
			},
		},
		{
			name: "string-with-newline-and-tab",
			finding: JSONFinding{
				File: "F.kt", Line: 1, Column: 1,
				RuleSet: "style", Rule: "X", Severity: "warning",
				Message:    "line1\nline2\twith tab",
				Confidence: 0.75,
			},
		},
		{
			name: "string-with-control-bytes",
			finding: JSONFinding{
				File: "G.kt", Line: 1, Column: 1,
				RuleSet: "style", Rule: "X", Severity: "warning",
				Message:    string([]byte{0x01, 0x02, 0x1f}),
				Confidence: 0.75,
			},
		},
		{
			name: "string-with-utf8-multibyte",
			finding: JSONFinding{
				File: "héllo/wörld/π.kt", Line: 1, Column: 1,
				RuleSet: "style", Rule: "X", Severity: "warning",
				Message:    "中文 message 🎉",
				Confidence: 0.75,
			},
		},
		{
			name: "confidence-non-round",
			finding: JSONFinding{
				File: "F.kt", Line: 1, Column: 1,
				RuleSet: "style", Rule: "X", Severity: "warning",
				Message: "x", Confidence: 0.6789012345,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := json.Marshal(tt.finding)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			got := appendFindingJSON(nil, tt.finding)
			if !bytes.Equal(got, want) {
				t.Errorf("appendFindingJSON byte mismatch:\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// TestAppendFindingJSON_BufferReuse confirms callers can re-use the
// destination buffer across many appends without losing earlier
// content — the canonical bulk-encode pattern.
func TestAppendFindingJSON_BufferReuse(t *testing.T) {
	f1 := JSONFinding{File: "A.kt", Line: 1, Column: 1, RuleSet: "s", Rule: "r", Severity: "warning", Message: "m"}
	f2 := JSONFinding{File: "B.kt", Line: 2, Column: 2, RuleSet: "s", Rule: "r", Severity: "warning", Message: "n"}
	buf := appendFindingJSON(nil, f1)
	buf = append(buf, ',')
	buf = appendFindingJSON(buf, f2)
	want := fmt.Sprintf("%s,%s",
		mustMarshal(t, f1),
		mustMarshal(t, f2))
	if string(buf) != want {
		t.Errorf("buffer-reuse drift:\n got: %s\nwant: %s", buf, want)
	}
}

func mustMarshal(t *testing.T, f JSONFinding) string {
	t.Helper()
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}

// TestAppendJSONFloat_MatchesJSONMarshal locks in byte-identical
// equivalence between our hand-rolled `appendJSONFloat` and
// `json.Marshal` across the value ranges the encoder switches between
// 'f' and 'e' formats and the exponent-padding boundary where the
// stdlib trims a leading zero from negative single-digit exponents.
func TestAppendJSONFloat_MatchesJSONMarshal(t *testing.T) {
	values := []float64{
		0,
		1,
		-1,
		0.5,
		-0.5,
		0.75,
		0.85,
		0.95,
		1e-5,
		1e-6,  // boundary: just below — uses 'e' on the stdlib side too.
		1e-7,  // strconv emits 1e-07; stdlib trims to 1e-7.
		1e-8,  // 1e-08 → 1e-8.
		1e-9,  // 1e-09 → 1e-9.
		1e-10, // two-digit exponent stays 1e-10.
		-1e-7, // negative number, negative exponent: same trim path.
		1e20,
		1e21, // boundary: at threshold — uses 'e'.
		1e22,
		1e100,
		-1e100,
		1234567.89,
		1.0 / 3.0,
	}
	for _, v := range values {
		t.Run(fmt.Sprintf("%g", v), func(t *testing.T) {
			want, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("json.Marshal(%v): %v", v, err)
			}
			got := appendJSONFloat(nil, v)
			if !bytes.Equal(got, want) {
				t.Fatalf("appendJSONFloat(%v) = %q, json.Marshal = %q", v, got, want)
			}
		})
	}
}
