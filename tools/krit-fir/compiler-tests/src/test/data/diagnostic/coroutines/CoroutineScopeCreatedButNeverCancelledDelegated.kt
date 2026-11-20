// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package scopeDelegatedCancellation

import kotlinx.coroutines.*

class LeakingOwner {
    // Go reports; FIR pins its conservative unknown relation to the delegated receiver below.
    val scope = CoroutineScope(Dispatchers.IO)
}

class DelegatedOwner : CoroutineScope by MainScope() {
    fun close() { cancel() }
}
