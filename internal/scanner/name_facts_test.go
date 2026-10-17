package scanner

import (
	"reflect"
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
	if h := ParseSourceHeader([]byte("fun f() {}\npackage late\n")); h.Package != "" {
		t.Fatalf("package after a declaration = %q, want empty", h.Package)
	}
}
