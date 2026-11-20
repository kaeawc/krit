// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package scopeReset

import kotlinx.coroutines.*

// Isolated so the extension's implicit receiver cannot conservatively exempt unrelated golden cases.
class ResetOwner {
    val scope = CoroutineScope(Dispatchers.IO)
    fun detach() { scope.reset() }
}
fun CoroutineScope.reset() { this.cancel() }
