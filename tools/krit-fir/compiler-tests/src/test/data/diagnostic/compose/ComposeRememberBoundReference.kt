// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: keyless remember whose calculation is a bound callable reference
// capturing the enclosing `model` parameter (`model::toString`). The memo never
// updates when model changes — should trigger COMPOSE_REMEMBER_WITHOUT_KEY.
package test

import androidx.compose.runtime.remember

fun MyComposable(model: Any) {
    val value = <!COMPOSE_REMEMBER_WITHOUT_KEY!>remember<!>(model::toString)
    println(value)
}
