// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: injected dispatcher parameter should NOT trigger
package test

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.withContext

suspend fun loadData(dispatcher: CoroutineDispatcher) {
    val data = withContext(dispatcher) { "data" }
    <!PrintlnInProduction!>println<!>(data)
}
