// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: remember { } with an explicit key — should NOT trigger ComposeRememberWithoutKey
package test

import androidx.compose.runtime.remember

fun MyComposable(input: String) {
    val value = remember(input) { input.length }
    println(value)
}
