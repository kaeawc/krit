package rules

import (
	"regexp"
	"strings"
	"testing"
)

var testAddKeyValueRe = regexp.MustCompile(`\baddKeyValue\s*\(\s*"([^"]+)"`)

// TestStripCommentsAndRawStrings_PreservesCodeAndMasksNonCode locks in the
// invariants used by line-pass rules (e.g. StructuredLogKeyMixedCase):
// real call expressions remain intact, while pattern occurrences inside
// comments and raw strings are scrubbed so they cannot drive a false
// positive.
func TestStripCommentsAndRawStrings_PreservesCodeAndMasksNonCode(t *testing.T) {
	cases := []struct {
		name     string
		lines    []string
		wantKeys map[int]string // line index → captured key after scrub
	}{
		{
			name:     "real call preserved",
			lines:    []string{`addKeyValue("user_id", id)`},
			wantKeys: map[int]string{0: "user_id"},
		},
		{
			name: "line comment ignored",
			lines: []string{
				`x() // addKeyValue("requestId", id)`,
				`addKeyValue("user_id", id)`,
			},
			wantKeys: map[int]string{1: "user_id"},
		},
		{
			name: "block comment ignored single line",
			lines: []string{
				`x() /* addKeyValue("requestId", id) */ y()`,
				`addKeyValue("user_id", id)`,
			},
			wantKeys: map[int]string{1: "user_id"},
		},
		{
			name: "block comment ignored across lines",
			lines: []string{
				`/* docs:`,
				` * addKeyValue("requestId", id)`,
				` */`,
				`addKeyValue("user_id", id)`,
			},
			wantKeys: map[int]string{3: "user_id"},
		},
		{
			name: "raw string ignored single line",
			lines: []string{
				`val s = """addKeyValue("requestId", id)"""`,
				`addKeyValue("user_id", id)`,
			},
			wantKeys: map[int]string{1: "user_id"},
		},
		{
			name: "raw string ignored across lines",
			lines: []string{
				`val s = """`,
				`  addKeyValue("requestId", id)`,
				`"""`,
				`addKeyValue("user_id", id)`,
			},
			wantKeys: map[int]string{3: "user_id"},
		},
		{
			// Escaped inner quotes break the regex's `"([^"]+)"` boundary,
			// so the regular string body cannot fabricate a match even
			// though we deliberately do NOT scrub regular-string bytes.
			name: "regular string with escaped quotes does not false-positive",
			lines: []string{
				`err("addKeyValue(\"requestId\", v)")`,
				`addKeyValue("user_id", id)`,
			},
			wantKeys: map[int]string{1: "user_id"},
		},
		{
			name: "triple-quote inside line comment does not enter raw mode",
			lines: []string{
				`// snippet: """raw with addKeyValue("requestId", id)"""`,
				`addKeyValue("user_id", id)`,
			},
			wantKeys: map[int]string{1: "user_id"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var st lineScanState
			gotKeys := map[int]string{}
			for i, line := range tc.lines {
				out := stripCommentsAndRawStrings(line, &st)
				if len(out) != len(line) {
					t.Fatalf("line %d: scrubbed length %d != input length %d (%q vs %q)", i, len(out), len(line), out, line)
				}
				if m := testAddKeyValueRe.FindStringSubmatch(out); m != nil {
					gotKeys[i] = m[1]
				}
			}
			for idx, want := range tc.wantKeys {
				if gotKeys[idx] != want {
					t.Fatalf("line %d: got captured key %q, want %q (scrubbed line: %q)", idx, gotKeys[idx], want, tc.lines[idx])
				}
			}
			for idx, got := range gotKeys {
				if _, ok := tc.wantKeys[idx]; !ok {
					t.Fatalf("line %d: unexpected captured key %q from %q", idx, got, tc.lines[idx])
				}
			}
		})
	}
}

// TestScanLineStateMatchesStrip ensures the original scanLineState advance
// path used by complexity rules stays in sync with the shared lexer
// body across both strip variants. Drift would silently corrupt
// LongMethod and friends.
func TestScanLineStateMatchesStrip(t *testing.T) {
	lines := []string{
		`fun f() {`,
		`    val s = """`,
		`      multi-line raw`,
		`    """`,
		`    /* block`,
		`       comment */`,
		`    val r = "regular \"quoted\" string"`,
		`}`,
	}
	var rawStripState lineScanState
	var keepStripState lineScanState
	var scanState lineScanState
	for i, line := range lines {
		_ = stripCommentsAndRawStrings(line, &rawStripState)
		_ = stripCommentsKeepStrings(line, &keepStripState)
		scanLineState(line, &scanState)
		if rawStripState != scanState {
			t.Fatalf("line %d (%q): state drift between stripCommentsAndRawStrings and scanLineState: strip=%+v scan=%+v", i, line, rawStripState, scanState)
		}
		if keepStripState != scanState {
			t.Fatalf("line %d (%q): state drift between stripCommentsKeepStrings and scanLineState: strip=%+v scan=%+v", i, line, keepStripState, scanState)
		}
	}
}

// TestStripCommentsKeepStrings_MasksOnlyComments locks in that the
// keep-strings variant scrubs line- and block-comment bytes while
// leaving regular and raw string bytes verbatim, which is what
// PackagedPrivateKey and similar rules require to detect markers
// pasted into PEM-style string literals.
func TestStripCommentsKeepStrings_MasksOnlyComments(t *testing.T) {
	cases := []struct {
		name      string
		lines     []string
		marker    string // must appear in scrubbed output where expected
		expectOn  []int  // 0-based lines that must still contain marker
		expectOff []int  // 0-based lines that must NOT contain marker
	}{
		{
			name: "marker in line comment is masked",
			lines: []string{
				`// header: -----BEGIN RSA PRIVATE KEY-----`,
				`val key = "-----BEGIN RSA PRIVATE KEY-----..."`,
			},
			marker:    "BEGIN RSA PRIVATE KEY",
			expectOn:  []int{1},
			expectOff: []int{0},
		},
		{
			name: "marker in block comment is masked",
			lines: []string{
				`/* -----BEGIN PRIVATE KEY----- example */`,
				`val key = "-----BEGIN PRIVATE KEY-----..."`,
			},
			marker:    "BEGIN PRIVATE KEY",
			expectOn:  []int{1},
			expectOff: []int{0},
		},
		{
			name: "marker in multi-line block comment is masked across lines",
			lines: []string{
				`/*`,
				` * -----BEGIN EC PRIVATE KEY-----`,
				` */`,
				`val key = "-----BEGIN EC PRIVATE KEY-----..."`,
			},
			marker:    "BEGIN EC PRIVATE KEY",
			expectOn:  []int{3},
			expectOff: []int{1},
		},
		{
			name: "marker in raw string is preserved",
			lines: []string{
				`val key = """`,
				`-----BEGIN RSA PRIVATE KEY-----`,
				`"""`,
			},
			marker:   "BEGIN RSA PRIVATE KEY",
			expectOn: []int{1},
		},
		{
			name: "marker in regular string is preserved",
			lines: []string{
				`val key = "-----BEGIN RSA PRIVATE KEY-----..."`,
			},
			marker:   "BEGIN RSA PRIVATE KEY",
			expectOn: []int{0},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var st lineScanState
			scrubbed := make([]string, len(tc.lines))
			for i, line := range tc.lines {
				out := stripCommentsKeepStrings(line, &st)
				if len(out) != len(line) {
					t.Fatalf("line %d: scrubbed length %d != input length %d", i, len(out), len(line))
				}
				scrubbed[i] = out
			}
			for _, idx := range tc.expectOn {
				if !strings.Contains(scrubbed[idx], tc.marker) {
					t.Fatalf("line %d: expected marker %q to remain in scrubbed %q", idx, tc.marker, scrubbed[idx])
				}
			}
			for _, idx := range tc.expectOff {
				if strings.Contains(scrubbed[idx], tc.marker) {
					t.Fatalf("line %d: expected marker %q to be masked, but it survived in %q", idx, tc.marker, scrubbed[idx])
				}
			}
		})
	}
}
