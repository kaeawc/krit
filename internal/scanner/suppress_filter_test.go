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

// --- Baseline() tests ---

func TestSuppressionFilter_Baseline_NilReceiver(t *testing.T) {
	var sf *SuppressionFilter
	if sf.Baseline() != nil {
		t.Error("nil receiver Baseline() must return nil")
	}
}

func TestSuppressionFilter_Baseline_NilBaseline(t *testing.T) {
	src := "class X\n"
	f := parsedFileForFilter(t, "X.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")
	if sf.Baseline() != nil {
		t.Error("filter built with nil baseline should return nil from Baseline()")
	}
}

func TestSuppressionFilter_Baseline_WithBaseline(t *testing.T) {
	src := "class X\n"
	f := parsedFileForFilter(t, "X.kt", src)
	bl := &Baseline{
		ManuallySuppressed: map[string]bool{"some-id": true},
		CurrentIssues:      map[string]bool{},
	}
	sf := BuildSuppressionFilter(f, bl, nil, "")
	got := sf.Baseline()
	if got == nil {
		t.Fatal("Baseline() should return the baseline passed to BuildSuppressionFilter")
	}
	if !got.ManuallySuppressed["some-id"] {
		t.Error("Baseline() returned wrong baseline — expected some-id in ManuallySuppressed")
	}
}

// --- Annotations() tests ---

func TestSuppressionFilter_Annotations_NilReceiver(t *testing.T) {
	var sf *SuppressionFilter
	if sf.Annotations() != nil {
		t.Error("nil receiver Annotations() must return nil")
	}
}

func TestSuppressionFilter_Annotations_NormalCase(t *testing.T) {
	src := "@Suppress(\"MagicNumber\")\nfun foo() { val x = 42 }\n"
	f := parsedFileForFilter(t, "foo.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")
	ann := sf.Annotations()
	if ann == nil {
		t.Fatal("Annotations() should return non-nil SuppressionIndex for annotated file")
	}
	if len(ann.suppressions) == 0 {
		t.Error("Annotations() index should contain at least one suppression entry")
	}
}

// --- @file:Suppress / flatProcessFileAnnotation tests ---

func TestSuppressFile_FileAnnotationSuppressesEntireFile(t *testing.T) {
	src := "@file:Suppress(\"SomeRule\")\n\nfun foo() {\n    val x = 1\n}\n\nfun bar() {\n    val y = 2\n}\n"
	f := parsedFileForFilter(t, "file.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	// Rule should be suppressed at any line in the file.
	for _, line := range []int{1, 3, 4, 7, 8} {
		if !sf.IsSuppressed("SomeRule", "", line) {
			t.Errorf("@file:Suppress(\"SomeRule\") should suppress SomeRule at line %d", line)
		}
	}
}

func TestSuppressFile_FileAnnotationDoesNotSuppressOtherRules(t *testing.T) {
	src := "@file:Suppress(\"SomeRule\")\n\nfun foo() {\n    val x = 1\n}\n"
	f := parsedFileForFilter(t, "file.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	if sf.IsSuppressed("OtherRule", "", 4) {
		t.Error("@file:Suppress(\"SomeRule\") must not suppress OtherRule")
	}
}

func TestSuppressFile_FileAnnotationMultipleRules(t *testing.T) {
	src := "@file:Suppress(\"RuleA\", \"RuleB\")\n\nfun foo() { val x = 1 }\n"
	f := parsedFileForFilter(t, "multi.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	for _, rule := range []string{"RuleA", "RuleB"} {
		if !sf.IsSuppressed(rule, "", 3) {
			t.Errorf("@file:Suppress should suppress %q at line 3", rule)
		}
	}
	if sf.IsSuppressed("RuleC", "", 3) {
		t.Error("@file:Suppress(\"RuleA\",\"RuleB\") must not suppress RuleC")
	}
}

// --- detekt: prefix stripping tests ---

func TestDetektPrefixColon_SuppressMatchesRule(t *testing.T) {
	src := "@Suppress(\"detekt:MagicNumber\")\nfun foo() {\n    val x = 42\n}\n"
	f := parsedFileForFilter(t, "foo.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	if !sf.IsSuppressed("MagicNumber", "", 3) {
		t.Error("@Suppress(\"detekt:MagicNumber\") should suppress MagicNumber (detekt: prefix stripped)")
	}
}

func TestDetektPrefixDot_SuppressMatchesRule(t *testing.T) {
	src := "@Suppress(\"detekt.UnusedVariable\")\nfun bar() {\n    val unused = 1\n}\n"
	f := parsedFileForFilter(t, "bar.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	if !sf.IsSuppressed("UnusedVariable", "", 3) {
		t.Error("@Suppress(\"detekt.UnusedVariable\") should suppress UnusedVariable (detekt. prefix stripped)")
	}
}

func TestDetektPrefix_DoesNotMatchDifferentRule(t *testing.T) {
	src := "@Suppress(\"detekt:MagicNumber\")\nfun foo() {\n    val x = 42\n}\n"
	f := parsedFileForFilter(t, "foo.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	if sf.IsSuppressed("UnusedVariable", "", 3) {
		t.Error("detekt:MagicNumber should not suppress UnusedVariable")
	}
}

// --- matchExcludePatternSlash tests ---

func TestMatchExcludePatternSlash_StarStarDirStarStar(t *testing.T) {
	// **/dir/** matches any path containing /dir/
	cases := []struct {
		path    string
		pattern string
		want    bool
	}{
		{"/repo/src/test/Foo.kt", "**/test/**", true},
		{"/repo/src/main/Foo.kt", "**/test/**", false},
		{"/repo/test/sub/Foo.kt", "**/test/**", true},
		{"/test/Foo.kt", "**/test/**", true}, // /test/ is a complete path segment
		{"/repo/testing/Foo.kt", "**/test/**", false},
	}
	for _, tc := range cases {
		got := matchExcludePatternSlash(tc.path, tc.pattern)
		if got != tc.want {
			t.Errorf("matchExcludePatternSlash(%q, %q) = %v, want %v", tc.path, tc.pattern, got, tc.want)
		}
	}
}

func TestMatchExcludePatternSlash_StarStarStarSuffix(t *testing.T) {
	// **/*suffix matches any path ending with suffix
	cases := []struct {
		path    string
		pattern string
		want    bool
	}{
		{"/repo/app/SomethingTest.kt", "**/*Test.kt", true},
		{"/repo/app/Something.kt", "**/*Test.kt", false},
		{"/repo/SomethingTest.kt", "**/*Test.kt", true},
		{"/repo/app/SomethingSpec.kt", "**/*Test.kt", false},
	}
	for _, tc := range cases {
		got := matchExcludePatternSlash(tc.path, tc.pattern)
		if got != tc.want {
			t.Errorf("matchExcludePatternSlash(%q, %q) = %v, want %v", tc.path, tc.pattern, got, tc.want)
		}
	}
}

func TestMatchExcludePatternSlash_StarStarName(t *testing.T) {
	// **/name matches any path ending with /name
	cases := []struct {
		path    string
		pattern string
		want    bool
	}{
		{"/repo/src/Ignored.kt", "**/Ignored.kt", true},
		{"/repo/src/NotIgnored.kt", "**/Ignored.kt", false},
		{"Ignored.kt", "**/Ignored.kt", true}, // bare basename equals suffix
		{"/repo/sub/SomethingIgnored.kt", "**/Ignored.kt", false},
	}
	for _, tc := range cases {
		got := matchExcludePatternSlash(tc.path, tc.pattern)
		if got != tc.want {
			t.Errorf("matchExcludePatternSlash(%q, %q) = %v, want %v", tc.path, tc.pattern, got, tc.want)
		}
	}
}

func TestMatchExcludePatternSlash_PlainGlob(t *testing.T) {
	// plain glob matches the basename only
	cases := []struct {
		path    string
		pattern string
		want    bool
	}{
		{"/repo/src/Foo.kt", "Foo.kt", true},
		{"/repo/src/Bar.kt", "Foo.kt", false},
		{"/repo/src/FooBar.kt", "Foo*.kt", true},
		{"/repo/src/BazFoo.kt", "Foo*.kt", false},
	}
	for _, tc := range cases {
		got := matchExcludePatternSlash(tc.path, tc.pattern)
		if got != tc.want {
			t.Errorf("matchExcludePatternSlash(%q, %q) = %v, want %v", tc.path, tc.pattern, got, tc.want)
		}
	}
}

// --- flatWalkForSuppressions coverage tests ---

func TestFlatWalkSuppressions_ClassLevelSuppress(t *testing.T) {
	src := `@Suppress("MagicNumber")
class MyClass {
    val field = 42
    fun method() {
        val x = 99
    }
}
`
	f := parsedFileForFilter(t, "MyClass.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	// Both field and method body should be suppressed.
	if !sf.IsSuppressed("MagicNumber", "", 3) {
		t.Error("class-level @Suppress should suppress rule at field declaration line")
	}
	if !sf.IsSuppressed("MagicNumber", "", 5) {
		t.Error("class-level @Suppress should suppress rule inside method body")
	}
}

func TestFlatWalkSuppressions_PropertyLevelSuppress(t *testing.T) {
	src := `class Holder {
    @Suppress("LargeClass")
    val data = listOf(1, 2, 3)
    val other = 0
}
`
	f := parsedFileForFilter(t, "Holder.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	if !sf.IsSuppressed("LargeClass", "", 3) {
		t.Error("property-level @Suppress should suppress rule at the property line")
	}
	// The suppression should not bleed to the next property.
	if sf.IsSuppressed("LargeClass", "", 4) {
		t.Error("property-level @Suppress must not bleed to next property")
	}
}

func TestFlatWalkSuppressions_NestedClasses(t *testing.T) {
	src := `class Outer {
    @Suppress("TooManyFunctions")
    class Inner {
        fun a() {}
        fun b() {}
        fun c() {}
    }
    fun outerFun() {}
}
`
	f := parsedFileForFilter(t, "Outer.kt", src)
	sf := BuildSuppressionFilter(f, nil, nil, "")

	// Lines inside Inner should be suppressed.
	if !sf.IsSuppressed("TooManyFunctions", "", 4) {
		t.Error("@Suppress on Inner class should suppress inside Inner")
	}
	// outerFun at line 8 is outside Inner — should not be suppressed.
	if sf.IsSuppressed("TooManyFunctions", "", 8) {
		t.Error("@Suppress on Inner class must not bleed to outerFun")
	}
}
