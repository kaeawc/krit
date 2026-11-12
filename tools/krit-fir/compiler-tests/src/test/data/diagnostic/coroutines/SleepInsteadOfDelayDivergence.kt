// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Where FIR follows the compiler and Go follows the spelling of the call.
package test

import kotlin.coroutines.suspendCoroutine
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.suspendCancellableCoroutine

inline fun <T> inPlace(block: () -> T): T = block()

fun notInPlace(block: () -> Unit) {}

// Go sees an unknown call name and stops; the project function is inline,
// so the lambda runs in the suspend caller.
suspend fun projectInline() {
    inPlace { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
}

// Neither engine reports: the lambda of a non-inline function is not a coroutine body.
suspend fun nonInlineLambda() {
    notInPlace { Thread.sleep(10) }
}

// The lambda of a suspend-typed parameter is a coroutine body even when the
// callee is a project function Go has never heard of.
suspend fun runSuspending(block: suspend () -> Unit) {}

fun projectSuspendBuilder(scope: CoroutineScope) {
    scope.launch {
        runSuspending { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
    }
}

// Go stops at the unknown call names; these crossinline blocks run before the
// function suspends, so the sleep blocks the coroutine's thread.
suspend fun suspendingBlocks() {
    suspendCoroutine<Int> { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
    suspendCancellableCoroutine<Int> { <!SleepInsteadOfDelay!>Thread.sleep(10)<!> }
}
