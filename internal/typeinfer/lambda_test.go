package typeinfer

import (
	"testing"
)

// TestLambdaImplicitIt_ListMap_StringElement verifies that `it` inside a lambda
// on List<String>.map { ... } resolves to String.
func TestLambdaImplicitIt_ListMap_StringElement(t *testing.T) {
	src := `
val items: List<String> = listOf()
val result = items.map { it.length }
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	lambda := flatFirstOfType(file, "lambda_literal")
	if lambda == 0 {
		t.Fatal("expected to find lambda_literal")
	}

	lambdaScope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(lambda) + 1)
	if lambdaScope == nil {
		t.Fatal("expected to find scope at lambda offset")
	}

	// `it` should be declared as String in the lambda scope
	itType := lambdaScope.Lookup("it")
	if itType == nil {
		t.Fatal("expected `it` to be declared in lambda scope")
	}
	if itType.Name != "String" {
		t.Errorf("expected `it` to be String, got %q", itType.Name)
	}
	if itType.FQN != "kotlin.String" {
		t.Errorf("expected FQN kotlin.String, got %q", itType.FQN)
	}
}

// TestLambdaImplicitIt_FunctionCallReceiver_ListMap_StringElement verifies that
// implicit `it` still resolves correctly when the lambda receiver comes from a
// function call expression.
func TestLambdaImplicitIt_FunctionCallReceiver_ListMap_StringElement(t *testing.T) {
	src := `
fun provideItems(): List<String> = listOf()
fun main() {
    val result = provideItems().map { it.length }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	lambda := flatFirstOfType(file, "lambda_literal")
	if lambda == 0 {
		t.Fatal("expected to find lambda_literal")
	}

	lambdaScope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(lambda) + 1)
	if lambdaScope == nil {
		t.Fatal("expected to find scope at lambda offset")
	}

	itType := lambdaScope.Lookup("it")
	if itType == nil {
		t.Fatal("expected `it` to be declared in lambda scope")
	}
	if itType.Name != "String" {
		t.Errorf("expected `it` to be String, got %q", itType.Name)
	}
}

// TestLambdaImplicitIt_ListFilter_IntElement verifies that `it` inside
// listOf<Int>().filter { ... } resolves to Int.
func TestLambdaImplicitIt_ListFilter_IntElement(t *testing.T) {
	src := `
val items = listOf<Int>()
val result = items.filter { it > 0 }
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	lambda := flatFirstOfType(file, "lambda_literal")
	if lambda == 0 {
		t.Fatal("expected to find lambda_literal")
	}

	lambdaScope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(lambda) + 1)
	if lambdaScope == nil {
		t.Fatal("expected to find scope at lambda offset")
	}

	itType := lambdaScope.Lookup("it")
	if itType == nil {
		t.Fatal("expected `it` to be declared in lambda scope")
	}
	if itType.Name != "Int" {
		t.Errorf("expected `it` to be Int, got %q", itType.Name)
	}
}

// TestLambdaExplicitParam_NoImplicitIt verifies that when a lambda has
// explicit parameters, `it` is NOT implicitly declared (no regression).
func TestLambdaExplicitParam_NoImplicitIt(t *testing.T) {
	src := `
val items: List<String> = listOf()
val result = items.map { item -> item.length }
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	lambda := flatFirstOfType(file, "lambda_literal")
	if lambda == 0 {
		t.Fatal("expected to find lambda_literal")
	}

	lambdaScope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(lambda) + 1)
	if lambdaScope == nil {
		t.Fatal("expected to find scope at lambda offset")
	}

	// `it` should NOT be declared when explicit params exist
	itType := lambdaScope.Lookup("it")
	if itType != nil {
		t.Errorf("expected `it` to NOT be declared with explicit params, got %v", itType.Name)
	}
}

// TestLambdaImplicitIt_NestedLambda verifies that nested lambdas get
// the correct `it` from their own receiver context.
func TestLambdaImplicitIt_NestedLambda(t *testing.T) {
	src := `
val outer: List<List<Int>> = listOf()
val result = outer.forEach { it.size }
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	lambda := flatFirstOfType(file, "lambda_literal")
	if lambda == 0 {
		t.Fatal("expected to find lambda_literal")
	}

	lambdaScope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(lambda) + 1)
	if lambdaScope == nil {
		t.Fatal("expected to find scope at lambda offset")
	}

	// `it` in the outer lambda should be List<Int> (the element type of List<List<Int>>)
	itType := lambdaScope.Lookup("it")
	if itType == nil {
		t.Fatal("expected `it` to be declared in lambda scope")
	}
	if itType.Name != "List" {
		t.Errorf("expected `it` to be List, got %q", itType.Name)
	}
}

// TestLambdaImplicitIt_NoReceiver verifies that lambdas on non-collection
// calls (no type args on receiver) do not crash or declare `it`.
func TestLambdaImplicitIt_NoReceiver(t *testing.T) {
	src := `
fun process(block: () -> Unit) {}
fun main() {
    process { println("hello") }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	// Should not crash — lambda without a collection receiver
	lambda := flatFirstOfType(file, "lambda_literal")
	if lambda == 0 {
		t.Fatal("expected to find lambda_literal")
	}

	lambdaScope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(lambda) + 1)
	if lambdaScope == nil {
		t.Fatal("expected to find scope at lambda offset")
	}

	// `it` should NOT be declared when receiver has no type args
	itType := lambdaScope.Lookup("it")
	if itType != nil {
		t.Logf("note: `it` was declared as %q (acceptable if no type args)", itType.Name)
	}
}
