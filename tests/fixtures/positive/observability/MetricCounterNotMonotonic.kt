package test

interface Counter {
    fun increment(amount: Double)
}

fun handle(counter: Counter) {
    counter.increment(-1.0)
}
