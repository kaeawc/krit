// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 34, 38
// Divergences from Go, which decides by the written name of the receiver and ignores lambdas.
package test.okhttpcallexecutesyncdivergence

import okhttp3.Call
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response

class Job {
    fun execute(): String = ""
}

class Divergence(private val client: OkHttpClient) {
    // Go reports a receiver named like a call; Job is not an okhttp3.Call.
    suspend fun lookalike(prepared: Job) = prepared.execute()

    suspend fun lookalikeCall(refreshCall: Job) = refreshCall.execute()

    // Go reads the receiver name and misses a call held in a variable that is not named like one.
    suspend fun anyName(): Response {
        val pending = client.newCall(Request.Builder().url("https://example.com").build())
        return pending.<!OkHttpCallExecuteSync!>execute<!>()
    }

    // Go misses the receiver `it`: the name does not look like a call.
    suspend fun inPlaceLambda(calls: List<Call>) {
        calls.forEach { it.<!OkHttpCallExecuteSync!>execute<!>().close() }
    }

    // Go reports a call inside a lambda the compiler runs on another thread; the coroutine's thread is free.
    suspend fun threaded(call: Call) {
        Thread { call.execute() }.start()
    }

    suspend fun stored(call: Call) {
        val task: () -> Unit = { call.execute() }
        task.invoke()
    }
}
