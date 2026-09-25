// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: unsafe `as` cast to a nullable type — should trigger UnsafeCastWhenNullable
package test

fun example(x: Any) {
    val y = x <!UnsafeCastWhenNullable!>as<!> String?
    println(y)
}
