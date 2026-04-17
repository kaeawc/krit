package fixtures.negative.coroutines

import kotlinx.coroutines.delay

suspend fun work() {
    println("starting")
    delay(1000)
    println("done")
}
