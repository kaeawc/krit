package coroutines

import kotlinx.coroutines.delay

suspend fun doWork() {
    try {
        println("working")
    } finally {
        delay(100)
    }
}
