// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 19, 21, 24, 25
// Calls with arguments and a trailing lambda, and calls on the result of a call, in receiver chains.
package test

class A {
    fun foo(): Int = 1
    fun same(): A = this
    fun blockCall(n: Int, f: () -> Int): Int = f()
    fun lambdaOnly(f: () -> Int): A = this
}
fun take(x: Any?): Int = 0
fun sideEffect() {}
fun inv(a: A): (Int) -> A = { a }
fun item(f: () -> Unit) {}

fun g1(a: A) { val r = inv(a)(1).<!UnnecessaryLet!>let<!> { x -> take(x) } }
fun g2(a: A) { val r = a.same().blockCall(1) { 2 }.let { x -> take(x) } }
fun g3(a: A) { val r = a.<!UnnecessaryLet!>let<!> { x -> a.blockCall(1) { take(x) }; sideEffect() } }
fun g4(a: A?) { a?.let { item { take(it) } } }
fun g5(a: A) { val r = a.<!UnnecessaryLet!>let<!> { x -> x.blockCall(1) { 2 } } }
fun g6(a: A) { val r = a.same().lambdaOnly { 2 }.let { x -> take(x) } }
fun g7(a: A) { val r = a.lambdaOnly { 2 }.let { x -> take(x) } }
fun g8(a: A) { val r = a.<!UnnecessaryLet!>let<!> { it.same().blockCall(1) { 2 } } }
fun g9(a: A) { val r = (a).<!UnnecessaryLet!>let<!> { x -> take(x) } }
fun g10(a: A) { val r = a.same().let { x -> take(x) }.toString() }
fun g11(a: A) { val r = inv(a)(1).same().let { x -> take(x) } }
fun g12(a: A) { val r = a.blockCall(1) { 2 }.let { x -> take(x) } }
