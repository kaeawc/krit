package typeinfer

import (
	"testing"
)

func TestSmartCast_IsCheckPositive(t *testing.T) {
	src := `
fun example(x: Any) {
    if (x is String) {
        val len = x.length
    }
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

	// x should be declared as Any in the function scope
	rawType := funcScope.lookupRaw("x")
	if rawType == nil {
		t.Fatal("expected x to be declared in function scope")
	}
	if rawType.Name != "Any" {
		t.Errorf("expected raw x to be Any, got %q", rawType.Name)
	}

	// Find the child scope that has SmartCastTypes for x
	var ifBodyScope *ScopeTable
	for _, child := range funcScope.Children {
		if child.SmartCastTypes != nil {
			if _, ok := child.SmartCastTypes["x"]; ok {
				ifBodyScope = child
				break
			}
		}
	}
	if ifBodyScope == nil {
		t.Fatal("expected a child scope with smart cast type for x")
	}

	// Inside the if-body scope, x should resolve to String
	got := ifBodyScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in if-body scope")
	}
	if got.Name != "String" {
		t.Errorf("expected x to be String inside if (x is String), got %q", got.Name)
	}

	// Outside the if body, x should still be Any
	got = funcScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in function scope")
	}
	if got.Name != "Any" {
		t.Errorf("expected x to still be Any outside if body, got %q", got.Name)
	}
}

func TestSmartCast_IsCheckNegativeEarlyReturn(t *testing.T) {
	src := `
fun example(x: Any) {
    if (x !is String) return
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

	// After `if (x !is String) return`, x should be narrowed to String in the function scope
	got := funcScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in function scope")
	}
	if got.Name != "String" {
		t.Errorf("expected x to be String after negated is-check with early return, got %q", got.Name)
	}
}

func TestSmartCast_IsCheckNotNarrowedOutside(t *testing.T) {
	// x should NOT be narrowed to String outside the if body when the check is positive
	src := `
fun example(x: Any) {
    if (x is String) {
        val len = x.length
    }
    val y = x.toString()
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

	// In the function scope (outside if), x should still be Any
	got := funcScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in function scope")
	}
	if got.Name != "Any" {
		t.Errorf("expected x to still be Any outside the if body, got %q", got.Name)
	}
}

func TestSmartCast_IsCheckScopeTable(t *testing.T) {
	// Direct scope table test: SmartCastTypes should narrow variable types
	parent := &ScopeTable{
		Entries:        make(map[string]*ResolvedType),
		SmartCasts:     make(map[string]bool),
		SmartCastTypes: make(map[string]*ResolvedType),
	}
	parent.Declare("x", &ResolvedType{Name: "Any", FQN: "kotlin.Any", Kind: TypePrimitive})

	child := parent.NewScope()
	child.SmartCastTypes["x"] = &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypePrimitive}

	// In parent scope, x should be Any
	got := parent.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in parent")
	}
	if got.Name != "Any" {
		t.Errorf("expected x to be Any in parent scope, got %q", got.Name)
	}

	// In child scope with smart cast type, x should be String
	got = child.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in child")
	}
	if got.Name != "String" {
		t.Errorf("expected x to be String in smart-cast child scope, got %q", got.Name)
	}
}

func TestSmartCast_IsCheckWithCustomType(t *testing.T) {
	src := `
import com.example.Foo

fun example(x: Any) {
    if (x is Foo) {
        val f = x
    }
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

	// Find the child scope that has SmartCastTypes for x
	var ifBodyScope *ScopeTable
	for _, child := range funcScope.Children {
		if child.SmartCastTypes != nil {
			if _, ok := child.SmartCastTypes["x"]; ok {
				ifBodyScope = child
				break
			}
		}
	}
	if ifBodyScope == nil {
		t.Fatal("expected a child scope with smart cast type for x")
	}

	got := ifBodyScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in if-body scope")
	}
	if got.Name != "Foo" {
		t.Errorf("expected x to be Foo inside if (x is Foo), got %q", got.Name)
	}
}

func TestSmartCast_WhenIsCheckBranch(t *testing.T) {
	src := `
fun example(x: Any) {
    when (x) {
        is String -> {
            val len = x.length
        }
    }
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

	var whenBodyScope *ScopeTable
	for _, child := range funcScope.Children {
		if typ := child.SmartCastTypes["x"]; typ != nil {
			whenBodyScope = child
			break
		}
	}
	if whenBodyScope == nil {
		t.Fatal("expected a child scope with smart cast type for x in when branch")
	}

	got := whenBodyScope.Lookup("x")
	if got == nil {
		t.Fatal("expected type for x in when branch scope")
	}
	if got.Name != "String" {
		t.Errorf("expected x to be String inside when (x) { is String -> ... }, got %q", got.Name)
	}
}
