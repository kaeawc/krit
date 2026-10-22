package scanner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func declNames(decls []TopLevelDecl) []string {
	names := make([]string, 0, len(decls))
	for _, d := range decls {
		names = append(names, d.Name)
	}
	return names
}

func TestTopLevelDeclaredNames(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{"typealias", "package p\ntypealias Alias = List<Int>\n", []string{"Alias"}},
		{"fun interface yields the name, not members", "package p\nfun interface Fi { fun run() }\n", []string{"Fi"}},
		{"fun interface with modifier and type parameter", "package p\npublic fun interface G<T> { fun run(t: T) }\n", []string{"G"}},
		{"annotated fun interface", "package p\n@FunctionalInterface fun interface H { fun run() }\n", []string{"H"}},
		{"fun interface with bound and supertype", "package p\nfun interface K<T : Any> : Base { fun run() }\n", []string{"K"}},
		{"context receiver function", "package p\ncontext(Foo) fun ctx() {}\n", []string{"ctx"}},
		{"private fun interface with annotation string", "package p\n@A(\"public\") private fun interface Pk { fun run() }\n", nil},
		{"private fun interface excluded", "package p\nprivate fun interface Fi { fun run() }\n", nil},
		{"extension property", "package p\nval String.ext: Int get() = 1\n", []string{"ext"}},
		{"extension function", "package p\nfun String.shout() = this\n", []string{"shout"}},
		{"expect declarations", "package p\nexpect fun ex(): Int\nexpect class Ec\n", []string{"Ec", "ex"}},
		{"object members do not leak", "package p\nobject O { fun member() {} val prop = 1 }\n", []string{"O"}},
		{"class members do not leak", "package p\nclass C(val x: Int) { fun m() {} class Nested }\n", []string{"C"}},
		{"private excluded", "package p\nprivate fun hid() {}\nprivate val v = 1\nprivate class K\nprivate typealias T = Int\nfun shown() {}\n", []string{"shown"}},
		{"internal included", "package p\ninternal fun inner() {}\n", []string{"inner"}},
		{"interface and enum", "package p\ninterface I\nenum class E { A }\n", []string{"E", "I"}},
		{"backticked name", "package p\nfun `my fun`() {}\n", []string{"my fun"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, decls := TopLevelDeclaredNames(parseKotlinSnippet(t, tc.src))
			got := declNames(decls)
			if len(got) == 0 {
				got = nil
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTopLevelDeclaredNamesOperatorAndPackage(t *testing.T) {
	pkg, decls := TopLevelDeclaredNames(parseKotlinSnippet(t, "/* package x */\n@file:Suppress(\"a\")\npackage `p`.q\n\noperator fun Any.plus(o: Any): Int? = null\nfun pick() {}\n"))
	if pkg != "p.q" {
		t.Fatalf("package = %q, want p.q", pkg)
	}
	ops := map[string]bool{}
	for _, d := range decls {
		ops[d.Name] = d.Operator
	}
	if !reflect.DeepEqual(ops, map[string]bool{"plus": true, "pick": false}) {
		t.Fatalf("operator flags = %v", ops)
	}
}

func sigOf(t *testing.T, src, name string) string {
	t.Helper()
	_, decls := TopLevelDeclaredNames(parseKotlinSnippet(t, src))
	for _, d := range decls {
		if d.Name == name {
			if d.Sig == "" {
				t.Fatalf("%s has an empty Sig", name)
			}
			return d.Sig
		}
	}
	t.Fatalf("%s not declared in %q", name, src)
	return ""
}

// Sig changes exactly when something a caller can observe changes.
func TestTopLevelDeclaredNamesSig(t *testing.T) {
	const base = "package p\nfun pick(x: Int): Int {\n    return 1\n}\n"
	same := []struct{ name, src string }{
		{"body edit", "package p\nfun pick(x: Int): Int {\n    return 22 + 33\n}\n"},
		{"comment in body", "package p\nfun pick(x: Int): Int {\n    // c\n    return 1\n}\n"},
		{"unrelated declaration added", base + "fun other() = 1\n"},
		{"declaration moved down", "package p\nfun other() = 1\nfun pick(x: Int): Int {\n    return 1\n}\n"},
	}
	for _, tc := range same {
		if got, want := sigOf(t, tc.src, "pick"), sigOf(t, base, "pick"); got != want {
			t.Errorf("%s changed Sig", tc.name)
		}
	}
	differs := []struct{ name, src string }{
		{"parameter type", "package p\nfun pick(x: Long): Int {\n    return 1\n}\n"},
		{"parameter name", "package p\nfun pick(y: Int): Int {\n    return 1\n}\n"},
		{"return type", "package p\nfun pick(x: Int): Long {\n    return 1\n}\n"},
		{"receiver", "package p\nfun String.pick(x: Int): Int {\n    return 1\n}\n"},
		{"modifier", "package p\ninfix fun pick(x: Int): Int {\n    return 1\n}\n"},
		{"annotation", "package p\n@Deprecated(\"x\")\nfun pick(x: Int): Int {\n    return 1\n}\n"},
		{"added overload", base + "fun pick(x: String): Int {\n    return 1\n}\n"},
		{"expression body", "package p\nfun pick(x: Int) = 1\n"},
		{"inline body", "package p\ninline fun pick(x: Int): Int {\n    return 1\n}\n"},
		{"contract body", "package p\nfun pick(x: Int): Int {\n    contract { returns() }\n    return 1\n}\n"},
	}
	for _, tc := range differs {
		if got, want := sigOf(t, tc.src, "pick"), sigOf(t, base, "pick"); got == want {
			t.Errorf("%s did not change Sig", tc.name)
		}
	}
	// The same overloads in another order are the same declarations.
	a := "package p\nfun pick(x: Int) {\n}\nfun pick(x: String) {\n}\n"
	b := "package p\nfun pick(x: String) {\n}\nfun pick(x: Int) {\n}\n"
	if sigOf(t, a, "pick") != sigOf(t, b, "pick") {
		t.Error("reordering overloads changed Sig")
	}
	// Expression bodies, properties, and classes keep their whole text.
	for _, pair := range []struct{ name, before, after string }{
		{"f", "package p\nfun f() = 1\n", "package p\nfun f() = 2\n"},
		{"k", "package p\nconst val k = 1\n", "package p\nconst val k = 2\n"},
		{"C", "package p\nclass C { fun m() = 1 }\n", "package p\nclass C { fun m() = 2 }\n"},
	} {
		if sigOf(t, pair.before, pair.name) == sigOf(t, pair.after, pair.name) {
			t.Errorf("%q and %q share a Sig", pair.before, pair.after)
		}
	}
}

func TestTopLevelJavaTypeNames(t *testing.T) {
	src := "package a.b;\n\nimport java.util.List;\n\npublic class Greeter {\n    private int x;\n    public Greeter(int x) {}\n    class Inner {}\n}\n\ninterface Speaker {}\nenum Mode { A }\nrecord Point(int x) {}\n@interface Marker {}\n"
	path := filepath.Join(t.TempDir(), "Greeter.java")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := ParseJavaFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	pkg, decls := TopLevelJavaTypeNames(file)
	if pkg != "a.b" {
		t.Fatalf("package = %q, want a.b", pkg)
	}
	if got, want := declNames(decls), []string{"Greeter", "Marker", "Mode", "Point", "Speaker"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
	sig := decls[0].Sig
	edited := strings.Replace(src, "public Greeter(int x) {}", "public Greeter(long x) {}", 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err = ParseJavaFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, after := TopLevelJavaTypeNames(file); after[0].Sig == sig {
		t.Fatal("changing a constructor must change the type's Sig")
	}
}

func TestParseSourceHeader(t *testing.T) {
	src := "// c\n/** doc */\n@file:JvmName(\"X\")\n@file:[A B]\npackage a.`b`.c\n\nimport x.y.Z\nimport x.w.* // wild\nimport q.r as s\n/* import no.Thing */\nfun f() { import fake.Not }\n"
	h := ParseSourceHeader([]byte(src))
	if h.Package != "a.b.c" {
		t.Fatalf("package = %q", h.Package)
	}
	want := []HeaderImport{
		{Path: "x.y.Z"},
		{Path: "x.w", Wildcard: true},
		{Path: "q.r", Alias: "s"},
	}
	if !reflect.DeepEqual(h.Imports, want) {
		t.Fatalf("imports = %+v, want %+v", h.Imports, want)
	}
	bom := ParseSourceHeader([]byte("\xEF\xBB\xBFpackage a.b\nimport c.D\n"))
	if bom.Package != "a.b" || len(bom.Imports) != 1 || bom.Imports[0].Path != "c.D" {
		t.Fatalf("BOM header = %+v", bom)
	}
	wrapped := ParseSourceHeader([]byte("package a .\n  b\nimport c . D\nimport e.\n  /* c */ *\n"))
	if wrapped.Package != "a.b" || !reflect.DeepEqual(wrapped.Imports, []HeaderImport{{Path: "c.D"}, {Path: "e", Wildcard: true}}) {
		t.Fatalf("wrapped header = %+v", wrapped)
	}
	if h := ParseSourceHeader([]byte("fun f() {}\npackage late\n")); h.Package != "" {
		t.Fatalf("package after a declaration = %q, want empty", h.Package)
	}
}
