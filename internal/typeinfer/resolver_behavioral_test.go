package typeinfer

import "testing"

// The tests in this file replace the node-vs-flat parity oracles that were
// deleted during the 2026-04-14 retirement of the node-era public API
// (see: flat-tree-migration/README.md, Track B). Each test exercises a
// specific scenario the old parity oracle verified, but asserts the
// flat-native result directly. If the flat impl silently diverges from its
// historical node-era twin, these tests should catch it.

// TestResolveFlatNode_GenericPropagationThroughCallReceiver replaces the
// `TestDefaultResolver_CallAndNavigation_NodeAndFlatAgree` parity oracle for
// the call-expression arm. Verifies that `provideItems().firstOrNull()`
// correctly propagates the `List<String>` return type's element type as
// nullable String, both when the receiver is a property and when it's itself
// a call expression.
func TestResolveFlatNode_GenericPropagationThroughCallReceiver(t *testing.T) {
	src := `
package com.example

fun provideItems(): List<String> = listOf()

fun main() {
    val items = provideItems().firstOrNull()
}
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "call_expression", "provideItems().firstOrNull()")
	if idx == 0 {
		t.Fatal("expected to find provideItems().firstOrNull() call_expression")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if got.Name != "String" {
		t.Errorf("expected String, got %q", got.Name)
	}
	if !got.Nullable {
		t.Error("expected nullable for firstOrNull result")
	}
}

// TestResolveFlatNode_CompanionNavigationResolution replaces the
// navigation-expression arm of the deleted parity oracle. Verifies that
// `AppConfig.instance` resolves to AppConfig via companion object lookup.
func TestResolveFlatNode_CompanionNavigationResolution(t *testing.T) {
	src := `
package com.example

class AppConfig {
    companion object {
        val instance: AppConfig = AppConfig()
    }
}

fun main() {
    val cfg = AppConfig.instance
}
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	idx := flatFirstOfTypeWithText(file, "navigation_expression", "AppConfig.instance")
	if idx == 0 {
		t.Fatal("expected to find AppConfig.instance navigation_expression")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if got.Name != "AppConfig" {
		t.Errorf("expected AppConfig, got %q", got.Name)
	}
}

// Note: ResolveByNameFlat scope-offset behavior is already covered by
// `TestDefaultResolver_ResolveByNameFlat_UsesScopeOffset` in resolver_test.go,
// which builds a manual inner scope and verifies the method walks the
// FindScopeAtOffset chain correctly. The deleted
// `TestDefaultResolver_ResolveByName_NodeAndFlatAgree` parity oracle only
// verified that the node and flat variants agreed with each other — it did
// not independently guarantee any scope-shadowing behavior beyond what the
// offset-based test already verifies. No additional replacement test is
// needed here.

// TestIsNullableFlat_LocalNullableDeclaration replaces the IsNullable arm of
// the deleted parity oracle. Verifies that a `val local: String? = null`
// reference resolves as nullable.
func TestIsNullableFlat_LocalNullableDeclaration(t *testing.T) {
	src := `
fun example() {
    val local: String? = null
    val value = local
}
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	// Find the `local` identifier in `val value = local` (the rhs reference,
	// not the declaration). Grab the 2nd "local" simple_identifier — the 1st
	// is the variable_declaration name.
	var refIdx uint32
	var count int
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if refIdx != 0 {
			return
		}
		if file.FlatType(idx) != "simple_identifier" {
			return
		}
		if file.FlatNodeText(idx) != "local" {
			return
		}
		count++
		if count == 2 {
			refIdx = idx
		}
	})
	if refIdx == 0 {
		t.Fatal("expected to find the rhs `local` reference")
	}

	got := resolver.IsNullableFlat(refIdx, file)
	if got == nil {
		t.Fatal("expected IsNullableFlat to return a result, got nil")
	}
	if !*got {
		t.Error("expected `local` reference to be nullable (declared String?)")
	}
}

func TestIsNullableFlat_InferredNullableIfExpression(t *testing.T) {
	src := `
fun example(flag: Boolean) {
    val local = if (flag) "ok" else null
    val value = local
}
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	var refIdx uint32
	var count int
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if refIdx != 0 || file.FlatType(idx) != "simple_identifier" || file.FlatNodeText(idx) != "local" {
			return
		}
		count++
		if count == 2 {
			refIdx = idx
		}
	})
	if refIdx == 0 {
		t.Fatal("expected to find the rhs `local` reference")
	}

	got := resolver.IsNullableFlat(refIdx, file)
	if got == nil {
		t.Fatal("expected IsNullableFlat to return a result, got nil")
	}
	if !*got {
		t.Error("expected `local` reference inferred from if/else null to be nullable")
	}
}

func TestIsNullableFlat_LocalDeclarationShadowsParent(t *testing.T) {
	src := `
fun example(input: String?) {
    run {
        val input: String = "local"
        val value = input
    }
}
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	var refIdx uint32
	var count int
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if refIdx != 0 || file.FlatType(idx) != "simple_identifier" || file.FlatNodeText(idx) != "input" {
			return
		}
		count++
		if count == 3 {
			refIdx = idx
		}
	})
	if refIdx == 0 {
		t.Fatal("expected to find the shadowed `input` reference")
	}

	got := resolver.IsNullableFlat(refIdx, file)
	if got == nil {
		t.Fatal("expected IsNullableFlat to return a result, got nil")
	}
	if *got {
		t.Error("expected shadowing local `input` reference to be non-null")
	}
}

func TestResolveFlatNode_TypeAliasCarriesNullableTarget(t *testing.T) {
	src := `
typealias NullableName = String?

fun example(input: String?) {
    val value = input as NullableName
}
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)

	typeIdx := flatFirstOfTypeWithText(file, "user_type", "NullableName")
	if typeIdx == 0 {
		t.Fatal("expected to find NullableName user_type")
	}

	got := resolver.ResolveFlatNode(typeIdx, file)
	if got == nil {
		t.Fatal("expected resolved alias target, got nil")
	}
	if got.Name != "String" {
		t.Errorf("expected alias target String, got %q", got.Name)
	}
	if !got.IsNullable() {
		t.Error("expected alias target to be nullable")
	}
}

// TestInferLambdaLastExpressionFlat_LazyWithCallExpression replaces
// `TestPropertyInference_LazyDelegate_NodeVsFlatLambdaLastExpression`.
// Verifies that `val answer by lazy { ... provideLabel() }` infers `answer`'s
// type from the lambda's last expression — a call to a declared function —
// via the lazy delegate pattern.
func TestInferLambdaLastExpressionFlat_LazyWithCallExpression(t *testing.T) {
	src := `
fun provideLabel(): String = "hello"

val answer by lazy {
    val prefix = "ignored"
    provideLabel()
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	answer := fi.RootScope.Lookup("answer")
	if answer == nil {
		t.Fatal("expected `answer` to be declared via lazy delegate")
	}
	if answer.Name != "String" {
		t.Errorf("expected `answer` type String (from provideLabel() last expression), got %q", answer.Name)
	}
}

// TestInferLambdaLastExpressionFlat_RememberWithNavigationExpression replaces
// `TestPropertyInference_RememberDelegate_NodeVsFlatLambdaLastExpression`.
// Verifies `val answer by remember { val local = "hello"; local.length }`
// infers Int from the navigation-expression last value, using a lambda-local
// `val` declaration that must be in scope when the last expression is typed.
func TestInferLambdaLastExpressionFlat_RememberWithNavigationExpression(t *testing.T) {
	src := `
val answer by remember {
    val local = "hello"
    local.length
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	answer := fi.RootScope.Lookup("answer")
	if answer == nil {
		t.Fatal("expected `answer` to be declared via remember delegate")
	}
	if answer.Name != "Int" {
		t.Errorf("expected `answer` type Int (from local.length last expression), got %q", answer.Name)
	}
}
