package typeinfer

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestExtensionFunction_BasicStringReceiver(t *testing.T) {
	src := `
package com.example

fun String.exclaim(): String = this + "!"

val result = "hello".exclaim()
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	// Verify extension is indexed
	if len(fi.Extensions) == 0 {
		t.Fatal("expected at least one extension function")
	}

	ext := fi.Extensions[0]
	if ext.ReceiverType != "String" {
		t.Errorf("expected receiver type String, got %q", ext.ReceiverType)
	}
	if ext.Name != "exclaim" {
		t.Errorf("expected function name exclaim, got %q", ext.Name)
	}
	if ext.ReturnType == nil || ext.ReturnType.Name != "String" {
		t.Errorf("expected return type String, got %v", ext.ReturnType)
	}

	// Verify the call resolves to String via the full resolver
	resolver := NewResolver()
	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}
	resolver.extensions = append(resolver.extensions, fi.Extensions...)
	for _, ci := range fi.Classes {
		resolver.classes[ci.Name] = ci
		if ci.FQN != "" {
			resolver.classFQN[ci.FQN] = ci
		}
	}

	// Find the call_expression "hello".exclaim() and resolve via flat idx.
	idx := flatFirstOfTypeWithText(file, "call_expression", `"hello".exclaim()`)
	if idx != 0 {
		resolved := resolver.ResolveFlatNode(idx, file)
		if resolved == nil || resolved.Name != "String" {
			t.Errorf("expected call to resolve to String, got %v", resolved)
		}
	}
}

func TestExtensionFunction_CustomTypeReceiver(t *testing.T) {
	src := `
package com.example

class User(val name: String)

fun User.greet(): String = "Hello, ${name}"
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if len(fi.Extensions) == 0 {
		t.Fatal("expected at least one extension function")
	}

	ext := fi.Extensions[0]
	if ext.ReceiverType != "User" {
		t.Errorf("expected receiver type User, got %q", ext.ReceiverType)
	}
	if ext.Name != "greet" {
		t.Errorf("expected function name greet, got %q", ext.Name)
	}
	if ext.ReturnType == nil || ext.ReturnType.Name != "String" {
		t.Errorf("expected return type String, got %v", ext.ReturnType)
	}
}

func TestExtensionFunction_WrongReceiverDoesNotResolve(t *testing.T) {
	src := `
package com.example

fun String.exclaim(): String = this + "!"

val x: Int = 42
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	// Build resolver with extensions
	resolver := NewResolver()
	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}
	resolver.extensions = append(resolver.extensions, fi.Extensions...)

	// Verify the extension is only for String, not Int
	found := false
	for _, ext := range resolver.extensions {
		if ext.Name == "exclaim" && ext.ReceiverType == "Int" {
			found = true
		}
	}
	if found {
		t.Error("should NOT have an extension function exclaim for Int receiver")
	}
}

func TestExtensionFunction_MultipleExtensions(t *testing.T) {
	src := `
package com.example

fun String.exclaim(): String = this + "!"
fun Int.double(): Int = this * 2
fun String.shout(): String = this.uppercase()
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if len(fi.Extensions) != 3 {
		t.Fatalf("expected 3 extension functions, got %d", len(fi.Extensions))
	}

	// Verify each extension
	byName := make(map[string]*ExtensionFuncInfo)
	for _, ext := range fi.Extensions {
		byName[ext.Name] = ext
	}

	if ext, ok := byName["exclaim"]; !ok {
		t.Error("missing extension exclaim")
	} else if ext.ReceiverType != "String" {
		t.Errorf("expected exclaim receiver String, got %q", ext.ReceiverType)
	}

	if ext, ok := byName["double"]; !ok {
		t.Error("missing extension double")
	} else {
		if ext.ReceiverType != "Int" {
			t.Errorf("expected double receiver Int, got %q", ext.ReceiverType)
		}
		if ext.ReturnType == nil || ext.ReturnType.Name != "Int" {
			t.Errorf("expected double return type Int, got %v", ext.ReturnType)
		}
	}

	if ext, ok := byName["shout"]; !ok {
		t.Error("missing extension shout")
	} else if ext.ReceiverType != "String" {
		t.Errorf("expected shout receiver String, got %q", ext.ReceiverType)
	}
}

func TestExtensionFunction_ParallelMerge(t *testing.T) {
	src := `
package com.example

fun String.exclaim(): String = this + "!"
`
	file := parseTestFile(t, src)
	resolver := NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)

	if len(resolver.extensions) == 0 {
		t.Fatal("expected extensions to be merged into resolver")
	}

	ext := resolver.extensions[0]
	if ext.ReceiverType != "String" || ext.Name != "exclaim" {
		t.Errorf("unexpected extension: receiver=%q name=%q", ext.ReceiverType, ext.Name)
	}
}

func TestExtensionFunction_RegularFunctionNotIndexedAsExtension(t *testing.T) {
	src := `
package com.example

fun calculate(): Int {
    return 42
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	if len(fi.Extensions) != 0 {
		t.Errorf("expected no extension functions, got %d", len(fi.Extensions))
	}

	// Regular function should still be indexed
	if _, ok := fi.Functions["calculate"]; !ok {
		t.Error("expected calculate to be indexed as a regular function")
	}
}
