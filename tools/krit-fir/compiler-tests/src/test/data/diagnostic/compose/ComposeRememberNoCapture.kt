// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: keyless remember whose lambda captures nothing external (a constant)
// is the canonical, correct idiom — must NOT trigger COMPOSE_REMEMBER_WITHOUT_KEY.
package test

import androidx.compose.runtime.remember

fun MyComposable() {
    val value = remember { "constant" }
    println(value)
}
