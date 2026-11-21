// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package scopeDelegatedCancellation

import kotlinx.coroutines.*

class DelegatedOwner : CoroutineScope by MainScope() {
    // Neither FIR nor Go reports on this delegated MainScope owner.
    fun close() { cancel() }
}
