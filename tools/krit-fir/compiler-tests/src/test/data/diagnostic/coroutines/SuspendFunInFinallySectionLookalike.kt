// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 33, 34, 43, 44, 45
// Project functions that only share a name with a well-known suspend function
// or coroutine builder. Go matches the call text against its name list and
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
