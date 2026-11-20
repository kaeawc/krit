// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package scopeKoinOwnership

import org.koin.core.annotation.Singleton
import kotlinx.coroutines.*

@Singleton
class ScopedOwner {
    val scope = CoroutineScope(Dispatchers.IO)
    fun close() {}
}
