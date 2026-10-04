// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14
// Lookalike: a same-package top-level `require` shadows the default-imported
// kotlin.require, so the call below is not kotlin.require.
package test

fun require(condition: Boolean, message: String = "") {
    if (!condition) throw IllegalArgumentException(message)
}

// Divergence: Go reports any call named `require`; this one is not
// kotlin.require, so kotlin.requireNotNull is not its replacement.
fun samePackage(x: Any?) {
    require(x != null)
}
