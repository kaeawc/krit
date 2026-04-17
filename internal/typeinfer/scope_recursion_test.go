package typeinfer

import (
	"strings"
	"testing"
)

func TestScopePopulation_NestedRecursiveScopes(t *testing.T) {
	src := `
fun outer(param: String) {
    val outerValue = param.length

    class Local {
        val classFlag: Boolean = true

        fun inner(innerParam: Int) {
            val items = listOf("a")
            for (item in items) {
                val forValue = item.length
                if (item.isNotEmpty()) {
                    val ifValue = item.length
                    val subject: Any = "hello"
                    when (subject) {
                        is String -> {
                            val branchValue = subject.length
                            listOf("x").map { value -> value.length }
                        }
                    }
                }
            }
        }
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	outerScope := fi.RootScope.FindScopeAtOffset(uint32(strings.Index(src, "outerValue")))
	if outerScope == nil {
		t.Fatal("expected outer function scope")
	}
	if got := outerScope.Lookup("param"); got == nil || got.Name != "String" {
		t.Fatalf("expected outer function param String, got %#v", got)
	}

	classScope := fi.RootScope.FindScopeAtOffset(uint32(strings.Index(src, "classFlag")))
	if classScope == nil {
		t.Fatal("expected class_body scope")
	}
	if classScope.Parent != outerScope {
		t.Fatalf("expected class scope parent to be outer function scope, outer=%p class=%p parent=%p", outerScope, classScope, classScope.Parent)
	}
	if got := classScope.Lookup("this"); got == nil || got.Name != "Local" {
		t.Fatalf("expected class scope this Local, got %#v", got)
	}

	innerScope := fi.RootScope.FindScopeAtOffset(uint32(strings.Index(src, "innerParam")))
	if innerScope == nil {
		t.Fatal("expected inner function scope")
	}
	if innerScope.Parent != classScope {
		t.Fatalf("expected inner function scope parent to be class scope, class=%p inner=%p parent=%p", classScope, innerScope, innerScope.Parent)
	}
	if got := innerScope.Lookup("innerParam"); got == nil || got.Name != "Int" {
		t.Fatalf("expected inner function param Int, got %#v", got)
	}

	forScope := fi.RootScope.FindScopeAtOffset(uint32(strings.Index(src, "forValue")))
	if forScope == nil {
		t.Fatal("expected for-loop scope")
	}
	if forScope.Parent != innerScope {
		t.Fatalf("expected for-loop scope parent to be inner function scope, inner=%p for=%p parent=%p", innerScope, forScope, forScope.Parent)
	}
	if got := forScope.Lookup("item"); got == nil {
		t.Fatalf("expected for-loop item to be declared, got %#v", got)
	}

	ifScope := fi.RootScope.FindScopeAtOffset(uint32(strings.Index(src, "ifValue")))
	if ifScope == nil {
		t.Fatal("expected if-body scope")
	}
	if got := ifScope.Lookup("item"); got == nil {
		t.Fatalf("expected if-body region to see item declared, got %#v", got)
	}

	whenScope := fi.RootScope.FindScopeAtOffset(uint32(strings.Index(src, "branchValue")))
	if whenScope == nil {
		t.Fatal("expected when-branch scope")
	}
	if whenScope.Parent != forScope {
		t.Fatalf("expected when-branch scope parent to be for-loop scope, for=%p when=%p parent=%p", forScope, whenScope, whenScope.Parent)
	}
	if got := whenScope.Lookup("subject"); got == nil || got.Name != "String" {
		t.Fatalf("expected when branch to smart-cast subject to String, got %#v", got)
	}

	lambdaScope := fi.RootScope.FindScopeAtOffset(uint32(strings.Index(src, "value.length")))
	if lambdaScope == nil {
		t.Fatal("expected lambda scope")
	}
	if lambdaScope.Parent != whenScope {
		t.Fatalf("expected lambda scope parent to be when-branch scope, when=%p lambda=%p parent=%p", whenScope, lambdaScope, lambdaScope.Parent)
	}
	if got := lambdaScope.Lookup("value"); got == nil {
		t.Fatalf("expected lambda param value to be declared, got %#v", got)
	}
}
