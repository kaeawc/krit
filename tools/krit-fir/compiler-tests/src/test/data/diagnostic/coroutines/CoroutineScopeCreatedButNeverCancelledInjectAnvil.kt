// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package scopeInjectAnvilOwnership

import software.amazon.lastmile.kotlin.inject.anvil.*
import kotlinx.coroutines.*

class AppScope

@SingleIn(AppScope::class)
class ScopedTeardownOwner {
    val scope = CoroutineScope(Dispatchers.IO)
    fun close() {}
}

@ContributesBinding(AppScope::class)
class BindingOwner {
    val scope = CoroutineScope(Dispatchers.IO)
}
