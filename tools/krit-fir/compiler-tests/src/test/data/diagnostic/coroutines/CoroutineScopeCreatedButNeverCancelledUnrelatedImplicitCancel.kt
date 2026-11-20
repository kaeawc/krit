// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package scopeUnrelatedImplicitCancellation

import kotlinx.coroutines.*

class LeakingOwner {
    // FIR treats an unresolved cancel anywhere in the file as possible management and stays silent (conservative); Go reports this scope.
    val scope = CoroutineScope(Dispatchers.IO)
}

class UnrelatedDelegatedOwner : CoroutineScope by MainScope() {
    fun close() { cancel() }
}
