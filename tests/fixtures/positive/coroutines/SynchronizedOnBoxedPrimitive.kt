package test

class Counter {
    val count: Int = 1

    fun work() {
        synchronized(count) {
            println("work")
        }
    }
}
