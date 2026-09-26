// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34, 42, 53, 61, 71
// Where the resolved call and the Go rule's syntax match disagree. Go reports
// any call written `x.await()` under a finally block unless a call named
// runCatching encloses it; FIR reports an await that awaits a Deferred or
// bridges a future into a coroutine (a member of Deferred, an await extension
// on a Deferred, any suspend await extension, or an await declared in
// kotlinx.coroutines) and runs in the finally block unguarded.
package test

import java.util.concurrent.CompletionStage
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.Deferred

class Gate {
    suspend fun await() {}
}

// A project's own suspend await bridge, not kotlinx.coroutines.future.await.
suspend fun <T> CompletionStage<T>.await(): T = TODO()

// A blocking await extension on a type that is not a Deferred.
fun CountDownLatch.await(timeoutMs: Long): Boolean = await(timeoutMs, TimeUnit.MILLISECONDS)

// Go reports these because the call is written `.await()`; FIR is correct to
// drop them because none awaits a Deferred or bridges a future into a
// coroutine: a Java CountDownLatch member, a local class member, a member of
// an object expression, and a blocking extension on a CountDownLatch.
fun javaLatch(latch: CountDownLatch) {
    try {
        println("working")
    } finally {
        latch.await()
    }
}

suspend fun lookalike(gate: Gate) {
    try {
        println("working")
    } finally {
        gate.await()
    }
}

suspend fun anonymousLookalike() {
    val barrier = object {
        suspend fun await() {}
    }
    try {
        println("working")
    } finally {
        barrier.await()
    }
}

fun blockingExtension(latch: CountDownLatch) {
    try {
        println("working")
    } finally {
        latch.await(100L)
    }
}

// Go reports this and so does FIR: a project's suspend await extension on a
// future rethrows the future's failure like kotlinx.coroutines.future.await.
suspend fun localExtension(stage: CompletionStage<Unit>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>stage.await()<!>
    }
}

// Go misses these because the call has no `.await` navigation; FIR reports
// them because the implicit receiver is a Deferred.
suspend fun implicitReceiver(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        with(cleanup) { <!DeferredAwaitInFinally!>await()<!> }
    }
}

suspend fun Deferred<Unit>.implicitExtensionReceiver() {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>await()<!>
    }
}

// Go exempts these because a call named runCatching encloses the await; FIR
// reports them because the runCatching does not guard it. The await is (or is
// inside) the receiver of runCatching, so it runs (and throws) first; or the
// runCatching is outside the finally block and only catches the await's
// exception after it has replaced the try block's.
suspend fun awaitIsReceiver(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>cleanup.await()<!>.runCatching { println(this) }
        listOf(<!DeferredAwaitInFinally!>cleanup.await()<!>).runCatching { println(this) }
    }
}

suspend fun runCatchingOutsideFinally(cleanup: Deferred<Unit>) {
    runCatching {
        try {
            println("working")
        } finally {
            <!DeferredAwaitInFinally!>cleanup.await()<!>
        }
    }
}

// Go misses this because tree-sitter does not parse an object expression that
// delegates an interface (`object : Deferred<Int> by delegate {}`), so it
// never sees the await; FIR reports it because the object is a Deferred.
// Kept last: the parse failure can swallow the declarations after it.
suspend fun anonymousSubtype(delegate: Deferred<Int>) {
    val anonymous = object : Deferred<Int> by delegate {}
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>anonymous.await()<!>
    }
}
