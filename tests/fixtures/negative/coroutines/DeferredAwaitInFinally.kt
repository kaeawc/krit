package coroutines

import kotlinx.coroutines.Deferred

suspend fun doWork(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        runCatching { cleanup.await() }
    }
}

suspend fun doCaughtWork(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        try {
            cleanup.await()
        } catch (e: Exception) {
            println("cleanup failed")
        }
    }
}
