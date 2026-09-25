// Smoke: builders with dispatchers as CoroutineContext, context composition,
// Job/Deferred APIs, exception handlers, and structured concurrency helpers.
// Kept at top level so INJECT_DISPATCHER (member-only) stays silent.
package stubs

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineExceptionHandler
import kotlinx.coroutines.CoroutineName
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.DelicateCoroutinesApi
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.MainScope
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.cancel
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.supervisorScope
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeout
import kotlinx.coroutines.withTimeoutOrNull
import kotlin.coroutines.CoroutineContext

suspend fun loadAll(scope: CoroutineScope): List<Int> {
    val handler = CoroutineExceptionHandler { _, throwable -> println(throwable) }
    val job: Job = scope.launch(Dispatchers.IO + handler) {
        withContext(Dispatchers.Default) { delay(10) }
        ensureActive()
    }
    scope.launch(start = CoroutineStart.LAZY) {}.start()
    val deferred: Deferred<Int> = scope.async(Dispatchers.Default) { 1 }
    val many: List<Deferred<Int>> = listOf(deferred, scope.async { 2 })
    job.join()
    job.cancel()
    return coroutineScope {
        val first = deferred.await()
        val rest = many.awaitAll()
        withContext(NonCancellable) { delay(1) }
        listOf(first) + rest
    }
}

fun buildScopes(dispatcher: CoroutineDispatcher): CoroutineContext {
    val context: CoroutineContext = SupervisorJob() + Dispatchers.Main + CoroutineName("smoke")
    val scope = CoroutineScope(context)
    val main = MainScope()
    val child = Job(parent = context[Job])
    println(scope.isActive)
    main.cancel()
    child.invokeOnCompletion { cause -> println(cause) }
    return dispatcher + Dispatchers.Main.immediate + Dispatchers.IO.limitedParallelism(2)
}

@OptIn(DelicateCoroutinesApi::class)
fun fireAndForget() {
    GlobalScope.launch { supervisorScope { launch { delay(1) } } }
}

fun blocking(): Int? = runBlocking {
    withTimeout(100) { delay(1) }
    withTimeoutOrNull(100L) { 42 }
}
