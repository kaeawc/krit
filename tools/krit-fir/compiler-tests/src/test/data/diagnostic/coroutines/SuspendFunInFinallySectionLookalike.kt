// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 33, 34, 43, 44, 45, 57, 66
// Project and Java functions that only share a name with a well-known suspend
// function or coroutine builder. Go matches the call text against its name list and
// reports every call below; FIR is correct to drop them because none of these
// calls is a suspend function or starts a coroutine, so nothing is skipped
// when the caller is cancelled.
package test

fun delay(ms: Long) {
    println(ms)
}

fun send(value: Int) {
    println(value)
}

fun launch(block: () -> Unit) {
    block()
}

class Pump {
    fun emit(value: Int) {
        println(value)
    }

    fun await(): Int = 0

    fun drain() {
        try {
            println("working")
        } finally {
            emit(1)
            await()
        }
    }
}

suspend fun lookalikes() {
    try {
        println("working")
    } finally {
        delay(100)
        send(1)
        launch { println("inline") }
    }
}

// Java lookalikes: Thread.join and CountDownLatch.await block the thread but
// are not suspend functions, so cancelling a coroutine cannot skip them. Go
// reports both by the name; FIR drops them.
class ShutdownThread : Thread() {
    fun shutdown() {
        try {
            interrupt()
        } finally {
            join()
        }
    }
}

fun latchWait(latch: java.util.concurrent.CountDownLatch) {
    try {
        println("working")
    } finally {
        with(latch) { await() }
    }
}
