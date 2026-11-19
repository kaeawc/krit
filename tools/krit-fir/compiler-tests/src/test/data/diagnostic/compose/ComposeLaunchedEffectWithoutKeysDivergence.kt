// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 26
// Where the checker reads resolution and Go reads names.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect as Effect

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

// A constant passed by name is still a constant key. Go compares the argument text `key1 = Unit` and misses it.
@Composable
fun NamedConstantKey(userId: String) {
    androidx.compose.runtime.<!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(key1 = Unit) {
        fetch(userId)
    }
}

// An import alias still resolves to Compose's LaunchedEffect. Go matches the written name and misses it.
@Composable
fun AliasedEffect(userId: String) {
    <!ComposeLaunchedEffectWithoutKeys!>Effect<!>(Unit) {
        fetch(userId)
    }
}
