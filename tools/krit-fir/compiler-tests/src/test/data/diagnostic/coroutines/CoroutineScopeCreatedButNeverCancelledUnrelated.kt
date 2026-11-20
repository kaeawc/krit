// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package scopeUnrelatedCancellation

import kotlinx.coroutines.*

class Poller {
    // Go reports; FIR stays conservative when the mutable Job receiver's ownership is unknown.
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var pollJob: Job? = null
    fun start() { pollJob = scope.launch {} }
    fun stop() { pollJob?.cancel() }
}
