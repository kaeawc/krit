package typeinfer

import (
	"testing"
)

func TestAsCast_UnsafeString(t *testing.T) {
	src := `
val x: Any = "hello"
val s = x as String
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "as_expression", "x as String")
	if idx == 0 {
		t.Fatal("expected to find as_expression for 'x as String'")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
	if got.Nullable {
		t.Error("expected non-nullable for unsafe cast")
	}
}

func TestAsCast_SafeString(t *testing.T) {
	src := `
val x: Any = "hello"
val s = x as? String
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "as_expression", "x as? String")
	if idx == 0 {
		t.Fatal("expected to find as_expression for 'x as? String'")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
	if !got.Nullable {
		t.Error("expected nullable for safe cast")
	}
}

func TestAsCast_UnsafeInt(t *testing.T) {
	src := `
val x: Any = 42
val n = x as Int
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "as_expression", "x as Int")
	if idx == 0 {
		t.Fatal("expected to find as_expression for 'x as Int'")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got.Name != "Int" {
		t.Errorf("expected Int, got %q", got.Name)
	}
	if got.Nullable {
		t.Error("expected non-nullable for unsafe cast")
	}
}

func TestAsCast_SafeList(t *testing.T) {
	src := `
val x: Any = listOf("a")
val items = x as? List<String>
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "as_expression", "x as? List<String>")
	if idx == 0 {
		t.Fatal("expected to find as_expression for 'x as? List<String>'")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got.Name != "List" {
		t.Errorf("expected List, got %q", got.Name)
	}
	if !got.Nullable {
		t.Error("expected nullable for safe cast")
	}
	if len(got.TypeArgs) != 1 {
		t.Fatalf("expected 1 type arg, got %d", len(got.TypeArgs))
	}
	if got.TypeArgs[0].Name != "String" {
		t.Errorf("expected type arg String, got %q", got.TypeArgs[0].Name)
	}
}
