// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 32, 41, 50, 59x2, 68, 71, 73, 82, 91, 100, 109, 110, 119, 132, 133, 145, 159, 160, 161, 162, 163, 164, 178, 179, 180, 181, 182, 183x2
// Positive: Deferred.await() inside a finally block, in every shape the Go
// rule reports. Go agrees on every case in this file.
package test

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.supervisorScope
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeout

suspend fun basic(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>cleanup.await()<!>
    }
}

suspend fun assigned(result: Deferred<Int>): Int {
    var value = 0
    try {
        println("working")
    } finally {
        value = <!DeferredAwaitInFinally!>result.await()<!>
    }
    return value
}

suspend fun safeCall(cleanup: Deferred<Unit>?) {
    try {
        println("working")
    } finally {
        cleanup?.<!DeferredAwaitInFinally!>await()<!>
    }
}

// Reported on the line the call expression starts on: the receiver's.
suspend fun splitChain(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>cleanup<!>
            .await()
    }
}

suspend fun twoOnOneLine(a: Deferred<Int>, b: Deferred<Int>) {
    try {
        println("working")
    } finally {
        println(<!DeferredAwaitInFinally!>a.await()<!> + <!DeferredAwaitInFinally!>b.await()<!>)
    }
}

suspend fun nestedInFinally(cleanup: Deferred<Unit>, flag: Boolean) {
    try {
        println("working")
    } finally {
        if (flag) {
            <!DeferredAwaitInFinally!>cleanup.await()<!>
        }
        try {
            <!DeferredAwaitInFinally!>cleanup.await()<!>
        } catch (e: IllegalStateException) {
            <!DeferredAwaitInFinally!>cleanup.await()<!>
        }
    }
}

suspend fun inlineLambda(all: List<Deferred<Unit>>) {
    try {
        println("working")
    } finally {
        all.forEach { <!DeferredAwaitInFinally!>it.await()<!> }
    }
}

// coroutineScope runs its block in place, inside the finally block.
suspend fun scopeBuilder(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        coroutineScope { <!DeferredAwaitInFinally!>cleanup.await()<!> }
    }
}

// The async block runs elsewhere, but the await on its result is here.
suspend fun awaitOnAsyncResult(scope: CoroutineScope) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>scope.async { 1 }.await()<!>
    }
}

class Holder(val pending: Deferred<String>) {
    suspend fun close() {
        try {
            println("working")
        } finally {
            <!DeferredAwaitInFinally!>pending.await()<!>
            <!DeferredAwaitInFinally!>this.pending.await()<!>
        }
    }
}

suspend fun Deferred<Int>.drain() {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>this.await()<!>
    }
}

// Subtypes of Deferred: a delegating class and a local class.
class Wrapped(delegate: Deferred<Int>) : Deferred<Int> by delegate

suspend fun subtypes(delegate: Deferred<Int>) {
    val wrapped = Wrapped(delegate)
    class Local(d: Deferred<Int>) : Deferred<Int> by d
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>wrapped.await()<!>
        <!DeferredAwaitInFinally!>Local(delegate).await()<!>
    }
}

// Go's runCatching lookup stops at the nearest named function, so a local
// function declared inside runCatching does not exempt its await.
suspend fun localFunctionInsideRunCatching(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        runCatching {
            suspend fun inner() {
                <!DeferredAwaitInFinally!>cleanup.await()<!>
            }
        }
    }
}

// An await in a launch or async block started in the finally block. A scope
// builder in the finally block (coroutineScope, withContext, runBlocking,
// withTimeout) waits for its children and rethrows a child's failure, so the
// await's exception still replaces the try block's.
suspend fun launchedInScopeBuilder(cleanup: Deferred<Unit>, pending: List<Deferred<Unit>>) {
    try {
        println("working")
    } finally {
        coroutineScope { launch { <!DeferredAwaitInFinally!>cleanup.await()<!> } }
        coroutineScope { async { <!DeferredAwaitInFinally!>cleanup.await()<!> } }
        withContext(NonCancellable) { launch { <!DeferredAwaitInFinally!>cleanup.await()<!> } }
        runBlocking { launch { <!DeferredAwaitInFinally!>cleanup.await()<!> } }
        withTimeout(1000L) { launch { <!DeferredAwaitInFinally!>cleanup.await()<!> } }
        coroutineScope { pending.map { async { <!DeferredAwaitInFinally!>it.await()<!> } }.awaitAll() }
    }
}

// On an explicit scope or under supervisorScope, whether the child's failure
// can replace the original exception depends on the scope it fails into: an
// explicit scope may be a parent that encloses the finally block. The checker
// cannot prove the scope is unrelated, so it reports these like Go. The inner
// await of `async { ... }.await()` is reported too (two findings on that
// line, like Go): its exception comes out of the outer await.
suspend fun launchedOnScope(scope: CoroutineScope, cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        scope.launch { <!DeferredAwaitInFinally!>cleanup.await()<!> }
        scope.async { <!DeferredAwaitInFinally!>cleanup.await()<!> }
        scope.launch(block = { <!DeferredAwaitInFinally!>cleanup.await()<!> })
        scope.launch { listOf(cleanup).forEach { <!DeferredAwaitInFinally!>it.await()<!> } }
        supervisorScope { launch { <!DeferredAwaitInFinally!>cleanup.await()<!> } }
        <!DeferredAwaitInFinally!>scope<!>.async { <!DeferredAwaitInFinally!>cleanup.await()<!> }.await()
    }
}
