// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: hardcoded dispatchers inside a top-level function and an extension
// function have no class/constructor to inject into, so they must NOT trigger
// INJECT_DISPATCHER.
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

// Top-level function: nothing to inject into.
suspend fun loadDataTopLevel(): String {
    return withContext(Dispatchers.IO) { "data" }
}

// Top-level extension function: no class owner, nothing to inject into.
suspend fun String.loadDataExtension(): String {
    return withContext(Dispatchers.Default) { this }
}
