// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: keyless remember whose calculation lambda captures the enclosing
// `input` parameter. The memo has no key, so the cached value never updates
// when input changes across recomposition — should trigger
// COMPOSE_REMEMBER_WITHOUT_KEY.
package test

import androidx.compose.runtime.remember

fun MyComposable(input: String) {
    val value = <!COMPOSE_REMEMBER_WITHOUT_KEY!>remember<!> { input.length }
    println(value)
}
