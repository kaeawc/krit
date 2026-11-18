// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 17, 18, 19, 27, 28, 29
// An await in the try block of a try inside the finally block whose catch
// takes Throwable cannot replace the original exception, so it is not
// reported. A catch of Exception (an Error still leaves it), a narrower catch, an
// await in the catch block itself, or a catch that throws again lets it escape.
package test

import kotlinx.coroutines.Deferred

suspend fun cleanup(a: Deferred<Unit>, b: Deferred<Unit>, c: Deferred<Unit>, d: Deferred<Unit>, e: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        try { a.await() } catch (t: Throwable) { }
        try { b.<!DeferredAwaitInFinally!>await<!>() } catch (x: kotlin.Exception) { }
        try { e.<!DeferredAwaitInFinally!>await<!>() } catch (x: IllegalStateException) { } catch (x: Exception) { }
        try { c.<!DeferredAwaitInFinally!>await<!>() } catch (x: IllegalStateException) { }
        try { } catch (x: Exception) { d.<!DeferredAwaitInFinally!>await<!>() }
    }
}

suspend fun rethrown(a: Deferred<Unit>, b: Deferred<Unit>, c: Deferred<Unit>, d: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        try { a.<!DeferredAwaitInFinally!>await<!>() } catch (t: Throwable) { throw t }
        try { b.<!DeferredAwaitInFinally!>await<!>() } catch (x: Exception) { println(x); throw IllegalStateException(x) }
        try { c.<!DeferredAwaitInFinally!>await<!>() } catch (x: Exception) { if (x is kotlinx.coroutines.CancellationException) throw x }
        try { d.await() } catch (x: Throwable) { println(x) }
    }
}
