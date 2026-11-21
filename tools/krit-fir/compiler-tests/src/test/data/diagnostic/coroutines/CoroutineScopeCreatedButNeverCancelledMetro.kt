// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package scopeMetroOwnership

import dev.zacsweers.metro.*
import kotlinx.coroutines.*

@SingleIn(AppScope::class)
@Inject
class ScopedOwner {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}

@SingleIn(AppScope::class)
class ScopedTeardownOwner {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun close() {}
}

@ContributesBinding(AppScope::class)
class BindingOwner {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}
