// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// Positive: Retrofit's `Call<T>.await()` bridge. It rethrows the call's failure
// like Deferred.await(), so in a finally block it can replace the try block's
// exception. Go reports it too. The bridge is declared here with its library
// signature because the stubs carry retrofit2.Call but not its Kotlin
// extensions.
package retrofit2

suspend fun <T : Any> Call<T>.await(): T = TODO()

suspend fun report(call: Call<String>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>call.await()<!>
    }
}
