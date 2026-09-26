// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 30, 47, 48, 49
// Positive: a project's own await extension on a Deferred, declared outside
// kotlinx.coroutines. The receiver is a Deferred and the call awaits it, so
// like Deferred.await() it can replace the try block's exception. Go reports
// each one because the call is written `.await()`.
package test

import kotlinx.coroutines.Deferred
import kotlinx.coroutines.withTimeout

suspend fun <T> Deferred<T>.await(timeoutMs: Long): T = withTimeout(timeoutMs) { await() }

suspend fun topLevelExtension(d: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>d.await(100L)<!>
    }
}

// A member extension: the containing class is H, not a Deferred.
class H {
    suspend fun <T> Deferred<T>.await(ms: Long): T = withTimeout(ms) { await() }

    suspend fun use(d: Deferred<Unit>) {
        try {
            println("working")
        } finally {
            <!DeferredAwaitInFinally!>d.await(1L)<!>
        }
    }
}

// Not suspend, but the receiver is a Deferred (declared as a Deferred, a
// nullable Deferred, or a type parameter bounded by Deferred).
fun <T> Deferred<T>.await(fallback: () -> T): T = if (isCompleted) getCompleted() else fallback()

fun <T> Deferred<T>?.await(missing: Boolean): T? = if (missing) null else this?.getCompleted()

fun <D : Deferred<*>> D.await(tag: String): D = this

fun nonSuspendDeferredExtensions(d: Deferred<Int>, maybe: Deferred<Int>?) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>d.await { 0 }<!>
        <!DeferredAwaitInFinally!>maybe.await(true)<!>
        <!DeferredAwaitInFinally!>d.await("tag")<!>
    }
}
