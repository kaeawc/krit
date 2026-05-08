package typeinfer

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestGenericTypeArg_ListFirst_ReturnsString verifies that first() on List<String>
// resolves to String via generic type argument propagation.
func TestGenericTypeArg_ListFirst_ReturnsString(t *testing.T) {
	src := `
val items: List<String> = listOf()
val result = items.first()
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "call_expression", "items.first()")
	if idx == 0 {
		t.Fatal("expected to find call_expression for items.first()")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for items.first(), got nil")
	}
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
	if got.Nullable {
		t.Error("expected non-nullable")
	}
}

// TestGenericTypeArg_FunctionCallReceiverFirst_ReturnsString verifies that
// generic type argument propagation still works when the receiver itself is a
// call expression.
func TestGenericTypeArg_FunctionCallReceiverFirst_ReturnsString(t *testing.T) {
	src := `
fun provideItems(): List<String> = listOf()
fun main() {
    val result = provideItems().first()
}
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "call_expression", "provideItems().first()")
	if idx == 0 {
		t.Fatal("expected to find call_expression for provideItems().first()")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for provideItems().first(), got nil")
	}
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
	if got.Nullable {
		t.Error("expected non-nullable")
	}
}

// TestGenericTypeArg_ListFirstOrNull_ReturnsNullableInt verifies firstOrNull()
// on List<Int> resolves to nullable Int.
func TestGenericTypeArg_ListFirstOrNull_ReturnsNullableInt(t *testing.T) {
	src := `
val items = listOf<Int>()
val result = items.firstOrNull()
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "call_expression", "items.firstOrNull()")
	if idx == 0 {
		t.Fatal("expected to find call_expression for items.firstOrNull()")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for items.firstOrNull(), got nil")
	}
	if got.Name != "Int" {
		t.Errorf("expected Int, got %q", got.Name)
	}
	if !got.Nullable {
		t.Error("expected nullable")
	}
}

// TestGenericTypeArg_FunctionCallReceiverFirstOrNull_ReturnsNullableString verifies
// nullable generic propagation through a call-expression receiver.
func TestGenericTypeArg_FunctionCallReceiverFirstOrNull_ReturnsNullableString(t *testing.T) {
	src := `
fun provideItems(): List<String> = listOf()
fun main() {
    val result = provideItems().firstOrNull()
}
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "call_expression", "provideItems().firstOrNull()")
	if idx == 0 {
		t.Fatal("expected to find call_expression for provideItems().firstOrNull()")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for provideItems().firstOrNull(), got nil")
	}
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
	if !got.Nullable {
		t.Error("expected nullable")
	}
}

// TestGenericTypeArg_MapGetValue_ReturnsInt verifies getValue() on Map<String,Int>
// resolves to Int (type arg index 1).
func TestGenericTypeArg_MapGetValue_ReturnsInt(t *testing.T) {
	src := `
val m: Map<String, Int> = mapOf()
val result = m.getValue("key")
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "call_expression", `m.getValue("key")`)
	if idx == 0 {
		t.Fatal("expected to find call_expression for m.getValue()")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for m.getValue(), got nil")
	}
	if got.Name != "Int" {
		t.Errorf("expected Int, got %q", got.Name)
	}
}

// TestGenericTypeArg_ListCount_ReturnsInt verifies count() on List<String>
// returns Int (fixed return type, not element type).
func TestGenericTypeArg_ListCount_ReturnsInt(t *testing.T) {
	src := `
val items: List<String> = listOf()
val result = items.count()
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "call_expression", "items.count()")
	if idx == 0 {
		t.Fatal("expected to find call_expression for items.count()")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for items.count(), got nil")
	}
	if got.Name != "Int" {
		t.Errorf("expected Int (not String), got %q", got.Name)
	}
}

// TestGenericTypeArg_ListIsEmpty_ReturnsBoolean verifies isEmpty() on List<String>
// returns Boolean (fixed return type).
func TestGenericTypeArg_ListIsEmpty_ReturnsBoolean(t *testing.T) {
	src := `
val items: List<String> = listOf()
val result = items.isEmpty()
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "call_expression", "items.isEmpty()")
	if idx == 0 {
		t.Fatal("expected to find call_expression for items.isEmpty()")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for items.isEmpty(), got nil")
	}
	if got.Name != "Boolean" {
		t.Errorf("expected Boolean, got %q", got.Name)
	}
}

// TestGenericTypeArg_ListOfCall_RetainsTypeArgument verifies that the flat call
// resolver preserves the explicit type argument on listOf<Int>().
func TestGenericTypeArg_ListOfCall_RetainsTypeArgument(t *testing.T) {
	src := `
val items = listOf<Int>()
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "call_expression", "listOf<Int>()")
	if idx == 0 {
		t.Fatal("expected to find call_expression for listOf<Int>()")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for listOf<Int>(), got nil")
	}
	if got.Name != "List" {
		t.Fatalf("expected List, got %q", got.Name)
	}
	if len(got.TypeArgs) != 1 {
		t.Fatalf("expected 1 type arg, got %d", len(got.TypeArgs))
	}
	if got.TypeArgs[0].Name != "Int" {
		t.Fatalf("expected type arg Int, got %q", got.TypeArgs[0].Name)
	}
}

// TestGenericTypeArg_MapOfCall_RetainsTypeArguments verifies that the flat call
// resolver preserves both explicit type arguments on mapOf<String, Int>().
func TestGenericTypeArg_MapOfCall_RetainsTypeArguments(t *testing.T) {
	src := `
val values = mapOf<String, Int>()
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "call_expression", "mapOf<String, Int>()")
	if idx == 0 {
		t.Fatal("expected to find call_expression for mapOf<String, Int>()")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for mapOf<String, Int>(), got nil")
	}
	if got.Name != "Map" {
		t.Fatalf("expected Map, got %q", got.Name)
	}
	if len(got.TypeArgs) != 2 {
		t.Fatalf("expected 2 type args, got %d", len(got.TypeArgs))
	}
	if got.TypeArgs[0].Name != "String" || got.TypeArgs[1].Name != "Int" {
		t.Fatalf("expected type args String, Int; got %q, %q", got.TypeArgs[0].Name, got.TypeArgs[1].Name)
	}
}

// buildTestResolver creates a fully-merged resolver from a single file.
func buildTestResolver(t *testing.T, file *scanner.File) *defaultResolver {
	t.Helper()
	resolver := NewResolver()
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}
	// Merge
	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for _, ci := range fi.Classes {
		resolver.classes[ci.Name] = ci
		if ci.FQN != "" {
			resolver.classFQN[ci.FQN] = ci
		}
	}
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}
	for name, targetType := range fi.TypeAliases {
		resolver.typeAliases[name] = targetType
	}
	return resolver
}
