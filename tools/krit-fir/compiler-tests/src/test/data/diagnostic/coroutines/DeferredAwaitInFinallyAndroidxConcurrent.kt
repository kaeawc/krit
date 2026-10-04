// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18
// Positive: androidx.concurrent.futures' await bridge on a ListenableFuture
// (CameraX's `cameraProviderFuture.await()`). It rethrows the future's failure
// like Deferred.await(), so in a finally block it can replace the try block's
// exception. Go reports it too. The bridge is declared here with its library
// signature because the stubs do not carry androidx.concurrent.futures.
package androidx.concurrent.futures

import com.google.common.util.concurrent.ListenableFuture

suspend fun <T> ListenableFuture<T>.await(): T = TODO()

suspend fun cameraProvider(cameraProviderFuture: ListenableFuture<String>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>cameraProviderFuture.await()<!>
    }
}
