package rules

import (
	"strings"
	"testing"
)

func TestStripKotlinComments_StripsLineComment(t *testing.T) {
	in := "val x = 1 // trailing\nval y = 2"
	out := stripKotlinComments(in)
	if strings.Contains(out, "trailing") {
		t.Fatalf("line comment not stripped: %q", out)
	}
	if !strings.Contains(out, "val y = 2") {
		t.Fatalf("code after line comment not preserved: %q", out)
	}
}

func TestStripKotlinComments_StripsBlockComment(t *testing.T) {
	in := "val x = /* drop me */ 1\nval y = 2"
	out := stripKotlinComments(in)
	if strings.Contains(out, "drop me") {
		t.Fatalf("block comment not stripped: %q", out)
	}
	if !strings.Contains(out, "val y = 2") {
		t.Fatalf("code after block comment not preserved: %q", out)
	}
}

func TestStripKotlinComments_PreservesRegularStringContent(t *testing.T) {
	in := `val s = "hello // world"`
	out := stripKotlinComments(in)
	if !strings.Contains(out, "hello // world") {
		t.Fatalf("regular string content was stripped: %q", out)
	}
}

func TestStripKotlinComments_TripleQuotedRawStringWithEmbeddedQuote(t *testing.T) {
	// Inside `"""..."""` the embedded `"hi"` is part of the raw-string
	// content. The old state machine toggled out of "string" state on
	// the lone `"`, leaving `hi` mis-classified as code.
	in := `val s = """he said "hi" then it"""`
	out := stripKotlinComments(in)
	if out != in {
		t.Fatalf("raw-string boundary corrupted:\n input: %q\noutput: %q", in, out)
	}
}

func TestStripKotlinComments_TripleQuotedRawStringWithEmbeddedLineComment(t *testing.T) {
	// `//` inside a raw string must NOT be treated as a Kotlin line
	// comment. The old state machine could exit string mode on a lone
	// embedded `"` and then strip the following `//...` as a comment.
	in := "val s = \"\"\"hi\"//keep me\nrest\"\"\""
	out := stripKotlinComments(in)
	if !strings.Contains(out, "//keep me") {
		t.Fatalf("inner // was mis-stripped as comment: %q", out)
	}
	if !strings.Contains(out, "rest") {
		t.Fatalf("raw-string body after embedded // was dropped: %q", out)
	}
}

func TestStripKotlinComments_TripleQuotedRawStringWithEmbeddedBlockCommentMarker(t *testing.T) {
	// `/* ... */` characters inside a raw string must not be treated as
	// a Kotlin block comment.
	in := "val s = \"\"\"hi\"/* keep me */tail\"\"\""
	out := stripKotlinComments(in)
	if !strings.Contains(out, "/* keep me */") {
		t.Fatalf("inner /* ... */ was mis-stripped as block comment: %q", out)
	}
	if !strings.Contains(out, "tail") {
		t.Fatalf("raw-string body after embedded block-comment-like text was dropped: %q", out)
	}
}

func TestStripKotlinComments_TripleQuotedRawStringSpansMultipleLines(t *testing.T) {
	in := "val s = \"\"\"\n    line1\n    line2\n\"\"\"\nval n = 1"
	out := stripKotlinComments(in)
	if !strings.Contains(out, "line1") || !strings.Contains(out, "line2") {
		t.Fatalf("multi-line raw-string body dropped: %q", out)
	}
	if !strings.Contains(out, "val n = 1") {
		t.Fatalf("code after raw string dropped: %q", out)
	}
}

func TestStripKotlinComments_UnterminatedBlockCommentPreservesNewlines(t *testing.T) {
	// An unterminated block comment at EOF must still flush trailing
	// newlines so line counts downstream remain stable.
	in := "val x = 1 /* runaway\nstill in comment\n"
	out := stripKotlinComments(in)
	if got, want := strings.Count(out, "\n"), strings.Count(in, "\n"); got != want {
		t.Fatalf("newline count drifted on unterminated block comment: want %d got %d (%q)", want, got, out)
	}
	if strings.Contains(out, "runaway") {
		t.Fatalf("block comment body leaked: %q", out)
	}
}

func TestStripKotlinComments_CodeAfterRawStringTracksOutsideStringState(t *testing.T) {
	// After `"""..."""` closes, the next `// foo` is a real line comment
	// and must be stripped.
	in := "val s = \"\"\"body\"\"\" // trailing\nval n = 1"
	out := stripKotlinComments(in)
	if strings.Contains(out, "trailing") {
		t.Fatalf("trailing line comment after raw string was not stripped: %q", out)
	}
	if !strings.Contains(out, "val n = 1") {
		t.Fatalf("code after raw string + line comment dropped: %q", out)
	}
}
