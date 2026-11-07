// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: infix `to`, references and calls that are not the kotlin.Pair
// constructor.
package test

class Factory {
    fun Pair(a: Int, b: Int): Int = a + b
}

fun other(f: Factory, a: Int) {
    val i = a to 1
    val m = f.Pair(1, 2)
    val r: (Int, Int) -> Pair<Int, Int> = ::Pair
    println(listOf(i, m, r))
}

// A typealias constructor is not the class constructor: detekt does not flag it either.
typealias IntPair = Pair<Int, Int>

fun viaTypealias() = IntPair(1, 2)
