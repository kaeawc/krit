// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package scopeAppScopeAlias

import kotlinx.coroutines.*
import software.amazon.lastmile.kotlin.inject.anvil.SingleIn

class SingletonScope

typealias AppScope = SingletonScope

@SingleIn(AppScope::class)
class ScopedOwner {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun close() {}
}
