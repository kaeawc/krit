// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 39x2, 40, 42x2, 43, 48, 56x3, 57x2, 72x3, 73x3, 74x3, 75x3, 77x3, 79, 92, 93, 135, 152x2, 153x2, 155x3, 168x2
// Where FIR resolution and the Go rule's call-text match disagree. Go reports
// an unqualified call whose name is on its fixed list of suspend functions and
// coroutine builders; FIR reports every call that resolves to a suspend
// function, and skips the contexts whose body runs even after cancellation.
package test

import kotlin.coroutines.CoroutineContext
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.suspendCoroutine
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.NonCancellable as Shielded
import kotlinx.coroutines.async
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.delay as pause
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeout
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first

// Go reports withContext (twice, see trailingLambdas) and the delay inside it;
// FIR is correct to drop them:
// withContext(NonCancellable) is the fix this rule asks for, and its block runs
// even when the caller is cancelled.
suspend fun nonCancellable(scope: CoroutineScope) {
    try {
        println("working")
    } finally {
        withContext(NonCancellable) {
            delay(1)
        }
        withContext(NonCancellable + Dispatchers.IO) {
            delay(2)
        }
        // Go reports only the delay (the launch is qualified); the child's
        // parent is NonCancellable, so its body runs.
        scope.launch(NonCancellable) {
            delay(3)
        }
        // Unqualified, Go also reports launch and async (each twice, see
        // trailingLambdas) besides the delay; FIR drops them with their
        // bodies: neither builder is a suspend function, and a child whose
        // parent is NonCancellable runs its body even when the scope is
        // cancelled.
        with(scope) {
            launch(NonCancellable) { delay(4) }
            async(NonCancellable) { 1 }
        }
    }
}

// The same NonCancellable context written other ways: a named argument, an
// import alias, a local val, a smart cast, and on either side of `+`. Go
// reports each withContext twice and each delay once; FIR drops them all. A
// context FIR cannot prove NonCancellable (a plain CoroutineContext) is
// reported by both.
suspend fun nonCancellableShapes(context: CoroutineContext) {
    val guard = NonCancellable
    try {
        println("working")
    } finally {
        withContext(context = NonCancellable) { delay(1) }
        withContext(Shielded) { delay(2) }
        withContext(guard) { delay(3) }
        withContext(Dispatchers.IO + NonCancellable) { delay(4) }
        if (context is NonCancellable) {
            withContext(context) { delay(5) }
        }
        <!SuspendFunInFinallySection!>withContext(context + EmptyCoroutineContext, {})<!>
    }
}

// Go reports runBlocking and the delay inside it; FIR is correct to drop both:
// runBlocking is not a suspend function, and without a context argument it
// starts a fresh coroutine that is not cancelled with the caller, so its block
// runs (blockingWithContext in SuspendFunInFinallySectionContext.kt covers a
// context that joins the caller's Job).
fun blockingCleanup() {
    try {
        println("working")
    } finally {
        runBlocking {
            delay(1)
        }
    }
}

suspend fun flush() {
    println("flushed")
}

// Go misses these because the call is qualified, or its name is not on Go's
// list; FIR reports them because each resolves to a suspend function and so
// throws instead of running once the caller is cancelled.
suspend fun qualifiedAndUnlisted(job: Job, deferred: Deferred<Int>, channel: Channel<Int>, source: Flow<Int>, block: suspend () -> Unit) {
    try {
        println("working")
    } finally {
        <!SuspendFunInFinallySection!>job.join()<!>
        <!SuspendFunInFinallySection!>deferred.await()<!>
        <!SuspendFunInFinallySection!>job.cancelAndJoin()<!>
        <!SuspendFunInFinallySection!>channel.send(1)<!>
        <!SuspendFunInFinallySection!>channel.receive()<!>
        <!SuspendFunInFinallySection!>source.first()<!>
        <!SuspendFunInFinallySection!>flush()<!>
        <!SuspendFunInFinallySection!>currentCoroutineContext()<!>
        <!SuspendFunInFinallySection!>block()<!>
        <!SuspendFunInFinallySection!>pause(1)<!>
    }
}

// Go misses `async<Int>` (its text match expects `async(`, `async {` or
// `async{`) and the call of a local suspend function; FIR reports both. Go
// reports the delay inside the local function, where it is declared; FIR
// reports the call instead, where the finally block runs it (see
// localDeclarations in SuspendFunInFinallySectionScopes.kt).
suspend fun genericAndLocal(scope: CoroutineScope) {
    try {
        println("working")
    } finally {
        with(scope) {
            <!SuspendFunInFinallySection!>async<Int> { 1 }<!>
        }
        suspend fun local() {
            delay(4)
        }
        <!SuspendFunInFinallySection!>local()<!>
    }
}

// Go reports a call written with parenthesized arguments and a trailing lambda
// twice, because tree-sitter nests the lambda's call around the argument
// call and both start with the name; FIR reports each call once. A call
// chained after it (`.also { }`) is a third call_expression whose text starts
// with the name, so Go reports that line three times. Go misses
// `suspendCoroutine<Unit>` (its text match expects `suspendCoroutine(`,
// `suspendCoroutine {` or `suspendCoroutine{`); FIR reports it.
suspend fun trailingLambdas() {
    try {
        println("working")
    } finally {
        <!SuspendFunInFinallySection!>withContext(Dispatchers.IO) { }<!>
        <!SuspendFunInFinallySection!>withTimeout(10) { }<!>
        <!SuspendFunInFinallySection!>suspendCoroutine<Unit> { }<!>
        <!SuspendFunInFinallySection!>withContext(Dispatchers.IO) { 1 }<!>.also { println(it) }
    }
}

// Go reports a call inside a nested finally once for each enclosing finally
// block (twice here); FIR reports it once.
suspend fun nestedFinally() {
    try {
        println("outer")
    } finally {
        try {
            println("inner")
        } finally {
            <!SuspendFunInFinallySection!>delay(1)<!>
        }
    }
}
