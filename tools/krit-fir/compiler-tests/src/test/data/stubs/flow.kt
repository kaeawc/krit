// Minimal stubs for kotlinx.coroutines.flow — used by compiler-tests, not shipped.
package kotlinx.coroutines.flow

interface Flow<out T>

fun <T> Flow<T>.collect(action: (T) -> Unit) {}
