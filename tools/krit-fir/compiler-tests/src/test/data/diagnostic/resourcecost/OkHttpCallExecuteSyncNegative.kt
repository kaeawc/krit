// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Calls Go and FIR both leave alone.
package test.okhttpcallexecutesyncnegative

import java.util.concurrent.Executor
import kotlin.coroutines.resume
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import okhttp3.Call
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response

class Fetcher(private val client: OkHttpClient, private val backgroundExecutor: Executor) {
    // Not a suspend function: blocking is the caller's decision.
    fun blocking(call: Call): Response = call.execute()

    suspend fun onIo(call: Call): Response = withContext(Dispatchers.IO) {
        call.execute()
    }

    suspend fun onDefault(call: Call): Response = withContext(Dispatchers.Default) { call.execute() }

    suspend fun namedDispatcher(call: Call, ioDispatcher: kotlin.coroutines.CoroutineContext): Response =
        withContext(ioDispatcher) { call.execute() }

    suspend fun executorBacked(call: Call) {
        backgroundExecutor.execute { call.execute() }
    }

    suspend fun cancellableBridge(request: Request): Response = suspendCancellableCoroutine { continuation ->
        val call = client.newCall(request)
        continuation.invokeOnCancellation { call.cancel() }
        continuation.resume(call.execute())
    }

    suspend fun enqueue(call: Call, callback: okhttp3.Callback) {
        call.enqueue(callback)
    }

    /** Callers must run this on [Dispatchers.IO]. */
    suspend fun documented(call: Call): Response = call.execute()

    // A request that is built but never executed.
    suspend fun build(url: String): Call = client.newCall(Request.Builder().url(url).build())

    fun launched(scope: CoroutineScope, call: Call) {
        // A suspend lambda in a plain function is not a suspend function.
        scope.launch { call.execute() }
    }
}

// Not an okhttp3.Call: Go reports it by the receiver name (see OkHttpCallExecuteSyncDivergence).
class Tracker {
    fun execute() {}
}

suspend fun track(tracker: Tracker) {
    tracker.execute()
}
