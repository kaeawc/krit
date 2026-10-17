package oracle

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeKt(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func demotedBy(t *testing.T, newFile string, hitFiles map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	var hits []*CacheEntry
	for name, body := range hitFiles {
		hits = append(hits, &CacheEntry{FilePath: writeKt(t, dir, name, body)})
	}
	miss := writeKt(t, dir, "New.kt", newFile)
	kept, misses := demoteNameCollidingHits(InvocationOptions{Backend: BackendKAA}, hits, []string{miss}, nil, false)
	var demoted []string
	for _, m := range misses {
		if m != miss {
			demoted = append(demoted, filepath.Base(m))
		}
	}
	sort.Strings(demoted)
	if len(kept)+len(demoted) != len(hits) {
		t.Fatalf("hits not conserved: kept=%d demoted=%d total=%d", len(kept), len(demoted), len(hits))
	}
	return demoted
}

func TestDemoteNameCollidingHits(t *testing.T) {
	cases := []struct {
		name string
		new  string
		hits map[string]string
		want []string
	}{
		{
			name: "same package overload demotes caller",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun usePick() = pick(1)!!\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "unrelated new file demotes nothing",
			new:  "package p\nfun somethingElse(): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun usePick() = pick(1)!!\n"},
		},
		{
			name: "other package without import is not demoted",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nfun usePick() = pick(1)\n"},
		},
		{
			name: "explicit import demotes",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nimport p.pick\nfun usePick() = pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "wildcard import demotes",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nimport p.*\nfun usePick() = pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "qualified use demotes",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nfun usePick() = p.pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "longer identifier is not a mention",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun pickAll() = picker()\n"},
		},
		{
			name: "private declaration is invisible",
			new:  "package p\nprivate fun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun usePick() = pick(1)\n"},
		},
		{
			name: "member declaration is not top level",
			new:  "package p\nclass Box { fun pick(x: Int): Int = 1 }\n",
			hits: map[string]string{"Use.kt": "package p\nfun usePick() = pick(1)\n"},
		},
		{
			name: "extension and property and type",
			new:  "package p\nfun String.pick(): Int = 1\nval limit = 3\nclass Widget\n",
			hits: map[string]string{
				"A.kt": "package p\nfun a() = \"x\".pick()\n",
				"B.kt": "package p\nfun b() = limit\n",
				"C.kt": "package p\nfun c(w: Widget) = w\n",
				"D.kt": "package p\nfun d() = 1\n",
			},
			want: []string{"A.kt", "B.kt", "C.kt"},
		},
		{
			name: "operator plus used only through syntax",
			new:  "package p\noperator fun Foo.plus(o: Foo): Foo = o\n",
			hits: map[string]string{
				"Use.kt":   "package p\nfun u(a: Any) = (a + a)!!\n",
				"Other.kt": "package p\nfun v() = 1\n",
			},
			want: []string{"Use.kt"},
		},
		{
			name: "operator get used only through indexing",
			new:  "package p\noperator fun Foo.get(i: Int): Int = i\n",
			hits: map[string]string{
				"Use.kt":   "package p\nfun u(f: Foo) = f[0]\n",
				"Other.kt": "package p\nfun v() = 1\n",
			},
			want: []string{"Use.kt"},
		},
		{
			name: "operator fun in imported package demotes syntax user",
			new:  "package p\noperator fun Foo.plus(o: Foo): Foo = o\n",
			hits: map[string]string{"Use.kt": "package q\nimport p.plus\nfun u(a: Foo) = a + a\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "plain fun named plus is not syntax-gated",
			new:  "package p\nfun Foo.plus(o: Foo): Foo = o\n",
			hits: map[string]string{"Use.kt": "package p\nfun u(a: Int) = a + a\n"},
		},
		{
			name: "destructuring reaches componentN",
			new:  "package p\noperator fun Foo.component1(): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun u(f: Foo) { val (a) = f }\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "delegate reaches getValue",
			new:  "package p\noperator fun Foo.getValue(t: Any?, p: Any?): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nval x: Int by Foo()\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "operator in other package without import is invisible",
			new:  "package p\noperator fun Foo.plus(o: Foo): Foo = o\n",
			hits: map[string]string{"Use.kt": "package q\nfun u(a: Foo) = a + a\n"},
		},
		{
			name: "import alias demotes",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nimport p.pick as choose\nfun u() = choose(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "no package header sees only root package declarations",
			new:  "fun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "fun u() = pick(1)\n", "Pkg.kt": "package q\nfun u() = pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "root package explicit import demotes",
			new:  "fun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nimport pick\nfun u() = pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "sub package does not see parent package",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p.sub\nfun u() = pick(1)\n"},
		},
		{
			name: "default import package",
			new:  "package kotlin.collections\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nfun u() = pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "longer qualified prefix is not the package",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package q\nfun u() = a.p.pick(1)\n"},
		},
		{
			name: "mention in line comment does not demote",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\n// pick the best\nfun u() = 1\n"},
		},
		{
			name: "mention in block comment and KDoc does not demote",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\n/** pick */\n/* nested /* pick */ pick */\nfun u() = 1\n"},
		},
		{
			name: "string template mention still demotes",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun u() = \"${pick(1)}\"\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "plain string text does not demote",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nfun u() = \"pick // me\"\n"},
		},
		{
			name: "comment marker inside string does not hide later code",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "package p\nval url = \"http://x\"\nfun u() = pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "package inside block comment is ignored",
			new:  "package p\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "/* package p */\npackage q\nfun u() = pick(1)\n"},
		},
		{
			name: "backtick package",
			new:  "package `p`\nfun pick(x: Int): Int = 1\n",
			hits: map[string]string{"Use.kt": "@file:Suppress(\"x\")\npackage `p`\nfun u() = pick(1)\n"},
			want: []string{"Use.kt"},
		},
		{
			name: "adjacent string templates record both names",
			new:  "package p\nfun foo() = 1\n",
			hits: map[string]string{
				"A.kt": "package p\nval s = \"$a$foo\"\n",
				"B.kt": "package p\nval s = \"$foo$a\"\n",
				"C.kt": "package p\nval s = \"$foo$\"\n",
				"D.kt": "package p\nval s = \"$$foo\"\n",
				"E.kt": "package p\nval s = \"$a$b\"\n",
			},
			want: []string{"A.kt", "B.kt", "C.kt", "D.kt"},
		},
		{
			name: "raw string closing with extra quotes does not hide code",
			new:  "package p\nfun foo() = 1\n",
			hits: map[string]string{
				"A.kt": "package p\nval s = \"\"\"a\"\"\"\" + foo()\n",
				"B.kt": "package p\nval s = \"\"\"a\"\"\"\" + \"foo\"\n",
			},
			want: []string{"A.kt"},
		},
		{
			name: "contains operator through unusually spaced in",
			new:  "package p\noperator fun Foo.contains(x: Int): Boolean = true\n",
			hits: map[string]string{
				"A.kt": "package p\nfun f(a: Foo) = 1 in\n a\n",
				"B.kt": "package p\nfun f(a: Foo) = when (1) {\n\tin a -> 1\n\telse -> 0\n}\n",
				"C.kt": "package p\nfun f(a: Foo) = 1 in(a)\n",
				"D.kt": "package p\nfun f() = 1\n",
			},
			want: []string{"A.kt", "B.kt", "C.kt"},
		},
		{
			name: "iterator operator through for without space",
			new:  "package p\noperator fun Foo.iterator(): Iterator<Int> = TODO()\n",
			hits: map[string]string{
				"A.kt": "package p\nfun f(a: Foo) { for(x in a) {} }\n",
				"B.kt": "package p\nfun f() = 1\n",
			},
			want: []string{"A.kt"},
		},
		{
			name: "delegate operator through by on its own line",
			new:  "package p\noperator fun Foo.getValue(t: Any?, p: Any?): Int = 1\n",
			hits: map[string]string{
				"A.kt": "package p\nval x: Int\n\t\tby foo\n",
				"B.kt": "package p\nval x = 1\n",
			},
			want: []string{"A.kt"},
		},
		{
			name: "component operator through typed and spaced destructuring",
			new:  "package p\noperator fun Foo.component1(): Int = 1\n",
			hits: map[string]string{
				"A.kt": "package p\nval f = { (x, y): Pair<Int, Int> -> x }\n",
				"B.kt": "package p\nfun f(a: List<Int>) { for ( (x, y) in a) {} }\n",
				"C.kt": "package p\nval g = { (x, y)\n -> x }\n",
				"D.kt": "package p\nfun f() = 1\n",
			},
			want: []string{"A.kt", "B.kt", "C.kt"},
		},
		{
			name: "invoke operator through trailing lambda",
			new:  "package p\noperator fun Foo.invoke(f: () -> Unit) {}\n",
			hits: map[string]string{
				"A.kt": "package p\nfun f(x: Foo) = x { }\n",
				"B.kt": "package p\nconst val k = 1\n",
			},
			want: []string{"A.kt"},
		},
		{
			name: "typealias and fun interface",
			new:  "package p\ntypealias Alias = List<Int>\nfun interface Fi { fun run() }\n",
			hits: map[string]string{
				"A.kt": "package p\nfun a(x: Alias) = x\n",
				"B.kt": "package p\nfun b(x: Fi) = x\n",
				"C.kt": "package p\nfun c() = run()\n",
			},
			want: []string{"A.kt", "B.kt"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := demotedBy(t, tc.new, tc.hits)
			if len(got) != len(tc.want) {
				t.Fatalf("demoted %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("demoted %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestDemoteNameCollidingHitsSkipsWholeCompilationBackend(t *testing.T) {
	dir := t.TempDir()
	hit := &CacheEntry{FilePath: writeKt(t, dir, "Use.kt", "package p\nfun u() = pick(1)\n")}
	miss := writeKt(t, dir, "New.kt", "package p\nfun pick(x: Int): Int = 1\n")
	hits, misses := demoteNameCollidingHits(InvocationOptions{Backend: BackendFIR}, []*CacheEntry{hit}, []string{miss}, nil, false)
	if len(hits) != 1 || len(misses) != 1 {
		t.Fatalf("FIR hits=%d misses=%d, want untouched", len(hits), len(misses))
	}
}

func TestDemoteNameCollidingHitsSkipsCrashedAndFailsClosed(t *testing.T) {
	dir := t.TempDir()
	crashed := &CacheEntry{FilePath: writeKt(t, dir, "Crashed.kt", "package p\nfun u() = pick(1)\n"), Crashed: true}
	missing := &CacheEntry{FilePath: filepath.Join(dir, "Gone.kt")}
	clean := &CacheEntry{FilePath: writeKt(t, dir, "Clean.kt", "package p\nfun u() = 1\n")}
	miss := writeKt(t, dir, "New.kt", "package p\nfun pick(x: Int): Int = 1\n")
	kept, misses := demoteNameCollidingHits(InvocationOptions{Backend: BackendKAA}, []*CacheEntry{crashed, missing, clean}, []string{miss}, nil, false)
	if len(kept) != 2 || kept[0] != crashed || kept[1] != clean {
		t.Fatalf("kept %v, want the crashed and clean hits", kept)
	}
	if len(misses) != 2 || misses[1] != missing.FilePath {
		t.Fatalf("misses %v, want New.kt plus the unreadable hit", misses)
	}
}
