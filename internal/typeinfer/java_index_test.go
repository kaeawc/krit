package typeinfer

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func parseTempJavaFile(t *testing.T, dir, name, src string) *scanner.File {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseJavaFile(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// A Java class's kind and modifiers follow Java's defaults, not Kotlin's: a
// plain class is open, an interface is abstract, and only `final`, records
// and enums are closed (#1885).
func TestIndexFileParallel_JavaClassKindsAndModifiers(t *testing.T) {
	dir := t.TempDir()
	file := parseTempJavaFile(t, dir, "Shapes.java", `package com.example;

public class Shapes {
    public interface Part {}
    public @interface Marker {}
    public static final class Closed {}
    public abstract static class Base {}
    public enum Color { RED }
    public record Point(int x, int y) {}
    public sealed interface Shape permits Circle {}
    public static final class Circle implements Shape {}
}
`)
	info := IndexFileParallel(file)
	if info == nil {
		t.Fatal("IndexFileParallel returned nil for a Java file")
	}
	byName := map[string]*ClassInfo{}
	for _, ci := range info.Classes {
		byName[ci.Name] = ci
	}
	tests := []struct {
		name           string
		kind           string
		open, abstract bool
		nested         bool
	}{
		{"Shapes", "class", true, false, false},
		{"Part", "interface", false, true, true},
		{"Marker", "interface", false, true, true},
		{"Closed", "class", false, false, true},
		{"Base", "class", true, true, true},
		{"Color", "enum", false, false, true},
		{"Point", "class", false, false, true},
		{"Shape", "sealed interface", false, true, true},
		{"Circle", "class", false, false, true},
	}
	for _, tt := range tests {
		ci := byName[tt.name]
		if ci == nil {
			t.Errorf("%s: not indexed", tt.name)
			continue
		}
		if ci.Kind != tt.kind || ci.IsOpen != tt.open || ci.IsAbstract != tt.abstract || ci.IsNested != tt.nested {
			t.Errorf("%s: got kind=%q open=%v abstract=%v nested=%v, want kind=%q open=%v abstract=%v nested=%v",
				tt.name, ci.Kind, ci.IsOpen, ci.IsAbstract, ci.IsNested, tt.kind, tt.open, tt.abstract, tt.nested)
		}
		if ci.File != file.Path {
			t.Errorf("%s: File = %q, want %q", tt.name, ci.File, file.Path)
		}
	}
	if got := byName["Shapes"].FQN; got != "com.example.Shapes" {
		t.Errorf("Shapes FQN = %q", got)
	}
	if got := byName["Part"].FQN; got != "com.example.Shapes.Part" {
		t.Errorf("Part FQN = %q", got)
	}
}

// A Java supertype named through a single-type import is recorded by its
// qualified name, so swapping only that import changes the class's
// cross-file facts; unimported names stay as written.
func TestIndexFileParallel_JavaSupertypesQualifiedThroughImports(t *testing.T) {
	dir := t.TempDir()
	src := func(imp string) string {
		return "package com.example;\n\nimport " + imp + ";\n\npublic class JView extends LinearLayout implements Local, java.io.Serializable {}\n"
	}
	a := IndexFileParallel(parseTempJavaFile(t, dir, "A.java", src("android.widget.LinearLayout")))
	if len(a.Classes) != 1 {
		t.Fatalf("classes = %d, want 1", len(a.Classes))
	}
	want := []string{"android.widget.LinearLayout", "Local", "java.io.Serializable"}
	if got := a.Classes[0].Supertypes; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("Supertypes = %v, want %v", got, want)
	}
	b := IndexFileParallel(parseTempJavaFile(t, dir, "B.java", src("com.other.LinearLayout")))
	if a.CrossFileFacts().ByName["JView"] == b.CrossFileFacts().ByName["JView"] {
		t.Error("swapping only the supertype's import left JView's cross-file facts unchanged")
	}
}

// Kotlin sees a Java source supertype through the merged resolver, and the
// supertype resolution pass does not rewrite javafacts' cached slices.
func TestIndexFilesParallel_KotlinClassSeesJavaSupertype(t *testing.T) {
	dir := t.TempDir()
	java := parseTempJavaFile(t, dir, "JBase.java", `package com.example;

import android.widget.LinearLayout;

public class JBase extends LinearLayout {}
`)
	kt := parseTempFile(t, dir, "KView.kt", `package com.example

class KView : JBase()
`)
	r := NewResolver()
	r.IndexFilesParallel([]*scanner.File{kt, java}, 2)
	kview := r.ClassHierarchy("com.example.KView")
	if kview == nil || !slices.Contains(kview.Supertypes, "com.example.JBase") {
		t.Fatalf("KView supertypes = %+v, want [com.example.JBase]", kview)
	}
	jbase := r.ClassHierarchy("com.example.JBase")
	if jbase == nil || len(jbase.Supertypes) != 1 || jbase.Supertypes[0] != "android.widget.LinearLayout" {
		t.Fatalf("JBase = %+v, want supertype android.widget.LinearLayout", jbase)
	}
	if !jbase.IsOpen {
		t.Error("JBase is a non-final Java class and must be open")
	}
}

// A Java class that shares a simple name with a Kotlin class leaves the
// Kotlin class on that name whichever path sorts last, and is still found by
// its qualified name.
func TestIndexFilesParallel_KotlinKeepsSimpleNameOverJava(t *testing.T) {
	for _, javaName := range []string{"a/Text.java", "z/Text.java"} {
		dir := t.TempDir()
		for _, sub := range []string{"a", "m", "z"} {
			if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		java := parseTempJavaFile(t, dir, javaName, "package com.java;\n\npublic class Text {}\n")
		kt := parseTempFile(t, dir, "m/Text.kt", "package com.kt\n\nclass Text\n")
		r := NewResolver()
		r.IndexFilesParallel([]*scanner.File{java, kt}, 2)
		if got := r.ClassHierarchy("Text"); got == nil || got.FQN != "com.kt.Text" {
			t.Errorf("%s: ClassHierarchy(Text) = %+v, want the Kotlin com.kt.Text", javaName, got)
		}
		if got := r.ClassHierarchy("com.java.Text"); got == nil || got.File != java.Path {
			t.Errorf("%s: ClassHierarchy(com.java.Text) = %+v, want the Java class", javaName, got)
		}
	}
}
