// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 51, 53
// Where FIR follows the declaration the call resolves to and Go follows the call name.
package test

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.launch

// A project function that only shares its name with a coroutines function.
fun withContext(block: () -> Unit) = block()

// The lambda of launch is its own coroutine; the outer function never suspends.
<!RedundantSuspendModifier!>suspend<!> fun startsCoroutine(scope: CoroutineScope) {
    scope.launch {
        delay(1)
    }
}

// The same for the lambda of flow: it runs when the flow is collected.
<!RedundantSuspendModifier!>suspend<!> fun buildsFlow(): Flow<Int> = flow {
    emit(1)
}

// Neither engine reports: the lambda of a project suspend function runs in the caller.
suspend fun runSuspending(block: suspend () -> Unit) {
    block()
}

suspend fun callsRunSuspending() {
    runSuspending { delay(1) }
}

// The project withContext is not suspend, so there is nothing to wait for.
<!RedundantSuspendModifier!>suspend<!> fun callsLookalike() {
    withContext { println("no suspension") }
}

inline fun <T> inPlace(block: () -> T): T = block()

fun notInPlace(block: () -> Unit) = block()

// The lambda of a project inline function runs in the suspend caller.
suspend fun inlineLambda() {
    inPlace { delay(1) }
}

// A stub awaiting its implementation keeps the modifier. Go reports the TODO() bodies.
suspend fun notImplemented(): Int = TODO()

suspend fun notImplementedBlock() {
    TODO("later")
}

suspend fun alwaysThrows() {
    throw IllegalStateException("unsupported")
}
