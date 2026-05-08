// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: unsafe `as` cast to a nullable type — should trigger UNSAFE_CAST_WHEN_NULLABLE
package test

fun example(x: Any) {
    val y = x <!UNSAFE_CAST_WHEN_NULLABLE!>as<!> String?
    println(y)
}
