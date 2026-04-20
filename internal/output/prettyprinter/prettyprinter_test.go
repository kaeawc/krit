package prettyprinter

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	got := RenderString(80, Text("hello"))
	if got != "hello" {
		t.Fatalf("Text: want %q, got %q", "hello", got)
	}
}

func TestConcatSkipsNil(t *testing.T) {
	d := Concat(Text("a"), Nil(), Text("b"), nil, Text("c"))
	got := RenderString(80, d)
	if got != "abc" {
		t.Fatalf("Concat: want %q, got %q", "abc", got)
	}
}

func TestEmptyText(t *testing.T) {
	d := Concat(Text(""), Text("x"), Text(""))
	if got := RenderString(80, d); got != "x" {
		t.Fatalf("empty Text: want %q, got %q", "x", got)
	}
}

func TestHardLineAlwaysBreaks(t *testing.T) {
	d := Group(Concat(Text("a"), HardLine(), Text("b")))
	got := RenderString(80, d)
	if got != "a\nb" {
		t.Fatalf("hard line: want %q, got %q", "a\nb", got)
	}
}

func TestSoftLineFlatFits(t *testing.T) {
	d := Group(Concat(Text("a"), SoftLine(), Text("b")))
	got := RenderString(80, d)
	if got != "a b" {
		t.Fatalf("soft line flat: want %q, got %q", "a b", got)
	}
}

func TestSoftLineBreaksWhenWide(t *testing.T) {
	d := Group(Concat(Text("aaaa"), SoftLine(), Text("bbbb")))
	// Width 5 can't fit "aaaa bbbb" (9 cols), so group breaks.
	got := RenderString(5, d)
	if got != "aaaa\nbbbb" {
		t.Fatalf("soft line break: want %q, got %q", "aaaa\nbbbb", got)
	}
}

func TestNestAppliesAfterBreak(t *testing.T) {
	d := Group(Nest(4, Concat(Text("head"), SoftLine(), Text("body"))))
	got := RenderString(5, d)
	want := "head\n    body"
	if got != want {
		t.Fatalf("nest after break: want %q, got %q", want, got)
	}
}

func TestNestZeroIsIdentity(t *testing.T) {
	d := Nest(0, Text("x"))
	if got := RenderString(80, d); got != "x" {
		t.Fatalf("nest 0: want %q, got %q", "x", got)
	}
}

func TestGroupFlattensInnerGroup(t *testing.T) {
	inner := Group(Concat(Text("x"), SoftLine(), Text("y")))
	outer := Group(Concat(Text("["), inner, Text("]")))
	got := RenderString(80, outer)
	if got != "[x y]" {
		t.Fatalf("nested flat: want %q, got %q", "[x y]", got)
	}
}

func TestOuterBreakAllowsInnerFlat(t *testing.T) {
	// Outer group has 3 items each with soft lines. When outer breaks,
	// each item is a small group that can still sit on one line.
	item := func(name string) Doc {
		return Group(Concat(Text(name), SoftLine(), Text("= 1")))
	}
	outer := Group(Join(SoftLine(), item("alpha"), item("beta"), item("gamma")))
	// Width 10 forces outer to break; inner groups each fit on their line.
	got := RenderString(10, outer)
	want := "alpha = 1\nbeta = 1\ngamma = 1"
	if got != want {
		t.Fatalf("outer break inner flat:\nwant %q\ngot  %q", want, got)
	}
}

func TestHardLineInGroupForcesBreak(t *testing.T) {
	// Short content, but a HardLine forces the group to break anyway.
	d := Group(Concat(Text("a"), HardLine(), Text("b"), SoftLine(), Text("c")))
	got := RenderString(80, d)
	want := "a\nb\nc"
	if got != want {
		t.Fatalf("hard line forces break:\nwant %q\ngot  %q", want, got)
	}
}

func TestZeroWidthForcesBreak(t *testing.T) {
	d := Group(Concat(Text("a"), SoftLine(), Text("b")))
	got := RenderString(0, d)
	if got != "a\nb" {
		t.Fatalf("zero width: want %q, got %q", "a\nb", got)
	}
}

func TestJoinWithSeparator(t *testing.T) {
	d := Join(Text(","), Text("a"), Text("b"), Text("c"))
	if got := RenderString(80, d); got != "a,b,c" {
		t.Fatalf("join: want %q, got %q", "a,b,c", got)
	}
}

func TestJoinEmpty(t *testing.T) {
	if got := RenderString(80, Join(Text(","))); got != "" {
		t.Fatalf("join empty: want %q, got %q", "", got)
	}
}

func TestRenderToStringBuilder(t *testing.T) {
	var sb strings.Builder
	if err := Render(&sb, 80, Text("ok")); err != nil {
		t.Fatalf("render: %v", err)
	}
	if sb.String() != "ok" {
		t.Fatalf("string builder: got %q", sb.String())
	}
}

func TestRenderToBuffer(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, 80, Concat(Text("a"), HardLine(), Text("b"))); err != nil {
		t.Fatalf("render: %v", err)
	}
	if buf.String() != "a\nb" {
		t.Fatalf("buffer: got %q", buf.String())
	}
}

type failingWriter struct{ err error }

func (f failingWriter) Write(p []byte) (int, error) { return 0, f.err }

func TestRenderPropagatesWriteError(t *testing.T) {
	sentinel := errors.New("boom")
	err := Render(failingWriter{err: sentinel}, 80, Text("x"))
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error, got %v", err)
	}
}

func TestRenderStopsOnError(t *testing.T) {
	sentinel := errors.New("boom")
	// Use a document that would write many times; ensure we don't panic
	// or infinite-loop and still return the error.
	d := Concat(Text("a"), HardLine(), Text("b"), HardLine(), Text("c"))
	err := Render(failingWriter{err: sentinel}, 80, d)
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error, got %v", err)
	}
}

func TestDeepIndent(t *testing.T) {
	// Indent beyond the 64-column reuse chunk to exercise the loop.
	d := Nest(130, Concat(HardLine(), Text("x")))
	got := RenderString(80, d)
	want := "\n" + strings.Repeat(" ", 130) + "x"
	if got != want {
		t.Fatalf("deep indent: want %d bytes, got %d", len(want), len(got))
	}
}

func TestNestedNestAccumulates(t *testing.T) {
	d := Nest(2, Nest(3, Concat(HardLine(), Text("x"))))
	got := RenderString(80, d)
	want := "\n     x"
	if got != want {
		t.Fatalf("nested nest: want %q, got %q", want, got)
	}
}

func TestGroupBoundedLookahead(t *testing.T) {
	// Build a chain of groups separated by HardLines. Each outer group's
	// fits() check must terminate at the first break-mode line rather
	// than scanning the full document.
	const n = 200
	d := Text("start")
	for i := 0; i < n; i++ {
		d = Concat(d, HardLine(), Group(Concat(Text("x"), SoftLine(), Text("y"))))
	}
	got := RenderString(80, d)
	// Each inner group fits flat as "x y".
	want := "start" + strings.Repeat("\nx y", n)
	if got != want {
		t.Fatalf("lookahead chain produced wrong output (len got=%d want=%d)", len(got), len(want))
	}
}

func TestFlatSoftLineIsSingleSpace(t *testing.T) {
	d := Group(Concat(Text("a"), SoftLine(), SoftLine(), Text("b")))
	if got := RenderString(80, d); got != "a  b" {
		t.Fatalf("consecutive soft lines flat: want %q, got %q", "a  b", got)
	}
}

func TestBrokenSoftLinesEachBreak(t *testing.T) {
	d := Group(Concat(Text("aaaa"), SoftLine(), Text("bbbb"), SoftLine(), Text("cccc")))
	got := RenderString(5, d)
	want := "aaaa\nbbbb\ncccc"
	if got != want {
		t.Fatalf("multi soft break: want %q, got %q", want, got)
	}
}

func TestGroupWithOnlyText(t *testing.T) {
	d := Group(Text("short"))
	if got := RenderString(80, d); got != "short" {
		t.Fatalf("group text only: got %q", got)
	}
}

func TestRenderAppendsWithMultipleTopLevelFrames(t *testing.T) {
	a := Group(Concat(Text("x"), SoftLine(), Text("y")))
	d := Concat(a, Text("!"))
	if got := RenderString(80, d); got != "x y!" {
		t.Fatalf("top-level concat: got %q", got)
	}
}
