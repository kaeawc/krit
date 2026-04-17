package coroutines

import kotlinx.coroutines.CancellationException

suspend fun doWork() {
    try {
        println("working")
    } catch (e: CancellationException) {
        println("cancelled: $e")
        throw e
    }
}
