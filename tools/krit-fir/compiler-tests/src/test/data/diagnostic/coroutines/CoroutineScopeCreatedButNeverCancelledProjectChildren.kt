// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package scopeProjectChildren

import kotlinx.coroutines.*

class Owner {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun stop() { scope.cancelChildren() }
}

fun CoroutineScope.cancelChildren() = coroutineContext.cancelChildren()
