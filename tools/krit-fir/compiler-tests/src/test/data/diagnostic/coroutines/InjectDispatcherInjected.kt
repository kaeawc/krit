// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: injected dispatcher parameter should NOT trigger
package test

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.withContext

suspend fun loadData(dispatcher: CoroutineDispatcher) {
    val data = withContext(dispatcher) { "data" }
    println(data)
}
