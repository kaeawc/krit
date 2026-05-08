// Minimal stubs for kotlinx.coroutines — used by compiler-tests, not shipped.
package kotlinx.coroutines

open class CoroutineDispatcher

object Dispatchers {
    val IO: CoroutineDispatcher = CoroutineDispatcher()
    val Default: CoroutineDispatcher = CoroutineDispatcher()
    val Unconfined: CoroutineDispatcher = CoroutineDispatcher()
    val Main: CoroutineDispatcher = CoroutineDispatcher()
}

suspend fun <T> withContext(context: CoroutineDispatcher, block: () -> T): T = block()
