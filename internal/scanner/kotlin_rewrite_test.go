package scanner

import (
	"fmt"
	"strings"
	"testing"
)

// sexp renders the named nodes of ft as an S-expression, the way tree-sitter
// prints trees.
func sexp(ft *FlatTree) string {
	var sb strings.Builder
	var walk func(n uint32)
	walk = func(n uint32) {
		sb.WriteString("(" + nodeTypeName(ft.Types[n]))
		for c := ft.FirstChildren[n]; c != 0; c = ft.NextSibs[c] {
			if ft.NodeIsNamed(c) {
				sb.WriteString(" ")
				walk(c)
			}
		}
		sb.WriteString(")")
	}
	walk(0)
	return sb.String()
}

// nodeTexts returns the source text of every node of the given type.
func nodeTexts(ft *FlatTree, src []byte, typ string) []string {
	var out []string
	for i := range ft.Types {
		if nodeTypeName(ft.Types[i]) == typ {
			out = append(out, string(src[ft.StartBytes[i]:ft.EndBytes[i]]))
		}
	}
	return out
}

func hasErrors(ft *FlatTree) bool {
	for i := range ft.Flags {
		if ft.Flags[i]&flatNodeFlagIsError != 0 && !inExplicitBackingField(ft, uint32(i)) {
			return true
		}
	}
	return false
}

func parentTypeOf(ft *FlatTree, src []byte, typ, text string) string {
	for i := range ft.Types {
		if nodeTypeName(ft.Types[i]) == typ && string(src[ft.StartBytes[i]:ft.EndBytes[i]]) == text {
			return nodeTypeName(ft.Types[ft.Parents[i]])
		}
	}
	return ""
}

// rewriteCases pin one rewrite each: the parse is clean and the checked nodes
// have the original text and the right parent.
var rewriteCases = []struct {
	name  string
	src   string
	check func(t *testing.T, ft *FlatTree, src []byte)
}{
	{
		name: "fun interface",
		src:  "fun interface Cb {\n  fun run(x: Int)\n}\nfun g() {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			want := "fun interface Cb {\n  fun run(x: Int)\n}"
			if got := nodeTexts(ft, src, "class_declaration"); len(got) != 1 || got[0] != want {
				t.Errorf("class_declaration = %q, want %q", got, want)
			}
			if p := parentTypeOf(ft, src, "function_declaration", "fun run(x: Int)"); p != "class_body" {
				t.Errorf("run parent = %q", p)
			}
		},
	},
	{
		name: "fun interface with modifier",
		src:  "package p\n\npublic fun interface G<T> : Base {\n  fun run(t: T)\n}\nfun g() {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "class_declaration"); len(got) != 1 || !strings.HasPrefix(got[0], "public fun interface G<T>") {
				t.Errorf("class_declaration = %q", got)
			}
		},
	},
	{
		name: "range until",
		src:  "fun f() {\n  for (i in 0..<10) {}\n  val r = 1 ..< 5\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			got := nodeTexts(ft, src, "range_expression")
			if len(got) != 2 || got[0] != "0..<10" || got[1] != "1 ..< 5" {
				t.Errorf("range_expression = %q", got)
			}
		},
	},
	{
		name: "binary literal",
		src:  "val a = 0b101\nval b = 0B1_0\nfun g() {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "hex_literal"); len(got) != 2 || got[0] != "0b101" || got[1] != "0B1_0" {
				t.Errorf("literals = %q", got)
			}
		},
	},
	{
		name: "context parameters on members keep the class body",
		src:  "class C {\n  context(a: A)\n  fun f() {}\n  context(b: B<X>, c: C)\n  fun g() {}\n  fun h() {}\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			got := nodeTexts(ft, src, "function_declaration")
			if len(got) != 3 || got[0] != "context(a: A)\n  fun f() {}" || got[1] != "context(b: B<X>, c: C)\n  fun g() {}" || got[2] != "fun h() {}" {
				t.Errorf("function_declaration = %q", got)
			}
			if p := parentTypeOf(ft, src, "function_declaration", "fun h() {}"); p != "class_body" {
				t.Errorf("h parent = %q", p)
			}
		},
	},
	{
		name: "context clause before modifiers and a class",
		src:  "context(A)\nprivate suspend fun f() {}\ncontext(A)\n@Ann\nclass K {\n  fun m() {}\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "function_declaration"); len(got) != 2 || got[0] != "context(A)\nprivate suspend fun f() {}" {
				t.Errorf("function_declaration = %q", got)
			}
			if got := nodeTexts(ft, src, "class_declaration"); len(got) != 1 || !strings.HasPrefix(got[0], "context(A)\n@Ann\nclass K") {
				t.Errorf("class_declaration = %q", got)
			}
		},
	},
	{
		name: "when guard keeps the guard expression",
		src:  "fun f(x: Any) = when (x) {\n  is Foo if x.ok -> 1\n  is Bar, is Baz if y > 1 -> 2\n  else -> 3\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "when_entry"); len(got) != 3 || got[0] != "is Foo if x.ok -> 1" {
				t.Errorf("when_entry = %q", got)
			}
		},
	},
	{
		name: "multi dollar strings",
		src:  "val a = $$\"\"\"{ \"k\": $$name, \"p\": $price, \"c\": ${'$'}{x} }\"\"\"\nval b = $$\"$x $$y $${z + 1}\"\nfun g() {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "string_literal"); len(got) < 2 {
				t.Errorf("string_literal = %q", got)
			}
		},
	},
	{
		name: "name based destructuring",
		src:  "fun f(pair: P, m: Map<A, B>) {\n  (val a, val b) = pair\n  [val c, var d] = pair\n  val [e, g] = pair\n  for ((val k, val v) in m) {}\n  m.forEach { (val k, val v) -> println(k) }\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			got := nodeTexts(ft, src, "multi_variable_declaration")
			if len(got) != 5 {
				t.Errorf("multi_variable_declaration = %q", got)
			}
		},
	},
	{
		name: "bracket lambda destructuring",
		src:  "fun f(l: List<P>) {\n  l.filter { [a, b] -> a > b }\n  l.mapIndexed { i, (c, d): P -> c }\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "lambda_literal"); len(got) != 2 {
				t.Errorf("lambda_literal = %q", got)
			}
		},
	},
	{
		name: "object literal delegation keeps the body",
		src:  "val o = object : Iface by impl() {\n  override fun f() {}\n}\nval p = object : A, Iface by impl {\n  fun h() {}\n}\nfun g() {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "explicit_delegation"); len(got) != 2 || got[0] != "Iface by impl()" || got[1] != "Iface by impl" {
				t.Errorf("explicit_delegation = %q", got)
			}
			if got := nodeTexts(ft, src, "class_body"); len(got) != 2 {
				t.Errorf("class_body = %q", got)
			}
			if p := parentTypeOf(ft, src, "function_declaration", "override fun f() {}"); p != "class_body" {
				t.Errorf("f parent = %q", p)
			}
			if got := nodeTexts(ft, src, "call_expression"); len(got) != 1 || got[0] != "impl()" {
				t.Errorf("call_expression = %q", got)
			}
		},
	},
	{
		name: "class delegation keeps the body",
		src:  "class A(val x: X) : B by foo(x) {\n  fun f() {}\n}\nclass C : D by e\nfun g() {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "explicit_delegation"); len(got) < 1 || got[0] != "B by foo(x)" {
				t.Errorf("explicit_delegation = %q", got)
			}
			if p := parentTypeOf(ft, src, "function_declaration", "fun f() {}"); p != "class_body" {
				t.Errorf("f parent = %q", p)
			}
		},
	},
	{
		name: "annotated supertypes",
		src:  "interface P : @Suppress(\"X\") a.B<V> {\n  fun f()\n}\nclass Q : Vm(), @Suppress(\"X\") a.B<V> {\n  fun g() {}\n}\nfun h() {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "class_declaration"); len(got) != 2 {
				t.Errorf("class_declaration = %q", got)
			}
			if p := parentTypeOf(ft, src, "function_declaration", "fun g() {}"); p != "class_body" {
				t.Errorf("g parent = %q", p)
			}
		},
	},
	{
		name: "as on its own line",
		src:  "fun f(a: Any) {\n  val x = a\n    as String\n  val y = a\n    as? Int\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "as_expression"); len(got) != 2 || got[0] != "a\n    as String" {
				t.Errorf("as_expression = %q", got)
			}
		},
	},
	{
		name: "when entries starting with in, !in and else",
		src:  "fun f(a: Int) = when (a) {\n  1 -> x\n  in 3..4 -> y\n  !in 5..6 -> z() // c\n  else -> w\n}\nfun g(a: Int) {\n  when (a) {\n    1 -> if (a > 0) b()\n    else -> c()\n  }\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "when_entry"); len(got) != 6 {
				t.Errorf("when_entry = %q", got)
			}
		},
	},
	{
		name: "leading dot number after a call",
		src:  "fun f() {\n  keyframes {\n    0f at 200\n    .9f at 500\n  }\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "real_literal"); len(got) != 2 || got[0] != "0f" {
				t.Errorf("real_literal = %q", got)
			}
		},
	},
	{
		name: "assignment to a call chain",
		src:  "fun f() {\n  verify(m).x = 1\n  // c.\n  a.b().c += foo(2) + 3\n  q()[0] = 5\n  g().h = if (z) 1 else 2\n  v.findViewById<Button>(i)?.isEnabled = false\n  (view.parent as View).touchDelegate = null\n  map.getOrPut(k) { mutableMapOf() }[i] = Unit\n  if (c) y().z = 1 else w().z = 2\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			got := nodeTexts(ft, src, "assignment")
			want := []string{
				"verify(m).x = 1",
				"a.b().c += foo(2) + 3",
				"q()[0] = 5",
				"g().h = if (z) 1 else 2",
				"v.findViewById<Button>(i)?.isEnabled = false",
				"(view.parent as View).touchDelegate = null",
				"map.getOrPut(k) { mutableMapOf() }[i] = Unit",
				"y().z = 1",
				"w().z = 2",
			}
			if len(got) != len(want) {
				t.Fatalf("assignment = %q, want %q", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("assignment[%d] = %q, want %q", i, got[i], want[i])
				}
			}
			for _, p := range nodeTexts(ft, src, "directly_assignable_expression") {
				if strings.HasPrefix(p, "(") && !strings.HasPrefix(p, "(view") {
					t.Errorf("assignment target kept the added parenthesis: %q", p)
				}
			}
		},
	},
	{
		name: "assignment to a call chain that spans lines",
		src:  "class H {\n  fun f(view: View) {\n    view\n      .animate()\n      .alpha(if (b) 1F else 0F)\n      .interpolator = X()\n    other\n      ?.get(1)\n      ?.value = 2\n  }\n\n  companion object {\n    private const val D = 1\n  }\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			got := nodeTexts(ft, src, "assignment")
			want := []string{
				"view\n      .animate()\n      .alpha(if (b) 1F else 0F)\n      .interpolator = X()",
				"other\n      ?.get(1)\n      ?.value = 2",
			}
			if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
				t.Fatalf("assignment = %q, want %q", got, want)
			}
			if n := len(nodeTexts(ft, src, "companion_object")); n != 1 {
				t.Errorf("companion_object nodes = %d, want 1", n)
			}
		},
	},
	{
		name: "negated identifier starting a when condition",
		src:  "fun f(a: Int) = when {\n  a == 1 -> x\n  !isDebug && c -> y\n  !isOnPage(a, b) -> z\n  else -> w\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "when_condition"); len(got) != 3 || got[1] != "!isDebug && c" || got[2] != "!isOnPage(a, b)" {
				t.Errorf("when_condition = %q", got)
			}
			if got := nodeTexts(ft, src, "parenthesized_expression"); len(got) != 0 {
				t.Errorf("parenthesized_expression = %q", got)
			}
		},
	},
	{
		name: "trailing commas the grammar rejects",
		src:  "fun f(\n  a: Foo<\n    A,\n    B,\n  >,\n  cb: (\n    X,\n    Y,\n  ) -> Unit,\n) {\n  m[\n    k,\n  ] = 1\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "function_declaration"); len(got) != 1 {
				t.Errorf("function_declaration = %q", got)
			}
		},
	},
	{
		name: "modifier words used as names",
		src:  "fun f(open: Open, list: List<A>) {\n  open.eventSink(1)\n  open += 2\n  list.forEach { annotation -> use(annotation) }\n  override.x()\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "lambda_literal"); len(got) != 1 {
				t.Errorf("lambda_literal = %q", got)
			}
		},
	},
	{
		name: "callable references with nullable and generic types",
		src:  "val a = list.map(Result<Item?>::getOrNull)\nval b = x.takeUnless(String?::isNullOrBlank)\nfun g() {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "callable_reference"); len(got) != 2 || got[0] != "Result<Item?>::getOrNull" || got[1] != "String?::isNullOrBlank" {
				t.Errorf("callable_reference = %q", got)
			}
		},
	},
	{
		name: "function types with qualified receivers",
		src:  "fun f(\n  a: Circuit.Builder.() -> Unit = {},\n  b: Transition.Segment<List<Color>>.() -> Spec,\n  c: (A.B).(String) -> Unit,\n  d: Uri?.(Res) -> String?,\n) {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "function_type"); len(got) != 4 {
				t.Errorf("function_type = %q", got)
			}
		},
	},
	{
		name: "non ASCII identifiers",
		src:  "val 変数 = 1\nval π = 3.14\nfun `テスト`() {}\nval s = \"日本語\" // 注釈\nfun g() {}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "simple_identifier"); len(got) < 2 || got[0] != "変数" || got[1] != "π" {
				t.Errorf("simple_identifier = %q", got)
			}
		},
	},
	{
		name: "constructor after a doc comment",
		src:  "class A\n/** @param x doc */\nconstructor(x: Int) : B(x) {\n  fun f() {}\n}\nclass C\n/**\n * Doc.\n */\nprivate constructor(y: Int) {\n  fun g() {}\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if p := parentTypeOf(ft, src, "function_declaration", "fun g() {}"); p != "class_body" {
				t.Errorf("g parent = %q", p)
			}
		},
	},
	{
		name: "consecutive annotations with arguments before a class",
		src:  "@A(X::class)\n@B(Y::class)\n@C(Z)\nclass K\n@Inject\nconstructor(a: A) : S {\n  fun f() {}\n}\n@Q(\"public\") private interface Pk {\n  val v: Int\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			got := nodeTexts(ft, src, "class_declaration")
			if len(got) != 2 || !strings.HasPrefix(got[0], "@A(X::class)\n@B(Y::class)\n@C(Z)\nclass K") || !strings.HasPrefix(got[1], "@Q(\"public\") private interface Pk") {
				t.Errorf("class_declaration = %q", got)
			}
		},
	},
	{
		name: "line-leading sign starts a statement",
		src:  "fun f(a: Int, b: Int) {\n    val x = a\n        + b\n    foo(a)\n    - b\n    /* c */ -a\n    a // c\n    + 1\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "additive_expression"); len(got) != 0 {
				t.Errorf("additive_expression = %q, want none", got)
			}
			if got := nodeTexts(ft, src, "prefix_expression"); len(got) != 4 {
				t.Errorf("prefix_expression = %q, want 4", got)
			}
		},
	},
	{
		name: "line-leading sign continues where Kotlin continues",
		src:  "fun f(a: Int, b: Int, c: Boolean) {\n    val p = (a\n        + b)\n    val q = g(a,\n        -b)\n    val r = g(a\n        - b)\n    val s = a +\n        b\n    val t = a -\n        -b\n    val u = \"${a\n        - b}\"\n    if (c)\n        -a\n    val v = if (c) a else\n        -b\n    h { x ->\n        x\n    }\n}\n",
		check: func(t *testing.T, ft *FlatTree, src []byte) {
			if got := nodeTexts(ft, src, "additive_expression"); len(got) != 5 {
				t.Errorf("additive_expression = %q, want 5", got)
			}
		},
	},
}

func TestKotlinRewriteCases(t *testing.T) {
	for _, tc := range rewriteCases {
		t.Run(tc.name, func(t *testing.T) {
			ft, src := parseSynthetic(t, tc.src)
			if hasErrors(ft) {
				t.Fatalf("parse has ERROR/MISSING nodes: %s", sexp(ft))
			}
			for c := ft.FirstChildren[0]; c != 0; c = ft.NextSibs[c] {
				switch nodeTypeName(ft.Types[c]) {
				case "prefix_expression", "infix_expression", "call_expression":
					t.Errorf("top-level %s: %s", nodeTypeName(ft.Types[c]), sexp(ft))
				}
			}
			tc.check(t, ft, src)
		})
	}
}

// Rows and columns must match the original source, and every node must stay
// inside its parent, whatever the rewrites did.
func TestKotlinRewritePositionsMatchOriginal(t *testing.T) {
	for _, tc := range rewriteCases {
		t.Run(tc.name, func(t *testing.T) {
			ft, src := parseSynthetic(t, tc.src)
			for i := range ft.Types {
				off := ft.StartBytes[i]
				row, col := 0, 0
				for _, b := range src[:off] {
					if b == '\n' {
						row++
						col = 0
					} else {
						col++
					}
				}
				if int(ft.StartRows[i]) != row || int(ft.StartCols[i]) != col {
					t.Fatalf("%s at byte %d: row/col = %d/%d, want %d/%d", nodeTypeName(ft.Types[i]), off, ft.StartRows[i], ft.StartCols[i], row, col)
				}
				if p := ft.Parents[i]; i != 0 && (ft.StartBytes[p] > off || ft.EndBytes[p] < ft.EndBytes[i]) {
					t.Fatalf("%s [%d,%d) escapes parent %s [%d,%d)", nodeTypeName(ft.Types[i]), off, ft.EndBytes[i], nodeTypeName(ft.Types[p]), ft.StartBytes[p], ft.EndBytes[p])
				}
			}
		})
	}
}

// The rewrites keep the byte length and leave strings and comments alone.
func TestKotlinRewriteLeavesNonCodeAlone(t *testing.T) {
	cases := []string{
		"val a = \"0..<10 and 0b101 and (val a, val b) = x\"\n",
		"// in 3 -> x\n// as T\n/* context(A)\nfun f() */\nfun g() {}\n",
		"val s = \"\"\"\n  as T\n  in 1 -> x\n  .5f\n  fun interface X\n\"\"\"\n",
		"val t = \"a ${1 + 2} b\" + \"fun interface\"\n",
		"fun `fun interface`() {}\n",
		"val c = '\"' // \" 0..<1\n",
	}
	for _, src := range cases {
		out, rw := rewriteKotlin([]byte(src))
		if rw != nil || string(out) != src {
			t.Errorf("rewrote non-code text:\n%q\n%q", src, out)
		}
	}
	for _, tc := range rewriteCases {
		out, _ := rewriteKotlin([]byte(tc.src))
		if len(out) != len(tc.src) {
			t.Errorf("%s: length %d -> %d", tc.name, len(tc.src), len(out))
		}
	}
}

// Valid code that looks like a rewrite trigger must parse the same as before.
func TestKotlinRewriteSkipsOrdinaryCode(t *testing.T) {
	cases := []string{
		"class A(val x: Int) { fun f() = x }\nfun g() { val y = if (a) b else c }\n",
		"fun f(x: Any) = when (x) {\n  is Foo -> 1\n  2 -> if (x == 1) 3 else 4\n}\n",
		"val x by lazy { 1 }\nclass A { val y by lazy { 2 } }\nfun g() {}\n",
		"fun f() {\n  a.b = 1\n  a[0] = 1\n  c().d\n  val v = e().f\n  g(h = i().j)\n}\n",
		"fun f(a: Int, b: Int) = a == b || a != b\nfun g() { x == y() }\n",
		"class A(val a: Int = 1, var b: String = \"\") : Base(a)\n",
		"fun f() {\n  foo(a,\n    b,\n  )\n  val l = listOf(1,\n    2)\n}\n",
	}
	for _, src := range cases {
		out, rw := rewriteKotlin([]byte(src))
		if rw != nil || string(out) != src {
			t.Errorf("rewrote ordinary code:\n%q\n%q", src, out)
		}
		ft, _ := parseSynthetic(t, src)
		if hasErrors(ft) {
			t.Errorf("ordinary code has errors: %q", src)
		}
	}
}

func TestKotlinRewriteSexpExamples(t *testing.T) {
	cases := []struct{ src, want string }{
		{
			"fun f(p: P) {\n  (val a, val b) = p\n}\n",
			"(source_file (function_declaration (simple_identifier) (function_value_parameters (parameter (simple_identifier) (user_type (type_identifier)))) (function_body (statements (property_declaration (binding_pattern_kind) (multi_variable_declaration (variable_declaration (simple_identifier)) (variable_declaration (simple_identifier))) (simple_identifier))))))",
		},
		{
			"val o = object : I by f() {\n  fun g() {}\n}\n",
			"(source_file (property_declaration (binding_pattern_kind) (variable_declaration (simple_identifier)) (object_literal (delegation_specifier (explicit_delegation (user_type (type_identifier)) (call_expression (simple_identifier) (call_suffix (value_arguments))))) (class_body (function_declaration (simple_identifier) (function_value_parameters) (function_body))))))",
		},
	}
	for _, tc := range cases {
		ft, _ := parseSynthetic(t, tc.src)
		if got := sexp(ft); got != tc.want {
			t.Errorf("sexp(%q)\n got  %s\n want %s", tc.src, got, tc.want)
		}
	}
	_ = fmt.Sprint
}

func TestKotlinRewriteContextParametersNode(t *testing.T) {
	ft, src := parseSynthetic(t, "context(table: Map<String, String>, _: Log)\nfun f(a: Int) {}\ncontext(Plain)\nfun g() {}\n")
	got := nodeTexts(ft, src, ContextParametersNodeType)
	if len(got) != 1 || got[0] != "(table: Map<String, String>, _: Log)" {
		t.Fatalf("context_parameters = %q", got)
	}
	if params := nodeTexts(ft, src, "parameter"); len(params) != 3 || params[0] != "table: Map<String, String>" {
		t.Errorf("parameter = %q", params)
	}
	if fns := nodeTexts(ft, src, "function_declaration"); len(fns) != 2 || !strings.HasPrefix(fns[0], "context(table") {
		t.Errorf("function_declaration = %q", fns)
	}
}
