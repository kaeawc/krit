package typeinfer

import "testing"

// --- buildImportTable ---

func TestBuildImportTable_RegularImports(t *testing.T) {
	src := `
import java.util.Date
import kotlin.collections.ArrayList

class Foo
`
	file := parseTestFile(t, src)

	it := buildImportTableFlat(file)

	if got, ok := it.Explicit["Date"]; !ok || got != "java.util.Date" {
		t.Errorf("expected Explicit[Date]=java.util.Date, got %q (ok=%v)", got, ok)
	}
	if got, ok := it.Explicit["ArrayList"]; !ok || got != "kotlin.collections.ArrayList" {
		t.Errorf("expected Explicit[ArrayList]=kotlin.collections.ArrayList, got %q (ok=%v)", got, ok)
	}
}

func TestBuildImportTable_AliasedImport(t *testing.T) {
	src := `
import kotlin.collections.MutableList as ML
import java.io.File as JFile

class Foo
`
	file := parseTestFile(t, src)

	it := buildImportTableFlat(file)

	if got, ok := it.Aliases["ML"]; !ok || got != "kotlin.collections.MutableList" {
		t.Errorf("expected Aliases[ML]=kotlin.collections.MutableList, got %q (ok=%v)", got, ok)
	}
	if got, ok := it.Aliases["JFile"]; !ok || got != "java.io.File" {
		t.Errorf("expected Aliases[JFile]=java.io.File, got %q (ok=%v)", got, ok)
	}
}

func TestBuildImportTable_WildcardImport(t *testing.T) {
	src := `
import com.example.*
import java.util.*

class Foo
`
	file := parseTestFile(t, src)

	it := buildImportTableFlat(file)

	if len(it.Wildcard) != 2 {
		t.Fatalf("expected 2 wildcard imports, got %d", len(it.Wildcard))
	}
	found := map[string]bool{}
	for _, w := range it.Wildcard {
		found[w] = true
	}
	if !found["com.example"] {
		t.Error("expected wildcard import for com.example")
	}
	if !found["java.util"] {
		t.Error("expected wildcard import for java.util")
	}
}

func TestBuildImportTable_MixedImports(t *testing.T) {
	src := `
import java.util.Date
import kotlin.collections.MutableList as ML
import com.example.*

class Foo
`
	file := parseTestFile(t, src)

	it := buildImportTableFlat(file)

	if _, ok := it.Explicit["Date"]; !ok {
		t.Error("expected explicit import for Date")
	}
	if _, ok := it.Aliases["ML"]; !ok {
		t.Error("expected alias import for ML")
	}
	if len(it.Wildcard) != 1 || it.Wildcard[0] != "com.example" {
		t.Errorf("expected wildcard [com.example], got %v", it.Wildcard)
	}
}

func TestBuildImportTable_NoImports(t *testing.T) {
	src := `class Foo
`
	file := parseTestFile(t, src)

	it := buildImportTableFlat(file)

	if len(it.Explicit) != 0 {
		t.Errorf("expected no explicit imports, got %d", len(it.Explicit))
	}
	if len(it.Aliases) != 0 {
		t.Errorf("expected no aliases, got %d", len(it.Aliases))
	}
	if len(it.Wildcard) != 0 {
		t.Errorf("expected no wildcards, got %d", len(it.Wildcard))
	}
}

// --- extractPackage ---

func TestExtractPackage_Present(t *testing.T) {
	src := `package com.example.app

class Foo
`
	file := parseTestFile(t, src)

	got := extractPackageFlat(file)
	if got != "com.example.app" {
		t.Errorf("expected 'com.example.app', got %q", got)
	}
}

func TestExtractPackage_NoPackage(t *testing.T) {
	src := `class Foo
`
	file := parseTestFile(t, src)

	got := extractPackageFlat(file)
	if got != "" {
		t.Errorf("expected empty string for no package, got %q", got)
	}
}

// Regression for #44 / #114: tree-sitter Kotlin sometimes attaches a
// trailing block comment to the package_header node. The pre-migration
// code did `TrimPrefix(text, "package ")` and stopped — leaving the
// trivia attached to the returned package name.
func TestExtractPackage_StripsTrailingBlockCommentTrivia(t *testing.T) {
	src := `package com.example.app
/* TODO: split this module */

class Foo
`
	file := parseTestFile(t, src)

	got := extractPackageFlat(file)
	if got != "com.example.app" {
		t.Errorf("expected 'com.example.app' (trivia stripped), got %q", got)
	}
}

// Regression for the same bug surfaced via buildImportTableFlat.
func TestImportTable_StripsTrailingBlockCommentTrivia(t *testing.T) {
	src := `package test

import com.example.Foo
/* trailing trivia attached to last import */

class Bar { fun f() = Foo() }
`
	file := parseTestFile(t, src)
	it := buildImportTableFlat(file)
	got, ok := it.Explicit["Foo"]
	if !ok {
		t.Fatalf("Foo missing from Explicit imports: %+v", it.Explicit)
	}
	if got != "com.example.Foo" {
		t.Errorf("Explicit[Foo] = %q, want com.example.Foo (trivia stripped)", got)
	}
}

// --- Kotlin auto-imports of java.lang.* ---

func TestImportTable_Resolve_KotlinAutoImports(t *testing.T) {
	src := `package test

class Runner { fun grep(p: String) { ProcessBuilder("sh", "-c", "grep $p").start() } }
`
	file := parseTestFile(t, src)

	it := buildImportTableFlat(file)

	if got := it.Resolve("ProcessBuilder"); got != "java.lang.ProcessBuilder" {
		t.Errorf("expected ProcessBuilder to auto-resolve to java.lang.ProcessBuilder, got %q", got)
	}
	if got := it.Resolve("Runtime"); got != "java.lang.Runtime" {
		t.Errorf("expected Runtime to auto-resolve to java.lang.Runtime, got %q", got)
	}
	if got := it.Resolve("Thread"); got != "java.lang.Thread" {
		t.Errorf("expected Thread to auto-resolve to java.lang.Thread, got %q", got)
	}
	// Explicit/alias must still win over auto-imports.
	it.Explicit["Runtime"] = "com.example.Runtime"
	if got := it.Resolve("Runtime"); got != "com.example.Runtime" {
		t.Errorf("expected explicit import to override auto-import, got %q", got)
	}
}

func TestExtractPackage_WithImports(t *testing.T) {
	src := `package org.example
import java.util.Date

class Foo
`
	file := parseTestFile(t, src)

	got := extractPackageFlat(file)
	if got != "org.example" {
		t.Errorf("expected 'org.example', got %q", got)
	}
}
