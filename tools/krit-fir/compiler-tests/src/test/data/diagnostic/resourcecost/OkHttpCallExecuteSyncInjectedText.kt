// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 22, 26, 30
// An injected dispatcher exempts the call only when an enclosing call reads it: a comment or string that names
// `appDispatchers.io`, or an IO block beside the call, leaves the call on the main dispatcher.
package test.okhttpcallexecutesyncinjectedtext

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.withContext
import okhttp3.Call
import okhttp3.Response

class AppDispatchers(val io: CoroutineDispatcher, val main: CoroutineDispatcher)

fun named(name: String): CoroutineDispatcher = TODO(name)

class Fetcher(private val appDispatchers: AppDispatchers) {
    suspend fun commented(call: Call): Response =
        withContext(appDispatchers.main /* appDispatchers.io */) {
            call.<!OkHttpCallExecuteSync!>execute<!>()
        }

    suspend fun quoted(call: Call): Response = withContext(named("appDispatchers.io")) { call.<!OkHttpCallExecuteSync!>execute<!>() }

    suspend fun nested(call: Call): Response = withContext(appDispatchers.main) {
        withContext(appDispatchers.io) { println() }
        call.<!OkHttpCallExecuteSync!>execute<!>()
    }

    suspend fun direct(call: Call): Response =
        withContext(appDispatchers.main) { call.<!OkHttpCallExecuteSync!>execute<!>() }

    suspend fun onIo(call: Call): Response = withContext(appDispatchers.io) { call.execute() }
}
