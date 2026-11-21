// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package scopeGuiceOwnership

import kotlinx.coroutines.*

@Target(AnnotationTarget.CLASS, AnnotationTarget.FUNCTION)
@Retention(AnnotationRetention.RUNTIME)
annotation class Singleton

@Singleton
class ScopedOwner {
    val scope = CoroutineScope(Dispatchers.IO)
    fun close() {}
}
