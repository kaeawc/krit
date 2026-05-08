package test

interface Counter {
    fun increment()
    fun increment(amount: Double)
}

interface Gauge {
    fun decrement()
}

fun handle(counter: Counter, gauge: Gauge) {
    counter.increment()
    counter.increment(1.0)
    gauge.decrement()
}
