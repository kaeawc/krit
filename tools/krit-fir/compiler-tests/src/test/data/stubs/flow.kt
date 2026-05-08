// Minimal stubs for kotlinx.coroutines.flow — used by compiler-tests, not shipped.
package kotlinx.coroutines.flow

interface Flow<T> {
    fun collect(action: (T) -> Unit) {}
}

fun <T> Flow<T>.collect(action: (T) -> Unit) {}
