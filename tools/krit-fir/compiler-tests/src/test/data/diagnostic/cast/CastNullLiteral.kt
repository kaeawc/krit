// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: `null as String?` always succeeds (Nothing? is a subtype of the
// nullable target) — must NOT trigger UnsafeCastWhenNullable.
package test

fun example() {
    val y = null as String?
    <!PrintlnInProduction!>println<!>(y)
}
