// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 31
// Where the resolved call and the Go rule's syntax match disagree. Go reports
// any call written `x.await()` under a finally block unless a call named
// runCatching encloses it; FIR reports an await declared by
// kotlinx.coroutines.Deferred that runs in the finally block unguarded.
package test

import java.util.concurrent.CountDownLatch
import kotlinx.coroutines.Deferred

class Gate {
    suspend fun await() {}
}

// Go reports these because the call is written `.await()`; FIR is correct to
// drop them because neither is Deferred.await(): a Java CountDownLatch and a
// local class.
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
// reports them because the runCatching does not guard it. The await is the
// receiver of runCatching, so it runs (and throws) first; or the runCatching
// is outside the finally block and only catches the await's exception after
// it has replaced the try block's.
suspend fun awaitIsReceiver(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>cleanup.await()<!>.runCatching { println(this) }
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
