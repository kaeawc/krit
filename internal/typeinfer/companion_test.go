package typeinfer

import (
	"testing"
)

func TestCompanionObject_FactoryFunction(t *testing.T) {
	src := `
package com.example

class User(val name: String) {
    companion object {
        fun create(name: String): User = User(name)
    }
}

val u = User.create("Alice")
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	// Verify the companion function is indexed as User.create
	retType, ok := fi.Functions["User.create"]
	if !ok {
		t.Fatal("expected User.create to be indexed in functions map")
	}
	if retType.Name != "User" {
		t.Errorf("expected return type User, got %q", retType.Name)
	}
}

func TestCompanionObject_DifferentReturnTypes(t *testing.T) {
	src := `
package com.example

class Config {
    companion object {
        fun default(): Config = Config()
        fun maxRetries(): Int = 3
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	// Config.default() -> Config
	retType, ok := fi.Functions["Config.default"]
	if !ok {
		t.Fatal("expected Config.default to be indexed")
	}
	if retType.Name != "Config" {
		t.Errorf("expected return type Config, got %q", retType.Name)
	}

	// Config.maxRetries() -> Int
	retType, ok = fi.Functions["Config.maxRetries"]
	if !ok {
		t.Fatal("expected Config.maxRetries to be indexed")
	}
	if retType.Name != "Int" {
		t.Errorf("expected return type Int, got %q", retType.Name)
	}
}

func TestCompanionObject_CallResolution(t *testing.T) {
	src := `
package com.example

class User(val name: String) {
    companion object {
        fun create(name: String): User = User(name)
    }
}

fun main() {
    val u = User.create("Alice")
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
	for _, ci := range fi.Classes {
		resolver.classes[ci.Name] = ci
		if ci.FQN != "" {
			resolver.classFQN[ci.FQN] = ci
		}
	}
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}

	// Find the call_expression for User.create("Alice")
	idx := flatFirstOfTypeWithText(file, "call_expression", `User.create("Alice")`)
	if idx == 0 {
		t.Fatal("expected to find User.create(\"Alice\") call_expression")
	}

	resolved := resolver.ResolveFlatNode(idx, file)
	if resolved == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if resolved.Name != "User" {
		t.Errorf("expected call to resolve to User, got %q", resolved.Name)
	}
}

func TestCompanionObject_PropertyAccess(t *testing.T) {
	src := `
package com.example

class AppConfig {
    companion object {
        val instance: AppConfig = AppConfig()
    }
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	retType, ok := fi.Functions["AppConfig.instance"]
	if !ok {
		t.Fatal("expected AppConfig.instance to be indexed in functions map")
	}
	if retType.Name != "AppConfig" {
		t.Errorf("expected type AppConfig, got %q", retType.Name)
	}
}

func TestCompanionObject_NavigationResolution(t *testing.T) {
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
	resolver := NewResolver()
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	// Simulate merge
	resolver.imports[fi.Path] = fi.ImportTable
	resolver.scopes[fi.Path] = fi.RootScope
	for _, ci := range fi.Classes {
		resolver.classes[ci.Name] = ci
		if ci.FQN != "" {
			resolver.classFQN[ci.FQN] = ci
		}
	}
	for name, retType := range fi.Functions {
		resolver.functions[name] = retType
	}

	// Find the navigation_expression for AppConfig.instance
	idx := flatFirstOfTypeWithText(file, "navigation_expression", "AppConfig.instance")
	if idx == 0 {
		t.Fatal("expected to find AppConfig.instance navigation_expression")
	}

	resolved := resolver.ResolveFlatNode(idx, file)
	if resolved == nil {
		t.Fatal("expected resolved type, got nil")
	}
	if resolved.Name != "AppConfig" {
		t.Errorf("expected navigation to resolve to AppConfig, got %q", resolved.Name)
	}
}
