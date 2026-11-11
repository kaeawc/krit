// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Thread.sleep outside a coroutine, and code that only looks like it.
package test

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlin.concurrent.thread

fun plainFunction() {
    Thread.sleep(10)
}

fun plainLambda(items: List<Int>) {
    items.map { Thread.sleep(10) }
}

suspend fun delays() {
    delay(10)
}

suspend fun storedReference() {
    val f = Thread::yield
    val g: (Long) -> Unit = Thread::sleep
}

suspend fun threadBuilder() {
    thread { Thread.sleep(10) }
}

suspend fun runnableLambda() {
    val r = Runnable { Thread.sleep(10) }
}

suspend fun localFunction() {
    fun blocking() {
        Thread.sleep(10)
    }
    blocking()
}

class Holder(private val scope: CoroutineScope) {
    val plain = Runnable { Thread.sleep(10) }

    fun start() {
        scope.launch { delay(10) }
    }
}
