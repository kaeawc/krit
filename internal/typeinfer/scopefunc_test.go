package typeinfer

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// flatFirstCallContaining returns the index of the first call_expression whose
// text contains the given substring.
func flatFirstCallContaining(file *scanner.File, substr string) uint32 {
	var found uint32
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found == 0 && file.FlatType(idx) == "call_expression" &&
			strings.Contains(file.FlatNodeText(idx), substr) {
			found = idx
		}
	})
	return found
}

// flatLongestCallContaining returns the index of the longest call_expression
// whose text contains the given substring. Used to pick the outermost call
// in a chain like `a.b { }.c { }`.
func flatLongestCallContaining(file *scanner.File, substr string) uint32 {
	var found uint32
	var foundLen int
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if file.FlatType(idx) != "call_expression" {
			return
		}
		text := file.FlatNodeText(idx)
		if !strings.Contains(text, substr) {
			return
		}
		if found == 0 || len(text) > foundLen {
			found = idx
			foundLen = len(text)
		}
	})
	return found
}

// TestScopeFunc_AlsoReturnsReceiver verifies that .also { } returns the receiver type.
func TestScopeFunc_AlsoReturnsReceiver(t *testing.T) {
	src := `
package com.example

fun main() {
    val result = listOf("a", "b").also { println(it) }
}
`
	file := parseTestFile(t, src)
	resolver := NewResolver()
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}

	idx := flatFirstCallContaining(file, "also")
	if idx == 0 {
		t.Fatal("expected to find the also call_expression")
	}

	resolved := resolver.ResolveFlatNode(idx, file)
	if resolved == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if resolved.Name != "List" {
		t.Errorf("expected also to return List (receiver type), got %q", resolved.Name)
	}
}

// TestScopeFunc_ApplyReturnsReceiver verifies that .apply { } returns the receiver type.
func TestScopeFunc_ApplyReturnsReceiver(t *testing.T) {
	src := `
package com.example

fun main() {
    val result = "hello".apply { }
}
`
	file := parseTestFile(t, src)
	resolver := NewResolver()
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}

	idx := flatFirstCallContaining(file, "apply")
	if idx == 0 {
		t.Fatal("expected to find the apply call_expression")
	}

	resolved := resolver.ResolveFlatNode(idx, file)
	if resolved == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if resolved.Name != "String" {
		t.Errorf("expected apply to return String (receiver type), got %q", resolved.Name)
	}
}

// TestScopeFunc_LetReturnsUnknown verifies that .let { } returns unknown (lambda result).
func TestScopeFunc_LetReturnsUnknown(t *testing.T) {
	src := `
package com.example

fun main() {
    val result = "hello".let { it.length }
}
`
	file := parseTestFile(t, src)
	resolver := NewResolver()
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}

	idx := flatFirstCallContaining(file, "let")
	if idx == 0 {
		t.Fatal("expected to find the let call_expression")
	}

	resolved := resolver.ResolveFlatNode(idx, file)
	if resolved == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if resolved.Kind != TypeUnknown {
		t.Errorf("expected let to return Unknown (lambda result), got %v (%q)", resolved.Kind, resolved.Name)
	}
}

// TestScopeFunc_AlsoOnInt verifies that 42.also { } returns Int.
func TestScopeFunc_AlsoOnInt(t *testing.T) {
	src := `
package com.example

fun main() {
    val x: Int = 42
    val result = x.also { println(it) }
}
`
	file := parseTestFile(t, src)
	resolver := NewResolver()
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}

	idx := flatFirstCallContaining(file, "also")
	if idx == 0 {
		t.Fatal("expected to find the also call_expression")
	}

	resolved := resolver.ResolveFlatNode(idx, file)
	if resolved == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if resolved.Name != "Int" {
		t.Errorf("expected also on Int to return Int, got %q", resolved.Name)
	}
}

// TestScopeFunc_WithReturnsUnknown verifies that with(obj) { } returns unknown.
func TestScopeFunc_WithReturnsUnknown(t *testing.T) {
	m := LookupStdlibMethod("_", "with")
	if m == nil {
		t.Fatal("expected stdlib entry for _.with")
	}
	if m.ReturnType.Kind != TypeUnknown {
		t.Errorf("expected with to return Unknown, got %v", m.ReturnType.Kind)
	}
}

// TestScopeFunc_RunReturnsUnknown verifies that .run { } returns unknown.
func TestScopeFunc_RunReturnsUnknown(t *testing.T) {
	m := LookupStdlibMethod("_", "run")
	if m == nil {
		t.Fatal("expected stdlib entry for _.run")
	}
	if m.ReturnType.Kind != TypeUnknown {
		t.Errorf("expected run to return Unknown, got %v", m.ReturnType.Kind)
	}
}

// TestScopeFunc_ApplyChain verifies chaining: listOf(1).also { }.apply { }.
func TestScopeFunc_ApplyChain(t *testing.T) {
	src := `
package com.example

fun main() {
    val result = listOf(1, 2, 3).also { println(it) }.apply { }
}
`
	file := parseTestFile(t, src)
	resolver := NewResolver()
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}

	// Pick the outermost (longest) call_expression that contains "apply".
	idx := flatLongestCallContaining(file, "apply")
	if idx == 0 {
		t.Fatal("expected to find the chained apply call_expression")
	}

	resolved := resolver.ResolveFlatNode(idx, file)
	if resolved == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if resolved.Name != "List" {
		t.Errorf("expected chained apply to return List, got %q", resolved.Name)
	}
}
