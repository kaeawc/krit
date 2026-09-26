// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18
// Positive: androidx.work's await bridge on a ListenableFuture (the future
// WorkManager returns from enqueue and getWorkInfos). It rethrows the future's
// failure like Deferred.await(), so in a finally block it can replace the try
// block's exception. Go reports it too. The bridge is declared here (without
// the library's `inline`) because the stubs do not carry androidx.work.
package androidx.work

import com.google.common.util.concurrent.ListenableFuture

suspend fun <R> ListenableFuture<R>.await(): R = TODO()

suspend fun enqueue(operation: ListenableFuture<Unit>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>operation.await()<!>
    }
}
