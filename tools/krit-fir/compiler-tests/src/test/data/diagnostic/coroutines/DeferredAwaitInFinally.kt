// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 25, 34, 43, 52x2, 61, 64, 66, 75, 84, 93, 102, 103, 112, 125, 126, 138
// Positive: Deferred.await() inside a finally block, in every shape the Go
// rule reports. Go agrees on every case in this file.
package test

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope

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
