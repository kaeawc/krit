package typeinfer

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestScopePopulation_FunctionParams(t *testing.T) {
	src := `
fun greet(name: String, count: Int) {
    println(name)
    println(count)
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	body := flatFirstOfType(file, "function_body")
	if body == 0 {
		t.Fatal("expected to find function_body")
	}

	scope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(body) + 1)
	if scope == nil {
		t.Fatal("expected to find function scope")
	}

	nameType := scope.Lookup("name")
	if nameType == nil || nameType.Name != "String" {
		t.Fatalf("expected function param name String, got %#v", nameType)
	}
	countType := scope.Lookup("count")
	if countType == nil || countType.Name != "Int" {
		t.Fatalf("expected function param count Int, got %#v", countType)
	}
}

func TestScopePopulation_LambdaParams(t *testing.T) {
	src := `
val result = listOf("a", "bb").map { value: String -> value.length }
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

	scope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(lambda) + 1)
	if scope == nil {
		t.Fatal("expected to find lambda scope")
	}

	valueType := scope.Lookup("value")
	if valueType == nil {
		t.Fatal("expected lambda param value to be declared")
	}
	if valueType.Name != "String" {
		t.Fatalf("expected lambda param value String, got %#v", valueType)
	}
}

func TestScopePopulation_LocalProperty(t *testing.T) {
	src := `
fun main() {
    val message = "hello"
    println(message)
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	prop := flatFirstOfType(file, "property_declaration")
	if prop == 0 {
		t.Fatal("expected to find property_declaration")
	}

	scope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(prop) + 1)
	if scope == nil {
		t.Fatal("expected to find property scope")
	}

	messageType := scope.Lookup("message")
	if messageType == nil {
		t.Fatal("expected local property message to be declared")
	}
	if messageType.Name != "String" {
		t.Fatalf("expected local property message String, got %#v", messageType)
	}
}

func TestScopePopulation_ForLoopVar(t *testing.T) {
	src := `
fun main() {
    val items = listOf("a", "bb")
    for (item in items) {
        println(item.length)
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	forNode := flatFirstOfType(file, "for_statement")
	if forNode == 0 {
		t.Fatal("expected to find for_statement")
	}

	scope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(forNode) + 1)
	if scope == nil {
		t.Fatal("expected to find for-loop scope")
	}

	itemType := scope.Lookup("item")
	if itemType == nil {
		t.Fatal("expected for-loop var item to be declared")
	}
}

func TestScopePopulation_LocalDestructuring(t *testing.T) {
	src := `
fun main() {
    val (count, name) = Pair(42, "hello")
    println(count)
    println(name)
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	destructuring := flatFirstOfType(file, "multi_variable_declaration")
	if destructuring == 0 {
		t.Fatal("expected to find multi_variable_declaration")
	}

	scope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(destructuring) + 1)
	if scope == nil {
		t.Fatal("expected to find function scope for destructuring")
	}

	countType := scope.Lookup("count")
	if countType == nil || countType.Name != "Int" {
		t.Fatalf("expected destructured count Int, got %#v", countType)
	}
	nameType := scope.Lookup("name")
	if nameType == nil || nameType.Name != "String" {
		t.Fatalf("expected destructured name String, got %#v", nameType)
	}
}

func TestScopePopulation_CompanionFunctionParamShadowsClassProperty(t *testing.T) {
	src := `
class U(val id: String) {
    companion object {
        fun isUserId(id: String?): Boolean = id != null
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}
	body := flatFirstOfType(file, "function_body")
	if body == 0 {
		t.Fatal("expected to find function_body")
	}
	scope := fi.RootScope.FindScopeAtOffset(file.FlatStartByte(body) + 1)
	if scope == nil {
		t.Fatal("expected to find function scope")
	}
	idType := scope.Lookup("id")
	if idType == nil || !idType.IsNullable() {
		t.Fatalf("the companion function parameter id: String? must win over the class property, got %#v", idType)
	}
}

func TestScopePopulation_SmartCastDoesNotLeakFromLaterDeclarationsToRoot(t *testing.T) {
	src := `
interface First { fun a() }

class Second {
    fun checked(url: String?) {
        requireNotNull(url)
    }
    fun other(url: String?) {
        println(url)
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}
	if fi.RootScope.SmartCasts["url"] {
		t.Fatal("a requireNotNull inside a function must not mark the root scope")
	}
	println := file.FlatStartByte(flatNthOfType(file, "function_body", 1)) + 1
	scope := fi.RootScope.FindScopeAtOffset(println)
	if typ := scope.LookupAt("url", println); typ == nil || !typ.IsNullable() {
		t.Fatalf("the url parameter of the second function must stay nullable, got %#v", typ)
	}
}

func flatNthOfType(file *scanner.File, nodeType string, n int) uint32 {
	var found uint32
	count := 0
	file.FlatWalkNodes(0, nodeType, func(idx uint32) {
		if count == n && found == 0 {
			found = idx
		}
		count++
	})
	return found
}
