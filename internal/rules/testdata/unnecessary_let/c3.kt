package c3

class B { fun baz(): Int = 2 }
class A {
    fun foo(): Int = 1
    val b: B = B()
    fun bar(x: Int): Int = x
    fun same(): A = this
    val it: Int = 0
    operator fun invoke(): Int = 3
}
fun take(x: Any?): Int = 0
fun takeNamed(x: Any?, y: Int = 0): Int = 0
fun sideEffect() {}
fun <T> id(t: T): T = t

fun d01(a: A) {
    val r = a
        .same()
        .let { it.foo() }
}
fun d02(a: A) {
    val r = a.same().let { it.foo() }
        .toString()
}
fun d03(n: A?, l: List<Int>) { l.forEach { n?.let { sideEffect() } } }
fun d04(n: A?, l: List<Int>) { l.map { n?.let { sideEffect() } } }
fun d05(n: A?) { val f: () -> Unit = { n?.let { sideEffect() } } }
fun d06(n: A?, l: List<Int>) { l.forEach { x -> n?.let { sideEffect() }; take(x) } }
fun d07(n: A?, c: Boolean) { if (c) { n?.let { sideEffect() } } else { n?.let { sideEffect() } } }
fun d08(n: A?, c: Boolean): Int { val x = if (c) { n?.let { sideEffect() } } else null; return 0 }
fun d09(n: A?, c: Boolean) { when (c) { true -> { n?.let { sideEffect() } } else -> { sideEffect() } } }
fun d10(n: A?, c: Boolean) { val x = when (c) { true -> { n?.let { sideEffect() } } else -> null } }
fun d11(n: A?) { try { n?.let { sideEffect() } } catch (e: Exception) { n?.let { sideEffect() } } finally { n?.let { sideEffect() } } }
fun d12(n: A?) { val x = try { n?.let { sideEffect() } } catch (e: Exception) { null } }
fun d13(n: A?, l: List<Int>) { for (i in l) { n?.let { sideEffect() } } }
fun d14(n: A?, c: Boolean) { while (c) { n?.let { sideEffect() } } }
class D15(val n: A?) {
    init { n?.let { sideEffect() } }
    val g: Int get() { n?.let { sideEffect() }; return 1 }
    constructor() : this(null) { val z = 1 }
}
class D16(val n: A?) {
    constructor(x: Int) : this(null) { n?.let { sideEffect() } }
    var s: Int = 0
        set(v) { n?.let { sideEffect() } }
}
fun d17(n: A?) { val f = fun() { n?.let { sideEffect() } } }
fun d18(n: A?, m: A?) { n?.let { m?.let { sideEffect() } } }
fun d19(a: A) { val r = a.let { it.b.let { it.baz() } } }
fun d20(a: A) { val r = a.let { run { it.foo() } } }
fun d21(a: A, l: List<Int>) { val r = a.let { l.map { it + 1 }; take(it) } }
fun d22(a: A, l: List<Int>) { a.let { l.forEach { take(it) } } }
fun d23(a: A, l: List<Int>) { val r = a.let { x -> l.map { it + x.foo() } } }
fun d24(f: () -> Int) { val r = f.let { it() } }
fun d25(f: (Int) -> Int) { val r = f.let { it(1) } }
fun d26(a: A) { val r = a.let { takeNamed(x = it) } }
fun d27(a: A) { val r = a.let { takeNamed(x = 1, y = it.foo()) } }
fun d28(a: A) { val r = a.let { a.it } }
fun d29(a: A) { val r = a.let { "$it.foo" } }
fun d30(a: A) { val r = a.let { "${it}" } }
fun d31(a: A) { val r = a.let<A, Int> { it.foo() } }
fun d32(a: A) { val r = a.let { it }!! }
fun d33(a: A) { val r = a.let { it }?.foo() }
fun d34(a: A) { val r = a.let { it.foo() }?.let { take(it) } }
fun d35(a: A) { val r = a.let lbl@{ it.foo() } }
fun d36(a: A) { val r = a.let { it.foo() /* c */ } }
fun d37(a: A) { val r = a.let { // c
    it.foo() } }
fun d38(a: A) { val r = a.let { it.foo(); } }
fun d39(n: A?) { val b = n?.let { sideEffect() } }
fun d40(n: A?): Unit? { return n?.let { sideEffect() } }
fun d41(n: A?) { take(n?.let { sideEffect() }) }
fun d42(n: A?) { n?.let { sideEffect() } ?: sideEffect() }
fun d43(n: A?, m: A?) { n?.same()?.let { sideEffect() } }
fun d44(a: A) { println("${a.let { it.foo() }}") }
fun d45(a: A) = a.let { sideEffect() }
fun d46(a: A) { val r = a.let { when (it) { else -> 1 } } }
fun d47(n: A?) { val r = n.let { if (it != null) it.foo() else 0 } }
fun d48(a: A) { val r = with(a) { let { take(this) } } }
fun d49(a: A, p: Int = a.let { it.foo() }) {}
fun d50(a: A) { val r = a.let { it.foo() }.let { it + 1 } }
fun d51(a: A) { a.same().let { take(it) }.toString() }
fun d52(a: A) { (a.same()).let { it.foo() } }
fun d53(a: A) { a!!.let { it.foo() } }
fun d54(a: A?) { a!!.let { take(it) } }
fun d55(a: A) { a.same()!!.let { take(it) } }
fun d56(a: A) { a.b.baz().let { take(it) } }
class D57 { fun m(a: A) { this.hashCode().let { take(it) } } }
fun d58(a: A) { a.let { it.foo() }.toString().let { take(it) } }
fun d59(a: A) { val r = a.let { x -> val y = x; y.foo() } }
fun d60(a: A) { val r = a.let { x -> take(x); sideEffect() } }
fun d61(a: A) { val r = a.let { x -> { x.foo() } } }
fun d62(a: A) { val r = a.let { x -> object { val v = x.foo() } } }
fun d63(a: A) { val r = a.let { it.foo() as Number } }
fun d64(a: A) { val r = a.let { it.b.baz()!! } }
fun d65(a: A) { val r = a.let { it.foo().toString().length } }
fun d66(a: A) { val r = a.let { !it.foo().equals(1) } }
fun d67(a: A) { val r = a.let { -it.foo() } }
fun d69(l: List<A>) { val r = l.let { it[0] } }
fun d70(l: List<A>) { val r = l.let { it.first().b.baz() } }
fun d71(l: Map<String, A>) { val r = l.let { it["k"] } }
fun d72(a: A) { val r = a.let { it to 1 } }
fun d73(a: A) { val r = a.let { it ?: 1 } }
fun d74(a: A?) { val r = a.let { it ?: 1 } }
fun d75(a: A?) { val r = a?.let { it ?: 1 } }
fun d76(a: A) { val r = a.let { it is A } }
fun d77(a: A) { val r = a.let { (it) } }
fun d78(a: A) { val r = a.let { (it).foo() } }
fun d79(a: A) { val r = a.let { it.foo().let { x -> x } } }
fun d80(a: A) { a.let { x -> x.also { sideEffect() } } }
