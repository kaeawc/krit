// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: safe `as?` cast — should NOT trigger UnsafeCastWhenNullable
package test

fun example(x: Any) {
    val y = x as? String
    <!PrintlnInProduction!>println<!>(y)
}
