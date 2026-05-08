package typeinfer

import (
	"testing"
)

func TestStringTemplate_SimpleInterpolation(t *testing.T) {
	src := `
val name = "world"
val greeting = "Hello ${name}!"
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "string_literal", `"Hello ${name}!"`)
	if idx == 0 {
		t.Fatal("expected to find string_literal for interpolated string")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for string template, got nil")
	}
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
}

func TestStringTemplate_MultilineInterpolation(t *testing.T) {
	src := "val items = listOf(\"a\")\nval multiline = \"\"\"\n    Count: ${items.size}\n\"\"\"\n"
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	// Find the multiline string literal (triple-quoted).
	var idx uint32
	file.FlatWalkAllNodes(0, func(i uint32) {
		if idx == 0 && file.FlatType(i) == "string_literal" {
			text := file.FlatNodeText(i)
			if len(text) > 6 && text[:3] == `"""` {
				idx = i
			}
		}
	})
	if idx == 0 {
		t.Fatal("expected to find string_literal for multiline string template")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for multiline string template, got nil")
	}
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
}

func TestStringTemplate_SimpleString(t *testing.T) {
	src := `val simple = "hello"
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "string_literal", `"hello"`)
	if idx == 0 {
		t.Fatal("expected to find string_literal for simple string")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for simple string, got nil")
	}
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
}

func TestStringTemplate_EmptyString(t *testing.T) {
	src := `val empty = ""
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "string_literal", `""`)
	if idx == 0 {
		t.Fatal("expected to find string_literal for empty string")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for empty string, got nil")
	}
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
}

func TestStringTemplate_NestedInterpolation(t *testing.T) {
	src := `
val x = 42
val msg = "value is ${x.toString()}"
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "string_literal", `"value is ${x.toString()}"`)
	if idx == 0 {
		t.Fatal("expected to find string_literal for nested interpolation")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for string with nested interpolation, got nil")
	}
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
}
