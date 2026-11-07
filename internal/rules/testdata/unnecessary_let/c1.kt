package c1

class B { fun baz(): Int = 2 }
class A {
    fun foo(): Int = 1
    val b: B = B()
    fun bar(x: Int): Int = x
    fun same(): A = this
    fun selfLet(): Int = let { it.foo() }
    fun implicitNone(): Int = let { hashCode() }
}
fun take(x: Any?): Int = 0
fun takeA(a: A): Int = 0
fun sideEffect() {}
fun <T> id(t: T): T = t
class H(val a: A, val n: A?)
fun A.ext(): Int = let { it.foo() }
fun A.ext2(): Int = let { take(it) }

fun c01(a: A) { val r = a.let { it.foo() } }
fun c02(a: A) { val r = a.let { it.b.baz() } }
fun c03(a: A) { val r = a.let { take(it) } }
fun c04(a: A) { val r = a.let { it } }
fun c05(a: A) { val r = a.let { 1 } }
fun c06(a: A) { a.let { sideEffect() } }
fun c07(n: A?) { val r = n?.let { it.foo() } }
fun c08(n: A?) { n?.let { it.foo() } }
fun c09(n: A?) { n?.let { sideEffect() } }
fun c10(n: A?) { val r = n?.let { sideEffect(); 1 } }
fun c11(n: A?) { n?.let { take(it) } }
fun c12(n: A?) { n?.let { it } }
fun c13(n: A?) { n?.let { it?.foo() } }
fun c14(a: A) { val r = a.let { x -> x.foo() } }
fun c15(a: A) { val r = a.let { x -> take(x) } }
fun c16(a: A) { val r = a.let { x -> 1 } }
fun c17(a: A) { val r = a.let { it.foo() + 1 } }
fun c18(a: A) { val r = a.let { it.foo() + it.bar(1) } }
fun c19(a: A) { val r = a.let { take(it); it.foo() } }
fun c20(a: A) { val r = a.let { sideEffect(); it.foo() } }
fun c21(h: H) { val r = h.a.let { take(it) } }
fun c22(h: H) { val r = h.a.let { it.foo() } }
fun c23(a: A) { val r = a.let { take(it) }.toString() }
fun c24(a: A) { val r = a.let { it.foo() }.toString() }
fun c25(a: A) { val r = a.let { x -> x.b.let { it.baz() } } }
fun c26(a: A) { val r = a.let { it.b.let { take(it) } } }
fun c27(a: A) { val r = a.let(::takeA) }
fun c28(n: A?) { val r = n?.let(::takeA) }
class C29 { fun m(a: A) { val r = a.let { this.hashCode() } } }
fun c32(a: A) { val r = a.let { it == null } }
fun c33(n: A?) { val r = n.let { it == null } }
fun c34(i: Int) { val r = i.let { it + 1 } }
fun c35(a: A) { val r = a.let { "$it" } }
fun c35b(a: A) { val r = a.let { "${it.foo()}" } }
fun c36(p: Pair<A, A>) { val r = p.let { (x, y) -> x.foo() } }
fun c37(p: Pair<A, A>) { val r = p.let { (x, y) -> x.foo() + y.foo() } }
fun c38(p: Pair<A, A>) { val r = p.let { (x, y) -> 1 } }
fun c39(p: Pair<A, A>) { val r = p.let { (x, _) -> x } }
fun c40(a: A) { val r = a.let { return@let 1 } }
fun c41(a: A) { val r = a.let { if (it.foo() > 0) 1 else 2 } }
fun c42(a: A) { val r = a.let { when { it.foo() > 0 -> 1 else -> 2 } } }
fun c43(a: A) { val f = { a.let { it.foo() } } }
fun c44(a: A): Int { return a.let { it.foo() } }
fun c45(): Int { val s = System.getProperty("x"); return s.let { it.length } }
fun c45b(): Int? { val s = System.getProperty("x"); return s?.let { it.length } }
fun c46(n: A?) { val f = { n?.let { it.foo() } } }
fun c47(n: A?) = n?.let { 1 }
fun c48(n: A?, c: Boolean) { if (c) n?.let { sideEffect() } }
fun c49(n: A?) { run { n?.let { sideEffect() } } }
fun c49b(n: A?) { n?.let { sideEffect() }; sideEffect() }
fun c49c(n: A?): Unit { n?.let { sideEffect() } }
fun c49d(n: A?) = n?.let { sideEffect() }
fun c50(n: A?) { val r = n?.let { it.foo() } ?: 0 }
fun c52(a: A) { val r = a.myLet { it.foo() } }
fun <T, R> T.myLet(block: (T) -> R): R = block(this)
fun c53(a: A) { val r = with(a) { let { it.foo() } } }
fun c54(a: A) { val r = a.let { _ -> 1 } }
fun c55(a: A) { val r = a.let({ it.foo() }) }
fun c55b(n: A?) { val r = n?.let({ it.foo() }) }
fun c56(a: A, l: List<Int>) { val r = a.let { l.map { x -> x + it.foo() } } }
fun c57(a: A) { val r = a.let { it }.foo() }
fun c58(a: A) { val r = take(a.let { it.foo() }) }
fun c59(a: A) { val r = a?.let { it.foo() } }
fun c60(a: A) { val r = a.let { it::foo } }
fun <T> c61(t: T) { val r = t.let { take(it) } }
fun c62(a: A) { a?.let { sideEffect() } }
fun c63(h: H) { h.n?.let { take(it) }; h.n?.let { it.foo() }; h.n?.b?.let { take(it) }; h.n?.b?.let { it.baz() } }
fun c65(a: A) { val r = a.let { x: A -> x.foo() } }
fun c66(a: A) { val r = a.same().let { it.foo() } }
val c67 = A().let { it.foo() }
fun c69(a: A) { val r = a.let { it.same().apply { foo() } } }
fun c71(h: H) { val r = h.a.let { it!!.foo() } }
fun c71b(a: A) { val r = a.let { it!!.foo() } }
fun c73(a: A) { a.let { it.foo() } }
fun c74(a: A) { a.let { take(it) } }
fun c75(a: A) { a.let { } }
fun c76(n: A?) { n?.let { } }
fun c77(a: A) { val r = a.let { it.foo(); sideEffect() } }
fun c78(a: A) { val r = a.let { x -> x.foo(); x.bar(1) } }
fun c79(a: A) { val r = a.let { id(it).foo() } }
fun c80(n: A?) { n?.let { id(it).foo() } }
fun c81(n: A?) { n?.let { it.b.baz() } }
fun c82(n: A?) { n?.let { x -> x.foo() } }
fun c83(n: A?) { val r = n?.let { x -> sideEffect() } }
fun c84(n: A?) { n?.let { x -> sideEffect() } }
fun c85(a: A) { a.let { it.foo() }.let { take(it) } }
