// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: safe `as?` cast — should NOT trigger UNSAFE_CAST_WHEN_NULLABLE
package test

fun example(x: Any) {
    val y = x as? String
    println(y)
}
