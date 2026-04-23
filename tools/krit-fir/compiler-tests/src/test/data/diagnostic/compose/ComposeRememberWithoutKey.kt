// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: remember { } with no key argument — should trigger COMPOSE_REMEMBER_WITHOUT_KEY
package test

import androidx.compose.runtime.remember

fun MyComposable() {
    val value = <!COMPOSE_REMEMBER_WITHOUT_KEY!>remember<!> { "hello" }
    println(value)
}
