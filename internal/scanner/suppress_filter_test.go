package scanner

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

func parsedFileForFilter(t *testing.T, path, src string) *File {
	t.Helper()
	parser := sitter.NewParser()
	parser.SetLanguage(kotlin.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return NewParsedFile(path, []byte(src), tree)
}

func TestSuppressionFilter_AnnotationMatches(t *testing.T) {
	src := "@Suppress(\"MagicNumber\")\nclass X {\n    val y = 42\n}\n"
	f := parsedFileForFilter(t, "X.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	if !sf.IsSuppressed("MagicNumber", "style", 3) {
		t.Error("annotation on class X should suppress MagicNumber at line 3")
	}
	if sf.IsSuppressed("OtherRule", "style", 3) {
		t.Error("annotation for MagicNumber should not suppress OtherRule")
	}
}

func TestSuppressionFilter_InlineIgnoreSpecificRule(t *testing.T) {
	src := "class A\nval x = 1 // krit:ignore[MagicNumber]\nval y = 2\n"
	f := parsedFileForFilter(t, "A.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	if !sf.IsSuppressed("MagicNumber", "", 2) {
		t.Error("inline [MagicNumber] should suppress on line 2")
	}
	if sf.IsSuppressed("OtherRule", "", 2) {
		t.Error("inline [MagicNumber] should not suppress OtherRule")
	}
	if sf.IsSuppressed("MagicNumber", "", 3) {
		t.Error("inline suppression should not bleed to line 3")
	}
}

func TestSuppressionFilter_InlineIgnoreAll(t *testing.T) {
	src := "class A\nval x = 1 // krit:ignore-all\n"
	f := parsedFileForFilter(t, "A.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	if !sf.IsSuppressed("AnyRule", "", 2) {
		t.Error("krit:ignore-all should suppress every rule on line 2")
	}
	if sf.IsSuppressed("AnyRule", "", 1) {
		t.Error("krit:ignore-all should not bleed to other lines")
	}
}

func TestSuppressionFilter_InlineIgnoreMultipleRules(t *testing.T) {
	src := "val x = 1 // krit:ignore[A, B, C]\n"
	f := parsedFileForFilter(t, "x.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	for _, rule := range []string{"A", "B", "C"} {
		if !sf.IsSuppressed(rule, "", 1) {
			t.Errorf("expected %q suppressed by inline comment", rule)
		}
	}
	if sf.IsSuppressed("D", "", 1) {
		t.Error("D not listed in inline comment — should not be suppressed")
	}
}

func TestSuppressionFilter_ExcludeGlob(t *testing.T) {
	src := "class T\n"
	f := parsedFileForFilter(t, "/repo/app/SomethingTest.kt", src)
	sf := BuildSuppressionFilter(f, nil, map[string][]string{
		"DeadSymbol": {"**/*Test.kt"},
		"OtherRule":  {"**/*Spec.kt"},
	}, "")

	if !sf.IsSuppressed("DeadSymbol", "", 1) {
		t.Error("exclude glob *Test.kt should suppress DeadSymbol for this path")
	}
	if sf.IsSuppressed("OtherRule", "", 1) {
		t.Error("*Spec.kt glob should not match SomethingTest.kt")
	}
	if !sf.IsFileExcluded("DeadSymbol") {
		t.Error("IsFileExcluded should report true for matched rule")
	}
	if sf.IsFileExcluded("OtherRule") {
		t.Error("IsFileExcluded should report false for non-matching rule")
	}
}

func TestSuppressionFilter_NilSafe(t *testing.T) {
	var sf *SuppressionFilter
	if sf.IsSuppressed("X", "", 1) {
		t.Error("nil filter must not report suppression")
	}
	if sf.IsFileExcluded("X") {
		t.Error("nil filter must not report file excluded")
	}
	if sf.Annotations() != nil {
		t.Error("nil filter.Annotations() must be nil")
	}
}

func TestSuppressionFilter_BuildWithNilFile(t *testing.T) {
	sf := BuildSuppressionFilter(nil, nil, nil, "")
	if sf == nil {
		t.Fatal("BuildSuppressionFilter(nil, ...) must return non-nil filter")
	}
	if sf.IsSuppressed("X", "", 1) {
		t.Error("nil-file filter must not report suppression")
	}
}

func TestSuppressionFilter_InlineInsideStringLiteralIgnored(t *testing.T) {
	// Heuristic: we require `//` before `krit:ignore` on the same line.
	// A string literal containing "krit:ignore" without a `//` should be
	// skipped so user-data lookalikes don't accidentally suppress rules.
	src := "val s = \"krit:ignore[Foo]\"\nval y = 1\n"
	f := parsedFileForFilter(t, "s.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")
	if sf.IsSuppressed("Foo", "", 1) {
		t.Error("krit:ignore inside a string literal must not suppress (no // prefix)")
	}
}
