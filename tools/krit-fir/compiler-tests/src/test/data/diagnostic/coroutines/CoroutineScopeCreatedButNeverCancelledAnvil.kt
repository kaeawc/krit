// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package scopeAnvilOwnership

import com.squareup.anvil.annotations.optional.SingleIn
import kotlinx.coroutines.*

class AppScope

@SingleIn(AppScope::class)
class ScopedTeardownOwner {
    val scope = CoroutineScope(Dispatchers.IO)
    fun close() {}
}
