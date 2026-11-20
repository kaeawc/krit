// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package scopeGuiceOwnership

import com.google.inject.Singleton
import kotlinx.coroutines.*

@Singleton
class ScopedOwner {
    val scope = CoroutineScope(Dispatchers.IO)
    fun close() {}
}
