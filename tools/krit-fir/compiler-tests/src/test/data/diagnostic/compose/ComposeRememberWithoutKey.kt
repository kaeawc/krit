// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14
// Positive: keyless remember whose calculation lambda captures the enclosing
// `input` parameter. The memo has no key, so the cached value never updates
// when input changes across recomposition — should trigger
// ComposeRememberWithoutKey.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun MyComposable(input: String) {
    val value = <!ComposeRememberWithoutKey!>remember<!> { input.length }
    <!PrintlnInProduction!>println<!>(value)
}
