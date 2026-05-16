package com.example

import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Greeting(name: String) {
    // Resource lookup — must not fire.
    Text(text = stringResource(R.string.hello))
    // Concatenation that still includes a resource lookup — also OK.
    Text(text = "Hello " + getString(R.string.world))
    // Pure interpolation — no literal text to localize.
    Text(text = "$name")
    Text(text = "${name.uppercase()}")
    Text(text = "$name$name")
}

// Non-Compose, non-Android data class. Even with a `text =` named arg,
// this must not fire — the file has no Android/Compose-relevant import
// and the callee is not in the known Compose set.
data class LogEntry(val text: String, val description: String)

fun build(): LogEntry = LogEntry(text = "raw log line", description = "raw description")

// Lower-case callee — not a composable. Must not fire even though the
// file imports Compose above.
fun emit(message: String) {
    error(description = "boom: $message")
}
