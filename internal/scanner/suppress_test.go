package scanner

import (
	"context"
	"strconv"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

func BuildSuppressionIndex(root *sitter.Node, content []byte) *SuppressionIndex {
	return BuildSuppressionIndexFlat(flattenTree(root), content)
}

// --- BuildSuppressionIndex tests ---

func TestBuildSuppressionIndex_WithSuppressAnnotation(t *testing.T) {
	src := `@Suppress("MagicNumber")
fun foo() {
    val x = 42
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	if len(idx.suppressions) == 0 {
		t.Fatal("expected at least one suppression, got none")
	}

	s := idx.suppressions[0]
	if s.Rules == nil {
		t.Fatal("expected non-nil rules map (not suppress-all)")
	}
	if !s.Rules["MagicNumber"] {
		t.Errorf("expected MagicNumber in suppressed rules, got %v", s.Rules)
	}
}

func TestBuildSuppressionIndex_NoSuppressions(t *testing.T) {
	src := `fun bar() {
    val y = 10
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	if len(idx.suppressions) != 0 {
		t.Errorf("expected 0 suppressions, got %d", len(idx.suppressions))
	}
}

func TestBuildSuppressionIndex_SuppressAll(t *testing.T) {
	src := `@Suppress("all")
fun baz() {
    val z = 99
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	if len(idx.suppressions) == 0 {
		t.Fatal("expected at least one suppression, got none")
	}

	s := idx.suppressions[0]
	if s.Rules != nil {
		t.Errorf("expected nil rules (suppress-all), got %v", s.Rules)
	}
}

func TestBuildSuppressionIndex_SuppressWarnings(t *testing.T) {
	src := `@SuppressWarnings("UnusedVariable")
fun qux() {
    val unused = 1
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	if len(idx.suppressions) == 0 {
		t.Fatal("expected at least one suppression, got none")
	}

	s := idx.suppressions[0]
	if s.Rules == nil {
		t.Fatal("expected non-nil rules map")
	}
	if !s.Rules["UnusedVariable"] {
		t.Errorf("expected UnusedVariable in suppressed rules, got %v", s.Rules)
	}
}

func TestBuildSuppressionIndex_ModifierAnnotation(t *testing.T) {
	src := `class Demo {
    @Suppress("MagicNumber")
    fun foo() {
        val x = 42
    }
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	valOffset := findSubstringOffset(content, "val x = 42")
	if valOffset < 0 {
		t.Fatal("could not find 'val x = 42' in source")
	}

	if !idx.IsSuppressed(valOffset, "MagicNumber", "") {
		t.Error("expected modifier-based suppression to apply inside function body")
	}
}

// --- IsSuppressed tests ---

func TestIsSuppressed_InsideSuppressedScope(t *testing.T) {
	src := `@Suppress("MagicNumber")
fun foo() {
    val x = 42
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	// Find the "42" literal inside the function body
	valOffset := findSubstringOffset(content, "val x = 42")
	if valOffset < 0 {
		t.Fatal("could not find 'val x = 42' in source")
	}

	if !idx.IsSuppressed(valOffset, "MagicNumber", "") {
		t.Error("expected IsSuppressed to return true inside suppressed scope")
	}
}

func TestIsSuppressed_OutsideSuppressedScope(t *testing.T) {
	src := `@Suppress("MagicNumber")
fun foo() {
    val x = 42
}

fun bar() {
    val y = 99
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	// bar's body should not be suppressed
	barOffset := findSubstringOffset(content, "val y = 99")
	if barOffset < 0 {
		t.Fatal("could not find 'val y = 99' in source")
	}

	if idx.IsSuppressed(barOffset, "MagicNumber", "") {
		t.Error("expected IsSuppressed to return false outside suppressed scope")
	}
}

func TestIsSuppressed_DifferentRuleName(t *testing.T) {
	src := `@Suppress("MagicNumber")
fun foo() {
    val x = 42
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	valOffset := findSubstringOffset(content, "val x = 42")
	if valOffset < 0 {
		t.Fatal("could not find 'val x = 42' in source")
	}

	if idx.IsSuppressed(valOffset, "UnusedVariable", "") {
		t.Error("expected IsSuppressed to return false for a different rule name")
	}
}

func TestIsSuppressed_AllSuppressesAnyRule(t *testing.T) {
	src := `@Suppress("all")
fun foo() {
    val x = 42
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	valOffset := findSubstringOffset(content, "val x = 42")
	if valOffset < 0 {
		t.Fatal("could not find 'val x = 42' in source")
	}

	if !idx.IsSuppressed(valOffset, "MagicNumber", "") {
		t.Error("expected 'all' suppression to suppress MagicNumber")
	}
	if !idx.IsSuppressed(valOffset, "UnusedVariable", "") {
		t.Error("expected 'all' suppression to suppress UnusedVariable")
	}
	if !idx.IsSuppressed(valOffset, "AnyArbitraryRule", "") {
		t.Error("expected 'all' suppression to suppress any arbitrary rule")
	}
}

func TestIsSuppressed_MultipleSuppressions(t *testing.T) {
	src := `@Suppress("MagicNumber")
fun foo() {
    val x = 42
}

@Suppress("UnusedVariable")
fun bar() {
    val y = 99
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	if len(idx.suppressions) < 2 {
		t.Fatalf("expected at least 2 suppressions, got %d", len(idx.suppressions))
	}

	fooOffset := findSubstringOffset(content, "val x = 42")
	barOffset := findSubstringOffset(content, "val y = 99")
	if fooOffset < 0 || barOffset < 0 {
		t.Fatal("could not find expected substrings in source")
	}

	// foo is suppressed for MagicNumber but not UnusedVariable
	if !idx.IsSuppressed(fooOffset, "MagicNumber", "") {
		t.Error("expected MagicNumber suppressed in foo")
	}
	if idx.IsSuppressed(fooOffset, "UnusedVariable", "") {
		t.Error("expected UnusedVariable NOT suppressed in foo")
	}

	// bar is suppressed for UnusedVariable but not MagicNumber
	if !idx.IsSuppressed(barOffset, "UnusedVariable", "") {
		t.Error("expected UnusedVariable suppressed in bar")
	}
	if idx.IsSuppressed(barOffset, "MagicNumber", "") {
		t.Error("expected MagicNumber NOT suppressed in bar")
	}
}

func TestIsSuppressed_EmptyIndex(t *testing.T) {
	idx := &SuppressionIndex{}
	if idx.IsSuppressed(0, "MagicNumber", "") {
		t.Error("expected IsSuppressed to return false on empty index")
	}
}

func TestIsSuppressed_MultipleRulesInSingleAnnotation(t *testing.T) {
	src := `@Suppress("MagicNumber", "UnusedVariable")
fun foo() {
    val x = 42
}
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	valOffset := findSubstringOffset(content, "val x = 42")
	if valOffset < 0 {
		t.Fatal("could not find 'val x = 42' in source")
	}

	if !idx.IsSuppressed(valOffset, "MagicNumber", "") {
		t.Error("expected MagicNumber suppressed")
	}
	if !idx.IsSuppressed(valOffset, "UnusedVariable", "") {
		t.Error("expected UnusedVariable suppressed")
	}
	if idx.IsSuppressed(valOffset, "SomeOtherRule", "") {
		t.Error("expected SomeOtherRule NOT suppressed")
	}
}

// TestIsSuppressed_ExclusiveEndBoundary verifies that the suppression
// range is treated as [StartByte, EndByte) — matching tree-sitter's
// convention where EndByte is one past the last byte of the node. A
// finding whose byte offset coincides with a suppression's EndByte
// is past the annotation's scope and must NOT be suppressed.
func TestIsSuppressed_ExclusiveEndBoundary(t *testing.T) {
	src := `@Suppress("MagicNumber")
fun foo() {
    val x = 42
}
val y = 99
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	if len(idx.suppressions) != 1 {
		t.Fatalf("expected 1 suppression, got %d", len(idx.suppressions))
	}
	s := idx.suppressions[0]

	// Boundary: the EndByte byte itself is past the annotation target.
	if idx.IsSuppressed(s.EndByte, "MagicNumber", "") {
		t.Errorf("byteOffset == EndByte (%d) must not be suppressed", s.EndByte)
	}
	// One byte before the end is still inside.
	if !idx.IsSuppressed(s.EndByte-1, "MagicNumber", "") {
		t.Errorf("byteOffset == EndByte-1 (%d) must still be suppressed", s.EndByte-1)
	}
	// Sanity: a finding past the annotation by a clearly separate value.
	yOffset := findSubstringOffset(content, "val y = 99")
	if idx.IsSuppressed(yOffset, "MagicNumber", "") {
		t.Errorf("finding outside annotation must not be suppressed (offset %d, suppression end %d)", yOffset, s.EndByte)
	}
}

// TestIsSuppressed_OuterFileLevelStillAppliesPastInner verifies that a
// finding past an inner @Suppress(...) annotation still sees an outer
// @file:Suppress(...) covering the entire file. The pre-fix code broke
// out of the suppression-walk loop on the first nested suppression
// whose EndByte was less than the finding's offset, missing the outer
// file-level suppression (which is sorted earlier by StartByte but has
// a larger EndByte).
func TestIsSuppressed_OuterFileLevelStillAppliesPastInner(t *testing.T) {
	src := `@file:Suppress("OuterRule")

@Suppress("InnerRule")
fun foo() {
    val x = 42
}

val y = 99
`
	root, content := parseKotlin(t, src)
	idx := BuildSuppressionIndex(root, content)

	if len(idx.suppressions) < 2 {
		t.Fatalf("expected at least 2 suppressions (file + function), got %d", len(idx.suppressions))
	}

	yOffset := findSubstringOffset(content, "val y = 99")
	if yOffset < 0 {
		t.Fatal("could not find 'val y = 99' in source")
	}

	if !idx.IsSuppressed(yOffset, "OuterRule", "") {
		t.Error("expected file-level OuterRule suppression to apply past the inner function annotation")
	}
	// Sanity: InnerRule is only suppressed inside fun foo, not at val y.
	if idx.IsSuppressed(yOffset, "InnerRule", "") {
		t.Error("InnerRule should not be suppressed past its function annotation")
	}
}

// findSubstringOffset returns the byte offset of the first occurrence of sub in content, or -1.
func findSubstringOffset(content []byte, sub string) int {
	s := string(content)
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func BenchmarkBuildSuppressionIndex_LargePrunedKotlin(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("package bench\n\n")
	for i := 0; i < 120; i++ {
		if i%24 == 0 {
			sb.WriteString("@Suppress(\"MagicNumber\")\n")
		}
		sb.WriteString("fun fn")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("(x: Int): Int {\n")
		sb.WriteString("    var acc = x\n")
		for j := 0; j < 20; j++ {
			sb.WriteString("    if (acc > ")
			sb.WriteString(strconv.Itoa(j))
			sb.WriteString(") {\n")
			sb.WriteString("        acc += ")
			sb.WriteString(strconv.Itoa(j))
			sb.WriteString("\n")
			sb.WriteString("    }\n")
			sb.WriteString("    acc += ")
			sb.WriteString(strconv.Itoa(j))
			sb.WriteString("\n")
		}
		sb.WriteString("    return acc\n")
		sb.WriteString("}\n\n")
	}
	content := []byte(sb.String())
	root, parsedContent := parseKotlinBench(b, string(content))
	_ = parsedContent
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildSuppressionIndex(root, content)
	}
}

func parseKotlinBench(b *testing.B, src string) (*sitter.Node, []byte) {
	b.Helper()
	content := []byte(src)
	parser := sitter.NewParser()
	parser.SetLanguage(kotlin.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		b.Fatalf("failed to parse Kotlin: %v", err)
	}
	return tree.RootNode(), content
}
