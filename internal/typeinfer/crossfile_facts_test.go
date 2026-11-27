package typeinfer

import (
	"slices"
	"strings"
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

func TestNamesDependingOnFollowsTypesOnly(t *testing.T) {
	info := IndexFileParallel(parseTestFile(t, "package p\n\nclass Sub : Hub() {\n    fun load() {}\n    fun onCreate() {}\n}\n\nclass Other\n\ntypealias H = Hub\n\nfun makeHub(): Hub = Hub()\n"))
	got := info.NamesDependingOn(map[string]bool{"Hub": true})
	for _, want := range []string{"Sub", "p.Sub", "H", "makeHub"} {
		if !slices.Contains(got, want) {
			t.Errorf("NamesDependingOn(Hub) = %v, missing %q", got, want)
		}
	}
	for _, unwanted := range []string{"Other", "load", "onCreate"} {
		if slices.Contains(got, unwanted) {
			t.Errorf("NamesDependingOn(Hub) = %v, must not expand %q", got, unwanted)
		}
	}
}

// An import that names a class's supertype is part of what other files resolve: a rule reading the supertype
// through the declaring file's imports sees a different class when only that import changes (#1857). Other import
// edits keep the hash.
func TestCrossFileFactsHashFollowsSupertypeImports(t *testing.T) {
	base := "package p\n\nimport android.view.View\n\nopen class Base : View()\n"
	same := map[string]string{
		"unrelated import added":   "package p\n\nimport android.view.View\nimport java.util.Date\n\nopen class Base : View()\n",
		"unrelated wildcard added": "package p\n\nimport android.view.View\nimport java.util.*\n\nopen class Base : View()\n",
		// The resolver never resolves through on-demand imports, so no lookup follows them.
		"wildcard swapped": "package p\n\nimport android.view.View\nimport q.*\n\nopen class Base : View()\n",
	}
	changed := map[string]string{
		"supertype import swapped": "package p\n\nimport q.View\n\nopen class Base : View()\n",
		"supertype import aliased": "package p\n\nimport q.Widget as View\n\nopen class Base : View()\n",
		"supertype import removed": "package p\n\nopen class Base : View()\n",
	}
	want := crossFileFactsOf(t, base).Hash
	for name, src := range same {
		if got := crossFileFactsOf(t, src).Hash; got != want {
			t.Errorf("%s changed the hash", name)
		}
	}
	for name, src := range changed {
		if got := crossFileFactsOf(t, src).Hash; got == want {
			t.Errorf("%s kept the hash", name)
		}
	}
}

// #1894's review (P1): an on-demand import never changes a lookup (ImportTable.Resolve doesn't read them), so adding
// one (an unrelated `import kotlin.random.*`) keeps the hash, whatever the class's supertypes. Hashing them made such
// an edit re-analyze every file using the class (600 files on ktor).
func TestCrossFileFactsHashIgnoresWildcardImports(t *testing.T) {
	for name, src := range map[string]string{
		"supertype from no import": "package p\n\nclass Base : Other()\n",
		"supertype from a star":    "package p\n\nimport android.view.*\n\nclass Base : View()\n",
		"same-file supertype":      "package p\n\nopen class Other\n\nclass Base : Other()\n",
		"qualified supertype":      "package p\n\nclass Base : android.view.View()\n",
		"explicit import":          "package p\n\nimport android.view.View\n\nclass Base : View()\n",
		"no supertype":             "package p\n\nclass Base\n",
	} {
		base := crossFileFactsOf(t, src).Hash
		withStar := crossFileFactsOf(t, strings.Replace(src, "package p\n", "package p\n\nimport kotlin.random.*\n", 1)).Hash
		if base != withStar {
			t.Errorf("%s: an unrelated wildcard import changed the hash", name)
		}
	}
}
