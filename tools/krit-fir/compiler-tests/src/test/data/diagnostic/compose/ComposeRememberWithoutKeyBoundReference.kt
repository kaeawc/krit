// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positive: keyless remember whose calculation is a bound callable reference
// capturing the enclosing `model` parameter (`model::toString`). The memo never
// updates when model changes — should trigger ComposeRememberWithoutKey.
// Go misses this because it needs a trailing lambda and reads `model::toString`
// as a key argument; the reference is the calculation, and there is no key.
package test

import androidx.compose.runtime.remember

fun MyComposable(model: Any) {
    val value = <!ComposeRememberWithoutKey!>remember<!>(model::toString)
    <!PrintlnInProduction!>println<!>(value)
}
