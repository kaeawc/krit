// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 17, 18, 19, 20, 21, 22, 23, 24, 26, 29, 30, 31, 32, 33, 34, 35, 36
// Lambda parameters of nested calls and lambdas that return Unit, checked against detekt 1.23.8.
package test

class A { fun foo(): Int = 1 }
fun take(x: Any?): Int = 0
fun sideEffect() {}
fun <R> block0(f: () -> R): R = f()
fun <T> block1(t: T, f: (T) -> Unit) { f(t) }
fun <T, R> T.apply2(f: T.() -> R): R = f()
fun cb(f: (Int) -> Unit) { f(1) }
fun cb0(f: () -> Unit) { f() }
fun cbr(f: () -> Int): Int = f()

fun f01(a: A) { val r = a.<!UnnecessaryLet!>let<!> { block0 { it.foo() } } }
fun f02(a: A) { val r = a.<!UnnecessaryLet!>let<!> { block1(1) { take(it) } } }
fun f03(a: A) { val r = a.<!UnnecessaryLet!>let<!> { block1(1) { take(it) }; it.foo() } }
fun f04(n: A?) { Thread { n?.<!UnnecessaryLet!>let<!> { sideEffect() } } }
fun f05(n: A?) { val r = Runnable { n?.<!UnnecessaryLet!>let<!> { sideEffect() } } }
fun f06(n: A?) { cb { n?.<!UnnecessaryLet!>let<!> { sideEffect() } } }
fun f07(n: A?) { cb0 { n?.<!UnnecessaryLet!>let<!> { sideEffect() } } }
fun f08(a: A) { val r = a.<!UnnecessaryLet!>let<!> { x -> val it = 1; take(it) + x.foo() } }
fun f09(a: A) { val r = a.<!UnnecessaryLet!>let<!> { it.apply2 { foo() } } }
fun f10(n: A?) { run { n?.let { sideEffect() } } }
fun f11(n: A?) = run { n?.<!UnnecessaryLet!>let<!> { sideEffect() }; 1 }
fun f13(n: A?) { listOf(1).map { n?.let { sideEffect() } } }
fun f14(n: A?) { val l = lazy { n?.let { sideEffect() } } }
fun f15(n: A?) { cb { x -> n?.<!UnnecessaryLet!>let<!> { sideEffect() } } }
fun f16(n: A?) { cbr { n?.<!UnnecessaryLet!>let<!> { sideEffect() }; 1 } }
fun f17(a: A) { val r = a.<!UnnecessaryLet!>let<!> { cb { take(it) }; it.foo() } }
fun f18(a: A) { val r = a.<!UnnecessaryLet!>let<!> { block1(it) { take(it) } } }
fun f19(a: A) { val r = a.<!UnnecessaryLet!>let<!> { cb0 { take(it) } } }
fun f20(a: A, l: List<Int>) { val r = a.<!UnnecessaryLet!>let<!> { l.map { take(it) } } }
fun f21(a: A, l: List<Int>) { val r = a.<!UnnecessaryLet!>let<!> { l.also { take(it) } } }
fun f22(a: A, l: List<Int>) { val r = a.<!UnnecessaryLet!>let<!> { l.apply { take(it) } } }
