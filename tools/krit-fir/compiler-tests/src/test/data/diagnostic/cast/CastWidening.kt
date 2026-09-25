// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: `x as String?` where x is already String always succeeds (redundant,
// USELESS_CAST — not unsafe) — must NOT trigger UnsafeCastWhenNullable.
package test

fun example(x: String) {
    val y = x as String?
    <!PrintlnInProduction!>println<!>(y)
}
