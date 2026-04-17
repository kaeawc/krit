package typeinfer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// parseTestFile writes Kotlin source to a temp file and parses it with tree-sitter.
func parseTestFile(t *testing.T, src string) *scanner.File {
	t.Helper()
	tmpFile := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(tmpFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// --- FakeResolver basic behavior ---

func TestFakeResolver_ResolveImport(t *testing.T) {
	fake := NewFakeResolver()
	fake.Imports["Date"] = "java.util.Date"
	fake.Imports["ML"] = "kotlin.collections.MutableList"

	src := `package com.example
class Foo
`
	file := parseTestFile(t, src)

	if got := fake.ResolveImport("Date", file); got != "java.util.Date" {
		t.Errorf("expected java.util.Date, got %q", got)
	}
	if got := fake.ResolveImport("ML", file); got != "kotlin.collections.MutableList" {
		t.Errorf("expected kotlin.collections.MutableList, got %q", got)
	}
	if got := fake.ResolveImport("Unknown", file); got != "" {
		t.Errorf("expected empty string for unknown import, got %q", got)
	}
}

func TestFakeResolver_ResolveByNameFlat(t *testing.T) {
	fake := NewFakeResolver()
	strType := &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypePrimitive}
	fake.NameTypes["name"] = strType

	file := parseTestFile(t, `fun example() { val name = "hello" }`)
	if got := fake.ResolveByNameFlat("name", 1, file); got != strType {
		t.Errorf("expected String type, got %v", got)
	}
	if got := fake.ResolveByNameFlat("missing", 1, file); got != nil {
		t.Errorf("expected nil for missing name, got %v", got)
	}
}

func TestFakeResolver_ClassHierarchy(t *testing.T) {
	fake := NewFakeResolver()
	fake.Classes["Result"] = &ClassInfo{
		Name:     "Result",
		FQN:      "com.example.Result",
		Kind:     "sealed class",
		IsSealed: true,
	}

	got := fake.ClassHierarchy("Result")
	if got == nil {
		t.Fatal("expected ClassInfo, got nil")
	}
	if got.FQN != "com.example.Result" {
		t.Errorf("expected com.example.Result, got %q", got.FQN)
	}
	if !got.IsSealed {
		t.Error("expected sealed class")
	}
	if fake.ClassHierarchy("Missing") != nil {
		t.Error("expected nil for missing class")
	}
}

func TestFakeResolver_SealedVariants(t *testing.T) {
	fake := NewFakeResolver()
	fake.SealedMap["Result"] = []string{"Success", "Failure"}

	got := fake.SealedVariants("Result")
	if len(got) != 2 || got[0] != "Success" || got[1] != "Failure" {
		t.Errorf("expected [Success Failure], got %v", got)
	}
	if fake.SealedVariants("Missing") != nil {
		t.Error("expected nil for missing sealed class")
	}
}

func TestFakeResolver_EnumEntries(t *testing.T) {
	fake := NewFakeResolver()
	fake.EnumMap["Color"] = []string{"RED", "GREEN", "BLUE"}

	got := fake.EnumEntries("Color")
	if len(got) != 3 || got[0] != "RED" || got[1] != "GREEN" || got[2] != "BLUE" {
		t.Errorf("expected [RED GREEN BLUE], got %v", got)
	}
	if fake.EnumEntries("Missing") != nil {
		t.Error("expected nil for missing enum")
	}
}

func TestFakeResolver_AnnotationValueFlat(t *testing.T) {
	fake := NewFakeResolver()
	fake.Annotations["RequiresApi.value"] = "26"
	fake.Annotations["Deprecated.message"] = "Use newMethod instead"

	src := `fun f() { }`
	file := parseTestFile(t, src)

	if got := fake.AnnotationValueFlat(1, file, "RequiresApi", "value"); got != "26" {
		t.Errorf("expected 26, got %q", got)
	}
	if got := fake.AnnotationValueFlat(1, file, "Deprecated", "message"); got != "Use newMethod instead" {
		t.Errorf("expected 'Use newMethod instead', got %q", got)
	}
	if got := fake.AnnotationValueFlat(1, file, "Missing", "arg"); got != "" {
		t.Errorf("expected empty for missing annotation, got %q", got)
	}
}

func TestDefaultResolver_ResolveByNameFlat_UsesScopeOffset(t *testing.T) {
	src := `
package com.example

fun example() {
    val value = "outer"
    if (true) {
        val value = 42
        println(value)
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected indexed file info")
	}

	resolver := NewResolver()
	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}

	outerType := &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypePrimitive}
	innerType := &ResolvedType{Name: "Int", FQN: "kotlin.Int", Kind: TypePrimitive}
	root := &ScopeTable{
		Entries:        map[string]*ResolvedType{"value": outerType},
		SmartCasts:     map[string]bool{},
		SmartCastTypes: map[string]*ResolvedType{},
	}
	inner := root.NewScope()
	inner.Entries["value"] = innerType
	innerStart := uint32(strings.Index(src, "val value = 42"))
	innerEnd := uint32(strings.Index(src, "println(value)") + len("println(value)"))
	inner.StartByte = innerStart
	inner.EndByte = innerEnd
	resolver.scopes[fi.Path] = root

	identStart := strings.Index(src, "println(value)") + len("println(")
	identEnd := identStart + len("value")
	idx, ok := file.FlatNamedDescendantForByteRange(uint32(identStart), uint32(identEnd))
	if !ok {
		t.Fatal("expected to locate flat identifier for inner value")
	}

	got := resolver.ResolveByNameFlat("value", idx, file)
	if got == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if got.Name != "Int" {
		t.Errorf("expected inner scope type Int, got %q", got.Name)
	}
}

func TestDefaultResolver_AnnotationValueFlat(t *testing.T) {
	src := `
@RequiresApi(value = 26)
fun oldApi() {}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected indexed file info")
	}

	resolver := NewResolver()
	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope

	start := strings.Index(src, "oldApi")
	idx, ok := file.FlatNamedDescendantForByteRange(uint32(start), uint32(start+len("oldApi")))
	if !ok {
		t.Fatal("expected function identifier index")
	}

	if got := resolver.AnnotationValueFlat(idx, file, "RequiresApi", "value"); got != "26" {
		t.Errorf("expected annotation value 26, got %q", got)
	}
}

// --- ImportTable with parsed Kotlin files ---

func TestImportTable_ResolveFromKotlinSource(t *testing.T) {
	// Simulate building an ImportTable from parsed Kotlin source
	src := `
package com.example
import java.util.Date
import kotlin.collections.MutableList as ML

class Foo {
    val d: Date = Date()
    val list: ML = mutableListOf()
}
`
	file := parseTestFile(t, src)
	if file.FlatTree == nil {
		t.Fatal("expected parsed tree")
	}

	// Build an ImportTable manually as a concrete resolver would
	table := &ImportTable{
		Explicit: map[string]string{
			"Date": "java.util.Date",
		},
		Aliases: map[string]string{
			"ML": "kotlin.collections.MutableList",
		},
	}

	if got := table.Resolve("Date"); got != "java.util.Date" {
		t.Errorf("expected java.util.Date, got %q", got)
	}
	if got := table.Resolve("ML"); got != "kotlin.collections.MutableList" {
		t.Errorf("expected kotlin.collections.MutableList, got %q", got)
	}
	if got := table.Resolve("String"); got != "kotlin.String" {
		t.Errorf("expected kotlin.String for primitive fallback, got %q", got)
	}
}

// --- ScopeTable with declaration types ---

func TestScopeTable_DeclarationTypes(t *testing.T) {
	src := `
fun example() {
    val name: String = "hello"
    val count = 42
    val flag = true
    val nullable: String? = null
    val list = mutableListOf<Int>()
}
`
	file := parseTestFile(t, src)
	if file.FlatTree == nil {
		t.Fatal("expected parsed tree")
	}

	// Simulate what a real resolver would populate in the scope table
	scope := &ScopeTable{Entries: make(map[string]*ResolvedType)}
	scope.Declare("name", &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypePrimitive})
	scope.Declare("count", &ResolvedType{Name: "Int", FQN: "kotlin.Int", Kind: TypePrimitive})
	scope.Declare("flag", &ResolvedType{Name: "Boolean", FQN: "kotlin.Boolean", Kind: TypePrimitive})
	scope.Declare("nullable", &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypeNullable, Nullable: true})
	scope.Declare("list", &ResolvedType{
		Name: "MutableList",
		FQN:  "kotlin.collections.MutableList",
		Kind: TypeClass,
		TypeArgs: []ResolvedType{
			{Name: "Int", FQN: "kotlin.Int", Kind: TypePrimitive},
		},
	})

	tests := []struct {
		name     string
		wantName string
		wantNull bool
		wantMut  bool
	}{
		{"name", "String", false, false},
		{"count", "Int", false, false},
		{"flag", "Boolean", false, false},
		{"nullable", "String", true, false},
		{"list", "MutableList", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scope.Lookup(tt.name)
			if got == nil {
				t.Fatalf("expected type for %q, got nil", tt.name)
			}
			if got.Name != tt.wantName {
				t.Errorf("expected name %q, got %q", tt.wantName, got.Name)
			}
			if got.IsNullable() != tt.wantNull {
				t.Errorf("expected nullable=%v, got %v", tt.wantNull, got.IsNullable())
			}
			if got.IsMutable() != tt.wantMut {
				t.Errorf("expected mutable=%v, got %v", tt.wantMut, got.IsMutable())
			}
		})
	}
}

// --- Nullability tracking ---

func TestNullability_ViaFakeResolver(t *testing.T) {
	src := `
fun example(x: String, y: String?) {
    val a: Int = 1
    val b: Int? = null
}
`
	file := parseTestFile(t, src)
	if file.FlatTree == nil {
		t.Fatal("expected parsed tree")
	}

	fake := NewFakeResolver()
	tr := true
	fa := false
	fake.Nullability["x"] = &fa
	fake.Nullability["y"] = &tr
	fake.Nullability["a"] = &fa
	fake.Nullability["b"] = &tr

	// Verify the configured nullability
	tests := []struct {
		name     string
		wantNull bool
	}{
		{"x", false},
		{"y", true},
		{"a", false},
		{"b", true},
	}

	for _, tt := range tests {
		val := fake.Nullability[tt.name]
		if val == nil {
			t.Fatalf("missing nullability for %q", tt.name)
		}
		if *val != tt.wantNull {
			t.Errorf("%q: expected nullable=%v, got %v", tt.name, tt.wantNull, *val)
		}
	}
}

// --- Class hierarchy: sealed classes and enums ---

func TestClassHierarchy_SealedAndEnum(t *testing.T) {
	src := `
sealed class Result {
    data class Success(val value: String) : Result()
    data class Failure(val error: Throwable) : Result()
}

enum class Color { RED, GREEN, BLUE }
`
	file := parseTestFile(t, src)
	if file.FlatTree == nil {
		t.Fatal("expected parsed tree")
	}

	fake := NewFakeResolver()
	fake.SealedMap["Result"] = []string{"Success", "Failure"}
	fake.EnumMap["Color"] = []string{"RED", "GREEN", "BLUE"}
	fake.Classes["Result"] = &ClassInfo{
		Name:     "Result",
		Kind:     "sealed class",
		IsSealed: true,
	}
	fake.Classes["Color"] = &ClassInfo{
		Name: "Color",
		Kind: "enum",
	}

	// Sealed variants
	variants := fake.SealedVariants("Result")
	if len(variants) != 2 {
		t.Fatalf("expected 2 sealed variants, got %d", len(variants))
	}
	if variants[0] != "Success" || variants[1] != "Failure" {
		t.Errorf("expected [Success Failure], got %v", variants)
	}

	// Enum entries
	entries := fake.EnumEntries("Color")
	if len(entries) != 3 {
		t.Fatalf("expected 3 enum entries, got %d", len(entries))
	}
	expected := []string{"RED", "GREEN", "BLUE"}
	for i, e := range expected {
		if entries[i] != e {
			t.Errorf("entry %d: expected %q, got %q", i, e, entries[i])
		}
	}
}

// --- Annotation values ---

func TestAnnotationValues_WithParsedSource(t *testing.T) {
	src := `
@RequiresApi(26)
fun newApi() { }

@Deprecated("Use newMethod instead")
fun oldMethod() { }
`
	file := parseTestFile(t, src)
	if file.FlatTree == nil {
		t.Fatal("expected parsed tree")
	}

	fake := NewFakeResolver()
	fake.Annotations["RequiresApi.value"] = "26"
	fake.Annotations["Deprecated.value"] = "Use newMethod instead"

	if got := fake.AnnotationValueFlat(0, file, "RequiresApi", "value"); got != "26" {
		t.Errorf("expected 26, got %q", got)
	}
	if got := fake.AnnotationValueFlat(0, file, "Deprecated", "value"); got != "Use newMethod instead" {
		t.Errorf("expected 'Use newMethod instead', got %q", got)
	}
}

// --- FakeResolver satisfies TypeResolver interface ---

func TestFakeResolver_ImplementsInterface(t *testing.T) {
	var _ TypeResolver = NewFakeResolver()
}

// --- Tree-sitter parsing sanity checks ---

func TestParseTestFile_ValidKotlin(t *testing.T) {
	src := `
package com.example

fun main() {
    println("Hello")
}
`
	file := parseTestFile(t, src)
	if file.FlatTree == nil || len(file.FlatTree.Nodes) == 0 {
		t.Fatal("expected parsed tree")
	}
	// Match the original "any children" semantics: the pre-migration test
	// called root.ChildCount() which counted both named and anonymous children.
	// `FlatChildCount` (not `FlatNamedChildCount`) is the direct equivalent.
	if file.FlatChildCount(0) == 0 {
		t.Error("expected root to have children")
	}
}

func TestParseTestFile_HasContent(t *testing.T) {
	src := `val x = 1`
	file := parseTestFile(t, src)
	if len(file.Content) == 0 {
		t.Error("expected non-empty content")
	}
	if file.Path == "" {
		t.Error("expected non-empty path")
	}
}

// --- Smart cast tests ---

func TestSmartCast_ScopeTable_Basic(t *testing.T) {
	// A scope with a smart cast should return non-null for a nullable variable
	parent := &ScopeTable{
		Entries:    make(map[string]*ResolvedType),
		SmartCasts: make(map[string]bool),
	}
	parent.Declare("x", &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypeNullable, Nullable: true})

	child := parent.NewScope()
	child.SmartCasts["x"] = true

	// In parent scope, x should be nullable
	got := parent.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in parent")
	}
	if !got.IsNullable() {
		t.Error("expected x to be nullable in parent scope")
	}

	// In child scope with smart cast, x should be non-null
	got = child.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in child")
	}
	if got.IsNullable() {
		t.Error("expected x to be non-null in smart-cast child scope")
	}
}

func TestSmartCast_IsSmartCastNonNull_Inheritance(t *testing.T) {
	parent := &ScopeTable{SmartCasts: map[string]bool{"x": true}}
	child := parent.NewScope()
	grandchild := child.NewScope()

	// Smart cast should be visible in all descendant scopes
	if !child.IsSmartCastNonNull("x") {
		t.Error("expected x to be smart-cast non-null in child")
	}
	if !grandchild.IsSmartCastNonNull("x") {
		t.Error("expected x to be smart-cast non-null in grandchild")
	}
	if parent.IsSmartCastNonNull("y") {
		t.Error("expected y to NOT be smart-cast non-null")
	}
}

func TestSmartCast_IfNotNull(t *testing.T) {
	src := `
fun example(x: String?) {
    if (x != null) {
        val len = x.length
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	// The function scope is a child of the root scope
	if len(fi.RootScope.Children) == 0 {
		t.Fatal("expected at least one child scope (function scope)")
	}
	funcScope := fi.RootScope.Children[0]

	// x should be declared as nullable in the function scope
	rawType := funcScope.lookupRaw("x")
	if rawType == nil {
		t.Fatal("expected x to be declared in function scope")
	}
	if !rawType.Nullable {
		t.Error("expected raw x declaration to be nullable")
	}

	// The if-body scope should have a smart cast for x
	if len(funcScope.Children) == 0 {
		t.Fatal("expected at least one child scope (if-body scope)")
	}
	// Find the child scope that has the smart cast
	var ifBodyScope *ScopeTable
	for _, child := range funcScope.Children {
		if child.SmartCasts["x"] {
			ifBodyScope = child
			break
		}
	}
	if ifBodyScope == nil {
		t.Fatal("expected a child scope with smart cast for x")
	}

	// Inside the if-body scope, x should be non-null via smart cast
	got := ifBodyScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in if-body scope")
	}
	if got.IsNullable() {
		t.Error("expected x to be non-null in if-body (smart cast)")
	}

	// In the function scope (outside if), x should still be nullable
	got = funcScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in function scope")
	}
	if !got.IsNullable() {
		t.Error("expected x to still be nullable outside if-body")
	}
}

func TestSmartCast_RequireNotNull(t *testing.T) {
	src := `
fun example(x: String?) {
    requireNotNull(x)
    val len = x.length
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	// The function scope is a child of the root scope
	if len(fi.RootScope.Children) == 0 {
		t.Fatal("expected at least one child scope (function scope)")
	}
	funcScope := fi.RootScope.Children[0]

	// After requireNotNull(x), the function scope should have x as smart-cast non-null
	if !funcScope.IsSmartCastNonNull("x") {
		t.Error("expected x to be smart-cast non-null after requireNotNull")
	}

	// Lookup should return non-null version
	got := funcScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x")
	}
	if got.IsNullable() {
		t.Error("expected x to be non-null after requireNotNull via Lookup")
	}
}

func TestSmartCast_CheckNotNull(t *testing.T) {
	src := `
fun example(x: String?) {
    checkNotNull(x)
    val len = x.length
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if len(fi.RootScope.Children) == 0 {
		t.Fatal("expected at least one child scope (function scope)")
	}
	funcScope := fi.RootScope.Children[0]

	if !funcScope.IsSmartCastNonNull("x") {
		t.Error("expected x to be smart-cast non-null after checkNotNull")
	}
}

func TestSmartCast_NotAppliedOutsideScope(t *testing.T) {
	// Smart casts inside if body should NOT leak to the outer scope
	parent := &ScopeTable{
		Entries:    make(map[string]*ResolvedType),
		SmartCasts: make(map[string]bool),
	}
	parent.Declare("x", &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypeNullable, Nullable: true})

	// Create a child scope that has the smart cast (simulating if-body)
	child := parent.NewScope()
	child.SmartCasts["x"] = true

	// Parent should still see x as nullable
	got := parent.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in parent")
	}
	if !got.IsNullable() {
		t.Error("expected x to still be nullable in outer scope")
	}

	// Child should see x as non-null
	got = child.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in child")
	}
	if got.IsNullable() {
		t.Error("expected x to be non-null in if-body scope")
	}
}

func TestSmartCast_NullCheckEarlyReturn(t *testing.T) {
	src := `
fun example(x: String?) {
    if (x == null) return
    val len = x.length
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if len(fi.RootScope.Children) == 0 {
		t.Fatal("expected at least one child scope (function scope)")
	}
	funcScope := fi.RootScope.Children[0]

	// After `if (x == null) return`, x should be smart-cast non-null in the function scope
	if !funcScope.IsSmartCastNonNull("x") {
		t.Error("expected x to be smart-cast non-null after null-check early return")
	}
}

func TestSmartCast_ElvisReturn(t *testing.T) {
	src := `
fun example(x: String?) {
    x ?: return
    val len = x.length
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if len(fi.RootScope.Children) == 0 {
		t.Fatal("expected at least one child scope (function scope)")
	}
	funcScope := fi.RootScope.Children[0]

	// After `x ?: return`, x should be smart-cast non-null
	if !funcScope.IsSmartCastNonNull("x") {
		t.Error("expected x to be smart-cast non-null after elvis return")
	}
}

func TestNewScopeForNode(t *testing.T) {
	// Use a real parsed Kotlin function declaration so we exercise the live
	// byte-range plumbing end-to-end, not a synthetic span.
	src := `
fun example() {
    val x = 1
}
`
	file := parseTestFile(t, src)
	funcIdx := flatFirstOfType(file, "function_declaration")
	if funcIdx == 0 {
		t.Fatal("expected to find a function_declaration")
	}

	span := flatScopeSpan{
		start: file.FlatStartByte(funcIdx),
		end:   file.FlatEndByte(funcIdx),
	}
	if span.end <= span.start {
		t.Fatalf("expected non-empty span, got start=%d end=%d", span.start, span.end)
	}

	parent := &ScopeTable{
		Entries:        make(map[string]*ResolvedType),
		SmartCasts:     make(map[string]bool),
		SmartCastTypes: make(map[string]*ResolvedType),
	}

	child := parent.NewScopeForNode(span)
	if child == nil {
		t.Fatal("expected non-nil child scope")
	}
	if child.Parent != parent {
		t.Error("expected child.Parent to be the parent scope")
	}
	if child.StartByte != span.start {
		t.Errorf("expected StartByte=%d, got %d", span.start, child.StartByte)
	}
	if child.EndByte != span.end {
		t.Errorf("expected EndByte=%d, got %d", span.end, child.EndByte)
	}
	if len(parent.Children) != 1 {
		t.Errorf("expected 1 child in parent, got %d", len(parent.Children))
	}

	// The produced scope should resolve back to itself via byte-offset lookup.
	found := parent.FindScopeAtOffset(span.start + 1)
	if found != child {
		t.Errorf("expected FindScopeAtOffset to return the child scope")
	}
}
