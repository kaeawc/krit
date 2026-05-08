package fixtures.positive.coroutines

import kotlinx.coroutines.*

suspend fun work() {
    println("starting")
    Thread.sleep(1000)
    println("done")
}
