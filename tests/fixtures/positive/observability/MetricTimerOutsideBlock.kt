package test

interface Timer {
    fun <T> record(block: () -> T): T
}

data class Holder(val field: String)

fun handle(timer: Timer, holder: Holder) {
    timer.record { holder.field }
}

fun empty(timer: Timer) {
    timer.record { }
}
