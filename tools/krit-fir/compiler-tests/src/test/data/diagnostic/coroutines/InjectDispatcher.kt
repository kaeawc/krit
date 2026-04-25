// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: hardcoded Dispatchers.IO passed to withContext() should trigger INJECT_DISPATCHER
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

suspend fun loadData() {
    val data = withContext(<!INJECT_DISPATCHER!>Dispatchers.IO<!>) { "data" }
    println(data)
}
