package typeinfer

import (
	"testing"
)

func TestFunctionReturnType_ExplicitAnnotation(t *testing.T) {
	src := `
package com.example

class User(val name: String)

fun getUser(): User {
    return User("Alice")
}

fun main() {
    val u = getUser()
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	retType, ok := fi.Functions["getUser"]
	if !ok {
		t.Fatal("expected getUser to be indexed in functions map")
	}
	if retType.Name != "User" {
		t.Errorf("expected return type User, got %q", retType.Name)
	}
	if retType.Kind == TypeUnknown {
		t.Error("expected non-unknown type kind")
	}
}

func TestFunctionReturnType_PrimitiveInt(t *testing.T) {
	src := `
fun calculate(): Int {
    return 42
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	retType, ok := fi.Functions["calculate"]
	if !ok {
		t.Fatal("expected calculate to be indexed in functions map")
	}
	if retType.Name != "Int" {
		t.Errorf("expected return type Int, got %q", retType.Name)
	}
	if retType.Kind != TypePrimitive {
		t.Errorf("expected TypePrimitive, got %v", retType.Kind)
	}
}

func TestFunctionReturnType_NoAnnotation(t *testing.T) {
	src := `
fun doSomething() {
    println("hello")
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if _, ok := fi.Functions["doSomething"]; ok {
		t.Error("expected doSomething NOT to be in functions map (no return type annotation)")
	}
}

func TestFunctionReturnType_ExpressionBody(t *testing.T) {
	src := `
fun name() = "hello"
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if _, ok := fi.Functions["name"]; ok {
		t.Error("expected name NOT to be in functions map (no explicit return type)")
	}
}

func TestFunctionReturnType_ExpressionBodyWithType(t *testing.T) {
	src := `
fun name(): String = "hello"
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	retType, ok := fi.Functions["name"]
	if !ok {
		t.Fatal("expected name to be indexed in functions map")
	}
	if retType.Name != "String" {
		t.Errorf("expected return type String, got %q", retType.Name)
	}
}

func TestFunctionReturnType_NullableReturn(t *testing.T) {
	src := `
fun findUser(): User? {
    return null
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	retType, ok := fi.Functions["findUser"]
	if !ok {
		t.Fatal("expected findUser to be indexed in functions map")
	}
	if !retType.IsNullable() {
		t.Error("expected nullable return type")
	}
}

func TestFunctionReturnType_CallResolution(t *testing.T) {
	src := `
package com.example

fun calculate(): Int {
    return 42
}

fun main() {
    val result = calculate()
}
`
	file := parseTestFile(t, src)
	resolver := NewResolver()
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	// Simulate merge
	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}

	// Find the call_expression for calculate()
	idx := flatFirstOfTypeWithText(file, "call_expression", "calculate()")
	if idx == 0 {
		t.Fatal("expected to find call_expression for calculate()")
	}

	resolved := resolver.ResolveFlatNode(idx, file)
	if resolved == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if resolved.Name != "Int" {
		t.Errorf("expected call to resolve to Int, got %q", resolved.Name)
	}
}

func TestFunctionReturnType_WithPackage(t *testing.T) {
	src := `
package com.example

fun getCount(): Int {
    return 10
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if _, ok := fi.Functions["getCount"]; !ok {
		t.Error("expected getCount to be indexed by simple name")
	}
	if _, ok := fi.Functions["com.example.getCount"]; !ok {
		t.Error("expected getCount to be indexed by FQN")
	}
}
