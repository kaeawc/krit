package typeinfer

import (
	"slices"
	"testing"
)

func crossFileFactsOf(t *testing.T, src string) CrossFileFacts {
	t.Helper()
	return IndexFileParallel(parseTestFile(t, src)).CrossFileFacts()
}

// The hash identifies what other files can resolve from a file: it must
// ignore body-only edits and move with every declaration change (#1778).
func TestCrossFileFactsHash(t *testing.T) {
	base := "package p\n\nopen class Base : View() {\n    fun f(): Int = 1\n    private fun g() = 2\n}\n"
	same := map[string]string{
		"body edit":           "package p\n\nopen class Base : View() {\n    fun f(): Int = 42\n    private fun g() = 2\n}\n",
		"shifted lines":       "package p\n\n\n\nopen class Base : View() {\n    fun f(): Int = 1\n    private fun g() = 2\n}\n",
		"private body change": "package p\n\nopen class Base : View() {\n    fun f(): Int = 1\n    private fun g() = 3\n}\n",
	}
	changed := map[string]string{
		"supertype removed":   "package p\n\nopen class Base {\n    fun f(): Int = 1\n    private fun g() = 2\n}\n",
		"modifier":            "package p\n\nabstract class Base : View() {\n    fun f(): Int = 1\n    private fun g() = 2\n}\n",
		"member signature":    "package p\n\nopen class Base : View() {\n    fun f(): Long = 1\n    private fun g() = 2\n}\n",
		"member added":        "package p\n\nopen class Base : View() {\n    fun f(): Int = 1\n    fun h() {}\n    private fun g() = 2\n}\n",
		"class added":         "package p\n\nopen class Base : View() {\n    fun f(): Int = 1\n    private fun g() = 2\n}\n\nclass Other\n",
		"top-level fun added": "package p\n\nopen class Base : View() {\n    fun f(): Int = 1\n    private fun g() = 2\n}\n\nfun top(): String = \"\"\n",
	}
	want := crossFileFactsOf(t, base)
	if want.Hash == "" {
		t.Fatal("empty hash")
	}
	for name, src := range same {
		if got := crossFileFactsOf(t, src).Hash; got != want.Hash {
			t.Errorf("%s changed the hash", name)
		}
	}
	for name, src := range changed {
		if got := crossFileFactsOf(t, src).Hash; got == want.Hash {
			t.Errorf("%s kept the hash", name)
		}
	}
}

func TestCrossFileFactsNamesIncludeSupertypes(t *testing.T) {
	facts := crossFileFactsOf(t, "package p\n\nsealed class Shape\n\nclass Circle : Shape()\n")
	for _, n := range []string{"Shape", "p.Shape", "Circle", "p.Circle"} {
		if _, ok := facts.ByName[n]; !ok {
			t.Errorf("names %v lack %q", facts.ByName, n)
		}
	}
	if (*FileTypeInfo)(nil).CrossFileFacts().Hash != "" {
		t.Error("nil info must give the zero facts")
	}
}

// Only the lookups a declaration change affects move, so the invalidation
// it seeds stays narrow.
func TestChangedNamesIsPerDeclaration(t *testing.T) {
	before := crossFileFactsOf(t, "package p\n\nclass A\n\nclass B\n").ByName
	after := crossFileFactsOf(t, "package p\n\nclass A\n\nclass B\n\nfun added(): Int = 1\n").ByName
	if got := ChangedNames(before, after); !slices.Equal(got, []string{"added", "p.added"}) && !slices.Equal(got, []string{"added"}) {
		t.Fatalf("adding a function changed %v, want only its name", got)
	}
	sub := crossFileFactsOf(t, "package p\n\nclass A\n\nclass B : A()\n").ByName
	got := ChangedNames(before, sub)
	if !slices.Contains(got, "A") || !slices.Contains(got, "B") {
		t.Fatalf("B becoming a subtype of A changed %v, want both A and B", got)
	}
}
