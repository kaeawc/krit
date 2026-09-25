// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: keyless remember whose calculation is a bound callable reference
// capturing the enclosing `model` parameter (`model::toString`). The memo never
// updates when model changes — should trigger ComposeRememberWithoutKey.
package test

import androidx.compose.runtime.remember

fun MyComposable(model: Any) {
    val value = <!ComposeRememberWithoutKey!>remember<!>(model::toString)
    <!PrintlnInProduction!>println<!>(value)
}
