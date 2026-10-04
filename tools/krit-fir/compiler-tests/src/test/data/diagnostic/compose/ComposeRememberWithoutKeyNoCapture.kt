// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: keyless remember whose lambda captures nothing external (a constant)
// is the canonical, correct idiom — must NOT trigger ComposeRememberWithoutKey.
package test

import androidx.compose.runtime.remember

fun MyComposable() {
    val value = remember { "constant" }
    <!PrintlnInProduction!>println<!>(value)
}
