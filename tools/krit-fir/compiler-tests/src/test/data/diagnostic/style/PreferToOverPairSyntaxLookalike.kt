// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Lookalikes: declarations spelled `Pair` that are not kotlin.Pair.
package test

class Factory {
    fun Pair(a: Int, b: Int): Int = a + b
}

fun localClass() {
    class Pair(val a: Int, val b: Int)
    println(Pair(1, 2).a)
}
