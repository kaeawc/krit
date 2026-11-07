package c4

class A { fun foo(): Int = 1 }
fun take(x: Any?): Int = 0
fun sideEffect() {}
fun <R> block0(f: () -> R): R = f()
fun <T> block1(t: T, f: (T) -> Unit) { f(t) }
fun <T, R> T.apply2(f: T.() -> R): R = f()
fun cb(f: (Int) -> Unit) { f(1) }
fun cb0(f: () -> Unit) { f() }
fun cbr(f: () -> Int): Int = f()

fun f01(a: A) { val r = a.let { block0 { it.foo() } } }
fun f02(a: A) { val r = a.let { block1(1) { take(it) } } }
fun f03(a: A) { val r = a.let { block1(1) { take(it) }; it.foo() } }
fun f04(n: A?) { Thread { n?.let { sideEffect() } } }
fun f05(n: A?) { val r = Runnable { n?.let { sideEffect() } } }
fun f06(n: A?) { cb { n?.let { sideEffect() } } }
fun f07(n: A?) { cb0 { n?.let { sideEffect() } } }
fun f08(a: A) { val r = a.let { x -> val it = 1; take(it) + x.foo() } }
fun f09(a: A) { val r = a.let { it.apply2 { foo() } } }
fun f10(n: A?) { run { n?.let { sideEffect() } } }
fun f11(n: A?) = run { n?.let { sideEffect() }; 1 }
fun f13(n: A?) { listOf(1).map { n?.let { sideEffect() } } }
fun f14(n: A?) { val l = lazy { n?.let { sideEffect() } } }
fun f15(n: A?) { cb { x -> n?.let { sideEffect() } } }
fun f16(n: A?) { cbr { n?.let { sideEffect() }; 1 } }
fun f17(a: A) { val r = a.let { cb { take(it) }; it.foo() } }
fun f18(a: A) { val r = a.let { block1(it) { take(it) } } }
fun f19(a: A) { val r = a.let { cb0 { take(it) } } }
fun f20(a: A, l: List<Int>) { val r = a.let { l.map { take(it) } } }
fun f21(a: A, l: List<Int>) { val r = a.let { l.also { take(it) } } }
fun f22(a: A, l: List<Int>) { val r = a.let { l.apply { take(it) } } }
