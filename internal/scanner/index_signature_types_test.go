package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseJavaSnippet(t *testing.T, src string) *File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "A.java")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := ParseJavaFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func ktFingerprint(t *testing.T, src string) string {
	t.Helper()
	return FileStructuralFingerprint(parseKotlinSnippet(t, src))
}

func javaFingerprint(t *testing.T, src string) string {
	t.Helper()
	return FileStructuralFingerprint(parseJavaSnippet(t, src))
}

func symbolSignatureOf(t *testing.T, f *File, name string) string {
	t.Helper()
	symbols, _ := indexFileForFingerprint(f)
	for _, s := range symbols {
		if s.Name == name {
			return s.Signature
		}
	}
	t.Fatalf("symbol %q not found", name)
	return ""
}

func TestFileStructuralFingerprint_JavaFieldType(t *testing.T) {
	str := javaFingerprint(t, "public class A { public String name; }")
	boxed := javaFingerprint(t, "public class A { public Integer name; }")
	if str == boxed {
		t.Fatal("Java field type change must move the fingerprint")
	}
	prim := javaFingerprint(t, "public class A { public int name; }")
	if prim == boxed || prim == str {
		t.Fatal("primitive vs boxed field type must move the fingerprint")
	}
	if javaFingerprint(t, "public class A { public List<String> name; }") ==
		javaFingerprint(t, "public class A { public List<Integer> name; }") {
		t.Fatal("generic argument change on a Java field must move the fingerprint")
	}
	if javaFingerprint(t, "public class A { public int name; }") ==
		javaFingerprint(t, "public class A { public int[] name; }") {
		t.Fatal("array field type must move the fingerprint")
	}
	if javaFingerprint(t, "public class A { public int name; }") ==
		javaFingerprint(t, "public class A { public int name[]; }") {
		t.Fatal("C-style array declarator must move the fingerprint")
	}

	// Initializer-only and whitespace-only edits are not structural.
	if javaFingerprint(t, "public class A { public String name = \"a\"; }") !=
		javaFingerprint(t, "public class A { public String name = \"b\"; }") {
		t.Fatal("field initializer edit must stay stable")
	}
	if javaFingerprint(t, "public class A { public Map<String,Integer> name; }") !=
		javaFingerprint(t, "public class A { public  Map< String ,  Integer >   name; }") {
		t.Fatal("whitespace inside a field type must stay stable")
	}
}

func TestFileStructuralFingerprint_JavaMethodTypes(t *testing.T) {
	base := javaFingerprint(t, "class A { String f(int x) { return null; } }")
	if base == javaFingerprint(t, "class A { String f(long x) { return null; } }") {
		t.Fatal("Java parameter type change must move the fingerprint")
	}
	if base == javaFingerprint(t, "class A { Integer f(int x) { return null; } }") {
		t.Fatal("Java return type change must move the fingerprint")
	}
	if base == javaFingerprint(t, "class A { void f(int x) { } }") {
		t.Fatal("Java return type to void must move the fingerprint")
	}
	if javaFingerprint(t, "class A { void f(String... x) { } }") == javaFingerprint(t, "class A { void f(String x) { } }") {
		t.Fatal("varargs vs plain parameter must move the fingerprint")
	}
	if javaFingerprint(t, "class A { void f(List<String> x, int y) { } }") ==
		javaFingerprint(t, "class A { void f(int y, List<String> x) { } }") {
		t.Fatal("parameter reordering must move the fingerprint")
	}
	// Parameter names, modifiers and bodies are not part of the type.
	if base != javaFingerprint(t, "class A { String f(final int x) { return \"s\"; } }") {
		t.Fatal("body literal and final modifier edits must stay stable")
	}
	if javaFingerprint(t, "class A { Map<String,Integer> f(List<String> x) { return null; } }") !=
		javaFingerprint(t, "class A { Map< String , Integer >\n    f( List< String >   x ) { return null; } }") {
		t.Fatal("whitespace inside method types must stay stable")
	}
}

func TestFileStructuralFingerprint_JavaConstructorTypes(t *testing.T) {
	if javaFingerprint(t, "class A { A(int x) { } }") == javaFingerprint(t, "class A { A(long x) { } }") {
		t.Fatal("Java constructor parameter type change must move the fingerprint")
	}
	if javaFingerprint(t, "class A { A(int x) { } }") != javaFingerprint(t, "class A { A( int  x ) { return; } }") {
		t.Fatal("Java constructor whitespace/body edit must stay stable")
	}
}

func TestFileStructuralFingerprint_KotlinFunctionParamTypes(t *testing.T) {
	base := ktFingerprint(t, "fun f(x: Int): String = \"\"")
	if base == ktFingerprint(t, "fun f(x: Long): String = \"\"") {
		t.Fatal("parameter type change must move the fingerprint")
	}
	if ktFingerprint(t, "fun f(x: Int?): String = \"\"") == base {
		t.Fatal("parameter nullability change must move the fingerprint")
	}
	if ktFingerprint(t, "fun f(x: List<Int?>) {}") == ktFingerprint(t, "fun f(x: List<Int>) {}") {
		t.Fatal("generic argument nullability in a parameter must move the fingerprint")
	}
	if ktFingerprint(t, "fun f(x: String) {}") == ktFingerprint(t, "fun f(vararg x: String) {}") {
		t.Fatal("vararg must move the fingerprint")
	}
	if ktFingerprint(t, "fun f(x: () -> Unit) {}") == ktFingerprint(t, "fun f(x: suspend () -> Unit) {}") {
		t.Fatal("suspend function type parameter must move the fingerprint")
	}
	if ktFingerprint(t, "fun f(a: Int, b: String) {}") == ktFingerprint(t, "fun f(a: String, b: Int) {}") {
		t.Fatal("parameter reordering must move the fingerprint")
	}
	if ktFingerprint(t, "fun Foo.f() {}") == ktFingerprint(t, "fun Bar.f() {}") {
		t.Fatal("extension receiver change must move the fingerprint")
	}
}

func TestFileStructuralFingerprint_KotlinFunctionReturnType(t *testing.T) {
	str := ktFingerprint(t, "fun f(x: Int): String = \"\"")
	nullable := ktFingerprint(t, "fun f(x: Int): String? = null")
	if str == nullable {
		t.Fatal("return type nullability change must move the fingerprint")
	}
	if str == ktFingerprint(t, "fun f(x: Int): CharSequence = \"\"") {
		t.Fatal("return type change must move the fingerprint")
	}
	if ktFingerprint(t, "fun f(): List<String> = emptyList()") == ktFingerprint(t, "fun f(): List<String?> = emptyList()") {
		t.Fatal("generic argument nullability in a return type must move the fingerprint")
	}
	// Block-body functions carry the type after the parameter list too.
	if ktFingerprint(t, "fun f(): Int { return 1 }") == ktFingerprint(t, "fun f(): Long { return 1 }") {
		t.Fatal("block-body return type change must move the fingerprint")
	}
	if ktFingerprint(t, "fun f(): Int { return 1 }") == ktFingerprint(t, "fun f() { }") {
		t.Fatal("adding an explicit return type must move the fingerprint")
	}
}

func TestFileStructuralFingerprint_KotlinFunctionStable(t *testing.T) {
	base := ktFingerprint(t, "fun f(x: Int): String = \"a\"")
	for name, src := range map[string]string{
		"body edit":           "fun f(x: Int): String = \"b\"",
		"block body":          "fun f(x: Int): String { return \"c\" }",
		"whitespace in sig":   "fun  f(  x :   Int  )  :   String   =  \"a\"",
		"default value added": "fun f(x: Int = 3): String = \"a\"",
	} {
		if got := ktFingerprint(t, src); got != base {
			t.Errorf("%s must not move the fingerprint", name)
		}
	}
	if ktFingerprint(t, "fun g() = 1") != ktFingerprint(t, "fun g() = \"s\"") {
		t.Fatal("inferred return type is body-like and must stay stable")
	}
	if ktFingerprint(t, "class C { fun f(m: Map<String,Int?>?): List<Int>? = null }") !=
		ktFingerprint(t, "class C { fun f(m: Map< String ,  Int? >? ): List< Int >? = null }") {
		t.Fatal("whitespace inside generic type arguments must stay stable")
	}
}

func TestFunctionSignatures_KeepArityPrefix(t *testing.T) {
	kt := parseKotlinSnippet(t, "class C { fun f(a: Int, b: String?): Long = 0\n fun g() = 1 }")
	if got, want := symbolSignatureOf(t, kt, "f"), "C#f/2(Int,String?):Long"; !strings.HasSuffix(got, want) {
		t.Errorf("f signature = %q, want suffix %q", got, want)
	}
	// Functions with nothing explicit keep exactly the legacy signature.
	if got := symbolSignatureOf(t, kt, "g"); !strings.HasSuffix(got, "C#g/0") {
		t.Errorf("g signature = %q, want legacy owner#name/arity", got)
	}
	java := parseJavaSnippet(t, "class A { String name; List<String> m(int a, String... b) { return null; } A(long c) { } }")
	if got := symbolSignatureOf(t, java, "name"); !strings.HasSuffix(got, "A#name/0:String") {
		t.Errorf("field signature = %q", got)
	}
	if got := symbolSignatureOf(t, java, "m"); !strings.HasSuffix(got, "A#m/2(int,String...):List<String>") {
		t.Errorf("method signature = %q", got)
	}
	if got := symbolSignatureOf(t, java, "A"); got == "" {
		t.Error("class signature must stay non-empty")
	}
}

func TestNormalizeTypeText(t *testing.T) {
	cases := map[string]string{
		"Map<String,  Int?>?":          "Map<String,Int?>?",
		"  List < String >  ":          "List<String>",
		"suspend   ()  ->  Unit":       "suspend()->Unit",
		"Foo<in  T,\n  out   R>":       "Foo<in T,out R>",
		"@Ann   Int":                   "@Ann Int",
		"java.util.Map . Entry<K , V>": "java.util.Map.Entry<K,V>",
		"":                             "",
	}
	for in, want := range cases {
		if got := normalizeTypeText(in); got != want {
			t.Errorf("normalizeTypeText(%q) = %q, want %q", in, got, want)
		}
	}
}
