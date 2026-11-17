// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 25, 29, 33, 37, 40, 42, 45, 50, 54
// Synchronous okhttp3.Call.execute() in a suspend function with no sign of a background context.
package test.okhttpcallexecutesync

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.coroutines.Dispatchers
import okhttp3.Call
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response

class Fetcher(private val client: OkHttpClient) {
    suspend fun direct(call: Call): String? {
        val response = call.<!OkHttpCallExecuteSync!>execute<!>()
        return response.body?.string()
    }

    suspend fun chained(url: String): Int {
        val request = Request.Builder().url(url).build()
        return client.newCall(request).<!OkHttpCallExecuteSync!>execute<!>().use { it.code }
    }

    suspend fun inPlaceLambda(calls: List<Call>) {
        calls.forEach { queuedCall -> queuedCall.<!OkHttpCallExecuteSync!>execute<!>().close() }
    }

    suspend fun suspendLambda(call: Call): Response = coroutineScope {
        call.<!OkHttpCallExecuteSync!>execute<!>()
    }

    suspend fun mainContext(call: Call): Response = withContext(Dispatchers.Main) {
        call.<!OkHttpCallExecuteSync!>execute<!>()
    }

    suspend fun implementor(call: RealCall): Response = call.<!OkHttpCallExecuteSync!>execute<!>()

    suspend fun safeCall(call: Call?): Response? = call?.<!OkHttpCallExecuteSync!>execute<!>()

    suspend fun launched(scope: CoroutineScope, call: Call) {
        scope.launch { call.<!OkHttpCallExecuteSync!>execute<!>() }
    }

    // The main dispatcher of an injected holder is not a background thread.
    suspend fun injectedMain(call: Call, dispatchers: AppDispatchers): Response = withContext(dispatchers.main) {
        call.<!OkHttpCallExecuteSync!>execute<!>()
    }

    suspend fun localSuspend(call: Call) {
        suspend fun inner(): Response = call.<!OkHttpCallExecuteSync!>execute<!>()
        inner()
    }
}

abstract class RealCall : Call

class AppDispatchers(val io: CoroutineDispatcher, val main: CoroutineDispatcher)
