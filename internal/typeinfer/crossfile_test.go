package typeinfer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// parseTempFile writes Kotlin source to a named temp file and parses it.
func parseTempFile(t *testing.T, dir, name, src string) *scanner.File {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCrossFile_InheritanceResolvesToFQN(t *testing.T) {
	dir := t.TempDir()

	file1 := parseTempFile(t, dir, "file1.kt", `package com.example
class Parent { fun greet(): String = "hello" }
`)

	file2 := parseTempFile(t, dir, "file2.kt", `package com.example
class Child : Parent()
`)

	resolver := NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file1, file2}, 1)

	info := resolver.ClassHierarchy("Child")
	if info == nil {
		t.Fatal("expected ClassInfo for Child, got nil")
	}
	if len(info.Supertypes) == 0 {
		t.Fatal("expected Child to have supertypes")
	}

	found := false
	for _, st := range info.Supertypes {
		if st == "com.example.Parent" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Child supertypes to contain 'com.example.Parent', got %v", info.Supertypes)
	}
}

func TestCrossFile_SealedVariantsAcrossFiles(t *testing.T) {
	dir := t.TempDir()

	file1 := parseTempFile(t, dir, "file1.kt", `package com.example
sealed class Result
`)

	file2 := parseTempFile(t, dir, "file2.kt", `package com.example
data class Success(val data: String) : Result()
data class Failure(val error: String) : Result()
`)

	resolver := NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file1, file2}, 1)

	variants := resolver.SealedVariants("Result")
	if len(variants) < 2 {
		t.Fatalf("expected at least 2 sealed variants for Result, got %d: %v", len(variants), variants)
	}

	has := func(name string) bool {
		for _, v := range variants {
			if v == name {
				return true
			}
		}
		return false
	}
	if !has("Success") {
		t.Errorf("expected sealed variants to contain 'Success', got %v", variants)
	}
	if !has("Failure") {
		t.Errorf("expected sealed variants to contain 'Failure', got %v", variants)
	}
}

func TestCrossFile_InterfaceImplementation(t *testing.T) {
	dir := t.TempDir()

	file1 := parseTempFile(t, dir, "file1.kt", `package com.example
interface Repository
`)

	file2 := parseTempFile(t, dir, "file2.kt", `package com.example
class UserRepository : Repository
`)

	resolver := NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file1, file2}, 1)

	info := resolver.ClassHierarchy("UserRepository")
	if info == nil {
		t.Fatal("expected ClassInfo for UserRepository, got nil")
	}
	if len(info.Supertypes) == 0 {
		t.Fatal("expected UserRepository to have supertypes")
	}

	found := false
	for _, st := range info.Supertypes {
		if st == "com.example.Repository" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UserRepository supertypes to contain 'com.example.Repository', got %v", info.Supertypes)
	}
}
