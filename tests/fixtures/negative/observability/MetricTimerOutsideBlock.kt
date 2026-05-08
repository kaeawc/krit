package test

interface Timer {
    fun <T> record(block: () -> T): T
}

fun handle(timer: Timer) {
    timer.record { expensiveIo() }
}

fun expensiveIo(): String = "ok"
