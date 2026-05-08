package coroutines

import kotlinx.coroutines.Deferred

suspend fun doWork(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        runCatching { cleanup.await() }
    }
}
