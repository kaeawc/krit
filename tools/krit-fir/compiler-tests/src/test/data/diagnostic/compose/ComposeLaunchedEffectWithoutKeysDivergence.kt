// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 25
// Where the checker reads resolution and Go reads names.
package test

import androidx.compose.runtime.Composable

fun fetch(value: Any?) {}

typealias Refresh = () -> Unit

// A project function named like the Compose effect is not the Compose effect.
fun LaunchedEffect(key: Any?, block: () -> Unit) {}

@Composable
fun LocalLookalike(userId: String) {
    LaunchedEffect(Unit) {
        fetch(userId)
    }
}

// A function type hidden behind a typealias is still a callback.
@Composable
fun AliasedCallback(refresh: Refresh) {
    androidx.compose.runtime.LaunchedEffect(Unit) {
        refresh()
    }
}
