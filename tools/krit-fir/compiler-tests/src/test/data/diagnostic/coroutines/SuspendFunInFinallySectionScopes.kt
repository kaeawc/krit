// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 42, 46, 61, 74, 86, 94, 109, 113, 118, 121, 140, 143, 146
// Which code written inside a finally block runs there. Go walks every call in
// the finally subtree, whatever scope it sits in. FIR walks only the code the
// finally block runs in its own coroutine, or in a child that is cancelled
// with it: lambdas passed to an inline or a suspend function, and the body of
// an unqualified launch / async. The rest runs later, or in a coroutine the
// caller's cancellation does not reach (a qualified launch, GlobalScope, a
// flow or sequence builder, runBlocking), so it is not skipped when the
// coroutine running the finally block is cancelled: the finding's message is
// false there, and FIR drops Go's findings as artifacts.
package test

import kotlin.coroutines.CoroutineContext
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow

class Repo {
    suspend fun flush() {
        println("flushed")
    }
}

suspend fun retry(block: suspend () -> Unit) {
    block()
}

// In place: lambdas passed to a suspend function run in the caller's
// coroutine. Go reports only the unqualified delay; FIR also reports the
// qualified collect, the unlisted retry and the project flush, since each
// suspends and so throws once the caller is cancelled.
suspend fun inPlace(source: Flow<Int>, repo: Repo) {
    try {
        println("working")
    } finally {
        source.<!SuspendFunInFinallySection!>collect<!> { value ->
            <!SuspendFunInFinallySection!>delay(value.toLong())<!>
        }
        <!SuspendFunInFinallySection!>retry<!> {
            <!SuspendFunInFinallySection!>repo.flush()<!>
            <!SuspendFunInFinallySection!>delay(1)<!>
        }
    }
}

// Detached: a non-suspend owner starts coroutines on other scopes. Their
// bodies run later, in coroutines this owner's finally block does not share.
// Go reports the unqualified delay in the GlobalScope body; FIR drops it.
// Neither reports the qualified flush calls.
fun onStop(scope: CoroutineScope, repo: Repo) {
    try {
        println("working")
    } finally {
        scope.launch { repo.flush() }
        GlobalScope.launch {
            delay(1)
        }
    }
}

// Detached: a flow builder's body runs when the flow is collected, not in the
// finally block. Go reports the delay; FIR drops it.
fun flowInFinally(repo: Repo): Flow<Int> {
    try {
        println("working")
    } finally {
        return flow<Int> {
            repo.flush()
            delay(1)
        }
    }
}

// Detached: a sequence builder's body runs when the sequence is iterated, and
// its yield is a restricted suspension with no Job to cancel. Go reports the
// yield in both functions; FIR drops them.
fun sequenceInFinally(): Sequence<Int> {
    try {
        println("working")
    } finally {
        return sequence { yield(1) }
    }
}

val numbers = sequence {
    try {
        yield(1)
    } finally {
        yield(2)
    }
}

// Detached: local functions, classes and objects declared in a finally block
// run only when called, and a stored suspend lambda only when invoked. Go
// reports the delay in each body; FIR drops them. A call of the local suspend
// function is reported where it is made (genericAndLocal in
// SuspendFunInFinallySectionDivergence.kt).
suspend fun localDeclarations(repo: Repo) {
    try {
        println("working")
    } finally {
        suspend fun later() {
            repo.flush()
            delay(1)
        }
        class Cleanup {
            suspend fun run() {
                delay(2)
            }
        }
        val cleanup = object {
            suspend fun run() {
                delay(3)
            }
        }
        val stored: suspend () -> Unit = { delay(4) }
        println(::later)
        println(Cleanup())
        println(cleanup)
        println(stored)
    }
}

// A builder whose context hands the new coroutine a Job that may be the
// caller's runs as that Job's child, so it is cancelled with the caller: FIR
// walks its body. Go reports the delay in both; FIR reports it too, and the
// flush Go misses (qualified). With a Job-free context the coroutine is not
// the caller's child: Go reports the delay, FIR drops it.
fun explicitContext(scope: CoroutineScope, job: Job, context: CoroutineContext, repo: Repo) {
    try {
        println("working")
    } finally {
        scope.launch(job) {
            <!SuspendFunInFinallySection!>repo.flush()<!>
            <!SuspendFunInFinallySection!>delay(1)<!>
        }
        GlobalScope.launch(context) {
            <!SuspendFunInFinallySection!>delay(2)<!>
        }
        GlobalScope.launch(kotlinx.coroutines.Dispatchers.IO) {
            delay(3)
        }
    }
}

// A safe call (`job?.join()`) is qualified, so Go misses it; FIR reports it
// because join is a suspend function.
suspend fun safeCall(job: Job?) {
    try {
        println("working")
    } finally {
        job?.<!SuspendFunInFinallySection!>join()<!>
    }
}
