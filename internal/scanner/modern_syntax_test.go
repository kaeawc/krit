package scanner

import (
	"strings"
	"testing"
)

// modernSyntaxCases are synthetic snippets of Kotlin 1.x-2.4 syntax. Each
// must parse without ERROR or MISSING nodes and without a misparsed
// top-level declaration, unless listed in modernSyntaxKnownGaps.
var modernSyntaxCases = []struct{ name, src string }{
	{"context parameters fun", "context(a: A, b: B)\nfun f() {}\n"},
	{"context receiver fun", "context(A)\nfun f() {}\n"},
	{"context class", "context(A)\nclass Foo {\n  fun g() {}\n}\n"},
	{"explicit backing field", "class C {\n  val x: List<Int>\n    field = mutableListOf()\n  fun g() {}\n}\n"},
	{"name based destructuring", "fun f(pair: P) {\n  (val a, val b) = pair\n  println(a + b)\n}\n"},
	{"name based destructuring square", "fun f(pair: P) {\n  [val a, val b] = pair\n}\n"},
	{"when guard", "fun f(x: Any) = when (x) {\n  is Foo if x.ok -> 1\n  else -> 2\n}\n"},
	{"multi dollar string", "val s = $$\"\"\"{ \"a\": $$name }\"\"\"\nval t = $$\"$$x\"\nfun g() {}\n"},
	{"non local break", "fun f() {\n  for (i in 1..3) {\n    run { if (i > 1) break else continue }\n  }\n}\n"},
	{"data object", "data object Foo\nfun g() {}\n"},
	{"value class", "@JvmInline\nvalue class Id(val v: Int)\nfun g() {}\n"},
	{"sealed interface", "sealed interface S {\n  data object A : S\n  data class B(val x: Int) : S\n}\nfun g() {}\n"},
	{"generic typealias", "typealias Handler<T> = (T) -> Unit\nfun g() {}\n"},
	{"fun interface", "fun interface Cb {\n  fun run(x: Int)\n}\nfun g() {}\n"},
	{"contract", "@OptIn(ExperimentalContracts::class)\nfun f(x: Any?) {\n  contract { returns() implies (x != null) }\n}\nfun g() {}\n"},
	{"expect actual", "expect class A {\n  fun f()\n}\nactual class B {\n  actual fun f() {}\n}\nfun g() {}\n"},
	{"use site targets", "class C(\n  @field:Ann val a: Int,\n  @get:Ann val b: Int,\n  @param:Ann val c: Int,\n  @set:Ann var d: Int,\n)\nfun g() {}\n"},
	{"enum entries", "enum class E(val v: Int) {\n  A(1), B(2);\n  fun f() = entries\n}\nfun g() {}\n"},
	{"operator fun", "class V {\n  operator fun plus(o: V): V = this\n  operator fun invoke() {}\n}\nfun g() {}\n"},
	{"suspend types", "val a: suspend () -> Unit = { }\nval b: suspend Foo.(Int) -> Unit = { }\nfun f(c: suspend (Int) -> String) {}\nfun g() {}\n"},
	{"inline modifiers", "inline fun <reified T> f(crossinline a: () -> Unit, noinline b: () -> Unit) {}\nfun g() {}\n"},
	{"labeled", "fun f() {\n  outer@ for (i in 1..3) {\n    for (j in 1..3) { if (j == 2) continue@outer; if (i == 3) break@outer }\n  }\n  listOf(1).forEach lit@{ if (it == 1) return@lit }\n}\nfun g() {}\n"},
	{"lambda destructuring", "fun f(m: Map<Int, Int>) {\n  m.forEach { (k, v) -> println(k + v) }\n  m.map { (_, v) -> v }\n}\nfun g() {}\n"},
	{"trailing commas", "fun f(\n  a: Int,\n  b: Int,\n) = listOf(\n  a,\n  b,\n)\nval x = f(1, 2,)\nfun g() {}\n"},
	{"open range", "fun f() {\n  for (i in 0..<10) {}\n  val r = 1..<5\n}\nfun g() {}\n"},
	{"char escapes", "val a = '\\n'\nval b = '\\u0041'\nval c = '\\''\nval d = '\\\\'\nfun g() {}\n"},
	{"raw string templates", "val a = \"\"\"\n  x ${y + 1} $z\n\"\"\"\nval b = \"a${\"b\"}c\"\nfun g() {}\n"},
	{"object by delegation", "val o = object : Iface by impl() {\n  override fun f() {}\n}\nfun g() {}\n"},
	{"object by simple", "val o = object : Iface by impl {\n  override fun f() {}\n}\nfun g() {}\n"},
	{"secondary ctor this", "class C(val a: Int) {\n  constructor() : this(1)\n  constructor(s: String) : this(s.length) { println() }\n  init { println() }\n}\nfun g() {}\n"},
	{"delegates", "class C {\n  val a by lazy { 1 }\n  var b by Delegates.observable(0) { _, _, _ -> }\n  lateinit var c: String\n  companion object { const val K = 1 }\n}\nfun g() {}\n"},
	{"file annotations", "@file:JvmName(\"X\")\n@file:Suppress(\"a\")\n\npackage p\n\nimport a.b.C as D\nimport a.b.*\n\nfun g() {}\n"},
	{"nested inner", "class A {\n  inner class B { inner class C }\n  class D\n  object E\n  interface F\n}\nfun g() {}\n"},
	{"where constraints", "fun <T> f(x: T) where T : Comparable<T>, T : Any {}\nclass K<T> where T : Any, T : Comparable<T>\nfun g() {}\n"},
	{"receiver function types", "val a: Foo.(Int) -> Unit = { }\nfun f(b: Foo.() -> Unit) {}\nval c: (Foo) -> Bar.() -> Unit = TODO()\nfun g() {}\n"},
	{"backticks unicode", "fun `a test name`() {}\nval 変数 = 1\nfun `in`() {}\nval π = 3.14\nfun g() {}\n"},
	{"annotations on lambda and expr", "fun f() {\n  val x = @Suppress(\"a\") 1\n  run @Ann { }\n}\nfun g() {}\n"},
	{"generic call and cast", "fun f(a: Any) {\n  val x = a as? List<*>\n  val y = a as List<String>\n  if (a is Map<*, *>) {}\n  val z = foo<Int, String>(1)\n}\nfun g() {}\n"},
	{"properties accessors", "class C {\n  var a: Int = 0\n    get() = field\n    private set(v) { field = v }\n  val b get() = 1\n}\nfun g() {}\n"},
	{"try expression", "fun f() = try { 1 } catch (e: Exception) { 2 } finally { }\nfun g() {}\n"},
	{"anonymous function", "val a = fun(x: Int): Int { return x }\nval b = fun Int.(): Int = this\nfun g() {}\n"},
	{"callable ref", "val a = ::foo\nval b = Foo::bar\nval c = Foo::class.java\nval d = this::x\nfun g() {}\n"},
	{"safe call elvis", "fun f(a: A?) {\n  a?.b?.c ?: return\n  a!!.d\n  a?.let { it }\n}\nfun g() {}\n"},
	{"interface default and sam", "interface I {\n  val x: Int\n  fun f() = 1\n  fun h()\n}\nfun g() {}\n"},
	{"annotation class", "annotation class A(val x: Int = 1, vararg val y: String)\n@Retention(AnnotationRetention.SOURCE)\n@Target(AnnotationTarget.CLASS)\nannotation class B\nfun g() {}\n"},
	{"unsigned and literals", "val a = 1u\nval b = 0xFFUL\nval c = 1_000L\nval d = 1.5f\nval e = 0b101\nfun g() {}\n"},
	{"class delegation", "class C(a: A) : A by a, B {\n  override fun f() {}\n}\nfun g() {}\n"},
	{"star projection vararg spread", "fun f(vararg a: Int) {}\nfun h() { f(*intArrayOf(1)) ; f(*arr, 2) }\nfun g() {}\n"},
	{"definitely non-null", "fun <T> f(x: T & Any) {}\nfun g() {}\n"},
	{"rich string templates", "val a = \"${x}\"\nval b = \"$x.y\"\nval c = \"\\$x\"\nval d = \"\\u0041\"\nfun g() {}\n"},
	{"if expression chains", "fun f(a: Int) = if (a > 1) 1 else if (a > 0) 2 else 3\nfun g() {}\n"},
	{"when subject binding", "fun f(a: Int) = when (val x = a + 1) {\n  1, 2 -> x\n  in 3..4 -> x\n  !in 5..6 -> 0\n  else -> 1\n}\nfun g() {}\n"},
	{"inline class like", "inline class Old(val v: Int)\nfun g() {}\n"},
	{"abstract and open", "abstract class A {\n  abstract fun f()\n  open val x: Int get() = 1\n  protected abstract val y: Int\n}\nfun g() {}\n"},
	{"infix and operator calls", "infix fun Int.foo(o: Int) = this\nfun f() { val x = 1 foo 2; val y = a to b; x shl 2 }\nfun g() {}\n"},
	{"suspend fun interface", "fun interface S { suspend fun run(): Int }\nfun g() {}\n"},
	{"external tailrec", "tailrec fun f(n: Int): Int = if (n == 0) 0 else f(n - 1)\nexternal fun nat()\nfun g() {}\n"},
}

// modernSyntaxKnownGaps names cases the vendored grammar cannot parse. Each
// is documented in docs/parser-limits.md.
var modernSyntaxKnownGaps = map[string]bool{}

func TestModernSyntaxParsesWithoutErrors(t *testing.T) {
	for _, tc := range modernSyntaxCases {
		t.Run(tc.name, func(t *testing.T) {
			ft, _ := parseSynthetic(t, tc.src)
			errs, missing, nonDecl := corpusErrorCounts(ft)
			bad := errs+missing+nonDecl > 0 || hasMisparse(ft)
			if modernSyntaxKnownGaps[tc.name] {
				if !bad {
					t.Errorf("known gap %q now parses; remove it from modernSyntaxKnownGaps and docs", tc.name)
				}
				return
			}
			if bad {
				t.Errorf("errors=%d missing=%d nonDecl=%d top=%s", errs, missing, nonDecl, strings.Join(topLevelTypes(ft), ","))
			}
		})
	}
}
