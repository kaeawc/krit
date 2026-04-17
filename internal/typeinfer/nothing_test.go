package typeinfer

import (
	"testing"
)

func TestNothing_TODO_ResolvesToNothing(t *testing.T) {
	src := `
fun example() {
    val x = TODO()
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	// Check that TODO() is registered as a top-level stdlib method returning Nothing
	m := LookupStdlibMethod("_", "TODO")
	if m == nil {
		t.Fatal("expected stdlib entry for TODO")
	}
	if m.ReturnType.Kind != TypeNothing {
		t.Errorf("expected TODO() return type kind TypeNothing, got %v", m.ReturnType.Kind)
	}
	if m.ReturnType.Name != "Nothing" {
		t.Errorf("expected TODO() return type name Nothing, got %q", m.ReturnType.Name)
	}
	if m.ReturnType.FQN != "kotlin.Nothing" {
		t.Errorf("expected TODO() return type FQN kotlin.Nothing, got %q", m.ReturnType.FQN)
	}
}

func TestNothing_Error_ResolvesToNothing(t *testing.T) {
	src := `
fun example() {
    val x = error("something went wrong")
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	m := LookupStdlibMethod("_", "error")
	if m == nil {
		t.Fatal("expected stdlib entry for error")
	}
	if m.ReturnType.Kind != TypeNothing {
		t.Errorf("expected error() return type kind TypeNothing, got %v", m.ReturnType.Kind)
	}
	if m.ReturnType.Name != "Nothing" {
		t.Errorf("expected error() return type name Nothing, got %q", m.ReturnType.Name)
	}
}

func TestNothing_ThrowExpression_ResolvesToNothing(t *testing.T) {
	src := `
fun example() {
    throw IllegalArgumentException("bad")
}
`
	file := parseTestFile(t, src)

	tmp := &defaultResolver{
		imports:        make(map[string]*ImportTable),
		scopes:         make(map[string]*ScopeTable),
		classes:        make(map[string]*ClassInfo),
		classFQN:       make(map[string]*ClassInfo),
		sealedVariants: make(map[string][]string),
		enumEntries:    make(map[string][]string),
		functions:      make(map[string]*ResolvedType),
	}
	it := buildImportTableFlat(0, file)

	var found bool
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found {
			return
		}
		if file.FlatType(idx) == "jump_expression" {
			typ := tmp.inferExpressionTypeFlat(idx, file, it)
			if typ.Kind != TypeNothing {
				t.Errorf("expected throw expression to resolve to TypeNothing, got %v", typ.Kind)
			}
			if typ.Name != "Nothing" {
				t.Errorf("expected throw expression type name Nothing, got %q", typ.Name)
			}
			found = true
		}
	})

	if !found {
		t.Error("did not find a jump_expression node for throw")
	}
}

func TestNothing_ReturnExpression_ResolvesToNothing(t *testing.T) {
	src := `
fun example(): Int {
    return 42
}
`
	file := parseTestFile(t, src)

	tmp := &defaultResolver{
		imports:        make(map[string]*ImportTable),
		scopes:         make(map[string]*ScopeTable),
		classes:        make(map[string]*ClassInfo),
		classFQN:       make(map[string]*ClassInfo),
		sealedVariants: make(map[string][]string),
		enumEntries:    make(map[string][]string),
		functions:      make(map[string]*ResolvedType),
	}
	it := buildImportTableFlat(0, file)

	var found bool
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found {
			return
		}
		if file.FlatType(idx) == "jump_expression" {
			typ := tmp.inferExpressionTypeFlat(idx, file, it)
			if typ.Kind != TypeNothing {
				t.Errorf("expected return expression to resolve to TypeNothing, got %v", typ.Kind)
			}
			found = true
		}
	})

	if !found {
		t.Error("did not find a jump_expression node for return")
	}
}

func TestNothing_ElvisWithTODO_SmartCast(t *testing.T) {
	src := `
fun example(x: String?) {
    x ?: TODO()
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

	// After `x ?: TODO()`, x should be smart-cast non-null
	if !funcScope.IsSmartCastNonNull("x") {
		t.Error("expected x to be smart-cast non-null after elvis with TODO()")
	}

	// Lookup should return non-null version
	got := funcScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x")
	}
	if got.IsNullable() {
		t.Error("expected x to be non-null after elvis with TODO()")
	}
}

func TestNothing_ElvisWithError_SmartCast(t *testing.T) {
	src := `
fun example(y: String?) {
    y ?: error("y must not be null")
    val len = y.length
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

	// After `y ?: error(...)`, y should be smart-cast non-null
	if !funcScope.IsSmartCastNonNull("y") {
		t.Error("expected y to be smart-cast non-null after elvis with error()")
	}
}
