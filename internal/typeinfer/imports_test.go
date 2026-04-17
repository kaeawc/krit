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

	it := buildImportTableFlat(0, file)

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

	it := buildImportTableFlat(0, file)

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

	it := buildImportTableFlat(0, file)

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

	it := buildImportTableFlat(0, file)

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

	it := buildImportTableFlat(0, file)

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

	got := extractPackageFlat(0, file)
	if got != "com.example.app" {
		t.Errorf("expected 'com.example.app', got %q", got)
	}
}

func TestExtractPackage_NoPackage(t *testing.T) {
	src := `class Foo
`
	file := parseTestFile(t, src)

	got := extractPackageFlat(0, file)
	if got != "" {
		t.Errorf("expected empty string for no package, got %q", got)
	}
}

func TestExtractPackage_WithImports(t *testing.T) {
	src := `package org.example
import java.util.Date

class Foo
`
	file := parseTestFile(t, src)

	got := extractPackageFlat(0, file)
	if got != "org.example" {
		t.Errorf("expected 'org.example', got %q", got)
	}
}
