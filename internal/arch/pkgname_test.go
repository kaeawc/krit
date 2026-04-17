package arch

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestExpectedPackage_Standard(t *testing.T) {
	got := ExpectedPackage(
		"/project/app/src/main/kotlin/com/example/feature/Foo.kt",
		"/project/app/src/main/kotlin",
	)
	if got != "com.example.feature" {
		t.Errorf("expected com.example.feature, got %q", got)
	}
}

func TestExpectedPackage_Nested(t *testing.T) {
	got := ExpectedPackage(
		"/project/src/main/kotlin/com/example/deep/nested/pkg/Bar.kt",
		"/project/src/main/kotlin",
	)
	if got != "com.example.deep.nested.pkg" {
		t.Errorf("expected com.example.deep.nested.pkg, got %q", got)
	}
}

func TestExpectedPackage_RootPackage(t *testing.T) {
	got := ExpectedPackage(
		"/project/src/main/kotlin/Main.kt",
		"/project/src/main/kotlin",
	)
	if got != "" {
		t.Errorf("expected empty string for root package, got %q", got)
	}
}

func TestPackageNameDrift_Match(t *testing.T) {
	f := &scanner.File{
		Path: "/project/src/main/kotlin/com/example/Foo.kt",
		Lines: []string{
			"package com.example",
			"",
			"class Foo",
		},
	}
	drift := PackageNameDrift(f, "/project/src/main/kotlin")
	if drift != nil {
		t.Errorf("expected nil (match), got drift: declared=%q expected=%q", drift.Declared, drift.Expected)
	}
}

func TestPackageNameDrift_Mismatch(t *testing.T) {
	f := &scanner.File{
		Path: "/project/src/main/kotlin/com/example/Foo.kt",
		Lines: []string{
			"package com.wrong",
			"",
			"class Foo",
		},
	}
	drift := PackageNameDrift(f, "/project/src/main/kotlin")
	if drift == nil {
		t.Fatal("expected drift, got nil")
	}
	if drift.Declared != "com.wrong" {
		t.Errorf("expected declared=com.wrong, got %q", drift.Declared)
	}
	if drift.Expected != "com.example" {
		t.Errorf("expected expected=com.example, got %q", drift.Expected)
	}
	if drift.Line != 1 {
		t.Errorf("expected line 1, got %d", drift.Line)
	}
}

func TestPackageNameDrift_NoDeclaration(t *testing.T) {
	f := &scanner.File{
		Path: "/project/src/main/kotlin/com/example/Foo.kt",
		Lines: []string{
			"// no package declaration",
			"class Foo",
		},
	}
	drift := PackageNameDrift(f, "/project/src/main/kotlin")
	if drift != nil {
		t.Errorf("expected nil (no declaration), got drift: declared=%q", drift.Declared)
	}
}
