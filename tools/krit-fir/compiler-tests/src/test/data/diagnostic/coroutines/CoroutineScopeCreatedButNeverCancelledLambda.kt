// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 10, 18
package scopeLambdaCancellation

import kotlinx.coroutines.*

class ManagedOwner {
    // Go misses lambda aliases; FIR proves these parameters are bound to their respective scopes.
    val letScope = CoroutineScope(Dispatchers.IO)
    val alsoScope = CoroutineScope(Dispatchers.IO)
    fun stop() {
        letScope.let { it.cancel() }
        alsoScope.also { bound -> bound.cancel() }
    }
}

class UnmanagedOwner {
    val <!CoroutineScopeCreatedButNeverCancelled!>unmanaged<!> = CoroutineScope(Dispatchers.IO)
}
