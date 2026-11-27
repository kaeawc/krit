package typeinfer

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// javaResolverOf indexes the named sources, .java as Java and .kt as Kotlin, each under dir/<name>.
func javaResolverOf(t *testing.T, sources map[string]string) (*defaultResolver, map[string]*scanner.File) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]*scanner.File{}
	var all []*scanner.File
	for name, src := range sources {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		var f *scanner.File
		if strings.HasSuffix(name, ".java") {
			f = parseTempJavaFile(t, dir, name, src)
		} else {
			f = parseTempFile(t, dir, name, src)
		}
		files[name] = f
		all = append(all, f)
	}
	r := NewResolver()
	r.IndexFilesParallel(all, 2)
	return r, files
}

// #1950 review (P2): a Java class's unqualified supertype resolves as javac resolves it, never to an unrelated
// same-named class of another package.
func TestIndexFilesParallel_JavaSupertypeLookupOrder(t *testing.T) {
	kotlinA := "package k\n\nopen class A\n"
	for _, tc := range []struct {
		name    string
		sources map[string]string
		class   string
		want    string
	}{
		{"a Kotlin class of another package is not it", map[string]string{
			"j/B.java": "package j;\n\npublic class B extends A {}\n", "k/A.kt": kotlinA,
		}, "j.B", "j.A"},
		{"the same package's class is", map[string]string{
			"j/B.java": "package j;\n\npublic class B extends A {}\n", "j/A.java": "package j;\n\npublic class A {}\n", "k/A.kt": kotlinA,
		}, "j.B", "j.A"},
		{"a member type of an enclosing class wins over the package", map[string]string{
			"j/Outer.java": "package j;\n\npublic class Outer {\n  static class A {}\n  static class B extends A {}\n}\n",
			"j/A.java":     "package j;\n\npublic class A {}\n",
		}, "j.Outer.B", "j.Outer.A"},
		{"an on-demand import's class", map[string]string{
			"j/B.java": "package j;\n\nimport k.*;\n\npublic class B extends A {}\n", "k/A.kt": kotlinA,
		}, "j.B", "k.A"},
		{"with one on-demand import and no match, a source class's name is the import's", map[string]string{
			"j/B.java": "package j;\n\nimport android.widget.*;\n\npublic class B extends A {}\n", "k/A.kt": kotlinA,
		}, "j.B", "android.widget.A"},
		{"with several on-demand imports and no match, a source class's name is the package's", map[string]string{
			"j/B.java": "package j;\n\nimport java.util.*;\nimport android.widget.*;\n\npublic class B extends A {}\n", "k/A.kt": kotlinA,
		}, "j.B", "j.A"},
		{"a name no source class has stays as written", map[string]string{
			"j/B.java": "package j;\n\nimport android.widget.*;\n\npublic class B extends ImageView {}\n", "k/A.kt": kotlinA,
		}, "j.B", "ImageView"},
		{"a parent package's class is not it", map[string]string{
			"j/sub/B.java": "package j.sub;\n\nimport k.*;\n\npublic class B extends A {}\n", "j/A.kt": "package j\n\nopen class A\n", "k/A.kt": kotlinA,
		}, "j.sub.B", "k.A"},
		{"nor without an on-demand import", map[string]string{
			"j/sub/B.java": "package j.sub;\n\npublic class B extends A {}\n", "j/A.kt": "package j\n\nopen class A\n",
		}, "j.sub.B", "j.sub.A"},
		{"a java.lang name stays as written", map[string]string{
			"j/B.java": "package j;\n\npublic class B extends Thread {}\n", "k/Thread.kt": "package k\n\nopen class Thread\n",
		}, "j.B", "Thread"},
		{"a single-type import wins", map[string]string{
			"j/B.java": "package j;\n\nimport k.A;\n\npublic class B extends A {}\n", "j/A.java": "package j;\n\npublic class A {}\n", "k/A.kt": kotlinA,
		}, "j.B", "k.A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := javaResolverOf(t, tc.sources)
			info := r.ClassHierarchy(tc.class)
			if info == nil || len(info.Supertypes) != 1 || info.Supertypes[0] != tc.want {
				t.Fatalf("%s = %+v, want supertypes [%s]", tc.class, info, tc.want)
			}
		})
	}
}

// The FileTypeInfo a cache stores keeps Java supertypes as written: the resolver serves resolved copies, so a cached
// entry reused by the next run (the daemon's resident cache) carries no stale resolution.
func TestIndexFilesParallel_JavaResolutionLeavesFileTypeInfoAsWritten(t *testing.T) {
	r, files := javaResolverOf(t, map[string]string{
		"j/B.java": "package j;\n\nimport k.*;\n\npublic class B extends A {}\n", "k/A.kt": "package k\n\nopen class A\n",
	})
	if got := r.ClassHierarchy("j.B").Supertypes; len(got) != 1 || got[0] != "k.A" {
		t.Fatalf("resolved supertypes = %v, want [k.A]", got)
	}
	if got := r.FileTypeInfoOf(files["j/B.java"].Path).Classes[0].Supertypes; len(got) != 1 || got[0] != "A" {
		t.Fatalf("FileTypeInfo supertypes = %v, want [A] as written", got)
	}
}

// Adding or removing an on-demand import changes what a Java class's unqualified supertype names, so it changes the
// class's cross-file facts; a class whose supertypes are all qualified keeps them.
func TestCrossFileFacts_JavaWildcardImports(t *testing.T) {
	dir := t.TempDir()
	facts := func(name, src string) CrossFileFacts {
		return IndexFileParallel(parseTempJavaFile(t, dir, name, src)).CrossFileFacts()
	}
	plain := facts("B1.java", "package j;\n\npublic class B extends A {}\n")
	starred := facts("B2.java", "package j;\n\nimport k.*;\n\npublic class B extends A {}\n")
	if plain.ByName["j.B"] == starred.ByName["j.B"] {
		t.Error("adding import k.* left j.B's facts unchanged")
	}
	q1 := facts("C1.java", "package j;\n\nimport k.A;\n\npublic class C extends A {}\n")
	q2 := facts("C2.java", "package j;\n\nimport k.A;\nimport z.*;\n\npublic class C extends A {}\n")
	if q1.ByName["j.C"] != q2.ByName["j.C"] {
		t.Error("an on-demand import changed the facts of a class whose supertypes are all qualified")
	}
}

// #1950 review (P3): nested classes of the same simple name in two outer classes are both indexed, and a non-static
// member class of a class is inner.
func TestIndexFileParallel_JavaNestedClasses(t *testing.T) {
	r, _ := javaResolverOf(t, map[string]string{
		"j/Outer.java": "package j;\n\npublic class Outer {\n  class Inner {}\n  static class Nested {}\n  interface I { class InI {} }\n}\n\nclass Other {\n  static class Inner extends Outer {}\n}\n",
	})
	for fqn, want := range map[string][2]bool{ // {IsNested, IsInner}
		"j.Outer.Inner": {true, true}, "j.Other.Inner": {true, false}, "j.Outer.Nested": {true, false},
		"j.Outer.I.InI": {true, false}, "j.Outer": {false, false},
	} {
		info := r.ClassHierarchy(fqn)
		if info == nil || info.FQN != fqn {
			t.Errorf("%s: not indexed (%+v)", fqn, info)
			continue
		}
		if info.IsNested != want[0] || info.IsInner != want[1] {
			t.Errorf("%s: nested=%v inner=%v, want nested=%v inner=%v", fqn, info.IsNested, info.IsInner, want[0], want[1])
		}
	}
	// SourceClass (#1928) finds a nested class through its Outer.
	if got := r.SourceClass("j.Outer.Inner"); got == nil || got.File == "" || got.Outer != "j.Outer" {
		t.Errorf("SourceClass(j.Outer.Inner) = %+v, want the Java class nested in j.Outer", got)
	}
	if got := r.ClassHierarchy("j.Outer.I.InI").Outer; got != "j.Outer.I" {
		t.Errorf("j.Outer.I.InI Outer = %q, want j.Outer.I", got)
	}
	if got := r.ClassHierarchy("j.Other.Inner").Supertypes; len(got) != 1 || got[0] != "j.Outer" {
		t.Errorf("j.Other.Inner supertypes = %v, want [j.Outer]", got)
	}
}
