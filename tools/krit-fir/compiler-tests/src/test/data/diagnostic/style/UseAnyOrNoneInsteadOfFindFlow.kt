// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 19, 22
// kotlinx.coroutines.flow's firstOrNull(predicate), with the Flow.any / none
// operators coroutines ships from 1.10 (declared here because the flow stub
// has only the predicate-less firstOrNull()). Go reports both comparisons and
// the replacement exists for a Flow, so the checker keeps them. Without
// Flow.any / none (older coroutines) the checker stays silent: see
// UseAnyOrNoneInsteadOfFindCounterpartTest.
package kotlinx.coroutines.flow

suspend fun <T> Flow<T>.firstOrNull(predicate: suspend (T) -> Boolean): T? = TODO()

suspend fun <T> Flow<T>.any(predicate: suspend (T) -> Boolean): Boolean = TODO()

suspend fun <T> Flow<T>.none(predicate: suspend (T) -> Boolean): Boolean = TODO()

suspend fun flowNotNull(flow: Flow<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>flow.firstOrNull { it > 0 } != null<!>

suspend fun flowIsNull(flow: Flow<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>flow.firstOrNull { it > 0 } == null<!>

// A Flow subtype.
suspend fun stateFlow(flow: StateFlow<String>): Boolean = <!UseAnyOrNoneInsteadOfFind!>flow.firstOrNull { it.isEmpty() } != null<!>

// No predicate: Go and the checker both leave it alone.
suspend fun noPredicate(flow: Flow<Int>): Boolean = flow.firstOrNull() != null
