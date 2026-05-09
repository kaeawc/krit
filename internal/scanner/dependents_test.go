package scanner

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func writeAndParseKotlin(t *testing.T, dir, name, content string) *File {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := ParseFile(path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

func TestDependentsIndex_BasicImports(t *testing.T) {
	dir := t.TempDir()
	a := writeAndParseKotlin(t, dir, "A.kt", `package a
import b.B
import c.C as Aliased

class A
`)
	b := writeAndParseKotlin(t, dir, "B.kt", `package b
class B
`)
	idx := BuildDependentsIndex([]*File{a, b})

	got := idx.ImportsOfFile(a.Path)
	want := []string{"b.B", "c.C"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ImportsOfFile(A) = %v, want %v", got, want)
	}
	if files := idx.FilesImporting("b.B"); !reflect.DeepEqual(files, []string{a.Path}) {
		t.Errorf("FilesImporting(b.B) = %v, want [%s]", files, a.Path)
	}
	if files := idx.FilesImporting("c.C"); !reflect.DeepEqual(files, []string{a.Path}) {
		t.Errorf("FilesImporting(c.C alias) = %v, want [%s]", files, a.Path)
	}
}

func TestDependentsIndex_WildcardImports(t *testing.T) {
	dir := t.TempDir()
	a := writeAndParseKotlin(t, dir, "A.kt", `package a
import b.*
class A
`)
	idx := BuildDependentsIndex([]*File{a})
	if files := idx.FilesImportingPackage("b"); !reflect.DeepEqual(files, []string{a.Path}) {
		t.Errorf("FilesImportingPackage(b) = %v, want [%s]", files, a.Path)
	}
	if files := idx.FilesImporting("b.SomeClass"); len(files) != 0 {
		t.Errorf("FilesImporting(b.SomeClass) = %v, want empty (only wildcard)", files)
	}
}

func TestDependentsIndex_FilesAffectedBy(t *testing.T) {
	dir := t.TempDir()
	a := writeAndParseKotlin(t, dir, "A.kt", `package x
import y.B
class A
`)
	b := writeAndParseKotlin(t, dir, "B.kt", `package y
class B
`)
	c := writeAndParseKotlin(t, dir, "C.kt", `package x
import y.*
class C
`)
	d := writeAndParseKotlin(t, dir, "D.kt", `package z
class D
`)
	idx := BuildDependentsIndex([]*File{a, b, c, d})

	// B.kt changed and declares y.B; A imports y.B explicitly, C imports y.*.
	got := idx.FilesAffectedBy([]string{b.Path}, []string{"y.B"})
	want := []string{a.Path, b.Path, c.Path}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilesAffectedBy = %v, want %v", got, want)
	}

	// D.kt changed but nobody imports z.D — only D.kt itself.
	got = idx.FilesAffectedBy([]string{d.Path}, []string{"z.D"})
	if !reflect.DeepEqual(got, []string{d.Path}) {
		t.Errorf("isolated change: got %v, want [%s]", got, d.Path)
	}

	// Multiple changed files: union with no FQN diff.
	got = idx.FilesAffectedBy([]string{a.Path, c.Path}, nil)
	want = []string{a.Path, c.Path}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changed-only: got %v, want %v", got, want)
	}
}

func TestDependentsIndex_NilSafe(t *testing.T) {
	var idx *DependentsIndex
	if got := idx.ImportsOfFile("anything"); got != nil {
		t.Errorf("nil ImportsOfFile got %v", got)
	}
	if got := idx.FilesImporting("anything"); got != nil {
		t.Errorf("nil FilesImporting got %v", got)
	}
	got := idx.FilesAffectedBy([]string{"f"}, []string{"x.Y"})
	if !reflect.DeepEqual(got, []string{"f"}) {
		t.Errorf("nil FilesAffectedBy got %v, want [f]", got)
	}
}

func TestDependentsIndex_DropsNonKotlin(t *testing.T) {
	dir := t.TempDir()
	javaPath := filepath.Join(dir, "Foo.java")
	if err := os.WriteFile(javaPath, []byte("package x; class Foo {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jf, err := ParseJavaFile(javaPath)
	if err != nil {
		t.Fatal(err)
	}
	idx := BuildDependentsIndex([]*File{jf})
	if got := idx.ImportsOfFile(javaPath); got != nil {
		t.Errorf("Java file should not be indexed; got imports %v", got)
	}
}
