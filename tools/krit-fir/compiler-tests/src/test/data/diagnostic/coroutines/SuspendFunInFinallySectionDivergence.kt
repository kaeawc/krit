// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 47, 53, 71x2, 72x2, 75x2, 77, 92, 136, 149, 150, 151, 152, 165x2
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

// withContext(NonCancellable) is the fix this rule asks for, and its block runs
// even when the caller is cancelled: neither Go nor FIR reports it or the delay
// inside it.
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
        // Go reports the delay (it only skips a NonCancellable withContext);
        // FIR drops it: the child's parent is NonCancellable, so its body runs.
        scope.launch(NonCancellable) {
            delay(3)
        }
        // Unqualified, the same: Go reports the delay, FIR drops it, since a
        // child whose parent is NonCancellable runs its body even when the
        // scope is cancelled. Neither reports launch or async themselves.
        with(scope) {
            launch(NonCancellable) { delay(4) }
            async(NonCancellable) { 1 }
        }
    }
}

// The same NonCancellable context written other ways: a named argument, an
// import alias, a local val, a smart cast, and on either side of `+`. Go skips
// the forms whose text names NonCancellable and reports withContext and the
// delay on the others (import alias, local val, smart cast); FIR drops them
// all. A context FIR cannot prove NonCancellable (a plain CoroutineContext) is
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

// Go reports the delay inside runBlocking; FIR is correct to drop it: without
// a context argument runBlocking starts a fresh coroutine that is not
// cancelled with the caller, so its block runs (blockingWithContext in
// SuspendFunInFinallySectionContext.kt covers a context that joins the
// caller's Job). Neither reports runBlocking itself, which is not a suspend
// function.
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
// throws instead of running once the caller is cancelled. Neither reports
// currentCoroutineContext(): it only reads the context, so it never suspends
// and runs even after cancellation.
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
        currentCoroutineContext()
        <!SuspendFunInFinallySection!>block()<!>
        <!SuspendFunInFinallySection!>pause(1)<!>
    }
}

// Go misses the call of a local suspend function; FIR reports it. Go reports
// the delay inside the local function, where it is declared; FIR reports the
// call instead, where the finally block runs it (see localDeclarations in
// SuspendFunInFinallySectionScopes.kt). Neither reports `async<Int>`, which is
// not a suspend function.
suspend fun genericAndLocal(scope: CoroutineScope) {
    try {
        println("working")
    } finally {
        with(scope) {
            async<Int> { 1 }
        }
        suspend fun local() {
            delay(4)
        }
        <!SuspendFunInFinallySection!>local()<!>
    }
}

// A call written with parenthesized arguments and a trailing lambda, with
// type arguments, or with a call chained after it (`.also { }`): Go and FIR
// both report each call once.
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
