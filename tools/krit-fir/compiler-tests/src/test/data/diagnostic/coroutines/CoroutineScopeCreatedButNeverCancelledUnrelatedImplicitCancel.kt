// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package scopeUnrelatedImplicitCancellation

import kotlinx.coroutines.*

class LeakingOwner {
    // FIR treats a cancel() anywhere in the file whose receiver it cannot relate to this property (here UnrelatedDelegatedOwner's implicit this) as possible management and stays silent; Go reports this scope.
    val scope = CoroutineScope(Dispatchers.IO)
}

class UnrelatedDelegatedOwner : CoroutineScope by MainScope() {
    fun close() { cancel() }
}
