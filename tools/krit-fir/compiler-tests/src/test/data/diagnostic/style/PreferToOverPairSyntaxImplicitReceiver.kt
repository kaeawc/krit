// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12
// Divergence: Go reports the implicit-receiver call spelled `Pair`, because
// it cannot tell that the receiver's member wins over kotlin.Pair. FIR
// resolves the member and reports nothing.
package test

class Factory {
    fun Pair(a: Int, b: Int): Int = a + b
}

fun implicitReceiver(f: Factory) = with(f) { Pair(1, 2) }
