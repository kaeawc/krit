// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Lookalike: a same-package top-level `require` shadows the default-imported
// kotlin.require, so the call below is not kotlin.require.
package test

fun require(condition: Boolean, message: String = "") {
    if (!condition) throw IllegalArgumentException(message)
}

// Not kotlin.require, so kotlin.requireNotNull is not its replacement. Go
// skips it too: the file declares a top-level `require`.
fun samePackage(x: Any?) {
    require(x != null)
}
