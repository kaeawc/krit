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

// launchWhenStarted only suspends the collector; it is NOT a safe wrapper, so a
// collect inside it in a lifecycle callback must still be flagged.
fun launchWhenStarted(block: () -> Unit) { block() }
