package test

import java.util.concurrent.atomic.AtomicInteger

object Counter {
    private val total = AtomicInteger(0)
    private var unused = 0
    var endpoint = "x"
        private set
    var queue = java.util.concurrent.ConcurrentLinkedQueue<Int>()
    var adder = java.util.concurrent.atomic.LongAdder()
}
